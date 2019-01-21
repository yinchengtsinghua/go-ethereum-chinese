
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

package network

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"
)

var requestedPeerID = enode.HexID("3431c3939e1ee2a6345e976a8234f9870152d64879f30bc272a074f6859e75e8")
var sourcePeerID = enode.HexID("99d8594b52298567d2ca3f4c441a5ba0140ee9245e26460d01102a52773c73b9")

//mockrequester在调用其dorequest函数时将每个请求推送到requestc通道
type mockRequester struct {
//请求[]请求
requestC  chan *Request   //当一个请求到来时，它被推送到请求C
waitTimes []time.Duration //使用waittimes[i]可以定义在第i个请求上等待的时间（可选）
count     int             //统计请求数
	quitC     chan struct{}
}

func newMockRequester(waitTimes ...time.Duration) *mockRequester {
	return &mockRequester{
		requestC:  make(chan *Request),
		waitTimes: waitTimes,
		quitC:     make(chan struct{}),
	}
}

func (m *mockRequester) doRequest(ctx context.Context, request *Request) (*enode.ID, chan struct{}, error) {
	waitTime := time.Duration(0)
	if m.count < len(m.waitTimes) {
		waitTime = m.waitTimes[m.count]
		m.count++
	}
	time.Sleep(waitTime)
	m.requestC <- request

//如果请求中存在源，请使用该源，如果不使用全局requestedpeerid
	source := request.Source
	if source == nil {
		source = &requestedPeerID
	}
	return source, m.quitC, nil
}

