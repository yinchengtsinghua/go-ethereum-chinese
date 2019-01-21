
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

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

//ensabi是用于从中生成绑定的输入abi。
const ENSABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"}],\"name\":\"resolver\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"}],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"label\",\"type\":\"bytes32\"},{\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"setSubnodeOwner\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"ttl\",\"type\":\"uint64\"}],\"name\":\"setTTL\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"}],\"name\":\"ttl\",\"outputs\":[{\"name\":\"\",\"type\":\"uint64\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"resolver\",\"type\":\"address\"}],\"name\":\"setResolver\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"setOwner\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":true,\"name\":\"label\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"NewOwner\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"Transfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"resolver\",\"type\":\"address\"}],\"name\":\"NewResolver\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"ttl\",\"type\":\"uint64\"}],\"name\":\"NewTTL\",\"type\":\"event\"}]"

//ensbin是用于部署新合同的编译字节码。
const ENSBin = `0x6060604052341561000f57600080fd5b60008080526020527fad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb58054600160a060020a033316600160a060020a0319909116179055610503806100626000396000f3006060604052600436106100825763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416630178b8bf811461008757806302571be3146100b957806306ab5923146100cf57806314ab9038146100f657806316a25cbd146101195780631896f70a1461014c5780635b0fc9c31461016e575b600080fd5b341561009257600080fd5b61009d600435610190565b604051600160a060020a03909116815260200160405180910390f35b34156100c457600080fd5b61009d6004356101ae565b34156100da57600080fd5b6100f4600435602435600160a060020a03604435166101c9565b005b341561010157600080fd5b6100f460043567ffffffffffffffff6024351661028b565b341561012457600080fd5b61012f600435610357565b60405167ffffffffffffffff909116815260200160405180910390f35b341561015757600080fd5b6100f4600435600160a060020a036024351661038e565b341561017957600080fd5b6100f4600435600160a060020a0360243516610434565b600090815260208190526040902060010154600160a060020a031690565b600090815260208190526040902054600160a060020a031690565b600083815260208190526040812054849033600160a060020a039081169116146101f257600080fd5b8484604051918252602082015260409081019051908190039020915083857fce0457fe73731f824cc272376169235128c118b49d344817417c6d108d155e8285604051600160a060020a03909116815260200160405180910390a3506000908152602081905260409020805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a03929092169190911790555050565b600082815260208190526040902054829033600160a060020a039081169116146102b457600080fd5b827f1d4f9bbfc9cab89d66e1a1562f2233ccbf1308cb4f63de2ead5787adddb8fa688360405167ffffffffffffffff909116815260200160405180910390a250600091825260208290526040909120600101805467ffffffffffffffff90921674010000000000000000000000000000000000000000027fffffffff0000000000000000ffffffffffffffffffffffffffffffffffffffff909216919091179055565b60009081526020819052604090206001015474010000000000000000000000000000000000000000900467ffffffffffffffff1690565b600082815260208190526040902054829033600160a060020a039081169116146103b757600080fd5b827f335721b01866dc23fbee8b6b2c7b1e14d6f05c28cd35a2c934239f94095602a083604051600160a060020a03909116815260200160405180910390a250600091825260208290526040909120600101805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a03909216919091179055565b600082815260208190526040902054829033600160a060020a0390811691161461045d57600080fd5b827fd4735d920b0f87494915f556dd9b54c8f309026070caea5c737245152564d26683604051600160a060020a03909116815260200160405180910390a250600091825260208290526040909120805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a039092169190911790555600a165627a7a72305820f4c798d4c84c9912f389f64631e85e8d16c3e6644f8c2e1579936015c7d5f6660029`

