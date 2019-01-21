
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2014 Go Ethereum作者
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
	"math/rand"
	"sync"
	"testing"
	"time"
)

type testEvent int

func TestSubCloseUnsub(t *testing.T) {
//这个测试的重点是**不要**恐慌。
	var mux TypeMux
	mux.Stop()
	sub := mux.Subscribe(int(0))
	sub.Unsubscribe()
}

func TestSub(t *testing.T) {
	mux := new(TypeMux)
	defer mux.Stop()

	sub := mux.Subscribe(testEvent(0))
	go func() {
		if err := mux.Post(testEvent(5)); err != nil {
			t.Errorf("Post returned unexpected error: %v", err)
		}
	}()
	ev := <-sub.Chan()

	if ev.Data.(testEvent) != testEvent(5) {
		t.Errorf("Got %v (%T), expected event %v (%T)",
			ev, ev, testEvent(5), testEvent(5))
	}
}

func TestMuxErrorAfterStop(t *testing.T) {
	mux := new(TypeMux)
	mux.Stop()

	sub := mux.Subscribe(testEvent(0))
	if _, isopen := <-sub.Chan(); isopen {
		t.Errorf("subscription channel was not closed")
	}
	if err := mux.Post(testEvent(0)); err != ErrMuxClosed {
		t.Errorf("Post error mismatch, got: %s, expected: %s", err, ErrMuxClosed)
	}
}

func TestUnsubscribeUnblockPost(t *testing.T) {
	mux := new(TypeMux)
	defer mux.Stop()

	sub := mux.Subscribe(testEvent(0))
	unblocked := make(chan bool)
	go func() {
		mux.Post(testEvent(5))
		unblocked <- true
	}()

	select {
	case <-unblocked:
		t.Errorf("Post returned before Unsubscribe")
	default:
		sub.Unsubscribe()
		<-unblocked
	}
}

func TestSubscribeDuplicateType(t *testing.T) {
	mux := new(TypeMux)
	expected := "event: duplicate type event.testEvent in Subscribe"

	defer func() {
		err := recover()
		if err == nil {
			t.Errorf("Subscribe didn't panic for duplicate type")
		} else if err != expected {
			t.Errorf("panic mismatch: got %#v, expected %#v", err, expected)
		}
	}()
	mux.Subscribe(testEvent(1), testEvent(2))
}

func TestMuxConcurrent(t *testing.T) {
	rand.Seed(time.Now().Unix())
	mux := new(TypeMux)
	defer mux.Stop()

	recv := make(chan int)
	poster := func() {
		for {
			err := mux.Post(testEvent(0))
			if err != nil {
				return
			}
		}
	}
	sub := func(i int) {
		time.Sleep(time.Duration(rand.Intn(99)) * time.Millisecond)
		sub := mux.Subscribe(testEvent(0))
		<-sub.Chan()
		sub.Unsubscribe()
		recv <- i
	}

	go poster()
	go poster()
	go poster()
	nsubs := 1000
	for i := 0; i < nsubs; i++ {
		go sub(i)
	}

//等到所有人都被服务了
	counts := make(map[int]int, nsubs)
	for i := 0; i < nsubs; i++ {
		counts[<-recv]++
	}
	for i, count := range counts {
		if count != 1 {
			t.Errorf("receiver %d called %d times, expected only 1 call", i, count)
		}
	}
}

func emptySubscriber(mux *TypeMux) {
	s := mux.Subscribe(testEvent(0))
	go func() {
		for range s.Chan() {
		}
	}()
}

func BenchmarkPost1000(b *testing.B) {
	var (
		mux              = new(TypeMux)
		subscribed, done sync.WaitGroup
		nsubs            = 1000
	)
	subscribed.Add(nsubs)
	done.Add(nsubs)
	for i := 0; i < nsubs; i++ {
		go func() {
			s := mux.Subscribe(testEvent(0))
			subscribed.Done()
			for range s.Chan() {
			}
			done.Done()
		}()
	}
	subscribed.Wait()

//实际基准。
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.Post(testEvent(0))
	}

	b.StopTimer()
	mux.Stop()
	done.Wait()
}

func BenchmarkPostConcurrent(b *testing.B) {
	var mux = new(TypeMux)
	defer mux.Stop()
	emptySubscriber(mux)
	emptySubscriber(mux)
	emptySubscriber(mux)

	var wg sync.WaitGroup
	poster := func() {
		for i := 0; i < b.N; i++ {
			mux.Post(testEvent(0))
		}
		wg.Done()
	}
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go poster()
	}
	wg.Wait()
}

//为了比较
func BenchmarkChanSend(b *testing.B) {
	c := make(chan interface{})
	closed := make(chan struct{})
	go func() {
		for range c {
		}
	}()

	for i := 0; i < b.N; i++ {
		select {
		case c <- i:
		case <-closed:
		}
	}
}
