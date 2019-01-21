
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

package abi

import (
	"fmt"
	"reflect"
	"strings"
)

//间接递归地取消对值的引用，直到它得到值为止
//或者找到一个大的.int
func indirect(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Ptr && v.Elem().Type() != derefbigT {
		return indirect(v.Elem())
	}
	return v
}

//ReflectIntKind返回使用给定大小和
//不拘一格
func reflectIntKindAndType(unsigned bool, size int) (reflect.Kind, reflect.Type) {
	switch size {
	case 8:
		if unsigned {
			return reflect.Uint8, uint8T
		}
		return reflect.Int8, int8T
	case 16:
		if unsigned {
			return reflect.Uint16, uint16T
		}
		return reflect.Int16, int16T
	case 32:
		if unsigned {
			return reflect.Uint32, uint32T
		}
		return reflect.Int32, int32T
	case 64:
		if unsigned {
			return reflect.Uint64, uint64T
		}
		return reflect.Int64, int64T
	}
	return reflect.Ptr, bigT
}

//mustArrayToBytesSlice创建与值大小完全相同的新字节片
//并将值中的字节复制到新切片。
func mustArrayToByteSlice(value reflect.Value) reflect.Value {
	slice := reflect.MakeSlice(reflect.TypeOf([]byte{}), value.Len(), value.Len())
	reflect.Copy(slice, value)
	return slice
}

//设置尝试通过设置、复制或其他方式将SRC分配给DST。
//
//当涉及到任务时，set要宽松一点，而不是强制
//严格的规则集为bare`reflect`的规则集。
func set(dst, src reflect.Value) error {
	dstType, srcType := dst.Type(), src.Type()
	switch {
	case dstType.Kind() == reflect.Interface:
		return set(dst.Elem(), src)
	case dstType.Kind() == reflect.Ptr && dstType.Elem() != derefbigT:
		return set(dst.Elem(), src)
	case srcType.AssignableTo(dstType) && dst.CanSet():
		dst.Set(src)
	case dstType.Kind() == reflect.Slice && srcType.Kind() == reflect.Slice:
		return setSlice(dst, src)
	default:
		return fmt.Errorf("abi: cannot unmarshal %v in to %v", src.Type(), dst.Type())
	}
	return nil
}

//当切片在默认情况下不可分配时，setslice尝试将src分配给dst
//例如，SRC:[]字节->DST:[[15]字节
func setSlice(dst, src reflect.Value) error {
	slice := reflect.MakeSlice(dst.Type(), src.Len(), src.Len())
	for i := 0; i < src.Len(); i++ {
		v := src.Index(i)
		reflect.Copy(slice.Index(i), v)
	}

	dst.Set(slice)
	return nil
}

//RequiresSignable确保“dest”是指针，而不是接口。
func requireAssignable(dst, src reflect.Value) error {
	if dst.Kind() != reflect.Ptr && dst.Kind() != reflect.Interface {
		return fmt.Errorf("abi: cannot unmarshal %v into %v", src.Type(), dst.Type())
	}
	return nil
}

//RequireUnpackKind验证将“args”解包为“kind”的前提条件
func requireUnpackKind(v reflect.Value, t reflect.Type, k reflect.Kind,
	args Arguments) error {

	switch k {
	case reflect.Struct:
	case reflect.Slice, reflect.Array:
		if minLen := args.LengthNonIndexed(); v.Len() < minLen {
			return fmt.Errorf("abi: insufficient number of elements in the list/array for unpack, want %d, got %d",
				minLen, v.Len())
		}
	default:
		return fmt.Errorf("abi: cannot unmarshal tuple into %v", t)
	}
	return nil
}

//mapargnamestostructfields将参数名的一部分映射到结构字段。
//第一轮：对于每个包含“abi:”标记的可导出字段
//这个字段名存在于给定的参数名列表中，将它们配对在一起。
//第二轮：对于每个尚未链接的参数名，
//如果变量存在且尚未映射，则查找该变量应映射到的对象。
//用过，配对。
//注意：此函数假定给定值是结构值。
func mapArgNamesToStructFields(argNames []string, value reflect.Value) (map[string]string, error) {
	typ := value.Type()

	abi2struct := make(map[string]string)
	struct2abi := make(map[string]string)

//第一轮~~~
	for i := 0; i < typ.NumField(); i++ {
		structFieldName := typ.Field(i).Name

//跳过私有结构字段。
		if structFieldName[:1] != strings.ToUpper(structFieldName[:1]) {
			continue
		}
//跳过没有abi:“”标记的字段。
		var ok bool
		var tagName string
		if tagName, ok = typ.Field(i).Tag.Lookup("abi"); !ok {
			continue
		}
//检查标签是否为空。
		if tagName == "" {
			return nil, fmt.Errorf("struct: abi tag in '%s' is empty", structFieldName)
		}
//检查哪个参数字段与ABI标记匹配。
		found := false
		for _, arg := range argNames {
			if arg == tagName {
				if abi2struct[arg] != "" {
					return nil, fmt.Errorf("struct: abi tag in '%s' already mapped", structFieldName)
				}
//配对他们
				abi2struct[arg] = structFieldName
				struct2abi[structFieldName] = arg
				found = true
			}
		}
//检查此标记是否已映射。
		if !found {
			return nil, fmt.Errorf("struct: abi tag '%s' defined but not found in abi", tagName)
		}
	}

//第二轮~~~
	for _, argName := range argNames {

		structFieldName := ToCamelCase(argName)

		if structFieldName == "" {
			return nil, fmt.Errorf("abi: purely underscored output cannot unpack to struct")
		}

//这个ABI已经配对了，跳过它…除非存在另一个尚未分配的
//具有相同字段名的结构字段。如果是，则引发错误：
//ABI：[“name”：“value”]
//结构value*big.int，value1*big.int`abi:“value”`
		if abi2struct[argName] != "" {
			if abi2struct[argName] != structFieldName &&
				struct2abi[structFieldName] == "" &&
				value.FieldByName(structFieldName).IsValid() {
				return nil, fmt.Errorf("abi: multiple variables maps to the same abi field '%s'", argName)
			}
			continue
		}

//如果此结构字段已配对，则返回错误。
		if struct2abi[structFieldName] != "" {
			return nil, fmt.Errorf("abi: multiple outputs mapping to the same struct field '%s'", structFieldName)
		}

		if value.FieldByName(structFieldName).IsValid() {
//配对他们
			abi2struct[argName] = structFieldName
			struct2abi[structFieldName] = argName
		} else {
//不是成对的，而是按使用进行注释，以检测
//ABI：[“name”：“value”，“name”：“value”]
//结构值*big.int
			struct2abi[structFieldName] = argName
		}
	}
	return abi2struct, nil
}
