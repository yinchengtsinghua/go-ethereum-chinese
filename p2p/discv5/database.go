
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

//包含节点数据库，存储以前看到的节点和收集的所有
//用于QoS的元数据。

package discv5

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/util"
)

var (
nodeDBNilNodeID      = NodeID{}       //要用作nil元素的特殊节点ID。
nodeDBNodeExpiration = 24 * time.Hour //删除未查看节点的时间。
nodeDBCleanupCycle   = time.Hour      //运行过期任务的时间段。
)

//nodedb存储我们知道的所有节点。
type nodeDB struct {
lvl    *leveldb.DB   //与数据库本身的接口
self   NodeID        //拥有节点ID以防止将其添加到数据库中
runner sync.Once     //确保我们最多可以启动一个到期日
quit   chan struct{} //向过期线程发出停止信号的通道
}

//节点数据库的架构布局
var (
nodeDBVersionKey = []byte("version") //更改时要刷新的数据库版本
nodeDBItemPrefix = []byte("n:")      //为节点条目加前缀的标识符

	nodeDBDiscoverRoot          = ":discover"
	nodeDBDiscoverPing          = nodeDBDiscoverRoot + ":lastping"
	nodeDBDiscoverPong          = nodeDBDiscoverRoot + ":lastpong"
	nodeDBDiscoverFindFails     = nodeDBDiscoverRoot + ":findfail"
	nodeDBDiscoverLocalEndpoint = nodeDBDiscoverRoot + ":localendpoint"
	nodeDBTopicRegTickets       = ":tickets"
)

//newnodedb创建一个新的节点数据库，用于存储和检索有关的信息
//网络中的已知对等点。如果没有给定路径，则内存中的
//数据库已构建。
func newNodeDB(path string, version int, self NodeID) (*nodeDB, error) {
	if path == "" {
		return newMemoryNodeDB(self)
	}
	return newPersistentNodeDB(path, version, self)
}

//newmemorynodedb创建一个新的内存节点数据库，而不使用持久的
//后端。
func newMemoryNodeDB(self NodeID) (*nodeDB, error) {
	db, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		return nil, err
	}
	return &nodeDB{
		lvl:  db,
		self: self,
		quit: make(chan struct{}),
	}, nil
}

//newpersistentnodedb创建/打开一个支持leveldb的持久节点数据库，
//同时在版本不匹配的情况下刷新其内容。
func newPersistentNodeDB(path string, version int, self NodeID) (*nodeDB, error) {
	opts := &opt.Options{OpenFilesCacheCapacity: 5}
	db, err := leveldb.OpenFile(path, opts)
	if _, iscorrupted := err.(*errors.ErrCorrupted); iscorrupted {
		db, err = leveldb.RecoverFile(path, nil)
	}
	if err != nil {
		return nil, err
	}
//缓存中包含的节点对应于某个协议版本。
//如果版本不匹配，则刷新所有节点。
	currentVer := make([]byte, binary.MaxVarintLen64)
	currentVer = currentVer[:binary.PutVarint(currentVer, int64(version))]

	blob, err := db.Get(nodeDBVersionKey, nil)
	switch err {
	case leveldb.ErrNotFound:
//找不到版本（即空缓存），请将其插入
		if err := db.Put(nodeDBVersionKey, currentVer, nil); err != nil {
			db.Close()
			return nil, err
		}

	case nil:
//存在版本，如果不同则刷新
		if !bytes.Equal(blob, currentVer) {
			db.Close()
			if err = os.RemoveAll(path); err != nil {
				return nil, err
			}
			return newPersistentNodeDB(path, version, self)
		}
	}
	return &nodeDB{
		lvl:  db,
		self: self,
		quit: make(chan struct{}),
	}, nil
}

//makekey从节点ID及其特定的
//感兴趣的领域。
func makeKey(id NodeID, field string) []byte {
	if bytes.Equal(id[:], nodeDBNilNodeID[:]) {
		return []byte(field)
	}
	return append(nodeDBItemPrefix, append(id[:], field...)...)
}

//SplitKey尝试将数据库键拆分为节点ID和字段部分。
func splitKey(key []byte) (id NodeID, field string) {
//如果键不是节点的，则直接返回
	if !bytes.HasPrefix(key, nodeDBItemPrefix) {
		return NodeID{}, string(key)
	}
//否则拆分ID和字段
	item := key[len(nodeDBItemPrefix):]
	copy(id[:], item[:len(id)])
	field = string(item[len(id):])

	return id, field
}

