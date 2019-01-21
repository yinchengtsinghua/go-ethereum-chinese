
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Go Ethereum作者
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

package abi

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const jsondata = `
[
	{ "type" : "function", "name" : "balance", "constant" : true },
	{ "type" : "function", "name" : "send", "constant" : false, "inputs" : [ { "name" : "amount", "type" : "uint256" } ] }
]`

const jsondata2 = `
[
	{ "type" : "function", "name" : "balance", "constant" : true },
	{ "type" : "function", "name" : "send", "constant" : false, "inputs" : [ { "name" : "amount", "type" : "uint256" } ] },
	{ "type" : "function", "name" : "test", "constant" : false, "inputs" : [ { "name" : "number", "type" : "uint32" } ] },
	{ "type" : "function", "name" : "string", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "string" } ] },
	{ "type" : "function", "name" : "bool", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "bool" } ] },
	{ "type" : "function", "name" : "address", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "address" } ] },
	{ "type" : "function", "name" : "uint64[2]", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "uint64[2]" } ] },
	{ "type" : "function", "name" : "uint64[]", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "uint64[]" } ] },
	{ "type" : "function", "name" : "foo", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "uint32" } ] },
	{ "type" : "function", "name" : "bar", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "uint32" }, { "name" : "string", "type" : "uint16" } ] },
	{ "type" : "function", "name" : "slice", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "uint32[2]" } ] },
	{ "type" : "function", "name" : "slice256", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "uint256[2]" } ] },
	{ "type" : "function", "name" : "sliceAddress", "constant" : false, "inputs" : [ { "name" : "inputs", "type" : "address[]" } ] },
	{ "type" : "function", "name" : "sliceMultiAddress", "constant" : false, "inputs" : [ { "name" : "a", "type" : "address[]" }, { "name" : "b", "type" : "address[]" } ] },
	{ "type" : "function", "name" : "nestedArray", "constant" : false, "inputs" : [ { "name" : "a", "type" : "uint256[2][2]" }, { "name" : "b", "type" : "address[]" } ] },
	{ "type" : "function", "name" : "nestedArray2", "constant" : false, "inputs" : [ { "name" : "a", "type" : "uint8[][2]" } ] },
	{ "type" : "function", "name" : "nestedSlice", "constant" : false, "inputs" : [ { "name" : "a", "type" : "uint8[][]" } ] }
]`

func TestReader(t *testing.T) {
	Uint256, _ := NewType("uint256", nil)
	exp := ABI{
		Methods: map[string]Method{
			"balance": {
				"balance", true, nil, nil,
			},
			"send": {
				"send", false, []Argument{
					{"amount", Uint256, false},
				}, nil,
			},
		},
	}

	abi, err := JSON(strings.NewReader(jsondata))
	if err != nil {
		t.Error(err)
	}

//深相等由于某种原因而失败
	for name, expM := range exp.Methods {
		gotM, exist := abi.Methods[name]
		if !exist {
			t.Errorf("Missing expected method %v", name)
		}
		if !reflect.DeepEqual(gotM, expM) {
			t.Errorf("\nGot abi method: \n%v\ndoes not match expected method\n%v", gotM, expM)
		}
	}

	for name, gotM := range abi.Methods {
		expM, exist := exp.Methods[name]
		if !exist {
			t.Errorf("Found extra method %v", name)
		}
		if !reflect.DeepEqual(gotM, expM) {
			t.Errorf("\nGot abi method: \n%v\ndoes not match expected method\n%v", gotM, expM)
		}
	}
}

func TestTestNumbers(t *testing.T) {
	abi, err := JSON(strings.NewReader(jsondata2))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if _, err := abi.Pack("balance"); err != nil {
		t.Error(err)
	}

	if _, err := abi.Pack("balance", 1); err == nil {
		t.Error("expected error for balance(1)")
	}

	if _, err := abi.Pack("doesntexist", nil); err == nil {
		t.Errorf("doesntexist shouldn't exist")
	}

	if _, err := abi.Pack("doesntexist", 1); err == nil {
		t.Errorf("doesntexist(1) shouldn't exist")
	}

	if _, err := abi.Pack("send", big.NewInt(1000)); err != nil {
		t.Error(err)
	}

	i := new(int)
	*i = 1000
	if _, err := abi.Pack("send", i); err == nil {
		t.Errorf("expected send( ptr ) to throw, requires *big.Int instead of *int")
	}

	if _, err := abi.Pack("test", uint32(1000)); err != nil {
		t.Error(err)
	}
}

