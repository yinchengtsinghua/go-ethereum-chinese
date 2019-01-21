
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

//此文件包含一些共享测试功能，多个共享测试功能
//正在测试的不同文件和模块。

package les

import (
	"crypto/rand"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/les/flowcontrol"
	"github.com/ethereum/go-ethereum/light"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
)

var (
	testBankKey, _  = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	testBankAddress = crypto.PubkeyToAddress(testBankKey.PublicKey)
	testBankFunds   = big.NewInt(1000000000000000000)

	acc1Key, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
	acc2Key, _ = crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
	acc1Addr   = crypto.PubkeyToAddress(acc1Key.PublicKey)
	acc2Addr   = crypto.PubkeyToAddress(acc2Key.PublicKey)

	testContractCode         = common.Hex2Bytes("606060405260cc8060106000396000f360606040526000357c01000000000000000000000000000000000000000000000000000000009004806360cd2685146041578063c16431b914606b57603f565b005b6055600480803590602001909190505060a9565b6040518082815260200191505060405180910390f35b60886004808035906020019091908035906020019091905050608a565b005b80600060005083606481101560025790900160005b50819055505b5050565b6000600060005082606481101560025790900160005b5054905060c7565b91905056")
	testContractAddr         common.Address
	testContractCodeDeployed = testContractCode[16:]
	testContractDeployed     = uint64(2)

	testEventEmitterCode = common.Hex2Bytes("60606040523415600e57600080fd5b7f57050ab73f6b9ebdd9f76b8d4997793f48cf956e965ee070551b9ca0bb71584e60405160405180910390a160358060476000396000f3006060604052600080fd00a165627a7a723058203f727efcad8b5811f8cb1fc2620ce5e8c63570d697aef968172de296ea3994140029")
	testEventEmitterAddr common.Address

	testBufLimit = uint64(100)
)

/*
合同测试

    uint256[100]数据；

    函数Put（uint256 addr，uint256 value）
        data[addr]=值；
    }

    函数get（uint256 addr）常量返回（uint256 value）
        返回数据[地址]；
    }
}
**/


func testChainGen(i int, block *core.BlockGen) {
	signer := types.HomesteadSigner{}

	switch i {
	case 0:
//在块1中，测试银行发送帐户1一些乙醚。
		tx, _ := types.SignTx(types.NewTransaction(block.TxNonce(testBankAddress), acc1Addr, big.NewInt(10000), params.TxGas, nil, nil), signer, testBankKey)
		block.AddTx(tx)
	case 1:
//在区块2中，测试银行向账户1发送更多乙醚。
//acc1addr将其传递到account_2。
//acc1addr创建一个测试合同。
//acc1addr创建一个测试事件。
		nonce := block.TxNonce(acc1Addr)

		tx1, _ := types.SignTx(types.NewTransaction(block.TxNonce(testBankAddress), acc1Addr, big.NewInt(1000), params.TxGas, nil, nil), signer, testBankKey)
		tx2, _ := types.SignTx(types.NewTransaction(nonce, acc2Addr, big.NewInt(1000), params.TxGas, nil, nil), signer, acc1Key)
		tx3, _ := types.SignTx(types.NewContractCreation(nonce+1, big.NewInt(0), 200000, big.NewInt(0), testContractCode), signer, acc1Key)
		testContractAddr = crypto.CreateAddress(acc1Addr, nonce+1)
		tx4, _ := types.SignTx(types.NewContractCreation(nonce+2, big.NewInt(0), 200000, big.NewInt(0), testEventEmitterCode), signer, acc1Key)
		testEventEmitterAddr = crypto.CreateAddress(acc1Addr, nonce+2)
		block.AddTx(tx1)
		block.AddTx(tx2)
		block.AddTx(tx3)
		block.AddTx(tx4)
	case 2:
//区块3为空，但由账户2开采。
		block.SetCoinbase(acc2Addr)
		block.SetExtra([]byte("yeehaw"))
		data := common.Hex2Bytes("C16431B900000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001")
		tx, _ := types.SignTx(types.NewTransaction(block.TxNonce(testBankAddress), testContractAddr, big.NewInt(0), 100000, nil, data), signer, testBankKey)
		block.AddTx(tx)
	case 3:
//块4包括块2和3作为叔叔头（带有修改的额外数据）。
		b2 := block.PrevBlock(1).Header()
		b2.Extra = []byte("foo")
		block.AddUncle(b2)
		b3 := block.PrevBlock(2).Header()
		b3.Extra = []byte("foo")
		block.AddUncle(b3)
		data := common.Hex2Bytes("C16431B900000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000002")
		tx, _ := types.SignTx(types.NewTransaction(block.TxNonce(testBankAddress), testContractAddr, big.NewInt(0), 100000, nil, data), signer, testBankKey)
		block.AddTx(tx)
	}
}

