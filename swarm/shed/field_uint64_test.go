
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
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
)

//testuint64字段验证Put和Get操作
//在uint64字段中。
func TestUint64Field(t *testing.T) {
	db, cleanupFunc := newTestDB(t)
	defer cleanupFunc()

	counter, err := db.NewUint64Field("counter")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("get empty", func(t *testing.T) {
		got, err := counter.Get()
		if err != nil {
			t.Fatal(err)
		}
		var want uint64
		if got != want {
			t.Errorf("got uint64 %v, want %v", got, want)
		}
	})

	t.Run("put", func(t *testing.T) {
		var want uint64 = 42
		err = counter.Put(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := counter.Get()
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("got uint64 %v, want %v", got, want)
		}

		t.Run("overwrite", func(t *testing.T) {
			var want uint64 = 84
			err = counter.Put(want)
			if err != nil {
				t.Fatal(err)
			}
			got, err := counter.Get()
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Errorf("got uint64 %v, want %v", got, want)
			}
		})
	})

	t.Run("put in batch", func(t *testing.T) {
		batch := new(leveldb.Batch)
		var want uint64 = 42
		counter.PutInBatch(batch, want)
		err = db.WriteBatch(batch)
		if err != nil {
			t.Fatal(err)
		}
		got, err := counter.Get()
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("got uint64 %v, want %v", got, want)
		}

		t.Run("overwrite", func(t *testing.T) {
			batch := new(leveldb.Batch)
			var want uint64 = 84
			counter.PutInBatch(batch, want)
			err = db.WriteBatch(batch)
			if err != nil {
				t.Fatal(err)
			}
			got, err := counter.Get()
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Errorf("got uint64 %v, want %v", got, want)
			}
		})
	})
}

//TESTUINT64字段_inc验证inc操作
//在uint64字段中。
func TestUint64Field_Inc(t *testing.T) {
	db, cleanupFunc := newTestDB(t)
	defer cleanupFunc()

	counter, err := db.NewUint64Field("counter")
	if err != nil {
		t.Fatal(err)
	}

	var want uint64 = 1
	got, err := counter.Inc()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got uint64 %v, want %v", got, want)
	}

	want = 2
	got, err = counter.Inc()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got uint64 %v, want %v", got, want)
	}
}

//testuint64字段“incinbatch”验证incinbatch操作
//在uint64字段中。
func TestUint64Field_IncInBatch(t *testing.T) {
	db, cleanupFunc := newTestDB(t)
	defer cleanupFunc()

	counter, err := db.NewUint64Field("counter")
	if err != nil {
		t.Fatal(err)
	}

	batch := new(leveldb.Batch)
	var want uint64 = 1
	got, err := counter.IncInBatch(batch)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got uint64 %v, want %v", got, want)
	}
	err = db.WriteBatch(batch)
	if err != nil {
		t.Fatal(err)
	}
	got, err = counter.Get()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got uint64 %v, want %v", got, want)
	}

	batch2 := new(leveldb.Batch)
	want = 2
	got, err = counter.IncInBatch(batch2)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got uint64 %v, want %v", got, want)
	}
	err = db.WriteBatch(batch2)
	if err != nil {
		t.Fatal(err)
	}
	got, err = counter.Get()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got uint64 %v, want %v", got, want)
	}
}

//TESTUINT64字段“DEC验证DEC操作”
//在uint64字段中。
func TestUint64Field_Dec(t *testing.T) {
	db, cleanupFunc := newTestDB(t)
	defer cleanupFunc()

	counter, err := db.NewUint64Field("counter")
	if err != nil {
		t.Fatal(err)
	}

//测试溢出保护
	var want uint64
	got, err := counter.Dec()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got uint64 %v, want %v", got, want)
	}

	want = 32
	err = counter.Put(want)
	if err != nil {
		t.Fatal(err)
	}

	want = 31
	got, err = counter.Dec()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got uint64 %v, want %v", got, want)
	}
}

//testuint64字段“decinbatch”验证decinbatch操作
//在uint64字段中。
func TestUint64Field_DecInBatch(t *testing.T) {
	db, cleanupFunc := newTestDB(t)
	defer cleanupFunc()

	counter, err := db.NewUint64Field("counter")
	if err != nil {
		t.Fatal(err)
	}

	batch := new(leveldb.Batch)
	var want uint64
	got, err := counter.DecInBatch(batch)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got uint64 %v, want %v", got, want)
	}
	err = db.WriteBatch(batch)
	if err != nil {
		t.Fatal(err)
	}
	got, err = counter.Get()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got uint64 %v, want %v", got, want)
	}

	batch2 := new(leveldb.Batch)
	want = 42
	counter.PutInBatch(batch2, want)
	err = db.WriteBatch(batch2)
	if err != nil {
		t.Fatal(err)
	}
	got, err = counter.Get()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got uint64 %v, want %v", got, want)
	}

	batch3 := new(leveldb.Batch)
	want = 41
	got, err = counter.DecInBatch(batch3)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got uint64 %v, want %v", got, want)
	}
	err = db.WriteBatch(batch3)
	if err != nil {
		t.Fatal(err)
	}
	got, err = counter.Get()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got uint64 %v, want %v", got, want)
	}
}
