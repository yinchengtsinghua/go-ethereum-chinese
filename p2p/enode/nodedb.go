
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

package enode

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/util"
)

//节点数据库中的键。
const (
dbVersionKey = "version" //更改时要刷新的数据库版本
dbItemPrefix = "n:"      //为节点条目加前缀的标识符

	dbDiscoverRoot      = ":discover"
	dbDiscoverSeq       = dbDiscoverRoot + ":seq"
	dbDiscoverPing      = dbDiscoverRoot + ":lastping"
	dbDiscoverPong      = dbDiscoverRoot + ":lastpong"
	dbDiscoverFindFails = dbDiscoverRoot + ":findfail"
	dbLocalRoot         = ":local"
	dbLocalSeq          = dbLocalRoot + ":seq"
)

var (
dbNodeExpiration = 24 * time.Hour //删除未查看节点的时间。
dbCleanupCycle   = time.Hour      //运行过期任务的时间段。
	dbVersion        = 7
)

//db是节点数据库，存储以前看到的节点和有关
//它们用于QoS目的。
type DB struct {
lvl    *leveldb.DB   //与数据库本身的接口
runner sync.Once     //确保我们最多可以启动一个到期日
quit   chan struct{} //向过期线程发出停止信号的通道
}

//opendb打开一个节点数据库，用于存储和检索
//网络。如果没有给内存中的路径，则构建临时数据库。
func OpenDB(path string) (*DB, error) {
	if path == "" {
		return newMemoryDB()
	}
	return newPersistentDB(path)
}

//newmemorynodedb创建一个没有持久后端的新内存节点数据库。
func newMemoryDB() (*DB, error) {
	db, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		return nil, err
	}
	return &DB{lvl: db, quit: make(chan struct{})}, nil
}

//newpersistentnodedb创建/打开一个支持leveldb的持久节点数据库，
//同时在版本不匹配的情况下刷新其内容。
func newPersistentDB(path string) (*DB, error) {
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
	currentVer = currentVer[:binary.PutVarint(currentVer, int64(dbVersion))]

	blob, err := db.Get([]byte(dbVersionKey), nil)
	switch err {
	case leveldb.ErrNotFound:
//找不到版本（即空缓存），请将其插入
		if err := db.Put([]byte(dbVersionKey), currentVer, nil); err != nil {
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
			return newPersistentDB(path)
		}
	}
	return &DB{lvl: db, quit: make(chan struct{})}, nil
}

//makekey从节点ID及其特定的
//感兴趣的领域。
func makeKey(id ID, field string) []byte {
	if (id == ID{}) {
		return []byte(field)
	}
	return append([]byte(dbItemPrefix), append(id[:], field...)...)
}

//SplitKey尝试将数据库键拆分为节点ID和字段部分。
func splitKey(key []byte) (id ID, field string) {
//如果键不是节点的，则直接返回
	if !bytes.HasPrefix(key, []byte(dbItemPrefix)) {
		return ID{}, string(key)
	}
//否则拆分ID和字段
	item := key[len(dbItemPrefix):]
	copy(id[:], item[:len(id)])
	field = string(item[len(id):])

	return id, field
}

//fetchint64检索与特定键关联的整数。
func (db *DB) fetchInt64(key []byte) int64 {
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

//storeInt64在给定的键中存储一个整数。
func (db *DB) storeInt64(key []byte, n int64) error {
	blob := make([]byte, binary.MaxVarintLen64)
	blob = blob[:binary.PutVarint(blob, n)]
	return db.lvl.Put(key, blob, nil)
}

//fetchuint64检索与特定键关联的整数。
func (db *DB) fetchUint64(key []byte) uint64 {
	blob, err := db.lvl.Get(key, nil)
	if err != nil {
		return 0
	}
	val, _ := binary.Uvarint(blob)
	return val
}

//storeUInt64在给定的键中存储一个整数。
func (db *DB) storeUint64(key []byte, n uint64) error {
	blob := make([]byte, binary.MaxVarintLen64)
	blob = blob[:binary.PutUvarint(blob, n)]
	return db.lvl.Put(key, blob, nil)
}

//节点从数据库中检索具有给定ID的节点。
func (db *DB) Node(id ID) *Node {
	blob, err := db.lvl.Get(makeKey(id, dbDiscoverRoot), nil)
	if err != nil {
		return nil
	}
	return mustDecodeNode(id[:], blob)
}

func mustDecodeNode(id, data []byte) *Node {
	node := new(Node)
	if err := rlp.DecodeBytes(data, &node.r); err != nil {
		panic(fmt.Errorf("p2p/enode: can't decode node %x in DB: %v", id, err))
	}
//还原节点ID缓存。
	copy(node.id[:], id)
	return node
}

//updateNode将节点插入到对等数据库中（可能会覆盖）。
func (db *DB) UpdateNode(node *Node) error {
	if node.Seq() < db.NodeSeq(node.ID()) {
		return nil
	}
	blob, err := rlp.EncodeToBytes(&node.r)
	if err != nil {
		return err
	}
	if err := db.lvl.Put(makeKey(node.ID(), dbDiscoverRoot), blob, nil); err != nil {
		return err
	}
	return db.storeUint64(makeKey(node.ID(), dbDiscoverSeq), node.Seq())
}

//nodeseq返回给定节点的存储记录序列号。
func (db *DB) NodeSeq(id ID) uint64 {
	return db.fetchUint64(makeKey(id, dbDiscoverSeq))
}

//如果节点具有较大的序列，resolve将返回该节点的存储记录
//比N多
func (db *DB) Resolve(n *Node) *Node {
	if n.Seq() > db.NodeSeq(n.ID()) {
		return n
	}
	return db.Node(n.ID())
}

//删除节点删除与节点关联的所有信息/键。
func (db *DB) DeleteNode(id ID) error {
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
func (db *DB) ensureExpirer() {
	db.runner.Do(func() { go db.expirer() })
}

//Expirer应该在Go例程中启动，并负责循环广告
//无穷大并从数据库中删除过时数据。
func (db *DB) expirer() {
	tick := time.NewTicker(dbCleanupCycle)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if err := db.expireNodes(); err != nil {
				log.Error("Failed to expire nodedb items", "err", err)
			}
		case <-db.quit:
			return
		}
	}
}

