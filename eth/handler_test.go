
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

package eth

import (
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/params"
)

//测试协议版本和操作模式是否正确匹配。
func TestProtocolCompatibility(t *testing.T) {
//定义兼容性图表
	tests := []struct {
		version    uint
		mode       downloader.SyncMode
		compatible bool
	}{
		{61, downloader.FullSync, true}, {62, downloader.FullSync, true}, {63, downloader.FullSync, true},
		{61, downloader.FastSync, false}, {62, downloader.FastSync, false}, {63, downloader.FastSync, true},
	}
//确保我们搞砸的东西都恢复了
	backup := ProtocolVersions
	defer func() { ProtocolVersions = backup }()

//尝试所有可用的兼容性配置并检查错误
	for i, tt := range tests {
		ProtocolVersions = []uint{tt.version}

		pm, _, err := newTestProtocolManager(tt.mode, 0, nil, nil)
		if pm != nil {
			defer pm.Stop()
		}
		if (err == nil && !tt.compatible) || (err != nil && tt.compatible) {
			t.Errorf("test %d: compatibility mismatch: have error %v, want compatibility %v", i, err, tt.compatible)
		}
	}
}

//可以根据用户查询从远程链中检索阻止头的测试。
func TestGetBlockHeaders62(t *testing.T) { testGetBlockHeaders(t, 62) }
func TestGetBlockHeaders63(t *testing.T) { testGetBlockHeaders(t, 63) }

