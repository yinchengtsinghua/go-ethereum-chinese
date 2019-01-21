
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

package adapters

import (
	"errors"
	"fmt"
	"math"
	"net"
	"sync"

	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations/pipes"
	"github.com/ethereum/go-ethereum/rpc"
)

//Simadapter是一个节点适配器，用于创建内存中的模拟节点和
//使用net.pipe连接它们
type SimAdapter struct {
	pipe     func() (net.Conn, net.Conn, error)
	mtx      sync.RWMutex
	nodes    map[enode.ID]*SimNode
	services map[string]ServiceFunc
}

//newsimadapter创建一个能够在内存中运行的simadapter
//运行任何给定服务（运行在
//特定节点将传递给nodeconfig中的newnode函数）
//适配器使用net.pipe进行内存中模拟的网络连接
func NewSimAdapter(services map[string]ServiceFunc) *SimAdapter {
	return &SimAdapter{
		pipe:     pipes.NetPipe,
		nodes:    make(map[enode.ID]*SimNode),
		services: services,
	}
}

func NewTCPAdapter(services map[string]ServiceFunc) *SimAdapter {
	return &SimAdapter{
		pipe:     pipes.TCPPipe,
		nodes:    make(map[enode.ID]*SimNode),
		services: services,
	}
}

//name返回用于日志记录的适配器的名称
func (s *SimAdapter) Name() string {
	return "sim-adapter"
}

//newnode使用给定的配置返回新的simnode
func (s *SimAdapter) NewNode(config *NodeConfig) (Node, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

//检查ID为的节点是否已存在
	id := config.ID
	if _, exists := s.nodes[id]; exists {
		return nil, fmt.Errorf("node already exists: %s", id)
	}

//检查服务是否有效
	if len(config.Services) == 0 {
		return nil, errors.New("node must have at least one service")
	}
	for _, service := range config.Services {
		if _, exists := s.services[service]; !exists {
			return nil, fmt.Errorf("unknown node service %q", service)
		}
	}

	n, err := node.New(&node.Config{
		P2P: p2p.Config{
			PrivateKey:      config.PrivateKey,
			MaxPeers:        math.MaxInt32,
			NoDiscovery:     true,
			Dialer:          s,
			EnableMsgEvents: config.EnableMsgEvents,
		},
		NoUSB:  true,
		Logger: log.New("node.id", id.String()),
	})
	if err != nil {
		return nil, err
	}

	simNode := &SimNode{
		ID:      id,
		config:  config,
		node:    n,
		adapter: s,
		running: make(map[string]node.Service),
	}
	s.nodes[id] = simNode
	return simNode, nil
}

//拨号通过使用连接到节点来实现p2p.nodeadialer接口。
//内存中的net.pipe
func (s *SimAdapter) Dial(dest *enode.Node) (conn net.Conn, err error) {
	node, ok := s.GetNode(dest.ID())
	if !ok {
		return nil, fmt.Errorf("unknown node: %s", dest.ID())
	}
	srv := node.Server()
	if srv == nil {
		return nil, fmt.Errorf("node not running: %s", dest.ID())
	}
//simadapter.pipe是net.pipe（newsimadapter）
	pipe1, pipe2, err := s.pipe()
	if err != nil {
		return nil, err
	}
//这是模拟的“倾听”
//异步调用拨号目标节点的P2P服务器
//在“监听”端建立连接
	go srv.SetupConn(pipe1, 0, nil)
	return pipe2, nil
}

//dialrpc通过创建内存中的rpc来实现rpcdialer接口
//给定节点的客户端
func (s *SimAdapter) DialRPC(id enode.ID) (*rpc.Client, error) {
	node, ok := s.GetNode(id)
	if !ok {
		return nil, fmt.Errorf("unknown node: %s", id)
	}
	handler, err := node.node.RPCHandler()
	if err != nil {
		return nil, err
	}
	return rpc.DialInProc(handler), nil
}

//getnode返回具有给定ID的节点（如果存在）
func (s *SimAdapter) GetNode(id enode.ID) (*SimNode, bool) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	node, ok := s.nodes[id]
	return node, ok
}

//Simnode是一个内存中的模拟节点，它使用
//net.pipe（参见simadapter.dial），直接在上面运行devp2p协议
//管
type SimNode struct {
	lock         sync.RWMutex
	ID           enode.ID
	config       *NodeConfig
	adapter      *SimAdapter
	node         *node.Node
	running      map[string]node.Service
	client       *rpc.Client
	registerOnce sync.Once
}

//addr返回节点的发现地址
func (sn *SimNode) Addr() []byte {
	return []byte(sn.Node().String())
}

