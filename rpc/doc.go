
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

/*
包RPC提供通过网络访问对象的导出方法的权限
或其他I/O连接。创建服务器实例后，可以注册对象，
使其从外部可见。遵循特定的导出方法
可以远程调用约定。它还支持发布/订阅
模式。

满足以下条件的方法可用于远程访问：
 -必须导出对象
 -必须导出方法
 -方法返回0、1（响应或错误）或2（响应和错误）值
 —方法参数必须导出或内置类型
 -方法返回值必须导出或内置类型

示例方法：
 func（s*calcservice）add（a，b int）（int，error）

当返回的错误不是nil时，将忽略返回的整数，错误为
发送回客户端。否则，返回的整数将被发送回客户端。

接受指针值作为参数支持可选参数。例如。
如果我们想在一个可选的有限域中做加法，我们可以接受一个mod
参数作为指针值。

 func（s*calservice）add（a，b int，mod*int）（int，错误）

可以使用2个整数和一个空值作为第三个参数调用此rpc方法。
在这种情况下，mod参数将为零。或者可以用3个整数来调用，
在这种情况下，mod将指向给定的第三个参数。因为可选
参数是rpc包还将接受2个整数作为
争论。它将把mod参数作为nil传递给rpc方法。

服务器提供了接受ServerCodec实例的servedec方法。它将
从编解码器读取请求，处理请求并将响应发送回
使用编解码器的客户端。服务器可以同时执行请求。响应
可以按顺序发送回客户端。

使用JSON编解码器的示例服务器：
 类型计算器服务结构

 func（s*计算器服务）添加（a，b int）int
 返回A+B
 }

 func（s*calculatorservice）div（a，b int）（int，error）
 如果B=＝0 {
  返回0，errors.new（“被零除”）。
 }
 返回A/B，NIL
 }

 计算器：=新建（CalculatorService）
 服务器：=newserver（）
 server.registername（“计算器”，计算器）

 l，：=net.listenunix（“unix”，&net.unixaddr net:“unix”，name:“/tmp/calculator.sock”）
 对于{
 c，：=l.acceptUnix（）。
 编解码器：=v2.newjsoncodec（c）
 go server.servedec（编解码器）
 }

包还通过使用订阅支持发布订阅模式。
被视为符合通知条件的方法必须满足以下条件：
 -必须导出对象
 -必须导出方法
 -第一个方法参数类型必须是Context.Context
 —方法参数必须导出或内置类型
 -方法必须返回元组订阅，错误

示例方法：
 func（s*blockchainservice）newblock（ctx context.context）（订阅，错误）
  …
 }

订阅在以下情况下被删除：
 -用户发送取消订阅请求
 -用于创建订阅的连接已关闭。这可以启动
   通过客户端和服务器。服务器将在发生写入错误或
   缓冲通知队列太大。
**/

package rpc
