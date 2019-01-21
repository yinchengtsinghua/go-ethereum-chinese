
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/pot"
)

/*

根据相对于固定点X的接近顺序，将点分类为
将空间（n字节长的字节序列）放入容器中。每个项目位于
最远距离X的一半是前一个箱中的项目。给出了一个
均匀分布项（任意序列上的哈希函数）
接近比例映射到一系列子集上，基数为负
指数尺度。

它还具有属于同一存储箱的任何两个项目所在的属性。
最远的距离是彼此距离x的一半。

如果我们把垃圾箱中的随机样本看作是网络中的连接，
互联节点的相对邻近性可以作为局部
当任务要在两个路径之间查找路径时，用于图形遍历的决策
点。因为在每个跳跃中，有限距离的一半
达到一个跳数所需的保证不变的最大跳数限制。
节点。
**/


var Pof = pot.DefaultPof(256)

//kadparams保存kademlia的配置参数
type KadParams struct {
//可调参数
MaxProxDisplay    int   //表显示的行数
NeighbourhoodSize int   //最近邻核最小基数
MinBinSize        int   //一行中的最小对等数
MaxBinSize        int   //修剪前一行中的最大对等数
RetryInterval     int64 //对等机首次重新拨号前的初始间隔
RetryExponent     int   //用指数乘以重试间隔
MaxRetries        int   //重拨尝试的最大次数
//制裁或阻止建议同伴的职能
	Reachable func(*BzzAddr) bool `json:"-"`
}

//newkadparams返回带有默认值的params结构
func NewKadParams() *KadParams {
	return &KadParams{
		MaxProxDisplay:    16,
		NeighbourhoodSize: 2,
		MinBinSize:        2,
		MaxBinSize:        4,
RetryInterval:     4200000000, //4.2秒
		MaxRetries:        42,
		RetryExponent:     2,
	}
}

//Kademlia是一个活动对等端表和一个已知对等端数据库（节点记录）
type Kademlia struct {
	lock       sync.RWMutex
*KadParams          //Kademlia配置参数
base       []byte   //表的不可变基址
addrs      *pot.Pot //用于已知对等地址的POTS容器
conns      *pot.Pot //用于实时对等连接的POTS容器
depth      uint8    //存储上一个当前饱和深度
nDepth     int      //存储上一个邻居深度
nDepthC    chan int //由depthc函数返回，用于信号邻域深度变化
addrCountC chan int //由addrcountc函数返回以指示对等计数更改
}

//newkademlia为基地址addr创建一个kademlia表
//参数与参数相同
//如果params为nil，则使用默认值
func NewKademlia(addr []byte, params *KadParams) *Kademlia {
	if params == nil {
		params = NewKadParams()
	}
	return &Kademlia{
		base:      addr,
		KadParams: params,
		addrs:     pot.NewPot(nil, 0),
		conns:     pot.NewPot(nil, 0),
	}
}

//条目表示一个kademlia表条目（bzzaddr的扩展）
type entry struct {
	*BzzAddr
	conn    *Peer
	seenAt  time.Time
	retries int
}

//NewEntry从*对等创建一个Kademlia对等
func newEntry(p *BzzAddr) *entry {
	return &entry{
		BzzAddr: p,
		seenAt:  time.Now(),
	}
}

//label是调试条目的短标记
func Label(e *entry) string {
	return fmt.Sprintf("%s (%d)", e.Hex()[:4], e.retries)
}

//十六进制是入口地址的十六进制序列化
func (e *entry) Hex() string {
	return fmt.Sprintf("%x", e.Address())
}

