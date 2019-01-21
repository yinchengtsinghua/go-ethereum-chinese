
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2012 Go作者。版权所有。
//此源代码的使用受BSD样式的控制
//可以在许可文件中找到的许可证。

//包BN256实现特定的双线性组。
//
//双线性群是许多新密码协议的基础。
//在过去的十年里提出的。它们由三个组成
//组（g_、g嫪和gt），以便存在一个函数e（g_、g嫪）=gt_
//（其中g_是相应组的生成器）。该函数被调用
//配对功能。
//
//这个包专门实现了256位以上的最佳ATE配对
//Barreto-Naehrig曲线，如中所述
//http://cryptojedi.org/papers/dclxvi-20100714.pdf.它的输出是兼容的
//以及本文中描述的实现。
//
//（此软件包以前声称在128位安全级别下运行。
//然而，最近对攻击的改进意味着这不再是真的了。见
//https://moderncrypto.org/mail archive/curves/2016/000740.html.网址）
package bn256

import (
	"crypto/rand"
	"errors"
	"io"
	"math/big"
)

//bug（agl）：这个实现不是固定的时间。
//todo（agl）：保持gf（p²）元素为蒙古语形式。

//g1是一个抽象的循环群。零值适合用作
//操作的输出，但不能用作输入。
type G1 struct {
	p *curvePoint
}

//randomg1返回x和g_，其中x是从r读取的随机非零数字。
func RandomG1(r io.Reader) (*big.Int, *G1, error) {
	var k *big.Int
	var err error

	for {
		k, err = rand.Int(r, Order)
		if err != nil {
			return nil, nil, err
		}
		if k.Sign() > 0 {
			break
		}
	}

	return k, new(G1).ScalarBaseMult(k), nil
}

func (e *G1) String() string {
	return "bn256.G1" + e.p.String()
}

//curve points以大整数返回p的曲线点
func (e *G1) CurvePoints() (*big.Int, *big.Int, *big.Int, *big.Int) {
	return e.p.x, e.p.y, e.p.z, e.p.t
}

//scalarbasemult将e设置为g*k，其中g是组的生成器，并且
//然后返回E。
func (e *G1) ScalarBaseMult(k *big.Int) *G1 {
	if e.p == nil {
		e.p = newCurvePoint(nil)
	}
	e.p.Mul(curveGen, k, new(bnPool))
	return e
}

//scalarmult将e设置为*k，然后返回e。
func (e *G1) ScalarMult(a *G1, k *big.Int) *G1 {
	if e.p == nil {
		e.p = newCurvePoint(nil)
	}
	e.p.Mul(a.p, k, new(bnPool))
	return e
}

//将集合e添加到a+b，然后返回e。
//bug（agl）：此函数不完整：a==b失败。
func (e *G1) Add(a, b *G1) *G1 {
	if e.p == nil {
		e.p = newCurvePoint(nil)
	}
	e.p.Add(a.p, b.p, new(bnPool))
	return e
}

//neg将e设置为-a，然后返回e。
func (e *G1) Neg(a *G1) *G1 {
	if e.p == nil {
		e.p = newCurvePoint(nil)
	}
	e.p.Negative(a.p)
	return e
}

//marshal将n转换为字节片。
func (e *G1) Marshal() []byte {
//每个值都是256位数字。
	const numBytes = 256 / 8

	if e.p.IsInfinity() {
		return make([]byte, numBytes*2)
	}

	e.p.MakeAffine(nil)

	xBytes := new(big.Int).Mod(e.p.x, P).Bytes()
	yBytes := new(big.Int).Mod(e.p.y, P).Bytes()

	ret := make([]byte, numBytes*2)
	copy(ret[1*numBytes-len(xBytes):], xBytes)
	copy(ret[2*numBytes-len(yBytes):], yBytes)

	return ret
}

