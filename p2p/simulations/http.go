
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

package simulations

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/websocket"
)

//DefaultClient是默认的模拟API客户端，它需要API
//在http://localhost:8888上运行
var DefaultClient = NewClient("http://本地主机：8888“）

//客户端是支持创建的模拟HTTP API的客户端
//以及管理模拟网络
type Client struct {
	URL string

	client *http.Client
}

//new client返回新的模拟API客户端
func NewClient(url string) *Client {
	return &Client{
		URL:    url,
		client: http.DefaultClient,
	}
}

//GetNetwork返回网络的详细信息
func (c *Client) GetNetwork() (*Network, error) {
	network := &Network{}
	return network, c.Get("/", network)
}

//StartNetwork启动模拟网络中的所有现有节点
func (c *Client) StartNetwork() error {
	return c.Post("/start", nil, nil)
}

//stopNetwork停止模拟网络中的所有现有节点
func (c *Client) StopNetwork() error {
	return c.Post("/stop", nil, nil)
}

//创建快照创建网络快照
func (c *Client) CreateSnapshot() (*Snapshot, error) {
	snap := &Snapshot{}
	return snap, c.Get("/snapshot", snap)
}

//LoadSnapshot将快照加载到网络中
func (c *Client) LoadSnapshot(snap *Snapshot) error {
	return c.Post("/snapshot", snap, nil)
}

//subscribeopts是订阅网络时要使用的选项集合
//事件
type SubscribeOpts struct {
//current指示服务器发送现有节点的事件和
//先连接
	Current bool

//筛选器指示服务器仅发送消息事件的子集
	Filter string
}

//订阅网络订阅从服务器发送的网络事件
//作为服务器发送的事件流，可以选择接收现有事件
//节点和连接以及筛选消息事件
func (c *Client) SubscribeNetwork(events chan *Event, opts SubscribeOpts) (event.Subscription, error) {
	url := fmt.Sprintf("%s/events?current=%t&filter=%s", c.URL, opts.Current, opts.Filter)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		response, _ := ioutil.ReadAll(res.Body)
		res.Body.Close()
		return nil, fmt.Errorf("unexpected HTTP status: %s: %s", res.Status, response)
	}

//定义要传递到事件的生产者函数。订阅
//从Res.Body读取服务器发送的事件并发送
//他们到活动频道
	producer := func(stop <-chan struct{}) error {
		defer res.Body.Close()

//在Goroutine中阅读Res.Body的台词，这样我们
//总是从停止通道读取
		lines := make(chan string)
		errC := make(chan error, 1)
		go func() {
			s := bufio.NewScanner(res.Body)
			for s.Scan() {
				select {
				case lines <- s.Text():
				case <-stop:
					return
				}
			}
			errC <- s.Err()
		}()

//检测以“数据：”开头的任何行，解码数据
//将其发送到事件频道
		for {
			select {
			case line := <-lines:
				if !strings.HasPrefix(line, "data:") {
					continue
				}
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				event := &Event{}
				if err := json.Unmarshal([]byte(data), event); err != nil {
					return fmt.Errorf("error decoding SSE event: %s", err)
				}
				select {
				case events <- event:
				case <-stop:
					return nil
				}
			case err := <-errC:
				return err
			case <-stop:
				return nil
			}
		}
	}

	return event.NewSubscription(producer), nil
}

//getnodes返回网络中存在的所有节点
func (c *Client) GetNodes() ([]*p2p.NodeInfo, error) {
	var nodes []*p2p.NodeInfo
	return nodes, c.Get("/nodes", &nodes)
}

//createNode使用给定的配置在网络中创建节点
func (c *Client) CreateNode(config *adapters.NodeConfig) (*p2p.NodeInfo, error) {
	node := &p2p.NodeInfo{}
	return node, c.Post("/nodes", config, node)
}

//getnode返回节点的详细信息
func (c *Client) GetNode(nodeID string) (*p2p.NodeInfo, error) {
	node := &p2p.NodeInfo{}
	return node, c.Get(fmt.Sprintf("/nodes/%s", nodeID), node)
}

