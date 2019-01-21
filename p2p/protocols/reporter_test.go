
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

package protocols

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

//TestReporter测试为P2P会计收集的度量
//在重新启动节点后将被持久化并可用。
//它通过重新创建数据库模拟重新启动，就像节点重新启动一样。
func TestReporter(t *testing.T) {
//创建测试目录
	dir, err := ioutil.TempDir("", "reporter-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

//设置指标
	log.Debug("Setting up metrics first time")
	reportInterval := 5 * time.Millisecond
	metrics := SetupAccountingMetrics(reportInterval, filepath.Join(dir, "test.db"))
	log.Debug("Done.")

//做一些度量
	mBalanceCredit.Inc(12)
	mBytesCredit.Inc(34)
	mMsgDebit.Inc(9)

//给报告者时间将指标写入数据库
	time.Sleep(20 * time.Millisecond)

//将度量值设置为零-这有效地模拟了关闭的节点…
	mBalanceCredit = nil
	mBytesCredit = nil
	mMsgDebit = nil
//同时关闭数据库，否则无法创建新数据库
	metrics.Close()

//再次设置指标
	log.Debug("Setting up metrics second time")
	metrics = SetupAccountingMetrics(reportInterval, filepath.Join(dir, "test.db"))
	defer metrics.Close()
	log.Debug("Done.")

//现在检查度量，它们应该与“关闭”之前的值相同。
	if mBalanceCredit.Count() != 12 {
		t.Fatalf("Expected counter to be %d, but is %d", 12, mBalanceCredit.Count())
	}
	if mBytesCredit.Count() != 34 {
		t.Fatalf("Expected counter to be %d, but is %d", 23, mBytesCredit.Count())
	}
	if mMsgDebit.Count() != 9 {
		t.Fatalf("Expected counter to be %d, but is %d", 9, mMsgDebit.Count())
	}
}
