
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

//+使用服务器生成

package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/simulation"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

/*
此文件中的测试需要使用

   -tags=带服务器

另外，如果单独执行，它们将暂停，因为它们等待
用于可视化前端发送post/runsim消息。
**/


//设置SIM，评估nodeCount和chunkCount并创建SIM
func setupSim(serviceMap map[string]simulation.ServiceFunc) (int, int, *simulation.Simulation) {
	nodeCount := *nodes
	chunkCount := *chunks

	if nodeCount == 0 || chunkCount == 0 {
		nodeCount = 32
		chunkCount = 1
	}

//使用服务器设置模拟，这意味着SIM卡无法运行
//直到它从前端接收到一个post/runsim
	sim := simulation.New(serviceMap).WithServer(":8888")
	return nodeCount, chunkCount, sim
}

//注意断开连接并等待健康
func watchSim(sim *simulation.Simulation) (context.Context, context.CancelFunc) {
	ctx, cancelSimRun := context.WithTimeout(context.Background(), 1*time.Minute)

	if _, err := sim.WaitTillHealthy(ctx); err != nil {
		panic(err)
	}

	disconnections := sim.PeerEvents(
		context.Background(),
		sim.NodeIDs(),
		simulation.NewPeerEventsFilter().Drop(),
	)

	go func() {
		for d := range disconnections {
			log.Error("peer drop", "node", d.NodeID, "peer", d.PeerID)
			panic("unexpected disconnect")
			cancelSimRun()
		}
	}()

	return ctx, cancelSimRun
}

//此测试请求网络中存在伪造哈希
func TestNonExistingHashesWithServer(t *testing.T) {

	nodeCount, _, sim := setupSim(retrievalSimServiceMap)
	defer sim.Close()

	err := sim.UploadSnapshot(fmt.Sprintf("testing/snapshot_%d.json", nodeCount))
	if err != nil {
		panic(err)
	}

	ctx, cancelSimRun := watchSim(sim)
	defer cancelSimRun()

//为了获得一些有意义的可视化效果，这是有益的
//定义此测试的最短持续时间
	testDuration := 20 * time.Second

	result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) error {
//检查节点的文件存储（netstore）
		id := sim.Net.GetRandomUpNode().ID()
		item, ok := sim.NodeItem(id, bucketKeyFileStore)
		if !ok {
			t.Fatalf("No filestore")
		}
		fileStore := item.(*storage.FileStore)
//创建伪哈希
		fakeHash := storage.GenerateRandomChunk(1000).Address()
//尝试检索它-将把retrieverequestmsg传播到网络中
		reader, _ := fileStore.Retrieve(context.TODO(), fakeHash)
		if _, err := reader.Size(ctx, nil); err != nil {
			log.Debug("expected error for non-existing chunk")
		}
//睡眠，以便前端可以显示一些内容
		time.Sleep(testDuration)

		return nil
	})
	if result.Error != nil {
		sendSimTerminatedEvent(sim)
		t.Fatal(result.Error)
	}

	sendSimTerminatedEvent(sim)

}

//向前端发送终止事件
func sendSimTerminatedEvent(sim *simulation.Simulation) {
	evt := &simulations.Event{
		Type:    EventTypeSimTerminated,
		Control: false,
	}
	sim.Net.Events().Send(evt)
}

//此测试与快照同步测试相同，
//但是使用HTTP服务器
//它还发送一些自定义事件，以便前端
//可以可视化消息，如sendOfferedMsg、wantedHashesMsg、deliveryMsg
func TestSnapshotSyncWithServer(t *testing.T) {
//t.skip（“暂时禁用为Simulations.WaittillHealthy，不可信任”）。

//定义一个包装对象以便能够传递数据
	wrapper := &netWrapper{}

	nodeCount := *nodes
	chunkCount := *chunks

	if nodeCount == 0 || chunkCount == 0 {
		nodeCount = 32
		chunkCount = 1
	}

	log.Info(fmt.Sprintf("Running the simulation with %d nodes and %d chunks", nodeCount, chunkCount))

	sim := simulation.New(map[string]simulation.ServiceFunc{
		"streamer": func(ctx *adapters.ServiceContext, bucket *sync.Map) (s node.Service, cleanup func(), err error) {
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

			tr := &testRegistry{
				Registry: r,
				w:        wrapper,
			}

			bucket.Store(bucketKeyRegistry, tr)

			cleanup = func() {
				netStore.Close()
				tr.Close()
				os.RemoveAll(datadir)
			}

			return tr, cleanup, nil
		},
}).WithServer(":8888") //从HTTP服务器开始

	nodeCount, chunkCount, sim := setupSim(simServiceMap)
	defer sim.Close()

	log.Info("Initializing test config")

	conf := &synctestConfig{}
//发现ID到该ID处预期的块索引的映射
	conf.idToChunksMap = make(map[enode.ID][]int)
//发现ID的覆盖地址映射
	conf.addrToIDMap = make(map[string]enode.ID)
//存储生成的块哈希的数组
	conf.hashes = make([]storage.Address, 0)
//将网络传递到包装对象
	wrapper.setNetwork(sim.Net)
	err := sim.UploadSnapshot(fmt.Sprintf("testing/snapshot_%d.json", nodeCount))
	if err != nil {
		panic(err)
	}

	ctx, cancelSimRun := watchSim(sim)
	defer cancelSimRun()

//运行SIM
	result := runSim(conf, ctx, sim, chunkCount)

//发送终止事件
	evt := &simulations.Event{
		Type:    EventTypeSimTerminated,
		Control: false,
	}
	go sim.Net.Events().Send(evt)

	if result.Error != nil {
		panic(result.Error)
	}
	log.Info("Simulation ended")
}

