
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

//Package downloader contains the manual full chain synchronisation.
package downloader

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/params"
)

var (
MaxHashFetch    = 512 //每个检索请求要获取的哈希数
MaxBlockFetch   = 128 //每个检索请求要获取的块的数量
MaxHeaderFetch  = 192 //Amount of block headers to be fetched per retrieval request
MaxSkeletonSize = 128 //骨架程序集所需的头提取数
MaxBodyFetch    = 128 //每个检索请求要获取的块体数量
MaxReceiptFetch = 256 //Amount of transaction receipts to allow fetching per request
MaxStateFetch   = 384 //允许每个请求提取的节点状态值的数量

MaxForkAncestry  = 3 * params.EpochDuration //最大链重组
rttMinEstimate   = 2 * time.Second          //下载请求到目标的最短往返时间
rttMaxEstimate   = 20 * time.Second         //下载请求到达目标的最大往返时间
rttMinConfidence = 0.1                      //估计的RTT值的置信系数更差
ttlScaling       = 3                        //RTT->TTL转换的恒定比例因子
ttlLimit         = time.Minute              //防止达到疯狂超时的最大TTL允许值

qosTuningPeers   = 5    //要基于的对等数（最佳对等数）
qosConfidenceCap = 10   //不修改RTT置信度的对等数
qosTuningImpact  = 0.25 //Impact that a new tuning target has on the previous value

maxQueuedHeaders  = 32 * 1024 //[ETH/62]要排队导入的头的最大数目（DOS保护）
maxHeadersProcess = 2048      //一次导入到链中的头下载结果数
maxResultsProcess = 2048      //一次导入到链中的内容下载结果数

reorgProtThreshold   = 48 //禁用mini reorg保护的最近块的阈值数目
reorgProtHeaderDelay = 2  //要延迟交付以覆盖小型重新订购的邮件头数

fsHeaderCheckFrequency = 100             //Verification frequency of the downloaded headers during fast sync
fsHeaderSafetyNet      = 2048            //检测到链冲突时要丢弃的头数
fsHeaderForceVerify    = 24              //要在接受透视之前和之后验证的标题数
fsHeaderContCheck      = 3 * time.Second //Time interval to check for header continuations during state download
fsMinFullBlocks        = 64              //即使在快速同步中也要完全检索的块数
)

var (
	errBusy                    = errors.New("busy")
	errUnknownPeer             = errors.New("peer is unknown or unhealthy")
	errBadPeer                 = errors.New("action from bad peer ignored")
	errStallingPeer            = errors.New("peer is stalling")
	errNoPeers                 = errors.New("no peers to keep download active")
	errTimeout                 = errors.New("timeout")
	errEmptyHeaderSet          = errors.New("empty header set by peer")
	errPeersUnavailable        = errors.New("no peers available or all tried for download")
	errInvalidAncestor         = errors.New("retrieved ancestor is invalid")
	errInvalidChain            = errors.New("retrieved hash chain is invalid")
	errInvalidBlock            = errors.New("retrieved block is invalid")
	errInvalidBody             = errors.New("retrieved block body is invalid")
	errInvalidReceipt          = errors.New("retrieved receipt is invalid")
	errCancelBlockFetch        = errors.New("block download canceled (requested)")
	errCancelHeaderFetch       = errors.New("block header download canceled (requested)")
	errCancelBodyFetch         = errors.New("block body download canceled (requested)")
	errCancelReceiptFetch      = errors.New("receipt download canceled (requested)")
	errCancelStateFetch        = errors.New("state data download canceled (requested)")
	errCancelHeaderProcessing  = errors.New("header processing canceled (requested)")
	errCancelContentProcessing = errors.New("content processing canceled (requested)")
	errNoSyncActive            = errors.New("no sync active")
	errTooOld                  = errors.New("peer doesn't speak recent enough protocol version (need version >= 62)")
)

type Downloader struct {
mode SyncMode       //定义所用策略的同步模式（每个同步周期）
mux  *event.TypeMux //事件同步器宣布同步操作事件

genesis uint64   //限制同步到的Genesis块编号（例如Light Client CHT）
queue   *queue   //用于选择要下载的哈希的计划程序
peers   *peerSet //可从中继续下载的活动对等点集
	stateDB ethdb.Database

rttEstimate   uint64 //目标下载请求的往返时间
rttConfidence uint64 //估计RTT的置信度（单位：百万分之一允许原子操作）

//统计
syncStatsChainOrigin uint64 //开始同步的起始块编号
syncStatsChainHeight uint64 //开始同步时已知的最高块号
	syncStatsState       stateSyncStats
syncStatsLock        sync.RWMutex //锁定保护同步状态字段

	lightchain LightChain
	blockchain BlockChain

//回调
dropPeer peerDropFn //因行为不端而丢掉一个同伴

//状态
synchroniseMock func(id string, hash common.Hash) error //Replacement for synchronise during testing
	synchronising   int32
	notified        int32
	committed       int32

//渠道
headerCh      chan dataPack        //[ETH/62]接收入站数据块头的通道
bodyCh        chan dataPack        //[ETH/62]接收入站闭塞体的信道
receiptCh     chan dataPack        //[ETH/63]接收入站收据的通道
bodyWakeCh    chan bool            //[ETH/62]向新任务的块体获取器发送信号的通道
receiptWakeCh chan bool            //[ETH/63]向接收新任务的接收者发送信号的通道
headerProcCh  chan []*types.Header //[eth/62] Channel to feed the header processor new tasks

//用于StateFetcher
	stateSyncStart chan *stateSync
	trackStateReq  chan *stateReq
stateCh        chan dataPack //[ETH/63]接收入站节点状态数据的通道

//取消和终止
cancelPeer string         //当前用作主机的对等机的标识符（删除时取消）
cancelCh   chan struct{}  //取消飞行中同步的频道
cancelLock sync.RWMutex   //锁定以保护取消通道和对等端传递
cancelWg   sync.WaitGroup //确保所有取出器Goroutine都已退出。

quitCh   chan struct{} //退出通道至信号终止
quitLock sync.RWMutex  //锁定以防止双重关闭

//测试钩
syncInitHook     func(uint64, uint64)  //启动新同步运行时调用的方法
bodyFetchHook    func([]*types.Header) //启动块体提取时要调用的方法
receiptFetchHook func([]*types.Header) //Method to call upon starting a receipt fetch
chainInsertHook  func([]*fetchResult)  //在插入块链时调用的方法（可能在多个调用中）
}

//LightChain封装了同步轻链所需的功能。
type LightChain interface {
//HasHeader verifies a header's presence in the local chain.
	HasHeader(common.Hash, uint64) bool

//GetHeaderByHash从本地链检索头。
	GetHeaderByHash(common.Hash) *types.Header

//currentHeader从本地链中检索头标头。
	CurrentHeader() *types.Header

//gettd返回本地块的总难度。
	GetTd(common.Hash, uint64) *big.Int

//InsertHeaderChain将一批头插入本地链。
	InsertHeaderChain([]*types.Header, int) (int, error)

//回滚从本地链中删除一些最近添加的元素。
	Rollback([]common.Hash)
}

//区块链封装了同步（完整或快速）区块链所需的功能。
type BlockChain interface {
	LightChain

//hasblock验证块在本地链中的存在。
	HasBlock(common.Hash, uint64) bool

//HasFastBlock验证快速块在本地链中的存在。
	HasFastBlock(common.Hash, uint64) bool

//GetBlockByHash从本地链中检索块。
	GetBlockByHash(common.Hash) *types.Block

//currentBlock从本地链检索头块。
	CurrentBlock() *types.Block

//currentFastBlock从本地链检索头快速块。
	CurrentFastBlock() *types.Block

//fastsynccommithead直接将头块提交给某个实体。
	FastSyncCommitHead(common.Hash) error

//插入链将一批块插入到本地链中。
	InsertChain(types.Blocks) (int, error)

//InsertReceiptChain将一批收据插入本地链。
	InsertReceiptChain(types.Blocks, []types.Receipts) (int, error)
}

