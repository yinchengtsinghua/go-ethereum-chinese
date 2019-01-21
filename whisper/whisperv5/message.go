
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

//包含耳语协议消息元素。

package whisperv5

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/ethereum/go-ethereum/log"
)

//messageparams指定将消息包装到信封中的确切方式。
type MessageParams struct {
	TTL      uint32
	Src      *ecdsa.PrivateKey
	Dst      *ecdsa.PublicKey
	KeySym   []byte
	Topic    TopicType
	WorkTime uint32
	PoW      float64
	Payload  []byte
	Padding  []byte
}

//sentmessage表示要通过
//耳语协议。它们被包装成信封，不需要
//中间节点理解，刚刚转发。
type sentMessage struct {
	Raw []byte
}

//ReceivedMessage表示要通过
//耳语协议。
type ReceivedMessage struct {
	Raw []byte

	Payload   []byte
	Padding   []byte
	Signature []byte

PoW   float64          //耳语规范中所述的工作证明
Sent  uint32           //消息发布到网络的时间
TTL   uint32           //消息允许的最长生存时间
Src   *ecdsa.PublicKey //邮件收件人（用于解码邮件的标识）
Dst   *ecdsa.PublicKey //邮件收件人（用于解码邮件的标识）
	Topic TopicType

SymKeyHash      common.Hash //与主题关联的密钥的keccak256hash
EnvelopeHash    common.Hash //作为唯一ID的消息信封哈希
	EnvelopeVersion uint64
}

func isMessageSigned(flags byte) bool {
	return (flags & signatureFlag) != 0
}

func (msg *ReceivedMessage) isSymmetricEncryption() bool {
	return msg.SymKeyHash != common.Hash{}
}

func (msg *ReceivedMessage) isAsymmetricEncryption() bool {
	return msg.Dst != nil
}

//newsentmessage创建并初始化一个未签名、未加密的悄悄消息。
func NewSentMessage(params *MessageParams) (*sentMessage, error) {
	msg := sentMessage{}
	msg.Raw = make([]byte, 1, len(params.Payload)+len(params.Padding)+signatureLength+padSizeLimit)
msg.Raw[0] = 0 //将所有标志设置为零
	err := msg.appendPadding(params)
	if err != nil {
		return nil, err
	}
	msg.Raw = append(msg.Raw, params.Payload...)
	return &msg, nil
}

//getsizeoflength返回对整个大小填充进行编码所需的字节数（包括这些字节）
func getSizeOfLength(b []byte) (sz int, err error) {
sz = intSize(len(b))      //第一次迭代
sz = intSize(len(b) + sz) //第二次迭代
	if sz > 3 {
		err = errors.New("oversized padding parameter")
	}
	return sz, err
}

//sizeofinsize返回对整数值进行编码所需的最小字节数
func intSize(i int) (s int) {
	for s = 1; i >= 256; s++ {
		i /= 256
	}
	return s
}

//AppendPadding附加伪随机填充字节并设置填充标志。
//最后一个字节包含填充大小（因此，其大小不能超过256）。
func (msg *sentMessage) appendPadding(params *MessageParams) error {
	rawSize := len(params.Payload) + 1
	if params.Src != nil {
		rawSize += signatureLength
	}
	odd := rawSize % padSizeLimit

	if len(params.Padding) != 0 {
		padSize := len(params.Padding)
		padLengthSize, err := getSizeOfLength(params.Padding)
		if err != nil {
			return err
		}
		totalPadSize := padSize + padLengthSize
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint32(buf, uint32(totalPadSize))
		buf = buf[:padLengthSize]
		msg.Raw = append(msg.Raw, buf...)
		msg.Raw = append(msg.Raw, params.Padding...)
msg.Raw[0] |= byte(padLengthSize) //指示填充大小的字节数
	} else if odd != 0 {
		totalPadSize := padSizeLimit - odd
		if totalPadSize > 255 {
//此算法仅在padsizelimit<256时有效。
//如果padsizelimit将更改，请修复算法
//（另请参见receivedmessage.extractpadding（）函数）。
			panic("please fix the padding algorithm before releasing new version")
		}
		buf := make([]byte, totalPadSize)
		_, err := crand.Read(buf[1:])
		if err != nil {
			return err
		}
		if totalPadSize > 6 && !validateSymmetricKey(buf) {
			return errors.New("failed to generate random padding of size " + strconv.Itoa(totalPadSize))
		}
		buf[0] = byte(totalPadSize)
		msg.Raw = append(msg.Raw, buf...)
msg.Raw[0] |= byte(0x1) //指示填充大小的字节数
	}
	return nil
}

//sign计算并设置消息的加密签名，
//同时设置标志标志。
func (msg *sentMessage) sign(key *ecdsa.PrivateKey) error {
	if isMessageSigned(msg.Raw[0]) {
//这不应该发生，但没有理由惊慌
		log.Error("failed to sign the message: already signed")
		return nil
	}

	msg.Raw[0] |= signatureFlag
	hash := crypto.Keccak256(msg.Raw)
	signature, err := crypto.Sign(hash, key)
	if err != nil {
msg.Raw[0] &= ^signatureFlag //清旗
		return err
	}
	msg.Raw = append(msg.Raw, signature...)
	return nil
}

