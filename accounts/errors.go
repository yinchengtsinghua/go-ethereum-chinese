
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

package accounts

import (
	"errors"
	"fmt"
)

//对于没有后端的任何请求操作，将返回errUnknownAccount。
//提供指定的帐户。
var ErrUnknownAccount = errors.New("unknown account")

//对于没有后端的任何请求操作，将返回errunknownwallet。
//提供指定的钱包。
var ErrUnknownWallet = errors.New("unknown wallet")

//从帐户请求操作时返回errnotsupported
//它不支持的后端。
var ErrNotSupported = errors.New("not supported")

//当解密操作收到错误消息时返回errInvalidPassphrase
//口令。
var ErrInvalidPassphrase = errors.New("invalid passphrase")

//如果试图打开钱包，则返回errwalletalreadyopen
//第二次。
var ErrWalletAlreadyOpen = errors.New("wallet already open")

//如果试图打开钱包，则返回errWalletClosed
//间隔时间。
var ErrWalletClosed = errors.New("wallet closed")

//后端返回authneedederror，用于在用户
//在签名成功之前需要提供进一步的身份验证。
//
//这通常意味着要么需要提供密码，要么
//某些硬件设备显示的一次性PIN码。
type AuthNeededError struct {
Needed string //用户需要提供的额外身份验证
}

//newauthneedederror创建一个新的身份验证错误，并提供额外的详细信息
//关于所需字段集。
func NewAuthNeededError(needed string) error {
	return &AuthNeededError{
		Needed: needed,
	}
}

//错误实现标准错误接口。
func (err *AuthNeededError) Error() string {
	return fmt.Sprintf("authentication needed: %s", err.Needed)
}
