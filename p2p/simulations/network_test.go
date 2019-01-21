
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

package simulations

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
)

//测试使用最小服务创建的快照只包含预期的连接
//当加载此快照时，网络仅包含这些相同的连接
func TestSnapshot(t *testing.T) {

//第一部分
//从环网创建快照

//这是一个最小的服务，其协议在退出前只接受一条消息或关闭连接
	adapter := adapters.NewSimAdapter(adapters.Services{
		"noopwoop": func(ctx *adapters.ServiceContext) (node.Service, error) {
			return NewNoopService(nil), nil
		},
	})

//创建网络
	network := NewNetwork(adapter, &NetworkConfig{
		DefaultService: "noopwoop",
	})
//\t要考虑成为网络成员，请在关机时设置为true threadsafe
	runningOne := true
	defer func() {
		if runningOne {
			network.Shutdown()
		}
	}()

//创建和启动节点
	nodeCount := 20
	ids := make([]enode.ID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		conf := adapters.RandomNodeConfig()
		node, err := network.NewNodeWithConfig(conf)
		if err != nil {
			t.Fatalf("error creating node: %s", err)
		}
		if err := network.Start(node.ID()); err != nil {
			t.Fatalf("error starting node: %s", err)
		}
		ids[i] = node.ID()
	}

//订阅对等事件
	evC := make(chan *Event)
	sub := network.Events().Subscribe(evC)
	defer sub.Unsubscribe()

//连接环中的节点
//生成单独的线程以避免事件侦听器中的死锁
	go func() {
		for i, id := range ids {
			peerID := ids[(i+1)%len(ids)]
			if err := network.Connect(id, peerID); err != nil {
				t.Fatal(err)
			}
		}
	}()

//收集达到预期数量的连接事件
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()
	checkIds := make(map[enode.ID][]enode.ID)
	connEventCount := nodeCount
OUTER:
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case ev := <-evC:
			if ev.Type == EventTypeConn && !ev.Control {

//断开时失败
				if !ev.Conn.Up {
					t.Fatalf("unexpected disconnect: %v -> %v", ev.Conn.One, ev.Conn.Other)
				}
				checkIds[ev.Conn.One] = append(checkIds[ev.Conn.One], ev.Conn.Other)
				checkIds[ev.Conn.Other] = append(checkIds[ev.Conn.Other], ev.Conn.One)
				connEventCount--
				log.Debug("ev", "count", connEventCount)
				if connEventCount == 0 {
					break OUTER
				}
			}
		}
	}

//创建当前网络的快照
	snap, err := network.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	j, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	log.Debug("snapshot taken", "nodes", len(snap.Nodes), "conns", len(snap.Conns), "json", string(j))

//验证对齐元素编号是否签出
	if len(checkIds) != len(snap.Conns) || len(checkIds) != len(snap.Nodes) {
		t.Fatalf("snapshot wrong node,conn counts %d,%d != %d", len(snap.Nodes), len(snap.Conns), len(checkIds))
	}

//关闭SIM网络
	runningOne = false
	sub.Unsubscribe()
	network.Shutdown()

//检查快照中是否有所有预期的连接
	for nodid, nodConns := range checkIds {
		for _, nodConn := range nodConns {
			var match bool
			for _, snapConn := range snap.Conns {
				if snapConn.One == nodid && snapConn.Other == nodConn {
					match = true
					break
				} else if snapConn.Other == nodid && snapConn.One == nodConn {
					match = true
					break
				}
			}
			if !match {
				t.Fatalf("snapshot missing conn %v -> %v", nodid, nodConn)
			}
		}
	}
	log.Info("snapshot checked")

//第二部分
//加载快照并验证是否形成完全相同的连接

	adapter = adapters.NewSimAdapter(adapters.Services{
		"noopwoop": func(ctx *adapters.ServiceContext) (node.Service, error) {
			return NewNoopService(nil), nil
		},
	})
	network = NewNetwork(adapter, &NetworkConfig{
		DefaultService: "noopwoop",
	})
	defer func() {
		network.Shutdown()
	}()

//订阅对等事件
//每个节点启动和连接事件将生成一个额外的控制事件
//因此，将计数乘以2
	evC = make(chan *Event, (len(snap.Conns)*2)+(len(snap.Nodes)*2))
	sub = network.Events().Subscribe(evC)
	defer sub.Unsubscribe()

