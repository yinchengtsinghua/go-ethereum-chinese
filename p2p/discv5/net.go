
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

package discv5

import (
	"bytes"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
)

var (
	errInvalidEvent = errors.New("invalid in current state")
	errNoQuery      = errors.New("no pending query")
)

const (
	autoRefreshInterval   = 1 * time.Hour
	bucketRefreshInterval = 1 * time.Minute
	seedCount             = 30
	seedMaxAge            = 5 * 24 * time.Hour
	lowPort               = 1024
)

const testTopic = "foo"

const (
	printTestImgLogs = false
)

//网络管理表和所有协议交互。
type Network struct {
db          *nodeDB //已知节点数据库
	conn        transport
	netrestrict *netutil.Netlist

closed           chan struct{}          //循环完成时关闭
closeReq         chan struct{}          //'关闭请求'
refreshReq       chan []*Node           //查找要求刷新此频道
refreshResp      chan (<-chan struct{}) //…让这个频道阻止
read             chan ingressPacket     //入口包到达这里
	timeout          chan timeoutEvent
queryReq         chan *findnodeQuery //查找在此频道上提交findnode查询
	tableOpReq       chan func()
	tableOpResp      chan struct{}
	topicRegisterReq chan topicRegisterReq
	topicSearchReq   chan topicSearchReq

//主循环的状态。
	tab           *Table
	topictab      *topicTable
	ticketStore   *ticketStore
	nursery       []*Node
nodes         map[NodeID]*Node //用状态跟踪活动节点！=已知
	timeoutTimers map[timeoutEvent]*time.Timer

//重新验证队列。
//放置在这些队列上的节点最终将被ping。
	slowRevalidateQueue []*Node
	fastRevalidateQueue []*Node

//状态转换缓冲区。
	sendBuf []*ingressPacket
}

//传输由UDP传输实现。
//它是一个接口，因此我们可以在不打开大量UDP的情况下进行测试
//不生成私钥的套接字。
type transport interface {
	sendPing(remote *Node, remoteAddr *net.UDPAddr, topics []Topic) (hash []byte)
	sendNeighbours(remote *Node, nodes []*Node)
	sendFindnodeHash(remote *Node, target common.Hash)
	sendTopicRegister(remote *Node, topics []Topic, topicIdx int, pong []byte)
	sendTopicNodes(remote *Node, queryHash common.Hash, nodes []*Node)

	send(remote *Node, ptype nodeEvent, p interface{}) (hash []byte)

	localAddr() *net.UDPAddr
	Close()
}

type findnodeQuery struct {
	remote   *Node
	target   common.Hash
	reply    chan<- []*Node
nresults int //接收节点计数器
}

type topicRegisterReq struct {
	add   bool
	topic Topic
}

type topicSearchReq struct {
	topic  Topic
	found  chan<- *Node
	lookup chan<- bool
	delay  time.Duration
}

type topicSearchResult struct {
	target lookupInfo
	nodes  []*Node
}

type timeoutEvent struct {
	ev   nodeEvent
	node *Node
}

func newNetwork(conn transport, ourPubkey ecdsa.PublicKey, dbPath string, netrestrict *netutil.Netlist) (*Network, error) {
	ourID := PubkeyID(&ourPubkey)

	var db *nodeDB
	if dbPath != "<no database>" {
		var err error
		if db, err = newNodeDB(dbPath, Version, ourID); err != nil {
			return nil, err
		}
	}

	tab := newTable(ourID, conn.localAddr())
	net := &Network{
		db:               db,
		conn:             conn,
		netrestrict:      netrestrict,
		tab:              tab,
		topictab:         newTopicTable(db, tab.self),
		ticketStore:      newTicketStore(),
		refreshReq:       make(chan []*Node),
		refreshResp:      make(chan (<-chan struct{})),
		closed:           make(chan struct{}),
		closeReq:         make(chan struct{}),
		read:             make(chan ingressPacket, 100),
		timeout:          make(chan timeoutEvent),
		timeoutTimers:    make(map[timeoutEvent]*time.Timer),
		tableOpReq:       make(chan func()),
		tableOpResp:      make(chan struct{}),
		queryReq:         make(chan *findnodeQuery),
		topicRegisterReq: make(chan topicRegisterReq),
		topicSearchReq:   make(chan topicSearchReq),
		nodes:            make(map[NodeID]*Node),
	}
	go net.loop()
	return net, nil
}

//close终止网络侦听器并刷新节点数据库。
func (net *Network) Close() {
	net.conn.Close()
	select {
	case <-net.closed:
	case net.closeReq <- struct{}{}:
		<-net.closed
	}
}

//self返回本地节点。
//调用方不应修改返回的节点。
func (net *Network) Self() *Node {
	return net.tab.self
}

//readrandomnodes用来自
//表。它不会多次写入同一节点。节点
//切片是副本，可以由调用方修改。
func (net *Network) ReadRandomNodes(buf []*Node) (n int) {
	net.reqTableOp(func() { n = net.tab.readRandomNodes(buf) })
	return n
}

