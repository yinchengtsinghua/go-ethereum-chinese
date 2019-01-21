
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

package types

import (
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
)

//go:生成gencodec-type log-field override logmarshaling-out gen_log_json.go

//日志表示合同日志事件。这些事件由日志操作码生成，并且
//由节点存储/索引。
type Log struct {
//共识领域：
//产生事件的合同的地址
	Address common.Address `json:"address" gencodec:"required"`
//合同提供的主题列表。
	Topics []common.Hash `json:"topics" gencodec:"required"`
//由合同提供，通常为ABI编码
	Data []byte `json:"data" gencodec:"required"`

//派生字段。这些字段由节点填充
//但没有达成共识。
//包含事务的块
	BlockNumber uint64 `json:"blockNumber"`
//事务的哈希
	TxHash common.Hash `json:"transactionHash" gencodec:"required"`
//块中事务的索引
	TxIndex uint `json:"transactionIndex" gencodec:"required"`
//包含事务的块的哈希
	BlockHash common.Hash `json:"blockHash"`
//块中日志的索引
	Index uint `json:"logIndex" gencodec:"required"`

//如果由于链重组而还原此日志，则删除的字段为真。
//如果通过筛选查询接收日志，则必须注意此字段。
	Removed bool `json:"removed"`
}

type logMarshaling struct {
	Data        hexutil.Bytes
	BlockNumber hexutil.Uint64
	TxIndex     hexutil.Uint
	Index       hexutil.Uint
}

type rlpLog struct {
	Address common.Address
	Topics  []common.Hash
	Data    []byte
}

type rlpStorageLog struct {
	Address     common.Address
	Topics      []common.Hash
	Data        []byte
	BlockNumber uint64
	TxHash      common.Hash
	TxIndex     uint
	BlockHash   common.Hash
	Index       uint
}

//encoderlp实现rlp.encoder。
func (l *Log) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, rlpLog{Address: l.Address, Topics: l.Topics, Data: l.Data})
}

//decoderlp实现rlp.decoder。
func (l *Log) DecodeRLP(s *rlp.Stream) error {
	var dec rlpLog
	err := s.Decode(&dec)
	if err == nil {
		l.Address, l.Topics, l.Data = dec.Address, dec.Topics, dec.Data
	}
	return err
}

//logForStorage是一个围绕一个日志的包装器，它扁平化并解析
//包含非共识字段的日志。
type LogForStorage Log

//encoderlp实现rlp.encoder。
func (l *LogForStorage) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, rlpStorageLog{
		Address:     l.Address,
		Topics:      l.Topics,
		Data:        l.Data,
		BlockNumber: l.BlockNumber,
		TxHash:      l.TxHash,
		TxIndex:     l.TxIndex,
		BlockHash:   l.BlockHash,
		Index:       l.Index,
	})
}

//decoderlp实现rlp.decoder。
func (l *LogForStorage) DecodeRLP(s *rlp.Stream) error {
	var dec rlpStorageLog
	err := s.Decode(&dec)
	if err == nil {
		*l = LogForStorage{
			Address:     dec.Address,
			Topics:      dec.Topics,
			Data:        dec.Data,
			BlockNumber: dec.BlockNumber,
			TxHash:      dec.TxHash,
			TxIndex:     dec.TxIndex,
			BlockHash:   dec.BlockHash,
			Index:       dec.Index,
		}
	}
	return err
}
