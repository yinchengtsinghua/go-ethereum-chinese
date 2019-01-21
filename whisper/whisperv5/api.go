
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

package whisperv5

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	ErrSymAsym              = errors.New("specify either a symmetric or an asymmetric key")
	ErrInvalidSymmetricKey  = errors.New("invalid symmetric key")
	ErrInvalidPublicKey     = errors.New("invalid public key")
	ErrInvalidSigningPubKey = errors.New("invalid signing public key")
	ErrTooLowPoW            = errors.New("message rejected, PoW too low")
	ErrNoTopics             = errors.New("missing topic(s)")
)

//publicWhisperAPI提供可以
//公开使用，不涉及安全问题。
type PublicWhisperAPI struct {
	w *Whisper

	mu       sync.Mutex
lastUsed map[string]time.Time //跟踪上次轮询筛选器的时间。
}

//NewPublicWhisperAPI创建新的RPC Whisper服务。
func NewPublicWhisperAPI(w *Whisper) *PublicWhisperAPI {
	api := &PublicWhisperAPI{
		w:        w,
		lastUsed: make(map[string]time.Time),
	}
	return api
}

//version返回耳语子协议版本。
func (api *PublicWhisperAPI) Version(ctx context.Context) string {
	return ProtocolVersionStr
}

//信息包含诊断信息。
type Info struct {
Memory         int     `json:"memory"`         //浮动消息的内存大小（字节）。
Messages       int     `json:"messages"`       //浮动消息数。
MinPow         float64 `json:"minPow"`         //最小可接受功率
MaxMessageSize uint32  `json:"maxMessageSize"` //最大接受邮件大小
}

//信息返回关于耳语节点的诊断信息。
func (api *PublicWhisperAPI) Info(ctx context.Context) Info {
	stats := api.w.Stats()
	return Info{
		Memory:         stats.memoryUsed,
		Messages:       len(api.w.messageQueue) + len(api.w.p2pMsgQueue),
		MinPow:         api.w.MinPow(),
		MaxMessageSize: api.w.MaxMessageSize(),
	}
}

//setmaxmessagesize设置可接受的最大消息大小。
//上限在Whisperv5.MaxMessageSize中定义。
func (api *PublicWhisperAPI) SetMaxMessageSize(ctx context.Context, size uint32) (bool, error) {
	return true, api.w.SetMaxMessageSize(size)
}

//setminpow设置消息在被接受之前的最小POW。
func (api *PublicWhisperAPI) SetMinPoW(ctx context.Context, pow float64) (bool, error) {
	return true, api.w.SetMinimumPoW(pow)
}

//marktrustedpeer标记受信任的对等机。，这将允许它发送历史（过期）消息。
//注意：此功能不添加新节点，节点需要作为对等节点存在。
func (api *PublicWhisperAPI) MarkTrustedPeer(ctx context.Context, url string) (bool, error) {
	n, err := enode.ParseV4(url)
	if err != nil {
		return false, err
	}
	return true, api.w.AllowP2PMessagesFromPeer(n.ID().Bytes())
}

//new key pair为消息解密和加密生成一个新的公钥和私钥对。
//它返回一个可用于引用密钥对的ID。
func (api *PublicWhisperAPI) NewKeyPair(ctx context.Context) (string, error) {
	return api.w.NewKeyPair()
}

//addprivatekey导入给定的私钥。
func (api *PublicWhisperAPI) AddPrivateKey(ctx context.Context, privateKey hexutil.Bytes) (string, error) {
	key, err := crypto.ToECDSA(privateKey)
	if err != nil {
		return "", err
	}
	return api.w.AddKeyPair(key)
}

//DeleteKeyPair删除具有给定密钥的密钥（如果存在）。
func (api *PublicWhisperAPI) DeleteKeyPair(ctx context.Context, key string) (bool, error) {
	if ok := api.w.DeleteKeyPair(key); ok {
		return true, nil
	}
	return false, fmt.Errorf("key pair %s not found", key)
}

//hasKeyPair返回节点是否有与给定ID关联的密钥对的指示。
func (api *PublicWhisperAPI) HasKeyPair(ctx context.Context, id string) bool {
	return api.w.HasKeyPair(id)
}

//GetPublicKey返回与给定键关联的公钥。关键是十六进制
//以ANSI X9.62第4.3.6节规定的格式对键进行编码表示。
func (api *PublicWhisperAPI) GetPublicKey(ctx context.Context, id string) (hexutil.Bytes, error) {
	key, err := api.w.GetPrivateKey(id)
	if err != nil {
		return hexutil.Bytes{}, err
	}
	return crypto.FromECDSAPub(&key.PublicKey), nil
}