//fetchin64检索与特定
//数据库密钥。
func (db *nodeDB) fetchInt64(key []byte) int64 {
	blob, err := db.lvl.Get(key, nil)
	if err != nil {
		return 0
	}
	val, read := binary.Varint(blob)
	if read <= 0 {
		return 0
	}
	return val
}

//storeInt64将特定数据库条目作为
//UNIX时间戳。
func (db *nodeDB) storeInt64(key []byte, n int64) error {
	blob := make([]byte, binary.MaxVarintLen64)
	blob = blob[:binary.PutVarint(blob, n)]
	return db.lvl.Put(key, blob, nil)
}

func (db *nodeDB) storeRLP(key []byte, val interface{}) error {
	blob, err := rlp.EncodeToBytes(val)
	if err != nil {
		return err
	}
	return db.lvl.Put(key, blob, nil)
}

func (db *nodeDB) fetchRLP(key []byte, val interface{}) error {
	blob, err := db.lvl.Get(key, nil)
	if err != nil {
		return err
	}
	err = rlp.DecodeBytes(blob, val)
	if err != nil {
		log.Warn(fmt.Sprintf("key %x (%T) %v", key, val, err))
	}
	return err
}

//节点从数据库中检索具有给定ID的节点。
func (db *nodeDB) node(id NodeID) *Node {
	var node Node
	if err := db.fetchRLP(makeKey(id, nodeDBDiscoverRoot), &node); err != nil {
		return nil
	}
	node.sha = crypto.Keccak256Hash(node.ID[:])
	return &node
}

//updateNode将节点插入到对等数据库中（可能会覆盖）。
func (db *nodeDB) updateNode(node *Node) error {
	return db.storeRLP(makeKey(node.ID, nodeDBDiscoverRoot), node)
}

//删除节点删除与节点关联的所有信息/键。
func (db *nodeDB) deleteNode(id NodeID) error {
	deleter := db.lvl.NewIterator(util.BytesPrefix(makeKey(id, "")), nil)
	for deleter.Next() {
		if err := db.lvl.Delete(deleter.Key(), nil); err != nil {
			return err
		}
	}
	return nil
}

//EnsureExpirer是一个小助手方法，可确保数据过期
//机制正在运行。如果到期goroutine已经在运行，则
//方法只返回。
//
//目标是在网络成功后才开始数据疏散
//引导自身（以防止转储可能有用的种子节点）。自从
//准确跟踪第一个成功的
//收敛性，在适当的时候“确保”正确的状态比较简单
//条件发生（即成功结合），并丢弃进一步的事件。
func (db *nodeDB) ensureExpirer() {
	db.runner.Do(func() { go db.expirer() })
}

//Expirer应该在Go例程中启动，并负责循环广告
//无穷大并从数据库中删除过时数据。
func (db *nodeDB) expirer() {
	tick := time.NewTicker(nodeDBCleanupCycle)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if err := db.expireNodes(); err != nil {
				log.Error(fmt.Sprintf("Failed to expire nodedb items: %v", err))
			}
		case <-db.quit:
			return
		}
	}
}

//expirenodes迭代数据库并删除所有没有
//在一段指定的时间内被看见（即收到乒乓球）。
func (db *nodeDB) expireNodes() error {
	threshold := time.Now().Add(-nodeDBNodeExpiration)

//查找发现的早于允许的节点
	it := db.lvl.NewIterator(nil, nil)
	defer it.Release()

	for it.Next() {
//如果不是发现节点，则跳过该项
		id, field := splitKey(it.Key())
		if field != nodeDBDiscoverRoot {
			continue
		}
//如果尚未过期（而不是自己），则跳过节点
		if !bytes.Equal(id[:], db.self[:]) {
			if seen := db.lastPong(id); seen.After(threshold) {
				continue
			}
		}
//否则删除所有相关信息
		db.deleteNode(id)
	}
	return nil
}

//Lastping检索发送到远程节点的最后一个ping数据包的时间，
//请求绑定。
func (db *nodeDB) lastPing(id NodeID) time.Time {
	return time.Unix(db.fetchInt64(makeKey(id, nodeDBDiscoverPing)), 0)
}

