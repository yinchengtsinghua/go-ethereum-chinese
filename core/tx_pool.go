
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2014 Go Ethereum作者
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

package core

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/prque"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/params"
)

const (
//ChainHeadChansize是侦听ChainHeadEvent的通道的大小。
	chainHeadChanSize = 10
)

var (
//如果事务包含无效签名，则返回errInvalidSender。
	ErrInvalidSender = errors.New("invalid sender")

//如果事务的nonce低于
//一个存在于本地链中。
	ErrNonceTooLow = errors.New("nonce too low")

//如果交易的天然气价格低于最低价格，则返回定价错误。
//为事务池配置。
	ErrUnderpriced = errors.New("transaction underpriced")

//如果试图替换事务，则返回erreplaceCunderpriced
//另一个没有要求的价格上涨。
	ErrReplaceUnderpriced = errors.New("replacement transaction underpriced")

//如果执行事务的总成本为
//高于用户帐户的余额。
	ErrInsufficientFunds = errors.New("insufficient funds for gas * price + value")

//如果交易指定使用更少的气体，则返回errintrinsicgas
//启动调用所需的。
	ErrIntrinsicGas = errors.New("intrinsic gas too low")

//如果交易请求的气体限制超过
//当前块的最大允许量。
	ErrGasLimit = errors.New("exceeds block gas limit")

//errNegativeValue是一个健全的错误，用于确保没有人能够指定
//负值的交易记录。
	ErrNegativeValue = errors.New("negative value")

//如果事务的输入数据大于
//而不是用户可能使用的一些有意义的限制。这不是共识错误
//使事务无效，而不是DoS保护。
	ErrOversizedData = errors.New("oversized data")
)

var (
evictionInterval    = time.Minute     //检查可收回事务的时间间隔
statsReportInterval = 8 * time.Second //报告事务池统计的时间间隔
)

var (
//挂起池的指标
	pendingDiscardCounter   = metrics.NewRegisteredCounter("txpool/pending/discard", nil)
	pendingReplaceCounter   = metrics.NewRegisteredCounter("txpool/pending/replace", nil)
pendingRateLimitCounter = metrics.NewRegisteredCounter("txpool/pending/ratelimit", nil) //由于速率限制而下降
pendingNofundsCounter   = metrics.NewRegisteredCounter("txpool/pending/nofunds", nil)   //因资金不足而放弃

//排队池的指标
	queuedDiscardCounter   = metrics.NewRegisteredCounter("txpool/queued/discard", nil)
	queuedReplaceCounter   = metrics.NewRegisteredCounter("txpool/queued/replace", nil)
queuedRateLimitCounter = metrics.NewRegisteredCounter("txpool/queued/ratelimit", nil) //由于速率限制而下降
queuedNofundsCounter   = metrics.NewRegisteredCounter("txpool/queued/nofunds", nil)   //因资金不足而放弃

//一般Tx指标
	invalidTxCounter     = metrics.NewRegisteredCounter("txpool/invalid", nil)
	underpricedTxCounter = metrics.NewRegisteredCounter("txpool/underpriced", nil)
)

//txstatus是由池看到的事务的当前状态。
type TxStatus uint

const (
	TxStatusUnknown TxStatus = iota
	TxStatusQueued
	TxStatusPending
	TxStatusIncluded
)

//区块链提供区块链的状态和当前的天然气限制。
//Tx池和事件订阅服务器中的一些预检查。
type blockChain interface {
	CurrentBlock() *types.Block
	GetBlock(hash common.Hash, number uint64) *types.Block
	StateAt(root common.Hash) (*state.StateDB, error)

	SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription
}

//TxPoolConfig是事务池的配置参数。
type TxPoolConfig struct {
Locals    []common.Address //默认情况下应视为本地地址的地址
NoLocals  bool             //是否应禁用本地事务处理
Journal   string           //在节点重新启动后幸存的本地事务日志
Rejournal time.Duration    //重新生成本地事务日记帐的时间间隔

PriceLimit uint64 //用于验收的最低天然气价格
PriceBump  uint64 //替换已存在交易的最低价格波动百分比（nonce）

AccountSlots uint64 //每个帐户保证的可执行事务槽数
GlobalSlots  uint64 //所有帐户的最大可执行事务槽数
AccountQueue uint64 //每个帐户允许的最大不可执行事务槽数
GlobalQueue  uint64 //所有帐户的最大不可执行事务槽数

Lifetime time.Duration //非可执行事务排队的最长时间
}

//DefaultTxPoolConfig包含事务的默认配置
//池。
var DefaultTxPoolConfig = TxPoolConfig{
	Journal:   "transactions.rlp",
	Rejournal: time.Hour,

	PriceLimit: 1,
	PriceBump:  10,

	AccountSlots: 16,
	GlobalSlots:  4096,
	AccountQueue: 64,
	GlobalQueue:  1024,

	Lifetime: 3 * time.Hour,
}

