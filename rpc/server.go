
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

package rpc

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/log"
)

const MetadataApi = "rpc"

//codecopy指定此codec支持哪种类型的消息
type CodecOption int

const (
//OptionMethodInvocation表示编解码器支持RPC方法调用
	OptionMethodInvocation CodecOption = 1 << iota

//选项订阅表示编解码器支持RPC通知
OptionSubscriptions = 1 << iota //支持酒吧子
)

//NewServer将创建一个没有注册处理程序的新服务器实例。
func NewServer() *Server {
	server := &Server{
		services: make(serviceRegistry),
		codecs:   mapset.NewSet(),
		run:      1,
	}

//注册一个默认服务，该服务将提供有关RPC服务的元信息，如服务和
//它提供的方法。
	rpcService := &RPCService{server}
	server.RegisterName(MetadataApi, rpcService)

	return server
}

//rpcservice提供有关服务器的元信息。
//例如，提供有关加载模块的信息。
type RPCService struct {
	server *Server
}

//模块返回带有版本号的RPC服务列表
func (s *RPCService) Modules() map[string]string {
	modules := make(map[string]string)
	for name := range s.server.services {
		modules[name] = "1.0"
	}
	return modules
}

//registername将在给定名称下为给定的rcvr类型创建服务。当给定的RCVR上没有方法时
//匹配条件以成为RPC方法或订阅，将返回错误。否则，新服务是
//已创建并添加到此服务器实例服务的服务集合中。
func (s *Server) RegisterName(name string, rcvr interface{}) error {
	if s.services == nil {
		s.services = make(serviceRegistry)
	}

	svc := new(service)
	svc.typ = reflect.TypeOf(rcvr)
	rcvrVal := reflect.ValueOf(rcvr)

	if name == "" {
		return fmt.Errorf("no service name for type %s", svc.typ.String())
	}
	if !isExported(reflect.Indirect(rcvrVal).Type().Name()) {
		return fmt.Errorf("%s is not exported", reflect.Indirect(rcvrVal).Type().Name())
	}

	methods, subscriptions := suitableCallbacks(rcvrVal, svc.typ)

	if len(methods) == 0 && len(subscriptions) == 0 {
		return fmt.Errorf("Service %T doesn't have any suitable methods/subscriptions to expose", rcvr)
	}

//已在给定名称下注册了以前的服务，合并方法/订阅
	if regsvc, present := s.services[name]; present {
		for _, m := range methods {
			regsvc.callbacks[formatName(m.method.Name)] = m
		}
		for _, s := range subscriptions {
			regsvc.subscriptions[formatName(s.method.Name)] = s
		}
		return nil
	}

	svc.name = name
	svc.callbacks, svc.subscriptions = methods, subscriptions

	s.services[svc.name] = svc
	return nil
}

//ServerRequest将从编解码器读取请求，调用RPC回调和
//将响应写入给定的编解码器。
//
//如果singleshot为true，它将处理单个请求，否则它将处理
//直到编解码器在读取请求时返回错误为止的请求（在大多数情况下
//EOF）。当singleshot为false时，它并行执行请求。
func (s *Server) serveRequest(ctx context.Context, codec ServerCodec, singleShot bool, options CodecOption) error {
	var pend sync.WaitGroup

	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			log.Error(string(buf))
		}
		s.codecsMu.Lock()
		s.codecs.Remove(codec)
		s.codecsMu.Unlock()
	}()

//ctx，取消：=context.withcancel（context.background（））
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

//如果编解码器支持通知，则包含回调可以使用的通知程序
//向客户端发送通知。它绑定到编解码器/连接。如果
//连接已关闭通知程序将停止并取消所有活动订阅。
	if options&OptionSubscriptions == OptionSubscriptions {
		ctx = context.WithValue(ctx, notifierKey{}, newNotifier(codec))
	}
	s.codecsMu.Lock()
if atomic.LoadInt32(&s.run) != 1 { //服务器停止
		s.codecsMu.Unlock()
		return &shutdownError{}
	}
	s.codecs.Add(codec)
	s.codecsMu.Unlock()

