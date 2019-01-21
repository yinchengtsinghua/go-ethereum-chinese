
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

package stream

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/simulation"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/mock"
	mockmem "github.com/ethereum/go-ethereum/swarm/storage/mock/mem"
	"github.com/ethereum/go-ethereum/swarm/testutil"
)

const dataChunkCount = 200

func TestSyncerSimulation(t *testing.T) {
	testSyncBetweenNodes(t, 2, dataChunkCount, true, 1)
	testSyncBetweenNodes(t, 4, dataChunkCount, true, 1)
	testSyncBetweenNodes(t, 8, dataChunkCount, true, 1)
	testSyncBetweenNodes(t, 16, dataChunkCount, true, 1)
}

func createMockStore(globalStore mock.GlobalStorer, id enode.ID, addr *network.BzzAddr) (lstore storage.ChunkStore, datadir string, err error) {
	address := common.BytesToAddress(id.Bytes())
	mockStore := globalStore.NewNodeStore(address)
	params := storage.NewDefaultLocalStoreParams()

	datadir, err = ioutil.TempDir("", "localMockStore-"+id.TerminalString())
	if err != nil {
		return nil, "", err
	}
	params.Init(datadir)
	params.BaseKey = addr.Over()
	lstore, err = storage.NewLocalStore(params, mockStore)
	if err != nil {
		return nil, "", err
	}
	return lstore, datadir, nil
}

