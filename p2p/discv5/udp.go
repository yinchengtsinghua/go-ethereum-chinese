
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
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	"github.com/ethereum/go-ethereum/rlp"
)

const Version = 4

//错误
var (
	errPacketTooSmall = errors.New("too small")
	errBadPrefix      = errors.New("bad prefix")
	errTimeout        = errors.New("RPC timeout")
)

//超时
const (
	respTimeout = 500 * time.Millisecond
	expiration  = 20 * time.Second

driftThreshold = 10 * time.Second //警告用户前允许的时钟漂移
)

//RPC请求结构
type (
	ping struct {
		Version    uint
		From, To   rpcEndpoint
		Expiration uint64

//V5
		Topics []Topic

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

//V5
		TopicHash    common.Hash
		TicketSerial uint32
		WaitPeriods  []uint32

//忽略其他字段（为了向前兼容）。
		Rest []rlp.RawValue `rlp:"tail"`
	}

//findnode是对接近给定目标的节点的查询。
	findnode struct {
Target     NodeID //不需要是实际的公钥
		Expiration uint64
//忽略其他字段（为了向前兼容）。
		Rest []rlp.RawValue `rlp:"tail"`
	}

//findnode是对接近给定目标的节点的查询。
	findnodeHash struct {
		Target     common.Hash
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

	topicRegister struct {
		Topics []Topic
		Idx    uint
		Pong   []byte
	}

	topicQuery struct {
		Topic      Topic
		Expiration uint64
	}

//答复topicquery
	topicNodes struct {
		Echo  common.Hash
		Nodes []rpcNode
	}

	rpcNode struct {
IP  net.IP //IPv4的len 4或IPv6的len 16
UDP uint16 //用于发现协议
TCP uint16 //对于RLPX协议
		ID  NodeID
	}

	rpcEndpoint struct {
IP  net.IP //IPv4的len 4或IPv6的len 16
UDP uint16 //用于发现协议
TCP uint16 //对于RLPX协议
	}
)

var (
	versionPrefix     = []byte("temporary discovery v5")
	versionPrefixSize = len(versionPrefix)
	sigSize           = 520 / 8
headSize          = versionPrefixSize + sigSize //包帧数据空间
)

//邻居答复通过多个数据包发送到
//低于1280字节的限制。我们计算最大数
//通过填充一个包直到它变得太大。
var maxNeighbors = func() int {
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
			return n
		}
	}
}()

var maxTopicNodes = func() int {
	p := topicNodes{}
	maxSizeNode := rpcNode{IP: make(net.IP, 16), UDP: ^uint16(0), TCP: ^uint16(0)}
	for n := 0; ; n++ {
		p.Nodes = append(p.Nodes, maxSizeNode)
		size, _, err := rlp.EncodeToReader(p)
		if err != nil {
//如果发生这种情况，它将被单元测试捕获。
			panic("cannot encode: " + err.Error())
		}
		if headSize+size+1 >= 1280 {
			return n
		}
	}
}()

func makeEndpoint(addr *net.UDPAddr, tcpPort uint16) rpcEndpoint {
	ip := addr.IP.To4()
	if ip == nil {
		ip = addr.IP.To16()
	}
	return rpcEndpoint{IP: ip, UDP: uint16(addr.Port), TCP: tcpPort}
}

func (e1 rpcEndpoint) equal(e2 rpcEndpoint) bool {
	return e1.UDP == e2.UDP && e1.TCP == e2.TCP && e1.IP.Equal(e2.IP)
}

func nodeFromRPC(sender *net.UDPAddr, rn rpcNode) (*Node, error) {
	if err := netutil.CheckRelayIP(sender.IP, rn.IP); err != nil {
		return nil, err
	}
	n := NewNode(rn.ID, rn.IP, rn.UDP, rn.TCP)
	err := n.validateComplete()
	return n, err
}

func nodeToRPC(n *Node) rpcNode {
	return rpcNode{ID: n.ID, IP: n.IP, UDP: n.UDP, TCP: n.TCP}
}

type ingressPacket struct {
	remoteID   NodeID
	remoteAddr *net.UDPAddr
	ev         nodeEvent
	hash       []byte
data       interface{} //rpc结构之一
	rawData    []byte
}

type conn interface {
	ReadFromUDP(b []byte) (n int, addr *net.UDPAddr, err error)
	WriteToUDP(b []byte, addr *net.UDPAddr) (n int, err error)
	Close() error
	LocalAddr() net.Addr
}

