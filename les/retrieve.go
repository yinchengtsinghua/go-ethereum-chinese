
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

//package light实现可按需检索的状态和链对象
//对于以太坊Light客户端。
package les

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/light"
)

var (
	retryQueue         = time.Millisecond * 100
	softRequestTimeout = time.Millisecond * 500
	hardRequestTimeout = time.Second * 10
)

//RetrieveManager是一个位于RequestDistributor之上的层，负责
//按请求ID匹配答复，并处理超时，必要时重新发送。
type retrieveManager struct {
	dist       *requestDistributor
	peers      *peerSet
	serverPool peerSelector

	lock     sync.RWMutex
	sentReqs map[uint64]*sentReq
}

//validatorfunc是一个处理回复消息的函数
type validatorFunc func(distPeer, *Msg) error

//peerSelector receives feedback info about response times and timeouts
type peerSelector interface {
	adjustResponseTime(*poolEntry, time.Duration, bool)
}

//sentreq表示由retrievemanager发送和跟踪的请求
type sentReq struct {
	rm       *retrieveManager
	req      *distReq
	id       uint64
	validate validatorFunc

	eventsCh chan reqPeerEvent
	stopCh   chan struct{}
	stopped  bool
	err      error

lock   sync.RWMutex //保护对Sento地图的访问
	sentTo map[distPeer]sentReqToPeer

lastReqQueued bool     //上一个请求已排队，但未发送
lastReqSentTo distPeer //如果不是零，则最后一个请求已发送给给定的对等方，但未超时。
reqSrtoCount  int      //达到软（但非硬）超时的请求数
}

//SentReqTopeer通知来自对等Goroutine（TryRequest）的请求有关响应
//由给定的对等方传递。每个对等端的每个请求只允许一次传递，
//在此之后，delivered设置为true，响应的有效性将发送到
//有效通道，不接受其他响应。
type sentReqToPeer struct {
	delivered bool
	valid     chan bool
}

//ReqPeerEvent由Peer Goroutine（TryRequest）的请求发送到
//通过eventsch通道请求状态机（retrieveloop）。
type reqPeerEvent struct {
	event int
	peer  distPeer
}

const (
rpSent = iota //if peer == nil, not sent (no suitable peers)
	rpSoftTimeout
	rpHardTimeout
	rpDeliveredValid
	rpDeliveredInvalid
)

//newRetrieveManager创建检索管理器
func newRetrieveManager(peers *peerSet, dist *requestDistributor, serverPool peerSelector) *retrieveManager {
	return &retrieveManager{
		peers:      peers,
		dist:       dist,
		serverPool: serverPool,
		sentReqs:   make(map[uint64]*sentReq),
	}
}

//检索发送请求（必要时发送给多个对等方）并等待响应
//通过传递函数传递并由
//验证程序回调。当传递有效答案或上下文为
//取消。
func (rm *retrieveManager) retrieve(ctx context.Context, reqID uint64, req *distReq, val validatorFunc, shutdown chan struct{}) error {
	sentReq := rm.sendReq(reqID, req, val)
	select {
	case <-sentReq.stopCh:
	case <-ctx.Done():
		sentReq.stop(ctx.Err())
	case <-shutdown:
		sentReq.stop(fmt.Errorf("Client is shutting down"))
	}
	return sentReq.getError()
}

//sendreq启动一个进程，该进程不断尝试为
//在停止或成功之前，从任何合适的对等方请求。
func (rm *retrieveManager) sendReq(reqID uint64, req *distReq, val validatorFunc) *sentReq {
	r := &sentReq{
		rm:       rm,
		req:      req,
		id:       reqID,
		sentTo:   make(map[distPeer]sentReqToPeer),
		stopCh:   make(chan struct{}),
		eventsCh: make(chan reqPeerEvent, 10),
		validate: val,
	}

	canSend := req.canSend
	req.canSend = func(p distPeer) bool {
//为cansend添加额外的检查：请求以前没有发送到同一个对等机
		r.lock.RLock()
		_, sent := r.sentTo[p]
		r.lock.RUnlock()
		return !sent && canSend(p)
	}

	request := req.request
	req.request = func(p distPeer) func() {
//在实际发送请求之前，在sentto映射中输入一个条目
		r.lock.Lock()
		r.sentTo[p] = sentReqToPeer{false, make(chan bool, 1)}
		r.lock.Unlock()
		return request(p)
	}
	rm.lock.Lock()
	rm.sentReqs[reqID] = r
	rm.lock.Unlock()

	go r.retrieveLoop()
	return r
}

