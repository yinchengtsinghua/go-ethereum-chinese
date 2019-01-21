
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
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"fmt"
	"hash"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/pot"
	"github.com/ethereum/go-ethereum/swarm/storage"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv5"
	"golang.org/x/crypto/sha3"
)

const (
	defaultPaddingByteSize     = 16
	DefaultMsgTTL              = time.Second * 120
	defaultDigestCacheTTL      = time.Second * 10
	defaultSymKeyCacheCapacity = 512
digestLength               = 32 //用于PSS缓存的摘要的字节长度（当前与Swarm Chunk哈希相同）
	defaultWhisperWorkTime     = 3
	defaultWhisperPoW          = 0.0000000001
	defaultMaxMsgSize          = 1024 * 1024
	defaultCleanInterval       = time.Second * 60 * 10
	defaultOutboxCapacity      = 100000
	pssProtocolName            = "pss"
	pssVersion                 = 2
	hasherCount                = 8
)

var (
	addressLength = len(pot.Address{})
)

//缓存用于防止反向路由
//也将有助于防洪机制
//和邮箱实现
type pssCacheEntry struct {
	expiresAt time.Time
}

//允许访问p2p.protocols.peer.send的抽象
type senderPeer interface {
	Info() *p2p.PeerInfo
	ID() enode.ID
	Address() []byte
	Send(context.Context, interface{}) error
}

//每键对等相关信息
//成员“protected”防止对实例进行垃圾收集
type pssPeer struct {
	lastSeen  time.Time
	address   PssAddress
	protected bool
}

//PSS配置参数
type PssParams struct {
	MsgTTL              time.Duration
	CacheTTL            time.Duration
	privateKey          *ecdsa.PrivateKey
	SymKeyCacheCapacity int
AllowRaw            bool //如果为真，则允许在不使用内置PSS加密的情况下发送和接收消息
}

//PSS的正常默认值
func NewPssParams() *PssParams {
	return &PssParams{
		MsgTTL:              DefaultMsgTTL,
		CacheTTL:            defaultDigestCacheTTL,
		SymKeyCacheCapacity: defaultSymKeyCacheCapacity,
	}
}

func (params *PssParams) WithPrivateKey(privatekey *ecdsa.PrivateKey) *PssParams {
	params.privateKey = privatekey
	return params
}

//顶级PSS对象，负责消息发送、接收、解密和加密、消息处理程序调度器和消息转发。
//
//实现节点服务
type Pss struct {
*network.Kademlia                   //我们可以从这里得到卡德米利亚的地址
privateKey        *ecdsa.PrivateKey //PSS可以拥有自己的独立密钥
w                 *whisper.Whisper  //密钥和加密后端
auxAPIs           []rpc.API         //内置（握手、测试）可以添加API

//发送和转发
fwdPool         map[string]*protocols.Peer //跟踪PSSMSG路由层上的所有对等端
	fwdPoolMu       sync.RWMutex
fwdCache        map[pssDigest]pssCacheEntry //pssmsg中映射到expiry、cache以确定是否删除msg的唯一字段的校验和
	fwdCacheMu      sync.RWMutex
cacheTTL        time.Duration //在fwdcache中保留消息的时间（未实现）
	msgTTL          time.Duration
	paddingByteSize int
	capstring       string
	outbox          chan *PssMsg

//密钥与对等体
pubKeyPool                 map[string]map[Topic]*pssPeer //按主题将十六进制公钥映射到对等地址。
	pubKeyPoolMu               sync.RWMutex
symKeyPool                 map[string]map[Topic]*pssPeer //按主题将symkeyid映射到对等地址。
	symKeyPoolMu               sync.RWMutex
symKeyDecryptCache         []*string //快速查找最近用于解密的符号键；最后使用的是堆栈顶部
symKeyDecryptCacheCursor   int       //指向上次使用的、包装在symkeydecryptcache数组上的模块化光标
symKeyDecryptCacheCapacity int       //要保留的符号键的最大数量。

//消息处理
handlers         map[Topic]map[*handler]bool //基于主题和版本的PSS有效负载处理程序。参见pss.handle（）。
	handlersMu       sync.RWMutex
	hashPool         sync.Pool
topicHandlerCaps map[Topic]*handlerCaps //缓存每个主题处理程序的功能（请参见handlercap*types.go中的consts）

//过程
	quitC chan struct{}
}

func (p *Pss) String() string {
	return fmt.Sprintf("pss: addr %x, pubkey %v", p.BaseAddr(), common.ToHex(crypto.FromECDSAPub(&p.privateKey.PublicKey)))
}

