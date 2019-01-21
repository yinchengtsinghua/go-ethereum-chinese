
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

//包BZZ的磁盘存储层
//dbstore实现chunkstore接口，文件存储将其用作
//块的持久存储
//它基于访问计数实现清除，允许外部控制
//最大容量

package storage

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"sync"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage/mock"
	"github.com/syndtr/goleveldb/leveldb"
)

const (
	defaultGCRatio    = 10
	defaultMaxGCRound = 10000
	defaultMaxGCBatch = 5000

	wEntryCnt  = 1 << 0
	wIndexCnt  = 1 << 1
	wAccessCnt = 1 << 2
)

var (
	dbEntryCount = metrics.NewRegisteredCounter("ldbstore.entryCnt", nil)
)

var (
	keyIndex       = byte(0)
	keyAccessCnt   = []byte{2}
	keyEntryCnt    = []byte{3}
	keyDataIdx     = []byte{4}
	keyData        = byte(6)
	keyDistanceCnt = byte(7)
	keySchema      = []byte{8}
keyGCIdx       = byte(9) //对块数据索引的访问，垃圾收集从第一个条目开始按升序使用
)

var (
	ErrDBClosed = errors.New("LDBStore closed")
)

type LDBStoreParams struct {
	*StoreParams
	Path string
	Po   func(Address) uint8
}

//newldbstoreparams用指定的值构造ldbstoreparams。
func NewLDBStoreParams(storeparams *StoreParams, path string) *LDBStoreParams {
	return &LDBStoreParams{
		StoreParams: storeparams,
		Path:        path,
		Po:          func(k Address) (ret uint8) { return uint8(Proximity(storeparams.BaseKey, k[:])) },
	}
}

type garbage struct {
maxRound int           //一轮垃圾收集中要删除的最大块数
maxBatch int           //一个数据库请求批中要删除的最大块数
ratio    int           //1/x比率，用于计算低容量数据库上GC的块数
count    int           //运行回合中删除的块数
target   int           //运行回合中要删除的块数
batch    *dbBatch      //删除批处理
runC     chan struct{} //chan中的结构表示gc未运行
}

type LDBStore struct {
	db *LDBDatabase

//这应该存储在数据库中，以事务方式访问
entryCnt  uint64 //级别数据库中的项目数
accessCnt uint64 //每次我们读取/访问一个条目时，累积的数字都会增加。
dataIdx   uint64 //类似于entrycnt，但我们只增加它
	capacity  uint64
	bucketCnt []uint64

	hashfunc SwarmHasher
	po       func(Address) uint8

	batchesC chan struct{}
	closed   bool
	batch    *dbBatch
	lock     sync.RWMutex
	quit     chan struct{}
	gc       *garbage

//函数encodedatafunc用于绕过
//dbstore的默认功能
//用于测试的mock.nodestore。
	encodeDataFunc func(chunk Chunk) []byte
//如果定义了getdatafunc，它将用于
//而是从本地检索块数据
//级别数据库。
	getDataFunc func(key Address) (data []byte, err error)
}

type dbBatch struct {
	*leveldb.Batch
	err error
	c   chan struct{}
}

func newBatch() *dbBatch {
	return &dbBatch{Batch: new(leveldb.Batch), c: make(chan struct{})}
}

