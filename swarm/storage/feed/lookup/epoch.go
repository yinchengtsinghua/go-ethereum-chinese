
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

package lookup

import (
	"encoding/binary"
	"errors"
	"fmt"
)

//epoch表示特定频率级别的时隙
type Epoch struct {
Time  uint64 `json:"time"`  //时间存储更新或查找发生的时间
Level uint8  `json:"level"` //级别表示频率级别，表示2次方的指数。
}

//epochid是一个时代的唯一标识符，基于它的级别和基准时间。
type EpochID [8]byte

//epoch length存储epoch的序列化二进制长度
const EpochLength = 8

//maxtime包含一个epoch可以处理的最大可能时间值
const MaxTime uint64 = (1 << 56) - 1

//BASE返回纪元的基准时间
func (e *Epoch) Base() uint64 {
	return getBaseTime(e.Time, e.Level)
}

//id返回这个时代的唯一标识符
func (e *Epoch) ID() EpochID {
	base := e.Base()
	var id EpochID
	binary.LittleEndian.PutUint64(id[:], base)
	id[7] = e.Level
	return id
}

//MarshalBinary实现Encoding.BinaryMarshaller接口
func (e *Epoch) MarshalBinary() (data []byte, err error) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b[:], e.Time)
	b[7] = e.Level
	return b, nil
}

//UnmarshalBinary实现encoding.BinaryUnmarshaller接口
func (e *Epoch) UnmarshalBinary(data []byte) error {
	if len(data) != EpochLength {
		return errors.New("Invalid data unmarshalling Epoch")
	}
	b := make([]byte, 8)
	copy(b, data)
	e.Level = b[7]
	b[7] = 0
	e.Time = binary.LittleEndian.Uint64(b)
	return nil
}

//如果此纪元发生在另一个纪元之后或正好发生在另一个纪元，则返回true。
func (e *Epoch) After(epoch Epoch) bool {
	if e.Time == epoch.Time {
		return e.Level < epoch.Level
	}
	return e.Time >= epoch.Time
}

//equals比较两个时期，如果它们引用同一时间段，则返回true。
func (e *Epoch) Equals(epoch Epoch) bool {
	return e.Level == epoch.Level && e.Base() == epoch.Base()
}

//字符串实现字符串接口。
func (e *Epoch) String() string {
	return fmt.Sprintf("Epoch{Time:%d, Level:%d}", e.Time, e.Level)
}
