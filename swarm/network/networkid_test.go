
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
	"bytes"
	"context"
	"flag"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	currentNetworkID int
	cnt              int
	nodeMap          map[int][]enode.ID
	kademlias        map[enode.ID]*Kademlia
)

const (
	NumberOfNets = 4
	MaxTimeout   = 6
)

func init() {
	flag.Parse()
	rand.Seed(time.Now().Unix())
}

/*
运行网络ID测试。
测试创建一个模拟。网络实例，
多个节点，然后在此网络中彼此连接节点。

每个节点都得到一个根据网络数量分配的网络ID。
拥有更多的网络ID只是为了排除
误报。

节点只能与具有相同网络ID的其他节点连接。
在设置阶段之后，测试将检查每个节点是否具有
预期的节点连接（不包括那些不共享网络ID的连接）。
**/

func TestNetworkID(t *testing.T) {
	log.Debug("Start test")
//任意设置节点数。可以是任何号码
	numNodes := 24
//nodemap用相同的网络ID（key）映射所有节点（切片值）
	nodeMap = make(map[int][]enode.ID)
//设置网络并连接节点
	net, err := setupNetwork(numNodes)
	if err != nil {
		t.Fatalf("Error setting up network: %v", err)
	}
//让我们休眠以确保所有节点都已连接
	time.Sleep(1 * time.Second)
//关闭网络以避免竞争条件
//网络节点访问Kademlias全局地图的研究
//正在接受消息
	net.Shutdown()
//对于共享相同网络ID的每个组…
	for _, netIDGroup := range nodeMap {
		log.Trace("netIDGroup size", "size", len(netIDGroup))
//…检查他们的花冠尺寸是否符合预期尺寸
//假设它应该是组的大小减去1（节点本身）。
		for _, node := range netIDGroup {
			if kademlias[node].addrs.Size() != len(netIDGroup)-1 {
				t.Fatalf("Kademlia size has not expected peer size. Kademlia size: %d, expected size: %d", kademlias[node].addrs.Size(), len(netIDGroup)-1)
			}
			kademlias[node].EachAddr(nil, 0, func(addr *BzzAddr, _ int) bool {
				found := false
				for _, nd := range netIDGroup {
					if bytes.Equal(kademlias[nd].BaseAddr(), addr.Address()) {
						found = true
					}
				}
				if !found {
					t.Fatalf("Expected node not found for node %s", node.String())
				}
				return true
			})
		}
	}
	log.Info("Test terminated successfully")
}

//使用bzz/discovery和pss服务设置模拟网络。
//连接圆中的节点
//如果设置了allowraw，则启用了省略内置PSS加密（请参阅PSSPARAMS）
func setupNetwork(numnodes int) (net *simulations.Network, err error) {
	log.Debug("Setting up network")
	quitC := make(chan struct{})
	errc := make(chan error)
	nodes := make([]*simulations.Node, numnodes)
	if numnodes < 16 {
		return nil, fmt.Errorf("Minimum sixteen nodes in network")
	}
	adapter := adapters.NewSimAdapter(newServices())
//创建网络
	net = simulations.NewNetwork(adapter, &simulations.NetworkConfig{
		ID:             "NetworkIdTestNet",
		DefaultService: "bzz",
	})
	log.Debug("Creating networks and nodes")

	var connCount int

//创建节点并相互连接
	for i := 0; i < numnodes; i++ {
		log.Trace("iteration: ", "i", i)
		nodeconf := adapters.RandomNodeConfig()
		nodes[i], err = net.NewNodeWithConfig(nodeconf)
		if err != nil {
			return nil, fmt.Errorf("error creating node %d: %v", i, err)
		}
		err = net.Start(nodes[i].ID())
		if err != nil {
			return nil, fmt.Errorf("error starting node %d: %v", i, err)
		}
		client, err := nodes[i].Client()
		if err != nil {
			return nil, fmt.Errorf("create node %d rpc client fail: %v", i, err)
		}
//现在设置并开始事件监视，以了解何时可以上载
		ctx, watchCancel := context.WithTimeout(context.Background(), MaxTimeout*time.Second)
		defer watchCancel()
		watchSubscriptionEvents(ctx, nodes[i].ID(), client, errc, quitC)
//在每次迭代中，我们都连接到以前的所有迭代
		for k := i - 1; k >= 0; k-- {
			connCount++
			log.Debug(fmt.Sprintf("Connecting node %d with node %d; connection count is %d", i, k, connCount))
			err = net.Connect(nodes[i].ID(), nodes[k].ID())
			if err != nil {
				if !strings.Contains(err.Error(), "already connected") {
					return nil, fmt.Errorf("error connecting nodes: %v", err)
				}
			}
		}
	}
//现在等待，直到完成预期订阅的数量
//`watchsubscriptionEvents`将用'nil'值写入errc
	for err := range errc {
		if err != nil {
			return nil, err
		}
//收到“nil”，递减计数
		connCount--
		log.Trace("count down", "cnt", connCount)
//收到的所有订阅
		if connCount == 0 {
			close(quitC)
			break
		}
	}
	log.Debug("Network setup phase terminated")
	return net, nil
}

