
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Go Ethereum作者
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

package tests

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

//TransactionTest检查事务的rlp解码和发送方派生。
type TransactionTest struct {
	json ttJSON
}

type ttJSON struct {
	BlockNumber math.HexOrDecimal64 `json:"blockNumber"`
	RLP         hexutil.Bytes       `json:"rlp"`
	Sender      hexutil.Bytes       `json:"sender"`
	Transaction *ttTransaction      `json:"transaction"`
}

//go:生成gencodec-type tttransaction-field override tttransactionmarshaling-out gen_tttransaction.go

type ttTransaction struct {
	Data     []byte         `gencodec:"required"`
	GasLimit uint64         `gencodec:"required"`
	GasPrice *big.Int       `gencodec:"required"`
	Nonce    uint64         `gencodec:"required"`
	Value    *big.Int       `gencodec:"required"`
	R        *big.Int       `gencodec:"required"`
	S        *big.Int       `gencodec:"required"`
	V        *big.Int       `gencodec:"required"`
	To       common.Address `gencodec:"required"`
}

type ttTransactionMarshaling struct {
	Data     hexutil.Bytes
	GasLimit math.HexOrDecimal64
	GasPrice *math.HexOrDecimal256
	Nonce    math.HexOrDecimal64
	Value    *math.HexOrDecimal256
	R        *math.HexOrDecimal256
	S        *math.HexOrDecimal256
	V        *math.HexOrDecimal256
}

func (tt *TransactionTest) Run(config *params.ChainConfig) error {
	tx := new(types.Transaction)
	if err := rlp.DecodeBytes(tt.json.RLP, tx); err != nil {
		if tt.json.Transaction == nil {
			return nil
		}
		return fmt.Errorf("RLP decoding failed: %v", err)
	}
//检查发送方派生。
	signer := types.MakeSigner(config, new(big.Int).SetUint64(uint64(tt.json.BlockNumber)))
	sender, err := types.Sender(signer, tx)
	if err != nil {
		return err
	}
	if sender != common.BytesToAddress(tt.json.Sender) {
		return fmt.Errorf("Sender mismatch: got %x, want %x", sender, tt.json.Sender)
	}
//检查解码字段。
	err = tt.json.Transaction.verify(signer, tx)
	if tt.json.Sender == nil && err == nil {
		return errors.New("field validations succeeded but should fail")
	}
	if tt.json.Sender != nil && err != nil {
		return fmt.Errorf("field validations failed after RLP decoding: %s", err)
	}
	return nil
}

func (tt *ttTransaction) verify(signer types.Signer, tx *types.Transaction) error {
	if !bytes.Equal(tx.Data(), tt.Data) {
		return fmt.Errorf("Tx input data mismatch: got %x want %x", tx.Data(), tt.Data)
	}
	if tx.Gas() != tt.GasLimit {
		return fmt.Errorf("GasLimit mismatch: got %d, want %d", tx.Gas(), tt.GasLimit)
	}
	if tx.GasPrice().Cmp(tt.GasPrice) != 0 {
		return fmt.Errorf("GasPrice mismatch: got %v, want %v", tx.GasPrice(), tt.GasPrice)
	}
	if tx.Nonce() != tt.Nonce {
		return fmt.Errorf("Nonce mismatch: got %v, want %v", tx.Nonce(), tt.Nonce)
	}
	v, r, s := tx.RawSignatureValues()
	if r.Cmp(tt.R) != 0 {
		return fmt.Errorf("R mismatch: got %v, want %v", r, tt.R)
	}
	if s.Cmp(tt.S) != 0 {
		return fmt.Errorf("S mismatch: got %v, want %v", s, tt.S)
	}
	if v.Cmp(tt.V) != 0 {
		return fmt.Errorf("V mismatch: got %v, want %v", v, tt.V)
	}
	if tx.To() == nil {
		if tt.To != (common.Address{}) {
			return fmt.Errorf("To mismatch when recipient is nil (contract creation): %x", tt.To)
		}
	} else if *tx.To() != tt.To {
		return fmt.Errorf("To mismatch: got %x, want %x", *tx.To(), tt.To)
	}
	if tx.Value().Cmp(tt.Value) != 0 {
		return fmt.Errorf("Value mismatch: got %x, want %x", tx.Value(), tt.Value)
	}
	return nil
}
