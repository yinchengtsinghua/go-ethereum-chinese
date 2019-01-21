
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

package discv5

import (
	"bytes"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var nodeDBKeyTests = []struct {
	id    NodeID
	field string
	key   []byte
}{
	{
		id:    NodeID{},
		field: "version",
key:   []byte{0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e}, //领域
	},
	{
		id:    MustHexID("0x1dd9d65c4552b5eb43d5ad55a2ee3f56c6cbc1c64a5c8d659f51fcd51bace24351232b8d7821617d2b29b54b81cdefb9b3e9c37d7fd5f63270bcc9e1a6f6a439"),
		field: ":discover",
key: []byte{0x6e, 0x3a, //前缀
0x1d, 0xd9, 0xd6, 0x5c, 0x45, 0x52, 0xb5, 0xeb, //节点ID
0x43, 0xd5, 0xad, 0x55, 0xa2, 0xee, 0x3f, 0x56, //
0xc6, 0xcb, 0xc1, 0xc6, 0x4a, 0x5c, 0x8d, 0x65, //
0x9f, 0x51, 0xfc, 0xd5, 0x1b, 0xac, 0xe2, 0x43, //
0x51, 0x23, 0x2b, 0x8d, 0x78, 0x21, 0x61, 0x7d, //
0x2b, 0x29, 0xb5, 0x4b, 0x81, 0xcd, 0xef, 0xb9, //
0xb3, 0xe9, 0xc3, 0x7d, 0x7f, 0xd5, 0xf6, 0x32, //
0x70, 0xbc, 0xc9, 0xe1, 0xa6, 0xf6, 0xa4, 0x39, //
0x3a, 0x64, 0x69, 0x73, 0x63, 0x6f, 0x76, 0x65, 0x72, //领域
		},
	},
}

func TestNodeDBKeys(t *testing.T) {
	for i, tt := range nodeDBKeyTests {
		if key := makeKey(tt.id, tt.field); !bytes.Equal(key, tt.key) {
			t.Errorf("make test %d: key mismatch: have 0x%x, want 0x%x", i, key, tt.key)
		}
		id, field := splitKey(tt.key)
		if !bytes.Equal(id[:], tt.id[:]) {
			t.Errorf("split test %d: id mismatch: have 0x%x, want 0x%x", i, id, tt.id)
		}
		if field != tt.field {
			t.Errorf("split test %d: field mismatch: have 0x%x, want 0x%x", i, field, tt.field)
		}
	}
}

var nodeDBInt64Tests = []struct {
	key   []byte
	value int64
}{
	{key: []byte{0x01}, value: 1},
	{key: []byte{0x02}, value: 2},
	{key: []byte{0x03}, value: 3},
}

func TestNodeDBInt64(t *testing.T) {
	db, _ := newNodeDB("", Version, NodeID{})
	defer db.close()

	tests := nodeDBInt64Tests
	for i := 0; i < len(tests); i++ {
//插入下一个值
		if err := db.storeInt64(tests[i].key, tests[i].value); err != nil {
			t.Errorf("test %d: failed to store value: %v", i, err)
		}
//检查所有现有和不存在的值
		for j := 0; j < len(tests); j++ {
			num := db.fetchInt64(tests[j].key)
			switch {
			case j <= i && num != tests[j].value:
				t.Errorf("test %d, item %d: value mismatch: have %v, want %v", i, j, num, tests[j].value)
			case j > i && num != 0:
				t.Errorf("test %d, item %d: value mismatch: have %v, want %v", i, j, num, 0)
			}
		}
	}
}

func TestNodeDBFetchStore(t *testing.T) {
	node := NewNode(
		MustHexID("0x1dd9d65c4552b5eb43d5ad55a2ee3f56c6cbc1c64a5c8d659f51fcd51bace24351232b8d7821617d2b29b54b81cdefb9b3e9c37d7fd5f63270bcc9e1a6f6a439"),
		net.IP{192, 168, 0, 1},
		30303,
		30303,
	)
	inst := time.Now()
	num := 314

	db, _ := newNodeDB("", Version, NodeID{})
	defer db.close()

//检查节点ping对象上的获取/存储操作
	if stored := db.lastPing(node.ID); stored.Unix() != 0 {
		t.Errorf("ping: non-existing object: %v", stored)
	}
	if err := db.updateLastPing(node.ID, inst); err != nil {
		t.Errorf("ping: failed to update: %v", err)
	}
	if stored := db.lastPing(node.ID); stored.Unix() != inst.Unix() {
		t.Errorf("ping: value mismatch: have %v, want %v", stored, inst)
	}
//检查节点pong对象上的fetch/store操作
	if stored := db.lastPong(node.ID); stored.Unix() != 0 {
		t.Errorf("pong: non-existing object: %v", stored)
	}
	if err := db.updateLastPong(node.ID, inst); err != nil {
		t.Errorf("pong: failed to update: %v", err)
	}
	if stored := db.lastPong(node.ID); stored.Unix() != inst.Unix() {
		t.Errorf("pong: value mismatch: have %v, want %v", stored, inst)
	}
//检查节点findnode失败对象上的提取/存储操作
	if stored := db.findFails(node.ID); stored != 0 {
		t.Errorf("find-node fails: non-existing object: %v", stored)
	}
	if err := db.updateFindFails(node.ID, num); err != nil {
		t.Errorf("find-node fails: failed to update: %v", err)
	}
	if stored := db.findFails(node.ID); stored != num {
		t.Errorf("find-node fails: value mismatch: have %v, want %v", stored, num)
	}
//检查实际节点对象上的获取/存储操作
	if stored := db.node(node.ID); stored != nil {
		t.Errorf("node: non-existing object: %v", stored)
	}
	if err := db.updateNode(node); err != nil {
		t.Errorf("node: failed to update: %v", err)
	}
	if stored := db.node(node.ID); stored == nil {
		t.Errorf("node: not found")
	} else if !reflect.DeepEqual(stored, node) {
		t.Errorf("node: data mismatch: have %v, want %v", stored, node)
	}
}

var nodeDBSeedQueryNodes = []struct {
	node *Node
	pong time.Time
}{
//这个不应该在结果集中，因为它是最后一个
//乒乓球时间过去太长了。
	{
		node: NewNode(
			MustHexID("0x84d9d65c4552b5eb43d5ad55a2ee3f56c6cbc1c64a5c8d659f51fcd51bace24351232b8d7821617d2b29b54b81cdefb9b3e9c37d7fd5f63270bcc9e1a6f6a439"),
			net.IP{127, 0, 0, 3},
			30303,
			30303,
		),
		pong: time.Now().Add(-3 * time.Hour),
	},
//这个不应该出现在结果集中，因为
//node id是本地节点的ID。
	{
		node: NewNode(
			MustHexID("0x57d9d65c4552b5eb43d5ad55a2ee3f56c6cbc1c64a5c8d659f51fcd51bace24351232b8d7821617d2b29b54b81cdefb9b3e9c37d7fd5f63270bcc9e1a6f6a439"),
			net.IP{127, 0, 0, 3},
			30303,
			30303,
		),
		pong: time.Now().Add(-4 * time.Second),
	},

//这些应该在结果集中。
	{
		node: NewNode(
			MustHexID("0x22d9d65c4552b5eb43d5ad55a2ee3f56c6cbc1c64a5c8d659f51fcd51bace24351232b8d7821617d2b29b54b81cdefb9b3e9c37d7fd5f63270bcc9e1a6f6a439"),
			net.IP{127, 0, 0, 1},
			30303,
			30303,
		),
		pong: time.Now().Add(-2 * time.Second),
	},
	{
		node: NewNode(
			MustHexID("0x44d9d65c4552b5eb43d5ad55a2ee3f56c6cbc1c64a5c8d659f51fcd51bace24351232b8d7821617d2b29b54b81cdefb9b3e9c37d7fd5f63270bcc9e1a6f6a439"),
			net.IP{127, 0, 0, 2},
			30303,
			30303,
		),
		pong: time.Now().Add(-3 * time.Second),
	},
	{
		node: NewNode(
			MustHexID("0xe2d9d65c4552b5eb43d5ad55a2ee3f56c6cbc1c64a5c8d659f51fcd51bace24351232b8d7821617d2b29b54b81cdefb9b3e9c37d7fd5f63270bcc9e1a6f6a439"),
			net.IP{127, 0, 0, 3},
			30303,
			30303,
		),
		pong: time.Now().Add(-1 * time.Second),
	},
}

func TestNodeDBSeedQuery(t *testing.T) {
	db, _ := newNodeDB("", Version, nodeDBSeedQueryNodes[1].node.ID)
	defer db.close()

//插入一批节点进行查询
	for i, seed := range nodeDBSeedQueryNodes {
		if err := db.updateNode(seed.node); err != nil {
			t.Fatalf("node %d: failed to insert: %v", i, err)
		}
		if err := db.updateLastPong(seed.node.ID, seed.pong); err != nil {
			t.Fatalf("node %d: failed to insert lastPong: %v", i, err)
		}
	}

//检索整个批并检查重复项
	seeds := db.querySeeds(len(nodeDBSeedQueryNodes)*2, time.Hour)
	have := make(map[NodeID]struct{})
	for _, seed := range seeds {
		have[seed.ID] = struct{}{}
	}
	want := make(map[NodeID]struct{})
	for _, seed := range nodeDBSeedQueryNodes[2:] {
		want[seed.node.ID] = struct{}{}
	}
	if len(seeds) != len(want) {
		t.Errorf("seed count mismatch: have %v, want %v", len(seeds), len(want))
	}
	for id := range have {
		if _, ok := want[id]; !ok {
			t.Errorf("extra seed: %v", id)
		}
	}
	for id := range want {
		if _, ok := have[id]; !ok {
			t.Errorf("missing seed: %v", id)
		}
	}
}

func TestNodeDBPersistency(t *testing.T) {
	root, err := ioutil.TempDir("", "nodedb-")
	if err != nil {
		t.Fatalf("failed to create temporary data folder: %v", err)
	}
	defer os.RemoveAll(root)

	var (
		testKey = []byte("somekey")
		testInt = int64(314)
	)

//创建持久数据库并存储一些值
	db, err := newNodeDB(filepath.Join(root, "database"), Version, NodeID{})
	if err != nil {
		t.Fatalf("failed to create persistent database: %v", err)
	}
	if err := db.storeInt64(testKey, testInt); err != nil {
		t.Fatalf("failed to store value: %v.", err)
	}
	db.close()

//重新打开数据库并检查值
	db, err = newNodeDB(filepath.Join(root, "database"), Version, NodeID{})
	if err != nil {
		t.Fatalf("failed to open persistent database: %v", err)
	}
	if val := db.fetchInt64(testKey); val != testInt {
		t.Fatalf("value mismatch: have %v, want %v", val, testInt)
	}
	db.close()

//更改数据库版本并检查刷新
	db, err = newNodeDB(filepath.Join(root, "database"), Version+1, NodeID{})
	if err != nil {
		t.Fatalf("failed to open persistent database: %v", err)
	}
	if val := db.fetchInt64(testKey); val != 0 {
		t.Fatalf("value mismatch: have %v, want %v", val, 0)
	}
	db.close()
}

var nodeDBExpirationNodes = []struct {
	node *Node
	pong time.Time
	exp  bool
}{
	{
		node: NewNode(
			MustHexID("0x01d9d65c4552b5eb43d5ad55a2ee3f56c6cbc1c64a5c8d659f51fcd51bace24351232b8d7821617d2b29b54b81cdefb9b3e9c37d7fd5f63270bcc9e1a6f6a439"),
			net.IP{127, 0, 0, 1},
			30303,
			30303,
		),
		pong: time.Now().Add(-nodeDBNodeExpiration + time.Minute),
		exp:  false,
	}, {
		node: NewNode(
			MustHexID("0x02d9d65c4552b5eb43d5ad55a2ee3f56c6cbc1c64a5c8d659f51fcd51bace24351232b8d7821617d2b29b54b81cdefb9b3e9c37d7fd5f63270bcc9e1a6f6a439"),
			net.IP{127, 0, 0, 2},
			30303,
			30303,
		),
		pong: time.Now().Add(-nodeDBNodeExpiration - time.Minute),
		exp:  true,
	},
}

func TestNodeDBExpiration(t *testing.T) {
	db, _ := newNodeDB("", Version, NodeID{})
	defer db.close()

//添加所有测试节点并设置它们的最后一次pong时间
	for i, seed := range nodeDBExpirationNodes {
		if err := db.updateNode(seed.node); err != nil {
			t.Fatalf("node %d: failed to insert: %v", i, err)
		}
		if err := db.updateLastPong(seed.node.ID, seed.pong); err != nil {
			t.Fatalf("node %d: failed to update pong: %v", i, err)
		}
	}
//使其中一些过期，并检查其余部分
	if err := db.expireNodes(); err != nil {
		t.Fatalf("failed to expire nodes: %v", err)
	}
	for i, seed := range nodeDBExpirationNodes {
		node := db.node(seed.node.ID)
		if (node == nil && !seed.exp) || (node != nil && seed.exp) {
			t.Errorf("node %d: expiration mismatch: have %v, want %v", i, node, seed.exp)
		}
	}
}

func TestNodeDBSelfExpiration(t *testing.T) {
//在测试中查找不应过期的节点，并将其指定为自己的节点
	var self NodeID
	for _, node := range nodeDBExpirationNodes {
		if !node.exp {
			self = node.node.ID
			break
		}
	}
	db, _ := newNodeDB("", Version, self)
	defer db.close()

//添加所有测试节点并设置它们的最后一次pong时间
	for i, seed := range nodeDBExpirationNodes {
		if err := db.updateNode(seed.node); err != nil {
			t.Fatalf("node %d: failed to insert: %v", i, err)
		}
		if err := db.updateLastPong(seed.node.ID, seed.pong); err != nil {
			t.Fatalf("node %d: failed to update pong: %v", i, err)
		}
	}
//使节点过期并确保自身也已被疏散
	if err := db.expireNodes(); err != nil {
		t.Fatalf("failed to expire nodes: %v", err)
	}
	node := db.node(self)
	if node != nil {
		t.Errorf("self not evacuated")
	}
}