//寄存器将每个地址作为kademlia对等记录输入
//已知对等地址数据库
func (k *Kademlia) Register(peers ...*BzzAddr) error {
	k.lock.Lock()
	defer k.lock.Unlock()
	var known, size int
	for _, p := range peers {
//如果自我接收错误，对等方应该知道得更好。
//应该为此受到惩罚
		if bytes.Equal(p.Address(), k.base) {
			return fmt.Errorf("add peers: %x is self", k.base)
		}
		var found bool
		k.addrs, _, found, _ = pot.Swap(k.addrs, p, Pof, func(v pot.Val) pot.Val {
//如果没有找到
			if v == nil {
//在conn中插入新的脱机对等
				return newEntry(p)
			}
//在已知的同龄人中发现，什么都不做
			return v
		})
		if found {
			known++
		}
		size++
	}
//仅当有新地址时才发送新地址计数值
	if k.addrCountC != nil && size-known > 0 {
		k.addrCountC <- k.addrs.Size()
	}

	k.sendNeighbourhoodDepthChange()
	return nil
}

//SuggestPeer返回未连接的对等地址作为连接的对等建议
func (k *Kademlia) SuggestPeer() (suggestedPeer *BzzAddr, saturationDepth int, changed bool) {
	k.lock.Lock()
	defer k.lock.Unlock()
	radius := neighbourhoodRadiusForPot(k.conns, k.NeighbourhoodSize, k.base)
//按连接对等数的升序收集未饱和的垃圾箱
//从浅到深（po的升序）
//将它们插入bin数组的映射中，并用连接的对等方的数量进行键控
	saturation := make(map[int][]int)
var lastPO int       //迭代中最后一个非空的po bin
saturationDepth = -1 //最深的Po，所有较浅的箱子都有大于等于k.minbinsizepeer的箱子。
var pastDepth bool   //迭代的po是否大于等于深度
	k.conns.EachBin(k.base, Pof, 0, func(po, size int, f func(func(val pot.Val) bool) bool) bool {
//处理跳过的空容器
		for ; lastPO < po; lastPO++ {
//找到最低的不饱和料仓
			if saturationDepth == -1 {
				saturationDepth = lastPO
			}
//如果有空的垃圾桶，深度肯定会过去。
			pastDepth = true
			saturation[0] = append(saturation[0], lastPO)
		}
		lastPO = po + 1
//通过半径，深度肯定通过
		if po >= radius {
			pastDepth = true
		}
//超出深度后，即使尺寸大于等于K.MinBinSize，料仓也被视为不饱和。
//为了实现与所有邻居的完全连接
		if pastDepth && size >= k.MinBinSize {
			size = k.MinBinSize - 1
		}
//处理非空不饱和料仓
		if size < k.MinBinSize {
//找到最低的不饱和料仓
			if saturationDepth == -1 {
				saturationDepth = po
			}
			saturation[size] = append(saturation[size], po)
		}
		return true
	})
//触发比最近连接更近的对等请求，包括
//从最近连接到最近地址的所有箱子都是不饱和的。
	var nearestAddrAt int
	k.addrs.EachNeighbour(k.base, Pof, func(_ pot.Val, po int) bool {
		nearestAddrAt = po
		return false
	})
//包含大小为0的容器会影响请求连接
//优先于非空的浅仓
	for ; lastPO <= nearestAddrAt; lastPO++ {
		saturation[0] = append(saturation[0], lastPO)
	}
//所有的po箱都是饱和的，即minsize>=k.minbinSize，不建议同行使用。
	if len(saturation) == 0 {
		return nil, 0, false
	}
//在通讯簿中查找第一个可调用的对等点
//从最小尺寸的箱子开始，从浅到深
//对于每个垃圾箱（直到邻里半径），我们可以找到可调用的候选对等物。
	for size := 0; size < k.MinBinSize && suggestedPeer == nil; size++ {
		bins, ok := saturation[size]
		if !ok {
//没有这种尺寸的箱子
			continue
		}
		cur := 0
		curPO := bins[0]
		k.addrs.EachBin(k.base, Pof, curPO, func(po, _ int, f func(func(pot.Val) bool) bool) bool {
			curPO = bins[cur]
//查找下一个大小为的纸盒
			if curPO == po {
				cur++
			} else {
//跳过没有地址的存储箱
				for ; cur < len(bins) && curPO < po; cur++ {
					curPO = bins[cur]
				}
				if po < curPO {
					cur--
					return true
				}
//没有地址时停止
				if curPO < po {
					return false
				}
			}
//科普发现
//从未饱和的bin中的地址中找到一个可调用的对等方
//如果发现停止
			f(func(val pot.Val) bool {
				e := val.(*entry)
				if k.callable(e) {
					suggestedPeer = e.BzzAddr
					return false
				}
				return true
			})
			return cur < len(bins) && suggestedPeer == nil
		})
	}

	if uint8(saturationDepth) < k.depth {
		k.depth = uint8(saturationDepth)
		return suggestedPeer, saturationDepth, true
	}
	return suggestedPeer, 0, false
}

