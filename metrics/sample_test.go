
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package metrics

import (
	"math/rand"
	"runtime"
	"testing"
	"time"
)

//基准计算，复制1000000证明，即使相对而言
//昂贵的计算，如方差，复制样本的成本，如
//近似于一份复制品，比
//对小样本的计算，而对大样本的计算则稍微少一些。
func BenchmarkCompute1000(b *testing.B) {
	s := make([]int64, 1000)
	for i := 0; i < len(s); i++ {
		s[i] = int64(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SampleVariance(s)
	}
}
func BenchmarkCompute1000000(b *testing.B) {
	s := make([]int64, 1000000)
	for i := 0; i < len(s); i++ {
		s[i] = int64(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SampleVariance(s)
	}
}
func BenchmarkCopy1000(b *testing.B) {
	s := make([]int64, 1000)
	for i := 0; i < len(s); i++ {
		s[i] = int64(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sCopy := make([]int64, len(s))
		copy(sCopy, s)
	}
}
func BenchmarkCopy1000000(b *testing.B) {
	s := make([]int64, 1000000)
	for i := 0; i < len(s); i++ {
		s[i] = int64(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sCopy := make([]int64, len(s))
		copy(sCopy, s)
	}
}

func BenchmarkExpDecaySample257(b *testing.B) {
	benchmarkSample(b, NewExpDecaySample(257, 0.015))
}

func BenchmarkExpDecaySample514(b *testing.B) {
	benchmarkSample(b, NewExpDecaySample(514, 0.015))
}

func BenchmarkExpDecaySample1028(b *testing.B) {
	benchmarkSample(b, NewExpDecaySample(1028, 0.015))
}

func BenchmarkUniformSample257(b *testing.B) {
	benchmarkSample(b, NewUniformSample(257))
}

func BenchmarkUniformSample514(b *testing.B) {
	benchmarkSample(b, NewUniformSample(514))
}

func BenchmarkUniformSample1028(b *testing.B) {
	benchmarkSample(b, NewUniformSample(1028))
}

func TestExpDecaySample10(t *testing.T) {
	rand.Seed(1)
	s := NewExpDecaySample(100, 0.99)
	for i := 0; i < 10; i++ {
		s.Update(int64(i))
	}
	if size := s.Count(); 10 != size {
		t.Errorf("s.Count(): 10 != %v\n", size)
	}
	if size := s.Size(); 10 != size {
		t.Errorf("s.Size(): 10 != %v\n", size)
	}
	if l := len(s.Values()); 10 != l {
		t.Errorf("len(s.Values()): 10 != %v\n", l)
	}
	for _, v := range s.Values() {
		if v > 10 || v < 0 {
			t.Errorf("out of range [0, 10): %v\n", v)
		}
	}
}

func TestExpDecaySample100(t *testing.T) {
	rand.Seed(1)
	s := NewExpDecaySample(1000, 0.01)
	for i := 0; i < 100; i++ {
		s.Update(int64(i))
	}
	if size := s.Count(); 100 != size {
		t.Errorf("s.Count(): 100 != %v\n", size)
	}
	if size := s.Size(); 100 != size {
		t.Errorf("s.Size(): 100 != %v\n", size)
	}
	if l := len(s.Values()); 100 != l {
		t.Errorf("len(s.Values()): 100 != %v\n", l)
	}
	for _, v := range s.Values() {
		if v > 100 || v < 0 {
			t.Errorf("out of range [0, 100): %v\n", v)
		}
	}
}

func TestExpDecaySample1000(t *testing.T) {
	rand.Seed(1)
	s := NewExpDecaySample(100, 0.99)
	for i := 0; i < 1000; i++ {
		s.Update(int64(i))
	}
	if size := s.Count(); 1000 != size {
		t.Errorf("s.Count(): 1000 != %v\n", size)
	}
	if size := s.Size(); 100 != size {
		t.Errorf("s.Size(): 100 != %v\n", size)
	}
	if l := len(s.Values()); 100 != l {
		t.Errorf("len(s.Values()): 100 != %v\n", l)
	}
	for _, v := range s.Values() {
		if v > 1000 || v < 0 {
			t.Errorf("out of range [0, 1000): %v\n", v)
		}
	}
}

//该测试确保样品的优先级不会因使用
//启动后的纳秒持续时间，而不是启动后的第二个持续时间。
//启动后，优先级迅速变为+inf，如果这样做，
//有效地冻结样本集，直到发生重新缩放步骤。
func TestExpDecaySampleNanosecondRegression(t *testing.T) {
	rand.Seed(1)
	s := NewExpDecaySample(100, 0.99)
	for i := 0; i < 100; i++ {
		s.Update(10)
	}
	time.Sleep(1 * time.Millisecond)
	for i := 0; i < 100; i++ {
		s.Update(20)
	}
	v := s.Values()
	avg := float64(0)
	for i := 0; i < len(v); i++ {
		avg += float64(v[i])
	}
	avg /= float64(len(v))
	if avg > 16 || avg < 14 {
		t.Errorf("out of range [14, 16]: %v\n", avg)
	}
}

func TestExpDecaySampleRescale(t *testing.T) {
	s := NewExpDecaySample(2, 0.001).(*ExpDecaySample)
	s.update(time.Now(), 1)
	s.update(time.Now().Add(time.Hour+time.Microsecond), 1)
	for _, v := range s.values.Values() {
		if v.k == 0.0 {
			t.Fatal("v.k == 0.0")
		}
	}
}

func TestExpDecaySampleSnapshot(t *testing.T) {
	now := time.Now()
	rand.Seed(1)
	s := NewExpDecaySample(100, 0.99)
	for i := 1; i <= 10000; i++ {
		s.(*ExpDecaySample).update(now.Add(time.Duration(i)), int64(i))
	}
	snapshot := s.Snapshot()
	s.Update(1)
	testExpDecaySampleStatistics(t, snapshot)
}

func TestExpDecaySampleStatistics(t *testing.T) {
	now := time.Now()
	rand.Seed(1)
	s := NewExpDecaySample(100, 0.99)
	for i := 1; i <= 10000; i++ {
		s.(*ExpDecaySample).update(now.Add(time.Duration(i)), int64(i))
	}
	testExpDecaySampleStatistics(t, s)
}

func TestUniformSample(t *testing.T) {
	rand.Seed(1)
	s := NewUniformSample(100)
	for i := 0; i < 1000; i++ {
		s.Update(int64(i))
	}
	if size := s.Count(); 1000 != size {
		t.Errorf("s.Count(): 1000 != %v\n", size)
	}
	if size := s.Size(); 100 != size {
		t.Errorf("s.Size(): 100 != %v\n", size)
	}
	if l := len(s.Values()); 100 != l {
		t.Errorf("len(s.Values()): 100 != %v\n", l)
	}
	for _, v := range s.Values() {
		if v > 1000 || v < 0 {
			t.Errorf("out of range [0, 100): %v\n", v)
		}
	}
}

func TestUniformSampleIncludesTail(t *testing.T) {
	rand.Seed(1)
	s := NewUniformSample(100)
	max := 100
	for i := 0; i < max; i++ {
		s.Update(int64(i))
	}
	v := s.Values()
	sum := 0
	exp := (max - 1) * max / 2
	for i := 0; i < len(v); i++ {
		sum += int(v[i])
	}
	if exp != sum {
		t.Errorf("sum: %v != %v\n", exp, sum)
	}
}

func TestUniformSampleSnapshot(t *testing.T) {
	s := NewUniformSample(100)
	for i := 1; i <= 10000; i++ {
		s.Update(int64(i))
	}
	snapshot := s.Snapshot()
	s.Update(1)
	testUniformSampleStatistics(t, snapshot)
}

func TestUniformSampleStatistics(t *testing.T) {
	rand.Seed(1)
	s := NewUniformSample(100)
	for i := 1; i <= 10000; i++ {
		s.Update(int64(i))
	}
	testUniformSampleStatistics(t, s)
}

func benchmarkSample(b *testing.B, s Sample) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	pauseTotalNs := memStats.PauseTotalNs
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Update(1)
	}
	b.StopTimer()
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	b.Logf("GC cost: %d ns/op", int(memStats.PauseTotalNs-pauseTotalNs)/b.N)
}

func testExpDecaySampleStatistics(t *testing.T, s Sample) {
	if count := s.Count(); 10000 != count {
		t.Errorf("s.Count(): 10000 != %v\n", count)
	}
	if min := s.Min(); 107 != min {
		t.Errorf("s.Min(): 107 != %v\n", min)
	}
	if max := s.Max(); 10000 != max {
		t.Errorf("s.Max(): 10000 != %v\n", max)
	}
	if mean := s.Mean(); 4965.98 != mean {
		t.Errorf("s.Mean(): 4965.98 != %v\n", mean)
	}
	if stdDev := s.StdDev(); 2959.825156930727 != stdDev {
		t.Errorf("s.StdDev(): 2959.825156930727 != %v\n", stdDev)
	}
	ps := s.Percentiles([]float64{0.5, 0.75, 0.99})
	if 4615 != ps[0] {
		t.Errorf("median: 4615 != %v\n", ps[0])
	}
	if 7672 != ps[1] {
		t.Errorf("75th percentile: 7672 != %v\n", ps[1])
	}
	if 9998.99 != ps[2] {
		t.Errorf("99th percentile: 9998.99 != %v\n", ps[2])
	}
}

func testUniformSampleStatistics(t *testing.T, s Sample) {
	if count := s.Count(); 10000 != count {
		t.Errorf("s.Count(): 10000 != %v\n", count)
	}
	if min := s.Min(); 37 != min {
		t.Errorf("s.Min(): 37 != %v\n", min)
	}
	if max := s.Max(); 9989 != max {
		t.Errorf("s.Max(): 9989 != %v\n", max)
	}
	if mean := s.Mean(); 4748.14 != mean {
		t.Errorf("s.Mean(): 4748.14 != %v\n", mean)
	}
	if stdDev := s.StdDev(); 2826.684117548333 != stdDev {
		t.Errorf("s.StdDev(): 2826.684117548333 != %v\n", stdDev)
	}
	ps := s.Percentiles([]float64{0.5, 0.75, 0.99})
	if 4599 != ps[0] {
		t.Errorf("median: 4599 != %v\n", ps[0])
	}
	if 7380.5 != ps[1] {
		t.Errorf("75th percentile: 7380.5 != %v\n", ps[1])
	}
	if 9986.429999999998 != ps[2] {
		t.Errorf("99th percentile: 9986.429999999998 != %v\n", ps[2])
	}
}

//testUniformSampleConcurrentUpdateCount将暴露数据争用问题
//当用-race调用测试时，对sample的并发更新和计数调用
//论点
func TestUniformSampleConcurrentUpdateCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	s := NewUniformSample(100)
	for i := 0; i < 100; i++ {
		s.Update(int64(i))
	}
	quit := make(chan struct{})
	go func() {
		t := time.NewTicker(10 * time.Millisecond)
		for {
			select {
			case <-t.C:
				s.Update(rand.Int63())
			case <-quit:
				t.Stop()
				return
			}
		}
	}()
	for i := 0; i < 1000; i++ {
		s.Count()
		time.Sleep(5 * time.Millisecond)
	}
	quit <- struct{}{}
}
