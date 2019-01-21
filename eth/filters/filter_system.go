
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

//包过滤器为块实现以太坊过滤系统，
//事务和日志事件。
package filters

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

//类型确定筛选器的类型，并用于将筛选器放入
//添加正确的桶。
type Type byte

const (
//UnknownSubscription表示未知的订阅类型
	UnknownSubscription Type = iota
//新日志或已删除日志的日志订阅查询（chain reorg）
	LogsSubscription
//PendingLogs对挂起块中的日志的订阅查询
	PendingLogsSubscription
//MinedAndPendingLogs对挖掘和挂起块中的日志的订阅查询。
	MinedAndPendingLogsSubscription
//PendingtTransactionsSubscription查询待处理的Tx哈希
//进入挂起状态的事务
	PendingTransactionsSubscription
//块订阅查询导入块的哈希
	BlocksSubscription
//LastSubscription跟踪最后一个索引
	LastIndexSubscription
)

const (

//txchanSize是侦听newtxSevent的频道的大小。
//该数字是根据Tx池的大小引用的。
	txChanSize = 4096
//rmlogschansize是侦听removedlogsevent的通道的大小。
	rmLogsChanSize = 10
//logschansize是侦听logsevent的通道的大小。
	logsChanSize = 10
//ChainevChansize是侦听ChainEvent的通道的大小。
	chainEvChanSize = 10
)

var (
	ErrInvalidSubscriptionID = errors.New("invalid id")
)

type subscription struct {
	id        rpc.ID
	typ       Type
	created   time.Time
	logsCrit  ethereum.FilterQuery
	logs      chan []*types.Log
	hashes    chan []common.Hash
	headers   chan *types.Header
installed chan struct{} //安装过滤器时关闭
err       chan error    //卸载筛选器时关闭
}

//EventSystem creates subscriptions, processes events and broadcasts them to the
//符合订阅条件的订阅。
type EventSystem struct {
	mux       *event.TypeMux
	backend   Backend
	lightMode bool
	lastHead  *types.Header

//订阅
txsSub        event.Subscription         //订阅新事务事件
logsSub       event.Subscription         //订阅新日志事件
rmLogsSub     event.Subscription         //已删除日志事件的订阅
chainSub      event.Subscription         //订阅新的链事件
pendingLogSub *event.TypeMuxSubscription //订阅挂起日志事件

//渠道
install   chan *subscription         //安装事件通知筛选器
uninstall chan *subscription         //remove filter for event notification
txsCh     chan core.NewTxsEvent      //接收新事务事件的通道
logsCh    chan []*types.Log          //接收新日志事件的通道
rmLogsCh  chan core.RemovedLogsEvent //接收已删除日志事件的通道
chainCh   chan core.ChainEvent       //接收新链事件的通道
}

//newEventSystem创建一个新的管理器，用于侦听给定mux上的事件，
//分析并过滤它们。它使用全部映射来检索过滤器更改。这个
//工作循环保存自己的索引，用于将事件转发到筛选器。
//
//返回的管理器有一个需要用stop函数停止的循环
//或者停止给定的多路复用器。
func NewEventSystem(mux *event.TypeMux, backend Backend, lightMode bool) *EventSystem {
	m := &EventSystem{
		mux:       mux,
		backend:   backend,
		lightMode: lightMode,
		install:   make(chan *subscription),
		uninstall: make(chan *subscription),
		txsCh:     make(chan core.NewTxsEvent, txChanSize),
		logsCh:    make(chan []*types.Log, logsChanSize),
		rmLogsCh:  make(chan core.RemovedLogsEvent, rmLogsChanSize),
		chainCh:   make(chan core.ChainEvent, chainEvChanSize),
	}

//订阅事件
	m.txsSub = m.backend.SubscribeNewTxsEvent(m.txsCh)
	m.logsSub = m.backend.SubscribeLogsEvent(m.logsCh)
	m.rmLogsSub = m.backend.SubscribeRemovedLogsEvent(m.rmLogsCh)
	m.chainSub = m.backend.SubscribeChainEvent(m.chainCh)
//TODO（RJL493456442）：使用feed订阅挂起的日志事件
	m.pendingLogSub = m.mux.Subscribe(core.PendingLogsEvent{})

//确保所有订阅都不为空
	if m.txsSub == nil || m.logsSub == nil || m.rmLogsSub == nil || m.chainSub == nil ||
		m.pendingLogSub.Closed() {
		log.Crit("Subscribe for event system failed")
	}

	go m.eventLoop()
	return m
}