//新建创建一个新的下载程序，从远程对等端获取哈希和块。
func New(mode SyncMode, stateDb ethdb.Database, mux *event.TypeMux, chain BlockChain, lightchain LightChain, dropPeer peerDropFn) *Downloader {
	if lightchain == nil {
		lightchain = chain
	}

	dl := &Downloader{
		mode:           mode,
		stateDB:        stateDb,
		mux:            mux,
		queue:          newQueue(),
		peers:          newPeerSet(),
		rttEstimate:    uint64(rttMaxEstimate),
		rttConfidence:  uint64(1000000),
		blockchain:     chain,
		lightchain:     lightchain,
		dropPeer:       dropPeer,
		headerCh:       make(chan dataPack, 1),
		bodyCh:         make(chan dataPack, 1),
		receiptCh:      make(chan dataPack, 1),
		bodyWakeCh:     make(chan bool, 1),
		receiptWakeCh:  make(chan bool, 1),
		headerProcCh:   make(chan []*types.Header, 1),
		quitCh:         make(chan struct{}),
		stateCh:        make(chan dataPack),
		stateSyncStart: make(chan *stateSync),
		syncStatsState: stateSyncStats{
			processed: rawdb.ReadFastTrieProgress(stateDb),
		},
		trackStateReq: make(chan *stateReq),
	}
	go dl.qosTuner()
	go dl.stateFetcher()
	return dl
}

//进程检索同步边界，特别是起源。
//同步开始于的块（可能已失败/暂停）；块
//或头同步当前位于；以及同步目标的最新已知块。
//
//此外，在快速同步的状态下载阶段，
//同时返回已处理状态和已知状态总数。否则
//这些都是零。
func (d *Downloader) Progress() ethereum.SyncProgress {
//锁定当前状态并返回进度
	d.syncStatsLock.RLock()
	defer d.syncStatsLock.RUnlock()

	current := uint64(0)
	switch d.mode {
	case FullSync:
		current = d.blockchain.CurrentBlock().NumberU64()
	case FastSync:
		current = d.blockchain.CurrentFastBlock().NumberU64()
	case LightSync:
		current = d.lightchain.CurrentHeader().Number.Uint64()
	}
	return ethereum.SyncProgress{
		StartingBlock: d.syncStatsChainOrigin,
		CurrentBlock:  current,
		HighestBlock:  d.syncStatsChainHeight,
		PulledStates:  d.syncStatsState.processed,
		KnownStates:   d.syncStatsState.processed + d.syncStatsState.pending,
	}
}

//同步返回下载程序当前是否正在检索块。
func (d *Downloader) Synchronising() bool {
	return atomic.LoadInt32(&d.synchronising) > 0
}

//registerpeer将一个新的下载对等注入到要
//用于从获取哈希和块。
func (d *Downloader) RegisterPeer(id string, version int, peer Peer) error {
	logger := log.New("peer", id)
	logger.Trace("Registering sync peer")
	if err := d.peers.Register(newPeerConnection(id, version, peer, logger)); err != nil {
		logger.Error("Failed to register sync peer", "err", err)
		return err
	}
	d.qosReduceConfidence()

	return nil
}

//Regiterlightpeer注入一个轻量级客户端对等端，将其包装起来，使其看起来像一个普通对等端。
func (d *Downloader) RegisterLightPeer(id string, version int, peer LightPeer) error {
	return d.RegisterPeer(id, version, &lightPeerWrapper{peer})
}

//注销对等机从已知列表中删除对等机，以阻止
//指定的对等机。还将努力将任何挂起的回迁返回到
//排队。
func (d *Downloader) UnregisterPeer(id string) error {
//从活动对等机集中注销对等机并撤消任何获取任务
	logger := log.New("peer", id)
	logger.Trace("Unregistering sync peer")
	if err := d.peers.Unregister(id); err != nil {
		logger.Error("Failed to unregister sync peer", "err", err)
		return err
	}
	d.queue.Revoke(id)

//如果此对等是主对等，则立即中止同步
	d.cancelLock.RLock()
	master := id == d.cancelPeer
	d.cancelLock.RUnlock()

	if master {
		d.cancel()
	}
	return nil
}

//Synchronise尝试将本地区块链与远程对等机同步，两者都是
//添加各种健全性检查，并用各种日志条目包装它。
func (d *Downloader) Synchronise(id string, head common.Hash, td *big.Int, mode SyncMode) error {
	err := d.synchronise(id, head, td, mode)
	switch err {
	case nil:
	case errBusy:

	case errTimeout, errBadPeer, errStallingPeer,
		errEmptyHeaderSet, errPeersUnavailable, errTooOld,
		errInvalidAncestor, errInvalidChain:
		log.Warn("Synchronisation failed, dropping peer", "peer", id, "err", err)
		if d.dropPeer == nil {
//当对本地副本使用“--copydb”时，droppeer方法为nil。
//如果压缩在错误的时间命中，则可能发生超时，并且可以忽略。
			log.Warn("Downloader wants to drop peer, but peerdrop-function is not set", "peer", id)
		} else {
			d.dropPeer(id)
		}
	default:
		log.Warn("Synchronisation failed, retrying", "err", err)
	}
	return err
}

//同步将选择对等机并使用它进行同步。如果给出空字符串
//如果它的td比我们自己的高，它将使用尽可能最好的对等机并进行同步。如果有
//检查失败，将返回错误。此方法是同步的
func (d *Downloader) synchronise(id string, hash common.Hash, td *big.Int, mode SyncMode) error {
//模拟同步如果测试
	if d.synchroniseMock != nil {
		return d.synchroniseMock(id, hash)
	}
//确保一次只允许一个Goroutine通过此点
	if !atomic.CompareAndSwapInt32(&d.synchronising, 0, 1) {
		return errBusy
	}
	defer atomic.StoreInt32(&d.synchronising, 0)

//发布同步的用户通知（每个会话仅一次）
	if atomic.CompareAndSwapInt32(&d.notified, 0, 1) {
		log.Info("Block synchronisation started")
	}
//重置队列、对等设置和唤醒通道以清除任何内部剩余状态
	d.queue.Reset()
	d.peers.Reset()

	for _, ch := range []chan bool{d.bodyWakeCh, d.receiptWakeCh} {
		select {
		case <-ch:
		default:
		}
	}
	for _, ch := range []chan dataPack{d.headerCh, d.bodyCh, d.receiptCh} {
		for empty := false; !empty; {
			select {
			case <-ch:
			default:
				empty = true
			}
		}
	}
	for empty := false; !empty; {
		select {
		case <-d.headerProcCh:
		default:
			empty = true
		}
	}
//为中途中止创建取消频道并标记主对等机
	d.cancelLock.Lock()
	d.cancelCh = make(chan struct{})
	d.cancelPeer = id
	d.cancelLock.Unlock()

defer d.Cancel() //不管怎样，我们不能让取消频道一直开着

//设置请求的同步模式，除非被禁止
	d.mode = mode

//Retrieve the origin peer and initiate the downloading process
	p := d.peers.Peer(id)
	if p == nil {
		return errUnknownPeer
	}
	return d.syncWithPeer(p, hash, td)
}

