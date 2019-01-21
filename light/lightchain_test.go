
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
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
)

//所以我们可以确定地植入不同的区块链
var (
	canonicalSeed = 1
	forkSeed      = 2
)

//MakeHeaderChain创建一个以父级为根的具有确定性的头链。
func makeHeaderChain(parent *types.Header, n int, db ethdb.Database, seed int) []*types.Header {
	blocks, _ := core.GenerateChain(params.TestChainConfig, types.NewBlockWithHeader(parent), ethash.NewFaker(), db, n, func(i int, b *core.BlockGen) {
		b.SetCoinbase(common.Address{0: byte(seed), 19: byte(i)})
	})
	headers := make([]*types.Header, len(blocks))
	for i, block := range blocks {
		headers[i] = block.Header()
	}
	return headers
}

//newcanonical创建一个链数据库，并注入一个确定性canonical
//链。根据完整标志，如果创建完整的区块链或
//仅限收割台链条。
func newCanonical(n int) (ethdb.Database, *LightChain, error) {
	db := ethdb.NewMemDatabase()
	gspec := core.Genesis{Config: params.TestChainConfig}
	genesis := gspec.MustCommit(db)
	blockchain, _ := NewLightChain(&dummyOdr{db: db, indexerConfig: TestClientIndexerConfig}, gspec.Config, ethash.NewFaker())

//创建并注入请求的链
	if n == 0 {
		return db, blockchain, nil
	}
//仅请求头链
	headers := makeHeaderChain(genesis.Header(), n, db, canonicalSeed)
	_, err := blockchain.InsertHeaderChain(headers, 1)
	return db, blockchain, err
}

//NewTestLightChain创建了一个不验证任何内容的LightChain。
func newTestLightChain() *LightChain {
	db := ethdb.NewMemDatabase()
	gspec := &core.Genesis{
		Difficulty: big.NewInt(1),
		Config:     params.TestChainConfig,
	}
	gspec.MustCommit(db)
	lc, err := NewLightChain(&dummyOdr{db: db}, gspec.Config, ethash.NewFullFaker())
	if err != nil {
		panic(err)
	}
	return lc
}

//从I区开始的长度为n的测试叉
func testFork(t *testing.T, LightChain *LightChain, i, n int, comparator func(td1, td2 *big.Int)) {
//将旧链复制到新数据库中
	db, LightChain2, err := newCanonical(i)
	if err != nil {
		t.Fatal("could not make new canonical in testFork", err)
	}
//断言链在i处具有相同的头/块
	var hash1, hash2 common.Hash
	hash1 = LightChain.GetHeaderByNumber(uint64(i)).Hash()
	hash2 = LightChain2.GetHeaderByNumber(uint64(i)).Hash()
	if hash1 != hash2 {
		t.Errorf("chain content mismatch at %d: have hash %v, want hash %v", i, hash2, hash1)
	}
//扩展新创建的链
	headerChainB := makeHeaderChain(LightChain2.CurrentHeader(), n, db, forkSeed)
	if _, err := LightChain2.InsertHeaderChain(headerChainB, 1); err != nil {
		t.Fatalf("failed to insert forking chain: %v", err)
	}
//健全性检查叉链是否可以导入原始
	var tdPre, tdPost *big.Int

	tdPre = LightChain.GetTdByHash(LightChain.CurrentHeader().Hash())
	if err := testHeaderChainImport(headerChainB, LightChain); err != nil {
		t.Fatalf("failed to import forked header chain: %v", err)
	}
	tdPost = LightChain.GetTdByHash(headerChainB[len(headerChainB)-1].Hash())
//比较链条的总难度
	comparator(tdPre, tdPost)
}

//testHeaderChainimport尝试处理一个头链，将其写入
//如果成功，则返回数据库。
func testHeaderChainImport(chain []*types.Header, lightchain *LightChain) error {
	for _, header := range chain {
//尝试验证标题
		if err := lightchain.engine.VerifyHeader(lightchain.hc, header, true); err != nil {
			return err
		}
//手动将头插入数据库，但不重新组织（允许后续测试）
		lightchain.chainmu.Lock()
		rawdb.WriteTd(lightchain.chainDb, header.Hash(), header.Number.Uint64(), new(big.Int).Add(header.Difficulty, lightchain.GetTdByHash(header.ParentHash)))
		rawdb.WriteHeader(lightchain.chainDb, header)
		lightchain.chainmu.Unlock()
	}
	return nil
}

