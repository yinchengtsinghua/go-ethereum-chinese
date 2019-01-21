
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
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
)

//TestServiceBucket使用子测试测试所有Bucket功能。
//它通过向两个节点的bucket中添加项来构造两个节点的模拟。
//在servicefunc构造函数中，然后通过setnodeitem。测试upnodesitems
//通过停止一个节点并验证其项的可用性来完成。
func TestServiceBucket(t *testing.T) {
	testKey := "Key"
	testValue := "Value"

	sim := New(map[string]ServiceFunc{
		"noop": func(ctx *adapters.ServiceContext, b *sync.Map) (node.Service, func(), error) {
			b.Store(testKey, testValue+ctx.Config.ID.String())
			return newNoopService(), nil, nil
		},
	})
	defer sim.Close()

	id1, err := sim.AddNode()
	if err != nil {
		t.Fatal(err)
	}

	id2, err := sim.AddNode()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("ServiceFunc bucket Store", func(t *testing.T) {
		v, ok := sim.NodeItem(id1, testKey)
		if !ok {
			t.Fatal("bucket item not found")
		}
		s, ok := v.(string)
		if !ok {
			t.Fatal("bucket item value is not string")
		}
		if s != testValue+id1.String() {
			t.Fatalf("expected %q, got %q", testValue+id1.String(), s)
		}

		v, ok = sim.NodeItem(id2, testKey)
		if !ok {
			t.Fatal("bucket item not found")
		}
		s, ok = v.(string)
		if !ok {
			t.Fatal("bucket item value is not string")
		}
		if s != testValue+id2.String() {
			t.Fatalf("expected %q, got %q", testValue+id2.String(), s)
		}
	})

	customKey := "anotherKey"
	customValue := "anotherValue"

	t.Run("SetNodeItem", func(t *testing.T) {
		sim.SetNodeItem(id1, customKey, customValue)

		v, ok := sim.NodeItem(id1, customKey)
		if !ok {
			t.Fatal("bucket item not found")
		}
		s, ok := v.(string)
		if !ok {
			t.Fatal("bucket item value is not string")
		}
		if s != customValue {
			t.Fatalf("expected %q, got %q", customValue, s)
		}

		_, ok = sim.NodeItem(id2, customKey)
		if ok {
			t.Fatal("bucket item should not be found")
		}
	})

	if err := sim.StopNode(id2); err != nil {
		t.Fatal(err)
	}

	t.Run("UpNodesItems", func(t *testing.T) {
		items := sim.UpNodesItems(testKey)

		v, ok := items[id1]
		if !ok {
			t.Errorf("node 1 item not found")
		}
		s, ok := v.(string)
		if !ok {
			t.Fatal("node 1 item value is not string")
		}
		if s != testValue+id1.String() {
			t.Fatalf("expected %q, got %q", testValue+id1.String(), s)
		}

		_, ok = items[id2]
		if ok {
			t.Errorf("node 2 item should not be found")
		}
	})

	t.Run("NodeItems", func(t *testing.T) {
		items := sim.NodesItems(testKey)

		v, ok := items[id1]
		if !ok {
			t.Errorf("node 1 item not found")
		}
		s, ok := v.(string)
		if !ok {
			t.Fatal("node 1 item value is not string")
		}
		if s != testValue+id1.String() {
			t.Fatalf("expected %q, got %q", testValue+id1.String(), s)
		}

		v, ok = items[id2]
		if !ok {
			t.Errorf("node 2 item not found")
		}
		s, ok = v.(string)
		if !ok {
			t.Fatal("node 1 item value is not string")
		}
		if s != testValue+id2.String() {
			t.Fatalf("expected %q, got %q", testValue+id2.String(), s)
		}
	})
}
