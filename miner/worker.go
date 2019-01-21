
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

package miner

import (
	"bytes"
	"errors"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

const (
//结果QueueSize为监听密封结果的通道大小。
	resultQueueSize = 10

//txchanSize是侦听newtxSevent的频道的大小。
//该数字是根据Tx池的大小引用的。
	txChanSize = 4096

//ChainHeadChansize是侦听ChainHeadEvent的通道的大小。
	chainHeadChanSize = 10

//ChainsideChansize是侦听ChainsideEvent的通道的大小。
	chainSideChanSize = 10

//SubmitadjustChansize是重新提交间隔调整通道的大小。
	resubmitAdjustChanSize = 10

//MiningLogAtDepth是记录成功挖掘之前的确认数。
	miningLogAtDepth = 7

//MinRecommitInterval是重新创建挖掘块所用的最小时间间隔。
//任何新到的交易。
	minRecommitInterval = 1 * time.Second

//MaxRecommitInterval是重新创建挖掘块所用的最大时间间隔
//任何新到的交易。
	maxRecommitInterval = 15 * time.Second

//间隙调整是单个间隙调整对密封工作的影响。
//重新提交间隔。
	intervalAdjustRatio = 0.1

//在新的重新提交间隔计算期间应用IntervalAdjustBias，有利于
//增大上限或减小下限，以便可以达到上限。
	intervalAdjustBias = 200 * 1000.0 * 1000.0

//StaleThreshold是可接受的Stale块的最大深度。
	staleThreshold = 7
)

//环境是工作人员的当前环境，保存所有当前状态信息。
type environment struct {
	signer types.Signer

state     *state.StateDB //在此应用状态更改
ancestors mapset.Set     //祖先集（用于检查叔叔父级有效性）
family    mapset.Set     //家庭设置（用于检查叔叔的无效性）
uncles    mapset.Set     //叔叔集
tcount    int            //周期中的Tx计数
gasPool   *core.GasPool  //用于包装交易的可用气体

	header   *types.Header
	txs      []*types.Transaction
	receipts []*types.Receipt
}

//任务包含共识引擎密封和结果提交的所有信息。
type task struct {
	receipts  []*types.Receipt
	state     *state.StateDB
	block     *types.Block
	createdAt time.Time
}

const (
	commitInterruptNone int32 = iota
	commitInterruptNewHead
	commitInterruptResubmit
)

//newworkreq表示使用相关中断通知程序提交新密封工作的请求。
type newWorkReq struct {
	interrupt *int32
	noempty   bool
	timestamp int64
}

//IntervalAdjust表示重新提交的间隔调整。
type intervalAdjust struct {
	ratio float64
	inc   bool
}

//工作人员是负责向共识引擎提交新工作的主要对象。
//收集密封结果。
type worker struct {
	config *params.ChainConfig
	engine consensus.Engine
	eth    Backend
	chain  *core.BlockChain

	gasFloor uint64
	gasCeil  uint64

//订阅
	mux          *event.TypeMux
	txsCh        chan core.NewTxsEvent
	txsSub       event.Subscription
	chainHeadCh  chan core.ChainHeadEvent
	chainHeadSub event.Subscription
	chainSideCh  chan core.ChainSideEvent
	chainSideSub event.Subscription

//渠道
	newWorkCh          chan *newWorkReq
	taskCh             chan *task
	resultCh           chan *types.Block
	startCh            chan struct{}
	exitCh             chan struct{}
	resubmitIntervalCh chan time.Duration
	resubmitAdjustCh   chan *intervalAdjust

current      *environment                 //当前运行周期的环境。
localUncles  map[common.Hash]*types.Block //局部生成的一组边块，作为可能的叔叔块。
remoteUncles map[common.Hash]*types.Block //一组侧块作为可能的叔叔块。
unconfirmed  *unconfirmedBlocks           //一组本地挖掘的块，等待规范性确认。

mu       sync.RWMutex //用于保护coinbase和额外字段的锁
	coinbase common.Address
	extra    []byte

	pendingMu    sync.RWMutex
	pendingTasks map[common.Hash]*task

snapshotMu    sync.RWMutex //用于保护块快照和状态快照的锁
	snapshotBlock *types.Block
	snapshotState *state.StateDB

//原子状态计数器
running int32 //共识引擎是否运行的指示器。
newTxs  int32 //自上次密封工作提交以来的新到达交易记录计数。

//外部功能
isLocalBlock func(block *types.Block) bool //用于确定指定块是否由本地矿工挖掘的函数。

//测试钩
newTaskHook  func(*task)                        //方法在收到新的密封任务时调用。
skipSealHook func(*task) bool                   //方法来决定是否跳过密封。
fullTaskHook func()                             //方法在执行完全密封任务之前调用。
resubmitHook func(time.Duration, time.Duration) //更新重新提交间隔时调用的方法。
}

func newWorker(config *params.ChainConfig, engine consensus.Engine, eth Backend, mux *event.TypeMux, recommit time.Duration, gasFloor, gasCeil uint64, isLocalBlock func(*types.Block) bool) *worker {
	worker := &worker{
		config:             config,
		engine:             engine,
		eth:                eth,
		mux:                mux,
		chain:              eth.BlockChain(),
		gasFloor:           gasFloor,
		gasCeil:            gasCeil,
		isLocalBlock:       isLocalBlock,
		localUncles:        make(map[common.Hash]*types.Block),
		remoteUncles:       make(map[common.Hash]*types.Block),
		unconfirmed:        newUnconfirmedBlocks(eth.BlockChain(), miningLogAtDepth),
		pendingTasks:       make(map[common.Hash]*task),
		txsCh:              make(chan core.NewTxsEvent, txChanSize),
		chainHeadCh:        make(chan core.ChainHeadEvent, chainHeadChanSize),
		chainSideCh:        make(chan core.ChainSideEvent, chainSideChanSize),
		newWorkCh:          make(chan *newWorkReq),
		taskCh:             make(chan *task),
		resultCh:           make(chan *types.Block, resultQueueSize),
		exitCh:             make(chan struct{}),
		startCh:            make(chan struct{}, 1),
		resubmitIntervalCh: make(chan time.Duration),
		resubmitAdjustCh:   make(chan *intervalAdjust, resubmitAdjustChanSize),
	}
//订阅Tx池的NewTxSevent
	worker.txsSub = eth.TxPool().SubscribeNewTxsEvent(worker.txsCh)
//为区块链订阅事件
	worker.chainHeadSub = eth.BlockChain().SubscribeChainHeadEvent(worker.chainHeadCh)
	worker.chainSideSub = eth.BlockChain().SubscribeChainSideEvent(worker.chainSideCh)

//如果用户指定的重新提交间隔太短，则清除重新提交间隔。
	if recommit < minRecommitInterval {
		log.Warn("Sanitizing miner recommit interval", "provided", recommit, "updated", minRecommitInterval)
		recommit = minRecommitInterval
	}

	go worker.mainLoop()
	go worker.newWorkLoop(recommit)
	go worker.resultLoop()
	go worker.taskLoop()

//提交第一个工作以初始化挂起状态。
	worker.startCh <- struct{}{}

	return worker
}

//setetherbase设置用于初始化块coinbase字段的etherbase。
func (w *worker) setEtherbase(addr common.Address) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.coinbase = addr
}

