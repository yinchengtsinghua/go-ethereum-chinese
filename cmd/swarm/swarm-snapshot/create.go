
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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/simulation"
	cli "gopkg.in/urfave/cli.v1"
)

//create用作“create”app命令的输入函数。
func create(ctx *cli.Context) error {
	log.PrintOrigins(true)
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(ctx.Int("verbosity")), log.StreamHandler(os.Stdout, log.TerminalFormat(true))))

	if len(ctx.Args()) < 1 {
		return errors.New("argument should be the filename to verify or write-to")
	}
	filename, err := touchPath(ctx.Args()[0])
	if err != nil {
		return err
	}
	return createSnapshot(filename, ctx.Int("nodes"), strings.Split(ctx.String("services"), ","))
}

//createSnapshot使用提供的文件名在文件系统上创建新的快照，
//节点数和服务名。
func createSnapshot(filename string, nodes int, services []string) (err error) {
	log.Debug("create snapshot", "filename", filename, "nodes", nodes, "services", services)

	sim := simulation.New(map[string]simulation.ServiceFunc{
		"bzz": func(ctx *adapters.ServiceContext, b *sync.Map) (node.Service, func(), error) {
			addr := network.NewAddr(ctx.Config.Node())
			kad := network.NewKademlia(addr.Over(), network.NewKadParams())
			hp := network.NewHiveParams()
			hp.KeepAliveInterval = time.Duration(200) * time.Millisecond
hp.Discovery = true //创建快照时必须启用发现

			config := &network.BzzConfig{
				OverlayAddr:  addr.Over(),
				UnderlayAddr: addr.Under(),
				HiveParams:   hp,
			}
			return network.NewBzz(config, kad, nil, nil, nil), nil, nil
		},
	})
	defer sim.Close()

	_, err = sim.AddNodes(nodes)
	if err != nil {
		return fmt.Errorf("add nodes: %v", err)
	}

	err = sim.Net.ConnectNodesRing(nil)
	if err != nil {
		return fmt.Errorf("connect nodes: %v", err)
	}

	ctx, cancelSimRun := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelSimRun()
	if _, err := sim.WaitTillHealthy(ctx); err != nil {
		return fmt.Errorf("wait for healthy kademlia: %v", err)
	}

	var snap *simulations.Snapshot
	if len(services) > 0 {
//如果提供了服务名称，请在快照中包含它们。
//但是，检查“bzz”服务是否不在其中以删除它
//在创建快照时形成快照。
		var removeServices []string
		var wantBzz bool
		for _, s := range services {
			if s == "bzz" {
				wantBzz = true
				break
			}
		}
		if !wantBzz {
			removeServices = []string{"bzz"}
		}
		snap, err = sim.Net.SnapshotWithServices(services, removeServices)
	} else {
		snap, err = sim.Net.Snapshot()
	}
	if err != nil {
		return fmt.Errorf("create snapshot: %v", err)
	}
	jsonsnapshot, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("json encode snapshot: %v", err)
	}
	return ioutil.WriteFile(filename, jsonsnapshot, 0666)
}

//
//不见了。
func touchPath(filename string) (string, error) {
	if path.IsAbs(filename) {
		if _, err := os.Stat(filename); err == nil {
//路径存在，覆盖
			return filename, nil
		}
	}

	d, f := path.Split(filename)
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", err
	}

	_, err = os.Stat(path.Join(dir, filename))
	if err == nil {
//路径存在，覆盖
		return filename, nil
	}

	dirPath := path.Join(dir, d)
	filePath := path.Join(dirPath, f)
	if d != "" {
		err = os.MkdirAll(dirPath, os.ModeDir)
		if err != nil {
			return "", err
		}
	}

	return filePath, nil
}
