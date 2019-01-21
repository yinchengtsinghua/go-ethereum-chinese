
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/p2p/enode"
	ch "github.com/ethereum/go-ethereum/swarm/chunk"
)

var sourcePeerID = enode.HexID("99d8594b52298567d2ca3f4c441a5ba0140ee9245e26460d01102a52773c73b9")

type mockNetFetcher struct {
	peers           *sync.Map
	sources         []*enode.ID
	peersPerRequest [][]Address
	requestCalled   bool
	offerCalled     bool
	quit            <-chan struct{}
	ctx             context.Context
	hopCounts       []uint8
	mu              sync.Mutex
}

func (m *mockNetFetcher) Offer(ctx context.Context, source *enode.ID) {
	m.offerCalled = true
	m.sources = append(m.sources, source)
}

func (m *mockNetFetcher) Request(ctx context.Context, hopCount uint8) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requestCalled = true
	var peers []Address
	m.peers.Range(func(key interface{}, _ interface{}) bool {
		peers = append(peers, common.FromHex(key.(string)))
		return true
	})
	m.peersPerRequest = append(m.peersPerRequest, peers)
	m.hopCounts = append(m.hopCounts, hopCount)
}

type mockNetFetchFuncFactory struct {
	fetcher *mockNetFetcher
}

func (m *mockNetFetchFuncFactory) newMockNetFetcher(ctx context.Context, _ Address, peers *sync.Map) NetFetcher {
	m.fetcher.peers = peers
	m.fetcher.quit = ctx.Done()
	m.fetcher.ctx = ctx
	return m.fetcher
}

func mustNewNetStore(t *testing.T) *NetStore {
	netStore, _ := mustNewNetStoreWithFetcher(t)
	return netStore
}

func mustNewNetStoreWithFetcher(t *testing.T) (*NetStore, *mockNetFetcher) {
	t.Helper()

	datadir, err := ioutil.TempDir("", "netstore")
	if err != nil {
		t.Fatal(err)
	}
	naddr := make([]byte, 32)
	params := NewDefaultLocalStoreParams()
	params.Init(datadir)
	params.BaseKey = naddr
	localStore, err := NewTestLocalStoreForAddr(params)
	if err != nil {
		t.Fatal(err)
	}

	fetcher := &mockNetFetcher{}
	mockNetFetchFuncFactory := &mockNetFetchFuncFactory{
		fetcher: fetcher,
	}
	netStore, err := NewNetStore(localStore, mockNetFetchFuncFactory.newMockNetFetcher)
	if err != nil {
		t.Fatal(err)
	}
	return netStore, fetcher
}

//调用netstore.get的testNetStoreGetAndPut测试将被阻止，直到放入同一块。
//放置之后，不应该有活动的获取器，为获取器创建的上下文应该
//被取消。
func TestNetStoreGetAndPut(t *testing.T) {
	netStore, fetcher := mustNewNetStoreWithFetcher(t)

	chunk := GenerateRandomChunk(ch.DefaultSize)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

c := make(chan struct{}) //此通道确保Put的Gouroutine不会在GeT之前运行。
	putErrC := make(chan error)
	go func() {
<-c                                //等待呼叫
time.Sleep(200 * time.Millisecond) //再多一点，这就是所谓的

//检查netstore是否在get调用中为不可用的块创建了一个获取程序
		if netStore.fetchers.Len() != 1 || netStore.getFetcher(chunk.Address()) == nil {
			putErrC <- errors.New("Expected netStore to use a fetcher for the Get call")
			return
		}

		err := netStore.Put(ctx, chunk)
		if err != nil {
			putErrC <- fmt.Errorf("Expected no err got %v", err)
			return
		}

		putErrC <- nil
	}()

	close(c)
recChunk, err := netStore.Get(ctx, chunk.Address()) //在完成上述放置之前，此操作将被阻止。
	if err != nil {
		t.Fatalf("Expected no err got %v", err)
	}

	if err := <-putErrC; err != nil {
		t.Fatal(err)
	}
//检索到的块应该与我们放置的块相同
	if !bytes.Equal(recChunk.Address(), chunk.Address()) || !bytes.Equal(recChunk.Data(), chunk.Data()) {
		t.Fatalf("Different chunk received than what was put")
	}
//块已经在本地可用，因此不应该有活动的获取程序等待它。
	if netStore.fetchers.Len() != 0 {
		t.Fatal("Expected netStore to remove the fetcher after delivery")
	}

//调用get时创建了一个获取程序（而块不可用）。块
//是随Put调用一起发送的，因此应立即取消提取程序。
	select {
	case <-fetcher.ctx.Done():
	default:
		t.Fatal("Expected fetcher context to be cancelled")
	}

}

