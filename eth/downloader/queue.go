
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

//Contains the block download scheduler to collect download tasks and schedule
//他们井然有序，步履维艰。

package downloader

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/prque"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

var (
blockCacheItems      = 8192             //限制下载前要缓存的最大块数
blockCacheMemory     = 64 * 1024 * 1024 //用于块缓存的最大内存量
blockCacheSizeWeight = 0.1              //乘数，根据过去的块大小近似平均块大小
)

var (
	errNoFetchesPending = errors.New("no fetches pending")
	errStaleDelivery    = errors.New("stale delivery")
)

//FetchRequest是当前正在运行的数据检索操作。
type fetchRequest struct {
Peer    *peerConnection //请求发送到的对等机
From    uint64          //[ETH/62]请求的链元素索引（仅用于骨架填充）
Headers []*types.Header //[ETH/62]请求的标题，按请求顺序排序
Time    time.Time       //提出请求的时间
}

//fetchresult是一个结构，从数据获取程序收集部分结果，直到
//所有未完成的部分都已完成，结果作为一个整体可以处理。
type fetchResult struct {
Pending int         //仍挂起的数据提取数
Hash    common.Hash //阻止重新计算的头的哈希

	Header       *types.Header
	Uncles       []*types.Header
	Transactions types.Transactions
	Receipts     types.Receipts
}

//队列表示需要获取或正在获取的哈希
type queue struct {
mode SyncMode //同步模式，用于确定要计划取件的块零件

//头是“特殊的”，它们批量下载，由一个框架链支持。
headerHead      common.Hash                    //[eth/62]验证顺序的最后一个排队头的哈希
headerTaskPool  map[uint64]*types.Header       //[ETH/62]挂起的头检索任务，将开始索引映射到骨架头
headerTaskQueue *prque.Prque                   //[eth/62]获取填充头的骨架索引的优先级队列
headerPeerMiss  map[string]map[uint64]struct{} //[eth/62]已知不可用的每对等头批的集合
headerPendPool  map[string]*fetchRequest       //[ETH/62]当前挂起的头检索操作
headerResults   []*types.Header                //[ETH/62]结果缓存累积已完成的头段
headerProced    int                            //[ETH／62 ]已从结果中处理的页眉数
headerOffset    uint64                         //[eth/62]结果缓存中第一个头的编号
headerContCh    chan bool                      //[ETH/62]当头下载完成时通知的通道

//下面的所有数据检索都基于已组装的头链
blockTaskPool  map[common.Hash]*types.Header //[ETH/62]挂起的块（体）检索任务，将哈希映射到头
blockTaskQueue *prque.Prque                  //[eth/62]头的优先级队列，用于获取块（体）
blockPendPool  map[string]*fetchRequest      //[ETH/62]当前正在等待的块（体）检索操作
blockDonePool  map[common.Hash]struct{}      //[ETH/62]完成的块（体）提取集

receiptTaskPool  map[common.Hash]*types.Header //[ETH/63]挂起的收据检索任务，将哈希映射到标题
receiptTaskQueue *prque.Prque                  //[eth/63] Priority queue of the headers to fetch the receipts for
receiptPendPool  map[string]*fetchRequest      //[ETH/63]当前正在等待收据检索操作
receiptDonePool  map[common.Hash]struct{}      //[ETH/63]一套完整的收据提取

resultCache  []*fetchResult     //已下载但尚未传递提取结果
resultOffset uint64             //块链中第一个缓存获取结果的偏移量
resultSize   common.StorageSize //块的近似大小（指数移动平均值）

	lock   *sync.Mutex
	active *sync.Cond
	closed bool
}

//new queue为计划块检索创建新的下载队列。
func newQueue() *queue {
	lock := new(sync.Mutex)
	return &queue{
		headerPendPool:   make(map[string]*fetchRequest),
		headerContCh:     make(chan bool),
		blockTaskPool:    make(map[common.Hash]*types.Header),
		blockTaskQueue:   prque.New(nil),
		blockPendPool:    make(map[string]*fetchRequest),
		blockDonePool:    make(map[common.Hash]struct{}),
		receiptTaskPool:  make(map[common.Hash]*types.Header),
		receiptTaskQueue: prque.New(nil),
		receiptPendPool:  make(map[string]*fetchRequest),
		receiptDonePool:  make(map[common.Hash]struct{}),
		resultCache:      make([]*fetchResult, blockCacheItems),
		active:           sync.NewCond(lock),
		lock:             lock,
	}
}

