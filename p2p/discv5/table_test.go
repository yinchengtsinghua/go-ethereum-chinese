
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

package discv5

import (
	"crypto/ecdsa"
	"fmt"
	"math/rand"

	"net"
	"reflect"
	"testing"
	"testing/quick"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type nullTransport struct{}

func (nullTransport) sendPing(remote *Node, remoteAddr *net.UDPAddr) []byte { return []byte{1} }
func (nullTransport) sendPong(remote *Node, pingHash []byte)                {}
func (nullTransport) sendFindnode(remote *Node, target NodeID)              {}
func (nullTransport) sendNeighbours(remote *Node, nodes []*Node)            {}
func (nullTransport) localAddr() *net.UDPAddr                               { return new(net.UDPAddr) }
func (nullTransport) Close()                                                {}

//func测试表_pingreplace（t*testing.t）
//doit：=func（newnodeisresponding，lastinbuckettisresponding bool）
//传输：=newPingRecorder（））
//tab，：=newTable（传输，节点ID，&net.udpaddr）
//延迟tab.close（）
//Pinsender：=新节点（musthexid（“A502AF0F59B2AAB774695408C79E9CA312D2793CC997E44FC55EDA62F0150BBB8C59A6F9269BA3A081518B62699EE807C7C19C20125DDFCCCA872608AF9E370”），net.ip，99，99）
//
////填满发送方的桶。
//最后：=FillBucket（tab，253）
//
////此绑定调用应替换最后一个节点
////如果节点没有响应，则在其存储桶中。
//transport.responding[last.id]=上次BucketisResponding
//transport.responding[pinsender.id]=新节点响应
//tab.bond（真，pinsender.id，&net.udpaddr，0）
//
////第一个ping转到发送方（绑定pingback）
//如果！transport.pinged[pingender.id]
//t.error（“表没有ping回发送方”）
//}
//如果newnodeisresponding
////第二个ping转到bucket中最早的节点
//看它是否还活着。
//如果！transport.pinged[上一个.id]
//t.error（“table did not ping last node in bucket”）。
//}
//}
//
//tab.mutex.lock（）。
//延迟tab.mutex.unlock（）
//如果l：=len（tab.buckets[253].entries）；l！= BukStige{
//t.errorf（“绑定后的存储桶大小错误：得到%d，想要%d”，l，bucket size）
//}
//
//如果最后一个问题是响应！新节点响应
//如果！包含（tab.buckets[253].entries，last.id）
//t.error（“删除最后一个条目”）
//}
//如果包含（tab.buckets[253].entries，pinsender.id）
//t.error（“添加新条目”）
//}
//}否则{
//如果包含（tab.buckets[253].entries，last.id）
//t.error（“最后一项未删除”）
//}
//如果！包含（tab.buckets[253].entries，pinsender.id）
//t.error（“未添加新条目”）
//}
//}
//}
//
//doit（真，真）
//doit（假，真）
//doit（对，错）
//doit（假，假）
//}

func TestBucket_bumpNoDuplicates(t *testing.T) {
	t.Parallel()
	cfg := &quick.Config{
		MaxCount: 1000,
		Rand:     rand.New(rand.NewSource(time.Now().Unix())),
		Values: func(args []reflect.Value, rand *rand.Rand) {
//生成节点的随机列表。这将是存储桶的内容。
			n := rand.Intn(bucketSize-1) + 1
			nodes := make([]*Node, n)
			for i := range nodes {
				nodes[i] = nodeAtDistance(common.Hash{}, 200)
			}
			args[0] = reflect.ValueOf(nodes)
//生成随机凹凸位置。
			bumps := make([]int, rand.Intn(100))
			for i := range bumps {
				bumps[i] = rand.Intn(len(nodes))
			}
			args[1] = reflect.ValueOf(bumps)
		},
	}

	prop := func(nodes []*Node, bumps []int) (ok bool) {
		b := &bucket{entries: make([]*Node, len(nodes))}
		copy(b.entries, nodes)
		for i, pos := range bumps {
			b.bump(b.entries[pos])
			if hasDuplicates(b.entries) {
				t.Logf("bucket has duplicates after %d/%d bumps:", i+1, len(bumps))
				for _, n := range b.entries {
					t.Logf("  %p", n)
				}
				return false
			}
		}
		return true
	}
	if err := quick.Check(prop, cfg); err != nil {
		t.Error(err)
	}
}

//FillBucket将节点插入给定的bucket，直到
//它已经满了。节点的ID与
//散列。
func fillBucket(tab *Table, ld int) (last *Node) {
	b := tab.buckets[ld]
	for len(b.entries) < bucketSize {
		b.entries = append(b.entries, nodeAtDistance(tab.self.sha, ld))
	}
	return b.entries[bucketSize-1]
}

//nodeAtDistance为其创建logDist（base，n.sha）=ld的节点。
//节点的ID与n.sha不对应。
func nodeAtDistance(base common.Hash, ld int) (n *Node) {
	n = new(Node)
	n.sha = hashAtDistance(base, ld)
copy(n.ID[:], n.sha[:]) //确保节点仍然具有唯一的ID
	return n
}

type pingRecorder struct{ responding, pinged map[NodeID]bool }

func newPingRecorder() *pingRecorder {
	return &pingRecorder{make(map[NodeID]bool), make(map[NodeID]bool)}
}

func (t *pingRecorder) findnode(toid NodeID, toaddr *net.UDPAddr, target NodeID) ([]*Node, error) {
	panic("findnode called on pingRecorder")
}
func (t *pingRecorder) close() {}
func (t *pingRecorder) waitping(from NodeID) error {
return nil //远程总是ping
}
func (t *pingRecorder) ping(toid NodeID, toaddr *net.UDPAddr) error {
	t.pinged[toid] = true
	if t.responding[toid] {
		return nil
	} else {
		return errTimeout
	}
}

func TestTable_closest(t *testing.T) {
	t.Parallel()

	test := func(test *closeTest) bool {
//对于任何节点表、目标和n
		tab := newTable(test.Self, &net.UDPAddr{})
		tab.stuff(test.All)

//检查doclosest（target，n）是否返回节点
		result := tab.closest(test.Target, test.N).entries
		if hasDuplicates(result) {
			t.Errorf("result contains duplicates")
			return false
		}
		if !sortedByDistanceTo(test.Target, result) {
			t.Errorf("result is not sorted by distance to target")
			return false
		}

//检查结果数是否为min（n，tablen）
		wantN := test.N
		if tab.count < test.N {
			wantN = tab.count
		}
		if len(result) != wantN {
			t.Errorf("wrong number of nodes: got %d, want %d", len(result), wantN)
			return false
		} else if len(result) == 0 {
return true //不需要检查距离
		}

//检查结果节点与目标的距离是否最小。
		for _, b := range tab.buckets {
			for _, n := range b.entries {
				if contains(result, n.ID) {
continue //不要对结果中的节点运行下面的检查
				}
				farthestResult := result[len(result)-1].sha
				if distcmp(test.Target, n.sha, farthestResult) < 0 {
					t.Errorf("table contains node that is closer to target but it's not in result")
					t.Logf("  Target:          %v", test.Target)
					t.Logf("  Farthest Result: %v", farthestResult)
					t.Logf("  ID:              %v", n.ID)
					return false
				}
			}
		}
		return true
	}
	if err := quick.Check(test, quickcfg()); err != nil {
		t.Error(err)
	}
}

func TestTable_ReadRandomNodesGetAll(t *testing.T) {
	cfg := &quick.Config{
		MaxCount: 200,
		Rand:     rand.New(rand.NewSource(time.Now().Unix())),
		Values: func(args []reflect.Value, rand *rand.Rand) {
			args[0] = reflect.ValueOf(make([]*Node, rand.Intn(1000)))
		},
	}
	test := func(buf []*Node) bool {
		tab := newTable(NodeID{}, &net.UDPAddr{})
		for i := 0; i < len(buf); i++ {
			ld := cfg.Rand.Intn(len(tab.buckets))
			tab.stuff([]*Node{nodeAtDistance(tab.self.sha, ld)})
		}
		gotN := tab.readRandomNodes(buf)
		if gotN != tab.count {
			t.Errorf("wrong number of nodes, got %d, want %d", gotN, tab.count)
			return false
		}
		if hasDuplicates(buf[:gotN]) {
			t.Errorf("result contains duplicates")
			return false
		}
		return true
	}
	if err := quick.Check(test, cfg); err != nil {
		t.Error(err)
	}
}

type closeTest struct {
	Self   NodeID
	Target common.Hash
	All    []*Node
	N      int
}

func (*closeTest) Generate(rand *rand.Rand, size int) reflect.Value {
	t := &closeTest{
		Self:   gen(NodeID{}, rand).(NodeID),
		Target: gen(common.Hash{}, rand).(common.Hash),
		N:      rand.Intn(bucketSize),
	}
	for _, id := range gen([]NodeID{}, rand).([]NodeID) {
		t.All = append(t.All, &Node{ID: id})
	}
	return reflect.ValueOf(t)
}

func hasDuplicates(slice []*Node) bool {
	seen := make(map[NodeID]bool)
	for i, e := range slice {
		if e == nil {
			panic(fmt.Sprintf("nil *Node at %d", i))
		}
		if seen[e.ID] {
			return true
		}
		seen[e.ID] = true
	}
	return false
}

func sortedByDistanceTo(distbase common.Hash, slice []*Node) bool {
	var last common.Hash
	for i, e := range slice {
		if i > 0 && distcmp(distbase, e.sha, last) < 0 {
			return false
		}
		last = e.sha
	}
	return true
}

func contains(ns []*Node, id NodeID) bool {
	for _, n := range ns {
		if n.ID == id {
			return true
		}
	}
	return false
}

//Gen包装速度快，价值高，使用方便。
//它生成给定值类型的随机值。
func gen(typ interface{}, rand *rand.Rand) interface{} {
	v, ok := quick.Value(reflect.TypeOf(typ), rand)
	if !ok {
		panic(fmt.Sprintf("couldn't generate random value of type %T", typ))
	}
	return v.Interface()
}

func newkey() *ecdsa.PrivateKey {
	key, err := crypto.GenerateKey()
	if err != nil {
		panic("couldn't generate key: " + err.Error())
	}
	return key
}