//SyncWithPeer根据来自
//指定的对等和头哈希。
func (d *Downloader) syncWithPeer(p *peerConnection, hash common.Hash, td *big.Int) (err error) {
	d.mux.Post(StartEvent{})
	defer func() {
//错误重置
		if err != nil {
			d.mux.Post(FailedEvent{err})
		} else {
			d.mux.Post(DoneEvent{})
		}
	}()
	if p.version < 62 {
		return errTooOld
	}

	log.Debug("Synchronising with the network", "peer", p.id, "eth", p.version, "head", hash, "td", td, "mode", d.mode)
	defer func(start time.Time) {
		log.Debug("Synchronisation terminated", "elapsed", time.Since(start))
	}(time.Now())

//查找同步边界：共同祖先和目标块
	latest, err := d.fetchHeight(p)
	if err != nil {
		return err
	}
	height := latest.Number.Uint64()

	origin, err := d.findAncestor(p, latest)
	if err != nil {
		return err
	}
	d.syncStatsLock.Lock()
	if d.syncStatsChainHeight <= origin || d.syncStatsChainOrigin > origin {
		d.syncStatsChainOrigin = origin
	}
	d.syncStatsChainHeight = height
	d.syncStatsLock.Unlock()

//确保我们的原点在任何快速同步轴点之下
	pivot := uint64(0)
	if d.mode == FastSync {
		if height <= uint64(fsMinFullBlocks) {
			origin = 0
		} else {
			pivot = height - uint64(fsMinFullBlocks)
			if pivot <= origin {
				origin = pivot - 1
			}
		}
	}
	d.committed = 1
	if d.mode == FastSync && pivot != 0 {
		d.committed = 0
	}
//使用并发头和内容检索算法启动同步
	d.queue.Prepare(origin+1, d.mode)
	if d.syncInitHook != nil {
		d.syncInitHook(origin, height)
	}

	fetchers := []func() error{
func() error { return d.fetchHeaders(p, origin+1, pivot) }, //始终检索邮件头
func() error { return d.fetchBodies(origin + 1) },          //在正常和快速同步期间检索主体
func() error { return d.fetchReceipts(origin + 1) },        //在快速同步过程中检索收据
		func() error { return d.processHeaders(origin+1, pivot, td) },
	}
	if d.mode == FastSync {
		fetchers = append(fetchers, func() error { return d.processFastSyncContent(latest) })
	} else if d.mode == FullSync {
		fetchers = append(fetchers, d.processFullSyncContent)
	}
	return d.spawnSync(fetchers)
}

//spawnSync runs d.process and all given fetcher functions to completion in
//分离goroutine，返回出现的第一个错误。
func (d *Downloader) spawnSync(fetchers []func() error) error {
	errc := make(chan error, len(fetchers))
	d.cancelWg.Add(len(fetchers))
	for _, fn := range fetchers {
		fn := fn
		go func() { defer d.cancelWg.Done(); errc <- fn() }()
	}
//等待第一个错误，然后终止其他错误。
	var err error
	for i := 0; i < len(fetchers); i++ {
		if i == len(fetchers)-1 {
//当所有提取程序退出时关闭队列。
//这将导致块处理器在
//它已经处理了队列。
			d.queue.Close()
		}
		if err = <-errc; err != nil {
			break
		}
	}
	d.queue.Close()
	d.Cancel()
	return err
}

//取消中止所有操作并重置队列。但是，取消是
//not wait for the running download goroutines to finish. This method should be
//从下载程序内部取消下载时使用。
func (d *Downloader) cancel() {
//关闭当前取消频道
	d.cancelLock.Lock()
	if d.cancelCh != nil {
		select {
		case <-d.cancelCh:
//频道已关闭
		default:
			close(d.cancelCh)
		}
	}
	d.cancelLock.Unlock()
}

//取消中止所有操作，并等待所有下载Goroutines到
//返回前完成。
func (d *Downloader) Cancel() {
	d.cancel()
	d.cancelWg.Wait()
}

//Terminate interrupts the downloader, canceling all pending operations.
//调用terminate后，下载程序不能再使用。
func (d *Downloader) Terminate() {
//关闭终端通道（确保允许双重关闭）
	d.quitLock.Lock()
	select {
	case <-d.quitCh:
	default:
		close(d.quitCh)
	}
	d.quitLock.Unlock()

//取消任何挂起的下载请求
	d.Cancel()
}

//fetchHeight retrieves the head header of the remote peer to aid in estimating
//等待同步所需的总时间。
func (d *Downloader) fetchHeight(p *peerConnection) (*types.Header, error) {
	p.log.Debug("Retrieving remote chain height")

//请求公布的远程头块并等待响应
	head, _ := p.peer.Head()
	go p.peer.RequestHeadersByHash(head, 1, 0, false)

	ttl := d.requestTTL()
	timeout := time.After(ttl)
	for {
		select {
		case <-d.cancelCh:
			return nil, errCancelBlockFetch

		case packet := <-d.headerCh:
//丢弃源对等机以外的任何内容
			if packet.PeerId() != p.id {
				log.Debug("Received headers from incorrect peer", "peer", packet.PeerId())
				break
			}
//Make sure the peer actually gave something valid
			headers := packet.(*headerPack).headers
			if len(headers) != 1 {
				p.log.Debug("Multiple headers for single request", "headers", len(headers))
				return nil, errBadPeer
			}
			head := headers[0]
			p.log.Debug("Remote head header identified", "number", head.Number, "hash", head.Hash())
			return head, nil

		case <-timeout:
			p.log.Debug("Waiting for head header timed out", "elapsed", ttl)
			return nil, errTimeout

		case <-d.bodyCh:
		case <-d.receiptCh:
//越界交货，忽略
		}
	}
}

//CalculateRequestSpan计算在试图确定
//共同祖先。
//返回peer.requestHeadersByNumber要使用的参数：
//起始块编号
//Count-要请求的头数
//skip-要跳过的头数
//并返回“max”，即远程对等方预期返回的最后一个块，
//给定（从、计数、跳过）
func calculateRequestSpan(remoteHeight, localHeight uint64) (int64, int, int, uint64) {
	var (
		from     int
		count    int
		MaxCount = MaxHeaderFetch / 16
	)
//请求头是我们将请求的最高块。如果请求头没有偏移，
//我们将要到达的最高街区是距离头部16个街区，这意味着我们
//在高度差的情况下，将不必要地获取14或15个块。
//我们和同龄人之间是1-2个街区，这是最常见的
	requestHead := int(remoteHeight) - 1
	if requestHead < 0 {
		requestHead = 0
	}
//RequestBottom是我们希望在查询中包含的最低块
//理想情况下，我们希望包括在自己的头脑下面
	requestBottom := int(localHeight - 1)
	if requestBottom < 0 {
		requestBottom = 0
	}
	totalSpan := requestHead - requestBottom
	span := 1 + totalSpan/MaxCount
	if span < 2 {
		span = 2
	}
	if span > 16 {
		span = 16
	}

	count = 1 + totalSpan/span
	if count > MaxCount {
		count = MaxCount
	}
	if count < 2 {
		count = 2
	}
	from = requestHead - (count-1)*span
	if from < 0 {
		from = 0
	}
	max := from + (count-1)*span
	return int64(from), count, span - 1, uint64(max)
}

