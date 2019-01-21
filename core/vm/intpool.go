
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

package vm

import (
	"math/big"
	"sync"
)

var checkVal = big.NewInt(-42)

const poolLimit = 256

//intpool是一个包含大整数的池，
//可用于所有大的.int操作。
type intPool struct {
	pool *Stack
}

func newIntPool() *intPool {
	return &intPool{pool: newstack()}
}

//get从池中检索一个大整数，如果池为空，则分配一个整数。
//注意，返回的int值是任意的，不会归零！
func (p *intPool) get() *big.Int {
	if p.pool.len() > 0 {
		return p.pool.pop()
	}
	return new(big.Int)
}

//GetZero从池中检索一个大整数，将其设置为零或分配
//如果池是空的，就换一个新的。
func (p *intPool) getZero() *big.Int {
	if p.pool.len() > 0 {
		return p.pool.pop().SetUint64(0)
	}
	return new(big.Int)
}

//Put返回一个分配给池的大int，以便稍后由get调用重用。
//注意，保存为原样的值；既不放置也不获取零，整数都不存在！
func (p *intPool) put(is ...*big.Int) {
	if len(p.pool.data) > poolLimit {
		return
	}
	for _, i := range is {
//VerifyPool是一个生成标志。池验证确保完整性
//通过将值与默认值进行比较来获得整数池的值。
		if verifyPool {
			i.Set(checkVal)
		}
		p.pool.push(i)
	}
}

//Intpool池的默认容量
const poolDefaultCap = 25

//IntpoolPool管理Intpools池。
type intPoolPool struct {
	pools []*intPool
	lock  sync.Mutex
}

var poolOfIntPools = &intPoolPool{
	pools: make([]*intPool, 0, poolDefaultCap),
}

//GET正在寻找可返回的可用池。
func (ipp *intPoolPool) get() *intPool {
	ipp.lock.Lock()
	defer ipp.lock.Unlock()

	if len(poolOfIntPools.pools) > 0 {
		ip := ipp.pools[len(ipp.pools)-1]
		ipp.pools = ipp.pools[:len(ipp.pools)-1]
		return ip
	}
	return newIntPool()
}

//放置已分配GET的池。
func (ipp *intPoolPool) put(ip *intPool) {
	ipp.lock.Lock()
	defer ipp.lock.Unlock()

	if len(ipp.pools) < cap(ipp.pools) {
		ipp.pools = append(ipp.pools, ip)
	}
}
