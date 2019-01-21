
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2014 Go Ethereum作者
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

//包P2P实现以太坊P2P网络协议。
package p2p

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	defaultDialTimeout = 15 * time.Second

//连接默认值。
	maxActiveDialTasks     = 16
	defaultMaxPendingPeers = 50
	defaultDialRatio       = 3

//读取完整邮件所允许的最长时间。
//这实际上是连接可以空闲的时间量。
	frameReadTimeout = 30 * time.Second

//写入完整消息所允许的最长时间。
	frameWriteTimeout = 20 * time.Second
)

var errServerStopped = errors.New("server stopped")

//配置保留服务器选项。
type Config struct {
//此字段必须设置为有效的secp256k1私钥。
	PrivateKey *ecdsa.PrivateKey `toml:"-"`

//MaxPeers是可以
//有联系的。它必须大于零。
	MaxPeers int

//MaxPendingPeers是在
//握手阶段，分别为入站和出站连接计数。
//零默认为预设值。
	MaxPendingPeers int `toml:",omitempty"`

//DialRatio控制入站与拨入连接的比率。
//示例：拨号比率2允许拨1/2的连接。
//将DialRatio设置为零将默认为3。
	DialRatio int `toml:",omitempty"`

//nodiscovery可用于禁用对等发现机制。
//禁用对于协议调试（手动拓扑）很有用。
	NoDiscovery bool

//Discoveryv5指定新的基于主题发现的v5发现
//是否启动协议。
	DiscoveryV5 bool `toml:",omitempty"`

//名称设置此服务器的节点名称。
//使用common.makename创建遵循现有约定的名称。
	Name string `toml:"-"`

//bootstrapnodes用于建立连接
//与网络的其他部分。
	BootstrapNodes []*enode.Node

//bootstrapnodesv5用于建立连接
//与网络的其余部分一起使用v5发现
//协议。
	BootstrapNodesV5 []*discv5.Node `toml:",omitempty"`

//静态节点用作预先配置的连接，这些连接总是
//在断开时保持并重新连接。
	StaticNodes []*enode.Node

//受信任节点用作预先配置的连接，这些连接总是
//允许连接，甚至高于对等限制。
	TrustedNodes []*enode.Node

//连接可以限制到某些IP网络。
//如果此选项设置为非nil值，则仅主机与
//考虑列表中包含的IP网络。
	NetRestrict *netutil.Netlist `toml:",omitempty"`

//nodedatabase是包含以前看到的
//网络中的活动节点。
	NodeDatabase string `toml:",omitempty"`

//协议应包含支持的协议
//由服务器。为启动匹配协议
//每个对等体。
	Protocols []Protocol `toml:"-"`

//如果listenaddr设置为非nil地址，则服务器
//将侦听传入的连接。
//
//如果端口为零，操作系统将选择一个端口。这个
//当
//服务器已启动。
	ListenAddr string

//如果设置为非零值，则指定的NAT端口映射器
//用于使侦听端口对
//互联网。
	NAT nat.Interface `toml:",omitempty"`

//如果拨号程序设置为非零值，则指定的拨号程序
//用于拨号出站对等连接。
	Dialer NodeDialer `toml:"-"`

//如果nodial为真，服务器将不会拨任何对等机。
	NoDial bool `toml:",omitempty"`

//如果设置了EnableMsgeEvents，则服务器将发出PeerEvents
//无论何时向对等端发送或从对等端接收消息
	EnableMsgEvents bool

//logger是用于p2p.server的自定义记录器。
	Logger log.Logger `toml:",omitempty"`
}

//服务器管理所有对等连接。
type Server struct {
//服务器运行时不能修改配置字段。
	Config

//测试用挂钩。这些是有用的，因为我们可以抑制
//整个协议栈。
	newTransport func(net.Conn) transport
	newPeerHook  func(*Peer)

lock    sync.Mutex //保护运行
	running bool

	nodedb       *enode.DB
	localnode    *enode.LocalNode
	ntab         discoverTable
	listener     net.Listener
	ourHandshake *protoHandshake
	lastLookup   time.Time
	DiscV5       *discv5.Network

//这些是为对等机，对等机计数（而不是其他任何东西）。
	peerOp     chan peerOpFunc
	peerOpDone chan struct{}

	quit          chan struct{}
	addstatic     chan *enode.Node
	removestatic  chan *enode.Node
	addtrusted    chan *enode.Node
	removetrusted chan *enode.Node
	posthandshake chan *conn
	addpeer       chan *conn
	delpeer       chan peerDrop
loopWG        sync.WaitGroup //循环，listenloop
	peerFeed      event.Feed
	log           log.Logger
}