//findancestor试图定位本地链的共同祖先链接，并且
//远程对等区块链。在一般情况下，当我们的节点处于同步状态时，
//在正确的链条上，检查顶部的N个链环应该已经得到了匹配。
//在罕见的情况下，当我们结束了长期的重组（即没有
//头部链接匹配），我们进行二进制搜索以找到共同的祖先。
func (d *Downloader) findAncestor(p *peerConnection, remoteHeader *types.Header) (uint64, error) {
//找出有效的祖先范围以防止重写攻击
	var (
		floor        = int64(-1)
		localHeight  uint64
		remoteHeight = remoteHeader.Number.Uint64()
	)
	switch d.mode {
	case FullSync:
		localHeight = d.blockchain.CurrentBlock().NumberU64()
	case FastSync:
		localHeight = d.blockchain.CurrentFastBlock().NumberU64()
	default:
		localHeight = d.lightchain.CurrentHeader().Number.Uint64()
	}
	p.log.Debug("Looking for common ancestor", "local", localHeight, "remote", remoteHeight)
	if localHeight >= MaxForkAncestry {
//我们超过了最大REORG阈值，找到最早的分叉点
		floor = int64(localHeight - MaxForkAncestry)

//如果我们进行灯光同步，确保地板不低于CHT，如
//在此点之前的所有标题都将丢失。
		if d.mode == LightSync {
//如果我们不知道当前的CHT位置，找到它
			if d.genesis == 0 {
				header := d.lightchain.CurrentHeader()
				for header != nil {
					d.genesis = header.Number.Uint64()
					if floor >= int64(d.genesis)-1 {
						break
					}
					header = d.lightchain.GetHeaderByHash(header.ParentHash)
				}
			}
//我们已经知道“创世”的街区号了，盖楼到那
			if floor < int64(d.genesis)-1 {
				floor = int64(d.genesis) - 1
			}
		}
	}
	from, count, skip, max := calculateRequestSpan(remoteHeight, localHeight)

	p.log.Trace("Span searching for common ancestor", "count", count, "from", from, "skip", skip)
	go p.peer.RequestHeadersByNumber(uint64(from), count, skip, false)

//等待对头提取的远程响应
	number, hash := uint64(0), common.Hash{}

	ttl := d.requestTTL()
	timeout := time.After(ttl)

	for finished := false; !finished; {
		select {
		case <-d.cancelCh:
			return 0, errCancelHeaderFetch

		case packet := <-d.headerCh:
//丢弃源对等机以外的任何内容
			if packet.PeerId() != p.id {
				log.Debug("Received headers from incorrect peer", "peer", packet.PeerId())
				break
			}
//确保对方给出了有效的信息
			headers := packet.(*headerPack).headers
			if len(headers) == 0 {
				p.log.Warn("Empty head header set")
				return 0, errEmptyHeaderSet
			}
//确保对等方的答复符合请求
			for i, header := range headers {
				expectNumber := from + int64(i)*int64((skip+1))
				if number := header.Number.Int64(); number != expectNumber {
					p.log.Warn("Head headers broke chain ordering", "index", i, "requested", expectNumber, "received", number)
					return 0, errInvalidChain
				}
			}
//检查是否找到共同祖先
			finished = true
			for i := len(headers) - 1; i >= 0; i-- {
//跳过任何下溢/溢出请求集的头
				if headers[i].Number.Int64() < from || headers[i].Number.Uint64() > max {
					continue
				}
//否则检查我们是否已经知道标题
				h := headers[i].Hash()
				n := headers[i].Number.Uint64()

				var known bool
				switch d.mode {
				case FullSync:
					known = d.blockchain.HasBlock(h, n)
				case FastSync:
					known = d.blockchain.HasFastBlock(h, n)
				default:
					known = d.lightchain.HasHeader(h, n)
				}
				if known {
					number, hash = n, h
					break
				}
			}

		case <-timeout:
			p.log.Debug("Waiting for head header timed out", "elapsed", ttl)
			return 0, errTimeout

		case <-d.bodyCh:
		case <-d.receiptCh:
//越界交货，忽略
		}
	}
//如果head fetch已经找到祖先，则返回
	if hash != (common.Hash{}) {
		if int64(number) <= floor {
			p.log.Warn("Ancestor below allowance", "number", number, "hash", hash, "allowance", floor)
			return 0, errInvalidAncestor
		}
		p.log.Debug("Found common ancestor", "number", number, "hash", hash)
		return number, nil
	}
//找不到祖先，我们需要在链上进行二进制搜索
	start, end := uint64(0), remoteHeight
	if floor > 0 {
		start = uint64(floor)
	}
	p.log.Trace("Binary searching for common ancestor", "start", start, "end", end)

	for start+1 < end {
//将链间隔拆分为两个，并请求哈希进行交叉检查
		check := (start + end) / 2

		ttl := d.requestTTL()
		timeout := time.After(ttl)

		go p.peer.RequestHeadersByNumber(check, 1, 0, false)

//等待答复到达此请求
		for arrived := false; !arrived; {
			select {
			case <-d.cancelCh:
				return 0, errCancelHeaderFetch

			case packer := <-d.headerCh:
//丢弃源对等机以外的任何内容
				if packer.PeerId() != p.id {
					log.Debug("Received headers from incorrect peer", "peer", packer.PeerId())
					break
				}
//确保对方给出了有效的信息
				headers := packer.(*headerPack).headers
				if len(headers) != 1 {
					p.log.Debug("Multiple headers for single request", "headers", len(headers))
					return 0, errBadPeer
				}
				arrived = true

//根据响应修改搜索间隔
				h := headers[0].Hash()
				n := headers[0].Number.Uint64()

				var known bool
				switch d.mode {
				case FullSync:
					known = d.blockchain.HasBlock(h, n)
				case FastSync:
					known = d.blockchain.HasFastBlock(h, n)
				default:
					known = d.lightchain.HasHeader(h, n)
				}
				if !known {
					end = check
					break
				}
header := d.lightchain.GetHeaderByHash(h) //独立于同步模式，头文件肯定存在
				if header.Number.Uint64() != check {
					p.log.Debug("Received non requested header", "number", header.Number, "hash", header.Hash(), "request", check)
					return 0, errBadPeer
				}
				start = check
				hash = h

			case <-timeout:
				p.log.Debug("Waiting for search header timed out", "elapsed", ttl)
				return 0, errTimeout

			case <-d.bodyCh:
			case <-d.receiptCh:
//越界交货，忽略
			}
		}
	}
//确保有效的祖传和回归
	if int64(start) <= floor {
		p.log.Warn("Ancestor below allowance", "number", start, "hash", hash, "allowance", floor)
		return 0, errInvalidAncestor
	}
	p.log.Debug("Found common ancestor", "number", start, "hash", hash)
	return start, nil
}

//FetchHeaders始终从数字中同时检索头
//请求，直到不再返回，可能会在途中限制。到
//方便并发，但仍能防止恶意节点发送错误
//headers，我们使用“origin”对等体构造一个header链骨架。
//正在与同步，并使用其他人填写丢失的邮件头。报头
//只有当其他对等点干净地映射到骨架时，才接受它们。如果没有人
//可以填充骨架-甚至不是源节点-它被假定为无效和
//原点被删除。
func (d *Downloader) fetchHeaders(p *peerConnection, from uint64, pivot uint64) error {
	p.log.Debug("Directing header downloads", "origin", from)
	defer p.log.Debug("Header download terminated")

//创建超时计时器和相关联的头提取程序
skeleton := true            //骨架装配阶段或完成
request := time.Now()       //最后一个骨架获取请求的时间
timeout := time.NewTimer(0) //转储非响应活动对等机的计时器
<-timeout.C                 //超时通道最初应为空
	defer timeout.Stop()

	var ttl time.Duration
	getHeaders := func(from uint64) {
		request = time.Now()

		ttl = d.requestTTL()
		timeout.Reset(ttl)

		if skeleton {
			p.log.Trace("Fetching skeleton headers", "count", MaxHeaderFetch, "from", from)
			go p.peer.RequestHeadersByNumber(from+uint64(MaxHeaderFetch)-1, MaxSkeletonSize, MaxHeaderFetch-1, false)
		} else {
			p.log.Trace("Fetching full headers", "count", MaxHeaderFetch, "from", from)
			go p.peer.RequestHeadersByNumber(from, MaxHeaderFetch, 0, false)
		}
	}
//开始拉动收割台链条骨架，直到全部完成。
	getHeaders(from)

	for {
		select {
		case <-d.cancelCh:
			return errCancelHeaderFetch

		case packet := <-d.headerCh:
//确保活动对等端正在向我们提供骨架头
			if packet.PeerId() != p.id {
				log.Debug("Received skeleton from incorrect peer", "peer", packet.PeerId())
				break
			}
			headerReqTimer.UpdateSince(request)
			timeout.Stop()

//如果骨架已完成，则直接从原点拉出任何剩余的头部标题。
			if packet.Items() == 0 && skeleton {
				skeleton = false
				getHeaders(from)
				continue
			}
//如果没有更多的头是入站的，通知内容提取程序并返回
			if packet.Items() == 0 {
//下载数据透视时不要中止头提取
				if atomic.LoadInt32(&d.committed) == 0 && pivot <= from {
					p.log.Debug("No headers, waiting for pivot commit")
					select {
					case <-time.After(fsHeaderContCheck):
						getHeaders(from)
						continue
					case <-d.cancelCh:
						return errCancelHeaderFetch
					}
				}
//透视完成（或不快速同步）并且没有更多的头，终止进程
				p.log.Debug("No more headers available")
				select {
				case d.headerProcCh <- nil:
					return nil
				case <-d.cancelCh:
					return errCancelHeaderFetch
				}
			}
			headers := packet.(*headerPack).headers

//如果我们接收到一个框架批处理，那么同时解析内部构件
			if skeleton {
				filled, proced, err := d.fillHeaderSkeleton(from, headers)
				if err != nil {
					p.log.Debug("Skeleton chain invalid", "err", err)
					return errInvalidChain
				}
				headers = filled[proced:]
				from += uint64(proced)
			} else {
//如果我们正在接近链头，但还没有到达，请延迟。
//最后几个头，这样头上的微小重新排序不会导致无效哈希
//链误差
				if n := len(headers); n > 0 {
//找回我们现在的头颅
					head := uint64(0)
					if d.mode == LightSync {
						head = d.lightchain.CurrentHeader().Number.Uint64()
					} else {
						head = d.blockchain.CurrentFastBlock().NumberU64()
						if full := d.blockchain.CurrentBlock().NumberU64(); head < full {
							head = full
						}
					}
//如果磁头比此批老得多，请延迟最后几个磁头
					if head+uint64(reorgProtThreshold) < headers[n-1].Number.Uint64() {
						delay := reorgProtHeaderDelay
						if delay > n {
							delay = n
						}
						headers = headers[:n-delay]
					}
				}
			}
//插入所有新标题并获取下一批
			if len(headers) > 0 {
				p.log.Trace("Scheduling new headers", "count", len(headers), "from", from)
				select {
				case d.headerProcCh <- headers:
				case <-d.cancelCh:
					return errCancelHeaderFetch
				}
				from += uint64(len(headers))
				getHeaders(from)
			} else {
//没有发送邮件头，或者所有邮件都被延迟，请稍睡片刻，然后重试。
				p.log.Trace("All headers delayed, waiting")
				select {
				case <-time.After(fsHeaderContCheck):
					getHeaders(from)
					continue
				case <-d.cancelCh:
					return errCancelHeaderFetch
				}
			}

		case <-timeout.C:
			if d.dropPeer == nil {
//当对本地副本使用“--copydb”时，droppeer方法为nil。
//如果压缩在错误的时间命中，则可能发生超时，并且可以忽略。
				p.log.Warn("Downloader wants to drop peer, but peerdrop-function is not set", "peer", p.id)
				break
			}
//头检索超时，考虑对等机错误并丢弃
			p.log.Debug("Header request timed out", "elapsed", ttl)
			headerTimeoutMeter.Mark(1)
			d.dropPeer(p.id)

//但是，请优雅地完成同步，而不是转储收集的数据
			for _, ch := range []chan bool{d.bodyWakeCh, d.receiptWakeCh} {
				select {
				case ch <- false:
				case <-d.cancelCh:
				}
			}
			select {
			case d.headerProcCh <- nil:
			case <-d.cancelCh:
			}
			return errBadPeer
		}
	}
}

