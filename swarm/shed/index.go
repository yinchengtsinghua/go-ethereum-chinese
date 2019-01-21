
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

package shed

import (
	"bytes"

	"github.com/syndtr/goleveldb/leveldb"
)

//项目包含与群块数据和元数据相关的字段。
//群存储和操作所需的所有信息
//必须在此处定义该存储。
//这种结构在逻辑上与群存储相连，
//这个包中唯一没有被概括的部分，
//主要是出于性能原因。
//
//项是用于检索、存储和编码的类型
//块数据和元数据。它作为参数传递给索引编码
//函数、获取函数和Put函数。
//但是，它还返回来自get函数调用的附加数据
//作为迭代器函数定义中的参数。
type Item struct {
	Address         []byte
	Data            []byte
	AccessTimestamp int64
	StoreTimestamp  int64
//useMockStore是用于标识
//join函数中字段的未设置状态。
	UseMockStore *bool
}

//合并是一个帮助器方法，用于构造新的
//用默认值填充字段
//一个特定项的值来自另一个项。
func (i Item) Merge(i2 Item) (new Item) {
	if i.Address == nil {
		i.Address = i2.Address
	}
	if i.Data == nil {
		i.Data = i2.Data
	}
	if i.AccessTimestamp == 0 {
		i.AccessTimestamp = i2.AccessTimestamp
	}
	if i.StoreTimestamp == 0 {
		i.StoreTimestamp = i2.StoreTimestamp
	}
	if i.UseMockStore == nil {
		i.UseMockStore = i2.UseMockStore
	}
	return i
}

//索引表示一组具有公共
//前缀。它具有编码和解码密钥和值的功能
//要对包含以下内容的已保存数据提供透明操作：
//-获取特定项目
//-保存特定项目
//-在已排序的级别数据库键上迭代
//它实现了indexIteratorInterface接口。
type Index struct {
	db              *DB
	prefix          []byte
	encodeKeyFunc   func(fields Item) (key []byte, err error)
	decodeKeyFunc   func(key []byte) (e Item, err error)
	encodeValueFunc func(fields Item) (value []byte, err error)
	decodeValueFunc func(keyFields Item, value []byte) (e Item, err error)
}

//indexfuncs结构定义了用于编码和解码的函数
//特定索引的LevelDB键和值。
type IndexFuncs struct {
	EncodeKey   func(fields Item) (key []byte, err error)
	DecodeKey   func(key []byte) (e Item, err error)
	EncodeValue func(fields Item) (value []byte, err error)
	DecodeValue func(keyFields Item, value []byte) (e Item, err error)
}

//new index返回一个新的索引实例，该实例具有定义的名称和
//编码函数。名称必须唯一并将被验证
//对于键前缀字节的数据库架构。
func (db *DB) NewIndex(name string, funcs IndexFuncs) (f Index, err error) {
	id, err := db.schemaIndexPrefix(name)
	if err != nil {
		return f, err
	}
	prefix := []byte{id}
	return Index{
		db:     db,
		prefix: prefix,
//此函数用于调整索引级别DB键
//通过附加提供的索引ID字节。
//这是为了避免不同钥匙之间的碰撞
//索引，因为所有索引ID都是唯一的。
		encodeKeyFunc: func(e Item) (key []byte, err error) {
			key, err = funcs.EncodeKey(e)
			if err != nil {
				return nil, err
			}
			return append(append(make([]byte, 0, len(key)+1), prefix...), key...), nil
		},
//此函数用于反转EncodeKeyFunc构造的键
//在没有索引ID的情况下透明地使用索引键。
//它假定索引键的前缀只有一个字节。
		decodeKeyFunc: func(key []byte) (e Item, err error) {
			return funcs.DecodeKey(key[1:])
		},
		encodeValueFunc: funcs.EncodeValue,
		decodeValueFunc: funcs.DecodeValue,
	}, nil
}

//get接受表示为要检索的项的键字段
//索引中的值并返回最大可用信息
//从另一项表示的索引。
func (f Index) Get(keyFields Item) (out Item, err error) {
	key, err := f.encodeKeyFunc(keyFields)
	if err != nil {
		return out, err
	}
	value, err := f.db.Get(key)
	if err != nil {
		return out, err
	}
	out, err = f.decodeValueFunc(keyFields, value)
	if err != nil {
		return out, err
	}
	return out.Merge(keyFields), nil
}

