
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

package core

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	mrand "math/rand"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/hashicorp/golang-lru"
)

const (
	headerCacheLimit = 512
	tdCacheLimit     = 1024
	numberCacheLimit = 2048
)

//HeaderChain实现由共享的基本块头链逻辑
//core.blockback和light.lightchain。它本身不可用，只是
//两种结构的一部分。
//它也不是线程安全的，封装链结构应该这样做
//必要的互斥锁/解锁。
type HeaderChain struct {
	config *params.ChainConfig

	chainDb       ethdb.Database
	genesisHeader *types.Header

currentHeader     atomic.Value //收割台链条的当前收割台（可能位于区块链上方！）
currentHeaderHash common.Hash  //头链当前头的哈希（禁止随时重新计算）

headerCache *lru.Cache //缓存最新的块头
tdCache     *lru.Cache //缓存最近的块总困难
numberCache *lru.Cache //缓存最新的块号

	procInterrupt func() bool

	rand   *mrand.Rand
	engine consensus.Engine
}

//新的headerchain创建新的headerchain结构。
//GetValidator应返回父级的验证程序
//procInterrupt指向父级的中断信号量
//wg指向父级的关闭等待组
func NewHeaderChain(chainDb ethdb.Database, config *params.ChainConfig, engine consensus.Engine, procInterrupt func() bool) (*HeaderChain, error) {
	headerCache, _ := lru.New(headerCacheLimit)
	tdCache, _ := lru.New(tdCacheLimit)
	numberCache, _ := lru.New(numberCacheLimit)

//设定一个快速但加密的随机生成器
	seed, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	hc := &HeaderChain{
		config:        config,
		chainDb:       chainDb,
		headerCache:   headerCache,
		tdCache:       tdCache,
		numberCache:   numberCache,
		procInterrupt: procInterrupt,
		rand:          mrand.New(mrand.NewSource(seed.Int64())),
		engine:        engine,
	}

	hc.genesisHeader = hc.GetHeaderByNumber(0)
	if hc.genesisHeader == nil {
		return nil, ErrNoGenesis
	}

	hc.currentHeader.Store(hc.genesisHeader)
	if head := rawdb.ReadHeadBlockHash(chainDb); head != (common.Hash{}) {
		if chead := hc.GetHeaderByHash(head); chead != nil {
			hc.currentHeader.Store(chead)
		}
	}
	hc.currentHeaderHash = hc.CurrentHeader().Hash()

	return hc, nil
}

//GetBlockNumber检索属于给定哈希的块号
//从缓存或数据库
func (hc *HeaderChain) GetBlockNumber(hash common.Hash) *uint64 {
	if cached, ok := hc.numberCache.Get(hash); ok {
		number := cached.(uint64)
		return &number
	}
	number := rawdb.ReadHeaderNumber(hc.chainDb, hash)
	if number != nil {
		hc.numberCache.Add(hash, *number)
	}
	return number
}

