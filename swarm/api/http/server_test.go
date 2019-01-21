
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
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/swarm/storage/feed/lookup"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/swarm/api"
	swarm "github.com/ethereum/go-ethereum/swarm/api/client"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/feed"
	"github.com/ethereum/go-ethereum/swarm/testutil"
)

func init() {
	loglevel := flag.Int("loglevel", 2, "loglevel")
	flag.Parse()
	log.Root().SetHandler(log.CallerFileHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(os.Stderr, log.TerminalFormat(true)))))
}

func serverFunc(api *api.API) TestServer {
	return NewServer(api, "")
}

func newTestSigner() (*feed.GenericSigner, error) {
	privKey, err := crypto.HexToECDSA("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err != nil {
		return nil, err
	}
	return feed.NewGenericSigner(privKey), nil
}

//使用bzz://scheme测试源更新的透明解析
//
//首先将数据上传到bzz:，并将swarm散列存储到feed更新中的结果清单中。
//这有效地使用提要来存储指向内容的指针，而不是内容本身。
//使用swarm散列检索更新应返回直接指向数据的清单。
//对散列的原始检索应该返回数据
func TestBzzWithFeed(t *testing.T) {

	signer, _ := newTestSigner()

//初始化Swarm测试服务器
	srv := NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

//为我们的测试收集一些数据：
	dataBytes := []byte(`
//
//创建一些我们的清单将指向的数据。数据可能非常大，不适合feed更新。
//因此，我们要做的是将其上传到swarm bzz/，并获取指向它的**清单哈希**：
//
//清单哈希-->数据
//
//然后，我们将该**清单哈希**存储到一个swarm feed更新中。一旦我们这样做了，
//我们可以使用bzz://feed manifest散列中的**feed manifest散列，方法是：bzz://feed manifest散列。
//
//源清单哈希——>清单哈希——>数据
//
//假设我们可以在任何时候用一个新的**清单哈希**更新提要，但是**提要清单哈希**。
//保持不变，我们有效地创建了一个固定地址来更改内容。（掌声）
//
//源清单哈希（相同）->清单哈希（2）->数据（2）…
//
	`)

//将数据发布到BZZ并返回指向它的内容地址**清单哈希**。
	resp, err := http.Post(fmt.Sprintf("%s/bzz:/", srv.URL), "text/plain", bytes.NewReader([]byte(dataBytes)))
	if err != nil {
		t.Fatal(err)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	manifestAddressHex, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		t.Fatal(err)
	}

	manifestAddress := common.FromHex(string(manifestAddressHex))

	log.Info("added data", "manifest", string(manifestAddressHex))

//此时，我们已经上传了数据，并有一个清单指向它。
//现在将该清单地址存储在源更新中。
//我们还需要一个提要清单，因此我们可以使用它来引用提要。

//首先，为我们的提要创建一个主题：
	topic, _ := feed.NewTopic("interesting topic indeed", nil)

//创建订阅源更新请求：
	updateRequest := feed.NewFirstRequest(topic)

//将**清单地址**作为数据存储到feed更新中。
	updateRequest.SetData(manifestAddress)

//签署更新
	if err := updateRequest.Sign(signer); err != nil {
		t.Fatal(err)
	}
	log.Info("added data", "data", common.ToHex(manifestAddress))

//生成源更新HTTP请求：
	feedUpdateURL, err := url.Parse(fmt.Sprintf("%s/bzz-feed:/", srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	query := feedUpdateURL.Query()
body := updateRequest.AppendValues(query) //这将添加所有查询参数并返回要发布的数据
query.Set("manifest", "1")                //指示我们要返回反馈清单
	feedUpdateURL.RawQuery = query.Encode()

//向Swarm提交feed更新请求
	resp, err = http.Post(feedUpdateURL.String(), "application/octet-stream", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}

	feedManifestAddressHex, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	feedManifestAddress := &storage.Address{}
	err = json.Unmarshal(feedManifestAddressHex, feedManifestAddress)
	if err != nil {
		t.Fatalf("data %s could not be unmarshaled: %v", feedManifestAddressHex, err)
	}

	correctManifestAddrHex := "747c402e5b9dc715a25a4393147512167bab018a007fad7cdcd9adc7fce1ced2"
	if feedManifestAddress.Hex() != correctManifestAddrHex {
		t.Fatalf("Response feed manifest address mismatch, expected '%s', got '%s'", correctManifestAddrHex, feedManifestAddress.Hex())
	}

//获取BZZ清单透明源更新解析
	getBzzURL := fmt.Sprintf("%s/bzz:/%s", srv.URL, feedManifestAddress)
	resp, err = http.Get(getBzzURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	retrievedData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(retrievedData, []byte(dataBytes)) {
		t.Fatalf("retrieved data mismatch, expected %x, got %x", dataBytes, retrievedData)
	}
}

//使用原始更新方法测试群饲料
func TestBzzFeed(t *testing.T) {
	srv := NewTestSwarmServer(t, serverFunc, nil)
	signer, _ := newTestSigner()

	defer srv.Close()

//更新1的数据
	update1Data := testutil.RandomBytes(1, 666)
	update1Timestamp := srv.CurrentTime
//更新2的数据
	update2Data := []byte("foo")

	topic, _ := feed.NewTopic("foo.eth", nil)
	updateRequest := feed.NewFirstRequest(topic)
	updateRequest.SetData(update1Data)

	if err := updateRequest.Sign(signer); err != nil {
		t.Fatal(err)
	}

//创建源并设置更新1
	testUrl, err := url.Parse(fmt.Sprintf("%s/bzz-feed:/", srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	urlQuery := testUrl.Query()
body := updateRequest.AppendValues(urlQuery) //这将添加所有查询参数
urlQuery.Set("manifest", "1")                //表明我们想要一份清单
	testUrl.RawQuery = urlQuery.Encode()

	resp, err := http.Post(testUrl.String(), "application/octet-stream", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	rsrcResp := &storage.Address{}
	err = json.Unmarshal(b, rsrcResp)
	if err != nil {
		t.Fatalf("data %s could not be unmarshaled: %v", b, err)
	}

	correctManifestAddrHex := "bb056a5264c295c2b0f613c8409b9c87ce9d71576ace02458160df4cc894210b"
	if rsrcResp.Hex() != correctManifestAddrHex {
		t.Fatalf("Response feed manifest mismatch, expected '%s', got '%s'", correctManifestAddrHex, rsrcResp.Hex())
	}

//获取清单
	testRawUrl := fmt.Sprintf("%s/bzz-raw:/%s", srv.URL, rsrcResp)
	resp, err = http.Get(testRawUrl)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	manifest := &api.Manifest{}
	err = json.Unmarshal(b, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entries) != 1 {
		t.Fatalf("Manifest has %d entries", len(manifest.Entries))
	}
	correctFeedHex := "0x666f6f2e65746800000000000000000000000000000000000000000000000000c96aaa54e2d44c299564da76e1cd3184a2386b8d"
	if manifest.Entries[0].Feed.Hex() != correctFeedHex {
		t.Fatalf("Expected manifest Feed '%s', got '%s'", correctFeedHex, manifest.Entries[0].Feed.Hex())
	}

//抓住机会让bzz:crash解决不包含的源更新
//群散列：
	testBzzUrl := fmt.Sprintf("%s/bzz:/%s", srv.URL, rsrcResp)
	resp, err = http.Get(testBzzUrl)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatal("Expected error status since feed update does not contain a Swarm hash. Received 200 OK")
	}
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

//获取不存在的名称，应失败
	testBzzResUrl := fmt.Sprintf("%s/bzz-feed:/bar", srv.URL)
	resp, err = http.Get(testBzzResUrl)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected get non-existent feed manifest to fail with StatusNotFound (404), got %d", resp.StatusCode)
	}

	resp.Body.Close()

//直接通过bzz feed获取最新更新
	log.Info("get update latest = 1.1", "addr", correctManifestAddrHex)
	testBzzResUrl = fmt.Sprintf("%s/bzz-feed:/%s", srv.URL, correctManifestAddrHex)
	resp, err = http.Get(testBzzResUrl)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(update1Data, b) {
		t.Fatalf("Expected body '%x', got '%x'", update1Data, b)
	}

//更新2
//向前拨钟1秒
	srv.CurrentTime++
	log.Info("update 2")

//1.-获取有关此源的元数据
	testBzzResUrl = fmt.Sprintf("%s/bzz-feed:/%s/", srv.URL, correctManifestAddrHex)
	resp, err = http.Get(testBzzResUrl + "?meta=1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Get feed metadata returned %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	updateRequest = &feed.Request{}
	if err = updateRequest.UnmarshalJSON(b); err != nil {
		t.Fatalf("Error decoding feed metadata: %s", err)
	}
	updateRequest.SetData(update2Data)
	if err = updateRequest.Sign(signer); err != nil {
		t.Fatal(err)
	}
	testUrl, err = url.Parse(fmt.Sprintf("%s/bzz-feed:/", srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	urlQuery = testUrl.Query()
body = updateRequest.AppendValues(urlQuery) //这将添加所有查询参数
goodQueryParameters := urlQuery.Encode()    //保存查询参数以便再次尝试

//创建缺少签名的错误查询参数
	urlQuery.Del("signature")
	testUrl.RawQuery = urlQuery.Encode()

//第一次尝试使用错误的查询参数，其中缺少签名
	resp, err = http.Post(testUrl.String(), "application/octet-stream", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	expectedCode := http.StatusBadRequest
	if resp.StatusCode != expectedCode {
		t.Fatalf("Update returned %s. Expected %d", resp.Status, expectedCode)
	}

//第二次尝试使用错误的查询参数，其中签名的长度不正确
urlQuery.Set("signature", "0xabcd") //应该是130个十六进制字符
	resp, err = http.Post(testUrl.String(), "application/octet-stream", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	expectedCode = http.StatusBadRequest
	if resp.StatusCode != expectedCode {
		t.Fatalf("Update returned %s. Expected %d", resp.Status, expectedCode)
	}

//第三次尝试，具有良好的查询参数：
	testUrl.RawQuery = goodQueryParameters
	resp, err = http.Post(testUrl.String(), "application/octet-stream", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	expectedCode = http.StatusOK
	if resp.StatusCode != expectedCode {
		t.Fatalf("Update returned %s. Expected %d", resp.Status, expectedCode)
	}

//直接通过bzz feed获取最新更新
	log.Info("get update 1.2")
	testBzzResUrl = fmt.Sprintf("%s/bzz-feed:/%s", srv.URL, correctManifestAddrHex)
	resp, err = http.Get(testBzzResUrl)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(update2Data, b) {
		t.Fatalf("Expected body '%x', got '%x'", update2Data, b)
	}

//测试清单较少的查询
	log.Info("get first update in update1Timestamp via direct query")
	query := feed.NewQuery(&updateRequest.Feed, update1Timestamp, lookup.NoClue)

	urlq, err := url.Parse(fmt.Sprintf("%s/bzz-feed:/", srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	values := urlq.Query()
query.AppendValues(values) //这将添加源查询参数
	urlq.RawQuery = values.Encode()
	resp, err = http.Get(urlq.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(update1Data, b) {
		t.Fatalf("Expected body '%x', got '%x'", update1Data, b)
	}

}

func TestBzzGetPath(t *testing.T) {
	testBzzGetPath(false, t)
	testBzzGetPath(true, t)
}

func testBzzGetPath(encrypted bool, t *testing.T) {
	var err error

	testmanifest := []string{
		`{"entries":[{"path":"b","hash":"011b4d03dd8c01f1049143cf9c4c817e4b167f1d1b83e5c6f0f10d89ba1e7bce","contentType":"","status":0},{"path":"c","hash":"011b4d03dd8c01f1049143cf9c4c817e4b167f1d1b83e5c6f0f10d89ba1e7bce","contentType":"","status":0}]}`,
		`{"entries":[{"path":"a","hash":"011b4d03dd8c01f1049143cf9c4c817e4b167f1d1b83e5c6f0f10d89ba1e7bce","contentType":"","status":0},{"path":"b/","hash":"<key0>","contentType":"application/bzz-manifest+json","status":0}]}`,
		`{"entries":[{"path":"a/","hash":"<key1>","contentType":"application/bzz-manifest+json","status":0}]}`,
	}

	testrequests := make(map[string]int)
	testrequests["/"] = 2
	testrequests["/a/"] = 1
	testrequests["/a/b/"] = 0
	testrequests["/x"] = 0
	testrequests[""] = 0

	expectedfailrequests := []string{"", "/x"}

	reader := [3]*bytes.Reader{}

	addr := [3]storage.Address{}

	srv := NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	for i, mf := range testmanifest {
		reader[i] = bytes.NewReader([]byte(mf))
		var wait func(context.Context) error
		ctx := context.TODO()
		addr[i], wait, err = srv.FileStore.Store(ctx, reader[i], int64(len(mf)), encrypted)
		if err != nil {
			t.Fatal(err)
		}
		for j := i + 1; j < len(testmanifest); j++ {
			testmanifest[j] = strings.Replace(testmanifest[j], fmt.Sprintf("<key%v>", i), addr[i].Hex(), -1)
		}
		err = wait(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	rootRef := addr[2].Hex()

	_, err = http.Get(srv.URL + "/bzz-raw:/" + rootRef + "/a")
	if err != nil {
		t.Fatalf("Failed to connect to proxy: %v", err)
	}

	for k, v := range testrequests {
		var resp *http.Response
		var respbody []byte

		url := srv.URL + "/bzz-raw:/"
		if k != "" {
			url += rootRef + "/" + k[1:] + "?content_type=text/plain"
		}
		resp, err = http.Get(url)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()
		respbody, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Error while reading response body: %v", err)
		}

		if string(respbody) != testmanifest[v] {
			isexpectedfailrequest := false

			for _, r := range expectedfailrequests {
				if k == r {
					isexpectedfailrequest = true
				}
			}
			if !isexpectedfailrequest {
				t.Fatalf("Response body does not match, expected: %v, got %v", testmanifest[v], string(respbody))
			}
		}
	}

	for k, v := range testrequests {
		var resp *http.Response
		var respbody []byte

		url := srv.URL + "/bzz-hash:/"
		if k != "" {
			url += rootRef + "/" + k[1:]
		}
		resp, err = http.Get(url)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()
		respbody, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Read request body: %v", err)
		}

		if string(respbody) != addr[v].Hex() {
			isexpectedfailrequest := false

			for _, r := range expectedfailrequests {
				if k == r {
					isexpectedfailrequest = true
				}
			}
			if !isexpectedfailrequest {
				t.Fatalf("Response body does not match, expected: %v, got %v", addr[v], string(respbody))
			}
		}
	}

	ref := addr[2].Hex()

	for _, c := range []struct {
		path          string
		json          string
		pageFragments []string
	}{
		{
			path: "/",
			json: `{"common_prefixes":["a/"]}`,
			pageFragments: []string{
				fmt.Sprintf("Swarm index of bzz:/%s/", ref),
				`<a class="normal-link" href="a/">a/</a>`,
			},
		},
		{
			path: "/a/",
			json: `{"common_prefixes":["a/b/"],"entries":[{"hash":"011b4d03dd8c01f1049143cf9c4c817e4b167f1d1b83e5c6f0f10d89ba1e7bce","path":"a/a","mod_time":"0001-01-01T00:00:00Z"}]}`,
			pageFragments: []string{
				fmt.Sprintf("Swarm index of bzz:/%s/a/", ref),
				`<a class="normal-link" href="b/">b/</a>`,
				fmt.Sprintf(`<a class="normal-link" href="/bzz:/%s/a/a">a</a>`, ref),
			},
		},
		{
			path: "/a/b/",
			json: `{"entries":[{"hash":"011b4d03dd8c01f1049143cf9c4c817e4b167f1d1b83e5c6f0f10d89ba1e7bce","path":"a/b/b","mod_time":"0001-01-01T00:00:00Z"},{"hash":"011b4d03dd8c01f1049143cf9c4c817e4b167f1d1b83e5c6f0f10d89ba1e7bce","path":"a/b/c","mod_time":"0001-01-01T00:00:00Z"}]}`,
			pageFragments: []string{
				fmt.Sprintf("Swarm index of bzz:/%s/a/b/", ref),
				fmt.Sprintf(`<a class="normal-link" href="/bzz:/%s/a/b/b">b</a>`, ref),
				fmt.Sprintf(`<a class="normal-link" href="/bzz:/%s/a/b/c">c</a>`, ref),
			},
		},
		{
			path: "/x",
		},
		{
			path: "",
		},
	} {
		k := c.path
		url := srv.URL + "/bzz-list:/"
		if k != "" {
			url += rootRef + "/" + k[1:]
		}
		t.Run("json list "+c.path, func(t *testing.T) {
			resp, err := http.Get(url)
			if err != nil {
				t.Fatalf("HTTP request: %v", err)
			}
			defer resp.Body.Close()
			respbody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Read response body: %v", err)
			}

			body := strings.TrimSpace(string(respbody))
			if body != c.json {
				isexpectedfailrequest := false

				for _, r := range expectedfailrequests {
					if k == r {
						isexpectedfailrequest = true
					}
				}
				if !isexpectedfailrequest {
					t.Errorf("Response list body %q does not match, expected: %v, got %v", k, c.json, body)
				}
			}
		})
		t.Run("html list "+c.path, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, url, nil)
			if err != nil {
				t.Fatalf("New request: %v", err)
			}
			req.Header.Set("Accept", "text/html")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("HTTP request: %v", err)
			}
			defer resp.Body.Close()
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Read response body: %v", err)
			}

			body := string(b)

			for _, f := range c.pageFragments {
				if !strings.Contains(body, f) {
					isexpectedfailrequest := false

					for _, r := range expectedfailrequests {
						if k == r {
							isexpectedfailrequest = true
						}
					}
					if !isexpectedfailrequest {
						t.Errorf("Response list body %q does not contain %q: body %q", k, f, body)
					}
				}
			}
		})
	}

	nonhashtests := []string{
		srv.URL + "/bzz:/name",
		srv.URL + "/bzz-immutable:/nonhash",
		srv.URL + "/bzz-raw:/nonhash",
		srv.URL + "/bzz-list:/nonhash",
		srv.URL + "/bzz-hash:/nonhash",
	}

	nonhashresponses := []string{
		`cannot resolve name: no DNS to resolve name: "name"`,
		`cannot resolve nonhash: no DNS to resolve name: "nonhash"`,
		`cannot resolve nonhash: no DNS to resolve name: "nonhash"`,
		`cannot resolve nonhash: no DNS to resolve name: "nonhash"`,
		`cannot resolve nonhash: no DNS to resolve name: "nonhash"`,
	}

	for i, url := range nonhashtests {
		var resp *http.Response
		var respbody []byte

		resp, err = http.Get(url)

		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()
		respbody, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}
		if !strings.Contains(string(respbody), nonhashresponses[i]) {
			t.Fatalf("Non-Hash response body does not match, expected: %v, got: %v", nonhashresponses[i], string(respbody))
		}
	}
}

func TestBzzTar(t *testing.T) {
	testBzzTar(false, t)
	testBzzTar(true, t)
}

func testBzzTar(encrypted bool, t *testing.T) {
	srv := NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()
	fileNames := []string{"tmp1.txt", "tmp2.lock", "tmp3.rtf"}
	fileContents := []string{"tmp1textfilevalue", "tmp2lockfilelocked", "tmp3isjustaplaintextfile"}

	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)
	defer tw.Close()

	for i, v := range fileNames {
		size := int64(len(fileContents[i]))
		hdr := &tar.Header{
			Name:    v,
			Mode:    0644,
			Size:    size,
			ModTime: time.Now(),
			Xattrs: map[string]string{
				"user.swarm.content-type": "text/plain",
			},
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}

//将文件复制到tar流中
		n, err := io.Copy(tw, bytes.NewBufferString(fileContents[i]))
		if err != nil {
			t.Fatal(err)
		} else if n != size {
			t.Fatal("size mismatch")
		}
	}

//后焦油流
	url := srv.URL + "/bzz:/"
	if encrypted {
		url = url + "encrypt"
	}
	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-tar")
	client := &http.Client{}
	resp2, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp2.Status)
	}
	swarmHash, err := ioutil.ReadAll(resp2.Body)
	resp2.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

//现在去拿一个柏油球回来
	req, err = http.NewRequest("GET", fmt.Sprintf(srv.URL+"/bzz:/%s", string(swarmHash)), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Accept", "application/x-tar")
	resp2, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	if h := resp2.Header.Get("Content-Type"); h != "application/x-tar" {
		t.Fatalf("Content-Type header expected: application/x-tar, got: %s", h)
	}

	expectedFileName := string(swarmHash) + ".tar"
	expectedContentDisposition := fmt.Sprintf("inline; filename=\"%s\"", expectedFileName)
	if h := resp2.Header.Get("Content-Disposition"); h != expectedContentDisposition {
		t.Fatalf("Content-Disposition header expected: %s, got: %s", expectedContentDisposition, h)
	}

	file, err := ioutil.TempFile("", "swarm-downloaded-tarball")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())
	_, err = io.Copy(file, resp2.Body)
	if err != nil {
		t.Fatalf("error getting tarball: %v", err)
	}
	file.Sync()
	file.Close()

	tarFileHandle, err := os.Open(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(tarFileHandle)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("error reading tar stream: %s", err)
		}
		bb := make([]byte, hdr.Size)
		_, err = tr.Read(bb)
		if err != nil && err != io.EOF {
			t.Fatal(err)
		}
		passed := false
		for i, v := range fileNames {
			if v == hdr.Name {
				if string(bb) == fileContents[i] {
					passed = true
					break
				}
			}
		}
		if !passed {
			t.Fatalf("file %s did not pass content assertion", hdr.Name)
		}
	}
}

//testbzzrootredirect测试获取清单的根路径时
//尾随斜杠被重定向为包含尾随斜杠，以便
//相对URL按预期工作。
func TestBzzRootRedirect(t *testing.T) {
	testBzzRootRedirect(false, t)
}
func TestBzzRootRedirectEncrypted(t *testing.T) {
	testBzzRootRedirect(true, t)
}

func testBzzRootRedirect(toEncrypt bool, t *testing.T) {
	srv := NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

//在根路径上创建包含一些数据的清单
	client := swarm.NewClient(srv.URL)
	data := []byte("data")
	file := &swarm.File{
		ReadCloser: ioutil.NopCloser(bytes.NewReader(data)),
		ManifestEntry: api.ManifestEntry{
			Path:        "",
			ContentType: "text/plain",
			Size:        int64(len(data)),
		},
	}
	hash, err := client.Upload(file, "", toEncrypt)
	if err != nil {
		t.Fatal(err)
	}

//定义一个checkRedirect钩子，确保只有一个
//重定向到正确的URL
	redirected := false
	httpClient := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if redirected {
				return errors.New("too many redirects")
			}
			redirected = true
			expectedPath := "/bzz:/" + hash + "/"
			if req.URL.Path != expectedPath {
				return fmt.Errorf("expected redirect to %q, got %q", expectedPath, req.URL.Path)
			}
			return nil
		},
	}

//执行GET请求并断言响应
	res, err := httpClient.Get(srv.URL + "/bzz:/" + hash)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if !redirected {
		t.Fatal("expected GET /bzz:/<hash> to redirect to /bzz:/<hash>/ but it didn't")
	}
	gotData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotData, data) {
		t.Fatalf("expected response to equal %q, got %q", data, gotData)
	}
}

