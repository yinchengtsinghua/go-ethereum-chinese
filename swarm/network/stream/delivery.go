
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

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/spancontext"
	"github.com/ethereum/go-ethereum/swarm/storage"
	opentracing "github.com/opentracing/opentracing-go"
)

const (
	swarmChunkServerStreamName = "RETRIEVE_REQUEST"
	deliveryCap                = 32
)

var (
	processReceivedChunksCount    = metrics.NewRegisteredCounter("network.stream.received_chunks.count", nil)
	handleRetrieveRequestMsgCount = metrics.NewRegisteredCounter("network.stream.handle_retrieve_request_msg.count", nil)
	retrieveChunkFail             = metrics.NewRegisteredCounter("network.stream.retrieve_chunks_fail.count", nil)

	requestFromPeersCount     = metrics.NewRegisteredCounter("network.stream.request_from_peers.count", nil)
	requestFromPeersEachCount = metrics.NewRegisteredCounter("network.stream.request_from_peers_each.count", nil)
)

type Delivery struct {
	chunkStore storage.SyncChunkStore
	kad        *network.Kademlia
	getPeer    func(enode.ID) *Peer
}

func NewDelivery(kad *network.Kademlia, chunkStore storage.SyncChunkStore) *Delivery {
	return &Delivery{
		chunkStore: chunkStore,
		kad:        kad,
	}
}

//swarmchunkserver实现服务器
type SwarmChunkServer struct {
	deliveryC  chan []byte
	batchC     chan []byte
	chunkStore storage.ChunkStore
	currentLen uint64
	quit       chan struct{}
}

//newswarmchunkserver是swarmchunkserver构造函数
func NewSwarmChunkServer(chunkStore storage.ChunkStore) *SwarmChunkServer {
	s := &SwarmChunkServer{
		deliveryC:  make(chan []byte, deliveryCap),
		batchC:     make(chan []byte),
		chunkStore: chunkStore,
		quit:       make(chan struct{}),
	}
	go s.processDeliveries()
	return s
}

//processDeliveries处理已传递的块散列
func (s *SwarmChunkServer) processDeliveries() {
	var hashes []byte
	var batchC chan []byte
	for {
		select {
		case <-s.quit:
			return
		case hash := <-s.deliveryC:
			hashes = append(hashes, hash...)
			batchC = s.batchC
		case batchC <- hashes:
			hashes = nil
			batchC = nil
		}
	}
}

//对于swarmchunkserver，sessionindex在所有情况下都返回零。
func (s *SwarmChunkServer) SessionIndex() (uint64, error) {
	return 0, nil
}

//设置下一批
func (s *SwarmChunkServer) SetNextBatch(_, _ uint64) (hashes []byte, from uint64, to uint64, proof *HandoverProof, err error) {
	select {
	case hashes = <-s.batchC:
	case <-s.quit:
		return
	}

	from = s.currentLen
	s.currentLen += uint64(len(hashes))
	to = s.currentLen
	return
}

//需要在流服务器上调用Close
func (s *SwarmChunkServer) Close() {
	close(s.quit)
}

//GetData从数据库存储检索块数据
func (s *SwarmChunkServer) GetData(ctx context.Context, key []byte) ([]byte, error) {
	chunk, err := s.chunkStore.Get(ctx, storage.Address(key))
	if err != nil {
		return nil, err
	}
	return chunk.Data(), nil
}

//retrieverequestmsg是块检索请求的协议msg
type RetrieveRequestMsg struct {
	Addr      storage.Address
	SkipCheck bool
	HopCount  uint8
}

