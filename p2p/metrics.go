
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

//包含网络层使用的仪表和计时器。

package p2p

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"

	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

const (
MetricsInboundConnects  = "p2p/InboundConnects"  //已注册的入站连接仪表的名称
MetricsInboundTraffic   = "p2p/InboundTraffic"   //已注册的入站流量表的名称
MetricsOutboundConnects = "p2p/OutboundConnects" //已注册的出站连接仪表的名称
MetricsOutboundTraffic  = "p2p/OutboundTraffic"  //已注册的出站流量表的名称

MeteredPeerLimit = 1024 //这些对等点的数量是单独计量的
)

var (
ingressConnectMeter = metrics.NewRegisteredMeter(MetricsInboundConnects, nil)  //计量入口连接
ingressTrafficMeter = metrics.NewRegisteredMeter(MetricsInboundTraffic, nil)   //计量累计入口流量的仪表
egressConnectMeter  = metrics.NewRegisteredMeter(MetricsOutboundConnects, nil) //计量出口连接
egressTrafficMeter  = metrics.NewRegisteredMeter(MetricsOutboundTraffic, nil)  //计量累计出口流量的仪表

PeerIngressRegistry = metrics.NewPrefixedChildRegistry(metrics.EphemeralRegistry, MetricsInboundTraffic+"/")  //包含对等入口的注册表
PeerEgressRegistry  = metrics.NewPrefixedChildRegistry(metrics.EphemeralRegistry, MetricsOutboundTraffic+"/") //包含对等出口的注册表

meteredPeerFeed  event.Feed //对等度量的事件源
meteredPeerCount int32      //实际存储的对等连接计数
)

//MeteredPeerEventType是由计量连接发出的对等事件类型。
type MeteredPeerEventType int

const (
//PeerConnected是当对等机成功时发出的事件类型
//做了握手。
	PeerConnected MeteredPeerEventType = iota

//PeerDisconnected是对等端断开连接时发出的事件类型。
	PeerDisconnected

//PeerHandshakeFailed是对等端失败时发出的事件类型。
//在握手之前进行握手或断开连接。
	PeerHandshakeFailed
)

//MeteredPeerEvent是对等端连接或断开连接时发出的事件。
type MeteredPeerEvent struct {
Type    MeteredPeerEventType //对等事件类型
IP      net.IP               //对等机的IP地址
ID      enode.ID             //对等节点ID
Elapsed time.Duration        //连接和握手/断开连接之间所用的时间
Ingress uint64               //事件发生时的入口计数
Egress  uint64               //事件发生时的出口计数
}

//subscribeMeteredPeerEvent为对等生命周期事件注册订阅
//如果启用了度量集合。
func SubscribeMeteredPeerEvent(ch chan<- MeteredPeerEvent) event.Subscription {
	return meteredPeerFeed.Subscribe(ch)
}

//meteredconn是一个围绕net.conn的包装器，用于测量
//入站和出站网络流量。
type meteredConn struct {
net.Conn //与计量包装的网络连接

connected time.Time //对等机连接时间
ip        net.IP    //对等机的IP地址
id        enode.ID  //对等节点ID

//TrafficMetered表示对等端是否在流量注册中心注册。
//如果计量的对等计数未达到
//对等连接的时刻。
	trafficMetered bool
ingressMeter   metrics.Meter //对等机的读取字节数
egressMeter    metrics.Meter //为对等机的写入字节计数

lock sync.RWMutex //保护计量连接内部的锁
}

//newmeteredconn创建一个新的计量连接，碰撞入口或出口
//连接仪表，也会增加计量的对等计数。如果度量
//系统被禁用或未指定IP地址，此函数返回
//原始对象。
func newMeteredConn(conn net.Conn, ingress bool, ip net.IP) net.Conn {
//如果禁用度量值，则短路
	if !metrics.Enabled {
		return conn
	}
	if ip.IsUnspecified() {
		log.Warn("Peer IP is unspecified")
		return conn
	}
//碰撞连接计数器并包装连接
	if ingress {
		ingressConnectMeter.Mark(1)
	} else {
		egressConnectMeter.Mark(1)
	}
	return &meteredConn{
		Conn:      conn,
		ip:        ip,
		connected: time.Now(),
	}
}

//读取将网络读取委托给基础连接，从而中断公共连接
//和同龄人的入口，一路上的交通米数。
func (c *meteredConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	ingressTrafficMeter.Mark(int64(n))
	c.lock.RLock()
	if c.trafficMetered {
		c.ingressMeter.Mark(int64(n))
	}
	c.lock.RUnlock()
	return n, err
}

//写入将网络写入委托给基础连接，从而中断公共连接
//而同行的出口，一路上的交通量是米。
func (c *meteredConn) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	egressTrafficMeter.Mark(int64(n))
	c.lock.RLock()
	if c.trafficMetered {
		c.egressMeter.Mark(int64(n))
	}
	c.lock.RUnlock()
	return n, err
}

//当对等握手完成时，将调用握手。将对等机注册到
//使用对等机的IP和节点ID的入口和出口流量注册，
//同时发出Connect事件。
func (c *meteredConn) handshakeDone(id enode.ID) {
	if atomic.AddInt32(&meteredPeerCount, 1) >= MeteredPeerLimit {
//不要在流量注册表中注册对等机。
		atomic.AddInt32(&meteredPeerCount, -1)
		c.lock.Lock()
		c.id, c.trafficMetered = id, false
		c.lock.Unlock()
		log.Warn("Metered peer count reached the limit")
	} else {
		key := fmt.Sprintf("%s/%s", c.ip, id.String())
		c.lock.Lock()
		c.id, c.trafficMetered = id, true
		c.ingressMeter = metrics.NewRegisteredMeter(key, PeerIngressRegistry)
		c.egressMeter = metrics.NewRegisteredMeter(key, PeerEgressRegistry)
		c.lock.Unlock()
	}
	meteredPeerFeed.Send(MeteredPeerEvent{
		Type:    PeerConnected,
		IP:      c.ip,
		ID:      id,
		Elapsed: time.Since(c.connected),
	})
}

//close将关闭操作委托给基础连接，取消注册
//来自流量注册中心的对等机发出关闭事件。
func (c *meteredConn) Close() error {
	err := c.Conn.Close()
	c.lock.RLock()
	if c.id == (enode.ID{}) {
//如果对等端在握手之前断开连接。
		c.lock.RUnlock()
		meteredPeerFeed.Send(MeteredPeerEvent{
			Type:    PeerHandshakeFailed,
			IP:      c.ip,
			Elapsed: time.Since(c.connected),
		})
		return err
	}
	id := c.id
	if !c.trafficMetered {
//如果对等端没有在流量注册中心注册。
		c.lock.RUnlock()
		meteredPeerFeed.Send(MeteredPeerEvent{
			Type: PeerDisconnected,
			IP:   c.ip,
			ID:   id,
		})
		return err
	}
	ingress, egress := uint64(c.ingressMeter.Count()), uint64(c.egressMeter.Count())
	c.lock.RUnlock()

//减少计量的对等计数
	atomic.AddInt32(&meteredPeerCount, -1)

//从流量注册表中注销对等机
	key := fmt.Sprintf("%s/%s", c.ip, id)
	PeerIngressRegistry.Unregister(key)
	PeerEgressRegistry.Unregister(key)

	meteredPeerFeed.Send(MeteredPeerEvent{
		Type:    PeerDisconnected,
		IP:      c.ip,
		ID:      id,
		Ingress: ingress,
		Egress:  egress,
	})
	return err
}
