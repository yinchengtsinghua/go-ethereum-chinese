
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//代码生成-不要编辑。
//此文件是生成的绑定，任何手动更改都将丢失。

package contract

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

//
const FIFSRegistrarABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"subnode\",\"type\":\"bytes32\"},{\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"register\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"ensAddr\",\"type\":\"address\"},{\"name\":\"node\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"}]"

//FIFSREgistrarbin是用于部署新合同的编译字节码。
const FIFSRegistrarBin = `0x6060604052341561000f57600080fd5b604051604080610224833981016040528080519190602001805160008054600160a060020a03909516600160a060020a03199095169490941790935550506001556101c58061005f6000396000f3006060604052600436106100275763ffffffff60e060020a600035041663d22057a9811461002c575b600080fd5b341561003757600080fd5b61004e600435600160a060020a0360243516610050565b005b816000806001548360405191825260208201526040908101905190819003902060008054919350600160a060020a03909116906302571be39084906040516020015260405160e060020a63ffffffff84160281526004810191909152602401602060405180830381600087803b15156100c857600080fd5b6102c65a03f115156100d957600080fd5b5050506040518051915050600160a060020a0381161580159061010e575033600160a060020a031681600160a060020a031614155b1561011857600080fd5b600054600154600160a060020a03909116906306ab592390878760405160e060020a63ffffffff861602815260048101939093526024830191909152600160a060020a03166044820152606401600060405180830381600087803b151561017e57600080fd5b6102c65a03f1151561018f57600080fd5b50505050505050505600a165627a7a723058206fb963cb168d5e3a51af12cd6bb23e324dbd32dd4954f43653ba27e66b68ea650029`

