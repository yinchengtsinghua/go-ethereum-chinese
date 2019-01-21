
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

//包trie实现Merkle Patricia尝试。
package trie

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

var (
//EmptyRoot是空trie的已知根哈希。
	emptyRoot = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

//EmptyState是空状态trie项的已知哈希。
	emptyState = crypto.Keccak256Hash(nil)
)

var (
	cacheMissCounter   = metrics.NewRegisteredCounter("trie/cachemiss", nil)
	cacheUnloadCounter = metrics.NewRegisteredCounter("trie/cacheunload", nil)
)

//cache misses检索测量缓存未命中数的全局计数器
//自进程启动后，trie已启动。除了
//测试调试目的。
func CacheMisses() int64 {
	return cacheMissCounter.Count()
}

//cacheUnloads检索测量缓存卸载数的全局计数器
//自进程启动后，trie执行了操作。除了
//测试调试目的。
func CacheUnloads() int64 {
	return cacheUnloadCounter.Count()
}

//leaf callback是当trie操作到达叶时调用的回调类型
//节点。状态同步和提交使用它来允许处理外部引用
//在帐户和存储尝试之间。
type LeafCallback func(leaf []byte, parent common.Hash) error

//特里尔是梅克尔·帕特里夏·特里尔。
//零值是一个没有数据库的空trie。
//使用new创建位于数据库顶部的trie。
//
//同时使用trie不安全。
type Trie struct {
	db   *Database
	root node

//缓存生成值。
//cachegen随着每次提交操作增加一个。
//新节点用当前生成标记并卸载
//当其生成时间超过cachegen cachelimit时。
	cachegen, cachelimit uint16
}

//setcachelimit设置要保留的“缓存生成数”。
//通过调用commit来创建缓存生成。
func (t *Trie) SetCacheLimit(l uint16) {
	t.cachelimit = l
}

//newflag返回新创建节点的缓存标志值。
func (t *Trie) newFlag() nodeFlag {
	return nodeFlag{dirty: true, gen: t.cachegen}
}

//新建使用数据库中的现有根节点创建trie。
//
//如果根是空字符串的零哈希或sha3哈希，则
//trie最初为空，不需要数据库。否则，
//如果db为nil，new将死机；如果root为nil，则返回MissingNodeError。
//数据库中不存在。访问trie将按需从db加载节点。
func New(root common.Hash, db *Database) (*Trie, error) {
	if db == nil {
		panic("trie.New called without a database")
	}
	trie := &Trie{
		db: db,
	}
	if root != (common.Hash{}) && root != emptyRoot {
		rootnode, err := trie.resolveHash(root[:], nil)
		if err != nil {
			return nil, err
		}
		trie.root = rootnode
	}
	return trie, nil
}

//nodeiterator返回返回trie节点的迭代器。迭代开始于
//给定开始键之后的键。
func (t *Trie) NodeIterator(start []byte) NodeIterator {
	return newNodeIterator(t, start)
}

//get返回存储在trie中的键的值。
//调用方不能修改值字节。
func (t *Trie) Get(key []byte) []byte {
	res, err := t.TryGet(key)
	if err != nil {
		log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	}
	return res
}

//Tryget返回存储在trie中的键的值。
//调用方不能修改值字节。
//如果在数据库中找不到节点，则返回MissingNodeError。
func (t *Trie) TryGet(key []byte) ([]byte, error) {
	key = keybytesToHex(key)
	value, newroot, didResolve, err := t.tryGet(t.root, key, 0)
	if err == nil && didResolve {
		t.root = newroot
	}
	return value, err
}

func (t *Trie) tryGet(origNode node, key []byte, pos int) (value []byte, newnode node, didResolve bool, err error) {
	switch n := (origNode).(type) {
	case nil:
		return nil, nil, false, nil
	case valueNode:
		return n, n, false, nil
	case *shortNode:
		if len(key)-pos < len(n.Key) || !bytes.Equal(n.Key, key[pos:pos+len(n.Key)]) {
//在trie中找不到密钥
			return nil, n, false, nil
		}
		value, newnode, didResolve, err = t.tryGet(n.Val, key, pos+len(n.Key))
		if err == nil && didResolve {
			n = n.copy()
			n.Val = newnode
			n.flags.gen = t.cachegen
		}
		return value, n, didResolve, err
	case *fullNode:
		value, newnode, didResolve, err = t.tryGet(n.Children[key[pos]], key, pos+1)
		if err == nil && didResolve {
			n = n.copy()
			n.flags.gen = t.cachegen
			n.Children[key[pos]] = newnode
		}
		return value, n, didResolve, err
	case hashNode:
		child, err := t.resolveHash(n, key[:pos])
		if err != nil {
			return nil, n, true, err
		}
		value, newnode, _, err := t.tryGet(child, key, pos)
		return value, newnode, true, err
	default:
		panic(fmt.Sprintf("%T: invalid node: %v", origNode, origNode))
	}
}