//unmarshal将e设置为将marshal的输出转换回
//一个group元素，然后返回e。
func (e *G1) Unmarshal(m []byte) ([]byte, error) {
//每个值都是256位数字。
	const numBytes = 256 / 8
	if len(m) != 2*numBytes {
		return nil, errors.New("bn256: not enough data")
	}
//解开这些点并检查它们的帽子
	if e.p == nil {
		e.p = newCurvePoint(nil)
	}
	e.p.x.SetBytes(m[0*numBytes : 1*numBytes])
	if e.p.x.Cmp(P) >= 0 {
		return nil, errors.New("bn256: coordinate exceeds modulus")
	}
	e.p.y.SetBytes(m[1*numBytes : 2*numBytes])
	if e.p.y.Cmp(P) >= 0 {
		return nil, errors.New("bn256: coordinate exceeds modulus")
	}
//确保点在曲线上
	if e.p.x.Sign() == 0 && e.p.y.Sign() == 0 {
//这是无穷远的点。
		e.p.y.SetInt64(1)
		e.p.z.SetInt64(0)
		e.p.t.SetInt64(0)
	} else {
		e.p.z.SetInt64(1)
		e.p.t.SetInt64(1)

		if !e.p.IsOnCurve() {
			return nil, errors.New("bn256: malformed point")
		}
	}
	return m[2*numBytes:], nil
}

//g2是一个抽象的循环群。零值适合用作
//操作的输出，但不能用作输入。
type G2 struct {
	p *twistPoint
}

//randomg1返回x和g，其中x是从r读取的随机非零数字。
func RandomG2(r io.Reader) (*big.Int, *G2, error) {
	var k *big.Int
	var err error

	for {
		k, err = rand.Int(r, Order)
		if err != nil {
			return nil, nil, err
		}
		if k.Sign() > 0 {
			break
		}
	}

	return k, new(G2).ScalarBaseMult(k), nil
}

func (e *G2) String() string {
	return "bn256.G2" + e.p.String()
}

//curve points返回p的曲线点，其中包括
//以及曲线点的虚部。
func (e *G2) CurvePoints() (*gfP2, *gfP2, *gfP2, *gfP2) {
	return e.p.x, e.p.y, e.p.z, e.p.t
}

//scalarbasemult将e设置为g*k，其中g是组的生成器，并且
//然后返回。
func (e *G2) ScalarBaseMult(k *big.Int) *G2 {
	if e.p == nil {
		e.p = newTwistPoint(nil)
	}
	e.p.Mul(twistGen, k, new(bnPool))
	return e
}

//scalarmult将e设置为*k，然后返回e。
func (e *G2) ScalarMult(a *G2, k *big.Int) *G2 {
	if e.p == nil {
		e.p = newTwistPoint(nil)
	}
	e.p.Mul(a.p, k, new(bnPool))
	return e
}

//将集合e添加到a+b，然后返回e。
//bug（agl）：此函数不完整：a==b失败。
func (e *G2) Add(a, b *G2) *G2 {
	if e.p == nil {
		e.p = newTwistPoint(nil)
	}
	e.p.Add(a.p, b.p, new(bnPool))
	return e
}

//marshal将n转换为字节片。
func (n *G2) Marshal() []byte {
//每个值都是256位数字。
	const numBytes = 256 / 8

	if n.p.IsInfinity() {
		return make([]byte, numBytes*4)
	}

	n.p.MakeAffine(nil)

	xxBytes := new(big.Int).Mod(n.p.x.x, P).Bytes()
	xyBytes := new(big.Int).Mod(n.p.x.y, P).Bytes()
	yxBytes := new(big.Int).Mod(n.p.y.x, P).Bytes()
	yyBytes := new(big.Int).Mod(n.p.y.y, P).Bytes()

	ret := make([]byte, numBytes*4)
	copy(ret[1*numBytes-len(xxBytes):], xxBytes)
	copy(ret[2*numBytes-len(xyBytes):], xyBytes)
	copy(ret[3*numBytes-len(yxBytes):], yxBytes)
	copy(ret[4*numBytes-len(yyBytes):], yyBytes)

	return ret
}