//TODO:不传递距离函数，只传递计算距离的地址
//为了避免可插拔距离度量的出现以及与提供
//一种不同于实际使用的函数。
func NewLDBStore(params *LDBStoreParams) (s *LDBStore, err error) {
	s = new(LDBStore)
	s.hashfunc = params.Hash
	s.quit = make(chan struct{})

	s.batchesC = make(chan struct{}, 1)
	go s.writeBatches()
	s.batch = newBatch()
//将encodedata与默认功能关联
	s.encodeDataFunc = encodeData

	s.db, err = NewLDBDatabase(params.Path)
	if err != nil {
		return nil, err
	}

	s.po = params.Po
	s.setCapacity(params.DbCapacity)

	s.bucketCnt = make([]uint64, 0x100)
	for i := 0; i < 0x100; i++ {
		k := make([]byte, 2)
		k[0] = keyDistanceCnt
		k[1] = uint8(i)
		cnt, _ := s.db.Get(k)
		s.bucketCnt[i] = BytesToU64(cnt)
	}
	data, _ := s.db.Get(keyEntryCnt)
	s.entryCnt = BytesToU64(data)
	data, _ = s.db.Get(keyAccessCnt)
	s.accessCnt = BytesToU64(data)
	data, _ = s.db.Get(keyDataIdx)
	s.dataIdx = BytesToU64(data)

//设置垃圾收集
	s.gc = &garbage{
		maxBatch: defaultMaxGCBatch,
		maxRound: defaultMaxGCRound,
		ratio:    defaultGCRatio,
	}

	s.gc.runC = make(chan struct{}, 1)
	s.gc.runC <- struct{}{}

	return s, nil
}

//markaccessed将访问计数器增加为块的最佳工作，因此
//块不会被垃圾收集。
func (s *LDBStore) MarkAccessed(addr Address) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.closed {
		return
	}

	proximity := s.po(addr)
	s.tryAccessIdx(addr, proximity)
}

//初始化并设置用于处理gc循环的值
func (s *LDBStore) startGC(c int) {

	s.gc.count = 0
//计算删除的目标数量
	if c >= s.gc.maxRound {
		s.gc.target = s.gc.maxRound
	} else {
		s.gc.target = c / s.gc.ratio
	}
	s.gc.batch = newBatch()
	log.Debug("startgc", "requested", c, "target", s.gc.target)
}

//newmockdbstore创建dbstore的新实例
//mockstore设置为提供的值。如果mockstore参数为零，
//此函数的行为与newdbstore完全相同。
func NewMockDbStore(params *LDBStoreParams, mockStore *mock.NodeStore) (s *LDBStore, err error) {
	s, err = NewLDBStore(params)
	if err != nil {
		return nil, err
	}

//用模拟商店功能替换Put和Get
	if mockStore != nil {
		s.encodeDataFunc = newMockEncodeDataFunc(mockStore)
		s.getDataFunc = newMockGetDataFunc(mockStore)
	}
	return
}

type dpaDBIndex struct {
	Idx    uint64
	Access uint64
}

func BytesToU64(data []byte) uint64 {
	if len(data) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(data)
}

func U64ToBytes(val uint64) []byte {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, val)
	return data
}

func getIndexKey(hash Address) []byte {
	hashSize := len(hash)
	key := make([]byte, hashSize+1)
	key[0] = keyIndex
	copy(key[1:], hash[:])
	return key
}

func getDataKey(idx uint64, po uint8) []byte {
	key := make([]byte, 10)
	key[0] = keyData
	key[1] = po
	binary.BigEndian.PutUint64(key[2:], idx)

	return key
}

func getGCIdxKey(index *dpaDBIndex) []byte {
	key := make([]byte, 9)
	key[0] = keyGCIdx
	binary.BigEndian.PutUint64(key[1:], index.Access)
	return key
}

func getGCIdxValue(index *dpaDBIndex, po uint8, addr Address) []byte {
val := make([]byte, 41) //po=1，index.index=8，address=32
	val[0] = po
	binary.BigEndian.PutUint64(val[1:], index.Idx)
	copy(val[9:], addr)
	return val
}

func parseIdxKey(key []byte) (byte, []byte) {
	return key[0], key[1:]
}

func parseGCIdxEntry(accessCnt []byte, val []byte) (index *dpaDBIndex, po uint8, addr Address) {
	index = &dpaDBIndex{
		Idx:    binary.BigEndian.Uint64(val[1:]),
		Access: binary.BigEndian.Uint64(accessCnt),
	}
	po = val[0]
	addr = val[9:]
	return
}

