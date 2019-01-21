
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

package fetcher

import (
	"errors"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
)

var (
	testdb       = ethdb.NewMemDatabase()
	testKey, _   = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	testAddress  = crypto.PubkeyToAddress(testKey.PublicKey)
	genesis      = core.GenesisBlockForTesting(testdb, testAddress, big.NewInt(1000000000))
	unknownBlock = types.NewBlock(&types.Header{GasLimit: params.GenesisGasLimit}, nil, nil, nil)
)

//makechain创建一个由n个块组成的链，从父块开始并包含父块。
//返回的哈希链是有序的head->parent。此外，每三个街区
//包含一个事务，每隔5分钟一个叔叔以允许测试正确的块
//重新组装。
func makeChain(n int, seed byte, parent *types.Block) ([]common.Hash, map[common.Hash]*types.Block) {
	blocks, _ := core.GenerateChain(params.TestChainConfig, parent, ethash.NewFaker(), testdb, n, func(i int, block *core.BlockGen) {
		block.SetCoinbase(common.Address{seed})

//如果区块号是3的倍数，则向矿工发送奖金交易。
		if parent == genesis && i%3 == 0 {
			signer := types.MakeSigner(params.TestChainConfig, block.Number())
			tx, err := types.SignTx(types.NewTransaction(block.TxNonce(testAddress), common.Address{seed}, big.NewInt(1000), params.TxGas, nil, nil), signer, testKey)
			if err != nil {
				panic(err)
			}
			block.AddTx(tx)
		}
//如果区块编号是5的倍数，请在区块中添加一个奖金叔叔。
		if i%5 == 0 {
			block.AddUncle(&types.Header{ParentHash: block.PrevBlock(i - 1).Hash(), Number: big.NewInt(int64(i - 1))})
		}
	})
	hashes := make([]common.Hash, n+1)
	hashes[len(hashes)-1] = parent.Hash()
	blockm := make(map[common.Hash]*types.Block, n+1)
	blockm[parent.Hash()] = parent
	for i, b := range blocks {
		hashes[len(hashes)-i-2] = b.Hash()
		blockm[b.Hash()] = b
	}
	return hashes, blockm
}

//fetcherTester is a test simulator for mocking out local block chain.
type fetcherTester struct {
	fetcher *Fetcher

hashes []common.Hash                //属于测试人员的哈希链
blocks map[common.Hash]*types.Block //属于测试仪的块
drops  map[string]bool              //取方丢弃的对等点图

	lock sync.RWMutex
}

//NewTester创建一个新的获取器测试mocker。
func newTester() *fetcherTester {
	tester := &fetcherTester{
		hashes: []common.Hash{genesis.Hash()},
		blocks: map[common.Hash]*types.Block{genesis.Hash(): genesis},
		drops:  make(map[string]bool),
	}
	tester.fetcher = New(tester.getBlock, tester.verifyHeader, tester.broadcastBlock, tester.chainHeight, tester.insertChain, tester.dropPeer)
	tester.fetcher.Start()

	return tester
}

//GetBlock从测试仪的区块链中检索一个区块。
func (f *fetcherTester) getBlock(hash common.Hash) *types.Block {
	f.lock.RLock()
	defer f.lock.RUnlock()

	return f.blocks[hash]
}

//verifyheader是块头验证的nop占位符。
func (f *fetcherTester) verifyHeader(header *types.Header) error {
	return nil
}

//BroadcastBlock是块广播的NOP占位符。
func (f *fetcherTester) broadcastBlock(block *types.Block, propagate bool) {
}

//chain height检索链的当前高度（块号）。
func (f *fetcherTester) chainHeight() uint64 {
	f.lock.RLock()
	defer f.lock.RUnlock()

	return f.blocks[f.hashes[len(f.hashes)-1]].NumberU64()
}

//insertchain向模拟链中注入新的块。
func (f *fetcherTester) insertChain(blocks types.Blocks) (int, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	for i, block := range blocks {
//确保中的父级
		if _, ok := f.blocks[block.ParentHash()]; !ok {
			return i, errors.New("unknown parent")
		}
//如果已经存在相同高度，则丢弃所有新块
		if block.NumberU64() <= f.blocks[f.hashes[len(f.hashes)-1]].NumberU64() {
			return i, nil
		}
//否则就建立我们现在的链条
		f.hashes = append(f.hashes, block.Hash())
		f.blocks[block.Hash()] = block
	}
	return 0, nil
}

