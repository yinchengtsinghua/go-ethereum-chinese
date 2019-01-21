
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

package shed_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/swarm/shed"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/syndtr/goleveldb/leveldb"
)

//存储保留字段和索引（包括其编码功能）
//并通过从中组合数据来定义对它们的操作。
//它实现了storage.chunkstore接口。
//这只是一个不支持并行操作的例子
//或者现实世界的实现。
type Store struct {
	db *shed.DB

//字段和索引
	schemaName     shed.StringField
	sizeCounter    shed.Uint64Field
	accessCounter  shed.Uint64Field
	retrievalIndex shed.Index
	accessIndex    shed.Index
	gcIndex        shed.Index
}

//新的返回新的存储。所有字段和索引都已初始化
//并检查与现有数据库中架构的可能冲突
//自动地。
func New(path string) (s *Store, err error) {
	db, err := shed.NewDB(path, "")
	if err != nil {
		return nil, err
	}
	s = &Store{
		db: db,
	}
//用任意名称标识当前存储架构。
	s.schemaName, err = db.NewStringField("schema-name")
	if err != nil {
		return nil, err
	}
//块访问的全局不断递增索引。
	s.accessCounter, err = db.NewUint64Field("access-counter")
	if err != nil {
		return nil, err
	}
//存储实际块地址、数据和存储时间戳的索引。
	s.retrievalIndex, err = db.NewIndex("Address->StoreTimestamp|Data", shed.IndexFuncs{
		EncodeKey: func(fields shed.Item) (key []byte, err error) {
			return fields.Address, nil
		},
		DecodeKey: func(key []byte) (e shed.Item, err error) {
			e.Address = key
			return e, nil
		},
		EncodeValue: func(fields shed.Item) (value []byte, err error) {
			b := make([]byte, 8)
			binary.BigEndian.PutUint64(b, uint64(fields.StoreTimestamp))
			value = append(b, fields.Data...)
			return value, nil
		},
		DecodeValue: func(keyItem shed.Item, value []byte) (e shed.Item, err error) {
			e.StoreTimestamp = int64(binary.BigEndian.Uint64(value[:8]))
			e.Data = value[8:]
			return e, nil
		},
	})
	if err != nil {
		return nil, err
	}
//存储特定地址的访问时间戳的索引。
//需要它来更新迭代顺序的GC索引键。
	s.accessIndex, err = db.NewIndex("Address->AccessTimestamp", shed.IndexFuncs{
		EncodeKey: func(fields shed.Item) (key []byte, err error) {
			return fields.Address, nil
		},
		DecodeKey: func(key []byte) (e shed.Item, err error) {
			e.Address = key
			return e, nil
		},
		EncodeValue: func(fields shed.Item) (value []byte, err error) {
			b := make([]byte, 8)
			binary.BigEndian.PutUint64(b, uint64(fields.AccessTimestamp))
			return b, nil
		},
		DecodeValue: func(keyItem shed.Item, value []byte) (e shed.Item, err error) {
			e.AccessTimestamp = int64(binary.BigEndian.Uint64(value))
			return e, nil
		},
	})
	if err != nil {
		return nil, err
	}
//索引键按访问时间戳排序，用于垃圾收集优先级。
	s.gcIndex, err = db.NewIndex("AccessTimestamp|StoredTimestamp|Address->nil", shed.IndexFuncs{
		EncodeKey: func(fields shed.Item) (key []byte, err error) {
			b := make([]byte, 16, 16+len(fields.Address))
			binary.BigEndian.PutUint64(b[:8], uint64(fields.AccessTimestamp))
			binary.BigEndian.PutUint64(b[8:16], uint64(fields.StoreTimestamp))
			key = append(b, fields.Address...)
			return key, nil
		},
		DecodeKey: func(key []byte) (e shed.Item, err error) {
			e.AccessTimestamp = int64(binary.BigEndian.Uint64(key[:8]))
			e.StoreTimestamp = int64(binary.BigEndian.Uint64(key[8:16]))
			e.Address = key[16:]
			return e, nil
		},
		EncodeValue: func(fields shed.Item) (value []byte, err error) {
			return nil, nil
		},
		DecodeValue: func(keyItem shed.Item, value []byte) (e shed.Item, err error) {
			return e, nil
		},
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}

//放置存储块并设置它存储时间戳。
func (s *Store) Put(_ context.Context, ch storage.Chunk) (err error) {
	return s.retrievalIndex.Put(shed.Item{
		Address:        ch.Address(),
		Data:           ch.Data(),
		StoreTimestamp: time.Now().UTC().UnixNano(),
	})
}

//get使用提供的地址检索块。
//它通过删除以前的索引来更新访问和GC索引
//并添加新项作为索引项的键
//被改变了。
func (s *Store) Get(_ context.Context, addr storage.Address) (c storage.Chunk, err error) {
	batch := new(leveldb.Batch)

//获取块数据和存储时间戳。
	item, err := s.retrievalIndex.Get(shed.Item{
		Address: addr,
	})
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, storage.ErrChunkNotFound
		}
		return nil, err
	}

//获取区块访问时间戳。
	accessItem, err := s.accessIndex.Get(shed.Item{
		Address: addr,
	})
	switch err {
	case nil:
//如果找到访问时间戳，则删除gc索引项。
		err = s.gcIndex.DeleteInBatch(batch, shed.Item{
			Address:         item.Address,
			StoreTimestamp:  accessItem.AccessTimestamp,
			AccessTimestamp: item.StoreTimestamp,
		})
		if err != nil {
			return nil, err
		}
	case leveldb.ErrNotFound:
//找不到访问时间戳。不要做任何事。
//这是firs get请求。
	default:
		return nil, err
	}

//指定新的访问时间戳
	accessTimestamp := time.Now().UTC().UnixNano()

//在访问索引中放置新的访问时间戳。
	err = s.accessIndex.PutInBatch(batch, shed.Item{
		Address:         addr,
		AccessTimestamp: accessTimestamp,
	})
	if err != nil {
		return nil, err
	}

//在gc索引中放置新的访问时间戳。
	err = s.gcIndex.PutInBatch(batch, shed.Item{
		Address:         item.Address,
		AccessTimestamp: accessTimestamp,
		StoreTimestamp:  item.StoreTimestamp,
	})
	if err != nil {
		return nil, err
	}

//递增访问计数器。
//目前此信息不在任何地方使用。
	_, err = s.accessCounter.IncInBatch(batch)
	if err != nil {
		return nil, err
	}

//编写批处理。
	err = s.db.WriteBatch(batch)
	if err != nil {
		return nil, err
	}

//返回块。
	return storage.NewChunk(item.Address, item.Data), nil
}

