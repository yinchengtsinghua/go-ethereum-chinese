
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2014 Go Ethereum作者
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

package p2p

import (
	"fmt"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

//协议表示P2P子协议实现。
type Protocol struct {
//名称应包含官方协议名称，
//通常是三个字母的单词。
	Name string

//版本应包含协议的版本号。
	Version uint

//长度应包含使用的消息代码数
//按照协议。
	Length uint64

//当协议
//与同行协商。它应该读写来自
//RW。每个消息的有效负载必须完全消耗。
//
//当Start返回时，对等连接将关闭。它应该会回来
//任何协议级错误（如I/O错误），即
//遇到。
	Run func(peer *Peer, rw MsgReadWriter) error

//nodeinfo是用于检索协议特定元数据的可选助手方法
//关于主机节点。
	NodeInfo func() interface{}

//peerinfo是一个可选的帮助器方法，用于检索协议特定的元数据
//关于网络中的某个对等点。如果设置了信息检索功能，
//但返回nil，假设协议握手仍在运行。
	PeerInfo func(id enode.ID) interface{}

//属性包含节点记录的协议特定信息。
	Attributes []enr.Entry
}

func (p Protocol) cap() Cap {
	return Cap{p.Name, p.Version}
}

//cap是对等能力的结构。
type Cap struct {
	Name    string
	Version uint
}

func (cap Cap) String() string {
	return fmt.Sprintf("%s/%d", cap.Name, cap.Version)
}

type capsByNameAndVersion []Cap

func (cs capsByNameAndVersion) Len() int      { return len(cs) }
func (cs capsByNameAndVersion) Swap(i, j int) { cs[i], cs[j] = cs[j], cs[i] }
func (cs capsByNameAndVersion) Less(i, j int) bool {
	return cs[i].Name < cs[j].Name || (cs[i].Name == cs[j].Name && cs[i].Version < cs[j].Version)
}

func (capsByNameAndVersion) ENRKey() string { return "cap" }
