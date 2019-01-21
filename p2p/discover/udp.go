
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

package discover

import (
	"bytes"
	"container/list"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	"github.com/ethereum/go-ethereum/rlp"
)

//错误
var (
	errPacketTooSmall   = errors.New("too small")
	errBadHash          = errors.New("bad hash")
	errExpired          = errors.New("expired")
	errUnsolicitedReply = errors.New("unsolicited reply")
	errUnknownNode      = errors.New("unknown node")
	errTimeout          = errors.New("RPC timeout")
	errClockWarp        = errors.New("reply deadline too far in the future")
	errClosed           = errors.New("socket closed")
)

//超时
const (
	respTimeout    = 500 * time.Millisecond
	expiration     = 20 * time.Second
	bondExpiration = 24 * time.Hour

ntpFailureThreshold = 32               //连续超时，之后检查NTP
ntpWarningCooldown  = 10 * time.Minute //重复NTP警告之前要经过的最短时间
driftThreshold      = 10 * time.Second //警告用户前允许的时钟漂移
)

//RPC数据包类型
const (
pingPacket = iota + 1 //零为“保留”
	pongPacket
	findnodePacket
	neighborsPacket
)

//RPC请求结构
type (
	ping struct {
		Version    uint
		From, To   rpcEndpoint
		Expiration uint64
//忽略其他字段（为了向前兼容）。
		Rest []rlp.RawValue `rlp:"tail"`
	}

//乒乓球是对乒乓球的回应。
	pong struct {
//此字段应镜像UDP信封地址
//提供了一种发现
//外部地址（在NAT之后）。
		To rpcEndpoint

ReplyTok   []byte //这包含ping包的哈希。
Expiration uint64 //数据包失效的绝对时间戳。
//忽略其他字段（为了向前兼容）。
		Rest []rlp.RawValue `rlp:"tail"`
	}

//findnode是对接近给定目标的节点的查询。
	findnode struct {
		Target     encPubkey
		Expiration uint64
//忽略其他字段（为了向前兼容）。
		Rest []rlp.RawValue `rlp:"tail"`
	}

//回复findnode
	neighbors struct {
		Nodes      []rpcNode
		Expiration uint64
//忽略其他字段（为了向前兼容）。
		Rest []rlp.RawValue `rlp:"tail"`
	}

	rpcNode struct {
IP  net.IP //IPv4的len 4或IPv6的len 16
UDP uint16 //用于发现协议
TCP uint16 //对于RLPX协议
		ID  encPubkey
	}

	rpcEndpoint struct {
IP  net.IP //IPv4的len 4或IPv6的len 16
UDP uint16 //用于发现协议
TCP uint16 //对于RLPX协议
	}
)

func makeEndpoint(addr *net.UDPAddr, tcpPort uint16) rpcEndpoint {
	ip := net.IP{}
	if ip4 := addr.IP.To4(); ip4 != nil {
		ip = ip4
	} else if ip6 := addr.IP.To16(); ip6 != nil {
		ip = ip6
	}
	return rpcEndpoint{IP: ip, UDP: uint16(addr.Port), TCP: tcpPort}
}

func (t *udp) nodeFromRPC(sender *net.UDPAddr, rn rpcNode) (*node, error) {
	if rn.UDP <= 1024 {
		return nil, errors.New("low port")
	}
	if err := netutil.CheckRelayIP(sender.IP, rn.IP); err != nil {
		return nil, err
	}
	if t.netrestrict != nil && !t.netrestrict.Contains(rn.IP) {
		return nil, errors.New("not contained in netrestrict whitelist")
	}
	key, err := decodePubkey(rn.ID)
	if err != nil {
		return nil, err
	}
	n := wrapNode(enode.NewV4(key, rn.IP, int(rn.TCP), int(rn.UDP)))
	err = n.ValidateComplete()
	return n, err
}

func nodeToRPC(n *node) rpcNode {
	var key ecdsa.PublicKey
	var ekey encPubkey
	if err := n.Load((*enode.Secp256k1)(&key)); err == nil {
		ekey = encodePubkey(&key)
	}
	return rpcNode{ID: ekey, IP: n.IP(), UDP: uint16(n.UDP()), TCP: uint16(n.TCP())}
}

type packet interface {
	handle(t *udp, from *net.UDPAddr, fromKey encPubkey, mac []byte) error
	name() string
}

type conn interface {
	ReadFromUDP(b []byte) (n int, addr *net.UDPAddr, err error)
	WriteToUDP(b []byte, addr *net.UDPAddr) (n int, err error)
	Close() error
	LocalAddr() net.Addr
}

