
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

package feed

import (
	"fmt"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/storage/feed/lookup"
)

//查询用于在执行更新查找时指定约束
//TimeLimit表示搜索的上限。设置为0表示“现在”
type Query struct {
	Feed
	Hint      lookup.Epoch
	TimeLimit uint64
}

//FromValues从字符串键值存储中反序列化此实例
//用于分析查询字符串
func (q *Query) FromValues(values Values) error {
	time, _ := strconv.ParseUint(values.Get("time"), 10, 64)
	q.TimeLimit = time

	level, _ := strconv.ParseUint(values.Get("hint.level"), 10, 32)
	q.Hint.Level = uint8(level)
	q.Hint.Time, _ = strconv.ParseUint(values.Get("hint.time"), 10, 64)
	if q.Feed.User == (common.Address{}) {
		return q.Feed.FromValues(values)
	}
	return nil
}

//AppendValues将此结构序列化到提供的字符串键值存储区中
//用于生成查询字符串
func (q *Query) AppendValues(values Values) {
	if q.TimeLimit != 0 {
		values.Set("time", fmt.Sprintf("%d", q.TimeLimit))
	}
	if q.Hint.Level != 0 {
		values.Set("hint.level", fmt.Sprintf("%d", q.Hint.Level))
	}
	if q.Hint.Time != 0 {
		values.Set("hint.time", fmt.Sprintf("%d", q.Hint.Time))
	}
	q.Feed.AppendValues(values)
}

//newquery构造一个查询结构以在“time”或之前查找更新
//如果time==0，将查找最新更新
func NewQuery(feed *Feed, time uint64, hint lookup.Epoch) *Query {
	return &Query{
		TimeLimit: time,
		Feed:      *feed,
		Hint:      hint,
	}
}

//newquerylatest生成查找参数，以查找源的最新更新
func NewQueryLatest(feed *Feed, hint lookup.Epoch) *Query {
	return NewQuery(feed, 0, hint)
}
