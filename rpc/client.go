
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
	"bytes"
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

var (
	ErrClientQuit                = errors.New("client is closed")
	ErrNoResult                  = errors.New("no result in JSON-RPC response")
	ErrSubscriptionQueueOverflow = errors.New("subscription queue overflow")
)

const (
//超时
	tcpKeepAliveInterval = 30 * time.Second
defaultDialTimeout   = 10 * time.Second //如果上下文没有截止时间，则在拨号时使用
defaultWriteTimeout  = 10 * time.Second //如果上下文没有截止时间，则用于调用
subscribeTimeout     = 5 * time.Second  //总超时eth_subscribe，rpc_模块调用
)

const (
//当订阅服务器无法跟上时，将删除订阅。
//
//这可以通过为通道提供足够大的缓冲区来解决，
//但这在文件中可能不方便也很难解释。另一个问题是
//缓冲通道是指即使不需要缓冲区，缓冲区也是静态的。
//大部分时间。
//
//这里采用的方法是维护每个订阅的链表缓冲区
//按需缩小。如果缓冲区达到以下大小，则订阅为
//下降。
	maxClientSubscriptionBuffer = 20000
)

//batchelem是批处理请求中的元素。
type BatchElem struct {
	Method string
	Args   []interface{}
//结果未显示在此字段中。结果必须设置为
//所需类型的非零指针值，否则响应将为
//丢弃的。
	Result interface{}
//如果服务器返回此请求的错误，或如果
//解组为结果失败。未设置I/O错误。
	Error error
}

