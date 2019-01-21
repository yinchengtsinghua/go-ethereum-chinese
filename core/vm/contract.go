
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

package vm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

//contractRef是对合同支持对象的引用
type ContractRef interface {
	Address() common.Address
}

//accountRef执行contractRef。
//
//在EVM初始化和
//它的主要用途是获取地址。删除此对象
//由于缓存的跳转目的地
//从父合同（即调用者）中提取，其中
//是ContractRef。
type AccountRef common.Address

//地址将accountRef强制转换为地址
func (ar AccountRef) Address() common.Address { return (common.Address)(ar) }

//契约表示状态数据库中的以太坊契约。它包含
//合同代码，调用参数。合同执行合同参考号
type Contract struct {
//CallerAddress是调用方初始化此项的结果
//合同。但是，当“调用方法”被委托时，这个值
//需要初始化为调用方的调用方的调用方。
	CallerAddress common.Address
	caller        ContractRef
	self          ContractRef

jumpdests map[common.Hash]bitvec //JumpDest分析的汇总结果。
analysis  bitvec                 //JumpDest分析的本地缓存结果

	Code     []byte
	CodeHash common.Hash
	CodeAddr *common.Address
	Input    []byte

	Gas   uint64
	value *big.Int
}

//NewContract返回执行EVM的新合同环境。
func NewContract(caller ContractRef, object ContractRef, value *big.Int, gas uint64) *Contract {
	c := &Contract{CallerAddress: caller.Address(), caller: caller, self: object}

	if parent, ok := caller.(*Contract); ok {
//如果可用，请重新使用父上下文中的JumpDest分析。
		c.jumpdests = parent.jumpdests
	} else {
		c.jumpdests = make(map[common.Hash]bitvec)
	}

//气体应该是一个指针，这样可以在运行过程中安全地减少气体。
//此指针将关闭状态转换
	c.Gas = gas
//确保设置了值
	c.value = value

	return c
}

func (c *Contract) validJumpdest(dest *big.Int) bool {
	udest := dest.Uint64()
//PC不能超过len（code），当然不能超过63位。
//在这种情况下，不要费心检查Jumpdest。
	if dest.BitLen() >= 63 || udest >= uint64(len(c.Code)) {
		return false
	}
//仅允许目的地使用JumpDest
	if OpCode(c.Code[udest]) != JUMPDEST {
		return false
	}
//我们已经有合同哈希了吗？
	if c.CodeHash != (common.Hash{}) {
//父上下文有分析吗？
		analysis, exist := c.jumpdests[c.CodeHash]
		if !exist {
//在父上下文中进行分析并保存
//我们不需要将它存储在c.analysis中。
			analysis = codeBitmap(c.Code)
			c.jumpdests[c.CodeHash] = analysis
		}
		return analysis.codeSegment(udest)
	}
//我们没有代码哈希，很可能还没有一段initcode
//在州TIE。在这种情况下，我们进行分析，并将其保存在本地，因此
//我们不必为执行中的每个跳转指令重新计算它
//但是，我们不会在父上下文中保存它。
	if c.analysis == nil {
		c.analysis = codeBitmap(c.Code)
	}
	return c.analysis.codeSegment(udest)
}

//asdelegate将协定设置为委托调用并返回当前
//合同（用于链接呼叫）
func (c *Contract) AsDelegate() *Contract {
//注：呼叫者必须始终是合同。这不应该发生
//打电话的不是合同。
	parent := c.caller.(*Contract)
	c.CallerAddress = parent.CallerAddress
	c.value = parent.value

	return c
}

//getop返回契约字节数组中的第n个元素
func (c *Contract) GetOp(n uint64) OpCode {
	return OpCode(c.GetByte(n))
}

//GetByte返回协定字节数组中的第n个字节
func (c *Contract) GetByte(n uint64) byte {
	if n < uint64(len(c.Code)) {
		return c.Code[n]
	}

	return 0
}

//调用者返回合同的调用者。
//
//当协定是委托时，调用方将递归调用调用方
//呼叫，包括呼叫者的呼叫。
func (c *Contract) Caller() common.Address {
	return c.CallerAddress
}

//use gas尝试使用气体并减去它，成功后返回true。
func (c *Contract) UseGas(gas uint64) (ok bool) {
	if c.Gas < gas {
		return false
	}
	c.Gas -= gas
	return true
}

//地址返回合同地址
func (c *Contract) Address() common.Address {
	return c.self.Address()
}

//value返回合同值（从调用方发送给它）
func (c *Contract) Value() *big.Int {
	return c.value
}

//setcallcode设置合同的代码和支持数据的地址
//对象
func (c *Contract) SetCallCode(addr *common.Address, hash common.Hash, code []byte) {
	c.Code = code
	c.CodeHash = hash
	c.CodeAddr = addr
}

//setcodeoptionalhash可用于提供代码，但提供hash是可选的。
//如果没有提供哈希，JumpDest分析将不会保存到父上下文中。
func (c *Contract) SetCodeOptionalHash(addr *common.Address, codeAndHash *codeAndHash) {
	c.Code = codeAndHash.code
	c.CodeHash = codeAndHash.hash
	c.CodeAddr = addr
}
