
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

package fdlimit

import "errors"

//raise尝试最大化此进程的文件描述符允许量
//达到操作系统允许的最大硬限制。
func Raise(max uint64) error {
//该方法设计为NOP：
//*Linux/Darwin对应程序需要手动增加每个进程的限制
//*在Windows上，Go使用CreateFile API，该API仅限于16K个文件，非
//可从正在运行的进程中更改
//这样，我们就可以“请求”提高限额，这两种情况都有
//或者基于我们运行的平台没有效果。
	if max > 16384 {
		return errors.New("file descriptor limit (16384) reached")
	}
	return nil
}

//当前检索允许由此打开的文件描述符数
//过程。
func Current() (int, error) {
//请参阅“加薪”，了解我们为什么使用硬编码16K作为限制的原因。
	return 16384, nil
}

//最大检索此进程的最大文件描述符数
//允许自己请求。
func Maximum() (int, error) {
	return Current()
}
