
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

package light

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
)

func NewState(ctx context.Context, head *types.Header, odr OdrBackend) *state.StateDB {
	state, _ := state.New(head.Root, NewStateDatabase(ctx, head, odr))
	return state
}

func NewStateDatabase(ctx context.Context, head *types.Header, odr OdrBackend) state.Database {
	return &odrDatabase{ctx, StateTrieID(head), odr}
}

type odrDatabase struct {
	ctx     context.Context
	id      *TrieID
	backend OdrBackend
}

func (db *odrDatabase) OpenTrie(root common.Hash) (state.Trie, error) {
	return &odrTrie{db: db, id: db.id}, nil
}

func (db *odrDatabase) OpenStorageTrie(addrHash, root common.Hash) (state.Trie, error) {
	return &odrTrie{db: db, id: StorageTrieID(db.id, addrHash, root)}, nil
}

func (db *odrDatabase) CopyTrie(t state.Trie) state.Trie {
	switch t := t.(type) {
	case *odrTrie:
		cpy := &odrTrie{db: t.db, id: t.id}
		if t.trie != nil {
			cpytrie := *t.trie
			cpy.trie = &cpytrie
		}
		return cpy
	default:
		panic(fmt.Errorf("unknown trie type %T", t))
	}
}

func (db *odrDatabase) ContractCode(addrHash, codeHash common.Hash) ([]byte, error) {
	if codeHash == sha3_nil {
		return nil, nil
	}
	if code, err := db.backend.Database().Get(codeHash[:]); err == nil {
		return code, nil
	}
	id := *db.id
	id.AccKey = addrHash[:]
	req := &CodeRequest{Id: &id, Hash: codeHash}
	err := db.backend.Retrieve(db.ctx, req)
	return req.Data, err
}

func (db *odrDatabase) ContractCodeSize(addrHash, codeHash common.Hash) (int, error) {
	code, err := db.ContractCode(addrHash, codeHash)
	return len(code), err
}

func (db *odrDatabase) TrieDB() *trie.Database {
	return nil
}

type odrTrie struct {
	db   *odrDatabase
	id   *TrieID
	trie *trie.Trie
}

func (t *odrTrie) TryGet(key []byte) ([]byte, error) {
	key = crypto.Keccak256(key)
	var res []byte
	err := t.do(key, func() (err error) {
		res, err = t.trie.TryGet(key)
		return err
	})
	return res, err
}

func (t *odrTrie) TryUpdate(key, value []byte) error {
	key = crypto.Keccak256(key)
	return t.do(key, func() error {
		return t.trie.TryUpdate(key, value)
	})
}

func (t *odrTrie) TryDelete(key []byte) error {
	key = crypto.Keccak256(key)
	return t.do(key, func() error {
		return t.trie.TryDelete(key)
	})
}

func (t *odrTrie) Commit(onleaf trie.LeafCallback) (common.Hash, error) {
	if t.trie == nil {
		return t.id.Root, nil
	}
	return t.trie.Commit(onleaf)
}

func (t *odrTrie) Hash() common.Hash {
	if t.trie == nil {
		return t.id.Root
	}
	return t.trie.Hash()
}

func (t *odrTrie) NodeIterator(startkey []byte) trie.NodeIterator {
	return newNodeIterator(t, startkey)
}

func (t *odrTrie) GetKey(sha []byte) []byte {
	return nil
}

func (t *odrTrie) Prove(key []byte, fromLevel uint, proofDb ethdb.Putter) error {
	return errors.New("not implemented, needs client/server interface split")
}

//尝试并重试执行一个函数，直到它返回时没有错误或
//MissingNodeError以外的错误类型
func (t *odrTrie) do(key []byte, fn func() error) error {
	for {
		var err error
		if t.trie == nil {
			t.trie, err = trie.New(t.id.Root, trie.NewDatabase(t.db.backend.Database()))
		}
		if err == nil {
			err = fn()
		}
		if _, ok := err.(*trie.MissingNodeError); !ok {
			return err
		}
		r := &TrieRequest{Id: t.id, Key: key}
		if err := t.db.backend.Retrieve(t.db.ctx, r); err != nil {
			return err
		}
	}
}

type nodeIterator struct {
	trie.NodeIterator
	t   *odrTrie
	err error
}

func newNodeIterator(t *odrTrie, startkey []byte) trie.NodeIterator {
	it := &nodeIterator{t: t}
//如果还没有打开实际的非odr trie。
	if t.trie == nil {
		it.do(func() error {
			t, err := trie.New(t.id.Root, trie.NewDatabase(t.db.backend.Database()))
			if err == nil {
				it.t.trie = t
			}
			return err
		})
	}
	it.do(func() error {
		it.NodeIterator = it.t.trie.NodeIterator(startkey)
		return it.NodeIterator.Error()
	})
	return it
}

func (it *nodeIterator) Next(descend bool) bool {
	var ok bool
	it.do(func() error {
		ok = it.NodeIterator.Next(descend)
		return it.NodeIterator.Error()
	})
	return ok
}

//do运行fn并尝试通过检索来填充丢失的节点。
func (it *nodeIterator) do(fn func() error) {
	var lasthash common.Hash
	for {
		it.err = fn()
		missing, ok := it.err.(*trie.MissingNodeError)
		if !ok {
			return
		}
		if missing.NodeHash == lasthash {
			it.err = fmt.Errorf("retrieve loop for trie node %x", missing.NodeHash)
			return
		}
		lasthash = missing.NodeHash
		r := &TrieRequest{Id: it.t.id, Key: nibblesToKey(missing.Path)}
		if it.err = it.t.db.backend.Retrieve(it.t.db.ctx, r); it.err != nil {
			return
		}
	}
}

func (it *nodeIterator) Error() error {
	if it.err != nil {
		return it.err
	}
	return it.NodeIterator.Error()
}

func nibblesToKey(nib []byte) []byte {
	if len(nib) > 0 && nib[len(nib)-1] == 0x10 {
nib = nib[:len(nib)-1] //液滴终止器
	}
	if len(nib)&1 == 1 {
nib = append(nib, 0) //使均匀
	}
	key := make([]byte, len(nib)/2)
	for bi, ni := 0, 0; ni < len(nib); bi, ni = bi+1, ni+2 {
		key[bi] = nib[ni]<<4 | nib[ni+1]
	}
	return key
}
