
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
	"bytes"
	"crypto/ecdsa"
	crand "crypto/rand"
	"crypto/sha256"
	"fmt"
	"runtime"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/sync/syncmap"
)

type Statistics struct {
	messagesCleared      int
	memoryCleared        int
	memoryUsed           int
	cycles               int
	totalMessagesCleared int
}

const (
minPowIdx     = iota //耳语节点所需的最小功率
maxMsgSizeIdx = iota //Whisper节点允许的最大消息长度
overflowIdx   = iota //消息队列溢出指示器
)

//低语代表通过以太坊的黑暗通信接口
//网络，使用自己的P2P通信层。
type Whisper struct {
protocol p2p.Protocol //协议描述和参数
filters  *Filters     //使用订阅功能安装的消息筛选器

privateKeys map[string]*ecdsa.PrivateKey //私钥存储
symKeys     map[string][]byte            //对称密钥存储
keyMu       sync.RWMutex                 //与密钥存储关联的互斥体

poolMu      sync.RWMutex              //用于同步消息和过期池的互斥体
envelopes   map[common.Hash]*Envelope //此节点当前跟踪的信封池
expirations map[uint32]mapset.Set     //邮件过期池

peerMu sync.RWMutex       //用于同步活动对等集的互斥体
peers  map[*Peer]struct{} //当前活动对等点集

messageQueue chan *Envelope //正常低语消息的消息队列
p2pMsgQueue  chan *Envelope //对等消息的消息队列（不再转发）
quit         chan struct{}  //用于优美出口的通道

settings syncmap.Map //保留可动态更改的配置设置

statsMu sync.Mutex //警卫统计
stats   Statistics //耳语节点统计

mailServer MailServer //邮件服务器接口
}

//New创建了一个准备好通过以太坊P2P网络通信的耳语客户端。
func New(cfg *Config) *Whisper {
	if cfg == nil {
		cfg = &DefaultConfig
	}

	whisper := &Whisper{
		privateKeys:  make(map[string]*ecdsa.PrivateKey),
		symKeys:      make(map[string][]byte),
		envelopes:    make(map[common.Hash]*Envelope),
		expirations:  make(map[uint32]mapset.Set),
		peers:        make(map[*Peer]struct{}),
		messageQueue: make(chan *Envelope, messageQueueLimit),
		p2pMsgQueue:  make(chan *Envelope, messageQueueLimit),
		quit:         make(chan struct{}),
	}

	whisper.filters = NewFilters(whisper)

	whisper.settings.Store(minPowIdx, cfg.MinimumAcceptedPOW)
	whisper.settings.Store(maxMsgSizeIdx, cfg.MaxMessageSize)
	whisper.settings.Store(overflowIdx, false)

//P2P低语子协议处理程序
	whisper.protocol = p2p.Protocol{
		Name:    ProtocolName,
		Version: uint(ProtocolVersion),
		Length:  NumberOfMessageCodes,
		Run:     whisper.HandlePeer,
		NodeInfo: func() interface{} {
			return map[string]interface{}{
				"version":        ProtocolVersionStr,
				"maxMessageSize": whisper.MaxMessageSize(),
				"minimumPoW":     whisper.MinPow(),
			}
		},
	}

	return whisper
}

func (w *Whisper) MinPow() float64 {
	val, _ := w.settings.Load(minPowIdx)
	return val.(float64)
}

//maxmessagesize返回可接受的最大消息大小。
func (w *Whisper) MaxMessageSize() uint32 {
	val, _ := w.settings.Load(maxMsgSizeIdx)
	return val.(uint32)
}

//溢出返回消息队列是否已满的指示。
func (w *Whisper) Overflow() bool {
	val, _ := w.settings.Load(overflowIdx)
	return val.(bool)
}

//API返回Whisper实现提供的RPC描述符
func (w *Whisper) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: ProtocolName,
			Version:   ProtocolVersionStr,
			Service:   NewPublicWhisperAPI(w),
			Public:    true,
		},
	}
}

//registerserver注册mailserver接口。
//邮件服务器将使用p2prequestcode处理所有传入的邮件。
func (w *Whisper) RegisterServer(server MailServer) {
	w.mailServer = server
}

