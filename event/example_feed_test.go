
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

package event_test

import (
	"fmt"

	"github.com/ethereum/go-ethereum/event"
)

func ExampleFeed_acknowledgedEvents() {
//此示例显示如何将send的返回值用于请求/答复
//活动消费者和生产者之间的互动。
	var feed event.Feed
	type ackedEvent struct {
		i   int
		ack chan<- struct{}
	}

//消费者等待feed上的事件并确认处理。
	done := make(chan struct{})
	defer close(done)
	for i := 0; i < 3; i++ {
		ch := make(chan ackedEvent, 100)
		sub := feed.Subscribe(ch)
		go func() {
			defer sub.Unsubscribe()
			for {
				select {
				case ev := <-ch:
fmt.Println(ev.i) //“处理”事件
					ev.ack <- struct{}{}
				case <-done:
					return
				}
			}
		}()
	}

//生产者发送AckedEvent类型的值，增加i的值。
//它在发送下一个事件之前等待所有消费者确认。
	for i := 0; i < 3; i++ {
		acksignal := make(chan struct{})
		n := feed.Send(ackedEvent{i, acksignal})
		for ack := 0; ack < n; ack++ {
			<-acksignal
		}
	}
//输出：
//零
//零
//零
//一
//一
//一
//二
//二
//二
}
