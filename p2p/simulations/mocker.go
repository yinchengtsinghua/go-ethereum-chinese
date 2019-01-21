
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

//包模拟模拟P2P网络。
//模拟程序模拟网络中真实节点的启动和停止。
package simulations

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
)

//mocker名称到其函数的映射
var mockerList = map[string]func(net *Network, quit chan struct{}, nodeCount int){
	"startStop":     startStop,
	"probabilistic": probabilistic,
	"boot":          boot,
}

//按名称查找mocker，返回mokerfn
func LookupMocker(mockerType string) func(net *Network, quit chan struct{}, nodeCount int) {
	return mockerList[mockerType]
}

//获取模拟程序列表（地图的键）
//用于前端构建可用的mocker选择
func GetMockerList() []string {
	list := make([]string, 0, len(mockerList))
	for k := range mockerList {
		list = append(list, k)
	}
	return list
}

//引导mokerfn只连接环中的节点，不执行任何其他操作
func boot(net *Network, quit chan struct{}, nodeCount int) {
	_, err := connectNodesInRing(net, nodeCount)
	if err != nil {
		panic("Could not startup node network for mocker")
	}
}

//startstop mokerfn在定义的时间段内停止和启动节点（ticker）
func startStop(net *Network, quit chan struct{}, nodeCount int) {
	nodes, err := connectNodesInRing(net, nodeCount)
	if err != nil {
		panic("Could not startup node network for mocker")
	}
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-quit:
			log.Info("Terminating simulation loop")
			return
		case <-tick.C:
			id := nodes[rand.Intn(len(nodes))]
			log.Info("stopping node", "id", id)
			if err := net.Stop(id); err != nil {
				log.Error("error stopping node", "id", id, "err", err)
				return
			}

			select {
			case <-quit:
				log.Info("Terminating simulation loop")
				return
			case <-time.After(3 * time.Second):
			}

			log.Debug("starting node", "id", id)
			if err := net.Start(id); err != nil {
				log.Error("error starting node", "id", id, "err", err)
				return
			}
		}
	}
}

//概率嘲弄者func有一个更为概率的模式。
//（实施可能会得到改进）：
//节点以环形连接，然后选择不同数量的随机节点，
//然后mocker以随机间隔停止并启动它们，并继续循环
func probabilistic(net *Network, quit chan struct{}, nodeCount int) {
	nodes, err := connectNodesInRing(net, nodeCount)
	if err != nil {
		select {
		case <-quit:
//错误可能是由于模拟中止；因此退出通道关闭
			return
		default:
			panic("Could not startup node network for mocker")
		}
	}
	for {
		select {
		case <-quit:
			log.Info("Terminating simulation loop")
			return
		default:
		}
		var lowid, highid int
		var wg sync.WaitGroup
		randWait := time.Duration(rand.Intn(5000)+1000) * time.Millisecond
		rand1 := rand.Intn(nodeCount - 1)
		rand2 := rand.Intn(nodeCount - 1)
		if rand1 < rand2 {
			lowid = rand1
			highid = rand2
		} else if rand1 > rand2 {
			highid = rand1
			lowid = rand2
		} else {
			if rand1 == 0 {
				rand2 = 9
			} else if rand1 == 9 {
				rand1 = 0
			}
			lowid = rand1
			highid = rand2
		}
		var steps = highid - lowid
		wg.Add(steps)
		for i := lowid; i < highid; i++ {
			select {
			case <-quit:
				log.Info("Terminating simulation loop")
				return
			case <-time.After(randWait):
			}
			log.Debug(fmt.Sprintf("node %v shutting down", nodes[i]))
			err := net.Stop(nodes[i])
			if err != nil {
				log.Error("Error stopping node", "node", nodes[i])
				wg.Done()
				continue
			}
			go func(id enode.ID) {
				time.Sleep(randWait)
				err := net.Start(id)
				if err != nil {
					log.Error("Error starting node", "node", id)
				}
				wg.Done()
			}(nodes[i])
		}
		wg.Wait()
	}

}

//连接节点计数环中的节点数
func connectNodesInRing(net *Network, nodeCount int) ([]enode.ID, error) {
	ids := make([]enode.ID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		conf := adapters.RandomNodeConfig()
		node, err := net.NewNodeWithConfig(conf)
		if err != nil {
			log.Error("Error creating a node!", "err", err)
			return nil, err
		}
		ids[i] = node.ID()
	}

	for _, id := range ids {
		if err := net.Start(id); err != nil {
			log.Error("Error starting a node!", "err", err)
			return nil, err
		}
		log.Debug(fmt.Sprintf("node %v starting up", id))
	}
	for i, id := range ids {
		peerID := ids[(i+1)%len(ids)]
		if err := net.Connect(id, peerID); err != nil {
			log.Error("Error connecting a node to a peer!", "err", err)
			return nil, err
		}
	}

	return ids, nil
}
