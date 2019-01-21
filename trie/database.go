
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/allegro/bigcache"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	memcacheCleanHitMeter   = metrics.NewRegisteredMeter("trie/memcache/clean/hit", nil)
	memcacheCleanMissMeter  = metrics.NewRegisteredMeter("trie/memcache/clean/miss", nil)
	memcacheCleanReadMeter  = metrics.NewRegisteredMeter("trie/memcache/clean/read", nil)
	memcacheCleanWriteMeter = metrics.NewRegisteredMeter("trie/memcache/clean/write", nil)

	memcacheFlushTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/flush/time", nil)
	memcacheFlushNodesMeter = metrics.NewRegisteredMeter("trie/memcache/flush/nodes", nil)
	memcacheFlushSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/flush/size", nil)

	memcacheGCTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/gc/time", nil)
	memcacheGCNodesMeter = metrics.NewRegisteredMeter("trie/memcache/gc/nodes", nil)
	memcacheGCSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/gc/size", nil)

	memcacheCommitTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/commit/time", nil)
	memcacheCommitNodesMeter = metrics.NewRegisteredMeter("trie/memcache/commit/nodes", nil)
	memcacheCommitSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/commit/size", nil)
)

//SecureKeyPrefix是用于存储trie节点预映像的数据库密钥前缀。
var secureKeyPrefix = []byte("secure-key-")

//SecureKeyLength是上述前缀的长度+32字节哈希。
const secureKeyLength = 11 + 32

//DatabaseReader包装get并具有trie的后备存储方法。
type DatabaseReader interface {
//get从数据库中检索与键关联的值。
	Get(key []byte) (value []byte, err error)

//获取数据库中是否存在键。
	Has(key []byte) (bool, error)
}

//数据库是Trie数据结构和
//磁盘数据库。其目的是在内存中积累trie写入
//定期刷新一对夫妇试图磁盘，垃圾收集剩余。
type Database struct {
diskdb ethdb.Database //成熟trie节点的持久存储

cleans  *bigcache.BigCache          //干净节点rlps的GC友好内存缓存
dirties map[common.Hash]*cachedNode //脏节点的数据和引用关系
oldest  common.Hash                 //最旧的跟踪节点，刷新列表头
newest  common.Hash                 //最新跟踪节点，刷新列表尾部

preimages map[common.Hash][]byte //安全trie中节点的预映像
seckeybuf [secureKeyLength]byte  //用于计算预映像密钥的临时缓冲区

gctime  time.Duration      //自上次提交以来在垃圾收集上花费的时间
gcnodes uint64             //自上次提交以来收集的节点垃圾
gcsize  common.StorageSize //自上次提交以来收集的数据存储垃圾

flushtime  time.Duration      //自上次提交以来在数据刷新上花费的时间
flushnodes uint64             //自上次提交以来刷新的节点
flushsize  common.StorageSize //自上次提交以来刷新的数据存储

dirtiesSize   common.StorageSize //脏节点缓存的存储大小（不包括FlushList）
preimagesSize common.StorageSize //预映像缓存的存储大小

	lock sync.RWMutex
}

//rawnode是一个简单的二进制blob，用于区分折叠的trie
//节点和已经编码的rlp二进制blob（同时存储它们
//在相同的缓存字段中）。
type rawNode []byte

func (n rawNode) canUnload(uint16, uint16) bool { panic("this should never end up in a live trie") }
func (n rawNode) cache() (hashNode, bool)       { panic("this should never end up in a live trie") }
func (n rawNode) fstring(ind string) string     { panic("this should never end up in a live trie") }

//rawfullnode仅表示完整节点的有用数据内容，其中
//去除缓存和标志以最小化其数据存储。这类荣誉
//与原始父级相同的RLP编码。
type rawFullNode [17]node

func (n rawFullNode) canUnload(uint16, uint16) bool { panic("this should never end up in a live trie") }
func (n rawFullNode) cache() (hashNode, bool)       { panic("this should never end up in a live trie") }
func (n rawFullNode) fstring(ind string) string     { panic("this should never end up in a live trie") }

