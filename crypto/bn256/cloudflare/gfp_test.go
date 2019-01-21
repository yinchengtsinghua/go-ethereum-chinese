
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package bn256

import (
	"testing"
)

//测试否定在装配优化和纯go上的工作方式相同
//实施。
func TestGFpNeg(t *testing.T) {
	n := &gfP{0x0123456789abcdef, 0xfedcba9876543210, 0xdeadbeefdeadbeef, 0xfeebdaedfeebdaed}
	w := &gfP{0xfedcba9876543211, 0x0123456789abcdef, 0x2152411021524110, 0x0114251201142512}
	h := &gfP{}

	gfpNeg(h, n)
	if *h != *w {
		t.Errorf("negation mismatch: have %#x, want %#x", *h, *w)
	}
}

//测试添加在装配优化和纯Go上的工作方式相同
//实施。
func TestGFpAdd(t *testing.T) {
	a := &gfP{0x0123456789abcdef, 0xfedcba9876543210, 0xdeadbeefdeadbeef, 0xfeebdaedfeebdaed}
	b := &gfP{0xfedcba9876543210, 0x0123456789abcdef, 0xfeebdaedfeebdaed, 0xdeadbeefdeadbeef}
	w := &gfP{0xc3df73e9278302b8, 0x687e956e978e3572, 0x254954275c18417f, 0xad354b6afc67f9b4}
	h := &gfP{}

	gfpAdd(h, a, b)
	if *h != *w {
		t.Errorf("addition mismatch: have %#x, want %#x", *h, *w)
	}
}

//减法在程序集优化和纯go上的工作方式相同
//实施。
func TestGFpSub(t *testing.T) {
	a := &gfP{0x0123456789abcdef, 0xfedcba9876543210, 0xdeadbeefdeadbeef, 0xfeebdaedfeebdaed}
	b := &gfP{0xfedcba9876543210, 0x0123456789abcdef, 0xfeebdaedfeebdaed, 0xdeadbeefdeadbeef}
	w := &gfP{0x02468acf13579bdf, 0xfdb97530eca86420, 0xdfc1e401dfc1e402, 0x203e1bfe203e1bfd}
	h := &gfP{}

	gfpSub(h, a, b)
	if *h != *w {
		t.Errorf("subtraction mismatch: have %#x, want %#x", *h, *w)
	}
}

//乘法在程序集优化和纯go上的工作方式相同的测试
//实施。
func TestGFpMul(t *testing.T) {
	a := &gfP{0x0123456789abcdef, 0xfedcba9876543210, 0xdeadbeefdeadbeef, 0xfeebdaedfeebdaed}
	b := &gfP{0xfedcba9876543210, 0x0123456789abcdef, 0xfeebdaedfeebdaed, 0xdeadbeefdeadbeef}
	w := &gfP{0xcbcbd377f7ad22d3, 0x3b89ba5d849379bf, 0x87b61627bd38b6d2, 0xc44052a2a0e654b2}
	h := &gfP{}

	gfpMul(h, a, b)
	if *h != *w {
		t.Errorf("multiplication mismatch: have %#x, want %#x", *h, *w)
	}
}