//清理检查提供的用户配置并更改
//不合理或不可行。
func (config *TxPoolConfig) sanitize() TxPoolConfig {
	conf := *config
	if conf.Rejournal < time.Second {
		log.Warn("Sanitizing invalid txpool journal time", "provided", conf.Rejournal, "updated", time.Second)
		conf.Rejournal = time.Second
	}
	if conf.PriceLimit < 1 {
		log.Warn("Sanitizing invalid txpool price limit", "provided", conf.PriceLimit, "updated", DefaultTxPoolConfig.PriceLimit)
		conf.PriceLimit = DefaultTxPoolConfig.PriceLimit
	}
	if conf.PriceBump < 1 {
		log.Warn("Sanitizing invalid txpool price bump", "provided", conf.PriceBump, "updated", DefaultTxPoolConfig.PriceBump)
		conf.PriceBump = DefaultTxPoolConfig.PriceBump
	}
	if conf.AccountSlots < 1 {
		log.Warn("Sanitizing invalid txpool account slots", "provided", conf.AccountSlots, "updated", DefaultTxPoolConfig.AccountSlots)
		conf.AccountSlots = DefaultTxPoolConfig.AccountSlots
	}
	if conf.GlobalSlots < 1 {
		log.Warn("Sanitizing invalid txpool global slots", "provided", conf.GlobalSlots, "updated", DefaultTxPoolConfig.GlobalSlots)
		conf.GlobalSlots = DefaultTxPoolConfig.GlobalSlots
	}
	if conf.AccountQueue < 1 {
		log.Warn("Sanitizing invalid txpool account queue", "provided", conf.AccountQueue, "updated", DefaultTxPoolConfig.AccountQueue)
		conf.AccountQueue = DefaultTxPoolConfig.AccountQueue
	}
	if conf.GlobalQueue < 1 {
		log.Warn("Sanitizing invalid txpool global queue", "provided", conf.GlobalQueue, "updated", DefaultTxPoolConfig.GlobalQueue)
		conf.GlobalQueue = DefaultTxPoolConfig.GlobalQueue
	}
	if conf.Lifetime < 1 {
		log.Warn("Sanitizing invalid txpool lifetime", "provided", conf.Lifetime, "updated", DefaultTxPoolConfig.Lifetime)
		conf.Lifetime = DefaultTxPoolConfig.Lifetime
	}
	return conf
}

//TxPool包含所有当前已知的事务。交易
//从网络接收或提交时输入池
//局部地。当它们包含在区块链中时，它们退出池。
//
//池将可处理的事务（可应用于
//当前状态）和未来交易。事务在这些事务之间移动
//两种状态随着时间的推移而被接收和处理。
type TxPool struct {
	config       TxPoolConfig
	chainconfig  *params.ChainConfig
	chain        blockChain
	gasPrice     *big.Int
	txFeed       event.Feed
	scope        event.SubscriptionScope
	chainHeadCh  chan ChainHeadEvent
	chainHeadSub event.Subscription
	signer       types.Signer
	mu           sync.RWMutex

currentState  *state.StateDB      //区块链头中的当前状态
pendingState  *state.ManagedState //挂起状态跟踪虚拟当前
currentMaxGas uint64              //交易上限的当前天然气限额

locals  *accountSet //要免除逐出规则的本地事务集
journal *txJournal  //备份到磁盘的本地事务日志

pending map[common.Address]*txList   //所有当前可处理的事务
queue   map[common.Address]*txList   //排队但不可处理的事务
beats   map[common.Address]time.Time //每个已知帐户的最后一个心跳
all     *txLookup                    //允许查找的所有事务
priced  *txPricedList                //按价格排序的所有交易记录

wg sync.WaitGroup //用于关机同步

	homestead bool
}

//newtxpool创建一个新的事务池来收集、排序和筛选入站事务
//来自网络的事务。
func NewTxPool(config TxPoolConfig, chainconfig *params.ChainConfig, chain blockChain) *TxPool {
//对输入进行消毒，以确保不设定脆弱的天然气价格。
	config = (&config).sanitize()

//使用事务池的初始设置创建事务池
	pool := &TxPool{
		config:      config,
		chainconfig: chainconfig,
		chain:       chain,
		signer:      types.NewEIP155Signer(chainconfig.ChainID),
		pending:     make(map[common.Address]*txList),
		queue:       make(map[common.Address]*txList),
		beats:       make(map[common.Address]time.Time),
		all:         newTxLookup(),
		chainHeadCh: make(chan ChainHeadEvent, chainHeadChanSize),
		gasPrice:    new(big.Int).SetUint64(config.PriceLimit),
	}
	pool.locals = newAccountSet(pool.signer)
	for _, addr := range config.Locals {
		log.Info("Setting new local account", "address", addr)
		pool.locals.add(addr)
	}
	pool.priced = newTxPricedList(pool.all)
	pool.reset(nil, chain.CurrentBlock().Header())

//如果启用了本地事务和日记，则从磁盘加载
	if !config.NoLocals && config.Journal != "" {
		pool.journal = newTxJournal(config.Journal)

		if err := pool.journal.load(pool.AddLocals); err != nil {
			log.Warn("Failed to load transaction journal", "err", err)
		}
		if err := pool.journal.rotate(pool.local()); err != nil {
			log.Warn("Failed to rotate transaction journal", "err", err)
		}
	}
//从区块链订阅事件
	pool.chainHeadSub = pool.chain.SubscribeChainHeadEvent(pool.chainHeadCh)

//启动事件循环并返回
	pool.wg.Add(1)
	go pool.loop()

	return pool
}

