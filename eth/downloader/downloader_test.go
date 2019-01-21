
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

package downloader

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/trie"
)

//减少一些参数以使测试仪更快。
func init() {
	MaxForkAncestry = uint64(10000)
	blockCacheItems = 1024
	fsHeaderContCheck = 500 * time.Millisecond
}

//下载测试仪是模拟本地区块链的测试模拟器。
type downloadTester struct {
	downloader *Downloader

genesis *types.Block   //测试人员和同行使用的Genesis块
stateDb ethdb.Database //测试人员用于从对等机同步的数据库
peerDb  ethdb.Database //包含所有数据的对等数据库
	peers   map[string]*downloadTesterPeer

ownHashes   []common.Hash                  //属于测试人员的哈希链
ownHeaders  map[common.Hash]*types.Header  //属于检测仪的收割台
ownBlocks   map[common.Hash]*types.Block   //属于测试仪的块
ownReceipts map[common.Hash]types.Receipts //属于测试人员的收据
ownChainTd  map[common.Hash]*big.Int       //本地链中块的总困难

	lock sync.RWMutex
}

//NewTester创建了一个新的下载程序测试mocker。
func newTester() *downloadTester {
	tester := &downloadTester{
		genesis:     testGenesis,
		peerDb:      testDB,
		peers:       make(map[string]*downloadTesterPeer),
		ownHashes:   []common.Hash{testGenesis.Hash()},
		ownHeaders:  map[common.Hash]*types.Header{testGenesis.Hash(): testGenesis.Header()},
		ownBlocks:   map[common.Hash]*types.Block{testGenesis.Hash(): testGenesis},
		ownReceipts: map[common.Hash]types.Receipts{testGenesis.Hash(): nil},
		ownChainTd:  map[common.Hash]*big.Int{testGenesis.Hash(): testGenesis.Difficulty()},
	}
	tester.stateDb = ethdb.NewMemDatabase()
	tester.stateDb.Put(testGenesis.Root().Bytes(), []byte{0x00})
	tester.downloader = New(FullSync, tester.stateDb, new(event.TypeMux), tester, nil, tester.dropPeer)
	return tester
}

//终止中止嵌入式下载程序上的任何操作并释放所有
//持有资源。
func (dl *downloadTester) terminate() {
	dl.downloader.Terminate()
}

//同步开始与远程对等机同步，直到同步完成。
func (dl *downloadTester) sync(id string, td *big.Int, mode SyncMode) error {
	dl.lock.RLock()
	hash := dl.peers[id].chain.headBlock().Hash()
//如果没有请求特定的TD，则从对等区块链加载
	if td == nil {
		td = dl.peers[id].chain.td(hash)
	}
	dl.lock.RUnlock()

//与选定的对等机同步，并确保随后正确清理
	err := dl.downloader.synchronise(id, hash, td, mode)
	select {
	case <-dl.downloader.cancelCh:
//好的，下载程序在同步循环后完全取消
	default:
//下载程序仍在接受数据包，可能会阻止对等机
panic("downloader active post sync cycle") //测试人员会发现恐慌
	}
	return err
}

//hasheader检查测试仪规范链中是否存在头。
func (dl *downloadTester) HasHeader(hash common.Hash, number uint64) bool {
	return dl.GetHeaderByHash(hash) != nil
}

//hasblock检查测试仪规范链中是否存在块。
func (dl *downloadTester) HasBlock(hash common.Hash, number uint64) bool {
	return dl.GetBlockByHash(hash) != nil
}

//HasFastBlock检查测试仪规范链中是否存在块。
func (dl *downloadTester) HasFastBlock(hash common.Hash, number uint64) bool {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	_, ok := dl.ownReceipts[hash]
	return ok
}

//getheader从测试人员规范链中检索一个头。
func (dl *downloadTester) GetHeaderByHash(hash common.Hash) *types.Header {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	return dl.ownHeaders[hash]
}

//GetBlock从测试程序规范链中检索一个块。
func (dl *downloadTester) GetBlockByHash(hash common.Hash) *types.Block {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	return dl.ownBlocks[hash]
}

//当前头从规范链中检索当前头。
func (dl *downloadTester) CurrentHeader() *types.Header {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	for i := len(dl.ownHashes) - 1; i >= 0; i-- {
		if header := dl.ownHeaders[dl.ownHashes[i]]; header != nil {
			return header
		}
	}
	return dl.genesis.Header()
}

//currentBlock从规范链中检索当前头块。
func (dl *downloadTester) CurrentBlock() *types.Block {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	for i := len(dl.ownHashes) - 1; i >= 0; i-- {
		if block := dl.ownBlocks[dl.ownHashes[i]]; block != nil {
			if _, err := dl.stateDb.Get(block.Root().Bytes()); err == nil {
				return block
			}
		}
	}
	return dl.genesis
}

//CurrentFastBlock从规范链中检索当前磁头快速同步块。
func (dl *downloadTester) CurrentFastBlock() *types.Block {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	for i := len(dl.ownHashes) - 1; i >= 0; i-- {
		if block := dl.ownBlocks[dl.ownHashes[i]]; block != nil {
			return block
		}
	}
	return dl.genesis
}

//fastsynccommithead手动将头块设置为给定哈希。
func (dl *downloadTester) FastSyncCommitHead(hash common.Hash) error {
//现在只检查状态trie是否正确。
	if block := dl.GetBlockByHash(hash); block != nil {
		_, err := trie.NewSecure(block.Root(), trie.NewDatabase(dl.stateDb), 0)
		return err
	}
	return fmt.Errorf("non existent block: %x", hash[:4])
}

//gettd从规范链中检索块的总难度。
func (dl *downloadTester) GetTd(hash common.Hash, number uint64) *big.Int {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	return dl.ownChainTd[hash]
}

//InsertHeaderChain将新的一批头注入到模拟链中。
func (dl *downloadTester) InsertHeaderChain(headers []*types.Header, checkFreq int) (i int, err error) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

//快速检查，因为区块链。插入头链不会插入任何内容以防出错。
	if _, ok := dl.ownHeaders[headers[0].ParentHash]; !ok {
		return 0, errors.New("unknown parent")
	}
	for i := 1; i < len(headers); i++ {
		if headers[i].ParentHash != headers[i-1].Hash() {
			return i, errors.New("unknown parent")
		}
	}
//如果预检查通过，则执行完全插入
	for i, header := range headers {
		if _, ok := dl.ownHeaders[header.Hash()]; ok {
			continue
		}
		if _, ok := dl.ownHeaders[header.ParentHash]; !ok {
			return i, errors.New("unknown parent")
		}
		dl.ownHashes = append(dl.ownHashes, header.Hash())
		dl.ownHeaders[header.Hash()] = header
		dl.ownChainTd[header.Hash()] = new(big.Int).Add(dl.ownChainTd[header.ParentHash], header.Difficulty)
	}
	return len(headers), nil
}

//insertchain向模拟链中注入一批新的块。
func (dl *downloadTester) InsertChain(blocks types.Blocks) (i int, err error) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	for i, block := range blocks {
		if parent, ok := dl.ownBlocks[block.ParentHash()]; !ok {
			return i, errors.New("unknown parent")
		} else if _, err := dl.stateDb.Get(parent.Root().Bytes()); err != nil {
			return i, fmt.Errorf("unknown parent state %x: %v", parent.Root(), err)
		}
		if _, ok := dl.ownHeaders[block.Hash()]; !ok {
			dl.ownHashes = append(dl.ownHashes, block.Hash())
			dl.ownHeaders[block.Hash()] = block.Header()
		}
		dl.ownBlocks[block.Hash()] = block
		dl.ownReceipts[block.Hash()] = make(types.Receipts, 0)
		dl.stateDb.Put(block.Root().Bytes(), []byte{0x00})
		dl.ownChainTd[block.Hash()] = new(big.Int).Add(dl.ownChainTd[block.ParentHash()], block.Difficulty())
	}
	return len(blocks), nil
}

