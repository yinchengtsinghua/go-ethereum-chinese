
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package metrics

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

//Duplicatemetric是注册表返回的错误。当度量
//已经存在。如果要注册该度量，必须首先
//注销现有度量。
type DuplicateMetric string

func (err DuplicateMetric) Error() string {
	return fmt.Sprintf("duplicate metric: %s", string(err))
}

//注册表按名称保存对一组度量的引用，并且可以迭代
//通过它们，调用用户提供的回调函数。
//
//这是一个接口，以鼓励其他结构实现
//注册表API（视情况而定）。
type Registry interface {

//为每个注册的度量调用给定的函数。
	Each(func(string, interface{}))

//按给定的名称获取度量，如果未注册，则为零。
	Get(string) interface{}

//获取注册表中的所有指标。
	GetAll() map[string]map[string]interface{}

//获取现有度量或注册给定的度量。
//如果在注册表中找不到接口，则该接口可以是要注册的度量。
//或者返回延迟实例化度量的函数。
	GetOrRegister(string, interface{}) interface{}

//在给定的名称下注册给定的度量。
	Register(string, interface{}) error

//运行所有已注册的运行状况检查。
	RunHealthchecks()

//用给定的名称注销度量。
	Unregister(string)

//注销所有度量。（主要用于测试。）
	UnregisterAll()
}

//注册表的标准实现是一个互斥保护映射
//从名称到度量。
type StandardRegistry struct {
	metrics map[string]interface{}
	mutex   sync.Mutex
}

//创建新注册表。
func NewRegistry() Registry {
	return &StandardRegistry{metrics: make(map[string]interface{})}
}

//为每个注册的度量调用给定的函数。
func (r *StandardRegistry) Each(f func(string, interface{})) {
	for name, i := range r.registered() {
		f(name, i)
	}
}

//按给定的名称获取度量，如果未注册，则为零。
func (r *StandardRegistry) Get(name string) interface{} {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.metrics[name]
}

//获取现有度量或创建并注册新度量。线程安全的
//在失败时调用get和register的替代方法。
//如果在注册表中找不到接口，则该接口可以是要注册的度量。
//或者返回延迟实例化度量的函数。
func (r *StandardRegistry) GetOrRegister(name string, i interface{}) interface{} {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if metric, ok := r.metrics[name]; ok {
		return metric
	}
	if v := reflect.ValueOf(i); v.Kind() == reflect.Func {
		i = v.Call(nil)[0].Interface()
	}
	r.register(name, i)
	return i
}

//在给定的名称下注册给定的度量。返回重复度量值
//如果已注册给定名称的度量值。
func (r *StandardRegistry) Register(name string, i interface{}) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.register(name, i)
}

//运行所有已注册的运行状况检查。
func (r *StandardRegistry) RunHealthchecks() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, i := range r.metrics {
		if h, ok := i.(Healthcheck); ok {
			h.Check()
		}
	}
}

//获取注册表中的所有指标
func (r *StandardRegistry) GetAll() map[string]map[string]interface{} {
	data := make(map[string]map[string]interface{})
	r.Each(func(name string, i interface{}) {
		values := make(map[string]interface{})
		switch metric := i.(type) {
		case Counter:
			values["count"] = metric.Count()
		case Gauge:
			values["value"] = metric.Value()
		case GaugeFloat64:
			values["value"] = metric.Value()
		case Healthcheck:
			values["error"] = nil
			metric.Check()
			if err := metric.Error(); nil != err {
				values["error"] = metric.Error().Error()
			}
		case Histogram:
			h := metric.Snapshot()
			ps := h.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999})
			values["count"] = h.Count()
			values["min"] = h.Min()
			values["max"] = h.Max()
			values["mean"] = h.Mean()
			values["stddev"] = h.StdDev()
			values["median"] = ps[0]
			values["75%"] = ps[1]
			values["95%"] = ps[2]
			values["99%"] = ps[3]
			values["99.9%"] = ps[4]
		case Meter:
			m := metric.Snapshot()
			values["count"] = m.Count()
			values["1m.rate"] = m.Rate1()
			values["5m.rate"] = m.Rate5()
			values["15m.rate"] = m.Rate15()
			values["mean.rate"] = m.RateMean()
		case Timer:
			t := metric.Snapshot()
			ps := t.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999})
			values["count"] = t.Count()
			values["min"] = t.Min()
			values["max"] = t.Max()
			values["mean"] = t.Mean()
			values["stddev"] = t.StdDev()
			values["median"] = ps[0]
			values["75%"] = ps[1]
			values["95%"] = ps[2]
			values["99%"] = ps[3]
			values["99.9%"] = ps[4]
			values["1m.rate"] = t.Rate1()
			values["5m.rate"] = t.Rate5()
			values["15m.rate"] = t.Rate15()
			values["mean.rate"] = t.RateMean()
		}
		data[name] = values
	})
	return data
}

