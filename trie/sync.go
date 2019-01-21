
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
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/prque"
	"github.com/ethereum/go-ethereum/ethdb"
)

//当请求trie-sync处理
//它没有请求的节点。
var ErrNotRequested = errors.New("not requested")

//当请求trie-sync处理
//它以前已经处理过的节点。
var ErrAlreadyProcessed = errors.New("already processed")

//请求表示预定的或已在飞行中的状态检索请求。
type request struct {
hash common.Hash //要检索的节点数据内容的哈希
data []byte      //节点的数据内容，缓存到所有子树完成
raw  bool        //这是原始条目（代码）还是trie节点

parents []*request //引用此条目的父状态节点（完成时通知所有节点）
depth   int        //Trie中的深度级别节点的位置可优先考虑DFS
deps    int        //允许提交此节点之前的依赖项数

callback LeafCallback //调用此分支上的叶节点
}

//SyncResult是一个简单的列表，用于返回丢失的节点及其请求
//散列。
type SyncResult struct {
Hash common.Hash //原始未知trie节点的哈希
Data []byte      //检索节点的数据内容
}

//SyncMembatch是成功下载但尚未下载的内存缓冲区。
//保留的数据项。
type syncMemBatch struct {
batch map[common.Hash][]byte //内存中最近完成的项的membatch
order []common.Hash          //完成订单以防止无序数据丢失
}

//newsyncmembatch为尚未持久化的trie节点分配新的内存缓冲区。
func newSyncMemBatch() *syncMemBatch {
	return &syncMemBatch{
		batch: make(map[common.Hash][]byte),
		order: make([]common.Hash, 0, 256),
	}
}

//同步是主要的状态三同步调度程序，它提供了
//要检索的trie哈希未知，接受与所述哈希关联的节点数据
//一步一步地重建Trie，直到全部完成。
type Sync struct {
database DatabaseReader           //用于检查现有条目的持久数据库
membatch *syncMemBatch            //内存缓冲区以避免频繁的数据库写入
requests map[common.Hash]*request //与密钥哈希相关的挂起请求
queue    *prque.Prque             //具有挂起请求的优先级队列
}

//newsync创建一个新的trie数据下载调度程序。
func NewSync(root common.Hash, database DatabaseReader, callback LeafCallback) *Sync {
	ts := &Sync{
		database: database,
		membatch: newSyncMemBatch(),
		requests: make(map[common.Hash]*request),
		queue:    prque.New(nil),
	}
	ts.AddSubTrie(root, 0, common.Hash{}, callback)
	return ts
}

//addsubrie将一个新的trie注册到同步代码，根位于指定的父级。
func (s *Sync) AddSubTrie(root common.Hash, depth int, parent common.Hash, callback LeafCallback) {
//如果Trie为空或已知，则短路
	if root == emptyRoot {
		return
	}
	if _, ok := s.membatch.batch[root]; ok {
		return
	}
	key := root.Bytes()
	blob, _ := s.database.Get(key)
	if local, err := decodeNode(key, blob, 0); local != nil && err == nil {
		return
	}
//组装新的Sub-Trie同步请求
	req := &request{
		hash:     root,
		depth:    depth,
		callback: callback,
	}
//如果此子目录有指定的父目录，请将它们链接在一起
	if parent != (common.Hash{}) {
		ancestor := s.requests[parent]
		if ancestor == nil {
			panic(fmt.Sprintf("sub-trie ancestor not found: %x", parent))
		}
		ancestor.deps++
		req.parents = append(req.parents, ancestor)
	}
	s.schedule(req)
}

//addrawentry计划直接检索不应
//解释为trie节点，但被接受并存储到数据库中
//事实也是如此。此方法的目标是支持各种状态元数据检索（例如
//合同代码）。
func (s *Sync) AddRawEntry(hash common.Hash, depth int, parent common.Hash) {
//如果条目为空或已知，则短路
	if hash == emptyState {
		return
	}
	if _, ok := s.membatch.batch[hash]; ok {
		return
	}
	if ok, _ := s.database.Has(hash.Bytes()); ok {
		return
	}
//组装新的Sub-Trie同步请求
	req := &request{
		hash:  hash,
		raw:   true,
		depth: depth,
	}
//如果此子目录有指定的父目录，请将它们链接在一起
	if parent != (common.Hash{}) {
		ancestor := s.requests[parent]
		if ancestor == nil {
			panic(fmt.Sprintf("raw-entry ancestor not found: %x", parent))
		}
		ancestor.deps++
		req.parents = append(req.parents, ancestor)
	}
	s.schedule(req)
}

