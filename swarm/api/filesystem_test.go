
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

package api

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

var testDownloadDir, _ = ioutil.TempDir(os.TempDir(), "bzz-test")

func testFileSystem(t *testing.T, f func(*FileSystem, bool)) {
	testAPI(t, func(api *API, toEncrypt bool) {
		f(NewFileSystem(api), toEncrypt)
	})
}

func readPath(t *testing.T, parts ...string) string {
	file := filepath.Join(parts...)
	content, err := ioutil.ReadFile(file)

	if err != nil {
		t.Fatalf("unexpected error reading '%v': %v", file, err)
	}
	return string(content)
}

func TestApiDirUpload0(t *testing.T) {
	testFileSystem(t, func(fs *FileSystem, toEncrypt bool) {
		api := fs.api
		bzzhash, err := fs.Upload(filepath.Join("testdata", "test0"), "", toEncrypt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content := readPath(t, "testdata", "test0", "index.html")
		resp := testGet(t, api, bzzhash, "index.html")
		exp := expResponse(content, "text/html; charset=utf-8", 0)
		checkResponse(t, resp, exp)

		content = readPath(t, "testdata", "test0", "index.css")
		resp = testGet(t, api, bzzhash, "index.css")
		exp = expResponse(content, "text/css; charset=utf-8", 0)
		checkResponse(t, resp, exp)

		addr := storage.Address(common.Hex2Bytes(bzzhash))
		_, _, _, _, err = api.Get(context.TODO(), NOOPDecrypt, addr, "")
		if err == nil {
			t.Fatalf("expected error: %v", err)
		}

		downloadDir := filepath.Join(testDownloadDir, "test0")
		defer os.RemoveAll(downloadDir)
		err = fs.Download(bzzhash, downloadDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		newbzzhash, err := fs.Upload(downloadDir, "", toEncrypt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
//TODO:当前哈希在加密的情况下不具有确定性
		if !toEncrypt && bzzhash != newbzzhash {
			t.Fatalf("download %v reuploaded has incorrect hash, expected %v, got %v", downloadDir, bzzhash, newbzzhash)
		}
	})
}

func TestApiDirUploadModify(t *testing.T) {
	testFileSystem(t, func(fs *FileSystem, toEncrypt bool) {
		api := fs.api
		bzzhash, err := fs.Upload(filepath.Join("testdata", "test0"), "", toEncrypt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		addr := storage.Address(common.Hex2Bytes(bzzhash))
		addr, err = api.Modify(context.TODO(), addr, "index.html", "", "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		index, err := ioutil.ReadFile(filepath.Join("testdata", "test0", "index.html"))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		ctx := context.TODO()
		hash, wait, err := api.Store(ctx, bytes.NewReader(index), int64(len(index)), toEncrypt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		err = wait(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		addr, err = api.Modify(context.TODO(), addr, "index2.html", hash.Hex(), "text/html; charset=utf-8")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		addr, err = api.Modify(context.TODO(), addr, "img/logo.png", hash.Hex(), "text/html; charset=utf-8")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		bzzhash = addr.Hex()

		content := readPath(t, "testdata", "test0", "index.html")
		resp := testGet(t, api, bzzhash, "index2.html")
		exp := expResponse(content, "text/html; charset=utf-8", 0)
		checkResponse(t, resp, exp)

		resp = testGet(t, api, bzzhash, "img/logo.png")
		exp = expResponse(content, "text/html; charset=utf-8", 0)
		checkResponse(t, resp, exp)

		content = readPath(t, "testdata", "test0", "index.css")
		resp = testGet(t, api, bzzhash, "index.css")
		exp = expResponse(content, "text/css; charset=utf-8", 0)
		checkResponse(t, resp, exp)

		_, _, _, _, err = api.Get(context.TODO(), nil, addr, "")
		if err == nil {
			t.Errorf("expected error: %v", err)
		}
	})
}

func TestApiDirUploadWithRootFile(t *testing.T) {
	testFileSystem(t, func(fs *FileSystem, toEncrypt bool) {
		api := fs.api
		bzzhash, err := fs.Upload(filepath.Join("testdata", "test0"), "index.html", toEncrypt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		content := readPath(t, "testdata", "test0", "index.html")
		resp := testGet(t, api, bzzhash, "")
		exp := expResponse(content, "text/html; charset=utf-8", 0)
		checkResponse(t, resp, exp)
	})
}

func TestApiFileUpload(t *testing.T) {
	testFileSystem(t, func(fs *FileSystem, toEncrypt bool) {
		api := fs.api
		bzzhash, err := fs.Upload(filepath.Join("testdata", "test0", "index.html"), "", toEncrypt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		content := readPath(t, "testdata", "test0", "index.html")
		resp := testGet(t, api, bzzhash, "index.html")
		exp := expResponse(content, "text/html; charset=utf-8", 0)
		checkResponse(t, resp, exp)
	})
}

func TestApiFileUploadWithRootFile(t *testing.T) {
	testFileSystem(t, func(fs *FileSystem, toEncrypt bool) {
		api := fs.api
		bzzhash, err := fs.Upload(filepath.Join("testdata", "test0", "index.html"), "index.html", toEncrypt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		content := readPath(t, "testdata", "test0", "index.html")
		resp := testGet(t, api, bzzhash, "")
		exp := expResponse(content, "text/html; charset=utf-8", 0)
		checkResponse(t, resp, exp)
	})
}
