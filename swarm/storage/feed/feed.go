
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
	"hash"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

//源表示特定用户对主题的更新流
type Feed struct {
	Topic Topic          `json:"topic"`
	User  common.Address `json:"user"`
}

//饲料布局：
//TopicLength字节
//useraddr common.addresslength字节
const feedLength = TopicLength + common.AddressLength

//mapkey计算此源的唯一ID。由“handler”中的缓存映射使用
func (f *Feed) mapKey() uint64 {
	serializedData := make([]byte, feedLength)
	f.binaryPut(serializedData)
	hasher := hashPool.Get().(hash.Hash)
	defer hashPool.Put(hasher)
	hasher.Reset()
	hasher.Write(serializedData)
	hash := hasher.Sum(nil)
	return *(*uint64)(unsafe.Pointer(&hash[0]))
}

//BinaryPut将此源实例序列化到提供的切片中
func (f *Feed) binaryPut(serializedData []byte) error {
	if len(serializedData) != feedLength {
		return NewErrorf(ErrInvalidValue, "Incorrect slice size to serialize feed. Expected %d, got %d", feedLength, len(serializedData))
	}
	var cursor int
	copy(serializedData[cursor:cursor+TopicLength], f.Topic[:TopicLength])
	cursor += TopicLength

	copy(serializedData[cursor:cursor+common.AddressLength], f.User[:])
	cursor += common.AddressLength

	return nil
}

//BinaryLength返回序列化时此结构的预期大小
func (f *Feed) binaryLength() int {
	return feedLength
}

//binaryget从传递的切片中包含的信息还原当前实例
func (f *Feed) binaryGet(serializedData []byte) error {
	if len(serializedData) != feedLength {
		return NewErrorf(ErrInvalidValue, "Incorrect slice size to read feed. Expected %d, got %d", feedLength, len(serializedData))
	}

	var cursor int
	copy(f.Topic[:], serializedData[cursor:cursor+TopicLength])
	cursor += TopicLength

	copy(f.User[:], serializedData[cursor:cursor+common.AddressLength])
	cursor += common.AddressLength

	return nil
}

//十六进制将提要序列化为十六进制字符串
func (f *Feed) Hex() string {
	serializedData := make([]byte, feedLength)
	f.binaryPut(serializedData)
	return hexutil.Encode(serializedData)
}

//FromValues从字符串键值存储中反序列化此实例
//用于分析查询字符串
func (f *Feed) FromValues(values Values) (err error) {
	topic := values.Get("topic")
	if topic != "" {
		if err := f.Topic.FromHex(values.Get("topic")); err != nil {
			return err
		}
} else { //查看用户集名称和相关内容
		name := values.Get("name")
		relatedContent, _ := hexutil.Decode(values.Get("relatedcontent"))
		if len(relatedContent) > 0 {
			if len(relatedContent) < storage.AddressLength {
				return NewErrorf(ErrInvalidValue, "relatedcontent field must be a hex-encoded byte array exactly %d bytes long", storage.AddressLength)
			}
			relatedContent = relatedContent[:storage.AddressLength]
		}
		f.Topic, err = NewTopic(name, relatedContent)
		if err != nil {
			return err
		}
	}
	f.User = common.HexToAddress(values.Get("user"))
	return nil
}

//AppendValues将此结构序列化到提供的字符串键值存储区中
//用于生成查询字符串
func (f *Feed) AppendValues(values Values) {
	values.Set("topic", f.Topic.Hex())
	values.Set("user", f.User.Hex())
}