func TestTestString(t *testing.T) {
	abi, err := JSON(strings.NewReader(jsondata2))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if _, err := abi.Pack("string", "hello world"); err != nil {
		t.Error(err)
	}
}

func TestTestBool(t *testing.T) {
	abi, err := JSON(strings.NewReader(jsondata2))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if _, err := abi.Pack("bool", true); err != nil {
		t.Error(err)
	}
}

func TestTestSlice(t *testing.T) {
	abi, err := JSON(strings.NewReader(jsondata2))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	slice := make([]uint64, 2)
	if _, err := abi.Pack("uint64[2]", slice); err != nil {
		t.Error(err)
	}

	if _, err := abi.Pack("uint64[]", slice); err != nil {
		t.Error(err)
	}
}

func TestMethodSignature(t *testing.T) {
	String, _ := NewType("string", nil)
	m := Method{"foo", false, []Argument{{"bar", String, false}, {"baz", String, false}}, nil}
	exp := "foo(string,string)"
	if m.Sig() != exp {
		t.Error("signature mismatch", exp, "!=", m.Sig())
	}

	idexp := crypto.Keccak256([]byte(exp))[:4]
	if !bytes.Equal(m.Id(), idexp) {
		t.Errorf("expected ids to match %x != %x", m.Id(), idexp)
	}

	uintt, _ := NewType("uint256", nil)
	m = Method{"foo", false, []Argument{{"bar", uintt, false}}, nil}
	exp = "foo(uint256)"
	if m.Sig() != exp {
		t.Error("signature mismatch", exp, "!=", m.Sig())
	}

//具有元组参数的方法
	s, _ := NewType("tuple", []ArgumentMarshaling{
		{Name: "a", Type: "int256"},
		{Name: "b", Type: "int256[]"},
		{Name: "c", Type: "tuple[]", Components: []ArgumentMarshaling{
			{Name: "x", Type: "int256"},
			{Name: "y", Type: "int256"},
		}},
		{Name: "d", Type: "tuple[2]", Components: []ArgumentMarshaling{
			{Name: "x", Type: "int256"},
			{Name: "y", Type: "int256"},
		}},
	})
	m = Method{"foo", false, []Argument{{"s", s, false}, {"bar", String, false}}, nil}
	exp = "foo((int256,int256[],(int256,int256)[],(int256,int256)[2]),string)"
	if m.Sig() != exp {
		t.Error("signature mismatch", exp, "!=", m.Sig())
	}
}

func TestMultiPack(t *testing.T) {
	abi, err := JSON(strings.NewReader(jsondata2))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	sig := crypto.Keccak256([]byte("bar(uint32,uint16)"))[:4]
	sig = append(sig, make([]byte, 64)...)
	sig[35] = 10
	sig[67] = 11

	packed, err := abi.Pack("bar", uint32(10), uint16(11))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}
}

func ExampleJSON() {
	const definition = `[{"constant":true,"inputs":[{"name":"","type":"address"}],"name":"isBar","outputs":[{"name":"","type":"bool"}],"type":"function"}]`

	abi, err := JSON(strings.NewReader(definition))
	if err != nil {
		log.Fatalln(err)
	}
	out, err := abi.Pack("isBar", common.HexToAddress("01"))
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("%x\n", out)
//输出：
//1FBC40920000000000000000000000000000000000000000000000000000000000000000001
}

