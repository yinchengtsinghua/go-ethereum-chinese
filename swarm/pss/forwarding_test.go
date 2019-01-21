
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package pss

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/pot"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv5"
)

type testCase struct {
	name      string
	recipient []byte
	peers     []pot.Address
	expected  []int
	exclusive bool
	nFails    int
	success   bool
	errors    string
}

var testCases []testCase

//此测试的目的是查看pss.forward（）函数是否正确。
//根据邮件地址选择邮件转发的对等方
//和卡德米利亚星座。
func TestForwardBasic(t *testing.T) {
	baseAddrBytes := make([]byte, 32)
	for i := 0; i < len(baseAddrBytes); i++ {
		baseAddrBytes[i] = 0xFF
	}
	var c testCase
	base := pot.NewAddressFromBytes(baseAddrBytes)
	var peerAddresses []pot.Address
	const depth = 10
	for i := 0; i <= depth; i++ {
//为每个接近顺序添加两个对等点
		a := pot.RandomAddressAt(base, i)
		peerAddresses = append(peerAddresses, a)
		a = pot.RandomAddressAt(base, i)
		peerAddresses = append(peerAddresses, a)
	}

//跳过一个级别，在更深的一个级别添加一个对等。
//因此，我们将在最近的邻居的垃圾箱中发现一个三个同行的边缘案例。
	peerAddresses = append(peerAddresses, pot.RandomAddressAt(base, depth+2))

	kad := network.NewKademlia(base[:], network.NewKadParams())
	ps := createPss(t, kad)
	addPeers(kad, peerAddresses)

const firstNearest = depth * 2 //在最近邻居的垃圾桶里最浅的人
	nearestNeighbours := []int{firstNearest, firstNearest + 1, firstNearest + 2}
var all []int //所有同行的指数
	for i := 0; i < len(peerAddresses); i++ {
		all = append(all, i)
	}

	for i := 0; i < len(peerAddresses); i++ {
//直接将消息发送到已知的对等方（收件人地址==对等方地址）
		c = testCase{
			name:      fmt.Sprintf("Send direct to known, id: [%d]", i),
			recipient: peerAddresses[i][:],
			peers:     peerAddresses,
			expected:  []int{i},
			exclusive: false,
		}
		testCases = append(testCases, c)
	}

	for i := 0; i < firstNearest; i++ {
//随机发送带有邻近订单的消息，与每个垃圾箱的采购订单相对应，
//一个对等机更接近收件人地址
		a := pot.RandomAddressAt(peerAddresses[i], 64)
		c = testCase{
			name:      fmt.Sprintf("Send random to each PO, id: [%d]", i),
			recipient: a[:],
			peers:     peerAddresses,
			expected:  []int{i},
			exclusive: false,
		}
		testCases = append(testCases, c)
	}

	for i := 0; i < firstNearest; i++ {
//随机发送带有邻近订单的消息，与每个垃圾箱的采购订单相对应，
//相对于收件人地址随机接近
		po := i / 2
		a := pot.RandomAddressAt(base, po)
		c = testCase{
			name:      fmt.Sprintf("Send direct to known, id: [%d]", i),
			recipient: a[:],
			peers:     peerAddresses,
			expected:  []int{po * 2, po*2 + 1},
			exclusive: true,
		}
		testCases = append(testCases, c)
	}

	for i := firstNearest; i < len(peerAddresses); i++ {
//收信人地址落在最近邻居的箱子里
		a := pot.RandomAddressAt(base, i)
		c = testCase{
			name:      fmt.Sprintf("recipient address falls into the nearest neighbours' bin, id: [%d]", i),
			recipient: a[:],
			peers:     peerAddresses,
			expected:  nearestNeighbours,
			exclusive: false,
		}
		testCases = append(testCases, c)
	}

//发送带有邻近命令的消息比最深的最近邻居深得多
	a2 := pot.RandomAddressAt(base, 77)
	c = testCase{
		name:      "proximity order much deeper than the deepest nearest neighbour",
		recipient: a2[:],
		peers:     peerAddresses,
		expected:  nearestNeighbours,
		exclusive: false,
	}
	testCases = append(testCases, c)

//部分地址测试
	const part = 12

	for i := 0; i < firstNearest; i++ {
//发送部分地址属于不同邻近顺序的消息
		po := i / 2
		if i%8 != 0 {
			c = testCase{
				name:      fmt.Sprintf("partial address falling into different proximity orders, id: [%d]", i),
				recipient: peerAddresses[i][:i],
				peers:     peerAddresses,
				expected:  []int{po * 2, po*2 + 1},
				exclusive: true,
			}
			testCases = append(testCases, c)
		}
		c = testCase{
			name:      fmt.Sprintf("extended partial address falling into different proximity orders, id: [%d]", i),
			recipient: peerAddresses[i][:part],
			peers:     peerAddresses,
			expected:  []int{po * 2, po*2 + 1},
			exclusive: true,
		}
		testCases = append(testCases, c)
	}

	for i := firstNearest; i < len(peerAddresses); i++ {
//部分地址落在最近邻居的箱子里。
		c = testCase{
			name:      fmt.Sprintf("partial address falls into the nearest neighbours' bin, id: [%d]", i),
			recipient: peerAddresses[i][:part],
			peers:     peerAddresses,
			expected:  nearestNeighbours,
			exclusive: false,
		}
		testCases = append(testCases, c)
	}

//部分地址，邻近顺序比任何最近邻居都深
	a3 := pot.RandomAddressAt(base, part)
	c = testCase{
		name:      "partial address with proximity order deeper than any of the nearest neighbour",
		recipient: a3[:part],
		peers:     peerAddresses,
		expected:  nearestNeighbours,
		exclusive: false,
	}
	testCases = append(testCases, c)

//部分地址与大量对等点匹配的特殊情况

//地址为零字节，应将消息传递给所有对等方
	c = testCase{
		name:      "zero bytes of address is given",
		recipient: []byte{},
		peers:     peerAddresses,
		expected:  all,
		exclusive: false,
	}
	testCases = append(testCases, c)

//发光半径8位，接近顺序8
	indexAtPo8 := 16
	c = testCase{
		name:      "luminous radius of 8 bits",
		recipient: []byte{0xFF},
		peers:     peerAddresses,
		expected:  all[indexAtPo8:],
		exclusive: false,
	}
	testCases = append(testCases, c)

//发光半径256位，接近8级
	a4 := pot.Address{}
	a4[0] = 0xFF
	c = testCase{
		name:      "luminous radius of 256 bits",
		recipient: a4[:],
		peers:     peerAddresses,
		expected:  []int{indexAtPo8, indexAtPo8 + 1},
		exclusive: true,
	}
	testCases = append(testCases, c)

//如果发送失败，请检查行为是否正确
	for i := 2; i < firstNearest-3; i += 2 {
		po := i / 2
//随机发送带有邻近订单的消息，与每个垃圾箱的采购订单相对应，
//尝试失败的次数不同。
//只有一个较深的对等方应接收消息。
		a := pot.RandomAddressAt(base, po)
		c = testCase{
			name:      fmt.Sprintf("Send direct to known, id: [%d]", i),
			recipient: a[:],
			peers:     peerAddresses,
			expected:  all[i+1:],
			exclusive: true,
			nFails:    rand.Int()%3 + 2,
		}
		testCases = append(testCases, c)
	}

	for _, c := range testCases {
		testForwardMsg(t, ps, &c)
	}
}

