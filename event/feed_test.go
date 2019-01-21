
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

package event

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestFeedPanics(t *testing.T) {
	{
		var f Feed
		f.Send(int(2))
		want := feedTypeError{op: "Send", got: reflect.TypeOf(uint64(0)), want: reflect.TypeOf(int(0))}
		if err := checkPanic(want, func() { f.Send(uint64(2)) }); err != nil {
			t.Error(err)
		}
	}
	{
		var f Feed
		ch := make(chan int)
		f.Subscribe(ch)
		want := feedTypeError{op: "Send", got: reflect.TypeOf(uint64(0)), want: reflect.TypeOf(int(0))}
		if err := checkPanic(want, func() { f.Send(uint64(2)) }); err != nil {
			t.Error(err)
		}
	}
	{
		var f Feed
		f.Send(int(2))
		want := feedTypeError{op: "Subscribe", got: reflect.TypeOf(make(chan uint64)), want: reflect.TypeOf(make(chan<- int))}
		if err := checkPanic(want, func() { f.Subscribe(make(chan uint64)) }); err != nil {
			t.Error(err)
		}
	}
	{
		var f Feed
		if err := checkPanic(errBadChannel, func() { f.Subscribe(make(<-chan int)) }); err != nil {
			t.Error(err)
		}
	}
	{
		var f Feed
		if err := checkPanic(errBadChannel, func() { f.Subscribe(int(0)) }); err != nil {
			t.Error(err)
		}
	}
}

func checkPanic(want error, fn func()) (err error) {
	defer func() {
		panic := recover()
		if panic == nil {
			err = fmt.Errorf("didn't panic")
		} else if !reflect.DeepEqual(panic, want) {
			err = fmt.Errorf("panicked with wrong error: got %q, want %q", panic, want)
		}
	}()
	fn()
	return nil
}

func TestFeed(t *testing.T) {
	var feed Feed
	var done, subscribed sync.WaitGroup
	subscriber := func(i int) {
		defer done.Done()

		subchan := make(chan int)
		sub := feed.Subscribe(subchan)
		timeout := time.NewTimer(2 * time.Second)
		subscribed.Done()

		select {
		case v := <-subchan:
			if v != 1 {
				t.Errorf("%d: received value %d, want 1", i, v)
			}
		case <-timeout.C:
			t.Errorf("%d: receive timeout", i)
		}

		sub.Unsubscribe()
		select {
		case _, ok := <-sub.Err():
			if ok {
				t.Errorf("%d: error channel not closed after unsubscribe", i)
			}
		case <-timeout.C:
			t.Errorf("%d: unsubscribe timeout", i)
		}
	}

	const n = 1000
	done.Add(n)
	subscribed.Add(n)
	for i := 0; i < n; i++ {
		go subscriber(i)
	}
	subscribed.Wait()
	if nsent := feed.Send(1); nsent != n {
		t.Errorf("first send delivered %d times, want %d", nsent, n)
	}
	if nsent := feed.Send(2); nsent != 0 {
		t.Errorf("second send delivered %d times, want 0", nsent)
	}
	done.Wait()
}

func TestFeedSubscribeSameChannel(t *testing.T) {
	var (
		feed Feed
		done sync.WaitGroup
		ch   = make(chan int)
		sub1 = feed.Subscribe(ch)
		sub2 = feed.Subscribe(ch)
		_    = feed.Subscribe(ch)
	)
	expectSends := func(value, n int) {
		if nsent := feed.Send(value); nsent != n {
			t.Errorf("send delivered %d times, want %d", nsent, n)
		}
		done.Done()
	}
	expectRecv := func(wantValue, n int) {
		for i := 0; i < n; i++ {
			if v := <-ch; v != wantValue {
				t.Errorf("received %d, want %d", v, wantValue)
			}
		}
	}

	done.Add(1)
	go expectSends(1, 3)
	expectRecv(1, 3)
	done.Wait()

	sub1.Unsubscribe()

	done.Add(1)
	go expectSends(2, 2)
	expectRecv(2, 2)
	done.Wait()

	sub2.Unsubscribe()

	done.Add(1)
	go expectSends(3, 1)
	expectRecv(3, 1)
	done.Wait()
}