//创建新的PSS实例。
//
//除了params，它还需要一个集群网络kademlia
//以及用于消息缓存存储的文件存储。
func NewPss(k *network.Kademlia, params *PssParams) (*Pss, error) {
	if params.privateKey == nil {
		return nil, errors.New("missing private key for pss")
	}
	cap := p2p.Cap{
		Name:    pssProtocolName,
		Version: pssVersion,
	}
	ps := &Pss{
		Kademlia:   k,
		privateKey: params.privateKey,
		w:          whisper.New(&whisper.DefaultConfig),
		quitC:      make(chan struct{}),

		fwdPool:         make(map[string]*protocols.Peer),
		fwdCache:        make(map[pssDigest]pssCacheEntry),
		cacheTTL:        params.CacheTTL,
		msgTTL:          params.MsgTTL,
		paddingByteSize: defaultPaddingByteSize,
		capstring:       cap.String(),
		outbox:          make(chan *PssMsg, defaultOutboxCapacity),

		pubKeyPool:                 make(map[string]map[Topic]*pssPeer),
		symKeyPool:                 make(map[string]map[Topic]*pssPeer),
		symKeyDecryptCache:         make([]*string, params.SymKeyCacheCapacity),
		symKeyDecryptCacheCapacity: params.SymKeyCacheCapacity,

		handlers:         make(map[Topic]map[*handler]bool),
		topicHandlerCaps: make(map[Topic]*handlerCaps),

		hashPool: sync.Pool{
			New: func() interface{} {
				return sha3.NewLegacyKeccak256()
			},
		},
	}

	for i := 0; i < hasherCount; i++ {
		hashfunc := storage.MakeHashFunc(storage.DefaultHash)()
		ps.hashPool.Put(hashfunc)
	}

	return ps, nil
}

////////////////////////////////////////////////
//章节：节点、服务接口
////////////////////////////////////////////////

func (p *Pss) Start(srv *p2p.Server) error {
	go func() {
		ticker := time.NewTicker(defaultCleanInterval)
		cacheTicker := time.NewTicker(p.cacheTTL)
		defer ticker.Stop()
		defer cacheTicker.Stop()
		for {
			select {
			case <-cacheTicker.C:
				p.cleanFwdCache()
			case <-ticker.C:
				p.cleanKeys()
			case <-p.quitC:
				return
			}
		}
	}()
	go func() {
		for {
			select {
			case msg := <-p.outbox:
				err := p.forward(msg)
				if err != nil {
					log.Error(err.Error())
					metrics.GetOrRegisterCounter("pss.forward.err", nil).Inc(1)
				}
			case <-p.quitC:
				return
			}
		}
	}()
	log.Info("Started Pss")
	log.Info("Loaded EC keys", "pubkey", common.ToHex(crypto.FromECDSAPub(p.PublicKey())), "secp256", common.ToHex(crypto.CompressPubkey(p.PublicKey())))
	return nil
}

func (p *Pss) Stop() error {
	log.Info("Pss shutting down")
	close(p.quitC)
	return nil
}

var pssSpec = &protocols.Spec{
	Name:       pssProtocolName,
	Version:    pssVersion,
	MaxMsgSize: defaultMaxMsgSize,
	Messages: []interface{}{
		PssMsg{},
	},
}

func (p *Pss) Protocols() []p2p.Protocol {
	return []p2p.Protocol{
		{
			Name:    pssSpec.Name,
			Version: pssSpec.Version,
			Length:  pssSpec.Length(),
			Run:     p.Run,
		},
	}
}

func (p *Pss) Run(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
	pp := protocols.NewPeer(peer, rw, pssSpec)
	p.fwdPoolMu.Lock()
	p.fwdPool[peer.Info().ID] = pp
	p.fwdPoolMu.Unlock()
	return pp.Run(p.handlePssMsg)
}

func (p *Pss) APIs() []rpc.API {
	apis := []rpc.API{
		{
			Namespace: "pss",
			Version:   "1.0",
			Service:   NewAPI(p),
			Public:    true,
		},
	}
	apis = append(apis, p.auxAPIs...)
	return apis
}

//向PSS API添加API方法
//必须在节点启动之前运行
func (p *Pss) addAPI(api rpc.API) {
	p.auxAPIs = append(p.auxAPIs, api)
}

//返回PSS节点的Swarm Kademlia地址
func (p *Pss) BaseAddr() []byte {
	return p.Kademlia.BaseAddr()
}

//返回PSS节点的公钥
func (p *Pss) PublicKey() *ecdsa.PublicKey {
	return &p.privateKey.PublicKey
}

////////////////////////////////////////////////
//部分：消息处理
////////////////////////////////////////////////