//协议返回此特定客户端运行的耳语子协议。
func (w *Whisper) Protocols() []p2p.Protocol {
	return []p2p.Protocol{w.protocol}
}

//version返回耳语子协议版本号。
func (w *Whisper) Version() uint {
	return w.protocol.Version
}

//setmaxmessagesize设置此节点允许的最大消息大小
func (w *Whisper) SetMaxMessageSize(size uint32) error {
	if size > MaxMessageSize {
		return fmt.Errorf("message size too large [%d>%d]", size, MaxMessageSize)
	}
	w.settings.Store(maxMsgSizeIdx, size)
	return nil
}

//setminimumPow设置此节点所需的最小Pow
func (w *Whisper) SetMinimumPoW(val float64) error {
	if val <= 0.0 {
		return fmt.Errorf("invalid PoW: %f", val)
	}
	w.settings.Store(minPowIdx, val)
	return nil
}

//getpeer按ID检索peer
func (w *Whisper) getPeer(peerID []byte) (*Peer, error) {
	w.peerMu.Lock()
	defer w.peerMu.Unlock()
	for p := range w.peers {
		id := p.peer.ID()
		if bytes.Equal(peerID, id[:]) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("Could not find peer with ID: %x", peerID)
}

//allowp2pmessagesfrompeer标记特定的受信任对等机，
//这将允许它发送历史（过期）消息。
func (w *Whisper) AllowP2PMessagesFromPeer(peerID []byte) error {
	p, err := w.getPeer(peerID)
	if err != nil {
		return err
	}
	p.trusted = true
	return nil
}

//RequestHistoricMessages向特定对等端发送带有p2prequestcode的消息，
//它可以实现mailserver接口，并且应该处理这个
//请求和响应一些对等消息（可能已过期），
//不能再转发了。
//耳语协议对信封的格式和内容不可知。
func (w *Whisper) RequestHistoricMessages(peerID []byte, envelope *Envelope) error {
	p, err := w.getPeer(peerID)
	if err != nil {
		return err
	}
	p.trusted = true
	return p2p.Send(p.ws, p2pRequestCode, envelope)
}

//sendp2pmessage向特定对等发送对等消息。
func (w *Whisper) SendP2PMessage(peerID []byte, envelope *Envelope) error {
	p, err := w.getPeer(peerID)
	if err != nil {
		return err
	}
	return w.SendP2PDirect(p, envelope)
}

//sendp2pdirect向特定对等发送对等消息。
func (w *Whisper) SendP2PDirect(peer *Peer, envelope *Envelope) error {
	return p2p.Send(peer.ws, p2pCode, envelope)
}

//NewKeyPair为客户端生成新的加密标识，并注入
//它进入已知的身份信息进行解密。返回新密钥对的ID。
func (w *Whisper) NewKeyPair() (string, error) {
	key, err := crypto.GenerateKey()
	if err != nil || !validatePrivateKey(key) {
key, err = crypto.GenerateKey() //重试一次
	}
	if err != nil {
		return "", err
	}
	if !validatePrivateKey(key) {
		return "", fmt.Errorf("failed to generate valid key")
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	w.keyMu.Lock()
	defer w.keyMu.Unlock()

	if w.privateKeys[id] != nil {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	w.privateKeys[id] = key
	return id, nil
}

//DeleteKeyPair删除指定的密钥（如果存在）。
func (w *Whisper) DeleteKeyPair(key string) bool {
	w.keyMu.Lock()
	defer w.keyMu.Unlock()

	if w.privateKeys[key] != nil {
		delete(w.privateKeys, key)
		return true
	}
	return false
}

//AddKeyPair导入非对称私钥并返回其标识符。
func (w *Whisper) AddKeyPair(key *ecdsa.PrivateKey) (string, error) {
	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	w.keyMu.Lock()
	w.privateKeys[id] = key
	w.keyMu.Unlock()

	return id, nil
}

//hasKeyPair检查耳语节点是否配置了私钥
//指定的公用对。
func (w *Whisper) HasKeyPair(id string) bool {
	w.keyMu.RLock()
	defer w.keyMu.RUnlock()
	return w.privateKeys[id] != nil
}

//getprivatekey检索指定标识的私钥。
func (w *Whisper) GetPrivateKey(id string) (*ecdsa.PrivateKey, error) {
	w.keyMu.RLock()
	defer w.keyMu.RUnlock()
	key := w.privateKeys[id]
	if key == nil {
		return nil, fmt.Errorf("invalid id")
	}
	return key, nil
}

//generatesymkey生成一个随机对称密钥并将其存储在id下，
//然后返回。将在将来用于会话密钥交换。
func (w *Whisper) GenerateSymKey() (string, error) {
	key := make([]byte, aesKeyLength)
	_, err := crand.Read(key)
	if err != nil {
		return "", err
	} else if !validateSymmetricKey(key) {
		return "", fmt.Errorf("error in GenerateSymKey: crypto/rand failed to generate random data")
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	w.keyMu.Lock()
	defer w.keyMu.Unlock()

	if w.symKeys[id] != nil {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	w.symKeys[id] = key
	return id, nil
}

//addsymkeydirect存储密钥并返回其ID。
func (w *Whisper) AddSymKeyDirect(key []byte) (string, error) {
	if len(key) != aesKeyLength {
		return "", fmt.Errorf("wrong key size: %d", len(key))
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	w.keyMu.Lock()
	defer w.keyMu.Unlock()

	if w.symKeys[id] != nil {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	w.symKeys[id] = key
	return id, nil
}

//addsymkeyfrompassword根据密码生成密钥，存储并返回其ID。
func (w *Whisper) AddSymKeyFromPassword(password string) (string, error) {
	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}
	if w.HasSymKey(id) {
		return "", fmt.Errorf("failed to generate unique ID")
	}

	derived, err := deriveKeyMaterial([]byte(password), EnvelopeVersion)
	if err != nil {
		return "", err
	}

	w.keyMu.Lock()
	defer w.keyMu.Unlock()

//需要进行双重检查，因为DeriveKeyMaterial（）非常慢
	if w.symKeys[id] != nil {
		return "", fmt.Errorf("critical error: failed to generate unique ID")
	}
	w.symKeys[id] = derived
	return id, nil
}

//如果有一个键与给定的ID相关联，HassymKey将返回true。
//否则返回false。
func (w *Whisper) HasSymKey(id string) bool {
	w.keyMu.RLock()
	defer w.keyMu.RUnlock()
	return w.symKeys[id] != nil
}

//DeleteSymkey删除与名称字符串关联的键（如果存在）。
func (w *Whisper) DeleteSymKey(id string) bool {
	w.keyMu.Lock()
	defer w.keyMu.Unlock()
	if w.symKeys[id] != nil {
		delete(w.symKeys, id)
		return true
	}
	return false
}

//GetSymkey返回与给定ID关联的对称密钥。
func (w *Whisper) GetSymKey(id string) ([]byte, error) {
	w.keyMu.RLock()
	defer w.keyMu.RUnlock()
	if w.symKeys[id] != nil {
		return w.symKeys[id], nil
	}
	return nil, fmt.Errorf("non-existent key ID")
}

//订阅安装用于筛选、解密的新消息处理程序
//以及随后存储的传入消息。
func (w *Whisper) Subscribe(f *Filter) (string, error) {
	return w.filters.Install(f)
}

//GetFilter按ID返回筛选器。
func (w *Whisper) GetFilter(id string) *Filter {
	return w.filters.Get(id)
}

//取消订阅将删除已安装的消息处理程序。
func (w *Whisper) Unsubscribe(id string) error {
	ok := w.filters.Uninstall(id)
	if !ok {
		return fmt.Errorf("Unsubscribe: Invalid ID")
	}
	return nil
}

//send将一条消息插入到whisper发送队列中，并在
//网络在未来的周期中。
func (w *Whisper) Send(envelope *Envelope) error {
	ok, err := w.add(envelope)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("failed to add envelope")
	}
	return err
}

//start实现node.service，启动后台数据传播线程
//关于耳语协议。
func (w *Whisper) Start(*p2p.Server) error {
	log.Info("started whisper v." + ProtocolVersionStr)
	go w.update()

	numCPU := runtime.NumCPU()
	for i := 0; i < numCPU; i++ {
		go w.processQueue()
	}

	return nil
}

//stop实现node.service，停止后台数据传播线程
//关于耳语协议。
func (w *Whisper) Stop() error {
	close(w.quit)
	log.Info("whisper stopped")
	return nil
}

//当低语子协议时，底层p2p层调用handlepeer。
//已协商连接。
func (w *Whisper) HandlePeer(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
//创建新的对等点并开始跟踪它
	whisperPeer := newPeer(w, peer, rw)

	w.peerMu.Lock()
	w.peers[whisperPeer] = struct{}{}
	w.peerMu.Unlock()

	defer func() {
		w.peerMu.Lock()
		delete(w.peers, whisperPeer)
		w.peerMu.Unlock()
	}()

//运行对等握手和状态更新
	if err := whisperPeer.handshake(); err != nil {
		return err
	}
	whisperPeer.start()
	defer whisperPeer.stop()

	return w.runMessageLoop(whisperPeer, rw)
}

//runmessageloop直接读取和处理入站消息以合并到客户端全局状态。
func (w *Whisper) runMessageLoop(p *Peer, rw p2p.MsgReadWriter) error {
	for {
//获取下一个数据包
		packet, err := rw.ReadMsg()
		if err != nil {
			log.Info("message loop", "peer", p.peer.ID(), "err", err)
			return err
		}
		if packet.Size > w.MaxMessageSize() {
			log.Warn("oversized message received", "peer", p.peer.ID())
			return errors.New("oversized message received")
		}

		switch packet.Code {
		case statusCode:
//这不应该发生，但不需要惊慌；忽略这条消息。
			log.Warn("unxepected status message received", "peer", p.peer.ID())
		case messagesCode:
//解码包含的信封
			var envelope Envelope
			if err := packet.Decode(&envelope); err != nil {
				log.Warn("failed to decode envelope, peer will be disconnected", "peer", p.peer.ID(), "err", err)
				return errors.New("invalid envelope")
			}
			cached, err := w.add(&envelope)
			if err != nil {
				log.Warn("bad envelope received, peer will be disconnected", "peer", p.peer.ID(), "err", err)
				return errors.New("invalid envelope")
			}
			if cached {
				p.mark(&envelope)
			}
		case p2pCode:
//点对点消息，直接发送给点绕过POW检查等。
//此消息不应转发给其他对等方，并且
//因此可能不满足POW、到期等要求。
//这些消息只能从受信任的对等方接收。
			if p.trusted {
				var envelope Envelope
				if err := packet.Decode(&envelope); err != nil {
					log.Warn("failed to decode direct message, peer will be disconnected", "peer", p.peer.ID(), "err", err)
					return errors.New("invalid direct message")
				}
				w.postEvent(&envelope, true)
			}
		case p2pRequestCode:
//如果实现了邮件服务器，则必须进行处理。否则忽略。
			if w.mailServer != nil {
				var request Envelope
				if err := packet.Decode(&request); err != nil {
					log.Warn("failed to decode p2p request message, peer will be disconnected", "peer", p.peer.ID(), "err", err)
					return errors.New("invalid p2p request")
				}
				w.mailServer.DeliverMail(p, &request)
			}
		default:
//新的消息类型可能在未来版本的Whisper中实现。
//对于前向兼容性，只需忽略即可。
		}

		packet.Discard()
	}
}

//添加将新信封插入要在其中分发的消息池
//低语网络。它还将信封插入
//适当的时间戳。如果出现错误，应断开连接。
func (w *Whisper) add(envelope *Envelope) (bool, error) {
	now := uint32(time.Now().Unix())
	sent := envelope.Expiry - envelope.TTL

	if sent > now {
		if sent-SynchAllowance > now {
			return false, fmt.Errorf("envelope created in the future [%x]", envelope.Hash())
		}
//重新计算POW，针对时差进行调整，再加上一秒钟的延迟时间
		envelope.calculatePoW(sent - now + 1)
	}

	if envelope.Expiry < now {
		if envelope.Expiry+SynchAllowance*2 < now {
			return false, fmt.Errorf("very old message")
		}
		log.Debug("expired envelope dropped", "hash", envelope.Hash().Hex())
return false, nil //不出错地丢弃信封
	}

	if uint32(envelope.size()) > w.MaxMessageSize() {
		return false, fmt.Errorf("huge messages are not allowed [%x]", envelope.Hash())
	}

	if len(envelope.Version) > 4 {
		return false, fmt.Errorf("oversized version [%x]", envelope.Hash())
	}

	aesNonceSize := len(envelope.AESNonce)
	if aesNonceSize != 0 && aesNonceSize != AESNonceLength {
//标准aes gcm nonce大小为12字节，
//但无法访问常量gcmstandardnoncosize（未导出）
		return false, fmt.Errorf("wrong size of AESNonce: %d bytes [env: %x]", aesNonceSize, envelope.Hash())
	}

	if envelope.PoW() < w.MinPow() {
		log.Debug("envelope with low PoW dropped", "PoW", envelope.PoW(), "hash", envelope.Hash().Hex())
return false, nil //不出错地丢弃信封
	}

	hash := envelope.Hash()

	w.poolMu.Lock()
	_, alreadyCached := w.envelopes[hash]
	if !alreadyCached {
		w.envelopes[hash] = envelope
		if w.expirations[envelope.Expiry] == nil {
			w.expirations[envelope.Expiry] = mapset.NewThreadUnsafeSet()
		}
		if !w.expirations[envelope.Expiry].Contains(hash) {
			w.expirations[envelope.Expiry].Add(hash)
		}
	}
	w.poolMu.Unlock()

	if alreadyCached {
		log.Trace("whisper envelope already cached", "hash", envelope.Hash().Hex())
	} else {
		log.Trace("cached whisper envelope", "hash", envelope.Hash().Hex())
		w.statsMu.Lock()
		w.stats.memoryUsed += envelope.size()
		w.statsMu.Unlock()
w.postEvent(envelope, false) //将新消息通知本地节点
		if w.mailServer != nil {
			w.mailServer.Archive(envelope)
		}
	}
	return true, nil
}

//PostEvent将消息排队以进行进一步处理。
func (w *Whisper) postEvent(envelope *Envelope, isP2P bool) {
//如果传入消息的版本高于
//当前支持的版本，无法解密，
//因此忽略这个信息
	if envelope.Ver() <= EnvelopeVersion {
		if isP2P {
			w.p2pMsgQueue <- envelope
		} else {
			w.checkOverflow()
			w.messageQueue <- envelope
		}
	}
}

//检查溢出检查消息队列是否发生溢出，必要时报告。
func (w *Whisper) checkOverflow() {
	queueSize := len(w.messageQueue)

	if queueSize == messageQueueLimit {
		if !w.Overflow() {
			w.settings.Store(overflowIdx, true)
			log.Warn("message queue overflow")
		}
	} else if queueSize <= messageQueueLimit/2 {
		if w.Overflow() {
			w.settings.Store(overflowIdx, false)
			log.Warn("message queue overflow fixed (back to normal)")
		}
	}
}

//processqueue在whisper节点的生命周期中将消息传递给观察者。
func (w *Whisper) processQueue() {
	var e *Envelope
	for {
		select {
		case <-w.quit:
			return

		case e = <-w.messageQueue:
			w.filters.NotifyWatchers(e, false)

		case e = <-w.p2pMsgQueue:
			w.filters.NotifyWatchers(e, true)
		}
	}
}

//更新循环，直到Whisper节点的生存期，更新其内部
//通过过期池中的过时消息来进行状态。
func (w *Whisper) update() {
//启动一个断续器以检查到期情况
	expire := time.NewTicker(expirationCycle)

//重复更新直到请求终止
	for {
		select {
		case <-expire.C:
			w.expire()

		case <-w.quit:
			return
		}
	}
}

//expire在所有过期时间戳上迭代，删除所有过时的
//来自池的消息。
func (w *Whisper) expire() {
	w.poolMu.Lock()
	defer w.poolMu.Unlock()

	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	w.stats.reset()
	now := uint32(time.Now().Unix())
	for expiry, hashSet := range w.expirations {
		if expiry < now {
//转储所有过期消息并删除时间戳
			hashSet.Each(func(v interface{}) bool {
				sz := w.envelopes[v.(common.Hash)].size()
				delete(w.envelopes, v.(common.Hash))
				w.stats.messagesCleared++
				w.stats.memoryCleared += sz
				w.stats.memoryUsed -= sz
				return false
			})
			w.expirations[expiry].Clear()
			delete(w.expirations, expiry)
		}
	}
}

//stats返回低语节点统计信息。
func (w *Whisper) Stats() Statistics {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()

	return w.stats
}

//信封检索节点当前汇集的所有消息。
func (w *Whisper) Envelopes() []*Envelope {
	w.poolMu.RLock()
	defer w.poolMu.RUnlock()

	all := make([]*Envelope, 0, len(w.envelopes))
	for _, envelope := range w.envelopes {
		all = append(all, envelope)
	}
	return all
}

//消息迭代当前所有浮动信封
//并检索此筛选器可以解密的所有消息。
func (w *Whisper) Messages(id string) []*ReceivedMessage {
	result := make([]*ReceivedMessage, 0)
	w.poolMu.RLock()
	defer w.poolMu.RUnlock()

	if filter := w.filters.Get(id); filter != nil {
		for _, env := range w.envelopes {
			msg := filter.processEnvelope(env)
			if msg != nil {
				result = append(result, msg)
			}
		}
	}
	return result
}

//ISenvelopecached检查是否已接收和缓存具有特定哈希的信封。
func (w *Whisper) isEnvelopeCached(hash common.Hash) bool {
	w.poolMu.Lock()
	defer w.poolMu.Unlock()

	_, exist := w.envelopes[hash]
	return exist
}

//重置在每个到期周期后重置节点的统计信息。
func (s *Statistics) reset() {
	s.cycles++
	s.totalMessagesCleared += s.messagesCleared

	s.memoryCleared = 0
	s.messagesCleared = 0
}

//validatePublickey检查给定公钥的格式。
func ValidatePublicKey(k *ecdsa.PublicKey) bool {
	return k != nil && k.X != nil && k.Y != nil && k.X.Sign() != 0 && k.Y.Sign() != 0
}

//validateprivatekey检查给定私钥的格式。
func validatePrivateKey(k *ecdsa.PrivateKey) bool {
	if k == nil || k.D == nil || k.D.Sign() == 0 {
		return false
	}
	return ValidatePublicKey(&k.PublicKey)
}

//如果键包含所有零，则validateSymmetrickey返回false
func validateSymmetricKey(k []byte) bool {
	return len(k) > 0 && !containsOnlyZeros(k)
}

//containsonlyzer检查数据是否只包含零。
func containsOnlyZeros(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}

//BytesTouintLittleEndian将切片转换为64位无符号整数。
func bytesToUintLittleEndian(b []byte) (res uint64) {
	mul := uint64(1)
	for i := 0; i < len(b); i++ {
		res += uint64(b[i]) * mul
		mul *= 256
	}
	return res
}

//bytestouintbigendian将切片转换为64位无符号整数。
func BytesToUintBigEndian(b []byte) (res uint64) {
	for i := 0; i < len(b); i++ {
		res *= 256
		res += uint64(b[i])
	}
	return res
}

//DeriveKeyMaterial从密钥或密码派生对称密钥材质。
//pbkdf2用于安全性，以防人们使用密码而不是随机生成的密钥。
func deriveKeyMaterial(key []byte, version uint64) (derivedKey []byte, err error) {
	if version == 0 {
//kdf的平均计算时间不应少于0.1秒，
//因为这是一次会议经验
		derivedKey := pbkdf2.Key(key, nil, 65356, aesKeyLength, sha256.New)
		return derivedKey, nil
	}
	return nil, unknownVersionError(version)
}

//GenerateRandomID生成一个随机字符串，然后返回该字符串用作键ID
func GenerateRandomID() (id string, err error) {
	buf := make([]byte, keyIdSize)
	_, err = crand.Read(buf)
	if err != nil {
		return "", err
	}
	if !validateSymmetricKey(buf) {
		return "", fmt.Errorf("error in generateRandomID: crypto/rand failed to generate random data")
	}
	id = common.Bytes2Hex(buf)
	return id, err
}
