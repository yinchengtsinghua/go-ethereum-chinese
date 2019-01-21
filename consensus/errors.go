
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

package consensus

import "errors"

var (
//当验证块需要祖先时返回errUnknownancestor
//这是未知的。
	ErrUnknownAncestor = errors.New("unknown ancestor")

//验证块需要祖先时返回errprunedancestor
//这是已知的，但其状态不可用。
	ErrPrunedAncestor = errors.New("pruned ancestor")

//当块的时间戳在将来时，根据
//到当前节点。
	ErrFutureBlock = errors.New("block in the future")

//如果块的编号不等于其父块的编号，则返回errInvalidNumber。
//加一。
	ErrInvalidNumber = errors.New("invalid block number")
)
