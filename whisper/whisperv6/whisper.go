
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

package whisperv6

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/sync/syncmap"
)

//统计数据包含多个与消息相关的计数器用于分析
//目的。
type Statistics struct {
	messagesCleared      int
	memoryCleared        int
	memoryUsed           int
	cycles               int
	totalMessagesCleared int
}

const (
maxMsgSizeIdx                            = iota //Whisper节点允许的最大消息长度
overflowIdx                                     //消息队列溢出指示器
minPowIdx                                       //耳语节点所需的最小功率
minPowToleranceIdx                              //在有限的时间内，耳语节点所允许的最小功率
bloomFilterIdx                                  //此节点感兴趣的主题的Bloom筛选器
bloomFilterToleranceIdx                         //低语节点在有限时间内允许的Bloom过滤器
lightClientModeIdx                              //轻客户端模式。（不转发任何消息）
restrictConnectionBetweenLightClientsIdx        //限制两个轻型客户端之间的连接
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

syncAllowance int //允许处理与耳语相关的消息的最长时间（秒）

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
		privateKeys:   make(map[string]*ecdsa.PrivateKey),
		symKeys:       make(map[string][]byte),
		envelopes:     make(map[common.Hash]*Envelope),
		expirations:   make(map[uint32]mapset.Set),
		peers:         make(map[*Peer]struct{}),
		messageQueue:  make(chan *Envelope, messageQueueLimit),
		p2pMsgQueue:   make(chan *Envelope, messageQueueLimit),
		quit:          make(chan struct{}),
		syncAllowance: DefaultSyncAllowance,
	}

	whisper.filters = NewFilters(whisper)

	whisper.settings.Store(minPowIdx, cfg.MinimumAcceptedPOW)
	whisper.settings.Store(maxMsgSizeIdx, cfg.MaxMessageSize)
	whisper.settings.Store(overflowIdx, false)
	whisper.settings.Store(restrictConnectionBetweenLightClientsIdx, cfg.RestrictConnectionBetweenLightClients)

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

//minpow返回此节点所需的pow值。
func (whisper *Whisper) MinPow() float64 {
	val, exist := whisper.settings.Load(minPowIdx)
	if !exist || val == nil {
		return DefaultMinimumPoW
	}
	v, ok := val.(float64)
	if !ok {
		log.Error("Error loading minPowIdx, using default")
		return DefaultMinimumPoW
	}
	return v
}

//minpowTolerance返回在有限的
//功率改变后的时间。如果已经过了足够的时间或电源没有变化
//一旦发生，返回值将与minpow（）的返回值相同。
func (whisper *Whisper) MinPowTolerance() float64 {
	val, exist := whisper.settings.Load(minPowToleranceIdx)
	if !exist || val == nil {
		return DefaultMinimumPoW
	}
	return val.(float64)
}

//Bloomfilter返回所有感兴趣主题的聚合Bloomfilter。
//节点只需发送与公布的Bloom筛选器匹配的消息。
//如果消息与bloom不匹配，则相当于垃圾邮件，对等端将
//断开连接。
func (whisper *Whisper) BloomFilter() []byte {
	val, exist := whisper.settings.Load(bloomFilterIdx)
	if !exist || val == nil {
		return nil
	}
	return val.([]byte)
}

//BloomFilterTolerance返回允许有限
//新花开后的一段时间被宣传给了同行们。如果已经过了足够的时间
//或者没有发生过布卢姆滤波器的变化，返回值将相同。
//作为bloomfilter（）的返回值。
func (whisper *Whisper) BloomFilterTolerance() []byte {
	val, exist := whisper.settings.Load(bloomFilterToleranceIdx)
	if !exist || val == nil {
		return nil
	}
	return val.([]byte)
}

//maxmessagesize返回可接受的最大消息大小。
func (whisper *Whisper) MaxMessageSize() uint32 {
	val, _ := whisper.settings.Load(maxMsgSizeIdx)
	return val.(uint32)
}

//溢出返回消息队列是否已满的指示。
func (whisper *Whisper) Overflow() bool {
	val, _ := whisper.settings.Load(overflowIdx)
	return val.(bool)
}