func testGetBlockHeaders(t *testing.T, protocol int) {
	pm, _ := newTestProtocolManagerMust(t, downloader.FullSync, downloader.MaxHashFetch+15, nil, nil)
	peer, _ := newTestPeer("peer", protocol, pm, true)
	defer peer.close()

//为测试创建一个“随机”的未知哈希
	var unknown common.Hash
	for i := range unknown {
		unknown[i] = byte(i)
	}
//为各种方案创建一批测试
	limit := uint64(downloader.MaxHeaderFetch)
	tests := []struct {
query  *getBlockHeadersData //要为头检索执行的查询
expect []common.Hash        //应为其头的块的哈希
	}{
//单个随机块也应该可以通过哈希和数字检索。
		{
			&getBlockHeadersData{Origin: hashOrNumber{Hash: pm.blockchain.GetBlockByNumber(limit / 2).Hash()}, Amount: 1},
			[]common.Hash{pm.blockchain.GetBlockByNumber(limit / 2).Hash()},
		}, {
			&getBlockHeadersData{Origin: hashOrNumber{Number: limit / 2}, Amount: 1},
			[]common.Hash{pm.blockchain.GetBlockByNumber(limit / 2).Hash()},
		},
//应可从两个方向检索多个邮件头
		{
			&getBlockHeadersData{Origin: hashOrNumber{Number: limit / 2}, Amount: 3},
			[]common.Hash{
				pm.blockchain.GetBlockByNumber(limit / 2).Hash(),
				pm.blockchain.GetBlockByNumber(limit/2 + 1).Hash(),
				pm.blockchain.GetBlockByNumber(limit/2 + 2).Hash(),
			},
		}, {
			&getBlockHeadersData{Origin: hashOrNumber{Number: limit / 2}, Amount: 3, Reverse: true},
			[]common.Hash{
				pm.blockchain.GetBlockByNumber(limit / 2).Hash(),
				pm.blockchain.GetBlockByNumber(limit/2 - 1).Hash(),
				pm.blockchain.GetBlockByNumber(limit/2 - 2).Hash(),
			},
		},
//应检索具有跳过列表的多个邮件头
		{
			&getBlockHeadersData{Origin: hashOrNumber{Number: limit / 2}, Skip: 3, Amount: 3},
			[]common.Hash{
				pm.blockchain.GetBlockByNumber(limit / 2).Hash(),
				pm.blockchain.GetBlockByNumber(limit/2 + 4).Hash(),
				pm.blockchain.GetBlockByNumber(limit/2 + 8).Hash(),
			},
		}, {
			&getBlockHeadersData{Origin: hashOrNumber{Number: limit / 2}, Skip: 3, Amount: 3, Reverse: true},
			[]common.Hash{
				pm.blockchain.GetBlockByNumber(limit / 2).Hash(),
				pm.blockchain.GetBlockByNumber(limit/2 - 4).Hash(),
				pm.blockchain.GetBlockByNumber(limit/2 - 8).Hash(),
			},
		},
//链端点应该是可检索的
		{
			&getBlockHeadersData{Origin: hashOrNumber{Number: 0}, Amount: 1},
			[]common.Hash{pm.blockchain.GetBlockByNumber(0).Hash()},
		}, {
			&getBlockHeadersData{Origin: hashOrNumber{Number: pm.blockchain.CurrentBlock().NumberU64()}, Amount: 1},
			[]common.Hash{pm.blockchain.CurrentBlock().Hash()},
		},
//确保遵守协议限制
		{
			&getBlockHeadersData{Origin: hashOrNumber{Number: pm.blockchain.CurrentBlock().NumberU64() - 1}, Amount: limit + 10, Reverse: true},
			pm.blockchain.GetBlockHashesFromHash(pm.blockchain.CurrentBlock().Hash(), limit),
		},
//检查请求是否处理得当
		{
			&getBlockHeadersData{Origin: hashOrNumber{Number: pm.blockchain.CurrentBlock().NumberU64() - 4}, Skip: 3, Amount: 3},
			[]common.Hash{
				pm.blockchain.GetBlockByNumber(pm.blockchain.CurrentBlock().NumberU64() - 4).Hash(),
				pm.blockchain.GetBlockByNumber(pm.blockchain.CurrentBlock().NumberU64()).Hash(),
			},
		}, {
			&getBlockHeadersData{Origin: hashOrNumber{Number: 4}, Skip: 3, Amount: 3, Reverse: true},
			[]common.Hash{
				pm.blockchain.GetBlockByNumber(4).Hash(),
				pm.blockchain.GetBlockByNumber(0).Hash(),
			},
		},
//检查请求是否处理得当，即使中间跳过
		{
			&getBlockHeadersData{Origin: hashOrNumber{Number: pm.blockchain.CurrentBlock().NumberU64() - 4}, Skip: 2, Amount: 3},
			[]common.Hash{
				pm.blockchain.GetBlockByNumber(pm.blockchain.CurrentBlock().NumberU64() - 4).Hash(),
				pm.blockchain.GetBlockByNumber(pm.blockchain.CurrentBlock().NumberU64() - 1).Hash(),
			},
		}, {
			&getBlockHeadersData{Origin: hashOrNumber{Number: 4}, Skip: 2, Amount: 3, Reverse: true},
			[]common.Hash{
				pm.blockchain.GetBlockByNumber(4).Hash(),
				pm.blockchain.GetBlockByNumber(1).Hash(),
			},
		},
//检查一个角情况，在这种情况下，请求更多可以迭代通过端点
		{
			&getBlockHeadersData{Origin: hashOrNumber{Number: 2}, Amount: 5, Reverse: true},
			[]common.Hash{
				pm.blockchain.GetBlockByNumber(2).Hash(),
				pm.blockchain.GetBlockByNumber(1).Hash(),
				pm.blockchain.GetBlockByNumber(0).Hash(),
			},
		},
//检查将溢出循环跳回到链起始处的角情况
		{
			&getBlockHeadersData{Origin: hashOrNumber{Hash: pm.blockchain.GetBlockByNumber(3).Hash()}, Amount: 2, Reverse: false, Skip: math.MaxUint64 - 1},
			[]common.Hash{
				pm.blockchain.GetBlockByNumber(3).Hash(),
			},
		},
//检查跳过溢出循环返回同一头的角情况
		{
			&getBlockHeadersData{Origin: hashOrNumber{Hash: pm.blockchain.GetBlockByNumber(1).Hash()}, Amount: 2, Reverse: false, Skip: math.MaxUint64},
			[]common.Hash{
				pm.blockchain.GetBlockByNumber(1).Hash(),
			},
		},
//检查是否未返回不存在的邮件头
		{
			&getBlockHeadersData{Origin: hashOrNumber{Hash: unknown}, Amount: 1},
			[]common.Hash{},
		}, {
			&getBlockHeadersData{Origin: hashOrNumber{Number: pm.blockchain.CurrentBlock().NumberU64() + 1}, Amount: 1},
			[]common.Hash{},
		},
	}
//运行每个测试并对照链验证结果
	for i, tt := range tests {
//收集响应中预期的头
		headers := []*types.Header{}
		for _, hash := range tt.expect {
			headers = append(headers, pm.blockchain.GetBlockByHash(hash).Header())
		}
//发送哈希请求并验证响应
		p2p.Send(peer.app, 0x03, tt.query)
		if err := p2p.ExpectMsg(peer.app, 0x04, headers); err != nil {
			t.Errorf("test %d: headers mismatch: %v", i, err)
		}
//如果测试使用的是数字源，也可以使用哈希进行重复。
		if tt.query.Origin.Hash == (common.Hash{}) {
			if origin := pm.blockchain.GetBlockByNumber(tt.query.Origin.Number); origin != nil {
				tt.query.Origin.Hash, tt.query.Origin.Number = origin.Hash(), 0

				p2p.Send(peer.app, 0x03, tt.query)
				if err := p2p.ExpectMsg(peer.app, 0x04, headers); err != nil {
					t.Errorf("test %d: headers mismatch: %v", i, err)
				}
			}
		}
	}
}

