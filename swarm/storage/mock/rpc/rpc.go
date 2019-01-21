
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

//package rpc实现一个连接到集中模拟存储的rpc客户机。
//中心化模拟存储可以是任何其他模拟存储实现，即
//以mockstore名称注册到以太坊RPC服务器。定义的方法
//mock.globalStore与rpc使用的相同。例子：
//
//服务器：=rpc.newserver（）
//server.registername（“mockstore”，mem.newGlobalStore（））
package rpc

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage/mock"
)

//GlobalStore是一个连接到中央模拟商店的rpc.client。
//关闭GlobalStore实例需要释放RPC客户端资源。
type GlobalStore struct {
	client *rpc.Client
}

//NewGlobalStore创建了一个新的GlobalStore实例。
func NewGlobalStore(client *rpc.Client) *GlobalStore {
	return &GlobalStore{
		client: client,
	}
}

//关闭关闭RPC客户端。
func (s *GlobalStore) Close() error {
	s.client.Close()
	return nil
}

//new nodestore返回一个新的nodestore实例，用于检索和存储
//仅对地址为的节点进行数据块处理。
func (s *GlobalStore) NewNodeStore(addr common.Address) *mock.NodeStore {
	return mock.NewNodeStore(addr, s)
}

//get调用rpc服务器的get方法。
func (s *GlobalStore) Get(addr common.Address, key []byte) (data []byte, err error) {
	err = s.client.Call(&data, "mockStore_get", addr, key)
	if err != nil && err.Error() == "not found" {
//传递模拟包的错误值，而不是一个RPC错误
		return data, mock.ErrNotFound
	}
	return data, err
}

//将一个Put方法调用到RPC服务器。
func (s *GlobalStore) Put(addr common.Address, key []byte, data []byte) error {
	err := s.client.Call(nil, "mockStore_put", addr, key, data)
	return err
}

//delete向rpc服务器调用delete方法。
func (s *GlobalStore) Delete(addr common.Address, key []byte) error {
	err := s.client.Call(nil, "mockStore_delete", addr, key)
	return err
}

//haskey向RPC服务器调用haskey方法。
func (s *GlobalStore) HasKey(addr common.Address, key []byte) bool {
	var has bool
	if err := s.client.Call(&has, "mockStore_hasKey", addr, key); err != nil {
		log.Error(fmt.Sprintf("mock store HasKey: addr %s, key %064x: %v", addr, key, err))
		return false
	}
	return has
}
