
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package metrics

import (
	"math"
	"sync"
	"sync/atomic"
)

//EWMA连续计算指数加权移动平均值
//基于外部时钟信号源。
type EWMA interface {
	Rate() float64
	Snapshot() EWMA
	Tick()
	Update(int64)
}

//new ewma用给定的alpha构造一个新的ewma。
func NewEWMA(alpha float64) EWMA {
	return &StandardEWMA{alpha: alpha}
}

//newewma1构建一个新的ewma，移动平均值为一分钟。
func NewEWMA1() EWMA {
	return NewEWMA(1 - math.Exp(-5.0/60.0/1))
}

//newewma5构建一个新的ewma，平均移动5分钟。
func NewEWMA5() EWMA {
	return NewEWMA(1 - math.Exp(-5.0/60.0/5))
}

//newewma15构建一个新的ewma，平均移动15分钟。
func NewEWMA15() EWMA {
	return NewEWMA(1 - math.Exp(-5.0/60.0/15))
}

//ewmasnapshot是另一个ewma的只读副本。
type EWMASnapshot float64

//Rate返回快照时每秒事件的速率
//拿。
func (a EWMASnapshot) Rate() float64 { return float64(a) }

//快照返回快照。
func (a EWMASnapshot) Snapshot() EWMA { return a }

//蜱恐慌。
func (EWMASnapshot) Tick() {
	panic("Tick called on an EWMASnapshot")
}

//更新恐慌。
func (EWMASnapshot) Update(int64) {
	panic("Update called on an EWMASnapshot")
}

//尼罗河流域是一个绝无仅有的EWMA。
type NilEWMA struct{}

//价格是不允许的。
func (NilEWMA) Rate() float64 { return 0.0 }

//快照是不可操作的。
func (NilEWMA) Snapshot() EWMA { return NilEWMA{} }

//滴答声是禁止的。
func (NilEWMA) Tick() {}

//更新是不可操作的。
func (NilEWMA) Update(n int64) {}

//StandardEWMA是EWMA的标准实现并跟踪
//对未计数的事件进行处理。它使用
//同步/原子包以管理未计数的事件。
type StandardEWMA struct {
uncounted int64 //！\这应该是确保64位对齐的第一个成员
	alpha     float64
	rate      float64
	init      bool
	mutex     sync.Mutex
}

//Rate返回每秒事件的移动平均速率。
func (a *StandardEWMA) Rate() float64 {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.rate * float64(1e9)
}

//快照返回EWMA的只读副本。
func (a *StandardEWMA) Snapshot() EWMA {
	return EWMASnapshot(a.Rate())
}

//勾选时钟以更新移动平均值。它假定它被调用
//每五秒钟。
func (a *StandardEWMA) Tick() {
	count := atomic.LoadInt64(&a.uncounted)
	atomic.AddInt64(&a.uncounted, -count)
	instantRate := float64(count) / float64(5e9)
	a.mutex.Lock()
	defer a.mutex.Unlock()
	if a.init {
		a.rate += a.alpha * (instantRate - a.rate)
	} else {
		a.init = true
		a.rate = instantRate
	}
}

//更新添加了n个未计数的事件。
func (a *StandardEWMA) Update(n int64) {
	atomic.AddInt64(&a.uncounted, n)
}
