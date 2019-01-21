
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Go Ethereum作者
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

package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rpc"
)

//privateAdminAPI是仅公开的管理API方法的集合
//通过安全的RPC通道。
type PrivateAdminAPI struct {
node *Node //此API接口的节点
}

//new private admin api为私有管理方法创建新的api定义
//节点本身的。
func NewPrivateAdminAPI(node *Node) *PrivateAdminAPI {
	return &PrivateAdminAPI{node: node}
}

//addpeer请求连接到远程节点，并维护新的
//随时连接，即使丢失也要重新连接。
func (api *PrivateAdminAPI) AddPeer(url string) (bool, error) {
//确保服务器正在运行，否则会失败。
	server := api.node.Server()
	if server == nil {
		return false, ErrNodeStopped
	}
//尝试将URL添加为静态对等，然后返回
	node, err := enode.ParseV4(url)
	if err != nil {
		return false, fmt.Errorf("invalid enode: %v", err)
	}
	server.AddPeer(node)
	return true, nil
}

//如果存在连接，则removepeer从远程节点断开连接
func (api *PrivateAdminAPI) RemovePeer(url string) (bool, error) {
//确保服务器正在运行，否则会失败。
	server := api.node.Server()
	if server == nil {
		return false, ErrNodeStopped
	}
//尝试删除静态对等的URL并返回
	node, err := enode.ParseV4(url)
	if err != nil {
		return false, fmt.Errorf("invalid enode: %v", err)
	}
	server.RemovePeer(node)
	return true, nil
}

//addtrustedpeer允许远程节点始终连接，即使插槽已满
func (api *PrivateAdminAPI) AddTrustedPeer(url string) (bool, error) {
//确保服务器正在运行，否则会失败。
	server := api.node.Server()
	if server == nil {
		return false, ErrNodeStopped
	}
	node, err := enode.ParseV4(url)
	if err != nil {
		return false, fmt.Errorf("invalid enode: %v", err)
	}
	server.AddTrustedPeer(node)
	return true, nil
}

//removeTrustedPeer从受信任的对等机集中删除远程节点，但它
//不会自动断开。
func (api *PrivateAdminAPI) RemoveTrustedPeer(url string) (bool, error) {
//确保服务器正在运行，否则会失败。
	server := api.node.Server()
	if server == nil {
		return false, ErrNodeStopped
	}
	node, err := enode.ParseV4(url)
	if err != nil {
		return false, fmt.Errorf("invalid enode: %v", err)
	}
	server.RemoveTrustedPeer(node)
	return true, nil
}

//PeerEvents创建一个RPC订阅，该订阅从
//节点的P2P服务器
func (api *PrivateAdminAPI) PeerEvents(ctx context.Context) (*rpc.Subscription, error) {
//确保服务器正在运行，否则会失败。
	server := api.node.Server()
	if server == nil {
		return nil, ErrNodeStopped
	}

//创建订阅
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return nil, rpc.ErrNotificationsUnsupported
	}
	rpcSub := notifier.CreateSubscription()

	go func() {
		events := make(chan *p2p.PeerEvent)
		sub := server.SubscribeEvents(events)
		defer sub.Unsubscribe()

		for {
			select {
			case event := <-events:
				notifier.Notify(rpcSub.ID, event)
			case <-sub.Err():
				return
			case <-rpcSub.Err():
				return
			case <-notifier.Closed():
				return
			}
		}
	}()

	return rpcSub, nil
}

//StartRPC启动HTTP RPC API服务器。
func (api *PrivateAdminAPI) StartRPC(host *string, port *int, cors *string, apis *string, vhosts *string) (bool, error) {
	api.node.lock.Lock()
	defer api.node.lock.Unlock()

	if api.node.httpHandler != nil {
		return false, fmt.Errorf("HTTP RPC already running on %s", api.node.httpEndpoint)
	}

	if host == nil {
		h := DefaultHTTPHost
		if api.node.config.HTTPHost != "" {
			h = api.node.config.HTTPHost
		}
		host = &h
	}
	if port == nil {
		port = &api.node.config.HTTPPort
	}

	allowedOrigins := api.node.config.HTTPCors
	if cors != nil {
		allowedOrigins = nil
		for _, origin := range strings.Split(*cors, ",") {
			allowedOrigins = append(allowedOrigins, strings.TrimSpace(origin))
		}
	}

	allowedVHosts := api.node.config.HTTPVirtualHosts
	if vhosts != nil {
		allowedVHosts = nil
		for _, vhost := range strings.Split(*host, ",") {
			allowedVHosts = append(allowedVHosts, strings.TrimSpace(vhost))
		}
	}

	modules := api.node.httpWhitelist
	if apis != nil {
		modules = nil
		for _, m := range strings.Split(*apis, ",") {
			modules = append(modules, strings.TrimSpace(m))
		}
	}

	if err := api.node.startHTTP(fmt.Sprintf("%s:%d", *host, *port), api.node.rpcAPIs, modules, allowedOrigins, allowedVHosts, api.node.config.HTTPTimeouts); err != nil {
		return false, err
	}
	return true, nil
}

