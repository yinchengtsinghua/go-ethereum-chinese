
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

package stream

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/swarm/log"
	bv "github.com/ethereum/go-ethereum/swarm/network/bitvector"
	"github.com/ethereum/go-ethereum/swarm/spancontext"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/opentracing/opentracing-go"
)

var syncBatchTimeout = 30 * time.Second

//流定义唯一的流标识符。
type Stream struct {
//名称用于标识客户机和服务器功能。
	Name string
//key是特定流数据的名称。
	Key string
//Live定义流是否只传递新数据
//对于特定流。
	Live bool
}

func NewStream(name string, key string, live bool) Stream {
	return Stream{
		Name: name,
		Key:  key,
		Live: live,
	}
}

//字符串基于所有流字段返回流ID。
func (s Stream) String() string {
	t := "h"
	if s.Live {
		t = "l"
	}
	return fmt.Sprintf("%s|%s|%s", s.Name, s.Key, t)
}

//subcribeMsg是用于请求流的协议消息（节）
type SubscribeMsg struct {
	Stream   Stream
	History  *Range `rlp:"nil"`
Priority uint8  //通过优先渠道交付
}

//request subscription msg是节点请求订阅的协议msg
//特定流
type RequestSubscriptionMsg struct {
	Stream   Stream
	History  *Range `rlp:"nil"`
Priority uint8  //通过优先渠道交付
}

func (p *Peer) handleRequestSubscription(ctx context.Context, req *RequestSubscriptionMsg) (err error) {
	log.Debug(fmt.Sprintf("handleRequestSubscription: streamer %s to subscribe to %s with stream %s", p.streamer.addr, p.ID(), req.Stream))
	if err = p.streamer.Subscribe(p.ID(), req.Stream, req.History, req.Priority); err != nil {
//错误将作为订阅错误消息发送
//不会返回，因为它将阻止任何新消息
//对等点之间的交换超过p2p。相反，将返回错误
//仅当有来自发送订阅错误消息的消息时。
		err = p.Send(ctx, SubscribeErrorMsg{
			Error: err.Error(),
		})
	}
	return err
}

func (p *Peer) handleSubscribeMsg(ctx context.Context, req *SubscribeMsg) (err error) {
	metrics.GetOrRegisterCounter("peer.handlesubscribemsg", nil).Inc(1)

	defer func() {
		if err != nil {
//错误将作为订阅错误消息发送
//不会返回，因为它将阻止任何新消息
//对等点之间的交换超过p2p。相反，将返回错误
//仅当有来自发送订阅错误消息的消息时。
			err = p.Send(context.TODO(), SubscribeErrorMsg{
				Error: err.Error(),
			})
		}
	}()

	log.Debug("received subscription", "from", p.streamer.addr, "peer", p.ID(), "stream", req.Stream, "history", req.History)

	f, err := p.streamer.GetServerFunc(req.Stream.Name)
	if err != nil {
		return err
	}

	s, err := f(p, req.Stream.Key, req.Stream.Live)
	if err != nil {
		return err
	}
	os, err := p.setServer(req.Stream, s, req.Priority)
	if err != nil {
		return err
	}

	var from uint64
	var to uint64
	if !req.Stream.Live && req.History != nil {
		from = req.History.From
		to = req.History.To
	}

	go func() {
		if err := p.SendOfferedHashes(os, from, to); err != nil {
			log.Warn("SendOfferedHashes error", "peer", p.ID().TerminalString(), "err", err)
		}
	}()

	if req.Stream.Live && req.History != nil {
//订阅历史流
		s, err := f(p, req.Stream.Key, false)
		if err != nil {
			return err
		}

		os, err := p.setServer(getHistoryStream(req.Stream), s, getHistoryPriority(req.Priority))
		if err != nil {
			return err
		}
		go func() {
			if err := p.SendOfferedHashes(os, req.History.From, req.History.To); err != nil {
				log.Warn("SendOfferedHashes error", "peer", p.ID().TerminalString(), "err", err)
			}
		}()
	}

	return nil
}

type SubscribeErrorMsg struct {
	Error string
}

func (p *Peer) handleSubscribeErrorMsg(req *SubscribeErrorMsg) (err error) {
//TODO应将错误传递给调用订阅的任何人
	return fmt.Errorf("subscribe to peer %s: %v", p.ID(), req.Error)
}

