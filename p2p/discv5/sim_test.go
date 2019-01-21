
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

package discv5

import (
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

//在这个测试中，节点试图随机地解析彼此。
func TestSimRandomResolve(t *testing.T) {
	t.Skip("boring")
	if runWithPlaygroundTime(t) {
		return
	}

	sim := newSimulation()
	bootnode := sim.launchNode(false)

//新节点每10秒联接一次。
	launcher := time.NewTicker(10 * time.Second)
	go func() {
		for range launcher.C {
			net := sim.launchNode(false)
			go randomResolves(t, sim, net)
			if err := net.SetFallbackNodes([]*Node{bootnode.Self()}); err != nil {
				panic(err)
			}
			fmt.Printf("launched @ %v: %x\n", time.Now(), net.Self().ID[:16])
		}
	}()

	time.Sleep(3 * time.Hour)
	launcher.Stop()
	sim.shutdown()
	sim.printStats()
}

func TestSimTopics(t *testing.T) {
	t.Skip("NaCl test")
	if runWithPlaygroundTime(t) {
		return
	}
	sim := newSimulation()
	bootnode := sim.launchNode(false)

	go func() {
		nets := make([]*Network, 1024)
		for i := range nets {
			net := sim.launchNode(false)
			nets[i] = net
			if err := net.SetFallbackNodes([]*Node{bootnode.Self()}); err != nil {
				panic(err)
			}
			time.Sleep(time.Second * 5)
		}

		for i, net := range nets {
			if i < 256 {
				stop := make(chan struct{})
				go net.RegisterTopic(testTopic, stop)
				go func() {
//时间.睡眠（time.second*36000）
					time.Sleep(time.Second * 40000)
					close(stop)
				}()
				time.Sleep(time.Millisecond * 100)
			}
//时间.睡眠（time.second*10）
//时间.睡眠（时间.秒）
   /*F I%500==499
    时间.睡眠（time.second*9501）
   }否则{
    时间.睡眠（时间.秒）
   */

		}
	}()

//新节点每10秒联接一次。
 /*启动程序：=time.newticker（5*time.second）
  CNT：＝0
  var printnet*网络
  转到函数（）
   用于远程发射器.c
    碳纳米管+
    如果cnt<=1000
     日志：=false/（cnt==500）
     网络：=sim.launchnode（日志）
     如果log {
      打印网
     }
     如果CNT＞500 {
      go net.registerTopic（测试主题，无）
     }
     如果错误：=net.setFallbackNodes（[]*node bootnode.self（））；错误！= nIL{
      惊慌（错误）
     }
    }
    //fmt.printf（“已启动@%v:%x\n”，time.now（），net.self（）.id[：16]）
   }
  }（）
 **/

	time.Sleep(55000 * time.Second)
//发射器
	sim.shutdown()
//sim.printstats（）。
//printnet.log.printlogs（）。
}

/*unc testHierarchicalTopics（i int）[]主题
 数字：=strconv.formatint（Int64（256+I/4），4）
 回复：=制造（[]主题，5）
 对于i，：=范围分辨率
  res[i]=主题（“foo”+数字[1:i+1]）
 }
 收益率
*/


func testHierarchicalTopics(i int) []Topic {
	digits := strconv.FormatInt(int64(128+i/8), 2)
	res := make([]Topic, 8)
	for i := range res {
		res[i] = Topic("foo" + digits[1:i+1])
	}
	return res
}

func TestSimTopicHierarchy(t *testing.T) {
	t.Skip("NaCl test")
	if runWithPlaygroundTime(t) {
		return
	}
	sim := newSimulation()
	bootnode := sim.launchNode(false)

	go func() {
		nets := make([]*Network, 1024)
		for i := range nets {
			net := sim.launchNode(false)
			nets[i] = net
			if err := net.SetFallbackNodes([]*Node{bootnode.Self()}); err != nil {
				panic(err)
			}
			time.Sleep(time.Second * 5)
		}

		stop := make(chan struct{})
		for i, net := range nets {
//如果i＜256 {
			for _, topic := range testHierarchicalTopics(i)[:5] {
//fmt.println（“注册”，主题）
				go net.RegisterTopic(topic, stop)
			}
			time.Sleep(time.Millisecond * 100)
//}
		}
		time.Sleep(time.Second * 90000)
		close(stop)
	}()

	time.Sleep(100000 * time.Second)
	sim.shutdown()
}

func randomResolves(t *testing.T, s *simulation, net *Network) {
	randtime := func() time.Duration {
		return time.Duration(rand.Intn(50)+20) * time.Second
	}
	lookup := func(target NodeID) bool {
		result := net.Resolve(target)
		return result != nil && result.ID == target
	}

	timer := time.NewTimer(randtime())
	for {
		select {
		case <-timer.C:
			target := s.randomNode().Self().ID
			if !lookup(target) {
				t.Errorf("node %x: target %x not found", net.Self().ID[:8], target[:8])
			}
			timer.Reset(randtime())
		case <-net.closed:
			return
		}
	}
}

type simulation struct {
	mu      sync.RWMutex
	nodes   map[NodeID]*Network
	nodectr uint32
}

func newSimulation() *simulation {
	return &simulation{nodes: make(map[NodeID]*Network)}
}

func (s *simulation) shutdown() {
	s.mu.RLock()
	alive := make([]*Network, 0, len(s.nodes))
	for _, n := range s.nodes {
		alive = append(alive, n)
	}
	defer s.mu.RUnlock()

	for _, n := range alive {
		n.Close()
	}
}

func (s *simulation) printStats() {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Println("node counter:", s.nodectr)
	fmt.Println("alive nodes:", len(s.nodes))

//对于u，n：=范围s.nodes
//fmt.printf（“%x\n”，n.tab.self.id[：8]）
//传输：=N.conn.（*SimTransport）
//fmt.println（“已加入：”，transport.jointime）
//fmt.println（“发送：”，transport.hashctr）
//fmt.println（“表格大小：”，n.tab.count）
//}

 /*或u，n：=范围s.nodes
  打印文件（）
  fmt.printf（“***节点%x\n”，n.tab.self.id[：8]）
  n.log.printlogs（）。
 */


}

func (s *simulation) randomNode() *Network {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := rand.Intn(len(s.nodes))
	for _, net := range s.nodes {
		if n == 0 {
			return net
		}
		n--
	}
	return nil
}

func (s *simulation) launchNode(log bool) *Network {
	var (
		num = s.nodectr
		key = newkey()
		id  = PubkeyID(&key.PublicKey)
		ip  = make(net.IP, 4)
	)
	s.nodectr++
	binary.BigEndian.PutUint32(ip, num)
	ip[0] = 10
	addr := &net.UDPAddr{IP: ip, Port: 30303}

	transport := &simTransport{joinTime: time.Now(), sender: id, senderAddr: addr, sim: s, priv: key}
	net, err := newNetwork(transport, key.PublicKey, "<no database>", nil)
	if err != nil {
		panic("cannot launch new node: " + err.Error())
	}

	s.mu.Lock()
	s.nodes[id] = net
	s.mu.Unlock()

	return net
}

func (s *simulation) dropNode(id NodeID) {
	s.mu.Lock()
	n := s.nodes[id]
	delete(s.nodes, id)
	s.mu.Unlock()

	n.Close()
}

type simTransport struct {
	joinTime   time.Time
	sender     NodeID
	senderAddr *net.UDPAddr
	sim        *simulation
	hashctr    uint64
	priv       *ecdsa.PrivateKey
}

func (st *simTransport) localAddr() *net.UDPAddr {
	return st.senderAddr
}

func (st *simTransport) Close() {}

func (st *simTransport) send(remote *Node, ptype nodeEvent, data interface{}) (hash []byte) {
	hash = st.nextHash()
	var raw []byte
	if ptype == pongPacket {
		var err error
		raw, _, err = encodePacket(st.priv, byte(ptype), data)
		if err != nil {
			panic(err)
		}
	}

	st.sendPacket(remote.ID, ingressPacket{
		remoteID:   st.sender,
		remoteAddr: st.senderAddr,
		hash:       hash,
		ev:         ptype,
		data:       data,
		rawData:    raw,
	})
	return hash
}

func (st *simTransport) sendPing(remote *Node, remoteAddr *net.UDPAddr, topics []Topic) []byte {
	hash := st.nextHash()
	st.sendPacket(remote.ID, ingressPacket{
		remoteID:   st.sender,
		remoteAddr: st.senderAddr,
		hash:       hash,
		ev:         pingPacket,
		data: &ping{
			Version:    4,
			From:       rpcEndpoint{IP: st.senderAddr.IP, UDP: uint16(st.senderAddr.Port), TCP: 30303},
			To:         rpcEndpoint{IP: remoteAddr.IP, UDP: uint16(remoteAddr.Port), TCP: 30303},
			Expiration: uint64(time.Now().Unix() + int64(expiration)),
			Topics:     topics,
		},
	})
	return hash
}

func (st *simTransport) sendPong(remote *Node, pingHash []byte) {
	raddr := remote.addr()

	st.sendPacket(remote.ID, ingressPacket{
		remoteID:   st.sender,
		remoteAddr: st.senderAddr,
		hash:       st.nextHash(),
		ev:         pongPacket,
		data: &pong{
			To:         rpcEndpoint{IP: raddr.IP, UDP: uint16(raddr.Port), TCP: 30303},
			ReplyTok:   pingHash,
			Expiration: uint64(time.Now().Unix() + int64(expiration)),
		},
	})
}

func (st *simTransport) sendFindnodeHash(remote *Node, target common.Hash) {
	st.sendPacket(remote.ID, ingressPacket{
		remoteID:   st.sender,
		remoteAddr: st.senderAddr,
		hash:       st.nextHash(),
		ev:         findnodeHashPacket,
		data: &findnodeHash{
			Target:     target,
			Expiration: uint64(time.Now().Unix() + int64(expiration)),
		},
	})
}

func (st *simTransport) sendTopicRegister(remote *Node, topics []Topic, idx int, pong []byte) {
//fmt.println（“发送”，主题，pong）
	st.sendPacket(remote.ID, ingressPacket{
		remoteID:   st.sender,
		remoteAddr: st.senderAddr,
		hash:       st.nextHash(),
		ev:         topicRegisterPacket,
		data: &topicRegister{
			Topics: topics,
			Idx:    uint(idx),
			Pong:   pong,
		},
	})
}

func (st *simTransport) sendTopicNodes(remote *Node, queryHash common.Hash, nodes []*Node) {
	rnodes := make([]rpcNode, len(nodes))
	for i := range nodes {
		rnodes[i] = nodeToRPC(nodes[i])
	}
	st.sendPacket(remote.ID, ingressPacket{
		remoteID:   st.sender,
		remoteAddr: st.senderAddr,
		hash:       st.nextHash(),
		ev:         topicNodesPacket,
		data:       &topicNodes{Echo: queryHash, Nodes: rnodes},
	})
}

func (st *simTransport) sendNeighbours(remote *Node, nodes []*Node) {
//TODO:发送多个数据包
	rnodes := make([]rpcNode, len(nodes))
	for i := range nodes {
		rnodes[i] = nodeToRPC(nodes[i])
	}
	st.sendPacket(remote.ID, ingressPacket{
		remoteID:   st.sender,
		remoteAddr: st.senderAddr,
		hash:       st.nextHash(),
		ev:         neighborsPacket,
		data: &neighbors{
			Nodes:      rnodes,
			Expiration: uint64(time.Now().Unix() + int64(expiration)),
		},
	})
}

func (st *simTransport) nextHash() []byte {
	v := atomic.AddUint64(&st.hashctr, 1)
	var hash common.Hash
	binary.BigEndian.PutUint64(hash[:], v)
	return hash[:]
}

const packetLoss = 0 //1/1000

func (st *simTransport) sendPacket(remote NodeID, p ingressPacket) {
	if rand.Int31n(1000) >= packetLoss {
		st.sim.mu.RLock()
		recipient := st.sim.nodes[remote]
		st.sim.mu.RUnlock()

		time.AfterFunc(200*time.Millisecond, func() {
			recipient.reqReadPacket(p)
		})
	}
}
