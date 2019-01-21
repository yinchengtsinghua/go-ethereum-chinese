
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

package storage

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/swarm/log"
	lru "github.com/hashicorp/golang-lru"
)

type (
	NewNetFetcherFunc func(ctx context.Context, addr Address, peers *sync.Map) NetFetcher
)

type NetFetcher interface {
	Request(ctx context.Context, hopCount uint8)
	Offer(ctx context.Context, source *enode.ID)
}

//NetStore是本地存储的扩展
//它实现chunkstore接口
//根据请求，它使用获取器启动远程云检索
//取数器对块是唯一的，并存储在取数器的LRU内存缓存中。
//fetchfuncFactory是为特定块地址创建提取函数的工厂对象
type NetStore struct {
	mu                sync.Mutex
	store             SyncChunkStore
	fetchers          *lru.Cache
	NewNetFetcherFunc NewNetFetcherFunc
	closeC            chan struct{}
}

var fetcherTimeout = 2 * time.Minute //即使请求正在传入，也要超时以取消获取程序

//NewNetStore使用给定的本地存储创建新的NetStore对象。newfetchfunc是一个
//可以为特定块地址创建提取函数的构造函数函数。
func NewNetStore(store SyncChunkStore, nnf NewNetFetcherFunc) (*NetStore, error) {
	fetchers, err := lru.New(defaultChunkRequestsCacheCapacity)
	if err != nil {
		return nil, err
	}
	return &NetStore{
		store:             store,
		fetchers:          fetchers,
		NewNetFetcherFunc: nnf,
		closeC:            make(chan struct{}),
	}, nil
}

//将块存储在localstore中，并使用存储在
//取数器缓存
func (n *NetStore) Put(ctx context.Context, ch Chunk) error {
	n.mu.Lock()
	defer n.mu.Unlock()

//放到存储区的块中，应该没有错误
	err := n.store.Put(ctx, ch)
	if err != nil {
		return err
	}

//如果块现在放在存储区中，请检查是否有活动的获取程序并调用传递。
//（这将通过获取器将块传递给请求者）
	if f := n.getFetcher(ch.Address()); f != nil {
		f.deliver(ctx, ch)
	}
	return nil
}

//get同步从netstore dpa中检索区块。
//它调用netstore.get，如果块不在本地存储中
//它调用带有请求的fetch，该请求将一直阻塞到块
//到达或上下文完成
func (n *NetStore) Get(rctx context.Context, ref Address) (Chunk, error) {
	chunk, fetch, err := n.get(rctx, ref)
	if err != nil {
		return nil, err
	}
	if chunk != nil {
		return chunk, nil
	}
	return fetch(rctx)
}

func (n *NetStore) BinIndex(po uint8) uint64 {
	return n.store.BinIndex(po)
}

func (n *NetStore) Iterator(from uint64, to uint64, po uint8, f func(Address, uint64) bool) error {
	return n.store.Iterator(from, to, po, f)
}

//如果存储包含给定地址，则fetchfunc返回nil。否则返回一个等待函数，
//在块可用或上下文完成后返回
func (n *NetStore) FetchFunc(ctx context.Context, ref Address) func(context.Context) error {
	chunk, fetch, _ := n.get(ctx, ref)
	if chunk != nil {
		return nil
	}
	return func(ctx context.Context) error {
		_, err := fetch(ctx)
		return err
	}
}

//关闭区块存储
func (n *NetStore) Close() {
	close(n.closeC)
	n.store.Close()
//TODO:循环访问获取器以取消它们
}

//获取从localstore检索区块的尝试
//如果未找到，则使用getorcreatefetcher:
//1。或者已经有一个获取器来检索它
//2。将创建一个新的回迁器并将其保存在回迁器缓存中。
//从这里开始，所有GET都将命中这个获取器，直到块被传递为止。
//或者所有获取器上下文都已完成。
//它返回一个块、一个获取函数和一个错误
//如果chunk为nil，则需要使用上下文调用返回的fetch函数来返回chunk。
func (n *NetStore) get(ctx context.Context, ref Address) (Chunk, func(context.Context) (Chunk, error), error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	chunk, err := n.store.Get(ctx, ref)
	if err != nil {
		if err != ErrChunkNotFound {
			log.Debug("Received error from LocalStore other than ErrNotFound", "err", err)
		}
//块在localstore中不可用，让我们为它获取取数器，或者创建一个新的取数器。
//如果它还不存在
		f := n.getOrCreateFetcher(ref)
//如果调用者需要块，它必须使用返回的fetch函数来获取块。
		return nil, f.Fetch, nil
	}

	return chunk, nil, nil
}

