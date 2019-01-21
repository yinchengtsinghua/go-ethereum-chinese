
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ethereum/go-ethereum/swarm/api"
	swarm "github.com/ethereum/go-ethereum/swarm/api/client"
	swarmhttp "github.com/ethereum/go-ethereum/swarm/api/http"
)

//测试清单更改测试清单添加、更新和删除
//不加密的CLI命令。
func TestManifestChange(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	testManifestChange(t, false)
}

//测试清单更改测试清单添加、更新和删除
//
func TestManifestChangeEncrypted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	testManifestChange(t, true)
}

//
//-清单添加
//
//-清单删除
//
//根清单或嵌套清单中的路径上的命令。
//参数encrypt控制是否使用加密。
func testManifestChange(t *testing.T, encrypt bool) {
	t.Parallel()
	srv := swarmhttp.NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	tmp, err := ioutil.TempDir("", "swarm-manifest-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	origDir := filepath.Join(tmp, "orig")
	if err := os.Mkdir(origDir, 0777); err != nil {
		t.Fatal(err)
	}

	indexDataFilename := filepath.Join(origDir, "index.html")
	err = ioutil.WriteFile(indexDataFilename, []byte("<h1>Test</h1>"), 0666)
	if err != nil {
		t.Fatal(err)
	}
//
//这将在路径“robots.”下生成一个嵌套清单的清单。
//这将允许在根清单和嵌套清单上测试清单更改。
	err = ioutil.WriteFile(filepath.Join(origDir, "robots.txt"), []byte("Disallow: /"), 0666)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(origDir, "robots.html"), []byte("<strong>No Robots Allowed</strong>"), 0666)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(origDir, "mutants.txt"), []byte("Frank\nMarcus"), 0666)
	if err != nil {
		t.Fatal(err)
	}

	args := []string{
		"--bzzapi",
		srv.URL,
		"--recursive",
		"--defaultpath",
		indexDataFilename,
		"up",
		origDir,
	}
	if encrypt {
		args = append(args, "--encrypt")
	}

	origManifestHash := runSwarmExpectHash(t, args...)

	checkHashLength(t, origManifestHash, encrypt)

	client := swarm.NewClient(srv.URL)

//上载新文件并使用其清单将其添加到原始清单。
	t.Run("add", func(t *testing.T) {
		humansData := []byte("Ann\nBob")
		humansDataFilename := filepath.Join(tmp, "humans.txt")
		err = ioutil.WriteFile(humansDataFilename, humansData, 0666)
		if err != nil {
			t.Fatal(err)
		}

		humansManifestHash := runSwarmExpectHash(t,
			"--bzzapi",
			srv.URL,
			"up",
			humansDataFilename,
		)

		newManifestHash := runSwarmExpectHash(t,
			"--bzzapi",
			srv.URL,
			"manifest",
			"add",
			origManifestHash,
			"humans.txt",
			humansManifestHash,
		)

		checkHashLength(t, newManifestHash, encrypt)

		newManifest := downloadManifest(t, client, newManifestHash, encrypt)

		var found bool
		for _, e := range newManifest.Entries {
			if e.Path == "humans.txt" {
				found = true
				if e.Size != int64(len(humansData)) {
					t.Errorf("expected humans.txt size %v, got %v", len(humansData), e.Size)
				}
				if e.ModTime.IsZero() {
					t.Errorf("got zero mod time for humans.txt")
				}
				ct := "text/plain; charset=utf-8"
				if e.ContentType != ct {
					t.Errorf("expected content type %q, got %q", ct, e.ContentType)
				}
				break
			}
		}
		if !found {
			t.Fatal("no humans.txt in new manifest")
		}

		checkFile(t, client, newManifestHash, "humans.txt", humansData)
	})

//上载新文件并使用其清单添加原始清单，
//但请确保该文件将位于原始文件的嵌套清单中。
	t.Run("add nested", func(t *testing.T) {
		robotsData := []byte(`{"disallow": "/"}`)
		robotsDataFilename := filepath.Join(tmp, "robots.json")
		err = ioutil.WriteFile(robotsDataFilename, robotsData, 0666)
		if err != nil {
			t.Fatal(err)
		}

		robotsManifestHash := runSwarmExpectHash(t,
			"--bzzapi",
			srv.URL,
			"up",
			robotsDataFilename,
		)

		newManifestHash := runSwarmExpectHash(t,
			"--bzzapi",
			srv.URL,
			"manifest",
			"add",
			origManifestHash,
			"robots.json",
			robotsManifestHash,
		)

		checkHashLength(t, newManifestHash, encrypt)

		newManifest := downloadManifest(t, client, newManifestHash, encrypt)

		var found bool
	loop:
		for _, e := range newManifest.Entries {
			if e.Path == "robots." {
				nestedManifest := downloadManifest(t, client, e.Hash, encrypt)
				for _, e := range nestedManifest.Entries {
					if e.Path == "json" {
						found = true
						if e.Size != int64(len(robotsData)) {
							t.Errorf("expected robots.json size %v, got %v", len(robotsData), e.Size)
						}
						if e.ModTime.IsZero() {
							t.Errorf("got zero mod time for robots.json")
						}
						ct := "application/json"
						if e.ContentType != ct {
							t.Errorf("expected content type %q, got %q", ct, e.ContentType)
						}
						break loop
					}
				}
			}
		}
		if !found {
			t.Fatal("no robots.json in new manifest")
		}

		checkFile(t, client, newManifestHash, "robots.json", robotsData)
	})