//setFallbackNodes设置初始接触点。这些节点
//如果表为空，则用于连接到网络
//数据库中没有已知节点。
func (net *Network) SetFallbackNodes(nodes []*Node) error {
	nursery := make([]*Node, 0, len(nodes))
	for _, n := range nodes {
		if err := n.validateComplete(); err != nil {
			return fmt.Errorf("bad bootstrap/fallback node %q (%v)", n, err)
		}
//重新计算cpy.sha，因为节点可能没有
//由newnode或parsenode创建。
		cpy := *n
		cpy.sha = crypto.Keccak256Hash(n.ID[:])
		nursery = append(nursery, &cpy)
	}
	net.reqRefresh(nursery)
	return nil
}

//解析搜索具有给定ID的特定节点。
//如果找不到节点，则返回nil。
func (net *Network) Resolve(targetID NodeID) *Node {
	result := net.lookup(crypto.Keccak256Hash(targetID[:]), true)
	for _, n := range result {
		if n.ID == targetID {
			return n
		}
	}
	return nil
}

//查找对关闭的节点执行网络搜索
//目标。它通过查询接近目标
//在每次迭代中离它更近的节点。
//给定目标不需要是实际节点
//标识符。
//
//结果中可能包含本地节点。
func (net *Network) Lookup(targetID NodeID) []*Node {
	return net.lookup(crypto.Keccak256Hash(targetID[:]), false)
}

func (net *Network) lookup(target common.Hash, stopOnMatch bool) []*Node {
	var (
		asked          = make(map[NodeID]bool)
		seen           = make(map[NodeID]bool)
		reply          = make(chan []*Node, alpha)
		result         = nodesByDistance{target: target}
		pendingQueries = 0
	)
//从本地节点获取初始答案。
	result.push(net.tab.self, bucketSize)
	for {
//询问我们尚未询问的α最近的节点。
		for i := 0; i < len(result.entries) && pendingQueries < alpha; i++ {
			n := result.entries[i]
			if !asked[n.ID] {
				asked[n.ID] = true
				pendingQueries++
				net.reqQueryFindnode(n, target, reply)
			}
		}
		if pendingQueries == 0 {
//我们要求所有最近的节点停止搜索。
			break
		}
//等待下一个答复。
		select {
		case nodes := <-reply:
			for _, n := range nodes {
				if n != nil && !seen[n.ID] {
					seen[n.ID] = true
					result.push(n, bucketSize)
					if stopOnMatch && n.sha == target {
						return result.entries
					}
				}
			}
			pendingQueries--
		case <-time.After(respTimeout):
//忽略所有挂起的请求，启动新的请求
			pendingQueries = 0
			reply = make(chan []*Node, alpha)
		}
	}
	return result.entries
}

func (net *Network) RegisterTopic(topic Topic, stop <-chan struct{}) {
	select {
	case net.topicRegisterReq <- topicRegisterReq{true, topic}:
	case <-net.closed:
		return
	}
	select {
	case <-net.closed:
	case <-stop:
		select {
		case net.topicRegisterReq <- topicRegisterReq{false, topic}:
		case <-net.closed:
		}
	}
}

func (net *Network) SearchTopic(topic Topic, setPeriod <-chan time.Duration, found chan<- *Node, lookup chan<- bool) {
	for {
		select {
		case <-net.closed:
			return
		case delay, ok := <-setPeriod:
			select {
			case net.topicSearchReq <- topicSearchReq{topic: topic, found: found, lookup: lookup, delay: delay}:
			case <-net.closed:
				return
			}
			if !ok {
				return
			}
		}
	}
}

func (net *Network) reqRefresh(nursery []*Node) <-chan struct{} {
	select {
	case net.refreshReq <- nursery:
		return <-net.refreshResp
	case <-net.closed:
		return net.closed
	}
}

func (net *Network) reqQueryFindnode(n *Node, target common.Hash, reply chan []*Node) bool {
	q := &findnodeQuery{remote: n, target: target, reply: reply}
	select {
	case net.queryReq <- q:
		return true
	case <-net.closed:
		return false
	}
}

func (net *Network) reqReadPacket(pkt ingressPacket) {
	select {
	case net.read <- pkt:
	case <-net.closed:
	}
}

func (net *Network) reqTableOp(f func()) (called bool) {
	select {
	case net.tableOpReq <- f:
		<-net.tableOpResp
		return true
	case <-net.closed:
		return false
	}
}

//TODO:外部地址处理。

type topicSearchInfo struct {
	lookupChn chan<- bool
	period    time.Duration
}

const maxSearchCount = 5

