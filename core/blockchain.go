
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

//软件包核心实现以太坊共识协议。
package core

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	mrand "math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/common/prque"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/hashicorp/golang-lru"
)

var (
	blockInsertTimer     = metrics.NewRegisteredTimer("chain/inserts", nil)
	blockValidationTimer = metrics.NewRegisteredTimer("chain/validation", nil)
	blockExecutionTimer  = metrics.NewRegisteredTimer("chain/execution", nil)
	blockWriteTimer      = metrics.NewRegisteredTimer("chain/write", nil)

	ErrNoGenesis = errors.New("Genesis not found in chain")
)

const (
	bodyCacheLimit      = 256
	blockCacheLimit     = 256
	receiptsCacheLimit  = 32
	maxFutureBlocks     = 256
	maxTimeFutureBlocks = 30
	badBlockLimit       = 10
	triesInMemory       = 128

//blockchainversion确保不兼容的数据库强制从头开始重新同步。
	BlockChainVersion uint64 = 3
)

//cacheconfig包含trie缓存/修剪的配置值
//它位于区块链中。
type CacheConfig struct {
Disabled       bool          //是否禁用trie写缓存（存档节点）
TrieCleanLimit int           //用于在内存中缓存trie节点的内存允许量（MB）
TrieDirtyLimit int           //开始将脏的trie节点刷新到磁盘的内存限制（MB）
TrieTimeLimit  time.Duration //刷新内存中当前磁盘的时间限制
}

//区块链表示给定数据库的标准链，其中包含一个Genesis
//块。区块链管理链导入、恢复、链重组。
//
//将块导入到块链中是根据规则集进行的
//由两阶段验证程序定义。块的处理是使用
//处理所包含事务的处理器。国家的确认
//在验证器的第二部分完成。失败导致中止
//进口。
//
//区块链也有助于从包含的**任何**链返回区块。
//以及表示规范链的块。它是
//需要注意的是，getBlock可以返回任何块，不需要
//包含在规范中，其中as getblockbynumber始终表示
//规范链。
type BlockChain struct {
chainConfig *params.ChainConfig //链和网络配置
cacheConfig *CacheConfig        //用于修剪的高速缓存配置

db     ethdb.Database //用于存储最终内容的低级持久数据库
triegc *prque.Prque   //优先级队列映射块号以尝试GC
gcproc time.Duration  //为trie转储累积规范块处理

	hc            *HeaderChain
	rmLogsFeed    event.Feed
	chainFeed     event.Feed
	chainSideFeed event.Feed
	chainHeadFeed event.Feed
	logsFeed      event.Feed
	scope         event.SubscriptionScope
	genesisBlock  *types.Block

chainmu sync.RWMutex //区块链插入锁
procmu  sync.RWMutex //块处理器锁

checkpoint       int          //检查站向新检查站计数
currentBlock     atomic.Value //当前区块链头
currentFastBlock atomic.Value //快速同步链的当前磁头（可能在区块链上方！）

stateCache    state.Database //要在导入之间重用的状态数据库（包含状态缓存）
bodyCache     *lru.Cache     //缓存最新的块体
bodyRLPCache  *lru.Cache     //以rlp编码格式缓存最新的块体
receiptsCache *lru.Cache     //缓存每个块最近的收据
blockCache    *lru.Cache     //缓存最近的整个块
futureBlocks  *lru.Cache     //未来的块是为以后的处理添加的块

quit    chan struct{} //区块链退出渠道
running int32         //运行必须以原子方式调用
//
procInterrupt int32          //
wg            sync.WaitGroup //

	engine    consensus.Engine
processor Processor //
validator Validator //
	vmConfig  vm.Config

badBlocks      *lru.Cache              //坏块高速缓存
shouldPreserve func(*types.Block) bool //用于确定是否应保留给定块的函数。
}

//newblockchain使用信息返回完全初始化的块链
//在数据库中可用。它初始化默认的以太坊验证器并
//处理器。
func NewBlockChain(db ethdb.Database, cacheConfig *CacheConfig, chainConfig *params.ChainConfig, engine consensus.Engine, vmConfig vm.Config, shouldPreserve func(block *types.Block) bool) (*BlockChain, error) {
	if cacheConfig == nil {
		cacheConfig = &CacheConfig{
			TrieCleanLimit: 256,
			TrieDirtyLimit: 256,
			TrieTimeLimit:  5 * time.Minute,
		}
	}
	bodyCache, _ := lru.New(bodyCacheLimit)
	bodyRLPCache, _ := lru.New(bodyCacheLimit)
	receiptsCache, _ := lru.New(receiptsCacheLimit)
	blockCache, _ := lru.New(blockCacheLimit)
	futureBlocks, _ := lru.New(maxFutureBlocks)
	badBlocks, _ := lru.New(badBlockLimit)

	bc := &BlockChain{
		chainConfig:    chainConfig,
		cacheConfig:    cacheConfig,
		db:             db,
		triegc:         prque.New(nil),
		stateCache:     state.NewDatabaseWithCache(db, cacheConfig.TrieCleanLimit),
		quit:           make(chan struct{}),
		shouldPreserve: shouldPreserve,
		bodyCache:      bodyCache,
		bodyRLPCache:   bodyRLPCache,
		receiptsCache:  receiptsCache,
		blockCache:     blockCache,
		futureBlocks:   futureBlocks,
		engine:         engine,
		vmConfig:       vmConfig,
		badBlocks:      badBlocks,
	}
	bc.SetValidator(NewBlockValidator(chainConfig, bc, engine))
	bc.SetProcessor(NewStateProcessor(chainConfig, bc, engine))

	var err error
	bc.hc, err = NewHeaderChain(db, chainConfig, engine, bc.getProcInterrupt)
	if err != nil {
		return nil, err
	}
	bc.genesisBlock = bc.GetBlockByNumber(0)
	if bc.genesisBlock == nil {
		return nil, ErrNoGenesis
	}
	if err := bc.loadLastState(); err != nil {
		return nil, err
	}
//检查块哈希的当前状态，确保链中没有任何坏块
	for hash := range BadHashes {
		if header := bc.GetHeaderByHash(hash); header != nil {
//获取与有问题的头的编号相对应的规范块
			headerByNumber := bc.GetHeaderByNumber(header.Number.Uint64())
//确保headerByNumber（如果存在）位于当前的规范链中。
			if headerByNumber != nil && headerByNumber.Hash() == header.Hash() {
				log.Error("Found bad hash, rewinding chain", "number", header.Number, "hash", header.ParentHash)
				bc.SetHead(header.Number.Uint64() - 1)
				log.Error("Chain rewind was successful, resuming normal operation")
			}
		}
	}
//取得这个国家的所有权
	go bc.update()
	return bc, nil
}

func (bc *BlockChain) getProcInterrupt() bool {
	return atomic.LoadInt32(&bc.procInterrupt) == 1
}

//getvmconfig返回块链vm config。
func (bc *BlockChain) GetVMConfig() *vm.Config {
	return &bc.vmConfig
}

