
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestError(t *testing.T) {
	srv := NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	var resp *http.Response
	var respbody []byte

	url := srv.URL + "/this_should_fail_as_no_bzz_protocol_present"
	resp, err := http.Get(url)

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	respbody, err = ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 404 && !strings.Contains(string(respbody), "Invalid URI &#34;/this_should_fail_as_no_bzz_protocol_present&#34;: unknown scheme") {
		t.Fatalf("Response body does not match, expected: %v, to contain: %v; received code %d, expected code: %d", string(respbody), "Invalid bzz URI: unknown scheme", 400, resp.StatusCode)
	}

	_, err = html.Parse(strings.NewReader(string(respbody)))
	if err != nil {
		t.Fatalf("HTML validation failed for error page returned!")
	}
}

func Test404Page(t *testing.T) {
	srv := NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	var resp *http.Response
	var respbody []byte

	url := srv.URL + "/bzz:/1234567890123456789012345678901234567890123456789012345678901234"
	resp, err := http.Get(url)

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	respbody, err = ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 404 || !strings.Contains(string(respbody), "404") {
		t.Fatalf("Invalid Status Code received, expected 404, got %d", resp.StatusCode)
	}

	_, err = html.Parse(strings.NewReader(string(respbody)))
	if err != nil {
		t.Fatalf("HTML validation failed for error page returned!")
	}
}

func Test500Page(t *testing.T) {
	srv := NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	var resp *http.Response
	var respbody []byte

	url := srv.URL + "/bzz:/thisShouldFailWith500Code"
	resp, err := http.Get(url)

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	respbody, err = ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 404 {
		t.Fatalf("Invalid Status Code received, expected 404, got %d", resp.StatusCode)
	}

	_, err = html.Parse(strings.NewReader(string(respbody)))
	if err != nil {
		t.Fatalf("HTML validation failed for error page returned!")
	}
}
func Test500PageWith0xHashPrefix(t *testing.T) {
	srv := NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	var resp *http.Response
	var respbody []byte

	url := srv.URL + "/bzz:/0xthisShouldFailWith500CodeAndAHelpfulMessage"
	resp, err := http.Get(url)

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	respbody, err = ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 404 {
		t.Fatalf("Invalid Status Code received, expected 404, got %d", resp.StatusCode)
	}

	if !strings.Contains(string(respbody), "The requested hash seems to be prefixed with") {
		t.Fatalf("Did not receive the expected error message")
	}

	_, err = html.Parse(strings.NewReader(string(respbody)))
	if err != nil {
		t.Fatalf("HTML validation failed for error page returned!")
	}
}

func TestJsonResponse(t *testing.T) {
	srv := NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	var resp *http.Response
	var respbody []byte

	url := srv.URL + "/bzz:/thisShouldFailWith500Code/"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	defer resp.Body.Close()
	respbody, err = ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 404 {
		t.Fatalf("Invalid Status Code received, expected 404, got %d", resp.StatusCode)
	}

	if !isJSON(string(respbody)) {
		t.Fatalf("Expected response to be JSON, received invalid JSON: %s", string(respbody))
	}

}

func isJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}
