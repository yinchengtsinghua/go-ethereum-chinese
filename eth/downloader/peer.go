
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Go Ethereum作者
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

//包含下载程序的活动对等集，维护两个故障
//以及信誉指标，以确定块检索的优先级。

package downloader

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

const (
maxLackingHashes  = 4096 //列表中允许或缺少项的最大项数
measurementImpact = 0.1  //单个度量对对等端最终吞吐量值的影响。
)

var (
	errAlreadyFetching   = errors.New("already fetching blocks from peer")
	errAlreadyRegistered = errors.New("peer is already registered")
	errNotRegistered     = errors.New("peer is not registered")
)

//对等连接表示从中检索哈希和块的活动对等。
type peerConnection struct {
id string //对等方的唯一标识符

headerIdle  int32 //对等机的当前头活动状态（空闲=0，活动=1）
blockIdle   int32 //对等机的当前块活动状态（空闲=0，活动=1）
receiptIdle int32 //对等机的当前接收活动状态（空闲=0，活动=1）
stateIdle   int32 //对等机的当前节点数据活动状态（空闲=0，活动=1）

headerThroughput  float64 //每秒可检索的头数
blockThroughput   float64 //每秒可检索的块（体）数
receiptThroughput float64 //每秒可检索的接收数
stateThroughput   float64 //每秒可检索的节点数据块数

rtt time.Duration //请求往返时间以跟踪响应（QoS）

headerStarted  time.Time //Time instance when the last header fetch was started
blockStarted   time.Time //上次块（体）提取开始时的时间实例
receiptStarted time.Time //Time instance when the last receipt fetch was started
stateStarted   time.Time //上次节点数据提取开始时的时间实例

lacking map[common.Hash]struct{} //不请求的哈希集（以前没有）

	peer Peer

version int        //ETH协议版本号转换策略
log     log.Logger //上下文记录器，用于向对等日志添加额外信息
	lock    sync.RWMutex
}

//light peer封装了与远程light peer同步所需的方法。
type LightPeer interface {
	Head() (common.Hash, *big.Int)
	RequestHeadersByHash(common.Hash, int, int, bool) error
	RequestHeadersByNumber(uint64, int, int, bool) error
}

//对等体封装了与远程完整对等体同步所需的方法。
type Peer interface {
	LightPeer
	RequestBodies([]common.Hash) error
	RequestReceipts([]common.Hash) error
	RequestNodeData([]common.Hash) error
}

//LightPeerWrapper包装了一个LightPeer结构，删除了仅限对等的方法。
type lightPeerWrapper struct {
	peer LightPeer
}

func (w *lightPeerWrapper) Head() (common.Hash, *big.Int) { return w.peer.Head() }
func (w *lightPeerWrapper) RequestHeadersByHash(h common.Hash, amount int, skip int, reverse bool) error {
	return w.peer.RequestHeadersByHash(h, amount, skip, reverse)
}
func (w *lightPeerWrapper) RequestHeadersByNumber(i uint64, amount int, skip int, reverse bool) error {
	return w.peer.RequestHeadersByNumber(i, amount, skip, reverse)
}
func (w *lightPeerWrapper) RequestBodies([]common.Hash) error {
	panic("RequestBodies not supported in light client mode sync")
}
func (w *lightPeerWrapper) RequestReceipts([]common.Hash) error {
	panic("RequestReceipts not supported in light client mode sync")
}
func (w *lightPeerWrapper) RequestNodeData([]common.Hash) error {
	panic("RequestNodeData not supported in light client mode sync")
}

//NexPeRead创建了一个新的下载器对等体。
func newPeerConnection(id string, version int, peer Peer, logger log.Logger) *peerConnection {
	return &peerConnection{
		id:      id,
		lacking: make(map[common.Hash]struct{}),

		peer: peer,

		version: version,
		log:     logger,
	}
}

//重置清除对等实体的内部状态。
func (p *peerConnection) Reset() {
	p.lock.Lock()
	defer p.lock.Unlock()

	atomic.StoreInt32(&p.headerIdle, 0)
	atomic.StoreInt32(&p.blockIdle, 0)
	atomic.StoreInt32(&p.receiptIdle, 0)
	atomic.StoreInt32(&p.stateIdle, 0)

	p.headerThroughput = 0
	p.blockThroughput = 0
	p.receiptThroughput = 0
	p.stateThroughput = 0

	p.lacking = make(map[common.Hash]struct{})
}

//fetchheaders向远程对等端发送头检索请求。
func (p *peerConnection) FetchHeaders(from uint64, count int) error {
//健全性检查协议版本
	if p.version < 62 {
		panic(fmt.Sprintf("header fetch [eth/62+] requested on eth/%d", p.version))
	}
//如果对等机已获取，则短路
	if !atomic.CompareAndSwapInt32(&p.headerIdle, 0, 1) {
		return errAlreadyFetching
	}
	p.headerStarted = time.Now()

//发出头检索请求（绝对向上，无间隙）
	go p.peer.RequestHeadersByNumber(from, count, 0, false)

	return nil
}