//在上，将对等机作为Kademlia对等机插入活动对等机
func (k *Kademlia) On(p *Peer) (uint8, bool) {
	k.lock.Lock()
	defer k.lock.Unlock()
	var ins bool
	k.conns, _, _, _ = pot.Swap(k.conns, p, Pof, func(v pot.Val) pot.Val {
//如果找不到现场
		if v == nil {
			ins = true
//在conns中插入新的在线对等点
			return p
		}
//在同龄人中找到，什么都不做
		return v
	})
	if ins && !p.BzzPeer.LightNode {
		a := newEntry(p.BzzAddr)
		a.conn = p
//在加法器中插入新的在线对等点
		k.addrs, _, _, _ = pot.Swap(k.addrs, p, Pof, func(v pot.Val) pot.Val {
			return a
		})
//仅当插入对等端时才发送新的地址计数值
		if k.addrCountC != nil {
			k.addrCountC <- k.addrs.Size()
		}
	}
	log.Trace(k.string())
//计算饱和深度是否改变
	depth := uint8(k.saturation())
	var changed bool
	if depth != k.depth {
		changed = true
		k.depth = depth
	}
	k.sendNeighbourhoodDepthChange()
	return k.depth, changed
}

//Neighbourhooddepthc返回发送新Kademlia的频道
//每一次变化的邻里深度。
//不从返回通道接收将阻塞功能
//当邻近深度改变时。
//托多：为什么要导出它，如果应该的话；为什么我们不能有多个订户？
func (k *Kademlia) NeighbourhoodDepthC() <-chan int {
	k.lock.Lock()
	defer k.lock.Unlock()
	if k.nDepthC == nil {
		k.nDepthC = make(chan int)
	}
	return k.nDepthC
}

//sendnieghbourhooddepthchange向k.ndepth通道发送新的邻近深度
//如果已初始化。
func (k *Kademlia) sendNeighbourhoodDepthChange() {
//当调用neighbourhooddepthc并由其返回时，将初始化ndepthc。
//它提供了邻域深度变化的信号。
//如果满足这个条件，代码的这一部分将向ndepthc发送新的邻域深度。
	if k.nDepthC != nil {
		nDepth := depthForPot(k.conns, k.NeighbourhoodSize, k.base)
		if nDepth != k.nDepth {
			k.nDepth = nDepth
			k.nDepthC <- nDepth
		}
	}
}

//addrCountc返回发送新的
//每次更改的地址计数值。
//不从返回通道接收将阻止寄存器功能
//地址计数值更改时。
func (k *Kademlia) AddrCountC() <-chan int {
	if k.addrCountC == nil {
		k.addrCountC = make(chan int)
	}
	return k.addrCountC
}

//关闭从活动对等中删除对等
func (k *Kademlia) Off(p *Peer) {
	k.lock.Lock()
	defer k.lock.Unlock()
	var del bool
	if !p.BzzPeer.LightNode {
		k.addrs, _, _, _ = pot.Swap(k.addrs, p, Pof, func(v pot.Val) pot.Val {
//v不能为零，必须选中，否则我们将覆盖条目
			if v == nil {
				panic(fmt.Sprintf("connected peer not found %v", p))
			}
			del = true
			return newEntry(p.BzzAddr)
		})
	} else {
		del = true
	}

	if del {
		k.conns, _, _, _ = pot.Swap(k.conns, p, Pof, func(_ pot.Val) pot.Val {
//V不能为零，但不需要检查
			return nil
		})
//仅当对等端被删除时才发送新的地址计数值
		if k.addrCountC != nil {
			k.addrCountC <- k.addrs.Size()
		}
		k.sendNeighbourhoodDepthChange()
	}
}

