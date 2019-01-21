
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package metrics

//柱状图根据一系列Int64值计算分布统计信息。
type Histogram interface {
	Clear()
	Count() int64
	Max() int64
	Mean() float64
	Min() int64
	Percentile(float64) float64
	Percentiles([]float64) []float64
	Sample() Sample
	Snapshot() Histogram
	StdDev() float64
	Sum() int64
	Update(int64)
	Variance() float64
}

//GetOrRegisterHistogram返回现有的柱状图或构造，以及
//注册新的标准柱状图。
func GetOrRegisterHistogram(name string, r Registry, s Sample) Histogram {
	if nil == r {
		r = DefaultRegistry
	}
	return r.GetOrRegister(name, func() Histogram { return NewHistogram(s) }).(Histogram)
}

//NewHistogram从一个样本构造一个新的标准柱状图。
func NewHistogram(s Sample) Histogram {
	if !Enabled {
		return NilHistogram{}
	}
	return &StandardHistogram{sample: s}
}

//NewRegisteredHistogram构造并注册来自
//样品。
func NewRegisteredHistogram(name string, r Registry, s Sample) Histogram {
	c := NewHistogram(s)
	if nil == r {
		r = DefaultRegistry
	}
	r.Register(name, c)
	return c
}

//HistogramSnapshot是另一个柱状图的只读副本。
type HistogramSnapshot struct {
	sample *SampleSnapshot
}

//清晰的恐慌。
func (*HistogramSnapshot) Clear() {
	panic("Clear called on a HistogramSnapshot")
}

//count返回快照时记录的样本数
//拿。
func (h *HistogramSnapshot) Count() int64 { return h.sample.Count() }

//max返回快照时样本中的最大值。
//拿。
func (h *HistogramSnapshot) Max() int64 { return h.sample.Max() }

//mean返回快照时样本中值的平均值
//被带走了。
func (h *HistogramSnapshot) Mean() float64 { return h.sample.Mean() }

//Min返回快照时样本中的最小值
//拿。
func (h *HistogramSnapshot) Min() int64 { return h.sample.Min() }

//Percentile返回在
//拍摄快照的时间。
func (h *HistogramSnapshot) Percentile(p float64) float64 {
	return h.sample.Percentile(p)
}

//Percentiles返回样本中任意百分位值的切片
//拍摄快照时。
func (h *HistogramSnapshot) Percentiles(ps []float64) []float64 {
	return h.sample.Percentiles(ps)
}

//sample返回柱状图下的样本。
func (h *HistogramSnapshot) Sample() Sample { return h.sample }

//快照返回快照。
func (h *HistogramSnapshot) Snapshot() Histogram { return h }

//stdev返回在
//拍摄快照的时间。
func (h *HistogramSnapshot) StdDev() float64 { return h.sample.StdDev() }

//sum返回快照拍摄时样本中的总和。
func (h *HistogramSnapshot) Sum() int64 { return h.sample.Sum() }

//更新恐慌。
func (*HistogramSnapshot) Update(int64) {
	panic("Update called on a HistogramSnapshot")
}

//variance返回拍摄快照时输入的方差。
func (h *HistogramSnapshot) Variance() float64 { return h.sample.Variance() }

//nilHistogram是一个无操作的柱状图。
type NilHistogram struct{}

//清除是不可操作的。
func (NilHistogram) Clear() {}

//计数是不允许的。
func (NilHistogram) Count() int64 { return 0 }

//马克斯不是一个OP。
func (NilHistogram) Max() int64 { return 0 }

//平均值是不允许的。
func (NilHistogram) Mean() float64 { return 0.0 }

//min是NO-OP。
func (NilHistogram) Min() int64 { return 0 }

//百分位数是不允许的。
func (NilHistogram) Percentile(p float64) float64 { return 0.0 }

//百分位数是不允许的。
func (NilHistogram) Percentiles(ps []float64) []float64 {
	return make([]float64, len(ps))
}

//样本是不可操作的。
func (NilHistogram) Sample() Sample { return NilSample{} }

//快照是不可操作的。
func (NilHistogram) Snapshot() Histogram { return NilHistogram{} }

//stdev是一个no-op。
func (NilHistogram) StdDev() float64 { return 0.0 }

//和是一个NO-op.
func (NilHistogram) Sum() int64 { return 0 }

//更新是不可操作的。
func (NilHistogram) Update(v int64) {}

//方差是不可操作的。
func (NilHistogram) Variance() float64 { return 0.0 }

//标准柱状图是柱状图的标准实现，使用
//用于绑定其内存使用的示例。
type StandardHistogram struct {
	sample Sample
}

//清除清除柱状图及其样本。
func (h *StandardHistogram) Clear() { h.sample.Clear() }

//count返回自上次直方图以来记录的样本数
//变明朗。
func (h *StandardHistogram) Count() int64 { return h.sample.Count() }

//max返回样本中的最大值。
func (h *StandardHistogram) Max() int64 { return h.sample.Max() }

//mean返回样本值的平均值。
func (h *StandardHistogram) Mean() float64 { return h.sample.Mean() }

//Min返回样本中的最小值。
func (h *StandardHistogram) Min() int64 { return h.sample.Min() }

//Percentile返回样本中任意百分位数的值。
func (h *StandardHistogram) Percentile(p float64) float64 {
	return h.sample.Percentile(p)
}

//Percentiles返回
//样品。
func (h *StandardHistogram) Percentiles(ps []float64) []float64 {
	return h.sample.Percentiles(ps)
}

//sample返回柱状图下的样本。
func (h *StandardHistogram) Sample() Sample { return h.sample }

//快照返回柱状图的只读副本。
func (h *StandardHistogram) Snapshot() Histogram {
	return &HistogramSnapshot{sample: h.sample.Snapshot().(*SampleSnapshot)}
}

//stdev返回样本值的标准偏差。
func (h *StandardHistogram) StdDev() float64 { return h.sample.StdDev() }

//sum返回样本中的和。
func (h *StandardHistogram) Sum() int64 { return h.sample.Sum() }

//更新示例新值。
func (h *StandardHistogram) Update(v int64) { h.sample.Update(v) }

//方差返回样本中值的方差。
func (h *StandardHistogram) Variance() float64 { return h.sample.Variance() }
