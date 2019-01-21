
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
	"math/big"
	"testing"
)

type marshalTest struct {
	input interface{}
	want  string
}

type unmarshalTest struct {
	input        string
	want         interface{}
wantErr      error //如果设置，则任何平台上的解码都必须失败。
wantErr32bit error //如果设置，则解码必须在32位平台上失败（用于uint测试）
}

var (
	encodeBytesTests = []marshalTest{
		{[]byte{}, "0x"},
		{[]byte{0}, "0x00"},
		{[]byte{0, 0, 1, 2}, "0x00000102"},
	}

	encodeBigTests = []marshalTest{
		{referenceBig("0"), "0x0"},
		{referenceBig("1"), "0x1"},
		{referenceBig("ff"), "0xff"},
		{referenceBig("112233445566778899aabbccddeeff"), "0x112233445566778899aabbccddeeff"},
		{referenceBig("80a7f2c1bcc396c00"), "0x80a7f2c1bcc396c00"},
		{referenceBig("-80a7f2c1bcc396c00"), "-0x80a7f2c1bcc396c00"},
	}

	encodeUint64Tests = []marshalTest{
		{uint64(0), "0x0"},
		{uint64(1), "0x1"},
		{uint64(0xff), "0xff"},
		{uint64(0x1122334455667788), "0x1122334455667788"},
	}

	encodeUintTests = []marshalTest{
		{uint(0), "0x0"},
		{uint(1), "0x1"},
		{uint(0xff), "0xff"},
		{uint(0x11223344), "0x11223344"},
	}

	decodeBytesTests = []unmarshalTest{
//无效
		{input: ``, wantErr: ErrEmptyString},
		{input: `0`, wantErr: ErrMissingPrefix},
		{input: `0x0`, wantErr: ErrOddLength},
		{input: `0x023`, wantErr: ErrOddLength},
		{input: `0xxx`, wantErr: ErrSyntax},
		{input: `0x01zz01`, wantErr: ErrSyntax},
//有效的
		{input: `0x`, want: []byte{}},
		{input: `0X`, want: []byte{}},
		{input: `0x02`, want: []byte{0x02}},
		{input: `0X02`, want: []byte{0x02}},
		{input: `0xffffffffff`, want: []byte{0xff, 0xff, 0xff, 0xff, 0xff}},
		{
			input: `0xffffffffffffffffffffffffffffffffffff`,
			want:  []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		},
	}

	decodeBigTests = []unmarshalTest{
//无效
		{input: `0`, wantErr: ErrMissingPrefix},
		{input: `0x`, wantErr: ErrEmptyNumber},
		{input: `0x01`, wantErr: ErrLeadingZero},
		{input: `0xx`, wantErr: ErrSyntax},
		{input: `0x1zz01`, wantErr: ErrSyntax},
		{
			input:   `0x10000000000000000000000000000000000000000000000000000000000000000`,
			wantErr: ErrBig256Range,
		},
//有效的
		{input: `0x0`, want: big.NewInt(0)},
		{input: `0x2`, want: big.NewInt(0x2)},
		{input: `0x2F2`, want: big.NewInt(0x2f2)},
		{input: `0X2F2`, want: big.NewInt(0x2f2)},
		{input: `0x1122aaff`, want: big.NewInt(0x1122aaff)},
		{input: `0xbBb`, want: big.NewInt(0xbbb)},
		{input: `0xfffffffff`, want: big.NewInt(0xfffffffff)},
		{
			input: `0x112233445566778899aabbccddeeff`,
			want:  referenceBig("112233445566778899aabbccddeeff"),
		},
		{
			input: `0xffffffffffffffffffffffffffffffffffff`,
			want:  referenceBig("ffffffffffffffffffffffffffffffffffff"),
		},
		{
			input: `0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff`,
			want:  referenceBig("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
		},
	}

	decodeUint64Tests = []unmarshalTest{
//无效
		{input: `0`, wantErr: ErrMissingPrefix},
		{input: `0x`, wantErr: ErrEmptyNumber},
		{input: `0x01`, wantErr: ErrLeadingZero},
		{input: `0xfffffffffffffffff`, wantErr: ErrUint64Range},
		{input: `0xx`, wantErr: ErrSyntax},
		{input: `0x1zz01`, wantErr: ErrSyntax},
//有效的
		{input: `0x0`, want: uint64(0)},
		{input: `0x2`, want: uint64(0x2)},
		{input: `0x2F2`, want: uint64(0x2f2)},
		{input: `0X2F2`, want: uint64(0x2f2)},
		{input: `0x1122aaff`, want: uint64(0x1122aaff)},
		{input: `0xbbb`, want: uint64(0xbbb)},
		{input: `0xffffffffffffffff`, want: uint64(0xffffffffffffffff)},
	}
)

func TestEncode(t *testing.T) {
	for _, test := range encodeBytesTests {
		enc := Encode(test.input.([]byte))
		if enc != test.want {
			t.Errorf("input %x: wrong encoding %s", test.input, enc)
		}
	}
}

func TestDecode(t *testing.T) {
	for _, test := range decodeBytesTests {
		dec, err := Decode(test.input)
		if !checkError(t, test.input, err, test.wantErr) {
			continue
		}
		if !bytes.Equal(test.want.([]byte), dec) {
			t.Errorf("input %s: value mismatch: got %x, want %x", test.input, dec, test.want)
			continue
		}
	}
}

func TestEncodeBig(t *testing.T) {
	for _, test := range encodeBigTests {
		enc := EncodeBig(test.input.(*big.Int))
		if enc != test.want {
			t.Errorf("input %x: wrong encoding %s", test.input, enc)
		}
	}
}

func TestDecodeBig(t *testing.T) {
	for _, test := range decodeBigTests {
		dec, err := DecodeBig(test.input)
		if !checkError(t, test.input, err, test.wantErr) {
			continue
		}
		if dec.Cmp(test.want.(*big.Int)) != 0 {
			t.Errorf("input %s: value mismatch: got %x, want %x", test.input, dec, test.want)
			continue
		}
	}
}

func TestEncodeUint64(t *testing.T) {
	for _, test := range encodeUint64Tests {
		enc := EncodeUint64(test.input.(uint64))
		if enc != test.want {
			t.Errorf("input %x: wrong encoding %s", test.input, enc)
		}
	}
}

func TestDecodeUint64(t *testing.T) {
	for _, test := range decodeUint64Tests {
		dec, err := DecodeUint64(test.input)
		if !checkError(t, test.input, err, test.wantErr) {
			continue
		}
		if dec != test.want.(uint64) {
			t.Errorf("input %s: value mismatch: got %x, want %x", test.input, dec, test.want)
			continue
		}
	}
}