//eachconn是一个带有args（base、po、f）的迭代器，将f应用于每个活动对等端
//从基地测量，接近订单为po或更低
//如果基为零，则使用Kademlia基地址
func (k *Kademlia) EachConn(base []byte, o int, f func(*Peer, int) bool) {
	k.lock.RLock()
	defer k.lock.RUnlock()
	k.eachConn(base, o, f)
}

func (k *Kademlia) eachConn(base []byte, o int, f func(*Peer, int) bool) {
	if len(base) == 0 {
		base = k.base
	}
	k.conns.EachNeighbour(base, Pof, func(val pot.Val, po int) bool {
		if po > o {
			return true
		}
		return f(val.(*Peer), po)
	})
}

//用（base，po，f）调用的eachaddr是一个迭代器，将f应用于每个已知的对等端
//从底部测量，接近顺序为O或更低
//如果基为零，则使用Kademlia基地址
func (k *Kademlia) EachAddr(base []byte, o int, f func(*BzzAddr, int) bool) {
	k.lock.RLock()
	defer k.lock.RUnlock()
	k.eachAddr(base, o, f)
}

func (k *Kademlia) eachAddr(base []byte, o int, f func(*BzzAddr, int) bool) {
	if len(base) == 0 {
		base = k.base
	}
	k.addrs.EachNeighbour(base, Pof, func(val pot.Val, po int) bool {
		if po > o {
			return true
		}
		return f(val.(*entry).BzzAddr, po)
	})
}

//Neighbourhooddepth返回壶的深度，请参见壶的深度
func (k *Kademlia) NeighbourhoodDepth() (depth int) {
	k.lock.RLock()
	defer k.lock.RUnlock()
	return depthForPot(k.conns, k.NeighbourhoodSize, k.base)
}

//邻里公路返回卡德米利亚的邻里半径。
//邻域半径包含大小大于等于neighbourhood size的最近邻域集
//也就是说，邻里半径是最深的，这样所有的垃圾箱就不会全部变浅。
//至少包含邻居大小连接的对等点
//如果连接的邻居大小的对等点总数少于，则返回0
//呼叫方必须持有锁
func neighbourhoodRadiusForPot(p *pot.Pot, neighbourhoodSize int, pivotAddr []byte) (depth int) {
	if p.Size() <= neighbourhoodSize {
		return 0
	}
//迭代中的对等方总数
	var size int
	f := func(v pot.Val, i int) bool {
//po==256表示addr是透视地址（self）
		if i == 256 {
			return true
		}
		size++

//这意味着我们都有NN同龄人。
//默认情况下，深度设置为最远nn对等端的bin。
		if size == neighbourhoodSize {
			depth = i
			return false
		}

		return true
	}
	p.EachNeighbour(pivotAddr, Pof, f)
	return depth
}

//depth for pot返回pot的深度
//深度是最近邻区最小延伸半径
//包括所有空的采购订单仓。也就是说，深度是最深的Po，因此
//-深度不超过邻里半径
//-所有比深度浅的箱子都不是空的。
//呼叫方必须持有锁
func depthForPot(p *pot.Pot, neighbourhoodSize int, pivotAddr []byte) (depth int) {
	if p.Size() <= neighbourhoodSize {
		return 0
	}
//确定深度是一个两步过程
//首先，我们找到邻近地区最浅的同类的近距离垃圾箱。
//深度的数值不能高于此值
	maxDepth := neighbourhoodRadiusForPot(p, neighbourhoodSize, pivotAddr)

//第二步是从最浅到最深依次测试空仓。
//如果发现空的垃圾箱，这将是实际深度
//如果达到第一步确定的最大深度，则停止迭代
	p.EachBin(pivotAddr, Pof, 0, func(po int, _ int, f func(func(pot.Val) bool) bool) bool {
		if po == depth {
			if maxDepth == depth {
				return false
			}
			depth++
			return true
		}
		return false
	})

	return depth
}

