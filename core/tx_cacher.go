
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

package core

import (
	"runtime"

	"github.com/ethereum/go-ethereum/core/types"
)

//SENDECCHACER是并发事务发送者恢复和缓存。
var senderCacher = newTxSenderCacher(runtime.NumCPU())

//txsendercacherRequest是一个用于恢复事务发送方的请求，
//特定的签名方案并将其缓存到事务本身中。
//
//inc字段定义每次恢复后要跳过的事务数，
//它用于向不同的线程提供相同的基础输入数组，但
//确保他们快速处理早期事务。
type txSenderCacherRequest struct {
	signer types.Signer
	txs    []*types.Transaction
	inc    int
}

//txsendercacher是用于并发ecrecover事务的辅助结构
//来自后台线程上数字签名的发件人。
type txSenderCacher struct {
	threads int
	tasks   chan *txSenderCacherRequest
}

//newTxSenderCacher creates a new transaction sender background cacher and starts
//gomaxprocs在构建时允许的处理goroutine的数量。
func newTxSenderCacher(threads int) *txSenderCacher {
	cacher := &txSenderCacher{
		tasks:   make(chan *txSenderCacherRequest, threads),
		threads: threads,
	}
	for i := 0; i < threads; i++ {
		go cacher.cache()
	}
	return cacher
}

//缓存是一个无限循环，缓存来自各种形式的事务发送者
//数据结构。
func (cacher *txSenderCacher) cache() {
	for task := range cacher.tasks {
		for i := 0; i < len(task.txs); i += task.inc {
			types.Sender(task.signer, task.txs[i])
		}
	}
}

//recover从一批事务中恢复发送方并缓存它们
//回到相同的数据结构中。没有进行验证，也没有
//对无效签名的任何反应。这取决于以后调用代码。
func (cacher *txSenderCacher) recover(signer types.Signer, txs []*types.Transaction) {
//如果没有什么可恢复的，中止
	if len(txs) == 0 {
		return
	}
//确保我们拥有有意义的任务规模并计划恢复
	tasks := cacher.threads
	if len(txs) < tasks*4 {
		tasks = (len(txs) + 3) / 4
	}
	for i := 0; i < tasks; i++ {
		cacher.tasks <- &txSenderCacherRequest{
			signer: signer,
			txs:    txs[i:],
			inc:    tasks,
		}
	}
}

//恢复器块从批处理中恢复发件人并缓存它们。
//回到相同的数据结构中。没有进行验证，也没有
//对无效签名的任何反应。这取决于以后调用代码。
func (cacher *txSenderCacher) recoverFromBlocks(signer types.Signer, blocks []*types.Block) {
	count := 0
	for _, block := range blocks {
		count += len(block.Transactions())
	}
	txs := make([]*types.Transaction, 0, count)
	for _, block := range blocks {
		txs = append(txs, block.Transactions()...)
	}
	cacher.recover(signer, txs)
}
