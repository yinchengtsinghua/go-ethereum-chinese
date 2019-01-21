
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package notify

import (
	"crypto/ecdsa"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/pss"
)

const (
//从请求者发送到更新者以请求开始通知
	MsgCodeStart = iota

//从更新程序发送到请求程序，包含一个通知和一个新的符号键来替换旧的
	MsgCodeNotifyWithKey

//从更新程序发送到请求程序，包含一个通知
	MsgCodeNotify

//从请求程序发送到更新程序以请求停止通知（当前未使用）
	MsgCodeStop
	MsgCodeMax
)

const (
	DefaultAddressLength = 1
symKeyLength         = 32 //这应该从源头上得到
)

var (
//在对称密钥颁发完成之前使用控制主题
	controlTopic = pss.Topic{0x00, 0x00, 0x00, 0x01}
)

//当代码为msgcodestart时，有效负载为地址
//当代码为msgcodenotifywithkey时，有效负载为notification symkey
//当代码为msgcodenotify时，有效负载为notification
//当代码为msgcodestop时，有效负载为地址
type Msg struct {
	Code       byte
	Name       []byte
	Payload    []byte
	namestring string
}

//newmsg创建新的通知消息对象
func NewMsg(code byte, name string, payload []byte) *Msg {
	return &Msg{
		Code:       code,
		Name:       []byte(name),
		Payload:    payload,
		namestring: name,
	}
}

//newmsgFromPayload将序列化消息负载解码为新的通知消息对象
func NewMsgFromPayload(payload []byte) (*Msg, error) {
	msg := &Msg{}
	err := rlp.DecodeBytes(payload, msg)
	if err != nil {
		return nil, err
	}
	msg.namestring = string(msg.Name)
	return msg, nil
}

//通知程序在向其发送消息的每个地址空间中都有一个sendbin条目。
type sendBin struct {
	address  pss.PssAddress
	symKeyId string
	count    int
}

//表示单个通知服务
//只有与通知客户端地址匹配的订阅地址箱才有条目。
type notifier struct {
	bins      map[string]*sendBin
topic     pss.Topic //标识PSS接收器的资源
threshold int       //箱中使用的地址字节数
	updateC   <-chan []byte
	quitC     chan struct{}
}

func (n *notifier) removeSubscription() {
	n.quitC <- struct{}{}
}

//表示公钥在特定地址/邻居处进行的单个订阅
type subscription struct {
	pubkeyId string
	address  pss.PssAddress
	handler  func(string, []byte) error
}

//控制器是控制、添加和删除通知服务和订阅的接口。
type Controller struct {
	pss           *pss.Pss
	notifiers     map[string]*notifier
	subscriptions map[string]*subscription
	mu            sync.Mutex
}

//NewController创建新的控制器对象
func NewController(ps *pss.Pss) *Controller {
	ctrl := &Controller{
		pss:           ps,
		notifiers:     make(map[string]*notifier),
		subscriptions: make(map[string]*subscription),
	}
	ctrl.pss.Register(&controlTopic, pss.NewHandler(ctrl.Handler))
	return ctrl
}

//IsActive用于检查是否存在指定ID字符串的通知服务
//如果存在则返回true，否则返回false
func (c *Controller) IsActive(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.isActive(name)
}

func (c *Controller) isActive(name string) bool {
	_, ok := c.notifiers[name]
	return ok
}

//客户端使用订阅从通知服务提供程序请求通知
//它将创建一个msgcodestart消息，并使用其公钥和路由地址不对称地发送给提供程序。
//handler函数是一个回调函数，当收到通知时将调用该回调函数。
//如果无法发送请求PSS或无法序列化更新消息，则失败
func (c *Controller) Subscribe(name string, pubkey *ecdsa.PublicKey, address pss.PssAddress, handler func(string, []byte) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	msg := NewMsg(MsgCodeStart, name, c.pss.BaseAddr())
	c.pss.SetPeerPublicKey(pubkey, controlTopic, address)
	pubkeyId := hexutil.Encode(crypto.FromECDSAPub(pubkey))
	smsg, err := rlp.EncodeToBytes(msg)
	if err != nil {
		return err
	}
	err = c.pss.SendAsym(pubkeyId, controlTopic, smsg)
	if err != nil {
		return err
	}
	c.subscriptions[name] = &subscription{
		pubkeyId: pubkeyId,
		address:  address,
		handler:  handler,
	}
	return nil
}

