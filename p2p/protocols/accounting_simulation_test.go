
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

package protocols

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/mattn/go-colorable"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
)

const (
	content = "123456789"
)

var (
	nodes    = flag.Int("nodes", 30, "number of nodes to create (default 30)")
	msgs     = flag.Int("msgs", 100, "number of messages sent by node (default 100)")
	loglevel = flag.Int("loglevel", 0, "verbosity of logs")
	rawlog   = flag.Bool("rawlog", false, "remove terminal formatting from logs")
)

func init() {
	flag.Parse()
	log.PrintOrigins(true)
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(!*rawlog))))
}

//测试计算模拟运行p2p/模拟仿真
//它创建一个*节点数的节点，彼此连接，
//然后发送随机选择的消息，最多可发送*msgs数量的消息
//来自测试协议规范。
//规范通过价格接口定义了一些记帐消息。
//测试会计算所有交换的消息，然后检查
//每个节点与对等节点具有相同的平衡，但符号相反。
//平衡（awithb）=0-平衡（bwitha）或abs平衡（awithb）==abs平衡（bwitha）
func TestAccountingSimulation(t *testing.T) {
//为每个节点设置Balances对象
	bal := newBalances(*nodes)
//设置度量系统或测试在尝试写入度量时将失败
	dir, err := ioutil.TempDir("", "account-sim")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	SetupAccountingMetrics(1*time.Second, filepath.Join(dir, "metrics.db"))
//定义此测试的node.service
	services := adapters.Services{
		"accounting": func(ctx *adapters.ServiceContext) (node.Service, error) {
			return bal.newNode(), nil
		},
	}
//设置模拟
	adapter := adapters.NewSimAdapter(services)
	net := simulations.NewNetwork(adapter, &simulations.NetworkConfig{DefaultService: "accounting"})
	defer net.Shutdown()

//我们每个节点发送MSGS消息，等待所有消息到达
	bal.wg.Add(*nodes * *msgs)
	trigger := make(chan enode.ID)
	go func() {
//等他们都到了
		bal.wg.Wait()
//然后触发检查
//触发器的选定节点不相关，
//我们只想触发模拟的结束
		trigger <- net.Nodes[0].ID()
	}()

//创建节点并启动它们
	for i := 0; i < *nodes; i++ {
		conf := adapters.RandomNodeConfig()
		bal.id2n[conf.ID] = i
		if _, err := net.NewNodeWithConfig(conf); err != nil {
			t.Fatal(err)
		}
		if err := net.Start(conf.ID); err != nil {
			t.Fatal(err)
		}
	}
//完全连接节点
	for i, n := range net.Nodes {
		for _, m := range net.Nodes[i+1:] {
			if err := net.Connect(n.ID(), m.ID()); err != nil {
				t.Fatal(err)
			}
		}
	}

//空动作
	action := func(ctx context.Context) error {
		return nil
	}
//检查始终签出
	check := func(ctx context.Context, id enode.ID) (bool, error) {
		return true, nil
	}

//运行仿真
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	result := simulations.NewSimulation(net).Run(ctx, &simulations.Step{
		Action:  action,
		Trigger: trigger,
		Expect: &simulations.Expectation{
			Nodes: []enode.ID{net.Nodes[0].ID()},
			Check: check,
		},
	})

	if result.Error != nil {
		t.Fatal(result.Error)
	}

//检查平衡矩阵是否对称
	if err := bal.symmetric(); err != nil {
		t.Fatal(err)
	}
}

//矩阵是节点及其平衡的矩阵
//矩阵实际上是一个大小为n*n的线性数组，
//所以任何节点a和b的余额都在索引处。
//a*n+b，b节点与a的平衡为
//B*N+A
//（数组中的n个条目将不会填充-
//节点本身的平衡）
type matrix struct {
n int     //节点数
m []int64 //余额数组
}

//创建新矩阵
func newMatrix(n int) *matrix {
	return &matrix{
		n: n,
		m: make([]int64, n*n),
	}
}

//从testBalance的add accounting函数调用：注册余额更改
func (m *matrix) add(i, j int, v int64) error {
//本地节点i与远程节点j的平衡索引为
//i*节点数+远程节点
	mi := i*m.n + j
//登记余额
	m.m[mi] += v
	return nil
}

