
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
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/reexec"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/net/websocket"
)

func init() {
//当当前二进制文件为
//作为“p2p节点”执行（而不是主（）函数通常执行的任何操作）。
	reexec.Register("p2p-node", execP2PNode)
}

//execadapter是一个节点适配器，通过执行当前二进制文件来运行模拟节点。
//作为一个孩子的过程。
type ExecAdapter struct {
//basedir是每个目录下的数据目录
//创建模拟节点。
	BaseDir string

	nodes map[enode.ID]*ExecNode
}

//newexecadapter返回一个execadapter，该execadapter将节点数据存储在
//给定基目录的子目录
func NewExecAdapter(baseDir string) *ExecAdapter {
	return &ExecAdapter{
		BaseDir: baseDir,
		nodes:   make(map[enode.ID]*ExecNode),
	}
}

//name返回用于日志记录的适配器的名称
func (e *ExecAdapter) Name() string {
	return "exec-adapter"
}

//newnode使用给定的配置返回新的execnode
func (e *ExecAdapter) NewNode(config *NodeConfig) (Node, error) {
	if len(config.Services) == 0 {
		return nil, errors.New("node must have at least one service")
	}
	for _, service := range config.Services {
		if _, exists := serviceFuncs[service]; !exists {
			return nil, fmt.Errorf("unknown node service %q", service)
		}
	}

//使用ID的前12个字符创建节点目录
//因为unix套接字路径不能超过256个字符
	dir := filepath.Join(e.BaseDir, config.ID.String()[:12])
	if err := os.Mkdir(dir, 0755); err != nil {
		return nil, fmt.Errorf("error creating node directory: %s", err)
	}

//生成配置
	conf := &execNodeConfig{
		Stack: node.DefaultConfig,
		Node:  config,
	}
	conf.Stack.DataDir = filepath.Join(dir, "data")
	conf.Stack.WSHost = "127.0.0.1"
	conf.Stack.WSPort = 0
	conf.Stack.WSOrigins = []string{"*"}
	conf.Stack.WSExposeAll = true
	conf.Stack.P2P.EnableMsgEvents = false
	conf.Stack.P2P.NoDiscovery = true
	conf.Stack.P2P.NAT = nil
	conf.Stack.NoUSB = true

//监听本地主机端口，当我们
//初始化nodeconfig（通常是随机端口）
	conf.Stack.P2P.ListenAddr = fmt.Sprintf(":%d", config.Port)

	node := &ExecNode{
		ID:      config.ID,
		Dir:     dir,
		Config:  conf,
		adapter: e,
	}
	node.newCmd = node.execCommand
	e.nodes[node.ID] = node
	return node, nil
}

//exec node通过执行当前二进制文件和
//运行配置的服务
type ExecNode struct {
	ID     enode.ID
	Dir    string
	Config *execNodeConfig
	Cmd    *exec.Cmd
	Info   *p2p.NodeInfo

	adapter *ExecAdapter
	client  *rpc.Client
	wsAddr  string
	newCmd  func() *exec.Cmd
	key     *ecdsa.PrivateKey
}

//addr返回节点的enode url
func (n *ExecNode) Addr() []byte {
	if n.Info == nil {
		return nil
	}
	return []byte(n.Info.Enode)
}

//客户端返回一个rpc.client，可用于与
//基础服务（节点启动后设置）
func (n *ExecNode) Client() (*rpc.Client, error) {
	return n.client, nil
}

//start exec是将ID和服务作为命令行参数传递的节点
//节点配置在环境变量中编码为JSON。
func (n *ExecNode) Start(snapshots map[string][]byte) (err error) {
	if n.Cmd != nil {
		return errors.New("already started")
	}
	defer func() {
		if err != nil {
			n.Stop()
		}
	}()

//对包含快照的配置副本进行编码
	confCopy := *n.Config
	confCopy.Snapshots = snapshots
	confCopy.PeerAddrs = make(map[string]string)
	for id, node := range n.adapter.nodes {
		confCopy.PeerAddrs[id.String()] = node.wsAddr
	}
	confData, err := json.Marshal(confCopy)
	if err != nil {
		return fmt.Errorf("error generating node config: %s", err)
	}

//启动等待启动信息的一次性服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	statusURL, statusC := n.waitForStartupJSON(ctx)

//启动节点
	cmd := n.newCmd()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		envStatusURL+"="+statusURL,
		envNodeConfig+"="+string(confData),
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting node: %s", err)
	}
	n.Cmd = cmd

//从stderr日志中读取websocket地址
	status := <-statusC
	if status.Err != "" {
		return errors.New(status.Err)
	}