func (net *Network) loop() {
	var (
		refreshTimer       = time.NewTicker(autoRefreshInterval)
		bucketRefreshTimer = time.NewTimer(bucketRefreshInterval)
refreshDone        chan struct{} //“刷新”查找结束时关闭
	)

//跟踪下一张要注册的票据。
	var (
		nextTicket        *ticketRef
		nextRegisterTimer *time.Timer
		nextRegisterTime  <-chan time.Time
	)
	defer func() {
		if nextRegisterTimer != nil {
			nextRegisterTimer.Stop()
		}
	}()
	resetNextTicket := func() {
		ticket, timeout := net.ticketStore.nextFilteredTicket()
		if nextTicket != ticket {
			nextTicket = ticket
			if nextRegisterTimer != nil {
				nextRegisterTimer.Stop()
				nextRegisterTime = nil
			}
			if ticket != nil {
				nextRegisterTimer = time.NewTimer(timeout)
				nextRegisterTime = nextRegisterTimer.C
			}
		}
	}

//跟踪注册和搜索查找。
	var (
		topicRegisterLookupTarget lookupInfo
		topicRegisterLookupDone   chan []*Node
		topicRegisterLookupTick   = time.NewTimer(0)
		searchReqWhenRefreshDone  []topicSearchReq
		searchInfo                = make(map[Topic]topicSearchInfo)
		activeSearchCount         int
	)
	topicSearchLookupDone := make(chan topicSearchResult, 100)
	topicSearch := make(chan Topic, 100)
	<-topicRegisterLookupTick.C

	statsDump := time.NewTicker(10 * time.Second)

loop:
	for {
		resetNextTicket()

		select {
		case <-net.closeReq:
			log.Trace("<-net.closeReq")
			break loop

//入口包处理。
		case pkt := <-net.read:
//fmt.println（“读取”，pkt.ev）
			log.Trace("<-net.read")
			n := net.internNode(&pkt)
			prestate := n.state
			status := "ok"
			if err := net.handle(n, pkt.ev, &pkt); err != nil {
				status = err.Error()
			}
			log.Trace("", "msg", log.Lazy{Fn: func() string {
				return fmt.Sprintf("<<< (%d) %v from %x@%v: %v -> %v (%v)",
					net.tab.count, pkt.ev, pkt.remoteID[:8], pkt.remoteAddr, prestate, n.state, status)
			}})
//TODO:如果n.state变为大于等于known，则保持状态；如果n.state变为小于等于known，则删除。

//状态转换超时。
		case timeout := <-net.timeout:
			log.Trace("<-net.timeout")
			if net.timeoutTimers[timeout] == nil {
//过时计时器（已中止）。
				continue
			}
			delete(net.timeoutTimers, timeout)
			prestate := timeout.node.state
			status := "ok"
			if err := net.handle(timeout.node, timeout.ev, nil); err != nil {
				status = err.Error()
			}
			log.Trace("", "msg", log.Lazy{Fn: func() string {
				return fmt.Sprintf("--- (%d) %v for %x@%v: %v -> %v (%v)",
					net.tab.count, timeout.ev, timeout.node.ID[:8], timeout.node.addr(), prestate, timeout.node.state, status)
			}})

//查询。
		case q := <-net.queryReq:
			log.Trace("<-net.queryReq")
			if !q.start(net) {
				q.remote.deferQuery(q)
			}

//与表交互。
		case f := <-net.tableOpReq:
			log.Trace("<-net.tableOpReq")
			f()
			net.tableOpResp <- struct{}{}

//主题注册资料。
		case req := <-net.topicRegisterReq:
			log.Trace("<-net.topicRegisterReq")
			if !req.add {
				net.ticketStore.removeRegisterTopic(req.topic)
				continue
			}
			net.ticketStore.addTopic(req.topic, true)
//如果我们正在等待空闲（没有可查找的内容），请给售票处
//有机会早点开始。这将加速半径的收敛
//确定新主题。
//如果topicRegisterLookupDone==nil
			if topicRegisterLookupTarget.target == (common.Hash{}) {
				log.Trace("topicRegisterLookupTarget == null")
				if topicRegisterLookupTick.Stop() {
					<-topicRegisterLookupTick.C
				}
				target, delay := net.ticketStore.nextRegisterLookup()
				topicRegisterLookupTarget = target
				topicRegisterLookupTick.Reset(delay)
			}

		case nodes := <-topicRegisterLookupDone:
			log.Trace("<-topicRegisterLookupDone")
			net.ticketStore.registerLookupDone(topicRegisterLookupTarget, nodes, func(n *Node) []byte {
				net.ping(n, n.addr())
				return n.pingEcho
			})
			target, delay := net.ticketStore.nextRegisterLookup()
			topicRegisterLookupTarget = target
			topicRegisterLookupTick.Reset(delay)
			topicRegisterLookupDone = nil

		case <-topicRegisterLookupTick.C:
			log.Trace("<-topicRegisterLookupTick")
			if (topicRegisterLookupTarget.target == common.Hash{}) {
				target, delay := net.ticketStore.nextRegisterLookup()
				topicRegisterLookupTarget = target
				topicRegisterLookupTick.Reset(delay)
				topicRegisterLookupDone = nil
			} else {
				topicRegisterLookupDone = make(chan []*Node)
				target := topicRegisterLookupTarget.target
				go func() { topicRegisterLookupDone <- net.lookup(target, false) }()
			}

		case <-nextRegisterTime:
			log.Trace("<-nextRegisterTime")
			net.ticketStore.ticketRegistered(*nextTicket)
//fmt.println（“sendtopicregister”，nextTicket.t.node.addr（）.string（），nextTicket.t.topics，nextTicket.idx，nextTicket.t.pong）
			net.conn.sendTopicRegister(nextTicket.t.node, nextTicket.t.topics, nextTicket.idx, nextTicket.t.pong)

		case req := <-net.topicSearchReq:
			if refreshDone == nil {
				log.Trace("<-net.topicSearchReq")
				info, ok := searchInfo[req.topic]
				if ok {
					if req.delay == time.Duration(0) {
						delete(searchInfo, req.topic)
						net.ticketStore.removeSearchTopic(req.topic)
					} else {
						info.period = req.delay
						searchInfo[req.topic] = info
					}
					continue
				}
				if req.delay != time.Duration(0) {
					var info topicSearchInfo
					info.period = req.delay
					info.lookupChn = req.lookup
					searchInfo[req.topic] = info
					net.ticketStore.addSearchTopic(req.topic, req.found)
					topicSearch <- req.topic
				}
			} else {
				searchReqWhenRefreshDone = append(searchReqWhenRefreshDone, req)
			}

		case topic := <-topicSearch:
			if activeSearchCount < maxSearchCount {
				activeSearchCount++
				target := net.ticketStore.nextSearchLookup(topic)
				go func() {
					nodes := net.lookup(target.target, false)
					topicSearchLookupDone <- topicSearchResult{target: target, nodes: nodes}
				}()
			}
			period := searchInfo[topic].period
			if period != time.Duration(0) {
				go func() {
					time.Sleep(period)
					topicSearch <- topic
				}()
			}

		case res := <-topicSearchLookupDone:
			activeSearchCount--
			if lookupChn := searchInfo[res.target.topic].lookupChn; lookupChn != nil {
				lookupChn <- net.ticketStore.radius[res.target.topic].converged
			}
			net.ticketStore.searchLookupDone(res.target, res.nodes, func(n *Node, topic Topic) []byte {
				if n.state != nil && n.state.canQuery {
return net.conn.send(n, topicQueryPacket, topicQuery{Topic: topic}) //TODO:设置过期
				}
				if n.state == unknown {
					net.ping(n, n.addr())
				}
				return nil
			})

		case <-statsDump.C:
			log.Trace("<-statsDump.C")
   /*，确定：=net.ticketstore.radius[testTopic]
   如果！好吧{
    fmt.printf（“（%x）no radius@%v\n”，net.tab.self.id[：8]，time.now（））
   }否则{
    主题：=len（net.ticketstore.tickets）
    票据：=len（net.ticketstore.nodes）
    半径：=r.radius/（maxradius/10000+1）
    fmt.printf（“（%x）主题：%d半径：%d票证：%d@%v\n”，net.tab.self.id[：8]，主题，rad，票证，time.now（））
   */


			tm := mclock.Now()
			for topic, r := range net.ticketStore.radius {
				if printTestImgLogs {
					rad := r.radius / (maxRadius/1000000 + 1)
					minrad := r.minRadius / (maxRadius/1000000 + 1)
					fmt.Printf("*R %d %v %016x %v\n", tm/1000000, topic, net.tab.self.sha[:8], rad)
					fmt.Printf("*MR %d %v %016x %v\n", tm/1000000, topic, net.tab.self.sha[:8], minrad)
				}
			}
			for topic, t := range net.topictab.topics {
				wp := t.wcl.nextWaitPeriod(tm)
				if printTestImgLogs {
					fmt.Printf("*W %d %v %016x %d\n", tm/1000000, topic, net.tab.self.sha[:8], wp/1000000)
				}
			}

//定期/查找启动的存储桶刷新。
		case <-refreshTimer.C:
			log.Trace("<-refreshTimer.C")
//TODO:理想情况下，我们会在
//第一次设置了回退节点。
			if refreshDone == nil {
				refreshDone = make(chan struct{})
				net.refresh(refreshDone)
			}
		case <-bucketRefreshTimer.C:
			target := net.tab.chooseBucketRefreshTarget()
			go func() {
				net.lookup(target, false)
				bucketRefreshTimer.Reset(bucketRefreshInterval)
			}()
		case newNursery := <-net.refreshReq:
			log.Trace("<-net.refreshReq")
			if newNursery != nil {
				net.nursery = newNursery
			}
			if refreshDone == nil {
				refreshDone = make(chan struct{})
				net.refresh(refreshDone)
			}
			net.refreshResp <- refreshDone
		case <-refreshDone:
			log.Trace("<-net.refreshDone", "table size", net.tab.count)
			if net.tab.count != 0 {
				refreshDone = nil
				list := searchReqWhenRefreshDone
				searchReqWhenRefreshDone = nil
				go func() {
					for _, req := range list {
						net.topicSearchReq <- req
					}
				}()
			} else {
				refreshDone = make(chan struct{})
				net.refresh(refreshDone)
			}
		}
	}
	log.Trace("loop stopped")

	log.Debug(fmt.Sprintf("shutting down"))
	if net.conn != nil {
		net.conn.Close()
	}
	if refreshDone != nil {
//TODO:等待挂起的刷新。
//<-刷新结果
	}
//取消所有挂起的超时。
	for _, timer := range net.timeoutTimers {
		timer.Stop()
	}
	if net.db != nil {
		net.db.close()
	}
	close(net.closed)
}

