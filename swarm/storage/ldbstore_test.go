
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

package storage

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ch "github.com/ethereum/go-ethereum/swarm/chunk"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage/mock/mem"
	ldberrors "github.com/syndtr/goleveldb/leveldb/errors"
)

type testDbStore struct {
	*LDBStore
	dir string
}

func newTestDbStore(mock bool, trusted bool) (*testDbStore, func(), error) {
	dir, err := ioutil.TempDir("", "bzz-storage-test")
	if err != nil {
		return nil, func() {}, err
	}

	var db *LDBStore
	storeparams := NewDefaultStoreParams()
	params := NewLDBStoreParams(storeparams, dir)
	params.Po = testPoFunc

	if mock {
		globalStore := mem.NewGlobalStore()
		addr := common.HexToAddress("0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed")
		mockStore := globalStore.NewNodeStore(addr)

		db, err = NewMockDbStore(params, mockStore)
	} else {
		db, err = NewLDBStore(params)
	}

	cleanup := func() {
		if db != nil {
			db.Close()
		}
		err = os.RemoveAll(dir)
		if err != nil {
			panic(fmt.Sprintf("db cleanup failed: %v", err))
		}
	}

	return &testDbStore{db, dir}, cleanup, err
}

func testPoFunc(k Address) (ret uint8) {
	basekey := make([]byte, 32)
	return uint8(Proximity(basekey, k[:]))
}

func testDbStoreRandom(n int, chunksize int64, mock bool, t *testing.T) {
	db, cleanup, err := newTestDbStore(mock, true)
	defer cleanup()
	if err != nil {
		t.Fatalf("init dbStore failed: %v", err)
	}
	testStoreRandom(db, n, chunksize, t)
}

func testDbStoreCorrect(n int, chunksize int64, mock bool, t *testing.T) {
	db, cleanup, err := newTestDbStore(mock, false)
	defer cleanup()
	if err != nil {
		t.Fatalf("init dbStore failed: %v", err)
	}
	testStoreCorrect(db, n, chunksize, t)
}

func TestMarkAccessed(t *testing.T) {
	db, cleanup, err := newTestDbStore(false, true)
	defer cleanup()
	if err != nil {
		t.Fatalf("init dbStore failed: %v", err)
	}

	h := GenerateRandomChunk(ch.DefaultSize)

	db.Put(context.Background(), h)

	var index dpaDBIndex
	addr := h.Address()
	idxk := getIndexKey(addr)

	idata, err := db.db.Get(idxk)
	if err != nil {
		t.Fatal(err)
	}
	decodeIndex(idata, &index)

	if index.Access != 0 {
		t.Fatalf("Expected the access index to be %d, but it is %d", 0, index.Access)
	}

	db.MarkAccessed(addr)
	db.writeCurrentBatch()

	idata, err = db.db.Get(idxk)
	if err != nil {
		t.Fatal(err)
	}
	decodeIndex(idata, &index)

	if index.Access != 1 {
		t.Fatalf("Expected the access index to be %d, but it is %d", 1, index.Access)
	}

}

func TestDbStoreRandom_1(t *testing.T) {
	testDbStoreRandom(1, 0, false, t)
}

func TestDbStoreCorrect_1(t *testing.T) {
	testDbStoreCorrect(1, 4096, false, t)
}

func TestDbStoreRandom_1k(t *testing.T) {
	testDbStoreRandom(1000, 0, false, t)
}

func TestDbStoreCorrect_1k(t *testing.T) {
	testDbStoreCorrect(1000, 4096, false, t)
}

func TestMockDbStoreRandom_1(t *testing.T) {
	testDbStoreRandom(1, 0, true, t)
}

func TestMockDbStoreCorrect_1(t *testing.T) {
	testDbStoreCorrect(1, 4096, true, t)
}

func TestMockDbStoreRandom_1k(t *testing.T) {
	testDbStoreRandom(1000, 0, true, t)
}

func TestMockDbStoreCorrect_1k(t *testing.T) {
	testDbStoreCorrect(1000, 4096, true, t)
}

func testDbStoreNotFound(t *testing.T, mock bool) {
	db, cleanup, err := newTestDbStore(mock, false)
	defer cleanup()
	if err != nil {
		t.Fatalf("init dbStore failed: %v", err)
	}

	_, err = db.Get(context.TODO(), ZeroAddr)
	if err != ErrChunkNotFound {
		t.Errorf("Expected ErrChunkNotFound, got %v", err)
	}
}