//重置将清除队列内容。
func (q *queue) Reset() {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.closed = false
	q.mode = FullSync

	q.headerHead = common.Hash{}
	q.headerPendPool = make(map[string]*fetchRequest)

	q.blockTaskPool = make(map[common.Hash]*types.Header)
	q.blockTaskQueue.Reset()
	q.blockPendPool = make(map[string]*fetchRequest)
	q.blockDonePool = make(map[common.Hash]struct{})

	q.receiptTaskPool = make(map[common.Hash]*types.Header)
	q.receiptTaskQueue.Reset()
	q.receiptPendPool = make(map[string]*fetchRequest)
	q.receiptDonePool = make(map[common.Hash]struct{})

	q.resultCache = make([]*fetchResult, blockCacheItems)
	q.resultOffset = 0
}

//Close marks the end of the sync, unblocking Results.
//即使队列已经关闭，也可以调用它。
func (q *queue) Close() {
	q.lock.Lock()
	q.closed = true
	q.lock.Unlock()
	q.active.Broadcast()
}

//PendingHeaders检索等待检索的头请求数。
func (q *queue) PendingHeaders() int {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.headerTaskQueue.Size()
}

//PungBug检索待检索的块（体）请求的数量。
func (q *queue) PendingBlocks() int {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.blockTaskQueue.Size()
}

//PendingReceipts检索待检索的块接收数。
func (q *queue) PendingReceipts() int {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.receiptTaskQueue.Size()
}

//InFlightHeaders retrieves whether there are header fetch requests currently
//在飞行中。
func (q *queue) InFlightHeaders() bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	return len(q.headerPendPool) > 0
}

//InlightBlocks检索当前是否存在块提取请求
//飞行。
func (q *queue) InFlightBlocks() bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	return len(q.blockPendPool) > 0
}

//航班收据检索当前是否有收据取索请求
//在飞行中。
func (q *queue) InFlightReceipts() bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	return len(q.receiptPendPool) > 0
}

//如果队列完全空闲或其中仍有一些数据，则IDLE返回。
func (q *queue) Idle() bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	queued := q.blockTaskQueue.Size() + q.receiptTaskQueue.Size()
	pending := len(q.blockPendPool) + len(q.receiptPendPool)
	cached := len(q.blockDonePool) + len(q.receiptDonePool)

	return (queued + pending + cached) == 0
}

//shouldthrottleBlocks检查是否应限制下载（活动块（正文）
//获取超过块缓存）。
func (q *queue) ShouldThrottleBlocks() bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.resultSlots(q.blockPendPool, q.blockDonePool) <= 0
}

//tottleReceipts是否应检查是否应限制下载（活动收据
//获取超过块缓存）。
func (q *queue) ShouldThrottleReceipts() bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.resultSlots(q.receiptPendPool, q.receiptDonePool) <= 0
}

//resultslots计算可用于请求的结果槽数
//whilst adhering to both the item and the memory limit too of the results
//隐藏物。
func (q *queue) resultSlots(pendPool map[string]*fetchRequest, donePool map[common.Hash]struct{}) int {
//计算内存限制限制的最大长度
	limit := len(q.resultCache)
	if common.StorageSize(len(q.resultCache))*q.resultSize > common.StorageSize(blockCacheMemory) {
		limit = int((common.StorageSize(blockCacheMemory) + q.resultSize - 1) / q.resultSize)
	}
//计算已完成的插槽数
	finished := 0
	for _, result := range q.resultCache[:limit] {
		if result == nil {
			break
		}
		if _, ok := donePool[result.Hash]; ok {
			finished++
		}
	}
//计算当前下载的插槽数
	pending := 0
	for _, request := range pendPool {
		for _, header := range request.Headers {
			if header.Number.Uint64() < q.resultOffset+uint64(limit) {
				pending++
			}
		}
	}
//返回空闲插槽进行分发
	return limit - finished - pending
}

