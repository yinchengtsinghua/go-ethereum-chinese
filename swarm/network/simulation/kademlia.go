
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

package simulation

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/swarm/network"
)

//BucketkeyKademlia是用于存储Kademlia的密钥
//特定节点的实例，通常位于servicefunc函数内部。
var BucketKeyKademlia BucketKey = "kademlia"

//等到所有花环的健康都是真实的时候，Waittillhealthy才会停止。
//如果错误不为零，则返回发现不健康的卡德米利亚地图。
//TODO:检查Kademilia深度计算逻辑更改后的正确性
func (s *Simulation) WaitTillHealthy(ctx context.Context) (ill map[enode.ID]*network.Kademlia, err error) {
//准备Peerpot地图检查卡德米利亚健康
	var ppmap map[string]*network.PeerPot
	kademlias := s.kademlias()
	addrs := make([][]byte, 0, len(kademlias))
//要验证所有花冠是否具有相同的参数
	for _, k := range kademlias {
		addrs = append(addrs, k.BaseAddr())
	}
	ppmap = network.NewPeerPotMap(s.neighbourhoodSize, addrs)

//在检查文件之前，在每个节点上等待健康的kademlia
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	ill = make(map[enode.ID]*network.Kademlia)
	for {
		select {
		case <-ctx.Done():
			return ill, ctx.Err()
		case <-ticker.C:
			for k := range ill {
				delete(ill, k)
			}
			log.Debug("kademlia health check", "addr count", len(addrs))
			for id, k := range kademlias {
//此节点的Peerpot
				addr := common.Bytes2Hex(k.BaseAddr())
				pp := ppmap[addr]
//调用健康的RPC
				h := k.Healthy(pp)
//打印信息
				log.Debug(k.String())
				log.Debug("kademlia", "connectNN", h.ConnectNN, "knowNN", h.KnowNN)
				log.Debug("kademlia", "health", h.ConnectNN && h.KnowNN, "addr", hex.EncodeToString(k.BaseAddr()), "node", id)
				log.Debug("kademlia", "ill condition", !h.ConnectNN, "addr", hex.EncodeToString(k.BaseAddr()), "node", id)
				if !h.ConnectNN {
					ill[id] = k
				}
			}
			if len(ill) == 0 {
				return nil, nil
			}
		}
	}
}

//Kademlias返回设置的所有Kademlia实例
//在模拟桶中。
func (s *Simulation) kademlias() (ks map[enode.ID]*network.Kademlia) {
	items := s.UpNodesItems(BucketKeyKademlia)
	ks = make(map[enode.ID]*network.Kademlia, len(items))
	for id, v := range items {
		k, ok := v.(*network.Kademlia)
		if !ok {
			continue
		}
		ks[id] = k
	}
	return ks
}
