
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

//包含Whisper协议信封元素。

package whisperv6

import (
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	gmath "math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/ethereum/go-ethereum/rlp"
)

//信封表示一个明文数据包，通过耳语进行传输。
//网络。其内容可以加密或不加密和签名。
type Envelope struct {
	Expiry uint32
	TTL    uint32
	Topic  TopicType
	Data   []byte
	Nonce  uint64

pow float64 //消息特定的POW，如Whisper规范中所述。

//不应直接访问以下变量，请改用相应的函数：hash（）、bloom（）。
hash  common.Hash //信封的缓存哈希，以避免每次重新刷新。
	bloom []byte
}

//大小返回信封发送时的大小（即仅公共字段）
func (e *Envelope) size() int {
	return EnvelopeHeaderLength + len(e.Data)
}

//rlpwithoutnonce返回rlp编码的信封内容，nonce除外。
func (e *Envelope) rlpWithoutNonce() []byte {
	res, _ := rlp.EncodeToBytes([]interface{}{e.Expiry, e.TTL, e.Topic, e.Data})
	return res
}

//newenvelope用过期和目标数据包装了一条私语消息
//包含在信封中，用于网络转发。
func NewEnvelope(ttl uint32, topic TopicType, msg *sentMessage) *Envelope {
	env := Envelope{
		Expiry: uint32(time.Now().Add(time.Second * time.Duration(ttl)).Unix()),
		TTL:    ttl,
		Topic:  topic,
		Data:   msg.Raw,
		Nonce:  0,
	}

	return &env
}

//Seal通过花费所需的时间作为证据来关闭信封
//关于散列数据的工作。
func (e *Envelope) Seal(options *MessageParams) error {
	if options.PoW == 0 {
//不需要POW
		return nil
	}

	var target, bestBit int
	if options.PoW < 0 {
//未设置目标-函数应运行一段时间
//工作时间参数中指定的时间。因为我们可以预测
//执行时间，我们也可以调整到期时间。
		e.Expiry += options.WorkTime
	} else {
		target = e.powToFirstBit(options.PoW)
	}

	buf := make([]byte, 64)
	h := crypto.Keccak256(e.rlpWithoutNonce())
	copy(buf[:32], h)

	finish := time.Now().Add(time.Duration(options.WorkTime) * time.Second).UnixNano()
	for nonce := uint64(0); time.Now().UnixNano() < finish; {
		for i := 0; i < 1024; i++ {
			binary.BigEndian.PutUint64(buf[56:], nonce)
			d := new(big.Int).SetBytes(crypto.Keccak256(buf))
			firstBit := math.FirstBitSet(d)
			if firstBit > bestBit {
				e.Nonce, bestBit = nonce, firstBit
				if target > 0 && bestBit >= target {
					return nil
				}
			}
			nonce++
		}
	}

	if target > 0 && bestBit < target {
		return fmt.Errorf("failed to reach the PoW target, specified pow time (%d seconds) was insufficient", options.WorkTime)
	}

	return nil
}

//POW计算（如有必要）并返回工作证明目标
//信封的。
func (e *Envelope) PoW() float64 {
	if e.pow == 0 {
		e.calculatePoW(0)
	}
	return e.pow
}

func (e *Envelope) calculatePoW(diff uint32) {
	buf := make([]byte, 64)
	h := crypto.Keccak256(e.rlpWithoutNonce())
	copy(buf[:32], h)
	binary.BigEndian.PutUint64(buf[56:], e.Nonce)
	d := new(big.Int).SetBytes(crypto.Keccak256(buf))
	firstBit := math.FirstBitSet(d)
	x := gmath.Pow(2, float64(firstBit))
	x /= float64(e.size())
	x /= float64(e.TTL + diff)
	e.pow = x
}

func (e *Envelope) powToFirstBit(pow float64) int {
	x := pow
	x *= float64(e.size())
	x *= float64(e.TTL)
	bits := gmath.Log2(x)
	bits = gmath.Ceil(bits)
	res := int(bits)
	if res < 1 {
		res = 1
	}
	return res
}

