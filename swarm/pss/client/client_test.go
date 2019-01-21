
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

package client

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/pss"
	"github.com/ethereum/go-ethereum/swarm/state"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv5"
)

type protoCtrl struct {
	C        chan bool
	protocol *pss.Protocol
	run      func(*p2p.Peer, p2p.MsgReadWriter) error
}

var (
	debugdebugflag = flag.Bool("vv", false, "veryverbose")
	debugflag      = flag.Bool("v", false, "verbose")
	w              *whisper.Whisper
	wapi           *whisper.PublicWhisperAPI
//自定义日志
	psslogmain   log.Logger
	pssprotocols map[string]*protoCtrl
	sendLimit    = uint16(256)
)

var services = newServices()

func init() {
	flag.Parse()
	rand.Seed(time.Now().Unix())

	adapters.RegisterServices(services)

	loglevel := log.LvlInfo
	if *debugflag {
		loglevel = log.LvlDebug
	} else if *debugdebugflag {
		loglevel = log.LvlTrace
	}

	psslogmain = log.New("psslog", "*")
	hs := log.StreamHandler(os.Stderr, log.TerminalFormat(true))
	hf := log.LvlFilterHandler(loglevel, hs)
	h := log.CallerFileHandler(hf)
	log.Root().SetHandler(h)

	w = whisper.New(&whisper.DefaultConfig)
	wapi = whisper.NewPublicWhisperAPI(w)

	pssprotocols = make(map[string]*protoCtrl)
}

//一个过期的symkey之间的乒乓交换
func TestClientHandshake(t *testing.T) {
	sendLimit = 3

	clients, err := setupNetwork(2)
	if err != nil {
		t.Fatal(err)
	}

	lpsc, err := NewClientWithRPC(clients[0])
	if err != nil {
		t.Fatal(err)
	}
	rpsc, err := NewClientWithRPC(clients[1])
	if err != nil {
		t.Fatal(err)
	}
	lpssping := &pss.Ping{
		OutC: make(chan bool),
		InC:  make(chan bool),
		Pong: false,
	}
	rpssping := &pss.Ping{
		OutC: make(chan bool),
		InC:  make(chan bool),
		Pong: false,
	}
	lproto := pss.NewPingProtocol(lpssping)
	rproto := pss.NewPingProtocol(rpssping)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	err = lpsc.RunProtocol(ctx, lproto)
	if err != nil {
		t.Fatal(err)
	}
	err = rpsc.RunProtocol(ctx, rproto)
	if err != nil {
		t.Fatal(err)
	}
	topic := pss.PingTopic.String()

	var loaddr string
	err = clients[0].Call(&loaddr, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 1 baseaddr fail: %v", err)
	}
	var roaddr string
	err = clients[1].Call(&roaddr, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 2 baseaddr fail: %v", err)
	}

	var lpubkey string
	err = clients[0].Call(&lpubkey, "pss_getPublicKey")
	if err != nil {
		t.Fatalf("rpc get node 1 pubkey fail: %v", err)
	}
	var rpubkey string
	err = clients[1].Call(&rpubkey, "pss_getPublicKey")
	if err != nil {
		t.Fatalf("rpc get node 2 pubkey fail: %v", err)
	}

	err = clients[0].Call(nil, "pss_setPeerPublicKey", rpubkey, topic, roaddr)
	if err != nil {
		t.Fatal(err)
	}
	err = clients[1].Call(nil, "pss_setPeerPublicKey", lpubkey, topic, loaddr)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)

	roaddrbytes, err := hexutil.Decode(roaddr)
	if err != nil {
		t.Fatal(err)
	}
	err = lpsc.AddPssPeer(rpubkey, roaddrbytes, pss.PingProtocol)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)

	for i := uint16(0); i <= sendLimit; i++ {
		lpssping.OutC <- false
		got := <-rpssping.InC
		log.Warn("ok", "idx", i, "got", got)
		time.Sleep(time.Second)
	}

	rw := lpsc.peerPool[pss.PingTopic][rpubkey]
	lpsc.RemovePssPeer(rpubkey, pss.PingProtocol)
	if err := rw.WriteMsg(p2p.Msg{
		Size:    3,
		Payload: bytes.NewReader([]byte("foo")),
	}); err == nil {
		t.Fatalf("expected error on write")
	}
}