type peerOpFunc func(map[enode.ID]*Peer)

type peerDrop struct {
	*Peer
	err       error
requested bool //如果对等方发出信号，则为真
}

type connFlag int32

const (
	dynDialedConn connFlag = 1 << iota
	staticDialedConn
	inboundConn
	trustedConn
)

//
//在两次握手中。
type conn struct {
	fd net.Conn
	transport
	node  *enode.Node
	flags connFlag
cont  chan error //运行循环使用cont向setupconn发送错误信号。
caps  []Cap      //协议握手后有效
name  string     //协议握手后有效
}

type transport interface {
//两人握手。
	doEncHandshake(prv *ecdsa.PrivateKey, dialDest *ecdsa.PublicKey) (*ecdsa.PublicKey, error)
	doProtoHandshake(our *protoHandshake) (*protoHandshake, error)
//msgreadwriter只能在加密后使用
//握手已完成。代码使用conn.id跟踪
//通过在加密握手后将其设置为非零值。
	MsgReadWriter
//传输必须提供close，因为我们在
//测试。关闭实际网络连接不起作用
//这些测试中的任何内容，因为msgpipe不使用它。
	close(err error)
}

func (c *conn) String() string {
	s := c.flags.String()
	if (c.node.ID() != enode.ID{}) {
		s += " " + c.node.ID().String()
	}
	s += " " + c.fd.RemoteAddr().String()
	return s
}

func (f connFlag) String() string {
	s := ""
	if f&trustedConn != 0 {
		s += "-trusted"
	}
	if f&dynDialedConn != 0 {
		s += "-dyndial"
	}
	if f&staticDialedConn != 0 {
		s += "-staticdial"
	}
	if f&inboundConn != 0 {
		s += "-inbound"
	}
	if s != "" {
		s = s[1:]
	}
	return s
}

func (c *conn) is(f connFlag) bool {
	flags := connFlag(atomic.LoadInt32((*int32)(&c.flags)))
	return flags&f != 0
}

func (c *conn) set(f connFlag, val bool) {
	for {
		oldFlags := connFlag(atomic.LoadInt32((*int32)(&c.flags)))
		flags := oldFlags
		if val {
			flags |= f
		} else {
			flags &= ^f
		}
		if atomic.CompareAndSwapInt32((*int32)(&c.flags), int32(oldFlags), int32(flags)) {
			return
		}
	}
}

//对等端返回所有连接的对等端。
func (srv *Server) Peers() []*Peer {
	var ps []*Peer
	select {
//注意：我们希望将此函数放入变量中，但是
//这似乎导致了一些
//环境。
	case srv.peerOp <- func(peers map[enode.ID]*Peer) {
		for _, p := range peers {
			ps = append(ps, p)
		}
	}:
		<-srv.peerOpDone
	case <-srv.quit:
	}
	return ps
}

//PeerCount返回连接的对等数。
func (srv *Server) PeerCount() int {
	var count int
	select {
	case srv.peerOp <- func(ps map[enode.ID]*Peer) { count = len(ps) }:
		<-srv.peerOpDone
	case <-srv.quit:
	}
	return count
}

//addpeer连接到给定的节点并保持连接，直到
//服务器已关闭。如果由于任何原因连接失败，服务器将
//尝试重新连接对等机。
func (srv *Server) AddPeer(node *enode.Node) {
	select {
	case srv.addstatic <- node:
	case <-srv.quit:
	}
}

//从给定节点删除对等机断开连接
func (srv *Server) RemovePeer(node *enode.Node) {
	select {
	case srv.removestatic <- node:
	case <-srv.quit:
	}
}

//addTrustedPeer将给定节点添加到保留的白名单中，该白名单允许
//要始终连接的节点，即使插槽已满。
func (srv *Server) AddTrustedPeer(node *enode.Node) {
	select {
	case srv.addtrusted <- node:
	case <-srv.quit:
	}
}

