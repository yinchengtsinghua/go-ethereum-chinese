
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

package ethapi

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

type AddrLocker struct {
	mu    sync.Mutex
	locks map[common.Address]*sync.Mutex
}

//lock returns the lock of the given address.
func (l *AddrLocker) lock(address common.Address) *sync.Mutex {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.locks == nil {
		l.locks = make(map[common.Address]*sync.Mutex)
	}
	if _, ok := l.locks[address]; !ok {
		l.locks[address] = new(sync.Mutex)
	}
	return l.locks[address]
}

//lockaddr锁定帐户的mutex。这用于阻止另一个Tx获取
//直到释放锁。互斥体阻止（相同的nonce）
//在签署第一个事务期间被再次读取。
func (l *AddrLocker) LockAddr(address common.Address) {
	l.lock(address).Lock()
}

//unlockaddr解锁给定帐户的互斥体。
func (l *AddrLocker) UnlockAddr(address common.Address) {
	l.lock(address).Unlock()
}