//此类型的值可以是JSON-RPC请求、通知、成功响应或
//错误响应。它是哪一个取决于田地。
type jsonrpcMessage struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *jsonError      `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

func (msg *jsonrpcMessage) isNotification() bool {
	return msg.ID == nil && msg.Method != ""
}

func (msg *jsonrpcMessage) isResponse() bool {
	return msg.hasValidID() && msg.Method == "" && len(msg.Params) == 0
}

func (msg *jsonrpcMessage) hasValidID() bool {
	return len(msg.ID) > 0 && msg.ID[0] != '{' && msg.ID[0] != '['
}

func (msg *jsonrpcMessage) String() string {
	b, _ := json.Marshal(msg)
	return string(b)
}

//客户端表示到RPC服务器的连接。
type Client struct {
	idCounter   uint32
	connectFunc func(ctx context.Context) (net.Conn, error)
	isHTTP      bool

//WRITECONN只能安全地访问外部调度，使用
//写入锁定保持。通过发送
//请求操作并通过发送发送完成释放。
	writeConn net.Conn

//派遣
	close       chan struct{}
closing     chan struct{}                  //客户端退出时关闭
didClose    chan struct{}                  //客户端退出时关闭
reconnected chan net.Conn                  //写入/重新连接发送新连接的位置
readErr     chan error                     //读取错误
readResp    chan []*jsonrpcMessage         //读取的有效消息
requestOp   chan *requestOp                //用于注册响应ID
sendDone    chan error                     //信号写入完成，释放写入锁定
respWait    map[string]*requestOp          //主动请求
subs        map[string]*ClientSubscription //活动订阅
}

type requestOp struct {
	ids  []json.RawMessage
	err  error
resp chan *jsonrpcMessage //接收最多len（id）响应
sub  *ClientSubscription  //仅为ethsubscribe请求设置
}

func (op *requestOp) wait(ctx context.Context) (*jsonrpcMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-op.resp:
		return resp, op.err
	}
}

//拨号为给定的URL创建新的客户端。
//
//当前支持的URL方案有“http”、“https”、“ws”和“wss”。如果RAWURL是
//没有URL方案的文件名，使用Unix建立本地套接字连接
//支持的平台上的域套接字和Windows上的命名管道。如果你想
//配置传输选项，使用dialHTTP、dialWebSocket或dialIPC。
//
//对于WebSocket连接，原点设置为本地主机名。
//
//如果连接丢失，客户端将自动重新连接。
func Dial(rawurl string) (*Client, error) {
	return DialContext(context.Background(), rawurl)
}

//DialContext创建一个新的RPC客户端，就像Dial一样。
//
//上下文用于取消或超时初始连接建立。它确实
//不影响与客户的后续交互。
func DialContext(ctx context.Context, rawurl string) (*Client, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "http", "https":
		return DialHTTP(rawurl)
	case "ws", "wss":
		return DialWebsocket(ctx, rawurl, "")
	case "stdio":
		return DialStdIO(ctx)
	case "":
		return DialIPC(ctx, rawurl)
	default:
		return nil, fmt.Errorf("no known transport for URL scheme %q", u.Scheme)
	}
}

func newClient(initctx context.Context, connectFunc func(context.Context) (net.Conn, error)) (*Client, error) {
	conn, err := connectFunc(initctx)
	if err != nil {
		return nil, err
	}
	_, isHTTP := conn.(*httpConn)
	c := &Client{
		writeConn:   conn,
		isHTTP:      isHTTP,
		connectFunc: connectFunc,
		close:       make(chan struct{}),
		closing:     make(chan struct{}),
		didClose:    make(chan struct{}),
		reconnected: make(chan net.Conn),
		readErr:     make(chan error),
		readResp:    make(chan []*jsonrpcMessage),
		requestOp:   make(chan *requestOp),
		sendDone:    make(chan error, 1),
		respWait:    make(map[string]*requestOp),
		subs:        make(map[string]*ClientSubscription),
	}
	if !isHTTP {
		go c.dispatch(conn)
	}
	return c, nil
}

func (c *Client) nextID() json.RawMessage {
	id := atomic.AddUint32(&c.idCounter, 1)
	return []byte(strconv.FormatUint(uint64(id), 10))
}

//supportedmodules调用rpc_modules方法，检索
//服务器上可用的API。
func (c *Client) SupportedModules() (map[string]string, error) {
	var result map[string]string
	ctx, cancel := context.WithTimeout(context.Background(), subscribeTimeout)
	defer cancel()
	err := c.CallContext(ctx, &result, "rpc_modules")
	return result, err
}

//CLOSE关闭客户机，中止任何飞行中的请求。
func (c *Client) Close() {
	if c.isHTTP {
		return
	}
	select {
	case c.close <- struct{}{}:
		<-c.didClose
	case <-c.didClose:
	}
}

//调用使用给定的参数执行JSON-RPC调用，并将其解组为
//如果没有发生错误，则返回结果。
//
//结果必须是一个指针，以便包JSON可以解组到其中。你
//也可以传递nil，在这种情况下，结果将被忽略。
func (c *Client) Call(result interface{}, method string, args ...interface{}) error {
	ctx := context.Background()
	return c.CallContext(ctx, result, method, args...)
}

//callContext使用给定的参数执行JSON-RPC调用。如果上下文是
//在成功返回调用之前取消，callContext立即返回。
//
//结果必须是一个指针，以便包JSON可以解组到其中。你
//也可以传递nil，在这种情况下，结果将被忽略。
func (c *Client) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	msg, err := c.newMessage(method, args...)
	if err != nil {
		return err
	}
	op := &requestOp{ids: []json.RawMessage{msg.ID}, resp: make(chan *jsonrpcMessage, 1)}

	if c.isHTTP {
		err = c.sendHTTP(ctx, op, msg)
	} else {
		err = c.send(ctx, op, msg)
	}
	if err != nil {
		return err
	}

//调度已接受请求，并将在退出时关闭通道。
	switch resp, err := op.wait(ctx); {
	case err != nil:
		return err
	case resp.Error != nil:
		return resp.Error
	case len(resp.Result) == 0:
		return ErrNoResult
	default:
		return json.Unmarshal(resp.Result, &result)
	}
}

//批处理调用将所有给定的请求作为单个批处理发送，并等待服务器
//为所有人返回响应。
//
//与调用不同，批调用只返回I/O错误。任何特定于的错误
//通过相应批处理元素的错误字段报告请求。
//
//注意，批处理调用不能在服务器端以原子方式执行。
func (c *Client) BatchCall(b []BatchElem) error {
	ctx := context.Background()
	return c.BatchCallContext(ctx, b)
}

//批处理调用将所有给定的请求作为单个批处理发送，并等待服务器
//为所有人返回响应。等待持续时间由
//上下文的最后期限。
//
//与callContext不同，batchcallContext只返回已发生的错误
//发送请求时。任何特定于请求的错误都会通过
//对应批处理元素的错误字段。
//
//注意，批处理调用不能在服务器端以原子方式执行。
func (c *Client) BatchCallContext(ctx context.Context, b []BatchElem) error {
	msgs := make([]*jsonrpcMessage, len(b))
	op := &requestOp{
		ids:  make([]json.RawMessage, len(b)),
		resp: make(chan *jsonrpcMessage, len(b)),
	}
	for i, elem := range b {
		msg, err := c.newMessage(elem.Method, elem.Args...)
		if err != nil {
			return err
		}
		msgs[i] = msg
		op.ids[i] = msg.ID
	}

	var err error
	if c.isHTTP {
		err = c.sendBatchHTTP(ctx, op, msgs)
	} else {
		err = c.send(ctx, op, msgs)
	}

//等待所有响应返回。
	for n := 0; n < len(b) && err == nil; n++ {
		var resp *jsonrpcMessage
		resp, err = op.wait(ctx)
		if err != nil {
			break
		}
//找到与此响应对应的元素。
//由于调度，元素被保证存在
//只向我们的频道发送有效的ID。
		var elem *BatchElem
		for i := range msgs {
			if bytes.Equal(msgs[i].ID, resp.ID) {
				elem = &b[i]
				break
			}
		}
		if resp.Error != nil {
			elem.Error = resp.Error
			continue
		}
		if len(resp.Result) == 0 {
			elem.Error = ErrNoResult
			continue
		}
		elem.Error = json.Unmarshal(resp.Result, elem.Result)
	}
	return err
}

//ethsubscribe在“eth”名称空间下注册一个订阅。
func (c *Client) EthSubscribe(ctx context.Context, channel interface{}, args ...interface{}) (*ClientSubscription, error) {
	return c.Subscribe(ctx, "eth", channel, args...)
}

//shhsubscribe在“shh”名称空间下注册一个订阅。
func (c *Client) ShhSubscribe(ctx context.Context, channel interface{}, args ...interface{}) (*ClientSubscription, error) {
	return c.Subscribe(ctx, "shh", channel, args...)
}

//subscribe使用给定的参数调用“<namespace>\u subscribe”方法，
//注册订阅。订阅的服务器通知为
//发送到给定的频道。通道的元素类型必须与
//订阅返回的内容类型应为。
//
//context参数取消设置订阅但没有的RPC请求
//订阅返回后对订阅的影响。
//
//缓慢的订户最终将被删除。客户端缓冲区最多8000个通知
//在考虑用户死亡之前。订阅错误通道将接收
//errSubscriptionQueueOverflow。在通道上使用足够大的缓冲区或确保
//通道通常至少有一个读卡器来防止这个问题。
func (c *Client) Subscribe(ctx context.Context, namespace string, channel interface{}, args ...interface{}) (*ClientSubscription, error) {
//首先检查通道类型。
	chanVal := reflect.ValueOf(channel)
	if chanVal.Kind() != reflect.Chan || chanVal.Type().ChanDir()&reflect.SendDir == 0 {
		panic("first argument to Subscribe must be a writable channel")
	}
	if chanVal.IsNil() {
		panic("channel given to Subscribe must not be nil")
	}
	if c.isHTTP {
		return nil, ErrNotificationsUnsupported
	}

	msg, err := c.newMessage(namespace+subscribeMethodSuffix, args...)
	if err != nil {
		return nil, err
	}
	op := &requestOp{
		ids:  []json.RawMessage{msg.ID},
		resp: make(chan *jsonrpcMessage),
		sub:  newClientSubscription(c, namespace, chanVal),
	}

//发送订阅请求。
//响应的到达和有效性在Sub.Quit上发出信号。
	if err := c.send(ctx, op, msg); err != nil {
		return nil, err
	}
	if _, err := op.wait(ctx); err != nil {
		return nil, err
	}
	return op.sub, nil
}

func (c *Client) newMessage(method string, paramsIn ...interface{}) (*jsonrpcMessage, error) {
	params, err := json.Marshal(paramsIn)
	if err != nil {
		return nil, err
	}
	return &jsonrpcMessage{Version: "2.0", ID: c.nextID(), Method: method, Params: params}, nil
}

//发送寄存器op和调度循环，然后在连接上发送msg。
//如果发送失败，则注销OP。
func (c *Client) send(ctx context.Context, op *requestOp, msg interface{}) error {
	select {
	case c.requestOp <- op:
		log.Trace("", "msg", log.Lazy{Fn: func() string {
			return fmt.Sprint("sending ", msg)
		}})
		err := c.write(ctx, msg)
		c.sendDone <- err
		return err
	case <-ctx.Done():
//如果客户端过载或无法跟上，就会发生这种情况。
//订阅通知。
		return ctx.Err()
	case <-c.closing:
		return ErrClientQuit
	case <-c.didClose:
		return ErrClientQuit
	}
}

func (c *Client) write(ctx context.Context, msg interface{}) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultWriteTimeout)
	}
//上一次写入失败。尝试建立新连接。
	if c.writeConn == nil {
		if err := c.reconnect(ctx); err != nil {
			return err
		}
	}
	c.writeConn.SetWriteDeadline(deadline)
	err := json.NewEncoder(c.writeConn).Encode(msg)
	c.writeConn.SetWriteDeadline(time.Time{})
	if err != nil {
		c.writeConn = nil
	}
	return err
}

func (c *Client) reconnect(ctx context.Context) error {
	newconn, err := c.connectFunc(ctx)
	if err != nil {
		log.Trace(fmt.Sprintf("reconnect failed: %v", err))
		return err
	}
	select {
	case c.reconnected <- newconn:
		c.writeConn = newconn
		return nil
	case <-c.didClose:
		newconn.Close()
		return ErrClientQuit
	}
}

//调度是客户机的主循环。
//它向等待调用和批调用发送读取消息
//注册订阅的订阅通知。
func (c *Client) dispatch(conn net.Conn) {
//生成初始读取循环。
	go c.read(conn)

	var (
lastOp        *requestOp    //跟踪上次发送操作
requestOpLock = c.requestOp //保持发送锁时为零
reading       = true        //如果为真，则运行读取循环
	)
	defer close(c.didClose)
	defer func() {
		close(c.closing)
		c.closeRequestOps(ErrClientQuit)
		conn.Close()
		if reading {
//清空读取通道，直到读取结束。
			for {
				select {
				case <-c.readResp:
				case <-c.readErr:
					return
				}
			}
		}
	}()

	for {
		select {
		case <-c.close:
			return

//读取路径。
		case batch := <-c.readResp:
			for _, msg := range batch {
				switch {
				case msg.isNotification():
					log.Trace("", "msg", log.Lazy{Fn: func() string {
						return fmt.Sprint("<-readResp: notification ", msg)
					}})
					c.handleNotification(msg)
				case msg.isResponse():
					log.Trace("", "msg", log.Lazy{Fn: func() string {
						return fmt.Sprint("<-readResp: response ", msg)
					}})
					c.handleResponse(msg)
				default:
					log.Debug("", "msg", log.Lazy{Fn: func() string {
						return fmt.Sprint("<-readResp: dropping weird message", msg)
					}})
//托多：也许接近
				}
			}

		case err := <-c.readErr:
			log.Debug("<-readErr", "err", err)
			c.closeRequestOps(err)
			conn.Close()
			reading = false

		case newconn := <-c.reconnected:
			log.Debug("<-reconnected", "reading", reading, "remote", conn.RemoteAddr())
			if reading {
//等待上一个读取循环退出。这是一个罕见的病例。
				conn.Close()
				<-c.readErr
			}
			go c.read(newconn)
			reading = true
			conn = newconn

//发送路径。
		case op := <-requestOpLock:
//停止侦听进一步的发送操作，直到当前操作完成。
			requestOpLock = nil
			lastOp = op
			for _, id := range op.ids {
				c.respWait[string(id)] = op
			}

		case err := <-c.sendDone:
			if err != nil {
//删除上次发送的响应处理程序。我们把它们移到这里
//因为错误已经在call或batchcall中处理。当
//读取循环停止，它将发出所有其他当前操作的信号。
				for _, id := range lastOp.ids {
					delete(c.respWait, string(id))
				}
			}
//再次收听发送操作。
			requestOpLock = c.requestOp
			lastOp = nil
		}
	}
}

//closerequestops取消阻止挂起的发送操作和活动订阅。
func (c *Client) closeRequestOps(err error) {
	didClose := make(map[*requestOp]bool)

	for id, op := range c.respWait {
//删除op，以便以后的调用不会再次关闭op.resp。
		delete(c.respWait, id)

		if !didClose[op] {
			op.err = err
			close(op.resp)
			didClose[op] = true
		}
	}
	for id, sub := range c.subs {
		delete(c.subs, id)
		sub.quitWithError(err, false)
	}
}

func (c *Client) handleNotification(msg *jsonrpcMessage) {
	if !strings.HasSuffix(msg.Method, notificationMethodSuffix) {
		log.Debug("dropping non-subscription message", "msg", msg)
		return
	}
	var subResult struct {
		ID     string          `json:"subscription"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(msg.Params, &subResult); err != nil {
		log.Debug("dropping invalid subscription message", "msg", msg)
		return
	}
	if c.subs[subResult.ID] != nil {
		c.subs[subResult.ID].deliver(subResult.Result)
	}
}

