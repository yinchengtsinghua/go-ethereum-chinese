
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

package enode

import (
	"crypto/ecdsa"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/p2p/netutil"
)

const (
//IP跟踪器配置
	iptrackMinStatements = 10
	iptrackWindow        = 5 * time.Minute
	iptrackContactWindow = 10 * time.Minute
)

//local node生成本地节点的签名节点记录，即在
//当前进程。通过set方法设置enr条目将更新记录。新版本
//当调用node方法时，将根据需要对记录进行签名。
type LocalNode struct {
cur atomic.Value //当记录是最新的时，保存一个非零节点指针。
	id  ID
	key *ecdsa.PrivateKey
	db  *DB

//下面的一切都有锁保护
	mu          sync.Mutex
	seq         uint64
	entries     map[string]enr.Entry
udpTrack    *netutil.IPTracker //预测外部UDP终结点
	staticIP    net.IP
	fallbackIP  net.IP
	fallbackUDP int
}

//newlocalnode创建本地节点。
func NewLocalNode(db *DB, key *ecdsa.PrivateKey) *LocalNode {
	ln := &LocalNode{
		id:       PubkeyToIDV4(&key.PublicKey),
		db:       db,
		key:      key,
		udpTrack: netutil.NewIPTracker(iptrackWindow, iptrackContactWindow, iptrackMinStatements),
		entries:  make(map[string]enr.Entry),
	}
	ln.seq = db.localSeq(ln.id)
	ln.invalidate()
	return ln
}

//数据库返回与本地节点关联的节点数据库。
func (ln *LocalNode) Database() *DB {
	return ln.db
}

//node返回本地节点记录的当前版本。
func (ln *LocalNode) Node() *Node {
	n := ln.cur.Load().(*Node)
	if n != nil {
		return n
	}
//记录无效，请重新签名。
	ln.mu.Lock()
	defer ln.mu.Unlock()
	ln.sign()
	return ln.cur.Load().(*Node)
}

//id返回本地节点id。
func (ln *LocalNode) ID() ID {
	return ln.id
}

//set将给定条目放入本地记录中，覆盖
//任何现有值。
func (ln *LocalNode) Set(e enr.Entry) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.set(e)
}

func (ln *LocalNode) set(e enr.Entry) {
	val, exists := ln.entries[e.ENRKey()]
	if !exists || !reflect.DeepEqual(val, e) {
		ln.entries[e.ENRKey()] = e
		ln.invalidate()
	}
}

//删除从本地记录中删除给定条目。
func (ln *LocalNode) Delete(e enr.Entry) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.delete(e)
}

func (ln *LocalNode) delete(e enr.Entry) {
	_, exists := ln.entries[e.ENRKey()]
	if exists {
		delete(ln.entries, e.ENRKey())
		ln.invalidate()
	}
}

//setstaticip无条件地将本地IP设置为给定IP。
//这将禁用端点预测。
func (ln *LocalNode) SetStaticIP(ip net.IP) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.staticIP = ip
	ln.updateEndpoints()
}

//setfallbackip设置最后的IP地址。这个地址被使用了
//如果无法进行端点预测，并且未设置静态IP。
func (ln *LocalNode) SetFallbackIP(ip net.IP) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.fallbackIP = ip
	ln.updateEndpoints()
}

//setfallbackudp设置最后的手段udp端口。此端口已被使用
//如果无法进行端点预测。
func (ln *LocalNode) SetFallbackUDP(port int) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.fallbackUDP = port
	ln.updateEndpoints()
}

//每当有关本地节点的语句
//接收到UDP终结点。它为本地端点预测器提供数据。
func (ln *LocalNode) UDPEndpointStatement(fromaddr, endpoint *net.UDPAddr) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.udpTrack.AddStatement(fromaddr.String(), endpoint.String())
	ln.updateEndpoints()
}

//每当本地节点向另一个节点通告自己时，都应调用udpcontact。
//通过UDP。它为本地端点预测器提供数据。
func (ln *LocalNode) UDPContact(toaddr *net.UDPAddr) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.udpTrack.AddContact(toaddr.String())
	ln.updateEndpoints()
}

func (ln *LocalNode) updateEndpoints() {
//确定端点。
	newIP := ln.fallbackIP
	newUDP := ln.fallbackUDP
	if ln.staticIP != nil {
		newIP = ln.staticIP
	} else if ip, port := predictAddr(ln.udpTrack); ip != nil {
		newIP = ip
		newUDP = port
	}

//更新记录。
	if newIP != nil && !newIP.IsUnspecified() {
		ln.set(enr.IP(newIP))
		if newUDP != 0 {
			ln.set(enr.UDP(newUDP))
		} else {
			ln.delete(enr.UDP(0))
		}
	} else {
		ln.delete(enr.IP{})
	}
}

//PredictAddr包装iptracker.PredictEndpoint，从其基于字符串的转换
//IP和端口类型的端点表示。
func predictAddr(t *netutil.IPTracker) (net.IP, int) {
	ep := t.PredictEndpoint()
	if ep == "" {
		return nil, 0
	}
	ipString, portString, _ := net.SplitHostPort(ep)
	ip := net.ParseIP(ipString)
	port, _ := strconv.Atoi(portString)
	return ip, port
}

func (ln *LocalNode) invalidate() {
	ln.cur.Store((*Node)(nil))
}

func (ln *LocalNode) sign() {
	if n := ln.cur.Load().(*Node); n != nil {
return //没有变化
	}

	var r enr.Record
	for _, e := range ln.entries {
		r.Set(e)
	}
	ln.bumpSeq()
	r.SetSeq(ln.seq)
	if err := SignV4(&r, ln.key); err != nil {
		panic(fmt.Errorf("enode: can't sign record: %v", err))
	}
	n, err := New(ValidSchemes, &r)
	if err != nil {
		panic(fmt.Errorf("enode: can't verify local record: %v", err))
	}
	ln.cur.Store(n)
	log.Info("New local node record", "seq", ln.seq, "id", n.ID(), "ip", n.IP(), "udp", n.UDP(), "tcp", n.TCP())
}

func (ln *LocalNode) bumpSeq() {
	ln.seq++
	ln.db.storeLocalSeq(ln.id, ln.seq)
}
