
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

package discv5

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

//节点表示网络上的主机。
//不能修改节点的公共字段。
type Node struct {
IP       net.IP //IPv4的len 4或IPv6的len 16
UDP, TCP uint16 //端口号
ID       NodeID //节点的公钥

//网络相关字段包含在节点中。
//这些字段不应该在
//network.loop goroutine。
	nodeNetGuts
}

//new node创建新节点。它主要用于
//测试目的。
func NewNode(id NodeID, ip net.IP, udpPort, tcpPort uint16) *Node {
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
	return &Node{
		IP:          ip,
		UDP:         udpPort,
		TCP:         tcpPort,
		ID:          id,
		nodeNetGuts: nodeNetGuts{sha: crypto.Keccak256Hash(id[:])},
	}
}

func (n *Node) addr() *net.UDPAddr {
	return &net.UDPAddr{IP: n.IP, Port: int(n.UDP)}
}

func (n *Node) setAddr(a *net.UDPAddr) {
	n.IP = a.IP
	if ipv4 := a.IP.To4(); ipv4 != nil {
		n.IP = ipv4
	}
	n.UDP = uint16(a.Port)
}

//将给定地址与存储值进行比较。
func (n *Node) addrEqual(a *net.UDPAddr) bool {
	ip := a.IP
	if ipv4 := a.IP.To4(); ipv4 != nil {
		ip = ipv4
	}
	return n.UDP == uint16(a.Port) && n.IP.Equal(ip)
}

//对于没有IP地址的节点，不完整返回true。
func (n *Node) Incomplete() bool {
	return n.IP == nil
}

//检查n是否为有效的完整节点。
func (n *Node) validateComplete() error {
	if n.Incomplete() {
		return errors.New("incomplete node")
	}
	if n.UDP == 0 {
		return errors.New("missing UDP port")
	}
	if n.TCP == 0 {
		return errors.New("missing TCP port")
	}
	if n.IP.IsMulticast() || n.IP.IsUnspecified() {
		return errors.New("invalid IP (multicast/unspecified)")
	}
_, err := n.ID.Pubkey() //验证密钥（在曲线上等）
	return err
}

//节点的字符串表示形式是一个URL。
//有关格式的描述，请参阅ParseNode。
func (n *Node) String() string {
	u := url.URL{Scheme: "enode"}
	if n.Incomplete() {
		u.Host = fmt.Sprintf("%x", n.ID[:])
	} else {
		addr := net.TCPAddr{IP: n.IP, Port: int(n.TCP)}
		u.User = url.User(fmt.Sprintf("%x", n.ID[:]))
		u.Host = addr.String()
		if n.UDP != n.TCP {
			u.RawQuery = "discport=" + strconv.Itoa(int(n.UDP))
		}
	}
	return u.String()
}

