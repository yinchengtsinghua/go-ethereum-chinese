
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
	"container/heap"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

//迭代器是一个键值trie迭代器，它遍历trie。
type Iterator struct {
	nodeIt NodeIterator

Key   []byte //迭代器所在的当前数据键
Value []byte //迭代器所在的当前数据值
	Err   error
}

//NewIterator从节点迭代器创建新的键值迭代器
func NewIterator(it NodeIterator) *Iterator {
	return &Iterator{
		nodeIt: it,
	}
}

//下一步将迭代器向前移动一个键值项。
func (it *Iterator) Next() bool {
	for it.nodeIt.Next(true) {
		if it.nodeIt.Leaf() {
			it.Key = it.nodeIt.LeafKey()
			it.Value = it.nodeIt.LeafBlob()
			return true
		}
	}
	it.Key = nil
	it.Value = nil
	it.Err = it.nodeIt.Error()
	return false
}

//prove为迭代器当前所在的叶节点生成merkle proof
//定位在。
func (it *Iterator) Prove() [][]byte {
	return it.nodeIt.LeafProof()
}

//nodeiterator是一个迭代器，用于遍历trie的pre-order。
type NodeIterator interface {
//下一步将迭代器移动到下一个节点。如果参数为false，则任何子级
//将跳过节点。
	Next(bool) bool

//错误返回迭代器的错误状态。
	Error() error

//哈希返回当前节点的哈希。
	Hash() common.Hash

//父节点返回当前节点父节点的哈希。哈希值可能是
//如果直接父节点是没有哈希的内部节点，则为祖父母。
	Parent() common.Hash

//path返回到当前节点的十六进制编码路径。
//调用方在调用next后不能保留对返回值的引用。
//对于叶节点，路径的最后一个元素是“终止符符号”0x10。
	Path() []byte

//如果当前节点是叶节点，则叶返回真值。
	Leaf() bool

//leaf key返回叶的键。如果迭代器不是
//放在一片叶子上。调用方不能保留对以下值的引用
//呼叫下一个。
	LeafKey() []byte

//leafblob返回叶的内容。如果迭代器
//不在叶上。调用方不能保留对值的引用
//在呼叫下一个之后。
	LeafBlob() []byte

//leaf proof返回叶子的merkle证明。如果
//迭代器未定位在叶上。调用方不能保留引用
//调用next后的值。
	LeafProof() [][]byte
}

//nodeiteratorState表示在
//trie，可以在以后的调用中恢复。
type nodeIteratorState struct {
hash    common.Hash //正在迭代的节点的哈希（如果不是独立的，则为零）
node    node        //正在迭代的trie节点
parent  common.Hash //第一个完整祖先节点的哈希（如果当前是根节点，则为零）
index   int         //下一个要处理的子级
pathlen int         //此节点的路径长度
}

type nodeIterator struct {
trie  *Trie                //正在迭代的trie
stack []*nodeIteratorState //保持迭代状态的trie节点的层次结构
path  []byte               //当前节点的路径
err   error                //迭代器中出现内部错误时的故障集
}

//迭代完成后，errIteratorEnd存储在nodeiterator.err中。
var errIteratorEnd = errors.New("end of iteration")

//如果初始搜索失败，则seek err存储在nodeiterator.err中。
type seekError struct {
	key []byte
	err error
}

func (e seekError) Error() string {
	return "seek error: " + e.err.Error()
}

func newNodeIterator(trie *Trie, start []byte) NodeIterator {
	if trie.Hash() == emptyState {
		return new(nodeIterator)
	}
	it := &nodeIterator{trie: trie}
	it.err = it.seek(start)
	return it
}

func (it *nodeIterator) Hash() common.Hash {
	if len(it.stack) == 0 {
		return common.Hash{}
	}
	return it.stack[len(it.stack)-1].hash
}

func (it *nodeIterator) Parent() common.Hash {
	if len(it.stack) == 0 {
		return common.Hash{}
	}
	return it.stack[len(it.stack)-1].parent
}

func (it *nodeIterator) Leaf() bool {
	return hasTerm(it.path)
}

