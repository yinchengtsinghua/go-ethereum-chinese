
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
//此文件是Go以太坊的一部分。
//
//Go以太坊是免费软件：您可以重新发布和/或修改它
//根据GNU通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊的分布希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU通用公共许可证了解更多详细信息。
//
//你应该已经收到一份GNU通用公共许可证的副本
//一起去以太坊吧。如果没有，请参见<http://www.gnu.org/licenses/>。

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	swarmapi "github.com/ethereum/go-ethereum/swarm/api/client"
	"github.com/ethereum/go-ethereum/swarm/testutil"
	"github.com/mattn/go-colorable"
)

func init() {
	log.PrintOrigins(true)
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(true))))
}

func TestSwarmUp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	initCluster(t)

	cases := []struct {
		name string
		f    func(t *testing.T)
	}{
		{"NoEncryption", testNoEncryption},
		{"Encrypted", testEncrypted},
		{"RecursiveNoEncryption", testRecursiveNoEncryption},
		{"RecursiveEncrypted", testRecursiveEncrypted},
		{"DefaultPathAll", testDefaultPathAll},
	}

	for _, tc := range cases {
		t.Run(tc.name, tc.f)
	}
}

//testnoEncryption运行“swarm up”生成结果文件的测试
//可通过HTTP API从所有节点获取
func testNoEncryption(t *testing.T) {
	testDefault(false, t)
}

//运行“swarm-up--encrypted”的测试加密的测试生成结果文件
//可通过HTTP API从所有节点获取
func testEncrypted(t *testing.T) {
	testDefault(true, t)
}

func testRecursiveNoEncryption(t *testing.T) {
	testRecursive(false, t)
}

func testRecursiveEncrypted(t *testing.T) {
	testRecursive(true, t)
}

func testDefault(toEncrypt bool, t *testing.T) {
	tmpFileName := testutil.TempFileWithContent(t, data)
	defer os.Remove(tmpFileName)

//将数据写入文件
	hashRegexp := `[a-f\d]{64}`
	flags := []string{
		"--bzzapi", cluster.Nodes[0].URL,
		"up",
		tmpFileName}
	if toEncrypt {
		hashRegexp = `[a-f\d]{128}`
		flags = []string{
			"--bzzapi", cluster.Nodes[0].URL,
			"up",
			"--encrypt",
			tmpFileName}
	}
//用“swarm up”上传文件，并期望得到一个哈希值
	log.Info(fmt.Sprintf("uploading file with 'swarm up'"))
	up := runSwarm(t, flags...)
	_, matches := up.ExpectRegexp(hashRegexp)
	up.ExpectExit()
	hash := matches[0]
	log.Info("file uploaded", "hash", hash)

//从每个节点的HTTP API获取文件
	for _, node := range cluster.Nodes {
		log.Info("getting file from node", "node", node.Name)

		res, err := http.Get(node.URL + "/bzz:/" + hash)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()

		reply, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 {
			t.Fatalf("expected HTTP status 200, got %s", res.Status)
		}
		if string(reply) != data {
			t.Fatalf("expected HTTP body %q, got %q", data, reply)
		}
		log.Debug("verifying uploaded file using `swarm down`")
//试着用“蜂群下降”来获取内容
		tmpDownload, err := ioutil.TempDir("", "swarm-test")
		tmpDownload = path.Join(tmpDownload, "tmpfile.tmp")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDownload)

		bzzLocator := "bzz:/" + hash
		flags = []string{
			"--bzzapi", cluster.Nodes[0].URL,
			"down",
			bzzLocator,
			tmpDownload,
		}

		down := runSwarm(t, flags...)
		down.ExpectExit()

		fi, err := os.Stat(tmpDownload)
		if err != nil {
			t.Fatalf("could not stat path: %v", err)
		}

		switch mode := fi.Mode(); {
		case mode.IsRegular():
			downloadedBytes, err := ioutil.ReadFile(tmpDownload)
			if err != nil {
				t.Fatalf("had an error reading the downloaded file: %v", err)
			}
			if !bytes.Equal(downloadedBytes, bytes.NewBufferString(data).Bytes()) {
				t.Fatalf("retrieved data and posted data not equal!")
			}

		default:
			t.Fatalf("expected to download regular file, got %s", fi.Mode())
		}
	}

	timeout := time.Duration(2 * time.Second)
	httpClient := http.Client{
		Timeout: timeout,
	}

//尝试通过从每个节点获取不存在的哈希来挤压超时
	for _, node := range cluster.Nodes {
		_, err := httpClient.Get(node.URL + "/bzz:/1023e8bae0f70be7d7b5f74343088ba408a218254391490c85ae16278e230340")
//由于NetStore在请求时有60秒的超时时间，我们正在加快超时时间。
		if err != nil && !strings.Contains(err.Error(), "Client.Timeout exceeded while awaiting headers") {
			t.Fatal(err)
		}
//由于NetStore超时需要60秒，因此此选项被禁用。
//如果是Res.StatusCode！= 404 {
//t.fatalf（“预期的HTTP状态404，得到%s”，res.status）
//}
	}
}