//getprivatekey返回与给定密钥关联的私钥。关键是十六进制
//以ANSI X9.62第4.3.6节规定的格式对键进行编码表示。
func (api *PublicWhisperAPI) GetPrivateKey(ctx context.Context, id string) (hexutil.Bytes, error) {
	key, err := api.w.GetPrivateKey(id)
	if err != nil {
		return hexutil.Bytes{}, err
	}
	return crypto.FromECDSA(key), nil
}

//newsymkey生成随机对称密钥。
//它返回一个可用于引用该键的ID。
//可用于加密和解密双方都知道密钥的消息。
func (api *PublicWhisperAPI) NewSymKey(ctx context.Context) (string, error) {
	return api.w.GenerateSymKey()
}

//addsymkey导入对称密钥。
//它返回一个可用于引用该键的ID。
//可用于加密和解密双方都知道密钥的消息。
func (api *PublicWhisperAPI) AddSymKey(ctx context.Context, key hexutil.Bytes) (string, error) {
	return api.w.AddSymKeyDirect([]byte(key))
}

//generatesymkeyfrompassword从给定的密码派生一个密钥，存储并返回其ID。
func (api *PublicWhisperAPI) GenerateSymKeyFromPassword(ctx context.Context, passwd string) (string, error) {
	return api.w.AddSymKeyFromPassword(passwd)
}

//hassymkey返回节点是否具有与给定密钥关联的对称密钥的指示。
func (api *PublicWhisperAPI) HasSymKey(ctx context.Context, id string) bool {
	return api.w.HasSymKey(id)
}

//GetSymkey返回与给定ID关联的对称密钥。
func (api *PublicWhisperAPI) GetSymKey(ctx context.Context, id string) (hexutil.Bytes, error) {
	return api.w.GetSymKey(id)
}

//DeleteSymkey删除与给定ID关联的对称密钥。
func (api *PublicWhisperAPI) DeleteSymKey(ctx context.Context, id string) bool {
	return api.w.DeleteSymKey(id)
}

//go:生成gencodec-键入newmessage-field override newmessage override-out gen_newmessage_json.go

//new message表示通过rpc发布的新低语消息。
type NewMessage struct {
	SymKeyID   string    `json:"symKeyID"`
	PublicKey  []byte    `json:"pubKey"`
	Sig        string    `json:"sig"`
	TTL        uint32    `json:"ttl"`
	Topic      TopicType `json:"topic"`
	Payload    []byte    `json:"payload"`
	Padding    []byte    `json:"padding"`
	PowTime    uint32    `json:"powTime"`
	PowTarget  float64   `json:"powTarget"`
	TargetPeer string    `json:"targetPeer"`
}

type newMessageOverride struct {
	PublicKey hexutil.Bytes
	Payload   hexutil.Bytes
	Padding   hexutil.Bytes
}

//在低语网络上发布消息。
func (api *PublicWhisperAPI) Post(ctx context.Context, req NewMessage) (bool, error) {
	var (
		symKeyGiven = len(req.SymKeyID) > 0
		pubKeyGiven = len(req.PublicKey) > 0
		err         error
	)

//用户必须指定对称密钥或非对称密钥
	if (symKeyGiven && pubKeyGiven) || (!symKeyGiven && !pubKeyGiven) {
		return false, ErrSymAsym
	}

	params := &MessageParams{
		TTL:      req.TTL,
		Payload:  req.Payload,
		Padding:  req.Padding,
		WorkTime: req.PowTime,
		PoW:      req.PowTarget,
		Topic:    req.Topic,
	}

//设置用于对消息签名的键
	if len(req.Sig) > 0 {
		if params.Src, err = api.w.GetPrivateKey(req.Sig); err != nil {
			return false, err
		}
	}

//设置用于加密消息的对称密钥
	if symKeyGiven {
if params.Topic == (TopicType{}) { //主题对于对称加密是必需的
			return false, ErrNoTopics
		}
		if params.KeySym, err = api.w.GetSymKey(req.SymKeyID); err != nil {
			return false, err
		}
		if !validateSymmetricKey(params.KeySym) {
			return false, ErrInvalidSymmetricKey
		}
	}

//设置用于加密消息的非对称密钥
	if pubKeyGiven {
		if params.Dst, err = crypto.UnmarshalPubkey(req.PublicKey); err != nil {
			return false, ErrInvalidPublicKey
		}
	}

//加密并发送消息
	whisperMsg, err := NewSentMessage(params)
	if err != nil {
		return false, err
	}

	env, err := whisperMsg.Wrap(params)
	if err != nil {
		return false, err
	}

//发送到特定节点（跳过电源检查）
	if len(req.TargetPeer) > 0 {
		n, err := enode.ParseV4(req.TargetPeer)
		if err != nil {
			return false, fmt.Errorf("failed to parse target peer: %s", err)
		}
		return true, api.w.SendP2PMessage(n.ID().Bytes(), env)
	}

//确保消息POW满足节点的最小可接受POW
	if req.PowTarget < api.w.MinPow() {
		return false, ErrTooLowPoW
	}

	return true, api.w.Send(env)
}