func encodeIndex(index *dpaDBIndex) []byte {
	data, _ := rlp.EncodeToBytes(index)
	return data
}

func encodeData(chunk Chunk) []byte {
//始终为返回的字节片创建新的基础数组。
//chunk.address数组可以在返回的切片中使用，
//可能在代码的后面或级别数据库中更改，从而导致
//地址也改变了。
	return append(append([]byte{}, chunk.Address()[:]...), chunk.Data()...)
}

func decodeIndex(data []byte, index *dpaDBIndex) error {
	dec := rlp.NewStream(bytes.NewReader(data), 0)
	return dec.Decode(index)
}

func decodeData(addr Address, data []byte) (*chunk, error) {
	return NewChunk(addr, data[32:]), nil
}

func (s *LDBStore) collectGarbage() error {

//防止重复GC在已经运行时启动
	select {
	case <-s.gc.runC:
	default:
		return nil
	}

	s.lock.Lock()
	entryCnt := s.entryCnt
	s.lock.Unlock()

	metrics.GetOrRegisterCounter("ldbstore.collectgarbage", nil).Inc(1)

//计算要收集和重置计数器的块的数量
	s.startGC(int(entryCnt))
	log.Debug("collectGarbage", "target", s.gc.target, "entryCnt", entryCnt)

	var totalDeleted int
	for s.gc.count < s.gc.target {
		it := s.db.NewIterator()
		ok := it.Seek([]byte{keyGCIdx})
		var singleIterationCount int

//每个批都需要一个锁，这样我们就可以避免条目同时更改accessIDX。
		s.lock.Lock()
		for ; ok && (singleIterationCount < s.gc.maxBatch); ok = it.Next() {

//如果没有更多的访问索引键，则退出
			itkey := it.Key()
			if (itkey == nil) || (itkey[0] != keyGCIdx) {
				break
			}

//从访问索引获取区块数据项
			val := it.Value()
			index, po, hash := parseGCIdxEntry(itkey[1:], val)
			keyIdx := make([]byte, 33)
			keyIdx[0] = keyIndex
			copy(keyIdx[1:], hash)

//将删除操作添加到批处理
			s.delete(s.gc.batch.Batch, index, keyIdx, po)
			singleIterationCount++
			s.gc.count++
			log.Trace("garbage collect enqueued chunk for deletion", "key", hash)

//如果目标不在最大垃圾批处理边界上，则中断
			if s.gc.count >= s.gc.target {
				break
			}
		}

		s.writeBatch(s.gc.batch, wEntryCnt)
		s.lock.Unlock()
		it.Release()
		log.Trace("garbage collect batch done", "batch", singleIterationCount, "total", s.gc.count)
	}

	s.gc.runC <- struct{}{}
	log.Debug("garbage collect done", "c", s.gc.count)

	metrics.GetOrRegisterCounter("ldbstore.collectgarbage.delete", nil).Inc(int64(totalDeleted))
	return nil
}

