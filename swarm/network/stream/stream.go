
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
	"math"
	"reflect"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/stream/intervals"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

const (
	Low uint8 = iota
	Mid
	High
	Top
PriorityQueue    = 4    //优先级队列数-低、中、高、顶
PriorityQueueCap = 4096 //队列容量
	HashSize         = 32
)

//枚举用于同步和检索的选项
type SyncingOption int
type RetrievalOption int

//同步选项
const (
//同步已禁用
	SyncingDisabled SyncingOption = iota
//注册客户端和服务器，但不订阅
	SyncingRegisterOnly
//客户端和服务器功能都已注册，订阅将自动发送
	SyncingAutoSubscribe
)

const (
//检索已禁用。主要用于隔离同步功能的测试（即仅同步）
	RetrievalDisabled RetrievalOption = iota
//仅注册检索请求的客户端。
//（轻节点不提供检索请求）
//一旦注册了客户端，总是发送用于检索请求流的订阅
	RetrievalClientOnly
//客户端和服务器功能都已注册，订阅将自动发送
	RetrievalEnabled
)

//subscriptionUnc用于确定执行订阅时要执行的操作
//通常我们会开始真正订阅节点，但是对于测试，可能需要其他功能
//（请参阅streamer_test.go中的testrequestpeersubscriptions）
var subscriptionFunc func(r *Registry, p *network.Peer, bin uint8, subs map[enode.ID]map[Stream]struct{}) bool = doRequestSubscription

//传出和传入拖缆构造函数的注册表
type Registry struct {
	addr           enode.ID
	api            *API
	skipCheck      bool
	clientMu       sync.RWMutex
	serverMu       sync.RWMutex
	peersMu        sync.RWMutex
	serverFuncs    map[string]func(*Peer, string, bool) (Server, error)
	clientFuncs    map[string]func(*Peer, string, bool) (Client, error)
	peers          map[enode.ID]*Peer
	delivery       *Delivery
	intervalsStore state.Store
autoRetrieval  bool //自动订阅以检索请求流
	maxPeerServers int
spec           *protocols.Spec   //本协议规范
balance        protocols.Balance //实现协议。平衡，用于记帐
prices         protocols.Prices  //执行协议。价格，为会计提供价格
}

//RegistryOptions保留NewRegistry构造函数的可选值。
type RegistryOptions struct {
	SkipCheck       bool
Syncing         SyncingOption   //定义同步行为
Retrieval       RetrievalOption //定义检索行为
	SyncUpdateDelay time.Duration
MaxPeerServers  int //注册表中每个对等服务器的限制
}