func TestMethodsNotAllowed(t *testing.T) {
	srv := NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()
	databytes := "bar"
	for _, c := range []struct {
		url  string
		code int
	}{
		{
			url:  fmt.Sprintf("%s/bzz-list:/", srv.URL),
			code: http.StatusMethodNotAllowed,
		}, {
			url:  fmt.Sprintf("%s/bzz-hash:/", srv.URL),
			code: http.StatusMethodNotAllowed,
		},
		{
			url:  fmt.Sprintf("%s/bzz-immutable:/", srv.URL),
			code: http.StatusMethodNotAllowed,
		},
	} {
		res, _ := http.Post(c.url, "text/plain", bytes.NewReader([]byte(databytes)))
		if res.StatusCode != c.code {
			t.Fatalf("should have failed. requested url: %s, expected code %d, got %d", c.url, c.code, res.StatusCode)
		}
	}

}

func httpDo(httpMethod string, url string, reqBody io.Reader, headers map[string]string, verbose bool, t *testing.T) (*http.Response, string) {
//生成请求
	req, err := http.NewRequest(httpMethod, url, reqBody)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if verbose {
		t.Log(req.Method, req.URL, req.Header, req.Body)
	}

//发送请求
	httpClient := &http.Client{}
	res, err := httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

//读取HTTP主体
	buffer, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body := string(buffer)

	return res, body
}

