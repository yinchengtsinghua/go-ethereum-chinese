
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

//+建设！NOPSS协议

package pss

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/swarm/log"
)

const (
	IsActiveProtocol = true
)

//用于通过PSS传输的devp2p协议消息的方便包装器
type ProtocolMsg struct {
	Code       uint64
	Size       uint32
	Payload    []byte
	ReceivedAt time.Time
}

//创建protocolmsg
func NewProtocolMsg(code uint64, msg interface{}) ([]byte, error) {

	rlpdata, err := rlp.EncodeToBytes(msg)
	if err != nil {
		return nil, err
	}

//TODO验证嵌套结构不能在RLP中使用
	smsg := &ProtocolMsg{
		Code:    code,
		Size:    uint32(len(rlpdata)),
		Payload: rlpdata,
	}

	return rlp.EncodeToBytes(smsg)
}

//要传递到新协议实例的协议选项
//
//参数指定允许哪些加密方案
type ProtocolParams struct {
	Asymmetric bool
	Symmetric  bool
}

//PSSReadWriter用devp2p协议发送/接收桥接PSS发送/接收
//
//实现p2p.msgreadwriter
type PssReadWriter struct {
	*Pss
	LastActive time.Time
	rw         chan p2p.Msg
	spec       *protocols.Spec
	topic      *Topic
	sendFunc   func(string, Topic, []byte) error
	key        string
	closed     bool
}

//实现p2p.msgreader
func (prw *PssReadWriter) ReadMsg() (p2p.Msg, error) {
	msg := <-prw.rw
	log.Trace(fmt.Sprintf("pssrw readmsg: %v", msg))
	return msg, nil
}

//实现p2p.msgWriter
func (prw *PssReadWriter) WriteMsg(msg p2p.Msg) error {
	log.Trace("pssrw writemsg", "msg", msg)
	if prw.closed {
		return fmt.Errorf("connection closed")
	}
	rlpdata := make([]byte, msg.Size)
	msg.Payload.Read(rlpdata)
	pmsg, err := rlp.EncodeToBytes(ProtocolMsg{
		Code:    msg.Code,
		Size:    msg.Size,
		Payload: rlpdata,
	})
	if err != nil {
		return err
	}
	return prw.sendFunc(prw.key, *prw.topic, pmsg)
}

//将p2p.msg注入msgreadwriter，使其出现在关联的p2p.msgreader上
func (prw *PssReadWriter) injectMsg(msg p2p.Msg) error {
	log.Trace(fmt.Sprintf("pssrw injectmsg: %v", msg))
	prw.rw <- msg
	return nil
}

//PSS上仿真devp2p的方便对象
type Protocol struct {
	*Pss
	proto        *p2p.Protocol
	topic        *Topic
	spec         *protocols.Spec
	pubKeyRWPool map[string]p2p.MsgReadWriter
	symKeyRWPool map[string]p2p.MsgReadWriter
	Asymmetric   bool
	Symmetric    bool
	RWPoolMu     sync.Mutex
}

//在特定PSS主题上激活devp2p仿真
//
//必须指定一个或两个加密方案。如果
//只指定了一个，协议将无效
//对于另一个，并将使消息处理程序
//返回错误
func RegisterProtocol(ps *Pss, topic *Topic, spec *protocols.Spec, targetprotocol *p2p.Protocol, options *ProtocolParams) (*Protocol, error) {
	if !options.Asymmetric && !options.Symmetric {
		return nil, fmt.Errorf("specify at least one of asymmetric or symmetric messaging mode")
	}
	pp := &Protocol{
		Pss:          ps,
		proto:        targetprotocol,
		topic:        topic,
		spec:         spec,
		pubKeyRWPool: make(map[string]p2p.MsgReadWriter),
		symKeyRWPool: make(map[string]p2p.MsgReadWriter),
		Asymmetric:   options.Asymmetric,
		Symmetric:    options.Symmetric,
	}
	return pp, nil
}