//将处理程序函数链接到主题
//
//信封主题与
//指定的主题将传递给给定的处理程序函数。
//
//每个主题可能有任意数量的处理程序函数。
//
//返回需要调用的注销函数
//注销处理程序，
func (p *Pss) Register(topic *Topic, hndlr *handler) func() {
	p.handlersMu.Lock()
	defer p.handlersMu.Unlock()
	handlers := p.handlers[*topic]
	if handlers == nil {
		handlers = make(map[*handler]bool)
		p.handlers[*topic] = handlers
		log.Debug("registered handler", "caps", hndlr.caps)
	}
	if hndlr.caps == nil {
		hndlr.caps = &handlerCaps{}
	}
	handlers[hndlr] = true
	if _, ok := p.topicHandlerCaps[*topic]; !ok {
		p.topicHandlerCaps[*topic] = &handlerCaps{}
	}
	if hndlr.caps.raw {
		p.topicHandlerCaps[*topic].raw = true
	}
	if hndlr.caps.prox {
		p.topicHandlerCaps[*topic].prox = true
	}
	return func() { p.deregister(topic, hndlr) }
}
func (p *Pss) deregister(topic *Topic, hndlr *handler) {
	p.handlersMu.Lock()
	defer p.handlersMu.Unlock()
	handlers := p.handlers[*topic]
	if len(handlers) > 1 {
		delete(p.handlers, *topic)
//既然处理程序不在，主题帽可能已经更改。
		caps := &handlerCaps{}
		for h := range handlers {
			if h.caps.raw {
				caps.raw = true
			}
			if h.caps.prox {
				caps.prox = true
			}
		}
		p.topicHandlerCaps[*topic] = caps
		return
	}
	delete(handlers, hndlr)
}

//获取各自主题的所有已注册处理程序
func (p *Pss) getHandlers(topic Topic) map[*handler]bool {
	p.handlersMu.RLock()
	defer p.handlersMu.RUnlock()
	return p.handlers[topic]
}

//筛选要处理或转发的传入邮件。
//检查地址是否部分匹配
//如果是的话，它可以是我们的，我们处理它
//仅当有效负载无效时才会将错误传递给PSS协议处理程序PSSMSG
func (p *Pss) handlePssMsg(ctx context.Context, msg interface{}) error {
	metrics.GetOrRegisterCounter("pss.handlepssmsg", nil).Inc(1)
	pssmsg, ok := msg.(*PssMsg)
	if !ok {
		return fmt.Errorf("invalid message type. Expected *PssMsg, got %T ", msg)
	}
	log.Trace("handler", "self", label(p.Kademlia.BaseAddr()), "topic", label(pssmsg.Payload.Topic[:]))
	if int64(pssmsg.Expire) < time.Now().Unix() {
		metrics.GetOrRegisterCounter("pss.expire", nil).Inc(1)
		log.Warn("pss filtered expired message", "from", common.ToHex(p.Kademlia.BaseAddr()), "to", common.ToHex(pssmsg.To))
		return nil
	}
	if p.checkFwdCache(pssmsg) {
		log.Trace("pss relay block-cache match (process)", "from", common.ToHex(p.Kademlia.BaseAddr()), "to", (common.ToHex(pssmsg.To)))
		return nil
	}
	p.addFwdCache(pssmsg)

	psstopic := Topic(pssmsg.Payload.Topic)

//raw是要检查的最简单的处理程序意外事件，因此请首先检查
	var isRaw bool
	if pssmsg.isRaw() {
		if _, ok := p.topicHandlerCaps[psstopic]; ok {
			if !p.topicHandlerCaps[psstopic].raw {
				log.Debug("No handler for raw message", "topic", psstopic)
				return nil
			}
		}
		isRaw = true
	}

//检查我们是否可以成为收件人：
//-消息和部分地址上没有代理处理程序匹配
//-消息上的代理处理程序，并且无论部分地址是否匹配，我们都在代理中
//存储此结果，这样我们就不会对每个处理程序重新计算
	var isProx bool
	if _, ok := p.topicHandlerCaps[psstopic]; ok {
		isProx = p.topicHandlerCaps[psstopic].prox
	}
	isRecipient := p.isSelfPossibleRecipient(pssmsg, isProx)
	if !isRecipient {
		log.Trace("pss was for someone else :'( ... forwarding", "pss", common.ToHex(p.BaseAddr()), "prox", isProx)
		return p.enqueue(pssmsg)
	}

	log.Trace("pss for us, yay! ... let's process!", "pss", common.ToHex(p.BaseAddr()), "prox", isProx, "raw", isRaw, "topic", label(pssmsg.Payload.Topic[:]))
	if err := p.process(pssmsg, isRaw, isProx); err != nil {
		qerr := p.enqueue(pssmsg)
		if qerr != nil {
			return fmt.Errorf("process fail: processerr %v, queueerr: %v", err, qerr)
		}
	}
	return nil

}

