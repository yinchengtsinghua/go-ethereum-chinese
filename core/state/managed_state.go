
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
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

type account struct {
	stateObject *stateObject
	nstart      uint64
	nonces      []bool
}

type ManagedState struct {
	*StateDB

	mu sync.RWMutex

	accounts map[common.Address]*account
}

//managedState返回一个新的托管状态，statedb作为它的支持层。
func ManageState(statedb *StateDB) *ManagedState {
	return &ManagedState{
		StateDB:  statedb.Copy(),
		accounts: make(map[common.Address]*account),
	}
}

//SetState sets the backing layer of the managed state
func (ms *ManagedState) SetState(statedb *StateDB) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.StateDB = statedb
}

//removenonce从托管状态中删除了nonce以及所有将来挂起的nonce
func (ms *ManagedState) RemoveNonce(addr common.Address, n uint64) {
	if ms.hasAccount(addr) {
		ms.mu.Lock()
		defer ms.mu.Unlock()

		account := ms.getAccount(addr)
		if n-account.nstart <= uint64(len(account.nonces)) {
			reslice := make([]bool, n-account.nstart)
			copy(reslice, account.nonces[:n-account.nstart])
			account.nonces = reslice
		}
	}
}

//new nonce返回托管帐户的新规范nonce
func (ms *ManagedState) NewNonce(addr common.Address) uint64 {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	account := ms.getAccount(addr)
	for i, nonce := range account.nonces {
		if !nonce {
			return account.nstart + uint64(i)
		}
	}
	account.nonces = append(account.nonces, true)

	return uint64(len(account.nonces)-1) + account.nstart
}

//GETNONCE返回托管或非托管帐户的规范NoCE。
//
//因为getnonce改变了db，所以我们必须获得一个写锁。
func (ms *ManagedState) GetNonce(addr common.Address) uint64 {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.hasAccount(addr) {
		account := ms.getAccount(addr)
		return uint64(len(account.nonces)) + account.nstart
	} else {
		return ms.StateDB.GetNonce(addr)
	}
}

//setnonce为托管状态设置新的规范nonce
func (ms *ManagedState) SetNonce(addr common.Address, nonce uint64) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	so := ms.GetOrNewStateObject(addr)
	so.SetNonce(nonce)

	ms.accounts[addr] = newAccount(so)
}

//hasAccount返回给定地址是否被管理
func (ms *ManagedState) HasAccount(addr common.Address) bool {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.hasAccount(addr)
}

func (ms *ManagedState) hasAccount(addr common.Address) bool {
	_, ok := ms.accounts[addr]
	return ok
}

//填充托管状态
func (ms *ManagedState) getAccount(addr common.Address) *account {
	if account, ok := ms.accounts[addr]; !ok {
		so := ms.GetOrNewStateObject(addr)
		ms.accounts[addr] = newAccount(so)
	} else {
//Always make sure the state account nonce isn't actually higher
//比跟踪的那个。
		so := ms.StateDB.getStateObject(addr)
		if so != nil && uint64(len(account.nonces))+account.nstart < so.Nonce() {
			ms.accounts[addr] = newAccount(so)
		}

	}

	return ms.accounts[addr]
}

func newAccount(so *stateObject) *account {
	return &account{so, so.Nonce(), nil}
}