type UnsubscribeMsg struct {
	Stream Stream
}

func (p *Peer) handleUnsubscribeMsg(req *UnsubscribeMsg) error {
	return p.removeServer(req.Stream)
}

type QuitMsg struct {
	Stream Stream
}

func (p *Peer) handleQuitMsg(req *QuitMsg) error {
	return p.removeClient(req.Stream)
}

//offeredhashemsg是一个协议消息，用于提供
//流段
type OfferedHashesMsg struct {
Stream         Stream //河流名称
From, To       uint64 //对等和数据库特定条目计数
Hashes         []byte //哈希流（128）
*HandoverProof        //防交
}

//提供的字符串漂亮打印
func (m OfferedHashesMsg) String() string {
	return fmt.Sprintf("Stream '%v' [%v-%v] (%v)", m.Stream, m.From, m.To, len(m.Hashes)/HashSize)
}

//handleofferedhashemsg协议消息处理程序调用传入拖缆接口
//滤波法
func (p *Peer) handleOfferedHashesMsg(ctx context.Context, req *OfferedHashesMsg) error {
	metrics.GetOrRegisterCounter("peer.handleofferedhashes", nil).Inc(1)

	var sp opentracing.Span
	ctx, sp = spancontext.StartSpan(
		ctx,
		"handle.offered.hashes")
	defer sp.Finish()

	c, _, err := p.getOrSetClient(req.Stream, req.From, req.To)
	if err != nil {
		return err
	}

	hashes := req.Hashes
	lenHashes := len(hashes)
	if lenHashes%HashSize != 0 {
		return fmt.Errorf("error invalid hashes length (len: %v)", lenHashes)
	}

	want, err := bv.New(lenHashes / HashSize)
	if err != nil {
		return fmt.Errorf("error initiaising bitvector of length %v: %v", lenHashes/HashSize, err)
	}

	ctr := 0
	errC := make(chan error)
	ctx, cancel := context.WithTimeout(ctx, syncBatchTimeout)

	ctx = context.WithValue(ctx, "source", p.ID().String())
	for i := 0; i < lenHashes; i += HashSize {
		hash := hashes[i : i+HashSize]

		if wait := c.NeedData(ctx, hash); wait != nil {
			ctr++
			want.Set(i/HashSize, true)
//创建请求并等待块数据到达并存储
			go func(w func(context.Context) error) {
				select {
				case errC <- w(ctx):
				case <-ctx.Done():
				}
			}(wait)
		}
	}

	go func() {
		defer cancel()
		for i := 0; i < ctr; i++ {
			select {
			case err := <-errC:
				if err != nil {
					log.Debug("client.handleOfferedHashesMsg() error waiting for chunk, dropping peer", "peer", p.ID(), "err", err)
					p.Drop(err)
					return
				}
			case <-ctx.Done():
				log.Debug("client.handleOfferedHashesMsg() context done", "ctx.Err()", ctx.Err())
				return
			case <-c.quit:
				log.Debug("client.handleOfferedHashesMsg() quit")
				return
			}
		}
		select {
		case c.next <- c.batchDone(p, req, hashes):
		case <-c.quit:
			log.Debug("client.handleOfferedHashesMsg() quit")
		case <-ctx.Done():
			log.Debug("client.handleOfferedHashesMsg() context done", "ctx.Err()", ctx.Err())
		}
	}()
//仅当前一批中所有丢失的块都到达时发送wantedkeysmsg
//除了
	if c.stream.Live {
		c.sessionAt = req.From
	}
	from, to := c.nextBatch(req.To + 1)
	log.Trace("set next batch", "peer", p.ID(), "stream", req.Stream, "from", req.From, "to", req.To, "addr", p.streamer.addr)
	if from == to {
		return nil
	}

	msg := &WantedHashesMsg{
		Stream: req.Stream,
		Want:   want.Bytes(),
		From:   from,
		To:     to,
	}
	go func() {
		log.Trace("sending want batch", "peer", p.ID(), "stream", msg.Stream, "from", msg.From, "to", msg.To)
		select {
		case err := <-c.next:
			if err != nil {
				log.Warn("c.next error dropping peer", "err", err)
				p.Drop(err)
				return
			}
		case <-c.quit:
			log.Debug("client.handleOfferedHashesMsg() quit")
			return
		case <-ctx.Done():
			log.Debug("client.handleOfferedHashesMsg() context done", "ctx.Err()", ctx.Err())
			return
		}
		log.Trace("sending want batch", "peer", p.ID(), "stream", msg.Stream, "from", msg.From, "to", msg.To)
		err := p.SendPriority(ctx, msg, c.priority)
		if err != nil {
			log.Warn("SendPriority error", "err", err)
		}
	}()
	return nil
}

