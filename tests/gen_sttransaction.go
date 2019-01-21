
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
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
)

var _ = (*stTransactionMarshaling)(nil)

func (s stTransaction) MarshalJSON() ([]byte, error) {
	type stTransaction struct {
		GasPrice   *math.HexOrDecimal256 `json:"gasPrice"`
		Nonce      math.HexOrDecimal64   `json:"nonce"`
		To         string                `json:"to"`
		Data       []string              `json:"data"`
		GasLimit   []math.HexOrDecimal64 `json:"gasLimit"`
		Value      []string              `json:"value"`
		PrivateKey hexutil.Bytes         `json:"secretKey"`
	}
	var enc stTransaction
	enc.GasPrice = (*math.HexOrDecimal256)(s.GasPrice)
	enc.Nonce = math.HexOrDecimal64(s.Nonce)
	enc.To = s.To
	enc.Data = s.Data
	if s.GasLimit != nil {
		enc.GasLimit = make([]math.HexOrDecimal64, len(s.GasLimit))
		for k, v := range s.GasLimit {
			enc.GasLimit[k] = math.HexOrDecimal64(v)
		}
	}
	enc.Value = s.Value
	enc.PrivateKey = s.PrivateKey
	return json.Marshal(&enc)
}

func (s *stTransaction) UnmarshalJSON(input []byte) error {
	type stTransaction struct {
		GasPrice   *math.HexOrDecimal256 `json:"gasPrice"`
		Nonce      *math.HexOrDecimal64  `json:"nonce"`
		To         *string               `json:"to"`
		Data       []string              `json:"data"`
		GasLimit   []math.HexOrDecimal64 `json:"gasLimit"`
		Value      []string              `json:"value"`
		PrivateKey *hexutil.Bytes        `json:"secretKey"`
	}
	var dec stTransaction
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.GasPrice != nil {
		s.GasPrice = (*big.Int)(dec.GasPrice)
	}
	if dec.Nonce != nil {
		s.Nonce = uint64(*dec.Nonce)
	}
	if dec.To != nil {
		s.To = *dec.To
	}
	if dec.Data != nil {
		s.Data = dec.Data
	}
	if dec.GasLimit != nil {
		s.GasLimit = make([]uint64, len(dec.GasLimit))
		for k, v := range dec.GasLimit {
			s.GasLimit[k] = uint64(v)
		}
	}
	if dec.Value != nil {
		s.Value = dec.Value
	}
	if dec.PrivateKey != nil {
		s.PrivateKey = *dec.PrivateKey
	}
	return nil
}
