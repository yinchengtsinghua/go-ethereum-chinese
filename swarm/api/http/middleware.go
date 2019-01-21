
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package http

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/swarm/api"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/sctx"
	"github.com/ethereum/go-ethereum/swarm/spancontext"
	"github.com/pborman/uuid"
)

//将链h（主请求处理程序）主处理程序适配到适配器（中间件处理程序）
//请注意，“adapters”的执行顺序是FIFO（适配器[0]将首先执行）
func Adapt(h http.Handler, adapters ...Adapter) http.Handler {
	for i := range adapters {
		adapter := adapters[len(adapters)-1-i]
		h = adapter(h)
	}
	return h
}

type Adapter func(http.Handler) http.Handler

func SetRequestID(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(SetRUID(r.Context(), uuid.New()[:8]))
		metrics.GetOrRegisterCounter(fmt.Sprintf("http.request.%s", r.Method), nil).Inc(1)
		log.Info("created ruid for request", "ruid", GetRUID(r.Context()), "method", r.Method, "url", r.RequestURI)

		h.ServeHTTP(w, r)
	})
}

func SetRequestHost(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(sctx.SetHost(r.Context(), r.Host))
		log.Info("setting request host", "ruid", GetRUID(r.Context()), "host", sctx.GetHost(r.Context()))

		h.ServeHTTP(w, r)
	})
}

func ParseURI(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uri, err := api.Parse(strings.TrimLeft(r.URL.Path, "/"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			respondError(w, r, fmt.Sprintf("invalid URI %q", r.URL.Path), http.StatusBadRequest)
			return
		}
		if uri.Addr != "" && strings.HasPrefix(uri.Addr, "0x") {
			uri.Addr = strings.TrimPrefix(uri.Addr, "0x")

			msg := fmt.Sprintf(`The requested hash seems to be prefixed with '0x'. You will be redirected to the correct URL within 5 seconds.<br/>
			Please click <a href='%[1]s'>here</a> if your browser does not redirect you within 5 seconds.<script>setTimeout("location.href='%[1]s';",5000);</script>`, "/"+uri.String())
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(msg))
			return
		}

		ctx := r.Context()
		r = r.WithContext(SetURI(ctx, uri))
		log.Debug("parsed request path", "ruid", GetRUID(r.Context()), "method", r.Method, "uri.Addr", uri.Addr, "uri.Path", uri.Path, "uri.Scheme", uri.Scheme)

		h.ServeHTTP(w, r)
	})
}

func InitLoggingResponseWriter(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tn := time.Now()

		writer := newLoggingResponseWriter(w)
		h.ServeHTTP(writer, r)

		ts := time.Since(tn)
		log.Info("request served", "ruid", GetRUID(r.Context()), "code", writer.statusCode, "time", ts)
		metrics.GetOrRegisterResettingTimer(fmt.Sprintf("http.request.%s.time", r.Method), nil).Update(ts)
		metrics.GetOrRegisterResettingTimer(fmt.Sprintf("http.request.%s.%d.time", r.Method, writer.statusCode), nil).Update(ts)
	})
}

func InstrumentOpenTracing(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uri := GetURI(r.Context())
		if uri == nil || r.Method == "" || (uri != nil && uri.Scheme == "") {
h.ServeHTTP(w, r) //软故障
			return
		}
		spanName := fmt.Sprintf("http.%s.%s", r.Method, uri.Scheme)
		ctx, sp := spancontext.StartSpan(r.Context(), spanName)

		defer sp.Finish()
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RecoverPanic(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("panic recovery!", "stack trace", string(debug.Stack()), "url", r.URL.String(), "headers", r.Header)
			}
		}()
		h.ServeHTTP(w, r)
	})
}