func (c *Client) handleResponse(msg *jsonrpcMessage) {
	op := c.respWait[string(msg.ID)]
	if op == nil {
		log.Debug("unsolicited response", "msg", msg)
		return
	}
	delete(c.respWait, string(msg.ID))
//对于正常响应，只需将响应转发到call/batchcall。
	if op.sub == nil {
		op.resp <- msg
		return
	}
//对于订阅响应，如果服务器
//表示成功。ethsubscribe在任何情况下都可以通过
//Op.Resp频道。
	defer close(op.resp)
	if msg.Error != nil {
		op.err = msg.Error
		return
	}
	if op.err = json.Unmarshal(msg.Result, &op.sub.subid); op.err == nil {
		go op.sub.start()
		c.subs[op.sub.subid] = op.sub
	}
}

//阅读发生在一个专门的Goroutine上。

func (c *Client) read(conn net.Conn) error {
	var (
		buf json.RawMessage
		dec = json.NewDecoder(conn)
	)
	readMessage := func() (rs []*jsonrpcMessage, err error) {
		buf = buf[:0]
		if err = dec.Decode(&buf); err != nil {
			return nil, err
		}
		if isBatch(buf) {
			err = json.Unmarshal(buf, &rs)
		} else {
			rs = make([]*jsonrpcMessage, 1)
			err = json.Unmarshal(buf, &rs[0])
		}
		return rs, err
	}

	for {
		resp, err := readMessage()
		if err != nil {
			c.readErr <- err
			return err
		}
		c.readResp <- resp
	}
}

