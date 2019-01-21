
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

package testing

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
)

var errTimedOut = errors.New("timed out")

//ProtocolSession是对运行的枢轴节点的准模拟。
//可以发送（触发器）或
//接收（预期）消息
type ProtocolSession struct {
	Server  *p2p.Server
	Nodes   []*enode.Node
	adapter *adapters.SimAdapter
	events  chan *p2p.PeerEvent
}

//交换是协议测试的基本单元
//数组中的触发器和预期将立即异步运行
//因此，对于具有不同消息类型的同一对等端，不能有多个预期值
//因为它是不可预测的，哪个期望会收到哪个消息
//（对于Expect 1和2，可能会发送消息2和1，两个Expect都会抱怨错误的消息代码）
//在会话上定义交换
type Exchange struct {
	Label    string
	Triggers []Trigger
	Expects  []Expect
	Timeout  time.Duration
}

//触发器是交换的一部分，透视节点的传入消息
//同行发送
type Trigger struct {
Msg     interface{}   //要发送的消息类型
Code    uint64        //给出报文代码
Peer    enode.ID      //向其发送消息的对等方
Timeout time.Duration //发送超时时间
}

//Expect是来自透视节点的交换传出消息的一部分
//由对等方接收
type Expect struct {
Msg     interface{}   //预期的消息类型
Code    uint64        //现在给出了消息代码
Peer    enode.ID      //期望消息的对等机
Timeout time.Duration //接收超时时间
}

//disconnect表示一个disconnect事件，由testdisconnected使用并检查
type Disconnect struct {
Peer  enode.ID //DiscConnected对等
Error error    //断开连接原因
}

//触发器从对等方发送消息
func (s *ProtocolSession) trigger(trig Trigger) error {
	simNode, ok := s.adapter.GetNode(trig.Peer)
	if !ok {
		return fmt.Errorf("trigger: peer %v does not exist (1- %v)", trig.Peer, len(s.Nodes))
	}
	mockNode, ok := simNode.Services()[0].(*mockNode)
	if !ok {
		return fmt.Errorf("trigger: peer %v is not a mock", trig.Peer)
	}

	errc := make(chan error)

	go func() {
		log.Trace(fmt.Sprintf("trigger %v (%v)....", trig.Msg, trig.Code))
		errc <- mockNode.Trigger(&trig)
		log.Trace(fmt.Sprintf("triggered %v (%v)", trig.Msg, trig.Code))
	}()

	t := trig.Timeout
	if t == time.Duration(0) {
		t = 1000 * time.Millisecond
	}
	select {
	case err := <-errc:
		return err
	case <-time.After(t):
		return fmt.Errorf("timout expecting %v to send to peer %v", trig.Msg, trig.Peer)
	}
}

//Expect检查透视节点发送的消息的期望值
func (s *ProtocolSession) expect(exps []Expect) error {
//构建每个节点的期望图
	peerExpects := make(map[enode.ID][]Expect)
	for _, exp := range exps {
		if exp.Msg == nil {
			return errors.New("no message to expect")
		}
		peerExpects[exp.Peer] = append(peerExpects[exp.Peer], exp)
	}

//为每个节点构造一个mocknode映射
	mockNodes := make(map[enode.ID]*mockNode)
	for nodeID := range peerExpects {
		simNode, ok := s.adapter.GetNode(nodeID)
		if !ok {
			return fmt.Errorf("trigger: peer %v does not exist (1- %v)", nodeID, len(s.Nodes))
		}
		mockNode, ok := simNode.Services()[0].(*mockNode)
		if !ok {
			return fmt.Errorf("trigger: peer %v is not a mock", nodeID)
		}
		mockNodes[nodeID] = mockNode
	}

//done chanell在函数返回时取消所有创建的goroutine
	done := make(chan struct{})
	defer close(done)
//errc从中捕获第一个错误
	errc := make(chan error)

	wg := &sync.WaitGroup{}
	wg.Add(len(mockNodes))
	for nodeID, mockNode := range mockNodes {
		nodeID := nodeID
		mockNode := mockNode
		go func() {
			defer wg.Done()

//求和所有期望超时值以给出最大值
//完成所有期望的时间。
//mocknode.expect检查所有接收到的消息
//每个消息的预期消息和超时列表
//其中不能单独检查。
			var t time.Duration
			for _, exp := range peerExpects[nodeID] {
				if exp.Timeout == time.Duration(0) {
					t += 2000 * time.Millisecond
				} else {
					t += exp.Timeout
				}
			}
			alarm := time.NewTimer(t)
			defer alarm.Stop()

//expecterrc用于检查是否返回错误
//从mocknode.expect不是nil并将其发送到
//只有在这种情况下才是errc。
//完成通道将在功能关闭时关闭
			expectErrc := make(chan error)
			go func() {
				select {
				case expectErrc <- mockNode.Expect(peerExpects[nodeID]...):
				case <-done:
				case <-alarm.C:
				}
			}()

			select {
			case err := <-expectErrc:
				if err != nil {
					select {
					case errc <- err:
					case <-done:
					case <-alarm.C:
						errc <- errTimedOut
					}
				}
			case <-done:
			case <-alarm.C:
				errc <- errTimedOut
			}

		}()
	}

	go func() {
		wg.Wait()
//当所有Goroutine完成时关闭errc，从errc返回nill err
		close(errc)
	}()

	return <-errc
}

//测试交换测试一系列针对会话的交换
func (s *ProtocolSession) TestExchanges(exchanges ...Exchange) error {
	for i, e := range exchanges {
		if err := s.testExchange(e); err != nil {
			return fmt.Errorf("exchange #%d %q: %v", i, e.Label, err)
		}
		log.Trace(fmt.Sprintf("exchange #%d %q: run successfully", i, e.Label))
	}
	return nil
}

//测试交换测试单个交换。
//默认超时值为2秒。
func (s *ProtocolSession) testExchange(e Exchange) error {
	errc := make(chan error)
	done := make(chan struct{})
	defer close(done)

	go func() {
		for _, trig := range e.Triggers {
			err := s.trigger(trig)
			if err != nil {
				errc <- err
				return
			}
		}

		select {
		case errc <- s.expect(e.Expects):
		case <-done:
		}
	}()

//全球超时或在所有期望都满足时结束
	t := e.Timeout
	if t == 0 {
		t = 2000 * time.Millisecond
	}
	alarm := time.NewTimer(t)
	select {
	case err := <-errc:
		return err
	case <-alarm.C:
		return errTimedOut
	}
}

//testDisconnected测试作为参数提供的断开
//disconnect结构描述了在哪个对等机上预期出现的断开连接错误
func (s *ProtocolSession) TestDisconnected(disconnects ...*Disconnect) error {
	expects := make(map[enode.ID]error)
	for _, disconnect := range disconnects {
		expects[disconnect.Peer] = disconnect.Error
	}

	timeout := time.After(time.Second)
	for len(expects) > 0 {
		select {
		case event := <-s.events:
			if event.Type != p2p.PeerEventTypeDrop {
				continue
			}
			expectErr, ok := expects[event.Peer]
			if !ok {
				continue
			}

			if !(expectErr == nil && event.Error == "" || expectErr != nil && expectErr.Error() == event.Error) {
				return fmt.Errorf("unexpected error on peer %v. expected '%v', got '%v'", event.Peer, expectErr, event.Error)
			}
			delete(expects, event.Peer)
		case <-timeout:
			return fmt.Errorf("timed out waiting for peers to disconnect")
		}
	}
	return nil
}