//循环是事务池的主事件循环，等待并响应
//外部区块链事件以及各种报告和交易
//驱逐事件。
func (pool *TxPool) loop() {
	defer pool.wg.Done()

//启动统计报告和事务逐出标记
	var prevPending, prevQueued, prevStales int

	report := time.NewTicker(statsReportInterval)
	defer report.Stop()

	evict := time.NewTicker(evictionInterval)
	defer evict.Stop()

	journal := time.NewTicker(pool.config.Rejournal)
	defer journal.Stop()

//跟踪事务重新排序的前一个标题
	head := pool.chain.CurrentBlock()

//持续等待并对各种事件作出反应
	for {
		select {
//处理ChainHeadEvent
		case ev := <-pool.chainHeadCh:
			if ev.Block != nil {
				pool.mu.Lock()
				if pool.chainconfig.IsHomestead(ev.Block.Number()) {
					pool.homestead = true
				}
				pool.reset(head.Header(), ev.Block.Header())
				head = ev.Block

				pool.mu.Unlock()
			}
//由于系统停止而取消订阅
		case <-pool.chainHeadSub.Err():
			return

//处理统计报告标记
		case <-report.C:
			pool.mu.RLock()
			pending, queued := pool.stats()
			stales := pool.priced.stales
			pool.mu.RUnlock()

			if pending != prevPending || queued != prevQueued || stales != prevStales {
				log.Debug("Transaction pool status report", "executable", pending, "queued", queued, "stales", stales)
				prevPending, prevQueued, prevStales = pending, queued, stales
			}

//处理非活动帐户事务收回
		case <-evict.C:
			pool.mu.Lock()
			for addr := range pool.queue {
//从逐出机制跳过本地事务
				if pool.locals.contains(addr) {
					continue
				}
//任何年龄足够大的非本地人都应该被除名。
				if time.Since(pool.beats[addr]) > pool.config.Lifetime {
					for _, tx := range pool.queue[addr].Flatten() {
						pool.removeTx(tx.Hash(), true)
					}
				}
			}
			pool.mu.Unlock()

//处理本地事务日记帐轮换
		case <-journal.C:
			if pool.journal != nil {
				pool.mu.Lock()
				if err := pool.journal.rotate(pool.local()); err != nil {
					log.Warn("Failed to rotate local tx journal", "err", err)
				}
				pool.mu.Unlock()
			}
		}
	}
}

//LockedReset是一个包装重置，允许在线程安全中调用它。
//态度。此方法仅在检测仪中使用！
func (pool *TxPool) lockedReset(oldHead, newHead *types.Header) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.reset(oldHead, newHead)
}

//重置检索区块链的当前状态并确保内容
//的事务池对于链状态有效。
func (pool *TxPool) reset(oldHead, newHead *types.Header) {
//如果要重新定位旧状态，请重新拒绝所有已删除的事务
	var reinject types.Transactions

	if oldHead != nil && oldHead.Hash() != newHead.ParentHash {
//如果REORG太深，请避免这样做（将在快速同步期间发生）
		oldNum := oldHead.Number.Uint64()
		newNum := newHead.Number.Uint64()

		if depth := uint64(math.Abs(float64(oldNum) - float64(newNum))); depth > 64 {
			log.Debug("Skipping deep transaction reorg", "depth", depth)
		} else {
//REORG看起来很浅，足以将所有事务拉入内存
			var discarded, included types.Transactions

			var (
				rem = pool.chain.GetBlock(oldHead.Hash(), oldHead.Number.Uint64())
				add = pool.chain.GetBlock(newHead.Hash(), newHead.Number.Uint64())
			)
			for rem.NumberU64() > add.NumberU64() {
				discarded = append(discarded, rem.Transactions()...)
				if rem = pool.chain.GetBlock(rem.ParentHash(), rem.NumberU64()-1); rem == nil {
					log.Error("Unrooted old chain seen by tx pool", "block", oldHead.Number, "hash", oldHead.Hash())
					return
				}
			}
			for add.NumberU64() > rem.NumberU64() {
				included = append(included, add.Transactions()...)
				if add = pool.chain.GetBlock(add.ParentHash(), add.NumberU64()-1); add == nil {
					log.Error("Unrooted new chain seen by tx pool", "block", newHead.Number, "hash", newHead.Hash())
					return
				}
			}
			for rem.Hash() != add.Hash() {
				discarded = append(discarded, rem.Transactions()...)
				if rem = pool.chain.GetBlock(rem.ParentHash(), rem.NumberU64()-1); rem == nil {
					log.Error("Unrooted old chain seen by tx pool", "block", oldHead.Number, "hash", oldHead.Hash())
					return
				}
				included = append(included, add.Transactions()...)
				if add = pool.chain.GetBlock(add.ParentHash(), add.NumberU64()-1); add == nil {
					log.Error("Unrooted new chain seen by tx pool", "block", newHead.Number, "hash", newHead.Hash())
					return
				}
			}
			reinject = types.TxDifference(discarded, included)
		}
	}
//将内部状态初始化为当前头部
	if newHead == nil {
newHead = pool.chain.CurrentBlock().Header() //测试过程中的特殊情况
	}
	statedb, err := pool.chain.StateAt(newHead.Root)
	if err != nil {
		log.Error("Failed to reset txpool state", "err", err)
		return
	}
	pool.currentState = statedb
	pool.pendingState = state.ManageState(statedb)
	pool.currentMaxGas = newHead.GasLimit

//插入由于重新排序而丢弃的任何事务
	log.Debug("Reinjecting stale transactions", "count", len(reinject))
	senderCacher.recover(pool.signer, reinject)
	pool.addTxsLocked(reinject, false)

//验证挂起事务池，这将删除
//包含在块中的任何交易或
//已因另一个交易（例如
//更高的天然气价格）
	pool.demoteUnexecutables()

//将所有帐户更新为最新的已知挂起的当前帐户
	for addr, list := range pool.pending {
txs := list.Flatten() //很重，但会被缓存，矿工无论如何都需要它。
		pool.pendingState.SetNonce(addr, txs[len(txs)-1].Nonce()+1)
	}
//检查队列并尽可能将事务转移到挂起的
//或者去掉那些已经失效的
	pool.promoteExecutables(nil)
}