//入口点，用于处理当前节点可以作为目标收件人的消息。
//尝试使用存储的密钥进行对称和非对称解密。
//将消息发送到与消息主题匹配的所有处理程序
func (p *Pss) process(pssmsg *PssMsg, raw bool, prox bool) error {
	metrics.GetOrRegisterCounter("pss.process", nil).Inc(1)

	var err error
	var recvmsg *whisper.ReceivedMessage
	var payload []byte
	var from PssAddress
	var asymmetric bool
	var keyid string
	var keyFunc func(envelope *whisper.Envelope) (*whisper.ReceivedMessage, string, PssAddress, error)

	envelope := pssmsg.Payload
	psstopic := Topic(envelope.Topic)

	if raw {
		payload = pssmsg.Payload.Data
	} else {
		if pssmsg.isSym() {
			keyFunc = p.processSym
		} else {
			asymmetric = true
			keyFunc = p.processAsym
		}

		recvmsg, keyid, from, err = keyFunc(envelope)
		if err != nil {
			return errors.New("Decryption failed")
		}
		payload = recvmsg.Payload
	}

	if len(pssmsg.To) < addressLength {
		if err := p.enqueue(pssmsg); err != nil {
			return err
		}
	}
	p.executeHandlers(psstopic, payload, from, raw, prox, asymmetric, keyid)

	return nil

}

func (p *Pss) executeHandlers(topic Topic, payload []byte, from PssAddress, raw bool, prox bool, asymmetric bool, keyid string) {
	handlers := p.getHandlers(topic)
	peer := p2p.NewPeer(enode.ID{}, fmt.Sprintf("%x", from), []p2p.Cap{})
	for h := range handlers {
		if !h.caps.raw && raw {
			log.Warn("norawhandler")
			continue
		}
		if !h.caps.prox && prox {
			log.Warn("noproxhandler")
			continue
		}
		err := (h.f)(payload, peer, asymmetric, keyid)
		if err != nil {
			log.Warn("Pss handler failed", "err", err)
		}
	}
}

//如果使用部分地址，将返回false
func (p *Pss) isSelfRecipient(msg *PssMsg) bool {
	return bytes.Equal(msg.To, p.Kademlia.BaseAddr())
}

//测试给定消息中最左边的字节与节点的kademlia地址的匹配
func (p *Pss) isSelfPossibleRecipient(msg *PssMsg, prox bool) bool {
	local := p.Kademlia.BaseAddr()

//如果部分地址匹配，无论代理服务器是什么，我们都是可能的收件人。
//如果没有，并且没有设置代理，我们肯定没有
	if bytes.Equal(msg.To, local[:len(msg.To)]) {

		return true
	} else if !prox {
		return false
	}

	depth := p.Kademlia.NeighbourhoodDepth()
	po, _ := network.Pof(p.Kademlia.BaseAddr(), msg.To, 0)
	log.Trace("selfpossible", "po", po, "depth", depth)

	return depth <= po
}

////////////////////////////////////////////////
//部分：加密
////////////////////////////////////////////////

//将对等ECDSA公钥链接到主题
//
//这对于非对称消息交换是必需的
//关于给定主题
//
//“address”中的值将用作
//公钥/主题关联
func (p *Pss) SetPeerPublicKey(pubkey *ecdsa.PublicKey, topic Topic, address PssAddress) error {
	if err := validateAddress(address); err != nil {
		return err
	}
	pubkeybytes := crypto.FromECDSAPub(pubkey)
	if len(pubkeybytes) == 0 {
		return fmt.Errorf("invalid public key: %v", pubkey)
	}
	pubkeyid := common.ToHex(pubkeybytes)
	psp := &pssPeer{
		address: address,
	}
	p.pubKeyPoolMu.Lock()
	if _, ok := p.pubKeyPool[pubkeyid]; !ok {
		p.pubKeyPool[pubkeyid] = make(map[Topic]*pssPeer)
	}
	p.pubKeyPool[pubkeyid][topic] = psp
	p.pubKeyPoolMu.Unlock()
	log.Trace("added pubkey", "pubkeyid", pubkeyid, "topic", topic, "address", address)
	return nil
}

//自动为主题和地址提示生成新的symkey
func (p *Pss) GenerateSymmetricKey(topic Topic, address PssAddress, addToCache bool) (string, error) {
	keyid, err := p.w.GenerateSymKey()
	if err != nil {
		return "", err
	}
	p.addSymmetricKeyToPool(keyid, topic, address, addToCache, false)
	return keyid, nil
}

//将对等对称密钥（任意字节序列）链接到主题
//
//这是对称加密邮件交换所必需的
//关于给定主题
//
//密钥存储在Whisper后端。
//
//如果addtocache设置为true，则密钥将添加到密钥的缓存中。
//用于尝试对传入消息进行对称解密。
//
//返回可用于检索键字节的字符串ID
//从Whisper后端（请参阅pss.getSymmetricKey（））
func (p *Pss) SetSymmetricKey(key []byte, topic Topic, address PssAddress, addtocache bool) (string, error) {
	if err := validateAddress(address); err != nil {
		return "", err
	}
	return p.setSymmetricKey(key, topic, address, addtocache, true)
}

