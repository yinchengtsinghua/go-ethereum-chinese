
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
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/syndtr/goleveldb/leveldb"
)

//structField是用于存储复杂结构的帮助程序
//以rlp格式对其进行编码。
type StructField struct {
	db  *DB
	key []byte
}

//NewstructField返回新的structField。
//它根据数据库模式验证其名称和类型。
func (db *DB) NewStructField(name string) (f StructField, err error) {
	key, err := db.schemaFieldKey(name, "struct-rlp")
	if err != nil {
		return f, err
	}
	return StructField{
		db:  db,
		key: key,
	}, nil
}

//将数据库中的数据解包到提供的VAL。
//如果找不到数据，则返回leveldb.errnotfound。
func (f StructField) Get(val interface{}) (err error) {
	b, err := f.db.Get(f.key)
	if err != nil {
		return err
	}
	return rlp.DecodeBytes(b, val)
}

//放入marshals提供的val并将其保存到数据库中。
func (f StructField) Put(val interface{}) (err error) {
	b, err := rlp.EncodeToBytes(val)
	if err != nil {
		return err
	}
	return f.db.Put(f.key, b)
}

//Putinbatch Marshals提供了VAL并将其放入批处理中。
func (f StructField) PutInBatch(batch *leveldb.Batch, val interface{}) (err error) {
	b, err := rlp.EncodeToBytes(val)
	if err != nil {
		return err
	}
	batch.Put(f.key, b)
	return nil
}
