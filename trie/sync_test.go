
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

package trie

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

//maketesttrie创建一个样本test trie来测试节点重建。
func makeTestTrie() (*Database, *Trie, map[string][]byte) {
//创建空的trie
	triedb := NewDatabase(ethdb.NewMemDatabase())
	trie, _ := New(common.Hash{}, triedb)

//用任意数据填充它
	content := make(map[string][]byte)
	for i := byte(0); i < 255; i++ {
//在多个键下映射相同的数据
		key, val := common.LeftPadBytes([]byte{1, i}, 32), []byte{i}
		content[string(key)] = val
		trie.Update(key, val)

		key, val = common.LeftPadBytes([]byte{2, i}, 32), []byte{i}
		content[string(key)] = val
		trie.Update(key, val)

//添加一些其他数据来填充trie
		for j := byte(3); j < 13; j++ {
			key, val = common.LeftPadBytes([]byte{j, i}, 32), []byte{j, i}
			content[string(key)] = val
			trie.Update(key, val)
		}
	}
	trie.Commit(nil)

//返回生成的trie
	return triedb, trie, content
}

//checktrieContents使用预期数据交叉引用重构的trie
//内容图。
func checkTrieContents(t *testing.T, db *Database, root []byte, content map[string][]byte) {
//检查根可用性和trie内容
	trie, err := New(common.BytesToHash(root), db)
	if err != nil {
		t.Fatalf("failed to create trie at %x: %v", root, err)
	}
	if err := checkTrieConsistency(db, common.BytesToHash(root)); err != nil {
		t.Fatalf("inconsistent trie at %x: %v", root, err)
	}
	for key, val := range content {
		if have := trie.Get([]byte(key)); !bytes.Equal(have, val) {
			t.Errorf("entry %x: content mismatch: have %x, want %x", key, have, val)
		}
	}
}

//checktrieconsibility检查trie中的所有节点是否确实存在。
func checkTrieConsistency(db *Database, root common.Hash) error {
//创建并迭代子节点中的trie
	trie, err := New(root, db)
	if err != nil {
return nil //认为不存在的状态是一致的
	}
	it := trie.NodeIterator(nil)
	for it.Next(true) {
	}
	return it.Error()
}

//测试空的trie没有计划同步。
func TestEmptySync(t *testing.T) {
	dbA := NewDatabase(ethdb.NewMemDatabase())
	dbB := NewDatabase(ethdb.NewMemDatabase())
	emptyA, _ := New(common.Hash{}, dbA)
	emptyB, _ := New(emptyRoot, dbB)

	for i, trie := range []*Trie{emptyA, emptyB} {
		if req := NewSync(trie.Hash(), ethdb.NewMemDatabase(), nil).Missing(1); len(req) != 0 {
			t.Errorf("test %d: content requested for empty trie: %v", i, req)
		}
	}
}

//测试给定根哈希，trie可以在单个线程上迭代同步，
//请求检索任务并一次性返回所有任务。
func TestIterativeSyncIndividual(t *testing.T) { testIterativeSync(t, 1) }
func TestIterativeSyncBatched(t *testing.T)    { testIterativeSync(t, 100) }

func testIterativeSync(t *testing.T, batch int) {
//创建要复制的随机trie
	srcDb, srcTrie, srcData := makeTestTrie()

//创建目标trie并与调度程序同步
	diskdb := ethdb.NewMemDatabase()
	triedb := NewDatabase(diskdb)
	sched := NewSync(srcTrie.Hash(), diskdb, nil)

	queue := append([]common.Hash{}, sched.Missing(batch)...)
	for len(queue) > 0 {
		results := make([]SyncResult, len(queue))
		for i, hash := range queue {
			data, err := srcDb.Node(hash)
			if err != nil {
				t.Fatalf("failed to retrieve node data for %x: %v", hash, err)
			}
			results[i] = SyncResult{hash, data}
		}
		if _, index, err := sched.Process(results); err != nil {
			t.Fatalf("failed to process result #%d: %v", index, err)
		}
		if index, err := sched.Commit(diskdb); err != nil {
			t.Fatalf("failed to commit data #%d: %v", index, err)
		}
		queue = append(queue[:0], sched.Missing(batch)...)
	}
//交叉检查两次尝试是否同步
	checkTrieContents(t, triedb, srcTrie.Root(), srcData)
}

