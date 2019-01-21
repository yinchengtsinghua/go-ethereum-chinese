
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

//+建设！Noclipse，！无协议

package client

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/pss"
)

const (
	handshakeRetryTimeout = 1000
	handshakeRetryCount   = 3
)

//PSS客户端通过PSS RPC API提供devp2p仿真，
//允许从不同的进程访问PSS方法
type Client struct {
	BaseAddrHex string

//同龄人
	peerPool map[pss.Topic]map[string]*pssRPCRW
	protos   map[pss.Topic]*p2p.Protocol

//RPC连接
	rpc  *rpc.Client
	subs []*rpc.ClientSubscription

//渠道
	topicsC chan []byte
	quitC   chan struct{}

	poolMu sync.Mutex
}

//实现p2p.msgreadwriter
type pssRPCRW struct {
	*Client
	topic    string
	msgC     chan []byte
	addr     pss.PssAddress
	pubKeyId string
	lastSeen time.Time
	closed   bool
}

func (c *Client) newpssRPCRW(pubkeyid string, addr pss.PssAddress, topicobj pss.Topic) (*pssRPCRW, error) {
	topic := topicobj.String()
	err := c.rpc.Call(nil, "pss_setPeerPublicKey", pubkeyid, topic, hexutil.Encode(addr[:]))
	if err != nil {
		return nil, fmt.Errorf("setpeer %s %s: %v", topic, pubkeyid, err)
	}
	return &pssRPCRW{
		Client:   c,
		topic:    topic,
		msgC:     make(chan []byte),
		addr:     addr,
		pubKeyId: pubkeyid,
	}, nil
}

func (rw *pssRPCRW) ReadMsg() (p2p.Msg, error) {
	msg := <-rw.msgC
	log.Trace("pssrpcrw read", "msg", msg)
	pmsg, err := pss.ToP2pMsg(msg)
	if err != nil {
		return p2p.Msg{}, err
	}

	return pmsg, nil
}

//如果只剩下一个信息槽
//然后通过握手请求新的
//如果缓冲区为空，握手请求将一直阻塞直到返回
//在此之后，指针将更改为缓冲区中的第一个新键
//如果：
//-任何API调用失败
//-握手重试在没有回复的情况下耗尽，
//-发送失败
func (rw *pssRPCRW) WriteMsg(msg p2p.Msg) error {
	log.Trace("got writemsg pssclient", "msg", msg)
	if rw.closed {
		return fmt.Errorf("connection closed")
	}
	rlpdata := make([]byte, msg.Size)
	msg.Payload.Read(rlpdata)
	pmsg, err := rlp.EncodeToBytes(pss.ProtocolMsg{
		Code:    msg.Code,
		Size:    msg.Size,
		Payload: rlpdata,
	})
	if err != nil {
		return err
	}

//拿到钥匙
	var symkeyids []string
	err = rw.Client.rpc.Call(&symkeyids, "pss_getHandshakeKeys", rw.pubKeyId, rw.topic, false, true)
	if err != nil {
		return err
	}

//检查第一把钥匙的容量
	var symkeycap uint16
	if len(symkeyids) > 0 {
		err = rw.Client.rpc.Call(&symkeycap, "pss_getHandshakeKeyCapacity", symkeyids[0])
		if err != nil {
			return err
		}
	}

	err = rw.Client.rpc.Call(nil, "pss_sendSym", symkeyids[0], rw.topic, hexutil.Encode(pmsg))
	if err != nil {
		return err
	}

//如果这是最后一条有效的消息，则启动新的握手
	if symkeycap == 1 {
		var retries int
		var sync bool
//如果它是唯一剩余的密钥，请确保在有新的密钥可供进一步写入之前不会继续。
		if len(symkeyids) == 1 {
			sync = true
		}
//开始握手
		_, err := rw.handshake(retries, sync, false)
		if err != nil {
			log.Warn("failing", "err", err)
			return err
		}
	}
	return nil
}

//握手API调用的重试和同步包装
//成功执行后返回第一个新symkeyid
func (rw *pssRPCRW) handshake(retries int, sync bool, flush bool) (string, error) {

	var symkeyids []string
	var i int
//请求新密钥
//如果密钥缓冲区已耗尽，则将其作为阻塞调用进行，并在放弃之前尝试几次。
	for i = 0; i < 1+retries; i++ {
		log.Debug("handshake attempt pssrpcrw", "pubkeyid", rw.pubKeyId, "topic", rw.topic, "sync", sync)
		err := rw.Client.rpc.Call(&symkeyids, "pss_handshake", rw.pubKeyId, rw.topic, sync, flush)
		if err == nil {
			var keyid string
			if sync {
				keyid = symkeyids[0]
			}
			return keyid, nil
		}
		if i-1+retries > 1 {
			time.Sleep(time.Millisecond * handshakeRetryTimeout)
		}
	}

	return "", fmt.Errorf("handshake failed after %d attempts", i)
}

//自定义构造函数
//
//提供对RPC对象的直接访问
func NewClient(rpcurl string) (*Client, error) {
	rpcclient, err := rpc.Dial(rpcurl)
	if err != nil {
		return nil, err
	}

	client, err := NewClientWithRPC(rpcclient)
	if err != nil {
		return nil, err
	}
	return client, nil
}