//unmarshal将e设置为将marshal的输出转换回
//一个group元素，然后返回e。
func (e *G2) Unmarshal(m []byte) ([]byte, error) {
//每个值都是256位数字。
	const numBytes = 256 / 8
	if len(m) != 4*numBytes {
		return nil, errors.New("bn256: not enough data")
	}
//解开这些点并检查它们的帽子
	if e.p == nil {
		e.p = newTwistPoint(nil)
	}
	e.p.x.x.SetBytes(m[0*numBytes : 1*numBytes])
	if e.p.x.x.Cmp(P) >= 0 {
		return nil, errors.New("bn256: coordinate exceeds modulus")
	}
	e.p.x.y.SetBytes(m[1*numBytes : 2*numBytes])
	if e.p.x.y.Cmp(P) >= 0 {
		return nil, errors.New("bn256: coordinate exceeds modulus")
	}
	e.p.y.x.SetBytes(m[2*numBytes : 3*numBytes])
	if e.p.y.x.Cmp(P) >= 0 {
		return nil, errors.New("bn256: coordinate exceeds modulus")
	}
	e.p.y.y.SetBytes(m[3*numBytes : 4*numBytes])
	if e.p.y.y.Cmp(P) >= 0 {
		return nil, errors.New("bn256: coordinate exceeds modulus")
	}
//确保点在曲线上
	if e.p.x.x.Sign() == 0 &&
		e.p.x.y.Sign() == 0 &&
		e.p.y.x.Sign() == 0 &&
		e.p.y.y.Sign() == 0 {
//这是无穷远的点。
		e.p.y.SetOne()
		e.p.z.SetZero()
		e.p.t.SetZero()
	} else {
		e.p.z.SetOne()
		e.p.t.SetOne()

		if !e.p.IsOnCurve() {
			return nil, errors.New("bn256: malformed point")
		}
	}
	return m[4*numBytes:], nil
}

//gt是一个抽象循环群。零值适合用作
//操作的输出，但不能用作输入。
type GT struct {
	p *gfP12
}

func (g *GT) String() string {
	return "bn256.GT" + g.p.String()
}

//scalarmult将e设置为*k，然后返回e。
func (e *GT) ScalarMult(a *GT, k *big.Int) *GT {
	if e.p == nil {
		e.p = newGFp12(nil)
	}
	e.p.Exp(a.p, k, new(bnPool))
	return e
}

//将集合e添加到a+b，然后返回e。
func (e *GT) Add(a, b *GT) *GT {
	if e.p == nil {
		e.p = newGFp12(nil)
	}
	e.p.Mul(a.p, b.p, new(bnPool))
	return e
}

//neg将e设置为-a，然后返回e。
func (e *GT) Neg(a *GT) *GT {
	if e.p == nil {
		e.p = newGFp12(nil)
	}
	e.p.Invert(a.p, new(bnPool))
	return e
}