//removeTrustedPeer从受信任的对等集删除给定节点。
func (srv *Server) RemoveTrustedPeer(node *enode.Node) {
	select {
	case srv.removetrusted <- node:
	case <-srv.quit:
	}
}

//subscribePeers订阅给定的通道到对等事件
func (srv *Server) SubscribeEvents(ch chan *PeerEvent) event.Subscription {
	return srv.peerFeed.Subscribe(ch)
}

//self返回本地节点的端点信息。
func (srv *Server) Self() *enode.Node {
	srv.lock.Lock()
	ln := srv.localnode
	srv.lock.Unlock()

	if ln == nil {
		return enode.NewV4(&srv.PrivateKey.PublicKey, net.ParseIP("0.0.0.0"), 0, 0)
	}
	return ln.Node()
}

//stop终止服务器和所有活动的对等连接。
//它会一直阻塞，直到关闭所有活动连接。
func (srv *Server) Stop() {
	srv.lock.Lock()
	if !srv.running {
		srv.lock.Unlock()
		return
	}
	srv.running = false
	if srv.listener != nil {
//此取消阻止侦听器接受
		srv.listener.Close()
	}
	close(srv.quit)
	srv.lock.Unlock()
	srv.loopWG.Wait()
}

//sharedudpconn实现共享连接。写将消息发送到基础连接，而读将返回
//发现无法处理的消息，并由主侦听器发送到未处理的通道。
type sharedUDPConn struct {
	*net.UDPConn
	unhandled chan discover.ReadPacket
}

//readfromudp实现discv5.conn
func (s *sharedUDPConn) ReadFromUDP(b []byte) (n int, addr *net.UDPAddr, err error) {
	packet, ok := <-s.unhandled
	if !ok {
		return 0, nil, errors.New("Connection was closed")
	}
	l := len(packet.Data)
	if l > len(b) {
		l = len(b)
	}
	copy(b[:l], packet.Data[:l])
	return l, packet.Addr, nil
}

//关闭机具discv5.conn
func (s *sharedUDPConn) Close() error {
	return nil
}

//开始运行服务器。
//服务器停止后不能重新使用。
func (srv *Server) Start() (err error) {
	srv.lock.Lock()
	defer srv.lock.Unlock()
	if srv.running {
		return errors.New("server already running")
	}
	srv.running = true
	srv.log = srv.Config.Logger
	if srv.log == nil {
		srv.log = log.New()
	}
	if srv.NoDial && srv.ListenAddr == "" {
		srv.log.Warn("P2P server will be useless, neither dialing nor listening")
	}

//静态场
	if srv.PrivateKey == nil {
		return errors.New("Server.PrivateKey must be set to a non-nil key")
	}
	if srv.newTransport == nil {
		srv.newTransport = newRLPX
	}
	if srv.Dialer == nil {
		srv.Dialer = TCPDialer{&net.Dialer{Timeout: defaultDialTimeout}}
	}
	srv.quit = make(chan struct{})
	srv.addpeer = make(chan *conn)
	srv.delpeer = make(chan peerDrop)
	srv.posthandshake = make(chan *conn)
	srv.addstatic = make(chan *enode.Node)
	srv.removestatic = make(chan *enode.Node)
	srv.addtrusted = make(chan *enode.Node)
	srv.removetrusted = make(chan *enode.Node)
	srv.peerOp = make(chan peerOpFunc)
	srv.peerOpDone = make(chan struct{})

	if err := srv.setupLocalNode(); err != nil {
		return err
	}
	if srv.ListenAddr != "" {
		if err := srv.setupListening(); err != nil {
			return err
		}
	}
	if err := srv.setupDiscovery(); err != nil {
		return err
	}

	dynPeers := srv.maxDialedConns()
	dialer := newDialState(srv.localnode.ID(), srv.StaticNodes, srv.BootstrapNodes, srv.ntab, dynPeers, srv.NetRestrict)
	srv.loopWG.Add(1)
	go srv.run(dialer)
	return nil
}

