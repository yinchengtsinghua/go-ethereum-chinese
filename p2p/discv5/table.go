
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

//包discv5实现了rlpx v5主题发现协议。
//
//主题发现协议提供了一种查找
//可以连接到。它使用一个类似kademlia的协议来维护
//所有监听的ID和端点的分布式数据库
//节点。
package discv5

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"sort"

	"github.com/ethereum/go-ethereum/common"
)

const (
alpha      = 3  //Kademlia并发因子
bucketSize = 16 //卡德米利亚水桶尺寸
	hashBits   = len(common.Hash{}) * 8
nBuckets   = hashBits + 1 //桶数

	maxFindnodeFailures = 5
)

type Table struct {
count         int               //节点数
buckets       [nBuckets]*bucket //已知节点的距离索引
nodeAddedHook func(*Node)       //用于测试
self          *Node             //本地节点的元数据
}

//bucket包含按其上一个活动排序的节点。条目
//最近激活的元素是条目中的第一个元素。
type bucket struct {
	entries      []*Node
	replacements []*Node
}

func newTable(ourID NodeID, ourAddr *net.UDPAddr) *Table {
	self := NewNode(ourID, ourAddr.IP, uint16(ourAddr.Port), uint16(ourAddr.Port))
	tab := &Table{self: self}
	for i := range tab.buckets {
		tab.buckets[i] = new(bucket)
	}
	return tab
}

const printTable = false

//选择bucketrefreshTarget选择随机刷新目标以保留所有kademlia
//存储桶中充满了活动连接，并保持网络拓扑的健康。
//这就需要选择更接近我们自己的地址，并且概率更高
//为了刷新更近的存储桶。
//
//该算法近似于
//从表中选择一个随机节点并选择一个目标地址
//距离小于所选节点距离的两倍。
//该算法稍后将得到改进，以专门针对最近
//旧桶
func (tab *Table) chooseBucketRefreshTarget() common.Hash {
	entries := 0
	if printTable {
		fmt.Println()
	}
	for i, b := range &tab.buckets {
		entries += len(b.entries)
		if printTable {
			for _, e := range b.entries {
				fmt.Println(i, e.state, e.addr().String(), e.ID.String(), e.sha.Hex())
			}
		}
	}

	prefix := binary.BigEndian.Uint64(tab.self.sha[0:8])
	dist := ^uint64(0)
	entry := int(randUint(uint32(entries + 1)))
	for _, b := range &tab.buckets {
		if entry < len(b.entries) {
			n := b.entries[entry]
			dist = binary.BigEndian.Uint64(n.sha[0:8]) ^ prefix
			break
		}
		entry -= len(b.entries)
	}

	ddist := ^uint64(0)
	if dist+dist > dist {
		ddist = dist
	}
	targetPrefix := prefix ^ randUint64n(ddist)

	var target common.Hash
	binary.BigEndian.PutUint64(target[0:8], targetPrefix)
	rand.Read(target[8:])
	return target
}

//readrandomnodes用来自
//表。它不会多次写入同一节点。节点
//切片是副本，可以由调用方修改。
func (tab *Table) readRandomNodes(buf []*Node) (n int) {
//TODO:基于树的桶在这里有帮助
//找到所有非空桶，并从中获取新的部分。
	var buckets [][]*Node
	for _, b := range &tab.buckets {
		if len(b.entries) > 0 {
			buckets = append(buckets, b.entries)
		}
	}
	if len(buckets) == 0 {
		return 0
	}
//洗牌。
	for i := uint32(len(buckets)) - 1; i > 0; i-- {
		j := randUint(i)
		buckets[i], buckets[j] = buckets[j], buckets[i]
	}
//将每个桶的头部移入buf，移除变空的桶。
	var i, j int
	for ; i < len(buf); i, j = i+1, (j+1)%len(buckets) {
		b := buckets[j]
		buf[i] = &(*b[0])
		buckets[j] = b[1:]
		if len(b) == 1 {
			buckets = append(buckets[:j], buckets[j+1:]...)
		}
		if len(buckets) == 0 {
			break
		}
	}
	return i + 1
}

func randUint(max uint32) uint32 {
	if max < 2 {
		return 0
	}
	var b [4]byte
	rand.Read(b[:])
	return binary.BigEndian.Uint32(b[:]) % max
}

func randUint64n(max uint64) uint64 {
	if max < 2 {
		return 0
	}
	var b [8]byte
	rand.Read(b[:])
	return binary.BigEndian.Uint64(b[:]) % max
}

