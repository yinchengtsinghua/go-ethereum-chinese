
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

package p2p

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	ErrShuttingDown = errors.New("shutting down")
)

const (
	baseProtocolVersion    = 5
	baseProtocolLength     = uint64(16)
	baseProtocolMaxMsgSize = 2 * 1024

	snappyProtocolVersion = 5

	pingInterval = 15 * time.Second
)

const (
//DEVP2P消息代码
	handshakeMsg = 0x00
	discMsg      = 0x01
	pingMsg      = 0x02
	pongMsg      = 0x03
)

//协议握手是协议握手的RLP结构。
type protoHandshake struct {
	Version    uint64
	Name       string
	Caps       []Cap
	ListenPort uint64
ID         []byte //secp256k1公钥

//忽略其他字段（为了向前兼容）。
	Rest []rlp.RawValue `rlp:"tail"`
}

//PeerEventType是P2P服务器发出的对等事件类型。
type PeerEventType string

const (
//PeerEventTypeAdd是添加对等方时发出的事件类型
//到P2P服务器
	PeerEventTypeAdd PeerEventType = "add"

//PeerEventTypeDrop是当对等机
//从p2p服务器上删除
	PeerEventTypeDrop PeerEventType = "drop"

//PeerEventTypeMSGSEND是在
//消息已成功发送到对等方
	PeerEventTypeMsgSend PeerEventType = "msgsend"

//PeerEventTypeMsgrecv是在
//从对等端接收消息
	PeerEventTypeMsgRecv PeerEventType = "msgrecv"
)

//PeerEvent是在添加或删除对等方时发出的事件
//一个p2p服务器，或者当通过对等连接发送或接收消息时
type PeerEvent struct {
	Type     PeerEventType `json:"type"`
	Peer     enode.ID      `json:"peer"`
	Error    string        `json:"error,omitempty"`
	Protocol string        `json:"protocol,omitempty"`
	MsgCode  *uint64       `json:"msg_code,omitempty"`
	MsgSize  *uint32       `json:"msg_size,omitempty"`
}

//对等表示已连接的远程节点。
type Peer struct {
	rw      *conn
	running map[string]*protoRW
	log     log.Logger
	created mclock.AbsTime

	wg       sync.WaitGroup
	protoErr chan error
	closed   chan struct{}
	disc     chan DiscReason

//事件接收消息发送/接收事件（如果设置）
	events *event.Feed
}

//newpeer返回用于测试目的的对等机。
func NewPeer(id enode.ID, name string, caps []Cap) *Peer {
	pipe, _ := net.Pipe()
	node := enode.SignNull(new(enr.Record), id)
	conn := &conn{fd: pipe, transport: nil, node: node, caps: caps, name: name}
	peer := newPeer(conn, nil)
close(peer.closed) //确保断开连接不会阻塞
	return peer
}

//ID返回节点的公钥。
func (p *Peer) ID() enode.ID {
	return p.rw.node.ID()
}

//node返回对等方的节点描述符。
func (p *Peer) Node() *enode.Node {
	return p.rw.node
}

//name返回远程节点公布的节点名。
func (p *Peer) Name() string {
	return p.rw.name
}

//caps返回远程对等机的功能（支持的子协议）。
func (p *Peer) Caps() []Cap {
//托多：也许还回副本
	return p.rw.caps
}

//remoteaddr返回网络连接的远程地址。
func (p *Peer) RemoteAddr() net.Addr {
	return p.rw.fd.RemoteAddr()
}

//localaddr返回网络连接的本地地址。
func (p *Peer) LocalAddr() net.Addr {
	return p.rw.fd.LocalAddr()
}

//disconnect以给定的原因终止对等连接。
//它会立即返回，并且不会等待连接关闭。
func (p *Peer) Disconnect(reason DiscReason) {
	select {
	case p.disc <- reason:
	case <-p.closed:
	}
}

//字符串实现fmt.stringer。
func (p *Peer) String() string {
	id := p.ID()
	return fmt.Sprintf("Peer %x %v", id[:8], p.RemoteAddr())
}

//如果对等端是入站连接，则inbound返回true
func (p *Peer) Inbound() bool {
	return p.rw.is(inboundConn)
}