func (n rawFullNode) EncodeRLP(w io.Writer) error {
	var nodes [17]node

	for i, child := range n {
		if child != nil {
			nodes[i] = child
		} else {
			nodes[i] = nilValueNode
		}
	}
	return rlp.Encode(w, nodes)
}

//rawshortnode仅表示短节点的有用数据内容，其中
//去除缓存和标志以最小化其数据存储。这类荣誉
//与原始父级相同的RLP编码。
type rawShortNode struct {
	Key []byte
	Val node
}

func (n rawShortNode) canUnload(uint16, uint16) bool { panic("this should never end up in a live trie") }
func (n rawShortNode) cache() (hashNode, bool)       { panic("this should never end up in a live trie") }
func (n rawShortNode) fstring(ind string) string     { panic("this should never end up in a live trie") }

//cached node是我们所知道的关于
//内存数据库写入层。
type cachedNode struct {
node node   //缓存的折叠的trie节点或原始rlp数据
size uint16 //有用缓存数据的字节大小

parents  uint32                 //引用此节点的活动节点数
children map[common.Hash]uint16 //此节点引用的外部子级

flushPrev common.Hash //刷新列表中的上一个节点
flushNext common.Hash //刷新列表中的下一个节点
}

//rlp返回缓存节点的原始rlp编码blob，可以直接从
//缓存，或者从折叠的节点重新生成缓存。
func (n *cachedNode) rlp() []byte {
	if node, ok := n.node.(rawNode); ok {
		return node
	}
	blob, err := rlp.EncodeToBytes(n.node)
	if err != nil {
		panic(err)
	}
	return blob
}

//obj直接从缓存返回解码和扩展的trie节点，
//或者从rlp编码的blob中重新生成它。
func (n *cachedNode) obj(hash common.Hash, cachegen uint16) node {
	if node, ok := n.node.(rawNode); ok {
		return mustDecodeNode(hash[:], node, cachegen)
	}
	return expandNode(hash[:], n.node, cachegen)
}

//childs返回此节点的所有被跟踪子节点，包括隐式子节点
//从节点内部以及节点外部的显式节点。
func (n *cachedNode) childs() []common.Hash {
	children := make([]common.Hash, 0, 16)
	for child := range n.children {
		children = append(children, child)
	}
	if _, ok := n.node.(rawNode); !ok {
		gatherChildren(n.node, &children)
	}
	return children
}

