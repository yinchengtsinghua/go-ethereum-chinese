
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

package p2p

import (
	"encoding/binary"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/p2p/netutil"
)

func init() {
	spew.Config.Indent = "\t"
}

type dialtest struct {
init   *dialstate //测试前后的状态。
	rounds []round
}

type round struct {
peers []*Peer //当前对等集
done  []task  //这一轮完成的任务
new   []task  //结果必须与此匹配
}

func runDialTest(t *testing.T, test dialtest) {
	var (
		vtime   time.Time
		running int
	)
	pm := func(ps []*Peer) map[enode.ID]*Peer {
		m := make(map[enode.ID]*Peer)
		for _, p := range ps {
			m[p.ID()] = p
		}
		return m
	}
	for i, round := range test.rounds {
		for _, task := range round.done {
			running--
			if running < 0 {
				panic("running task counter underflow")
			}
			test.init.taskDone(task, vtime)
		}

		new := test.init.newTasks(running, pm(round.peers), vtime)
		if !sametasks(new, round.new) {
			t.Errorf("round %d: new tasks mismatch:\ngot %v\nwant %v\nstate: %v\nrunning: %v\n",
				i, spew.Sdump(new), spew.Sdump(round.new), spew.Sdump(test.init), spew.Sdump(running))
		}
		t.Log("tasks:", spew.Sdump(new))

//每轮时间提前16秒。
		vtime = vtime.Add(16 * time.Second)
		running += len(new)
	}
}

type fakeTable []*enode.Node

func (t fakeTable) Self() *enode.Node                     { return new(enode.Node) }
func (t fakeTable) Close()                                {}
func (t fakeTable) LookupRandom() []*enode.Node           { return nil }
func (t fakeTable) Resolve(*enode.Node) *enode.Node       { return nil }
func (t fakeTable) ReadRandomNodes(buf []*enode.Node) int { return copy(buf, t) }

//此测试检查是否从发现结果启动动态拨号。
func TestDialStateDynDial(t *testing.T) {
	runDialTest(t, dialtest{
		init: newDialState(enode.ID{}, nil, nil, fakeTable{}, 5, nil),
		rounds: []round{
//将启动发现查询。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(0), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
				},
				new: []task{&discoverTask{}},
			},
//动态拨号完成后启动。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(0), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
				},
				done: []task{
					&discoverTask{results: []*enode.Node{
newNode(uintID(2), nil), //这个已经连接，没有拨号。
						newNode(uintID(3), nil),
						newNode(uintID(4), nil),
						newNode(uintID(5), nil),
newNode(uintID(6), nil), //因为最大动态拨号是5，所以不尝试使用。
newNode(uintID(7), nil), //…
					}},
				},
				new: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(3), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(4), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(5), nil)},
				},
			},
//有些拨号盘已完成，但尚未启动新的拨号盘，因为
//活动拨号计数和动态对等计数之和=MaxDynDials。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(0), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(3), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(4), nil)}},
				},
				done: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(3), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(4), nil)},
				},
			},
//此轮中没有启动新的拨号任务，因为
//已达到MaxDynDials。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(0), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(3), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(4), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(5), nil)}},
				},
				done: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(5), nil)},
				},
				new: []task{
					&waitExpireTask{Duration: 14 * time.Second},
				},
			},
//在这一轮中，ID为2的对等机将关闭。查询
//重新使用上次发现查找的结果。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(0), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(3), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(4), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(5), nil)}},
				},
				new: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(6), nil)},
				},
			},
//更多的对等端（3，4）停止并拨出ID 6完成。
//重新使用发现查找的最后一个查询结果
//因为需要更多的候选人，所以产生了一个新的候选人。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(0), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(5), nil)}},
				},
				done: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(6), nil)},
				},
				new: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(7), nil)},
					&discoverTask{},
				},
			},
//对等7已连接，但仍然没有足够的动态对等
//（5个中有4个）。但是，发现已经在运行，因此请确保
//没有新的开始。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(0), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(5), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(7), nil)}},
				},
				done: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(7), nil)},
				},
			},
