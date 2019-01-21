
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

package p2p

import (
	"container/heap"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/netutil"
)

const (
//这是介于
//重拨某个节点。
	dialHistoryExpiration = 30 * time.Second

//发现查找受到限制，只能运行
//每隔几秒钟一次。
	lookupInterval = 4 * time.Second

//如果在这段时间内找不到对等点，则初始引导节点为
//试图连接。
	fallbackInterval = 20 * time.Second

//端点分辨率通过有界退避进行限制。
	initialResolveDelay = 60 * time.Second
	maxResolveDelay     = time.Hour
)

//nodeadialer用于连接到网络中的节点，通常使用
//一个底层的net.dialer，但在测试中也使用了net.pipe
type NodeDialer interface {
	Dial(*enode.Node) (net.Conn, error)
}

//tcpDialer通过使用net.dialer
//创建到网络中节点的TCP连接
type TCPDialer struct {
	*net.Dialer
}

//拨号创建到节点的TCP连接
func (t TCPDialer) Dial(dest *enode.Node) (net.Conn, error) {
	addr := &net.TCPAddr{IP: dest.IP(), Port: dest.TCP()}
	return t.Dialer.Dial("tcp", addr.String())
}

//拨号状态计划拨号和查找。
//每次迭代都有机会计算新任务
//在server.run中的主循环。
type dialstate struct {
	maxDynDials int
	ntab        discoverTable
	netrestrict *netutil.Netlist
	self        enode.ID

	lookupRunning bool
	dialing       map[enode.ID]connFlag
lookupBuf     []*enode.Node //当前发现查找结果
randomNodes   []*enode.Node //从表中填充
	static        map[enode.ID]*dialTask
	hist          *dialHistory

start     time.Time     //拨号器首次使用的时间
bootnodes []*enode.Node //没有对等机时的默认拨号
}

type discoverTable interface {
	Close()
	Resolve(*enode.Node) *enode.Node
	LookupRandom() []*enode.Node
	ReadRandomNodes([]*enode.Node) int
}

//拨号历史记录会记住最近的拨号。
type dialHistory []pastDial

//PastDial是拨号历史记录中的一个条目。
type pastDial struct {
	id  enode.ID
	exp time.Time
}

type task interface {
	Do(*Server)
}

//为所拨的每个节点生成一个拨号任务。它的
//任务运行时无法访问字段。
type dialTask struct {
	flags        connFlag
	dest         *enode.Node
	lastResolved time.Time
	resolveDelay time.Duration
}

//discovertask运行发现表操作。
//任何时候只有一个discovertask处于活动状态。
//discovertask.do执行随机查找。
type discoverTask struct {
	results []*enode.Node
}

//如果没有其他任务，则生成waitexpiretask
//在server.run中保持循环。
type waitExpireTask struct {
	time.Duration
}

func newDialState(self enode.ID, static []*enode.Node, bootnodes []*enode.Node, ntab discoverTable, maxdyn int, netrestrict *netutil.Netlist) *dialstate {
	s := &dialstate{
		maxDynDials: maxdyn,
		ntab:        ntab,
		self:        self,
		netrestrict: netrestrict,
		static:      make(map[enode.ID]*dialTask),
		dialing:     make(map[enode.ID]connFlag),
		bootnodes:   make([]*enode.Node, len(bootnodes)),
		randomNodes: make([]*enode.Node, maxdyn/2),
		hist:        new(dialHistory),
	}
	copy(s.bootnodes, bootnodes)
	for _, n := range static {
		s.addStatic(n)
	}
	return s
}

func (s *dialstate) addStatic(n *enode.Node) {
//这将覆盖任务，而不是更新现有的
//输入，让用户有机会强制执行解决操作。
	s.static[n.ID()] = &dialTask{flags: staticDialedConn, dest: n}
}

func (s *dialstate) removeStatic(n *enode.Node) {
//这将删除一个任务，因此将来不会尝试连接。
	delete(s.static, n.ID())
//这将删除以前的拨号时间戳，以便应用程序
//可以强制服务器立即与所选对等机重新连接。
	s.hist.remove(n.ID())
}

