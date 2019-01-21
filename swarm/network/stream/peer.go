
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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/swarm/log"
	pq "github.com/ethereum/go-ethereum/swarm/network/priorityqueue"
	"github.com/ethereum/go-ethereum/swarm/network/stream/intervals"
	"github.com/ethereum/go-ethereum/swarm/spancontext"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
	opentracing "github.com/opentracing/opentracing-go"
)

type notFoundError struct {
	t string
	s Stream
}

func newNotFoundError(t string, s Stream) *notFoundError {
	return &notFoundError{t: t, s: s}
}

func (e *notFoundError) Error() string {
	return fmt.Sprintf("%s not found for stream %q", e.t, e.s)
}

//如果达到对等服务器限制，将返回errmaxpeerservers。
//它将在subscriberormsg中发送。
var ErrMaxPeerServers = errors.New("max peer servers")

//Peer是流协议的对等扩展
type Peer struct {
	*protocols.Peer
	streamer *Registry
	pq       *pq.PriorityQueue
	serverMu sync.RWMutex
clientMu sync.RWMutex //保护客户端和客户端参数
	servers  map[Stream]*server
	clients  map[Stream]*client
//clientparams映射保留必需的客户端参数
//在注册表上设置的。订阅并使用
//在提供的哈希处理程序中创建新客户端。
	clientParams map[Stream]*clientParams
	quit         chan struct{}
}

type WrappedPriorityMsg struct {
	Context context.Context
	Msg     interface{}
}

//newpeer是peer的构造函数
func NewPeer(peer *protocols.Peer, streamer *Registry) *Peer {
	p := &Peer{
		Peer:         peer,
		pq:           pq.New(int(PriorityQueue), PriorityQueueCap),
		streamer:     streamer,
		servers:      make(map[Stream]*server),
		clients:      make(map[Stream]*client),
		clientParams: make(map[Stream]*clientParams),
		quit:         make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	go p.pq.Run(ctx, func(i interface{}) {
		wmsg := i.(WrappedPriorityMsg)
		err := p.Send(wmsg.Context, wmsg.Msg)
		if err != nil {
			log.Error("Message send error, dropping peer", "peer", p.ID(), "err", err)
			p.Drop(err)
		}
	})

//PQ争用的基本监控
	go func(pq *pq.PriorityQueue) {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				var len_maxi int
				var cap_maxi int
				for k := range pq.Queues {
					if len_maxi < len(pq.Queues[k]) {
						len_maxi = len(pq.Queues[k])
					}

					if cap_maxi < cap(pq.Queues[k]) {
						cap_maxi = cap(pq.Queues[k])
					}
				}

				metrics.GetOrRegisterGauge(fmt.Sprintf("pq_len_%s", p.ID().TerminalString()), nil).Update(int64(len_maxi))
				metrics.GetOrRegisterGauge(fmt.Sprintf("pq_cap_%s", p.ID().TerminalString()), nil).Update(int64(cap_maxi))
			case <-p.quit:
				return
			}
		}
	}(p.pq)

	go func() {
		<-p.quit
		cancel()
	}()
	return p
}

//传递向对等端发送storerequestmsg协议消息
//根据“同步”参数，我们发送不同的消息类型
func (p *Peer) Deliver(ctx context.Context, chunk storage.Chunk, priority uint8, syncing bool) error {
	var sp opentracing.Span
	var msg interface{}

	spanName := "send.chunk.delivery"

//如果传递是为了同步或检索，我们会发送不同类型的消息，
//即使消息的处理和内容相同，
//因为交换记帐根据消息类型决定哪些消息需要记帐
	if syncing {
		msg = &ChunkDeliveryMsgSyncing{
			Addr:  chunk.Address(),
			SData: chunk.Data(),
		}
		spanName += ".syncing"
	} else {
		msg = &ChunkDeliveryMsgRetrieval{
			Addr:  chunk.Address(),
			SData: chunk.Data(),
		}
		spanName += ".retrieval"
	}
	ctx, sp = spancontext.StartSpan(
		ctx,
		spanName)
	defer sp.Finish()

	return p.SendPriority(ctx, msg, priority)
}

//sendpriority使用传出优先级队列向对等端发送消息
func (p *Peer) SendPriority(ctx context.Context, msg interface{}, priority uint8) error {
	defer metrics.GetOrRegisterResettingTimer(fmt.Sprintf("peer.sendpriority_t.%d", priority), nil).UpdateSince(time.Now())
	metrics.GetOrRegisterCounter(fmt.Sprintf("peer.sendpriority.%d", priority), nil).Inc(1)
	wmsg := WrappedPriorityMsg{
		Context: ctx,
		Msg:     msg,
	}
	err := p.pq.Push(wmsg, int(priority))
	if err == pq.ErrContention {
		log.Warn("dropping peer on priority queue contention", "peer", p.ID())
		p.Drop(err)
	}
	return err
}

//sendforferedhashes发送offeredhashemsg协议消息
func (p *Peer) SendOfferedHashes(s *server, f, t uint64) error {
	var sp opentracing.Span
	ctx, sp := spancontext.StartSpan(
		context.TODO(),
		"send.offered.hashes")
	defer sp.Finish()

	hashes, from, to, proof, err := s.setNextBatch(f, t)
	if err != nil {
		return err
	}
//只有在退出时才为真
	if len(hashes) == 0 {
		return nil
	}
	if proof == nil {
		proof = &HandoverProof{
			Handover: &Handover{},
		}
	}
	s.currentBatch = hashes
	msg := &OfferedHashesMsg{
		HandoverProof: proof,
		Hashes:        hashes,
		From:          from,
		To:            to,
		Stream:        s.stream,
	}
	log.Trace("Swarm syncer offer batch", "peer", p.ID(), "stream", s.stream, "len", len(hashes), "from", from, "to", to)
	return p.SendPriority(ctx, msg, s.priority)
}