//stop终止事务池。
func (pool *TxPool) Stop() {
//取消订阅从txpool注册的所有订阅
	pool.scope.Close()

//取消订阅从区块链注册的订阅
	pool.chainHeadSub.Unsubscribe()
	pool.wg.Wait()

	if pool.journal != nil {
		pool.journal.close()
	}
	log.Info("Transaction pool stopped")
}

//subscripeWtxsEvent注册newtxSevent和的订阅
//开始向给定通道发送事件。
func (pool *TxPool) SubscribeNewTxsEvent(ch chan<- NewTxsEvent) event.Subscription {
	return pool.scope.Track(pool.txFeed.Subscribe(ch))
}

//Gasprice返回交易池强制执行的当前天然气价格。
func (pool *TxPool) GasPrice() *big.Int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return new(big.Int).Set(pool.gasPrice)
}

//setgasprice更新交易池要求的
//新事务，并将所有事务降低到此阈值以下。
func (pool *TxPool) SetGasPrice(price *big.Int) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.gasPrice = price
	for _, tx := range pool.priced.Cap(price, pool.locals) {
		pool.removeTx(tx.Hash(), false)
	}
	log.Info("Transaction pool price threshold updated", "price", price)
}

//状态返回事务池的虚拟托管状态。
func (pool *TxPool) State() *state.ManagedState {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.pendingState
}

//stats检索当前池的统计信息，即挂起的数目和
//排队（不可执行）的事务数。
func (pool *TxPool) Stats() (int, int) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.stats()
}

//stats检索当前池的统计信息，即挂起的数目和
//排队（不可执行）的事务数。
func (pool *TxPool) stats() (int, int) {
	pending := 0
	for _, list := range pool.pending {
		pending += list.Len()
	}
	queued := 0
	for _, list := range pool.queue {
		queued += list.Len()
	}
	return pending, queued
}

//Content检索事务池的数据内容，并返回
//挂起的和排队的事务，按帐户分组并按nonce排序。
func (pool *TxPool) Content() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address]types.Transactions)
	for addr, list := range pool.pending {
		pending[addr] = list.Flatten()
	}
	queued := make(map[common.Address]types.Transactions)
	for addr, list := range pool.queue {
		queued[addr] = list.Flatten()
	}
	return pending, queued
}

//挂起检索所有当前可处理的事务，按来源分组
//帐户并按nonce排序。返回的事务集是一个副本，可以
//通过调用代码自由修改。
func (pool *TxPool) Pending() (map[common.Address]types.Transactions, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address]types.Transactions)
	for addr, list := range pool.pending {
		pending[addr] = list.Flatten()
	}
	return pending, nil
}

//局部变量检索池当前认为是本地的帐户。
func (pool *TxPool) Locals() []common.Address {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.locals.flatten()
}

//本地检索所有当前已知的本地事务，按来源分组
//帐户并按nonce排序。返回的事务集是一个副本，可以
//通过调用代码自由修改。
func (pool *TxPool) local() map[common.Address]types.Transactions {
	txs := make(map[common.Address]types.Transactions)
	for addr := range pool.locals.accounts {
		if pending := pool.pending[addr]; pending != nil {
			txs[addr] = append(txs[addr], pending.Flatten()...)
		}
		if queued := pool.queue[addr]; queued != nil {
			txs[addr] = append(txs[addr], queued.Flatten()...)
		}
	}
	return txs
}

//validatetx根据共识检查交易是否有效
//规则并遵守本地节点的一些启发式限制（价格和大小）。
func (pool *TxPool) validateTx(tx *types.Transaction, local bool) error {
//启发式限制，拒绝超过32KB的事务以防止DoS攻击
	if tx.Size() > 32*1024 {
		return ErrOversizedData
	}
//交易记录不能为负数。使用RLP解码可能永远不会发生这种情况。
//但如果使用RPC创建事务，则可能发生事务。
	if tx.Value().Sign() < 0 {
		return ErrNegativeValue
	}
//确保交易不超过当前区块限制天然气。
	if pool.currentMaxGas < tx.Gas() {
		return ErrGasLimit
	}
//确保事务签名正确
	from, err := types.Sender(pool.signer, tx)
	if err != nil {
		return ErrInvalidSender
	}
//以我们自己接受的最低天然气价格取消非本地交易
local = local || pool.locals.contains(from) //即使事务从网络到达，帐户也可以是本地的
	if !local && pool.gasPrice.Cmp(tx.GasPrice()) > 0 {
		return ErrUnderpriced
	}
//确保事务遵循非紧急排序
	if pool.currentState.GetNonce(from) > tx.Nonce() {
		return ErrNonceTooLow
	}
//交易人应该有足够的资金来支付费用。
//成本==V+gp*gl
	if pool.currentState.GetBalance(from).Cmp(tx.Cost()) < 0 {
		return ErrInsufficientFunds
	}
	intrGas, err := IntrinsicGas(tx.Data(), tx.To() == nil, pool.homestead)
	if err != nil {
		return err
	}
	if tx.Gas() < intrGas {
		return ErrIntrinsicGas
	}
	return nil
}