func (s *dialstate) newTasks(nRunning int, peers map[enode.ID]*Peer, now time.Time) []task {
	if s.start.IsZero() {
		s.start = now
	}

	var newtasks []task
	addDial := func(flag connFlag, n *enode.Node) bool {
		if err := s.checkDial(n, peers); err != nil {
			log.Trace("Skipping dial candidate", "id", n.ID(), "addr", &net.TCPAddr{IP: n.IP(), Port: n.TCP()}, "err", err)
			return false
		}
		s.dialing[n.ID()] = flag
		newtasks = append(newtasks, &dialTask{flags: flag, dest: n})
		return true
	}

//计算此时所需的动态拨号数。
	needDynDials := s.maxDynDials
	for _, p := range peers {
		if p.rw.is(dynDialedConn) {
			needDynDials--
		}
	}
	for _, flag := range s.dialing {
		if flag&dynDialedConn != 0 {
			needDynDials--
		}
	}

//每次调用时使拨号历史记录过期。
	s.hist.expire(now)

//如果静态节点未连接，则为其创建拨号。
	for id, t := range s.static {
		err := s.checkDial(t.dest, peers)
		switch err {
		case errNotWhitelisted, errSelf:
			log.Warn("Removing static dial candidate", "id", t.dest.ID, "addr", &net.TCPAddr{IP: t.dest.IP(), Port: t.dest.TCP()}, "err", err)
			delete(s.static, t.dest.ID())
		case nil:
			s.dialing[id] = t.flags
			newtasks = append(newtasks, t)
		}
	}
//如果我们没有任何对等点，请尝试拨打随机引导节点。这个
//场景对于发现
//桌子上可能满是坏同学，很难找到好同学。
	if len(peers) == 0 && len(s.bootnodes) > 0 && needDynDials > 0 && now.Sub(s.start) > fallbackInterval {
		bootnode := s.bootnodes[0]
		s.bootnodes = append(s.bootnodes[:0], s.bootnodes[1:]...)
		s.bootnodes = append(s.bootnodes, bootnode)

		if addDial(dynDialedConn, bootnode) {
			needDynDials--
		}
	}
//将表中的随机节点用于所需的一半
//动态拨号。
	randomCandidates := needDynDials / 2
	if randomCandidates > 0 {
		n := s.ntab.ReadRandomNodes(s.randomNodes)
		for i := 0; i < randomCandidates && i < n; i++ {
			if addDial(dynDialedConn, s.randomNodes[i]) {
				needDynDials--
			}
		}
	}
//从随机查找结果创建动态拨号，已尝试删除
//结果缓冲区中的项。
	i := 0
	for ; i < len(s.lookupBuf) && needDynDials > 0; i++ {
		if addDial(dynDialedConn, s.lookupBuf[i]) {
			needDynDials--
		}
	}
	s.lookupBuf = s.lookupBuf[:copy(s.lookupBuf, s.lookupBuf[i:])]
//如果需要更多候选项，则启动查找。
	if len(s.lookupBuf) < needDynDials && !s.lookupRunning {
		s.lookupRunning = true
		newtasks = append(newtasks, &discoverTask{})
	}

//启动计时器，等待下一个节点全部过期
//候选人已被试用，目前没有活动任务。
//这样可以防止拨号程序逻辑没有勾选的情况发生。
//因为没有挂起的事件。
	if nRunning == 0 && len(newtasks) == 0 && s.hist.Len() > 0 {
		t := &waitExpireTask{s.hist.min().exp.Sub(now)}
		newtasks = append(newtasks, t)
	}
	return newtasks
}

var (
	errSelf             = errors.New("is self")
	errAlreadyDialing   = errors.New("already dialing")
	errAlreadyConnected = errors.New("already connected")
	errRecentlyDialed   = errors.New("recently dialed")
	errNotWhitelisted   = errors.New("not contained in netrestrict whitelist")
)

func (s *dialstate) checkDial(n *enode.Node, peers map[enode.ID]*Peer) error {
	_, dialing := s.dialing[n.ID()]
	switch {
	case dialing:
		return errAlreadyDialing
	case peers[n.ID()] != nil:
		return errAlreadyConnected
	case n.ID() == s.self:
		return errSelf
	case s.netrestrict != nil && !s.netrestrict.Contains(n.IP()):
		return errNotWhitelisted
	case s.hist.contains(n.ID()):
		return errRecentlyDialed
	}
	return nil
}

func (s *dialstate) taskDone(t task, now time.Time) {
	switch t := t.(type) {
	case *dialTask:
		s.hist.add(t.dest.ID(), now.Add(dialHistoryExpiration))
		delete(s.dialing, t.dest.ID())
	case *discoverTask:
		s.lookupRunning = false
		s.lookupBuf = append(s.lookupBuf, t.results...)
	}
}

