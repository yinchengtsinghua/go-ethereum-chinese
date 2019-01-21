
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
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/swarm/log"
)

var (
	nodeCount = 16
)

//此测试用于测试叠加模拟。
//由于模拟是通过主系统执行的，因此很容易错过更改。
//自动测试将阻止
//测试只是连接到模拟，启动网络，
//启动mocker，获取节点数，然后再次停止它。
//它还提供了前端所需步骤的文档
//使用模拟
func TestOverlaySim(t *testing.T) {
t.Skip("Test is flaky, see: https://github.com/ethersphere/go-ethereum/issues/592“）
//启动模拟
	log.Info("Start simulation backend")
//获取模拟网络；需要订阅up事件
	net := newSimulationNetwork()
//创建覆盖模拟
	sim := newOverlaySim(net)
//用它创建一个HTTP测试服务器
	srv := httptest.NewServer(sim)
	defer srv.Close()

	log.Debug("Http simulation server started. Start simulation network")
//启动仿真网络（仿真初始化）
	resp, err := http.Post(srv.URL+"/start", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected Status Code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	log.Debug("Start mocker")
//启动mocker，需要一个节点计数和一个ID
	resp, err = http.PostForm(srv.URL+"/mocker/start",
		url.Values{
			"node-count":  {fmt.Sprintf("%d", nodeCount)},
			"mocker-type": {simulations.GetMockerList()[0]},
		})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		reason, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Expected Status Code %d, got %d, response body %s", http.StatusOK, resp.StatusCode, string(reason))
	}

//等待节点启动所需的变量
	var upCount int
	trigger := make(chan enode.ID)

//等待所有节点启动
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

//开始监视节点启动事件…
	go watchSimEvents(net, ctx, trigger)

//…并等待直到收到所有预期的向上事件（nodeCount）
LOOP:
	for {
		select {
		case <-trigger:
//接收到新节点向上事件，增加计数器
			upCount++
//接收到所有预期的节点向上事件
			if upCount == nodeCount {
				break LOOP
			}
		case <-ctx.Done():
			t.Fatalf("Timed out waiting for up events")
		}

	}

//此时，我们可以查询服务器
	log.Info("Get number of nodes")
//获取节点数
	resp, err = http.Get(srv.URL + "/nodes")
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

//从JSON响应中取消标记节点数
	var nodesArr []simulations.Node
	err = json.Unmarshal(b, &nodesArr)
	if err != nil {
		t.Fatal(err)
	}

//检查接收的节点数是否与发送的节点数相同
	if len(nodesArr) != nodeCount {
		t.Fatal(fmt.Errorf("Expected %d number of nodes, got %d", nodeCount, len(nodesArr)))
	}

//需要让它运行一段时间，否则立即停止它会因运行节点而崩溃。
//希望连接到已停止的节点
	time.Sleep(1 * time.Second)

	log.Info("Stop the network")
//停止网络
	resp, err = http.Post(srv.URL+"/stop", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}

	log.Info("Reset the network")
//重置网络（删除所有节点和连接）
	resp, err = http.Post(srv.URL+"/reset", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("err %s", resp.Status)
	}
}

//注意事件，以便我们知道所有节点何时启动
func watchSimEvents(net *simulations.Network, ctx context.Context, trigger chan enode.ID) {
	events := make(chan *simulations.Event)
	sub := net.Events().Subscribe(events)
	defer sub.Unsubscribe()

	for {
		select {
		case ev := <-events:
//仅捕获节点向上事件
			if ev.Type == simulations.EventTypeNode {
				if ev.Node.Up {
					log.Debug("got node up event", "event", ev, "node", ev.Node.Config.ID)
					select {
					case trigger <- ev.Node.Config.ID:
					case <-ctx.Done():
						return
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