//UDP实现Discovery v4 UDP有线协议。
type udp struct {
	conn        conn
	netrestrict *netutil.Netlist
	priv        *ecdsa.PrivateKey
	localNode   *enode.LocalNode
	db          *enode.DB
	tab         *Table
	wg          sync.WaitGroup

	addpending chan *pending
	gotreply   chan reply
	closing    chan struct{}
}

//挂起表示挂起的答复。
//
//协议的某些实现希望发送多个
//将数据包回复到findnode。一般来说，任何邻居包都不能
//与特定的findnode包匹配。
//
//我们的实现通过存储
//每个等待答复。来自节点的传入数据包被调度
//到该节点的所有回调函数。
type pending struct {
//这些字段必须在答复中匹配。
	from  enode.ID
	ptype byte

//请求必须完成的时间
	deadline time.Time

//当匹配的答复到达时调用回调。如果它回来
//如果为true，则从挂起的答复队列中删除回调。
//如果返回错误，则认为答复不完整，并且
//将为下一个匹配的答复再次调用回调。
	callback func(resp interface{}) (done bool)

//当回调指示完成或
//如果在超时时间内没有收到进一步的答复，则出错。
	errc chan<- error
}

type reply struct {
	from  enode.ID
	ptype byte
	data  interface{}
//循环指示是否存在
//通过此频道发送的匹配请求。
	matched chan<- bool
}

//无法处理readpacket时，会将其发送到未处理的通道。
type ReadPacket struct {
	Data []byte
	Addr *net.UDPAddr
}

//配置保存与表相关的设置。
type Config struct {
//需要这些设置并配置UDP侦听器：
	PrivateKey *ecdsa.PrivateKey

//这些设置是可选的：
NetRestrict *netutil.Netlist  //网络白名单
Bootnodes   []*enode.Node     //引导程序节点列表
Unhandled   chan<- ReadPacket //在此通道上发送未处理的数据包
}

//listenudp返回一个新表，用于侦听laddr上的udp包。
func ListenUDP(c conn, ln *enode.LocalNode, cfg Config) (*Table, error) {
	tab, _, err := newUDP(c, ln, cfg)
	if err != nil {
		return nil, err
	}
	return tab, nil
}

func newUDP(c conn, ln *enode.LocalNode, cfg Config) (*Table, *udp, error) {
	udp := &udp{
		conn:        c,
		priv:        cfg.PrivateKey,
		netrestrict: cfg.NetRestrict,
		localNode:   ln,
		db:          ln.Database(),
		closing:     make(chan struct{}),
		gotreply:    make(chan reply),
		addpending:  make(chan *pending),
	}
	tab, err := newTable(udp, ln.Database(), cfg.Bootnodes)
	if err != nil {
		return nil, nil, err
	}
	udp.tab = tab

	udp.wg.Add(2)
	go udp.loop()
	go udp.readLoop(cfg.Unhandled)
	return udp.tab, udp, nil
}

func (t *udp) self() *enode.Node {
	return t.localNode.Node()
}

func (t *udp) close() {
	close(t.closing)
	t.conn.Close()
	t.wg.Wait()
}

func (t *udp) ourEndpoint() rpcEndpoint {
	n := t.self()
	a := &net.UDPAddr{IP: n.IP(), Port: n.UDP()}
	return makeEndpoint(a, uint16(n.TCP()))
}

//ping向给定节点发送ping消息并等待答复。
func (t *udp) ping(toid enode.ID, toaddr *net.UDPAddr) error {
	return <-t.sendPing(toid, toaddr, nil)
}

//发送ping向给定节点发送ping消息并调用回调
//当回复到达时。
func (t *udp) sendPing(toid enode.ID, toaddr *net.UDPAddr, callback func()) <-chan error {
	req := &ping{
		Version:    4,
		From:       t.ourEndpoint(),
To:         makeEndpoint(toaddr, 0), //TODO:可能使用数据库中已知的TCP端口
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}
	packet, hash, err := encodePacket(t.priv, pingPacket, req)
	if err != nil {
		errc := make(chan error, 1)
		errc <- err
		return errc
	}
	errc := t.pending(toid, pongPacket, func(p interface{}) bool {
		ok := bytes.Equal(p.(*pong).ReplyTok, hash)
		if ok && callback != nil {
			callback()
		}
		return ok
	})
	t.localNode.UDPContact(toaddr)
	t.write(toaddr, req.name(), packet)
	return errc
}

