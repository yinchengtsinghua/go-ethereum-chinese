
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//将Go度量挂钩到expvar中
//在任何/debug/metrics请求上，将注册表中的所有var加载到expvar，并执行常规expvar处理程序
package exp

import (
	"expvar"
	"fmt"
	"net/http"
	"sync"

	"github.com/ethereum/go-ethereum/metrics"
)

type exp struct {
expvarLock sync.Mutex //如果您尝试注册同一个var两次，expvar将崩溃，因此我们必须安全地探测它。
	registry   metrics.Registry
}

func (exp *exp) expHandler(w http.ResponseWriter, r *http.Request) {
//将变量加载到expvar中
	exp.syncToExpvar()

//现在只需运行官方的expvar处理程序代码（它不是可公开调用的，所以是内联粘贴的）
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, "{\n")
	first := true
	expvar.Do(func(kv expvar.KeyValue) {
		if !first {
			fmt.Fprintf(w, ",\n")
		}
		first = false
		fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
	})
	fmt.Fprintf(w, "\n}\n")
}

//exp将使用http.defaultservemux在“/debug/vars”上注册一个expvar支持的度量处理程序。
func Exp(r metrics.Registry) {
	h := ExpHandler(r)
//这会引起恐慌：
//死机：http:/debug/vars的多个注册
//http.handlefunc（“/debug/vars”，e.exphandler）
//还没有找到一种优雅的方式，所以只需使用一个不同的端点
	http.Handle("/debug/metrics", h)
}

//exphandler将返回一个expvar支持的度量处理程序。
func ExpHandler(r metrics.Registry) http.Handler {
	e := exp{sync.Mutex{}, r}
	return http.HandlerFunc(e.expHandler)
}

func (exp *exp) getInt(name string) *expvar.Int {
	var v *expvar.Int
	exp.expvarLock.Lock()
	p := expvar.Get(name)
	if p != nil {
		v = p.(*expvar.Int)
	} else {
		v = new(expvar.Int)
		expvar.Publish(name, v)
	}
	exp.expvarLock.Unlock()
	return v
}

func (exp *exp) getFloat(name string) *expvar.Float {
	var v *expvar.Float
	exp.expvarLock.Lock()
	p := expvar.Get(name)
	if p != nil {
		v = p.(*expvar.Float)
	} else {
		v = new(expvar.Float)
		expvar.Publish(name, v)
	}
	exp.expvarLock.Unlock()
	return v
}

func (exp *exp) publishCounter(name string, metric metrics.Counter) {
	v := exp.getInt(name)
	v.Set(metric.Count())
}

func (exp *exp) publishGauge(name string, metric metrics.Gauge) {
	v := exp.getInt(name)
	v.Set(metric.Value())
}
func (exp *exp) publishGaugeFloat64(name string, metric metrics.GaugeFloat64) {
	exp.getFloat(name).Set(metric.Value())
}

func (exp *exp) publishHistogram(name string, metric metrics.Histogram) {
	h := metric.Snapshot()
	ps := h.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999})
	exp.getInt(name + ".count").Set(h.Count())
	exp.getFloat(name + ".min").Set(float64(h.Min()))
	exp.getFloat(name + ".max").Set(float64(h.Max()))
	exp.getFloat(name + ".mean").Set(h.Mean())
	exp.getFloat(name + ".std-dev").Set(h.StdDev())
	exp.getFloat(name + ".50-percentile").Set(ps[0])
	exp.getFloat(name + ".75-percentile").Set(ps[1])
	exp.getFloat(name + ".95-percentile").Set(ps[2])
	exp.getFloat(name + ".99-percentile").Set(ps[3])
	exp.getFloat(name + ".999-percentile").Set(ps[4])
}

func (exp *exp) publishMeter(name string, metric metrics.Meter) {
	m := metric.Snapshot()
	exp.getInt(name + ".count").Set(m.Count())
	exp.getFloat(name + ".one-minute").Set(m.Rate1())
	exp.getFloat(name + ".five-minute").Set(m.Rate5())
	exp.getFloat(name + ".fifteen-minute").Set((m.Rate15()))
	exp.getFloat(name + ".mean").Set(m.RateMean())
}

func (exp *exp) publishTimer(name string, metric metrics.Timer) {
	t := metric.Snapshot()
	ps := t.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999})
	exp.getInt(name + ".count").Set(t.Count())
	exp.getFloat(name + ".min").Set(float64(t.Min()))
	exp.getFloat(name + ".max").Set(float64(t.Max()))
	exp.getFloat(name + ".mean").Set(t.Mean())
	exp.getFloat(name + ".std-dev").Set(t.StdDev())
	exp.getFloat(name + ".50-percentile").Set(ps[0])
	exp.getFloat(name + ".75-percentile").Set(ps[1])
	exp.getFloat(name + ".95-percentile").Set(ps[2])
	exp.getFloat(name + ".99-percentile").Set(ps[3])
	exp.getFloat(name + ".999-percentile").Set(ps[4])
	exp.getFloat(name + ".one-minute").Set(t.Rate1())
	exp.getFloat(name + ".five-minute").Set(t.Rate5())
	exp.getFloat(name + ".fifteen-minute").Set(t.Rate15())
	exp.getFloat(name + ".mean-rate").Set(t.RateMean())
}

func (exp *exp) publishResettingTimer(name string, metric metrics.ResettingTimer) {
	t := metric.Snapshot()
	ps := t.Percentiles([]float64{50, 75, 95, 99})
	exp.getInt(name + ".count").Set(int64(len(t.Values())))
	exp.getFloat(name + ".mean").Set(t.Mean())
	exp.getInt(name + ".50-percentile").Set(ps[0])
	exp.getInt(name + ".75-percentile").Set(ps[1])
	exp.getInt(name + ".95-percentile").Set(ps[2])
	exp.getInt(name + ".99-percentile").Set(ps[3])
}

func (exp *exp) syncToExpvar() {
	exp.registry.Each(func(name string, i interface{}) {
		switch i := i.(type) {
		case metrics.Counter:
			exp.publishCounter(name, i)
		case metrics.Gauge:
			exp.publishGauge(name, i)
		case metrics.GaugeFloat64:
			exp.publishGaugeFloat64(name, i)
		case metrics.Histogram:
			exp.publishHistogram(name, i)
		case metrics.Meter:
			exp.publishMeter(name, i)
		case metrics.Timer:
			exp.publishTimer(name, i)
		case metrics.ResettingTimer:
			exp.publishResettingTimer(name, i)
		default:
			panic(fmt.Sprintf("unsupported type for '%s': %T", name, i))
		}
	})
}
