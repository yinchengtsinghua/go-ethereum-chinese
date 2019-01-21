
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

//包含p2p包的包装。

package geth

import (
	"errors"

	"github.com/ethereum/go-ethereum/p2p"
)

//nodeinfo表示已知主机信息的pi简短摘要。
type NodeInfo struct {
	info *p2p.NodeInfo
}

func (ni *NodeInfo) GetID() string              { return ni.info.ID }
func (ni *NodeInfo) GetName() string            { return ni.info.Name }
func (ni *NodeInfo) GetEnode() string           { return ni.info.Enode }
func (ni *NodeInfo) GetIP() string              { return ni.info.IP }
func (ni *NodeInfo) GetDiscoveryPort() int      { return ni.info.Ports.Discovery }
func (ni *NodeInfo) GetListenerPort() int       { return ni.info.Ports.Listener }
func (ni *NodeInfo) GetListenerAddress() string { return ni.info.ListenAddr }
func (ni *NodeInfo) GetProtocols() *Strings {
	protos := []string{}
	for proto := range ni.info.Protocols {
		protos = append(protos, proto)
	}
	return &Strings{protos}
}

//peerinfo表示关于pi-connected peer已知信息的pi简短摘要。
type PeerInfo struct {
	info *p2p.PeerInfo
}

func (pi *PeerInfo) GetID() string            { return pi.info.ID }
func (pi *PeerInfo) GetName() string          { return pi.info.Name }
func (pi *PeerInfo) GetCaps() *Strings        { return &Strings{pi.info.Caps} }
func (pi *PeerInfo) GetLocalAddress() string  { return pi.info.Network.LocalAddress }
func (pi *PeerInfo) GetRemoteAddress() string { return pi.info.Network.RemoteAddress }

//PeerInfos表示关于远程对等机的一部分信息。
type PeerInfos struct {
	infos []*p2p.PeerInfo
}

//SIZE返回切片中的对等信息条目数。
func (pi *PeerInfos) Size() int {
	return len(pi.infos)
}

//GET从切片返回给定索引处的对等信息。
func (pi *PeerInfos) Get(index int) (info *PeerInfo, _ error) {
	if index < 0 || index >= len(pi.infos) {
		return nil, errors.New("index out of bounds")
	}
	return &PeerInfo{pi.infos[index]}, nil
}