//loadLastState从数据库加载最后一个已知的链状态。这种方法
//假定保持链管理器互斥锁。
func (bc *BlockChain) loadLastState() error {
//恢复上一个已知的头块
	head := rawdb.ReadHeadBlockHash(bc.db)
	if head == (common.Hash{}) {
//数据库已损坏或为空，从头开始初始化
		log.Warn("Empty database, resetting chain")
		return bc.Reset()
	}
//确保整个头块可用
	currentBlock := bc.GetBlockByHash(head)
	if currentBlock == nil {
//数据库已损坏或为空，从头开始初始化
		log.Warn("Head block missing, resetting chain", "hash", head)
		return bc.Reset()
	}
//确保与块关联的状态可用
	if _, err := state.New(currentBlock.Root(), bc.stateCache); err != nil {
//没有关联状态的挂起块，从头开始初始化
		log.Warn("Head state missing, repairing chain", "number", currentBlock.Number(), "hash", currentBlock.Hash())
		if err := bc.repair(&currentBlock); err != nil {
			return err
		}
	}
//一切似乎都很好，设为头挡
	bc.currentBlock.Store(currentBlock)

//恢复上一个已知的头段
	currentHeader := currentBlock.Header()
	if head := rawdb.ReadHeadHeaderHash(bc.db); head != (common.Hash{}) {
		if header := bc.GetHeaderByHash(head); header != nil {
			currentHeader = header
		}
	}
	bc.hc.SetCurrentHeader(currentHeader)

//恢复上一个已知的头快速块
	bc.currentFastBlock.Store(currentBlock)
	if head := rawdb.ReadHeadFastBlockHash(bc.db); head != (common.Hash{}) {
		if block := bc.GetBlockByHash(head); block != nil {
			bc.currentFastBlock.Store(block)
		}
	}

//为用户发出状态日志
	currentFastBlock := bc.CurrentFastBlock()

	headerTd := bc.GetTd(currentHeader.Hash(), currentHeader.Number.Uint64())
	blockTd := bc.GetTd(currentBlock.Hash(), currentBlock.NumberU64())
	fastTd := bc.GetTd(currentFastBlock.Hash(), currentFastBlock.NumberU64())

	log.Info("Loaded most recent local header", "number", currentHeader.Number, "hash", currentHeader.Hash(), "td", headerTd, "age", common.PrettyAge(time.Unix(currentHeader.Time.Int64(), 0)))
	log.Info("Loaded most recent local full block", "number", currentBlock.Number(), "hash", currentBlock.Hash(), "td", blockTd, "age", common.PrettyAge(time.Unix(currentBlock.Time().Int64(), 0)))
	log.Info("Loaded most recent local fast block", "number", currentFastBlock.Number(), "hash", currentFastBlock.Hash(), "td", fastTd, "age", common.PrettyAge(time.Unix(currentFastBlock.Time().Int64(), 0)))

	return nil
}

//sethead将本地链重绕到新的head。在头的情况下，一切
//上面的新头部将被删除和新的一套。如果是积木
//但是，如果块体丢失（非存档），头部可能会被进一步重绕
//快速同步后的节点）。
func (bc *BlockChain) SetHead(head uint64) error {
	log.Warn("Rewinding blockchain", "target", head)

	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

//倒带标题链，删除所有块体，直到
	delFn := func(db rawdb.DatabaseDeleter, hash common.Hash, num uint64) {
		rawdb.DeleteBody(db, hash, num)
	}
	bc.hc.SetHead(head, delFn)
	currentHeader := bc.hc.CurrentHeader()

//从缓存中清除所有过时的内容
	bc.bodyCache.Purge()
	bc.bodyRLPCache.Purge()
	bc.receiptsCache.Purge()
	bc.blockCache.Purge()
	bc.futureBlocks.Purge()

//倒带区块链，确保不会以无状态头区块结束。
	if currentBlock := bc.CurrentBlock(); currentBlock != nil && currentHeader.Number.Uint64() < currentBlock.NumberU64() {
		bc.currentBlock.Store(bc.GetBlock(currentHeader.Hash(), currentHeader.Number.Uint64()))
	}
	if currentBlock := bc.CurrentBlock(); currentBlock != nil {
		if _, err := state.New(currentBlock.Root(), bc.stateCache); err != nil {
//重绕状态丢失，回滚到轴之前，重置为Genesis
			bc.currentBlock.Store(bc.genesisBlock)
		}
	}
//以简单的方式将快速块倒回目标头
	if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock != nil && currentHeader.Number.Uint64() < currentFastBlock.NumberU64() {
		bc.currentFastBlock.Store(bc.GetBlock(currentHeader.Hash(), currentHeader.Number.Uint64()))
	}
//如果任一块达到零，则重置为“创世”状态。
	if currentBlock := bc.CurrentBlock(); currentBlock == nil {
		bc.currentBlock.Store(bc.genesisBlock)
	}
	if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock == nil {
		bc.currentFastBlock.Store(bc.genesisBlock)
	}
	currentBlock := bc.CurrentBlock()
	currentFastBlock := bc.CurrentFastBlock()

	rawdb.WriteHeadBlockHash(bc.db, currentBlock.Hash())
	rawdb.WriteHeadFastBlockHash(bc.db, currentFastBlock.Hash())

	return bc.loadLastState()
}

//fastsynccommithead将当前头块设置为哈希定义的头块
//与之前的链内容无关。
func (bc *BlockChain) FastSyncCommitHead(hash common.Hash) error {
//确保块及其状态trie都存在
	block := bc.GetBlockByHash(hash)
	if block == nil {
		return fmt.Errorf("non existent block [%x…]", hash[:4])
	}
	if _, err := trie.NewSecure(block.Root(), bc.stateCache.TrieDB(), 0); err != nil {
		return err
	}
//如果全部签出，手动设置头块
	bc.chainmu.Lock()
	bc.currentBlock.Store(block)
	bc.chainmu.Unlock()

	log.Info("Committed new head block", "number", block.Number(), "hash", hash)
	return nil
}

//gas limit返回当前头块的气体限制。
func (bc *BlockChain) GasLimit() uint64 {
	return bc.CurrentBlock().GasLimit()
}

//currentBlock检索规范链的当前头块。这个
//块从区块链的内部缓存中检索。
func (bc *BlockChain) CurrentBlock() *types.Block {
	return bc.currentBlock.Load().(*types.Block)
}

//currentFastBlock检索规范的当前快速同步头块
//链。块从区块链的内部缓存中检索。
func (bc *BlockChain) CurrentFastBlock() *types.Block {
	return bc.currentFastBlock.Load().(*types.Block)
}

//setprocessor设置进行状态修改所需的处理器。
func (bc *BlockChain) SetProcessor(processor Processor) {
	bc.procmu.Lock()
	defer bc.procmu.Unlock()
	bc.processor = processor
}

//setvalidator设置用于验证传入块的验证程序。
func (bc *BlockChain) SetValidator(validator Validator) {
	bc.procmu.Lock()
	defer bc.procmu.Unlock()
	bc.validator = validator
}

//验证器返回当前验证器。
func (bc *BlockChain) Validator() Validator {
	bc.procmu.RLock()
	defer bc.procmu.RUnlock()
	return bc.validator
}

//处理器返回当前处理器。
func (bc *BlockChain) Processor() Processor {
	bc.procmu.RLock()
	defer bc.procmu.RUnlock()
	return bc.processor
}

//State返回基于当前头块的新可变状态。
func (bc *BlockChain) State() (*state.StateDB, error) {
	return bc.StateAt(bc.CurrentBlock().Root())
}

//stateat返回基于特定时间点的新可变状态。
func (bc *BlockChain) StateAt(root common.Hash) (*state.StateDB, error) {
	return state.New(root, bc.stateCache)
}

//StateCache返回支撑区块链实例的缓存数据库。
func (bc *BlockChain) StateCache() state.Database {
	return bc.stateCache
}

//重置清除整个区块链，将其恢复到其创始状态。
func (bc *BlockChain) Reset() error {
	return bc.ResetWithGenesisBlock(bc.genesisBlock)
}

//ResetWithGenerisBlock清除整个区块链，将其恢复到
//指定的创世状态。
func (bc *BlockChain) ResetWithGenesisBlock(genesis *types.Block) error {
//转储整个块链并清除缓存
	if err := bc.SetHead(0); err != nil {
		return err
	}
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

//准备Genesis块并重新初始化链
	if err := bc.hc.WriteTd(genesis.Hash(), genesis.NumberU64(), genesis.Difficulty()); err != nil {
		log.Crit("Failed to write genesis block TD", "err", err)
	}
	rawdb.WriteBlock(bc.db, genesis)

	bc.genesisBlock = genesis
	bc.insert(bc.genesisBlock)
	bc.currentBlock.Store(bc.genesisBlock)
	bc.hc.SetGenesis(bc.genesisBlock.Header())
	bc.hc.SetCurrentHeader(bc.genesisBlock.Header())
	bc.currentFastBlock.Store(bc.genesisBlock)

	return nil
}