//Droppeer是一个对等删除的仿真器，只需将
//同龄人被牵线人甩了。
func (f *fetcherTester) dropPeer(peer string) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.drops[peer] = true
}

//makeHeaderFetcher retrieves a block header fetcher associated with a simulated peer.
func (f *fetcherTester) makeHeaderFetcher(peer string, blocks map[common.Hash]*types.Block, drift time.Duration) headerRequesterFn {
	closure := make(map[common.Hash]*types.Block)
	for hash, block := range blocks {
		closure[hash] = block
	}
//创建一个从闭包返回头的函数
	return func(hash common.Hash) error {
//收集积木返回
		headers := make([]*types.Header, 0, 1)
		if block, ok := closure[hash]; ok {
			headers = append(headers, block.Header())
		}
//返回新线程
		go f.fetcher.FilterHeaders(peer, headers, time.Now().Add(drift))

		return nil
	}
}

//MakeBodyFetcher检索与模拟对等体关联的块体Fetcher。
func (f *fetcherTester) makeBodyFetcher(peer string, blocks map[common.Hash]*types.Block, drift time.Duration) bodyRequesterFn {
	closure := make(map[common.Hash]*types.Block)
	for hash, block := range blocks {
		closure[hash] = block
	}
//创建从闭包返回块的函数
	return func(hashes []common.Hash) error {
//收集块体返回
		transactions := make([][]*types.Transaction, 0, len(hashes))
		uncles := make([][]*types.Header, 0, len(hashes))

		for _, hash := range hashes {
			if block, ok := closure[hash]; ok {
				transactions = append(transactions, block.Transactions())
				uncles = append(uncles, block.Uncles())
			}
		}
//返回新线程
		go f.fetcher.FilterBodies(peer, transactions, uncles, time.Now().Add(drift))

		return nil
	}
}

