
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

package bloombits

import (
	"sync"
)

//请求表示一个Bloom检索任务，以确定优先级并从本地
//数据库或从网络远程访问。
type request struct {
section uint64 //从中检索位向量的节索引
bit     uint   //段内的位索引，用于检索
}

//响应表示通过调度程序请求的位向量的状态。
type response struct {
cached []byte        //用于消除多个请求的缓存位
done   chan struct{} //允许等待完成的通道
}

//调度程序处理Bloom过滤器检索操作的调度
//整段批次属于一个单独的开孔钻头。除了安排
//retrieval operations, this struct also deduplicates the requests and caches
//即使在复杂的过滤中，也能最大限度地减少网络/数据库开销。
//情节。
type scheduler struct {
bit       uint                 //此计划程序负责的Bloom筛选器中位的索引
responses map[uint64]*response //当前挂起的检索请求或已缓存的响应
lock      sync.Mutex           //防止响应并发访问的锁
}

//Newscheduler为特定的
//位索引。
func newScheduler(idx uint) *scheduler {
	return &scheduler{
		bit:       idx,
		responses: make(map[uint64]*response),
	}
}

//运行创建检索管道，从节和
//通过完成通道以相同顺序返回结果。同时发生的
//允许运行同一个调度程序，从而导致检索任务重复数据消除。
func (s *scheduler) run(sections chan uint64, dist chan *request, done chan []byte, quit chan struct{}, wg *sync.WaitGroup) {
//在与相同大小的请求和响应之间创建转发器通道
//分配通道（因为这样会阻塞管道）。
	pend := make(chan uint64, cap(dist))

//启动管道调度程序在用户->分发服务器->用户之间转发
	wg.Add(2)
	go s.scheduleRequests(sections, dist, pend, quit, wg)
	go s.scheduleDeliveries(pend, done, quit, wg)
}

//重置将清除以前运行的所有剩余部分。这是在
//重新启动以确保以前没有请求但从未传递的状态将
//导致锁定。
func (s *scheduler) reset() {
	s.lock.Lock()
	defer s.lock.Unlock()

	for section, res := range s.responses {
		if res.cached == nil {
			delete(s.responses, section)
		}
	}
}

//schedulerequests从输入通道读取段检索请求，
//消除流中的重复数据并将唯一的检索任务推送到分发中
//数据库或网络层的通道。
func (s *scheduler) scheduleRequests(reqs chan uint64, dist chan *request, pend chan uint64, quit chan struct{}, wg *sync.WaitGroup) {
//完成后清理Goroutine和管道
	defer wg.Done()
	defer close(pend)

//继续阅读和安排分区请求
	for {
		select {
		case <-quit:
			return

		case section, ok := <-reqs:
//请求新分区检索
			if !ok {
				return
			}
//消除重复检索请求
			unique := false

			s.lock.Lock()
			if s.responses[section] == nil {
				s.responses[section] = &response{
					done: make(chan struct{}),
				}
				unique = true
			}
			s.lock.Unlock()

//安排检索部分的时间，并通知交付者期望此部分
			if unique {
				select {
				case <-quit:
					return
				case dist <- &request{bit: s.bit, section: section}:
				}
			}
			select {
			case <-quit:
				return
			case pend <- section:
			}
		}
	}
}

//ScheduledDeliveries读取区段接受通知并等待它们
//要传递，将它们推入输出数据缓冲区。
func (s *scheduler) scheduleDeliveries(pend chan uint64, done chan []byte, quit chan struct{}, wg *sync.WaitGroup) {
//完成后清理Goroutine和管道
	defer wg.Done()
	defer close(done)

//继续阅读通知并安排交货
	for {
		select {
		case <-quit:
			return

		case idx, ok := <-pend:
//新分区检索挂起
			if !ok {
				return
			}
//等到请求得到满足为止
			s.lock.Lock()
			res := s.responses[idx]
			s.lock.Unlock()

			select {
			case <-quit:
				return
			case <-res.done:
			}
//交付结果
			select {
			case <-quit:
				return
			case done <- res.cached:
			}
		}
	}
}

//请求分发服务器在对请求的答复到达时调用传递。
func (s *scheduler) deliver(sections []uint64, data [][]byte) {
	s.lock.Lock()
	defer s.lock.Unlock()

	for i, section := range sections {
if res := s.responses[section]; res != nil && res.cached == nil { //避免非请求和双倍交货
			res.cached = data[i]
			close(res.done)
		}
	}
}