func testSyncBetweenNodes(t *testing.T, nodes, chunkCount int, skipCheck bool, po uint8) {

	sim := simulation.New(map[string]simulation.ServiceFunc{
		"streamer": func(ctx *adapters.ServiceContext, bucket *sync.Map) (s node.Service, cleanup func(), err error) {
			var store storage.ChunkStore
			var datadir string

			node := ctx.Config.Node()
			addr := network.NewAddr(node)
//黑客把地址放在同一个空间
			addr.OAddr[0] = byte(0)

			if *useMockStore {
				store, datadir, err = createMockStore(mockmem.NewGlobalStore(), node.ID(), addr)
			} else {
				store, datadir, err = createTestLocalStorageForID(node.ID(), addr)
			}
			if err != nil {
				return nil, nil, err
			}
			bucket.Store(bucketKeyStore, store)
			cleanup = func() {
				store.Close()
				os.RemoveAll(datadir)
			}
			localStore := store.(*storage.LocalStore)
			netStore, err := storage.NewNetStore(localStore, nil)
			if err != nil {
				return nil, nil, err
			}
			bucket.Store(bucketKeyDB, netStore)
			kad := network.NewKademlia(addr.Over(), network.NewKadParams())
			delivery := NewDelivery(kad, netStore)
			netStore.NewNetFetcherFunc = network.NewFetcherFactory(delivery.RequestFromPeers, true).New

			bucket.Store(bucketKeyDelivery, delivery)

			r := NewRegistry(addr.ID(), delivery, netStore, state.NewInmemoryStore(), &RegistryOptions{
				Retrieval: RetrievalDisabled,
				Syncing:   SyncingAutoSubscribe,
				SkipCheck: skipCheck,
			}, nil)

			fileStore := storage.NewFileStore(netStore, storage.NewFileStoreParams())
			bucket.Store(bucketKeyFileStore, fileStore)

			return r, cleanup, nil

		},
	})
	defer sim.Close()

//为模拟运行创建上下文
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
//延迟取消应在延迟模拟拆卸之前出现
	defer cancel()

	_, err := sim.AddNodesAndConnectChain(nodes)
	if err != nil {
		t.Fatal(err)
	}
	result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) (err error) {
		nodeIDs := sim.UpNodeIDs()

		nodeIndex := make(map[enode.ID]int)
		for i, id := range nodeIDs {
			nodeIndex[id] = i
		}

		disconnections := sim.PeerEvents(
			context.Background(),
			sim.NodeIDs(),
			simulation.NewPeerEventsFilter().Drop(),
		)

		var disconnected atomic.Value
		go func() {
			for d := range disconnections {
				if d.Error != nil {
					log.Error("peer drop", "node", d.NodeID, "peer", d.PeerID)
					disconnected.Store(true)
				}
			}
		}()
		defer func() {
			if err != nil {
				if yes, ok := disconnected.Load().(bool); ok && yes {
					err = errors.New("disconnect events received")
				}
			}
		}()

//每个节点都订阅彼此的swarmChunkServerStreamName
		for j := 0; j < nodes-1; j++ {
			id := nodeIDs[j]
			client, err := sim.Net.GetNode(id).Client()
			if err != nil {
				t.Fatal(err)
			}
			sid := nodeIDs[j+1]
			client.CallContext(ctx, nil, "stream_subscribeStream", sid, NewStream("SYNC", FormatSyncBinKey(1), false), NewRange(0, 0), Top)
			if err != nil {
				return err
			}
			if j > 0 || nodes == 2 {
				item, ok := sim.NodeItem(nodeIDs[j], bucketKeyFileStore)
				if !ok {
					return fmt.Errorf("No filestore")
				}
				fileStore := item.(*storage.FileStore)
				size := chunkCount * chunkSize
				_, wait, err := fileStore.Store(ctx, testutil.RandomReader(j, size), int64(size), false)
				if err != nil {
					t.Fatal(err.Error())
				}
				wait(ctx)
			}
		}
//在这里，我们将随机文件的块分发到存储1…节点中
		if _, err := sim.WaitTillHealthy(ctx); err != nil {
			return err
		}

//为每个节点收集po 1 bin中的哈希
		hashes := make([][]storage.Address, nodes)
		totalHashes := 0
		hashCounts := make([]int, nodes)
		for i := nodes - 1; i >= 0; i-- {
			if i < nodes-1 {
				hashCounts[i] = hashCounts[i+1]
			}
			item, ok := sim.NodeItem(nodeIDs[i], bucketKeyDB)
			if !ok {
				return fmt.Errorf("No DB")
			}
			netStore := item.(*storage.NetStore)
			netStore.Iterator(0, math.MaxUint64, po, func(addr storage.Address, index uint64) bool {
				hashes[i] = append(hashes[i], addr)
				totalHashes++
				hashCounts[i]++
				return true
			})
		}
		var total, found int
		for _, node := range nodeIDs {
			i := nodeIndex[node]

			for j := i; j < nodes; j++ {
				total += len(hashes[j])
				for _, key := range hashes[j] {
					item, ok := sim.NodeItem(nodeIDs[j], bucketKeyDB)
					if !ok {
						return fmt.Errorf("No DB")
					}
					db := item.(*storage.NetStore)
					_, err := db.Get(ctx, key)
					if err == nil {
						found++
					}
				}
			}
			log.Debug("sync check", "node", node, "index", i, "bin", po, "found", found, "total", total)
		}
		if total == found && total > 0 {
			return nil
		}
		return fmt.Errorf("Total not equallying found: total is %d", total)
	})

	if result.Error != nil {
		t.Fatal(result.Error)
	}
}

