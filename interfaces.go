
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

//包以太坊定义了与以太坊交互的接口。
package ethereum

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

//如果请求的项不存在，则API方法将返回NotFound。
var NotFound = errors.New("not found")

//TODO:将订阅移动到包事件

//订阅表示事件订阅，其中
//通过数据通道传送。
type Subscription interface {
//Unsubscribe cancels the sending of events to the data channel
//关闭错误通道。
	Unsubscribe()
//err返回订阅错误通道。错误通道接收
//如果订阅存在问题（例如网络连接）的值
//传递活动已关闭）。将只发送一个值。
//通过退订来关闭错误通道。
	Err() <-chan error
}

//ChainReader提供对区块链的访问。此接口中的方法访问原始
//来自规范链（按块号请求时）或任何
//以前由节点下载和处理的区块链分支。街区
//number参数可以为nil以选择最新的规范块。读取块头
//应尽可能优先于全块。
//
//如果请求的项不存在，则找不到返回的错误。
type ChainReader interface {
	BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
	HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	TransactionCount(ctx context.Context, blockHash common.Hash) (uint, error)
	TransactionInBlock(ctx context.Context, blockHash common.Hash, index uint) (*types.Transaction, error)

//此方法订阅有关
//规范链。
	SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (Subscription, error)
}

//TransactionReader提供对过去事务及其收据的访问。
//实施可能会对以下交易和收据施加任意限制：
//can be retrieved. Historic transactions may not be available.
//
//尽可能避免依赖此接口。合同日志（通过日志过滤器
//接口）更可靠，在有链条的情况下通常更安全。
//重组。
//
//如果请求的项不存在，则找不到返回的错误。
type TransactionReader interface {
//TransactionByHash除了检查
//块链。ISPUPDATE返回值指示事务是否已被关闭。
//开采了。请注意，事务可能不是规范链的一部分，即使
//它没有挂起。
	TransactionByHash(ctx context.Context, txHash common.Hash) (tx *types.Transaction, isPending bool, err error)
//TransactionReceipt返回挖掘的事务的收据。请注意
//事务可能不包括在当前规范链中，即使收据
//存在。
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
}

//ChainStateReader包装对规范区块链的状态trie的访问。注意
//接口的实现可能无法返回旧块的状态值。
//在许多情况下，使用CallContract比读取原始合同存储更可取。
type ChainStateReader interface {
	BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
	StorageAt(ctx context.Context, account common.Address, key common.Hash, blockNumber *big.Int) ([]byte, error)
	CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error)
	NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error)
}

//当节点与
//以太坊网络。
type SyncProgress struct {
StartingBlock uint64 //Block number where sync began
CurrentBlock  uint64 //同步所在的当前块号
HighestBlock  uint64 //链中最高的声称块数
PulledStates  uint64 //已下载的状态trie条目数
KnownStates   uint64 //已知的State Trie条目的总数
}

//ChainSyncReader将访问打包到节点的当前同步状态。如果没有
//同步当前正在运行，它返回零。
type ChainSyncReader interface {
	SyncProgress(ctx context.Context) (*SyncProgress, error)
}

//callmsg包含合同调用的参数。
type CallMsg struct {
From     common.Address  //“交易”的发送方
To       *common.Address //目的地合同（合同创建为零）
Gas      uint64          //如果为0，则调用以接近无穷大的气体执行。
GasPrice *big.Int        //气体交换率
Value    *big.Int        //随呼叫发送的wei数量
Data     []byte          //输入数据，通常是ABI编码的合同方法调用
}

//A ContractCaller provides contract calls, essentially transactions that are executed by
//EVM，但没有挖掘到区块链中。ContractCall是用于
//执行此类调用。对于围绕特定合同构建的应用程序，
//AbigEn工具提供了一种更好的、正确类型的执行调用的方法。
type ContractCaller interface {
	CallContract(ctx context.Context, call CallMsg, blockNumber *big.Int) ([]byte, error)
}

//filterquery包含用于合同日志筛选的选项。
type FilterQuery struct {
BlockHash *common.Hash     //used by eth_getLogs, return logs only from block with this hash
FromBlock *big.Int         //查询范围的开始，零表示Genesis块
ToBlock   *big.Int         //范围结束，零表示最新块
Addresses []common.Address //restricts matches to events created by specific contracts

//主题列表限制与特定事件主题的匹配。每个事件都有一个列表
//话题。Topics matches a prefix of that list. An empty element slice matches any
//话题。非空元素表示与
//包含的主题。
//
//实例：
//或零匹配任何主题列表
//{{A}}              matches topic A in first position
//，b匹配第一位置的任何主题，B匹配第二位置的任何主题
//A，B匹配第一位置的主题A，第二位置的主题B
//A，B，C，D匹配第一位置的主题（A或B），第二位置的主题（C或D）
	Topics [][]common.Hash
}

//LogFilter提供使用一次性查询或连续查询访问合同日志事件的权限
//事件订阅。
//
//Logs received through a streaming query subscription may have Removed set to true,
//指示由于链重组而恢复日志。
type LogFilterer interface {
	FilterLogs(ctx context.Context, q FilterQuery) ([]types.Log, error)
	SubscribeFilterLogs(ctx context.Context, q FilterQuery, ch chan<- types.Log) (Subscription, error)
}

//TransactionSender包装事务发送。sendTransaction方法注入
//已将事务签名到挂起的事务池中以供执行。如果交易
//是合同创建的，TransactionReceipt方法可用于检索
//挖掘交易记录后的合同地址。
//
//必须对该事务进行签名并包含一个有效的nonce。消费者
//API可以使用包帐户来维护本地私钥，并且需要检索
//下一个可用的nonce使用pendingnonceat。
type TransactionSender interface {
	SendTransaction(ctx context.Context, tx *types.Transaction) error
}

//Gasparicer包装了天然气价格甲骨文，甲骨文监控区块链以确定
//在当前收费市场条件下的最优天然气价格。
type GasPricer interface {
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
}

//一个PungStuteReDead提供对挂起状态的访问，这是所有结果的结果。
//尚未包含在区块链中的已知可执行交易。它是
//通常用于显示“未确认”操作的结果（例如钱包价值
//传输）由用户启动。PendingNoncoat操作是一种很好的方法
//检索特定帐户的下一个可用事务。
type PendingStateReader interface {
	PendingBalanceAt(ctx context.Context, account common.Address) (*big.Int, error)
	PendingStorageAt(ctx context.Context, account common.Address, key common.Hash) ([]byte, error)
	PendingCodeAt(ctx context.Context, account common.Address) ([]byte, error)
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
	PendingTransactionCount(ctx context.Context) (uint, error)
}

//PendingContractCaller可用于对挂起状态执行调用。
type PendingContractCaller interface {
	PendingCallContract(ctx context.Context, call CallMsg) ([]byte, error)
}

//GasEstimator包装EstimateGas，它试图估计执行
//基于未决状态的特定事务。不能保证这是
//真正的天然气限制要求，因为其他交易可能由矿工添加或删除，但
//它应为设定合理违约提供依据。
type GasEstimator interface {
	EstimateGas(ctx context.Context, call CallMsg) (uint64, error)
}

//PendingStateEventer提供对
//悬而未决的状态。
type PendingStateEventer interface {
	SubscribePendingTransactions(ctx context.Context, ch chan<- *types.Transaction) (Subscription, error)
}
