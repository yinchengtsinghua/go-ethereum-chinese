
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

package light

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
//ChainHeadChansize是侦听ChainHeadEvent的通道的大小。
	chainHeadChanSize = 10
)

//TxPermanent是挖掘事务之后挖掘的块数
//被认为是永久的，不需要回滚
var txPermanent = uint64(500)

//TxPool为轻型客户机实现事务池，从而跟踪
//本地创建的事务的状态，检测是否包含这些事务
//在一个块（矿）或回卷。自从我们
//始终以相同的顺序接收所有本地签名的事务
//创建。
type TxPool struct {
	config       *params.ChainConfig
	signer       types.Signer
	quit         chan bool
	txFeed       event.Feed
	scope        event.SubscriptionScope
	chainHeadCh  chan core.ChainHeadEvent
	chainHeadSub event.Subscription
	mu           sync.RWMutex
	chain        *LightChain
	odr          OdrBackend
	chainDb      ethdb.Database
	relay        TxRelayBackend
	head         common.Hash
nonce        map[common.Address]uint64            //“待定”
pending      map[common.Hash]*types.Transaction   //按Tx哈希排序的挂起事务
mined        map[common.Hash][]*types.Transaction //按块哈希挖掘的事务
clearIdx     uint64                               //可包含已开采Tx信息的最早区块编号

	homestead bool
}

//txtrelaybackend提供了一个接口，用于转发Transacion的机制
//到ETH网络。函数的实现应该是非阻塞的。
//
//发送指示后端转发新事务
//new head在tx池处理后通知后端有关新头的信息，
//包括自上次事件以来的已挖掘和回滚事务
//Discard通知后端应该丢弃的事务
//因为它们被重新发送或被挖掘所取代
//很久以前，不需要回滚
type TxRelayBackend interface {
	Send(txs types.Transactions)
	NewHead(head common.Hash, mined []common.Hash, rollback []common.Hash)
	Discard(hashes []common.Hash)
}

//newtxpool创建新的轻型事务池
func NewTxPool(config *params.ChainConfig, chain *LightChain, relay TxRelayBackend) *TxPool {
	pool := &TxPool{
		config:      config,
		signer:      types.NewEIP155Signer(config.ChainID),
		nonce:       make(map[common.Address]uint64),
		pending:     make(map[common.Hash]*types.Transaction),
		mined:       make(map[common.Hash][]*types.Transaction),
		quit:        make(chan bool),
		chainHeadCh: make(chan core.ChainHeadEvent, chainHeadChanSize),
		chain:       chain,
		relay:       relay,
		odr:         chain.Odr(),
		chainDb:     chain.Odr().Database(),
		head:        chain.CurrentHeader().Hash(),
		clearIdx:    chain.CurrentHeader().Number.Uint64(),
	}
//从区块链订阅事件
	pool.chainHeadSub = pool.chain.SubscribeChainHeadEvent(pool.chainHeadCh)
	go pool.eventLoop()

	return pool
}

//currentState返回当前头段的灯状态
func (pool *TxPool) currentState(ctx context.Context) *state.StateDB {
	return NewState(ctx, pool.chain.CurrentHeader(), pool.odr)
}

//getnonce返回给定地址的“挂起”nonce。它总是询问
//也属于最新标题的nonce，以便检测是否有其他标题
//使用相同密钥的客户端发送了一个事务。
func (pool *TxPool) GetNonce(ctx context.Context, addr common.Address) (uint64, error) {
	state := pool.currentState(ctx)
	nonce := state.GetNonce(addr)
	if state.Error() != nil {
		return 0, state.Error()
	}
	sn, ok := pool.nonce[addr]
	if ok && sn > nonce {
		nonce = sn
	}
	if !ok || sn < nonce {
		pool.nonce[addr] = nonce
	}
	return nonce, nil
}

//TxStateChanges存储的挂起/挖掘状态之间的最近更改
//交易。“真”表示已开采，“假”表示已回退，“不进入”表示无变化。
type txStateChanges map[common.Hash]bool

//setState将Tx的状态设置为“最近开采”或“最近回滚”
func (txc txStateChanges) setState(txHash common.Hash, mined bool) {
	val, ent := txc[txHash]
	if ent && (val != mined) {
		delete(txc, txHash)
	} else {
		txc[txHash] = mined
	}
}

