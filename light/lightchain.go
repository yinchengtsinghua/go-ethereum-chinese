
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
	"errors"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/hashicorp/golang-lru"
)

var (
	bodyCacheLimit  = 256
	blockCacheLimit = 256
)

//lightchain表示默认情况下只处理块的规范链
//通过ODR按需下载块体和收据。
//接口。它只在链插入期间进行头验证。
type LightChain struct {
	hc            *core.HeaderChain
	indexerConfig *IndexerConfig
	chainDb       ethdb.Database
	odr           OdrBackend
	chainFeed     event.Feed
	chainSideFeed event.Feed
	chainHeadFeed event.Feed
	scope         event.SubscriptionScope
	genesisBlock  *types.Block

	chainmu sync.RWMutex

bodyCache    *lru.Cache //缓存最新的块体
bodyRLPCache *lru.Cache //以rlp编码格式缓存最新的块体
blockCache   *lru.Cache //缓存最近的整个块

	quit    chan struct{}
running int32 //必须自动调用running
//必须原子地调用ProcInterrupt
procInterrupt int32 //用于块处理的中断信号器
	wg            sync.WaitGroup

	engine consensus.Engine
}

//newlightchain使用信息返回完全初始化的光链
//在数据库中可用。它初始化默认的以太坊头文件
//验证器。
func NewLightChain(odr OdrBackend, config *params.ChainConfig, engine consensus.Engine) (*LightChain, error) {
	bodyCache, _ := lru.New(bodyCacheLimit)
	bodyRLPCache, _ := lru.New(bodyCacheLimit)
	blockCache, _ := lru.New(blockCacheLimit)

	bc := &LightChain{
		chainDb:       odr.Database(),
		indexerConfig: odr.IndexerConfig(),
		odr:           odr,
		quit:          make(chan struct{}),
		bodyCache:     bodyCache,
		bodyRLPCache:  bodyRLPCache,
		blockCache:    blockCache,
		engine:        engine,
	}
	var err error
	bc.hc, err = core.NewHeaderChain(odr.Database(), config, bc.engine, bc.getProcInterrupt)
	if err != nil {
		return nil, err
	}
	bc.genesisBlock, _ = bc.GetBlockByNumber(NoOdr, 0)
	if bc.genesisBlock == nil {
		return nil, core.ErrNoGenesis
	}
	if cp, ok := trustedCheckpoints[bc.genesisBlock.Hash()]; ok {
		bc.addTrustedCheckpoint(cp)
	}
	if err := bc.loadLastState(); err != nil {
		return nil, err
	}
//检查块哈希的当前状态，确保链中没有任何坏块
	for hash := range core.BadHashes {
		if header := bc.GetHeaderByHash(hash); header != nil {
			log.Error("Found bad hash, rewinding chain", "number", header.Number, "hash", header.ParentHash)
			bc.SetHead(header.Number.Uint64() - 1)
			log.Error("Chain rewind was successful, resuming normal operation")
		}
	}
	return bc, nil
}

//AddTrustedCheckpoint向区块链添加受信任的检查点
func (self *LightChain) addTrustedCheckpoint(cp *params.TrustedCheckpoint) {
	if self.odr.ChtIndexer() != nil {
		StoreChtRoot(self.chainDb, cp.SectionIndex, cp.SectionHead, cp.CHTRoot)
		self.odr.ChtIndexer().AddCheckpoint(cp.SectionIndex, cp.SectionHead)
	}
	if self.odr.BloomTrieIndexer() != nil {
		StoreBloomTrieRoot(self.chainDb, cp.SectionIndex, cp.SectionHead, cp.BloomRoot)
		self.odr.BloomTrieIndexer().AddCheckpoint(cp.SectionIndex, cp.SectionHead)
	}
	if self.odr.BloomIndexer() != nil {
		self.odr.BloomIndexer().AddCheckpoint(cp.SectionIndex, cp.SectionHead)
	}
	log.Info("Added trusted checkpoint", "chain", cp.Name, "block", (cp.SectionIndex+1)*self.indexerConfig.ChtSize-1, "hash", cp.SectionHead)
}

func (self *LightChain) getProcInterrupt() bool {
	return atomic.LoadInt32(&self.procInterrupt) == 1
}

//ODR返回链的ODR后端
func (self *LightChain) Odr() OdrBackend {
	return self.odr
}

