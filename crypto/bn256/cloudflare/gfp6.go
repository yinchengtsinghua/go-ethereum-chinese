
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

//gfp6将大小为p_的字段作为gfp2的三次扩展，其中τ3=ξ
//和Zi= i + 3。
type gfP6 struct {
x, y, z gfP2 //值为xτ2+yτ+z
}

func (e *gfP6) String() string {
	return "(" + e.x.String() + ", " + e.y.String() + ", " + e.z.String() + ")"
}

func (e *gfP6) Set(a *gfP6) *gfP6 {
	e.x.Set(&a.x)
	e.y.Set(&a.y)
	e.z.Set(&a.z)
	return e
}

func (e *gfP6) SetZero() *gfP6 {
	e.x.SetZero()
	e.y.SetZero()
	e.z.SetZero()
	return e
}

func (e *gfP6) SetOne() *gfP6 {
	e.x.SetZero()
	e.y.SetZero()
	e.z.SetOne()
	return e
}

func (e *gfP6) IsZero() bool {
	return e.x.IsZero() && e.y.IsZero() && e.z.IsZero()
}

func (e *gfP6) IsOne() bool {
	return e.x.IsZero() && e.y.IsZero() && e.z.IsOne()
}

func (e *gfP6) Neg(a *gfP6) *gfP6 {
	e.x.Neg(&a.x)
	e.y.Neg(&a.y)
	e.z.Neg(&a.z)
	return e
}

func (e *gfP6) Frobenius(a *gfP6) *gfP6 {
	e.x.Conjugate(&a.x)
	e.y.Conjugate(&a.y)
	e.z.Conjugate(&a.z)

	e.x.Mul(&e.x, xiTo2PMinus2Over3)
	e.y.Mul(&e.y, xiToPMinus1Over3)
	return e
}

//Frobeniusp2计算（xτ2+yτ+z）^（p²）=xτ^（2p²）+yτ^（p²）+z
func (e *gfP6) FrobeniusP2(a *gfP6) *gfP6 {
//τ^（2p平方）=ττ^（2p平方-2）=τξ^（（2p平方-2）/3）
	e.x.MulScalar(&a.x, xiTo2PSquaredMinus2Over3)
//τ^（p²）=τ^（p²-1）=τξ^（（p²-1）/3）
	e.y.MulScalar(&a.y, xiToPSquaredMinus1Over3)
	e.z.Set(&a.z)
	return e
}

func (e *gfP6) FrobeniusP4(a *gfP6) *gfP6 {
	e.x.MulScalar(&a.x, xiToPSquaredMinus1Over3)
	e.y.MulScalar(&a.y, xiTo2PSquaredMinus2Over3)
	e.z.Set(&a.z)
	return e
}

func (e *gfP6) Add(a, b *gfP6) *gfP6 {
	e.x.Add(&a.x, &b.x)
	e.y.Add(&a.y, &b.y)
	e.z.Add(&a.z, &b.z)
	return e
}

func (e *gfP6) Sub(a, b *gfP6) *gfP6 {
	e.x.Sub(&a.x, &b.x)
	e.y.Sub(&a.y, &b.y)
	e.z.Sub(&a.z, &b.z)
	return e
}

func (e *gfP6) Mul(a, b *gfP6) *gfP6 {
//“对友好字段的乘法和平方”
//第4节，Karatsuba方法。
//http://eprint.iacr.org/2006/471.pdf
	v0 := (&gfP2{}).Mul(&a.z, &b.z)
	v1 := (&gfP2{}).Mul(&a.y, &b.y)
	v2 := (&gfP2{}).Mul(&a.x, &b.x)

	t0 := (&gfP2{}).Add(&a.x, &a.y)
	t1 := (&gfP2{}).Add(&b.x, &b.y)
	tz := (&gfP2{}).Mul(t0, t1)
	tz.Sub(tz, v1).Sub(tz, v2).MulXi(tz).Add(tz, v0)

	t0.Add(&a.y, &a.z)
	t1.Add(&b.y, &b.z)
	ty := (&gfP2{}).Mul(t0, t1)
	t0.MulXi(v2)
	ty.Sub(ty, v0).Sub(ty, v1).Add(ty, t0)

	t0.Add(&a.x, &a.z)
	t1.Add(&b.x, &b.z)
	tx := (&gfP2{}).Mul(t0, t1)
	tx.Sub(tx, v0).Add(tx, v1).Sub(tx, v2)

	e.x.Set(tx)
	e.y.Set(ty)
	e.z.Set(tz)
	return e
}