//StopRPC终止已在运行的HTTP RPC API终结点。
func (api *PrivateAdminAPI) StopRPC() (bool, error) {
	api.node.lock.Lock()
	defer api.node.lock.Unlock()

	if api.node.httpHandler == nil {
		return false, fmt.Errorf("HTTP RPC not running")
	}
	api.node.stopHTTP()
	return true, nil
}

//startws启动WebSocket RPC API服务器。
func (api *PrivateAdminAPI) StartWS(host *string, port *int, allowedOrigins *string, apis *string) (bool, error) {
	api.node.lock.Lock()
	defer api.node.lock.Unlock()

	if api.node.wsHandler != nil {
		return false, fmt.Errorf("WebSocket RPC already running on %s", api.node.wsEndpoint)
	}

	if host == nil {
		h := DefaultWSHost
		if api.node.config.WSHost != "" {
			h = api.node.config.WSHost
		}
		host = &h
	}
	if port == nil {
		port = &api.node.config.WSPort
	}

	origins := api.node.config.WSOrigins
	if allowedOrigins != nil {
		origins = nil
		for _, origin := range strings.Split(*allowedOrigins, ",") {
			origins = append(origins, strings.TrimSpace(origin))
		}
	}

	modules := api.node.config.WSModules
	if apis != nil {
		modules = nil
		for _, m := range strings.Split(*apis, ",") {
			modules = append(modules, strings.TrimSpace(m))
		}
	}

	if err := api.node.startWS(fmt.Sprintf("%s:%d", *host, *port), api.node.rpcAPIs, modules, origins, api.node.config.WSExposeAll); err != nil {
		return false, err
	}
	return true, nil
}

//stopws终止已在运行的WebSocket RPC API终结点。
func (api *PrivateAdminAPI) StopWS() (bool, error) {
	api.node.lock.Lock()
	defer api.node.lock.Unlock()

	if api.node.wsHandler == nil {
		return false, fmt.Errorf("WebSocket RPC not running")
	}
	api.node.stopWS()
	return true, nil
}

//publicAdminAPI是在
//安全和不安全的RPC通道。
type PublicAdminAPI struct {
node *Node //此API接口的节点
}

//NewPublicAdminAPI为公共管理方法创建新的API定义
//节点本身的。
func NewPublicAdminAPI(node *Node) *PublicAdminAPI {
	return &PublicAdminAPI{node: node}
}

//对等端检索我们知道的有关每个对等端的所有信息
//协议粒度。
func (api *PublicAdminAPI) Peers() ([]*p2p.PeerInfo, error) {
	server := api.node.Server()
	if server == nil {
		return nil, ErrNodeStopped
	}
	return server.PeersInfo(), nil
}

//nodeinfo检索我们知道的有关主机节点的所有信息
//协议粒度。
func (api *PublicAdminAPI) NodeInfo() (*p2p.NodeInfo, error) {
	server := api.node.Server()
	if server == nil {
		return nil, ErrNodeStopped
	}
	return server.NodeInfo(), nil
}

//datadir检索节点使用的当前数据目录。
func (api *PublicAdminAPI) Datadir() string {
	return api.node.DataDir()
}

//publicDebugAPI是在
//安全和不安全的RPC通道。
type PublicDebugAPI struct {
node *Node //此API接口的节点
}

//NewPublicDebugGapi为公共调试方法创建新的API定义
//节点本身的。
func NewPublicDebugAPI(node *Node) *PublicDebugAPI {
	return &PublicDebugAPI{node: node}
}