func (srv *Server) setupLocalNode() error {
//创建devp2p握手。
	pubkey := crypto.FromECDSAPub(&srv.PrivateKey.PublicKey)
	srv.ourHandshake = &protoHandshake{Version: baseProtocolVersion, Name: srv.Name, ID: pubkey[1:]}
	for _, p := range srv.Protocols {
		srv.ourHandshake.Caps = append(srv.ourHandshake.Caps, p.cap())
	}
	sort.Sort(capsByNameAndVersion(srv.ourHandshake.Caps))

//创建本地节点。
	db, err := enode.OpenDB(srv.Config.NodeDatabase)
	if err != nil {
		return err
	}
	srv.nodedb = db
	srv.localnode = enode.NewLocalNode(db, srv.PrivateKey)
	srv.localnode.SetFallbackIP(net.IP{127, 0, 0, 1})
	srv.localnode.Set(capsByNameAndVersion(srv.ourHandshake.Caps))
//TODO:检查冲突
	for _, p := range srv.Protocols {
		for _, e := range p.Attributes {
			srv.localnode.Set(e)
		}
	}
	switch srv.NAT.(type) {
	case nil:
//没有NAT接口，什么都不做。
	case nat.ExtIP:
//extip不阻塞，立即设置ip。
		ip, _ := srv.NAT.ExternalIP()
		srv.localnode.SetStaticIP(ip)
	default:
//询问路由器有关IP的信息。这需要一段时间来阻止启动，
//在后台进行。
		srv.loopWG.Add(1)
		go func() {
			defer srv.loopWG.Done()
			if ip, err := srv.NAT.ExternalIP(); err == nil {
				srv.localnode.SetStaticIP(ip)
			}
		}()
	}
	return nil
}

