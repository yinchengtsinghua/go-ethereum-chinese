
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
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/simulation"
	"github.com/ethereum/go-ethereum/swarm/pot"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/mock"
	mockmem "github.com/ethereum/go-ethereum/swarm/storage/mock/mem"
	"github.com/ethereum/go-ethereum/swarm/testutil"
)

const MaxTimeout = 600

type synctestConfig struct {
	addrs         [][]byte
	hashes        []storage.Address
	idToChunksMap map[enode.ID][]int
//chunkstonodesmap map[string][]int
	addrToIDMap map[string]enode.ID
}

const (
//EventTypeNode是当节点为
//创建、启动或停止
	EventTypeChunkCreated   simulations.EventType = "chunkCreated"
	EventTypeChunkOffered   simulations.EventType = "chunkOffered"
	EventTypeChunkWanted    simulations.EventType = "chunkWanted"
	EventTypeChunkDelivered simulations.EventType = "chunkDelivered"
	EventTypeChunkArrived   simulations.EventType = "chunkArrived"
	EventTypeSimTerminated  simulations.EventType = "simTerminated"
)

//此文件中的测试不应向对等方请求块。
//此函数将死机，表示如果发出请求，则存在问题。
func dummyRequestFromPeers(_ context.Context, req *network.Request) (*enode.ID, chan struct{}, error) {
	panic(fmt.Sprintf("unexpected request: address %s, source %s", req.Addr.String(), req.Source.String()))
}

//此测试是节点的同步测试。
//随机选择一个节点作为轴节点。
//块和节点的可配置数量可以是
//提供给测试时，将上载块的数量
//到透视节点，我们检查节点是否获取块
//它们将根据同步协议进行存储。
//块和节点的数量也可以通过命令行提供。
func TestSyncingViaGlobalSync(t *testing.T) {
	if runtime.GOOS == "darwin" && os.Getenv("TRAVIS") == "true" {
		t.Skip("Flaky on mac on travis")
	}
//如果节点/块是通过命令行提供的，
//使用这些值运行测试
	if *nodes != 0 && *chunks != 0 {
		log.Info(fmt.Sprintf("Running test with %d chunks and %d nodes...", *chunks, *nodes))
		testSyncingViaGlobalSync(t, *chunks, *nodes)
	} else {
		var nodeCnt []int
		var chnkCnt []int
//如果已提供“longrunning”标志
//运行更多测试组合
		if *longrunning {
			chnkCnt = []int{1, 8, 32, 256, 1024}
			nodeCnt = []int{16, 32, 64, 128, 256}
		} else {
//缺省测试
			chnkCnt = []int{4, 32}
			nodeCnt = []int{32, 16}
		}
		for _, chnk := range chnkCnt {
			for _, n := range nodeCnt {
				log.Info(fmt.Sprintf("Long running test with %d chunks and %d nodes...", chnk, n))
				testSyncingViaGlobalSync(t, chnk, n)
			}
		}
	}
}

var simServiceMap = map[string]simulation.ServiceFunc{
	"streamer": streamerFunc,
}

func streamerFunc(ctx *adapters.ServiceContext, bucket *sync.Map) (s node.Service, cleanup func(), err error) {
	n := ctx.Config.Node()
	addr := network.NewAddr(n)
	store, datadir, err := createTestLocalStorageForID(n.ID(), addr)
	if err != nil {
		return nil, nil, err
	}
	bucket.Store(bucketKeyStore, store)
	localStore := store.(*storage.LocalStore)
	netStore, err := storage.NewNetStore(localStore, nil)
	if err != nil {
		return nil, nil, err
	}
	kad := network.NewKademlia(addr.Over(), network.NewKadParams())
	delivery := NewDelivery(kad, netStore)
	netStore.NewNetFetcherFunc = network.NewFetcherFactory(dummyRequestFromPeers, true).New

	r := NewRegistry(addr.ID(), delivery, netStore, state.NewInmemoryStore(), &RegistryOptions{
		Retrieval:       RetrievalDisabled,
		Syncing:         SyncingAutoSubscribe,
		SyncUpdateDelay: 3 * time.Second,
	}, nil)

	bucket.Store(bucketKeyRegistry, r)

	cleanup = func() {
		os.RemoveAll(datadir)
		netStore.Close()
		r.Close()
	}

	return r, cleanup, nil

}

