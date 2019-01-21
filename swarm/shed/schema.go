
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
	"encoding/json"
	"errors"
	"fmt"
)

var (
//用于存储架构的LevelDB键值。
	keySchema = []byte{0}
//所有字段类型的leveldb键前缀。
//将通过将名称值附加到此前缀来构造LevelDB键。
	keyPrefixFields byte = 1
//索引键起始的级别数据库键前缀。
//每个索引都有自己的键前缀，这个值定义了第一个。
keyPrefixIndexStart byte = 2 //问：或者可能是更高的数字，比如7，为潜在的特定性能提供更多的空间。
)

//架构用于序列化已知的数据库结构信息。
type schema struct {
Fields  map[string]fieldSpec `json:"fields"`  //键是字段名
Indexes map[byte]indexSpec   `json:"indexes"` //键是索引前缀字节
}

//fieldspec保存有关特定字段的信息。
//它不需要名称字段，因为它包含在
//架构。字段映射键。
type fieldSpec struct {
	Type string `json:"type"`
}

//indxspec保存有关特定索引的信息。
//它不包含索引类型，因为索引没有类型。
type indexSpec struct {
	Name string `json:"name"`
}

//SchemaFieldKey检索的完整级别数据库键
//一个特定的字段构成了模式定义。
func (db *DB) schemaFieldKey(name, fieldType string) (key []byte, err error) {
	if name == "" {
		return nil, errors.New("field name can not be blank")
	}
	if fieldType == "" {
		return nil, errors.New("field type can not be blank")
	}
	s, err := db.getSchema()
	if err != nil {
		return nil, err
	}
	var found bool
	for n, f := range s.Fields {
		if n == name {
			if f.Type != fieldType {
				return nil, fmt.Errorf("field %q of type %q stored as %q in db", name, fieldType, f.Type)
			}
			break
		}
	}
	if !found {
		s.Fields[name] = fieldSpec{
			Type: fieldType,
		}
		err := db.putSchema(s)
		if err != nil {
			return nil, err
		}
	}
	return append([]byte{keyPrefixFields}, []byte(name)...), nil
}

//SchemaIndexID检索的完整级别数据库前缀
//一种特殊的索引。
func (db *DB) schemaIndexPrefix(name string) (id byte, err error) {
	if name == "" {
		return 0, errors.New("index name can not be blank")
	}
	s, err := db.getSchema()
	if err != nil {
		return 0, err
	}
	nextID := keyPrefixIndexStart
	for i, f := range s.Indexes {
		if i >= nextID {
			nextID = i + 1
		}
		if f.Name == name {
			return i, nil
		}
	}
	id = nextID
	s.Indexes[id] = indexSpec{
		Name: name,
	}
	return id, db.putSchema(s)
}

//GetSchema从中检索完整的架构
//数据库。
func (db *DB) getSchema() (s schema, err error) {
	b, err := db.Get(keySchema)
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(b, &s)
	return s, err
}

//PutSchema将完整的架构存储到
//数据库。
func (db *DB) putSchema(s schema) (err error) {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return db.Put(keySchema, b)
}