//修复试图通过回滚当前块来修复当前区块链
//直到找到一个具有关联状态的。这需要修复不完整的数据库
//由崩溃/断电或简单的未提交尝试引起的写入。
//
//此方法只回滚当前块。当前标题和当前
//快速挡块完好无损。
func (bc *BlockChain) repair(head **types.Block) error {
	for {
//如果我们重绕到一个有关联状态的头块，则中止
		if _, err := state.New((*head).Root(), bc.stateCache); err == nil {
			log.Info("Rewound blockchain to past state", "number", (*head).Number(), "hash", (*head).Hash())
			return nil
		}
//否则，倒带一个块并在那里重新检查状态可用性
		block := bc.GetBlock((*head).ParentHash(), (*head).NumberU64()-1)
		if block == nil {
			return fmt.Errorf("missing block %d [%x]", (*head).NumberU64()-1, (*head).ParentHash())
		}
		(*head) = block
	}
}

//export将活动链写入给定的编写器。
func (bc *BlockChain) Export(w io.Writer) error {
	return bc.ExportN(w, uint64(0), bc.CurrentBlock().NumberU64())
}

//exportn将活动链的一个子集写入给定的编写器。
func (bc *BlockChain) ExportN(w io.Writer, first uint64, last uint64) error {
	bc.chainmu.RLock()
	defer bc.chainmu.RUnlock()

	if first > last {
		return fmt.Errorf("export failed: first (%d) is greater than last (%d)", first, last)
	}
	log.Info("Exporting batch of blocks", "count", last-first+1)

	start, reported := time.Now(), time.Now()
	for nr := first; nr <= last; nr++ {
		block := bc.GetBlockByNumber(nr)
		if block == nil {
			return fmt.Errorf("export failed on #%d: not found", nr)
		}
		if err := block.EncodeRLP(w); err != nil {
			return err
		}
		if time.Since(reported) >= statsReportLimit {
			log.Info("Exporting blocks", "exported", block.NumberU64()-first, "elapsed", common.PrettyDuration(time.Since(start)))
			reported = time.Now()
		}
	}
	return nil
}

//插入将新的头块插入当前的块链。这种方法
//假设该块确实是一个真正的头。它还将重置头部
//头和头快速同步块与此非常相同的块（如果它们较旧）
//或者如果它们在另一条边链上。
//
//注意，此函数假定保持“mu”互斥！
func (bc *BlockChain) insert(block *types.Block) {
//如果木块位于侧链或未知链条上，也应将其他头压到链条上。
	updateHeads := rawdb.ReadCanonicalHash(bc.db, block.NumberU64()) != block.Hash()

//将块添加到规范链编号方案并标记为头
	rawdb.WriteCanonicalHash(bc.db, block.Hash(), block.NumberU64())
	rawdb.WriteHeadBlockHash(bc.db, block.Hash())

	bc.currentBlock.Store(block)

//如果块比我们的头更好或位于不同的链上，则强制更新头
	if updateHeads {
		bc.hc.SetCurrentHeader(block.Header())
		rawdb.WriteHeadFastBlockHash(bc.db, block.Hash())

		bc.currentFastBlock.Store(block)
	}
}

//Genesis检索链的Genesis块。
func (bc *BlockChain) Genesis() *types.Block {
	return bc.genesisBlock
}

//getbody通过以下方式从数据库中检索块体（事务和uncles）
//哈希，如果找到，则缓存它。
func (bc *BlockChain) GetBody(hash common.Hash) *types.Body {
//如果主体已在缓存中，则短路，否则检索
	if cached, ok := bc.bodyCache.Get(hash); ok {
		body := cached.(*types.Body)
		return body
	}
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	body := rawdb.ReadBody(bc.db, hash, *number)
	if body == nil {
		return nil
	}
//缓存下一次找到的正文并返回
	bc.bodyCache.Add(hash, body)
	return body
}

//getBodyrlp通过哈希从数据库中检索以rlp编码的块体，
//如果找到，则缓存它。
func (bc *BlockChain) GetBodyRLP(hash common.Hash) rlp.RawValue {
//如果主体已在缓存中，则短路，否则检索
	if cached, ok := bc.bodyRLPCache.Get(hash); ok {
		return cached.(rlp.RawValue)
	}
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	body := rawdb.ReadBodyRLP(bc.db, hash, *number)
	if len(body) == 0 {
		return nil
	}
//缓存下一次找到的正文并返回
	bc.bodyRLPCache.Add(hash, body)
	return body
}

//hasblock检查数据库中是否完全存在块。
func (bc *BlockChain) HasBlock(hash common.Hash, number uint64) bool {
	if bc.blockCache.Contains(hash) {
		return true
	}
	return rawdb.HasBody(bc.db, hash, number)
}

//hasFastBlock检查数据库中是否完全存在快速块。
func (bc *BlockChain) HasFastBlock(hash common.Hash, number uint64) bool {
	if !bc.HasBlock(hash, number) {
		return false
	}
	if bc.receiptsCache.Contains(hash) {
		return true
	}
	return rawdb.HasReceipts(bc.db, hash, number)
}

//hasstate检查数据库中是否完全存在状态trie。
func (bc *BlockChain) HasState(hash common.Hash) bool {
	_, err := bc.stateCache.OpenTrie(hash)
	return err == nil
}

//hasblockandstate检查块和关联状态trie是否完全存在
//在数据库中或不在数据库中，缓存它（如果存在）。
func (bc *BlockChain) HasBlockAndState(hash common.Hash, number uint64) bool {
//首先检查块本身是否已知
	block := bc.GetBlock(hash, number)
	if block == nil {
		return false
	}
	return bc.HasState(block.Root())
}

//GetBlock按哈希和数字从数据库中检索块，
//如果找到，则缓存它。
func (bc *BlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
//如果块已在缓存中，则短路，否则检索
	if block, ok := bc.blockCache.Get(hash); ok {
		return block.(*types.Block)
	}
	block := rawdb.ReadBlock(bc.db, hash, number)
	if block == nil {
		return nil
	}
//下次缓存找到的块并返回
	bc.blockCache.Add(block.Hash(), block)
	return block
}

//GetBlockByHash通过哈希从数据库中检索一个块，如果找到该块，则将其缓存。
func (bc *BlockChain) GetBlockByHash(hash common.Hash) *types.Block {
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return bc.GetBlock(hash, *number)
}

//GetBlockByNumber按编号从数据库中检索块，并将其缓存
//（与哈希关联）如果找到。
func (bc *BlockChain) GetBlockByNumber(number uint64) *types.Block {
	hash := rawdb.ReadCanonicalHash(bc.db, number)
	if hash == (common.Hash{}) {
		return nil
	}
	return bc.GetBlock(hash, number)
}

//getReceiptsByHash检索给定块中所有事务的收据。
func (bc *BlockChain) GetReceiptsByHash(hash common.Hash) types.Receipts {
	if receipts, ok := bc.receiptsCache.Get(hash); ok {
		return receipts.(types.Receipts)
	}
	number := rawdb.ReadHeaderNumber(bc.db, hash)
	if number == nil {
		return nil
	}
	receipts := rawdb.ReadReceipts(bc.db, hash, *number)
	if receipts == nil {
		return nil
	}
	bc.receiptsCache.Add(hash, receipts)
	return receipts
}

//GetBlocksFromHash返回与哈希和N-1祖先对应的块。
//[被ETH/62否决]
func (bc *BlockChain) GetBlocksFromHash(hash common.Hash, n int) (blocks []*types.Block) {
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	for i := 0; i < n; i++ {
		block := bc.GetBlock(hash, *number)
		if block == nil {
			break
		}
		blocks = append(blocks, block)
		hash = block.ParentHash()
		*number--
	}
	return
}

//getUnbenshinchain从给定块中向后检索所有叔叔，直到
//达到特定距离。
func (bc *BlockChain) GetUnclesInChain(block *types.Block, length int) []*types.Header {
	uncles := []*types.Header{}
	for i := 0; block != nil && i < length; i++ {
		uncles = append(uncles, block.Uncles()...)
		block = bc.GetBlock(block.ParentHash(), block.NumberU64()-1)
	}
	return uncles
}

