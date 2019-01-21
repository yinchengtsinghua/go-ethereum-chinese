
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
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

type NotificationTestService struct {
	mu                      sync.Mutex
	unsubscribed            chan string
	gotHangSubscriptionReq  chan struct{}
	unblockHangSubscription chan struct{}
}

func (s *NotificationTestService) Echo(i int) int {
	return i
}

func (s *NotificationTestService) Unsubscribe(subid string) {
	if s.unsubscribed != nil {
		s.unsubscribed <- subid
	}
}

func (s *NotificationTestService) SomeSubscription(ctx context.Context, n, val int) (*Subscription, error) {
	notifier, supported := NotifierFromContext(ctx)
	if !supported {
		return nil, ErrNotificationsUnsupported
	}

//通过显式创建订阅，我们确保将订阅ID发送回客户端
//在第一次订阅之前。调用notify。否则，事件可能会在响应之前发送
//对于eth-subscribe方法。
	subscription := notifier.CreateSubscription()

	go func() {
//测试需要n个事件，如果我们立即开始发送事件，则某些事件
//可能会删除，因为订阅ID可能不会发送到
//客户。
		for i := 0; i < n; i++ {
			if err := notifier.Notify(subscription.ID, val+i); err != nil {
				return
			}
		}

		select {
		case <-notifier.Closed():
		case <-subscription.Err():
		}
		if s.unsubscribed != nil {
			s.unsubscribed <- string(subscription.ID)
		}
	}()

	return subscription, nil
}

//在s.unblockhangsubscription上挂起订阅块
//发送任何东西。
func (s *NotificationTestService) HangSubscription(ctx context.Context, val int) (*Subscription, error) {
	notifier, supported := NotifierFromContext(ctx)
	if !supported {
		return nil, ErrNotificationsUnsupported
	}

	s.gotHangSubscriptionReq <- struct{}{}
	<-s.unblockHangSubscription
	subscription := notifier.CreateSubscription()

	go func() {
		notifier.Notify(subscription.ID, val)
	}()
	return subscription, nil
}

func TestNotifications(t *testing.T) {
	server := NewServer()
	service := &NotificationTestService{unsubscribed: make(chan string)}

	if err := server.RegisterName("eth", service); err != nil {
		t.Fatalf("unable to register test service %v", err)
	}

	clientConn, serverConn := net.Pipe()

	go server.ServeCodec(NewJSONCodec(serverConn), OptionMethodInvocation|OptionSubscriptions)

	out := json.NewEncoder(clientConn)
	in := json.NewDecoder(clientConn)

	n := 5
	val := 12345
	request := map[string]interface{}{
		"id":      1,
		"method":  "eth_subscribe",
		"version": "2.0",
		"params":  []interface{}{"someSubscription", n, val},
	}

//创建订阅
	if err := out.Encode(request); err != nil {
		t.Fatal(err)
	}

	var subid string
	response := jsonSuccessResponse{Result: subid}
	if err := in.Decode(&response); err != nil {
		t.Fatal(err)
	}

	var ok bool
	if _, ok = response.Result.(string); !ok {
		t.Fatalf("expected subscription id, got %T", response.Result)
	}

	for i := 0; i < n; i++ {
		var notification jsonNotification
		if err := in.Decode(&notification); err != nil {
			t.Fatalf("%v", err)
		}

		if int(notification.Params.Result.(float64)) != val+i {
			t.Fatalf("expected %d, got %d", val+i, notification.Params.Result)
		}
	}

clientConn.Close() //导致调用通知取消订阅回调
	select {
	case <-service.unsubscribed:
	case <-time.After(1 * time.Second):
		t.Fatal("Unsubscribe not called after one second")
	}
}

