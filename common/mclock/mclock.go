
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

//包mclock是单调时钟源的包装器
package mclock

import (
	"time"

	"github.com/aristanetworks/goarista/monotime"
)

//Abstime代表绝对单调时间。
type AbsTime time.Duration

//现在返回当前绝对单调时间。
func Now() AbsTime {
	return AbsTime(monotime.Now())
}

//添加返回t+d。
func (t AbsTime) Add(d time.Duration) AbsTime {
	return t + AbsTime(d)
}

//时钟接口使得用
//模拟时钟。
type Clock interface {
	Now() AbsTime
	Sleep(time.Duration)
	After(time.Duration) <-chan time.Time
}

//系统使用系统时钟实现时钟。
type System struct{}

//现在实现时钟。
func (System) Now() AbsTime {
	return AbsTime(monotime.Now())
}

//睡眠实现时钟。
func (System) Sleep(d time.Duration) {
	time.Sleep(d)
}

//在执行时钟之后。
func (System) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}