//可以基于哈希从远程链中检索阻止内容的测试。
func TestGetBlockBodies62(t *testing.T) { testGetBlockBodies(t, 62) }
func TestGetBlockBodies63(t *testing.T) { testGetBlockBodies(t, 63) }

func testGetBlockBodies(t *testing.T, protocol int) {
	pm, _ := newTestProtocolManagerMust(t, downloader.FullSync, downloader.MaxBlockFetch+15, nil, nil)
	peer, _ := newTestPeer("peer", protocol, pm, true)
	defer peer.close()

//为各种方案创建一批测试
	limit := downloader.MaxBlockFetch
	tests := []struct {
random    int           //从链中随机提取的块数
explicit  []common.Hash //显式请求的块
available []bool        //显式请求块的可用性
expected  int           //期望的现有块总数
	}{
{1, nil, nil, 1},             //单个随机块应该是可检索的
{10, nil, nil, 10},           //应可检索多个随机块
{limit, nil, nil, limit},     //最大可能的块应该是可检索的
{limit + 1, nil, nil, limit}, //不应返回超过可能的块计数
{0, []common.Hash{pm.blockchain.Genesis().Hash()}, []bool{true}, 1},      //Genesis区块应该是可回收的。
{0, []common.Hash{pm.blockchain.CurrentBlock().Hash()}, []bool{true}, 1}, //链头滑轮应可回收。
{0, []common.Hash{{}}, []bool{false}, 0},                                 //不应返回不存在的块

//现有和不存在的块交错不应导致问题
		{0, []common.Hash{
			{},
			pm.blockchain.GetBlockByNumber(1).Hash(),
			{},
			pm.blockchain.GetBlockByNumber(10).Hash(),
			{},
			pm.blockchain.GetBlockByNumber(100).Hash(),
			{},
		}, []bool{false, true, false, true, false, true, false}, 3},
	}
//运行每个测试并对照链验证结果
	for i, tt := range tests {
//收集要请求的哈希值和预期的响应
		hashes, seen := []common.Hash{}, make(map[int64]bool)
		bodies := []*blockBody{}

		for j := 0; j < tt.random; j++ {
			for {
				num := rand.Int63n(int64(pm.blockchain.CurrentBlock().NumberU64()))
				if !seen[num] {
					seen[num] = true

					block := pm.blockchain.GetBlockByNumber(uint64(num))
					hashes = append(hashes, block.Hash())
					if len(bodies) < tt.expected {
						bodies = append(bodies, &blockBody{Transactions: block.Transactions(), Uncles: block.Uncles()})
					}
					break
				}
			}
		}
		for j, hash := range tt.explicit {
			hashes = append(hashes, hash)
			if tt.available[j] && len(bodies) < tt.expected {
				block := pm.blockchain.GetBlockByHash(hash)
				bodies = append(bodies, &blockBody{Transactions: block.Transactions(), Uncles: block.Uncles()})
			}
		}
//发送哈希请求并验证响应
		p2p.Send(peer.app, 0x05, hashes)
		if err := p2p.ExpectMsg(peer.app, 0x06, bodies); err != nil {
			t.Errorf("test %d: bodies mismatch: %v", i, err)
		}
	}
}

//测试可以基于哈希检索节点状态数据库。
func TestGetNodeData63(t *testing.T) { testGetNodeData(t, 63) }