//加载快照
//生成单独的线程以避免事件侦听器中的死锁
	err = network.Load(snap)
	if err != nil {
		t.Fatal(err)
	}

//收集达到预期数量的连接事件
	ctx, cancel = context.WithTimeout(context.TODO(), time.Second*3)
	defer cancel()

	connEventCount = nodeCount

OUTER_TWO:
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case ev := <-evC:
			if ev.Type == EventTypeConn && !ev.Control {

//断开时失败
				if !ev.Conn.Up {
					t.Fatalf("unexpected disconnect: %v -> %v", ev.Conn.One, ev.Conn.Other)
				}
				log.Debug("conn", "on", ev.Conn.One, "other", ev.Conn.Other)
				checkIds[ev.Conn.One] = append(checkIds[ev.Conn.One], ev.Conn.Other)
				checkIds[ev.Conn.Other] = append(checkIds[ev.Conn.Other], ev.Conn.One)
				connEventCount--
				log.Debug("ev", "count", connEventCount)
				if connEventCount == 0 {
					break OUTER_TWO
				}
			}
		}
	}

//检查网络中的所有预期连接
	for _, snapConn := range snap.Conns {
		var match bool
		for nodid, nodConns := range checkIds {
			for _, nodConn := range nodConns {
				if snapConn.One == nodid && snapConn.Other == nodConn {
					match = true
					break
				} else if snapConn.Other == nodid && snapConn.One == nodConn {
					match = true
					break
				}
			}
		}
		if !match {
			t.Fatalf("network missing conn %v -> %v", snapConn.One, snapConn.Other)
		}
	}

//验证在合理时间内收集到的连接事件之后，网络没有生成任何其他附加连接事件。
	ctx, cancel = context.WithTimeout(context.TODO(), time.Second)
	defer cancel()
	select {
	case <-ctx.Done():
	case ev := <-evC:
		if ev.Type == EventTypeConn {
			t.Fatalf("Superfluous conn found %v -> %v", ev.Conn.One, ev.Conn.Other)
		}
	}

//此测试验证快照中的所有连接
//在网络中创建。
	t.Run("conns after load", func(t *testing.T) {
//创建新网络。
		n := NewNetwork(
			adapters.NewSimAdapter(adapters.Services{
				"noopwoop": func(ctx *adapters.ServiceContext) (node.Service, error) {
					return NewNoopService(nil), nil
				},
			}),
			&NetworkConfig{
				DefaultService: "noopwoop",
			},
		)
		defer n.Shutdown()

//加载同一快照。
		err := n.Load(snap)
		if err != nil {
			t.Fatal(err)
		}

//检查快照中的每个连接
//如果它也在网络中。
		for _, c := range snap.Conns {
			if n.GetConn(c.One, c.Other) == nil {
				t.Errorf("missing connection: %s -> %s", c.One, c.Other)
			}
		}
	})
}