func testSyncingViaGlobalSync(t *testing.T, chunkCount int, nodeCount int) {
	sim := simulation.New(simServiceMap)
	defer sim.Close()

	log.Info("Initializing test config")

	conf := &synctestConfig{}
//发现ID到该ID处预期的块索引的映射
	conf.idToChunksMap = make(map[enode.ID][]int)
//发现ID的覆盖地址映射
	conf.addrToIDMap = make(map[string]enode.ID)
//存储生成的块哈希的数组
	conf.hashes = make([]storage.Address, 0)

	err := sim.UploadSnapshot(fmt.Sprintf("testing/snapshot_%d.json", nodeCount))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancelSimRun := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelSimRun()

	if _, err := sim.WaitTillHealthy(ctx); err != nil {
		t.Fatal(err)
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

	result := runSim(conf, ctx, sim, chunkCount)

	if result.Error != nil {
		t.Fatal(result.Error)
	}
	if yes, ok := disconnected.Load().(bool); ok && yes {
		t.Fatal("disconnect events received")
	}
	log.Info("Simulation ended")
}

func runSim(conf *synctestConfig, ctx context.Context, sim *simulation.Simulation, chunkCount int) simulation.Result {

	return sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) error {
		nodeIDs := sim.UpNodeIDs()
		for _, n := range nodeIDs {
//从此ID获取Kademlia覆盖地址
			a := n.Bytes()
//将它附加到所有覆盖地址的数组中
			conf.addrs = append(conf.addrs, a)
//邻近度计算在叠加地址上，
//p2p/simulations检查enode.id上的func触发器，
//所以我们需要知道哪个overlay addr映射到哪个nodeid
			conf.addrToIDMap[string(a)] = n
		}

//获取该索引处的节点
//这是选择上载的节点
		node := sim.Net.GetRandomUpNode()
		item, ok := sim.NodeItem(node.ID(), bucketKeyStore)
		if !ok {
			return fmt.Errorf("No localstore")
		}
		lstore := item.(*storage.LocalStore)
		hashes, err := uploadFileToSingleNodeStore(node.ID(), chunkCount, lstore)
		if err != nil {
			return err
		}
		for _, h := range hashes {
			evt := &simulations.Event{
				Type: EventTypeChunkCreated,
				Node: sim.Net.GetNode(node.ID()),
				Data: h.String(),
			}
			sim.Net.Events().Send(evt)
		}
		conf.hashes = append(conf.hashes, hashes...)
		mapKeysToNodes(conf)

//重复文件检索检查，直到从所有节点检索所有上载的文件
//或者直到超时。
		var globalStore mock.GlobalStorer
		if *useMockStore {
			globalStore = mockmem.NewGlobalStore()
		}
	REPEAT:
		for {
			for _, id := range nodeIDs {
//对于每个预期的块，检查它是否在本地存储区中
				localChunks := conf.idToChunksMap[id]
				for _, ch := range localChunks {
//通过索引数组中的索引获取实际块
					chunk := conf.hashes[ch]
					log.Trace(fmt.Sprintf("node has chunk: %s:", chunk))
//检查本地存储区中是否确实存在预期的块。
					var err error
					if *useMockStore {
//如果应使用MockStore，请使用GlobalStore；在这种情况下，
//完整的本地存储堆栈将被绕过以获取块。
						_, err = globalStore.Get(common.BytesToAddress(id.Bytes()), chunk)
					} else {
//使用实际的本地存储
						item, ok := sim.NodeItem(id, bucketKeyStore)
						if !ok {
							return fmt.Errorf("Error accessing localstore")
						}
						lstore := item.(*storage.LocalStore)
						_, err = lstore.Get(ctx, chunk)
					}
					if err != nil {
						log.Debug(fmt.Sprintf("Chunk %s NOT found for id %s", chunk, id))
//不要因为记录警告信息而发疯
						time.Sleep(500 * time.Millisecond)
						continue REPEAT
					}
					evt := &simulations.Event{
						Type: EventTypeChunkArrived,
						Node: sim.Net.GetNode(id),
						Data: chunk.String(),
					}
					sim.Net.Events().Send(evt)
					log.Debug(fmt.Sprintf("Chunk %s IS FOUND for id %s", chunk, id))
				}
			}
			return nil
		}
	})
}

//将区块键映射到负责的地址
func mapKeysToNodes(conf *synctestConfig) {
	nodemap := make(map[string][]int)
//为大块散列构建一个容器
	np := pot.NewPot(nil, 0)
	indexmap := make(map[string]int)
	for i, a := range conf.addrs {
		indexmap[string(a)] = i
		np, _, _ = pot.Add(np, a, pof)
	}

	ppmap := network.NewPeerPotMap(network.NewKadParams().NeighbourhoodSize, conf.addrs)

//对于每个地址，在chunk hashes pot上运行eachneighbour以标识最近的节点
	log.Trace(fmt.Sprintf("Generated hash chunk(s): %v", conf.hashes))
	for i := 0; i < len(conf.hashes); i++ {
		var a []byte
		np.EachNeighbour([]byte(conf.hashes[i]), pof, func(val pot.Val, po int) bool {
//取第一个地址
			a = val.([]byte)
			return false
		})

		nns := ppmap[common.Bytes2Hex(a)].NNSet
		nns = append(nns, a)

		for _, p := range nns {
			nodemap[string(p)] = append(nodemap[string(p)], i)
		}
	}
	for addr, chunks := range nodemap {
//这将选择希望在给定节点中找到的块
		conf.idToChunksMap[conf.addrToIDMap[addr]] = chunks
	}
	log.Debug(fmt.Sprintf("Map of expected chunks by ID: %v", conf.idToChunksMap))
}

//将文件（块）上载到单个本地节点存储区
func uploadFileToSingleNodeStore(id enode.ID, chunkCount int, lstore *storage.LocalStore) ([]storage.Address, error) {
	log.Debug(fmt.Sprintf("Uploading to node id: %s", id))
	fileStore := storage.NewFileStore(lstore, storage.NewFileStoreParams())
	size := chunkSize
	var rootAddrs []storage.Address
	for i := 0; i < chunkCount; i++ {
		rk, wait, err := fileStore.Store(context.TODO(), testutil.RandomReader(i, size), int64(size), false)
		if err != nil {
			return nil, err
		}
		err = wait(context.TODO())
		if err != nil {
			return nil, err
		}
		rootAddrs = append(rootAddrs, (rk))
	}

	return rootAddrs, nil
}