func TestGet(t *testing.T) {
	srv := NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	for _, testCase := range []struct {
		uri                string
		method             string
		headers            map[string]string
		expectedStatusCode int
		assertResponseBody string
		verbose            bool
	}{
		{
			uri:                fmt.Sprintf("%s/", srv.URL),
			method:             "GET",
			headers:            map[string]string{"Accept": "text/html"},
			expectedStatusCode: http.StatusOK,
			assertResponseBody: "Swarm: Serverless Hosting Incentivised Peer-To-Peer Storage And Content Distribution",
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/", srv.URL),
			method:             "GET",
			headers:            map[string]string{"Accept": "application/json"},
			expectedStatusCode: http.StatusOK,
			assertResponseBody: "Swarm: Please request a valid ENS or swarm hash with the appropriate bzz scheme",
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/robots.txt", srv.URL),
			method:             "GET",
			headers:            map[string]string{"Accept": "text/html"},
			expectedStatusCode: http.StatusOK,
			assertResponseBody: "User-agent: *\nDisallow: /",
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/nonexistent_path", srv.URL),
			method:             "GET",
			headers:            map[string]string{},
			expectedStatusCode: http.StatusNotFound,
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/bzz:asdf/", srv.URL),
			method:             "GET",
			headers:            map[string]string{},
			expectedStatusCode: http.StatusNotFound,
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/tbz2/", srv.URL),
			method:             "GET",
			headers:            map[string]string{},
			expectedStatusCode: http.StatusNotFound,
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/bzz-rack:/", srv.URL),
			method:             "GET",
			headers:            map[string]string{},
			expectedStatusCode: http.StatusNotFound,
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/bzz-ls", srv.URL),
			method:             "GET",
			headers:            map[string]string{},
			expectedStatusCode: http.StatusNotFound,
			verbose:            false,
		}} {
		t.Run("GET "+testCase.uri, func(t *testing.T) {
			res, body := httpDo(testCase.method, testCase.uri, nil, testCase.headers, testCase.verbose, t)
			if res.StatusCode != testCase.expectedStatusCode {
				t.Fatalf("expected status code %d but got %d", testCase.expectedStatusCode, res.StatusCode)
			}
			if testCase.assertResponseBody != "" && !strings.Contains(body, testCase.assertResponseBody) {
				t.Fatalf("expected response to be: %s but got: %s", testCase.assertResponseBody, body)
			}
		})
	}
}