//UDP实现RPC协议。
type udp struct {
	conn        conn
	priv        *ecdsa.PrivateKey
	ourEndpoint rpcEndpoint
	nat         nat.Interface
	net         *Network
}

//listenudp返回一个新表，用于侦听laddr上的udp包。
func ListenUDP(priv *ecdsa.PrivateKey, conn conn, nodeDBPath string, netrestrict *netutil.Netlist) (*Network, error) {
	realaddr := conn.LocalAddr().(*net.UDPAddr)
	transport, err := listenUDP(priv, conn, realaddr)
	if err != nil {
		return nil, err
	}
	net, err := newNetwork(transport, priv.PublicKey, nodeDBPath, netrestrict)
	if err != nil {
		return nil, err
	}
	log.Info("UDP listener up", "net", net.tab.self)
	transport.net = net
	go transport.readLoop()
	return net, nil
}

func listenUDP(priv *ecdsa.PrivateKey, conn conn, realaddr *net.UDPAddr) (*udp, error) {
	return &udp{conn: conn, priv: priv, ourEndpoint: makeEndpoint(realaddr, uint16(realaddr.Port))}, nil
}

func (t *udp) localAddr() *net.UDPAddr {
	return t.conn.LocalAddr().(*net.UDPAddr)
}

func (t *udp) Close() {
	t.conn.Close()
}

func (t *udp) send(remote *Node, ptype nodeEvent, data interface{}) (hash []byte) {
	hash, _ = t.sendPacket(remote.ID, remote.addr(), byte(ptype), data)
	return hash
}

func (t *udp) sendPing(remote *Node, toaddr *net.UDPAddr, topics []Topic) (hash []byte) {
	hash, _ = t.sendPacket(remote.ID, toaddr, byte(pingPacket), ping{
		Version:    Version,
		From:       t.ourEndpoint,
To:         makeEndpoint(toaddr, uint16(toaddr.Port)), //TODO:可能使用数据库中已知的TCP端口
		Expiration: uint64(time.Now().Add(expiration).Unix()),
		Topics:     topics,
	})
	return hash
}

func (t *udp) sendFindnode(remote *Node, target NodeID) {
	t.sendPacket(remote.ID, remote.addr(), byte(findnodePacket), findnode{
		Target:     target,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	})
}

func (t *udp) sendNeighbours(remote *Node, results []*Node) {
//以块形式发送邻居，每个数据包最多有maxneighbors
//低于1280字节的限制。
	p := neighbors{Expiration: uint64(time.Now().Add(expiration).Unix())}
	for i, result := range results {
		p.Nodes = append(p.Nodes, nodeToRPC(result))
		if len(p.Nodes) == maxNeighbors || i == len(results)-1 {
			t.sendPacket(remote.ID, remote.addr(), byte(neighborsPacket), p)
			p.Nodes = p.Nodes[:0]
		}
	}
}

func (t *udp) sendFindnodeHash(remote *Node, target common.Hash) {
	t.sendPacket(remote.ID, remote.addr(), byte(findnodeHashPacket), findnodeHash{
		Target:     target,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	})
}

func (t *udp) sendTopicRegister(remote *Node, topics []Topic, idx int, pong []byte) {
	t.sendPacket(remote.ID, remote.addr(), byte(topicRegisterPacket), topicRegister{
		Topics: topics,
		Idx:    uint(idx),
		Pong:   pong,
	})
}

func (t *udp) sendTopicNodes(remote *Node, queryHash common.Hash, nodes []*Node) {
	p := topicNodes{Echo: queryHash}
	var sent bool
	for _, result := range nodes {
		if result.IP.Equal(t.net.tab.self.IP) || netutil.CheckRelayIP(remote.IP, result.IP) == nil {
			p.Nodes = append(p.Nodes, nodeToRPC(result))
		}
		if len(p.Nodes) == maxTopicNodes {
			t.sendPacket(remote.ID, remote.addr(), byte(topicNodesPacket), p)
			p.Nodes = p.Nodes[:0]
			sent = true
		}
	}
	if !sent || len(p.Nodes) > 0 {
		t.sendPacket(remote.ID, remote.addr(), byte(topicNodesPacket), p)
	}
}

