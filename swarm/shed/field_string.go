
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
	"github.com/syndtr/goleveldb/leveldb"
)

//StringField是最简单的字段实现
//它在特定的leveldb键下存储一个任意字符串。
type StringField struct {
	db  *DB
	key []byte
}

//newStringField重新运行StringField的新实例。
//它根据数据库模式验证其名称和类型。
func (db *DB) NewStringField(name string) (f StringField, err error) {
	key, err := db.schemaFieldKey(name, "string")
	if err != nil {
		return f, err
	}
	return StringField{
		db:  db,
		key: key,
	}, nil
}

//get返回数据库中的字符串值。
//如果找不到该值，则返回空字符串
//没有错误。
func (f StringField) Get() (val string, err error) {
	b, err := f.db.Get(f.key)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

//将存储字符串放入数据库。
func (f StringField) Put(val string) (err error) {
	return f.db.Put(f.key, []byte(val))
}

//putinbatch将字符串存储在可以
//稍后保存在数据库中。
func (f StringField) PutInBatch(batch *leveldb.Batch, val string) {
	batch.Put(f.key, []byte(val))
}
