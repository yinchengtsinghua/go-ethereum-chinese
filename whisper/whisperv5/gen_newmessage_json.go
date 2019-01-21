
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//代码由github.com/fjl/gencodec生成。不要编辑。

package whisperv5

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

var _ = (*newMessageOverride)(nil)

func (n NewMessage) MarshalJSON() ([]byte, error) {
	type NewMessage struct {
		SymKeyID   string        `json:"symKeyID"`
		PublicKey  hexutil.Bytes `json:"pubKey"`
		Sig        string        `json:"sig"`
		TTL        uint32        `json:"ttl"`
		Topic      TopicType     `json:"topic"`
		Payload    hexutil.Bytes `json:"payload"`
		Padding    hexutil.Bytes `json:"padding"`
		PowTime    uint32        `json:"powTime"`
		PowTarget  float64       `json:"powTarget"`
		TargetPeer string        `json:"targetPeer"`
	}
	var enc NewMessage
	enc.SymKeyID = n.SymKeyID
	enc.PublicKey = n.PublicKey
	enc.Sig = n.Sig
	enc.TTL = n.TTL
	enc.Topic = n.Topic
	enc.Payload = n.Payload
	enc.Padding = n.Padding
	enc.PowTime = n.PowTime
	enc.PowTarget = n.PowTarget
	enc.TargetPeer = n.TargetPeer
	return json.Marshal(&enc)
}

func (n *NewMessage) UnmarshalJSON(input []byte) error {
	type NewMessage struct {
		SymKeyID   *string        `json:"symKeyID"`
		PublicKey  *hexutil.Bytes `json:"pubKey"`
		Sig        *string        `json:"sig"`
		TTL        *uint32        `json:"ttl"`
		Topic      *TopicType     `json:"topic"`
		Payload    *hexutil.Bytes `json:"payload"`
		Padding    *hexutil.Bytes `json:"padding"`
		PowTime    *uint32        `json:"powTime"`
		PowTarget  *float64       `json:"powTarget"`
		TargetPeer *string        `json:"targetPeer"`
	}
	var dec NewMessage
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.SymKeyID != nil {
		n.SymKeyID = *dec.SymKeyID
	}
	if dec.PublicKey != nil {
		n.PublicKey = *dec.PublicKey
	}
	if dec.Sig != nil {
		n.Sig = *dec.Sig
	}
	if dec.TTL != nil {
		n.TTL = *dec.TTL
	}
	if dec.Topic != nil {
		n.Topic = *dec.Topic
	}
	if dec.Payload != nil {
		n.Payload = *dec.Payload
	}
	if dec.Padding != nil {
		n.Padding = *dec.Padding
	}
	if dec.PowTime != nil {
		n.PowTime = *dec.PowTime
	}
	if dec.PowTarget != nil {
		n.PowTarget = *dec.PowTarget
	}
	if dec.TargetPeer != nil {
		n.TargetPeer = *dec.TargetPeer
	}
	return nil
}