//export将存储区中的所有块写入tar存档，并返回
//写入的块数。
func (s *LDBStore) Export(out io.Writer) (int64, error) {
	tw := tar.NewWriter(out)
	defer tw.Close()

	it := s.db.NewIterator()
	defer it.Release()
	var count int64
	for ok := it.Seek([]byte{keyIndex}); ok; ok = it.Next() {
		key := it.Key()
		if (key == nil) || (key[0] != keyIndex) {
			break
		}

		var index dpaDBIndex

		hash := key[1:]
		decodeIndex(it.Value(), &index)
		po := s.po(hash)
		datakey := getDataKey(index.Idx, po)
		log.Trace("store.export", "dkey", fmt.Sprintf("%x", datakey), "dataidx", index.Idx, "po", po)
		data, err := s.db.Get(datakey)
		if err != nil {
			log.Warn(fmt.Sprintf("Chunk %x found but could not be accessed: %v", key, err))
			continue
		}

		hdr := &tar.Header{
			Name: hex.EncodeToString(hash),
			Mode: 0644,
			Size: int64(len(data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return count, err
		}
		if _, err := tw.Write(data); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

//块读取。
func (s *LDBStore) Import(in io.Reader) (int64, error) {
	tr := tar.NewReader(in)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	countC := make(chan int64)
	errC := make(chan error)
	var count int64
	go func() {
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			} else if err != nil {
				select {
				case errC <- err:
				case <-ctx.Done():
				}
			}

			if len(hdr.Name) != 64 {
				log.Warn("ignoring non-chunk file", "name", hdr.Name)
				continue
			}

			keybytes, err := hex.DecodeString(hdr.Name)
			if err != nil {
				log.Warn("ignoring invalid chunk file", "name", hdr.Name, "err", err)
				continue
			}

			data, err := ioutil.ReadAll(tr)
			if err != nil {
				select {
				case errC <- err:
				case <-ctx.Done():
				}
			}
			key := Address(keybytes)
			chunk := NewChunk(key, data[32:])

			go func() {
				select {
				case errC <- s.Put(ctx, chunk):
				case <-ctx.Done():
				}
			}()

			count++
		}
		countC <- count
	}()

//等待存储所有块
	i := int64(0)
	var total int64
	for {
		select {
		case err := <-errC:
			if err != nil {
				return count, err
			}
			i++
		case total = <-countC:
		case <-ctx.Done():
			return i, ctx.Err()
		}
		if total > 0 && i == total {
			return total, nil
		}
	}
}

//cleanup循环访问数据库，并在块通过“f”条件时删除块。
func (s *LDBStore) Cleanup(f func(*chunk) bool) {
	var errorsFound, removed, total int

	it := s.db.NewIterator()
	defer it.Release()
	for ok := it.Seek([]byte{keyIndex}); ok; ok = it.Next() {
		key := it.Key()
		if (key == nil) || (key[0] != keyIndex) {
			break
		}
		total++
		var index dpaDBIndex
		err := decodeIndex(it.Value(), &index)
		if err != nil {
			log.Warn("Cannot decode")
			errorsFound++
			continue
		}
		hash := key[1:]
		po := s.po(hash)
		datakey := getDataKey(index.Idx, po)
		data, err := s.db.Get(datakey)
		if err != nil {
			found := false

//最大可能接近度为255
			for po = 1; po <= 255; po++ {
				datakey = getDataKey(index.Idx, po)
				data, err = s.db.Get(datakey)
				if err == nil {
					found = true
					break
				}
			}

			if !found {
				log.Warn(fmt.Sprintf("Chunk %x found but count not be accessed with any po", key))
				errorsFound++
				continue
			}
		}

		ck := data[:32]
		c, err := decodeData(ck, data)
		if err != nil {
			log.Error("decodeData error", "err", err)
			continue
		}

		cs := int64(binary.LittleEndian.Uint64(c.sdata[:8]))
		log.Trace("chunk", "key", fmt.Sprintf("%x", key), "ck", fmt.Sprintf("%x", ck), "dkey", fmt.Sprintf("%x", datakey), "dataidx", index.Idx, "po", po, "len data", len(data), "len sdata", len(c.sdata), "size", cs)

//如果要删除块
		if f(c) {
			log.Warn("chunk for cleanup", "key", fmt.Sprintf("%x", key), "ck", fmt.Sprintf("%x", ck), "dkey", fmt.Sprintf("%x", datakey), "dataidx", index.Idx, "po", po, "len data", len(data), "len sdata", len(c.sdata), "size", cs)
			s.deleteNow(&index, getIndexKey(key[1:]), po)
			removed++
			errorsFound++
		}
	}

	log.Warn(fmt.Sprintf("Found %v errors out of %v entries. Removed %v chunks.", errorsFound, total, removed))
}

//CleangIndex从头重建垃圾收集器索引，而
//删除不一致的元素，例如缺少数据块的索引。
//警告：这是一个相当重的长期运行功能。
func (s *LDBStore) CleanGCIndex() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	batch := leveldb.Batch{}

	var okEntryCount uint64
	var totalEntryCount uint64

//抛开所有GC索引，我们将从清除的索引重新生成
	it := s.db.NewIterator()
	it.Seek([]byte{keyGCIdx})
	var gcDeletes int
	for it.Valid() {
		rowType, _ := parseIdxKey(it.Key())
		if rowType != keyGCIdx {
			break
		}
		batch.Delete(it.Key())
		gcDeletes++
		it.Next()
	}
	log.Debug("gc", "deletes", gcDeletes)
	if err := s.db.Write(&batch); err != nil {
		return err
	}
	batch.Reset()

	it.Release()

//修正的采购订单索引指针值
	var poPtrs [256]uint64

//如果块计数不在4096迭代边界上，则设置为true
	var doneIterating bool

//上一次迭代中的最后一个键索引
	lastIdxKey := []byte{keyIndex}

//调试输出计数器
	var cleanBatchCount int

//浏览所有键索引项
	for !doneIterating {
		cleanBatchCount++
		var idxs []dpaDBIndex
		var chunkHashes [][]byte
		var pos []uint8
		it := s.db.NewIterator()

		it.Seek(lastIdxKey)

//4096只是一个很好的数字，不要在这里寻找任何隐藏的意义…
		var i int
		for i = 0; i < 4096; i++ {

//除非数据库为空，否则不会发生这种情况。
//但为了安全起见
			if !it.Valid() {
				doneIterating = true
				break
			}

//如果不再是keyindex，我们就完成了迭代
			rowType, chunkHash := parseIdxKey(it.Key())
			if rowType != keyIndex {
				doneIterating = true
				break
			}

//解码检索到的索引
			var idx dpaDBIndex
			err := decodeIndex(it.Value(), &idx)
			if err != nil {
				return fmt.Errorf("corrupt index: %v", err)
			}
			po := s.po(chunkHash)
			lastIdxKey = it.Key()

//如果找不到数据密钥，请删除该条目
//如果找到它，请添加到新的GC索引数组中以创建
			dataKey := getDataKey(idx.Idx, po)
			_, err = s.db.Get(dataKey)
			if err != nil {
				log.Warn("deleting inconsistent index (missing data)", "key", chunkHash)
				batch.Delete(it.Key())
			} else {
				idxs = append(idxs, idx)
				chunkHashes = append(chunkHashes, chunkHash)
				pos = append(pos, po)
				okEntryCount++
				if idx.Idx > poPtrs[po] {
					poPtrs[po] = idx.Idx
				}
			}
			totalEntryCount++
			it.Next()
		}
		it.Release()

//刷新键索引更正
		err := s.db.Write(&batch)
		if err != nil {
			return err
		}
		batch.Reset()

//添加正确的GC索引
		for i, okIdx := range idxs {
			gcIdxKey := getGCIdxKey(&okIdx)
			gcIdxData := getGCIdxValue(&okIdx, pos[i], chunkHashes[i])
			batch.Put(gcIdxKey, gcIdxData)
			log.Trace("clean ok", "key", chunkHashes[i], "gcKey", gcIdxKey, "gcData", gcIdxData)
		}

//冲洗它们
		err = s.db.Write(&batch)
		if err != nil {
			return err
		}
		batch.Reset()

		log.Debug("clean gc index pass", "batch", cleanBatchCount, "checked", i, "kept", len(idxs))
	}

	log.Debug("gc cleanup entries", "ok", okEntryCount, "total", totalEntryCount, "batchlen", batch.Len())

//最后添加更新的条目计数
	var entryCount [8]byte
	binary.BigEndian.PutUint64(entryCount[:], okEntryCount)
	batch.Put(keyEntryCnt, entryCount[:])

//并添加新的采购订单索引指针
	var poKey [2]byte
	poKey[0] = keyDistanceCnt
	for i, poPtr := range poPtrs {
		poKey[1] = uint8(i)
		if poPtr == 0 {
			batch.Delete(poKey[:])
		} else {
			var idxCount [8]byte
			binary.BigEndian.PutUint64(idxCount[:], poPtr)
			batch.Put(poKey[:], idxCount[:])
		}
	}

//如果你做到这一步，你的硬盘就活下来了。祝贺你
	return s.db.Write(&batch)
}

//删除是删除块并更新索引。
//线程安全
func (s *LDBStore) Delete(addr Address) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	ikey := getIndexKey(addr)

	idata, err := s.db.Get(ikey)
	if err != nil {
		return err
	}

	var idx dpaDBIndex
	decodeIndex(idata, &idx)
	proximity := s.po(addr)
	return s.deleteNow(&idx, ikey, proximity)
}

