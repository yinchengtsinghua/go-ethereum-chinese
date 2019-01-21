
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
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"
)

func TestMocker(t *testing.T) {
//启动模拟HTTP服务器
	_, s := testHTTPServer(t)
	defer s.Close()

//创建客户端
	client := NewClient(s.URL)

//启动网络
	err := client.StartNetwork()
	if err != nil {
		t.Fatalf("Could not start test network: %s", err)
	}
//停止网络以终止
	defer func() {
		err = client.StopNetwork()
		if err != nil {
			t.Fatalf("Could not stop test network: %s", err)
		}
	}()

//获取可用的mocker类型列表
	resp, err := http.Get(s.URL + "/mocker")
	if err != nil {
		t.Fatalf("Could not get mocker list: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("Invalid Status Code received, expected 200, got %d", resp.StatusCode)
	}

//检查列表的大小是否至少为1
	var mockerlist []string
	err = json.NewDecoder(resp.Body).Decode(&mockerlist)
	if err != nil {
		t.Fatalf("Error decoding JSON mockerlist: %s", err)
	}

	if len(mockerlist) < 1 {
		t.Fatalf("No mockers available")
	}

	nodeCount := 10
	var wg sync.WaitGroup

	events := make(chan *Event, 10)
	var opts SubscribeOpts
	sub, err := client.SubscribeNetwork(events, opts)
	defer sub.Unsubscribe()
//等待所有节点启动并连接
//将每个节点向上事件存储在映射中（值不相关，模拟集数据类型）
	nodemap := make(map[enode.ID]bool)
	wg.Add(1)
	nodesComplete := false
	connCount := 0
	go func() {
		for {
			select {
			case event := <-events:
//如果事件仅为节点向上事件
				if event.Node != nil && event.Node.Up {
//将相应的节点ID添加到映射中
					nodemap[event.Node.Config.ID] = true
//这意味着所有节点都有一个nodeup事件，因此我们可以继续测试
					if len(nodemap) == nodeCount {
						nodesComplete = true
//等待3秒，因为模拟机需要时间连接节点
//时间。睡眠（3*时间。秒）
					}
				} else if event.Conn != nil && nodesComplete {
					connCount += 1
					if connCount == (nodeCount-1)*2 {
						wg.Done()
						return
					}
				}
			case <-time.After(30 * time.Second):
				wg.Done()
				t.Fatalf("Timeout waiting for nodes being started up!")
			}
		}
	}()

//将mokerlist的最后一个元素作为默认mocker类型，以确保启用了mocker类型。
	mockertype := mockerlist[len(mockerlist)-1]
//不过，如果有的话，使用硬编码的“概率”一个；（）
	for _, m := range mockerlist {
		if m == "probabilistic" {
			mockertype = m
			break
		}
	}
//用节点数启动mocker
	resp, err = http.PostForm(s.URL+"/mocker/start", url.Values{"mocker-type": {mockertype}, "node-count": {strconv.Itoa(nodeCount)}})
	if err != nil {
		t.Fatalf("Could not start mocker: %s", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Invalid Status Code received for starting mocker, expected 200, got %d", resp.StatusCode)
	}

	wg.Wait()

//检查网络中是否有节点计数
	nodesInfo, err := client.GetNodes()
	if err != nil {
		t.Fatalf("Could not get nodes list: %s", err)
	}

	if len(nodesInfo) != nodeCount {
		t.Fatalf("Expected %d number of nodes, got: %d", nodeCount, len(nodesInfo))
	}

//停止嘲笑者
	resp, err = http.Post(s.URL+"/mocker/stop", "", nil)
	if err != nil {
		t.Fatalf("Could not stop mocker: %s", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Invalid Status Code received for stopping mocker, expected 200, got %d", resp.StatusCode)
	}

//重置网络
	_, err = http.Post(s.URL+"/reset", "", nil)
	if err != nil {
		t.Fatalf("Could not reset network: %s", err)
	}

//现在网络中的节点数应该为零
	nodesInfo, err = client.GetNodes()
	if err != nil {
		t.Fatalf("Could not get nodes list: %s", err)
	}

	if len(nodesInfo) != 0 {
		t.Fatalf("Expected empty list of nodes, got: %d", len(nodesInfo))
	}
}
