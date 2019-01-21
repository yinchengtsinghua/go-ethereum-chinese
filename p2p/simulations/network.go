
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
)

var DialBanTimeout = 200 * time.Millisecond

//networkconfig定义用于启动网络的配置选项
type NetworkConfig struct {
	ID             string `json:"id"`
	DefaultService string `json:"default_service,omitempty"`
}

//网络模型一个P2P仿真网络，它由一组
//模拟节点及其之间存在的连接。
//
//网络有一个单独的节点适配器，它实际上负责
//启动节点并将它们连接在一起。
//
//当节点启动和停止时，网络会发出事件
//连接和断开连接，以及在节点之间发送消息时。
type Network struct {
	NetworkConfig

	Nodes   []*Node `json:"nodes"`
	nodeMap map[enode.ID]int

	Conns   []*Conn `json:"conns"`
	connMap map[string]int

	nodeAdapter adapters.NodeAdapter
	events      event.Feed
	lock        sync.RWMutex
	quitc       chan struct{}
}

//newnetwork返回使用给定nodeadapter和networkconfig的网络
func NewNetwork(nodeAdapter adapters.NodeAdapter, conf *NetworkConfig) *Network {
	return &Network{
		NetworkConfig: *conf,
		nodeAdapter:   nodeAdapter,
		nodeMap:       make(map[enode.ID]int),
		connMap:       make(map[string]int),
		quitc:         make(chan struct{}),
	}
}

//事件返回网络的输出事件源。
func (net *Network) Events() *event.Feed {
	return &net.events
}

//new node with config使用给定的配置向网络添加新节点，
//如果已存在具有相同ID或名称的节点，则返回错误
func (net *Network) NewNodeWithConfig(conf *adapters.NodeConfig) (*Node, error) {
	net.lock.Lock()
	defer net.lock.Unlock()

	if conf.Reachable == nil {
		conf.Reachable = func(otherID enode.ID) bool {
			_, err := net.InitConn(conf.ID, otherID)
			if err != nil && bytes.Compare(conf.ID.Bytes(), otherID.Bytes()) < 0 {
				return false
			}
			return true
		}
	}

//检查节点是否已存在
	if node := net.getNode(conf.ID); node != nil {
		return nil, fmt.Errorf("node with ID %q already exists", conf.ID)
	}
	if node := net.getNodeByName(conf.Name); node != nil {
		return nil, fmt.Errorf("node with name %q already exists", conf.Name)
	}

//如果未配置任何服务，请使用默认服务
	if len(conf.Services) == 0 {
		conf.Services = []string{net.DefaultService}
	}

//使用nodeadapter创建节点
	adapterNode, err := net.nodeAdapter.NewNode(conf)
	if err != nil {
		return nil, err
	}
	node := &Node{
		Node:   adapterNode,
		Config: conf,
	}
	log.Trace("Node created", "id", conf.ID)
	net.nodeMap[conf.ID] = len(net.Nodes)
	net.Nodes = append(net.Nodes, node)

//发出“控制”事件
	net.events.Send(ControlEvent(node))

	return node, nil
}

//config返回网络配置
func (net *Network) Config() *NetworkConfig {
	return &net.NetworkConfig
}

//StartAll启动网络中的所有节点
func (net *Network) StartAll() error {
	for _, node := range net.Nodes {
		if node.Up {
			continue
		}
		if err := net.Start(node.ID()); err != nil {
			return err
		}
	}
	return nil
}

//stopall停止网络中的所有节点
func (net *Network) StopAll() error {
	for _, node := range net.Nodes {
		if !node.Up {
			continue
		}
		if err := net.Stop(node.ID()); err != nil {
			return err
		}
	}
	return nil
}

//Start用给定的ID启动节点
func (net *Network) Start(id enode.ID) error {
	return net.startWithSnapshots(id, nil)
}