//Callable决定地址条目是否表示可调用对等
func (k *Kademlia) callable(e *entry) bool {
//如果对等方处于活动状态或超过了maxretries，则不可调用
	if e.conn != nil || e.retries > k.MaxRetries {
		return false
	}
//根据上次看到后经过的时间计算允许的重试次数
	timeAgo := int64(time.Since(e.seenAt))
	div := int64(k.RetryExponent)
	div += (150000 - rand.Int63n(300000)) * div / 1000000
	var retries int
	for delta := timeAgo; delta > k.RetryInterval; delta /= div {
		retries++
	}
//它从不并发调用，因此可以安全地递增
//可以再次重试对等机
	if retries < e.retries {
		log.Trace(fmt.Sprintf("%08x: %v long time since last try (at %v) needed before retry %v, wait only warrants %v", k.BaseAddr()[:4], e, timeAgo, e.retries, retries))
		return false
	}
//制裁或阻止建议同伴的职能
	if k.Reachable != nil && !k.Reachable(e.BzzAddr) {
		log.Trace(fmt.Sprintf("%08x: peer %v is temporarily not callable", k.BaseAddr()[:4], e))
		return false
	}
	e.retries++
	log.Trace(fmt.Sprintf("%08x: peer %v is callable", k.BaseAddr()[:4], e))

	return true
}

//baseaddr返回kademlia基地址
func (k *Kademlia) BaseAddr() []byte {
	return k.base
}

//字符串返回用ASCII显示的kademlia表+kaddb表
func (k *Kademlia) String() string {
	k.lock.RLock()
	defer k.lock.RUnlock()
	return k.string()
}

//字符串返回用ASCII显示的kademlia表+kaddb表
//呼叫方必须持有锁
func (k *Kademlia) string() string {
	wsrow := "                          "
	var rows []string

	rows = append(rows, "=========================================================================")
	rows = append(rows, fmt.Sprintf("%v KΛÐΞMLIΛ hive: queen's address: %x", time.Now().UTC().Format(time.UnixDate), k.BaseAddr()[:3]))
	rows = append(rows, fmt.Sprintf("population: %d (%d), NeighbourhoodSize: %d, MinBinSize: %d, MaxBinSize: %d", k.conns.Size(), k.addrs.Size(), k.NeighbourhoodSize, k.MinBinSize, k.MaxBinSize))

	liverows := make([]string, k.MaxProxDisplay)
	peersrows := make([]string, k.MaxProxDisplay)

	depth := depthForPot(k.conns, k.NeighbourhoodSize, k.base)
	rest := k.conns.Size()
	k.conns.EachBin(k.base, Pof, 0, func(po, size int, f func(func(val pot.Val) bool) bool) bool {
		var rowlen int
		if po >= k.MaxProxDisplay {
			po = k.MaxProxDisplay - 1
		}
		row := []string{fmt.Sprintf("%2d", size)}
		rest -= size
		f(func(val pot.Val) bool {
			e := val.(*Peer)
			row = append(row, fmt.Sprintf("%x", e.Address()[:2]))
			rowlen++
			return rowlen < 4
		})
		r := strings.Join(row, " ")
		r = r + wsrow
		liverows[po] = r[:31]
		return true
	})

	k.addrs.EachBin(k.base, Pof, 0, func(po, size int, f func(func(val pot.Val) bool) bool) bool {
		var rowlen int
		if po >= k.MaxProxDisplay {
			po = k.MaxProxDisplay - 1
		}
		if size < 0 {
			panic("wtf")
		}
		row := []string{fmt.Sprintf("%2d", size)}
//我们也在现场展示同龄人
		f(func(val pot.Val) bool {
			e := val.(*entry)
			row = append(row, Label(e))
			rowlen++
			return rowlen < 4
		})
		peersrows[po] = strings.Join(row, " ")
		return true
	})

	for i := 0; i < k.MaxProxDisplay; i++ {
		if i == depth {
			rows = append(rows, fmt.Sprintf("============ DEPTH: %d ==========================================", i))
		}
		left := liverows[i]
		right := peersrows[i]
		if len(left) == 0 {
			left = " 0                             "
		}
		if len(right) == 0 {
			right = " 0"
		}
		rows = append(rows, fmt.Sprintf("%03d %v | %v", i, left, right))
	}
	rows = append(rows, "=========================================================================")
	return "\n" + strings.Join(rows, "\n")
}

