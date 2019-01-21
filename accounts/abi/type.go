
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
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

//
const (
	IntTy byte = iota
	UintTy
	BoolTy
	StringTy
	SliceTy
	ArrayTy
	TupleTy
	AddressTy
	FixedBytesTy
	BytesTy
	HashTy
	FixedPointTy
	FunctionTy
)

//类型是受支持的参数类型的反射
type Type struct {
	Elem *Type
	Kind reflect.Kind
	Type reflect.Type
	Size int
T    byte //我们自己的类型检查

stringKind string //保留用于派生签名的未分析字符串

//元组相对字段
TupleElems    []*Type  //所有元组字段的类型信息
TupleRawNames []string //所有元组字段的原始字段名
}

var (
//typeregex解析ABI子类型
	typeRegex = regexp.MustCompile("([a-zA-Z]+)(([0-9]+)(x([0-9]+))?)?")
)

//new type创建在t中给定的abi类型的新反射类型。
func NewType(t string, components []ArgumentMarshaling) (typ Type, err error) {
//检查数组括号是否相等（如果存在）
	if strings.Count(t, "[") != strings.Count(t, "]") {
		return Type{}, fmt.Errorf("invalid arg type in abi")
	}

	typ.stringKind = t

//如果有括号，准备进入切片/阵列模式
//递归创建类型
	if strings.Count(t, "[") != 0 {
		i := strings.LastIndex(t, "[")
//递归嵌入类型
		embeddedType, err := NewType(t[:i], components)
		if err != nil {
			return Type{}, err
		}
//抓取最后一个单元格并从中创建类型
		sliced := t[i:]
//用regexp获取切片大小
		re := regexp.MustCompile("[0-9]+")
		intz := re.FindAllString(sliced, -1)

		if len(intz) == 0 {
//是一片
			typ.T = SliceTy
			typ.Kind = reflect.Slice
			typ.Elem = &embeddedType
			typ.Type = reflect.SliceOf(embeddedType.Type)
			if embeddedType.T == TupleTy {
				typ.stringKind = embeddedType.stringKind + sliced
			}
		} else if len(intz) == 1 {
//是一个数组
			typ.T = ArrayTy
			typ.Kind = reflect.Array
			typ.Elem = &embeddedType
			typ.Size, err = strconv.Atoi(intz[0])
			if err != nil {
				return Type{}, fmt.Errorf("abi: error parsing variable size: %v", err)
			}
			typ.Type = reflect.ArrayOf(typ.Size, embeddedType.Type)
			if embeddedType.T == TupleTy {
				typ.stringKind = embeddedType.stringKind + sliced
			}
		} else {
			return Type{}, fmt.Errorf("invalid formatting of array type")
		}
		return typ, err
	}
//分析ABI类型的类型和大小。
	matches := typeRegex.FindAllStringSubmatch(t, -1)
	if len(matches) == 0 {
		return Type{}, fmt.Errorf("invalid type '%v'", t)
	}
	parsedType := matches[0]

//varsize是变量的大小
	var varSize int
	if len(parsedType[3]) > 0 {
		var err error
		varSize, err = strconv.Atoi(parsedType[2])
		if err != nil {
			return Type{}, fmt.Errorf("abi: error parsing variable size: %v", err)
		}
	} else {
		if parsedType[0] == "uint" || parsedType[0] == "int" {
//这应该失败，因为这意味着
//ABI类型（编译器应始终将其格式化为大小…始终）
			return Type{}, fmt.Errorf("unsupported arg type: %s", t)
		}
	}
//vartype是解析的abi类型
	switch varType := parsedType[1]; varType {
	case "int":
		typ.Kind, typ.Type = reflectIntKindAndType(false, varSize)
		typ.Size = varSize
		typ.T = IntTy
	case "uint":
		typ.Kind, typ.Type = reflectIntKindAndType(true, varSize)
		typ.Size = varSize
		typ.T = UintTy
	case "bool":
		typ.Kind = reflect.Bool
		typ.T = BoolTy
		typ.Type = reflect.TypeOf(bool(false))
	case "address":
		typ.Kind = reflect.Array
		typ.Type = addressT
		typ.Size = 20
		typ.T = AddressTy
	case "string":
		typ.Kind = reflect.String
		typ.Type = reflect.TypeOf("")
		typ.T = StringTy
	case "bytes":
		if varSize == 0 {
			typ.T = BytesTy
			typ.Kind = reflect.Slice
			typ.Type = reflect.SliceOf(reflect.TypeOf(byte(0)))
		} else {
			typ.T = FixedBytesTy
			typ.Kind = reflect.Array
			typ.Size = varSize
			typ.Type = reflect.ArrayOf(varSize, reflect.TypeOf(byte(0)))
		}
	case "tuple":
		var (
			fields     []reflect.StructField
			elems      []*Type
			names      []string
expression string //规范参数表达式
		)
		expression += "("
		for idx, c := range components {
			cType, err := NewType(c.Type, c.Components)
			if err != nil {
				return Type{}, err
			}
			if ToCamelCase(c.Name) == "" {
				return Type{}, errors.New("abi: purely anonymous or underscored field is not supported")
			}
			fields = append(fields, reflect.StructField{
Name: ToCamelCase(c.Name), //reflect.structof将恐慌任何导出字段。
				Type: cType.Type,
			})
			elems = append(elems, &cType)
			names = append(names, c.Name)
			expression += cType.stringKind
			if idx != len(components)-1 {
				expression += ","
			}
		}
		expression += ")"
		typ.Kind = reflect.Struct
		typ.Type = reflect.StructOf(fields)
		typ.TupleElems = elems
		typ.TupleRawNames = names
		typ.T = TupleTy
		typ.stringKind = expression
	case "function":
		typ.Kind = reflect.Array
		typ.T = FunctionTy
		typ.Size = 24
		typ.Type = reflect.ArrayOf(24, reflect.TypeOf(byte(0)))
	default:
		return Type{}, fmt.Errorf("unsupported arg type: %s", t)
	}

	return
}