//StartWithSnapshots使用给定的ID启动节点
//快照
func (net *Network) startWithSnapshots(id enode.ID, snapshots map[string][]byte) error {
	net.lock.Lock()
	defer net.lock.Unlock()

	node := net.getNode(id)
	if node == nil {
		return fmt.Errorf("node %v does not exist", id)
	}
	if node.Up {
		return fmt.Errorf("node %v already up", id)
	}
	log.Trace("Starting node", "id", id, "adapter", net.nodeAdapter.Name())
	if err := node.Start(snapshots); err != nil {
		log.Warn("Node startup failed", "id", id, "err", err)
		return err
	}
	node.Up = true
	log.Info("Started node", "id", id)

	net.events.Send(NewEvent(node))

//订阅对等事件
	client, err := node.Client()
	if err != nil {
		return fmt.Errorf("error getting rpc client  for node %v: %s", id, err)
	}
	events := make(chan *p2p.PeerEvent)
	sub, err := client.Subscribe(context.Background(), "admin", events, "peerEvents")
	if err != nil {
		return fmt.Errorf("error getting peer events for node %v: %s", id, err)
	}
	go net.watchPeerEvents(id, events, sub)
	return nil
}

//WatchPeerEvents从给定通道读取对等事件并发出
//相应的网络事件
func (net *Network) watchPeerEvents(id enode.ID, events chan *p2p.PeerEvent, sub event.Subscription) {
	defer func() {
		sub.Unsubscribe()

//假设节点现在已关闭
		net.lock.Lock()
		defer net.lock.Unlock()
		node := net.getNode(id)
		if node == nil {
			return
		}
		node.Up = false
		net.events.Send(NewEvent(node))
	}()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			peer := event.Peer
			switch event.Type {

			case p2p.PeerEventTypeAdd:
				net.DidConnect(id, peer)

			case p2p.PeerEventTypeDrop:
				net.DidDisconnect(id, peer)

			case p2p.PeerEventTypeMsgSend:
				net.DidSend(id, peer, event.Protocol, *event.MsgCode)

			case p2p.PeerEventTypeMsgRecv:
				net.DidReceive(peer, id, event.Protocol, *event.MsgCode)

			}

		case err := <-sub.Err():
			if err != nil {
				log.Error("Error in peer event subscription", "id", id, "err", err)
			}
			return
		}
	}
}

//stop停止具有给定ID的节点
func (net *Network) Stop(id enode.ID) error {
	net.lock.Lock()
	node := net.getNode(id)
	if node == nil {
		return fmt.Errorf("node %v does not exist", id)
	}
	if !node.Up {
		return fmt.Errorf("node %v already down", id)
	}
	node.Up = false
	net.lock.Unlock()

	err := node.Stop()
	if err != nil {
		net.lock.Lock()
		node.Up = true
		net.lock.Unlock()
		return err
	}
	log.Info("Stopped node", "id", id, "err", err)
	net.events.Send(ControlEvent(node))
	return nil
}

//connect通过调用“admin_addpeer”rpc将两个节点连接在一起
//方法，以便它连接到“另一个”节点
func (net *Network) Connect(oneID, otherID enode.ID) error {
	log.Debug("Connecting nodes with addPeer", "id", oneID, "other", otherID)
	conn, err := net.InitConn(oneID, otherID)
	if err != nil {
		return err
	}
	client, err := conn.one.Client()
	if err != nil {
		return err
	}
	net.events.Send(ControlEvent(conn))
	return client.Call(nil, "admin_addPeer", string(conn.other.Addr()))
}

//断开连接通过调用“admin-removepeer”rpc断开两个节点的连接
//方法，以便它与“另一个”节点断开连接
func (net *Network) Disconnect(oneID, otherID enode.ID) error {
	conn := net.GetConn(oneID, otherID)
	if conn == nil {
		return fmt.Errorf("connection between %v and %v does not exist", oneID, otherID)
	}
	if !conn.Up {
		return fmt.Errorf("%v and %v already disconnected", oneID, otherID)
	}
	client, err := conn.one.Client()
	if err != nil {
		return err
	}
	net.events.Send(ControlEvent(conn))
	return client.Call(nil, "admin_removePeer", string(conn.other.Addr()))
}

//didconnect跟踪“一个”节点连接到“另一个”节点的事实
func (net *Network) DidConnect(one, other enode.ID) error {
	net.lock.Lock()
	defer net.lock.Unlock()
	conn, err := net.getOrCreateConn(one, other)
	if err != nil {
		return fmt.Errorf("connection between %v and %v does not exist", one, other)
	}
	if conn.Up {
		return fmt.Errorf("%v and %v already connected", one, other)
	}
	conn.Up = true
	net.events.Send(NewEvent(conn))
	return nil
}

