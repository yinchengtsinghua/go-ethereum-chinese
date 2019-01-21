
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有（c）2013 Kyle Isom<kyle@tyrfingr.is>
//版权所有（c）2012 The Go作者。版权所有。
//
//以源和二进制形式重新分配和使用，有或无
//允许修改，前提是以下条件
//遇见：
//
//*源代码的再分配必须保留上述版权。
//注意，此条件列表和以下免责声明。
//*二进制形式的再分配必须复制上述内容
//版权声明、此条件列表和以下免责声明
//在提供的文件和/或其他材料中，
//分布。
//*无论是谷歌公司的名称还是其
//贡献者可用于支持或推广源自
//本软件未经事先明确书面许可。
//
//本软件由版权所有者和贡献者提供。
//“原样”和任何明示或暗示的保证，包括但不包括
//仅限于对适销性和适用性的暗示保证
//不承认特定目的。在任何情况下，版权
//所有人或出资人对任何直接、间接、附带的，
//特殊、惩戒性或后果性损害（包括但不包括
//仅限于采购替代货物或服务；使用损失，
//数据或利润；或业务中断），无论如何引起的
//责任理论，无论是合同责任、严格责任还是侵权责任。
//（包括疏忽或其他）因使用不当而引起的
//即使已告知此类损坏的可能性。

package ecies

//此文件包含用于ECIES加密的参数，指定
//对称加密和HMAC参数。

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

var (
	DefaultCurve                  = ethcrypto.S256()
	ErrUnsupportedECDHAlgorithm   = fmt.Errorf("ecies: unsupported ECDH algorithm")
	ErrUnsupportedECIESParameters = fmt.Errorf("ecies: unsupported ECIES parameters")
)

type ECIESParams struct {
Hash      func() hash.Hash //哈希函数
	hashAlgo  crypto.Hash
Cipher    func([]byte) (cipher.Block, error) //对称加密
BlockSize int                                //对称密码的块大小
KeyLen    int                                //对称密钥长度
}

//Standard ECIES parameters:
//*使用AES128和HMAC-SHA-256-16的ECIES
//*使用AES256和HMAC-SHA-256-32的ECIE
//* ECIES using AES256 and HMAC-SHA-384-48
//*使用AES256和HMAC-SHA-512-64的ECIES

var (
	ECIES_AES128_SHA256 = &ECIESParams{
		Hash:      sha256.New,
		hashAlgo:  crypto.SHA256,
		Cipher:    aes.NewCipher,
		BlockSize: aes.BlockSize,
		KeyLen:    16,
	}

	ECIES_AES256_SHA256 = &ECIESParams{
		Hash:      sha256.New,
		hashAlgo:  crypto.SHA256,
		Cipher:    aes.NewCipher,
		BlockSize: aes.BlockSize,
		KeyLen:    32,
	}

	ECIES_AES256_SHA384 = &ECIESParams{
		Hash:      sha512.New384,
		hashAlgo:  crypto.SHA384,
		Cipher:    aes.NewCipher,
		BlockSize: aes.BlockSize,
		KeyLen:    32,
	}

	ECIES_AES256_SHA512 = &ECIESParams{
		Hash:      sha512.New,
		hashAlgo:  crypto.SHA512,
		Cipher:    aes.NewCipher,
		BlockSize: aes.BlockSize,
		KeyLen:    32,
	}
)

var paramsFromCurve = map[elliptic.Curve]*ECIESParams{
	ethcrypto.S256(): ECIES_AES128_SHA256,
	elliptic.P256():  ECIES_AES128_SHA256,
	elliptic.P384():  ECIES_AES256_SHA384,
	elliptic.P521():  ECIES_AES256_SHA512,
}

func AddParamsForCurve(curve elliptic.Curve, params *ECIESParams) {
	paramsFromCurve[curve] = params
}

//paramsfromcurve为选定的椭圆曲线选择最佳参数。
//仅支持曲线P256、P38 4和P512。
func ParamsFromCurve(curve elliptic.Curve) (params *ECIESParams) {
	return paramsFromCurve[curve]
}
