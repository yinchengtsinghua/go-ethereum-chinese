
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
	"testing"
)

//testdb_schemafieldkey验证schemafieldkey的正确性。
func TestDB_schemaFieldKey(t *testing.T) {
	db, cleanupFunc := newTestDB(t)
	defer cleanupFunc()

	t.Run("empty name or type", func(t *testing.T) {
		_, err := db.schemaFieldKey("", "")
		if err == nil {
			t.Errorf("error not returned, but expected")
		}
		_, err = db.schemaFieldKey("", "type")
		if err == nil {
			t.Errorf("error not returned, but expected")
		}

		_, err = db.schemaFieldKey("test", "")
		if err == nil {
			t.Errorf("error not returned, but expected")
		}
	})

	t.Run("same field", func(t *testing.T) {
		key1, err := db.schemaFieldKey("test", "undefined")
		if err != nil {
			t.Fatal(err)
		}

		key2, err := db.schemaFieldKey("test", "undefined")
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(key1, key2) {
			t.Errorf("schema keys for the same field name are not the same: %q, %q", string(key1), string(key2))
		}
	})

	t.Run("different fields", func(t *testing.T) {
		key1, err := db.schemaFieldKey("test1", "undefined")
		if err != nil {
			t.Fatal(err)
		}

		key2, err := db.schemaFieldKey("test2", "undefined")
		if err != nil {
			t.Fatal(err)
		}

		if bytes.Equal(key1, key2) {
			t.Error("schema keys for the same field name are the same, but must not be")
		}
	})

	t.Run("same field name different types", func(t *testing.T) {
		_, err := db.schemaFieldKey("the-field", "one-type")
		if err != nil {
			t.Fatal(err)
		}

		_, err = db.schemaFieldKey("the-field", "another-type")
		if err == nil {
			t.Errorf("error not returned, but expected")
		}
	})
}

//testdb_schemaIndexPrefix验证schemaIndexPrefix的正确性。
func TestDB_schemaIndexPrefix(t *testing.T) {
	db, cleanupFunc := newTestDB(t)
	defer cleanupFunc()

	t.Run("same name", func(t *testing.T) {
		id1, err := db.schemaIndexPrefix("test")
		if err != nil {
			t.Fatal(err)
		}

		id2, err := db.schemaIndexPrefix("test")
		if err != nil {
			t.Fatal(err)
		}

		if id1 != id2 {
			t.Errorf("schema keys for the same field name are not the same: %v, %v", id1, id2)
		}
	})

	t.Run("different names", func(t *testing.T) {
		id1, err := db.schemaIndexPrefix("test1")
		if err != nil {
			t.Fatal(err)
		}

		id2, err := db.schemaIndexPrefix("test2")
		if err != nil {
			t.Fatal(err)
		}

		if id1 == id2 {
			t.Error("schema ids for the same index name are the same, but must not be")
		}
	})
}