func (it *nodeIterator) LeafKey() []byte {
	if len(it.stack) > 0 {
		if _, ok := it.stack[len(it.stack)-1].node.(valueNode); ok {
			return hexToKeybytes(it.path)
		}
	}
	panic("not at leaf")
}

func (it *nodeIterator) LeafBlob() []byte {
	if len(it.stack) > 0 {
		if node, ok := it.stack[len(it.stack)-1].node.(valueNode); ok {
			return []byte(node)
		}
	}
	panic("not at leaf")
}

func (it *nodeIterator) LeafProof() [][]byte {
	if len(it.stack) > 0 {
		if _, ok := it.stack[len(it.stack)-1].node.(valueNode); ok {
			hasher := newHasher(0, 0, nil)
			defer returnHasherToPool(hasher)

			proofs := make([][]byte, 0, len(it.stack))

			for i, item := range it.stack[:len(it.stack)-1] {
//收集最终成为哈希节点（或根节点）的节点
				node, _, _ := hasher.hashChildren(item.node, nil)
				hashed, _ := hasher.store(node, nil, false)
				if _, ok := hashed.(hashNode); ok || i == 0 {
					enc, _ := rlp.EncodeToBytes(node)
					proofs = append(proofs, enc)
				}
			}
			return proofs
		}
	}
	panic("not at leaf")
}

func (it *nodeIterator) Path() []byte {
	return it.path
}

func (it *nodeIterator) Error() error {
	if it.err == errIteratorEnd {
		return nil
	}
	if seek, ok := it.err.(seekError); ok {
		return seek.err
	}
	return it.err
}

//next将迭代器移动到下一个节点，返回是否存在
//进一步的节点。如果出现内部错误，此方法将返回false，并且
//将错误字段设置为遇到的故障。如果'Descend'为false，
//跳过当前节点的任何子节点的迭代。
func (it *nodeIterator) Next(descend bool) bool {
	if it.err == errIteratorEnd {
		return false
	}
	if seek, ok := it.err.(seekError); ok {
		if it.err = it.seek(seek.key); it.err != nil {
			return false
		}
	}
//否则，使用迭代器前进并报告任何错误。
	state, parentIndex, path, err := it.peek(descend)
	it.err = err
	if it.err != nil {
		return false
	}
	it.push(state, parentIndex, path)
	return true
}

func (it *nodeIterator) seek(prefix []byte) error {
//我们要查找的路径是不带终止符的十六进制编码键。
	key := keybytesToHex(prefix)
	key = key[:len(key)-1]
//向前移动，直到我们刚好在最接近的匹配键之前。
	for {
		state, parentIndex, path, err := it.peek(bytes.HasPrefix(key, it.path))
		if err == errIteratorEnd {
			return errIteratorEnd
		} else if err != nil {
			return seekError{prefix, err}
		} else if bytes.Compare(path, key) >= 0 {
			return nil
		}
		it.push(state, parentIndex, path)
	}
}

//Peek创建迭代器的下一个状态。
func (it *nodeIterator) peek(descend bool) (*nodeIteratorState, *int, []byte, error) {
	if len(it.stack) == 0 {
//如果我们刚刚开始，初始化迭代器。
		root := it.trie.Hash()
		state := &nodeIteratorState{node: it.trie.root, index: -1}
		if root != emptyRoot {
			state.hash = root
		}
		err := state.resolve(it.trie, nil)
		return state, nil, nil, err
	}
	if !descend {
//如果跳过子节点，请先弹出当前节点
		it.pop()
	}

//继续迭代到下一个子级
	for len(it.stack) > 0 {
		parent := it.stack[len(it.stack)-1]
		ancestor := parent.hash
		if (ancestor == common.Hash{}) {
			ancestor = parent.parent
		}
		state, path, ok := it.nextChild(parent, ancestor)
		if ok {
			if err := state.resolve(it.trie, path); err != nil {
				return parent, &parent.index, path, err
			}
			return state, &parent.index, path, nil
		}
//不再有子节点，请向后移动。
		it.pop()
	}
	return nil, nil, nil, errIteratorEnd
}

