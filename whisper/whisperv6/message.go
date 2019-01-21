
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

package whisperv6

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	mrand "math/rand"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/ethereum/go-ethereum/log"
)

//messageparams指定邮件的包装方式
//装进信封里。
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
//并成功解密。
type ReceivedMessage struct {
	Raw []byte

	Payload   []byte
	Padding   []byte
	Signature []byte
	Salt      []byte

PoW   float64          //耳语规范中所述的工作证明
Sent  uint32           //消息发布到网络的时间
TTL   uint32           //消息允许的最长生存时间
Src   *ecdsa.PublicKey //邮件收件人（用于解码邮件的标识）
Dst   *ecdsa.PublicKey //邮件收件人（用于解码邮件的标识）
	Topic TopicType

SymKeyHash   common.Hash //密钥的keccak256哈希
EnvelopeHash common.Hash //作为唯一ID的消息信封哈希
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
	const payloadSizeFieldMaxSize = 4
	msg := sentMessage{}
	msg.Raw = make([]byte, 1,
		flagsLength+payloadSizeFieldMaxSize+len(params.Payload)+len(params.Padding)+signatureLength+padSizeLimit)
msg.Raw[0] = 0 //将所有标志设置为零
	msg.addPayloadSizeField(params.Payload)
	msg.Raw = append(msg.Raw, params.Payload...)
	err := msg.appendPadding(params)
	return &msg, err
}

//AddPayLoadSizeField附加包含有效负载大小的辅助字段
func (msg *sentMessage) addPayloadSizeField(payload []byte) {
	fieldSize := getSizeOfPayloadSizeField(payload)
	field := make([]byte, 4)
	binary.LittleEndian.PutUint32(field, uint32(len(payload)))
	field = field[:fieldSize]
	msg.Raw = append(msg.Raw, field...)
	msg.Raw[0] |= byte(fieldSize)
}

//GetSizeOfPayLoadSizeField返回对有效负载大小进行编码所需的字节数。
func getSizeOfPayloadSizeField(payload []byte) int {
	s := 1
	for i := len(payload); i >= 256; i /= 256 {
		s++
	}
	return s
}

