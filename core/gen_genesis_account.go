
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//代码由github.com/fjl/gencodec生成。不要编辑。

package core

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
)

var _ = (*genesisAccountMarshaling)(nil)

func (g GenesisAccount) MarshalJSON() ([]byte, error) {
	type GenesisAccount struct {
		Code       hexutil.Bytes               `json:"code,omitempty"`
		Storage    map[storageJSON]storageJSON `json:"storage,omitempty"`
		Balance    *math.HexOrDecimal256       `json:"balance" gencodec:"required"`
		Nonce      math.HexOrDecimal64         `json:"nonce,omitempty"`
		PrivateKey hexutil.Bytes               `json:"secretKey,omitempty"`
	}
	var enc GenesisAccount
	enc.Code = g.Code
	if g.Storage != nil {
		enc.Storage = make(map[storageJSON]storageJSON, len(g.Storage))
		for k, v := range g.Storage {
			enc.Storage[storageJSON(k)] = storageJSON(v)
		}
	}
	enc.Balance = (*math.HexOrDecimal256)(g.Balance)
	enc.Nonce = math.HexOrDecimal64(g.Nonce)
	enc.PrivateKey = g.PrivateKey
	return json.Marshal(&enc)
}

func (g *GenesisAccount) UnmarshalJSON(input []byte) error {
	type GenesisAccount struct {
		Code       *hexutil.Bytes              `json:"code,omitempty"`
		Storage    map[storageJSON]storageJSON `json:"storage,omitempty"`
		Balance    *math.HexOrDecimal256       `json:"balance" gencodec:"required"`
		Nonce      *math.HexOrDecimal64        `json:"nonce,omitempty"`
		PrivateKey *hexutil.Bytes              `json:"secretKey,omitempty"`
	}
	var dec GenesisAccount
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Code != nil {
		g.Code = *dec.Code
	}
	if dec.Storage != nil {
		g.Storage = make(map[common.Hash]common.Hash, len(dec.Storage))
		for k, v := range dec.Storage {
			g.Storage[common.Hash(k)] = common.Hash(v)
		}
	}
	if dec.Balance == nil {
		return errors.New("missing required field 'balance' for GenesisAccount")
	}
	g.Balance = (*big.Int)(dec.Balance)
	if dec.Nonce != nil {
		g.Nonce = uint64(*dec.Nonce)
	}
	if dec.PrivateKey != nil {
		g.PrivateKey = *dec.PrivateKey
	}
	return nil
}
