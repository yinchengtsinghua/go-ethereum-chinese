
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
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rlp"
)

//msg定义p2p消息的结构。
//
//注意，由于有效负载读卡器
//发送时消耗。无法创建消息和
//发送任意次数。如果要重用编码的
//结构，将有效负载编码为字节数组并创建
//用字节分隔消息。读卡器作为每次发送的有效负载。
type Msg struct {
	Code       uint64
Size       uint32 //Paylod的规模
	Payload    io.Reader
	ReceivedAt time.Time
}

//解码将消息的rlp内容解析为
//给定值，必须是指针。
//
//解码规则见RLP包。
func (msg Msg) Decode(val interface{}) error {
	s := rlp.NewStream(msg.Payload, uint64(msg.Size))
	if err := s.Decode(val); err != nil {
		return newPeerError(errInvalidMsg, "(code %x) (size %d) %v", msg.Code, msg.Size, err)
	}
	return nil
}

func (msg Msg) String() string {
	return fmt.Sprintf("msg #%v (%v bytes)", msg.Code, msg.Size)
}

//丢弃将剩余的有效载荷数据读取到黑洞中。
func (msg Msg) Discard() error {
	_, err := io.Copy(ioutil.Discard, msg.Payload)
	return err
}

type MsgReader interface {
	ReadMsg() (Msg, error)
}

type MsgWriter interface {
//writemsg发送消息。它将一直阻塞到消息
//另一端消耗了有效载荷。
//
//请注意，消息只能发送一次，因为它们
//有效负载读卡器已排空。
	WriteMsg(Msg) error
}

//msgreadwriter提供对编码消息的读写。
//实现应该确保readmsg和writemsg可以
//从多个goroutine同时调用。
type MsgReadWriter interface {
	MsgReader
	MsgWriter
}

//send用给定的代码编写一个rlp编码的消息。
//数据应编码为RLP列表。
func Send(w MsgWriter, msgcode uint64, data interface{}) error {
	size, r, err := rlp.EncodeToReader(data)
	if err != nil {
		return err
	}
	return w.WriteMsg(Msg{Code: msgcode, Size: uint32(size), Payload: r})
}

//senditems用给定的代码和数据元素编写一个rlp。
//对于以下电话：
//
//发送项（W、代码、E1、E2、E3）
//
//消息有效负载将是包含以下项目的RLP列表：
//
//[E1，E2，E3]
//
func SendItems(w MsgWriter, msgcode uint64, elems ...interface{}) error {
	return Send(w, msgcode, elems)
}

//eofsignal用eof信号包裹读卡器。EOF通道是
//当包装的读卡器返回错误或计数字节时关闭
//已经看过了。
type eofSignal struct {
	wrapped io.Reader
count   uint32 //剩余字节数
	eof     chan<- struct{}
}

//注意：使用eofsignal检测消息有效负载时
//已被读取，对于零大小的邮件可能不调用读取。
func (r *eofSignal) Read(buf []byte) (int, error) {
	if r.count == 0 {
		if r.eof != nil {
			r.eof <- struct{}{}
			r.eof = nil
		}
		return 0, io.EOF
	}

	max := len(buf)
	if int(r.count) < len(buf) {
		max = int(r.count)
	}
	n, err := r.wrapped.Read(buf[:max])
	r.count -= uint32(n)
	if (err != nil || r.count == 0) && r.eof != nil {
r.eof <- struct{}{} //告诉对等机消息已被消耗
		r.eof = nil
	}
	return n, err
}

//msgpipe创建消息管道。一端读取匹配
//写在另一个上面。管道为全双工，两端
//实现msgreadwriter。
func MsgPipe() (*MsgPipeRW, *MsgPipeRW) {
	var (
		c1, c2  = make(chan Msg), make(chan Msg)
		closing = make(chan struct{})
		closed  = new(int32)
		rw1     = &MsgPipeRW{c1, c2, closing, closed}
		rw2     = &MsgPipeRW{c2, c1, closing, closed}
	)
	return rw1, rw2
}

//errPipeClosed在
//管道已关闭。
var ErrPipeClosed = errors.New("p2p: read or write on closed message pipe")

//msgpiperw是msgreadwriter管道的端点。
type MsgPipeRW struct {
	w       chan<- Msg
	r       <-chan Msg
	closing chan struct{}
	closed  *int32
}