//startnode启动节点
func (c *Client) StartNode(nodeID string) error {
	return c.Post(fmt.Sprintf("/nodes/%s/start", nodeID), nil, nil)
}

//停止节点停止节点
func (c *Client) StopNode(nodeID string) error {
	return c.Post(fmt.Sprintf("/nodes/%s/stop", nodeID), nil, nil)
}

//ConnectNode将节点连接到对等节点
func (c *Client) ConnectNode(nodeID, peerID string) error {
	return c.Post(fmt.Sprintf("/nodes/%s/conn/%s", nodeID, peerID), nil, nil)
}

//断开节点断开节点与对等节点的连接
func (c *Client) DisconnectNode(nodeID, peerID string) error {
	return c.Delete(fmt.Sprintf("/nodes/%s/conn/%s", nodeID, peerID))
}

//rpc client返回连接到节点的rpc客户端
func (c *Client) RPCClient(ctx context.Context, nodeID string) (*rpc.Client, error) {
	baseURL := strings.Replace(c.URL, "http", "ws", 1)
	return rpc.DialWebsocket(ctx, fmt.Sprintf("%s/nodes/%s/rpc", baseURL, nodeID), "")
}

//GET执行HTTP GET请求，对结果JSON响应进行解码。
//入“出”
func (c *Client) Get(path string, out interface{}) error {
	return c.Send("GET", path, nil, out)
}

//post执行一个HTTP post请求，将“in”作为JSON主体发送，并且
//将得到的JSON响应解码为“out”
func (c *Client) Post(path string, in, out interface{}) error {
	return c.Send("POST", path, in, out)
}

//删除执行HTTP删除请求
func (c *Client) Delete(path string) error {
	return c.Send("DELETE", path, nil, nil)
}

//send执行一个HTTP请求，将“in”作为JSON请求体发送，并且
//将JSON响应解码为“out”
func (c *Client) Send(method, path string, in, out interface{}) error {
	var body []byte
	if in != nil {
		var err error
		body, err = json.Marshal(in)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, c.URL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		response, _ := ioutil.ReadAll(res.Body)
		return fmt.Errorf("unexpected HTTP status: %s: %s", res.Status, response)
	}
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			return err
		}
	}
	return nil
}

//服务器是一个HTTP服务器，提供用于管理模拟网络的API
type Server struct {
	router     *httprouter.Router
	network    *Network
mockerStop chan struct{} //设置后，停止当前mocker
mockerMtx  sync.Mutex    //同步访问Mockerstop字段
}

//NewServer返回新的模拟API服务器
func NewServer(network *Network) *Server {
	s := &Server{
		router:  httprouter.New(),
		network: network,
	}

	s.OPTIONS("/", s.Options)
	s.GET("/", s.GetNetwork)
	s.POST("/start", s.StartNetwork)
	s.POST("/stop", s.StopNetwork)
	s.POST("/mocker/start", s.StartMocker)
	s.POST("/mocker/stop", s.StopMocker)
	s.GET("/mocker", s.GetMockers)
	s.POST("/reset", s.ResetNetwork)
	s.GET("/events", s.StreamNetworkEvents)
	s.GET("/snapshot", s.CreateSnapshot)
	s.POST("/snapshot", s.LoadSnapshot)
	s.POST("/nodes", s.CreateNode)
	s.GET("/nodes", s.GetNodes)
	s.GET("/nodes/:nodeid", s.GetNode)
	s.POST("/nodes/:nodeid/start", s.StartNode)
	s.POST("/nodes/:nodeid/stop", s.StopNode)
	s.POST("/nodes/:nodeid/conn/:peerid", s.ConnectNode)
	s.DELETE("/nodes/:nodeid/conn/:peerid", s.DisconnectNode)
	s.GET("/nodes/:nodeid/rpc", s.NodeRPC)

	return s
}

//GetNetwork返回网络的详细信息
func (s *Server) GetNetwork(w http.ResponseWriter, req *http.Request) {
	s.JSON(w, http.StatusOK, s.network)
}