func (p *Pss) setSymmetricKey(key []byte, topic Topic, address PssAddress, addtocache bool, protected bool) (string, error) {
	keyid, err := p.w.AddSymKeyDirect(key)
	if err != nil {
		return "", err
	}
	p.addSymmetricKeyToPool(keyid, topic, address, addtocache, protected)
	return keyid, nil
}

//向PSS密钥池添加对称密钥，并可以选择添加密钥
//用于尝试对称解密的密钥集合
//传入消息
func (p *Pss) addSymmetricKeyToPool(keyid string, topic Topic, address PssAddress, addtocache bool, protected bool) {
	psp := &pssPeer{
		address:   address,
		protected: protected,
	}
	p.symKeyPoolMu.Lock()
	if _, ok := p.symKeyPool[keyid]; !ok {
		p.symKeyPool[keyid] = make(map[Topic]*pssPeer)
	}
	p.symKeyPool[keyid][topic] = psp
	p.symKeyPoolMu.Unlock()
	if addtocache {
		p.symKeyDecryptCacheCursor++
		p.symKeyDecryptCache[p.symKeyDecryptCacheCursor%cap(p.symKeyDecryptCache)] = &keyid
	}
	key, _ := p.GetSymmetricKey(keyid)
	log.Trace("added symkey", "symkeyid", keyid, "symkey", common.ToHex(key), "topic", topic, "address", address, "cache", addtocache)
}

//返回存储在Whisper后端中的对称密钥字节seqyence
//通过其唯一ID
//
//从Whisper后端传递错误值
func (p *Pss) GetSymmetricKey(symkeyid string) ([]byte, error) {
	symkey, err := p.w.GetSymKey(symkeyid)
	if err != nil {
		return nil, err
	}
	return symkey, nil
}

//返回特定公钥的所有录制主题和地址组合
func (p *Pss) GetPublickeyPeers(keyid string) (topic []Topic, address []PssAddress, err error) {
	p.pubKeyPoolMu.RLock()
	defer p.pubKeyPoolMu.RUnlock()
	for t, peer := range p.pubKeyPool[keyid] {
		topic = append(topic, t)
		address = append(address, peer.address)
	}

	return topic, address, nil
}

func (p *Pss) getPeerAddress(keyid string, topic Topic) (PssAddress, error) {
	p.pubKeyPoolMu.RLock()
	defer p.pubKeyPoolMu.RUnlock()
	if peers, ok := p.pubKeyPool[keyid]; ok {
		if t, ok := peers[topic]; ok {
			return t.address, nil
		}
	}
	return nil, fmt.Errorf("peer with pubkey %s, topic %x not found", keyid, topic)
}

//尝试解密、验证和解包
//对称加密消息
//如果成功，则返回解包的Whisper ReceivedMessage结构
//封装解密的消息和Whisper后端ID
//用于解密消息的对称密钥。
//如果解密消息失败或消息已损坏，则失败。
func (p *Pss) processSym(envelope *whisper.Envelope) (*whisper.ReceivedMessage, string, PssAddress, error) {
	metrics.GetOrRegisterCounter("pss.process.sym", nil).Inc(1)

	for i := p.symKeyDecryptCacheCursor; i > p.symKeyDecryptCacheCursor-cap(p.symKeyDecryptCache) && i > 0; i-- {
		symkeyid := p.symKeyDecryptCache[i%cap(p.symKeyDecryptCache)]
		symkey, err := p.w.GetSymKey(*symkeyid)
		if err != nil {
			continue
		}
		recvmsg, err := envelope.OpenSymmetric(symkey)
		if err != nil {
			continue
		}
		if !recvmsg.Validate() {
			return nil, "", nil, fmt.Errorf("symmetrically encrypted message has invalid signature or is corrupt")
		}
		p.symKeyPoolMu.Lock()
		from := p.symKeyPool[*symkeyid][Topic(envelope.Topic)].address
		p.symKeyPoolMu.Unlock()
		p.symKeyDecryptCacheCursor++
		p.symKeyDecryptCache[p.symKeyDecryptCacheCursor%cap(p.symKeyDecryptCache)] = symkeyid
		return recvmsg, *symkeyid, from, nil
	}
	return nil, "", nil, fmt.Errorf("could not decrypt message")
}