//loadLastState从数据库加载最后一个已知的链状态。这种方法
//假定保持链管理器互斥锁。
func (self *LightChain) loadLastState() error {
	if head := rawdb.ReadHeadHeaderHash(self.chainDb); head == (common.Hash{}) {
//数据库已损坏或为空，从头开始初始化
		self.Reset()
	} else {
		if header := self.GetHeaderByHash(head); header != nil {
			self.hc.SetCurrentHeader(header)
		}
	}

//发布状态日志并返回
	header := self.hc.CurrentHeader()
	headerTd := self.GetTd(header.Hash(), header.Number.Uint64())
	log.Info("Loaded most recent local header", "number", header.Number, "hash", header.Hash(), "td", headerTd, "age", common.PrettyAge(time.Unix(header.Time.Int64(), 0)))

	return nil
}

//sethead将本地链重绕到新的head。一切都在新的之上
//头将被删除，新的一套。
func (bc *LightChain) SetHead(head uint64) {
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	bc.hc.SetHead(head, nil)
	bc.loadLastState()
}

//gas limit返回当前头块的气体限制。
func (self *LightChain) GasLimit() uint64 {
	return self.hc.CurrentHeader().GasLimit
}

//重置清除整个区块链，将其恢复到其创始状态。
func (bc *LightChain) Reset() {
	bc.ResetWithGenesisBlock(bc.genesisBlock)
}

//ResetWithGenerisBlock清除整个区块链，将其恢复到
//指定的创世状态。
func (bc *LightChain) ResetWithGenesisBlock(genesis *types.Block) {
//转储整个块链并清除缓存
	bc.SetHead(0)

	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

//准备Genesis块并重新初始化链
	rawdb.WriteTd(bc.chainDb, genesis.Hash(), genesis.NumberU64(), genesis.Difficulty())
	rawdb.WriteBlock(bc.chainDb, genesis)

	bc.genesisBlock = genesis
	bc.hc.SetGenesis(bc.genesisBlock.Header())
	bc.hc.SetCurrentHeader(bc.genesisBlock.Header())
}

//访问器

//引擎检索轻链的共识引擎。
func (bc *LightChain) Engine() consensus.Engine { return bc.engine }

//Genesis返回Genesis块
func (bc *LightChain) Genesis() *types.Block {
	return bc.genesisBlock
}

//State返回基于当前头块的新可变状态。
func (bc *LightChain) State() (*state.StateDB, error) {
	return nil, errors.New("not implemented, needs client/server interface split")
}

//getbody从数据库中检索块体（事务和uncles）
//或者通过散列来实现ODR服务，如果找到了，就缓存它。
func (self *LightChain) GetBody(ctx context.Context, hash common.Hash) (*types.Body, error) {
//如果主体已在缓存中，则短路，否则检索
	if cached, ok := self.bodyCache.Get(hash); ok {
		body := cached.(*types.Body)
		return body, nil
	}
	number := self.hc.GetBlockNumber(hash)
	if number == nil {
		return nil, errors.New("unknown block")
	}
	body, err := GetBody(ctx, self.odr, hash, *number)
	if err != nil {
		return nil, err
	}
//缓存下一次找到的正文并返回
	self.bodyCache.Add(hash, body)
	return body, nil
}

//getBodyrlp从数据库中检索以rlp编码的块体，或者
//通过哈希的ODR服务，如果找到，将其缓存。
func (self *LightChain) GetBodyRLP(ctx context.Context, hash common.Hash) (rlp.RawValue, error) {
//如果主体已在缓存中，则短路，否则检索
	if cached, ok := self.bodyRLPCache.Get(hash); ok {
		return cached.(rlp.RawValue), nil
	}
	number := self.hc.GetBlockNumber(hash)
	if number == nil {
		return nil, errors.New("unknown block")
	}
	body, err := GetBodyRLP(ctx, self.odr, hash, *number)
	if err != nil {
		return nil, err
	}
//缓存下一次找到的正文并返回
	self.bodyRLPCache.Add(hash, body)
	return body, nil
}

//HasBlock checks if a block is fully present in the database or not, caching
//如果存在的话。
func (bc *LightChain) HasBlock(hash common.Hash, number uint64) bool {
	blk, _ := bc.GetBlock(NoOdr, hash, number)
	return blk != nil
}

//GetBlock通过哈希和数字从数据库或ODR服务中检索块，
//如果找到，则缓存它。
func (self *LightChain) GetBlock(ctx context.Context, hash common.Hash, number uint64) (*types.Block, error) {
//如果块已在缓存中，则短路，否则检索
	if block, ok := self.blockCache.Get(hash); ok {
		return block.(*types.Block), nil
	}
	block, err := GetBlock(ctx, self.odr, hash, number)
	if err != nil {
		return nil, err
	}
//下次缓存找到的块并返回
	self.blockCache.Add(block.Hash(), block)
	return block, nil
}

