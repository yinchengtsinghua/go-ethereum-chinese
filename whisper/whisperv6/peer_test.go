
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

package whisperv6

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	mrand "math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"net"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/rlp"
)

var keys = []string{
	"d49dcf37238dc8a7aac57dc61b9fee68f0a97f062968978b9fafa7d1033d03a9",
	"73fd6143c48e80ed3c56ea159fe7494a0b6b393a392227b422f4c3e8f1b54f98",
	"119dd32adb1daa7a4c7bf77f847fb28730785aa92947edf42fdd997b54de40dc",
	"deeda8709dea935bb772248a3144dea449ffcc13e8e5a1fd4ef20ce4e9c87837",
	"5bd208a079633befa349441bdfdc4d85ba9bd56081525008380a63ac38a407cf",
	"1d27fb4912002d58a2a42a50c97edb05c1b3dffc665dbaa42df1fe8d3d95c9b5",
	"15def52800c9d6b8ca6f3066b7767a76afc7b611786c1276165fbc61636afb68",
	"51be6ab4b2dc89f251ff2ace10f3c1cc65d6855f3e083f91f6ff8efdfd28b48c",
	"ef1ef7441bf3c6419b162f05da6037474664f198b58db7315a6f4de52414b4a0",
	"09bdf6985aabc696dc1fbeb5381aebd7a6421727343872eb2fadfc6d82486fd9",
	"15d811bf2e01f99a224cdc91d0cf76cea08e8c67905c16fee9725c9be71185c4",
	"2f83e45cf1baaea779789f755b7da72d8857aeebff19362dd9af31d3c9d14620",
	"73f04e34ac6532b19c2aae8f8e52f38df1ac8f5cd10369f92325b9b0494b0590",
	"1e2e07b69e5025537fb73770f483dc8d64f84ae3403775ef61cd36e3faf162c1",
	"8963d9bbb3911aac6d30388c786756b1c423c4fbbc95d1f96ddbddf39809e43a",
	"0422da85abc48249270b45d8de38a4cc3c02032ede1fcf0864a51092d58a2f1f",
	"8ae5c15b0e8c7cade201fdc149831aa9b11ff626a7ffd27188886cc108ad0fa8",
	"acd8f5a71d4aecfcb9ad00d32aa4bcf2a602939b6a9dd071bab443154184f805",
	"a285a922125a7481600782ad69debfbcdb0316c1e97c267aff29ef50001ec045",
	"28fd4eee78c6cd4bf78f39f8ab30c32c67c24a6223baa40e6f9c9a0e1de7cef5",
	"c5cca0c9e6f043b288c6f1aef448ab59132dab3e453671af5d0752961f013fc7",
	"46df99b051838cb6f8d1b73f232af516886bd8c4d0ee07af9a0a033c391380fd",
	"c6a06a53cbaadbb432884f36155c8f3244e244881b5ee3e92e974cfa166d793f",
	"783b90c75c63dc72e2f8d11b6f1b4de54d63825330ec76ee8db34f06b38ea211",
	"9450038f10ca2c097a8013e5121b36b422b95b04892232f930a29292d9935611",
	"e215e6246ed1cfdcf7310d4d8cdbe370f0d6a8371e4eb1089e2ae05c0e1bc10f",
	"487110939ed9d64ebbc1f300adeab358bc58875faf4ca64990fbd7fe03b78f2b",
	"824a70ea76ac81366da1d4f4ac39de851c8ac49dca456bb3f0a186ceefa269a5",
	"ba8f34fa40945560d1006a328fe70c42e35cc3d1017e72d26864cd0d1b150f15",
	"30a5dfcfd144997f428901ea88a43c8d176b19c79dde54cc58eea001aa3d246c",
	"de59f7183aca39aa245ce66a05245fecfc7e2c75884184b52b27734a4a58efa2",
	"92629e2ff5f0cb4f5f08fffe0f64492024d36f045b901efb271674b801095c5a",
	"7184c1701569e3a4c4d2ddce691edd983b81e42e09196d332e1ae2f1e062cff4",
}

type TestData struct {
	started int64
	counter [NumNodes]int
	mutex   sync.RWMutex
}

type TestNode struct {
	shh     *Whisper
	id      *ecdsa.PrivateKey
	server  *p2p.Server
	filerID string
}

const NumNodes = 8 //不得超过键数（32）