func TestInputVariableInputLength(t *testing.T) {
	const definition = `[
	{ "type" : "function", "name" : "strOne", "constant" : true, "inputs" : [ { "name" : "str", "type" : "string" } ] },
	{ "type" : "function", "name" : "bytesOne", "constant" : true, "inputs" : [ { "name" : "str", "type" : "bytes" } ] },
	{ "type" : "function", "name" : "strTwo", "constant" : true, "inputs" : [ { "name" : "str", "type" : "string" }, { "name" : "str1", "type" : "string" } ] }
	]`

	abi, err := JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}

//测试一个字符串
	strin := "hello world"
	strpack, err := abi.Pack("strOne", strin)
	if err != nil {
		t.Error(err)
	}

	offset := make([]byte, 32)
	offset[31] = 32
	length := make([]byte, 32)
	length[31] = byte(len(strin))
	value := common.RightPadBytes([]byte(strin), 32)
	exp := append(offset, append(length, value...)...)

//忽略输出的前4个字节。这是函数标识符
	strpack = strpack[4:]
	if !bytes.Equal(strpack, exp) {
		t.Errorf("expected %x, got %x\n", exp, strpack)
	}

//测试一个字节
	btspack, err := abi.Pack("bytesOne", []byte(strin))
	if err != nil {
		t.Error(err)
	}
//忽略输出的前4个字节。这是函数标识符
	btspack = btspack[4:]
	if !bytes.Equal(btspack, exp) {
		t.Errorf("expected %x, got %x\n", exp, btspack)
	}

//测试两个字符串
	str1 := "hello"
	str2 := "world"
	str2pack, err := abi.Pack("strTwo", str1, str2)
	if err != nil {
		t.Error(err)
	}

	offset1 := make([]byte, 32)
	offset1[31] = 64
	length1 := make([]byte, 32)
	length1[31] = byte(len(str1))
	value1 := common.RightPadBytes([]byte(str1), 32)

	offset2 := make([]byte, 32)
	offset2[31] = 128
	length2 := make([]byte, 32)
	length2[31] = byte(len(str2))
	value2 := common.RightPadBytes([]byte(str2), 32)

	exp2 := append(offset1, offset2...)
	exp2 = append(exp2, append(length1, value1...)...)
	exp2 = append(exp2, append(length2, value2...)...)

//忽略输出的前4个字节。这是函数标识符
	str2pack = str2pack[4:]
	if !bytes.Equal(str2pack, exp2) {
		t.Errorf("expected %x, got %x\n", exp, str2pack)
	}

//测试两个字符串，第一个>32，第二个<32
	str1 = strings.Repeat("a", 33)
	str2pack, err = abi.Pack("strTwo", str1, str2)
	if err != nil {
		t.Error(err)
	}

	offset1 = make([]byte, 32)
	offset1[31] = 64
	length1 = make([]byte, 32)
	length1[31] = byte(len(str1))
	value1 = common.RightPadBytes([]byte(str1), 64)
	offset2[31] = 160

	exp2 = append(offset1, offset2...)
	exp2 = append(exp2, append(length1, value1...)...)
	exp2 = append(exp2, append(length2, value2...)...)

//忽略输出的前4个字节。这是函数标识符
	str2pack = str2pack[4:]
	if !bytes.Equal(str2pack, exp2) {
		t.Errorf("expected %x, got %x\n", exp, str2pack)
	}

//测试两个字符串，第一个>32，第二个>32
	str1 = strings.Repeat("a", 33)
	str2 = strings.Repeat("a", 33)
	str2pack, err = abi.Pack("strTwo", str1, str2)
	if err != nil {
		t.Error(err)
	}

	offset1 = make([]byte, 32)
	offset1[31] = 64
	length1 = make([]byte, 32)
	length1[31] = byte(len(str1))
	value1 = common.RightPadBytes([]byte(str1), 64)

	offset2 = make([]byte, 32)
	offset2[31] = 160
	length2 = make([]byte, 32)
	length2[31] = byte(len(str2))
	value2 = common.RightPadBytes([]byte(str2), 64)

	exp2 = append(offset1, offset2...)
	exp2 = append(exp2, append(length1, value1...)...)
	exp2 = append(exp2, append(length2, value2...)...)