//LES协议管理器调用Deliver将回复消息传递给等待的请求。
func (rm *retrieveManager) deliver(peer distPeer, msg *Msg) error {
	rm.lock.RLock()
	req, ok := rm.sentReqs[msg.ReqID]
	rm.lock.RUnlock()

	if ok {
		return req.deliver(peer, msg)
	}
	return errResp(ErrUnexpectedResponse, "reqID = %v", msg.ReqID)
}

//reqstatefn表示检索循环状态机的状态
type reqStateFn func() reqStateFn

//RetrieveLoop是检索状态机事件循环
func (r *sentReq) retrieveLoop() {
	go r.tryRequest()
	r.lastReqQueued = true
	state := r.stateRequesting

	for state != nil {
		state = state()
	}

	r.rm.lock.Lock()
	delete(r.rm.sentReqs, r.id)
	r.rm.lock.Unlock()
}

//状态请求：请求最近已排队或发送；当达到软超时时，
//将向新对等发送新请求
func (r *sentReq) stateRequesting() reqStateFn {
	select {
	case ev := <-r.eventsCh:
		r.update(ev)
		switch ev.event {
		case rpSent:
			if ev.peer == nil {
//请求发送失败，没有合适的对等机
				if r.waiting() {
//我们已经在等待可能成功的已发送请求，请继续等待
					return r.stateNoMorePeers
				}
//无需等待，无需询问其他对等方，返回时出错
				r.stop(light.ErrNoPeers)
//无需转到停止状态，因为waiting（）已返回false
				return nil
			}
		case rpSoftTimeout:
//上次请求超时，请尝试询问新的对等方
			go r.tryRequest()
			r.lastReqQueued = true
			return r.stateRequesting
		case rpDeliveredInvalid:
//如果是上次发送的请求（更新时设置为nil），则启动新的请求。
			if !r.lastReqQueued && r.lastReqSentTo == nil {
				go r.tryRequest()
				r.lastReqQueued = true
			}
			return r.stateRequesting
		case rpDeliveredValid:
			r.stop(nil)
			return r.stateStopped
		}
		return r.stateRequesting
	case <-r.stopCh:
		return r.stateStopped
	}
}

//StateNoMorepeers:无法发送更多请求，因为没有合适的对等机可用。
//对等方稍后可能会适合某个请求，或者可能会出现新的对等方，因此我们
//继续努力。
func (r *sentReq) stateNoMorePeers() reqStateFn {
	select {
	case <-time.After(retryQueue):
		go r.tryRequest()
		r.lastReqQueued = true
		return r.stateRequesting
	case ev := <-r.eventsCh:
		r.update(ev)
		if ev.event == rpDeliveredValid {
			r.stop(nil)
			return r.stateStopped
		}
		if r.waiting() {
			return r.stateNoMorePeers
		}
		r.stop(light.ErrNoPeers)
		return nil
	case <-r.stopCh:
		return r.stateStopped
	}
}

//Statestopped:请求成功或取消，只是等待一些对等方
//要么回答，要么很难超时
func (r *sentReq) stateStopped() reqStateFn {
	for r.waiting() {
		r.update(<-r.eventsCh)
	}
	return nil
}

