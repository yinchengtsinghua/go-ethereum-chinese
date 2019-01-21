
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
	"encoding/binary"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
)

//用于此文件中测试的索引函数。
var retrievalIndexFuncs = IndexFuncs{
	EncodeKey: func(fields Item) (key []byte, err error) {
		return fields.Address, nil
	},
	DecodeKey: func(key []byte) (e Item, err error) {
		e.Address = key
		return e, nil
	},
	EncodeValue: func(fields Item) (value []byte, err error) {
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(fields.StoreTimestamp))
		value = append(b, fields.Data...)
		return value, nil
	},
	DecodeValue: func(keyItem Item, value []byte) (e Item, err error) {
		e.StoreTimestamp = int64(binary.BigEndian.Uint64(value[:8]))
		e.Data = value[8:]
		return e, nil
	},
}

//testindex验证索引实现的put、get和delete函数。
func TestIndex(t *testing.T) {
	db, cleanupFunc := newTestDB(t)
	defer cleanupFunc()

	index, err := db.NewIndex("retrieval", retrievalIndexFuncs)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("put", func(t *testing.T) {
		want := Item{
			Address:        []byte("put-hash"),
			Data:           []byte("DATA"),
			StoreTimestamp: time.Now().UTC().UnixNano(),
		}

		err := index.Put(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := index.Get(Item{
			Address: want.Address,
		})
		if err != nil {
			t.Fatal(err)
		}
		checkItem(t, got, want)

		t.Run("overwrite", func(t *testing.T) {
			want := Item{
				Address:        []byte("put-hash"),
				Data:           []byte("New DATA"),
				StoreTimestamp: time.Now().UTC().UnixNano(),
			}

			err = index.Put(want)
			if err != nil {
				t.Fatal(err)
			}
			got, err := index.Get(Item{
				Address: want.Address,
			})
			if err != nil {
				t.Fatal(err)
			}
			checkItem(t, got, want)
		})
	})

	t.Run("put in batch", func(t *testing.T) {
		want := Item{
			Address:        []byte("put-in-batch-hash"),
			Data:           []byte("DATA"),
			StoreTimestamp: time.Now().UTC().UnixNano(),
		}

		batch := new(leveldb.Batch)
		index.PutInBatch(batch, want)
		err := db.WriteBatch(batch)
		if err != nil {
			t.Fatal(err)
		}
		got, err := index.Get(Item{
			Address: want.Address,
		})
		if err != nil {
			t.Fatal(err)
		}
		checkItem(t, got, want)

		t.Run("overwrite", func(t *testing.T) {
			want := Item{
				Address:        []byte("put-in-batch-hash"),
				Data:           []byte("New DATA"),
				StoreTimestamp: time.Now().UTC().UnixNano(),
			}

			batch := new(leveldb.Batch)
			index.PutInBatch(batch, want)
			db.WriteBatch(batch)
			if err != nil {
				t.Fatal(err)
			}
			got, err := index.Get(Item{
				Address: want.Address,
			})
			if err != nil {
				t.Fatal(err)
			}
			checkItem(t, got, want)
		})
	})

	t.Run("put in batch twice", func(t *testing.T) {
//确保具有相同db键的最后一项
//实际已保存
		batch := new(leveldb.Batch)
		address := []byte("put-in-batch-twice-hash")

//放第一个项目
		index.PutInBatch(batch, Item{
			Address:        address,
			Data:           []byte("DATA"),
			StoreTimestamp: time.Now().UTC().UnixNano(),
		})

		want := Item{
			Address:        address,
			Data:           []byte("New DATA"),
			StoreTimestamp: time.Now().UTC().UnixNano(),
		}
//然后放入将产生相同密钥的项
//但数据库中的值不同
		index.PutInBatch(batch, want)
		db.WriteBatch(batch)
		if err != nil {
			t.Fatal(err)
		}
		got, err := index.Get(Item{
			Address: address,
		})
		if err != nil {
			t.Fatal(err)
		}
		checkItem(t, got, want)
	})

	t.Run("delete", func(t *testing.T) {
		want := Item{
			Address:        []byte("delete-hash"),
			Data:           []byte("DATA"),
			StoreTimestamp: time.Now().UTC().UnixNano(),
		}

		err := index.Put(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := index.Get(Item{
			Address: want.Address,
		})
		if err != nil {
			t.Fatal(err)
		}
		checkItem(t, got, want)

		err = index.Delete(Item{
			Address: want.Address,
		})
		if err != nil {
			t.Fatal(err)
		}

		wantErr := leveldb.ErrNotFound
		got, err = index.Get(Item{
			Address: want.Address,
		})
		if err != wantErr {
			t.Fatalf("got error %v, want %v", err, wantErr)
		}
	})

	t.Run("delete in batch", func(t *testing.T) {
		want := Item{
			Address:        []byte("delete-in-batch-hash"),
			Data:           []byte("DATA"),
			StoreTimestamp: time.Now().UTC().UnixNano(),
		}

		err := index.Put(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := index.Get(Item{
			Address: want.Address,
		})
		if err != nil {
			t.Fatal(err)
		}
		checkItem(t, got, want)

		batch := new(leveldb.Batch)
		index.DeleteInBatch(batch, Item{
			Address: want.Address,
		})
		err = db.WriteBatch(batch)
		if err != nil {
			t.Fatal(err)
		}

		wantErr := leveldb.ErrNotFound
		got, err = index.Get(Item{
			Address: want.Address,
		})
		if err != wantErr {
			t.Fatalf("got error %v, want %v", err, wantErr)
		}
	})
}

//TestIndex迭代验证索引迭代
//正确性功能。
func TestIndex_Iterate(t *testing.T) {
	db, cleanupFunc := newTestDB(t)
	defer cleanupFunc()

	index, err := db.NewIndex("retrieval", retrievalIndexFuncs)
	if err != nil {
		t.Fatal(err)
	}

	items := []Item{
		{
			Address: []byte("iterate-hash-01"),
			Data:    []byte("data80"),
		},
		{
			Address: []byte("iterate-hash-03"),
			Data:    []byte("data22"),
		},
		{
			Address: []byte("iterate-hash-05"),
			Data:    []byte("data41"),
		},
		{
			Address: []byte("iterate-hash-02"),
			Data:    []byte("data84"),
		},
		{
			Address: []byte("iterate-hash-06"),
			Data:    []byte("data1"),
		},
	}
	batch := new(leveldb.Batch)
	for _, i := range items {
		index.PutInBatch(batch, i)
	}
	err = db.WriteBatch(batch)
	if err != nil {
		t.Fatal(err)
	}
	item04 := Item{
		Address: []byte("iterate-hash-04"),
		Data:    []byte("data0"),
	}
	err = index.Put(item04)
	if err != nil {
		t.Fatal(err)
	}
	items = append(items, item04)

	sort.SliceStable(items, func(i, j int) bool {
		return bytes.Compare(items[i].Address, items[j].Address) < 0
	})

	t.Run("all", func(t *testing.T) {
		var i int
		err := index.Iterate(func(item Item) (stop bool, err error) {
			if i > len(items)-1 {
				return true, fmt.Errorf("got unexpected index item: %#v", item)
			}
			want := items[i]
			checkItem(t, item, want)
			i++
			return false, nil
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("start from", func(t *testing.T) {
		startIndex := 2
		i := startIndex
		err := index.Iterate(func(item Item) (stop bool, err error) {
			if i > len(items)-1 {
				return true, fmt.Errorf("got unexpected index item: %#v", item)
			}
			want := items[i]
			checkItem(t, item, want)
			i++
			return false, nil
		}, &IterateOptions{
			StartFrom: &items[startIndex],
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("skip start from", func(t *testing.T) {
		startIndex := 2
		i := startIndex + 1
		err := index.Iterate(func(item Item) (stop bool, err error) {
			if i > len(items)-1 {
				return true, fmt.Errorf("got unexpected index item: %#v", item)
			}
			want := items[i]
			checkItem(t, item, want)
			i++
			return false, nil
		}, &IterateOptions{
			StartFrom:         &items[startIndex],
			SkipStartFromItem: true,
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("stop", func(t *testing.T) {
		var i int
		stopIndex := 3
		var count int
		err := index.Iterate(func(item Item) (stop bool, err error) {
			if i > len(items)-1 {
				return true, fmt.Errorf("got unexpected index item: %#v", item)
			}
			want := items[i]
			checkItem(t, item, want)
			count++
			if i == stopIndex {
				return true, nil
			}
			i++
			return false, nil
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		wantItemsCount := stopIndex + 1
		if count != wantItemsCount {
			t.Errorf("got %v items, expected %v", count, wantItemsCount)
		}
	})

	t.Run("no overflow", func(t *testing.T) {
		secondIndex, err := db.NewIndex("second-index", retrievalIndexFuncs)
		if err != nil {
			t.Fatal(err)
		}

		secondItem := Item{
			Address: []byte("iterate-hash-10"),
			Data:    []byte("data-second"),
		}
		err = secondIndex.Put(secondItem)
		if err != nil {
			t.Fatal(err)
		}

		var i int
		err = index.Iterate(func(item Item) (stop bool, err error) {
			if i > len(items)-1 {
				return true, fmt.Errorf("got unexpected index item: %#v", item)
			}
			want := items[i]
			checkItem(t, item, want)
			i++
			return false, nil
		}, nil)
		if err != nil {
			t.Fatal(err)
		}

		i = 0
		err = secondIndex.Iterate(func(item Item) (stop bool, err error) {
			if i > 1 {
				return true, fmt.Errorf("got unexpected index item: %#v", item)
			}
			checkItem(t, item, secondItem)
			i++
			return false, nil
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
	})
}

//testindex_iterate_withprefix验证索引迭代
//功能正确。
func TestIndex_Iterate_withPrefix(t *testing.T) {
	db, cleanupFunc := newTestDB(t)
	defer cleanupFunc()

	index, err := db.NewIndex("retrieval", retrievalIndexFuncs)
	if err != nil {
		t.Fatal(err)
	}

	allItems := []Item{
		{Address: []byte("want-hash-00"), Data: []byte("data80")},
		{Address: []byte("skip-hash-01"), Data: []byte("data81")},
		{Address: []byte("skip-hash-02"), Data: []byte("data82")},
		{Address: []byte("skip-hash-03"), Data: []byte("data83")},
		{Address: []byte("want-hash-04"), Data: []byte("data84")},
		{Address: []byte("want-hash-05"), Data: []byte("data85")},
		{Address: []byte("want-hash-06"), Data: []byte("data86")},
		{Address: []byte("want-hash-07"), Data: []byte("data87")},
		{Address: []byte("want-hash-08"), Data: []byte("data88")},
		{Address: []byte("want-hash-09"), Data: []byte("data89")},
		{Address: []byte("skip-hash-10"), Data: []byte("data90")},
	}
	batch := new(leveldb.Batch)
	for _, i := range allItems {
		index.PutInBatch(batch, i)
	}
	err = db.WriteBatch(batch)
	if err != nil {
		t.Fatal(err)
	}

	prefix := []byte("want")

	items := make([]Item, 0)
	for _, item := range allItems {
		if bytes.HasPrefix(item.Address, prefix) {
			items = append(items, item)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return bytes.Compare(items[i].Address, items[j].Address) < 0
	})

	t.Run("with prefix", func(t *testing.T) {
		var i int
		err := index.Iterate(func(item Item) (stop bool, err error) {
			if i > len(items)-1 {
				return true, fmt.Errorf("got unexpected index item: %#v", item)
			}
			want := items[i]
			checkItem(t, item, want)
			i++
			return false, nil
		}, &IterateOptions{
			Prefix: prefix,
		})
		if err != nil {
			t.Fatal(err)
		}
		if i != len(items) {
			t.Errorf("got %v items, want %v", i, len(items))
		}
	})

	t.Run("with prefix and start from", func(t *testing.T) {
		startIndex := 2
		var count int
		i := startIndex
		err := index.Iterate(func(item Item) (stop bool, err error) {
			if i > len(items)-1 {
				return true, fmt.Errorf("got unexpected index item: %#v", item)
			}
			want := items[i]
			checkItem(t, item, want)
			i++
			count++
			return false, nil
		}, &IterateOptions{
			StartFrom: &items[startIndex],
			Prefix:    prefix,
		})
		if err != nil {
			t.Fatal(err)
		}
		wantCount := len(items) - startIndex
		if count != wantCount {
			t.Errorf("got %v items, want %v", count, wantCount)
		}
	})

	t.Run("with prefix and skip start from", func(t *testing.T) {
		startIndex := 2
		var count int
		i := startIndex + 1
		err := index.Iterate(func(item Item) (stop bool, err error) {
			if i > len(items)-1 {
				return true, fmt.Errorf("got unexpected index item: %#v", item)
			}
			want := items[i]
			checkItem(t, item, want)
			i++
			count++
			return false, nil
		}, &IterateOptions{
			StartFrom:         &items[startIndex],
			SkipStartFromItem: true,
			Prefix:            prefix,
		})
		if err != nil {
			t.Fatal(err)
		}
		wantCount := len(items) - startIndex - 1
		if count != wantCount {
			t.Errorf("got %v items, want %v", count, wantCount)
		}
	})

	t.Run("stop", func(t *testing.T) {
		var i int
		stopIndex := 3
		var count int
		err := index.Iterate(func(item Item) (stop bool, err error) {
			if i > len(items)-1 {
				return true, fmt.Errorf("got unexpected index item: %#v", item)
			}
			want := items[i]
			checkItem(t, item, want)
			count++
			if i == stopIndex {
				return true, nil
			}
			i++
			return false, nil
		}, &IterateOptions{
			Prefix: prefix,
		})
		if err != nil {
			t.Fatal(err)
		}
		wantItemsCount := stopIndex + 1
		if count != wantItemsCount {
			t.Errorf("got %v items, expected %v", count, wantItemsCount)
		}
	})

	t.Run("no overflow", func(t *testing.T) {
		secondIndex, err := db.NewIndex("second-index", retrievalIndexFuncs)
		if err != nil {
			t.Fatal(err)
		}

		secondItem := Item{
			Address: []byte("iterate-hash-10"),
			Data:    []byte("data-second"),
		}
		err = secondIndex.Put(secondItem)
		if err != nil {
			t.Fatal(err)
		}

		var i int
		err = index.Iterate(func(item Item) (stop bool, err error) {
			if i > len(items)-1 {
				return true, fmt.Errorf("got unexpected index item: %#v", item)
			}
			want := items[i]
			checkItem(t, item, want)
			i++
			return false, nil
		}, &IterateOptions{
			Prefix: prefix,
		})
		if err != nil {
			t.Fatal(err)
		}
		if i != len(items) {
			t.Errorf("got %v items, want %v", i, len(items))
		}
	})
}

//如果index.count和index.countFrom
//返回正确的项目数。
func TestIndex_count(t *testing.T) {
	db, cleanupFunc := newTestDB(t)
	defer cleanupFunc()

	index, err := db.NewIndex("retrieval", retrievalIndexFuncs)
	if err != nil {
		t.Fatal(err)
	}

	items := []Item{
		{
			Address: []byte("iterate-hash-01"),
			Data:    []byte("data80"),
		},
		{
			Address: []byte("iterate-hash-02"),
			Data:    []byte("data84"),
		},
		{
			Address: []byte("iterate-hash-03"),
			Data:    []byte("data22"),
		},
		{
			Address: []byte("iterate-hash-04"),
			Data:    []byte("data41"),
		},
		{
			Address: []byte("iterate-hash-05"),
			Data:    []byte("data1"),
		},
	}
	batch := new(leveldb.Batch)
	for _, i := range items {
		index.PutInBatch(batch, i)
	}
	err = db.WriteBatch(batch)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Count", func(t *testing.T) {
		got, err := index.Count()
		if err != nil {
			t.Fatal(err)
		}

		want := len(items)
		if got != want {
			t.Errorf("got %v items count, want %v", got, want)
		}
	})

	t.Run("CountFrom", func(t *testing.T) {
		got, err := index.CountFrom(Item{
			Address: items[1].Address,
		})
		if err != nil {
			t.Fatal(err)
		}

		want := len(items) - 1
		if got != want {
			t.Errorf("got %v items count, want %v", got, want)
		}
	})

//用其他项更新索引
	t.Run("add item", func(t *testing.T) {
		item04 := Item{
			Address: []byte("iterate-hash-06"),
			Data:    []byte("data0"),
		}
		err = index.Put(item04)
		if err != nil {
			t.Fatal(err)
		}

		count := len(items) + 1

		t.Run("Count", func(t *testing.T) {
			got, err := index.Count()
			if err != nil {
				t.Fatal(err)
			}

			want := count
			if got != want {
				t.Errorf("got %v items count, want %v", got, want)
			}
		})

		t.Run("CountFrom", func(t *testing.T) {
			got, err := index.CountFrom(Item{
				Address: items[1].Address,
			})
			if err != nil {
				t.Fatal(err)
			}

			want := count - 1
			if got != want {
				t.Errorf("got %v items count, want %v", got, want)
			}
		})
	})

//删除一些项目
	t.Run("delete items", func(t *testing.T) {
		deleteCount := 3

		for _, item := range items[:deleteCount] {
			err := index.Delete(item)
			if err != nil {
				t.Fatal(err)
			}
		}

		count := len(items) + 1 - deleteCount

		t.Run("Count", func(t *testing.T) {
			got, err := index.Count()
			if err != nil {
				t.Fatal(err)
			}

			want := count
			if got != want {
				t.Errorf("got %v items count, want %v", got, want)
			}
		})

		t.Run("CountFrom", func(t *testing.T) {
			got, err := index.CountFrom(Item{
				Address: items[deleteCount+1].Address,
			})
			if err != nil {
				t.Fatal(err)
			}

			want := count - 1
			if got != want {
				t.Errorf("got %v items count, want %v", got, want)
			}
		})
	})
}

//checkitem是一个测试助手函数，用于比较两个索引项是否相同。
func checkItem(t *testing.T, got, want Item) {
	t.Helper()

	if !bytes.Equal(got.Address, want.Address) {
		t.Errorf("got hash %q, expected %q", string(got.Address), string(want.Address))
	}
	if !bytes.Equal(got.Data, want.Data) {
		t.Errorf("got data %q, expected %q", string(got.Data), string(want.Data))
	}
	if got.StoreTimestamp != want.StoreTimestamp {
		t.Errorf("got store timestamp %v, expected %v", got.StoreTimestamp, want.StoreTimestamp)
	}
	if got.AccessTimestamp != want.AccessTimestamp {
		t.Errorf("got access timestamp %v, expected %v", got.AccessTimestamp, want.AccessTimestamp)
	}
}
