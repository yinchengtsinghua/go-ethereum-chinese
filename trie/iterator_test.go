
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2014 Go Ethereum作者
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

package trie

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

func TestIterator(t *testing.T) {
	trie := newEmpty()
	vals := []struct{ k, v string }{
		{"do", "verb"},
		{"ether", "wookiedoo"},
		{"horse", "stallion"},
		{"shaman", "horse"},
		{"doge", "coin"},
		{"dog", "puppy"},
		{"somethingveryoddindeedthis is", "myothernodedata"},
	}
	all := make(map[string]string)
	for _, val := range vals {
		all[val.k] = val.v
		trie.Update([]byte(val.k), []byte(val.v))
	}
	trie.Commit(nil)

	found := make(map[string]string)
	it := NewIterator(trie.NodeIterator(nil))
	for it.Next() {
		found[string(it.Key)] = string(it.Value)
	}

	for k, v := range all {
		if found[k] != v {
			t.Errorf("iterator value mismatch for %s: got %q want %q", k, found[k], v)
		}
	}
}

type kv struct {
	k, v []byte
	t    bool
}

func TestIteratorLargeData(t *testing.T) {
	trie := newEmpty()
	vals := make(map[string]*kv)

	for i := byte(0); i < 255; i++ {
		value := &kv{common.LeftPadBytes([]byte{i}, 32), []byte{i}, false}
		value2 := &kv{common.LeftPadBytes([]byte{10, i}, 32), []byte{i}, false}
		trie.Update(value.k, value.v)
		trie.Update(value2.k, value2.v)
		vals[string(value.k)] = value
		vals[string(value2.k)] = value2
	}

	it := NewIterator(trie.NodeIterator(nil))
	for it.Next() {
		vals[string(it.Key)].t = true
	}

	var untouched []*kv
	for _, value := range vals {
		if !value.t {
			untouched = append(untouched, value)
		}
	}

	if len(untouched) > 0 {
		t.Errorf("Missed %d nodes", len(untouched))
		for _, value := range untouched {
			t.Error(value)
		}
	}
}

//测试节点迭代器是否确实遍历整个数据库内容。
func TestNodeIteratorCoverage(t *testing.T) {
//创建一些要迭代的任意测试trie
	db, trie, _ := makeTestTrie()

//收集迭代器找到的所有节点散列
	hashes := make(map[common.Hash]struct{})
	for it := trie.NodeIterator(nil); it.Next(true); {
		if it.Hash() != (common.Hash{}) {
			hashes[it.Hash()] = struct{}{}
		}
	}
//交叉检查哈希和数据库本身
	for hash := range hashes {
		if _, err := db.Node(hash); err != nil {
			t.Errorf("failed to retrieve reported node %x: %v", hash, err)
		}
	}
	for hash, obj := range db.dirties {
		if obj != nil && hash != (common.Hash{}) {
			if _, ok := hashes[hash]; !ok {
				t.Errorf("state entry not reported %x", hash)
			}
		}
	}
	for _, key := range db.diskdb.(*ethdb.MemDatabase).Keys() {
		if _, ok := hashes[common.BytesToHash(key)]; !ok {
			t.Errorf("state entry not reported %x", key)
		}
	}
}

type kvs struct{ k, v string }

var testdata1 = []kvs{
	{"barb", "ba"},
	{"bard", "bc"},
	{"bars", "bb"},
	{"bar", "b"},
	{"fab", "z"},
	{"food", "ab"},
	{"foos", "aa"},
	{"foo", "a"},
}

var testdata2 = []kvs{
	{"aardvark", "c"},
	{"bar", "b"},
	{"barb", "bd"},
	{"bars", "be"},
	{"fab", "z"},
	{"foo", "a"},
	{"foos", "aa"},
	{"food", "ab"},
	{"jars", "d"},
}

func TestIteratorSeek(t *testing.T) {
	trie := newEmpty()
	for _, val := range testdata1 {
		trie.Update([]byte(val.k), []byte(val.v))
	}

//寻求中间。
	it := NewIterator(trie.NodeIterator([]byte("fab")))
	if err := checkIteratorOrder(testdata1[4:], it); err != nil {
		t.Fatal(err)
	}

//查找不存在的密钥。
	it = NewIterator(trie.NodeIterator([]byte("barc")))
	if err := checkIteratorOrder(testdata1[1:], it); err != nil {
		t.Fatal(err)
	}

//超越终点。
	it = NewIterator(trie.NodeIterator([]byte("z")))
	if err := checkIteratorOrder(nil, it); err != nil {
		t.Fatal(err)
	}
}