//TestRegistry嵌入注册表
//它允许替换协议运行功能
type testRegistry struct {
	*Registry
	w *netWrapper
}

//协议替换协议的运行功能
func (tr *testRegistry) Protocols() []p2p.Protocol {
	regProto := tr.Registry.Protocols()
//用testregistry的run函数设置'stream'协议的run函数
	regProto[0].Run = tr.runProto
	return regProto
}

//runproto是此测试的新覆盖协议的run函数
func (tr *testRegistry) runProto(p *p2p.Peer, rw p2p.MsgReadWriter) error {
//创建自定义的rw消息读写器
	testRw := &testMsgReadWriter{
		MsgReadWriter: rw,
		Peer:          p,
		w:             tr.w,
		Registry:      tr.Registry,
	}
//现在运行实际的上层“注册表”的协议函数
	return tr.runProtocol(p, testRw)
}

//testmsgreadwriter是一个自定义的rw
//它将允许我们重复使用该消息两次
type testMsgReadWriter struct {
	*Registry
	p2p.MsgReadWriter
	*p2p.Peer
	w *netWrapper
}

//NetRapper包装器对象，以便我们可以传递数据
type netWrapper struct {
	net *simulations.Network
}

//将网络设置为包装器以供以后使用（在自定义rw中使用）
func (w *netWrapper) setNetwork(n *simulations.Network) {
	w.net = n
}

//从包装器获取网络（在自定义rw中使用）
func (w *netWrapper) getNetwork() *simulations.Network {
	return w.net
}

//readmsg从基础msgreadwriter读取消息并发出
//“收到消息”事件
//我们这样做是因为我们对定制使用的消息的有效负载感兴趣
//在此测试中，但消息只能使用一次（stream io.reader）
func (ev *testMsgReadWriter) ReadMsg() (p2p.Msg, error) {
//从底层的rw读取消息
	msg, err := ev.MsgReadWriter.ReadMsg()
	if err != nil {
		return msg, err
	}

//不要对我们实际上不需要/不需要阅读的消息代码做任何事情
	subCodes := []uint64{1, 2, 10}
	found := false
	for _, c := range subCodes {
		if c == msg.Code {
			found = true
		}
	}
//如果不是我们感兴趣的消息代码，请返回
	if !found {
		return msg, nil
	}

//我们使用IO.teeReader，这样我们可以读两次消息
//有效负载是一个IO.reader，所以如果我们从中读取，实际的协议处理程序
//无法再访问它。
//但是我们需要这个处理程序能够正常地使用消息，
//好像我们不会在这里用那个信息做任何事情
	var buf bytes.Buffer
	tee := io.TeeReader(msg.Payload, &buf)

	mcp := &p2p.Msg{
		Code:       msg.Code,
		Size:       msg.Size,
		ReceivedAt: msg.ReceivedAt,
		Payload:    tee,
	}
//分配副本供以后使用
	msg.Payload = &buf

//现在让我们看看这个消息
	var wmsg protocols.WrappedMsg
	err = mcp.Decode(&wmsg)
	if err != nil {
		log.Error(err.Error())
		return msg, err
	}
//从代码创建新消息
	val, ok := ev.Registry.GetSpec().NewMsg(mcp.Code)
	if !ok {
		return msg, errors.New(fmt.Sprintf("Invalid message code: %v", msg.Code))
	}
//解码它
	if err := rlp.DecodeBytes(wmsg.Payload, val); err != nil {
		return msg, errors.New(fmt.Sprintf("Decoding error <= %v: %v", msg, err))
	}
//现在，对于我们感兴趣的每种消息类型，创建一个自定义事件并发送它
	var evt *simulations.Event
	switch val := val.(type) {
	case *OfferedHashesMsg:
		evt = &simulations.Event{
			Type:    EventTypeChunkOffered,
			Node:    ev.w.getNetwork().GetNode(ev.ID()),
			Control: false,
			Data:    val.Hashes,
		}
	case *WantedHashesMsg:
		evt = &simulations.Event{
			Type:    EventTypeChunkWanted,
			Node:    ev.w.getNetwork().GetNode(ev.ID()),
			Control: false,
		}
	case *ChunkDeliveryMsgSyncing:
		evt = &simulations.Event{
			Type:    EventTypeChunkDelivered,
			Node:    ev.w.getNetwork().GetNode(ev.ID()),
			Control: false,
			Data:    val.Addr.String(),
		}
	}
	if evt != nil {
//将自定义事件发送到订阅源；前端将侦听该事件并显示
		ev.w.getNetwork().Events().Send(evt)
	}
	return msg, nil
}
