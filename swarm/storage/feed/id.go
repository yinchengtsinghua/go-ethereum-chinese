
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
	"hash"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/storage/feed/lookup"

	"github.com/ethereum/go-ethereum/swarm/storage"
)

//ID唯一标识网络上的更新。
type ID struct {
	Feed         `json:"feed"`
	lookup.Epoch `json:"epoch"`
}

//ID布局：
//进纸长度字节
//时代纪元
const idLength = feedLength + lookup.EpochLength

//addr计算与此ID对应的源更新块地址
func (u *ID) Addr() (updateAddr storage.Address) {
	serializedData := make([]byte, idLength)
	var cursor int
	u.Feed.binaryPut(serializedData[cursor : cursor+feedLength])
	cursor += feedLength

	eid := u.Epoch.ID()
	copy(serializedData[cursor:cursor+lookup.EpochLength], eid[:])

	hasher := hashPool.Get().(hash.Hash)
	defer hashPool.Put(hasher)
	hasher.Reset()
	hasher.Write(serializedData)
	return hasher.Sum(nil)
}

//BinaryPut将此实例序列化到提供的切片中
func (u *ID) binaryPut(serializedData []byte) error {
	if len(serializedData) != idLength {
		return NewErrorf(ErrInvalidValue, "Incorrect slice size to serialize ID. Expected %d, got %d", idLength, len(serializedData))
	}
	var cursor int
	if err := u.Feed.binaryPut(serializedData[cursor : cursor+feedLength]); err != nil {
		return err
	}
	cursor += feedLength

	epochBytes, err := u.Epoch.MarshalBinary()
	if err != nil {
		return err
	}
	copy(serializedData[cursor:cursor+lookup.EpochLength], epochBytes[:])
	cursor += lookup.EpochLength

	return nil
}

//BinaryLength返回序列化时此结构的预期大小
func (u *ID) binaryLength() int {
	return idLength
}

//binaryget从传递的切片中包含的信息还原当前实例
func (u *ID) binaryGet(serializedData []byte) error {
	if len(serializedData) != idLength {
		return NewErrorf(ErrInvalidValue, "Incorrect slice size to read ID. Expected %d, got %d", idLength, len(serializedData))
	}

	var cursor int
	if err := u.Feed.binaryGet(serializedData[cursor : cursor+feedLength]); err != nil {
		return err
	}
	cursor += feedLength

	if err := u.Epoch.UnmarshalBinary(serializedData[cursor : cursor+lookup.EpochLength]); err != nil {
		return err
	}
	cursor += lookup.EpochLength

	return nil
}

//FromValues从字符串键值存储中反序列化此实例
//用于分析查询字符串
func (u *ID) FromValues(values Values) error {
	level, _ := strconv.ParseUint(values.Get("level"), 10, 32)
	u.Epoch.Level = uint8(level)
	u.Epoch.Time, _ = strconv.ParseUint(values.Get("time"), 10, 64)

	if u.Feed.User == (common.Address{}) {
		return u.Feed.FromValues(values)
	}
	return nil
}

//AppendValues将此结构序列化到提供的字符串键值存储区中
//用于生成查询字符串
func (u *ID) AppendValues(values Values) {
	values.Set("level", fmt.Sprintf("%d", u.Epoch.Level))
	values.Set("time", fmt.Sprintf("%d", u.Epoch.Time))
	u.Feed.AppendValues(values)
}
