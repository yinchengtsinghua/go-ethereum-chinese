
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

import (
	"math/big"
)

//curvePoint实现椭圆曲线y？=x？+3。积分保留在
//雅可比形式，有效时t=z²。G_是曲线上的一组点
//GF（P）。
type curvePoint struct {
	x, y, z, t *big.Int
}

var curveB = new(big.Int).SetInt64(3)

//曲线发电机是G_的发电机。
var curveGen = &curvePoint{
	new(big.Int).SetInt64(1),
	new(big.Int).SetInt64(2),
	new(big.Int).SetInt64(1),
	new(big.Int).SetInt64(1),
}

func newCurvePoint(pool *bnPool) *curvePoint {
	return &curvePoint{
		pool.Get(),
		pool.Get(),
		pool.Get(),
		pool.Get(),
	}
}

func (c *curvePoint) String() string {
	c.MakeAffine(new(bnPool))
	return "(" + c.x.String() + ", " + c.y.String() + ")"
}

func (c *curvePoint) Put(pool *bnPool) {
	pool.Put(c.x)
	pool.Put(c.y)
	pool.Put(c.z)
	pool.Put(c.t)
}

func (c *curvePoint) Set(a *curvePoint) {
	c.x.Set(a.x)
	c.y.Set(a.y)
	c.z.Set(a.z)
	c.t.Set(a.t)
}

//is on curve返回真正的iff c在曲线上，其中c必须是仿射形式。
func (c *curvePoint) IsOnCurve() bool {
	yy := new(big.Int).Mul(c.y, c.y)
	xxx := new(big.Int).Mul(c.x, c.x)
	xxx.Mul(xxx, c.x)
	yy.Sub(yy, xxx)
	yy.Sub(yy, curveB)
	if yy.Sign() < 0 || yy.Cmp(P) >= 0 {
		yy.Mod(yy, P)
	}
	return yy.Sign() == 0
}

func (c *curvePoint) SetInfinity() {
	c.z.SetInt64(0)
}

func (c *curvePoint) IsInfinity() bool {
	return c.z.Sign() == 0
}

func (c *curvePoint) Add(a, b *curvePoint, pool *bnPool) {
	if a.IsInfinity() {
		c.Set(b)
		return
	}
	if b.IsInfinity() {
		c.Set(a)
		return
	}

//见http://hyper椭圆形.org/efd/g1p/auto-code/shortw/jacobian-0/addition/add-2007-bl.op3

//通过替换a=[x1:y1:z1]和b=[x2:y2:z2]来规范化点。
//通过[U1:S1:Z1·Z2]和[U2:S2:Z1·Z2]
//式中：U1=x1·z2？、S1=y1·z2？、U1=x2·z1？、S2=y2·z1？？
	z1z1 := pool.Get().Mul(a.z, a.z)
	z1z1.Mod(z1z1, P)
	z2z2 := pool.Get().Mul(b.z, b.z)
	z2z2.Mod(z2z2, P)
	u1 := pool.Get().Mul(a.x, z2z2)
	u1.Mod(u1, P)
	u2 := pool.Get().Mul(b.x, z1z1)
	u2.Mod(u2, P)

	t := pool.Get().Mul(b.z, z2z2)
	t.Mod(t, P)
	s1 := pool.Get().Mul(a.y, t)
	s1.Mod(s1, P)

	t.Mul(a.z, z1z1)
	t.Mod(t, P)
	s2 := pool.Get().Mul(b.y, t)
	s2.Mod(s2, P)

//计算x=（2h）2（s²-u1-u2）
//其中s=（s2-s1）/（u2-u1）是线路通过的坡度
//（U1、S1）和（U2、S2）。额外因子2h=2（u2-u1）来自下面的z值。
//这也是：
//4（s2-s1）2-4h 2（u1+u2）=4（s2-s1）2-4h 3-4h 2（2u1）
//R＝J-2V
//以及下面的注释。
	h := pool.Get().Sub(u2, u1)
	xEqual := h.Sign() == 0

	t.Add(h, h)
//I= 4H
	i := pool.Get().Mul(t, t)
	i.Mod(i, P)
//J= 4H
	j := pool.Get().Mul(h, i)
	j.Mod(j, P)

	t.Sub(s2, s1)
	yEqual := t.Sign() == 0
	if xEqual && yEqual {
		c.Double(a, pool)
		return
	}
	r := pool.Get().Add(t, t)

	v := pool.Get().Mul(u1, i)
	v.Mod(v, P)

//T4＝4（S2-S1）
	t4 := pool.Get().Mul(r, r)
	t4.Mod(t4, P)
	t.Add(v, v)
	t6 := pool.Get().Sub(t4, j)
	c.x.Sub(t6, t)

//设置y=—（2h）³（s1+s*（x/4h²-u1））
//这也是
//y=-2·s1·j-（s2-s1）（2x-2i·u1）=r（v-x）-2·s1·j
t.Sub(v, c.x) //T7
t4.Mul(s1, j) //T8
	t4.Mod(t4, P)
t6.Add(t4, t4) //T9
t4.Mul(r, t)   //T10
	t4.Mod(t4, P)
	c.y.Sub(t4, t6)

//设置z=2（u2-u1）·z1·z2=2h·z1·z2
t.Add(a.z, b.z) //T11
t4.Mul(t, t)    //T12
	t4.Mod(t4, P)
t.Sub(t4, z1z1) //T13
t4.Sub(t, z2z2) //T14
	c.z.Mul(t4, h)
	c.z.Mod(c.z, P)

	pool.Put(z1z1)
	pool.Put(z2z2)
	pool.Put(u1)
	pool.Put(u2)
	pool.Put(t)
	pool.Put(s1)
	pool.Put(s2)
	pool.Put(h)
	pool.Put(i)
	pool.Put(j)
	pool.Put(r)
	pool.Put(v)
	pool.Put(t4)
	pool.Put(t6)
}

