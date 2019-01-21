
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

package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	p2ptest "github.com/ethereum/go-ethereum/p2p/testing"
	"github.com/ethereum/go-ethereum/swarm/network"
	"golang.org/x/crypto/sha3"
)

func TestStreamerSubscribe(t *testing.T) {
	tester, streamer, _, teardown, err := newStreamerTester(t, nil)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	stream := NewStream("foo", "", true)
	err = streamer.Subscribe(tester.Nodes[0].ID(), stream, NewRange(0, 0), Top)
	if err == nil || err.Error() != "stream foo not registered" {
		t.Fatalf("Expected error %v, got %v", "stream foo not registered", err)
	}
}

func TestStreamerRequestSubscription(t *testing.T) {
	tester, streamer, _, teardown, err := newStreamerTester(t, nil)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	stream := NewStream("foo", "", false)
	err = streamer.RequestSubscription(tester.Nodes[0].ID(), stream, &Range{}, Top)
	if err == nil || err.Error() != "stream foo not registered" {
		t.Fatalf("Expected error %v, got %v", "stream foo not registered", err)
	}
}

var (
	hash0         = sha3.Sum256([]byte{0})
	hash1         = sha3.Sum256([]byte{1})
	hash2         = sha3.Sum256([]byte{2})
	hashesTmp     = append(hash0[:], hash1[:]...)
	hashes        = append(hashesTmp, hash2[:]...)
	corruptHashes = append(hashes[:40])
)

type testClient struct {
	t              string
	wait0          chan bool
	wait2          chan bool
	batchDone      chan bool
	receivedHashes map[string][]byte
}

func newTestClient(t string) *testClient {
	return &testClient{
		t:              t,
		wait0:          make(chan bool),
		wait2:          make(chan bool),
		batchDone:      make(chan bool),
		receivedHashes: make(map[string][]byte),
	}
}

func (self *testClient) NeedData(ctx context.Context, hash []byte) func(context.Context) error {
	self.receivedHashes[string(hash)] = hash
	if bytes.Equal(hash, hash0[:]) {
		return func(context.Context) error {
			<-self.wait0
			return nil
		}
	} else if bytes.Equal(hash, hash2[:]) {
		return func(context.Context) error {
			<-self.wait2
			return nil
		}
	}
	return nil
}

func (self *testClient) BatchDone(Stream, uint64, []byte, []byte) func() (*TakeoverProof, error) {
	close(self.batchDone)
	return nil
}

func (self *testClient) Close() {}

type testServer struct {
	t            string
	sessionIndex uint64
}

func newTestServer(t string, sessionIndex uint64) *testServer {
	return &testServer{
		t:            t,
		sessionIndex: sessionIndex,
	}
}

func (s *testServer) SessionIndex() (uint64, error) {
	return s.sessionIndex, nil
}

func (self *testServer) SetNextBatch(from uint64, to uint64) ([]byte, uint64, uint64, *HandoverProof, error) {
	return make([]byte, HashSize), from + 1, to + 1, nil, nil
}

func (self *testServer) GetData(context.Context, []byte) ([]byte, error) {
	return nil, nil
}

func (self *testServer) Close() {
}