func (d *Delivery) handleRetrieveRequestMsg(ctx context.Context, sp *Peer, req *RetrieveRequestMsg) error {
	log.Trace("received request", "peer", sp.ID(), "hash", req.Addr)
	handleRetrieveRequestMsgCount.Inc(1)

	var osp opentracing.Span
	ctx, osp = spancontext.StartSpan(
		ctx,
		"retrieve.request")
	defer osp.Finish()

	s, err := sp.getServer(NewStream(swarmChunkServerStreamName, "", true))
	if err != nil {
		return err
	}
	streamer := s.Server.(*SwarmChunkServer)

	var cancel func()
//TODO:用这个硬编码超时做点什么，将来可能使用TTL
	ctx = context.WithValue(ctx, "peer", sp.ID().String())
	ctx = context.WithValue(ctx, "hopcount", req.HopCount)
	ctx, cancel = context.WithTimeout(ctx, network.RequestTimeout)

	go func() {
		select {
		case <-ctx.Done():
		case <-streamer.quit:
		}
		cancel()
	}()

	go func() {
		chunk, err := d.chunkStore.Get(ctx, req.Addr)
		if err != nil {
			retrieveChunkFail.Inc(1)
			log.Debug("ChunkStore.Get can not retrieve chunk", "peer", sp.ID().String(), "addr", req.Addr, "hopcount", req.HopCount, "err", err)
			return
		}
		if req.SkipCheck {
			syncing := false
			err = sp.Deliver(ctx, chunk, s.priority, syncing)
			if err != nil {
				log.Warn("ERROR in handleRetrieveRequestMsg", "err", err)
			}
			return
		}
		select {
		case streamer.deliveryC <- chunk.Address()[:]:
		case <-streamer.quit:
		}

	}()

	return nil
}

//区块传送总是使用相同的讯息类型….
type ChunkDeliveryMsg struct {
	Addr  storage.Address
SData []byte //存储的块数据（包括大小）
peer  *Peer  //设置在handlechunkdeliverymsg中
}

//…但如果交换记帐是用于同步或检索的传递，则需要消除歧义。
//根据消息类型决定是否需要解释此消息

//定义用于检索的块传递（使用记帐）
type ChunkDeliveryMsgRetrieval ChunkDeliveryMsg

//定义用于同步的块传递（无记帐）
type ChunkDeliveryMsgSyncing ChunkDeliveryMsg

//TODO:修复上下文snafu
func (d *Delivery) handleChunkDeliveryMsg(ctx context.Context, sp *Peer, req *ChunkDeliveryMsg) error {
	var osp opentracing.Span
	ctx, osp = spancontext.StartSpan(
		ctx,
		"chunk.delivery")
	defer osp.Finish()

	processReceivedChunksCount.Inc(1)

	go func() {
		req.peer = sp
		err := d.chunkStore.Put(ctx, storage.NewChunk(req.Addr, req.SData))
		if err != nil {
			if err == storage.ErrChunkInvalid {
//我们删除了这个日志，因为它会破坏日志
//TODO:启用此日志行
//log.warn（“传递的块无效”，“对等”，sp.id（），“块”，req.addr，）
				req.peer.Drop(err)
			}
		}
	}()
	return nil
}

//RequestFromPeers将块检索请求发送到
func (d *Delivery) RequestFromPeers(ctx context.Context, req *network.Request) (*enode.ID, chan struct{}, error) {
	requestFromPeersCount.Inc(1)
	var sp *Peer
	spID := req.Source

	if spID != nil {
		sp = d.getPeer(*spID)
		if sp == nil {
			return nil, nil, fmt.Errorf("source peer %v not found", spID.String())
		}
	} else {
		d.kad.EachConn(req.Addr[:], 255, func(p *network.Peer, po int) bool {
			id := p.ID()
			if p.LightNode {
//跳过灯光节点
				return true
			}
			if req.SkipPeer(id.String()) {
				log.Trace("Delivery.RequestFromPeers: skip peer", "peer id", id)
				return true
			}
			sp = d.getPeer(id)
			if sp == nil {
//log.warn（“delivery.requestfrompeers:找不到对等机”，“id”，id）
				return true
			}
			spID = &id
			return false
		})
		if sp == nil {
			return nil, nil, errors.New("no peer found")
		}
	}

	err := sp.SendPriority(ctx, &RetrieveRequestMsg{
		Addr:      req.Addr,
		SkipCheck: req.SkipCheck,
		HopCount:  req.HopCount,
	}, Top)
	if err != nil {
		return nil, nil, err
	}
	requestFromPeersEachCount.Inc(1)

	return spID, sp.quit, nil
}