//忽略输出的前4个字节。这是函数标识符
	str2pack = str2pack[4:]
	if !bytes.Equal(str2pack, exp2) {
		t.Errorf("expected %x, got %x\n", exp, str2pack)
	}
}

func TestInputFixedArrayAndVariableInputLength(t *testing.T) {
	const definition = `[
	{ "type" : "function", "name" : "fixedArrStr", "constant" : true, "inputs" : [ { "name" : "str", "type" : "string" }, { "name" : "fixedArr", "type" : "uint256[2]" } ] },
	{ "type" : "function", "name" : "fixedArrBytes", "constant" : true, "inputs" : [ { "name" : "str", "type" : "bytes" }, { "name" : "fixedArr", "type" : "uint256[2]" } ] },
    { "type" : "function", "name" : "mixedArrStr", "constant" : true, "inputs" : [ { "name" : "str", "type" : "string" }, { "name" : "fixedArr", "type": "uint256[2]" }, { "name" : "dynArr", "type": "uint256[]" } ] },
    { "type" : "function", "name" : "doubleFixedArrStr", "constant" : true, "inputs" : [ { "name" : "str", "type" : "string" }, { "name" : "fixedArr1", "type": "uint256[2]" }, { "name" : "fixedArr2", "type": "uint256[3]" } ] },
    { "type" : "function", "name" : "multipleMixedArrStr", "constant" : true, "inputs" : [ { "name" : "str", "type" : "string" }, { "name" : "fixedArr1", "type": "uint256[2]" }, { "name" : "dynArr", "type" : "uint256[]" }, { "name" : "fixedArr2", "type" : "uint256[3]" } ] }
	]`

	abi, err := JSON(strings.NewReader(definition))
	if err != nil {
		t.Error(err)
	}

//测试字符串，固定数组uint256[2]
	strin := "hello world"
	arrin := [2]*big.Int{big.NewInt(1), big.NewInt(2)}
	fixedArrStrPack, err := abi.Pack("fixedArrStr", strin, arrin)
	if err != nil {
		t.Error(err)
	}

//生成预期输出
	offset := make([]byte, 32)
	offset[31] = 96
	length := make([]byte, 32)
	length[31] = byte(len(strin))
	strvalue := common.RightPadBytes([]byte(strin), 32)
	arrinvalue1 := common.LeftPadBytes(arrin[0].Bytes(), 32)
	arrinvalue2 := common.LeftPadBytes(arrin[1].Bytes(), 32)
	exp := append(offset, arrinvalue1...)
	exp = append(exp, arrinvalue2...)
	exp = append(exp, append(length, strvalue...)...)

//忽略输出的前4个字节。这是函数标识符
	fixedArrStrPack = fixedArrStrPack[4:]
	if !bytes.Equal(fixedArrStrPack, exp) {
		t.Errorf("expected %x, got %x\n", exp, fixedArrStrPack)
	}

//测试字节数组，固定数组uint256[2]
	bytesin := []byte(strin)
	arrin = [2]*big.Int{big.NewInt(1), big.NewInt(2)}
	fixedArrBytesPack, err := abi.Pack("fixedArrBytes", bytesin, arrin)
	if err != nil {
		t.Error(err)
	}

//生成预期输出
	offset = make([]byte, 32)
	offset[31] = 96
	length = make([]byte, 32)
	length[31] = byte(len(strin))
	strvalue = common.RightPadBytes([]byte(strin), 32)
	arrinvalue1 = common.LeftPadBytes(arrin[0].Bytes(), 32)
	arrinvalue2 = common.LeftPadBytes(arrin[1].Bytes(), 32)
	exp = append(offset, arrinvalue1...)
	exp = append(exp, arrinvalue2...)
	exp = append(exp, append(length, strvalue...)...)

//忽略输出的前4个字节。这是函数标识符
	fixedArrBytesPack = fixedArrBytesPack[4:]
	if !bytes.Equal(fixedArrBytesPack, exp) {
		t.Errorf("expected %x, got %x\n", exp, fixedArrBytesPack)
	}

//测试字符串，固定数组uint256[2]，动态数组uint256[]
	strin = "hello world"
	fixedarrin := [2]*big.Int{big.NewInt(1), big.NewInt(2)}
	dynarrin := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	mixedArrStrPack, err := abi.Pack("mixedArrStr", strin, fixedarrin, dynarrin)
	if err != nil {
		t.Error(err)
	}

//生成预期输出
	stroffset := make([]byte, 32)
	stroffset[31] = 128
	strlength := make([]byte, 32)
	strlength[31] = byte(len(strin))
	strvalue = common.RightPadBytes([]byte(strin), 32)
	fixedarrinvalue1 := common.LeftPadBytes(fixedarrin[0].Bytes(), 32)
	fixedarrinvalue2 := common.LeftPadBytes(fixedarrin[1].Bytes(), 32)
	dynarroffset := make([]byte, 32)
	dynarroffset[31] = byte(160 + ((len(strin)/32)+1)*32)
	dynarrlength := make([]byte, 32)
	dynarrlength[31] = byte(len(dynarrin))
	dynarrinvalue1 := common.LeftPadBytes(dynarrin[0].Bytes(), 32)
	dynarrinvalue2 := common.LeftPadBytes(dynarrin[1].Bytes(), 32)
	dynarrinvalue3 := common.LeftPadBytes(dynarrin[2].Bytes(), 32)
	exp = append(stroffset, fixedarrinvalue1...)
	exp = append(exp, fixedarrinvalue2...)
	exp = append(exp, dynarroffset...)
	exp = append(exp, append(strlength, strvalue...)...)
	dynarrarg := append(dynarrlength, dynarrinvalue1...)
	dynarrarg = append(dynarrarg, dynarrinvalue2...)
	dynarrarg = append(dynarrarg, dynarrinvalue3...)
	exp = append(exp, dynarrarg...)

//忽略输出的前4个字节。这是函数标识符
	mixedArrStrPack = mixedArrStrPack[4:]
	if !bytes.Equal(mixedArrStrPack, exp) {
		t.Errorf("expected %x, got %x\n", exp, mixedArrStrPack)
	}

//测试字符串，固定数组uint256[2]，固定数组uint256[3]
	strin = "hello world"
	fixedarrin1 := [2]*big.Int{big.NewInt(1), big.NewInt(2)}
	fixedarrin2 := [3]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	doubleFixedArrStrPack, err := abi.Pack("doubleFixedArrStr", strin, fixedarrin1, fixedarrin2)
	if err != nil {
		t.Error(err)
	}

//生成预期输出
	stroffset = make([]byte, 32)
	stroffset[31] = 192
	strlength = make([]byte, 32)
	strlength[31] = byte(len(strin))
	strvalue = common.RightPadBytes([]byte(strin), 32)
	fixedarrin1value1 := common.LeftPadBytes(fixedarrin1[0].Bytes(), 32)
	fixedarrin1value2 := common.LeftPadBytes(fixedarrin1[1].Bytes(), 32)
	fixedarrin2value1 := common.LeftPadBytes(fixedarrin2[0].Bytes(), 32)
	fixedarrin2value2 := common.LeftPadBytes(fixedarrin2[1].Bytes(), 32)
	fixedarrin2value3 := common.LeftPadBytes(fixedarrin2[2].Bytes(), 32)
	exp = append(stroffset, fixedarrin1value1...)
	exp = append(exp, fixedarrin1value2...)
	exp = append(exp, fixedarrin2value1...)
	exp = append(exp, fixedarrin2value2...)
	exp = append(exp, fixedarrin2value3...)
	exp = append(exp, append(strlength, strvalue...)...)

//忽略输出的前4个字节。这是函数标识符
	doubleFixedArrStrPack = doubleFixedArrStrPack[4:]
	if !bytes.Equal(doubleFixedArrStrPack, exp) {
		t.Errorf("expected %x, got %x\n", exp, doubleFixedArrStrPack)
	}

//测试字符串，固定数组uint256[2]，动态数组uint256[]，固定数组uint256[3]
	strin = "hello world"
	fixedarrin1 = [2]*big.Int{big.NewInt(1), big.NewInt(2)}
	dynarrin = []*big.Int{big.NewInt(1), big.NewInt(2)}
	fixedarrin2 = [3]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	multipleMixedArrStrPack, err := abi.Pack("multipleMixedArrStr", strin, fixedarrin1, dynarrin, fixedarrin2)
	if err != nil {
		t.Error(err)
	}

//生成预期输出
	stroffset = make([]byte, 32)
	stroffset[31] = 224
	strlength = make([]byte, 32)
	strlength[31] = byte(len(strin))
	strvalue = common.RightPadBytes([]byte(strin), 32)
	fixedarrin1value1 = common.LeftPadBytes(fixedarrin1[0].Bytes(), 32)
	fixedarrin1value2 = common.LeftPadBytes(fixedarrin1[1].Bytes(), 32)
	dynarroffset = U256(big.NewInt(int64(256 + ((len(strin)/32)+1)*32)))
	dynarrlength = make([]byte, 32)
	dynarrlength[31] = byte(len(dynarrin))
	dynarrinvalue1 = common.LeftPadBytes(dynarrin[0].Bytes(), 32)
	dynarrinvalue2 = common.LeftPadBytes(dynarrin[1].Bytes(), 32)
	fixedarrin2value1 = common.LeftPadBytes(fixedarrin2[0].Bytes(), 32)
	fixedarrin2value2 = common.LeftPadBytes(fixedarrin2[1].Bytes(), 32)
	fixedarrin2value3 = common.LeftPadBytes(fixedarrin2[2].Bytes(), 32)
	exp = append(stroffset, fixedarrin1value1...)
	exp = append(exp, fixedarrin1value2...)
	exp = append(exp, dynarroffset...)
	exp = append(exp, fixedarrin2value1...)
	exp = append(exp, fixedarrin2value2...)
	exp = append(exp, fixedarrin2value3...)
	exp = append(exp, append(strlength, strvalue...)...)
	dynarrarg = append(dynarrlength, dynarrinvalue1...)
	dynarrarg = append(dynarrarg, dynarrinvalue2...)
	exp = append(exp, dynarrarg...)

//忽略输出的前4个字节。这是函数标识符
	multipleMixedArrStrPack = multipleMixedArrStrPack[4:]
	if !bytes.Equal(multipleMixedArrStrPack, exp) {
		t.Errorf("expected %x, got %x\n", exp, multipleMixedArrStrPack)
	}
}

