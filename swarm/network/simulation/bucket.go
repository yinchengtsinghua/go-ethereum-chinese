
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

package simulation

import "github.com/ethereum/go-ethereum/p2p/enode"

//BucketKey是模拟存储桶中的键应该使用的类型。
type BucketKey string

//nodeItem返回在servicefunc函数中为特定节点设置的项。
func (s *Simulation) NodeItem(id enode.ID, key interface{}) (value interface{}, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.buckets[id]; !ok {
		return nil, false
	}
	return s.buckets[id].Load(key)
}

//setnodeitem设置与提供了nodeid的节点关联的新项。
//应使用存储桶来避免管理单独的模拟全局状态。
func (s *Simulation) SetNodeItem(id enode.ID, key interface{}, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.buckets[id].Store(key, value)
}

//nodes items返回在
//同样的BucketKey。
func (s *Simulation) NodesItems(key interface{}) (values map[enode.ID]interface{}) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.NodeIDs()
	values = make(map[enode.ID]interface{}, len(ids))
	for _, id := range ids {
		if _, ok := s.buckets[id]; !ok {
			continue
		}
		if v, ok := s.buckets[id].Load(key); ok {
			values[id] = v
		}
	}
	return values
}

//up nodes items从所有向上的节点返回具有相同bucketkey的项的映射。
func (s *Simulation) UpNodesItems(key interface{}) (values map[enode.ID]interface{}) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.UpNodeIDs()
	values = make(map[enode.ID]interface{})
	for _, id := range ids {
		if _, ok := s.buckets[id]; !ok {
			continue
		}
		if v, ok := s.buckets[id].Load(key); ok {
			values[id] = v
		}
	}
	return values
}