func (t *udp) waitping(from enode.ID) error {
	return <-t.pending(from, pingPacket, func(interface{}) bool { return true })
}

//findnode向给定节点发送findnode请求，并等待直到
//节点已发送到k个邻居。
func (t *udp) findnode(toid enode.ID, toaddr *net.UDPAddr, target encPubkey) ([]*node, error) {
//如果我们有一段时间没有看到目标节点的ping，它将不会记得
//我们的端点证明和拒绝findnode。先打个乒乓球。
	if time.Since(t.db.LastPingReceived(toid)) > bondExpiration {
		t.ping(toid, toaddr)
		t.waitping(toid)
	}

	nodes := make([]*node, 0, bucketSize)
	nreceived := 0
	errc := t.pending(toid, neighborsPacket, func(r interface{}) bool {
		reply := r.(*neighbors)
		for _, rn := range reply.Nodes {
			nreceived++
			n, err := t.nodeFromRPC(toaddr, rn)
			if err != nil {
				log.Trace("Invalid neighbor node received", "ip", rn.IP, "addr", toaddr, "err", err)
				continue
			}
			nodes = append(nodes, n)
		}
		return nreceived >= bucketSize
	})
	t.send(toaddr, findnodePacket, &findnode{
		Target:     target,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	})
	return nodes, <-errc
}

//挂起向挂起的答复队列添加答复回调。
//有关详细说明，请参阅“挂起”类型的文档。
func (t *udp) pending(id enode.ID, ptype byte, callback func(interface{}) bool) <-chan error {
	ch := make(chan error, 1)
	p := &pending{from: id, ptype: ptype, callback: callback, errc: ch}
	select {
	case t.addpending <- p:
//循环将处理它
	case <-t.closing:
		ch <- errClosed
	}
	return ch
}

func (t *udp) handleReply(from enode.ID, ptype byte, req packet) bool {
	matched := make(chan bool, 1)
	select {
	case t.gotreply <- reply{from, ptype, req, matched}:
//循环将处理它
		return <-matched
	case <-t.closing:
		return false
	}
}

//循环在自己的Goroutine中运行。它跟踪
//刷新计时器和挂起的答复队列。
func (t *udp) loop() {
	defer t.wg.Done()

	var (
		plist        = list.New()
		timeout      = time.NewTimer(0)
nextTimeout  *pending //上次重置超时时的plist头
contTimeouts = 0      //要执行NTP检查的连续超时数
		ntpWarnTime  = time.Unix(0, 0)
	)
<-timeout.C //忽略第一次超时
	defer timeout.Stop()

	resetTimeout := func() {
		if plist.Front() == nil || nextTimeout == plist.Front().Value {
			return
		}
//启动计时器，以便在下一个挂起的答复过期时触发。
		now := time.Now()
		for el := plist.Front(); el != nil; el = el.Next() {
			nextTimeout = el.Value.(*pending)
			if dist := nextTimeout.deadline.Sub(now); dist < 2*respTimeout {
				timeout.Reset(dist)
				return
			}
//删除截止时间太长的挂起答复
//未来。如果系统时钟跳变，就会发生这种情况。
//在最后期限被分配后向后。
			nextTimeout.errc <- errClockWarp
			plist.Remove(el)
		}
		nextTimeout = nil
		timeout.Stop()
	}

	for {
		resetTimeout()

		select {
		case <-t.closing:
			for el := plist.Front(); el != nil; el = el.Next() {
				el.Value.(*pending).errc <- errClosed
			}
			return

		case p := <-t.addpending:
			p.deadline = time.Now().Add(respTimeout)
			plist.PushBack(p)

		case r := <-t.gotreply:
			var matched bool
			for el := plist.Front(); el != nil; el = el.Next() {
				p := el.Value.(*pending)
				if p.from == r.from && p.ptype == r.ptype {
					matched = true
//如果Matcher的回调指示
//所有答复都已收到。这是
//需要多个数据包类型
//应答包。
					if p.callback(r.data) {
						p.errc <- nil
						plist.Remove(el)
					}
//重置连续超时计数器（时间漂移检测）
					contTimeouts = 0
				}
			}
			r.matched <- matched

		case now := <-timeout.C:
			nextTimeout = nil

//通知并删除期限已过的回调。
			for el := plist.Front(); el != nil; el = el.Next() {
				p := el.Value.(*pending)
				if now.After(p.deadline) || now.Equal(p.deadline) {
					p.errc <- errTimeout
					plist.Remove(el)
					contTimeouts++
				}
			}
//如果我们累积了太多超时，请执行NTP时间同步检查
			if contTimeouts > ntpFailureThreshold {
				if time.Since(ntpWarnTime) >= ntpWarningCooldown {
					ntpWarnTime = time.Now()
					go checkClockDrift()
				}
				contTimeouts = 0
			}
		}
	}
}

