
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
	"sync"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations"
)

//PeerEvent是Simulation.PeerEvents返回的通道类型。
type PeerEvent struct {
//node id是捕获事件的节点的ID。
	NodeID enode.ID
//PeerID是捕获事件的对等节点的ID。
	PeerID enode.ID
//事件是捕获的事件。
	Event *simulations.Event
//错误是事件监视期间可能发生的错误。
	Error error
}

//PeerEventsFilter定义一个对PeerEvents的筛选器，以排除具有
//定义的属性。使用PeerEventsFilter方法设置所需选项。
type PeerEventsFilter struct {
	eventType simulations.EventType

	connUp *bool

	msgReceive *bool
	protocol   *string
	msgCode    *uint64
}

//NewPeerEventsFilter返回新的PeerEventsFilter实例。
func NewPeerEventsFilter() *PeerEventsFilter {
	return &PeerEventsFilter{}
}

//连接将筛选器设置为两个节点连接时的事件。
func (f *PeerEventsFilter) Connect() *PeerEventsFilter {
	f.eventType = simulations.EventTypeConn
	b := true
	f.connUp = &b
	return f
}

//DROP将筛选器设置为两个节点断开连接时的事件。
func (f *PeerEventsFilter) Drop() *PeerEventsFilter {
	f.eventType = simulations.EventTypeConn
	b := false
	f.connUp = &b
	return f
}

//ReceivedMessages将筛选器设置为仅接收的消息。
func (f *PeerEventsFilter) ReceivedMessages() *PeerEventsFilter {
	f.eventType = simulations.EventTypeMsg
	b := true
	f.msgReceive = &b
	return f
}

//sent messages将筛选器设置为只发送消息。
func (f *PeerEventsFilter) SentMessages() *PeerEventsFilter {
	f.eventType = simulations.EventTypeMsg
	b := false
	f.msgReceive = &b
	return f
}

//协议将筛选器设置为仅一个消息协议。
func (f *PeerEventsFilter) Protocol(p string) *PeerEventsFilter {
	f.eventType = simulations.EventTypeMsg
	f.protocol = &p
	return f
}

//msg code将筛选器设置为仅一个msg代码。
func (f *PeerEventsFilter) MsgCode(c uint64) *PeerEventsFilter {
	f.eventType = simulations.EventTypeMsg
	f.msgCode = &c
	return f
}

//PeerEvents返回由管理PeerEvents捕获的事件通道
//具有提供的nodeid的订阅节点。可以将其他筛选器设置为忽略
//不相关的事件。
func (s *Simulation) PeerEvents(ctx context.Context, ids []enode.ID, filters ...*PeerEventsFilter) <-chan PeerEvent {
	eventC := make(chan PeerEvent)

//等待组以确保已建立对管理对等事件的所有订阅
//在此函数返回之前。
	var subsWG sync.WaitGroup
	for _, id := range ids {
		s.shutdownWG.Add(1)
		subsWG.Add(1)
		go func(id enode.ID) {
			defer s.shutdownWG.Done()

			events := make(chan *simulations.Event)
			sub := s.Net.Events().Subscribe(events)
			defer sub.Unsubscribe()

			subsWG.Done()

			for {
				select {
				case <-ctx.Done():
					if err := ctx.Err(); err != nil {
						select {
						case eventC <- PeerEvent{NodeID: id, Error: err}:
						case <-s.Done():
						}
					}
					return
				case <-s.Done():
					return
				case e := <-events:
//忽略控制事件
					if e.Control {
						continue
					}
match := len(filters) == 0 //如果没有匹配所有事件的筛选器
					for _, f := range filters {
						if f.eventType == simulations.EventTypeConn && e.Conn != nil {
							if *f.connUp != e.Conn.Up {
								continue
							}
//所有连接过滤器参数匹配，中断循环
							match = true
							break
						}
						if f.eventType == simulations.EventTypeMsg && e.Msg != nil {
							if f.msgReceive != nil && *f.msgReceive != e.Msg.Received {
								continue
							}
							if f.protocol != nil && *f.protocol != e.Msg.Protocol {
								continue
							}
							if f.msgCode != nil && *f.msgCode != e.Msg.Code {
								continue
							}
//所有消息过滤器参数匹配，中断循环
							match = true
							break
						}
					}
					var peerID enode.ID
					switch e.Type {
					case simulations.EventTypeConn:
						peerID = e.Conn.One
						if peerID == id {
							peerID = e.Conn.Other
						}
					case simulations.EventTypeMsg:
						peerID = e.Msg.One
						if peerID == id {
							peerID = e.Msg.Other
						}
					}
					if match {
						select {
						case eventC <- PeerEvent{NodeID: id, PeerID: peerID, Event: e}:
						case <-ctx.Done():
							if err := ctx.Err(); err != nil {
								select {
								case eventC <- PeerEvent{NodeID: id, PeerID: peerID, Error: err}:
								case <-s.Done():
								}
							}
							return
						case <-s.Done():
							return
						}
					}
				case err := <-sub.Err():
					if err != nil {
						select {
						case eventC <- PeerEvent{NodeID: id, Error: err}:
						case <-ctx.Done():
							if err := ctx.Err(); err != nil {
								select {
								case eventC <- PeerEvent{NodeID: id, Error: err}:
								case <-s.Done():
								}
							}
							return
						case <-s.Done():
							return
						}
					}
				}
			}
		}(id)
	}

//等待所有订阅
	subsWG.Wait()
	return eventC
}