//didisconnect跟踪“one”节点与
//“其他”节点
func (net *Network) DidDisconnect(one, other enode.ID) error {
	net.lock.Lock()
	defer net.lock.Unlock()
	conn := net.getConn(one, other)
	if conn == nil {
		return fmt.Errorf("connection between %v and %v does not exist", one, other)
	}
	if !conn.Up {
		return fmt.Errorf("%v and %v already disconnected", one, other)
	}
	conn.Up = false
	conn.initiated = time.Now().Add(-DialBanTimeout)
	net.events.Send(NewEvent(conn))
	return nil
}

//didsend跟踪“sender”向“receiver”发送消息的事实
func (net *Network) DidSend(sender, receiver enode.ID, proto string, code uint64) error {
	msg := &Msg{
		One:      sender,
		Other:    receiver,
		Protocol: proto,
		Code:     code,
		Received: false,
	}
	net.events.Send(NewEvent(msg))
	return nil
}

//DidReceive跟踪“Receiver”从“Sender”收到消息的事实
func (net *Network) DidReceive(sender, receiver enode.ID, proto string, code uint64) error {
	msg := &Msg{
		One:      sender,
		Other:    receiver,
		Protocol: proto,
		Code:     code,
		Received: true,
	}
	net.events.Send(NewEvent(msg))
	return nil
}

//getnode获取具有给定ID的节点，如果该节点没有，则返回nil
//存在
func (net *Network) GetNode(id enode.ID) *Node {
	net.lock.RLock()
	defer net.lock.RUnlock()
	return net.getNode(id)
}

//getnode获取具有给定名称的节点，如果该节点执行此操作，则返回nil
//不存在
func (net *Network) GetNodeByName(name string) *Node {
	net.lock.RLock()
	defer net.lock.RUnlock()
	return net.getNodeByName(name)
}

func (net *Network) getNodeByName(name string) *Node {
	for _, node := range net.Nodes {
		if node.Config.Name == name {
			return node
		}
	}
	return nil
}

//GetNodes返回现有节点
func (net *Network) GetNodes() (nodes []*Node) {
	net.lock.RLock()
	defer net.lock.RUnlock()

	nodes = append(nodes, net.Nodes...)
	return nodes
}

func (net *Network) getNode(id enode.ID) *Node {
	i, found := net.nodeMap[id]
	if !found {
		return nil
	}
	return net.Nodes[i]
}

//getrandomupnode返回网络上正在运行的随机节点。
func (net *Network) GetRandomUpNode(excludeIDs ...enode.ID) *Node {
	net.lock.RLock()
	defer net.lock.RUnlock()
	return net.getRandomNode(net.getUpNodeIDs(), excludeIDs)
}

func (net *Network) getUpNodeIDs() (ids []enode.ID) {
	for _, node := range net.Nodes {
		if node.Up {
			ids = append(ids, node.ID())
		}
	}
	return ids
}

//GetRandomDownNode返回网络上的随机节点，该节点已停止。
func (net *Network) GetRandomDownNode(excludeIDs ...enode.ID) *Node {
	net.lock.RLock()
	defer net.lock.RUnlock()
	return net.getRandomNode(net.getDownNodeIDs(), excludeIDs)
}

func (net *Network) getDownNodeIDs() (ids []enode.ID) {
	for _, node := range net.GetNodes() {
		if !node.Up {
			ids = append(ids, node.ID())
		}
	}
	return ids
}

func (net *Network) getRandomNode(ids []enode.ID, excludeIDs []enode.ID) *Node {
	filtered := filterIDs(ids, excludeIDs)

	l := len(filtered)
	if l == 0 {
		return nil
	}
	return net.GetNode(filtered[rand.Intn(l)])
}

func filterIDs(ids []enode.ID, excludeIDs []enode.ID) []enode.ID {
	exclude := make(map[enode.ID]bool)
	for _, id := range excludeIDs {
		exclude[id] = true
	}
	var filtered []enode.ID
	for _, id := range ids {
		if _, found := exclude[id]; !found {
			filtered = append(filtered, id)
		}
	}
	return filtered
}

//getconn返回“一”和“另一”之间存在的连接
//无论哪个节点启动了连接
func (net *Network) GetConn(oneID, otherID enode.ID) *Conn {
	net.lock.RLock()
	defer net.lock.RUnlock()
	return net.getConn(oneID, otherID)
}