//trie node检索与trie节点（或代码哈希）关联的一个数据块。
//要么来自短暂的内存缓存，要么来自持久存储。
func (bc *BlockChain) TrieNode(hash common.Hash) ([]byte, error) {
	return bc.stateCache.TrieDB().Node(hash)
}

//停止停止区块链服务。如果任何导入当前正在进行中
//它将使用procInterrupt中止它们。
func (bc *BlockChain) Stop() {
	if !atomic.CompareAndSwapInt32(&bc.running, 0, 1) {
		return
	}
//取消订阅从区块链注册的所有订阅
	bc.scope.Close()
	close(bc.quit)
	atomic.StoreInt32(&bc.procInterrupt, 1)

	bc.wg.Wait()

//在退出之前，请确保最近块的状态也存储到磁盘。
//我们编写了三种不同的状态来捕捉不同的重启场景：
//头：所以一般情况下我们不需要重新处理任何块。
//-head-1:所以如果我们的头变成叔叔，我们就不会进行大重组。
//-HEAD-127：因此我们对重新执行的块数有一个硬限制
	if !bc.cacheConfig.Disabled {
		triedb := bc.stateCache.TrieDB()

		for _, offset := range []uint64{0, 1, triesInMemory - 1} {
			if number := bc.CurrentBlock().NumberU64(); number > offset {
				recent := bc.GetBlockByNumber(number - offset)

				log.Info("Writing cached state to disk", "block", recent.Number(), "hash", recent.Hash(), "root", recent.Root())
				if err := triedb.Commit(recent.Root(), true); err != nil {
					log.Error("Failed to commit recent state trie", "err", err)
				}
			}
		}
		for !bc.triegc.Empty() {
			triedb.Dereference(bc.triegc.PopItem().(common.Hash))
		}
		if size, _ := triedb.Size(); size != 0 {
			log.Error("Dangling trie nodes after full cleanup")
		}
	}
	log.Info("Blockchain manager stopped")
}

func (bc *BlockChain) procFutureBlocks() {
	blocks := make([]*types.Block, 0, bc.futureBlocks.Len())
	for _, hash := range bc.futureBlocks.Keys() {
		if block, exist := bc.futureBlocks.Peek(hash); exist {
			blocks = append(blocks, block.(*types.Block))
		}
	}
	if len(blocks) > 0 {
		types.BlockBy(types.Number).Sort(blocks)

//逐个插入，因为链插入需要块之间的连续祖先
		for i := range blocks {
			bc.InsertChain(blocks[i : i+1])
		}
	}
}

//写入状态写入状态
type WriteStatus byte

const (
	NonStatTy WriteStatus = iota
	CanonStatTy
	SideStatTy
)

//回滚的目的是从数据库中删除一系列链接，而这些链接不是
//足够确定有效。
func (bc *BlockChain) Rollback(chain []common.Hash) {
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	for i := len(chain) - 1; i >= 0; i-- {
		hash := chain[i]

		currentHeader := bc.hc.CurrentHeader()
		if currentHeader.Hash() == hash {
			bc.hc.SetCurrentHeader(bc.GetHeader(currentHeader.ParentHash, currentHeader.Number.Uint64()-1))
		}
		if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock.Hash() == hash {
			newFastBlock := bc.GetBlock(currentFastBlock.ParentHash(), currentFastBlock.NumberU64()-1)
			bc.currentFastBlock.Store(newFastBlock)
			rawdb.WriteHeadFastBlockHash(bc.db, newFastBlock.Hash())
		}
		if currentBlock := bc.CurrentBlock(); currentBlock.Hash() == hash {
			newBlock := bc.GetBlock(currentBlock.ParentHash(), currentBlock.NumberU64()-1)
			bc.currentBlock.Store(newBlock)
			rawdb.WriteHeadBlockHash(bc.db, newBlock.Hash())
		}
	}
}

//setReceiptsData计算收据的所有非共识字段
func SetReceiptsData(config *params.ChainConfig, block *types.Block, receipts types.Receipts) error {
	signer := types.MakeSigner(config, block.Number())

	transactions, logIndex := block.Transactions(), uint(0)
	if len(transactions) != len(receipts) {
		return errors.New("transaction and receipt count mismatch")
	}

	for j := 0; j < len(receipts); j++ {
//可以从事务本身检索事务哈希
		receipts[j].TxHash = transactions[j].Hash()

//合同地址可以从事务本身派生
		if transactions[j].To() == nil {
//获得签名者很昂贵，只有在实际需要的时候才这么做
			from, _ := types.Sender(signer, transactions[j])
			receipts[j].ContractAddress = crypto.CreateAddress(from, transactions[j].Nonce())
		}
//使用过的气体可根据以前的收据进行计算。
		if j == 0 {
			receipts[j].GasUsed = receipts[j].CumulativeGasUsed
		} else {
			receipts[j].GasUsed = receipts[j].CumulativeGasUsed - receipts[j-1].CumulativeGasUsed
		}
//派生的日志字段可以简单地从块和事务中设置。
		for k := 0; k < len(receipts[j].Logs); k++ {
			receipts[j].Logs[k].BlockNumber = block.NumberU64()
			receipts[j].Logs[k].BlockHash = block.Hash()
			receipts[j].Logs[k].TxHash = receipts[j].TxHash
			receipts[j].Logs[k].TxIndex = uint(j)
			receipts[j].Logs[k].Index = logIndex
			logIndex++
		}
	}
	return nil
}

//InsertReceiptChain尝试使用
//交易和收据数据。
func (bc *BlockChain) InsertReceiptChain(blockChain types.Blocks, receiptChain []types.Receipts) (int, error) {
	bc.wg.Add(1)
	defer bc.wg.Done()

//做一个健全的检查，确保提供的链实际上是有序的和链接的
	for i := 1; i < len(blockChain); i++ {
		if blockChain[i].NumberU64() != blockChain[i-1].NumberU64()+1 || blockChain[i].ParentHash() != blockChain[i-1].Hash() {
			log.Error("Non contiguous receipt insert", "number", blockChain[i].Number(), "hash", blockChain[i].Hash(), "parent", blockChain[i].ParentHash(),
				"prevnumber", blockChain[i-1].Number(), "prevhash", blockChain[i-1].Hash())
			return 0, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, blockChain[i-1].NumberU64(),
				blockChain[i-1].Hash().Bytes()[:4], i, blockChain[i].NumberU64(), blockChain[i].Hash().Bytes()[:4], blockChain[i].ParentHash().Bytes()[:4])
		}
	}

	var (
		stats = struct{ processed, ignored int32 }{}
		start = time.Now()
		bytes = 0
		batch = bc.db.NewBatch()
	)
	for i, block := range blockChain {
		receipts := receiptChain[i]
//关闭或处理失败时短路插入
		if atomic.LoadInt32(&bc.procInterrupt) == 1 {
			return 0, nil
		}
//所有者标题未知时短路
		if !bc.HasHeader(block.Hash(), block.NumberU64()) {
			return i, fmt.Errorf("containing header #%d [%x…] unknown", block.Number(), block.Hash().Bytes()[:4])
		}
//如果整个数据已知，则跳过
		if bc.HasBlock(block.Hash(), block.NumberU64()) {
			stats.ignored++
			continue
		}
//计算收据的所有非一致字段
		if err := SetReceiptsData(bc.chainConfig, block, receipts); err != nil {
			return i, fmt.Errorf("failed to set receipts data: %v", err)
		}
//将所有数据写入数据库
		rawdb.WriteBody(batch, block.Hash(), block.NumberU64(), block.Body())
		rawdb.WriteReceipts(batch, block.Hash(), block.NumberU64(), receipts)
		rawdb.WriteTxLookupEntries(batch, block)

		stats.processed++

		if batch.ValueSize() >= ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				return 0, err
			}
			bytes += batch.ValueSize()
			batch.Reset()
		}
	}
	if batch.ValueSize() > 0 {
		bytes += batch.ValueSize()
		if err := batch.Write(); err != nil {
			return 0, err
		}
	}

//如果更好的话，更新head fast sync块
	bc.chainmu.Lock()
	head := blockChain[len(blockChain)-1]
