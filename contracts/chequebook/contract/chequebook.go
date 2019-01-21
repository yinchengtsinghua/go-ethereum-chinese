
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

//checkbookabi是用于从中生成绑定的输入abi。
const ChequebookABI = "[{\"constant\":false,\"inputs\":[],\"name\":\"kill\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"sent\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"beneficiary\",\"type\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\"},{\"name\":\"sig_v\",\"type\":\"uint8\"},{\"name\":\"sig_r\",\"type\":\"bytes32\"},{\"name\":\"sig_s\",\"type\":\"bytes32\"}],\"name\":\"cash\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"fallback\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"deadbeat\",\"type\":\"address\"}],\"name\":\"Overdraft\",\"type\":\"event\"}]"

//checkbookbin是用于部署新合同的编译字节码。
const ChequebookBin = `0x606060405260008054600160a060020a033316600160a060020a03199091161790556102ec806100306000396000f3006060604052600436106100565763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166341c0e1b581146100585780637bf786f81461006b578063fbf788d61461009c575b005b341561006357600080fd5b6100566100ca565b341561007657600080fd5b61008a600160a060020a03600435166100f1565b60405190815260200160405180910390f35b34156100a757600080fd5b610056600160a060020a036004351660243560ff60443516606435608435610103565b60005433600160a060020a03908116911614156100ef57600054600160a060020a0316ff5b565b60016020526000908152604090205481565b600160a060020a0385166000908152600160205260408120548190861161012957600080fd5b3087876040516c01000000000000000000000000600160a060020a03948516810282529290931690910260148301526028820152604801604051809103902091506001828686866040516000815260200160405260006040516020015260405193845260ff90921660208085019190915260408085019290925260608401929092526080909201915160208103908084039060008661646e5a03f115156101cf57600080fd5b505060206040510351600054600160a060020a039081169116146101f257600080fd5b50600160a060020a03808716600090815260016020526040902054860390301631811161026257600160a060020a0387166000818152600160205260409081902088905582156108fc0290839051600060405180830381858888f19350505050151561025d57600080fd5b6102b7565b6000547f2250e2993c15843b32621c89447cc589ee7a9f049c026986e545d3c2c0c6f97890600160a060020a0316604051600160a060020a03909116815260200160405180910390a186600160a060020a0316ff5b505050505050505600a165627a7a72305820533e856fc37e3d64d1706bcc7dfb6b1d490c8d566ea498d9d01ec08965a896ca0029`

