
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

package les

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/light"
)

//同步器负责定期与网络同步，两者都是
//下载哈希和块以及处理公告处理程序。
func (pm *ProtocolManager) syncer() {
//启动并确保清除同步机制
//pm.fetcher.start（）。
//延迟pm.fetcher.stop（）
	defer pm.downloader.Terminate()

//等待不同事件触发同步操作
//forceSync：=time.tick（forceSyncCycle）
	for {
		select {
		case <-pm.newPeerCh:
   /*//确保要从中选择对等点，然后同步
      如果pm.peers.len（）<mindesiredpeerCount_
       打破
      }
      转到pm.synchronize（pm.peers.bestpeer（））
   **/

  /*ASE<-强制同步：
  //即使没有足够的对等点，也强制同步
  转到pm.synchronize（pm.peers.bestpeer（））
  **/

		case <-pm.noMorePeers:
			return
		}
	}
}

func (pm *ProtocolManager) needToSync(peerHead blockInfo) bool {
	head := pm.blockchain.CurrentHeader()
	currentTd := rawdb.ReadTd(pm.chainDb, head.Hash(), head.Number.Uint64())
	return currentTd != nil && peerHead.Td.Cmp(currentTd) > 0
}

//同步尝试同步我们的本地块链与远程对等。
func (pm *ProtocolManager) synchronise(peer *peer) {
//如果没有对等点，则短路
	if peer == nil {
		return
	}

//确保同行的TD高于我们自己的TD。
	if !pm.needToSync(peer.headBlockInfo()) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	pm.blockchain.(*light.LightChain).SyncCht(ctx)
	pm.downloader.Synchronise(peer.id, peer.Head(), peer.Td(), downloader.LightSync)
}