//WRITEHEADER将头写入本地链，因为它的父级是
//已经知道了。如果新插入的标题的总难度变为
//大于当前已知的td，则重新路由规范链。
//
//注意：此方法与同时插入块不同时安全
//在链中，由于无法模拟由重组引起的副作用
//没有真正的街区。因此，只应直接编写头文件
//在两种情况下：纯头段操作模式（轻客户端），或正确
//单独的头段/块阶段（非存档客户端）。
func (hc *HeaderChain) WriteHeader(header *types.Header) (status WriteStatus, err error) {
//缓存一些值以防止常量重新计算
	var (
		hash   = header.Hash()
		number = header.Number.Uint64()
	)
//计算收割台的总难度
	ptd := hc.GetTd(header.ParentHash, number-1)
	if ptd == nil {
		return NonStatTy, consensus.ErrUnknownAncestor
	}
	localTd := hc.GetTd(hc.currentHeaderHash, hc.CurrentHeader().Number.Uint64())
	externTd := new(big.Int).Add(header.Difficulty, ptd)

//与规范状态无关，将td和header写入数据库
	if err := hc.WriteTd(hash, number, externTd); err != nil {
		log.Crit("Failed to write header total difficulty", "err", err)
	}
	rawdb.WriteHeader(hc.chainDb, header)

//如果总的困难比我们已知的要高，就把它加到规范链中去。
//if语句中的第二个子句减少了自私挖掘的脆弱性。
//请参阅http://www.cs.cornell.edu/~ie53/publications/btcrpocfc.pdf
	if externTd.Cmp(localTd) > 0 || (externTd.Cmp(localTd) == 0 && mrand.Float64() < 0.5) {
//删除新标题上方的所有规范编号分配
		batch := hc.chainDb.NewBatch()
		for i := number + 1; ; i++ {
			hash := rawdb.ReadCanonicalHash(hc.chainDb, i)
			if hash == (common.Hash{}) {
				break
			}
			rawdb.DeleteCanonicalHash(batch, i)
		}
		batch.Write()

//覆盖任何过时的规范编号分配
		var (
			headHash   = header.ParentHash
			headNumber = header.Number.Uint64() - 1
			headHeader = hc.GetHeader(headHash, headNumber)
		)
		for rawdb.ReadCanonicalHash(hc.chainDb, headNumber) != headHash {
			rawdb.WriteCanonicalHash(hc.chainDb, headHash, headNumber)

			headHash = headHeader.ParentHash
			headNumber = headHeader.Number.Uint64() - 1
			headHeader = hc.GetHeader(headHash, headNumber)
		}
//用新的头扩展规范链
		rawdb.WriteCanonicalHash(hc.chainDb, hash, number)
		rawdb.WriteHeadHeaderHash(hc.chainDb, hash)

		hc.currentHeaderHash = hash
		hc.currentHeader.Store(types.CopyHeader(header))

		status = CanonStatTy
	} else {
		status = SideStatTy
	}

	hc.headerCache.Add(hash, header)
	hc.numberCache.Add(hash, number)

	return
}

//whcallback是用于插入单个头的回调函数。
//回调有两个原因：第一，在LightChain中，状态应为
//处理并发送轻链事件，而在区块链中，这不是
//这是必需的，因为在插入块后发送链事件。第二，
//头写入应该由父链互斥体单独保护。
type WhCallback func(*types.Header) error

func (hc *HeaderChain) ValidateHeaderChain(chain []*types.Header, checkFreq int) (int, error) {
//做一个健全的检查，确保提供的链实际上是有序的和链接的
	for i := 1; i < len(chain); i++ {
		if chain[i].Number.Uint64() != chain[i-1].Number.Uint64()+1 || chain[i].ParentHash != chain[i-1].Hash() {
//断链祖先、记录消息（编程错误）和跳过插入
			log.Error("Non contiguous header insert", "number", chain[i].Number, "hash", chain[i].Hash(),
				"parent", chain[i].ParentHash, "prevnumber", chain[i-1].Number, "prevhash", chain[i-1].Hash())

			return 0, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, chain[i-1].Number,
				chain[i-1].Hash().Bytes()[:4], i, chain[i].Number, chain[i].Hash().Bytes()[:4], chain[i].ParentHash[:4])
		}
	}

//生成密封验证请求列表，并启动并行验证程序
	seals := make([]bool, len(chain))
	for i := 0; i < len(seals)/checkFreq; i++ {
		index := i*checkFreq + hc.rand.Intn(checkFreq)
		if index >= len(seals) {
			index = len(seals) - 1
		}
		seals[index] = true
	}
seals[len(seals)-1] = true //应始终验证最后一个以避免垃圾

	abort, results := hc.engine.VerifyHeaders(hc, chain, seals)
	defer close(abort)

//遍历头并确保它们都签出
	for i, header := range chain {
//如果链终止，则停止处理块
		if hc.procInterrupt() {
			log.Debug("Premature abort during headers verification")
			return 0, errors.New("aborted")
		}
//如果标题是禁止的，直接中止
		if BadHashes[header.Hash()] {
			return i, ErrBlacklistedHash
		}
//否则，等待头检查并确保它们通过
		if err := <-results; err != nil {
			return i, err
		}
	}

	return 0, nil
}

