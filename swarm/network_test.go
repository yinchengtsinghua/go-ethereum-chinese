
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/api"
	"github.com/ethereum/go-ethereum/swarm/network/simulation"
	"github.com/ethereum/go-ethereum/swarm/storage"
	colorable "github.com/mattn/go-colorable"
)

var (
	loglevel     = flag.Int("loglevel", 2, "verbosity of logs")
	longrunning  = flag.Bool("longrunning", false, "do run long-running tests")
	waitKademlia = flag.Bool("waitkademlia", false, "wait for healthy kademlia before checking files availability")
)

func init() {
	rand.Seed(time.Now().UnixNano())

	flag.Parse()

	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(true))))
}

//testswarmnetwork使用
//网络仿真中的静态和动态群节点
//将文件上载到每个节点并检索它们。
func TestSwarmNetwork(t *testing.T) {
	for _, tc := range []struct {
		name     string
		steps    []testSwarmNetworkStep
		options  *testSwarmNetworkOptions
		disabled bool
	}{
		{
			name: "10_nodes",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 10,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout: 45 * time.Second,
			},
		},
		{
			name: "10_nodes_skip_check",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 10,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout:   45 * time.Second,
				SkipCheck: true,
			},
		},
		{
			name: "50_nodes",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 50,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout: 3 * time.Minute,
			},
			disabled: !*longrunning,
		},
		{
			name: "50_nodes_skip_check",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 50,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout:   3 * time.Minute,
				SkipCheck: true,
			},
			disabled: !*longrunning,
		},
		{
			name: "inc_node_count",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 2,
				},
				{
					nodeCount: 5,
				},
				{
					nodeCount: 10,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout: 90 * time.Second,
			},
			disabled: !*longrunning,
		},
		{
			name: "dec_node_count",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 10,
				},
				{
					nodeCount: 6,
				},
				{
					nodeCount: 3,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout: 90 * time.Second,
			},
			disabled: !*longrunning,
		},
		{
			name: "dec_inc_node_count",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 3,
				},
				{
					nodeCount: 1,
				},
				{
					nodeCount: 5,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout: 90 * time.Second,
			},
		},
		{
			name: "inc_dec_node_count",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 3,
				},
				{
					nodeCount: 5,
				},
				{
					nodeCount: 25,
				},
				{
					nodeCount: 10,
				},
				{
					nodeCount: 4,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout: 5 * time.Minute,
			},
			disabled: !*longrunning,
		},
		{
			name: "inc_dec_node_count_skip_check",
			steps: []testSwarmNetworkStep{
				{
					nodeCount: 3,
				},
				{
					nodeCount: 5,
				},
				{
					nodeCount: 25,
				},
				{
					nodeCount: 10,
				},
				{
					nodeCount: 4,
				},
			},
			options: &testSwarmNetworkOptions{
				Timeout:   5 * time.Minute,
				SkipCheck: true,
			},
			disabled: !*longrunning,
		},
	} {
		if tc.disabled {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			testSwarmNetwork(t, tc.options, tc.steps...)
		})
	}
}

//testswarmnetworkstep是配置
//模拟网络的状态。
type testSwarmNetworkStep struct {
//必须处于向上状态的群节点数
	nodeCount int
}

//文件表示在特定节点上上载的文件。
type file struct {
	addr   storage.Address
	data   string
	nodeID enode.ID
}

//check表示对检索到的文件的引用
//来自特定节点。
type check struct {
	key    string
	nodeID enode.ID
}

//testswarmnetworkoptions包含用于运行的可选参数
//测试工作。
type testSwarmNetworkOptions struct {
	Timeout   time.Duration
	SkipCheck bool
}

//testswarmnetwork是用于测试不同
//静态和动态群网络模拟。
//它负责：
//-建立一个群网络仿真，并根据步骤更新网络中每个步骤的节点数。
//-每一步向每个节点上载一个唯一的文件。
//-可能等待每个节点上的花环健康。
//-检查是否可以从所有节点检索文件。
func testSwarmNetwork(t *testing.T, o *testSwarmNetworkOptions, steps ...testSwarmNetworkStep) {

	if o == nil {
		o = new(testSwarmNetworkOptions)
	}

	sim := simulation.New(map[string]simulation.ServiceFunc{
		"swarm": func(ctx *adapters.ServiceContext, bucket *sync.Map) (s node.Service, cleanup func(), err error) {
			config := api.NewConfig()

			dir, err := ioutil.TempDir("", "swarm-network-test-node")
			if err != nil {
				return nil, nil, err
			}
			cleanup = func() {
				err := os.RemoveAll(dir)
				if err != nil {
					log.Error("cleaning up swarm temp dir", "err", err)
				}
			}

			config.Path = dir

			privkey, err := crypto.GenerateKey()
			if err != nil {
				return nil, cleanup, err
			}

			config.Init(privkey)
			config.DeliverySkipCheck = o.SkipCheck
			config.Port = ""

			swarm, err := NewSwarm(config, nil)
			if err != nil {
				return nil, cleanup, err
			}
			bucket.Store(simulation.BucketKeyKademlia, swarm.bzz.Hive.Kademlia)
			log.Info("new swarm", "bzzKey", config.BzzKey, "baseAddr", fmt.Sprintf("%x", swarm.bzz.BaseAddr()))
			return swarm, cleanup, nil
		},
	})
	defer sim.Close()

	ctx := context.Background()
	if o.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
		defer cancel()
	}

	files := make([]file, 0)

	for i, step := range steps {
		log.Debug("test sync step", "n", i+1, "nodes", step.nodeCount)

		change := step.nodeCount - len(sim.UpNodeIDs())

		if change > 0 {
			_, err := sim.AddNodesAndConnectChain(change)
			if err != nil {
				t.Fatal(err)
			}
		} else if change < 0 {
			_, err := sim.StopRandomNodes(-change)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			t.Logf("step %v: no change in nodes", i)
			continue
		}

		var checkStatusM sync.Map
		var nodeStatusM sync.Map
		var totalFoundCount uint64

		result := sim.Run(ctx, func(ctx context.Context, sim *simulation.Simulation) error {
			nodeIDs := sim.UpNodeIDs()
			rand.Shuffle(len(nodeIDs), func(i, j int) {
				nodeIDs[i], nodeIDs[j] = nodeIDs[j], nodeIDs[i]
			})
			for _, id := range nodeIDs {
				key, data, err := uploadFile(sim.Service("swarm", id).(*Swarm))
				if err != nil {
					return err
				}
				log.Trace("file uploaded", "node", id, "key", key.String())
				files = append(files, file{
					addr:   key,
					data:   data,
					nodeID: id,
				})
			}

			if *waitKademlia {
				if _, err := sim.WaitTillHealthy(ctx); err != nil {
					return err
				}
			}

//重复文件检索检查，直到从所有节点检索所有上载的文件
//或者直到超时。
			for {
				if retrieve(sim, files, &checkStatusM, &nodeStatusM, &totalFoundCount) == 0 {
					return nil
				}
			}
		})

		if result.Error != nil {
			t.Fatal(result.Error)
		}
		log.Debug("done: test sync step", "n", i+1, "nodes", step.nodeCount)
	}
}