func checkIteratorOrder(want []kvs, it *Iterator) error {
	for it.Next() {
		if len(want) == 0 {
			return fmt.Errorf("didn't expect any more values, got key %q", it.Key)
		}
		if !bytes.Equal(it.Key, []byte(want[0].k)) {
			return fmt.Errorf("wrong key: got %q, want %q", it.Key, want[0].k)
		}
		want = want[1:]
	}
	if len(want) > 0 {
		return fmt.Errorf("iterator ended early, want key %q", want[0])
	}
	return nil
}

func TestDifferenceIterator(t *testing.T) {
	triea := newEmpty()
	for _, val := range testdata1 {
		triea.Update([]byte(val.k), []byte(val.v))
	}
	triea.Commit(nil)

	trieb := newEmpty()
	for _, val := range testdata2 {
		trieb.Update([]byte(val.k), []byte(val.v))
	}
	trieb.Commit(nil)

	found := make(map[string]string)
	di, _ := NewDifferenceIterator(triea.NodeIterator(nil), trieb.NodeIterator(nil))
	it := NewIterator(di)
	for it.Next() {
		found[string(it.Key)] = string(it.Value)
	}

	all := []struct{ k, v string }{
		{"aardvark", "c"},
		{"barb", "bd"},
		{"bars", "be"},
		{"jars", "d"},
	}
	for _, item := range all {
		if found[item.k] != item.v {
			t.Errorf("iterator value mismatch for %s: got %v want %v", item.k, found[item.k], item.v)
		}
	}
	if len(found) != len(all) {
		t.Errorf("iterator count mismatch: got %d values, want %d", len(found), len(all))
	}
}

func TestUnionIterator(t *testing.T) {
	triea := newEmpty()
	for _, val := range testdata1 {
		triea.Update([]byte(val.k), []byte(val.v))
	}
	triea.Commit(nil)

	trieb := newEmpty()
	for _, val := range testdata2 {
		trieb.Update([]byte(val.k), []byte(val.v))
	}
	trieb.Commit(nil)

	di, _ := NewUnionIterator([]NodeIterator{triea.NodeIterator(nil), trieb.NodeIterator(nil)})
	it := NewIterator(di)

	all := []struct{ k, v string }{
		{"aardvark", "c"},
		{"barb", "ba"},
		{"barb", "bd"},
		{"bard", "bc"},
		{"bars", "bb"},
		{"bars", "be"},
		{"bar", "b"},
		{"fab", "z"},
		{"food", "ab"},
		{"foos", "aa"},
		{"foo", "a"},
		{"jars", "d"},
	}

	for i, kv := range all {
		if !it.Next() {
			t.Errorf("Iterator ends prematurely at element %d", i)
		}
		if kv.k != string(it.Key) {
			t.Errorf("iterator value mismatch for element %d: got key %s want %s", i, it.Key, kv.k)
		}
		if kv.v != string(it.Value) {
			t.Errorf("iterator value mismatch for element %d: got value %s want %s", i, it.Value, kv.v)
		}
	}
	if it.Next() {
		t.Errorf("Iterator returned extra values.")
	}
}

func TestIteratorNoDups(t *testing.T) {
	var tr Trie
	for _, val := range testdata1 {
		tr.Update([]byte(val.k), []byte(val.v))
	}
	checkIteratorNoDups(t, tr.NodeIterator(nil), nil)
}

//此测试检查nodeiterator。插入缺少的trie节点后，可以重试next。
func TestIteratorContinueAfterErrorDisk(t *testing.T)    { testIteratorContinueAfterError(t, false) }
func TestIteratorContinueAfterErrorMemonly(t *testing.T) { testIteratorContinueAfterError(t, true) }

