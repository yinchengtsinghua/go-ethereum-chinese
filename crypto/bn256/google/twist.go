
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

//Twistpoint在gf（p²）上实现椭圆曲线y²=x³+3/ξ。点是
//以雅可比形式保存，有效时t=z²。G组是一组
//n——该曲线在gf（p²）上的扭转点（其中n=阶数）
type twistPoint struct {
	x, y, z, t *gfP2
}

var twistB = &gfP2{
	bigFromBase10("266929791119991161246907387137283842545076965332900288569378510910307636690"),
	bigFromBase10("19485874751759354771024239261021720505790618469301721065564631296452457478373"),
}

//TwistGen是G组的发生器。
var twistGen = &twistPoint{
	&gfP2{
		bigFromBase10("11559732032986387107991004021392285783925812861821192530917403151452391805634"),
		bigFromBase10("10857046999023057135944570762232829481370756359578518086990519993285655852781"),
	},
	&gfP2{
		bigFromBase10("4082367875863433681332203403145435568316851327593401208105741076214120093531"),
		bigFromBase10("8495653923123431417604973247489272438418190587263600148770280649306958101930"),
	},
	&gfP2{
		bigFromBase10("0"),
		bigFromBase10("1"),
	},
	&gfP2{
		bigFromBase10("0"),
		bigFromBase10("1"),
	},
}

func newTwistPoint(pool *bnPool) *twistPoint {
	return &twistPoint{
		newGFp2(pool),
		newGFp2(pool),
		newGFp2(pool),
		newGFp2(pool),
	}
}

func (c *twistPoint) String() string {
	return "(" + c.x.String() + ", " + c.y.String() + ", " + c.z.String() + ")"
}

func (c *twistPoint) Put(pool *bnPool) {
	c.x.Put(pool)
	c.y.Put(pool)
	c.z.Put(pool)
	c.t.Put(pool)
}

func (c *twistPoint) Set(a *twistPoint) {
	c.x.Set(a.x)
	c.y.Set(a.y)
	c.z.Set(a.z)
	c.t.Set(a.t)
}

//is on curve返回真正的iff c在曲线上，其中c必须是仿射形式。
func (c *twistPoint) IsOnCurve() bool {
	pool := new(bnPool)
	yy := newGFp2(pool).Square(c.y, pool)
	xxx := newGFp2(pool).Square(c.x, pool)
	xxx.Mul(xxx, c.x, pool)
	yy.Sub(yy, xxx)
	yy.Sub(yy, twistB)
	yy.Minimal()

	if yy.x.Sign() != 0 || yy.y.Sign() != 0 {
		return false
	}
	cneg := newTwistPoint(pool)
	cneg.Mul(c, Order, pool)
	return cneg.z.IsZero()
}

func (c *twistPoint) SetInfinity() {
	c.z.SetZero()
}

func (c *twistPoint) IsInfinity() bool {
	return c.z.IsZero()
}

