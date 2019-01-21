
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/rs/cors"
)

const (
	maxRequestContentLength = 1024 * 512
)

var (
//https://www.json rpc.org/historical/json-rpc over http.html id13
	acceptedContentTypes = []string{"application/json", "application/json-rpc", "application/jsonrequest"}
	contentType          = acceptedContentTypes[0]
	nullAddr, _          = net.ResolveTCPAddr("tcp", "127.0.0.1:0")
)

type httpConn struct {
	client    *http.Client
	req       *http.Request
	closeOnce sync.Once
	closed    chan struct{}
}

//HTTPconn由客户特别处理。
func (hc *httpConn) LocalAddr() net.Addr              { return nullAddr }
func (hc *httpConn) RemoteAddr() net.Addr             { return nullAddr }
func (hc *httpConn) SetReadDeadline(time.Time) error  { return nil }
func (hc *httpConn) SetWriteDeadline(time.Time) error { return nil }
func (hc *httpConn) SetDeadline(time.Time) error      { return nil }
func (hc *httpConn) Write([]byte) (int, error)        { panic("Write called") }

func (hc *httpConn) Read(b []byte) (int, error) {
	<-hc.closed
	return 0, io.EOF
}

func (hc *httpConn) Close() error {
	hc.closeOnce.Do(func() { close(hc.closed) })
	return nil
}

//httpTimeouts表示HTTP RPC服务器的配置参数。
type HTTPTimeouts struct {
//readTimeout是读取整个
//请求，包括正文。
//
//因为readTimeout不允许处理程序按请求执行
//对每个请求主体可接受的截止日期或
//上传率，大多数用户更喜欢使用
//读取headerTimeout。两者都用是有效的。
	ReadTimeout time.Duration

//WriteTimeout是超时前的最长持续时间
//写入响应。每当有新的
//请求的头被读取。和readTimeout一样，它没有
//让处理程序基于每个请求做出决策。
	WriteTimeout time.Duration

//IdleTimeout是等待
//启用keep alives时的下一个请求。如果IdleTimeout
//为零，使用readTimeout的值。如果两者都是
//零，使用readHeaderTimeout。
	IdleTimeout time.Duration
}

//DefaultHTTPTimeouts表示进一步使用的默认超时值
//未提供配置。
var DefaultHTTPTimeouts = HTTPTimeouts{
	ReadTimeout:  30 * time.Second,
	WriteTimeout: 30 * time.Second,
	IdleTimeout:  120 * time.Second,
}

//dialhttpwithclient创建通过HTTP连接到RPC服务器的新RPC客户端
//使用提供的HTTP客户端。
func DialHTTPWithClient(endpoint string, client *http.Client) (*Client, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", contentType)

	initctx := context.Background()
	return newClient(initctx, func(context.Context) (net.Conn, error) {
		return &httpConn{client: client, req: req, closed: make(chan struct{})}, nil
	})
}

//DialHTTP创建一个新的RPC客户端，通过HTTP连接到一个RPC服务器。
func DialHTTP(endpoint string) (*Client, error) {
	return DialHTTPWithClient(endpoint, new(http.Client))
}

func (c *Client) sendHTTP(ctx context.Context, op *requestOp, msg interface{}) error {
	hc := c.writeConn.(*httpConn)
	respBody, err := hc.doRequest(ctx, msg)
	if respBody != nil {
		defer respBody.Close()
	}

	if err != nil {
		if respBody != nil {
			buf := new(bytes.Buffer)
			if _, err2 := buf.ReadFrom(respBody); err2 == nil {
				return fmt.Errorf("%v %v", err, buf.String())
			}
		}
		return err
	}
	var respmsg jsonrpcMessage
	if err := json.NewDecoder(respBody).Decode(&respmsg); err != nil {
		return err
	}
	op.resp <- &respmsg
	return nil
}

func (c *Client) sendBatchHTTP(ctx context.Context, op *requestOp, msgs []*jsonrpcMessage) error {
	hc := c.writeConn.(*httpConn)
	respBody, err := hc.doRequest(ctx, msgs)
	if err != nil {
		return err
	}
	defer respBody.Close()
	var respmsgs []jsonrpcMessage
	if err := json.NewDecoder(respBody).Decode(&respmsgs); err != nil {
		return err
	}
	for i := 0; i < len(respmsgs); i++ {
		op.resp <- &respmsgs[i]
	}
	return nil
}

func (hc *httpConn) doRequest(ctx context.Context, msg interface{}) (io.ReadCloser, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	req := hc.req.WithContext(ctx)
	req.Body = ioutil.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))

	resp, err := hc.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.Body, errors.New(resp.Status)
	}
	return resp.Body, nil
}

//httpreadwritenocloser使用nop close方法包装IO.reader和IO.writer。
type httpReadWriteNopCloser struct {
	io.Reader
	io.Writer
}

//CLOSE什么也不做，返回的总是零
func (t *httpReadWriteNopCloser) Close() error {
	return nil
}