func newPeer(conn *conn, protocols []Protocol) *Peer {
	protomap := matchProtocols(protocols, conn.caps, conn)
	p := &Peer{
		rw:       conn,
		running:  protomap,
		created:  mclock.Now(),
		disc:     make(chan DiscReason),
protoErr: make(chan error, len(protomap)+1), //协议+PingLoop
		closed:   make(chan struct{}),
		log:      log.New("id", conn.node.ID(), "conn", conn.flags),
	}
	return p
}

func (p *Peer) Log() log.Logger {
	return p.log
}

func (p *Peer) run() (remoteRequested bool, err error) {
	var (
		writeStart = make(chan struct{}, 1)
		writeErr   = make(chan error, 1)
		readErr    = make(chan error, 1)
reason     DiscReason //发送给对等方
	)
	p.wg.Add(2)
	go p.readLoop(readErr)
	go p.pingLoop()

//启动所有协议处理程序。
	writeStart <- struct{}{}
	p.startProtocols(writeStart, writeErr)

//等待错误或断开连接。
loop:
	for {
		select {
		case err = <-writeErr:
//已完成写入。允许在以下情况下开始下一次写入
//没有错误。
			if err != nil {
				reason = DiscNetworkError
				break loop
			}
			writeStart <- struct{}{}
		case err = <-readErr:
			if r, ok := err.(DiscReason); ok {
				remoteRequested = true
				reason = r
			} else {
				reason = DiscNetworkError
			}
			break loop
		case err = <-p.protoErr:
			reason = discReasonForError(err)
			break loop
		case err = <-p.disc:
			reason = discReasonForError(err)
			break loop
		}
	}

	close(p.closed)
	p.rw.close(reason)
	p.wg.Wait()
	return remoteRequested, err
}

func (p *Peer) pingLoop() {
	ping := time.NewTimer(pingInterval)
	defer p.wg.Done()
	defer ping.Stop()
	for {
		select {
		case <-ping.C:
			if err := SendItems(p.rw, pingMsg); err != nil {
				p.protoErr <- err
				return
			}
			ping.Reset(pingInterval)
		case <-p.closed:
			return
		}
	}
}

func (p *Peer) readLoop(errc chan<- error) {
	defer p.wg.Done()
	for {
		msg, err := p.rw.ReadMsg()
		if err != nil {
			errc <- err
			return
		}
		msg.ReceivedAt = time.Now()
		if err = p.handle(msg); err != nil {
			errc <- err
			return
		}
	}
}

func (p *Peer) handle(msg Msg) error {
	switch {
	case msg.Code == pingMsg:
		msg.Discard()
		go SendItems(p.rw, pongMsg)
	case msg.Code == discMsg:
		var reason [1]DiscReason
//这是最后一条信息。我们不需要抛弃或
//检查错误，因为连接之后将关闭。
		rlp.Decode(msg.Payload, &reason)
		return reason[0]
	case msg.Code < baseProtocolLength:
//忽略其他基本协议消息
		return msg.Discard()
	default:
//这是一个子协议消息
		proto, err := p.getProto(msg.Code)
		if err != nil {
			return fmt.Errorf("msg code out of range: %v", msg.Code)
		}
		select {
		case proto.in <- msg:
			return nil
		case <-p.closed:
			return io.EOF
		}
	}
	return nil
}

func countMatchingProtocols(protocols []Protocol, caps []Cap) int {
	n := 0
	for _, cap := range caps {
		for _, proto := range protocols {
			if proto.Name == cap.Name && proto.Version == cap.Version {
				n++
			}
		}
	}
	return n
}

//MatchProtocols为匹配命名的子协议创建结构。
func matchProtocols(protocols []Protocol, caps []Cap, rw MsgReadWriter) map[string]*protoRW {
	sort.Sort(capsByNameAndVersion(caps))
	offset := baseProtocolLength
	result := make(map[string]*protoRW)

outer:
	for _, cap := range caps {
		for _, proto := range protocols {
			if proto.Name == cap.Name && proto.Version == cap.Version {
//如果旧协议版本匹配，请将其还原
				if old := result[cap.Name]; old != nil {
					offset -= old.Length
				}
//分配新匹配项
				result[cap.Name] = &protoRW{Protocol: proto, offset: offset, in: make(chan Msg), w: rw}
				offset += proto.Length

				continue outer
			}
		}
	}
	return result
}