func testGetNodeData(t *testing.T, protocol int) {
//定义三个科目以模拟交易
	acc1Key, _ := crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
	acc2Key, _ := crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
	acc1Addr := crypto.PubkeyToAddress(acc1Key.PublicKey)
	acc2Addr := crypto.PubkeyToAddress(acc2Key.PublicKey)

	signer := types.HomesteadSigner{}
//用一些简单的事务创建一个链生成器（公然从@fjl/chain_markets_test中被盗）
	generator := func(i int, block *core.BlockGen) {
		switch i {
		case 0:
//在块1中，测试银行发送帐户1一些乙醚。
			tx, _ := types.SignTx(types.NewTransaction(block.TxNonce(testBank), acc1Addr, big.NewInt(10000), params.TxGas, nil, nil), signer, testBankKey)
			block.AddTx(tx)
		case 1:
//在区块2中，测试银行向账户1发送更多乙醚。
//acc1addr将其传递到account_2。
			tx1, _ := types.SignTx(types.NewTransaction(block.TxNonce(testBank), acc1Addr, big.NewInt(1000), params.TxGas, nil, nil), signer, testBankKey)
			tx2, _ := types.SignTx(types.NewTransaction(block.TxNonce(acc1Addr), acc2Addr, big.NewInt(1000), params.TxGas, nil, nil), signer, acc1Key)
			block.AddTx(tx1)
			block.AddTx(tx2)
		case 2:
//区块3为空，但由账户2开采。
			block.SetCoinbase(acc2Addr)
			block.SetExtra([]byte("yeehaw"))
		case 3:
//块4包括块2和3作为叔叔头（带有修改的额外数据）。
			b2 := block.PrevBlock(1).Header()
			b2.Extra = []byte("foo")
			block.AddUncle(b2)
			b3 := block.PrevBlock(2).Header()
			b3.Extra = []byte("foo")
			block.AddUncle(b3)
		}
	}
//组装测试环境
	pm, db := newTestProtocolManagerMust(t, downloader.FullSync, 4, generator, nil)
	peer, _ := newTestPeer("peer", protocol, pm, true)
	defer peer.close()

//现在获取整个链数据库
	hashes := []common.Hash{}
	for _, key := range db.Keys() {
		if len(key) == len(common.Hash{}) {
			hashes = append(hashes, common.BytesToHash(key))
		}
	}
	p2p.Send(peer.app, 0x0d, hashes)
	msg, err := peer.app.ReadMsg()
	if err != nil {
		t.Fatalf("failed to read node data response: %v", err)
	}
	if msg.Code != 0x0e {
		t.Fatalf("response packet code mismatch: have %x, want %x", msg.Code, 0x0c)
	}
	var data [][]byte
	if err := msg.Decode(&data); err != nil {
		t.Fatalf("failed to decode response node data: %v", err)
	}
//验证所有哈希是否与请求的数据对应，并重建状态树
	for i, want := range hashes {
		if hash := crypto.Keccak256Hash(data[i]); hash != want {
			t.Errorf("data hash mismatch: have %x, want %x", hash, want)
		}
	}
	statedb := ethdb.NewMemDatabase()
	for i := 0; i < len(data); i++ {
		statedb.Put(hashes[i].Bytes(), data[i])
	}
	accounts := []common.Address{testBank, acc1Addr, acc2Addr}
	for i := uint64(0); i <= pm.blockchain.CurrentBlock().NumberU64(); i++ {
		trie, _ := state.New(pm.blockchain.GetBlockByNumber(i).Root(), state.NewDatabase(statedb))

		for j, acc := range accounts {
			state, _ := pm.blockchain.State()
			bw := state.GetBalance(acc)
			bh := trie.GetBalance(acc)

			if (bw != nil && bh == nil) || (bw == nil && bh != nil) {
				t.Errorf("test %d, account %d: balance mismatch: have %v, want %v", i, j, bh, bw)
			}
			if bw != nil && bh != nil && bw.Cmp(bw) != 0 {
				t.Errorf("test %d, account %d: balance mismatch: have %v, want %v", i, j, bh, bw)
			}
		}
	}
}

//测试是否可以基于哈希值检索事务回执。
func TestGetReceipt63(t *testing.T) { testGetReceipt(t, 63) }

