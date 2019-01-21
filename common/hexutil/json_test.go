
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

package hexutil

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"testing"
)

func checkError(t *testing.T, input string, got, want error) bool {
	if got == nil {
		if want != nil {
			t.Errorf("input %s: got no error, want %q", input, want)
			return false
		}
		return true
	}
	if want == nil {
		t.Errorf("input %s: unexpected error %q", input, got)
	} else if got.Error() != want.Error() {
		t.Errorf("input %s: got error %q, want %q", input, got, want)
	}
	return false
}

func referenceBig(s string) *big.Int {
	b, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic("invalid")
	}
	return b
}

func referenceBytes(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

var errJSONEOF = errors.New("unexpected end of JSON input")

var unmarshalBytesTests = []unmarshalTest{
//无效的编码
	{input: "", wantErr: errJSONEOF},
	{input: "null", wantErr: errNonString(bytesT)},
	{input: "10", wantErr: errNonString(bytesT)},
	{input: `"0"`, wantErr: wrapTypeError(ErrMissingPrefix, bytesT)},
	{input: `"0x0"`, wantErr: wrapTypeError(ErrOddLength, bytesT)},
	{input: `"0xxx"`, wantErr: wrapTypeError(ErrSyntax, bytesT)},
	{input: `"0x01zz01"`, wantErr: wrapTypeError(ErrSyntax, bytesT)},

//有效编码
	{input: `""`, want: referenceBytes("")},
	{input: `"0x"`, want: referenceBytes("")},
	{input: `"0x02"`, want: referenceBytes("02")},
	{input: `"0X02"`, want: referenceBytes("02")},
	{input: `"0xffffffffff"`, want: referenceBytes("ffffffffff")},
	{
		input: `"0xffffffffffffffffffffffffffffffffffff"`,
		want:  referenceBytes("ffffffffffffffffffffffffffffffffffff"),
	},
}

func TestUnmarshalBytes(t *testing.T) {
	for _, test := range unmarshalBytesTests {
		var v Bytes
		err := json.Unmarshal([]byte(test.input), &v)
		if !checkError(t, test.input, err, test.wantErr) {
			continue
		}
		if !bytes.Equal(test.want.([]byte), []byte(v)) {
			t.Errorf("input %s: value mismatch: got %x, want %x", test.input, &v, test.want)
			continue
		}
	}
}

func BenchmarkUnmarshalBytes(b *testing.B) {
	input := []byte(`"0x123456789abcdef123456789abcdef"`)
	for i := 0; i < b.N; i++ {
		var v Bytes
		if err := v.UnmarshalJSON(input); err != nil {
			b.Fatal(err)
		}
	}
}

func TestMarshalBytes(t *testing.T) {
	for _, test := range encodeBytesTests {
		in := test.input.([]byte)
		out, err := json.Marshal(Bytes(in))
		if err != nil {
			t.Errorf("%x: %v", in, err)
			continue
		}
		if want := `"` + test.want + `"`; string(out) != want {
			t.Errorf("%x: MarshalJSON output mismatch: got %q, want %q", in, out, want)
			continue
		}
		if out := Bytes(in).String(); out != test.want {
			t.Errorf("%x: String mismatch: got %q, want %q", in, out, test.want)
			continue
		}
	}
}

var unmarshalBigTests = []unmarshalTest{
//无效的编码
	{input: "", wantErr: errJSONEOF},
	{input: "null", wantErr: errNonString(bigT)},
	{input: "10", wantErr: errNonString(bigT)},
	{input: `"0"`, wantErr: wrapTypeError(ErrMissingPrefix, bigT)},
	{input: `"0x"`, wantErr: wrapTypeError(ErrEmptyNumber, bigT)},
	{input: `"0x01"`, wantErr: wrapTypeError(ErrLeadingZero, bigT)},
	{input: `"0xx"`, wantErr: wrapTypeError(ErrSyntax, bigT)},
	{input: `"0x1zz01"`, wantErr: wrapTypeError(ErrSyntax, bigT)},
	{
		input:   `"0x10000000000000000000000000000000000000000000000000000000000000000"`,
		wantErr: wrapTypeError(ErrBig256Range, bigT),
	},

//有效编码
	{input: `""`, want: big.NewInt(0)},
	{input: `"0x0"`, want: big.NewInt(0)},
	{input: `"0x2"`, want: big.NewInt(0x2)},
	{input: `"0x2F2"`, want: big.NewInt(0x2f2)},
	{input: `"0X2F2"`, want: big.NewInt(0x2f2)},
	{input: `"0x1122aaff"`, want: big.NewInt(0x1122aaff)},
	{input: `"0xbBb"`, want: big.NewInt(0xbbb)},
	{input: `"0xfffffffff"`, want: big.NewInt(0xfffffffff)},
	{
		input: `"0x112233445566778899aabbccddeeff"`,
		want:  referenceBig("112233445566778899aabbccddeeff"),
	},
	{
		input: `"0xffffffffffffffffffffffffffffffffffff"`,
		want:  referenceBig("ffffffffffffffffffffffffffffffffffff"),
	},
	{
		input: `"0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"`,
		want:  referenceBig("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
	},
}

func TestUnmarshalBig(t *testing.T) {
	for _, test := range unmarshalBigTests {
		var v Big
		err := json.Unmarshal([]byte(test.input), &v)
		if !checkError(t, test.input, err, test.wantErr) {
			continue
		}
		if test.want != nil && test.want.(*big.Int).Cmp((*big.Int)(&v)) != 0 {
			t.Errorf("input %s: value mismatch: got %x, want %x", test.input, (*big.Int)(&v), test.want)
			continue
		}
	}
}

func BenchmarkUnmarshalBig(b *testing.B) {
	input := []byte(`"0x123456789abcdef123456789abcdef"`)
	for i := 0; i < b.N; i++ {
		var v Big
		if err := v.UnmarshalJSON(input); err != nil {
			b.Fatal(err)
		}
	}
}

func TestMarshalBig(t *testing.T) {
	for _, test := range encodeBigTests {
		in := test.input.(*big.Int)
		out, err := json.Marshal((*Big)(in))
		if err != nil {
			t.Errorf("%d: %v", in, err)
			continue
		}
		if want := `"` + test.want + `"`; string(out) != want {
			t.Errorf("%d: MarshalJSON output mismatch: got %q, want %q", in, out, want)
			continue
		}
		if out := (*Big)(in).String(); out != test.want {
			t.Errorf("%x: String mismatch: got %q, want %q", in, out, test.want)
			continue
		}
	}
}

var unmarshalUint64Tests = []unmarshalTest{
//无效的编码
	{input: "", wantErr: errJSONEOF},
	{input: "null", wantErr: errNonString(uint64T)},
	{input: "10", wantErr: errNonString(uint64T)},
	{input: `"0"`, wantErr: wrapTypeError(ErrMissingPrefix, uint64T)},
	{input: `"0x"`, wantErr: wrapTypeError(ErrEmptyNumber, uint64T)},
	{input: `"0x01"`, wantErr: wrapTypeError(ErrLeadingZero, uint64T)},
	{input: `"0xfffffffffffffffff"`, wantErr: wrapTypeError(ErrUint64Range, uint64T)},
	{input: `"0xx"`, wantErr: wrapTypeError(ErrSyntax, uint64T)},
	{input: `"0x1zz01"`, wantErr: wrapTypeError(ErrSyntax, uint64T)},

//有效编码
	{input: `""`, want: uint64(0)},
	{input: `"0x0"`, want: uint64(0)},
	{input: `"0x2"`, want: uint64(0x2)},
	{input: `"0x2F2"`, want: uint64(0x2f2)},
	{input: `"0X2F2"`, want: uint64(0x2f2)},
	{input: `"0x1122aaff"`, want: uint64(0x1122aaff)},
	{input: `"0xbbb"`, want: uint64(0xbbb)},
	{input: `"0xffffffffffffffff"`, want: uint64(0xffffffffffffffff)},
}

func TestUnmarshalUint64(t *testing.T) {
	for _, test := range unmarshalUint64Tests {
		var v Uint64
		err := json.Unmarshal([]byte(test.input), &v)
		if !checkError(t, test.input, err, test.wantErr) {
			continue
		}
		if uint64(v) != test.want.(uint64) {
			t.Errorf("input %s: value mismatch: got %d, want %d", test.input, v, test.want)
			continue
		}
	}
}

func BenchmarkUnmarshalUint64(b *testing.B) {
	input := []byte(`"0x123456789abcdf"`)
	for i := 0; i < b.N; i++ {
		var v Uint64
		v.UnmarshalJSON(input)
	}
}

func TestMarshalUint64(t *testing.T) {
	for _, test := range encodeUint64Tests {
		in := test.input.(uint64)
		out, err := json.Marshal(Uint64(in))
		if err != nil {
			t.Errorf("%d: %v", in, err)
			continue
		}
		if want := `"` + test.want + `"`; string(out) != want {
			t.Errorf("%d: MarshalJSON output mismatch: got %q, want %q", in, out, want)
			continue
		}
		if out := (Uint64)(in).String(); out != test.want {
			t.Errorf("%x: String mismatch: got %q, want %q", in, out, test.want)
			continue
		}
	}
}

func TestMarshalUint(t *testing.T) {
	for _, test := range encodeUintTests {
		in := test.input.(uint)
		out, err := json.Marshal(Uint(in))
		if err != nil {
			t.Errorf("%d: %v", in, err)
			continue
		}
		if want := `"` + test.want + `"`; string(out) != want {
			t.Errorf("%d: MarshalJSON output mismatch: got %q, want %q", in, out, want)
			continue
		}
		if out := (Uint)(in).String(); out != test.want {
			t.Errorf("%x: String mismatch: got %q, want %q", in, out, test.want)
			continue
		}
	}
}

var (
//这些是变量（不是常量），以避免常量溢出
//在32位平台上签入编译器。
	maxUint33bits = uint64(^uint32(0)) + 1
	maxUint64bits = ^uint64(0)
)

var unmarshalUintTests = []unmarshalTest{
//无效的编码
	{input: "", wantErr: errJSONEOF},
	{input: "null", wantErr: errNonString(uintT)},
	{input: "10", wantErr: errNonString(uintT)},
	{input: `"0"`, wantErr: wrapTypeError(ErrMissingPrefix, uintT)},
	{input: `"0x"`, wantErr: wrapTypeError(ErrEmptyNumber, uintT)},
	{input: `"0x01"`, wantErr: wrapTypeError(ErrLeadingZero, uintT)},
	{input: `"0x100000000"`, want: uint(maxUint33bits), wantErr32bit: wrapTypeError(ErrUintRange, uintT)},
	{input: `"0xfffffffffffffffff"`, wantErr: wrapTypeError(ErrUintRange, uintT)},
	{input: `"0xx"`, wantErr: wrapTypeError(ErrSyntax, uintT)},
	{input: `"0x1zz01"`, wantErr: wrapTypeError(ErrSyntax, uintT)},

//有效编码
	{input: `""`, want: uint(0)},
	{input: `"0x0"`, want: uint(0)},
	{input: `"0x2"`, want: uint(0x2)},
	{input: `"0x2F2"`, want: uint(0x2f2)},
	{input: `"0X2F2"`, want: uint(0x2f2)},
	{input: `"0x1122aaff"`, want: uint(0x1122aaff)},
	{input: `"0xbbb"`, want: uint(0xbbb)},
	{input: `"0xffffffff"`, want: uint(0xffffffff)},
	{input: `"0xffffffffffffffff"`, want: uint(maxUint64bits), wantErr32bit: wrapTypeError(ErrUintRange, uintT)},
}

func TestUnmarshalUint(t *testing.T) {
	for _, test := range unmarshalUintTests {
		var v Uint
		err := json.Unmarshal([]byte(test.input), &v)
		if uintBits == 32 && test.wantErr32bit != nil {
			checkError(t, test.input, err, test.wantErr32bit)
			continue
		}
		if !checkError(t, test.input, err, test.wantErr) {
			continue
		}
		if uint(v) != test.want.(uint) {
			t.Errorf("input %s: value mismatch: got %d, want %d", test.input, v, test.want)
			continue
		}
	}
}

func TestUnmarshalFixedUnprefixedText(t *testing.T) {
	tests := []struct {
		input   string
		want    []byte
		wantErr error
	}{
		{input: "0x2", wantErr: ErrOddLength},
		{input: "2", wantErr: ErrOddLength},
		{input: "4444", wantErr: errors.New("hex string has length 4, want 8 for x")},
		{input: "4444", wantErr: errors.New("hex string has length 4, want 8 for x")},
//检查输出是否为部分正确输入而未修改
		{input: "444444gg", wantErr: ErrSyntax, want: []byte{0, 0, 0, 0}},
		{input: "0x444444gg", wantErr: ErrSyntax, want: []byte{0, 0, 0, 0}},
//有效输入
		{input: "44444444", want: []byte{0x44, 0x44, 0x44, 0x44}},
		{input: "0x44444444", want: []byte{0x44, 0x44, 0x44, 0x44}},
	}

	for _, test := range tests {
		out := make([]byte, 4)
		err := UnmarshalFixedUnprefixedText("x", []byte(test.input), out)
		switch {
		case err == nil && test.wantErr != nil:
			t.Errorf("%q: got no error, expected %q", test.input, test.wantErr)
		case err != nil && test.wantErr == nil:
			t.Errorf("%q: unexpected error %q", test.input, err)
		case err != nil && err.Error() != test.wantErr.Error():
			t.Errorf("%q: error mismatch: got %q, want %q", test.input, err, test.wantErr)
		}
		if test.want != nil && !bytes.Equal(out, test.want) {
			t.Errorf("%q: output mismatch: got %x, want %x", test.input, out, test.want)
		}
	}
}