//添加验证事务并将其插入到的不可执行队列中
//稍后等待提升和执行。如果交易是
//一个已挂起或排队的，它将覆盖上一个并返回此
//所以外部代码不会毫无用处地调用promote。
//
//如果新添加的事务标记为本地，则其发送帐户将
//白名单，防止任何关联交易退出
//由于定价限制而形成的池。
func (pool *TxPool) add(tx *types.Transaction, local bool) (bool, error) {
//如果事务已经知道，则丢弃它
	hash := tx.Hash()
	if pool.all.Get(hash) != nil {
		log.Trace("Discarding already known transaction", "hash", hash)
		return false, fmt.Errorf("known transaction: %x", hash)
	}
//如果事务未能通过基本验证，则放弃它
	if err := pool.validateTx(tx, local); err != nil {
		log.Trace("Discarding invalid transaction", "hash", hash, "err", err)
		invalidTxCounter.Inc(1)
		return false, err
	}
//如果事务池已满，则放弃定价过低的事务
	if uint64(pool.all.Count()) >= pool.config.GlobalSlots+pool.config.GlobalQueue {
//如果新交易定价过低，不要接受
		if !local && pool.priced.Underpriced(tx, pool.locals) {
			log.Trace("Discarding underpriced transaction", "hash", hash, "price", tx.GasPrice())
			underpricedTxCounter.Inc(1)
			return false, ErrUnderpriced
		}
//新的交易比我们糟糕的交易好，给它腾出空间。
		drop := pool.priced.Discard(pool.all.Count()-int(pool.config.GlobalSlots+pool.config.GlobalQueue-1), pool.locals)
		for _, tx := range drop {
			log.Trace("Discarding freshly underpriced transaction", "hash", tx.Hash(), "price", tx.GasPrice())
			underpricedTxCounter.Inc(1)
			pool.removeTx(tx.Hash(), false)
		}
	}
//如果事务正在替换已挂起的事务，请直接执行
from, _ := types.Sender(pool.signer, tx) //已验证
	if list := pool.pending[from]; list != nil && list.Overlaps(tx) {
//一旦已经挂起，检查是否满足所需的价格上涨
		inserted, old := list.Add(tx, pool.config.PriceBump)
		if !inserted {
			pendingDiscardCounter.Inc(1)
			return false, ErrReplaceUnderpriced
		}
//新交易更好，替换旧交易
		if old != nil {
			pool.all.Remove(old.Hash())
			pool.priced.Removed()
			pendingReplaceCounter.Inc(1)
		}
		pool.all.Add(tx)
		pool.priced.Put(tx)
		pool.journalTx(from, tx)

		log.Trace("Pooled new executable transaction", "hash", hash, "from", from, "to", tx.To())

//我们直接注入了一个替换事务，通知子系统
		go pool.txFeed.Send(NewTxsEvent{types.Transactions{tx}})

		return old != nil, nil
	}
//新事务没有替换挂起的事务，请推入队列
	replace, err := pool.enqueueTx(hash, tx)
	if err != nil {
		return false, err
	}
//标记本地地址和日记帐本地交易记录
	if local {
		if !pool.locals.contains(from) {
			log.Info("Setting new local account", "address", from)
			pool.locals.add(from)
		}
	}
	pool.journalTx(from, tx)

	log.Trace("Pooled new future transaction", "hash", hash, "from", from, "to", tx.To())
	return replace, nil
}

//enqueuetx将新事务插入到不可执行的事务队列中。
//
//注意，此方法假定池锁被保持！
func (pool *TxPool) enqueueTx(hash common.Hash, tx *types.Transaction) (bool, error) {
//尝试将事务插入将来的队列
from, _ := types.Sender(pool.signer, tx) //已验证
	if pool.queue[from] == nil {
		pool.queue[from] = newTxList(false)
	}
	inserted, old := pool.queue[from].Add(tx, pool.config.PriceBump)
	if !inserted {
//旧的交易更好，放弃这个
		queuedDiscardCounter.Inc(1)
		return false, ErrReplaceUnderpriced
	}
//放弃任何以前的交易并标记此交易
	if old != nil {
		pool.all.Remove(old.Hash())
		pool.priced.Removed()
		queuedReplaceCounter.Inc(1)
	}
	if pool.all.Get(hash) == nil {
		pool.all.Add(tx)
		pool.priced.Put(tx)
	}
	return old != nil, nil
}

//JournalTx将指定的事务添加到本地磁盘日志（如果是）
//视为从本地帐户发送。
func (pool *TxPool) journalTx(from common.Address, tx *types.Transaction) {
//只有启用了日记帐且事务是本地的
	if pool.journal == nil || !pool.locals.contains(from) {
		return
	}
	if err := pool.journal.insert(tx); err != nil {
		log.Warn("Failed to journal local transaction", "err", err)
	}
}

//promotetx将事务添加到挂起（可处理）的事务列表中
//并返回它是插入的还是旧的更好。
//
//注意，此方法假定池锁被保持！
func (pool *TxPool) promoteTx(addr common.Address, hash common.Hash, tx *types.Transaction) bool {
//尝试将事务插入挂起队列
	if pool.pending[addr] == nil {
		pool.pending[addr] = newTxList(true)
	}
	list := pool.pending[addr]

	inserted, old := list.Add(tx, pool.config.PriceBump)
	if !inserted {
//旧的交易更好，放弃这个
		pool.all.Remove(hash)
		pool.priced.Removed()

		pendingDiscardCounter.Inc(1)
		return false
	}
//否则放弃任何以前的交易并标记此
	if old != nil {
		pool.all.Remove(old.Hash())
		pool.priced.Removed()

		pendingReplaceCounter.Inc(1)
	}
//故障保护以绕过直接挂起的插入（测试）
	if pool.all.Get(hash) == nil {
		pool.all.Add(tx)
		pool.priced.Put(tx)
	}
//设置潜在的新挂起nonce并通知新tx的任何子系统
	pool.beats[addr] = time.Now()
	pool.pendingState.SetNonce(addr, tx.Nonce()+1)

	return true
}