//NewRegistry是拖缆构造函数
func NewRegistry(localID enode.ID, delivery *Delivery, syncChunkStore storage.SyncChunkStore, intervalsStore state.Store, options *RegistryOptions, balance protocols.Balance) *Registry {
	if options == nil {
		options = &RegistryOptions{}
	}
	if options.SyncUpdateDelay <= 0 {
		options.SyncUpdateDelay = 15 * time.Second
	}
//检查是否已禁用检索
	retrieval := options.Retrieval != RetrievalDisabled

	streamer := &Registry{
		addr:           localID,
		skipCheck:      options.SkipCheck,
		serverFuncs:    make(map[string]func(*Peer, string, bool) (Server, error)),
		clientFuncs:    make(map[string]func(*Peer, string, bool) (Client, error)),
		peers:          make(map[enode.ID]*Peer),
		delivery:       delivery,
		intervalsStore: intervalsStore,
		autoRetrieval:  retrieval,
		maxPeerServers: options.MaxPeerServers,
		balance:        balance,
	}

	streamer.setupSpec()

	streamer.api = NewAPI(streamer)
	delivery.getPeer = streamer.getPeer

//如果启用了检索，请注册服务器func，以便为检索请求提供服务（仅限非轻型节点）
	if options.Retrieval == RetrievalEnabled {
		streamer.RegisterServerFunc(swarmChunkServerStreamName, func(_ *Peer, _ string, live bool) (Server, error) {
			if !live {
				return nil, errors.New("only live retrieval requests supported")
			}
			return NewSwarmChunkServer(delivery.chunkStore), nil
		})
	}

//如果未禁用检索，则注册客户机func（轻节点和普通节点都可以发出检索请求）
	if options.Retrieval != RetrievalDisabled {
		streamer.RegisterClientFunc(swarmChunkServerStreamName, func(p *Peer, t string, live bool) (Client, error) {
			return NewSwarmSyncerClient(p, syncChunkStore, NewStream(swarmChunkServerStreamName, t, live))
		})
	}

//如果未禁用同步，则会注册同步功能（客户端和服务器）
	if options.Syncing != SyncingDisabled {
		RegisterSwarmSyncerServer(streamer, syncChunkStore)
		RegisterSwarmSyncerClient(streamer, syncChunkStore)
	}

//如果同步设置为自动订阅同步流，则启动订阅过程。
	if options.Syncing == SyncingAutoSubscribe {
//latestintc函数确保
//-通过for循环内部的处理不会阻止来自in chan的接收
//-处理完成后，将最新的int值传递给循环
//在邻近地区：
//在循环内完成同步更新之后，我们不需要在中间更新
//深度变化，仅限于最新的
		latestIntC := func(in <-chan int) <-chan int {
			out := make(chan int, 1)

			go func() {
				defer close(out)

				for i := range in {
					select {
					case <-out:
					default:
					}
					out <- i
				}
			}()

			return out
		}

		go func() {
//等待卡德米利亚桌健康
			time.Sleep(options.SyncUpdateDelay)

			kad := streamer.delivery.kad
			depthC := latestIntC(kad.NeighbourhoodDepthC())
			addressBookSizeC := latestIntC(kad.AddrCountC())

//同步对对等方订阅的初始请求
			streamer.updateSyncing()

			for depth := range depthC {
				log.Debug("Kademlia neighbourhood depth change", "depth", depth)

//通过等待直到没有
//新的对等连接。同步流更新将在否之后完成
//对等机连接的时间至少为SyncUpdateDelay。
				timer := time.NewTimer(options.SyncUpdateDelay)
//硬限制同步更新延迟，防止长延迟
//在一个非常动态的网络上
				maxTimer := time.NewTimer(3 * time.Minute)
			loop:
				for {
					select {
					case <-maxTimer.C:
//达到硬超时时强制同步更新
						log.Trace("Sync subscriptions update on hard timeout")
//请求将订阅同步到新对等方
						streamer.updateSyncing()
						break loop
					case <-timer.C:
//开始同步，因为没有新的同级添加到Kademlia
//一段时间
						log.Trace("Sync subscriptions update")
//请求将订阅同步到新对等方
						streamer.updateSyncing()
						break loop
					case size := <-addressBookSizeC:
						log.Trace("Kademlia address book size changed on depth change", "size", size)
//Kademlia增加了新的同行，
//重置计时器以阻止早期同步订阅
						if !timer.Stop() {
							<-timer.C
						}
						timer.Reset(options.SyncUpdateDelay)
					}
				}
				timer.Stop()
				maxTimer.Stop()
			}
		}()
	}

	return streamer
}

//这是一个记帐协议，因此我们需要为规范提供定价挂钩
//为了使模拟能够运行多个节点，而不覆盖钩子的平衡，
//我们需要为每个节点实例构造一个规范实例
func (r *Registry) setupSpec() {
//首先创建“裸”规范
	r.createSpec()
//现在创建定价对象
	r.createPriceOracle()
//如果余额为零，则此节点已在不支持交换的情况下启动（swapEnabled标志为假）
	if r.balance != nil && !reflect.ValueOf(r.balance).IsNil() {
//交换已启用，因此设置挂钩
		r.spec.Hook = protocols.NewAccounting(r.balance, r.prices)
	}
}