//setextra设置用于初始化块额外字段的内容。
func (w *worker) setExtra(extra []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.extra = extra
}

//setrecommittinterval更新矿工密封工作重新投入的时间间隔。
func (w *worker) setRecommitInterval(interval time.Duration) {
	w.resubmitIntervalCh <- interval
}

//Pending返回Pending状态和相应的块。
func (w *worker) pending() (*types.Block, *state.StateDB) {
//返回快照以避免对当前mutex的争用
	w.snapshotMu.RLock()
	defer w.snapshotMu.RUnlock()
	if w.snapshotState == nil {
		return nil, nil
	}
	return w.snapshotBlock, w.snapshotState.Copy()
}

//PendingBlock返回PendingBlock。
func (w *worker) pendingBlock() *types.Block {
//返回快照以避免对当前mutex的争用
	w.snapshotMu.RLock()
	defer w.snapshotMu.RUnlock()
	return w.snapshotBlock
}

//Start将运行状态设置为1并触发新工作提交。
func (w *worker) start() {
	atomic.StoreInt32(&w.running, 1)
	w.startCh <- struct{}{}
}

//stop将运行状态设置为0。
func (w *worker) stop() {
	atomic.StoreInt32(&w.running, 0)
}

//is running返回一个指示工作者是否正在运行的指示器。
func (w *worker) isRunning() bool {
	return atomic.LoadInt32(&w.running) == 1
}

