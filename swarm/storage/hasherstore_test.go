
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
	"bytes"
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/swarm/storage/encryption"

	"github.com/ethereum/go-ethereum/common"
)

func TestHasherStore(t *testing.T) {
	var tests = []struct {
		chunkLength int
		toEncrypt   bool
	}{
		{10, false},
		{100, false},
		{1000, false},
		{4096, false},
		{10, true},
		{100, true},
		{1000, true},
		{4096, true},
	}

	for _, tt := range tests {
		chunkStore := NewMapChunkStore()
		hasherStore := NewHasherStore(chunkStore, MakeHashFunc(DefaultHash), tt.toEncrypt)

//将两个随机块放入哈希存储中
		chunkData1 := GenerateRandomChunk(int64(tt.chunkLength)).Data()
		ctx, cancel := context.WithTimeout(context.Background(), getTimeout)
		defer cancel()
		key1, err := hasherStore.Put(ctx, chunkData1)
		if err != nil {
			t.Fatalf("Expected no error got \"%v\"", err)
		}

		chunkData2 := GenerateRandomChunk(int64(tt.chunkLength)).Data()
		key2, err := hasherStore.Put(ctx, chunkData2)
		if err != nil {
			t.Fatalf("Expected no error got \"%v\"", err)
		}

		hasherStore.Close()

//等待块真正存储
		err = hasherStore.Wait(ctx)
		if err != nil {
			t.Fatalf("Expected no error got \"%v\"", err)
		}

//获取第一块
		retrievedChunkData1, err := hasherStore.Get(ctx, key1)
		if err != nil {
			t.Fatalf("Expected no error, got \"%v\"", err)
		}

//检索到的数据应与原始数据相同
		if !bytes.Equal(chunkData1, retrievedChunkData1) {
			t.Fatalf("Expected retrieved chunk data %v, got %v", common.Bytes2Hex(chunkData1), common.Bytes2Hex(retrievedChunkData1))
		}

//获取第二块
		retrievedChunkData2, err := hasherStore.Get(ctx, key2)
		if err != nil {
			t.Fatalf("Expected no error, got \"%v\"", err)
		}

//检索到的数据应与原始数据相同
		if !bytes.Equal(chunkData2, retrievedChunkData2) {
			t.Fatalf("Expected retrieved chunk data %v, got %v", common.Bytes2Hex(chunkData2), common.Bytes2Hex(retrievedChunkData2))
		}

		hash1, encryptionKey1, err := parseReference(key1, hasherStore.hashSize)
		if err != nil {
			t.Fatalf("Expected no error, got \"%v\"", err)
		}

		if tt.toEncrypt {
			if encryptionKey1 == nil {
				t.Fatal("Expected non-nil encryption key, got nil")
			} else if len(encryptionKey1) != encryption.KeyLength {
				t.Fatalf("Expected encryption key length %v, got %v", encryption.KeyLength, len(encryptionKey1))
			}
		}
		if !tt.toEncrypt && encryptionKey1 != nil {
			t.Fatalf("Expected nil encryption key, got key with length %v", len(encryptionKey1))
		}

//检查存储区中的块数据是否加密
		chunkInStore, err := chunkStore.Get(ctx, hash1)
		if err != nil {
			t.Fatalf("Expected no error got \"%v\"", err)
		}

		chunkDataInStore := chunkInStore.Data()

		if tt.toEncrypt && bytes.Equal(chunkData1, chunkDataInStore) {
			t.Fatalf("Chunk expected to be encrypted but it is stored without encryption")
		}
		if !tt.toEncrypt && !bytes.Equal(chunkData1, chunkDataInStore) {
			t.Fatalf("Chunk expected to be not encrypted but stored content is different. Expected %v got %v", common.Bytes2Hex(chunkData1), common.Bytes2Hex(chunkDataInStore))
		}
	}
}