//addlocal将单个事务排入池中（如果该事务有效），标记
//同时将发送方作为本地发送方，确保它绕过本地发送方
//定价限制。
func (pool *TxPool) AddLocal(tx *types.Transaction) error {
	return pool.addTx(tx, !pool.config.NoLocals)
}

//如果单个事务有效，则addremote将其排入池中。如果
//发送方不属于本地跟踪的发送方，完全定价约束将
//申请。
func (pool *TxPool) AddRemote(tx *types.Transaction) error {
	return pool.addTx(tx, false)
}

//addlocals将一批事务排队放入池中，如果它们有效，
//同时将发送者标记为本地发送者，确保他们四处走动
//本地定价限制。
func (pool *TxPool) AddLocals(txs []*types.Transaction) []error {
	return pool.addTxs(txs, !pool.config.NoLocals)
}

//如果一批事务有效，addremotes会将其排队放入池中。
//如果发送方不在本地跟踪的发送方中，则完全定价约束
//将适用。
func (pool *TxPool) AddRemotes(txs []*types.Transaction) []error {
	return pool.addTxs(txs, false)
}

//addtx将单个事务排队放入池中（如果该事务有效）。
func (pool *TxPool) addTx(tx *types.Transaction, local bool) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

//尝试插入事务并更新任何状态
	replace, err := pool.add(tx, local)
	if err != nil {
		return err
	}
//如果我们添加了一个新事务，运行提升检查并返回
	if !replace {
from, _ := types.Sender(pool.signer, tx) //已验证
		pool.promoteExecutables([]common.Address{from})
	}
	return nil
}

//如果一批事务有效，addtx将尝试对其进行排队。
func (pool *TxPool) addTxs(txs []*types.Transaction, local bool) []error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.addTxsLocked(txs, local)
}

//addtxtslocked尝试对一批事务进行排队，如果它们有效，
//同时假定事务池锁已被持有。
func (pool *TxPool) addTxsLocked(txs []*types.Transaction, local bool) []error {
//添加交易批次，跟踪接受的交易
	dirty := make(map[common.Address]struct{})
	errs := make([]error, len(txs))

	for i, tx := range txs {
		var replace bool
		if replace, errs[i] = pool.add(tx, local); errs[i] == nil && !replace {
from, _ := types.Sender(pool.signer, tx) //已验证
			dirty[from] = struct{}{}
		}
	}
//仅当实际添加了某些内容时才重新处理内部状态
	if len(dirty) > 0 {
		addrs := make([]common.Address, 0, len(dirty))
		for addr := range dirty {
			addrs = append(addrs, addr)
		}
		pool.promoteExecutables(addrs)
	}
	return errs
}

//status返回一批事务的状态（未知/挂起/排队）
//通过散列标识。
func (pool *TxPool) Status(hashes []common.Hash) []TxStatus {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	status := make([]TxStatus, len(hashes))
	for i, hash := range hashes {
		if tx := pool.all.Get(hash); tx != nil {
from, _ := types.Sender(pool.signer, tx) //已验证
			if pool.pending[from] != nil && pool.pending[from].txs.items[tx.Nonce()] != nil {
				status[i] = TxStatusPending
			} else {
				status[i] = TxStatusQueued
			}
		}
	}
	return status
}

//get返回包含在池中的事务
//否则为零。
func (pool *TxPool) Get(hash common.Hash) *types.Transaction {
	return pool.all.Get(hash)
}

//removetx从队列中删除单个事务，移动所有后续事务
//事务返回到未来队列。
func (pool *TxPool) removeTx(hash common.Hash, outofbound bool) {
//获取我们要删除的事务
	tx := pool.all.Get(hash)
	if tx == nil {
		return
	}
addr, _ := types.Sender(pool.signer, tx) //已在插入过程中验证

//将其从已知事务列表中删除
	pool.all.Remove(hash)
	if outofbound {
		pool.priced.Removed()
	}
//从挂起列表中删除该事务并立即重置帐户
	if pending := pool.pending[addr]; pending != nil {
		if removed, invalids := pending.Remove(tx); removed {
//如果没有剩余的挂起事务，请删除该列表
			if pending.Empty() {
				delete(pool.pending, addr)
				delete(pool.beats, addr)
			}
//推迟任何失效的交易
			for _, tx := range invalids {
				pool.enqueueTx(tx.Hash(), tx)
			}
//如果需要，立即更新帐户
			if nonce := tx.Nonce(); pool.pendingState.GetNonce(addr) > nonce {
				pool.pendingState.SetNonce(addr, nonce)
			}
			return
		}
	}
//事务在将来的队列中
	if future := pool.queue[addr]; future != nil {
		future.Remove(tx)
		if future.Empty() {
			delete(pool.queue, addr)
		}
	}
}

