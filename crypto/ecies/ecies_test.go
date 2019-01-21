
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

import (
	"bytes"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

var dumpEnc bool

func init() {
	flDump := flag.Bool("dump", false, "write encrypted test message to file")
	flag.Parse()
	dumpEnc = *flDump
}

//确保KDF生成大小适当的密钥。
func TestKDF(t *testing.T) {
	msg := []byte("Hello, world")
	h := sha256.New()

	k, err := concatKDF(h, msg, nil, 64)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
	if len(k) != 64 {
		fmt.Printf("KDF: generated key is the wrong size (%d instead of 64\n", len(k))
		t.FailNow()
	}
}

var ErrBadSharedKeys = fmt.Errorf("ecies: shared keys don't match")

//cmpfarams比较一组ecies参数。我们假设，根据
//文档，认为AES是唯一支持的对称加密算法。
func cmpParams(p1, p2 *ECIESParams) bool {
	return p1.hashAlgo == p2.hashAlgo &&
		p1.KeyLen == p2.KeyLen &&
		p1.BlockSize == p2.BlockSize
}

//如果两个公钥表示相同的pojnt，则cmppublic返回true。
func cmpPublic(pub1, pub2 PublicKey) bool {
	if pub1.X == nil || pub1.Y == nil {
		fmt.Println(ErrInvalidPublicKey.Error())
		return false
	}
	if pub2.X == nil || pub2.Y == nil {
		fmt.Println(ErrInvalidPublicKey.Error())
		return false
	}
	pub1Out := elliptic.Marshal(pub1.Curve, pub1.X, pub1.Y)
	pub2Out := elliptic.Marshal(pub2.Curve, pub2.X, pub2.Y)

	return bytes.Equal(pub1Out, pub2Out)
}

//如果两个私钥相同，则cmprivate返回true。
func cmpPrivate(prv1, prv2 *PrivateKey) bool {
	if prv1 == nil || prv1.D == nil {
		return false
	} else if prv2 == nil || prv2.D == nil {
		return false
	} else if prv1.D.Cmp(prv2.D) != 0 {
		return false
	} else {
		return cmpPublic(prv1.PublicKey, prv2.PublicKey)
	}
}

//验证ECDH组件。
func TestSharedKey(t *testing.T) {
	prv1, err := GenerateKey(rand.Reader, DefaultCurve, nil)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
	skLen := MaxSharedKeyLength(&prv1.PublicKey) / 2

	prv2, err := GenerateKey(rand.Reader, DefaultCurve, nil)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	sk1, err := prv1.GenerateShared(&prv2.PublicKey, skLen, skLen)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	sk2, err := prv2.GenerateShared(&prv1.PublicKey, skLen, skLen)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	if !bytes.Equal(sk1, sk2) {
		fmt.Println(ErrBadSharedKeys.Error())
		t.FailNow()
	}
}

func TestSharedKeyPadding(t *testing.T) {
//清醒检查
	prv0 := hexKey("1adf5c18167d96a1f9a0b1ef63be8aa27eaf6032c233b2b38f7850cf5b859fd9")
	prv1 := hexKey("0097a076fc7fcd9208240668e31c9abee952cbb6e375d1b8febc7499d6e16f1a")
	x0, _ := new(big.Int).SetString("1a8ed022ff7aec59dc1b440446bdda5ff6bcb3509a8b109077282b361efffbd8", 16)
	x1, _ := new(big.Int).SetString("6ab3ac374251f638d0abb3ef596d1dc67955b507c104e5f2009724812dc027b8", 16)
	y0, _ := new(big.Int).SetString("e040bd480b1deccc3bc40bd5b1fdcb7bfd352500b477cb9471366dbd4493f923", 16)
	y1, _ := new(big.Int).SetString("8ad915f2b503a8be6facab6588731fefeb584fd2dfa9a77a5e0bba1ec439e4fa", 16)

	if prv0.PublicKey.X.Cmp(x0) != 0 {
		t.Errorf("mismatched prv0.X:\nhave: %x\nwant: %x\n", prv0.PublicKey.X.Bytes(), x0.Bytes())
	}
	if prv0.PublicKey.Y.Cmp(y0) != 0 {
		t.Errorf("mismatched prv0.Y:\nhave: %x\nwant: %x\n", prv0.PublicKey.Y.Bytes(), y0.Bytes())
	}
	if prv1.PublicKey.X.Cmp(x1) != 0 {
		t.Errorf("mismatched prv1.X:\nhave: %x\nwant: %x\n", prv1.PublicKey.X.Bytes(), x1.Bytes())
	}
	if prv1.PublicKey.Y.Cmp(y1) != 0 {
		t.Errorf("mismatched prv1.Y:\nhave: %x\nwant: %x\n", prv1.PublicKey.Y.Bytes(), y1.Bytes())
	}

//测试共享秘密生成
	sk1, err := prv0.GenerateShared(&prv1.PublicKey, 16, 16)
	if err != nil {
		fmt.Println(err.Error())
	}

	sk2, err := prv1.GenerateShared(&prv0.PublicKey, 16, 16)
	if err != nil {
		t.Fatal(err.Error())
	}

	if !bytes.Equal(sk1, sk2) {
		t.Fatal(ErrBadSharedKeys.Error())
	}
}

//验证当密钥数据太多时密钥生成代码是否失败
//请求。
func TestTooBigSharedKey(t *testing.T) {
	prv1, err := GenerateKey(rand.Reader, DefaultCurve, nil)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	prv2, err := GenerateKey(rand.Reader, DefaultCurve, nil)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	_, err = prv1.GenerateShared(&prv2.PublicKey, 32, 32)
	if err != ErrSharedKeyTooBig {
		fmt.Println("ecdh: shared key should be too large for curve")
		t.FailNow()
	}

	_, err = prv2.GenerateShared(&prv1.PublicKey, 32, 32)
	if err != ErrSharedKeyTooBig {
		fmt.Println("ecdh: shared key should be too large for curve")
		t.FailNow()
	}
}

//对p256密钥的生成进行基准测试。
func BenchmarkGenerateKeyP256(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := GenerateKey(rand.Reader, elliptic.P256(), nil); err != nil {
			fmt.Println(err.Error())
			b.FailNow()
		}
	}
}