//marshal将n转换为字节片。
func (n *GT) Marshal() []byte {
	n.p.Minimal()

	xxxBytes := n.p.x.x.x.Bytes()
	xxyBytes := n.p.x.x.y.Bytes()
	xyxBytes := n.p.x.y.x.Bytes()
	xyyBytes := n.p.x.y.y.Bytes()
	xzxBytes := n.p.x.z.x.Bytes()
	xzyBytes := n.p.x.z.y.Bytes()
	yxxBytes := n.p.y.x.x.Bytes()
	yxyBytes := n.p.y.x.y.Bytes()
	yyxBytes := n.p.y.y.x.Bytes()
	yyyBytes := n.p.y.y.y.Bytes()
	yzxBytes := n.p.y.z.x.Bytes()
	yzyBytes := n.p.y.z.y.Bytes()

//每个值都是256位数字。
	const numBytes = 256 / 8

	ret := make([]byte, numBytes*12)
	copy(ret[1*numBytes-len(xxxBytes):], xxxBytes)
	copy(ret[2*numBytes-len(xxyBytes):], xxyBytes)
	copy(ret[3*numBytes-len(xyxBytes):], xyxBytes)
	copy(ret[4*numBytes-len(xyyBytes):], xyyBytes)
	copy(ret[5*numBytes-len(xzxBytes):], xzxBytes)
	copy(ret[6*numBytes-len(xzyBytes):], xzyBytes)
	copy(ret[7*numBytes-len(yxxBytes):], yxxBytes)
	copy(ret[8*numBytes-len(yxyBytes):], yxyBytes)
	copy(ret[9*numBytes-len(yyxBytes):], yyxBytes)
	copy(ret[10*numBytes-len(yyyBytes):], yyyBytes)
	copy(ret[11*numBytes-len(yzxBytes):], yzxBytes)
	copy(ret[12*numBytes-len(yzyBytes):], yzyBytes)

	return ret
}

//unmarshal将e设置为将marshal的输出转换回
//一个group元素，然后返回e。
func (e *GT) Unmarshal(m []byte) (*GT, bool) {
//每个值都是256位数字。
	const numBytes = 256 / 8

	if len(m) != 12*numBytes {
		return nil, false
	}

	if e.p == nil {
		e.p = newGFp12(nil)
	}

	e.p.x.x.x.SetBytes(m[0*numBytes : 1*numBytes])
	e.p.x.x.y.SetBytes(m[1*numBytes : 2*numBytes])
	e.p.x.y.x.SetBytes(m[2*numBytes : 3*numBytes])
	e.p.x.y.y.SetBytes(m[3*numBytes : 4*numBytes])
	e.p.x.z.x.SetBytes(m[4*numBytes : 5*numBytes])
	e.p.x.z.y.SetBytes(m[5*numBytes : 6*numBytes])
	e.p.y.x.x.SetBytes(m[6*numBytes : 7*numBytes])
	e.p.y.x.y.SetBytes(m[7*numBytes : 8*numBytes])
	e.p.y.y.x.SetBytes(m[8*numBytes : 9*numBytes])
	e.p.y.y.y.SetBytes(m[9*numBytes : 10*numBytes])
	e.p.y.z.x.SetBytes(m[10*numBytes : 11*numBytes])
	e.p.y.z.y.SetBytes(m[11*numBytes : 12*numBytes])

	return e, true
}

//配对计算最佳ATE配对。
func Pair(g1 *G1, g2 *G2) *GT {
	return &GT{optimalAte(g2.p, g1.p, new(bnPool))}
}

//pairingcheck计算一组点的最佳ate对。
func PairingCheck(a []*G1, b []*G2) bool {
	pool := new(bnPool)

	acc := newGFp12(pool)
	acc.SetOne()

	for i := 0; i < len(a); i++ {
		if a[i].p.IsInfinity() || b[i].p.IsInfinity() {
			continue
		}
		acc.Mul(acc, miller(b[i].p, a[i].p, pool), pool)
	}
	ret := finalExponentiation(acc, pool)
	acc.Put(pool)

	return ret.IsOne()
}

//bnpool实现一个*big.int对象的小缓存，用于减少
//处理期间进行的分配数。
type bnPool struct {
	bns   []*big.Int
	count int
}

func (pool *bnPool) Get() *big.Int {
	if pool == nil {
		return new(big.Int)
	}

	pool.count++
	l := len(pool.bns)
	if l == 0 {
		return new(big.Int)
	}

	bn := pool.bns[l-1]
	pool.bns = pool.bns[:l-1]
	return bn
}

func (pool *bnPool) Put(bn *big.Int) {
	if pool == nil {
		return
	}
	pool.bns = append(pool.bns, bn)
	pool.count--
}

func (pool *bnPool) Count() int {
	return pool.count
}