//使用空集完成正在运行的节点发现。一种新的查找
//应立即请求。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(0), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(5), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(7), nil)}},
				},
				done: []task{
					&discoverTask{},
				},
				new: []task{
					&discoverTask{},
				},
			},
		},
	})
}

//测试在没有连接对等机的情况下是否拨号引导节点，否则不拨号。
func TestDialStateDynDialBootnode(t *testing.T) {
	bootnodes := []*enode.Node{
		newNode(uintID(1), nil),
		newNode(uintID(2), nil),
		newNode(uintID(3), nil),
	}
	table := fakeTable{
		newNode(uintID(4), nil),
		newNode(uintID(5), nil),
		newNode(uintID(6), nil),
		newNode(uintID(7), nil),
		newNode(uintID(8), nil),
	}
	runDialTest(t, dialtest{
		init: newDialState(enode.ID{}, nil, bootnodes, table, 5, nil),
		rounds: []round{
//已尝试2个动态拨号，启动节点挂起回退间隔
			{
				new: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(4), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(5), nil)},
					&discoverTask{},
				},
			},
//没有拨号成功，引导节点仍挂起回退间隔
			{
				done: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(4), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(5), nil)},
				},
			},
//没有拨号成功，引导节点仍挂起回退间隔
			{},
//没有成功的拨号，尝试了2个动态拨号，同时达到了回退间隔，还尝试了1个引导节点。
			{
				new: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(1), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(4), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(5), nil)},
				},
			},
//没有拨号成功，尝试第二个bootnode
			{
				done: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(1), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(4), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(5), nil)},
				},
				new: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(2), nil)},
				},
			},
//没有拨号成功，尝试第三个启动节点
			{
				done: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(2), nil)},
				},
				new: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(3), nil)},
				},
			},
//没有拨号成功，再次尝试第一个bootnode，重试过期的随机节点
			{
				done: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(3), nil)},
				},
				new: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(1), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(4), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(5), nil)},
				},
			},
//随机拨号成功，不再尝试启动节点
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(4), nil)}},
				},
				done: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(1), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(4), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(5), nil)},
				},
			},
		},
	})
}

func TestDialStateDynDialFromTable(t *testing.T) {
//此表始终返回相同的随机节点
//按照下面给出的顺序。
	table := fakeTable{
		newNode(uintID(1), nil),
		newNode(uintID(2), nil),
		newNode(uintID(3), nil),
		newNode(uintID(4), nil),
		newNode(uintID(5), nil),
		newNode(uintID(6), nil),
		newNode(uintID(7), nil),
		newNode(uintID(8), nil),
	}

	runDialTest(t, dialtest{
		init: newDialState(enode.ID{}, nil, nil, table, 10, nil),
		rounds: []round{
//readrandomnodes返回的8个节点中有5个被拨号。
			{
				new: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(1), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(2), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(3), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(4), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(5), nil)},
					&discoverTask{},
				},
			},
//拨号节点1、2成功。启动查找中的拨号。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
				},
				done: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(1), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(2), nil)},
					&discoverTask{results: []*enode.Node{
						newNode(uintID(10), nil),
						newNode(uintID(11), nil),
						newNode(uintID(12), nil),
					}},
				},
				new: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(10), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(11), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(12), nil)},
					&discoverTask{},
				},
			},
//拨号节点3、4、5失败。查找中的拨号成功。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(10), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(11), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(12), nil)}},
				},
				done: []task{
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(3), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(4), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(5), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(10), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(11), nil)},
					&dialTask{flags: dynDialedConn, dest: newNode(uintID(12), nil)},
				},
			},
//正在等待到期。没有启动waitexpiretask，因为
//发现查询仍在运行。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(10), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(11), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(12), nil)}},
				},
			},
//节点3,4不会再次尝试，因为只有前两个
//返回的随机节点（节点1,2）将被尝试
//已经连接。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(10), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(11), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(12), nil)}},
				},
			},
		},
	})
}

func newNode(id enode.ID, ip net.IP) *enode.Node {
	var r enr.Record
	if ip != nil {
		r.Set(enr.IP(ip))
	}
	return enode.SignNull(&r, id)
}