//TraceSkelon将一批报头检索任务添加到队列中以填充
//找到一个已经检索到的头部骨架。
func (q *queue) ScheduleSkeleton(from uint64, skeleton []*types.Header) {
	q.lock.Lock()
	defer q.lock.Unlock()

//无法进行骨架检索，如果进行，则很难失败（巨大的实现错误）
	if q.headerResults != nil {
		panic("skeleton assembly already in progress")
	}
//为骨架程序集安排所有头检索任务
	q.headerTaskPool = make(map[uint64]*types.Header)
	q.headerTaskQueue = prque.New(nil)
q.headerPeerMiss = make(map[string]map[uint64]struct{}) //重置可用性以更正无效的链
	q.headerResults = make([]*types.Header, len(skeleton)*MaxHeaderFetch)
	q.headerProced = 0
	q.headerOffset = from
	q.headerContCh = make(chan bool, 1)

	for i, header := range skeleton {
		index := from + uint64(i*MaxHeaderFetch)

		q.headerTaskPool[index] = header
		q.headerTaskQueue.Push(index, -int64(index))
	}
}

//RetrieveHeaders根据调度的
//骨骼。
func (q *queue) RetrieveHeaders() ([]*types.Header, int) {
	q.lock.Lock()
	defer q.lock.Unlock()

	headers, proced := q.headerResults, q.headerProced
	q.headerResults, q.headerProced = nil, 0

	return headers, proced
}

//schedule为下载队列添加一组头以便进行调度，返回
//遇到新邮件头。
func (q *queue) Schedule(headers []*types.Header, from uint64) []*types.Header {
	q.lock.Lock()
	defer q.lock.Unlock()

//插入按包含的块编号排序的所有标题
	inserts := make([]*types.Header, 0, len(headers))
	for _, header := range headers {
//确保连锁订单始终得到遵守和保存
		hash := header.Hash()
		if header.Number == nil || header.Number.Uint64() != from {
			log.Warn("Header broke chain ordering", "number", header.Number, "hash", hash, "expected", from)
			break
		}
		if q.headerHead != (common.Hash{}) && q.headerHead != header.ParentHash {
			log.Warn("Header broke chain ancestry", "number", header.Number, "hash", hash)
			break
		}
//Make sure no duplicate requests are executed
		if _, ok := q.blockTaskPool[hash]; ok {
			log.Warn("Header already scheduled for block fetch", "number", header.Number, "hash", hash)
			continue
		}
		if _, ok := q.receiptTaskPool[hash]; ok {
			log.Warn("Header already scheduled for receipt fetch", "number", header.Number, "hash", hash)
			continue
		}
//将标题排队以进行内容检索
		q.blockTaskPool[hash] = header
		q.blockTaskQueue.Push(header, -int64(header.Number.Uint64()))

		if q.mode == FastSync {
			q.receiptTaskPool[hash] = header
			q.receiptTaskQueue.Push(header, -int64(header.Number.Uint64()))
		}
		inserts = append(inserts, header)
		q.headerHead = hash
		from++
	}
	return inserts
}

//结果从中检索并永久删除一批提取结果
//高速缓存。如果队列已关闭，则结果切片将为空。
func (q *queue) Results(block bool) []*fetchResult {
	q.lock.Lock()
	defer q.lock.Unlock()

//统计可供处理的项目数
	nproc := q.countProcessableItems()
	for nproc == 0 && !q.closed {
		if !block {
			return nil
		}
		q.active.Wait()
		nproc = q.countProcessableItems()
	}
//因为我们有一个批量限制，所以不要在“悬空”的记忆中多拉一些。
	if nproc > maxResultsProcess {
		nproc = maxResultsProcess
	}
	results := make([]*fetchResult, nproc)
	copy(results, q.resultCache[:nproc])
	if len(results) > 0 {
//将结果标记为已完成，然后将其从缓存中删除。
		for _, result := range results {
			hash := result.Header.Hash()
			delete(q.blockDonePool, hash)
			delete(q.receiptDonePool, hash)
		}
//从缓存中删除结果并清除尾部。
		copy(q.resultCache, q.resultCache[nproc:])
		for i := len(q.resultCache) - nproc; i < len(q.resultCache); i++ {
			q.resultCache[i] = nil
		}
//前进第一个缓存项的预期块号。
		q.resultOffset += uint64(nproc)

//重新计算结果项权重以防止内存耗尽
		for _, result := range results {
			size := result.Header.Size()
			for _, uncle := range result.Uncles {
				size += uncle.Size()
			}
			for _, receipt := range result.Receipts {
				size += receipt.Size()
			}
			for _, tx := range result.Transactions {
				size += tx.Size()
			}
			q.resultSize = common.StorageSize(blockCacheSizeWeight)*size + (1-common.StorageSize(blockCacheSizeWeight))*q.resultSize
		}
	}
	return results
}