//go：生成gencodec-类型标准-字段覆盖标准覆盖-out gen_标准\json.go

//条件保存入站消息的各种筛选选项。
type Criteria struct {
	SymKeyID     string      `json:"symKeyID"`
	PrivateKeyID string      `json:"privateKeyID"`
	Sig          []byte      `json:"sig"`
	MinPow       float64     `json:"minPow"`
	Topics       []TopicType `json:"topics"`
	AllowP2P     bool        `json:"allowP2P"`
}

type criteriaOverride struct {
	Sig hexutil.Bytes
}

//消息设置了一个订阅，该订阅在消息到达时触发匹配的事件
//给定的一组标准。
func (api *PublicWhisperAPI) Messages(ctx context.Context, crit Criteria) (*rpc.Subscription, error) {
	var (
		symKeyGiven = len(crit.SymKeyID) > 0
		pubKeyGiven = len(crit.PrivateKeyID) > 0
		err         error
	)

//确保RPC连接支持订阅
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return nil, rpc.ErrNotificationsUnsupported
	}

//用户必须指定对称密钥或非对称密钥
	if (symKeyGiven && pubKeyGiven) || (!symKeyGiven && !pubKeyGiven) {
		return nil, ErrSymAsym
	}

	filter := Filter{
		PoW:      crit.MinPow,
		Messages: make(map[common.Hash]*ReceivedMessage),
		AllowP2P: crit.AllowP2P,
	}

	if len(crit.Sig) > 0 {
		if filter.Src, err = crypto.UnmarshalPubkey(crit.Sig); err != nil {
			return nil, ErrInvalidSigningPubKey
		}
	}

	for i, bt := range crit.Topics {
		if len(bt) == 0 || len(bt) > 4 {
			return nil, fmt.Errorf("subscribe: topic %d has wrong size: %d", i, len(bt))
		}
		filter.Topics = append(filter.Topics, bt[:])
	}

//侦听使用给定对称密钥加密的消息
	if symKeyGiven {
		if len(filter.Topics) == 0 {
			return nil, ErrNoTopics
		}
		key, err := api.w.GetSymKey(crit.SymKeyID)
		if err != nil {
			return nil, err
		}
		if !validateSymmetricKey(key) {
			return nil, ErrInvalidSymmetricKey
		}
		filter.KeySym = key
		filter.SymKeyHash = crypto.Keccak256Hash(filter.KeySym)
	}

//侦听使用给定公钥加密的消息
	if pubKeyGiven {
		filter.KeyAsym, err = api.w.GetPrivateKey(crit.PrivateKeyID)
		if err != nil || filter.KeyAsym == nil {
			return nil, ErrInvalidPublicKey
		}
	}

	id, err := api.w.Subscribe(&filter)
	if err != nil {
		return nil, err
	}

//创建订阅并开始等待消息事件
	rpcSub := notifier.CreateSubscription()
	go func() {
//现在在内部进行投票，重构内部以获得通道支持
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if filter := api.w.GetFilter(id); filter != nil {
					for _, rpcMessage := range toMessage(filter.Retrieve()) {
						if err := notifier.Notify(rpcSub.ID, rpcMessage); err != nil {
							log.Error("Failed to send notification", "err", err)
						}
					}
				}
			case <-rpcSub.Err():
				api.w.Unsubscribe(id)
				return
			case <-notifier.Closed():
				api.w.Unsubscribe(id)
				return
			}
		}
	}()

	return rpcSub, nil
}

//go:生成gencodec-类型message-字段覆盖message override-out gen_message_json.go