func waitForMessages(t *testing.T, in *json.Decoder, successes chan<- jsonSuccessResponse,
	failures chan<- jsonErrResponse, notifications chan<- jsonNotification, errors chan<- error) {

//读取和分析服务器消息
	for {
		var rmsg json.RawMessage
		if err := in.Decode(&rmsg); err != nil {
			return
		}

		var responses []map[string]interface{}
		if rmsg[0] == '[' {
			if err := json.Unmarshal(rmsg, &responses); err != nil {
				errors <- fmt.Errorf("Received invalid message: %s", rmsg)
				return
			}
		} else {
			var msg map[string]interface{}
			if err := json.Unmarshal(rmsg, &msg); err != nil {
				errors <- fmt.Errorf("Received invalid message: %s", rmsg)
				return
			}
			responses = append(responses, msg)
		}

		for _, msg := range responses {
//确定接收和广播的消息类型
//通过相应的通道
			if _, found := msg["result"]; found {
				successes <- jsonSuccessResponse{
					Version: msg["jsonrpc"].(string),
					Id:      msg["id"],
					Result:  msg["result"],
				}
				continue
			}
			if _, found := msg["error"]; found {
				params := msg["params"].(map[string]interface{})
				failures <- jsonErrResponse{
					Version: msg["jsonrpc"].(string),
					Id:      msg["id"],
					Error:   jsonError{int(params["subscription"].(float64)), params["message"].(string), params["data"]},
				}
				continue
			}
			if _, found := msg["params"]; found {
				params := msg["params"].(map[string]interface{})
				notifications <- jsonNotification{
					Version: msg["jsonrpc"].(string),
					Method:  msg["method"].(string),
					Params:  jsonSubscription{params["subscription"].(string), params["result"]},
				}
				continue
			}
			errors <- fmt.Errorf("Received invalid message: %s", msg)
		}
	}
}

//testsubscriptionmultiplename空间确保订阅可以存在
//对于多个不同的命名空间。
func TestSubscriptionMultipleNamespaces(t *testing.T) {
	var (
		namespaces        = []string{"eth", "shh", "bzz"}
		service           = NotificationTestService{}
		subCount          = len(namespaces) * 2
		notificationCount = 3

		server                 = NewServer()
		clientConn, serverConn = net.Pipe()
		out                    = json.NewEncoder(clientConn)
		in                     = json.NewDecoder(clientConn)
		successes              = make(chan jsonSuccessResponse)
		failures               = make(chan jsonErrResponse)
		notifications          = make(chan jsonNotification)
		errors                 = make(chan error, 10)
	)

//安装并启动服务器
	for _, namespace := range namespaces {
		if err := server.RegisterName(namespace, &service); err != nil {
			t.Fatalf("unable to register test service %v", err)
		}
	}

	go server.ServeCodec(NewJSONCodec(serverConn), OptionMethodInvocation|OptionSubscriptions)
	defer server.Stop()

//等待消息并将其写入给定的通道
	go waitForMessages(t, in, successes, failures, notifications, errors)

//逐个创建订阅
	for i, namespace := range namespaces {
		request := map[string]interface{}{
			"id":      i,
			"method":  fmt.Sprintf("%s_subscribe", namespace),
			"version": "2.0",
			"params":  []interface{}{"someSubscription", notificationCount, i},
		}

		if err := out.Encode(&request); err != nil {
			t.Fatalf("Could not create subscription: %v", err)
		}
	}

//在一批中创建所有订阅
	var requests []interface{}
	for i, namespace := range namespaces {
		requests = append(requests, map[string]interface{}{
			"id":      i,
			"method":  fmt.Sprintf("%s_subscribe", namespace),
			"version": "2.0",
			"params":  []interface{}{"someSubscription", notificationCount, i},
		})
	}

	if err := out.Encode(&requests); err != nil {
		t.Fatalf("Could not create subscription in batch form: %v", err)
	}

	timeout := time.After(30 * time.Second)
	subids := make(map[string]string, subCount)
	count := make(map[string]int, subCount)
	allReceived := func() bool {
		done := len(count) == subCount
		for _, c := range count {
			if c < notificationCount {
				done = false
			}
		}
		return done
	}

	for !allReceived() {
		select {
case suc := <-successes: //已创建订阅
			subids[namespaces[int(suc.Id.(float64))]] = suc.Result.(string)
		case notification := <-notifications:
			count[notification.Params.Subscription]++
		case err := <-errors:
			t.Fatal(err)
		case failure := <-failures:
			t.Errorf("received error: %v", failure.Error)
		case <-timeout:
			for _, namespace := range namespaces {
				subid, found := subids[namespace]
				if !found {
					t.Errorf("subscription for %q not created", namespace)
					continue
				}
				if count, found := count[subid]; !found || count < notificationCount {
					t.Errorf("didn't receive all notifications (%d<%d) in time for namespace %q", count, notificationCount, namespace)
				}
			}
			t.Fatal("timed out")
		}
	}
}