//CLOSE终止工作线程维护的所有后台线程。
//注意工人不支持多次关闭。
func (w *worker) close() {
	close(w.exitCh)
}

//newworkoop是一个独立的goroutine，用于在收到事件时提交新的挖掘工作。
func (w *worker) newWorkLoop(recommit time.Duration) {
	var (
		interrupt   *int32
minRecommit = recommit //用户指定的最小重新提交间隔。
timestamp   int64      //每轮挖掘的时间戳。
	)

	timer := time.NewTimer(0)
<-timer.C //放弃初始勾号

//commit使用给定的信号中止正在执行的事务，并重新提交一个新的事务。
	commit := func(noempty bool, s int32) {
		if interrupt != nil {
			atomic.StoreInt32(interrupt, s)
		}
		interrupt = new(int32)
		w.newWorkCh <- &newWorkReq{interrupt: interrupt, noempty: noempty, timestamp: timestamp}
		timer.Reset(recommit)
		atomic.StoreInt32(&w.newTxs, 0)
	}
//重新计算提交根据反馈重新计算重新提交间隔。
	recalcRecommit := func(target float64, inc bool) {
		var (
			prev = float64(recommit.Nanoseconds())
			next float64
		)
		if inc {
			next = prev*(1-intervalAdjustRatio) + intervalAdjustRatio*(target+intervalAdjustBias)
//回顾间隔是否大于最大时间间隔
			if next > float64(maxRecommitInterval.Nanoseconds()) {
				next = float64(maxRecommitInterval.Nanoseconds())
			}
		} else {
			next = prev*(1-intervalAdjustRatio) + intervalAdjustRatio*(target-intervalAdjustBias)
//如果间隔小于用户指定的最小值，则重述
			if next < float64(minRecommit.Nanoseconds()) {
				next = float64(minRecommit.Nanoseconds())
			}
		}
		recommit = time.Duration(int64(next))
	}
//ClearPending清除过时的挂起任务。
	clearPending := func(number uint64) {
		w.pendingMu.Lock()
		for h, t := range w.pendingTasks {
			if t.block.NumberU64()+staleThreshold <= number {
				delete(w.pendingTasks, h)
			}
		}
		w.pendingMu.Unlock()
	}

	for {
		select {
		case <-w.startCh:
			clearPending(w.chain.CurrentBlock().NumberU64())
			timestamp = time.Now().Unix()
			commit(false, commitInterruptNewHead)

		case head := <-w.chainHeadCh:
			clearPending(head.Block.NumberU64())
			timestamp = time.Now().Unix()
			commit(false, commitInterruptNewHead)

		case <-timer.C:
//如果正在运行挖掘，请定期重新提交新的工作周期以拉入
//高价交易。对挂起的块禁用此开销。
			if w.isRunning() && (w.config.Clique == nil || w.config.Clique.Period > 0) {
//如果没有新交易到达，则短路。
				if atomic.LoadInt32(&w.newTxs) == 0 {
					timer.Reset(recommit)
					continue
				}
				commit(true, commitInterruptResubmit)
			}

		case interval := <-w.resubmitIntervalCh:
//按用户明确调整重新提交间隔。
			if interval < minRecommitInterval {
				log.Warn("Sanitizing miner recommit interval", "provided", interval, "updated", minRecommitInterval)
				interval = minRecommitInterval
			}
			log.Info("Miner recommit interval update", "from", minRecommit, "to", interval)
			minRecommit, recommit = interval, interval

			if w.resubmitHook != nil {
				w.resubmitHook(minRecommit, recommit)
			}

		case adjust := <-w.resubmitAdjustCh:
//通过反馈调整重新提交间隔。
			if adjust.inc {
				before := recommit
				recalcRecommit(float64(recommit.Nanoseconds())/adjust.ratio, true)
				log.Trace("Increase miner recommit interval", "from", before, "to", recommit)
			} else {
				before := recommit
				recalcRecommit(float64(minRecommit.Nanoseconds()), false)
				log.Trace("Decrease miner recommit interval", "from", before, "to", recommit)
			}

			if w.resubmitHook != nil {
				w.resubmitHook(minRecommit, recommit)
			}

		case <-w.exitCh:
			return
		}
	}
}

