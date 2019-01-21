
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2014 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

package rlp

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func TestStreamKind(t *testing.T) {
	tests := []struct {
		input    string
		wantKind Kind
		wantLen  uint64
	}{
		{"00", Byte, 0},
		{"01", Byte, 0},
		{"7F", Byte, 0},
		{"80", String, 0},
		{"B7", String, 55},
		{"B90400", String, 1024},
		{"BFFFFFFFFFFFFFFFFF", String, ^uint64(0)},
		{"C0", List, 0},
		{"C8", List, 8},
		{"F7", List, 55},
		{"F90400", List, 1024},
		{"FFFFFFFFFFFFFFFFFF", List, ^uint64(0)},
	}

	for i, test := range tests {
//使用普通读卡器抑制输入极限误差。
		s := NewStream(newPlainReader(unhex(test.input)), 0)
		kind, len, err := s.Kind()
		if err != nil {
			t.Errorf("test %d: Kind returned error: %v", i, err)
			continue
		}
		if kind != test.wantKind {
			t.Errorf("test %d: kind mismatch: got %d, want %d", i, kind, test.wantKind)
		}
		if len != test.wantLen {
			t.Errorf("test %d: len mismatch: got %d, want %d", i, len, test.wantLen)
		}
	}
}

func TestNewListStream(t *testing.T) {
	ls := NewListStream(bytes.NewReader(unhex("0101010101")), 3)
	if k, size, err := ls.Kind(); k != List || size != 3 || err != nil {
		t.Errorf("Kind() returned (%v, %d, %v), expected (List, 3, nil)", k, size, err)
	}
	if size, err := ls.List(); size != 3 || err != nil {
		t.Errorf("List() returned (%d, %v), expected (3, nil)", size, err)
	}
	for i := 0; i < 3; i++ {
		if val, err := ls.Uint(); val != 1 || err != nil {
			t.Errorf("Uint() returned (%d, %v), expected (1, nil)", val, err)
		}
	}
	if err := ls.ListEnd(); err != nil {
		t.Errorf("ListEnd() returned %v, expected (3, nil)", err)
	}
}