//PromoteeExecutables将可从
//对一组挂起事务的未来队列。在此过程中，所有
//已删除失效的事务（低nonce、低余额）。
func (pool *TxPool) promoteExecutables(accounts []common.Address) {
//跟踪已提升的事务以立即广播它们
	var promoted []*types.Transaction

//收集所有可能需要更新的帐户
	if accounts == nil {
		accounts = make([]common.Address, 0, len(pool.queue))
		for addr := range pool.queue {
			accounts = append(accounts, addr)
		}
	}
//遍历所有帐户并升级任何可执行事务
	for _, addr := range accounts {
		list := pool.queue[addr]
		if list == nil {
continue //以防有人用不存在的帐户打电话
		}
//删除所有被认为太旧的事务（低nonce）
		for _, tx := range list.Forward(pool.currentState.GetNonce(addr)) {
			hash := tx.Hash()
			log.Trace("Removed old queued transaction", "hash", hash)
			pool.all.Remove(hash)
			pool.priced.Removed()
		}
//放弃所有成本过高的交易（低余额或无天然气）
		drops, _ := list.Filter(pool.currentState.GetBalance(addr), pool.currentMaxGas)
		for _, tx := range drops {
			hash := tx.Hash()
			log.Trace("Removed unpayable queued transaction", "hash", hash)
			pool.all.Remove(hash)
			pool.priced.Removed()
			queuedNofundsCounter.Inc(1)
		}
//收集所有可执行事务并升级它们
		for _, tx := range list.Ready(pool.pendingState.GetNonce(addr)) {
			hash := tx.Hash()
			if pool.promoteTx(addr, hash, tx) {
				log.Trace("Promoting queued transaction", "hash", hash)
				promoted = append(promoted, tx)
			}
		}
//删除超过允许限制的所有交易记录
		if !pool.locals.contains(addr) {
			for _, tx := range list.Cap(int(pool.config.AccountQueue)) {
				hash := tx.Hash()
				pool.all.Remove(hash)
				pool.priced.Removed()
				queuedRateLimitCounter.Inc(1)
				log.Trace("Removed cap-exceeding queued transaction", "hash", hash)
			}
		}
//如果整个队列条目变为空，则将其删除。
		if list.Empty() {
			delete(pool.queue, addr)
		}
	}
//为新升级的事务通知子系统。
	if len(promoted) > 0 {
		go pool.txFeed.Send(NewTxsEvent{promoted})
	}
//如果待定限额溢出，开始均衡限额
	pending := uint64(0)
	for _, list := range pool.pending {
		pending += uint64(list.Len())
	}
	if pending > pool.config.GlobalSlots {
		pendingBeforeCap := pending
//首先收集一个垃圾邮件命令来惩罚大型交易对手
		spammers := prque.New(nil)
		for addr, list := range pool.pending {
//仅从高收入者逐出交易
			if !pool.locals.contains(addr) && uint64(list.Len()) > pool.config.AccountSlots {
				spammers.Push(addr, int64(list.Len()))
			}
		}
//逐步取消罪犯的交易
		offenders := []common.Address{}
		for pending > pool.config.GlobalSlots && !spammers.Empty() {
//如果不是本地地址，则检索下一个罪犯
			offender, _ := spammers.Pop()
			offenders = append(offenders, offender.(common.Address))

//平衡平衡直到所有相同或低于阈值
			if len(offenders) > 1 {
//计算当前所有罪犯的均衡阈值
				threshold := pool.pending[offender.(common.Address)].Len()

//反复减少所有违规者，直至达到限额或阈值以下。
				for pending > pool.config.GlobalSlots && pool.pending[offenders[len(offenders)-2]].Len() > threshold {
					for i := 0; i < len(offenders)-1; i++ {
						list := pool.pending[offenders[i]]
						for _, tx := range list.Cap(list.Len() - 1) {
//也从全局池中删除事务
							hash := tx.Hash()
							pool.all.Remove(hash)
							pool.priced.Removed()

//将当前帐户更新为删除的交易记录
							if nonce := tx.Nonce(); pool.pendingState.GetNonce(offenders[i]) > nonce {
								pool.pendingState.SetNonce(offenders[i], nonce)
							}
							log.Trace("Removed fairness-exceeding pending transaction", "hash", hash)
						}
						pending--
					}
				}
			}
		}
//如果仍高于临界值，则降低至极限或最小允许值
		if pending > pool.config.GlobalSlots && len(offenders) > 0 {
			for pending > pool.config.GlobalSlots && uint64(pool.pending[offenders[len(offenders)-1]].Len()) > pool.config.AccountSlots {
				for _, addr := range offenders {
					list := pool.pending[addr]
					for _, tx := range list.Cap(list.Len() - 1) {
//也从全局池中删除事务
						hash := tx.Hash()
						pool.all.Remove(hash)
						pool.priced.Removed()

//将当前帐户更新为删除的交易记录
						if nonce := tx.Nonce(); pool.pendingState.GetNonce(addr) > nonce {
							pool.pendingState.SetNonce(addr, nonce)
						}
						log.Trace("Removed fairness-exceeding pending transaction", "hash", hash)
					}
					pending--
				}
			}
		}
		pendingRateLimitCounter.Inc(int64(pendingBeforeCap - pending))
	}
//如果排队的事务超过了硬限制，请删除最旧的事务。
	queued := uint64(0)
	for _, list := range pool.queue {
		queued += uint64(list.Len())
	}
	if queued > pool.config.GlobalQueue {
//按心跳对所有具有排队事务的帐户排序
		addresses := make(addressesByHeartbeat, 0, len(pool.queue))
		for addr := range pool.queue {
if !pool.locals.contains(addr) { //不要删除本地变量
				addresses = append(addresses, addressByHeartbeat{addr, pool.beats[addr]})
			}
		}
		sort.Sort(addresses)

//删除事务，直到总数低于限制或只保留局部变量
		for drop := queued - pool.config.GlobalQueue; drop > 0 && len(addresses) > 0; {
			addr := addresses[len(addresses)-1]
			list := pool.queue[addr.address]

			addresses = addresses[:len(addresses)-1]

//如果小于溢出，则删除所有事务
			if size := uint64(list.Len()); size <= drop {
				for _, tx := range list.Flatten() {
					pool.removeTx(tx.Hash(), true)
				}
				drop -= size
				queuedRateLimitCounter.Inc(int64(size))
				continue
			}
//否则只删除最后几个事务
			txs := list.Flatten()
			for i := len(txs) - 1; i >= 0 && drop > 0; i-- {
				pool.removeTx(txs[i].Hash(), true)
				drop--
				queuedRateLimitCounter.Inc(1)
			}
		}
	}
}

