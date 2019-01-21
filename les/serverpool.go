
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

//包les实现轻以太坊子协议。
package les

import (
	"crypto/ecdsa"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
//连接结束或超时后，会有一段等待时间
//才能再次选择连接。
//等待时间=基本延迟*（1+随机（1））
//基本延迟=在a之后的第一个shortretrycnt时间的shortretryplay
//连接成功，在应用longretryplay之后
	shortRetryCnt   = 5
	shortRetryDelay = time.Second * 5
	longRetryDelay  = time.Minute * 10
//MaxNewEntries是新发现（从未连接）节点的最大数目。
//如果达到了这个限度，那么最近发现的最少的一个就会被剔除。
	maxNewEntries = 1000
//MaxKnownEntries是已知（已连接）节点的最大数目。
//如果达到了限制，则会丢弃最近连接的限制。
//（与新条目不同的是，已知条目是持久的）
	maxKnownEntries = 1000
//同时连接的服务器的目标
	targetServerCount = 5
//从已知表中选择的服务器的目标
//（如果有新的，我们留有试用的空间）
	targetKnownSelect = 3
//拨号超时后，考虑服务器不可用并调整统计信息
	dialTimeout = time.Second * 30
//TargetConntime是服务器之前的最小预期连接持续时间
//无任何特定原因删除客户端
	targetConnTime = time.Minute * 10
//基于最新发现时间的新条目选择权重计算：
//unity until discoverExpireStart, then exponential decay with discoverExpireConst
	discoverExpireStart = time.Minute * 20
	discoverExpireConst = time.Minute * 20
//已知条目选择权重在
//每次连接失败（成功后恢复）
	failDropLn = 0.1
//已知节点连接成功和质量统计数据具有长期平均值
//以及一个以指数形式调整的短期值，其系数为
//pstatrecentadjust与每个拨号/连接同时以指数方式返回
//到时间常数pstatornetomantc的平均值
	pstatReturnToMeanTC = time.Hour
//节点地址选择权重在
//每次连接失败（成功后恢复）
	addrFailDropLn = math.Ln2
//响应coretc和delayscoretc是
//根据响应时间和块延迟时间计算选择机会
	responseScoreTC = time.Millisecond * 100
	delayScoreTC    = time.Second * 5
	timeoutPow      = 10
//initstatsweight用于初始化以前未知的具有良好
//统计学给自己一个证明自己的机会
	initStatsWeight = 1
)

//connreq表示对等连接请求。
type connReq struct {
	p      *peer
	node   *enode.Node
	result chan *poolEntry
}

//disconnreq表示对等端断开连接的请求。
type disconnReq struct {
	entry   *poolEntry
	stopped bool
	done    chan struct{}
}

//registerreq表示对等注册请求。
type registerReq struct {
	entry *poolEntry
	done  chan struct{}
}

//ServerPool实现用于存储和选择新发现的
//已知的轻型服务器节点。它接收发现的节点，存储关于
//已知节点，并始终保持足够高质量的服务器连接。
type serverPool struct {
	db     ethdb.Database
	dbKey  []byte
	server *p2p.Server
	quit   chan struct{}
	wg     *sync.WaitGroup
	connWg sync.WaitGroup

	topic discv5.Topic

	discSetPeriod chan time.Duration
	discNodes     chan *enode.Node
	discLookups   chan bool

	entries              map[enode.ID]*poolEntry
	timeout, enableRetry chan *poolEntry
	adjustStats          chan poolStatAdjust

	connCh     chan *connReq
	disconnCh  chan *disconnReq
	registerCh chan *registerReq

	knownQueue, newQueue       poolEntryQueue
	knownSelect, newSelect     *weightedRandomSelect
	knownSelected, newSelected int
	fastDiscover               bool
}