func setupNetwork(numnodes int) (clients []*rpc.Client, err error) {
	nodes := make([]*simulations.Node, numnodes)
	clients = make([]*rpc.Client, numnodes)
	if numnodes < 2 {
		return nil, fmt.Errorf("Minimum two nodes in network")
	}
	adapter := adapters.NewSimAdapter(services)
	net := simulations.NewNetwork(adapter, &simulations.NetworkConfig{
		ID:             "0",
		DefaultService: "bzz",
	})
	for i := 0; i < numnodes; i++ {
		nodeconf := adapters.RandomNodeConfig()
		nodeconf.Services = []string{"bzz", "pss"}
		nodes[i], err = net.NewNodeWithConfig(nodeconf)
		if err != nil {
			return nil, fmt.Errorf("error creating node 1: %v", err)
		}
		err = net.Start(nodes[i].ID())
		if err != nil {
			return nil, fmt.Errorf("error starting node 1: %v", err)
		}
		if i > 0 {
			err = net.Connect(nodes[i].ID(), nodes[i-1].ID())
			if err != nil {
				return nil, fmt.Errorf("error connecting nodes: %v", err)
			}
		}
		clients[i], err = nodes[i].Client()
		if err != nil {
			return nil, fmt.Errorf("create node 1 rpc client fail: %v", err)
		}
	}
	if numnodes > 2 {
		err = net.Connect(nodes[0].ID(), nodes[len(nodes)-1].ID())
		if err != nil {
			return nil, fmt.Errorf("error connecting first and last nodes")
		}
	}
	return clients, nil
}

func newServices() adapters.Services {
	stateStore := state.NewInmemoryStore()
	kademlias := make(map[enode.ID]*network.Kademlia)
	kademlia := func(id enode.ID) *network.Kademlia {
		if k, ok := kademlias[id]; ok {
			return k
		}
		params := network.NewKadParams()
		params.NeighbourhoodSize = 2
		params.MaxBinSize = 3
		params.MinBinSize = 1
		params.MaxRetries = 1000
		params.RetryExponent = 2
		params.RetryInterval = 1000000
		kademlias[id] = network.NewKademlia(id[:], params)
		return kademlias[id]
	}
	return adapters.Services{
		"pss": func(ctx *adapters.ServiceContext) (node.Service, error) {
			ctxlocal, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			keys, err := wapi.NewKeyPair(ctxlocal)
			if err != nil {
				return nil, err
			}
			privkey, err := w.GetPrivateKey(keys)
			if err != nil {
				return nil, err
			}
			psparams := pss.NewPssParams().WithPrivateKey(privkey)
			pskad := kademlia(ctx.Config.ID)
			ps, err := pss.NewPss(pskad, psparams)
			if err != nil {
				return nil, err
			}
			pshparams := pss.NewHandshakeParams()
			pshparams.SymKeySendLimit = sendLimit
			err = pss.SetHandshakeController(ps, pshparams)
			if err != nil {
				return nil, fmt.Errorf("handshake controller fail: %v", err)
			}
			return ps, nil
		},
		"bzz": func(ctx *adapters.ServiceContext) (node.Service, error) {
			addr := network.NewAddr(ctx.Config.Node())
			hp := network.NewHiveParams()
			hp.Discovery = false
			config := &network.BzzConfig{
				OverlayAddr:  addr.Over(),
				UnderlayAddr: addr.Under(),
				HiveParams:   hp,
			}
			return network.NewBzz(config, kademlia(ctx.Config.ID), stateStore, nil, nil), nil
		},
	}
}

//从swarm/network/protocol\u test\u go复制
type testStore struct {
	sync.Mutex

	values map[string][]byte
}

func (t *testStore) Load(key string) ([]byte, error) {
	return nil, nil
}

func (t *testStore) Save(key string, v []byte) error {
	return nil
}
