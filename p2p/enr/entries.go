
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

package enr

import (
	"fmt"
	"io"
	"net"

	"github.com/ethereum/go-ethereum/rlp"
)

//条目由已知的节点记录条目类型实现。
//
//要定义要包含在节点记录中的新条目，
//创建满足此接口的Go类型。类型应该
//如果需要对值进行额外检查，还可以实现rlp.decoder。
type Entry interface {
	ENRKey() string
}

type generic struct {
	key   string
	value interface{}
}

func (g generic) ENRKey() string { return g.key }

func (g generic) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, g.value)
}

func (g *generic) DecodeRLP(s *rlp.Stream) error {
	return s.Decode(g.value)
}

//WithEntry用键名包装任何值。它可用于设置和加载任意值
//在记录中。值v必须由rlp支持。要在加载时使用WithEntry，值
//必须是指针。
func WithEntry(k string, v interface{}) Entry {
	return &generic{key: k, value: v}
}

//tcp是“tcp”密钥，它保存节点的tcp端口。
type TCP uint16

func (v TCP) ENRKey() string { return "tcp" }

//udp是“udp”密钥，它保存节点的udp端口。
type UDP uint16

func (v UDP) ENRKey() string { return "udp" }

//ID是“ID”键，它保存标识方案的名称。
type ID string

const IDv4 = ID("v4") //默认标识方案

func (v ID) ENRKey() string { return "id" }

//IP是“IP”密钥，它保存节点的IP地址。
type IP net.IP

func (v IP) ENRKey() string { return "ip" }

//encoderlp实现rlp.encoder。
func (v IP) EncodeRLP(w io.Writer) error {
	if ip4 := net.IP(v).To4(); ip4 != nil {
		return rlp.Encode(w, ip4)
	}
	return rlp.Encode(w, net.IP(v))
}

//decoderlp实现rlp.decoder。
func (v *IP) DecodeRLP(s *rlp.Stream) error {
	if err := s.Decode((*net.IP)(v)); err != nil {
		return err
	}
	if len(*v) != 4 && len(*v) != 16 {
		return fmt.Errorf("invalid IP address, want 4 or 16 bytes: %v", *v)
	}
	return nil
}

//keyError是一个与键相关的错误。
type KeyError struct {
	Key string
	Err error
}

//错误实现错误。
func (err *KeyError) Error() string {
	if err.Err == errNotFound {
		return fmt.Sprintf("missing ENR key %q", err.Key)
	}
	return fmt.Sprintf("ENR key %q: %v", err.Key, err.Err)
}

//IsNotFound报告给定的错误是否意味着键/值对
//记录中缺少。
func IsNotFound(err error) bool {
	kerr, ok := err.(*KeyError)
	return ok && kerr.Err == errNotFound
}
