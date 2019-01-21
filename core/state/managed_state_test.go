
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

package state

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

var addr = common.BytesToAddress([]byte("test"))

func create() (*ManagedState, *account) {
	statedb, _ := New(common.Hash{}, NewDatabase(ethdb.NewMemDatabase()))
	ms := ManageState(statedb)
	ms.StateDB.SetNonce(addr, 100)
	ms.accounts[addr] = newAccount(ms.StateDB.getStateObject(addr))
	return ms, ms.accounts[addr]
}

func TestNewNonce(t *testing.T) {
	ms, _ := create()

	nonce := ms.NewNonce(addr)
	if nonce != 100 {
		t.Error("expected nonce 100. got", nonce)
	}

	nonce = ms.NewNonce(addr)
	if nonce != 101 {
		t.Error("expected nonce 101. got", nonce)
	}
}

func TestRemove(t *testing.T) {
	ms, account := create()

	nn := make([]bool, 10)
	for i := range nn {
		nn[i] = true
	}
	account.nonces = append(account.nonces, nn...)

	i := uint64(5)
	ms.RemoveNonce(addr, account.nstart+i)
	if len(account.nonces) != 5 {
		t.Error("expected", i, "'th index to be false")
	}
}

func TestReuse(t *testing.T) {
	ms, account := create()

	nn := make([]bool, 10)
	for i := range nn {
		nn[i] = true
	}
	account.nonces = append(account.nonces, nn...)

	i := uint64(5)
	ms.RemoveNonce(addr, account.nstart+i)
	nonce := ms.NewNonce(addr)
	if nonce != 105 {
		t.Error("expected nonce to be 105. got", nonce)
	}
}

func TestRemoteNonceChange(t *testing.T) {
	ms, account := create()
	nn := make([]bool, 10)
	for i := range nn {
		nn[i] = true
	}
	account.nonces = append(account.nonces, nn...)
	ms.NewNonce(addr)

	ms.StateDB.stateObjects[addr].data.Nonce = 200
	nonce := ms.NewNonce(addr)
	if nonce != 200 {
		t.Error("expected nonce after remote update to be", 200, "got", nonce)
	}
	ms.NewNonce(addr)
	ms.NewNonce(addr)
	ms.NewNonce(addr)
	ms.StateDB.stateObjects[addr].data.Nonce = 200
	nonce = ms.NewNonce(addr)
	if nonce != 204 {
		t.Error("expected nonce after remote update to be", 204, "got", nonce)
	}
}

func TestSetNonce(t *testing.T) {
	ms, _ := create()

	var addr common.Address
	ms.SetNonce(addr, 10)

	if ms.GetNonce(addr) != 10 {
		t.Error("Expected nonce of 10, got", ms.GetNonce(addr))
	}

	addr[0] = 1
	ms.StateDB.SetNonce(addr, 1)

	if ms.GetNonce(addr) != 1 {
		t.Error("Expected nonce of 1, got", ms.GetNonce(addr))
	}
}