//StartNetwork启动网络中的所有节点
func (s *Server) StartNetwork(w http.ResponseWriter, req *http.Request) {
	if err := s.network.StartAll(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

//stopNetwork停止网络中的所有节点
func (s *Server) StopNetwork(w http.ResponseWriter, req *http.Request) {
	if err := s.network.StopAll(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

//StartMocker启动Mocker节点模拟
func (s *Server) StartMocker(w http.ResponseWriter, req *http.Request) {
	s.mockerMtx.Lock()
	defer s.mockerMtx.Unlock()
	if s.mockerStop != nil {
		http.Error(w, "mocker already running", http.StatusInternalServerError)
		return
	}
	mockerType := req.FormValue("mocker-type")
	mockerFn := LookupMocker(mockerType)
	if mockerFn == nil {
		http.Error(w, fmt.Sprintf("unknown mocker type %q", mockerType), http.StatusBadRequest)
		return
	}
	nodeCount, err := strconv.Atoi(req.FormValue("node-count"))
	if err != nil {
		http.Error(w, "invalid node-count provided", http.StatusBadRequest)
		return
	}
	s.mockerStop = make(chan struct{})
	go mockerFn(s.network, s.mockerStop, nodeCount)

	w.WriteHeader(http.StatusOK)
}

//StopMocker停止Mocker节点模拟
func (s *Server) StopMocker(w http.ResponseWriter, req *http.Request) {
	s.mockerMtx.Lock()
	defer s.mockerMtx.Unlock()
	if s.mockerStop == nil {
		http.Error(w, "stop channel not initialized", http.StatusInternalServerError)
		return
	}
	close(s.mockerStop)
	s.mockerStop = nil

	w.WriteHeader(http.StatusOK)
}

//getmokerlist返回可用mocker的列表
func (s *Server) GetMockers(w http.ResponseWriter, req *http.Request) {

	list := GetMockerList()
	s.JSON(w, http.StatusOK, list)
}

//ResetNetwork将网络的所有属性重置为其初始（空）状态
func (s *Server) ResetNetwork(w http.ResponseWriter, req *http.Request) {
	s.network.Reset()

	w.WriteHeader(http.StatusOK)
}

//streamNetworkEvents将网络事件作为服务器发送的事件流进行流式处理
func (s *Server) StreamNetworkEvents(w http.ResponseWriter, req *http.Request) {
	events := make(chan *Event)
	sub := s.network.events.Subscribe(events)
	defer sub.Unsubscribe()

//如果客户端不在，则停止流
	var clientGone <-chan bool
	if cn, ok := w.(http.CloseNotifier); ok {
		clientGone = cn.CloseNotify()
	}

//写入将给定事件和数据写入流，如下所示：
//
//事件：<事件>
//数据：<数据>
//
	write := func(event, data string) {
		fmt.Fprintf(w, "event: %s\n", event)
		fmt.Fprintf(w, "data: %s\n\n", data)
		if fw, ok := w.(http.Flusher); ok {
			fw.Flush()
		}
	}
	writeEvent := func(event *Event) error {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		write("network", string(data))
		return nil
	}
	writeErr := func(err error) {
		write("error", err.Error())
	}

//检查是否已请求筛选
	var filters MsgFilters
	if filterParam := req.URL.Query().Get("filter"); filterParam != "" {
		var err error
		filters, err = NewMsgFilters(filterParam)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "\n\n")
	if fw, ok := w.(http.Flusher); ok {
		fw.Flush()
	}

//可选发送现有节点和连接
	if req.URL.Query().Get("current") == "true" {
		snap, err := s.network.Snapshot()
		if err != nil {
			writeErr(err)
			return
		}
		for _, node := range snap.Nodes {
			event := NewEvent(&node.Node)
			if err := writeEvent(event); err != nil {
				writeErr(err)
				return
			}
		}
		for _, conn := range snap.Conns {
			event := NewEvent(&conn)
			if err := writeEvent(event); err != nil {
				writeErr(err)
				return
			}
		}
	}

	for {
		select {
		case event := <-events:
//仅发送与筛选器匹配的消息事件
			if event.Msg != nil && !filters.Match(event.Msg) {
				continue
			}
			if err := writeEvent(event); err != nil {
				writeErr(err)
				return
			}
		case <-clientGone:
			return
		}
	}
}

//newmsgfilters从URL查询构造消息筛选器集合
//参数。
//
//该参数应为单独过滤器的虚线分隔列表，
//每个都具有格式“<proto>：<codes>”，其中<proto>是
//Protocol和<codes>是以逗号分隔的消息代码列表。
//
//“*”或“-1”的消息代码被视为通配符并与任何代码匹配。
func NewMsgFilters(filterParam string) (MsgFilters, error) {
	filters := make(MsgFilters)
	for _, filter := range strings.Split(filterParam, "-") {
		protoCodes := strings.SplitN(filter, ":", 2)
		if len(protoCodes) != 2 || protoCodes[0] == "" || protoCodes[1] == "" {
			return nil, fmt.Errorf("invalid message filter: %s", filter)
		}
		proto := protoCodes[0]
		for _, code := range strings.Split(protoCodes[1], ",") {
			if code == "*" || code == "-1" {
				filters[MsgFilter{Proto: proto, Code: -1}] = struct{}{}
				continue
			}
			n, err := strconv.ParseUint(code, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid message code: %s", code)
			}
			filters[MsgFilter{Proto: proto, Code: int64(n)}] = struct{}{}
		}
	}
	return filters, nil
}

//msgfilters是用于筛选消息的筛选器的集合
//事件
type MsgFilters map[MsgFilter]struct{}

//匹配检查给定消息是否与任何筛选器匹配
func (m MsgFilters) Match(msg *Msg) bool {
//检查是否存在消息协议的通配符筛选器
	if _, ok := m[MsgFilter{Proto: msg.Protocol, Code: -1}]; ok {
		return true
	}

//检查是否有消息协议和代码的筛选器
	if _, ok := m[MsgFilter{Proto: msg.Protocol, Code: int64(msg.Code)}]; ok {
		return true
	}

	return false
}

//msgfilter用于根据协议和消息筛选消息事件
//代码
type MsgFilter struct {
//协议与消息的协议匹配
	Proto string

//代码与消息的代码匹配，其中-1与所有代码匹配
	Code int64
}

//创建快照创建网络快照
func (s *Server) CreateSnapshot(w http.ResponseWriter, req *http.Request) {
	snap, err := s.network.Snapshot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.JSON(w, http.StatusOK, snap)
}

//LoadSnapshot将快照加载到网络中
func (s *Server) LoadSnapshot(w http.ResponseWriter, req *http.Request) {
	snap := &Snapshot{}
	if err := json.NewDecoder(req.Body).Decode(snap); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.network.Load(snap); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.JSON(w, http.StatusOK, s.network)
}

//createNode使用给定的配置在网络中创建节点
func (s *Server) CreateNode(w http.ResponseWriter, req *http.Request) {
	config := &adapters.NodeConfig{}

	err := json.NewDecoder(req.Body).Decode(config)
	if err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node, err := s.network.NewNodeWithConfig(config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.JSON(w, http.StatusCreated, node.NodeInfo())
}

//getnodes返回网络中存在的所有节点
func (s *Server) GetNodes(w http.ResponseWriter, req *http.Request) {
	nodes := s.network.GetNodes()

	infos := make([]*p2p.NodeInfo, len(nodes))
	for i, node := range nodes {
		infos[i] = node.NodeInfo()
	}

	s.JSON(w, http.StatusOK, infos)
}

//getnode返回节点的详细信息
func (s *Server) GetNode(w http.ResponseWriter, req *http.Request) {
	node := req.Context().Value("node").(*Node)

	s.JSON(w, http.StatusOK, node.NodeInfo())
}

//startnode启动节点
func (s *Server) StartNode(w http.ResponseWriter, req *http.Request) {
	node := req.Context().Value("node").(*Node)

	if err := s.network.Start(node.ID()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.JSON(w, http.StatusOK, node.NodeInfo())
}

//停止节点停止节点
func (s *Server) StopNode(w http.ResponseWriter, req *http.Request) {
	node := req.Context().Value("node").(*Node)

	if err := s.network.Stop(node.ID()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.JSON(w, http.StatusOK, node.NodeInfo())
}

//ConnectNode将节点连接到对等节点
func (s *Server) ConnectNode(w http.ResponseWriter, req *http.Request) {
	node := req.Context().Value("node").(*Node)
	peer := req.Context().Value("peer").(*Node)

	if err := s.network.Connect(node.ID(), peer.ID()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.JSON(w, http.StatusOK, node.NodeInfo())
}

//断开节点断开节点与对等节点的连接
func (s *Server) DisconnectNode(w http.ResponseWriter, req *http.Request) {
	node := req.Context().Value("node").(*Node)
	peer := req.Context().Value("peer").(*Node)

	if err := s.network.Disconnect(node.ID(), peer.ID()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.JSON(w, http.StatusOK, node.NodeInfo())
}

//选项通过返回200 OK响应来响应选项HTTP方法
//将“访问控制允许邮件头”邮件头设置为“内容类型”
func (s *Server) Options(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(http.StatusOK)
}

//node rpc通过websocket将rpc请求转发到网络中的节点
//连接
func (s *Server) NodeRPC(w http.ResponseWriter, req *http.Request) {
	node := req.Context().Value("node").(*Node)

	handler := func(conn *websocket.Conn) {
		node.ServeRPC(conn)
	}

	websocket.Server{Handler: handler}.ServeHTTP(w, req)
}

//servehtp通过委托给
//底层httprouter.router
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.router.ServeHTTP(w, req)
}

//get为特定路径的get请求注册一个处理程序
func (s *Server) GET(path string, handle http.HandlerFunc) {
	s.router.GET(path, s.wrapHandler(handle))
}

//post为特定路径的post请求注册一个处理程序
func (s *Server) POST(path string, handle http.HandlerFunc) {
	s.router.POST(path, s.wrapHandler(handle))
}

//delete为特定路径的删除请求注册处理程序
func (s *Server) DELETE(path string, handle http.HandlerFunc) {
	s.router.DELETE(path, s.wrapHandler(handle))
}

//选项为特定路径的选项请求注册处理程序
func (s *Server) OPTIONS(path string, handle http.HandlerFunc) {
 /*outer.options（“/*路径”，s.wraphandler（handle））
}

//JSON以JSON HTTP响应的形式发送“data”
func（s*server）json（w http.responsewriter，status int，数据接口）
 w.header（）.set（“内容类型”，“应用程序/json”）。
 w.writeheader（状态）
 json.newencoder（w）.encode（数据）
}

//wraphandler返回一个httprouter.handle，它将http.handlerFunc包装为
//用URL参数中的任何对象填充request.context
func（s*server）wraphandler（handlerHTTP.handlerFunc）httprouter.handle_
 返回func（w http.responsewriter，req*http.request，params httprouter.params）
  w.header（）.set（“访问控制允许来源”，“*”）
  w.header（）.set（“访问控制允许方法”、“获取、发布、放置、删除、选项”）。

  ctx：=context.background（）。

  如果id：=params.byname（“nodeid”）；id！=“{”
   变量nodeid enode.id
   VAR节点*节点
   if nodeid.unmashaltext（[]byte（id））==nil
    node=s.network.getnode（nodeid）
   }否则{
    node=s.network.getnodebyname（id）
   }
   如果节点==nil
    未找到（w，req）
    返回
   }
   ctx=context.withValue（ctx，“节点”，节点）
  }

  如果id：=params.byname（“peerid”）；id！=“{”
   变量peerid enode.id
   VaR对等节点
   if peerID.unmashaltext（[]byte（id））==nil
    peer=s.network.getnode（peerid）
   }否则{
    peer=s.network.getnodebyname（id）
   }
   如果peer==nil
    未找到（w，req）
    返回
   }
   ctx=context.withValue（ctx，“对等”，对等）
  }

  处理程序（w，req.withContext（ctx））。
 }
}