//尝试解密、验证和解包
//非对称加密消息
//如果成功，则返回解包的Whisper ReceivedMessage结构
//封装解密的消息，以及
//用于解密消息的公钥。
//如果解密消息失败或消息已损坏，则失败。
func (p *Pss) processAsym(envelope *whisper.Envelope) (*whisper.ReceivedMessage, string, PssAddress, error) {
	metrics.GetOrRegisterCounter("pss.process.asym", nil).Inc(1)

	recvmsg, err := envelope.OpenAsymmetric(p.privateKey)
	if err != nil {
		return nil, "", nil, fmt.Errorf("could not decrypt message: %s", err)
	}
//检查签名（如果有签名），去掉填充
	if !recvmsg.Validate() {
		return nil, "", nil, fmt.Errorf("invalid message")
	}
	pubkeyid := common.ToHex(crypto.FromECDSAPub(recvmsg.Src))
	var from PssAddress
	p.pubKeyPoolMu.Lock()
	if p.pubKeyPool[pubkeyid][Topic(envelope.Topic)] != nil {
		from = p.pubKeyPool[pubkeyid][Topic(envelope.Topic)].address
	}
	p.pubKeyPoolMu.Unlock()
	return recvmsg, pubkeyid, from, nil
}

//symkey垃圾收集
//如果出现以下情况，钥匙将被取下：
//-未标记为受保护
//-不在传入解密缓存中
func (p *Pss) cleanKeys() (count int) {
	for keyid, peertopics := range p.symKeyPool {
		var expiredtopics []Topic
		for topic, psp := range peertopics {
			if psp.protected {
				continue
			}

			var match bool
			for i := p.symKeyDecryptCacheCursor; i > p.symKeyDecryptCacheCursor-cap(p.symKeyDecryptCache) && i > 0; i-- {
				cacheid := p.symKeyDecryptCache[i%cap(p.symKeyDecryptCache)]
				if *cacheid == keyid {
					match = true
				}
			}
			if !match {
				expiredtopics = append(expiredtopics, topic)
			}
		}
		for _, topic := range expiredtopics {
			p.symKeyPoolMu.Lock()
			delete(p.symKeyPool[keyid], topic)
			log.Trace("symkey cleanup deletion", "symkeyid", keyid, "topic", topic, "val", p.symKeyPool[keyid])
			p.symKeyPoolMu.Unlock()
			count++
		}
	}
	return
}

////////////////////////////////////////////////
//部分：消息发送
////////////////////////////////////////////////

func (p *Pss) enqueue(msg *PssMsg) error {
	select {
	case p.outbox <- msg:
		return nil
	default:
	}

	metrics.GetOrRegisterCounter("pss.enqueue.outbox.full", nil).Inc(1)
	return errors.New("outbox full")
}

//发送原始消息（任何加密都由调用客户端负责）
//
//如果不允许原始消息，将失败
func (p *Pss) SendRaw(address PssAddress, topic Topic, msg []byte) error {
	if err := validateAddress(address); err != nil {
		return err
	}
	pssMsgParams := &msgParams{
		raw: true,
	}
	payload := &whisper.Envelope{
		Data:  msg,
		Topic: whisper.TopicType(topic),
	}
	pssMsg := newPssMsg(pssMsgParams)
	pssMsg.To = address
	pssMsg.Expire = uint32(time.Now().Add(p.msgTTL).Unix())
	pssMsg.Payload = payload
	p.addFwdCache(pssMsg)
	err := p.enqueue(pssMsg)
	if err != nil {
		return err
	}

//如果我们有关于这个主题的代理处理程序
//同时向我们自己传递信息
	if _, ok := p.topicHandlerCaps[topic]; ok {
		if p.isSelfPossibleRecipient(pssMsg, true) && p.topicHandlerCaps[topic].prox {
			return p.process(pssMsg, true, true)
		}
	}
	return nil
}

//使用对称加密发送消息
//
//如果密钥ID与任何存储的对称密钥不匹配，则失败
func (p *Pss) SendSym(symkeyid string, topic Topic, msg []byte) error {
	symkey, err := p.GetSymmetricKey(symkeyid)
	if err != nil {
		return fmt.Errorf("missing valid send symkey %s: %v", symkeyid, err)
	}
	p.symKeyPoolMu.Lock()
	psp, ok := p.symKeyPool[symkeyid][topic]
	p.symKeyPoolMu.Unlock()
	if !ok {
		return fmt.Errorf("invalid topic '%s' for symkey '%s'", topic.String(), symkeyid)
	}
	return p.send(psp.address, topic, msg, false, symkey)
}

//使用非对称加密发送消息
//
//如果密钥ID与存储的公钥中的任何一个不匹配，则失败
func (p *Pss) SendAsym(pubkeyid string, topic Topic, msg []byte) error {
	if _, err := crypto.UnmarshalPubkey(common.FromHex(pubkeyid)); err != nil {
		return fmt.Errorf("Cannot unmarshal pubkey: %x", pubkeyid)
	}
	p.pubKeyPoolMu.Lock()
	psp, ok := p.pubKeyPool[pubkeyid][topic]
	p.pubKeyPoolMu.Unlock()
	if !ok {
		return fmt.Errorf("invalid topic '%s' for pubkey '%s'", topic.String(), pubkeyid)
	}
	return p.send(psp.address, topic, msg, true, common.FromHex(pubkeyid))
}

