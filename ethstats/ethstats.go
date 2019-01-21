
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

//包ethstats实现网络状态报告服务。
package ethstats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/les"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/net/websocket"
)

const (
//HistoryUpdateRange是节点登录时应报告的块数，或者
//历史要求。
	historyUpdateRange = 50

//txchanSize是侦听newtxSevent的频道的大小。
//该数字是根据Tx池的大小引用的。
	txChanSize = 4096
//ChainHeadChansize是侦听ChainHeadEvent的通道的大小。
	chainHeadChanSize = 10
)

type txPool interface {
//subscribenewtxsevent应返回的事件订阅
//newtxSevent并将事件发送到给定的通道。
	SubscribeNewTxsEvent(chan<- core.NewTxsEvent) event.Subscription
}

type blockChain interface {
	SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription
}

//服务实现一个以太坊netstats报告守护进程，该守护进程将本地
//将统计信息链接到监控服务器。
type Service struct {
server *p2p.Server        //对等服务器检索网络信息
eth    *eth.Ethereum      //如果监视完整节点，则提供完整的以太坊服务
les    *les.LightEthereum //光以太坊服务，如果监控光节点
engine consensus.Engine   //用于检索可变块字段的协商引擎

node string //要在监视页上显示的节点的名称
pass string //Password to authorize access to the monitoring page
host string //监控服务的远程地址

pongCh chan struct{} //pong通知被送入此频道
histCh chan []uint64 //历史请求块编号被馈入该信道。
}

//new返回一个监控服务，准备好进行状态报告。
func New(url string, ethServ *eth.Ethereum, lesServ *les.LightEthereum) (*Service, error) {
//分析netstats连接URL
	re := regexp.MustCompile("([^:@]*)(:([^@]*))?@(.+)")
	parts := re.FindStringSubmatch(url)
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid netstats url: \"%s\", should be nodename:secret@host:port", url)
	}
//集合并返回统计服务
	var engine consensus.Engine
	if ethServ != nil {
		engine = ethServ.Engine()
	} else {
		engine = lesServ.Engine()
	}
	return &Service{
		eth:    ethServ,
		les:    lesServ,
		engine: engine,
		node:   parts[1],
		pass:   parts[3],
		host:   parts[4],
		pongCh: make(chan struct{}),
		histCh: make(chan []uint64, 1),
	}, nil
}

//协议实现node.service，返回使用的p2p网络协议
//通过stats服务（无，因为它不使用devp2p覆盖网络）。
func (s *Service) Protocols() []p2p.Protocol { return nil }

//API实现node.service，返回由
//统计服务（无，因为它不提供任何用户可调用的API）。
func (s *Service) APIs() []rpc.API { return nil }

//start实现node.service，启动监视和报告守护进程。
func (s *Service) Start(server *p2p.Server) error {
	s.server = server
	go s.loop()

	log.Info("Stats daemon started")
	return nil
}

//stop实现node.service，终止监视和报告守护进程。
func (s *Service) Stop() error {
	log.Info("Stats daemon stopped")
	return nil
}