func testIteratorContinueAfterError(t *testing.T, memonly bool) {
	diskdb := ethdb.NewMemDatabase()
	triedb := NewDatabase(diskdb)

	tr, _ := New(common.Hash{}, triedb)
	for _, val := range testdata1 {
		tr.Update([]byte(val.k), []byte(val.v))
	}
	tr.Commit(nil)
	if !memonly {
		triedb.Commit(tr.Hash(), true)
	}
	wantNodeCount := checkIteratorNoDups(t, tr.NodeIterator(nil), nil)

	var (
		diskKeys [][]byte
		memKeys  []common.Hash
	)
	if memonly {
		memKeys = triedb.Nodes()
	} else {
		diskKeys = diskdb.Keys()
	}
	for i := 0; i < 20; i++ {
//创建将从数据库加载所有节点的trie。
		tr, _ := New(tr.Hash(), triedb)

//从数据库中删除随机节点。它不能是根节点
//因为那个已经加载了。
		var (
			rkey common.Hash
			rval []byte
			robj *cachedNode
		)
		for {
			if memonly {
				rkey = memKeys[rand.Intn(len(memKeys))]
			} else {
				copy(rkey[:], diskKeys[rand.Intn(len(diskKeys))])
			}
			if rkey != tr.Hash() {
				break
			}
		}
		if memonly {
			robj = triedb.dirties[rkey]
			delete(triedb.dirties, rkey)
		} else {
			rval, _ = diskdb.Get(rkey[:])
			diskdb.Delete(rkey[:])
		}
//迭代直到命中错误。
		seen := make(map[string]bool)
		it := tr.NodeIterator(nil)
		checkIteratorNoDups(t, it, seen)
		missing, ok := it.Error().(*MissingNodeError)
		if !ok || missing.NodeHash != rkey {
			t.Fatal("didn't hit missing node, got", it.Error())
		}

//重新添加节点并继续迭代。
		if memonly {
			triedb.dirties[rkey] = robj
		} else {
			diskdb.Put(rkey[:], rval)
		}
		checkIteratorNoDups(t, it, seen)
		if it.Error() != nil {
			t.Fatal("unexpected error", it.Error())
		}
		if len(seen) != wantNodeCount {
			t.Fatal("wrong node iteration count, got", len(seen), "want", wantNodeCount)
		}
	}
}

//与上面的测试类似，这个测试检查在
//调用next时，某些键前缀的行为正确。接下来的期望是
//应在第一次返回true之前重试查找。
func TestIteratorContinueAfterSeekErrorDisk(t *testing.T) {
	testIteratorContinueAfterSeekError(t, false)
}
func TestIteratorContinueAfterSeekErrorMemonly(t *testing.T) {
	testIteratorContinueAfterSeekError(t, true)
}

func testIteratorContinueAfterSeekError(t *testing.T, memonly bool) {
//将测试trie提交到db，然后删除包含“bars”的节点。
	diskdb := ethdb.NewMemDatabase()
	triedb := NewDatabase(diskdb)

	ctr, _ := New(common.Hash{}, triedb)
	for _, val := range testdata1 {
		ctr.Update([]byte(val.k), []byte(val.v))
	}
	root, _ := ctr.Commit(nil)
	if !memonly {
		triedb.Commit(root, true)
	}
	barNodeHash := common.HexToHash("05041990364eb72fcb1127652ce40d8bab765f2bfe53225b1170d276cc101c2e")
	var (
		barNodeBlob []byte
		barNodeObj  *cachedNode
	)
	if memonly {
		barNodeObj = triedb.dirties[barNodeHash]
		delete(triedb.dirties, barNodeHash)
	} else {
		barNodeBlob, _ = diskdb.Get(barNodeHash[:])
		diskdb.Delete(barNodeHash[:])
	}
//创建一个寻找“条”的新迭代器。搜索无法继续，因为
//缺少节点。
	tr, _ := New(root, triedb)
	it := tr.NodeIterator([]byte("bars"))
	missing, ok := it.Error().(*MissingNodeError)
	if !ok {
		t.Fatal("want MissingNodeError, got", it.Error())
	} else if missing.NodeHash != barNodeHash {
		t.Fatal("wrong node missing")
	}
//重新插入丢失的节点。
	if memonly {
		triedb.dirties[barNodeHash] = barNodeObj
	} else {
		diskdb.Put(barNodeHash[:], barNodeBlob)
	}
//检查迭代是否生成正确的值集。
	if err := checkIteratorOrder(testdata1[2:], NewIterator(it)); err != nil {
		t.Fatal(err)
	}
}

func checkIteratorNoDups(t *testing.T, it NodeIterator, seen map[string]bool) int {
	if seen == nil {
		seen = make(map[string]bool)
	}
	for it.Next(true) {
		if seen[string(it.Path())] {
			t.Fatalf("iterator visited node path %x twice", it.Path())
		}
		seen[string(it.Path())] = true
	}
	return len(seen)
}