//测试服务器是否被命令停止
	for atomic.LoadInt32(&s.run) == 1 {
		reqs, batch, err := s.readRequest(codec)
		if err != nil {
//如果发生分析错误，则发送错误
			if err.Error() != "EOF" {
				log.Debug(fmt.Sprintf("read error %v\n", err))
				codec.Write(codec.CreateErrorResponse(nil, err))
			}
//错误或流结束，等待请求并关闭
			pend.Wait()
			return nil
		}

//检查服务器是否被命令关闭并返回错误
//告诉客户他的请求失败了。
		if atomic.LoadInt32(&s.run) != 1 {
			err = &shutdownError{}
			if batch {
				resps := make([]interface{}, len(reqs))
				for i, r := range reqs {
					resps[i] = codec.CreateErrorResponse(&r.id, err)
				}
				codec.Write(resps)
			} else {
				codec.Write(codec.CreateErrorResponse(&reqs[0].id, err))
			}
			return nil
		}
//如果正在执行单次放炮请求，请立即运行并返回
		if singleShot {
			if batch {
				s.execBatch(ctx, codec, reqs)
			} else {
				s.exec(ctx, codec, reqs[0])
			}
			return nil
		}
//对于多发连接，启动Goroutine发球并回环
		pend.Add(1)

		go func(reqs []*serverRequest, batch bool) {
			defer pend.Done()
			if batch {
				s.execBatch(ctx, codec, reqs)
			} else {
				s.exec(ctx, codec, reqs[0])
			}
		}(reqs, batch)
	}
	return nil
}

//ServeDec从编解码器读取传入的请求，调用适当的回调并写入
//使用给定的编解码器返回响应。它将一直阻塞，直到关闭编解码器或服务器
//停止。无论哪种情况，编解码器都是关闭的。
func (s *Server) ServeCodec(codec ServerCodec, options CodecOption) {
	defer codec.Close()
	s.serveRequest(context.Background(), codec, false, options)
}

//ServeSingleRequest从给定的编解码器读取和处理单个RPC请求。它不会
//除非发生不可恢复的错误，否则请关闭编解码器。注意，此方法将在
//已处理单个请求！
func (s *Server) ServeSingleRequest(ctx context.Context, codec ServerCodec, options CodecOption) {
	s.serveRequest(ctx, codec, true, options)
}

//stop将停止读取新请求，等待stoppendingrequestTimeout允许挂起的请求完成，
//关闭将取消挂起请求/订阅的所有编解码器。
func (s *Server) Stop() {
	if atomic.CompareAndSwapInt32(&s.run, 1, 0) {
		log.Debug("RPC Server shutdown initiatied")
		s.codecsMu.Lock()
		defer s.codecsMu.Unlock()
		s.codecs.Each(func(c interface{}) bool {
			c.(ServerCodec).Close()
			return true
		})
	}
}

//CreateSubscription将调用订阅回调并返回订阅ID或错误。
func (s *Server) createSubscription(ctx context.Context, c ServerCodec, req *serverRequest) (ID, error) {
//订阅的第一个参数是可选参数后面的上下文
	args := []reflect.Value{req.callb.rcvr, reflect.ValueOf(ctx)}
	args = append(args, req.args...)
	reply := req.callb.method.Func.Call(args)

if !reply[1].IsNil() { //订阅创建失败
		return "", reply[1].Interface().(error)
	}

	return reply[0].Interface().(*Subscription).ID, nil
}

//句柄执行请求并返回回调的响应。
func (s *Server) handle(ctx context.Context, codec ServerCodec, req *serverRequest) (interface{}, func()) {
	if req.err != nil {
		return codec.CreateErrorResponse(&req.id, req.err), nil
	}

if req.isUnsubscribe { //取消订阅，第一个参数必须是订阅ID
		if len(req.args) >= 1 && req.args[0].Kind() == reflect.String {
			notifier, supported := NotifierFromContext(ctx)
if !supported { //接口不支持订阅（例如http）
				return codec.CreateErrorResponse(&req.id, &callbackError{ErrNotificationsUnsupported.Error()}), nil
			}

			subid := ID(req.args[0].String())
			if err := notifier.unsubscribe(subid); err != nil {
				return codec.CreateErrorResponse(&req.id, &callbackError{err.Error()}), nil
			}

			return codec.CreateResponse(req.id, true), nil
		}
		return codec.CreateErrorResponse(&req.id, &invalidParamsError{"Expected subscription id as first argument"}), nil
	}

	if req.callb.isSubscribe {
		subid, err := s.createSubscription(ctx, codec, req)
		if err != nil {
			return codec.CreateErrorResponse(&req.id, &callbackError{err.Error()}), nil
		}

//在子ID成功发送到客户端后激活订阅
		activateSub := func() {
			notifier, _ := NotifierFromContext(ctx)
			notifier.activate(subid, req.svcname)
		}

		return codec.CreateResponse(req.id, subid), activateSub
	}

//常规RPC调用，准备参数
	if len(req.args) != len(req.callb.argTypes) {
		rpcErr := &invalidParamsError{fmt.Sprintf("%s%s%s expects %d parameters, got %d",
			req.svcname, serviceMethodSeparator, req.callb.method.Name,
			len(req.callb.argTypes), len(req.args))}
		return codec.CreateErrorResponse(&req.id, rpcErr), nil
	}

	arguments := []reflect.Value{req.callb.rcvr}
	if req.callb.hasCtx {
		arguments = append(arguments, reflect.ValueOf(ctx))
	}
	if len(req.args) > 0 {
		arguments = append(arguments, req.args...)
	}

//执行rpc方法并返回结果
	reply := req.callb.method.Func.Call(arguments)
	if len(reply) == 0 {
		return codec.CreateResponse(req.id, nil), nil
	}
if req.callb.errPos >= 0 { //测试方法是否返回错误
		if !reply[req.callb.errPos].IsNil() {
			e := reply[req.callb.errPos].Interface().(error)
			res := codec.CreateErrorResponse(&req.id, &callbackError{e.Error()})
			return res, nil
		}
	}
	return codec.CreateResponse(req.id, reply[0].Interface()), nil
}