//send不区分有效负载，并将接受任何字节片作为有效负载
//它为指定的收件人和主题生成一个耳语信封，
//并将消息有效负载包装在其中。
//TODO:实现正确的消息填充
func (p *Pss) send(to []byte, topic Topic, msg []byte, asymmetric bool, key []byte) error {
	metrics.GetOrRegisterCounter("pss.send", nil).Inc(1)

	if key == nil || bytes.Equal(key, []byte{}) {
		return fmt.Errorf("Zero length key passed to pss send")
	}
	padding := make([]byte, p.paddingByteSize)
	c, err := rand.Read(padding)
	if err != nil {
		return err
	} else if c < p.paddingByteSize {
		return fmt.Errorf("invalid padding length: %d", c)
	}
	wparams := &whisper.MessageParams{
		TTL:      defaultWhisperTTL,
		Src:      p.privateKey,
		Topic:    whisper.TopicType(topic),
		WorkTime: defaultWhisperWorkTime,
		PoW:      defaultWhisperPoW,
		Payload:  msg,
		Padding:  padding,
	}
	if asymmetric {
		pk, err := crypto.UnmarshalPubkey(key)
		if err != nil {
			return fmt.Errorf("Cannot unmarshal pubkey: %x", key)
		}
		wparams.Dst = pk
	} else {
		wparams.KeySym = key
	}
//设置传出消息容器，该容器执行加密和信封包装
	woutmsg, err := whisper.NewSentMessage(wparams)
	if err != nil {
		return fmt.Errorf("failed to generate whisper message encapsulation: %v", err)
	}
//执行加密。
//由于设置难度很低，无法执行/执行可忽略的POW
//之后，消息就可以发送了
	envelope, err := woutmsg.Wrap(wparams)
	if err != nil {
		return fmt.Errorf("failed to perform whisper encryption: %v", err)
	}
	log.Trace("pssmsg whisper done", "env", envelope, "wparams payload", common.ToHex(wparams.Payload), "to", common.ToHex(to), "asym", asymmetric, "key", common.ToHex(key))

//准备DEVP2P传输
	pssMsgParams := &msgParams{
		sym: !asymmetric,
	}
	pssMsg := newPssMsg(pssMsgParams)
	pssMsg.To = to
	pssMsg.Expire = uint32(time.Now().Add(p.msgTTL).Unix())
	pssMsg.Payload = envelope
	err = p.enqueue(pssMsg)
	if err != nil {
		return err
	}
	if _, ok := p.topicHandlerCaps[topic]; ok {
		if p.isSelfPossibleRecipient(pssMsg, true) && p.topicHandlerCaps[topic].prox {
			return p.process(pssMsg, true, true)
		}
	}
	return nil
}

//sendfunc是一个帮助函数，它尝试发送消息并在成功时返回true。
//它在这里设置为在生产中使用，并在测试中可选地重写。
var sendFunc func(p *Pss, sp *network.Peer, msg *PssMsg) bool = sendMsg

//尝试发送消息，如果成功则返回true
func sendMsg(p *Pss, sp *network.Peer, msg *PssMsg) bool {
	var isPssEnabled bool
	info := sp.Info()
	for _, capability := range info.Caps {
		if capability == p.capstring {
			isPssEnabled = true
			break
		}
	}
	if !isPssEnabled {
		log.Error("peer doesn't have matching pss capabilities, skipping", "peer", info.Name, "caps", info.Caps)
		return false
	}

//从转发对等缓存获取协议对等
	p.fwdPoolMu.RLock()
	pp := p.fwdPool[sp.Info().ID]
	p.fwdPoolMu.RUnlock()

	err := pp.Send(context.TODO(), msg)
	if err != nil {
		metrics.GetOrRegisterCounter("pss.pp.send.error", nil).Inc(1)
		log.Error(err.Error())
	}

	return err == nil
}

