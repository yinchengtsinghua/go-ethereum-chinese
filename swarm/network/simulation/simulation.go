
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

package simulation

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/network"
)

//此包中的函数返回的常见错误。
var (
	ErrNodeNotFound = errors.New("node not found")
)

//仿真提供了网络、节点和服务的方法
//管理它们。
type Simulation struct {
//NET作为一种访问低级功能的方式被公开
//P2P/Simulations.Network的。
	Net *simulations.Network

	serviceNames      []string
	cleanupFuncs      []func()
	buckets           map[enode.ID]*sync.Map
	shutdownWG        sync.WaitGroup
	done              chan struct{}
	mu                sync.RWMutex
	neighbourhoodSize int

httpSrv *http.Server        //通过模拟选项附加HTTP服务器
handler *simulations.Server //服务器的HTTP处理程序
runC    chan struct{}       //前端信号准备就绪的通道
}

//servicefunc在new中用于声明新的服务构造函数。
//第一个参数提供来自适配器包的ServiceContext
//例如对nodeid的访问。第二个参数是sync.map
//所有与服务相关的“全局”状态都应该保存在哪里。
//施工服务和任何其他施工所需的所有清理
//对象应该在单个返回的清理函数中提供。
//close函数将调用返回的cleanup函数
//网络关闭后。
type ServiceFunc func(ctx *adapters.ServiceContext, bucket *sync.Map) (s node.Service, cleanup func(), err error)

//新建创建新的模拟实例
//服务映射必须具有唯一的键作为服务名和
//每个servicefunc都必须返回一个唯一类型的node.service。
//node.node.start（）函数需要此限制
//用于启动node.servicefunc返回的服务。
func New(services map[string]ServiceFunc) (s *Simulation) {
	s = &Simulation{
		buckets:           make(map[enode.ID]*sync.Map),
		done:              make(chan struct{}),
		neighbourhoodSize: network.NewKadParams().NeighbourhoodSize,
	}

	adapterServices := make(map[string]adapters.ServiceFunc, len(services))
	for name, serviceFunc := range services {
//正确地确定这个变量的范围
//因为它们将出现在稍后访问的adapterservices[name]函数中。
		name, serviceFunc := name, serviceFunc
		s.serviceNames = append(s.serviceNames, name)
		adapterServices[name] = func(ctx *adapters.ServiceContext) (node.Service, error) {
			b := new(sync.Map)
			service, cleanup, err := serviceFunc(ctx, b)
			if err != nil {
				return nil, err
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			if cleanup != nil {
				s.cleanupFuncs = append(s.cleanupFuncs, cleanup)
			}
			s.buckets[ctx.Config.ID] = b
			return service, nil
		}
	}

	s.Net = simulations.NewNetwork(
		adapters.NewTCPAdapter(adapterServices),
		&simulations.NetworkConfig{ID: "0"},
	)

	return s
}

//runfunc是将调用的函数
//在Simulation.Run方法调用中。
type RunFunc func(context.Context, *Simulation) error

//结果是Simulation.Run方法的返回值。
type Result struct {
	Duration time.Duration
	Error    error
}

//run在处理时调用runfunc函数
//通过上下文提供的取消。
func (s *Simulation) Run(ctx context.Context, f RunFunc) (r Result) {
//如果该选项设置为使用模拟运行HTTP服务器，
//初始化服务器并启动它
	start := time.Now()
	if s.httpSrv != nil {
		log.Info("Waiting for frontend to be ready...(send POST /runsim to HTTP server)")
//等待前端连接
		select {
		case <-s.runC:
		case <-ctx.Done():
			return Result{
				Duration: time.Since(start),
				Error:    ctx.Err(),
			}
		}
		log.Info("Received signal from frontend - starting simulation run.")
	}
	errc := make(chan error)
	quit := make(chan struct{})
	defer close(quit)
	go func() {
		select {
		case errc <- f(ctx, s):
		case <-quit:
		}
	}()
	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-errc:
	}
	return Result{
		Duration: time.Since(start),
		Error:    err,
	}
}

//对上清理函数的最大并行调用数
//模拟关闭。
var maxParallelCleanups = 10

//close调用由返回的所有清理函数
//servicefunc，等待它们全部完成
//显式阻止shutdownwg的函数
//（如Simulation.PeerEvents）并关闭网络
//最后。它用于清除
//模拟。
func (s *Simulation) Close() {
	close(s.done)

	sem := make(chan struct{}, maxParallelCleanups)
	s.mu.RLock()
	cleanupFuncs := make([]func(), len(s.cleanupFuncs))
	for i, f := range s.cleanupFuncs {
		if f != nil {
			cleanupFuncs[i] = f
		}
	}
	s.mu.RUnlock()
	var cleanupWG sync.WaitGroup
	for _, cleanup := range cleanupFuncs {
		cleanupWG.Add(1)
		sem <- struct{}{}
		go func(cleanup func()) {
			defer cleanupWG.Done()
			defer func() { <-sem }()

			cleanup()
		}(cleanup)
	}
	cleanupWG.Wait()

	if s.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := s.httpSrv.Shutdown(ctx)
		if err != nil {
			log.Error("Error shutting down HTTP server!", "err", err)
		}
		close(s.runC)
	}

	s.shutdownWG.Wait()
	s.Net.Shutdown()
}

//完成返回模拟时关闭的通道
//用Close方法关闭。它对信号终端很有用
//在测试中创建的所有可能的goroutine中。
func (s *Simulation) Done() <-chan struct{} {
	return s.done
}