//GatherChildren遍历折叠存储节点的节点层次结构，并
//检索所有哈希节点子级。
func gatherChildren(n node, children *[]common.Hash) {
	switch n := n.(type) {
	case *rawShortNode:
		gatherChildren(n.Val, children)

	case rawFullNode:
		for i := 0; i < 16; i++ {
			gatherChildren(n[i], children)
		}
	case hashNode:
		*children = append(*children, common.BytesToHash(n))

	case valueNode, nil:

	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

//SimplifyNode遍历扩展内存节点的层次结构并丢弃
//所有内部缓存，返回只包含原始数据的节点。
func simplifyNode(n node) node {
	switch n := n.(type) {
	case *shortNode:
//短节点丢弃标志并层叠
		return &rawShortNode{Key: n.Key, Val: simplifyNode(n.Val)}

	case *fullNode:
//完整节点丢弃标志并层叠
		node := rawFullNode(n.Children)
		for i := 0; i < len(node); i++ {
			if node[i] != nil {
				node[i] = simplifyNode(node[i])
			}
		}
		return node

	case valueNode, hashNode, rawNode:
		return n

	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

//expandnode遍历折叠存储节点的节点层次结构并转换
//将所有字段和键转换为扩展内存形式。
func expandNode(hash hashNode, n node, cachegen uint16) node {
	switch n := n.(type) {
	case *rawShortNode:
//短节点需要密钥和子扩展
		return &shortNode{
			Key: compactToHex(n.Key),
			Val: expandNode(nil, n.Val, cachegen),
			flags: nodeFlag{
				hash: hash,
				gen:  cachegen,
			},
		}

	case rawFullNode:
//完整节点需要子扩展
		node := &fullNode{
			flags: nodeFlag{
				hash: hash,
				gen:  cachegen,
			},
		}
		for i := 0; i < len(node.Children); i++ {
			if n[i] != nil {
				node.Children[i] = expandNode(nil, n[i], cachegen)
			}
		}
		return node

	case valueNode, hashNode:
		return n

	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

//new database创建一个新的trie数据库来存储之前的临时trie内容
//它被写入磁盘或垃圾收集。没有创建读缓存，因此
//数据检索将命中基础磁盘数据库。
func NewDatabase(diskdb ethdb.Database) *Database {
	return NewDatabaseWithCache(diskdb, 0)
}

//newdatabasewithcache创建新的trie数据库以存储临时trie内容
//在写入磁盘或垃圾收集之前。它还充当读缓存
//用于从磁盘加载的节点。
func NewDatabaseWithCache(diskdb ethdb.Database, cache int) *Database {
	var cleans *bigcache.BigCache
	if cache > 0 {
		cleans, _ = bigcache.NewBigCache(bigcache.Config{
			Shards:             1024,
			LifeWindow:         time.Hour,
			MaxEntriesInWindow: cache * 1024,
			MaxEntrySize:       512,
			HardMaxCacheSize:   cache,
		})
	}
	return &Database{
		diskdb:    diskdb,
		cleans:    cleans,
		dirties:   map[common.Hash]*cachedNode{{}: {}},
		preimages: make(map[common.Hash][]byte),
	}
}

//diskdb检索支持trie数据库的持久存储。
func (db *Database) DiskDB() DatabaseReader {
	return db.diskdb
}

//insertblob将新的引用跟踪blob写入内存数据库
//但未知。此方法只能用于需要
//引用计数，因为trie节点直接通过
//他们的孩子。
func (db *Database) InsertBlob(hash common.Hash, blob []byte) {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.insert(hash, blob, rawNode(blob))
}

//插入将折叠的trie节点插入内存数据库。这种方法是
//更通用的insertblob版本，支持原始blob插入
//例如三节点插入。必须始终指定blob以允许
//尺寸跟踪。
func (db *Database) insert(hash common.Hash, blob []byte, node node) {
//如果节点已缓存，则跳过
	if _, ok := db.dirties[hash]; ok {
		return
	}
//为此节点创建缓存项
	entry := &cachedNode{
		node:      simplifyNode(node),
		size:      uint16(len(blob)),
		flushPrev: db.newest,
	}
	for _, child := range entry.childs() {
		if c := db.dirties[child]; c != nil {
			c.parents++
		}
	}
	db.dirties[hash] = entry

//更新刷新列表端点
	if db.oldest == (common.Hash{}) {
		db.oldest, db.newest = hash, hash
	} else {
		db.dirties[db.newest].flushNext, db.newest = hash, hash
	}
	db.dirtiesSize += common.StorageSize(common.HashLength + entry.size)
}

//insertpreimage将新的trie节点pre映像写入内存数据库（如果是）
//但未知。该方法将复制切片。
//
//注意，此方法假定数据库的锁被持有！
func (db *Database) insertPreimage(hash common.Hash, preimage []byte) {
	if _, ok := db.preimages[hash]; ok {
		return
	}
	db.preimages[hash] = common.CopyBytes(preimage)
	db.preimagesSize += common.StorageSize(common.HashLength + len(preimage))
}

//节点从内存中检索缓存的trie节点，或者如果没有节点，则返回nil
//在内存缓存中找到。
func (db *Database) node(hash common.Hash, cachegen uint16) node {
//从干净缓存中检索节点（如果可用）
	if db.cleans != nil {
		if enc, err := db.cleans.Get(string(hash[:])); err == nil && enc != nil {
			memcacheCleanHitMeter.Mark(1)
			memcacheCleanReadMeter.Mark(int64(len(enc)))
			return mustDecodeNode(hash[:], enc, cachegen)
		}
	}
//从脏缓存中检索节点（如果可用）
	db.lock.RLock()
	dirty := db.dirties[hash]
	db.lock.RUnlock()

	if dirty != nil {
		return dirty.obj(hash, cachegen)
	}
//内容在内存中不可用，请尝试从磁盘检索
	enc, err := db.diskdb.Get(hash[:])
	if err != nil || enc == nil {
		return nil
	}
	if db.cleans != nil {
		db.cleans.Set(string(hash[:]), enc)
		memcacheCleanMissMeter.Mark(1)
		memcacheCleanWriteMeter.Mark(int64(len(enc)))
	}
	return mustDecodeNode(hash[:], enc, cachegen)
}

//节点从内存中检索编码缓存的trie节点。如果找不到
//缓存后，该方法查询持久数据库中的内容。
func (db *Database) Node(hash common.Hash) ([]byte, error) {
//从干净缓存中检索节点（如果可用）
	if db.cleans != nil {
		if enc, err := db.cleans.Get(string(hash[:])); err == nil && enc != nil {
			memcacheCleanHitMeter.Mark(1)
			memcacheCleanReadMeter.Mark(int64(len(enc)))
			return enc, nil
		}
	}
//从脏缓存中检索节点（如果可用）
	db.lock.RLock()
	dirty := db.dirties[hash]
	db.lock.RUnlock()

	if dirty != nil {
		return dirty.rlp(), nil
	}
//内容在内存中不可用，请尝试从磁盘检索
	enc, err := db.diskdb.Get(hash[:])
	if err == nil && enc != nil {
		if db.cleans != nil {
			db.cleans.Set(string(hash[:]), enc)
			memcacheCleanMissMeter.Mark(1)
			memcacheCleanWriteMeter.Mark(int64(len(enc)))
		}
	}
	return enc, err
}

//pre image从内存中检索缓存的trie节点pre映像。如果不能
//找到缓存的，该方法查询持久数据库中的内容。
func (db *Database) preimage(hash common.Hash) ([]byte, error) {
//从缓存中检索节点（如果可用）
	db.lock.RLock()
	preimage := db.preimages[hash]
	db.lock.RUnlock()

	if preimage != nil {
		return preimage, nil
	}
//内容在内存中不可用，请尝试从磁盘检索
	return db.diskdb.Get(db.secureKey(hash[:]))
}

//securekey返回密钥的预映像的数据库密钥，作为临时的
//缓冲器。调用方不能保留返回值，因为它将成为
//下次呼叫无效。
func (db *Database) secureKey(key []byte) []byte {
	buf := append(db.seckeybuf[:0], secureKeyPrefix...)
	buf = append(buf, key...)
	return buf
}

//节点检索内存数据库中缓存的所有节点的哈希值。
//此方法非常昂贵，只应用于验证内部
//测试代码中的状态。
func (db *Database) Nodes() []common.Hash {
	db.lock.RLock()
	defer db.lock.RUnlock()

	var hashes = make([]common.Hash, 0, len(db.dirties))
	for hash := range db.dirties {
if hash != (common.Hash{}) { //“根”引用/节点的特殊情况
			hashes = append(hashes, hash)
		}
	}
	return hashes
}

//引用将新引用从父节点添加到子节点。
func (db *Database) Reference(child common.Hash, parent common.Hash) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	db.reference(child, parent)
}

//引用是引用的私有锁定版本。
func (db *Database) reference(child common.Hash, parent common.Hash) {
//如果节点不存在，它是从磁盘中提取的节点，跳过
	node, ok := db.dirties[child]
	if !ok {
		return
	}
//如果引用已存在，则只为根复制
	if db.dirties[parent].children == nil {
		db.dirties[parent].children = make(map[common.Hash]uint16)
	} else if _, ok = db.dirties[parent].children[child]; ok && parent != (common.Hash{}) {
		return
	}
	node.parents++
	db.dirties[parent].children[child]++
}

//取消引用从根节点删除现有引用。
func (db *Database) Dereference(root common.Hash) {
//健全性检查以确保元根目录未被删除
	if root == (common.Hash{}) {
		log.Error("Attempted to dereference the trie cache meta root")
		return
	}
	db.lock.Lock()
	defer db.lock.Unlock()

	nodes, storage, start := len(db.dirties), db.dirtiesSize, time.Now()
	db.dereference(root, common.Hash{})

	db.gcnodes += uint64(nodes - len(db.dirties))
	db.gcsize += storage - db.dirtiesSize
	db.gctime += time.Since(start)

	memcacheGCTimeTimer.Update(time.Since(start))
	memcacheGCSizeMeter.Mark(int64(storage - db.dirtiesSize))
	memcacheGCNodesMeter.Mark(int64(nodes - len(db.dirties)))

	log.Debug("Dereferenced trie from memory database", "nodes", nodes-len(db.dirties), "size", storage-db.dirtiesSize, "time", time.Since(start),
		"gcnodes", db.gcnodes, "gcsize", db.gcsize, "gctime", db.gctime, "livenodes", len(db.dirties), "livesize", db.dirtiesSize)
}

//解引用是解引用的私有锁定版本。
func (db *Database) dereference(child common.Hash, parent common.Hash) {
//取消对父子关系的引用
	node := db.dirties[parent]

	if node.children != nil && node.children[child] > 0 {
		node.children[child]--
		if node.children[child] == 0 {
			delete(node.children, child)
		}
	}
//如果子节点不存在，则它是以前提交的节点。
	node, ok := db.dirties[child]
	if !ok {
		return
	}
//如果没有对该子级的引用，请将其删除并层叠
	if node.parents > 0 {
//这是一种特殊的情况，其中从磁盘加载的节点（即不在
//memcache）作为新节点（短节点拆分为完整节点，
//然后恢复为短），导致缓存节点没有父节点。那就是
//这本身没问题，但不要让最大的父母离开。
		node.parents--
	}
	if node.parents == 0 {
//从刷新列表中删除节点
		switch child {
		case db.oldest:
			db.oldest = node.flushNext
			db.dirties[node.flushNext].flushPrev = common.Hash{}
		case db.newest:
			db.newest = node.flushPrev
			db.dirties[node.flushPrev].flushNext = common.Hash{}
		default:
			db.dirties[node.flushPrev].flushNext = node.flushNext
			db.dirties[node.flushNext].flushPrev = node.flushPrev
		}
//取消引用所有子节点并删除节点
		for _, hash := range node.childs() {
			db.dereference(hash, child)
		}
		delete(db.dirties, child)
		db.dirtiesSize -= common.StorageSize(common.HashLength + int(node.size))
	}
}

//cap循环刷新旧的但仍引用的trie节点，直到总数
//内存使用率低于给定阈值。
func (db *Database) Cap(limit common.StorageSize) error {
//创建一个数据库批处理以将持久性数据清除。重要的是
//外部代码没有看到不一致的状态（引用的数据从
//提交期间内存缓存，但尚未在持久存储中）。这是保证的。
//只有在数据库写入完成时才取消对现有数据的缓存。
	db.lock.RLock()

	nodes, storage, start := len(db.dirties), db.dirtiesSize, time.Now()
	batch := db.diskdb.NewBatch()

//db.dirtiessize只包含缓存中的有用数据，但在报告时
//总的内存消耗，维护元数据也需要
//计数。对于每个有用的节点，我们跟踪2个额外的散列作为flushlist。
	size := db.dirtiesSize + common.StorageSize((len(db.dirties)-1)*2*common.HashLength)

//如果预映像缓存足够大，请推到磁盘。如果它还小的话
//留待以后删除重复写入。
	flushPreimages := db.preimagesSize > 4*1024*1024
	if flushPreimages {
		for hash, preimage := range db.preimages {
			if err := batch.Put(db.secureKey(hash[:]), preimage); err != nil {
				log.Error("Failed to commit preimage from trie database", "err", err)
				db.lock.RUnlock()
				return err
			}
			if batch.ValueSize() > ethdb.IdealBatchSize {
				if err := batch.Write(); err != nil {
					db.lock.RUnlock()
					return err
				}
				batch.Reset()
			}
		}
	}
//保持提交刷新列表中的节点，直到低于允许值为止
	oldest := db.oldest
	for size > limit && oldest != (common.Hash{}) {
//获取最旧的引用节点并推入批处理
		node := db.dirties[oldest]
		if err := batch.Put(oldest[:], node.rlp()); err != nil {
			db.lock.RUnlock()
			return err
		}
//如果超出了理想的批处理大小，请提交并重置
		if batch.ValueSize() >= ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				log.Error("Failed to write flush list to disk", "err", err)
				db.lock.RUnlock()
				return err
			}
			batch.Reset()
		}
//迭代到下一个刷新项，或者在达到大小上限时中止。尺寸
//是总大小，包括有用的缓存数据（hash->blob），作为
//以及flushlist元数据（2*hash）。从缓存刷新项目时，
//我们需要两者都减少。
		size -= common.StorageSize(3*common.HashLength + int(node.size))
		oldest = node.flushNext
	}
//从上一批中清除所有剩余数据
	if err := batch.Write(); err != nil {
		log.Error("Failed to write flush list to disk", "err", err)
		db.lock.RUnlock()
		return err
	}
	db.lock.RUnlock()

//写入成功，清除刷新的数据
	db.lock.Lock()
	defer db.lock.Unlock()

	if flushPreimages {
		db.preimages = make(map[common.Hash][]byte)
		db.preimagesSize = 0
	}
	for db.oldest != oldest {
		node := db.dirties[db.oldest]
		delete(db.dirties, db.oldest)
		db.oldest = node.flushNext

		db.dirtiesSize -= common.StorageSize(common.HashLength + int(node.size))
	}
	if db.oldest != (common.Hash{}) {
		db.dirties[db.oldest].flushPrev = common.Hash{}
	}
	db.flushnodes += uint64(nodes - len(db.dirties))
	db.flushsize += storage - db.dirtiesSize
	db.flushtime += time.Since(start)

	memcacheFlushTimeTimer.Update(time.Since(start))
	memcacheFlushSizeMeter.Mark(int64(storage - db.dirtiesSize))
	memcacheFlushNodesMeter.Mark(int64(nodes - len(db.dirties)))

	log.Debug("Persisted nodes from memory database", "nodes", nodes-len(db.dirties), "size", storage-db.dirtiesSize, "time", time.Since(start),
		"flushnodes", db.flushnodes, "flushsize", db.flushsize, "flushtime", db.flushtime, "livenodes", len(db.dirties), "livesize", db.dirtiesSize)

	return nil
}

//commit迭代特定节点的所有子节点，并将其写出
//在磁盘上，强制删除两个方向上的所有引用。
//
//作为一个副作用，所有的预图像积累到这一点也写。
func (db *Database) Commit(node common.Hash, report bool) error {
//创建一个数据库批处理以将持久性数据清除。重要的是
//外部代码没有看到不一致的状态（引用的数据从
//提交期间内存缓存，但尚未在持久存储中）。这是保证的。
//只有在数据库写入完成时才取消对现有数据的缓存。
	db.lock.RLock()

	start := time.Now()
	batch := db.diskdb.NewBatch()

//将所有累积的预映像移到一个写入批处理中
	for hash, preimage := range db.preimages {
		if err := batch.Put(db.secureKey(hash[:]), preimage); err != nil {
			log.Error("Failed to commit preimage from trie database", "err", err)
			db.lock.RUnlock()
			return err
		}
		if batch.ValueSize() > ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				return err
			}
			batch.Reset()
		}
	}
//将trie本身移到批处理中，如果积累了足够的数据，则进行刷新。
	nodes, storage := len(db.dirties), db.dirtiesSize
	if err := db.commit(node, batch); err != nil {
		log.Error("Failed to commit trie from trie database", "err", err)
		db.lock.RUnlock()
		return err
	}
//编写批处理就绪，在持久性期间为读卡器解锁
	if err := batch.Write(); err != nil {
		log.Error("Failed to write trie to disk", "err", err)
		db.lock.RUnlock()
		return err
	}
	db.lock.RUnlock()

//写入成功，清除刷新的数据
	db.lock.Lock()
	defer db.lock.Unlock()

	db.preimages = make(map[common.Hash][]byte)
	db.preimagesSize = 0

	db.uncache(node)

	memcacheCommitTimeTimer.Update(time.Since(start))
	memcacheCommitSizeMeter.Mark(int64(storage - db.dirtiesSize))
	memcacheCommitNodesMeter.Mark(int64(nodes - len(db.dirties)))

	logger := log.Info
	if !report {
		logger = log.Debug
	}
	logger("Persisted trie from memory database", "nodes", nodes-len(db.dirties)+int(db.flushnodes), "size", storage-db.dirtiesSize+db.flushsize, "time", time.Since(start)+db.flushtime,
		"gcnodes", db.gcnodes, "gcsize", db.gcsize, "gctime", db.gctime, "livenodes", len(db.dirties), "livesize", db.dirtiesSize)

//重置垃圾收集统计信息
	db.gcnodes, db.gcsize, db.gctime = 0, 0, 0
	db.flushnodes, db.flushsize, db.flushtime = 0, 0, 0

	return nil
}

//commit是commit的私有锁定版本。
func (db *Database) commit(hash common.Hash, batch ethdb.Batch) error {
//如果该节点不存在，则它是以前提交的节点
	node, ok := db.dirties[hash]
	if !ok {
		return nil
	}
	for _, child := range node.childs() {
		if err := db.commit(child, batch); err != nil {
			return err
		}
	}
	if err := batch.Put(hash[:], node.rlp()); err != nil {
		return err
	}
//如果我们已经达到了最佳的批处理大小，请提交并重新开始
	if batch.ValueSize() >= ethdb.IdealBatchSize {
		if err := batch.Write(); err != nil {
			return err
		}
		batch.Reset()
	}
	return nil
}

//uncache是提交操作的后处理步骤，其中
//保留的trie将从缓存中删除。两阶段背后的原因
//提交是为了在从内存移动时确保一致的数据可用性。
//到磁盘。
func (db *Database) uncache(hash common.Hash) {
//如果节点不存在，我们就在这条路径上完成了
	node, ok := db.dirties[hash]
	if !ok {
		return
	}
//节点仍然存在，请将其从刷新列表中删除
	switch hash {
	case db.oldest:
		db.oldest = node.flushNext
		db.dirties[node.flushNext].flushPrev = common.Hash{}
	case db.newest:
		db.newest = node.flushPrev
		db.dirties[node.flushPrev].flushNext = common.Hash{}
	default:
		db.dirties[node.flushPrev].flushNext = node.flushNext
		db.dirties[node.flushNext].flushPrev = node.flushPrev
	}
//打开节点的子文件并移除节点本身
	for _, child := range node.childs() {
		db.uncache(child)
	}
	delete(db.dirties, hash)
	db.dirtiesSize -= common.StorageSize(common.HashLength + int(node.size))
}

//大小返回在
//持久数据库层。
func (db *Database) Size() (common.StorageSize, common.StorageSize) {
	db.lock.RLock()
	defer db.lock.RUnlock()

//db.dirtiessize只包含缓存中的有用数据，但在报告时
//总的内存消耗，维护元数据也需要
//计数。对于每个有用的节点，我们跟踪2个额外的散列作为flushlist。
	var flushlistSize = common.StorageSize((len(db.dirties) - 1) * 2 * common.HashLength)
	return db.dirtiesSize + flushlistSize, db.preimagesSize
}

//VerifyIntegrity是一种调试方法，用于在存储在
//内存并检查每个节点是否可以从元根访问。目标
//查找可能导致内存泄漏和/或trie节点丢失的任何错误
//失踪。
//
//这种方法非常占用CPU和内存，只能在必要时使用。
func (db *Database) verifyIntegrity() {
//循环访问所有缓存节点并将它们累积到一个集合中
	reachable := map[common.Hash]struct{}{{}: {}}

	for child := range db.dirties[common.Hash{}].children {
		db.accumulate(child, reachable)
	}
//查找任何不可访问但缓存的节点
	unreachable := []string{}
	for hash, node := range db.dirties {
		if _, ok := reachable[hash]; !ok {
			unreachable = append(unreachable, fmt.Sprintf("%x: {Node: %v, Parents: %d, Prev: %x, Next: %x}",
				hash, node.node, node.parents, node.flushPrev, node.flushNext))
		}
	}
	if len(unreachable) != 0 {
		panic(fmt.Sprintf("trie cache memory leak: %v", unreachable))
	}
}

//对哈希定义的trie进行累加迭代，并将所有
//在内存中找到缓存的子级。
func (db *Database) accumulate(hash common.Hash, reachable map[common.Hash]struct{}) {
//将节点标记为可访问（如果存在于内存缓存中）
	node, ok := db.dirties[hash]
	if !ok {
		return
	}
	reachable[hash] = struct{}{}

//遍历所有子级并对其进行累积
	for _, child := range node.childs() {
		db.accumulate(child, reachable)
	}
}