//CountProcessableItems统计可处理的项。
func (q *queue) countProcessableItems() int {
	for i, result := range q.resultCache {
		if result == nil || result.Pending > 0 {
			return i
		}
	}
	return len(q.resultCache)
}

//ReserveHeaders为给定的对等端保留一组头，跳过任何
//以前失败的批。
func (q *queue) ReserveHeaders(p *peerConnection, count int) *fetchRequest {
	q.lock.Lock()
	defer q.lock.Unlock()

//如果对方已经下载了内容，则短路（请检查是否正常）
//未损坏状态）
	if _, ok := q.headerPendPool[p.id]; ok {
		return nil
	}
//检索一批哈希，跳过以前失败的哈希
	send, skip := uint64(0), []uint64{}
	for send == 0 && !q.headerTaskQueue.Empty() {
		from, _ := q.headerTaskQueue.Pop()
		if q.headerPeerMiss[p.id] != nil {
			if _, ok := q.headerPeerMiss[p.id][from.(uint64)]; ok {
				skip = append(skip, from.(uint64))
				continue
			}
		}
		send = from.(uint64)
	}
//将所有跳过的批合并回
	for _, from := range skip {
		q.headerTaskQueue.Push(from, -int64(from))
	}
//汇编并返回块下载请求
	if send == 0 {
		return nil
	}
	request := &fetchRequest{
		Peer: p,
		From: send,
		Time: time.Now(),
	}
	q.headerPendPool[p.id] = request
	return request
}

//ReserveBodies reserves a set of body fetches for the given peer, skipping any
//以前下载失败。除了下一批需要的回迁之外，它还
//返回一个标志，指示是否已将空块排队以进行处理。
func (q *queue) ReserveBodies(p *peerConnection, count int) (*fetchRequest, bool, error) {
	isNoop := func(header *types.Header) bool {
		return header.TxHash == types.EmptyRootHash && header.UncleHash == types.EmptyUncleHash
	}
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.reserveHeaders(p, count, q.blockTaskPool, q.blockTaskQueue, q.blockPendPool, q.blockDonePool, isNoop)
}

//ReserveReceipts reserves a set of receipt fetches for the given peer, skipping
//任何以前失败的下载。除了下一批需要的回迁之外，它
//还返回一个标志，指示是否已将空收据排队，需要导入。
func (q *queue) ReserveReceipts(p *peerConnection, count int) (*fetchRequest, bool, error) {
	isNoop := func(header *types.Header) bool {
		return header.ReceiptHash == types.EmptyRootHash
	}
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.reserveHeaders(p, count, q.receiptTaskPool, q.receiptTaskQueue, q.receiptPendPool, q.receiptDonePool, isNoop)
}

//reserveHeaders reserves a set of data download operations for a given peer,
//跳过任何以前失败的。此方法是使用的通用版本
//通过单独的特殊预订功能。
//
//注意，此方法预期队列锁已被保存用于写入。这个
//此处未获取锁的原因是参数已经需要
//要访问队列，因此它们无论如何都需要一个锁。
func (q *queue) reserveHeaders(p *peerConnection, count int, taskPool map[common.Hash]*types.Header, taskQueue *prque.Prque,
	pendPool map[string]*fetchRequest, donePool map[common.Hash]struct{}, isNoop func(*types.Header) bool) (*fetchRequest, bool, error) {
//如果池已耗尽或对等机已耗尽，则短路
//正在下载某些内容（健全性检查不到损坏状态）
	if taskQueue.Empty() {
		return nil, false, nil
	}
	if _, ok := pendPool[p.id]; ok {
		return nil, false, nil
	}
//计算我们可能获取的项目的上限（即限制）
	space := q.resultSlots(pendPool, donePool)

//检索一批任务，跳过以前失败的任务
	send := make([]*types.Header, 0, count)
	skip := make([]*types.Header, 0)

	progress := false
	for proc := 0; proc < space && len(send) < count && !taskQueue.Empty(); proc++ {
		header := taskQueue.PopItem().(*types.Header)
		hash := header.Hash()

//If we're the first to request this task, initialise the result container
		index := int(header.Number.Int64() - int64(q.resultOffset))
		if index >= len(q.resultCache) || index < 0 {
			common.Report("index allocation went beyond available resultCache space")
			return nil, false, errInvalidChain
		}
		if q.resultCache[index] == nil {
			components := 1
			if q.mode == FastSync {
				components = 2
			}
			q.resultCache[index] = &fetchResult{
				Pending: components,
				Hash:    hash,
				Header:  header,
			}
		}
//如果此提取任务是NOOP，则跳过此提取操作
		if isNoop(header) {
			donePool[hash] = struct{}{}
			delete(taskPool, hash)

			space, proc = space-1, proc-1
			q.resultCache[index].Pending--
			progress = true
			continue
		}
//否则，除非知道对等端没有数据，否则将添加到检索列表中。
		if p.Lacks(hash) {
			skip = append(skip, header)
		} else {
			send = append(send, header)
		}
	}
//将所有跳过的邮件头合并回
	for _, header := range skip {
		taskQueue.Push(header, -int64(header.Number.Uint64()))
	}
	if progress {
//唤醒结果，结果缓存已修改
		q.active.Signal()
	}
//汇编并返回块下载请求
	if len(send) == 0 {
		return nil, progress, nil
	}
	request := &fetchRequest{
		Peer:    p,
		Headers: send,
		Time:    time.Now(),
	}
	pendPool[p.id] = request

	return request, progress, nil
}

