
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Go Ethereum作者
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

package node

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/internal/debug"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/prometheus/prometheus/util/flock"
)

//节点是一个可以在其上注册服务的容器。
type Node struct {
eventmux *event.TypeMux //在堆栈服务之间使用的事件多路复用器
	config   *Config
	accman   *accounts.Manager

ephemeralKeystore string         //如果非空，则停止将删除的密钥目录
instanceDirLock   flock.Releaser //防止同时使用实例目录

	serverConfig p2p.Config
server       *p2p.Server //当前运行的P2P网络层

serviceFuncs []ServiceConstructor     //服务构造函数（按依赖顺序）
services     map[reflect.Type]Service //当前正在运行的服务

rpcAPIs       []rpc.API   //节点当前提供的API列表
inprocHandler *rpc.Server //处理API请求的进程内RPC请求处理程序

ipcEndpoint string       //要侦听的IPC终结点（空=禁用IPC）
ipcListener net.Listener //用于服务API请求的IPC RPC侦听器套接字
ipcHandler  *rpc.Server  //用于处理API请求的IPC RPC请求处理程序

httpEndpoint  string       //要侦听的HTTP端点（接口+端口）（空=禁用HTTP）
httpWhitelist []string     //允许通过此终结点的HTTP RPC模块
httpListener  net.Listener //HTTP RPC侦听器套接字到服务器API请求
httpHandler   *rpc.Server  //处理API请求的HTTP RPC请求处理程序

wsEndpoint string       //要侦听的WebSocket终结点（接口+端口）（空=禁用WebSocket）
wsListener net.Listener //WebSocket RPC侦听器套接字到服务器API请求
wsHandler  *rpc.Server  //用于处理API请求的WebSocket RPC请求处理程序

stop chan struct{} //等待终止通知的通道
	lock sync.RWMutex

	log log.Logger
}

//new创建一个新的p2p节点，准备好进行协议注册。
func New(conf *Config) (*Node, error) {
//复制config并解析datadir，以便将来对当前
//工作目录不影响节点。
	confCopy := *conf
	conf = &confCopy
	if conf.DataDir != "" {
		absdatadir, err := filepath.Abs(conf.DataDir)
		if err != nil {
			return nil, err
		}
		conf.DataDir = absdatadir
	}
//确保实例名不会与
//数据目录中的其他文件。
	if strings.ContainsAny(conf.Name, `/\`) {
		return nil, errors.New(`Config.Name must not contain '/' or '\'`)
	}
	if conf.Name == datadirDefaultKeyStore {
		return nil, errors.New(`Config.Name cannot be "` + datadirDefaultKeyStore + `"`)
	}
	if strings.HasSuffix(conf.Name, ".ipc") {
		return nil, errors.New(`Config.Name cannot end in ".ipc"`)
	}
//确保AccountManager方法在节点启动之前工作。
//我们在命令/geth中依赖这个。
	am, ephemeralKeystore, err := makeAccountManager(conf)
	if err != nil {
		return nil, err
	}
	if conf.Logger == nil {
		conf.Logger = log.New()
	}
//注意：与config的任何交互都会创建/触摸文件
//在数据目录或实例目录中延迟到启动。
	return &Node{
		accman:            am,
		ephemeralKeystore: ephemeralKeystore,
		config:            conf,
		serviceFuncs:      []ServiceConstructor{},
		ipcEndpoint:       conf.IPCEndpoint(),
		httpEndpoint:      conf.HTTPEndpoint(),
		wsEndpoint:        conf.WSEndpoint(),
		eventmux:          new(event.TypeMux),
		log:               conf.Logger,
	}, nil
}

//寄存器将一个新服务注入到节点的堆栈中。服务创建者
//传递的构造函数在其类型中对于同级构造函数必须是唯一的。
func (n *Node) Register(constructor ServiceConstructor) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.server != nil {
		return ErrNodeRunning
	}
	n.serviceFuncs = append(n.serviceFuncs, constructor)
	return nil
}

