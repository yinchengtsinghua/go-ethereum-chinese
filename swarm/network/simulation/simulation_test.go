
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
	"flag"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/mattn/go-colorable"
)

var (
	loglevel = flag.Int("loglevel", 2, "verbosity of logs")
)

func init() {
	flag.Parse()
	log.PrintOrigins(true)
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(true))))
}

//如果run方法调用runfunc并正确处理上下文，则测试run。
func TestRun(t *testing.T) {
	sim := New(noopServiceFuncMap)
	defer sim.Close()

	t.Run("call", func(t *testing.T) {
		expect := "something"
		var got string
		r := sim.Run(context.Background(), func(ctx context.Context, sim *Simulation) error {
			got = expect
			return nil
		})

		if r.Error != nil {
			t.Errorf("unexpected error: %v", r.Error)
		}
		if got != expect {
			t.Errorf("expected %q, got %q", expect, got)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		r := sim.Run(ctx, func(ctx context.Context, sim *Simulation) error {
			time.Sleep(time.Second)
			return nil
		})

		if r.Error != context.DeadlineExceeded {
			t.Errorf("unexpected error: %v", r.Error)
		}
	})

	t.Run("context value and duration", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "hey", "there")
		sleep := 50 * time.Millisecond

		r := sim.Run(ctx, func(ctx context.Context, sim *Simulation) error {
			if ctx.Value("hey") != "there" {
				return errors.New("expected context value not passed")
			}
			time.Sleep(sleep)
			return nil
		})

		if r.Error != nil {
			t.Errorf("unexpected error: %v", r.Error)
		}
		if r.Duration < sleep {
			t.Errorf("reported run duration less then expected: %s", r.Duration)
		}
	})
}

//testclose测试是close方法，触发所有close函数，所有节点都不再是up。
func TestClose(t *testing.T) {
	var mu sync.Mutex
	var cleanupCount int

	sleep := 50 * time.Millisecond

	sim := New(map[string]ServiceFunc{
		"noop": func(ctx *adapters.ServiceContext, b *sync.Map) (node.Service, func(), error) {
			return newNoopService(), func() {
				time.Sleep(sleep)
				mu.Lock()
				defer mu.Unlock()
				cleanupCount++
			}, nil
		},
	})

	nodeCount := 30

	_, err := sim.AddNodes(nodeCount)
	if err != nil {
		t.Fatal(err)
	}

	var upNodeCount int
	for _, n := range sim.Net.GetNodes() {
		if n.Up {
			upNodeCount++
		}
	}
	if upNodeCount != nodeCount {
		t.Errorf("all nodes should be up, insted only %v are up", upNodeCount)
	}

	sim.Close()

	if cleanupCount != nodeCount {
		t.Errorf("number of cleanups expected %v, got %v", nodeCount, cleanupCount)
	}

	upNodeCount = 0
	for _, n := range sim.Net.GetNodes() {
		if n.Up {
			upNodeCount++
		}
	}
	if upNodeCount != 0 {
		t.Errorf("all nodes should be down, insted %v are up", upNodeCount)
	}
}

//testdone检查close方法是否触发了done通道的关闭。
func TestDone(t *testing.T) {
	sim := New(noopServiceFuncMap)
	sleep := 50 * time.Millisecond
	timeout := 2 * time.Second

	start := time.Now()
	go func() {
		time.Sleep(sleep)
		sim.Close()
	}()

	select {
	case <-time.After(timeout):
		t.Error("done channel closing timed out")
	case <-sim.Done():
		if d := time.Since(start); d < sleep {
			t.Errorf("done channel closed sooner then expected: %s", d)
		}
	}
}

//用于不执行任何操作的常规服务的帮助者映射
var noopServiceFuncMap = map[string]ServiceFunc{
	"noop": noopServiceFunc,
}

//最基本的noop服务的助手函数
func noopServiceFunc(_ *adapters.ServiceContext, _ *sync.Map) (node.Service, func(), error) {
	return newNoopService(), nil, nil
}

func newNoopService() node.Service {
	return &noopService{}
}

//最基本的noop服务的助手函数
//不同类型的，然后NoopService进行测试
//一个节点上有多个服务。
func noopService2Func(_ *adapters.ServiceContext, _ *sync.Map) (node.Service, func(), error) {
	return new(noopService2), nil, nil
}

//NoopService2是不做任何事情的服务
//但实现了node.service接口。
type noopService2 struct {
	simulations.NoopService
}

type noopService struct {
	simulations.NoopService
}