//NewServerPool创建新的ServerPool实例
func newServerPool(db ethdb.Database, quit chan struct{}, wg *sync.WaitGroup) *serverPool {
	pool := &serverPool{
		db:           db,
		quit:         quit,
		wg:           wg,
		entries:      make(map[enode.ID]*poolEntry),
		timeout:      make(chan *poolEntry, 1),
		adjustStats:  make(chan poolStatAdjust, 100),
		enableRetry:  make(chan *poolEntry, 1),
		connCh:       make(chan *connReq),
		disconnCh:    make(chan *disconnReq),
		registerCh:   make(chan *registerReq),
		knownSelect:  newWeightedRandomSelect(),
		newSelect:    newWeightedRandomSelect(),
		fastDiscover: true,
	}
	pool.knownQueue = newPoolEntryQueue(maxKnownEntries, pool.removeEntry)
	pool.newQueue = newPoolEntryQueue(maxNewEntries, pool.removeEntry)
	return pool
}

func (pool *serverPool) start(server *p2p.Server, topic discv5.Topic) {
	pool.server = server
	pool.topic = topic
	pool.dbKey = append([]byte("serverPool/"), []byte(topic)...)
	pool.wg.Add(1)
	pool.loadNodes()

	if pool.server.DiscV5 != nil {
		pool.discSetPeriod = make(chan time.Duration, 1)
		pool.discNodes = make(chan *enode.Node, 100)
		pool.discLookups = make(chan bool, 100)
		go pool.discoverNodes()
	}
	pool.checkDial()
	go pool.eventLoop()
}

//discovernodes包装搜索主题，将结果节点转换为enode.node。
func (pool *serverPool) discoverNodes() {
	ch := make(chan *discv5.Node)
	go func() {
		pool.server.DiscV5.SearchTopic(pool.topic, pool.discSetPeriod, ch, pool.discLookups)
		close(ch)
	}()
	for n := range ch {
		pubkey, err := decodePubkey64(n.ID[:])
		if err != nil {
			continue
		}
		pool.discNodes <- enode.NewV4(pubkey, n.IP, int(n.TCP), int(n.UDP))
	}
}

//任何传入连接都应调用Connect。如果连接
//最近由服务器池拨号，返回相应的池条目。
//否则，应拒绝连接。
//请注意，无论何时接受连接并返回池条目，
//也应始终调用断开连接。
func (pool *serverPool) connect(p *peer, node *enode.Node) *poolEntry {
	log.Debug("Connect new entry", "enode", p.id)
	req := &connReq{p: p, node: node, result: make(chan *poolEntry, 1)}
	select {
	case pool.connCh <- req:
	case <-pool.quit:
		return nil
	}
	return <-req.result
}

//应在成功握手后调用已注册
func (pool *serverPool) registered(entry *poolEntry) {
	log.Debug("Registered new entry", "enode", entry.node.ID())
	req := &registerReq{entry: entry, done: make(chan struct{})}
	select {
	case pool.registerCh <- req:
	case <-pool.quit:
		return
	}
	<-req.done
}

//结束连接时应调用Disconnect。服务质量统计
//可以选择更新（在这种情况下，如果没有注册，则不更新
//只更新连接统计信息，就像在超时情况下一样）
func (pool *serverPool) disconnect(entry *poolEntry) {
	stopped := false
	select {
	case <-pool.quit:
		stopped = true
	default:
	}
	log.Debug("Disconnected old entry", "enode", entry.node.ID())
	req := &disconnReq{entry: entry, stopped: stopped, done: make(chan struct{})}

//阻止，直到断开请求被送达。
	pool.disconnCh <- req
	<-req.done
}

const (
	pseBlockDelay = iota
	pseResponseTime
	pseResponseTimeout
)

//poolStatAdjust records are sent to adjust peer block delay/response time statistics
type poolStatAdjust struct {
	adjustType int
	entry      *poolEntry
	time       time.Duration
}

//AdjustBlockDelay调整节点的块公告延迟统计信息
func (pool *serverPool) adjustBlockDelay(entry *poolEntry, time time.Duration) {
	if entry == nil {
		return
	}
	pool.adjustStats <- poolStatAdjust{pseBlockDelay, entry, time}
}