//Deploychequebook部署新的以太坊合同，将支票簿的实例绑定到该合同。
func DeployChequebook(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Chequebook, error) {
	parsed, err := abi.JSON(strings.NewReader(ChequebookABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(ChequebookBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Chequebook{ChequebookCaller: ChequebookCaller{contract: contract}, ChequebookTransactor: ChequebookTransactor{contract: contract}, ChequebookFilterer: ChequebookFilterer{contract: contract}}, nil
}

//支票簿是围绕以太坊合同自动生成的Go绑定。
type Chequebook struct {
ChequebookCaller     //对合同具有只读约束力
ChequebookTransactor //只写对合同有约束力
ChequebookFilterer   //合同事件的日志筛选程序
}

//支票簿调用者是围绕以太坊合同自动生成的只读Go绑定。
type ChequebookCaller struct {
contract *bind.BoundContract //用于低级调用的通用协定包装器
}

//支票簿交易是围绕以太坊合同自动生成的只写即用绑定。
type ChequebookTransactor struct {
contract *bind.BoundContract //用于低级调用的通用协定包装器
}

//支票簿筛选器是围绕以太坊合同事件自动生成的日志筛选Go绑定。
type ChequebookFilterer struct {
contract *bind.BoundContract //用于低级调用的通用协定包装器
}

//支票簿会话是围绕以太坊合同自动生成的Go绑定，
//具有预设的调用和事务处理选项。
type ChequebookSession struct {
Contract     *Chequebook       //为其设置会话的通用约定绑定
CallOpts     bind.CallOpts     //在整个会话中使用的调用选项
TransactOpts bind.TransactOpts //要在此会话中使用的事务验证选项
}

//CheQueBooCoprError会话是一个围绕Ethunm合同的自动生成只读GO绑定，
//带预设通话选项。
type ChequebookCallerSession struct {
Contract *ChequebookCaller //用于设置会话的通用协定调用方绑定
CallOpts bind.CallOpts     //在整个会话中使用的调用选项
}

//支票簿事务会话是围绕以太坊合同自动生成的只写即用绑定，
//具有预设的Transact选项。
type ChequebookTransactorSession struct {
Contract     *ChequebookTransactor //用于设置会话的通用合同事务处理程序绑定
TransactOpts bind.TransactOpts     //要在此会话中使用的事务验证选项
}

//ChequebookRaw是围绕以太坊合同自动生成的低级Go绑定。
type ChequebookRaw struct {
Contract *Chequebook //用于访问上的原始方法的通用合同绑定
}

//支票簿Callerraw是围绕以太坊合同自动生成的低级只读Go绑定。
type ChequebookCallerRaw struct {
Contract *ChequebookCaller //用于访问上的原始方法的通用只读协定绑定
}

//支票簿Transactorraw是围绕以太坊合同自动生成的低级只写即用绑定。
type ChequebookTransactorRaw struct {
Contract *ChequebookTransactor //用于访问上的原始方法的通用只写协定绑定
}

//newcheckbook创建一个新的checkbook实例，绑定到特定的已部署合同。
func NewChequebook(address common.Address, backend bind.ContractBackend) (*Chequebook, error) {
	contract, err := bindChequebook(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Chequebook{ChequebookCaller: ChequebookCaller{contract: contract}, ChequebookTransactor: ChequebookTransactor{contract: contract}, ChequebookFilterer: ChequebookFilterer{contract: contract}}, nil
}

//newcheckbookcaller创建一个新的支票簿只读实例，绑定到特定的已部署合同。
func NewChequebookCaller(address common.Address, caller bind.ContractCaller) (*ChequebookCaller, error) {
	contract, err := bindChequebook(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ChequebookCaller{contract: contract}, nil
}

//newcheckbooktransaction创建一个新的支票簿的只写实例，绑定到特定的已部署合同。
func NewChequebookTransactor(address common.Address, transactor bind.ContractTransactor) (*ChequebookTransactor, error) {
	contract, err := bindChequebook(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ChequebookTransactor{contract: contract}, nil
}

//newcheckbookfilter创建一个新的checkbook日志过滤器实例，绑定到一个特定的已部署合同。
func NewChequebookFilterer(address common.Address, filterer bind.ContractFilterer) (*ChequebookFilterer, error) {
	contract, err := bindChequebook(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ChequebookFilterer{contract: contract}, nil
}

//bindcheckbook将通用包装绑定到已部署的协定。
func bindChequebook(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(ChequebookABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

//调用调用（常量）contract方法，参数作为输入值，并且
//将输出设置为结果。结果类型可能是用于
//返回、匿名返回的接口切片和命名的结构
//返回。
func (_Chequebook *ChequebookRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Chequebook.Contract.ChequebookCaller.contract.Call(opts, result, method, params...)
}

//转账启动普通交易以将资金转移到合同，调用
//它的默认方法（如果有）。
func (_Chequebook *ChequebookRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Chequebook.Contract.ChequebookTransactor.contract.Transfer(opts)
}

//Transact使用参数作为输入值调用（付费）Contract方法。
func (_Chequebook *ChequebookRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Chequebook.Contract.ChequebookTransactor.contract.Transact(opts, method, params...)
}

//调用调用（常量）contract方法，参数作为输入值，并且
//将输出设置为结果。结果类型可能是用于
//返回、匿名返回的接口切片和命名的结构
//返回。
func (_Chequebook *ChequebookCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Chequebook.Contract.contract.Call(opts, result, method, params...)
}

//转账启动普通交易以将资金转移到合同，调用
//它的默认方法（如果有）。
func (_Chequebook *ChequebookTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Chequebook.Contract.contract.Transfer(opts)
}

//Transact使用参数作为输入值调用（付费）Contract方法。
func (_Chequebook *ChequebookTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Chequebook.Contract.contract.Transact(opts, method, params...)
}

//发送是一个绑定合同方法0x7Bf786f8的免费数据检索调用。
//
//solidity:函数发送（地址）常量返回（uint256）
func (_Chequebook *ChequebookCaller) Sent(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Chequebook.contract.Call(opts, out, "sent", arg0)
	return *ret0, err
}

//发送是一个绑定合同方法0x7Bf786f8的免费数据检索调用。
//
//solidity:函数发送（地址）常量返回（uint256）
func (_Chequebook *ChequebookSession) Sent(arg0 common.Address) (*big.Int, error) {
	return _Chequebook.Contract.Sent(&_Chequebook.CallOpts, arg0)
}

//发送是一个绑定合同方法0x7Bf786f8的免费数据检索调用。
//
//solidity:函数发送（地址）常量返回（uint256）
func (_Chequebook *ChequebookCallerSession) Sent(arg0 common.Address) (*big.Int, error) {
	return _Chequebook.Contract.Sent(&_Chequebook.CallOpts, arg0)
}

//现金是一个受合同方法0xFBF788D6约束的已付款的变元交易。
//
//稳固性：功能现金（受益人地址，金额：uint256，sig_v uint8，sig_r bytes 32，sig_s bytes 32）返回（）
func (_Chequebook *ChequebookTransactor) Cash(opts *bind.TransactOpts, beneficiary common.Address, amount *big.Int, sig_v uint8, sig_r [32]byte, sig_s [32]byte) (*types.Transaction, error) {
	return _Chequebook.contract.Transact(opts, "cash", beneficiary, amount, sig_v, sig_r, sig_s)
}

//现金是一个受合同方法0xFBF788D6约束的已付款的变元交易。
//
//稳固性：功能现金（受益人地址，金额：uint256，sig_v uint8，sig_r bytes 32，sig_s bytes 32）返回（）
func (_Chequebook *ChequebookSession) Cash(beneficiary common.Address, amount *big.Int, sig_v uint8, sig_r [32]byte, sig_s [32]byte) (*types.Transaction, error) {
	return _Chequebook.Contract.Cash(&_Chequebook.TransactOpts, beneficiary, amount, sig_v, sig_r, sig_s)
}

//现金是一个受合同方法0xFBF788D6约束的已付款的变元交易。
//
//稳固性：功能现金（受益人地址，金额：uint256，sig_v uint8，sig_r bytes 32，sig_s bytes 32）返回（）
func (_Chequebook *ChequebookTransactorSession) Cash(beneficiary common.Address, amount *big.Int, sig_v uint8, sig_r [32]byte, sig_s [32]byte) (*types.Transaction, error) {
	return _Chequebook.Contract.Cash(&_Chequebook.TransactOpts, beneficiary, amount, sig_v, sig_r, sig_s)
}

//kill是一个付费的mutator事务，绑定合同方法0x41c0e1b5。
//
//solidity:函数kill（）返回（）
func (_Chequebook *ChequebookTransactor) Kill(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Chequebook.contract.Transact(opts, "kill")
}

//kill是一个付费的mutator事务，绑定合同方法0x41c0e1b5。
//
//solidity:函数kill（）返回（）
func (_Chequebook *ChequebookSession) Kill() (*types.Transaction, error) {
	return _Chequebook.Contract.Kill(&_Chequebook.TransactOpts)
}

//kill是一个付费的mutator事务，绑定合同方法0x41c0e1b5。
//
//solidity:函数kill（）返回（）
func (_Chequebook *ChequebookTransactorSession) Kill() (*types.Transaction, error) {
	return _Chequebook.Contract.Kill(&_Chequebook.TransactOpts)
}

//从filterOvertaft返回checkbookOvertaftIterator，用于对支票簿合同引发的透支事件的原始日志和未打包数据进行迭代。
type ChequebookOverdraftIterator struct {
Event *ChequebookOverdraft //包含合同细节和原始日志的事件

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
func (it *ChequebookOverdraftIterator) Next() bool {
//如果迭代器失败，请停止迭代
	if it.fail != nil {
		return false
	}
//如果迭代器已完成，则直接传递可用的
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ChequebookOverdraft)
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
		it.Event = new(ChequebookOverdraft)
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
func (it *ChequebookOverdraftIterator) Error() error {
	return it.fail
}

//关闭终止迭代过程，释放任何挂起的基础
//资源。
func (it *ChequebookOverdraftIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

//支票簿透支表示支票簿合同引发的透支事件。
type ChequebookOverdraft struct {
	Deadbeat common.Address
Raw      types.Log //区块链特定的上下文信息
}

//filter透支是一个自由的日志检索操作，绑定合同事件0x2250e2993c15843b362621c89447cc589ee7a9f049c0226986e545d3c2c0c6f978。
//
//坚固性：事件透支（死区地址）
func (_Chequebook *ChequebookFilterer) FilterOverdraft(opts *bind.FilterOpts) (*ChequebookOverdraftIterator, error) {

	logs, sub, err := _Chequebook.contract.FilterLogs(opts, "Overdraft")
	if err != nil {
		return nil, err
	}
	return &ChequebookOverdraftIterator{contract: _Chequebook.contract, event: "Overdraft", logs: logs, sub: sub}, nil
}

//watchOverflft是一个免费的日志订阅操作，绑定合同事件0x2250e2993c15843b362621c89447cc589ee7a9f049c0226986e545d3c2c0c6f978。
//
//坚固性：事件透支（死区地址）
func (_Chequebook *ChequebookFilterer) WatchOverdraft(opts *bind.WatchOpts, sink chan<- *ChequebookOverdraft) (event.Subscription, error) {

	logs, sub, err := _Chequebook.contract.WatchLogs(opts, "Overdraft")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
//新日志到达，分析事件并转发给用户
				event := new(ChequebookOverdraft)
				if err := _Chequebook.contract.UnpackLog(event, "Overdraft", log); err != nil {
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