//测试netstore和测试调用netstore.put，然后调用netstore.get。
//在Put之后，块在本地可用，因此get可以从localstore中检索它，
//不需要创建回迁器。
func TestNetStoreGetAfterPut(t *testing.T) {
	netStore, fetcher := mustNewNetStoreWithFetcher(t)

	chunk := GenerateRandomChunk(ch.DefaultSize)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

//首先我们放置块，这样块将在本地可用
	err := netStore.Put(ctx, chunk)
	if err != nil {
		t.Fatalf("Expected no err got %v", err)
	}

//get应该从localstore中检索块，而不创建gether
	recChunk, err := netStore.Get(ctx, chunk.Address())
	if err != nil {
		t.Fatalf("Expected no err got %v", err)
	}
//检索到的块应该与我们放置的块相同
	if !bytes.Equal(recChunk.Address(), chunk.Address()) || !bytes.Equal(recChunk.Data(), chunk.Data()) {
		t.Fatalf("Different chunk received than what was put")
	}
//不应为本地可用的块创建提取程序提供或请求
	if fetcher.offerCalled || fetcher.requestCalled {
		t.Fatal("NetFetcher.offerCalled or requestCalled not expected to be called")
	}
//不应为本地可用块创建提取程序
	if netStore.fetchers.Len() != 0 {
		t.Fatal("Expected netStore to not have fetcher")
	}

}

//testnetstoregettimeout测试对不可用块的get调用并等待超时
func TestNetStoreGetTimeout(t *testing.T) {
	netStore, fetcher := mustNewNetStoreWithFetcher(t)

	chunk := GenerateRandomChunk(ch.DefaultSize)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

c := make(chan struct{}) //该通道确保Gouroutine不会在GET之前运行
	fetcherErrC := make(chan error)
	go func() {
<-c                                //等待呼叫
time.Sleep(200 * time.Millisecond) //再多一点，这就是所谓的

//检查netstore是否在get调用中为不可用的块创建了一个获取程序
		if netStore.fetchers.Len() != 1 || netStore.getFetcher(chunk.Address()) == nil {
			fetcherErrC <- errors.New("Expected netStore to use a fetcher for the Get call")
			return
		}

		fetcherErrC <- nil
	}()

	close(c)
//我们调用不在本地存储区中的get on这个块。我们一点也不放，所以会有
//超时
	_, err := netStore.Get(ctx, chunk.Address())

//检查是否发生超时
	if err != context.DeadlineExceeded {
		t.Fatalf("Expected context.DeadLineExceeded err got %v", err)
	}

	if err := <-fetcherErrC; err != nil {
		t.Fatal(err)
	}

//已创建提取程序，请检查是否已在超时后将其删除。
	if netStore.fetchers.Len() != 0 {
		t.Fatal("Expected netStore to remove the fetcher after timeout")
	}

//检查超时后是否已取消获取器上下文
	select {
	case <-fetcher.ctx.Done():
	default:
		t.Fatal("Expected fetcher context to be cancelled")
	}
}

//testnetstoregetcancel测试get调用中的不可用块，然后取消上下文并检查
//错误
func TestNetStoreGetCancel(t *testing.T) {
	netStore, fetcher := mustNewNetStoreWithFetcher(t)

	chunk := GenerateRandomChunk(ch.DefaultSize)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

c := make(chan struct{}) //此通道确保取消的Gouroutine运行时间不早于get
	fetcherErrC := make(chan error, 1)
	go func() {
<-c                                //等待呼叫
time.Sleep(200 * time.Millisecond) //再多一点，这就是所谓的
//检查netstore是否在get调用中为不可用的块创建了一个获取程序
		if netStore.fetchers.Len() != 1 || netStore.getFetcher(chunk.Address()) == nil {
			fetcherErrC <- errors.New("Expected netStore to use a fetcher for the Get call")
			return
		}

		fetcherErrC <- nil
		cancel()
	}()

	close(c)

//我们使用一个不可用的块来调用get，因此它将创建一个获取器并等待传递
	_, err := netStore.Get(ctx, chunk.Address())

	if err := <-fetcherErrC; err != nil {
		t.Fatal(err)
	}

//取消上下文后，上面的get应返回并返回一个错误
	if err != context.Canceled {
		t.Fatalf("Expected context.Canceled err got %v", err)
	}

//已创建提取程序，请检查取消后是否已删除该提取程序。
	if netStore.fetchers.Len() != 0 {
		t.Fatal("Expected netStore to remove the fetcher after cancel")
	}

//检查请求上下文取消后是否已取消提取程序上下文
	select {
	case <-fetcher.ctx.Done():
	default:
		t.Fatal("Expected fetcher context to be cancelled")
	}
}