//下面的所有内容都在network.loop goroutine上运行
//可随时修改节点、表和网络，无需锁定。

func (net *Network) refresh(done chan<- struct{}) {
	var seeds []*Node
	if net.db != nil {
		seeds = net.db.querySeeds(seedCount, seedMaxAge)
	}
	if len(seeds) == 0 {
		seeds = net.nursery
	}
	if len(seeds) == 0 {
		log.Trace("no seed nodes found")
		time.AfterFunc(time.Second*10, func() { close(done) })
		return
	}
	for _, n := range seeds {
		log.Debug("", "msg", log.Lazy{Fn: func() string {
			var age string
			if net.db != nil {
				age = time.Since(net.db.lastPong(n.ID)).String()
			} else {
				age = "unknown"
			}
			return fmt.Sprintf("seed node (age %s): %v", age, n)
		}})
		n = net.internNodeFromDB(n)
		if n.state == unknown {
			net.transition(n, verifyinit)
		}
//强制添加种子节点，以便查找可以执行某些操作。
//如果验证失败，将再次删除。
		net.tab.add(n)
	}
//开始自查找以填充存储桶。
	go func() {
		net.Lookup(net.tab.self.ID)
		close(done)
	}()
}

//节点交互。

func (net *Network) internNode(pkt *ingressPacket) *Node {
	if n := net.nodes[pkt.remoteID]; n != nil {
		n.IP = pkt.remoteAddr.IP
		n.UDP = uint16(pkt.remoteAddr.Port)
		n.TCP = uint16(pkt.remoteAddr.Port)
		return n
	}
	n := NewNode(pkt.remoteID, pkt.remoteAddr.IP, uint16(pkt.remoteAddr.Port), uint16(pkt.remoteAddr.Port))
	n.state = unknown
	net.nodes[pkt.remoteID] = n
	return n
}

