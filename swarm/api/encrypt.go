
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

package api

import (
	"encoding/binary"
	"errors"

	"github.com/ethereum/go-ethereum/swarm/storage/encryption"
	"golang.org/x/crypto/sha3"
)

type RefEncryption struct {
	refSize int
	span    []byte
}

func NewRefEncryption(refSize int) *RefEncryption {
	span := make([]byte, 8)
	binary.LittleEndian.PutUint64(span, uint64(refSize))
	return &RefEncryption{
		refSize: refSize,
		span:    span,
	}
}

func (re *RefEncryption) Encrypt(ref []byte, key []byte) ([]byte, error) {
	spanEncryption := encryption.New(key, 0, uint32(re.refSize/32), sha3.NewLegacyKeccak256)
	encryptedSpan, err := spanEncryption.Encrypt(re.span)
	if err != nil {
		return nil, err
	}
	dataEncryption := encryption.New(key, re.refSize, 0, sha3.NewLegacyKeccak256)
	encryptedData, err := dataEncryption.Encrypt(ref)
	if err != nil {
		return nil, err
	}
	encryptedRef := make([]byte, len(ref)+8)
	copy(encryptedRef[:8], encryptedSpan)
	copy(encryptedRef[8:], encryptedData)

	return encryptedRef, nil
}

func (re *RefEncryption) Decrypt(ref []byte, key []byte) ([]byte, error) {
	spanEncryption := encryption.New(key, 0, uint32(re.refSize/32), sha3.NewLegacyKeccak256)
	decryptedSpan, err := spanEncryption.Decrypt(ref[:8])
	if err != nil {
		return nil, err
	}

	size := binary.LittleEndian.Uint64(decryptedSpan)
	if size != uint64(len(ref)-8) {
		return nil, errors.New("invalid span in encrypted reference")
	}

	dataEncryption := encryption.New(key, re.refSize, 0, sha3.NewLegacyKeccak256)
	decryptedRef, err := dataEncryption.Decrypt(ref[8:])
	if err != nil {
		return nil, err
	}

	return decryptedRef, nil
}
