
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//包BN256以128位安全性实现特定的双线性组
//水平。
//
//双线性群是许多新的密码协议的基础
//在过去的十年中被提议。它们由三组组成
//（g_、g櫒和gt）这样就存在一个函数e（g_，g櫒）=gt_（其中g_
//是相应组的生成器）。这个函数称为配对
//功能。
//
//这个包专门实现了256位以上的最佳ATE配对
//Barreto-Naehrig曲线，如中所述
//http://cryptojedi.org/papers/dclxvi-20100714.pdf.它的输出是兼容的
//以及本文中描述的实现。
package bn256

import (
	"crypto/rand"
	"errors"
	"io"
	"math/big"
)

func randomK(r io.Reader) (k *big.Int, err error) {
	for {
		k, err = rand.Int(r, Order)
		if k.Sign() > 0 || err != nil {
			return
		}
	}
}

//g1是一个抽象的循环群。零值适合用作
//操作的输出，但不能用作输入。
type G1 struct {
	p *curvePoint
}

//randomg1返回x和g_，其中x是从r读取的随机非零数字。
func RandomG1(r io.Reader) (*big.Int, *G1, error) {
	k, err := randomK(r)
	if err != nil {
		return nil, nil, err
	}

	return k, new(G1).ScalarBaseMult(k), nil
}

func (g *G1) String() string {
	return "bn256.G1" + g.p.String()
}

//scalarbasemult将e设置为g*k，其中g是组的生成器，然后
//返回E
func (e *G1) ScalarBaseMult(k *big.Int) *G1 {
	if e.p == nil {
		e.p = &curvePoint{}
	}
	e.p.Mul(curveGen, k)
	return e
}

//scalarmult将e设置为*k，然后返回e。
func (e *G1) ScalarMult(a *G1, k *big.Int) *G1 {
	if e.p == nil {
		e.p = &curvePoint{}
	}
	e.p.Mul(a.p, k)
	return e
}

//将集合e添加到a+b，然后返回e。
func (e *G1) Add(a, b *G1) *G1 {
	if e.p == nil {
		e.p = &curvePoint{}
	}
	e.p.Add(a.p, b.p)
	return e
}

//neg将e设置为-a，然后返回e。
func (e *G1) Neg(a *G1) *G1 {
	if e.p == nil {
		e.p = &curvePoint{}
	}
	e.p.Neg(a.p)
	return e
}

//将e设置为a，然后返回e。
func (e *G1) Set(a *G1) *G1 {
	if e.p == nil {
		e.p = &curvePoint{}
	}
	e.p.Set(a.p)
	return e
}

//封送将E转换为字节片。
func (e *G1) Marshal() []byte {
//每个值都是256位数字。
	const numBytes = 256 / 8

	e.p.MakeAffine()
	ret := make([]byte, numBytes*2)
	if e.p.IsInfinity() {
		return ret
	}
	temp := &gfP{}

	montDecode(temp, &e.p.x)
	temp.Marshal(ret)
	montDecode(temp, &e.p.y)
	temp.Marshal(ret[numBytes:])

	return ret
}

