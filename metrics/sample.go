
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
	"math/rand"
	"sort"
	"sync"
	"time"
)

const rescaleThreshold = time.Hour

//样本保持统计意义上的值选择
//小溪
type Sample interface {
	Clear()
	Count() int64
	Max() int64
	Mean() float64
	Min() int64
	Percentile(float64) float64
	Percentiles([]float64) []float64
	Size() int
	Snapshot() Sample
	StdDev() float64
	Sum() int64
	Update(int64)
	Values() []int64
	Variance() float64
}

//expdecaysample是使用正向衰减的指数衰减采样
//优先水库。参见Cormode等人的“前向衰减：一个实际时间”
//流媒体系统的衰减模型”。
//
//<http://dimacs.rutgers.edu/~graham/pubs/papers/fwdecay.pdf>
type ExpDecaySample struct {
	alpha         float64
	count         int64
	mutex         sync.Mutex
	reservoirSize int
	t0, t1        time.Time
	values        *expDecaySampleHeap
}

//newexpdecaysample使用
//给定储层大小和α。
func NewExpDecaySample(reservoirSize int, alpha float64) Sample {
	if !Enabled {
		return NilSample{}
	}
	s := &ExpDecaySample{
		alpha:         alpha,
		reservoirSize: reservoirSize,
		t0:            time.Now(),
		values:        newExpDecaySampleHeap(reservoirSize),
	}
	s.t1 = s.t0.Add(rescaleThreshold)
	return s
}

//清除清除所有样本。
func (s *ExpDecaySample) Clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.count = 0
	s.t0 = time.Now()
	s.t1 = s.t0.Add(rescaleThreshold)
	s.values.Clear()
}

//count返回记录的样本数，可能超过
//油藏规模
func (s *ExpDecaySample) Count() int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.count
}

//max返回样本中的最大值，该值可能不是最大值。
//值永远是样本的一部分。
func (s *ExpDecaySample) Max() int64 {
	return SampleMax(s.Values())
}

//mean返回样本值的平均值。
func (s *ExpDecaySample) Mean() float64 {
	return SampleMean(s.Values())
}

//Min返回样本中的最小值，该值可能不是最小值
//值永远是样本的一部分。
func (s *ExpDecaySample) Min() int64 {
	return SampleMin(s.Values())
}

//Percentile返回样本中任意百分位数的值。
func (s *ExpDecaySample) Percentile(p float64) float64 {
	return SamplePercentile(s.Values(), p)
}

//Percentiles返回
//样品。
func (s *ExpDecaySample) Percentiles(ps []float64) []float64 {
	return SamplePercentiles(s.Values(), ps)
}

//SIZE返回样本的大小，最多是储层大小。
func (s *ExpDecaySample) Size() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.values.Size()
}

//快照返回示例的只读副本。
func (s *ExpDecaySample) Snapshot() Sample {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	vals := s.values.Values()
	values := make([]int64, len(vals))
	for i, v := range vals {
		values[i] = v.v
	}
	return &SampleSnapshot{
		count:  s.count,
		values: values,
	}
}

//stdev返回样本值的标准偏差。
func (s *ExpDecaySample) StdDev() float64 {
	return SampleStdDev(s.Values())
}

//sum返回样本中值的总和。
func (s *ExpDecaySample) Sum() int64 {
	return SampleSum(s.Values())
}

//更新示例新值。
func (s *ExpDecaySample) Update(v int64) {
	s.update(time.Now(), v)
}

//值返回示例中值的副本。
func (s *ExpDecaySample) Values() []int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	vals := s.values.Values()
	values := make([]int64, len(vals))
	for i, v := range vals {
		values[i] = v.v
	}
	return values
}

//方差返回样本中值的方差。
func (s *ExpDecaySample) Variance() float64 {
	return SampleVariance(s.Values())
}

//更新在特定时间戳处对新值进行采样。这是一个方法
//它本身有助于测试。
func (s *ExpDecaySample) update(t time.Time, v int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.count++
	if s.values.Size() == s.reservoirSize {
		s.values.Pop()
	}
	s.values.Push(expDecaySample{
		k: math.Exp(t.Sub(s.t0).Seconds()*s.alpha) / rand.Float64(),
		v: v,
	})
	if t.After(s.t1) {
		values := s.values.Values()
		t0 := s.t0
		s.values.Clear()
		s.t0 = t
		s.t1 = s.t0.Add(rescaleThreshold)
		for _, v := range values {
			v.k = v.k * math.Exp(-s.alpha*s.t0.Sub(t0).Seconds())
			s.values.Push(v)
		}
	}
}

//nilsample是一个no op示例。
type NilSample struct{}

//清除是不可操作的。
func (NilSample) Clear() {}

//计数是不允许的。
func (NilSample) Count() int64 { return 0 }

//马克斯不是一个OP。
func (NilSample) Max() int64 { return 0 }

//平均值是不允许的。
func (NilSample) Mean() float64 { return 0.0 }

//min是NO-OP。
func (NilSample) Min() int64 { return 0 }

