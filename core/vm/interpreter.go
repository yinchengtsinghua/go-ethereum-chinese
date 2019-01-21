
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

package vm

import (
	"fmt"
	"hash"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/params"
)

//config是解释器的配置选项
type Config struct {
//启用调试的调试解释器选项
	Debug bool
//跟踪程序是操作代码记录器
	Tracer Tracer
//无加密禁用的解释程序调用，调用代码，
//委派呼叫并创建。
	NoRecursion bool
//启用sha3/keccak preimages的录制
	EnablePreimageRecording bool
//JumpTable包含EVM指令表。这个
//可能未初始化，并将设置为默认值
//表。
	JumpTable [256]operation

//ewasm解释器的类型
	EWASMInterpreter string
//EVM解释器的类型
	EVMInterpreter string
}

//解释器用于运行基于以太坊的合同，并将使用
//passed environment to query external sources for state information.
//解释器将根据传递的
//配置。
type Interpreter interface {
//运行循环并使用给定的输入数据评估契约的代码并返回
//返回字节切片，如果出现错误，则返回一个错误。
	Run(contract *Contract, input []byte, static bool) ([]byte, error)
//canrun告诉作为参数传递的契约是否可以
//由当前解释器运行。这意味着
//呼叫方可以执行以下操作：
//
//'Gangang'
//对于u，解释器：=测距解释器
//if explorer.canrun（contract.code）
//解释器.run（contract.code，input）
//}
//}
//` `
	CanRun([]byte) bool
}

//keccakstate包裹sha3.state。除了通常的哈希方法外，它还支持
//读取以从哈希状态获取可变数量的数据。读取比求和快
//因为它不复制内部状态，而是修改内部状态。
type keccakState interface {
	hash.Hash
	Read([]byte) (int, error)
}

//evm interpreter表示evm解释器
type EVMInterpreter struct {
	evm      *EVM
	cfg      Config
	gasTable params.GasTable

	intPool *intPool

hasher    keccakState //keccak256哈希实例跨操作码共享
hasherBuf common.Hash //keccak256哈希结果数组共享aross操作码

readOnly   bool   //是否进行状态修改
returnData []byte //最后一次调用的返回数据供后续重用
}

//NewEvminterPreter返回解释器的新实例。
func NewEVMInterpreter(evm *EVM, cfg Config) *EVMInterpreter {
//我们使用停止指令是否看到
//跳转表已初始化。如果不是
//我们将设置默认跳转表。
	if !cfg.JumpTable[STOP].valid {
		switch {
		case evm.ChainConfig().IsConstantinople(evm.BlockNumber):
			cfg.JumpTable = constantinopleInstructionSet
		case evm.ChainConfig().IsByzantium(evm.BlockNumber):
			cfg.JumpTable = byzantiumInstructionSet
		case evm.ChainConfig().IsHomestead(evm.BlockNumber):
			cfg.JumpTable = homesteadInstructionSet
		default:
			cfg.JumpTable = frontierInstructionSet
		}
	}

	return &EVMInterpreter{
		evm:      evm,
		cfg:      cfg,
		gasTable: evm.ChainConfig().GasTable(evm.BlockNumber),
	}
}

func (in *EVMInterpreter) enforceRestrictions(op OpCode, operation operation, stack *Stack) error {
	if in.evm.chainRules.IsByzantium {
		if in.readOnly {
//如果解释器在只读模式下工作，请确保否
//执行状态修改操作。第三个堆栈项
//对于一个调用操作来说是值。从一个转移价值
//对其他人的帐户意味着状态被修改，并且应该
//返回时出错。
			if operation.writes || (op == CALL && stack.Back(2).BitLen() > 0) {
				return errWriteProtection
			}
		}
	}
	return nil
}

