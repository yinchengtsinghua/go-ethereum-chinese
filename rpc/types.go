
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

package rpc

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"

	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

//API描述了通过RPC接口提供的一组方法
type API struct {
Namespace string      //暴露服务的RPC方法的命名空间
Version   string      //DAPP的API版本
Service   interface{} //保存方法的接收器实例
Public    bool        //是否必须将这些方法视为公共使用安全的指示
}

//回调是在服务器中注册的方法回调
type callback struct {
rcvr        reflect.Value  //方法接受者
method      reflect.Method //回调
argTypes    []reflect.Type //输入参数类型
hasCtx      bool           //方法的第一个参数是上下文（不包括在argtype中）
errPos      int            //当方法无法返回错误时，错误返回IDX，共-1个
isSubscribe bool           //指示回调是否为订阅
}

//服务表示已注册的对象
type service struct {
name          string        //服务的名称
typ           reflect.Type  //接收机类型
callbacks     callbacks     //已注册的处理程序
subscriptions subscriptions //可用订阅/通知
}

//ServerRequest是一个传入请求
type serverRequest struct {
	id            interface{}
	svcname       string
	callb         *callback
	args          []reflect.Value
	isUnsubscribe bool
	err           Error
}

type serviceRegistry map[string]*service //服务收集
type callbacks map[string]*callback      //RPC回调集合
type subscriptions map[string]*callback  //认购回拨集合

//服务器表示RPC服务器
type Server struct {
	services serviceRegistry

	run      int32
	codecsMu sync.Mutex
	codecs   mapset.Set
}

//rpc request表示原始传入的rpc请求
type rpcRequest struct {
	service  string
	method   string
	id       interface{}
	isPubSub bool
	params   interface{}
err      Error //批元素无效
}

//错误包装了RPC错误，其中除消息外还包含错误代码。
type Error interface {
Error() string  //返回消息
ErrorCode() int //返回代码
}

//ServerCodec实现对服务器端的RPC消息的读取、分析和写入
//一个RPC会话。由于可以调用编解码器，因此实现必须是安全的执行例程。
//同时执行多个go例程。
type ServerCodec interface {
//阅读下一个请求
	ReadRequestHeaders() ([]rpcRequest, bool, Error)
//将请求参数解析为给定类型
	ParseRequestArguments(argTypes []reflect.Type, params interface{}) ([]reflect.Value, Error)
//组装成功响应，期望响应ID和有效负载
	CreateResponse(id interface{}, reply interface{}) interface{}
//组装错误响应，需要响应ID和错误
	CreateErrorResponse(id interface{}, err Error) interface{}
//使用有关错误的额外信息通过信息组装错误响应
	CreateErrorResponseWithInfo(id interface{}, err Error, info interface{}) interface{}
//
	CreateNotification(id, namespace string, event interface{}) interface{}
//将消息写入客户端。
	Write(msg interface{}) error
//关闭基础数据流
	Close()
//当基础连接关闭时关闭
	Closed() <-chan interface{}
}

type BlockNumber int64

const (
	PendingBlockNumber  = BlockNumber(-2)
	LatestBlockNumber   = BlockNumber(-1)
	EarliestBlockNumber = BlockNumber(0)
)

//unmashaljson将给定的json片段解析为一个blocknumber。它支持：
//-“最新”、“最早”或“挂起”作为字符串参数
//-区块编号
//返回的错误：
//-当给定参数不是已知字符串时出现无效的块号错误
//-当给定的块号太小或太大时出现超出范围的错误。
func (bn *BlockNumber) UnmarshalJSON(data []byte) error {
	input := strings.TrimSpace(string(data))
	if len(input) >= 2 && input[0] == '"' && input[len(input)-1] == '"' {
		input = input[1 : len(input)-1]
	}

	switch input {
	case "earliest":
		*bn = EarliestBlockNumber
		return nil
	case "latest":
		*bn = LatestBlockNumber
		return nil
	case "pending":
		*bn = PendingBlockNumber
		return nil
	}

	blckNum, err := hexutil.DecodeUint64(input)
	if err != nil {
		return err
	}
	if blckNum > math.MaxInt64 {
		return fmt.Errorf("Blocknumber too high")
	}

	*bn = BlockNumber(blckNum)
	return nil
}

func (bn BlockNumber) Int64() int64 {
	return (int64)(bn)
}