func (srv *Server) setupDiscovery() error {
	if srv.NoDiscovery && !srv.DiscoveryV5 {
		return nil
	}

	addr, err := net.ResolveUDPAddr("udp", srv.ListenAddr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	realaddr := conn.LocalAddr().(*net.UDPAddr)
	srv.log.Debug("UDP listener up", "addr", realaddr)
	if srv.NAT != nil {
		if !realaddr.IP.IsLoopback() {
			go nat.Map(srv.NAT, srv.quit, "udp", realaddr.Port, realaddr.Port, "ethereum discovery")
		}
	}
	srv.localnode.SetFallbackUDP(realaddr.Port)

//发现V4
	var unhandled chan discover.ReadPacket
	var sconn *sharedUDPConn
	if !srv.NoDiscovery {
		if srv.DiscoveryV5 {
			unhandled = make(chan discover.ReadPacket, 100)
			sconn = &sharedUDPConn{conn, unhandled}
		}
		cfg := discover.Config{
			PrivateKey:  srv.PrivateKey,
			NetRestrict: srv.NetRestrict,
			Bootnodes:   srv.BootstrapNodes,
			Unhandled:   unhandled,
		}
		ntab, err := discover.ListenUDP(conn, srv.localnode, cfg)
		if err != nil {
			return err
		}
		srv.ntab = ntab
	}
//发现V5
	if srv.DiscoveryV5 {
		var ntab *discv5.Network
		var err error
		if sconn != nil {
			ntab, err = discv5.ListenUDP(srv.PrivateKey, sconn, "", srv.NetRestrict)
		} else {
			ntab, err = discv5.ListenUDP(srv.PrivateKey, conn, "", srv.NetRestrict)
		}
		if err != nil {
			return err
		}
		if err := ntab.SetFallbackNodes(srv.BootstrapNodesV5); err != nil {
			return err
		}
		srv.DiscV5 = ntab
	}
	return nil
}

func (srv *Server) setupListening() error {
//启动TCP侦听器。
	listener, err := net.Listen("tcp", srv.ListenAddr)
	if err != nil {
		return err
	}
	laddr := listener.Addr().(*net.TCPAddr)
	srv.ListenAddr = laddr.String()
	srv.listener = listener
	srv.localnode.Set(enr.TCP(laddr.Port))

	srv.loopWG.Add(1)
	go srv.listenLoop()

//如果配置了NAT，则映射TCP侦听端口。
	if !laddr.IP.IsLoopback() && srv.NAT != nil {
		srv.loopWG.Add(1)
		go func() {
			nat.Map(srv.NAT, srv.quit, "tcp", laddr.Port, laddr.Port, "ethereum p2p")
			srv.loopWG.Done()
		}()
	}
	return nil
}

type dialer interface {
	newTasks(running int, peers map[enode.ID]*Peer, now time.Time) []task
	taskDone(task, time.Time)
	addStatic(*enode.Node)
	removeStatic(*enode.Node)
}

func (srv *Server) run(dialstate dialer) {
	srv.log.Info("Started P2P networking", "self", srv.localnode.Node())
	defer srv.loopWG.Done()
	defer srv.nodedb.Close()

	var (
		peers        = make(map[enode.ID]*Peer)
		inboundCount = 0
		trusted      = make(map[enode.ID]bool, len(srv.TrustedNodes))
		taskdone     = make(chan task, maxActiveDialTasks)
		runningTasks []task
queuedTasks  []task //尚未运行的任务
	)
//将受信任的节点放入映射以加快检查速度。
//可信对等机在启动时加载或通过addtrustedpeer rpc添加。
	for _, n := range srv.TrustedNodes {
		trusted[n.ID()] = true
	}

//从运行任务中删除t
	delTask := func(t task) {
		for i := range runningTasks {
			if runningTasks[i] == t {
				runningTasks = append(runningTasks[:i], runningTasks[i+1:]...)
				break
			}
		}
	}
//在满足最大活动任务数之前启动
	startTasks := func(ts []task) (rest []task) {
		i := 0
		for ; len(runningTasks) < maxActiveDialTasks && i < len(ts); i++ {
			t := ts[i]
			srv.log.Trace("New dial task", "task", t)
			go func() { t.Do(srv); taskdone <- t }()
			runningTasks = append(runningTasks, t)
		}
		return ts[i:]
	}
	scheduleTasks := func() {
//先从队列开始。
		queuedTasks = append(queuedTasks[:0], startTasks(queuedTasks)...)
//查询拨号程序以查找新任务，并立即尽可能多地启动。
		if len(runningTasks) < maxActiveDialTasks {
			nt := dialstate.newTasks(len(runningTasks)+len(queuedTasks), peers, time.Now())
			queuedTasks = append(queuedTasks, startTasks(nt)...)
		}
	}

running:
	for {
		scheduleTasks()

		select {
		case <-srv.quit:
//服务器已停止。运行清除逻辑。
			break running
		case n := <-srv.addstatic:
//addpeer使用此通道添加到
//短暂的静态对等列表。把它加到拨号器上，
//它将保持节点连接。
			srv.log.Trace("Adding static node", "node", n)
			dialstate.addStatic(n)
		case n := <-srv.removestatic:
//removepeer使用此通道发送
//断开对对等机的请求并开始
//停止保持节点连接。
			srv.log.Trace("Removing static node", "node", n)
			dialstate.removeStatic(n)
			if p, ok := peers[n.ID()]; ok {
				p.Disconnect(DiscRequested)
			}
		case n := <-srv.addtrusted:
//addTrustedPeer使用此通道添加enode
//到受信任的节点集。
			srv.log.Trace("Adding trusted node", "node", n)
			trusted[n.ID()] = true
//将任何已连接的对等机标记为受信任
			if p, ok := peers[n.ID()]; ok {
				p.rw.set(trustedConn, true)
			}
		case n := <-srv.removetrusted:
//此通道由removeTrustedPeer用于删除enode
//来自受信任的节点集。
			srv.log.Trace("Removing trusted node", "node", n)
			if _, ok := trusted[n.ID()]; ok {
				delete(trusted, n.ID())
			}
//将任何已连接的对等机取消标记为受信任
			if p, ok := peers[n.ID()]; ok {
				p.rw.set(trustedConn, false)
			}
		case op := <-srv.peerOp:
//此通道由对等方和对等方使用。
			op(peers)
			srv.peerOpDone <- struct{}{}
		case t := <-taskdone:
//任务完成了。告诉Dialstate，所以
//可以更新其状态并将其从活动
//任务列表。
			srv.log.Trace("Dial task done", "task", t)
			dialstate.taskDone(t, time.Now())
			delTask(t)
		case c := <-srv.posthandshake:
//连接已通过加密握手，因此
//远程标识已知（但尚未验证）。
			if trusted[c.node.ID()] {
//在对MaxPeers进行检查之前，请确保设置了可信标志。
				c.flags |= trustedConn
			}
//TODO:跟踪正在进行的入站节点ID（预对等），以避免拨号。
			select {
			case c.cont <- srv.encHandshakeChecks(peers, inboundCount, c):
			case <-srv.quit:
				break running
			}
		case c := <-srv.addpeer:
//此时，连接已超过协议握手。
//它的功能是已知的，并且远程身份被验证。
			err := srv.protoHandshakeChecks(peers, inboundCount, c)
			if err == nil {
//握手完成，通过所有检查。
				p := newPeer(c, srv.Protocols)
//如果启用了消息事件，则传递peerfeed
//向同行
				if srv.EnableMsgEvents {
					p.events = &srv.peerFeed
				}
				name := truncateName(c.name)
				srv.log.Debug("Adding p2p peer", "name", name, "addr", c.fd.RemoteAddr(), "peers", len(peers)+1)
				go srv.runPeer(p)
				peers[c.node.ID()] = p
				if p.Inbound() {
					inboundCount++
				}
			}
//拨号程序逻辑依赖于以下假设：
//添加对等机后完成拨号任务，或者
//丢弃的。最后取消阻止任务。
			select {
			case c.cont <- err:
			case <-srv.quit:
				break running
			}
		case pd := <-srv.delpeer:
//已断开连接的对等机。
			d := common.PrettyDuration(mclock.Now() - pd.created)
			pd.log.Debug("Removing p2p peer", "duration", d, "peers", len(peers)-1, "req", pd.requested, "err", pd.err)
			delete(peers, pd.ID())
			if pd.Inbound() {
				inboundCount--
			}
		}
	}

	srv.log.Trace("P2P networking is spinning down")

//终止发现。如果有正在运行的查找，它将很快终止。
	if srv.ntab != nil {
		srv.ntab.Close()
	}
	if srv.DiscV5 != nil {
		srv.DiscV5.Close()
	}
//断开所有对等机的连接。
	for _, p := range peers {
		p.Disconnect(DiscQuitting)
	}
//等待对等机关闭。挂起的连接和任务是
//此处未处理，将很快终止，因为srv.quit
//关闭。
	for len(peers) > 0 {
		p := <-srv.delpeer
		p.log.Trace("<-delpeer (spindown)", "remainingTasks", len(runningTasks))
		delete(peers, p.ID())
	}
}

func (srv *Server) protoHandshakeChecks(peers map[enode.ID]*Peer, inboundCount int, c *conn) error {
//删除没有匹配协议的连接。
	if len(srv.Protocols) > 0 && countMatchingProtocols(srv.Protocols, c.caps) == 0 {
		return DiscUselessPeer
	}
//重复加密握手检查，因为
//对等机集可能在握手之间发生了更改。
	return srv.encHandshakeChecks(peers, inboundCount, c)
}

func (srv *Server) encHandshakeChecks(peers map[enode.ID]*Peer, inboundCount int, c *conn) error {
	switch {
	case !c.is(trustedConn|staticDialedConn) && len(peers) >= srv.MaxPeers:
		return DiscTooManyPeers
	case !c.is(trustedConn) && c.is(inboundConn) && inboundCount >= srv.maxInboundConns():
		return DiscTooManyPeers
	case peers[c.node.ID()] != nil:
		return DiscAlreadyConnected
	case c.node.ID() == srv.localnode.ID():
		return DiscSelf
	default:
		return nil
	}
}

func (srv *Server) maxInboundConns() int {
	return srv.MaxPeers - srv.maxDialedConns()
}
func (srv *Server) maxDialedConns() int {
	if srv.NoDiscovery || srv.NoDial {
		return 0
	}
	r := srv.DialRatio
	if r == 0 {
		r = defaultDialRatio
	}
	return srv.MaxPeers / r
}

//listenloop在自己的goroutine中运行并接受
//入站连接。
func (srv *Server) listenLoop() {
	defer srv.loopWG.Done()
	srv.log.Debug("TCP listener up", "addr", srv.listener.Addr())

	tokens := defaultMaxPendingPeers
	if srv.MaxPendingPeers > 0 {
		tokens = srv.MaxPendingPeers
	}
	slots := make(chan struct{}, tokens)
	for i := 0; i < tokens; i++ {
		slots <- struct{}{}
	}

	for {
//在接受前等待握手槽。
		<-slots

		var (
			fd  net.Conn
			err error
		)
		for {
			fd, err = srv.listener.Accept()
			if netutil.IsTemporaryError(err) {
				srv.log.Debug("Temporary read error", "err", err)
				continue
			} else if err != nil {
				srv.log.Debug("Read error", "err", err)
				return
			}
			break
		}

//拒绝与NetRestrict不匹配的连接。
		if srv.NetRestrict != nil {
			if tcp, ok := fd.RemoteAddr().(*net.TCPAddr); ok && !srv.NetRestrict.Contains(tcp.IP) {
				srv.log.Debug("Rejected conn (not whitelisted in NetRestrict)", "addr", fd.RemoteAddr())
				fd.Close()
				slots <- struct{}{}
				continue
			}
		}

		var ip net.IP
		if tcp, ok := fd.RemoteAddr().(*net.TCPAddr); ok {
			ip = tcp.IP
		}
		fd = newMeteredConn(fd, true, ip)
		srv.log.Trace("Accepted connection", "addr", fd.RemoteAddr())
		go func() {
			srv.SetupConn(fd, inboundConn, nil)
			slots <- struct{}{}
		}()
	}
}

//SetupConn运行握手并尝试添加连接
//作为同伴。当连接添加为对等时返回
//或者握手失败。
func (srv *Server) SetupConn(fd net.Conn, flags connFlag, dialDest *enode.Node) error {
	c := &conn{fd: fd, transport: srv.newTransport(fd), flags: flags, cont: make(chan error)}
	err := srv.setupConn(c, flags, dialDest)
	if err != nil {
		c.close(err)
		srv.log.Trace("Setting up connection failed", "addr", fd.RemoteAddr(), "err", err)
	}
	return err
}

func (srv *Server) setupConn(c *conn, flags connFlag, dialDest *enode.Node) error {
//防止剩余的挂起conn进入握手。
	srv.lock.Lock()
	running := srv.running
	srv.lock.Unlock()
	if !running {
		return errServerStopped
	}
//如果拨号，请找出远程公钥。
	var dialPubkey *ecdsa.PublicKey
	if dialDest != nil {
		dialPubkey = new(ecdsa.PublicKey)
		if err := dialDest.Load((*enode.Secp256k1)(dialPubkey)); err != nil {
			return errors.New("dial destination doesn't have a secp256k1 public key")
		}
	}
//运行加密握手。
	remotePubkey, err := c.doEncHandshake(srv.PrivateKey, dialPubkey)
	if err != nil {
		srv.log.Trace("Failed RLPx handshake", "addr", c.fd.RemoteAddr(), "conn", c.flags, "err", err)
		return err
	}
	if dialDest != nil {
//对于拨号连接，请检查远程公钥是否匹配。
		if dialPubkey.X.Cmp(remotePubkey.X) != 0 || dialPubkey.Y.Cmp(remotePubkey.Y) != 0 {
			return DiscUnexpectedIdentity
		}
		c.node = dialDest
	} else {
		c.node = nodeFromConn(remotePubkey, c.fd)
	}
	if conn, ok := c.fd.(*meteredConn); ok {
		conn.handshakeDone(c.node.ID())
	}
	clog := srv.log.New("id", c.node.ID(), "addr", c.fd.RemoteAddr(), "conn", c.flags)
	err = srv.checkpoint(c, srv.posthandshake)
	if err != nil {
		clog.Trace("Rejected peer before protocol handshake", "err", err)
		return err
	}
//运行协议握手
	phs, err := c.doProtoHandshake(srv.ourHandshake)
	if err != nil {
		clog.Trace("Failed proto handshake", "err", err)
		return err
	}
	if id := c.node.ID(); !bytes.Equal(crypto.Keccak256(phs.ID), id[:]) {
		clog.Trace("Wrong devp2p handshake identity", "phsid", hex.EncodeToString(phs.ID))
		return DiscUnexpectedIdentity
	}
	c.caps, c.name = phs.Caps, phs.Name
	err = srv.checkpoint(c, srv.addpeer)
	if err != nil {
		clog.Trace("Rejected peer", "err", err)
		return err
	}
//如果检查成功完成，runpeer现在已经
//由run启动。
	clog.Trace("connection set up", "inbound", dialDest == nil)
	return nil
}

func nodeFromConn(pubkey *ecdsa.PublicKey, conn net.Conn) *enode.Node {
	var ip net.IP
	var port int
	if tcp, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		ip = tcp.IP
		port = tcp.Port
	}
	return enode.NewV4(pubkey, ip, port, port)
}