func (st *nodeIteratorState) resolve(tr *Trie, path []byte) error {
	if hash, ok := st.node.(hashNode); ok {
		resolved, err := tr.resolveHash(hash, path)
		if err != nil {
			return err
		}
		st.node = resolved
		st.hash = common.BytesToHash(hash)
	}
	return nil
}

func (it *nodeIterator) nextChild(parent *nodeIteratorState, ancestor common.Hash) (*nodeIteratorState, []byte, bool) {
	switch node := parent.node.(type) {
	case *fullNode:
//完整节点，移动到第一个非零子节点。
		for i := parent.index + 1; i < len(node.Children); i++ {
			child := node.Children[i]
			if child != nil {
				hash, _ := child.cache()
				state := &nodeIteratorState{
					hash:    common.BytesToHash(hash),
					node:    child,
					parent:  ancestor,
					index:   -1,
					pathlen: len(it.path),
				}
				path := append(it.path, byte(i))
				parent.index = i - 1
				return state, path, true
			}
		}
	case *shortNode:
//短节点，返回指针singleton子级
		if parent.index < 0 {
			hash, _ := node.Val.cache()
			state := &nodeIteratorState{
				hash:    common.BytesToHash(hash),
				node:    node.Val,
				parent:  ancestor,
				index:   -1,
				pathlen: len(it.path),
			}
			path := append(it.path, node.Key...)
			return state, path, true
		}
	}
	return parent, it.path, false
}

func (it *nodeIterator) push(state *nodeIteratorState, parentIndex *int, path []byte) {
	it.path = path
	it.stack = append(it.stack, state)
	if parentIndex != nil {
		*parentIndex++
	}
}

func (it *nodeIterator) pop() {
	parent := it.stack[len(it.stack)-1]
	it.path = it.path[:parent.pathlen]
	it.stack = it.stack[:len(it.stack)-1]
}

func compareNodes(a, b NodeIterator) int {
	if cmp := bytes.Compare(a.Path(), b.Path()); cmp != 0 {
		return cmp
	}
	if a.Leaf() && !b.Leaf() {
		return -1
	} else if b.Leaf() && !a.Leaf() {
		return 1
	}
	if cmp := bytes.Compare(a.Hash().Bytes(), b.Hash().Bytes()); cmp != 0 {
		return cmp
	}
	if a.Leaf() && b.Leaf() {
		return bytes.Compare(a.LeafBlob(), b.LeafBlob())
	}
	return 0
}

type differenceIterator struct {
a, b  NodeIterator //返回的节点是B-A中的节点。
eof   bool         //表示元素已用完
count int          //在任一个trie上扫描的节点数
}

//newDifferenceInterator构造一个nodeiterator，它迭代b中的元素，
//不在a中。返回迭代器和一个指向记录数字的整数的指针
//节点看到。
func NewDifferenceIterator(a, b NodeIterator) (NodeIterator, *int) {
	a.Next(true)
	it := &differenceIterator{
		a: a,
		b: b,
	}
	return it, &it.count
}

func (it *differenceIterator) Hash() common.Hash {
	return it.b.Hash()
}

func (it *differenceIterator) Parent() common.Hash {
	return it.b.Parent()
}

func (it *differenceIterator) Leaf() bool {
	return it.b.Leaf()
}

func (it *differenceIterator) LeafKey() []byte {
	return it.b.LeafKey()
}

func (it *differenceIterator) LeafBlob() []byte {
	return it.b.LeafBlob()
}

func (it *differenceIterator) LeafProof() [][]byte {
	return it.b.LeafProof()
}

func (it *differenceIterator) Path() []byte {
	return it.b.Path()
}

func (it *differenceIterator) Next(bool) bool {
//Invariants：
//-我们总是在b中至少推进一个元素。
//—在该函数的开头，a的路径在词汇上大于b的路径。
	if !it.b.Next(true) {
		return false
	}
	it.count++

	if it.eof {
//A已达到EOF，所以我们只返回B的所有元素
		return true
	}

	for {
		switch compareNodes(it.a, it.b) {
		case -1:
//B跳过A；前进A
			if !it.a.Next(true) {
				it.eof = true
				return true
			}
			it.count++
		case 1:
//B在A之前
			return true
		case 0:
//A和B是相同的；如果节点具有哈希值，则跳过整个子树
			hasHash := it.a.Hash() == common.Hash{}
			if !it.b.Next(hasHash) {
				return false
			}
			it.count++
			if !it.a.Next(hasHash) {
				it.eof = true
				return true
			}
			it.count++
		}
	}
}