//Peerpot保存有关预期最近邻居的信息
//仅用于测试
//TODO移动到单独的测试工具文件
type PeerPot struct {
	NNSet [][]byte
}

//newpeerpotmap用键创建一个pot记录的映射*bzzaddr
//作为地址的十六进制表示。
//使用通过的卡德米利亚的邻里大小
//仅用于测试
//TODO移动到单独的测试工具文件
func NewPeerPotMap(neighbourhoodSize int, addrs [][]byte) map[string]*PeerPot {

//为运行状况检查创建所有节点的表
	np := pot.NewPot(nil, 0)
	for _, addr := range addrs {
		np, _, _ = pot.Add(np, addr, Pof)
	}
	ppmap := make(map[string]*PeerPot)

//为连接生成一个通晓真相的来源
//每经过一个卡德米利亚
	for i, a := range addrs {

//实际Kademlia深度
		depth := depthForPot(np, neighbourhoodSize, a)

//全神经网络节点
		var nns [][]byte

//从最深的地方到最浅的地方
		np.EachNeighbour(a, Pof, func(val pot.Val, po int) bool {
			addr := val.([]byte)
//po==256表示addr是透视地址（self）
//我们在地图上不包括自己
			if po == 256 {
				return true
			}
//附加找到的任何邻居
//邻居是指深度内或深度之外的任何对等体。
			if po >= depth {
				nns = append(nns, addr)
				return true
			}
			return false
		})

		log.Trace(fmt.Sprintf("%x PeerPotMap NNS: %s", addrs[i][:4], LogAddrs(nns)))
		ppmap[common.Bytes2Hex(a)] = &PeerPot{
			NNSet: nns,
		}
	}
	return ppmap
}

//饱和返回节点具有小于minbinsize对等点的最小采购订单值。
//如果迭代器达到邻域半径，则返回最后一个bin+1
func (k *Kademlia) saturation() int {
	prev := -1
	radius := neighbourhoodRadiusForPot(k.conns, k.NeighbourhoodSize, k.base)
	k.conns.EachBin(k.base, Pof, 0, func(po, size int, f func(func(val pot.Val) bool) bool) bool {
		prev++
		if po >= radius {
			return false
		}
		return prev == po && size >= k.MinBinSize
	})
	if prev < 0 {
		return 0
	}
	return prev
}

//知道的邻居测试是否所有邻居都在同一个地方
//在卡德米利亚已知的同龄人中发现
//它仅用于健康功能的测试
//TODO移动到单独的测试工具文件
func (k *Kademlia) knowNeighbours(addrs [][]byte) (got bool, n int, missing [][]byte) {
	pm := make(map[string]bool)
	depth := depthForPot(k.conns, k.NeighbourhoodSize, k.base)
//创建一张地图，让所有同行深入了解卡德米利亚。
	k.eachAddr(nil, 255, func(p *BzzAddr, po int) bool {
//与卡德米利亚基地地址相比，从最深到最浅
//包括所有料仓（自身除外）（0<=料仓<=255）
		if po < depth {
			return false
		}
		pk := common.Bytes2Hex(p.Address())
		pm[pk] = true
		return true
	})

//遍历Peerpot地图中最近的邻居
//如果我们在上面创建的地图中找不到邻居
//那我们就不了解所有的邻居了
//（可悲的是，这在现代社会太普遍了）
	var gots int
	var culprits [][]byte
	for _, p := range addrs {
		pk := common.Bytes2Hex(p)
		if pm[pk] {
			gots++
		} else {
			log.Trace(fmt.Sprintf("%08x: known nearest neighbour %s not found", k.base, pk))
			culprits = append(culprits, p)
		}
	}
	return gots == len(addrs), gots, culprits
}