func TestModify(t *testing.T) {
	srv := NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	swarmClient := swarm.NewClient(srv.URL)
	data := []byte("data")
	file := &swarm.File{
		ReadCloser: ioutil.NopCloser(bytes.NewReader(data)),
		ManifestEntry: api.ManifestEntry{
			Path:        "",
			ContentType: "text/plain",
			Size:        int64(len(data)),
		},
	}

	hash, err := swarmClient.Upload(file, "", false)
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range []struct {
		uri                   string
		method                string
		headers               map[string]string
		requestBody           []byte
		expectedStatusCode    int
		assertResponseBody    string
		assertResponseHeaders map[string]string
		verbose               bool
	}{
		{
			uri:                fmt.Sprintf("%s/bzz:/%s", srv.URL, hash),
			method:             "DELETE",
			headers:            map[string]string{},
			expectedStatusCode: http.StatusOK,
			assertResponseBody: "8b634aea26eec353ac0ecbec20c94f44d6f8d11f38d4578a4c207a84c74ef731",
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/bzz:/%s", srv.URL, hash),
			method:             "PUT",
			headers:            map[string]string{},
			expectedStatusCode: http.StatusMethodNotAllowed,
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/bzz-raw:/%s", srv.URL, hash),
			method:             "PUT",
			headers:            map[string]string{},
			expectedStatusCode: http.StatusMethodNotAllowed,
			verbose:            false,
		},
		{
			uri:                fmt.Sprintf("%s/bzz:/%s", srv.URL, hash),
			method:             "PATCH",
			headers:            map[string]string{},
			expectedStatusCode: http.StatusMethodNotAllowed,
			verbose:            false,
		},
		{
			uri:                   fmt.Sprintf("%s/bzz-raw:/", srv.URL),
			method:                "POST",
			headers:               map[string]string{},
			requestBody:           []byte("POSTdata"),
			expectedStatusCode:    http.StatusOK,
			assertResponseHeaders: map[string]string{"Content-Length": "64"},
			verbose:               false,
		},
		{
			uri:                   fmt.Sprintf("%s/bzz-raw:/encrypt", srv.URL),
			method:                "POST",
			headers:               map[string]string{},
			requestBody:           []byte("POSTdata"),
			expectedStatusCode:    http.StatusOK,
			assertResponseHeaders: map[string]string{"Content-Length": "128"},
			verbose:               false,
		},
	} {
		t.Run(testCase.method+" "+testCase.uri, func(t *testing.T) {
			reqBody := bytes.NewReader(testCase.requestBody)
			res, body := httpDo(testCase.method, testCase.uri, reqBody, testCase.headers, testCase.verbose, t)

			if res.StatusCode != testCase.expectedStatusCode {
				t.Fatalf("expected status code %d but got %d, %s", testCase.expectedStatusCode, res.StatusCode, body)
			}
			if testCase.assertResponseBody != "" && !strings.Contains(body, testCase.assertResponseBody) {
				t.Log(body)
				t.Fatalf("expected response %s but got %s", testCase.assertResponseBody, body)
			}
			for key, value := range testCase.assertResponseHeaders {
				if res.Header.Get(key) != value {
					t.Logf("expected %s=%s in HTTP response header but got %s", key, value, res.Header.Get(key))
				}
			}
		})
	}
}

