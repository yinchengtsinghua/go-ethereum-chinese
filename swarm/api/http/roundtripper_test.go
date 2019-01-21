
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

package http

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRoundTripper(t *testing.T) {
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "text/plain")
			http.ServeContent(w, r, "", time.Unix(0, 0), strings.NewReader(r.RequestURI))
		} else {
			http.Error(w, "Method "+r.Method+" is not supported.", http.StatusMethodNotAllowed)
		}
	})

	srv := httptest.NewServer(serveMux)
	defer srv.Close()

	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	rt := &RoundTripper{Host: host, Port: port}
	trans := &http.Transport{}
	trans.RegisterProtocol("bzz", rt)
	client := &http.Client{Transport: trans}
resp, err := client.Get("bzz://COM/PATH“测试”
	if err != nil {
		t.Errorf("expected no error, got %v", err)
		return
	}

	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
		return
	}
	if string(content) != "/HTTP/1.1:/test.com/path" {
		t.Errorf("incorrect response from http server: expected '%v', got '%v'", "/HTTP/1.1:/test.com/path", string(content))
	}

}