//AppendPadding追加参数中指定的填充。
//如果参数中没有提供填充，则生成随机填充。
func (msg *sentMessage) appendPadding(params *MessageParams) error {
	if len(params.Padding) != 0 {
//填充数据由DAPP提供，按原样使用
		msg.Raw = append(msg.Raw, params.Padding...)
		return nil
	}

	rawSize := flagsLength + getSizeOfPayloadSizeField(params.Payload) + len(params.Payload)
	if params.Src != nil {
		rawSize += signatureLength
	}
	odd := rawSize % padSizeLimit
	paddingSize := padSizeLimit - odd
	pad := make([]byte, paddingSize)
	_, err := crand.Read(pad)
	if err != nil {
		return err
	}
	if !validateDataIntegrity(pad, paddingSize) {
		return errors.New("failed to generate random padding of size " + strconv.Itoa(paddingSize))
	}
	msg.Raw = append(msg.Raw, pad...)
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

msg.Raw[0] |= signatureFlag //在签名前设置此标志很重要
	hash := crypto.Keccak256(msg.Raw)
	signature, err := crypto.Sign(hash, key)
	if err != nil {
msg.Raw[0] &= (0xFF ^ signatureFlag) //清旗
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
func (msg *sentMessage) encryptSymmetric(key []byte) (err error) {
	if !validateDataIntegrity(key, aesKeyLength) {
		return errors.New("invalid key provided for symmetric encryption, size: " + strconv.Itoa(len(key)))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
salt, err := generateSecureRandomData(aesNonceLength) //对于给定的键，不要使用超过2^32个随机nonce
	if err != nil {
		return err
	}
	encrypted := aesgcm.Seal(nil, salt, msg.Raw, nil)
	msg.Raw = append(encrypted, salt...)
	return nil
}

//generatesecurerandomdata在需要额外安全性的地方生成随机数据。
//此功能的目的是防止软件或硬件中的某些错误
//提供不太随机的数据。这对aes nonce特别有用，
//真正的随机性并不重要，但是
//每封邮件都是独一无二的。
func generateSecureRandomData(length int) ([]byte, error) {
	x := make([]byte, length)
	y := make([]byte, length)
	res := make([]byte, length)

	_, err := crand.Read(x)
	if err != nil {
		return nil, err
	} else if !validateDataIntegrity(x, length) {
		return nil, errors.New("crypto/rand failed to generate secure random data")
	}
	_, err = mrand.Read(y)
	if err != nil {
		return nil, err
	} else if !validateDataIntegrity(y, length) {
		return nil, errors.New("math/rand failed to generate secure random data")
	}
	for i := 0; i < length; i++ {
		res[i] = x[i] ^ y[i]
	}
	if !validateDataIntegrity(res, length) {
		return nil, errors.New("failed to generate secure random data")
	}
	return res, nil
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
	if options.Dst != nil {
		err = msg.encryptAsymmetric(options.Dst)
	} else if options.KeySym != nil {
		err = msg.encryptSymmetric(options.KeySym)
	} else {
		err = errors.New("unable to encrypt the message: neither symmetric nor assymmetric key provided")
	}
	if err != nil {
		return nil, err
	}

	envelope = NewEnvelope(options.TTL, options.Topic, msg)
	if err = envelope.Seal(options); err != nil {
		return nil, err
	}
	return envelope, nil
}

//decryptsymmetric使用aes-gcm-256使用主题密钥解密消息。
//nonce大小应为12个字节（请参阅cipher.gcmstandardnoncosize）。
func (msg *ReceivedMessage) decryptSymmetric(key []byte) error {
//对称消息应在有效负载末尾包含12字节的nonce
	if len(msg.Raw) < aesNonceLength {
		return errors.New("missing salt or invalid payload in symmetric message")
	}
	salt := msg.Raw[len(msg.Raw)-aesNonceLength:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	decrypted, err := aesgcm.Open(nil, salt, msg.Raw[:len(msg.Raw)-aesNonceLength], nil)
	if err != nil {
		return err
	}
	msg.Raw = decrypted
	msg.Salt = salt
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

//validateAndParse检查消息的有效性，并在成功时提取字段。
func (msg *ReceivedMessage) ValidateAndParse() bool {
	end := len(msg.Raw)
	if end < 1 {
		return false
	}

	if isMessageSigned(msg.Raw[0]) {
		end -= signatureLength
		if end <= 1 {
			return false
		}
		msg.Signature = msg.Raw[end : end+signatureLength]
		msg.Src = msg.SigToPubKey()
		if msg.Src == nil {
			return false
		}
	}

	beg := 1
	payloadSize := 0
sizeOfPayloadSizeField := int(msg.Raw[0] & SizeMask) //指示有效负载大小的字节数
	if sizeOfPayloadSizeField != 0 {
		payloadSize = int(bytesToUintLittleEndian(msg.Raw[beg : beg+sizeOfPayloadSizeField]))
		if payloadSize+1 > end {
			return false
		}
		beg += sizeOfPayloadSizeField
		msg.Payload = msg.Raw[beg : beg+payloadSize]
	}

	beg += payloadSize
	msg.Padding = msg.Raw[beg:end]
	return true
}

//sigtopubkey返回与消息的
//签名。
func (msg *ReceivedMessage) SigToPubKey() *ecdsa.PublicKey {
defer func() { recover() }() //如果签名无效

	pub, err := crypto.SigToPub(msg.hash(), msg.Signature)
	if err != nil {
		log.Error("failed to recover public key from signature", "err", err)
		return nil
	}
	return pub
}

//hash计算消息标志、有效负载大小字段、有效负载和填充的sha3校验和。
func (msg *ReceivedMessage) hash() []byte {
	if isMessageSigned(msg.Raw[0]) {
		sz := len(msg.Raw) - signatureLength
		return crypto.Keccak256(msg.Raw[:sz])
	}
	return crypto.Keccak256(msg.Raw)
}