func (c *curvePoint) Double(a *curvePoint, pool *bnPool) {
//请参阅http://hyper椭圆形.org/efd/g1p/auto-code/shortw/jacobian-0/double/dbl-2009-l.op3
	A := pool.Get().Mul(a.x, a.x)
	A.Mod(A, P)
	B := pool.Get().Mul(a.y, a.y)
	B.Mod(B, P)
	C_ := pool.Get().Mul(B, B)
	C_.Mod(C_, P)

	t := pool.Get().Add(a.x, B)
	t2 := pool.Get().Mul(t, t)
	t2.Mod(t2, P)
	t.Sub(t2, A)
	t2.Sub(t, C_)
	d := pool.Get().Add(t2, t2)
	t.Add(A, A)
	e := pool.Get().Add(t, A)
	f := pool.Get().Mul(e, e)
	f.Mod(f, P)

	t.Add(d, d)
	c.x.Sub(f, t)

	t.Add(C_, C_)
	t2.Add(t, t)
	t.Add(t2, t2)
	c.y.Sub(d, c.x)
	t2.Mul(e, c.y)
	t2.Mod(t2, P)
	c.y.Sub(t2, t)

	t.Mul(a.y, a.z)
	t.Mod(t, P)
	c.z.Add(t, t)

	pool.Put(A)
	pool.Put(B)
	pool.Put(C_)
	pool.Put(t)
	pool.Put(t2)
	pool.Put(d)
	pool.Put(e)
	pool.Put(f)
}

func (c *curvePoint) Mul(a *curvePoint, scalar *big.Int, pool *bnPool) *curvePoint {
	sum := newCurvePoint(pool)
	sum.SetInfinity()
	t := newCurvePoint(pool)

	for i := scalar.BitLen(); i >= 0; i-- {
		t.Double(sum, pool)
		if scalar.Bit(i) != 0 {
			sum.Add(t, a, pool)
		} else {
			sum.Set(t)
		}
	}

	c.Set(sum)
	sum.Put(pool)
	t.Put(pool)
	return c
}

//makeaffine将c转换为affine形式并返回c。如果c是∞，则它设置
//C到0：1：0。
func (c *curvePoint) MakeAffine(pool *bnPool) *curvePoint {
	if words := c.z.Bits(); len(words) == 1 && words[0] == 1 {
		return c
	}
	if c.IsInfinity() {
		c.x.SetInt64(0)
		c.y.SetInt64(1)
		c.z.SetInt64(0)
		c.t.SetInt64(0)
		return c
	}
	zInv := pool.Get().ModInverse(c.z, P)
	t := pool.Get().Mul(c.y, zInv)
	t.Mod(t, P)
	zInv2 := pool.Get().Mul(zInv, zInv)
	zInv2.Mod(zInv2, P)
	c.y.Mul(t, zInv2)
	c.y.Mod(c.y, P)
	t.Mul(c.x, zInv2)
	t.Mod(t, P)
	c.x.Set(t)
	c.z.SetInt64(1)
	c.t.SetInt64(1)

	pool.Put(zInv)
	pool.Put(t)
	pool.Put(zInv2)

	return c
}

func (c *curvePoint) Negative(a *curvePoint) {
	c.x.Set(a.x)
	c.y.Neg(a.y)
	c.z.Set(a.z)
	c.t.SetInt64(0)
}
