
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2010 Go作者。版权所有。
//版权所有2011 Thepiachu。版权所有。
//版权所有2015 Jeffrey Wilcke、Felix Lange、Gustav Simonsson。版权所有。
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
//*不得使用招标人的名称来支持或推广产品。
//未经事先书面许可，从本软件派生。
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

package secp256k1

import (
	"crypto/elliptic"
	"math/big"
	"unsafe"
)

/*
#include "libsecp256k1/include/secp256k1.h"
extern int secp256k1_ext_scalar_mul（const secp256k1_context*ctx，const unsigned char*point，const unsigned char*scalar）；
**/

import "C"

const (
//一个大字的位数。
	wordBits = 32 << (uint64(^big.Word(0)) >> 63)
//一个大的.word中的字节数
	wordBytes = wordBits / 8
)

//readbits将bigint的绝对值编码为big-endian字节。呼叫者
//必须确保buf有足够的空间。如果buf太短，结果将
//不完整。
func readBits(bigint *big.Int, buf []byte) {
	i := len(buf)
	for _, d := range bigint.Bits() {
		for j := 0; j < wordBytes && i > 0; j++ {
			i--
			buf[i] = byte(d)
			d >>= 8
		}
	}
}

//此代码来自https://github.com/thepiachu/gobit和implements
//several Koblitz elliptic curves over prime fields.
//
//在雅可比坐标系内的曲线法。对于给定的
//（x，y）曲线上的位置，雅可比坐标为（x1，y1，
//z1) where x = x1/z1² and y = y1/z1³. The greatest speedups come
//当整个计算可以在转换中执行时
//（如scalarmult和scalarbasemult）。但即使是加和双，
//应用和反转转换比在
//仿射坐标。

//Bitcurve表示a=0的Koblitz曲线。
//See http://www.hyperelliptic.org/EFD/g1p/auto-shortw.html
type BitCurve struct {
P       *big.Int //基础字段的顺序
N       *big.Int //基点顺序
B       *big.Int //比特库夫方程的常数
Gx, Gy  *big.Int //基点的（x，y）
BitSize int      //基础字段的大小
}

func (BitCurve *BitCurve) Params() *elliptic.CurveParams {
	return &elliptic.CurveParams{
		P:       BitCurve.P,
		N:       BitCurve.N,
		B:       BitCurve.B,
		Gx:      BitCurve.Gx,
		Gy:      BitCurve.Gy,
		BitSize: BitCurve.BitSize,
	}
}

//如果给定的（x，y）位于比特币上，isoncurve返回true。
func (BitCurve *BitCurve) IsOnCurve(x, y *big.Int) bool {
//y= x+b
y2 := new(big.Int).Mul(y, y) //钇
y2.Mod(y2, BitCurve.P)       //Y-%P

x3 := new(big.Int).Mul(x, x) //X
x3.Mul(x3, x)                //X

x3.Add(x3, BitCurve.B) //X＋B
x3.Mod(x3, BitCurve.P) //（x+b）%p

	return x3.Cmp(y2) == 0
}

//TODO:再次检查功能是否正常
//雅可比变换的反义形式。查看评论
//文件顶部。
func (BitCurve *BitCurve) affineFromJacobian(x, y, z *big.Int) (xOut, yOut *big.Int) {
	zinv := new(big.Int).ModInverse(z, BitCurve.P)
	zinvsq := new(big.Int).Mul(zinv, zinv)

	xOut = new(big.Int).Mul(x, zinvsq)
	xOut.Mod(xOut, BitCurve.P)
	zinvsq.Mul(zinvsq, zinv)
	yOut = new(big.Int).Mul(y, zinvsq)
	yOut.Mod(yOut, BitCurve.P)
	return
}

//add返回（x1，y1）和（x2，y2）的和
func (BitCurve *BitCurve) Add(x1, y1, x2, y2 *big.Int) (*big.Int, *big.Int) {
	z := new(big.Int).SetInt64(1)
	return BitCurve.affineFromJacobian(BitCurve.addJacobian(x1, y1, z, x2, y2, z))
}