//API返回Whisper实现提供的RPC描述符
func (whisper *Whisper) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: ProtocolName,
			Version:   ProtocolVersionStr,
			Service:   NewPublicWhisperAPI(whisper),
			Public:    true,
		},
	}
}

//registerserver注册mailserver接口。
//邮件服务器将使用p2prequestcode处理所有传入的邮件。
func (whisper *Whisper) RegisterServer(server MailServer) {
	whisper.mailServer = server
}

//协议返回此特定客户端运行的耳语子协议。
func (whisper *Whisper) Protocols() []p2p.Protocol {
	return []p2p.Protocol{whisper.protocol}
}

//version返回耳语子协议版本号。
func (whisper *Whisper) Version() uint {
	return whisper.protocol.Version
}

//setmaxmessagesize设置此节点允许的最大消息大小
func (whisper *Whisper) SetMaxMessageSize(size uint32) error {
	if size > MaxMessageSize {
		return fmt.Errorf("message size too large [%d>%d]", size, MaxMessageSize)
	}
	whisper.settings.Store(maxMsgSizeIdx, size)
	return nil
}

//setbloomfilter设置新的bloom filter
func (whisper *Whisper) SetBloomFilter(bloom []byte) error {
	if len(bloom) != BloomFilterSize {
		return fmt.Errorf("invalid bloom filter size: %d", len(bloom))
	}

	b := make([]byte, BloomFilterSize)
	copy(b, bloom)

	whisper.settings.Store(bloomFilterIdx, b)
	whisper.notifyPeersAboutBloomFilterChange(b)

	go func() {
//在所有对等方处理通知之前允许一段时间
		time.Sleep(time.Duration(whisper.syncAllowance) * time.Second)
		whisper.settings.Store(bloomFilterToleranceIdx, b)
	}()

	return nil
}

//setminimumPow设置此节点所需的最小Pow
func (whisper *Whisper) SetMinimumPoW(val float64) error {
	if val < 0.0 {
		return fmt.Errorf("invalid PoW: %f", val)
	}

	whisper.settings.Store(minPowIdx, val)
	whisper.notifyPeersAboutPowRequirementChange(val)

	go func() {
//在所有对等方处理通知之前允许一段时间
		time.Sleep(time.Duration(whisper.syncAllowance) * time.Second)
		whisper.settings.Store(minPowToleranceIdx, val)
	}()

	return nil
}

//setminimumPowTest设置测试环境中的最小Pow
func (whisper *Whisper) SetMinimumPowTest(val float64) {
	whisper.settings.Store(minPowIdx, val)
	whisper.notifyPeersAboutPowRequirementChange(val)
	whisper.settings.Store(minPowToleranceIdx, val)
}

//setlightclientmode生成节点light client（不转发任何消息）
func (whisper *Whisper) SetLightClientMode(v bool) {
	whisper.settings.Store(lightClientModeIdx, v)
}

//lightclientmode表示此节点是light client（不转发任何消息）
func (whisper *Whisper) LightClientMode() bool {
	val, exist := whisper.settings.Load(lightClientModeIdx)
	if !exist || val == nil {
		return false
	}
	v, ok := val.(bool)
	return v && ok
}

//LightClientModeConnectionRestricted表示不允许以轻型客户端模式连接到轻型客户端
func (whisper *Whisper) LightClientModeConnectionRestricted() bool {
	val, exist := whisper.settings.Load(restrictConnectionBetweenLightClientsIdx)
	if !exist || val == nil {
		return false
	}
	v, ok := val.(bool)
	return v && ok
}

func (whisper *Whisper) notifyPeersAboutPowRequirementChange(pow float64) {
	arr := whisper.getPeers()
	for _, p := range arr {
		err := p.notifyAboutPowRequirementChange(pow)
		if err != nil {
//允许重试
			err = p.notifyAboutPowRequirementChange(pow)
		}
		if err != nil {
			log.Warn("failed to notify peer about new pow requirement", "peer", p.ID(), "error", err)
		}
	}
}

