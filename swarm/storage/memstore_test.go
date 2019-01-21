
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
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/swarm/log"
)

func newTestMemStore() *MemStore {
	storeparams := NewDefaultStoreParams()
	return NewMemStore(storeparams, nil)
}

func testMemStoreRandom(n int, chunksize int64, t *testing.T) {
	m := newTestMemStore()
	defer m.Close()
	testStoreRandom(m, n, chunksize, t)
}

func testMemStoreCorrect(n int, chunksize int64, t *testing.T) {
	m := newTestMemStore()
	defer m.Close()
	testStoreCorrect(m, n, chunksize, t)
}

func TestMemStoreRandom_1(t *testing.T) {
	testMemStoreRandom(1, 0, t)
}

func TestMemStoreCorrect_1(t *testing.T) {
	testMemStoreCorrect(1, 4104, t)
}

func TestMemStoreRandom_1k(t *testing.T) {
	testMemStoreRandom(1000, 0, t)
}

func TestMemStoreCorrect_1k(t *testing.T) {
	testMemStoreCorrect(100, 4096, t)
}

func TestMemStoreNotFound(t *testing.T) {
	m := newTestMemStore()
	defer m.Close()

	_, err := m.Get(context.TODO(), ZeroAddr)
	if err != ErrChunkNotFound {
		t.Errorf("Expected ErrChunkNotFound, got %v", err)
	}
}

func benchmarkMemStorePut(n int, processors int, chunksize int64, b *testing.B) {
	m := newTestMemStore()
	defer m.Close()
	benchmarkStorePut(m, n, chunksize, b)
}

func benchmarkMemStoreGet(n int, processors int, chunksize int64, b *testing.B) {
	m := newTestMemStore()
	defer m.Close()
	benchmarkStoreGet(m, n, chunksize, b)
}

func BenchmarkMemStorePut_1_500(b *testing.B) {
	benchmarkMemStorePut(500, 1, 4096, b)
}

func BenchmarkMemStorePut_8_500(b *testing.B) {
	benchmarkMemStorePut(500, 8, 4096, b)
}

func BenchmarkMemStoreGet_1_500(b *testing.B) {
	benchmarkMemStoreGet(500, 1, 4096, b)
}

func BenchmarkMemStoreGet_8_500(b *testing.B) {
	benchmarkMemStoreGet(500, 8, 4096, b)
}

func TestMemStoreAndLDBStore(t *testing.T) {
	ldb, cleanup := newLDBStore(t)
	ldb.setCapacity(4000)
	defer cleanup()

	cacheCap := 200
	memStore := NewMemStore(NewStoreParams(4000, 200, nil, nil), nil)

	tests := []struct {
n         int   //要推送到memstore的块数
chunkSize int64 //块的大小（在Swarm-4096中默认）
	}{
		{
			n:         1,
			chunkSize: 4096,
		},
		{
			n:         101,
			chunkSize: 4096,
		},
		{
			n:         501,
			chunkSize: 4096,
		},
		{
			n:         1100,
			chunkSize: 4096,
		},
	}

	for i, tt := range tests {
		log.Info("running test", "idx", i, "tt", tt)
		var chunks []Chunk

		for i := 0; i < tt.n; i++ {
			c := GenerateRandomChunk(tt.chunkSize)
			chunks = append(chunks, c)
		}

		for i := 0; i < tt.n; i++ {
			err := ldb.Put(context.TODO(), chunks[i])
			if err != nil {
				t.Fatal(err)
			}
			err = memStore.Put(context.TODO(), chunks[i])
			if err != nil {
				t.Fatal(err)
			}

			if got := memStore.cache.Len(); got > cacheCap {
				t.Fatalf("expected to get cache capacity less than %v, but got %v", cacheCap, got)
			}

		}

		for i := 0; i < tt.n; i++ {
			_, err := memStore.Get(context.TODO(), chunks[i].Address())
			if err != nil {
				if err == ErrChunkNotFound {
					_, err := ldb.Get(context.TODO(), chunks[i].Address())
					if err != nil {
						t.Fatalf("couldn't get chunk %v from ldb, got error: %v", i, err)
					}
				} else {
					t.Fatalf("got error from memstore: %v", err)
				}
			}
		}
	}
}