func (e *gfP6) MulScalar(a *gfP6, b *gfP2) *gfP6 {
	e.x.Mul(&a.x, b)
	e.y.Mul(&a.y, b)
	e.z.Mul(&a.z, b)
	return e
}

func (e *gfP6) MulGFP(a *gfP6, b *gfP) *gfP6 {
	e.x.MulScalar(&a.x, b)
	e.y.MulScalar(&a.y, b)
	e.z.MulScalar(&a.z, b)
	return e
}

//multau计算τ·（aτ2+bτ+c）=bτ2+cτ+aξ
func (e *gfP6) MulTau(a *gfP6) *gfP6 {
	tz := (&gfP2{}).MulXi(&a.x)
	ty := (&gfP2{}).Set(&a.y)

	e.y.Set(&a.z)
	e.x.Set(ty)
	e.z.Set(tz)
	return e
}

func (e *gfP6) Square(a *gfP6) *gfP6 {
	v0 := (&gfP2{}).Square(&a.z)
	v1 := (&gfP2{}).Square(&a.y)
	v2 := (&gfP2{}).Square(&a.x)

	c0 := (&gfP2{}).Add(&a.x, &a.y)
	c0.Square(c0).Sub(c0, v1).Sub(c0, v2).MulXi(c0).Add(c0, v0)

	c1 := (&gfP2{}).Add(&a.y, &a.z)
	c1.Square(c1).Sub(c1, v0).Sub(c1, v1)
	xiV2 := (&gfP2{}).MulXi(v2)
	c1.Add(c1, xiV2)

	c2 := (&gfP2{}).Add(&a.x, &a.z)
	c2.Square(c2).Sub(c2, v0).Add(c2, v1).Sub(c2, v2)

	e.x.Set(c2)
	e.y.Set(c1)
	e.z.Set(c0)
	return e
}

func (e *gfP6) Invert(a *gfP6) *gfP6 {
//见“实现密码配对”，M.Scott，第3.2节。
//ftp://136.206.11.249/pub/crypto/pairing.pdf文件

//这里我们可以简单解释一下它是如何工作的：让j是
//单位为gf（p平方），因此1+j+j平方=0。
//则（xτ2+yτ+z）（xjτ2+yjτ+z）（xjτ2+yjτ+z）
//=（xτ2+yτ+z）（cτ2+bτ+a）
//=（X酴酴酴+Y酴酴酴酴+Z酴-3酴x y z）=F是基场（范数）的元素。
//
//另一方面（xjτ2+yjτ+z）（xjτ2+yjτ+z）
//=τ2（y 2-ξxz）+τ（ξx 2-y z）+（z 2-ξxy）
//
//所以这就是为什么a=（z-ξxy），b=（ξx-y z），c=（y-ξxz）
	t1 := (&gfP2{}).Mul(&a.x, &a.y)
	t1.MulXi(t1)

	A := (&gfP2{}).Square(&a.z)
	A.Sub(A, t1)

	B := (&gfP2{}).Square(&a.x)
	B.MulXi(B)
	t1.Mul(&a.y, &a.z)
	B.Sub(B, t1)

	C := (&gfP2{}).Square(&a.y)
	t1.Mul(&a.x, &a.z)
	C.Sub(C, t1)

	F := (&gfP2{}).Mul(C, &a.y)
	F.MulXi(F)
	t1.Mul(A, &a.z)
	F.Add(F, t1)
	t1.Mul(B, &a.x).MulXi(t1)
	F.Add(F, t1)

	F.Invert(F)

	e.x.Mul(C, F)
	e.y.Mul(B, F)
	e.z.Mul(A, F)
	return e
}
