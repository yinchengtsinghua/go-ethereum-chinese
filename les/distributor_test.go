
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
	"math/rand"
	"sync"
	"testing"
	"time"
)

type testDistReq struct {
	cost, procTime, order uint64
	canSendTo             map[*testDistPeer]struct{}
}

func (r *testDistReq) getCost(dp distPeer) uint64 {
	return r.cost
}

func (r *testDistReq) canSend(dp distPeer) bool {
	_, ok := r.canSendTo[dp.(*testDistPeer)]
	return ok
}

func (r *testDistReq) request(dp distPeer) func() {
	return func() { dp.(*testDistPeer).send(r) }
}

type testDistPeer struct {
	sent    []*testDistReq
	sumCost uint64
	lock    sync.RWMutex
}

func (p *testDistPeer) send(r *testDistReq) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.sent = append(p.sent, r)
	p.sumCost += r.cost
}

func (p *testDistPeer) worker(t *testing.T, checkOrder bool, stop chan struct{}) {
	var last uint64
	for {
		wait := time.Millisecond
		p.lock.Lock()
		if len(p.sent) > 0 {
			rq := p.sent[0]
			wait = time.Duration(rq.procTime)
			p.sumCost -= rq.cost
			if checkOrder {
				if rq.order <= last {
					t.Errorf("Requests processed in wrong order")
				}
				last = rq.order
			}
			p.sent = p.sent[1:]
		}
		p.lock.Unlock()
		select {
		case <-stop:
			return
		case <-time.After(wait):
		}
	}
}

const (
	testDistBufLimit       = 10000000
	testDistMaxCost        = 1000000
	testDistPeerCount      = 5
	testDistReqCount       = 5000
	testDistMaxResendCount = 3
)

func (p *testDistPeer) waitBefore(cost uint64) (time.Duration, float64) {
	p.lock.RLock()
	sumCost := p.sumCost + cost
	p.lock.RUnlock()
	if sumCost < testDistBufLimit {
		return 0, float64(testDistBufLimit-sumCost) / float64(testDistBufLimit)
	}
	return time.Duration(sumCost - testDistBufLimit), 0
}

func (p *testDistPeer) canQueue() bool {
	return true
}

func (p *testDistPeer) queueSend(f func()) {
	f()
}

func TestRequestDistributor(t *testing.T) {
	testRequestDistributor(t, false)
}

func TestRequestDistributorResend(t *testing.T) {
	testRequestDistributor(t, true)
}

func testRequestDistributor(t *testing.T, resend bool) {
	stop := make(chan struct{})
	defer close(stop)

	dist := newRequestDistributor(nil, stop)
	var peers [testDistPeerCount]*testDistPeer
	for i := range peers {
		peers[i] = &testDistPeer{}
		go peers[i].worker(t, !resend, stop)
		dist.registerTestPeer(peers[i])
	}

	var wg sync.WaitGroup

	for i := 1; i <= testDistReqCount; i++ {
		cost := uint64(rand.Int63n(testDistMaxCost))
		procTime := uint64(rand.Int63n(int64(cost + 1)))
		rq := &testDistReq{
			cost:      cost,
			procTime:  procTime,
			order:     uint64(i),
			canSendTo: make(map[*testDistPeer]struct{}),
		}
		for _, peer := range peers {
			if rand.Intn(2) != 0 {
				rq.canSendTo[peer] = struct{}{}
			}
		}

		wg.Add(1)
		req := &distReq{
			getCost: rq.getCost,
			canSend: rq.canSend,
			request: rq.request,
		}
		chn := dist.queue(req)
		go func() {
			cnt := 1
			if resend && len(rq.canSendTo) != 0 {
				cnt = rand.Intn(testDistMaxResendCount) + 1
			}
			for i := 0; i < cnt; i++ {
				if i != 0 {
					chn = dist.queue(req)
				}
				p := <-chn
				if p == nil {
					if len(rq.canSendTo) != 0 {
						t.Errorf("Request that could have been sent was dropped")
					}
				} else {
					peer := p.(*testDistPeer)
					if _, ok := rq.canSendTo[peer]; !ok {
						t.Errorf("Request sent to wrong peer")
					}
				}
			}
			wg.Done()
		}()
		if rand.Intn(1000) == 0 {
			time.Sleep(time.Duration(rand.Intn(5000000)))
		}
	}

	wg.Wait()
}