//insert header chain尝试将给定的头链插入本地
//链，可能创建REORG。如果返回错误，它将返回
//失败头的索引号以及描述出错原因的错误。
//
//verify参数可用于微调非ce验证
//是否应该做。可选检查背后的原因是
//其中的头检索机制已经需要验证nonce，以及
//因为nonce可以被稀疏地验证，不需要检查每一个。
func (hc *HeaderChain) InsertHeaderChain(chain []*types.Header, writeHeader WhCallback, start time.Time) (int, error) {
//收集一些要报告的进口统计数据
	stats := struct{ processed, ignored int }{}
//所有头都通过了验证，将它们导入数据库
	for i, header := range chain {
//关闭时短路插入
		if hc.procInterrupt() {
			log.Debug("Premature abort during headers import")
			return i, errors.New("aborted")
		}
//如果头已经知道，跳过它，否则存储
		if hc.HasHeader(header.Hash(), header.Number.Uint64()) {
			stats.ignored++
			continue
		}
		if err := writeHeader(header); err != nil {
			return i, err
		}
		stats.processed++
	}
//报告一些公共统计数据，这样用户就可以知道发生了什么。
	last := chain[len(chain)-1]

	context := []interface{}{
		"count", stats.processed, "elapsed", common.PrettyDuration(time.Since(start)),
		"number", last.Number, "hash", last.Hash(),
	}
	if timestamp := time.Unix(last.Time.Int64(), 0); time.Since(timestamp) > time.Minute {
		context = append(context, []interface{}{"age", common.PrettyAge(timestamp)}...)
	}
	if stats.ignored > 0 {
		context = append(context, []interface{}{"ignored", stats.ignored}...)
	}
	log.Info("Imported new block headers", context...)

	return 0, nil
}

//GetBlockHashesFromHash从给定的
//哈什，向创世纪街区走去。
func (hc *HeaderChain) GetBlockHashesFromHash(hash common.Hash, max uint64) []common.Hash {
//获取要从中获取的源标题
	header := hc.GetHeaderByHash(hash)
	if header == nil {
		return nil
	}
//重复这些头文件，直到收集到足够的文件或达到创世标准。
	chain := make([]common.Hash, 0, max)
	for i := uint64(0); i < max; i++ {
		next := header.ParentHash
		if header = hc.GetHeader(next, header.Number.Uint64()-1); header == nil {
			break
		}
		chain = append(chain, next)
		if header.Number.Sign() == 0 {
			break
		}
	}
	return chain
}

//getAncestor检索给定块的第n个祖先。它假定给定的块或
//它的近亲是典型的。maxnoncanonical指向向下计数器，限制
//到达规范链之前要单独检查的块数。
//
//注意：ancestor==0返回相同的块，1返回其父块，依此类推。
func (hc *HeaderChain) GetAncestor(hash common.Hash, number, ancestor uint64, maxNonCanonical *uint64) (common.Hash, uint64) {
	if ancestor > number {
		return common.Hash{}, 0
	}
	if ancestor == 1 {
//在这种情况下，只需读取标题就更便宜了
		if header := hc.GetHeader(hash, number); header != nil {
			return header.ParentHash, number - 1
		} else {
			return common.Hash{}, 0
		}
	}
	for ancestor != 0 {
		if rawdb.ReadCanonicalHash(hc.chainDb, number) == hash {
			number -= ancestor
			return rawdb.ReadCanonicalHash(hc.chainDb, number), number
		}
		if *maxNonCanonical == 0 {
			return common.Hash{}, 0
		}
		*maxNonCanonical--
		ancestor--
		header := hc.GetHeader(hash, number)
		if header == nil {
			return common.Hash{}, 0
		}
		hash = header.ParentHash
		number--
	}
	return hash, number
}

//gettd从
//按哈希和数字排列的数据库，如果找到，则将其缓存。
func (hc *HeaderChain) GetTd(hash common.Hash, number uint64) *big.Int {
//如果td已经在缓存中，则短路，否则检索
	if cached, ok := hc.tdCache.Get(hash); ok {
		return cached.(*big.Int)
	}
	td := rawdb.ReadTd(hc.chainDb, hash, number)
	if td == nil {
		return nil
	}
//缓存下一次找到的正文并返回
	hc.tdCache.Add(hash, td)
	return td
}

//getDByHash从
//通过哈希对数据库进行缓存（如果找到）。
func (hc *HeaderChain) GetTdByHash(hash common.Hash) *big.Int {
	number := hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return hc.GetTd(hash, *number)
}

//writetd将一个块的总难度存储到数据库中，并对其进行缓存
//一路走来。
func (hc *HeaderChain) WriteTd(hash common.Hash, number uint64, td *big.Int) error {
	rawdb.WriteTd(hc.chainDb, hash, number, td)
	hc.tdCache.Add(hash, new(big.Int).Set(td))
	return nil
}