func TestMultiPartUpload(t *testing.T) {
//post/bzz:/content-type:multipart/form-data
	verbose := false
//设置群
	srv := NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	url := fmt.Sprintf("%s/bzz:/", srv.URL)

	buf := new(bytes.Buffer)
	form := multipart.NewWriter(buf)
	form.WriteField("name", "John Doe")
	file1, _ := form.CreateFormFile("cv", "cv.txt")
	file1.Write([]byte("John Doe's Credentials"))
	file2, _ := form.CreateFormFile("profile_picture", "profile.jpg")
	file2.Write([]byte("imaginethisisjpegdata"))
	form.Close()

	headers := map[string]string{
		"Content-Type":   form.FormDataContentType(),
		"Content-Length": strconv.Itoa(buf.Len()),
	}
	res, body := httpDo("POST", url, buf, headers, verbose, t)

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected POST multipart/form-data to return 200, but it returned %d", res.StatusCode)
	}
	if len(body) != 64 {
		t.Fatalf("expected POST multipart/form-data to return a 64 char manifest but the answer was %d chars long", len(body))
	}
}

//testbzzgetfilewithresolver测试使用模拟的ens resolver获取文件
func TestBzzGetFileWithResolver(t *testing.T) {
	resolver := newTestResolveValidator("")
	srv := NewTestSwarmServer(t, serverFunc, resolver)
	defer srv.Close()
	fileNames := []string{"dir1/tmp1.txt", "dir2/tmp2.lock", "dir3/tmp3.rtf"}
	fileContents := []string{"tmp1textfilevalue", "tmp2lockfilelocked", "tmp3isjustaplaintextfile"}

	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)

	for i, v := range fileNames {
		size := len(fileContents[i])
		hdr := &tar.Header{
			Name:    v,
			Mode:    0644,
			Size:    int64(size),
			ModTime: time.Now(),
			Xattrs: map[string]string{
				"user.swarm.content-type": "text/plain",
			},
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}

//将文件复制到tar流中
		n, err := io.WriteString(tw, fileContents[i])
		if err != nil {
			t.Fatal(err)
		} else if n != size {
			t.Fatal("size mismatch")
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

//后焦油流
	url := srv.URL + "/bzz:/"

	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-tar")
	client := &http.Client{}
	serverResponse, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if serverResponse.StatusCode != http.StatusOK {
		t.Fatalf("err %s", serverResponse.Status)
	}
	swarmHash, err := ioutil.ReadAll(serverResponse.Body)
	serverResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
//将解析散列设置为我们刚上传内容的群散列
	hash := common.HexToHash(string(swarmHash))
	resolver.hash = &hash
	for _, v := range []struct {
		addr                string
		path                string
		expectedStatusCode  int
		expectedContentType string
		expectedFileName    string
	}{
		{
			addr:                string(swarmHash),
			path:                fileNames[0],
			expectedStatusCode:  http.StatusOK,
			expectedContentType: "text/plain",
			expectedFileName:    path.Base(fileNames[0]),
		},
		{
			addr:                "somebogusensname",
			path:                fileNames[0],
			expectedStatusCode:  http.StatusOK,
			expectedContentType: "text/plain",
			expectedFileName:    path.Base(fileNames[0]),
		},
	} {
		req, err := http.NewRequest("GET", fmt.Sprintf(srv.URL+"/bzz:/%s/%s", v.addr, v.path), nil)
		if err != nil {
			t.Fatal(err)
		}
		serverResponse, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer serverResponse.Body.Close()
		if serverResponse.StatusCode != v.expectedStatusCode {
			t.Fatalf("expected %d, got %d", v.expectedStatusCode, serverResponse.StatusCode)
		}

		if h := serverResponse.Header.Get("Content-Type"); h != v.expectedContentType {
			t.Fatalf("Content-Type header expected: %s, got %s", v.expectedContentType, h)
		}

		expectedContentDisposition := fmt.Sprintf("inline; filename=\"%s\"", v.expectedFileName)
		if h := serverResponse.Header.Get("Content-Disposition"); h != expectedContentDisposition {
			t.Fatalf("Content-Disposition header expected: %s, got: %s", expectedContentDisposition, h)
		}

	}
}

//TestResolver实现Resolver接口，并返回给定的
//如果设置了哈希，则返回“找不到名称”错误
type testResolveValidator struct {
	hash *common.Hash
}

func newTestResolveValidator(addr string) *testResolveValidator {
	r := &testResolveValidator{}
	if addr != "" {
		hash := common.HexToHash(addr)
		r.hash = &hash
	}
	return r
}

func (t *testResolveValidator) Resolve(addr string) (common.Hash, error) {
	if t.hash == nil {
		return common.Hash{}, fmt.Errorf("DNS name not found: %q", addr)
	}
	return *t.hash, nil
}

func (t *testResolveValidator) Owner(node [32]byte) (addr common.Address, err error) {
	return
}
func (t *testResolveValidator) HeaderByNumber(context.Context, *big.Int) (header *types.Header, err error) {
	return
}
