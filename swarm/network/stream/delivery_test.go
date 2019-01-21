
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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	p2ptest "github.com/ethereum/go-ethereum/p2p/testing"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	pq "github.com/ethereum/go-ethereum/swarm/network/priorityqueue"
	"github.com/ethereum/go-ethereum/swarm/network/simulation"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/testutil"
)

//初始化检索请求的测试
func TestStreamerRetrieveRequest(t *testing.T) {
	regOpts := &RegistryOptions{
		Retrieval: RetrievalClientOnly,
		Syncing:   SyncingDisabled,
	}
	tester, streamer, _, teardown, err := newStreamerTester(t, regOpts)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	node := tester.Nodes[0]

	ctx := context.Background()
	req := network.NewRequest(
		storage.Address(hash0[:]),
		true,
		&sync.Map{},
	)
	streamer.delivery.RequestFromPeers(ctx, req)

	stream := NewStream(swarmChunkServerStreamName, "", true)

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "RetrieveRequestMsg",
		Expects: []p2ptest.Expect{
{ //由于'retrievalclientonly'，开始要求为retrieve请求订阅
				Code: 4,
				Msg: &SubscribeMsg{
					Stream:   stream,
					History:  nil,
					Priority: Top,
				},
				Peer: node.ID(),
			},
{ //期望给定哈希的检索请求消息
				Code: 5,
				Msg: &RetrieveRequestMsg{
					Addr:      hash0[:],
					SkipCheck: true,
				},
				Peer: node.ID(),
			},
		},
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

//测试从对等端请求一个块，然后发出一个“空的”offeredhashemsg（还没有可用的哈希）
//应超时，因为对等端没有块（以前未发生同步）
func TestStreamerUpstreamRetrieveRequestMsgExchangeWithoutStore(t *testing.T) {
	tester, streamer, _, teardown, err := newStreamerTester(t, &RegistryOptions{
		Retrieval: RetrievalEnabled,
Syncing:   SyncingDisabled, //不同步
	})
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	node := tester.Nodes[0]

	chunk := storage.NewChunk(storage.Address(hash0[:]), nil)

	peer := streamer.getPeer(node.ID())

	stream := NewStream(swarmChunkServerStreamName, "", true)
//模拟预订阅以检索对等机上的请求流
	peer.handleSubscribeMsg(context.TODO(), &SubscribeMsg{
		Stream:   stream,
		History:  nil,
		Priority: Top,
	})

//测试交换
	err = tester.TestExchanges(p2ptest.Exchange{
		Expects: []p2ptest.Expect{
{ //首先需要对检索请求流的订阅
				Code: 4,
				Msg: &SubscribeMsg{
					Stream:   stream,
					History:  nil,
					Priority: Top,
				},
				Peer: node.ID(),
			},
		},
	}, p2ptest.Exchange{
		Label: "RetrieveRequestMsg",
		Triggers: []p2ptest.Trigger{
{ //然后实际的检索请求….
				Code: 5,
				Msg: &RetrieveRequestMsg{
					Addr: chunk.Address()[:],
				},
				Peer: node.ID(),
			},
		},
		Expects: []p2ptest.Expect{
{ //对等端用提供的哈希响应
				Code: 1,
				Msg: &OfferedHashesMsg{
					HandoverProof: nil,
					Hashes:        nil,
					From:          0,
					To:            0,
				},
				Peer: node.ID(),
			},
		},
	})

//作为我们请求的对等机，应该失败并超时
//来自的块没有块
	expectedError := `exchange #1 "RetrieveRequestMsg": timed out`
	if err == nil || err.Error() != expectedError {
		t.Fatalf("Expected error %v, got %v", expectedError, err)
	}
}

//上游请求服务器接收检索请求并用
//如果skipphash设置为true，则提供哈希或传递
func TestStreamerUpstreamRetrieveRequestMsgExchange(t *testing.T) {
	tester, streamer, localStore, teardown, err := newStreamerTester(t, &RegistryOptions{
		Retrieval: RetrievalEnabled,
		Syncing:   SyncingDisabled,
	})
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	node := tester.Nodes[0]

	peer := streamer.getPeer(node.ID())

	stream := NewStream(swarmChunkServerStreamName, "", true)

	peer.handleSubscribeMsg(context.TODO(), &SubscribeMsg{
		Stream:   stream,
		History:  nil,
		Priority: Top,
	})

	hash := storage.Address(hash0[:])
	chunk := storage.NewChunk(hash, hash)
	err = localStore.Put(context.TODO(), chunk)
	if err != nil {
		t.Fatalf("Expected no err got %v", err)
	}

	err = tester.TestExchanges(p2ptest.Exchange{
		Expects: []p2ptest.Expect{
			{
				Code: 4,
				Msg: &SubscribeMsg{
					Stream:   stream,
					History:  nil,
					Priority: Top,
				},
				Peer: node.ID(),
			},
		},
	}, p2ptest.Exchange{
		Label: "RetrieveRequestMsg",
		Triggers: []p2ptest.Trigger{
			{
				Code: 5,
				Msg: &RetrieveRequestMsg{
					Addr: hash,
				},
				Peer: node.ID(),
			},
		},
		Expects: []p2ptest.Expect{
			{
				Code: 1,
				Msg: &OfferedHashesMsg{
					HandoverProof: &HandoverProof{
						Handover: &Handover{},
					},
					Hashes: hash,
					From:   0,
//托多：这是为什么32？？？？
					To:     32,
					Stream: stream,
				},
				Peer: node.ID(),
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	hash = storage.Address(hash1[:])
	chunk = storage.NewChunk(hash, hash1[:])
	err = localStore.Put(context.TODO(), chunk)
	if err != nil {
		t.Fatalf("Expected no err got %v", err)
	}

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "RetrieveRequestMsg",
		Triggers: []p2ptest.Trigger{
			{
				Code: 5,
				Msg: &RetrieveRequestMsg{
					Addr:      hash,
					SkipCheck: true,
				},
				Peer: node.ID(),
			},
		},
		Expects: []p2ptest.Expect{
			{
				Code: 6,
				Msg: &ChunkDeliveryMsg{
					Addr:  hash,
					SData: hash,
				},
				Peer: node.ID(),
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}
}

//如果Kademlia中有一个对等点，则对等点的请求应返回它。
func TestRequestFromPeers(t *testing.T) {
	dummyPeerID := enode.HexID("3431c3939e1ee2a6345e976a8234f9870152d64879f30bc272a074f6859e75e8")

	addr := network.RandomAddr()
	to := network.NewKademlia(addr.OAddr, network.NewKadParams())
	delivery := NewDelivery(to, nil)
	protocolsPeer := protocols.NewPeer(p2p.NewPeer(dummyPeerID, "dummy", nil), nil, nil)
	peer := network.NewPeer(&network.BzzPeer{
		BzzAddr:   network.RandomAddr(),
		LightNode: false,
		Peer:      protocolsPeer,
	}, to)
	to.On(peer)
	r := NewRegistry(addr.ID(), delivery, nil, nil, nil, nil)

//必须创建空的PriorityQueue，以防止在测试完成后调用Goroutine。
	sp := &Peer{
		Peer:     protocolsPeer,
		pq:       pq.New(int(PriorityQueue), PriorityQueueCap),
		streamer: r,
	}
	r.setPeer(sp)
	req := network.NewRequest(
		storage.Address(hash0[:]),
		true,
		&sync.Map{},
	)
	ctx := context.Background()
	id, _, err := delivery.RequestFromPeers(ctx, req)

	if err != nil {
		t.Fatal(err)
	}
	if *id != dummyPeerID {
		t.Fatalf("Expected an id, got %v", id)
	}
}

//对等方的请求不应返回轻节点
func TestRequestFromPeersWithLightNode(t *testing.T) {
	dummyPeerID := enode.HexID("3431c3939e1ee2a6345e976a8234f9870152d64879f30bc272a074f6859e75e8")

	addr := network.RandomAddr()
	to := network.NewKademlia(addr.OAddr, network.NewKadParams())
	delivery := NewDelivery(to, nil)

	protocolsPeer := protocols.NewPeer(p2p.NewPeer(dummyPeerID, "dummy", nil), nil, nil)
//设置lightnode
	peer := network.NewPeer(&network.BzzPeer{
		BzzAddr:   network.RandomAddr(),
		LightNode: true,
		Peer:      protocolsPeer,
	}, to)
	to.On(peer)
	r := NewRegistry(addr.ID(), delivery, nil, nil, nil, nil)
//必须创建空的PriorityQueue，以防止在测试完成后调用Goroutine。
	sp := &Peer{
		Peer:     protocolsPeer,
		pq:       pq.New(int(PriorityQueue), PriorityQueueCap),
		streamer: r,
	}
	r.setPeer(sp)

	req := network.NewRequest(
		storage.Address(hash0[:]),
		true,
		&sync.Map{},
	)

	ctx := context.Background()
//提出一个应返回“未找到对等方”的请求
	_, _, err := delivery.RequestFromPeers(ctx, req)

	expectedError := "no peer found"
	if err.Error() != expectedError {
		t.Fatalf("expected '%v', got %v", expectedError, err)
	}
}

func TestStreamerDownstreamChunkDeliveryMsgExchange(t *testing.T) {
	tester, streamer, localStore, teardown, err := newStreamerTester(t, &RegistryOptions{
		Retrieval: RetrievalDisabled,
		Syncing:   SyncingDisabled,
	})
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	streamer.RegisterClientFunc("foo", func(p *Peer, t string, live bool) (Client, error) {
		return &testClient{
			t: t,
		}, nil
	})

	node := tester.Nodes[0]

//订阅自定义流
	stream := NewStream("foo", "", true)
	err = streamer.Subscribe(node.ID(), stream, NewRange(5, 8), Top)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	chunkKey := hash0[:]
	chunkData := hash1[:]

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "Subscribe message",
		Expects: []p2ptest.Expect{
{ //首先需要订阅自定义流…
				Code: 4,
				Msg: &SubscribeMsg{
					Stream:   stream,
					History:  NewRange(5, 8),
					Priority: Top,
				},
				Peer: node.ID(),
			},
		},
	},
		p2ptest.Exchange{
			Label: "ChunkDelivery message",
			Triggers: []p2ptest.Trigger{
{ //…然后从对等端触发给定块的块传递，以便
//本地节点以获取块传递
					Code: 6,
					Msg: &ChunkDeliveryMsg{
						Addr:  chunkKey,
						SData: chunkData,
					},
					Peer: node.ID(),
				},
			},
		})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

//等待区块存储
	storedChunk, err := localStore.Get(ctx, chunkKey)
	for err != nil {
		select {
		case <-ctx.Done():
			t.Fatalf("Chunk is not in localstore after timeout, err: %v", err)
		default:
		}
		storedChunk, err = localStore.Get(ctx, chunkKey)
		time.Sleep(50 * time.Millisecond)
	}

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !bytes.Equal(storedChunk.Data(), chunkData) {
		t.Fatal("Retrieved chunk has different data than original")
	}

}

func TestDeliveryFromNodes(t *testing.T) {
	testDeliveryFromNodes(t, 2, dataChunkCount, true)
	testDeliveryFromNodes(t, 2, dataChunkCount, false)
	testDeliveryFromNodes(t, 4, dataChunkCount, true)
	testDeliveryFromNodes(t, 4, dataChunkCount, false)
	testDeliveryFromNodes(t, 8, dataChunkCount, true)
	testDeliveryFromNodes(t, 8, dataChunkCount, false)
	testDeliveryFromNodes(t, 16, dataChunkCount, true)
	testDeliveryFromNodes(t, 16, dataChunkCount, false)
}

func testDeliveryFromNodes(t *testing.T, nodes, chunkCount int, skipCheck bool) {
	sim := simulation.New(map[string]simulation.ServiceFunc{
		"streamer": func(ctx *adapters.ServiceContext, bucket *sync.Map) (s node.Service, cleanup func(), err error) {
			node := ctx.Config.Node()
			addr := network.NewAddr(node)
			store, datadir, err := createTestLocalStorageForID(node.ID(), addr)
			if err != nil {
				return nil, nil, err
			}
			bucket.Store(bucketKeyStore, store)
			cleanup = func() {
				os.RemoveAll(datadir)
				store.Close()
			}
			localStore := store.(*storage.LocalStore)
			netStore, err := storage.NewNetStore(localStore, nil)
			if err != nil {
				return nil, nil, err
			}

			kad := network.NewKademlia(addr.Over(), network.NewKadParams())
			delivery := NewDelivery(kad, netStore)
			netStore.NewNetFetcherFunc = network.NewFetcherFactory(delivery.RequestFromPeers, true).New

			r := NewRegistry(addr.ID(), delivery, netStore, state.NewInmemoryStore(), &RegistryOptions{
				SkipCheck: skipCheck,
				Syncing:   SyncingDisabled,
				Retrieval: RetrievalEnabled,
			}, nil)
			bucket.Store(bucketKeyRegistry, r)

			fileStore := storage.NewFileStore(netStore, storage.NewFileStoreParams())
			bucket.Store(bucketKeyFileStore, fileStore)

			return r, cleanup, nil

		},
	})
	defer sim.Close()

	log.Info("Adding nodes to simulation")
	_, err := sim.AddNodesAndConnectChain(nodes)
	if err != nil {
		t.Fatal(err)
	}

	log.Info("Starting simulation")
	ctx := context.Background()
	result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) (err error) {
		nodeIDs := sim.UpNodeIDs()
//确定要作为模拟的第一个节点的轴节点
		pivot := nodeIDs[0]

//将随机文件的块分布到节点1到节点的存储中
//我们将通过创建具有底层循环存储的文件存储来实现这一点：
//文件存储将为上载的文件创建哈希，但每个块都将
//通过循环调度分发到不同的节点
		log.Debug("Writing file to round-robin file store")
//为此，我们为chunkstores创建一个数组（长度减去1，即透视节点）。
		stores := make([]storage.ChunkStore, len(nodeIDs)-1)
//然后我们需要从SIM卡获取所有商店…
		lStores := sim.NodesItems(bucketKeyStore)
		i := 0
//…迭代存储桶…
		for id, bucketVal := range lStores {
//…并移除作为轴心节点的节点
			if id == pivot {
				continue
			}
//其他的被添加到数组中…
			stores[i] = bucketVal.(storage.ChunkStore)
			i++
		}
//…然后传递到循环文件存储
		roundRobinFileStore := storage.NewFileStore(newRoundRobinStore(stores...), storage.NewFileStoreParams())
//现在我们可以将一个（随机）文件上传到循环存储
		size := chunkCount * chunkSize
		log.Debug("Storing data to file store")
		fileHash, wait, err := roundRobinFileStore.Store(ctx, testutil.RandomReader(1, size), int64(size), false)
//等待所有块存储
		if err != nil {
			return err
		}
		err = wait(ctx)
		if err != nil {
			return err
		}

		log.Debug("Waiting for kademlia")
//Todo这似乎不是函数的正确用法，因为模拟可能没有kademlias
		if _, err := sim.WaitTillHealthy(ctx); err != nil {
			return err
		}

//获取透视节点的文件存储
		item, ok := sim.NodeItem(pivot, bucketKeyFileStore)
		if !ok {
			return fmt.Errorf("No filestore")
		}
		pivotFileStore := item.(*storage.FileStore)
		log.Debug("Starting retrieval routine")
		retErrC := make(chan error)
		go func() {
//在透视节点上启动检索-这将为丢失的块生成检索请求
//在请求之前，我们必须等待对等连接启动
			n, err := readAll(pivotFileStore, fileHash)
			log.Info(fmt.Sprintf("retrieved %v", fileHash), "read", n, "err", err)
			retErrC <- err
		}()

		log.Debug("Watching for disconnections")
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

//最后检查透视节点是否通过根哈希获取所有块
		log.Debug("Check retrieval")
		success := true
		var total int64
		total, err = readAll(pivotFileStore, fileHash)
		if err != nil {
			return err
		}
		log.Info(fmt.Sprintf("check if %08x is available locally: number of bytes read %v/%v (error: %v)", fileHash, total, size, err))
		if err != nil || total != int64(size) {
			success = false
		}

		if !success {
			return fmt.Errorf("Test failed, chunks not available on all nodes")
		}
		if err := <-retErrC; err != nil {
			t.Fatalf("requesting chunks: %v", err)
		}
		log.Debug("Test terminated successfully")
		return nil
	})
	if result.Error != nil {
		t.Fatal(result.Error)
	}
}

func BenchmarkDeliveryFromNodesWithoutCheck(b *testing.B) {
	for chunks := 32; chunks <= 128; chunks *= 2 {
		for i := 2; i < 32; i *= 2 {
			b.Run(
				fmt.Sprintf("nodes=%v,chunks=%v", i, chunks),
				func(b *testing.B) {
					benchmarkDeliveryFromNodes(b, i, chunks, true)
				},
			)
		}
	}
}

func BenchmarkDeliveryFromNodesWithCheck(b *testing.B) {
	for chunks := 32; chunks <= 128; chunks *= 2 {
		for i := 2; i < 32; i *= 2 {
			b.Run(
				fmt.Sprintf("nodes=%v,chunks=%v", i, chunks),
				func(b *testing.B) {
					benchmarkDeliveryFromNodes(b, i, chunks, false)
				},
			)
		}
	}
}

func benchmarkDeliveryFromNodes(b *testing.B, nodes, chunkCount int, skipCheck bool) {
	sim := simulation.New(map[string]simulation.ServiceFunc{
		"streamer": func(ctx *adapters.ServiceContext, bucket *sync.Map) (s node.Service, cleanup func(), err error) {
			node := ctx.Config.Node()
			addr := network.NewAddr(node)
			store, datadir, err := createTestLocalStorageForID(node.ID(), addr)
			if err != nil {
				return nil, nil, err
			}
			bucket.Store(bucketKeyStore, store)
			cleanup = func() {
				os.RemoveAll(datadir)
				store.Close()
			}
			localStore := store.(*storage.LocalStore)
			netStore, err := storage.NewNetStore(localStore, nil)
			if err != nil {
				return nil, nil, err
			}
			kad := network.NewKademlia(addr.Over(), network.NewKadParams())
			delivery := NewDelivery(kad, netStore)
			netStore.NewNetFetcherFunc = network.NewFetcherFactory(delivery.RequestFromPeers, true).New

			r := NewRegistry(addr.ID(), delivery, netStore, state.NewInmemoryStore(), &RegistryOptions{
				SkipCheck:       skipCheck,
				Syncing:         SyncingDisabled,
				Retrieval:       RetrievalDisabled,
				SyncUpdateDelay: 0,
			}, nil)

			fileStore := storage.NewFileStore(netStore, storage.NewFileStoreParams())
			bucket.Store(bucketKeyFileStore, fileStore)

			return r, cleanup, nil

		},
	})
	defer sim.Close()

	log.Info("Initializing test config")
	_, err := sim.AddNodesAndConnectChain(nodes)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) (err error) {
		nodeIDs := sim.UpNodeIDs()
		node := nodeIDs[len(nodeIDs)-1]

		item, ok := sim.NodeItem(node, bucketKeyFileStore)
		if !ok {
			b.Fatal("No filestore")
		}
		remoteFileStore := item.(*storage.FileStore)

		pivotNode := nodeIDs[0]
		item, ok = sim.NodeItem(pivotNode, bucketKeyNetStore)
		if !ok {
			b.Fatal("No filestore")
		}
		netStore := item.(*storage.NetStore)

		if _, err := sim.WaitTillHealthy(ctx); err != nil {
			return err
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
//基准环
		b.ResetTimer()
		b.StopTimer()
	Loop:
		for i := 0; i < b.N; i++ {
//将chunkcount随机块上载到最后一个节点
			hashes := make([]storage.Address, chunkCount)
			for i := 0; i < chunkCount; i++ {
//创建实际大小的实际块
				ctx := context.TODO()
				hash, wait, err := remoteFileStore.Store(ctx, testutil.RandomReader(i, chunkSize), int64(chunkSize), false)
				if err != nil {
					b.Fatalf("expected no error. got %v", err)
				}
//等待所有块存储
				err = wait(ctx)
				if err != nil {
					b.Fatalf("expected no error. got %v", err)
				}
//收集哈希
				hashes[i] = hash
			}
//现在以实际检索为基准
//对go例程中的每个哈希调用netstore.get，并收集错误
			b.StartTimer()
			errs := make(chan error)
			for _, hash := range hashes {
				go func(h storage.Address) {
					_, err := netStore.Get(ctx, h)
					log.Warn("test check netstore get", "hash", h, "err", err)
					errs <- err
				}(hash)
			}
//计数和报告检索错误
//如果有未命中，则块超时对于距离和音量而言太低（？）
			var total, misses int
			for err := range errs {
				if err != nil {
					log.Warn(err.Error())
					misses++
				}
				total++
				if total == chunkCount {
					break
				}
			}
			b.StopTimer()

			if misses > 0 {
				err = fmt.Errorf("%v chunk not found out of %v", misses, total)
				break Loop
			}
		}
		if err != nil {
			b.Fatal(err)
		}
		return nil
	})
	if result.Error != nil {
		b.Fatal(result.Error)
	}

}