//字符串实现字符串
func (t Type) String() (out string) {
	return t.stringKind
}

func (t Type) pack(v reflect.Value) ([]byte, error) {
//如果指针是指针，则先取消引用指针
	v = indirect(v)
	if err := typeCheck(t, v); err != nil {
		return nil, err
	}

	switch t.T {
	case SliceTy, ArrayTy:
		var ret []byte

		if t.requiresLengthPrefix() {
//追加长度
			ret = append(ret, packNum(reflect.ValueOf(v.Len()))...)
		}

//计算偏移量（如果有）
		offset := 0
		offsetReq := isDynamicType(*t.Elem)
		if offsetReq {
			offset = getTypeSize(*t.Elem) * v.Len()
		}
		var tail []byte
		for i := 0; i < v.Len(); i++ {
			val, err := t.Elem.pack(v.Index(i))
			if err != nil {
				return nil, err
			}
			if !offsetReq {
				ret = append(ret, val...)
				continue
			}
			ret = append(ret, packNum(reflect.ValueOf(offset))...)
			offset += len(val)
			tail = append(tail, val...)
		}
		return append(ret, tail...), nil
	case TupleTy:
//（T1，…，Tk）对于k>=0和任何类型的T1，…，Tk
//Enc（x）=Head（x（1））…头（x（k））尾（x（1））…尾部（x（k））
//其中x=（x（1），…，x（k））和头部和尾部是为ti定义的静态
//类型为
//head（x（i））=enc（x（i）），tail（x（i））=“”（空字符串）
//作为
//头（x（i））=ENC（长度（头（x（1））..头（x（k））尾（x（1））…尾部（X（I-1））
//尾（x（i））=enc（x（i））
//否则，即，如果ti是动态类型。
		fieldmap, err := mapArgNamesToStructFields(t.TupleRawNames, v)
		if err != nil {
			return nil, err
		}
//计算前缀占用的大小。
		offset := 0
		for _, elem := range t.TupleElems {
			offset += getTypeSize(*elem)
		}
		var ret, tail []byte
		for i, elem := range t.TupleElems {
			field := v.FieldByName(fieldmap[t.TupleRawNames[i]])
			if !field.IsValid() {
				return nil, fmt.Errorf("field %s for tuple not found in the given struct", t.TupleRawNames[i])
			}
			val, err := elem.pack(field)
			if err != nil {
				return nil, err
			}
			if isDynamicType(*elem) {
				ret = append(ret, packNum(reflect.ValueOf(offset))...)
				tail = append(tail, val...)
				offset += len(val)
			} else {
				ret = append(ret, val...)
			}
		}
		return append(ret, tail...), nil

	default:
		return packElement(t, v), nil
	}
}

//RequireLengthPrefix返回类型是否需要任何长度
//前缀。
func (t Type) requiresLengthPrefix() bool {
	return t.T == StringTy || t.T == BytesTy || t.T == SliceTy
}

//如果类型是动态的，is dynamic type返回true。
//以下类型称为“动态”类型：
//＊字节
//＊字符串
//＊t[]用于任何t
//*t[k]对于任何动态t和任何k>=0
//*（T1，…，Tk）如果Ti是动态的，则1<=i<=k
func isDynamicType(t Type) bool {
	if t.T == TupleTy {
		for _, elem := range t.TupleElems {
			if isDynamicType(*elem) {
				return true
			}
		}
		return false
	}
	return t.T == StringTy || t.T == BytesTy || t.T == SliceTy || (t.T == ArrayTy && isDynamicType(*t.Elem))
}

//GettypeSize返回此类型需要占用的大小。
//我们区分静态和动态类型。静态类型就地编码
//动态类型在
//当前块。
//因此对于静态变量，返回的大小表示
//变量实际占用。
//对于动态变量，返回的大小是固定的32字节，使用
//存储实际值存储的位置引用。
func getTypeSize(t Type) int {
	if t.T == ArrayTy && !isDynamicType(*t.Elem) {
//如果是嵌套数组，则递归计算类型大小
		if t.Elem.T == ArrayTy {
			return t.Size * getTypeSize(*t.Elem)
		}
		return t.Size * 32
	} else if t.T == TupleTy && !isDynamicType(t) {
		total := 0
		for _, elem := range t.TupleElems {
			total += getTypeSize(*elem)
		}
		return total
	}
	return 32
}
