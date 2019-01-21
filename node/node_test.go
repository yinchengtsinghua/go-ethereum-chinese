
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

package node

import (
	"errors"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	testNodeKey, _ = crypto.GenerateKey()
)

func testNodeConfig() *Config {
	return &Config{
		Name: "test node",
		P2P:  p2p.Config{PrivateKey: testNodeKey},
	}
}

//测试是否可以启动、重新启动和停止空协议堆栈。
func TestNodeLifeCycle(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
//确保停止的节点可以再次停止
	for i := 0; i < 3; i++ {
		if err := stack.Stop(); err != ErrNodeStopped {
			t.Fatalf("iter %d: stop failure mismatch: have %v, want %v", i, err, ErrNodeStopped)
		}
	}
//确保节点可以成功启动，但只能启动一次
	if err := stack.Start(); err != nil {
		t.Fatalf("failed to start node: %v", err)
	}
	if err := stack.Start(); err != ErrNodeRunning {
		t.Fatalf("start failure mismatch: have %v, want %v ", err, ErrNodeRunning)
	}
//确保节点可以任意多次重新启动
	for i := 0; i < 3; i++ {
		if err := stack.Restart(); err != nil {
			t.Fatalf("iter %d: failed to restart node: %v", i, err)
		}
	}
//确保节点可以停止，但只能停止一次
	if err := stack.Stop(); err != nil {
		t.Fatalf("failed to stop node: %v", err)
	}
	if err := stack.Stop(); err != ErrNodeStopped {
		t.Fatalf("stop failure mismatch: have %v, want %v ", err, ErrNodeStopped)
	}
}

//测试如果data dir已在使用中，则返回适当的错误。
func TestNodeUsedDataDir(t *testing.T) {
//创建用作数据目录的临时文件夹
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("failed to create temporary data directory: %v", err)
	}
	defer os.RemoveAll(dir)

//基于数据目录创建新节点
	original, err := New(&Config{DataDir: dir})
	if err != nil {
		t.Fatalf("failed to create original protocol stack: %v", err)
	}
	if err := original.Start(); err != nil {
		t.Fatalf("failed to start original protocol stack: %v", err)
	}
	defer original.Stop()

//基于相同的数据目录创建第二个节点并确保失败
	duplicate, err := New(&Config{DataDir: dir})
	if err != nil {
		t.Fatalf("failed to create duplicate protocol stack: %v", err)
	}
	if err := duplicate.Start(); err != ErrDatadirUsed {
		t.Fatalf("duplicate datadir failure mismatch: have %v, want %v", err, ErrDatadirUsed)
	}
}

//测试是否可以注册服务并捕获重复项。
func TestServiceRegistry(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
//注册一批唯一的服务并确保它们成功启动
	services := []ServiceConstructor{NewNoopServiceA, NewNoopServiceB, NewNoopServiceC}
	for i, constructor := range services {
		if err := stack.Register(constructor); err != nil {
			t.Fatalf("service #%d: registration failed: %v", i, err)
		}
	}
	if err := stack.Start(); err != nil {
		t.Fatalf("failed to start original service stack: %v", err)
	}
	if err := stack.Stop(); err != nil {
		t.Fatalf("failed to stop original service stack: %v", err)
	}
//复制一个服务，然后重试启动节点
	if err := stack.Register(NewNoopServiceB); err != nil {
		t.Fatalf("duplicate registration failed: %v", err)
	}
	if err := stack.Start(); err == nil {
		t.Fatalf("duplicate service started")
	} else {
		if _, ok := err.(*DuplicateServiceError); !ok {
			t.Fatalf("duplicate error mismatch: have %v, want %v", err, DuplicateServiceError{})
		}
	}
}