//AdjusteResponseTime调整节点的请求响应时间统计信息
func (pool *serverPool) adjustResponseTime(entry *poolEntry, time time.Duration, timeout bool) {
	if entry == nil {
		return
	}
	if timeout {
		pool.adjustStats <- poolStatAdjust{pseResponseTimeout, entry, time}
	} else {
		pool.adjustStats <- poolStatAdjust{pseResponseTime, entry, time}
	}
}

//事件循环处理池事件和所有内部函数的互斥锁
func (pool *serverPool) eventLoop() {
	lookupCnt := 0
	var convTime mclock.AbsTime
	if pool.discSetPeriod != nil {
		pool.discSetPeriod <- time.Millisecond * 100
	}

//根据连接时间断开连接更新服务质量统计信息
//以及断开启动器。
	disconnect := func(req *disconnReq, stopped bool) {
//处理对等端断开请求。
		entry := req.entry
		if entry.state == psRegistered {
			connAdjust := float64(mclock.Now()-entry.regTime) / float64(targetConnTime)
			if connAdjust > 1 {
				connAdjust = 1
			}
			if stopped {
//我们要求断开连接。
				entry.connectStats.add(1, connAdjust)
			} else {
//disconnect requested by server side.
				entry.connectStats.add(connAdjust, 1)
			}
		}
		entry.state = psNotConnected

		if entry.knownSelected {
			pool.knownSelected--
		} else {
			pool.newSelected--
		}
		pool.setRetryDial(entry)
		pool.connWg.Done()
		close(req.done)
	}

	for {
		select {
		case entry := <-pool.timeout:
			if !entry.removed {
				pool.checkDialTimeout(entry)
			}

		case entry := <-pool.enableRetry:
			if !entry.removed {
				entry.delayedRetry = false
				pool.updateCheckDial(entry)
			}

		case adj := <-pool.adjustStats:
			switch adj.adjustType {
			case pseBlockDelay:
				adj.entry.delayStats.add(float64(adj.time), 1)
			case pseResponseTime:
				adj.entry.responseStats.add(float64(adj.time), 1)
				adj.entry.timeoutStats.add(0, 1)
			case pseResponseTimeout:
				adj.entry.timeoutStats.add(1, 1)
			}

		case node := <-pool.discNodes:
			entry := pool.findOrNewNode(node)
			pool.updateCheckDial(entry)

		case conv := <-pool.discLookups:
			if conv {
				if lookupCnt == 0 {
					convTime = mclock.Now()
				}
				lookupCnt++
				if pool.fastDiscover && (lookupCnt == 50 || time.Duration(mclock.Now()-convTime) > time.Minute) {
					pool.fastDiscover = false
					if pool.discSetPeriod != nil {
						pool.discSetPeriod <- time.Minute
					}
				}
			}

		case req := <-pool.connCh:
//处理对等连接请求。
			entry := pool.entries[req.p.ID()]
			if entry == nil {
				entry = pool.findOrNewNode(req.node)
			}
			if entry.state == psConnected || entry.state == psRegistered {
				req.result <- nil
				continue
			}
			pool.connWg.Add(1)
			entry.peer = req.p
			entry.state = psConnected
			addr := &poolEntryAddress{
				ip:       req.node.IP(),
				port:     uint16(req.node.TCP()),
				lastSeen: mclock.Now(),
			}
			entry.lastConnected = addr
			entry.addr = make(map[string]*poolEntryAddress)
			entry.addr[addr.strKey()] = addr
			entry.addrSelect = *newWeightedRandomSelect()
			entry.addrSelect.update(addr)
			req.result <- entry

		case req := <-pool.registerCh:
//处理对等注册请求。
			entry := req.entry
			entry.state = psRegistered
			entry.regTime = mclock.Now()
			if !entry.known {
				pool.newQueue.remove(entry)
				entry.known = true
			}
			pool.knownQueue.setLatest(entry)
			entry.shortRetry = shortRetryCnt
			close(req.done)

		case req := <-pool.disconnCh:
//处理对等端断开请求。
			disconnect(req, req.stopped)

		case <-pool.quit:
			if pool.discSetPeriod != nil {
				close(pool.discSetPeriod)
			}

//在断开所有连接后，生成一个goroutine以关闭断开连接。
			go func() {
				pool.connWg.Wait()
				close(pool.disconnCh)
			}()

//退出前处理所有剩余的断开请求。
			for req := range pool.disconnCh {
				disconnect(req, true)
			}
			pool.saveNodes()
			pool.wg.Done()
			return
		}
	}
}