//订阅是在客户端为特定事件注册自身时创建的。
type Subscription struct {
	ID        rpc.ID
	f         *subscription
	es        *EventSystem
	unsubOnce sync.Once
}

//Err returns a channel that is closed when unsubscribed.
func (sub *Subscription) Err() <-chan error {
	return sub.f.err
}

//取消订阅从事件广播循环中卸载订阅。
func (sub *Subscription) Unsubscribe() {
	sub.unsubOnce.Do(func() {
	uninstallLoop:
		for {
//编写卸载请求并使用日志/哈希。这防止
//写入时死锁的EventLoop广播方法
//订阅循环等待时筛选事件通道
//此方法返回（因此不读取这些事件）。
			select {
			case sub.es.uninstall <- sub.f:
				break uninstallLoop
			case <-sub.f.logs:
			case <-sub.f.hashes:
			case <-sub.f.headers:
			}
		}

//在返回之前，请等待在工作循环中卸载筛选器
//this ensures that the manager won't use the event channel which
//在该方法返回后，客户端可能会尽快关闭。
		<-sub.Err()
	})
}

//订阅在事件广播循环中安装订阅。
func (es *EventSystem) subscribe(sub *subscription) *Subscription {
	es.install <- sub
	<-sub.installed
	return &Subscription{ID: sub.id, f: sub, es: es}
}

//subscriptLogs创建一个订阅，该订阅将写入与
//给定日志通道的给定条件。从和到的默认值
//块为“最新”。如果FromBlock>ToBlock，则返回错误。
func (es *EventSystem) SubscribeLogs(crit ethereum.FilterQuery, logs chan []*types.Log) (*Subscription, error) {
	var from, to rpc.BlockNumber
	if crit.FromBlock == nil {
		from = rpc.LatestBlockNumber
	} else {
		from = rpc.BlockNumber(crit.FromBlock.Int64())
	}
	if crit.ToBlock == nil {
		to = rpc.LatestBlockNumber
	} else {
		to = rpc.BlockNumber(crit.ToBlock.Int64())
	}

//只对挂起的日志感兴趣
	if from == rpc.PendingBlockNumber && to == rpc.PendingBlockNumber {
		return es.subscribePendingLogs(crit, logs), nil
	}
//只对新开采的原木感兴趣
	if from == rpc.LatestBlockNumber && to == rpc.LatestBlockNumber {
		return es.subscribeLogs(crit, logs), nil
	}
//仅对特定区块范围内的开采原木感兴趣
	if from >= 0 && to >= 0 && to >= from {
		return es.subscribeLogs(crit, logs), nil
	}
//interested in mined logs from a specific block number, new logs and pending logs
	if from >= rpc.LatestBlockNumber && to == rpc.PendingBlockNumber {
		return es.subscribeMinedPendingLogs(crit, logs), nil
	}
//对从特定区块编号到新开采区块的原木感兴趣
	if from >= 0 && to == rpc.LatestBlockNumber {
		return es.subscribeLogs(crit, logs), nil
	}
	return nil, fmt.Errorf("invalid from and to block combination: from > to")
}

//subscripbeMinedPendingLogs创建一个返回已挖掘和
//pending logs that match the given criteria.
func (es *EventSystem) subscribeMinedPendingLogs(crit ethereum.FilterQuery, logs chan []*types.Log) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       MinedAndPendingLogsSubscription,
		logsCrit:  crit,
		created:   time.Now(),
		logs:      logs,
		hashes:    make(chan []common.Hash),
		headers:   make(chan *types.Header),
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