//mainLoop是一个独立的goroutine，用于根据接收到的事件重新生成密封任务。
func (w *worker) mainLoop() {
	defer w.txsSub.Unsubscribe()
	defer w.chainHeadSub.Unsubscribe()
	defer w.chainSideSub.Unsubscribe()

	for {
		select {
		case req := <-w.newWorkCh:
			w.commitNewWork(req.interrupt, req.noempty, req.timestamp)

		case ev := <-w.chainSideCh:
//重复侧块短路
			if _, exist := w.localUncles[ev.Block.Hash()]; exist {
				continue
			}
			if _, exist := w.remoteUncles[ev.Block.Hash()]; exist {
				continue
			}
//根据作者，将侧块添加到可能的叔叔块集。
			if w.isLocalBlock != nil && w.isLocalBlock(ev.Block) {
				w.localUncles[ev.Block.Hash()] = ev.Block
			} else {
				w.remoteUncles[ev.Block.Hash()] = ev.Block
			}
//如果我们的采矿区块少于2个叔叔区块，
//添加新的叔叔块（如果有效）并重新生成挖掘块。
			if w.isRunning() && w.current != nil && w.current.uncles.Cardinality() < 2 {
				start := time.Now()
				if err := w.commitUncle(w.current, ev.Block.Header()); err == nil {
					var uncles []*types.Header
					w.current.uncles.Each(func(item interface{}) bool {
						hash, ok := item.(common.Hash)
						if !ok {
							return false
						}
						uncle, exist := w.localUncles[hash]
						if !exist {
							uncle, exist = w.remoteUncles[hash]
						}
						if !exist {
							return false
						}
						uncles = append(uncles, uncle.Header())
						return false
					})
					w.commit(uncles, nil, true, start)
				}
			}

		case ev := <-w.txsCh:
//如果不挖掘，将事务应用于挂起状态。
//
//注意：收到的所有交易可能与交易不连续。
//已包含在当前挖掘块中。这些交易将
//自动消除。
			if !w.isRunning() && w.current != nil {
				w.mu.RLock()
				coinbase := w.coinbase
				w.mu.RUnlock()

				txs := make(map[common.Address]types.Transactions)
				for _, tx := range ev.Txs {
					acc, _ := types.Sender(w.current.signer, tx)
					txs[acc] = append(txs[acc], tx)
				}
				txset := types.NewTransactionsByPriceAndNonce(w.current.signer, txs)
				w.commitTransactions(txset, coinbase, nil)
				w.updateSnapshot()
			} else {
//如果我们正在挖掘，但没有处理任何事务，请唤醒新事务
				if w.config.Clique != nil && w.config.Clique.Period == 0 {
					w.commitNewWork(nil, false, time.Now().Unix())
				}
			}
			atomic.AddInt32(&w.newTxs, int32(len(ev.Txs)))

//系统停止
		case <-w.exitCh:
			return
		case <-w.txsSub.Err():
			return
		case <-w.chainHeadSub.Err():
			return
		case <-w.chainSideSub.Err():
			return
		}
	}
}

//taskloop是一个独立的goroutine，用于从生成器获取密封任务，并且
//把他们推到共识引擎。
func (w *worker) taskLoop() {
	var (
		stopCh chan struct{}
		prev   common.Hash
	)

//中断中止飞行中的密封任务。
	interrupt := func() {
		if stopCh != nil {
			close(stopCh)
			stopCh = nil
		}
	}
	for {
		select {
		case task := <-w.taskCh:
			if w.newTaskHook != nil {
				w.newTaskHook(task)
			}
//因重新提交而拒绝重复密封工作。
			sealHash := w.engine.SealHash(task.block.Header())
			if sealHash == prev {
				continue
			}
//中断之前的密封操作
			interrupt()
			stopCh, prev = make(chan struct{}), sealHash

			if w.skipSealHook != nil && w.skipSealHook(task) {
				continue
			}
			w.pendingMu.Lock()
			w.pendingTasks[w.engine.SealHash(task.block.Header())] = task
			w.pendingMu.Unlock()

			if err := w.engine.Seal(w.chain, task.block, w.resultCh, stopCh); err != nil {
				log.Warn("Block sealing failed", "err", err)
			}
		case <-w.exitCh:
			interrupt()
			return
		}
	}
}