//运行循环并使用给定的输入数据评估契约的代码并返回
//返回字节切片，如果出现错误，则返回一个错误。
//
//需要注意的是，解释程序返回的任何错误都应该
//被认为是一种还原和消耗除
//errExecutionReverted，这意味着还原并保留气体。
func (in *EVMInterpreter) Run(contract *Contract, input []byte, readOnly bool) (ret []byte, err error) {
	if in.intPool == nil {
		in.intPool = poolOfIntPools.get()
		defer func() {
			poolOfIntPools.put(in.intPool)
			in.intPool = nil
		}()
	}

//增加限制为1024的调用深度
	in.evm.depth++
	defer func() { in.evm.depth-- }()

//确保仅当我们还未处于只读状态时才设置只读。
//这还确保不会删除子调用的只读标志。
	if readOnly && !in.readOnly {
		in.readOnly = true
		defer func() { in.readOnly = false }()
	}

//重置上一个呼叫的返回数据。保留旧的缓冲区并不重要
//因为每次回电都会返回新的数据。
	in.returnData = nil

//如果没有代码，就不必费心执行。
	if len(contract.Code) == 0 {
		return nil, nil
	}

	var (
op    OpCode        //当前操作码
mem   = NewMemory() //绑定内存
stack = newstack()  //本地栈
//为了优化，我们使用uint64作为程序计数器。
//理论上可以超过2^64。YP定义PC
//为UIT2525。实际上不那么可行。
pc   = uint64(0) //程序计数器
		cost uint64
//追踪器使用的副本
pcCopy  uint64 //延期追踪器需要
gasCopy uint64 //用于示踪剂记录执行前的剩余气体
logged  bool   //延迟跟踪程序应忽略已记录的步骤
	)
	contract.Input = input

//在执行停止时将堆栈作为int池回收
	defer func() { in.intPool.put(stack.data...) }()

	if in.cfg.Debug {
		defer func() {
			if err != nil {
				if !logged {
					in.cfg.Tracer.CaptureState(in.evm, pcCopy, op, gasCopy, cost, mem, stack, contract, in.evm.depth, err)
				} else {
					in.cfg.Tracer.CaptureFault(in.evm, pcCopy, op, gasCopy, cost, mem, stack, contract, in.evm.depth, err)
				}
			}
		}()
	}
//解释器主运行循环（上下文）。此循环运行到
//执行显式停止、返回或自毁函数，期间发生错误
//执行一个操作，或直到完成标志由
//父上下文。
	for atomic.LoadInt32(&in.evm.abort) == 0 {
		if in.cfg.Debug {
//捕获执行前的值以进行跟踪。
			logged, pcCopy, gasCopy = false, pc, contract.Gas
		}

//从跳转表中获取操作并验证堆栈以确保
//有足够的堆栈项可用于执行该操作。
		op = contract.GetOp(pc)
		operation := in.cfg.JumpTable[op]
		if !operation.valid {
			return nil, fmt.Errorf("invalid opcode 0x%x", int(op))
		}
		if err := operation.validateStack(stack); err != nil {
			return nil, err
		}
//如果操作有效，则强制执行并写入限制
		if err := in.enforceRestrictions(op, operation, stack); err != nil {
			return nil, err
		}

		var memorySize uint64
//计算新内存大小并展开内存以适应
//手术
		if operation.memorySize != nil {
			memSize, overflow := bigUint64(operation.memorySize(stack))
			if overflow {
				return nil, errGasUintOverflow
			}
//内存以32字节的字扩展。气体
//也用文字计算。
			if memorySize, overflow = math.SafeMul(toWordSize(memSize), 32); overflow {
				return nil, errGasUintOverflow
			}
		}
//如果没有足够的气体可用，则消耗气体并返回错误。
//明确设置成本，以便捕获状态延迟方法可以获得适当的成本
		cost, err = operation.gasCost(in.gasTable, in.evm, contract, stack, mem, memorySize)
		if err != nil || !contract.UseGas(cost) {
			return nil, ErrOutOfGas
		}
		if memorySize > 0 {
			mem.Resize(memorySize)
		}

		if in.cfg.Debug {
			in.cfg.Tracer.CaptureState(in.evm, pc, op, gasCopy, cost, mem, stack, contract, in.evm.depth, err)
			logged = true
		}

//执行操作
		res, err := operation.execute(&pc, in, contract, mem, stack)
//VerifyPool是一个生成标志。池验证确保完整性
//通过将值与默认值进行比较来获得整数池的值。
		if verifyPool {
			verifyIntegerPool(in.intPool)
		}
//如果操作清除返回数据（例如，它有返回数据）
//将最后一个返回设置为操作结果。
		if operation.returns {
			in.returnData = res
		}

		switch {
		case err != nil:
			return nil, err
		case operation.reverts:
			return res, errExecutionReverted
		case operation.halts:
			return res, nil
		case !operation.jumps:
			pc++
		}
	}
	return nil, nil
}

//canrun告诉作为参数传递的契约是否可以
//由当前解释器运行。
func (in *EVMInterpreter) CanRun(code []byte) bool {
	return true
}