//循环不断尝试连接到netstats服务器，报告链事件
//直到终止。
func (s *Service) loop() {
//订阅链事件以在其上执行更新
	var blockchain blockChain
	var txpool txPool
	if s.eth != nil {
		blockchain = s.eth.BlockChain()
		txpool = s.eth.TxPool()
	} else {
		blockchain = s.les.BlockChain()
		txpool = s.les.TxPool()
	}

	chainHeadCh := make(chan core.ChainHeadEvent, chainHeadChanSize)
	headSub := blockchain.SubscribeChainHeadEvent(chainHeadCh)
	defer headSub.Unsubscribe()

	txEventCh := make(chan core.NewTxsEvent, txChanSize)
	txSub := txpool.SubscribeNewTxsEvent(txEventCh)
	defer txSub.Unsubscribe()

//启动一个排出子脚本的goroutine，以避免事件堆积。
	var (
		quitCh = make(chan struct{})
		headCh = make(chan *types.Block, 1)
		txCh   = make(chan struct{}, 1)
	)
	go func() {
		var lastTx mclock.AbsTime

	HandleLoop:
		for {
			select {
//通知链头事件，但如果太频繁，则丢弃
			case head := <-chainHeadCh:
				select {
				case headCh <- head.Block:
				default:
				}

//通知新的事务事件，但如果太频繁则删除
			case <-txEventCh:
				if time.Duration(mclock.Now()-lastTx) < time.Second {
					continue
				}
				lastTx = mclock.Now()

				select {
				case txCh <- struct{}{}:
				default:
				}

//节点停止
			case <-txSub.Err():
				break HandleLoop
			case <-headSub.Err():
				break HandleLoop
			}
		}
		close(quitCh)
	}()
//循环报告直到终止
	for {
//Resolve the URL, defaulting to TLS, but falling back to none too
		path := fmt.Sprintf("%s/api", s.host)
		urls := []string{path}

if !strings.Contains(path, "://“）//url.parse和url.isabs不适合（https://github.com/golang/go/issues/19779）
urls = []string{"wss://“+路径，”ws://“+路径
		}
//Establish a websocket connection to the server on any supported URL
		var (
			conf *websocket.Config
			conn *websocket.Conn
			err  error
		)
		for _, url := range urls {
if conf, err = websocket.NewConfig(url, "http://localhost/“）；错误！= nIL{
				continue
			}
			conf.Dialer = &net.Dialer{Timeout: 5 * time.Second}
			if conn, err = websocket.DialConfig(conf); err == nil {
				break
			}
		}
		if err != nil {
			log.Warn("Stats server unreachable", "err", err)
			time.Sleep(10 * time.Second)
			continue
		}
//向服务器验证客户端
		if err = s.login(conn); err != nil {
			log.Warn("Stats login failed", "err", err)
			conn.Close()
			time.Sleep(10 * time.Second)
			continue
		}
		go s.readLoop(conn)

//发送初始统计信息，以便我们的节点从一开始就看起来不错
		if err = s.report(conn); err != nil {
			log.Warn("Initial stats report failed", "err", err)
			conn.Close()
			continue
		}
//继续发送状态更新，直到连接断开
		fullReport := time.NewTicker(15 * time.Second)

		for err == nil {
			select {
			case <-quitCh:
				conn.Close()
				return

			case <-fullReport.C:
				if err = s.report(conn); err != nil {
					log.Warn("Full stats report failed", "err", err)
				}
			case list := <-s.histCh:
				if err = s.reportHistory(conn, list); err != nil {
					log.Warn("Requested history report failed", "err", err)
				}
			case head := <-headCh:
				if err = s.reportBlock(conn, head); err != nil {
					log.Warn("Block stats report failed", "err", err)
				}
				if err = s.reportPending(conn); err != nil {
					log.Warn("Post-block transaction stats report failed", "err", err)
				}
			case <-txCh:
				if err = s.reportPending(conn); err != nil {
					log.Warn("Transaction stats report failed", "err", err)
				}
			}
		}
//确保连接已关闭
		conn.Close()
	}
}

//只要连接处于活动状态并检索数据包，readloop就会循环。
//从网络插座。如果其中任何一个匹配活动的请求，它将转发
//如果他们自己是请求，它会启动一个回复，最后它会下降。
//未知数据包。
func (s *Service) readLoop(conn *websocket.Conn) {
//如果存在读取循环，请关闭连接
	defer conn.Close()

	for {
//检索下一个通用网络包并在出错时退出
		var msg map[string][]interface{}
		if err := websocket.JSON.Receive(conn, &msg); err != nil {
			log.Warn("Failed to decode stats server message", "err", err)
			return
		}
		log.Trace("Received message from stats server", "msg", msg)
		if len(msg["emit"]) == 0 {
			log.Warn("Stats server sent non-broadcast", "msg", msg)
			return
		}
		command, ok := msg["emit"][0].(string)
		if !ok {
			log.Warn("Invalid stats server message type", "type", msg["emit"][0])
			return
		}
//如果消息是ping回复，则传递（必须有人正在收听！）
		if len(msg["emit"]) == 2 && command == "node-pong" {
			select {
			case s.pongCh <- struct{}{}:
//Pong已发送，继续收听
				continue
			default:
//Ping程序死亡，中止
				log.Warn("Stats server pinger seems to have died")
				return
			}
		}
//如果消息是历史请求，则转发到事件处理器。
		if len(msg["emit"]) == 2 && command == "history" {
//确保请求是有效的并且不会崩溃
			request, ok := msg["emit"][1].(map[string]interface{})
			if !ok {
				log.Warn("Invalid stats history request", "msg", msg["emit"][1])
				s.histCh <- nil
continue //ethstats有时发送无效的历史请求，忽略这些请求
			}
			list, ok := request["list"].([]interface{})
			if !ok {
				log.Warn("Invalid stats history block list", "list", request["list"])
				return
			}
//将块编号列表转换为整数列表
			numbers := make([]uint64, len(list))
			for i, num := range list {
				n, ok := num.(float64)
				if !ok {
					log.Warn("Invalid stats history block number", "number", num)
					return
				}
				numbers[i] = uint64(n)
			}
			select {
			case s.histCh <- numbers:
				continue
			default:
			}
		}
//报告其他内容并继续
		log.Info("Unknown stats message", "msg", msg)
	}
}

//nodeinfo是有关显示的节点的元信息的集合。
//在监控页面上。
type nodeInfo struct {
	Name     string `json:"name"`
	Node     string `json:"node"`
	Port     int    `json:"port"`
	Network  string `json:"net"`
	Protocol string `json:"protocol"`
	API      string `json:"api"`
	Os       string `json:"os"`
	OsVer    string `json:"os_v"`
	Client   string `json:"client"`
	History  bool   `json:"canUpdateHistory"`
}

//authmsg是登录监控服务器所需的身份验证信息。
type authMsg struct {
	ID     string   `json:"id"`
	Info   nodeInfo `json:"info"`
	Secret string   `json:"secret"`
}

//登录尝试在远程服务器上授权客户端。
func (s *Service) login(conn *websocket.Conn) error {
//构造并发送登录验证
	infos := s.server.NodeInfo()

	var network, protocol string
	if info := infos.Protocols["eth"]; info != nil {
		network = fmt.Sprintf("%d", info.(*eth.NodeInfo).Network)
		protocol = fmt.Sprintf("eth/%d", eth.ProtocolVersions[0])
	} else {
		network = fmt.Sprintf("%d", infos.Protocols["les"].(*les.NodeInfo).Network)
		protocol = fmt.Sprintf("les/%d", les.ClientProtocolVersions[0])
	}
	auth := &authMsg{
		ID: s.node,
		Info: nodeInfo{
			Name:     s.node,
			Node:     infos.Name,
			Port:     infos.Ports.Listener,
			Network:  network,
			Protocol: protocol,
			API:      "No",
			Os:       runtime.GOOS,
			OsVer:    runtime.GOARCH,
			Client:   "0.1.1",
			History:  true,
		},
		Secret: s.pass,
	}
	login := map[string][]interface{}{
		"emit": {"hello", auth},
	}
	if err := websocket.JSON.Send(conn, login); err != nil {
		return err
	}
//检索远程确认或连接终止
	var ack map[string][]string
	if err := websocket.JSON.Receive(conn, &ack); err != nil || len(ack["emit"]) != 1 || ack["emit"][0] != "ready" {
		return errors.New("unauthorized")
	}
	return nil
}

//报告收集所有可能的数据进行报告，并将其发送到Stats服务器。
//这只能用于重新连接，或很少用于避免
//服务器。使用单独的方法报告订阅的事件。
func (s *Service) report(conn *websocket.Conn) error {
	if err := s.reportLatency(conn); err != nil {
		return err
	}
	if err := s.reportBlock(conn, nil); err != nil {
		return err
	}
	if err := s.reportPending(conn); err != nil {
		return err
	}
	if err := s.reportStats(conn); err != nil {
		return err
	}
	return nil
}

//reportlatency向服务器发送ping请求，测量RTT时间和
//最后发送延迟更新。
func (s *Service) reportLatency(conn *websocket.Conn) error {
//将当前时间发送到ethstats服务器
	start := time.Now()

	ping := map[string][]interface{}{
		"emit": {"node-ping", map[string]string{
			"id":         s.node,
			"clientTime": start.String(),
		}},
	}
	if err := websocket.JSON.Send(conn, ping); err != nil {
		return err
	}
//等待PONG请求返回
	select {
	case <-s.pongCh:
//Pong delivered, report the latency
	case <-time.After(5 * time.Second):
//Ping超时，中止
		return errors.New("ping timed out")
	}
	latency := strconv.Itoa(int((time.Since(start) / time.Duration(2)).Nanoseconds() / 1000000))

//发送回测量的延迟
	log.Trace("Sending measured latency to ethstats", "latency", latency)

	stats := map[string][]interface{}{
		"emit": {"latency", map[string]string{
			"id":      s.node,
			"latency": latency,
		}},
	}
	return websocket.JSON.Send(conn, stats)
}

//BuBSTATS是报告单个块的信息。
type blockStats struct {
	Number     *big.Int       `json:"number"`
	Hash       common.Hash    `json:"hash"`
	ParentHash common.Hash    `json:"parentHash"`
	Timestamp  *big.Int       `json:"timestamp"`
	Miner      common.Address `json:"miner"`
	GasUsed    uint64         `json:"gasUsed"`
	GasLimit   uint64         `json:"gasLimit"`
	Diff       string         `json:"difficulty"`
	TotalDiff  string         `json:"totalDifficulty"`
	Txs        []txStats      `json:"transactions"`
	TxHash     common.Hash    `json:"transactionsRoot"`
	Root       common.Hash    `json:"stateRoot"`
	Uncles     uncleStats     `json:"uncles"`
}

//TxStats是要报告单个交易的信息。
type txStats struct {
	Hash common.Hash `json:"hash"`
}

//Unclestats是一个自定义包装器，它围绕一个叔叔数组强制序列化。
//empty arrays instead of returning null for them.
type uncleStats []*types.Header

func (s uncleStats) MarshalJSON() ([]byte, error) {
	if uncles := ([]*types.Header)(s); len(uncles) > 0 {
		return json.Marshal(uncles)
	}
	return []byte("[]"), nil
}

//ReportBlock检索当前链头并将其报告给Stats服务器。
func (s *Service) reportBlock(conn *websocket.Conn, block *types.Block) error {
//从收割台或区块链收集区块详细信息
	details := s.assembleBlockStats(block)

//组装块报表并发送给服务器
	log.Trace("Sending new block to ethstats", "number", details.Number, "hash", details.Hash)

	stats := map[string]interface{}{
		"id":    s.node,
		"block": details,
	}
	report := map[string][]interface{}{
		"emit": {"block", stats},
	}
	return websocket.JSON.Send(conn, report)
}

//assembleBlockstats检索报告单个块所需的任何元数据
//并汇编块统计。如果block为nil，则处理当前头。
func (s *Service) assembleBlockStats(block *types.Block) *blockStats {
//从本地区块链收集区块信息
	var (
		header *types.Header
		td     *big.Int
		txs    []txStats
		uncles []*types.Header
	)
	if s.eth != nil {
//完整节点具有所有可用的所需信息
		if block == nil {
			block = s.eth.BlockChain().CurrentBlock()
		}
		header = block.Header()
		td = s.eth.BlockChain().GetTd(header.Hash(), header.Number.Uint64())

		txs = make([]txStats, len(block.Transactions()))
		for i, tx := range block.Transactions() {
			txs[i].Hash = tx.Hash()
		}
		uncles = block.Uncles()
	} else {
//轻节点需要按需查找事务/叔叔，跳过
		if block != nil {
			header = block.Header()
		} else {
			header = s.les.BlockChain().CurrentHeader()
		}
		td = s.les.BlockChain().GetTd(header.Hash(), header.Number.Uint64())
		txs = []txStats{}
	}
//集合并返回块统计信息
	author, _ := s.engine.Author(header)

	return &blockStats{
		Number:     header.Number,
		Hash:       header.Hash(),
		ParentHash: header.ParentHash,
		Timestamp:  header.Time,
		Miner:      author,
		GasUsed:    header.GasUsed,
		GasLimit:   header.GasLimit,
		Diff:       header.Difficulty.String(),
		TotalDiff:  td.String(),
		Txs:        txs,
		TxHash:     header.TxHash,
		Root:       header.Root,
		Uncles:     uncles,
	}
}

//ReportHistory检索最近一批块并将其报告给
//统计服务器。
func (s *Service) reportHistory(conn *websocket.Conn, list []uint64) error {
//找出需要报告的索引
	indexes := make([]uint64, 0, historyUpdateRange)
	if len(list) > 0 {
//请求的特定索引，尤其是将其发回
		indexes = append(indexes, list...)
	} else {
//没有请求索引，请将最上面的索引发回
		var head int64
		if s.eth != nil {
			head = s.eth.BlockChain().CurrentHeader().Number.Int64()
		} else {
			head = s.les.BlockChain().CurrentHeader().Number.Int64()
		}
		start := head - historyUpdateRange + 1
		if start < 0 {
			start = 0
		}
		for i := uint64(start); i <= uint64(head); i++ {
			indexes = append(indexes, i)
		}
	}
//Gather the batch of blocks to report
	history := make([]*blockStats, len(indexes))
	for i, number := range indexes {
//如果我们知道下一个块，就取回它
		var block *types.Block
		if s.eth != nil {
			block = s.eth.BlockChain().GetBlockByNumber(number)
		} else {
			if header := s.les.BlockChain().GetHeaderByNumber(number); header != nil {
				block = types.NewBlockWithHeader(header)
			}
		}
//如果我们确实有这个块，请添加到历史记录并继续
		if block != nil {
			history[len(history)-1-i] = s.assembleBlockStats(block)
			continue
		}
//快用完了，把报告剪短然后发送
		history = history[len(history)-i:]
		break
	}
//组装历史报告并将其发送到服务器
	if len(history) > 0 {
		log.Trace("Sending historical blocks to ethstats", "first", history[0].Number, "last", history[len(history)-1].Number)
	} else {
		log.Trace("No history to send to stats server")
	}
	stats := map[string]interface{}{
		"id":      s.node,
		"history": history,
	}
	report := map[string][]interface{}{
		"emit": {"history", stats},
	}
	return websocket.JSON.Send(conn, report)
}

//PendStats是有关挂起事务的报告信息。
type pendStats struct {
	Pending int `json:"pending"`
}

//reportPending检索当前挂起的事务和报告数
//到统计服务器。
func (s *Service) reportPending(conn *websocket.Conn) error {
//Retrieve the pending count from the local blockchain
	var pending int
	if s.eth != nil {
		pending, _ = s.eth.TxPool().Stats()
	} else {
		pending = s.les.TxPool().Stats()
	}
//组装事务状态并将其发送到服务器
	log.Trace("Sending pending transactions to ethstats", "count", pending)

	stats := map[string]interface{}{
		"id": s.node,
		"stats": &pendStats{
			Pending: pending,
		},
	}
	report := map[string][]interface{}{
		"emit": {"pending", stats},
	}
	return websocket.JSON.Send(conn, report)
}

//nodestats是要报告的有关本地节点的信息。
type nodeStats struct {
	Active   bool `json:"active"`
	Syncing  bool `json:"syncing"`
	Mining   bool `json:"mining"`
	Hashrate int  `json:"hashrate"`
	Peers    int  `json:"peers"`
	GasPrice int  `json:"gasPrice"`
	Uptime   int  `json:"uptime"`
}

//reportPending检索有关网络节点的各种统计信息，以及
//挖掘层并将其报告给Stats服务器。
func (s *Service) reportStats(conn *websocket.Conn) error {
//从本地矿工实例收集同步和挖掘信息
	var (
		mining   bool
		hashrate int
		syncing  bool
		gasprice int
	)
	if s.eth != nil {
		mining = s.eth.Miner().Mining()
		hashrate = int(s.eth.Miner().HashRate())

		sync := s.eth.Downloader().Progress()
		syncing = s.eth.BlockChain().CurrentHeader().Number.Uint64() >= sync.HighestBlock

		price, _ := s.eth.APIBackend.SuggestPrice(context.Background())
		gasprice = int(price.Uint64())
	} else {
		sync := s.les.Downloader().Progress()
		syncing = s.les.BlockChain().CurrentHeader().Number.Uint64() >= sync.HighestBlock
	}
//Assemble the node stats and send it to the server
	log.Trace("Sending node details to ethstats")

	stats := map[string]interface{}{
		"id": s.node,
		"stats": &nodeStats{
			Active:   true,
			Mining:   mining,
			Hashrate: hashrate,
			Peers:    s.server.PeerCount(),
			GasPrice: gasprice,
			Syncing:  syncing,
			Uptime:   100,
		},
	}
	report := map[string][]interface{}{
		"emit": {"stats", stats},
	}
	return websocket.JSON.Send(conn, report)
}