//百分位数是不允许的。
func (NilSample) Percentile(p float64) float64 { return 0.0 }

//百分位数是不允许的。
func (NilSample) Percentiles(ps []float64) []float64 {
	return make([]float64, len(ps))
}

//尺寸是不允许的。
func (NilSample) Size() int { return 0 }

//样本是不可操作的。
func (NilSample) Snapshot() Sample { return NilSample{} }

//stdev是一个no-op。
func (NilSample) StdDev() float64 { return 0.0 }

//和是一个NO-op.
func (NilSample) Sum() int64 { return 0 }

//更新是不可操作的。
func (NilSample) Update(v int64) {}

//值是不可操作的。
func (NilSample) Values() []int64 { return []int64{} }

//方差是不可操作的。
func (NilSample) Variance() float64 { return 0.0 }

//sampleMax返回Int64切片的最大值。
func SampleMax(values []int64) int64 {
	if 0 == len(values) {
		return 0
	}
	var max int64 = math.MinInt64
	for _, v := range values {
		if max < v {
			max = v
		}
	}
	return max
}

//samplemean返回Int64切片的平均值。
func SampleMean(values []int64) float64 {
	if 0 == len(values) {
		return 0.0
	}
	return float64(SampleSum(values)) / float64(len(values))
}

//samplemin返回int64切片的最小值。
func SampleMin(values []int64) int64 {
	if 0 == len(values) {
		return 0
	}
	var min int64 = math.MaxInt64
	for _, v := range values {
		if min > v {
			min = v
		}
	}
	return min
}

//samplePercentles返回Int64切片的任意百分比。
func SamplePercentile(values int64Slice, p float64) float64 {
	return SamplePercentiles(values, []float64{p})[0]
}

//samplePercenties返回切片的任意百分比切片
//英特64
func SamplePercentiles(values int64Slice, ps []float64) []float64 {
	scores := make([]float64, len(ps))
	size := len(values)
	if size > 0 {
		sort.Sort(values)
		for i, p := range ps {
			pos := p * float64(size+1)
			if pos < 1.0 {
				scores[i] = float64(values[0])
			} else if pos >= float64(size) {
				scores[i] = float64(values[size-1])
			} else {
				lower := float64(values[int(pos)-1])
				upper := float64(values[int(pos)])
				scores[i] = lower + (pos-math.Floor(pos))*(upper-lower)
			}
		}
	}
	return scores
}

//samplesnapshot是另一个示例的只读副本。
type SampleSnapshot struct {
	count  int64
	values []int64
}

func NewSampleSnapshot(count int64, values []int64) *SampleSnapshot {
	return &SampleSnapshot{
		count:  count,
		values: values,
	}
}

//清晰的恐慌。
func (*SampleSnapshot) Clear() {
	panic("Clear called on a SampleSnapshot")
}

//Count返回拍摄快照时的输入计数。
func (s *SampleSnapshot) Count() int64 { return s.count }

//max返回快照拍摄时的最大值。
func (s *SampleSnapshot) Max() int64 { return SampleMax(s.values) }

//mean返回拍摄快照时的平均值。
func (s *SampleSnapshot) Mean() float64 { return SampleMean(s.values) }

//Min返回拍摄快照时的最小值。
func (s *SampleSnapshot) Min() int64 { return SampleMin(s.values) }

//percentile返回在
//已拍摄快照。
func (s *SampleSnapshot) Percentile(p float64) float64 {
	return SamplePercentile(s.values, p)
}

//Percentiles返回当时任意百分位值的切片
//快照已拍摄。
func (s *SampleSnapshot) Percentiles(ps []float64) []float64 {
	return SamplePercentiles(s.values, ps)
}

//SIZE返回拍摄快照时样本的大小。
func (s *SampleSnapshot) Size() int { return len(s.values) }

//快照返回快照。
func (s *SampleSnapshot) Snapshot() Sample { return s }

//stdev返回快照时值的标准偏差
//拿。
func (s *SampleSnapshot) StdDev() float64 { return SampleStdDev(s.values) }

//SUM返回拍摄快照时的值总和。
func (s *SampleSnapshot) Sum() int64 { return SampleSum(s.values) }

//更新恐慌。
func (*SampleSnapshot) Update(int64) {
	panic("Update called on a SampleSnapshot")
}

//值返回示例中值的副本。
func (s *SampleSnapshot) Values() []int64 {
	values := make([]int64, len(s.values))
	copy(values, s.values)
	return values
}

//variance返回拍摄快照时值的方差。
func (s *SampleSnapshot) Variance() float64 { return SampleVariance(s.values) }

//samplestdev返回int64切片的标准偏差。
func SampleStdDev(values []int64) float64 {
	return math.Sqrt(SampleVariance(values))
}

//samplesum返回Int64切片的和。
func SampleSum(values []int64) int64 {
	var sum int64
	for _, v := range values {
		sum += v
	}
	return sum
}