//立即执行一个删除操作
//请参见*ldbstore.delete
func (s *LDBStore) deleteNow(idx *dpaDBIndex, idxKey []byte, po uint8) error {
	batch := new(leveldb.Batch)
	s.delete(batch, idx, idxKey, po)
	return s.db.Write(batch)
}

//向提供的批添加删除块操作
//如果直接调用，则递减entrycount，无论删除时块是否存在。包裹风险最大为UInt64
func (s *LDBStore) delete(batch *leveldb.Batch, idx *dpaDBIndex, idxKey []byte, po uint8) {
	metrics.GetOrRegisterCounter("ldbstore.delete", nil).Inc(1)

	gcIdxKey := getGCIdxKey(idx)
	batch.Delete(gcIdxKey)
	dataKey := getDataKey(idx.Idx, po)
	batch.Delete(dataKey)
	batch.Delete(idxKey)
	s.entryCnt--
	dbEntryCount.Dec(1)
	cntKey := make([]byte, 2)
	cntKey[0] = keyDistanceCnt
	cntKey[1] = po
	batch.Put(keyEntryCnt, U64ToBytes(s.entryCnt))
	batch.Put(cntKey, U64ToBytes(s.bucketCnt[po]))
}

func (s *LDBStore) BinIndex(po uint8) uint64 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.bucketCnt[po]
}

