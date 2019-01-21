
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
	"flag"
	"fmt"
	"math/rand"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/mattn/go-colorable"
)

var (
	loglevel = flag.Int("loglevel", 2, "verbosity of logs")
)

func init() {
	flag.Parse()

	log.PrintOrigins(true)
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(true))))
}

//testservice实现node.service接口并提供协议
//以及用于测试模拟网络中节点的API
type testService struct {
	id enode.ID

//一旦执行了对等握手，对等方计数就会增加。
	peerCount int64

	peers    map[enode.ID]*testPeer
	peersMtx sync.Mutex

//状态存储用于测试创建和加载的[]字节
//快照
	state atomic.Value
}

func newTestService(ctx *adapters.ServiceContext) (node.Service, error) {
	svc := &testService{
		id:    ctx.Config.ID,
		peers: make(map[enode.ID]*testPeer),
	}
	svc.state.Store(ctx.Snapshot)
	return svc, nil
}

type testPeer struct {
	testReady chan struct{}
	dumReady  chan struct{}
}

func (t *testService) peer(id enode.ID) *testPeer {
	t.peersMtx.Lock()
	defer t.peersMtx.Unlock()
	if peer, ok := t.peers[id]; ok {
		return peer
	}
	peer := &testPeer{
		testReady: make(chan struct{}),
		dumReady:  make(chan struct{}),
	}
	t.peers[id] = peer
	return peer
}

func (t *testService) Protocols() []p2p.Protocol {
	return []p2p.Protocol{
		{
			Name:    "test",
			Version: 1,
			Length:  3,
			Run:     t.RunTest,
		},
		{
			Name:    "dum",
			Version: 1,
			Length:  1,
			Run:     t.RunDum,
		},
		{
			Name:    "prb",
			Version: 1,
			Length:  1,
			Run:     t.RunPrb,
		},
	}
}

func (t *testService) APIs() []rpc.API {
	return []rpc.API{{
		Namespace: "test",
		Version:   "1.0",
		Service: &TestAPI{
			state:     &t.state,
			peerCount: &t.peerCount,
		},
	}}
}

func (t *testService) Start(server *p2p.Server) error {
	return nil
}

func (t *testService) Stop() error {
	return nil
}

//握手通过发送和期望空的
//带有给定代码的消息
func (t *testService) handshake(rw p2p.MsgReadWriter, code uint64) error {
	errc := make(chan error, 2)
	go func() { errc <- p2p.Send(rw, code, struct{}{}) }()
	go func() { errc <- p2p.ExpectMsg(rw, code, struct{}{}) }()
	for i := 0; i < 2; i++ {
		if err := <-errc; err != nil {
			return err
		}
	}
	return nil
}

func (t *testService) RunTest(p *p2p.Peer, rw p2p.MsgReadWriter) error {
	peer := t.peer(p.ID())

//用三个不同的信息码进行三次握手，
//用于测试消息发送和筛选
	if err := t.handshake(rw, 2); err != nil {
		return err
	}
	if err := t.handshake(rw, 1); err != nil {
		return err
	}
	if err := t.handshake(rw, 0); err != nil {
		return err
	}

//关闭TestReady通道，以便其他协议可以运行
	close(peer.testReady)

//追踪同行
	atomic.AddInt64(&t.peerCount, 1)
	defer atomic.AddInt64(&t.peerCount, -1)

//阻止，直到删除对等机
	for {
		_, err := rw.ReadMsg()
		if err != nil {
			return err
		}
	}
}

func (t *testService) RunDum(p *p2p.Peer, rw p2p.MsgReadWriter) error {
	peer := t.peer(p.ID())

//等待测试协议执行其握手
	<-peer.testReady

//握手
	if err := t.handshake(rw, 0); err != nil {
		return err
	}

//关闭DumReady通道，以便其他协议可以运行
	close(peer.dumReady)

//阻止，直到删除对等机
	for {
		_, err := rw.ReadMsg()
		if err != nil {
			return err
		}
	}
}
func (t *testService) RunPrb(p *p2p.Peer, rw p2p.MsgReadWriter) error {
	peer := t.peer(p.ID())

//等待dum协议执行握手
	<-peer.dumReady

//握手
	if err := t.handshake(rw, 0); err != nil {
		return err
	}

//阻止，直到删除对等机
	for {
		_, err := rw.ReadMsg()
		if err != nil {
			return err
		}
	}
}

