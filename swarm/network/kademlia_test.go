
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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/swarm/pot"
)

func init() {
	h := log.LvlFilterHandler(log.LvlWarn, log.StreamHandler(os.Stderr, log.TerminalFormat(true)))
	log.Root().SetHandler(h)
}

func testKadPeerAddr(s string) *BzzAddr {
	a := pot.NewAddressFromString(s)
	return &BzzAddr{OAddr: a, UAddr: a}
}

func newTestKademliaParams() *KadParams {
	params := NewKadParams()
	params.MinBinSize = 2
	params.NeighbourhoodSize = 2
	return params
}

type testKademlia struct {
	*Kademlia
	t *testing.T
}

func newTestKademlia(t *testing.T, b string) *testKademlia {
	base := pot.NewAddressFromString(b)
	return &testKademlia{
		Kademlia: NewKademlia(base, newTestKademliaParams()),
		t:        t,
	}
}

func (tk *testKademlia) newTestKadPeer(s string, lightNode bool) *Peer {
	return NewPeer(&BzzPeer{BzzAddr: testKadPeerAddr(s), LightNode: lightNode}, tk.Kademlia)
}

func (tk *testKademlia) On(ons ...string) {
	for _, s := range ons {
		tk.Kademlia.On(tk.newTestKadPeer(s, false))
	}
}

func (tk *testKademlia) Off(offs ...string) {
	for _, s := range offs {
		tk.Kademlia.Off(tk.newTestKadPeer(s, false))
	}
}

func (tk *testKademlia) Register(regs ...string) {
	var as []*BzzAddr
	for _, s := range regs {
		as = append(as, testKadPeerAddr(s))
	}
	err := tk.Kademlia.Register(as...)
	if err != nil {
		panic(err.Error())
	}
}

//检验邻域深度计算的有效性
//
//特别是，它测试如果有一个或多个连续的
//然后清空最远“最近邻居”上方的垃圾箱。
//深度应该设置在那些空箱子的最远处。
//
//TODO：使测试适应邻里关系的变化
func TestNeighbourhoodDepth(t *testing.T) {
	baseAddressBytes := RandomAddr().OAddr
	kad := NewKademlia(baseAddressBytes, NewKadParams())

	baseAddress := pot.NewAddressFromBytes(baseAddressBytes)

//生成对等点
	var peers []*Peer
	for i := 0; i < 7; i++ {
		addr := pot.RandomAddressAt(baseAddress, i)
		peers = append(peers, newTestDiscoveryPeer(addr, kad))
	}
	var sevenPeers []*Peer
	for i := 0; i < 2; i++ {
		addr := pot.RandomAddressAt(baseAddress, 7)
		sevenPeers = append(sevenPeers, newTestDiscoveryPeer(addr, kad))
	}

	testNum := 0
//第一次尝试空花环
	depth := kad.NeighbourhoodDepth()
	if depth != 0 {
		t.Fatalf("%d expected depth 0, was %d", testNum, depth)
	}
	testNum++

//在7上添加一个对等点
	kad.On(sevenPeers[0])
	depth = kad.NeighbourhoodDepth()
	if depth != 0 {
		t.Fatalf("%d expected depth 0, was %d", testNum, depth)
	}
	testNum++

//7点再加一秒
	kad.On(sevenPeers[1])
	depth = kad.NeighbourhoodDepth()
	if depth != 0 {
		t.Fatalf("%d expected depth 0, was %d", testNum, depth)
	}
	testNum++

//从0增加到6
	for i, p := range peers {
		kad.On(p)
		depth = kad.NeighbourhoodDepth()
		if depth != i+1 {
			t.Fatalf("%d.%d expected depth %d, was %d", i+1, testNum, i, depth)
		}
	}
	testNum++

	kad.Off(sevenPeers[1])
	depth = kad.NeighbourhoodDepth()
	if depth != 6 {
		t.Fatalf("%d expected depth 6, was %d", testNum, depth)
	}
	testNum++

	kad.Off(peers[4])
	depth = kad.NeighbourhoodDepth()
	if depth != 4 {
		t.Fatalf("%d expected depth 4, was %d", testNum, depth)
	}
	testNum++

	kad.Off(peers[3])
	depth = kad.NeighbourhoodDepth()
	if depth != 3 {
		t.Fatalf("%d expected depth 3, was %d", testNum, depth)
	}
	testNum++
}