func testGetReceipt(t *testing.T, protocol int) {
//定义三个科目以模拟交易
	acc1Key, _ := crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
	acc2Key, _ := crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
	acc1Addr := crypto.PubkeyToAddress(acc1Key.PublicKey)
	acc2Addr := crypto.PubkeyToAddress(acc2Key.PublicKey)

	signer := types.HomesteadSigner{}
//用一些简单的事务创建一个链生成器（公然从@fjl/chain_markets_test中被盗）
	generator := func(i int, block *core.BlockGen) {
		switch i {
		case 0:
//在块1中，测试银行发送帐户1一些乙醚。
			tx, _ := types.SignTx(types.NewTransaction(block.TxNonce(testBank), acc1Addr, big.NewInt(10000), params.TxGas, nil, nil), signer, testBankKey)
			block.AddTx(tx)
		case 1:
//在区块2中，测试银行向账户1发送更多乙醚。
//acc1addr将其传递到account_2。
			tx1, _ := types.SignTx(types.NewTransaction(block.TxNonce(testBank), acc1Addr, big.NewInt(1000), params.TxGas, nil, nil), signer, testBankKey)
			tx2, _ := types.SignTx(types.NewTransaction(block.TxNonce(acc1Addr), acc2Addr, big.NewInt(1000), params.TxGas, nil, nil), signer, acc1Key)
			block.AddTx(tx1)
			block.AddTx(tx2)
		case 2:
//区块3为空，但由账户2开采。
			block.SetCoinbase(acc2Addr)
			block.SetExtra([]byte("yeehaw"))
		case 3:
//块4包括块2和3作为叔叔头（带有修改的额外数据）。
			b2 := block.PrevBlock(1).Header()
			b2.Extra = []byte("foo")
			block.AddUncle(b2)
			b3 := block.PrevBlock(2).Header()
			b3.Extra = []byte("foo")
			block.AddUncle(b3)
		}
	}
//组装测试环境
	pm, _ := newTestProtocolManagerMust(t, downloader.FullSync, 4, generator, nil)
	peer, _ := newTestPeer("peer", protocol, pm, true)
	defer peer.close()

//收集要请求的哈希值和预期的响应
	hashes, receipts := []common.Hash{}, []types.Receipts{}
	for i := uint64(0); i <= pm.blockchain.CurrentBlock().NumberU64(); i++ {
		block := pm.blockchain.GetBlockByNumber(i)

		hashes = append(hashes, block.Hash())
		receipts = append(receipts, pm.blockchain.GetReceiptsByHash(block.Hash()))
	}
//发送哈希请求并验证响应
	p2p.Send(peer.app, 0x0f, hashes)
	if err := p2p.ExpectMsg(peer.app, 0x10, receipts); err != nil {
		t.Errorf("receipts mismatch: %v", err)
	}
}

//发布ETH协议握手的测试，启用DAO分叉的客户端也会执行
//一个DAO“挑战”验证彼此的DAO分叉头，以确保它们处于打开状态
//兼容的链条。
func TestDAOChallengeNoVsNo(t *testing.T)       { testDAOChallenge(t, false, false, false) }
func TestDAOChallengeNoVsPro(t *testing.T)      { testDAOChallenge(t, false, true, false) }
func TestDAOChallengeProVsNo(t *testing.T)      { testDAOChallenge(t, true, false, false) }
func TestDAOChallengeProVsPro(t *testing.T)     { testDAOChallenge(t, true, true, false) }
func TestDAOChallengeNoVsTimeout(t *testing.T)  { testDAOChallenge(t, false, false, true) }
func TestDAOChallengeProVsTimeout(t *testing.T) { testDAOChallenge(t, true, true, true) }

