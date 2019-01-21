
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
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/swarm/storage"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv5"
)

const (
	defaultWhisperTTL = 6000
)

const (
	pssControlSym = 1
	pssControlRaw = 1 << 1
)

var (
	topicHashMutex = sync.Mutex{}
	topicHashFunc  = storage.MakeHashFunc("SHA256")()
	rawTopic       = Topic{}
)

//主题是耳语主题类型的PSS封装
type Topic whisper.TopicType

func (t *Topic) String() string {
	return hexutil.Encode(t[:])
}

//marshaljson实现json.marshaler接口
func (t Topic) MarshalJSON() (b []byte, err error) {
	return json.Marshal(t.String())
}

//marshaljson实现json.marshaler接口
func (t *Topic) UnmarshalJSON(input []byte) error {
	topicbytes, err := hexutil.Decode(string(input[1 : len(input)-1]))
	if err != nil {
		return err
	}
	copy(t[:], topicbytes)
	return nil
}

//pssadress是[]字节的别名。它表示可变长度的地址
type PssAddress []byte

//marshaljson实现json.marshaler接口
func (a PssAddress) MarshalJSON() ([]byte, error) {
	return json.Marshal(hexutil.Encode(a[:]))
}

//unmashaljson实现json.marshaler接口
func (a *PssAddress) UnmarshalJSON(input []byte) error {
	b, err := hexutil.Decode(string(input[1 : len(input)-1]))
	if err != nil {
		return err
	}
	for _, bb := range b {
		*a = append(*a, bb)
	}
	return nil
}

//保存用于缓存的消息摘要
type pssDigest [digestLength]byte

//隐藏对控件标志字节的按位操作
type msgParams struct {
	raw bool
	sym bool
}

func newMsgParamsFromBytes(paramBytes []byte) *msgParams {
	if len(paramBytes) != 1 {
		return nil
	}
	return &msgParams{
		raw: paramBytes[0]&pssControlRaw > 0,
		sym: paramBytes[0]&pssControlSym > 0,
	}
}

func (m *msgParams) Bytes() (paramBytes []byte) {
	var b byte
	if m.raw {
		b |= pssControlRaw
	}
	if m.sym {
		b |= pssControlSym
	}
	paramBytes = append(paramBytes, b)
	return paramBytes
}

//PSSMSG封装通过PSS传输的消息。
type PssMsg struct {
	To      []byte
	Control []byte
	Expire  uint32
	Payload *whisper.Envelope
}

func newPssMsg(param *msgParams) *PssMsg {
	return &PssMsg{
		Control: param.Bytes(),
	}
}

//邮件被标记为原始/外部加密
func (msg *PssMsg) isRaw() bool {
	return msg.Control[0]&pssControlRaw > 0
}

//消息被标记为对称加密
func (msg *PssMsg) isSym() bool {
	return msg.Control[0]&pssControlSym > 0
}

//序列化消息以在缓存中使用
func (msg *PssMsg) serialize() []byte {
	rlpdata, _ := rlp.EncodeToBytes(struct {
		To      []byte
		Payload *whisper.Envelope
	}{
		To:      msg.To,
		Payload: msg.Payload,
	})
	return rlpdata
}

//pssmsg的字符串表示法
func (msg *PssMsg) String() string {
	return fmt.Sprintf("PssMsg: Recipient: %x", common.ToHex(msg.To))
}

//pssmsg的消息处理程序函数的签名
//此类型的实现将传递给PSS。请与主题一起注册，
type HandlerFunc func(msg []byte, p *p2p.Peer, asymmetric bool, keyid string) error

type handlerCaps struct {
	raw  bool
	prox bool
}

//处理程序定义接收内容时要执行的代码。
type handler struct {
	f    HandlerFunc
	caps *handlerCaps
}

//NewHandler返回新的消息处理程序
func NewHandler(f HandlerFunc) *handler {
	return &handler{
		f:    f,
		caps: &handlerCaps{},
	}
}

//withraw是一种可链接的方法，允许处理原始消息。
func (h *handler) WithRaw() *handler {
	h.caps.raw = true
	return h
}

//WithProxbin是一种可链接的方法，它允许使用Kademlia深度作为参考向邻近地区发送具有完整地址的消息。
func (h *handler) WithProxBin() *handler {
	h.caps.prox = true
	return h
}

//StateStore处理保存和加载PSS对等机及其相应的密钥。
//目前尚未实施
type stateStore struct {
	values map[string][]byte
}

func (store *stateStore) Load(key string) ([]byte, error) {
	return nil, nil
}

func (store *stateStore) Save(key string, v []byte) error {
	return nil
}

//bytestoopic散列任意长度的字节片，并将其截断为主题的长度，只使用摘要的第一个字节。
func BytesToTopic(b []byte) Topic {
	topicHashMutex.Lock()
	defer topicHashMutex.Unlock()
	topicHashFunc.Reset()
	topicHashFunc.Write(b)
	return Topic(whisper.BytesToTopic(topicHashFunc.Sum(nil)))
}