//testwithersinglerequest使用mockrequester创建一个取数器，并使用一组要跳过的对等方来运行它。
//mockrequester每次调用请求函数时都在通道上推送请求。使用
//这个通道我们测试调用fetcher.request是否调用request函数，以及它是否使用
//我们为fetcher.run函数提供的要跳过的正确对等点。
func TestFetcherSingleRequest(t *testing.T) {
	requester := newMockRequester()
	addr := make([]byte, 32)
	fetcher := NewFetcher(addr, requester.doRequest, true)

	peers := []string{"a", "b", "c", "d"}
	peersToSkip := &sync.Map{}
	for _, p := range peers {
		peersToSkip.Store(p, time.Now())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go fetcher.run(ctx, peersToSkip)

	rctx := context.Background()
	fetcher.Request(rctx, 0)

	select {
	case request := <-requester.requestC:
//请求应包含从PeersToSkip提供给获取程序的所有对等方
		for _, p := range peers {
			if _, ok := request.peersToSkip.Load(p); !ok {
				t.Fatalf("request.peersToSkip misses peer")
			}
		}

//源对等端最终也应添加到对等端跳过
		time.Sleep(100 * time.Millisecond)
		if _, ok := request.peersToSkip.Load(requestedPeerID.String()); !ok {
			t.Fatalf("request.peersToSkip does not contain peer returned by the request function")
		}

//转发请求中的跃点计数应递增
		if request.HopCount != 1 {
			t.Fatalf("Expected request.HopCount 1 got %v", request.HopCount)
		}

//fetch应该触发一个请求，如果它没有及时发生，测试应该失败。
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("fetch timeout")
	}
}

//testCancelStopsFetcher测试已取消的获取程序即使调用了其获取函数，也不会启动进一步的请求。
func TestFetcherCancelStopsFetcher(t *testing.T) {
	requester := newMockRequester()
	addr := make([]byte, 32)
	fetcher := NewFetcher(addr, requester.doRequest, true)

	peersToSkip := &sync.Map{}

	ctx, cancel := context.WithCancel(context.Background())

//我们启动提取程序，然后立即取消上下文
	go fetcher.run(ctx, peersToSkip)
	cancel()

	rctx, rcancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer rcancel()
//我们使用活动上下文调用请求
	fetcher.Request(rctx, 0)

//回取器不应该启动请求，我们只能通过等待一点并确保没有发生请求来进行检查。
	select {
	case <-requester.requestC:
		t.Fatalf("cancelled fetcher initiated request")
	case <-time.After(200 * time.Millisecond):
	}
}

//TestFetchCancelStopsRequest测试使用已取消的上下文调用请求函数不会启动请求
func TestFetcherCancelStopsRequest(t *testing.T) {
	requester := newMockRequester(100 * time.Millisecond)
	addr := make([]byte, 32)
	fetcher := NewFetcher(addr, requester.doRequest, true)

	peersToSkip := &sync.Map{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

//我们使用活动上下文启动提取程序
	go fetcher.run(ctx, peersToSkip)

	rctx, rcancel := context.WithCancel(context.Background())
	rcancel()

//我们用取消的上下文调用请求
	fetcher.Request(rctx, 0)

//回取器不应该启动请求，我们只能通过等待一点并确保没有发生请求来进行检查。
	select {
	case <-requester.requestC:
		t.Fatalf("cancelled fetch function initiated request")
	case <-time.After(200 * time.Millisecond):
	}

//如果有另一个具有活动上下文的请求，则应该有一个请求，因为提取程序本身没有被取消。
	rctx = context.Background()
	fetcher.Request(rctx, 0)

	select {
	case <-requester.requestC:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected request")
	}
}

//TestOfferUsesSource测试获取器提供行为。
//在这种情况下，应该有1个（并且只有一个）来自源对等端的请求，并且
//源节点ID应出现在PeerStoskip映射中。
func TestFetcherOfferUsesSource(t *testing.T) {
	requester := newMockRequester(100 * time.Millisecond)
	addr := make([]byte, 32)
	fetcher := NewFetcher(addr, requester.doRequest, true)

	peersToSkip := &sync.Map{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

//启动取纸器
	go fetcher.run(ctx, peersToSkip)

	rctx := context.Background()
//使用源对等调用offer函数
	fetcher.Offer(rctx, &sourcePeerID)

//提取程序不应启动请求
	select {
	case <-requester.requestC:
		t.Fatalf("fetcher initiated request")
	case <-time.After(200 * time.Millisecond):
	}

//报价后的呼叫请求
	rctx = context.Background()
	fetcher.Request(rctx, 0)

//从取数器中应该正好有一个请求
	var request *Request
	select {
	case request = <-requester.requestC:
		if *request.Source != sourcePeerID {
			t.Fatalf("Expected source id %v got %v", sourcePeerID, request.Source)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("fetcher did not initiate request")
	}

	select {
	case <-requester.requestC:
		t.Fatalf("Fetcher number of requests expected 1 got 2")
	case <-time.After(200 * time.Millisecond):
	}

//源对等端最终应添加到对等端跳过
	time.Sleep(100 * time.Millisecond)
	if _, ok := request.peersToSkip.Load(sourcePeerID.String()); !ok {
		t.Fatalf("SourcePeerId not added to peersToSkip")
	}
}

func TestFetcherOfferAfterRequestUsesSourceFromContext(t *testing.T) {
	requester := newMockRequester(100 * time.Millisecond)
	addr := make([]byte, 32)
	fetcher := NewFetcher(addr, requester.doRequest, true)

	peersToSkip := &sync.Map{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

//启动取纸器
	go fetcher.run(ctx, peersToSkip)

//先呼叫请求
	rctx := context.Background()
	fetcher.Request(rctx, 0)

//应该有一个来自回迁者的请求
	var request *Request
	select {
	case request = <-requester.requestC:
		if request.Source != nil {
			t.Fatalf("Incorrect source peer id, expected nil got %v", request.Source)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("fetcher did not initiate request")
	}

//请求后呼叫提供
	fetcher.Offer(context.Background(), &sourcePeerID)

//应该有一个来自回迁者的请求
	select {
	case request = <-requester.requestC:
		if *request.Source != sourcePeerID {
			t.Fatalf("Incorrect source peer id, expected %v got %v", sourcePeerID, request.Source)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("fetcher did not initiate request")
	}

//源对等端最终应添加到对等端跳过
	time.Sleep(100 * time.Millisecond)
	if _, ok := request.peersToSkip.Load(sourcePeerID.String()); !ok {
		t.Fatalf("SourcePeerId not added to peersToSkip")
	}
}

//TestFetcherRetryOnTimeout测试在SearchTimeout通过后获取重试
func TestFetcherRetryOnTimeout(t *testing.T) {
	requester := newMockRequester()
	addr := make([]byte, 32)
	fetcher := NewFetcher(addr, requester.doRequest, true)
//将SearchTimeout设置为低值，以便测试更快
	fetcher.searchTimeout = 250 * time.Millisecond

	peersToSkip := &sync.Map{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

//启动取纸器
	go fetcher.run(ctx, peersToSkip)

//使用活动上下文调用fetch函数
	rctx := context.Background()
	fetcher.Request(rctx, 0)

//100毫秒后，应启动第一个请求
	time.Sleep(100 * time.Millisecond)

	select {
	case <-requester.requestC:
	default:
		t.Fatalf("fetch did not initiate request")
	}

//再过100毫秒后，不应启动新的请求，因为搜索超时为250毫秒。
	time.Sleep(100 * time.Millisecond)

	select {
	case <-requester.requestC:
		t.Fatalf("unexpected request from fetcher")
	default:
	}

//在另一个300ms搜索超时结束后，应该有一个新的请求
	time.Sleep(300 * time.Millisecond)

	select {
	case <-requester.requestC:
	default:
		t.Fatalf("fetch did not retry request")
	}
}

//TestFetcherFactory创建一个FetcherFactory并检查工厂是否真正创建并启动
//回迁函数时的回迁函数。我们只需检查
//调用FETCH函数时启动请求
func TestFetcherFactory(t *testing.T) {
	requester := newMockRequester(100 * time.Millisecond)
	addr := make([]byte, 32)
	fetcherFactory := NewFetcherFactory(requester.doRequest, false)

	peersToSkip := &sync.Map{}

	fetcher := fetcherFactory.New(context.Background(), addr, peersToSkip)

	fetcher.Request(context.Background(), 0)

//检查创建的fetchFunction是否真的启动了一个fetcher并启动了一个请求
	select {
	case <-requester.requestC:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("fetch timeout")
	}

}

func TestFetcherRequestQuitRetriesRequest(t *testing.T) {
	requester := newMockRequester()
	addr := make([]byte, 32)
	fetcher := NewFetcher(addr, requester.doRequest, true)

//确保SearchTimeout很长，以确保请求不是
//由于超时而重试
	fetcher.searchTimeout = 10 * time.Second

	peersToSkip := &sync.Map{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go fetcher.run(ctx, peersToSkip)

	rctx := context.Background()
	fetcher.Request(rctx, 0)

	select {
	case <-requester.requestC:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("request is not initiated")
	}

	close(requester.quitC)

	select {
	case <-requester.requestC:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("request is not initiated after failed request")
	}
}

//testRequestSkipper检查peerSkip函数是否将跳过提供的peer
//不要跳过未知的。
func TestRequestSkipPeer(t *testing.T) {
	addr := make([]byte, 32)
	peers := []enode.ID{
		enode.HexID("3431c3939e1ee2a6345e976a8234f9870152d64879f30bc272a074f6859e75e8"),
		enode.HexID("99d8594b52298567d2ca3f4c441a5ba0140ee9245e26460d01102a52773c73b9"),
	}

	peersToSkip := new(sync.Map)
	peersToSkip.Store(peers[0].String(), time.Now())
	r := NewRequest(addr, false, peersToSkip)

	if !r.SkipPeer(peers[0].String()) {
		t.Errorf("peer not skipped")
	}

	if r.SkipPeer(peers[1].String()) {
		t.Errorf("peer skipped")
	}
}

//testRequestSkipperExpired检查是否未跳过要跳过的对等机
//请求超时之后。
func TestRequestSkipPeerExpired(t *testing.T) {
	addr := make([]byte, 32)
	peer := enode.HexID("3431c3939e1ee2a6345e976a8234f9870152d64879f30bc272a074f6859e75e8")

//将requestTimeout设置为低值，并在测试后重置它
	defer func(t time.Duration) { RequestTimeout = t }(RequestTimeout)
	RequestTimeout = 250 * time.Millisecond

	peersToSkip := new(sync.Map)
	peersToSkip.Store(peer.String(), time.Now())
	r := NewRequest(addr, false, peersToSkip)

	if !r.SkipPeer(peer.String()) {
		t.Errorf("peer not skipped")
	}

	time.Sleep(500 * time.Millisecond)

	if r.SkipPeer(peer.String()) {
		t.Errorf("peer skipped")
	}
}

//testRequestSkipperPermanent检查是否跳过要跳过的对等机
//如果设置为永久跳过，则不会跳过请求超时之后
//按值到PeersToSkip映射不是Time.Duration。
func TestRequestSkipPeerPermanent(t *testing.T) {
	addr := make([]byte, 32)
	peer := enode.HexID("3431c3939e1ee2a6345e976a8234f9870152d64879f30bc272a074f6859e75e8")

//将requestTimeout设置为低值，并在测试后重置它
	defer func(t time.Duration) { RequestTimeout = t }(RequestTimeout)
	RequestTimeout = 250 * time.Millisecond

	peersToSkip := new(sync.Map)
	peersToSkip.Store(peer.String(), true)
	r := NewRequest(addr, false, peersToSkip)

	if !r.SkipPeer(peer.String()) {
		t.Errorf("peer not skipped")
	}

	time.Sleep(500 * time.Millisecond)

	if !r.SkipPeer(peer.String()) {
		t.Errorf("peer not skipped")
	}
}

func TestFetcherMaxHopCount(t *testing.T) {
	requester := newMockRequester()
	addr := make([]byte, 32)
	fetcher := NewFetcher(addr, requester.doRequest, true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	peersToSkip := &sync.Map{}

	go fetcher.run(ctx, peersToSkip)

	rctx := context.Background()
	fetcher.Request(rctx, maxHopCount)

//如果HopCount已达到最大值，则不应启动任何请求。
	select {
	case <-requester.requestC:
		t.Fatalf("cancelled fetcher initiated request")
	case <-time.After(200 * time.Millisecond):
	}
}
