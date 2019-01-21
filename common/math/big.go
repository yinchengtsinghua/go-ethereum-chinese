
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

//包数学提供整数数学实用程序。
package math

import (
	"fmt"
	"math/big"
)

//各种大整数限制值。
var (
	tt255     = BigPow(2, 255)
	tt256     = BigPow(2, 256)
	tt256m1   = new(big.Int).Sub(tt256, big.NewInt(1))
	tt63      = BigPow(2, 63)
	MaxBig256 = new(big.Int).Set(tt256m1)
	MaxBig63  = new(big.Int).Sub(tt63, big.NewInt(1))
)

const (
//一个大字的位数。
	wordBits = 32 << (uint64(^big.Word(0)) >> 63)
//一个大的.word中的字节数
	wordBytes = wordBits / 8
)

//hexordecimal256将big.int封送为十六进制或十进制。
type HexOrDecimal256 big.Int

//UnmarshalText实现encoding.textUnmarshaller。
func (i *HexOrDecimal256) UnmarshalText(input []byte) error {
	bigint, ok := ParseBig256(string(input))
	if !ok {
		return fmt.Errorf("invalid hex or decimal integer %q", input)
	}
	*i = HexOrDecimal256(*bigint)
	return nil
}

//MarshalText实现Encoding.TextMarshaler。
func (i *HexOrDecimal256) MarshalText() ([]byte, error) {
	if i == nil {
		return []byte("0x0"), nil
	}
	return []byte(fmt.Sprintf("%#x", (*big.Int)(i))), nil
}

//parseBig256以十进制或十六进制语法解析为256位整数。
//可接受前导零。空字符串解析为零。
func ParseBig256(s string) (*big.Int, bool) {
	if s == "" {
		return new(big.Int), true
	}
	var bigint *big.Int
	var ok bool
	if len(s) >= 2 && (s[:2] == "0x" || s[:2] == "0X") {
		bigint, ok = new(big.Int).SetString(s[2:], 16)
	} else {
		bigint, ok = new(big.Int).SetString(s, 10)
	}
	if ok && bigint.BitLen() > 256 {
		bigint, ok = nil, false
	}
	return bigint, ok
}

//mustParseBig256解析为256位大整数，如果字符串无效，则会恐慌。
func MustParseBig256(s string) *big.Int {
	v, ok := ParseBig256(s)
	if !ok {
		panic("invalid 256 bit integer: " + s)
	}
	return v
}

//bigpow返回一个**b作为大整数。
func BigPow(a, b int64) *big.Int {
	r := big.NewInt(a)
	return r.Exp(r, big.NewInt(b), nil)
}

//bigmax返回x或y中的较大值。
func BigMax(x, y *big.Int) *big.Int {
	if x.Cmp(y) < 0 {
		return y
	}
	return x
}

//bigmin返回x或y中的较小值。
func BigMin(x, y *big.Int) *big.Int {
	if x.Cmp(y) > 0 {
		return y
	}
	return x
}

//firstbitset返回v中第一个1位的索引，从lsb开始计数。
func FirstBitSet(v *big.Int) int {
	for i := 0; i < v.BitLen(); i++ {
		if v.Bit(i) > 0 {
			return i
		}
	}
	return v.BitLen()
}

//paddedbigbytes将一个大整数编码为一个big endian字节片。长度
//切片中至少有n个字节。
func PaddedBigBytes(bigint *big.Int, n int) []byte {
	if bigint.BitLen()/8 >= n {
		return bigint.Bytes()
	}
	ret := make([]byte, n)
	ReadBits(bigint, ret)
	return ret
}

//bigendianbyteat返回位置n处的字节，
//在big-endian编码中
//因此n==0返回最低有效字节
func bigEndianByteAt(bigint *big.Int, n int) byte {
	words := bigint.Bits()
//检查字节将驻留的字桶
	i := n / wordBytes
	if i >= len(words) {
		return byte(0)
	}
	word := words[i]
//字节偏移量
	shift := 8 * uint(n%wordBytes)

	return byte(word >> shift)
}

//byte返回位置n处的字节，
//以小尾数编码提供的padLength。
//n==0返回最高位
//示例：Bigint“5”，PadLength 32，n=31=>5
func Byte(bigint *big.Int, padlength, n int) byte {
	if n >= padlength {
		return byte(0)
	}
	return bigEndianByteAt(bigint, padlength-1-n)
}

//readbits将bigint的绝对值编码为big-endian字节。呼叫者必须确保
//那个流浪汉有足够的空间。如果buf太短，结果将不完整。
func ReadBits(bigint *big.Int, buf []byte) {
	i := len(buf)
	for _, d := range bigint.Bits() {
		for j := 0; j < wordBytes && i > 0; j++ {
			i--
			buf[i] = byte(d)
			d >>= 8
		}
	}
}

//U256编码为256位2的补码。这项行动具有破坏性。
func U256(x *big.Int) *big.Int {
	return x.And(x, tt256m1)
}

//s256将x解释为2的补码。
//X不能超过256位（如果结果未定义），并且不能修改。
//
//s256（0）=0
//s256（1）=1
//S256（2**255）=-2**255
//s256（2**256-1）=-1
func S256(x *big.Int) *big.Int {
	if x.Cmp(tt255) < 0 {
		return x
	}
	return new(big.Int).Sub(x, tt256)
}

//exp通过平方实现求幂。
//exp返回新分配的大整数，不更改
//基或指数。结果被截断为256位。
//
//由@karalabe和@chfast提供
func Exp(base, exponent *big.Int) *big.Int {
	result := big.NewInt(1)

	for _, word := range exponent.Bits() {
		for i := 0; i < wordBits; i++ {
			if word&1 == 1 {
				U256(result.Mul(result, base))
			}
			U256(base.Mul(base, base))
			word >>= 1
		}
	}
	return result
}
