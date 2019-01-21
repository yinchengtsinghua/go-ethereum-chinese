
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
//此文件是Go以太坊的一部分。
//
//Go以太坊是免费软件：您可以重新发布和/或修改它
//根据GNU通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊的分布希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU通用公共许可证了解更多详细信息。
//
//你应该已经收到一份GNU通用公共许可证的副本
//一起去以太坊吧。如果没有，请参见<http://www.gnu.org/licenses/>。

package core

import (
	"fmt"
	"strings"
	"testing"

	"io/ioutil"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func verify(t *testing.T, jsondata, calldata string, exp []interface{}) {

	abispec, err := abi.JSON(strings.NewReader(jsondata))
	if err != nil {
		t.Fatal(err)
	}
	cd := common.Hex2Bytes(calldata)
	sigdata, argdata := cd[:4], cd[4:]
	method, err := abispec.MethodById(sigdata)
	if err != nil {
		t.Fatal(err)
	}
	data, err := method.Inputs.UnpackValues(argdata)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != len(exp) {
		t.Fatalf("Mismatched length, expected %d, got %d", len(exp), len(data))
	}
	for i, elem := range data {
		if !reflect.DeepEqual(elem, exp[i]) {
			t.Fatalf("Unpack error, arg %d, got %v, want %v", i, elem, exp[i])
		}
	}
}
func TestNewUnpacker(t *testing.T) {
	type unpackTest struct {
		jsondata string
		calldata string
		exp      []interface{}
	}
	testcases := []unpackTest{
{ //https://solidity.readthedocs.io/en/develop/abi-spec.html使用动态类型
			`[{"type":"function","name":"f", "inputs":[{"type":"uint256"},{"type":"uint32[]"},{"type":"bytes10"},{"type":"bytes"}]}]`,
//0x123，[0x456，0x789]，“1234567890”，“你好，世界！”
			"8be65246" + "00000000000000000000000000000000000000000000000000000000000001230000000000000000000000000000000000000000000000000000000000000080313233343536373839300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000004560000000000000000000000000000000000000000000000000000000000000789000000000000000000000000000000000000000000000000000000000000000d48656c6c6f2c20776f726c642100000000000000000000000000000000000000",
			[]interface{}{
				big.NewInt(0x123),
				[]uint32{0x456, 0x789},
				[10]byte{49, 50, 51, 52, 53, 54, 55, 56, 57, 48},
				common.Hex2Bytes("48656c6c6f2c20776f726c6421"),
			},
}, { //https://github.com/ethereum/wiki/wiki/ethereum contract abi示例
			`[{"type":"function","name":"sam","inputs":[{"type":"bytes"},{"type":"bool"},{"type":"uint256[]"}]}]`,
//“Dave”，真的和[1,2,3]
			"a5643bf20000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000000464617665000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
			[]interface{}{
				[]byte{0x64, 0x61, 0x76, 0x65},
				true,
				[]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
			},
		}, {
			`[{"type":"function","name":"send","inputs":[{"type":"uint256"}]}]`,
			"a52c101e0000000000000000000000000000000000000000000000000000000000000012",
			[]interface{}{big.NewInt(0x12)},
		}, {
			`[{"type":"function","name":"compareAndApprove","inputs":[{"type":"address"},{"type":"uint256"},{"type":"uint256"}]}]`,
			"751e107900000000000000000000000000000133700000deadbeef00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001",
			[]interface{}{
				common.HexToAddress("0x00000133700000deadbeef000000000000000000"),
				new(big.Int).SetBytes([]byte{0x00}),
				big.NewInt(0x1),
			},
		},
	}
	for _, c := range testcases {
		verify(t, c.jsondata, c.calldata, c.exp)
	}

}

func TestCalldataDecoding(t *testing.T) {

//发送（uint256）：a52c101e
//比较数据证明（地址，uint256，uint256）：751E1079
//问题（地址[]，uint256）：42958b54
	jsondata := `
[
	{"type":"function","name":"send","inputs":[{"name":"a","type":"uint256"}]},
	{"type":"function","name":"compareAndApprove","inputs":[{"name":"a","type":"address"},{"name":"a","type":"uint256"},{"name":"a","type":"uint256"}]},
	{"type":"function","name":"issue","inputs":[{"name":"a","type":"address[]"},{"name":"a","type":"uint256"}]},
	{"type":"function","name":"sam","inputs":[{"name":"a","type":"bytes"},{"name":"a","type":"bool"},{"name":"a","type":"uint256[]"}]}
]`
//预期故障
	for i, hexdata := range []string{
		"a52c101e00000000000000000000000000000000000000000000000000000000000000120000000000000000000000000000000000000000000000000000000000000042",
		"a52c101e000000000000000000000000000000000000000000000000000000000000001200",
		"a52c101e00000000000000000000000000000000000000000000000000000000000000",
		"a52c101e",
		"a52c10",
		"",
//太短
		"751e10790000000000000000000000000000000000000000000000000000000000000012",
		"751e1079FFffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
//不是32的有效倍数
		"deadbeef00000000000000000000000000000000000000000000000000000000000000",
//“问题”太短
		"42958b5400000000000000000000000000000000000000000000000000000000000000120000000000000000000000000000000000000000000000000000000000000042",
//比较过短，不适用
		"a52c101e00ff0000000000000000000000000000000000000000000000000000000000120000000000000000000000000000000000000000000000000000000000000042",
//来自https://github.com/ethereum/wiki/wiki/ethereum-contract-abi
//包含具有非法值的bool
		"a5643bf20000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000001100000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000000464617665000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
	} {
		_, err := parseCallData(common.Hex2Bytes(hexdata), jsondata)
		if err == nil {
			t.Errorf("test %d: expected decoding to fail: %s", i, hexdata)
		}
	}
//预期成功
	for i, hexdata := range []string{
//来自https://github.com/ethereum/wiki/wiki/ethereum-contract-abi
		"a5643bf20000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000000464617665000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
		"a52c101e0000000000000000000000000000000000000000000000000000000000000012",
		"a52c101eFFffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		"751e1079000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		"42958b54" +
//动态类型的开始
			"0000000000000000000000000000000000000000000000000000000000000040" +
//UIT2525
			"0000000000000000000000000000000000000000000000000000000000000001" +
//阵列长度
			"0000000000000000000000000000000000000000000000000000000000000002" +
//数组值
			"000000000000000000000000000000000000000000000000000000000000dead" +
			"000000000000000000000000000000000000000000000000000000000000beef",
	} {
		_, err := parseCallData(common.Hex2Bytes(hexdata), jsondata)
		if err != nil {
			t.Errorf("test %d: unexpected failure on input %s:\n %v (%d bytes) ", i, hexdata, err, len(common.Hex2Bytes(hexdata)))
		}
	}
}

func TestSelectorUnmarshalling(t *testing.T) {
	var (
		db        *AbiDb
		err       error
		abistring []byte
		abistruct abi.ABI
	)

	db, err = NewAbiDBFromFile("../../cmd/clef/4byte.json")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("DB size %v\n", db.Size())
	for id, selector := range db.db {

		abistring, err = MethodSelectorToAbi(selector)
		if err != nil {
			t.Error(err)
			return
		}
		abistruct, err = abi.JSON(strings.NewReader(string(abistring)))
		if err != nil {
			t.Error(err)
			return
		}
		m, err := abistruct.MethodById(common.Hex2Bytes(id[2:]))
		if err != nil {
			t.Error(err)
			return
		}
		if m.Sig() != selector {
			t.Errorf("Expected equality: %v != %v", m.Sig(), selector)
		}
	}

}

func TestCustomABI(t *testing.T) {
	d, err := ioutil.TempDir("", "signer-4byte-test")
	if err != nil {
		t.Fatal(err)
	}
	filename := fmt.Sprintf("%s/4byte_custom.json", d)
	abidb, err := NewAbiDBFromFiles("../../cmd/clef/4byte.json", filename)
	if err != nil {
		t.Fatal(err)
	}
//现在我们将删除所有现有签名
	abidb.db = make(map[string]string)
	calldata := common.Hex2Bytes("a52c101edeadbeef")
	_, err = abidb.LookupMethodSelector(calldata)
	if err == nil {
		t.Fatalf("Should not find a match on empty db")
	}
	if err = abidb.AddSignature("send(uint256)", calldata); err != nil {
		t.Fatalf("Failed to save file: %v", err)
	}
	_, err = abidb.LookupMethodSelector(calldata)
	if err != nil {
		t.Fatalf("Should find a match for abi signature, got: %v", err)
	}
//检查它是否写入文件
	abidb2, err := NewAbiDBFromFile(filename)
	if err != nil {
		t.Fatalf("Failed to create new abidb: %v", err)
	}
	_, err = abidb2.LookupMethodSelector(calldata)
	if err != nil {
		t.Fatalf("Save failed: should find a match for abi signature after loading from disk")
	}
}

func TestMaliciousAbiStrings(t *testing.T) {
	tests := []string{
		"func(uint256,uint256,[]uint256)",
		"func(uint256,uint256,uint256,)",
		"func(,uint256,uint256,uint256)",
	}
	data := common.Hex2Bytes("4401a6e40000000000000000000000000000000000000000000000000000000000000012")
	for i, tt := range tests {
		_, err := testSelector(tt, data)
		if err == nil {
			t.Errorf("test %d: expected error for selector '%v'", i, tt)
		}
	}
}