//FillHeaderskeleton同时从所有可用的对等端检索头
//并将它们映射到提供的骨架头链。
//
//从骨架开始的任何部分结果（如果可能）都将被转发
//立即发送到头处理器，以保持管道的其余部分保持平衡
//如果收割台失速。
//
//该方法返回整个填充骨架以及头的数量。
//已转发进行处理。
func (d *Downloader) fillHeaderSkeleton(from uint64, skeleton []*types.Header) ([]*types.Header, int, error) {
	log.Debug("Filling up skeleton", "from", from)
	d.queue.ScheduleSkeleton(from, skeleton)

	var (
		deliver = func(packet dataPack) (int, error) {
			pack := packet.(*headerPack)
			return d.queue.DeliverHeaders(pack.peerID, pack.headers, d.headerProcCh)
		}
		expire   = func() map[string]int { return d.queue.ExpireHeaders(d.requestTTL()) }
		throttle = func() bool { return false }
		reserve  = func(p *peerConnection, count int) (*fetchRequest, bool, error) {
			return d.queue.ReserveHeaders(p, count), false, nil
		}
		fetch    = func(p *peerConnection, req *fetchRequest) error { return p.FetchHeaders(req.From, MaxHeaderFetch) }
		capacity = func(p *peerConnection) int { return p.HeaderCapacity(d.requestRTT()) }
		setIdle  = func(p *peerConnection, accepted int) { p.SetHeadersIdle(accepted) }
	)
	err := d.fetchParts(errCancelHeaderFetch, d.headerCh, deliver, d.queue.headerContCh, expire,
		d.queue.PendingHeaders, d.queue.InFlightHeaders, throttle, reserve,
		nil, fetch, d.queue.CancelHeaders, capacity, d.peers.HeaderIdlePeers, setIdle, "headers")

	log.Debug("Skeleton fill terminated", "err", err)

	filled, proced := d.queue.RetrieveHeaders()
	return filled, proced, err
}

//fetchbodies迭代下载计划的块体，获取
//可用对等机，为每个对等机保留一大块数据块，等待传递
//并定期检查超时情况。
func (d *Downloader) fetchBodies(from uint64) error {
	log.Debug("Downloading block bodies", "origin", from)

	var (
		deliver = func(packet dataPack) (int, error) {
			pack := packet.(*bodyPack)
			return d.queue.DeliverBodies(pack.peerID, pack.transactions, pack.uncles)
		}
		expire   = func() map[string]int { return d.queue.ExpireBodies(d.requestTTL()) }
		fetch    = func(p *peerConnection, req *fetchRequest) error { return p.FetchBodies(req) }
		capacity = func(p *peerConnection) int { return p.BlockCapacity(d.requestRTT()) }
		setIdle  = func(p *peerConnection, accepted int) { p.SetBodiesIdle(accepted) }
	)
	err := d.fetchParts(errCancelBodyFetch, d.bodyCh, deliver, d.bodyWakeCh, expire,
		d.queue.PendingBlocks, d.queue.InFlightBlocks, d.queue.ShouldThrottleBlocks, d.queue.ReserveBodies,
		d.bodyFetchHook, fetch, d.queue.CancelBodies, capacity, d.peers.BodyIdlePeers, setIdle, "bodies")

	log.Debug("Block body download terminated", "err", err)
	return err
}

//fetchreceipts迭代地下载计划的块接收，获取
//可用的对等方，为每个对等方保留一大块收据，等待传递
//并定期检查超时情况。
func (d *Downloader) fetchReceipts(from uint64) error {
	log.Debug("Downloading transaction receipts", "origin", from)

	var (
		deliver = func(packet dataPack) (int, error) {
			pack := packet.(*receiptPack)
			return d.queue.DeliverReceipts(pack.peerID, pack.receipts)
		}
		expire   = func() map[string]int { return d.queue.ExpireReceipts(d.requestTTL()) }
		fetch    = func(p *peerConnection, req *fetchRequest) error { return p.FetchReceipts(req) }
		capacity = func(p *peerConnection) int { return p.ReceiptCapacity(d.requestRTT()) }
		setIdle  = func(p *peerConnection, accepted int) { p.SetReceiptsIdle(accepted) }
	)
	err := d.fetchParts(errCancelReceiptFetch, d.receiptCh, deliver, d.receiptWakeCh, expire,
		d.queue.PendingReceipts, d.queue.InFlightReceipts, d.queue.ShouldThrottleReceipts, d.queue.ReserveReceipts,
		d.receiptFetchHook, fetch, d.queue.CancelReceipts, capacity, d.peers.ReceiptIdlePeers, setIdle, "receipts")

	log.Debug("Transaction receipt download terminated", "err", err)
	return err
}

