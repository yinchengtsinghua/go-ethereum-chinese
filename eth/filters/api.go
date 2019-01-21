
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

package filters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
deadline = 5 * time.Minute //如果在截止日期内未对某个筛选器进行轮询，则认为该筛选器处于非活动状态
)

//filter是一个在filter类型上保存元信息的helper结构
//以及事件系统中的关联订阅。
type filter struct {
	typ      Type
deadline *time.Timer //截止时间触发时，筛选器不活动
	hashes   []common.Hash
	crit     FilterCriteria
	logs     []*types.Log
s        *Subscription //事件系统中的关联订阅
}

//publicfilterapi提供创建和管理过滤器的支持。这将允许外部客户端检索
//与以太坊协议相关的信息，如ALS块、事务和日志。
type PublicFilterAPI struct {
	backend   Backend
	mux       *event.TypeMux
	quit      chan struct{}
	chainDb   ethdb.Database
	events    *EventSystem
	filtersMu sync.Mutex
	filters   map[rpc.ID]*filter
}

//new publicfilterapi返回新的publicfilterapi实例。
func NewPublicFilterAPI(backend Backend, lightMode bool) *PublicFilterAPI {
	api := &PublicFilterAPI{
		backend: backend,
		mux:     backend.EventMux(),
		chainDb: backend.ChainDb(),
		events:  NewEventSystem(backend.EventMux(), backend, lightMode),
		filters: make(map[rpc.ID]*filter),
	}
	go api.timeoutLoop()

	return api
}

//TimeoutLoop每5分钟运行一次，并删除最近未使用的筛选器。
//TT在创建API时启动。
func (api *PublicFilterAPI) timeoutLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	for {
		<-ticker.C
		api.filtersMu.Lock()
		for id, f := range api.filters {
			select {
			case <-f.deadline.C:
				f.s.Unsubscribe()
				delete(api.filters, id)
			default:
				continue
			}
		}
		api.filtersMu.Unlock()
	}
}

//NewPendingtTransactionFilter创建一个筛选器，用于获取挂起的事务哈希
//当事务进入挂起状态时。
//
//它是筛选器包的一部分，因为此筛选器可以通过
//“eth”getfilterchanges“轮询方法，也用于日志筛选器。
//
//https://github.com/ethereum/wiki/wiki/json-rpc_eth_newpendingtransactionfilter
func (api *PublicFilterAPI) NewPendingTransactionFilter() rpc.ID {
	var (
		pendingTxs   = make(chan []common.Hash)
		pendingTxSub = api.events.SubscribePendingTxs(pendingTxs)
	)

	api.filtersMu.Lock()
	api.filters[pendingTxSub.ID] = &filter{typ: PendingTransactionsSubscription, deadline: time.NewTimer(deadline), hashes: make([]common.Hash, 0), s: pendingTxSub}
	api.filtersMu.Unlock()

	go func() {
		for {
			select {
			case ph := <-pendingTxs:
				api.filtersMu.Lock()
				if f, found := api.filters[pendingTxSub.ID]; found {
					f.hashes = append(f.hashes, ph...)
				}
				api.filtersMu.Unlock()
			case <-pendingTxSub.Err():
				api.filtersMu.Lock()
				delete(api.filters, pendingTxSub.ID)
				api.filtersMu.Unlock()
				return
			}
		}
	}()

	return pendingTxSub.ID
}