//updateLastping更新上次尝试联系远程节点时的更新。
func (db *nodeDB) updateLastPing(id NodeID, instance time.Time) error {
	return db.storeInt64(makeKey(id, nodeDBDiscoverPing), instance.Unix())
}

//lastpong从远程节点检索上次成功联系的时间。
func (db *nodeDB) lastPong(id NodeID) time.Time {
	return time.Unix(db.fetchInt64(makeKey(id, nodeDBDiscoverPong)), 0)
}

//updateLastpong更新上次成功联系远程节点的时间。
func (db *nodeDB) updateLastPong(id NodeID, instance time.Time) error {
	return db.storeInt64(makeKey(id, nodeDBDiscoverPong), instance.Unix())
}

//findfails检索自绑定以来findnode失败的次数。
func (db *nodeDB) findFails(id NodeID) int {
	return int(db.fetchInt64(makeKey(id, nodeDBDiscoverFindFails)))
}

//updatefindfails更新自绑定以来findnode失败的次数。
func (db *nodeDB) updateFindFails(id NodeID, fails int) error {
	return db.storeInt64(makeKey(id, nodeDBDiscoverFindFails), int64(fails))
}

//local endpoint返回与
//给定的远程节点。
func (db *nodeDB) localEndpoint(id NodeID) *rpcEndpoint {
	var ep rpcEndpoint
	if err := db.fetchRLP(makeKey(id, nodeDBDiscoverLocalEndpoint), &ep); err != nil {
		return nil
	}
	return &ep
}

func (db *nodeDB) updateLocalEndpoint(id NodeID, ep rpcEndpoint) error {
	return db.storeRLP(makeKey(id, nodeDBDiscoverLocalEndpoint), &ep)
}

//queryseeds检索用作潜在种子节点的随机节点
//用于引导。
func (db *nodeDB) querySeeds(n int, maxAge time.Duration) []*Node {
	var (
		now   = time.Now()
		nodes = make([]*Node, 0, n)
		it    = db.lvl.NewIterator(nil, nil)
		id    NodeID
	)
	defer it.Release()

seek:
	for seeks := 0; len(nodes) < n && seeks < n*5; seeks++ {
//寻找一个随机条目。第一个字节的增量为
//每次随机数以增加可能性
//攻击非常小的数据库中的所有现有节点。
		ctr := id[0]
		rand.Read(id[:])
		id[0] = ctr + id[0]%16
		it.Seek(makeKey(id, nodeDBDiscoverRoot))

		n := nextNode(it)
		if n == nil {
			id[0] = 0
continue seek //迭代器已用完
		}
		if n.ID == db.self {
			continue seek
		}
		if now.Sub(db.lastPong(n.ID)) > maxAge {
			continue seek
		}
		for i := range nodes {
			if nodes[i].ID == n.ID {
continue seek //复制品
			}
		}
		nodes = append(nodes, n)
	}
	return nodes
}

func (db *nodeDB) fetchTopicRegTickets(id NodeID) (issued, used uint32) {
	key := makeKey(id, nodeDBTopicRegTickets)
	blob, _ := db.lvl.Get(key, nil)
	if len(blob) != 8 {
		return 0, 0
	}
	issued = binary.BigEndian.Uint32(blob[0:4])
	used = binary.BigEndian.Uint32(blob[4:8])
	return
}

func (db *nodeDB) updateTopicRegTickets(id NodeID, issued, used uint32) error {
	key := makeKey(id, nodeDBTopicRegTickets)
	blob := make([]byte, 8)
	binary.BigEndian.PutUint32(blob[0:4], issued)
	binary.BigEndian.PutUint32(blob[4:8], used)
	return db.lvl.Put(key, blob, nil)
}

//从迭代器读取下一个节点记录，跳过其他节点记录
//数据库条目。
func nextNode(it iterator.Iterator) *Node {
	for end := false; !end; end = !it.Next() {
		id, field := splitKey(it.Key())
		if field != nodeDBDiscoverRoot {
			continue
		}
		var n Node
		if err := rlp.DecodeBytes(it.Value(), &n); err != nil {
			log.Warn(fmt.Sprintf("invalid node %x: %v", id, err))
			continue
		}
		return &n
	}
	return nil
}

//关闭刷新并关闭数据库文件。
func (db *nodeDB) close() {
	close(db.quit)
	db.lvl.Close()
}