func TestStreamErrors(t *testing.T) {
	withoutInputLimit := func(b []byte) *Stream {
		return NewStream(newPlainReader(b), 0)
	}
	withCustomInputLimit := func(limit uint64) func([]byte) *Stream {
		return func(b []byte) *Stream {
			return NewStream(bytes.NewReader(b), limit)
		}
	}

	type calls []string
	tests := []struct {
		string
		calls
newStream func([]byte) *Stream //使用bytes.reader（如果为nil）
		error     error
	}{
		{"C0", calls{"Bytes"}, nil, ErrExpectedString},
		{"C0", calls{"Uint"}, nil, ErrExpectedString},
		{"89000000000000000001", calls{"Uint"}, nil, errUintOverflow},
		{"00", calls{"List"}, nil, ErrExpectedList},
		{"80", calls{"List"}, nil, ErrExpectedList},
		{"C0", calls{"List", "Uint"}, nil, EOL},
		{"C8C9010101010101010101", calls{"List", "Kind"}, nil, ErrElemTooLarge},
		{"C3C2010201", calls{"List", "List", "Uint", "Uint", "ListEnd", "Uint"}, nil, EOL},
		{"00", calls{"ListEnd"}, nil, errNotInList},
		{"C401020304", calls{"List", "Uint", "ListEnd"}, nil, errNotAtEOL},

//非规范整数（例如前导零字节）。
		{"00", calls{"Uint"}, nil, ErrCanonInt},
		{"820002", calls{"Uint"}, nil, ErrCanonInt},
		{"8133", calls{"Uint"}, nil, ErrCanonSize},
		{"817F", calls{"Uint"}, nil, ErrCanonSize},
		{"8180", calls{"Uint"}, nil, nil},

//无效布尔值
		{"02", calls{"Bool"}, nil, errors.New("rlp: invalid boolean value: 2")},

//大小标记必须使用尽可能小的编码。
//大小标记中的前导零字节也被拒绝。
		{"8100", calls{"Uint"}, nil, ErrCanonSize},
		{"8100", calls{"Bytes"}, nil, ErrCanonSize},
		{"8101", calls{"Bytes"}, nil, ErrCanonSize},
		{"817F", calls{"Bytes"}, nil, ErrCanonSize},
		{"8180", calls{"Bytes"}, nil, nil},
		{"B800", calls{"Kind"}, withoutInputLimit, ErrCanonSize},
		{"B90000", calls{"Kind"}, withoutInputLimit, ErrCanonSize},
		{"B90055", calls{"Kind"}, withoutInputLimit, ErrCanonSize},
		{"BA0002FFFF", calls{"Bytes"}, withoutInputLimit, ErrCanonSize},
		{"F800", calls{"Kind"}, withoutInputLimit, ErrCanonSize},
		{"F90000", calls{"Kind"}, withoutInputLimit, ErrCanonSize},
		{"F90055", calls{"Kind"}, withoutInputLimit, ErrCanonSize},
		{"FA0002FFFF", calls{"List"}, withoutInputLimit, ErrCanonSize},

//预期EOF
		{"", calls{"Kind"}, nil, io.EOF},
		{"", calls{"Uint"}, nil, io.EOF},
		{"", calls{"List"}, nil, io.EOF},
		{"8180", calls{"Uint", "Uint"}, nil, io.EOF},
		{"C0", calls{"List", "ListEnd", "List"}, nil, io.EOF},

		{"", calls{"List"}, withoutInputLimit, io.EOF},
		{"8180", calls{"Uint", "Uint"}, withoutInputLimit, io.EOF},
		{"C0", calls{"List", "ListEnd", "List"}, withoutInputLimit, io.EOF},

//输入限制错误。
		{"81", calls{"Bytes"}, nil, ErrValueTooLarge},
		{"81", calls{"Uint"}, nil, ErrValueTooLarge},
		{"81", calls{"Raw"}, nil, ErrValueTooLarge},
		{"BFFFFFFFFFFFFFFFFFFF", calls{"Bytes"}, nil, ErrValueTooLarge},
		{"C801", calls{"List"}, nil, ErrValueTooLarge},

//测试列表元素大小检查溢出。
		{"CD04040404FFFFFFFFFFFFFFFFFF0303", calls{"List", "Uint", "Uint", "Uint", "Uint", "List"}, nil, ErrElemTooLarge},

//输入极限溢出测试。因为我们在计算限额
//在stream.remaining中向下接近零，如果读数太远，则可能溢出
//保留为大值，有效禁用限制。
		{"C40102030401", calls{"Raw", "Uint"}, withCustomInputLimit(5), io.EOF},
		{"C4010203048180", calls{"Raw", "Uint"}, withCustomInputLimit(6), ErrValueTooLarge},

//检查相同的呼叫是否可以无限制地进行。
		{"C40102030401", calls{"Raw", "Uint"}, withoutInputLimit, nil},
		{"C4010203048180", calls{"Raw", "Uint"}, withoutInputLimit, nil},

//意外的EOF。只有当
//没有输入限制，所以读卡器需要“减速”。
		{"81", calls{"Bytes"}, withoutInputLimit, io.ErrUnexpectedEOF},
		{"81", calls{"Uint"}, withoutInputLimit, io.ErrUnexpectedEOF},
		{"BFFFFFFFFFFFFFFF", calls{"Bytes"}, withoutInputLimit, io.ErrUnexpectedEOF},
		{"C801", calls{"List", "Uint", "Uint"}, withoutInputLimit, io.ErrUnexpectedEOF},

//此测试验证输入位置是否提前。
//为空字符串调用字节时正确。可以称之为
//和之间的任何次数都不会提前。
		{"C3808080", calls{
"List",  //进入列表
"Bytes", //超过第一个元素

"Kind", "Kind", "Kind", //这不应该提前

"Bytes", //过去的第二个元素

"Kind", "Kind", //尝试是不会受伤的

"Bytes", //过去的最后一个元素
"Bytes", //这个应该失败
		}, nil, EOL},
	}

testfor:
	for i, test := range tests {
		if test.newStream == nil {
			test.newStream = func(b []byte) *Stream { return NewStream(bytes.NewReader(b), 0) }
		}
		s := test.newStream(unhex(test.string))
		rs := reflect.ValueOf(s)
		for j, call := range test.calls {
			fval := rs.MethodByName(call)
			ret := fval.Call(nil)
			err := "<nil>"
			if lastret := ret[len(ret)-1].Interface(); lastret != nil {
				err = lastret.(error).Error()
			}
			if j == len(test.calls)-1 {
				want := "<nil>"
				if test.error != nil {
					want = test.error.Error()
				}
				if err != want {
					t.Log(test)
					t.Errorf("test %d: last call (%s) error mismatch\ngot:  %s\nwant: %s",
						i, call, err, test.error)
				}
			} else if err != "<nil>" {
				t.Log(test)
				t.Errorf("test %d: call %d (%s) unexpected error: %q", i, j, call, err)
				continue testfor
			}
		}
	}
}