//GetBlockByHash通过哈希从数据库或ODR服务中检索块，
//如果找到，则缓存它。
func (self *LightChain) GetBlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	number := self.hc.GetBlockNumber(hash)
	if number == nil {
		return nil, errors.New("unknown block")
	}
	return self.GetBlock(ctx, hash, *number)
}

//GetBlockByNumber通过以下方式从数据库或ODR服务中检索块
//数字，如果找到，则缓存它（与其哈希关联）。
func (self *LightChain) GetBlockByNumber(ctx context.Context, number uint64) (*types.Block, error) {
	hash, err := GetCanonicalHash(ctx, self.odr, number)
	if hash == (common.Hash{}) || err != nil {
		return nil, err
	}
	return self.GetBlock(ctx, hash, number)
}

//停止停止区块链服务。如果任何导入当前正在进行中
//它将使用procInterrupt中止它们。
func (bc *LightChain) Stop() {
	if !atomic.CompareAndSwapInt32(&bc.running, 0, 1) {
		return
	}
	close(bc.quit)
	atomic.StoreInt32(&bc.procInterrupt, 1)

	bc.wg.Wait()
	log.Info("Blockchain manager stopped")
}

//回滚的目的是从数据库中删除一系列链接，而这些链接不是
//足够确定有效。
func (self *LightChain) Rollback(chain []common.Hash) {
	self.chainmu.Lock()
	defer self.chainmu.Unlock()

	for i := len(chain) - 1; i >= 0; i-- {
		hash := chain[i]

		if head := self.hc.CurrentHeader(); head.Hash() == hash {
			self.hc.SetCurrentHeader(self.GetHeader(head.ParentHash, head.Number.Uint64()-1))
		}
	}
}

//Postchainevents迭代链插入生成的事件，并
//将它们发布到事件提要中。
func (self *LightChain) postChainEvents(events []interface{}) {
	for _, event := range events {
		switch ev := event.(type) {
		case core.ChainEvent:
			if self.CurrentHeader().Hash() == ev.Hash {
				self.chainHeadFeed.Send(core.ChainHeadEvent{Block: ev.Block})
			}
			self.chainFeed.Send(ev)
		case core.ChainSideEvent:
			self.chainSideFeed.Send(ev)
		}
	}
}

//insert header chain尝试将给定的头链插入本地
//链，可能创建REORG。如果返回错误，它将返回
//失败头的索引号以及描述出错原因的错误。
//
//verify参数可用于微调非ce验证
//是否应该做。可选检查背后的原因是
//其中的头检索机制已经需要验证nonce，以及
//因为nonce可以被稀疏地验证，不需要检查每一个。
//
//对于光链，插入头链也创建和发布光
//必要时链接事件。
func (self *LightChain) InsertHeaderChain(chain []*types.Header, checkFreq int) (int, error) {
	start := time.Now()
	if i, err := self.hc.ValidateHeaderChain(chain, checkFreq); err != nil {
		return i, err
	}

//确保一次只有一个线程操作链
	self.chainmu.Lock()
	defer self.chainmu.Unlock()

	self.wg.Add(1)
	defer self.wg.Done()

	var events []interface{}
	whFunc := func(header *types.Header) error {
		status, err := self.hc.WriteHeader(header)

		switch status {
		case core.CanonStatTy:
			log.Debug("Inserted new header", "number", header.Number, "hash", header.Hash())
			events = append(events, core.ChainEvent{Block: types.NewBlockWithHeader(header), Hash: header.Hash()})

		case core.SideStatTy:
			log.Debug("Inserted forked header", "number", header.Number, "hash", header.Hash())
			events = append(events, core.ChainSideEvent{Block: types.NewBlockWithHeader(header)})
		}
		return err
	}
	i, err := self.hc.InsertHeaderChain(chain, whFunc, start)
	self.postChainEvents(events)
	return i, err
}

//当前头检索规范链的当前头。这个
//从HeaderChain的内部缓存中检索头。
func (self *LightChain) CurrentHeader() *types.Header {
	return self.hc.CurrentHeader()
}

//gettd从
//按哈希和数字排列的数据库，如果找到，则将其缓存。
func (self *LightChain) GetTd(hash common.Hash, number uint64) *big.Int {
	return self.hc.GetTd(hash, number)
}

//getDByHash从
//通过哈希对数据库进行缓存（如果找到）。
func (self *LightChain) GetTdByHash(hash common.Hash) *big.Int {
	return self.hc.GetTdByHash(hash)
}

//GetHeader按哈希和数字从数据库中检索块头，
//如果找到，则缓存它。
func (self *LightChain) GetHeader(hash common.Hash, number uint64) *types.Header {
	return self.hc.GetHeader(hash, number)
}