//已注册服务的测试将正确启动和停止。
func TestServiceLifeCycle(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
//注册一批生命周期仪表化服务
	services := map[string]InstrumentingWrapper{
		"A": InstrumentedServiceMakerA,
		"B": InstrumentedServiceMakerB,
		"C": InstrumentedServiceMakerC,
	}
	started := make(map[string]bool)
	stopped := make(map[string]bool)

	for id, maker := range services {
id := id //施工单位关闭
		constructor := func(*ServiceContext) (Service, error) {
			return &InstrumentedService{
				startHook: func(*p2p.Server) { started[id] = true },
				stopHook:  func() { stopped[id] = true },
			}, nil
		}
		if err := stack.Register(maker(constructor)); err != nil {
			t.Fatalf("service %s: registration failed: %v", id, err)
		}
	}
//启动节点并检查所有服务是否都在运行
	if err := stack.Start(); err != nil {
		t.Fatalf("failed to start protocol stack: %v", err)
	}
	for id := range services {
		if !started[id] {
			t.Fatalf("service %s: freshly started service not running", id)
		}
		if stopped[id] {
			t.Fatalf("service %s: freshly started service already stopped", id)
		}
	}
//停止节点并检查所有服务是否已停止
	if err := stack.Stop(); err != nil {
		t.Fatalf("failed to stop protocol stack: %v", err)
	}
	for id := range services {
		if !stopped[id] {
			t.Fatalf("service %s: freshly terminated service still running", id)
		}
	}
}

//测试服务是否作为新实例干净地重新启动。
func TestServiceRestarts(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
//定义不支持重新启动的服务
	var (
		running bool
		started int
	)
	constructor := func(*ServiceContext) (Service, error) {
		running = false

		return &InstrumentedService{
			startHook: func(*p2p.Server) {
				if running {
					panic("already running")
				}
				running = true
				started++
			},
		}, nil
	}
//注册服务并启动协议栈
	if err := stack.Register(constructor); err != nil {
		t.Fatalf("failed to register the service: %v", err)
	}
	if err := stack.Start(); err != nil {
		t.Fatalf("failed to start protocol stack: %v", err)
	}
	defer stack.Stop()

	if !running || started != 1 {
		t.Fatalf("running/started mismatch: have %v/%d, want true/1", running, started)
	}
//重新启动堆栈几次，并检查成功的服务重新启动
	for i := 0; i < 3; i++ {
		if err := stack.Restart(); err != nil {
			t.Fatalf("iter %d: failed to restart stack: %v", i, err)
		}
	}
	if !running || started != 4 {
		t.Fatalf("running/started mismatch: have %v/%d, want true/4", running, started)
	}
}

//测试如果一个服务未能初始化自身，则没有其他服务
//甚至可以启动。
func TestServiceConstructionAbortion(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
//定义一批好服务
	services := map[string]InstrumentingWrapper{
		"A": InstrumentedServiceMakerA,
		"B": InstrumentedServiceMakerB,
		"C": InstrumentedServiceMakerC,
	}
	started := make(map[string]bool)
	for id, maker := range services {
id := id //施工单位关闭
		constructor := func(*ServiceContext) (Service, error) {
			return &InstrumentedService{
				startHook: func(*p2p.Server) { started[id] = true },
			}, nil
		}
		if err := stack.Register(maker(constructor)); err != nil {
			t.Fatalf("service %s: registration failed: %v", id, err)
		}
	}
//注册无法构造自身的服务
	failure := errors.New("fail")
	failer := func(*ServiceContext) (Service, error) {
		return nil, failure
	}
	if err := stack.Register(failer); err != nil {
		t.Fatalf("failer registration failed: %v", err)
	}
//启动协议栈并确保没有任何服务启动
	for i := 0; i < 100; i++ {
		if err := stack.Start(); err != failure {
			t.Fatalf("iter %d: stack startup failure mismatch: have %v, want %v", i, err, failure)
		}
		for id := range services {
			if started[id] {
				t.Fatalf("service %s: started should not have", id)
			}
			delete(started, id)
		}
	}
}

//测试如果一个服务未能启动，所有其他服务将在启动之前启动
//关闭。
func TestServiceStartupAbortion(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
//注册一批优质服务
	services := map[string]InstrumentingWrapper{
		"A": InstrumentedServiceMakerA,
		"B": InstrumentedServiceMakerB,
		"C": InstrumentedServiceMakerC,
	}
	started := make(map[string]bool)
	stopped := make(map[string]bool)

	for id, maker := range services {
id := id //施工单位关闭
		constructor := func(*ServiceContext) (Service, error) {
			return &InstrumentedService{
				startHook: func(*p2p.Server) { started[id] = true },
				stopHook:  func() { stopped[id] = true },
			}, nil
		}
		if err := stack.Register(maker(constructor)); err != nil {
			t.Fatalf("service %s: registration failed: %v", id, err)
		}
	}
//注册无法启动的服务
	failure := errors.New("fail")
	failer := func(*ServiceContext) (Service, error) {
		return &InstrumentedService{
			start: failure,
		}, nil
	}
	if err := stack.Register(failer); err != nil {
		t.Fatalf("failer registration failed: %v", err)
	}
//启动协议栈并确保所有启动的服务都停止
	for i := 0; i < 100; i++ {
		if err := stack.Start(); err != failure {
			t.Fatalf("iter %d: stack startup failure mismatch: have %v, want %v", i, err, failure)
		}
		for id := range services {
			if started[id] && !stopped[id] {
				t.Fatalf("service %s: started but not stopped", id)
			}
			delete(started, id)
			delete(stopped, id)
		}
	}
}