func (t *udp) sendPacket(toid NodeID, toaddr *net.UDPAddr, ptype byte, req interface{}) (hash []byte, err error) {
//fmt.println（“发送包”，nodeEvent（ptype），toaddr.string（），toid.string（））
	packet, hash, err := encodePacket(t.priv, ptype, req)
	if err != nil {
//fmt.println（错误）
		return hash, err
	}
	log.Trace(fmt.Sprintf(">>> %v to %x@%v", nodeEvent(ptype), toid[:8], toaddr))
	if nbytes, err := t.conn.WriteToUDP(packet, toaddr); err != nil {
		log.Trace(fmt.Sprint("UDP send failed:", err))
	} else {
		egressTrafficMeter.Mark(int64(nbytes))
	}
//fmt.println（错误）
	return hash, err
}

//编码包的零填充空间。
var headSpace = make([]byte, headSize)

func encodePacket(priv *ecdsa.PrivateKey, ptype byte, req interface{}) (p, hash []byte, err error) {
	b := new(bytes.Buffer)
	b.Write(headSpace)
	b.WriteByte(ptype)
	if err := rlp.Encode(b, req); err != nil {
		log.Error(fmt.Sprint("error encoding packet:", err))
		return nil, nil, err
	}
	packet := b.Bytes()
	sig, err := crypto.Sign(crypto.Keccak256(packet[headSize:]), priv)
	if err != nil {
		log.Error(fmt.Sprint("could not sign packet:", err))
		return nil, nil, err
	}
	copy(packet, versionPrefix)
	copy(packet[versionPrefixSize:], sig)
	hash = crypto.Keccak256(packet[versionPrefixSize:])
	return packet, hash, nil
}

//readloop在自己的goroutine中运行。它注入入口UDP包
//进入网络循环。
func (t *udp) readLoop() {
	defer t.conn.Close()
//发现数据包被定义为不大于1280字节。
//大于此尺寸的包装将在末端切割并处理
//因为它们的哈希不匹配而无效。
	buf := make([]byte, 1280)
	for {
		nbytes, from, err := t.conn.ReadFromUDP(buf)
		ingressTrafficMeter.Mark(int64(nbytes))
		if netutil.IsTemporaryError(err) {
//忽略临时读取错误。
			log.Debug(fmt.Sprintf("Temporary read error: %v", err))
			continue
		} else if err != nil {
//关闭永久错误循环。
			log.Debug(fmt.Sprintf("Read error: %v", err))
			return
		}
		t.handlePacket(from, buf[:nbytes])
	}
}

func (t *udp) handlePacket(from *net.UDPAddr, buf []byte) error {
	pkt := ingressPacket{remoteAddr: from}
	if err := decodePacket(buf, &pkt); err != nil {
		log.Debug(fmt.Sprintf("Bad packet from %v: %v", from, err))
//fmt.println（“坏包”，err）
		return err
	}
	t.net.reqReadPacket(pkt)
	return nil
}

func decodePacket(buffer []byte, pkt *ingressPacket) error {
	if len(buffer) < headSize+1 {
		return errPacketTooSmall
	}
	buf := make([]byte, len(buffer))
	copy(buf, buffer)
	prefix, sig, sigdata := buf[:versionPrefixSize], buf[versionPrefixSize:headSize], buf[headSize:]
	if !bytes.Equal(prefix, versionPrefix) {
		return errBadPrefix
	}
	fromID, err := recoverNodeID(crypto.Keccak256(buf[headSize:]), sig)
	if err != nil {
		return err
	}
	pkt.rawData = buf
	pkt.hash = crypto.Keccak256(buf[versionPrefixSize:])
	pkt.remoteID = fromID
	switch pkt.ev = nodeEvent(sigdata[0]); pkt.ev {
	case pingPacket:
		pkt.data = new(ping)
	case pongPacket:
		pkt.data = new(pong)
	case findnodePacket:
		pkt.data = new(findnode)
	case neighborsPacket:
		pkt.data = new(neighbors)
	case findnodeHashPacket:
		pkt.data = new(findnodeHash)
	case topicRegisterPacket:
		pkt.data = new(topicRegister)
	case topicQueryPacket:
		pkt.data = new(topicQuery)
	case topicNodesPacket:
		pkt.data = new(topicNodes)
	default:
		return fmt.Errorf("unknown packet type: %d", sigdata[0])
	}
	s := rlp.NewStream(bytes.NewReader(sigdata[1:]), 0)
	err = s.Decode(pkt.data)
	return err
}