//GetHeaderByHash通过哈希从数据库中检索块头，如果
//找到了。
func (self *LightChain) GetHeaderByHash(hash common.Hash) *types.Header {
	return self.hc.GetHeaderByHash(hash)
}

//hasheader检查数据库中是否存在块头，缓存
//如果存在的话。
func (bc *LightChain) HasHeader(hash common.Hash, number uint64) bool {
	return bc.hc.HasHeader(hash, number)
}

//GetBlockHashesFromHash从给定的
//哈什，向创世纪街区走去。
func (self *LightChain) GetBlockHashesFromHash(hash common.Hash, max uint64) []common.Hash {
	return self.hc.GetBlockHashesFromHash(hash, max)
}

//getAncestor检索给定块的第n个祖先。它假定给定的块或
//它的近亲是典型的。maxnoncanonical指向向下计数器，限制
//到达规范链之前要单独检查的块数。
//
//注意：ancestor==0返回相同的块，1返回其父块，依此类推。
func (bc *LightChain) GetAncestor(hash common.Hash, number, ancestor uint64, maxNonCanonical *uint64) (common.Hash, uint64) {
	bc.chainmu.RLock()
	defer bc.chainmu.RUnlock()

	return bc.hc.GetAncestor(hash, number, ancestor, maxNonCanonical)
}

//GetHeaderByNumber按编号从数据库中检索块头，
//如果找到，则缓存它（与其哈希关联）。
func (self *LightChain) GetHeaderByNumber(number uint64) *types.Header {
	return self.hc.GetHeaderByNumber(number)
}

//GetHeaderByNumberODR从数据库或网络检索块头
//按数字，如果找到，则缓存它（与其哈希关联）。
func (self *LightChain) GetHeaderByNumberOdr(ctx context.Context, number uint64) (*types.Header, error) {
	if header := self.hc.GetHeaderByNumber(number); header != nil {
		return header, nil
	}
	return GetHeaderByNumber(ctx, self.odr, number)
}

//config检索头链的链配置。
func (self *LightChain) Config() *params.ChainConfig { return self.hc.Config() }

func (self *LightChain) SyncCht(ctx context.Context) bool {
//如果没有CHT索引器，请中止
	if self.odr.ChtIndexer() == nil {
		return false
	}
//确保远程CHT头在我们前面
	head := self.CurrentHeader().Number.Uint64()
	sections, _, _ := self.odr.ChtIndexer().Sections()

	latest := sections*self.indexerConfig.ChtSize - 1
	if clique := self.hc.Config().Clique; clique != nil {
latest -= latest % clique.Epoch //集团的时代快照
	}
	if head >= latest {
		return false
	}
//检索最新的有用头并对其进行更新
	if header, err := GetHeaderByNumber(ctx, self.odr, latest); header != nil && err == nil {
		self.chainmu.Lock()
		defer self.chainmu.Unlock()

//确保链条在检索时没有移动过最新的块。
		if self.hc.CurrentHeader().Number.Uint64() < header.Number.Uint64() {
			log.Info("Updated latest header based on CHT", "number", header.Number, "hash", header.Hash(), "age", common.PrettyAge(time.Unix(header.Time.Int64(), 0)))
			self.hc.SetCurrentHeader(header)
		}
		return true
	}
	return false
}

//lockchain为读取而锁定链互斥体，以便可以
//在确保它们属于同一版本的链时检索
func (self *LightChain) LockChain() {
	self.chainmu.RLock()
}

//解锁链解锁链互斥体
func (self *LightChain) UnlockChain() {
	self.chainmu.RUnlock()
}

//subscribeChainevent注册chainEvent的订阅。
func (self *LightChain) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return self.scope.Track(self.chainFeed.Subscribe(ch))
}

//subscribeChainHeadEvent注册chainHeadEvent的订阅。
func (self *LightChain) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return self.scope.Track(self.chainHeadFeed.Subscribe(ch))
}

//subscribeChainSideEvent注册chainSideEvent的订阅。
func (self *LightChain) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return self.scope.Track(self.chainSideFeed.Subscribe(ch))
}

//subscriptLogSevent实现了filters.backend的接口
//LightChain不发送日志事件，因此返回空订阅。
func (self *LightChain) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return self.scope.Track(new(event.Feed).Subscribe(ch))
}

//subscripreMovedLogSevent实现筛选器的接口。后端
//LightChain不发送core.removedLogSevent，因此返回空订阅。
func (self *LightChain) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return self.scope.Track(new(event.Feed).Subscribe(ch))
}
