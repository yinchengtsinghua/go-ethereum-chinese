
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
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/log"
)

func TestClientRequest(t *testing.T) {
	server := newTestServer("service", new(Service))
	defer server.Stop()
	client := DialInProc(server)
	defer client.Close()

	var resp Result
	if err := client.Call(&resp, "service_echo", "hello", 10, &Args{"world"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(resp, Result{"hello", 10, &Args{"world"}}) {
		t.Errorf("incorrect result %#v", resp)
	}
}

func TestClientBatchRequest(t *testing.T) {
	server := newTestServer("service", new(Service))
	defer server.Stop()
	client := DialInProc(server)
	defer client.Close()

	batch := []BatchElem{
		{
			Method: "service_echo",
			Args:   []interface{}{"hello", 10, &Args{"world"}},
			Result: new(Result),
		},
		{
			Method: "service_echo",
			Args:   []interface{}{"hello2", 11, &Args{"world"}},
			Result: new(Result),
		},
		{
			Method: "no_such_method",
			Args:   []interface{}{1, 2, 3},
			Result: new(int),
		},
	}
	if err := client.BatchCall(batch); err != nil {
		t.Fatal(err)
	}
	wantResult := []BatchElem{
		{
			Method: "service_echo",
			Args:   []interface{}{"hello", 10, &Args{"world"}},
			Result: &Result{"hello", 10, &Args{"world"}},
		},
		{
			Method: "service_echo",
			Args:   []interface{}{"hello2", 11, &Args{"world"}},
			Result: &Result{"hello2", 11, &Args{"world"}},
		},
		{
			Method: "no_such_method",
			Args:   []interface{}{1, 2, 3},
			Result: new(int),
			Error:  &jsonError{Code: -32601, Message: "The method no_such_method_ does not exist/is not available"},
		},
	}
	if !reflect.DeepEqual(batch, wantResult) {
		t.Errorf("batch results mismatch:\ngot %swant %s", spew.Sdump(batch), spew.Sdump(wantResult))
	}
}

//func testclientcancel inproc（t*testing.t）testclientcancel（“inproc”，t）
func TestClientCancelWebsocket(t *testing.T) { testClientCancel("ws", t) }
func TestClientCancelHTTP(t *testing.T)      { testClientCancel("http", t) }
func TestClientCancelIPC(t *testing.T)       { testClientCancel("ipc", t) }

//此测试检查通过callContext发出的请求是否可以通过取消来取消
//语境。
func testClientCancel(transport string, t *testing.T) {
	server := newTestServer("service", new(Service))
	defer server.Stop()

//我们想要实现的是取消上下文
//在请求处理的各个阶段。有趣的案例
//是：
//-拨号时取消
//-执行HTTP请求时取消
//-等待响应时取消
//
//为了触发这些，时间的选择使得连接
//在每个其他呼叫的截止时间内终止（maxkillTimeout
//是2x maxCancelTimeout）。
//
//一旦连接断开，很有可能无法连接
//成功，因为接受延迟了1秒。
	maxContextCancelTimeout := 300 * time.Millisecond
	fl := &flakeyListener{
		maxAcceptDelay: 1 * time.Second,
		maxKillTimeout: 600 * time.Millisecond,
	}

	var client *Client
	switch transport {
	case "ws", "http":
		c, hs := httpTestClient(server, transport, fl)
		defer hs.Close()
		client = c
	case "ipc":
		c, l := ipcTestClient(server, fl)
		defer l.Close()
		client = c
	default:
		panic("unknown transport: " + transport)
	}

//这些测试需要很多时间，一次运行它们。
//您可能希望使用-parallel 1或comment out运行
//如果启用日志记录，则调用T.Parallel。
	t.Parallel()

//实际测试从这里开始。
	var (
		wg       sync.WaitGroup
		nreqs    = 10
		ncallers = 6
	)
	caller := func(index int) {
		defer wg.Done()
		for i := 0; i < nreqs; i++ {
			var (
				ctx     context.Context
				cancel  func()
				timeout = time.Duration(rand.Int63n(int64(maxContextCancelTimeout)))
			)
			if index < ncallers/2 {
//对于一半的调用者，创建一个没有截止日期的上下文
//以后再取消。
				ctx, cancel = context.WithCancel(context.Background())
				time.AfterFunc(timeout, cancel)
			} else {
//对于另一半，创建一个带有截止日期的上下文。这是
//不同，因为上下文期限用于设置套接字写入
//截止日期。
				ctx, cancel = context.WithTimeout(context.Background(), timeout)
			}
//现在使用上下文执行调用。
//这里的关键是没有一个呼叫能成功完成。
			err := client.CallContext(ctx, nil, "service_sleep", 2*maxContextCancelTimeout)
			if err != nil {
				log.Debug(fmt.Sprint("got expected error:", err))
			} else {
				t.Errorf("no error for call with %v wait time", timeout)
			}
			cancel()
		}
	}
	wg.Add(ncallers)
	for i := 0; i < ncallers; i++ {
		go caller(i)
	}
	wg.Wait()
}

func TestClientSubscribeInvalidArg(t *testing.T) {
	server := newTestServer("service", new(Service))
	defer server.Stop()
	client := DialInProc(server)
	defer client.Close()

	check := func(shouldPanic bool, arg interface{}) {
		defer func() {
			err := recover()
			if shouldPanic && err == nil {
				t.Errorf("EthSubscribe should've panicked for %#v", arg)
			}
			if !shouldPanic && err != nil {
				t.Errorf("EthSubscribe shouldn't have panicked for %#v", arg)
				buf := make([]byte, 1024*1024)
				buf = buf[:runtime.Stack(buf, false)]
				t.Error(err)
				t.Error(string(buf))
			}
		}()
		client.EthSubscribe(context.Background(), arg, "foo_bar")
	}
	check(true, nil)
	check(true, 1)
	check(true, (chan int)(nil))
	check(true, make(<-chan int))
	check(false, make(chan int))
	check(false, make(chan<- int))
}

func TestClientSubscribe(t *testing.T) {
	server := newTestServer("eth", new(NotificationTestService))
	defer server.Stop()
	client := DialInProc(server)
	defer client.Close()

	nc := make(chan int)
	count := 10
	sub, err := client.EthSubscribe(context.Background(), nc, "someSubscription", count, 0)
	if err != nil {
		t.Fatal("can't subscribe:", err)
	}
	for i := 0; i < count; i++ {
		if val := <-nc; val != i {
			t.Fatalf("value mismatch: got %d, want %d", val, i)
		}
	}

	sub.Unsubscribe()
	select {
	case v := <-nc:
		t.Fatal("received value after unsubscribe:", v)
	case err := <-sub.Err():
		if err != nil {
			t.Fatalf("Err returned a non-nil error after explicit unsubscribe: %q", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("subscription not closed within 1s after unsubscribe")
	}
}

func TestClientSubscribeCustomNamespace(t *testing.T) {
	namespace := "custom"
	server := newTestServer(namespace, new(NotificationTestService))
	defer server.Stop()
	client := DialInProc(server)
	defer client.Close()

	nc := make(chan int)
	count := 10
	sub, err := client.Subscribe(context.Background(), namespace, nc, "someSubscription", count, 0)
	if err != nil {
		t.Fatal("can't subscribe:", err)
	}
	for i := 0; i < count; i++ {
		if val := <-nc; val != i {
			t.Fatalf("value mismatch: got %d, want %d", val, i)
		}
	}

	sub.Unsubscribe()
	select {
	case v := <-nc:
		t.Fatal("received value after unsubscribe:", v)
	case err := <-sub.Err():
		if err != nil {
			t.Fatalf("Err returned a non-nil error after explicit unsubscribe: %q", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("subscription not closed within 1s after unsubscribe")
	}
}

//在这个测试中，当ethsubscribe
//正在等待响应。
func TestClientSubscribeClose(t *testing.T) {
	service := &NotificationTestService{
		gotHangSubscriptionReq:  make(chan struct{}),
		unblockHangSubscription: make(chan struct{}),
	}
	server := newTestServer("eth", service)
	defer server.Stop()
	client := DialInProc(server)
	defer client.Close()

	var (
		nc   = make(chan int)
		errc = make(chan error)
		sub  *ClientSubscription
		err  error
	)
	go func() {
		sub, err = client.EthSubscribe(context.Background(), nc, "hangSubscription", 999)
		errc <- err
	}()

	<-service.gotHangSubscriptionReq
	client.Close()
	service.unblockHangSubscription <- struct{}{}

	select {
	case err := <-errc:
		if err == nil {
			t.Errorf("EthSubscribe returned nil error after Close")
		}
		if sub != nil {
			t.Error("EthSubscribe returned non-nil subscription after Close")
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("EthSubscribe did not return within 1s after Close")
	}
}

//此测试复制了https://github.com/ethereum/go-ethereum/issues/17837，其中
//当unsubscribe与client.close竞争时，客户机在关闭期间挂起。
func TestClientCloseUnsubscribeRace(t *testing.T) {
	service := &NotificationTestService{}
	server := newTestServer("eth", service)
	defer server.Stop()

	for i := 0; i < 20; i++ {
		client := DialInProc(server)
		nc := make(chan int)
		sub, err := client.EthSubscribe(context.Background(), nc, "someSubscription", 3, 1)
		if err != nil {
			t.Fatal(err)
		}
		go client.Close()
		go sub.Unsubscribe()
		select {
		case <-sub.Err():
		case <-time.After(5 * time.Second):
			t.Fatal("subscription not closed within timeout")
		}
	}
}

//此测试检查当单个订阅服务器
//不读取订阅事件。
func TestClientNotificationStorm(t *testing.T) {
	server := newTestServer("eth", new(NotificationTestService))
	defer server.Stop()

	doTest := func(count int, wantError bool) {
		client := DialInProc(server)
		defer client.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

//订阅服务器。它将开始发送许多通知
//很快。
		nc := make(chan int)
		sub, err := client.EthSubscribe(ctx, nc, "someSubscription", count, 0)
		if err != nil {
			t.Fatal("can't subscribe:", err)
		}
		defer sub.Unsubscribe()

//处理每个通知，尝试在它们之间运行一个调用。
		for i := 0; i < count; i++ {
			select {
			case val := <-nc:
				if val != i {
					t.Fatalf("(%d/%d) unexpected value %d", i, count, val)
				}
			case err := <-sub.Err():
				if wantError && err != ErrSubscriptionQueueOverflow {
					t.Fatalf("(%d/%d) got error %q, want %q", i, count, err, ErrSubscriptionQueueOverflow)
				} else if !wantError {
					t.Fatalf("(%d/%d) got unexpected error %q", i, count, err)
				}
				return
			}
			var r int
			err := client.CallContext(ctx, &r, "eth_echo", i)
			if err != nil {
				if !wantError {
					t.Fatalf("(%d/%d) call error: %v", i, count, err)
				}
				return
			}
		}
	}

	doTest(8000, false)
	doTest(10000, true)
}

func TestClientHTTP(t *testing.T) {
	server := newTestServer("service", new(Service))
	defer server.Stop()

	client, hs := httpTestClient(server, "http", nil)
	defer hs.Close()
	defer client.Close()

//启动并发请求。
	var (
		results    = make([]Result, 100)
		errc       = make(chan error)
		wantResult = Result{"a", 1, new(Args)}
	)
	defer client.Close()
	for i := range results {
		i := i
		go func() {
			errc <- client.Call(&results[i], "service_echo",
				wantResult.String, wantResult.Int, wantResult.Args)
		}()
	}

//等待所有任务完成。
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for i := range results {
		select {
		case err := <-errc:
			if err != nil {
				t.Fatal(err)
			}
		case <-timeout.C:
			t.Fatalf("timeout (got %d/%d) results)", i+1, len(results))
		}
	}

//检查结果。
	for i := range results {
		if !reflect.DeepEqual(results[i], wantResult) {
			t.Errorf("result %d mismatch: got %#v, want %#v", i, results[i], wantResult)
		}
	}
}

func TestClientReconnect(t *testing.T) {
	startServer := func(addr string) (*Server, net.Listener) {
		srv := newTestServer("service", new(Service))
		l, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		go http.Serve(l, srv.WebsocketHandler([]string{"*"}))
		return srv, l
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

//启动服务器和相应的客户机。
	s1, l1 := startServer("127.0.0.1:0")
client, err := DialContext(ctx, "ws://“+l1.addr（）.string（））
	if err != nil {
		t.Fatal("can't dial", err)
	}

//打个电话。这应该可以工作，因为服务器已启动。
	var resp Result
	if err := client.CallContext(ctx, &resp, "service_echo", "", 1, nil); err != nil {
		t.Fatal(err)
	}

//关闭服务器，然后再次尝试呼叫。它不应该起作用。
	l1.Close()
	s1.Stop()
	if err := client.CallContext(ctx, &resp, "service_echo", "", 2, nil); err == nil {
		t.Error("successful call while the server is down")
		t.Logf("resp: %#v", resp)
	}

//留出一些冷却时间，以便我们可以再次收听相同的地址。
	time.Sleep(2 * time.Second)

//重新启动然后再打电话。应重新建立连接。
//我们在这里生成多个调用来检查这个是否以某种方式挂起。
	s2, l2 := startServer(l1.Addr().String())
	defer l2.Close()
	defer s2.Stop()

	start := make(chan struct{})
	errors := make(chan error, 20)
	for i := 0; i < cap(errors); i++ {
		go func() {
			<-start
			var resp Result
			errors <- client.CallContext(ctx, &resp, "service_echo", "", 3, nil)
		}()
	}
	close(start)
	errcount := 0
	for i := 0; i < cap(errors); i++ {
		if err = <-errors; err != nil {
			errcount++
		}
	}
	t.Log("err:", err)
	if errcount > 1 {
		t.Errorf("expected one error after disconnect, got %d", errcount)
	}
}

func newTestServer(serviceName string, service interface{}) *Server {
	server := NewServer()
	if err := server.RegisterName(serviceName, service); err != nil {
		panic(err)
	}
	return server
}

func httpTestClient(srv *Server, transport string, fl *flakeyListener) (*Client, *httptest.Server) {
//创建HTTP服务器。
	var hs *httptest.Server
	switch transport {
	case "ws":
		hs = httptest.NewUnstartedServer(srv.WebsocketHandler([]string{"*"}))
	case "http":
		hs = httptest.NewUnstartedServer(srv)
	default:
		panic("unknown HTTP transport: " + transport)
	}
//根据需要包装侦听器。
	if fl != nil {
		fl.Listener = hs.Listener
		hs.Listener = fl
	}
//连接客户端。
	hs.Start()
client, err := Dial(transport + "://“+hs.listener.addr（）.string（））
	if err != nil {
		panic(err)
	}
	return client, hs
}

func ipcTestClient(srv *Server, fl *flakeyListener) (*Client, net.Listener) {
//在随机端点上侦听。
	endpoint := fmt.Sprintf("go-ethereum-test-ipc-%d-%d", os.Getpid(), rand.Int63())
	if runtime.GOOS == "windows" {
		endpoint = `\\.\pipe\` + endpoint
	} else {
		endpoint = os.TempDir() + "/" + endpoint
	}
	l, err := ipcListen(endpoint)
	if err != nil {
		panic(err)
	}
//将侦听器连接到服务器。
	if fl != nil {
		fl.Listener = l
		l = fl
	}
	go srv.ServeListener(l)
//连接客户端。
	client, err := Dial(endpoint)
	if err != nil {
		panic(err)
	}
	return client, l
}

//flakeylistener在随机超时后终止接受的连接。
type flakeyListener struct {
	net.Listener
	maxKillTimeout time.Duration
	maxAcceptDelay time.Duration
}

func (l *flakeyListener) Accept() (net.Conn, error) {
	delay := time.Duration(rand.Int63n(int64(l.maxAcceptDelay)))
	time.Sleep(delay)

	c, err := l.Listener.Accept()
	if err == nil {
		timeout := time.Duration(rand.Int63n(int64(l.maxKillTimeout)))
		time.AfterFunc(timeout, func() {
			log.Debug(fmt.Sprintf("killing conn %v after %v", c.LocalAddr(), timeout))
			c.Close()
		})
	}
	return c, err
}