//基准测试p256共享密钥的生成。
func BenchmarkGenSharedKeyP256(b *testing.B) {
	prv, err := GenerateKey(rand.Reader, elliptic.P256(), nil)
	if err != nil {
		fmt.Println(err.Error())
		b.FailNow()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := prv.GenerateShared(&prv.PublicKey, 16, 16)
		if err != nil {
			fmt.Println(err.Error())
			b.FailNow()
		}
	}
}

//对S256共享密钥的生成进行基准测试。
func BenchmarkGenSharedKeyS256(b *testing.B) {
	prv, err := GenerateKey(rand.Reader, crypto.S256(), nil)
	if err != nil {
		fmt.Println(err.Error())
		b.FailNow()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := prv.GenerateShared(&prv.PublicKey, 16, 16)
		if err != nil {
			fmt.Println(err.Error())
			b.FailNow()
		}
	}
}

//验证加密邮件是否可以成功解密。
func TestEncryptDecrypt(t *testing.T) {
	prv1, err := GenerateKey(rand.Reader, DefaultCurve, nil)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	prv2, err := GenerateKey(rand.Reader, DefaultCurve, nil)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	message := []byte("Hello, world.")
	ct, err := Encrypt(rand.Reader, &prv2.PublicKey, message, nil, nil)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	pt, err := prv2.Decrypt(ct, nil, nil)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	if !bytes.Equal(pt, message) {
		fmt.Println("ecies: plaintext doesn't match message")
		t.FailNow()
	}

	_, err = prv1.Decrypt(ct, nil, nil)
	if err == nil {
		fmt.Println("ecies: encryption should not have succeeded")
		t.FailNow()
	}
}

func TestDecryptShared2(t *testing.T) {
	prv, err := GenerateKey(rand.Reader, DefaultCurve, nil)
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("Hello, world.")
	shared2 := []byte("shared data 2")
	ct, err := Encrypt(rand.Reader, &prv.PublicKey, message, nil, shared2)
	if err != nil {
		t.Fatal(err)
	}

//检查使用正确的共享数据进行解密是否有效。
	pt, err := prv.Decrypt(ct, nil, shared2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pt, message) {
		t.Fatal("ecies: plaintext doesn't match message")
	}

//在没有共享数据或共享数据不正确的情况下解密失败。
	if _, err = prv.Decrypt(ct, nil, nil); err == nil {
		t.Fatal("ecies: decrypting without shared data didn't fail")
	}
	if _, err = prv.Decrypt(ct, nil, []byte("garbage")); err == nil {
		t.Fatal("ecies: decrypting with incorrect shared data didn't fail")
	}
}

type testCase struct {
	Curve    elliptic.Curve
	Name     string
	Expected *ECIESParams
}

var testCases = []testCase{
	{
		Curve:    elliptic.P256(),
		Name:     "P256",
		Expected: ECIES_AES128_SHA256,
	},
	{
		Curve:    elliptic.P384(),
		Name:     "P384",
		Expected: ECIES_AES256_SHA384,
	},
	{
		Curve:    elliptic.P521(),
		Name:     "P521",
		Expected: ECIES_AES256_SHA512,
	},
}