func (p *Peer) getServer(s Stream) (*server, error) {
	p.serverMu.RLock()
	defer p.serverMu.RUnlock()

	server := p.servers[s]
	if server == nil {
		return nil, newNotFoundError("server", s)
	}
	return server, nil
}

func (p *Peer) setServer(s Stream, o Server, priority uint8) (*server, error) {
	p.serverMu.Lock()
	defer p.serverMu.Unlock()

	if p.servers[s] != nil {
		return nil, fmt.Errorf("server %s already registered", s)
	}

	if p.streamer.maxPeerServers > 0 && len(p.servers) >= p.streamer.maxPeerServers {
		return nil, ErrMaxPeerServers
	}

	sessionIndex, err := o.SessionIndex()
	if err != nil {
		return nil, err
	}
	os := &server{
		Server:       o,
		stream:       s,
		priority:     priority,
		sessionIndex: sessionIndex,
	}
	p.servers[s] = os
	return os, nil
}

func (p *Peer) removeServer(s Stream) error {
	p.serverMu.Lock()
	defer p.serverMu.Unlock()

	server, ok := p.servers[s]
	if !ok {
		return newNotFoundError("server", s)
	}
	server.Close()
	delete(p.servers, s)
	return nil
}

func (p *Peer) getClient(ctx context.Context, s Stream) (c *client, err error) {
	var params *clientParams
	func() {
		p.clientMu.RLock()
		defer p.clientMu.RUnlock()

		c = p.clients[s]
		if c != nil {
			return
		}
		params = p.clientParams[s]
	}()
	if c != nil {
		return c, nil
	}

	if params != nil {
//调试.printstack（）
		if err := params.waitClient(ctx); err != nil {
			return nil, err
		}
	}

	p.clientMu.RLock()
	defer p.clientMu.RUnlock()

	c = p.clients[s]
	if c != nil {
		return c, nil
	}
	return nil, newNotFoundError("client", s)
}

func (p *Peer) getOrSetClient(s Stream, from, to uint64) (c *client, created bool, err error) {
	p.clientMu.Lock()
	defer p.clientMu.Unlock()

	c = p.clients[s]
	if c != nil {
		return c, false, nil
	}

	f, err := p.streamer.GetClientFunc(s.Name)
	if err != nil {
		return nil, false, err
	}

	is, err := f(p, s.Key, s.Live)
	if err != nil {
		return nil, false, err
	}

	cp, err := p.getClientParams(s)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		if err == nil {
			if err := p.removeClientParams(s); err != nil {
				log.Error("stream set client: remove client params", "stream", s, "peer", p, "err", err)
			}
		}
	}()

	intervalsKey := peerStreamIntervalsKey(p, s)
	if s.Live {
//尝试查找以前的历史记录和实时间隔，并将实时记录合并到历史记录中
		historyKey := peerStreamIntervalsKey(p, NewStream(s.Name, s.Key, false))
		historyIntervals := &intervals.Intervals{}
		err := p.streamer.intervalsStore.Get(historyKey, historyIntervals)
		switch err {
		case nil:
			liveIntervals := &intervals.Intervals{}
			err := p.streamer.intervalsStore.Get(intervalsKey, liveIntervals)
			switch err {
			case nil:
				historyIntervals.Merge(liveIntervals)
				if err := p.streamer.intervalsStore.Put(historyKey, historyIntervals); err != nil {
					log.Error("stream set client: put history intervals", "stream", s, "peer", p, "err", err)
				}
			case state.ErrNotFound:
			default:
				log.Error("stream set client: get live intervals", "stream", s, "peer", p, "err", err)
			}
		case state.ErrNotFound:
		default:
			log.Error("stream set client: get history intervals", "stream", s, "peer", p, "err", err)
		}
	}

	if err := p.streamer.intervalsStore.Put(intervalsKey, intervals.NewIntervals(from)); err != nil {
		return nil, false, err
	}

	next := make(chan error, 1)
	c = &client{
		Client:         is,
		stream:         s,
		priority:       cp.priority,
		to:             cp.to,
		next:           next,
		quit:           make(chan struct{}),
		intervalsStore: p.streamer.intervalsStore,
		intervalsKey:   intervalsKey,
	}
	p.clients[s] = c
cp.clientCreated() //取消阻止正在等待的所有可能的getclient调用
next <- nil        //这是为了在第一批到达之前允许wantedkeysmsg
	return c, true, nil
}

func (p *Peer) removeClient(s Stream) error {
	p.clientMu.Lock()
	defer p.clientMu.Unlock()

	client, ok := p.clients[s]
	if !ok {
		return newNotFoundError("client", s)
	}
	client.close()
	delete(p.clients, s)
	return nil
}

func (p *Peer) setClientParams(s Stream, params *clientParams) error {
	p.clientMu.Lock()
	defer p.clientMu.Unlock()

	if p.clients[s] != nil {
		return fmt.Errorf("client %s already exists", s)
	}
	if p.clientParams[s] != nil {
		return fmt.Errorf("client params %s already set", s)
	}
	p.clientParams[s] = params
	return nil
}

func (p *Peer) getClientParams(s Stream) (*clientParams, error) {
	params := p.clientParams[s]
	if params == nil {
		return nil, fmt.Errorf("client params '%v' not provided to peer %v", s, p.ID())
	}
	return params, nil
}

func (p *Peer) removeClientParams(s Stream) error {
	_, ok := p.clientParams[s]
	if !ok {
		return newNotFoundError("client params", s)
	}
	delete(p.clientParams, s)
	return nil
}

func (p *Peer) close() {
	for _, s := range p.servers {
		s.Close()
	}
}
