
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

package core

import (
	"container/heap"
	"math"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

//nonceheap是堆。接口实现超过64位无符号整数
//从可能有间隙的未来队列中检索已排序的事务。
type nonceHeap []uint64

func (h nonceHeap) Len() int           { return len(h) }
func (h nonceHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h nonceHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *nonceHeap) Push(x interface{}) {
	*h = append(*h, x.(uint64))
}

func (h *nonceHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

//txsortedmap是一个nonce->transaction哈希映射，具有基于堆的索引，允许
//以非递增方式迭代内容。
type txSortedMap struct {
items map[uint64]*types.Transaction //存储事务数据的哈希图
index *nonceHeap                    //所有存储事务的当前堆（非严格模式）
cache types.Transactions            //缓存已排序的事务
}

//newtxsortedmap创建新的非ce排序事务映射。
func newTxSortedMap() *txSortedMap {
	return &txSortedMap{
		items: make(map[uint64]*types.Transaction),
		index: new(nonceHeap),
	}
}

//get检索与给定nonce关联的当前事务。
func (m *txSortedMap) Get(nonce uint64) *types.Transaction {
	return m.items[nonce]
}

//在映射中插入新事务，同时更新映射的nonce
//索引。如果已存在具有相同nonce的事务，则将覆盖该事务。
func (m *txSortedMap) Put(tx *types.Transaction) {
	nonce := tx.Nonce()
	if m.items[nonce] == nil {
		heap.Push(m.index, nonce)
	}
	m.items[nonce], m.cache = tx, nil
}

//forward从映射中删除所有事务，其中nonce小于
//提供阈值。对于任何删除后的事务，都会返回每个已删除的事务。
//维护。
func (m *txSortedMap) Forward(threshold uint64) types.Transactions {
	var removed types.Transactions

//弹出堆项，直到达到阈值
	for m.index.Len() > 0 && (*m.index)[0] < threshold {
		nonce := heap.Pop(m.index).(uint64)
		removed = append(removed, m.items[nonce])
		delete(m.items, nonce)
	}
//如果我们有一个缓存的订单，转移前面
	if m.cache != nil {
		m.cache = m.cache[len(removed):]
	}
	return removed
}

//筛选器迭代事务列表并删除其中所有
//指定函数的计算结果为true。
func (m *txSortedMap) Filter(filter func(*types.Transaction) bool) types.Transactions {
	var removed types.Transactions

//收集所有事务以筛选出
	for nonce, tx := range m.items {
		if filter(tx) {
			removed = append(removed, tx)
			delete(m.items, nonce)
		}
	}
//如果删除了事务，则会破坏堆和缓存
	if len(removed) > 0 {
		*m.index = make([]uint64, 0, len(m.items))
		for nonce := range m.items {
			*m.index = append(*m.index, nonce)
		}
		heap.Init(m.index)

		m.cache = nil
	}
	return removed
}

//cap对项目数进行硬限制，返回所有事务
//超过那个限度。
func (m *txSortedMap) Cap(threshold int) types.Transactions {
//项目数量低于限制时短路
	if len(m.items) <= threshold {
		return nil
	}
//否则，收集并删除最高的非ce'd事务
	var drops types.Transactions

	sort.Sort(*m.index)
	for size := len(m.items); size > threshold; size-- {
		drops = append(drops, m.items[(*m.index)[size-1]])
		delete(m.items, (*m.index)[size-1])
	}
	*m.index = (*m.index)[:threshold]
	heap.Init(m.index)

//如果我们有一个缓存，把它移到后面
	if m.cache != nil {
		m.cache = m.cache[:len(m.cache)-len(drops)]
	}
	return drops
}

//移除从维护的映射中删除事务，返回
//找到事务。
func (m *txSortedMap) Remove(nonce uint64) bool {
//无交易时短路
	_, ok := m.items[nonce]
	if !ok {
		return false
	}
//否则，删除事务并修复堆索引
	for i := 0; i < m.index.Len(); i++ {
		if (*m.index)[i] == nonce {
			heap.Remove(m.index, i)
			break
		}
	}
	delete(m.items, nonce)
	m.cache = nil

	return true
}

//Ready retrieves a sequentially increasing list of transactions starting at the
//提供了一个准备好进行处理的nonce。返回的事务将
//已从列表中删除。
//
//注意，所有非起始值低于起始值的事务也将返回到
//防止进入无效状态。这不是应该有的事
//发生但最好是自我纠正而不是失败！
func (m *txSortedMap) Ready(start uint64) types.Transactions {
//如果没有可用的交易，则短路
	if m.index.Len() == 0 || (*m.index)[0] > start {
		return nil
	}
//否则开始累积增量事务
	var ready types.Transactions
	for next := (*m.index)[0]; m.index.Len() > 0 && (*m.index)[0] == next; next++ {
		ready = append(ready, m.items[next])
		delete(m.items, next)
		heap.Pop(m.index)
	}
	m.cache = nil

	return ready
}

//len返回事务映射的长度。
func (m *txSortedMap) Len() int {
	return len(m.items)
}

//Flatten基于松散的
//已排序的内部表示。排序结果缓存在
//在对内容进行任何修改之前，需要再次修改。
func (m *txSortedMap) Flatten() types.Transactions {
//如果排序尚未缓存，请创建并缓存排序
	if m.cache == nil {
		m.cache = make(types.Transactions, 0, len(m.items))
		for _, tx := range m.items {
			m.cache = append(m.cache, tx)
		}
		sort.Sort(types.TxByNonce(m.cache))
	}
//复制缓存以防止意外修改
	txs := make(types.Transactions, len(m.cache))
	copy(txs, m.cache)
	return txs
}

//txlist是属于一个账户的交易的“列表”，按账户排序。
//临时的同一类型可用于存储
//可执行/挂起队列；用于存储非-
//可执行/将来的队列，有轻微的行为更改。
type txList struct {
strict bool         //是否严格连续
txs    *txSortedMap //事务的堆索引排序哈希图

costcap *big.Int //最高成本核算交易记录的价格（仅当超过余额时重置）
gascap  uint64   //最高支出交易的气体限额（仅在超过区块限额时重置）
}

//newtxlist创建一个新的事务列表，用于快速维护非索引，
//有间隙、可排序的事务列表。
func newTxList(strict bool) *txList {
	return &txList{
		strict:  strict,
		txs:     newTxSortedMap(),
		costcap: new(big.Int),
	}
}

//overlaps返回指定的事务是否与一个事务具有相同的nonce
//已经包含在列表中。
func (l *txList) Overlaps(tx *types.Transaction) bool {
	return l.txs.Get(tx.Nonce()) != nil
}

//add尝试将新事务插入列表，返回
//交易已被接受，如果是，则替换以前的任何交易。
//
//如果新交易被接受到清单中，清单的成本和天然气
//阈值也可能更新。
func (l *txList) Add(tx *types.Transaction, priceBump uint64) (bool, *types.Transaction) {
//如果有旧的更好的事务，请中止
	old := l.txs.Get(tx.Nonce())
	if old != nil {
		threshold := new(big.Int).Div(new(big.Int).Mul(old.GasPrice(), big.NewInt(100+int64(priceBump))), big.NewInt(100))
//必须确保新的天然气价格高于旧的天然气价格
//价格以及检查百分比阈值以确保
//这对于低（wei级）天然气价格的替代品是准确的。
		if old.GasPrice().Cmp(tx.GasPrice()) >= 0 || threshold.Cmp(tx.GasPrice()) > 0 {
			return false, nil
		}
	}
//否则，用当前事务覆盖旧事务
	l.txs.Put(tx)
	if cost := tx.Cost(); l.costcap.Cmp(cost) < 0 {
		l.costcap = cost
	}
	if gas := tx.Gas(); l.gascap < gas {
		l.gascap = gas
	}
	return true, old
}

//Forward从列表中删除所有事务，其中一个nonce低于
//提供阈值。对于任何删除后的事务，都会返回每个已删除的事务。
//维护。
func (l *txList) Forward(threshold uint64) types.Transactions {
	return l.txs.Forward(threshold)
}

//过滤器从列表中删除成本或气体限制更高的所有事务
//超过提供的阈值。对于任何
//拆卸后维护。严格模式失效的事务也
//返回。
//
//此方法使用缓存的CostCap和GasCap快速确定
//计算所有成本的一个点，或者如果余额覆盖了所有成本。如果门槛
//低于costgas上限，移除后上限将重置为新的上限
//新失效的交易。
func (l *txList) Filter(costLimit *big.Int, gasLimit uint64) (types.Transactions, types.Transactions) {
//如果所有事务低于阈值，则短路
	if l.costcap.Cmp(costLimit) <= 0 && l.gascap <= gasLimit {
		return nil, nil
	}
l.costcap = new(big.Int).Set(costLimit) //将上限降低到阈值
	l.gascap = gasLimit

//过滤掉账户资金上方的所有交易
	removed := l.txs.Filter(func(tx *types.Transaction) bool { return tx.Cost().Cmp(costLimit) > 0 || tx.Gas() > gasLimit })

//如果列表是严格的，则筛选高于最低当前值的任何内容
	var invalids types.Transactions

	if l.strict && len(removed) > 0 {
		lowest := uint64(math.MaxUint64)
		for _, tx := range removed {
			if nonce := tx.Nonce(); lowest > nonce {
				lowest = nonce
			}
		}
		invalids = l.txs.Filter(func(tx *types.Transaction) bool { return tx.Nonce() > lowest })
	}
	return removed, invalids
}

//cap对项目数进行硬限制，返回所有事务
//超过那个限度。
func (l *txList) Cap(threshold int) types.Transactions {
	return l.txs.Cap(threshold)
}

//移除从维护列表中删除事务，返回
//找到交易，并返回因以下原因而失效的任何交易
//删除（仅限严格模式）。
func (l *txList) Remove(tx *types.Transaction) (bool, types.Transactions) {
//从集合中移除事务
	nonce := tx.Nonce()
	if removed := l.txs.Remove(nonce); !removed {
		return false, nil
	}
//在严格模式下，筛选出不可执行的事务
	if l.strict {
		return true, l.txs.Filter(func(tx *types.Transaction) bool { return tx.Nonce() > nonce })
	}
	return true, nil
}

//就绪检索从
//提供了一个准备好进行处理的nonce。返回的事务将
//已从列表中删除。
//
//注意，所有非起始值低于起始值的事务也将返回到
//防止进入无效状态。这不是应该有的事
//发生但最好是自我纠正而不是失败！
func (l *txList) Ready(start uint64) types.Transactions {
	return l.txs.Ready(start)
}

//len返回事务列表的长度。
func (l *txList) Len() int {
	return l.txs.Len()
}

//empty返回事务列表是否为空。
func (l *txList) Empty() bool {
	return l.Len() == 0
}

//Flatten基于松散的
//已排序的内部表示。排序结果缓存在
//在对内容进行任何修改之前，需要再次修改。
func (l *txList) Flatten() types.Transactions {
	return l.txs.Flatten()
}

//PriceHeap是一个堆。用于检索的事务的接口实现
//池满时要丢弃的按价格排序的交易记录。
type priceHeap []*types.Transaction

func (h priceHeap) Len() int      { return len(h) }
func (h priceHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h priceHeap) Less(i, j int) bool {
//主要按价格排序，返回较便宜的
	switch h[i].GasPrice().Cmp(h[j].GasPrice()) {
	case -1:
		return true
	case 1:
		return false
	}
//如果价格匹配，通过nonce稳定（高nonce更糟）
	return h[i].Nonce() > h[j].Nonce()
}

func (h *priceHeap) Push(x interface{}) {
	*h = append(*h, x.(*types.Transaction))
}

func (h *priceHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

//txPricedList是一个价格排序堆，允许对事务池进行操作
//价格递增的内容。
type txPricedList struct {
all    *txLookup  //指向所有事务映射的指针
items  *priceHeap //所有存储事务的价格堆
stales int        //失效价格点的数目（重新堆触发器）
}

//newtxPricedList创建一个新的按价格排序的事务堆。
func newTxPricedList(all *txLookup) *txPricedList {
	return &txPricedList{
		all:   all,
		items: new(priceHeap),
	}
}

//PUT向堆中插入新事务。
func (l *txPricedList) Put(tx *types.Transaction) {
	heap.Push(l.items, tx)
}

//已删除通知价格事务列表旧事务已删除
//从游泳池。列表只保留过时对象的计数器并更新
//如果足够大的事务比率过时，则为堆。
func (l *txPricedList) Removed() {
//撞击陈旧的计数器，但如果仍然过低（<25%），则退出。
	l.stales++
	if l.stales <= len(*l.items)/4 {
		return
	}
//似乎我们已经达到了一个关键的陈旧的交易数量，reheap
	reheap := make(priceHeap, 0, l.all.Count())

	l.stales, l.items = 0, &reheap
	l.all.Range(func(hash common.Hash, tx *types.Transaction) bool {
		*l.items = append(*l.items, tx)
		return true
	})
	heap.Init(l.items)
}

//cap查找低于给定价格阈值的所有交易，并将其删除
//从定价列表中返回它们以便从整个池中进一步删除。
func (l *txPricedList) Cap(threshold *big.Int, local *accountSet) types.Transactions {
drop := make(types.Transactions, 0, 128) //要删除的远程低价交易
save := make(types.Transactions, 0, 64)  //要保留的本地定价过低交易

	for len(*l.items) > 0 {
//如果在清理过程中发现过时的事务，则放弃这些事务
		tx := heap.Pop(l.items).(*types.Transaction)
		if l.all.Get(tx.Hash()) == nil {
			l.stales--
			continue
		}
//如果我们达到了临界值，就停止丢弃
		if tx.GasPrice().Cmp(threshold) >= 0 {
			save = append(save, tx)
			break
		}
//找到未过期的事务，除非本地
		if local.containsTx(tx) {
			save = append(save, tx)
		} else {
			drop = append(drop, tx)
		}
	}
	for _, tx := range save {
		heap.Push(l.items, tx)
	}
	return drop
}

//低价检查交易是否比
//当前正在跟踪的最低价格交易记录。
func (l *txPricedList) Underpriced(tx *types.Transaction, local *accountSet) bool {
//本地交易不能定价过低
	if local.containsTx(tx) {
		return false
	}
//如果在堆开始处找到过时的价格点，则丢弃它们
	for len(*l.items) > 0 {
		head := []*types.Transaction(*l.items)[0]
		if l.all.Get(head.Hash()) == nil {
			l.stales--
			heap.Pop(l.items)
			continue
		}
		break
	}
//检查交易是否定价过低
	if len(*l.items) == 0 {
log.Error("Pricing query for empty pool") //这不可能发生，打印以捕获编程错误
		return false
	}
	cheapest := []*types.Transaction(*l.items)[0]
	return cheapest.GasPrice().Cmp(tx.GasPrice()) >= 0
}

//Discard查找许多定价最低的事务，将它们从
//并返回它们以便从整个池中进一步删除。
func (l *txPricedList) Discard(count int, local *accountSet) types.Transactions {
drop := make(types.Transactions, 0, count) //要删除的远程低价交易
save := make(types.Transactions, 0, 64)    //要保留的本地定价过低交易

	for len(*l.items) > 0 && count > 0 {
//如果在清理过程中发现过时的事务，则放弃这些事务
		tx := heap.Pop(l.items).(*types.Transaction)
		if l.all.Get(tx.Hash()) == nil {
			l.stales--
			continue
		}
//找到未过期的事务，除非本地
		if local.containsTx(tx) {
			save = append(save, tx)
		} else {
			drop = append(drop, tx)
			count--
		}
	}
	for _, tx := range save {
		heap.Push(l.items, tx)
	}
	return drop
}