//CancelHeaders中止提取请求，将所有挂起的骨架索引返回到队列。
func (q *queue) CancelHeaders(request *fetchRequest) {
	q.cancel(request, q.headerTaskQueue, q.headerPendPool)
}

//取消主体中止主体提取请求，将所有挂起的头返回到
//任务队列。
func (q *queue) CancelBodies(request *fetchRequest) {
	q.cancel(request, q.blockTaskQueue, q.blockPendPool)
}

//CancelReceipts中止主体提取请求，将所有挂起的头返回到
//任务队列。
func (q *queue) CancelReceipts(request *fetchRequest) {
	q.cancel(request, q.receiptTaskQueue, q.receiptPendPool)
}

//取消中止提取请求，将所有挂起的哈希返回到任务队列。
func (q *queue) cancel(request *fetchRequest, taskQueue *prque.Prque, pendPool map[string]*fetchRequest) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if request.From > 0 {
		taskQueue.Push(request.From, -int64(request.From))
	}
	for _, header := range request.Headers {
		taskQueue.Push(header, -int64(header.Number.Uint64()))
	}
	delete(pendPool, request.Peer.id)
}

//REVOKE取消属于给定对等机的所有挂起请求。这种方法是
//用于在对等机丢弃期间调用以快速重新分配拥有的数据提取
//到其余节点。
func (q *queue) Revoke(peerID string) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if request, ok := q.blockPendPool[peerID]; ok {
		for _, header := range request.Headers {
			q.blockTaskQueue.Push(header, -int64(header.Number.Uint64()))
		}
		delete(q.blockPendPool, peerID)
	}
	if request, ok := q.receiptPendPool[peerID]; ok {
		for _, header := range request.Headers {
			q.receiptTaskQueue.Push(header, -int64(header.Number.Uint64()))
		}
		delete(q.receiptPendPool, peerID)
	}
}

//ExpireHeaders检查是否有超过超时允许的飞行请求，
//取消他们并将负责的同僚送回受罚。
func (q *queue) ExpireHeaders(timeout time.Duration) map[string]int {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.expire(timeout, q.headerPendPool, q.headerTaskQueue, headerTimeoutMeter)
}

//ExpireBodies checks for in flight block body requests that exceeded a timeout
//津贴，取消他们，并将负责的同龄人送回受罚。
func (q *queue) ExpireBodies(timeout time.Duration) map[string]int {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.expire(timeout, q.blockPendPool, q.blockTaskQueue, bodyTimeoutMeter)
}

//ExpireReceipts检查超过超时的飞行中接收请求
//津贴，取消他们，并将负责的同龄人送回受罚。
func (q *queue) ExpireReceipts(timeout time.Duration) map[string]int {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.expire(timeout, q.receiptPendPool, q.receiptTaskQueue, receiptTimeoutMeter)
}

