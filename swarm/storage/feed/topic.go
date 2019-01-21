
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
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common/bitutil"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

//topic length建立主题字符串的最大长度
const TopicLength = storage.AddressLength

//主题表示提要的内容
type Topic [TopicLength]byte

//创建名称/相关内容太长的主题时返回errtopictolong
var ErrTopicTooLong = fmt.Errorf("Topic is too long. Max length is %d", TopicLength)

//new topic从提供的名称和“相关内容”字节数组创建新主题，
//将两者合并在一起。
//如果RelatedContent或Name长于TopicLength，它们将被截断并返回错误
//名称可以是空字符串
//相关内容可以为零
func NewTopic(name string, relatedContent []byte) (topic Topic, err error) {
	if relatedContent != nil {
		contentLength := len(relatedContent)
		if contentLength > TopicLength {
			contentLength = TopicLength
			err = ErrTopicTooLong
		}
		copy(topic[:], relatedContent[:contentLength])
	}
	nameBytes := []byte(name)
	nameLength := len(nameBytes)
	if nameLength > TopicLength {
		nameLength = TopicLength
		err = ErrTopicTooLong
	}
	bitutil.XORBytes(topic[:], topic[:], nameBytes[:nameLength])
	return topic, err
}

//hex将返回编码为十六进制字符串的主题
func (t *Topic) Hex() string {
	return hexutil.Encode(t[:])
}

//FromHex将把十六进制字符串解析到此主题实例中
func (t *Topic) FromHex(hex string) error {
	bytes, err := hexutil.Decode(hex)
	if err != nil || len(bytes) != len(t) {
		return NewErrorf(ErrInvalidValue, "Cannot decode topic")
	}
	copy(t[:], bytes)
	return nil
}

//name将尝试从主题中提取主题名称
func (t *Topic) Name(relatedContent []byte) string {
	nameBytes := *t
	if relatedContent != nil {
		contentLength := len(relatedContent)
		if contentLength > TopicLength {
			contentLength = TopicLength
		}
		bitutil.XORBytes(nameBytes[:], t[:], relatedContent[:contentLength])
	}
	z := bytes.IndexByte(nameBytes[:], 0)
	if z < 0 {
		z = TopicLength
	}
	return string(nameBytes[:z])

}

//unmashaljson实现json.unmarshaller接口
func (t *Topic) UnmarshalJSON(data []byte) error {
	var hex string
	json.Unmarshal(data, &hex)
	return t.FromHex(hex)
}

//marshaljson实现json.marshaller接口
func (t *Topic) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Hex())
}
