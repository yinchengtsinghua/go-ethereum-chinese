
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

//+构建JS

package ethdb

import (
	"errors"
)

var errNotSupported = errors.New("ethdb: not supported")

type LDBDatabase struct {
}

//NewLdbDatabase返回一个LevelDB包装的对象。
func NewLDBDatabase(file string, cache int, handles int) (*LDBDatabase, error) {
	return nil, errNotSupported
}

//path返回数据库目录的路径。
func (db *LDBDatabase) Path() string {
	return ""
}

//Put将给定的键/值放入队列
func (db *LDBDatabase) Put(key []byte, value []byte) error {
	return errNotSupported
}

func (db *LDBDatabase) Has(key []byte) (bool, error) {
	return false, errNotSupported
}

//get返回给定的键（如果存在）。
func (db *LDBDatabase) Get(key []byte) ([]byte, error) {
	return nil, errNotSupported
}

//删除从队列和数据库中删除键
func (db *LDBDatabase) Delete(key []byte) error {
	return errNotSupported
}

func (db *LDBDatabase) Close() {
}

//Meter配置数据库度量收集器和
func (db *LDBDatabase) Meter(prefix string) {
}

func (db *LDBDatabase) NewBatch() Batch {
	return nil
}