//resultLoop是一个独立的goroutine，用于处理密封结果提交
//并将相关数据刷新到数据库。
func (w *worker) resultLoop() {
	for {
		select {
		case block := <-w.resultCh:
//收到空结果时短路。
			if block == nil {
				continue
			}
//由于重新提交而收到重复结果时短路。
			if w.chain.HasBlock(block.Hash(), block.NumberU64()) {
				continue
			}
			var (
				sealhash = w.engine.SealHash(block.Header())
				hash     = block.Hash()
			)
			w.pendingMu.RLock()
			task, exist := w.pendingTasks[sealhash]
			w.pendingMu.RUnlock()
			if !exist {
				log.Error("Block found but no relative pending task", "number", block.Number(), "sealhash", sealhash, "hash", hash)
				continue
			}
//不同的块可以共享相同的sealhash，在这里进行深度复制以防止写写冲突。
			var (
				receipts = make([]*types.Receipt, len(task.receipts))
				logs     []*types.Log
			)
			for i, receipt := range task.receipts {
				receipts[i] = new(types.Receipt)
				*receipts[i] = *receipt
//更新所有日志中的块哈希，因为它现在可用，而不是
//已创建单个交易的收据/日志。
				for _, log := range receipt.Logs {
					log.BlockHash = hash
				}
				logs = append(logs, receipt.Logs...)
			}
//将块和状态提交到数据库。
			stat, err := w.chain.WriteBlockWithState(block, receipts, task.state)
			if err != nil {
				log.Error("Failed writing block to chain", "err", err)
				continue
			}
			log.Info("Successfully sealed new block", "number", block.Number(), "sealhash", sealhash, "hash", hash,
				"elapsed", common.PrettyDuration(time.Since(task.createdAt)))

//广播块并宣布链插入事件
			w.mux.Post(core.NewMinedBlockEvent{Block: block})

			var events []interface{}
			switch stat {
			case core.CanonStatTy:
				events = append(events, core.ChainEvent{Block: block, Hash: block.Hash(), Logs: logs})
				events = append(events, core.ChainHeadEvent{Block: block})
			case core.SideStatTy:
				events = append(events, core.ChainSideEvent{Block: block})
			}
			w.chain.PostChainEvents(events, logs)

//将块插入一组挂起的结果循环以进行确认
			w.unconfirmed.Insert(block.NumberU64(), block.Hash())

		case <-w.exitCh:
			return
		}
	}
}

//makecurrent为当前循环创建新环境。
func (w *worker) makeCurrent(parent *types.Block, header *types.Header) error {
	state, err := w.chain.StateAt(parent.Root())
	if err != nil {
		return err
	}
	env := &environment{
		signer:    types.NewEIP155Signer(w.config.ChainID),
		state:     state,
		ancestors: mapset.NewSet(),
		family:    mapset.NewSet(),
		uncles:    mapset.NewSet(),
		header:    header,
	}

//处理08时，祖先包含07（快速块）
	for _, ancestor := range w.chain.GetBlocksFromHash(parent.Hash(), 7) {
		for _, uncle := range ancestor.Uncles() {
			env.family.Add(uncle.Hash())
		}
		env.family.Add(ancestor.Hash())
		env.ancestors.Add(ancestor.Hash())
	}

//跟踪返回错误的事务，以便删除它们
	env.tcount = 0
	w.current = env
	return nil
}

//commituncle将给定的块添加到叔叔块集，如果添加失败则返回错误。
func (w *worker) commitUncle(env *environment, uncle *types.Header) error {
	hash := uncle.Hash()
	if env.uncles.Contains(hash) {
		return errors.New("uncle not unique")
	}
	if env.header.ParentHash == uncle.ParentHash {
		return errors.New("uncle is sibling")
	}
	if !env.ancestors.Contains(uncle.ParentHash) {
		return errors.New("uncle's parent unknown")
	}
	if env.family.Contains(hash) {
		return errors.New("uncle already included")
	}
	env.uncles.Add(uncle.Hash())
	return nil
}

