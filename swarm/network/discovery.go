
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
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/swarm/pot"
)

//用于请求和中继节点地址记录的Discovery BZZ扩展

//Peer包装BZZPeer并嵌入Kademlia覆盖连接驱动程序
type Peer struct {
	*BzzPeer
	kad       *Kademlia
sentPeers bool            //是否已将对等机发送到该地址附近
mtx       sync.RWMutex    //
peers     map[string]bool //跟踪发送到对等机的节点记录
depth     uint8           //远程通知的接近顺序为饱和深度
}

//newpeer构造发现对等
func NewPeer(p *BzzPeer, kad *Kademlia) *Peer {
	d := &Peer{
		kad:     kad,
		BzzPeer: p,
		peers:   make(map[string]bool),
	}
//记录所见的远程信息，因此我们从不向对等端发送自己的记录。
	d.seen(p.BzzAddr)
	return d
}

//handlemsg是委托传入消息的消息处理程序
func (d *Peer) HandleMsg(ctx context.Context, msg interface{}) error {
	switch msg := msg.(type) {

	case *peersMsg:
		return d.handlePeersMsg(msg)

	case *subPeersMsg:
		return d.handleSubPeersMsg(msg)

	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}
}

//如果饱和深度更改，notifydepth将向所有连接发送消息
func NotifyDepth(depth uint8, kad *Kademlia) {
	f := func(val *Peer, po int) bool {
		val.NotifyDepth(depth)
		return true
	}
	kad.EachConn(nil, 255, f)
}

//notifypeer通知所有对等方新添加的节点
func NotifyPeer(p *BzzAddr, k *Kademlia) {
	f := func(val *Peer, po int) bool {
		val.NotifyPeer(p, uint8(po))
		return true
	}
	k.EachConn(p.Address(), 255, f)
}

//如果出现以下情况，notifypeer将通知远程节点（收件人）有关对等机的信息：
//对等方的采购订单在收件人的广告深度内
//或者对方比自己更接近对方
//除非在连接会话期间已通知
func (d *Peer) NotifyPeer(a *BzzAddr, po uint8) {
//立即返回
	if (po < d.getDepth() && pot.ProxCmp(d.kad.BaseAddr(), d, a) != 1) || d.seen(a) {
		return
	}
	resp := &peersMsg{
		Peers: []*BzzAddr{a},
	}
	go d.Send(context.TODO(), resp)
}

//notifydepth向接收者发送一个子程序msg，通知他们
//饱和深度的变化
func (d *Peer) NotifyDepth(po uint8) {
	go d.Send(context.TODO(), &subPeersMsg{Depth: po})
}

/*
peersmsg是传递对等信息的消息
它始终是对peersrequestmsg的响应

对等地址的编码与DEVP2P基本协议对等机相同。
消息：[IP，端口，节点ID]，
请注意，节点的文件存储地址不是nodeid，而是nodeid的哈希。

TODO：
为了减轻伪对等端消息的影响，请求应该被记住。
应检查响应的正确性。

如果响应中对等端的proxybin不正确，则发送方应
断开的
**/


//peersmsg封装了一组对等地址
//用于交流已知的对等点
//与引导连接和更新对等集相关
type peersMsg struct {
	Peers []*BzzAddr
}

//字符串漂亮打印一个peersmsg
func (msg peersMsg) String() string {
	return fmt.Sprintf("%T: %v", msg, msg.Peers)
}

//接收对等集时协议调用的handlepeersmsg（用于目标地址）
//节点列表（[]peeraddr in peerasmsg）使用
//寄存器接口方法
func (d *Peer) handlePeersMsg(msg *peersMsg) error {
//注册所有地址
	if len(msg.Peers) == 0 {
		return nil
	}

	for _, a := range msg.Peers {
		d.seen(a)
		NotifyPeer(a, d.kad)
	}
	return d.kad.Register(msg.Peers...)
}

//子进程消息正在通信对等进程覆盖表的深度。
type subPeersMsg struct {
	Depth uint8
}

//字符串返回漂亮的打印机
func (msg subPeersMsg) String() string {
	return fmt.Sprintf("%T: request peers > PO%02d. ", msg, msg.Depth)
}

func (d *Peer) handleSubPeersMsg(msg *subPeersMsg) error {
	if !d.sentPeers {
		d.setDepth(msg.Depth)
		var peers []*BzzAddr
		d.kad.EachConn(d.Over(), 255, func(p *Peer, po int) bool {
			if pob, _ := Pof(d, d.kad.BaseAddr(), 0); pob > po {
				return false
			}
			if !d.seen(p.BzzAddr) {
				peers = append(peers, p.BzzAddr)
			}
			return true
		})
		if len(peers) > 0 {
			go d.Send(context.TODO(), &peersMsg{Peers: peers})
		}
	}
	d.sentPeers = true
	return nil
}

//seen获取对等地址并检查是否已将其发送到对等。
//如果没有，则将对等机标记为已发送
func (d *Peer) seen(p *BzzAddr) bool {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	k := string(p.Address())
	if d.peers[k] {
		return true
	}
	d.peers[k] = true
	return false
}

func (d *Peer) getDepth() uint8 {
	d.mtx.RLock()
	defer d.mtx.RUnlock()
	return d.depth
}

func (d *Peer) setDepth(depth uint8) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	d.depth = depth
}