//开始创建一个活动的P2P节点并开始运行它。
func (n *Node) Start() error {
	n.lock.Lock()
	defer n.lock.Unlock()

//如果节点已经运行，则短路
	if n.server != nil {
		return ErrNodeRunning
	}
	if err := n.openDataDir(); err != nil {
		return err
	}

//初始化P2P服务器。这将创建节点键并
//发现数据库。
	n.serverConfig = n.config.P2P
	n.serverConfig.PrivateKey = n.config.NodeKey()
	n.serverConfig.Name = n.config.NodeName()
	n.serverConfig.Logger = n.log
	if n.serverConfig.StaticNodes == nil {
		n.serverConfig.StaticNodes = n.config.StaticNodes()
	}
	if n.serverConfig.TrustedNodes == nil {
		n.serverConfig.TrustedNodes = n.config.TrustedNodes()
	}
	if n.serverConfig.NodeDatabase == "" {
		n.serverConfig.NodeDatabase = n.config.NodeDB()
	}
	running := &p2p.Server{Config: n.serverConfig}
	n.log.Info("Starting peer-to-peer node", "instance", n.serverConfig.Name)

//否则，复制并专门化p2p配置
	services := make(map[reflect.Type]Service)
	for _, constructor := range n.serviceFuncs {
//为特定服务创建新上下文
		ctx := &ServiceContext{
			config:         n.config,
			services:       make(map[reflect.Type]Service),
			EventMux:       n.eventmux,
			AccountManager: n.accman,
		}
for kind, s := range services { //线程访问所需的副本
			ctx.services[kind] = s
		}
//构建并保存服务
		service, err := constructor(ctx)
		if err != nil {
			return err
		}
		kind := reflect.TypeOf(service)
		if _, exists := services[kind]; exists {
			return &DuplicateServiceError{Kind: kind}
		}
		services[kind] = service
	}
//收集协议并启动新组装的P2P服务器
	for _, service := range services {
		running.Protocols = append(running.Protocols, service.Protocols()...)
	}
	if err := running.Start(); err != nil {
		return convertFileLockError(err)
	}
//启动每个服务
	started := []reflect.Type{}
	for kind, service := range services {
//启动下一个服务，失败时停止所有上一个服务
		if err := service.Start(running); err != nil {
			for _, kind := range started {
				services[kind].Stop()
			}
			running.Stop()

			return err
		}
//标记服务已启动以进行潜在清理
		started = append(started, kind)
	}
//最后启动配置的RPC接口
	if err := n.startRPC(services); err != nil {
		for _, service := range services {
			service.Stop()
		}
		running.Stop()
		return err
	}
//完成初始化启动
	n.services = services
	n.server = running
	n.stop = make(chan struct{})

	return nil
}

func (n *Node) openDataDir() error {
	if n.config.DataDir == "" {
return nil //短暂的
	}

	instdir := filepath.Join(n.config.DataDir, n.config.name())
	if err := os.MkdirAll(instdir, 0700); err != nil {
		return err
	}
//锁定实例目录以防止另一个实例同时使用
//意外地将实例目录用作数据库。
	release, _, err := flock.New(filepath.Join(instdir, "LOCK"))
	if err != nil {
		return convertFileLockError(err)
	}
	n.instanceDirLock = release
	return nil
}

//start rpc是一个帮助方法，用于在节点期间启动所有不同的rpc端点。
//启动。以后任何时候都不应该打电话，因为它可以确定
//关于节点状态的假设。
func (n *Node) startRPC(services map[reflect.Type]Service) error {
//收集所有可能的API到表面
	apis := n.apis()
	for _, service := range services {
		apis = append(apis, service.APIs()...)
	}
//启动各种API端点，在出现错误时终止所有端点
	if err := n.startInProc(apis); err != nil {
		return err
	}
	if err := n.startIPC(apis); err != nil {
		n.stopInProc()
		return err
	}
	if err := n.startHTTP(n.httpEndpoint, apis, n.config.HTTPModules, n.config.HTTPCors, n.config.HTTPVirtualHosts, n.config.HTTPTimeouts); err != nil {
		n.stopIPC()
		n.stopInProc()
		return err
	}
	if err := n.startWS(n.wsEndpoint, apis, n.config.WSModules, n.config.WSOrigins, n.config.WSExposeAll); err != nil {
		n.stopHTTP()
		n.stopIPC()
		n.stopInProc()
		return err
	}
//所有API终结点已成功启动
	n.rpcAPIs = apis
	return nil
}

//StartInProc初始化进程内RPC终结点。
func (n *Node) startInProc(apis []rpc.API) error {
//注册服务公开的所有API
	handler := rpc.NewServer()
	for _, api := range apis {
		if err := handler.RegisterName(api.Namespace, api.Service); err != nil {
			return err
		}
		n.log.Debug("InProc registered", "namespace", api.Namespace)
	}
	n.inprocHandler = handler
	return nil
}

//stopinproc终止进程内RPC终结点。
func (n *Node) stopInProc() {
	if n.inprocHandler != nil {
		n.inprocHandler.Stop()
		n.inprocHandler = nil
	}
}