//testnetstoremultipleegandput测试四个get调用相同的不可用块。块是
//我们必须确保所有的get调用都返回，并且它们使用一个gether
//用于区块检索
func TestNetStoreMultipleGetAndPut(t *testing.T) {
	netStore, fetcher := mustNewNetStoreWithFetcher(t)

	chunk := GenerateRandomChunk(ch.DefaultSize)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	putErrC := make(chan error)
	go func() {
//睡一觉，以确保在所有的获取之后都调用Put。
		time.Sleep(500 * time.Millisecond)
//检查netstore是否为所有get调用创建了一个提取程序
		if netStore.fetchers.Len() != 1 {
			putErrC <- errors.New("Expected netStore to use one fetcher for all Get calls")
			return
		}
		err := netStore.Put(ctx, chunk)
		if err != nil {
			putErrC <- fmt.Errorf("Expected no err got %v", err)
			return
		}
		putErrC <- nil
	}()

	count := 4
//对相同的不可用块调用get 4次。呼叫将被阻止，直到上面的Put。
	errC := make(chan error)
	for i := 0; i < count; i++ {
		go func() {
			recChunk, err := netStore.Get(ctx, chunk.Address())
			if err != nil {
				errC <- fmt.Errorf("Expected no err got %v", err)
			}
			if !bytes.Equal(recChunk.Address(), chunk.Address()) || !bytes.Equal(recChunk.Data(), chunk.Data()) {
				errC <- errors.New("Different chunk received than what was put")
			}
			errC <- nil
		}()
	}

	if err := <-putErrC; err != nil {
		t.Fatal(err)
	}

	timeout := time.After(1 * time.Second)

//get调用应该在put之后返回，因此不需要超时
	for i := 0; i < count; i++ {
		select {
		case err := <-errC:
			if err != nil {
				t.Fatal(err)
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for Get calls to return")
		}
	}

//已创建提取程序，请检查取消后是否已删除该提取程序。
	if netStore.fetchers.Len() != 0 {
		t.Fatal("Expected netStore to remove the fetcher after delivery")
	}

//已创建提取程序，请检查它是否在传递后被删除。
	select {
	case <-fetcher.ctx.Done():
	default:
		t.Fatal("Expected fetcher context to be cancelled")
	}

}

//testNetStoreFetchFunctionTimeout测试fetchfunc调用是否存在不可用的块并等待超时
func TestNetStoreFetchFuncTimeout(t *testing.T) {
	netStore, fetcher := mustNewNetStoreWithFetcher(t)

	chunk := GenerateRandomChunk(ch.DefaultSize)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

//对不可用的块调用了fetchfunc，因此返回的wait函数不应为nil。
	wait := netStore.FetchFunc(ctx, chunk.Address())
	if wait == nil {
		t.Fatal("Expected wait function to be not nil")
	}

//在fetchfunc调用之后，应该有一个活动的块提取程序
	if netStore.fetchers.Len() != 1 || netStore.getFetcher(chunk.Address()) == nil {
		t.Fatalf("Expected netStore to have one fetcher for the requested chunk")
	}

//等待函数应该超时，因为我们不使用Put传递块
	err := wait(ctx)
	if err != context.DeadlineExceeded {
		t.Fatalf("Expected context.DeadLineExceeded err got %v", err)
	}

//回卷器应该在超时后移除
	if netStore.fetchers.Len() != 0 {
		t.Fatal("Expected netStore to remove the fetcher after timeout")
	}

//超时后应取消获取器上下文
	select {
	case <-fetcher.ctx.Done():
	default:
		t.Fatal("Expected fetcher context to be cancelled")
	}
}

//testnetstorefetchfuncafterput测试fetchfunc应该为本地可用块返回nil
func TestNetStoreFetchFuncAfterPut(t *testing.T) {
	netStore := mustNewNetStore(t)

	chunk := GenerateRandomChunk(ch.DefaultSize)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

//我们用Put传递创建的块
	err := netStore.Put(ctx, chunk)
	if err != nil {
		t.Fatalf("Expected no err got %v", err)
	}

//fetchfunc应该返回nil，因为块在本地可用，所以不需要获取它。
	wait := netStore.FetchFunc(ctx, chunk.Address())
	if wait != nil {
		t.Fatal("Expected wait to be nil")
	}

//根本不应该创建回迁器
	if netStore.fetchers.Len() != 0 {
		t.Fatal("Expected netStore to not have fetcher")
	}
}

//如果在NetFetcher上为不可用的块创建了请求，则测试NetStoreGetCallsRequest测试
func TestNetStoreGetCallsRequest(t *testing.T) {
	netStore, fetcher := mustNewNetStoreWithFetcher(t)

	chunk := GenerateRandomChunk(ch.DefaultSize)

	ctx := context.WithValue(context.Background(), "hopcount", uint8(5))
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

//我们调用get获取一个不可用的块，它将超时，因为该块未被传递
	_, err := netStore.Get(ctx, chunk.Address())

	if err != context.DeadlineExceeded {
		t.Fatalf("Expected context.DeadlineExceeded err got %v", err)
	}

//netstore应该调用netfetcher.request并等待数据块
	if !fetcher.requestCalled {
		t.Fatal("Expected NetFetcher.Request to be called")
	}

	if fetcher.hopCounts[0] != 5 {
		t.Fatalf("Expected NetFetcher.Request be called with hopCount 5, got %v", fetcher.hopCounts[0])
	}
}

//如果在NetFetcher上为不可用的块创建了一个请求，则测试netStoreGetCallsOffer测试
//在上下文中提供源对等的情况下。
func TestNetStoreGetCallsOffer(t *testing.T) {
	netStore, fetcher := mustNewNetStoreWithFetcher(t)

	chunk := GenerateRandomChunk(ch.DefaultSize)

//如果将源对等添加到上下文中，NetStore将把它作为一个提供来处理。
	ctx := context.WithValue(context.Background(), "source", sourcePeerID.String())
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

//我们调用get获取一个不可用的块，它将超时，因为该块未被传递
	_, err := netStore.Get(ctx, chunk.Address())

	if err != context.DeadlineExceeded {
		t.Fatalf("Expect error %v got %v", context.DeadlineExceeded, err)
	}

//NetStore应调用NetFetcher。与源对等端一起提供
	if !fetcher.offerCalled {
		t.Fatal("Expected NetFetcher.Request to be called")
	}

	if len(fetcher.sources) != 1 {
		t.Fatalf("Expected fetcher sources length 1 got %v", len(fetcher.sources))
	}

	if fetcher.sources[0].String() != sourcePeerID.String() {
		t.Fatalf("Expected fetcher source %v got %v", sourcePeerID, fetcher.sources[0])
	}

}

//testNetStoreFetcherCountPeers测试多个netstore.get调用上下文中的Peer。
//没有Put调用，因此get调用超时
func TestNetStoreFetcherCountPeers(t *testing.T) {

	netStore, fetcher := mustNewNetStoreWithFetcher(t)

	addr := randomAddr()
	peers := []string{randomAddr().Hex(), randomAddr().Hex(), randomAddr().Hex()}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	errC := make(chan error)
	nrGets := 3

//使用上下文中的对等方调用get 3次
	for i := 0; i < nrGets; i++ {
		peer := peers[i]
		go func() {
			ctx := context.WithValue(ctx, "peer", peer)
			_, err := netStore.Get(ctx, addr)
			errC <- err
		}()
	}

//所有3个get调用都应超时
	for i := 0; i < nrGets; i++ {
		err := <-errC
		if err != context.DeadlineExceeded {
			t.Fatalf("Expected \"%v\" error got \"%v\"", context.DeadlineExceeded, err)
		}
	}

//回卷器应在超时后关闭
	select {
	case <-fetcher.quit:
	case <-time.After(3 * time.Second):
		t.Fatalf("mockNetFetcher not closed after timeout")
	}

//在3个GET调用后，应将所有3个对等端都提供给NetFetcher。
	if len(fetcher.peersPerRequest) != nrGets {
		t.Fatalf("Expected 3 got %v", len(fetcher.peersPerRequest))
	}

	for i, peers := range fetcher.peersPerRequest {
		if len(peers) < i+1 {
			t.Fatalf("Expected at least %v got %v", i+1, len(peers))
		}
	}
}

//testnetstorefetchfuncalledmultipleTimes调用fetchfunc给出的等待函数三次，
//并且检查一个块是否仍然只有一个取件器。在传送数据块后，它会检查
//如果取纸器关闭。
func TestNetStoreFetchFuncCalledMultipleTimes(t *testing.T) {
	netStore, fetcher := mustNewNetStoreWithFetcher(t)

	chunk := GenerateRandomChunk(ch.DefaultSize)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

//fetchfunc应返回非nil等待函数，因为块不可用
	wait := netStore.FetchFunc(ctx, chunk.Address())
	if wait == nil {
		t.Fatal("Expected wait function to be not nil")
	}

//块应该只有一个取件器
	if netStore.fetchers.Len() != 1 || netStore.getFetcher(chunk.Address()) == nil {
		t.Fatalf("Expected netStore to have one fetcher for the requested chunk")
	}

//同时呼叫等待三次
	count := 3
	errC := make(chan error)
	for i := 0; i < count; i++ {
		go func() {
			errC <- wait(ctx)
		}()
	}

//稍微睡一会儿，以便上面调用wait函数
	time.Sleep(100 * time.Millisecond)

//应该仍然只有一个取数器，因为所有的等待调用都是针对同一块的。
	if netStore.fetchers.Len() != 1 || netStore.getFetcher(chunk.Address()) == nil {
		t.Fatal("Expected netStore to have one fetcher for the requested chunk")
	}

//用Put交付块
	err := netStore.Put(ctx, chunk)
	if err != nil {
		t.Fatalf("Expected no err got %v", err)
	}

//等待所有等待调用返回（因为块已传递）
	for i := 0; i < count; i++ {
		err := <-errC
		if err != nil {
			t.Fatal(err)
		}
	}

//对于已传递的块不应该有更多的获取程序
	if netStore.fetchers.Len() != 0 {
		t.Fatal("Expected netStore to remove the fetcher after delivery")
	}

//回迁器的上下文应在传递后取消。
	select {
	case <-fetcher.ctx.Done():
	default:
		t.Fatal("Expected fetcher context to be cancelled")
	}
}

//testNetStoreFetcherLifecycleWithTimeout类似于testNetStoreFetchFunccalledMultipleTimes，
//唯一的区别是我们不解除块的划分，只需等待超时
func TestNetStoreFetcherLifeCycleWithTimeout(t *testing.T) {
	netStore, fetcher := mustNewNetStoreWithFetcher(t)

	chunk := GenerateRandomChunk(ch.DefaultSize)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

//fetchfunc应返回非nil等待函数，因为块不可用
	wait := netStore.FetchFunc(ctx, chunk.Address())
	if wait == nil {
		t.Fatal("Expected wait function to be not nil")
	}

//块应该只有一个取件器
	if netStore.fetchers.Len() != 1 || netStore.getFetcher(chunk.Address()) == nil {
		t.Fatalf("Expected netStore to have one fetcher for the requested chunk")
	}

//同时呼叫等待三次
	count := 3
	errC := make(chan error)
	for i := 0; i < count; i++ {
		go func() {
			rctx, rcancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer rcancel()
			err := wait(rctx)
			if err != context.DeadlineExceeded {
				errC <- fmt.Errorf("Expected err %v got %v", context.DeadlineExceeded, err)
				return
			}
			errC <- nil
		}()
	}

//等待直到所有等待调用超时
	for i := 0; i < count; i++ {
		err := <-errC
		if err != nil {
			t.Fatal(err)
		}
	}

//超时后不应该有更多的获取程序
	if netStore.fetchers.Len() != 0 {
		t.Fatal("Expected netStore to remove the fetcher after delivery")
	}

//回迁器的上下文应在超时后取消。
	select {
	case <-fetcher.ctx.Done():
	default:
		t.Fatal("Expected fetcher context to be cancelled")
	}
}

func randomAddr() Address {
	addr := make([]byte, 32)
	rand.Read(addr)
	return Address(addr)
}