func (pool *serverPool) findOrNewNode(node *enode.Node) *poolEntry {
	now := mclock.Now()
	entry := pool.entries[node.ID()]
	if entry == nil {
		log.Debug("Discovered new entry", "id", node.ID())
		entry = &poolEntry{
			node:       node,
			addr:       make(map[string]*poolEntryAddress),
			addrSelect: *newWeightedRandomSelect(),
			shortRetry: shortRetryCnt,
		}
		pool.entries[node.ID()] = entry
//用良好的统计数据初始化以前未知的对等点，以提供证明自己的机会
		entry.connectStats.add(1, initStatsWeight)
		entry.delayStats.add(0, initStatsWeight)
		entry.responseStats.add(0, initStatsWeight)
		entry.timeoutStats.add(0, initStatsWeight)
	}
	entry.lastDiscovered = now
	addr := &poolEntryAddress{ip: node.IP(), port: uint16(node.TCP())}
	if a, ok := entry.addr[addr.strKey()]; ok {
		addr = a
	} else {
		entry.addr[addr.strKey()] = addr
	}
	addr.lastSeen = now
	entry.addrSelect.update(addr)
	if !entry.known {
		pool.newQueue.setLatest(entry)
	}
	return entry
}

//loadNodes从数据库加载已知节点及其统计信息
func (pool *serverPool) loadNodes() {
	enc, err := pool.db.Get(pool.dbKey)
	if err != nil {
		return
	}
	var list []*poolEntry
	err = rlp.DecodeBytes(enc, &list)
	if err != nil {
		log.Debug("Failed to decode node list", "err", err)
		return
	}
	for _, e := range list {
		log.Debug("Loaded server stats", "id", e.node.ID(), "fails", e.lastConnected.fails,
			"conn", fmt.Sprintf("%v/%v", e.connectStats.avg, e.connectStats.weight),
			"delay", fmt.Sprintf("%v/%v", time.Duration(e.delayStats.avg), e.delayStats.weight),
			"response", fmt.Sprintf("%v/%v", time.Duration(e.responseStats.avg), e.responseStats.weight),
			"timeout", fmt.Sprintf("%v/%v", e.timeoutStats.avg, e.timeoutStats.weight))
		pool.entries[e.node.ID()] = e
		pool.knownQueue.setLatest(e)
		pool.knownSelect.update((*knownEntry)(e))
	}
}

//savenodes将已知节点及其统计信息保存到数据库中。节点是
//从最少订购到最近连接。
func (pool *serverPool) saveNodes() {
	list := make([]*poolEntry, len(pool.knownQueue.queue))
	for i := range list {
		list[i] = pool.knownQueue.fetchOldest()
	}
	enc, err := rlp.EncodeToBytes(list)
	if err == nil {
		pool.db.Put(pool.dbKey, enc)
	}
}

//当达到项计数限制时，removeentry将删除池项。
//请注意，它是由新的/已知的队列调用的，该条目已经从这些队列中
//已删除，因此不需要将其从队列中删除。
func (pool *serverPool) removeEntry(entry *poolEntry) {
	pool.newSelect.remove((*discoveredEntry)(entry))
	pool.knownSelect.remove((*knownEntry)(entry))
	entry.removed = true
	delete(pool.entries, entry.node.ID())
}

