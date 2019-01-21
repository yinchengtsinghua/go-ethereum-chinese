
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

/*
p2p/测试包提供了一个单元测试方案来检查
协议消息与一个透视节点和多个虚拟对等点交换
透视测试节点运行一个节点。服务，虚拟对等运行一个模拟节点
可用于发送和接收消息的
**/


package testing

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
)

//ProtocolTester是用于单元测试协议的测试环境
//消息交换。它使用P2P/仿真框架
type ProtocolTester struct {
	*ProtocolSession
	network *simulations.Network
}

//NewProtocolTester构造了一个新的ProtocolTester
//它将透视节点ID、虚拟对等数和
//P2P服务器在对等连接上调用的协议运行函数
func NewProtocolTester(t *testing.T, id enode.ID, n int, run func(*p2p.Peer, p2p.MsgReadWriter) error) *ProtocolTester {
	services := adapters.Services{
		"test": func(ctx *adapters.ServiceContext) (node.Service, error) {
			return &testNode{run}, nil
		},
		"mock": func(ctx *adapters.ServiceContext) (node.Service, error) {
			return newMockNode(), nil
		},
	}
	adapter := adapters.NewSimAdapter(services)
	net := simulations.NewNetwork(adapter, &simulations.NetworkConfig{})
	if _, err := net.NewNodeWithConfig(&adapters.NodeConfig{
		ID:              id,
		EnableMsgEvents: true,
		Services:        []string{"test"},
	}); err != nil {
		panic(err.Error())
	}
	if err := net.Start(id); err != nil {
		panic(err.Error())
	}

	node := net.GetNode(id).Node.(*adapters.SimNode)
	peers := make([]*adapters.NodeConfig, n)
	nodes := make([]*enode.Node, n)
	for i := 0; i < n; i++ {
		peers[i] = adapters.RandomNodeConfig()
		peers[i].Services = []string{"mock"}
		nodes[i] = peers[i].Node()
	}
	events := make(chan *p2p.PeerEvent, 1000)
	node.SubscribeEvents(events)
	ps := &ProtocolSession{
		Server:  node.Server(),
		Nodes:   nodes,
		adapter: adapter,
		events:  events,
	}
	self := &ProtocolTester{
		ProtocolSession: ps,
		network:         net,
	}

	self.Connect(id, peers...)

	return self
}

//停止停止P2P服务器
func (t *ProtocolTester) Stop() error {
	t.Server.Stop()
	return nil
}

//Connect打开远程对等节点并使用
//P2P/模拟与内存网络适配器的网络连接
func (t *ProtocolTester) Connect(selfID enode.ID, peers ...*adapters.NodeConfig) {
	for _, peer := range peers {
		log.Trace(fmt.Sprintf("start node %v", peer.ID))
		if _, err := t.network.NewNodeWithConfig(peer); err != nil {
			panic(fmt.Sprintf("error starting peer %v: %v", peer.ID, err))
		}
		if err := t.network.Start(peer.ID); err != nil {
			panic(fmt.Sprintf("error starting peer %v: %v", peer.ID, err))
		}
		log.Trace(fmt.Sprintf("connect to %v", peer.ID))
		if err := t.network.Connect(selfID, peer.ID); err != nil {
			panic(fmt.Sprintf("error connecting to peer %v: %v", peer.ID, err))
		}
	}

}

//testnode包装协议运行函数并实现node.service
//界面
type testNode struct {
	run func(*p2p.Peer, p2p.MsgReadWriter) error
}

func (t *testNode) Protocols() []p2p.Protocol {
	return []p2p.Protocol{{
		Length: 100,
		Run:    t.run,
	}}
}

func (t *testNode) APIs() []rpc.API {
	return nil
}

func (t *testNode) Start(server *p2p.Server) error {
	return nil
}

func (t *testNode) Stop() error {
	return nil
}

//mocknode是一个没有实际运行协议的testnode
//公开通道，以便测试可以手动触发并预期
//信息
type mockNode struct {
	testNode

	trigger  chan *Trigger
	expect   chan []Expect
	err      chan error
	stop     chan struct{}
	stopOnce sync.Once
}

func newMockNode() *mockNode {
	mock := &mockNode{
		trigger: make(chan *Trigger),
		expect:  make(chan []Expect),
		err:     make(chan error),
		stop:    make(chan struct{}),
	}
	mock.testNode.run = mock.Run
	return mock
}

//运行是一个协议运行函数，它只循环等待测试
//指示它触发或期望来自对等端的消息
func (m *mockNode) Run(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
	for {
		select {
		case trig := <-m.trigger:
			wmsg := Wrap(trig.Msg)
			m.err <- p2p.Send(rw, trig.Code, wmsg)
		case exps := <-m.expect:
			m.err <- expectMsgs(rw, exps)
		case <-m.stop:
			return nil
		}
	}
}

func (m *mockNode) Trigger(trig *Trigger) error {
	m.trigger <- trig
	return <-m.err
}

func (m *mockNode) Expect(exp ...Expect) error {
	m.expect <- exp
	return <-m.err
}

func (m *mockNode) Stop() error {
	m.stopOnce.Do(func() { close(m.stop) })
	return nil
}

func expectMsgs(rw p2p.MsgReadWriter, exps []Expect) error {
	matched := make([]bool, len(exps))
	for {
		msg, err := rw.ReadMsg()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		actualContent, err := ioutil.ReadAll(msg.Payload)
		if err != nil {
			return err
		}
		var found bool
		for i, exp := range exps {
			if exp.Code == msg.Code && bytes.Equal(actualContent, mustEncodeMsg(Wrap(exp.Msg))) {
				if matched[i] {
					return fmt.Errorf("message #%d received two times", i)
				}
				matched[i] = true
				found = true
				break
			}
		}
		if !found {
			expected := make([]string, 0)
			for i, exp := range exps {
				if matched[i] {
					continue
				}
				expected = append(expected, fmt.Sprintf("code %d payload %x", exp.Code, mustEncodeMsg(Wrap(exp.Msg))))
			}
			return fmt.Errorf("unexpected message code %d payload %x, expected %s", msg.Code, actualContent, strings.Join(expected, " or "))
		}
		done := true
		for _, m := range matched {
			if !m {
				done = false
				break
			}
		}
		if done {
			return nil
		}
	}
	for i, m := range matched {
		if !m {
			return fmt.Errorf("expected message #%d not received", i)
		}
	}
	return nil
}

//mustencodemsg使用rlp对消息进行编码。
//一旦出错，它就会惊慌失措。
func mustEncodeMsg(msg interface{}) []byte {
	contentEnc, err := rlp.EncodeToBytes(msg)
	if err != nil {
		panic("content encode error: " + err.Error())
	}
	return contentEnc
}

type WrappedMsg struct {
	Context []byte
	Size    uint32
	Payload []byte
}

func Wrap(msg interface{}) interface{} {
	data, _ := rlp.EncodeToBytes(msg)
	return &WrappedMsg{
		Size:    uint32(len(data)),
		Payload: data,
	}
}
