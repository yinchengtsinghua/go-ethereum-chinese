
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

package simulation

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
)

//nodeids返回网络中所有节点的nodeid。
func (s *Simulation) NodeIDs() (ids []enode.ID) {
	nodes := s.Net.GetNodes()
	ids = make([]enode.ID, len(nodes))
	for i, node := range nodes {
		ids[i] = node.ID()
	}
	return ids
}

//up nodeids返回网络上的节点的nodeid。
func (s *Simulation) UpNodeIDs() (ids []enode.ID) {
	nodes := s.Net.GetNodes()
	for _, node := range nodes {
		if node.Up {
			ids = append(ids, node.ID())
		}
	}
	return ids
}

//downnodeids返回网络中停止的节点的nodeid。
func (s *Simulation) DownNodeIDs() (ids []enode.ID) {
	nodes := s.Net.GetNodes()
	for _, node := range nodes {
		if !node.Up {
			ids = append(ids, node.ID())
		}
	}
	return ids
}

//AddNodeOption定义可以传递的选项
//到simulation.addnode方法。
type AddNodeOption func(*adapters.NodeConfig)

//addnodewithmsgevents设置启用msgevents选项
//给NodeConfig。
func AddNodeWithMsgEvents(enable bool) AddNodeOption {
	return func(o *adapters.NodeConfig) {
		o.EnableMsgEvents = enable
	}
}

//addNodeWithService指定一个应为
//在节点上启动。此选项可以作为变量重复
//参数toe add node和其他与添加节点相关的方法。
//如果未指定addNodeWithService，则将启动所有服务。
func AddNodeWithService(serviceName string) AddNodeOption {
	return func(o *adapters.NodeConfig) {
		o.Services = append(o.Services, serviceName)
	}
}

//addnode创建一个随机配置的新节点，
//将提供的选项应用于配置并将节点添加到网络。
//默认情况下，所有服务都将在一个节点上启动。如果一个或多个
//提供了addnodeWithService选项，将仅启动指定的服务。
func (s *Simulation) AddNode(opts ...AddNodeOption) (id enode.ID, err error) {
	conf := adapters.RandomNodeConfig()
	for _, o := range opts {
		o(conf)
	}
	if len(conf.Services) == 0 {
		conf.Services = s.serviceNames
	}
	node, err := s.Net.NewNodeWithConfig(conf)
	if err != nil {
		return id, err
	}
	return node.ID(), s.Net.Start(node.ID())
}