//fetchparts迭代地下载计划的块部件，获取任何可用的
//对等机，为每个对等机保留一大块获取请求，等待传递和
//还要定期检查超时情况。
//
//由于所有下载的数据的调度/超时逻辑基本相同
//类型，此方法由每个方法用于数据收集，并使用
//处理它们之间的细微差别的各种回调。
//
//仪器参数：
//-errCancel:取消提取操作时返回的错误类型（主要使日志记录更好）
//-deliverych：从中检索下载数据包的通道（从所有并发对等机合并）
//-deliver:处理回调以将数据包传递到特定于类型的下载队列（通常在“queue”内）
//-wakech：通知通道，用于在新任务可用（或同步完成）时唤醒提取程序。
//-expire：任务回调方法，用于中止耗时太长的请求并返回故障对等端（流量形成）
//-挂起：对仍需要下载的请求数的任务回调（检测完成/不可完成性）
//-机上：正在进行的请求数的任务回调（等待所有活动下载完成）
//-限制：任务回调以检查处理队列是否已满并激活限制（绑定内存使用）
//-reserve：任务回调，将新的下载任务保留给特定的对等方（也表示部分完成）
//-fetchhook:tester回调，通知正在启动的新任务（允许测试调度逻辑）
//-fetch：网络回调，实际向物理远程对等端发送特定下载请求
//-取消：任务回调，以中止飞行中的下载请求并允许重新安排（如果对等机丢失）
//-容量：网络回调以检索对等机的估计类型特定带宽容量（流量形成）
//-idle：网络回调以检索当前（特定类型）可分配任务的空闲对等机
//-set idle：网络回调，将对等机设置回空闲状态，并更新其估计容量（流量形成）
//-kind:下载类型的文本标签，显示在日志消息中
func (d *Downloader) fetchParts(errCancel error, deliveryCh chan dataPack, deliver func(dataPack) (int, error), wakeCh chan bool,
	expire func() map[string]int, pending func() int, inFlight func() bool, throttle func() bool, reserve func(*peerConnection, int) (*fetchRequest, bool, error),
	fetchHook func([]*types.Header), fetch func(*peerConnection, *fetchRequest) error, cancel func(*fetchRequest), capacity func(*peerConnection) int,
	idle func() ([]*peerConnection, int), setIdle func(*peerConnection, int), kind string) error {

//创建一个标记器以检测过期的检索任务
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	update := make(chan struct{}, 1)

//准备队列并获取块部件，直到块头获取器完成
	finished := false
	for {
		select {
		case <-d.cancelCh:
			return errCancel

		case packet := <-deliveryCh:
//如果之前同伴被禁止并且未能递送包裹
//在合理的时间范围内，忽略其消息。
			if peer := d.peers.Peer(packet.PeerId()); peer != nil {
//传递接收到的数据块并检查链的有效性
				accepted, err := deliver(packet)
				if err == errInvalidChain {
					return err
				}
//除非一个同伴提供了完全不需要的东西（通常
//由最后通过的超时请求引起），将其设置为
//空闲的如果传递已过时，则对等端应该已经空闲。
				if err != errStaleDelivery {
					setIdle(peer, accepted)
				}
//向用户发布日志以查看发生了什么
				switch {
				case err == nil && packet.Items() == 0:
					peer.log.Trace("Requested data not delivered", "type", kind)
				case err == nil:
					peer.log.Trace("Delivered new batch of data", "type", kind, "count", packet.Stats())
				default:
					peer.log.Trace("Failed to deliver retrieved data", "type", kind, "err", err)
				}
			}
//组装块，尝试更新进度
			select {
			case update <- struct{}{}:
			default:
			}

		case cont := <-wakeCh:
//头提取程序发送了一个继续标志，检查是否已完成
			if !cont {
				finished = true
			}
//邮件头到达，请尝试更新进度
			select {
			case update <- struct{}{}:
			default:
			}

		case <-ticker.C:
//健全检查更新进度
			select {
			case update <- struct{}{}:
			default:
			}

		case <-update:
//如果我们失去所有同龄人，就会短路
			if d.peers.Len() == 0 {
				return errNoPeers
			}
//检查获取请求超时并降级负责的对等方
			for pid, fails := range expire() {
				if peer := d.peers.Peer(pid); peer != nil {
//如果许多检索元素过期，我们可能高估了远程对等机，或者
//我们自己。只重置为最小吞吐量，但不要立即下降。即使是最短的时间
//在同步方面，我们需要摆脱同龄人。
//
//最小阈值为2的原因是下载程序试图估计带宽
//以及对等端的延迟，这需要稍微推一下度量容量并查看
//响应时间如何反应，对它总是要求一个以上的最小值（即最小2）。
					if fails > 2 {
						peer.log.Trace("Data delivery timed out", "type", kind)
						setIdle(peer, 0)
					} else {
						peer.log.Debug("Stalling delivery, dropping", "type", kind)
						if d.dropPeer == nil {
//当对本地副本使用“--copydb”时，droppeer方法为nil。
//如果压缩在错误的时间命中，则可能发生超时，并且可以忽略。
							peer.log.Warn("Downloader wants to drop peer, but peerdrop-function is not set", "peer", pid)
						} else {
							d.dropPeer(pid)
						}
					}
				}
			}
//如果没有其他东西可以获取，请等待或终止
			if pending() == 0 {
				if !inFlight() && finished {
					log.Debug("Data fetching completed", "type", kind)
					return nil
				}
				break
			}
//向所有空闲对等端发送下载请求，直到被阻止
			progressed, throttled, running := false, false, inFlight()
			idles, total := idle()

			for _, peer := range idles {
//节流启动时短路
				if throttle() {
					throttled = true
					break
				}
//如果没有更多可用任务，则短路。
				if pending() == 0 {
					break
				}
//为对等机保留一大块获取。一个零可以意味着
//没有更多的头可用，或者对等端已知不可用
//拥有它们。
				request, progress, err := reserve(peer, capacity(peer))
				if err != nil {
					return err
				}
				if progress {
					progressed = true
				}
				if request == nil {
					continue
				}
				if request.From > 0 {
					peer.log.Trace("Requesting new batch of data", "type", kind, "from", request.From)
				} else {
					peer.log.Trace("Requesting new batch of data", "type", kind, "count", len(request.Headers), "from", request.Headers[0].Number)
				}
//获取块并确保任何错误都将哈希返回到队列
				if fetchHook != nil {
					fetchHook(request.Headers)
				}
				if err := fetch(peer, request); err != nil {
//虽然我们可以尝试修复此错误，但实际上
//意味着我们已经将一个获取任务双重分配给了一个对等方。如果那是
//案例，下载器和队列的内部状态是非常错误的，所以
//更好的硬崩溃和注意错误，而不是默默地累积到
//更大的问题。
					panic(fmt.Sprintf("%v: %s fetch assignment failed", peer, kind))
				}
				running = true
			}
//确保我们有可供提取的对等点。如果所有同龄人都被试过
//所有的失败都会引发一个错误
			if !progressed && !throttled && !running && len(idles) == total && pending() > 0 {
				return errPeersUnavailable
			}
		}
	}
}