//根据算法，将基于收件人地址的PSS消息转发给对等方
//如下所述。收件人地址可以是任意长度，字节片将匹配
//等效长度的对等地址的MSB切片。
//
//如果收件人地址（或部分地址）在转发的邻近深度内
//节点，然后它将被转发到转发节点的所有最近邻居。万一
//部分地址，如果存在，则应转发给与部分地址匹配的所有对等方
//是任意的；否则仅限于最接近收件人地址的一个对等机。在任何情况下，如果
//转发失败，节点应尝试将其转发到下一个最佳对等端，直到消息
//已成功转发到至少一个对等机。
func (p *Pss) forward(msg *PssMsg) error {
	metrics.GetOrRegisterCounter("pss.forward", nil).Inc(1)
sent := 0 //成功发送的次数
	to := make([]byte, addressLength)
	copy(to[:len(msg.To)], msg.To)
	neighbourhoodDepth := p.Kademlia.NeighbourhoodDepth()

//光明与黑暗是对立的。从地址中删除的字节越多，越黑，
//但是亮度变小了。这里亮度等于目的地址中给定的位数。
	luminosityRadius := len(msg.To) * 8

//接近顺序功能匹配到邻接位（po<=neighbourhooddepth）
	pof := pot.DefaultPof(neighbourhoodDepth)

//消息广播软阈值
	broadcastThreshold, _ := pof(to, p.BaseAddr(), 0)
	if broadcastThreshold > luminosityRadius {
		broadcastThreshold = luminosityRadius
	}

var onlySendOnce bool //指示是否只应将消息发送到一个地址最近的对等机

//如果从收件人地址而不是基地址测量（参见kademlia.eachconn
//呼叫下面），然后将出现与收件人地址位于同一个近箱中的对等机。
//[至少]靠近一位，但只有在收件人地址中给出这些附加位时。
	if broadcastThreshold < luminosityRadius && broadcastThreshold < neighbourhoodDepth {
		broadcastThreshold++
		onlySendOnce = true
	}

	p.Kademlia.EachConn(to, addressLength*8, func(sp *network.Peer, po int) bool {
		if po < broadcastThreshold && sent > 0 {
return false //停止迭代
		}
		if sendFunc(p, sp, msg) {
			sent++
			if onlySendOnce {
				return false
			}
			if po == addressLength*8 {
//如果成功发送到确切的收件人，则停止迭代（完全匹配完整地址）
				return false
			}
		}
		return true
	})

//如果发送失败，请在发送队列中重新插入消息
	if sent == 0 {
		log.Debug("unable to forward to any peers")
		if err := p.enqueue(msg); err != nil {
			metrics.GetOrRegisterCounter("pss.forward.enqueue.error", nil).Inc(1)
			log.Error(err.Error())
			return err
		}
	}

//缓存消息
	p.addFwdCache(msg)
	return nil
}

////////////////////////////////////////////////
//部分：缓存
////////////////////////////////////////////////

//cleanfwdcache用于定期从转发缓存中删除过期条目。
func (p *Pss) cleanFwdCache() {
	metrics.GetOrRegisterCounter("pss.cleanfwdcache", nil).Inc(1)
	p.fwdCacheMu.Lock()
	defer p.fwdCacheMu.Unlock()
	for k, v := range p.fwdCache {
		if v.expiresAt.Before(time.Now()) {
			delete(p.fwdCache, k)
		}
	}
}

func label(b []byte) string {
	return fmt.Sprintf("%04x", b[:2])
}

//向缓存中添加消息
func (p *Pss) addFwdCache(msg *PssMsg) error {
	metrics.GetOrRegisterCounter("pss.addfwdcache", nil).Inc(1)

	var entry pssCacheEntry
	var ok bool

	p.fwdCacheMu.Lock()
	defer p.fwdCacheMu.Unlock()

	digest := p.digest(msg)
	if entry, ok = p.fwdCache[digest]; !ok {
		entry = pssCacheEntry{}
	}
	entry.expiresAt = time.Now().Add(p.cacheTTL)
	p.fwdCache[digest] = entry
	return nil
}

//检查消息是否在缓存中
func (p *Pss) checkFwdCache(msg *PssMsg) bool {
	p.fwdCacheMu.Lock()
	defer p.fwdCacheMu.Unlock()

	digest := p.digest(msg)
	entry, ok := p.fwdCache[digest]
	if ok {
		if entry.expiresAt.After(time.Now()) {
			log.Trace("unexpired cache", "digest", fmt.Sprintf("%x", digest))
			metrics.GetOrRegisterCounter("pss.checkfwdcache.unexpired", nil).Inc(1)
			return true
		}
		metrics.GetOrRegisterCounter("pss.checkfwdcache.expired", nil).Inc(1)
	}
	return false
}

//消息摘要
func (p *Pss) digest(msg *PssMsg) pssDigest {
	return p.digestBytes(msg.serialize())
}

func (p *Pss) digestBytes(msg []byte) pssDigest {
	hasher := p.hashPool.Get().(hash.Hash)
	defer p.hashPool.Put(hasher)
	hasher.Reset()
	hasher.Write(msg)
	digest := pssDigest{}
	key := hasher.Sum(nil)
	copy(digest[:], key[:digestLength])
	return digest
}

func validateAddress(addr PssAddress) error {
	if len(addr) > addressLength {
		return errors.New("address too long")
	}
	return nil
}