//expire is the generic check that move expired tasks from a pending pool back
//在任务池中，返回捕获到过期任务的所有实体。
//
//注意，此方法期望队列锁已经被持有。这个
//此处未获取锁的原因是参数已经需要
//要访问队列，因此它们无论如何都需要一个锁。
func (q *queue) expire(timeout time.Duration, pendPool map[string]*fetchRequest, taskQueue *prque.Prque, timeoutMeter metrics.Meter) map[string]int {
//迭代过期的请求并将每个请求返回到队列
	expiries := make(map[string]int)
	for id, request := range pendPool {
		if time.Since(request.Time) > timeout {
//用超时更新度量
			timeoutMeter.Mark(1)

//将任何未满足的请求返回池
			if request.From > 0 {
				taskQueue.Push(request.From, -int64(request.From))
			}
			for _, header := range request.Headers {
				taskQueue.Push(header, -int64(header.Number.Uint64()))
			}
//将对等端添加到到期报告中，同时添加失败的请求数。
			expiries[id] = len(request.Headers)

//直接从挂起池中删除过期的请求
			delete(pendPool, id)
		}
	}
	return expiries
}

//DeliverHeaders将头检索响应注入头结果中
//隐藏物。此方法要么接受它接收到的所有头，要么不接受任何头。
//如果它们不能正确映射到骨架。
//
//如果头被接受，该方法将尝试传递集合
//准备好的头到处理器以保持管道满。然而它会
//not block to prevent stalling other pending deliveries.
func (q *queue) DeliverHeaders(id string, headers []*types.Header, headerProcCh chan []*types.Header) (int, error) {
	q.lock.Lock()
	defer q.lock.Unlock()

//如果从未请求数据，则短路
	request := q.headerPendPool[id]
	if request == nil {
		return 0, errNoFetchesPending
	}
	headerReqTimer.UpdateSince(request.Time)
	delete(q.headerPendPool, id)

//确保头可以映射到骨架链上
	target := q.headerTaskPool[request.From].Hash()

	accepted := len(headers) == MaxHeaderFetch
	if accepted {
		if headers[0].Number.Uint64() != request.From {
			log.Trace("First header broke chain ordering", "peer", id, "number", headers[0].Number, "hash", headers[0].Hash(), request.From)
			accepted = false
		} else if headers[len(headers)-1].Hash() != target {
			log.Trace("Last header broke skeleton structure ", "peer", id, "number", headers[len(headers)-1].Number, "hash", headers[len(headers)-1].Hash(), "expected", target)
			accepted = false
		}
	}
	if accepted {
		for i, header := range headers[1:] {
			hash := header.Hash()
			if want := request.From + 1 + uint64(i); header.Number.Uint64() != want {
				log.Warn("Header broke chain ordering", "peer", id, "number", header.Number, "hash", hash, "expected", want)
				accepted = false
				break
			}
			if headers[i].Hash() != header.ParentHash {
				log.Warn("Header broke chain ancestry", "peer", id, "number", header.Number, "hash", hash)
				accepted = false
				break
			}
		}
	}
//如果批头未被接受，则标记为不可用
	if !accepted {
		log.Trace("Skeleton filling not accepted", "peer", id, "from", request.From)

		miss := q.headerPeerMiss[id]
		if miss == nil {
			q.headerPeerMiss[id] = make(map[uint64]struct{})
			miss = q.headerPeerMiss[id]
		}
		miss[request.From] = struct{}{}

		q.headerTaskQueue.Push(request.From, -int64(request.From))
		return 0, errors.New("delivery not accepted")
	}
//清除成功的获取并尝试传递任何子结果
	copy(q.headerResults[request.From-q.headerOffset:], headers)
	delete(q.headerTaskPool, request.From)

	ready := 0
	for q.headerProced+ready < len(q.headerResults) && q.headerResults[q.headerProced+ready] != nil {
		ready += MaxHeaderFetch
	}
	if ready > 0 {
//收割台准备好交付，收集它们并向前推（非阻塞）
		process := make([]*types.Header, ready)
		copy(process, q.headerResults[q.headerProced:q.headerProced+ready])

		select {
		case headerProcCh <- process:
			log.Trace("Pre-scheduled new headers", "peer", id, "count", len(process), "from", process[0].Number)
			q.headerProced += len(process)
		default:
		}
	}
//Check for termination and return
	if len(q.headerTaskPool) == 0 {
		q.headerContCh <- false
	}
	return len(headers), nil
}