//InsertReceiptChain将新的一批收据注入到模拟链中。
func (dl *downloadTester) InsertReceiptChain(blocks types.Blocks, receipts []types.Receipts) (i int, err error) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	for i := 0; i < len(blocks) && i < len(receipts); i++ {
		if _, ok := dl.ownHeaders[blocks[i].Hash()]; !ok {
			return i, errors.New("unknown owner")
		}
		if _, ok := dl.ownBlocks[blocks[i].ParentHash()]; !ok {
			return i, errors.New("unknown parent")
		}
		dl.ownBlocks[blocks[i].Hash()] = blocks[i]
		dl.ownReceipts[blocks[i].Hash()] = receipts[i]
	}
	return len(blocks), nil
}

//回滚从链中删除一些最近添加的元素。
func (dl *downloadTester) Rollback(hashes []common.Hash) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	for i := len(hashes) - 1; i >= 0; i-- {
		if dl.ownHashes[len(dl.ownHashes)-1] == hashes[i] {
			dl.ownHashes = dl.ownHashes[:len(dl.ownHashes)-1]
		}
		delete(dl.ownChainTd, hashes[i])
		delete(dl.ownHeaders, hashes[i])
		delete(dl.ownReceipts, hashes[i])
		delete(dl.ownBlocks, hashes[i])
	}
}

//newpeer向下载程序注册一个新的块下载源。
func (dl *downloadTester) newPeer(id string, version int, chain *testChain) error {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	peer := &downloadTesterPeer{dl: dl, id: id, chain: chain}
	dl.peers[id] = peer
	return dl.downloader.RegisterPeer(id, version, peer)
}

//DropPeer模拟从连接池中删除硬对等。
func (dl *downloadTester) dropPeer(id string) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	delete(dl.peers, id)
	dl.downloader.UnregisterPeer(id)
}

type downloadTesterPeer struct {
	dl            *downloadTester
	id            string
	lock          sync.RWMutex
	chain         *testChain
missingStates map[common.Hash]bool //快速同步不应返回的状态项
}

//head构造一个函数来检索对等端的当前head哈希
//以及全部的困难。
func (dlp *downloadTesterPeer) Head() (common.Hash, *big.Int) {
	b := dlp.chain.headBlock()
	return b.Hash(), dlp.chain.td(b.Hash())
}

//RequestHeadersByHash基于哈希构造GetBlockHeaders函数
//源站；与下载测试仪中的特定对等点关联。归还的人
//函数可用于从特定的对等端检索成批的头。
func (dlp *downloadTesterPeer) RequestHeadersByHash(origin common.Hash, amount int, skip int, reverse bool) error {
	if reverse {
		panic("reverse header requests not supported")
	}

	result := dlp.chain.headersByHash(origin, amount, skip)
	go dlp.dl.downloader.DeliverHeaders(dlp.id, result)
	return nil
}

//RequestHeadersByNumber基于编号的
//源站；与下载测试仪中的特定对等点关联。归还的人
//函数可用于从特定的对等端检索成批的头。
func (dlp *downloadTesterPeer) RequestHeadersByNumber(origin uint64, amount int, skip int, reverse bool) error {
	if reverse {
		panic("reverse header requests not supported")
	}

	result := dlp.chain.headersByNumber(origin, amount, skip)
	go dlp.dl.downloader.DeliverHeaders(dlp.id, result)
	return nil
}

//RequestBodies构造与特定
//在下载测试仪中查找。返回的函数可用于检索
//来自特定请求对等端的一批块体。
func (dlp *downloadTesterPeer) RequestBodies(hashes []common.Hash) error {
	txs, uncles := dlp.chain.bodies(hashes)
	go dlp.dl.downloader.DeliverBodies(dlp.id, txs, uncles)
	return nil
}

//RequestReceipts构造与特定
//在下载测试仪中查找。返回的函数可用于检索
//来自特定请求对等方的批量块接收。
func (dlp *downloadTesterPeer) RequestReceipts(hashes []common.Hash) error {
	receipts := dlp.chain.receipts(hashes)
	go dlp.dl.downloader.DeliverReceipts(dlp.id, receipts)
	return nil
}

//RequestNodeData构造与特定
//在下载测试仪中查找。返回的函数可用于检索
//来自特定请求对等端的节点状态数据批。
func (dlp *downloadTesterPeer) RequestNodeData(hashes []common.Hash) error {
	dlp.dl.lock.RLock()
	defer dlp.dl.lock.RUnlock()

	results := make([][]byte, 0, len(hashes))
	for _, hash := range hashes {
		if data, err := dlp.dl.peerDb.Get(hash.Bytes()); err == nil {
			if !dlp.missingStates[hash] {
				results = append(results, data)
			}
		}
	}
	go dlp.dl.downloader.DeliverNodeData(dlp.id, results)
	return nil
}

//AssertownChain检查本地链是否包含正确数量的项
//各种链条组件。
func assertOwnChain(t *testing.T, tester *downloadTester, length int) {
//将此方法标记为在调用站点（而不是在此处）报告错误的帮助程序
	t.Helper()

	assertOwnForkedChain(t, tester, 1, []int{length})
}

//AssertOwnMarkedChain检查本地分叉链是否包含正确的
//各种链组件的项数。
func assertOwnForkedChain(t *testing.T, tester *downloadTester, common int, lengths []int) {
//将此方法标记为在调用站点（而不是在此处）报告错误的帮助程序
	t.Helper()

//初始化第一个分叉的计数器
	headers, blocks, receipts := lengths[0], lengths[0], lengths[0]

//更新每个后续分叉的计数器
	for _, length := range lengths[1:] {
		headers += length - common
		blocks += length - common
		receipts += length - common
	}
	if tester.downloader.mode == LightSync {
		blocks, receipts = 1, 1
	}
	if hs := len(tester.ownHeaders); hs != headers {
		t.Fatalf("synchronised headers mismatch: have %v, want %v", hs, headers)
	}
	if bs := len(tester.ownBlocks); bs != blocks {
		t.Fatalf("synchronised blocks mismatch: have %v, want %v", bs, blocks)
	}
	if rs := len(tester.ownReceipts); rs != receipts {
		t.Fatalf("synchronised receipts mismatch: have %v, want %v", rs, receipts)
	}
}

//测试针对规范链的简单同步是否正常工作。
//在此测试中，公共祖先查找应该是短路的，不需要
//二进制搜索。
func TestCanonicalSynchronisation62(t *testing.T)      { testCanonicalSynchronisation(t, 62, FullSync) }
func TestCanonicalSynchronisation63Full(t *testing.T)  { testCanonicalSynchronisation(t, 63, FullSync) }
func TestCanonicalSynchronisation63Fast(t *testing.T)  { testCanonicalSynchronisation(t, 63, FastSync) }
func TestCanonicalSynchronisation64Full(t *testing.T)  { testCanonicalSynchronisation(t, 64, FullSync) }
func TestCanonicalSynchronisation64Fast(t *testing.T)  { testCanonicalSynchronisation(t, 64, FastSync) }
func TestCanonicalSynchronisation64Light(t *testing.T) { testCanonicalSynchronisation(t, 64, LightSync) }

func testCanonicalSynchronisation(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建一个足够小的区块链来下载
	chain := testChainBase.shorten(blockCacheItems - 15)
	tester.newPeer("peer", protocol, chain)

//与对等机同步并确保检索到所有相关数据
	if err := tester.sync("peer", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, chain.len())
}

