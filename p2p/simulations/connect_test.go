
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

package simulations

import (
	"testing"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
)

func newTestNetwork(t *testing.T, nodeCount int) (*Network, []enode.ID) {
	t.Helper()
	adapter := adapters.NewSimAdapter(adapters.Services{
		"noopwoop": func(ctx *adapters.ServiceContext) (node.Service, error) {
			return NewNoopService(nil), nil
		},
	})

//创建网络
	network := NewNetwork(adapter, &NetworkConfig{
		DefaultService: "noopwoop",
	})

//创建和启动节点
	ids := make([]enode.ID, nodeCount)
	for i := range ids {
		conf := adapters.RandomNodeConfig()
		node, err := network.NewNodeWithConfig(conf)
		if err != nil {
			t.Fatalf("error creating node: %s", err)
		}
		if err := network.Start(node.ID()); err != nil {
			t.Fatalf("error starting node: %s", err)
		}
		ids[i] = node.ID()
	}

	if len(network.Conns) > 0 {
		t.Fatal("no connections should exist after just adding nodes")
	}

	return network, ids
}

func TestConnectToLastNode(t *testing.T) {
	net, ids := newTestNetwork(t, 10)
	defer net.Shutdown()

	first := ids[0]
	if err := net.ConnectToLastNode(first); err != nil {
		t.Fatal(err)
	}

	last := ids[len(ids)-1]
	for i, id := range ids {
		if id == first || id == last {
			continue
		}

		if net.GetConn(first, id) != nil {
			t.Errorf("connection must not exist with node(ind: %v, id: %v)", i, id)
		}
	}

	if net.GetConn(first, last) == nil {
		t.Error("first and last node must be connected")
	}
}

func TestConnectToRandomNode(t *testing.T) {
	net, ids := newTestNetwork(t, 10)
	defer net.Shutdown()

	err := net.ConnectToRandomNode(ids[0])
	if err != nil {
		t.Fatal(err)
	}

	var cc int
	for i, a := range ids {
		for _, b := range ids[i:] {
			if net.GetConn(a, b) != nil {
				cc++
			}
		}
	}

	if cc != 1 {
		t.Errorf("expected one connection, got %v", cc)
	}
}

func TestConnectNodesFull(t *testing.T) {
	tests := []struct {
		name      string
		nodeCount int
	}{
		{name: "no node", nodeCount: 0},
		{name: "single node", nodeCount: 1},
		{name: "2 nodes", nodeCount: 2},
		{name: "3 nodes", nodeCount: 3},
		{name: "even number of nodes", nodeCount: 12},
		{name: "odd number of nodes", nodeCount: 13},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			net, ids := newTestNetwork(t, test.nodeCount)
			defer net.Shutdown()

			err := net.ConnectNodesFull(ids)
			if err != nil {
				t.Fatal(err)
			}

			VerifyFull(t, net, ids)
		})
	}
}

func TestConnectNodesChain(t *testing.T) {
	net, ids := newTestNetwork(t, 10)
	defer net.Shutdown()

	err := net.ConnectNodesChain(ids)
	if err != nil {
		t.Fatal(err)
	}

	VerifyChain(t, net, ids)
}

func TestConnectNodesRing(t *testing.T) {
	net, ids := newTestNetwork(t, 10)
	defer net.Shutdown()

	err := net.ConnectNodesRing(ids)
	if err != nil {
		t.Fatal(err)
	}

	VerifyRing(t, net, ids)
}

func TestConnectNodesStar(t *testing.T) {
	net, ids := newTestNetwork(t, 10)
	defer net.Shutdown()

	pivotIndex := 2

	err := net.ConnectNodesStar(ids, ids[pivotIndex])
	if err != nil {
		t.Fatal(err)
	}

	VerifyStar(t, net, ids, pivotIndex)
}