//通过devp2p仿真接收消息的通用处理程序
//
//将传递给pss.register（）。
//
//将在新的传入对等机上运行协议，前提是
//消息的加密密钥在内部具有匹配项
//PSS密钥池
//
//如果协议对消息加密方案无效，则失败，
//如果添加新对等端失败，或者消息不是序列化的
//p2p.msg（如果它是从这个对象发送的，它将一直是这样）。
func (p *Protocol) Handle(msg []byte, peer *p2p.Peer, asymmetric bool, keyid string) error {
	var vrw *PssReadWriter
	if p.Asymmetric != asymmetric && p.Symmetric == !asymmetric {
		return fmt.Errorf("invalid protocol encryption")
	} else if (!p.isActiveSymKey(keyid, *p.topic) && !asymmetric) ||
		(!p.isActiveAsymKey(keyid, *p.topic) && asymmetric) {

		rw, err := p.AddPeer(peer, *p.topic, asymmetric, keyid)
		if err != nil {
			return err
		} else if rw == nil {
			return fmt.Errorf("handle called on nil MsgReadWriter for new key " + keyid)
		}
		vrw = rw.(*PssReadWriter)
	}

	pmsg, err := ToP2pMsg(msg)
	if err != nil {
		return fmt.Errorf("could not decode pssmsg")
	}
	if asymmetric {
		if p.pubKeyRWPool[keyid] == nil {
			return fmt.Errorf("handle called on nil MsgReadWriter for key " + keyid)
		}
		vrw = p.pubKeyRWPool[keyid].(*PssReadWriter)
	} else {
		if p.symKeyRWPool[keyid] == nil {
			return fmt.Errorf("handle called on nil MsgReadWriter for key " + keyid)
		}
		vrw = p.symKeyRWPool[keyid].(*PssReadWriter)
	}
	vrw.injectMsg(pmsg)
	return nil
}

//检查（对等）对称密钥当前是否已注册到此主题
func (p *Protocol) isActiveSymKey(key string, topic Topic) bool {
	return p.symKeyRWPool[key] != nil
}

//检查（对等）非对称密钥当前是否已注册到此主题
func (p *Protocol) isActiveAsymKey(key string, topic Topic) bool {
	return p.pubKeyRWPool[key] != nil
}

//创建p2p.msg的序列化（非缓冲）版本，用于专门的内部p2p.msgreadwriter实现
func ToP2pMsg(msg []byte) (p2p.Msg, error) {
	payload := &ProtocolMsg{}
	if err := rlp.DecodeBytes(msg, payload); err != nil {
		return p2p.Msg{}, fmt.Errorf("pss protocol handler unable to decode payload as p2p message: %v", err)
	}

	return p2p.Msg{
		Code:       payload.Code,
		Size:       uint32(len(payload.Payload)),
		ReceivedAt: time.Now(),
		Payload:    bytes.NewBuffer(payload.Payload),
	}, nil
}

//在指定的对等机上运行模拟的PSS协议，
//链接到特定主题
//`key`和'asymmetric`指定什么加密密钥
//将对等机链接到。
//在添加对等机之前，密钥必须存在于PSS存储区中。
func (p *Protocol) AddPeer(peer *p2p.Peer, topic Topic, asymmetric bool, key string) (p2p.MsgReadWriter, error) {
	rw := &PssReadWriter{
		Pss:   p.Pss,
		rw:    make(chan p2p.Msg),
		spec:  p.spec,
		topic: p.topic,
		key:   key,
	}
	if asymmetric {
		rw.sendFunc = p.Pss.SendAsym
	} else {
		rw.sendFunc = p.Pss.SendSym
	}
	if asymmetric {
		p.Pss.pubKeyPoolMu.Lock()
		if _, ok := p.Pss.pubKeyPool[key]; !ok {
			return nil, fmt.Errorf("asym key does not exist: %s", key)
		}
		p.Pss.pubKeyPoolMu.Unlock()
		p.RWPoolMu.Lock()
		p.pubKeyRWPool[key] = rw
		p.RWPoolMu.Unlock()
	} else {
		p.Pss.symKeyPoolMu.Lock()
		if _, ok := p.Pss.symKeyPool[key]; !ok {
			return nil, fmt.Errorf("symkey does not exist: %s", key)
		}
		p.Pss.symKeyPoolMu.Unlock()
		p.RWPoolMu.Lock()
		p.symKeyRWPool[key] = rw
		p.RWPoolMu.Unlock()
	}
	go func() {
		err := p.proto.Run(peer, rw)
		log.Warn(fmt.Sprintf("pss vprotocol quit on %v topic %v: %v", peer, topic, err))
	}()
	return rw, nil
}

func (p *Protocol) RemovePeer(asymmetric bool, key string) {
	log.Debug("closing pss peer", "asym", asymmetric, "key", key)
	p.RWPoolMu.Lock()
	defer p.RWPoolMu.Unlock()
	if asymmetric {
		rw := p.pubKeyRWPool[key].(*PssReadWriter)
		rw.closed = true
		delete(p.pubKeyRWPool, key)
	} else {
		rw := p.symKeyRWPool[key].(*PssReadWriter)
		rw.closed = true
		delete(p.symKeyRWPool, key)
	}
}

//协议说明符到主题的统一翻译
func ProtocolTopic(spec *protocols.Spec) Topic {
	return BytesToTopic([]byte(fmt.Sprintf("%s:%d", spec.Name, spec.Version)))
}
