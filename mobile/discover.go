
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

//包含来自帐户包的所有包装，以支持客户端enode
//移动平台管理。

package geth

import (
	"errors"

	"github.com/ethereum/go-ethereum/p2p/discv5"
)

//enode表示网络上的主机。
type Enode struct {
	node *discv5.Node
}

//newenode解析节点指示符。
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
func NewEnode(rawurl string) (enode *Enode, _ error) {
	node, err := discv5.ParseNode(rawurl)
	if err != nil {
		return nil, err
	}
	return &Enode{node}, nil
}

//Enodes代表一部分账户。
type Enodes struct{ nodes []*discv5.Node }

//newenodes创建一个未初始化的enodes切片。
func NewEnodes(size int) *Enodes {
	return &Enodes{
		nodes: make([]*discv5.Node, size),
	}
}

//newEnodeEmpty创建一个enode值的空切片。
func NewEnodesEmpty() *Enodes {
	return NewEnodes(0)
}

//SIZE返回切片中的enodes数。
func (e *Enodes) Size() int {
	return len(e.nodes)
}

//get从切片返回给定索引处的enode。
func (e *Enodes) Get(index int) (enode *Enode, _ error) {
	if index < 0 || index >= len(e.nodes) {
		return nil, errors.New("index out of bounds")
	}
	return &Enode{e.nodes[index]}, nil
}

//set在切片中的给定索引处设置enode。
func (e *Enodes) Set(index int, enode *Enode) error {
	if index < 0 || index >= len(e.nodes) {
		return errors.New("index out of bounds")
	}
	e.nodes[index] = enode.node
	return nil
}

//append在切片的末尾添加一个新的enode元素。
func (e *Enodes) Append(enode *Enode) {
	e.nodes = append(e.nodes, enode.node)
}
