
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

package pss

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/log"
)

//使用PSS API时用于接收PSS消息的包装器
//提供对消息发送者的访问
type APIMsg struct {
	Msg        hexutil.Bytes
	Asymmetric bool
	Key        string
}

//通过PSS的API可访问的其他公共方法
type API struct {
	*Pss
}

func NewAPI(ps *Pss) *API {
	return &API{Pss: ps}
}

//为调用者创建新的订阅。启用对传入消息的外部处理。
//
//在PSS中为提供的主题注册了一个新的处理程序
//
//与此主题匹配的节点的所有传入消息都将封装在apimsg中
//结构并发送到订阅服务器
func (pssapi *API) Receive(ctx context.Context, topic Topic, raw bool, prox bool) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return nil, fmt.Errorf("Subscribe not supported")
	}

	psssub := notifier.CreateSubscription()

	hndlr := NewHandler(func(msg []byte, p *p2p.Peer, asymmetric bool, keyid string) error {
		apimsg := &APIMsg{
			Msg:        hexutil.Bytes(msg),
			Asymmetric: asymmetric,
			Key:        keyid,
		}
		if err := notifier.Notify(psssub.ID, apimsg); err != nil {
			log.Warn(fmt.Sprintf("notification on pss sub topic rpc (sub %v) msg %v failed!", psssub.ID, msg))
		}
		return nil
	})
	if raw {
		hndlr.caps.raw = true
	}
	if prox {
		hndlr.caps.prox = true
	}

	deregf := pssapi.Register(&topic, hndlr)
	go func() {
		defer deregf()
		select {
		case err := <-psssub.Err():
			log.Warn(fmt.Sprintf("caught subscription error in pss sub topic %x: %v", topic, err))
		case <-notifier.Closed():
			log.Warn(fmt.Sprintf("rpc sub notifier closed"))
		}
	}()

	return psssub, nil
}

func (pssapi *API) GetAddress(topic Topic, asymmetric bool, key string) (PssAddress, error) {
	var addr PssAddress
	if asymmetric {
		peer, ok := pssapi.Pss.pubKeyPool[key][topic]
		if !ok {
			return nil, fmt.Errorf("pubkey/topic pair %x/%x doesn't exist", key, topic)
		}
		addr = peer.address
	} else {
		peer, ok := pssapi.Pss.symKeyPool[key][topic]
		if !ok {
			return nil, fmt.Errorf("symkey/topic pair %x/%x doesn't exist", key, topic)
		}
		addr = peer.address

	}
	return addr, nil
}

//以十六进制形式检索节点的基地址
func (pssapi *API) BaseAddr() (PssAddress, error) {
	return PssAddress(pssapi.Pss.BaseAddr()), nil
}

//以十六进制形式检索节点的公钥
func (pssapi *API) GetPublicKey() (keybytes hexutil.Bytes) {
	key := pssapi.Pss.PublicKey()
	keybytes = crypto.FromECDSAPub(key)
	return keybytes
}

//将公钥设置为与特定PSS对等机关联
func (pssapi *API) SetPeerPublicKey(pubkey hexutil.Bytes, topic Topic, addr PssAddress) error {
	pk, err := crypto.UnmarshalPubkey(pubkey)
	if err != nil {
		return fmt.Errorf("Cannot unmarshal pubkey: %x", pubkey)
	}
	err = pssapi.Pss.SetPeerPublicKey(pk, topic, addr)
	if err != nil {
		return fmt.Errorf("Invalid key: %x", pk)
	}
	return nil
}

func (pssapi *API) GetSymmetricKey(symkeyid string) (hexutil.Bytes, error) {
	symkey, err := pssapi.Pss.GetSymmetricKey(symkeyid)
	return hexutil.Bytes(symkey), err
}

func (pssapi *API) GetSymmetricAddressHint(topic Topic, symkeyid string) (PssAddress, error) {
	return pssapi.Pss.symKeyPool[symkeyid][topic].address, nil
}

func (pssapi *API) GetAsymmetricAddressHint(topic Topic, pubkeyid string) (PssAddress, error) {
	return pssapi.Pss.pubKeyPool[pubkeyid][topic].address, nil
}

func (pssapi *API) StringToTopic(topicstring string) (Topic, error) {
	topicbytes := BytesToTopic([]byte(topicstring))
	if topicbytes == rawTopic {
		return rawTopic, errors.New("Topic string hashes to 0x00000000 and cannot be used")
	}
	return topicbytes, nil
}

func (pssapi *API) SendAsym(pubkeyhex string, topic Topic, msg hexutil.Bytes) error {
	if err := validateMsg(msg); err != nil {
		return err
	}
	return pssapi.Pss.SendAsym(pubkeyhex, topic, msg[:])
}

func (pssapi *API) SendSym(symkeyhex string, topic Topic, msg hexutil.Bytes) error {
	if err := validateMsg(msg); err != nil {
		return err
	}
	return pssapi.Pss.SendSym(symkeyhex, topic, msg[:])
}

func (pssapi *API) SendRaw(addr hexutil.Bytes, topic Topic, msg hexutil.Bytes) error {
	if err := validateMsg(msg); err != nil {
		return err
	}
	return pssapi.Pss.SendRaw(PssAddress(addr), topic, msg[:])
}

func (pssapi *API) GetPeerTopics(pubkeyhex string) ([]Topic, error) {
	topics, _, err := pssapi.Pss.GetPublickeyPeers(pubkeyhex)
	return topics, err

}

func (pssapi *API) GetPeerAddress(pubkeyhex string, topic Topic) (PssAddress, error) {
	return pssapi.Pss.getPeerAddress(pubkeyhex, topic)
}

func validateMsg(msg []byte) error {
	if len(msg) == 0 {
		return errors.New("invalid message length")
	}
	return nil
}
