
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

//包netutil包含对网络包的扩展。
package netutil

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
)

var lan4, lan6, special4, special6 Netlist

func init() {
//来自RFC 5735、RFC 5156的列表，
//https://www.iana.org/assignments/iana-ipv4-special-registry/
lan4.Add("0.0.0.0/8")              //“这个”网络
lan4.Add("10.0.0.0/8")             //私人使用
lan4.Add("172.16.0.0/12")          //私人使用
lan4.Add("192.168.0.0/16")         //私人使用
lan6.Add("fe80::/10")              //链路本地
lan6.Add("fc00::/7")               //独特的地方
special4.Add("192.0.0.0/29")       //IPv4服务连续性
special4.Add("192.0.0.9/32")       //PCP选播
special4.Add("192.0.0.170/32")     //NAT64/DNS64发现
special4.Add("192.0.0.171/32")     //NAT64/DNS64发现
special4.Add("192.0.2.0/24")       //测试网络-1
special4.Add("192.31.196.0/24")    //AS112
special4.Add("192.52.193.0/24")    //AMT
special4.Add("192.88.99.0/24")     //6to4继电器选播
special4.Add("192.175.48.0/24")    //AS112
special4.Add("198.18.0.0/15")      //设备基准测试
special4.Add("198.51.100.0/24")    //测试网NET-2
special4.Add("203.0.113.0/24")     //测试网-3
special4.Add("255.255.255.255/32") //有限广播

//http://www.iana.org/assignments/iana-ipv6-special-registry/
	special6.Add("100::/64")
	special6.Add("2001::/32")
	special6.Add("2001:1::1/128")
	special6.Add("2001:2::/48")
	special6.Add("2001:3::/32")
	special6.Add("2001:4:112::/48")
	special6.Add("2001:5::/32")
	special6.Add("2001:10::/28")
	special6.Add("2001:20::/28")
	special6.Add("2001:db8::/32")
	special6.Add("2002::/16")
}

//netlist是IP网络的列表。
type Netlist []net.IPNet

//ParseNetList解析CIDR掩码的逗号分隔列表。
//空白和多余的逗号将被忽略。
func ParseNetlist(s string) (*Netlist, error) {
	ws := strings.NewReplacer(" ", "", "\n", "", "\t", "")
	masks := strings.Split(ws.Replace(s), ",")
	l := make(Netlist, 0)
	for _, mask := range masks {
		if mask == "" {
			continue
		}
		_, n, err := net.ParseCIDR(mask)
		if err != nil {
			return nil, err
		}
		l = append(l, *n)
	}
	return &l, nil
}

//marshaltoml实现toml.marshalerrec。
func (l Netlist) MarshalTOML() interface{} {
	list := make([]string, 0, len(l))
	for _, net := range l {
		list = append(list, net.String())
	}
	return list
}

//unmarshaltoml实现toml.unmarshalerrec。
func (l *Netlist) UnmarshalTOML(fn func(interface{}) error) error {
	var masks []string
	if err := fn(&masks); err != nil {
		return err
	}
	for _, mask := range masks {
		_, n, err := net.ParseCIDR(mask)
		if err != nil {
			return err
		}
		*l = append(*l, *n)
	}
	return nil
}

//添加分析CIDR掩码并将其附加到列表中。它对无效的掩码感到恐慌，并且
//用于设置静态列表。
func (l *Netlist) Add(cidr string) {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	*l = append(*l, *n)
}

//包含报告给定IP是否包含在列表中。
func (l *Netlist) Contains(ip net.IP) bool {
	if l == nil {
		return false
	}
	for _, net := range *l {
		if net.Contains(ip) {
			return true
		}
	}
	return false
}

//islan报告IP是否为本地网络地址。
func IsLAN(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		return lan4.Contains(v4)
	}
	return lan6.Contains(ip)
}

//IsSpecialNetwork报告IP是否位于专用网络范围内
//这包括广播、多播和文档地址。
func IsSpecialNetwork(ip net.IP) bool {
	if ip.IsMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		return special4.Contains(v4)
	}
	return special6.Contains(ip)
}

var (
	errInvalid     = errors.New("invalid IP")
	errUnspecified = errors.New("zero address")
	errSpecial     = errors.New("special network")
	errLoopback    = errors.New("loopback address from non-loopback host")
	errLAN         = errors.New("LAN address from WAN host")
)

