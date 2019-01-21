
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

//包优先级队列实现基于通道的优先级队列
//在任意类型上。它提供了一个
//一个自动操作循环，将一个函数应用于始终遵守的项
//他们的优先权。结构只是准一致的，即如果
//优先项是自动停止的，保证有一点
//当没有更高优先级的项目时，即不能保证
//有一点低优先级的项目存在
//但更高的不是

package priorityqueue

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/log"
)

var (
	ErrContention = errors.New("contention")

	errBadPriority = errors.New("bad priority")

	wakey = struct{}{}
)

//PriorityQueue是基本结构
type PriorityQueue struct {
	Queues []chan interface{}
	wakeup chan struct{}
}

//New是PriorityQueue的构造函数
func New(n int, l int) *PriorityQueue {
	var queues = make([]chan interface{}, n)
	for i := range queues {
		queues[i] = make(chan interface{}, l)
	}
	return &PriorityQueue{
		Queues: queues,
		wakeup: make(chan struct{}, 1),
	}
}

//运行是从队列中弹出项目的永久循环
func (pq *PriorityQueue) Run(ctx context.Context, f func(interface{})) {
	top := len(pq.Queues) - 1
	p := top
READ:
	for {
		q := pq.Queues[p]
		select {
		case <-ctx.Done():
			return
		case x := <-q:
			log.Trace("priority.queue f(x)", "p", p, "len(Queues[p])", len(pq.Queues[p]))
			f(x)
			p = top
		default:
			if p > 0 {
				p--
				log.Trace("priority.queue p > 0", "p", p)
				continue READ
			}
			p = top
			select {
			case <-ctx.Done():
				return
			case <-pq.wakeup:
				log.Trace("priority.queue wakeup", "p", p)
			}
		}
	}
}

//push将项目推送到priority参数中指定的适当队列
//如果给定了上下文，它将一直等到推送该项或上下文中止为止。
func (pq *PriorityQueue) Push(x interface{}, p int) error {
	if p < 0 || p >= len(pq.Queues) {
		return errBadPriority
	}
	log.Trace("priority.queue push", "p", p, "len(Queues[p])", len(pq.Queues[p]))
	select {
	case pq.Queues[p] <- x:
	default:
		return ErrContention
	}
	select {
	case pq.wakeup <- wakey:
	default:
	}
	return nil
}