//message是whisper消息的RPC表示。
type Message struct {
	Sig       []byte    `json:"sig,omitempty"`
	TTL       uint32    `json:"ttl"`
	Timestamp uint32    `json:"timestamp"`
	Topic     TopicType `json:"topic"`
	Payload   []byte    `json:"payload"`
	Padding   []byte    `json:"padding"`
	PoW       float64   `json:"pow"`
	Hash      []byte    `json:"hash"`
	Dst       []byte    `json:"recipientPublicKey,omitempty"`
}

type messageOverride struct {
	Sig     hexutil.Bytes
	Payload hexutil.Bytes
	Padding hexutil.Bytes
	Hash    hexutil.Bytes
	Dst     hexutil.Bytes
}

//TowHisPermessage将内部消息转换为API版本。
func ToWhisperMessage(message *ReceivedMessage) *Message {
	msg := Message{
		Payload:   message.Payload,
		Padding:   message.Padding,
		Timestamp: message.Sent,
		TTL:       message.TTL,
		PoW:       message.PoW,
		Hash:      message.EnvelopeHash.Bytes(),
		Topic:     message.Topic,
	}

	if message.Dst != nil {
		b := crypto.FromECDSAPub(message.Dst)
		if b != nil {
			msg.Dst = b
		}
	}

	if isMessageSigned(message.Raw[0]) {
		b := crypto.FromECDSAPub(message.SigToPubKey())
		if b != nil {
			msg.Sig = b
		}
	}

	return &msg
}

//ToMessage将一组消息转换为其RPC表示形式。
func toMessage(messages []*ReceivedMessage) []*Message {
	msgs := make([]*Message, len(messages))
	for i, msg := range messages {
		msgs[i] = ToWhisperMessage(msg)
	}
	return msgs
}

//getfiltermessages返回符合筛选条件和
//从上一次投票到现在。
func (api *PublicWhisperAPI) GetFilterMessages(id string) ([]*Message, error) {
	api.mu.Lock()
	f := api.w.GetFilter(id)
	if f == nil {
		api.mu.Unlock()
		return nil, fmt.Errorf("filter not found")
	}
	api.lastUsed[id] = time.Now()
	api.mu.Unlock()

	receivedMessages := f.Retrieve()
	messages := make([]*Message, 0, len(receivedMessages))
	for _, msg := range receivedMessages {
		messages = append(messages, ToWhisperMessage(msg))
	}

	return messages, nil
}

//DeleteMessageFilter删除一个筛选器。
func (api *PublicWhisperAPI) DeleteMessageFilter(id string) (bool, error) {
	api.mu.Lock()
	defer api.mu.Unlock()

	delete(api.lastUsed, id)
	return true, api.w.Unsubscribe(id)
}

//NewMessageFilter创建一个可用于轮询的新筛选器
//（新）满足给定条件的消息。
func (api *PublicWhisperAPI) NewMessageFilter(req Criteria) (string, error) {
	var (
		src     *ecdsa.PublicKey
		keySym  []byte
		keyAsym *ecdsa.PrivateKey
		topics  [][]byte

		symKeyGiven  = len(req.SymKeyID) > 0
		asymKeyGiven = len(req.PrivateKeyID) > 0

		err error
	)

//用户必须指定对称密钥或非对称密钥
	if (symKeyGiven && asymKeyGiven) || (!symKeyGiven && !asymKeyGiven) {
		return "", ErrSymAsym
	}

	if len(req.Sig) > 0 {
		if src, err = crypto.UnmarshalPubkey(req.Sig); err != nil {
			return "", ErrInvalidSigningPubKey
		}
	}

	if symKeyGiven {
		if keySym, err = api.w.GetSymKey(req.SymKeyID); err != nil {
			return "", err
		}
		if !validateSymmetricKey(keySym) {
			return "", ErrInvalidSymmetricKey
		}
	}

	if asymKeyGiven {
		if keyAsym, err = api.w.GetPrivateKey(req.PrivateKeyID); err != nil {
			return "", err
		}
	}

	if len(req.Topics) > 0 {
		topics = make([][]byte, 0, len(req.Topics))
		for _, topic := range req.Topics {
			topics = append(topics, topic[:])
		}
	}

	f := &Filter{
		Src:      src,
		KeySym:   keySym,
		KeyAsym:  keyAsym,
		PoW:      req.MinPow,
		AllowP2P: req.AllowP2P,
		Topics:   topics,
		Messages: make(map[common.Hash]*ReceivedMessage),
	}

	id, err := api.w.Subscribe(f)
	if err != nil {
		return "", err
	}

	api.mu.Lock()
	api.lastUsed[id] = time.Now()
	api.mu.Unlock()

	return id, nil
}