if td := bc.GetTd(head.Hash(), head.NumberU64()); td != nil { //可能发生倒带，在这种情况下跳过
		currentFastBlock := bc.CurrentFastBlock()
		if bc.GetTd(currentFastBlock.Hash(), currentFastBlock.NumberU64()).Cmp(td) < 0 {
			rawdb.WriteHeadFastBlockHash(bc.db, head.Hash())
			bc.currentFastBlock.Store(head)
		}
	}
	bc.chainmu.Unlock()

	context := []interface{}{
		"count", stats.processed, "elapsed", common.PrettyDuration(time.Since(start)),
		"number", head.Number(), "hash", head.Hash(), "age", common.PrettyAge(time.Unix(head.Time().Int64(), 0)),
		"size", common.StorageSize(bytes),
	}
	if stats.ignored > 0 {
		context = append(context, []interface{}{"ignored", stats.ignored}...)
	}
	log.Info("Imported new block receipts", context...)

	return 0, nil
}

var lastWrite uint64

//WriteBlockWithOutState只将块及其元数据写入数据库，
//但不写任何状态。这是用来构建竞争侧叉
//直到他们超过了标准的总难度。
func (bc *BlockChain) WriteBlockWithoutState(block *types.Block, td *big.Int) (err error) {
	bc.wg.Add(1)
	defer bc.wg.Done()

	if err := bc.hc.WriteTd(block.Hash(), block.NumberU64(), td); err != nil {
		return err
	}
	rawdb.WriteBlock(bc.db, block)

	return nil
}

//WriteBlockWithState将块和所有关联状态写入数据库。
func (bc *BlockChain) WriteBlockWithState(block *types.Block, receipts []*types.Receipt, state *state.StateDB) (status WriteStatus, err error) {
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	return bc.writeBlockWithState(block, receipts, state)
}

//WriteBlockWithState将块和所有关联状态写入数据库，
//但IS希望保持链互斥。
func (bc *BlockChain) writeBlockWithState(block *types.Block, receipts []*types.Receipt, state *state.StateDB) (status WriteStatus, err error) {
	bc.wg.Add(1)
	defer bc.wg.Done()

//计算块的总难度
	ptd := bc.GetTd(block.ParentHash(), block.NumberU64()-1)
	if ptd == nil {
		return NonStatTy, consensus.ErrUnknownAncestor
	}
//确保插入期间没有不一致的状态泄漏
	currentBlock := bc.CurrentBlock()
	localTd := bc.GetTd(currentBlock.Hash(), currentBlock.NumberU64())
	externTd := new(big.Int).Add(block.Difficulty(), ptd)

//与规范状态无关，将块本身写入数据库
	if err := bc.hc.WriteTd(block.Hash(), block.NumberU64(), externTd); err != nil {
		return NonStatTy, err
	}
	rawdb.WriteBlock(bc.db, block)

	root, err := state.Commit(bc.chainConfig.IsEIP158(block.Number()))
	if err != nil {
		return NonStatTy, err
	}
	triedb := bc.stateCache.TrieDB()

//如果我们正在运行存档节点，请始终刷新
	if bc.cacheConfig.Disabled {
		if err := triedb.Commit(root, false); err != nil {
			return NonStatTy, err
		}
	} else {
//完整但不是存档节点，请执行正确的垃圾收集
triedb.Reference(root, common.Hash{}) //保持trie活动的元数据引用
		bc.triegc.Push(root, -int64(block.NumberU64()))

		if current := block.NumberU64(); current > triesInMemory {
//如果超出内存限制，将成熟的单例节点刷新到磁盘
			var (
				nodes, imgs = triedb.Size()
				limit       = common.StorageSize(bc.cacheConfig.TrieDirtyLimit) * 1024 * 1024
			)
			if nodes > limit || imgs > 4*1024*1024 {
				triedb.Cap(limit - ethdb.IdealBatchSize)
			}
//找到我们需要承诺的下一个州
			header := bc.GetHeaderByNumber(current - triesInMemory)
			chosen := header.Number.Uint64()

//如果超出超时限制，将整个trie刷新到磁盘
			if bc.gcproc > bc.cacheConfig.TrieTimeLimit {
//如果我们超出了限制，但没有达到足够大的内存缺口，
//警告用户系统正在变得不稳定。
				if chosen < lastWrite+triesInMemory && bc.gcproc >= 2*bc.cacheConfig.TrieTimeLimit {
					log.Info("State in memory for too long, committing", "time", bc.gcproc, "allowance", bc.cacheConfig.TrieTimeLimit, "optimum", float64(chosen-lastWrite)/triesInMemory)
				}
//刷新整个trie并重新启动计数器
				triedb.Commit(header.Root, true)
				lastWrite = chosen
				bc.gcproc = 0
			}
//垃圾收集低于我们要求的写保留的任何内容
			for !bc.triegc.Empty() {
				root, number := bc.triegc.Pop()
				if uint64(-number) > chosen {
					bc.triegc.Push(root, number)
					break
				}
				triedb.Dereference(root.(common.Hash))
			}
		}
	}

//使用批处理写入其他块数据。
	batch := bc.db.NewBatch()
	rawdb.WriteReceipts(batch, block.Hash(), block.NumberU64(), receipts)

//如果总的困难比我们已知的要高，就把它加到规范链中去。
//if语句中的第二个子句减少了自私挖掘的脆弱性。
//请参阅http://www.cs.cornell.edu/~ie53/publications/btcrpocfc.pdf
	reorg := externTd.Cmp(localTd) > 0
	currentBlock = bc.CurrentBlock()
	if !reorg && externTd.Cmp(localTd) == 0 {
//按数字拆分相同的难度块，然后优先选择
//由本地矿工生成的作为规范块的块。
		if block.NumberU64() < currentBlock.NumberU64() {
			reorg = true
		} else if block.NumberU64() == currentBlock.NumberU64() {
			var currentPreserve, blockPreserve bool
			if bc.shouldPreserve != nil {
				currentPreserve, blockPreserve = bc.shouldPreserve(currentBlock), bc.shouldPreserve(block)
			}
			reorg = !currentPreserve && (blockPreserve || mrand.Float64() < 0.5)
		}
	}
	if reorg {
//如果父级不是头块，则重新组织链
		if block.ParentHash() != currentBlock.Hash() {
			if err := bc.reorg(currentBlock, block); err != nil {
				return NonStatTy, err
			}
		}
//为事务/收据查找和预映像编写位置元数据
		rawdb.WriteTxLookupEntries(batch, block)
		rawdb.WritePreimages(batch, state.Preimages())

		status = CanonStatTy
	} else {
		status = SideStatTy
	}
	if err := batch.Write(); err != nil {
		return NonStatTy, err
	}

//树立新的头脑。
	if status == CanonStatTy {
		bc.insert(block)
	}
	bc.futureBlocks.Remove(block.Hash())
	return status, nil
}

//AddFutureBlock检查块是否在允许的最大窗口内
//接受以供将来处理，如果块太远则返回错误
//前面没有添加。
func (bc *BlockChain) addFutureBlock(block *types.Block) error {
	max := big.NewInt(time.Now().Unix() + maxTimeFutureBlocks)
	if block.Time().Cmp(max) > 0 {
		return fmt.Errorf("future block timestamp %v > allowed %v", block.Time(), max)
	}
	bc.futureBlocks.Add(block.Hash(), block)
	return nil
}

//
//用链子或其他方法，创建一个叉子。如果返回错误，它将返回
//失败块的索引号以及描述所执行操作的错误
//错了。
//
//插入完成后，将激发所有累积的事件。
func (bc *BlockChain) InsertChain(chain types.Blocks) (int, error) {
//检查我们有什么有意义的东西要导入
	if len(chain) == 0 {
		return 0, nil
	}
//做一个健全的检查，确保提供的链实际上是有序的和链接的
	for i := 1; i < len(chain); i++ {
		if chain[i].NumberU64() != chain[i-1].NumberU64()+1 || chain[i].ParentHash() != chain[i-1].Hash() {
//断链祖先、记录消息（编程错误）和跳过插入
			log.Error("Non contiguous block insert", "number", chain[i].Number(), "hash", chain[i].Hash(),
				"parent", chain[i].ParentHash(), "prevnumber", chain[i-1].Number(), "prevhash", chain[i-1].Hash())

			return 0, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, chain[i-1].NumberU64(),
				chain[i-1].Hash().Bytes()[:4], i, chain[i].NumberU64(), chain[i].Hash().Bytes()[:4], chain[i].ParentHash().Bytes()[:4])
		}
	}
