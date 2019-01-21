
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

package les

import (
	"encoding/binary"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/light"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

func expectResponse(r p2p.MsgReader, msgcode, reqID, bv uint64, data interface{}) error {
	type resp struct {
		ReqID, BV uint64
		Data      interface{}
	}
	return p2p.ExpectMsg(r, msgcode, resp{reqID, bv, data})
}

//可以根据用户查询从远程链中检索阻止头的测试。
func TestGetBlockHeadersLes1(t *testing.T) { testGetBlockHeaders(t, 1) }
func TestGetBlockHeadersLes2(t *testing.T) { testGetBlockHeaders(t, 2) }

func testGetBlockHeaders(t *testing.T, protocol int) {
	server, tearDown := newServerEnv(t, downloader.MaxHashFetch+15, protocol, nil)
	defer tearDown()
	bc := server.pm.blockchain.(*core.BlockChain)

//为测试创建一个“随机”的未知哈希
	var unknown common.Hash
	for i := range unknown {
		unknown[i] = byte(i)
	}
//为各种方案创建一批测试
	limit := uint64(MaxHeaderFetch)
	tests := []struct {
query  *getBlockHeadersData //要为头检索执行的查询
expect []common.Hash        //应为其头的块的哈希
	}{
//单个随机块也应该可以通过哈希和数字检索。
		{
			&getBlockHeadersData{Origin: hashOrNumber{Hash: bc.GetBlockByNumber(limit / 2).Hash()}, Amount: 1},
			[]common.Hash{bc.GetBlockByNumber(limit / 2).Hash()},
		}, {
			&getBlockHeadersData{Origin: hashOrNumber{Number: limit / 2}, Amount: 1},
			[]common.Hash{bc.GetBlockByNumber(limit / 2).Hash()},
		},
//应可从两个方向检索多个邮件头
		{
			&getBlockHeadersData{Origin: hashOrNumber{Number: limit / 2}, Amount: 3},
			[]common.Hash{
				bc.GetBlockByNumber(limit / 2).Hash(),
				bc.GetBlockByNumber(limit/2 + 1).Hash(),
				bc.GetBlockByNumber(limit/2 + 2).Hash(),
			},
		}, {
			&getBlockHeadersData{Origin: hashOrNumber{Number: limit / 2}, Amount: 3, Reverse: true},
			[]common.Hash{
				bc.GetBlockByNumber(limit / 2).Hash(),
				bc.GetBlockByNumber(limit/2 - 1).Hash(),
				bc.GetBlockByNumber(limit/2 - 2).Hash(),
			},
		},
//应检索具有跳过列表的多个邮件头
		{
			&getBlockHeadersData{Origin: hashOrNumber{Number: limit / 2}, Skip: 3, Amount: 3},
			[]common.Hash{
				bc.GetBlockByNumber(limit / 2).Hash(),
				bc.GetBlockByNumber(limit/2 + 4).Hash(),
				bc.GetBlockByNumber(limit/2 + 8).Hash(),
			},
		}, {
			&getBlockHeadersData{Origin: hashOrNumber{Number: limit / 2}, Skip: 3, Amount: 3, Reverse: true},
			[]common.Hash{
				bc.GetBlockByNumber(limit / 2).Hash(),
				bc.GetBlockByNumber(limit/2 - 4).Hash(),
				bc.GetBlockByNumber(limit/2 - 8).Hash(),
			},
		},
//链端点应该是可检索的
		{
			&getBlockHeadersData{Origin: hashOrNumber{Number: 0}, Amount: 1},
			[]common.Hash{bc.GetBlockByNumber(0).Hash()},
		}, {
			&getBlockHeadersData{Origin: hashOrNumber{Number: bc.CurrentBlock().NumberU64()}, Amount: 1},
			[]common.Hash{bc.CurrentBlock().Hash()},
		},
//确保遵守协议限制
  /*
   &getBlockHeadersData_origin:hashornumber_number:bc.currentBlock（）.numberU64（）-1，amount:limit+10，reverse:true，
   bc.getBlockHashesFromHash（bc.currentBlock（）.hash（），限制），
  }*/

//检查请求是否处理得当
		{
			&getBlockHeadersData{Origin: hashOrNumber{Number: bc.CurrentBlock().NumberU64() - 4}, Skip: 3, Amount: 3},
			[]common.Hash{
				bc.GetBlockByNumber(bc.CurrentBlock().NumberU64() - 4).Hash(),
				bc.GetBlockByNumber(bc.CurrentBlock().NumberU64()).Hash(),
			},
		}, {
			&getBlockHeadersData{Origin: hashOrNumber{Number: 4}, Skip: 3, Amount: 3, Reverse: true},
			[]common.Hash{
				bc.GetBlockByNumber(4).Hash(),
				bc.GetBlockByNumber(0).Hash(),
			},
		},
//检查请求是否处理得当，即使中间跳过
		{
			&getBlockHeadersData{Origin: hashOrNumber{Number: bc.CurrentBlock().NumberU64() - 4}, Skip: 2, Amount: 3},
			[]common.Hash{
				bc.GetBlockByNumber(bc.CurrentBlock().NumberU64() - 4).Hash(),
				bc.GetBlockByNumber(bc.CurrentBlock().NumberU64() - 1).Hash(),
			},
		}, {
			&getBlockHeadersData{Origin: hashOrNumber{Number: 4}, Skip: 2, Amount: 3, Reverse: true},
			[]common.Hash{
				bc.GetBlockByNumber(4).Hash(),
				bc.GetBlockByNumber(1).Hash(),
			},
		},
//检查是否未返回不存在的邮件头
		{
			&getBlockHeadersData{Origin: hashOrNumber{Hash: unknown}, Amount: 1},
			[]common.Hash{},
		}, {
			&getBlockHeadersData{Origin: hashOrNumber{Number: bc.CurrentBlock().NumberU64() + 1}, Amount: 1},
			[]common.Hash{},
		},
	}
//运行每个测试并对照链验证结果
	var reqID uint64
	for i, tt := range tests {
//收集响应中预期的头
		headers := []*types.Header{}
		for _, hash := range tt.expect {
			headers = append(headers, bc.GetHeaderByHash(hash))
		}
//发送哈希请求并验证响应
		reqID++
		cost := server.tPeer.GetRequestCost(GetBlockHeadersMsg, int(tt.query.Amount))
		sendRequest(server.tPeer.app, GetBlockHeadersMsg, reqID, cost, tt.query)
		if err := expectResponse(server.tPeer.app, BlockHeadersMsg, reqID, testBufLimit, headers); err != nil {
			t.Errorf("test %d: headers mismatch: %v", i, err)
		}
	}
}

//可以基于哈希从远程链中检索阻止内容的测试。
func TestGetBlockBodiesLes1(t *testing.T) { testGetBlockBodies(t, 1) }
func TestGetBlockBodiesLes2(t *testing.T) { testGetBlockBodies(t, 2) }

func testGetBlockBodies(t *testing.T, protocol int) {
	server, tearDown := newServerEnv(t, downloader.MaxBlockFetch+15, protocol, nil)
	defer tearDown()
	bc := server.pm.blockchain.(*core.BlockChain)

//为各种方案创建一批测试
	limit := MaxBodyFetch
	tests := []struct {
random    int           //从链中随机提取的块数
explicit  []common.Hash //显式请求的块
available []bool        //显式请求块的可用性
expected  int           //期望的现有块总数
	}{
{1, nil, nil, 1},         //单个随机块应该是可检索的
{10, nil, nil, 10},       //应可检索多个随机块
{limit, nil, nil, limit}, //最大可能的块应该是可检索的
//limit+1，nil，nil，limit，//返回的块数不应超过可能的块数
{0, []common.Hash{bc.Genesis().Hash()}, []bool{true}, 1},      //Genesis区块应该是可回收的。
{0, []common.Hash{bc.CurrentBlock().Hash()}, []bool{true}, 1}, //链头滑轮应可回收。
{0, []common.Hash{{}}, []bool{false}, 0},                      //不应返回不存在的块

//现有和不存在的块交错不应导致问题
		{0, []common.Hash{
			{},
			bc.GetBlockByNumber(1).Hash(),
			{},
			bc.GetBlockByNumber(10).Hash(),
			{},
			bc.GetBlockByNumber(100).Hash(),
			{},
		}, []bool{false, true, false, true, false, true, false}, 3},
	}
//运行每个测试并对照链验证结果
	var reqID uint64
	for i, tt := range tests {
//收集要请求的哈希值和预期的响应
		hashes, seen := []common.Hash{}, make(map[int64]bool)
		bodies := []*types.Body{}

		for j := 0; j < tt.random; j++ {
			for {
				num := rand.Int63n(int64(bc.CurrentBlock().NumberU64()))
				if !seen[num] {
					seen[num] = true

					block := bc.GetBlockByNumber(uint64(num))
					hashes = append(hashes, block.Hash())
					if len(bodies) < tt.expected {
						bodies = append(bodies, &types.Body{Transactions: block.Transactions(), Uncles: block.Uncles()})
					}
					break
				}
			}
		}
		for j, hash := range tt.explicit {
			hashes = append(hashes, hash)
			if tt.available[j] && len(bodies) < tt.expected {
				block := bc.GetBlockByHash(hash)
				bodies = append(bodies, &types.Body{Transactions: block.Transactions(), Uncles: block.Uncles()})
			}
		}
		reqID++
//发送哈希请求并验证响应
		cost := server.tPeer.GetRequestCost(GetBlockBodiesMsg, len(hashes))
		sendRequest(server.tPeer.app, GetBlockBodiesMsg, reqID, cost, hashes)
		if err := expectResponse(server.tPeer.app, BlockBodiesMsg, reqID, testBufLimit, bodies); err != nil {
			t.Errorf("test %d: bodies mismatch: %v", i, err)
		}
	}
}

//测试是否可以根据帐户地址检索合同代码。
func TestGetCodeLes1(t *testing.T) { testGetCode(t, 1) }
func TestGetCodeLes2(t *testing.T) { testGetCode(t, 2) }

func testGetCode(t *testing.T, protocol int) {
//组装测试环境
	server, tearDown := newServerEnv(t, 4, protocol, nil)
	defer tearDown()
	bc := server.pm.blockchain.(*core.BlockChain)

	var codereqs []*CodeReq
	var codes [][]byte

	for i := uint64(0); i <= bc.CurrentBlock().NumberU64(); i++ {
		header := bc.GetHeaderByNumber(i)
		req := &CodeReq{
			BHash:  header.Hash(),
			AccKey: crypto.Keccak256(testContractAddr[:]),
		}
		codereqs = append(codereqs, req)
		if i >= testContractDeployed {
			codes = append(codes, testContractCodeDeployed)
		}
	}

	cost := server.tPeer.GetRequestCost(GetCodeMsg, len(codereqs))
	sendRequest(server.tPeer.app, GetCodeMsg, 42, cost, codereqs)
	if err := expectResponse(server.tPeer.app, CodeMsg, 42, testBufLimit, codes); err != nil {
		t.Errorf("codes mismatch: %v", err)
	}
}

//测试是否可以基于哈希值检索事务回执。
func TestGetReceiptLes1(t *testing.T) { testGetReceipt(t, 1) }
func TestGetReceiptLes2(t *testing.T) { testGetReceipt(t, 2) }

func testGetReceipt(t *testing.T, protocol int) {
//组装测试环境
	server, tearDown := newServerEnv(t, 4, protocol, nil)
	defer tearDown()
	bc := server.pm.blockchain.(*core.BlockChain)

//收集要请求的哈希值和预期的响应
	hashes, receipts := []common.Hash{}, []types.Receipts{}
	for i := uint64(0); i <= bc.CurrentBlock().NumberU64(); i++ {
		block := bc.GetBlockByNumber(i)

		hashes = append(hashes, block.Hash())
		receipts = append(receipts, rawdb.ReadReceipts(server.db, block.Hash(), block.NumberU64()))
	}
//发送哈希请求并验证响应
	cost := server.tPeer.GetRequestCost(GetReceiptsMsg, len(hashes))
	sendRequest(server.tPeer.app, GetReceiptsMsg, 42, cost, hashes)
	if err := expectResponse(server.tPeer.app, ReceiptsMsg, 42, testBufLimit, receipts); err != nil {
		t.Errorf("receipts mismatch: %v", err)
	}
}

//可检索检索检索Merkle校对的测试
func TestGetProofsLes1(t *testing.T) { testGetProofs(t, 1) }
func TestGetProofsLes2(t *testing.T) { testGetProofs(t, 2) }

func testGetProofs(t *testing.T, protocol int) {
//组装测试环境
	server, tearDown := newServerEnv(t, 4, protocol, nil)
	defer tearDown()
	bc := server.pm.blockchain.(*core.BlockChain)

	var (
		proofreqs []ProofReq
		proofsV1  [][]rlp.RawValue
	)
	proofsV2 := light.NewNodeSet()

	accounts := []common.Address{testBankAddress, acc1Addr, acc2Addr, {}}
	for i := uint64(0); i <= bc.CurrentBlock().NumberU64(); i++ {
		header := bc.GetHeaderByNumber(i)
		root := header.Root
		trie, _ := trie.New(root, trie.NewDatabase(server.db))

		for _, acc := range accounts {
			req := ProofReq{
				BHash: header.Hash(),
				Key:   crypto.Keccak256(acc[:]),
			}
			proofreqs = append(proofreqs, req)

			switch protocol {
			case 1:
				var proof light.NodeList
				trie.Prove(crypto.Keccak256(acc[:]), 0, &proof)
				proofsV1 = append(proofsV1, proof)
			case 2:
				trie.Prove(crypto.Keccak256(acc[:]), 0, proofsV2)
			}
		}
	}
//发送证明请求并验证响应
	switch protocol {
	case 1:
		cost := server.tPeer.GetRequestCost(GetProofsV1Msg, len(proofreqs))
		sendRequest(server.tPeer.app, GetProofsV1Msg, 42, cost, proofreqs)
		if err := expectResponse(server.tPeer.app, ProofsV1Msg, 42, testBufLimit, proofsV1); err != nil {
			t.Errorf("proofs mismatch: %v", err)
		}
	case 2:
		cost := server.tPeer.GetRequestCost(GetProofsV2Msg, len(proofreqs))
		sendRequest(server.tPeer.app, GetProofsV2Msg, 42, cost, proofreqs)
		if err := expectResponse(server.tPeer.app, ProofsV2Msg, 42, testBufLimit, proofsV2.NodeList()); err != nil {
			t.Errorf("proofs mismatch: %v", err)
		}
	}
}

//能够正确检索CHT证明的测试。
func TestGetCHTProofsLes1(t *testing.T) { testGetCHTProofs(t, 1) }
func TestGetCHTProofsLes2(t *testing.T) { testGetCHTProofs(t, 2) }

func testGetCHTProofs(t *testing.T, protocol int) {
	config := light.TestServerIndexerConfig
	frequency := config.ChtSize
	if protocol == 2 {
		frequency = config.PairChtSize
	}

	waitIndexers := func(cIndexer, bIndexer, btIndexer *core.ChainIndexer) {
		expectSections := frequency / config.ChtSize
		for {
			cs, _, _ := cIndexer.Sections()
			bs, _, _ := bIndexer.Sections()
			if cs >= expectSections && bs >= expectSections {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	server, tearDown := newServerEnv(t, int(frequency+config.ChtConfirms), protocol, waitIndexers)
	defer tearDown()
	bc := server.pm.blockchain.(*core.BlockChain)

//从不同的协议中收集证据
	header := bc.GetHeaderByNumber(frequency - 1)
	rlp, _ := rlp.EncodeToBytes(header)

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, frequency-1)

	proofsV1 := []ChtResp{{
		Header: header,
	}}
	proofsV2 := HelperTrieResps{
		AuxData: [][]byte{rlp},
	}
	switch protocol {
	case 1:
		root := light.GetChtRoot(server.db, 0, bc.GetHeaderByNumber(frequency-1).Hash())
		trie, _ := trie.New(root, trie.NewDatabase(ethdb.NewTable(server.db, light.ChtTablePrefix)))

		var proof light.NodeList
		trie.Prove(key, 0, &proof)
		proofsV1[0].Proof = proof

	case 2:
		root := light.GetChtRoot(server.db, (frequency/config.ChtSize)-1, bc.GetHeaderByNumber(frequency-1).Hash())
		trie, _ := trie.New(root, trie.NewDatabase(ethdb.NewTable(server.db, light.ChtTablePrefix)))
		trie.Prove(key, 0, &proofsV2.Proofs)
	}
//汇编不同协议的请求
	requestsV1 := []ChtReq{{
		ChtNum:   frequency / config.ChtSize,
		BlockNum: frequency - 1,
	}}
	requestsV2 := []HelperTrieReq{{
		Type:    htCanonical,
		TrieIdx: frequency/config.PairChtSize - 1,
		Key:     key,
		AuxReq:  auxHeader,
	}}
//发送证明请求并验证响应
	switch protocol {
	case 1:
		cost := server.tPeer.GetRequestCost(GetHeaderProofsMsg, len(requestsV1))
		sendRequest(server.tPeer.app, GetHeaderProofsMsg, 42, cost, requestsV1)
		if err := expectResponse(server.tPeer.app, HeaderProofsMsg, 42, testBufLimit, proofsV1); err != nil {
			t.Errorf("proofs mismatch: %v", err)
		}
	case 2:
		cost := server.tPeer.GetRequestCost(GetHelperTrieProofsMsg, len(requestsV2))
		sendRequest(server.tPeer.app, GetHelperTrieProofsMsg, 42, cost, requestsV2)
		if err := expectResponse(server.tPeer.app, HelperTrieProofsMsg, 42, testBufLimit, proofsV2); err != nil {
			t.Errorf("proofs mismatch: %v", err)
		}
	}
}

//可以正确检索Bloombits校对的测试。
func TestGetBloombitsProofs(t *testing.T) {
	config := light.TestServerIndexerConfig

	waitIndexers := func(cIndexer, bIndexer, btIndexer *core.ChainIndexer) {
		for {
			cs, _, _ := cIndexer.Sections()
			bs, _, _ := bIndexer.Sections()
			bts, _, _ := btIndexer.Sections()
			if cs >= 8 && bs >= 8 && bts >= 1 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	server, tearDown := newServerEnv(t, int(config.BloomTrieSize+config.BloomTrieConfirms), 2, waitIndexers)
	defer tearDown()
	bc := server.pm.blockchain.(*core.BlockChain)

//请求并验证每个bloom位的证明
	for bit := 0; bit < 2048; bit++ {
//为Bloombits收集请求和证据
		key := make([]byte, 10)

		binary.BigEndian.PutUint16(key[:2], uint16(bit))
//只有第一个Bloom部分有数据。
		binary.BigEndian.PutUint64(key[2:], 0)

		requests := []HelperTrieReq{{
			Type:    htBloomBits,
			TrieIdx: 0,
			Key:     key,
		}}
		var proofs HelperTrieResps

		root := light.GetBloomTrieRoot(server.db, 0, bc.GetHeaderByNumber(config.BloomTrieSize-1).Hash())
		trie, _ := trie.New(root, trie.NewDatabase(ethdb.NewTable(server.db, light.BloomTrieTablePrefix)))
		trie.Prove(key, 0, &proofs.Proofs)

//发送证明请求并验证响应
		cost := server.tPeer.GetRequestCost(GetHelperTrieProofsMsg, len(requests))
		sendRequest(server.tPeer.app, GetHelperTrieProofsMsg, 42, cost, requests)
		if err := expectResponse(server.tPeer.app, HelperTrieProofsMsg, 42, testBufLimit, proofs); err != nil {
			t.Errorf("bit %d: proofs mismatch: %v", bit, err)
		}
	}
}

func TestTransactionStatusLes2(t *testing.T) {
	db := ethdb.NewMemDatabase()
	pm := newTestProtocolManagerMust(t, false, 0, nil, nil, nil, db)
	chain := pm.blockchain.(*core.BlockChain)
	config := core.DefaultTxPoolConfig
	config.Journal = ""
	txpool := core.NewTxPool(config, params.TestChainConfig, chain)
	pm.txpool = txpool
	peer, _ := newTestPeer(t, "peer", 2, pm, true)
	defer peer.close()

	var reqID uint64

	test := func(tx *types.Transaction, send bool, expStatus txStatus) {
		reqID++
		if send {
			cost := peer.GetRequestCost(SendTxV2Msg, 1)
			sendRequest(peer.app, SendTxV2Msg, reqID, cost, types.Transactions{tx})
		} else {
			cost := peer.GetRequestCost(GetTxStatusMsg, 1)
			sendRequest(peer.app, GetTxStatusMsg, reqID, cost, []common.Hash{tx.Hash()})
		}
		if err := expectResponse(peer.app, TxStatusMsg, reqID, testBufLimit, []txStatus{expStatus}); err != nil {
			t.Errorf("transaction status mismatch")
		}
	}

	signer := types.HomesteadSigner{}

//通过发送定价过低的事务来测试错误状态
	tx0, _ := types.SignTx(types.NewTransaction(0, acc1Addr, big.NewInt(10000), params.TxGas, nil, nil), signer, testBankKey)
	test(tx0, true, txStatus{Status: core.TxStatusUnknown, Error: core.ErrUnderpriced.Error()})

	tx1, _ := types.SignTx(types.NewTransaction(0, acc1Addr, big.NewInt(10000), params.TxGas, big.NewInt(100000000000), nil), signer, testBankKey)
test(tx1, false, txStatus{Status: core.TxStatusUnknown}) //发送前查询，应为未知
test(tx1, true, txStatus{Status: core.TxStatusPending})  //发送有效的可处理Tx，应返回挂起
test(tx1, true, txStatus{Status: core.TxStatusPending})  //再次添加不应返回错误

	tx2, _ := types.SignTx(types.NewTransaction(1, acc1Addr, big.NewInt(10000), params.TxGas, big.NewInt(100000000000), nil), signer, testBankKey)
	tx3, _ := types.SignTx(types.NewTransaction(2, acc1Addr, big.NewInt(10000), params.TxGas, big.NewInt(100000000000), nil), signer, testBankKey)
//send transactions in the wrong order, tx3 should be queued
	test(tx3, true, txStatus{Status: core.TxStatusQueued})
	test(tx2, true, txStatus{Status: core.TxStatusPending})
//再次查询，现在tx3也应该挂起
	test(tx3, false, txStatus{Status: core.TxStatusPending})

//生成并添加包含tx1和tx2的块
	gchain, _ := core.GenerateChain(params.TestChainConfig, chain.GetBlockByNumber(0), ethash.NewFaker(), db, 1, func(i int, block *core.BlockGen) {
		block.AddTx(tx1)
		block.AddTx(tx2)
	})
	if _, err := chain.InsertChain(gchain); err != nil {
		panic(err)
	}
//等待txpool处理插入的块
	for i := 0; i < 10; i++ {
		if pending, _ := txpool.Stats(); pending == 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if pending, _ := txpool.Stats(); pending != 1 {
		t.Fatalf("pending count mismatch: have %d, want 1", pending)
	}

//检查他们的状态现在是否包括在内
	block1hash := rawdb.ReadCanonicalHash(db, 1)
	test(tx1, false, txStatus{Status: core.TxStatusIncluded, Lookup: &rawdb.TxLookupEntry{BlockHash: block1hash, BlockIndex: 1, Index: 0}})
	test(tx2, false, txStatus{Status: core.TxStatusIncluded, Lookup: &rawdb.TxLookupEntry{BlockHash: block1hash, BlockIndex: 1, Index: 1}})

//创建一个REORG，将其回滚
	gchain, _ = core.GenerateChain(params.TestChainConfig, chain.GetBlockByNumber(0), ethash.NewFaker(), db, 2, func(i int, block *core.BlockGen) {})
	if _, err := chain.InsertChain(gchain); err != nil {
		panic(err)
	}
//等待txpool处理REORG
	for i := 0; i < 10; i++ {
		if pending, _ := txpool.Stats(); pending == 3 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if pending, _ := txpool.Stats(); pending != 3 {
		t.Fatalf("pending count mismatch: have %d, want 3", pending)
	}
//检查其状态是否再次处于挂起状态
	test(tx1, false, txStatus{Status: core.TxStatusPending})
	test(tx2, false, txStatus{Status: core.TxStatusPending})
}