func (t *dialTask) Do(srv *Server) {
	if t.dest.Incomplete() {
		if !t.resolve(srv) {
			return
		}
	}
	err := t.dial(srv, t.dest)
	if err != nil {
		log.Trace("Dial error", "task", t, "err", err)
//如果拨号失败，请尝试解析静态节点的ID。
		if _, ok := err.(*dialError); ok && t.flags&staticDialedConn != 0 {
			if t.resolve(srv) {
				t.dial(srv, t.dest)
			}
		}
	}
}

//解决查找目标的当前终结点的尝试
//使用发现。
//
//解决操作通过后退进行节流，以避免淹没
//对不存在的节点进行无用查询的发现网络。
//当找到节点时，退避延迟重置。
func (t *dialTask) resolve(srv *Server) bool {
	if srv.ntab == nil {
		log.Debug("Can't resolve node", "id", t.dest.ID, "err", "discovery is disabled")
		return false
	}
	if t.resolveDelay == 0 {
		t.resolveDelay = initialResolveDelay
	}
	if time.Since(t.lastResolved) < t.resolveDelay {
		return false
	}
	resolved := srv.ntab.Resolve(t.dest)
	t.lastResolved = time.Now()
	if resolved == nil {
		t.resolveDelay *= 2
		if t.resolveDelay > maxResolveDelay {
			t.resolveDelay = maxResolveDelay
		}
		log.Debug("Resolving node failed", "id", t.dest.ID, "newdelay", t.resolveDelay)
		return false
	}
//找到节点。
	t.resolveDelay = initialResolveDelay
	t.dest = resolved
	log.Debug("Resolved node", "id", t.dest.ID, "addr", &net.TCPAddr{IP: t.dest.IP(), Port: t.dest.TCP()})
	return true
}

type dialError struct {
	error
}

//拨号执行实际连接尝试。
func (t *dialTask) dial(srv *Server, dest *enode.Node) error {
	fd, err := srv.Dialer.Dial(dest)
	if err != nil {
		return &dialError{err}
	}
	mfd := newMeteredConn(fd, false, dest.IP())
	return srv.SetupConn(mfd, t.flags, dest)
}

func (t *dialTask) String() string {
	id := t.dest.ID()
	return fmt.Sprintf("%v %x %v:%d", t.flags, id[:8], t.dest.IP(), t.dest.TCP())
}

func (t *discoverTask) Do(srv *Server) {
//每当动态拨号
//必要的。查找需要花费一些时间，否则
//事件循环旋转过快。
	next := srv.lastLookup.Add(lookupInterval)
	if now := time.Now(); now.Before(next) {
		time.Sleep(next.Sub(now))
	}
	srv.lastLookup = time.Now()
	t.results = srv.ntab.LookupRandom()
}

func (t *discoverTask) String() string {
	s := "discovery lookup"
	if len(t.results) > 0 {
		s += fmt.Sprintf(" (%d results)", len(t.results))
	}
	return s
}

func (t waitExpireTask) Do(*Server) {
	time.Sleep(t.Duration)
}
func (t waitExpireTask) String() string {
	return fmt.Sprintf("wait for dial hist expire (%v)", t.Duration)
}

//仅使用这些方法访问或修改拨号历史记录。
func (h dialHistory) min() pastDial {
	return h[0]
}
func (h *dialHistory) add(id enode.ID, exp time.Time) {
	heap.Push(h, pastDial{id, exp})

}
func (h *dialHistory) remove(id enode.ID) bool {
	for i, v := range *h {
		if v.id == id {
			heap.Remove(h, i)
			return true
		}
	}
	return false
}
func (h dialHistory) contains(id enode.ID) bool {
	for _, v := range h {
		if v.id == id {
			return true
		}
	}
	return false
}
func (h *dialHistory) expire(now time.Time) {
	for h.Len() > 0 && h.min().exp.Before(now) {
		heap.Pop(h)
	}
}

//堆接口样板
func (h dialHistory) Len() int           { return len(h) }
func (h dialHistory) Less(i, j int) bool { return h[i].exp.Before(h[j].exp) }
func (h dialHistory) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *dialHistory) Push(x interface{}) {
	*h = append(*h, x.(pastDial))
}
func (h *dialHistory) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