//更新快照更新挂起的快照块和状态。
//注意：此函数假定当前变量是线程安全的。
func (w *worker) updateSnapshot() {
	w.snapshotMu.Lock()
	defer w.snapshotMu.Unlock()

	var uncles []*types.Header
	w.current.uncles.Each(func(item interface{}) bool {
		hash, ok := item.(common.Hash)
		if !ok {
			return false
		}
		uncle, exist := w.localUncles[hash]
		if !exist {
			uncle, exist = w.remoteUncles[hash]
		}
		if !exist {
			return false
		}
		uncles = append(uncles, uncle.Header())
		return false
	})

	w.snapshotBlock = types.NewBlock(
		w.current.header,
		w.current.txs,
		uncles,
		w.current.receipts,
	)

	w.snapshotState = w.current.state.Copy()
}

func (w *worker) commitTransaction(tx *types.Transaction, coinbase common.Address) ([]*types.Log, error) {
	snap := w.current.state.Snapshot()

	receipt, _, err := core.ApplyTransaction(w.config, w.chain, &coinbase, w.current.gasPool, w.current.state, w.current.header, tx, &w.current.header.GasUsed, *w.chain.GetVMConfig())
	if err != nil {
		w.current.state.RevertToSnapshot(snap)
		return nil, err
	}
	w.current.txs = append(w.current.txs, tx)
	w.current.receipts = append(w.current.receipts, receipt)

	return receipt.Logs, nil
}

func (w *worker) commitTransactions(txs *types.TransactionsByPriceAndNonce, coinbase common.Address, interrupt *int32) bool {
//电流为零时短路
	if w.current == nil {
		return true
	}

	if w.current.gasPool == nil {
		w.current.gasPool = new(core.GasPool).AddGas(w.current.header.GasLimit)
	}

	var coalescedLogs []*types.Log

	for {
//在以下三种情况下，我们将中断事务的执行。
//（1）新的头块事件到达，中断信号为1
//（2）工人启动或重启，中断信号为1
//（3）工人用任何新到达的事务重新创建挖掘块，中断信号为2。
//前两种情况下，半成品将被丢弃。
//对于第三种情况，半成品将提交给共识引擎。
		if interrupt != nil && atomic.LoadInt32(interrupt) != commitInterruptNone {
//由于提交太频繁，通知重新提交循环以增加重新提交间隔。
			if atomic.LoadInt32(interrupt) == commitInterruptResubmit {
				ratio := float64(w.current.header.GasLimit-w.current.gasPool.Gas()) / float64(w.current.header.GasLimit)
				if ratio < 0.1 {
					ratio = 0.1
				}
				w.resubmitAdjustCh <- &intervalAdjust{
					ratio: ratio,
					inc:   true,
				}
			}
			return atomic.LoadInt32(interrupt) == commitInterruptNewHead
		}
//如果我们没有足够的汽油进行进一步的交易，那么我们就完成了
		if w.current.gasPool.Gas() < params.TxGas {
			log.Trace("Not enough gas for further transactions", "have", w.current.gasPool, "want", params.TxGas)
			break
		}
//检索下一个事务，完成后中止
		tx := txs.Peek()
		if tx == nil {
			break
		}
//此处可以忽略错误。已检查错误
//在事务接受期间是事务池。
//
//我们使用EIP155签名者，不管当前的高频。
		from, _ := types.Sender(w.current.signer, tx)
//检查Tx是否受重播保护。如果我们不在EIP155高频
//阶段，开始忽略发送者，直到我们这样做。
		if tx.Protected() && !w.config.IsEIP155(w.current.header.Number) {
			log.Trace("Ignoring reply protected transaction", "hash", tx.Hash(), "eip155", w.config.EIP155Block)

			txs.Pop()
			continue
		}
//开始执行事务
		w.current.state.Prepare(tx.Hash(), common.Hash{}, w.current.tcount)

		logs, err := w.commitTransaction(tx, coinbase)
		switch err {
		case core.ErrGasLimitReached:
//从账户中弹出当前的天然气外交易，而不在下一个账户中移动。
			log.Trace("Gas limit exceeded for current block", "sender", from)
			txs.Pop()

		case core.ErrNonceTooLow:
//事务池和矿工之间的新头通知数据竞赛，shift
			log.Trace("Skipping transaction with low nonce", "sender", from, "nonce", tx.Nonce())
			txs.Shift()

		case core.ErrNonceTooHigh:
//交易池和矿工之间的REORG通知数据竞赛，跳过帐户=
			log.Trace("Skipping account with hight nonce", "sender", from, "nonce", tx.Nonce())
			txs.Pop()

		case nil:
//一切正常，收集日志并从同一帐户转入下一个事务
			coalescedLogs = append(coalescedLogs, logs...)
			w.current.tcount++
			txs.Shift()

		default:
//奇怪的错误，丢弃事务并将下一个事务处理到行中（注意，
//nonce-too-high子句将阻止我们无效执行）。
			log.Debug("Transaction failed, account skipped", "hash", tx.Hash(), "err", err)
			txs.Shift()
		}
	}

	if !w.isRunning() && len(coalescedLogs) > 0 {
//我们在采矿时不会推悬垂的日志。原因是
//当我们进行挖掘时，工人将每3秒重新生成一个挖掘块。
//为了避免推送重复的挂起日志，我们禁用挂起日志推送。

//复制，状态缓存日志，这些日志从挂起升级到挖掘。
//当块由当地矿工开采时，通过填充块散列进行记录。这个罐头
//如果在处理PendingLogSevent之前日志已“升级”，则会导致竞争条件。
		cpy := make([]*types.Log, len(coalescedLogs))
		for i, l := range coalescedLogs {
			cpy[i] = new(types.Log)
			*cpy[i] = *l
		}
		go w.mux.Post(core.PendingLogsEvent{Logs: cpy})
	}
//如果当前间隔较大，通知重新提交循环以减少重新提交间隔
//而不是用户指定的。
	if interrupt != nil {
		w.resubmitAdjustCh <- &intervalAdjust{inc: false}
	}
	return false
}

