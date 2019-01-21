
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package notify

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/pss"
	"github.com/ethereum/go-ethereum/swarm/state"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv5"
)

var (
	loglevel = flag.Int("l", 3, "loglevel")
	psses    map[string]*pss.Pss
	w        *whisper.Whisper
	wapi     *whisper.PublicWhisperAPI
)

func init() {
	flag.Parse()
	hs := log.StreamHandler(os.Stderr, log.TerminalFormat(true))
	hf := log.LvlFilterHandler(log.Lvl(*loglevel), hs)
	h := log.CallerFileHandler(hf)
	log.Root().SetHandler(h)

	w = whisper.New(&whisper.DefaultConfig)
	wapi = whisper.NewPublicWhisperAPI(w)
	psses = make(map[string]*pss.Pss)
}

//创建客户机节点和通知程序节点
//客户端发送PSS通知请求
//通知程序使用对称密钥发送初始通知，以及
//第二个通知对称加密
func TestStart(t *testing.T) {
	adapter := adapters.NewSimAdapter(newServices(false))
	net := simulations.NewNetwork(adapter, &simulations.NetworkConfig{
		ID:             "0",
		DefaultService: "bzz",
	})
	leftNodeConf := adapters.RandomNodeConfig()
	leftNodeConf.Services = []string{"bzz", "pss"}
	leftNode, err := net.NewNodeWithConfig(leftNodeConf)
	if err != nil {
		t.Fatal(err)
	}
	err = net.Start(leftNode.ID())
	if err != nil {
		t.Fatal(err)
	}

	rightNodeConf := adapters.RandomNodeConfig()
	rightNodeConf.Services = []string{"bzz", "pss"}
	rightNode, err := net.NewNodeWithConfig(rightNodeConf)
	if err != nil {
		t.Fatal(err)
	}
	err = net.Start(rightNode.ID())
	if err != nil {
		t.Fatal(err)
	}

	err = net.Connect(rightNode.ID(), leftNode.ID())
	if err != nil {
		t.Fatal(err)
	}

	leftRpc, err := leftNode.Client()
	if err != nil {
		t.Fatal(err)
	}

	rightRpc, err := rightNode.Client()
	if err != nil {
		t.Fatal(err)
	}

	var leftAddr string
	err = leftRpc.Call(&leftAddr, "pss_baseAddr")
	if err != nil {
		t.Fatal(err)
	}

	var rightAddr string
	err = rightRpc.Call(&rightAddr, "pss_baseAddr")
	if err != nil {
		t.Fatal(err)
	}

	var leftPub string
	err = leftRpc.Call(&leftPub, "pss_getPublicKey")
	if err != nil {
		t.Fatal(err)
	}

	var rightPub string
	err = rightRpc.Call(&rightPub, "pss_getPublicKey")
	if err != nil {
		t.Fatal(err)
	}

	rsrcName := "foo.eth"
	rsrcTopic := pss.BytesToTopic([]byte(rsrcName))

//等待Kademlia表填充
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	rmsgC := make(chan *pss.APIMsg)
	rightSub, err := rightRpc.Subscribe(ctx, "pss", rmsgC, "receive", controlTopic, false, false)
	if err != nil {
		t.Fatal(err)
	}
	defer rightSub.Unsubscribe()

	updateC := make(chan []byte)
	updateMsg := []byte{}
	ctrlClient := NewController(psses[rightPub])
	ctrlNotifier := NewController(psses[leftPub])
	ctrlNotifier.NewNotifier("foo.eth", 2, updateC)

	pubkeybytes, err := hexutil.Decode(leftPub)
	if err != nil {
		t.Fatal(err)
	}
	pubkey, err := crypto.UnmarshalPubkey(pubkeybytes)
	if err != nil {
		t.Fatal(err)
	}
	addrbytes, err := hexutil.Decode(leftAddr)
	if err != nil {
		t.Fatal(err)
	}
	ctrlClient.Subscribe(rsrcName, pubkey, addrbytes, func(s string, b []byte) error {
		if s != "foo.eth" || !bytes.Equal(updateMsg, b) {
			t.Fatalf("unexpected result in client handler: '%s':'%x'", s, b)
		}
		log.Info("client handler receive", "s", s, "b", b)
		return nil
	})

	var inMsg *pss.APIMsg
	select {
	case inMsg = <-rmsgC:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	dMsg, err := NewMsgFromPayload(inMsg.Msg)
	if err != nil {
		t.Fatal(err)
	}
	if dMsg.namestring != rsrcName {
		t.Fatalf("expected name '%s', got '%s'", rsrcName, dMsg.namestring)
	}
	if !bytes.Equal(dMsg.Payload[:len(updateMsg)], updateMsg) {
		t.Fatalf("expected payload first %d bytes '%x', got '%x'", len(updateMsg), updateMsg, dMsg.Payload[:len(updateMsg)])
	}
	if len(updateMsg)+symKeyLength != len(dMsg.Payload) {
		t.Fatalf("expected payload length %d, have %d", len(updateMsg)+symKeyLength, len(dMsg.Payload))
	}

	rightSubUpdate, err := rightRpc.Subscribe(ctx, "pss", rmsgC, "receive", rsrcTopic, false, false)
	if err != nil {
		t.Fatal(err)
	}
	defer rightSubUpdate.Unsubscribe()

	updateMsg = []byte("plugh")
	updateC <- updateMsg
	select {
	case inMsg = <-rmsgC:
	case <-ctx.Done():
		log.Error("timed out waiting for msg", "topic", fmt.Sprintf("%x", rsrcTopic))
		t.Fatal(ctx.Err())
	}
	dMsg, err = NewMsgFromPayload(inMsg.Msg)
	if err != nil {
		t.Fatal(err)
	}
	if dMsg.namestring != rsrcName {
		t.Fatalf("expected name %s, got %s", rsrcName, dMsg.namestring)
	}
	if !bytes.Equal(dMsg.Payload, updateMsg) {
		t.Fatalf("expected payload '%x', got '%x'", updateMsg, dMsg.Payload)
	}

}

func newServices(allowRaw bool) adapters.Services {
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
			pssp := pss.NewPssParams().WithPrivateKey(privkey)
			pssp.MsgTTL = time.Second * 30
			pssp.AllowRaw = allowRaw
			pskad := kademlia(ctx.Config.ID)
			ps, err := pss.NewPss(pskad, pssp)
			if err != nil {
				return nil, err
			}
//psses[common.tohex（crypto.fromecdsapub（&privkey.publickey））]=ps
			psses[hexutil.Encode(crypto.FromECDSAPub(&privkey.PublicKey))] = ps
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
