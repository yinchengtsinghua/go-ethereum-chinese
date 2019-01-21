
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

package mclock

import (
	"sync"
	"time"
)

//模拟实现了一个可重复的时间敏感测试的虚拟时钟。它
//在实际处理占用零时间的虚拟时间刻度上模拟调度程序。
//
//虚拟时钟本身不前进，调用run前进并执行计时器。
//由于无法影响Go调度程序，因此测试涉及的超时行为
//戈罗蒂内斯需要特别照顾。测试这种超时的一个好方法是：首先
//执行应该超时的操作。确保要测试的计时器
//创建。然后运行时钟直到超时之后。最后观察
//使用通道或信号量的超时。
type Simulated struct {
	now       AbsTime
	scheduled []event
	mu        sync.RWMutex
	cond      *sync.Cond
}

type event struct {
	do func()
	at AbsTime
}

//RUN按给定的持续时间移动时钟，在该持续时间之前执行所有计时器。
func (s *Simulated) Run(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.init()

	end := s.now + AbsTime(d)
	for len(s.scheduled) > 0 {
		ev := s.scheduled[0]
		if ev.at > end {
			break
		}
		s.now = ev.at
		ev.do()
		s.scheduled = s.scheduled[1:]
	}
	s.now = end
}

func (s *Simulated) ActiveTimers() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.scheduled)
}

func (s *Simulated) WaitForTimers(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.init()

	for len(s.scheduled) < n {
		s.cond.Wait()
	}
}

//现在实现时钟。
func (s *Simulated) Now() AbsTime {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.now
}

//睡眠实现时钟。
func (s *Simulated) Sleep(d time.Duration) {
	<-s.After(d)
}

//在执行时钟之后。
func (s *Simulated) After(d time.Duration) <-chan time.Time {
	after := make(chan time.Time, 1)
	s.insert(d, func() {
		after <- (time.Time{}).Add(time.Duration(s.now))
	})
	return after
}

func (s *Simulated) insert(d time.Duration, do func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.init()

	at := s.now + AbsTime(d)
	l, h := 0, len(s.scheduled)
	ll := h
	for l != h {
		m := (l + h) / 2
		if at < s.scheduled[m].at {
			h = m
		} else {
			l = m + 1
		}
	}
	s.scheduled = append(s.scheduled, event{})
	copy(s.scheduled[l+1:], s.scheduled[l:ll])
	s.scheduled[l] = event{do: do, at: at}
	s.cond.Broadcast()
}

func (s *Simulated) init() {
	if s.cond == nil {
		s.cond = sync.NewCond(&s.mu)
	}
}