func TestStreamList(t *testing.T) {
	s := NewStream(bytes.NewReader(unhex("C80102030405060708")), 0)

	len, err := s.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len != 8 {
		t.Fatalf("List returned invalid length, got %d, want 8", len)
	}

	for i := uint64(1); i <= 8; i++ {
		v, err := s.Uint()
		if err != nil {
			t.Fatalf("Uint error: %v", err)
		}
		if i != v {
			t.Errorf("Uint returned wrong value, got %d, want %d", v, i)
		}
	}

	if _, err := s.Uint(); err != EOL {
		t.Errorf("Uint error mismatch, got %v, want %v", err, EOL)
	}
	if err = s.ListEnd(); err != nil {
		t.Fatalf("ListEnd error: %v", err)
	}
}

func TestStreamRaw(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{
			"C58401010101",
			"8401010101",
		},
		{
			"F842B84001010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101",
			"B84001010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101",
		},
	}
	for i, tt := range tests {
		s := NewStream(bytes.NewReader(unhex(tt.input)), 0)
		s.List()

		want := unhex(tt.output)
		raw, err := s.Raw()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(want, raw) {
			t.Errorf("test %d: raw mismatch: got %x, want %x", i, raw, want)
		}
	}
}

func TestDecodeErrors(t *testing.T) {
	r := bytes.NewReader(nil)

	if err := Decode(r, nil); err != errDecodeIntoNil {
		t.Errorf("Decode(r, nil) error mismatch, got %q, want %q", err, errDecodeIntoNil)
	}

	var nilptr *struct{}
	if err := Decode(r, nilptr); err != errDecodeIntoNil {
		t.Errorf("Decode(r, nilptr) error mismatch, got %q, want %q", err, errDecodeIntoNil)
	}

	if err := Decode(r, struct{}{}); err != errNoPointer {
		t.Errorf("Decode(r, struct{}{}) error mismatch, got %q, want %q", err, errNoPointer)
	}

	expectErr := "rlp: type chan bool is not RLP-serializable"
	if err := Decode(r, new(chan bool)); err == nil || err.Error() != expectErr {
		t.Errorf("Decode(r, new(chan bool)) error mismatch, got %q, want %q", err, expectErr)
	}

	if err := Decode(r, new(uint)); err != io.EOF {
		t.Errorf("Decode(r, new(int)) error mismatch, got %q, want %q", err, io.EOF)
	}
}

type decodeTest struct {
	input string
	ptr   interface{}
	value interface{}
	error string
}

type simplestruct struct {
	A uint
	B string
}

type recstruct struct {
	I     uint
	Child *recstruct `rlp:"nil"`
}

type invalidTail1 struct {
	A uint `rlp:"tail"`
	B string
}

type invalidTail2 struct {
	A uint
	B string `rlp:"tail"`
}

type tailRaw struct {
	A    uint
	Tail []RawValue `rlp:"tail"`
}

type tailUint struct {
	A    uint
	Tail []uint `rlp:"tail"`
}

var (
	veryBigInt = big.NewInt(0).Add(
		big.NewInt(0).Lsh(big.NewInt(0xFFFFFFFFFFFFFF), 16),
		big.NewInt(0xFFFF),
	)
)

type hasIgnoredField struct {
	A uint
	B uint `rlp:"-"`
	C uint
}

