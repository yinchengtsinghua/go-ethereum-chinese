
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
	"sort"
	"sync"
	"time"
)

//存储在重置计时器中的值的初始切片容量
const InitialResettingTimerSliceCap = 10

//ResettingTimer用于存储计时器的聚合值，这些值在每个刷新间隔都会重置。
type ResettingTimer interface {
	Values() []int64
	Snapshot() ResettingTimer
	Percentiles([]float64) []int64
	Mean() float64
	Time(func())
	Update(time.Duration)
	UpdateSince(time.Time)
}

//GetOrRegisterResettingTimer返回现有的ResettingTimer或构造并注册
//新标准重置计时器。
func GetOrRegisterResettingTimer(name string, r Registry) ResettingTimer {
	if nil == r {
		r = DefaultRegistry
	}
	return r.GetOrRegister(name, NewResettingTimer).(ResettingTimer)
}

//NewRegisteredResettingTimer构造并注册新的StandardResettingTimer。
func NewRegisteredResettingTimer(name string, r Registry) ResettingTimer {
	c := NewResettingTimer()
	if nil == r {
		r = DefaultRegistry
	}
	r.Register(name, c)
	return c
}

//NewResettingTimer构造新的标准ResettingTimer
func NewResettingTimer() ResettingTimer {
	if !Enabled {
		return NilResettingTimer{}
	}
	return &StandardResettingTimer{
		values: make([]int64, 0, InitialResettingTimerSliceCap),
	}
}

//NilResettingTimer是一个不允许的ResettingTimer。
type NilResettingTimer struct {
}

//值是不可操作的。
func (NilResettingTimer) Values() []int64 { return nil }

//快照是不可操作的。
func (NilResettingTimer) Snapshot() ResettingTimer {
	return &ResettingTimerSnapshot{
		values: []int64{},
	}
}

//时间是不允许的。
func (NilResettingTimer) Time(func()) {}

//更新是不可操作的。
func (NilResettingTimer) Update(time.Duration) {}

//百分位数恐慌。
func (NilResettingTimer) Percentiles([]float64) []int64 {
	panic("Percentiles called on a NilResettingTimer")
}

//平均恐慌。
func (NilResettingTimer) Mean() float64 {
	panic("Mean called on a NilResettingTimer")
}

//updateSince是一个no-op。
func (NilResettingTimer) UpdateSince(time.Time) {}

//StandardResettingTimer是ResettingTimer的标准实现。
//和米。
type StandardResettingTimer struct {
	values []int64
	mutex  sync.Mutex
}

//值返回包含所有测量值的切片。
func (t *StandardResettingTimer) Values() []int64 {
	return t.values
}

//快照重置计时器并返回其内容的只读副本。
func (t *StandardResettingTimer) Snapshot() ResettingTimer {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	currentValues := t.values
	t.values = make([]int64, 0, InitialResettingTimerSliceCap)

	return &ResettingTimerSnapshot{
		values: currentValues,
	}
}

//百分位数恐慌。
func (t *StandardResettingTimer) Percentiles([]float64) []int64 {
	panic("Percentiles called on a StandardResettingTimer")
}

//平均恐慌。
func (t *StandardResettingTimer) Mean() float64 {
	panic("Mean called on a StandardResettingTimer")
}

//记录给定函数执行的持续时间。
func (t *StandardResettingTimer) Time(f func()) {
	ts := time.Now()
	f()
	t.Update(time.Since(ts))
}

//记录事件的持续时间。
func (t *StandardResettingTimer) Update(d time.Duration) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.values = append(t.values, int64(d))
}

//记录一次开始到现在结束的事件的持续时间。
func (t *StandardResettingTimer) UpdateSince(ts time.Time) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.values = append(t.values, int64(time.Since(ts)))
}

//ResettingTimersSnapshot是另一个ResettingTimer的时间点副本。
type ResettingTimerSnapshot struct {
	values              []int64
	mean                float64
	thresholdBoundaries []int64
	calculated          bool
}

//快照返回快照。
func (t *ResettingTimerSnapshot) Snapshot() ResettingTimer { return t }

//时间恐慌。
func (*ResettingTimerSnapshot) Time(func()) {
	panic("Time called on a ResettingTimerSnapshot")
}

//更新恐慌。
func (*ResettingTimerSnapshot) Update(time.Duration) {
	panic("Update called on a ResettingTimerSnapshot")
}

//更新自恐慌。
func (*ResettingTimerSnapshot) UpdateSince(time.Time) {
	panic("UpdateSince called on a ResettingTimerSnapshot")
}

//值返回快照中的所有值。
func (t *ResettingTimerSnapshot) Values() []int64 {
	return t.values
}

//Percentiles返回输入百分位数的边界。
func (t *ResettingTimerSnapshot) Percentiles(percentiles []float64) []int64 {
	t.calc(percentiles)

	return t.thresholdBoundaries
}

//mean返回快照值的平均值
func (t *ResettingTimerSnapshot) Mean() float64 {
	if !t.calculated {
		t.calc([]float64{})
	}

	return t.mean
}

func (t *ResettingTimerSnapshot) calc(percentiles []float64) {
	sort.Sort(Int64Slice(t.values))

	count := len(t.values)
	if count > 0 {
		min := t.values[0]
		max := t.values[count-1]

		cumulativeValues := make([]int64, count)
		cumulativeValues[0] = min
		for i := 1; i < count; i++ {
			cumulativeValues[i] = t.values[i] + cumulativeValues[i-1]
		}

		t.thresholdBoundaries = make([]int64, len(percentiles))

		thresholdBoundary := max

		for i, pct := range percentiles {
			if count > 1 {
				var abs float64
				if pct >= 0 {
					abs = pct
				} else {
					abs = 100 + pct
				}
//可怜人的数学。圆（X）：
//数学楼层（x+0.5）
				indexOfPerc := int(math.Floor(((abs / 100.0) * float64(count)) + 0.5))
				if pct >= 0 && indexOfPerc > 0 {
indexOfPerc -= 1 //索引偏移＝0
				}
				thresholdBoundary = t.values[indexOfPerc]
			}

			t.thresholdBoundaries[i] = thresholdBoundary
		}

		sum := cumulativeValues[count-1]
		t.mean = float64(sum) / float64(count)
	} else {
		t.thresholdBoundaries = make([]int64, len(percentiles))
		t.mean = 0
	}

	t.calculated = true
}

//Int64Slice将sort.interface方法附加到[]Int64，按递增顺序排序。
type Int64Slice []int64

func (s Int64Slice) Len() int           { return len(s) }
func (s Int64Slice) Less(i, j int) bool { return s[i] < s[j] }
func (s Int64Slice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