//new http server围绕API提供程序创建一个新的HTTP RPC服务器。
//
//已弃用：服务器实现http.handler
func NewHTTPServer(cors []string, vhosts []string, timeouts HTTPTimeouts, srv *Server) *http.Server {
//在主机处理程序中包装CORS处理程序
	handler := newCorsHandler(srv, cors)
	handler = newVHostHandler(vhosts, handler)

//确保超时值有意义
	if timeouts.ReadTimeout < time.Second {
		log.Warn("Sanitizing invalid HTTP read timeout", "provided", timeouts.ReadTimeout, "updated", DefaultHTTPTimeouts.ReadTimeout)
		timeouts.ReadTimeout = DefaultHTTPTimeouts.ReadTimeout
	}
	if timeouts.WriteTimeout < time.Second {
		log.Warn("Sanitizing invalid HTTP write timeout", "provided", timeouts.WriteTimeout, "updated", DefaultHTTPTimeouts.WriteTimeout)
		timeouts.WriteTimeout = DefaultHTTPTimeouts.WriteTimeout
	}
	if timeouts.IdleTimeout < time.Second {
		log.Warn("Sanitizing invalid HTTP idle timeout", "provided", timeouts.IdleTimeout, "updated", DefaultHTTPTimeouts.IdleTimeout)
		timeouts.IdleTimeout = DefaultHTTPTimeouts.IdleTimeout
	}
//捆绑并启动HTTP服务器
	return &http.Server{
		Handler:      handler,
		ReadTimeout:  timeouts.ReadTimeout,
		WriteTimeout: timeouts.WriteTimeout,
		IdleTimeout:  timeouts.IdleTimeout,
	}
}

//servehtp通过HTTP服务JSON-RPC请求。
func (srv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//允许远程健康检查（AWS）的哑空请求
	if r.Method == http.MethodGet && r.ContentLength == 0 && r.URL.RawQuery == "" {
		return
	}
	if code, err := validateRequest(r); err != nil {
		http.Error(w, err.Error(), code)
		return
	}
//通过所有检查，创建一个直接从请求主体读取的编解码器
//直到并将响应写入w，然后命令服务器处理
//单一请求。
	ctx := r.Context()
	ctx = context.WithValue(ctx, "remote", r.RemoteAddr)
	ctx = context.WithValue(ctx, "scheme", r.Proto)
	ctx = context.WithValue(ctx, "local", r.Host)
	if ua := r.Header.Get("User-Agent"); ua != "" {
		ctx = context.WithValue(ctx, "User-Agent", ua)
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		ctx = context.WithValue(ctx, "Origin", origin)
	}

	body := io.LimitReader(r.Body, maxRequestContentLength)
	codec := NewJSONCodec(&httpReadWriteNopCloser{body, w})
	defer codec.Close()

	w.Header().Set("content-type", contentType)
	srv.ServeSingleRequest(ctx, codec, OptionMethodInvocation)
}

//validateRequest返回非零响应代码和错误消息，如果
//请求无效。
func validateRequest(r *http.Request) (int, error) {
	if r.Method == http.MethodPut || r.Method == http.MethodDelete {
		return http.StatusMethodNotAllowed, errors.New("method not allowed")
	}
	if r.ContentLength > maxRequestContentLength {
		err := fmt.Errorf("content length too large (%d>%d)", r.ContentLength, maxRequestContentLength)
		return http.StatusRequestEntityTooLarge, err
	}
//允许选项（不考虑内容类型）
	if r.Method == http.MethodOptions {
		return 0, nil
	}
//检查内容类型
	if mt, _, err := mime.ParseMediaType(r.Header.Get("content-type")); err == nil {
		for _, accepted := range acceptedContentTypes {
			if accepted == mt {
				return 0, nil
			}
		}
	}
//内容类型无效
	err := fmt.Errorf("invalid content type, only %s is supported", contentType)
	return http.StatusUnsupportedMediaType, err
}

func newCorsHandler(srv *Server, allowedOrigins []string) http.Handler {
//如果用户未指定自定义CORS配置，则禁用CORS支持
	if len(allowedOrigins) == 0 {
		return srv
	}
	c := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{http.MethodPost, http.MethodGet},
		MaxAge:         600,
		AllowedHeaders: []string{"*"},
	})
	return c.Handler(srv)
}

//virtualHostHandler是验证传入请求的主机头的处理程序。
//virtualHostHandler可以防止不使用CORS头的DNS重新绑定攻击，
//因为它们是针对RPC API的域请求。相反，我们可以在主机头上看到
//使用了哪个域，并根据白名单验证这一点。
type virtualHostHandler struct {
	vhosts map[string]struct{}
	next   http.Handler
}

//servehtp通过http服务json-rpc请求，实现http.handler
func (h *virtualHostHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//如果没有设置r.host，我们可以继续服务，因为浏览器会设置主机头
	if r.Host == "" {
		h.next.ServeHTTP(w, r)
		return
	}
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
//无效（冒号太多）或未指定端口
		host = r.Host
	}
	if ipAddr := net.ParseIP(host); ipAddr != nil {
//这是一个IP地址，我们可以提供
		h.next.ServeHTTP(w, r)
		return

	}
//不是IP地址，而是主机名。需要验证
	if _, exist := h.vhosts["*"]; exist {
		h.next.ServeHTTP(w, r)
		return
	}
	if _, exist := h.vhosts[host]; exist {
		h.next.ServeHTTP(w, r)
		return
	}
	http.Error(w, "invalid host specified", http.StatusForbidden)
}

func newVHostHandler(vhosts []string, next http.Handler) http.Handler {
	vhostMap := make(map[string]struct{})
	for _, allowedHost := range vhosts {
		vhostMap[strings.ToLower(allowedHost)] = struct{}{}
	}
	return &virtualHostHandler{vhostMap, next}
}