//预检查通过，开始全块导入
	bc.wg.Add(1)
	bc.chainmu.Lock()
	n, events, logs, err := bc.insertChain(chain, true)
	bc.chainmu.Unlock()
	bc.wg.Done()

	bc.PostChainEvents(events, logs)
	return n, err
}

//insertchain是insertchain的内部实现，它假定
//1）链是连续的，2）链互斥被保持。
//
//此方法被拆分，以便导入需要重新注入的批
//历史块可以这样做，而不释放锁，这可能导致
//赛马行为。如果侧链导入正在进行，以及历史状态
//是导入的，但在实际侧链之前添加新的Canon头
//完成，然后历史状态可以再次修剪
func (bc *BlockChain) insertChain(chain types.Blocks, verifySeals bool) (int, []interface{}, []*types.Log, error) {
//如果链终止，甚至不用麻烦启动U
	if atomic.LoadInt32(&bc.procInterrupt) == 1 {
		return 0, nil, nil, nil
	}
//开始并行签名恢复（签名者将在fork转换时出错，性能损失最小）
	senderCacher.recoverFromBlocks(types.MakeSigner(bc.chainConfig, chain[0].Number()), chain)

//用于传递事件的排队方法。这通常是
//比直接传送更快，而且需要更少的互斥。
//获取。
	var (
		stats         = insertStats{startTime: mclock.Now()}
		events        = make([]interface{}, 0, len(chain))
		lastCanon     *types.Block
		coalescedLogs []*types.Log
	)
//启动并行头验证程序
	headers := make([]*types.Header, len(chain))
	seals := make([]bool, len(chain))

	for i, block := range chain {
		headers[i] = block.Header()
		seals[i] = verifySeals
	}
	abort, results := bc.engine.VerifyHeaders(bc, headers, seals)
	defer close(abort)

//查看第一个块的错误以决定直接导入逻辑
	it := newInsertIterator(chain, results, bc.Validator())

	block, err := it.next()
	switch {
//第一个块被修剪，插入为侧链，只有当td足够长时才重新排序。
	case err == consensus.ErrPrunedAncestor:
		return bc.insertSidechain(it)

//第一个块是未来，将其（和所有子块）推送到未来队列（未知的祖先）
	case err == consensus.ErrFutureBlock || (err == consensus.ErrUnknownAncestor && bc.futureBlocks.Contains(it.first().ParentHash())):
		for block != nil && (it.index == 0 || err == consensus.ErrUnknownAncestor) {
			if err := bc.addFutureBlock(block); err != nil {
				return it.index, events, coalescedLogs, err
			}
			block, err = it.next()
		}
		stats.queued += it.processed()
		stats.ignored += it.remaining()

//如果还有剩余，则标记为“忽略”
		return it.index, events, coalescedLogs, err

//已知第一个块（和状态）
//1。我们做了一个回滚，现在应该重新导入
//2。该块存储为侧链，并基于它的StateRoot，传递一个StateRoot
//来自规范链，尚未验证。
	case err == ErrKnownBlock:
//跳过我们身后所有已知的街区
		current := bc.CurrentBlock().NumberU64()

		for block != nil && err == ErrKnownBlock && current >= block.NumberU64() {
			stats.ignored++
			block, err = it.next()
		}
//通过块导入

//发生其他错误，中止
	case err != nil:
		stats.ignored += len(it.chain)
		bc.reportBlock(block, nil, err)
		return it.index, events, coalescedLogs, err
	}
//第一个块没有验证错误（或跳过链前缀）
	for ; block != nil && err == nil; block, err = it.next() {
//如果链终止，则停止处理块
		if atomic.LoadInt32(&bc.procInterrupt) == 1 {
			log.Debug("Premature abort during blocks processing")
			break
		}
//如果标题是禁止的，直接中止
		if BadHashes[block.Hash()] {
			bc.reportBlock(block, nil, ErrBlacklistedHash)
			return it.index, events, coalescedLogs, ErrBlacklistedHash
		}
//检索父块，它的状态为在上面执行
		start := time.Now()

		parent := it.previous()
		if parent == nil {
			parent = bc.GetBlock(block.ParentHash(), block.NumberU64()-1)
		}
		state, err := state.New(parent.Root(), bc.stateCache)
		if err != nil {
			return it.index, events, coalescedLogs, err
		}
//使用父状态作为参考点处理块。
		t0 := time.Now()
		receipts, logs, usedGas, err := bc.processor.Process(block, state, bc.vmConfig)
		t1 := time.Now()
		if err != nil {
			bc.reportBlock(block, receipts, err)
			return it.index, events, coalescedLogs, err
		}
//使用默认验证器验证状态
		if err := bc.Validator().ValidateState(block, parent, state, receipts, usedGas); err != nil {
			bc.reportBlock(block, receipts, err)
			return it.index, events, coalescedLogs, err
		}
		t2 := time.Now()
		proctime := time.Since(start)

//将块写入链并获取状态。
		status, err := bc.writeBlockWithState(block, receipts, state)
		t3 := time.Now()
		if err != nil {
			return it.index, events, coalescedLogs, err
		}
		blockInsertTimer.UpdateSince(start)
		blockExecutionTimer.Update(t1.Sub(t0))
		blockValidationTimer.Update(t2.Sub(t1))
		blockWriteTimer.Update(t3.Sub(t2))
		switch status {
		case CanonStatTy:
			log.Debug("Inserted new block", "number", block.Number(), "hash", block.Hash(),
				"uncles", len(block.Uncles()), "txs", len(block.Transactions()), "gas", block.GasUsed(),
				"elapsed", common.PrettyDuration(time.Since(start)),
				"root", block.Root())

			coalescedLogs = append(coalescedLogs, logs...)
			events = append(events, ChainEvent{block, block.Hash(), logs})
			lastCanon = block

//仅统计GC处理时间的规范块
			bc.gcproc += proctime

		case SideStatTy:
			log.Debug("Inserted forked block", "number", block.Number(), "hash", block.Hash(),
				"diff", block.Difficulty(), "elapsed", common.PrettyDuration(time.Since(start)),
				"txs", len(block.Transactions()), "gas", block.GasUsed(), "uncles", len(block.Uncles()),
				"root", block.Root())
			events = append(events, ChainSideEvent{block})
		}
		blockInsertTimer.UpdateSince(start)
		stats.processed++
		stats.usedGas += usedGas

		cache, _ := bc.stateCache.TrieDB().Size()
		stats.report(chain, it.index, cache)
	}
//还有街区吗？我们唯一关心的是未来的
	if block != nil && err == consensus.ErrFutureBlock {
		if err := bc.addFutureBlock(block); err != nil {
			return it.index, events, coalescedLogs, err
		}
		block, err = it.next()

		for ; block != nil && err == consensus.ErrUnknownAncestor; block, err = it.next() {
			if err := bc.addFutureBlock(block); err != nil {
				return it.index, events, coalescedLogs, err
			}
			stats.queued++
		}
	}
	stats.ignored += it.remaining()

//如果我们已经进行了链，则附加一个单链头事件
	if lastCanon != nil && bc.CurrentBlock().Hash() == lastCanon.Hash() {
		events = append(events, ChainHeadEvent{lastCanon})
	}
	return it.index, events, coalescedLogs, err
}