//testHealthStrict测试最简单的健康定义
//这意味着我们是否与我们所认识的所有邻居都有联系
func TestHealthStrict(t *testing.T) {

//基址都是零
//没有对等体
//不健康（和孤独）
	tk := newTestKademlia(t, "11111111")
	tk.checkHealth(false, false)

//知道一个对等点但没有连接
//不健康的
	tk.Register("11100000")
	tk.checkHealth(false, false)

//认识一个同伴并建立联系
//健康的
	tk.On("11100000")
	tk.checkHealth(true, false)

//认识两个同龄人，只有一个相连
//不健康的
	tk.Register("11111100")
	tk.checkHealth(false, false)

//认识两个同龄人并与两个同龄人都有联系
//健康的
	tk.On("11111100")
	tk.checkHealth(true, false)

//认识三个同龄人，连到两个最深的
//健康的
	tk.Register("00000000")
	tk.checkHealth(true, false)

//认识三个同龄人，连接到所有三个
//健康的
	tk.On("00000000")
	tk.checkHealth(true, false)

//添加比当前深度更深的第四个对等点
//不健康的
	tk.Register("11110000")
	tk.checkHealth(false, false)

//连接到三个最深的对等点
//健康的
	tk.On("11110000")
	tk.checkHealth(true, false)

//在同一个bin中添加其他对等机作为最深对等机
//不健康的
	tk.Register("11111101")
	tk.checkHealth(false, false)

//五个对等点中连接最深的四个
//健康的
	tk.On("11111101")
	tk.checkHealth(true, false)
}

func (tk *testKademlia) checkHealth(expectHealthy bool, expectSaturation bool) {
	tk.t.Helper()
	kid := common.Bytes2Hex(tk.BaseAddr())
	addrs := [][]byte{tk.BaseAddr()}
	tk.EachAddr(nil, 255, func(addr *BzzAddr, po int) bool {
		addrs = append(addrs, addr.Address())
		return true
	})

	pp := NewPeerPotMap(tk.NeighbourhoodSize, addrs)
	healthParams := tk.Healthy(pp[kid])

//健康的定义，所有条件，但必须真实：
//-我们至少认识一个同伴
//我们认识所有的邻居
//-我们与所有已知的邻居都有联系
	health := healthParams.KnowNN && healthParams.ConnectNN && healthParams.CountKnowNN > 0
	if expectHealthy != health {
		tk.t.Fatalf("expected kademlia health %v, is %v\n%v", expectHealthy, health, tk.String())
	}
}

func (tk *testKademlia) checkSuggestPeer(expAddr string, expDepth int, expChanged bool) {
	tk.t.Helper()
	addr, depth, changed := tk.SuggestPeer()
	log.Trace("suggestPeer return", "addr", addr, "depth", depth, "changed", changed)
	if binStr(addr) != expAddr {
		tk.t.Fatalf("incorrect peer address suggested. expected %v, got %v", expAddr, binStr(addr))
	}
	if depth != expDepth {
		tk.t.Fatalf("incorrect saturation depth suggested. expected %v, got %v", expDepth, depth)
	}
	if changed != expChanged {
		tk.t.Fatalf("expected depth change = %v, got %v", expChanged, changed)
	}
}

func binStr(a *BzzAddr) string {
	if a == nil {
		return "<nil>"
	}
	return pot.ToBin(a.Address())[:8]
}

func TestSuggestPeerFindPeers(t *testing.T) {
	tk := newTestKademlia(t, "00000000")
	tk.On("00100000")
	tk.checkSuggestPeer("<nil>", 0, false)

	tk.On("00010000")
	tk.checkSuggestPeer("<nil>", 0, false)

	tk.On("10000000", "10000001")
	tk.checkSuggestPeer("<nil>", 0, false)

	tk.On("01000000")
	tk.Off("10000001")
	tk.checkSuggestPeer("10000001", 0, true)

	tk.On("00100001")
	tk.Off("01000000")
	tk.checkSuggestPeer("01000000", 0, false)

//第二次断开连接的对等机不可调用
//间隔合理
	tk.checkSuggestPeer("<nil>", 0, false)

//一次又一次地打开和关闭，对等呼叫再次
	tk.On("01000000")
	tk.Off("01000000")
	tk.checkSuggestPeer("01000000", 0, false)

	tk.On("01000000", "10000001")
	tk.checkSuggestPeer("<nil>", 0, false)

	tk.Register("00010001")
	tk.checkSuggestPeer("00010001", 0, false)

	tk.On("00010001")
	tk.Off("01000000")
	tk.checkSuggestPeer("01000000", 0, false)

	tk.On("01000000")
	tk.checkSuggestPeer("<nil>", 0, false)

	tk.Register("01000001")
	tk.checkSuggestPeer("01000001", 0, false)

	tk.On("01000001")
	tk.checkSuggestPeer("<nil>", 0, false)

	tk.Register("10000010", "01000010", "00100010")
	tk.checkSuggestPeer("<nil>", 0, false)

	tk.Register("00010010")
	tk.checkSuggestPeer("00010010", 0, false)

	tk.Off("00100001")
	tk.checkSuggestPeer("00100010", 2, true)

	tk.Off("01000001")
	tk.checkSuggestPeer("01000010", 1, true)

	tk.checkSuggestPeer("01000001", 0, false)
	tk.checkSuggestPeer("00100001", 0, false)
	tk.checkSuggestPeer("<nil>", 0, false)

	tk.On("01000001", "00100001")
	tk.Register("10000100", "01000100", "00100100")
	tk.Register("00000100", "00000101", "00000110")
	tk.Register("00000010", "00000011", "00000001")

	tk.checkSuggestPeer("00000110", 0, false)
	tk.checkSuggestPeer("00000101", 0, false)
	tk.checkSuggestPeer("00000100", 0, false)
	tk.checkSuggestPeer("00000011", 0, false)
	tk.checkSuggestPeer("00000010", 0, false)
	tk.checkSuggestPeer("00000001", 0, false)
	tk.checkSuggestPeer("<nil>", 0, false)

}