var decodeTests = []decodeTest{
//布尔运算
	{input: "01", ptr: new(bool), value: true},
	{input: "80", ptr: new(bool), value: false},
	{input: "02", ptr: new(bool), error: "rlp: invalid boolean value: 2"},

//整数
	{input: "05", ptr: new(uint32), value: uint32(5)},
	{input: "80", ptr: new(uint32), value: uint32(0)},
	{input: "820505", ptr: new(uint32), value: uint32(0x0505)},
	{input: "83050505", ptr: new(uint32), value: uint32(0x050505)},
	{input: "8405050505", ptr: new(uint32), value: uint32(0x05050505)},
	{input: "850505050505", ptr: new(uint32), error: "rlp: input string too long for uint32"},
	{input: "C0", ptr: new(uint32), error: "rlp: expected input string or byte for uint32"},
	{input: "00", ptr: new(uint32), error: "rlp: non-canonical integer (leading zero bytes) for uint32"},
	{input: "8105", ptr: new(uint32), error: "rlp: non-canonical size information for uint32"},
	{input: "820004", ptr: new(uint32), error: "rlp: non-canonical integer (leading zero bytes) for uint32"},
	{input: "B8020004", ptr: new(uint32), error: "rlp: non-canonical size information for uint32"},

//片
	{input: "C0", ptr: new([]uint), value: []uint{}},
	{input: "C80102030405060708", ptr: new([]uint), value: []uint{1, 2, 3, 4, 5, 6, 7, 8}},
	{input: "F8020004", ptr: new([]uint), error: "rlp: non-canonical size information for []uint"},

//数组
	{input: "C50102030405", ptr: new([5]uint), value: [5]uint{1, 2, 3, 4, 5}},
	{input: "C0", ptr: new([5]uint), error: "rlp: input list has too few elements for [5]uint"},
	{input: "C102", ptr: new([5]uint), error: "rlp: input list has too few elements for [5]uint"},
	{input: "C6010203040506", ptr: new([5]uint), error: "rlp: input list has too many elements for [5]uint"},
	{input: "F8020004", ptr: new([5]uint), error: "rlp: non-canonical size information for [5]uint"},

//零大小数组
	{input: "C0", ptr: new([0]uint), value: [0]uint{}},
	{input: "C101", ptr: new([0]uint), error: "rlp: input list has too many elements for [0]uint"},

//字节切片
	{input: "01", ptr: new([]byte), value: []byte{1}},
	{input: "80", ptr: new([]byte), value: []byte{}},
	{input: "8D6162636465666768696A6B6C6D", ptr: new([]byte), value: []byte("abcdefghijklm")},
	{input: "C0", ptr: new([]byte), error: "rlp: expected input string or byte for []uint8"},
	{input: "8105", ptr: new([]byte), error: "rlp: non-canonical size information for []uint8"},

//字节数组
	{input: "02", ptr: new([1]byte), value: [1]byte{2}},
	{input: "8180", ptr: new([1]byte), value: [1]byte{128}},
	{input: "850102030405", ptr: new([5]byte), value: [5]byte{1, 2, 3, 4, 5}},

//字节数组错误
	{input: "02", ptr: new([5]byte), error: "rlp: input string too short for [5]uint8"},
	{input: "80", ptr: new([5]byte), error: "rlp: input string too short for [5]uint8"},
	{input: "820000", ptr: new([5]byte), error: "rlp: input string too short for [5]uint8"},
	{input: "C0", ptr: new([5]byte), error: "rlp: expected input string or byte for [5]uint8"},
	{input: "C3010203", ptr: new([5]byte), error: "rlp: expected input string or byte for [5]uint8"},
	{input: "86010203040506", ptr: new([5]byte), error: "rlp: input string too long for [5]uint8"},
	{input: "8105", ptr: new([1]byte), error: "rlp: non-canonical size information for [1]uint8"},
	{input: "817F", ptr: new([1]byte), error: "rlp: non-canonical size information for [1]uint8"},

//零大小字节数组
	{input: "80", ptr: new([0]byte), value: [0]byte{}},
	{input: "01", ptr: new([0]byte), error: "rlp: input string too long for [0]uint8"},
	{input: "8101", ptr: new([0]byte), error: "rlp: input string too long for [0]uint8"},

//串
	{input: "00", ptr: new(string), value: "\000"},
	{input: "8D6162636465666768696A6B6C6D", ptr: new(string), value: "abcdefghijklm"},
	{input: "C0", ptr: new(string), error: "rlp: expected input string or byte for string"},

//大企业
	{input: "01", ptr: new(*big.Int), value: big.NewInt(1)},
	{input: "89FFFFFFFFFFFFFFFFFF", ptr: new(*big.Int), value: veryBigInt},
{input: "10", ptr: new(big.Int), value: *big.NewInt(16)}, //非指针也有效
	{input: "C0", ptr: new(*big.Int), error: "rlp: expected input string or byte for *big.Int"},
	{input: "820001", ptr: new(big.Int), error: "rlp: non-canonical integer (leading zero bytes) for *big.Int"},
	{input: "8105", ptr: new(big.Int), error: "rlp: non-canonical size information for *big.Int"},

//结构体
	{
		input: "C50583343434",
		ptr:   new(simplestruct),
		value: simplestruct{5, "444"},
	},
	{
		input: "C601C402C203C0",
		ptr:   new(recstruct),
		value: recstruct{1, &recstruct{2, &recstruct{3, nil}}},
	},

//结构错误
	{
		input: "C0",
		ptr:   new(simplestruct),
		error: "rlp: too few elements for rlp.simplestruct",
	},
	{
		input: "C105",
		ptr:   new(simplestruct),
		error: "rlp: too few elements for rlp.simplestruct",
	},
	{
		input: "C7C50583343434C0",
		ptr:   new([]*simplestruct),
		error: "rlp: too few elements for rlp.simplestruct, decoding into ([]*rlp.simplestruct)[1]",
	},
	{
		input: "83222222",
		ptr:   new(simplestruct),
		error: "rlp: expected input list for rlp.simplestruct",
	},
	{
		input: "C3010101",
		ptr:   new(simplestruct),
		error: "rlp: input list has too many elements for rlp.simplestruct",
	},
	{
		input: "C501C3C00000",
		ptr:   new(recstruct),
		error: "rlp: expected input string or byte for uint, decoding into (rlp.recstruct).Child.I",
	},
	{
		input: "C0",
		ptr:   new(invalidTail1),
		error: "rlp: invalid struct tag \"tail\" for rlp.invalidTail1.A (must be on last field)",
	},
	{
		input: "C0",
		ptr:   new(invalidTail2),
		error: "rlp: invalid struct tag \"tail\" for rlp.invalidTail2.B (field type is not slice)",
	},
	{
		input: "C50102C20102",
		ptr:   new(tailUint),
		error: "rlp: expected input string or byte for uint, decoding into (rlp.tailUint).Tail[1]",
	},

//结构标记“tail”
	{
		input: "C3010203",
		ptr:   new(tailRaw),
		value: tailRaw{A: 1, Tail: []RawValue{unhex("02"), unhex("03")}},
	},
	{
		input: "C20102",
		ptr:   new(tailRaw),
		value: tailRaw{A: 1, Tail: []RawValue{unhex("02")}},
	},
	{
		input: "C101",
		ptr:   new(tailRaw),
		value: tailRaw{A: 1, Tail: []RawValue{}},
	},

//“标签”结构
	{
		input: "C20102",
		ptr:   new(hasIgnoredField),
		value: hasIgnoredField{A: 1, C: 2},
	},

//RAW值
	{input: "01", ptr: new(RawValue), value: RawValue(unhex("01"))},
	{input: "82FFFF", ptr: new(RawValue), value: RawValue(unhex("82FFFF"))},
	{input: "C20102", ptr: new([]RawValue), value: []RawValue{unhex("01"), unhex("02")}},

//指针
	{input: "00", ptr: new(*[]byte), value: &[]byte{0}},
	{input: "80", ptr: new(*uint), value: uintp(0)},
	{input: "C0", ptr: new(*uint), error: "rlp: expected input string or byte for uint"},
	{input: "07", ptr: new(*uint), value: uintp(7)},
	{input: "817F", ptr: new(*uint), error: "rlp: non-canonical size information for uint"},
	{input: "8180", ptr: new(*uint), value: uintp(0x80)},
	{input: "C109", ptr: new(*[]uint), value: &[]uint{9}},
	{input: "C58403030303", ptr: new(*[][]byte), value: &[][]byte{{3, 3, 3, 3}}},

//检查输入位置是否也为空值。
	{input: "C3808005", ptr: new([]*uint), value: []*uint{uintp(0), uintp(0), uintp(5)}},

//接口{}
	{input: "00", ptr: new(interface{}), value: []byte{0}},
	{input: "01", ptr: new(interface{}), value: []byte{1}},
	{input: "80", ptr: new(interface{}), value: []byte{}},
	{input: "850505050505", ptr: new(interface{}), value: []byte{5, 5, 5, 5, 5}},
	{input: "C0", ptr: new(interface{}), value: []interface{}{}},
	{input: "C50183040404", ptr: new(interface{}), value: []interface{}{[]byte{1}, []byte{4, 4, 4}}},
	{
		input: "C3010203",
		ptr:   new([]io.Reader),
		error: "rlp: type io.Reader is not RLP-serializable",
	},

//模糊崩溃
	{
		input: "c330f9c030f93030ce3030303030303030bd303030303030",
		ptr:   new(interface{}),
		error: "rlp: element is larger than containing list",
	},
}