func TestDefaultFunctionParsing(t *testing.T) {
	const definition = `[{ "name" : "balance" }]`

	abi, err := JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := abi.Methods["balance"]; !ok {
		t.Error("expected 'balance' to be present")
	}
}

func TestBareEvents(t *testing.T) {
	const definition = `[
	{ "type" : "event", "name" : "balance" },
	{ "type" : "event", "name" : "anon", "anonymous" : true},
	{ "type" : "event", "name" : "args", "inputs" : [{ "indexed":false, "name":"arg0", "type":"uint256" }, { "indexed":true, "name":"arg1", "type":"address" }] },
	{ "type" : "event", "name" : "tuple", "inputs" : [{ "indexed":false, "name":"t", "type":"tuple", "components":[{"name":"a", "type":"uint256"}] }, { "indexed":true, "name":"arg1", "type":"address" }] }
	]`

	arg0, _ := NewType("uint256", nil)
	arg1, _ := NewType("address", nil)
	tuple, _ := NewType("tuple", []ArgumentMarshaling{{Name: "a", Type: "uint256"}})

	expectedEvents := map[string]struct {
		Anonymous bool
		Args      []Argument
	}{
		"balance": {false, nil},
		"anon":    {true, nil},
		"args": {false, []Argument{
			{Name: "arg0", Type: arg0, Indexed: false},
			{Name: "arg1", Type: arg1, Indexed: true},
		}},
		"tuple": {false, []Argument{
			{Name: "t", Type: tuple, Indexed: false},
			{Name: "arg1", Type: arg1, Indexed: true},
		}},
	}

	abi, err := JSON(strings.NewReader(definition))
	if err != nil {
		t.Fatal(err)
	}

	if len(abi.Events) != len(expectedEvents) {
		t.Fatalf("invalid number of events after parsing, want %d, got %d", len(expectedEvents), len(abi.Events))
	}

	for name, exp := range expectedEvents {
		got, ok := abi.Events[name]
		if !ok {
			t.Errorf("could not found event %s", name)
			continue
		}
		if got.Anonymous != exp.Anonymous {
			t.Errorf("invalid anonymous indication for event %s, want %v, got %v", name, exp.Anonymous, got.Anonymous)
		}
		if len(got.Inputs) != len(exp.Args) {
			t.Errorf("invalid number of args, want %d, got %d", len(exp.Args), len(got.Inputs))
			continue
		}
		for i, arg := range exp.Args {
			if arg.Name != got.Inputs[i].Name {
				t.Errorf("events[%s].Input[%d] has an invalid name, want %s, got %s", name, i, arg.Name, got.Inputs[i].Name)
			}
			if arg.Indexed != got.Inputs[i].Indexed {
				t.Errorf("events[%s].Input[%d] has an invalid indexed indication, want %v, got %v", name, i, arg.Indexed, got.Inputs[i].Indexed)
			}
			if arg.Type.T != got.Inputs[i].Type.T {
				t.Errorf("events[%s].Input[%d] has an invalid type, want %x, got %x", name, i, arg.Type.T, got.Inputs[i].Type.T)
			}
		}
	}
}

