
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
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"math/rand"
	"net"
	"strings"

	"github.com/ethereum/go-ethereum/p2p/enr"
)

//节点表示网络上的主机。
type Node struct {
	r  enr.Record
	id ID
}

//New包装节点记录。根据给定的记录必须是有效的
//身份方案。
func New(validSchemes enr.IdentityScheme, r *enr.Record) (*Node, error) {
	if err := r.VerifySignature(validSchemes); err != nil {
		return nil, err
	}
	node := &Node{r: *r}
	if n := copy(node.id[:], validSchemes.NodeAddr(&node.r)); n != len(ID{}) {
		return nil, fmt.Errorf("invalid node ID length %d, need %d", n, len(ID{}))
	}
	return node, nil
}

//ID返回节点标识符。
func (n *Node) ID() ID {
	return n.id
}

//seq返回基础记录的序列号。
func (n *Node) Seq() uint64 {
	return n.r.Seq()
}

//对于没有IP地址的节点，不完整返回true。
func (n *Node) Incomplete() bool {
	return n.IP() == nil
}

//LOAD从基础记录中检索一个条目。
func (n *Node) Load(k enr.Entry) error {
	return n.r.Load(k)
}

//IP返回节点的IP地址。
func (n *Node) IP() net.IP {
	var ip net.IP
	n.Load((*enr.IP)(&ip))
	return ip
}

//udp返回节点的udp端口。
func (n *Node) UDP() int {
	var port enr.UDP
	n.Load(&port)
	return int(port)
}

//UDP返回节点的TCP端口。
func (n *Node) TCP() int {
	var port enr.TCP
	n.Load(&port)
	return int(port)
}

//pubkey返回节点的secp256k1公钥（如果存在）。
func (n *Node) Pubkey() *ecdsa.PublicKey {
	var key ecdsa.PublicKey
	if n.Load((*Secp256k1)(&key)) != nil {
		return nil
	}
	return &key
}

//record返回节点的记录。返回值是一个副本，可以
//由调用者修改。
func (n *Node) Record() *enr.Record {
	cpy := n.r
	return &cpy
}

//检查n是否为有效的完整节点。
func (n *Node) ValidateComplete() error {
	if n.Incomplete() {
		return errors.New("incomplete node")
	}
	if n.UDP() == 0 {
		return errors.New("missing UDP port")
	}
	ip := n.IP()
	if ip.IsMulticast() || ip.IsUnspecified() {
		return errors.New("invalid IP (multicast/unspecified)")
	}
//验证节点键（在曲线上等）。
	var key Secp256k1
	return n.Load(&key)
}

//节点的字符串表示形式是一个URL。
//有关格式的描述，请参阅ParseNode。
func (n *Node) String() string {
	return n.v4URL()
}

//MarshalText实现Encoding.TextMarshaler。
func (n *Node) MarshalText() ([]byte, error) {
	return []byte(n.v4URL()), nil
}

//UnmarshalText实现encoding.textUnmarshaller。
func (n *Node) UnmarshalText(text []byte) error {
	dec, err := ParseV4(string(text))
	if err == nil {
		*n = *dec
	}
	return err
}

//ID是每个节点的唯一标识符。
type ID [32]byte

//bytes返回ID的字节片表示形式
func (n ID) Bytes() []byte {
	return n[:]
}

//ID以十六进制长数字打印。
func (n ID) String() string {
	return fmt.Sprintf("%x", n[:])
}

//ID的go语法表示是对hexid的调用。
func (n ID) GoString() string {
	return fmt.Sprintf("enode.HexID(\"%x\")", n[:])
}

//TerminalString返回用于终端日志记录的缩短的十六进制字符串。
func (n ID) TerminalString() string {
	return hex.EncodeToString(n[:8])
}

//MarshalText实现Encoding.TextMarshaler接口。
func (n ID) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(n[:])), nil
}

//UnmarshalText实现encoding.textUnmarshaller接口。
func (n *ID) UnmarshalText(text []byte) error {
	id, err := parseID(string(text))
	if err != nil {
		return err
	}
	*n = id
	return nil
}

//hex id将十六进制字符串转换为ID。
//字符串的前缀可以是0x。
//如果字符串不是有效的ID，则会恐慌。
func HexID(in string) ID {
	id, err := parseID(in)
	if err != nil {
		panic(err)
	}
	return id
}

func parseID(in string) (ID, error) {
	var id ID
	b, err := hex.DecodeString(strings.TrimPrefix(in, "0x"))
	if err != nil {
		return id, err
	} else if len(b) != len(id) {
		return id, fmt.Errorf("wrong length, want %d hex chars", len(id)*2)
	}
	copy(id[:], b)
	return id, nil
}

//distcmp比较距离a->target和b->target。
//如果a接近目标返回-1，如果b接近目标返回1
//如果相等，则为0。
func DistCmp(target, a, b ID) int {
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

//logdist返回a和b之间的对数距离，log2（a^b）。
func LogDist(a, b ID) int {
	lz := 0
	for i := range a {
		x := a[i] ^ b[i]
		if x == 0 {
			lz += 8
		} else {
			lz += bits.LeadingZeros8(x)
			break
		}
	}
	return len(a)*8 - lz
}

//随机返回一个随机ID B，使得logdist（a，b）=n。
func RandomID(a ID, n int) (b ID) {
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