func uintp(i uint) *uint { return &i }

func runTests(t *testing.T, decode func([]byte, interface{}) error) {
	for i, test := range decodeTests {
		input, err := hex.DecodeString(test.input)
		if err != nil {
			t.Errorf("test %d: invalid hex input %q", i, test.input)
			continue
		}
		err = decode(input, test.ptr)
		if err != nil && test.error == "" {
			t.Errorf("test %d: unexpected Decode error: %v\ndecoding into %T\ninput %q",
				i, err, test.ptr, test.input)
			continue
		}
		if test.error != "" && fmt.Sprint(err) != test.error {
			t.Errorf("test %d: Decode error mismatch\ngot  %v\nwant %v\ndecoding into %T\ninput %q",
				i, err, test.error, test.ptr, test.input)
			continue
		}
		deref := reflect.ValueOf(test.ptr).Elem().Interface()
		if err == nil && !reflect.DeepEqual(deref, test.value) {
			t.Errorf("test %d: value mismatch\ngot  %#v\nwant %#v\ndecoding into %T\ninput %q",
				i, deref, test.value, test.ptr, test.input)
		}
	}
}

func TestDecodeWithByteReader(t *testing.T) {
	runTests(t, func(input []byte, into interface{}) error {
		return Decode(bytes.NewReader(input), into)
	})
}