const (
	macSize  = 256 / 8
	sigSize  = 520 / 8
headSize = macSize + sigSize //包帧数据空间
)

var (
	headSpace = make([]byte, headSize)

//邻居答复通过多个数据包发送到
//低于1280字节的限制。我们计算最大数
//通过填充一个包直到它变得太大。
	maxNeighbors int
)

func init() {
	p := neighbors{Expiration: ^uint64(0)}
	maxSizeNode := rpcNode{IP: make(net.IP, 16), UDP: ^uint16(0), TCP: ^uint16(0)}
	for n := 0; ; n++ {
		p.Nodes = append(p.Nodes, maxSizeNode)
		size, _, err := rlp.EncodeToReader(p)
		if err != nil {
//如果发生这种情况，它将被单元测试捕获。
			panic("cannot encode: " + err.Error())
		}
		if headSize+size+1 >= 1280 {
			maxNeighbors = n
			break
		}
	}
}

func (t *udp) send(toaddr *net.UDPAddr, ptype byte, req packet) ([]byte, error) {
	packet, hash, err := encodePacket(t.priv, ptype, req)
	if err != nil {
		return hash, err
	}
	return hash, t.write(toaddr, req.name(), packet)
}

func (t *udp) write(toaddr *net.UDPAddr, what string, packet []byte) error {
	_, err := t.conn.WriteToUDP(packet, toaddr)
	log.Trace(">> "+what, "addr", toaddr, "err", err)
	return err
}

func encodePacket(priv *ecdsa.PrivateKey, ptype byte, req interface{}) (packet, hash []byte, err error) {
	b := new(bytes.Buffer)
	b.Write(headSpace)
	b.WriteByte(ptype)
	if err := rlp.Encode(b, req); err != nil {
		log.Error("Can't encode discv4 packet", "err", err)
		return nil, nil, err
	}
	packet = b.Bytes()
	sig, err := crypto.Sign(crypto.Keccak256(packet[headSize:]), priv)
	if err != nil {
		log.Error("Can't sign discv4 packet", "err", err)
		return nil, nil, err
	}
	copy(packet[macSize:], sig)
//将哈希添加到前面。注意：这不保护
//以任何方式打包。我们的公钥将是这个哈希的一部分
//未来。
	hash = crypto.Keccak256(packet[macSize:])
	copy(packet, hash)
	return packet, hash, nil
}

//readloop在自己的goroutine中运行。它处理传入的UDP数据包。
func (t *udp) readLoop(unhandled chan<- ReadPacket) {
	defer t.wg.Done()
	if unhandled != nil {
		defer close(unhandled)
	}

//发现数据包被定义为不大于1280字节。
//大于此尺寸的包装将在末端切割并处理
//因为它们的哈希不匹配而无效。
	buf := make([]byte, 1280)
	for {
		nbytes, from, err := t.conn.ReadFromUDP(buf)
		if netutil.IsTemporaryError(err) {
//忽略临时读取错误。
			log.Debug("Temporary UDP read error", "err", err)
			continue
		} else if err != nil {
//关闭永久错误循环。
			log.Debug("UDP read error", "err", err)
			return
		}
		if t.handlePacket(from, buf[:nbytes]) != nil && unhandled != nil {
			select {
			case unhandled <- ReadPacket{buf[:nbytes], from}:
			default:
			}
		}
	}
}

func (t *udp) handlePacket(from *net.UDPAddr, buf []byte) error {
	packet, fromID, hash, err := decodePacket(buf)
	if err != nil {
		log.Debug("Bad discv4 packet", "addr", from, "err", err)
		return err
	}
	err = packet.handle(t, from, fromID, hash)
	log.Trace("<< "+packet.name(), "addr", from, "err", err)
	return err
}