//放置接受项以对来自它的信息进行编码
//并保存到数据库。
func (f Index) Put(i Item) (err error) {
	key, err := f.encodeKeyFunc(i)
	if err != nil {
		return err
	}
	value, err := f.encodeValueFunc(i)
	if err != nil {
		return err
	}
	return f.db.Put(key, value)
}

//putinbatch与put方法相同，但它只是
//将键/值对保存到批处理中
//直接到数据库。
func (f Index) PutInBatch(batch *leveldb.Batch, i Item) (err error) {
	key, err := f.encodeKeyFunc(i)
	if err != nil {
		return err
	}
	value, err := f.encodeValueFunc(i)
	if err != nil {
		return err
	}
	batch.Put(key, value)
	return nil
}

//删除接受项以删除键/值对
//基于其字段的数据库。
func (f Index) Delete(keyFields Item) (err error) {
	key, err := f.encodeKeyFunc(keyFields)
	if err != nil {
		return err
	}
	return f.db.Delete(key)
}

//deleteinbatch与只删除操作相同
//在批处理上而不是在数据库上执行。
func (f Index) DeleteInBatch(batch *leveldb.Batch, keyFields Item) (err error) {
	key, err := f.encodeKeyFunc(keyFields)
	if err != nil {
		return err
	}
	batch.Delete(key)
	return nil
}

//indexiterfunc是对解码的每个项的回调
//通过迭代索引键。
//通过返回一个true for stop变量，迭代将
//停止，返回错误，该错误将
//传播到索引上被调用的迭代器方法。
type IndexIterFunc func(item Item) (stop bool, err error)

//迭代器定义迭代器函数的可选参数。
type IterateOptions struct {
//StartFrom是从中开始迭代的项。
	StartFrom *Item
//如果skipStartFromItem为true，则StartFrom项不会
//重复进行。
	SkipStartFromItem bool
//迭代键具有公共前缀的项。
	Prefix []byte
}

//迭代函数迭代索引的键。
//如果迭代次数为零，则迭代次数将覆盖所有键。
func (f Index) Iterate(fn IndexIterFunc, options *IterateOptions) (err error) {
	if options == nil {
		options = new(IterateOptions)
	}
//用索引前缀和可选的公共密钥前缀构造前缀
	prefix := append(f.prefix, options.Prefix...)
//从前缀开始
	startKey := prefix
	if options.StartFrom != nil {
//从提供的StartFrom项键值开始
		startKey, err = f.encodeKeyFunc(*options.StartFrom)
		if err != nil {
			return err
		}
	}
	it := f.db.NewIterator()
	defer it.Release()

//将光标移到开始键
	ok := it.Seek(startKey)
	if !ok {
//如果查找失败，则停止迭代器
		return it.Error()
	}
	if options.SkipStartFromItem && bytes.Equal(startKey, it.Key()) {
//如果是第一个键，则跳过“从项目开始”
//它被明确配置为跳过它
		ok = it.Next()
	}
	for ; ok; ok = it.Next() {
		key := it.Key()
		if !bytes.HasPrefix(key, prefix) {
			break
		}
//创建一个键字节切片的副本，不共享级别数据库基线切片数组
		keyItem, err := f.decodeKeyFunc(append([]byte(nil), key...))
		if err != nil {
			return err
		}
//创建一个值字节切片的副本，不共享级别数据库基线切片数组
		valueItem, err := f.decodeValueFunc(keyItem, append([]byte(nil), it.Value()...))
		if err != nil {
			return err
		}
		stop, err := fn(keyItem.Merge(valueItem))
		if err != nil {
			return err
		}
		if stop {
			break
		}
	}
	return it.Error()
}

//count返回索引中的项数。
func (f Index) Count() (count int, err error) {
	it := f.db.NewIterator()
	defer it.Release()

	for ok := it.Seek(f.prefix); ok; ok = it.Next() {
		key := it.Key()
		if key[0] != f.prefix[0] {
			break
		}
		count++
	}
	return count, it.Error()
}

//CountFrom返回索引键中的项数
//从从提供的项编码的密钥开始。
func (f Index) CountFrom(start Item) (count int, err error) {
	startKey, err := f.encodeKeyFunc(start)
	if err != nil {
		return 0, err
	}
	it := f.db.NewIterator()
	defer it.Release()

	for ok := it.Seek(startKey); ok; ok = it.Next() {
		key := it.Key()
		if key[0] != f.prefix[0] {
			break
		}
		count++
	}
	return count, it.Error()
}