//exec执行给定的请求，并使用codec将结果写回。
func (s *Server) exec(ctx context.Context, codec ServerCodec, req *serverRequest) {
	var response interface{}
	var callback func()
	if req.err != nil {
		response = codec.CreateErrorResponse(&req.id, req.err)
	} else {
		response, callback = s.handle(ctx, codec, req)
	}

	if err := codec.Write(response); err != nil {
		log.Error(fmt.Sprintf("%v\n", err))
		codec.Close()
	}

//当请求是订阅请求时，允许激活这些订阅
	if callback != nil {
		callback()
	}
}

//execbatch执行给定的请求，并使用codec将结果写回。
//它只会在处理最后一个请求时写回响应。
func (s *Server) execBatch(ctx context.Context, codec ServerCodec, requests []*serverRequest) {
	responses := make([]interface{}, len(requests))
	var callbacks []func()
	for i, req := range requests {
		if req.err != nil {
			responses[i] = codec.CreateErrorResponse(&req.id, req.err)
		} else {
			var callback func()
			if responses[i], callback = s.handle(ctx, codec, req); callback != nil {
				callbacks = append(callbacks, callback)
			}
		}
	}

	if err := codec.Write(responses); err != nil {
		log.Error(fmt.Sprintf("%v\n", err))
		codec.Close()
	}

//当请求持有多个订阅请求之一时，这将允许激活这些订阅
	for _, c := range callbacks {
		c()
	}
}

//readrequest从编解码器请求下一个（批处理）请求。它会把收藏品送回
//请求数，指示请求是否为批处理、无效的请求标识符和
//无法读取/分析请求时出错。
func (s *Server) readRequest(codec ServerCodec) ([]*serverRequest, bool, Error) {
	reqs, batch, err := codec.ReadRequestHeaders()
	if err != nil {
		return nil, batch, err
	}

	requests := make([]*serverRequest, len(reqs))

//验证请求
	for i, r := range reqs {
		var ok bool
		var svc *service

		if r.err != nil {
			requests[i] = &serverRequest{id: r.id, err: r.err}
			continue
		}

		if r.isPubSub && strings.HasSuffix(r.method, unsubscribeMethodSuffix) {
			requests[i] = &serverRequest{id: r.id, isUnsubscribe: true}
argTypes := []reflect.Type{reflect.TypeOf("")} //预期订阅ID为第一个参数
			if args, err := codec.ParseRequestArguments(argTypes, r.params); err == nil {
				requests[i].args = args
			} else {
				requests[i].err = &invalidParamsError{err.Error()}
			}
			continue
		}

if svc, ok = s.services[r.service]; !ok { //RPC方法不可用
			requests[i] = &serverRequest{id: r.id, err: &methodNotFoundError{r.service, r.method}}
			continue
		}

if r.isPubSub { //eth-subscribe，r.method包含订阅方法名称
			if callb, ok := svc.subscriptions[r.method]; ok {
				requests[i] = &serverRequest{id: r.id, svcname: svc.name, callb: callb}
				if r.params != nil && len(callb.argTypes) > 0 {
					argTypes := []reflect.Type{reflect.TypeOf("")}
					argTypes = append(argTypes, callb.argTypes...)
					if args, err := codec.ParseRequestArguments(argTypes, r.params); err == nil {
requests[i].args = args[1:] //第一个是service.method name，它不是实际参数
					} else {
						requests[i].err = &invalidParamsError{err.Error()}
					}
				}
			} else {
				requests[i] = &serverRequest{id: r.id, err: &methodNotFoundError{r.service, r.method}}
			}
			continue
		}

if callb, ok := svc.callbacks[r.method]; ok { //查找RPC方法
			requests[i] = &serverRequest{id: r.id, svcname: svc.name, callb: callb}
			if r.params != nil && len(callb.argTypes) > 0 {
				if args, err := codec.ParseRequestArguments(callb.argTypes, r.params); err == nil {
					requests[i].args = args
				} else {
					requests[i].err = &invalidParamsError{err.Error()}
				}
			}
			continue
		}

		requests[i] = &serverRequest{id: r.id, err: &methodNotFoundError{r.service, r.method}}
	}

	return requests, batch, nil
}