//测试即使注册的服务无法完全关闭，它也会
//不会影响其余的关机调用。
func TestServiceTerminationGuarantee(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
//注册一批优质服务
	services := map[string]InstrumentingWrapper{
		"A": InstrumentedServiceMakerA,
		"B": InstrumentedServiceMakerB,
		"C": InstrumentedServiceMakerC,
	}
	started := make(map[string]bool)
	stopped := make(map[string]bool)

	for id, maker := range services {
id := id //施工单位关闭
		constructor := func(*ServiceContext) (Service, error) {
			return &InstrumentedService{
				startHook: func(*p2p.Server) { started[id] = true },
				stopHook:  func() { stopped[id] = true },
			}, nil
		}
		if err := stack.Register(maker(constructor)); err != nil {
			t.Fatalf("service %s: registration failed: %v", id, err)
		}
	}
//注册一个无法完全关闭的服务
	failure := errors.New("fail")
	failer := func(*ServiceContext) (Service, error) {
		return &InstrumentedService{
			stop: failure,
		}, nil
	}
	if err := stack.Register(failer); err != nil {
		t.Fatalf("failer registration failed: %v", err)
	}
//启动协议栈，并确保失败的关机终止所有
	for i := 0; i < 100; i++ {
//启动堆栈并确保全部联机
		if err := stack.Start(); err != nil {
			t.Fatalf("iter %d: failed to start protocol stack: %v", i, err)
		}
		for id := range services {
			if !started[id] {
				t.Fatalf("iter %d, service %s: service not running", i, id)
			}
			if stopped[id] {
				t.Fatalf("iter %d, service %s: service already stopped", i, id)
			}
		}
//停止堆栈，验证故障并检查所有终端
		err := stack.Stop()
		if err, ok := err.(*StopError); !ok {
			t.Fatalf("iter %d: termination failure mismatch: have %v, want StopError", i, err)
		} else {
			failer := reflect.TypeOf(&InstrumentedService{})
			if err.Services[failer] != failure {
				t.Fatalf("iter %d: failer termination failure mismatch: have %v, want %v", i, err.Services[failer], failure)
			}
			if len(err.Services) != 1 {
				t.Fatalf("iter %d: failure count mismatch: have %d, want %d", i, len(err.Services), 1)
			}
		}
		for id := range services {
			if !stopped[id] {
				t.Fatalf("iter %d, service %s: service not terminated", i, id)
			}
			delete(started, id)
			delete(stopped, id)
		}
	}
}

//TestServiceRetrieval测试可以检索单个服务。
func TestServiceRetrieval(t *testing.T) {
//创建一个简单的堆栈并注册两种服务类型
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
	if err := stack.Register(NewNoopService); err != nil {
		t.Fatalf("noop service registration failed: %v", err)
	}
	if err := stack.Register(NewInstrumentedService); err != nil {
		t.Fatalf("instrumented service registration failed: %v", err)
	}
//确保在启动之前无法检索任何服务
	var noopServ *NoopService
	if err := stack.Service(&noopServ); err != ErrNodeStopped {
		t.Fatalf("noop service retrieval mismatch: have %v, want %v", err, ErrNodeStopped)
	}
	var instServ *InstrumentedService
	if err := stack.Service(&instServ); err != ErrNodeStopped {
		t.Fatalf("instrumented service retrieval mismatch: have %v, want %v", err, ErrNodeStopped)
	}
//启动堆栈并确保现在可以检索所有内容
	if err := stack.Start(); err != nil {
		t.Fatalf("failed to start stack: %v", err)
	}
	defer stack.Stop()

	if err := stack.Service(&noopServ); err != nil {
		t.Fatalf("noop service retrieval mismatch: have %v, want %v", err, nil)
	}
	if err := stack.Service(&instServ); err != nil {
		t.Fatalf("instrumented service retrieval mismatch: have %v, want %v", err, nil)
	}
}