func TestStreamerDownstreamSubscribeUnsubscribeMsgExchange(t *testing.T) {
	tester, streamer, _, teardown, err := newStreamerTester(t, nil)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	streamer.RegisterClientFunc("foo", func(p *Peer, t string, live bool) (Client, error) {
		return newTestClient(t), nil
	})

	node := tester.Nodes[0]

	stream := NewStream("foo", "", true)
	err = streamer.Subscribe(node.ID(), stream, NewRange(5, 8), Top)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = tester.TestExchanges(
		p2ptest.Exchange{
			Label: "Subscribe message",
			Expects: []p2ptest.Expect{
				{
					Code: 4,
					Msg: &SubscribeMsg{
						Stream:   stream,
						History:  NewRange(5, 8),
						Priority: Top,
					},
					Peer: node.ID(),
				},
			},
		},
//触发器offeredhashemsg以实际创建客户端
		p2ptest.Exchange{
			Label: "OfferedHashes message",
			Triggers: []p2ptest.Trigger{
				{
					Code: 1,
					Msg: &OfferedHashesMsg{
						HandoverProof: &HandoverProof{
							Handover: &Handover{},
						},
						Hashes: hashes,
						From:   5,
						To:     8,
						Stream: stream,
					},
					Peer: node.ID(),
				},
			},
			Expects: []p2ptest.Expect{
				{
					Code: 2,
					Msg: &WantedHashesMsg{
						Stream: stream,
						Want:   []byte{5},
						From:   9,
						To:     0,
					},
					Peer: node.ID(),
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	err = streamer.Unsubscribe(node.ID(), stream)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "Unsubscribe message",
		Expects: []p2ptest.Expect{
			{
				Code: 0,
				Msg: &UnsubscribeMsg{
					Stream: stream,
				},
				Peer: node.ID(),
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestStreamerUpstreamSubscribeUnsubscribeMsgExchange(t *testing.T) {
	tester, streamer, _, teardown, err := newStreamerTester(t, nil)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	stream := NewStream("foo", "", false)

	streamer.RegisterServerFunc("foo", func(p *Peer, t string, live bool) (Server, error) {
		return newTestServer(t, 10), nil
	})

	node := tester.Nodes[0]

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "Subscribe message",
		Triggers: []p2ptest.Trigger{
			{
				Code: 4,
				Msg: &SubscribeMsg{
					Stream:   stream,
					History:  NewRange(5, 8),
					Priority: Top,
				},
				Peer: node.ID(),
			},
		},
		Expects: []p2ptest.Expect{
			{
				Code: 1,
				Msg: &OfferedHashesMsg{
					Stream: stream,
					HandoverProof: &HandoverProof{
						Handover: &Handover{},
					},
					Hashes: make([]byte, HashSize),
					From:   6,
					To:     9,
				},
				Peer: node.ID(),
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "unsubscribe message",
		Triggers: []p2ptest.Trigger{
			{
				Code: 0,
				Msg: &UnsubscribeMsg{
					Stream: stream,
				},
				Peer: node.ID(),
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestStreamerUpstreamSubscribeUnsubscribeMsgExchangeLive(t *testing.T) {
	tester, streamer, _, teardown, err := newStreamerTester(t, nil)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	stream := NewStream("foo", "", true)

	streamer.RegisterServerFunc("foo", func(p *Peer, t string, live bool) (Server, error) {
		return newTestServer(t, 0), nil
	})

	node := tester.Nodes[0]

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "Subscribe message",
		Triggers: []p2ptest.Trigger{
			{
				Code: 4,
				Msg: &SubscribeMsg{
					Stream:   stream,
					Priority: Top,
				},
				Peer: node.ID(),
			},
		},
		Expects: []p2ptest.Expect{
			{
				Code: 1,
				Msg: &OfferedHashesMsg{
					Stream: stream,
					HandoverProof: &HandoverProof{
						Handover: &Handover{},
					},
					Hashes: make([]byte, HashSize),
					From:   1,
					To:     0,
				},
				Peer: node.ID(),
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "unsubscribe message",
		Triggers: []p2ptest.Trigger{
			{
				Code: 0,
				Msg: &UnsubscribeMsg{
					Stream: stream,
				},
				Peer: node.ID(),
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestStreamerUpstreamSubscribeErrorMsgExchange(t *testing.T) {
	tester, streamer, _, teardown, err := newStreamerTester(t, nil)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	streamer.RegisterServerFunc("foo", func(p *Peer, t string, live bool) (Server, error) {
		return newTestServer(t, 0), nil
	})

	stream := NewStream("bar", "", true)

	node := tester.Nodes[0]

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "Subscribe message",
		Triggers: []p2ptest.Trigger{
			{
				Code: 4,
				Msg: &SubscribeMsg{
					Stream:   stream,
					History:  NewRange(5, 8),
					Priority: Top,
				},
				Peer: node.ID(),
			},
		},
		Expects: []p2ptest.Expect{
			{
				Code: 7,
				Msg: &SubscribeErrorMsg{
					Error: "stream bar not registered",
				},
				Peer: node.ID(),
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestStreamerUpstreamSubscribeLiveAndHistory(t *testing.T) {
	tester, streamer, _, teardown, err := newStreamerTester(t, nil)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	stream := NewStream("foo", "", true)

	streamer.RegisterServerFunc("foo", func(p *Peer, t string, live bool) (Server, error) {
		return newTestServer(t, 10), nil
	})

	node := tester.Nodes[0]

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "Subscribe message",
		Triggers: []p2ptest.Trigger{
			{
				Code: 4,
				Msg: &SubscribeMsg{
					Stream:   stream,
					History:  NewRange(5, 8),
					Priority: Top,
				},
				Peer: node.ID(),
			},
		},
		Expects: []p2ptest.Expect{
			{
				Code: 1,
				Msg: &OfferedHashesMsg{
					Stream: NewStream("foo", "", false),
					HandoverProof: &HandoverProof{
						Handover: &Handover{},
					},
					Hashes: make([]byte, HashSize),
					From:   6,
					To:     9,
				},
				Peer: node.ID(),
			},
			{
				Code: 1,
				Msg: &OfferedHashesMsg{
					Stream: stream,
					HandoverProof: &HandoverProof{
						Handover: &Handover{},
					},
					From:   11,
					To:     0,
					Hashes: make([]byte, HashSize),
				},
				Peer: node.ID(),
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestStreamerDownstreamCorruptHashesMsgExchange(t *testing.T) {
	tester, streamer, _, teardown, err := newStreamerTester(t, nil)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	stream := NewStream("foo", "", true)

	var tc *testClient

	streamer.RegisterClientFunc("foo", func(p *Peer, t string, live bool) (Client, error) {
		tc = newTestClient(t)
		return tc, nil
	})

	node := tester.Nodes[0]

	err = streamer.Subscribe(node.ID(), stream, NewRange(5, 8), Top)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "Subscribe message",
		Expects: []p2ptest.Expect{
			{
				Code: 4,
				Msg: &SubscribeMsg{
					Stream:   stream,
					History:  NewRange(5, 8),
					Priority: Top,
				},
				Peer: node.ID(),
			},
		},
	},
		p2ptest.Exchange{
			Label: "Corrupt offered hash message",
			Triggers: []p2ptest.Trigger{
				{
					Code: 1,
					Msg: &OfferedHashesMsg{
						HandoverProof: &HandoverProof{
							Handover: &Handover{},
						},
						Hashes: corruptHashes,
						From:   5,
						To:     8,
						Stream: stream,
					},
					Peer: node.ID(),
				},
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	expectedError := errors.New("Message handler error: (msg code 1): error invalid hashes length (len: 40)")
	if err := tester.TestDisconnected(&p2ptest.Disconnect{Peer: node.ID(), Error: expectedError}); err != nil {
		t.Fatal(err)
	}
}

func TestStreamerDownstreamOfferedHashesMsgExchange(t *testing.T) {
	tester, streamer, _, teardown, err := newStreamerTester(t, nil)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	stream := NewStream("foo", "", true)

	var tc *testClient

	streamer.RegisterClientFunc("foo", func(p *Peer, t string, live bool) (Client, error) {
		tc = newTestClient(t)
		return tc, nil
	})

	node := tester.Nodes[0]

	err = streamer.Subscribe(node.ID(), stream, NewRange(5, 8), Top)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "Subscribe message",
		Expects: []p2ptest.Expect{
			{
				Code: 4,
				Msg: &SubscribeMsg{
					Stream:   stream,
					History:  NewRange(5, 8),
					Priority: Top,
				},
				Peer: node.ID(),
			},
		},
	},
		p2ptest.Exchange{
			Label: "WantedHashes message",
			Triggers: []p2ptest.Trigger{
				{
					Code: 1,
					Msg: &OfferedHashesMsg{
						HandoverProof: &HandoverProof{
							Handover: &Handover{},
						},
						Hashes: hashes,
						From:   5,
						To:     8,
						Stream: stream,
					},
					Peer: node.ID(),
				},
			},
			Expects: []p2ptest.Expect{
				{
					Code: 2,
					Msg: &WantedHashesMsg{
						Stream: stream,
						Want:   []byte{5},
						From:   9,
						To:     0,
					},
					Peer: node.ID(),
				},
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	if len(tc.receivedHashes) != 3 {
		t.Fatalf("Expected number of received hashes %v, got %v", 3, len(tc.receivedHashes))
	}

	close(tc.wait0)

	timeout := time.NewTimer(100 * time.Millisecond)
	defer timeout.Stop()

	select {
	case <-tc.batchDone:
		t.Fatal("batch done early")
	case <-timeout.C:
	}

	close(tc.wait2)

	timeout2 := time.NewTimer(10000 * time.Millisecond)
	defer timeout2.Stop()

	select {
	case <-tc.batchDone:
	case <-timeout2.C:
		t.Fatal("timeout waiting batchdone call")
	}

}

func TestStreamerRequestSubscriptionQuitMsgExchange(t *testing.T) {
	tester, streamer, _, teardown, err := newStreamerTester(t, nil)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	streamer.RegisterServerFunc("foo", func(p *Peer, t string, live bool) (Server, error) {
		return newTestServer(t, 10), nil
	})

	node := tester.Nodes[0]

	stream := NewStream("foo", "", true)
	err = streamer.RequestSubscription(node.ID(), stream, NewRange(5, 8), Top)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = tester.TestExchanges(
		p2ptest.Exchange{
			Label: "RequestSubscription message",
			Expects: []p2ptest.Expect{
				{
					Code: 8,
					Msg: &RequestSubscriptionMsg{
						Stream:   stream,
						History:  NewRange(5, 8),
						Priority: Top,
					},
					Peer: node.ID(),
				},
			},
		},
		p2ptest.Exchange{
			Label: "Subscribe message",
			Triggers: []p2ptest.Trigger{
				{
					Code: 4,
					Msg: &SubscribeMsg{
						Stream:   stream,
						History:  NewRange(5, 8),
						Priority: Top,
					},
					Peer: node.ID(),
				},
			},
			Expects: []p2ptest.Expect{
				{
					Code: 1,
					Msg: &OfferedHashesMsg{
						Stream: NewStream("foo", "", false),
						HandoverProof: &HandoverProof{
							Handover: &Handover{},
						},
						Hashes: make([]byte, HashSize),
						From:   6,
						To:     9,
					},
					Peer: node.ID(),
				},
				{
					Code: 1,
					Msg: &OfferedHashesMsg{
						Stream: stream,
						HandoverProof: &HandoverProof{
							Handover: &Handover{},
						},
						From:   11,
						To:     0,
						Hashes: make([]byte, HashSize),
					},
					Peer: node.ID(),
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	err = streamer.Quit(node.ID(), stream)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "Quit message",
		Expects: []p2ptest.Expect{
			{
				Code: 9,
				Msg: &QuitMsg{
					Stream: stream,
				},
				Peer: node.ID(),
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	historyStream := getHistoryStream(stream)

	err = streamer.Quit(node.ID(), historyStream)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "Quit message",
		Expects: []p2ptest.Expect{
			{
				Code: 9,
				Msg: &QuitMsg{
					Stream: historyStream,
				},
				Peer: node.ID(),
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}
}

//testmaxpeerserverswithunsubscribe创建一个注册表，其中
//流服务器的数量，并使用订阅和
//取消订阅，检查取消订阅是否将删除流，
//离开新河流的地方。
func TestMaxPeerServersWithUnsubscribe(t *testing.T) {
	var maxPeerServers = 6
	tester, streamer, _, teardown, err := newStreamerTester(t, &RegistryOptions{
		Retrieval:      RetrievalDisabled,
		Syncing:        SyncingDisabled,
		MaxPeerServers: maxPeerServers,
	})
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	streamer.RegisterServerFunc("foo", func(p *Peer, t string, live bool) (Server, error) {
		return newTestServer(t, 0), nil
	})

	node := tester.Nodes[0]

	for i := 0; i < maxPeerServers+10; i++ {
		stream := NewStream("foo", strconv.Itoa(i), true)

		err = tester.TestExchanges(p2ptest.Exchange{
			Label: "Subscribe message",
			Triggers: []p2ptest.Trigger{
				{
					Code: 4,
					Msg: &SubscribeMsg{
						Stream:   stream,
						Priority: Top,
					},
					Peer: node.ID(),
				},
			},
			Expects: []p2ptest.Expect{
				{
					Code: 1,
					Msg: &OfferedHashesMsg{
						Stream: stream,
						HandoverProof: &HandoverProof{
							Handover: &Handover{},
						},
						Hashes: make([]byte, HashSize),
						From:   1,
						To:     0,
					},
					Peer: node.ID(),
				},
			},
		})

		if err != nil {
			t.Fatal(err)
		}

		err = tester.TestExchanges(p2ptest.Exchange{
			Label: "unsubscribe message",
			Triggers: []p2ptest.Trigger{
				{
					Code: 0,
					Msg: &UnsubscribeMsg{
						Stream: stream,
					},
					Peer: node.ID(),
				},
			},
		})

		if err != nil {
			t.Fatal(err)
		}
	}
}

//testmaxpeerserverswithountsubscribe创建一个注册表，其中
//流服务器的数量，并执行订阅以检测订阅
//错误消息交换。
func TestMaxPeerServersWithoutUnsubscribe(t *testing.T) {
	var maxPeerServers = 6
	tester, streamer, _, teardown, err := newStreamerTester(t, &RegistryOptions{
		MaxPeerServers: maxPeerServers,
	})
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	streamer.RegisterServerFunc("foo", func(p *Peer, t string, live bool) (Server, error) {
		return newTestServer(t, 0), nil
	})

	node := tester.Nodes[0]

	for i := 0; i < maxPeerServers+10; i++ {
		stream := NewStream("foo", strconv.Itoa(i), true)

		if i >= maxPeerServers {
			err = tester.TestExchanges(p2ptest.Exchange{
				Label: "Subscribe message",
				Triggers: []p2ptest.Trigger{
					{
						Code: 4,
						Msg: &SubscribeMsg{
							Stream:   stream,
							Priority: Top,
						},
						Peer: node.ID(),
					},
				},
				Expects: []p2ptest.Expect{
					{
						Code: 7,
						Msg: &SubscribeErrorMsg{
							Error: ErrMaxPeerServers.Error(),
						},
						Peer: node.ID(),
					},
				},
			})

			if err != nil {
				t.Fatal(err)
			}
			continue
		}

		err = tester.TestExchanges(p2ptest.Exchange{
			Label: "Subscribe message",
			Triggers: []p2ptest.Trigger{
				{
					Code: 4,
					Msg: &SubscribeMsg{
						Stream:   stream,
						Priority: Top,
					},
					Peer: node.ID(),
				},
			},
			Expects: []p2ptest.Expect{
				{
					Code: 1,
					Msg: &OfferedHashesMsg{
						Stream: stream,
						HandoverProof: &HandoverProof{
							Handover: &Handover{},
						},
						Hashes: make([]byte, HashSize),
						From:   1,
						To:     0,
					},
					Peer: node.ID(),
				},
			},
		})

		if err != nil {
			t.Fatal(err)
		}
	}
}

//TestHasPriceImplementation将检查注册表是否具有
//`price`接口实现
func TestHasPriceImplementation(t *testing.T) {
	_, r, _, teardown, err := newStreamerTester(t, &RegistryOptions{
		Retrieval: RetrievalDisabled,
		Syncing:   SyncingDisabled,
	})
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	if r.prices == nil {
		t.Fatal("No prices implementation available for the stream protocol")
	}

	pricesInstance, ok := r.prices.(*StreamerPrices)
	if !ok {
		t.Fatal("`Registry` does not have the expected Prices instance")
	}
	price := pricesInstance.Price(&ChunkDeliveryMsgRetrieval{})
	if price == nil || price.Value == 0 || price.Value != pricesInstance.getChunkDeliveryMsgRetrievalPrice() {
		t.Fatal("No prices set for chunk delivery msg")
	}

	price = pricesInstance.Price(&RetrieveRequestMsg{})
	if price == nil || price.Value == 0 || price.Value != pricesInstance.getRetrieveRequestMsgPrice() {
		t.Fatal("No prices set for chunk delivery msg")
	}
}

/*
TestRequestPeerSubscriptions是流的请求同步订阅的单元测试。

测试如下：
 *将每个连接的对等机分配给一个箱映射
  *提前建立一个已知的花环
 *运行eachconn函数，该函数返回假定的订阅容器
 *在地图中存储每个对等机的所有假定箱
 *检查所有对等方是否具有预期订阅

此KAD表及其对等项从network.testKademliacase1复制，
它表示一个边缘情况，但用于测试
同步订阅很好。

此测试中使用的地址作为仿真网络的一部分被发现
在流的高级测试中。它们是随机产生的。

生成的卡德米利亚如下：
=========================================================
12月21日星期五20:02:39 UTC 2018 K∧∧Mli∧Hive:皇后地址：7EFEF1
人口：12（12），MinProxBin大小：2，MinBin大小：2，MaxBin大小：4
000 2 8196 835F 2 8196（0）835F（0）
001 2 2690 28f0 2 2690（0）28f0（0）
002 2 4D72 4A45 2 4D72（0）4A45（0）
003 1 646E 1 646E（0）
004 3 769C 76D1 7656 3 769C（0）76D1（0）7656（0）
===========深度：5=============================
005 1 7A48 1 7A48（0）
006 1 7CBD 1 7CBD（0）
007 0_0
008 0_0
009 0_0
010 0_0
011 0_0
012 0_0
013 0_0
014 0_0
015 0_0
=========================================================
**/

func TestRequestPeerSubscriptions(t *testing.T) {
//透视地址；这是实际的Kademlia节点
	pivotAddr := "7efef1c41d77f843ad167be95f6660567eb8a4a59f39240000cce2e0d65baf8e"

//从给定的卡德米利亚到地址的bin编号的地图
	binMap := make(map[int][]string)
	binMap[0] = []string{
		"835fbbf1d16ba7347b6e2fc552d6e982148d29c624ea20383850df3c810fa8fc",
		"81968a2d8fb39114342ee1da85254ec51e0608d7f0f6997c2a8354c260a71009",
	}
	binMap[1] = []string{
		"28f0bc1b44658548d6e05dd16d4c2fe77f1da5d48b6774bc4263b045725d0c19",
		"2690a910c33ee37b91eb6c4e0731d1d345e2dc3b46d308503a6e85bbc242c69e",
	}
	binMap[2] = []string{
		"4a45f1fc63e1a9cb9dfa44c98da2f3d20c2923e5d75ff60b2db9d1bdb0c54d51",
		"4d72a04ddeb851a68cd197ef9a92a3e2ff01fbbff638e64929dd1a9c2e150112",
	}
	binMap[3] = []string{
		"646e9540c84f6a2f9cf6585d45a4c219573b4fd1b64a3c9a1386fc5cf98c0d4d",
	}
	binMap[4] = []string{
		"7656caccdc79cd8d7ce66d415cc96a718e8271c62fb35746bfc2b49faf3eebf3",
		"76d1e83c71ca246d042e37ff1db181f2776265fbcfdc890ce230bfa617c9c2f0",
		"769ce86aa90b518b7ed382f9fdacfbed93574e18dc98fe6c342e4f9f409c2d5a",
	}
	binMap[5] = []string{
		"7a48f75f8ca60487ae42d6f92b785581b40b91f2da551ae73d5eae46640e02e8",
	}
	binMap[6] = []string{
		"7cbd42350bde8e18ae5b955b5450f8e2cef3419f92fbf5598160c60fd78619f0",
	}

//创建轴的花环
	addr := common.FromHex(pivotAddr)
	k := network.NewKademlia(addr, network.NewKadParams())

//构建同行和卡德米利亚
	for _, binaddrs := range binMap {
		for _, a := range binaddrs {
			addr := common.FromHex(a)
			k.On(network.NewPeer(&network.BzzPeer{BzzAddr: &network.BzzAddr{OAddr: addr}}, k))
		}
	}

//TODO:检查KAD表是否相同
//当前k.string（）打印日期，使其永远不会相同：）
//-->KAD表的JSON表示实现
	log.Debug(k.String())

//模拟一下我们要订阅：只需存储bin编号
	fakeSubscriptions := make(map[string][]int)
//测试完成后，我们需要将subscriptionUnc重置为默认值
	defer func() { subscriptionFunc = doRequestSubscription }()
//定义应该为每个连接运行的函数
//我们不做真正的订阅，只存储箱号
	subscriptionFunc = func(r *Registry, p *network.Peer, bin uint8, subs map[enode.ID]map[Stream]struct{}) bool {
//获取对等体ID
		peerstr := fmt.Sprintf("%x", p.Over())
//为每个对等机创建容器数组
		if _, ok := fakeSubscriptions[peerstr]; !ok {
			fakeSubscriptions[peerstr] = make([]int, 0)
		}
//存储（假）bin订阅
		log.Debug(fmt.Sprintf("Adding fake subscription for peer %s with bin %d", peerstr, bin))
		fakeSubscriptions[peerstr] = append(fakeSubscriptions[peerstr], int(bin))
		return true
	}
//只创建一个简单的注册表对象，以便能够调用…
	r := &Registry{}
	r.requestPeerSubscriptions(k, nil)
//计算Kademlia深度
	kdepth := k.NeighbourhoodDepth()

//现在，检查所有对等方是否都有预期（假）订阅
//迭代bin映射
	for bin, peers := range binMap {
//对于每一个同伴…
		for _, peer := range peers {
//…获取其（假的）订阅
			fakeSubsForPeer := fakeSubscriptions[peer]
//如果同伴的箱子比花环的深度浅…
			if bin < kdepth {
//（重复所有（假）订阅）
				for _, subbin := range fakeSubsForPeer {
//…只应“订阅”对等机的bin
//（因此只有一个订阅）
					if subbin != bin || len(fakeSubsForPeer) != 1 {
						t.Fatalf("Did not get expected subscription for bin < depth; bin of peer %s: %d, subscription: %d", peer, bin, subbin)
					}
				}
} else { //如果同伴的箱子等于或高于卡德米利亚深度…
//（重复所有（假）订阅）
				for i, subbin := range fakeSubsForPeer {
//…从对等机的bin号到k.maxproxydisplay的每个bin都应该是“订阅的”
//当我们从深度开始时，我们可以使用迭代索引来检查
					if subbin != i+kdepth {
						t.Fatalf("Did not get expected subscription for bin > depth; bin of peer %s: %d, subscription: %d", peer, bin, subbin)
					}
//最后一个“订阅”应该是k.maxproxplay
					if i == len(fakeSubsForPeer)-1 && subbin != k.MaxProxDisplay {
						t.Fatalf("Expected last subscription to be: %d, but is: %d", k.MaxProxDisplay, subbin)
					}
				}
			}
		}
	}

//打印一些输出
	for p, subs := range fakeSubscriptions {
		log.Debug(fmt.Sprintf("Peer %s has the following fake subscriptions: ", p))
		for _, bin := range subs {
			log.Debug(fmt.Sprintf("%d,", bin))
		}
	}
}