func (t *testService) Snapshot() ([]byte, error) {
	return t.state.Load().([]byte), nil
}

//test api提供了一个测试API，用于：
//*获取对等计数
//*获取并设置任意状态字节片
//*获取并递增一个计数器
//*订阅计数器增量事件
type TestAPI struct {
	state     *atomic.Value
	peerCount *int64
	counter   int64
	feed      event.Feed
}

func (t *TestAPI) PeerCount() int64 {
	return atomic.LoadInt64(t.peerCount)
}

func (t *TestAPI) Get() int64 {
	return atomic.LoadInt64(&t.counter)
}

func (t *TestAPI) Add(delta int64) {
	atomic.AddInt64(&t.counter, delta)
	t.feed.Send(delta)
}

func (t *TestAPI) GetState() []byte {
	return t.state.Load().([]byte)
}

func (t *TestAPI) SetState(state []byte) {
	t.state.Store(state)
}

func (t *TestAPI) Events(ctx context.Context) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return nil, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		events := make(chan int64)
		sub := t.feed.Subscribe(events)
		defer sub.Unsubscribe()

		for {
			select {
			case event := <-events:
				notifier.Notify(rpcSub.ID, event)
			case <-sub.Err():
				return
			case <-rpcSub.Err():
				return
			case <-notifier.Closed():
				return
			}
		}
	}()

	return rpcSub, nil
}

var testServices = adapters.Services{
	"test": newTestService,
}

func testHTTPServer(t *testing.T) (*Network, *httptest.Server) {
	t.Helper()
	adapter := adapters.NewSimAdapter(testServices)
	network := NewNetwork(adapter, &NetworkConfig{
		DefaultService: "test",
	})
	return network, httptest.NewServer(NewServer(network))
}

//testhttpnetwork使用http与仿真网络交互的测试
//美国石油学会
func TestHTTPNetwork(t *testing.T) {
//启动服务器
	network, s := testHTTPServer(t)
	defer s.Close()

//订阅活动，以便稍后检查
	client := NewClient(s.URL)
	events := make(chan *Event, 100)
	var opts SubscribeOpts
	sub, err := client.SubscribeNetwork(events, opts)
	if err != nil {
		t.Fatalf("error subscribing to network events: %s", err)
	}
	defer sub.Unsubscribe()

//检查我们是否可以检索网络的详细信息
	gotNetwork, err := client.GetNetwork()
	if err != nil {
		t.Fatalf("error getting network: %s", err)
	}
	if gotNetwork.ID != network.ID {
		t.Fatalf("expected network to have ID %q, got %q", network.ID, gotNetwork.ID)
	}

//启动模拟网络
	nodeIDs := startTestNetwork(t, client)

//检查我们有所有的活动
	x := &expectEvents{t, events, sub}
	x.expect(
		x.nodeEvent(nodeIDs[0], false),
		x.nodeEvent(nodeIDs[1], false),
		x.nodeEvent(nodeIDs[0], true),
		x.nodeEvent(nodeIDs[1], true),
		x.connEvent(nodeIDs[0], nodeIDs[1], false),
		x.connEvent(nodeIDs[0], nodeIDs[1], true),
	)

//重新连接流并检查当前节点和conn
	events = make(chan *Event, 100)
	opts.Current = true
	sub, err = client.SubscribeNetwork(events, opts)
	if err != nil {
		t.Fatalf("error subscribing to network events: %s", err)
	}
	defer sub.Unsubscribe()
	x = &expectEvents{t, events, sub}
	x.expect(
		x.nodeEvent(nodeIDs[0], true),
		x.nodeEvent(nodeIDs[1], true),
		x.connEvent(nodeIDs[0], nodeIDs[1], true),
	)
}