//subscriptLogs创建一个订阅，该订阅将写入与
//给定日志通道的给定条件。
func (es *EventSystem) subscribeLogs(crit ethereum.FilterQuery, logs chan []*types.Log) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       LogsSubscription,
		logsCrit:  crit,
		created:   time.Now(),
		logs:      logs,
		hashes:    make(chan []common.Hash),
		headers:   make(chan *types.Header),
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

//subscribePendingLogs创建一个订阅，该订阅为
//进入事务池的事务。
func (es *EventSystem) subscribePendingLogs(crit ethereum.FilterQuery, logs chan []*types.Log) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       PendingLogsSubscription,
		logsCrit:  crit,
		created:   time.Now(),
		logs:      logs,
		hashes:    make(chan []common.Hash),
		headers:   make(chan *types.Header),
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

//SUBSCRIBENEWEADS创建一个订阅，该订阅写入的块头为
//进口链。
func (es *EventSystem) SubscribeNewHeads(headers chan *types.Header) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       BlocksSubscription,
		created:   time.Now(),
		logs:      make(chan []*types.Log),
		hashes:    make(chan []common.Hash),
		headers:   headers,
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

//订阅BuffixTxs创建一个订阅事务哈希的订阅。
//进入事务池的事务。
func (es *EventSystem) SubscribePendingTxs(hashes chan []common.Hash) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       PendingTransactionsSubscription,
		created:   time.Now(),
		logs:      make(chan []*types.Log),
		hashes:    hashes,
		headers:   make(chan *types.Header),
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

type filterIndex map[Type]map[rpc.ID]*subscription

//将事件广播到符合条件的筛选器。
func (es *EventSystem) broadcast(filters filterIndex, ev interface{}) {
	if ev == nil {
		return
	}

	switch e := ev.(type) {
	case []*types.Log:
		if len(e) > 0 {
			for _, f := range filters[LogsSubscription] {
				if matchedLogs := filterLogs(e, f.logsCrit.FromBlock, f.logsCrit.ToBlock, f.logsCrit.Addresses, f.logsCrit.Topics); len(matchedLogs) > 0 {
					f.logs <- matchedLogs
				}
			}
		}
	case core.RemovedLogsEvent:
		for _, f := range filters[LogsSubscription] {
			if matchedLogs := filterLogs(e.Logs, f.logsCrit.FromBlock, f.logsCrit.ToBlock, f.logsCrit.Addresses, f.logsCrit.Topics); len(matchedLogs) > 0 {
				f.logs <- matchedLogs
			}
		}
	case *event.TypeMuxEvent:
		if muxe, ok := e.Data.(core.PendingLogsEvent); ok {
			for _, f := range filters[PendingLogsSubscription] {
				if e.Time.After(f.created) {
					if matchedLogs := filterLogs(muxe.Logs, nil, f.logsCrit.ToBlock, f.logsCrit.Addresses, f.logsCrit.Topics); len(matchedLogs) > 0 {
						f.logs <- matchedLogs
					}
				}
			}
		}
	case core.NewTxsEvent:
		hashes := make([]common.Hash, 0, len(e.Txs))
		for _, tx := range e.Txs {
			hashes = append(hashes, tx.Hash())
		}
		for _, f := range filters[PendingTransactionsSubscription] {
			f.hashes <- hashes
		}
	case core.ChainEvent:
		for _, f := range filters[BlocksSubscription] {
			f.headers <- e.Block.Header()
		}
		if es.lightMode && len(filters[LogsSubscription]) > 0 {
			es.lightFilterNewHead(e.Block.Header(), func(header *types.Header, remove bool) {
				for _, f := range filters[LogsSubscription] {
					if matchedLogs := es.lightFilterLogs(header, f.logsCrit.Addresses, f.logsCrit.Topics, remove); len(matchedLogs) > 0 {
						f.logs <- matchedLogs
					}
				}
			})
		}
	}
}