//addjacobian在jacobian坐标中取两点（x1，y1，z1）和
//（x2，y2，z2）并返回它们的和，也是雅可比形式。
func (BitCurve *BitCurve) addJacobian(x1, y1, z1, x2, y2, z2 *big.Int) (*big.Int, *big.Int, *big.Int) {
//请参见http://hyper椭圆形.org/efd/g1p/auto-shortw-jacobian-0.html addition-add-2007-bl
	z1z1 := new(big.Int).Mul(z1, z1)
	z1z1.Mod(z1z1, BitCurve.P)
	z2z2 := new(big.Int).Mul(z2, z2)
	z2z2.Mod(z2z2, BitCurve.P)

	u1 := new(big.Int).Mul(x1, z2z2)
	u1.Mod(u1, BitCurve.P)
	u2 := new(big.Int).Mul(x2, z1z1)
	u2.Mod(u2, BitCurve.P)
	h := new(big.Int).Sub(u2, u1)
	if h.Sign() == -1 {
		h.Add(h, BitCurve.P)
	}
	i := new(big.Int).Lsh(h, 1)
	i.Mul(i, i)
	j := new(big.Int).Mul(h, i)

	s1 := new(big.Int).Mul(y1, z2)
	s1.Mul(s1, z2z2)
	s1.Mod(s1, BitCurve.P)
	s2 := new(big.Int).Mul(y2, z1)
	s2.Mul(s2, z1z1)
	s2.Mod(s2, BitCurve.P)
	r := new(big.Int).Sub(s2, s1)
	if r.Sign() == -1 {
		r.Add(r, BitCurve.P)
	}
	r.Lsh(r, 1)
	v := new(big.Int).Mul(u1, i)

	x3 := new(big.Int).Set(r)
	x3.Mul(x3, x3)
	x3.Sub(x3, j)
	x3.Sub(x3, v)
	x3.Sub(x3, v)
	x3.Mod(x3, BitCurve.P)

	y3 := new(big.Int).Set(r)
	v.Sub(v, x3)
	y3.Mul(y3, v)
	s1.Mul(s1, j)
	s1.Lsh(s1, 1)
	y3.Sub(y3, s1)
	y3.Mod(y3, BitCurve.P)

	z3 := new(big.Int).Add(z1, z2)
	z3.Mul(z3, z3)
	z3.Sub(z3, z1z1)
	if z3.Sign() == -1 {
		z3.Add(z3, BitCurve.P)
	}
	z3.Sub(z3, z2z2)
	if z3.Sign() == -1 {
		z3.Add(z3, BitCurve.P)
	}
	z3.Mul(z3, h)
	z3.Mod(z3, BitCurve.P)

	return x3, y3, z3
}

//双回路2*（x，y）
func (BitCurve *BitCurve) Double(x1, y1 *big.Int) (*big.Int, *big.Int) {
	z1 := new(big.Int).SetInt64(1)
	return BitCurve.affineFromJacobian(BitCurve.doubleJacobian(x1, y1, z1))
}

//Doublejacobian在雅可比坐标（x，y，z）中取一点，并且
//returns its double, also in Jacobian form.
func (BitCurve *BitCurve) doubleJacobian(x, y, z *big.Int) (*big.Int, *big.Int, *big.Int) {
//请参见http://hyper椭圆形.org/efd/g1p/auto-shortw-jacobian-0.html doubling-dbl-2009-l

a := new(big.Int).Mul(x, x) //X1
b := new(big.Int).Mul(y, y) //Y1
c := new(big.Int).Mul(b, b) //乙

d := new(big.Int).Add(x, b) //X1+B
d.Mul(d, d)                 //（x1+b）
d.Sub(d, a)                 //（x1+b）-a
d.Sub(d, c)                 //（X1+B）-A—C
d.Mul(d, big.NewInt(2))     //2*（（x1+b）²-a-c）

e := new(big.Int).Mul(big.NewInt(3), a) //3＊a
f := new(big.Int).Mul(e, e)             //娥

x3 := new(big.Int).Mul(big.NewInt(2), d) //2×D
x3.Sub(f, x3)                            //F 2*D
	x3.Mod(x3, BitCurve.P)

y3 := new(big.Int).Sub(d, x3)                  //DX3
y3.Mul(e, y3)                                  //E*（D x3）
y3.Sub(y3, new(big.Int).Mul(big.NewInt(8), c)) //E*（D x3）- 8×C
	y3.Mod(y3, BitCurve.P)

z3 := new(big.Int).Mul(y, z) //Y1*Z1
z3.Mul(big.NewInt(2), z3)    //3＊Y1*Z1
	z3.Mod(z3, BitCurve.P)

	return x3, y3, z3
}