//测试trie调度程序是否可以正确地重建状态，即使只有
//返回部分结果，其他结果只在稍后发送。
func TestIterativeDelayedSync(t *testing.T) {
//创建要复制的随机trie
	srcDb, srcTrie, srcData := makeTestTrie()

//创建目标trie并与调度程序同步
	diskdb := ethdb.NewMemDatabase()
	triedb := NewDatabase(diskdb)
	sched := NewSync(srcTrie.Hash(), diskdb, nil)

	queue := append([]common.Hash{}, sched.Missing(10000)...)
	for len(queue) > 0 {
//只同步一半的计划节点
		results := make([]SyncResult, len(queue)/2+1)
		for i, hash := range queue[:len(results)] {
			data, err := srcDb.Node(hash)
			if err != nil {
				t.Fatalf("failed to retrieve node data for %x: %v", hash, err)
			}
			results[i] = SyncResult{hash, data}
		}
		if _, index, err := sched.Process(results); err != nil {
			t.Fatalf("failed to process result #%d: %v", index, err)
		}
		if index, err := sched.Commit(diskdb); err != nil {
			t.Fatalf("failed to commit data #%d: %v", index, err)
		}
		queue = append(queue[len(results):], sched.Missing(10000)...)
	}
//交叉检查两次尝试是否同步
	checkTrieContents(t, triedb, srcTrie.Root(), srcData)
}

//测试给定根哈希，trie可以在单个线程上迭代同步，
//请求检索任务并一次性返回所有任务，但是在
//随机顺序。
func TestIterativeRandomSyncIndividual(t *testing.T) { testIterativeRandomSync(t, 1) }
func TestIterativeRandomSyncBatched(t *testing.T)    { testIterativeRandomSync(t, 100) }

func testIterativeRandomSync(t *testing.T, batch int) {
//创建要复制的随机trie
	srcDb, srcTrie, srcData := makeTestTrie()

//创建目标trie并与调度程序同步
	diskdb := ethdb.NewMemDatabase()
	triedb := NewDatabase(diskdb)
	sched := NewSync(srcTrie.Hash(), diskdb, nil)

	queue := make(map[common.Hash]struct{})
	for _, hash := range sched.Missing(batch) {
		queue[hash] = struct{}{}
	}
	for len(queue) > 0 {
//以随机顺序获取所有排队的节点
		results := make([]SyncResult, 0, len(queue))
		for hash := range queue {
			data, err := srcDb.Node(hash)
			if err != nil {
				t.Fatalf("failed to retrieve node data for %x: %v", hash, err)
			}
			results = append(results, SyncResult{hash, data})
		}
//将检索到的结果反馈并将新任务排队
		if _, index, err := sched.Process(results); err != nil {
			t.Fatalf("failed to process result #%d: %v", index, err)
		}
		if index, err := sched.Commit(diskdb); err != nil {
			t.Fatalf("failed to commit data #%d: %v", index, err)
		}
		queue = make(map[common.Hash]struct{})
		for _, hash := range sched.Missing(batch) {
			queue[hash] = struct{}{}
		}
	}
//交叉检查两次尝试是否同步
	checkTrieContents(t, triedb, srcTrie.Root(), srcData)
}

//测试trie调度程序是否可以正确地重建状态，即使只有
//部分结果会被返回（甚至是随机返回的结果），其他结果只会在稍后发送。
func TestIterativeRandomDelayedSync(t *testing.T) {
//创建要复制的随机trie
	srcDb, srcTrie, srcData := makeTestTrie()

//创建目标trie并与调度程序同步
	diskdb := ethdb.NewMemDatabase()
	triedb := NewDatabase(diskdb)
	sched := NewSync(srcTrie.Hash(), diskdb, nil)

	queue := make(map[common.Hash]struct{})
	for _, hash := range sched.Missing(10000) {
		queue[hash] = struct{}{}
	}
	for len(queue) > 0 {
//只同步一半的计划节点，甚至是随机顺序的节点
		results := make([]SyncResult, 0, len(queue)/2+1)
		for hash := range queue {
			data, err := srcDb.Node(hash)
			if err != nil {
				t.Fatalf("failed to retrieve node data for %x: %v", hash, err)
			}
			results = append(results, SyncResult{hash, data})

			if len(results) >= cap(results) {
				break
			}
		}
//将检索到的结果反馈并将新任务排队
		if _, index, err := sched.Process(results); err != nil {
			t.Fatalf("failed to process result #%d: %v", index, err)
		}
		if index, err := sched.Commit(diskdb); err != nil {
			t.Fatalf("failed to commit data #%d: %v", index, err)
		}
		for _, result := range results {
			delete(queue, result.Hash)
		}
		for _, hash := range sched.Missing(10000) {
			queue[hash] = struct{}{}
		}
	}
//交叉检查两次尝试是否同步
	checkTrieContents(t, triedb, srcTrie.Root(), srcData)
}