//Put向数据库添加一个块，添加索引并增加全局计数器。
//如果它已经存在，它只增加现有条目的访问计数。
//线程安全
func (s *LDBStore) Put(ctx context.Context, chunk Chunk) error {
	metrics.GetOrRegisterCounter("ldbstore.put", nil).Inc(1)
	log.Trace("ldbstore.put", "key", chunk.Address())

	ikey := getIndexKey(chunk.Address())
	var index dpaDBIndex

	po := s.po(chunk.Address())

	s.lock.Lock()

	if s.closed {
		s.lock.Unlock()
		return ErrDBClosed
	}
	batch := s.batch

	log.Trace("ldbstore.put: s.db.Get", "key", chunk.Address(), "ikey", fmt.Sprintf("%x", ikey))
	_, err := s.db.Get(ikey)
	if err != nil {
		s.doPut(chunk, &index, po)
	}
	idata := encodeIndex(&index)
	s.batch.Put(ikey, idata)

//添加用于垃圾收集的访问chunkindex索引
	gcIdxKey := getGCIdxKey(&index)
	gcIdxData := getGCIdxValue(&index, po, chunk.Address())
	s.batch.Put(gcIdxKey, gcIdxData)
	s.lock.Unlock()

	select {
	case s.batchesC <- struct{}{}:
	default:
	}

	select {
	case <-batch.c:
		return batch.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

//强制放入数据库，不检查或更新必要的索引
func (s *LDBStore) doPut(chunk Chunk, index *dpaDBIndex, po uint8) {
	data := s.encodeDataFunc(chunk)
	dkey := getDataKey(s.dataIdx, po)
	s.batch.Put(dkey, data)
	index.Idx = s.dataIdx
	s.bucketCnt[po] = s.dataIdx
	s.entryCnt++
	dbEntryCount.Inc(1)
	s.dataIdx++
	index.Access = s.accessCnt
	s.accessCnt++
	cntKey := make([]byte, 2)
	cntKey[0] = keyDistanceCnt
	cntKey[1] = po
	s.batch.Put(cntKey, U64ToBytes(s.bucketCnt[po]))
}

func (s *LDBStore) writeBatches() {
	for {
		select {
		case <-s.quit:
			log.Debug("DbStore: quit batch write loop")
			return
		case <-s.batchesC:
			err := s.writeCurrentBatch()
			if err != nil {
				log.Debug("DbStore: quit batch write loop", "err", err.Error())
				return
			}
		}
	}

}

func (s *LDBStore) writeCurrentBatch() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	b := s.batch
	l := b.Len()
	if l == 0 {
		return nil
	}
	s.batch = newBatch()
	b.err = s.writeBatch(b, wEntryCnt|wAccessCnt|wIndexCnt)
	close(b.c)
	if s.entryCnt >= s.capacity {
		go s.collectGarbage()
	}
	return nil
}

//必须非并发调用
func (s *LDBStore) writeBatch(b *dbBatch, wFlag uint8) error {
	if wFlag&wEntryCnt > 0 {
		b.Put(keyEntryCnt, U64ToBytes(s.entryCnt))
	}
	if wFlag&wIndexCnt > 0 {
		b.Put(keyDataIdx, U64ToBytes(s.dataIdx))
	}
	if wFlag&wAccessCnt > 0 {
		b.Put(keyAccessCnt, U64ToBytes(s.accessCnt))
	}
	l := b.Len()
	if err := s.db.Write(b.Batch); err != nil {
		return fmt.Errorf("unable to write batch: %v", err)
	}
	log.Trace(fmt.Sprintf("batch write (%d entries)", l))
	return nil
}

//newmockencodedatafunc返回一个存储块数据的函数
//到模拟存储，以绕过默认功能encodedata。
//构造的函数总是返回nil数据，就像dbstore一样
//不需要存储数据，但仍然需要创建索引。
func newMockEncodeDataFunc(mockStore *mock.NodeStore) func(chunk Chunk) []byte {
	return func(chunk Chunk) []byte {
		if err := mockStore.Put(chunk.Address(), encodeData(chunk)); err != nil {
			log.Error(fmt.Sprintf("%T: Chunk %v put: %v", mockStore, chunk.Address().Log(), err))
		}
		return chunk.Address()[:]
	}
}

//TryAccessIDX尝试查找索引项。如果找到，则增加访问
//对垃圾收集计数，并返回索引项，对found返回true，
//否则返回nil和false。
func (s *LDBStore) tryAccessIdx(addr Address, po uint8) (*dpaDBIndex, bool) {
	ikey := getIndexKey(addr)
	idata, err := s.db.Get(ikey)
	if err != nil {
		return nil, false
	}

	index := new(dpaDBIndex)
	decodeIndex(idata, index)
	oldGCIdxKey := getGCIdxKey(index)
	s.batch.Put(keyAccessCnt, U64ToBytes(s.accessCnt))
	index.Access = s.accessCnt
	idata = encodeIndex(index)
	s.accessCnt++
	s.batch.Put(ikey, idata)
	newGCIdxKey := getGCIdxKey(index)
	newGCIdxData := getGCIdxValue(index, po, ikey[1:])
	s.batch.Delete(oldGCIdxKey)
	s.batch.Put(newGCIdxKey, newGCIdxData)
	select {
	case s.batchesC <- struct{}{}:
	default:
	}
	return index, true
}

//GetSchema正在返回从LevelDB读取的数据存储的当前命名架构。
func (s *LDBStore) GetSchema() (string, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	data, err := s.db.Get(keySchema)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return DbSchemaNone, nil
		}
		return "", err
	}

	return string(data), nil
}

