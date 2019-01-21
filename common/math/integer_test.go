
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

package math

import (
	"testing"
)

type operation byte

const (
	sub operation = iota
	add
	mul
)

func TestOverflow(t *testing.T) {
	for i, test := range []struct {
		x        uint64
		y        uint64
		overflow bool
		op       operation
	}{
//添加操作
		{MaxUint64, 1, true, add},
		{MaxUint64 - 1, 1, false, add},

//子操作
		{0, 1, true, sub},
		{0, 0, false, sub},

//多重运算
		{0, 0, false, mul},
		{10, 10, false, mul},
		{MaxUint64, 2, true, mul},
		{MaxUint64, 1, false, mul},
	} {
		var overflows bool
		switch test.op {
		case sub:
			_, overflows = SafeSub(test.x, test.y)
		case add:
			_, overflows = SafeAdd(test.x, test.y)
		case mul:
			_, overflows = SafeMul(test.x, test.y)
		}

		if test.overflow != overflows {
			t.Errorf("%d failed. Expected test to be %v, got %v", i, test.overflow, overflows)
		}
	}
}

func TestHexOrDecimal64(t *testing.T) {
	tests := []struct {
		input string
		num   uint64
		ok    bool
	}{
		{"", 0, true},
		{"0", 0, true},
		{"0x0", 0, true},
		{"12345678", 12345678, true},
		{"0x12345678", 0x12345678, true},
		{"0X12345678", 0x12345678, true},
//超前零行为测试：
{"0123456789", 123456789, true}, //注：不是八进制
		{"0x00", 0, true},
		{"0x012345678abc", 0x12345678abc, true},
//无效语法：
		{"abcdef", 0, false},
		{"0xgg", 0, false},
//不适合64位：
		{"18446744073709551617", 0, false},
	}
	for _, test := range tests {
		var num HexOrDecimal64
		err := num.UnmarshalText([]byte(test.input))
		if (err == nil) != test.ok {
			t.Errorf("ParseUint64(%q) -> (err == nil) = %t, want %t", test.input, err == nil, test.ok)
			continue
		}
		if err == nil && uint64(num) != test.num {
			t.Errorf("ParseUint64(%q) -> %d, want %d", test.input, num, test.num)
		}
	}
}

func TestMustParseUint64(t *testing.T) {
	if v := MustParseUint64("12345"); v != 12345 {
		t.Errorf(`MustParseUint64("12345") = %d, want 12345`, v)
	}
}

func TestMustParseUint64Panic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustParseBig should've panicked")
		}
	}()
	MustParseUint64("ggg")
}