func testRecursive(toEncrypt bool, t *testing.T) {
	tmpUploadDir, err := ioutil.TempDir("", "swarm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpUploadDir)
//创建tmp文件
	for _, path := range []string{"tmp1", "tmp2"} {
		if err := ioutil.WriteFile(filepath.Join(tmpUploadDir, path), bytes.NewBufferString(data).Bytes(), 0644); err != nil {
			t.Fatal(err)
		}
	}

	hashRegexp := `[a-f\d]{64}`
	flags := []string{
		"--bzzapi", cluster.Nodes[0].URL,
		"--recursive",
		"up",
		tmpUploadDir}
	if toEncrypt {
		hashRegexp = `[a-f\d]{128}`
		flags = []string{
			"--bzzapi", cluster.Nodes[0].URL,
			"--recursive",
			"up",
			"--encrypt",
			tmpUploadDir}
	}
//用“swarm up”上传文件，并期望得到一个哈希值
	log.Info(fmt.Sprintf("uploading file with 'swarm up'"))
	up := runSwarm(t, flags...)
	_, matches := up.ExpectRegexp(hashRegexp)
	up.ExpectExit()
	hash := matches[0]
	log.Info("dir uploaded", "hash", hash)

//从每个节点的HTTP API获取文件
	for _, node := range cluster.Nodes {
		log.Info("getting file from node", "node", node.Name)
//试着用“蜂群下降”来获取内容
		tmpDownload, err := ioutil.TempDir("", "swarm-test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDownload)
		bzzLocator := "bzz:/" + hash
		flagss := []string{
			"--bzzapi", cluster.Nodes[0].URL,
			"down",
			"--recursive",
			bzzLocator,
			tmpDownload,
		}

		fmt.Println("downloading from swarm with recursive")
		down := runSwarm(t, flagss...)
		down.ExpectExit()

		files, err := ioutil.ReadDir(tmpDownload)
		for _, v := range files {
			fi, err := os.Stat(path.Join(tmpDownload, v.Name()))
			if err != nil {
				t.Fatalf("got an error: %v", err)
			}

			switch mode := fi.Mode(); {
			case mode.IsRegular():
				if file, err := swarmapi.Open(path.Join(tmpDownload, v.Name())); err != nil {
					t.Fatalf("encountered an error opening the file returned from the CLI: %v", err)
				} else {
					ff := make([]byte, len(data))
					io.ReadFull(file, ff)
					buf := bytes.NewBufferString(data)

					if !bytes.Equal(ff, buf.Bytes()) {
						t.Fatalf("retrieved data and posted data not equal!")
					}
				}
			default:
				t.Fatalf("this shouldnt happen")
			}
		}
		if err != nil {
			t.Fatalf("could not list files at: %v", files)
		}
	}
}

//testdefaultpathall测试群递归上传，具有相对和绝对
//默认路径和加密。
func testDefaultPathAll(t *testing.T) {
	testDefaultPath(false, false, t)
	testDefaultPath(false, true, t)
	testDefaultPath(true, false, t)
	testDefaultPath(true, true, t)
}

func testDefaultPath(toEncrypt bool, absDefaultPath bool, t *testing.T) {
	tmp, err := ioutil.TempDir("", "swarm-defaultpath-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	err = ioutil.WriteFile(filepath.Join(tmp, "index.html"), []byte("<h1>Test</h1>"), 0666)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(tmp, "robots.txt"), []byte("Disallow: /"), 0666)
	if err != nil {
		t.Fatal(err)
	}

	defaultPath := "index.html"
	if absDefaultPath {
		defaultPath = filepath.Join(tmp, defaultPath)
	}

	args := []string{
		"--bzzapi",
		cluster.Nodes[0].URL,
		"--recursive",
		"--defaultpath",
		defaultPath,
		"up",
		tmp,
	}
	if toEncrypt {
		args = append(args, "--encrypt")
	}

	up := runSwarm(t, args...)
	hashRegexp := `[a-f\d]{64,128}`
	_, matches := up.ExpectRegexp(hashRegexp)
	up.ExpectExit()
	hash := matches[0]

	client := swarmapi.NewClient(cluster.Nodes[0].URL)

	m, isEncrypted, err := client.DownloadManifest(hash)
	if err != nil {
		t.Fatal(err)
	}

	if toEncrypt != isEncrypted {
		t.Error("downloaded manifest is not encrypted")
	}

	var found bool
	var entriesCount int
	for _, e := range m.Entries {
		entriesCount++
		if e.Path == "" {
			found = true
		}
	}

	if !found {
		t.Error("manifest default entry was not found")
	}

	if entriesCount != 3 {
		t.Errorf("manifest contains %v entries, expected %v", entriesCount, 3)
	}
}