//测试如果下载了大量块，则会阻止
//直到检索到缓存块。
func TestThrottling62(t *testing.T)     { testThrottling(t, 62, FullSync) }
func TestThrottling63Full(t *testing.T) { testThrottling(t, 63, FullSync) }
func TestThrottling63Fast(t *testing.T) { testThrottling(t, 63, FastSync) }
func TestThrottling64Full(t *testing.T) { testThrottling(t, 64, FullSync) }
func TestThrottling64Fast(t *testing.T) { testThrottling(t, 64, FastSync) }

func testThrottling(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()
	tester := newTester()
	defer tester.terminate()

//创建要下载的长区块链和测试仪
	targetBlocks := testChainBase.len() - 1
	tester.newPeer("peer", protocol, testChainBase)

//包裹导入器以允许步进
	blocked, proceed := uint32(0), make(chan struct{})
	tester.downloader.chainInsertHook = func(results []*fetchResult) {
		atomic.StoreUint32(&blocked, uint32(len(results)))
		<-proceed
	}
//同时启动同步
	errc := make(chan error)
	go func() {
		errc <- tester.sync("peer", nil, mode)
	}()
//迭代获取一些块，始终检查检索计数
	for {
//同步检查检索计数（！这个丑陋街区的原因）
		tester.lock.RLock()
		retrieved := len(tester.ownBlocks)
		tester.lock.RUnlock()
		if retrieved >= targetBlocks+1 {
			break
		}
//等一下，让同步自行调节。
		var cached, frozen int
		for start := time.Now(); time.Since(start) < 3*time.Second; {
			time.Sleep(25 * time.Millisecond)

			tester.lock.Lock()
			tester.downloader.queue.lock.Lock()
			cached = len(tester.downloader.queue.blockDonePool)
			if mode == FastSync {
				if receipts := len(tester.downloader.queue.receiptDonePool); receipts < cached {
					cached = receipts
				}
			}
			frozen = int(atomic.LoadUint32(&blocked))
			retrieved = len(tester.ownBlocks)
			tester.downloader.queue.lock.Unlock()
			tester.lock.Unlock()

			if cached == blockCacheItems || cached == blockCacheItems-reorgProtHeaderDelay || retrieved+cached+frozen == targetBlocks+1 || retrieved+cached+frozen == targetBlocks+1-reorgProtHeaderDelay {
				break
			}
		}
//确保我们填满了缓存，然后耗尽它
time.Sleep(25 * time.Millisecond) //给它一个搞砸的机会

		tester.lock.RLock()
		retrieved = len(tester.ownBlocks)
		tester.lock.RUnlock()
		if cached != blockCacheItems && cached != blockCacheItems-reorgProtHeaderDelay && retrieved+cached+frozen != targetBlocks+1 && retrieved+cached+frozen != targetBlocks+1-reorgProtHeaderDelay {
			t.Fatalf("block count mismatch: have %v, want %v (owned %v, blocked %v, target %v)", cached, blockCacheItems, retrieved, frozen, targetBlocks+1)
		}
//允许导入被阻止的块
		if atomic.LoadUint32(&blocked) > 0 {
			atomic.StoreUint32(&blocked, uint32(0))
			proceed <- struct{}{}
		}
	}
//检查我们没有拖过多的街区
	assertOwnChain(t, tester, targetBlocks+1)
	if err := <-errc; err != nil {
		t.Fatalf("block synchronization failed: %v", err)
	}
}

//测试针对分叉链的简单同步是否正确工作。在
//此测试公共祖先查找应*不*短路，并且
//应执行二进制搜索。
func TestForkedSync62(t *testing.T)      { testForkedSync(t, 62, FullSync) }
func TestForkedSync63Full(t *testing.T)  { testForkedSync(t, 63, FullSync) }
func TestForkedSync63Fast(t *testing.T)  { testForkedSync(t, 63, FastSync) }
func TestForkedSync64Full(t *testing.T)  { testForkedSync(t, 64, FullSync) }
func TestForkedSync64Fast(t *testing.T)  { testForkedSync(t, 64, FastSync) }
func TestForkedSync64Light(t *testing.T) { testForkedSync(t, 64, LightSync) }

func testForkedSync(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

	chainA := testChainForkLightA.shorten(testChainBase.len() + 80)
	chainB := testChainForkLightB.shorten(testChainBase.len() + 80)
	tester.newPeer("fork A", protocol, chainA)
	tester.newPeer("fork B", protocol, chainB)

//与对等机同步并确保检索到所有块
	if err := tester.sync("fork A", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, chainA.len())

//与第二个对等机同步，并确保拨叉也被拉动
	if err := tester.sync("fork B", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnForkedChain(t, tester, testChainBase.len(), []int{chainA.len(), chainB.len()})
}

//与较短但较重的叉子同步的测试
//正确无误，不会掉落。
func TestHeavyForkedSync62(t *testing.T)      { testHeavyForkedSync(t, 62, FullSync) }
func TestHeavyForkedSync63Full(t *testing.T)  { testHeavyForkedSync(t, 63, FullSync) }
func TestHeavyForkedSync63Fast(t *testing.T)  { testHeavyForkedSync(t, 63, FastSync) }
func TestHeavyForkedSync64Full(t *testing.T)  { testHeavyForkedSync(t, 64, FullSync) }
func TestHeavyForkedSync64Fast(t *testing.T)  { testHeavyForkedSync(t, 64, FastSync) }
func TestHeavyForkedSync64Light(t *testing.T) { testHeavyForkedSync(t, 64, LightSync) }

func testHeavyForkedSync(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

	chainA := testChainForkLightA.shorten(testChainBase.len() + 80)
	chainB := testChainForkHeavy.shorten(testChainBase.len() + 80)
	tester.newPeer("light", protocol, chainA)
	tester.newPeer("heavy", protocol, chainB)

//与对等机同步并确保检索到所有块
	if err := tester.sync("light", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, chainA.len())

//与第二个对等机同步，并确保拨叉也被拉动
	if err := tester.sync("heavy", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnForkedChain(t, tester, testChainBase.len(), []int{chainA.len(), chainB.len()})
}

//测试链叉是否包含在电流的某个间隔内
//链头，确保恶意同行不会通过喂养浪费资源
//死链长。
func TestBoundedForkedSync62(t *testing.T)      { testBoundedForkedSync(t, 62, FullSync) }
func TestBoundedForkedSync63Full(t *testing.T)  { testBoundedForkedSync(t, 63, FullSync) }
func TestBoundedForkedSync63Fast(t *testing.T)  { testBoundedForkedSync(t, 63, FastSync) }
func TestBoundedForkedSync64Full(t *testing.T)  { testBoundedForkedSync(t, 64, FullSync) }
func TestBoundedForkedSync64Fast(t *testing.T)  { testBoundedForkedSync(t, 64, FastSync) }
func TestBoundedForkedSync64Light(t *testing.T) { testBoundedForkedSync(t, 64, LightSync) }

func testBoundedForkedSync(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

	chainA := testChainForkLightA
	chainB := testChainForkLightB
	tester.newPeer("original", protocol, chainA)
	tester.newPeer("rewriter", protocol, chainB)

//与对等机同步并确保检索到所有块
	if err := tester.sync("original", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, chainA.len())

//与第二个对等机同步，并确保叉被拒绝太旧
	if err := tester.sync("rewriter", nil, mode); err != errInvalidAncestor {
		t.Fatalf("sync failure mismatch: have %v, want %v", err, errInvalidAncestor)
	}
}

//测试链叉是否包含在电流的某个间隔内
//链头也适用于短而重的叉子。这些有点特别，因为它们
//采用不同的祖先查找路径。
func TestBoundedHeavyForkedSync62(t *testing.T)      { testBoundedHeavyForkedSync(t, 62, FullSync) }
func TestBoundedHeavyForkedSync63Full(t *testing.T)  { testBoundedHeavyForkedSync(t, 63, FullSync) }
func TestBoundedHeavyForkedSync63Fast(t *testing.T)  { testBoundedHeavyForkedSync(t, 63, FastSync) }
func TestBoundedHeavyForkedSync64Full(t *testing.T)  { testBoundedHeavyForkedSync(t, 64, FullSync) }
func TestBoundedHeavyForkedSync64Fast(t *testing.T)  { testBoundedHeavyForkedSync(t, 64, FastSync) }
func TestBoundedHeavyForkedSync64Light(t *testing.T) { testBoundedHeavyForkedSync(t, 64, LightSync) }

func testBoundedHeavyForkedSync(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建足够长的分叉链
	chainA := testChainForkLightA
	chainB := testChainForkHeavy
	tester.newPeer("original", protocol, chainA)
	tester.newPeer("heavy-rewriter", protocol, chainB)

//与对等机同步并确保检索到所有块
	if err := tester.sync("original", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, chainA.len())

//与第二个对等机同步，并确保叉被拒绝太旧
	if err := tester.sync("heavy-rewriter", nil, mode); err != errInvalidAncestor {
		t.Fatalf("sync failure mismatch: have %v, want %v", err, errInvalidAncestor)
	}
}

//测试非活动下载程序是否不接受传入的块头，以及
//身体。
func TestInactiveDownloader62(t *testing.T) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//检查是否不接受块头和块体
	if err := tester.downloader.DeliverHeaders("bad peer", []*types.Header{}); err != errNoSyncActive {
		t.Errorf("error mismatch: have %v, want %v", err, errNoSyncActive)
	}
	if err := tester.downloader.DeliverBodies("bad peer", [][]*types.Transaction{}, [][]*types.Header{}); err != errNoSyncActive {
		t.Errorf("error mismatch: have %v, want  %v", err, errNoSyncActive)
	}
}

//测试非活动下载程序是否不接受传入的块头，
//机构和收据。
func TestInactiveDownloader63(t *testing.T) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//检查是否不接受块头和块体
	if err := tester.downloader.DeliverHeaders("bad peer", []*types.Header{}); err != errNoSyncActive {
		t.Errorf("error mismatch: have %v, want %v", err, errNoSyncActive)
	}
	if err := tester.downloader.DeliverBodies("bad peer", [][]*types.Transaction{}, [][]*types.Header{}); err != errNoSyncActive {
		t.Errorf("error mismatch: have %v, want %v", err, errNoSyncActive)
	}
	if err := tester.downloader.DeliverReceipts("bad peer", [][]*types.Receipt{}); err != errNoSyncActive {
		t.Errorf("error mismatch: have %v, want %v", err, errNoSyncActive)
	}
}

