
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

//包JSRE为JavaScript提供执行环境。
package jsre

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/internal/jsre/deps"
	"github.com/robertkrimen/otto"
)

var (
	BigNumber_JS = deps.MustAsset("bignumber.js")
	Web3_JS      = deps.MustAsset("web3.js")
)

/*
JSRE是嵌入OttoJS解释器的通用JS运行时环境。
它为
-从文件加载代码
-运行代码段
-需要库
-绑定本机go对象
**/

type JSRE struct {
	assetPath     string
	output        io.Writer
	evalQueue     chan *evalReq
	stopEventLoop chan bool
	closed        chan struct{}
}

//JSTimer是带有回调函数的单个计时器实例
type jsTimer struct {
	timer    *time.Timer
	duration time.Duration
	interval bool
	call     otto.FunctionCall
}

//evalReq是一个由runEventLoop处理的序列化VM执行请求。
type evalReq struct {
	fn   func(vm *otto.Otto)
	done chan bool
}

//运行时在使用后必须使用stop（）停止，在停止后不能使用。
func New(assetPath string, output io.Writer) *JSRE {
	re := &JSRE{
		assetPath:     assetPath,
		output:        output,
		closed:        make(chan struct{}),
		evalQueue:     make(chan *evalReq),
		stopEventLoop: make(chan bool),
	}
	go re.runEventLoop()
	re.Set("loadScript", re.loadScript)
	re.Set("inspect", re.prettyPrintJS)
	return re
}

//RandomSource返回一个伪随机值生成器。
func randomSource() *rand.Rand {
	bytes := make([]byte, 8)
	seed := time.Now().UnixNano()
	if _, err := crand.Read(bytes); err == nil {
		seed = int64(binary.LittleEndian.Uint64(bytes))
	}

	src := rand.NewSource(seed)
	return rand.New(src)
}

//此函数从启动的goroutine运行主事件循环
//创建JSRE时。在退出之前使用stop（）来正确地停止它。
//事件循环处理来自evalqueue的VM访问请求
//序列化方式，并在适当的时间调用计时器回调函数。

//导出的函数总是通过事件队列访问VM。你可以
//直接调用OttoVM的函数以绕过队列。这些
//当且仅当运行的例程
//通过RPC调用从JS调用。
func (re *JSRE) runEventLoop() {
	defer close(re.closed)

	vm := otto.New()
	r := randomSource()
	vm.SetRandomSource(r.Float64)

	registry := map[*jsTimer]*jsTimer{}
	ready := make(chan *jsTimer)

	newTimer := func(call otto.FunctionCall, interval bool) (*jsTimer, otto.Value) {
		delay, _ := call.Argument(1).ToInteger()
		if 0 >= delay {
			delay = 1
		}
		timer := &jsTimer{
			duration: time.Duration(delay) * time.Millisecond,
			call:     call,
			interval: interval,
		}
		registry[timer] = timer

		timer.timer = time.AfterFunc(timer.duration, func() {
			ready <- timer
		})

		value, err := call.Otto.ToValue(timer)
		if err != nil {
			panic(err)
		}
		return timer, value
	}

	setTimeout := func(call otto.FunctionCall) otto.Value {
		_, value := newTimer(call, false)
		return value
	}

	setInterval := func(call otto.FunctionCall) otto.Value {
		_, value := newTimer(call, true)
		return value
	}

	clearTimeout := func(call otto.FunctionCall) otto.Value {
		timer, _ := call.Argument(0).Export()
		if timer, ok := timer.(*jsTimer); ok {
			timer.timer.Stop()
			delete(registry, timer)
		}
		return otto.UndefinedValue()
	}
	vm.Set("_setTimeout", setTimeout)
	vm.Set("_setInterval", setInterval)
	vm.Run(`var setTimeout = function(args) {
		if (arguments.length < 1) {
			throw TypeError("Failed to execute 'setTimeout': 1 argument required, but only 0 present.");
		}
		return _setTimeout.apply(this, arguments);
	}`)
	vm.Run(`var setInterval = function(args) {
		if (arguments.length < 1) {
			throw TypeError("Failed to execute 'setInterval': 1 argument required, but only 0 present.");
		}
		return _setInterval.apply(this, arguments);
	}`)
	vm.Set("clearTimeout", clearTimeout)
	vm.Set("clearInterval", clearTimeout)

	var waitForCallbacks bool

loop:
	for {
		select {
		case timer := <-ready:
//执行回调，删除/重新安排计时器
			var arguments []interface{}
			if len(timer.call.ArgumentList) > 2 {
				tmp := timer.call.ArgumentList[2:]
				arguments = make([]interface{}, 2+len(tmp))
				for i, value := range tmp {
					arguments[i+2] = value
				}
			} else {
				arguments = make([]interface{}, 1)
			}
			arguments[0] = timer.call.ArgumentList[0]
			_, err := vm.Call(`Function.call.call`, nil, arguments...)
			if err != nil {
				fmt.Println("js error:", err, arguments)
			}

_, inreg := registry[timer] //当从回调中调用ClearInterval时，不要重置它
			if timer.interval && inreg {
				timer.timer.Reset(timer.duration)
			} else {
				delete(registry, timer)
				if waitForCallbacks && (len(registry) == 0) {
					break loop
				}
			}
		case req := <-re.evalQueue:
//运行代码，返回结果
			req.fn(vm)
			close(req.done)
			if waitForCallbacks && (len(registry) == 0) {
				break loop
			}
		case waitForCallbacks = <-re.stopEventLoop:
			if !waitForCallbacks || (len(registry) == 0) {
				break loop
			}
		}
	}

	for _, timer := range registry {
		timer.timer.Stop()
		delete(registry, timer)
	}
}

