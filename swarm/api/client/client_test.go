
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

package client

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/feed/lookup"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/swarm/api"
	swarmhttp "github.com/ethereum/go-ethereum/swarm/api/http"
	"github.com/ethereum/go-ethereum/swarm/storage/feed"
)

func serverFunc(api *api.API) swarmhttp.TestServer {
	return swarmhttp.NewServer(api, "")
}

//测试客户端上传下载原始测试上传和下载原始数据到Swarm
func TestClientUploadDownloadRaw(t *testing.T) {
	testClientUploadDownloadRaw(false, t)
}
func TestClientUploadDownloadRawEncrypted(t *testing.T) {
	testClientUploadDownloadRaw(true, t)
}

func testClientUploadDownloadRaw(toEncrypt bool, t *testing.T) {
	srv := swarmhttp.NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	client := NewClient(srv.URL)

//上传一些原始数据
	data := []byte("foo123")
	hash, err := client.UploadRaw(bytes.NewReader(data), int64(len(data)), toEncrypt)
	if err != nil {
		t.Fatal(err)
	}

//检查我们是否可以下载相同的数据
	res, isEncrypted, err := client.DownloadRaw(hash)
	if err != nil {
		t.Fatal(err)
	}
	if isEncrypted != toEncrypt {
		t.Fatalf("Expected encyption status %v got %v", toEncrypt, isEncrypted)
	}
	defer res.Close()
	gotData, err := ioutil.ReadAll(res)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotData, data) {
		t.Fatalf("expected downloaded data to be %q, got %q", data, gotData)
	}
}

//测试客户端上传下载文件测试上传和下载文件到Swarm
//清单
func TestClientUploadDownloadFiles(t *testing.T) {
	testClientUploadDownloadFiles(false, t)
}

func TestClientUploadDownloadFilesEncrypted(t *testing.T) {
	testClientUploadDownloadFiles(true, t)
}

func testClientUploadDownloadFiles(toEncrypt bool, t *testing.T) {
	srv := swarmhttp.NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	client := NewClient(srv.URL)
	upload := func(manifest, path string, data []byte) string {
		file := &File{
			ReadCloser: ioutil.NopCloser(bytes.NewReader(data)),
			ManifestEntry: api.ManifestEntry{
				Path:        path,
				ContentType: "text/plain",
				Size:        int64(len(data)),
			},
		}
		hash, err := client.Upload(file, manifest, toEncrypt)
		if err != nil {
			t.Fatal(err)
		}
		return hash
	}
	checkDownload := func(manifest, path string, expected []byte) {
		file, err := client.Download(manifest, path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if file.Size != int64(len(expected)) {
			t.Fatalf("expected downloaded file to be %d bytes, got %d", len(expected), file.Size)
		}
		if file.ContentType != "text/plain" {
			t.Fatalf("expected downloaded file to have type %q, got %q", "text/plain", file.ContentType)
		}
		data, err := ioutil.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, expected) {
			t.Fatalf("expected downloaded data to be %q, got %q", expected, data)
		}
	}

//将文件上载到清单的根目录
	rootData := []byte("some-data")
	rootHash := upload("", "", rootData)

//检查我们是否可以下载根文件
	checkDownload(rootHash, "", rootData)

//将另一个文件上载到同一清单
	otherData := []byte("some-other-data")
	newHash := upload(rootHash, "some/other/path", otherData)

//检查我们可以从新清单下载这两个文件
	checkDownload(newHash, "", rootData)
	checkDownload(newHash, "some/other/path", otherData)

//用不同的数据替换根文件
	newHash = upload(newHash, "", otherData)

//检查两个文件是否都有其他数据
	checkDownload(newHash, "", otherData)
	checkDownload(newHash, "some/other/path", otherData)
}

var testDirFiles = []string{
	"file1.txt",
	"file2.txt",
	"dir1/file3.txt",
	"dir1/file4.txt",
	"dir2/file5.txt",
	"dir2/dir3/file6.txt",
	"dir2/dir4/file7.txt",
	"dir2/dir4/file8.txt",
}