//setretrydial启动计时器，该计时器将再次启用拨号某个节点
func (pool *serverPool) setRetryDial(entry *poolEntry) {
	delay := longRetryDelay
	if entry.shortRetry > 0 {
		entry.shortRetry--
		delay = shortRetryDelay
	}
	delay += time.Duration(rand.Int63n(int64(delay) + 1))
	entry.delayedRetry = true
	go func() {
		select {
		case <-pool.quit:
		case <-time.After(delay):
			select {
			case <-pool.quit:
			case pool.enableRetry <- entry:
			}
		}
	}()
}

//当一个条目可能再次拨号时，调用updateCheckDial。资讯科技更新
//它的选择权重和检查是否可以/应该进行新的拨号。
func (pool *serverPool) updateCheckDial(entry *poolEntry) {
	pool.newSelect.update((*discoveredEntry)(entry))
	pool.knownSelect.update((*knownEntry)(entry))
	pool.checkDial()
}

//checkDial checks if new dials can/should be made. It tries to select servers both
//基于良好的统计数据和最近的发现。
func (pool *serverPool) checkDial() {
	fillWithKnownSelects := !pool.fastDiscover
	for pool.knownSelected < targetKnownSelect {
		entry := pool.knownSelect.choose()
		if entry == nil {
			fillWithKnownSelects = false
			break
		}
		pool.dial((*poolEntry)(entry.(*knownEntry)), true)
	}
	for pool.knownSelected+pool.newSelected < targetServerCount {
		entry := pool.newSelect.choose()
		if entry == nil {
			break
		}
		pool.dial((*poolEntry)(entry.(*discoveredEntry)), false)
	}
	if fillWithKnownSelects {
//没有新发现的节点可供选择，并且自快速发现阶段以来
//结束了，我们可能在不久的将来找不到更多，所以选择更多
//已知条目（如果可能）
		for pool.knownSelected < targetServerCount {
			entry := pool.knownSelect.choose()
			if entry == nil {
				break
			}
			pool.dial((*poolEntry)(entry.(*knownEntry)), true)
		}
	}
}

//拨号启动新连接
func (pool *serverPool) dial(entry *poolEntry, knownSelected bool) {
	if pool.server == nil || entry.state != psNotConnected {
		return
	}
	entry.state = psDialed
	entry.knownSelected = knownSelected
	if knownSelected {
		pool.knownSelected++
	} else {
		pool.newSelected++
	}
	addr := entry.addrSelect.choose().(*poolEntryAddress)
	log.Debug("Dialing new peer", "lesaddr", entry.node.ID().String()+"@"+addr.strKey(), "set", len(entry.addr), "known", knownSelected)
	entry.dialed = addr
	go func() {
		pool.server.AddPeer(entry.node)
		select {
		case <-pool.quit:
		case <-time.After(dialTimeout):
			select {
			case <-pool.quit:
			case pool.timeout <- entry:
			}
		}
	}()
}

//CheckDialTimeout检查节点是否仍处于拨号状态，如果仍然处于拨号状态，则将其重置。
//并相应地调整连接统计。
func (pool *serverPool) checkDialTimeout(entry *poolEntry) {
	if entry.state != psDialed {
		return
	}
	log.Debug("Dial timeout", "lesaddr", entry.node.ID().String()+"@"+entry.dialed.strKey())
	entry.state = psNotConnected
	if entry.knownSelected {
		pool.knownSelected--
	} else {
		pool.newSelected--
	}
	entry.connectStats.add(0, 1)
	entry.dialed.fails++
	pool.setRetryDial(entry)
}

const (
	psNotConnected = iota
	psDialed
	psConnected
	psRegistered
)

//Poolentry表示服务器节点，并存储其当前状态和统计信息。
type poolEntry struct {
	peer                  *peer
pubkey                [64]byte //secp256k1节点密钥
	addr                  map[string]*poolEntryAddress
	node                  *enode.Node
	lastConnected, dialed *poolEntryAddress
	addrSelect            weightedRandomSelect

	lastDiscovered              mclock.AbsTime
	known, knownSelected        bool
	connectStats, delayStats    poolStats
	responseStats, timeoutStats poolStats
	state                       int
	regTime                     mclock.AbsTime
	queueIdx                    int
	removed                     bool

	delayedRetry bool
	shortRetry   int
}

