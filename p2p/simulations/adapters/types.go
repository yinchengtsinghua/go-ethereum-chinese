
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

package adapters

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/docker/docker/pkg/reexec"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rpc"
)

//节点表示模拟网络中由
//节点适配器，例如：
//
//*simnode-内存中的节点
//*execnode-子进程节点
//*DockerNode-Docker容器节点
//
type Node interface {
//addr返回节点的地址（例如e node url）
	Addr() []byte

//客户端返回在节点
//启动和运行
	Client() (*rpc.Client, error)

//serverpc通过给定的连接提供RPC请求
	ServeRPC(net.Conn) error

//Start用给定的快照启动节点
	Start(snapshots map[string][]byte) error

//停止停止节点
	Stop() error

//nodeinfo返回有关节点的信息
	NodeInfo() *p2p.NodeInfo

//快照创建正在运行的服务的快照
	Snapshots() (map[string][]byte, error)
}

//节点适配器用于在仿真网络中创建节点。
type NodeAdapter interface {
//name返回用于日志记录的适配器的名称
	Name() string

//new node创建具有给定配置的新节点
	NewNode(config *NodeConfig) (Node, error)
}

//nodeconfig是用于在模拟中启动节点的配置
//网络
type NodeConfig struct {
//ID是节点的ID，用于标识
//仿真网络
	ID enode.ID

//private key是节点的私钥，由devp2p使用
//用于加密通信的堆栈
	PrivateKey *ecdsa.PrivateKey

//为MSG启用对等事件
	EnableMsgEvents bool

//名称是节点的友好名称，如“node01”
	Name string

//服务是在以下情况下应该运行的服务的名称：
//启动节点（对于Simnodes，它应该是服务的名称
//包含在simadapter.services中，对于其他节点，它应该
//通过调用registerservice函数注册的服务）
	Services []string

//制裁或阻止建议同伴的职能
	Reachable func(id enode.ID) bool

	Port uint16
}

//nodeconfig json用于通过编码将nodeconfig编码和解码为json。
//所有字段作为字符串
type nodeConfigJSON struct {
	ID              string   `json:"id"`
	PrivateKey      string   `json:"private_key"`
	Name            string   `json:"name"`
	Services        []string `json:"services"`
	EnableMsgEvents bool     `json:"enable_msg_events"`
	Port            uint16   `json:"port"`
}

//marshaljson通过编码配置来实现json.marshaler接口
//字段作为字符串
func (n *NodeConfig) MarshalJSON() ([]byte, error) {
	confJSON := nodeConfigJSON{
		ID:              n.ID.String(),
		Name:            n.Name,
		Services:        n.Services,
		Port:            n.Port,
		EnableMsgEvents: n.EnableMsgEvents,
	}
	if n.PrivateKey != nil {
		confJSON.PrivateKey = hex.EncodeToString(crypto.FromECDSA(n.PrivateKey))
	}
	return json.Marshal(confJSON)
}

//unmashaljson通过解码json来实现json.unmasheler接口
//在配置字段中字符串值
func (n *NodeConfig) UnmarshalJSON(data []byte) error {
	var confJSON nodeConfigJSON
	if err := json.Unmarshal(data, &confJSON); err != nil {
		return err
	}

	if confJSON.ID != "" {
		if err := n.ID.UnmarshalText([]byte(confJSON.ID)); err != nil {
			return err
		}
	}

	if confJSON.PrivateKey != "" {
		key, err := hex.DecodeString(confJSON.PrivateKey)
		if err != nil {
			return err
		}
		privKey, err := crypto.ToECDSA(key)
		if err != nil {
			return err
		}
		n.PrivateKey = privKey
	}

	n.Name = confJSON.Name
	n.Services = confJSON.Services
	n.Port = confJSON.Port
	n.EnableMsgEvents = confJSON.EnableMsgEvents

	return nil
}

//node返回由配置表示的节点描述符。
func (n *NodeConfig) Node() *enode.Node {
	return enode.NewV4(&n.PrivateKey.PublicKey, net.IP{127, 0, 0, 1}, int(n.Port), int(n.Port))
}

//randomnodeconfig返回具有随机生成的ID和
//私人钥匙
func RandomNodeConfig() *NodeConfig {
	key, err := crypto.GenerateKey()
	if err != nil {
		panic("unable to generate key")
	}

	id := enode.PubkeyToIDV4(&key.PublicKey)
	port, err := assignTCPPort()
	if err != nil {
		panic("unable to assign tcp port")
	}
	return &NodeConfig{
		ID:              id,
		Name:            fmt.Sprintf("node_%s", id.String()),
		PrivateKey:      key,
		Port:            port,
		EnableMsgEvents: true,
	}
}

func assignTCPPort() (uint16, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, err
	}
	p, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint16(p), nil
}

//ServiceContext是可以使用的选项和方法的集合
//启动服务时
type ServiceContext struct {
	RPCDialer

	NodeContext *node.ServiceContext
	Config      *NodeConfig
	Snapshot    []byte
}

//rpcdialer用于初始化需要连接到的服务
//网络中的其他节点（例如需要
//连接到geth节点以解析ens名称）
type RPCDialer interface {
	DialRPC(id enode.ID) (*rpc.Client, error)
}

//服务是可以在模拟中运行的服务集合
type Services map[string]ServiceFunc

//servicefunc返回可用于引导devp2p节点的node.service
type ServiceFunc func(ctx *ServiceContext) (node.Service, error)

//servicefuncs是用于引导devp2p的已注册服务的映射
//结点
var serviceFuncs = make(Services)

//RegisterServices注册给定的服务，然后可以使用这些服务
//使用exec或docker适配器启动devp2p节点。
//
//应该在in it函数中调用它，这样它就有机会
//在调用main（）之前执行服务。
func RegisterServices(services Services) {
	for name, f := range services {
		if _, exists := serviceFuncs[name]; exists {
			panic(fmt.Sprintf("node service already exists: %q", name))
		}
		serviceFuncs[name] = f
	}

//现在我们已经注册了服务，运行reexec.init（），它将
//如果当前二进制文件
//已执行，argv[0]设置为“p2p节点”
	if reexec.Init() {
		os.Exit(0)
	}
}
