
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//代码由github.com/fjl/gencodec生成。不要编辑。

package whisperv6

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

var _ = (*messageOverride)(nil)

//marshaljson将类型消息封送到json字符串
func (m Message) MarshalJSON() ([]byte, error) {
	type Message struct {
		Sig       hexutil.Bytes `json:"sig,omitempty"`
		TTL       uint32        `json:"ttl"`
		Timestamp uint32        `json:"timestamp"`
		Topic     TopicType     `json:"topic"`
		Payload   hexutil.Bytes `json:"payload"`
		Padding   hexutil.Bytes `json:"padding"`
		PoW       float64       `json:"pow"`
		Hash      hexutil.Bytes `json:"hash"`
		Dst       hexutil.Bytes `json:"recipientPublicKey,omitempty"`
	}
	var enc Message
	enc.Sig = m.Sig
	enc.TTL = m.TTL
	enc.Timestamp = m.Timestamp
	enc.Topic = m.Topic
	enc.Payload = m.Payload
	enc.Padding = m.Padding
	enc.PoW = m.PoW
	enc.Hash = m.Hash
	enc.Dst = m.Dst
	return json.Marshal(&enc)
}

//将类型消息解封为JSON字符串
func (m *Message) UnmarshalJSON(input []byte) error {
	type Message struct {
		Sig       *hexutil.Bytes `json:"sig,omitempty"`
		TTL       *uint32        `json:"ttl"`
		Timestamp *uint32        `json:"timestamp"`
		Topic     *TopicType     `json:"topic"`
		Payload   *hexutil.Bytes `json:"payload"`
		Padding   *hexutil.Bytes `json:"padding"`
		PoW       *float64       `json:"pow"`
		Hash      *hexutil.Bytes `json:"hash"`
		Dst       *hexutil.Bytes `json:"recipientPublicKey,omitempty"`
	}
	var dec Message
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Sig != nil {
		m.Sig = *dec.Sig
	}
	if dec.TTL != nil {
		m.TTL = *dec.TTL
	}
	if dec.Timestamp != nil {
		m.Timestamp = *dec.Timestamp
	}
	if dec.Topic != nil {
		m.Topic = *dec.Topic
	}
	if dec.Payload != nil {
		m.Payload = *dec.Payload
	}
	if dec.Padding != nil {
		m.Padding = *dec.Padding
	}
	if dec.PoW != nil {
		m.PoW = *dec.PoW
	}
	if dec.Hash != nil {
		m.Hash = *dec.Hash
	}
	if dec.Dst != nil {
		m.Dst = *dec.Dst
	}
	return nil
}