//更新trie中的关联键和值。后续呼叫
//get将返回值。如果值的长度为零，则任何现有值
//从trie中删除，调用get将返回nil。
//
//当值字节是
//存储在trie中。
func (t *Trie) Update(key, value []byte) {
	if err := t.TryUpdate(key, value); err != nil {
		log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	}
}

//TryUpdate将键与trie中的值关联。后续呼叫
//get将返回值。如果值的长度为零，则任何现有值
//从trie中删除，调用get将返回nil。
//
//当值字节是
//存储在trie中。
//
//如果在数据库中找不到节点，则返回MissingNodeError。
func (t *Trie) TryUpdate(key, value []byte) error {
	k := keybytesToHex(key)
	if len(value) != 0 {
		_, n, err := t.insert(t.root, nil, k, valueNode(value))
		if err != nil {
			return err
		}
		t.root = n
	} else {
		_, n, err := t.delete(t.root, nil, k)
		if err != nil {
			return err
		}
		t.root = n
	}
	return nil
}

func (t *Trie) insert(n node, prefix, key []byte, value node) (bool, node, error) {
	if len(key) == 0 {
		if v, ok := n.(valueNode); ok {
			return !bytes.Equal(v, value.(valueNode)), value, nil
		}
		return true, value, nil
	}
	switch n := n.(type) {
	case *shortNode:
		matchlen := prefixLen(key, n.Key)
//如果整个键匹配，则保持此短节点不变
//只更新值。
		if matchlen == len(n.Key) {
			dirty, nn, err := t.insert(n.Val, append(prefix, key[:matchlen]...), key[matchlen:], value)
			if !dirty || err != nil {
				return false, n, err
			}
			return true, &shortNode{n.Key, nn, t.newFlag()}, nil
		}
//否则，在不同的索引处进行分支。
		branch := &fullNode{flags: t.newFlag()}
		var err error
		_, branch.Children[n.Key[matchlen]], err = t.insert(nil, append(prefix, n.Key[:matchlen+1]...), n.Key[matchlen+1:], n.Val)
		if err != nil {
			return false, nil, err
		}
		_, branch.Children[key[matchlen]], err = t.insert(nil, append(prefix, key[:matchlen+1]...), key[matchlen+1:], value)
		if err != nil {
			return false, nil, err
		}
//如果此短代码出现在索引0处，则将其替换为分支。
		if matchlen == 0 {
			return true, branch, nil
		}
//否则，将其替换为通向分支的短节点。
		return true, &shortNode{key[:matchlen], branch, t.newFlag()}, nil

	case *fullNode:
		dirty, nn, err := t.insert(n.Children[key[0]], append(prefix, key[0]), key[1:], value)
		if !dirty || err != nil {
			return false, n, err
		}
		n = n.copy()
		n.flags = t.newFlag()
		n.Children[key[0]] = nn
		return true, n, nil

	case nil:
		return true, &shortNode{key, value, t.newFlag()}, nil

	case hashNode:
//我们击中了一个尚未加载的部分trie。负载
//并插入到节点中。这将使所有子节点保持打开状态
//trie中值的路径。
		rn, err := t.resolveHash(n, prefix)
		if err != nil {
			return false, nil, err
		}
		dirty, nn, err := t.insert(rn, prefix, key, value)
		if !dirty || err != nil {
			return false, rn, err
		}
		return true, nn, nil

	default:
		panic(fmt.Sprintf("%T: invalid node: %v", n, n))
	}
}

//删除从trie中删除键的任何现有值。
func (t *Trie) Delete(key []byte) {
	if err := t.TryDelete(key); err != nil {
		log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	}
}

//Trydelete从trie中删除键的任何现有值。
//如果在数据库中找不到节点，则返回MissingNodeError。
func (t *Trie) TryDelete(key []byte) error {
	k := keybytesToHex(key)
	_, n, err := t.delete(t.root, nil, k)
	if err != nil {
		return err
	}
	t.root = n
	return nil
}