//最近返回表中最接近
//给定的ID。调用方必须保持tab.mutex。
func (tab *Table) closest(target common.Hash, nresults int) *nodesByDistance {
//这是一种非常浪费的查找最近节点的方法，但是
//显然是正确的。我相信以树为基础的桶可以
//这更容易有效地实施。
	close := &nodesByDistance{target: target}
	for _, b := range &tab.buckets {
		for _, n := range b.entries {
			close.push(n, nresults)
		}
	}
	return close
}

//添加尝试添加给定节点的相应存储桶。如果
//bucket有空间，添加节点立即成功。
//否则，节点将添加到存储桶的替换缓存中。
func (tab *Table) add(n *Node) (contested *Node) {
//fmt.println（“添加”，n.addr（）.string（），n.id.string（），n.sha.hex（））
	if n.ID == tab.self.ID {
		return
	}
	b := tab.buckets[logdist(tab.self.sha, n.sha)]
	switch {
	case b.bump(n):
//N存在于B中。
		return nil
	case len(b.entries) < bucketSize:
//B有空位。
		b.addFront(n)
		tab.count++
		if tab.nodeAddedHook != nil {
			tab.nodeAddedHook(n)
		}
		return nil
	default:
//B没有剩余空间，添加到替换缓存
//并重新验证最后一个条目。
//TODO:删除上一个节点
		b.replacements = append(b.replacements, n)
		if len(b.replacements) > bucketSize {
			copy(b.replacements, b.replacements[1:])
			b.replacements = b.replacements[:len(b.replacements)-1]
		}
		return b.entries[len(b.entries)-1]
	}
}

//stufacture将表中的节点添加到相应bucket的末尾
//如果桶没满。
func (tab *Table) stuff(nodes []*Node) {
outer:
	for _, n := range nodes {
		if n.ID == tab.self.ID {
continue //不要增加自我
		}
		bucket := tab.buckets[logdist(tab.self.sha, n.sha)]
		for i := range bucket.entries {
			if bucket.entries[i].ID == n.ID {
continue outer //已经在桶里了
			}
		}
		if len(bucket.entries) < bucketSize {
			bucket.entries = append(bucket.entries, n)
			tab.count++
			if tab.nodeAddedHook != nil {
				tab.nodeAddedHook(n)
			}
		}
	}
}

//删除从节点表中删除一个条目（用于清空
//失败/未绑定的发现对等机）。
func (tab *Table) delete(node *Node) {
//fmt.println（“删除”，node.addr（）.string（），node.id.string（），node.sha.hex（））
	bucket := tab.buckets[logdist(tab.self.sha, node.sha)]
	for i := range bucket.entries {
		if bucket.entries[i].ID == node.ID {
			bucket.entries = append(bucket.entries[:i], bucket.entries[i+1:]...)
			tab.count--
			return
		}
	}
}

func (tab *Table) deleteReplace(node *Node) {
	b := tab.buckets[logdist(tab.self.sha, node.sha)]
	i := 0
	for i < len(b.entries) {
		if b.entries[i].ID == node.ID {
			b.entries = append(b.entries[:i], b.entries[i+1:]...)
			tab.count--
		} else {
			i++
		}
	}
//从替换缓存重新填充
//TODO:可能使用随机索引
	if len(b.entries) < bucketSize && len(b.replacements) > 0 {
		ri := len(b.replacements) - 1
		b.addFront(b.replacements[ri])
		tab.count++
		b.replacements[ri] = nil
		b.replacements = b.replacements[:ri]
	}
}

func (b *bucket) addFront(n *Node) {
	b.entries = append(b.entries, nil)
	copy(b.entries[1:], b.entries)
	b.entries[0] = n
}

func (b *bucket) bump(n *Node) bool {
	for i := range b.entries {
		if b.entries[i].ID == n.ID {
//把它移到前面
			copy(b.entries[1:], b.entries[:i])
			b.entries[0] = n
			return true
		}
	}
	return false
}

//nodesByDistance是节点列表，按
//距离目标。
type nodesByDistance struct {
	entries []*Node
	target  common.Hash
}

//push将给定节点添加到列表中，使总大小保持在maxelems以下。
func (h *nodesByDistance) push(n *Node, maxElems int) {
	ix := sort.Search(len(h.entries), func(i int) bool {
		return distcmp(h.target, h.entries[i].sha, n.sha) > 0
	})
	if len(h.entries) < maxElems {
		h.entries = append(h.entries, n)
	}
	if ix == len(h.entries) {
//比我们现有的所有节点都要远。
//如果有空间，那么节点现在是最后一个元素。
	} else {
//向下滑动现有条目以腾出空间
//这将覆盖我们刚刚附加的条目。
		copy(h.entries[ix+1:], h.entries[ix:])
		h.entries[ix] = n
	}
}