//主要施工单位
//
//“rpc client”参数允许传递内存中的RPC客户端充当远程WebSocket RPC。
func NewClientWithRPC(rpcclient *rpc.Client) (*Client, error) {
	client := newClient()
	client.rpc = rpcclient
	err := client.rpc.Call(&client.BaseAddrHex, "pss_baseAddr")
	if err != nil {
		return nil, fmt.Errorf("cannot get pss node baseaddress: %v", err)
	}
	return client, nil
}

func newClient() (client *Client) {
	client = &Client{
		quitC:    make(chan struct{}),
		peerPool: make(map[pss.Topic]map[string]*pssRPCRW),
		protos:   make(map[pss.Topic]*p2p.Protocol),
	}
	return
}

//在PSS连接上安装新的devp2p protcool
//
//协议别名为“PSS主题”
//使用来自p2p/协议包的普通devp2p发送和传入消息处理程序例程
//
//当从客户端尚不知道的对等端接收到传入消息时，
//这个对等对象被实例化，并且协议在它上面运行。
func (c *Client) RunProtocol(ctx context.Context, proto *p2p.Protocol) error {
	topicobj := pss.BytesToTopic([]byte(fmt.Sprintf("%s:%d", proto.Name, proto.Version)))
	topichex := topicobj.String()
	msgC := make(chan pss.APIMsg)
	c.peerPool[topicobj] = make(map[string]*pssRPCRW)
	sub, err := c.rpc.Subscribe(ctx, "pss", msgC, "receive", topichex, false, false)
	if err != nil {
		return fmt.Errorf("pss event subscription failed: %v", err)
	}
	c.subs = append(c.subs, sub)
	err = c.rpc.Call(nil, "pss_addHandshake", topichex)
	if err != nil {
		return fmt.Errorf("pss handshake activation failed: %v", err)
	}

//发送传入消息
	go func() {
		for {
			select {
			case msg := <-msgC:
//我们这里只允许sym msgs
				if msg.Asymmetric {
					continue
				}
//我们通过了symkeyid
//需要symkey本身解析为对等机的pubkey
				var pubkeyid string
				err = c.rpc.Call(&pubkeyid, "pss_getHandshakePublicKey", msg.Key)
				if err != nil || pubkeyid == "" {
					log.Trace("proto err or no pubkey", "err", err, "symkeyid", msg.Key)
					continue
				}
//如果我们还没有这个协议上的对等方，请创建它
//这或多或少与addpsspeer相同，而不是握手启动
				if c.peerPool[topicobj][pubkeyid] == nil {
					var addrhex string
					err := c.rpc.Call(&addrhex, "pss_getAddress", topichex, false, msg.Key)
					if err != nil {
						log.Trace(err.Error())
						continue
					}
					addrbytes, err := hexutil.Decode(addrhex)
					if err != nil {
						log.Trace(err.Error())
						break
					}
					addr := pss.PssAddress(addrbytes)
					rw, err := c.newpssRPCRW(pubkeyid, addr, topicobj)
					if err != nil {
						break
					}
					c.peerPool[topicobj][pubkeyid] = rw
					p := p2p.NewPeer(enode.ID{}, fmt.Sprintf("%v", addr), []p2p.Cap{})
					go proto.Run(p, c.peerPool[topicobj][pubkeyid])
				}
				go func() {
					c.peerPool[topicobj][pubkeyid].msgC <- msg.Msg
				}()
			case <-c.quitC:
				return
			}
		}
	}()

	c.protos[topicobj] = proto
	return nil
}

//始终调用此函数以确保我们干净地退出
func (c *Client) Close() error {
	for _, s := range c.subs {
		s.Unsubscribe()
	}
	return nil
}

//添加PSS对等（公钥）并在其上运行协议
//
//具有匹配主题的client.runprotocol必须
//在添加对等机之前运行，否则此方法将
//返回一个错误。
//
//密钥必须存在于PSS节点的密钥存储中
//在添加对等机之前。该方法将返回一个错误
//如果不是。
func (c *Client) AddPssPeer(pubkeyid string, addr []byte, spec *protocols.Spec) error {
	topic := pss.ProtocolTopic(spec)
	if c.peerPool[topic] == nil {
		return errors.New("addpeer on unset topic")
	}
	if c.peerPool[topic][pubkeyid] == nil {
		rw, err := c.newpssRPCRW(pubkeyid, addr, topic)
		if err != nil {
			return err
		}
		_, err = rw.handshake(handshakeRetryCount, true, true)
		if err != nil {
			return err
		}
		c.poolMu.Lock()
		c.peerPool[topic][pubkeyid] = rw
		c.poolMu.Unlock()
		p := p2p.NewPeer(enode.ID{}, fmt.Sprintf("%v", addr), []p2p.Cap{})
		go c.protos[topic].Run(p, c.peerPool[topic][pubkeyid])
	}
	return nil
}

//删除PSS对等
//
//TODO:底层清理
func (c *Client) RemovePssPeer(pubkeyid string, spec *protocols.Spec) {
	log.Debug("closing pss client peer", "pubkey", pubkeyid, "protoname", spec.Name, "protoversion", spec.Version)
	c.poolMu.Lock()
	defer c.poolMu.Unlock()
	topic := pss.ProtocolTopic(spec)
	c.peerPool[topic][pubkeyid].closed = true
	delete(c.peerPool[topic], pubkeyid)
}