func TestDbStoreNotFound(t *testing.T) {
	testDbStoreNotFound(t, false)
}
func TestMockDbStoreNotFound(t *testing.T) {
	testDbStoreNotFound(t, true)
}

func testIterator(t *testing.T, mock bool) {
	var chunkcount int = 32
	var i int
	var poc uint
	chunkkeys := NewAddressCollection(chunkcount)
	chunkkeys_results := NewAddressCollection(chunkcount)

	db, cleanup, err := newTestDbStore(mock, false)
	defer cleanup()
	if err != nil {
		t.Fatalf("init dbStore failed: %v", err)
	}

	chunks := GenerateRandomChunks(ch.DefaultSize, chunkcount)

	for i = 0; i < len(chunks); i++ {
		chunkkeys[i] = chunks[i].Address()
		err := db.Put(context.TODO(), chunks[i])
		if err != nil {
			t.Fatalf("dbStore.Put failed: %v", err)
		}
	}

	for i = 0; i < len(chunkkeys); i++ {
		log.Trace(fmt.Sprintf("Chunk array pos %d/%d: '%v'", i, chunkcount, chunkkeys[i]))
	}
	i = 0
	for poc = 0; poc <= 255; poc++ {
		err := db.SyncIterator(0, uint64(chunkkeys.Len()), uint8(poc), func(k Address, n uint64) bool {
			log.Trace(fmt.Sprintf("Got key %v number %d poc %d", k, n, uint8(poc)))
			chunkkeys_results[n] = k
			i++
			return true
		})
		if err != nil {
			t.Fatalf("Iterator call failed: %v", err)
		}
	}

	for i = 0; i < chunkcount; i++ {
		if !bytes.Equal(chunkkeys[i], chunkkeys_results[i]) {
			t.Fatalf("Chunk put #%d key '%v' does not match iterator's key '%v'", i, chunkkeys[i], chunkkeys_results[i])
		}
	}

}

func TestIterator(t *testing.T) {
	testIterator(t, false)
}
func TestMockIterator(t *testing.T) {
	testIterator(t, true)
}

func benchmarkDbStorePut(n int, processors int, chunksize int64, mock bool, b *testing.B) {
	db, cleanup, err := newTestDbStore(mock, true)
	defer cleanup()
	if err != nil {
		b.Fatalf("init dbStore failed: %v", err)
	}
	benchmarkStorePut(db, n, chunksize, b)
}

func benchmarkDbStoreGet(n int, processors int, chunksize int64, mock bool, b *testing.B) {
	db, cleanup, err := newTestDbStore(mock, true)
	defer cleanup()
	if err != nil {
		b.Fatalf("init dbStore failed: %v", err)
	}
	benchmarkStoreGet(db, n, chunksize, b)
}

func BenchmarkDbStorePut_1_500(b *testing.B) {
	benchmarkDbStorePut(500, 1, 4096, false, b)
}

func BenchmarkDbStorePut_8_500(b *testing.B) {
	benchmarkDbStorePut(500, 8, 4096, false, b)
}

func BenchmarkDbStoreGet_1_500(b *testing.B) {
	benchmarkDbStoreGet(500, 1, 4096, false, b)
}

func BenchmarkDbStoreGet_8_500(b *testing.B) {
	benchmarkDbStoreGet(500, 8, 4096, false, b)
}

func BenchmarkMockDbStorePut_1_500(b *testing.B) {
	benchmarkDbStorePut(500, 1, 4096, true, b)
}

func BenchmarkMockDbStorePut_8_500(b *testing.B) {
	benchmarkDbStorePut(500, 8, 4096, true, b)
}

func BenchmarkMockDbStoreGet_1_500(b *testing.B) {
	benchmarkDbStoreGet(500, 1, 4096, true, b)
}

func BenchmarkMockDbStoreGet_8_500(b *testing.B) {
	benchmarkDbStoreGet(500, 8, 4096, true, b)
}

