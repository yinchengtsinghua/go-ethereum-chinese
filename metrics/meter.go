
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

//米数事件以产生指数加权移动平均速率
//以1分钟、5分钟和15分钟的平均速率。
type Meter interface {
	Count() int64
	Mark(int64)
	Rate1() float64
	Rate5() float64
	Rate15() float64
	RateMean() float64
	Snapshot() Meter
	Stop()
}

//GetOrRegisterMeter返回现有的仪表或构造并注册
//新标准仪表。
//一旦没有必要从注册表中注销仪表
//允许垃圾收集。
func GetOrRegisterMeter(name string, r Registry) Meter {
	if nil == r {
		r = DefaultRegistry
	}
	return r.GetOrRegister(name, NewMeter).(Meter)
}

//GetOrRegisterMeterForced返回现有的仪表或构造并注册
//新标准仪表，无论是否启用全局开关。
//一旦没有必要从注册表中注销仪表
//允许垃圾收集。
func GetOrRegisterMeterForced(name string, r Registry) Meter {
	if nil == r {
		r = DefaultRegistry
	}
	return r.GetOrRegister(name, NewMeterForced).(Meter)
}

//NewMeter构建了一个新的StandardMeter并启动了一个Goroutine。
//当计量器不允许垃圾收集时，一定要调用stop（）。
func NewMeter() Meter {
	if !Enabled {
		return NilMeter{}
	}
	m := newStandardMeter()
	arbiter.Lock()
	defer arbiter.Unlock()
	arbiter.meters[m] = struct{}{}
	if !arbiter.started {
		arbiter.started = true
		go arbiter.tick()
	}
	return m
}

//newmeterforced构建了一个新的StandardMeter并启动Goroutine
//全局开关是否启用。
//当计量器不允许垃圾收集时，一定要调用stop（）。
func NewMeterForced() Meter {
	m := newStandardMeter()
	arbiter.Lock()
	defer arbiter.Unlock()
	arbiter.meters[m] = struct{}{}
	if !arbiter.started {
		arbiter.started = true
		go arbiter.tick()
	}
	return m
}

//NewRegisteredMeter构造并注册一个新的StandardMeter
//启动Goroutine。
//一旦没有必要从注册表中注销仪表
//允许垃圾收集。
func NewRegisteredMeter(name string, r Registry) Meter {
	c := NewMeter()
	if nil == r {
		r = DefaultRegistry
	}
	r.Register(name, c)
	return c
}

//newregisteredmeterforced构造并注册新的标准仪表
//并启动Goroutine，无论全局开关是否启用。
//一旦没有必要从注册表中注销仪表
//允许垃圾收集。
func NewRegisteredMeterForced(name string, r Registry) Meter {
	c := NewMeterForced()
	if nil == r {
		r = DefaultRegistry
	}
	r.Register(name, c)
	return c
}

//metersnapshot是另一个meter的只读副本。
type MeterSnapshot struct {
	count                          int64
	rate1, rate5, rate15, rateMean float64
}

//Count返回拍摄快照时的事件计数。
func (m *MeterSnapshot) Count() int64 { return m.count }

//马克恐慌。
func (*MeterSnapshot) Mark(n int64) {
	panic("Mark called on a MeterSnapshot")
}

//RATE1返回在
//拍摄快照的时间。
func (m *MeterSnapshot) Rate1() float64 { return m.rate1 }

//RATE5返回每秒5分钟的移动平均事件速率
//拍摄快照的时间。
func (m *MeterSnapshot) Rate5() float64 { return m.rate5 }

//Rate15返回每秒15分钟的事件移动平均速率
//拍摄快照时。
func (m *MeterSnapshot) Rate15() float64 { return m.rate15 }

//RateMean返回在
//已拍摄快照。
func (m *MeterSnapshot) RateMean() float64 { return m.rateMean }

//快照返回快照。
func (m *MeterSnapshot) Snapshot() Meter { return m }

//停止是不允许的。
func (m *MeterSnapshot) Stop() {}

//nilmeter是一个无运算表。
type NilMeter struct{}

//计数是不允许的。
func (NilMeter) Count() int64 { return 0 }

//马克是个无赖。
func (NilMeter) Mark(n int64) {}

