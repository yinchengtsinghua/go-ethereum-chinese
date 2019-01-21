
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

//包les实现轻以太坊子协议。
package les

import (
	"io"
	"math"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/common/prque"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

//FreeClientPool实现限制连接时间的客户端数据库
//对每个客户机，并管理接受/拒绝传入连接，甚至
//排除一些连接的客户机。池计算最近的使用时间
//对于每个已知客户机（当客户机
//已连接，未连接时按指数递减）。低级别客户
//最新的用法是首选的，未知节点具有最高优先级。已经
//连接的节点会收到一个有利于它们的小偏差，以避免接受
//立刻把客户赶出去。
//
//注意：池可以使用任何字符串来标识客户机。使用签名
//如果知道钥匙有负面影响，那么就没有意义了。
//客户端的值。目前，LES协议管理器使用IP地址
//（没有端口地址）以标识客户机。
type freeClientPool struct {
	db     ethdb.Database
	lock   sync.Mutex
	clock  mclock.Clock
	closed bool

	connectedLimit, totalLimit int

	addressMap            map[string]*freeClientPoolEntry
	connPool, disconnPool *prque.Prque
	startupTime           mclock.AbsTime
	logOffsetAtStartup    int64
}

const (
recentUsageExpTC     = time.Hour   //“最近”服务器使用的指数加权窗口的时间常数
fixedPointMultiplier = 0x1000000   //常量将对数转换为定点格式
connectedBias        = time.Minute //这种偏见适用于已经建立联系的客户，以避免他们很快被淘汰。
)

//NewFreeClientPool创建新的免费客户端池
func newFreeClientPool(db ethdb.Database, connectedLimit, totalLimit int, clock mclock.Clock) *freeClientPool {
	pool := &freeClientPool{
		db:             db,
		clock:          clock,
		addressMap:     make(map[string]*freeClientPoolEntry),
		connPool:       prque.New(poolSetIndex),
		disconnPool:    prque.New(poolSetIndex),
		connectedLimit: connectedLimit,
		totalLimit:     totalLimit,
	}
	pool.loadFromDb()
	return pool
}

func (f *freeClientPool) stop() {
	f.lock.Lock()
	f.closed = true
	f.saveToDb()
	f.lock.Unlock()
}

//成功握手后应调用Connect。如果连接是
//拒绝，不需要呼叫断开。
//
//注意：disconnectfn回调不应阻塞。
func (f *freeClientPool) connect(address string, disconnectFn func()) bool {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.closed {
		return false
	}
	e := f.addressMap[address]
	now := f.clock.Now()
	var recentUsage int64
	if e == nil {
		e = &freeClientPoolEntry{address: address, index: -1}
		f.addressMap[address] = e
	} else {
		if e.connected {
			log.Debug("Client already connected", "address", address)
			return false
		}
		recentUsage = int64(math.Exp(float64(e.logUsage-f.logOffset(now)) / fixedPointMultiplier))
	}
	e.linUsage = recentUsage - int64(now)
//检查（Linusage+ConnectedBias）是否小于连接池中的最高条目
	if f.connPool.Size() == f.connectedLimit {
		i := f.connPool.PopItem().(*freeClientPoolEntry)
		if e.linUsage+int64(connectedBias)-i.linUsage < 0 {
//把它踢出去接受新客户
			f.connPool.Remove(i.index)
			f.calcLogUsage(i, now)
			i.connected = false
			f.disconnPool.Push(i, -i.logUsage)
			log.Debug("Client kicked out", "address", i.address)
			i.disconnectFn()
		} else {
//保留旧客户并拒绝新客户
			f.connPool.Push(i, i.linUsage)
			log.Debug("Client rejected", "address", address)
			return false
		}
	}
	f.disconnPool.Remove(e.index)
	e.connected = true
	e.disconnectFn = disconnectFn
	f.connPool.Push(e, e.linUsage)
	if f.connPool.Size()+f.disconnPool.Size() > f.totalLimit {
		f.disconnPool.Pop()
	}
	log.Debug("Client accepted", "address", address)
	return true
}

//连接终止时应调用Disconnect。如果断开
//由池本身使用disconnectfn启动，然后调用disconnect是
//不必要，但允许。
func (f *freeClientPool) disconnect(address string) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.closed {
		return
	}
	e := f.addressMap[address]
	now := f.clock.Now()
	if !e.connected {
		log.Debug("Client already disconnected", "address", address)
		return
	}

	f.connPool.Remove(e.index)
	f.calcLogUsage(e, now)
	e.connected = false
	f.disconnPool.Push(e, -e.logUsage)
	log.Debug("Client disconnected", "address", address)
}