//testsameversionid只检查如果版本没有更改，
//然后流媒体对等端看到彼此
func TestSameVersionID(t *testing.T) {
//测试版本ID
	v := uint(1)
	sim := simulation.New(map[string]simulation.ServiceFunc{
		"streamer": func(ctx *adapters.ServiceContext, bucket *sync.Map) (s node.Service, cleanup func(), err error) {
			var store storage.ChunkStore
			var datadir string

			node := ctx.Config.Node()
			addr := network.NewAddr(node)

			store, datadir, err = createTestLocalStorageForID(node.ID(), addr)
			if err != nil {
				return nil, nil, err
			}
			bucket.Store(bucketKeyStore, store)
			cleanup = func() {
				store.Close()
				os.RemoveAll(datadir)
			}
			localStore := store.(*storage.LocalStore)
			netStore, err := storage.NewNetStore(localStore, nil)
			if err != nil {
				return nil, nil, err
			}
			bucket.Store(bucketKeyDB, netStore)
			kad := network.NewKademlia(addr.Over(), network.NewKadParams())
			delivery := NewDelivery(kad, netStore)
			netStore.NewNetFetcherFunc = network.NewFetcherFactory(delivery.RequestFromPeers, true).New

			bucket.Store(bucketKeyDelivery, delivery)

			r := NewRegistry(addr.ID(), delivery, netStore, state.NewInmemoryStore(), &RegistryOptions{
				Retrieval: RetrievalDisabled,
				Syncing:   SyncingAutoSubscribe,
			}, nil)
//为每个节点分配相同的版本ID
			r.spec.Version = v

			bucket.Store(bucketKeyRegistry, r)

			return r, cleanup, nil

		},
	})
	defer sim.Close()

//只连接两个节点
	log.Info("Adding nodes to simulation")
	_, err := sim.AddNodesAndConnectChain(2)
	if err != nil {
		t.Fatal(err)
	}

	log.Info("Starting simulation")
	ctx := context.Background()
//确保他们有时间连接
	time.Sleep(200 * time.Millisecond)
	result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) error {
//获取透视节点的文件存储
		nodes := sim.UpNodeIDs()

		item, ok := sim.NodeItem(nodes[0], bucketKeyRegistry)
		if !ok {
			return fmt.Errorf("No filestore")
		}
		registry := item.(*Registry)

//对等端应连接，因此获取对等端不应返回零
		if registry.getPeer(nodes[1]) == nil {
			t.Fatal("Expected the peer to not be nil, but it is")
		}
		return nil
	})
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	log.Info("Simulation ended")
}

//TestDifferentVersionID证明如果拖缆协议版本不匹配，
//然后，在拖缆级别上没有连接对等端。
func TestDifferentVersionID(t *testing.T) {
//创建保存版本ID的变量
	v := uint(0)
	sim := simulation.New(map[string]simulation.ServiceFunc{
		"streamer": func(ctx *adapters.ServiceContext, bucket *sync.Map) (s node.Service, cleanup func(), err error) {
			var store storage.ChunkStore
			var datadir string

			node := ctx.Config.Node()
			addr := network.NewAddr(node)

			store, datadir, err = createTestLocalStorageForID(node.ID(), addr)
			if err != nil {
				return nil, nil, err
			}
			bucket.Store(bucketKeyStore, store)
			cleanup = func() {
				store.Close()
				os.RemoveAll(datadir)
			}
			localStore := store.(*storage.LocalStore)
			netStore, err := storage.NewNetStore(localStore, nil)
			if err != nil {
				return nil, nil, err
			}
			bucket.Store(bucketKeyDB, netStore)
			kad := network.NewKademlia(addr.Over(), network.NewKadParams())
			delivery := NewDelivery(kad, netStore)
			netStore.NewNetFetcherFunc = network.NewFetcherFactory(delivery.RequestFromPeers, true).New

			bucket.Store(bucketKeyDelivery, delivery)

			r := NewRegistry(addr.ID(), delivery, netStore, state.NewInmemoryStore(), &RegistryOptions{
				Retrieval: RetrievalDisabled,
				Syncing:   SyncingAutoSubscribe,
			}, nil)

//增加每个节点的版本ID
			v++
			r.spec.Version = v

			bucket.Store(bucketKeyRegistry, r)

			return r, cleanup, nil

		},
	})
	defer sim.Close()

//连接节点
	log.Info("Adding nodes to simulation")
	_, err := sim.AddNodesAndConnectChain(2)
	if err != nil {
		t.Fatal(err)
	}

	log.Info("Starting simulation")
	ctx := context.Background()
//确保他们有时间连接
	time.Sleep(200 * time.Millisecond)
	result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) error {
//获取透视节点的文件存储
		nodes := sim.UpNodeIDs()

		item, ok := sim.NodeItem(nodes[0], bucketKeyRegistry)
		if !ok {
			return fmt.Errorf("No filestore")
		}
		registry := item.(*Registry)

//由于版本号不同，获取另一个对等机应该失败
		if registry.getPeer(nodes[1]) != nil {
			t.Fatal("Expected the peer to be nil, but it is not")
		}
		return nil
	})
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	log.Info("Simulation ended")

}