//WantedHashesMsg是用于发送哈希的协议消息数据
//在offeredhashemsg中提供的下游对等方实际希望发送
type WantedHashesMsg struct {
	Stream   Stream
Want     []byte //位向量，指示批处理中需要哪些键
From, To uint64 //下一个间隔偏移量-如果不继续，则为空
}

//字符串漂亮打印WantedHashesMsg
func (m WantedHashesMsg) String() string {
	return fmt.Sprintf("Stream '%v', Want: %x, Next: [%v-%v]", m.Stream, m.Want, m.From, m.To)
}

//handlevantedhashesmsg协议消息处理程序
//*发送下一批未同步的密钥
//*根据WantedHashesMsg发送实际数据块
func (p *Peer) handleWantedHashesMsg(ctx context.Context, req *WantedHashesMsg) error {
	metrics.GetOrRegisterCounter("peer.handlewantedhashesmsg", nil).Inc(1)

	log.Trace("received wanted batch", "peer", p.ID(), "stream", req.Stream, "from", req.From, "to", req.To)
	s, err := p.getServer(req.Stream)
	if err != nil {
		return err
	}
	hashes := s.currentBatch
//从getbatch块开始在go例程中启动，直到新哈希到达
	go func() {
		if err := p.SendOfferedHashes(s, req.From, req.To); err != nil {
			log.Warn("SendOfferedHashes error", "peer", p.ID().TerminalString(), "err", err)
		}
	}()
//转到p.sendOfferedHashes（s，req.from，req.to）
	l := len(hashes) / HashSize

	log.Trace("wanted batch length", "peer", p.ID(), "stream", req.Stream, "from", req.From, "to", req.To, "lenhashes", len(hashes), "l", l)
	want, err := bv.NewFromBytes(req.Want, l)
	if err != nil {
		return fmt.Errorf("error initiaising bitvector of length %v: %v", l, err)
	}
	for i := 0; i < l; i++ {
		if want.Get(i) {
			metrics.GetOrRegisterCounter("peer.handlewantedhashesmsg.actualget", nil).Inc(1)

			hash := hashes[i*HashSize : (i+1)*HashSize]
			data, err := s.GetData(ctx, hash)
			if err != nil {
				return fmt.Errorf("handleWantedHashesMsg get data %x: %v", hash, err)
			}
			chunk := storage.NewChunk(hash, data)
			syncing := true
			if err := p.Deliver(ctx, chunk, s.priority, syncing); err != nil {
				return err
			}
		}
	}
	return nil
}

//移交表示上游对等方移交流段的声明。
type Handover struct {
Stream     Stream //河流名称
Start, End uint64 //散列索引
Root       []byte //索引段包含证明的根哈希
}

//handOverfloof表示上游对等端移交流部分的签名语句
type HandoverProof struct {
Sig []byte //签名（哈希（串行化（移交）））
	*Handover
}

//接管表示下游对等方接管的语句（存储所有数据）
//移交
type Takeover Handover

//takeoveroof表示下游对等方接管的签名声明
//河道断面
type TakeoverProof struct {
Sig []byte //符号（哈希（序列化（接管）））
	*Takeover
}

//takeoveroofmsg是下游对等机发送的协议消息
type TakeoverProofMsg TakeoverProof

//字符串漂亮打印takeoveroofmsg
func (m TakeoverProofMsg) String() string {
	return fmt.Sprintf("Stream: '%v' [%v-%v], Root: %x, Sig: %x", m.Stream, m.Start, m.End, m.Root, m.Sig)
}

func (p *Peer) handleTakeoverProofMsg(ctx context.Context, req *TakeoverProofMsg) error {
	_, err := p.getServer(req.Stream)
//在拖缆中存储流的最强takeoveroof
	return err
}