//PutSchema正在将命名架构保存到LevelDB数据存储
func (s *LDBStore) PutSchema(schema string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.db.Put(keySchema, []byte(schema))
}

//get从数据库中检索与提供的键匹配的块。
//如果块条目不存在，则返回错误
//更新访问计数并且是线程安全的
func (s *LDBStore) Get(_ context.Context, addr Address) (chunk Chunk, err error) {
	metrics.GetOrRegisterCounter("ldbstore.get", nil).Inc(1)
	log.Trace("ldbstore.get", "key", addr)

	s.lock.Lock()
	defer s.lock.Unlock()
	return s.get(addr)
}

//TODO:为了符合此对象的其他私有方法，不应更新索引
func (s *LDBStore) get(addr Address) (chunk *chunk, err error) {
	if s.closed {
		return nil, ErrDBClosed
	}
	proximity := s.po(addr)
	index, found := s.tryAccessIdx(addr, proximity)
	if found {
		var data []byte
		if s.getDataFunc != nil {
//如果定义了getdatafunc，则使用它来检索块数据
			log.Trace("ldbstore.get retrieve with getDataFunc", "key", addr)
			data, err = s.getDataFunc(addr)
			if err != nil {
				return
			}
		} else {
//用于检索块数据的默认dbstore功能
			datakey := getDataKey(index.Idx, proximity)
			data, err = s.db.Get(datakey)
			log.Trace("ldbstore.get retrieve", "key", addr, "indexkey", index.Idx, "datakey", fmt.Sprintf("%x", datakey), "proximity", proximity)
			if err != nil {
				log.Trace("ldbstore.get chunk found but could not be accessed", "key", addr, "err", err)
				s.deleteNow(index, getIndexKey(addr), s.po(addr))
				return
			}
		}

		return decodeData(addr, data)
	} else {
		err = ErrChunkNotFound
	}

	return
}

