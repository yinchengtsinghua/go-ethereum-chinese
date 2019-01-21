
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

//gfp2实现了一个大小为p²的字段作为基的二次扩展
//字段，其中i？=-1。
type gfP2 struct {
x, y *big.Int //值是X+Y。
}

func newGFp2(pool *bnPool) *gfP2 {
	return &gfP2{pool.Get(), pool.Get()}
}

func (e *gfP2) String() string {
	x := new(big.Int).Mod(e.x, P)
	y := new(big.Int).Mod(e.y, P)
	return "(" + x.String() + "," + y.String() + ")"
}

func (e *gfP2) Put(pool *bnPool) {
	pool.Put(e.x)
	pool.Put(e.y)
}

func (e *gfP2) Set(a *gfP2) *gfP2 {
	e.x.Set(a.x)
	e.y.Set(a.y)
	return e
}

func (e *gfP2) SetZero() *gfP2 {
	e.x.SetInt64(0)
	e.y.SetInt64(0)
	return e
}

func (e *gfP2) SetOne() *gfP2 {
	e.x.SetInt64(0)
	e.y.SetInt64(1)
	return e
}

func (e *gfP2) Minimal() {
	if e.x.Sign() < 0 || e.x.Cmp(P) >= 0 {
		e.x.Mod(e.x, P)
	}
	if e.y.Sign() < 0 || e.y.Cmp(P) >= 0 {
		e.y.Mod(e.y, P)
	}
}

func (e *gfP2) IsZero() bool {
	return e.x.Sign() == 0 && e.y.Sign() == 0
}

func (e *gfP2) IsOne() bool {
	if e.x.Sign() != 0 {
		return false
	}
	words := e.y.Bits()
	return len(words) == 1 && words[0] == 1
}

func (e *gfP2) Conjugate(a *gfP2) *gfP2 {
	e.y.Set(a.y)
	e.x.Neg(a.x)
	return e
}

func (e *gfP2) Negative(a *gfP2) *gfP2 {
	e.x.Neg(a.x)
	e.y.Neg(a.y)
	return e
}

func (e *gfP2) Add(a, b *gfP2) *gfP2 {
	e.x.Add(a.x, b.x)
	e.y.Add(a.y, b.y)
	return e
}

func (e *gfP2) Sub(a, b *gfP2) *gfP2 {
	e.x.Sub(a.x, b.x)
	e.y.Sub(a.y, b.y)
	return e
}

func (e *gfP2) Double(a *gfP2) *gfP2 {
	e.x.Lsh(a.x, 1)
	e.y.Lsh(a.y, 1)
	return e
}

func (c *gfP2) Exp(a *gfP2, power *big.Int, pool *bnPool) *gfP2 {
	sum := newGFp2(pool)
	sum.SetOne()
	t := newGFp2(pool)

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

//参见“配对友好字段中的乘法和平方”，
//http://eprint.iacr.org/2006/471.pdf
func (e *gfP2) Mul(a, b *gfP2, pool *bnPool) *gfP2 {
	tx := pool.Get().Mul(a.x, b.y)
	t := pool.Get().Mul(b.x, a.y)
	tx.Add(tx, t)
	tx.Mod(tx, P)

	ty := pool.Get().Mul(a.y, b.y)
	t.Mul(a.x, b.x)
	ty.Sub(ty, t)
	e.y.Mod(ty, P)
	e.x.Set(tx)

	pool.Put(tx)
	pool.Put(ty)
	pool.Put(t)

	return e
}

func (e *gfP2) MulScalar(a *gfP2, b *big.Int) *gfP2 {
	e.x.Mul(a.x, b)
	e.y.Mul(a.y, b)
	return e
}

//mulxi设置e=ξa，其中ξ=i+9，然后返回e。
func (e *gfP2) MulXi(a *gfP2, pool *bnPool) *gfP2 {
//（X+Y）（i＋3）＝（9x+y）i+（9y- x）
	tx := pool.Get().Lsh(a.x, 3)
	tx.Add(tx, a.x)
	tx.Add(tx, a.y)

	ty := pool.Get().Lsh(a.y, 3)
	ty.Add(ty, a.y)
	ty.Sub(ty, a.x)

	e.x.Set(tx)
	e.y.Set(ty)

	pool.Put(tx)
	pool.Put(ty)

	return e
}

func (e *gfP2) Square(a *gfP2, pool *bnPool) *gfP2 {
//复杂平方算法：
//（X+B）＝（x+y）（y- x）+2＊i*x*y
	t1 := pool.Get().Sub(a.y, a.x)
	t2 := pool.Get().Add(a.x, a.y)
	ty := pool.Get().Mul(t1, t2)
	ty.Mod(ty, P)

	t1.Mul(a.x, a.y)
	t1.Lsh(t1, 1)

	e.x.Mod(t1, P)
	e.y.Set(ty)

	pool.Put(t1)
	pool.Put(t2)
	pool.Put(ty)

	return e
}

func (e *gfP2) Invert(a *gfP2, pool *bnPool) *gfP2 {
//见“实现密码配对”，M.Scott，第3.2节。
//ftp://136.206.11.249/pub/crypto/pairing.pdf文件
	t := pool.Get()
	t.Mul(a.y, a.y)
	t2 := pool.Get()
	t2.Mul(a.x, a.x)
	t.Add(t, t2)

	inv := pool.Get()
	inv.ModInverse(t, P)

	e.x.Neg(a.x)
	e.x.Mul(e.x, inv)
	e.x.Mod(e.x, P)

	e.y.Mul(a.y, inv)
	e.y.Mod(e.y, P)

	pool.Put(t)
	pool.Put(t2)
	pool.Put(inv)

	return e
}

func (e *gfP2) Real() *big.Int {
	return e.x
}

func (e *gfP2) Imag() *big.Int {
	return e.y
}
