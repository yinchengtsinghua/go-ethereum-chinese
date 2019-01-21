
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

package rpc

import (
	"context"
	"errors"
	"sync"
)

var (
//当连接不支持通知时，返回errNotificationsUnsupported。
	ErrNotificationsUnsupported = errors.New("notifications not supported")
//找不到给定ID的通知时返回errNotificationNotFound
	ErrSubscriptionNotFound = errors.New("subscription not found")
)

//ID定义用于标识RPC订阅的伪随机数。
type ID string

//订阅由通知程序创建，并与该通知程序紧密相连。客户可以使用
//此订阅要等待客户端的取消订阅请求，请参阅err（）。
type Subscription struct {
	ID        ID
	namespace string
err       chan error //取消订阅时关闭
}

//err返回当客户端发送取消订阅请求时关闭的通道。
func (s *Subscription) Err() <-chan error {
	return s.err
}

//notifierkey用于在连接上下文中存储通知程序。
type notifierKey struct{}

//通知程序与支持订阅的RPC连接紧密相连。
//服务器回调使用通知程序发送通知。
type Notifier struct {
	codec    ServerCodec
	subMu    sync.Mutex
	active   map[ID]*Subscription
	inactive map[ID]*Subscription
buffer   map[ID][]interface{} //未发送的非活动订阅通知
}

//NewNotifier创建可用于发送订阅的新通知程序
//通知客户端。
func newNotifier(codec ServerCodec) *Notifier {
	return &Notifier{
		codec:    codec,
		active:   make(map[ID]*Subscription),
		inactive: make(map[ID]*Subscription),
		buffer:   make(map[ID][]interface{}),
	}
}

//notifierFromContext返回存储在CTX中的notifier值（如果有）。
func NotifierFromContext(ctx context.Context) (*Notifier, bool) {
	n, ok := ctx.Value(notifierKey{}).(*Notifier)
	return n, ok
}

//CreateSubscription返回耦合到
//RPC连接。默认情况下，订阅不活动，通知
//删除，直到订阅标记为活动。这样做了
//由RPC服务器在订阅ID发送到客户端之后发送。
func (n *Notifier) CreateSubscription() *Subscription {
	s := &Subscription{ID: NewID(), err: make(chan error)}
	n.subMu.Lock()
	n.inactive[s.ID] = s
	n.subMu.Unlock()
	return s
}

//通知将给定数据作为有效负载发送给客户机。
//如果发生错误，则关闭RPC连接并返回错误。
func (n *Notifier) Notify(id ID, data interface{}) error {
	n.subMu.Lock()
	defer n.subMu.Unlock()

	if sub, active := n.active[id]; active {
		n.send(sub, data)
	} else {
		n.buffer[id] = append(n.buffer[id], data)
	}
	return nil
}

func (n *Notifier) send(sub *Subscription, data interface{}) error {
	notification := n.codec.CreateNotification(string(sub.ID), sub.namespace, data)
	err := n.codec.Write(notification)
	if err != nil {
		n.codec.Close()
	}
	return err
}

//CLOSED返回在RPC连接关闭时关闭的通道。
func (n *Notifier) Closed() <-chan interface{} {
	return n.codec.Closed()
}

//取消订阅订阅。
//如果找不到订阅，则返回errscriptionNotFound。
func (n *Notifier) unsubscribe(id ID) error {
	n.subMu.Lock()
	defer n.subMu.Unlock()
	if s, found := n.active[id]; found {
		close(s.err)
		delete(n.active, id)
		return nil
	}
	return ErrSubscriptionNotFound
}

//激活启用订阅。在启用订阅之前
//通知被删除。此方法由RPC服务器在
//订阅ID已发送到客户端。这将阻止通知
//在将订阅ID发送到客户端之前发送到客户端。
func (n *Notifier) activate(id ID, namespace string) {
	n.subMu.Lock()
	defer n.subMu.Unlock()

	if sub, found := n.inactive[id]; found {
		sub.namespace = namespace
		n.active[id] = sub
		delete(n.inactive, id)
//发送缓冲通知。
		for _, data := range n.buffer[id] {
			n.send(sub, data)
		}
		delete(n.buffer, id)
	}
}
