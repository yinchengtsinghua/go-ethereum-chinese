
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

package feed

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

//Kv模拟密钥值存储
type KV map[string]string

func (kv KV) Get(key string) string {
	return kv[key]
}
func (kv KV) Set(key, value string) {
	kv[key] = value
}

func compareByteSliceToExpectedHex(t *testing.T, variableName string, actualValue []byte, expectedHex string) {
	if hexutil.Encode(actualValue) != expectedHex {
		t.Fatalf("%s: Expected %s to be %s, got %s", t.Name(), variableName, expectedHex, hexutil.Encode(actualValue))
	}
}

func testBinarySerializerRecovery(t *testing.T, bin binarySerializer, expectedHex string) {
	name := reflect.TypeOf(bin).Elem().Name()
	serialized := make([]byte, bin.binaryLength())
	if err := bin.binaryPut(serialized); err != nil {
		t.Fatalf("%s.binaryPut error when trying to serialize structure: %s", name, err)
	}

	compareByteSliceToExpectedHex(t, name, serialized, expectedHex)

	recovered := reflect.New(reflect.TypeOf(bin).Elem()).Interface().(binarySerializer)
	if err := recovered.binaryGet(serialized); err != nil {
		t.Fatalf("%s.binaryGet error when trying to deserialize structure: %s", name, err)
	}

	if !reflect.DeepEqual(bin, recovered) {
		t.Fatalf("Expected that the recovered %s equals the marshalled %s", name, name)
	}

	serializedWrongLength := make([]byte, 1)
	copy(serializedWrongLength[:], serialized)
	if err := recovered.binaryGet(serializedWrongLength); err == nil {
		t.Fatalf("Expected %s.binaryGet to fail since data is too small", name)
	}
}

func testBinarySerializerLengthCheck(t *testing.T, bin binarySerializer) {
	name := reflect.TypeOf(bin).Elem().Name()
//使切片太小，无法包含元数据
	serialized := make([]byte, bin.binaryLength()-1)

	if err := bin.binaryPut(serialized); err == nil {
		t.Fatalf("Expected %s.binaryPut to fail, since target slice is too small", name)
	}
}

func testValueSerializer(t *testing.T, v valueSerializer, expected KV) {
	name := reflect.TypeOf(v).Elem().Name()
	kv := make(KV)

	v.AppendValues(kv)
	if !reflect.DeepEqual(expected, kv) {
		expj, _ := json.Marshal(expected)
		gotj, _ := json.Marshal(kv)
		t.Fatalf("Expected %s.AppendValues to return %s, got %s", name, string(expj), string(gotj))
	}

	recovered := reflect.New(reflect.TypeOf(v).Elem()).Interface().(valueSerializer)
	err := recovered.FromValues(kv)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(recovered, v) {
		t.Fatalf("Expected recovered %s to be the same", name)
	}
}