//取消订阅，也许不足为奇，会取消订阅的效果。
//如果订阅不存在、无法发送请求PSS或无法序列化更新消息，则失败
func (c *Controller) Unsubscribe(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	sub, ok := c.subscriptions[name]
	if !ok {
		return fmt.Errorf("Unknown subscription '%s'", name)
	}
	msg := NewMsg(MsgCodeStop, name, sub.address)
	smsg, err := rlp.EncodeToBytes(msg)
	if err != nil {
		return err
	}
	err = c.pss.SendAsym(sub.pubkeyId, controlTopic, smsg)
	if err != nil {
		return err
	}
	delete(c.subscriptions, name)
	return nil
}

//通知服务提供程序使用NewNotifier创建新的通知服务
//它将名称作为资源的标识符，一个阈值指示订阅地址bin的粒度。
//然后，它启动一个事件循环，该循环监听所提供的更新通道，并在通道接收时执行通知。
//如果通知程序已在名称上注册，则失败
//func（c*controller）newnotifier（name string，threshold int，contentfunc func（string）（[]byte，error））错误
func (c *Controller) NewNotifier(name string, threshold int, updateC <-chan []byte) (func(), error) {
	c.mu.Lock()
	if c.isActive(name) {
		c.mu.Unlock()
		return nil, fmt.Errorf("Notification service %s already exists in controller", name)
	}
	quitC := make(chan struct{})
	c.notifiers[name] = &notifier{
		bins:      make(map[string]*sendBin),
		topic:     pss.BytesToTopic([]byte(name)),
		threshold: threshold,
		updateC:   updateC,
		quitC:     quitC,
//contentfunc：contentfunc，
	}
	c.mu.Unlock()
	go func() {
		for {
			select {
			case <-quitC:
				return
			case data := <-updateC:
				c.notify(name, data)
			}
		}
	}()

	return c.notifiers[name].removeSubscription, nil
}

//removenotifier用于停止通知服务。
//它取消了侦听通知提供程序的更新通道的事件循环
func (c *Controller) RemoveNotifier(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	currentNotifier, ok := c.notifiers[name]
	if !ok {
		return fmt.Errorf("Unknown notification service %s", name)
	}
	currentNotifier.removeSubscription()
	delete(c.notifiers, name)
	return nil
}

//通知由通知服务提供程序调用以发出新通知
//它使用通知服务的名称和要发送的数据。
//如果不存在具有此名称的通知程序或无法序列化数据，则失败。
//请注意，发送消息失败时不会失败。
func (c *Controller) notify(name string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.isActive(name) {
		return fmt.Errorf("Notification service %s doesn't exist", name)
	}
	msg := NewMsg(MsgCodeNotify, name, data)
	smsg, err := rlp.EncodeToBytes(msg)
	if err != nil {
		return err
	}
	for _, m := range c.notifiers[name].bins {
		log.Debug("sending pss notify", "name", name, "addr", fmt.Sprintf("%x", m.address), "topic", fmt.Sprintf("%x", c.notifiers[name].topic), "data", data)
		go func(m *sendBin) {
			err = c.pss.SendSym(m.symKeyId, c.notifiers[name].topic, smsg)
			if err != nil {
				log.Warn("Failed to send notify to addr %x: %v", m.address, err)
			}
		}(m)
	}
	return nil
}

//检查一下我们是否已经有箱子了
//如果这样做，从中检索symkey并增加计数
//如果我们不做一个新的symkey和一个新的bin条目
func (c *Controller) addToBin(ntfr *notifier, address []byte) (symKeyId string, pssAddress pss.PssAddress, err error) {

//解析消息中的地址，如果长度超过bin阈值，则截断。
	if len(address) > ntfr.threshold {
		address = address[:ntfr.threshold]
	}

	pssAddress = pss.PssAddress(address)
	hexAddress := fmt.Sprintf("%x", address)
	currentBin, ok := ntfr.bins[hexAddress]
	if ok {
		currentBin.count++
		symKeyId = currentBin.symKeyId
	} else {
		symKeyId, err = c.pss.GenerateSymmetricKey(ntfr.topic, pssAddress, false)
		if err != nil {
			return "", nil, err
		}
		ntfr.bins[hexAddress] = &sendBin{
			address:  address,
			symKeyId: symKeyId,
			count:    1,
		}
	}
	return symKeyId, pssAddress, nil
}