//poolentryenc是poolentry的rlp编码。
type poolEntryEnc struct {
	Pubkey                     []byte
	IP                         net.IP
	Port                       uint16
	Fails                      uint
	CStat, DStat, RStat, TStat poolStats
}

func (e *poolEntry) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &poolEntryEnc{
		Pubkey: encodePubkey64(e.node.Pubkey()),
		IP:     e.lastConnected.ip,
		Port:   e.lastConnected.port,
		Fails:  e.lastConnected.fails,
		CStat:  e.connectStats,
		DStat:  e.delayStats,
		RStat:  e.responseStats,
		TStat:  e.timeoutStats,
	})
}

func (e *poolEntry) DecodeRLP(s *rlp.Stream) error {
	var entry poolEntryEnc
	if err := s.Decode(&entry); err != nil {
		return err
	}
	pubkey, err := decodePubkey64(entry.Pubkey)
	if err != nil {
		return err
	}
	addr := &poolEntryAddress{ip: entry.IP, port: entry.Port, fails: entry.Fails, lastSeen: mclock.Now()}
	e.node = enode.NewV4(pubkey, entry.IP, int(entry.Port), int(entry.Port))
	e.addr = make(map[string]*poolEntryAddress)
	e.addr[addr.strKey()] = addr
	e.addrSelect = *newWeightedRandomSelect()
	e.addrSelect.update(addr)
	e.lastConnected = addr
	e.connectStats = entry.CStat
	e.delayStats = entry.DStat
	e.responseStats = entry.RStat
	e.timeoutStats = entry.TStat
	e.shortRetry = shortRetryCnt
	e.known = true
	return nil
}

func encodePubkey64(pub *ecdsa.PublicKey) []byte {
	return crypto.FromECDSAPub(pub)[1:]
}

func decodePubkey64(b []byte) (*ecdsa.PublicKey, error) {
	return crypto.UnmarshalPubkey(append([]byte{0x04}, b...))
}

//DiscoveredEntry实现WRSitem
type discoveredEntry poolEntry

//权重为新发现的条目计算随机选择权重
func (e *discoveredEntry) Weight() int64 {
	if e.state != psNotConnected || e.delayedRetry {
		return 0
	}
	t := time.Duration(mclock.Now() - e.lastDiscovered)
	if t <= discoverExpireStart {
		return 1000000000
	}
	return int64(1000000000 * math.Exp(-float64(t-discoverExpireStart)/float64(discoverExpireConst)))
}

//知识工具
type knownEntry poolEntry

//权重计算已知条目的随机选择权重
func (e *knownEntry) Weight() int64 {
	if e.state != psNotConnected || !e.known || e.delayedRetry {
		return 0
	}
	return int64(1000000000 * e.connectStats.recentAvg() * math.Exp(-float64(e.lastConnected.fails)*failDropLn-e.responseStats.recentAvg()/float64(responseScoreTC)-e.delayStats.recentAvg()/float64(delayScoreTC)) * math.Pow(1-e.timeoutStats.recentAvg(), timeoutPow))
}

//PoolentryAddress是一个单独的对象，因为当前需要记住
//池项的多个潜在网络地址。这将在
//v5发现的最终实现，它将检索签名和序列
//编号的广告，使其明确哪个IP/端口是最新的。
type poolEntryAddress struct {
	ip       net.IP
	port     uint16
lastSeen mclock.AbsTime //上次从数据库发现、连接或加载它时
fails    uint           //自上次成功连接以来的连接失败（持久）
}

func (a *poolEntryAddress) Weight() int64 {
	t := time.Duration(mclock.Now() - a.lastSeen)
	return int64(1000000*math.Exp(-float64(t)/float64(discoverExpireConst)-float64(a.fails)*addrFailDropLn)) + 1
}