//getorCreateConn与getconn类似，但如果不相同，则创建连接
//已经存在
func (net *Network) GetOrCreateConn(oneID, otherID enode.ID) (*Conn, error) {
	net.lock.Lock()
	defer net.lock.Unlock()
	return net.getOrCreateConn(oneID, otherID)
}

func (net *Network) getOrCreateConn(oneID, otherID enode.ID) (*Conn, error) {
	if conn := net.getConn(oneID, otherID); conn != nil {
		return conn, nil
	}

	one := net.getNode(oneID)
	if one == nil {
		return nil, fmt.Errorf("node %v does not exist", oneID)
	}
	other := net.getNode(otherID)
	if other == nil {
		return nil, fmt.Errorf("node %v does not exist", otherID)
	}
	conn := &Conn{
		One:   oneID,
		Other: otherID,
		one:   one,
		other: other,
	}
	label := ConnLabel(oneID, otherID)
	net.connMap[label] = len(net.Conns)
	net.Conns = append(net.Conns, conn)
	return conn, nil
}

func (net *Network) getConn(oneID, otherID enode.ID) *Conn {
	label := ConnLabel(oneID, otherID)
	i, found := net.connMap[label]
	if !found {
		return nil
	}
	return net.Conns[i]
}

//initconn（一个，另一个）检索
//彼此对等，如果不存在则创建一个新的
//节点顺序无关紧要，即conn（i，j）==conn（j，i）
//它检查连接是否已经启动，以及节点是否正在运行
//注：
//它还检查最近是否有连接对等端的尝试
//这是欺骗，因为模拟被用作甲骨文并知道
//远程对等机尝试连接到一个节点，该节点随后将不会启动连接。
func (net *Network) InitConn(oneID, otherID enode.ID) (*Conn, error) {
	net.lock.Lock()
	defer net.lock.Unlock()
	if oneID == otherID {
		return nil, fmt.Errorf("refusing to connect to self %v", oneID)
	}
	conn, err := net.getOrCreateConn(oneID, otherID)
	if err != nil {
		return nil, err
	}
	if conn.Up {
		return nil, fmt.Errorf("%v and %v already connected", oneID, otherID)
	}
	if time.Since(conn.initiated) < DialBanTimeout {
		return nil, fmt.Errorf("connection between %v and %v recently attempted", oneID, otherID)
	}

	err = conn.nodesUp()
	if err != nil {
		log.Trace("Nodes not up", "err", err)
		return nil, fmt.Errorf("nodes not up: %v", err)
	}
	log.Debug("Connection initiated", "id", oneID, "other", otherID)
	conn.initiated = time.Now()
	return conn, nil
}

//shutdown停止网络中的所有节点并关闭退出通道
func (net *Network) Shutdown() {
	for _, node := range net.Nodes {
		log.Debug("Stopping node", "id", node.ID())
		if err := node.Stop(); err != nil {
			log.Warn("Can't stop node", "id", node.ID(), "err", err)
		}
	}
	close(net.quitc)
}

//重置重置所有网络属性：
//清空节点和连接列表
func (net *Network) Reset() {
	net.lock.Lock()
	defer net.lock.Unlock()

//重新初始化映射
	net.connMap = make(map[string]int)
	net.nodeMap = make(map[enode.ID]int)

	net.Nodes = nil
	net.Conns = nil
}

//node是围绕adapters.node的包装器，用于跟踪状态
//网络中节点的
type Node struct {
	adapters.Node `json:"-"`

//如果用于创建节点的配置
	Config *adapters.NodeConfig `json:"config"`

//向上跟踪节点是否正在运行
	Up bool `json:"up"`
}

//ID返回节点的ID
func (n *Node) ID() enode.ID {
	return n.Config.ID
}

//字符串返回日志友好的字符串
func (n *Node) String() string {
	return fmt.Sprintf("Node %v", n.ID().TerminalString())
}

//nodeinfo返回有关节点的信息
func (n *Node) NodeInfo() *p2p.NodeInfo {
//如果节点尚未启动，请避免出现恐慌。
	if n.Node == nil {
		return nil
	}
	info := n.Node.NodeInfo()
	info.Name = n.Config.Name
	return info
}

