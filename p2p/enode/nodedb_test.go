
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

package enode

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var nodeDBKeyTests = []struct {
	id    ID
	field string
	key   []byte
}{
	{
		id:    ID{},
		field: "version",
key:   []byte{0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e}, //领域
	},
	{
		id:    HexID("51232b8d7821617d2b29b54b81cdefb9b3e9c37d7fd5f63270bcc9e1a6f6a439"),
		field: ":discover",
		key: []byte{
0x6e, 0x3a, //前缀
0x51, 0x23, 0x2b, 0x8d, 0x78, 0x21, 0x61, 0x7d, //节点ID
0x2b, 0x29, 0xb5, 0x4b, 0x81, 0xcd, 0xef, 0xb9, //
0xb3, 0xe9, 0xc3, 0x7d, 0x7f, 0xd5, 0xf6, 0x32, //
0x70, 0xbc, 0xc9, 0xe1, 0xa6, 0xf6, 0xa4, 0x39, //
0x3a, 0x64, 0x69, 0x73, 0x63, 0x6f, 0x76, 0x65, 0x72, //领域
		},
	},
}

func TestDBKeys(t *testing.T) {
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

func TestDBInt64(t *testing.T) {
	db, _ := OpenDB("")
	defer db.Close()

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

func TestDBFetchStore(t *testing.T) {
	node := NewV4(
		hexPubkey("1dd9d65c4552b5eb43d5ad55a2ee3f56c6cbc1c64a5c8d659f51fcd51bace24351232b8d7821617d2b29b54b81cdefb9b3e9c37d7fd5f63270bcc9e1a6f6a439"),
		net.IP{192, 168, 0, 1},
		30303,
		30303,
	)
	inst := time.Now()
	num := 314

	db, _ := OpenDB("")
	defer db.Close()

//检查节点ping对象上的获取/存储操作
	if stored := db.LastPingReceived(node.ID()); stored.Unix() != 0 {
		t.Errorf("ping: non-existing object: %v", stored)
	}
	if err := db.UpdateLastPingReceived(node.ID(), inst); err != nil {
		t.Errorf("ping: failed to update: %v", err)
	}
	if stored := db.LastPingReceived(node.ID()); stored.Unix() != inst.Unix() {
		t.Errorf("ping: value mismatch: have %v, want %v", stored, inst)
	}
//检查节点pong对象上的fetch/store操作
	if stored := db.LastPongReceived(node.ID()); stored.Unix() != 0 {
		t.Errorf("pong: non-existing object: %v", stored)
	}
	if err := db.UpdateLastPongReceived(node.ID(), inst); err != nil {
		t.Errorf("pong: failed to update: %v", err)
	}
	if stored := db.LastPongReceived(node.ID()); stored.Unix() != inst.Unix() {
		t.Errorf("pong: value mismatch: have %v, want %v", stored, inst)
	}
//检查节点findnode失败对象上的提取/存储操作
	if stored := db.FindFails(node.ID()); stored != 0 {
		t.Errorf("find-node fails: non-existing object: %v", stored)
	}
	if err := db.UpdateFindFails(node.ID(), num); err != nil {
		t.Errorf("find-node fails: failed to update: %v", err)
	}
	if stored := db.FindFails(node.ID()); stored != num {
		t.Errorf("find-node fails: value mismatch: have %v, want %v", stored, num)
	}
//检查实际节点对象上的获取/存储操作
	if stored := db.Node(node.ID()); stored != nil {
		t.Errorf("node: non-existing object: %v", stored)
	}
	if err := db.UpdateNode(node); err != nil {
		t.Errorf("node: failed to update: %v", err)
	}
	if stored := db.Node(node.ID()); stored == nil {
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
		node: NewV4(
			hexPubkey("1dd9d65c4552b5eb43d5ad55a2ee3f56c6cbc1c64a5c8d659f51fcd51bace24351232b8d7821617d2b29b54b81cdefb9b3e9c37d7fd5f63270bcc9e1a6f6a439"),
			net.IP{127, 0, 0, 3},
			30303,
			30303,
		),
		pong: time.Now().Add(-3 * time.Hour),
	},
//这个不应该出现在结果集中，因为
//node id是本地节点的ID。
	{
		node: NewV4(
			hexPubkey("ff93ff820abacd4351b0f14e47b324bc82ff014c226f3f66a53535734a3c150e7e38ca03ef0964ba55acddc768f5e99cd59dea95ddd4defbab1339c92fa319b2"),
			net.IP{127, 0, 0, 3},
			30303,
			30303,
		),
		pong: time.Now().Add(-4 * time.Second),
	},

//这些应该在结果集中。
	{
		node: NewV4(
			hexPubkey("c2b5eb3f5dde05f815b63777809ee3e7e0cbb20035a6b00ce327191e6eaa8f26a8d461c9112b7ab94698e7361fa19fd647e603e73239002946d76085b6f928d6"),
			net.IP{127, 0, 0, 1},
			30303,
			30303,
		),
		pong: time.Now().Add(-2 * time.Second),
	},
	{
		node: NewV4(
			hexPubkey("6ca1d400c8ddf8acc94bcb0dd254911ad71a57bed5e0ae5aa205beed59b28c2339908e97990c493499613cff8ecf6c3dc7112a8ead220cdcd00d8847ca3db755"),
			net.IP{127, 0, 0, 2},
			30303,
			30303,
		),
		pong: time.Now().Add(-3 * time.Second),
	},
	{
		node: NewV4(
			hexPubkey("234dc63fe4d131212b38236c4c3411288d7bec61cbf7b120ff12c43dc60c96182882f4291d209db66f8a38e986c9c010ff59231a67f9515c7d1668b86b221a47"),
			net.IP{127, 0, 0, 3},
			30303,
			30303,
		),
		pong: time.Now().Add(-1 * time.Second),
	},
	{
		node: NewV4(
			hexPubkey("c013a50b4d1ebce5c377d8af8cb7114fd933ffc9627f96ad56d90fef5b7253ec736fd07ef9a81dc2955a997e54b7bf50afd0aa9f110595e2bec5bb7ce1657004"),
			net.IP{127, 0, 0, 3},
			30303,
			30303,
		),
		pong: time.Now().Add(-2 * time.Second),
	},
	{
		node: NewV4(
			hexPubkey("f141087e3e08af1aeec261ff75f48b5b1637f594ea9ad670e50051646b0416daa3b134c28788cbe98af26992a47652889cd8577ccc108ac02c6a664db2dc1283"),
			net.IP{127, 0, 0, 3},
			30303,
			30303,
		),
		pong: time.Now().Add(-2 * time.Second),
	},
}

func TestDBSeedQuery(t *testing.T) {
//查询种子使用查找可能找不到所有节点
//每次当数据库很小的时候。多次运行测试
//避免剥落的时间。
	const attempts = 15
	var err error
	for i := 0; i < attempts; i++ {
		if err = testSeedQuery(); err == nil {
			return
		}
	}
	if err != nil {
		t.Errorf("no successful run in %d attempts: %v", attempts, err)
	}
}

func testSeedQuery() error {
	db, _ := OpenDB("")
	defer db.Close()

//插入一批节点进行查询
	for i, seed := range nodeDBSeedQueryNodes {
		if err := db.UpdateNode(seed.node); err != nil {
			return fmt.Errorf("node %d: failed to insert: %v", i, err)
		}
		if err := db.UpdateLastPongReceived(seed.node.ID(), seed.pong); err != nil {
			return fmt.Errorf("node %d: failed to insert bondTime: %v", i, err)
		}
	}

//检索整个批并检查重复项
	seeds := db.QuerySeeds(len(nodeDBSeedQueryNodes)*2, time.Hour)
	have := make(map[ID]struct{})
	for _, seed := range seeds {
		have[seed.ID()] = struct{}{}
	}
	want := make(map[ID]struct{})
	for _, seed := range nodeDBSeedQueryNodes[1:] {
		want[seed.node.ID()] = struct{}{}
	}
	if len(seeds) != len(want) {
		return fmt.Errorf("seed count mismatch: have %v, want %v", len(seeds), len(want))
	}
	for id := range have {
		if _, ok := want[id]; !ok {
			return fmt.Errorf("extra seed: %v", id)
		}
	}
	for id := range want {
		if _, ok := have[id]; !ok {
			return fmt.Errorf("missing seed: %v", id)
		}
	}
	return nil
}

func TestDBPersistency(t *testing.T) {
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
	db, err := OpenDB(filepath.Join(root, "database"))
	if err != nil {
		t.Fatalf("failed to create persistent database: %v", err)
	}
	if err := db.storeInt64(testKey, testInt); err != nil {
		t.Fatalf("failed to store value: %v.", err)
	}
	db.Close()

//重新打开数据库并检查值
	db, err = OpenDB(filepath.Join(root, "database"))
	if err != nil {
		t.Fatalf("failed to open persistent database: %v", err)
	}
	if val := db.fetchInt64(testKey); val != testInt {
		t.Fatalf("value mismatch: have %v, want %v", val, testInt)
	}
	db.Close()
}

var nodeDBExpirationNodes = []struct {
	node *Node
	pong time.Time
	exp  bool
}{
	{
		node: NewV4(
			hexPubkey("8d110e2ed4b446d9b5fb50f117e5f37fb7597af455e1dab0e6f045a6eeaa786a6781141659020d38bdc5e698ed3d4d2bafa8b5061810dfa63e8ac038db2e9b67"),
			net.IP{127, 0, 0, 1},
			30303,
			30303,
		),
		pong: time.Now().Add(-dbNodeExpiration + time.Minute),
		exp:  false,
	}, {
		node: NewV4(
			hexPubkey("913a205579c32425b220dfba999d215066e5bdbf900226b11da1907eae5e93eb40616d47412cf819664e9eacbdfcca6b0c6e07e09847a38472d4be46ab0c3672"),
			net.IP{127, 0, 0, 2},
			30303,
			30303,
		),
		pong: time.Now().Add(-dbNodeExpiration - time.Minute),
		exp:  true,
	},
}

func TestDBExpiration(t *testing.T) {
	db, _ := OpenDB("")
	defer db.Close()

//添加所有测试节点并设置它们的最后一次pong时间
	for i, seed := range nodeDBExpirationNodes {
		if err := db.UpdateNode(seed.node); err != nil {
			t.Fatalf("node %d: failed to insert: %v", i, err)
		}
		if err := db.UpdateLastPongReceived(seed.node.ID(), seed.pong); err != nil {
			t.Fatalf("node %d: failed to update bondTime: %v", i, err)
		}
	}
//使其中一些过期，并检查其余部分
	if err := db.expireNodes(); err != nil {
		t.Fatalf("failed to expire nodes: %v", err)
	}
	for i, seed := range nodeDBExpirationNodes {
		node := db.Node(seed.node.ID())
		if (node == nil && !seed.exp) || (node != nil && seed.exp) {
			t.Errorf("node %d: expiration mismatch: have %v, want %v", i, node, seed.exp)
		}
	}
}
