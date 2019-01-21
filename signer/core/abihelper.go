
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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"bytes"
	"os"
	"regexp"
)

type decodedArgument struct {
	soltype abi.Argument
	value   interface{}
}
type decodedCallData struct {
	signature string
	name      string
	inputs    []decodedArgument
}

//字符串实现Stringer接口，尝试使用基础值类型
func (arg decodedArgument) String() string {
	var value string
	switch val := arg.value.(type) {
	case fmt.Stringer:
		value = val.String()
	default:
		value = fmt.Sprintf("%v", val)
	}
	return fmt.Sprintf("%v: %v", arg.soltype.Type.String(), value)
}

//字符串实现用于decodedcalldata的字符串接口
func (cd decodedCallData) String() string {
	args := make([]string, len(cd.inputs))
	for i, arg := range cd.inputs {
		args[i] = arg.String()
	}
	return fmt.Sprintf("%s(%s)", cd.name, strings.Join(args, ","))
}

//ParseCallData将提供的调用数据与ABI定义匹配，
//并返回包含实际go类型值的结构
func parseCallData(calldata []byte, abidata string) (*decodedCallData, error) {

	if len(calldata) < 4 {
		return nil, fmt.Errorf("Invalid ABI-data, incomplete method signature of (%d bytes)", len(calldata))
	}

	sigdata, argdata := calldata[:4], calldata[4:]
	if len(argdata)%32 != 0 {
		return nil, fmt.Errorf("Not ABI-encoded data; length should be a multiple of 32 (was %d)", len(argdata))
	}

	abispec, err := abi.JSON(strings.NewReader(abidata))
	if err != nil {
		return nil, fmt.Errorf("Failed parsing JSON ABI: %v, abidata: %v", err, abidata)
	}

	method, err := abispec.MethodById(sigdata)
	if err != nil {
		return nil, err
	}

	v, err := method.Inputs.UnpackValues(argdata)
	if err != nil {
		return nil, err
	}

	decoded := decodedCallData{signature: method.Sig(), name: method.Name}

	for n, argument := range method.Inputs {
		if err != nil {
			return nil, fmt.Errorf("Failed to decode argument %d (signature %v): %v", n, method.Sig(), err)
		}
		decodedArg := decodedArgument{
			soltype: argument,
			value:   v[n],
		}
		decoded.inputs = append(decoded.inputs, decodedArg)
	}

//数据解码完毕。此时，我们对解码后的数据进行编码，以查看它是否与
//原始数据。如果我们不这样做，就可以在参数中填充额外的数据，例如
//仅通过解码数据检测不到。

	var (
		encoded []byte
	)
	encoded, err = method.Inputs.PackValues(v)

	if err != nil {
		return nil, err
	}

	if !bytes.Equal(encoded, argdata) {
		was := common.Bytes2Hex(encoded)
		exp := common.Bytes2Hex(argdata)
		return nil, fmt.Errorf("WARNING: Supplied data is stuffed with extra data. \nWant %s\nHave %s\nfor method %v", exp, was, method.Sig())
	}
	return &decoded, nil
}

//methodSelectorToAbi将方法选择器转换为ABI结构。返回的数据是有效的JSON字符串
//可由标准ABI包装使用。
func MethodSelectorToAbi(selector string) ([]byte, error) {

	re := regexp.MustCompile(`^([^\)]+)\(([a-z0-9,\[\]]*)\)`)

	type fakeArg struct {
		Type string `json:"type"`
	}
	type fakeABI struct {
		Name   string    `json:"name"`
		Type   string    `json:"type"`
		Inputs []fakeArg `json:"inputs"`
	}
	groups := re.FindStringSubmatch(selector)
	if len(groups) != 3 {
		return nil, fmt.Errorf("Did not match: %v (%v matches)", selector, len(groups))
	}
	name := groups[1]
	args := groups[2]
	arguments := make([]fakeArg, 0)
	if len(args) > 0 {
		for _, arg := range strings.Split(args, ",") {
			arguments = append(arguments, fakeArg{arg})
		}
	}
	abicheat := fakeABI{
		name, "function", arguments,
	}
	return json.Marshal([]fakeABI{abicheat})

}

type AbiDb struct {
	db           map[string]string
	customdb     map[string]string
	customdbPath string
}

//出于测试目的，存在newEmptyBidb
func NewEmptyAbiDB() (*AbiDb, error) {
	return &AbiDb{make(map[string]string), make(map[string]string), ""}, nil
}

//newabidbfrmfile从文件加载签名数据库，以及
//如果文件不是有效的JSON，则会出错。没有其他内容验证
func NewAbiDBFromFile(path string) (*AbiDb, error) {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	db, err := NewEmptyAbiDB()
	if err != nil {
		return nil, err
	}
	json.Unmarshal(raw, &db.db)
	return db, nil
}

//newabidbfrmfiles同时加载标准签名数据库和自定义数据库。后者将被使用
//如果通过API提交新值，则将其写入
func NewAbiDBFromFiles(standard, custom string) (*AbiDb, error) {

	db := &AbiDb{make(map[string]string), make(map[string]string), custom}
	db.customdbPath = custom

	raw, err := ioutil.ReadFile(standard)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(raw, &db.db)
//自定义文件可能不存在。如果需要，将在保存期间创建
	if _, err := os.Stat(custom); err == nil {
		raw, err = ioutil.ReadFile(custom)
		if err != nil {
			return nil, err
		}
		json.Unmarshal(raw, &db.customdb)
	}

	return db, nil
}

//LookupMethodSelector对照已知的ABI方法检查给定的4字节序列。
//obs：此方法不验证匹配，假定调用方将验证匹配
func (db *AbiDb) LookupMethodSelector(id []byte) (string, error) {
	if len(id) < 4 {
		return "", fmt.Errorf("Expected 4-byte id, got %d", len(id))
	}
	sig := common.ToHex(id[:4])
	if key, exists := db.db[sig]; exists {
		return key, nil
	}
	if key, exists := db.customdb[sig]; exists {
		return key, nil
	}
	return "", fmt.Errorf("Signature %v not found", sig)
}
func (db *AbiDb) Size() int {
	return len(db.db)
}

//savecustomabi临时保存签名。如果使用自定义文件，也将保存到磁盘
func (db *AbiDb) saveCustomAbi(selector, signature string) error {
	db.customdb[signature] = selector
	if db.customdbPath == "" {
return nil //本身不是错误，只是没有使用
	}
	d, err := json.Marshal(db.customdb)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(db.customdbPath, d, 0600)
	return err
}

//如果启用了自定义数据库保存，则向数据库添加签名。
//OBS：这种方法不能验证数据的正确性，
//假定呼叫者已经这样做了
func (db *AbiDb) AddSignature(selector string, data []byte) error {
	if len(data) < 4 {
		return nil
	}
	_, err := db.LookupMethodSelector(data[:4])
	if err == nil {
		return nil
	}
	sig := common.ToHex(data[:4])
	return db.saveCustomAbi(selector, sig)
}
