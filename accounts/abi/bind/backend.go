
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
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var (
//errnocode由请求的调用和事务处理操作返回
//要操作的收件人合同在状态db中不存在或不存在
//有任何与之相关的代码（即自杀）。
	ErrNoCode = errors.New("no contract code at given address")

//尝试执行挂起状态操作时出现此错误
//在不实现PendingContractCaller的后端上。
	ErrNoPendingState = errors.New("backend does not support pending state")

//如果合同创建离开
//后面是空合同。
	ErrNoCodeAfterDeploy = errors.New("no contract code after deployment")
)

//ContractCaller定义允许在读取时使用约定进行操作所需的方法
//只有基础。
type ContractCaller interface {
//codeat返回给定帐户的代码。这是为了区分
//在合同内部错误和本地链不同步之间。
	CodeAt(ctx context.Context, contract common.Address, blockNumber *big.Int) ([]byte, error)
//ContractCall以指定的数据作为
//输入。
	CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)
}

//PendingContractCaller定义在挂起状态下执行协定调用的方法。
//当请求访问挂起状态时，调用将尝试发现此接口。
//如果后端不支持挂起状态，则call返回errnopendingState。
type PendingContractCaller interface {
//PendingCodeAt返回处于挂起状态的给定帐户的代码。
	PendingCodeAt(ctx context.Context, contract common.Address) ([]byte, error)
//PendingCallContract针对挂起状态执行以太坊合同调用。
	PendingCallContract(ctx context.Context, call ethereum.CallMsg) ([]byte, error)
}

//ContractTransactor定义了允许使用Contract操作所需的方法
//只写。除了事务处理方法，其余的是帮助器
//当用户不提供某些需要的值，而是将其保留时使用
//交交易人决定。
type ContractTransactor interface {
//PendingCodeAt返回处于挂起状态的给定帐户的代码。
	PendingCodeAt(ctx context.Context, account common.Address) ([]byte, error)
//pendingnonceat检索与帐户关联的当前挂起的nonce。
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
//SuggestGasprice检索当前建议的天然气价格，以便及时
//交易的执行。
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
//EstimateGas试图估计执行特定
//基于后端区块链当前挂起状态的交易。
//不能保证这是真正的气体限值要求
//交易可以由矿工添加或删除，但它应该提供一个基础
//设置合理的默认值。
	EstimateGas(ctx context.Context, call ethereum.CallMsg) (gas uint64, err error)
//sendTransaction将事务注入挂起池以执行。
	SendTransaction(ctx context.Context, tx *types.Transaction) error
}

//ContractFilter定义了使用一次性访问日志事件所需的方法
//查询或连续事件订阅。
type ContractFilterer interface {
//filterlogs执行日志筛选操作，在执行期间阻塞，以及
//一批返回所有结果。
//
//TODO（karalabe）：当订阅可以返回过去的数据时，取消预测。
	FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error)

//subscribeBilterLogs创建后台日志筛选操作，返回
//立即订阅，可用于流式处理找到的事件。
	SubscribeFilterLogs(ctx context.Context, query ethereum.FilterQuery, ch chan<- types.Log) (ethereum.Subscription, error)
}

//deploybackend包装waitmined和waitdeployed所需的操作。
type DeployBackend interface {
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error)
}

//ContractBackend定义了在读写基础上处理合同所需的方法。
type ContractBackend interface {
	ContractCaller
	ContractTransactor
	ContractFilterer
}