func (c *Controller) handleStartMsg(msg *Msg, keyid string) (err error) {

	keyidbytes, err := hexutil.Decode(keyid)
	if err != nil {
		return err
	}
	pubkey, err := crypto.UnmarshalPubkey(keyidbytes)
	if err != nil {
		return err
	}

//如果没有为通知注册名称，我们将不会响应
	currentNotifier, ok := c.notifiers[msg.namestring]
	if !ok {
		return fmt.Errorf("Subscribe attempted on unknown resource '%s'", msg.namestring)
	}

//添加或打开新肥料箱
	symKeyId, pssAddress, err := c.addToBin(currentNotifier, msg.Payload)
	if err != nil {
		return err
	}

//添加到通讯簿以发送初始通知
	symkey, err := c.pss.GetSymmetricKey(symKeyId)
	if err != nil {
		return err
	}
	err = c.pss.SetPeerPublicKey(pubkey, controlTopic, pssAddress)
	if err != nil {
		return err
	}

//TODO这被设置为零长度字节，等待初始消息的协议决定，是否应该包含消息，以及如何触发初始消息，以便在订阅时发送Swarm Feed的当前状态。
	notify := []byte{}
	replyMsg := NewMsg(MsgCodeNotifyWithKey, msg.namestring, make([]byte, len(notify)+symKeyLength))
	copy(replyMsg.Payload, notify)
	copy(replyMsg.Payload[len(notify):], symkey)
	sReplyMsg, err := rlp.EncodeToBytes(replyMsg)
	if err != nil {
		return err
	}
	return c.pss.SendAsym(keyid, controlTopic, sReplyMsg)
}

func (c *Controller) handleNotifyWithKeyMsg(msg *Msg) error {
	symkey := msg.Payload[len(msg.Payload)-symKeyLength:]
	topic := pss.BytesToTopic(msg.Name)

//\t要跟踪并添加实际地址
	updaterAddr := pss.PssAddress([]byte{})
	c.pss.SetSymmetricKey(symkey, topic, updaterAddr, true)
	c.pss.Register(&topic, pss.NewHandler(c.Handler))
	return c.subscriptions[msg.namestring].handler(msg.namestring, msg.Payload[:len(msg.Payload)-symKeyLength])
}

func (c *Controller) handleStopMsg(msg *Msg) error {
//如果没有为通知注册名称，我们将不会响应
	currentNotifier, ok := c.notifiers[msg.namestring]
	if !ok {
		return fmt.Errorf("Unsubscribe attempted on unknown resource '%s'", msg.namestring)
	}

//解析消息中的地址，如果长度超过了bin的地址长度阈值，则截断。
	address := msg.Payload
	if len(msg.Payload) > currentNotifier.threshold {
		address = address[:currentNotifier.threshold]
	}

//从箱子中取出条目（如果存在），如果是最后一个剩余的，则取出箱子。
	hexAddress := fmt.Sprintf("%x", address)
	currentBin, ok := currentNotifier.bins[hexAddress]
	if !ok {
		return fmt.Errorf("found no active bin for address %s", hexAddress)
	}
	currentBin.count--
if currentBin.count == 0 { //如果此bin中没有更多客户端，请将其删除
		delete(currentNotifier.bins, hexAddress)
	}
	return nil
}

//handler是用于处理通知服务消息的PSS主题处理程序
//它应该在PSS中注册到任何提供的通知服务和使用该服务的客户机。
func (c *Controller) Handler(smsg []byte, p *p2p.Peer, asymmetric bool, keyid string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	log.Debug("notify controller handler", "keyid", keyid)

//查看消息是否有效
	msg, err := NewMsgFromPayload(smsg)
	if err != nil {
		return err
	}

	switch msg.Code {
	case MsgCodeStart:
		return c.handleStartMsg(msg, keyid)
	case MsgCodeNotifyWithKey:
		return c.handleNotifyWithKeyMsg(msg)
	case MsgCodeNotify:
		return c.subscriptions[msg.namestring].handler(msg.namestring, msg.Payload)
	case MsgCodeStop:
		return c.handleStopMsg(msg)
	}

	return fmt.Errorf("Invalid message code: %d", msg.Code)
}