//newmockgetfunc返回一个函数，该函数从
//模拟数据库，用作dbstore.getfunc的值
//绕过具有模拟存储的dbstore的默认功能。
func newMockGetDataFunc(mockStore *mock.NodeStore) func(addr Address) (data []byte, err error) {
	return func(addr Address) (data []byte, err error) {
		data, err = mockStore.Get(addr)
		if err == mock.ErrNotFound {
//保留errChunkOnFound错误
			err = ErrChunkNotFound
		}
		return data, err
	}
}

func (s *LDBStore) setCapacity(c uint64) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.capacity = c

	for s.entryCnt > c {
		s.collectGarbage()
	}
}

func (s *LDBStore) Close() {
	close(s.quit)
	s.lock.Lock()
	s.closed = true
	s.lock.Unlock()
//强制写出当前批
	s.writeCurrentBatch()
	close(s.batchesC)
	s.db.Close()
}

//synciterator（start、stop、po、f）从开始到停止对bin po的每个哈希调用f
func (s *LDBStore) SyncIterator(since uint64, until uint64, po uint8, f func(Address, uint64) bool) error {
	metrics.GetOrRegisterCounter("ldbstore.synciterator", nil).Inc(1)

	sincekey := getDataKey(since, po)
	untilkey := getDataKey(until, po)
	it := s.db.NewIterator()
	defer it.Release()

	for ok := it.Seek(sincekey); ok; ok = it.Next() {
		metrics.GetOrRegisterCounter("ldbstore.synciterator.seek", nil).Inc(1)

		dbkey := it.Key()
		if dbkey[0] != keyData || dbkey[1] != po || bytes.Compare(untilkey, dbkey) < 0 {
			break
		}
		key := make([]byte, 32)
		val := it.Value()
		copy(key, val[:32])
		if !f(Address(key), binary.BigEndian.Uint64(dbkey[2:])) {
			break
		}
	}
	return it.Error()
}
