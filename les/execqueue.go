
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

package les

import "sync"

//execqueue实现在单个线程中执行函数调用的队列，
//与排队的顺序相同。
type execQueue struct {
	mu        sync.Mutex
	cond      *sync.Cond
	funcs     []func()
	closeWait chan struct{}
}

//NeXeExcel队列创建一个新的执行队列。
func newExecQueue(capacity int) *execQueue {
	q := &execQueue{funcs: make([]func(), 0, capacity)}
	q.cond = sync.NewCond(&q.mu)
	go q.loop()
	return q
}

func (q *execQueue) loop() {
	for f := q.waitNext(false); f != nil; f = q.waitNext(true) {
		f()
	}
	close(q.closeWait)
}

func (q *execQueue) waitNext(drop bool) (f func()) {
	q.mu.Lock()
	if drop {
//删除刚刚执行的函数。我们在这里而不是在什么时候
//出列so len（q.funcs）包括正在运行的函数。
		q.funcs = append(q.funcs[:0], q.funcs[1:]...)
	}
	for !q.isClosed() {
		if len(q.funcs) > 0 {
			f = q.funcs[0]
			break
		}
		q.cond.Wait()
	}
	q.mu.Unlock()
	return f
}

func (q *execQueue) isClosed() bool {
	return q.closeWait != nil
}

//如果可以向执行队列中添加更多函数调用，则can queue返回true。
func (q *execQueue) canQueue() bool {
	q.mu.Lock()
	ok := !q.isClosed() && len(q.funcs) < cap(q.funcs)
	q.mu.Unlock()
	return ok
}

//queue向执行队列添加函数调用。如果成功，则返回true。
func (q *execQueue) queue(f func()) bool {
	q.mu.Lock()
	ok := !q.isClosed() && len(q.funcs) < cap(q.funcs)
	if ok {
		q.funcs = append(q.funcs, f)
		q.cond.Signal()
	}
	q.mu.Unlock()
	return ok
}

//quit停止执行队列。
//quit在返回之前等待当前执行完成。
func (q *execQueue) quit() {
	q.mu.Lock()
	if !q.isClosed() {
		q.closeWait = make(chan struct{})
		q.cond.Signal()
	}
	q.mu.Unlock()
	<-q.closeWait
}