//更新根据事件更新排队/发送标志和超时对等计数器
func (r *sentReq) update(ev reqPeerEvent) {
	switch ev.event {
	case rpSent:
		r.lastReqQueued = false
		r.lastReqSentTo = ev.peer
	case rpSoftTimeout:
		r.lastReqSentTo = nil
		r.reqSrtoCount++
	case rpHardTimeout:
		r.reqSrtoCount--
	case rpDeliveredValid, rpDeliveredInvalid:
		if ev.peer == r.lastReqSentTo {
			r.lastReqSentTo = nil
		} else {
			r.reqSrtoCount--
		}
	}
}

//如果检索机制正在等待来自的答案，则waiting返回true。
//任何同龄人
func (r *sentReq) waiting() bool {
	return r.lastReqQueued || r.lastReqSentTo != nil || r.reqSrtoCount > 0
}

//TryRequest尝试将请求发送到新的对等机，并等待它
//succeed or time out if it has been sent. It also sends the appropriate reqPeerEvent
//发送到请求的事件通道的消息。
func (r *sentReq) tryRequest() {
	sent := r.rm.dist.queue(r.req)
	var p distPeer
	select {
	case p = <-sent:
	case <-r.stopCh:
		if r.rm.dist.cancel(r.req) {
			p = nil
		} else {
			p = <-sent
		}
	}

	r.eventsCh <- reqPeerEvent{rpSent, p}
	if p == nil {
		return
	}

	reqSent := mclock.Now()
	srto, hrto := false, false

	r.lock.RLock()
	s, ok := r.sentTo[p]
	r.lock.RUnlock()
	if !ok {
		panic(nil)
	}

	defer func() {
//向服务器池发送反馈，并在发生硬超时时删除对等机
		pp, ok := p.(*peer)
		if ok && r.rm.serverPool != nil {
			respTime := time.Duration(mclock.Now() - reqSent)
			r.rm.serverPool.adjustResponseTime(pp.poolEntry, respTime, srto)
		}
		if hrto {
			pp.Log().Debug("Request timed out hard")
			if r.rm.peers != nil {
				r.rm.peers.Unregister(pp.id)
			}
		}

		r.lock.Lock()
		delete(r.sentTo, p)
		r.lock.Unlock()
	}()

	select {
	case ok := <-s.valid:
		if ok {
			r.eventsCh <- reqPeerEvent{rpDeliveredValid, p}
		} else {
			r.eventsCh <- reqPeerEvent{rpDeliveredInvalid, p}
		}
		return
	case <-time.After(softRequestTimeout):
		srto = true
		r.eventsCh <- reqPeerEvent{rpSoftTimeout, p}
	}

	select {
	case ok := <-s.valid:
		if ok {
			r.eventsCh <- reqPeerEvent{rpDeliveredValid, p}
		} else {
			r.eventsCh <- reqPeerEvent{rpDeliveredInvalid, p}
		}
	case <-time.After(hardRequestTimeout):
		hrto = true
		r.eventsCh <- reqPeerEvent{rpHardTimeout, p}
	}
}

//传递属于此请求的答复
func (r *sentReq) deliver(peer distPeer, msg *Msg) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	s, ok := r.sentTo[peer]
	if !ok || s.delivered {
		return errResp(ErrUnexpectedResponse, "reqID = %v", msg.ReqID)
	}
	valid := r.validate(peer, msg) == nil
	r.sentTo[peer] = sentReqToPeer{true, s.valid}
	s.valid <- valid
	if !valid {
		return errResp(ErrInvalidResponse, "reqID = %v", msg.ReqID)
	}
	return nil
}

//停止停止检索进程并设置将返回的错误代码
//通过获得错误
func (r *sentReq) stop(err error) {
	r.lock.Lock()
	if !r.stopped {
		r.stopped = true
		r.err = err
		close(r.stopCh)
	}
	r.lock.Unlock()
}

//GetError返回任何检索错误（由
//停止功能）停止后
func (r *sentReq) getError() error {
	return r.err
}

//genreqid生成新的随机请求ID
func genReqID() uint64 {
	var rnd [8]byte
	rand.Read(rnd[:])
	return binary.BigEndian.Uint64(rnd[:])
}