//当导入批处理碰到修剪后的祖先时调用InsertSideChain。
//错误，当具有足够旧的叉块的侧链
//找到了。
//
//该方法将所有（头和正文有效）块写入磁盘，然后尝试
//如果td超过当前链，则切换到新链。
func (bc *BlockChain) insertSidechain(it *insertIterator) (int, []interface{}, []*types.Log, error) {
	var (
		externTd *big.Int
		current  = bc.CurrentBlock()
	)
//第一个侧链块错误已被验证为errprunedancestor。
//既然我们不在这里进口它们，我们希望剩下的人知道错误。
//那些。任何其他错误表示块无效，不应写入
//到磁盘。
	block, err := it.current(), consensus.ErrPrunedAncestor
	for ; block != nil && (err == consensus.ErrPrunedAncestor); block, err = it.next() {
//检查该数字的规范化状态根目录
		if number := block.NumberU64(); current.NumberU64() >= number {
			if canonical := bc.GetBlockByNumber(number); canonical != nil && canonical.Root() == block.Root() {
//这很可能是影子国家的攻击。当叉子导入到
//数据库，它最终达到一个未修剪的块高度，我们
//刚刚发现状态已经存在！这意味着侧链块
//指已经存在于我们的佳能链中的状态。
//
//如果不选中，我们现在将继续导入块，而实际上
//已经验证了前面块的状态。
				log.Warn("Sidechain ghost-state attack detected", "number", block.NumberU64(), "sideroot", block.Root(), "canonroot", canonical.Root())

//如果有人合法地将地雷挡在一边，它们仍然会像往常一样被进口。然而，
//当未验证的块明显以修剪为目标时，我们不能冒险将它们写入磁盘。
//机制。
				return it.index, nil, nil, errors.New("sidechain ghost-state attack")
			}
		}
		if externTd == nil {
			externTd = bc.GetTd(block.ParentHash(), block.NumberU64()-1)
		}
		externTd = new(big.Int).Add(externTd, block.Difficulty())

		if !bc.HasBlock(block.Hash(), block.NumberU64()) {
			start := time.Now()
			if err := bc.WriteBlockWithoutState(block, externTd); err != nil {
				return it.index, nil, nil, err
			}
			log.Debug("Inserted sidechain block", "number", block.Number(), "hash", block.Hash(),
				"diff", block.Difficulty(), "elapsed", common.PrettyDuration(time.Since(start)),
				"txs", len(block.Transactions()), "gas", block.GasUsed(), "uncles", len(block.Uncles()),
				"root", block.Root())
		}
	}
//此时，我们已经将所有的侧链块写入数据库。循环结束
//或者是其他错误，或者是全部被处理。如果还有其他的
//错误，我们可以忽略这些块的其余部分。
//
//如果externtd大于本地td，我们现在需要重新导入上一个
//重新生成所需状态的块
	localTd := bc.GetTd(current.Hash(), current.NumberU64())
	if localTd.Cmp(externTd) > 0 {
		log.Info("Sidechain written to disk", "start", it.first().NumberU64(), "end", it.previous().NumberU64(), "sidetd", externTd, "localtd", localTd)
		return it.index, nil, nil, err
	}
//收集所有侧链散列（完整的块可能内存很重）
	var (
		hashes  []common.Hash
		numbers []uint64
	)
	parent := bc.GetHeader(it.previous().Hash(), it.previous().NumberU64())
	for parent != nil && !bc.HasState(parent.Root) {
		hashes = append(hashes, parent.Hash())
		numbers = append(numbers, parent.Number.Uint64())

		parent = bc.GetHeader(parent.ParentHash, parent.Number.Uint64()-1)
	}
	if parent == nil {
		return it.index, nil, nil, errors.New("missing parent")
	}
//导入所有修剪的块以使状态可用
	var (
		blocks []*types.Block
		memory common.StorageSize
	)
	for i := len(hashes) - 1; i >= 0; i-- {
//将下一个块追加到批处理中
		block := bc.GetBlock(hashes[i], numbers[i])

		blocks = append(blocks, block)
		memory += block.Size()

//如果内存使用量增长过大，请导入并继续。可悲的是我们需要抛弃
//所有从通知中引发的事件和日志，因为我们对
//记忆在这里。
		if len(blocks) >= 2048 || memory > 64*1024*1024 {
			log.Info("Importing heavy sidechain segment", "blocks", len(blocks), "start", blocks[0].NumberU64(), "end", block.NumberU64())
			if _, _, _, err := bc.insertChain(blocks, false); err != nil {
				return 0, nil, nil, err
			}
			blocks, memory = blocks[:0], 0

//如果链终止，则停止处理块
			if atomic.LoadInt32(&bc.procInterrupt) == 1 {
				log.Debug("Premature abort during blocks processing")
				return 0, nil, nil, nil
			}
		}
	}
	if len(blocks) > 0 {
		log.Info("Importing sidechain segment", "start", blocks[0].NumberU64(), "end", blocks[len(blocks)-1].NumberU64())
		return bc.insertChain(blocks, false)
	}
	return 0, nil, nil, nil
}

//REORG需要两个块，一个旧链和一个新链，并将重建块并插入它们
//成为新规范链的一部分并累积潜在的丢失事务，然后发布
//关于他们的事件
func (bc *BlockChain) reorg(oldBlock, newBlock *types.Block) error {
	var (
		newChain    types.Blocks
		oldChain    types.Blocks
		commonBlock *types.Block
		deletedTxs  types.Transactions
		deletedLogs []*types.Log
//CollectLogs收集在
//处理与给定哈希对应的块。
//这些日志随后被宣布为已删除。
		collectLogs = func(hash common.Hash) {
//合并日志并设置为“已删除”。
			number := bc.hc.GetBlockNumber(hash)
			if number == nil {
				return
			}
			receipts := rawdb.ReadReceipts(bc.db, hash, *number)
			for _, receipt := range receipts {
				for _, log := range receipt.Logs {
					del := *log
					del.Removed = true
					deletedLogs = append(deletedLogs, &del)
				}
			}
		}
	)

//先降低谁是上界
	if oldBlock.NumberU64() > newBlock.NumberU64() {
//减少旧链条
		for ; oldBlock != nil && oldBlock.NumberU64() != newBlock.NumberU64(); oldBlock = bc.GetBlock(oldBlock.ParentHash(), oldBlock.NumberU64()-1) {
			oldChain = append(oldChain, oldBlock)
			deletedTxs = append(deletedTxs, oldBlock.Transactions()...)

			collectLogs(oldBlock.Hash())
		}
	} else {
//减少新的链并附加新的链块以便以后插入
		for ; newBlock != nil && newBlock.NumberU64() != oldBlock.NumberU64(); newBlock = bc.GetBlock(newBlock.ParentHash(), newBlock.NumberU64()-1) {
			newChain = append(newChain, newBlock)
		}
	}
	if oldBlock == nil {
		return fmt.Errorf("Invalid old chain")
	}
	if newBlock == nil {
		return fmt.Errorf("Invalid new chain")
	}

	for {
		if oldBlock.Hash() == newBlock.Hash() {
			commonBlock = oldBlock
			break
		}

		oldChain = append(oldChain, oldBlock)
		newChain = append(newChain, newBlock)
		deletedTxs = append(deletedTxs, oldBlock.Transactions()...)
		collectLogs(oldBlock.Hash())

		oldBlock, newBlock = bc.GetBlock(oldBlock.ParentHash(), oldBlock.NumberU64()-1), bc.GetBlock(newBlock.ParentHash(), newBlock.NumberU64()-1)
		if oldBlock == nil {
			return fmt.Errorf("Invalid old chain")
		}
		if newBlock == nil {
			return fmt.Errorf("Invalid new chain")
		}
	}
//确保用户看到大量的订单
	if len(oldChain) > 0 && len(newChain) > 0 {
		logFn := log.Debug
		if len(oldChain) > 63 {
			logFn = log.Warn
		}
		logFn("Chain split detected", "number", commonBlock.Number(), "hash", commonBlock.Hash(),
			"drop", len(oldChain), "dropfrom", oldChain[0].Hash(), "add", len(newChain), "addfrom", newChain[0].Hash())
	} else {
		log.Error("Impossible reorg, please file an issue", "oldnum", oldBlock.Number(), "oldhash", oldBlock.Hash(), "newnum", newBlock.Number(), "newhash", newBlock.Hash())
	}
//插入新链条，注意正确的增量顺序
	var addedTxs types.Transactions
	for i := len(newChain) - 1; i >= 0; i-- {
//以规范的方式插入块，重新编写历史记录
		bc.insert(newChain[i])
//为基于哈希的交易/收据搜索写入查找条目
		rawdb.WriteTxLookupEntries(bc.db, newChain[i])
		addedTxs = append(addedTxs, newChain[i].Transactions()...)
	}
//计算已删除和已添加交易记录之间的差额
	diff := types.TxDifference(deletedTxs, addedTxs)
//当事务从数据库中删除时，这意味着
//在fork中创建的收据也必须删除
	batch := bc.db.NewBatch()
	for _, tx := range diff {
		rawdb.DeleteTxLookupEntry(batch, tx.Hash())
	}
	batch.Write()

	if len(deletedLogs) > 0 {
		go bc.rmLogsFeed.Send(RemovedLogsEvent{deletedLogs})
	}
	if len(oldChain) > 0 {
		go func() {
			for _, block := range oldChain {
				bc.chainSideFeed.Send(ChainSideEvent{Block: block})
			}
		}()
	}

	return nil
}

