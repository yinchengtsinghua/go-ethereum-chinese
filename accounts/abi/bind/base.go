
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

package bind

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
)

//当约定要求方法
//提交前签署交易。
type SignerFn func(types.Signer, common.Address, *types.Transaction) (*types.Transaction, error)

//Callopts是对合同调用请求进行微调的选项集合。
type CallOpts struct {
Pending     bool            //是否对挂起状态或最后一个已知状态进行操作
From        common.Address  //可选发件人地址，否则使用第一个帐户
BlockNumber *big.Int        //可选应在其上执行调用的块编号
Context     context.Context //支持取消和超时的网络上下文（nil=无超时）
}

//TransactioOpts是创建
//有效的以太坊事务。
type TransactOpts struct {
From   common.Address //用于发送交易的以太坊帐户
Nonce  *big.Int       //nonce用于事务执行（nil=使用挂起状态）
Signer SignerFn       //用于签署交易的方法（强制）

Value    *big.Int //随交易转移的资金（零=0=无资金）
GasPrice *big.Int //用于交易执行的天然气价格（零=天然气价格Oracle）
GasLimit uint64   //为交易执行设定的气体限制（0=估计）

Context context.Context //支持取消和超时的网络上下文（nil=无超时）
}

//filteropts是用于微调事件筛选的选项集合。
//在有约束力的合同中。
type FilterOpts struct {
Start uint64  //查询范围的开始
End   *uint64 //范围结束（零=最新）

Context context.Context //支持取消和超时的网络上下文（nil=无超时）
}

//watchopts是对事件订阅进行微调的选项集合。
//在有约束力的合同中。
type WatchOpts struct {
Start   *uint64         //查询范围的开始（nil=最新）
Context context.Context //支持取消和超时的网络上下文（nil=无超时）
}

//BoundContract是反映在
//以太坊网络。它包含由
//要操作的更高级别合同绑定。
type BoundContract struct {
address    common.Address     //以太坊区块链上合同的部署地址
abi        abi.ABI            //基于反射的ABI访问正确的以太坊方法
caller     ContractCaller     //读取与区块链交互的界面
transactor ContractTransactor //编写与区块链交互的接口
filterer   ContractFilterer   //与区块链交互的事件过滤
}

//NewboundContract创建一个低级合同接口，通过它调用
//交易可以通过。
func NewBoundContract(address common.Address, abi abi.ABI, caller ContractCaller, transactor ContractTransactor, filterer ContractFilterer) *BoundContract {
	return &BoundContract{
		address:    address,
		abi:        abi,
		caller:     caller,
		transactor: transactor,
		filterer:   filterer,
	}
}

//DeployContract将合同部署到以太坊区块链上，并绑定
//使用Go包装器的部署地址。
func DeployContract(opts *TransactOpts, abi abi.ABI, bytecode []byte, backend ContractBackend, params ...interface{}) (common.Address, *types.Transaction, *BoundContract, error) {
//否则，尝试部署合同
	c := NewBoundContract(common.Address{}, abi, backend, backend, backend)

	input, err := c.abi.Pack("", params...)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	tx, err := c.transact(opts, nil, append(bytecode, input...))
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	c.address = crypto.CreateAddress(opts.From, tx.Nonce())
	return c.address, tx, c, nil
}

//调用调用（常量）contract方法，参数作为输入值，并且
//将输出设置为结果。结果类型可能是用于
//返回、匿名返回的接口切片和命名的结构
//返回。
func (c *BoundContract) Call(opts *CallOpts, result interface{}, method string, params ...interface{}) error {
//不要在懒惰的用户身上崩溃
	if opts == nil {
		opts = new(CallOpts)
	}
//打包输入，调用并解压缩结果
	input, err := c.abi.Pack(method, params...)
	if err != nil {
		return err
	}
	var (
		msg    = ethereum.CallMsg{From: opts.From, To: &c.address, Data: input}
		ctx    = ensureContext(opts.Context)
		code   []byte
		output []byte
	)
	if opts.Pending {
		pb, ok := c.caller.(PendingContractCaller)
		if !ok {
			return ErrNoPendingState
		}
		output, err = pb.PendingCallContract(ctx, msg)
		if err == nil && len(output) == 0 {
//确保我们有一份合同要执行，否则就要保释。
			if code, err = pb.PendingCodeAt(ctx, c.address); err != nil {
				return err
			} else if len(code) == 0 {
				return ErrNoCode
			}
		}
	} else {
		output, err = c.caller.CallContract(ctx, msg, opts.BlockNumber)
		if err == nil && len(output) == 0 {
//确保我们有一份合同要执行，否则就要保释。
			if code, err = c.caller.CodeAt(ctx, c.address, opts.BlockNumber); err != nil {
				return err
			} else if len(code) == 0 {
				return ErrNoCode
			}
		}
	}
	if err != nil {
		return err
	}
	return c.abi.Unpack(result, method, output)
}

//Transact使用参数作为输入值调用（付费）Contract方法。
func (c *BoundContract) Transact(opts *TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
//否则，打包参数并调用合同
	input, err := c.abi.Pack(method, params...)
	if err != nil {
		return nil, err
	}
	return c.transact(opts, &c.address, input)
}

//转账启动普通交易以将资金转移到合同，调用
//它的默认方法（如果有）。
func (c *BoundContract) Transfer(opts *TransactOpts) (*types.Transaction, error) {
	return c.transact(opts, &c.address, nil)
}