//TestUnpackEvent基于此合同：
//合同t{
//接收到的事件（地址发送方、uint金额、字节备忘录）；
//事件接收数据（地址发送者）；
//功能接收（字节备忘录）外部应付款
//接收（msg.sender，msg.value，memo）；
//接收数据（消息发送方）；
//}
//}
//当使用发送方0x00调用Receive（“X”）时…以及值1，它生成此Tx收据：
//收据状态=1 cgas=23949 bloom=0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000 logs=[日志：B6818C8064F645CD82D99B59A1267D61117EF[75FD880D39C1DAF53B6547AB6CB59451FC6452D27CAA90E5B6649DD8293B9EED]000000000000000376C47978271565f56deb45495afa69e59c16ab200000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000018 9ae378b6d4409eada347a5dc0c180f186cb62dc68fcc0f43425eb917335aa28 0 95d429d309bb9d753954195fe2d69bd140b4ae731b9b5b605c4323DE162CF00 0]
func TestUnpackEvent(t *testing.T) {
	const abiJSON = `[{"constant":false,"inputs":[{"name":"memo","type":"bytes"}],"name":"receive","outputs":[],"payable":true,"stateMutability":"payable","type":"function"},{"anonymous":false,"inputs":[{"indexed":false,"name":"sender","type":"address"},{"indexed":false,"name":"amount","type":"uint256"},{"indexed":false,"name":"memo","type":"bytes"}],"name":"received","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"name":"sender","type":"address"}],"name":"receivedAddr","type":"event"}]`
	abi, err := JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Fatal(err)
	}

	const hexdata = `000000000000000000000000376c47978271565f56deb45495afa69e59c16ab200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000158`
	data, err := hex.DecodeString(hexdata)
	if err != nil {
		t.Fatal(err)
	}
	if len(data)%32 == 0 {
		t.Errorf("len(data) is %d, want a non-multiple of 32", len(data))
	}

	type ReceivedEvent struct {
		Sender common.Address
		Amount *big.Int
		Memo   []byte
	}
	var ev ReceivedEvent

	err = abi.Unpack(&ev, "received", data)
	if err != nil {
		t.Error(err)
	}

	type ReceivedAddrEvent struct {
		Sender common.Address
	}
	var receivedAddrEv ReceivedAddrEvent
	err = abi.Unpack(&receivedAddrEv, "receivedAddr", data)
	if err != nil {
		t.Error(err)
	}
}

