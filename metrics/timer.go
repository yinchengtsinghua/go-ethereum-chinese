
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package metrics

import (
	"sync"
	"time"
)

//计时器捕获事件的持续时间和速率。
type Timer interface {
	Count() int64
	Max() int64
	Mean() float64
	Min() int64
	Percentile(float64) float64
	Percentiles([]float64) []float64
	Rate1() float64
	Rate5() float64
	Rate15() float64
	RateMean() float64
	Snapshot() Timer
	StdDev() float64
	Stop()
	Sum() int64
	Time(func())
	Update(time.Duration)
	UpdateSince(time.Time)
	Variance() float64
}

//GetOrRegisterTimer返回现有计时器或构造并注册
//新标准计时器。
//一旦没有必要从注册表中注销仪表
//允许垃圾收集。
func GetOrRegisterTimer(name string, r Registry) Timer {
	if nil == r {
		r = DefaultRegistry
	}
	return r.GetOrRegister(name, NewTimer).(Timer)
}

//NewCustomTimer从柱状图和仪表构造新的StandardTimer。
//确保在计时器不允许垃圾收集时调用stop（）。
func NewCustomTimer(h Histogram, m Meter) Timer {
	if !Enabled {
		return NilTimer{}
	}
	return &StandardTimer{
		histogram: h,
		meter:     m,
	}
}

//NewRegisteredTimer构造并注册新的StandardTimer。
//一旦没有必要从注册表中注销仪表
//允许垃圾收集。
func NewRegisteredTimer(name string, r Registry) Timer {
	c := NewTimer()
	if nil == r {
		r = DefaultRegistry
	}
	r.Register(name, c)
	return c
}

//NewTimer使用指数衰减构造新的StandardTimer
//具有与Unix平均负荷相同的水库大小和alpha的样本。
//确保在计时器不允许垃圾收集时调用stop（）。
func NewTimer() Timer {
	if !Enabled {
		return NilTimer{}
	}
	return &StandardTimer{
		histogram: NewHistogram(NewExpDecaySample(1028, 0.015)),
		meter:     NewMeter(),
	}
}

//niltimer是一个无操作计时器。
type NilTimer struct {
	h Histogram
	m Meter
}

//计数是不允许的。
func (NilTimer) Count() int64 { return 0 }

//马克斯不是一个OP。
func (NilTimer) Max() int64 { return 0 }

//平均值是不允许的。
func (NilTimer) Mean() float64 { return 0.0 }

//min是NO-OP。
func (NilTimer) Min() int64 { return 0 }

//百分位数是不允许的。
func (NilTimer) Percentile(p float64) float64 { return 0.0 }

//百分位数是不允许的。
func (NilTimer) Percentiles(ps []float64) []float64 {
	return make([]float64, len(ps))
}

//RATE1是不可操作的。
func (NilTimer) Rate1() float64 { return 0.0 }

//RATE5是不允许的。
func (NilTimer) Rate5() float64 { return 0.0 }

//RATE15是不允许的。
func (NilTimer) Rate15() float64 { return 0.0 }

//RateMean是一个不允许的人。
func (NilTimer) RateMean() float64 { return 0.0 }

//快照是不可操作的。
func (NilTimer) Snapshot() Timer { return NilTimer{} }

//stdev是一个no-op。
func (NilTimer) StdDev() float64 { return 0.0 }

//停止是不允许的。
func (NilTimer) Stop() {}

//和是一个NO-op.
func (NilTimer) Sum() int64 { return 0 }

//时间是不允许的。
func (NilTimer) Time(func()) {}

//更新是不可操作的。
func (NilTimer) Update(time.Duration) {}

//updateSince是一个no-op。
func (NilTimer) UpdateSince(time.Time) {}

//方差是不可操作的。
func (NilTimer) Variance() float64 { return 0.0 }

//StandardTimer是计时器的标准实现，使用柱状图
//和米。
type StandardTimer struct {
	histogram Histogram
	meter     Meter
	mutex     sync.Mutex
}

//count返回记录的事件数。
func (t *StandardTimer) Count() int64 {
	return t.histogram.Count()
}

//max返回样本中的最大值。
func (t *StandardTimer) Max() int64 {
	return t.histogram.Max()
}

//mean返回样本值的平均值。
func (t *StandardTimer) Mean() float64 {
	return t.histogram.Mean()
}

//Min返回样本中的最小值。
func (t *StandardTimer) Min() int64 {
	return t.histogram.Min()
}

//Percentile返回样本中任意百分位数的值。
func (t *StandardTimer) Percentile(p float64) float64 {
	return t.histogram.Percentile(p)
}