//testldbstore没有收集垃圾测试，我们可以在leveldb存储中放置许多随机块，以及
//如果我们不撞到垃圾收集，就把它们取回
func TestLDBStoreWithoutCollectGarbage(t *testing.T) {
	capacity := 50
	n := 10

	ldb, cleanup := newLDBStore(t)
	ldb.setCapacity(uint64(capacity))
	defer cleanup()

	chunks, err := mputRandomChunks(ldb, n, int64(ch.DefaultSize))
	if err != nil {
		t.Fatal(err.Error())
	}

	log.Info("ldbstore", "entrycnt", ldb.entryCnt, "accesscnt", ldb.accessCnt)

	for _, ch := range chunks {
		ret, err := ldb.Get(context.TODO(), ch.Address())
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(ret.Data(), ch.Data()) {
			t.Fatal("expected to get the same data back, but got smth else")
		}
	}

	if ldb.entryCnt != uint64(n) {
		t.Fatalf("expected entryCnt to be equal to %v, but got %v", n, ldb.entryCnt)
	}

	if ldb.accessCnt != uint64(2*n) {
		t.Fatalf("expected accessCnt to be equal to %v, but got %v", 2*n, ldb.accessCnt)
	}
}

//testldbstorecollectgarbage测试，我们可以放入比leveldb容量更多的块，以及
//只检索其中的一部分，因为垃圾收集必须已部分清除存储区
//还测试我们是否可以删除块以及是否可以触发垃圾收集
func TestLDBStoreCollectGarbage(t *testing.T) {

//低于马克洛德
	initialCap := defaultMaxGCRound / 100
	cap := initialCap / 2
	t.Run(fmt.Sprintf("A/%d/%d", cap, cap*4), testLDBStoreCollectGarbage)
	t.Run(fmt.Sprintf("B/%d/%d", cap, cap*4), testLDBStoreRemoveThenCollectGarbage)

//在最大回合
	cap = initialCap
	t.Run(fmt.Sprintf("A/%d/%d", cap, cap*4), testLDBStoreCollectGarbage)
	t.Run(fmt.Sprintf("B/%d/%d", cap, cap*4), testLDBStoreRemoveThenCollectGarbage)

//大于最大值，不在阈值上
	cap = initialCap + 500
	t.Run(fmt.Sprintf("A/%d/%d", cap, cap*4), testLDBStoreCollectGarbage)
	t.Run(fmt.Sprintf("B/%d/%d", cap, cap*4), testLDBStoreRemoveThenCollectGarbage)

}

func testLDBStoreCollectGarbage(t *testing.T) {
	params := strings.Split(t.Name(), "/")
	capacity, err := strconv.Atoi(params[2])
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(params[3])
	if err != nil {
		t.Fatal(err)
	}

	ldb, cleanup := newLDBStore(t)
	ldb.setCapacity(uint64(capacity))
	defer cleanup()

//检索数据库容量的gc舍入目标计数
	ldb.startGC(capacity)
	roundTarget := ldb.gc.target

//将放置计数拆分为gc目标计数阈值，并等待gc在这两个阈值之间完成
	var allChunks []Chunk
	remaining := n
	for remaining > 0 {
		var putCount int
		if remaining < roundTarget {
			putCount = remaining
		} else {
			putCount = roundTarget
		}
		remaining -= putCount
		chunks, err := mputRandomChunks(ldb, putCount, int64(ch.DefaultSize))
		if err != nil {
			t.Fatal(err.Error())
		}
		allChunks = append(allChunks, chunks...)
		log.Debug("ldbstore", "entrycnt", ldb.entryCnt, "accesscnt", ldb.accessCnt, "cap", capacity, "n", n)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		waitGc(ctx, ldb)
	}

//尝试获取所有放置块
	var missing int
	for _, ch := range allChunks {
		ret, err := ldb.Get(context.TODO(), ch.Address())
		if err == ErrChunkNotFound || err == ldberrors.ErrNotFound {
			missing++
			continue
		}
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(ret.Data(), ch.Data()) {
			t.Fatal("expected to get the same data back, but got smth else")
		}

		log.Trace("got back chunk", "chunk", ret)
	}

//所有剩余的块都应该丢失
	expectMissing := roundTarget + (((n - capacity) / roundTarget) * roundTarget)
	if missing != expectMissing {
		t.Fatalf("gc failure: expected to miss %v chunks, but only %v are actually missing", expectMissing, missing)
	}

	log.Info("ldbstore", "total", n, "missing", missing, "entrycnt", ldb.entryCnt, "accesscnt", ldb.accessCnt)
}