//checkrelayip报告是否从给定的发送方IP中继IP
//是有效的连接目标。
//
//有四条规则：
//-特殊网络地址永远无效。
//-如果由回送主机中继，则回送地址正常。
//-如果由LAN主机中继，则LAN地址正常。
//-所有其他地址始终可以接受。
func CheckRelayIP(sender, addr net.IP) error {
	if len(addr) != net.IPv4len && len(addr) != net.IPv6len {
		return errInvalid
	}
	if addr.IsUnspecified() {
		return errUnspecified
	}
	if IsSpecialNetwork(addr) {
		return errSpecial
	}
	if addr.IsLoopback() && !sender.IsLoopback() {
		return errLoopback
	}
	if IsLAN(addr) && !IsLAN(sender) {
		return errLAN
	}
	return nil
}

//Samenet报告两个IP地址是否具有相同的给定位长度前缀。
func SameNet(bits uint, ip, other net.IP) bool {
	ip4, other4 := ip.To4(), other.To4()
	switch {
	case (ip4 == nil) != (other4 == nil):
		return false
	case ip4 != nil:
		return sameNet(bits, ip4, other4)
	default:
		return sameNet(bits, ip.To16(), other.To16())
	}
}

func sameNet(bits uint, ip, other net.IP) bool {
	nb := int(bits / 8)
	mask := ^byte(0xFF >> (bits % 8))
	if mask != 0 && nb < len(ip) && ip[nb]&mask != other[nb]&mask {
		return false
	}
	return nb <= len(ip) && bytes.Equal(ip[:nb], other[:nb])
}

//DistinctNetset跟踪IP，确保最多N个IP
//属于同一网络范围。
type DistinctNetSet struct {
Subnet uint //公共前缀位数
Limit  uint //每个子网中的最大IP数

	members map[string]uint
	buf     net.IP
}

//添加将IP地址添加到集合中。如果
//定义范围内的现有IP数超过限制。
func (s *DistinctNetSet) Add(ip net.IP) bool {
	key := s.key(ip)
	n := s.members[string(key)]
	if n < s.Limit {
		s.members[string(key)] = n + 1
		return true
	}
	return false
}

//移除从集合中移除IP。
func (s *DistinctNetSet) Remove(ip net.IP) {
	key := s.key(ip)
	if n, ok := s.members[string(key)]; ok {
		if n == 1 {
			delete(s.members, string(key))
		} else {
			s.members[string(key)] = n - 1
		}
	}
}

//包含给定IP是否包含在集合中。
func (s DistinctNetSet) Contains(ip net.IP) bool {
	key := s.key(ip)
	_, ok := s.members[string(key)]
	return ok
}

//len返回跟踪的IP数。
func (s DistinctNetSet) Len() int {
	n := uint(0)
	for _, i := range s.members {
		n += i
	}
	return int(n)
}

//键将地址的映射键编码到临时缓冲区中。
//
//密钥的第一个字节是“4”或“6”，用于区分IPv4/IPv6地址类型。
//密钥的其余部分是IP，截断为位数。
func (s *DistinctNetSet) key(ip net.IP) net.IP {
//延迟初始化存储。
	if s.members == nil {
		s.members = make(map[string]uint)
		s.buf = make(net.IP, 17)
	}
//将IP和位规范化。
	typ := byte('6')
	if ip4 := ip.To4(); ip4 != nil {
		typ, ip = '4', ip4
	}
	bits := s.Subnet
	if bits > uint(len(ip)*8) {
		bits = uint(len(ip) * 8)
	}
//将前缀编码为s.buf。
	nb := int(bits / 8)
	mask := ^byte(0xFF >> (bits % 8))
	s.buf[0] = typ
	buf := append(s.buf[:1], ip[:nb]...)
	if nb < len(ip) && mask != 0 {
		buf = append(buf, ip[nb]&mask)
	}
	return buf
}

//字符串实现fmt.stringer
func (s DistinctNetSet) String() string {
	var buf bytes.Buffer
	buf.WriteString("{")
	keys := make([]string, 0, len(s.members))
	for k := range s.members {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		var ip net.IP
		if k[0] == '4' {
			ip = make(net.IP, 4)
		} else {
			ip = make(net.IP, 16)
		}
		copy(ip, k[1:])
		fmt.Fprintf(&buf, "%v×%d", ip, s.members[k])
		if i != len(keys)-1 {
			buf.WriteString(" ")
		}
	}
	buf.WriteString("}")
	return buf.String()
}
