
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2014 Go Ethereum作者
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

package ethdb

//使用批处理的代码应该尝试向批处理中添加这么多的数据。
//该值是根据经验确定的。
const IdealBatchSize = 100 * 1024

//推杆包装批处理和常规数据库都支持的数据库写入操作。
type Putter interface {
	Put(key []byte, value []byte) error
}

//删除程序包装批处理数据库和常规数据库都支持的数据库删除操作。
type Deleter interface {
	Delete(key []byte) error
}

//数据库包装所有数据库操作。所有方法对于并发使用都是安全的。
type Database interface {
	Putter
	Deleter
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)
	Close()
	NewBatch() Batch
}

//批处理是一个只写的数据库，它将更改提交到其主机数据库。
//当调用写入时。批处理不能同时使用。
type Batch interface {
	Putter
	Deleter
ValueSize() int //批中的数据量
	Write() error
//重置将批重置为可重用
	Reset()
}