//CommitnewWork基于父块生成几个新的密封任务。
func (w *worker) commitNewWork(interrupt *int32, noempty bool, timestamp int64) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	tstart := time.Now()
	parent := w.chain.CurrentBlock()

	if parent.Time().Cmp(new(big.Int).SetInt64(timestamp)) >= 0 {
		timestamp = parent.Time().Int64() + 1
	}
//这将确保我们今后不会走得太远。
	if now := time.Now().Unix(); timestamp > now+1 {
		wait := time.Duration(timestamp-now) * time.Second
		log.Info("Mining too far in the future", "wait", common.PrettyDuration(wait))
		time.Sleep(wait)
	}

	num := parent.Number()
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, common.Big1),
		GasLimit:   core.CalcGasLimit(parent, w.gasFloor, w.gasCeil),
		Extra:      w.extra,
		Time:       big.NewInt(timestamp),
	}
//只有在我们的共识引擎运行时才设置coinbase（避免虚假的块奖励）
	if w.isRunning() {
		if w.coinbase == (common.Address{}) {
			log.Error("Refusing to mine without etherbase")
			return
		}
		header.Coinbase = w.coinbase
	}
	if err := w.engine.Prepare(w.chain, header); err != nil {
		log.Error("Failed to prepare header for mining", "err", err)
		return
	}
//如果我们关心DAO硬分叉，请检查是否覆盖额外的数据
	if daoBlock := w.config.DAOForkBlock; daoBlock != nil {
//检查块是否在fork额外覆盖范围内
		limit := new(big.Int).Add(daoBlock, params.DAOForkExtraRange)
		if header.Number.Cmp(daoBlock) >= 0 && header.Number.Cmp(limit) < 0 {
//根据我们是支持还是反对叉子，以不同的方式超越
			if w.config.DAOForkSupport {
				header.Extra = common.CopyBytes(params.DAOForkBlockExtra)
			} else if bytes.Equal(header.Extra, params.DAOForkBlockExtra) {
header.Extra = []byte{} //如果Miner反对，不要让它使用保留的额外数据
			}
		}
	}
//如果在一个奇怪的状态下开始挖掘，可能会发生这种情况。
	err := w.makeCurrent(parent, header)
	if err != nil {
		log.Error("Failed to create mining context", "err", err)
		return
	}