//PlainReader从字节片读取，但不读取
//实现readbyte。它也不被
//尺寸验证。这对于测试解码器
//在非缓冲输入流上运行。
type plainReader []byte

func newPlainReader(b []byte) io.Reader {
	return (*plainReader)(&b)
}

func (r *plainReader) Read(buf []byte) (n int, err error) {
	if len(*r) == 0 {
		return 0, io.EOF
	}
	n = copy(buf, *r)
	*r = (*r)[n:]
	return n, nil
}

func TestDecodeWithNonByteReader(t *testing.T) {
	runTests(t, func(input []byte, into interface{}) error {
		return Decode(newPlainReader(input), into)
	})
}

func TestDecodeStreamReset(t *testing.T) {
	s := NewStream(nil, 0)
	runTests(t, func(input []byte, into interface{}) error {
		s.Reset(bytes.NewReader(input), 0)
		return s.Decode(into)
	})
}

type testDecoder struct{ called bool }

func (t *testDecoder) DecodeRLP(s *Stream) error {
	if _, err := s.Uint(); err != nil {
		return err
	}
	t.called = true
	return nil
}

func TestDecodeDecoder(t *testing.T) {
	var s struct {
		T1 testDecoder
		T2 *testDecoder
		T3 **testDecoder
	}
	if err := Decode(bytes.NewReader(unhex("C3010203")), &s); err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if !s.T1.called {
		t.Errorf("DecodeRLP was not called for (non-pointer) testDecoder")
	}

	if s.T2 == nil {
		t.Errorf("*testDecoder has not been allocated")
	} else if !s.T2.called {
		t.Errorf("DecodeRLP was not called for *testDecoder")
	}

	if s.T3 == nil || *s.T3 == nil {
		t.Errorf("**testDecoder has not been allocated")
	} else if !(*s.T3).called {
		t.Errorf("DecodeRLP was not called for **testDecoder")
	}
}

type byteDecoder byte