//GetLists创建挖掘和回滚的Tx哈希列表
func (txc txStateChanges) getLists() (mined []common.Hash, rollback []common.Hash) {
	for hash, val := range txc {
		if val {
			mined = append(mined, hash)
		} else {
			rollback = append(rollback, hash)
		}
	}
	return
}

//checkminedTxs为当前挂起的事务检查新添加的块
//必要时标记为已开采。它还将块位置存储在数据库中
//并将它们添加到接收到的txtStateChanges映射中。
func (pool *TxPool) checkMinedTxs(ctx context.Context, hash common.Hash, number uint64, txc txStateChanges) error {
//如果没有交易悬而未决，我们什么都不在乎
	if len(pool.pending) == 0 {
		return nil
	}
	block, err := GetBlock(ctx, pool.odr, hash, number)
	if err != nil {
		return err
	}
//收集在此块中挖掘的所有本地事务
	list := pool.mined[hash]
	for _, tx := range block.Transactions() {
		if _, ok := pool.pending[tx.Hash()]; ok {
			list = append(list, tx)
		}
	}
//如果挖掘了某些事务，请将所需数据写入磁盘并更新
	if list != nil {
//检索属于此块的所有收据并写入循环表
if _, err := GetBlockReceipts(ctx, pool.odr, hash, number); err != nil { //ODR缓存，忽略结果
			return err
		}
		rawdb.WriteTxLookupEntries(pool.chainDb, block)

//更新事务池的状态
		for _, tx := range list {
			delete(pool.pending, tx.Hash())
			txc.setState(tx.Hash(), true)
		}
		pool.mined[hash] = list
	}
	return nil
}

//RollbackTXS标记最近回滚块中包含的事务
//像卷回一样。它还删除任何位置查找条目。
func (pool *TxPool) rollbackTxs(hash common.Hash, txc txStateChanges) {
	batch := pool.chainDb.NewBatch()
	if list, ok := pool.mined[hash]; ok {
		for _, tx := range list {
			txHash := tx.Hash()
			rawdb.DeleteTxLookupEntry(batch, txHash)
			pool.pending[txHash] = tx
			txc.setState(txHash, false)
		}
		delete(pool.mined, hash)
	}
	batch.Write()
}

//重新定位new head设置新的head header，处理（必要时回滚）
//从上一个已知头开始的块，并返回包含
//最近挖掘和回滚的事务哈希。如果出现错误（上下文
//超时）在检查新块时发生，它离开本地已知的头
//在最近选中的块上，仍然返回有效的txtStateChanges，使其
//可以在下一个链头事件中继续检查丢失的块
func (pool *TxPool) reorgOnNewHead(ctx context.Context, newHeader *types.Header) (txStateChanges, error) {
	txc := make(txStateChanges)
	oldh := pool.chain.GetHeaderByHash(pool.head)
	newh := newHeader
//查找公共祖先，创建回滚和新块哈希的列表
	var oldHashes, newHashes []common.Hash
	for oldh.Hash() != newh.Hash() {
		if oldh.Number.Uint64() >= newh.Number.Uint64() {
			oldHashes = append(oldHashes, oldh.Hash())
			oldh = pool.chain.GetHeader(oldh.ParentHash, oldh.Number.Uint64()-1)
		}
		if oldh.Number.Uint64() < newh.Number.Uint64() {
			newHashes = append(newHashes, newh.Hash())
			newh = pool.chain.GetHeader(newh.ParentHash, newh.Number.Uint64()-1)
			if newh == nil {
//当CHT同步时发生，无需执行任何操作
				newh = oldh
			}
		}
	}
	if oldh.Number.Uint64() < pool.clearIdx {
		pool.clearIdx = oldh.Number.Uint64()
	}
//回滚旧块
	for _, hash := range oldHashes {
		pool.rollbackTxs(hash, txc)
	}
	pool.head = oldh.Hash()
//检查新块的挖掘Txs（数组顺序相反）
	for i := len(newHashes) - 1; i >= 0; i-- {
		hash := newHashes[i]
		if err := pool.checkMinedTxs(ctx, hash, newHeader.Number.Uint64()-uint64(i), txc); err != nil {
			return txc, err
		}
		pool.head = hash
	}

//清除旧块的旧挖掘Tx条目
	if idx := newHeader.Number.Uint64(); idx > pool.clearIdx+txPermanent {
		idx2 := idx - txPermanent
		if len(pool.mined) > 0 {
			for i := pool.clearIdx; i < idx2; i++ {
				hash := rawdb.ReadCanonicalHash(pool.chainDb, i)
				if list, ok := pool.mined[hash]; ok {
					hashes := make([]common.Hash, len(list))
					for i, tx := range list {
						hashes[i] = tx.Hash()
					}
					pool.relay.Discard(hashes)
					delete(pool.mined, hash)
				}
			}
		}
		pool.clearIdx = idx2
	}

	return txc, nil
}

