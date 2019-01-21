
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

package discovery

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/state"
	colorable "github.com/mattn/go-colorable"
)

//servicename与exec适配器一起使用，因此exec'd二进制文件知道
//要执行的服务
const serviceName = "discovery"
const testNeighbourhoodSize = 2
const discoveryPersistenceDatadir = "discovery_persistence_test_store"

var discoveryPersistencePath = path.Join(os.TempDir(), discoveryPersistenceDatadir)
var discoveryEnabled = true
var persistenceEnabled = false

var services = adapters.Services{
	serviceName: newService,
}

func cleanDbStores() error {
	entries, err := ioutil.ReadDir(os.TempDir())
	if err != nil {
		return err
	}

	for _, f := range entries {
		if strings.HasPrefix(f.Name(), discoveryPersistenceDatadir) {
			os.RemoveAll(path.Join(os.TempDir(), f.Name()))
		}
	}
	return nil

}

func getDbStore(nodeID string) (*state.DBStore, error) {
	if _, err := os.Stat(discoveryPersistencePath + "_" + nodeID); os.IsNotExist(err) {
		log.Info(fmt.Sprintf("directory for nodeID %s does not exist. creating...", nodeID))
		ioutil.TempDir("", discoveryPersistencePath+"_"+nodeID)
	}
	log.Info(fmt.Sprintf("opening storage directory for nodeID %s", nodeID))
	store, err := state.NewDBStore(discoveryPersistencePath + "_" + nodeID)
	if err != nil {
		return nil, err
	}
	return store, nil
}

var (
	nodeCount = flag.Int("nodes", 32, "number of nodes to create (default 32)")
	initCount = flag.Int("conns", 1, "number of originally connected peers	 (default 1)")
	loglevel  = flag.Int("loglevel", 3, "verbosity of logs")
	rawlog    = flag.Bool("rawlog", false, "remove terminal formatting from logs")
)

func init() {
	flag.Parse()
//注册将作为devp2p运行的发现服务
//使用Exec适配器时的协议
	adapters.RegisterServices(services)

	log.PrintOrigins(true)
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(!*rawlog))))
}

//测试n节点环所需平均时间的基准
//充分利用健康的卡德米利亚拓扑
func BenchmarkDiscovery_8_1(b *testing.B)   { benchmarkDiscovery(b, 8, 1) }
func BenchmarkDiscovery_16_1(b *testing.B)  { benchmarkDiscovery(b, 16, 1) }
func BenchmarkDiscovery_32_1(b *testing.B)  { benchmarkDiscovery(b, 32, 1) }
func BenchmarkDiscovery_64_1(b *testing.B)  { benchmarkDiscovery(b, 64, 1) }
func BenchmarkDiscovery_128_1(b *testing.B) { benchmarkDiscovery(b, 128, 1) }
func BenchmarkDiscovery_256_1(b *testing.B) { benchmarkDiscovery(b, 256, 1) }

func BenchmarkDiscovery_8_2(b *testing.B)   { benchmarkDiscovery(b, 8, 2) }
func BenchmarkDiscovery_16_2(b *testing.B)  { benchmarkDiscovery(b, 16, 2) }
func BenchmarkDiscovery_32_2(b *testing.B)  { benchmarkDiscovery(b, 32, 2) }
func BenchmarkDiscovery_64_2(b *testing.B)  { benchmarkDiscovery(b, 64, 2) }
func BenchmarkDiscovery_128_2(b *testing.B) { benchmarkDiscovery(b, 128, 2) }
func BenchmarkDiscovery_256_2(b *testing.B) { benchmarkDiscovery(b, 256, 2) }

func BenchmarkDiscovery_8_4(b *testing.B)   { benchmarkDiscovery(b, 8, 4) }
func BenchmarkDiscovery_16_4(b *testing.B)  { benchmarkDiscovery(b, 16, 4) }
func BenchmarkDiscovery_32_4(b *testing.B)  { benchmarkDiscovery(b, 32, 4) }
func BenchmarkDiscovery_64_4(b *testing.B)  { benchmarkDiscovery(b, 64, 4) }
func BenchmarkDiscovery_128_4(b *testing.B) { benchmarkDiscovery(b, 128, 4) }
func BenchmarkDiscovery_256_4(b *testing.B) { benchmarkDiscovery(b, 256, 4) }

func TestDiscoverySimulationExecAdapter(t *testing.T) {
	testDiscoverySimulationExecAdapter(t, *nodeCount, *initCount)
}