//Missing从trie中检索已知的丢失节点以进行检索。
func (s *Sync) Missing(max int) []common.Hash {
	requests := []common.Hash{}
	for !s.queue.Empty() && (max == 0 || len(requests) < max) {
		requests = append(requests, s.queue.PopItem().(common.Hash))
	}
	return requests
}

//进程注入一批检索到的trie节点数据，如果有什么返回
//已提交到数据库，如果处理
//它失败了。
func (s *Sync) Process(results []SyncResult) (bool, int, error) {
	committed := false

	for i, item := range results {
//如果该项目没有被要求，请退出。
		request := s.requests[item.Hash]
		if request == nil {
			return committed, i, ErrNotRequested
		}
		if request.data != nil {
			return committed, i, ErrAlreadyProcessed
		}
//如果该项是原始输入请求，则直接提交
		if request.raw {
			request.data = item.Data
			s.commit(request)
			committed = true
			continue
		}
//解码节点数据内容并更新请求
		node, err := decodeNode(item.Hash[:], item.Data, 0)
		if err != nil {
			return committed, i, err
		}
		request.data = item.Data

//为所有子节点创建和调度请求
		requests, err := s.children(request, node)
		if err != nil {
			return committed, i, err
		}
		if len(requests) == 0 && request.deps == 0 {
			s.commit(request)
			committed = true
			continue
		}
		request.deps += len(requests)
		for _, child := range requests {
			s.schedule(child)
		}
	}
	return committed, 0, nil
}

//commit将存储在内部membatch中的数据刷新为persistent
//存储，返回写入的项目数和发生的任何错误。
func (s *Sync) Commit(dbw ethdb.Putter) (int, error) {
//将membatch转储到数据库dbw中
	for i, key := range s.membatch.order {
		if err := dbw.Put(key[:], s.membatch.batch[key]); err != nil {
			return i, err
		}
	}
	written := len(s.membatch.order)

//删除membatch数据并返回
	s.membatch = newSyncMemBatch()
	return written, nil
}

//Pending返回当前等待下载的状态条目数。
func (s *Sync) Pending() int {
	return len(s.requests)
}

//计划将新的状态检索请求插入提取队列。如果有
//已经是此节点的挂起请求，新请求将被丢弃。
//只有一个父引用添加到旧的引用中。
func (s *Sync) schedule(req *request) {
//如果我们已经请求这个节点，添加一个新的引用并停止
	if old, ok := s.requests[req.hash]; ok {
		old.parents = append(old.parents, req.parents...)
		return
	}
//为以后的检索安排请求
	s.queue.Push(req.hash, int64(req.depth))
	s.requests[req.hash] = req
}

//children为将来检索一个state trie项中所有丢失的子项
//检索调度。
func (s *Sync) children(req *request, object node) ([]*request, error) {
//收集节点的所有子节点，无论是否已知，都不相关
	type child struct {
		node  node
		depth int
	}
	children := []child{}

	switch node := (object).(type) {
	case *shortNode:
		children = []child{{
			node:  node.Val,
			depth: req.depth + len(node.Key),
		}}
	case *fullNode:
		for i := 0; i < 17; i++ {
			if node.Children[i] != nil {
				children = append(children, child{
					node:  node.Children[i],
					depth: req.depth + 1,
				})
			}
		}
	default:
		panic(fmt.Sprintf("unknown node: %+v", node))
	}
//循环访问子级，并请求所有未知的子级
	requests := make([]*request, 0, len(children))
	for _, child := range children {
//通知任何外部观察程序新的键/值节点
		if req.callback != nil {
			if node, ok := (child.node).(valueNode); ok {
				if err := req.callback(node, req.hash); err != nil {
					return nil, err
				}
			}
		}
//如果子节点引用另一个节点，请解析或调度
		if node, ok := (child.node).(hashNode); ok {
//尝试从本地数据库解析节点
			hash := common.BytesToHash(node)
			if _, ok := s.membatch.batch[hash]; ok {
				continue
			}
			if ok, _ := s.database.Has(node); ok {
				continue
			}
//本地未知节点，检索计划
			requests = append(requests, &request{
				hash:     hash,
				parents:  []*request{req},
				depth:    child.depth,
				callback: req.callback,
			})
		}
	}
	return requests, nil
}

//提交完成检索请求并将其存储到membatch中。如果有的话
//在由于这个提交而完成的引用父请求中，它们也是
//承诺自己。
func (s *Sync) commit(req *request) (err error) {
//将节点内容写入membatch
	s.membatch.batch[req.hash] = req.data
	s.membatch.order = append(s.membatch.order, req.hash)

	delete(s.requests, req.hash)

//检查所有家长是否完成
	for _, parent := range req.parents {
		parent.deps--
		if parent.deps == 0 {
			if err := s.commit(parent); err != nil {
				return err
			}
		}
	}
	return nil
}