//testindexers为测试目的创建一组具有指定参数的索引器。
func testIndexers(db ethdb.Database, odr light.OdrBackend, iConfig *light.IndexerConfig) (*core.ChainIndexer, *core.ChainIndexer, *core.ChainIndexer) {
	chtIndexer := light.NewChtIndexer(db, odr, iConfig.ChtSize, iConfig.ChtConfirms)
	bloomIndexer := eth.NewBloomIndexer(db, iConfig.BloomSize, iConfig.BloomConfirms)
	bloomTrieIndexer := light.NewBloomTrieIndexer(db, odr, iConfig.BloomSize, iConfig.BloomTrieSize)
	bloomIndexer.AddChildIndexer(bloomTrieIndexer)
	return chtIndexer, bloomIndexer, bloomTrieIndexer
}

func testRCL() RequestCostList {
	cl := make(RequestCostList, len(reqList))
	for i, code := range reqList {
		cl[i].MsgCode = code
		cl[i].BaseCost = 0
		cl[i].ReqCost = 0
	}
	return cl
}

//NewTestProtocolManager为测试目的创建了一个新的协议管理器，
//已知给定的块数，潜在的通知
//用于不同事件和相对链索引器数组的通道。
func newTestProtocolManager(lightSync bool, blocks int, generator func(int, *core.BlockGen), odr *LesOdr, peers *peerSet, db ethdb.Database) (*ProtocolManager, error) {
	var (
		evmux  = new(event.TypeMux)
		engine = ethash.NewFaker()
		gspec  = core.Genesis{
			Config: params.TestChainConfig,
			Alloc:  core.GenesisAlloc{testBankAddress: {Balance: testBankFunds}},
		}
		genesis = gspec.MustCommit(db)
		chain   BlockChain
	)
	if peers == nil {
		peers = newPeerSet()
	}

	if lightSync {
		chain, _ = light.NewLightChain(odr, gspec.Config, engine)
	} else {
		blockchain, _ := core.NewBlockChain(db, nil, gspec.Config, engine, vm.Config{}, nil)
		gchain, _ := core.GenerateChain(gspec.Config, genesis, ethash.NewFaker(), db, blocks, generator)
		if _, err := blockchain.InsertChain(gchain); err != nil {
			panic(err)
		}
		chain = blockchain
	}

	indexConfig := light.TestServerIndexerConfig
	if lightSync {
		indexConfig = light.TestClientIndexerConfig
	}
	pm, err := NewProtocolManager(gspec.Config, indexConfig, lightSync, NetworkId, evmux, engine, peers, chain, nil, db, odr, nil, nil, make(chan struct{}), new(sync.WaitGroup))
	if err != nil {
		return nil, err
	}
	if !lightSync {
		srv := &LesServer{lesCommons: lesCommons{protocolManager: pm}}
		pm.server = srv

		srv.defParams = &flowcontrol.ServerParams{
			BufLimit:    testBufLimit,
			MinRecharge: 1,
		}

		srv.fcManager = flowcontrol.NewClientManager(50, 10, 1000000000)
		srv.fcCostStats = newCostStats(nil)
	}
	pm.Start(1000)
	return pm, nil
}

//newTestProtocolManager必须为测试目的创建新的协议管理器，
//已知给定的块数，潜在的通知
//用于不同事件和相对链索引器数组的通道。如果出现错误，构造函数强制-
//测试失败。
func newTestProtocolManagerMust(t *testing.T, lightSync bool, blocks int, generator func(int, *core.BlockGen), odr *LesOdr, peers *peerSet, db ethdb.Database) *ProtocolManager {
	pm, err := newTestProtocolManager(lightSync, blocks, generator, odr, peers, db)
	if err != nil {
		t.Fatalf("Failed to create protocol manager: %v", err)
	}
	return pm
}

//testpeer是允许测试直接网络调用的模拟对等机。
type testPeer struct {
net p2p.MsgReadWriter //模拟远程消息传递的网络层读写器
app *p2p.MsgPipeRW    //应用层读写器模拟本地端
	*peer
}

