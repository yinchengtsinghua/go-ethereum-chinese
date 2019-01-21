
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
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

//securetrie使用键哈希对trie进行包装。在安全的测试中，所有
//访问操作使用keccak256散列密钥。这防止
//通过创建长的节点链来调用代码
//增加访问时间。
//
//与常规trie相反，只能使用
//新建，必须具有附加的数据库。数据库还存储
//每个键的预映像。
//
//SecureTrie不能同时使用。
type SecureTrie struct {
	trie             Trie
	hashKeyBuf       [common.HashLength]byte
	secKeyCache      map[string][]byte
secKeyCacheOwner *SecureTrie //指向self的指针，不匹配时替换密钥缓存
}

//NewSecure从备份数据库创建具有现有根节点的trie
//以及可选的中间内存节点池。
//
//如果根是空字符串的零哈希或sha3哈希，则
//trie最初是空的。否则，如果db为零，new将恐慌。
//如果找不到根节点，则返回MissingNodeError。
//
//访问trie会根据需要从数据库或节点池加载节点。
//加载的节点将一直保留，直到其“缓存生成”过期。
//每次调用commit都会创建新的缓存生成。
//cachelimit设置要保留的上一代缓存的数量。
func NewSecure(root common.Hash, db *Database, cachelimit uint16) (*SecureTrie, error) {
	if db == nil {
		panic("trie.NewSecure called without a database")
	}
	trie, err := New(root, db)
	if err != nil {
		return nil, err
	}
	trie.SetCacheLimit(cachelimit)
	return &SecureTrie{trie: *trie}, nil
}

//get返回存储在trie中的键的值。
//调用方不能修改值字节。
func (t *SecureTrie) Get(key []byte) []byte {
	res, err := t.TryGet(key)
	if err != nil {
		log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	}
	return res
}

//Tryget返回存储在trie中的键的值。
//调用方不能修改值字节。
//如果在数据库中找不到节点，则返回MissingNodeError。
func (t *SecureTrie) TryGet(key []byte) ([]byte, error) {
	return t.trie.TryGet(t.hashKey(key))
}

//更新trie中的关联键和值。后续呼叫
//get将返回值。如果值的长度为零，则任何现有值
//从trie中删除，调用get将返回nil。
//
//当值字节是
//存储在trie中。
func (t *SecureTrie) Update(key, value []byte) {
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
func (t *SecureTrie) TryUpdate(key, value []byte) error {
	hk := t.hashKey(key)
	err := t.trie.TryUpdate(hk, value)
	if err != nil {
		return err
	}
	t.getSecKeyCache()[string(hk)] = common.CopyBytes(key)
	return nil
}

//删除从trie中删除键的任何现有值。
func (t *SecureTrie) Delete(key []byte) {
	if err := t.TryDelete(key); err != nil {
		log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	}
}

//Trydelete从trie中删除键的任何现有值。
//如果在数据库中找不到节点，则返回MissingNodeError。
func (t *SecureTrie) TryDelete(key []byte) error {
	hk := t.hashKey(key)
	delete(t.getSecKeyCache(), string(hk))
	return t.trie.TryDelete(hk)
}

//getkey返回哈希键的sha3 preimage
//以前用于存储值。
func (t *SecureTrie) GetKey(shaKey []byte) []byte {
	if key, ok := t.getSecKeyCache()[string(shaKey)]; ok {
		return key
	}
	key, _ := t.trie.db.preimage(common.BytesToHash(shaKey))
	return key
}

//commit将所有节点和安全哈希预映像写入trie的数据库。
//节点以其sha3散列作为密钥存储。
//
//提交将刷新内存中的节点。后续的get调用将加载节点
//从数据库中。
func (t *SecureTrie) Commit(onleaf LeafCallback) (root common.Hash, err error) {
//将所有预映像写入实际磁盘数据库
	if len(t.getSecKeyCache()) > 0 {
		t.trie.db.lock.Lock()
		for hk, key := range t.secKeyCache {
			t.trie.db.insertPreimage(common.BytesToHash([]byte(hk)), key)
		}
		t.trie.db.lock.Unlock()

		t.secKeyCache = make(map[string][]byte)
	}
//将trie提交到其中间节点数据库
	return t.trie.Commit(onleaf)
}

//hash返回securetrie的根哈希。它不会写信给
//即使trie没有数据库也可以使用。
func (t *SecureTrie) Hash() common.Hash {
	return t.trie.Hash()
}

//root返回securetrie的根哈希。
//已弃用：请改用哈希。
func (t *SecureTrie) Root() []byte {
	return t.trie.Root()
}

//copy返回securetrie的副本。
func (t *SecureTrie) Copy() *SecureTrie {
	cpy := *t
	return &cpy
}

//nodeiterator返回返回底层trie节点的迭代器。迭代
//从给定的开始键之后的键开始。
func (t *SecureTrie) NodeIterator(start []byte) NodeIterator {
	return t.trie.NodeIterator(start)
}

//hash key返回作为临时缓冲区的键的哈希。
//调用方不能保留返回值，因为它将成为
//下次调用hashkey或seckey时无效。
func (t *SecureTrie) hashKey(key []byte) []byte {
	h := newHasher(0, 0, nil)
	h.sha.Reset()
	h.sha.Write(key)
	buf := h.sha.Sum(t.hashKeyBuf[:0])
	returnHasherToPool(h)
	return buf
}

//GetSecKeyCache返回当前的安全密钥缓存，如果
//所有权已更改（即当前安全的trie是另一个拥有者的副本
//实际缓存）。
func (t *SecureTrie) getSecKeyCache() map[string][]byte {
	if t != t.secKeyCacheOwner {
		t.secKeyCacheOwner = t
		t.secKeyCache = make(map[string][]byte)
	}
	return t.secKeyCache
}