func TestFeedSubscribeBlockedPost(t *testing.T) {
	var (
		feed   Feed
		nsends = 2000
		ch1    = make(chan int)
		ch2    = make(chan int)
		wg     sync.WaitGroup
	)
	defer wg.Wait()

	feed.Subscribe(ch1)
	wg.Add(nsends)
	for i := 0; i < nsends; i++ {
		go func() {
			feed.Send(99)
			wg.Done()
		}()
	}

	sub2 := feed.Subscribe(ch2)
	defer sub2.Unsubscribe()

//当ch1收到n次时，我们就完成了。
//CH2上的接收数取决于调度。
	for i := 0; i < nsends; {
		select {
		case <-ch1:
			i++
		case <-ch2:
		}
	}
}

func TestFeedUnsubscribeBlockedPost(t *testing.T) {
	var (
		feed   Feed
		nsends = 200
		chans  = make([]chan int, 2000)
		subs   = make([]Subscription, len(chans))
		bchan  = make(chan int)
		bsub   = feed.Subscribe(bchan)
		wg     sync.WaitGroup
	)
	for i := range chans {
		chans[i] = make(chan int, nsends)
	}

//排队发送一些邮件。当bchan不被读取时，这些都不能取得进展。
	wg.Add(nsends)
	for i := 0; i < nsends; i++ {
		go func() {
			feed.Send(99)
			wg.Done()
		}()
	}
//订阅其他频道。
	for i, ch := range chans {
		subs[i] = feed.Subscribe(ch)
	}
//再次取消订阅。
	for _, sub := range subs {
		sub.Unsubscribe()
	}
//取消阻止发送。
	bsub.Unsubscribe()
	wg.Wait()
}

//检查在发送期间取消订阅频道是否有效
//频道已发送。
func TestFeedUnsubscribeSentChan(t *testing.T) {
	var (
		feed Feed
		ch1  = make(chan int)
		ch2  = make(chan int)
		sub1 = feed.Subscribe(ch1)
		sub2 = feed.Subscribe(ch2)
		wg   sync.WaitGroup
	)
	defer sub2.Unsubscribe()

	wg.Add(1)
	go func() {
		feed.Send(0)
		wg.Done()
	}()

//等待ch1上的值。
	<-ch1
//取消订阅ch1，将其从发送案例中删除。
	sub1.Unsubscribe()

//接收CH2，完成发送。
	<-ch2
	wg.Wait()

//再发一次。这应该只发送到ch2，因此等待组将取消阻止
//一旦收到CH2上的值。
	wg.Add(1)
	go func() {
		feed.Send(0)
		wg.Done()
	}()
	<-ch2
	wg.Wait()
}

func TestFeedUnsubscribeFromInbox(t *testing.T) {
	var (
		feed Feed
		ch1  = make(chan int)
		ch2  = make(chan int)
		sub1 = feed.Subscribe(ch1)
		sub2 = feed.Subscribe(ch1)
		sub3 = feed.Subscribe(ch2)
	)
	if len(feed.inbox) != 3 {
		t.Errorf("inbox length != 3 after subscribe")
	}
	if len(feed.sendCases) != 1 {
		t.Errorf("sendCases is non-empty after unsubscribe")
	}

	sub1.Unsubscribe()
	sub2.Unsubscribe()
	sub3.Unsubscribe()
	if len(feed.inbox) != 0 {
		t.Errorf("inbox is non-empty after unsubscribe")
	}
	if len(feed.sendCases) != 1 {
		t.Errorf("sendCases is non-empty after unsubscribe")
	}
}

func BenchmarkFeedSend1000(b *testing.B) {
	var (
		done  sync.WaitGroup
		feed  Feed
		nsubs = 1000
	)
	subscriber := func(ch <-chan int) {
		for i := 0; i < b.N; i++ {
			<-ch
		}
		done.Done()
	}
	done.Add(nsubs)
	for i := 0; i < nsubs; i++ {
		ch := make(chan int, 200)
		feed.Subscribe(ch)
		go subscriber(ch)
	}

//实际基准。
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if feed.Send(i) != nsubs {
			panic("wrong number of sends")
		}
	}

	b.StopTimer()
	done.Wait()
}