//fetchbodies向远程对等端发送一个块体检索请求。
func (p *peerConnection) FetchBodies(request *fetchRequest) error {
//健全性检查协议版本
	if p.version < 62 {
		panic(fmt.Sprintf("body fetch [eth/62+] requested on eth/%d", p.version))
	}
//如果对等机已获取，则短路
	if !atomic.CompareAndSwapInt32(&p.blockIdle, 0, 1) {
		return errAlreadyFetching
	}
	p.blockStarted = time.Now()

//将标题集转换为可检索切片
	hashes := make([]common.Hash, 0, len(request.Headers))
	for _, header := range request.Headers {
		hashes = append(hashes, header.Hash())
	}
	go p.peer.RequestBodies(hashes)

	return nil
}

//FetchReceipts sends a receipt retrieval request to the remote peer.
func (p *peerConnection) FetchReceipts(request *fetchRequest) error {
//健全性检查协议版本
	if p.version < 63 {
		panic(fmt.Sprintf("body fetch [eth/63+] requested on eth/%d", p.version))
	}
//如果对等机已获取，则短路
	if !atomic.CompareAndSwapInt32(&p.receiptIdle, 0, 1) {
		return errAlreadyFetching
	}
	p.receiptStarted = time.Now()

//将标题集转换为可检索切片
	hashes := make([]common.Hash, 0, len(request.Headers))
	for _, header := range request.Headers {
		hashes = append(hashes, header.Hash())
	}
	go p.peer.RequestReceipts(hashes)

	return nil
}

//FETCHNODEDATA向远程对等体发送节点状态数据检索请求。
func (p *peerConnection) FetchNodeData(hashes []common.Hash) error {
//健全性检查协议版本
	if p.version < 63 {
		panic(fmt.Sprintf("node data fetch [eth/63+] requested on eth/%d", p.version))
	}
//如果对等机已获取，则短路
	if !atomic.CompareAndSwapInt32(&p.stateIdle, 0, 1) {
		return errAlreadyFetching
	}
	p.stateStarted = time.Now()

	go p.peer.RequestNodeData(hashes)

	return nil
}

//setheadersidle将对等机设置为空闲，允许它执行新的头检索
//请求。它的估计头检索吞吐量用测量值更新。
//刚才。
func (p *peerConnection) SetHeadersIdle(delivered int) {
	p.setIdle(p.headerStarted, delivered, &p.headerThroughput, &p.headerIdle)
}

//SetBodiesIdle sets the peer to idle, allowing it to execute block body retrieval
//请求。它的估计身体检索吞吐量是用测量值更新的。
//刚才。
func (p *peerConnection) SetBodiesIdle(delivered int) {
	p.setIdle(p.blockStarted, delivered, &p.blockThroughput, &p.blockIdle)
}

//setReceiptSidle将对等机设置为空闲，允许它执行新的接收
//检索请求。更新其估计的收据检索吞吐量
//刚刚测量的。
func (p *peerConnection) SetReceiptsIdle(delivered int) {
	p.setIdle(p.receiptStarted, delivered, &p.receiptThroughput, &p.receiptIdle)
}

//setnodedataidle将对等机设置为空闲，允许它执行新的状态trie
//数据检索请求。它的估计状态检索吞吐量被更新
//刚刚测量的。
func (p *peerConnection) SetNodeDataIdle(delivered int) {
	p.setIdle(p.stateStarted, delivered, &p.stateThroughput, &p.stateIdle)
}

//setidle将对等机设置为idle，允许它执行新的检索请求。
//它的估计检索吞吐量用刚才测量的更新。
func (p *peerConnection) setIdle(started time.Time, delivered int, throughput *float64, idle *int32) {
//与扩展无关，确保对等端最终空闲
	defer atomic.StoreInt32(idle, 0)

	p.lock.Lock()
	defer p.lock.Unlock()

//如果没有发送任何内容（硬超时/不可用数据），则将吞吐量降至最低
	if delivered == 0 {
		*throughput = 0
		return
	}
//否则，以新的测量来更新吞吐量。
elapsed := time.Since(started) + 1 //+1（ns）以确保非零除数
	measured := float64(delivered) / (float64(elapsed) / float64(time.Second))

	*throughput = (1-measurementImpact)*(*throughput) + measurementImpact*measured
	p.rtt = time.Duration((1-measurementImpact)*float64(p.rtt) + measurementImpact*float64(elapsed))

	p.log.Trace("Peer throughput measurements updated",
		"hps", p.headerThroughput, "bps", p.blockThroughput,
		"rps", p.receiptThroughput, "sps", p.stateThroughput,
		"miss", len(p.lacking), "rtt", p.rtt)
}