//用给定的名称注销度量。
func (r *StandardRegistry) Unregister(name string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.stop(name)
	delete(r.metrics, name)
}

//注销所有度量。（主要用于测试。）
func (r *StandardRegistry) UnregisterAll() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for name := range r.metrics {
		r.stop(name)
		delete(r.metrics, name)
	}
}

func (r *StandardRegistry) register(name string, i interface{}) error {
	if _, ok := r.metrics[name]; ok {
		return DuplicateMetric(name)
	}
	switch i.(type) {
	case Counter, Gauge, GaugeFloat64, Healthcheck, Histogram, Meter, Timer, ResettingTimer:
		r.metrics[name] = i
	}
	return nil
}

func (r *StandardRegistry) registered() map[string]interface{} {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	metrics := make(map[string]interface{}, len(r.metrics))
	for name, i := range r.metrics {
		metrics[name] = i
	}
	return metrics
}

func (r *StandardRegistry) stop(name string) {
	if i, ok := r.metrics[name]; ok {
		if s, ok := i.(Stoppable); ok {
			s.Stop()
		}
	}
}

//stoppable定义必须停止的度量。
type Stoppable interface {
	Stop()
}

type PrefixedRegistry struct {
	underlying Registry
	prefix     string
}

func NewPrefixedRegistry(prefix string) Registry {
	return &PrefixedRegistry{
		underlying: NewRegistry(),
		prefix:     prefix,
	}
}

func NewPrefixedChildRegistry(parent Registry, prefix string) Registry {
	return &PrefixedRegistry{
		underlying: parent,
		prefix:     prefix,
	}
}

//为每个注册的度量调用给定的函数。
func (r *PrefixedRegistry) Each(fn func(string, interface{})) {
	wrappedFn := func(prefix string) func(string, interface{}) {
		return func(name string, iface interface{}) {
			if strings.HasPrefix(name, prefix) {
				fn(name, iface)
			} else {
				return
			}
		}
	}

	baseRegistry, prefix := findPrefix(r, "")
	baseRegistry.Each(wrappedFn(prefix))
}

func findPrefix(registry Registry, prefix string) (Registry, string) {
	switch r := registry.(type) {
	case *PrefixedRegistry:
		return findPrefix(r.underlying, r.prefix+prefix)
	case *StandardRegistry:
		return r, prefix
	}
	return nil, ""
}

//按给定的名称获取度量，如果未注册，则为零。
func (r *PrefixedRegistry) Get(name string) interface{} {
	realName := r.prefix + name
	return r.underlying.Get(realName)
}

//获取现有度量或注册给定的度量。
//如果在注册表中找不到接口，则该接口可以是要注册的度量。
//或者返回延迟实例化度量的函数。
func (r *PrefixedRegistry) GetOrRegister(name string, metric interface{}) interface{} {
	realName := r.prefix + name
	return r.underlying.GetOrRegister(realName, metric)
}

//在给定的名称下注册给定的度量。名称将以前缀形式出现。
func (r *PrefixedRegistry) Register(name string, metric interface{}) error {
	realName := r.prefix + name
	return r.underlying.Register(realName, metric)
}

//运行所有已注册的运行状况检查。
func (r *PrefixedRegistry) RunHealthchecks() {
	r.underlying.RunHealthchecks()
}

//获取注册表中的所有指标
func (r *PrefixedRegistry) GetAll() map[string]map[string]interface{} {
	return r.underlying.GetAll()
}

//用给定的名称注销度量。名称将以前缀形式出现。
func (r *PrefixedRegistry) Unregister(name string) {
	realName := r.prefix + name
	r.underlying.Unregister(realName)
}

//注销所有度量。（主要用于测试。）
func (r *PrefixedRegistry) UnregisterAll() {
	r.underlying.UnregisterAll()
}

var (
	DefaultRegistry   = NewRegistry()
	EphemeralRegistry = NewRegistry()
)

//为每个注册的度量调用给定的函数。
func Each(f func(string, interface{})) {
	DefaultRegistry.Each(f)
}

//按给定的名称获取度量，如果未注册，则为零。
func Get(name string) interface{} {
	return DefaultRegistry.Get(name)
}

//获取现有度量或创建并注册新度量。线程安全的
//在失败时调用get和register的替代方法。
func GetOrRegister(name string, i interface{}) interface{} {
	return DefaultRegistry.GetOrRegister(name, i)
}

//在给定的名称下注册给定的度量。返回重复度量值
//如果已注册给定名称的度量值。
func Register(name string, i interface{}) error {
	return DefaultRegistry.Register(name, i)
}

//在给定的名称下注册给定的度量。如果一个度量由
//给定的名称已注册。
func MustRegister(name string, i interface{}) {
	if err := Register(name, i); err != nil {
		panic(err)
	}
}

//运行所有已注册的运行状况检查。
func RunHealthchecks() {
	DefaultRegistry.RunHealthchecks()
}

//用给定的名称注销度量。
func Unregister(name string) {
	DefaultRegistry.Unregister(name)
}