func (es *EventSystem) lightFilterNewHead(newHeader *types.Header, callBack func(*types.Header, bool)) {
	oldh := es.lastHead
	es.lastHead = newHeader
	if oldh == nil {
		return
	}
	newh := newHeader
//find common ancestor, create list of rolled back and new block hashes
	var oldHeaders, newHeaders []*types.Header
	for oldh.Hash() != newh.Hash() {
		if oldh.Number.Uint64() >= newh.Number.Uint64() {
			oldHeaders = append(oldHeaders, oldh)
			oldh = rawdb.ReadHeader(es.backend.ChainDb(), oldh.ParentHash, oldh.Number.Uint64()-1)
		}
		if oldh.Number.Uint64() < newh.Number.Uint64() {
			newHeaders = append(newHeaders, newh)
			newh = rawdb.ReadHeader(es.backend.ChainDb(), newh.ParentHash, newh.Number.Uint64()-1)
			if newh == nil {
//当CHT同步时发生，无需执行任何操作
				newh = oldh
			}
		}
	}
//回滚旧块
	for _, h := range oldHeaders {
		callBack(h, true)
	}
//检查新块（数组顺序相反）
	for i := len(newHeaders) - 1; i >= 0; i-- {
		callBack(newHeaders[i], false)
	}
}

//在轻型客户端模式下筛选单个头的日志
func (es *EventSystem) lightFilterLogs(header *types.Header, addresses []common.Address, topics [][]common.Hash, remove bool) []*types.Log {
	if bloomFilter(header.Bloom, addresses, topics) {
//获取块的日志
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		logsList, err := es.backend.GetLogs(ctx, header.Hash())
		if err != nil {
			return nil
		}
		var unfiltered []*types.Log
		for _, logs := range logsList {
			for _, log := range logs {
				logcopy := *log
				logcopy.Removed = remove
				unfiltered = append(unfiltered, &logcopy)
			}
		}
		logs := filterLogs(unfiltered, nil, nil, addresses, topics)
		if len(logs) > 0 && logs[0].TxHash == (common.Hash{}) {
//我们有匹配但非派生的日志
			receipts, err := es.backend.GetReceipts(ctx, header.Hash())
			if err != nil {
				return nil
			}
			unfiltered = unfiltered[:0]
			for _, receipt := range receipts {
				for _, log := range receipt.Logs {
					logcopy := *log
					logcopy.Removed = remove
					unfiltered = append(unfiltered, &logcopy)
				}
			}
			logs = filterLogs(unfiltered, nil, nil, addresses, topics)
		}
		return logs
	}
	return nil
}

//EventLoop（un）安装过滤器并处理mux事件。
func (es *EventSystem) eventLoop() {
//确保清除所有订阅
	defer func() {
		es.pendingLogSub.Unsubscribe()
		es.txsSub.Unsubscribe()
		es.logsSub.Unsubscribe()
		es.rmLogsSub.Unsubscribe()
		es.chainSub.Unsubscribe()
	}()

	index := make(filterIndex)
	for i := UnknownSubscription; i < LastIndexSubscription; i++ {
		index[i] = make(map[rpc.ID]*subscription)
	}

	for {
		select {
//处理订阅的事件
		case ev := <-es.txsCh:
			es.broadcast(index, ev)
		case ev := <-es.logsCh:
			es.broadcast(index, ev)
		case ev := <-es.rmLogsCh:
			es.broadcast(index, ev)
		case ev := <-es.chainCh:
			es.broadcast(index, ev)
		case ev, active := <-es.pendingLogSub.Chan():
if !active { //系统停止
				return
			}
			es.broadcast(index, ev)

		case f := <-es.install:
			if f.typ == MinedAndPendingLogsSubscription {
//类型为日志和挂起的日志订阅
				index[LogsSubscription][f.id] = f
				index[PendingLogsSubscription][f.id] = f
			} else {
				index[f.typ][f.id] = f
			}
			close(f.installed)

		case f := <-es.uninstall:
			if f.typ == MinedAndPendingLogsSubscription {
//类型为日志和挂起的日志订阅
				delete(index[LogsSubscription], f.id)
				delete(index[PendingLogsSubscription], f.id)
			} else {
				delete(index[f.typ], f.id)
			}
			close(f.err)

//系统停止
		case <-es.txsSub.Err():
			return
		case <-es.logsSub.Err():
			return
		case <-es.rmLogsSub.Err():
			return
		case <-es.chainSub.Err():
			return
		}
	}
}
