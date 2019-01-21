
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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

//参数包含参数的名称和相应的类型。
//类型在打包和测试参数时使用。
type Argument struct {
	Name    string
	Type    Type
Indexed bool //索引仅由事件使用
}

type Arguments []Argument

type ArgumentMarshaling struct {
	Name       string
	Type       string
	Components []ArgumentMarshaling
	Indexed    bool
}

//unmashaljson实现json.unmasheler接口
func (argument *Argument) UnmarshalJSON(data []byte) error {
	var arg ArgumentMarshaling
	err := json.Unmarshal(data, &arg)
	if err != nil {
		return fmt.Errorf("argument json err: %v", err)
	}

	argument.Type, err = NewType(arg.Type, arg.Components)
	if err != nil {
		return err
	}
	argument.Name = arg.Name
	argument.Indexed = arg.Indexed

	return nil
}

//lengthnonindexed返回不计算“indexed”参数时的参数数。只有事件
//不能有“indexed”参数，方法输入/输出的参数应始终为false
func (arguments Arguments) LengthNonIndexed() int {
	out := 0
	for _, arg := range arguments {
		if !arg.Indexed {
			out++
		}
	}
	return out
}

//无索引返回已筛选出索引参数的参数
func (arguments Arguments) NonIndexed() Arguments {
	var ret []Argument
	for _, arg := range arguments {
		if !arg.Indexed {
			ret = append(ret, arg)
		}
	}
	return ret
}

//Istuple为非原子结构返回true，如（uint、uint）或uint[]
func (arguments Arguments) isTuple() bool {
	return len(arguments) > 1
}

//unpack执行hexdata->go-format操作
func (arguments Arguments) Unpack(v interface{}, data []byte) error {
//确保传递的值是参数指针
	if reflect.Ptr != reflect.ValueOf(v).Kind() {
		return fmt.Errorf("abi: Unpack(non-pointer %T)", v)
	}
	marshalledValues, err := arguments.UnpackValues(data)
	if err != nil {
		return err
	}
	if arguments.isTuple() {
		return arguments.unpackTuple(v, marshalledValues)
	}
	return arguments.unpackAtomic(v, marshalledValues[0])
}

//unpack将未标记的值设置为go格式。
//注意这里的DST必须是可设置的。
func unpack(t *Type, dst interface{}, src interface{}) error {
	var (
		dstVal = reflect.ValueOf(dst).Elem()
		srcVal = reflect.ValueOf(src)
	)

	if t.T != TupleTy && !((t.T == SliceTy || t.T == ArrayTy) && t.Elem.T == TupleTy) {
		return set(dstVal, srcVal)
	}

	switch t.T {
	case TupleTy:
		if dstVal.Kind() != reflect.Struct {
			return fmt.Errorf("abi: invalid dst value for unpack, want struct, got %s", dstVal.Kind())
		}
		fieldmap, err := mapArgNamesToStructFields(t.TupleRawNames, dstVal)
		if err != nil {
			return err
		}
		for i, elem := range t.TupleElems {
			fname := fieldmap[t.TupleRawNames[i]]
			field := dstVal.FieldByName(fname)
			if !field.IsValid() {
				return fmt.Errorf("abi: field %s can't found in the given value", t.TupleRawNames[i])
			}
			if err := unpack(elem, field.Addr().Interface(), srcVal.Field(i).Interface()); err != nil {
				return err
			}
		}
		return nil
	case SliceTy:
		if dstVal.Kind() != reflect.Slice {
			return fmt.Errorf("abi: invalid dst value for unpack, want slice, got %s", dstVal.Kind())
		}
		slice := reflect.MakeSlice(dstVal.Type(), srcVal.Len(), srcVal.Len())
		for i := 0; i < slice.Len(); i++ {
			if err := unpack(t.Elem, slice.Index(i).Addr().Interface(), srcVal.Index(i).Interface()); err != nil {
				return err
			}
		}
		dstVal.Set(slice)
	case ArrayTy:
		if dstVal.Kind() != reflect.Array {
			return fmt.Errorf("abi: invalid dst value for unpack, want array, got %s", dstVal.Kind())
		}
		array := reflect.New(dstVal.Type()).Elem()
		for i := 0; i < array.Len(); i++ {
			if err := unpack(t.Elem, array.Index(i).Addr().Interface(), srcVal.Index(i).Interface()); err != nil {
				return err
			}
		}
		dstVal.Set(array)
	}
	return nil
}

//解包原子解包（hexdata->go）单个值
func (arguments Arguments) unpackAtomic(v interface{}, marshalledValues interface{}) error {
	if arguments.LengthNonIndexed() == 0 {
		return nil
	}
	argument := arguments.NonIndexed()[0]
	elem := reflect.ValueOf(v).Elem()

	if elem.Kind() == reflect.Struct {
		fieldmap, err := mapArgNamesToStructFields([]string{argument.Name}, elem)
		if err != nil {
			return err
		}
		field := elem.FieldByName(fieldmap[argument.Name])
		if !field.IsValid() {
			return fmt.Errorf("abi: field %s can't be found in the given value", argument.Name)
		}
		return unpack(&argument.Type, field.Addr().Interface(), marshalledValues)
	}
	return unpack(&argument.Type, elem.Addr().Interface(), marshalledValues)
}

