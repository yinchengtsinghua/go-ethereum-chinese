
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

package simulation

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/network"
)

func TestUpDownNodeIDs(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	ids, err := sim.AddNodes(10)
	if err != nil {
		t.Fatal(err)
	}

	gotIDs := sim.NodeIDs()

	if !equalNodeIDs(ids, gotIDs) {
		t.Error("returned nodes are not equal to added ones")
	}

	stoppedIDs, err := sim.StopRandomNodes(3)
	if err != nil {
		t.Fatal(err)
	}

	gotIDs = sim.UpNodeIDs()

	for _, id := range gotIDs {
		if !sim.Net.GetNode(id).Up {
			t.Errorf("node %s should not be down", id)
		}
	}

	if !equalNodeIDs(ids, append(gotIDs, stoppedIDs...)) {
		t.Error("returned nodes are not equal to added ones")
	}

	gotIDs = sim.DownNodeIDs()

	for _, id := range gotIDs {
		if sim.Net.GetNode(id).Up {
			t.Errorf("node %s should not be up", id)
		}
	}

	if !equalNodeIDs(stoppedIDs, gotIDs) {
		t.Error("returned nodes are not equal to the stopped ones")
	}
}

func equalNodeIDs(one, other []enode.ID) bool {
	if len(one) != len(other) {
		return false
	}
	var count int
	for _, a := range one {
		var found bool
		for _, b := range other {
			if a == b {
				found = true
				break
			}
		}
		if found {
			count++
		} else {
			return false
		}
	}
	return count == len(one)
}

func TestAddNode(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	id, err := sim.AddNode()
	if err != nil {
		t.Fatal(err)
	}

	n := sim.Net.GetNode(id)
	if n == nil {
		t.Fatal("node not found")
	}

	if !n.Up {
		t.Error("node not started")
	}
}

func TestAddNodeWithMsgEvents(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	id, err := sim.AddNode(AddNodeWithMsgEvents(true))
	if err != nil {
		t.Fatal(err)
	}

	if !sim.Net.GetNode(id).Config.EnableMsgEvents {
		t.Error("EnableMsgEvents is false")
	}

	id, err = sim.AddNode(AddNodeWithMsgEvents(false))
	if err != nil {
		t.Fatal(err)
	}

	if sim.Net.GetNode(id).Config.EnableMsgEvents {
		t.Error("EnableMsgEvents is true")
	}
}

func TestAddNodeWithService(t *testing.T) {
	sim := New(map[string]ServiceFunc{
		"noop1": noopServiceFunc,
		"noop2": noopServiceFunc,
	})
	defer sim.Close()

	id, err := sim.AddNode(AddNodeWithService("noop1"))
	if err != nil {
		t.Fatal(err)
	}

	n := sim.Net.GetNode(id).Node.(*adapters.SimNode)
	if n.Service("noop1") == nil {
		t.Error("service noop1 not found on node")
	}
	if n.Service("noop2") != nil {
		t.Error("service noop2 should not be found on node")
	}
}

func TestAddNodeMultipleServices(t *testing.T) {
	sim := New(map[string]ServiceFunc{
		"noop1": noopServiceFunc,
		"noop2": noopService2Func,
	})
	defer sim.Close()

	id, err := sim.AddNode()
	if err != nil {
		t.Fatal(err)
	}

	n := sim.Net.GetNode(id).Node.(*adapters.SimNode)
	if n.Service("noop1") == nil {
		t.Error("service noop1 not found on node")
	}
	if n.Service("noop2") == nil {
		t.Error("service noop2 not found on node")
	}
}

func TestAddNodeDuplicateServiceError(t *testing.T) {
	sim := New(map[string]ServiceFunc{
		"noop1": noopServiceFunc,
		"noop2": noopServiceFunc,
	})
	defer sim.Close()

	wantErr := "duplicate service: *simulation.noopService"
	_, err := sim.AddNode()
	if err.Error() != wantErr {
		t.Errorf("got error %q, want %q", err, wantErr)
	}
}

func TestAddNodes(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	nodesCount := 12

	ids, err := sim.AddNodes(nodesCount)
	if err != nil {
		t.Fatal(err)
	}

	count := len(ids)
	if count != nodesCount {
		t.Errorf("expected %v nodes, got %v", nodesCount, count)
	}

	count = len(sim.Net.GetNodes())
	if count != nodesCount {
		t.Errorf("expected %v nodes, got %v", nodesCount, count)
	}
}

func TestAddNodesAndConnectFull(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	n := 12

	ids, err := sim.AddNodesAndConnectFull(n)
	if err != nil {
		t.Fatal(err)
	}

	simulations.VerifyFull(t, sim.Net, ids)
}

func TestAddNodesAndConnectChain(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	_, err := sim.AddNodesAndConnectChain(12)
	if err != nil {
		t.Fatal(err)
	}

//添加另一组要测试的节点
//如果连接两条链条
	_, err = sim.AddNodesAndConnectChain(7)
	if err != nil {
		t.Fatal(err)
	}

	simulations.VerifyChain(t, sim.Net, sim.UpNodeIDs())
}

