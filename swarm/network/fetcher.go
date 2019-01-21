
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
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

const (
	defaultSearchTimeout = 1 * time.Second
//最大转发请求数（跃点），以确保请求不
//在对等循环中永久转发
	maxHopCount uint8 = 20
)

//考虑跳过对等机的时间。
//也用于流传送。
var RequestTimeout = 10 * time.Second

type RequestFunc func(context.Context, *Request) (*enode.ID, chan struct{}, error)

//当在本地找不到块时，将创建提取程序。它启动一次请求处理程序循环，然后
//在所有活动请求完成之前保持活动状态。这可能发生：
//1。或者因为块被传递
//2。或者因为请求者取消/超时
//获取器在完成后自行销毁。
//TODO:取消终止后的所有转发请求
type Fetcher struct {
protoRequestFunc RequestFunc     //请求函数获取器调用以发出对块的检索请求
addr             storage.Address //要获取的块的地址
offerC           chan *enode.ID  //源通道（对等节点ID字符串）
requestC         chan uint8      //接收请求的通道（其中包含hopCount值）
	searchTimeout    time.Duration
	skipCheck        bool
}

type Request struct {
Addr        storage.Address //块地址
Source      *enode.ID       //请求方的节点ID（可以为零）
SkipCheck   bool            //是先提供块还是直接交付
peersToSkip *sync.Map       //不从中请求块的对等方（仅当源为零时才有意义）
HopCount    uint8           //转发请求数（跃点）
}

//NewRequest返回基于块地址跳过检查和
//要跳过的对等映射。
func NewRequest(addr storage.Address, skipCheck bool, peersToSkip *sync.Map) *Request {
	return &Request{
		Addr:        addr,
		SkipCheck:   skipCheck,
		peersToSkip: peersToSkip,
	}
}

//如果不应请求具有nodeid的对等端传递块，则skippeer返回。
//要跳过的对等点在每个请求和请求超时的一段时间内保持不变。
//此函数用于delivery.requestfrompeers中的流包中以优化
//请求块。
func (r *Request) SkipPeer(nodeID string) bool {
	val, ok := r.peersToSkip.Load(nodeID)
	if !ok {
		return false
	}
	t, ok := val.(time.Time)
	if ok && time.Now().After(t.Add(RequestTimeout)) {
//截止日期已过期
		r.peersToSkip.Delete(nodeID)
		return false
	}
	return true
}

//FetcherFactory是用请求函数初始化的，可以创建Fetcher
type FetcherFactory struct {
	request   RequestFunc
	skipCheck bool
}

//NewFetcherFactory接受请求函数并跳过检查参数并创建FetcherFactory
func NewFetcherFactory(request RequestFunc, skipCheck bool) *FetcherFactory {
	return &FetcherFactory{
		request:   request,
		skipCheck: skipCheck,
	}
}

//new为给定的块构造一个新的获取器。PeersToSkip中的所有对等机
//不要求传递给定的块。PeersToSkip应该始终
//包含主动请求此块的对等方，以确保
//不要向他们要求回块。
//创建的获取器将启动并返回。
func (f *FetcherFactory) New(ctx context.Context, source storage.Address, peersToSkip *sync.Map) storage.NetFetcher {
	fetcher := NewFetcher(source, f.request, f.skipCheck)
	go fetcher.run(ctx, peersToSkip)
	return fetcher
}

//new fetcher使用给定的请求函数为给定的块地址创建一个新的fetcher。
func NewFetcher(addr storage.Address, rf RequestFunc, skipCheck bool) *Fetcher {
	return &Fetcher{
		addr:             addr,
		protoRequestFunc: rf,
		offerC:           make(chan *enode.ID),
		requestC:         make(chan uint8),
		searchTimeout:    defaultSearchTimeout,
		skipCheck:        skipCheck,
	}
}

//当上游对等端通过同步作为“offeredhashemsg”的一部分来提供区块，并且节点在本地没有区块时，调用offer。
func (f *Fetcher) Offer(ctx context.Context, source *enode.ID) {
//首先，我们需要进行此选择，以确保在上下文完成时返回
	select {
	case <-ctx.Done():
		return
	default:
	}

//仅此选择并不能保证返回上下文已完成，它可能
//如果提供offerc，则推送至offerc（请参阅https://golang.org/ref/spec select_语句中的数字2）
	select {
	case f.offerC <- source:
	case <-ctx.Done():
	}
}

//当上游对等端作为“retrieverequestmsg”的一部分或通过filestore从本地请求请求请求块，并且节点在本地没有块时，调用请求。
func (f *Fetcher) Request(ctx context.Context, hopCount uint8) {
//首先，我们需要进行此选择，以确保在上下文完成时返回
	select {
	case <-ctx.Done():
		return
	default:
	}

	if hopCount >= maxHopCount {
		log.Debug("fetcher request hop count limit reached", "hops", hopCount)
		return
	}

//仅此选择并不能保证返回上下文已完成，它可能
//如果提供offerc，则推送至offerc（请参阅https://golang.org/ref/spec select_语句中的数字2）
	select {
	case f.requestC <- hopCount + 1:
	case <-ctx.Done():
	}
}