var result TestData
var nodes [NumNodes]*TestNode
var sharedKey = hexutil.MustDecode("0x03ca634cae0d49acb401d8a4c6b6fe8c55b70d115bf400769cc1400f3258cd31")
var wrongKey = hexutil.MustDecode("0xf91156714d7ec88d3edc1c652c2181dbb3044e8771c683f3b30d33c12b986b11")
var sharedTopic = TopicType{0xF, 0x1, 0x2, 0}
var wrongTopic = TopicType{0, 0, 0, 0}
var expectedMessage = []byte("per aspera ad astra")
var unexpectedMessage = []byte("per rectum ad astra")
var masterBloomFilter []byte
var masterPow = 0.00000001
var round = 1
var debugMode = false
var prevTime time.Time
var cntPrev int

func TestSimulation(t *testing.T) {
//创建一系列耳语节点，
//使用共享（预定义）参数安装筛选器
	initialize(t)

//每个节点发送一条随机（不可解密）消息
	for i := 0; i < NumNodes; i++ {
		sendMsg(t, false, i)
	}

//节点0发送一条预期（可解密）消息
	sendMsg(t, true, 0)

//检查每个节点是否只接收和解密了一条消息
	checkPropagation(t, true)

//检查状态消息是否正确解码
	checkBloomFilterExchange(t)
	checkPowExchange(t)

//发送新的Pow和Bloom交换消息
	resetParams(t)

//节点1发送一条预期（可解密）消息
	sendMsg(t, true, 1)

//检查每个节点（节点0除外）是否接收并解密了一条消息
	checkPropagation(t, false)

//检查相应的协议级消息是否正确解码
	checkPowExchangeForNodeZero(t)
	checkBloomFilterExchange(t)

	stopServers()
}

func resetParams(t *testing.T) {
//仅为节点零更改POW
	masterPow = 7777777.0
	nodes[0].shh.SetMinimumPoW(masterPow)

//为所有节点更改Bloom
	masterBloomFilter = TopicToBloom(sharedTopic)
	for i := 0; i < NumNodes; i++ {
		nodes[i].shh.SetBloomFilter(masterBloomFilter)
	}

	round++
}

func initBloom(t *testing.T) {
	masterBloomFilter = make([]byte, BloomFilterSize)
	_, err := mrand.Read(masterBloomFilter)
	if err != nil {
		t.Fatalf("rand failed: %s.", err)
	}

	msgBloom := TopicToBloom(sharedTopic)
	masterBloomFilter = addBloom(masterBloomFilter, msgBloom)
	for i := 0; i < 32; i++ {
		masterBloomFilter[i] = 0xFF
	}

	if !BloomFilterMatch(masterBloomFilter, msgBloom) {
		t.Fatalf("bloom mismatch on initBloom.")
	}
}

func initialize(t *testing.T) {
	initBloom(t)

	var err error

	for i := 0; i < NumNodes; i++ {
		var node TestNode
		b := make([]byte, BloomFilterSize)
		copy(b, masterBloomFilter)
		node.shh = New(&DefaultConfig)
		node.shh.SetMinimumPoW(masterPow)
		node.shh.SetBloomFilter(b)
		if !bytes.Equal(node.shh.BloomFilter(), masterBloomFilter) {
			t.Fatalf("bloom mismatch on init.")
		}
		node.shh.Start(nil)
		topics := make([]TopicType, 0)
		topics = append(topics, sharedTopic)
		f := Filter{KeySym: sharedKey}
		f.Topics = [][]byte{topics[0][:]}
		node.filerID, err = node.shh.Subscribe(&f)
		if err != nil {
			t.Fatalf("failed to install the filter: %s.", err)
		}
		node.id, err = crypto.HexToECDSA(keys[i])
		if err != nil {
			t.Fatalf("failed convert the key: %s.", keys[i])
		}
		name := common.MakeName("whisper-go", "2.0")

		node.server = &p2p.Server{
			Config: p2p.Config{
				PrivateKey: node.id,
				MaxPeers:   NumNodes/2 + 1,
				Name:       name,
				Protocols:  node.shh.Protocols(),
				ListenAddr: "127.0.0.1:0",
				NAT:        nat.Any(),
			},
		}

		go startServer(t, node.server)

		nodes[i] = &node
	}

	waitForServersToStart(t)

	for i := 0; i < NumNodes; i++ {
		for j := 0; j < i; j++ {
			peerNodeId := nodes[j].id
			address, _ := net.ResolveTCPAddr("tcp", nodes[j].server.ListenAddr)
			peer := enode.NewV4(&peerNodeId.PublicKey, address.IP, address.Port, address.Port)
			nodes[i].server.AddPeer(peer)
		}
	}
}

func startServer(t *testing.T, s *p2p.Server) {
	err := s.Start()
	if err != nil {
		t.Fatalf("failed to start the first server. err: %v", err)
	}

	atomic.AddInt64(&result.started, 1)
}

func stopServers() {
	for i := 0; i < NumNodes; i++ {
		n := nodes[i]
		if n != nil {
			n.shh.Unsubscribe(n.filerID)
			n.shh.Stop()
			n.server.Stop()
		}
	}
}