//上载文件，将短文件上载到Swarm实例
//使用api.put方法。
func uploadFile(swarm *Swarm) (storage.Address, string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return nil, "", err
	}
//文件数据非常短，但可以确保
//独特性是非常确定的。
	data := fmt.Sprintf("test content %s %x", time.Now().Round(0), b)
	ctx := context.TODO()
	k, wait, err := swarm.api.Put(ctx, data, "text/plain", false)
	if err != nil {
		return nil, "", err
	}
	if wait != nil {
		err = wait(ctx)
	}
	return k, data, err
}

//retrieve是用于检查
//在testswarmnetwork test helper函数中上载了文件。
func retrieve(
	sim *simulation.Simulation,
	files []file,
	checkStatusM *sync.Map,
	nodeStatusM *sync.Map,
	totalFoundCount *uint64,
) (missing uint64) {
	rand.Shuffle(len(files), func(i, j int) {
		files[i], files[j] = files[j], files[i]
	})

	var totalWg sync.WaitGroup
	errc := make(chan error)

	nodeIDs := sim.UpNodeIDs()

	totalCheckCount := len(nodeIDs) * len(files)

	for _, id := range nodeIDs {
		if _, ok := nodeStatusM.Load(id); ok {
			continue
		}
		start := time.Now()
		var checkCount uint64
		var foundCount uint64

		totalWg.Add(1)

		var wg sync.WaitGroup

		swarm := sim.Service("swarm", id).(*Swarm)
		for _, f := range files {

			checkKey := check{
				key:    f.addr.String(),
				nodeID: id,
			}
			if n, ok := checkStatusM.Load(checkKey); ok && n.(int) == 0 {
				continue
			}

			checkCount++
			wg.Add(1)
			go func(f file, id enode.ID) {
				defer wg.Done()

				log.Debug("api get: check file", "node", id.String(), "key", f.addr.String(), "total files found", atomic.LoadUint64(totalFoundCount))

				r, _, _, _, err := swarm.api.Get(context.TODO(), api.NOOPDecrypt, f.addr, "/")
				if err != nil {
					errc <- fmt.Errorf("api get: node %s, key %s, kademlia %s: %v", id, f.addr, swarm.bzz.Hive, err)
					return
				}
				d, err := ioutil.ReadAll(r)
				if err != nil {
					errc <- fmt.Errorf("api get: read response: node %s, key %s: kademlia %s: %v", id, f.addr, swarm.bzz.Hive, err)
					return
				}
				data := string(d)
				if data != f.data {
					errc <- fmt.Errorf("file contend missmatch: node %s, key %s, expected %q, got %q", id, f.addr, f.data, data)
					return
				}
				checkStatusM.Store(checkKey, 0)
				atomic.AddUint64(&foundCount, 1)
				log.Info("api get: file found", "node", id.String(), "key", f.addr.String(), "content", data, "files found", atomic.LoadUint64(&foundCount))
			}(f, id)
		}

		go func(id enode.ID) {
			defer totalWg.Done()
			wg.Wait()

			atomic.AddUint64(totalFoundCount, foundCount)

			if foundCount == checkCount {
				log.Info("all files are found for node", "id", id.String(), "duration", time.Since(start))
				nodeStatusM.Store(id, 0)
				return
			}
			log.Debug("files missing for node", "id", id.String(), "check", checkCount, "found", foundCount)
		}(id)

	}

	go func() {
		totalWg.Wait()
		close(errc)
	}()

	var errCount int
	for err := range errc {
		if err != nil {
			errCount++
		}
		log.Warn(err.Error())
	}

	log.Info("check stats", "total check count", totalCheckCount, "total files found", atomic.LoadUint64(totalFoundCount), "total errors", errCount)

	return uint64(totalCheckCount) - atomic.LoadUint64(totalFoundCount)
}