//addnodes创建具有随机配置的新节点，
//将提供的选项应用于配置并将节点添加到网络。
func (s *Simulation) AddNodes(count int, opts ...AddNodeOption) (ids []enode.ID, err error) {
	ids = make([]enode.ID, 0, count)
	for i := 0; i < count; i++ {
		id, err := s.AddNode(opts...)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

//addnodesandconnectfull是一个结合了
//addnodes和connectnodesfull。将只连接新节点。
func (s *Simulation) AddNodesAndConnectFull(count int, opts ...AddNodeOption) (ids []enode.ID, err error) {
	if count < 2 {
		return nil, errors.New("count of nodes must be at least 2")
	}
	ids, err = s.AddNodes(count, opts...)
	if err != nil {
		return nil, err
	}
	err = s.Net.ConnectNodesFull(ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

//AddNodesAndConnectChain是一个结合了
//添加节点和连接节点链。链条将从最后一个继续
//添加节点，如果在使用connectToLastNode方法的模拟中有节点。
func (s *Simulation) AddNodesAndConnectChain(count int, opts ...AddNodeOption) (ids []enode.ID, err error) {
	if count < 2 {
		return nil, errors.New("count of nodes must be at least 2")
	}
	id, err := s.AddNode(opts...)
	if err != nil {
		return nil, err
	}
	err = s.Net.ConnectToLastNode(id)
	if err != nil {
		return nil, err
	}
	ids, err = s.AddNodes(count-1, opts...)
	if err != nil {
		return nil, err
	}
	ids = append([]enode.ID{id}, ids...)
	err = s.Net.ConnectNodesChain(ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

//AddNodesAndConnectRing是一个结合了
//添加节点和连接节点。
func (s *Simulation) AddNodesAndConnectRing(count int, opts ...AddNodeOption) (ids []enode.ID, err error) {
	if count < 2 {
		return nil, errors.New("count of nodes must be at least 2")
	}
	ids, err = s.AddNodes(count, opts...)
	if err != nil {
		return nil, err
	}
	err = s.Net.ConnectNodesRing(ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

//addnodesandconnectstar是一个结合了
//添加节点和ConnectNodesStar。
func (s *Simulation) AddNodesAndConnectStar(count int, opts ...AddNodeOption) (ids []enode.ID, err error) {
	if count < 2 {
		return nil, errors.New("count of nodes must be at least 2")
	}
	ids, err = s.AddNodes(count, opts...)
	if err != nil {
		return nil, err
	}
	err = s.Net.ConnectNodesStar(ids[1:], ids[0])
	if err != nil {
		return nil, err
	}
	return ids, nil
}

//uploadSnapshot将快照上载到模拟
//此方法尝试打开提供的JSON文件，将配置应用于所有节点。
//然后将快照加载到仿真网络中
func (s *Simulation) UploadSnapshot(snapshotFile string, opts ...AddNodeOption) error {
	f, err := os.Open(snapshotFile)
	if err != nil {
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Error("Error closing snapshot file", "err", err)
		}
	}()
	jsonbyte, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	var snap simulations.Snapshot
	err = json.Unmarshal(jsonbyte, &snap)
	if err != nil {
		return err
	}

//快照可能未设置EnableMsgeEvents属性
//以防万一，把它设为真！
//（我们需要在上传前等待消息）
	for _, n := range snap.Nodes {
		n.Node.Config.EnableMsgEvents = true
		n.Node.Config.Services = s.serviceNames
		for _, o := range opts {
			o(n.Node.Config)
		}
	}

	log.Info("Waiting for p2p connections to be established...")

//现在我们可以加载快照了
	err = s.Net.Load(&snap)
	if err != nil {
		return err
	}
	log.Info("Snapshot loaded")
	return nil
}

//startnode按nodeid启动节点。
func (s *Simulation) StartNode(id enode.ID) (err error) {
	return s.Net.Start(id)
}

//startrandomnode启动随机节点。
func (s *Simulation) StartRandomNode() (id enode.ID, err error) {
	n := s.Net.GetRandomDownNode()
	if n == nil {
		return id, ErrNodeNotFound
	}
	return n.ID(), s.Net.Start(n.ID())
}

//startrandomnodes启动随机节点。
func (s *Simulation) StartRandomNodes(count int) (ids []enode.ID, err error) {
	ids = make([]enode.ID, 0, count)
	for i := 0; i < count; i++ {
		n := s.Net.GetRandomDownNode()
		if n == nil {
			return nil, ErrNodeNotFound
		}
		err = s.Net.Start(n.ID())
		if err != nil {
			return nil, err
		}
		ids = append(ids, n.ID())
	}
	return ids, nil
}

//stopnode按nodeid停止节点。
func (s *Simulation) StopNode(id enode.ID) (err error) {
	return s.Net.Stop(id)
}

//StopRandomNode停止随机节点。
func (s *Simulation) StopRandomNode() (id enode.ID, err error) {
	n := s.Net.GetRandomUpNode()
	if n == nil {
		return id, ErrNodeNotFound
	}
	return n.ID(), s.Net.Stop(n.ID())
}

//StopRandomNodes停止随机节点。
func (s *Simulation) StopRandomNodes(count int) (ids []enode.ID, err error) {
	ids = make([]enode.ID, 0, count)
	for i := 0; i < count; i++ {
		n := s.Net.GetRandomUpNode()
		if n == nil {
			return nil, ErrNodeNotFound
		}
		err = s.Net.Stop(n.ID())
		if err != nil {
			return nil, err
		}
		ids = append(ids, n.ID())
	}
	return ids, nil
}

//为simulation.randomnode种子随机生成器。
func init() {
	rand.Seed(time.Now().UnixNano())
}