//
	t.Run("update", func(t *testing.T) {
		indexData := []byte("<h1>Ethereum Swarm</h1>")
		indexDataFilename := filepath.Join(tmp, "index.html")
		err = ioutil.WriteFile(indexDataFilename, indexData, 0666)
		if err != nil {
			t.Fatal(err)
		}

		indexManifestHash := runSwarmExpectHash(t,
			"--bzzapi",
			srv.URL,
			"up",
			indexDataFilename,
		)

		newManifestHash := runSwarmExpectHash(t,
			"--bzzapi",
			srv.URL,
			"manifest",
			"update",
			origManifestHash,
			"index.html",
			indexManifestHash,
		)

		checkHashLength(t, newManifestHash, encrypt)

		newManifest := downloadManifest(t, client, newManifestHash, encrypt)

		var found bool
		for _, e := range newManifest.Entries {
			if e.Path == "index.html" {
				found = true
				if e.Size != int64(len(indexData)) {
					t.Errorf("expected index.html size %v, got %v", len(indexData), e.Size)
				}
				if e.ModTime.IsZero() {
					t.Errorf("got zero mod time for index.html")
				}
				ct := "text/html; charset=utf-8"
				if e.ContentType != ct {
					t.Errorf("expected content type %q, got %q", ct, e.ContentType)
				}
				break
			}
		}
		if !found {
			t.Fatal("no index.html in new manifest")
		}

		checkFile(t, client, newManifestHash, "index.html", indexData)

//检查默认条目更改
		checkFile(t, client, newManifestHash, "", indexData)
	})

//上载新文件并使用其清单将文件更改为原始清单，
//
	t.Run("update nested", func(t *testing.T) {
		robotsData := []byte(`<string>Only humans allowed!!!</strong>`)
		robotsDataFilename := filepath.Join(tmp, "robots.html")
		err = ioutil.WriteFile(robotsDataFilename, robotsData, 0666)
		if err != nil {
			t.Fatal(err)
		}

		humansManifestHash := runSwarmExpectHash(t,
			"--bzzapi",
			srv.URL,
			"up",
			robotsDataFilename,
		)

		newManifestHash := runSwarmExpectHash(t,
			"--bzzapi",
			srv.URL,
			"manifest",
			"update",
			origManifestHash,
			"robots.html",
			humansManifestHash,
		)

		checkHashLength(t, newManifestHash, encrypt)

		newManifest := downloadManifest(t, client, newManifestHash, encrypt)

		var found bool
	loop:
		for _, e := range newManifest.Entries {
			if e.Path == "robots." {
				nestedManifest := downloadManifest(t, client, e.Hash, encrypt)
				for _, e := range nestedManifest.Entries {
					if e.Path == "html" {
						found = true
						if e.Size != int64(len(robotsData)) {
							t.Errorf("expected robots.html size %v, got %v", len(robotsData), e.Size)
						}
						if e.ModTime.IsZero() {
							t.Errorf("got zero mod time for robots.html")
						}
						ct := "text/html; charset=utf-8"
						if e.ContentType != ct {
							t.Errorf("expected content type %q, got %q", ct, e.ContentType)
						}
						break loop
					}
				}
			}
		}
		if !found {
			t.Fatal("no robots.html in new manifest")
		}

		checkFile(t, client, newManifestHash, "robots.html", robotsData)
	})

//从清单中删除文件。
	t.Run("remove", func(t *testing.T) {
		newManifestHash := runSwarmExpectHash(t,
			"--bzzapi",
			srv.URL,
			"manifest",
			"remove",
			origManifestHash,
			"mutants.txt",
		)

		checkHashLength(t, newManifestHash, encrypt)

		newManifest := downloadManifest(t, client, newManifestHash, encrypt)

		var found bool
		for _, e := range newManifest.Entries {
			if e.Path == "mutants.txt" {
				found = true
				break
			}
		}
		if found {
			t.Fatal("mutants.txt is not removed")
		}
	})

//从清单中删除文件，但确保该文件位于
//原始清单的嵌套清单。
	t.Run("remove nested", func(t *testing.T) {
		newManifestHash := runSwarmExpectHash(t,
			"--bzzapi",
			srv.URL,
			"manifest",
			"remove",
			origManifestHash,
			"robots.html",
		)

		checkHashLength(t, newManifestHash, encrypt)

		newManifest := downloadManifest(t, client, newManifestHash, encrypt)

		var found bool
	loop:
		for _, e := range newManifest.Entries {
			if e.Path == "robots." {
				nestedManifest := downloadManifest(t, client, e.Hash, encrypt)
				for _, e := range nestedManifest.Entries {
					if e.Path == "html" {
						found = true
						break loop
					}
				}
			}
		}
		if found {
			t.Fatal("robots.html in not removed")
		}
	})
}