func checkPropagation(t *testing.T, includingNodeZero bool) {
	if t.Failed() {
		return
	}

	prevTime = time.Now()
//（循环*迭代次数）不应超过50秒，因为ttl=50
const cycle = 200 //时间（毫秒）
	const iterations = 250

	first := 0
	if !includingNodeZero {
		first = 1
	}

	for j := 0; j < iterations; j++ {
		for i := first; i < NumNodes; i++ {
			f := nodes[i].shh.GetFilter(nodes[i].filerID)
			if f == nil {
				t.Fatalf("failed to get filterId %s from node %d, round %d.", nodes[i].filerID, i, round)
			}

			mail := f.Retrieve()
			validateMail(t, i, mail)

			if isTestComplete() {
				checkTestStatus()
				return
			}
		}

		checkTestStatus()
		time.Sleep(cycle * time.Millisecond)
	}

	if !includingNodeZero {
		f := nodes[0].shh.GetFilter(nodes[0].filerID)
		if f != nil {
			t.Fatalf("node zero received a message with low PoW.")
		}
	}

	t.Fatalf("Test was not complete (%d round): timeout %d seconds. nodes=%v", round, iterations*cycle/1000, nodes)
}

func validateMail(t *testing.T, index int, mail []*ReceivedMessage) {
	var cnt int
	for _, m := range mail {
		if bytes.Equal(m.Payload, expectedMessage) {
			cnt++
		}
	}

	if cnt == 0 {
//还没有收到消息：没什么问题
		return
	}
	if cnt > 1 {
		t.Fatalf("node %d received %d.", index, cnt)
	}

	if cnt == 1 {
		result.mutex.Lock()
		defer result.mutex.Unlock()
		result.counter[index] += cnt
		if result.counter[index] > 1 {
			t.Fatalf("node %d accumulated %d.", index, result.counter[index])
		}
	}
}

func checkTestStatus() {
	var cnt int
	var arr [NumNodes]int

	for i := 0; i < NumNodes; i++ {
		arr[i] = nodes[i].server.PeerCount()
		envelopes := nodes[i].shh.Envelopes()
		if len(envelopes) >= NumNodes {
			cnt++
		}
	}

	if debugMode {
		if cntPrev != cnt {
			fmt.Printf(" %v \t number of nodes that have received all msgs: %d, number of peers per node: %v \n",
				time.Since(prevTime), cnt, arr)
			prevTime = time.Now()
			cntPrev = cnt
		}
	}
}

func isTestComplete() bool {
	result.mutex.RLock()
	defer result.mutex.RUnlock()

	for i := 0; i < NumNodes; i++ {
		if result.counter[i] < 1 {
			return false
		}
	}

	for i := 0; i < NumNodes; i++ {
		envelopes := nodes[i].shh.Envelopes()
		if len(envelopes) < NumNodes+1 {
			return false
		}
	}

	return true
}

func sendMsg(t *testing.T, expected bool, id int) {
	if t.Failed() {
		return
	}

	opt := MessageParams{KeySym: sharedKey, Topic: sharedTopic, Payload: expectedMessage, PoW: 0.00000001, WorkTime: 1}
	if !expected {
		opt.KeySym = wrongKey
		opt.Topic = wrongTopic
		opt.Payload = unexpectedMessage
		opt.Payload[0] = byte(id)
	}

	msg, err := NewSentMessage(&opt)
	if err != nil {
		t.Fatalf("failed to create new message with seed %d: %s.", seed, err)
	}
	envelope, err := msg.Wrap(&opt)
	if err != nil {
		t.Fatalf("failed to seal message: %s", err)
	}

	err = nodes[id].shh.Send(envelope)
	if err != nil {
		t.Fatalf("failed to send message: %s", err)
	}
}

func TestPeerBasic(t *testing.T) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		t.Fatalf("failed generateMessageParams with seed %d.", seed)
	}

	params.PoW = 0.001
	msg, err := NewSentMessage(params)
	if err != nil {
		t.Fatalf("failed to create new message with seed %d: %s.", seed, err)
	}
	env, err := msg.Wrap(params)
	if err != nil {
		t.Fatalf("failed Wrap with seed %d.", seed)
	}

	p := newPeer(nil, nil, nil)
	p.mark(env)
	if !p.marked(env) {
		t.Fatalf("failed mark with seed %d.", seed)
	}
}