//Percentiles返回
//样品。
func (t *StandardTimer) Percentiles(ps []float64) []float64 {
	return t.histogram.Percentiles(ps)
}

//Rate1返回每秒事件的一分钟移动平均速率。
func (t *StandardTimer) Rate1() float64 {
	return t.meter.Rate1()
}

//Rate5返回每秒事件的5分钟移动平均速率。
func (t *StandardTimer) Rate5() float64 {
	return t.meter.Rate5()
}

//Rate15返回每秒事件的15分钟移动平均速率。
func (t *StandardTimer) Rate15() float64 {
	return t.meter.Rate15()
}

//RateMean返回仪表每秒事件的平均速率。
func (t *StandardTimer) RateMean() float64 {
	return t.meter.RateMean()
}

//快照返回计时器的只读副本。
func (t *StandardTimer) Snapshot() Timer {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return &TimerSnapshot{
		histogram: t.histogram.Snapshot().(*HistogramSnapshot),
		meter:     t.meter.Snapshot().(*MeterSnapshot),
	}
}

//stdev返回样本值的标准偏差。
func (t *StandardTimer) StdDev() float64 {
	return t.histogram.StdDev()
}

//停止停止计时表。
func (t *StandardTimer) Stop() {
	t.meter.Stop()
}

//sum返回样本中的和。
func (t *StandardTimer) Sum() int64 {
	return t.histogram.Sum()
}

//记录给定函数执行的持续时间。
func (t *StandardTimer) Time(f func()) {
	ts := time.Now()
	f()
	t.Update(time.Since(ts))
}

//记录事件的持续时间。
func (t *StandardTimer) Update(d time.Duration) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.histogram.Update(int64(d))
	t.meter.Mark(1)
}

//记录一次开始到现在结束的事件的持续时间。
func (t *StandardTimer) UpdateSince(ts time.Time) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.histogram.Update(int64(time.Since(ts)))
	t.meter.Mark(1)
}

//方差返回样本中值的方差。
func (t *StandardTimer) Variance() float64 {
	return t.histogram.Variance()
}

//TimersSnapshot是另一个计时器的只读副本。
type TimerSnapshot struct {
	histogram *HistogramSnapshot
	meter     *MeterSnapshot
}

//count返回快照时记录的事件数
//拿。
func (t *TimerSnapshot) Count() int64 { return t.histogram.Count() }

//max返回快照拍摄时的最大值。
func (t *TimerSnapshot) Max() int64 { return t.histogram.Max() }

//mean返回拍摄快照时的平均值。
func (t *TimerSnapshot) Mean() float64 { return t.histogram.Mean() }

//Min返回拍摄快照时的最小值。
func (t *TimerSnapshot) Min() int64 { return t.histogram.Min() }

//percentile返回在
//已拍摄快照。
func (t *TimerSnapshot) Percentile(p float64) float64 {
	return t.histogram.Percentile(p)
}

//Percentiles返回在
//拍摄快照的时间。
func (t *TimerSnapshot) Percentiles(ps []float64) []float64 {
	return t.histogram.Percentiles(ps)
}

//RATE1返回在
//拍摄快照的时间。
func (t *TimerSnapshot) Rate1() float64 { return t.meter.Rate1() }

//RATE5返回每秒5分钟的移动平均事件速率
//拍摄快照的时间。
func (t *TimerSnapshot) Rate5() float64 { return t.meter.Rate5() }

//Rate15返回每秒15分钟的事件移动平均速率
//拍摄快照时。
func (t *TimerSnapshot) Rate15() float64 { return t.meter.Rate15() }

//RateMean返回在
//已拍摄快照。
func (t *TimerSnapshot) RateMean() float64 { return t.meter.RateMean() }

//快照返回快照。
func (t *TimerSnapshot) Snapshot() Timer { return t }

//stdev返回快照时值的标准偏差
//被带走了。
func (t *TimerSnapshot) StdDev() float64 { return t.histogram.StdDev() }

//停止是不允许的。
func (t *TimerSnapshot) Stop() {}

//sum返回拍摄快照时的总和。
func (t *TimerSnapshot) Sum() int64 { return t.histogram.Sum() }

//时间恐慌。
func (*TimerSnapshot) Time(func()) {
	panic("Time called on a TimerSnapshot")
}

//更新恐慌。
func (*TimerSnapshot) Update(time.Duration) {
	panic("Update called on a TimerSnapshot")
}

//更新自恐慌。
func (*TimerSnapshot) UpdateSince(time.Time) {
	panic("UpdateSince called on a TimerSnapshot")
}

//variance返回快照时值的方差
//拿。
func (t *TimerSnapshot) Variance() float64 { return t.histogram.Variance() }