//processHeaders从输入通道获取一批检索到的头，并且
//继续处理并将它们调度到头链和下载程序中
//排队直到流结束或发生故障。
func (d *Downloader) processHeaders(origin uint64, pivot uint64, td *big.Int) error {
//保留不确定的头数以回滚
	rollback := []*types.Header{}
	defer func() {
		if len(rollback) > 0 {
//压平收割台并将其回滚
			hashes := make([]common.Hash, len(rollback))
			for i, header := range rollback {
				hashes[i] = header.Hash()
			}
			lastHeader, lastFastBlock, lastBlock := d.lightchain.CurrentHeader().Number, common.Big0, common.Big0
			if d.mode != LightSync {
				lastFastBlock = d.blockchain.CurrentFastBlock().Number()
				lastBlock = d.blockchain.CurrentBlock().Number()
			}
			d.lightchain.Rollback(hashes)
			curFastBlock, curBlock := common.Big0, common.Big0
			if d.mode != LightSync {
				curFastBlock = d.blockchain.CurrentFastBlock().Number()
				curBlock = d.blockchain.CurrentBlock().Number()
			}
			log.Warn("Rolled back headers", "count", len(hashes),
				"header", fmt.Sprintf("%d->%d", lastHeader, d.lightchain.CurrentHeader().Number),
				"fast", fmt.Sprintf("%d->%d", lastFastBlock, curFastBlock),
				"block", fmt.Sprintf("%d->%d", lastBlock, curBlock))
		}
	}()

//等待处理成批的邮件头
	gotHeaders := false

	for {
		select {
		case <-d.cancelCh:
			return errCancelHeaderProcessing

		case headers := <-d.headerProcCh:
//如果同步，则终止头处理
			if len(headers) == 0 {
//通知所有人邮件头已完全处理
				for _, ch := range []chan bool{d.bodyWakeCh, d.receiptWakeCh} {
					select {
					case ch <- false:
					case <-d.cancelCh:
					}
				}
//如果没有检索到任何头，则对等端违反了其td承诺，即
//链条比我们的好。唯一的例外是如果它承诺的块
//已通过其他方式导入（例如，获取器）：
//
//R<remote peer>，L<local node>：都在数据块10上
//R：我的11号区，然后传播到L
//L:队列块11用于导入
//L：注意R的头和TD比我们的要高，开始同步。
//L：11号块饰面的进口
//L:Sync开始，在11找到共同祖先
//L：从11点开始请求新的标题（R的td更高，它必须有一些东西）
//R：没什么可以给的
				if d.mode != LightSync {
					head := d.blockchain.CurrentBlock()
					if !gotHeaders && td.Cmp(d.blockchain.GetTd(head.Hash(), head.NumberU64())) > 0 {
						return errStallingPeer
					}
				}
//如果同步速度快或很轻，请确保确实交付了承诺的头文件。这是
//需要检测攻击者输入错误的轴然后诱饵离开的场景
//传递将标记无效内容的post-pivot块。
//
//由于块可能仍然是
//头下载完成后排队等待处理。但是，只要
//同行给了我们一些有用的东西，我们已经很高兴/进步了（上面的检查）。
				if d.mode == FastSync || d.mode == LightSync {
					head := d.lightchain.CurrentHeader()
					if td.Cmp(d.lightchain.GetTd(head.Hash(), head.Number.Uint64())) > 0 {
						return errStallingPeer
					}
				}
//禁用任何回滚并返回
				rollback = nil
				return nil
			}
//否则，将头块分割成批并处理它们
			gotHeaders = true

			for len(headers) > 0 {
//在处理块之间发生故障时终止
				select {
				case <-d.cancelCh:
					return errCancelHeaderProcessing
				default:
				}
//选择要导入的下一个标题块
				limit := maxHeadersProcess
				if limit > len(headers) {
					limit = len(headers)
				}
				chunk := headers[:limit]

//如果只同步头，请立即验证块。
				if d.mode == FastSync || d.mode == LightSync {
//收集尚未确定的邮件头，将其标记为不确定邮件头
					unknown := make([]*types.Header, 0, len(headers))
					for _, header := range chunk {
						if !d.lightchain.HasHeader(header.Hash(), header.Number.Uint64()) {
							unknown = append(unknown, header)
						}
					}
//如果我们要导入纯头，请根据它们的最近性进行验证。
					frequency := fsHeaderCheckFrequency
					if chunk[len(chunk)-1].Number.Uint64()+uint64(fsHeaderForceVerify) > pivot {
						frequency = 1
					}
					if n, err := d.lightchain.InsertHeaderChain(chunk, frequency); err != nil {
//如果插入了一些头，请将它们也添加到回滚列表中。
						if n > 0 {
							rollback = append(rollback, chunk[:n]...)
						}
						log.Debug("Invalid header encountered", "number", chunk[n].Number, "hash", chunk[n].Hash(), "err", err)
						return errInvalidChain
					}
//所有验证通过，存储新发现的不确定头
					rollback = append(rollback, unknown...)
					if len(rollback) > fsHeaderSafetyNet {
						rollback = append(rollback[:0], rollback[len(rollback)-fsHeaderSafetyNet:]...)
					}
				}
//除非我们在做轻链，否则请为相关的内容检索安排标题。
				if d.mode == FullSync || d.mode == FastSync {
//如果达到了允许的挂起头的数目，请暂停一点。
					for d.queue.PendingBlocks() >= maxQueuedHeaders || d.queue.PendingReceipts() >= maxQueuedHeaders {
						select {
						case <-d.cancelCh:
							return errCancelHeaderProcessing
						case <-time.After(time.Second):
						}
					}
//否则插入标题进行内容检索
					inserts := d.queue.Schedule(chunk, origin)
					if len(inserts) != len(chunk) {
						log.Debug("Stale headers")
						return errBadPeer
					}
				}
				headers = headers[limit:]
				origin += uint64(limit)
			}

//更新我们知道的最高块号，如果找到更高的块号。
			d.syncStatsLock.Lock()
			if d.syncStatsChainHeight < origin {
				d.syncStatsChainHeight = origin - 1
			}
			d.syncStatsLock.Unlock()

//向内容下载者发出新任务可用性的信号
			for _, ch := range []chan bool{d.bodyWakeCh, d.receiptWakeCh} {
				select {
				case ch <- true:
				default:
				}
			}
		}
	}
}

//processfullsyncContent从队列中获取结果并将其导入到链中。
func (d *Downloader) processFullSyncContent() error {
	for {
		results := d.queue.Results(true)
		if len(results) == 0 {
			return nil
		}
		if d.chainInsertHook != nil {
			d.chainInsertHook(results)
		}
		if err := d.importBlockResults(results); err != nil {
			return err
		}
	}
}

func (d *Downloader) importBlockResults(results []*fetchResult) error {
//检查是否有任何提前终止请求
	if len(results) == 0 {
		return nil
	}
	select {
	case <-d.quitCh:
		return errCancelContentProcessing
	default:
	}
//检索要导入的一批结果
	first, last := results[0].Header, results[len(results)-1].Header
	log.Debug("Inserting downloaded chain", "items", len(results),
		"firstnum", first.Number, "firsthash", first.Hash(),
		"lastnum", last.Number, "lasthash", last.Hash(),
	)
	blocks := make([]*types.Block, len(results))
	for i, result := range results {
		blocks[i] = types.NewBlockWithHeader(result.Header).WithBody(result.Transactions, result.Uncles)
	}
	if index, err := d.blockchain.InsertChain(blocks); err != nil {
		if index < len(results) {
			log.Debug("Downloaded item processing failed", "number", results[index].Header.Number, "hash", results[index].Header.Hash(), "err", err)
		} else {
//区块链.go中的insertchain方法有时会返回一个越界索引，
//当需要预处理块以导入侧链时。
//导入程序将汇总一个新的要导入的块列表，这是一个超集
//从下载器发送的块中，索引将关闭。
			log.Debug("Downloaded item processing failed on sidechain import", "index", index, "err", err)
		}
		return errInvalidChain
	}
	return nil
}

//processFastSyncContent从队列获取结果并将其写入
//数据库。它还控制枢轴块状态节点的同步。
func (d *Downloader) processFastSyncContent(latest *types.Header) error {
//开始同步报告的头块的状态。这应该让我们
//透视图块的状态。
	stateSync := d.syncState(latest.Root)
	defer stateSync.Cancel()
	go func() {
		if err := stateSync.Wait(); err != nil && err != errCancelStateFetch {
d.queue.Close() //唤醒结果
		}
	}()
//找出理想的轴块。注意，如果
//同步需要足够长的时间，链头才能显著移动。
	pivot := uint64(0)
	if height := latest.Number.Uint64(); height > uint64(fsMinFullBlocks) {
		pivot = height - uint64(fsMinFullBlocks)
	}
//为了适应移动的轴点，跟踪轴块，然后
//单独累计下载结果。
	var (
oldPivot *fetchResult   //锁定在轴块中，可能最终更改
oldTail  []*fetchResult //在透视之后下载的内容
	)
	for {
//等待下一批下载的数据可用，如果
//布洛克变僵了，移动门柱
results := d.queue.Results(oldPivot == nil) //如果我们不监视数据透视过时，请阻止
		if len(results) == 0 {
//如果透视同步完成，则停止
			if oldPivot == nil {
				return stateSync.Cancel()
			}
//如果同步失败，请停止
			select {
			case <-d.cancelCh:
				return stateSync.Cancel()
			default:
			}
		}
		if d.chainInsertHook != nil {
			d.chainInsertHook(results)
		}
		if oldPivot != nil {
			results = append(append([]*fetchResult{oldPivot}, oldTail...), results...)
		}
//围绕轴块拆分并通过快速/完全同步处理两侧
		if atomic.LoadInt32(&d.committed) == 0 {
			latest = results[len(results)-1].Header
			if height := latest.Number.Uint64(); height > pivot+2*uint64(fsMinFullBlocks) {
				log.Warn("Pivot became stale, moving", "old", pivot, "new", height-uint64(fsMinFullBlocks))
				pivot = height - uint64(fsMinFullBlocks)
			}
		}
		P, beforeP, afterP := splitAroundPivot(pivot, results)
		if err := d.commitFastSyncData(beforeP, stateSync); err != nil {
			return err
		}
		if P != nil {
//如果找到新的数据透视块，请取消旧的状态检索并重新启动
			if oldPivot != P {
				stateSync.Cancel()

				stateSync = d.syncState(P.Header.Root)
				defer stateSync.Cancel()
				go func() {
					if err := stateSync.Wait(); err != nil && err != errCancelStateFetch {
d.queue.Close() //唤醒结果
					}
				}()
				oldPivot = P
			}
//等待完成，偶尔检查数据透视是否过时
			select {
			case <-stateSync.done:
				if stateSync.err != nil {
					return stateSync.err
				}
				if err := d.commitPivotBlock(P); err != nil {
					return err
				}
				oldPivot = nil

			case <-time.After(time.Second):
				oldTail = afterP
				continue
			}
		}
//快速同步完成，透视提交完成，完全导入
		if err := d.importBlockResults(afterP); err != nil {
			return err
		}
	}
}

