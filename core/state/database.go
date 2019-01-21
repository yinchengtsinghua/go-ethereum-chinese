
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

package state

import (
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
	lru "github.com/hashicorp/golang-lru"
)

//Trie cache generation limit after which to evict trie nodes from memory.
var MaxTrieCacheGen = uint16(120)

const (
//要保留的过去尝试次数。此值的选择方式如下：
//合理的链条重铺深度将达到现有的三重。
	maxPastTries = 12

//要保留的codehash->大小关联数。
	codeSizeCacheSize = 100000
)

//数据库将访问权限包装为“尝试”和“合同代码”。
type Database interface {
//opentrie打开主帐户trie。
	OpenTrie(root common.Hash) (Trie, error)

//openstoragetrie打开帐户的存储trie。
	OpenStorageTrie(addrHash, root common.Hash) (Trie, error)

//copy trie返回给定trie的独立副本。
	CopyTrie(Trie) Trie

//ContractCode检索特定合同的代码。
	ContractCode(addrHash, codeHash common.Hash) ([]byte, error)

//ContractCodeSize检索特定合同代码的大小。
	ContractCodeSize(addrHash, codeHash common.Hash) (int, error)

//triedb检索用于数据存储的低级trie数据库。
	TrieDB() *trie.Database
}

//特里亚是以太梅克尔特里亚。
type Trie interface {
	TryGet(key []byte) ([]byte, error)
	TryUpdate(key, value []byte) error
	TryDelete(key []byte) error
	Commit(onleaf trie.LeafCallback) (common.Hash, error)
	Hash() common.Hash
	NodeIterator(startKey []byte) trie.NodeIterator
GetKey([]byte) []byte //TODO（FJL）：移除SecureTrie时移除此项
	Prove(key []byte, fromLevel uint, proofDb ethdb.Putter) error
}

//NeXDATA为状态创建后备存储。返回的数据库是安全的
//并发使用并在内存中保留一些最近扩展的trie节点。保持
//内存中的更多历史状态，请使用newdatabasewithcache构造函数。
func NewDatabase(db ethdb.Database) Database {
	return NewDatabaseWithCache(db, 0)
}

//NeXDATA为状态创建后备存储。返回的数据库是安全的
//在内存中同时使用并保留最近扩展的两个trie节点，如
//以及一个大内存缓存中的许多折叠的rlp trie节点。
func NewDatabaseWithCache(db ethdb.Database, cache int) Database {
	csc, _ := lru.New(codeSizeCacheSize)
	return &cachingDB{
		db:            trie.NewDatabaseWithCache(db, cache),
		codeSizeCache: csc,
	}
}

type cachingDB struct {
	db            *trie.Database
	mu            sync.Mutex
	pastTries     []*trie.SecureTrie
	codeSizeCache *lru.Cache
}

//opentrie打开主帐户trie。
func (db *cachingDB) OpenTrie(root common.Hash) (Trie, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	for i := len(db.pastTries) - 1; i >= 0; i-- {
		if db.pastTries[i].Hash() == root {
			return cachedTrie{db.pastTries[i].Copy(), db}, nil
		}
	}
	tr, err := trie.NewSecure(root, db.db, MaxTrieCacheGen)
	if err != nil {
		return nil, err
	}
	return cachedTrie{tr, db}, nil
}

func (db *cachingDB) pushTrie(t *trie.SecureTrie) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if len(db.pastTries) >= maxPastTries {
		copy(db.pastTries, db.pastTries[1:])
		db.pastTries[len(db.pastTries)-1] = t
	} else {
		db.pastTries = append(db.pastTries, t)
	}
}

//openstoragetrie打开帐户的存储trie。
func (db *cachingDB) OpenStorageTrie(addrHash, root common.Hash) (Trie, error) {
	return trie.NewSecure(root, db.db, 0)
}

//copy trie返回给定trie的独立副本。
func (db *cachingDB) CopyTrie(t Trie) Trie {
	switch t := t.(type) {
	case cachedTrie:
		return cachedTrie{t.SecureTrie.Copy(), db}
	case *trie.SecureTrie:
		return t.Copy()
	default:
		panic(fmt.Errorf("unknown trie type %T", t))
	}
}

//ContractCode检索特定合同的代码。
func (db *cachingDB) ContractCode(addrHash, codeHash common.Hash) ([]byte, error) {
	code, err := db.db.Node(codeHash)
	if err == nil {
		db.codeSizeCache.Add(codeHash, len(code))
	}
	return code, err
}

//ContractCodeSize检索特定合同代码的大小。
func (db *cachingDB) ContractCodeSize(addrHash, codeHash common.Hash) (int, error) {
	if cached, ok := db.codeSizeCache.Get(codeHash); ok {
		return cached.(int), nil
	}
	code, err := db.ContractCode(addrHash, codeHash)
	return len(code), err
}

//triedb检索任何中间trie节点缓存层。
func (db *cachingDB) TrieDB() *trie.Database {
	return db.db
}

//cachedtrie在提交时将其trie插入cachingdb。
type cachedTrie struct {
	*trie.SecureTrie
	db *cachingDB
}

func (m cachedTrie) Commit(onleaf trie.LeafCallback) (common.Hash, error) {
	root, err := m.SecureTrie.Commit(onleaf)
	if err == nil {
		m.db.pushTrie(m.SecureTrie)
	}
	return root, err
}

func (m cachedTrie) Prove(key []byte, fromLevel uint, proofDb ethdb.Putter) error {
	return m.SecureTrie.Prove(key, fromLevel, proofDb)
}