//订阅。

//客户端订阅表示通过ethsubscribe建立的订阅。
type ClientSubscription struct {
	client    *Client
	etype     reflect.Type
	channel   reflect.Value
	namespace string
	subid     string
	in        chan json.RawMessage

quitOnce sync.Once     //确保退出关闭一次
quit     chan struct{} //退出订阅时将关闭Quit
errOnce  sync.Once     //确保err关闭一次
	err      chan error
}

func newClientSubscription(c *Client, namespace string, channel reflect.Value) *ClientSubscription {
	sub := &ClientSubscription{
		client:    c,
		namespace: namespace,
		etype:     channel.Type().Elem(),
		channel:   channel,
		quit:      make(chan struct{}),
		err:       make(chan error, 1),
		in:        make(chan json.RawMessage),
	}
	return sub
}

//err返回订阅错误通道。err的预期用途是
//客户端连接意外关闭时重新订阅。
//
//当订阅到期时，错误通道接收到一个值。
//出错。如果调用了close，则接收到的错误为nil。
//在基础客户端上，没有发生其他错误。
//
//当对订阅调用Unsubscribe时，错误通道将关闭。
func (sub *ClientSubscription) Err() <-chan error {
	return sub.err
}

//取消订阅取消订阅通知并关闭错误通道。
//它可以安全地被多次调用。
func (sub *ClientSubscription) Unsubscribe() {
	sub.quitWithError(nil, true)
	sub.errOnce.Do(func() { close(sub.err) })
}