//每条曲线的测试参数选择，P224自动失效
//参数选择（有关p224的讨论，请参阅自述文件）。确保
//为给定曲线自动选择一组参数会起作用。
func TestParamSelection(t *testing.T) {
	for _, c := range testCases {
		testParamSelection(t, c)
	}
}

func testParamSelection(t *testing.T, c testCase) {
	params := ParamsFromCurve(c.Curve)
	if params == nil && c.Expected != nil {
		fmt.Printf("%s (%s)\n", ErrInvalidParams.Error(), c.Name)
		t.FailNow()
	} else if params != nil && !cmpParams(params, c.Expected) {
		fmt.Printf("ecies: parameters should be invalid (%s)\n",
			c.Name)
		t.FailNow()
	}

	prv1, err := GenerateKey(rand.Reader, DefaultCurve, nil)
	if err != nil {
		fmt.Printf("%s (%s)\n", err.Error(), c.Name)
		t.FailNow()
	}

	prv2, err := GenerateKey(rand.Reader, DefaultCurve, nil)
	if err != nil {
		fmt.Printf("%s (%s)\n", err.Error(), c.Name)
		t.FailNow()
	}

	message := []byte("Hello, world.")
	ct, err := Encrypt(rand.Reader, &prv2.PublicKey, message, nil, nil)
	if err != nil {
		fmt.Printf("%s (%s)\n", err.Error(), c.Name)
		t.FailNow()
	}

	pt, err := prv2.Decrypt(ct, nil, nil)
	if err != nil {
		fmt.Printf("%s (%s)\n", err.Error(), c.Name)
		t.FailNow()
	}

	if !bytes.Equal(pt, message) {
		fmt.Printf("ecies: plaintext doesn't match message (%s)\n",
			c.Name)
		t.FailNow()
	}

	_, err = prv1.Decrypt(ct, nil, nil)
	if err == nil {
		fmt.Printf("ecies: encryption should not have succeeded (%s)\n",
			c.Name)
		t.FailNow()
	}

}

//确保解密操作中的基本公钥验证
//作品。
func TestBasicKeyValidation(t *testing.T) {
	badBytes := []byte{0, 1, 5, 6, 7, 8, 9}

	prv, err := GenerateKey(rand.Reader, DefaultCurve, nil)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	message := []byte("Hello, world.")
	ct, err := Encrypt(rand.Reader, &prv.PublicKey, message, nil, nil)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	for _, b := range badBytes {
		ct[0] = b
		_, err := prv.Decrypt(ct, nil, nil)
		if err != ErrInvalidPublicKey {
			fmt.Println("ecies: validated an invalid key")
			t.FailNow()
		}
	}
}

func TestBox(t *testing.T) {
	prv1 := hexKey("4b50fa71f5c3eeb8fdc452224b2395af2fcc3d125e06c32c82e048c0559db03f")
	prv2 := hexKey("d0b043b4c5d657670778242d82d68a29d25d7d711127d17b8e299f156dad361a")
	pub2 := &prv2.PublicKey

	message := []byte("Hello, world.")
	ct, err := Encrypt(rand.Reader, pub2, message, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	pt, err := prv2.Decrypt(ct, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pt, message) {
		t.Fatal("ecies: plaintext doesn't match message")
	}
	if _, err = prv1.Decrypt(ct, nil, nil); err == nil {
		t.Fatal("ecies: encryption should not have succeeded")
	}
}

//Verify GenerateShared against static values - useful when
//调试基础libs中的更改
func TestSharedKeyStatic(t *testing.T) {
	prv1 := hexKey("7ebbc6a8358bc76dd73ebc557056702c8cfc34e5cfcd90eb83af0347575fd2ad")
	prv2 := hexKey("6a3d6396903245bba5837752b9e0348874e72db0c4e11e9c485a81b4ea4353b9")

	skLen := MaxSharedKeyLength(&prv1.PublicKey) / 2

	sk1, err := prv1.GenerateShared(&prv2.PublicKey, skLen, skLen)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	sk2, err := prv2.GenerateShared(&prv1.PublicKey, skLen, skLen)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	if !bytes.Equal(sk1, sk2) {
		fmt.Println(ErrBadSharedKeys.Error())
		t.FailNow()
	}

	sk, _ := hex.DecodeString("167ccc13ac5e8a26b131c3446030c60fbfac6aa8e31149d0869f93626a4cdf62")
	if !bytes.Equal(sk1, sk) {
		t.Fatalf("shared secret mismatch: want: %x have: %x", sk, sk1)
	}
}

func hexKey(prv string) *PrivateKey {
	key, err := crypto.HexToECDSA(prv)
	if err != nil {
		panic(err)
	}
	return ImportECDSA(key)
}
