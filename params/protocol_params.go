
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

package params

import "math/big"

const (
GasLimitBoundDivisor uint64 = 1024    //气体极限的界限除数，用于更新计算。
MinGasLimit          uint64 = 5000    //气体极限可能是最小值。
GenesisGasLimit      uint64 = 4712388 //成因区块气限。

MaximumExtraDataSize  uint64 = 32    //最大尺寸的额外数据可能在Genesis之后。
ExpByteGas            uint64 = 10    //乘以exp指令的CEIL（log256（指数））。
SloadGas              uint64 = 50    //乘以为任何*复制操作复制（舍入）并添加的32字节字数。
CallValueTransferGas  uint64 = 9000  //当价值转移为非零时支付呼叫费用。
CallNewAccountGas     uint64 = 25000 //当目标地址以前不存在时支付呼叫费用。
TxGas                 uint64 = 21000 //未创建合同的每笔交易。注：交易之间的通话数据不支付。
TxGasContractCreation uint64 = 53000 //创建合同的每个事务。注：交易之间的通话数据不支付。
TxDataZeroGas         uint64 = 4     //附加到等于零的事务的每字节数据。注：交易之间的通话数据不支付。
QuadCoeffDiv          uint64 = 512   //内存成本方程二次粒子的除数。
LogDataGas            uint64 = 8     //日志*操作数据中的每个字节。
CallStipend           uint64 = 2300  //呼叫开始时提供的游离气体。

Sha3Gas     uint64 = 30 //每运行一次。
Sha3WordGas uint64 = 6  //sha3操作数据的每个字一次。

SstoreSetGas    uint64 = 20000 //每个sload操作一次。
SstoreResetGas  uint64 = 5000  //如果零度从零变为零，则每个SStore操作一次。
SstoreClearGas  uint64 = 5000  //如果零度不变，则每个sstore操作一次。
SstoreRefundGas uint64 = 15000 //如果零度更改为零，则每个SStore操作一次。

NetSstoreNoopGas  uint64 = 200   //如果值不变，则每个sstore操作一次。
NetSstoreInitGas  uint64 = 20000 //从清除零开始，每个SStore操作一次。
NetSstoreCleanGas uint64 = 5000  //每个SStore操作一次，从clean non-zero开始。
NetSstoreDirtyGas uint64 = 200   //每个SStore操作一次。

NetSstoreClearRefund      uint64 = 15000 //每个SStore操作一次，用于清除原来存在的存储槽
NetSstoreResetRefund      uint64 = 4800  //每次存储操作一次，用于重置为原始非零值
NetSstoreResetClearRefund uint64 = 19800 //每次存储操作一次，以重置为原始零值

JumpdestGas      uint64 = 1     //如果零度变为零，则每次存储操作一次，返回气体。
EpochDuration    uint64 = 30000 //工作证明阶段之间的持续时间。
CallGas          uint64 = 40    //每次呼叫操作和消息呼叫事务一次。
CreateDataGas    uint64 = 200   //
CallCreateDepth  uint64 = 1024  //调用/创建堆栈的最大深度。
ExpGas           uint64 = 10    //每exp指令一次
LogGas           uint64 = 375   //每个日志*操作。
CopyGas          uint64 = 3     //
StackLimit       uint64 = 1024  //允许的VM堆栈的最大大小。
TierStepGas      uint64 = 0     //每个操作一次，供选择。
LogTopicGas      uint64 = 375   //乘以每个日志事务的日志*。例如，log0招致0*c_txlogtopicgas，log4招致4*c_txlogtopicgas。
CreateGas        uint64 = 32000 //每次创建操作和合同创建交易一次。
Create2Gas       uint64 = 32000 //每个create2操作一次
SuicideRefundGas uint64 = 24000 //自杀手术后退款。
MemoryGas        uint64 = 3     //乘以（内存中引用的最高字节数+1）的地址。注意：引用发生在读、写以及返回和调用等指令中。
TxDataNonZeroGas uint64 = 68    //附加到不等于零的事务的每字节数据。注：交易之间的通话数据不支付。

MaxCodeSize = 24576 //合同允许的最大字节码

//预编译合同天然气价格

EcrecoverGas            uint64 = 3000   //椭圆曲线发送器回收气价格
Sha256BaseGas           uint64 = 60     //sha256操作的基价
Sha256PerWordGas        uint64 = 12     //sha256操作的每字价格
Ripemd160BaseGas        uint64 = 600    //ripemd160操作的基价
Ripemd160PerWordGas     uint64 = 120    //ripemd160操作的每字价格
IdentityBaseGas         uint64 = 15     //数据复制操作的基价
IdentityPerWordGas      uint64 = 3      //数据复制操作的每个工作价格
ModExpQuadCoeffDiv      uint64 = 20     //大整数模幂的二次粒子的除数
Bn256AddGas             uint64 = 500    //椭圆曲线加法所需的气体
Bn256ScalarMulGas       uint64 = 40000  //椭圆曲线标量乘法所需的气体
Bn256PairingBaseGas     uint64 = 100000 //椭圆曲线配对检验的基价
Bn256PairingPerPointGas uint64 = 80000  //椭圆曲线配对检查的每点价格
)

var (
DifficultyBoundDivisor = big.NewInt(2048)   //难度的界限除数，用于更新计算。
GenesisDifficulty      = big.NewInt(131072) //创世纪板块的难度。
MinimumDifficulty      = big.NewInt(131072) //难度可能是最小的。
DurationLimit          = big.NewInt(13)     //用于确定难度是否应增加的块时间持续时间的决策边界。
)
