
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
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/simulation"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

//用于随机文件生成的常量
const (
	minFileSize = 2
	maxFileSize = 40
)

//此测试是节点的检索测试。
//可以配置多个节点
//提供给测试。
//文件上载到节点，其他节点尝试检索文件
//节点数量也可以通过命令行提供。
func TestFileRetrieval(t *testing.T) {
	if *nodes != 0 {
		err := runFileRetrievalTest(*nodes)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		nodeCnt := []int{16}
//如果已提供“longrunning”标志
//运行更多测试组合
		if *longrunning {
			nodeCnt = append(nodeCnt, 32, 64, 128)
		}
		for _, n := range nodeCnt {
			err := runFileRetrievalTest(n)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

//此测试是节点的检索测试。
//随机选择一个节点作为轴节点。
//块和节点的可配置数量可以是
//提供给测试时，将上载块的数量
//到透视节点和其他节点，尝试检索块。
//块和节点的数量也可以通过命令行提供。
func TestRetrieval(t *testing.T) {
//如果节点/块是通过命令行提供的，
//使用这些值运行测试
	if *nodes != 0 && *chunks != 0 {
		err := runRetrievalTest(*chunks, *nodes)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		var nodeCnt []int
		var chnkCnt []int
//如果已提供“longrunning”标志
//运行更多测试组合
		if *longrunning {
			nodeCnt = []int{16, 32, 128}
			chnkCnt = []int{4, 32, 256}
		} else {
//缺省测试
			nodeCnt = []int{16}
			chnkCnt = []int{32}
		}
		for _, n := range nodeCnt {
			for _, c := range chnkCnt {
				err := runRetrievalTest(c, n)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

var retrievalSimServiceMap = map[string]simulation.ServiceFunc{
	"streamer": retrievalStreamerFunc,
}

func retrievalStreamerFunc(ctx *adapters.ServiceContext, bucket *sync.Map) (s node.Service, cleanup func(), err error) {
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
	netStore.NewNetFetcherFunc = network.NewFetcherFactory(delivery.RequestFromPeers, true).New

	r := NewRegistry(addr.ID(), delivery, netStore, state.NewInmemoryStore(), &RegistryOptions{
		Retrieval:       RetrievalEnabled,
		Syncing:         SyncingAutoSubscribe,
		SyncUpdateDelay: 3 * time.Second,
	}, nil)

	fileStore := storage.NewFileStore(netStore, storage.NewFileStoreParams())
	bucket.Store(bucketKeyFileStore, fileStore)

	cleanup = func() {
		os.RemoveAll(datadir)
		netStore.Close()
		r.Close()
	}

	return r, cleanup, nil
}

/*
测试加载快照文件构建群网络，
假设快照文件标识一个健康的
卡德米利亚网络。然而，健康检查运行在
模拟的“action”函数。

快照的服务列表中应包含“streamer”。
**/

func runFileRetrievalTest(nodeCount int) error {
	sim := simulation.New(retrievalSimServiceMap)
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
		return err
	}

	ctx, cancelSimRun := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelSimRun()

	result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) error {
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

//随机文件的数组
		var randomFiles []string
//上传完成后的信号通道
//上传完成：=make（chan struct）
//触发新节点检查的通道

		conf.hashes, randomFiles, err = uploadFilesToNodes(sim)
		if err != nil {
			return err
		}
		if _, err := sim.WaitTillHealthy(ctx); err != nil {
			return err
		}

//重复文件检索检查，直到从所有节点检索所有上载的文件
//或者直到超时。
	REPEAT:
		for {
			for _, id := range nodeIDs {
//对于每个期望的文件，检查它是否在本地存储区中
				item, ok := sim.NodeItem(id, bucketKeyFileStore)
				if !ok {
					return fmt.Errorf("No filestore")
				}
				fileStore := item.(*storage.FileStore)
//检查所有块
				for i, hash := range conf.hashes {
					reader, _ := fileStore.Retrieve(context.TODO(), hash)
//检查我们是否可以读取文件大小，以及它是否与生成的文件大小相对应。
					if s, err := reader.Size(ctx, nil); err != nil || s != int64(len(randomFiles[i])) {
						log.Debug("Retrieve error", "err", err, "hash", hash, "nodeId", id)
						time.Sleep(500 * time.Millisecond)
						continue REPEAT
					}
					log.Debug(fmt.Sprintf("File with root hash %x successfully retrieved", hash))
				}
			}
			return nil
		}
	})

	if result.Error != nil {
		return result.Error
	}

	return nil
}

/*
测试生成给定数量的块。

测试加载快照文件构建群网络，
假设快照文件标识一个健康的
卡德米利亚网络。然而，健康检查运行在
模拟的“action”函数。

快照的服务列表中应包含“streamer”。
**/

func runRetrievalTest(chunkCount int, nodeCount int) error {
	sim := simulation.New(retrievalSimServiceMap)
	defer sim.Close()

	conf := &synctestConfig{}
//发现ID到该ID处预期的块索引的映射
	conf.idToChunksMap = make(map[enode.ID][]int)
//发现ID的覆盖地址映射
	conf.addrToIDMap = make(map[string]enode.ID)
//存储生成的块哈希的数组
	conf.hashes = make([]storage.Address, 0)

	err := sim.UploadSnapshot(fmt.Sprintf("testing/snapshot_%d.json", nodeCount))
	if err != nil {
		return err
	}

	ctx := context.Background()
	result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) error {
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

//这是选择上载的节点
		node := sim.Net.GetRandomUpNode()
		item, ok := sim.NodeItem(node.ID(), bucketKeyStore)
		if !ok {
			return fmt.Errorf("No localstore")
		}
		lstore := item.(*storage.LocalStore)
		conf.hashes, err = uploadFileToSingleNodeStore(node.ID(), chunkCount, lstore)
		if err != nil {
			return err
		}
		if _, err := sim.WaitTillHealthy(ctx); err != nil {
			return err
		}

//重复文件检索检查，直到从所有节点检索所有上载的文件
//或者直到超时。
	REPEAT:
		for {
			for _, id := range nodeIDs {
//对于每个预期的块，检查它是否在本地存储区中
//检查节点的文件存储（netstore）
				item, ok := sim.NodeItem(id, bucketKeyFileStore)
				if !ok {
					return fmt.Errorf("No filestore")
				}
				fileStore := item.(*storage.FileStore)
//检查所有块
				for _, hash := range conf.hashes {
					reader, _ := fileStore.Retrieve(context.TODO(), hash)
//检查我们是否可以读取块大小，以及它是否与生成的块大小相对应。
					if s, err := reader.Size(ctx, nil); err != nil || s != int64(chunkSize) {
						log.Debug("Retrieve error", "err", err, "hash", hash, "nodeId", id, "size", s)
						time.Sleep(500 * time.Millisecond)
						continue REPEAT
					}
					log.Debug(fmt.Sprintf("Chunk with root hash %x successfully retrieved", hash))
				}
			}
//找到所有节点和文件，退出循环并返回而不出错
			return nil
		}
	})

	if result.Error != nil {
		return result.Error
	}

	return nil
}