//newtestpeer创建在给定的协议管理器上注册的新对等。
func newTestPeer(t *testing.T, name string, version int, pm *ProtocolManager, shake bool) (*testPeer, <-chan error) {
//创建消息管道以通过
	app, net := p2p.MsgPipe()

//生成随机ID并创建对等机
	var id enode.ID
	rand.Read(id[:])

	peer := pm.newPeer(version, NetworkId, p2p.NewPeer(id, name, nil), net)

//在新线程上启动对等机
	errc := make(chan error, 1)
	go func() {
		select {
		case pm.newPeerCh <- peer:
			errc <- pm.handle(peer)
		case <-pm.quitSync:
			errc <- p2p.DiscQuitting
		}
	}()
	tp := &testPeer{
		app:  app,
		net:  net,
		peer: peer,
	}
//执行任何隐式请求的握手并返回
	if shake {
		var (
			genesis = pm.blockchain.Genesis()
			head    = pm.blockchain.CurrentHeader()
			td      = pm.blockchain.GetTd(head.Hash(), head.Number.Uint64())
		)
		tp.handshake(t, td, head.Hash(), head.Number.Uint64(), genesis.Hash())
	}
	return tp, errc
}

func newTestPeerPair(name string, version int, pm, pm2 *ProtocolManager) (*peer, <-chan error, *peer, <-chan error) {
//创建消息管道以通过
	app, net := p2p.MsgPipe()

//生成随机ID并创建对等机
	var id enode.ID
	rand.Read(id[:])

	peer := pm.newPeer(version, NetworkId, p2p.NewPeer(id, name, nil), net)
	peer2 := pm2.newPeer(version, NetworkId, p2p.NewPeer(id, name, nil), app)

//在新线程上启动对等机
	errc := make(chan error, 1)
	errc2 := make(chan error, 1)
	go func() {
		select {
		case pm.newPeerCh <- peer:
			errc <- pm.handle(peer)
		case <-pm.quitSync:
			errc <- p2p.DiscQuitting
		}
	}()
	go func() {
		select {
		case pm2.newPeerCh <- peer2:
			errc2 <- pm2.handle(peer2)
		case <-pm2.quitSync:
			errc2 <- p2p.DiscQuitting
		}
	}()
	return peer, errc, peer2, errc2
}

//握手模拟一个简单的握手，它期望
//我们在本地模拟的远程端。
func (p *testPeer) handshake(t *testing.T, td *big.Int, head common.Hash, headNum uint64, genesis common.Hash) {
	var expList keyValueList
	expList = expList.add("protocolVersion", uint64(p.version))
	expList = expList.add("networkId", uint64(NetworkId))
	expList = expList.add("headTd", td)
	expList = expList.add("headHash", head)
	expList = expList.add("headNum", headNum)
	expList = expList.add("genesisHash", genesis)
	sendList := make(keyValueList, len(expList))
	copy(sendList, expList)
	expList = expList.add("serveHeaders", nil)
	expList = expList.add("serveChainSince", uint64(0))
	expList = expList.add("serveStateSince", uint64(0))
	expList = expList.add("txRelay", nil)
	expList = expList.add("flowControl/BL", testBufLimit)
	expList = expList.add("flowControl/MRR", uint64(1))
	expList = expList.add("flowControl/MRC", testRCL())

	if err := p2p.ExpectMsg(p.app, StatusMsg, expList); err != nil {
		t.Fatalf("status recv: %v", err)
	}
	if err := p2p.Send(p.app, StatusMsg, sendList); err != nil {
		t.Fatalf("status send: %v", err)
	}

	p.fcServerParams = &flowcontrol.ServerParams{
		BufLimit:    testBufLimit,
		MinRecharge: 1,
	}
}

//CLOSE终止对等端的本地端，通知远程协议
//终止经理。
func (p *testPeer) close() {
	p.app.Close()
}

//TestEntity表示使用必要辅助字段进行测试的网络实体。
type TestEntity struct {
	db    ethdb.Database
	rPeer *peer
	tPeer *testPeer
	peers *peerSet
	pm    *ProtocolManager
//索引器
	chtIndexer       *core.ChainIndexer
	bloomIndexer     *core.ChainIndexer
	bloomTrieIndexer *core.ChainIndexer
}