client, err := rpc.DialWebsocket(ctx, status.WSEndpoint, "http://“本地主机”
	if err != nil {
		return fmt.Errorf("can't connect to RPC server: %v", err)
	}

//节点准备好：
	n.client = client
	n.wsAddr = status.WSEndpoint
	n.Info = status.NodeInfo
	return nil
}

//WaitForStartupJSON运行一个一次性HTTP服务器来接收启动报告。
func (n *ExecNode) waitForStartupJSON(ctx context.Context) (string, chan nodeStartupJSON) {
	var (
		ch       = make(chan nodeStartupJSON, 1)
		quitOnce sync.Once
		srv      http.Server
	)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		ch <- nodeStartupJSON{Err: err.Error()}
		return "", ch
	}
	quit := func(status nodeStartupJSON) {
		quitOnce.Do(func() {
			l.Close()
			ch <- status
		})
	}
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var status nodeStartupJSON
		if err := json.NewDecoder(r.Body).Decode(&status); err != nil {
			status.Err = fmt.Sprintf("can't decode startup report: %v", err)
		}
		quit(status)
	})
//运行HTTP服务器，但不要一直等待并关闭它
//如果上下文被取消。
	go srv.Serve(l)
	go func() {
		<-ctx.Done()
		quit(nodeStartupJSON{Err: "didn't get startup report"})
	}()

url := "http://“+l.addr（）.string（）”
	return url, ch
}

//exec command返回一个命令，该命令通过exeng在本地运行节点
//当前二进制文件，但将argv[0]设置为“p2p node”，以便子级
//运行execp2pnode
func (n *ExecNode) execCommand() *exec.Cmd {
	return &exec.Cmd{
		Path: reexec.Self(),
		Args: []string{"p2p-node", strings.Join(n.Config.Node.Services, ","), n.ID.String()},
	}
}

//stop首先发送sigterm，然后在节点
//在5秒内没有停止
func (n *ExecNode) Stop() error {
	if n.Cmd == nil {
		return nil
	}
	defer func() {
		n.Cmd = nil
	}()

	if n.client != nil {
		n.client.Close()
		n.client = nil
		n.wsAddr = ""
		n.Info = nil
	}

	if err := n.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return n.Cmd.Process.Kill()
	}
	waitErr := make(chan error)
	go func() {
		waitErr <- n.Cmd.Wait()
	}()
	select {
	case err := <-waitErr:
		return err
	case <-time.After(5 * time.Second):
		return n.Cmd.Process.Kill()
	}
}

//nodeinfo返回有关节点的信息
func (n *ExecNode) NodeInfo() *p2p.NodeInfo {
	info := &p2p.NodeInfo{
		ID: n.ID.String(),
	}
	if n.client != nil {
		n.client.Call(&info, "admin_nodeInfo")
	}
	return info
}

//serverpc通过拨
//节点的WebSocket地址和连接两个连接
func (n *ExecNode) ServeRPC(clientConn net.Conn) error {
conn, err := websocket.Dial(n.wsAddr, "", "http://“本地主机”
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	wg.Add(2)
	join := func(src, dst net.Conn) {
		defer wg.Done()
		io.Copy(dst, src)
//关闭目标连接的写入端
		if cw, ok := dst.(interface {
			CloseWrite() error
		}); ok {
			cw.CloseWrite()
		} else {
			dst.Close()
		}
	}
	go join(conn, clientConn)
	go join(clientConn, conn)
	wg.Wait()
	return nil
}

//快照通过调用
//模拟快照RPC方法
func (n *ExecNode) Snapshots() (map[string][]byte, error) {
	if n.client == nil {
		return nil, errors.New("RPC not started")
	}
	var snapshots map[string][]byte
	return snapshots, n.client.Call(&snapshots, "simulation_snapshot")
}

//ExecOnDeconfig用于序列化节点配置，以便
//作为JSON编码的环境变量传递给子进程
type execNodeConfig struct {
	Stack     node.Config       `json:"stack"`
	Node      *NodeConfig       `json:"node"`
	Snapshots map[string][]byte `json:"snapshots,omitempty"`
	PeerAddrs map[string]string `json:"peer_addrs,omitempty"`
}

//执行当前二进制文件时，execp2pnode启动模拟节点
//argv[0]为“p2p节点”，从argv[1]/argv[2]读取服务/id
//以及环境变量中的节点配置。
func execP2PNode() {
	glogger := log.NewGlogHandler(log.StreamHandler(os.Stderr, log.LogfmtFormat()))
	glogger.Verbosity(log.LvlInfo)
	log.Root().SetHandler(glogger)
	statusURL := os.Getenv(envStatusURL)
	if statusURL == "" {
		log.Crit("missing " + envStatusURL)
	}

//启动节点并收集启动报告。
	var status nodeStartupJSON
	stack, stackErr := startExecNodeStack()
	if stackErr != nil {
		status.Err = stackErr.Error()
	} else {
status.WSEndpoint = "ws://“+stack.wsendpoint（）”
		status.NodeInfo = stack.Server().NodeInfo()
	}

//向主机发送状态。
	statusJSON, _ := json.Marshal(status)
	if _, err := http.Post(statusURL, "application/json", bytes.NewReader(statusJSON)); err != nil {
		log.Crit("Can't post startup info", "url", statusURL, "err", err)
	}
	if stackErr != nil {
		os.Exit(1)
	}

//如果我们得到一个sigterm信号，就停止堆栈。
	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGTERM)
		defer signal.Stop(sigc)
		<-sigc
		log.Info("Received SIGTERM, shutting down...")
		stack.Stop()
	}()