//BlockCheckTimeout是检查挖掘的新块的时间限制
//交易。如果超时，则在下一个链头事件时检查恢复。
const blockCheckTimeout = time.Second * 3

//EventLoop处理链头事件并通知Tx中继后端
//关于新头哈希和Tx状态更改
func (pool *TxPool) eventLoop() {
	for {
		select {
		case ev := <-pool.chainHeadCh:
			pool.setNewHead(ev.Block.Header())
//为了避免锁被霸占，这部分会
//替换为后续的pr。
			time.Sleep(time.Millisecond)

//系统停止
		case <-pool.chainHeadSub.Err():
			return
		}
	}
}

func (pool *TxPool) setNewHead(head *types.Header) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), blockCheckTimeout)
	defer cancel()

	txc, _ := pool.reorgOnNewHead(ctx, head)
	m, r := txc.getLists()
	pool.relay.NewHead(pool.head, m, r)
	pool.homestead = pool.config.IsHomestead(head.Number)
	pool.signer = types.MakeSigner(pool.config, head.Number)
}

//停止停止轻型事务处理池
func (pool *TxPool) Stop() {
//取消订阅从txpool注册的所有订阅
	pool.scope.Close()
//取消订阅从区块链注册的订阅
	pool.chainHeadSub.Unsubscribe()
	close(pool.quit)
	log.Info("Transaction pool stopped")
}

//subscripeWtxEvent注册core.newtxSevent和的订阅
//开始向给定通道发送事件。
func (pool *TxPool) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return pool.scope.Track(pool.txFeed.Subscribe(ch))
}

//stats返回当前挂起（本地创建）的事务数
func (pool *TxPool) Stats() (pending int) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	pending = len(pool.pending)
	return
}

//validatetx根据共识规则检查交易是否有效。
func (pool *TxPool) validateTx(ctx context.Context, tx *types.Transaction) error {
//验证发送器
	var (
		from common.Address
		err  error
	)

//验证事务发送方及其SIG。投掷
//如果“发件人”字段无效。
	if from, err = types.Sender(pool.signer, tx); err != nil {
		return core.ErrInvalidSender
	}
//最后但不是最不重要的检查非关键错误
	currentState := pool.currentState(ctx)
	if n := currentState.GetNonce(from); n > tx.Nonce() {
		return core.ErrNonceTooLow
	}

//检查交易不超过当前
//阻塞限制气体。
	header := pool.chain.GetHeaderByHash(pool.head)
	if header.GasLimit < tx.Gas() {
		return core.ErrGasLimit
	}

//交易记录不能为负数。这可能永远不会发生
//使用RLP解码的事务，但如果创建
//例如，使用RPC的事务。
	if tx.Value().Sign() < 0 {
		return core.ErrNegativeValue
	}

//交易人应该有足够的资金来支付费用。
//成本==V+gp*gl
	if b := currentState.GetBalance(from); b.Cmp(tx.Cost()) < 0 {
		return core.ErrInsufficientFunds
	}

//应提供足够的固有气体
	gas, err := core.IntrinsicGas(tx.Data(), tx.To() == nil, pool.homestead)
	if err != nil {
		return err
	}
	if tx.Gas() < gas {
		return core.ErrIntrinsicGas
	}
	return currentState.Error()
}