func startTestNetwork(t *testing.T, client *Client) []string {
//创建两个节点
	nodeCount := 2
	nodeIDs := make([]string, nodeCount)
	for i := 0; i < nodeCount; i++ {
		config := adapters.RandomNodeConfig()
		node, err := client.CreateNode(config)
		if err != nil {
			t.Fatalf("error creating node: %s", err)
		}
		nodeIDs[i] = node.ID
	}

//检查两个节点是否存在
	nodes, err := client.GetNodes()
	if err != nil {
		t.Fatalf("error getting nodes: %s", err)
	}
	if len(nodes) != nodeCount {
		t.Fatalf("expected %d nodes, got %d", nodeCount, len(nodes))
	}
	for i, nodeID := range nodeIDs {
		if nodes[i].ID != nodeID {
			t.Fatalf("expected node %d to have ID %q, got %q", i, nodeID, nodes[i].ID)
		}
		node, err := client.GetNode(nodeID)
		if err != nil {
			t.Fatalf("error getting node %d: %s", i, err)
		}
		if node.ID != nodeID {
			t.Fatalf("expected node %d to have ID %q, got %q", i, nodeID, node.ID)
		}
	}

//启动两个节点
	for _, nodeID := range nodeIDs {
		if err := client.StartNode(nodeID); err != nil {
			t.Fatalf("error starting node %q: %s", nodeID, err)
		}
	}

//连接节点
	for i := 0; i < nodeCount-1; i++ {
		peerId := i + 1
		if i == nodeCount-1 {
			peerId = 0
		}
		if err := client.ConnectNode(nodeIDs[i], nodeIDs[peerId]); err != nil {
			t.Fatalf("error connecting nodes: %s", err)
		}
	}

	return nodeIDs
}

type expectEvents struct {
	*testing.T

	events chan *Event
	sub    event.Subscription
}

func (t *expectEvents) nodeEvent(id string, up bool) *Event {
	return &Event{
		Type: EventTypeNode,
		Node: &Node{
			Config: &adapters.NodeConfig{
				ID: enode.HexID(id),
			},
			Up: up,
		},
	}
}

func (t *expectEvents) connEvent(one, other string, up bool) *Event {
	return &Event{
		Type: EventTypeConn,
		Conn: &Conn{
			One:   enode.HexID(one),
			Other: enode.HexID(other),
			Up:    up,
		},
	}
}

func (t *expectEvents) expectMsgs(expected map[MsgFilter]int) {
	actual := make(map[MsgFilter]int)
	timeout := time.After(10 * time.Second)
loop:
	for {
		select {
		case event := <-t.events:
			t.Logf("received %s event: %s", event.Type, event)

			if event.Type != EventTypeMsg || event.Msg.Received {
				continue loop
			}
			if event.Msg == nil {
				t.Fatal("expected event.Msg to be set")
			}
			filter := MsgFilter{
				Proto: event.Msg.Protocol,
				Code:  int64(event.Msg.Code),
			}
			actual[filter]++
			if actual[filter] > expected[filter] {
				t.Fatalf("received too many msgs for filter: %v", filter)
			}
			if reflect.DeepEqual(actual, expected) {
				return
			}

		case err := <-t.sub.Err():
			t.Fatalf("network stream closed unexpectedly: %s", err)

		case <-timeout:
			t.Fatal("timed out waiting for expected events")
		}
	}
}

