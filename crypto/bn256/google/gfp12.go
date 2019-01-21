
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
x, y *gfP6 //值为xω+y
}

func newGFp12(pool *bnPool) *gfP12 {
	return &gfP12{newGFp6(pool), newGFp6(pool)}
}

func (e *gfP12) String() string {
	return "(" + e.x.String() + "," + e.y.String() + ")"
}

func (e *gfP12) Put(pool *bnPool) {
	e.x.Put(pool)
	e.y.Put(pool)
}

func (e *gfP12) Set(a *gfP12) *gfP12 {
	e.x.Set(a.x)
	e.y.Set(a.y)
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

func (e *gfP12) Minimal() {
	e.x.Minimal()
	e.y.Minimal()
}

func (e *gfP12) IsZero() bool {
	e.Minimal()
	return e.x.IsZero() && e.y.IsZero()
}

func (e *gfP12) IsOne() bool {
	e.Minimal()
	return e.x.IsZero() && e.y.IsOne()
}

func (e *gfP12) Conjugate(a *gfP12) *gfP12 {
	e.x.Negative(a.x)
	e.y.Set(a.y)
	return a
}

func (e *gfP12) Negative(a *gfP12) *gfP12 {
	e.x.Negative(a.x)
	e.y.Negative(a.y)
	return e
}

//弗罗贝尼乌斯计算（xω+y）^p=x^pω·ξ^（（p-1）/6）+y^p
func (e *gfP12) Frobenius(a *gfP12, pool *bnPool) *gfP12 {
	e.x.Frobenius(a.x, pool)
	e.y.Frobenius(a.y, pool)
	e.x.MulScalar(e.x, xiToPMinus1Over6, pool)
	return e
}

//frobiniusp2计算（xω+y）^p²=x^p²ω·ξ^（（p²-1）/6）+y^p²
func (e *gfP12) FrobeniusP2(a *gfP12, pool *bnPool) *gfP12 {
	e.x.FrobeniusP2(a.x)
	e.x.MulGFP(e.x, xiToPSquaredMinus1Over6)
	e.y.FrobeniusP2(a.y)
	return e
}

func (e *gfP12) Add(a, b *gfP12) *gfP12 {
	e.x.Add(a.x, b.x)
	e.y.Add(a.y, b.y)
	return e
}

func (e *gfP12) Sub(a, b *gfP12) *gfP12 {
	e.x.Sub(a.x, b.x)
	e.y.Sub(a.y, b.y)
	return e
}

func (e *gfP12) Mul(a, b *gfP12, pool *bnPool) *gfP12 {
	tx := newGFp6(pool)
	tx.Mul(a.x, b.y, pool)
	t := newGFp6(pool)
	t.Mul(b.x, a.y, pool)
	tx.Add(tx, t)

	ty := newGFp6(pool)
	ty.Mul(a.y, b.y, pool)
	t.Mul(a.x, b.x, pool)
	t.MulTau(t, pool)
	e.y.Add(ty, t)
	e.x.Set(tx)

	tx.Put(pool)
	ty.Put(pool)
	t.Put(pool)
	return e
}

func (e *gfP12) MulScalar(a *gfP12, b *gfP6, pool *bnPool) *gfP12 {
	e.x.Mul(e.x, b, pool)
	e.y.Mul(e.y, b, pool)
	return e
}

func (c *gfP12) Exp(a *gfP12, power *big.Int, pool *bnPool) *gfP12 {
	sum := newGFp12(pool)
	sum.SetOne()
	t := newGFp12(pool)

	for i := power.BitLen() - 1; i >= 0; i-- {
		t.Square(sum, pool)
		if power.Bit(i) != 0 {
			sum.Mul(t, a, pool)
		} else {
			sum.Set(t)
		}
	}

	c.Set(sum)

	sum.Put(pool)
	t.Put(pool)

	return c
}

func (e *gfP12) Square(a *gfP12, pool *bnPool) *gfP12 {
//复平方算法
	v0 := newGFp6(pool)
	v0.Mul(a.x, a.y, pool)

	t := newGFp6(pool)
	t.MulTau(a.x, pool)
	t.Add(a.y, t)
	ty := newGFp6(pool)
	ty.Add(a.x, a.y)
	ty.Mul(ty, t, pool)
	ty.Sub(ty, v0)
	t.MulTau(v0, pool)
	ty.Sub(ty, t)

	e.y.Set(ty)
	e.x.Double(v0)

	v0.Put(pool)
	t.Put(pool)
	ty.Put(pool)

	return e
}

func (e *gfP12) Invert(a *gfP12, pool *bnPool) *gfP12 {
//见“实现密码配对”，M.Scott，第3.2节。
//ftp://136.206.11.249/pub/crypto/pairing.pdf文件
	t1 := newGFp6(pool)
	t2 := newGFp6(pool)

	t1.Square(a.x, pool)
	t2.Square(a.y, pool)
	t1.MulTau(t1, pool)
	t1.Sub(t2, t1)
	t2.Invert(t1, pool)

	e.x.Negative(a.x)
	e.y.Set(a.y)
	e.MulScalar(e, t2, pool)

	t1.Put(pool)
	t2.Put(pool)

	return e
}