func splitAroundPivot(pivot uint64, results []*fetchResult) (p *fetchResult, before, after []*fetchResult) {
	for _, result := range results {
		num := result.Header.Number.Uint64()
		switch {
		case num < pivot:
			before = append(before, result)
		case num == pivot:
			p = result
		default:
			after = append(after, result)
		}
	}
	return p, before, after
}

func (d *Downloader) commitFastSyncData(results []*fetchResult, stateSync *stateSync) error {
//检查是否有任何提前终止请求
	if len(results) == 0 {
		return nil
	}
	select {
	case <-d.quitCh:
		return errCancelContentProcessing
	case <-stateSync.done:
		if err := stateSync.Wait(); err != nil {
			return err
		}
	default:
	}
//检索要导入的一批结果
	first, last := results[0].Header, results[len(results)-1].Header
	log.Debug("Inserting fast-sync blocks", "items", len(results),
		"firstnum", first.Number, "firsthash", first.Hash(),
		"lastnumn", last.Number, "lasthash", last.Hash(),
	)
	blocks := make([]*types.Block, len(results))
	receipts := make([]types.Receipts, len(results))
	for i, result := range results {
		blocks[i] = types.NewBlockWithHeader(result.Header).WithBody(result.Transactions, result.Uncles)
		receipts[i] = result.Receipts
	}
	if index, err := d.blockchain.InsertReceiptChain(blocks, receipts); err != nil {
		log.Debug("Downloaded item processing failed", "number", results[index].Header.Number, "hash", results[index].Header.Hash(), "err", err)
		return errInvalidChain
	}
	return nil
}

func (d *Downloader) commitPivotBlock(result *fetchResult) error {
	block := types.NewBlockWithHeader(result.Header).WithBody(result.Transactions, result.Uncles)
	log.Debug("Committing fast sync pivot as new head", "number", block.Number(), "hash", block.Hash())
	if _, err := d.blockchain.InsertReceiptChain([]*types.Block{block}, []types.Receipts{result.Receipts}); err != nil {
		return err
	}
	if err := d.blockchain.FastSyncCommitHead(block.Hash()); err != nil {
		return err
	}
	atomic.StoreInt32(&d.committed, 1)
	return nil
}

//DeliverHeaders插入从远程服务器接收的新批块头
//进入下载计划。
func (d *Downloader) DeliverHeaders(id string, headers []*types.Header) (err error) {
	return d.deliver(id, d.headerCh, &headerPack{id, headers}, headerInMeter, headerDropMeter)
}

//deliverbodies注入从远程节点接收的新批块体。
func (d *Downloader) DeliverBodies(id string, transactions [][]*types.Transaction, uncles [][]*types.Header) (err error) {
	return d.deliver(id, d.bodyCh, &bodyPack{id, transactions, uncles}, bodyInMeter, bodyDropMeter)
}

//DeliverReceipts插入从远程节点接收的新一批收据。
func (d *Downloader) DeliverReceipts(id string, receipts [][]*types.Receipt) (err error) {
	return d.deliver(id, d.receiptCh, &receiptPack{id, receipts}, receiptInMeter, receiptDropMeter)
}

//DeliverNodeData注入从远程节点接收到的新一批节点状态数据。
func (d *Downloader) DeliverNodeData(id string, data [][]byte) (err error) {
	return d.deliver(id, d.stateCh, &statePack{id, data}, stateInMeter, stateDropMeter)
}

//deliver注入从远程节点接收的新批数据。
func (d *Downloader) deliver(id string, destCh chan dataPack, packet dataPack, inMeter, dropMeter metrics.Meter) (err error) {
//更新好交付和失败交付的交付指标
	inMeter.Mark(int64(packet.Items()))
	defer func() {
		if err != nil {
			dropMeter.Mark(int64(packet.Items()))
		}
	}()
//如果在排队时取消同步，则传递或中止
	d.cancelLock.RLock()
	cancel := d.cancelCh
	d.cancelLock.RUnlock()
	if cancel == nil {
		return errNoSyncActive
	}
	select {
	case destCh <- packet:
		return nil
	case <-cancel:
		return errNoSyncActive
	}
}

//Qostener是服务质量优化循环，偶尔收集
//对等延迟统计并更新估计的请求往返时间。
func (d *Downloader) qosTuner() {
	for {
//检索当前中间RTT并集成到以前的目标RTT中
		rtt := time.Duration((1-qosTuningImpact)*float64(atomic.LoadUint64(&d.rttEstimate)) + qosTuningImpact*float64(d.peers.medianRTT()))
		atomic.StoreUint64(&d.rttEstimate, uint64(rtt))

//通过了一个新的RTT周期，增加了我们对估计的RTT的信心。
		conf := atomic.LoadUint64(&d.rttConfidence)
		conf = conf + (1000000-conf)/2
		atomic.StoreUint64(&d.rttConfidence, conf)

//记录新的QoS值并休眠到下一个RTT
		log.Debug("Recalculated downloader QoS values", "rtt", rtt, "confidence", float64(conf)/1000000.0, "ttl", d.requestTTL())
		select {
		case <-d.quitCh:
			return
		case <-time.After(rtt):
		}
	}
}

//QosReduceConfidence是指当新对等加入下载程序时调用的。
//对等集，需要降低我们对QoS估计的信心。
func (d *Downloader) qosReduceConfidence() {
//如果我们只有一个同伴，那么信心总是1
	peers := uint64(d.peers.Len())
	if peers == 0 {
//确保对等连接竞赛不会让我们措手不及
		return
	}
	if peers == 1 {
		atomic.StoreUint64(&d.rttConfidence, 1000000)
		return
	}
//如果我们有很多同龄人，不要放弃信心）
	if peers >= uint64(qosConfidenceCap) {
		return
	}
//否则，降低置信系数
	conf := atomic.LoadUint64(&d.rttConfidence) * (peers - 1) / peers
	if float64(conf)/1000000 < rttMinConfidence {
		conf = uint64(rttMinConfidence * 1000000)
	}
	atomic.StoreUint64(&d.rttConfidence, conf)

	rtt := time.Duration(atomic.LoadUint64(&d.rttEstimate))
	log.Debug("Relaxed downloader QoS values", "rtt", rtt, "confidence", float64(conf)/1000000.0, "ttl", d.requestTTL())
}

//requestrtt返回下载请求的当前目标往返时间
//完成。
//
//注意，返回的RTT是实际估计RTT的.9。原因是
//下载程序尝试使查询适应RTT，因此多个RTT值可以
//适应，但较小的是首选（更稳定的下载流）。
func (d *Downloader) requestRTT() time.Duration {
	return time.Duration(atomic.LoadUint64(&d.rttEstimate)) * 9 / 10
}

//REQUESTTL返回单个下载请求的当前超时允许值
//在…之下完成。
func (d *Downloader) requestTTL() time.Duration {
	var (
		rtt  = time.Duration(atomic.LoadUint64(&d.rttEstimate))
		conf = float64(atomic.LoadUint64(&d.rttConfidence)) / 1000000.0
	)
	ttl := time.Duration(ttlScaling) * time.Duration(float64(rtt)/conf)
	if ttl > ttlLimit {
		ttl = ttlLimit
	}
	return ttl
}