func (net *Network) internNodeFromDB(dbn *Node) *Node {
	if n := net.nodes[dbn.ID]; n != nil {
		return n
	}
	n := NewNode(dbn.ID, dbn.IP, dbn.UDP, dbn.TCP)
	n.state = unknown
	net.nodes[n.ID] = n
	return n
}

func (net *Network) internNodeFromNeighbours(sender *net.UDPAddr, rn rpcNode) (n *Node, err error) {
	if rn.ID == net.tab.self.ID {
		return nil, errors.New("is self")
	}
	if rn.UDP <= lowPort {
		return nil, errors.New("low port")
	}
	n = net.nodes[rn.ID]
	if n == nil {
//我们以前没有见过这个节点。
		n, err = nodeFromRPC(sender, rn)
		if net.netrestrict != nil && !net.netrestrict.Contains(n.IP) {
			return n, errors.New("not contained in netrestrict whitelist")
		}
		if err == nil {
			n.state = unknown
			net.nodes[n.ID] = n
		}
		return n, err
	}
	if !n.IP.Equal(rn.IP) || n.UDP != rn.UDP || n.TCP != rn.TCP {
		if n.state == known {
//如果我们知道节点，则拒绝地址更改
			err = fmt.Errorf("metadata mismatch: got %v, want %v", rn, n)
		} else {
//否则接受；这将通过签署的ENR更好地处理。
			n.IP = rn.IP
			n.UDP = rn.UDP
			n.TCP = rn.TCP
		}
	}
	return n, err
}

//节点网嵌入在节点中，包含字段。
type nodeNetGuts struct {
//这是用于节点的sha3（id）的缓存副本
//距离计算。这是节点的一部分，以便
//可以编写在一定距离需要节点的测试。
//在这些测试中，sha的内容实际上并不对应
//和ID.
	sha common.Hash

//状态机字段。访问这些字段
//仅限于network.loop goroutine。
	state             *nodeState
pingEcho          []byte           //我们发送的最后一个ping的哈希
pingTopics        []Topic          //上次Ping中我们发送的主题集
deferredQueries   []*findnodeQuery //尚未发送的查询
pendingNeighbours *findnodeQuery   //当前查询，等待答复
	queryTimeouts     int
}

func (n *nodeNetGuts) deferQuery(q *findnodeQuery) {
	n.deferredQueries = append(n.deferredQueries, q)
}

func (n *nodeNetGuts) startNextQuery(net *Network) {
	if len(n.deferredQueries) == 0 {
		return
	}
	nextq := n.deferredQueries[0]
	if nextq.start(net) {
		n.deferredQueries = append(n.deferredQueries[:0], n.deferredQueries[1:]...)
	}
}