//如果一个节点从Kademlia中删除，它应该留在通讯簿中。
func TestOffEffectingAddressBookNormalNode(t *testing.T) {
	tk := newTestKademlia(t, "00000000")
//添加到Kademlia的对等
	tk.On("01000000")
//对等机应该在通讯簿中
	if tk.addrs.Size() != 1 {
		t.Fatal("known peer addresses should contain 1 entry")
	}
//对等端应位于活动连接之间
	if tk.conns.Size() != 1 {
		t.Fatal("live peers should contain 1 entry")
	}
//从Kademlia中删除对等
	tk.Off("01000000")
//对等机应该在通讯簿中
	if tk.addrs.Size() != 1 {
		t.Fatal("known peer addresses should contain 1 entry")
	}
//对等端不应位于活动连接之间
	if tk.conns.Size() != 0 {
		t.Fatal("live peers should contain 0 entry")
	}
}

//轻节点不应在通讯簿中
func TestOffEffectingAddressBookLightNode(t *testing.T) {
	tk := newTestKademlia(t, "00000000")
//添加到Kademlia的光节点对等体
	tk.Kademlia.On(tk.newTestKadPeer("01000000", true))
//对等机不应在通讯簿中
	if tk.addrs.Size() != 0 {
		t.Fatal("known peer addresses should contain 0 entry")
	}
//对等端应位于活动连接之间
	if tk.conns.Size() != 1 {
		t.Fatal("live peers should contain 1 entry")
	}
//从Kademlia中删除对等
	tk.Kademlia.Off(tk.newTestKadPeer("01000000", true))
//对等机不应在通讯簿中
	if tk.addrs.Size() != 0 {
		t.Fatal("known peer addresses should contain 0 entry")
	}
//对等端不应位于活动连接之间
	if tk.conns.Size() != 0 {
		t.Fatal("live peers should contain 0 entry")
	}
}

func TestSuggestPeerRetries(t *testing.T) {
	tk := newTestKademlia(t, "00000000")
tk.RetryInterval = int64(300 * time.Millisecond) //周期
	tk.MaxRetries = 50
	tk.RetryExponent = 2
	sleep := func(n int) {
		ts := tk.RetryInterval
		for i := 1; i < n; i++ {
			ts *= int64(tk.RetryExponent)
		}
		time.Sleep(time.Duration(ts))
	}

	tk.Register("01000000")
	tk.On("00000001", "00000010")
	tk.checkSuggestPeer("01000000", 0, false)

	tk.checkSuggestPeer("<nil>", 0, false)

	sleep(1)
	tk.checkSuggestPeer("01000000", 0, false)

	tk.checkSuggestPeer("<nil>", 0, false)

	sleep(1)
	tk.checkSuggestPeer("01000000", 0, false)

	tk.checkSuggestPeer("<nil>", 0, false)

	sleep(2)
	tk.checkSuggestPeer("01000000", 0, false)

	tk.checkSuggestPeer("<nil>", 0, false)

	sleep(2)
	tk.checkSuggestPeer("<nil>", 0, false)
}

func TestKademliaHiveString(t *testing.T) {
	tk := newTestKademlia(t, "00000000")
	tk.On("01000000", "00100000")
	tk.Register("10000000", "10000001")
	tk.MaxProxDisplay = 8
	h := tk.String()
	expH := "\n=========================================================================\nMon Feb 27 12:10:28 UTC 2017 KΛÐΞMLIΛ hive: queen's address: 000000\npopulation: 2 (4), NeighbourhoodSize: 2, MinBinSize: 2, MaxBinSize: 4\n============ DEPTH: 0 ==========================================\n000  0                              |  2 8100 (0) 8000 (0)\n001  1 4000                         |  1 4000 (0)\n002  1 2000                         |  1 2000 (0)\n003  0                              |  0\n004  0                              |  0\n005  0                              |  0\n006  0                              |  0\n007  0                              |  0\n========================================================================="
	if expH[104:] != h[104:] {
		t.Fatalf("incorrect hive output. expected %v, got %v", expH, h)
	}
}

func newTestDiscoveryPeer(addr pot.Address, kad *Kademlia) *Peer {
	rw := &p2p.MsgPipeRW{}
	p := p2p.NewPeer(enode.ID{}, "foo", []p2p.Cap{})
	pp := protocols.NewPeer(p, rw, &protocols.Spec{})
	bp := &BzzPeer{
		Peer: pp,
		BzzAddr: &BzzAddr{
			OAddr: addr.Bytes(),
			UAddr: []byte(fmt.Sprintf("%x", addr[:])),
		},
	}
	return NewPeer(bp, kad)
}