//StartIPC初始化并启动IPC RPC终结点。
func (n *Node) startIPC(apis []rpc.API) error {
	if n.ipcEndpoint == "" {
return nil //IPC禁用。
	}
	listener, handler, err := rpc.StartIPCEndpoint(n.ipcEndpoint, apis)
	if err != nil {
		return err
	}
	n.ipcListener = listener
	n.ipcHandler = handler
	n.log.Info("IPC endpoint opened", "url", n.ipcEndpoint)
	return nil
}

//StopIPC终止IPC RPC终结点。
func (n *Node) stopIPC() {
	if n.ipcListener != nil {
		n.ipcListener.Close()
		n.ipcListener = nil

		n.log.Info("IPC endpoint closed", "url", n.ipcEndpoint)
	}
	if n.ipcHandler != nil {
		n.ipcHandler.Stop()
		n.ipcHandler = nil
	}
}

//StartHTTP初始化并启动HTTP RPC终结点。
func (n *Node) startHTTP(endpoint string, apis []rpc.API, modules []string, cors []string, vhosts []string, timeouts rpc.HTTPTimeouts) error {
//如果HTTP端点未暴露，则短路
	if endpoint == "" {
		return nil
	}
	listener, handler, err := rpc.StartHTTPEndpoint(endpoint, apis, modules, cors, vhosts, timeouts)
	if err != nil {
		return err
	}
n.log.Info("HTTP endpoint opened", "url", fmt.Sprintf("http://%s“，终结点），”cors“，strings.join（cors，”，“），”vhosts“，strings.join（vhosts，”，“））
//所有侦听器都已成功启动
	n.httpEndpoint = endpoint
	n.httpListener = listener
	n.httpHandler = handler

	return nil
}

//StopHTTP终止HTTP RPC终结点。
func (n *Node) stopHTTP() {
	if n.httpListener != nil {
		n.httpListener.Close()
		n.httpListener = nil

n.log.Info("HTTP endpoint closed", "url", fmt.Sprintf("http://%s“，n.httpendpoint）
	}
	if n.httpHandler != nil {
		n.httpHandler.Stop()
		n.httpHandler = nil
	}
}

//startws初始化并启动WebSocket RPC终结点。
func (n *Node) startWS(endpoint string, apis []rpc.API, modules []string, wsOrigins []string, exposeAll bool) error {
//如果没有暴露WS端点，则短路
	if endpoint == "" {
		return nil
	}
	listener, handler, err := rpc.StartWSEndpoint(endpoint, apis, modules, wsOrigins, exposeAll)
	if err != nil {
		return err
	}
n.log.Info("WebSocket endpoint opened", "url", fmt.Sprintf("ws://%s“，listener.addr（）））
//所有侦听器都已成功启动
	n.wsEndpoint = endpoint
	n.wsListener = listener
	n.wsHandler = handler

	return nil
}

//stopws终止WebSocket RPC终结点。
func (n *Node) stopWS() {
	if n.wsListener != nil {
		n.wsListener.Close()
		n.wsListener = nil

n.log.Info("WebSocket endpoint closed", "url", fmt.Sprintf("ws://%s“，n.wsendpoint）
	}
	if n.wsHandler != nil {
		n.wsHandler.Stop()
		n.wsHandler = nil
	}
}

//stop终止正在运行的节点及其所有服务。节点中
//未启动，返回错误。
func (n *Node) Stop() error {
	n.lock.Lock()
	defer n.lock.Unlock()

//节点未运行时短路
	if n.server == nil {
		return ErrNodeStopped
	}

//终止API、服务和P2P服务器。
	n.stopWS()
	n.stopHTTP()
	n.stopIPC()
	n.rpcAPIs = nil
	failure := &StopError{
		Services: make(map[reflect.Type]error),
	}
	for kind, service := range n.services {
		if err := service.Stop(); err != nil {
			failure.Services[kind] = err
		}
	}
	n.server.Stop()
	n.services = nil
	n.server = nil

//释放实例目录锁。
	if n.instanceDirLock != nil {
		if err := n.instanceDirLock.Release(); err != nil {
			n.log.Error("Can't release datadir lock", "err", err)
		}
		n.instanceDirLock = nil
	}

//等待，等待
	close(n.stop)

//如果密钥库是临时创建的，请将其删除。
	var keystoreErr error
	if n.ephemeralKeystore != "" {
		keystoreErr = os.RemoveAll(n.ephemeralKeystore)
	}

	if len(failure.Services) > 0 {
		return failure
	}
	if keystoreErr != nil {
		return keystoreErr
	}
	return nil
}

//等待将阻止线程，直到节点停止。如果节点没有运行
//调用时，该方法立即返回。
func (n *Node) Wait() {
	n.lock.RLock()
	if n.server == nil {
		n.lock.RUnlock()
		return
	}
	stop := n.stop
	n.lock.RUnlock()

	<-stop
}

//重新启动终止正在运行的节点，并在其位置启动新节点。如果
//节点未运行，返回错误。
func (n *Node) Restart() error {
	if err := n.Stop(); err != nil {
		return err
	}
	if err := n.Start(); err != nil {
		return err
	}
	return nil
}

//附加创建一个附加到进程内API处理程序的RPC客户端。
func (n *Node) Attach() (*rpc.Client, error) {
	n.lock.RLock()
	defer n.lock.RUnlock()

	if n.server == nil {
		return nil, ErrNodeStopped
	}
	return rpc.DialInProc(n.inprocHandler), nil
}

//rpc handler返回进程内的rpc请求处理程序。
func (n *Node) RPCHandler() (*rpc.Server, error) {
	n.lock.RLock()
	defer n.lock.RUnlock()

	if n.inprocHandler == nil {
		return nil, ErrNodeStopped
	}
	return n.inprocHandler, nil
}

//服务器检索当前运行的P2P网络层。这个方法的意思是
//仅检查当前运行的服务器的字段，生命周期管理
//应该留给这个节点实体。
func (n *Node) Server() *p2p.Server {
	n.lock.RLock()
	defer n.lock.RUnlock()

	return n.server
}

//服务检索当前正在运行的特定类型的注册服务。
func (n *Node) Service(service interface{}) error {
	n.lock.RLock()
	defer n.lock.RUnlock()

//节点未运行时短路
	if n.server == nil {
		return ErrNodeStopped
	}
//否则，请尝试查找要返回的服务
	element := reflect.ValueOf(service).Elem()
	if running, ok := n.services[element.Type()]; ok {
		element.Set(reflect.ValueOf(running))
		return nil
	}
	return ErrServiceUnknown
}

//datadir检索协议堆栈使用的当前datadir。
//已弃用：此目录中不应存储任何文件，请改用instancedir。
func (n *Node) DataDir() string {
	return n.config.DataDir
}

//instancedir检索协议堆栈使用的实例目录。
func (n *Node) InstanceDir() string {
	return n.config.instanceDir()
}

//AccountManager检索协议堆栈使用的帐户管理器。
func (n *Node) AccountManager() *accounts.Manager {
	return n.accman
}

//ipc endpoint检索协议堆栈使用的当前IPC终结点。
func (n *Node) IPCEndpoint() string {
	return n.ipcEndpoint
}

//http endpoint检索协议堆栈使用的当前HTTP端点。
func (n *Node) HTTPEndpoint() string {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.httpListener != nil {
		return n.httpListener.Addr().String()
	}
	return n.httpEndpoint
}

//ws endpoint检索协议堆栈使用的当前ws endpoint。
func (n *Node) WSEndpoint() string {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.wsListener != nil {
		return n.wsListener.Addr().String()
	}
	return n.wsEndpoint
}

//eventmux检索中所有网络服务使用的事件多路复用器
//当前协议堆栈。
func (n *Node) EventMux() *event.TypeMux {
	return n.eventmux
}

//opendatabase打开具有给定名称的现有数据库（如果没有，则创建一个）
//可以在节点的实例目录中找到上一个）。如果节点是
//短暂的，返回内存数据库。
func (n *Node) OpenDatabase(name string, cache, handles int) (ethdb.Database, error) {
	if n.config.DataDir == "" {
		return ethdb.NewMemDatabase(), nil
	}
	return ethdb.NewLDBDatabase(n.config.ResolvePath(name), cache, handles)
}

//resolvepath返回实例目录中资源的绝对路径。
func (n *Node) ResolvePath(x string) string {
	return n.config.ResolvePath(x)
}

//API返回此节点提供的RPC描述符的集合。
func (n *Node) apis() []rpc.API {
	return []rpc.API{
		{
			Namespace: "admin",
			Version:   "1.0",
			Service:   NewPrivateAdminAPI(n),
		}, {
			Namespace: "admin",
			Version:   "1.0",
			Service:   NewPublicAdminAPI(n),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   debug.Handler,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPublicDebugAPI(n),
			Public:    true,
		}, {
			Namespace: "web3",
			Version:   "1.0",
			Service:   NewPublicWeb3API(n),
			Public:    true,
		},
	}
}
