
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
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/swarm/testutil"
)

const testDataSize = 0x0001000

func TestFileStorerandom(t *testing.T) {
	testFileStoreRandom(false, t)
	testFileStoreRandom(true, t)
}

func testFileStoreRandom(toEncrypt bool, t *testing.T) {
	tdb, cleanup, err := newTestDbStore(false, false)
	defer cleanup()
	if err != nil {
		t.Fatalf("init dbStore failed: %v", err)
	}
	db := tdb.LDBStore
	db.setCapacity(50000)
	memStore := NewMemStore(NewDefaultStoreParams(), db)
	localStore := &LocalStore{
		memStore: memStore,
		DbStore:  db,
	}

	fileStore := NewFileStore(localStore, NewFileStoreParams())
	defer os.RemoveAll("/tmp/bzz")

	slice := testutil.RandomBytes(1, testDataSize)
	ctx := context.TODO()
	key, wait, err := fileStore.Store(ctx, bytes.NewReader(slice), testDataSize, toEncrypt)
	if err != nil {
		t.Fatalf("Store error: %v", err)
	}
	err = wait(ctx)
	if err != nil {
		t.Fatalf("Store waitt error: %v", err.Error())
	}
	resultReader, isEncrypted := fileStore.Retrieve(context.TODO(), key)
	if isEncrypted != toEncrypt {
		t.Fatalf("isEncrypted expected %v got %v", toEncrypt, isEncrypted)
	}
	resultSlice := make([]byte, testDataSize)
	n, err := resultReader.ReadAt(resultSlice, 0)
	if err != io.EOF {
		t.Fatalf("Retrieve error: %v", err)
	}
	if n != testDataSize {
		t.Fatalf("Slice size error got %d, expected %d.", n, testDataSize)
	}
	if !bytes.Equal(slice, resultSlice) {
		t.Fatalf("Comparison error.")
	}
	ioutil.WriteFile("/tmp/slice.bzz.16M", slice, 0666)
	ioutil.WriteFile("/tmp/result.bzz.16M", resultSlice, 0666)
	localStore.memStore = NewMemStore(NewDefaultStoreParams(), db)
	resultReader, isEncrypted = fileStore.Retrieve(context.TODO(), key)
	if isEncrypted != toEncrypt {
		t.Fatalf("isEncrypted expected %v got %v", toEncrypt, isEncrypted)
	}
	for i := range resultSlice {
		resultSlice[i] = 0
	}
	n, err = resultReader.ReadAt(resultSlice, 0)
	if err != io.EOF {
		t.Fatalf("Retrieve error after removing memStore: %v", err)
	}
	if n != len(slice) {
		t.Fatalf("Slice size error after removing memStore got %d, expected %d.", n, len(slice))
	}
	if !bytes.Equal(slice, resultSlice) {
		t.Fatalf("Comparison error after removing memStore.")
	}
}

func TestFileStoreCapacity(t *testing.T) {
	testFileStoreCapacity(false, t)
	testFileStoreCapacity(true, t)
}

func testFileStoreCapacity(toEncrypt bool, t *testing.T) {
	tdb, cleanup, err := newTestDbStore(false, false)
	defer cleanup()
	if err != nil {
		t.Fatalf("init dbStore failed: %v", err)
	}
	db := tdb.LDBStore
	memStore := NewMemStore(NewDefaultStoreParams(), db)
	localStore := &LocalStore{
		memStore: memStore,
		DbStore:  db,
	}
	fileStore := NewFileStore(localStore, NewFileStoreParams())
	slice := testutil.RandomBytes(1, testDataSize)
	ctx := context.TODO()
	key, wait, err := fileStore.Store(ctx, bytes.NewReader(slice), testDataSize, toEncrypt)
	if err != nil {
		t.Errorf("Store error: %v", err)
	}
	err = wait(ctx)
	if err != nil {
		t.Fatalf("Store error: %v", err)
	}
	resultReader, isEncrypted := fileStore.Retrieve(context.TODO(), key)
	if isEncrypted != toEncrypt {
		t.Fatalf("isEncrypted expected %v got %v", toEncrypt, isEncrypted)
	}
	resultSlice := make([]byte, len(slice))
	n, err := resultReader.ReadAt(resultSlice, 0)
	if err != io.EOF {
		t.Fatalf("Retrieve error: %v", err)
	}
	if n != len(slice) {
		t.Fatalf("Slice size error got %d, expected %d.", n, len(slice))
	}
	if !bytes.Equal(slice, resultSlice) {
		t.Fatalf("Comparison error.")
	}
//清除内存
	memStore.setCapacity(0)
//检查它是否确实是空的
	fileStore.ChunkStore = memStore
	resultReader, isEncrypted = fileStore.Retrieve(context.TODO(), key)
	if isEncrypted != toEncrypt {
		t.Fatalf("isEncrypted expected %v got %v", toEncrypt, isEncrypted)
	}
	if _, err = resultReader.ReadAt(resultSlice, 0); err == nil {
		t.Fatalf("Was able to read %d bytes from an empty memStore.", len(slice))
	}
//检查它如何与本地商店一起工作
	fileStore.ChunkStore = localStore
//localstore.dbstore.setCapacity（0）
	resultReader, isEncrypted = fileStore.Retrieve(context.TODO(), key)
	if isEncrypted != toEncrypt {
		t.Fatalf("isEncrypted expected %v got %v", toEncrypt, isEncrypted)
	}
	for i := range resultSlice {
		resultSlice[i] = 0
	}
	n, err = resultReader.ReadAt(resultSlice, 0)
	if err != io.EOF {
		t.Fatalf("Retrieve error after clearing memStore: %v", err)
	}
	if n != len(slice) {
		t.Fatalf("Slice size error after clearing memStore got %d, expected %d.", n, len(slice))
	}
	if !bytes.Equal(slice, resultSlice) {
		t.Fatalf("Comparison error after clearing memStore.")
	}
}
