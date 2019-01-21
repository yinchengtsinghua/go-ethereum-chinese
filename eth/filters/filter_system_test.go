
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

package filters

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"reflect"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

type testBackend struct {
	mux        *event.TypeMux
	db         ethdb.Database
	sections   uint64
	txFeed     *event.Feed
	rmLogsFeed *event.Feed
	logsFeed   *event.Feed
	chainFeed  *event.Feed
}

func (b *testBackend) ChainDb() ethdb.Database {
	return b.db
}

func (b *testBackend) EventMux() *event.TypeMux {
	return b.mux
}

func (b *testBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	var (
		hash common.Hash
		num  uint64
	)
	if blockNr == rpc.LatestBlockNumber {
		hash = rawdb.ReadHeadBlockHash(b.db)
		number := rawdb.ReadHeaderNumber(b.db, hash)
		if number == nil {
			return nil, nil
		}
		num = *number
	} else {
		num = uint64(blockNr)
		hash = rawdb.ReadCanonicalHash(b.db, num)
	}
	return rawdb.ReadHeader(b.db, hash, num), nil
}

func (b *testBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	number := rawdb.ReadHeaderNumber(b.db, hash)
	if number == nil {
		return nil, nil
	}
	return rawdb.ReadHeader(b.db, hash, *number), nil
}

func (b *testBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	if number := rawdb.ReadHeaderNumber(b.db, hash); number != nil {
		return rawdb.ReadReceipts(b.db, hash, *number), nil
	}
	return nil, nil
}

