
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
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

//插件在块插入时跟踪和报告。
type insertStats struct {
	queued, processed, ignored int
	usedGas                    uint64
	lastIndex                  int
	startTime                  mclock.AbsTime
}

//statsreportlimit是导入和导出期间的时间限制，在此之后我们
//总是打印出进度。这避免了用户想知道发生了什么。
const statsReportLimit = 8 * time.Second

//如果处理了一些块，则报告将打印统计信息
//或者自上一条消息以来已经过了几秒钟。
func (st *insertStats) report(chain []*types.Block, index int, cache common.StorageSize) {
//获取批的计时
	var (
		now     = mclock.Now()
		elapsed = time.Duration(now) - time.Duration(st.startTime)
	)
//如果我们在到达的批或报告周期的最后一个块，请记录
	if index == len(chain)-1 || elapsed >= statsReportLimit {
//计算此段中的事务数
		var txs int
		for _, block := range chain[st.lastIndex : index+1] {
			txs += len(block.Transactions())
		}
		end := chain[index]

//组装日志上下文并将其发送到记录器
		context := []interface{}{
			"blocks", st.processed, "txs", txs, "mgas", float64(st.usedGas) / 1000000,
			"elapsed", common.PrettyDuration(elapsed), "mgasps", float64(st.usedGas) * 1000 / float64(elapsed),
			"number", end.Number(), "hash", end.Hash(),
		}
		if timestamp := time.Unix(end.Time().Int64(), 0); time.Since(timestamp) > time.Minute {
			context = append(context, []interface{}{"age", common.PrettyAge(timestamp)}...)
		}
		context = append(context, []interface{}{"cache", cache}...)

		if st.queued > 0 {
			context = append(context, []interface{}{"queued", st.queued}...)
		}
		if st.ignored > 0 {
			context = append(context, []interface{}{"ignored", st.ignored}...)
		}
		log.Info("Imported new chain segment", context...)

//将报告的统计数据转发到下一节
		*st = insertStats{startTime: now, lastIndex: index + 1}
	}
}

//插入器是在链导入过程中提供帮助的助手。
type insertIterator struct {
	chain     types.Blocks
	results   <-chan error
	index     int
	validator Validator
}

//newinsertiator基于给定的块创建一个新的迭代器，它是
//假定为连续链。
func newInsertIterator(chain types.Blocks, results <-chan error, validator Validator) *insertIterator {
	return &insertIterator{
		chain:     chain,
		results:   results,
		index:     -1,
		validator: validator,
	}
}

//next返回迭代器中的下一个块，以及任何可能的验证
//该块出错。当结束时，它将返回（零，零）。
func (it *insertIterator) next() (*types.Block, error) {
	if it.index+1 >= len(it.chain) {
		it.index = len(it.chain)
		return nil, nil
	}
	it.index++
	if err := <-it.results; err != nil {
		return it.chain[it.index], err
	}
	return it.chain[it.index], it.validator.ValidateBody(it.chain[it.index])
}

//current返回正在处理的当前块。
func (it *insertIterator) current() *types.Block {
	if it.index < 0 || it.index+1 >= len(it.chain) {
		return nil
	}
	return it.chain[it.index]
}

//previous返回正在处理的前一个块，或者为nil
func (it *insertIterator) previous() *types.Block {
	if it.index < 1 {
		return nil
	}
	return it.chain[it.index-1]
}

//首先返回IT中的第一个块。
func (it *insertIterator) first() *types.Block {
	return it.chain[0]
}

//remaining返回剩余块的数目。
func (it *insertIterator) remaining() int {
	return len(it.chain) - it.index
}

//processed返回已处理的块数。
func (it *insertIterator) processed() int {
	return it.index + 1
}