//创建当前工作任务并检查所需的任何分叉转换
	env := w.current
	if w.config.DAOForkSupport && w.config.DAOForkBlock != nil && w.config.DAOForkBlock.Cmp(header.Number) == 0 {
		misc.ApplyDAOHardFork(env.state)
	}
//累积当前块的叔叔
	uncles := make([]*types.Header, 0, 2)
	commitUncles := func(blocks map[common.Hash]*types.Block) {
//先清理陈旧的叔叔街区
		for hash, uncle := range blocks {
			if uncle.NumberU64()+staleThreshold <= header.Number.Uint64() {
				delete(blocks, hash)
			}
		}
		for hash, uncle := range blocks {
			if len(uncles) == 2 {
				break
			}
			if err := w.commitUncle(env, uncle.Header()); err != nil {
				log.Trace("Possible uncle rejected", "hash", hash, "reason", err)
			} else {
				log.Debug("Committing new uncle to block", "hash", hash)
				uncles = append(uncles, uncle.Header())
			}
		}
	}
//更喜欢本地生成的叔叔
	commitUncles(w.localUncles)
	commitUncles(w.remoteUncles)

	if !noempty {
//基于临时复制状态创建一个空块，在不等待块的情况下提前密封
//执行完成。
		w.commit(uncles, nil, false, tstart)
	}

//用所有可用的挂起事务填充块。
	pending, err := w.eth.TxPool().Pending()
	if err != nil {
		log.Error("Failed to fetch pending transactions", "err", err)
		return
	}
//如果没有可用的挂起事务，则短路
	if len(pending) == 0 {
		w.updateSnapshot()
		return
	}
//将挂起的事务拆分为本地和远程
	localTxs, remoteTxs := make(map[common.Address]types.Transactions), pending
	for _, account := range w.eth.TxPool().Locals() {
		if txs := remoteTxs[account]; len(txs) > 0 {
			delete(remoteTxs, account)
			localTxs[account] = txs
		}
	}
	if len(localTxs) > 0 {
		txs := types.NewTransactionsByPriceAndNonce(w.current.signer, localTxs)
		if w.commitTransactions(txs, w.coinbase, interrupt) {
			return
		}
	}
	if len(remoteTxs) > 0 {
		txs := types.NewTransactionsByPriceAndNonce(w.current.signer, remoteTxs)
		if w.commitTransactions(txs, w.coinbase, interrupt) {
			return
		}
	}
	w.commit(uncles, w.fullTaskHook, true, tstart)
}

//commit运行任何事务后状态修改，组装最终块
//如果共识引擎正在运行，则提交新的工作。
func (w *worker) commit(uncles []*types.Header, interval func(), update bool, start time.Time) error {
//在此深度复制收据以避免不同任务之间的交互。
	receipts := make([]*types.Receipt, len(w.current.receipts))
	for i, l := range w.current.receipts {
		receipts[i] = new(types.Receipt)
		*receipts[i] = *l
	}
	s := w.current.state.Copy()
	block, err := w.engine.Finalize(w.chain, w.current.header, s, w.current.txs, uncles, w.current.receipts)
	if err != nil {
		return err
	}
	if w.isRunning() {
		if interval != nil {
			interval()
		}
		select {
		case w.taskCh <- &task{receipts: receipts, state: s, block: block, createdAt: time.Now()}:
			w.unconfirmed.Shift(block.NumberU64() - 1)

			feesWei := new(big.Int)
			for i, tx := range block.Transactions() {
				feesWei.Add(feesWei, new(big.Int).Mul(new(big.Int).SetUint64(receipts[i].GasUsed), tx.GasPrice()))
			}
			feesEth := new(big.Float).Quo(new(big.Float).SetInt(feesWei), new(big.Float).SetInt(big.NewInt(params.Ether)))

			log.Info("Commit new mining work", "number", block.Number(), "sealhash", w.engine.SealHash(block.Header()),
				"uncles", len(uncles), "txs", w.current.tcount, "gas", block.GasUsed(), "fees", feesEth, "elapsed", common.PrettyDuration(time.Since(start)))

		case <-w.exitCh:
			log.Info("Worker has exited")
		}
	}
	if update {
		w.updateSnapshot()
	}
	return nil
}