//HeaderCapacity根据其
//以前发现的吞吐量。
func (p *peerConnection) HeaderCapacity(targetRTT time.Duration) int {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return int(math.Min(1+math.Max(1, p.headerThroughput*float64(targetRTT)/float64(time.Second)), float64(MaxHeaderFetch)))
}

//BlockCapacity根据其
//以前发现的吞吐量。
func (p *peerConnection) BlockCapacity(targetRTT time.Duration) int {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return int(math.Min(1+math.Max(1, p.blockThroughput*float64(targetRTT)/float64(time.Second)), float64(MaxBlockFetch)))
}

//ReceiptCapacity根据其
//以前发现的吞吐量。
func (p *peerConnection) ReceiptCapacity(targetRTT time.Duration) int {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return int(math.Min(1+math.Max(1, p.receiptThroughput*float64(targetRTT)/float64(time.Second)), float64(MaxReceiptFetch)))
}

//nodeDataCapacity根据其
//以前发现的吞吐量。
func (p *peerConnection) NodeDataCapacity(targetRTT time.Duration) int {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return int(math.Min(1+math.Max(1, p.stateThroughput*float64(targetRTT)/float64(time.Second)), float64(MaxStateFetch)))
}

//MaxDebug将新实体添加到一组项目（块、收据、状态）中。
//已知某个对等机没有（即之前已被请求）。如果
//集合达到其最大允许容量，项目被随机丢弃。
func (p *peerConnection) MarkLacking(hash common.Hash) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for len(p.lacking) >= maxLackingHashes {
		for drop := range p.lacking {
			delete(p.lacking, drop)
			break
		}
	}
	p.lacking[hash] = struct{}{}
}

//缺少检索区块链项目的哈希是否在缺少的对等项上
//列出（即，我们是否知道同伴没有它）。
func (p *peerConnection) Lacks(hash common.Hash) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()

	_, ok := p.lacking[hash]
	return ok
}

//peerSet represents the collection of active peer participating in the chain
//下载过程。
type peerSet struct {
	peers        map[string]*peerConnection
	newPeerFeed  event.Feed
	peerDropFeed event.Feed
	lock         sync.RWMutex
}

//new peer set创建一个新的peer set top跟踪活动的下载源。
func newPeerSet() *peerSet {
	return &peerSet{
		peers: make(map[string]*peerConnection),
	}
}

//订阅方订阅对等到达事件。
func (ps *peerSet) SubscribeNewPeers(ch chan<- *peerConnection) event.Subscription {
	return ps.newPeerFeed.Subscribe(ch)
}

//订阅对等删除订阅对等离开事件。
func (ps *peerSet) SubscribePeerDrops(ch chan<- *peerConnection) event.Subscription {
	return ps.peerDropFeed.Subscribe(ch)
}

//重置迭代当前对等集，并重置每个已知对等
//为下一批块检索做准备。
func (ps *peerSet) Reset() {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	for _, peer := range ps.peers {
		peer.Reset()
	}
}

//寄存器向工作集中注入一个新的对等点，或者返回一个错误，如果
//对等机已经知道。
//
//该方法还将新对等机的起始吞吐量值设置为
//对所有现有同龄人的平均数，以使其有实际的使用机会
//用于数据检索。
func (ps *peerSet) Register(p *peerConnection) error {
//检索当前中间值RTT作为健全的默认值
	p.rtt = ps.medianRTT()

//用一些有意义的默认值注册新的对等机
	ps.lock.Lock()
	if _, ok := ps.peers[p.id]; ok {
		ps.lock.Unlock()
		return errAlreadyRegistered
	}
	if len(ps.peers) > 0 {
		p.headerThroughput, p.blockThroughput, p.receiptThroughput, p.stateThroughput = 0, 0, 0, 0

		for _, peer := range ps.peers {
			peer.lock.RLock()
			p.headerThroughput += peer.headerThroughput
			p.blockThroughput += peer.blockThroughput
			p.receiptThroughput += peer.receiptThroughput
			p.stateThroughput += peer.stateThroughput
			peer.lock.RUnlock()
		}
		p.headerThroughput /= float64(len(ps.peers))
		p.blockThroughput /= float64(len(ps.peers))
		p.receiptThroughput /= float64(len(ps.peers))
		p.stateThroughput /= float64(len(ps.peers))
	}
	ps.peers[p.id] = p
	ps.lock.Unlock()

	ps.newPeerFeed.Send(p)
	return nil
}