//do在JS事件循环上执行给定的函数。
func (re *JSRE) Do(fn func(*otto.Otto)) {
	done := make(chan bool)
	req := &evalReq{fn, done}
	re.evalQueue <- req
	<-done
}

//在退出前停止事件循环，或者等待所有计时器过期
func (re *JSRE) Stop(waitForCallbacks bool) {
	select {
	case <-re.closed:
	case re.stopEventLoop <- waitForCallbacks:
		<-re.closed
	}
}

//exec（文件）加载并运行文件的内容
//if a relative path is given, the jsre's assetPath is used
func (re *JSRE) Exec(file string) error {
	code, err := ioutil.ReadFile(common.AbsolutePath(re.assetPath, file))
	if err != nil {
		return err
	}
	var script *otto.Script
	re.Do(func(vm *otto.Otto) {
		script, err = vm.Compile(file, code)
		if err != nil {
			return
		}
		_, err = vm.Run(script)
	})
	return err
}

//bind将值v赋给JS环境中的变量
//此方法已弃用，请使用set。
func (re *JSRE) Bind(name string, v interface{}) error {
	return re.Set(name, v)
}

//运行运行一段JS代码。
func (re *JSRE) Run(code string) (v otto.Value, err error) {
	re.Do(func(vm *otto.Otto) { v, err = vm.Run(code) })
	return v, err
}

//get返回JS环境中变量的值。
func (re *JSRE) Get(ns string) (v otto.Value, err error) {
	re.Do(func(vm *otto.Otto) { v, err = vm.Get(ns) })
	return v, err
}

//set将值v赋给JS环境中的变量。
func (re *JSRE) Set(ns string, v interface{}) (err error) {
	re.Do(func(vm *otto.Otto) { err = vm.Set(ns, v) })
	return err
}

//loadscript从当前执行的JS代码内部执行JS脚本。
func (re *JSRE) loadScript(call otto.FunctionCall) otto.Value {
	file, err := call.Argument(0).ToString()
	if err != nil {
//TODO:引发异常
		return otto.FalseValue()
	}
	file = common.AbsolutePath(re.assetPath, file)
	source, err := ioutil.ReadFile(file)
	if err != nil {
//TODO:引发异常
		return otto.FalseValue()
	}
	if _, err := compileAndRun(call.Otto, file, source); err != nil {
//TODO:引发异常
		fmt.Println("err:", err)
		return otto.FalseValue()
	}
//TODO:返回评估结果
	return otto.TrueValue()
}

//evaluate执行代码并将结果漂亮地打印到指定的输出
//溪流。
func (re *JSRE) Evaluate(code string, w io.Writer) error {
	var fail error

	re.Do(func(vm *otto.Otto) {
		val, err := vm.Run(code)
		if err != nil {
			prettyError(vm, err, w)
		} else {
			prettyPrint(vm, val, w)
		}
		fmt.Fprintln(w)
	})
	return fail
}

//编译然后运行一段JS代码。
func (re *JSRE) Compile(filename string, src interface{}) (err error) {
	re.Do(func(vm *otto.Otto) { _, err = compileAndRun(vm, filename, src) })
	return err
}

func compileAndRun(vm *otto.Otto, filename string, src interface{}) (otto.Value, error) {
	script, err := vm.Compile(filename, src)
	if err != nil {
		return otto.Value{}, err
	}
	return vm.Run(script)
}