//开始准备获取程序
//它在传递的上下文的生命周期内保持提取程序的活动状态
func (f *Fetcher) run(ctx context.Context, peers *sync.Map) {
	var (
doRequest bool             //确定是否在当前迭代中启动检索
wait      *time.Timer      //搜索超时计时器
waitC     <-chan time.Time //计时器通道
sources   []*enode.ID      //已知来源，即提供数据块的对等方
requested bool             //如果块是实际请求的，则为true
		hopCount  uint8
	)
gone := make(chan *enode.ID) //向我们请求的对等机发出信号的通道

//保持提取进程活动的循环
//每次请求后，都会设置一个计时器。如果发生这种情况，我们会再次向另一位同行请求
//请注意，上一个请求仍然有效，并且有机会传递，因此
//再次请求将扩展搜索范围。I.
//如果我们请求的对等点不在，我们将发出一个新的请求，因此活动的
//请求从不减少
	for {
		select {

//来料报价
		case source := <-f.offerC:
			log.Trace("new source", "peer addr", source, "request addr", f.addr)
//1）块由同步对等提供
//添加到已知源
			sources = append(sources, source)
//向源iff发送请求请求块被请求（不仅仅是因为同步对等提供了块）
			doRequest = requested

//传入请求
		case hopCount = <-f.requestC:
			log.Trace("new request", "request addr", f.addr)
//2）请求块，设置请求标志
//启动一个请求如果还没有启动
			doRequest = !requested
			requested = true

//我们请求的同伴不见了。回到另一个
//从对等映射中删除对等
		case id := <-gone:
			log.Trace("peer gone", "peer id", id.String(), "request addr", f.addr)
			peers.Delete(id.String())
			doRequest = requested

//搜索超时：自上次请求以来经过的时间太长，
//如果我们能找到一个新的对等点，就把搜索扩展到一个新的对等点。
		case <-waitC:
			log.Trace("search timed out: requesting", "request addr", f.addr)
			doRequest = requested

//所有提取程序上下文都已关闭，无法退出
		case <-ctx.Done():
			log.Trace("terminate fetcher", "request addr", f.addr)
//TODO:向对等映射中剩余的所有对等发送取消通知（即，我们请求的那些对等）
			return
		}

//需要发出新请求
		if doRequest {
			var err error
			sources, err = f.doRequest(ctx, gone, peers, sources, hopCount)
			if err != nil {
				log.Info("unable to request", "request addr", f.addr, "err", err)
			}
		}

//如果未设置等待通道，则将其设置为计时器。
		if requested {
			if wait == nil {
				wait = time.NewTimer(f.searchTimeout)
				defer wait.Stop()
				waitC = wait.C
			} else {
//如果之前没有排空，请停止计时器并排空通道。
				if !wait.Stop() {
					select {
					case <-wait.C:
					default:
					}
				}
//将计时器重置为在默认搜索超时后关闭
				wait.Reset(f.searchTimeout)
			}
		}
		doRequest = false
	}
}

//Dorequest试图找到一个对等机来请求块
//*首先，它尝试从已知提供了块的对等方显式请求
//*如果没有这样的对等点（可用），它会尝试从最接近块地址的对等点请求它。
//排除PeersToSkip地图中的那些
//*如果未找到此类对等机，则返回错误。
//
//如果请求成功，
//*将对等机的地址添加到要跳过的对等机集合中。
//*从预期来源中删除对等方的地址，以及
//*如果对等端断开连接（或终止其拖缆），将启动一个Go例行程序，报告消失的通道。
func (f *Fetcher) doRequest(ctx context.Context, gone chan *enode.ID, peersToSkip *sync.Map, sources []*enode.ID, hopCount uint8) ([]*enode.ID, error) {
	var i int
	var sourceID *enode.ID
	var quit chan struct{}

	req := &Request{
		Addr:        f.addr,
		SkipCheck:   f.skipCheck,
		peersToSkip: peersToSkip,
		HopCount:    hopCount,
	}

	foundSource := false
//迭代已知源
	for i = 0; i < len(sources); i++ {
		req.Source = sources[i]
		var err error
		sourceID, quit, err = f.protoRequestFunc(ctx, req)
		if err == nil {
//从已知源中删除对等机
//注意：我们可以修改源代码，尽管我们在它上面循环，因为我们会立即从循环中断。
			sources = append(sources[:i], sources[i+1:]...)
			foundSource = true
			break
		}
	}

//如果没有已知的源，或者没有可用的源，我们尝试从最近的节点请求。
	if !foundSource {
		req.Source = nil
		var err error
		sourceID, quit, err = f.protoRequestFunc(ctx, req)
		if err != nil {
//如果找不到对等方请求
			return sources, err
		}
	}
//将对等添加到要从现在开始跳过的对等集
	peersToSkip.Store(sourceID.String(), time.Now())

//如果退出通道关闭，则表示我们请求的源对等端
//断开或终止拖缆
//这里开始一个执行例程，监视这个通道并报告消失通道上的源对等点。
//如果已完成获取器全局上下文以防止进程泄漏，则此go例程将退出。
	go func() {
		select {
		case <-quit:
			gone <- sourceID
		case <-ctx.Done():
		}
	}()
	return sources, nil
}