//测试由单个服务定义的所有协议是否启动。
func TestProtocolGather(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
//用一些已配置的协议数注册一批服务
	services := map[string]struct {
		Count int
		Maker InstrumentingWrapper
	}{
		"zero": {0, InstrumentedServiceMakerA},
		"one":  {1, InstrumentedServiceMakerB},
		"many": {10, InstrumentedServiceMakerC},
	}
	for id, config := range services {
		protocols := make([]p2p.Protocol, config.Count)
		for i := 0; i < len(protocols); i++ {
			protocols[i].Name = id
			protocols[i].Version = uint(i)
		}
		constructor := func(*ServiceContext) (Service, error) {
			return &InstrumentedService{
				protocols: protocols,
			}, nil
		}
		if err := stack.Register(config.Maker(constructor)); err != nil {
			t.Fatalf("service %s: registration failed: %v", id, err)
		}
	}
//启动服务并确保所有协议成功启动
	if err := stack.Start(); err != nil {
		t.Fatalf("failed to start protocol stack: %v", err)
	}
	defer stack.Stop()

	protocols := stack.Server().Protocols
	if len(protocols) != 11 {
		t.Fatalf("mismatching number of protocols launched: have %d, want %d", len(protocols), 26)
	}
	for id, config := range services {
		for ver := 0; ver < config.Count; ver++ {
			launched := false
			for i := 0; i < len(protocols); i++ {
				if protocols[i].Name == id && protocols[i].Version == uint(ver) {
					launched = true
					break
				}
			}
			if !launched {
				t.Errorf("configured protocol not launched: %s v%d", id, ver)
			}
		}
	}
}

//测试由单个服务定义的所有API是否公开。
func TestAPIGather(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
//用一些已配置的API注册一批服务
	calls := make(chan string, 1)
	makeAPI := func(result string) *OneMethodAPI {
		return &OneMethodAPI{fun: func() { calls <- result }}
	}
	services := map[string]struct {
		APIs  []rpc.API
		Maker InstrumentingWrapper
	}{
		"Zero APIs": {
			[]rpc.API{}, InstrumentedServiceMakerA},
		"Single API": {
			[]rpc.API{
				{Namespace: "single", Version: "1", Service: makeAPI("single.v1"), Public: true},
			}, InstrumentedServiceMakerB},
		"Many APIs": {
			[]rpc.API{
				{Namespace: "multi", Version: "1", Service: makeAPI("multi.v1"), Public: true},
				{Namespace: "multi.v2", Version: "2", Service: makeAPI("multi.v2"), Public: true},
				{Namespace: "multi.v2.nested", Version: "2", Service: makeAPI("multi.v2.nested"), Public: true},
			}, InstrumentedServiceMakerC},
	}

	for id, config := range services {
		config := config
		constructor := func(*ServiceContext) (Service, error) {
			return &InstrumentedService{apis: config.APIs}, nil
		}
		if err := stack.Register(config.Maker(constructor)); err != nil {
			t.Fatalf("service %s: registration failed: %v", id, err)
		}
	}
//启动服务并确保所有API成功启动
	if err := stack.Start(); err != nil {
		t.Fatalf("failed to start protocol stack: %v", err)
	}
	defer stack.Stop()

//连接到RPC服务器并验证各种已注册的终结点
	client, err := stack.Attach()
	if err != nil {
		t.Fatalf("failed to connect to the inproc API server: %v", err)
	}
	defer client.Close()

	tests := []struct {
		Method string
		Result string
	}{
		{"single_theOneMethod", "single.v1"},
		{"multi_theOneMethod", "multi.v1"},
		{"multi.v2_theOneMethod", "multi.v2"},
		{"multi.v2.nested_theOneMethod", "multi.v2.nested"},
	}
	for i, test := range tests {
		if err := client.Call(nil, test.Method); err != nil {
			t.Errorf("test %d: API request failed: %v", i, err)
		}
		select {
		case result := <-calls:
			if result != test.Result {
				t.Errorf("test %d: result mismatch: have %s, want %s", i, result, test.Result)
			}
		case <-time.After(time.Second):
			t.Fatalf("test %d: rpc execution timeout", i)
		}
	}
}