func checkPowExchangeForNodeZero(t *testing.T) {
	const iterations = 200
	for j := 0; j < iterations; j++ {
		lastCycle := (j == iterations-1)
		ok := checkPowExchangeForNodeZeroOnce(t, lastCycle)
		if ok {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func checkPowExchangeForNodeZeroOnce(t *testing.T, mustPass bool) bool {
	cnt := 0
	for i, node := range nodes {
		for peer := range node.shh.peers {
			if peer.peer.ID() == nodes[0].server.Self().ID() {
				cnt++
				if peer.powRequirement != masterPow {
					if mustPass {
						t.Fatalf("node %d: failed to set the new pow requirement for node zero.", i)
					} else {
						return false
					}
				}
			}
		}
	}
	if cnt == 0 {
		t.Fatalf("looking for node zero: no matching peers found.")
	}
	return true
}

func checkPowExchange(t *testing.T) {
	for i, node := range nodes {
		for peer := range node.shh.peers {
			if peer.peer.ID() != nodes[0].server.Self().ID() {
				if peer.powRequirement != masterPow {
					t.Fatalf("node %d: failed to exchange pow requirement in round %d; expected %f, got %f",
						i, round, masterPow, peer.powRequirement)
				}
			}
		}
	}
}

func checkBloomFilterExchangeOnce(t *testing.T, mustPass bool) bool {
	for i, node := range nodes {
		for peer := range node.shh.peers {
			peer.bloomMu.Lock()
			equals := bytes.Equal(peer.bloomFilter, masterBloomFilter)
			peer.bloomMu.Unlock()
			if !equals {
				if mustPass {
					t.Fatalf("node %d: failed to exchange bloom filter requirement in round %d. \n%x expected \n%x got",
						i, round, masterBloomFilter, peer.bloomFilter)
				} else {
					return false
				}
			}
		}
	}

	return true
}

func checkBloomFilterExchange(t *testing.T) {
	const iterations = 200
	for j := 0; j < iterations; j++ {
		lastCycle := (j == iterations-1)
		ok := checkBloomFilterExchangeOnce(t, lastCycle)
		if ok {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func waitForServersToStart(t *testing.T) {
	const iterations = 200
	var started int64
	for j := 0; j < iterations; j++ {
		time.Sleep(50 * time.Millisecond)
		started = atomic.LoadInt64(&result.started)
		if started == NumNodes {
			return
		}
	}
	t.Fatalf("Failed to start all the servers, running: %d", started)
}

//两个普通的耳语节点握手
func TestPeerHandshakeWithTwoFullNode(t *testing.T) {
	w1 := Whisper{}
	p1 := newPeer(&w1, p2p.NewPeer(enode.ID{}, "test", []p2p.Cap{}), &rwStub{[]interface{}{ProtocolVersion, uint64(123), make([]byte, BloomFilterSize), false}})
	err := p1.handshake()
	if err != nil {
		t.Fatal()
	}
}

//两个普通的耳语节点握手。一个不发轻旗
func TestHandshakeWithOldVersionWithoutLightModeFlag(t *testing.T) {
	w1 := Whisper{}
	p1 := newPeer(&w1, p2p.NewPeer(enode.ID{}, "test", []p2p.Cap{}), &rwStub{[]interface{}{ProtocolVersion, uint64(123), make([]byte, BloomFilterSize)}})
	err := p1.handshake()
	if err != nil {
		t.Fatal()
	}
}

//两个光节点握手。限制已禁用
func TestTwoLightPeerHandshakeRestrictionOff(t *testing.T) {
	w1 := Whisper{}
	w1.settings.Store(restrictConnectionBetweenLightClientsIdx, false)
	w1.SetLightClientMode(true)
	p1 := newPeer(&w1, p2p.NewPeer(enode.ID{}, "test", []p2p.Cap{}), &rwStub{[]interface{}{ProtocolVersion, uint64(123), make([]byte, BloomFilterSize), true}})
	err := p1.handshake()
	if err != nil {
		t.FailNow()
	}
}

//两个光节点握手。限制已启用
func TestTwoLightPeerHandshakeError(t *testing.T) {
	w1 := Whisper{}
	w1.settings.Store(restrictConnectionBetweenLightClientsIdx, true)
	w1.SetLightClientMode(true)
	p1 := newPeer(&w1, p2p.NewPeer(enode.ID{}, "test", []p2p.Cap{}), &rwStub{[]interface{}{ProtocolVersion, uint64(123), make([]byte, BloomFilterSize), true}})
	err := p1.handshake()
	if err == nil {
		t.FailNow()
	}
}

type rwStub struct {
	payload []interface{}
}

func (stub *rwStub) ReadMsg() (p2p.Msg, error) {
	size, r, err := rlp.EncodeToReader(stub.payload)
	if err != nil {
		return p2p.Msg{}, err
	}
	return p2p.Msg{Code: statusCode, Size: uint32(size), Payload: r}, nil
}

func (stub *rwStub) WriteMsg(m p2p.Msg) error {
	return nil
}