//
//
func TestNestedDefaultEntryUpdate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	testNestedDefaultEntryUpdate(t, false)
}

//如果默认项为
//如果嵌套清单中的文件
//
func TestNestedDefaultEntryUpdateEncrypted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	testNestedDefaultEntryUpdate(t, true)
}

func testNestedDefaultEntryUpdate(t *testing.T, encrypt bool) {
	t.Parallel()
	srv := swarmhttp.NewTestSwarmServer(t, serverFunc, nil)
	defer srv.Close()

	tmp, err := ioutil.TempDir("", "swarm-manifest-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	origDir := filepath.Join(tmp, "orig")
	if err := os.Mkdir(origDir, 0777); err != nil {
		t.Fatal(err)
	}

	indexData := []byte("<h1>Test</h1>")
	indexDataFilename := filepath.Join(origDir, "index.html")
	err = ioutil.WriteFile(indexDataFilename, indexData, 0666)
	if err != nil {
		t.Fatal(err)
	}
//添加另一个具有公共前缀的文件作为默认项，以测试
//
	err = ioutil.WriteFile(filepath.Join(origDir, "index.txt"), []byte("Test"), 0666)
	if err != nil {
		t.Fatal(err)
	}

	args := []string{
		"--bzzapi",
		srv.URL,
		"--recursive",
		"--defaultpath",
		indexDataFilename,
		"up",
		origDir,
	}
	if encrypt {
		args = append(args, "--encrypt")
	}

	origManifestHash := runSwarmExpectHash(t, args...)

	checkHashLength(t, origManifestHash, encrypt)

	client := swarm.NewClient(srv.URL)

	newIndexData := []byte("<h1>Ethereum Swarm</h1>")
	newIndexDataFilename := filepath.Join(tmp, "index.html")
	err = ioutil.WriteFile(newIndexDataFilename, newIndexData, 0666)
	if err != nil {
		t.Fatal(err)
	}

	newIndexManifestHash := runSwarmExpectHash(t,
		"--bzzapi",
		srv.URL,
		"up",
		newIndexDataFilename,
	)

	newManifestHash := runSwarmExpectHash(t,
		"--bzzapi",
		srv.URL,
		"manifest",
		"update",
		origManifestHash,
		"index.html",
		newIndexManifestHash,
	)

	checkHashLength(t, newManifestHash, encrypt)

	newManifest := downloadManifest(t, client, newManifestHash, encrypt)

	var found bool
	for _, e := range newManifest.Entries {
		if e.Path == "index." {
			found = true
			newManifest = downloadManifest(t, client, e.Hash, encrypt)
			break
		}
	}
	if !found {
		t.Fatal("no index. path in new manifest")
	}

	found = false
	for _, e := range newManifest.Entries {
		if e.Path == "html" {
			found = true
			if e.Size != int64(len(newIndexData)) {
				t.Errorf("expected index.html size %v, got %v", len(newIndexData), e.Size)
			}
			if e.ModTime.IsZero() {
				t.Errorf("got zero mod time for index.html")
			}
			ct := "text/html; charset=utf-8"
			if e.ContentType != ct {
				t.Errorf("expected content type %q, got %q", ct, e.ContentType)
			}
			break
		}
	}
	if !found {
		t.Fatal("no html in new manifest")
	}

	checkFile(t, client, newManifestHash, "index.html", newIndexData)

//检查默认条目更改
	checkFile(t, client, newManifestHash, "", newIndexData)
}

func runSwarmExpectHash(t *testing.T, args ...string) (hash string) {
	t.Helper()
	hashRegexp := `[a-f\d]{64,128}`
	up := runSwarm(t, args...)
	_, matches := up.ExpectRegexp(hashRegexp)
	up.ExpectExit()

	if len(matches) < 1 {
		t.Fatal("no matches found")
	}
	return matches[0]
}

func checkHashLength(t *testing.T, hash string, encrypted bool) {
	t.Helper()
	l := len(hash)
	if encrypted && l != 128 {
		t.Errorf("expected hash length 128, got %v", l)
	}
	if !encrypted && l != 64 {
		t.Errorf("expected hash length 64, got %v", l)
	}
}

func downloadManifest(t *testing.T, client *swarm.Client, hash string, encrypted bool) (manifest *api.Manifest) {
	t.Helper()
	m, isEncrypted, err := client.DownloadManifest(hash)
	if err != nil {
		t.Fatal(err)
	}

	if encrypted != isEncrypted {
		t.Error("new manifest encryption flag is not correct")
	}
	return m
}

func checkFile(t *testing.T, client *swarm.Client, hash, path string, expected []byte) {
	t.Helper()
	f, err := client.Download(hash, path)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, expected) {
		t.Errorf("expected file content %q, got %q", expected, got)
	}
}