func (t *expectEvents) expect(events ...*Event) {
	timeout := time.After(10 * time.Second)
	i := 0
	for {
		select {
		case event := <-t.events:
			t.Logf("received %s event: %s", event.Type, event)

			expected := events[i]
			if event.Type != expected.Type {
				t.Fatalf("expected event %d to have type %q, got %q", i, expected.Type, event.Type)
			}

			switch expected.Type {

			case EventTypeNode:
				if event.Node == nil {
					t.Fatal("expected event.Node to be set")
				}
				if event.Node.ID() != expected.Node.ID() {
					t.Fatalf("expected node event %d to have id %q, got %q", i, expected.Node.ID().TerminalString(), event.Node.ID().TerminalString())
				}
				if event.Node.Up != expected.Node.Up {
					t.Fatalf("expected node event %d to have up=%t, got up=%t", i, expected.Node.Up, event.Node.Up)
				}

			case EventTypeConn:
				if event.Conn == nil {
					t.Fatal("expected event.Conn to be set")
				}
				if event.Conn.One != expected.Conn.One {
					t.Fatalf("expected conn event %d to have one=%q, got one=%q", i, expected.Conn.One.TerminalString(), event.Conn.One.TerminalString())
				}
				if event.Conn.Other != expected.Conn.Other {
					t.Fatalf("expected conn event %d to have other=%q, got other=%q", i, expected.Conn.Other.TerminalString(), event.Conn.Other.TerminalString())
				}
				if event.Conn.Up != expected.Conn.Up {
					t.Fatalf("expected conn event %d to have up=%t, got up=%t", i, expected.Conn.Up, event.Conn.Up)
				}

			}

			i++
			if i == len(events) {
				return
			}

		case err := <-t.sub.Err():
			t.Fatalf("network stream closed unexpectedly: %s", err)

		case <-timeout:
			t.Fatal("timed out waiting for expected events")
		}
	}
}