//测试给定一个给定大小的起始规范链，它可以被扩展
//有各种长度的链条。
func TestExtendCanonicalHeaders(t *testing.T) {
	length := 5

//从创世纪开始制造第一条链条
	_, processor, err := newCanonical(length)
	if err != nil {
		t.Fatalf("failed to make new canonical chain: %v", err)
	}
//定义难度比较器
	better := func(td1, td2 *big.Int) {
		if td2.Cmp(td1) <= 0 {
			t.Errorf("total difficulty mismatch: have %v, expected more than %v", td2, td1)
		}
	}
//从当前高度开始拨叉
	testFork(t, processor, length, 1, better)
	testFork(t, processor, length, 2, better)
	testFork(t, processor, length, 5, better)
	testFork(t, processor, length, 10, better)
}

//测试给定给定大小的起始规范链，创建较短的
//forks不具有规范的所有权。
func TestShorterForkHeaders(t *testing.T) {
	length := 10

//从创世纪开始制造第一条链条
	_, processor, err := newCanonical(length)
	if err != nil {
		t.Fatalf("failed to make new canonical chain: %v", err)
	}
//定义难度比较器
	worse := func(td1, td2 *big.Int) {
		if td2.Cmp(td1) >= 0 {
			t.Errorf("total difficulty mismatch: have %v, expected less than %v", td2, td1)
		}
	}
//数字之和必须小于“length”，才能成为较短的分叉
	testFork(t, processor, 0, 3, worse)
	testFork(t, processor, 0, 7, worse)
	testFork(t, processor, 1, 1, worse)
	testFork(t, processor, 1, 7, worse)
	testFork(t, processor, 5, 3, worse)
	testFork(t, processor, 5, 4, worse)
}

//测试给定给定大小的起始规范链，创建更长的
//forks确实拥有规范的所有权。
func TestLongerForkHeaders(t *testing.T) {
	length := 10

//从创世纪开始制造第一条链条
	_, processor, err := newCanonical(length)
	if err != nil {
		t.Fatalf("failed to make new canonical chain: %v", err)
	}
//定义难度比较器
	better := func(td1, td2 *big.Int) {
		if td2.Cmp(td1) <= 0 {
			t.Errorf("total difficulty mismatch: have %v, expected more than %v", td2, td1)
		}
	}
//数字之和必须大于“length”，才能成为较长的分叉
	testFork(t, processor, 0, 11, better)
	testFork(t, processor, 0, 15, better)
	testFork(t, processor, 1, 10, better)
	testFork(t, processor, 1, 12, better)
	testFork(t, processor, 5, 6, better)
	testFork(t, processor, 5, 8, better)
}

//测试给定给定大小的起始规范链，创建相等的
//forks确实拥有规范的所有权。
func TestEqualForkHeaders(t *testing.T) {
	length := 10

//从创世纪开始制造第一条链条
	_, processor, err := newCanonical(length)
	if err != nil {
		t.Fatalf("failed to make new canonical chain: %v", err)
	}
//定义难度比较器
	equal := func(td1, td2 *big.Int) {
		if td2.Cmp(td1) != 0 {
			t.Errorf("total difficulty mismatch: have %v, want %v", td2, td1)
		}
	}
//数字之和必须等于“length”，才能成为相等的叉
	testFork(t, processor, 0, 10, equal)
	testFork(t, processor, 1, 9, equal)
	testFork(t, processor, 2, 8, equal)
	testFork(t, processor, 5, 5, equal)
	testFork(t, processor, 6, 4, equal)
	testFork(t, processor, 9, 1, equal)
}

//处理器不接受链接缺失的测试。
func TestBrokenHeaderChain(t *testing.T) {
//从Genesis开始制作链条
	db, LightChain, err := newCanonical(10)
	if err != nil {
		t.Fatalf("failed to make new canonical chain: %v", err)
	}
//创建分叉链，并尝试插入缺少的链接
	chain := makeHeaderChain(LightChain.CurrentHeader(), 5, db, forkSeed)[1:]
	if err := testHeaderChainImport(chain, LightChain); err == nil {
		t.Errorf("broken header chain not reported")
	}
}

