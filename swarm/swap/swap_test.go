
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

package swap

import (
	"flag"
	"fmt"
	"io/ioutil"
	mrand "math/rand"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/state"
	colorable "github.com/mattn/go-colorable"
)

var (
	loglevel = flag.Int("loglevel", 2, "verbosity of logs")
)

func init() {
	flag.Parse()
	mrand.Seed(time.Now().UnixNano())

	log.PrintOrigins(true)
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(true))))
}

//测试获得同伴的平衡
func TestGetPeerBalance(t *testing.T) {
//创建测试交换帐户
	swap, testDir := createTestSwap(t)
	defer os.RemoveAll(testDir)

//测试正确值
	testPeer := newDummyPeer()
	swap.balances[testPeer.ID()] = 888
	b, err := swap.GetPeerBalance(testPeer.ID())
	if err != nil {
		t.Fatal(err)
	}
	if b != 888 {
		t.Fatalf("Expected peer's balance to be %d, but is %d", 888, b)
	}

//不存在节点的测试
	id := adapters.RandomNodeConfig().ID
	_, err = swap.GetPeerBalance(id)
	if err == nil {
		t.Fatal("Expected call to fail, but it didn't!")
	}
	if err.Error() != "Peer not found" {
		t.Fatalf("Expected test to fail with %s, but is %s", "Peer not found", err.Error())
	}
}

//测试重复预订是否正确记帐
func TestRepeatedBookings(t *testing.T) {
//创建测试交换帐户
	swap, testDir := createTestSwap(t)
	defer os.RemoveAll(testDir)

	testPeer := newDummyPeer()
	amount := mrand.Intn(100)
	cnt := 1 + mrand.Intn(10)
	for i := 0; i < cnt; i++ {
		swap.Add(int64(amount), testPeer.Peer)
	}
	expectedBalance := int64(cnt * amount)
	realBalance := swap.balances[testPeer.ID()]
	if expectedBalance != realBalance {
		t.Fatal(fmt.Sprintf("After %d credits of %d, expected balance to be: %d, but is: %d", cnt, amount, expectedBalance, realBalance))
	}

	testPeer2 := newDummyPeer()
	amount = mrand.Intn(100)
	cnt = 1 + mrand.Intn(10)
	for i := 0; i < cnt; i++ {
		swap.Add(0-int64(amount), testPeer2.Peer)
	}
	expectedBalance = int64(0 - (cnt * amount))
	realBalance = swap.balances[testPeer2.ID()]
	if expectedBalance != realBalance {
		t.Fatal(fmt.Sprintf("After %d debits of %d, expected balance to be: %d, but is: %d", cnt, amount, expectedBalance, realBalance))
	}

//借贷混合
	amount1 := mrand.Intn(100)
	amount2 := mrand.Intn(55)
	amount3 := mrand.Intn(999)
	swap.Add(int64(amount1), testPeer2.Peer)
	swap.Add(int64(0-amount2), testPeer2.Peer)
	swap.Add(int64(0-amount3), testPeer2.Peer)

	expectedBalance = expectedBalance + int64(amount1-amount2-amount3)
	realBalance = swap.balances[testPeer2.ID()]

	if expectedBalance != realBalance {
		t.Fatal(fmt.Sprintf("After mixed debits and credits, expected balance to be: %d, but is: %d", expectedBalance, realBalance))
	}
}

//尝试从状态存储恢复平衡
//这是通过创建一个节点来模拟的，
//给它分配一个任意的平衡，
//然后关闭状态存储。
//然后我们重新打开国营商店检查一下
//余额还是一样的
func TestRestoreBalanceFromStateStore(t *testing.T) {
//创建测试交换帐户
	swap, testDir := createTestSwap(t)
	defer os.RemoveAll(testDir)

	testPeer := newDummyPeer()
	swap.balances[testPeer.ID()] = -8888

	tmpBalance := swap.balances[testPeer.ID()]
	swap.stateStore.Put(testPeer.ID().String(), &tmpBalance)

	swap.stateStore.Close()
	swap.stateStore = nil

	stateStore, err := state.NewDBStore(testDir)
	if err != nil {
		t.Fatal(err)
	}

	var newBalance int64
	stateStore.Get(testPeer.ID().String(), &newBalance)

//比较余额
	if tmpBalance != newBalance {
		t.Fatal(fmt.Sprintf("Unexpected balance value after sending cheap message test. Expected balance: %d, balance is: %d",
			tmpBalance, newBalance))
	}
}

//创建测试交换帐户
//为持久性和交换帐户创建StateStore
func createTestSwap(t *testing.T) (*Swap, string) {
	dir, err := ioutil.TempDir("", "swap_test_store")
	if err != nil {
		t.Fatal(err)
	}
	stateStore, err2 := state.NewDBStore(dir)
	if err2 != nil {
		t.Fatal(err2)
	}
	swap := New(stateStore)
	return swap, dir
}

type dummyPeer struct {
	*protocols.Peer
}

//创建虚拟协议。使用虚拟msgreadwriter进行对等
func newDummyPeer() *dummyPeer {
	id := adapters.RandomNodeConfig().ID
	protoPeer := protocols.NewPeer(p2p.NewPeer(id, "testPeer", nil), nil, nil)
	dummy := &dummyPeer{
		Peer: protoPeer,
	}
	return dummy
}