//RegisterClient注册一个传入的拖缆构造函数
func (r *Registry) RegisterClientFunc(stream string, f func(*Peer, string, bool) (Client, error)) {
	r.clientMu.Lock()
	defer r.clientMu.Unlock()

	r.clientFuncs[stream] = f
}

//RegisterServer注册传出拖缆构造函数
func (r *Registry) RegisterServerFunc(stream string, f func(*Peer, string, bool) (Server, error)) {
	r.serverMu.Lock()
	defer r.serverMu.Unlock()

	r.serverFuncs[stream] = f
}

//用于传入拖缆构造函数的getclient访问器
func (r *Registry) GetClientFunc(stream string) (func(*Peer, string, bool) (Client, error), error) {
	r.clientMu.RLock()
	defer r.clientMu.RUnlock()

	f := r.clientFuncs[stream]
	if f == nil {
		return nil, fmt.Errorf("stream %v not registered", stream)
	}
	return f, nil
}

//用于传入拖缆构造函数的getserver访问器
func (r *Registry) GetServerFunc(stream string) (func(*Peer, string, bool) (Server, error), error) {
	r.serverMu.RLock()
	defer r.serverMu.RUnlock()

	f := r.serverFuncs[stream]
	if f == nil {
		return nil, fmt.Errorf("stream %v not registered", stream)
	}
	return f, nil
}

func (r *Registry) RequestSubscription(peerId enode.ID, s Stream, h *Range, prio uint8) error {
//检查流是否已注册
	if _, err := r.GetServerFunc(s.Name); err != nil {
		return err
	}

	peer := r.getPeer(peerId)
	if peer == nil {
		return fmt.Errorf("peer not found %v", peerId)
	}

	if _, err := peer.getServer(s); err != nil {
		if e, ok := err.(*notFoundError); ok && e.t == "server" {
//仅当未创建此流的服务器时才请求订阅
			log.Debug("RequestSubscription ", "peer", peerId, "stream", s, "history", h)
			return peer.Send(context.TODO(), &RequestSubscriptionMsg{
				Stream:   s,
				History:  h,
				Priority: prio,
			})
		}
		return err
	}
	log.Trace("RequestSubscription: already subscribed", "peer", peerId, "stream", s, "history", h)
	return nil
}

//订阅启动拖缆
func (r *Registry) Subscribe(peerId enode.ID, s Stream, h *Range, priority uint8) error {
//检查流是否已注册
	if _, err := r.GetClientFunc(s.Name); err != nil {
		return err
	}

	peer := r.getPeer(peerId)
	if peer == nil {
		return fmt.Errorf("peer not found %v", peerId)
	}

	var to uint64
	if !s.Live && h != nil {
		to = h.To
	}

	err := peer.setClientParams(s, newClientParams(priority, to))
	if err != nil {
		return err
	}
	if s.Live && h != nil {
		if err := peer.setClientParams(
			getHistoryStream(s),
			newClientParams(getHistoryPriority(priority), h.To),
		); err != nil {
			return err
		}
	}

	msg := &SubscribeMsg{
		Stream:   s,
		History:  h,
		Priority: priority,
	}
	log.Debug("Subscribe ", "peer", peerId, "stream", s, "history", h)

	return peer.SendPriority(context.TODO(), msg, priority)
}

func (r *Registry) Unsubscribe(peerId enode.ID, s Stream) error {
	peer := r.getPeer(peerId)
	if peer == nil {
		return fmt.Errorf("peer not found %v", peerId)
	}

	msg := &UnsubscribeMsg{
		Stream: s,
	}
	log.Debug("Unsubscribe ", "peer", peerId, "stream", s)

	if err := peer.Send(context.TODO(), msg); err != nil {
		return err
	}
	return peer.removeClient(s)
}

//quit将quitmsg发送到对等端以删除
//流对等客户端并终止流。
func (r *Registry) Quit(peerId enode.ID, s Stream) error {
	peer := r.getPeer(peerId)
	if peer == nil {
		log.Debug("stream quit: peer not found", "peer", peerId, "stream", s)
//如果找不到对等点，则中止请求
		return nil
	}

	msg := &QuitMsg{
		Stream: s,
	}
	log.Debug("Quit ", "peer", peerId, "stream", s)

	return peer.Send(context.TODO(), msg)
}

