
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
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

//publicResolverabi是用于从中生成绑定的输入abi。
const PublicResolverABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"interfaceID\",\"type\":\"bytes4\"}],\"name\":\"supportsInterface\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"value\",\"type\":\"string\"}],\"name\":\"setText\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"contentTypes\",\"type\":\"uint256\"}],\"name\":\"ABI\",\"outputs\":[{\"name\":\"contentType\",\"type\":\"uint256\"},{\"name\":\"data\",\"type\":\"bytes\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"x\",\"type\":\"bytes32\"},{\"name\":\"y\",\"type\":\"bytes32\"}],\"name\":\"setPubkey\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"}],\"name\":\"content\",\"outputs\":[{\"name\":\"ret\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"}],\"name\":\"addr\",\"outputs\":[{\"name\":\"ret\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"key\",\"type\":\"string\"}],\"name\":\"text\",\"outputs\":[{\"name\":\"ret\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"contentType\",\"type\":\"uint256\"},{\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"setABI\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"}],\"name\":\"name\",\"outputs\":[{\"name\":\"ret\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"name\",\"type\":\"string\"}],\"name\":\"setName\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"hash\",\"type\":\"bytes32\"}],\"name\":\"setContent\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"}],\"name\":\"pubkey\",\"outputs\":[{\"name\":\"x\",\"type\":\"bytes32\"},{\"name\":\"y\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"setAddr\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"ensAddr\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"a\",\"type\":\"address\"}],\"name\":\"AddrChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"hash\",\"type\":\"bytes32\"}],\"name\":\"ContentChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"name\",\"type\":\"string\"}],\"name\":\"NameChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":true,\"name\":\"contentType\",\"type\":\"uint256\"}],\"name\":\"ABIChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"x\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"y\",\"type\":\"bytes32\"}],\"name\":\"PubkeyChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":true,\"name\":\"indexedKey\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"key\",\"type\":\"string\"}],\"name\":\"TextChanged\",\"type\":\"event\"}]"

//publicResolverbin是用于部署新合同的编译字节码。
const PublicResolverBin = `0x6060604052341561000f57600080fd5b6040516020806111b28339810160405280805160008054600160a060020a03909216600160a060020a0319909216919091179055505061115e806100546000396000f3006060604052600436106100ab5763ffffffff60e060020a60003504166301ffc9a781146100b057806310f13a8c146100e45780632203ab561461017e57806329cd62ea146102155780632dff6941146102315780633b3b57de1461025957806359d1d43c1461028b578063623195b014610358578063691f3431146103b457806377372213146103ca578063c3d014d614610420578063c869023314610439578063d5fa2b0014610467575b600080fd5b34156100bb57600080fd5b6100d0600160e060020a031960043516610489565b604051901515815260200160405180910390f35b34156100ef57600080fd5b61017c600480359060446024803590810190830135806020601f8201819004810201604051908101604052818152929190602084018383808284378201915050505050509190803590602001908201803590602001908080601f0160208091040260200160405190810160405281815292919060208401838380828437509496506105f695505050505050565b005b341561018957600080fd5b610197600435602435610807565b60405182815260406020820181815290820183818151815260200191508051906020019080838360005b838110156101d95780820151838201526020016101c1565b50505050905090810190601f1680156102065780820380516001836020036101000a031916815260200191505b50935050505060405180910390f35b341561022057600080fd5b61017c600435602435604435610931565b341561023c57600080fd5b610247600435610a30565b60405190815260200160405180910390f35b341561026457600080fd5b61026f600435610a46565b604051600160a060020a03909116815260200160405180910390f35b341561029657600080fd5b6102e1600480359060446024803590810190830135806020601f82018190048102016040519081016040528181529291906020840183838082843750949650610a6195505050505050565b60405160208082528190810183818151815260200191508051906020019080838360005b8381101561031d578082015183820152602001610305565b50505050905090810190601f16801561034a5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b341561036357600080fd5b61017c600480359060248035919060649060443590810190830135806020601f82018190048102016040519081016040528181529291906020840183838082843750949650610b8095505050505050565b34156103bf57600080fd5b6102e1600435610c7c565b34156103d557600080fd5b61017c600480359060446024803590810190830135806020601f82018190048102016040519081016040528181529291906020840183838082843750949650610d4295505050505050565b341561042b57600080fd5b61017c600435602435610e8c565b341561044457600080fd5b61044f600435610f65565b60405191825260208201526040908101905180910390f35b341561047257600080fd5b61017c600435600160a060020a0360243516610f82565b6000600160e060020a031982167f3b3b57de0000000000000000000000000000000000000000000000000000000014806104ec5750600160e060020a031982167fd8389dc500000000000000000000000000000000000000000000000000000000145b806105205750600160e060020a031982167f691f343100000000000000000000000000000000000000000000000000000000145b806105545750600160e060020a031982167f2203ab5600000000000000000000000000000000000000000000000000000000145b806105885750600160e060020a031982167fc869023300000000000000000000000000000000000000000000000000000000145b806105bc5750600160e060020a031982167f59d1d43c00000000000000000000000000000000000000000000000000000000145b806105f05750600160e060020a031982167f01ffc9a700000000000000000000000000000000000000000000000000000000145b92915050565b600080548491600160a060020a033381169216906302571be39084906040516020015260405160e060020a63ffffffff84160281526004810191909152602401602060405180830381600087803b151561064f57600080fd5b6102c65a03f1151561066057600080fd5b50505060405180519050600160a060020a031614151561067f57600080fd5b6000848152600160205260409081902083916005909101908590518082805190602001908083835b602083106106c65780518252601f1990920191602091820191016106a7565b6001836020036101000a038019825116818451168082178552505050505050905001915050908152602001604051809103902090805161070a929160200190611085565b50826040518082805190602001908083835b6020831061073b5780518252601f19909201916020918201910161071c565b6001836020036101000a0380198251168184511617909252505050919091019250604091505051908190039020847fd8c9334b1a9c2f9da342a0a2b32629c1a229b6445dad78947f674b44444a75508560405160208082528190810183818151815260200191508051906020019080838360005b838110156107c75780820151838201526020016107af565b50505050905090810190601f1680156107f45780820380516001836020036101000a031916815260200191505b509250505060405180910390a350505050565b6000610811611103565b60008481526001602081905260409091209092505b838311610924578284161580159061085f5750600083815260068201602052604081205460026000196101006001841615020190911604115b15610919578060060160008481526020019081526020016000208054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561090d5780601f106108e25761010080835404028352916020019161090d565b820191906000526020600020905b8154815290600101906020018083116108f057829003601f168201915b50505050509150610929565b600290920291610826565b600092505b509250929050565b600080548491600160a060020a033381169216906302571be39084906040516020015260405160e060020a63ffffffff84160281526004810191909152602401602060405180830381600087803b151561098a57600080fd5b6102c65a03f1151561099b57600080fd5b50505060405180519050600160a060020a03161415156109ba57600080fd5b6040805190810160409081528482526020808301859052600087815260019091522060030181518155602082015160019091015550837f1d6f5e03d3f63eb58751986629a5439baee5079ff04f345becb66e23eb154e46848460405191825260208201526040908101905180910390a250505050565b6000908152600160208190526040909120015490565b600090815260016020526040902054600160a060020a031690565b610a69611103565b60008381526001602052604090819020600501908390518082805190602001908083835b60208310610aac5780518252601f199092019160209182019101610a8d565b6001836020036101000a03801982511681845116808217855250505050505090500191505090815260200160405180910390208054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610b735780601f10610b4857610100808354040283529160200191610b73565b820191906000526020600020905b815481529060010190602001808311610b5657829003601f168201915b5050505050905092915050565b600080548491600160a060020a033381169216906302571be39084906040516020015260405160e060020a63ffffffff84160281526004810191909152602401602060405180830381600087803b1515610bd957600080fd5b6102c65a03f11515610bea57600080fd5b50505060405180519050600160a060020a0316141515610c0957600080fd5b6000198301831615610c1a57600080fd5b60008481526001602090815260408083208684526006019091529020828051610c47929160200190611085565b5082847faa121bbeef5f32f5961a2a28966e769023910fc9479059ee3495d4c1a696efe360405160405180910390a350505050565b610c84611103565b6001600083600019166000191681526020019081526020016000206002018054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610d365780601f10610d0b57610100808354040283529160200191610d36565b820191906000526020600020905b815481529060010190602001808311610d1957829003601f168201915b50505050509050919050565b600080548391600160a060020a033381169216906302571be39084906040516020015260405160e060020a63ffffffff84160281526004810191909152602401602060405180830381600087803b1515610d9b57600080fd5b6102c65a03f11515610dac57600080fd5b50505060405180519050600160a060020a0316141515610dcb57600080fd5b6000838152600160205260409020600201828051610ded929160200190611085565b50827fb7d29e911041e8d9b843369e890bcb72c9388692ba48b65ac54e7214c4c348f78360405160208082528190810183818151815260200191508051906020019080838360005b83811015610e4d578082015183820152602001610e35565b50505050905090810190601f168015610e7a5780820380516001836020036101000a031916815260200191505b509250505060405180910390a2505050565b600080548391600160a060020a033381169216906302571be39084906040516020015260405160e060020a63ffffffff84160281526004810191909152602401602060405180830381600087803b1515610ee557600080fd5b6102c65a03f11515610ef657600080fd5b50505060405180519050600160a060020a0316141515610f1557600080fd5b6000838152600160208190526040918290200183905583907f0424b6fe0d9c3bdbece0e7879dc241bb0c22e900be8b6c168b4ee08bd9bf83bc9084905190815260200160405180910390a2505050565b600090815260016020526040902060038101546004909101549091565b600080548391600160a060020a033381169216906302571be39084906040516020015260405160e060020a63ffffffff84160281526004810191909152602401602060405180830381600087803b1515610fdb57600080fd5b6102c65a03f11515610fec57600080fd5b50505060405180519050600160a060020a031614151561100b57600080fd5b60008381526001602052604090819020805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a03851617905583907f52d7d861f09ab3d26239d492e8968629f95e9e318cf0b73bfddc441522a15fd290849051600160a060020a03909116815260200160405180910390a2505050565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106110c657805160ff19168380011785556110f3565b828001600101855582156110f3579182015b828111156110f35782518255916020019190600101906110d8565b506110ff929150611115565b5090565b60206040519081016040526000815290565b61112f91905b808211156110ff576000815560010161111b565b905600a165627a7a723058201ecacbc445b9fbcd91b0ab164389f69d7283b856883bc7437eeed1008345a4920029`

