
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
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/state"
)

//swap swarm会计协议
//点对点小额支付系统
//一个节点与每一个对等节点保持单个平衡。
//只有有价格的消息才会被计入
type Swap struct {
stateStore state.Store        //需要Statestore才能在会话之间保持平衡
lock       sync.RWMutex       //锁定余额
balances   map[enode.ID]int64 //每个对等点的平衡图
}

//新建-交换构造函数
func New(stateStore state.Store) (swap *Swap) {
	swap = &Swap{
		stateStore: stateStore,
		balances:   make(map[enode.ID]int64),
	}
	return
}

//swap实现协议。平衡接口
//添加是（唯一）会计功能
func (s *Swap) Add(amount int64, peer *protocols.Peer) (err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

//从状态存储加载现有余额
	err = s.loadState(peer)
	if err != nil && err != state.ErrNotFound {
		return
	}
//调整平衡
//如果金额为负数，则会减少，否则会增加
	s.balances[peer.ID()] += amount
//将新余额保存到状态存储
	peerBalance := s.balances[peer.ID()]
	err = s.stateStore.Put(peer.ID().String(), &peerBalance)

	log.Debug(fmt.Sprintf("balance for peer %s: %s", peer.ID().String(), strconv.FormatInt(peerBalance, 10)))
	return err
}

//GetPeerBalance返回给定对等机的余额
func (swap *Swap) GetPeerBalance(peer enode.ID) (int64, error) {
	swap.lock.RLock()
	defer swap.lock.RUnlock()
	if p, ok := swap.balances[peer]; ok {
		return p, nil
	}
	return 0, errors.New("Peer not found")
}

//状态存储的负载平衡（持久）
func (s *Swap) loadState(peer *protocols.Peer) (err error) {
	var peerBalance int64
	peerID := peer.ID()
//仅当当前实例没有此对等方的
//内存平衡
	if _, ok := s.balances[peerID]; !ok {
		err = s.stateStore.Get(peerID.String(), &peerBalance)
		s.balances[peerID] = peerBalance
	}
	return
}

//清理交换
func (swap *Swap) Close() {
	swap.stateStore.Close()
}