//Collect垃圾是索引迭代的一个例子。
//它不提供可靠的垃圾收集功能。
func (s *Store) CollectGarbage() (err error) {
	const maxTrashSize = 100
maxRounds := 10 //任意数，需要计算

//进行几轮GC测试。
	for roundCount := 0; roundCount < maxRounds; roundCount++ {
		var garbageCount int
//新一轮CG的新批次。
		trash := new(leveldb.Batch)
//遍历所有索引项，并在需要时中断。
		err = s.gcIndex.Iterate(func(item shed.Item) (stop bool, err error) {
//移除区块。
			err = s.retrievalIndex.DeleteInBatch(trash, item)
			if err != nil {
				return false, err
			}
//删除gc索引中的元素。
			err = s.gcIndex.DeleteInBatch(trash, item)
			if err != nil {
				return false, err
			}
//删除访问索引中的关系。
			err = s.accessIndex.DeleteInBatch(trash, item)
			if err != nil {
				return false, err
			}
			garbageCount++
			if garbageCount >= maxTrashSize {
				return true, nil
			}
			return false, nil
		}, nil)
		if err != nil {
			return err
		}
		if garbageCount == 0 {
			return nil
		}
		err = s.db.WriteBatch(trash)
		if err != nil {
			return err
		}
	}
	return nil
}

//getSchema是检索最简单的
//数据库字段中的字符串。
func (s *Store) GetSchema() (name string, err error) {
	name, err = s.schemaName.Get()
	if err == leveldb.ErrNotFound {
		return "", nil
	}
	return name, err
}

//getSchema是存储最简单的
//数据库字段中的字符串。
func (s *Store) PutSchema(name string) (err error) {
	return s.schemaName.Put(name)
}

//关闭关闭基础数据库。
func (s *Store) Close() error {
	return s.db.Close()
}

//示例存储使用shed包构造一个简单的存储实现。
func Example_store() {
	dir, err := ioutil.TempDir("", "ephemeral")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	s, err := New(dir)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	ch := storage.GenerateRandomChunk(1024)
	err = s.Put(context.Background(), ch)
	if err != nil {
		log.Fatal(err)
	}

	got, err := s.Get(context.Background(), ch.Address())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(bytes.Equal(got.Data(), ch.Data()))

//输出：真
}