func (p *Peer) startProtocols(writeStart <-chan struct{}, writeErr chan<- error) {
	p.wg.Add(len(p.running))
	for _, proto := range p.running {
		proto := proto
		proto.closed = p.closed
		proto.wstart = writeStart
		proto.werr = writeErr
		var rw MsgReadWriter = proto
		if p.events != nil {
			rw = newMsgEventer(rw, p.events, p.ID(), proto.Name)
		}
		p.log.Trace(fmt.Sprintf("Starting protocol %s/%d", proto.Name, proto.Version))
		go func() {
			err := proto.Run(p, rw)
			if err == nil {
				p.log.Trace(fmt.Sprintf("Protocol %s/%d returned", proto.Name, proto.Version))
				err = errProtocolReturned
			} else if err != io.EOF {
				p.log.Trace(fmt.Sprintf("Protocol %s/%d failed", proto.Name, proto.Version), "err", err)
			}
			p.protoErr <- err
			p.wg.Done()
		}()
	}
}

//getproto查找负责处理的协议
//给定的消息代码。
func (p *Peer) getProto(code uint64) (*protoRW, error) {
	for _, proto := range p.running {
		if code >= proto.offset && code < proto.offset+proto.Length {
			return proto, nil
		}
	}
	return nil, newPeerError(errInvalidMsgCode, "%d", code)
}

type protoRW struct {
	Protocol
in     chan Msg        //接收已读消息
closed <-chan struct{} //对等机关闭时接收
wstart <-chan struct{} //当写入可能开始时接收
werr   chan<- error    //用于写入结果
	offset uint64
	w      MsgWriter
}

func (rw *protoRW) WriteMsg(msg Msg) (err error) {
	if msg.Code >= rw.Length {
		return newPeerError(errInvalidMsgCode, "not handled")
	}
	msg.Code += rw.offset
	select {
	case <-rw.wstart:
		err = rw.w.WriteMsg(msg)
//将写状态报告回peer.run。它将启动
//如果错误为非零，则关闭并取消阻止下一次写入
//否则。如果出现错误，调用协议代码应退出。
//但我们不想依赖它。
		rw.werr <- err
	case <-rw.closed:
		err = ErrShuttingDown
	}
	return err
}

func (rw *protoRW) ReadMsg() (Msg, error) {
	select {
	case msg := <-rw.in:
		msg.Code -= rw.offset
		return msg, nil
	case <-rw.closed:
		return Msg{}, io.EOF
	}
}

//PeerInfo表示有关连接的
//同龄人。子协议独立字段包含在此处并在此处初始化，使用
//协议细节委托给所有连接的子协议。
type PeerInfo struct {
Enode   string   `json:"enode"` //节点URL
ID      string   `json:"id"`    //唯一节点标识符
Name    string   `json:"name"`  //节点的名称，包括客户端类型、版本、操作系统、自定义数据
Caps    []string `json:"caps"`  //此对等方公布的协议
	Network struct {
LocalAddress  string `json:"localAddress"`  //TCP数据连接的本地终结点
RemoteAddress string `json:"remoteAddress"` //TCP数据连接的远程终结点
		Inbound       bool   `json:"inbound"`
		Trusted       bool   `json:"trusted"`
		Static        bool   `json:"static"`
	} `json:"network"`
Protocols map[string]interface{} `json:"protocols"` //子协议特定的元数据字段
}

//INFO收集并返回有关对等机的已知元数据集合。
func (p *Peer) Info() *PeerInfo {
//收集协议功能
	var caps []string
	for _, cap := range p.Caps() {
		caps = append(caps, cap.String())
	}
//组装通用对等元数据
	info := &PeerInfo{
		Enode:     p.Node().String(),
		ID:        p.ID().String(),
		Name:      p.Name(),
		Caps:      caps,
		Protocols: make(map[string]interface{}),
	}
	info.Network.LocalAddress = p.LocalAddr().String()
	info.Network.RemoteAddress = p.RemoteAddr().String()
	info.Network.Inbound = p.rw.is(inboundConn)
	info.Network.Trusted = p.rw.is(trustedConn)
	info.Network.Static = p.rw.is(staticDialedConn)

//收集所有正在运行的协议信息
	for _, proto := range p.running {
		protoInfo := interface{}("unknown")
		if query := proto.Protocol.PeerInfo; query != nil {
			if metadata := query(p.ID()); metadata != nil {
				protoInfo = metadata
			} else {
				protoInfo = "handshake"
			}
		}
		info.Protocols[proto.Name] = protoInfo
	}
	return info
}