//Logoffset计算对数的时间相关偏移量
//最近使用的表示法
func (f *freeClientPool) logOffset(now mclock.AbsTime) int64 {
//注意：这里fixedpointmultipler用作乘数；除数的原因
//是为了避免Int64溢出。我们假设int64（recentusageexptc）>>固定点乘数。
	logDecay := int64((time.Duration(now - f.startupTime)) / (recentUsageExpTC / fixedPointMultiplier))
	return f.logOffsetAtStartup + logDecay
}

//calclogusage将最近的用法从线性表示转换为对数表示
//断开对等机连接或关闭客户端池时
func (f *freeClientPool) calcLogUsage(e *freeClientPoolEntry, now mclock.AbsTime) {
	dt := e.linUsage + int64(now)
	if dt < 1 {
		dt = 1
	}
	e.logUsage = int64(math.Log(float64(dt))*fixedPointMultiplier) + f.logOffset(now)
}

//FreeClientPoolStorage是池数据库存储的RLP表示形式
type freeClientPoolStorage struct {
	LogOffset uint64
	List      []*freeClientPoolEntry
}

//loadfromdb从数据库存储还原池状态
//（初始化时自动调用）
func (f *freeClientPool) loadFromDb() {
	enc, err := f.db.Get([]byte("freeClientPool"))
	if err != nil {
		return
	}
	var storage freeClientPoolStorage
	err = rlp.DecodeBytes(enc, &storage)
	if err != nil {
		log.Error("Failed to decode client list", "err", err)
		return
	}
	f.logOffsetAtStartup = int64(storage.LogOffset)
	f.startupTime = f.clock.Now()
	for _, e := range storage.List {
		log.Debug("Loaded free client record", "address", e.address, "logUsage", e.logUsage)
		f.addressMap[e.address] = e
		f.disconnPool.Push(e, -e.logUsage)
	}
}

//savetodb将池状态保存到数据库存储
//（关机时自动调用）
func (f *freeClientPool) saveToDb() {
	now := f.clock.Now()
	storage := freeClientPoolStorage{
		LogOffset: uint64(f.logOffset(now)),
		List:      make([]*freeClientPoolEntry, len(f.addressMap)),
	}
	i := 0
	for _, e := range f.addressMap {
		if e.connected {
			f.calcLogUsage(e, now)
		}
		storage.List[i] = e
		i++
	}
	enc, err := rlp.EncodeToBytes(storage)
	if err != nil {
		log.Error("Failed to encode client list", "err", err)
	} else {
		f.db.Put([]byte("freeClientPool"), enc)
	}
}

//FreeClientPoolentry表示池已知的客户端地址。
//连接后，最近的使用量计算为linusage+int64（clock.now（））
//断开连接时，计算为exp（logusage-logoffset），其中logoffset
//服务器运行时，也会随着时间线性增长。
//线性和对数表示之间的转换发生在连接时
//或者断开节点。
//
//注：linusage和logusage是不断增加偏移量的值，因此
//even though they are close to each other at any time they may wrap around int64
//时间的限制。应进行相应的比较。
type freeClientPoolEntry struct {
	address            string
	connected          bool
	disconnectFn       func()
	linUsage, logUsage int64
	index              int
}

func (e *freeClientPoolEntry) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{e.address, uint64(e.logUsage)})
}

func (e *freeClientPoolEntry) DecodeRLP(s *rlp.Stream) error {
	var entry struct {
		Address  string
		LogUsage uint64
	}
	if err := s.Decode(&entry); err != nil {
		return err
	}
	e.address = entry.Address
	e.logUsage = int64(entry.LogUsage)
	e.connected = false
	e.index = -1
	return nil
}

//poolSetIndex callback is used by both priority queues to set/update the index of
//队列中的元素。需要索引来删除顶部元素以外的元素。
func poolSetIndex(a interface{}, i int) {
	a.(*freeClientPoolEntry).index = i
}