//node返回表示simnode的节点描述符
func (sn *SimNode) Node() *enode.Node {
	return sn.config.Node()
}

//客户端返回一个rpc.client，可用于与
//基础服务（节点启动后设置）
func (sn *SimNode) Client() (*rpc.Client, error) {
	sn.lock.RLock()
	defer sn.lock.RUnlock()
	if sn.client == nil {
		return nil, errors.New("node not started")
	}
	return sn.client, nil
}

//serverpc通过创建一个
//节点的RPC服务器的内存中客户端
func (sn *SimNode) ServeRPC(conn net.Conn) error {
	handler, err := sn.node.RPCHandler()
	if err != nil {
		return err
	}
	handler.ServeCodec(rpc.NewJSONCodec(conn), rpc.OptionMethodInvocation|rpc.OptionSubscriptions)
	return nil
}

//快照通过调用
//模拟快照RPC方法
func (sn *SimNode) Snapshots() (map[string][]byte, error) {
	sn.lock.RLock()
	services := make(map[string]node.Service, len(sn.running))
	for name, service := range sn.running {
		services[name] = service
	}
	sn.lock.RUnlock()
	if len(services) == 0 {
		return nil, errors.New("no running services")
	}
	snapshots := make(map[string][]byte)
	for name, service := range services {
		if s, ok := service.(interface {
			Snapshot() ([]byte, error)
		}); ok {
			snap, err := s.Snapshot()
			if err != nil {
				return nil, err
			}
			snapshots[name] = snap
		}
	}
	return snapshots, nil
}

//start注册服务并启动底层devp2p节点
func (sn *SimNode) Start(snapshots map[string][]byte) error {
	newService := func(name string) func(ctx *node.ServiceContext) (node.Service, error) {
		return func(nodeCtx *node.ServiceContext) (node.Service, error) {
			ctx := &ServiceContext{
				RPCDialer:   sn.adapter,
				NodeContext: nodeCtx,
				Config:      sn.config,
			}
			if snapshots != nil {
				ctx.Snapshot = snapshots[name]
			}
			serviceFunc := sn.adapter.services[name]
			service, err := serviceFunc(ctx)
			if err != nil {
				return nil, err
			}
			sn.running[name] = service
			return service, nil
		}
	}

//确保在节点的情况下只注册一次服务
//停止然后重新启动
	var regErr error
	sn.registerOnce.Do(func() {
		for _, name := range sn.config.Services {
			if err := sn.node.Register(newService(name)); err != nil {
				regErr = err
				break
			}
		}
	})
	if regErr != nil {
		return regErr
	}

	if err := sn.node.Start(); err != nil {
		return err
	}

//创建进程内RPC客户端
	handler, err := sn.node.RPCHandler()
	if err != nil {
		return err
	}

	sn.lock.Lock()
	sn.client = rpc.DialInProc(handler)
	sn.lock.Unlock()

	return nil
}

//stop关闭RPC客户端并停止底层devp2p节点
func (sn *SimNode) Stop() error {
	sn.lock.Lock()
	if sn.client != nil {
		sn.client.Close()
		sn.client = nil
	}
	sn.lock.Unlock()
	return sn.node.Stop()
}

//服务按名称返回正在运行的服务
func (sn *SimNode) Service(name string) node.Service {
	sn.lock.RLock()
	defer sn.lock.RUnlock()
	return sn.running[name]
}

//服务返回基础服务的副本
func (sn *SimNode) Services() []node.Service {
	sn.lock.RLock()
	defer sn.lock.RUnlock()
	services := make([]node.Service, 0, len(sn.running))
	for _, service := range sn.running {
		services = append(services, service)
	}
	return services
}

//ServiceMap按基础服务的名称返回映射
func (sn *SimNode) ServiceMap() map[string]node.Service {
	sn.lock.RLock()
	defer sn.lock.RUnlock()
	services := make(map[string]node.Service, len(sn.running))
	for name, service := range sn.running {
		services[name] = service
	}
	return services
}

//服务器返回基础p2p.server
func (sn *SimNode) Server() *p2p.Server {
	return sn.node.Server()
}

//subscribeEvents订阅来自
//底层p2p.server
func (sn *SimNode) SubscribeEvents(ch chan *p2p.PeerEvent) event.Subscription {
	srv := sn.Server()
	if srv == nil {
		panic("node not running")
	}
	return srv.SubscribeEvents(ch)
}

//nodeinfo返回有关节点的信息
func (sn *SimNode) NodeInfo() *p2p.NodeInfo {
	server := sn.Server()
	if server == nil {
		return &p2p.NodeInfo{
			ID:    sn.ID.String(),
			Enode: sn.Node().String(),
		}
	}
	return server.NodeInfo()
}
