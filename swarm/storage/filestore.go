
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
	"io"
)

/*
filestore提供客户端API入口点存储和检索到存储和检索
它可以存储任何具有字节片表示的内容，例如文件或序列化对象等。

存储：filestore调用chunker将任意大小的输入数据流分段到一个merkle散列的块树。根块的键将返回到客户端。

检索：给定根块的键，文件存储将检索块块块并重建原始数据，并将其作为一个懒惰的读卡器传递回去。懒惰的读卡器是具有按需延迟处理的读卡器，即，只有在实际读取文档的特定部分时，才提取和处理重建大型文件所需的块。

当chunker生成块时，filestore将它们发送到自己的块存储区。
用于存储或检索的实现。
**/


const (
defaultLDBCapacity                = 5000000 //LEVELDB的容量，默认为5*10^6*4096字节==20GB
defaultCacheCapacity              = 10000   //内存块缓存的容量
defaultChunkRequestsCacheCapacity = 5000000 //容纳块的传出请求的容器的容量。应设置为LEVELDB容量
)

type FileStore struct {
	ChunkStore
	hashFunc SwarmHasher
}

type FileStoreParams struct {
	Hash string
}

func NewFileStoreParams() *FileStoreParams {
	return &FileStoreParams{
		Hash: DefaultHash,
	}
}

//用于本地测试
func NewLocalFileStore(datadir string, basekey []byte) (*FileStore, error) {
	params := NewDefaultLocalStoreParams()
	params.Init(datadir)
	localStore, err := NewLocalStore(params, nil)
	if err != nil {
		return nil, err
	}
	localStore.Validators = append(localStore.Validators, NewContentAddressValidator(MakeHashFunc(DefaultHash)))
	return NewFileStore(localStore, NewFileStoreParams()), nil
}

func NewFileStore(store ChunkStore, params *FileStoreParams) *FileStore {
	hashFunc := MakeHashFunc(params.Hash)
	return &FileStore{
		ChunkStore: store,
		hashFunc:   hashFunc,
	}
}

//公共API。文档直接检索的主要入口点。使用的
//支持fs的api和httpaccess
//NetStore请求上的区块检索块超时，因此读卡器将
//如果在请求的范围内检索块超时，则报告错误。
//它返回一个读卡器，其中包含块数据以及内容是否加密。
func (f *FileStore) Retrieve(ctx context.Context, addr Address) (reader *LazyChunkReader, isEncrypted bool) {
	isEncrypted = len(addr) > f.hashFunc().Size()
	getter := NewHasherStore(f.ChunkStore, f.hashFunc, isEncrypted)
	reader = TreeJoin(ctx, addr, getter, 0)
	return
}

//公共API。文档直接存储的主要入口点。使用的
//支持fs的api和httpaccess
func (f *FileStore) Store(ctx context.Context, data io.Reader, size int64, toEncrypt bool) (addr Address, wait func(context.Context) error, err error) {
	putter := NewHasherStore(f.ChunkStore, f.hashFunc, toEncrypt)
	return PyramidSplit(ctx, data, putter, putter)
}

func (f *FileStore) HashSize() int {
	return f.hashFunc().Size()
}
