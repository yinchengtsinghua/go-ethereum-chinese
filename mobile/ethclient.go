
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

//包含以太坊客户端的包装。

package geth

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

//以太坊客户端提供对以太坊API的访问。
type EthereumClient struct {
	client *ethclient.Client
}

//newethereumclient将客户机连接到给定的URL。
func NewEthereumClient(rawurl string) (client *EthereumClient, _ error) {
	rawClient, err := ethclient.Dial(rawurl)
	return &EthereumClient{rawClient}, err
}

//GetBlockByHash返回给定的完整块。
func (ec *EthereumClient) GetBlockByHash(ctx *Context, hash *Hash) (block *Block, _ error) {
	rawBlock, err := ec.client.BlockByHash(ctx.context, hash.hash)
	return &Block{rawBlock}, err
}

//GetBlockByNumber返回当前规范链中的块。如果数字小于0，则
//返回最新的已知块。
func (ec *EthereumClient) GetBlockByNumber(ctx *Context, number int64) (block *Block, _ error) {
	if number < 0 {
		rawBlock, err := ec.client.BlockByNumber(ctx.context, nil)
		return &Block{rawBlock}, err
	}
	rawBlock, err := ec.client.BlockByNumber(ctx.context, big.NewInt(number))
	return &Block{rawBlock}, err
}

//GetHeaderByHash返回具有给定哈希的块头。
func (ec *EthereumClient) GetHeaderByHash(ctx *Context, hash *Hash) (header *Header, _ error) {
	rawHeader, err := ec.client.HeaderByHash(ctx.context, hash.hash)
	return &Header{rawHeader}, err
}

//GetHeaderByNumber返回当前规范链的块头。如果数字小于0，
//返回最新的已知头。
func (ec *EthereumClient) GetHeaderByNumber(ctx *Context, number int64) (header *Header, _ error) {
	if number < 0 {
		rawHeader, err := ec.client.HeaderByNumber(ctx.context, nil)
		return &Header{rawHeader}, err
	}
	rawHeader, err := ec.client.HeaderByNumber(ctx.context, big.NewInt(number))
	return &Header{rawHeader}, err
}

//GetTransactionByHash返回具有给定哈希的事务。
func (ec *EthereumClient) GetTransactionByHash(ctx *Context, hash *Hash) (tx *Transaction, _ error) {
//TODO（karalabe）：句柄显示
	rawTx, _, err := ec.client.TransactionByHash(ctx.context, hash.hash)
	return &Transaction{rawTx}, err
}

//GetTransactionSsender返回事务的发件人地址。交易必须
//在给定的块和索引中包含在区块链中。
func (ec *EthereumClient) GetTransactionSender(ctx *Context, tx *Transaction, blockhash *Hash, index int) (sender *Address, _ error) {
	addr, err := ec.client.TransactionSender(ctx.context, tx.tx, blockhash.hash, uint(index))
	return &Address{addr}, err
}

//GetTransactionCount返回给定块中的事务总数。
func (ec *EthereumClient) GetTransactionCount(ctx *Context, hash *Hash) (count int, _ error) {
	rawCount, err := ec.client.TransactionCount(ctx.context, hash.hash)
	return int(rawCount), err
}

//GetTransactionInBlock返回给定块中索引处的单个事务。
func (ec *EthereumClient) GetTransactionInBlock(ctx *Context, hash *Hash, index int) (tx *Transaction, _ error) {
	rawTx, err := ec.client.TransactionInBlock(ctx.context, hash.hash, uint(index))
	return &Transaction{rawTx}, err

}

//GetTransactionReceipt按事务哈希返回事务的接收。
//请注意，收据不可用于待处理的交易。
func (ec *EthereumClient) GetTransactionReceipt(ctx *Context, hash *Hash) (receipt *Receipt, _ error) {
	rawReceipt, err := ec.client.TransactionReceipt(ctx.context, hash.hash)
	return &Receipt{rawReceipt}, err
}

//SyncProgress检索同步算法的当前进度。如果有
//当前没有运行同步，它返回零。
func (ec *EthereumClient) SyncProgress(ctx *Context) (progress *SyncProgress, _ error) {
	rawProgress, err := ec.client.SyncProgress(ctx.context)
	if rawProgress == nil {
		return nil, err
	}
	return &SyncProgress{*rawProgress}, err
}