//NewServerEnv创建了一个服务器测试环境，其中连接了用于测试的测试对等端。
func newServerEnv(t *testing.T, blocks int, protocol int, waitIndexers func(*core.ChainIndexer, *core.ChainIndexer, *core.ChainIndexer)) (*TestEntity, func()) {
	db := ethdb.NewMemDatabase()
	cIndexer, bIndexer, btIndexer := testIndexers(db, nil, light.TestServerIndexerConfig)

	pm := newTestProtocolManagerMust(t, false, blocks, testChainGen, nil, nil, db)
	peer, _ := newTestPeer(t, "peer", protocol, pm, true)

	cIndexer.Start(pm.blockchain.(*core.BlockChain))
	bIndexer.Start(pm.blockchain.(*core.BlockChain))

//等待索引器生成足够的索引数据。
	if waitIndexers != nil {
		waitIndexers(cIndexer, bIndexer, btIndexer)
	}

	return &TestEntity{
			db:               db,
			tPeer:            peer,
			pm:               pm,
			chtIndexer:       cIndexer,
			bloomIndexer:     bIndexer,
			bloomTrieIndexer: btIndexer,
		}, func() {
			peer.close()
//注：BloomTrie索引器将由其父级递归关闭。
			cIndexer.Close()
			bIndexer.Close()
		}
}

//newclientserverenv使用连接的LES服务器和轻型客户端对创建客户机/服务器架构环境
//用于测试。
func newClientServerEnv(t *testing.T, blocks int, protocol int, waitIndexers func(*core.ChainIndexer, *core.ChainIndexer, *core.ChainIndexer), newPeer bool) (*TestEntity, *TestEntity, func()) {
	db, ldb := ethdb.NewMemDatabase(), ethdb.NewMemDatabase()
	peers, lPeers := newPeerSet(), newPeerSet()

	dist := newRequestDistributor(lPeers, make(chan struct{}))
	rm := newRetrieveManager(lPeers, dist, nil)
	odr := NewLesOdr(ldb, light.TestClientIndexerConfig, rm)

	cIndexer, bIndexer, btIndexer := testIndexers(db, nil, light.TestServerIndexerConfig)
	lcIndexer, lbIndexer, lbtIndexer := testIndexers(ldb, odr, light.TestClientIndexerConfig)
	odr.SetIndexers(lcIndexer, lbtIndexer, lbIndexer)

	pm := newTestProtocolManagerMust(t, false, blocks, testChainGen, nil, peers, db)
	lpm := newTestProtocolManagerMust(t, true, 0, nil, odr, lPeers, ldb)

	startIndexers := func(clientMode bool, pm *ProtocolManager) {
		if clientMode {
			lcIndexer.Start(pm.blockchain.(*light.LightChain))
			lbIndexer.Start(pm.blockchain.(*light.LightChain))
		} else {
			cIndexer.Start(pm.blockchain.(*core.BlockChain))
			bIndexer.Start(pm.blockchain.(*core.BlockChain))
		}
	}

	startIndexers(false, pm)
	startIndexers(true, lpm)

//如果指定了函数，则执行wait-until函数。
	if waitIndexers != nil {
		waitIndexers(cIndexer, bIndexer, btIndexer)
	}

	var (
		peer, lPeer *peer
		err1, err2  <-chan error
	)
	if newPeer {
		peer, err1, lPeer, err2 = newTestPeerPair("peer", protocol, pm, lpm)
		select {
		case <-time.After(time.Millisecond * 100):
		case err := <-err1:
			t.Fatalf("peer 1 handshake error: %v", err)
		case err := <-err2:
			t.Fatalf("peer 2 handshake error: %v", err)
		}
	}

	return &TestEntity{
			db:               db,
			pm:               pm,
			rPeer:            peer,
			peers:            peers,
			chtIndexer:       cIndexer,
			bloomIndexer:     bIndexer,
			bloomTrieIndexer: btIndexer,
		}, &TestEntity{
			db:               ldb,
			pm:               lpm,
			rPeer:            lPeer,
			peers:            lPeers,
			chtIndexer:       lcIndexer,
			bloomIndexer:     lbIndexer,
			bloomTrieIndexer: lbtIndexer,
		}, func() {
//Note bloom trie indexers will be closed by their parents recursively.
			cIndexer.Close()
			bIndexer.Close()
			lcIndexer.Close()
			lbIndexer.Close()
		}
}