//GetOrCreateFetcher尝试检索现有的获取程序
//如果不存在，则创建一个并将其保存在提取缓存中。
//呼叫方必须持有锁
func (n *NetStore) getOrCreateFetcher(ref Address) *fetcher {
	if f := n.getFetcher(ref); f != nil {
		return f
	}

//没有给定地址的回信器，我们必须创建一个新地址
	key := hex.EncodeToString(ref)
//创建保持提取活动的上下文
	ctx, cancel := context.WithTimeout(context.Background(), fetcherTimeout)
//当所有请求完成时调用destroy
	destroy := func() {
//从回卷器中移除回卷器
		n.fetchers.Remove(key)
//通过取消在以下情况下调用的上下文来停止提取程序
//所有请求已取消/超时或块已传递
		cancel()
	}
//对等方总是存储对块有活动请求的所有对等方。它是共享的
//在fetchfunc函数和fetchfunc函数之间。它是NewFetchFunc所需要的，因为
//不应请求请求块的对等方来传递它。
	peers := &sync.Map{}

	fetcher := newFetcher(ref, n.NewNetFetcherFunc(ctx, ref, peers), destroy, peers, n.closeC)
	n.fetchers.Add(key, fetcher)

	return fetcher
}

//getfetcher从fetchers缓存中检索给定地址的fetcher（如果存在），
//否则返回零
func (n *NetStore) getFetcher(ref Address) *fetcher {
	key := hex.EncodeToString(ref)
	f, ok := n.fetchers.Get(key)
	if ok {
		return f.(*fetcher)
	}
	return nil
}

//requestscachelen返回当前存储在缓存中的传出请求数
func (n *NetStore) RequestsCacheLen() int {
	return n.fetchers.Len()
}

//一个获取器对象负责为一个地址获取一个块，并跟踪所有
//已请求但尚未收到的同龄人。
type fetcher struct {
addr        Address       //区块地址
chunk       Chunk         //获取器可以在获取器上设置块
deliveredC  chan struct{} //chan向请求发送块传递信号
cancelledC  chan struct{} //通知回迁器已取消的chan（从netstore的回迁器中删除）
netFetcher  NetFetcher    //使用从上下文获取的请求源调用的远程提取函数
cancel      func()        //当调用所有上游上下文时，远程获取程序调用的清除函数
peers       *sync.Map     //要求分块的同龄人
requestCnt  int32         //此区块上的请求数。如果所有请求都已完成（已传递或上下文已完成），则调用cancel函数
deliverOnce *sync.Once    //保证我们只关闭一次交货
}

//new fetcher为fiven addr创建一个新的fetcher对象。fetch是一个函数，实际上
//是否进行检索（在非测试情况下，这是来自网络包）。取消功能是
//要么调用
//1。当提取到块时，所有对等方都已收到通知或其上下文已完成。
//2。尚未提取块，但已完成所有请求中的所有上下文
//对等映射存储已请求块的所有对等。
func newFetcher(addr Address, nf NetFetcher, cancel func(), peers *sync.Map, closeC chan struct{}) *fetcher {
cancelOnce := &sync.Once{} //只应调用一次Cancel
	return &fetcher{
		addr:        addr,
		deliveredC:  make(chan struct{}),
		deliverOnce: &sync.Once{},
		cancelledC:  closeC,
		netFetcher:  nf,
		cancel: func() {
			cancelOnce.Do(func() {
				cancel()
			})
		},
		peers: peers,
	}
}

//fetch同步获取块，由netstore调用。get是块不可用
//局部地。
func (f *fetcher) Fetch(rctx context.Context) (Chunk, error) {
	atomic.AddInt32(&f.requestCnt, 1)
	defer func() {
//如果所有请求都完成了，则可以取消提取程序。
		if atomic.AddInt32(&f.requestCnt, -1) == 0 {
			f.cancel()
		}
	}()

//请求块的对等机。存储在共享对等映射中，但在请求后删除
//已交付
	peer := rctx.Value("peer")
	if peer != nil {
		f.peers.Store(peer, time.Now())
		defer f.peers.Delete(peer)
	}

//如果上下文中有一个源，那么它是一个提供，否则是一个请求
	sourceIF := rctx.Value("source")

	hopCount, _ := rctx.Value("hopcount").(uint8)

	if sourceIF != nil {
		var source enode.ID
		if err := source.UnmarshalText([]byte(sourceIF.(string))); err != nil {
			return nil, err
		}
		f.netFetcher.Offer(rctx, &source)
	} else {
		f.netFetcher.Request(rctx, hopCount)
	}

//等待块被传递或上下文完成
	select {
	case <-rctx.Done():
		return nil, rctx.Err()
	case <-f.deliveredC:
		return f.chunk, nil
	case <-f.cancelledC:
		return nil, fmt.Errorf("fetcher cancelled")
	}
}

//传递由NetStore调用。Put通知所有挂起的请求
func (f *fetcher) deliver(ctx context.Context, ch Chunk) {
	f.deliverOnce.Do(func() {
		f.chunk = ch
//关闭deliveredc通道将终止正在进行的请求
		close(f.deliveredC)
	})
}