//VerifyFetchingEvent验证单个事件是否到达提取通道。
func verifyFetchingEvent(t *testing.T, fetching chan []common.Hash, arrive bool) {
	if arrive {
		select {
		case <-fetching:
		case <-time.After(time.Second):
			t.Fatalf("fetching timeout")
		}
	} else {
		select {
		case <-fetching:
			t.Fatalf("fetching invoked")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

//VerifyCompletingEvent验证单个事件是否到达完成通道。
func verifyCompletingEvent(t *testing.T, completing chan []common.Hash, arrive bool) {
	if arrive {
		select {
		case <-completing:
		case <-time.After(time.Second):
			t.Fatalf("completing timeout")
		}
	} else {
		select {
		case <-completing:
			t.Fatalf("completing invoked")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

//verifyimportevent验证一个事件是否到达导入通道。
func verifyImportEvent(t *testing.T, imported chan *types.Block, arrive bool) {
	if arrive {
		select {
		case <-imported:
		case <-time.After(time.Second):
			t.Fatalf("import timeout")
		}
	} else {
		select {
		case <-imported:
			t.Fatalf("import invoked")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

//verifyimportcount验证在
//导入挂钩通道。
func verifyImportCount(t *testing.T, imported chan *types.Block, count int) {
	for i := 0; i < count; i++ {
		select {
		case <-imported:
		case <-time.After(time.Second):
			t.Fatalf("block %d: import timeout", i+1)
		}
	}
	verifyImportDone(t, imported)
}

//verifyimportdone验证导入通道上没有其他事件到达。
func verifyImportDone(t *testing.T, imported chan *types.Block) {
	select {
	case <-imported:
		t.Fatalf("extra block imported")
	case <-time.After(50 * time.Millisecond):
	}
}

//测试获取程序接受块通知并为
//成功导入到本地链。
func TestSequentialAnnouncements62(t *testing.T) { testSequentialAnnouncements(t, 62) }
func TestSequentialAnnouncements63(t *testing.T) { testSequentialAnnouncements(t, 63) }
func TestSequentialAnnouncements64(t *testing.T) { testSequentialAnnouncements(t, 64) }

func testSequentialAnnouncements(t *testing.T, protocol int) {
//创建要导入的块链
	targetBlocks := 4 * hashLimit
	hashes, blocks := makeChain(targetBlocks, 0, genesis)

	tester := newTester()
	headerFetcher := tester.makeHeaderFetcher("valid", blocks, -gatherSlack)
	bodyFetcher := tester.makeBodyFetcher("valid", blocks, 0)

//迭代地通知块，直到全部导入
	imported := make(chan *types.Block)
	tester.fetcher.importedHook = func(block *types.Block) { imported <- block }

	for i := len(hashes) - 2; i >= 0; i-- {
		tester.fetcher.Notify("valid", hashes[i], uint64(len(hashes)-i-1), time.Now().Add(-arriveTimeout), headerFetcher, bodyFetcher)
		verifyImportEvent(t, imported, true)
	}
	verifyImportDone(t, imported)
}

//测试多个对等方（甚至同一个bug）是否宣布块
//它们最多只能下载一次。
func TestConcurrentAnnouncements62(t *testing.T) { testConcurrentAnnouncements(t, 62) }
func TestConcurrentAnnouncements63(t *testing.T) { testConcurrentAnnouncements(t, 63) }
func TestConcurrentAnnouncements64(t *testing.T) { testConcurrentAnnouncements(t, 64) }

func testConcurrentAnnouncements(t *testing.T, protocol int) {
//创建要导入的块链
	targetBlocks := 4 * hashLimit
	hashes, blocks := makeChain(targetBlocks, 0, genesis)

//为请求装配带有内置计数器的测试仪
	tester := newTester()
	firstHeaderFetcher := tester.makeHeaderFetcher("first", blocks, -gatherSlack)
	firstBodyFetcher := tester.makeBodyFetcher("first", blocks, 0)
	secondHeaderFetcher := tester.makeHeaderFetcher("second", blocks, -gatherSlack)
	secondBodyFetcher := tester.makeBodyFetcher("second", blocks, 0)

	counter := uint32(0)
	firstHeaderWrapper := func(hash common.Hash) error {
		atomic.AddUint32(&counter, 1)
		return firstHeaderFetcher(hash)
	}
	secondHeaderWrapper := func(hash common.Hash) error {
		atomic.AddUint32(&counter, 1)
		return secondHeaderFetcher(hash)
	}
//迭代地通知块，直到全部导入
	imported := make(chan *types.Block)
	tester.fetcher.importedHook = func(block *types.Block) { imported <- block }

	for i := len(hashes) - 2; i >= 0; i-- {
		tester.fetcher.Notify("first", hashes[i], uint64(len(hashes)-i-1), time.Now().Add(-arriveTimeout), firstHeaderWrapper, firstBodyFetcher)
		tester.fetcher.Notify("second", hashes[i], uint64(len(hashes)-i-1), time.Now().Add(-arriveTimeout+time.Millisecond), secondHeaderWrapper, secondBodyFetcher)
		tester.fetcher.Notify("second", hashes[i], uint64(len(hashes)-i-1), time.Now().Add(-arriveTimeout-time.Millisecond), secondHeaderWrapper, secondBodyFetcher)
		verifyImportEvent(t, imported, true)
	}
	verifyImportDone(t, imported)

//确保两次未检索到任何块
	if int(counter) != targetBlocks {
		t.Fatalf("retrieval count mismatch: have %v, want %v", counter, targetBlocks)
	}
}

//在提取前一个通知时通知到达的测试
//导致有效导入。
func TestOverlappingAnnouncements62(t *testing.T) { testOverlappingAnnouncements(t, 62) }
func TestOverlappingAnnouncements63(t *testing.T) { testOverlappingAnnouncements(t, 63) }
func TestOverlappingAnnouncements64(t *testing.T) { testOverlappingAnnouncements(t, 64) }

func testOverlappingAnnouncements(t *testing.T, protocol int) {
//创建要导入的块链
	targetBlocks := 4 * hashLimit
	hashes, blocks := makeChain(targetBlocks, 0, genesis)

	tester := newTester()
	headerFetcher := tester.makeHeaderFetcher("valid", blocks, -gatherSlack)
	bodyFetcher := tester.makeBodyFetcher("valid", blocks, 0)

//迭代地通知块，但连续地重叠它们
	overlap := 16
	imported := make(chan *types.Block, len(hashes)-1)
	for i := 0; i < overlap; i++ {
		imported <- nil
	}
	tester.fetcher.importedHook = func(block *types.Block) { imported <- block }

	for i := len(hashes) - 2; i >= 0; i-- {
		tester.fetcher.Notify("valid", hashes[i], uint64(len(hashes)-i-1), time.Now().Add(-arriveTimeout), headerFetcher, bodyFetcher)
		select {
		case <-imported:
		case <-time.After(time.Second):
			t.Fatalf("block %d: import timeout", len(hashes)-i)
		}
	}
//等待所有导入完成并检查计数
	verifyImportCount(t, imported, overlap)
}

//宣布已检索的测试将不会重复。
func TestPendingDeduplication62(t *testing.T) { testPendingDeduplication(t, 62) }
func TestPendingDeduplication63(t *testing.T) { testPendingDeduplication(t, 63) }
func TestPendingDeduplication64(t *testing.T) { testPendingDeduplication(t, 64) }

func testPendingDeduplication(t *testing.T, protocol int) {
//创建哈希和相应的块
	hashes, blocks := makeChain(1, 0, genesis)

//用内置计数器和延迟回卷器组装测试仪
	tester := newTester()
	headerFetcher := tester.makeHeaderFetcher("repeater", blocks, -gatherSlack)
	bodyFetcher := tester.makeBodyFetcher("repeater", blocks, 0)

	delay := 50 * time.Millisecond
	counter := uint32(0)
	headerWrapper := func(hash common.Hash) error {
		atomic.AddUint32(&counter, 1)

//模拟长时间运行的获取
		go func() {
			time.Sleep(delay)
			headerFetcher(hash)
		}()
		return nil
	}
//多次通知同一个块，直到它被提取（等待任何挂起的操作）
	for tester.getBlock(hashes[0]) == nil {
		tester.fetcher.Notify("repeater", hashes[0], 1, time.Now().Add(-arriveTimeout), headerWrapper, bodyFetcher)
		time.Sleep(time.Millisecond)
	}
	time.Sleep(delay)

//Check that all blocks were imported and none fetched twice
	if imported := len(tester.blocks); imported != 2 {
		t.Fatalf("synchronised block mismatch: have %v, want %v", imported, 2)
	}
	if int(counter) != 1 {
		t.Fatalf("retrieval count mismatch: have %v, want %v", counter, 1)
	}
}

//以随机顺序检索公告的测试将被缓存，并最终
//填充所有间隙时导入。
func TestRandomArrivalImport62(t *testing.T) { testRandomArrivalImport(t, 62) }
func TestRandomArrivalImport63(t *testing.T) { testRandomArrivalImport(t, 63) }
func TestRandomArrivalImport64(t *testing.T) { testRandomArrivalImport(t, 64) }

func testRandomArrivalImport(t *testing.T, protocol int) {
//创建要导入的块链，然后选择一个要延迟的块链
	targetBlocks := maxQueueDist
	hashes, blocks := makeChain(targetBlocks, 0, genesis)
	skip := targetBlocks / 2

	tester := newTester()
	headerFetcher := tester.makeHeaderFetcher("valid", blocks, -gatherSlack)
	bodyFetcher := tester.makeBodyFetcher("valid", blocks, 0)

//迭代地通知块，跳过一个条目
	imported := make(chan *types.Block, len(hashes)-1)
	tester.fetcher.importedHook = func(block *types.Block) { imported <- block }

	for i := len(hashes) - 1; i >= 0; i-- {
		if i != skip {
			tester.fetcher.Notify("valid", hashes[i], uint64(len(hashes)-i-1), time.Now().Add(-arriveTimeout), headerFetcher, bodyFetcher)
			time.Sleep(time.Millisecond)
		}
	}
//最后宣布跳过的条目并检查完全导入
	tester.fetcher.Notify("valid", hashes[skip], uint64(len(hashes)-skip-1), time.Now().Add(-arriveTimeout), headerFetcher, bodyFetcher)
	verifyImportCount(t, imported, len(hashes)-1)
}

//引导块排队的测试（由于块传播与哈希公告）
//正确安排、填充和导入队列间隙。
func TestQueueGapFill62(t *testing.T) { testQueueGapFill(t, 62) }
func TestQueueGapFill63(t *testing.T) { testQueueGapFill(t, 63) }
func TestQueueGapFill64(t *testing.T) { testQueueGapFill(t, 64) }

func testQueueGapFill(t *testing.T, protocol int) {
//创建一个要导入的块链，并选择一个完全不公告的块链
	targetBlocks := maxQueueDist
	hashes, blocks := makeChain(targetBlocks, 0, genesis)
	skip := targetBlocks / 2

	tester := newTester()
	headerFetcher := tester.makeHeaderFetcher("valid", blocks, -gatherSlack)
	bodyFetcher := tester.makeBodyFetcher("valid", blocks, 0)

//迭代地通知块，跳过一个条目
	imported := make(chan *types.Block, len(hashes)-1)
	tester.fetcher.importedHook = func(block *types.Block) { imported <- block }

	for i := len(hashes) - 1; i >= 0; i-- {
		if i != skip {
			tester.fetcher.Notify("valid", hashes[i], uint64(len(hashes)-i-1), time.Now().Add(-arriveTimeout), headerFetcher, bodyFetcher)
			time.Sleep(time.Millisecond)
		}
	}
//直接填充缺失的块，就像传播一样
	tester.fetcher.Enqueue("valid", blocks[hashes[skip]])
	verifyImportCount(t, imported, len(hashes)-1)
}

//阻止来自不同源（多个传播、哈希）的测试
//不安排多次导入。
func TestImportDeduplication62(t *testing.T) { testImportDeduplication(t, 62) }
func TestImportDeduplication63(t *testing.T) { testImportDeduplication(t, 63) }
func TestImportDeduplication64(t *testing.T) { testImportDeduplication(t, 64) }

func testImportDeduplication(t *testing.T, protocol int) {
//创建两个要导入的块（一个用于复制，另一个用于暂停）
	hashes, blocks := makeChain(2, 0, genesis)

//创建检测仪并用计数器包装进口商
	tester := newTester()
	headerFetcher := tester.makeHeaderFetcher("valid", blocks, -gatherSlack)
	bodyFetcher := tester.makeBodyFetcher("valid", blocks, 0)

	counter := uint32(0)
	tester.fetcher.insertChain = func(blocks types.Blocks) (int, error) {
		atomic.AddUint32(&counter, uint32(len(blocks)))
		return tester.insertChain(blocks)
	}
//检测获取和导入的事件
	fetching := make(chan []common.Hash)
	imported := make(chan *types.Block, len(hashes)-1)
	tester.fetcher.fetchingHook = func(hashes []common.Hash) { fetching <- hashes }
	tester.fetcher.importedHook = func(block *types.Block) { imported <- block }

//Announce the duplicating block, wait for retrieval, and also propagate directly
	tester.fetcher.Notify("valid", hashes[0], 1, time.Now().Add(-arriveTimeout), headerFetcher, bodyFetcher)
	<-fetching

	tester.fetcher.Enqueue("valid", blocks[hashes[0]])
	tester.fetcher.Enqueue("valid", blocks[hashes[0]])
	tester.fetcher.Enqueue("valid", blocks[hashes[0]])

//像传播一样直接填充缺失的块，并检查导入的唯一性
	tester.fetcher.Enqueue("valid", blocks[hashes[1]])
	verifyImportCount(t, imported, 2)

	if counter != 2 {
		t.Fatalf("import invocation count mismatch: have %v, want %v", counter, 2)
	}
}

//用远低于或高于当前磁头的数字阻塞的测试得到
//丢弃以防止将资源浪费在来自故障对等机的无用块上。
func TestDistantPropagationDiscarding(t *testing.T) {
//创建一个长链以导入和定义丢弃边界
	hashes, blocks := makeChain(3*maxQueueDist, 0, genesis)
	head := hashes[len(hashes)/2]

	low, high := len(hashes)/2+maxUncleDist+1, len(hashes)/2-maxQueueDist-1

//创建一个测试仪并模拟上边链中间的头块
	tester := newTester()

	tester.lock.Lock()
	tester.hashes = []common.Hash{head}
	tester.blocks = map[common.Hash]*types.Block{head: blocks[head]}
	tester.lock.Unlock()

//Ensure that a block with a lower number than the threshold is discarded
	tester.fetcher.Enqueue("lower", blocks[hashes[low]])
	time.Sleep(10 * time.Millisecond)
	if !tester.fetcher.queue.Empty() {
		t.Fatalf("fetcher queued stale block")
	}
//确保丢弃数字高于阈值的块
	tester.fetcher.Enqueue("higher", blocks[hashes[high]])
	time.Sleep(10 * time.Millisecond)
	if !tester.fetcher.queue.Empty() {
		t.Fatalf("fetcher queued future block")
	}
}

//数字远低于或高于输出电流的测试
//头被丢弃，以防止浪费资源在无用的块故障
//同龄人。
func TestDistantAnnouncementDiscarding62(t *testing.T) { testDistantAnnouncementDiscarding(t, 62) }
func TestDistantAnnouncementDiscarding63(t *testing.T) { testDistantAnnouncementDiscarding(t, 63) }
func TestDistantAnnouncementDiscarding64(t *testing.T) { testDistantAnnouncementDiscarding(t, 64) }

func testDistantAnnouncementDiscarding(t *testing.T, protocol int) {
//创建一个长链以导入和定义丢弃边界
	hashes, blocks := makeChain(3*maxQueueDist, 0, genesis)
	head := hashes[len(hashes)/2]

	low, high := len(hashes)/2+maxUncleDist+1, len(hashes)/2-maxQueueDist-1

//创建一个测试仪并模拟上边链中间的头块
	tester := newTester()

	tester.lock.Lock()
	tester.hashes = []common.Hash{head}
	tester.blocks = map[common.Hash]*types.Block{head: blocks[head]}
	tester.lock.Unlock()

	headerFetcher := tester.makeHeaderFetcher("lower", blocks, -gatherSlack)
	bodyFetcher := tester.makeBodyFetcher("lower", blocks, 0)

	fetching := make(chan struct{}, 2)
	tester.fetcher.fetchingHook = func(hashes []common.Hash) { fetching <- struct{}{} }

//确保丢弃数字低于阈值的块
	tester.fetcher.Notify("lower", hashes[low], blocks[hashes[low]].NumberU64(), time.Now().Add(-arriveTimeout), headerFetcher, bodyFetcher)
	select {
	case <-time.After(50 * time.Millisecond):
	case <-fetching:
		t.Fatalf("fetcher requested stale header")
	}
//确保丢弃数字高于阈值的块
	tester.fetcher.Notify("higher", hashes[high], blocks[hashes[high]].NumberU64(), time.Now().Add(-arriveTimeout), headerFetcher, bodyFetcher)
	select {
	case <-time.After(50 * time.Millisecond):
	case <-fetching:
		t.Fatalf("fetcher requested future header")
	}
}

//对等方用无效数字（即不匹配）宣布块的测试
//随后提供的头）作为恶意丢弃。
func TestInvalidNumberAnnouncement62(t *testing.T) { testInvalidNumberAnnouncement(t, 62) }
func TestInvalidNumberAnnouncement63(t *testing.T) { testInvalidNumberAnnouncement(t, 63) }
func TestInvalidNumberAnnouncement64(t *testing.T) { testInvalidNumberAnnouncement(t, 64) }

func testInvalidNumberAnnouncement(t *testing.T, protocol int) {
//创建一个块以导入和检查编号
	hashes, blocks := makeChain(1, 0, genesis)

	tester := newTester()
	badHeaderFetcher := tester.makeHeaderFetcher("bad", blocks, -gatherSlack)
	badBodyFetcher := tester.makeBodyFetcher("bad", blocks, 0)

	imported := make(chan *types.Block)
	tester.fetcher.importedHook = func(block *types.Block) { imported <- block }

//宣布一个有错误数字的块，检查是否立即丢弃
	tester.fetcher.Notify("bad", hashes[0], 2, time.Now().Add(-arriveTimeout), badHeaderFetcher, badBodyFetcher)
	verifyImportEvent(t, imported, false)

	tester.lock.RLock()
	dropped := tester.drops["bad"]
	tester.lock.RUnlock()

	if !dropped {
		t.Fatalf("peer with invalid numbered announcement not dropped")
	}

	goodHeaderFetcher := tester.makeHeaderFetcher("good", blocks, -gatherSlack)
	goodBodyFetcher := tester.makeBodyFetcher("good", blocks, 0)
//确保一个好的通知一滴一滴地通过
	tester.fetcher.Notify("good", hashes[0], 1, time.Now().Add(-arriveTimeout), goodHeaderFetcher, goodBodyFetcher)
	verifyImportEvent(t, imported, true)

	tester.lock.RLock()
	dropped = tester.drops["good"]
	tester.lock.RUnlock()

	if dropped {
		t.Fatalf("peer with valid numbered announcement dropped")
	}
	verifyImportDone(t, imported)
}

//测试如果一个块为空（即仅限头），则不应该有body请求
//制作，而不是集管本身组装成一个整体。
func TestEmptyBlockShortCircuit62(t *testing.T) { testEmptyBlockShortCircuit(t, 62) }
func TestEmptyBlockShortCircuit63(t *testing.T) { testEmptyBlockShortCircuit(t, 63) }
func TestEmptyBlockShortCircuit64(t *testing.T) { testEmptyBlockShortCircuit(t, 64) }

func testEmptyBlockShortCircuit(t *testing.T, protocol int) {
//创建要导入的块链
	hashes, blocks := makeChain(32, 0, genesis)

	tester := newTester()
	headerFetcher := tester.makeHeaderFetcher("valid", blocks, -gatherSlack)
	bodyFetcher := tester.makeBodyFetcher("valid", blocks, 0)

//为所有内部事件添加监视挂钩
	fetching := make(chan []common.Hash)
	tester.fetcher.fetchingHook = func(hashes []common.Hash) { fetching <- hashes }

	completing := make(chan []common.Hash)
	tester.fetcher.completingHook = func(hashes []common.Hash) { completing <- hashes }

	imported := make(chan *types.Block)
	tester.fetcher.importedHook = func(block *types.Block) { imported <- block }

//迭代地通知块，直到全部导入
	for i := len(hashes) - 2; i >= 0; i-- {
		tester.fetcher.Notify("valid", hashes[i], uint64(len(hashes)-i-1), time.Now().Add(-arriveTimeout), headerFetcher, bodyFetcher)

//所有公告都应获取标题
		verifyFetchingEvent(t, fetching, true)

//Only blocks with data contents should request bodies
		verifyCompletingEvent(t, completing, len(blocks[hashes[i]].Transactions()) > 0 || len(blocks[hashes[i]].Uncles()) > 0)

//与构造无关，导入应该成功
		verifyImportEvent(t, imported, true)
	}
	verifyImportDone(t, imported)
}

//测试对等端无法使用无限发送的无限内存
//阻止向节点发布公告，但即使面对此类攻击，
//取纸器仍在工作。
func TestHashMemoryExhaustionAttack62(t *testing.T) { testHashMemoryExhaustionAttack(t, 62) }
func TestHashMemoryExhaustionAttack63(t *testing.T) { testHashMemoryExhaustionAttack(t, 63) }
func TestHashMemoryExhaustionAttack64(t *testing.T) { testHashMemoryExhaustionAttack(t, 64) }

func testHashMemoryExhaustionAttack(t *testing.T, protocol int) {
//使用插入仪器的导入挂钩创建测试仪
	tester := newTester()

	imported, announces := make(chan *types.Block), int32(0)
	tester.fetcher.importedHook = func(block *types.Block) { imported <- block }
	tester.fetcher.announceChangeHook = func(hash common.Hash, added bool) {
		if added {
			atomic.AddInt32(&announces, 1)
		} else {
			atomic.AddInt32(&announces, -1)
		}
	}
//创建有效的链和无限的垃圾链
	targetBlocks := hashLimit + 2*maxQueueDist
	hashes, blocks := makeChain(targetBlocks, 0, genesis)
	validHeaderFetcher := tester.makeHeaderFetcher("valid", blocks, -gatherSlack)
	validBodyFetcher := tester.makeBodyFetcher("valid", blocks, 0)

	attack, _ := makeChain(targetBlocks, 0, unknownBlock)
	attackerHeaderFetcher := tester.makeHeaderFetcher("attacker", nil, -gatherSlack)
	attackerBodyFetcher := tester.makeBodyFetcher("attacker", nil, 0)

//向测试人员提供来自攻击者的大量哈希集，以及来自有效对等端的有限哈希集
	for i := 0; i < len(attack); i++ {
		if i < maxQueueDist {
			tester.fetcher.Notify("valid", hashes[len(hashes)-2-i], uint64(i+1), time.Now(), validHeaderFetcher, validBodyFetcher)
		}
  /*ter.fetcher.notify（“攻击者”，攻击[i]，1/*不要距离下降*/，time.now（），攻击者headerfetcher，攻击者bodyfetcher）
 }
 if count := atomic.LoadInt32(&announces); count != hashLimit+maxQueueDist {
  t.fatalf（“排队的公告计数不匹配：有%d，需要%d”，count，hashlimit+maxqueuedist）
 }
 //等待回迁完成
 verifyimportcount（t，导入，maxqueuedist）

 //馈送剩余的有效哈希以确保DOS保护状态保持干净
 对于i：=len（hashes）-maxqueuedist-2；i>=0；i--
  tester.fetcher.notify（“valid”，hashes[i]，uint64（len（hashes）-i-1），time.now（）.add（-arrieveTimeout），validHeaderFetcher，validBodyFetcher）
  verifyimportevent（t，导入，真）
 }
 验证导入完成（T，导入）
}

//测试发送到获取程序的块（通过传播或通过哈希
//announces和retrievals）不要无限期地堆积起来，这样会耗尽可用的资源。
//系统内存。
func testblockmemoryxhausationattack（t*testing.t）
 //使用插入指令的导入挂钩创建测试仪
 测试仪：=新测试仪（）

 导入，排队：=make（chan*types.block），int32（0）
 tester.fetcher.importedhook=func（block*types.block）imported<-block
 tester.fetcher.queuechangehook=func（hash common.hash，added bool）
  如果添加{
   Atomic.AddInt32（&Enqueued，1）
  }否则{
   原子.addint32（&enqueued，-1）
  }
 }
 //创建有效的链和一批悬空（但在范围内）块
 目标块：=hashlimit+2*maxqueuedist
 哈希，块：=makechain（targetBlocks，0，genesis）
 攻击：=make（map[common.hash]*types.block）
 对于i：=byte（0）；len（attack）<blocklimit+2*maxqueuedist；i++
  哈希，块：=makechain（maxqueuedist-1，i，unknownblock）
  对于u，哈希：=范围哈希[：maxQueueDist-2]
   攻击[hash]=块[hash]
  }
 }
 //尝试馈送所有攻击者块，确保只接受有限的批处理
 对于uu，封锁：=距离攻击
  tester.fetcher.enqueue（“攻击者”，block）
 }
 时间.睡眠（200*时间.毫秒）
 如果排队：=atomic.loadint32（&enqueued）；排队！=块限制{
  t.fatalf（“排队的块计数不匹配：有%d个，需要%d个”，排队，块限制）
 }
 //将一批有效的块排队，并检查是否允许新的对等机这样做
 对于i：=0；i<maxqueuedist-1；i++
  tester.fetcher.enqueue（“有效”，blocks[hashes[len（hashes）-3-i]）
 }
 时间.睡眠（100*时间.毫秒）
 如果排队：=atomic.loadint32（&enqueued）；排队！=blocklimit+maxqueuedist-1_
  t.fatalf（“排队的块计数不匹配：有%d，需要%d”，排队，blocklimit+maxqueuedist-1）
 }
 /插入丢失的片段（和健全性检查导入）
 tester.fetcher.enqueue（“有效”，blocks[hashes[len（hashes）-2]）
 verifyimportcount（t，导入，maxqueuedist）

 //将剩余的块分块插入以确保干净的DOS保护
 对于i：=maxqueuedist；i<len（hashes）-1；i++
  tester.fetcher.enqueue（“有效”，blocks[hashes[len（hashes）-2-i]）
  verifyimportevent（t，导入，真）
 }
 验证导入完成（T，导入）
}