//此测试检查是否未拨出与NetRestrict列表不匹配的候选人。
func TestDialStateNetRestrict(t *testing.T) {
//此表始终返回相同的随机节点
//按照下面给出的顺序。
	table := fakeTable{
		newNode(uintID(1), net.ParseIP("127.0.0.1")),
		newNode(uintID(2), net.ParseIP("127.0.0.2")),
		newNode(uintID(3), net.ParseIP("127.0.0.3")),
		newNode(uintID(4), net.ParseIP("127.0.0.4")),
		newNode(uintID(5), net.ParseIP("127.0.2.5")),
		newNode(uintID(6), net.ParseIP("127.0.2.6")),
		newNode(uintID(7), net.ParseIP("127.0.2.7")),
		newNode(uintID(8), net.ParseIP("127.0.2.8")),
	}
	restrict := new(netutil.Netlist)
	restrict.Add("127.0.2.0/24")

	runDialTest(t, dialtest{
		init: newDialState(enode.ID{}, nil, nil, table, 10, restrict),
		rounds: []round{
			{
				new: []task{
					&dialTask{flags: dynDialedConn, dest: table[4]},
					&discoverTask{},
				},
			},
		},
	})
}

//此测试检查是否启动了静态拨号。
func TestDialStateStaticDial(t *testing.T) {
	wantStatic := []*enode.Node{
		newNode(uintID(1), nil),
		newNode(uintID(2), nil),
		newNode(uintID(3), nil),
		newNode(uintID(4), nil),
		newNode(uintID(5), nil),
	}

	runDialTest(t, dialtest{
		init: newDialState(enode.ID{}, wantStatic, nil, fakeTable{}, 0, nil),
		rounds: []round{
//为以下节点启动静态拨号：
//尚未连接。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
				},
				new: []task{
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(3), nil)},
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(4), nil)},
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(5), nil)},
				},
			},
//由于所有静态任务
//节点已连接或仍在拨号。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(3), nil)}},
				},
				done: []task{
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(3), nil)},
				},
			},
//没有启动新的拨号任务，因为所有的拨号任务都是静态的
//节点现在已连接。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(3), nil)}},
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(4), nil)}},
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(5), nil)}},
				},
				done: []task{
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(4), nil)},
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(5), nil)},
				},
				new: []task{
					&waitExpireTask{Duration: 14 * time.Second},
				},
			},
//等待一轮拨号历史记录过期，不应生成新任务。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(3), nil)}},
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(4), nil)}},
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(5), nil)}},
				},
			},
//如果一个静态节点被删除，它应该立即被重新拨号，
//不管它最初是静态的还是动态的。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(3), nil)}},
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(5), nil)}},
				},
				new: []task{
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(2), nil)},
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(4), nil)},
				},
			},
		},
	})
}

//此测试检查静态对等点如果被重新添加到静态列表中，是否会立即重新启用。
func TestDialStaticAfterReset(t *testing.T) {
	wantStatic := []*enode.Node{
		newNode(uintID(1), nil),
		newNode(uintID(2), nil),
	}

	rounds := []round{
//为尚未连接的节点启动静态拨号。
		{
			peers: nil,
			new: []task{
				&dialTask{flags: staticDialedConn, dest: newNode(uintID(1), nil)},
				&dialTask{flags: staticDialedConn, dest: newNode(uintID(2), nil)},
			},
		},
//没有新的拨号任务，所有对等端都已连接。
		{
			peers: []*Peer{
				{rw: &conn{flags: staticDialedConn, node: newNode(uintID(1), nil)}},
				{rw: &conn{flags: staticDialedConn, node: newNode(uintID(2), nil)}},
			},
			done: []task{
				&dialTask{flags: staticDialedConn, dest: newNode(uintID(1), nil)},
				&dialTask{flags: staticDialedConn, dest: newNode(uintID(2), nil)},
			},
			new: []task{
				&waitExpireTask{Duration: 30 * time.Second},
			},
		},
	}
	dTest := dialtest{
		init:   newDialState(enode.ID{}, wantStatic, nil, fakeTable{}, 0, nil),
		rounds: rounds,
	}
	runDialTest(t, dTest)
	for _, n := range wantStatic {
		dTest.init.removeStatic(n)
		dTest.init.addStatic(n)
	}
//如果不删除同龄人，他们将被视为最近拨打过电话。
	runDialTest(t, dTest)
}