//连接的邻居测试Peerpot中的所有邻居
//目前连接在卡德米利亚
//它仅用于健康功能的测试
func (k *Kademlia) connectedNeighbours(peers [][]byte) (got bool, n int, missing [][]byte) {
	pm := make(map[string]bool)

//创建一个地图，所有深度和深度的对等点都连接在Kademlia中。
//与卡德米利亚基地地址相比，从最深到最浅
//包括所有料仓（自身除外）（0<=料仓<=255）
	depth := depthForPot(k.conns, k.NeighbourhoodSize, k.base)
	k.eachConn(nil, 255, func(p *Peer, po int) bool {
		if po < depth {
			return false
		}
		pk := common.Bytes2Hex(p.Address())
		pm[pk] = true
		return true
	})

//遍历Peerpot地图中最近的邻居
//如果我们在上面创建的地图中找不到邻居
//那我们就不了解所有的邻居了
	var gots int
	var culprits [][]byte
	for _, p := range peers {
		pk := common.Bytes2Hex(p)
		if pm[pk] {
			gots++
		} else {
			log.Trace(fmt.Sprintf("%08x: ExpNN: %s not found", k.base, pk))
			culprits = append(culprits, p)
		}
	}
	return gots == len(peers), gots, culprits
}

//卡德米利亚的健康状况
//仅用于测试
type Health struct {
KnowNN           bool     //节点是否知道其所有邻居
CountKnowNN      int      //已知邻居数量
MissingKnowNN    [][]byte //我们应该知道哪些邻居，但我们不知道
ConnectNN        bool     //节点是否连接到其所有邻居
CountConnectNN   int      //连接到的邻居数量
MissingConnectNN [][]byte //我们应该和哪个邻居有联系，但我们没有
Saturated        bool     //我们是否与所有我们想联系的同龄人建立了联系
	Hive             string
}

//健康报告Kademlia连接性的健康状态
//
//peerpot参数提供了网络的全知视图
//结果健康对象是
//问题中的卡德米利亚（接受者）的实际组成是什么，以及
//当我们考虑到我们对网络的所有了解时，应该是什么情况呢？
//
//仅用于测试
func (k *Kademlia) Healthy(pp *PeerPot) *Health {
	k.lock.RLock()
	defer k.lock.RUnlock()
	if len(pp.NNSet) < k.NeighbourhoodSize {
		log.Warn("peerpot NNSet < NeighbourhoodSize")
	}
	gotnn, countgotnn, culpritsgotnn := k.connectedNeighbours(pp.NNSet)
	knownn, countknownn, culpritsknownn := k.knowNeighbours(pp.NNSet)
	depth := depthForPot(k.conns, k.NeighbourhoodSize, k.base)
	saturated := k.saturation() < depth
	log.Trace(fmt.Sprintf("%08x: healthy: knowNNs: %v, gotNNs: %v, saturated: %v\n", k.base, knownn, gotnn, saturated))
	return &Health{
		KnowNN:           knownn,
		CountKnowNN:      countknownn,
		MissingKnowNN:    culpritsknownn,
		ConnectNN:        gotnn,
		CountConnectNN:   countgotnn,
		MissingConnectNN: culpritsgotnn,
		Saturated:        saturated,
		Hive:             k.string(),
	}
}