func newTestDirectory(t *testing.T) string {
	dir, err := ioutil.TempDir("", "swarm-client-test")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range testDirFiles {
		path := filepath.Join(dir, file)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			os.RemoveAll(dir)
			t.Fatalf("error creating dir for %s: %s", path, err)
		}
		if err := ioutil.WriteFile(path, []byte(file), 0644); err != nil {
			os.RemoveAll(dir)
			t.Fatalf("error writing file %s: %s", path, err)
		}
	}

	return dir
}

//测试客户端上载下载目录测试上载和下载
//Swarm清单的文件目录
func TestClientUploadDownloadDirectory(t *testing.T) {
	srv := swarmhttp.NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	dir := newTestDirectory(t)
	defer os.RemoveAll(dir)

//上传目录
	client := NewClient(srv.URL)
	defaultPath := testDirFiles[0]
	hash, err := client.UploadDirectory(dir, defaultPath, "", false)
	if err != nil {
		t.Fatalf("error uploading directory: %s", err)
	}

//检查我们是否可以下载单独的文件
	checkDownloadFile := func(path string, expected []byte) {
		file, err := client.Download(hash, path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		data, err := ioutil.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, expected) {
			t.Fatalf("expected data to be %q, got %q", expected, data)
		}
	}
	for _, file := range testDirFiles {
		checkDownloadFile(file, []byte(file))
	}

//检查我们是否可以下载默认路径
	checkDownloadFile("", []byte(testDirFiles[0]))

//检查我们可以下载目录
	tmp, err := ioutil.TempDir("", "swarm-client-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	if err := client.DownloadDirectory(hash, "", tmp, ""); err != nil {
		t.Fatal(err)
	}
	for _, file := range testDirFiles {
		data, err := ioutil.ReadFile(filepath.Join(tmp, file))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, []byte(file)) {
			t.Fatalf("expected data to be %q, got %q", file, data)
		}
	}
}

//testclientfilelist在swarm清单中列出文件的测试
func TestClientFileList(t *testing.T) {
	testClientFileList(false, t)
}

func TestClientFileListEncrypted(t *testing.T) {
	testClientFileList(true, t)
}

func testClientFileList(toEncrypt bool, t *testing.T) {
	srv := swarmhttp.NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	dir := newTestDirectory(t)
	defer os.RemoveAll(dir)

	client := NewClient(srv.URL)
	hash, err := client.UploadDirectory(dir, "", "", toEncrypt)
	if err != nil {
		t.Fatalf("error uploading directory: %s", err)
	}

	ls := func(prefix string) []string {
		list, err := client.List(hash, prefix, "")
		if err != nil {
			t.Fatal(err)
		}
		paths := make([]string, 0, len(list.CommonPrefixes)+len(list.Entries))
		paths = append(paths, list.CommonPrefixes...)
		for _, entry := range list.Entries {
			paths = append(paths, entry.Path)
		}
		sort.Strings(paths)
		return paths
	}

	tests := map[string][]string{
		"":                    {"dir1/", "dir2/", "file1.txt", "file2.txt"},
		"file":                {"file1.txt", "file2.txt"},
		"file1":               {"file1.txt"},
		"file2.txt":           {"file2.txt"},
		"file12":              {},
		"dir":                 {"dir1/", "dir2/"},
		"dir1":                {"dir1/"},
		"dir1/":               {"dir1/file3.txt", "dir1/file4.txt"},
		"dir1/file":           {"dir1/file3.txt", "dir1/file4.txt"},
		"dir1/file3.txt":      {"dir1/file3.txt"},
		"dir1/file34":         {},
		"dir2/":               {"dir2/dir3/", "dir2/dir4/", "dir2/file5.txt"},
		"dir2/file":           {"dir2/file5.txt"},
		"dir2/dir":            {"dir2/dir3/", "dir2/dir4/"},
		"dir2/dir3/":          {"dir2/dir3/file6.txt"},
		"dir2/dir4/":          {"dir2/dir4/file7.txt", "dir2/dir4/file8.txt"},
		"dir2/dir4/file":      {"dir2/dir4/file7.txt", "dir2/dir4/file8.txt"},
		"dir2/dir4/file7.txt": {"dir2/dir4/file7.txt"},
		"dir2/dir4/file78":    {},
	}
	for prefix, expected := range tests {
		actual := ls(prefix)
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("expected prefix %q to return %v, got %v", prefix, expected, actual)
		}
	}
}