//hash返回信封的sha3散列，如果还没有完成，则计算它。
func (e *Envelope) Hash() common.Hash {
	if (e.hash == common.Hash{}) {
		encoded, _ := rlp.EncodeToBytes(e)
		e.hash = crypto.Keccak256Hash(encoded)
	}
	return e.hash
}

//decoderlp从rlp数据流解码信封。
func (e *Envelope) DecodeRLP(s *rlp.Stream) error {
	raw, err := s.Raw()
	if err != nil {
		return err
	}
//信封的解码使用结构字段，但也需要
//计算整个RLP编码信封的散列值。这个
//类型具有与信封相同的结构，但不是
//rlp.decoder（不实现decoderlp函数）。
//只对公共成员进行编码。
	type rlpenv Envelope
	if err := rlp.DecodeBytes(raw, (*rlpenv)(e)); err != nil {
		return err
	}
	e.hash = crypto.Keccak256Hash(raw)
	return nil
}

//OpenAsymmetric试图解密一个信封，可能用一个特定的密钥加密。
func (e *Envelope) OpenAsymmetric(key *ecdsa.PrivateKey) (*ReceivedMessage, error) {
	message := &ReceivedMessage{Raw: e.Data}
	err := message.decryptAsymmetric(key)
	switch err {
	case nil:
		return message, nil
case ecies.ErrInvalidPublicKey: //写给别人的
		return nil, err
	default:
		return nil, fmt.Errorf("unable to open envelope, decrypt failed: %v", err)
	}
}

//opensymmetric试图解密一个可能用特定密钥加密的信封。
func (e *Envelope) OpenSymmetric(key []byte) (msg *ReceivedMessage, err error) {
	msg = &ReceivedMessage{Raw: e.Data}
	err = msg.decryptSymmetric(key)
	if err != nil {
		msg = nil
	}
	return msg, err
}

//open试图解密信封，并在成功时填充消息字段。
func (e *Envelope) Open(watcher *Filter) (msg *ReceivedMessage) {
	if watcher == nil {
		return nil
	}

//API接口禁止过滤器同时进行对称和非对称加密。
	if watcher.expectsAsymmetricEncryption() && watcher.expectsSymmetricEncryption() {
		return nil
	}

	if watcher.expectsAsymmetricEncryption() {
		msg, _ = e.OpenAsymmetric(watcher.KeyAsym)
		if msg != nil {
			msg.Dst = &watcher.KeyAsym.PublicKey
		}
	} else if watcher.expectsSymmetricEncryption() {
		msg, _ = e.OpenSymmetric(watcher.KeySym)
		if msg != nil {
			msg.SymKeyHash = crypto.Keccak256Hash(watcher.KeySym)
		}
	}

	if msg != nil {
		ok := msg.ValidateAndParse()
		if !ok {
			return nil
		}
		msg.Topic = e.Topic
		msg.PoW = e.PoW()
		msg.TTL = e.TTL
		msg.Sent = e.Expiry - e.TTL
		msg.EnvelopeHash = e.Hash()
	}
	return msg
}

//Bloom将4字节主题映射到64字节的Bloom过滤器中，并设置了3位（最多）。
func (e *Envelope) Bloom() []byte {
	if e.bloom == nil {
		e.bloom = TopicToBloom(e.Topic)
	}
	return e.bloom
}

//TopicToBloom将主题（4字节）转换为Bloom筛选器（64字节）
func TopicToBloom(topic TopicType) []byte {
	b := make([]byte, BloomFilterSize)
	var index [3]int
	for j := 0; j < 3; j++ {
		index[j] = int(topic[j])
		if (topic[3] & (1 << uint(j))) != 0 {
			index[j] += 256
		}
	}

	for j := 0; j < 3; j++ {
		byteIndex := index[j] / 8
		bitIndex := index[j] % 8
		b[byteIndex] = (1 << uint(bitIndex))
	}
	return b
}

//GetEnvelope通过其哈希从消息队列中检索信封。
//如果找不到信封，则返回零。
func (w *Whisper) GetEnvelope(hash common.Hash) *Envelope {
	w.poolMu.RLock()
	defer w.poolMu.RUnlock()
	return w.envelopes[hash]
}