func truncateName(s string) string {
	if len(s) > 20 {
		return s[:20] + "..."
	}
	return s
}

//检查点发送conn运行，它执行
//阶段的后握手检查（后握手，addpeer）。
func (srv *Server) checkpoint(c *conn, stage chan<- *conn) error {
	select {
	case stage <- c:
	case <-srv.quit:
		return errServerStopped
	}
	select {
	case err := <-c.cont:
		return err
	case <-srv.quit:
		return errServerStopped
	}
}

//runpeer为每个对等运行自己的goroutine。
//它等待直到对等逻辑返回并删除
//同辈。
func (srv *Server) runPeer(p *Peer) {
	if srv.newPeerHook != nil {
		srv.newPeerHook(p)
	}

//广播对等添加
	srv.peerFeed.Send(&PeerEvent{
		Type: PeerEventTypeAdd,
		Peer: p.ID(),
	})

//运行协议
	remoteRequested, err := p.run()

//广播对等丢弃
	srv.peerFeed.Send(&PeerEvent{
		Type:  PeerEventTypeDrop,
		Peer:  p.ID(),
		Error: err.Error(),
	})

//注意：run等待现有对等机在srv.delpeer上发送
//返回之前，不应在srv.quit上选择此发送。
	srv.delpeer <- peerDrop{p, err, remoteRequested}
}