func (bd *byteDecoder) DecodeRLP(s *Stream) error {
	_, err := s.Uint()
	*bd = 255
	return err
}

func (bd byteDecoder) called() bool {
	return bd == 255
}

//此测试验证字节片/字节数组逻辑
//不适用于实现解码器的元素类型。
func TestDecoderInByteSlice(t *testing.T) {
	var slice []byteDecoder
	if err := Decode(bytes.NewReader(unhex("C101")), &slice); err != nil {
		t.Errorf("unexpected Decode error %v", err)
	} else if !slice[0].called() {
		t.Errorf("DecodeRLP not called for slice element")
	}

	var array [1]byteDecoder
	if err := Decode(bytes.NewReader(unhex("C101")), &array); err != nil {
		t.Errorf("unexpected Decode error %v", err)
	} else if !array[0].called() {
		t.Errorf("DecodeRLP not called for array element")
	}
}

func ExampleDecode() {
	input, _ := hex.DecodeString("C90A1486666F6F626172")

	type example struct {
		A, B    uint
private uint //忽略私有字段
		String  string
	}

	var s example
	err := Decode(bytes.NewReader(input), &s)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Decoded value: %#v\n", s)
	}
//输出：
//解码值：rlp。示例a:0xa，b:0x14，private:0x0，string:“foobar”
}

func ExampleDecode_structTagNil() {
//在本例中，我们将使用“nil”结构标记来更改
//如何解码指针类型字段。输入包含一个RLP
//一个元素的列表，空字符串。
	input := []byte{0xC1, 0x80}

//此类型使用常规规则。
//空输入字符串被解码为指向空go字符串的指针。
	var normalRules struct {
		String *string
	}
	Decode(bytes.NewReader(input), &normalRules)
	fmt.Printf("normal: String = %q\n", *normalRules.String)

//此类型使用struct标记。
//空输入字符串被解码为零指针。
	var withEmptyOK struct {
		String *string `rlp:"nil"`
	}
	Decode(bytes.NewReader(input), &withEmptyOK)
	fmt.Printf("with nil tag: String = %v\n", withEmptyOK.String)

//输出：
//正常：字符串=“”
//带nil标记：string=<nil>
}

func ExampleStream() {
	input, _ := hex.DecodeString("C90A1486666F6F626172")
	s := NewStream(bytes.NewReader(input), 0)

//检查前面有什么价值
	kind, size, _ := s.Kind()
	fmt.Printf("Kind: %v size:%d\n", kind, size)

//进入列表
	if _, err := s.List(); err != nil {
		fmt.Printf("List error: %v\n", err)
		return
	}

//解码元件
	fmt.Println(s.Uint())
	fmt.Println(s.Uint())
	fmt.Println(s.Bytes())

//确认列表结尾
	if err := s.ListEnd(); err != nil {
		fmt.Printf("ListEnd error: %v\n", err)
	}
//输出：
//种类：列表大小：9
//10 <NIL >
//20 <NIL >
//[102 111 111 98 97 114]<nil>
}

func BenchmarkDecode(b *testing.B) {
	enc := encodeTestSlice(90000)
	b.SetBytes(int64(len(enc)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var s []uint
		r := bytes.NewReader(enc)
		if err := Decode(r, &s); err != nil {
			b.Fatalf("Decode error: %v", err)
		}
	}
}

func BenchmarkDecodeIntSliceReuse(b *testing.B) {
	enc := encodeTestSlice(100000)
	b.SetBytes(int64(len(enc)))
	b.ReportAllocs()
	b.ResetTimer()

	var s []uint
	for i := 0; i < b.N; i++ {
		r := bytes.NewReader(enc)
		if err := Decode(r, &s); err != nil {
			b.Fatalf("Decode error: %v", err)
		}
	}
}

func encodeTestSlice(n uint) []byte {
	s := make([]uint, n)
	for i := uint(0); i < n; i++ {
		s[i] = i
	}
	b, err := EncodeToBytes(s)
	if err != nil {
		panic(fmt.Sprintf("encode error: %v", err))
	}
	return b
}

func unhex(str string) []byte {
	b, err := hex.DecodeString(strings.Replace(str, " ", "", -1))
	if err != nil {
		panic(fmt.Sprintf("invalid hex string: %q", str))
	}
	return b
}