func (a *poolEntryAddress) strKey() string {
	return a.ip.String() + ":" + strconv.Itoa(int(a.port))
}

//poolstats使用长期平均值对特定数量进行统计
//以及一个以指数形式调整的短期值，其系数为
//pstatrecentadjust与每个更新同时以指数形式返回到
//时间常数pstatorntomeantc的平均值
type poolStats struct {
	sum, weight, avg, recent float64
	lastRecalc               mclock.AbsTime
}

//init使用从数据库中检索到的长期sum/update count对初始化统计信息
func (s *poolStats) init(sum, weight float64) {
	s.sum = sum
	s.weight = weight
	var avg float64
	if weight > 0 {
		avg = s.sum / weight
	}
	s.avg = avg
	s.recent = avg
	s.lastRecalc = mclock.Now()
}

//重新计算近期值返回平均值和长期平均值
func (s *poolStats) recalc() {
	now := mclock.Now()
	s.recent = s.avg + (s.recent-s.avg)*math.Exp(-float64(now-s.lastRecalc)/float64(pstatReturnToMeanTC))
	if s.sum == 0 {
		s.avg = 0
	} else {
		if s.sum > s.weight*1e30 {
			s.avg = 1e30
		} else {
			s.avg = s.sum / s.weight
		}
	}
	s.lastRecalc = now
}

//添加用新值更新统计信息
func (s *poolStats) add(value, weight float64) {
	s.weight += weight
	s.sum += value * weight
	s.recalc()
}

//recentavg返回短期调整平均值
func (s *poolStats) recentAvg() float64 {
	s.recalc()
	return s.recent
}

func (s *poolStats) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{math.Float64bits(s.sum), math.Float64bits(s.weight)})
}

func (s *poolStats) DecodeRLP(st *rlp.Stream) error {
	var stats struct {
		SumUint, WeightUint uint64
	}
	if err := st.Decode(&stats); err != nil {
		return err
	}
	s.init(math.Float64frombits(stats.SumUint), math.Float64frombits(stats.WeightUint))
	return nil
}

//PoolentryQueue跟踪其最近访问次数最少的条目并删除
//当条目数达到限制时
type poolEntryQueue struct {
queue                  map[int]*poolEntry //known nodes indexed by their latest lastConnCnt value
	newPtr, oldPtr, maxCnt int
	removeFromPool         func(*poolEntry)
}

//NewPoolentryQueue返回新的PoolentryQueue
func newPoolEntryQueue(maxCnt int, removeFromPool func(*poolEntry)) poolEntryQueue {
	return poolEntryQueue{queue: make(map[int]*poolEntry), maxCnt: maxCnt, removeFromPool: removeFromPool}
}

//fetcholdst返回并删除最近访问次数最少的条目
func (q *poolEntryQueue) fetchOldest() *poolEntry {
	if len(q.queue) == 0 {
		return nil
	}
	for {
		if e := q.queue[q.oldPtr]; e != nil {
			delete(q.queue, q.oldPtr)
			q.oldPtr++
			return e
		}
		q.oldPtr++
	}
}

//删除从队列中删除一个条目
func (q *poolEntryQueue) remove(entry *poolEntry) {
	if q.queue[entry.queueIdx] == entry {
		delete(q.queue, entry.queueIdx)
	}
}

//setlatest添加或更新最近访问的条目。它还检查旧条目
//需要移除，并用回调函数从父池中移除它。
func (q *poolEntryQueue) setLatest(entry *poolEntry) {
	if q.queue[entry.queueIdx] == entry {
		delete(q.queue, entry.queueIdx)
	} else {
		if len(q.queue) == q.maxCnt {
			e := q.fetchOldest()
			q.remove(e)
			q.removeFromPool(e)
		}
	}
	entry.queueIdx = q.newPtr
	q.queue[entry.queueIdx] = entry
	q.newPtr++
}