func (r *Registry) Close() error {
	return r.intervalsStore.Close()
}

func (r *Registry) getPeer(peerId enode.ID) *Peer {
	r.peersMu.RLock()
	defer r.peersMu.RUnlock()

	return r.peers[peerId]
}

func (r *Registry) setPeer(peer *Peer) {
	r.peersMu.Lock()
	r.peers[peer.ID()] = peer
	metrics.GetOrRegisterGauge("registry.peers", nil).Update(int64(len(r.peers)))
	r.peersMu.Unlock()
}

func (r *Registry) deletePeer(peer *Peer) {
	r.peersMu.Lock()
	delete(r.peers, peer.ID())
	metrics.GetOrRegisterGauge("registry.peers", nil).Update(int64(len(r.peers)))
	r.peersMu.Unlock()
}

func (r *Registry) peersCount() (c int) {
	r.peersMu.Lock()
	c = len(r.peers)
	r.peersMu.Unlock()
	return
}

//运行协议运行函数
func (r *Registry) Run(p *network.BzzPeer) error {
	sp := NewPeer(p.Peer, r)
	r.setPeer(sp)
	defer r.deletePeer(sp)
	defer close(sp.quit)
	defer sp.close()

	if r.autoRetrieval && !p.LightNode {
		err := r.Subscribe(p.ID(), NewStream(swarmChunkServerStreamName, "", true), nil, Top)
		if err != nil {
			return err
		}
	}

	return sp.Run(sp.HandleMsg)
}

//通过迭代更新同步订阅以同步流
//卡德米利亚连接和箱子。如果存在同步流
//并且在迭代之后不再需要它们，请求退出
//它们将被发送到适当的对等方。
func (r *Registry) updateSyncing() {
	kad := r.delivery.kad
//所有对等端的所有同步流的映射
//在和中用于删除服务器
//不再需要了
	subs := make(map[enode.ID]map[Stream]struct{})
	r.peersMu.RLock()
	for id, peer := range r.peers {
		peer.serverMu.RLock()
		for stream := range peer.servers {
			if stream.Name == "SYNC" {
				if _, ok := subs[id]; !ok {
					subs[id] = make(map[Stream]struct{})
				}
				subs[id][stream] = struct{}{}
			}
		}
		peer.serverMu.RUnlock()
	}
	r.peersMu.RUnlock()

//开始从对等方请求订阅
	r.requestPeerSubscriptions(kad, subs)

//删除不需要订阅的同步服务器
	for id, streams := range subs {
		if len(streams) == 0 {
			continue
		}
		peer := r.getPeer(id)
		if peer == nil {
			continue
		}
		for stream := range streams {
			log.Debug("Remove sync server", "peer", id, "stream", stream)
			err := r.Quit(peer.ID(), stream)
			if err != nil && err != p2p.ErrShuttingDown {
				log.Error("quit", "err", err, "peer", peer.ID(), "stream", stream)
			}
		}
	}
}

//请求对等订阅对kademlia表中的每个活动对等调用
//并根据对等端的bin向其发送“requestsubscription”
//以及他们与卡德米利亚的关系。
//还要检查“testrequestpeersubscriptions”以了解
//预期行为。
//函数期望：
//*卡德米利亚
//*订阅地图
//*实际订阅功能
//（在测试的情况下，它不做真正的订阅）
func (r *Registry) requestPeerSubscriptions(kad *network.Kademlia, subs map[enode.ID]map[Stream]struct{}) {

	var startPo int
	var endPo int
	var ok bool

//卡德米利亚深度
	kadDepth := kad.NeighbourhoodDepth()
//请求订阅所有节点和容器
//nil作为base取节点的base；我们需要传递255作为'eachconn'运行
//从最深的箱子向后
	kad.EachConn(nil, 255, func(p *network.Peer, po int) bool {
//如果同伴的箱子比卡德米利亚的深度浅，
//只应订阅对等机的bin
		if po < kadDepth {
			startPo = po
			endPo = po
		} else {
//如果同伴的垃圾桶等于或深于卡德米利亚的深度，
//从深度到k.maxproxplay的每个bin都应该订阅
			startPo = kadDepth
			endPo = kad.MaxProxDisplay
		}

		for bin := startPo; bin <= endPo; bin++ {
//做实际订阅
			ok = subscriptionFunc(r, p, uint8(bin), subs)
		}
		return ok
	})
}

