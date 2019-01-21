
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//代码由github.com/fjl/gencodec生成。不要编辑。

package tests

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
)

var _ = (*ttTransactionMarshaling)(nil)

func (t ttTransaction) MarshalJSON() ([]byte, error) {
	type ttTransaction struct {
		Data     hexutil.Bytes         `gencodec:"required"`
		GasLimit math.HexOrDecimal64   `gencodec:"required"`
		GasPrice *math.HexOrDecimal256 `gencodec:"required"`
		Nonce    math.HexOrDecimal64   `gencodec:"required"`
		Value    *math.HexOrDecimal256 `gencodec:"required"`
		R        *math.HexOrDecimal256 `gencodec:"required"`
		S        *math.HexOrDecimal256 `gencodec:"required"`
		V        *math.HexOrDecimal256 `gencodec:"required"`
		To       common.Address        `gencodec:"required"`
	}
	var enc ttTransaction
	enc.Data = t.Data
	enc.GasLimit = math.HexOrDecimal64(t.GasLimit)
	enc.GasPrice = (*math.HexOrDecimal256)(t.GasPrice)
	enc.Nonce = math.HexOrDecimal64(t.Nonce)
	enc.Value = (*math.HexOrDecimal256)(t.Value)
	enc.R = (*math.HexOrDecimal256)(t.R)
	enc.S = (*math.HexOrDecimal256)(t.S)
	enc.V = (*math.HexOrDecimal256)(t.V)
	enc.To = t.To
	return json.Marshal(&enc)
}

func (t *ttTransaction) UnmarshalJSON(input []byte) error {
	type ttTransaction struct {
		Data     *hexutil.Bytes        `gencodec:"required"`
		GasLimit *math.HexOrDecimal64  `gencodec:"required"`
		GasPrice *math.HexOrDecimal256 `gencodec:"required"`
		Nonce    *math.HexOrDecimal64  `gencodec:"required"`
		Value    *math.HexOrDecimal256 `gencodec:"required"`
		R        *math.HexOrDecimal256 `gencodec:"required"`
		S        *math.HexOrDecimal256 `gencodec:"required"`
		V        *math.HexOrDecimal256 `gencodec:"required"`
		To       *common.Address       `gencodec:"required"`
	}
	var dec ttTransaction
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Data == nil {
		return errors.New("missing required field 'data' for ttTransaction")
	}
	t.Data = *dec.Data
	if dec.GasLimit == nil {
		return errors.New("missing required field 'gasLimit' for ttTransaction")
	}
	t.GasLimit = uint64(*dec.GasLimit)
	if dec.GasPrice == nil {
		return errors.New("missing required field 'gasPrice' for ttTransaction")
	}
	t.GasPrice = (*big.Int)(dec.GasPrice)
	if dec.Nonce == nil {
		return errors.New("missing required field 'nonce' for ttTransaction")
	}
	t.Nonce = uint64(*dec.Nonce)
	if dec.Value == nil {
		return errors.New("missing required field 'value' for ttTransaction")
	}
	t.Value = (*big.Int)(dec.Value)
	if dec.R == nil {
		return errors.New("missing required field 'r' for ttTransaction")
	}
	t.R = (*big.Int)(dec.R)
	if dec.S == nil {
		return errors.New("missing required field 's' for ttTransaction")
	}
	t.S = (*big.Int)(dec.S)
	if dec.V == nil {
		return errors.New("missing required field 'v' for ttTransaction")
	}
	t.V = (*big.Int)(dec.V)
	if dec.To == nil {
		return errors.New("missing required field 'to' for ttTransaction")
	}
	t.To = *dec.To
	return nil
}