//samplevariance返回Int64切片的方差。
func SampleVariance(values []int64) float64 {
	if 0 == len(values) {
		return 0.0
	}
	m := SampleMean(values)
	var sum float64
	for _, v := range values {
		d := float64(v) - m
		sum += d * d
	}
	return sum / float64(len(values))
}

//使用维特算法R的均匀样本。
//
//<http://www.cs.umd.edu/~samir/498/vitter.pdf>
type UniformSample struct {
	count         int64
	mutex         sync.Mutex
	reservoirSize int
	values        []int64
}

//NewUniformSample构造一个新的具有给定储层的均匀样品
//尺寸。
func NewUniformSample(reservoirSize int) Sample {
	if !Enabled {
		return NilSample{}
	}
	return &UniformSample{
		reservoirSize: reservoirSize,
		values:        make([]int64, 0, reservoirSize),
	}
}

//清除清除所有样本。
func (s *UniformSample) Clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.count = 0
	s.values = make([]int64, 0, s.reservoirSize)
}

//count返回记录的样本数，可能超过
//油藏规模
func (s *UniformSample) Count() int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.count
}

//max返回样本中的最大值，该值可能不是最大值。
//值永远是样本的一部分。
func (s *UniformSample) Max() int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return SampleMax(s.values)
}

//mean返回样本值的平均值。
func (s *UniformSample) Mean() float64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return SampleMean(s.values)
}

//Min返回样本中的最小值，该值可能不是最小值
//值永远是样本的一部分。
func (s *UniformSample) Min() int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return SampleMin(s.values)
}

//Percentile返回样本中任意百分位数的值。
func (s *UniformSample) Percentile(p float64) float64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return SamplePercentile(s.values, p)
}

//Percentiles返回
//样品。
func (s *UniformSample) Percentiles(ps []float64) []float64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return SamplePercentiles(s.values, ps)
}

//SIZE返回样本的大小，最多是储层大小。
func (s *UniformSample) Size() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return len(s.values)
}

//快照返回示例的只读副本。
func (s *UniformSample) Snapshot() Sample {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	values := make([]int64, len(s.values))
	copy(values, s.values)
	return &SampleSnapshot{
		count:  s.count,
		values: values,
	}
}

//stdev返回样本值的标准偏差。
func (s *UniformSample) StdDev() float64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return SampleStdDev(s.values)
}

//sum返回样本中值的总和。
func (s *UniformSample) Sum() int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return SampleSum(s.values)
}

//更新示例新值。
func (s *UniformSample) Update(v int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.count++
	if len(s.values) < s.reservoirSize {
		s.values = append(s.values, v)
	} else {
		r := rand.Int63n(s.count)
		if r < int64(len(s.values)) {
			s.values[int(r)] = v
		}
	}
}

//值返回示例中值的副本。
func (s *UniformSample) Values() []int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	values := make([]int64, len(s.values))
	copy(values, s.values)
	return values
}

//方差返回样本中值的方差。
func (s *UniformSample) Variance() float64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return SampleVariance(s.values)
}

//expdecaysample表示堆中的单个样本。
type expDecaySample struct {
	k float64
	v int64
}

func newExpDecaySampleHeap(reservoirSize int) *expDecaySampleHeap {
	return &expDecaySampleHeap{make([]expDecaySample, 0, reservoirSize)}
}

//expdecaysampleheap是expdecaysamples的最小堆。
//从标准库的容器/堆复制内部实现
type expDecaySampleHeap struct {
	s []expDecaySample
}

func (h *expDecaySampleHeap) Clear() {
	h.s = h.s[:0]
}

func (h *expDecaySampleHeap) Push(s expDecaySample) {
	n := len(h.s)
	h.s = h.s[0 : n+1]
	h.s[n] = s
	h.up(n)
}

func (h *expDecaySampleHeap) Pop() expDecaySample {
	n := len(h.s) - 1
	h.s[0], h.s[n] = h.s[n], h.s[0]
	h.down(0, n)

	n = len(h.s)
	s := h.s[n-1]
	h.s = h.s[0 : n-1]
	return s
}

func (h *expDecaySampleHeap) Size() int {
	return len(h.s)
}

func (h *expDecaySampleHeap) Values() []expDecaySample {
	return h.s
}

func (h *expDecaySampleHeap) up(j int) {
	for {
i := (j - 1) / 2 //起源
		if i == j || !(h.s[j].k < h.s[i].k) {
			break
		}
		h.s[i], h.s[j] = h.s[j], h.s[i]
		j = i
	}
}

func (h *expDecaySampleHeap) down(i, n int) {
	for {
		j1 := 2*i + 1
if j1 >= n || j1 < 0 { //int溢出后j1<0
			break
		}
j := j1 //留守儿童
		if j2 := j1 + 1; j2 < n && !(h.s[j1].k < h.s[j2].k) {
j = j2 //=2*i+2//右儿童
		}
		if !(h.s[j].k < h.s[i].k) {
			break
		}
		h.s[i], h.s[j] = h.s[j], h.s[i]
		i = j
	}
}

type int64Slice []int64

func (p int64Slice) Len() int           { return len(p) }
func (p int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