//marshaljson实现json.marshaler接口，以便
//json包括nodeinfo
func (n *Node) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Info   *p2p.NodeInfo        `json:"info,omitempty"`
		Config *adapters.NodeConfig `json:"config,omitempty"`
		Up     bool                 `json:"up"`
	}{
		Info:   n.NodeInfo(),
		Config: n.Config,
		Up:     n.Up,
	})
}

//conn表示网络中两个节点之间的连接
type Conn struct {
//一个是启动连接的节点
	One enode.ID `json:"one"`

//另一个是连接到的节点
	Other enode.ID `json:"other"`

//向上跟踪连接是否处于活动状态
	Up bool `json:"up"`
//当连接被抓取拨号时注册
	initiated time.Time

	one   *Node
	other *Node
}

//nodes up返回两个节点当前是否都已启动
func (c *Conn) nodesUp() error {
	if !c.one.Up {
		return fmt.Errorf("one %v is not up", c.One)
	}
	if !c.other.Up {
		return fmt.Errorf("other %v is not up", c.Other)
	}
	return nil
}

//字符串返回日志友好的字符串
func (c *Conn) String() string {
	return fmt.Sprintf("Conn %v->%v", c.One.TerminalString(), c.Other.TerminalString())
}

//msg表示网络中两个节点之间发送的P2P消息
type Msg struct {
	One      enode.ID `json:"one"`
	Other    enode.ID `json:"other"`
	Protocol string   `json:"protocol"`
	Code     uint64   `json:"code"`
	Received bool     `json:"received"`
}

//字符串返回日志友好的字符串
func (m *Msg) String() string {
	return fmt.Sprintf("Msg(%d) %v->%v", m.Code, m.One.TerminalString(), m.Other.TerminalString())
}

//connlabel生成表示连接的确定字符串
//两个节点之间，用于比较两个连接是否相同
//结点
func ConnLabel(source, target enode.ID) string {
	var first, second enode.ID
	if bytes.Compare(source.Bytes(), target.Bytes()) > 0 {
		first = target
		second = source
	} else {
		first = source
		second = target
	}
	return fmt.Sprintf("%v-%v", first, second)
}

//快照表示网络在单个时间点的状态，可以
//用于恢复网络状态
type Snapshot struct {
	Nodes []NodeSnapshot `json:"nodes,omitempty"`
	Conns []Conn         `json:"conns,omitempty"`
}

//nodesnapshot表示网络中节点的状态
type NodeSnapshot struct {
	Node Node `json:"node,omitempty"`

//快照是从调用节点收集的任意数据。快照（）
	Snapshots map[string][]byte `json:"snapshots,omitempty"`
}

//快照创建网络快照
func (net *Network) Snapshot() (*Snapshot, error) {
	return net.snapshot(nil, nil)
}

func (net *Network) SnapshotWithServices(addServices []string, removeServices []string) (*Snapshot, error) {
	return net.snapshot(addServices, removeServices)
}

func (net *Network) snapshot(addServices []string, removeServices []string) (*Snapshot, error) {
	net.lock.Lock()
	defer net.lock.Unlock()
	snap := &Snapshot{
		Nodes: make([]NodeSnapshot, len(net.Nodes)),
	}
	for i, node := range net.Nodes {
		snap.Nodes[i] = NodeSnapshot{Node: *node}
		if !node.Up {
			continue
		}
		snapshots, err := node.Snapshots()
		if err != nil {
			return nil, err
		}
		snap.Nodes[i].Snapshots = snapshots
		for _, addSvc := range addServices {
			haveSvc := false
			for _, svc := range snap.Nodes[i].Node.Config.Services {
				if svc == addSvc {
					haveSvc = true
					break
				}
			}
			if !haveSvc {
				snap.Nodes[i].Node.Config.Services = append(snap.Nodes[i].Node.Config.Services, addSvc)
			}
		}
		if len(removeServices) > 0 {
			var cleanedServices []string
			for _, svc := range snap.Nodes[i].Node.Config.Services {
				haveSvc := false
				for _, rmSvc := range removeServices {
					if rmSvc == svc {
						haveSvc = true
						break
					}
				}
				if !haveSvc {
					cleanedServices = append(cleanedServices, svc)
				}

			}
			snap.Nodes[i].Node.Config.Services = cleanedServices
		}
	}
	for _, conn := range net.Conns {
		if conn.Up {
			snap.Conns = append(snap.Conns, *conn)
		}
	}
	return snap, nil
}

