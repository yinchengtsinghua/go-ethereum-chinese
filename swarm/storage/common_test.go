
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
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	ch "github.com/ethereum/go-ethereum/swarm/chunk"
	"github.com/mattn/go-colorable"
)

var (
	loglevel   = flag.Int("loglevel", 3, "verbosity of logs")
	getTimeout = 30 * time.Second
)

func init() {
	flag.Parse()
	log.PrintOrigins(true)
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(true))))
}

type brokenLimitedReader struct {
	lr    io.Reader
	errAt int
	off   int
	size  int
}

func brokenLimitReader(data io.Reader, size int, errAt int) *brokenLimitedReader {
	return &brokenLimitedReader{
		lr:    data,
		errAt: errAt,
		size:  size,
	}
}

func newLDBStore(t *testing.T) (*LDBStore, func()) {
	dir, err := ioutil.TempDir("", "bzz-storage-test")
	if err != nil {
		t.Fatal(err)
	}
	log.Trace("memstore.tempdir", "dir", dir)

	ldbparams := NewLDBStoreParams(NewDefaultStoreParams(), dir)
	db, err := NewLDBStore(ldbparams)
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		db.Close()
		err := os.RemoveAll(dir)
		if err != nil {
			t.Fatal(err)
		}
	}

	return db, cleanup
}

func mputRandomChunks(store ChunkStore, n int, chunksize int64) ([]Chunk, error) {
	return mput(store, n, GenerateRandomChunk)
}

func mput(store ChunkStore, n int, f func(i int64) Chunk) (hs []Chunk, err error) {
//放入本地存储并等待存储的通道
//不检查传递错误状态
	errc := make(chan error)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for i := int64(0); i < int64(n); i++ {
		chunk := f(ch.DefaultSize)
		go func() {
			select {
			case errc <- store.Put(ctx, chunk):
			case <-ctx.Done():
			}
		}()
		hs = append(hs, chunk)
	}

//等待存储所有块
	for i := 0; i < n; i++ {
		err := <-errc
		if err != nil {
			return nil, err
		}
	}
	return hs, nil
}

func mget(store ChunkStore, hs []Address, f func(h Address, chunk Chunk) error) error {
	wg := sync.WaitGroup{}
	wg.Add(len(hs))
	errc := make(chan error)

	for _, k := range hs {
		go func(h Address) {
			defer wg.Done()
//TODO:使用上下文写入超时
			chunk, err := store.Get(context.TODO(), h)
			if err != nil {
				errc <- err
				return
			}
			if f != nil {
				err = f(h, chunk)
				if err != nil {
					errc <- err
					return
				}
			}
		}(k)
	}
	go func() {
		wg.Wait()
		close(errc)
	}()
	var err error
	select {
	case err = <-errc:
	case <-time.NewTimer(5 * time.Second).C:
		err = fmt.Errorf("timed out after 5 seconds")
	}
	return err
}

func (r *brokenLimitedReader) Read(buf []byte) (int, error) {
	if r.off+len(buf) > r.errAt {
		return 0, fmt.Errorf("Broken reader")
	}
	r.off += len(buf)
	return r.lr.Read(buf)
}

func testStoreRandom(m ChunkStore, n int, chunksize int64, t *testing.T) {
	chunks, err := mputRandomChunks(m, n, chunksize)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = mget(m, chunkAddresses(chunks), nil)
	if err != nil {
		t.Fatalf("testStore failed: %v", err)
	}
}

func testStoreCorrect(m ChunkStore, n int, chunksize int64, t *testing.T) {
	chunks, err := mputRandomChunks(m, n, chunksize)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	f := func(h Address, chunk Chunk) error {
		if !bytes.Equal(h, chunk.Address()) {
			return fmt.Errorf("key does not match retrieved chunk Address")
		}
		hasher := MakeHashFunc(DefaultHash)()
		data := chunk.Data()
		hasher.ResetWithLength(data[:8])
		hasher.Write(data[8:])
		exp := hasher.Sum(nil)
		if !bytes.Equal(h, exp) {
			return fmt.Errorf("key is not hash of chunk data")
		}
		return nil
	}
	err = mget(m, chunkAddresses(chunks), f)
	if err != nil {
		t.Fatalf("testStore failed: %v", err)
	}
}

func benchmarkStorePut(store ChunkStore, n int, chunksize int64, b *testing.B) {
	chunks := make([]Chunk, n)
	i := 0
	f := func(dataSize int64) Chunk {
		chunk := GenerateRandomChunk(dataSize)
		chunks[i] = chunk
		i++
		return chunk
	}

	mput(store, n, f)

	f = func(dataSize int64) Chunk {
		chunk := chunks[i]
		i++
		return chunk
	}

	b.ReportAllocs()
	b.ResetTimer()

	for j := 0; j < b.N; j++ {
		i = 0
		mput(store, n, f)
	}
}

func benchmarkStoreGet(store ChunkStore, n int, chunksize int64, b *testing.B) {
	chunks, err := mputRandomChunks(store, n, chunksize)
	if err != nil {
		b.Fatalf("expected no error, got %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	addrs := chunkAddresses(chunks)
	for i := 0; i < b.N; i++ {
		err := mget(store, addrs, nil)
		if err != nil {
			b.Fatalf("mget failed: %v", err)
		}
	}
}

//map chunkstore是一个非常简单的chunkstore实现，用于将块存储在内存中的映射中。
type MapChunkStore struct {
	chunks map[string]Chunk
	mu     sync.RWMutex
}

func NewMapChunkStore() *MapChunkStore {
	return &MapChunkStore{
		chunks: make(map[string]Chunk),
	}
}

func (m *MapChunkStore) Put(_ context.Context, ch Chunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chunks[ch.Address().Hex()] = ch
	return nil
}

func (m *MapChunkStore) Get(_ context.Context, ref Address) (Chunk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	chunk := m.chunks[ref.Hex()]
	if chunk == nil {
		return nil, ErrChunkNotFound
	}
	return chunk, nil
}

func (m *MapChunkStore) Close() {
}

func chunkAddresses(chunks []Chunk) []Address {
	addrs := make([]Address, len(chunks))
	for i, ch := range chunks {
		addrs[i] = ch.Address()
	}
	return addrs
}