func (whisper *Whisper) notifyPeersAboutBloomFilterChange(bloom []byte) {
	arr := whisper.getPeers()
	for _, p := range arr {
		err := p.notifyAboutBloomFilterChange(bloom)
		if err != nil {
//允许重试
			err = p.notifyAboutBloomFilterChange(bloom)
		}
		if err != nil {
			log.Warn("failed to notify peer about new bloom filter", "peer", p.ID(), "error", err)
		}
	}
}

func (whisper *Whisper) getPeers() []*Peer {
	arr := make([]*Peer, len(whisper.peers))
	i := 0
	whisper.peerMu.Lock()
	for p := range whisper.peers {
		arr[i] = p
		i++
	}
	whisper.peerMu.Unlock()
	return arr
}

//getpeer按ID检索peer
func (whisper *Whisper) getPeer(peerID []byte) (*Peer, error) {
	whisper.peerMu.Lock()
	defer whisper.peerMu.Unlock()
	for p := range whisper.peers {
		id := p.peer.ID()
		if bytes.Equal(peerID, id[:]) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("Could not find peer with ID: %x", peerID)
}

//allowp2pmessagesfrompeer标记特定的受信任对等机，
//这将允许它发送历史（过期）消息。
func (whisper *Whisper) AllowP2PMessagesFromPeer(peerID []byte) error {
	p, err := whisper.getPeer(peerID)
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
func (whisper *Whisper) RequestHistoricMessages(peerID []byte, envelope *Envelope) error {
	p, err := whisper.getPeer(peerID)
	if err != nil {
		return err
	}
	p.trusted = true
	return p2p.Send(p.ws, p2pRequestCode, envelope)
}

//sendp2pmessage向特定对等发送对等消息。
func (whisper *Whisper) SendP2PMessage(peerID []byte, envelope *Envelope) error {
	p, err := whisper.getPeer(peerID)
	if err != nil {
		return err
	}
	return whisper.SendP2PDirect(p, envelope)
}

//sendp2pdirect向特定对等发送对等消息。
func (whisper *Whisper) SendP2PDirect(peer *Peer, envelope *Envelope) error {
	return p2p.Send(peer.ws, p2pMessageCode, envelope)
}

//NewKeyPair为客户端生成新的加密标识，并注入
//它进入已知的身份信息进行解密。返回新密钥对的ID。
func (whisper *Whisper) NewKeyPair() (string, error) {
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

	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

	if whisper.privateKeys[id] != nil {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	whisper.privateKeys[id] = key
	return id, nil
}

//DeleteKeyPair删除指定的密钥（如果存在）。
func (whisper *Whisper) DeleteKeyPair(key string) bool {
	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

	if whisper.privateKeys[key] != nil {
		delete(whisper.privateKeys, key)
		return true
	}
	return false
}

//AddKeyPair导入非对称私钥并返回其标识符。
func (whisper *Whisper) AddKeyPair(key *ecdsa.PrivateKey) (string, error) {
	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	whisper.keyMu.Lock()
	whisper.privateKeys[id] = key
	whisper.keyMu.Unlock()

	return id, nil
}

//hasKeyPair检查耳语节点是否配置了私钥
//指定的公用对。
func (whisper *Whisper) HasKeyPair(id string) bool {
	whisper.keyMu.RLock()
	defer whisper.keyMu.RUnlock()
	return whisper.privateKeys[id] != nil
}

//getprivatekey检索指定标识的私钥。
func (whisper *Whisper) GetPrivateKey(id string) (*ecdsa.PrivateKey, error) {
	whisper.keyMu.RLock()
	defer whisper.keyMu.RUnlock()
	key := whisper.privateKeys[id]
	if key == nil {
		return nil, fmt.Errorf("invalid id")
	}
	return key, nil
}

//generatesymkey生成一个随机对称密钥并将其存储在id下，
//然后返回。将在将来用于会话密钥交换。
func (whisper *Whisper) GenerateSymKey() (string, error) {
	key, err := generateSecureRandomData(aesKeyLength)
	if err != nil {
		return "", err
	} else if !validateDataIntegrity(key, aesKeyLength) {
		return "", fmt.Errorf("error in GenerateSymKey: crypto/rand failed to generate random data")
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

	if whisper.symKeys[id] != nil {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	whisper.symKeys[id] = key
	return id, nil
}

//addsymkeydirect存储密钥并返回其ID。
func (whisper *Whisper) AddSymKeyDirect(key []byte) (string, error) {
	if len(key) != aesKeyLength {
		return "", fmt.Errorf("wrong key size: %d", len(key))
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

	if whisper.symKeys[id] != nil {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	whisper.symKeys[id] = key
	return id, nil
}

//addsymkeyfrompassword根据密码生成密钥，存储并返回其ID。
func (whisper *Whisper) AddSymKeyFromPassword(password string) (string, error) {
	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}
	if whisper.HasSymKey(id) {
		return "", fmt.Errorf("failed to generate unique ID")
	}

//kdf在平均计算机上运行不少于0.1秒，
//因为这是一次会议经验
	derived := pbkdf2.Key([]byte(password), nil, 65356, aesKeyLength, sha256.New)
	if err != nil {
		return "", err
	}

	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

//需要进行双重检查，因为DeriveKeyMaterial（）非常慢
	if whisper.symKeys[id] != nil {
		return "", fmt.Errorf("critical error: failed to generate unique ID")
	}
	whisper.symKeys[id] = derived
	return id, nil
}

//如果有一个键与给定的ID相关联，HassymKey将返回true。
//否则返回false。
func (whisper *Whisper) HasSymKey(id string) bool {
	whisper.keyMu.RLock()
	defer whisper.keyMu.RUnlock()
	return whisper.symKeys[id] != nil
}

//DeleteSymkey删除与名称字符串关联的键（如果存在）。
func (whisper *Whisper) DeleteSymKey(id string) bool {
	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()
	if whisper.symKeys[id] != nil {
		delete(whisper.symKeys, id)
		return true
	}
	return false
}

//GetSymkey返回与给定ID关联的对称密钥。
func (whisper *Whisper) GetSymKey(id string) ([]byte, error) {
	whisper.keyMu.RLock()
	defer whisper.keyMu.RUnlock()
	if whisper.symKeys[id] != nil {
		return whisper.symKeys[id], nil
	}
	return nil, fmt.Errorf("non-existent key ID")
}

//订阅安装用于筛选、解密的新消息处理程序
//以及随后存储的传入消息。
func (whisper *Whisper) Subscribe(f *Filter) (string, error) {
	s, err := whisper.filters.Install(f)
	if err == nil {
		whisper.updateBloomFilter(f)
	}
	return s, err
}

//更新BloomFilter重新计算BloomFilter的新值，
//必要时通知同行。
func (whisper *Whisper) updateBloomFilter(f *Filter) {
	aggregate := make([]byte, BloomFilterSize)
	for _, t := range f.Topics {
		top := BytesToTopic(t)
		b := TopicToBloom(top)
		aggregate = addBloom(aggregate, b)
	}

	if !BloomFilterMatch(whisper.BloomFilter(), aggregate) {
//必须更新现有的Bloom过滤器
		aggregate = addBloom(whisper.BloomFilter(), aggregate)
		whisper.SetBloomFilter(aggregate)
	}
}

//GetFilter按ID返回筛选器。
func (whisper *Whisper) GetFilter(id string) *Filter {
	return whisper.filters.Get(id)
}

//取消订阅将删除已安装的消息处理程序。
func (whisper *Whisper) Unsubscribe(id string) error {
	ok := whisper.filters.Uninstall(id)
	if !ok {
		return fmt.Errorf("Unsubscribe: Invalid ID")
	}
	return nil
}

//send将一条消息插入到whisper发送队列中，并在
//网络在未来的周期中。
func (whisper *Whisper) Send(envelope *Envelope) error {
	ok, err := whisper.add(envelope, false)
	if err == nil && !ok {
		return fmt.Errorf("failed to add envelope")
	}
	return err
}

//start实现node.service，启动后台数据传播线程
//关于耳语协议。
func (whisper *Whisper) Start(*p2p.Server) error {
	log.Info("started whisper v." + ProtocolVersionStr)
	go whisper.update()

	numCPU := runtime.NumCPU()
	for i := 0; i < numCPU; i++ {
		go whisper.processQueue()
	}

	return nil
}

//stop实现node.service，停止后台数据传播线程
//关于耳语协议。
func (whisper *Whisper) Stop() error {
	close(whisper.quit)
	log.Info("whisper stopped")
	return nil
}

//当低语子协议时，底层p2p层调用handlepeer。
//已协商连接。
func (whisper *Whisper) HandlePeer(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
//创建新的对等点并开始跟踪它
	whisperPeer := newPeer(whisper, peer, rw)

	whisper.peerMu.Lock()
	whisper.peers[whisperPeer] = struct{}{}
	whisper.peerMu.Unlock()

	defer func() {
		whisper.peerMu.Lock()
		delete(whisper.peers, whisperPeer)
		whisper.peerMu.Unlock()
	}()

//运行对等握手和状态更新
	if err := whisperPeer.handshake(); err != nil {
		return err
	}
	whisperPeer.start()
	defer whisperPeer.stop()

	return whisper.runMessageLoop(whisperPeer, rw)
}

//runmessageloop直接读取和处理入站消息以合并到客户端全局状态。
func (whisper *Whisper) runMessageLoop(p *Peer, rw p2p.MsgReadWriter) error {
	for {
//获取下一个数据包
		packet, err := rw.ReadMsg()
		if err != nil {
			log.Info("message loop", "peer", p.peer.ID(), "err", err)
			return err
		}
		if packet.Size > whisper.MaxMessageSize() {
			log.Warn("oversized message received", "peer", p.peer.ID())
			return errors.New("oversized message received")
		}

		switch packet.Code {
		case statusCode:
//这不应该发生，但不需要惊慌；忽略这条消息。
			log.Warn("unxepected status message received", "peer", p.peer.ID())
		case messagesCode:
//解码包含的信封
			var envelopes []*Envelope
			if err := packet.Decode(&envelopes); err != nil {
				log.Warn("failed to decode envelopes, peer will be disconnected", "peer", p.peer.ID(), "err", err)
				return errors.New("invalid envelopes")
			}

			trouble := false
			for _, env := range envelopes {
				cached, err := whisper.add(env, whisper.LightClientMode())
				if err != nil {
					trouble = true
					log.Error("bad envelope received, peer will be disconnected", "peer", p.peer.ID(), "err", err)
				}
				if cached {
					p.mark(env)
				}
			}

			if trouble {
				return errors.New("invalid envelope")
			}
		case powRequirementCode:
			s := rlp.NewStream(packet.Payload, uint64(packet.Size))
			i, err := s.Uint()
			if err != nil {
				log.Warn("failed to decode powRequirementCode message, peer will be disconnected", "peer", p.peer.ID(), "err", err)
				return errors.New("invalid powRequirementCode message")
			}
			f := math.Float64frombits(i)
			if math.IsInf(f, 0) || math.IsNaN(f) || f < 0.0 {
				log.Warn("invalid value in powRequirementCode message, peer will be disconnected", "peer", p.peer.ID(), "err", err)
				return errors.New("invalid value in powRequirementCode message")
			}
			p.powRequirement = f
		case bloomFilterExCode:
			var bloom []byte
			err := packet.Decode(&bloom)
			if err == nil && len(bloom) != BloomFilterSize {
				err = fmt.Errorf("wrong bloom filter size %d", len(bloom))
			}

			if err != nil {
				log.Warn("failed to decode bloom filter exchange message, peer will be disconnected", "peer", p.peer.ID(), "err", err)
				return errors.New("invalid bloom filter exchange message")
			}
			p.setBloomFilter(bloom)
		case p2pMessageCode:
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
				whisper.postEvent(&envelope, true)
			}
		case p2pRequestCode:
//如果实现了邮件服务器，则必须进行处理。否则忽略。
			if whisper.mailServer != nil {
				var request Envelope
				if err := packet.Decode(&request); err != nil {
					log.Warn("failed to decode p2p request message, peer will be disconnected", "peer", p.peer.ID(), "err", err)
					return errors.New("invalid p2p request")
				}
				whisper.mailServer.DeliverMail(p, &request)
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
//参数isp2p指示消息是否为对等（不应转发）。
func (whisper *Whisper) add(envelope *Envelope, isP2P bool) (bool, error) {
	now := uint32(time.Now().Unix())
	sent := envelope.Expiry - envelope.TTL

	if sent > now {
		if sent-DefaultSyncAllowance > now {
			return false, fmt.Errorf("envelope created in the future [%x]", envelope.Hash())
		}
//重新计算POW，针对时差进行调整，再加上一秒钟的延迟时间
		envelope.calculatePoW(sent - now + 1)
	}

	if envelope.Expiry < now {
		if envelope.Expiry+DefaultSyncAllowance*2 < now {
			return false, fmt.Errorf("very old message")
		}
		log.Debug("expired envelope dropped", "hash", envelope.Hash().Hex())
return false, nil //不出错地丢弃信封
	}

	if uint32(envelope.size()) > whisper.MaxMessageSize() {
		return false, fmt.Errorf("huge messages are not allowed [%x]", envelope.Hash())
	}

	if envelope.PoW() < whisper.MinPow() {
//也许这个值最近发生了变化，同行们还没有调整。
//在这种情况下，以前的值由minPowTolerance（）检索
//短时间的对等同步。
		if envelope.PoW() < whisper.MinPowTolerance() {
			return false, fmt.Errorf("envelope with low PoW received: PoW=%f, hash=[%v]", envelope.PoW(), envelope.Hash().Hex())
		}
	}

	if !BloomFilterMatch(whisper.BloomFilter(), envelope.Bloom()) {
//也许这个值最近发生了变化，同行们还没有调整。
//在这种情况下，上一个值由BloomFilterTolerance（）检索
//短时间的对等同步。
		if !BloomFilterMatch(whisper.BloomFilterTolerance(), envelope.Bloom()) {
			return false, fmt.Errorf("envelope does not match bloom filter, hash=[%v], bloom: \n%x \n%x \n%x",
				envelope.Hash().Hex(), whisper.BloomFilter(), envelope.Bloom(), envelope.Topic)
		}
	}

	hash := envelope.Hash()

	whisper.poolMu.Lock()
	_, alreadyCached := whisper.envelopes[hash]
	if !alreadyCached {
		whisper.envelopes[hash] = envelope
		if whisper.expirations[envelope.Expiry] == nil {
			whisper.expirations[envelope.Expiry] = mapset.NewThreadUnsafeSet()
		}
		if !whisper.expirations[envelope.Expiry].Contains(hash) {
			whisper.expirations[envelope.Expiry].Add(hash)
		}
	}
	whisper.poolMu.Unlock()

	if alreadyCached {
		log.Trace("whisper envelope already cached", "hash", envelope.Hash().Hex())
	} else {
		log.Trace("cached whisper envelope", "hash", envelope.Hash().Hex())
		whisper.statsMu.Lock()
		whisper.stats.memoryUsed += envelope.size()
		whisper.statsMu.Unlock()
whisper.postEvent(envelope, isP2P) //将新消息通知本地节点
		if whisper.mailServer != nil {
			whisper.mailServer.Archive(envelope)
		}
	}
	return true, nil
}

//PostEvent将消息排队以进行进一步处理。
func (whisper *Whisper) postEvent(envelope *Envelope, isP2P bool) {
	if isP2P {
		whisper.p2pMsgQueue <- envelope
	} else {
		whisper.checkOverflow()
		whisper.messageQueue <- envelope
	}
}

//检查溢出检查消息队列是否发生溢出，必要时报告。
func (whisper *Whisper) checkOverflow() {
	queueSize := len(whisper.messageQueue)

	if queueSize == messageQueueLimit {
		if !whisper.Overflow() {
			whisper.settings.Store(overflowIdx, true)
			log.Warn("message queue overflow")
		}
	} else if queueSize <= messageQueueLimit/2 {
		if whisper.Overflow() {
			whisper.settings.Store(overflowIdx, false)
			log.Warn("message queue overflow fixed (back to normal)")
		}
	}
}

//processqueue在whisper节点的生命周期中将消息传递给观察者。
func (whisper *Whisper) processQueue() {
	var e *Envelope
	for {
		select {
		case <-whisper.quit:
			return

		case e = <-whisper.messageQueue:
			whisper.filters.NotifyWatchers(e, false)

		case e = <-whisper.p2pMsgQueue:
			whisper.filters.NotifyWatchers(e, true)
		}
	}
}

//更新循环，直到Whisper节点的生存期，更新其内部
//通过过期池中的过时消息来进行状态。
func (whisper *Whisper) update() {
//启动一个断续器以检查到期情况
	expire := time.NewTicker(expirationCycle)

//重复更新直到请求终止
	for {
		select {
		case <-expire.C:
			whisper.expire()

		case <-whisper.quit:
			return
		}
	}
}

//expire在所有过期时间戳上迭代，删除所有过时的
//来自池的消息。
func (whisper *Whisper) expire() {
	whisper.poolMu.Lock()
	defer whisper.poolMu.Unlock()

	whisper.statsMu.Lock()
	defer whisper.statsMu.Unlock()
	whisper.stats.reset()
	now := uint32(time.Now().Unix())
	for expiry, hashSet := range whisper.expirations {
		if expiry < now {
//转储所有过期消息并删除时间戳
			hashSet.Each(func(v interface{}) bool {
				sz := whisper.envelopes[v.(common.Hash)].size()
				delete(whisper.envelopes, v.(common.Hash))
				whisper.stats.messagesCleared++
				whisper.stats.memoryCleared += sz
				whisper.stats.memoryUsed -= sz
				return false
			})
			whisper.expirations[expiry].Clear()
			delete(whisper.expirations, expiry)
		}
	}
}

//stats返回低语节点统计信息。
func (whisper *Whisper) Stats() Statistics {
	whisper.statsMu.Lock()
	defer whisper.statsMu.Unlock()

	return whisper.stats
}

//信封检索节点当前汇集的所有消息。
func (whisper *Whisper) Envelopes() []*Envelope {
	whisper.poolMu.RLock()
	defer whisper.poolMu.RUnlock()

	all := make([]*Envelope, 0, len(whisper.envelopes))
	for _, envelope := range whisper.envelopes {
		all = append(all, envelope)
	}
	return all
}

//ISenvelopecached检查是否已接收和缓存具有特定哈希的信封。
func (whisper *Whisper) isEnvelopeCached(hash common.Hash) bool {
	whisper.poolMu.Lock()
	defer whisper.poolMu.Unlock()

	_, exist := whisper.envelopes[hash]
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

//如果数据错误或包含所有零，validatedAtainegrity将返回false，
//这是最简单也是最常见的错误。
func validateDataIntegrity(k []byte, expectedSize int) bool {
	if len(k) != expectedSize {
		return false
	}
	if expectedSize > 3 && containsOnlyZeros(k) {
		return false
	}
	return true
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

//GenerateRandomID生成一个随机字符串，然后返回该字符串用作键ID
func GenerateRandomID() (id string, err error) {
	buf, err := generateSecureRandomData(keyIDSize)
	if err != nil {
		return "", err
	}
	if !validateDataIntegrity(buf, keyIDSize) {
		return "", fmt.Errorf("error in generateRandomID: crypto/rand failed to generate random data")
	}
	id = common.Bytes2Hex(buf)
	return id, err
}

func isFullNode(bloom []byte) bool {
	if bloom == nil {
		return true
	}
	for _, b := range bloom {
		if b != 255 {
			return false
		}
	}
	return true
}

func BloomFilterMatch(filter, sample []byte) bool {
	if filter == nil {
		return true
	}

	for i := 0; i < BloomFilterSize; i++ {
		f := filter[i]
		s := sample[i]
		if (f | s) != f {
			return false
		}
	}

	return true
}

func addBloom(a, b []byte) []byte {
	c := make([]byte, BloomFilterSize)
	for i := 0; i < BloomFilterSize; i++ {
		c[i] = a[i] | b[i]
	}
	return c
}