stack.Wait() //等待堆栈退出。
}

func startExecNodeStack() (*node.Node, error) {
//从argv读取服务
	serviceNames := strings.Split(os.Args[1], ",")

//解码配置
	confEnv := os.Getenv(envNodeConfig)
	if confEnv == "" {
		return nil, fmt.Errorf("missing " + envNodeConfig)
	}
	var conf execNodeConfig
	if err := json.Unmarshal([]byte(confEnv), &conf); err != nil {
		return nil, fmt.Errorf("error decoding %s: %v", envNodeConfig, err)
	}
	conf.Stack.P2P.PrivateKey = conf.Node.PrivateKey
	conf.Stack.Logger = log.New("node.id", conf.Node.ID.String())

//初始化devp2p堆栈
	stack, err := node.New(&conf.Stack)
	if err != nil {
		return nil, fmt.Errorf("error creating node stack: %v", err)
	}

//注册服务，将它们收集到地图中，以便我们可以包装
//它们在快照服务中
	services := make(map[string]node.Service, len(serviceNames))
	for _, name := range serviceNames {
		serviceFunc, exists := serviceFuncs[name]
		if !exists {
			return nil, fmt.Errorf("unknown node service %q", err)
		}
		constructor := func(nodeCtx *node.ServiceContext) (node.Service, error) {
			ctx := &ServiceContext{
				RPCDialer:   &wsRPCDialer{addrs: conf.PeerAddrs},
				NodeContext: nodeCtx,
				Config:      conf.Node,
			}
			if conf.Snapshots != nil {
				ctx.Snapshot = conf.Snapshots[name]
			}
			service, err := serviceFunc(ctx)
			if err != nil {
				return nil, err
			}
			services[name] = service
			return service, nil
		}
		if err := stack.Register(constructor); err != nil {
			return stack, fmt.Errorf("error registering service %q: %v", name, err)
		}
	}

//注册快照服务
	err = stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
		return &snapshotService{services}, nil
	})
	if err != nil {
		return stack, fmt.Errorf("error starting snapshot service: %v", err)
	}

//启动堆栈
	if err = stack.Start(); err != nil {
		err = fmt.Errorf("error starting stack: %v", err)
	}
	return stack, err
}

const (
	envStatusURL  = "_P2P_STATUS_URL"
	envNodeConfig = "_P2P_NODE_CONFIG"
)

//启动后将nodestartupjson发送到仿真主机。
type nodeStartupJSON struct {
	Err        string
	WSEndpoint string
	NodeInfo   *p2p.NodeInfo
}

//SnapshotService是一个node.service，它包装了服务列表和
//公开API以生成这些服务的快照
type snapshotService struct {
	services map[string]node.Service
}

func (s *snapshotService) APIs() []rpc.API {
	return []rpc.API{{
		Namespace: "simulation",
		Version:   "1.0",
		Service:   SnapshotAPI{s.services},
	}}
}

func (s *snapshotService) Protocols() []p2p.Protocol {
	return nil
}

func (s *snapshotService) Start(*p2p.Server) error {
	return nil
}

func (s *snapshotService) Stop() error {
	return nil
}

//Snapshotapi提供了一个RPC方法来创建服务的快照
type SnapshotAPI struct {
	services map[string]node.Service
}

func (api SnapshotAPI) Snapshot() (map[string][]byte, error) {
	snapshots := make(map[string][]byte)
	for name, service := range api.services {
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

type wsRPCDialer struct {
	addrs map[string]string
}

//DialRPC通过创建WebSocket RPC来实现RpcDialer接口
//给定节点的客户端
func (w *wsRPCDialer) DialRPC(id enode.ID) (*rpc.Client, error) {
	addr, ok := w.addrs[id.String()]
	if !ok {
		return nil, fmt.Errorf("unknown node: %s", id)
	}
return rpc.DialWebsocket(context.Background(), addr, "http://“本地主机”
}