func (q *findnodeQuery) start(net *Network) bool {
//直接满足对本地节点的查询。
	if q.remote == net.tab.self {
		closest := net.tab.closest(q.target, bucketSize)
		q.reply <- closest.entries
		return true
	}
	if q.remote.state.canQuery && q.remote.pendingNeighbours == nil {
		net.conn.sendFindnodeHash(q.remote, q.target)
		net.timedEvent(respTimeout, q.remote, neighboursTimeout)
		q.remote.pendingNeighbours = q
		return true
	}
//如果还不知道节点，它将不接受查询。
//启动到已知的转换。
//稍后当节点达到已知状态时，将发送请求。
	if q.remote.state == unknown {
		net.transition(q.remote, verifyinit)
	}
	return false
}

//节点事件（状态机的输入）。

type nodeEvent uint

//go:生成字符串-type=nodeEvent

const (

//数据包类型事件。
//它们对应于UDP协议中的数据包类型。
	pingPacket = iota + 1
	pongPacket
	findnodePacket
	neighborsPacket
	findnodeHashPacket
	topicRegisterPacket
	topicQueryPacket
	topicNodesPacket

//非数据包事件。
//此类别中的事件值在外部分配
//数据包类型范围（数据包类型编码为单字节）。
	pongTimeout nodeEvent = iota + 256
	pingTimeout
	neighboursTimeout
)

//节点状态机。

type nodeState struct {
	name     string
	handle   func(*Network, *Node, nodeEvent, *ingressPacket) (next *nodeState, err error)
	enter    func(*Network, *Node)
	canQuery bool
}

func (s *nodeState) String() string {
	return s.name
}

var (
	unknown          *nodeState
	verifyinit       *nodeState
	verifywait       *nodeState
	remoteverifywait *nodeState
	known            *nodeState
	contested        *nodeState
	unresponsive     *nodeState
)

func init() {
	unknown = &nodeState{
		name: "unknown",
		enter: func(net *Network, n *Node) {
			net.tab.delete(n)
			n.pingEcho = nil
//中止活动查询。
			for _, q := range n.deferredQueries {
				q.reply <- nil
			}
			n.deferredQueries = nil
			if n.pendingNeighbours != nil {
				n.pendingNeighbours.reply <- nil
				n.pendingNeighbours = nil
			}
			n.queryTimeouts = 0
		},
		handle: func(net *Network, n *Node, ev nodeEvent, pkt *ingressPacket) (*nodeState, error) {
			switch ev {
			case pingPacket:
				net.handlePing(n, pkt)
				net.ping(n, pkt.remoteAddr)
				return verifywait, nil
			default:
				return unknown, errInvalidEvent
			}
		},
	}

	verifyinit = &nodeState{
		name: "verifyinit",
		enter: func(net *Network, n *Node) {
			net.ping(n, n.addr())
		},
		handle: func(net *Network, n *Node, ev nodeEvent, pkt *ingressPacket) (*nodeState, error) {
			switch ev {
			case pingPacket:
				net.handlePing(n, pkt)
				return verifywait, nil
			case pongPacket:
				err := net.handleKnownPong(n, pkt)
				return remoteverifywait, err
			case pongTimeout:
				return unknown, nil
			default:
				return verifyinit, errInvalidEvent
			}
		},
	}

	verifywait = &nodeState{
		name: "verifywait",
		handle: func(net *Network, n *Node, ev nodeEvent, pkt *ingressPacket) (*nodeState, error) {
			switch ev {
			case pingPacket:
				net.handlePing(n, pkt)
				return verifywait, nil
			case pongPacket:
				err := net.handleKnownPong(n, pkt)
				return known, err
			case pongTimeout:
				return unknown, nil
			default:
				return verifywait, errInvalidEvent
			}
		},
	}

	remoteverifywait = &nodeState{
		name: "remoteverifywait",
		enter: func(net *Network, n *Node) {
			net.timedEvent(respTimeout, n, pingTimeout)
		},
		handle: func(net *Network, n *Node, ev nodeEvent, pkt *ingressPacket) (*nodeState, error) {
			switch ev {
			case pingPacket:
				net.handlePing(n, pkt)
				return remoteverifywait, nil
			case pingTimeout:
				return known, nil
			default:
				return remoteverifywait, errInvalidEvent
			}
		},
	}

	known = &nodeState{
		name:     "known",
		canQuery: true,
		enter: func(net *Network, n *Node) {
			n.queryTimeouts = 0
			n.startNextQuery(net)
//插入到表中并开始最后一个节点的重新验证
//在桶里，如果满了。
			last := net.tab.add(n)
			if last != nil && last.state == known {
//TODO:异步执行此操作
				net.transition(last, contested)
			}
		},
		handle: func(net *Network, n *Node, ev nodeEvent, pkt *ingressPacket) (*nodeState, error) {
			switch ev {
			case pingPacket:
				net.handlePing(n, pkt)
				return known, nil
			case pongPacket:
				err := net.handleKnownPong(n, pkt)
				return known, err
			default:
				return net.handleQueryEvent(n, ev, pkt)
			}
		},
	}

	contested = &nodeState{
		name:     "contested",
		canQuery: true,
		enter: func(net *Network, n *Node) {
			net.ping(n, n.addr())
		},
		handle: func(net *Network, n *Node, ev nodeEvent, pkt *ingressPacket) (*nodeState, error) {
			switch ev {
			case pongPacket:
//节点仍处于活动状态。
				err := net.handleKnownPong(n, pkt)
				return known, err
			case pongTimeout:
				net.tab.deleteReplace(n)
				return unresponsive, nil
			case pingPacket:
				net.handlePing(n, pkt)
				return contested, nil
			default:
				return net.handleQueryEvent(n, ev, pkt)
			}
		},
	}

	unresponsive = &nodeState{
		name:     "unresponsive",
		canQuery: true,
		handle: func(net *Network, n *Node, ev nodeEvent, pkt *ingressPacket) (*nodeState, error) {
			switch ev {
			case pingPacket:
				net.handlePing(n, pkt)
				return known, nil
			case pongPacket:
				err := net.handleKnownPong(n, pkt)
				return known, err
			default:
				return net.handleQueryEvent(n, ev, pkt)
			}
		},
	}
}

