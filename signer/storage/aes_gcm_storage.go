
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
//此文件是Go以太坊的一部分。
//
//Go以太坊是免费软件：您可以重新发布和/或修改它
//根据GNU通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊的分布希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU通用公共许可证了解更多详细信息。
//
//你应该已经收到一份GNU通用公共许可证的副本
//一起去以太坊吧。如果没有，请参见<http://www.gnu.org/licenses/>。
//

package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"

	"github.com/ethereum/go-ethereum/log"
)

type storedCredential struct {
//四维
	Iv []byte `json:"iv"`
//密文
	CipherText []byte `json:"c"`
}

//AESEncryptedStorage是一种由JSON文件支持的存储类型。json文件包含
//密钥值映射，其中密钥没有加密，只有值是加密的。
type AESEncryptedStorage struct {
//要读取/写入凭据的文件
	filename string
//密钥存储在base64中
	key []byte
}

//newaesEncryptedStorage创建由给定文件/密钥支持的新加密存储
func NewAESEncryptedStorage(filename string, key []byte) *AESEncryptedStorage {
	return &AESEncryptedStorage{
		filename: filename,
		key:      key,
	}
}

//按键存储值。0长度键导致无操作
func (s *AESEncryptedStorage) Put(key, value string) {
	if len(key) == 0 {
		return
	}
	data, err := s.readEncryptedStorage()
	if err != nil {
		log.Warn("Failed to read encrypted storage", "err", err, "file", s.filename)
		return
	}
	ciphertext, iv, err := encrypt(s.key, []byte(value), []byte(key))
	if err != nil {
		log.Warn("Failed to encrypt entry", "err", err)
		return
	}
	encrypted := storedCredential{Iv: iv, CipherText: ciphertext}
	data[key] = encrypted
	if err = s.writeEncryptedStorage(data); err != nil {
		log.Warn("Failed to write entry", "err", err)
	}
}

//get返回以前存储的值，如果该值不存在或键的长度为0，则返回空字符串
func (s *AESEncryptedStorage) Get(key string) string {
	if len(key) == 0 {
		return ""
	}
	data, err := s.readEncryptedStorage()
	if err != nil {
		log.Warn("Failed to read encrypted storage", "err", err, "file", s.filename)
		return ""
	}
	encrypted, exist := data[key]
	if !exist {
		log.Warn("Key does not exist", "key", key)
		return ""
	}
	entry, err := decrypt(s.key, encrypted.Iv, encrypted.CipherText, []byte(key))
	if err != nil {
		log.Warn("Failed to decrypt key", "key", key)
		return ""
	}
	return string(entry)
}

//ReadEncryptedStorage使用加密的凭据读取文件
func (s *AESEncryptedStorage) readEncryptedStorage() (map[string]storedCredential, error) {
	creds := make(map[string]storedCredential)
	raw, err := ioutil.ReadFile(s.filename)

	if err != nil {
		if os.IsNotExist(err) {
//还不存在
			return creds, nil
		}
		log.Warn("Failed to read encrypted storage", "err", err, "file", s.filename)
	}
	if err = json.Unmarshal(raw, &creds); err != nil {
		log.Warn("Failed to unmarshal encrypted storage", "err", err, "file", s.filename)
		return nil, err
	}
	return creds, nil
}

//WriteEncryptedStorage使用加密的凭据写入文件
func (s *AESEncryptedStorage) writeEncryptedStorage(creds map[string]storedCredential) error {
	raw, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	if err = ioutil.WriteFile(s.filename, raw, 0600); err != nil {
		return err
	}
	return nil
}

//加密用给定的密钥加密明文，并附加数据
//“附加数据”用于将（明文）kv存储密钥放入v，
//以防止改变一个k，或相互交换kv存储中的两个条目。
func encrypt(key []byte, plaintext []byte, additionalData []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	if err != nil {
		return nil, nil, err
	}
	ciphertext := aesgcm.Seal(nil, nonce, plaintext, additionalData)
	return ciphertext, nonce, nil
}

func decrypt(key []byte, nonce []byte, ciphertext []byte, additionalData []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, additionalData)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