//DeployyFifsRegistrar部署新的以太坊合同，将FifsRegistrar的实例绑定到该合同。
func DeployFIFSRegistrar(auth *bind.TransactOpts, backend bind.ContractBackend, ensAddr common.Address, node [32]byte) (common.Address, *types.Transaction, *FIFSRegistrar, error) {
	parsed, err := abi.JSON(strings.NewReader(FIFSRegistrarABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(FIFSRegistrarBin), backend, ensAddr, node)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &FIFSRegistrar{FIFSRegistrarCaller: FIFSRegistrarCaller{contract: contract}, FIFSRegistrarTransactor: FIFSRegistrarTransactor{contract: contract}, FIFSRegistrarFilterer: FIFSRegistrarFilterer{contract: contract}}, nil
}

//FIFSREGISTRAR是围绕以太坊合同自动生成的Go绑定。
type FIFSRegistrar struct {
FIFSRegistrarCaller     //对合同具有只读约束力
FIFSRegistrarTransactor //只写对合同有约束力
FIFSRegistrarFilterer   //合同事件的日志筛选程序
}

//FifsRegistrarCaller是围绕以太坊合约自动生成的只读Go绑定。
type FIFSRegistrarCaller struct {
contract *bind.BoundContract //用于低级调用的通用协定包装器
}

//FIFSREgistrarTransactior是围绕以太坊合同自动生成的只写Go绑定。
type FIFSRegistrarTransactor struct {
contract *bind.BoundContract //
}

//FIFSREgistrarFilter是围绕以太坊合同事件自动生成的日志筛选Go绑定。
type FIFSRegistrarFilterer struct {
contract *bind.BoundContract //用于低级调用的通用协定包装器
}

//FifsRegistrarSession是围绕以太坊合同自动生成的Go绑定，
//具有预设的调用和事务处理选项。
type FIFSRegistrarSession struct {
Contract     *FIFSRegistrar    //为其设置会话的通用约定绑定
CallOpts     bind.CallOpts     //在整个会话中使用的调用选项
TransactOpts bind.TransactOpts //要在此会话中使用的事务验证选项
}

//FifsRegistrarCallersession是围绕以太坊合约自动生成的只读Go绑定，
//带预设通话选项。
type FIFSRegistrarCallerSession struct {
Contract *FIFSRegistrarCaller //用于设置会话的通用协定调用方绑定
CallOpts bind.CallOpts        //在整个会话中使用的调用选项
}

//FIFSREgistrarTransactiorSession是围绕以太坊合同自动生成的只写Go绑定，
//具有预设的Transact选项。
type FIFSRegistrarTransactorSession struct {
Contract     *FIFSRegistrarTransactor //用于设置会话的通用合同事务处理程序绑定
TransactOpts bind.TransactOpts        //要在此会话中使用的事务验证选项
}

//FIFSREGISTRRAW是围绕以太坊合同自动生成的低级Go绑定。
type FIFSRegistrarRaw struct {
Contract *FIFSRegistrar //用于访问上的原始方法的通用合同绑定
}

//FifsRegistrarCallerraw是围绕以太坊合约自动生成的低级只读Go绑定。
type FIFSRegistrarCallerRaw struct {
Contract *FIFSRegistrarCaller //用于访问上的原始方法的通用只读协定绑定
}

//FIFSREgistrarTransactorraw是围绕以太坊合同自动生成的低级只写即用绑定。
type FIFSRegistrarTransactorRaw struct {
Contract *FIFSRegistrarTransactor //用于访问上的原始方法的通用只写协定绑定
}

//new fifsregistrar创建一个新的fifsregistrar实例，绑定到特定的已部署合同。
func NewFIFSRegistrar(address common.Address, backend bind.ContractBackend) (*FIFSRegistrar, error) {
	contract, err := bindFIFSRegistrar(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &FIFSRegistrar{FIFSRegistrarCaller: FIFSRegistrarCaller{contract: contract}, FIFSRegistrarTransactor: FIFSRegistrarTransactor{contract: contract}, FIFSRegistrarFilterer: FIFSRegistrarFilterer{contract: contract}}, nil
}

//NewFifsRegistrarCaller创建一个新的FifsRegistrar只读实例，绑定到特定的已部署协定。
func NewFIFSRegistrarCaller(address common.Address, caller bind.ContractCaller) (*FIFSRegistrarCaller, error) {
	contract, err := bindFIFSRegistrar(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &FIFSRegistrarCaller{contract: contract}, nil
}

//NewFifsRegistrarTransactior创建一个新的FifsRegistrar的只写实例，绑定到一个特定的已部署协定。
func NewFIFSRegistrarTransactor(address common.Address, transactor bind.ContractTransactor) (*FIFSRegistrarTransactor, error) {
	contract, err := bindFIFSRegistrar(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &FIFSRegistrarTransactor{contract: contract}, nil
}

//NewFifsRegistrarFilter创建一个新的FifsRegistrar日志筛选器实例，绑定到特定的已部署协定。
func NewFIFSRegistrarFilterer(address common.Address, filterer bind.ContractFilterer) (*FIFSRegistrarFilterer, error) {
	contract, err := bindFIFSRegistrar(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &FIFSRegistrarFilterer{contract: contract}, nil
}

//bindFifsRegistrar将通用包装绑定到已部署的协定。
func bindFIFSRegistrar(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(FIFSRegistrarABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

//调用调用（常量）contract方法，参数作为输入值，并且
//将输出设置为结果。结果类型可能是用于
//返回、匿名返回的接口切片和命名的结构
//返回。
func (_FIFSRegistrar *FIFSRegistrarRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _FIFSRegistrar.Contract.FIFSRegistrarCaller.contract.Call(opts, result, method, params...)
}

//转账启动普通交易以将资金转移到合同，调用
//它的默认方法（如果有）。
func (_FIFSRegistrar *FIFSRegistrarRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FIFSRegistrar.Contract.FIFSRegistrarTransactor.contract.Transfer(opts)
}

//Transact使用参数作为输入值调用（付费）Contract方法。
func (_FIFSRegistrar *FIFSRegistrarRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _FIFSRegistrar.Contract.FIFSRegistrarTransactor.contract.Transact(opts, method, params...)
}

//调用调用（常量）contract方法，参数作为输入值，并且
//将输出设置为结果。结果类型可能是用于
//返回、匿名返回的接口切片和命名的结构
//返回。
func (_FIFSRegistrar *FIFSRegistrarCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _FIFSRegistrar.Contract.contract.Call(opts, result, method, params...)
}

//转账启动普通交易以将资金转移到合同，调用
//它的默认方法（如果有）。
func (_FIFSRegistrar *FIFSRegistrarTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FIFSRegistrar.Contract.contract.Transfer(opts)
}

//Transact使用参数作为输入值调用（付费）Contract方法。
func (_FIFSRegistrar *FIFSRegistrarTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _FIFSRegistrar.Contract.contract.Transact(opts, method, params...)
}

//寄存器是一个受合同方法0xD22057A9约束的付费转换器事务。
//
//solidity:函数寄存器（子节点字节32，所有者地址）返回（）
func (_FIFSRegistrar *FIFSRegistrarTransactor) Register(opts *bind.TransactOpts, subnode [32]byte, owner common.Address) (*types.Transaction, error) {
	return _FIFSRegistrar.contract.Transact(opts, "register", subnode, owner)
}

//寄存器是一个受合同方法0xD22057A9约束的付费转换器事务。
//
//solidity:函数寄存器（子节点字节32，所有者地址）返回（）
func (_FIFSRegistrar *FIFSRegistrarSession) Register(subnode [32]byte, owner common.Address) (*types.Transaction, error) {
	return _FIFSRegistrar.Contract.Register(&_FIFSRegistrar.TransactOpts, subnode, owner)
}

//寄存器是一个受合同方法0xD22057A9约束的付费转换器事务。
//
//solidity:函数寄存器（子节点字节32，所有者地址）返回（）
func (_FIFSRegistrar *FIFSRegistrarTransactorSession) Register(subnode [32]byte, owner common.Address) (*types.Transaction, error) {
	return _FIFSRegistrar.Contract.Register(&_FIFSRegistrar.TransactOpts, subnode, owner)
}