//DoRequestSubscription将实际的RequestSubscription发送到对等端
func doRequestSubscription(r *Registry, p *network.Peer, bin uint8, subs map[enode.ID]map[Stream]struct{}) bool {
	log.Debug("Requesting subscription by registry:", "registry", r.addr, "peer", p.ID(), "bin", bin)
//bin总是小于256，可以安全地将其转换为uint8类型
	stream := NewStream("SYNC", FormatSyncBinKey(bin), true)
	if streams, ok := subs[p.ID()]; ok {
//从映射中删除实时流和历史流，以便在发出退出请求时不会将其删除。
		delete(streams, stream)
		delete(streams, getHistoryStream(stream))
	}
	err := r.RequestSubscription(p.ID(), stream, NewRange(0, 0), High)
	if err != nil {
		log.Debug("Request subscription", "err", err, "peer", p.ID(), "stream", stream)
		return false
	}
	return true
}

func (r *Registry) runProtocol(p *p2p.Peer, rw p2p.MsgReadWriter) error {
	peer := protocols.NewPeer(p, rw, r.spec)
	bp := network.NewBzzPeer(peer)
	np := network.NewPeer(bp, r.delivery.kad)
	r.delivery.kad.On(np)
	defer r.delivery.kad.Off(np)
	return r.Run(bp)
}

//handlemsg是委托传入消息的消息处理程序
func (p *Peer) HandleMsg(ctx context.Context, msg interface{}) error {
	switch msg := msg.(type) {

	case *SubscribeMsg:
		return p.handleSubscribeMsg(ctx, msg)

	case *SubscribeErrorMsg:
		return p.handleSubscribeErrorMsg(msg)

	case *UnsubscribeMsg:
		return p.handleUnsubscribeMsg(msg)

	case *OfferedHashesMsg:
		return p.handleOfferedHashesMsg(ctx, msg)

	case *TakeoverProofMsg:
		return p.handleTakeoverProofMsg(ctx, msg)

	case *WantedHashesMsg:
		return p.handleWantedHashesMsg(ctx, msg)

	case *ChunkDeliveryMsgRetrieval:
//对于检索和同步，处理块传递是相同的，所以让我们将msg
		return p.streamer.delivery.handleChunkDeliveryMsg(ctx, p, ((*ChunkDeliveryMsg)(msg)))

	case *ChunkDeliveryMsgSyncing:
//对于检索和同步，处理块传递是相同的，所以让我们将msg
		return p.streamer.delivery.handleChunkDeliveryMsg(ctx, p, ((*ChunkDeliveryMsg)(msg)))

	case *RetrieveRequestMsg:
		return p.streamer.delivery.handleRetrieveRequestMsg(ctx, p, msg)

	case *RequestSubscriptionMsg:
		return p.handleRequestSubscription(ctx, msg)

	case *QuitMsg:
		return p.handleQuitMsg(msg)

	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}
}

type server struct {
	Server
	stream       Stream
	priority     uint8
	currentBatch []byte
	sessionIndex uint64
}