//此测试检查过去的拨号是否有一段时间没有重试。
func TestDialStateCache(t *testing.T) {
	wantStatic := []*enode.Node{
		newNode(uintID(1), nil),
		newNode(uintID(2), nil),
		newNode(uintID(3), nil),
	}

	runDialTest(t, dialtest{
		init: newDialState(enode.ID{}, wantStatic, nil, fakeTable{}, 0, nil),
		rounds: []round{
//为以下节点启动静态拨号：
//尚未连接。
			{
				peers: nil,
				new: []task{
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(1), nil)},
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(2), nil)},
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(3), nil)},
				},
			},
//由于所有静态任务
//节点已连接或仍在拨号。
			{
				peers: []*Peer{
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: staticDialedConn, node: newNode(uintID(2), nil)}},
				},
				done: []task{
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(1), nil)},
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(2), nil)},
				},
			},
//启动补救任务以等待节点3的历史记录
//条目将过期。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
				},
				done: []task{
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(3), nil)},
				},
				new: []task{
					&waitExpireTask{Duration: 14 * time.Second},
				},
			},
//
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
				},
			},
//节点3的缓存项已过期并重试。
			{
				peers: []*Peer{
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(1), nil)}},
					{rw: &conn{flags: dynDialedConn, node: newNode(uintID(2), nil)}},
				},
				new: []task{
					&dialTask{flags: staticDialedConn, dest: newNode(uintID(3), nil)},
				},
			},
		},
	})
}

func TestDialResolve(t *testing.T) {
	resolved := newNode(uintID(1), net.IP{127, 0, 55, 234})
	table := &resolveMock{answer: resolved}
	state := newDialState(enode.ID{}, nil, nil, table, 0, nil)

//检查是否使用不完整的ID生成任务。
	dest := newNode(uintID(1), nil)
	state.addStatic(dest)
	tasks := state.newTasks(0, nil, time.Time{})
	if !reflect.DeepEqual(tasks, []task{&dialTask{flags: staticDialedConn, dest: dest}}) {
		t.Fatalf("expected dial task, got %#v", tasks)
	}

//现在运行任务，它应该解析一次ID。
	config := Config{Dialer: TCPDialer{&net.Dialer{Deadline: time.Now().Add(-5 * time.Minute)}}}
	srv := &Server{ntab: table, Config: config}
	tasks[0].Do(srv)
	if !reflect.DeepEqual(table.resolveCalls, []*enode.Node{dest}) {
		t.Fatalf("wrong resolve calls, got %v", table.resolveCalls)
	}

//向拨号程序报告完成情况，拨号程序应更新静态节点记录。
	state.taskDone(tasks[0], time.Now())
	if state.static[uintID(1)].dest != resolved {
		t.Fatalf("state.dest not updated")
	}
}

//比较任务列表，但不关心顺序。
func sametasks(a, b []task) bool {
	if len(a) != len(b) {
		return false
	}
next:
	for _, ta := range a {
		for _, tb := range b {
			if reflect.DeepEqual(ta, tb) {
				continue next
			}
		}
		return false
	}
	return true
}

func uintID(i uint32) enode.ID {
	var id enode.ID
	binary.BigEndian.PutUint32(id[:], i)
	return id
}

//实现TestDialResolve的discovertable
type resolveMock struct {
	resolveCalls []*enode.Node
	answer       *enode.Node
}

func (t *resolveMock) Resolve(n *enode.Node) *enode.Node {
	t.resolveCalls = append(t.resolveCalls, n)
	return t.answer
}

func (t *resolveMock) Self() *enode.Node                     { return new(enode.Node) }
func (t *resolveMock) Close()                                {}
func (t *resolveMock) LookupRandom() []*enode.Node           { return nil }
func (t *resolveMock) ReadRandomNodes(buf []*enode.Node) int { return 0 }