//delete返回删除键的trie的新根。
//它通过简化将trie简化为最小形式
//递归删除后，节点正在向上移动。
func (t *Trie) delete(n node, prefix, key []byte) (bool, node, error) {
	switch n := n.(type) {
	case *shortNode:
		matchlen := prefixLen(key, n.Key)
		if matchlen < len(n.Key) {
return false, n, nil //不匹配时不替换n
		}
		if matchlen == len(key) {
return true, nil, nil //对整个匹配完全删除n
		}
//密钥比N.key长。删除其余后缀
//从子目录。孩子在这里永远不可能是零，因为
//Subrie必须包含至少两个具有键的其他值
//比N.key长。
		dirty, child, err := t.delete(n.Val, append(prefix, key[:len(n.Key)]...), key[len(n.Key):])
		if !dirty || err != nil {
			return false, n, err
		}
		switch child := child.(type) {
		case *shortNode:
//从子目录中删除将其还原为另一个子目录
//短节点。合并节点以避免创建
//短代码…，短代码…。使用concat（其中
//总是创建一个新切片），而不是附加到
//避免修改n.key，因为它可能与共享
//其他节点。
			return true, &shortNode{concat(n.Key, child.Key...), child.Val, t.newFlag()}, nil
		default:
			return true, &shortNode{n.Key, child, t.newFlag()}, nil
		}

	case *fullNode:
		dirty, nn, err := t.delete(n.Children[key[0]], append(prefix, key[0]), key[1:])
		if !dirty || err != nil {
			return false, n, err
		}
		n = n.copy()
		n.flags = t.newFlag()
		n.Children[key[0]] = nn

//检查删除后还有多少非零条目，以及
//如果只有一个条目，则将完整节点缩减为短节点
//左边。因为N至少有两个孩子
//删除前（否则将不是完整节点）n
//不能减为零。
//
//循环完成后，pos包含单个
//如果n至少包含两个，则保留在n或-2中的值
//价值观。
		pos := -1
		for i, cld := range &n.Children {
			if cld != nil {
				if pos == -1 {
					pos = i
				} else {
					pos = -2
					break
				}
			}
		}
		if pos >= 0 {
			if pos != 16 {
//如果其余条目是短节点，则它将替换
//它的钥匙把丢失的小齿钉在
//前面。这样可以避免创建无效的
//短代码…，短代码…。进入以来
//可能尚未加载，仅为此解决
//检查。
				cnode, err := t.resolve(n.Children[pos], prefix)
				if err != nil {
					return false, nil, err
				}
				if cnode, ok := cnode.(*shortNode); ok {
					k := append([]byte{byte(pos)}, cnode.Key...)
					return true, &shortNode{k, cnode.Val, t.newFlag()}, nil
				}
			}
//否则，n替换为一个半字节的短节点
//包含孩子。
			return true, &shortNode{[]byte{byte(pos)}, n.Children[pos], t.newFlag()}, nil
		}
//n仍然包含至少两个值，不能减少。
		return true, n, nil

	case valueNode:
		return true, nil, nil

	case nil:
		return false, nil, nil

	case hashNode:
//我们击中了一个尚未加载的部分trie。负载
//并从中删除。这将使所有子节点保持打开状态
//trie中值的路径。
		rn, err := t.resolveHash(n, prefix)
		if err != nil {
			return false, nil, err
		}
		dirty, nn, err := t.delete(rn, prefix, key)
		if !dirty || err != nil {
			return false, rn, err
		}
		return true, nn, nil

	default:
		panic(fmt.Sprintf("%T: invalid node: %v (%v)", n, n, key))
	}
}

func concat(s1 []byte, s2 ...byte) []byte {
	r := make([]byte, len(s1)+len(s2))
	copy(r, s1)
	copy(r[len(s1):], s2)
	return r
}

func (t *Trie) resolve(n node, prefix []byte) (node, error) {
	if n, ok := n.(hashNode); ok {
		return t.resolveHash(n, prefix)
	}
	return n, nil
}

func (t *Trie) resolveHash(n hashNode, prefix []byte) (node, error) {
	cacheMissCounter.Inc(1)

	hash := common.BytesToHash(n)
	if node := t.db.node(hash, t.cachegen); node != nil {
		return node, nil
	}
	return nil, &MissingNodeError{NodeHash: hash, Path: prefix}
}

//根返回trie的根哈希。
//已弃用：请改用哈希。
func (t *Trie) Root() []byte { return t.Hash().Bytes() }

//哈希返回trie的根哈希。它不会写信给
//即使trie没有数据库也可以使用。
func (t *Trie) Hash() common.Hash {
	hash, cached, _ := t.hashRoot(nil, nil)
	t.root = cached
	return common.BytesToHash(hash.(hashNode))
}

//提交将所有节点写入trie的内存数据库，跟踪内部
//和外部（用于帐户尝试）引用。
func (t *Trie) Commit(onleaf LeafCallback) (root common.Hash, err error) {
	if t.db == nil {
		panic("commit called on trie with nil database")
	}
	hash, cached, err := t.hashRoot(t.db, onleaf)
	if err != nil {
		return common.Hash{}, err
	}
	t.root = cached
	t.cachegen++
	return common.BytesToHash(hash.(hashNode)), nil
}

func (t *Trie) hashRoot(db *Database, onleaf LeafCallback) (node, node, error) {
	if t.root == nil {
		return hashNode(emptyRoot.Bytes()), nil, nil
	}
	h := newHasher(t.cachegen, t.cachelimit, onleaf)
	defer returnHasherToPool(h)
	return h.hash(t.root, db, true)
}