func (it *differenceIterator) Error() error {
	if err := it.a.Error(); err != nil {
		return err
	}
	return it.b.Error()
}

type nodeIteratorHeap []NodeIterator

func (h nodeIteratorHeap) Len() int            { return len(h) }
func (h nodeIteratorHeap) Less(i, j int) bool  { return compareNodes(h[i], h[j]) < 0 }
func (h nodeIteratorHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *nodeIteratorHeap) Push(x interface{}) { *h = append(*h, x.(NodeIterator)) }
func (h *nodeIteratorHeap) Pop() interface{} {
	n := len(*h)
	x := (*h)[n-1]
	*h = (*h)[0 : n-1]
	return x
}

type unionIterator struct {
items *nodeIteratorHeap //返回的节点是这些迭代器中的节点的联合
count int               //在所有尝试中扫描的节点数
}

//NewUnionIterator构造一个节点运算符，它迭代联合中的元素
//提供的节点运算符。返回迭代器和指向整数的指针
//记录访问的节点数。
func NewUnionIterator(iters []NodeIterator) (NodeIterator, *int) {
	h := make(nodeIteratorHeap, len(iters))
	copy(h, iters)
	heap.Init(&h)

	ui := &unionIterator{items: &h}
	return ui, &ui.count
}

func (it *unionIterator) Hash() common.Hash {
	return (*it.items)[0].Hash()
}

func (it *unionIterator) Parent() common.Hash {
	return (*it.items)[0].Parent()
}

func (it *unionIterator) Leaf() bool {
	return (*it.items)[0].Leaf()
}

func (it *unionIterator) LeafKey() []byte {
	return (*it.items)[0].LeafKey()
}

func (it *unionIterator) LeafBlob() []byte {
	return (*it.items)[0].LeafBlob()
}

func (it *unionIterator) LeafProof() [][]byte {
	return (*it.items)[0].LeafProof()
}

func (it *unionIterator) Path() []byte {
	return (*it.items)[0].Path()
}

//next返回正在迭代的尝试联合中的下一个节点。
//
//它通过维护一堆迭代器（按迭代排序）来实现这一点。
//下一个元素的顺序，每个源trie有一个条目。各
//调用next（）时，需要从堆中返回最少的元素，
//推进任何其他也指向同一元素的迭代器。这些
//使用Descend=false调用迭代器，因为我们知道
//这些节点也将是重复的，可以在当前选定的迭代器中找到。
//每当一个迭代器被提升时，如果它仍然被推回到堆中
//还有个元素。
//
//在Descend=false的情况下-例如，我们被要求忽略
//当前节点-我们还推进堆中具有当前节点的任何迭代器
//作为前缀的路径。
func (it *unionIterator) Next(descend bool) bool {
	if len(*it.items) == 0 {
		return false
	}

//从工会那里拿到下一把钥匙
	least := heap.Pop(it.items).(NodeIterator)

//跳过其他节点，只要它们相同，或者，如果我们不降序，则为
//只要它们具有与当前节点相同的前缀。
	for len(*it.items) > 0 && ((!descend && bytes.HasPrefix((*it.items)[0].Path(), least.Path())) || compareNodes(least, (*it.items)[0]) == 0) {
		skipped := heap.Pop(it.items).(NodeIterator)
//如果节点具有哈希值，则跳过整个子树；否则只跳过此节点。
		if skipped.Next(skipped.Hash() == common.Hash{}) {
			it.count++
//如果有更多的元素，将迭代器推回到堆上
			heap.Push(it.items, skipped)
		}
	}
	if least.Next(descend) {
		it.count++
		heap.Push(it.items, least)
	}
	return len(*it.items) > 0
}

func (it *unionIterator) Error() error {
	for i := 0; i < len(*it.items); i++ {
		if err := (*it.items)[i].Error(); err != nil {
			return err
		}
	}
	return nil
}