func makeHeaderChainWithDiff(genesis *types.Block, d []int, seed byte) []*types.Header {
	var chain []*types.Header
	for i, difficulty := range d {
		header := &types.Header{
			Coinbase:    common.Address{seed},
			Number:      big.NewInt(int64(i + 1)),
			Difficulty:  big.NewInt(int64(difficulty)),
			UncleHash:   types.EmptyUncleHash,
			TxHash:      types.EmptyRootHash,
			ReceiptHash: types.EmptyRootHash,
		}
		if i == 0 {
			header.ParentHash = genesis.Hash()
		} else {
			header.ParentHash = chain[i-1].Hash()
		}
		chain = append(chain, types.CopyHeader(header))
	}
	return chain
}

type dummyOdr struct {
	OdrBackend
	db            ethdb.Database
	indexerConfig *IndexerConfig
}

func (odr *dummyOdr) Database() ethdb.Database {
	return odr.db
}

func (odr *dummyOdr) Retrieve(ctx context.Context, req OdrRequest) error {
	return nil
}

func (odr *dummyOdr) IndexerConfig() *IndexerConfig {
	return odr.indexerConfig
}

//在短而容易的链之后重新组织长而难的链的测试
//覆盖数据库中的规范数字和链接。
func TestReorgLongHeaders(t *testing.T) {
	testReorg(t, []int{1, 2, 4}, []int{1, 2, 3, 4}, 10)
}

//在长而容易的链之后重新组织短而难的链的测试
//覆盖数据库中的规范数字和链接。
func TestReorgShortHeaders(t *testing.T) {
	testReorg(t, []int{1, 2, 3, 4}, []int{1, 10}, 11)
}

func testReorg(t *testing.T, first, second []int, td int64) {
	bc := newTestLightChain()

//然后插入一个容易和困难的链条
	bc.InsertHeaderChain(makeHeaderChainWithDiff(bc.genesisBlock, first, 11), 1)
	bc.InsertHeaderChain(makeHeaderChainWithDiff(bc.genesisBlock, second, 22), 1)
//检查链条是否为有效的数字和链接方式
	prev := bc.CurrentHeader()
	for header := bc.GetHeaderByNumber(bc.CurrentHeader().Number.Uint64() - 1); header.Number.Uint64() != 0; prev, header = header, bc.GetHeaderByNumber(header.Number.Uint64()-1) {
		if prev.ParentHash != header.Hash() {
			t.Errorf("parent header hash mismatch: have %x, want %x", prev.ParentHash, header.Hash())
		}
	}
//确保链条的总难度是正确的。
	want := new(big.Int).Add(bc.genesisBlock.Difficulty(), big.NewInt(td))
	if have := bc.GetTdByHash(bc.CurrentHeader().Hash()); have.Cmp(want) != 0 {
		t.Errorf("total difficulty mismatch: have %v, want %v", have, want)
	}
}

//测试插入函数是否检测禁止的哈希。
func TestBadHeaderHashes(t *testing.T) {
	bc := newTestLightChain()

//创建链，禁止散列并尝试导入
	var err error
	headers := makeHeaderChainWithDiff(bc.genesisBlock, []int{1, 2, 4}, 10)
	core.BadHashes[headers[2].Hash()] = true
	if _, err = bc.InsertHeaderChain(headers, 1); err != core.ErrBlacklistedHash {
		t.Errorf("error mismatch: have: %v, want %v", err, core.ErrBlacklistedHash)
	}
}

//测试在引导时检测到错误的哈希值，并将chan回滚到
//坏哈希前的良好状态。
func TestReorgBadHeaderHashes(t *testing.T) {
	bc := newTestLightChain()

//创建一个链，然后导入和禁止
	headers := makeHeaderChainWithDiff(bc.genesisBlock, []int{1, 2, 3, 4}, 10)

	if _, err := bc.InsertHeaderChain(headers, 1); err != nil {
		t.Fatalf("failed to import headers: %v", err)
	}
	if bc.CurrentHeader().Hash() != headers[3].Hash() {
		t.Errorf("last header hash mismatch: have: %x, want %x", bc.CurrentHeader().Hash(), headers[3].Hash())
	}
	core.BadHashes[headers[3].Hash()] = true
	defer func() { delete(core.BadHashes, headers[3].Hash()) }()

//创建一个新的lightchain并检查它是否回滚状态。
	ncm, err := NewLightChain(&dummyOdr{db: bc.chainDb}, params.TestChainConfig, ethash.NewFaker())
	if err != nil {
		t.Fatalf("failed to create new chain manager: %v", err)
	}
	if ncm.CurrentHeader().Hash() != headers[2].Hash() {
		t.Errorf("last header hash mismatch: have: %x, want %x", ncm.CurrentHeader().Hash(), headers[2].Hash())
	}
}
