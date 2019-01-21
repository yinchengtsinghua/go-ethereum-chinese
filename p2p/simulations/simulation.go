
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

package simulations

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"
)

//模拟为在模拟网络中运行操作提供了一个框架
//然后等待期望得到满足
type Simulation struct {
	network *Network
}

//新模拟返回在给定网络中运行的新模拟
func NewSimulation(network *Network) *Simulation {
	return &Simulation{
		network: network,
	}
}

//运行通过执行步骤的操作执行模拟步骤，并
//然后等待步骤的期望得到满足
func (s *Simulation) Run(ctx context.Context, step *Step) (result *StepResult) {
	result = newStepResult()

	result.StartedAt = time.Now()
	defer func() { result.FinishedAt = time.Now() }()

//在步骤期间监视网络事件
	stop := s.watchNetwork(result)
	defer stop()

//执行操作
	if err := step.Action(ctx); err != nil {
		result.Error = err
		return
	}

//等待所有节点期望通过、错误或超时
	nodes := make(map[enode.ID]struct{}, len(step.Expect.Nodes))
	for _, id := range step.Expect.Nodes {
		nodes[id] = struct{}{}
	}
	for len(result.Passes) < len(nodes) {
		select {
		case id := <-step.Trigger:
//如果不检查节点，则跳过
			if _, ok := nodes[id]; !ok {
				continue
			}

//如果节点已通过，则跳过
			if _, ok := result.Passes[id]; ok {
				continue
			}

//运行节点期望检查
			pass, err := step.Expect.Check(ctx, id)
			if err != nil {
				result.Error = err
				return
			}
			if pass {
				result.Passes[id] = time.Now()
			}
		case <-ctx.Done():
			result.Error = ctx.Err()
			return
		}
	}

	return
}

func (s *Simulation) watchNetwork(result *StepResult) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	events := make(chan *Event)
	sub := s.network.Events().Subscribe(events)
	go func() {
		defer close(done)
		defer sub.Unsubscribe()
		for {
			select {
			case event := <-events:
				result.NetworkEvents = append(result.NetworkEvents, event)
			case <-stop:
				return
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

type Step struct {
//操作是为此步骤执行的操作
	Action func(context.Context) error

//触发器是一个接收节点ID并触发
//该节点的预期检查
	Trigger chan enode.ID

//Expect是执行此步骤时等待的期望
	Expect *Expectation
}

type Expectation struct {
//节点是要检查的节点列表
	Nodes []enode.ID

//检查给定节点是否满足预期
	Check func(context.Context, enode.ID) (bool, error)
}

func newStepResult() *StepResult {
	return &StepResult{
		Passes: make(map[enode.ID]time.Time),
	}
}

type StepResult struct {
//错误是运行步骤时遇到的错误
	Error error

//Startedat是步骤开始的时间
	StartedAt time.Time

//FinishedAt是步骤完成的时间。
	FinishedAt time.Time

//传递是成功节点期望的时间戳
	Passes map[enode.ID]time.Time

//NetworkEvents是在步骤中发生的网络事件。
	NetworkEvents []*Event
}