//添加将验证新事务，并将其状态设置为挂起（如果可处理）。
//如果需要，它还会更新本地存储的nonce。
func (self *TxPool) add(ctx context.Context, tx *types.Transaction) error {
	hash := tx.Hash()

	if self.pending[hash] != nil {
		return fmt.Errorf("Known transaction (%x)", hash[:4])
	}
	err := self.validateTx(ctx, tx)
	if err != nil {
		return err
	}

	if _, ok := self.pending[hash]; !ok {
		self.pending[hash] = tx

		nonce := tx.Nonce() + 1

		addr, _ := types.Sender(self.signer, tx)
		if nonce > self.nonce[addr] {
			self.nonce[addr] = nonce
		}

//通知订户。此事件在Goroutine中发布
//因为在“删除事务”后的某个位置
//调用，然后等待全局Tx池锁定和死锁。
		go self.txFeed.Send(core.NewTxsEvent{Txs: types.Transactions{tx}})
	}

//如果设置了足够低的级别，则打印日志消息
	log.Debug("Pooled new transaction", "hash", hash, "from", log.Lazy{Fn: func() common.Address { from, _ := types.Sender(self.signer, tx); return from }}, "to", tx.To())
	return nil
}

//添加将事务添加到池中（如果有效）并将其传递到Tx中继
//后端
func (self *TxPool) Add(ctx context.Context, tx *types.Transaction) error {
	self.mu.Lock()
	defer self.mu.Unlock()

	data, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return err
	}

	if err := self.add(ctx, tx); err != nil {
		return err
	}
//fmt.println（“发送”，tx.hash（））
	self.relay.Send(types.Transactions{tx})

	self.chainDb.Put(tx.Hash().Bytes(), data)
	return nil
}

//addTransactions将所有有效事务添加到池中，并将它们传递给
//Tx中继后端
func (self *TxPool) AddBatch(ctx context.Context, txs []*types.Transaction) {
	self.mu.Lock()
	defer self.mu.Unlock()
	var sendTx types.Transactions

	for _, tx := range txs {
		if err := self.add(ctx, tx); err == nil {
			sendTx = append(sendTx, tx)
		}
	}
	if len(sendTx) > 0 {
		self.relay.Send(sendTx)
	}
}

//GetTransaction返回包含在池中的事务
//否则为零。
func (tp *TxPool) GetTransaction(hash common.Hash) *types.Transaction {
//先检查TXS
	if tx, ok := tp.pending[hash]; ok {
		return tx
	}
	return nil
}

//GetTransactions返回所有当前可处理的事务。
//调用程序可以修改返回的切片。
func (self *TxPool) GetTransactions() (txs types.Transactions, err error) {
	self.mu.RLock()
	defer self.mu.RUnlock()

	txs = make(types.Transactions, len(self.pending))
	i := 0
	for _, tx := range self.pending {
		txs[i] = tx
		i++
	}
	return txs, nil
}

//Content检索事务池的数据内容，并返回
//挂起和排队的事务，按帐户和nonce分组。
func (self *TxPool) Content() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	self.mu.RLock()
	defer self.mu.RUnlock()

//检索所有挂起的事务，并按帐户和当前排序
	pending := make(map[common.Address]types.Transactions)
	for _, tx := range self.pending {
		account, _ := types.Sender(self.signer, tx)
		pending[account] = append(pending[account], tx)
	}
//光池中没有排队的事务，只返回一个空映射
	queued := make(map[common.Address]types.Transactions)
	return pending, queued
}

//removeTransactions从池中删除所有给定的事务。
func (self *TxPool) RemoveTransactions(txs types.Transactions) {
	self.mu.Lock()
	defer self.mu.Unlock()

	var hashes []common.Hash
	batch := self.chainDb.NewBatch()
	for _, tx := range txs {
		hash := tx.Hash()
		delete(self.pending, hash)
		batch.Delete(hash.Bytes())
		hashes = append(hashes, hash)
	}
	batch.Write()
	self.relay.Discard(hashes)
}

//removetx从池中删除具有给定哈希的事务。
func (pool *TxPool) RemoveTx(hash common.Hash) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
//从挂起池中删除
	delete(pool.pending, hash)
	pool.chainDb.Delete(hash[:])
	pool.relay.Discard([]common.Hash{hash})
}
