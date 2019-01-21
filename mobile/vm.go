
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

//包含core/types包中的所有包装器。

package geth

import (
	"errors"

	"github.com/ethereum/go-ethereum/core/types"
)

//日志表示合同日志事件。这些事件由日志生成
//操作码并由节点存储/索引。
type Log struct {
	log *types.Log
}

func (l *Log) GetAddress() *Address  { return &Address{l.log.Address} }
func (l *Log) GetTopics() *Hashes    { return &Hashes{l.log.Topics} }
func (l *Log) GetData() []byte       { return l.log.Data }
func (l *Log) GetBlockNumber() int64 { return int64(l.log.BlockNumber) }
func (l *Log) GetTxHash() *Hash      { return &Hash{l.log.TxHash} }
func (l *Log) GetTxIndex() int       { return int(l.log.TxIndex) }
func (l *Log) GetBlockHash() *Hash   { return &Hash{l.log.BlockHash} }
func (l *Log) GetIndex() int         { return int(l.log.Index) }

//日志表示VM日志的一部分。
type Logs struct{ logs []*types.Log }

//SIZE返回切片中的日志数。
func (l *Logs) Size() int {
	return len(l.logs)
}

//get返回切片中给定索引处的日志。
func (l *Logs) Get(index int) (log *Log, _ error) {
	if index < 0 || index >= len(l.logs) {
		return nil, errors.New("index out of bounds")
	}
	return &Log{l.logs[index]}, nil
}
