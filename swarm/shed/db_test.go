
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
	"io/ioutil"
	"os"
	"testing"
)

//testnewdb构造新的db
//并验证架构是否正确初始化。
func TestNewDB(t *testing.T) {
	db, cleanupFunc := newTestDB(t)
	defer cleanupFunc()

	s, err := db.getSchema()
	if err != nil {
		t.Fatal(err)
	}
	if s.Fields == nil {
		t.Error("schema fields are empty")
	}
	if len(s.Fields) != 0 {
		t.Errorf("got schema fields length %v, want %v", len(s.Fields), 0)
	}
	if s.Indexes == nil {
		t.Error("schema indexes are empty")
	}
	if len(s.Indexes) != 0 {
		t.Errorf("got schema indexes length %v, want %v", len(s.Indexes), 0)
	}
}

//testdb_persistence创建一个db，保存一个字段并关闭该db。
//然后，它构造另一个数据库并trues来检索保存的值。
func TestDB_persistence(t *testing.T) {
	dir, err := ioutil.TempDir("", "shed-test-persistence")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := NewDB(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	stringField, err := db.NewStringField("preserve-me")
	if err != nil {
		t.Fatal(err)
	}
	want := "persistent value"
	err = stringField.Put(want)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Close()
	if err != nil {
		t.Fatal(err)
	}

	db2, err := NewDB(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	stringField2, err := db2.NewStringField("preserve-me")
	if err != nil {
		t.Fatal(err)
	}
	got, err := stringField2.Get()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got string %q, want %q", got, want)
	}
}

//newtestdb是构造
//临时数据库并返回一个清理函数，该函数必须
//调用以删除数据。
func newTestDB(t *testing.T) (db *DB, cleanupFunc func()) {
	t.Helper()

	dir, err := ioutil.TempDir("", "shed-test")
	if err != nil {
		t.Fatal(err)
	}
	cleanupFunc = func() { os.RemoveAll(dir) }
	db, err = NewDB(dir, "")
	if err != nil {
		cleanupFunc()
		t.Fatal(err)
	}
	return db, cleanupFunc
}