//解包解包（hexdata->go）一批值。
func (arguments Arguments) unpackTuple(v interface{}, marshalledValues []interface{}) error {
	var (
		value = reflect.ValueOf(v).Elem()
		typ   = value.Type()
		kind  = value.Kind()
	)
	if err := requireUnpackKind(value, typ, kind, arguments); err != nil {
		return err
	}

//如果接口是结构，则获取abi->struct_字段映射
	var abi2struct map[string]string
	if kind == reflect.Struct {
		var (
			argNames []string
			err      error
		)
		for _, arg := range arguments.NonIndexed() {
			argNames = append(argNames, arg.Name)
		}
		abi2struct, err = mapArgNamesToStructFields(argNames, value)
		if err != nil {
			return err
		}
	}
	for i, arg := range arguments.NonIndexed() {
		switch kind {
		case reflect.Struct:
			field := value.FieldByName(abi2struct[arg.Name])
			if !field.IsValid() {
				return fmt.Errorf("abi: field %s can't be found in the given value", arg.Name)
			}
			if err := unpack(&arg.Type, field.Addr().Interface(), marshalledValues[i]); err != nil {
				return err
			}
		case reflect.Slice, reflect.Array:
			if value.Len() < i {
				return fmt.Errorf("abi: insufficient number of arguments for unpack, want %d, got %d", len(arguments), value.Len())
			}
			v := value.Index(i)
			if err := requireAssignable(v, reflect.ValueOf(marshalledValues[i])); err != nil {
				return err
			}
			if err := unpack(&arg.Type, v.Addr().Interface(), marshalledValues[i]); err != nil {
				return err
			}
		default:
			return fmt.Errorf("abi:[2] cannot unmarshal tuple in to %v", typ)
		}
	}
	return nil

}

//解包值可用于根据ABI规范解包ABI编码的十六进制数据。
//不提供要解包的结构。相反，此方法返回一个包含
//价值观。原子参数将是一个包含一个元素的列表。
func (arguments Arguments) UnpackValues(data []byte) ([]interface{}, error) {
	retval := make([]interface{}, 0, arguments.LengthNonIndexed())
	virtualArgs := 0
	for index, arg := range arguments.NonIndexed() {
		marshalledValue, err := toGoType((index+virtualArgs)*32, arg.Type, data)
		if arg.Type.T == ArrayTy && !isDynamicType(arg.Type) {
//如果我们有一个静态数组，比如[3]uint256，那么这些数组被编码为
//就像uint256、uint256、uint256一样。
//这意味着当
//我们从现在开始计算索引。
//
//嵌套多层深度的数组值也以内联方式编码：
//[2][3]uint256:uint256、uint256、uint256、uint256、uint256、uint256、uint256
//
//计算完整的数组大小以获得下一个参数的正确偏移量。
//将其减小1，因为仍应用正常索引增量。
			virtualArgs += getTypeSize(arg.Type)/32 - 1
		} else if arg.Type.T == TupleTy && !isDynamicType(arg.Type) {
//如果我们有一个静态元组，比如（uint256、bool、uint256），这些是
//代码与uint256、bool、uint256相同
			virtualArgs += getTypeSize(arg.Type)/32 - 1
		}
		if err != nil {
			return nil, err
		}
		retval = append(retval, marshalledValue)
	}
	return retval, nil
}

//packvalues执行操作go format->hexdata
//它在语义上与unpackvalues相反
func (arguments Arguments) PackValues(args []interface{}) ([]byte, error) {
	return arguments.Pack(args...)
}

//pack执行操作go format->hexdata
func (arguments Arguments) Pack(args ...interface{}) ([]byte, error) {
//确保参数匹配并打包
	abiArgs := arguments
	if len(args) != len(abiArgs) {
		return nil, fmt.Errorf("argument count mismatch: %d for %d", len(args), len(abiArgs))
	}
//变量输入是在压缩结束时附加的输出
//输出。这用于字符串和字节类型输入。
	var variableInput []byte

//输入偏移量是压缩输出的字节偏移量
	inputOffset := 0
	for _, abiArg := range abiArgs {
		inputOffset += getTypeSize(abiArg.Type)
	}
	var ret []byte
	for i, a := range args {
		input := abiArgs[i]
//打包输入
		packed, err := input.Type.pack(reflect.ValueOf(a))
		if err != nil {
			return nil, err
		}
//检查动态类型
		if isDynamicType(input.Type) {
//设置偏移量
			ret = append(ret, packNum(reflect.ValueOf(inputOffset))...)
//计算下一个偏移量
			inputOffset += len(packed)
//附加到变量输入
			variableInput = append(variableInput, packed...)
		} else {
//将压缩值追加到输入
			ret = append(ret, packed...)
		}
	}
//在压缩输入的末尾附加变量输入
	ret = append(ret, variableInput...)

	return ret, nil
}

//to camel case将欠分数字符串转换为驼色大小写字符串
func ToCamelCase(input string) string {
	parts := strings.Split(input, "_")
	for i, s := range parts {
		if len(s) > 0 {
			parts[i] = strings.ToUpper(s[:1]) + s[1:]
		}
	}
	return strings.Join(parts, "")
}
