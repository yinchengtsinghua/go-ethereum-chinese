
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
	"crypto"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/bmt"
	ch "github.com/ethereum/go-ethereum/swarm/chunk"
	"golang.org/x/crypto/sha3"
)

const MaxPO = 16
const AddressLength = 32

type SwarmHasher func() SwarmHash

type Address []byte

//接近（x，y）返回x和y之间的最高位距离的接近顺序。
//
//两个等长字节序列x和y的距离度量msb（x，y）是
//x^y的二进制整数转换值，即x和y位xor-ed。
//二进制转换是big-endian：最高有效位优先（=msb）。
//
//邻近度（x，y）是最大有效位距离的离散对数标度。
//它被定义为基2整数部分的倒序。
//距离的对数。
//它是通过计算（msb）中公共前导零的数目来计算的。
//x^y的二进制表示。
//
//（最远0个，最近255个，自己256个）
func Proximity(one, other []byte) (ret int) {
	b := (MaxPO-1)/8 + 1
	if b > len(one) {
		b = len(one)
	}
	m := 8
	for i := 0; i < b; i++ {
		oxo := one[i] ^ other[i]
		for j := 0; j < m; j++ {
			if (oxo>>uint8(7-j))&0x01 != 0 {
				return i*8 + j
			}
		}
	}
	return MaxPO
}

var ZeroAddr = Address(common.Hash{}.Bytes())

func MakeHashFunc(hash string) SwarmHasher {
	switch hash {
	case "SHA256":
		return func() SwarmHash { return &HashWithLength{crypto.SHA256.New()} }
	case "SHA3":
		return func() SwarmHash { return &HashWithLength{sha3.NewLegacyKeccak256()} }
	case "BMT":
		return func() SwarmHash {
			hasher := sha3.NewLegacyKeccak256
			hasherSize := hasher().Size()
			segmentCount := ch.DefaultSize / hasherSize
			pool := bmt.NewTreePool(hasher, segmentCount, bmt.PoolSize)
			return bmt.New(pool)
		}
	}
	return nil
}

func (a Address) Hex() string {
	return fmt.Sprintf("%064x", []byte(a[:]))
}

func (a Address) Log() string {
	if len(a[:]) < 8 {
		return fmt.Sprintf("%x", []byte(a[:]))
	}
	return fmt.Sprintf("%016x", []byte(a[:8]))
}

func (a Address) String() string {
	return fmt.Sprintf("%064x", []byte(a))
}

func (a Address) MarshalJSON() (out []byte, err error) {
	return []byte(`"` + a.String() + `"`), nil
}

func (a *Address) UnmarshalJSON(value []byte) error {
	s := string(value)
	*a = make([]byte, 32)
	h := common.Hex2Bytes(s[1 : len(s)-1])
	copy(*a, h)
	return nil
}

type AddressCollection []Address

func NewAddressCollection(l int) AddressCollection {
	return make(AddressCollection, l)
}

func (c AddressCollection) Len() int {
	return len(c)
}

func (c AddressCollection) Less(i, j int) bool {
	return bytes.Compare(c[i], c[j]) == -1
}

func (c AddressCollection) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

//由context.context和数据块实现的块接口
type Chunk interface {
	Address() Address
	Data() []byte
}

type chunk struct {
	addr  Address
	sdata []byte
	span  int64
}

func NewChunk(addr Address, data []byte) *chunk {
	return &chunk{
		addr:  addr,
		sdata: data,
		span:  -1,
	}
}

func (c *chunk) Address() Address {
	return c.addr
}

func (c *chunk) Data() []byte {
	return c.sdata
}

//字符串（）用于漂亮打印
func (self *chunk) String() string {
	return fmt.Sprintf("Address: %v TreeSize: %v Chunksize: %v", self.addr.Log(), self.span, len(self.sdata))
}

func GenerateRandomChunk(dataSize int64) Chunk {
	hasher := MakeHashFunc(DefaultHash)()
	sdata := make([]byte, dataSize+8)
	rand.Read(sdata[8:])
	binary.LittleEndian.PutUint64(sdata[:8], uint64(dataSize))
	hasher.ResetWithLength(sdata[:8])
	hasher.Write(sdata[8:])
	return NewChunk(hasher.Sum(nil), sdata)
}

