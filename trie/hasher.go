
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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
	"hash"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
)

type hasher struct {
	tmp        sliceBuffer
	sha        keccakState
	cachegen   uint16
	cachelimit uint16
	onleaf     LeafCallback
}

//keccakstate包裹sha3.state。除了通常的哈希方法外，它还支持
//读取以从哈希状态获取可变数量的数据。读取比求和快
//因为它不复制内部状态，而是修改内部状态。
type keccakState interface {
	hash.Hash
	Read([]byte) (int, error)
}

type sliceBuffer []byte

func (b *sliceBuffer) Write(data []byte) (n int, err error) {
	*b = append(*b, data...)
	return len(data), nil
}

func (b *sliceBuffer) Reset() {
	*b = (*b)[:0]
}

//哈希存在于全局数据库中。
var hasherPool = sync.Pool{
	New: func() interface{} {
		return &hasher{
tmp: make(sliceBuffer, 0, 550), //cap和完整的fullnode一样大。
			sha: sha3.NewLegacyKeccak256().(keccakState),
		}
	},
}

func newHasher(cachegen, cachelimit uint16, onleaf LeafCallback) *hasher {
	h := hasherPool.Get().(*hasher)
	h.cachegen, h.cachelimit, h.onleaf = cachegen, cachelimit, onleaf
	return h
}

func returnHasherToPool(h *hasher) {
	hasherPool.Put(h)
}

//哈希将节点向下折叠为哈希节点，同时返回
//原始节点初始化为计算哈希以替换原始节点。
func (h *hasher) hash(n node, db *Database, force bool) (node, node, error) {
//如果我们不存储节点，只需要散列，使用可用的缓存数据
	if hash, dirty := n.cache(); hash != nil {
		if db == nil {
			return hash, n, nil
		}
		if n.canUnload(h.cachegen, h.cachelimit) {
//从缓存中卸载节点。其所有子节点都将具有一个较低或相等的
//缓存生成编号。
			cacheUnloadCounter.Inc(1)
			return hash, hash, nil
		}
		if !dirty {
			return hash, n, nil
		}
	}
//尚未处理或需要存储，请带孩子走
	collapsed, cached, err := h.hashChildren(n, db)
	if err != nil {
		return hashNode{}, n, err
	}
	hashed, err := h.store(collapsed, db, force)
	if err != nil {
		return hashNode{}, n, err
	}
//缓存节点的哈希以便稍后重用和删除
//提交模式下的脏标志。可以直接指定这些值
//不首先复制节点，因为hashchildren会复制它。
	cachedHash, _ := hashed.(hashNode)
	switch cn := cached.(type) {
	case *shortNode:
		cn.flags.hash = cachedHash
		if db != nil {
			cn.flags.dirty = false
		}
	case *fullNode:
		cn.flags.hash = cachedHash
		if db != nil {
			cn.flags.dirty = false
		}
	}
	return hashed, cached, nil
}

//hashchildren将节点的子节点替换为其哈希，如果
//子节点的大小大于哈希，同时返回折叠的节点
//用缓存的子哈希替换原始节点。
func (h *hasher) hashChildren(original node, db *Database) (node, node, error) {
	var err error

	switch n := original.(type) {
	case *shortNode:
//散列短节点的子节点，缓存新散列的子树
		collapsed, cached := n.copy(), n.copy()
		collapsed.Key = hexToCompact(n.Key)
		cached.Key = common.CopyBytes(n.Key)

		if _, ok := n.Val.(valueNode); !ok {
			collapsed.Val, cached.Val, err = h.hash(n.Val, db, false)
			if err != nil {
				return original, original, err
			}
		}
		return collapsed, cached, nil

	case *fullNode:
//散列完整节点的子节点，缓存新散列的子树
		collapsed, cached := n.copy(), n.copy()

		for i := 0; i < 16; i++ {
			if n.Children[i] != nil {
				collapsed.Children[i], cached.Children[i], err = h.hash(n.Children[i], db, false)
				if err != nil {
					return original, original, err
				}
			}
		}
		cached.Children[16] = n.Children[16]
		return collapsed, cached, nil

	default:
//值和哈希节点没有子节点，因此它们保持原样
		return n, original, nil
	}
}

//store散列节点n，如果指定了存储层，它将写入
//它的键/值对并跟踪任何节点->子引用以及任何
//节点->外部trie引用。
func (h *hasher) store(n node, db *Database, force bool) (node, error) {
//不要存储哈希或空节点。
	if _, isHash := n.(hashNode); n == nil || isHash {
		return n, nil
	}
//生成节点的rlp编码
	h.tmp.Reset()
	if err := rlp.Encode(&h.tmp, n); err != nil {
		panic("encode error: " + err.Error())
	}
	if len(h.tmp) < 32 && !force {
return n, nil //小于32字节的节点存储在其父节点中
	}
//较大的节点被其散列替换并存储在数据库中。
	hash, _ := n.cache()
	if hash == nil {
		hash = h.makeHashNode(h.tmp)
	}

	if db != nil {
//我们正在将trie节点合并到中间内存缓存中
		hash := common.BytesToHash(hash)

		db.lock.Lock()
		db.insert(hash, h.tmp, n)
		db.lock.Unlock()

//从“帐户”->“存储检索”跟踪外部引用
		if h.onleaf != nil {
			switch n := n.(type) {
			case *shortNode:
				if child, ok := n.Val.(valueNode); ok {
					h.onleaf(child, hash)
				}
			case *fullNode:
				for i := 0; i < 16; i++ {
					if child, ok := n.Children[i].(valueNode); ok {
						h.onleaf(child, hash)
					}
				}
			}
		}
	}
	return hash, nil
}

func (h *hasher) makeHashNode(data []byte) hashNode {
	n := make(hashNode, h.sha.Size())
	h.sha.Reset()
	h.sha.Write(data)
	h.sha.Read(n)
	return n
}