func newServices() adapters.Services {
	kademlias = make(map[enode.ID]*Kademlia)
	kademlia := func(id enode.ID) *Kademlia {
		if k, ok := kademlias[id]; ok {
			return k
		}
		params := NewKadParams()
		params.NeighbourhoodSize = 2
		params.MaxBinSize = 3
		params.MinBinSize = 1
		params.MaxRetries = 1000
		params.RetryExponent = 2
		params.RetryInterval = 1000000
		kademlias[id] = NewKademlia(id[:], params)
		return kademlias[id]
	}
	return adapters.Services{
		"bzz": func(ctx *adapters.ServiceContext) (node.Service, error) {
			addr := NewAddr(ctx.Config.Node())
			hp := NewHiveParams()
			hp.Discovery = false
			cnt++
//分配网络ID
			currentNetworkID = cnt % NumberOfNets
			if ok := nodeMap[currentNetworkID]; ok == nil {
				nodeMap[currentNetworkID] = make([]enode.ID, 0)
			}
//将此节点添加到共享相同网络ID的组中
			nodeMap[currentNetworkID] = append(nodeMap[currentNetworkID], ctx.Config.ID)
			log.Debug("current network ID:", "id", currentNetworkID)
			config := &BzzConfig{
				OverlayAddr:  addr.Over(),
				UnderlayAddr: addr.Under(),
				HiveParams:   hp,
				NetworkID:    uint64(currentNetworkID),
			}
			return NewBzz(config, kademlia(ctx.Config.ID), nil, nil, nil), nil
		},
	}
}

func watchSubscriptionEvents(ctx context.Context, id enode.ID, client *rpc.Client, errc chan error, quitC chan struct{}) {
	events := make(chan *p2p.PeerEvent)
	sub, err := client.Subscribe(context.Background(), "admin", events, "peerEvents")
	if err != nil {
		log.Error(err.Error())
		errc <- fmt.Errorf("error getting peer events for node %v: %s", id, err)
		return
	}
	go func() {
		defer func() {
			sub.Unsubscribe()
			log.Trace("watch subscription events: unsubscribe", "id", id)
		}()

		for {
			select {
			case <-quitC:
				return
			case <-ctx.Done():
				select {
				case errc <- ctx.Err():
				case <-quitC:
				}
				return
			case e := <-events:
				if e.Type == p2p.PeerEventTypeAdd {
					errc <- nil
				}
			case err := <-sub.Err():
				if err != nil {
					select {
					case errc <- fmt.Errorf("error getting peer events for node %v: %v", id, err):
					case <-quitC:
					}
					return
				}
			}
		}
	}()
}
