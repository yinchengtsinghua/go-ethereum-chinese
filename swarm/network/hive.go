
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

package network

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/state"
)

/*
Hive是Swarm的物流经理

当蜂窝启动时，将启动一个永久循环，
问卡德米利亚
建议对等机到引导连接
**/


//hiveparams保存配置选项以进行配置
type HiveParams struct {
Discovery             bool  //如果不想发现
PeersBroadcastSetSize uint8 //中继时要使用多少对等点
MaxPeersPerRequest    uint8 //对等地址批的最大大小
	KeepAliveInterval     time.Duration
}

//newhiveparams返回hive config，只使用
func NewHiveParams() *HiveParams {
	return &HiveParams{
		Discovery:             true,
		PeersBroadcastSetSize: 3,
		MaxPeersPerRequest:    5,
		KeepAliveInterval:     500 * time.Millisecond,
	}
}

//蜂巢管理群节点的网络连接
type Hive struct {
*HiveParams                   //设置
*Kademlia                     //覆盖连接驱动程序
Store       state.Store       //存储接口，用于跨会话保存对等端
addPeer     func(*enode.Node) //连接到对等机的服务器回调
//簿记
	lock   sync.Mutex
	peers  map[enode.ID]*BzzPeer
	ticker *time.Ticker
}

//new hive构造新的hive
//hiveparams:配置参数
//Kademlia：使用网络拓扑的连接驱动程序
//Statestore：保存会话之间的对等点
func NewHive(params *HiveParams, kad *Kademlia, store state.Store) *Hive {
	return &Hive{
		HiveParams: params,
		Kademlia:   kad,
		Store:      store,
		peers:      make(map[enode.ID]*BzzPeer),
	}
}

//启动星型配置单元，仅在启动时接收p2p.server
//服务器用于基于其nodeid或enode url连接到对等机
//在节点上运行的p2p.server上调用这些
func (h *Hive) Start(server *p2p.Server) error {
	log.Info("Starting hive", "baseaddr", fmt.Sprintf("%x", h.BaseAddr()[:4]))
//如果指定了状态存储，则加载对等端以预填充覆盖通讯簿
	if h.Store != nil {
		log.Info("Detected an existing store. trying to load peers")
		if err := h.loadPeers(); err != nil {
			log.Error(fmt.Sprintf("%08x hive encoutered an error trying to load peers", h.BaseAddr()[:4]))
			return err
		}
	}
//分配p2p.server addpeer函数以连接到对等机
	h.addPeer = server.AddPeer
//保持蜂巢存活的标签
	h.ticker = time.NewTicker(h.KeepAliveInterval)
//这个循环正在执行引导并维护一个健康的表
	go h.connect()
	return nil
}

//stop终止updateLoop并保存对等方
func (h *Hive) Stop() error {
	log.Info(fmt.Sprintf("%08x hive stopping, saving peers", h.BaseAddr()[:4]))
	h.ticker.Stop()
	if h.Store != nil {
		if err := h.savePeers(); err != nil {
			return fmt.Errorf("could not save peers to persistence store: %v", err)
		}
		if err := h.Store.Close(); err != nil {
			return fmt.Errorf("could not close file handle to persistence store: %v", err)
		}
	}
	log.Info(fmt.Sprintf("%08x hive stopped, dropping peers", h.BaseAddr()[:4]))
	h.EachConn(nil, 255, func(p *Peer, _ int) bool {
		log.Info(fmt.Sprintf("%08x dropping peer %08x", h.BaseAddr()[:4], p.Address()[:4]))
		p.Drop(nil)
		return true
	})

	log.Info(fmt.Sprintf("%08x all peers dropped", h.BaseAddr()[:4]))
	return nil
}

//连接是一个永久循环
//在每次迭代中，要求覆盖驱动程序建议要连接到的最首选对等机
//如果需要的话还可以宣传饱和深度
func (h *Hive) connect() {
	for range h.ticker.C {

		addr, depth, changed := h.SuggestPeer()
		if h.Discovery && changed {
			NotifyDepth(uint8(depth), h.Kademlia)
		}
		if addr == nil {
			continue
		}

		log.Trace(fmt.Sprintf("%08x hive connect() suggested %08x", h.BaseAddr()[:4], addr.Address()[:4]))
		under, err := enode.ParseV4(string(addr.Under()))
		if err != nil {
			log.Warn(fmt.Sprintf("%08x unable to connect to bee %08x: invalid node URL: %v", h.BaseAddr()[:4], addr.Address()[:4], err))
			continue
		}
		log.Trace(fmt.Sprintf("%08x attempt to connect to bee %08x", h.BaseAddr()[:4], addr.Address()[:4]))
		h.addPeer(under)
	}
}

//运行协议运行函数
func (h *Hive) Run(p *BzzPeer) error {
	h.trackPeer(p)
	defer h.untrackPeer(p)

	dp := NewPeer(p, h.Kademlia)
	depth, changed := h.On(dp)
//如果我们想要发现，就宣传深度的变化。
	if h.Discovery {
		if changed {
//如果深度改变，发送给所有对等方
			NotifyDepth(depth, h.Kademlia)
		} else {
//否则，只需向新对等发送深度
			dp.NotifyDepth(depth)
		}
		NotifyPeer(p.BzzAddr, h.Kademlia)
	}
	defer h.Off(dp)
	return dp.Run(dp.HandleMsg)
}

func (h *Hive) trackPeer(p *BzzPeer) {
	h.lock.Lock()
	h.peers[p.ID()] = p
	h.lock.Unlock()
}

func (h *Hive) untrackPeer(p *BzzPeer) {
	h.lock.Lock()
	delete(h.peers, p.ID())
	h.lock.Unlock()
}

//p2p.server rpc接口使用nodeinfo函数显示
//协议特定的节点信息
func (h *Hive) NodeInfo() interface{} {
	return h.String()
}

//p2p.server rpc接口使用peerinfo函数来显示
//协议特定信息节点ID引用的任何连接的对等点
func (h *Hive) PeerInfo(id enode.ID) interface{} {
	h.lock.Lock()
	p := h.peers[id]
	h.lock.Unlock()

	if p == nil {
		return nil
	}
	addr := NewAddr(p.Node())
	return struct {
		OAddr hexutil.Bytes
		UAddr hexutil.Bytes
	}{
		OAddr: addr.OAddr,
		UAddr: addr.UAddr,
	}
}

//loadpeers，savepeer实现持久回调/
func (h *Hive) loadPeers() error {
	var as []*BzzAddr
	err := h.Store.Get("peers", &as)
	if err != nil {
		if err == state.ErrNotFound {
			log.Info(fmt.Sprintf("hive %08x: no persisted peers found", h.BaseAddr()[:4]))
			return nil
		}
		return err
	}
	log.Info(fmt.Sprintf("hive %08x: peers loaded", h.BaseAddr()[:4]))

	return h.Register(as...)
}

//savepeer，savepeer实现持久回调/
func (h *Hive) savePeers() error {
	var peers []*BzzAddr
	h.Kademlia.EachAddr(nil, 256, func(pa *BzzAddr, i int) bool {
		if pa == nil {
			log.Warn(fmt.Sprintf("empty addr: %v", i))
			return true
		}
		log.Trace("saving peer", "peer", pa)
		peers = append(peers, pa)
		return true
	})
	if err := h.Store.Put("peers", peers); err != nil {
		return fmt.Errorf("could not save peers: %v", err)
	}
	return nil
}