//setNextbatch根据会话索引和是否
//溪流是生命或历史。它调用服务器setnextbatch
//并返回批处理散列及其间隔。
func (s *server) setNextBatch(from, to uint64) ([]byte, uint64, uint64, *HandoverProof, error) {
	if s.stream.Live {
		if from == 0 {
			from = s.sessionIndex
		}
		if to <= from || from >= s.sessionIndex {
			to = math.MaxUint64
		}
	} else {
		if (to < from && to != 0) || from > s.sessionIndex {
			return nil, 0, 0, nil, nil
		}
		if to == 0 || to > s.sessionIndex {
			to = s.sessionIndex
		}
	}
	return s.SetNextBatch(from, to)
}

//传出对等拖缆的服务器接口
type Server interface {
//初始化服务器时调用sessionindex
//获取流数据的当前光标状态。
//基于此索引，实时和历史流间隔
//将在调用setnextbatch之前进行调整。
	SessionIndex() (uint64, error)
	SetNextBatch(uint64, uint64) (hashes []byte, from uint64, to uint64, proof *HandoverProof, err error)
	GetData(context.Context, []byte) ([]byte, error)
	Close()
}

type client struct {
	Client
	stream    Stream
	priority  uint8
	sessionAt uint64
	to        uint64
	next      chan error
	quit      chan struct{}

	intervalsKey   string
	intervalsStore state.Store
}

func peerStreamIntervalsKey(p *Peer, s Stream) string {
	return p.ID().String() + s.String()
}

func (c *client) AddInterval(start, end uint64) (err error) {
	i := &intervals.Intervals{}
	if err = c.intervalsStore.Get(c.intervalsKey, i); err != nil {
		return err
	}
	i.Add(start, end)
	return c.intervalsStore.Put(c.intervalsKey, i)
}

func (c *client) NextInterval() (start, end uint64, err error) {
	i := &intervals.Intervals{}
	err = c.intervalsStore.Get(c.intervalsKey, i)
	if err != nil {
		return 0, 0, err
	}
	start, end = i.Next()
	return start, end, nil
}

//传入对等拖缆的客户端接口
type Client interface {
	NeedData(context.Context, []byte) func(context.Context) error
	BatchDone(Stream, uint64, []byte, []byte) func() (*TakeoverProof, error)
	Close()
}

func (c *client) nextBatch(from uint64) (nextFrom uint64, nextTo uint64) {
	if c.to > 0 && from >= c.to {
		return 0, 0
	}
	if c.stream.Live {
		return from, 0
	} else if from >= c.sessionAt {
		if c.to > 0 {
			return from, c.to
		}
		return from, math.MaxUint64
	}
	nextFrom, nextTo, err := c.NextInterval()
	if err != nil {
		log.Error("next intervals", "stream", c.stream)
		return
	}
	if nextTo > c.to {
		nextTo = c.to
	}
	if nextTo == 0 {
		nextTo = c.sessionAt
	}
	return
}

func (c *client) batchDone(p *Peer, req *OfferedHashesMsg, hashes []byte) error {
	if tf := c.BatchDone(req.Stream, req.From, hashes, req.Root); tf != nil {
		tp, err := tf()
		if err != nil {
			return err
		}
		if err := p.SendPriority(context.TODO(), tp, c.priority); err != nil {
			return err
		}
		if c.to > 0 && tp.Takeover.End >= c.to {
			return p.streamer.Unsubscribe(p.Peer.ID(), req.Stream)
		}
		return nil
	}
	return c.AddInterval(req.From, req.To)
}

func (c *client) close() {
	select {
	case <-c.quit:
	default:
		close(c.quit)
	}
	c.Close()
}

//clientparams存储新客户端的参数
//在订阅和初始提供的哈希请求处理之间。
type clientParams struct {
	priority uint8
	to       uint64
//创建客户端时发出信号
	clientCreatedC chan struct{}
}

func newClientParams(priority uint8, to uint64) *clientParams {
	return &clientParams{
		priority:       priority,
		to:             to,
		clientCreatedC: make(chan struct{}),
	}
}

func (c *clientParams) waitClient(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.clientCreatedC:
		return nil
	}
}

func (c *clientParams) clientCreated() {
	close(c.clientCreatedC)
}

