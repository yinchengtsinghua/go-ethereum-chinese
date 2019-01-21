
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

package core

import (
	"errors"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

var (
	errInsufficientBalanceForGas = errors.New("insufficient balance to pay for gas")
)

/*
状态转换模型

状态转换是将事务应用于当前世界状态时所做的更改。
状态转换模型完成了所有必要的工作，以计算出有效的新状态根。

1）非紧急处理
2）预付费煤气
3）如果收件人是\0*32，则创建新的状态对象
4）价值转移
==如果合同创建==
  4a）尝试运行事务数据
  4b）如果有效，将结果用作新状态对象的代码
=结束＝=
5）运行脚本部分
6）派生新状态根
**/

type StateTransition struct {
	gp         *GasPool
	msg        Message
	gas        uint64
	gasPrice   *big.Int
	initialGas uint64
	value      *big.Int
	data       []byte
	state      vm.StateDB
	evm        *vm.EVM
}

//消息表示发送到合同的消息。
type Message interface {
	From() common.Address
//fromfrontier（）（common.address，错误）
	To() *common.Address

	GasPrice() *big.Int
	Gas() uint64
	Value() *big.Int

	Nonce() uint64
	CheckNonce() bool
	Data() []byte
}

//IntrinsicGas用给定的数据计算消息的“固有气体”。
func IntrinsicGas(data []byte, contractCreation, homestead bool) (uint64, error) {
//设置原始交易的起始气体
	var gas uint64
	if contractCreation && homestead {
		gas = params.TxGasContractCreation
	} else {
		gas = params.TxGas
	}
//按事务数据量通气所需的气体
	if len(data) > 0 {
//零字节和非零字节的定价不同
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
//确保所有数据组合都不超过uint64
		if (math.MaxUint64-gas)/params.TxDataNonZeroGas < nz {
			return 0, vm.ErrOutOfGas
		}
		gas += nz * params.TxDataNonZeroGas

		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/params.TxDataZeroGas < z {
			return 0, vm.ErrOutOfGas
		}
		gas += z * params.TxDataZeroGas
	}
	return gas, nil
}

//NewStateTransmission初始化并返回新的状态转换对象。
func NewStateTransition(evm *vm.EVM, msg Message, gp *GasPool) *StateTransition {
	return &StateTransition{
		gp:       gp,
		evm:      evm,
		msg:      msg,
		gasPrice: msg.GasPrice(),
		value:    msg.Value(),
		data:     msg.Data(),
		state:    evm.StateDB,
	}
}

//ApplyMessage通过应用给定消息来计算新状态
//反对环境中的旧状态。
//
//ApplyMessage返回任何EVM执行（如果发生）返回的字节，
//使用的气体（包括气体退款）和失败的错误。总是出错
//指示一个核心错误，意味着该消息将总是失败。
//在一个街区内永远不会被接受。
func ApplyMessage(evm *vm.EVM, msg Message, gp *GasPool) ([]byte, uint64, bool, error) {
	return NewStateTransition(evm, msg, gp).TransitionDb()
}

//返回邮件的收件人。
func (st *StateTransition) to() common.Address {
 /*st.msg==nil st.msg.to（）==nil/*合同创建*/
  返回公共地址
 }
 返回*st.msg.to（）。
}

func (st *StateTransition) useGas(amount uint64) error {
 如果st.gas<amount
  返回vm.erroutofgas
 }
 ST.气体-=数量

 返回零
}

func（st*statetransition）buygas（）错误
 mgval：=new（big.int）.mul（new（big.int）.setuint64（st.msg.gas（）），st.gasprice）
 if st.state.getbalance（st.msg.from（））.cmp（mgval）<0_
  返回不平衡气体
 }
 如果错误：=st.gp.subgas（st.msg.gas（））；错误！= nIL{
  返回错误
 }
 st.gas+=st.msg.gas（）。

 st.initialgas=st.msg.gas（）。
 st.state.subbalance（st.msg.from（），mgval）
 返回零
}

func（st*statetransition）precheck（）错误
 //确保此事务的nonce是正确的。
 if st.msg.checknonce（）
  nonce:=st.state.getnonce（st.msg.from（））
  如果nonce<st.msg.nonce（）
   返回erroncetohohigh
  else if nonce>st.msg.nonce（）
   返回erroncetoolow
  }
 }
 返回圣布伊加斯（St.Buygas）
}

//transitionDB将通过应用当前消息和
//返回包括已用气体在内的结果。如果失败，则返回错误。
//错误表示一致性问题。
func（st*statetransition）transitiondb（）（ret[]byte，usedgas uint64，failed bool，err error）
 如果err=st.precheck（）；err！= nIL{
  返回
 }
 MSG：= ST.MSG
 发件人：=vm.accountRef（msg.from（））
 homestead:=st.evm.chainconfig（）.ishomestead（st.evm.blocknumber）
 合同创建：=msg.to（）==nil

 //支付天然气
 气体，误差：=内部气体（St.Data，ContractCreation，Homestead）
 如果犯错！= nIL{
  返回nil、0、false、err
 }
 如果err=st.usegas（gas）；err！= nIL{
  返回nil、0、false、err
 }

 var
  EVM＝ST.EVM
  //VM错误不影响共识，因此
  //未分配给err，余额不足除外
  /错误。
  VMER误差
 ）
 如果合同创建
  ret，，st.gas，vmerr=evm.create（发送方，st.data，st.gas，st.value）
 }否则{
  //为下一个事务增加nonce
  st.state.setnonce（msg.from（），st.state.getnonce（sender.address（））+1）
  ret，st.gas，vmerr=evm.call（sender，st.to（），st.data，st.gas，st.value）
 }
 如果VMRR！= nIL{
  log.debug（“返回的VM有错误”，“err”，vm err）
  //唯一可能的共识错误是如果没有
  //有足够的余额进行转移。第一
  //余额转移永远不会失败。
  如果vmerr==vm.errUnsuffictBalance
   返回nil、0、false、vmerr
  }
 }
 S.ReffgdGas（）
 添加平衡（st.evm.coinbase，new（big.int）.mul（new（big.int）.setuint64（st.gasused（）），st.gasprice）

 返回ret，st.gasused（），vmerr！=零
}

func（st*statetransition）refundgas（）
 //申请退款柜台，上限为已用气体的一半。
 退款：=st.gasused（）/2
 如果退款>st.state.get退款（）
  退款=st.state.get退款（）
 }
 ST.GAS+=退款

 //剩余气体返回eth，按原汇率交换。
 剩余：=new（big.int）.mul（new（big.int）.setuint64（st.gas），st.gasprice）
 st.state.addbalance（st.msg.from（），剩余）

 //同时将剩余气体返回至区块气计数器，因此
 //可用于下一个事务。
 添加气体（ST.GAS）
}

//gas used返回状态转换所消耗的气体量。
func（st*statetransition）gasused（）uint64_
 返回St.InitialGas-St.Gas
}
