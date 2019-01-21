
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package bn256

//有关所用算法的详细信息，请参阅
//配对友好字段，Devegili等人
//http://eprint.iacr.org/2006/471.pdf。

import (
	"math/big"
)

//gfp12将大小为p的字段实现为gfp6的二次扩展
//其中ω＝TA.
type gfP12 struct {
x, y gfP6 //值为xω+y
}

func (e *gfP12) String() string {
	return "(" + e.x.String() + "," + e.y.String() + ")"
}

func (e *gfP12) Set(a *gfP12) *gfP12 {
	e.x.Set(&a.x)
	e.y.Set(&a.y)
	return e
}

func (e *gfP12) SetZero() *gfP12 {
	e.x.SetZero()
	e.y.SetZero()
	return e
}

func (e *gfP12) SetOne() *gfP12 {
	e.x.SetZero()
	e.y.SetOne()
	return e
}

func (e *gfP12) IsZero() bool {
	return e.x.IsZero() && e.y.IsZero()
}

func (e *gfP12) IsOne() bool {
	return e.x.IsZero() && e.y.IsOne()
}

func (e *gfP12) Conjugate(a *gfP12) *gfP12 {
	e.x.Neg(&a.x)
	e.y.Set(&a.y)
	return e
}

func (e *gfP12) Neg(a *gfP12) *gfP12 {
	e.x.Neg(&a.x)
	e.y.Neg(&a.y)
	return e
}

//弗罗贝尼乌斯计算（xω+y）^p=x^pω·ξ^（（p-1）/6）+y^p
func (e *gfP12) Frobenius(a *gfP12) *gfP12 {
	e.x.Frobenius(&a.x)
	e.y.Frobenius(&a.y)
	e.x.MulScalar(&e.x, xiToPMinus1Over6)
	return e
}

//frobiniusp2计算（xω+y）^p²=x^p²ω·ξ^（（p²-1）/6）+y^p²
func (e *gfP12) FrobeniusP2(a *gfP12) *gfP12 {
	e.x.FrobeniusP2(&a.x)
	e.x.MulGFP(&e.x, xiToPSquaredMinus1Over6)
	e.y.FrobeniusP2(&a.y)
	return e
}

func (e *gfP12) FrobeniusP4(a *gfP12) *gfP12 {
	e.x.FrobeniusP4(&a.x)
	e.x.MulGFP(&e.x, xiToPSquaredMinus1Over3)
	e.y.FrobeniusP4(&a.y)
	return e
}

func (e *gfP12) Add(a, b *gfP12) *gfP12 {
	e.x.Add(&a.x, &b.x)
	e.y.Add(&a.y, &b.y)
	return e
}

func (e *gfP12) Sub(a, b *gfP12) *gfP12 {
	e.x.Sub(&a.x, &b.x)
	e.y.Sub(&a.y, &b.y)
	return e
}

func (e *gfP12) Mul(a, b *gfP12) *gfP12 {
	tx := (&gfP6{}).Mul(&a.x, &b.y)
	t := (&gfP6{}).Mul(&b.x, &a.y)
	tx.Add(tx, t)

	ty := (&gfP6{}).Mul(&a.y, &b.y)
	t.Mul(&a.x, &b.x).MulTau(t)

	e.x.Set(tx)
	e.y.Add(ty, t)
	return e
}

func (e *gfP12) MulScalar(a *gfP12, b *gfP6) *gfP12 {
	e.x.Mul(&e.x, b)
	e.y.Mul(&e.y, b)
	return e
}

func (c *gfP12) Exp(a *gfP12, power *big.Int) *gfP12 {
	sum := (&gfP12{}).SetOne()
	t := &gfP12{}

	for i := power.BitLen() - 1; i >= 0; i-- {
		t.Square(sum)
		if power.Bit(i) != 0 {
			sum.Mul(t, a)
		} else {
			sum.Set(t)
		}
	}

	c.Set(sum)
	return c
}

func (e *gfP12) Square(a *gfP12) *gfP12 {
//复平方算法
	v0 := (&gfP6{}).Mul(&a.x, &a.y)

	t := (&gfP6{}).MulTau(&a.x)
	t.Add(&a.y, t)
	ty := (&gfP6{}).Add(&a.x, &a.y)
	ty.Mul(ty, t).Sub(ty, v0)
	t.MulTau(v0)
	ty.Sub(ty, t)

	e.x.Add(v0, v0)
	e.y.Set(ty)
	return e
}

func (e *gfP12) Invert(a *gfP12) *gfP12 {
//见“实现密码配对”，M.Scott，第3.2节。
//ftp://136.206.11.249/pub/crypto/pairing.pdf文件
	t1, t2 := &gfP6{}, &gfP6{}

	t1.Square(&a.x)
	t2.Square(&a.y)
	t1.MulTau(t1).Sub(t2, t1)
	t2.Invert(t1)

	e.x.Neg(&a.x)
	e.y.Set(&a.y)
	e.MulScalar(e, t2)
	return e
}