func TestAddNodesAndConnectRing(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	ids, err := sim.AddNodesAndConnectRing(12)
	if err != nil {
		t.Fatal(err)
	}

	simulations.VerifyRing(t, sim.Net, ids)
}

func TestAddNodesAndConnectStar(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	ids, err := sim.AddNodesAndConnectStar(12)
	if err != nil {
		t.Fatal(err)
	}

	simulations.VerifyStar(t, sim.Net, ids, 0)
}

//测试上载快照是否有效
func TestUploadSnapshot(t *testing.T) {
	log.Debug("Creating simulation")
	s := New(map[string]ServiceFunc{
		"bzz": func(ctx *adapters.ServiceContext, b *sync.Map) (node.Service, func(), error) {
			addr := network.NewAddr(ctx.Config.Node())
			hp := network.NewHiveParams()
			hp.Discovery = false
			config := &network.BzzConfig{
				OverlayAddr:  addr.Over(),
				UnderlayAddr: addr.Under(),
				HiveParams:   hp,
			}
			kad := network.NewKademlia(addr.Over(), network.NewKadParams())
			return network.NewBzz(config, kad, nil, nil, nil), nil, nil
		},
	})
	defer s.Close()

	nodeCount := 16
	log.Debug("Uploading snapshot")
	err := s.UploadSnapshot(fmt.Sprintf("../stream/testing/snapshot_%d.json", nodeCount))
	if err != nil {
		t.Fatalf("Error uploading snapshot to simulation network: %v", err)
	}

	ctx := context.Background()
	log.Debug("Starting simulation...")
	s.Run(ctx, func(ctx context.Context, sim *Simulation) error {
		log.Debug("Checking")
		nodes := sim.UpNodeIDs()
		if len(nodes) != nodeCount {
			t.Fatal("Simulation network node number doesn't match snapshot node number")
		}
		return nil
	})
	log.Debug("Done.")
}

func TestStartStopNode(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	id, err := sim.AddNode()
	if err != nil {
		t.Fatal(err)
	}

	n := sim.Net.GetNode(id)
	if n == nil {
		t.Fatal("node not found")
	}
	if !n.Up {
		t.Error("node not started")
	}

	err = sim.StopNode(id)
	if err != nil {
		t.Fatal(err)
	}
	if n.Up {
		t.Error("node not stopped")
	}

//在此休眠以确保network.watchpeerevents延迟功能
//在重新启动节点之前设置了“node.up=false”。
//P2P/模拟/网络。go:215
//
//相同节点停止并再次启动，并且在启动时
//WatchPeerEvents在Goroutine中启动。如果节点停止
//然后很快就开始了，Goroutine可能会被安排在后面
//然后启动并强制其defer函数中的“node.up=false”。
//这将使测试不可靠。
	time.Sleep(time.Second)

	err = sim.StartNode(id)
	if err != nil {
		t.Fatal(err)
	}
	if !n.Up {
		t.Error("node not started")
	}
}

func TestStartStopRandomNode(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	_, err := sim.AddNodes(3)
	if err != nil {
		t.Fatal(err)
	}

	id, err := sim.StopRandomNode()
	if err != nil {
		t.Fatal(err)
	}

	n := sim.Net.GetNode(id)
	if n == nil {
		t.Fatal("node not found")
	}
	if n.Up {
		t.Error("node not stopped")
	}

	id2, err := sim.StopRandomNode()
	if err != nil {
		t.Fatal(err)
	}

//在此休眠以确保network.watchpeerevents延迟功能
//在重新启动节点之前设置了“node.up=false”。
//P2P/模拟/网络。go:215
//
//相同节点停止并再次启动，并且在启动时
//WatchPeerEvents在Goroutine中启动。如果节点停止
//然后很快就开始了，Goroutine可能会被安排在后面
//然后启动并强制其defer函数中的“node.up=false”。
//这将使测试不可靠。
	time.Sleep(time.Second)

	idStarted, err := sim.StartRandomNode()
	if err != nil {
		t.Fatal(err)
	}

	if idStarted != id && idStarted != id2 {
		t.Error("unexpected started node ID")
	}
}

func TestStartStopRandomNodes(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	_, err := sim.AddNodes(10)
	if err != nil {
		t.Fatal(err)
	}

	ids, err := sim.StopRandomNodes(3)
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range ids {
		n := sim.Net.GetNode(id)
		if n == nil {
			t.Fatal("node not found")
		}
		if n.Up {
			t.Error("node not stopped")
		}
	}

//在此休眠以确保network.watchpeerevents延迟功能
//在重新启动节点之前设置了“node.up=false”。
//P2P/模拟/网络。go:215
//
//相同节点停止并再次启动，并且在启动时
//WatchPeerEvents在Goroutine中启动。如果节点停止
//然后很快就开始了，Goroutine可能会被安排在后面
//然后启动并强制其defer函数中的“node.up=false”。
//这将使测试不可靠。
	time.Sleep(time.Second)

	ids, err = sim.StartRandomNodes(2)
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range ids {
		n := sim.Net.GetNode(id)
		if n == nil {
			t.Fatal("node not found")
		}
		if !n.Up {
			t.Error("node not started")
		}
	}
}
