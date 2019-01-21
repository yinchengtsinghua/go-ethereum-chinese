
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

package storage

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	ch "github.com/ethereum/go-ethereum/swarm/chunk"
)

var (
	hashfunc = MakeHashFunc(DefaultHash)
)

//测试内容地址验证器是否正确检查数据
//通过内容地址验证器传递源更新块的测试
//检查资源更新验证器内部正确性的测试在storage/feeds/handler_test.go中找到。
func TestValidator(t *testing.T) {
//设置本地存储
	datadir, err := ioutil.TempDir("", "storage-testvalidator")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(datadir)

	params := NewDefaultLocalStoreParams()
	params.Init(datadir)
	store, err := NewLocalStore(params, nil)
	if err != nil {
		t.Fatal(err)
	}

//不带验证器的检验结果，均成功
	chunks := GenerateRandomChunks(259, 2)
	goodChunk := chunks[0]
	badChunk := chunks[1]
	copy(badChunk.Data(), goodChunk.Data())

	errs := putChunks(store, goodChunk, badChunk)
	if errs[0] != nil {
		t.Fatalf("expected no error on good content address chunk in spite of no validation, but got: %s", err)
	}
	if errs[1] != nil {
		t.Fatalf("expected no error on bad content address chunk in spite of no validation, but got: %s", err)
	}

//添加内容地址验证程序并检查Puts
//坏的应该失败，好的应该通过。
	store.Validators = append(store.Validators, NewContentAddressValidator(hashfunc))
	chunks = GenerateRandomChunks(ch.DefaultSize, 2)
	goodChunk = chunks[0]
	badChunk = chunks[1]
	copy(badChunk.Data(), goodChunk.Data())

	errs = putChunks(store, goodChunk, badChunk)
	if errs[0] != nil {
		t.Fatalf("expected no error on good content address chunk with content address validator only, but got: %s", err)
	}
	if errs[1] == nil {
		t.Fatal("expected error on bad content address chunk with content address validator only, but got nil")
	}

//附加一个始终拒绝的验证器
//坏的应该失败，好的应该通过，
	var negV boolTestValidator
	store.Validators = append(store.Validators, negV)

	chunks = GenerateRandomChunks(ch.DefaultSize, 2)
	goodChunk = chunks[0]
	badChunk = chunks[1]
	copy(badChunk.Data(), goodChunk.Data())

	errs = putChunks(store, goodChunk, badChunk)
	if errs[0] != nil {
		t.Fatalf("expected no error on good content address chunk with content address validator only, but got: %s", err)
	}
	if errs[1] == nil {
		t.Fatal("expected error on bad content address chunk with content address validator only, but got nil")
	}

//附加一个始终批准的验证器
//一切都将通过
	var posV boolTestValidator = true
	store.Validators = append(store.Validators, posV)

	chunks = GenerateRandomChunks(ch.DefaultSize, 2)
	goodChunk = chunks[0]
	badChunk = chunks[1]
	copy(badChunk.Data(), goodChunk.Data())

	errs = putChunks(store, goodChunk, badChunk)
	if errs[0] != nil {
		t.Fatalf("expected no error on good content address chunk with content address validator only, but got: %s", err)
	}
	if errs[1] != nil {
		t.Fatalf("expected no error on bad content address chunk in spite of no validation, but got: %s", err)
	}

}

type boolTestValidator bool

func (self boolTestValidator) Validate(chunk Chunk) bool {
	return bool(self)
}

//PutChunks将块添加到LocalStore
//它等待存储通道上的接收
//它记录但在传递错误时不会失败
func putChunks(store *LocalStore, chunks ...Chunk) []error {
	i := 0
	f := func(n int64) Chunk {
		chunk := chunks[i]
		i++
		return chunk
	}
	_, errs := put(store, len(chunks), f)
	return errs
}

func put(store *LocalStore, n int, f func(i int64) Chunk) (hs []Address, errs []error) {
	for i := int64(0); i < int64(n); i++ {
		chunk := f(ch.DefaultSize)
		err := store.Put(context.TODO(), chunk)
		errs = append(errs, err)
		hs = append(hs, chunk.Address())
	}
	return hs, errs
}

//testgetfrequentlyaccessedchunkwontgetgarbag收集的测试
//频繁访问的块不是从ldbstore收集的垃圾，即，
//当我们达到容量并且垃圾收集器运行时，从磁盘开始。为此
//我们开始将随机块放入数据库，同时不断地访问
//我们关心的块，然后检查我们是否仍然可以从磁盘中检索到它。
func TestGetFrequentlyAccessedChunkWontGetGarbageCollected(t *testing.T) {
	ldbCap := defaultGCRatio
	store, cleanup := setupLocalStore(t, ldbCap)
	defer cleanup()

	var chunks []Chunk
	for i := 0; i < ldbCap; i++ {
		chunks = append(chunks, GenerateRandomChunk(ch.DefaultSize))
	}

	mostAccessed := chunks[0].Address()
	for _, chunk := range chunks {
		if err := store.Put(context.Background(), chunk); err != nil {
			t.Fatal(err)
		}

		if _, err := store.Get(context.Background(), mostAccessed); err != nil {
			t.Fatal(err)
		}
//添加markaccessed（）在单独的goroutine中完成的时间
		time.Sleep(1 * time.Millisecond)
	}

	store.DbStore.collectGarbage()
	if _, err := store.DbStore.Get(context.Background(), mostAccessed); err != nil {
		t.Logf("most frequntly accessed chunk not found on disk (key: %v)", mostAccessed)
		t.Fatal(err)
	}

}

func setupLocalStore(t *testing.T, ldbCap int) (ls *LocalStore, cleanup func()) {
	t.Helper()

	var err error
	datadir, err := ioutil.TempDir("", "storage")
	if err != nil {
		t.Fatal(err)
	}

	params := &LocalStoreParams{
		StoreParams: NewStoreParams(uint64(ldbCap), uint(ldbCap), nil, nil),
	}
	params.Init(datadir)

	store, err := NewLocalStore(params, nil)
	if err != nil {
		_ = os.RemoveAll(datadir)
		t.Fatal(err)
	}

	cleanup = func() {
		store.Close()
		_ = os.RemoveAll(datadir)
	}

	return store, cleanup
}
