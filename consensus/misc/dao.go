
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

package misc

import (
	"bytes"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

var (
//如果头不支持
//Pro Fork客户端。
	ErrBadProDAOExtra = errors.New("bad DAO pro-fork extra-data")

//如果头确实支持no-
//分支客户机。
	ErrBadNoDAOExtra = errors.New("bad DAO no-fork extra-data")
)

//verifydaoHeaderextradata验证块头的额外数据字段
//确保符合刀硬叉规则。
//
//DAO硬分叉扩展到头的有效性：
//
//使用fork特定的额外数据集
//b）如果节点是pro fork，则需要特定范围内的块具有
//唯一的额外数据集。
func VerifyDAOHeaderExtraData(config *params.ChainConfig, header *types.Header) error {
//如果节点不关心DAO分叉，则进行短路验证
	if config.DAOForkBlock == nil {
		return nil
	}
//确保块在fork修改的额外数据范围内
	limit := new(big.Int).Add(config.DAOForkBlock, params.DAOForkExtraRange)
	if header.Number.Cmp(config.DAOForkBlock) < 0 || header.Number.Cmp(limit) >= 0 {
		return nil
	}
//根据我们支持还是反对fork，验证额外的数据内容
	if config.DAOForkSupport {
		if !bytes.Equal(header.Extra, params.DAOForkBlockExtra) {
			return ErrBadProDAOExtra
		}
	} else {
		if bytes.Equal(header.Extra, params.DAOForkBlockExtra) {
			return ErrBadNoDAOExtra
		}
	}
//好吧，header有我们期望的额外数据
	return nil
}

//ApplyDaoHardFork根据DAO Hard Fork修改状态数据库
//规则，将一组DAO帐户的所有余额转移到单个退款
//合同。
func ApplyDAOHardFork(statedb *state.StateDB) {
//检索要将余额退款到的合同
	if !statedb.Exist(params.DAORefundContract) {
		statedb.CreateAccount(params.DAORefundContract)
	}

//将每个DAO帐户和额外的余额帐户资金转移到退款合同中
	for _, addr := range params.DAODrainList() {
		statedb.AddBalance(params.DAORefundContract, statedb.GetBalance(addr))
		statedb.SetBalance(addr, new(big.Int))
	}
}