func decodePacket(buf []byte) (packet, encPubkey, []byte, error) {
	if len(buf) < headSize+1 {
		return nil, encPubkey{}, nil, errPacketTooSmall
	}
	hash, sig, sigdata := buf[:macSize], buf[macSize:headSize], buf[headSize:]
	shouldhash := crypto.Keccak256(buf[macSize:])
	if !bytes.Equal(hash, shouldhash) {
		return nil, encPubkey{}, nil, errBadHash
	}
	fromKey, err := recoverNodeKey(crypto.Keccak256(buf[headSize:]), sig)
	if err != nil {
		return nil, fromKey, hash, err
	}

	var req packet
	switch ptype := sigdata[0]; ptype {
	case pingPacket:
		req = new(ping)
	case pongPacket:
		req = new(pong)
	case findnodePacket:
		req = new(findnode)
	case neighborsPacket:
		req = new(neighbors)
	default:
		return nil, fromKey, hash, fmt.Errorf("unknown type: %d", ptype)
	}
	s := rlp.NewStream(bytes.NewReader(sigdata[1:]), 0)
	err = s.Decode(req)
	return req, fromKey, hash, err
}

func (req *ping) handle(t *udp, from *net.UDPAddr, fromKey encPubkey, mac []byte) error {
	if expired(req.Expiration) {
		return errExpired
	}
	key, err := decodePubkey(fromKey)
	if err != nil {
		return fmt.Errorf("invalid public key: %v", err)
	}
	t.send(from, pongPacket, &pong{
		To:         makeEndpoint(from, req.From.TCP),
		ReplyTok:   mac,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	})
	n := wrapNode(enode.NewV4(key, from.IP, int(req.From.TCP), from.Port))
	t.handleReply(n.ID(), pingPacket, req)
	if time.Since(t.db.LastPongReceived(n.ID())) > bondExpiration {
		t.sendPing(n.ID(), from, func() { t.tab.addThroughPing(n) })
	} else {
		t.tab.addThroughPing(n)
	}
	t.localNode.UDPEndpointStatement(from, &net.UDPAddr{IP: req.To.IP, Port: int(req.To.UDP)})
	t.db.UpdateLastPingReceived(n.ID(), time.Now())
	return nil
}

func (req *ping) name() string { return "PING/v4" }

func (req *pong) handle(t *udp, from *net.UDPAddr, fromKey encPubkey, mac []byte) error {
	if expired(req.Expiration) {
		return errExpired
	}
	fromID := fromKey.id()
	if !t.handleReply(fromID, pongPacket, req) {
		return errUnsolicitedReply
	}
	t.localNode.UDPEndpointStatement(from, &net.UDPAddr{IP: req.To.IP, Port: int(req.To.UDP)})
	t.db.UpdateLastPongReceived(fromID, time.Now())
	return nil
}

func (req *pong) name() string { return "PONG/v4" }

func (req *findnode) handle(t *udp, from *net.UDPAddr, fromKey encPubkey, mac []byte) error {
	if expired(req.Expiration) {
		return errExpired
	}
	fromID := fromKey.id()
	if time.Since(t.db.LastPongReceived(fromID)) > bondExpiration {
//不存在端点验证pong，我们不处理数据包。这可以防止
//攻击向量，发现协议可用于放大
//DDoS攻击。恶意参与者将使用IP地址发送findnode请求
//目标的UDP端口作为源地址。findnode的接收者
//然后，包将发送一个邻居包（比
//找到受害者。
		return errUnknownNode
	}
	target := enode.ID(crypto.Keccak256Hash(req.Target[:]))
	t.tab.mutex.Lock()
	closest := t.tab.closest(target, bucketSize).entries
	t.tab.mutex.Unlock()

	p := neighbors{Expiration: uint64(time.Now().Add(expiration).Unix())}
	var sent bool
//以块形式发送邻居，每个数据包最多有maxneighbors
//低于1280字节的限制。
	for _, n := range closest {
		if netutil.CheckRelayIP(from.IP, n.IP()) == nil {
			p.Nodes = append(p.Nodes, nodeToRPC(n))
		}
		if len(p.Nodes) == maxNeighbors {
			t.send(from, neighborsPacket, &p)
			p.Nodes = p.Nodes[:0]
			sent = true
		}
	}
	if len(p.Nodes) > 0 || !sent {
		t.send(from, neighborsPacket, &p)
	}
	return nil
}

func (req *findnode) name() string { return "FINDNODE/v4" }

func (req *neighbors) handle(t *udp, from *net.UDPAddr, fromKey encPubkey, mac []byte) error {
	if expired(req.Expiration) {
		return errExpired
	}
	if !t.handleReply(fromKey.id(), neighborsPacket, req) {
		return errUnsolicitedReply
	}
	return nil
}

func (req *neighbors) name() string { return "NEIGHBORS/v4" }

func expired(ts uint64) bool {
	return time.Unix(int64(ts), 0).Before(time.Now())
}