//检查天平是否对称：
//i节点与j节点的平衡与j节点与i节点的平衡相同，但有倒置的符号。
func (m *matrix) symmetric() error {
//迭代所有节点
	for i := 0; i < m.n; i++ {
//迭代开始+1
		for j := i + 1; j < m.n; j++ {
			log.Debug("bal", "1", i, "2", j, "i,j", m.m[i*m.n+j], "j,i", m.m[j*m.n+i])
			if m.m[i*m.n+j] != -m.m[j*m.n+i] {
				return fmt.Errorf("value mismatch. m[%v, %v] = %v; m[%v, %v] = %v", i, j, m.m[i*m.n+j], j, i, m.m[j*m.n+i])
			}
		}
	}
	return nil
}

//所有余额
type balances struct {
	i int
	*matrix
	id2n map[enode.ID]int
	wg   *sync.WaitGroup
}

func newBalances(n int) *balances {
	return &balances{
		matrix: newMatrix(n),
		id2n:   make(map[enode.ID]int),
		wg:     &sync.WaitGroup{},
	}
}

//为作为服务一部分创建的每个节点创建一个新的测试节点
func (b *balances) newNode() *testNode {
	defer func() { b.i++ }()
	return &testNode{
		bal:   b,
		i:     b.i,
peers: make([]*testPeer, b.n), //节点将连接到n-1对等机
	}
}

type testNode struct {
	bal       *balances
	i         int
	lock      sync.Mutex
	peers     []*testPeer
	peerCount int
}

//计算对等方的测试协议
//testnode实现协议。平衡
func (t *testNode) Add(a int64, p *Peer) error {
//获取远程对等机的索引
	remote := t.bal.id2n[p.ID()]
	log.Debug("add", "local", t.i, "remote", remote, "amount", a)
	return t.bal.add(t.i, remote, a)
}

//运行p2p协议
//对于由testnode表示的每个节点，创建一个远程testpeer
func (t *testNode) run(p *p2p.Peer, rw p2p.MsgReadWriter) error {
	spec := createTestSpec()
//创建会计挂钩
	spec.Hook = NewAccounting(t, &dummyPrices{})

//为此节点创建对等点
	tp := &testPeer{NewPeer(p, rw, spec), t.i, t.bal.id2n[p.ID()], t.bal.wg}
	t.lock.Lock()
	t.peers[t.bal.id2n[p.ID()]] = tp
	t.peerCount++
	if t.peerCount == t.bal.n-1 {
//建立所有对等连接后，开始从此对等端发送消息
		go t.send()
	}
	t.lock.Unlock()
	return tp.Run(tp.handle)
}

//P2P消息接收处理函数
func (tp *testPeer) handle(ctx context.Context, msg interface{}) error {
	tp.wg.Done()
	log.Debug("receive", "from", tp.remote, "to", tp.local, "type", reflect.TypeOf(msg), "msg", msg)
	return nil
}

type testPeer struct {
	*Peer
	local, remote int
	wg            *sync.WaitGroup
}

func (t *testNode) send() {
	log.Debug("start sending")
	for i := 0; i < *msgs; i++ {
//随机确定要发送到哪个对等机
		whom := rand.Intn(t.bal.n - 1)
		if whom >= t.i {
			whom++
		}
		t.lock.Lock()
		p := t.peers[whom]
		t.lock.Unlock()

//从要发送的规范消息中确定随机消息
		which := rand.Intn(len(p.spec.Messages))
		msg := p.spec.Messages[which]
		switch msg.(type) {
		case *perBytesMsgReceiverPays:
			msg = &perBytesMsgReceiverPays{Content: content[:rand.Intn(len(content))]}
		case *perBytesMsgSenderPays:
			msg = &perBytesMsgSenderPays{Content: content[:rand.Intn(len(content))]}
		}
		log.Debug("send", "from", t.i, "to", whom, "type", reflect.TypeOf(msg), "msg", msg)
		p.Send(context.TODO(), msg)
	}
}

//定义协议
func (t *testNode) Protocols() []p2p.Protocol {
	return []p2p.Protocol{{
		Length: 100,
		Run:    t.run,
	}}
}

func (t *testNode) APIs() []rpc.API {
	return nil
}

func (t *testNode) Start(server *p2p.Server) error {
	return nil
}

func (t *testNode) Stop() error {
	return nil
}