//此功能测试单个邮件的转发。收件人地址作为参数传递，
//以及所有对等方的地址，以及期望接收消息的对等方的索引。
func testForwardMsg(t *testing.T, ps *Pss, c *testCase) {
	recipientAddr := c.recipient
	peers := c.peers
	expected := c.expected
	exclusive := c.exclusive
	nFails := c.nFails
tries := 0 //上次失败的尝试次数

	resultMap := make(map[pot.Address]int)

	defer func() { sendFunc = sendMsg }()
	sendFunc = func(_ *Pss, sp *network.Peer, _ *PssMsg) bool {
		if tries < nFails {
			tries++
			return false
		}
		a := pot.NewAddressFromBytes(sp.Address())
		resultMap[a]++
		return true
	}

	msg := newTestMsg(recipientAddr)
	ps.forward(msg)

//检查测试结果
	var fail bool
	precision := len(recipientAddr)
	if precision > 4 {
		precision = 4
	}
	s := fmt.Sprintf("test [%s]\nmsg address: %x..., radius: %d", c.name, recipientAddr[:precision], 8*len(recipientAddr))

//错误否定（预期消息未到达对等端）
	if exclusive {
		var cnt int
		for _, i := range expected {
			a := peers[i]
			cnt += resultMap[a]
			resultMap[a] = 0
		}
		if cnt != 1 {
			s += fmt.Sprintf("\n%d messages received by %d peers with indices: [%v]", cnt, len(expected), expected)
			fail = true
		}
	} else {
		for _, i := range expected {
			a := peers[i]
			received := resultMap[a]
			if received != 1 {
				s += fmt.Sprintf("\npeer number %d [%x...] received %d messages", i, a[:4], received)
				fail = true
			}
			resultMap[a] = 0
		}
	}

//误报（到达对等端的意外消息）
	for k, v := range resultMap {
		if v != 0 {
//查找假阳性对等体的索引
			var j int
			for j = 0; j < len(peers); j++ {
				if peers[j] == k {
					break
				}
			}
			s += fmt.Sprintf("\npeer number %d [%x...] received %d messages", j, k[:4], v)
			fail = true
		}
	}

	if fail {
		t.Fatal(s)
	}
}

func addPeers(kad *network.Kademlia, addresses []pot.Address) {
	for _, a := range addresses {
		p := newTestDiscoveryPeer(a, kad)
		kad.On(p)
	}
}

func createPss(t *testing.T, kad *network.Kademlia) *Pss {
	privKey, err := crypto.GenerateKey()
	pssp := NewPssParams().WithPrivateKey(privKey)
	ps, err := NewPss(kad, pssp)
	if err != nil {
		t.Fatal(err.Error())
	}
	return ps
}

func newTestDiscoveryPeer(addr pot.Address, kad *network.Kademlia) *network.Peer {
	rw := &p2p.MsgPipeRW{}
	p := p2p.NewPeer(enode.ID{}, "test", []p2p.Cap{})
	pp := protocols.NewPeer(p, rw, &protocols.Spec{})
	bp := &network.BzzPeer{
		Peer: pp,
		BzzAddr: &network.BzzAddr{
			OAddr: addr.Bytes(),
			UAddr: []byte(fmt.Sprintf("%x", addr[:])),
		},
	}
	return network.NewPeer(bp, kad)
}

func newTestMsg(addr []byte) *PssMsg {
	msg := newPssMsg(&msgParams{})
	msg.To = addr[:]
	msg.Expire = uint32(time.Now().Add(time.Second * 60).Unix())
	msg.Payload = &whisper.Envelope{
		Topic: [4]byte{},
		Data:  []byte("i have nothing to hide"),
	}
	return msg
}