//部署公共解析器部署一个新的EUTHUM合同，将公共解析器的实例绑定到它。
func DeployPublicResolver(auth *bind.TransactOpts, backend bind.ContractBackend, ensAddr common.Address) (common.Address, *types.Transaction, *PublicResolver, error) {
	parsed, err := abi.JSON(strings.NewReader(PublicResolverABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(PublicResolverBin), backend, ensAddr)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &PublicResolver{PublicResolverCaller: PublicResolverCaller{contract: contract}, PublicResolverTransactor: PublicResolverTransactor{contract: contract}, PublicResolverFilterer: PublicResolverFilterer{contract: contract}}, nil
}

//PublicResolver是围绕以太坊契约自动生成的Go绑定。
type PublicResolver struct {
PublicResolverCaller     //对合同具有只读约束力
PublicResolverTransactor //只写对合同有约束力
PublicResolverFilterer   //合同事件的日志筛选程序
}

//PublicResolverCaller是围绕以太坊契约自动生成的只读Go绑定。
type PublicResolverCaller struct {
contract *bind.BoundContract //用于低级调用的通用协定包装器
}

//PublicResolverTransactior是围绕以太坊契约自动生成的只写Go绑定。
type PublicResolverTransactor struct {
contract *bind.BoundContract //用于低级调用的通用协定包装器
}

//PublicResolverFilter是围绕以太坊契约事件自动生成的日志筛选Go绑定。
type PublicResolverFilterer struct {
contract *bind.BoundContract //用于低级调用的通用协定包装器
}

//PublicResolverSession是围绕以太坊合同自动生成的Go绑定，
//具有预设的调用和事务处理选项。
type PublicResolverSession struct {
Contract     *PublicResolver   //为其设置会话的通用约定绑定
CallOpts     bind.CallOpts     //在整个会话中使用的调用选项
TransactOpts bind.TransactOpts //要在此会话中使用的事务验证选项
}

//PublicResolverCallersession是围绕以太坊合约自动生成的只读Go绑定，
//带预设通话选项。
type PublicResolverCallerSession struct {
Contract *PublicResolverCaller //用于设置会话的通用协定调用方绑定
CallOpts bind.CallOpts         //在整个会话中使用的调用选项
}

//PublicResolverTransactiorSession是围绕以太坊合同自动生成的只写Go绑定，
//具有预设的Transact选项。
type PublicResolverTransactorSession struct {
Contract     *PublicResolverTransactor //用于设置会话的通用合同事务处理程序绑定
TransactOpts bind.TransactOpts         //要在此会话中使用的事务验证选项
}

//PublicResolverRaw是围绕以太坊合同自动生成的低级Go绑定。
type PublicResolverRaw struct {
Contract *PublicResolver //用于访问上的原始方法的通用合同绑定
}

//PublicResolverCallerraw是围绕以太坊契约自动生成的低级别只读Go绑定。
type PublicResolverCallerRaw struct {
Contract *PublicResolverCaller //用于访问上的原始方法的通用只读协定绑定
}

//PublicResolverTransactorraw是围绕以太坊合同自动生成的低级只写绑定。
type PublicResolverTransactorRaw struct {
Contract *PublicResolverTransactor //用于访问上的原始方法的通用只写协定绑定
}

//NewPublicResolver创建绑定到特定部署协定的PublicResolver的新实例。
func NewPublicResolver(address common.Address, backend bind.ContractBackend) (*PublicResolver, error) {
	contract, err := bindPublicResolver(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &PublicResolver{PublicResolverCaller: PublicResolverCaller{contract: contract}, PublicResolverTransactor: PublicResolverTransactor{contract: contract}, PublicResolverFilterer: PublicResolverFilterer{contract: contract}}, nil
}

//NewPublicResolver调用程序创建绑定到特定部署协定的PublicResolver的新只读实例。
func NewPublicResolverCaller(address common.Address, caller bind.ContractCaller) (*PublicResolverCaller, error) {
	contract, err := bindPublicResolver(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &PublicResolverCaller{contract: contract}, nil
}

//NewPublicResolverTransactior创建绑定到特定部署协定的PublicResolver的新的只写实例。
func NewPublicResolverTransactor(address common.Address, transactor bind.ContractTransactor) (*PublicResolverTransactor, error) {
	contract, err := bindPublicResolver(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &PublicResolverTransactor{contract: contract}, nil
}

//NewPublicResolverFilter创建PublicResolver的新日志筛选器实例，该实例绑定到特定的已部署协定。
func NewPublicResolverFilterer(address common.Address, filterer bind.ContractFilterer) (*PublicResolverFilterer, error) {
	contract, err := bindPublicResolver(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &PublicResolverFilterer{contract: contract}, nil
}

//BindPublicResolver将通用包装绑定到已部署的协定。
func bindPublicResolver(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(PublicResolverABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

//调用调用（常量）contract方法，参数作为输入值，并且
//将输出设置为结果。结果类型可能是用于
//返回、匿名返回的接口切片和命名的结构
//返回。
func (_PublicResolver *PublicResolverRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _PublicResolver.Contract.PublicResolverCaller.contract.Call(opts, result, method, params...)
}

//转账启动普通交易以将资金转移到合同，调用
//它的默认方法（如果有）。
func (_PublicResolver *PublicResolverRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _PublicResolver.Contract.PublicResolverTransactor.contract.Transfer(opts)
}

//Transact使用参数作为输入值调用（付费）Contract方法。
func (_PublicResolver *PublicResolverRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _PublicResolver.Contract.PublicResolverTransactor.contract.Transact(opts, method, params...)
}

//调用调用（常量）contract方法，参数作为输入值，并且
//将输出设置为结果。结果类型可能是用于
//返回、匿名返回的接口切片和命名的结构
//返回。
func (_PublicResolver *PublicResolverCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _PublicResolver.Contract.contract.Call(opts, result, method, params...)
}

//转账启动普通交易以将资金转移到合同，调用
//它的默认方法（如果有）。
func (_PublicResolver *PublicResolverTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _PublicResolver.Contract.contract.Transfer(opts)
}

//Transact使用参数作为输入值调用（付费）Contract方法。
func (_PublicResolver *PublicResolverTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _PublicResolver.Contract.contract.Transact(opts, method, params...)
}

//ABI是一个绑定契约方法0x2203ab56的免费数据检索调用。
//
//solidity:函数abi（node bytes32，contenttype uint256）常量返回（contenttype uint256，data bytes）
func (_PublicResolver *PublicResolverCaller) ABI(opts *bind.CallOpts, node [32]byte, contentTypes *big.Int) (struct {
	ContentType *big.Int
	Data        []byte
}, error) {
	ret := new(struct {
		ContentType *big.Int
		Data        []byte
	})
	out := ret
	err := _PublicResolver.contract.Call(opts, out, "ABI", node, contentTypes)
	return *ret, err
}

//ABI是一个绑定契约方法0x2203ab56的免费数据检索调用。
//
//solidity:函数abi（node bytes32，contenttype uint256）常量返回（contenttype uint256，data bytes）
func (_PublicResolver *PublicResolverSession) ABI(node [32]byte, contentTypes *big.Int) (struct {
	ContentType *big.Int
	Data        []byte
}, error) {
	return _PublicResolver.Contract.ABI(&_PublicResolver.CallOpts, node, contentTypes)
}

//ABI是一个绑定契约方法0x2203ab56的免费数据检索调用。
//
//solidity:函数abi（node bytes32，contenttype uint256）常量返回（contenttype uint256，data bytes）
func (_PublicResolver *PublicResolverCallerSession) ABI(node [32]byte, contentTypes *big.Int) (struct {
	ContentType *big.Int
	Data        []byte
}, error) {
	return _PublicResolver.Contract.ABI(&_PublicResolver.CallOpts, node, contentTypes)
}

//addr是绑定contract方法0x3b3b57de的自由数据检索调用。
//
//solidity:函数addr（node bytes32）常量返回（ret address）
func (_PublicResolver *PublicResolverCaller) Addr(opts *bind.CallOpts, node [32]byte) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _PublicResolver.contract.Call(opts, out, "addr", node)
	return *ret0, err
}

//addr是绑定contract方法0x3b3b57de的自由数据检索调用。
//
//solidity:函数addr（node bytes32）常量返回（ret address）
func (_PublicResolver *PublicResolverSession) Addr(node [32]byte) (common.Address, error) {
	return _PublicResolver.Contract.Addr(&_PublicResolver.CallOpts, node)
}

//addr是绑定contract方法0x3b3b57de的自由数据检索调用。
//
//solidity:函数addr（node bytes32）常量返回（ret address）
func (_PublicResolver *PublicResolverCallerSession) Addr(node [32]byte) (common.Address, error) {
	return _PublicResolver.Contract.Addr(&_PublicResolver.CallOpts, node)
}

//content是一个绑定contract方法0x2dff6941的免费数据检索调用。
//
//solidity:函数内容（节点字节32）常量返回（ret字节32）
func (_PublicResolver *PublicResolverCaller) Content(opts *bind.CallOpts, node [32]byte) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _PublicResolver.contract.Call(opts, out, "content", node)
	return *ret0, err
}

//content是一个绑定contract方法0x2dff6941的免费数据检索调用。
//
//solidity:函数内容（节点字节32）常量返回（ret字节32）
func (_PublicResolver *PublicResolverSession) Content(node [32]byte) ([32]byte, error) {
	return _PublicResolver.Contract.Content(&_PublicResolver.CallOpts, node)
}

//content是一个绑定contract方法0x2dff6941的免费数据检索调用。
//
//solidity:函数内容（节点字节32）常量返回（ret字节32）
func (_PublicResolver *PublicResolverCallerSession) Content(node [32]byte) ([32]byte, error) {
	return _PublicResolver.Contract.Content(&_PublicResolver.CallOpts, node)
}

//name是绑定协定方法0x691F3431的免费数据检索调用。
//
//solidity:函数名（node bytes32）常量返回（ret string）
func (_PublicResolver *PublicResolverCaller) Name(opts *bind.CallOpts, node [32]byte) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _PublicResolver.contract.Call(opts, out, "name", node)
	return *ret0, err
}

//name是绑定协定方法0x691F3431的免费数据检索调用。
//
//solidity:函数名（node bytes32）常量返回（ret string）
func (_PublicResolver *PublicResolverSession) Name(node [32]byte) (string, error) {
	return _PublicResolver.Contract.Name(&_PublicResolver.CallOpts, node)
}

//name是绑定协定方法0x691F3431的免费数据检索调用。
//
//solidity:函数名（node bytes32）常量返回（ret string）
func (_PublicResolver *PublicResolverCallerSession) Name(node [32]byte) (string, error) {
	return _PublicResolver.Contract.Name(&_PublicResolver.CallOpts, node)
}

//pubkey是绑定契约方法0xC8690233的免费数据检索调用。
//
//solidity:函数pubkey（node bytes 32）常量返回（x bytes 32，y bytes 32）
func (_PublicResolver *PublicResolverCaller) Pubkey(opts *bind.CallOpts, node [32]byte) (struct {
	X [32]byte
	Y [32]byte
}, error) {
	ret := new(struct {
		X [32]byte
		Y [32]byte
	})
	out := ret
	err := _PublicResolver.contract.Call(opts, out, "pubkey", node)
	return *ret, err
}

//pubkey是绑定契约方法0xC8690233的免费数据检索调用。
//
//solidity:函数pubkey（node bytes 32）常量返回（x bytes 32，y bytes 32）
func (_PublicResolver *PublicResolverSession) Pubkey(node [32]byte) (struct {
	X [32]byte
	Y [32]byte
}, error) {
	return _PublicResolver.Contract.Pubkey(&_PublicResolver.CallOpts, node)
}

//pubkey是绑定契约方法0xC8690233的免费数据检索调用。
//
//solidity:函数pubkey（node bytes 32）常量返回（x bytes 32，y bytes 32）
func (_PublicResolver *PublicResolverCallerSession) Pubkey(node [32]byte) (struct {
	X [32]byte
	Y [32]byte
}, error) {
	return _PublicResolver.Contract.Pubkey(&_PublicResolver.CallOpts, node)
}

//SUPPORTSInterface是一个绑定契约方法0x01FC9A7的免费数据检索调用。
//
//solidity:函数支持接口（interfaceid bytes 4）常量返回（bool）
func (_PublicResolver *PublicResolverCaller) SupportsInterface(opts *bind.CallOpts, interfaceID [4]byte) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _PublicResolver.contract.Call(opts, out, "supportsInterface", interfaceID)
	return *ret0, err
}

//SUPPORTSInterface是一个绑定契约方法0x01FC9A7的免费数据检索调用。
//
//solidity:函数支持接口（interfaceid bytes 4）常量返回（bool）
func (_PublicResolver *PublicResolverSession) SupportsInterface(interfaceID [4]byte) (bool, error) {
	return _PublicResolver.Contract.SupportsInterface(&_PublicResolver.CallOpts, interfaceID)
}

//SUPPORTSInterface是一个绑定契约方法0x01FC9A7的免费数据检索调用。
//
//solidity:函数支持接口（interfaceid bytes 4）常量返回（bool）
func (_PublicResolver *PublicResolverCallerSession) SupportsInterface(interfaceID [4]byte) (bool, error) {
	return _PublicResolver.Contract.SupportsInterface(&_PublicResolver.CallOpts, interfaceID)
}

//文本是绑定契约方法0x59D1D43C的免费数据检索调用。
//
//solidity:函数文本（节点字节32，键字符串）常量返回（ret字符串）
func (_PublicResolver *PublicResolverCaller) Text(opts *bind.CallOpts, node [32]byte, key string) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _PublicResolver.contract.Call(opts, out, "text", node, key)
	return *ret0, err
}

//文本是绑定契约方法0x59D1D43C的免费数据检索调用。
//
//solidity:函数文本（节点字节32，键字符串）常量返回（ret字符串）
func (_PublicResolver *PublicResolverSession) Text(node [32]byte, key string) (string, error) {
	return _PublicResolver.Contract.Text(&_PublicResolver.CallOpts, node, key)
}

//文本是绑定契约方法0x59D1D43C的免费数据检索调用。
//
//solidity:函数文本（节点字节32，键字符串）常量返回（ret字符串）
func (_PublicResolver *PublicResolverCallerSession) Text(node [32]byte, key string) (string, error) {
	return _PublicResolver.Contract.Text(&_PublicResolver.CallOpts, node, key)
}

//setabi是一个受合同方法0x623195b0约束的付费变元事务。
//
//solidity:函数setabi（node bytes 32，contenttype uint256，data bytes）返回（）
func (_PublicResolver *PublicResolverTransactor) SetABI(opts *bind.TransactOpts, node [32]byte, contentType *big.Int, data []byte) (*types.Transaction, error) {
	return _PublicResolver.contract.Transact(opts, "setABI", node, contentType, data)
}

//setabi是一个受合同方法0x623195b0约束的付费变元事务。
//
//solidity:函数setabi（node bytes 32，contenttype uint256，data bytes）返回（）
func (_PublicResolver *PublicResolverSession) SetABI(node [32]byte, contentType *big.Int, data []byte) (*types.Transaction, error) {
	return _PublicResolver.Contract.SetABI(&_PublicResolver.TransactOpts, node, contentType, data)
}

//setabi是一个受合同方法0x623195b0约束的付费变元事务。
//
//solidity:函数setabi（node bytes 32，contenttype uint256，data bytes）返回（）
func (_PublicResolver *PublicResolverTransactorSession) SetABI(node [32]byte, contentType *big.Int, data []byte) (*types.Transaction, error) {
	return _PublicResolver.Contract.SetABI(&_PublicResolver.TransactOpts, node, contentType, data)
}

//setaddr是一个付费的mutator事务，绑定合同方法0xD5FA2B00。
//
//solidity:函数setaddr（node bytes32，addr address）返回（）
func (_PublicResolver *PublicResolverTransactor) SetAddr(opts *bind.TransactOpts, node [32]byte, addr common.Address) (*types.Transaction, error) {
	return _PublicResolver.contract.Transact(opts, "setAddr", node, addr)
}

//setaddr是一个付费的mutator事务，绑定合同方法0xD5FA2B00。
//
//solidity:函数setaddr（node bytes32，addr address）返回（）
func (_PublicResolver *PublicResolverSession) SetAddr(node [32]byte, addr common.Address) (*types.Transaction, error) {
	return _PublicResolver.Contract.SetAddr(&_PublicResolver.TransactOpts, node, addr)
}

//setaddr是一个付费的mutator事务，绑定合同方法0xD5FA2B00。
//
//solidity:函数setaddr（node bytes32，addr address）返回（）
func (_PublicResolver *PublicResolverTransactorSession) SetAddr(node [32]byte, addr common.Address) (*types.Transaction, error) {
	return _PublicResolver.Contract.SetAddr(&_PublicResolver.TransactOpts, node, addr)
}

//setContent是一个付费的mutator事务，绑定合同方法0xc3d014d6。
//
//solidity:函数setcontent（node bytes 32，hash bytes 32）返回（）
func (_PublicResolver *PublicResolverTransactor) SetContent(opts *bind.TransactOpts, node [32]byte, hash [32]byte) (*types.Transaction, error) {
	return _PublicResolver.contract.Transact(opts, "setContent", node, hash)
}

//setContent是一个付费的mutator事务，绑定合同方法0xc3d014d6。
//
//solidity:函数setcontent（node bytes 32，hash bytes 32）返回（）
func (_PublicResolver *PublicResolverSession) SetContent(node [32]byte, hash [32]byte) (*types.Transaction, error) {
	return _PublicResolver.Contract.SetContent(&_PublicResolver.TransactOpts, node, hash)
}

//setContent是一个付费的mutator事务，绑定合同方法0xc3d014d6。
//
//solidity:函数setcontent（node bytes 32，hash bytes 32）返回（）
func (_PublicResolver *PublicResolverTransactorSession) SetContent(node [32]byte, hash [32]byte) (*types.Transaction, error) {
	return _PublicResolver.Contract.SetContent(&_PublicResolver.TransactOpts, node, hash)
}

//setname是一个付费的mutator事务，绑定合同方法0x77372213。
//
//solidity:函数setname（node bytes32，name string）返回（）
func (_PublicResolver *PublicResolverTransactor) SetName(opts *bind.TransactOpts, node [32]byte, name string) (*types.Transaction, error) {
	return _PublicResolver.contract.Transact(opts, "setName", node, name)
}

//setname是一个付费的mutator事务，绑定合同方法0x77372213。
//
//solidity:函数setname（node bytes32，name string）返回（）
func (_PublicResolver *PublicResolverSession) SetName(node [32]byte, name string) (*types.Transaction, error) {
	return _PublicResolver.Contract.SetName(&_PublicResolver.TransactOpts, node, name)
}

//setname是一个付费的mutator事务，绑定合同方法0x77372213。
//
//solidity:函数setname（node bytes32，name string）返回（）
func (_PublicResolver *PublicResolverTransactorSession) SetName(node [32]byte, name string) (*types.Transaction, error) {
	return _PublicResolver.Contract.SetName(&_PublicResolver.TransactOpts, node, name)
}

//setpubkey是一个付费的mutator事务，绑定合同方法0x29CD62EA。
//
//solidity:函数setpubkey（node bytes 32，x bytes 32，y bytes 32）返回（）
func (_PublicResolver *PublicResolverTransactor) SetPubkey(opts *bind.TransactOpts, node [32]byte, x [32]byte, y [32]byte) (*types.Transaction, error) {
	return _PublicResolver.contract.Transact(opts, "setPubkey", node, x, y)
}

//setpubkey是一个付费的mutator事务，绑定合同方法0x29CD62EA。
//
//solidity:函数setpubkey（node bytes 32，x bytes 32，y bytes 32）返回（）
func (_PublicResolver *PublicResolverSession) SetPubkey(node [32]byte, x [32]byte, y [32]byte) (*types.Transaction, error) {
	return _PublicResolver.Contract.SetPubkey(&_PublicResolver.TransactOpts, node, x, y)
}

//setpubkey是一个付费的mutator事务，绑定合同方法0x29CD62EA。
//
//solidity:函数setpubkey（node bytes 32，x bytes 32，y bytes 32）返回（）
func (_PublicResolver *PublicResolverTransactorSession) SetPubkey(node [32]byte, x [32]byte, y [32]byte) (*types.Transaction, error) {
	return _PublicResolver.Contract.SetPubkey(&_PublicResolver.TransactOpts, node, x, y)
}

//settext是一个付费的mutator事务，绑定合同方法0x10F13A8C。
//
//solidity:函数settext（node bytes 32，key string，value string）返回（）
func (_PublicResolver *PublicResolverTransactor) SetText(opts *bind.TransactOpts, node [32]byte, key string, value string) (*types.Transaction, error) {
	return _PublicResolver.contract.Transact(opts, "setText", node, key, value)
}

//settext是一个付费的mutator事务，绑定合同方法0x10F13A8C。
//
//solidity:函数settext（node bytes 32，key string，value string）返回（）
func (_PublicResolver *PublicResolverSession) SetText(node [32]byte, key string, value string) (*types.Transaction, error) {
	return _PublicResolver.Contract.SetText(&_PublicResolver.TransactOpts, node, key, value)
}

//settext是一个付费的mutator事务，绑定合同方法0x10F13A8C。
//
//solidity:函数settext（node bytes 32，key string，value string）返回（）
func (_PublicResolver *PublicResolverTransactorSession) SetText(node [32]byte, key string, value string) (*types.Transaction, error) {
	return _PublicResolver.Contract.SetText(&_PublicResolver.TransactOpts, node, key, value)
}

//publicResolvabicChangedEditor从filterabichanged返回，用于对publicResolver协定引发的abichanged事件的原始日志和解包数据进行迭代。
type PublicResolverABIChangedIterator struct {
Event *PublicResolverABIChanged //包含合同细节和原始日志的事件

contract *bind.BoundContract //用于解包事件数据的通用合同
event    string              //用于解包事件数据的事件名称

logs chan types.Log        //日志通道接收找到的合同事件
sub  ethereum.Subscription //错误、完成和终止订阅
done bool                  //订阅是否完成传递日志
fail error                 //停止迭代时出错
}

//next将迭代器前进到后续事件，返回是否存在
//是否找到更多事件。在检索或分析错误的情况下，false是
//返回错误（），可以查询错误（）的确切错误。
func (it *PublicResolverABIChangedIterator) Next() bool {
//如果迭代器失败，请停止迭代
	if it.fail != nil {
		return false
	}
//如果迭代器已完成，则直接传递可用的
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(PublicResolverABIChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
//迭代器仍在进行中，请等待数据或错误事件
	select {
	case log := <-it.logs:
		it.Event = new(PublicResolverABIChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

//重试时出错。筛选过程中出现任何检索或分析错误。
func (it *PublicResolverABIChangedIterator) Error() error {
	return it.fail
}

//关闭终止迭代过程，释放任何挂起的基础
//资源。
func (it *PublicResolverABIChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

//PublicResolverAbiChanged表示PublicResolver协定引发的AbiChanged事件。
type PublicResolverABIChanged struct {
	Node        [32]byte
	ContentType *big.Int
Raw         types.Log //区块链特定的上下文信息
}

//filterabichanged是绑定合同事件0xaa121bbeef5f32f5961a2896e769023910fc9479059ee3495d4c1a696efe3的自由日志检索操作。
//
//solidity:事件abichanged（节点索引字节32，内容类型索引uint256）
func (_PublicResolver *PublicResolverFilterer) FilterABIChanged(opts *bind.FilterOpts, node [][32]byte, contentType []*big.Int) (*PublicResolverABIChangedIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}
	var contentTypeRule []interface{}
	for _, contentTypeItem := range contentType {
		contentTypeRule = append(contentTypeRule, contentTypeItem)
	}

	logs, sub, err := _PublicResolver.contract.FilterLogs(opts, "ABIChanged", nodeRule, contentTypeRule)
	if err != nil {
		return nil, err
	}
	return &PublicResolverABIChangedIterator{contract: _PublicResolver.contract, event: "ABIChanged", logs: logs, sub: sub}, nil
}

//watchabiChanged是一个绑定合同事件0xaa121bbeef5f32f5961a2896e769023910fc9479059ee3495d4c1a696efe3的自由日志订阅操作。
//
//solidity:事件abichanged（节点索引字节32，内容类型索引uint256）
func (_PublicResolver *PublicResolverFilterer) WatchABIChanged(opts *bind.WatchOpts, sink chan<- *PublicResolverABIChanged, node [][32]byte, contentType []*big.Int) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}
	var contentTypeRule []interface{}
	for _, contentTypeItem := range contentType {
		contentTypeRule = append(contentTypeRule, contentTypeItem)
	}

	logs, sub, err := _PublicResolver.contract.WatchLogs(opts, "ABIChanged", nodeRule, contentTypeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
//新日志到达，分析事件并转发给用户
				event := new(PublicResolverABIChanged)
				if err := _PublicResolver.contract.UnpackLog(event, "ABIChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

//publicResolver addrChangediterator从filterAddrChanged返回，用于迭代publicResolver协定引发的addrChanged事件的原始日志和解包数据。
type PublicResolverAddrChangedIterator struct {
Event *PublicResolverAddrChanged //包含合同细节和原始日志的事件

contract *bind.BoundContract //用于解包事件数据的通用合同
event    string              //用于解包事件数据的事件名称

logs chan types.Log        //日志通道接收找到的合同事件
sub  ethereum.Subscription //错误、完成和终止订阅
done bool                  //订阅是否完成传递日志
fail error                 //停止迭代时出错
}

//next将迭代器前进到后续事件，返回是否存在
//是否找到更多事件。在检索或分析错误的情况下，false是
//返回错误（），可以查询错误（）的确切错误。
func (it *PublicResolverAddrChangedIterator) Next() bool {
//如果迭代器失败，请停止迭代
	if it.fail != nil {
		return false
	}
//如果迭代器已完成，则直接传递可用的
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(PublicResolverAddrChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
//迭代器仍在进行中，请等待数据或错误事件
	select {
	case log := <-it.logs:
		it.Event = new(PublicResolverAddrChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

//重试时出错。筛选过程中出现任何检索或分析错误。
func (it *PublicResolverAddrChangedIterator) Error() error {
	return it.fail
}

//关闭终止迭代过程，释放任何挂起的基础
//资源。
func (it *PublicResolverAddrChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

//PublicResolverAddrChanged表示PublicResolver协定引发的AddrChanged事件。
type PublicResolverAddrChanged struct {
	Node [32]byte
	A    common.Address
Raw  types.Log //区块链特定的上下文信息
}

//filterAddrChanged是一个绑定合同事件0x52d7d861f09ab3d26239d492e8968629f95e9e318cf0b73bfddc441522a15fd2的免费日志检索操作。
//
//solidity:事件addrChanged（节点索引字节32，地址）
func (_PublicResolver *PublicResolverFilterer) FilterAddrChanged(opts *bind.FilterOpts, node [][32]byte) (*PublicResolverAddrChangedIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _PublicResolver.contract.FilterLogs(opts, "AddrChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return &PublicResolverAddrChangedIterator{contract: _PublicResolver.contract, event: "AddrChanged", logs: logs, sub: sub}, nil
}

//watchAddrChanged是绑定合同事件0x52d7d861f09ab3d26239d492e8968629f95e9e318cf0b73bfddc441522a15fd2的免费日志订阅操作。
//
//solidity:事件addrChanged（节点索引字节32，地址）
func (_PublicResolver *PublicResolverFilterer) WatchAddrChanged(opts *bind.WatchOpts, sink chan<- *PublicResolverAddrChanged, node [][32]byte) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _PublicResolver.contract.WatchLogs(opts, "AddrChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
//新日志到达，分析事件并转发给用户
				event := new(PublicResolverAddrChanged)
				if err := _PublicResolver.contract.UnpackLog(event, "AddrChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

//publicResolverContentChangedEditor从filterContentChanged返回，用于为publicResolver协定引发的ContentChanged事件迭代原始日志和解包数据。
type PublicResolverContentChangedIterator struct {
Event *PublicResolverContentChanged //包含合同细节和原始日志的事件

contract *bind.BoundContract //用于解包事件数据的通用合同
event    string              //用于解包事件数据的事件名称

logs chan types.Log        //日志通道接收找到的合同事件
sub  ethereum.Subscription //错误、完成和终止订阅
done bool                  //订阅是否完成传递日志
fail error                 //停止迭代时出错
}

//next将迭代器前进到后续事件，返回是否存在
//是否找到更多事件。在检索或分析错误的情况下，false是
//返回错误（），可以查询错误（）的确切错误。
func (it *PublicResolverContentChangedIterator) Next() bool {
//如果迭代器失败，请停止迭代
	if it.fail != nil {
		return false
	}
//如果迭代器已完成，则直接传递可用的
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(PublicResolverContentChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
//迭代器仍在进行中，请等待数据或错误事件
	select {
	case log := <-it.logs:
		it.Event = new(PublicResolverContentChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

//重试时出错。筛选过程中出现任何检索或分析错误。
func (it *PublicResolverContentChangedIterator) Error() error {
	return it.fail
}

//关闭终止迭代过程，释放任何挂起的基础
//资源。
func (it *PublicResolverContentChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

//PublicResolverContentChanged表示PublicResolver协定引发的ContentChanged事件。
type PublicResolverContentChanged struct {
	Node [32]byte
	Hash [32]byte
Raw  types.Log //区块链特定的上下文信息
}

//filterContentChanged是绑定合同事件0x0424B6FE0D9c3bBece07799dc241bb0c22e900be8b6c168b4ee08bd9bf83bc的自由日志检索操作。
//
//solidity:事件内容已更改（节点索引字节32、哈希字节32）
func (_PublicResolver *PublicResolverFilterer) FilterContentChanged(opts *bind.FilterOpts, node [][32]byte) (*PublicResolverContentChangedIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _PublicResolver.contract.FilterLogs(opts, "ContentChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return &PublicResolverContentChangedIterator{contract: _PublicResolver.contract, event: "ContentChanged", logs: logs, sub: sub}, nil
}

//watchContentChanged是绑定合同事件0x0424B6FE0D9c3bBece07799dc241bb0c22e900be8b6c168b4ee08bd9bf83bc的自由日志订阅操作。
//
//solidity:事件内容已更改（节点索引字节32、哈希字节32）
func (_PublicResolver *PublicResolverFilterer) WatchContentChanged(opts *bind.WatchOpts, sink chan<- *PublicResolverContentChanged, node [][32]byte) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _PublicResolver.contract.WatchLogs(opts, "ContentChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
//新日志到达，分析事件并转发给用户
				event := new(PublicResolverContentChanged)
				if err := _PublicResolver.contract.UnpackLog(event, "ContentChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

//publicResolverNameChangedEditor从filterNameChanged返回，用于迭代publicResolver协定引发的名称更改事件的原始日志和解包数据。
type PublicResolverNameChangedIterator struct {
Event *PublicResolverNameChanged //包含合同细节和原始日志的事件

contract *bind.BoundContract //用于解包事件数据的通用合同
event    string              //用于解包事件数据的事件名称

logs chan types.Log        //日志通道接收找到的合同事件
sub  ethereum.Subscription //错误、完成和终止订阅
done bool                  //订阅是否完成传递日志
fail error                 //停止迭代时出错
}

//next将迭代器前进到后续事件，返回是否存在
//是否找到更多事件。在检索或分析错误的情况下，false是
//返回错误（），可以查询错误（）的确切错误。
func (it *PublicResolverNameChangedIterator) Next() bool {
//如果迭代器失败，请停止迭代
	if it.fail != nil {
		return false
	}
//如果迭代器已完成，则直接传递可用的
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(PublicResolverNameChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
//迭代器仍在进行中，请等待数据或错误事件
	select {
	case log := <-it.logs:
		it.Event = new(PublicResolverNameChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

//重试时出错。筛选过程中出现任何检索或分析错误。
func (it *PublicResolverNameChangedIterator) Error() error {
	return it.fail
}

//关闭终止迭代过程，释放任何挂起的基础
//资源。
func (it *PublicResolverNameChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

//PublicResolverNameChanged表示PublicResolver协定引发的NameChanged事件。
type PublicResolverNameChanged struct {
	Node [32]byte
	Name string
Raw  types.Log //区块链特定的上下文信息
}

//filternamechanged是绑定合同事件0xb7d29e911041e8d9b843369e890bcb72c9388692ba48b65ac54e724c4c348f7的自由日志检索操作。
//
//solidity:事件名称已更改（节点索引字节32，名称字符串）
func (_PublicResolver *PublicResolverFilterer) FilterNameChanged(opts *bind.FilterOpts, node [][32]byte) (*PublicResolverNameChangedIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _PublicResolver.contract.FilterLogs(opts, "NameChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return &PublicResolverNameChangedIterator{contract: _PublicResolver.contract, event: "NameChanged", logs: logs, sub: sub}, nil
}

//watchnamechanged是绑定合同事件0xb7d29e911041e8d9b843369e890bcb72c9388692ba48b65ac54e724c4c348f7的免费日志订阅操作。
//
//solidity:事件名称已更改（节点索引字节32，名称字符串）
func (_PublicResolver *PublicResolverFilterer) WatchNameChanged(opts *bind.WatchOpts, sink chan<- *PublicResolverNameChanged, node [][32]byte) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _PublicResolver.contract.WatchLogs(opts, "NameChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
//新日志到达，分析事件并转发给用户
				event := new(PublicResolverNameChanged)
				if err := _PublicResolver.contract.UnpackLog(event, "NameChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

//PublicResolverPubKeyChangedEditor从filterPubKeyChanged返回，用于迭代PublicResolver协定引发的PubKeyChanged事件的原始日志和解包数据。
type PublicResolverPubkeyChangedIterator struct {
Event *PublicResolverPubkeyChanged //包含合同细节和原始日志的事件

contract *bind.BoundContract //用于解包事件数据的通用合同
event    string              //用于解包事件数据的事件名称

logs chan types.Log        //日志通道接收找到的合同事件
sub  ethereum.Subscription //错误、完成和终止订阅
done bool                  //订阅是否完成传递日志
fail error                 //停止迭代时出错
}

//next将迭代器前进到后续事件，返回是否存在
//是否找到更多事件。在检索或分析错误的情况下，false是
//返回错误（），可以查询错误（）的确切错误。
func (it *PublicResolverPubkeyChangedIterator) Next() bool {
//如果迭代器失败，请停止迭代
	if it.fail != nil {
		return false
	}
//如果迭代器已完成，则直接传递可用的
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(PublicResolverPubkeyChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
//迭代器仍在进行中，请等待数据或错误事件
	select {
	case log := <-it.logs:
		it.Event = new(PublicResolverPubkeyChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

//重试时出错。筛选过程中出现任何检索或分析错误。
func (it *PublicResolverPubkeyChangedIterator) Error() error {
	return it.fail
}

//关闭终止迭代过程，释放任何挂起的基础
//资源。
func (it *PublicResolverPubkeyChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

//PublicResolverPubKeyChanged表示PublicResolver协定引发的PubKeyChanged事件。
type PublicResolverPubkeyChanged struct {
	Node [32]byte
	X    [32]byte
	Y    [32]byte
Raw  types.Log //区块链特定的上下文信息
}

//filterPubKeyChanged是一个自由的日志检索操作，绑定合同事件0x1d6f5e03d3f63eb58751986629a5439baee5079ff04f345becb66e23eb154e46。
//
//Solidity:事件PubKeyChanged（节点索引字节32、X字节32、Y字节32）
func (_PublicResolver *PublicResolverFilterer) FilterPubkeyChanged(opts *bind.FilterOpts, node [][32]byte) (*PublicResolverPubkeyChangedIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _PublicResolver.contract.FilterLogs(opts, "PubkeyChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return &PublicResolverPubkeyChangedIterator{contract: _PublicResolver.contract, event: "PubkeyChanged", logs: logs, sub: sub}, nil
}

//watchPubKeyChanged是一个绑定合同事件0x1d6f5e03d3f63eb58751986629a5439baee5079ff04f345becb66e23eb154e46的免费日志订阅操作。
//
//Solidity:事件PubKeyChanged（节点索引字节32、X字节32、Y字节32）
func (_PublicResolver *PublicResolverFilterer) WatchPubkeyChanged(opts *bind.WatchOpts, sink chan<- *PublicResolverPubkeyChanged, node [][32]byte) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _PublicResolver.contract.WatchLogs(opts, "PubkeyChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
//新日志到达，分析事件并转发给用户
				event := new(PublicResolverPubkeyChanged)
				if err := _PublicResolver.contract.UnpackLog(event, "PubkeyChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

//publicResolverTextChangedEditor从filterTextChanged返回，用于为publicResolver协定引发的textChanged事件迭代原始日志和解包数据。
type PublicResolverTextChangedIterator struct {
Event *PublicResolverTextChanged //包含合同细节和原始日志的事件

contract *bind.BoundContract //用于解包事件数据的通用合同
event    string              //用于解包事件数据的事件名称

logs chan types.Log        //日志通道接收找到的合同事件
sub  ethereum.Subscription //错误、完成和终止订阅
done bool                  //订阅是否完成传递日志
fail error                 //停止迭代时出错
}

//next将迭代器前进到后续事件，返回是否存在
//是否找到更多事件。在检索或分析错误的情况下，false是
//返回错误（），可以查询错误（）的确切错误。
func (it *PublicResolverTextChangedIterator) Next() bool {
//如果迭代器失败，请停止迭代
	if it.fail != nil {
		return false
	}
//如果迭代器已完成，则直接传递可用的
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(PublicResolverTextChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
//迭代器仍在进行中，请等待数据或错误事件
	select {
	case log := <-it.logs:
		it.Event = new(PublicResolverTextChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

//重试时出错。筛选过程中出现任何检索或分析错误。
func (it *PublicResolverTextChangedIterator) Error() error {
	return it.fail
}

//关闭终止迭代过程，释放任何挂起的基础
//资源。
func (it *PublicResolverTextChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

//PublicResolverTextChanged表示PublicResolver协定引发的TextChanged事件。
type PublicResolverTextChanged struct {
	Node       [32]byte
	IndexedKey common.Hash
	Key        string
Raw        types.Log //区块链特定的上下文信息
}

//filtertextchanged是绑定合同事件0xD8C9334B1A9C2F9DA342A2A232629C1A229B6445DAD78947F674B44444A7550的自由日志检索操作。
//
//solidity:事件文本已更改（节点索引字节32、索引键索引字符串、键字符串）
func (_PublicResolver *PublicResolverFilterer) FilterTextChanged(opts *bind.FilterOpts, node [][32]byte, indexedKey []string) (*PublicResolverTextChangedIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}
	var indexedKeyRule []interface{}
	for _, indexedKeyItem := range indexedKey {
		indexedKeyRule = append(indexedKeyRule, indexedKeyItem)
	}

	logs, sub, err := _PublicResolver.contract.FilterLogs(opts, "TextChanged", nodeRule, indexedKeyRule)
	if err != nil {
		return nil, err
	}
	return &PublicResolverTextChangedIterator{contract: _PublicResolver.contract, event: "TextChanged", logs: logs, sub: sub}, nil
}

//watchTextChanged是一个绑定合同事件0xD8C9334B1A9C2F9DA342A2A232629C1A229B6445DAD78947F674B44444A7550的免费日志订阅操作。
//
//solidity:事件文本已更改（节点索引字节32、索引键索引字符串、键字符串）
func (_PublicResolver *PublicResolverFilterer) WatchTextChanged(opts *bind.WatchOpts, sink chan<- *PublicResolverTextChanged, node [][32]byte, indexedKey []string) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}
	var indexedKeyRule []interface{}
	for _, indexedKeyItem := range indexedKey {
		indexedKeyRule = append(indexedKeyRule, indexedKeyItem)
	}

	logs, sub, err := _PublicResolver.contract.WatchLogs(opts, "TextChanged", nodeRule, indexedKeyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
//新日志到达，分析事件并转发给用户
				event := new(PublicResolverTextChanged)
				if err := _PublicResolver.contract.UnpackLog(event, "TextChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}
