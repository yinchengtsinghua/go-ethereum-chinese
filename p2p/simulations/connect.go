
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

package simulations

import (
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/p2p/enode"
)

var (
	ErrNodeNotFound = errors.New("node not found")
)

//ConnectToLastNode将节点与提供的节点ID连接起来
//到上一个节点，并避免连接到自身。
//它在构建链网络拓扑结构时很有用
//当网络动态添加和删除节点时。
func (net *Network) ConnectToLastNode(id enode.ID) (err error) {
	ids := net.getUpNodeIDs()
	l := len(ids)
	if l < 2 {
		return nil
	}
	last := ids[l-1]
	if last == id {
		last = ids[l-2]
	}
	return net.connect(last, id)
}

//connecttorandomnode将节点与提供的nodeid连接起来
//向上的随机节点发送。
func (net *Network) ConnectToRandomNode(id enode.ID) (err error) {
	selected := net.GetRandomUpNode(id)
	if selected == nil {
		return ErrNodeNotFound
	}
	return net.connect(selected.ID(), id)
}

//ConnectNodesFull将所有节点连接到另一个。
//它在网络中提供了完整的连接
//这应该是很少需要的。
func (net *Network) ConnectNodesFull(ids []enode.ID) (err error) {
	if ids == nil {
		ids = net.getUpNodeIDs()
	}
	for i, lid := range ids {
		for _, rid := range ids[i+1:] {
			if err = net.connect(lid, rid); err != nil {
				return err
			}
		}
	}
	return nil
}

//connectnodeschain连接链拓扑中的所有节点。
//如果ids参数为nil，则所有打开的节点都将被连接。
func (net *Network) ConnectNodesChain(ids []enode.ID) (err error) {
	if ids == nil {
		ids = net.getUpNodeIDs()
	}
	l := len(ids)
	for i := 0; i < l-1; i++ {
		if err := net.connect(ids[i], ids[i+1]); err != nil {
			return err
		}
	}
	return nil
}

//ConnectNodesRing连接环拓扑中的所有节点。
//如果ids参数为nil，则所有打开的节点都将被连接。
func (net *Network) ConnectNodesRing(ids []enode.ID) (err error) {
	if ids == nil {
		ids = net.getUpNodeIDs()
	}
	l := len(ids)
	if l < 2 {
		return nil
	}
	if err := net.ConnectNodesChain(ids); err != nil {
		return err
	}
	return net.connect(ids[l-1], ids[0])
}

//connectnodestar将所有节点连接到星形拓扑中
//如果ids参数为nil，则所有打开的节点都将被连接。
func (net *Network) ConnectNodesStar(ids []enode.ID, center enode.ID) (err error) {
	if ids == nil {
		ids = net.getUpNodeIDs()
	}
	for _, id := range ids {
		if center == id {
			continue
		}
		if err := net.connect(center, id); err != nil {
			return err
		}
	}
	return nil
}

//连接连接两个节点，但忽略已连接的错误。
func (net *Network) connect(oneID, otherID enode.ID) error {
	return ignoreAlreadyConnectedErr(net.Connect(oneID, otherID))
}

func ignoreAlreadyConnectedErr(err error) error {
	if err == nil || strings.Contains(err.Error(), "already connected") {
		return nil
	}
	return err
}