func testDiscoverySimulationExecAdapter(t *testing.T, nodes, conns int) {
	baseDir, err := ioutil.TempDir("", "swarm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(baseDir)
	testDiscoverySimulation(t, nodes, conns, adapters.NewExecAdapter(baseDir))
}

func TestDiscoverySimulationSimAdapter(t *testing.T) {
	testDiscoverySimulationSimAdapter(t, *nodeCount, *initCount)
}

func TestDiscoveryPersistenceSimulationSimAdapter(t *testing.T) {
	testDiscoveryPersistenceSimulationSimAdapter(t, *nodeCount, *initCount)
}

func testDiscoveryPersistenceSimulationSimAdapter(t *testing.T, nodes, conns int) {
	testDiscoveryPersistenceSimulation(t, nodes, conns, adapters.NewSimAdapter(services))
}

func testDiscoverySimulationSimAdapter(t *testing.T, nodes, conns int) {
	testDiscoverySimulation(t, nodes, conns, adapters.NewSimAdapter(services))
}

func testDiscoverySimulation(t *testing.T, nodes, conns int, adapter adapters.NodeAdapter) {
	startedAt := time.Now()
	result, err := discoverySimulation(nodes, conns, adapter)
	if err != nil {
		t.Fatalf("Setting up simulation failed: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("Simulation failed: %s", result.Error)
	}
	t.Logf("Simulation with %d nodes passed in %s", nodes, result.FinishedAt.Sub(result.StartedAt))
	var min, max time.Duration
	var sum int
	for _, pass := range result.Passes {
		duration := pass.Sub(result.StartedAt)
		if sum == 0 || duration < min {
			min = duration
		}
		if duration > max {
			max = duration
		}
		sum += int(duration.Nanoseconds())
	}
	t.Logf("Min: %s, Max: %s, Average: %s", min, max, time.Duration(sum/len(result.Passes))*time.Nanosecond)
	finishedAt := time.Now()
	t.Logf("Setup: %s, shutdown: %s", result.StartedAt.Sub(startedAt), finishedAt.Sub(result.FinishedAt))
}

func testDiscoveryPersistenceSimulation(t *testing.T, nodes, conns int, adapter adapters.NodeAdapter) map[int][]byte {
	persistenceEnabled = true
	discoveryEnabled = true

	result, err := discoveryPersistenceSimulation(nodes, conns, adapter)

	if err != nil {
		t.Fatalf("Setting up simulation failed: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("Simulation failed: %s", result.Error)
	}
	t.Logf("Simulation with %d nodes passed in %s", nodes, result.FinishedAt.Sub(result.StartedAt))
//再次将发现和持久性标志设置为默认值，以便
//测试不会受到影响
	discoveryEnabled = true
	persistenceEnabled = false
	return nil
}

func benchmarkDiscovery(b *testing.B, nodes, conns int) {
	for i := 0; i < b.N; i++ {
		result, err := discoverySimulation(nodes, conns, adapters.NewSimAdapter(services))
		if err != nil {
			b.Fatalf("setting up simulation failed: %v", err)
		}
		if result.Error != nil {
			b.Logf("simulation failed: %s", result.Error)
		}
	}
}

func discoverySimulation(nodes, conns int, adapter adapters.NodeAdapter) (*simulations.StepResult, error) {
//创建网络
	net := simulations.NewNetwork(adapter, &simulations.NetworkConfig{
		ID:             "0",
		DefaultService: serviceName,
	})
	defer net.Shutdown()
	trigger := make(chan enode.ID)
	ids := make([]enode.ID, nodes)
	for i := 0; i < nodes; i++ {
		conf := adapters.RandomNodeConfig()
		node, err := net.NewNodeWithConfig(conf)
		if err != nil {
			return nil, fmt.Errorf("error starting node: %s", err)
		}
		if err := net.Start(node.ID()); err != nil {
			return nil, fmt.Errorf("error starting node %s: %s", node.ID().TerminalString(), err)
		}
		if err := triggerChecks(trigger, net, node.ID()); err != nil {
			return nil, fmt.Errorf("error triggering checks for node %s: %s", node.ID().TerminalString(), err)
		}
		ids[i] = node.ID()
	}

//运行一个模拟，将一个环中的10个节点连接起来并等待
//用于完全对等机发现
	var addrs [][]byte
	action := func(ctx context.Context) error {
		return nil
	}
	for i := range ids {
//收集覆盖地址，至
		addrs = append(addrs, ids[i].Bytes())
	}
	err := net.ConnectNodesChain(nil)
	if err != nil {
		return nil, err
	}
	log.Debug(fmt.Sprintf("nodes: %v", len(addrs)))
//建造对等壶，以便检查卡德米利亚的健康状况。
	ppmap := network.NewPeerPotMap(network.NewKadParams().NeighbourhoodSize, addrs)
	check := func(ctx context.Context, id enode.ID) (bool, error) {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		node := net.GetNode(id)
		if node == nil {
			return false, fmt.Errorf("unknown node: %s", id)
		}
		client, err := node.Client()
		if err != nil {
			return false, fmt.Errorf("error getting node client: %s", err)
		}

		healthy := &network.Health{}
		if err := client.Call(&healthy, "hive_healthy", ppmap[common.Bytes2Hex(id.Bytes())]); err != nil {
			return false, fmt.Errorf("error getting node health: %s", err)
		}
		log.Debug(fmt.Sprintf("node %4s healthy: connected nearest neighbours: %v, know nearest neighbours: %v,\n\n%v", id, healthy.ConnectNN, healthy.KnowNN, healthy.Hive))
		return healthy.KnowNN && healthy.ConnectNN, nil
	}

//64节点~1min
//128节点~
	timeout := 300 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	result := simulations.NewSimulation(net).Run(ctx, &simulations.Step{
		Action:  action,
		Trigger: trigger,
		Expect: &simulations.Expectation{
			Nodes: ids,
			Check: check,
		},
	})
	if result.Error != nil {
		return result, nil
	}
	return result, nil
}

func discoveryPersistenceSimulation(nodes, conns int, adapter adapters.NodeAdapter) (*simulations.StepResult, error) {
	cleanDbStores()
	defer cleanDbStores()

//创建网络
	net := simulations.NewNetwork(adapter, &simulations.NetworkConfig{
		ID:             "0",
		DefaultService: serviceName,
	})
	defer net.Shutdown()
	trigger := make(chan enode.ID)
	ids := make([]enode.ID, nodes)
	var addrs [][]byte

	for i := 0; i < nodes; i++ {
		conf := adapters.RandomNodeConfig()
		node, err := net.NewNodeWithConfig(conf)
		if err != nil {
			panic(err)
		}
		if err != nil {
			return nil, fmt.Errorf("error starting node: %s", err)
		}
		if err := net.Start(node.ID()); err != nil {
			return nil, fmt.Errorf("error starting node %s: %s", node.ID().TerminalString(), err)
		}
		if err := triggerChecks(trigger, net, node.ID()); err != nil {
			return nil, fmt.Errorf("error triggering checks for node %s: %s", node.ID().TerminalString(), err)
		}
//Todo我们不应该这样把underddr和overddr等同起来，因为它们在生产中是不一样的。
		ids[i] = node.ID()
		a := ids[i].Bytes()

		addrs = append(addrs, a)
	}

//运行一个模拟，将一个环中的10个节点连接起来并等待
//用于完全对等机发现

	var restartTime time.Time

	action := func(ctx context.Context) error {
		ticker := time.NewTicker(500 * time.Millisecond)

		for range ticker.C {
			isHealthy := true
			for _, id := range ids {
//调用健康的RPC
				node := net.GetNode(id)
				if node == nil {
					return fmt.Errorf("unknown node: %s", id)
				}
				client, err := node.Client()
				if err != nil {
					return fmt.Errorf("error getting node client: %s", err)
				}
				healthy := &network.Health{}
				addr := id.String()
				ppmap := network.NewPeerPotMap(network.NewKadParams().NeighbourhoodSize, addrs)
				if err := client.Call(&healthy, "hive_healthy", ppmap[common.Bytes2Hex(id.Bytes())]); err != nil {
					return fmt.Errorf("error getting node health: %s", err)
				}

				log.Info(fmt.Sprintf("NODE: %s, IS HEALTHY: %t", addr, healthy.ConnectNN && healthy.KnowNN && healthy.CountKnowNN > 0))
				var nodeStr string
				if err := client.Call(&nodeStr, "hive_string"); err != nil {
					return fmt.Errorf("error getting node string %s", err)
				}
				log.Info(nodeStr)
				for _, a := range addrs {
					log.Info(common.Bytes2Hex(a))
				}
				if !healthy.ConnectNN || healthy.CountKnowNN == 0 {
					isHealthy = false
					break
				}
			}
			if isHealthy {
				break
			}
		}
		ticker.Stop()

		log.Info("reached healthy kademlia. starting to shutdown nodes.")
		shutdownStarted := time.Now()
//停止所有ID，然后重新启动它们
		for _, id := range ids {
			node := net.GetNode(id)

			if err := net.Stop(node.ID()); err != nil {
				return fmt.Errorf("error stopping node %s: %s", node.ID().TerminalString(), err)
			}
		}
		log.Info(fmt.Sprintf("shutting down nodes took: %s", time.Since(shutdownStarted)))
		persistenceEnabled = true
		discoveryEnabled = false
		restartTime = time.Now()
		for _, id := range ids {
			node := net.GetNode(id)
			if err := net.Start(node.ID()); err != nil {
				return fmt.Errorf("error starting node %s: %s", node.ID().TerminalString(), err)
			}
			if err := triggerChecks(trigger, net, node.ID()); err != nil {
				return fmt.Errorf("error triggering checks for node %s: %s", node.ID().TerminalString(), err)
			}
		}

		log.Info(fmt.Sprintf("restarting nodes took: %s", time.Since(restartTime)))

		return nil
	}
	net.ConnectNodesChain(nil)
	log.Debug(fmt.Sprintf("nodes: %v", len(addrs)))
//建造对等壶，以便检查卡德米利亚的健康状况。
	check := func(ctx context.Context, id enode.ID) (bool, error) {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		node := net.GetNode(id)
		if node == nil {
			return false, fmt.Errorf("unknown node: %s", id)
		}
		client, err := node.Client()
		if err != nil {
			return false, fmt.Errorf("error getting node client: %s", err)
		}
		healthy := &network.Health{}
		ppmap := network.NewPeerPotMap(network.NewKadParams().NeighbourhoodSize, addrs)

		if err := client.Call(&healthy, "hive_healthy", ppmap[common.Bytes2Hex(id.Bytes())]); err != nil {
			return false, fmt.Errorf("error getting node health: %s", err)
		}
		log.Info(fmt.Sprintf("node %4s healthy: got nearest neighbours: %v, know nearest neighbours: %v", id, healthy.ConnectNN, healthy.KnowNN))

		return healthy.KnowNN && healthy.ConnectNN, nil
	}

//64节点~1min
//128节点~
	timeout := 300 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	result := simulations.NewSimulation(net).Run(ctx, &simulations.Step{
		Action:  action,
		Trigger: trigger,
		Expect: &simulations.Expectation{
			Nodes: ids,
			Check: check,
		},
	})
	if result.Error != nil {
		return result, nil
	}

	return result, nil
}

//每当添加对等机或
//从给定的节点中删除，并且每隔一秒删除一次，以避免
//同伴活动和卡德米利亚变得健康
func triggerChecks(trigger chan enode.ID, net *simulations.Network, id enode.ID) error {
	node := net.GetNode(id)
	if node == nil {
		return fmt.Errorf("unknown node: %s", id)
	}
	client, err := node.Client()
	if err != nil {
		return err
	}
	events := make(chan *p2p.PeerEvent)
	sub, err := client.Subscribe(context.Background(), "admin", events, "peerEvents")
	if err != nil {
		return fmt.Errorf("error getting peer events for node %v: %s", id, err)
	}
	go func() {
		defer sub.Unsubscribe()

		tick := time.NewTicker(time.Second)
		defer tick.Stop()

		for {
			select {
			case <-events:
				trigger <- id
			case <-tick.C:
				trigger <- id
			case err := <-sub.Err():
				if err != nil {
					log.Error(fmt.Sprintf("error getting peer events for node %v", id), "err", err)
				}
				return
			}
		}
	}()
	return nil
}

func newService(ctx *adapters.ServiceContext) (node.Service, error) {
	addr := network.NewAddr(ctx.Config.Node())

	kp := network.NewKadParams()
	kp.NeighbourhoodSize = testNeighbourhoodSize

	if ctx.Config.Reachable != nil {
		kp.Reachable = func(o *network.BzzAddr) bool {
			return ctx.Config.Reachable(o.ID())
		}
	}
	kad := network.NewKademlia(addr.Over(), kp)
	hp := network.NewHiveParams()
	hp.KeepAliveInterval = time.Duration(200) * time.Millisecond
	hp.Discovery = discoveryEnabled

	log.Info(fmt.Sprintf("discovery for nodeID %s is %t", ctx.Config.ID.String(), hp.Discovery))

	config := &network.BzzConfig{
		OverlayAddr:  addr.Over(),
		UnderlayAddr: addr.Under(),
		HiveParams:   hp,
	}

	if persistenceEnabled {
		log.Info(fmt.Sprintf("persistence enabled for nodeID %s", ctx.Config.ID.String()))
		store, err := getDbStore(ctx.Config.ID.String())
		if err != nil {
			return nil, err
		}
		return network.NewBzz(config, kad, store, nil, nil), nil
	}

	return network.NewBzz(config, kad, nil, nil, nil), nil
}