//Deployeens部署一个新的以太坊契约，将ENS实例绑定到它。
func DeployENS(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *ENS, error) {
	parsed, err := abi.JSON(strings.NewReader(ENSABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(ENSBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &ENS{ENSCaller: ENSCaller{contract: contract}, ENSTransactor: ENSTransactor{contract: contract}, ENSFilterer: ENSFilterer{contract: contract}}, nil
}

//ENS是围绕以太坊合同自动生成的Go绑定。
type ENS struct {
ENSCaller     //对合同具有只读约束力
ENSTransactor //只写对合同有约束力
ENSFilterer   //合同事件的日志筛选程序
}

//EnsCaller是围绕以太坊契约自动生成的只读Go绑定。
type ENSCaller struct {
contract *bind.BoundContract //用于低级调用的通用协定包装器
}

//EnsTransactor是围绕以太坊合同自动生成的只写Go绑定。
type ENSTransactor struct {
contract *bind.BoundContract //用于低级调用的通用协定包装器
}

//EnsFilter是围绕以太坊合同事件自动生成的日志筛选Go绑定。
type ENSFilterer struct {
contract *bind.BoundContract //用于低级调用的通用协定包装器
}

//EnSession是围绕以太坊合同自动生成的Go绑定，
//具有预设的调用和事务处理选项。
type ENSSession struct {
Contract     *ENS              //为其设置会话的通用约定绑定
CallOpts     bind.CallOpts     //在整个会话中使用的调用选项
TransactOpts bind.TransactOpts //要在此会话中使用的事务验证选项
}

//EnCallersession是围绕以太坊合约自动生成的只读Go绑定，
//带预设通话选项。
type ENSCallerSession struct {
Contract *ENSCaller    //用于设置会话的通用协定调用方绑定
CallOpts bind.CallOpts //在整个会话中使用的调用选项
}

//enstransactiorsession是围绕以太坊合同自动生成的只写Go绑定，
//具有预设的Transact选项。
type ENSTransactorSession struct {
Contract     *ENSTransactor    //用于设置会话的通用合同事务处理程序绑定
TransactOpts bind.TransactOpts //要在此会话中使用的事务验证选项
}

//Ensraw是围绕以太坊合同自动生成的低级Go绑定。
type ENSRaw struct {
Contract *ENS //用于访问上的原始方法的通用合同绑定
}

//EnsCallerraw是围绕以太坊合约自动生成的低级只读Go绑定。
type ENSCallerRaw struct {
Contract *ENSCaller //
}

//
type ENSTransactorRaw struct {
Contract *ENSTransactor //用于访问上的原始方法的通用只写协定绑定
}

//new ens创建一个新的ens实例，绑定到特定的已部署契约。
func NewENS(address common.Address, backend bind.ContractBackend) (*ENS, error) {
	contract, err := bindENS(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ENS{ENSCaller: ENSCaller{contract: contract}, ENSTransactor: ENSTransactor{contract: contract}, ENSFilterer: ENSFilterer{contract: contract}}, nil
}

//newenscaller创建一个新的ENS只读实例，绑定到特定的已部署契约。
func NewENSCaller(address common.Address, caller bind.ContractCaller) (*ENSCaller, error) {
	contract, err := bindENS(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ENSCaller{contract: contract}, nil
}

//
func NewENSTransactor(address common.Address, transactor bind.ContractTransactor) (*ENSTransactor, error) {
	contract, err := bindENS(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ENSTransactor{contract: contract}, nil
}

//newensfilter创建一个新的ens日志筛选器实例，绑定到特定的已部署协定。
func NewENSFilterer(address common.Address, filterer bind.ContractFilterer) (*ENSFilterer, error) {
	contract, err := bindENS(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ENSFilterer{contract: contract}, nil
}

//bindens将通用包装绑定到已部署的协定。
func bindENS(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(ENSABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

//调用调用（常量）contract方法，参数作为输入值，并且
//将输出设置为结果。结果类型可能是用于
//返回、匿名返回的接口切片和命名的结构
//返回。
func (_ENS *ENSRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _ENS.Contract.ENSCaller.contract.Call(opts, result, method, params...)
}

//转账启动普通交易以将资金转移到合同，调用
//它的默认方法（如果有）。
func (_ENS *ENSRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ENS.Contract.ENSTransactor.contract.Transfer(opts)
}

//Transact使用参数作为输入值调用（付费）Contract方法。
func (_ENS *ENSRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ENS.Contract.ENSTransactor.contract.Transact(opts, method, params...)
}

//调用调用（常量）contract方法，参数作为输入值，并且
//将输出设置为结果。结果类型可能是用于
//返回、匿名返回的接口切片和命名的结构
//返回。
func (_ENS *ENSCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _ENS.Contract.contract.Call(opts, result, method, params...)
}

//转账启动普通交易以将资金转移到合同，调用
//它的默认方法（如果有）。
func (_ENS *ENSTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ENS.Contract.contract.Transfer(opts)
}

//Transact使用参数作为输入值调用（付费）Contract方法。
func (_ENS *ENSTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ENS.Contract.contract.Transact(opts, method, params...)
}

//owner是一个绑定契约方法0x02571BE3的免费数据检索调用。
//
//solidity:函数所有者（节点字节32）常量返回（地址）
func (_ENS *ENSCaller) Owner(opts *bind.CallOpts, node [32]byte) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _ENS.contract.Call(opts, out, "owner", node)
	return *ret0, err
}

//owner是一个绑定契约方法0x02571BE3的免费数据检索调用。
//
//solidity:函数所有者（节点字节32）常量返回（地址）
func (_ENS *ENSSession) Owner(node [32]byte) (common.Address, error) {
	return _ENS.Contract.Owner(&_ENS.CallOpts, node)
}

//owner是一个绑定契约方法0x02571BE3的免费数据检索调用。
//
//solidity:函数所有者（节点字节32）常量返回（地址）
func (_ENS *ENSCallerSession) Owner(node [32]byte) (common.Address, error) {
	return _ENS.Contract.Owner(&_ENS.CallOpts, node)
}

//解析器是一个自由的数据检索调用，绑定契约方法0x0178B8bf。
//
//solidity:函数解析程序（节点字节32）常量返回（地址）
func (_ENS *ENSCaller) Resolver(opts *bind.CallOpts, node [32]byte) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _ENS.contract.Call(opts, out, "resolver", node)
	return *ret0, err
}

//解析器是一个自由的数据检索调用，绑定契约方法0x0178B8bf。
//
//solidity:函数解析程序（节点字节32）常量返回（地址）
func (_ENS *ENSSession) Resolver(node [32]byte) (common.Address, error) {
	return _ENS.Contract.Resolver(&_ENS.CallOpts, node)
}

//解析器是一个自由的数据检索调用，绑定契约方法0x0178B8bf。
//
//solidity:函数解析程序（节点字节32）常量返回（地址）
func (_ENS *ENSCallerSession) Resolver(node [32]byte) (common.Address, error) {
	return _ENS.Contract.Resolver(&_ENS.CallOpts, node)
}

//TTL是一个绑定契约方法0x16A25CBD的免费数据检索调用。
//
//solidity:函数ttl（节点字节32）常量返回（uint64）
func (_ENS *ENSCaller) Ttl(opts *bind.CallOpts, node [32]byte) (uint64, error) {
	var (
		ret0 = new(uint64)
	)
	out := ret0
	err := _ENS.contract.Call(opts, out, "ttl", node)
	return *ret0, err
}

//TTL是一个绑定契约方法0x16A25CBD的免费数据检索调用。
//
//solidity:函数ttl（节点字节32）常量返回（uint64）
func (_ENS *ENSSession) Ttl(node [32]byte) (uint64, error) {
	return _ENS.Contract.Ttl(&_ENS.CallOpts, node)
}

//TTL是一个绑定契约方法0x16A25CBD的免费数据检索调用。
//
//solidity:函数ttl（节点字节32）常量返回（uint64）
func (_ENS *ENSCallerSession) Ttl(node [32]byte) (uint64, error) {
	return _ENS.Contract.Ttl(&_ENS.CallOpts, node)
}

//setowner是一个受合同方法0x5b0fc9c3约束的付费mutator事务。
//
//solidity:函数setowner（node bytes32，owner address）返回（）
func (_ENS *ENSTransactor) SetOwner(opts *bind.TransactOpts, node [32]byte, owner common.Address) (*types.Transaction, error) {
	return _ENS.contract.Transact(opts, "setOwner", node, owner)
}

//setowner是一个受合同方法0x5b0fc9c3约束的付费mutator事务。
//
//solidity:函数setowner（node bytes32，owner address）返回（）
func (_ENS *ENSSession) SetOwner(node [32]byte, owner common.Address) (*types.Transaction, error) {
	return _ENS.Contract.SetOwner(&_ENS.TransactOpts, node, owner)
}

//setowner是一个受合同方法0x5b0fc9c3约束的付费mutator事务。
//
//solidity:函数setowner（node bytes32，owner address）返回（）
func (_ENS *ENSTransactorSession) SetOwner(node [32]byte, owner common.Address) (*types.Transaction, error) {
	return _ENS.Contract.SetOwner(&_ENS.TransactOpts, node, owner)
}

//setresolver是一个受契约方法0x1896F70A约束的付费转换器事务。
//
//solidity:函数setresolver（node bytes32，resolver address）返回（）
func (_ENS *ENSTransactor) SetResolver(opts *bind.TransactOpts, node [32]byte, resolver common.Address) (*types.Transaction, error) {
	return _ENS.contract.Transact(opts, "setResolver", node, resolver)
}

//setresolver是一个受契约方法0x1896F70A约束的付费转换器事务。
//
//solidity:函数setresolver（node bytes32，resolver address）返回（）
func (_ENS *ENSSession) SetResolver(node [32]byte, resolver common.Address) (*types.Transaction, error) {
	return _ENS.Contract.SetResolver(&_ENS.TransactOpts, node, resolver)
}

//setresolver是一个受契约方法0x1896F70A约束的付费转换器事务。
//
//solidity:函数setresolver（node bytes32，resolver address）返回（）
func (_ENS *ENSTransactorSession) SetResolver(node [32]byte, resolver common.Address) (*types.Transaction, error) {
	return _ENS.Contract.SetResolver(&_ENS.TransactOpts, node, resolver)
}

//setSubnodeOwner是一个付费的mutator事务，绑定合同方法0x06AB5923。
//
//solidity:函数setSubNodeOwner（node bytes32，label bytes32，owner address）返回（）
func (_ENS *ENSTransactor) SetSubnodeOwner(opts *bind.TransactOpts, node [32]byte, label [32]byte, owner common.Address) (*types.Transaction, error) {
	return _ENS.contract.Transact(opts, "setSubnodeOwner", node, label, owner)
}

//setSubnodeOwner是一个付费的mutator事务，绑定合同方法0x06AB5923。
//
//solidity:函数setSubNodeOwner（node bytes32，label bytes32，owner address）返回（）
func (_ENS *ENSSession) SetSubnodeOwner(node [32]byte, label [32]byte, owner common.Address) (*types.Transaction, error) {
	return _ENS.Contract.SetSubnodeOwner(&_ENS.TransactOpts, node, label, owner)
}

//setSubnodeOwner是一个付费的mutator事务，绑定合同方法0x06AB5923。
//
//solidity:函数setSubNodeOwner（node bytes32，label bytes32，owner address）返回（）
func (_ENS *ENSTransactorSession) SetSubnodeOwner(node [32]byte, label [32]byte, owner common.Address) (*types.Transaction, error) {
	return _ENS.Contract.SetSubnodeOwner(&_ENS.TransactOpts, node, label, owner)
}

//settl是一个受合同方法0x14AB9038约束的付费的转换程序事务。
//
//solidity:函数settl（节点字节32，ttl uint64）返回（）
func (_ENS *ENSTransactor) SetTTL(opts *bind.TransactOpts, node [32]byte, ttl uint64) (*types.Transaction, error) {
	return _ENS.contract.Transact(opts, "setTTL", node, ttl)
}

//settl是一个受合同方法0x14AB9038约束的付费的转换程序事务。
//
//solidity:函数settl（节点字节32，ttl uint64）返回（）
func (_ENS *ENSSession) SetTTL(node [32]byte, ttl uint64) (*types.Transaction, error) {
	return _ENS.Contract.SetTTL(&_ENS.TransactOpts, node, ttl)
}

//settl是一个受合同方法0x14AB9038约束的付费的转换程序事务。
//
//solidity:函数settl（节点字节32，ttl uint64）返回（）
func (_ENS *ENSTransactorSession) SetTTL(node [32]byte, ttl uint64) (*types.Transaction, error) {
	return _ENS.Contract.SetTTL(&_ENS.TransactOpts, node, ttl)
}

//EnsNewOwnerIterator从filterNewOwner返回，用于迭代ENS合同引发的NewOwner事件的原始日志和解包数据。
type ENSNewOwnerIterator struct {
Event *ENSNewOwner //包含合同细节和原始日志的事件

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
func (it *ENSNewOwnerIterator) Next() bool {
//如果迭代器失败，请停止迭代
	if it.fail != nil {
		return false
	}
//如果迭代器已完成，则直接传递可用的
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ENSNewOwner)
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
		it.Event = new(ENSNewOwner)
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
func (it *ENSNewOwnerIterator) Error() error {
	return it.fail
}

//关闭终止迭代过程，释放任何挂起的基础
//资源。
func (it *ENSNewOwnerIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

//EnsNewOwner表示ENS合同引发的NewOwner事件。
type ENSNewOwner struct {
	Node  [32]byte
	Label [32]byte
	Owner common.Address
Raw   types.Log //区块链特定的上下文信息
}

//FieldNeWOrthor是一个绑定日志事件的免费日志检索操作，绑定了合同事件0xCE047FE737、F824cc26316695128128C118B49 D34 81717417C6D108D155E82.
//
//solidity:事件newowner（节点索引字节32，标签索引字节32，所有者地址）
func (_ENS *ENSFilterer) FilterNewOwner(opts *bind.FilterOpts, node [][32]byte, label [][32]byte) (*ENSNewOwnerIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}
	var labelRule []interface{}
	for _, labelItem := range label {
		labelRule = append(labelRule, labelItem)
	}

	logs, sub, err := _ENS.contract.FilterLogs(opts, "NewOwner", nodeRule, labelRule)
	if err != nil {
		return nil, err
	}
	return &ENSNewOwnerIterator{contract: _ENS.contract, event: "NewOwner", logs: logs, sub: sub}, nil
}

//WatchNewOwner是一个绑定合同事件0xCE0457FE73731F824CC27236169235128C118B49D344817417C6D108D155E82的免费日志订阅操作。
//
//solidity:事件newowner（节点索引字节32，标签索引字节32，所有者地址）
func (_ENS *ENSFilterer) WatchNewOwner(opts *bind.WatchOpts, sink chan<- *ENSNewOwner, node [][32]byte, label [][32]byte) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}
	var labelRule []interface{}
	for _, labelItem := range label {
		labelRule = append(labelRule, labelItem)
	}

	logs, sub, err := _ENS.contract.WatchLogs(opts, "NewOwner", nodeRule, labelRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
//新日志到达，分析事件并转发给用户
				event := new(ENSNewOwner)
				if err := _ENS.contract.UnpackLog(event, "NewOwner", log); err != nil {
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

//
type ENSNewResolverIterator struct {
Event *ENSNewResolver //包含合同细节和原始日志的事件

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
func (it *ENSNewResolverIterator) Next() bool {
//如果迭代器失败，请停止迭代
	if it.fail != nil {
		return false
	}
//如果迭代器已完成，则直接传递可用的
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ENSNewResolver)
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
		it.Event = new(ENSNewResolver)
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
func (it *ENSNewResolverIterator) Error() error {
	return it.fail
}

//关闭终止迭代过程，释放任何挂起的基础
//资源。
func (it *ENSNewResolverIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

//ens newresolver表示由ens协定引发的newresolver事件。
type ENSNewResolver struct {
	Node     [32]byte
	Resolver common.Address
Raw      types.Log //区块链特定的上下文信息
}

//filternewresolver是一个自由的日志检索操作，绑定合同事件0x335721b01866dc23fbee8B6b2c7b1e14d6f05c28cd35a2c934239f94095602a0。
//
//solidity:事件newresolver（节点索引字节32，解析程序地址）
func (_ENS *ENSFilterer) FilterNewResolver(opts *bind.FilterOpts, node [][32]byte) (*ENSNewResolverIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _ENS.contract.FilterLogs(opts, "NewResolver", nodeRule)
	if err != nil {
		return nil, err
	}
	return &ENSNewResolverIterator{contract: _ENS.contract, event: "NewResolver", logs: logs, sub: sub}, nil
}

//WatchNewResolver是一个绑定合同事件0x335721b01866dc23fbee8B6b2c7b1e14d6f05c28cd35a2c934239f94095602a0的免费日志订阅操作。
//
//solidity:事件newresolver（节点索引字节32，解析程序地址）
func (_ENS *ENSFilterer) WatchNewResolver(opts *bind.WatchOpts, sink chan<- *ENSNewResolver, node [][32]byte) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _ENS.contract.WatchLogs(opts, "NewResolver", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
//新日志到达，分析事件并转发给用户
				event := new(ENSNewResolver)
				if err := _ENS.contract.UnpackLog(event, "NewResolver", log); err != nil {
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

//ensnewttliterator从filternewttl返回，用于迭代ens协定引发的newttl事件的原始日志和解包数据。
type ENSNewTTLIterator struct {
Event *ENSNewTTL //包含合同细节和原始日志的事件

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
func (it *ENSNewTTLIterator) Next() bool {
//如果迭代器失败，请停止迭代
	if it.fail != nil {
		return false
	}
//如果迭代器已完成，则直接传递可用的
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ENSNewTTL)
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
		it.Event = new(ENSNewTTL)
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
func (it *ENSNewTTLIterator) Error() error {
	return it.fail
}

//关闭终止迭代过程，释放任何挂起的基础
//资源。
func (it *ENSNewTTLIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

//ens newttl表示由ens协定引发的newttl事件。
type ENSNewTTL struct {
	Node [32]byte
	Ttl  uint64
Raw  types.Log //区块链特定的上下文信息
}

//filternewttl是一个自由的日志检索操作，绑定合同事件0x1d4f9bbfc9cab89d66e1a1562f2233ccbf1308cb4f63de2ead5787adddb8fa68。
//
//solidity:事件newttl（节点索引字节32，ttl uint64）
func (_ENS *ENSFilterer) FilterNewTTL(opts *bind.FilterOpts, node [][32]byte) (*ENSNewTTLIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _ENS.contract.FilterLogs(opts, "NewTTL", nodeRule)
	if err != nil {
		return nil, err
	}
	return &ENSNewTTLIterator{contract: _ENS.contract, event: "NewTTL", logs: logs, sub: sub}, nil
}

//watchNewTTL是一个绑定合同事件0x1D4F9BFC9CAB89D66E1A1562F223CCBF308CB4F63DE2EAD5787ADDDB8FA68的免费日志订阅操作。
//
//solidity:事件newttl（节点索引字节32，ttl uint64）
func (_ENS *ENSFilterer) WatchNewTTL(opts *bind.WatchOpts, sink chan<- *ENSNewTTL, node [][32]byte) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _ENS.contract.WatchLogs(opts, "NewTTL", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
//新日志到达，分析事件并转发给用户
				event := new(ENSNewTTL)
				if err := _ENS.contract.UnpackLog(event, "NewTTL", log); err != nil {
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

//enstransferriterator从filtertransfer返回，用于迭代ens协定引发的传输事件的原始日志和解包数据。
type ENSTransferIterator struct {
Event *ENSTransfer //包含合同细节和原始日志的事件

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
func (it *ENSTransferIterator) Next() bool {
//如果迭代器失败，请停止迭代
	if it.fail != nil {
		return false
	}
//如果迭代器已完成，则直接传递可用的
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ENSTransfer)
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
		it.Event = new(ENSTransfer)
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
func (it *ENSTransferIterator) Error() error {
	return it.fail
}

//关闭终止迭代过程，释放任何挂起的基础
//资源。
func (it *ENSTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

//EnsTransfer表示ENS合同引发的转让事件。
type ENSTransfer struct {
	Node  [32]byte
	Owner common.Address
Raw   types.Log //区块链特定的上下文信息
}

//filtertransfer是一个自由的日志检索操作，绑定合同事件0xd4735d920bf87494915f556dd9b54c8f309026070caeaa5c737245152564d266。
//
//solidity:事件传输（节点索引字节32，所有者地址）
func (_ENS *ENSFilterer) FilterTransfer(opts *bind.FilterOpts, node [][32]byte) (*ENSTransferIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _ENS.contract.FilterLogs(opts, "Transfer", nodeRule)
	if err != nil {
		return nil, err
	}
	return &ENSTransferIterator{contract: _ENS.contract, event: "Transfer", logs: logs, sub: sub}, nil
}

//WatchTransfer是一个免费的日志订阅操作，绑定合同事件0xd4735d920bf87494915f556dd9b54c8f309026070caeaa5c737245152564d266。
//
//solidity:事件传输（节点索引字节32，所有者地址）
func (_ENS *ENSFilterer) WatchTransfer(opts *bind.WatchOpts, sink chan<- *ENSTransfer, node [][32]byte) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _ENS.contract.WatchLogs(opts, "Transfer", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
//新日志到达，分析事件并转发给用户
				event := new(ENSTransfer)
				if err := _ENS.contract.UnpackLog(event, "Transfer", log); err != nil {
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