//NewHeadHandler是一个客户端订阅回调，用于在事件和
//订阅失败。
type NewHeadHandler interface {
	OnNewHead(header *Header)
	OnError(failure string)
}

//订阅订阅当前区块链头的通知
//在给定的频道上。
func (ec *EthereumClient) SubscribeNewHead(ctx *Context, handler NewHeadHandler, buffer int) (sub *Subscription, _ error) {
//在内部订阅事件
	ch := make(chan *types.Header, buffer)
	rawSub, err := ec.client.SubscribeNewHead(ctx.context, ch)
	if err != nil {
		return nil, err
	}
//启动一个调度器以反馈回拨
	go func() {
		for {
			select {
			case header := <-ch:
				handler.OnNewHead(&Header{header})

			case err := <-rawSub.Err():
				if err != nil {
					handler.OnError(err.Error())
				}
				return
			}
		}
	}()
	return &Subscription{rawSub}, nil
}

//状态访问

//getbalanceat返回给定帐户的wei余额。
//块编号可以小于0，在这种情况下，余额取自最新的已知块。
func (ec *EthereumClient) GetBalanceAt(ctx *Context, account *Address, number int64) (balance *BigInt, _ error) {
	if number < 0 {
		rawBalance, err := ec.client.BalanceAt(ctx.context, account.address, nil)
		return &BigInt{rawBalance}, err
	}
	rawBalance, err := ec.client.BalanceAt(ctx.context, account.address, big.NewInt(number))
	return &BigInt{rawBalance}, err
}

//GetStorageAt返回给定帐户的合同存储中密钥的值。
//块编号可以小于0，在这种情况下，该值取自最新的已知块。
func (ec *EthereumClient) GetStorageAt(ctx *Context, account *Address, key *Hash, number int64) (storage []byte, _ error) {
	if number < 0 {
		return ec.client.StorageAt(ctx.context, account.address, key.hash, nil)
	}
	return ec.client.StorageAt(ctx.context, account.address, key.hash, big.NewInt(number))
}

//getcodeat返回给定帐户的合同代码。
//块编号可以小于0，在这种情况下，代码取自最新的已知块。
func (ec *EthereumClient) GetCodeAt(ctx *Context, account *Address, number int64) (code []byte, _ error) {
	if number < 0 {
		return ec.client.CodeAt(ctx.context, account.address, nil)
	}
	return ec.client.CodeAt(ctx.context, account.address, big.NewInt(number))
}

//getnonceat返回给定帐户的nonce帐户。
//块号可以小于0，在这种情况下，nonce是从最新的已知块中获取的。
func (ec *EthereumClient) GetNonceAt(ctx *Context, account *Address, number int64) (nonce int64, _ error) {
	if number < 0 {
		rawNonce, err := ec.client.NonceAt(ctx.context, account.address, nil)
		return int64(rawNonce), err
	}
	rawNonce, err := ec.client.NonceAt(ctx.context, account.address, big.NewInt(number))
	return int64(rawNonce), err
}

//过滤器

//filterlogs执行筛选器查询。
func (ec *EthereumClient) FilterLogs(ctx *Context, query *FilterQuery) (logs *Logs, _ error) {
	rawLogs, err := ec.client.FilterLogs(ctx.context, query.query)
	if err != nil {
		return nil, err
	}
//由于vm.logs为[]*vm.log，临时黑客
	res := make([]*types.Log, len(rawLogs))
	for i := range rawLogs {
		res[i] = &rawLogs[i]
	}
	return &Logs{res}, nil
}

//filterlogshandler是一个客户端订阅回调，用于在事件和
//订阅失败。
type FilterLogsHandler interface {
	OnFilterLogs(log *Log)
	OnError(failure string)
}