func (sub *ClientSubscription) quitWithError(err error, unsubscribeServer bool) {
	sub.quitOnce.Do(func() {
//调度循环将无法执行取消订阅调用
//如果交货时受阻。关闭Sub.Quit First，因为它
//解除锁定传递。
		close(sub.quit)
		if unsubscribeServer {
			sub.requestUnsubscribe()
		}
		if err != nil {
			if err == ErrClientQuit {
err = nil //遵循订阅语义。
			}
			sub.err <- err
		}
	})
}

func (sub *ClientSubscription) deliver(result json.RawMessage) (ok bool) {
	select {
	case sub.in <- result:
		return true
	case <-sub.quit:
		return false
	}
}

func (sub *ClientSubscription) start() {
	sub.quitWithError(sub.forward())
}

func (sub *ClientSubscription) forward() (err error, unsubscribeServer bool) {
	cases := []reflect.SelectCase{
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(sub.quit)},
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(sub.in)},
		{Dir: reflect.SelectSend, Chan: sub.channel},
	}
	buffer := list.New()
	defer buffer.Init()
	for {
		var chosen int
		var recv reflect.Value
		if buffer.Len() == 0 {
//空闲，省略发送案例。
			chosen, recv, _ = reflect.Select(cases[:2])
		} else {
//非空缓冲区，发送第一个排队的项目。
			cases[2].Send = reflect.ValueOf(buffer.Front().Value)
			chosen, recv, _ = reflect.Select(cases)
		}

		switch chosen {
case 0: //<退出
			return nil, false
case 1: //<-in in
			val, err := sub.unmarshal(recv.Interface().(json.RawMessage))
			if err != nil {
				return err, true
			}
			if buffer.Len() == maxClientSubscriptionBuffer {
				return ErrSubscriptionQueueOverflow, true
			}
			buffer.PushBack(val)
case 2: //子通道<
cases[2].Send = reflect.Value{} //不要抓住价值。
			buffer.Remove(buffer.Front())
		}
	}
}

func (sub *ClientSubscription) unmarshal(result json.RawMessage) (interface{}, error) {
	val := reflect.New(sub.etype)
	err := json.Unmarshal(result, val.Interface())
	return val.Elem().Interface(), err
}

func (sub *ClientSubscription) requestUnsubscribe() error {
	var result interface{}
	return sub.client.Call(&result, sub.namespace+unsubscribeMethodSuffix, sub.subid)
}