//testldbstoreaddremove我们可以放置的测试，然后删除给定的块。
func TestLDBStoreAddRemove(t *testing.T) {
	ldb, cleanup := newLDBStore(t)
	ldb.setCapacity(200)
	defer cleanup()

	n := 100
	chunks, err := mputRandomChunks(ldb, n, int64(ch.DefaultSize))
	if err != nil {
		t.Fatalf(err.Error())
	}

	for i := 0; i < n; i++ {
//删除所有偶数索引块
		if i%2 == 0 {
			ldb.Delete(chunks[i].Address())
		}
	}

	log.Info("ldbstore", "entrycnt", ldb.entryCnt, "accesscnt", ldb.accessCnt)

	for i := 0; i < n; i++ {
		ret, err := ldb.Get(context.TODO(), chunks[i].Address())

		if i%2 == 0 {
//预期甚至会丢失块
			if err == nil {
				t.Fatal("expected chunk to be missing, but got no error")
			}
		} else {
//希望成功检索奇数块
			if err != nil {
				t.Fatalf("expected no error, but got %s", err)
			}

			if !bytes.Equal(ret.Data(), chunks[i].Data()) {
				t.Fatal("expected to get the same data back, but got smth else")
			}
		}
	}
}

func testLDBStoreRemoveThenCollectGarbage(t *testing.T) {

	params := strings.Split(t.Name(), "/")
	capacity, err := strconv.Atoi(params[2])
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(params[3])
	if err != nil {
		t.Fatal(err)
	}

	ldb, cleanup := newLDBStore(t)
	defer cleanup()
	ldb.setCapacity(uint64(capacity))

//放置容量计数块数
	chunks := make([]Chunk, n)
	for i := 0; i < n; i++ {
		c := GenerateRandomChunk(ch.DefaultSize)
		chunks[i] = c
		log.Trace("generate random chunk", "idx", i, "chunk", c)
	}

	for i := 0; i < n; i++ {
		err := ldb.Put(context.TODO(), chunks[i])
		if err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	waitGc(ctx, ldb)

//删除所有块
//（只统计实际删除的部分，其余部分将被gc'd删除）
	deletes := 0
	for i := 0; i < n; i++ {
		if ldb.Delete(chunks[i].Address()) == nil {
			deletes++
		}
	}

	log.Info("ldbstore", "entrycnt", ldb.entryCnt, "accesscnt", ldb.accessCnt)

	if ldb.entryCnt != 0 {
		t.Fatalf("ldb.entrCnt expected 0 got %v", ldb.entryCnt)
	}

//手动删除会增加accesscnt，所以我们需要在验证当前计数时添加它。
	expAccessCnt := uint64(n)
	if ldb.accessCnt != expAccessCnt {
		t.Fatalf("ldb.accessCnt expected %v got %v", expAccessCnt, ldb.accessCnt)
	}

//检索数据库容量的gc舍入目标计数
	ldb.startGC(capacity)
	roundTarget := ldb.gc.target

	remaining := n
	var puts int
	for remaining > 0 {
		var putCount int
		if remaining < roundTarget {
			putCount = remaining
		} else {
			putCount = roundTarget
		}
		remaining -= putCount
		for putCount > 0 {
			ldb.Put(context.TODO(), chunks[puts])
			log.Debug("ldbstore", "entrycnt", ldb.entryCnt, "accesscnt", ldb.accessCnt, "cap", capacity, "n", n, "puts", puts, "remaining", remaining, "roundtarget", roundTarget)
			puts++
			putCount--
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		waitGc(ctx, ldb)
	}

//由于第一个剩余块具有最小的访问值，因此它们将丢失。
	expectMissing := roundTarget + (((n - capacity) / roundTarget) * roundTarget)
	for i := 0; i < expectMissing; i++ {
		_, err := ldb.Get(context.TODO(), chunks[i].Address())
		if err == nil {
			t.Fatalf("expected surplus chunk %d to be missing, but got no error", i)
		}
	}

//希望最后一个块出现，因为它们具有最大的访问值
	for i := expectMissing; i < n; i++ {
		ret, err := ldb.Get(context.TODO(), chunks[i].Address())
		if err != nil {
			t.Fatalf("chunk %v: expected no error, but got %s", i, err)
		}
		if !bytes.Equal(ret.Data(), chunks[i].Data()) {
			t.Fatal("expected to get the same data back, but got smth else")
		}
	}
}

//testldbstorecollectgarbageaccessunlkiendex测试垃圾收集，其中accessCount与indexCount不同
func TestLDBStoreCollectGarbageAccessUnlikeIndex(t *testing.T) {

	capacity := defaultMaxGCRound / 100 * 2
	n := capacity - 1

	ldb, cleanup := newLDBStore(t)
	ldb.setCapacity(uint64(capacity))
	defer cleanup()

	chunks, err := mputRandomChunks(ldb, n, int64(ch.DefaultSize))
	if err != nil {
		t.Fatal(err.Error())
	}
	log.Info("ldbstore", "entrycnt", ldb.entryCnt, "accesscnt", ldb.accessCnt)

//将第一个添加的容量/2块设置为最高访问计数
	for i := 0; i < capacity/2; i++ {
		_, err := ldb.Get(context.TODO(), chunks[i].Address())
		if err != nil {
			t.Fatalf("fail add chunk #%d - %s: %v", i, chunks[i].Address(), err)
		}
	}
	_, err = mputRandomChunks(ldb, 2, int64(ch.DefaultSize))
	if err != nil {
		t.Fatal(err.Error())
	}

//等待垃圾收集启动负责的参与者
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	waitGc(ctx, ldb)

	var missing int
	for i, ch := range chunks[2 : capacity/2] {
		ret, err := ldb.Get(context.TODO(), ch.Address())
		if err == ErrChunkNotFound || err == ldberrors.ErrNotFound {
			t.Fatalf("fail find chunk #%d - %s: %v", i, ch.Address(), err)
		}

		if !bytes.Equal(ret.Data(), ch.Data()) {
			t.Fatal("expected to get the same data back, but got smth else")
		}
		log.Trace("got back chunk", "chunk", ret)
	}

	log.Info("ldbstore", "total", n, "missing", missing, "entrycnt", ldb.entryCnt, "accesscnt", ldb.accessCnt)
}

func TestCleanIndex(t *testing.T) {
	capacity := 5000
	n := 3

	ldb, cleanup := newLDBStore(t)
	ldb.setCapacity(uint64(capacity))
	defer cleanup()

	chunks, err := mputRandomChunks(ldb, n, 4096)
	if err != nil {
		t.Fatal(err)
	}

//删除第一个块的数据
	po := ldb.po(chunks[0].Address()[:])
	dataKey := make([]byte, 10)
	dataKey[0] = keyData
	dataKey[1] = byte(po)
//datakey[2:10]=第一个区块在[2:10]上具有storageIDX 0
	if _, err := ldb.db.Get(dataKey); err != nil {
		t.Fatal(err)
	}
	if err := ldb.db.Delete(dataKey); err != nil {
		t.Fatal(err)
	}

//删除第一个块的GC索引行
	gcFirstCorrectKey := make([]byte, 9)
	gcFirstCorrectKey[0] = keyGCIdx
	if err := ldb.db.Delete(gcFirstCorrectKey); err != nil {
		t.Fatal(err)
	}

//扭曲第二个块的GC数据
//清洁后，此数据应再次正确。
	gcSecondCorrectKey := make([]byte, 9)
	gcSecondCorrectKey[0] = keyGCIdx
	binary.BigEndian.PutUint64(gcSecondCorrectKey[1:], uint64(1))
	gcSecondCorrectVal, err := ldb.db.Get(gcSecondCorrectKey)
	if err != nil {
		t.Fatal(err)
	}
	warpedGCVal := make([]byte, len(gcSecondCorrectVal)+1)
	copy(warpedGCVal[1:], gcSecondCorrectVal)
	if err := ldb.db.Delete(gcSecondCorrectKey); err != nil {
		t.Fatal(err)
	}
	if err := ldb.db.Put(gcSecondCorrectKey, warpedGCVal); err != nil {
		t.Fatal(err)
	}

	if err := ldb.CleanGCIndex(); err != nil {
		t.Fatal(err)
	}

//没有相应数据的索引应该已被删除
	idxKey := make([]byte, 33)
	idxKey[0] = keyIndex
	copy(idxKey[1:], chunks[0].Address())
	if _, err := ldb.db.Get(idxKey); err == nil {
		t.Fatalf("expected chunk 0 idx to be pruned: %v", idxKey)
	}

//另外两个指数也应该存在
	copy(idxKey[1:], chunks[1].Address())
	if _, err := ldb.db.Get(idxKey); err != nil {
		t.Fatalf("expected chunk 1 idx to be present: %v", idxKey)
	}

	copy(idxKey[1:], chunks[2].Address())
	if _, err := ldb.db.Get(idxKey); err != nil {
		t.Fatalf("expected chunk 2 idx to be present: %v", idxKey)
	}

//第一个GC索引应该仍然不存在
	if _, err := ldb.db.Get(gcFirstCorrectKey); err == nil {
		t.Fatalf("expected gc 0 idx to be pruned: %v", idxKey)
	}

//第二个GC索引应该仍然是固定的
	if _, err := ldb.db.Get(gcSecondCorrectKey); err != nil {
		t.Fatalf("expected gc 1 idx to be present: %v", idxKey)
	}

//第三个GC索引应保持不变
	binary.BigEndian.PutUint64(gcSecondCorrectKey[1:], uint64(2))
	if _, err := ldb.db.Get(gcSecondCorrectKey); err != nil {
		t.Fatalf("expected gc 2 idx to be present: %v", idxKey)
	}

	c, err := ldb.db.Get(keyEntryCnt)
	if err != nil {
		t.Fatalf("expected gc 2 idx to be present: %v", idxKey)
	}

//EntryCount现在应该少一个
	entryCount := binary.BigEndian.Uint64(c)
	if entryCount != 2 {
		t.Fatalf("expected entrycnt to be 2, was %d", c)
	}

//块可能意外地在同一个容器中
//如果是这样的话，这个bin计数器现在将是2-最大的添加索引。
//如果没有，总共是3个
	poBins := []uint8{ldb.po(chunks[1].Address()), ldb.po(chunks[2].Address())}
	if poBins[0] == poBins[1] {
		poBins = poBins[:1]
	}

	var binTotal uint64
	var currentBin [2]byte
	currentBin[0] = keyDistanceCnt
	if len(poBins) == 1 {
		currentBin[1] = poBins[0]
		c, err := ldb.db.Get(currentBin[:])
		if err != nil {
			t.Fatalf("expected gc 2 idx to be present: %v", idxKey)
		}
		binCount := binary.BigEndian.Uint64(c)
		if binCount != 2 {
			t.Fatalf("expected entrycnt to be 2, was %d", binCount)
		}
	} else {
		for _, bin := range poBins {
			currentBin[1] = bin
			c, err := ldb.db.Get(currentBin[:])
			if err != nil {
				t.Fatalf("expected gc 2 idx to be present: %v", idxKey)
			}
			binCount := binary.BigEndian.Uint64(c)
			binTotal += binCount

		}
		if binTotal != 3 {
			t.Fatalf("expected sum of bin indices to be 3, was %d", binTotal)
		}
	}

//检查迭代器是否正确退出
	chunks, err = mputRandomChunks(ldb, 4100, 4096)
	if err != nil {
		t.Fatal(err)
	}

	po = ldb.po(chunks[4099].Address()[:])
	dataKey = make([]byte, 10)
	dataKey[0] = keyData
	dataKey[1] = byte(po)
	binary.BigEndian.PutUint64(dataKey[2:], 4099+3)
	if _, err := ldb.db.Get(dataKey); err != nil {
		t.Fatal(err)
	}
	if err := ldb.db.Delete(dataKey); err != nil {
		t.Fatal(err)
	}

	if err := ldb.CleanGCIndex(); err != nil {
		t.Fatal(err)
	}

//EntryCount现在应该是添加的块中少一个
	c, err = ldb.db.Get(keyEntryCnt)
	if err != nil {
		t.Fatalf("expected gc 2 idx to be present: %v", idxKey)
	}
	entryCount = binary.BigEndian.Uint64(c)
	if entryCount != 4099+2 {
		t.Fatalf("expected entrycnt to be 2, was %d", c)
	}
}

func waitGc(ctx context.Context, ldb *LDBStore) {
	<-ldb.gc.runC
	ldb.gc.runC <- struct{}{}
}