//处理由n发送的包和与n相关的事件。
func (net *Network) handle(n *Node, ev nodeEvent, pkt *ingressPacket) error {
//fmt.println（“handle”，n.addr（）.string（），n.state，ev）
	if pkt != nil {
		if err := net.checkPacket(n, ev, pkt); err != nil {
//fmt.println（“检查错误：”，err）
			return err
		}
//在第一个到期之后启动后台到期goroutine
//沟通成功。如果后续调用
//已经在运行。我们在这里而不是其他地方做这个
//因此搜索种子节点也会考虑较旧的节点
//否则将由到期人删除。
		if net.db != nil {
			net.db.ensureExpirer()
		}
	}
	if n.state == nil {
n.state = unknown //？？？？
	}
	next, err := n.state.handle(net, n, ev, pkt)
	net.transition(n, next)
//fmt.println（“新状态：”，n.state）
	return err
}

func (net *Network) checkPacket(n *Node, ev nodeEvent, pkt *ingressPacket) error {
//重播预防检查。
	switch ev {
	case pingPacket, findnodeHashPacket, neighborsPacket:
//TODO:检查日期>看到的最后一个日期
//TODO:检查ping版本
	case pongPacket:
		if !bytes.Equal(pkt.data.(*pong).ReplyTok, n.pingEcho) {
//fmt.println（“pong reply token mismatch”）。
			return fmt.Errorf("pong reply token mismatch")
		}
		n.pingEcho = nil
	}
//地址验证。
//托多：理想情况下，我们将执行以下操作：
//-拒绝所有地址错误的数据包，ping除外。
//-对于使用新地址的ping，转换到verifywait，但保留
//上一个节点（带旧地址）周围。如果新的发现，
//把它换出来。
	return nil
}

func (net *Network) transition(n *Node, next *nodeState) {
	if n.state != next {
		n.state = next
		if next.enter != nil {
			next.enter(net, n)
		}
	}

//TODO:持久/非持久节点
}

func (net *Network) timedEvent(d time.Duration, n *Node, ev nodeEvent) {
	timeout := timeoutEvent{ev, n}
	net.timeoutTimers[timeout] = time.AfterFunc(d, func() {
		select {
		case net.timeout <- timeout:
		case <-net.closed:
		}
	})
}

func (net *Network) abortTimedEvent(n *Node, ev nodeEvent) {
	timer := net.timeoutTimers[timeoutEvent{ev, n}]
	if timer != nil {
		timer.Stop()
		delete(net.timeoutTimers, timeoutEvent{ev, n})
	}
}

func (net *Network) ping(n *Node, addr *net.UDPAddr) {
//fmt.println（“ping”，n.addr（）.string（），n.id.string（），n.sha.hex（））
	if n.pingEcho != nil || n.ID == net.tab.self.ID {
//fmt.println（“未发送”）
		return
	}
	log.Trace("Pinging remote node", "node", n.ID)
	n.pingTopics = net.ticketStore.regTopicSet()
	n.pingEcho = net.conn.sendPing(n, addr, n.pingTopics)
	net.timedEvent(respTimeout, n, pongTimeout)
}

func (net *Network) handlePing(n *Node, pkt *ingressPacket) {
	log.Trace("Handling remote ping", "node", n.ID)
	ping := pkt.data.(*ping)
	n.TCP = ping.From.TCP
	t := net.topictab.getTicket(n, ping.Topics)

	pong := &pong{
To:         makeEndpoint(n.addr(), n.TCP), //TODO:可能使用数据库中已知的TCP端口
		ReplyTok:   pkt.hash,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}
	ticketToPong(t, pong)
	net.conn.send(n, pongPacket, pong)
}

