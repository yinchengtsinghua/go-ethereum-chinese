
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package metrics

import "sync"

//GaugeFloat64保留可任意设置的float64值。
type GaugeFloat64 interface {
	Snapshot() GaugeFloat64
	Update(float64)
	Value() float64
}

//GetOrRegisterGaugeFloat64返回现有GaugeFloat64或构造并注册
//新标准计量器Float64。
func GetOrRegisterGaugeFloat64(name string, r Registry) GaugeFloat64 {
	if nil == r {
		r = DefaultRegistry
	}
	return r.GetOrRegister(name, NewGaugeFloat64()).(GaugeFloat64)
}

//NewGaugeFloat64构建了一个新的标准GaugeFloat64。
func NewGaugeFloat64() GaugeFloat64 {
	if !Enabled {
		return NilGaugeFloat64{}
	}
	return &StandardGaugeFloat64{
		value: 0.0,
	}
}

//newregisteredgaugefloat64构造并注册一个新的StandardGaugefloat64。
func NewRegisteredGaugeFloat64(name string, r Registry) GaugeFloat64 {
	c := NewGaugeFloat64()
	if nil == r {
		r = DefaultRegistry
	}
	r.Register(name, c)
	return c
}

//NewFunctionalGauge构造了一个新的FunctionalGauge。
func NewFunctionalGaugeFloat64(f func() float64) GaugeFloat64 {
	if !Enabled {
		return NilGaugeFloat64{}
	}
	return &FunctionalGaugeFloat64{value: f}
}

//NewRegisteredFunctionalGauge构造并注册新的StandardGauge。
func NewRegisteredFunctionalGaugeFloat64(name string, r Registry, f func() float64) GaugeFloat64 {
	c := NewFunctionalGaugeFloat64(f)
	if nil == r {
		r = DefaultRegistry
	}
	r.Register(name, c)
	return c
}

//GaugeFloat64快照是另一个GaugeFloat64的只读副本。
type GaugeFloat64Snapshot float64

//快照返回快照。
func (g GaugeFloat64Snapshot) Snapshot() GaugeFloat64 { return g }

//更新恐慌。
func (GaugeFloat64Snapshot) Update(float64) {
	panic("Update called on a GaugeFloat64Snapshot")
}

//值返回拍摄快照时的值。
func (g GaugeFloat64Snapshot) Value() float64 { return float64(g) }

//nilgauge是一个不可操作的量表。
type NilGaugeFloat64 struct{}

//快照是不可操作的。
func (NilGaugeFloat64) Snapshot() GaugeFloat64 { return NilGaugeFloat64{} }

//更新是不可操作的。
func (NilGaugeFloat64) Update(v float64) {}

//值是不可操作的。
func (NilGaugeFloat64) Value() float64 { return 0.0 }

//StandardGaugeFloat64是GaugeFloat64的标准实现和使用
//同步.mutex以管理单个float64值。
type StandardGaugeFloat64 struct {
	mutex sync.Mutex
	value float64
}

//快照返回仪表的只读副本。
func (g *StandardGaugeFloat64) Snapshot() GaugeFloat64 {
	return GaugeFloat64Snapshot(g.Value())
}

//更新更新更新仪表值。
func (g *StandardGaugeFloat64) Update(v float64) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.value = v
}

//值返回仪表的当前值。
func (g *StandardGaugeFloat64) Value() float64 {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return g.value
}

//FunctionalGaugeFloat64从给定函数返回值
type FunctionalGaugeFloat64 struct {
	value func() float64
}

//值返回仪表的当前值。
func (g FunctionalGaugeFloat64) Value() float64 {
	return g.value()
}

//快照返回快照。
func (g FunctionalGaugeFloat64) Snapshot() GaugeFloat64 { return GaugeFloat64Snapshot(g.Value()) }

//更新恐慌。
func (FunctionalGaugeFloat64) Update(float64) {
	panic("Update called on a FunctionalGaugeFloat64")
}