//unmarshal将e设置为将marshal的输出转换回
//一个group元素，然后返回e。
func (e *G1) Unmarshal(m []byte) ([]byte, error) {
//每个值都是256位数字。
	const numBytes = 256 / 8
	if len(m) < 2*numBytes {
		return nil, errors.New("bn256: not enough data")
	}
//解开这些点并检查它们的帽子
	if e.p == nil {
		e.p = &curvePoint{}
	} else {
		e.p.x, e.p.y = gfP{0}, gfP{0}
	}
	var err error
	if err = e.p.x.Unmarshal(m); err != nil {
		return nil, err
	}
	if err = e.p.y.Unmarshal(m[numBytes:]); err != nil {
		return nil, err
	}
//编码成蒙哥马利形式并确保它在曲线上
	montEncode(&e.p.x, &e.p.x)
	montEncode(&e.p.y, &e.p.y)

	zero := gfP{0}
	if e.p.x == zero && e.p.y == zero {
//这是无穷远的点。
		e.p.y = *newGFp(1)
		e.p.z = gfP{0}
		e.p.t = gfP{0}
	} else {
		e.p.z = *newGFp(1)
		e.p.t = *newGFp(1)

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

//randomg2返回x和g，其中x是从r读取的随机非零数字。
func RandomG2(r io.Reader) (*big.Int, *G2, error) {
	k, err := randomK(r)
	if err != nil {
		return nil, nil, err
	}

	return k, new(G2).ScalarBaseMult(k), nil
}

func (e *G2) String() string {
	return "bn256.G2" + e.p.String()
}

//scalarbasemult将e设置为g*k，其中g是组的生成器，然后
//退回。
func (e *G2) ScalarBaseMult(k *big.Int) *G2 {
	if e.p == nil {
		e.p = &twistPoint{}
	}
	e.p.Mul(twistGen, k)
	return e
}

//scalarmult将e设置为*k，然后返回e。
func (e *G2) ScalarMult(a *G2, k *big.Int) *G2 {
	if e.p == nil {
		e.p = &twistPoint{}
	}
	e.p.Mul(a.p, k)
	return e
}

//将集合e添加到a+b，然后返回e。
func (e *G2) Add(a, b *G2) *G2 {
	if e.p == nil {
		e.p = &twistPoint{}
	}
	e.p.Add(a.p, b.p)
	return e
}

//neg将e设置为-a，然后返回e。
func (e *G2) Neg(a *G2) *G2 {
	if e.p == nil {
		e.p = &twistPoint{}
	}
	e.p.Neg(a.p)
	return e
}

//将e设置为a，然后返回e。
func (e *G2) Set(a *G2) *G2 {
	if e.p == nil {
		e.p = &twistPoint{}
	}
	e.p.Set(a.p)
	return e
}

//封送将E转换为字节片。
func (e *G2) Marshal() []byte {
//每个值都是256位数字。
	const numBytes = 256 / 8

	if e.p == nil {
		e.p = &twistPoint{}
	}

	e.p.MakeAffine()
	ret := make([]byte, numBytes*4)
	if e.p.IsInfinity() {
		return ret
	}
	temp := &gfP{}

	montDecode(temp, &e.p.x.x)
	temp.Marshal(ret)
	montDecode(temp, &e.p.x.y)
	temp.Marshal(ret[numBytes:])
	montDecode(temp, &e.p.y.x)
	temp.Marshal(ret[2*numBytes:])
	montDecode(temp, &e.p.y.y)
	temp.Marshal(ret[3*numBytes:])

	return ret
}

//unmarshal将e设置为将marshal的输出转换回
//一个group元素，然后返回e。
func (e *G2) Unmarshal(m []byte) ([]byte, error) {
//每个值都是256位数字。
	const numBytes = 256 / 8
	if len(m) < 4*numBytes {
		return nil, errors.New("bn256: not enough data")
	}
//解开这些点并检查它们的帽子
	if e.p == nil {
		e.p = &twistPoint{}
	}
	var err error
	if err = e.p.x.x.Unmarshal(m); err != nil {
		return nil, err
	}
	if err = e.p.x.y.Unmarshal(m[numBytes:]); err != nil {
		return nil, err
	}
	if err = e.p.y.x.Unmarshal(m[2*numBytes:]); err != nil {
		return nil, err
	}
	if err = e.p.y.y.Unmarshal(m[3*numBytes:]); err != nil {
		return nil, err
	}
//编码成蒙哥马利形式并确保它在曲线上
	montEncode(&e.p.x.x, &e.p.x.x)
	montEncode(&e.p.x.y, &e.p.x.y)
	montEncode(&e.p.y.x, &e.p.y.x)
	montEncode(&e.p.y.y, &e.p.y.y)

	if e.p.x.IsZero() && e.p.y.IsZero() {
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

//配对计算最佳ATE配对。
func Pair(g1 *G1, g2 *G2) *GT {
	return &GT{optimalAte(g2.p, g1.p)}
}

//pairingcheck计算一组点的最佳ate对。
func PairingCheck(a []*G1, b []*G2) bool {
	acc := new(gfP12)
	acc.SetOne()

	for i := 0; i < len(a); i++ {
		if a[i].p.IsInfinity() || b[i].p.IsInfinity() {
			continue
		}
		acc.Mul(acc, miller(b[i].p, a[i].p))
	}
	return finalExponentiation(acc).IsOne()
}

//Miller应用Miller算法，它是
//源组到f_p^12。Miller（g1，g2）.Finalize（）等价于pair（g1，g2）。
//G2）。
func Miller(g1 *G1, g2 *G2) *GT {
	return &GT{miller(g2.p, g1.p)}
}

func (g *GT) String() string {
	return "bn256.GT" + g.p.String()
}

//scalarmult将e设置为*k，然后返回e。
func (e *GT) ScalarMult(a *GT, k *big.Int) *GT {
	if e.p == nil {
		e.p = &gfP12{}
	}
	e.p.Exp(a.p, k)
	return e
}

//将集合e添加到a+b，然后返回e。
func (e *GT) Add(a, b *GT) *GT {
	if e.p == nil {
		e.p = &gfP12{}
	}
	e.p.Mul(a.p, b.p)
	return e
}

//neg将e设置为-a，然后返回e。
func (e *GT) Neg(a *GT) *GT {
	if e.p == nil {
		e.p = &gfP12{}
	}
	e.p.Conjugate(a.p)
	return e
}

//将e设置为a，然后返回e。
func (e *GT) Set(a *GT) *GT {
	if e.p == nil {
		e.p = &gfP12{}
	}
	e.p.Set(a.p)
	return e
}

//Finalize是一个从f_p^12到gt的线性函数。
func (e *GT) Finalize() *GT {
	ret := finalExponentiation(e.p)
	e.p.Set(ret)
	return e
}

//封送将E转换为字节片。
func (e *GT) Marshal() []byte {
//每个值都是256位数字。
	const numBytes = 256 / 8

	ret := make([]byte, numBytes*12)
	temp := &gfP{}

	montDecode(temp, &e.p.x.x.x)
	temp.Marshal(ret)
	montDecode(temp, &e.p.x.x.y)
	temp.Marshal(ret[numBytes:])
	montDecode(temp, &e.p.x.y.x)
	temp.Marshal(ret[2*numBytes:])
	montDecode(temp, &e.p.x.y.y)
	temp.Marshal(ret[3*numBytes:])
	montDecode(temp, &e.p.x.z.x)
	temp.Marshal(ret[4*numBytes:])
	montDecode(temp, &e.p.x.z.y)
	temp.Marshal(ret[5*numBytes:])
	montDecode(temp, &e.p.y.x.x)
	temp.Marshal(ret[6*numBytes:])
	montDecode(temp, &e.p.y.x.y)
	temp.Marshal(ret[7*numBytes:])
	montDecode(temp, &e.p.y.y.x)
	temp.Marshal(ret[8*numBytes:])
	montDecode(temp, &e.p.y.y.y)
	temp.Marshal(ret[9*numBytes:])
	montDecode(temp, &e.p.y.z.x)
	temp.Marshal(ret[10*numBytes:])
	montDecode(temp, &e.p.y.z.y)
	temp.Marshal(ret[11*numBytes:])

	return ret
}

//unmarshal将e设置为将marshal的输出转换回
//一个group元素，然后返回e。
func (e *GT) Unmarshal(m []byte) ([]byte, error) {
//每个值都是256位数字。
	const numBytes = 256 / 8

	if len(m) < 12*numBytes {
		return nil, errors.New("bn256: not enough data")
	}

	if e.p == nil {
		e.p = &gfP12{}
	}

	var err error
	if err = e.p.x.x.x.Unmarshal(m); err != nil {
		return nil, err
	}
	if err = e.p.x.x.y.Unmarshal(m[numBytes:]); err != nil {
		return nil, err
	}
	if err = e.p.x.y.x.Unmarshal(m[2*numBytes:]); err != nil {
		return nil, err
	}
	if err = e.p.x.y.y.Unmarshal(m[3*numBytes:]); err != nil {
		return nil, err
	}
	if err = e.p.x.z.x.Unmarshal(m[4*numBytes:]); err != nil {
		return nil, err
	}
	if err = e.p.x.z.y.Unmarshal(m[5*numBytes:]); err != nil {
		return nil, err
	}
	if err = e.p.y.x.x.Unmarshal(m[6*numBytes:]); err != nil {
		return nil, err
	}
	if err = e.p.y.x.y.Unmarshal(m[7*numBytes:]); err != nil {
		return nil, err
	}
	if err = e.p.y.y.x.Unmarshal(m[8*numBytes:]); err != nil {
		return nil, err
	}
	if err = e.p.y.y.y.Unmarshal(m[9*numBytes:]); err != nil {
		return nil, err
	}
	if err = e.p.y.z.x.Unmarshal(m[10*numBytes:]); err != nil {
		return nil, err
	}
	if err = e.p.y.z.y.Unmarshal(m[11*numBytes:]); err != nil {
		return nil, err
	}
	montEncode(&e.p.x.x.x, &e.p.x.x.x)
	montEncode(&e.p.x.x.y, &e.p.x.x.y)
	montEncode(&e.p.x.y.x, &e.p.x.y.x)
	montEncode(&e.p.x.y.y, &e.p.x.y.y)
	montEncode(&e.p.x.z.x, &e.p.x.z.x)
	montEncode(&e.p.x.z.y, &e.p.x.z.y)
	montEncode(&e.p.y.x.x, &e.p.y.x.x)
	montEncode(&e.p.y.x.y, &e.p.y.x.y)
	montEncode(&e.p.y.y.x, &e.p.y.y.x)
	montEncode(&e.p.y.y.y, &e.p.y.y.y)
	montEncode(&e.p.y.z.x, &e.p.y.z.x)
	montEncode(&e.p.y.z.y, &e.p.y.z.y)

	return m[12*numBytes:], nil
}