//DemoteNextExecutables从池中删除无效和已处理的事务
//可执行/挂起队列以及任何无法执行的后续事务
//将移回将来的队列。
func (pool *TxPool) demoteUnexecutables() {
//迭代所有帐户并降级任何不可执行的事务
	for addr, list := range pool.pending {
		nonce := pool.currentState.GetNonce(addr)

//删除所有被认为太旧的事务（低nonce）
		for _, tx := range list.Forward(nonce) {
			hash := tx.Hash()
			log.Trace("Removed old pending transaction", "hash", hash)
			pool.all.Remove(hash)
			pool.priced.Removed()
		}
//删除所有成本过高的事务（余额不足或没有汽油），并将任何无效的事务排队等待稍后处理。
		drops, invalids := list.Filter(pool.currentState.GetBalance(addr), pool.currentMaxGas)
		for _, tx := range drops {
			hash := tx.Hash()
			log.Trace("Removed unpayable pending transaction", "hash", hash)
			pool.all.Remove(hash)
			pool.priced.Removed()
			pendingNofundsCounter.Inc(1)
		}
		for _, tx := range invalids {
			hash := tx.Hash()
			log.Trace("Demoting pending transaction", "hash", hash)
			pool.enqueueTx(hash, tx)
		}
//如果前面有空白，警告（不应该发生）并推迟所有交易
		if list.Len() > 0 && list.txs.Get(nonce) == nil {
			for _, tx := range list.Cap(0) {
				hash := tx.Hash()
				log.Error("Demoting invalidated transaction", "hash", hash)
				pool.enqueueTx(hash, tx)
			}
		}
//如果整个队列条目变为空，则将其删除。
		if list.Empty() {
			delete(pool.pending, addr)
			delete(pool.beats, addr)
		}
	}
}

//AddressByHeartbeat是用其最后一个活动时间戳标记的帐户地址。
type addressByHeartbeat struct {
	address   common.Address
	heartbeat time.Time
}

type addressesByHeartbeat []addressByHeartbeat

func (a addressesByHeartbeat) Len() int           { return len(a) }
func (a addressesByHeartbeat) Less(i, j int) bool { return a[i].heartbeat.Before(a[j].heartbeat) }
func (a addressesByHeartbeat) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

//accountset只是检查是否存在的一组地址，以及一个签名者
//能够从交易中获得地址。
type accountSet struct {
	accounts map[common.Address]struct{}
	signer   types.Signer
	cache    *[]common.Address
}

//newaccountset创建一个新地址集，其中包含发送者的关联签名者
//导子。
func newAccountSet(signer types.Signer) *accountSet {
	return &accountSet{
		accounts: make(map[common.Address]struct{}),
		signer:   signer,
	}
}

//包含检查给定地址是否包含在集合中。
func (as *accountSet) contains(addr common.Address) bool {
	_, exist := as.accounts[addr]
	return exist
}

//containstx检查给定Tx的发送方是否在集合内。如果发送者
//无法派生，此方法返回false。
func (as *accountSet) containsTx(tx *types.Transaction) bool {
	if addr, err := types.Sender(as.signer, tx); err == nil {
		return as.contains(addr)
	}
	return false
}

//添加在要跟踪的集合中插入新地址。
func (as *accountSet) add(addr common.Address) {
	as.accounts[addr] = struct{}{}
	as.cache = nil
}

//flatten返回此集合中的地址列表，并将其缓存以备以后使用
//重新使用。The returned slice should not be changed!
func (as *accountSet) flatten() []common.Address {
	if as.cache == nil {
		accounts := make([]common.Address, 0, len(as.accounts))
		for account := range as.accounts {
			accounts = append(accounts, account)
		}
		as.cache = &accounts
	}
	return *as.cache
}

//TXLoopUp在TXPLE内部用于跟踪事务，同时允许查找
//互斥争用。
//
//注意，尽管此类型受到适当的保护，以防并发访问，但它
//是**不是**类型，应该在
//事务池，因为它的内部状态与池紧密耦合
//内部机制。该类型的唯一目的是允许出界
//偷看txpool中的池。无需获取范围广泛的
//txpool.mutex。
type txLookup struct {
	all  map[common.Hash]*types.Transaction
	lock sync.RWMutex
}

//new txlookup返回新的txlookup结构。
func newTxLookup() *txLookup {
	return &txLookup{
		all: make(map[common.Hash]*types.Transaction),
	}
}

//范围对地图中的每个键和值调用f。
func (t *txLookup) Range(f func(hash common.Hash, tx *types.Transaction) bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	for key, value := range t.all {
		if !f(key, value) {
			break
		}
	}
}

//get返回查找中存在的事务，如果未找到则返回nil。
func (t *txLookup) Get(hash common.Hash) *types.Transaction {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.all[hash]
}

//count返回查找中的当前项目数。
func (t *txLookup) Count() int {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return len(t.all)
}

//添加将事务添加到查找中。
func (t *txLookup) Add(tx *types.Transaction) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.all[tx.Hash()] = tx
}

//删除从查找中删除事务。
func (t *txLookup) Remove(hash common.Hash) {
	t.lock.Lock()
	defer t.lock.Unlock()

	delete(t.all, hash)
}
