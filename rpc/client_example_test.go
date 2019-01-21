
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

package rpc_test

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
)

//在本例中，我们的客户希望跟踪最新的“块编号”
//服务器已知。服务器支持两种方法：
//
//eth_getBlockByNumber（“最新”，）
//返回最新的块对象。
//
//ETH订阅（“newblocks”）
//创建在新块到达时激发块对象的订阅。

type Block struct {
	Number *big.Int
}

func ExampleClientSubscription() {
//连接客户端。
client, _ := rpc.Dial("ws://127.0.0.1:8485“）
	subch := make(chan Block)

//确保Subch接收到最新的块。
	go func() {
		for i := 0; ; i++ {
			if i > 0 {
				time.Sleep(2 * time.Second)
			}
			subscribeBlocks(client, subch)
		}
	}()

//到达时打印订阅中的事件。
	for block := range subch {
		fmt.Println("latest block:", block.Number)
	}
}

//subscribeBlocks在自己的goroutine中运行并维护
//新块的订阅。
func subscribeBlocks(client *rpc.Client, subch chan Block) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

//订阅新块。
	sub, err := client.EthSubscribe(ctx, subch, "newHeads")
	if err != nil {
		fmt.Println("subscribe error:", err)
		return
	}

//现在已建立连接。
//用当前块更新频道。
	var lastBlock Block
	if err := client.CallContext(ctx, &lastBlock, "eth_getBlockByNumber", "latest"); err != nil {
		fmt.Println("can't get latest block:", err)
		return
	}
	subch <- lastBlock

//订阅将向通道传递事件。等待
//订阅以任何原因结束，然后循环重新建立
//连接。
	fmt.Println("connection lost: ", <-sub.Err())
}