//测试取消的下载是否会清除所有以前累积的状态。
func TestCancel62(t *testing.T)      { testCancel(t, 62, FullSync) }
func TestCancel63Full(t *testing.T)  { testCancel(t, 63, FullSync) }
func TestCancel63Fast(t *testing.T)  { testCancel(t, 63, FastSync) }
func TestCancel64Full(t *testing.T)  { testCancel(t, 64, FullSync) }
func TestCancel64Fast(t *testing.T)  { testCancel(t, 64, FastSync) }
func TestCancel64Light(t *testing.T) { testCancel(t, 64, LightSync) }

func testCancel(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

	chain := testChainBase.shorten(MaxHeaderFetch)
	tester.newPeer("peer", protocol, chain)

//确保取消与原始下载程序一起工作
	tester.downloader.Cancel()
	if !tester.downloader.queue.Idle() {
		t.Errorf("download queue not idle")
	}
//与同级同步，但随后取消
	if err := tester.sync("peer", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	tester.downloader.Cancel()
	if !tester.downloader.queue.Idle() {
		t.Errorf("download queue not idle")
	}
}

//多个对等机同步按预期工作的测试（多线程健全性测试）。
func TestMultiSynchronisation62(t *testing.T)      { testMultiSynchronisation(t, 62, FullSync) }
func TestMultiSynchronisation63Full(t *testing.T)  { testMultiSynchronisation(t, 63, FullSync) }
func TestMultiSynchronisation63Fast(t *testing.T)  { testMultiSynchronisation(t, 63, FastSync) }
func TestMultiSynchronisation64Full(t *testing.T)  { testMultiSynchronisation(t, 64, FullSync) }
func TestMultiSynchronisation64Fast(t *testing.T)  { testMultiSynchronisation(t, 64, FastSync) }
func TestMultiSynchronisation64Light(t *testing.T) { testMultiSynchronisation(t, 64, LightSync) }

func testMultiSynchronisation(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//用链的不同部分创建不同的对等点
	targetPeers := 8
	chain := testChainBase.shorten(targetPeers * 100)

	for i := 0; i < targetPeers; i++ {
		id := fmt.Sprintf("peer #%d", i)
		tester.newPeer(id, protocol, chain.shorten(chain.len()/(i+1)))
	}
	if err := tester.sync("peer #0", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, chain.len())
}

//在多版本协议环境中同步运行良好的测试
//不会对网络中的其他节点造成破坏。
func TestMultiProtoSynchronisation62(t *testing.T)      { testMultiProtoSync(t, 62, FullSync) }
func TestMultiProtoSynchronisation63Full(t *testing.T)  { testMultiProtoSync(t, 63, FullSync) }
func TestMultiProtoSynchronisation63Fast(t *testing.T)  { testMultiProtoSync(t, 63, FastSync) }
func TestMultiProtoSynchronisation64Full(t *testing.T)  { testMultiProtoSync(t, 64, FullSync) }
func TestMultiProtoSynchronisation64Fast(t *testing.T)  { testMultiProtoSync(t, 64, FastSync) }
func TestMultiProtoSynchronisation64Light(t *testing.T) { testMultiProtoSync(t, 64, LightSync) }

func testMultiProtoSync(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建一个足够小的区块链来下载
	chain := testChainBase.shorten(blockCacheItems - 15)

//创建每种类型的对等点
	tester.newPeer("peer 62", 62, chain)
	tester.newPeer("peer 63", 63, chain)
	tester.newPeer("peer 64", 64, chain)

//与请求的对等机同步，并确保检索到所有块
	if err := tester.sync(fmt.Sprintf("peer %d", protocol), nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, chain.len())

//检查是否没有同伴掉线
	for _, version := range []int{62, 63, 64} {
		peer := fmt.Sprintf("peer %d", version)
		if _, ok := tester.peers[peer]; !ok {
			t.Errorf("%s dropped", peer)
		}
	}
}

//测试如果一个块为空（例如，仅限头），则不应执行任何Body请求
//制作，而不是集管本身组装成一个整体。
func TestEmptyShortCircuit62(t *testing.T)      { testEmptyShortCircuit(t, 62, FullSync) }
func TestEmptyShortCircuit63Full(t *testing.T)  { testEmptyShortCircuit(t, 63, FullSync) }
func TestEmptyShortCircuit63Fast(t *testing.T)  { testEmptyShortCircuit(t, 63, FastSync) }
func TestEmptyShortCircuit64Full(t *testing.T)  { testEmptyShortCircuit(t, 64, FullSync) }
func TestEmptyShortCircuit64Fast(t *testing.T)  { testEmptyShortCircuit(t, 64, FastSync) }
func TestEmptyShortCircuit64Light(t *testing.T) { testEmptyShortCircuit(t, 64, LightSync) }

func testEmptyShortCircuit(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建要下载的区块链
	chain := testChainBase
	tester.newPeer("peer", protocol, chain)

//向下载器发送信号，以发出Body请求
	bodiesHave, receiptsHave := int32(0), int32(0)
	tester.downloader.bodyFetchHook = func(headers []*types.Header) {
		atomic.AddInt32(&bodiesHave, int32(len(headers)))
	}
	tester.downloader.receiptFetchHook = func(headers []*types.Header) {
		atomic.AddInt32(&receiptsHave, int32(len(headers)))
	}
//与对等机同步并确保检索到所有块
	if err := tester.sync("peer", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, chain.len())

//验证应请求的块体数
	bodiesNeeded, receiptsNeeded := 0, 0
	for _, block := range chain.blockm {
		if mode != LightSync && block != tester.genesis && (len(block.Transactions()) > 0 || len(block.Uncles()) > 0) {
			bodiesNeeded++
		}
	}
	for _, receipt := range chain.receiptm {
		if mode == FastSync && len(receipt) > 0 {
			receiptsNeeded++
		}
	}
	if int(bodiesHave) != bodiesNeeded {
		t.Errorf("body retrieval count mismatch: have %v, want %v", bodiesHave, bodiesNeeded)
	}
	if int(receiptsHave) != receiptsNeeded {
		t.Errorf("receipt retrieval count mismatch: have %v, want %v", receiptsHave, receiptsNeeded)
	}
}

//测试头是否连续排队，以防止恶意节点
//通过喂入有间隙的收割台链条来安装下载器。
func TestMissingHeaderAttack62(t *testing.T)      { testMissingHeaderAttack(t, 62, FullSync) }
func TestMissingHeaderAttack63Full(t *testing.T)  { testMissingHeaderAttack(t, 63, FullSync) }
func TestMissingHeaderAttack63Fast(t *testing.T)  { testMissingHeaderAttack(t, 63, FastSync) }
func TestMissingHeaderAttack64Full(t *testing.T)  { testMissingHeaderAttack(t, 64, FullSync) }
func TestMissingHeaderAttack64Fast(t *testing.T)  { testMissingHeaderAttack(t, 64, FastSync) }
func TestMissingHeaderAttack64Light(t *testing.T) { testMissingHeaderAttack(t, 64, LightSync) }

func testMissingHeaderAttack(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

	chain := testChainBase.shorten(blockCacheItems - 15)
	brokenChain := chain.shorten(chain.len())
	delete(brokenChain.headerm, brokenChain.chain[brokenChain.len()/2])
	tester.newPeer("attack", protocol, brokenChain)

	if err := tester.sync("attack", nil, mode); err == nil {
		t.Fatalf("succeeded attacker synchronisation")
	}
//与有效的对等机同步并确保同步成功
	tester.newPeer("valid", protocol, chain)
	if err := tester.sync("valid", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, chain.len())
}

//测试如果请求的头被移动（即第一个头丢失），队列
//检测无效编号。
func TestShiftedHeaderAttack62(t *testing.T)      { testShiftedHeaderAttack(t, 62, FullSync) }
func TestShiftedHeaderAttack63Full(t *testing.T)  { testShiftedHeaderAttack(t, 63, FullSync) }
func TestShiftedHeaderAttack63Fast(t *testing.T)  { testShiftedHeaderAttack(t, 63, FastSync) }
func TestShiftedHeaderAttack64Full(t *testing.T)  { testShiftedHeaderAttack(t, 64, FullSync) }
func TestShiftedHeaderAttack64Fast(t *testing.T)  { testShiftedHeaderAttack(t, 64, FastSync) }
func TestShiftedHeaderAttack64Light(t *testing.T) { testShiftedHeaderAttack(t, 64, LightSync) }

func testShiftedHeaderAttack(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

	chain := testChainBase.shorten(blockCacheItems - 15)

//尝试与攻击者进行完全同步以输入移位的头文件
	brokenChain := chain.shorten(chain.len())
	delete(brokenChain.headerm, brokenChain.chain[1])
	delete(brokenChain.blockm, brokenChain.chain[1])
	delete(brokenChain.receiptm, brokenChain.chain[1])
	tester.newPeer("attack", protocol, brokenChain)
	if err := tester.sync("attack", nil, mode); err == nil {
		t.Fatalf("succeeded attacker synchronisation")
	}

//与有效的对等机同步并确保同步成功
	tester.newPeer("valid", protocol, chain)
	if err := tester.sync("valid", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertOwnChain(t, tester, chain.len())
}

//测试在检测到无效的头时，将回滚最近的头
//对于各种故障情况。之后，尝试进行完全同步
//确保没有状态损坏。
func TestInvalidHeaderRollback63Fast(t *testing.T)  { testInvalidHeaderRollback(t, 63, FastSync) }
func TestInvalidHeaderRollback64Fast(t *testing.T)  { testInvalidHeaderRollback(t, 64, FastSync) }
func TestInvalidHeaderRollback64Light(t *testing.T) { testInvalidHeaderRollback(t, 64, LightSync) }

func testInvalidHeaderRollback(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

//创建一个足够小的区块链来下载
	targetBlocks := 3*fsHeaderSafetyNet + 256 + fsMinFullBlocks
	chain := testChainBase.shorten(targetBlocks)

//尝试与在快速同步阶段提供垃圾的攻击者同步。
//这将导致最后一个fsheadersafetynet头回滚。
	missing := fsHeaderSafetyNet + MaxHeaderFetch + 1
	fastAttackChain := chain.shorten(chain.len())
	delete(fastAttackChain.headerm, fastAttackChain.chain[missing])
	tester.newPeer("fast-attack", protocol, fastAttackChain)

	if err := tester.sync("fast-attack", nil, mode); err == nil {
		t.Fatalf("succeeded fast attacker synchronisation")
	}
	if head := tester.CurrentHeader().Number.Int64(); int(head) > MaxHeaderFetch {
		t.Errorf("rollback head mismatch: have %v, want at most %v", head, MaxHeaderFetch)
	}

//尝试与在块导入阶段提供垃圾的攻击者同步。
//这将导致最后一个fsheadersafetynet头的数量
//回滚，同时将轴点还原为非块状态。
	missing = 3*fsHeaderSafetyNet + MaxHeaderFetch + 1
	blockAttackChain := chain.shorten(chain.len())
delete(fastAttackChain.headerm, fastAttackChain.chain[missing]) //Make sure the fast-attacker doesn't fill in
	delete(blockAttackChain.headerm, blockAttackChain.chain[missing])
	tester.newPeer("block-attack", protocol, blockAttackChain)

	if err := tester.sync("block-attack", nil, mode); err == nil {
		t.Fatalf("succeeded block attacker synchronisation")
	}
	if head := tester.CurrentHeader().Number.Int64(); int(head) > 2*fsHeaderSafetyNet+MaxHeaderFetch {
		t.Errorf("rollback head mismatch: have %v, want at most %v", head, 2*fsHeaderSafetyNet+MaxHeaderFetch)
	}
	if mode == FastSync {
		if head := tester.CurrentBlock().NumberU64(); head != 0 {
			t.Errorf("fast sync pivot block #%d not rolled back", head)
		}
	}

//尝试与在
//快速同步轴点。这可能是让节点
//但已经导入了透视图块。
	withholdAttackChain := chain.shorten(chain.len())
	tester.newPeer("withhold-attack", protocol, withholdAttackChain)
	tester.downloader.syncInitHook = func(uint64, uint64) {
		for i := missing; i < withholdAttackChain.len(); i++ {
			delete(withholdAttackChain.headerm, withholdAttackChain.chain[i])
		}
		tester.downloader.syncInitHook = nil
	}
	if err := tester.sync("withhold-attack", nil, mode); err == nil {
		t.Fatalf("succeeded withholding attacker synchronisation")
	}
	if head := tester.CurrentHeader().Number.Int64(); int(head) > 2*fsHeaderSafetyNet+MaxHeaderFetch {
		t.Errorf("rollback head mismatch: have %v, want at most %v", head, 2*fsHeaderSafetyNet+MaxHeaderFetch)
	}
	if mode == FastSync {
		if head := tester.CurrentBlock().NumberU64(); head != 0 {
			t.Errorf("fast sync pivot block #%d not rolled back", head)
		}
	}

//与有效的对等机同步，并确保同步成功。自上次回滚以来
//还应为此进程禁用快速同步，请验证我们是否执行了新的完全同步
//同步。Note, we can't assert anything about the receipts since we won't purge the
//它们的数据库，因此我们不能使用断言。
	tester.newPeer("valid", protocol, chain)
	if err := tester.sync("valid", nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	if hs := len(tester.ownHeaders); hs != chain.len() {
		t.Fatalf("synchronised headers mismatch: have %v, want %v", hs, chain.len())
	}
	if mode != LightSync {
		if bs := len(tester.ownBlocks); bs != chain.len() {
			t.Fatalf("synchronised blocks mismatch: have %v, want %v", bs, chain.len())
		}
	}
}

//测试一个发布高TD的同龄人是否能够阻止下载者
//之后不发送任何有用的哈希。
func TestHighTDStarvationAttack62(t *testing.T)      { testHighTDStarvationAttack(t, 62, FullSync) }
func TestHighTDStarvationAttack63Full(t *testing.T)  { testHighTDStarvationAttack(t, 63, FullSync) }
func TestHighTDStarvationAttack63Fast(t *testing.T)  { testHighTDStarvationAttack(t, 63, FastSync) }
func TestHighTDStarvationAttack64Full(t *testing.T)  { testHighTDStarvationAttack(t, 64, FullSync) }
func TestHighTDStarvationAttack64Fast(t *testing.T)  { testHighTDStarvationAttack(t, 64, FastSync) }
func TestHighTDStarvationAttack64Light(t *testing.T) { testHighTDStarvationAttack(t, 64, LightSync) }

func testHighTDStarvationAttack(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()

	chain := testChainBase.shorten(1)
	tester.newPeer("attack", protocol, chain)
	if err := tester.sync("attack", big.NewInt(1000000), mode); err != errStallingPeer {
		t.Fatalf("synchronisation error mismatch: have %v, want %v", err, errStallingPeer)
	}
}

//行为不端的测试是断开连接的，而行为不正确的测试则是断开连接的。
func TestBlockHeaderAttackerDropping62(t *testing.T) { testBlockHeaderAttackerDropping(t, 62) }
func TestBlockHeaderAttackerDropping63(t *testing.T) { testBlockHeaderAttackerDropping(t, 63) }
func TestBlockHeaderAttackerDropping64(t *testing.T) { testBlockHeaderAttackerDropping(t, 64) }

func testBlockHeaderAttackerDropping(t *testing.T, protocol int) {
	t.Parallel()

//定义单个哈希获取错误的断开连接要求
	tests := []struct {
		result error
		drop   bool
	}{
{nil, false},                        //同步成功，一切正常
{errBusy, false},                    //同步已在进行中，没问题
{errUnknownPeer, false},             //对等机未知，已被删除，不要重复删除
{errBadPeer, true},                  //因为某种原因，同伴被认为是坏的，放下它。
{errStallingPeer, true},             //检测到对等机正在停止，请删除它
{errNoPeers, false},                 //无需下载同龄人，软件竞赛，无问题
{errTimeout, true},                  //在适当的时间内没有收到哈希，请删除对等机
{errEmptyHeaderSet, true},           //No headers were returned as a response, drop as it's a dead end
{errPeersUnavailable, true},         //没有人有广告块，把广告丢了
{errInvalidAncestor, true},          //同意祖先是不可接受的，请删除链重写器
{errInvalidChain, true},             //哈希链被检测为无效，肯定会丢失
{errInvalidBlock, false},            //检测到一个错误的对等机，但不是同步源
{errInvalidBody, false},             //检测到一个错误的对等机，但不是同步源
{errInvalidReceipt, false},          //检测到一个错误的对等机，但不是同步源
{errCancelBlockFetch, false},        //同步被取消，原点可能是无辜的，不要掉下来
{errCancelHeaderFetch, false},       //同步被取消，原点可能是无辜的，不要掉下来
{errCancelBodyFetch, false},         //同步被取消，原点可能是无辜的，不要掉下来
{errCancelReceiptFetch, false},      //同步被取消，原点可能是无辜的，不要掉下来
{errCancelHeaderProcessing, false},  //同步被取消，原点可能是无辜的，不要掉下来
{errCancelContentProcessing, false}, //同步被取消，原点可能是无辜的，不要掉下来
	}
//运行测试并检查断开状态
	tester := newTester()
	defer tester.terminate()
	chain := testChainBase.shorten(1)

	for i, tt := range tests {
//注册新的对等点并确保其存在
		id := fmt.Sprintf("test %d", i)
		if err := tester.newPeer(id, protocol, chain); err != nil {
			t.Fatalf("test %d: failed to register new peer: %v", i, err)
		}
		if _, ok := tester.peers[id]; !ok {
			t.Fatalf("test %d: registered peer not found", i)
		}
//模拟同步并检查所需结果
		tester.downloader.synchroniseMock = func(string, common.Hash) error { return tt.result }

		tester.downloader.Synchronise(id, tester.genesis.Hash(), big.NewInt(1000), FullSync)
		if _, ok := tester.peers[id]; !ok != tt.drop {
			t.Errorf("test %d: peer drop mismatch for %v: have %v, want %v", i, tt.result, !ok, tt.drop)
		}
	}
}

//Tests that synchronisation progress (origin block number, current block number
//以及最高的块号）被正确地跟踪和更新。
func TestSyncProgress62(t *testing.T)      { testSyncProgress(t, 62, FullSync) }
func TestSyncProgress63Full(t *testing.T)  { testSyncProgress(t, 63, FullSync) }
func TestSyncProgress63Fast(t *testing.T)  { testSyncProgress(t, 63, FastSync) }
func TestSyncProgress64Full(t *testing.T)  { testSyncProgress(t, 64, FullSync) }
func TestSyncProgress64Fast(t *testing.T)  { testSyncProgress(t, 64, FastSync) }
func TestSyncProgress64Light(t *testing.T) { testSyncProgress(t, 64, LightSync) }

func testSyncProgress(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()
	chain := testChainBase.shorten(blockCacheItems - 15)

//设置同步初始化挂钩以捕获进度更改
	starting := make(chan struct{})
	progress := make(chan struct{})

	tester.downloader.syncInitHook = func(origin, latest uint64) {
		starting <- struct{}{}
		<-progress
	}
	checkProgress(t, tester.downloader, "pristine", ethereum.SyncProgress{})

//同步一半块并检查初始进度
	tester.newPeer("peer-half", protocol, chain.shorten(chain.len()/2))
	pending := new(sync.WaitGroup)
	pending.Add(1)

	go func() {
		defer pending.Done()
		if err := tester.sync("peer-half", nil, mode); err != nil {
			panic(fmt.Sprintf("failed to synchronise blocks: %v", err))
		}
	}()
	<-starting
	checkProgress(t, tester.downloader, "initial", ethereum.SyncProgress{
		HighestBlock: uint64(chain.len()/2 - 1),
	})
	progress <- struct{}{}
	pending.Wait()

//同步所有块并检查继续进度
	tester.newPeer("peer-full", protocol, chain)
	pending.Add(1)
	go func() {
		defer pending.Done()
		if err := tester.sync("peer-full", nil, mode); err != nil {
			panic(fmt.Sprintf("failed to synchronise blocks: %v", err))
		}
	}()
	<-starting
	checkProgress(t, tester.downloader, "completing", ethereum.SyncProgress{
		StartingBlock: uint64(chain.len()/2 - 1),
		CurrentBlock:  uint64(chain.len()/2 - 1),
		HighestBlock:  uint64(chain.len() - 1),
	})

//同步成功后检查最终进度
	progress <- struct{}{}
	pending.Wait()
	checkProgress(t, tester.downloader, "final", ethereum.SyncProgress{
		StartingBlock: uint64(chain.len()/2 - 1),
		CurrentBlock:  uint64(chain.len() - 1),
		HighestBlock:  uint64(chain.len() - 1),
	})
}

func checkProgress(t *testing.T, d *Downloader, stage string, want ethereum.SyncProgress) {
//将此方法标记为在调用站点（而不是在此处）报告错误的帮助程序
	t.Helper()

	p := d.Progress()
	p.KnownStates, p.PulledStates = 0, 0
	want.KnownStates, want.PulledStates = 0, 0
	if p != want {
		t.Fatalf("%s progress mismatch:\nhave %+v\nwant %+v", stage, p, want)
	}
}

//测试同步进程（起始块编号和最高块
//如果是叉子（或手动头），则正确跟踪和更新数字。
//回复）
func TestForkedSyncProgress62(t *testing.T)      { testForkedSyncProgress(t, 62, FullSync) }
func TestForkedSyncProgress63Full(t *testing.T)  { testForkedSyncProgress(t, 63, FullSync) }
func TestForkedSyncProgress63Fast(t *testing.T)  { testForkedSyncProgress(t, 63, FastSync) }
func TestForkedSyncProgress64Full(t *testing.T)  { testForkedSyncProgress(t, 64, FullSync) }
func TestForkedSyncProgress64Fast(t *testing.T)  { testForkedSyncProgress(t, 64, FastSync) }
func TestForkedSyncProgress64Light(t *testing.T) { testForkedSyncProgress(t, 64, LightSync) }

func testForkedSyncProgress(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()
	chainA := testChainForkLightA.shorten(testChainBase.len() + MaxHashFetch)
	chainB := testChainForkLightB.shorten(testChainBase.len() + MaxHashFetch)

//设置同步初始化挂钩以捕获进度更改
	starting := make(chan struct{})
	progress := make(chan struct{})

	tester.downloader.syncInitHook = func(origin, latest uint64) {
		starting <- struct{}{}
		<-progress
	}
	checkProgress(t, tester.downloader, "pristine", ethereum.SyncProgress{})

//与其中一个拨叉同步并检查进度
	tester.newPeer("fork A", protocol, chainA)
	pending := new(sync.WaitGroup)
	pending.Add(1)
	go func() {
		defer pending.Done()
		if err := tester.sync("fork A", nil, mode); err != nil {
			panic(fmt.Sprintf("failed to synchronise blocks: %v", err))
		}
	}()
	<-starting

	checkProgress(t, tester.downloader, "initial", ethereum.SyncProgress{
		HighestBlock: uint64(chainA.len() - 1),
	})
	progress <- struct{}{}
	pending.Wait()

//在分叉上方模拟成功的同步
	tester.downloader.syncStatsChainOrigin = tester.downloader.syncStatsChainHeight

//与第二个拨叉同步并检查进度重置
	tester.newPeer("fork B", protocol, chainB)
	pending.Add(1)
	go func() {
		defer pending.Done()
		if err := tester.sync("fork B", nil, mode); err != nil {
			panic(fmt.Sprintf("failed to synchronise blocks: %v", err))
		}
	}()
	<-starting
	checkProgress(t, tester.downloader, "forking", ethereum.SyncProgress{
		StartingBlock: uint64(testChainBase.len()) - 1,
		CurrentBlock:  uint64(chainA.len() - 1),
		HighestBlock:  uint64(chainB.len() - 1),
	})

//同步成功后检查最终进度
	progress <- struct{}{}
	pending.Wait()
	checkProgress(t, tester.downloader, "final", ethereum.SyncProgress{
		StartingBlock: uint64(testChainBase.len()) - 1,
		CurrentBlock:  uint64(chainB.len() - 1),
		HighestBlock:  uint64(chainB.len() - 1),
	})
}

//测试如果同步由于某些故障而中止，则进度
//原点不会在下一个同步周期中更新，因为它应该被视为
//继续上一次同步，而不是新实例。
func TestFailedSyncProgress62(t *testing.T)      { testFailedSyncProgress(t, 62, FullSync) }
func TestFailedSyncProgress63Full(t *testing.T)  { testFailedSyncProgress(t, 63, FullSync) }
func TestFailedSyncProgress63Fast(t *testing.T)  { testFailedSyncProgress(t, 63, FastSync) }
func TestFailedSyncProgress64Full(t *testing.T)  { testFailedSyncProgress(t, 64, FullSync) }
func TestFailedSyncProgress64Fast(t *testing.T)  { testFailedSyncProgress(t, 64, FastSync) }
func TestFailedSyncProgress64Light(t *testing.T) { testFailedSyncProgress(t, 64, LightSync) }

func testFailedSyncProgress(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()
	chain := testChainBase.shorten(blockCacheItems - 15)

//设置同步初始化挂钩以捕获进度更改
	starting := make(chan struct{})
	progress := make(chan struct{})

	tester.downloader.syncInitHook = func(origin, latest uint64) {
		starting <- struct{}{}
		<-progress
	}
	checkProgress(t, tester.downloader, "pristine", ethereum.SyncProgress{})

//尝试与有故障的对等机完全同步
	brokenChain := chain.shorten(chain.len())
	missing := brokenChain.len() / 2
	delete(brokenChain.headerm, brokenChain.chain[missing])
	delete(brokenChain.blockm, brokenChain.chain[missing])
	delete(brokenChain.receiptm, brokenChain.chain[missing])
	tester.newPeer("faulty", protocol, brokenChain)

	pending := new(sync.WaitGroup)
	pending.Add(1)
	go func() {
		defer pending.Done()
		if err := tester.sync("faulty", nil, mode); err == nil {
			panic("succeeded faulty synchronisation")
		}
	}()
	<-starting
	checkProgress(t, tester.downloader, "initial", ethereum.SyncProgress{
		HighestBlock: uint64(brokenChain.len() - 1),
	})
	progress <- struct{}{}
	pending.Wait()
	afterFailedSync := tester.downloader.Progress()

//与一个好的同行同步，并检查进度来源是否提醒相同
//失败后
	tester.newPeer("valid", protocol, chain)
	pending.Add(1)
	go func() {
		defer pending.Done()
		if err := tester.sync("valid", nil, mode); err != nil {
			panic(fmt.Sprintf("failed to synchronise blocks: %v", err))
		}
	}()
	<-starting
	checkProgress(t, tester.downloader, "completing", afterFailedSync)

//同步成功后检查最终进度
	progress <- struct{}{}
	pending.Wait()
	checkProgress(t, tester.downloader, "final", ethereum.SyncProgress{
		CurrentBlock: uint64(chain.len() - 1),
		HighestBlock: uint64(chain.len() - 1),
	})
}

//测试在检测到攻击后，如果攻击者伪造链高度，
//在下次同步调用时，进度高度已成功降低。
func TestFakedSyncProgress62(t *testing.T)      { testFakedSyncProgress(t, 62, FullSync) }
func TestFakedSyncProgress63Full(t *testing.T)  { testFakedSyncProgress(t, 63, FullSync) }
func TestFakedSyncProgress63Fast(t *testing.T)  { testFakedSyncProgress(t, 63, FastSync) }
func TestFakedSyncProgress64Full(t *testing.T)  { testFakedSyncProgress(t, 64, FullSync) }
func TestFakedSyncProgress64Fast(t *testing.T)  { testFakedSyncProgress(t, 64, FastSync) }
func TestFakedSyncProgress64Light(t *testing.T) { testFakedSyncProgress(t, 64, LightSync) }

func testFakedSyncProgress(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()

	tester := newTester()
	defer tester.terminate()
	chain := testChainBase.shorten(blockCacheItems - 15)

//设置同步初始化挂钩以捕获进度更改
	starting := make(chan struct{})
	progress := make(chan struct{})
	tester.downloader.syncInitHook = func(origin, latest uint64) {
		starting <- struct{}{}
		<-progress
	}
	checkProgress(t, tester.downloader, "pristine", ethereum.SyncProgress{})

//创建一个攻击者并与之同步，该攻击者承诺拥有比可用的更高的链。
	brokenChain := chain.shorten(chain.len())
	numMissing := 5
	for i := brokenChain.len() - 2; i > brokenChain.len()-numMissing; i-- {
		delete(brokenChain.headerm, brokenChain.chain[i])
	}
	tester.newPeer("attack", protocol, brokenChain)

	pending := new(sync.WaitGroup)
	pending.Add(1)
	go func() {
		defer pending.Done()
		if err := tester.sync("attack", nil, mode); err == nil {
			panic("succeeded attacker synchronisation")
		}
	}()
	<-starting
	checkProgress(t, tester.downloader, "initial", ethereum.SyncProgress{
		HighestBlock: uint64(brokenChain.len() - 1),
	})
	progress <- struct{}{}
	pending.Wait()
	afterFailedSync := tester.downloader.Progress()

//与一个好的同伴同步，并检查进度高度是否已降低到
//真正的价值。
	validChain := chain.shorten(chain.len() - numMissing)
	tester.newPeer("valid", protocol, validChain)
	pending.Add(1)

	go func() {
		defer pending.Done()
		if err := tester.sync("valid", nil, mode); err != nil {
			panic(fmt.Sprintf("failed to synchronise blocks: %v", err))
		}
	}()
	<-starting
	checkProgress(t, tester.downloader, "completing", ethereum.SyncProgress{
		CurrentBlock: afterFailedSync.CurrentBlock,
		HighestBlock: uint64(validChain.len() - 1),
	})

//同步成功后检查最终进度。
	progress <- struct{}{}
	pending.Wait()
	checkProgress(t, tester.downloader, "final", ethereum.SyncProgress{
		CurrentBlock: uint64(validChain.len() - 1),
		HighestBlock: uint64(validChain.len() - 1),
	})
}

//此测试复制了一个问题，即意外交付
//如果他们到达正确的时间，无限期地封锁。
func TestDeliverHeadersHang(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		protocol int
		syncMode SyncMode
	}{
		{62, FullSync},
		{63, FullSync},
		{63, FastSync},
		{64, FullSync},
		{64, FastSync},
		{64, LightSync},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("protocol %d mode %v", tc.protocol, tc.syncMode), func(t *testing.T) {
			t.Parallel()
			testDeliverHeadersHang(t, tc.protocol, tc.syncMode)
		})
	}
}

func testDeliverHeadersHang(t *testing.T, protocol int, mode SyncMode) {
	master := newTester()
	defer master.terminate()
	chain := testChainBase.shorten(15)

	for i := 0; i < 200; i++ {
		tester := newTester()
		tester.peerDb = master.peerDb
		tester.newPeer("peer", protocol, chain)

//每当下载程序请求报头时，就用
//很多未请求的头部交付。
		tester.downloader.peers.peers["peer"].peer = &floodingTestPeer{
			peer:   tester.downloader.peers.peers["peer"].peer,
			tester: tester,
		}
		if err := tester.sync("peer", nil, mode); err != nil {
			t.Errorf("test %d: sync failed: %v", i, err)
		}
		tester.terminate()
	}
}

type floodingTestPeer struct {
	peer   Peer
	tester *downloadTester
}

func (ftp *floodingTestPeer) Head() (common.Hash, *big.Int) { return ftp.peer.Head() }
func (ftp *floodingTestPeer) RequestHeadersByHash(hash common.Hash, count int, skip int, reverse bool) error {
	return ftp.peer.RequestHeadersByHash(hash, count, skip, reverse)
}
func (ftp *floodingTestPeer) RequestBodies(hashes []common.Hash) error {
	return ftp.peer.RequestBodies(hashes)
}
func (ftp *floodingTestPeer) RequestReceipts(hashes []common.Hash) error {
	return ftp.peer.RequestReceipts(hashes)
}
func (ftp *floodingTestPeer) RequestNodeData(hashes []common.Hash) error {
	return ftp.peer.RequestNodeData(hashes)
}

func (ftp *floodingTestPeer) RequestHeadersByNumber(from uint64, count, skip int, reverse bool) error {
	deliveriesDone := make(chan struct{}, 500)
	for i := 0; i < cap(deliveriesDone)-1; i++ {
		peer := fmt.Sprintf("fake-peer%d", i)
		go func() {
			ftp.tester.downloader.DeliverHeaders(peer, []*types.Header{{}, {}, {}, {}})
			deliveriesDone <- struct{}{}
		}()
	}

//额外交货不应受阻。
	timeout := time.After(60 * time.Second)
	launched := false
	for i := 0; i < cap(deliveriesDone); i++ {
		select {
		case <-deliveriesDone:
			if !launched {
//开始传递请求的邮件头
//在其中一个洪水响应到达之后。
				go func() {
					ftp.peer.RequestHeadersByNumber(from, count, skip, reverse)
					deliveriesDone <- struct{}{}
				}()
				launched = true
			}
		case <-timeout:
			panic("blocked")
		}
	}
	return nil
}

func TestRemoteHeaderRequestSpan(t *testing.T) {
	testCases := []struct {
		remoteHeight uint64
		localHeight  uint64
		expected     []int
	}{
//遥控器更高。我们应该要遥控器的头，然后往回走
		{1500, 1000,
			[]int{1323, 1339, 1355, 1371, 1387, 1403, 1419, 1435, 1451, 1467, 1483, 1499},
		},
		{15000, 13006,
			[]int{14823, 14839, 14855, 14871, 14887, 14903, 14919, 14935, 14951, 14967, 14983, 14999},
		},
//遥控器离我们很近。我们不必取那么多
		{1200, 1150,
			[]int{1149, 1154, 1159, 1164, 1169, 1174, 1179, 1184, 1189, 1194, 1199},
		},
//远程等于我们（所以在TD较高的分叉上）
//我们应该找到最亲近的两个祖先
		{1500, 1500,
			[]int{1497, 1499},
		},
//我们比遥控器还高！奇数
		{1000, 1500,
			[]int{997, 999},
		},
//检查一些奇怪的边缘，因为它的行为有些理性。
		{0, 1500,
			[]int{0, 2},
		},
		{6000000, 0,
			[]int{5999823, 5999839, 5999855, 5999871, 5999887, 5999903, 5999919, 5999935, 5999951, 5999967, 5999983, 5999999},
		},
		{0, 0,
			[]int{0, 2},
		},
	}
	reqs := func(from, count, span int) []int {
		var r []int
		num := from
		for len(r) < count {
			r = append(r, num)
			num += span + 1
		}
		return r
	}
	for i, tt := range testCases {
		from, count, span, max := calculateRequestSpan(tt.remoteHeight, tt.localHeight)
		data := reqs(int(from), count, span)

		if max != uint64(data[len(data)-1]) {
			t.Errorf("test %d: wrong last value %d != %d", i, data[len(data)-1], max)
		}
		failed := false
		if len(data) != len(tt.expected) {
			failed = true
			t.Errorf("test %d: length wrong, expected %d got %d", i, len(tt.expected), len(data))
		} else {
			for j, n := range data {
				if n != tt.expected[j] {
					failed = true
					break
				}
			}
		}
		if failed {
			res := strings.Replace(fmt.Sprint(data), " ", ",", -1)
			exp := strings.Replace(fmt.Sprint(tt.expected), " ", ",", -1)
			fmt.Printf("got: %v\n", res)
			fmt.Printf("exp: %v\n", exp)
			t.Errorf("test %d: wrong values", i)
		}
	}
}
