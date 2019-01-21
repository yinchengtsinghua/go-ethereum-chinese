
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
	"testing"

	p2ptest "github.com/ethereum/go-ethereum/p2p/testing"
)

//此测试检查服务器的默认行为，即
//当它提供检索请求时。
func TestLigthnodeRetrieveRequestWithRetrieve(t *testing.T) {
	registryOptions := &RegistryOptions{
		Retrieval: RetrievalClientOnly,
		Syncing:   SyncingDisabled,
	}
	tester, _, _, teardown, err := newStreamerTester(t, registryOptions)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	node := tester.Nodes[0]

	stream := NewStream(swarmChunkServerStreamName, "", false)

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "SubscribeMsg",
		Triggers: []p2ptest.Trigger{
			{
				Code: 4,
				Msg: &SubscribeMsg{
					Stream: stream,
				},
				Peer: node.ID(),
			},
		},
	})
	if err != nil {
		t.Fatalf("Got %v", err)
	}

	err = tester.TestDisconnected(&p2ptest.Disconnect{Peer: node.ID()})
	if err == nil || err.Error() != "timed out waiting for peers to disconnect" {
		t.Fatalf("Expected no disconnect, got %v", err)
	}
}

//此测试在服务检索时检查服务器的lightnode行为
//请求被禁用
func TestLigthnodeRetrieveRequestWithoutRetrieve(t *testing.T) {
	registryOptions := &RegistryOptions{
		Retrieval: RetrievalDisabled,
		Syncing:   SyncingDisabled,
	}
	tester, _, _, teardown, err := newStreamerTester(t, registryOptions)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	node := tester.Nodes[0]

	stream := NewStream(swarmChunkServerStreamName, "", false)

	err = tester.TestExchanges(
		p2ptest.Exchange{
			Label: "SubscribeMsg",
			Triggers: []p2ptest.Trigger{
				{
					Code: 4,
					Msg: &SubscribeMsg{
						Stream: stream,
					},
					Peer: node.ID(),
				},
			},
			Expects: []p2ptest.Expect{
				{
					Code: 7,
					Msg: &SubscribeErrorMsg{
						Error: "stream RETRIEVE_REQUEST not registered",
					},
					Peer: node.ID(),
				},
			},
		})
	if err != nil {
		t.Fatalf("Got %v", err)
	}
}

//此测试检查服务器的默认行为，即
//启用同步时。
func TestLigthnodeRequestSubscriptionWithSync(t *testing.T) {
	registryOptions := &RegistryOptions{
		Retrieval: RetrievalDisabled,
		Syncing:   SyncingRegisterOnly,
	}
	tester, _, _, teardown, err := newStreamerTester(t, registryOptions)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	node := tester.Nodes[0]

	syncStream := NewStream("SYNC", FormatSyncBinKey(1), false)

	err = tester.TestExchanges(
		p2ptest.Exchange{
			Label: "RequestSubscription",
			Triggers: []p2ptest.Trigger{
				{
					Code: 8,
					Msg: &RequestSubscriptionMsg{
						Stream: syncStream,
					},
					Peer: node.ID(),
				},
			},
			Expects: []p2ptest.Expect{
				{
					Code: 4,
					Msg: &SubscribeMsg{
						Stream: syncStream,
					},
					Peer: node.ID(),
				},
			},
		})

	if err != nil {
		t.Fatalf("Got %v", err)
	}
}

//此测试检查服务器的lightnode行为，即
//当同步被禁用时。
func TestLigthnodeRequestSubscriptionWithoutSync(t *testing.T) {
	registryOptions := &RegistryOptions{
		Retrieval: RetrievalDisabled,
		Syncing:   SyncingDisabled,
	}
	tester, _, _, teardown, err := newStreamerTester(t, registryOptions)
	defer teardown()
	if err != nil {
		t.Fatal(err)
	}

	node := tester.Nodes[0]

	syncStream := NewStream("SYNC", FormatSyncBinKey(1), false)

	err = tester.TestExchanges(p2ptest.Exchange{
		Label: "RequestSubscription",
		Triggers: []p2ptest.Trigger{
			{
				Code: 8,
				Msg: &RequestSubscriptionMsg{
					Stream: syncStream,
				},
				Peer: node.ID(),
			},
		},
		Expects: []p2ptest.Expect{
			{
				Code: 7,
				Msg: &SubscribeErrorMsg{
					Error: "stream SYNC not registered",
				},
				Peer: node.ID(),
			},
		},
	}, p2ptest.Exchange{
		Label: "RequestSubscription",
		Triggers: []p2ptest.Trigger{
			{
				Code: 4,
				Msg: &SubscribeMsg{
					Stream: syncStream,
				},
				Peer: node.ID(),
			},
		},
		Expects: []p2ptest.Expect{
			{
				Code: 7,
				Msg: &SubscribeErrorMsg{
					Error: "stream SYNC not registered",
				},
				Peer: node.ID(),
			},
		},
	})

	if err != nil {
		t.Fatalf("Got %v", err)
	}
}
