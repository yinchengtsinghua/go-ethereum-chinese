
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

package node_test

import (
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
)

//sampleService是一个普通的网络服务，可以连接到
//生命周期管理。
//
//实现node.service需要以下方法：
//-protocols（）[]p2p.protocol-服务可以通信的devp2p协议
//-apis（）[]rpc.api-服务希望在rpc通道上公开的api方法
//-start（）错误-节点准备启动服务时调用的方法
//-stop（）错误-节点终止服务时调用的方法
type SampleService struct{}

func (s *SampleService) Protocols() []p2p.Protocol { return nil }
func (s *SampleService) APIs() []rpc.API           { return nil }
func (s *SampleService) Start(*p2p.Server) error   { fmt.Println("Service starting..."); return nil }
func (s *SampleService) Stop() error               { fmt.Println("Service stopping..."); return nil }

func ExampleService() {
//创建一个网络节点以运行具有默认值的协议。
	stack, err := node.New(&node.Config{})
	if err != nil {
		log.Fatalf("Failed to create network node: %v", err)
	}
//创建并注册一个简单的网络服务。这是通过定义完成的
//将实例化node.service的node.serviceconstructor。原因
//工厂方法的方法是支持服务重新启动，而不依赖
//单个实现对此类操作的支持。
	constructor := func(context *node.ServiceContext) (node.Service, error) {
		return new(SampleService), nil
	}
	if err := stack.Register(constructor); err != nil {
		log.Fatalf("Failed to register service: %v", err)
	}
//启动整个协议栈，重新启动并终止
	if err := stack.Start(); err != nil {
		log.Fatalf("Failed to start the protocol stack: %v", err)
	}
	if err := stack.Restart(); err != nil {
		log.Fatalf("Failed to restart the protocol stack: %v", err)
	}
	if err := stack.Stop(); err != nil {
		log.Fatalf("Failed to stop the protocol stack: %v", err)
	}
//输出：
//服务正在启动…
//服务正在停止…
//服务正在启动…
//服务正在停止…
}