//注销从活动集删除远程对等，进一步禁用
//对该特定实体采取的行动。
func (ps *peerSet) Unregister(id string) error {
	ps.lock.Lock()
	p, ok := ps.peers[id]
	if !ok {
		defer ps.lock.Unlock()
		return errNotRegistered
	}
	delete(ps.peers, id)
	ps.lock.Unlock()

	ps.peerDropFeed.Send(p)
	return nil
}

//对等端检索具有给定ID的注册对等端。
func (ps *peerSet) Peer(id string) *peerConnection {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.peers[id]
}

//len返回集合中当前的对等数。
func (ps *peerSet) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return len(ps.peers)
}

//Allpeers检索集合中所有对等方的简单列表。
func (ps *peerSet) AllPeers() []*peerConnection {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*peerConnection, 0, len(ps.peers))
	for _, p := range ps.peers {
		list = append(list, p)
	}
	return list
}

//HeaderIdlePeers检索当前所有头空闲对等的简单列表
//在活动对等集内，按其声誉排序。
func (ps *peerSet) HeaderIdlePeers() ([]*peerConnection, int) {
	idle := func(p *peerConnection) bool {
		return atomic.LoadInt32(&p.headerIdle) == 0
	}
	throughput := func(p *peerConnection) float64 {
		p.lock.RLock()
		defer p.lock.RUnlock()
		return p.headerThroughput
	}
	return ps.idlePeers(62, 64, idle, throughput)
}

//BodyIdlePeers检索当前位于
//按其声誉排序的活动对等集。
func (ps *peerSet) BodyIdlePeers() ([]*peerConnection, int) {
	idle := func(p *peerConnection) bool {
		return atomic.LoadInt32(&p.blockIdle) == 0
	}
	throughput := func(p *peerConnection) float64 {
		p.lock.RLock()
		defer p.lock.RUnlock()
		return p.blockThroughput
	}
	return ps.idlePeers(62, 64, idle, throughput)
}

//ReceiptIdlePeers检索当前所有接收空闲对等的简单列表
//在活动对等集内，按其声誉排序。
func (ps *peerSet) ReceiptIdlePeers() ([]*peerConnection, int) {
	idle := func(p *peerConnection) bool {
		return atomic.LoadInt32(&p.receiptIdle) == 0
	}
	throughput := func(p *peerConnection) float64 {
		p.lock.RLock()
		defer p.lock.RUnlock()
		return p.receiptThroughput
	}
	return ps.idlePeers(63, 64, idle, throughput)
}

//nodedataidlepeers检索当前所有空闲节点数据的简单列表
//活动对等集内的对等点，按其声誉排序。
func (ps *peerSet) NodeDataIdlePeers() ([]*peerConnection, int) {
	idle := func(p *peerConnection) bool {
		return atomic.LoadInt32(&p.stateIdle) == 0
	}
	throughput := func(p *peerConnection) float64 {
		p.lock.RLock()
		defer p.lock.RUnlock()
		return p.stateThroughput
	}
	return ps.idlePeers(63, 64, idle, throughput)
}

//idle peers检索当前满足
//协议版本约束，使用提供的函数检查空闲。
//由此产生的一组对等机按其度量吞吐量进行排序。
func (ps *peerSet) idlePeers(minProtocol, maxProtocol int, idleCheck func(*peerConnection) bool, throughput func(*peerConnection) float64) ([]*peerConnection, int) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	idle, total := make([]*peerConnection, 0, len(ps.peers)), 0
	for _, p := range ps.peers {
		if p.version >= minProtocol && p.version <= maxProtocol {
			if idleCheck(p) {
				idle = append(idle, p)
			}
			total++
		}
	}
	for i := 0; i < len(idle); i++ {
		for j := i + 1; j < len(idle); j++ {
			if throughput(idle[i]) < throughput(idle[j]) {
				idle[i], idle[j] = idle[j], idle[i]
			}
		}
	}
	return idle, total
}

//MediaNRTT返回对等集的中间RTT，只考虑调优
//如果有更多可用的对等机，则为对等机。
func (ps *peerSet) medianRTT() time.Duration {
//Gather all the currently measured round trip times
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	rtts := make([]float64, 0, len(ps.peers))
	for _, p := range ps.peers {
		p.lock.RLock()
		rtts = append(rtts, float64(p.rtt))
		p.lock.RUnlock()
	}
	sort.Float64s(rtts)

	median := rttMaxEstimate
	if qosTuningPeers <= len(rtts) {
median = time.Duration(rtts[qosTuningPeers/2]) //调优同行的中位数
	} else if len(rtts) > 0 {
median = time.Duration(rtts[len(rtts)/2]) //我们连接的对等点的中位数（甚至保持一些基线QoS）
	}
//Restrict the RTT into some QoS defaults, irrelevant of true RTT
	if median < rttMinEstimate {
		median = rttMinEstimate
	}
	if median > rttMaxEstimate {
		median = rttMaxEstimate
	}
	return median
}
