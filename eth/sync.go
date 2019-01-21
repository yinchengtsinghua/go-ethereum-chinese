
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

package eth

import (
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

const (
forceSyncCycle      = 10 * time.Second //强制同步的时间间隔，即使没有可用的对等机
minDesiredPeerCount = 5                //开始同步所需的对等机数量

//这是由TxSyncLoop发送的事务包的目标大小。
//如果单个事务超过此大小，包可能会大于此值。
	txsyncPackSize = 100 * 1024
)

type txsync struct {
	p   *peer
	txs []*types.Transaction
}

//SyncTransactions开始将所有当前挂起的事务发送给给定的对等方。
func (pm *ProtocolManager) syncTransactions(p *peer) {
	var txs types.Transactions
	pending, _ := pm.txpool.Pending()
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	if len(txs) == 0 {
		return
	}
	select {
	case pm.txsyncCh <- &txsync{p, txs}:
	case <-pm.quitSync:
	}
}

//TxSyncLoop负责每个新事务的初始事务同步
//连接。当一个新的对等点出现时，我们会中继所有当前挂起的
//交易。为了尽量减少出口带宽的使用，我们发送
//一次将小数据包中的事务打包到一个对等机。
func (pm *ProtocolManager) txsyncLoop() {
	var (
		pending = make(map[enode.ID]*txsync)
sending = false               //发送是否处于活动状态
pack    = new(txsync)         //正在发送的包
done    = make(chan error, 1) //发送的结果
	)

//发送从同步开始发送一组事务。
	send := func(s *txsync) {
//用达到目标大小的事务填充包。
		size := common.StorageSize(0)
		pack.p = s.p
		pack.txs = pack.txs[:0]
		for i := 0; i < len(s.txs) && size < txsyncPackSize; i++ {
			pack.txs = append(pack.txs, s.txs[i])
			size += s.txs[i].Size()
		}
//删除将发送的事务。
		s.txs = s.txs[:copy(s.txs, s.txs[len(pack.txs):])]
		if len(s.txs) == 0 {
			delete(pending, s.p.ID())
		}
//在后台发送包。
		s.p.Log().Trace("Sending batch of transactions", "count", len(pack.txs), "bytes", size)
		sending = true
		go func() { done <- pack.p.SendTransactions(pack.txs) }()
	}

//选择下一个挂起的同步。
	pick := func() *txsync {
		if len(pending) == 0 {
			return nil
		}
		n := rand.Intn(len(pending)) + 1
		for _, s := range pending {
			if n--; n == 0 {
				return s
			}
		}
		return nil
	}

	for {
		select {
		case s := <-pm.txsyncCh:
			pending[s.p.ID()] = s
			if !sending {
				send(s)
			}
		case err := <-done:
			sending = false
//停止跟踪导致发送失败的对等机。
			if err != nil {
				pack.p.Log().Debug("Transaction send failed", "err", err)
				delete(pending, pack.p.ID())
			}
//安排下一次发送。
			if s := pick(); s != nil {
				send(s)
			}
		case <-pm.quitSync:
			return
		}
	}
}

//同步器负责定期与网络同步，两者都是
//下载哈希和块以及处理公告处理程序。
func (pm *ProtocolManager) syncer() {
//启动并确保清除同步机制
	pm.fetcher.Start()
	defer pm.fetcher.Stop()
	defer pm.downloader.Terminate()

//等待不同事件触发同步操作
	forceSync := time.NewTicker(forceSyncCycle)
	defer forceSync.Stop()

	for {
		select {
		case <-pm.newPeerCh:
//确保我们有同行可供选择，然后同步
			if pm.peers.Len() < minDesiredPeerCount {
				break
			}
			go pm.synchronise(pm.peers.BestPeer())

		case <-forceSync.C:
//即使没有足够的对等点，也强制同步
			go pm.synchronise(pm.peers.BestPeer())

		case <-pm.noMorePeers:
			return
		}
	}
}

//同步尝试同步我们的本地块链与远程对等。
func (pm *ProtocolManager) synchronise(peer *peer) {
//如果没有对等点，则短路
	if peer == nil {
		return
	}
//确保同行的TD高于我们自己的TD
	currentBlock := pm.blockchain.CurrentBlock()
	td := pm.blockchain.GetTd(currentBlock.Hash(), currentBlock.NumberU64())

	pHead, pTd := peer.Head()
	if pTd.Cmp(td) <= 0 {
		return
	}
//否则，尝试与下载程序同步
	mode := downloader.FullSync
	if atomic.LoadUint32(&pm.fastSync) == 1 {
//已显式请求并授予快速同步
		mode = downloader.FastSync
	} else if currentBlock.NumberU64() == 0 && pm.blockchain.CurrentFastBlock().NumberU64() > 0 {
//数据库似乎是空的，因为当前块是Genesis。然而快速
//块在前面，因此在某个点为该节点启用了快速同步。
//唯一可能发生这种情况的场景是用户手动（或通过
//坏块）将快速同步节点回滚到同步点以下。在这种情况下
//但是重新启用快速同步是安全的。
		atomic.StoreUint32(&pm.fastSync, 1)
		mode = downloader.FastSync
	}

	if mode == downloader.FastSync {
//确保我们正在同步的对等机的总难度更高。
		if pm.blockchain.GetTdByHash(pm.blockchain.CurrentFastBlock().Hash()).Cmp(pTd) >= 0 {
			return
		}
	}

//运行同步循环，并禁用快速同步（如果我们已通过透视图块）
	if err := pm.downloader.Synchronise(peer.id, pHead, pTd, mode); err != nil {
		return
	}
	if atomic.LoadUint32(&pm.fastSync) == 1 {
		log.Info("Fast sync complete, auto disabling")
		atomic.StoreUint32(&pm.fastSync, 0)
	}
atomic.StoreUint32(&pm.acceptTxs, 1) //标记初始同步完成
	if head := pm.blockchain.CurrentBlock(); head.NumberU64() > 0 {
//我们已经完成了一个同步循环，通知所有对等方新状态。这条路是
//在需要通知网关节点的星型拓扑网络中至关重要
//它的所有过时的新块的可用性对等。这次失败
//场景通常会出现在私人网络和黑客网络中，
//连接性能下降，但对主网来说也应该是健康的
//更可靠地更新对等点或本地TD状态。
		go pm.BroadcastBlock(head, false)
	}
}