var incompleteNodeURL = regexp.MustCompile("(?i)^(?:enode://）？（[09a- f] +）$

//ParseNode解析节点指示符。
//
//节点指示符有两种基本形式
//-节点不完整，只有公钥（节点ID）
//-包含公钥和IP/端口信息的完整节点
//
//对于不完整的节点，指示器必须类似于
//
//enode://<hex node id>
//<十六进制节点ID >
//
//对于完整的节点，节点ID编码在用户名部分
//以@符号与主机分隔的URL。主机名可以
//仅作为IP地址提供，不允许使用DNS域名。
//主机名部分中的端口是TCP侦听端口。如果
//TCP和UDP（发现）端口不同，UDP端口指定为
//查询参数“discport”。
//
//在下面的示例中，节点URL描述了
//IP地址为10.3.58.6、TCP侦听端口为30303的节点。
//和UDP发现端口30301。
//
//enode://<hex node id>@10.3.58.6:30303？磁盘端口＝30301
func ParseNode(rawurl string) (*Node, error) {
	if m := incompleteNodeURL.FindStringSubmatch(rawurl); m != nil {
		id, err := HexID(m[1])
		if err != nil {
			return nil, fmt.Errorf("invalid node ID (%v)", err)
		}
		return NewNode(id, nil, 0, 0), nil
	}
	return parseComplete(rawurl)
}

func parseComplete(rawurl string) (*Node, error) {
	var (
		id               NodeID
		ip               net.IP
		tcpPort, udpPort uint64
	)
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "enode" {
		return nil, errors.New("invalid URL scheme, want \"enode\"")
	}
//从用户部分分析节点ID。
	if u.User == nil {
		return nil, errors.New("does not contain node ID")
	}
	if id, err = HexID(u.User.String()); err != nil {
		return nil, fmt.Errorf("invalid node ID (%v)", err)
	}
//分析IP地址。
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, fmt.Errorf("invalid host: %v", err)
	}
	if ip = net.ParseIP(host); ip == nil {
		return nil, errors.New("invalid IP address")
	}
//确保IPv4地址的IP长度为4字节。
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
//分析端口号。
	if tcpPort, err = strconv.ParseUint(port, 10, 16); err != nil {
		return nil, errors.New("invalid port")
	}
	udpPort = tcpPort
	qv := u.Query()
	if qv.Get("discport") != "" {
		udpPort, err = strconv.ParseUint(qv.Get("discport"), 10, 16)
		if err != nil {
			return nil, errors.New("invalid discport in query")
		}
	}
	return NewNode(id, ip, uint16(udpPort), uint16(tcpPort)), nil
}

//mustParseNode解析节点URL。如果URL无效，它会恐慌。
func MustParseNode(rawurl string) *Node {
	n, err := ParseNode(rawurl)
	if err != nil {
		panic("invalid node URL: " + err.Error())
	}
	return n
}

//MarshalText实现Encoding.TextMarshaler。
func (n *Node) MarshalText() ([]byte, error) {
	return []byte(n.String()), nil
}

//UnmarshalText实现encoding.textUnmarshaller。
func (n *Node) UnmarshalText(text []byte) error {
	dec, err := ParseNode(string(text))
	if err == nil {
		*n = *dec
	}
	return err
}

//键入nodequeue[]*node
//
////如果pushnew不存在，则在末尾添加n。
//func（nl*nodelist）appendnew（n*node）
//对于u，输入：=范围n
//IF项＝n{
//返回
//}
//}
//*nq=附加（*nq，n）
//}
//
////PopRandom删除一个随机节点。节点接近
////到开头的概率稍高。
//func（nl*nodelist）popRandom（）*节点
//ix：=rand.intn（长度（*nq））
////TODO:如上所述的概率。
//nl.删除索引（ix）
//}
//
//func（nl*nodelist）removeindex（i int）*节点
//切片＝*nl
//如果len（*slice）<=i
//返回零
//}
//*nl=append（slice[：i]，slice[i+1:……）
//}

const nodeIDBits = 512

//nodeid是每个节点的唯一标识符。
//节点标识符是一个封送椭圆曲线公钥。
type NodeID [nodeIDBits / 8]byte

//nodeid以十六进制长数字打印。
func (n NodeID) String() string {
	return fmt.Sprintf("%x", n[:])
}

//nodeid的go语法表示是对hexid的调用。
func (n NodeID) GoString() string {
	return fmt.Sprintf("discover.HexID(\"%x\")", n[:])
}

//TerminalString返回用于终端日志记录的缩短的十六进制字符串。
func (n NodeID) TerminalString() string {
	return hex.EncodeToString(n[:8])
}

//hexid将十六进制字符串转换为nodeid。
//字符串的前缀可以是0x。
func HexID(in string) (NodeID, error) {
	var id NodeID
	b, err := hex.DecodeString(strings.TrimPrefix(in, "0x"))
	if err != nil {
		return id, err
	} else if len(b) != len(id) {
		return id, fmt.Errorf("wrong length, want %d hex chars", len(id)*2)
	}
	copy(id[:], b)
	return id, nil
}