func (c *twistPoint) Add(a, b *twistPoint, pool *bnPool) {
//有关其他注释，请参见curve.go中的相同函数。

	if a.IsInfinity() {
		c.Set(b)
		return
	}
	if b.IsInfinity() {
		c.Set(a)
		return
	}

//见http://hyper椭圆形.org/efd/g1p/auto-code/shortw/jacobian-0/addition/add-2007-bl.op3
	z1z1 := newGFp2(pool).Square(a.z, pool)
	z2z2 := newGFp2(pool).Square(b.z, pool)
	u1 := newGFp2(pool).Mul(a.x, z2z2, pool)
	u2 := newGFp2(pool).Mul(b.x, z1z1, pool)

	t := newGFp2(pool).Mul(b.z, z2z2, pool)
	s1 := newGFp2(pool).Mul(a.y, t, pool)

	t.Mul(a.z, z1z1, pool)
	s2 := newGFp2(pool).Mul(b.y, t, pool)

	h := newGFp2(pool).Sub(u2, u1)
	xEqual := h.IsZero()

	t.Add(h, h)
	i := newGFp2(pool).Square(t, pool)
	j := newGFp2(pool).Mul(h, i, pool)

	t.Sub(s2, s1)
	yEqual := t.IsZero()
	if xEqual && yEqual {
		c.Double(a, pool)
		return
	}
	r := newGFp2(pool).Add(t, t)

	v := newGFp2(pool).Mul(u1, i, pool)

	t4 := newGFp2(pool).Square(r, pool)
	t.Add(v, v)
	t6 := newGFp2(pool).Sub(t4, j)
	c.x.Sub(t6, t)

t.Sub(v, c.x)       //T7
t4.Mul(s1, j, pool) //T8
t6.Add(t4, t4)      //T9
t4.Mul(r, t, pool)  //T10
	c.y.Sub(t4, t6)

t.Add(a.z, b.z)    //T11
t4.Square(t, pool) //T12
t.Sub(t4, z1z1)    //T13
t4.Sub(t, z2z2)    //T14
	c.z.Mul(t4, h, pool)

	z1z1.Put(pool)
	z2z2.Put(pool)
	u1.Put(pool)
	u2.Put(pool)
	t.Put(pool)
	s1.Put(pool)
	s2.Put(pool)
	h.Put(pool)
	i.Put(pool)
	j.Put(pool)
	r.Put(pool)
	v.Put(pool)
	t4.Put(pool)
	t6.Put(pool)
}

func (c *twistPoint) Double(a *twistPoint, pool *bnPool) {
//请参阅http://hyper椭圆形.org/efd/g1p/auto-code/shortw/jacobian-0/double/dbl-2009-l.op3
	A := newGFp2(pool).Square(a.x, pool)
	B := newGFp2(pool).Square(a.y, pool)
	C_ := newGFp2(pool).Square(B, pool)

	t := newGFp2(pool).Add(a.x, B)
	t2 := newGFp2(pool).Square(t, pool)
	t.Sub(t2, A)
	t2.Sub(t, C_)
	d := newGFp2(pool).Add(t2, t2)
	t.Add(A, A)
	e := newGFp2(pool).Add(t, A)
	f := newGFp2(pool).Square(e, pool)

	t.Add(d, d)
	c.x.Sub(f, t)

	t.Add(C_, C_)
	t2.Add(t, t)
	t.Add(t2, t2)
	c.y.Sub(d, c.x)
	t2.Mul(e, c.y, pool)
	c.y.Sub(t2, t)

	t.Mul(a.y, a.z, pool)
	c.z.Add(t, t)

	A.Put(pool)
	B.Put(pool)
	C_.Put(pool)
	t.Put(pool)
	t2.Put(pool)
	d.Put(pool)
	e.Put(pool)
	f.Put(pool)
}

func (c *twistPoint) Mul(a *twistPoint, scalar *big.Int, pool *bnPool) *twistPoint {
	sum := newTwistPoint(pool)
	sum.SetInfinity()
	t := newTwistPoint(pool)

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
func (c *twistPoint) MakeAffine(pool *bnPool) *twistPoint {
	if c.z.IsOne() {
		return c
	}
	if c.IsInfinity() {
		c.x.SetZero()
		c.y.SetOne()
		c.z.SetZero()
		c.t.SetZero()
		return c
	}
	zInv := newGFp2(pool).Invert(c.z, pool)
	t := newGFp2(pool).Mul(c.y, zInv, pool)
	zInv2 := newGFp2(pool).Square(zInv, pool)
	c.y.Mul(t, zInv2, pool)
	t.Mul(c.x, zInv2, pool)
	c.x.Set(t)
	c.z.SetOne()
	c.t.SetOne()

	zInv.Put(pool)
	t.Put(pool)
	zInv2.Put(pool)

	return c
}

func (c *twistPoint) Negative(a *twistPoint, pool *bnPool) {
	c.x.Set(a.x)
	c.y.SetZero()
	c.y.Sub(c.y, a.y)
	c.z.Set(a.z)
	c.t.SetZero()
}
