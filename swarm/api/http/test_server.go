
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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/swarm/api"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/feed"
)

type TestServer interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

func NewTestSwarmServer(t *testing.T, serverFunc func(*api.API) TestServer, resolver api.Resolver) *TestSwarmServer {
	dir, err := ioutil.TempDir("", "swarm-storage-test")
	if err != nil {
		t.Fatal(err)
	}
	storeparams := storage.NewDefaultLocalStoreParams()
	storeparams.DbCapacity = 5000000
	storeparams.CacheCapacity = 5000
	storeparams.Init(dir)
	localStore, err := storage.NewLocalStore(storeparams, nil)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatal(err)
	}
	fileStore := storage.NewFileStore(localStore, storage.NewFileStoreParams())

//Swarm Feeds测试设置
	feedsDir, err := ioutil.TempDir("", "swarm-feeds-test")
	if err != nil {
		t.Fatal(err)
	}

	rhparams := &feed.HandlerParams{}
	rh, err := feed.NewTestHandler(feedsDir, rhparams)
	if err != nil {
		t.Fatal(err)
	}

	a := api.NewAPI(fileStore, resolver, rh.Handler, nil)
	srv := httptest.NewServer(serverFunc(a))
	tss := &TestSwarmServer{
		Server:    srv,
		FileStore: fileStore,
		dir:       dir,
		Hasher:    storage.MakeHashFunc(storage.DefaultHash)(),
		cleanup: func() {
			srv.Close()
			rh.Close()
			os.RemoveAll(dir)
			os.RemoveAll(feedsDir)
		},
		CurrentTime: 42,
	}
	feed.TimestampProvider = tss
	return tss
}

type TestSwarmServer struct {
	*httptest.Server
	Hasher      storage.SwarmHash
	FileStore   *storage.FileStore
	dir         string
	cleanup     func()
	CurrentTime uint64
}

func (t *TestSwarmServer) Close() {
	t.cleanup()
}

func (t *TestSwarmServer) Now() feed.Timestamp {
	return feed.Timestamp{Time: t.CurrentTime}
}