func (b *testBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	number := rawdb.ReadHeaderNumber(b.db, hash)
	if number == nil {
		return nil, nil
	}
	receipts := rawdb.ReadReceipts(b.db, hash, *number)

	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

func (b *testBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.txFeed.Subscribe(ch)
}

func (b *testBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.rmLogsFeed.Subscribe(ch)
}

func (b *testBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.logsFeed.Subscribe(ch)
}

func (b *testBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.chainFeed.Subscribe(ch)
}

func (b *testBackend) BloomStatus() (uint64, uint64) {
	return params.BloomBitsBlocks, b.sections
}

func (b *testBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	requests := make(chan chan *bloombits.Retrieval)

	go session.Multiplex(16, 0, requests)
	go func() {
		for {
//等待服务请求或关闭
			select {
			case <-ctx.Done():
				return

			case request := <-requests:
				task := <-request

				task.Bitsets = make([][]byte, len(task.Sections))
				for i, section := range task.Sections {
if rand.Int()%4 != 0 { //处理偶尔丢失的交货
						head := rawdb.ReadCanonicalHash(b.db, (section+1)*params.BloomBitsBlocks-1)
						task.Bitsets[i], _ = rawdb.ReadBloomBits(b.db, task.Bit, section, head)
					}
				}
				request <- task
			}
		}
	}()
}

//testblocksubscription测试块订阅是否返回已发布链事件的块哈希。
//它创建多个订阅：
//-一个在开始处，应接收所有已发布的链事件和一秒钟（blockhash）
//-在切断力矩后创建，在第二个切断力矩后卸载（blockhashes[cutoff1:cutoff2]）
//-在第二个截止力矩后创建的一个（blockhashes[cutoff2:]
func TestBlockSubscription(t *testing.T) {
	t.Parallel()

	var (
		mux         = new(event.TypeMux)
		db          = ethdb.NewMemDatabase()
		txFeed      = new(event.Feed)
		rmLogsFeed  = new(event.Feed)
		logsFeed    = new(event.Feed)
		chainFeed   = new(event.Feed)
		backend     = &testBackend{mux, db, 0, txFeed, rmLogsFeed, logsFeed, chainFeed}
		api         = NewPublicFilterAPI(backend, false)
		genesis     = new(core.Genesis).MustCommit(db)
		chain, _    = core.GenerateChain(params.TestChainConfig, genesis, ethash.NewFaker(), db, 10, func(i int, gen *core.BlockGen) {})
		chainEvents = []core.ChainEvent{}
	)

	for _, blk := range chain {
		chainEvents = append(chainEvents, core.ChainEvent{Hash: blk.Hash(), Block: blk})
	}

	chan0 := make(chan *types.Header)
	sub0 := api.events.SubscribeNewHeads(chan0)
	chan1 := make(chan *types.Header)
	sub1 := api.events.SubscribeNewHeads(chan1)

go func() { //模拟客户端
		i1, i2 := 0, 0
		for i1 != len(chainEvents) || i2 != len(chainEvents) {
			select {
			case header := <-chan0:
				if chainEvents[i1].Hash != header.Hash() {
					t.Errorf("sub0 received invalid hash on index %d, want %x, got %x", i1, chainEvents[i1].Hash, header.Hash())
				}
				i1++
			case header := <-chan1:
				if chainEvents[i2].Hash != header.Hash() {
					t.Errorf("sub1 received invalid hash on index %d, want %x, got %x", i2, chainEvents[i2].Hash, header.Hash())
				}
				i2++
			}
		}

		sub0.Unsubscribe()
		sub1.Unsubscribe()
	}()

	time.Sleep(1 * time.Second)
	for _, e := range chainEvents {
		chainFeed.Send(e)
	}

	<-sub0.Err()
	<-sub1.Err()
}

//testpendingtxfilter测试挂起的tx筛选器是否检索发布到事件mux的所有挂起事务。
func TestPendingTxFilter(t *testing.T) {
	t.Parallel()

	var (
		mux        = new(event.TypeMux)
		db         = ethdb.NewMemDatabase()
		txFeed     = new(event.Feed)
		rmLogsFeed = new(event.Feed)
		logsFeed   = new(event.Feed)
		chainFeed  = new(event.Feed)
		backend    = &testBackend{mux, db, 0, txFeed, rmLogsFeed, logsFeed, chainFeed}
		api        = NewPublicFilterAPI(backend, false)

		transactions = []*types.Transaction{
			types.NewTransaction(0, common.HexToAddress("0xb794f5ea0ba39494ce83a213fffba74279579268"), new(big.Int), 0, new(big.Int), nil),
			types.NewTransaction(1, common.HexToAddress("0xb794f5ea0ba39494ce83a213fffba74279579268"), new(big.Int), 0, new(big.Int), nil),
			types.NewTransaction(2, common.HexToAddress("0xb794f5ea0ba39494ce83a213fffba74279579268"), new(big.Int), 0, new(big.Int), nil),
			types.NewTransaction(3, common.HexToAddress("0xb794f5ea0ba39494ce83a213fffba74279579268"), new(big.Int), 0, new(big.Int), nil),
			types.NewTransaction(4, common.HexToAddress("0xb794f5ea0ba39494ce83a213fffba74279579268"), new(big.Int), 0, new(big.Int), nil),
		}

		hashes []common.Hash
	)

	fid0 := api.NewPendingTransactionFilter()

	time.Sleep(1 * time.Second)
	txFeed.Send(core.NewTxsEvent{Txs: transactions})

	timeout := time.Now().Add(1 * time.Second)
	for {
		results, err := api.GetFilterChanges(fid0)
		if err != nil {
			t.Fatalf("Unable to retrieve logs: %v", err)
		}

		h := results.([]common.Hash)
		hashes = append(hashes, h...)
		if len(hashes) >= len(transactions) {
			break
		}
//检查超时
		if time.Now().After(timeout) {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	if len(hashes) != len(transactions) {
		t.Errorf("invalid number of transactions, want %d transactions(s), got %d", len(transactions), len(hashes))
		return
	}
	for i := range hashes {
		if hashes[i] != transactions[i].Hash() {
			t.Errorf("hashes[%d] invalid, want %x, got %x", i, transactions[i].Hash(), hashes[i])
		}
	}
}

//testlogfiltercreation测试给定的筛选条件是否有意义。
//否则，它必须返回一个错误。
func TestLogFilterCreation(t *testing.T) {
	var (
		mux        = new(event.TypeMux)
		db         = ethdb.NewMemDatabase()
		txFeed     = new(event.Feed)
		rmLogsFeed = new(event.Feed)
		logsFeed   = new(event.Feed)
		chainFeed  = new(event.Feed)
		backend    = &testBackend{mux, db, 0, txFeed, rmLogsFeed, logsFeed, chainFeed}
		api        = NewPublicFilterAPI(backend, false)

		testCases = []struct {
			crit    FilterCriteria
			success bool
		}{
//默认值
			{FilterCriteria{}, true},
//有效的块编号范围
			{FilterCriteria{FromBlock: big.NewInt(1), ToBlock: big.NewInt(2)}, true},
//“已开采”区块范围待定
			{FilterCriteria{FromBlock: big.NewInt(1), ToBlock: big.NewInt(rpc.LatestBlockNumber.Int64())}, true},
//新建已开采和待定区块
			{FilterCriteria{FromBlock: big.NewInt(rpc.LatestBlockNumber.Int64()), ToBlock: big.NewInt(rpc.PendingBlockNumber.Int64())}, true},
//从“高”区到“高”区
			{FilterCriteria{FromBlock: big.NewInt(2), ToBlock: big.NewInt(1)}, false},
//从“高”区到“高”区
			{FilterCriteria{FromBlock: big.NewInt(rpc.LatestBlockNumber.Int64()), ToBlock: big.NewInt(100)}, false},
//从“高”区到“高”区
			{FilterCriteria{FromBlock: big.NewInt(rpc.PendingBlockNumber.Int64()), ToBlock: big.NewInt(100)}, false},
//从“高”区到“高”区
			{FilterCriteria{FromBlock: big.NewInt(rpc.PendingBlockNumber.Int64()), ToBlock: big.NewInt(rpc.LatestBlockNumber.Int64())}, false},
		}
	)

	for i, test := range testCases {
		_, err := api.NewFilter(test.crit)
		if test.success && err != nil {
			t.Errorf("expected filter creation for case %d to success, got %v", i, err)
		}
		if !test.success && err == nil {
			t.Errorf("expected testcase %d to fail with an error", i)
		}
	}
}

//TestInvalidLogFilterCreation测试无效的筛选器日志条件是否导致错误
//创建筛选器时。
func TestInvalidLogFilterCreation(t *testing.T) {
	t.Parallel()

	var (
		mux        = new(event.TypeMux)
		db         = ethdb.NewMemDatabase()
		txFeed     = new(event.Feed)
		rmLogsFeed = new(event.Feed)
		logsFeed   = new(event.Feed)
		chainFeed  = new(event.Feed)
		backend    = &testBackend{mux, db, 0, txFeed, rmLogsFeed, logsFeed, chainFeed}
		api        = NewPublicFilterAPI(backend, false)
	)

//日志过滤器创建失败的不同情况。
//原因：FromBlock>ToBlock
	testCases := []FilterCriteria{
		0: {FromBlock: big.NewInt(rpc.PendingBlockNumber.Int64()), ToBlock: big.NewInt(rpc.LatestBlockNumber.Int64())},
		1: {FromBlock: big.NewInt(rpc.PendingBlockNumber.Int64()), ToBlock: big.NewInt(100)},
		2: {FromBlock: big.NewInt(rpc.LatestBlockNumber.Int64()), ToBlock: big.NewInt(100)},
	}

	for i, test := range testCases {
		if _, err := api.NewFilter(test); err == nil {
			t.Errorf("Expected NewFilter for case #%d to fail", i)
		}
	}
}

func TestInvalidGetLogsRequest(t *testing.T) {
	var (
		mux        = new(event.TypeMux)
		db         = ethdb.NewMemDatabase()
		txFeed     = new(event.Feed)
		rmLogsFeed = new(event.Feed)
		logsFeed   = new(event.Feed)
		chainFeed  = new(event.Feed)
		backend    = &testBackend{mux, db, 0, txFeed, rmLogsFeed, logsFeed, chainFeed}
		api        = NewPublicFilterAPI(backend, false)
		blockHash  = common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	)

//原因：不能同时指定blockhash和fromblock/toblock）
	testCases := []FilterCriteria{
		0: {BlockHash: &blockHash, FromBlock: big.NewInt(100)},
		1: {BlockHash: &blockHash, ToBlock: big.NewInt(500)},
		2: {BlockHash: &blockHash, FromBlock: big.NewInt(rpc.LatestBlockNumber.Int64())},
	}

	for i, test := range testCases {
		if _, err := api.GetLogs(context.Background(), test); err == nil {
			t.Errorf("Expected Logs for case #%d to fail", i)
		}
	}
}

//testlogfilter测试日志筛选器是否匹配发布到事件馈送的正确日志。
func TestLogFilter(t *testing.T) {
	t.Parallel()

	var (
		mux        = new(event.TypeMux)
		db         = ethdb.NewMemDatabase()
		txFeed     = new(event.Feed)
		rmLogsFeed = new(event.Feed)
		logsFeed   = new(event.Feed)
		chainFeed  = new(event.Feed)
		backend    = &testBackend{mux, db, 0, txFeed, rmLogsFeed, logsFeed, chainFeed}
		api        = NewPublicFilterAPI(backend, false)

		firstAddr      = common.HexToAddress("0x1111111111111111111111111111111111111111")
		secondAddr     = common.HexToAddress("0x2222222222222222222222222222222222222222")
		thirdAddress   = common.HexToAddress("0x3333333333333333333333333333333333333333")
		notUsedAddress = common.HexToAddress("0x9999999999999999999999999999999999999999")
		firstTopic     = common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
		secondTopic    = common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222")
		notUsedTopic   = common.HexToHash("0x9999999999999999999999999999999999999999999999999999999999999999")

//发布两次，一次作为vm.logs，一次作为core.pendinglogsevent
		allLogs = []*types.Log{
			{Address: firstAddr},
			{Address: firstAddr, Topics: []common.Hash{firstTopic}, BlockNumber: 1},
			{Address: secondAddr, Topics: []common.Hash{firstTopic}, BlockNumber: 1},
			{Address: thirdAddress, Topics: []common.Hash{secondTopic}, BlockNumber: 2},
			{Address: thirdAddress, Topics: []common.Hash{secondTopic}, BlockNumber: 3},
		}

		expectedCase7  = []*types.Log{allLogs[3], allLogs[4], allLogs[0], allLogs[1], allLogs[2], allLogs[3], allLogs[4]}
		expectedCase11 = []*types.Log{allLogs[1], allLogs[2], allLogs[1], allLogs[2]}

		testCases = []struct {
			crit     FilterCriteria
			expected []*types.Log
			id       rpc.ID
		}{
//全部匹配
			0: {FilterCriteria{}, allLogs, ""},
//由于没有匹配的地址而不匹配
			1: {FilterCriteria{Addresses: []common.Address{{}, notUsedAddress}, Topics: [][]common.Hash{nil}}, []*types.Log{}, ""},
//根据地址匹配日志，忽略主题
			2: {FilterCriteria{Addresses: []common.Address{firstAddr}}, allLogs[:2], ""},
//由于没有匹配的主题（与地址匹配），因此不匹配
			3: {FilterCriteria{Addresses: []common.Address{secondAddr}, Topics: [][]common.Hash{{notUsedTopic}}}, []*types.Log{}, ""},
//根据地址和主题匹配日志
			4: {FilterCriteria{Addresses: []common.Address{thirdAddress}, Topics: [][]common.Hash{{firstTopic, secondTopic}}}, allLogs[3:5], ""},
//基于多个地址和“或”主题匹配日志
			5: {FilterCriteria{Addresses: []common.Address{secondAddr, thirdAddress}, Topics: [][]common.Hash{{firstTopic, secondTopic}}}, allLogs[2:5], ""},
//在挂起块中登录
			6: {FilterCriteria{Addresses: []common.Address{firstAddr}, FromBlock: big.NewInt(rpc.PendingBlockNumber.Int64()), ToBlock: big.NewInt(rpc.PendingBlockNumber.Int64())}, allLogs[:2], ""},
//块数大于等于2或挂起日志的挖掘日志
			7: {FilterCriteria{FromBlock: big.NewInt(2), ToBlock: big.NewInt(rpc.PendingBlockNumber.Int64())}, expectedCase7, ""},
//块数大于等于2的所有“已开采”日志
			8: {FilterCriteria{FromBlock: big.NewInt(2), ToBlock: big.NewInt(rpc.LatestBlockNumber.Int64())}, allLogs[3:], ""},
//所有“开采”的原木
			9: {FilterCriteria{ToBlock: big.NewInt(rpc.LatestBlockNumber.Int64())}, allLogs, ""},
//所有“已挖掘”日志，其中1>=block num<=2和topic secondtopic
			10: {FilterCriteria{FromBlock: big.NewInt(1), ToBlock: big.NewInt(2), Topics: [][]common.Hash{{secondTopic}}}, allLogs[3:4], ""},
//主题为FirstTopic的所有“已挖掘”和挂起日志
			11: {FilterCriteria{FromBlock: big.NewInt(rpc.LatestBlockNumber.Int64()), ToBlock: big.NewInt(rpc.PendingBlockNumber.Int64()), Topics: [][]common.Hash{{firstTopic}}}, expectedCase11, ""},
//根据通配符主题匹配所有日志
			12: {FilterCriteria{Topics: [][]common.Hash{nil}}, allLogs[1:], ""},
		}
	)

//创建所有筛选器
	for i := range testCases {
		testCases[i].id, _ = api.NewFilter(testCases[i].crit)
	}

//提高事件
	time.Sleep(1 * time.Second)
	if nsend := logsFeed.Send(allLogs); nsend == 0 {
		t.Fatal("Shoud have at least one subscription")
	}
	if err := mux.Post(core.PendingLogsEvent{Logs: allLogs}); err != nil {
		t.Fatal(err)
	}

	for i, tt := range testCases {
		var fetched []*types.Log
		timeout := time.Now().Add(1 * time.Second)
for { //获取所有预期日志
			results, err := api.GetFilterChanges(tt.id)
			if err != nil {
				t.Fatalf("Unable to fetch logs: %v", err)
			}

			fetched = append(fetched, results.([]*types.Log)...)
			if len(fetched) >= len(tt.expected) {
				break
			}
//检查超时
			if time.Now().After(timeout) {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}

		if len(fetched) != len(tt.expected) {
			t.Errorf("invalid number of logs for case %d, want %d log(s), got %d", i, len(tt.expected), len(fetched))
			return
		}

		for l := range fetched {
			if fetched[l].Removed {
				t.Errorf("expected log not to be removed for log %d in case %d", l, i)
			}
			if !reflect.DeepEqual(fetched[l], tt.expected[l]) {
				t.Errorf("invalid log on index %d for case %d", l, i)
			}
		}
	}
}

//testpendinglogssubscription测试订阅是否接收到发布到事件源的正确挂起日志。
func TestPendingLogsSubscription(t *testing.T) {
	t.Parallel()

	var (
		mux        = new(event.TypeMux)
		db         = ethdb.NewMemDatabase()
		txFeed     = new(event.Feed)
		rmLogsFeed = new(event.Feed)
		logsFeed   = new(event.Feed)
		chainFeed  = new(event.Feed)
		backend    = &testBackend{mux, db, 0, txFeed, rmLogsFeed, logsFeed, chainFeed}
		api        = NewPublicFilterAPI(backend, false)

		firstAddr      = common.HexToAddress("0x1111111111111111111111111111111111111111")
		secondAddr     = common.HexToAddress("0x2222222222222222222222222222222222222222")
		thirdAddress   = common.HexToAddress("0x3333333333333333333333333333333333333333")
		notUsedAddress = common.HexToAddress("0x9999999999999999999999999999999999999999")
		firstTopic     = common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
		secondTopic    = common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222")
		thirdTopic     = common.HexToHash("0x3333333333333333333333333333333333333333333333333333333333333333")
		fourthTopic    = common.HexToHash("0x4444444444444444444444444444444444444444444444444444444444444444")
		notUsedTopic   = common.HexToHash("0x9999999999999999999999999999999999999999999999999999999999999999")

		allLogs = []core.PendingLogsEvent{
			{Logs: []*types.Log{{Address: firstAddr, Topics: []common.Hash{}, BlockNumber: 0}}},
			{Logs: []*types.Log{{Address: firstAddr, Topics: []common.Hash{firstTopic}, BlockNumber: 1}}},
			{Logs: []*types.Log{{Address: secondAddr, Topics: []common.Hash{firstTopic}, BlockNumber: 2}}},
			{Logs: []*types.Log{{Address: thirdAddress, Topics: []common.Hash{secondTopic}, BlockNumber: 3}}},
			{Logs: []*types.Log{{Address: thirdAddress, Topics: []common.Hash{secondTopic}, BlockNumber: 4}}},
			{Logs: []*types.Log{
				{Address: thirdAddress, Topics: []common.Hash{firstTopic}, BlockNumber: 5},
				{Address: thirdAddress, Topics: []common.Hash{thirdTopic}, BlockNumber: 5},
				{Address: thirdAddress, Topics: []common.Hash{fourthTopic}, BlockNumber: 5},
				{Address: firstAddr, Topics: []common.Hash{firstTopic}, BlockNumber: 5},
			}},
		}

		convertLogs = func(pl []core.PendingLogsEvent) []*types.Log {
			var logs []*types.Log
			for _, l := range pl {
				logs = append(logs, l.Logs...)
			}
			return logs
		}

		testCases = []struct {
			crit     ethereum.FilterQuery
			expected []*types.Log
			c        chan []*types.Log
			sub      *Subscription
		}{
//全部匹配
			{ethereum.FilterQuery{}, convertLogs(allLogs), nil, nil},
//由于没有匹配的地址而不匹配
			{ethereum.FilterQuery{Addresses: []common.Address{{}, notUsedAddress}, Topics: [][]common.Hash{nil}}, []*types.Log{}, nil, nil},
//根据地址匹配日志，忽略主题
			{ethereum.FilterQuery{Addresses: []common.Address{firstAddr}}, append(convertLogs(allLogs[:2]), allLogs[5].Logs[3]), nil, nil},
//由于没有匹配的主题（与地址匹配），因此不匹配
			{ethereum.FilterQuery{Addresses: []common.Address{secondAddr}, Topics: [][]common.Hash{{notUsedTopic}}}, []*types.Log{}, nil, nil},
//根据地址和主题匹配日志
			{ethereum.FilterQuery{Addresses: []common.Address{thirdAddress}, Topics: [][]common.Hash{{firstTopic, secondTopic}}}, append(convertLogs(allLogs[3:5]), allLogs[5].Logs[0]), nil, nil},
//基于多个地址和“或”主题匹配日志
			{ethereum.FilterQuery{Addresses: []common.Address{secondAddr, thirdAddress}, Topics: [][]common.Hash{{firstTopic, secondTopic}}}, append(convertLogs(allLogs[2:5]), allLogs[5].Logs[0]), nil, nil},
//对于使用新的***筛选器创建的筛选器，将忽略块号，这些块号将在状态更改时返回与给定条件匹配的所有日志。
			{ethereum.FilterQuery{Addresses: []common.Address{firstAddr}, FromBlock: big.NewInt(2), ToBlock: big.NewInt(3)}, append(convertLogs(allLogs[:2]), allLogs[5].Logs[3]), nil, nil},
//多个挂起的日志，应该只匹配块5中日志中的2个主题
			{ethereum.FilterQuery{Addresses: []common.Address{thirdAddress}, Topics: [][]common.Hash{{firstTopic, fourthTopic}}}, []*types.Log{allLogs[5].Logs[0], allLogs[5].Logs[2]}, nil, nil},
		}
	)

//创建所有订阅，这将确保在发布事件之前创建所有订阅。
//在速度较慢的计算机上，这可能会导致在以下时间之后创建订阅时丢失事件
//（某些）事件已发布。
	for i := range testCases {
		testCases[i].c = make(chan []*types.Log)
		testCases[i].sub, _ = api.events.SubscribeLogs(testCases[i].crit, testCases[i].c)
	}

	for n, test := range testCases {
		i := n
		tt := test
		go func() {
			var fetched []*types.Log
		fetchLoop:
			for {
				logs := <-tt.c
				fetched = append(fetched, logs...)
				if len(fetched) >= len(tt.expected) {
					break fetchLoop
				}
			}

			if len(fetched) != len(tt.expected) {
				panic(fmt.Sprintf("invalid number of logs for case %d, want %d log(s), got %d", i, len(tt.expected), len(fetched)))
			}

			for l := range fetched {
				if fetched[l].Removed {
					panic(fmt.Sprintf("expected log not to be removed for log %d in case %d", l, i))
				}
				if !reflect.DeepEqual(fetched[l], tt.expected[l]) {
					panic(fmt.Sprintf("invalid log on index %d for case %d", l, i))
				}
			}
		}()
	}

//提高事件
	time.Sleep(1 * time.Second)
//所有日志都是core.pendingLogSevent类型
	for _, l := range allLogs {
		if err := mux.Post(l); err != nil {
			t.Fatal(err)
		}
	}
}