//testhttpnoderpc测试通过HTTP API在节点上调用RPC方法
func TestHTTPNodeRPC(t *testing.T) {
//启动服务器
	_, s := testHTTPServer(t)
	defer s.Close()

//启动网络中的节点
	client := NewClient(s.URL)

	config := adapters.RandomNodeConfig()
	node, err := client.CreateNode(config)
	if err != nil {
		t.Fatalf("error creating node: %s", err)
	}
	if err := client.StartNode(node.ID); err != nil {
		t.Fatalf("error starting node: %s", err)
	}

//创建两个RPC客户端
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rpcClient1, err := client.RPCClient(ctx, node.ID)
	if err != nil {
		t.Fatalf("error getting node RPC client: %s", err)
	}
	rpcClient2, err := client.RPCClient(ctx, node.ID)
	if err != nil {
		t.Fatalf("error getting node RPC client: %s", err)
	}

//使用客户端1订阅事件
	events := make(chan int64, 1)
	sub, err := rpcClient1.Subscribe(ctx, "test", events, "events")
	if err != nil {
		t.Fatalf("error subscribing to events: %s", err)
	}
	defer sub.Unsubscribe()

//使用客户端2调用某些RPC方法
	if err := rpcClient2.CallContext(ctx, nil, "test_add", 10); err != nil {
		t.Fatalf("error calling RPC method: %s", err)
	}
	var result int64
	if err := rpcClient2.CallContext(ctx, &result, "test_get"); err != nil {
		t.Fatalf("error calling RPC method: %s", err)
	}
	if result != 10 {
		t.Fatalf("expected result to be 10, got %d", result)
	}

//检查我们从客户机1收到一个事件
	select {
	case event := <-events:
		if event != 10 {
			t.Fatalf("expected event to be 10, got %d", event)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

//testhttpsnapshot测试创建和加载网络快照
func TestHTTPSnapshot(t *testing.T) {
//启动服务器
	network, s := testHTTPServer(t)
	defer s.Close()

	var eventsDone = make(chan struct{})
	count := 1
	eventsDoneChan := make(chan *Event)
	eventSub := network.Events().Subscribe(eventsDoneChan)
	go func() {
		defer eventSub.Unsubscribe()
		for event := range eventsDoneChan {
			if event.Type == EventTypeConn && !event.Control {
				count--
				if count == 0 {
					eventsDone <- struct{}{}
					return
				}
			}
		}
	}()

//创建两节点网络
	client := NewClient(s.URL)
	nodeCount := 2
	nodes := make([]*p2p.NodeInfo, nodeCount)
	for i := 0; i < nodeCount; i++ {
		config := adapters.RandomNodeConfig()
		node, err := client.CreateNode(config)
		if err != nil {
			t.Fatalf("error creating node: %s", err)
		}
		if err := client.StartNode(node.ID); err != nil {
			t.Fatalf("error starting node: %s", err)
		}
		nodes[i] = node
	}
	if err := client.ConnectNode(nodes[0].ID, nodes[1].ID); err != nil {
		t.Fatalf("error connecting nodes: %s", err)
	}

//在测试服务中存储一些状态
	states := make([]string, nodeCount)
	for i, node := range nodes {
		rpc, err := client.RPCClient(context.Background(), node.ID)
		if err != nil {
			t.Fatalf("error getting RPC client: %s", err)
		}
		defer rpc.Close()
		state := fmt.Sprintf("%x", rand.Int())
		if err := rpc.Call(nil, "test_setState", []byte(state)); err != nil {
			t.Fatalf("error setting service state: %s", err)
		}
		states[i] = state
	}
	<-eventsDone
//创建快照
	snap, err := client.CreateSnapshot()
	if err != nil {
		t.Fatalf("error creating snapshot: %s", err)
	}
	for i, state := range states {
		gotState := snap.Nodes[i].Snapshots["test"]
		if string(gotState) != state {
			t.Fatalf("expected snapshot state %q, got %q", state, gotState)
		}
	}

//创建另一个网络
	network2, s := testHTTPServer(t)
	defer s.Close()
	client = NewClient(s.URL)
	count = 1
	eventSub = network2.Events().Subscribe(eventsDoneChan)
	go func() {
		defer eventSub.Unsubscribe()
		for event := range eventsDoneChan {
			if event.Type == EventTypeConn && !event.Control {
				count--
				if count == 0 {
					eventsDone <- struct{}{}
					return
				}
			}
		}
	}()

//订阅活动，以便稍后检查
	events := make(chan *Event, 100)
	var opts SubscribeOpts
	sub, err := client.SubscribeNetwork(events, opts)
	if err != nil {
		t.Fatalf("error subscribing to network events: %s", err)
	}
	defer sub.Unsubscribe()

//加载快照
	if err := client.LoadSnapshot(snap); err != nil {
		t.Fatalf("error loading snapshot: %s", err)
	}
	<-eventsDone

//检查节点和连接是否存在
	net, err := client.GetNetwork()
	if err != nil {
		t.Fatalf("error getting network: %s", err)
	}
	if len(net.Nodes) != nodeCount {
		t.Fatalf("expected network to have %d nodes, got %d", nodeCount, len(net.Nodes))
	}
	for i, node := range nodes {
		id := net.Nodes[i].ID().String()
		if id != node.ID {
			t.Fatalf("expected node %d to have ID %s, got %s", i, node.ID, id)
		}
	}
	if len(net.Conns) != 1 {
		t.Fatalf("expected network to have 1 connection, got %d", len(net.Conns))
	}
	conn := net.Conns[0]
	if conn.One.String() != nodes[0].ID {
		t.Fatalf("expected connection to have one=%q, got one=%q", nodes[0].ID, conn.One)
	}
	if conn.Other.String() != nodes[1].ID {
		t.Fatalf("expected connection to have other=%q, got other=%q", nodes[1].ID, conn.Other)
	}
	if !conn.Up {
		t.Fatal("should be up")
	}

//检查节点状态是否已还原
	for i, node := range nodes {
		rpc, err := client.RPCClient(context.Background(), node.ID)
		if err != nil {
			t.Fatalf("error getting RPC client: %s", err)
		}
		defer rpc.Close()
		var state []byte
		if err := rpc.Call(&state, "test_getState"); err != nil {
			t.Fatalf("error getting service state: %s", err)
		}
		if string(state) != states[i] {
			t.Fatalf("expected snapshot state %q, got %q", states[i], state)
		}
	}

//检查我们有所有的活动
	x := &expectEvents{t, events, sub}
	x.expect(
		x.nodeEvent(nodes[0].ID, false),
		x.nodeEvent(nodes[0].ID, true),
		x.nodeEvent(nodes[1].ID, false),
		x.nodeEvent(nodes[1].ID, true),
		x.connEvent(nodes[0].ID, nodes[1].ID, false),
		x.connEvent(nodes[0].ID, nodes[1].ID, true),
	)
}

//testmsgfilterpassmultiple使用筛选器测试流式消息事件
//有多种协议
func TestMsgFilterPassMultiple(t *testing.T) {
//启动服务器
	_, s := testHTTPServer(t)
	defer s.Close()

//使用消息筛选器订阅事件
	client := NewClient(s.URL)
	events := make(chan *Event, 10)
	opts := SubscribeOpts{
		Filter: "prb:0-test:0",
	}
	sub, err := client.SubscribeNetwork(events, opts)
	if err != nil {
		t.Fatalf("error subscribing to network events: %s", err)
	}
	defer sub.Unsubscribe()

//启动模拟网络
	startTestNetwork(t, client)

//检查我们得到了预期的事件
	x := &expectEvents{t, events, sub}
	x.expectMsgs(map[MsgFilter]int{
		{"test", 0}: 2,
		{"prb", 0}:  2,
	})
}

//testmsgfilterpasswildcard使用筛选器测试流式消息事件
//使用代码通配符
func TestMsgFilterPassWildcard(t *testing.T) {
//启动服务器
	_, s := testHTTPServer(t)
	defer s.Close()

//使用消息筛选器订阅事件
	client := NewClient(s.URL)
	events := make(chan *Event, 10)
	opts := SubscribeOpts{
		Filter: "prb:0,2-test:*",
	}
	sub, err := client.SubscribeNetwork(events, opts)
	if err != nil {
		t.Fatalf("error subscribing to network events: %s", err)
	}
	defer sub.Unsubscribe()

//启动模拟网络
	startTestNetwork(t, client)

//检查我们得到了预期的事件
	x := &expectEvents{t, events, sub}
	x.expectMsgs(map[MsgFilter]int{
		{"test", 2}: 2,
		{"test", 1}: 2,
		{"test", 0}: 2,
		{"prb", 0}:  2,
	})
}

//testmsgfilterpasssingle使用筛选器测试流式消息事件
//只有一个协议和代码
func TestMsgFilterPassSingle(t *testing.T) {
//启动服务器
	_, s := testHTTPServer(t)
	defer s.Close()

//使用消息筛选器订阅事件
	client := NewClient(s.URL)
	events := make(chan *Event, 10)
	opts := SubscribeOpts{
		Filter: "dum:0",
	}
	sub, err := client.SubscribeNetwork(events, opts)
	if err != nil {
		t.Fatalf("error subscribing to network events: %s", err)
	}
	defer sub.Unsubscribe()

//启动模拟网络
	startTestNetwork(t, client)

//检查我们得到了预期的事件
	x := &expectEvents{t, events, sub}
	x.expectMsgs(map[MsgFilter]int{
		{"dum", 0}: 2,
	})
}

//testmsgfilterpasssingle使用无效的
//滤波器
func TestMsgFilterFailBadParams(t *testing.T) {
//启动服务器
	_, s := testHTTPServer(t)
	defer s.Close()

	client := NewClient(s.URL)
	events := make(chan *Event, 10)
	opts := SubscribeOpts{
		Filter: "foo:",
	}
	_, err := client.SubscribeNetwork(events, opts)
	if err == nil {
		t.Fatalf("expected event subscription to fail but succeeded!")
	}

	opts.Filter = "bzz:aa"
	_, err = client.SubscribeNetwork(events, opts)
	if err == nil {
		t.Fatalf("expected event subscription to fail but succeeded!")
	}

	opts.Filter = "invalid"
	_, err = client.SubscribeNetwork(events, opts)
	if err == nil {
		t.Fatalf("expected event subscription to fail but succeeded!")
	}
}