//testclientmultipartupload测试使用多部分将文件上载到swarm
//上传
func TestClientMultipartUpload(t *testing.T) {
	srv := swarmhttp.NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

//定义上载程序，该上载程序使用某些数据上载testdir文件
	data := []byte("some-data")
	uploader := UploaderFunc(func(upload UploadFn) error {
		for _, name := range testDirFiles {
			file := &File{
				ReadCloser: ioutil.NopCloser(bytes.NewReader(data)),
				ManifestEntry: api.ManifestEntry{
					Path:        name,
					ContentType: "text/plain",
					Size:        int64(len(data)),
				},
			}
			if err := upload(file); err != nil {
				return err
			}
		}
		return nil
	})

//以多部分上载方式上载文件
	client := NewClient(srv.URL)
	hash, err := client.MultipartUpload("", uploader)
	if err != nil {
		t.Fatal(err)
	}

//检查我们是否可以下载单独的文件
	checkDownloadFile := func(path string) {
		file, err := client.Download(hash, path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		gotData, err := ioutil.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(gotData, data) {
			t.Fatalf("expected data to be %q, got %q", data, gotData)
		}
	}
	for _, file := range testDirFiles {
		checkDownloadFile(file)
	}
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
func TestClientBzzWithFeed(t *testing.T) {

	signer, _ := newTestSigner()

//初始化Swarm测试服务器
	srv := swarmhttp.NewTestSwarmServer(t, serverFunc, nil)
	swarmClient := NewClient(srv.URL)
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
//源清单哈希（相同）->清单哈希（2）->数据（2）
//
	`)

//从包含上述数据的内存中创建虚拟文件
	f := &File{
		ReadCloser: ioutil.NopCloser(bytes.NewReader(dataBytes)),
		ManifestEntry: api.ManifestEntry{
			ContentType: "text/plain",
			Mode:        0660,
			Size:        int64(len(dataBytes)),
		},
	}

//将数据上载到bzz://并检索内容寻址清单哈希，十六进制编码。
	manifestAddressHex, err := swarmClient.Upload(f, "", false)
	if err != nil {
		t.Fatalf("Error creating manifest: %s", err)
	}

//将十六进制编码的清单哈希转换为32字节的切片
	manifestAddress := common.FromHex(manifestAddressHex)

	if len(manifestAddress) != storage.AddressLength {
		t.Fatalf("Something went wrong. Got a hash of an unexpected length. Expected %d bytes. Got %d", storage.AddressLength, len(manifestAddress))
	}

//现在创建一个**源清单**。为此，我们需要一个主题：
	topic, _ := feed.NewTopic("interesting topic indeed", nil)

//生成源请求以更新数据
	request := feed.NewFirstRequest(topic)

//将清单的32字节地址放入源更新中
	request.SetData(manifestAddress)

//签署更新
	if err := request.Sign(signer); err != nil {
		t.Fatalf("Error signing update: %s", err)
	}

//发布更新，同时请求创建一个**源清单**。
	feedManifestAddressHex, err := swarmClient.CreateFeedWithManifest(request)
	if err != nil {
		t.Fatalf("Error creating feed manifest: %s", err)
	}

//检查我们是否收到了预期的确切**源清单**。
//给定主题和用户对更新进行签名：
	correctFeedManifestAddrHex := "747c402e5b9dc715a25a4393147512167bab018a007fad7cdcd9adc7fce1ced2"
	if feedManifestAddressHex != correctFeedManifestAddrHex {
		t.Fatalf("Response feed manifest mismatch, expected '%s', got '%s'", correctFeedManifestAddrHex, feedManifestAddressHex)
	}

//检查我们在尝试获取包含组合清单的源更新时是否出现未找到错误
	_, err = swarmClient.QueryFeed(nil, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != ErrNoFeedUpdatesFound {
		t.Fatalf("Expected to receive ErrNoFeedUpdatesFound error. Got: %s", err)
	}

//如果我们直接查询feed，我们应该得到**manifest hash**返回：
	reader, err := swarmClient.QueryFeed(nil, correctFeedManifestAddrHex)
	if err != nil {
		t.Fatalf("Error retrieving feed updates: %s", err)
	}
	defer reader.Close()
	gotData, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

//检查是否确实检索到了**清单哈希**。
	if !bytes.Equal(manifestAddress, gotData) {
		t.Fatalf("Expected: %v, got %v", manifestAddress, gotData)
	}

//现在，我们正在寻找的最后一个测试是：使用bzz://<feed manifest>并且应该解析所有清单
//直接返回原始数据：
	f, err = swarmClient.Download(feedManifestAddressHex, "")
	if err != nil {
		t.Fatal(err)
	}
	gotData, err = ioutil.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}

//检查并返回原始数据：
	if !bytes.Equal(dataBytes, gotData) {
		t.Fatalf("Expected: %v, got %v", manifestAddress, gotData)
	}
}

//TestClientCreateUpdateFeed将检查是否可以通过HTTP客户端创建和更新源。
func TestClientCreateUpdateFeed(t *testing.T) {

	signer, _ := newTestSigner()

	srv := swarmhttp.NewTestSwarmServer(t, serverFunc, nil)
	client := NewClient(srv.URL)
	defer srv.Close()

//设置源更新的原始数据
	databytes := []byte("En un lugar de La Mancha, de cuyo nombre no quiero acordarme...")

//我们的订阅源主题名称
	topic, _ := feed.NewTopic("El Quijote", nil)
	createRequest := feed.NewFirstRequest(topic)

	createRequest.SetData(databytes)
	if err := createRequest.Sign(signer); err != nil {
		t.Fatalf("Error signing update: %s", err)
	}

	feedManifestHash, err := client.CreateFeedWithManifest(createRequest)
	if err != nil {
		t.Fatal(err)
	}

	correctManifestAddrHex := "0e9b645ebc3da167b1d56399adc3276f7a08229301b72a03336be0e7d4b71882"
	if feedManifestHash != correctManifestAddrHex {
		t.Fatalf("Response feed manifest mismatch, expected '%s', got '%s'", correctManifestAddrHex, feedManifestHash)
	}

	reader, err := client.QueryFeed(nil, correctManifestAddrHex)
	if err != nil {
		t.Fatalf("Error retrieving feed updates: %s", err)
	}
	defer reader.Close()
	gotData, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(databytes, gotData) {
		t.Fatalf("Expected: %v, got %v", databytes, gotData)
	}

//定义不同的数据
	databytes = []byte("... no ha mucho tiempo que vivía un hidalgo de los de lanza en astillero ...")

	updateRequest, err := client.GetFeedRequest(nil, correctManifestAddrHex)
	if err != nil {
		t.Fatalf("Error retrieving update request template: %s", err)
	}

	updateRequest.SetData(databytes)
	if err := updateRequest.Sign(signer); err != nil {
		t.Fatalf("Error signing update: %s", err)
	}

	if err = client.UpdateFeed(updateRequest); err != nil {
		t.Fatalf("Error updating feed: %s", err)
	}

	reader, err = client.QueryFeed(nil, correctManifestAddrHex)
	if err != nil {
		t.Fatalf("Error retrieving feed updates: %s", err)
	}
	defer reader.Close()
	gotData, err = ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(databytes, gotData) {
		t.Fatalf("Expected: %v, got %v", databytes, gotData)
	}

//现在尝试在没有清单的情况下检索源更新

	fd := &feed.Feed{
		Topic: topic,
		User:  signer.Address(),
	}

	lookupParams := feed.NewQueryLatest(fd, lookup.NoClue)
	reader, err = client.QueryFeed(lookupParams, "")
	if err != nil {
		t.Fatalf("Error retrieving feed updates: %s", err)
	}
	defer reader.Close()
	gotData, err = ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(databytes, gotData) {
		t.Fatalf("Expected: %v, got %v", databytes, gotData)
	}
}