//nodeinfo表示有关主机的已知信息的简短摘要。
type NodeInfo struct {
ID    string `json:"id"`    //唯一节点标识符（也是加密密钥）
Name  string `json:"name"`  //节点的名称，包括客户端类型、版本、操作系统、自定义数据
Enode string `json:"enode"` //用于从远程对等添加此对等的enode url
ENR   string `json:"enr"`   //以太坊节点记录
IP    string `json:"ip"`    //节点的IP地址
	Ports struct {
Discovery int `json:"discovery"` //发现协议的UDP侦听端口
Listener  int `json:"listener"`  //rlpx的TCP侦听端口
	} `json:"ports"`
	ListenAddr string                 `json:"listenAddr"`
	Protocols  map[string]interface{} `json:"protocols"`
}

//nodeinfo收集并返回有关主机的已知元数据集合。
func (srv *Server) NodeInfo() *NodeInfo {
//收集和组装通用节点信息
	node := srv.Self()
	info := &NodeInfo{
		Name:       srv.Name,
		Enode:      node.String(),
		ID:         node.ID().String(),
		IP:         node.IP().String(),
		ListenAddr: srv.ListenAddr,
		Protocols:  make(map[string]interface{}),
	}
	info.Ports.Discovery = node.UDP()
	info.Ports.Listener = node.TCP()
	if enc, err := rlp.EncodeToBytes(node.Record()); err == nil {
		info.ENR = "0x" + hex.EncodeToString(enc)
	}

//收集所有正在运行的协议信息（每个协议类型仅一次）
	for _, proto := range srv.Protocols {
		if _, ok := info.Protocols[proto.Name]; !ok {
			nodeInfo := interface{}("unknown")
			if query := proto.NodeInfo; query != nil {
				nodeInfo = proto.NodeInfo()
			}
			info.Protocols[proto.Name] = nodeInfo
		}
	}
	return info
}

//PeerSinfo返回描述已连接对等端的元数据对象数组。
func (srv *Server) PeersInfo() []*PeerInfo {
//收集所有通用和子协议特定的信息
	infos := make([]*PeerInfo, 0, srv.PeerCount())
	for _, peer := range srv.Peers() {
		if peer != nil {
			infos = append(infos, peer.Info())
		}
	}
//按节点标识符的字母顺序对结果数组排序
	for i := 0; i < len(infos); i++ {
		for j := i + 1; j < len(infos); j++ {
			if infos[i].ID > infos[j].ID {
				infos[i], infos[j] = infos[j], infos[i]
			}
		}
	}
	return infos
}
