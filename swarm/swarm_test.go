
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

package swarm

import (
	"context"
	"encoding/hex"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/api"
)

//TestNewsWarm验证repsect中的Swarm字段是否符合所提供的配置。
func TestNewSwarm(t *testing.T) {
	dir, err := ioutil.TempDir("", "swarm")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

//用于测试拨号的简单RPC终结点
	ipcEndpoint := path.Join(dir, "TestSwarm.ipc")

//Windows名称管道不在文件系统上，而是在NPF上
	if runtime.GOOS == "windows" {
		b := make([]byte, 8)
		rand.Read(b)
		ipcEndpoint = `\\.\pipe\TestSwarm-` + hex.EncodeToString(b)
	}

	_, server, err := rpc.StartIPCEndpoint(ipcEndpoint, nil)
	if err != nil {
		t.Error(err)
	}
	defer server.Stop()

	for _, tc := range []struct {
		name      string
		configure func(*api.Config)
		check     func(*testing.T, *Swarm, *api.Config)
	}{
		{
			name:      "defaults",
			configure: nil,
			check: func(t *testing.T, s *Swarm, config *api.Config) {
				if s.config != config {
					t.Error("config is not the same object")
				}
				if s.backend != nil {
					t.Error("backend is not nil")
				}
				if s.privateKey == nil {
					t.Error("private key is not set")
				}
				if !s.config.HiveParams.Discovery {
					t.Error("config.HiveParams.Discovery is false, must be true regardless the configuration")
				}
				if s.dns != nil {
					t.Error("dns initialized, but it should not be")
				}
				if s.netStore == nil {
					t.Error("netStore not initialized")
				}
				if s.streamer == nil {
					t.Error("streamer not initialized")
				}
				if s.fileStore == nil {
					t.Error("fileStore not initialized")
				}
				if s.bzz == nil {
					t.Error("bzz not initialized")
				}
				if s.ps == nil {
					t.Error("pss not initialized")
				}
				if s.api == nil {
					t.Error("api not initialized")
				}
				if s.sfs == nil {
					t.Error("swarm filesystem not initialized")
				}
			},
		},
		{
			name: "with swap",
			configure: func(config *api.Config) {
				config.SwapAPI = ipcEndpoint
				config.SwapEnabled = true
			},
			check: func(t *testing.T, s *Swarm, _ *api.Config) {
				if s.backend == nil {
					t.Error("backend is nil")
				}
			},
		},
		{
			name: "with swap disabled",
			configure: func(config *api.Config) {
				config.SwapAPI = ipcEndpoint
				config.SwapEnabled = false
			},
			check: func(t *testing.T, s *Swarm, _ *api.Config) {
				if s.backend != nil {
					t.Error("backend is not nil")
				}
			},
		},
		{
			name: "with swap enabled and api endpoint blank",
			configure: func(config *api.Config) {
				config.SwapAPI = ""
				config.SwapEnabled = true
			},
			check: func(t *testing.T, s *Swarm, _ *api.Config) {
				if s.backend != nil {
					t.Error("backend is not nil")
				}
			},
		},
		{
			name: "ens",
			configure: func(config *api.Config) {
				config.EnsAPIs = []string{
"http://127.0.0.1:8888“，
				}
			},
			check: func(t *testing.T, s *Swarm, _ *api.Config) {
				if s.dns == nil {
					t.Error("dns is not initialized")
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := api.NewConfig()

			dir, err := ioutil.TempDir("", "swarm")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			config.Path = dir

			privkey, err := crypto.GenerateKey()
			if err != nil {
				t.Fatal(err)
			}

			config.Init(privkey)

			if tc.configure != nil {
				tc.configure(config)
			}

			s, err := NewSwarm(config, nil)
			if err != nil {
				t.Fatal(err)
			}

			if tc.check != nil {
				tc.check(t, s, config)
			}
		})
	}
}

func TestParseEnsAPIAddress(t *testing.T) {
	for _, x := range []struct {
		description string
		value       string
		tld         string
		endpoint    string
		addr        common.Address
	}{
		{
			description: "IPC endpoint",
			value:       "/data/testnet/geth.ipc",
			endpoint:    "/data/testnet/geth.ipc",
		},
		{
			description: "HTTP endpoint",
value:       "http://127.0.0.1:1234“，
endpoint:    "http://127.0.0.1:1234“，
		},
		{
			description: "WS endpoint",
value:       "ws://127.0.0.1:1234“，
endpoint:    "ws://127.0.0.1:1234“，
		},
		{
			description: "IPC Endpoint and TLD",
			value:       "test:/data/testnet/geth.ipc",
			endpoint:    "/data/testnet/geth.ipc",
			tld:         "test",
		},
		{
			description: "HTTP endpoint and TLD",
value:       "test:http://127.0.0.1:1234“，
endpoint:    "http://127.0.0.1:1234“，
			tld:         "test",
		},
		{
			description: "WS endpoint and TLD",
value:       "test:ws://127.0.0.1:1234“，
endpoint:    "ws://127.0.0.1:1234“，
			tld:         "test",
		},
		{
			description: "IPC Endpoint and contract address",
			value:       "314159265dD8dbb310642f98f50C066173C1259b@/data/testnet/geth.ipc",
			endpoint:    "/data/testnet/geth.ipc",
			addr:        common.HexToAddress("314159265dD8dbb310642f98f50C066173C1259b"),
		},
		{
			description: "HTTP endpoint and contract address",
value:       "314159265dD8dbb310642f98f50C066173C1259b@http://127.0.0.1:1234“，
endpoint:    "http://127.0.0.1:1234“，
			addr:        common.HexToAddress("314159265dD8dbb310642f98f50C066173C1259b"),
		},
		{
			description: "WS endpoint and contract address",
value:       "314159265dD8dbb310642f98f50C066173C1259b@ws://127.0.0.1:1234“，
endpoint:    "ws://127.0.0.1:1234“，
			addr:        common.HexToAddress("314159265dD8dbb310642f98f50C066173C1259b"),
		},
		{
			description: "IPC Endpoint, TLD and contract address",
			value:       "test:314159265dD8dbb310642f98f50C066173C1259b@/data/testnet/geth.ipc",
			endpoint:    "/data/testnet/geth.ipc",
			addr:        common.HexToAddress("314159265dD8dbb310642f98f50C066173C1259b"),
			tld:         "test",
		},
		{
			description: "HTTP endpoint, TLD and contract address",
value:       "eth:314159265dD8dbb310642f98f50C066173C1259b@http://127.0.0.1:1234“，
endpoint:    "http://127.0.0.1:1234“，
			addr:        common.HexToAddress("314159265dD8dbb310642f98f50C066173C1259b"),
			tld:         "eth",
		},
		{
			description: "WS endpoint, TLD and contract address",
value:       "eth:314159265dD8dbb310642f98f50C066173C1259b@ws://127.0.0.1:1234“，
endpoint:    "ws://127.0.0.1:1234“，
			addr:        common.HexToAddress("314159265dD8dbb310642f98f50C066173C1259b"),
			tld:         "eth",
		},
	} {
		t.Run(x.description, func(t *testing.T) {
			tld, endpoint, addr := parseEnsAPIAddress(x.value)
			if endpoint != x.endpoint {
				t.Errorf("expected Endpoint %q, got %q", x.endpoint, endpoint)
			}
			if addr != x.addr {
				t.Errorf("expected ContractAddress %q, got %q", x.addr.String(), addr.String())
			}
			if tld != x.tld {
				t.Errorf("expected TLD %q, got %q", x.tld, tld)
			}
		})
	}
}

//testlocalstoreandretrieve在上载不同大小文件的情况下运行多个测试
//使用api存储到单个swarm实例，并根据返回的内容进行检查
//通过API检索函数。
//
//此测试旨在验证chunker store和join函数的功能。
//以及它们在群中的相互作用，而不将结果与
//另一个chunker实现，在swarm/storage测试中完成。
func TestLocalStoreAndRetrieve(t *testing.T) {
	config := api.NewConfig()

	dir, err := ioutil.TempDir("", "node")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	config.Path = dir

	privkey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	config.Init(privkey)

	swarm, err := NewSwarm(config, nil)
	if err != nil {
		t.Fatal(err)
	}

//默认情况下，只测试单独的块情况
	sizes := []int{1, 60, 4097, 524288 + 1, 7*524288 + 1}

	if *longrunning {
//如果设置了-longruning标志，则测试更广泛的案例集
		sizes = append(sizes, 83, 179, 253, 1024, 4095, 4096, 8191, 8192, 8193, 12287, 12288, 12289, 123456, 2345678, 67298391, 524288, 524288+4096, 524288+4097, 7*524288, 7*524288+4096, 7*524288+4097, 128*524288+1, 128*524288, 128*524288+4096, 128*524288+4097, 816778334)
	}
	for _, n := range sizes {
		testLocalStoreAndRetrieve(t, swarm, n, true)
		testLocalStoreAndRetrieve(t, swarm, n, false)
	}
}

//testlocalstoreandretrieve正在使用单个swarm实例上传
//一个长度为n的文件，带有可选的随机数据，使用api存储函数，
//并检查同一实例上API检索函数的输出。
//这是针对问题的回归测试
//网址：https://github.com/ethersphere/go-ethereum/issues/639
//如果金字塔chunker没有正确拆分长度为
//是否存在块和树参数的边缘情况，取决于是否存在
//是一个只有一个数据块的树块以及压缩功能
//改变了树。
func testLocalStoreAndRetrieve(t *testing.T, swarm *Swarm, n int, randomData bool) {
	slice := make([]byte, n)
	if randomData {
		rand.Seed(time.Now().UnixNano())
		rand.Read(slice)
	}
	dataPut := string(slice)

	ctx := context.TODO()
	k, wait, err := swarm.api.Store(ctx, strings.NewReader(dataPut), int64(len(dataPut)), false)
	if err != nil {
		t.Fatal(err)
	}
	if wait != nil {
		err = wait(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	r, _ := swarm.api.Retrieve(context.TODO(), k)

	d, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	dataGet := string(d)

	if len(dataPut) != len(dataGet) {
		t.Fatalf("data not matched: length expected %v, got %v", len(dataPut), len(dataGet))
	} else {
		if dataPut != dataGet {
			t.Fatal("data not matched")
		}
	}
}
