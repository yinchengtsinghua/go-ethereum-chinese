
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

package tests

import (
	"testing"
)

func TestBlockchain(t *testing.T) {
	t.Parallel()

	bt := new(testMatcher)
//一般的状态测试作为区块链测试“导出”，但我们可以在本地运行它们。
	bt.skipLoad(`^GeneralStateTests/`)
//跳过由于自私的挖掘测试而导致的随机失败。
	bt.skipLoad(`^bcForgedTest/bcForkUncle\.json`)
	bt.skipLoad(`^bcMultiChainTest/(ChainAtoChainB_blockorder|CallContractFromNotBestBlock)`)
	bt.skipLoad(`^bcTotalDifficultyTest/(lotsOfLeafs|lotsOfBranches|sideChainWithMoreTransactions)`)
//慢测试
	bt.slow(`^bcExploitTest/DelegateCallSpam.json`)
	bt.slow(`^bcExploitTest/ShanghaiLove.json`)
	bt.slow(`^bcExploitTest/SuicideIssue.json`)
	bt.slow(`^bcForkStressTest/`)
	bt.slow(`^bcGasPricerTest/RPC_API_Test.json`)
	bt.slow(`^bcWalletTest/`)

//仍未能通过我们需要调查的测试
//bt.失败（`^bcstatetests/suicidethecheckbalance.json/suicidethecheckbalance constantinople`，'todo:investive'）
//bt.失败（`^bcstatetests/suicidestoragecheckvcreate2.json/suicidestoragecheckvcreate2_Constantinople`，'todo:investive'）
//bt.失败（`^bcstatetests/suicidestoragecheckvcreate.json/suicidestoragecheckvcreate_Constantinople`，'todo:investive'）
//bt.失败（`^bcstatetests/suicidestoragecheck.json/suicidestoragecheck_constantinople`，'todo:investive'）

	bt.walk(t, blockTestDir, func(t *testing.T, name string, test *BlockTest) {
		if err := bt.checkFailure(t, name, test.Run()); err != nil {
			t.Error(err)
		}
	})
}
