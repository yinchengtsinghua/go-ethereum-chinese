
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

package simulations

import (
	"fmt"
	"time"
)

//EventType是模拟网络发出的事件类型
type EventType string

const (
//EventTypeNode是当节点为
//创建、启动或停止
	EventTypeNode EventType = "node"

//EventTypeConn是连接时发出的事件类型
//在两个节点之间建立或删除
	EventTypeConn EventType = "conn"

//eventtypmsg是p2p消息时发出的事件类型。
//在两个节点之间发送
	EventTypeMsg EventType = "msg"
)

//事件是模拟网络发出的事件
type Event struct {
//类型是事件的类型
	Type EventType `json:"type"`

//时间是事件发生的时间
	Time time.Time `json:"time"`

//控件指示事件是否是受控件的结果
//网络中的操作
	Control bool `json:"control"`

//如果类型为EventTypeNode，则设置节点
	Node *Node `json:"node,omitempty"`

//如果类型为eventtypconn，则设置conn
	Conn *Conn `json:"conn,omitempty"`

//如果类型为eventtypmsg，则设置msg。
	Msg *Msg `json:"msg,omitempty"`

//可选提供数据（当前仅用于模拟前端）
	Data interface{} `json:"data"`
}

//NewEvent为给定对象创建一个新事件，该事件应为
//节点、连接或消息。
//
//复制对象以便事件表示对象的状态
//调用NewEvent时。
func NewEvent(v interface{}) *Event {
	event := &Event{Time: time.Now()}
	switch v := v.(type) {
	case *Node:
		event.Type = EventTypeNode
		node := *v
		event.Node = &node
	case *Conn:
		event.Type = EventTypeConn
		conn := *v
		event.Conn = &conn
	case *Msg:
		event.Type = EventTypeMsg
		msg := *v
		event.Msg = &msg
	default:
		panic(fmt.Sprintf("invalid event type: %T", v))
	}
	return event
}

//ControlEvent创建新的控件事件
func ControlEvent(v interface{}) *Event {
	event := NewEvent(v)
	event.Control = true
	return event
}

//字符串返回事件的字符串表示形式
func (e *Event) String() string {
	switch e.Type {
	case EventTypeNode:
		return fmt.Sprintf("<node-event> id: %s up: %t", e.Node.ID().TerminalString(), e.Node.Up)
	case EventTypeConn:
		return fmt.Sprintf("<conn-event> nodes: %s->%s up: %t", e.Conn.One.TerminalString(), e.Conn.Other.TerminalString(), e.Conn.Up)
	case EventTypeMsg:
		return fmt.Sprintf("<msg-event> nodes: %s->%s proto: %s, code: %d, received: %t", e.Msg.One.TerminalString(), e.Msg.Other.TerminalString(), e.Msg.Protocol, e.Msg.Code, e.Msg.Received)
	default:
		return ""
	}
}