//subscribeFilterLogs订阅流式筛选查询的结果。
func (ec *EthereumClient) SubscribeFilterLogs(ctx *Context, query *FilterQuery, handler FilterLogsHandler, buffer int) (sub *Subscription, _ error) {
//在内部订阅事件
	ch := make(chan types.Log, buffer)
	rawSub, err := ec.client.SubscribeFilterLogs(ctx.context, query.query, ch)
	if err != nil {
		return nil, err
	}
//启动一个调度器以反馈回拨
	go func() {
		for {
			select {
			case log := <-ch:
				handler.OnFilterLogs(&Log{&log})

			case err := <-rawSub.Err():
				if err != nil {
					handler.OnError(err.Error())
				}
				return
			}
		}
	}()
	return &Subscription{rawSub}, nil
}

//预备状态

//getPendingBalanceAt返回处于挂起状态的给定帐户的wei余额。
func (ec *EthereumClient) GetPendingBalanceAt(ctx *Context, account *Address) (balance *BigInt, _ error) {
	rawBalance, err := ec.client.PendingBalanceAt(ctx.context, account.address)
	return &BigInt{rawBalance}, err
}

//GetPendingStorageAt返回处于挂起状态的给定帐户的合同存储中键的值。
func (ec *EthereumClient) GetPendingStorageAt(ctx *Context, account *Address, key *Hash) (storage []byte, _ error) {
	return ec.client.PendingStorageAt(ctx.context, account.address, key.hash)
}

//GetPendingCodeAt返回处于挂起状态的给定帐户的合同代码。
func (ec *EthereumClient) GetPendingCodeAt(ctx *Context, account *Address) (code []byte, _ error) {
	return ec.client.PendingCodeAt(ctx.context, account.address)
}

//GetPendingOnCate返回处于挂起状态的给定帐户的帐户nonce。
//这是应该用于下一个事务的nonce。
func (ec *EthereumClient) GetPendingNonceAt(ctx *Context, account *Address) (nonce int64, _ error) {
	rawNonce, err := ec.client.PendingNonceAt(ctx.context, account.address)
	return int64(rawNonce), err
}

//GetPendingtTransactionCount返回处于挂起状态的事务总数。
func (ec *EthereumClient) GetPendingTransactionCount(ctx *Context) (count int, _ error) {
	rawCount, err := ec.client.PendingTransactionCount(ctx.context)
	return int(rawCount), err
}

//合同呼叫

//CallContract执行消息调用事务，该事务直接在VM中执行
//但从未开发到区块链中。
//
//BlockNumber选择运行调用的块高度。它可以小于0，其中
//如果代码取自最新的已知块。注意这个状态从很老
//块可能不可用。
func (ec *EthereumClient) CallContract(ctx *Context, msg *CallMsg, number int64) (output []byte, _ error) {
	if number < 0 {
		return ec.client.CallContract(ctx.context, msg.msg, nil)
	}
	return ec.client.CallContract(ctx.context, msg.msg, big.NewInt(number))
}

//PendingCallContract使用EVM执行消息调用事务。
//合同调用所看到的状态是挂起状态。
func (ec *EthereumClient) PendingCallContract(ctx *Context, msg *CallMsg) (output []byte, _ error) {
	return ec.client.PendingCallContract(ctx.context, msg.msg)
}

//SuggestGasprice检索当前建议的天然气价格，以便及时
//交易的执行。
func (ec *EthereumClient) SuggestGasPrice(ctx *Context) (price *BigInt, _ error) {
	rawPrice, err := ec.client.SuggestGasPrice(ctx.context)
	return &BigInt{rawPrice}, err
}

//EstimateGas试图根据
//后端区块链的当前挂起状态。不能保证这是
//矿工可能增加或移除的其他交易的实际气体限制要求，
//但它应为设定合理违约提供依据。
func (ec *EthereumClient) EstimateGas(ctx *Context, msg *CallMsg) (gas int64, _ error) {
	rawGas, err := ec.client.EstimateGas(ctx.context, msg.msg)
	return int64(rawGas), err
}

//sendTransaction将签名的事务注入挂起池以执行。
//
//如果事务是合同创建，请使用TransactionReceipt方法获取
//挖掘交易记录后的合同地址。
func (ec *EthereumClient) SendTransaction(ctx *Context, tx *Transaction) error {
	return ec.client.SendTransaction(ctx.context, tx.tx)
}