//GetHeader按哈希和数字从数据库中检索块头，
//如果找到，则缓存它。
func (hc *HeaderChain) GetHeader(hash common.Hash, number uint64) *types.Header {
//如果头已经在缓存中，则短路，否则检索
	if header, ok := hc.headerCache.Get(hash); ok {
		return header.(*types.Header)
	}
	header := rawdb.ReadHeader(hc.chainDb, hash, number)
	if header == nil {
		return nil
	}
//下次缓存找到的头并返回
	hc.headerCache.Add(hash, header)
	return header
}

//GetHeaderByHash通过哈希从数据库中检索块头，如果
//找到了。
func (hc *HeaderChain) GetHeaderByHash(hash common.Hash) *types.Header {
	number := hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return hc.GetHeader(hash, *number)
}

//hasheader检查数据库中是否存在块头。
func (hc *HeaderChain) HasHeader(hash common.Hash, number uint64) bool {
	if hc.numberCache.Contains(hash) || hc.headerCache.Contains(hash) {
		return true
	}
	return rawdb.HasHeader(hc.chainDb, hash, number)
}

//GetHeaderByNumber按编号从数据库中检索块头，
//如果找到，则缓存它（与其哈希关联）。
func (hc *HeaderChain) GetHeaderByNumber(number uint64) *types.Header {
	hash := rawdb.ReadCanonicalHash(hc.chainDb, number)
	if hash == (common.Hash{}) {
		return nil
	}
	return hc.GetHeader(hash, number)
}

//当前头检索规范链的当前头。这个
//从HeaderChain的内部缓存中检索头。
func (hc *HeaderChain) CurrentHeader() *types.Header {
	return hc.currentHeader.Load().(*types.Header)
}

//setcurrentheader设置规范链的当前头。
func (hc *HeaderChain) SetCurrentHeader(head *types.Header) {
	rawdb.WriteHeadHeaderHash(hc.chainDb, head.Hash())

	hc.currentHeader.Store(head)
	hc.currentHeaderHash = head.Hash()
}

//deleteCallback是一个回调函数，由sethead在
//删除每个标题。
type DeleteCallback func(rawdb.DatabaseDeleter, common.Hash, uint64)

//sethead将本地链重绕到新的head。新脑袋上的一切
//将被删除和新的一组。
func (hc *HeaderChain) SetHead(head uint64, delFn DeleteCallback) {
	height := uint64(0)

	if hdr := hc.CurrentHeader(); hdr != nil {
		height = hdr.Number.Uint64()
	}
	batch := hc.chainDb.NewBatch()
	for hdr := hc.CurrentHeader(); hdr != nil && hdr.Number.Uint64() > head; hdr = hc.CurrentHeader() {
		hash := hdr.Hash()
		num := hdr.Number.Uint64()
		if delFn != nil {
			delFn(batch, hash, num)
		}
		rawdb.DeleteHeader(batch, hash, num)
		rawdb.DeleteTd(batch, hash, num)

		hc.currentHeader.Store(hc.GetHeader(hdr.ParentHash, hdr.Number.Uint64()-1))
	}
//回滚规范链编号
	for i := height; i > head; i-- {
		rawdb.DeleteCanonicalHash(batch, i)
	}
	batch.Write()

//从缓存中清除所有过时的内容
	hc.headerCache.Purge()
	hc.tdCache.Purge()
	hc.numberCache.Purge()

	if hc.CurrentHeader() == nil {
		hc.currentHeader.Store(hc.genesisHeader)
	}
	hc.currentHeaderHash = hc.CurrentHeader().Hash()

	rawdb.WriteHeadHeaderHash(hc.chainDb, hc.currentHeaderHash)
}

//setGenesis为链设置新的Genesis块头
func (hc *HeaderChain) SetGenesis(head *types.Header) {
	hc.genesisHeader = head
}

//config检索头链的链配置。
func (hc *HeaderChain) Config() *params.ChainConfig { return hc.config }

//引擎检索收割台链的共识引擎。
func (hc *HeaderChain) Engine() consensus.Engine { return hc.engine }

//getBlock实现consumeration.chainReader，并为每个输入返回nil作为
//标题链没有可供检索的块。
func (hc *HeaderChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	return nil
}