//DeliverBodies将块体检索响应注入结果队列。
//该方法返回从传递中接受的块体数，以及
//同时唤醒等待数据传递的任何线程。
func (q *queue) DeliverBodies(id string, txLists [][]*types.Transaction, uncleLists [][]*types.Header) (int, error) {
	q.lock.Lock()
	defer q.lock.Unlock()

	reconstruct := func(header *types.Header, index int, result *fetchResult) error {
		if types.DeriveSha(types.Transactions(txLists[index])) != header.TxHash || types.CalcUncleHash(uncleLists[index]) != header.UncleHash {
			return errInvalidBody
		}
		result.Transactions = txLists[index]
		result.Uncles = uncleLists[index]
		return nil
	}
	return q.deliver(id, q.blockTaskPool, q.blockTaskQueue, q.blockPendPool, q.blockDonePool, bodyReqTimer, len(txLists), reconstruct)
}

//DeliverReceipts将收据检索响应插入结果队列。
//该方法返回从传递中接受的事务处理接收数。
//and also wakes any threads waiting for data delivery.
func (q *queue) DeliverReceipts(id string, receiptList [][]*types.Receipt) (int, error) {
	q.lock.Lock()
	defer q.lock.Unlock()

	reconstruct := func(header *types.Header, index int, result *fetchResult) error {
		if types.DeriveSha(types.Receipts(receiptList[index])) != header.ReceiptHash {
			return errInvalidReceipt
		}
		result.Receipts = receiptList[index]
		return nil
	}
	return q.deliver(id, q.receiptTaskPool, q.receiptTaskQueue, q.receiptPendPool, q.receiptDonePool, receiptReqTimer, len(receiptList), reconstruct)
}

//传递将数据检索响应注入结果队列。
//
//注意，此方法预期队列锁已被保存用于写入。这个
//此处未获取锁的原因是参数已经需要
//要访问队列，因此它们无论如何都需要一个锁。
func (q *queue) deliver(id string, taskPool map[common.Hash]*types.Header, taskQueue *prque.Prque,
	pendPool map[string]*fetchRequest, donePool map[common.Hash]struct{}, reqTimer metrics.Timer,
	results int, reconstruct func(header *types.Header, index int, result *fetchResult) error) (int, error) {

//如果从未请求数据，则短路
	request := pendPool[id]
	if request == nil {
		return 0, errNoFetchesPending
	}
	reqTimer.UpdateSince(request.Time)
	delete(pendPool, id)

//如果未检索到数据项，则将其标记为对源对等机不可用
	if results == 0 {
		for _, header := range request.Headers {
			request.Peer.MarkLacking(header.Hash())
		}
	}
//将每个结果与其标题和检索到的数据部分组合在一起
	var (
		accepted int
		failure  error
		useful   bool
	)
	for i, header := range request.Headers {
//如果找不到更多提取结果，则短路程序集
		if i >= results {
			break
		}
//如果内容匹配，则重建下一个结果
		index := int(header.Number.Int64() - int64(q.resultOffset))
		if index >= len(q.resultCache) || index < 0 || q.resultCache[index] == nil {
			failure = errInvalidChain
			break
		}
		if err := reconstruct(header, i, q.resultCache[index]); err != nil {
			failure = err
			break
		}
		hash := header.Hash()

		donePool[hash] = struct{}{}
		q.resultCache[index].Pending--
		useful = true
		accepted++

//清除成功的获取
		request.Headers[i] = nil
		delete(taskPool, hash)
	}
//将所有失败或丢失的提取返回到队列
	for _, header := range request.Headers {
		if header != nil {
			taskQueue.Push(header, -int64(header.Number.Uint64()))
		}
	}
//唤醒结果
	if accepted > 0 {
		q.active.Signal()
	}
//如果没有一个数据是好的，那就是过时的交付
	switch {
	case failure == nil || failure == errInvalidChain:
		return accepted, failure
	case useful:
		return accepted, fmt.Errorf("partial failure: %v", failure)
	default:
		return accepted, errStaleDelivery
	}
}

//准备将结果缓存配置为允许接受和缓存入站
//获取结果。
func (q *queue) Prepare(offset uint64, mode SyncMode) {
	q.lock.Lock()
	defer q.lock.Unlock()

//为同步结果准备队列
	if q.resultOffset < offset {
		q.resultOffset = offset
	}
	q.mode = mode
}