//度量检索节点收集的所有已知系统度量。
func (api *PublicDebugAPI) Metrics(raw bool) (map[string]interface{}, error) {
//创建速率格式化程序
	units := []string{"", "K", "M", "G", "T", "E", "P"}
	round := func(value float64, prec int) string {
		unit := 0
		for value >= 1000 {
			unit, value, prec = unit+1, value/1000, 2
		}
		return fmt.Sprintf(fmt.Sprintf("%%.%df%s", prec, units[unit]), value)
	}
	format := func(total float64, rate float64) string {
		return fmt.Sprintf("%s (%s/s)", round(total, 0), round(rate, 2))
	}
//迭代所有度量，现在只转储
	counters := make(map[string]interface{})
	metrics.DefaultRegistry.Each(func(name string, metric interface{}) {
//创建或检索此度量的计数器层次结构
		root, parts := counters, strings.Split(name, "/")
		for _, part := range parts[:len(parts)-1] {
			if _, ok := root[part]; !ok {
				root[part] = make(map[string]interface{})
			}
			root = root[part].(map[string]interface{})
		}
		name = parts[len(parts)-1]

//在计数器中填入度量详细信息，如果需要，请格式化。
		if raw {
			switch metric := metric.(type) {
			case metrics.Counter:
				root[name] = map[string]interface{}{
					"Overall": float64(metric.Count()),
				}

			case metrics.Meter:
				root[name] = map[string]interface{}{
					"AvgRate01Min": metric.Rate1(),
					"AvgRate05Min": metric.Rate5(),
					"AvgRate15Min": metric.Rate15(),
					"MeanRate":     metric.RateMean(),
					"Overall":      float64(metric.Count()),
				}

			case metrics.Timer:
				root[name] = map[string]interface{}{
					"AvgRate01Min": metric.Rate1(),
					"AvgRate05Min": metric.Rate5(),
					"AvgRate15Min": metric.Rate15(),
					"MeanRate":     metric.RateMean(),
					"Overall":      float64(metric.Count()),
					"Percentiles": map[string]interface{}{
						"5":  metric.Percentile(0.05),
						"20": metric.Percentile(0.2),
						"50": metric.Percentile(0.5),
						"80": metric.Percentile(0.8),
						"95": metric.Percentile(0.95),
					},
				}

			case metrics.ResettingTimer:
				t := metric.Snapshot()
				ps := t.Percentiles([]float64{5, 20, 50, 80, 95})
				root[name] = map[string]interface{}{
					"Measurements": len(t.Values()),
					"Mean":         t.Mean(),
					"Percentiles": map[string]interface{}{
						"5":  ps[0],
						"20": ps[1],
						"50": ps[2],
						"80": ps[3],
						"95": ps[4],
					},
				}

			default:
				root[name] = "Unknown metric type"
			}
		} else {
			switch metric := metric.(type) {
			case metrics.Counter:
				root[name] = map[string]interface{}{
					"Overall": float64(metric.Count()),
				}

			case metrics.Meter:
				root[name] = map[string]interface{}{
					"Avg01Min": format(metric.Rate1()*60, metric.Rate1()),
					"Avg05Min": format(metric.Rate5()*300, metric.Rate5()),
					"Avg15Min": format(metric.Rate15()*900, metric.Rate15()),
					"Overall":  format(float64(metric.Count()), metric.RateMean()),
				}

			case metrics.Timer:
				root[name] = map[string]interface{}{
					"Avg01Min": format(metric.Rate1()*60, metric.Rate1()),
					"Avg05Min": format(metric.Rate5()*300, metric.Rate5()),
					"Avg15Min": format(metric.Rate15()*900, metric.Rate15()),
					"Overall":  format(float64(metric.Count()), metric.RateMean()),
					"Maximum":  time.Duration(metric.Max()).String(),
					"Minimum":  time.Duration(metric.Min()).String(),
					"Percentiles": map[string]interface{}{
						"5":  time.Duration(metric.Percentile(0.05)).String(),
						"20": time.Duration(metric.Percentile(0.2)).String(),
						"50": time.Duration(metric.Percentile(0.5)).String(),
						"80": time.Duration(metric.Percentile(0.8)).String(),
						"95": time.Duration(metric.Percentile(0.95)).String(),
					},
				}

			case metrics.ResettingTimer:
				t := metric.Snapshot()
				ps := t.Percentiles([]float64{5, 20, 50, 80, 95})
				root[name] = map[string]interface{}{
					"Measurements": len(t.Values()),
					"Mean":         time.Duration(t.Mean()).String(),
					"Percentiles": map[string]interface{}{
						"5":  time.Duration(ps[0]).String(),
						"20": time.Duration(ps[1]).String(),
						"50": time.Duration(ps[2]).String(),
						"80": time.Duration(ps[3]).String(),
						"95": time.Duration(ps[4]).String(),
					},
				}

			default:
				root[name] = "Unknown metric type"
			}
		}
	})
	return counters, nil
}

//publicWeb3API提供helper实用程序
type PublicWeb3API struct {
	stack *Node
}

//NewPublicWeb3API创建新的Web3Service实例
func NewPublicWeb3API(stack *Node) *PublicWeb3API {
	return &PublicWeb3API{stack}
}

//clientversion返回节点名
func (s *PublicWeb3API) ClientVersion() string {
	return s.stack.Server().Name
}

//sha3对输入应用ethereum sha3实现。
//它假定输入是十六进制编码的。
func (s *PublicWeb3API) Sha3(input hexutil.Bytes) hexutil.Bytes {
	return crypto.Keccak256(input)
}