func TestABI_MethodById(t *testing.T) {
	const abiJSON = `[
		{"type":"function","name":"receive","constant":false,"inputs":[{"name":"memo","type":"bytes"}],"outputs":[],"payable":true,"stateMutability":"payable"},
		{"type":"event","name":"received","anonymous":false,"inputs":[{"indexed":false,"name":"sender","type":"address"},{"indexed":false,"name":"amount","type":"uint256"},{"indexed":false,"name":"memo","type":"bytes"}]},
		{"type":"function","name":"fixedArrStr","constant":true,"inputs":[{"name":"str","type":"string"},{"name":"fixedArr","type":"uint256[2]"}]},
		{"type":"function","name":"fixedArrBytes","constant":true,"inputs":[{"name":"str","type":"bytes"},{"name":"fixedArr","type":"uint256[2]"}]},
		{"type":"function","name":"mixedArrStr","constant":true,"inputs":[{"name":"str","type":"string"},{"name":"fixedArr","type":"uint256[2]"},{"name":"dynArr","type":"uint256[]"}]},
		{"type":"function","name":"doubleFixedArrStr","constant":true,"inputs":[{"name":"str","type":"string"},{"name":"fixedArr1","type":"uint256[2]"},{"name":"fixedArr2","type":"uint256[3]"}]},
		{"type":"function","name":"multipleMixedArrStr","constant":true,"inputs":[{"name":"str","type":"string"},{"name":"fixedArr1","type":"uint256[2]"},{"name":"dynArr","type":"uint256[]"},{"name":"fixedArr2","type":"uint256[3]"}]},
		{"type":"function","name":"balance","constant":true},
		{"type":"function","name":"send","constant":false,"inputs":[{"name":"amount","type":"uint256"}]},
		{"type":"function","name":"test","constant":false,"inputs":[{"name":"number","type":"uint32"}]},
		{"type":"function","name":"string","constant":false,"inputs":[{"name":"inputs","type":"string"}]},
		{"type":"function","name":"bool","constant":false,"inputs":[{"name":"inputs","type":"bool"}]},
		{"type":"function","name":"address","constant":false,"inputs":[{"name":"inputs","type":"address"}]},
		{"type":"function","name":"uint64[2]","constant":false,"inputs":[{"name":"inputs","type":"uint64[2]"}]},
		{"type":"function","name":"uint64[]","constant":false,"inputs":[{"name":"inputs","type":"uint64[]"}]},
		{"type":"function","name":"foo","constant":false,"inputs":[{"name":"inputs","type":"uint32"}]},
		{"type":"function","name":"bar","constant":false,"inputs":[{"name":"inputs","type":"uint32"},{"name":"string","type":"uint16"}]},
		{"type":"function","name":"_slice","constant":false,"inputs":[{"name":"inputs","type":"uint32[2]"}]},
		{"type":"function","name":"__slice256","constant":false,"inputs":[{"name":"inputs","type":"uint256[2]"}]},
		{"type":"function","name":"sliceAddress","constant":false,"inputs":[{"name":"inputs","type":"address[]"}]},
		{"type":"function","name":"sliceMultiAddress","constant":false,"inputs":[{"name":"a","type":"address[]"},{"name":"b","type":"address[]"}]}
	]
`
	abi, err := JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Fatal(err)
	}
	for name, m := range abi.Methods {
		a := fmt.Sprintf("%v", m)
		m2, err := abi.MethodById(m.Id())
		if err != nil {
			t.Fatalf("Failed to look up ABI method: %v", err)
		}
		b := fmt.Sprintf("%v", m2)
		if a != b {
			t.Errorf("Method %v (id %v) not 'findable' by id in ABI", name, common.ToHex(m.Id()))
		}
	}
//也测试空
	if _, err := abi.MethodById([]byte{0x00}); err == nil {
		t.Errorf("Expected error, too short to decode data")
	}
	if _, err := abi.MethodById([]byte{}); err == nil {
		t.Errorf("Expected error, too short to decode data")
	}
	if _, err := abi.MethodById(nil); err == nil {
		t.Errorf("Expected error, nil is short to decode data")
	}
}