func testDAOChallenge(t *testing.T, localForked, remoteForked bool, timeout bool) {
//减少DAO握手挑战超时
	if timeout {
		defer func(old time.Duration) { daoChallengeTimeout = old }(daoChallengeTimeout)
		daoChallengeTimeout = 500 * time.Millisecond
	}
//创建DAO感知协议管理器
	var (
		evmux   = new(event.TypeMux)
		pow     = ethash.NewFaker()
		db      = ethdb.NewMemDatabase()
		config  = &params.ChainConfig{DAOForkBlock: big.NewInt(1), DAOForkSupport: localForked}
		gspec   = &core.Genesis{Config: config}
		genesis = gspec.MustCommit(db)
	)
	blockchain, err := core.NewBlockChain(db, nil, config, pow, vm.Config{}, nil)
	if err != nil {
		t.Fatalf("failed to create new blockchain: %v", err)
	}
	pm, err := NewProtocolManager(config, downloader.FullSync, DefaultConfig.NetworkId, evmux, new(testTxPool), pow, blockchain, db, nil)
	if err != nil {
		t.Fatalf("failed to start test protocol manager: %v", err)
	}
	pm.Start(1000)
	defer pm.Stop()

//连接新的对等点并检查我们是否收到DAO挑战
	peer, _ := newTestPeer("peer", eth63, pm, true)
	defer peer.close()

	challenge := &getBlockHeadersData{
		Origin:  hashOrNumber{Number: config.DAOForkBlock.Uint64()},
		Amount:  1,
		Skip:    0,
		Reverse: false,
	}
	if err := p2p.ExpectMsg(peer.app, GetBlockHeadersMsg, challenge); err != nil {
		t.Fatalf("challenge mismatch: %v", err)
	}
//如果没有模拟超时，则创建一个块以答复质询
	if !timeout {
		blocks, _ := core.GenerateChain(&params.ChainConfig{}, genesis, ethash.NewFaker(), db, 1, func(i int, block *core.BlockGen) {
			if remoteForked {
				block.SetExtra(params.DAOForkBlockExtra)
			}
		})
		if err := p2p.Send(peer.app, BlockHeadersMsg, []*types.Header{blocks[0].Header()}); err != nil {
			t.Fatalf("failed to answer challenge: %v", err)
		}
time.Sleep(100 * time.Millisecond) //睡眠以避免验证与水滴赛跑
	} else {
//否则，等待测试超时通过
		time.Sleep(daoChallengeTimeout + 500*time.Millisecond)
	}
//验证是否根据分叉端维护或删除远程对等机
	if localForked == remoteForked && !timeout {
		if peers := pm.peers.Len(); peers != 1 {
			t.Fatalf("peer count mismatch: have %d, want %d", peers, 1)
		}
	} else {
		if peers := pm.peers.Len(); peers != 0 {
			t.Fatalf("peer count mismatch: have %d, want %d", peers, 0)
		}
	}
}

func TestBroadcastBlock(t *testing.T) {
	var tests = []struct {
		totalPeers        int
		broadcastExpected int
	}{
		{1, 1},
		{2, 2},
		{3, 3},
		{4, 4},
		{5, 4},
		{9, 4},
		{12, 4},
		{16, 4},
		{26, 5},
		{100, 10},
	}
	for _, test := range tests {
		testBroadcastBlock(t, test.totalPeers, test.broadcastExpected)
	}
}

func testBroadcastBlock(t *testing.T, totalPeers, broadcastExpected int) {
	var (
		evmux   = new(event.TypeMux)
		pow     = ethash.NewFaker()
		db      = ethdb.NewMemDatabase()
		config  = &params.ChainConfig{}
		gspec   = &core.Genesis{Config: config}
		genesis = gspec.MustCommit(db)
	)
	blockchain, err := core.NewBlockChain(db, nil, config, pow, vm.Config{}, nil)
	if err != nil {
		t.Fatalf("failed to create new blockchain: %v", err)
	}
	pm, err := NewProtocolManager(config, downloader.FullSync, DefaultConfig.NetworkId, evmux, new(testTxPool), pow, blockchain, db, nil)
	if err != nil {
		t.Fatalf("failed to start test protocol manager: %v", err)
	}
	pm.Start(1000)
	defer pm.Stop()
	var peers []*testPeer
	for i := 0; i < totalPeers; i++ {
		peer, _ := newTestPeer(fmt.Sprintf("peer %d", i), eth63, pm, true)
		defer peer.close()
		peers = append(peers, peer)
	}
	chain, _ := core.GenerateChain(gspec.Config, genesis, ethash.NewFaker(), db, 1, func(i int, gen *core.BlockGen) {})
 /*广播块（链[0]，真/*传播*/）

 错误：=make（chan错误，totalpeers）
 Donech：=制造（Chan结构，totalpeers）
 对于u，对等：=范围对等
  Go func（P*testpeer）
   如果错误：=p2p.expectmsg（p.app，newblockmsg，&newblockdata block:chain[0]，td:big.newint（131136））；错误！= nIL{
    埃尔奇-厄尔
   }否则{
    Donech<-结构
   }
  }（对等体）
 }
 超时：=time.after（300*time.millisecond）
 var接收计数int
外部：
 对于{
  选择{
  案例错误=<-错误：
   打破外部
  案例：DONECH：
   接收计数+
   如果receivedCount==totalPeers
    打破外部
   }
  案例<超时：
   打破外部
  }
 }
 对于u，对等：=范围对等
  peer.app.close（）。
 }
 如果犯错！= nIL{
  t.errorf（“按对等方匹配块时出错：%v”，err）
 }
 如果收到账单！=需要广播
  t.errorf（“块广播到%d个对等方，应为%d”，ReceivedCount，应为BroadcastExpected）
 }
}
