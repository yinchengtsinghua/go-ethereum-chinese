
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
	"encoding/json"
	"time"
)

//TimestampProvider设置源包的时间源
var TimestampProvider timestampProvider = NewDefaultTimestampProvider()

//timestamp将时间点编码为unix epoch
type Timestamp struct {
Time uint64 `json:"time"` //unix epoch时间戳（秒）
}

//TimestampProvider接口描述时间戳信息的来源
type timestampProvider interface {
Now() Timestamp //返回当前时间戳信息
}

//unmashaljson实现json.unmarshaller接口
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &t.Time)
}

//marshaljson实现json.marshaller接口
func (t *Timestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Time)
}

//DefaultTimestampProvider是使用系统时间的TimestampProvider
//作为时间来源
type DefaultTimestampProvider struct {
}

//NewDefaultTimestampProvider创建基于系统时钟的时间戳提供程序
func NewDefaultTimestampProvider() *DefaultTimestampProvider {
	return &DefaultTimestampProvider{}
}

//现在根据此提供程序返回当前时间
func (dtp *DefaultTimestampProvider) Now() Timestamp {
	return Timestamp{
		Time: uint64(time.Now().Unix()),
	}
}