//测试trie-sync不会多次请求节点，即使它们
//有这样的证明人。
func TestDuplicateAvoidanceSync(t *testing.T) {
//创建要复制的随机trie
	srcDb, srcTrie, srcData := makeTestTrie()

//创建目标trie并与调度程序同步
	diskdb := ethdb.NewMemDatabase()
	triedb := NewDatabase(diskdb)
	sched := NewSync(srcTrie.Hash(), diskdb, nil)

	queue := append([]common.Hash{}, sched.Missing(0)...)
	requested := make(map[common.Hash]struct{})

	for len(queue) > 0 {
		results := make([]SyncResult, len(queue))
		for i, hash := range queue {
			data, err := srcDb.Node(hash)
			if err != nil {
				t.Fatalf("failed to retrieve node data for %x: %v", hash, err)
			}
			if _, ok := requested[hash]; ok {
				t.Errorf("hash %x already requested once", hash)
			}
			requested[hash] = struct{}{}

			results[i] = SyncResult{hash, data}
		}
		if _, index, err := sched.Process(results); err != nil {
			t.Fatalf("failed to process result #%d: %v", index, err)
		}
		if index, err := sched.Commit(diskdb); err != nil {
			t.Fatalf("failed to commit data #%d: %v", index, err)
		}
		queue = append(queue[:0], sched.Missing(0)...)
	}
//交叉检查两次尝试是否同步
	checkTrieContents(t, triedb, srcTrie.Root(), srcData)
}

//测试在同步过程中的任何时间点，只有完整的子尝试处于
//数据库。
func TestIncompleteSync(t *testing.T) {
//创建要复制的随机trie
	srcDb, srcTrie, _ := makeTestTrie()

//创建目标trie并与调度程序同步
	diskdb := ethdb.NewMemDatabase()
	triedb := NewDatabase(diskdb)
	sched := NewSync(srcTrie.Hash(), diskdb, nil)

	added := []common.Hash{}
	queue := append([]common.Hash{}, sched.Missing(1)...)
	for len(queue) > 0 {
//获取一批trie节点
		results := make([]SyncResult, len(queue))
		for i, hash := range queue {
			data, err := srcDb.Node(hash)
			if err != nil {
				t.Fatalf("failed to retrieve node data for %x: %v", hash, err)
			}
			results[i] = SyncResult{hash, data}
		}
//处理每个trie节点
		if _, index, err := sched.Process(results); err != nil {
			t.Fatalf("failed to process result #%d: %v", index, err)
		}
		if index, err := sched.Commit(diskdb); err != nil {
			t.Fatalf("failed to commit data #%d: %v", index, err)
		}
		for _, result := range results {
			added = append(added, result.Hash)
		}
//检查同步trie中的所有已知子尝试是否完成
		for _, root := range added {
			if err := checkTrieConsistency(triedb, root); err != nil {
				t.Fatalf("trie inconsistent: %v", err)
			}
		}
//获取要检索的下一批
		queue = append(queue[:0], sched.Missing(1)...)
	}
//健全性检查是否检测到从数据库中删除任何节点
	for _, node := range added[1:] {
		key := node.Bytes()
		value, _ := diskdb.Get(key)

		diskdb.Delete(key)
		if err := checkTrieConsistency(triedb, added[0]); err == nil {
			t.Fatalf("trie inconsistency not caught, missing: %x", key)
		}
		diskdb.Put(key, value)
	}
}