func (net *Network) handleKnownPong(n *Node, pkt *ingressPacket) error {
	log.Trace("Handling known pong", "node", n.ID)
	net.abortTimedEvent(n, pongTimeout)
	now := mclock.Now()
	ticket, err := pongToTicket(now, n.pingTopics, n, pkt)
	if err == nil {
//fmt.printf（“（%x）票据：%+v\n”，net.tab.self.id[：8]，pkt.data）
		net.ticketStore.addTicket(now, pkt.data.(*pong).ReplyTok, ticket)
	} else {
		log.Trace("Failed to convert pong to ticket", "err", err)
	}
	n.pingEcho = nil
	n.pingTopics = nil
	return err
}

func (net *Network) handleQueryEvent(n *Node, ev nodeEvent, pkt *ingressPacket) (*nodeState, error) {
	switch ev {
	case findnodePacket:
		target := crypto.Keccak256Hash(pkt.data.(*findnode).Target[:])
		results := net.tab.closest(target, bucketSize).entries
		net.conn.sendNeighbours(n, results)
		return n.state, nil
	case neighborsPacket:
		err := net.handleNeighboursPacket(n, pkt)
		return n.state, err
	case neighboursTimeout:
		if n.pendingNeighbours != nil {
			n.pendingNeighbours.reply <- nil
			n.pendingNeighbours = nil
		}
		n.queryTimeouts++
		if n.queryTimeouts > maxFindnodeFailures && n.state == known {
			return contested, errors.New("too many timeouts")
		}
		return n.state, nil

//V5

	case findnodeHashPacket:
		results := net.tab.closest(pkt.data.(*findnodeHash).Target, bucketSize).entries
		net.conn.sendNeighbours(n, results)
		return n.state, nil
	case topicRegisterPacket:
//fmt.println（“获取到MicroregisterPacket”）
		regdata := pkt.data.(*topicRegister)
		pong, err := net.checkTopicRegister(regdata)
		if err != nil {
//fmt.println（错误）
			return n.state, fmt.Errorf("bad waiting ticket: %v", err)
		}
		net.topictab.useTicket(n, pong.TicketSerial, regdata.Topics, int(regdata.Idx), pong.Expiration, pong.WaitPeriods)
		return n.state, nil
	case topicQueryPacket:
//TODO:句柄过期
		topic := pkt.data.(*topicQuery).Topic
		results := net.topictab.getEntries(topic)
		if _, ok := net.ticketStore.tickets[topic]; ok {
results = append(results, net.tab.self) //我们没有在自己的桌子上登记，但如果我们在做广告，也要把自己还给我们自己。
		}
		if len(results) > 10 {
			results = results[:10]
		}
		var hash common.Hash
		copy(hash[:], pkt.hash)
		net.conn.sendTopicNodes(n, hash, results)
		return n.state, nil
	case topicNodesPacket:
		p := pkt.data.(*topicNodes)
		if net.ticketStore.gotTopicNodes(n, p.Echo, p.Nodes) {
			n.queryTimeouts++
			if n.queryTimeouts > maxFindnodeFailures && n.state == known {
				return contested, errors.New("too many timeouts")
			}
		}
		return n.state, nil

	default:
		return n.state, errInvalidEvent
	}
}

func (net *Network) checkTopicRegister(data *topicRegister) (*pong, error) {
	var pongpkt ingressPacket
	if err := decodePacket(data.Pong, &pongpkt); err != nil {
		return nil, err
	}
	if pongpkt.ev != pongPacket {
		return nil, errors.New("is not pong packet")
	}
	if pongpkt.remoteID != net.tab.self.ID {
		return nil, errors.New("not signed by us")
	}
//检查我们以前授权的所有主题
//对方正在尝试注册。
	if rlpHash(data.Topics) != pongpkt.data.(*pong).TopicHash {
		return nil, errors.New("topic hash mismatch")
	}
	if data.Idx >= uint(len(data.Topics)) {
		return nil, errors.New("topic index out of range")
	}
	return pongpkt.data.(*pong), nil
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

func (net *Network) handleNeighboursPacket(n *Node, pkt *ingressPacket) error {
	if n.pendingNeighbours == nil {
		return errNoQuery
	}
	net.abortTimedEvent(n, neighboursTimeout)

	req := pkt.data.(*neighbors)
	nodes := make([]*Node, len(req.Nodes))
	for i, rn := range req.Nodes {
		nn, err := net.internNodeFromNeighbours(pkt.remoteAddr, rn)
		if err != nil {
			log.Debug(fmt.Sprintf("invalid neighbour (%v) from %x@%v: %v", rn.IP, n.ID[:8], pkt.remoteAddr, err))
			continue
		}
		nodes[i] = nn
//立即开始验证查询结果。
//这很快就填满了桌子。
//TODO:生成的数据包太多，可能是通过队列生成的。
		if nn.state == unknown {
			net.transition(nn, verifyinit)
		}
	}
//TODO:不要忽略第二个数据包
	n.pendingNeighbours.reply <- nodes
	n.pendingNeighbours = nil
//既然这个查询完成了，就开始下一个查询。
	n.startNextQuery(net)
	return nil
}
