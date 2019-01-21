
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package metrics

import "sync/atomic"

//仪表保持一个可以任意设置的Int64值。
type Gauge interface {
	Snapshot() Gauge
	Update(int64)
	Value() int64
}

//GetOrRegisterGauge返回现有仪表或构造并注册
//新标准仪表。
func GetOrRegisterGauge(name string, r Registry) Gauge {
	if nil == r {
		r = DefaultRegistry
	}
	return r.GetOrRegister(name, NewGauge).(Gauge)
}

//NewGauge构造了一个新的StandardGauge。
func NewGauge() Gauge {
	if !Enabled {
		return NilGauge{}
	}
	return &StandardGauge{0}
}

//newregisteredgauge构造并注册新的标准仪表。
func NewRegisteredGauge(name string, r Registry) Gauge {
	c := NewGauge()
	if nil == r {
		r = DefaultRegistry
	}
	r.Register(name, c)
	return c
}

//NewFunctionalGauge构造了一个新的FunctionalGauge。
func NewFunctionalGauge(f func() int64) Gauge {
	if !Enabled {
		return NilGauge{}
	}
	return &FunctionalGauge{value: f}
}

//NewRegisteredFunctionalGauge构造并注册新的StandardGauge。
func NewRegisteredFunctionalGauge(name string, r Registry, f func() int64) Gauge {
	c := NewFunctionalGauge(f)
	if nil == r {
		r = DefaultRegistry
	}
	r.Register(name, c)
	return c
}

//GaugeSnapshot是另一个仪表的只读副本。
type GaugeSnapshot int64

//快照返回快照。
func (g GaugeSnapshot) Snapshot() Gauge { return g }

//更新恐慌。
func (GaugeSnapshot) Update(int64) {
	panic("Update called on a GaugeSnapshot")
}

//值返回拍摄快照时的值。
func (g GaugeSnapshot) Value() int64 { return int64(g) }

//nilgauge是一个不可操作的量表。
type NilGauge struct{}

//快照是不可操作的。
func (NilGauge) Snapshot() Gauge { return NilGauge{} }

//更新是不可操作的。
func (NilGauge) Update(v int64) {}

//值是不可操作的。
func (NilGauge) Value() int64 { return 0 }

//标准仪表是仪表的标准实现，使用
//同步/atomic包以管理单个int64值。
type StandardGauge struct {
	value int64
}

//快照返回仪表的只读副本。
func (g *StandardGauge) Snapshot() Gauge {
	return GaugeSnapshot(g.Value())
}

//更新更新更新仪表值。
func (g *StandardGauge) Update(v int64) {
	atomic.StoreInt64(&g.value, v)
}

//值返回仪表的当前值。
func (g *StandardGauge) Value() int64 {
	return atomic.LoadInt64(&g.value)
}

//函数仪表从给定函数返回值
type FunctionalGauge struct {
	value func() int64
}

//值返回仪表的当前值。
func (g FunctionalGauge) Value() int64 {
	return g.value()
}

//快照返回快照。
func (g FunctionalGauge) Snapshot() Gauge { return GaugeSnapshot(g.Value()) }

//更新恐慌。
func (FunctionalGauge) Update(int64) {
	panic("Update called on a FunctionalGauge")
}