//expirenodes迭代数据库并删除所有没有
//在一段指定的时间内被看见（即收到乒乓球）。
func (db *DB) expireNodes() error {
	threshold := time.Now().Add(-dbNodeExpiration)

//查找发现的早于允许的节点
	it := db.lvl.NewIterator(nil, nil)
	defer it.Release()

	for it.Next() {
//如果不是发现节点，则跳过该项
		id, field := splitKey(it.Key())
		if field != dbDiscoverRoot {
			continue
		}
//如果尚未过期（而不是自己），则跳过节点
		if seen := db.LastPongReceived(id); seen.After(threshold) {
			continue
		}
//否则删除所有相关信息
		db.DeleteNode(id)
	}
	return nil
}

//LastpingReceived检索从中接收的最后一个ping数据包的时间
//远程节点。
func (db *DB) LastPingReceived(id ID) time.Time {
	return time.Unix(db.fetchInt64(makeKey(id, dbDiscoverPing)), 0)
}

//上一次尝试联系远程节点时，UpdateLastpingReceived更新。
func (db *DB) UpdateLastPingReceived(id ID, instance time.Time) error {
	return db.storeInt64(makeKey(id, dbDiscoverPing), instance.Unix())
}

//lastpongreceived从远程节点检索上次成功的pong的时间。
func (db *DB) LastPongReceived(id ID) time.Time {
//发射呼气
	db.ensureExpirer()
	return time.Unix(db.fetchInt64(makeKey(id, dbDiscoverPong)), 0)
}

//updateLastPongreeived更新节点的最后一次挂起时间。
func (db *DB) UpdateLastPongReceived(id ID, instance time.Time) error {
	return db.storeInt64(makeKey(id, dbDiscoverPong), instance.Unix())
}

//findfails检索自绑定以来findnode失败的次数。
func (db *DB) FindFails(id ID) int {
	return int(db.fetchInt64(makeKey(id, dbDiscoverFindFails)))
}

//updatefindfails更新自绑定以来findnode失败的次数。
func (db *DB) UpdateFindFails(id ID, fails int) error {
	return db.storeInt64(makeKey(id, dbDiscoverFindFails), int64(fails))
}

//localseq检索本地记录序列计数器。
func (db *DB) localSeq(id ID) uint64 {
	return db.fetchUint64(makeKey(id, dbLocalSeq))
}

//storelocalseq存储本地记录序列计数器。
func (db *DB) storeLocalSeq(id ID, n uint64) {
	db.storeUint64(makeKey(id, dbLocalSeq), n)
}

//queryseeds检索用作潜在种子节点的随机节点
//用于引导。
func (db *DB) QuerySeeds(n int, maxAge time.Duration) []*Node {
	var (
		now   = time.Now()
		nodes = make([]*Node, 0, n)
		it    = db.lvl.NewIterator(nil, nil)
		id    ID
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
		it.Seek(makeKey(id, dbDiscoverRoot))

		n := nextNode(it)
		if n == nil {
			id[0] = 0
continue seek //迭代器已用完
		}
		if now.Sub(db.LastPongReceived(n.ID())) > maxAge {
			continue seek
		}
		for i := range nodes {
			if nodes[i].ID() == n.ID() {
continue seek //复制品
			}
		}
		nodes = append(nodes, n)
	}
	return nodes
}

//从迭代器读取下一个节点记录，跳过其他节点记录
//数据库条目。
func nextNode(it iterator.Iterator) *Node {
	for end := false; !end; end = !it.Next() {
		id, field := splitKey(it.Key())
		if field != dbDiscoverRoot {
			continue
		}
		return mustDecodeNode(id[:], it.Value())
	}
	return nil
}

//关闭刷新并关闭数据库文件。
func (db *DB) Close() {
	close(db.quit)
	db.lvl.Close()
}