//NewPendingtTransactions创建每次事务触发的订阅
//进入事务池，并从该节点管理的事务之一签名。
func (api *PublicFilterAPI) NewPendingTransactions(ctx context.Context) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		txHashes := make(chan []common.Hash, 128)
		pendingTxSub := api.events.SubscribePendingTxs(txHashes)

		for {
			select {
			case hashes := <-txHashes:
//要保持原始行为，请在一个通知中发送单个Tx哈希。
//TODO（RJL493456442）在一个通知中发送一批Tx哈希
				for _, h := range hashes {
					notifier.Notify(rpcSub.ID, h)
				}
			case <-rpcSub.Err():
				pendingTxSub.Unsubscribe()
				return
			case <-notifier.Closed():
				pendingTxSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

//newblockfilter创建一个过滤器，用于获取导入到链中的块。
//它是筛选包的一部分，因为轮询与ethgetfilterchanges一起进行。
//
//https://github.com/ethereum/wiki/wiki/json-rpc eth newblockfilter
func (api *PublicFilterAPI) NewBlockFilter() rpc.ID {
	var (
		headers   = make(chan *types.Header)
		headerSub = api.events.SubscribeNewHeads(headers)
	)

	api.filtersMu.Lock()
	api.filters[headerSub.ID] = &filter{typ: BlocksSubscription, deadline: time.NewTimer(deadline), hashes: make([]common.Hash, 0), s: headerSub}
	api.filtersMu.Unlock()

	go func() {
		for {
			select {
			case h := <-headers:
				api.filtersMu.Lock()
				if f, found := api.filters[headerSub.ID]; found {
					f.hashes = append(f.hashes, h.Hash())
				}
				api.filtersMu.Unlock()
			case <-headerSub.Err():
				api.filtersMu.Lock()
				delete(api.filters, headerSub.ID)
				api.filtersMu.Unlock()
				return
			}
		}
	}()

	return headerSub.ID
}

//每当新（header）块附加到链时，newheads都会发送通知。
func (api *PublicFilterAPI) NewHeads(ctx context.Context) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		headers := make(chan *types.Header)
		headersSub := api.events.SubscribeNewHeads(headers)

		for {
			select {
			case h := <-headers:
				notifier.Notify(rpcSub.ID, h)
			case <-rpcSub.Err():
				headersSub.Unsubscribe()
				return
			case <-notifier.Closed():
				headersSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

//日志创建一个订阅，该订阅为所有符合给定筛选条件的新日志激发。
func (api *PublicFilterAPI) Logs(ctx context.Context, crit FilterCriteria) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	var (
		rpcSub      = notifier.CreateSubscription()
		matchedLogs = make(chan []*types.Log)
	)

	logsSub, err := api.events.SubscribeLogs(ethereum.FilterQuery(crit), matchedLogs)
	if err != nil {
		return nil, err
	}

	go func() {

		for {
			select {
			case logs := <-matchedLogs:
				for _, log := range logs {
					notifier.Notify(rpcSub.ID, &log)
				}
case <-rpcSub.Err(): //客户端发送取消订阅请求
				logsSub.Unsubscribe()
				return
case <-notifier.Closed(): //连接已断开
				logsSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

//FilterCriteria表示创建新筛选器的请求。
//与ethereum.filterquery相同，但使用unmashaljson（）方法。
type FilterCriteria ethereum.FilterQuery

//new filter创建新的筛选器并返回筛选器ID。它可以是
//用于在状态更改时检索日志。此方法不能
//用于获取已存储在状态中的日志。
//
//“从”和“到”块的默认条件为“最新”。
//使用“最新”作为块号将返回已开采块的日志。
//使用“挂起”作为块号返回尚未挖掘（挂起）块的日志。
//如果日志被删除（链ReRoG），则返回以前返回的日志。
//再次，但将移除的属性设置为true。
//
//如果“fromblock”>toblock，则返回错误。
//
//https://github.com/ethereum/wiki/wiki/json-rpc_eth_newfilter
func (api *PublicFilterAPI) NewFilter(crit FilterCriteria) (rpc.ID, error) {
	logs := make(chan []*types.Log)
	logsSub, err := api.events.SubscribeLogs(ethereum.FilterQuery(crit), logs)
	if err != nil {
		return rpc.ID(""), err
	}

	api.filtersMu.Lock()
	api.filters[logsSub.ID] = &filter{typ: LogsSubscription, crit: crit, deadline: time.NewTimer(deadline), logs: make([]*types.Log, 0), s: logsSub}
	api.filtersMu.Unlock()

	go func() {
		for {
			select {
			case l := <-logs:
				api.filtersMu.Lock()
				if f, found := api.filters[logsSub.ID]; found {
					f.logs = append(f.logs, l...)
				}
				api.filtersMu.Unlock()
			case <-logsSub.Err():
				api.filtersMu.Lock()
				delete(api.filters, logsSub.ID)
				api.filtersMu.Unlock()
				return
			}
		}
	}()

	return logsSub.ID, nil
}

//GetLogs返回与存储在状态中的给定参数匹配的日志。
//
//https://github.com/ethereum/wiki/wiki/json-rpc eth getlogs
func (api *PublicFilterAPI) GetLogs(ctx context.Context, crit FilterCriteria) ([]*types.Log, error) {
	var filter *Filter
	if crit.BlockHash != nil {
//请求块筛选器，构造一个单镜头筛选器
		filter = NewBlockFilter(api.backend, *crit.BlockHash, crit.Addresses, crit.Topics)
	} else {
//将RPC块编号转换为内部表示形式
		begin := rpc.LatestBlockNumber.Int64()
		if crit.FromBlock != nil {
			begin = crit.FromBlock.Int64()
		}
		end := rpc.LatestBlockNumber.Int64()
		if crit.ToBlock != nil {
			end = crit.ToBlock.Int64()
		}
//构造范围过滤器
		filter = NewRangeFilter(api.backend, begin, end, crit.Addresses, crit.Topics)
	}
//运行过滤器并返回所有日志
	logs, err := filter.Logs(ctx)
	if err != nil {
		return nil, err
	}
	return returnLogs(logs), err
}

//uninstallfilter删除具有给定筛选器ID的筛选器。
//
//https://github.com/ethereum/wiki/wiki/JSON-RPC#eth_uninstallfilter
func (api *PublicFilterAPI) UninstallFilter(id rpc.ID) bool {
	api.filtersMu.Lock()
	f, found := api.filters[id]
	if found {
		delete(api.filters, id)
	}
	api.filtersMu.Unlock()
	if found {
		f.s.Unsubscribe()
	}

	return found
}

//GetFilterLogs返回具有给定ID的筛选器的日志。
//如果找不到筛选器，则返回一个空的日志数组。
//
//https://github.com/ethereum/wiki/wiki/json-rpc_eth_getfilterlogs
func (api *PublicFilterAPI) GetFilterLogs(ctx context.Context, id rpc.ID) ([]*types.Log, error) {
	api.filtersMu.Lock()
	f, found := api.filters[id]
	api.filtersMu.Unlock()

	if !found || f.typ != LogsSubscription {
		return nil, fmt.Errorf("filter not found")
	}

	var filter *Filter
	if f.crit.BlockHash != nil {
//请求块筛选器，构造一个单镜头筛选器
		filter = NewBlockFilter(api.backend, *f.crit.BlockHash, f.crit.Addresses, f.crit.Topics)
	} else {
//将RPC块编号转换为内部表示形式
		begin := rpc.LatestBlockNumber.Int64()
		if f.crit.FromBlock != nil {
			begin = f.crit.FromBlock.Int64()
		}
		end := rpc.LatestBlockNumber.Int64()
		if f.crit.ToBlock != nil {
			end = f.crit.ToBlock.Int64()
		}
//构造范围过滤器
		filter = NewRangeFilter(api.backend, begin, end, f.crit.Addresses, f.crit.Topics)
	}
//运行过滤器并返回所有日志
	logs, err := filter.Logs(ctx)
	if err != nil {
		return nil, err
	}
	return returnLogs(logs), nil
}

//GetFilterChanges返回具有给定ID的筛选器的日志，自
//上次打电话的时候。这可以用于轮询。
//
//对于挂起的事务和块筛选器，结果是[]common.hash。
//（挂起）日志筛选器返回[]日志。
//
//https://github.com/ethereum/wiki/wiki/json-rpc_eth_getfilterchanges
func (api *PublicFilterAPI) GetFilterChanges(id rpc.ID) (interface{}, error) {
	api.filtersMu.Lock()
	defer api.filtersMu.Unlock()

	if f, found := api.filters[id]; found {
		if !f.deadline.Stop() {
//计时器已过期，但在超时循环中尚未删除筛选器
//接收计时器值并重置计时器
			<-f.deadline.C
		}
		f.deadline.Reset(deadline)

		switch f.typ {
		case PendingTransactionsSubscription, BlocksSubscription:
			hashes := f.hashes
			f.hashes = nil
			return returnHashes(hashes), nil
		case LogsSubscription:
			logs := f.logs
			f.logs = nil
			return returnLogs(logs), nil
		}
	}

	return []interface{}{}, fmt.Errorf("filter not found")
}

//ReturnHashes是一个助手，在给定哈希数组为零的情况下将返回空哈希数组，
//否则返回给定的哈希数组。
func returnHashes(hashes []common.Hash) []common.Hash {
	if hashes == nil {
		return []common.Hash{}
	}
	return hashes
}

//ReturnLogs是一个帮助程序，当给定的日志数组为零时，它将返回空的日志数组。
//否则返回给定的logs数组。
func returnLogs(logs []*types.Log) []*types.Log {
	if logs == nil {
		return []*types.Log{}
	}
	return logs
}

//用给定的数据取消标记JSON集合*参数字段。
func (args *FilterCriteria) UnmarshalJSON(data []byte) error {
	type input struct {
		BlockHash *common.Hash     `json:"blockHash"`
		FromBlock *rpc.BlockNumber `json:"fromBlock"`
		ToBlock   *rpc.BlockNumber `json:"toBlock"`
		Addresses interface{}      `json:"address"`
		Topics    []interface{}    `json:"topics"`
	}

	var raw input
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.BlockHash != nil {
		if raw.FromBlock != nil || raw.ToBlock != nil {
//blockhash与fromblock/toblock条件互斥
			return fmt.Errorf("cannot specify both BlockHash and FromBlock/ToBlock, choose one or the other")
		}
		args.BlockHash = raw.BlockHash
	} else {
		if raw.FromBlock != nil {
			args.FromBlock = big.NewInt(raw.FromBlock.Int64())
		}

		if raw.ToBlock != nil {
			args.ToBlock = big.NewInt(raw.ToBlock.Int64())
		}
	}

	args.Addresses = []common.Address{}

	if raw.Addresses != nil {
//原始地址可以包含单个地址或地址数组
		switch rawAddr := raw.Addresses.(type) {
		case []interface{}:
			for i, addr := range rawAddr {
				if strAddr, ok := addr.(string); ok {
					addr, err := decodeAddress(strAddr)
					if err != nil {
						return fmt.Errorf("invalid address at index %d: %v", i, err)
					}
					args.Addresses = append(args.Addresses, addr)
				} else {
					return fmt.Errorf("non-string address at index %d", i)
				}
			}
		case string:
			addr, err := decodeAddress(rawAddr)
			if err != nil {
				return fmt.Errorf("invalid address: %v", err)
			}
			args.Addresses = []common.Address{addr}
		default:
			return errors.New("invalid addresses in query")
		}
	}

//主题是由字符串和/或字符串数组组成的数组。
//JSON空值转换为common.hash并被筛选器管理器忽略。
	if len(raw.Topics) > 0 {
		args.Topics = make([][]common.Hash, len(raw.Topics))
		for i, t := range raw.Topics {
			switch topic := t.(type) {
			case nil:
//匹配日志时忽略主题

			case string:
//匹配特定主题
				top, err := decodeTopic(topic)
				if err != nil {
					return err
				}
				args.Topics[i] = []common.Hash{top}

			case []interface{}:
//或案例，例如[空，“topic0”，“topic1”]
				for _, rawTopic := range topic {
					if rawTopic == nil {
//空组件，全部匹配
						args.Topics[i] = nil
						break
					}
					if topic, ok := rawTopic.(string); ok {
						parsed, err := decodeTopic(topic)
						if err != nil {
							return err
						}
						args.Topics[i] = append(args.Topics[i], parsed)
					} else {
						return fmt.Errorf("invalid topic(s)")
					}
				}
			default:
				return fmt.Errorf("invalid topic(s)")
			}
		}
	}

	return nil
}

func decodeAddress(s string) (common.Address, error) {
	b, err := hexutil.Decode(s)
	if err == nil && len(b) != common.AddressLength {
		err = fmt.Errorf("hex has invalid length %d after decoding; expected %d for address", len(b), common.AddressLength)
	}
	return common.BytesToAddress(b), err
}

func decodeTopic(s string) (common.Hash, error) {
	b, err := hexutil.Decode(s)
	if err == nil && len(b) != common.HashLength {
		err = fmt.Errorf("hex has invalid length %d after decoding; expected %d for topic", len(b), common.HashLength)
	}
	return common.BytesToHash(b), err
}
