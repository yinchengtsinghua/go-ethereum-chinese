
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

package miner

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

//noopchainretriever是headerretriever的一个实现，它始终
//对于任何请求的头返回nil。
type noopChainRetriever struct{}

func (r *noopChainRetriever) GetHeaderByNumber(number uint64) *types.Header {
	return nil
}
func (r *noopChainRetriever) GetBlockByNumber(number uint64) *types.Block {
	return nil
}

//将块插入未确认集的测试将累积这些块，直到
//达到所需深度后，开始下降。
func TestUnconfirmedInsertBounds(t *testing.T) {
	limit := uint(10)

	pool := newUnconfirmedBlocks(new(noopChainRetriever), limit)
	for depth := uint64(0); depth < 2*uint64(limit); depth++ {
//为同一级别插入多个块以强调它
		for i := 0; i < int(depth); i++ {
			pool.Insert(depth, common.Hash([32]byte{byte(depth), byte(i)}))
		}
//确认深度允许范围以下没有块留在
		pool.blocks.Do(func(block interface{}) {
			if block := block.(*unconfirmedBlock); block.index+uint64(limit) <= depth {
				t.Errorf("depth %d: block %x not dropped", depth, block.hash)
			}
		})
	}
}

//将块移出未确认集的测试正常工作
//箱，以及角箱，如空箱、空班或满箱
//轮班。
func TestUnconfirmedShifts(t *testing.T) {
//在不同深度上创建带有几个块的池
	limit, start := uint(10), uint64(25)

	pool := newUnconfirmedBlocks(new(noopChainRetriever), limit)
	for depth := start; depth < start+uint64(limit); depth++ {
		pool.Insert(depth, common.Hash([32]byte{byte(depth)}))
	}
//试着移到极限以下，确保没有掉块。
	pool.Shift(start + uint64(limit) - 1)
	if n := pool.blocks.Len(); n != int(limit) {
		t.Errorf("unconfirmed count mismatch: have %d, want %d", n, limit)
	}
//试着把一半的积木移走，并核实剩余部分。
	pool.Shift(start + uint64(limit) - 1 + uint64(limit/2))
	if n := pool.blocks.Len(); n != int(limit)/2 {
		t.Errorf("unconfirmed count mismatch: have %d, want %d", n, limit/2)
	}
//尝试将所有剩余的块移出并验证是否为空。
	pool.Shift(start + 2*uint64(limit))
	if n := pool.blocks.Len(); n != 0 {
		t.Errorf("unconfirmed count mismatch: have %d, want %d", n, 0)
	}
//试着从空的那套换出来，确保它不坏
	pool.Shift(start + 3*uint64(limit))
	if n := pool.blocks.Len(); n != 0 {
		t.Errorf("unconfirmed count mismatch: have %d, want %d", n, 0)
	}
}