//writemsg在管道上发送消息。
//它会一直阻塞，直到接收器消耗了消息有效负载。
func (p *MsgPipeRW) WriteMsg(msg Msg) error {
	if atomic.LoadInt32(p.closed) == 0 {
		consumed := make(chan struct{}, 1)
		msg.Payload = &eofSignal{msg.Payload, msg.Size, consumed}
		select {
		case p.w <- msg:
			if msg.Size > 0 {
//等待有效负载读取或丢弃
				select {
				case <-consumed:
				case <-p.closing:
				}
			}
			return nil
		case <-p.closing:
		}
	}
	return ErrPipeClosed
}

//readmsg返回在管道另一端发送的消息。
func (p *MsgPipeRW) ReadMsg() (Msg, error) {
	if atomic.LoadInt32(p.closed) == 0 {
		select {
		case msg := <-p.r:
			return msg, nil
		case <-p.closing:
		}
	}
	return Msg{}, ErrPipeClosed
}

//关闭取消阻止两端所有挂起的readmsg和writemsg调用
//管子的它们将返回errpipeClosed。关闭也
//中断对消息有效负载的任何读取。
func (p *MsgPipeRW) Close() error {
	if atomic.AddInt32(p.closed, 1) != 1 {
//其他人已经关门了
atomic.StoreInt32(p.closed, 1) //避免溢出
		return nil
	}
	close(p.closing)
	return nil
}

//expectmsg从r读取消息并验证其
//代码和编码的rlp内容与提供的值匹配。
//如果内容为零，则丢弃有效负载，不进行验证。
func ExpectMsg(r MsgReader, code uint64, content interface{}) error {
	msg, err := r.ReadMsg()
	if err != nil {
		return err
	}
	if msg.Code != code {
		return fmt.Errorf("message code mismatch: got %d, expected %d", msg.Code, code)
	}
	if content == nil {
		return msg.Discard()
	}
	contentEnc, err := rlp.EncodeToBytes(content)
	if err != nil {
		panic("content encode error: " + err.Error())
	}
	if int(msg.Size) != len(contentEnc) {
		return fmt.Errorf("message size mismatch: got %d, want %d", msg.Size, len(contentEnc))
	}
	actualContent, err := ioutil.ReadAll(msg.Payload)
	if err != nil {
		return err
	}
	if !bytes.Equal(actualContent, contentEnc) {
		return fmt.Errorf("message payload mismatch:\ngot:  %x\nwant: %x", actualContent, contentEnc)
	}
	return nil
}

//msgeventer包装msgreadwriter并在每次发送消息时发送事件
//或收到
type msgEventer struct {
	MsgReadWriter

	feed     *event.Feed
	peerID   enode.ID
	Protocol string
}

//newmsgeventer返回一个msgeventer，它将消息事件发送到给定的
//喂养
func newMsgEventer(rw MsgReadWriter, feed *event.Feed, peerID enode.ID, proto string) *msgEventer {
	return &msgEventer{
		MsgReadWriter: rw,
		feed:          feed,
		peerID:        peerID,
		Protocol:      proto,
	}
}

//readmsg从基础msgreadwriter读取消息并发出
//“收到消息”事件
func (ev *msgEventer) ReadMsg() (Msg, error) {
	msg, err := ev.MsgReadWriter.ReadMsg()
	if err != nil {
		return msg, err
	}
	ev.feed.Send(&PeerEvent{
		Type:     PeerEventTypeMsgRecv,
		Peer:     ev.peerID,
		Protocol: ev.Protocol,
		MsgCode:  &msg.Code,
		MsgSize:  &msg.Size,
	})
	return msg, nil
}

//writemsg将消息写入基础msgreadwriter并发出
//“消息已发送”事件
func (ev *msgEventer) WriteMsg(msg Msg) error {
	err := ev.MsgReadWriter.WriteMsg(msg)
	if err != nil {
		return err
	}
	ev.feed.Send(&PeerEvent{
		Type:     PeerEventTypeMsgSend,
		Peer:     ev.peerID,
		Protocol: ev.Protocol,
		MsgCode:  &msg.Code,
		MsgSize:  &msg.Size,
	})
	return nil
}

//close如果实现IO.closer，则关闭基础msgreadwriter。
//界面
func (ev *msgEventer) Close() error {
	if v, ok := ev.MsgReadWriter.(io.Closer); ok {
		return v.Close()
	}
	return nil
}