//getspec将拖缆规格返回给调用者
//这曾经是一个全局变量，但用于模拟
//多个节点其字段（尤其是钩子）将被覆盖
func (r *Registry) GetSpec() *protocols.Spec {
	return r.spec
}

func (r *Registry) createSpec() {
//规范是拖缆协议的规范
	var spec = &protocols.Spec{
		Name:       "stream",
		Version:    8,
		MaxMsgSize: 10 * 1024 * 1024,
		Messages: []interface{}{
			UnsubscribeMsg{},
			OfferedHashesMsg{},
			WantedHashesMsg{},
			TakeoverProofMsg{},
			SubscribeMsg{},
			RetrieveRequestMsg{},
			ChunkDeliveryMsgRetrieval{},
			SubscribeErrorMsg{},
			RequestSubscriptionMsg{},
			QuitMsg{},
			ChunkDeliveryMsgSyncing{},
		},
	}
	r.spec = spec
}

//有责任感的信息需要附加一些元信息
//为了评估正确的价格
type StreamerPrices struct {
	priceMatrix map[reflect.Type]*protocols.Price
	registry    *Registry
}

//price实现会计接口并返回特定消息的价格
func (sp *StreamerPrices) Price(msg interface{}) *protocols.Price {
	t := reflect.TypeOf(msg).Elem()
	return sp.priceMatrix[t]
}

//不是硬编码价格，而是得到它
//通过一个函数-它在未来可能非常复杂
func (sp *StreamerPrices) getRetrieveRequestMsgPrice() uint64 {
	return uint64(1)
}

//不是硬编码价格，而是得到它
//通过一个函数-它在未来可能非常复杂
func (sp *StreamerPrices) getChunkDeliveryMsgRetrievalPrice() uint64 {
	return uint64(1)
}

//CreatePriceOracle设置一个矩阵，可以查询该矩阵以获取
//通过Price方法发送消息的价格
func (r *Registry) createPriceOracle() {
	sp := &StreamerPrices{
		registry: r,
	}
	sp.priceMatrix = map[reflect.Type]*protocols.Price{
		reflect.TypeOf(ChunkDeliveryMsgRetrieval{}): {
Value:   sp.getChunkDeliveryMsgRetrievalPrice(), //目前任意价格
			PerByte: true,
			Payer:   protocols.Receiver,
		},
		reflect.TypeOf(RetrieveRequestMsg{}): {
Value:   sp.getRetrieveRequestMsgPrice(), //目前任意价格
			PerByte: false,
			Payer:   protocols.Sender,
		},
	}
	r.prices = sp
}

func (r *Registry) Protocols() []p2p.Protocol {
	return []p2p.Protocol{
		{
			Name:    r.spec.Name,
			Version: r.spec.Version,
			Length:  r.spec.Length(),
			Run:     r.runProtocol,
		},
	}
}

func (r *Registry) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: "stream",
			Version:   "3.0",
			Service:   r.api,
			Public:    true,
		},
	}
}

func (r *Registry) Start(server *p2p.Server) error {
	log.Info("Streamer started")
	return nil
}

func (r *Registry) Stop() error {
	return nil
}

type Range struct {
	From, To uint64
}

func NewRange(from, to uint64) *Range {
	return &Range{
		From: from,
		To:   to,
	}
}

func (r *Range) String() string {
	return fmt.Sprintf("%v-%v", r.From, r.To)
}

func getHistoryPriority(priority uint8) uint8 {
	if priority == 0 {
		return 0
	}
	return priority - 1
}

func getHistoryStream(s Stream) Stream {
	return NewStream(s.Name, s.Key, false)
}

type API struct {
	streamer *Registry
}

func NewAPI(r *Registry) *API {
	return &API{
		streamer: r,
	}
}

func (api *API) SubscribeStream(peerId enode.ID, s Stream, history *Range, priority uint8) error {
	return api.streamer.Subscribe(peerId, s, history, priority)
}

func (api *API) UnsubscribeStream(peerId enode.ID, s Stream) error {
	return api.streamer.Unsubscribe(peerId, s)
}