//RATE1是不可操作的。
func (NilMeter) Rate1() float64 { return 0.0 }

//RATE5是不允许的。
func (NilMeter) Rate5() float64 { return 0.0 }

//RATE15是不允许的。
func (NilMeter) Rate15() float64 { return 0.0 }

//RateMean是一个不允许的人。
func (NilMeter) RateMean() float64 { return 0.0 }

//快照是不可操作的。
func (NilMeter) Snapshot() Meter { return NilMeter{} }

//停止是不允许的。
func (NilMeter) Stop() {}

//标准仪表是仪表的标准实现。
type StandardMeter struct {
	lock        sync.RWMutex
	snapshot    *MeterSnapshot
	a1, a5, a15 EWMA
	startTime   time.Time
	stopped     bool
}

func newStandardMeter() *StandardMeter {
	return &StandardMeter{
		snapshot:  &MeterSnapshot{},
		a1:        NewEWMA1(),
		a5:        NewEWMA5(),
		a15:       NewEWMA15(),
		startTime: time.Now(),
	}
}

//停止停止计时表，如果您在停止后使用它，mark（）将是一个no op。
func (m *StandardMeter) Stop() {
	m.lock.Lock()
	stopped := m.stopped
	m.stopped = true
	m.lock.Unlock()
	if !stopped {
		arbiter.Lock()
		delete(arbiter.meters, m)
		arbiter.Unlock()
	}
}

//count返回记录的事件数。
func (m *StandardMeter) Count() int64 {
	m.lock.RLock()
	count := m.snapshot.count
	m.lock.RUnlock()
	return count
}

//标记记录n个事件的发生。
func (m *StandardMeter) Mark(n int64) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.stopped {
		return
	}
	m.snapshot.count += n
	m.a1.Update(n)
	m.a5.Update(n)
	m.a15.Update(n)
	m.updateSnapshot()
}

//Rate1返回每秒事件的一分钟移动平均速率。
func (m *StandardMeter) Rate1() float64 {
	m.lock.RLock()
	rate1 := m.snapshot.rate1
	m.lock.RUnlock()
	return rate1
}

//Rate5返回每秒事件的5分钟移动平均速率。
func (m *StandardMeter) Rate5() float64 {
	m.lock.RLock()
	rate5 := m.snapshot.rate5
	m.lock.RUnlock()
	return rate5
}

//Rate15返回每秒事件的15分钟移动平均速率。
func (m *StandardMeter) Rate15() float64 {
	m.lock.RLock()
	rate15 := m.snapshot.rate15
	m.lock.RUnlock()
	return rate15
}

//RateMean返回仪表每秒事件的平均速率。
func (m *StandardMeter) RateMean() float64 {
	m.lock.RLock()
	rateMean := m.snapshot.rateMean
	m.lock.RUnlock()
	return rateMean
}

//快照返回仪表的只读副本。
func (m *StandardMeter) Snapshot() Meter {
	m.lock.RLock()
	snapshot := *m.snapshot
	m.lock.RUnlock()
	return &snapshot
}

func (m *StandardMeter) updateSnapshot() {
//应在m.lock上保持写锁的情况下运行
	snapshot := m.snapshot
	snapshot.rate1 = m.a1.Rate()
	snapshot.rate5 = m.a5.Rate()
	snapshot.rate15 = m.a15.Rate()
	snapshot.rateMean = float64(snapshot.count) / time.Since(m.startTime).Seconds()
}

func (m *StandardMeter) tick() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.a1.Tick()
	m.a5.Tick()
	m.a15.Tick()
	m.updateSnapshot()
}

//计量员每5秒从一个Goroutine数米。
//仪表是一套用于将来停车的参考。
type meterArbiter struct {
	sync.RWMutex
	started bool
	meters  map[*StandardMeter]struct{}
	ticker  *time.Ticker
}

var arbiter = meterArbiter{ticker: time.NewTicker(5e9), meters: make(map[*StandardMeter]struct{})}

//按计划间隔计时米
func (ma *meterArbiter) tick() {
	for range ma.ticker.C {
		ma.tickMeters()
	}
}

func (ma *meterArbiter) tickMeters() {
	ma.RLock()
	defer ma.RUnlock()
	for meter := range ma.meters {
		meter.tick()
	}
}
