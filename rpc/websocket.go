
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
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/net/websocket"
)

//WebSocketJSoncodec是一个自定义的JSON编解码器，具有有效负载大小强制和
//特殊数字分析。
var websocketJSONCodec = websocket.Codec{
//Marshal也是WebSocket库使用的常用JSON Marshaller。
	Marshal: func(v interface{}) ([]byte, byte, error) {
		msg, err := json.Marshal(v)
		return msg, websocket.TextFrame, err
	},
//解组是一种特殊的解组器，用于正确转换数字。
	Unmarshal: func(msg []byte, payloadType byte, v interface{}) error {
		dec := json.NewDecoder(bytes.NewReader(msg))
		dec.UseNumber()

		return dec.Decode(v)
	},
}

//WebSocketHandler返回一个为JSON-RPC到WebSocket连接提供服务的处理程序。
//
//allowedorigins应该是允许的原始URL的逗号分隔列表。
//要允许与任何来源的连接，请通过“*”。
func (srv *Server) WebsocketHandler(allowedOrigins []string) http.Handler {
	return websocket.Server{
		Handshake: wsHandshakeValidator(allowedOrigins),
		Handler: func(conn *websocket.Conn) {
//创建自定义编码/解码对以强制有效负载大小和数字编码
			conn.MaxPayloadBytes = maxRequestContentLength

			encoder := func(v interface{}) error {
				return websocketJSONCodec.Send(conn, v)
			}
			decoder := func(v interface{}) error {
				return websocketJSONCodec.Receive(conn, v)
			}
			srv.ServeCodec(NewCodec(conn, encoder, decoder), OptionMethodInvocation|OptionSubscriptions)
		},
	}
}

//newwsserver围绕API提供程序创建新的WebSocket RPC服务器。
//
//已弃用：使用server.websockethandler
func NewWSServer(allowedOrigins []string, srv *Server) *http.Server {
	return &http.Server{Handler: srv.WebsocketHandler(allowedOrigins)}
}

//wshandshakevalidator返回一个处理程序，该处理程序在
//WebSocket升级过程。当将“*”指定为允许的源时，所有
//接受连接。
func wsHandshakeValidator(allowedOrigins []string) func(*websocket.Config, *http.Request) error {
	origins := mapset.NewSet()
	allowAllOrigins := false

	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAllOrigins = true
		}
		if origin != "" {
			origins.Add(strings.ToLower(origin))
		}
	}

//如果未指定allowedorigins，则允许localhost。
	if len(origins.ToSlice()) == 0 {
origins.Add("http://“本地主机”
		if hostname, err := os.Hostname(); err == nil {
origins.Add("http://“+strings.tolower（主机名））
		}
	}

	log.Debug(fmt.Sprintf("Allowed origin(s) for WS RPC interface %v\n", origins.ToSlice()))

	f := func(cfg *websocket.Config, req *http.Request) error {
		origin := strings.ToLower(req.Header.Get("Origin"))
		if allowAllOrigins || origins.Contains(origin) {
			return nil
		}
		log.Warn(fmt.Sprintf("origin '%s' not allowed on WS-RPC interface\n", origin))
		return fmt.Errorf("origin %s not allowed", origin)
	}

	return f
}

func wsGetConfig(endpoint, origin string) (*websocket.Config, error) {
	if origin == "" {
		var err error
		if origin, err = os.Hostname(); err != nil {
			return nil, err
		}
		if strings.HasPrefix(endpoint, "wss") {
origin = "https://“+strings.tolower（原点）
		} else {
origin = "http://“+strings.tolower（原点）
		}
	}
	config, err := websocket.NewConfig(endpoint, origin)
	if err != nil {
		return nil, err
	}

	if config.Location.User != nil {
		b64auth := base64.StdEncoding.EncodeToString([]byte(config.Location.User.String()))
		config.Header.Add("Authorization", "Basic "+b64auth)
		config.Location.User = nil
	}
	return config, nil
}

//DialWebSocket创建一个新的与JSON-RPC服务器通信的RPC客户端
//正在侦听给定的端点。
//
//上下文用于建立初始连接。它不
//影响与客户的后续交互。
func DialWebsocket(ctx context.Context, endpoint, origin string) (*Client, error) {
	config, err := wsGetConfig(endpoint, origin)
	if err != nil {
		return nil, err
	}

	return newClient(ctx, func(ctx context.Context) (net.Conn, error) {
		return wsDialContext(ctx, config)
	})
}

func wsDialContext(ctx context.Context, config *websocket.Config) (*websocket.Conn, error) {
	var conn net.Conn
	var err error
	switch config.Location.Scheme {
	case "ws":
		conn, err = dialContext(ctx, "tcp", wsDialAddress(config.Location))
	case "wss":
		dialer := contextDialer(ctx)
		conn, err = tls.DialWithDialer(dialer, "tcp", wsDialAddress(config.Location), config.TlsConfig)
	default:
		err = websocket.ErrBadScheme
	}
	if err != nil {
		return nil, err
	}
	ws, err := websocket.NewClient(config, conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return ws, err
}

var wsPortMap = map[string]string{"ws": "80", "wss": "443"}

func wsDialAddress(location *url.URL) string {
	if _, ok := wsPortMap[location.Scheme]; ok {
		if _, _, err := net.SplitHostPort(location.Host); err != nil {
			return net.JoinHostPort(location.Host, wsPortMap[location.Scheme])
		}
	}
	return location.Host
}

func dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	d := &net.Dialer{KeepAlive: tcpKeepAliveInterval}
	return d.DialContext(ctx, network, addr)
}

func contextDialer(ctx context.Context) *net.Dialer {
	dialer := &net.Dialer{Cancel: ctx.Done(), KeepAlive: tcpKeepAliveInterval}
	if deadline, ok := ctx.Deadline(); ok {
		dialer.Deadline = deadline
	} else {
		dialer.Deadline = time.Now().Add(defaultDialTimeout)
	}
	return dialer
}
