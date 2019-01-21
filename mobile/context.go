
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

//包含golang.org/x/net/context包中要支持的所有包装器
//移动平台上的客户端上下文管理。

package geth

import (
	"context"
	"time"
)

//上下文在API中包含截止日期、取消信号和其他值。
//边界。
type Context struct {
	context context.Context
	cancel  context.CancelFunc
}

//NewContext返回非零的空上下文。从来没有取消过，没有
//价值观，没有最后期限。它通常由主功能使用，
//初始化和测试，以及作为传入请求的顶级上下文。
func NewContext() *Context {
	return &Context{
		context: context.Background(),
	}
}

//withcancel返回具有取消机制的原始上下文的副本
//包括。
//
//取消此上下文将释放与其关联的资源，因此代码应该
//在此上下文中运行的操作完成后立即调用Cancel。
func (c *Context) WithCancel() *Context {
	child, cancel := context.WithCancel(c.context)
	return &Context{
		context: child,
		cancel:  cancel,
	}
}

//WithDeadline返回原始上下文的副本，调整了截止时间
//不迟于规定时间。
//
//取消此上下文将释放与其关联的资源，因此代码应该
//在此上下文中运行的操作完成后立即调用Cancel。
func (c *Context) WithDeadline(sec int64, nsec int64) *Context {
	child, cancel := context.WithDeadline(c.context, time.Unix(sec, nsec))
	return &Context{
		context: child,
		cancel:  cancel,
	}
}

//WithTimeout返回原始上下文的副本，并调整最后期限
//不迟于现在+指定的持续时间。
//
//取消此上下文将释放与其关联的资源，因此代码应该
//在此上下文中运行的操作完成后立即调用Cancel。
func (c *Context) WithTimeout(nsec int64) *Context {
	child, cancel := context.WithTimeout(c.context, time.Duration(nsec))
	return &Context{
		context: child,
		cancel:  cancel,
	}
}