//musthexid将十六进制字符串转换为nodeid。
//如果字符串不是有效的nodeid，它会恐慌。
func MustHexID(in string) NodeID {
	id, err := HexID(in)
	if err != nil {
		panic(err)
	}
	return id
}

//pubkeyid返回给定公钥的封送表示形式。
func PubkeyID(pub *ecdsa.PublicKey) NodeID {
	var id NodeID
	pbytes := elliptic.Marshal(pub.Curve, pub.X, pub.Y)
	if len(pbytes)-1 != len(id) {
		panic(fmt.Errorf("need %d bit pubkey, got %d bits", (len(id)+1)*8, len(pbytes)))
	}
	copy(id[:], pbytes[1:])
	return id
}

//pubkey返回由节点ID表示的公钥。
//如果ID不是曲线上的点，则返回错误。
func (n NodeID) Pubkey() (*ecdsa.PublicKey, error) {
	p := &ecdsa.PublicKey{Curve: crypto.S256(), X: new(big.Int), Y: new(big.Int)}
	half := len(n) / 2
	p.X.SetBytes(n[:half])
	p.Y.SetBytes(n[half:])
	if !p.Curve.IsOnCurve(p.X, p.Y) {
		return nil, errors.New("id is invalid secp256k1 curve point")
	}
	return p, nil
}

func (id NodeID) mustPubkey() ecdsa.PublicKey {
	pk, err := id.Pubkey()
	if err != nil {
		panic(err)
	}
	return *pk
}

//recovernodeid计算用于签名的公钥
//从签名中给定哈希。
func recoverNodeID(hash, sig []byte) (id NodeID, err error) {
	pubkey, err := crypto.Ecrecover(hash, sig)
	if err != nil {
		return id, err
	}
	if len(pubkey)-1 != len(id) {
		return id, fmt.Errorf("recovered pubkey has %d bits, want %d bits", len(pubkey)*8, (len(id)+1)*8)
	}
	for i := range id {
		id[i] = pubkey[i+1]
	}
	return id, nil
}

//distcmp比较距离a->target和b->target。
//如果a接近目标返回-1，如果b接近目标返回1
//如果相等，则为0。
func distcmp(target, a, b common.Hash) int {
	for i := range target {
		da := a[i] ^ target[i]
		db := b[i] ^ target[i]
		if da > db {
			return 1
		} else if da < db {
			return -1
		}
	}
	return 0
}

//字节[0..255]的前导零计数表
var lzcount = [256]int{
	8, 7, 6, 6, 5, 5, 5, 5,
	4, 4, 4, 4, 4, 4, 4, 4,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
	2, 2, 2, 2, 2, 2, 2, 2,
	2, 2, 2, 2, 2, 2, 2, 2,
	2, 2, 2, 2, 2, 2, 2, 2,
	2, 2, 2, 2, 2, 2, 2, 2,
	1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
}

//logdist返回a和b之间的对数距离，log2（a^b）。
func logdist(a, b common.Hash) int {
	lz := 0
	for i := range a {
		x := a[i] ^ b[i]
		if x == 0 {
			lz += 8
		} else {
			lz += lzcount[x]
			break
		}
	}
	return len(a)*8 - lz
}

//hashatDistance返回一个随机哈希，使logDist（a，b）=n
func hashAtDistance(a common.Hash, n int) (b common.Hash) {
	if n == 0 {
		return a
	}
//在n位置翻转钻头，其余部分用随机钻头填满。
	b = a
	pos := len(a) - n/8 - 1
	bit := byte(0x01) << (byte(n%8) - 1)
	if bit == 0 {
		pos++
		bit = 0x80
	}
b[pos] = a[pos]&^bit | ^a[pos]&bit //TODO:随机结束位
	for i := pos + 1; i < len(a); i++ {
		b[i] = byte(rand.Intn(255))
	}
	return b
}