//Postchainevents迭代链插入生成的事件，并
//将它们发布到事件提要中。
//托多：不应该暴露后遗症。应在WriteBlock中发布链事件。
func (bc *BlockChain) PostChainEvents(events []interface{}, logs []*types.Log) {
//发布事件日志以进行进一步处理
	if logs != nil {
		bc.logsFeed.Send(logs)
	}
	for _, event := range events {
		switch ev := event.(type) {
		case ChainEvent:
			bc.chainFeed.Send(ev)

		case ChainHeadEvent:
			bc.chainHeadFeed.Send(ev)

		case ChainSideEvent:
			bc.chainSideFeed.Send(ev)
		}
	}
}

func (bc *BlockChain) update() {
	futureTimer := time.NewTicker(5 * time.Second)
	defer futureTimer.Stop()
	for {
		select {
		case <-futureTimer.C:
			bc.procFutureBlocks()
		case <-bc.quit:
			return
		}
	}
}

//bad blocks返回客户端在网络上看到的最后一个“坏块”的列表
func (bc *BlockChain) BadBlocks() []*types.Block {
	blocks := make([]*types.Block, 0, bc.badBlocks.Len())
	for _, hash := range bc.badBlocks.Keys() {
		if blk, exist := bc.badBlocks.Peek(hash); exist {
			block := blk.(*types.Block)
			blocks = append(blocks, block)
		}
	}
	return blocks
}

//addbadblock向坏块lru缓存添加坏块
func (bc *BlockChain) addBadBlock(block *types.Block) {
	bc.badBlocks.Add(block.Hash(), block)
}

//ReportBlock记录错误的块错误。
func (bc *BlockChain) reportBlock(block *types.Block, receipts types.Receipts, err error) {
	bc.addBadBlock(block)

	var receiptString string
	for i, receipt := range receipts {
		receiptString += fmt.Sprintf("\t %d: cumulative: %v gas: %v contract: %v status: %v tx: %v logs: %v bloom: %x state: %x\n",
			i, receipt.CumulativeGasUsed, receipt.GasUsed, receipt.ContractAddress.Hex(),
			receipt.Status, receipt.TxHash.Hex(), receipt.Logs, receipt.Bloom, receipt.PostState)
	}
	log.Error(fmt.Sprintf(`
########## BAD BLOCK #########
Chain config: %v

Number: %v
Hash: 0x%x
%v

Error: %v
##############################
`, bc.chainConfig, block.Number(), block.Hash(), receiptString, err))
}

//insert header chain尝试将给定的头链插入本地
//链，可能创建REORG。如果返回错误，它将返回
//失败头的索引号以及描述出错原因的错误。
//
//verify参数可用于微调非ce验证
//是否应该做。可选检查背后的原因是
//其中的头检索机制已经需要验证nonce，以及
//因为nonce可以被稀疏地验证，不需要检查每一个。
func (bc *BlockChain) InsertHeaderChain(chain []*types.Header, checkFreq int) (int, error) {
	start := time.Now()
	if i, err := bc.hc.ValidateHeaderChain(chain, checkFreq); err != nil {
		return i, err
	}

//确保一次只有一个线程操作链
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	bc.wg.Add(1)
	defer bc.wg.Done()

	whFunc := func(header *types.Header) error {
		_, err := bc.hc.WriteHeader(header)
		return err
	}
	return bc.hc.InsertHeaderChain(chain, whFunc, start)
}

//当前头检索规范链的当前头。这个
//从HeaderChain的内部缓存中检索头。
func (bc *BlockChain) CurrentHeader() *types.Header {
	return bc.hc.CurrentHeader()
}

//gettd从
//按哈希和数字排列的数据库，如果找到，则将其缓存。
func (bc *BlockChain) GetTd(hash common.Hash, number uint64) *big.Int {
	return bc.hc.GetTd(hash, number)
}

//getDByHash从
//通过哈希对数据库进行缓存（如果找到）。
func (bc *BlockChain) GetTdByHash(hash common.Hash) *big.Int {
	return bc.hc.GetTdByHash(hash)
}

//GetHeader按哈希和数字从数据库中检索块头，
//如果找到，则缓存它。
func (bc *BlockChain) GetHeader(hash common.Hash, number uint64) *types.Header {
	return bc.hc.GetHeader(hash, number)
}

//GetHeaderByHash通过哈希从数据库中检索块头，如果
//找到了。
func (bc *BlockChain) GetHeaderByHash(hash common.Hash) *types.Header {
	return bc.hc.GetHeaderByHash(hash)
}

//hasheader检查数据库中是否存在块头，缓存
//如果存在的话。
func (bc *BlockChain) HasHeader(hash common.Hash, number uint64) bool {
	return bc.hc.HasHeader(hash, number)
}

//GetBlockHashesFromHash从给定的
//哈什，向创世纪街区走去。
func (bc *BlockChain) GetBlockHashesFromHash(hash common.Hash, max uint64) []common.Hash {
	return bc.hc.GetBlockHashesFromHash(hash, max)
}

//getAncestor检索给定块的第n个祖先。它假定给定的块或
//它的近亲是典型的。maxnoncanonical指向向下计数器，限制
//到达规范链之前要单独检查的块数。
//
//注意：ancestor==0返回相同的块，1返回其父块，依此类推。
func (bc *BlockChain) GetAncestor(hash common.Hash, number, ancestor uint64, maxNonCanonical *uint64) (common.Hash, uint64) {
	bc.chainmu.RLock()
	defer bc.chainmu.RUnlock()

	return bc.hc.GetAncestor(hash, number, ancestor, maxNonCanonical)
}

//GetHeaderByNumber按编号从数据库中检索块头，
//如果找到，则缓存它（与其哈希关联）。
func (bc *BlockChain) GetHeaderByNumber(number uint64) *types.Header {
	return bc.hc.GetHeaderByNumber(number)
}

//config检索区块链的链配置。
func (bc *BlockChain) Config() *params.ChainConfig { return bc.chainConfig }

//引擎检索区块链的共识引擎。
func (bc *BlockChain) Engine() consensus.Engine { return bc.engine }

//subscripreMovedLogSevent注册removedLogSevent的订阅。
func (bc *BlockChain) SubscribeRemovedLogsEvent(ch chan<- RemovedLogsEvent) event.Subscription {
	return bc.scope.Track(bc.rmLogsFeed.Subscribe(ch))
}

//subscribeChainevent注册chainEvent的订阅。
func (bc *BlockChain) SubscribeChainEvent(ch chan<- ChainEvent) event.Subscription {
	return bc.scope.Track(bc.chainFeed.Subscribe(ch))
}

//subscribeChainHeadEvent注册chainHeadEvent的订阅。
func (bc *BlockChain) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return bc.scope.Track(bc.chainHeadFeed.Subscribe(ch))
}

//subscribeChainSideEvent注册chainSideEvent的订阅。
func (bc *BlockChain) SubscribeChainSideEvent(ch chan<- ChainSideEvent) event.Subscription {
	return bc.scope.Track(bc.chainSideFeed.Subscribe(ch))
}

//subscriptLogSevent注册了一个订阅[]*types.log。
func (bc *BlockChain) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return bc.scope.Track(bc.logsFeed.Subscribe(ch))
}