//Transact执行实际的事务调用，首先派生任何缺少的
//授权字段，然后安排事务执行。
func (c *BoundContract) transact(opts *TransactOpts, contract *common.Address, input []byte) (*types.Transaction, error) {
	var err error

//确保有效的值字段并立即解析帐户
	value := opts.Value
	if value == nil {
		value = new(big.Int)
	}
	var nonce uint64
	if opts.Nonce == nil {
		nonce, err = c.transactor.PendingNonceAt(ensureContext(opts.Context), opts.From)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
		}
	} else {
		nonce = opts.Nonce.Uint64()
	}
//计算燃气补贴和燃气价格
	gasPrice := opts.GasPrice
	if gasPrice == nil {
		gasPrice, err = c.transactor.SuggestGasPrice(ensureContext(opts.Context))
		if err != nil {
			return nil, fmt.Errorf("failed to suggest gas price: %v", err)
		}
	}
	gasLimit := opts.GasLimit
	if gasLimit == 0 {
//如果没有方法调用代码，则无法成功估计气体
		if contract != nil {
			if code, err := c.transactor.PendingCodeAt(ensureContext(opts.Context), c.address); err != nil {
				return nil, err
			} else if len(code) == 0 {
				return nil, ErrNoCode
			}
		}
//如果合同确实有代码（或不需要代码），则估计交易
		msg := ethereum.CallMsg{From: opts.From, To: contract, Value: value, Data: input}
		gasLimit, err = c.transactor.EstimateGas(ensureContext(opts.Context), msg)
		if err != nil {
			return nil, fmt.Errorf("failed to estimate gas needed: %v", err)
		}
	}
//创建事务，签名并计划执行
	var rawTx *types.Transaction
	if contract == nil {
		rawTx = types.NewContractCreation(nonce, value, gasLimit, gasPrice, input)
	} else {
		rawTx = types.NewTransaction(nonce, c.address, value, gasLimit, gasPrice, input)
	}
	if opts.Signer == nil {
		return nil, errors.New("no signer to authorize the transaction with")
	}
	signedTx, err := opts.Signer(types.HomesteadSigner{}, opts.From, rawTx)
	if err != nil {
		return nil, err
	}
	if err := c.transactor.SendTransaction(ensureContext(opts.Context), signedTx); err != nil {
		return nil, err
	}
	return signedTx, nil
}

//filterlogs过滤过去块的合同日志，返回必要的
//在其上构造强类型绑定迭代器的通道。
func (c *BoundContract) FilterLogs(opts *FilterOpts, name string, query ...[]interface{}) (chan types.Log, event.Subscription, error) {
//不要在懒惰的用户身上崩溃
	if opts == nil {
		opts = new(FilterOpts)
	}
//将事件选择器附加到查询参数并构造主题集
	query = append([][]interface{}{{c.abi.Events[name].Id()}}, query...)

	topics, err := makeTopics(query...)
	if err != nil {
		return nil, nil, err
	}
//启动后台筛选
	logs := make(chan types.Log, 128)

	config := ethereum.FilterQuery{
		Addresses: []common.Address{c.address},
		Topics:    topics,
		FromBlock: new(big.Int).SetUint64(opts.Start),
	}
	if opts.End != nil {
		config.ToBlock = new(big.Int).SetUint64(*opts.End)
	}
 /*TODO（karalabe）：在支持时用此替换下面方法的其余部分
 sub，err：=c.filter.subscribeBilterLogs（ensureContext（opts.context）、config、logs）
 **/

	buff, err := c.filterer.FilterLogs(ensureContext(opts.Context), config)
	if err != nil {
		return nil, nil, err
	}
	sub, err := event.NewSubscription(func(quit <-chan struct{}) error {
		for _, log := range buff {
			select {
			case logs <- log:
			case <-quit:
				return nil
			}
		}
		return nil
	}), nil

	if err != nil {
		return nil, nil, err
	}
	return logs, sub, nil
}

//watchlogs过滤器订阅未来块的合同日志，返回
//可用于关闭观察程序的订阅对象。
func (c *BoundContract) WatchLogs(opts *WatchOpts, name string, query ...[]interface{}) (chan types.Log, event.Subscription, error) {
//不要在懒惰的用户身上崩溃
	if opts == nil {
		opts = new(WatchOpts)
	}
//将事件选择器附加到查询参数并构造主题集
	query = append([][]interface{}{{c.abi.Events[name].Id()}}, query...)

	topics, err := makeTopics(query...)
	if err != nil {
		return nil, nil, err
	}
//启动后台筛选
	logs := make(chan types.Log, 128)

	config := ethereum.FilterQuery{
		Addresses: []common.Address{c.address},
		Topics:    topics,
	}
	if opts.Start != nil {
		config.FromBlock = new(big.Int).SetUint64(*opts.Start)
	}
	sub, err := c.filterer.SubscribeFilterLogs(ensureContext(opts.Context), config, logs)
	if err != nil {
		return nil, nil, err
	}
	return logs, sub, nil
}

//解包日志将检索到的日志解包到提供的输出结构中。
func (c *BoundContract) UnpackLog(out interface{}, event string, log types.Log) error {
	if len(log.Data) > 0 {
		if err := c.abi.Unpack(out, event, log.Data); err != nil {
			return err
		}
	}
	var indexed abi.Arguments
	for _, arg := range c.abi.Events[event].Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	return parseTopics(out, indexed, log.Topics[1:])
}

//EnsureContext是一个助手方法，用于确保上下文不为零，即使
//用户指定了它。
func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.TODO()
	}
	return ctx
}