//加密非对称使用公钥加密消息。
func (msg *sentMessage) encryptAsymmetric(key *ecdsa.PublicKey) error {
	if !ValidatePublicKey(key) {
		return errors.New("invalid public key provided for asymmetric encryption")
	}
	encrypted, err := ecies.Encrypt(crand.Reader, ecies.ImportECDSAPublic(key), msg.Raw, nil, nil)
	if err == nil {
		msg.Raw = encrypted
	}
	return err
}

//encryptsymmetric使用aes-gcm-256使用主题密钥对消息进行加密。
//nonce大小应为12个字节（请参阅cipher.gcmstandardnoncosize）。
func (msg *sentMessage) encryptSymmetric(key []byte) (nonce []byte, err error) {
	if !validateSymmetricKey(key) {
		return nil, errors.New("invalid key provided for symmetric encryption")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

//对于给定的键，不要使用超过2^32个随机nonce
	nonce = make([]byte, aesgcm.NonceSize())
	_, err = crand.Read(nonce)
	if err != nil {
		return nil, err
	} else if !validateSymmetricKey(nonce) {
		return nil, errors.New("crypto/rand failed to generate nonce")
	}

	msg.Raw = aesgcm.Seal(nil, nonce, msg.Raw, nil)
	return nonce, nil
}

//将消息打包成一个信封，通过网络传输。
func (msg *sentMessage) Wrap(options *MessageParams) (envelope *Envelope, err error) {
	if options.TTL == 0 {
		options.TTL = DefaultTTL
	}
	if options.Src != nil {
		if err = msg.sign(options.Src); err != nil {
			return nil, err
		}
	}
	var nonce []byte
	if options.Dst != nil {
		err = msg.encryptAsymmetric(options.Dst)
	} else if options.KeySym != nil {
		nonce, err = msg.encryptSymmetric(options.KeySym)
	} else {
		err = errors.New("unable to encrypt the message: neither symmetric nor assymmetric key provided")
	}
	if err != nil {
		return nil, err
	}

	envelope = NewEnvelope(options.TTL, options.Topic, nonce, msg)
	if err = envelope.Seal(options); err != nil {
		return nil, err
	}
	return envelope, nil
}

//decryptsymmetric使用aes-gcm-256使用主题密钥解密消息。
//nonce大小应为12个字节（请参阅cipher.gcmstandardnoncosize）。
func (msg *ReceivedMessage) decryptSymmetric(key []byte, nonce []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if len(nonce) != aesgcm.NonceSize() {
		log.Error("decrypting the message", "AES nonce size", len(nonce))
		return errors.New("wrong AES nonce size")
	}
	decrypted, err := aesgcm.Open(nil, nonce, msg.Raw, nil)
	if err != nil {
		return err
	}
	msg.Raw = decrypted
	return nil
}

//解密非对称使用私钥解密加密的负载。
func (msg *ReceivedMessage) decryptAsymmetric(key *ecdsa.PrivateKey) error {
	decrypted, err := ecies.ImportECDSA(key).Decrypt(msg.Raw, nil, nil)
	if err == nil {
		msg.Raw = decrypted
	}
	return err
}

//验证检查有效性并在成功时提取字段
func (msg *ReceivedMessage) Validate() bool {
	end := len(msg.Raw)
	if end < 1 {
		return false
	}

	if isMessageSigned(msg.Raw[0]) {
		end -= signatureLength
		if end <= 1 {
			return false
		}
		msg.Signature = msg.Raw[end:]
		msg.Src = msg.SigToPubKey()
		if msg.Src == nil {
			return false
		}
	}

	padSize, ok := msg.extractPadding(end)
	if !ok {
		return false
	}

	msg.Payload = msg.Raw[1+padSize : end]
	return true
}

//提取填充从原始消息中提取填充。
//尽管我们不支持发送填充大小的邮件
//超过255字节，此类消息完全有效，并且
//可以成功解密。
func (msg *ReceivedMessage) extractPadding(end int) (int, bool) {
	paddingSize := 0
sz := int(msg.Raw[0] & paddingMask) //指示填充整个大小的字节数（包括这些字节）
//可能是零——意味着没有填充
	if sz != 0 {
		paddingSize = int(bytesToUintLittleEndian(msg.Raw[1 : 1+sz]))
		if paddingSize < sz || paddingSize+1 > end {
			return 0, false
		}
		msg.Padding = msg.Raw[1+sz : 1+paddingSize]
	}
	return paddingSize, true
}

//sigtopubkey检索消息签名者的公钥。
func (msg *ReceivedMessage) SigToPubKey() *ecdsa.PublicKey {
defer func() { recover() }() //如果签名无效

	pub, err := crypto.SigToPub(msg.hash(), msg.Signature)
	if err != nil {
		log.Error("failed to recover public key from signature", "err", err)
		return nil
	}
	return pub
}

//hash计算消息标志、有效负载和填充的sha3校验和。
func (msg *ReceivedMessage) hash() []byte {
	if isMessageSigned(msg.Raw[0]) {
		sz := len(msg.Raw) - signatureLength
		return crypto.Keccak256(msg.Raw[:sz])
	}
	return crypto.Keccak256(msg.Raw)
}