//TestNetworkSimulation使用每个节点创建多节点仿真网络
//在环形拓扑中连接，检查所有节点是否成功握手
//彼此之间，快照完全代表所需的拓扑
func TestNetworkSimulation(t *testing.T) {
//使用20个testservice节点创建模拟网络
	adapter := adapters.NewSimAdapter(adapters.Services{
		"test": newTestService,
	})
	network := NewNetwork(adapter, &NetworkConfig{
		DefaultService: "test",
	})
	defer network.Shutdown()
	nodeCount := 20
	ids := make([]enode.ID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		conf := adapters.RandomNodeConfig()
		node, err := network.NewNodeWithConfig(conf)
		if err != nil {
			t.Fatalf("error creating node: %s", err)
		}
		if err := network.Start(node.ID()); err != nil {
			t.Fatalf("error starting node: %s", err)
		}
		ids[i] = node.ID()
	}

//执行连接环中节点的检查（因此每个节点
//然后检查所有节点
//通过检查他们的对等计数进行了两次握手
	action := func(_ context.Context) error {
		for i, id := range ids {
			peerID := ids[(i+1)%len(ids)]
			if err := network.Connect(id, peerID); err != nil {
				return err
			}
		}
		return nil
	}
	check := func(ctx context.Context, id enode.ID) (bool, error) {
//检查一下我们的时间没有用完
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

//获取节点
		node := network.GetNode(id)
		if node == nil {
			return false, fmt.Errorf("unknown node: %s", id)
		}

//检查它是否有两个同龄人
		client, err := node.Client()
		if err != nil {
			return false, err
		}
		var peerCount int64
		if err := client.CallContext(ctx, &peerCount, "test_peerCount"); err != nil {
			return false, err
		}
		switch {
		case peerCount < 2:
			return false, nil
		case peerCount == 2:
			return true, nil
		default:
			return false, fmt.Errorf("unexpected peerCount: %d", peerCount)
		}
	}

	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

//每100毫秒触发一次检查
	trigger := make(chan enode.ID)
	go triggerChecks(ctx, ids, trigger, 100*time.Millisecond)

	result := NewSimulation(network).Run(ctx, &Step{
		Action:  action,
		Trigger: trigger,
		Expect: &Expectation{
			Nodes: ids,
			Check: check,
		},
	})
	if result.Error != nil {
		t.Fatalf("simulation failed: %s", result.Error)
	}

//获取网络快照并检查它是否包含正确的拓扑
	snap, err := network.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Nodes) != nodeCount {
		t.Fatalf("expected snapshot to contain %d nodes, got %d", nodeCount, len(snap.Nodes))
	}
	if len(snap.Conns) != nodeCount {
		t.Fatalf("expected snapshot to contain %d connections, got %d", nodeCount, len(snap.Conns))
	}
	for i, id := range ids {
		conn := snap.Conns[i]
		if conn.One != id {
			t.Fatalf("expected conn[%d].One to be %s, got %s", i, id, conn.One)
		}
		peerID := ids[(i+1)%len(ids)]
		if conn.Other != peerID {
			t.Fatalf("expected conn[%d].Other to be %s, got %s", i, peerID, conn.Other)
		}
	}
}

func triggerChecks(ctx context.Context, ids []enode.ID, trigger chan enode.ID, interval time.Duration) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			for _, id := range ids {
				select {
				case trigger <- id:
				case <-ctx.Done():
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

//\todo:重构以实现shapshots
//一旦将配置方法从
//swarm/网络/仿真/connect.go
func BenchmarkMinimalService(b *testing.B) {
	b.Run("ring/32", benchmarkMinimalServiceTmp)
}

func benchmarkMinimalServiceTmp(b *testing.B) {

//停止计时器以丢弃设置时间污染
	args := strings.Split(b.Name(), "/")
	nodeCount, err := strconv.ParseInt(args[2], 10, 16)
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
//这是一个最小的服务，其协议将在协议运行时关闭通道。
//使服务启动和协议实际运行所需的时间成为可能
		protoCMap := make(map[enode.ID]map[enode.ID]chan struct{})
		adapter := adapters.NewSimAdapter(adapters.Services{
			"noopwoop": func(ctx *adapters.ServiceContext) (node.Service, error) {
				protoCMap[ctx.Config.ID] = make(map[enode.ID]chan struct{})
				svc := NewNoopService(protoCMap[ctx.Config.ID])
				return svc, nil
			},
		})

//创建网络
		network := NewNetwork(adapter, &NetworkConfig{
			DefaultService: "noopwoop",
		})
		defer network.Shutdown()

//创建和启动节点
		ids := make([]enode.ID, nodeCount)
		for i := 0; i < int(nodeCount); i++ {
			conf := adapters.RandomNodeConfig()
			node, err := network.NewNodeWithConfig(conf)
			if err != nil {
				b.Fatalf("error creating node: %s", err)
			}
			if err := network.Start(node.ID()); err != nil {
				b.Fatalf("error starting node: %s", err)
			}
			ids[i] = node.ID()
		}

//准备，设置，去
		b.ResetTimer()

//连接环中的节点
		for i, id := range ids {
			peerID := ids[(i+1)%len(ids)]
			if err := network.Connect(id, peerID); err != nil {
				b.Fatal(err)
			}
		}

//等待所有协议发出关闭信号
		ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
		defer cancel()
		for nodid, peers := range protoCMap {
			for peerid, peerC := range peers {
				log.Debug("getting ", "node", nodid, "peer", peerid)
				select {
				case <-ctx.Done():
					b.Fatal(ctx.Err())
				case <-peerC:
				}
			}
		}
	}
}