var snapshotLoadTimeout = 120 * time.Second

//加载加载网络快照
func (net *Network) Load(snap *Snapshot) error {
//启动节点。
	for _, n := range snap.Nodes {
		if _, err := net.NewNodeWithConfig(n.Node.Config); err != nil {
			return err
		}
		if !n.Node.Up {
			continue
		}
		if err := net.startWithSnapshots(n.Node.Config.ID, n.Snapshots); err != nil {
			return err
		}
	}

//准备连接事件计数器。
allConnected := make(chan struct{}) //建立所有连接时关闭
done := make(chan struct{})         //确保事件循环goroutine已终止
	defer close(done)

//订阅事件通道。
//它需要在事件循环Goroutine之外完成（创建于下面）
//以确保在进行连接调用之前事件通道被阻塞。
	events := make(chan *Event)
	sub := net.Events().Subscribe(events)
	defer sub.Unsubscribe()

	go func() {
//预期的连接数。
		total := len(snap.Conns)
//从快照建立的所有连接的集合，而不是其他连接。
//键数组元素0是连接一个字段值，元素1连接另一个字段。
		connections := make(map[[2]enode.ID]struct{}, total)

		for {
			select {
			case e := <-events:
//忽略控件事件，因为它们不表示
//连接或断开（向上）状态更改。
				if e.Control {
					continue
				}
//仅检测连接事件。
				if e.Type != EventTypeConn {
					continue
				}
				connection := [2]enode.ID{e.Conn.One, e.Conn.Other}
//节点仍然未连接或已断开连接。
				if !e.Conn.Up {
//从已建立的连接集中删除连接。
//这样可以防止断开时出现假阳性。
					delete(connections, connection)
					log.Warn("load snapshot: unexpected disconnection", "one", e.Conn.One, "other", e.Conn.Other)
					continue
				}
//检查连接是否来自快照。
				for _, conn := range snap.Conns {
					if conn.One == e.Conn.One && conn.Other == e.Conn.Other {
//将连接添加到已建立的连接集。
						connections[connection] = struct{}{}
						if len(connections) == total {
//所有节点都已连接的信号。
							close(allConnected)
							return
						}

						break
					}
				}
			case <-done:
//加载函数返回，终止此goroutine。
				return
			}
		}
	}()

//开始连接。
	for _, conn := range snap.Conns {

		if !net.GetNode(conn.One).Up || !net.GetNode(conn.Other).Up {
//在这种情况下，连接的至少一个节点没有启动，
//所以会导致快照“加载”失败
			continue
		}
		if err := net.Connect(conn.One, conn.Other); err != nil {
			return err
		}
	}

	select {
//等待快照的所有连接建立。
	case <-allConnected:
//确保我们不会永远等待。
	case <-time.After(snapshotLoadTimeout):
		return errors.New("snapshot connections not established")
	}
	return nil
}

//订阅从通道读取控制事件并执行它们
func (net *Network) Subscribe(events chan *Event) {
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			if event.Control {
				net.executeControlEvent(event)
			}
		case <-net.quitc:
			return
		}
	}
}

func (net *Network) executeControlEvent(event *Event) {
	log.Trace("Executing control event", "type", event.Type, "event", event)
	switch event.Type {
	case EventTypeNode:
		if err := net.executeNodeEvent(event); err != nil {
			log.Error("Error executing node event", "event", event, "err", err)
		}
	case EventTypeConn:
		if err := net.executeConnEvent(event); err != nil {
			log.Error("Error executing conn event", "event", event, "err", err)
		}
	case EventTypeMsg:
		log.Warn("Ignoring control msg event")
	}
}

func (net *Network) executeNodeEvent(e *Event) error {
	if !e.Node.Up {
		return net.Stop(e.Node.ID())
	}

	if _, err := net.NewNodeWithConfig(e.Node.Config); err != nil {
		return err
	}
	return net.Start(e.Node.ID())
}

func (net *Network) executeConnEvent(e *Event) error {
	if e.Conn.Up {
		return net.Connect(e.Conn.One, e.Conn.Other)
	} else {
		return net.Disconnect(e.Conn.One, e.Conn.Other)
	}
}