func (BitCurve *BitCurve) ScalarMult(Bx, By *big.Int, scalar []byte) (*big.Int, *big.Int) {
//确保标量正好是32个字节。我们总是垫，即使
//标量的长度为32字节，以避免出现定时侧通道。
	if len(scalar) > 32 {
		panic("can't handle scalars > 256 bits")
	}
//注意：潜在的时间问题
	padded := make([]byte, 32)
	copy(padded[32-len(scalar):], scalar)
	scalar = padded

//用C做乘法，更新点。
	point := make([]byte, 64)
	readBits(Bx, point[:32])
	readBits(By, point[32:])

	pointPtr := (*C.uchar)(unsafe.Pointer(&point[0]))
	scalarPtr := (*C.uchar)(unsafe.Pointer(&scalar[0]))
	res := C.secp256k1_ext_scalar_mul(context, pointPtr, scalarPtr)

//解包结果并清除临时文件。
	x := new(big.Int).SetBytes(point[:32])
	y := new(big.Int).SetBytes(point[32:])
	for i := range point {
		point[i] = 0
	}
	for i := range padded {
		scalar[i] = 0
	}
	if res != 1 {
		return nil, nil
	}
	return x, y
}

//scalarbasemult返回k*g，其中g是组的基点，k是
//大尾数形式的整数。
func (BitCurve *BitCurve) ScalarBaseMult(k []byte) (*big.Int, *big.Int) {
	return BitCurve.ScalarMult(BitCurve.Gx, BitCurve.Gy, k)
}

//marshal将点转换为ANSI第4.3.6节中指定的格式
//X962.
func (BitCurve *BitCurve) Marshal(x, y *big.Int) []byte {
	byteLen := (BitCurve.BitSize + 7) >> 3
	ret := make([]byte, 1+2*byteLen)
ret[0] = 4 //未压缩点标志
	readBits(x, ret[1:1+byteLen])
	readBits(y, ret[1+byteLen:])
	return ret
}

//unmarshal将一个点（由marshal序列化）转换为x，y对。论
//误差，x= nIL。
func (BitCurve *BitCurve) Unmarshal(data []byte) (x, y *big.Int) {
	byteLen := (BitCurve.BitSize + 7) >> 3
	if len(data) != 1+2*byteLen {
		return
	}
if data[0] != 4 { //未压缩形式
		return
	}
	x = new(big.Int).SetBytes(data[1 : 1+byteLen])
	y = new(big.Int).SetBytes(data[1+byteLen:])
	return
}

var theCurve = new(BitCurve)

func init() {
//见第2节第2.7.1节
//曲线参数取自：
//http://www.secg.org/sec2-v2.pdf
	theCurve.P, _ = new(big.Int).SetString("0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F", 0)
	theCurve.N, _ = new(big.Int).SetString("0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 0)
	theCurve.B, _ = new(big.Int).SetString("0x0000000000000000000000000000000000000000000000000000000000000007", 0)
	theCurve.Gx, _ = new(big.Int).SetString("0x79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798", 0)
	theCurve.Gy, _ = new(big.Int).SetString("0x483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8", 0)
	theCurve.BitSize = 256
}

//s256返回一个实现secp256k1的bitcurve。
func S256() *BitCurve {
	return theCurve
}