func GenerateRandomChunks(dataSize int64, count int) (chunks []Chunk) {
	for i := 0; i < count; i++ {
		ch := GenerateRandomChunk(dataSize)
		chunks = append(chunks, ch)
	}
	return chunks
}

//大小、查找、读取、读取
type LazySectionReader interface {
	Context() context.Context
	Size(context.Context, chan bool) (int64, error)
	io.Seeker
	io.Reader
	io.ReaderAt
}

type LazyTestSectionReader struct {
	*io.SectionReader
}

func (r *LazyTestSectionReader) Size(context.Context, chan bool) (int64, error) {
	return r.SectionReader.Size(), nil
}

func (r *LazyTestSectionReader) Context() context.Context {
	return context.TODO()
}

type StoreParams struct {
	Hash          SwarmHasher `toml:"-"`
	DbCapacity    uint64
	CacheCapacity uint
	BaseKey       []byte
}

func NewDefaultStoreParams() *StoreParams {
	return NewStoreParams(defaultLDBCapacity, defaultCacheCapacity, nil, nil)
}

func NewStoreParams(ldbCap uint64, cacheCap uint, hash SwarmHasher, basekey []byte) *StoreParams {
	if basekey == nil {
		basekey = make([]byte, 32)
	}
	if hash == nil {
		hash = MakeHashFunc(DefaultHash)
	}
	return &StoreParams{
		Hash:          hash,
		DbCapacity:    ldbCap,
		CacheCapacity: cacheCap,
		BaseKey:       basekey,
	}
}

type ChunkData []byte

type Reference []byte

//推杆负责存储数据并为其创建参考。
type Putter interface {
	Put(context.Context, ChunkData) (Reference, error)
//refsize返回此推杆创建的参考长度
	RefSize() int64
//CLOSE是指在这个推杆上不再放置数据块。
	Close()
//如果所有数据都已存储并调用了close（），则wait返回。
	Wait(context.Context) error
}

//getter是一个通过引用来检索块数据的接口。
type Getter interface {
	Get(context.Context, Reference) (ChunkData, error)
}

//注意：如果块被加密，这将返回无效数据。
func (c ChunkData) Size() uint64 {
	return binary.LittleEndian.Uint64(c[:8])
}

type ChunkValidator interface {
	Validate(chunk Chunk) bool
}

//提供对内容地址进行块验证的方法
//保留相应的哈希值以创建地址
type ContentAddressValidator struct {
	Hasher SwarmHasher
}

//构造函数
func NewContentAddressValidator(hasher SwarmHasher) *ContentAddressValidator {
	return &ContentAddressValidator{
		Hasher: hasher,
	}
}

//验证给定密钥是否为给定数据的有效内容地址
func (v *ContentAddressValidator) Validate(chunk Chunk) bool {
	data := chunk.Data()
	if l := len(data); l < 9 || l > ch.DefaultSize+8 {
//log.error（“块大小无效”，“块”，addr.hex（），“大小”，l）
		return false
	}

	hasher := v.Hasher()
	hasher.ResetWithLength(data[:8])
	hasher.Write(data[8:])
	hash := hasher.Sum(nil)

	return bytes.Equal(hash, chunk.Address())
}

type ChunkStore interface {
	Put(ctx context.Context, ch Chunk) (err error)
	Get(rctx context.Context, ref Address) (ch Chunk, err error)
	Close()
}

//SyncChunkStore是支持同步的ChunkStore
type SyncChunkStore interface {
	ChunkStore
	BinIndex(po uint8) uint64
	Iterator(from uint64, to uint64, po uint8, f func(Address, uint64) bool) error
	FetchFunc(ctx context.Context, ref Address) func(context.Context) error
}

//FakeChunkStore不存储任何内容，只实现ChunkStore接口
//如果您不想实际存储数据，可以使用它来注入哈希存储，只需执行
//哈希
type FakeChunkStore struct {
}

//Put不存储任何东西它只是在这里实现chunkstore
func (f *FakeChunkStore) Put(_ context.Context, ch Chunk) error {
	return nil
}

//gut不存储任何东西，它只是在这里实现chunkstore
func (f *FakeChunkStore) Get(_ context.Context, ref Address) (Chunk, error) {
	panic("FakeChunkStore doesn't support Get")
}

//close不存储任何内容，它只是在这里实现chunkstore
func (f *FakeChunkStore) Close() {
}
