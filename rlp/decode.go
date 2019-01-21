
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
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

package rlp

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strings"
)

var (
//当前列表结束时返回EOL
//已在流式处理期间到达。
	EOL = errors.New("rlp: end of list")

//实际误差
	ErrExpectedString   = errors.New("rlp: expected String or Byte")
	ErrExpectedList     = errors.New("rlp: expected List")
	ErrCanonInt         = errors.New("rlp: non-canonical integer format")
	ErrCanonSize        = errors.New("rlp: non-canonical size information")
	ErrElemTooLarge     = errors.New("rlp: element is larger than containing list")
	ErrValueTooLarge    = errors.New("rlp: value size exceeds available input length")
	ErrMoreThanOneValue = errors.New("rlp: input contains more than one value")

//内部错误
	errNotInList     = errors.New("rlp: call of ListEnd outside of any list")
	errNotAtEOL      = errors.New("rlp: call of ListEnd not positioned at EOL")
	errUintOverflow  = errors.New("rlp: uint overflow")
	errNoPointer     = errors.New("rlp: interface given to Decode must be a pointer")
	errDecodeIntoNil = errors.New("rlp: pointer given to Decode must not be nil")
)

//解码器由需要自定义rlp的类型实现
//解码规则或需要解码为私有字段。
//
//decoderlp方法应从给定的
//溪流。不禁止少读或多读，但可能
//令人困惑。
type Decoder interface {
	DecodeRLP(*Stream) error
}

//解码解析r中的rlp编码数据，并将结果存储在
//val.val指向的值必须是非零指针。如果R确实如此
//不实现bytereader，decode将自己进行缓冲。
//
//解码使用以下与类型相关的解码规则：
//
//如果类型实现解码器接口，则解码调用
//DecodeRLP。
//
//要解码为指针，解码将解码为指向的值
//去。如果指针为零，则指针元素的新值
//类型已分配。如果指针为非零，则为现有值
//将被重复使用。
//
//要解码为结构，decode要求输入为rlp
//名单。列表的解码元素被分配给每个公共
//字段的顺序由结构定义给出。输入表
//必须为每个解码字段包含一个元素。decode返回
//元素太少或太多时出错。
//
//对结构字段的解码将授予某些结构标记“tail”，
//“零”和“-”。
//
//“-”标记忽略字段。
//
//有关“tail”的解释，请参见示例。
//
//“nil”标记应用于指针类型的字段并更改解码
//字段的规则，使大小为零的输入值解码为零
//指针。此标记在解码递归类型时很有用。
//
//类型结构WithEmptyok结构
//foo*[20]字节'rlp:“nil”`
//}
//
//要解码成一个切片，输入必须是一个列表和结果
//slice将按顺序包含输入元素。对于字节片，
//输入必须是RLP字符串。数组类型解码类似，使用
//输入元素数量（或
//字节）必须与数组的长度匹配。
//
//要解码为go字符串，输入必须是rlp字符串。这个
//输入字节按原样处理，不一定是有效的UTF-8。
//
//要解码为无符号整数类型，输入还必须是RLP
//字符串。字节被解释为
//整数。如果RLP字符串大于
//类型，decode将返回错误。decode还支持*big.int。
//大整数没有大小限制。
//
//要解码为接口值，decode将存储其中一个值
//价值：
//
//[]接口，用于RLP列表
//[]字节，用于RLP字符串
//
//不支持非空接口类型，布尔值也不支持，
//有符号整数、浮点数、映射、通道和
//功能。
//
//请注意，decode不为所有读卡器设置输入限制
//而且可能容易受到巨大价值规模导致的恐慌。如果
//您需要输入限制，使用
//
//新闻流（R，限制）。解码（VAL）
func Decode(r io.Reader, val interface{}) error {
//TODO:这可能使用来自池的流。
	return NewStream(r, 0).Decode(val)
}

//decodebytes将rlp数据从b解析为val。
//解码规则见解码文档。
//输入必须正好包含一个值，并且没有尾随数据。
func DecodeBytes(b []byte, val interface{}) error {
//TODO:这可能使用来自池的流。
	r := bytes.NewReader(b)
	if err := NewStream(r, uint64(len(b))).Decode(val); err != nil {
		return err
	}
	if r.Len() > 0 {
		return ErrMoreThanOneValue
	}
	return nil
}

type decodeError struct {
	msg string
	typ reflect.Type
	ctx []string
}

func (err *decodeError) Error() string {
	ctx := ""
	if len(err.ctx) > 0 {
		ctx = ", decoding into "
		for i := len(err.ctx) - 1; i >= 0; i-- {
			ctx += err.ctx[i]
		}
	}
	return fmt.Sprintf("rlp: %s for %v%s", err.msg, err.typ, ctx)
}

func wrapStreamError(err error, typ reflect.Type) error {
	switch err {
	case ErrCanonInt:
		return &decodeError{msg: "non-canonical integer (leading zero bytes)", typ: typ}
	case ErrCanonSize:
		return &decodeError{msg: "non-canonical size information", typ: typ}
	case ErrExpectedList:
		return &decodeError{msg: "expected input list", typ: typ}
	case ErrExpectedString:
		return &decodeError{msg: "expected input string or byte", typ: typ}
	case errUintOverflow:
		return &decodeError{msg: "input string too long", typ: typ}
	case errNotAtEOL:
		return &decodeError{msg: "input list has too many elements", typ: typ}
	}
	return err
}

func addErrorContext(err error, ctx string) error {
	if decErr, ok := err.(*decodeError); ok {
		decErr.ctx = append(decErr.ctx, ctx)
	}
	return err
}

var (
	decoderInterface = reflect.TypeOf(new(Decoder)).Elem()
	bigInt           = reflect.TypeOf(big.Int{})
)

func makeDecoder(typ reflect.Type, tags tags) (dec decoder, err error) {
	kind := typ.Kind()
	switch {
	case typ == rawValueType:
		return decodeRawValue, nil
	case typ.Implements(decoderInterface):
		return decodeDecoder, nil
	case kind != reflect.Ptr && reflect.PtrTo(typ).Implements(decoderInterface):
		return decodeDecoderNoPtr, nil
	case typ.AssignableTo(reflect.PtrTo(bigInt)):
		return decodeBigInt, nil
	case typ.AssignableTo(bigInt):
		return decodeBigIntNoPtr, nil
	case isUint(kind):
		return decodeUint, nil
	case kind == reflect.Bool:
		return decodeBool, nil
	case kind == reflect.String:
		return decodeString, nil
	case kind == reflect.Slice || kind == reflect.Array:
		return makeListDecoder(typ, tags)
	case kind == reflect.Struct:
		return makeStructDecoder(typ)
	case kind == reflect.Ptr:
		if tags.nilOK {
			return makeOptionalPtrDecoder(typ)
		}
		return makePtrDecoder(typ)
	case kind == reflect.Interface:
		return decodeInterface, nil
	default:
		return nil, fmt.Errorf("rlp: type %v is not RLP-serializable", typ)
	}
}

func decodeRawValue(s *Stream, val reflect.Value) error {
	r, err := s.Raw()
	if err != nil {
		return err
	}
	val.SetBytes(r)
	return nil
}

func decodeUint(s *Stream, val reflect.Value) error {
	typ := val.Type()
	num, err := s.uint(typ.Bits())
	if err != nil {
		return wrapStreamError(err, val.Type())
	}
	val.SetUint(num)
	return nil
}

func decodeBool(s *Stream, val reflect.Value) error {
	b, err := s.Bool()
	if err != nil {
		return wrapStreamError(err, val.Type())
	}
	val.SetBool(b)
	return nil
}

func decodeString(s *Stream, val reflect.Value) error {
	b, err := s.Bytes()
	if err != nil {
		return wrapStreamError(err, val.Type())
	}
	val.SetString(string(b))
	return nil
}

func decodeBigIntNoPtr(s *Stream, val reflect.Value) error {
	return decodeBigInt(s, val.Addr())
}

func decodeBigInt(s *Stream, val reflect.Value) error {
	b, err := s.Bytes()
	if err != nil {
		return wrapStreamError(err, val.Type())
	}
	i := val.Interface().(*big.Int)
	if i == nil {
		i = new(big.Int)
		val.Set(reflect.ValueOf(i))
	}
//拒绝前导零字节
	if len(b) > 0 && b[0] == 0 {
		return wrapStreamError(ErrCanonInt, val.Type())
	}
	i.SetBytes(b)
	return nil
}

func makeListDecoder(typ reflect.Type, tag tags) (decoder, error) {
	etype := typ.Elem()
	if etype.Kind() == reflect.Uint8 && !reflect.PtrTo(etype).Implements(decoderInterface) {
		if typ.Kind() == reflect.Array {
			return decodeByteArray, nil
		}
		return decodeByteSlice, nil
	}
	etypeinfo, err := cachedTypeInfo1(etype, tags{})
	if err != nil {
		return nil, err
	}
	var dec decoder
	switch {
	case typ.Kind() == reflect.Array:
		dec = func(s *Stream, val reflect.Value) error {
			return decodeListArray(s, val, etypeinfo.decoder)
		}
	case tag.tail:
//带有“tail”标记的切片可以作为最后一个字段出现
//一个结构的，应该吞下所有剩余的
//列出元素。结构解码器已经调用了s.list，
//直接解码元素。
		dec = func(s *Stream, val reflect.Value) error {
			return decodeSliceElems(s, val, etypeinfo.decoder)
		}
	default:
		dec = func(s *Stream, val reflect.Value) error {
			return decodeListSlice(s, val, etypeinfo.decoder)
		}
	}
	return dec, nil
}

func decodeListSlice(s *Stream, val reflect.Value, elemdec decoder) error {
	size, err := s.List()
	if err != nil {
		return wrapStreamError(err, val.Type())
	}
	if size == 0 {
		val.Set(reflect.MakeSlice(val.Type(), 0, 0))
		return s.ListEnd()
	}
	if err := decodeSliceElems(s, val, elemdec); err != nil {
		return err
	}
	return s.ListEnd()
}

func decodeSliceElems(s *Stream, val reflect.Value, elemdec decoder) error {
	i := 0
	for ; ; i++ {
//必要时增加切片
		if i >= val.Cap() {
			newcap := val.Cap() + val.Cap()/2
			if newcap < 4 {
				newcap = 4
			}
			newv := reflect.MakeSlice(val.Type(), val.Len(), newcap)
			reflect.Copy(newv, val)
			val.Set(newv)
		}
		if i >= val.Len() {
			val.SetLen(i + 1)
		}
//解码为元素
		if err := elemdec(s, val.Index(i)); err == EOL {
			break
		} else if err != nil {
			return addErrorContext(err, fmt.Sprint("[", i, "]"))
		}
	}
	if i < val.Len() {
		val.SetLen(i)
	}
	return nil
}

func decodeListArray(s *Stream, val reflect.Value, elemdec decoder) error {
	if _, err := s.List(); err != nil {
		return wrapStreamError(err, val.Type())
	}
	vlen := val.Len()
	i := 0
	for ; i < vlen; i++ {
		if err := elemdec(s, val.Index(i)); err == EOL {
			break
		} else if err != nil {
			return addErrorContext(err, fmt.Sprint("[", i, "]"))
		}
	}
	if i < vlen {
		return &decodeError{msg: "input list has too few elements", typ: val.Type()}
	}
	return wrapStreamError(s.ListEnd(), val.Type())
}

func decodeByteSlice(s *Stream, val reflect.Value) error {
	b, err := s.Bytes()
	if err != nil {
		return wrapStreamError(err, val.Type())
	}
	val.SetBytes(b)
	return nil
}

func decodeByteArray(s *Stream, val reflect.Value) error {
	kind, size, err := s.Kind()
	if err != nil {
		return err
	}
	vlen := val.Len()
	switch kind {
	case Byte:
		if vlen == 0 {
			return &decodeError{msg: "input string too long", typ: val.Type()}
		}
		if vlen > 1 {
			return &decodeError{msg: "input string too short", typ: val.Type()}
		}
		bv, _ := s.Uint()
		val.Index(0).SetUint(bv)
	case String:
		if uint64(vlen) < size {
			return &decodeError{msg: "input string too long", typ: val.Type()}
		}
		if uint64(vlen) > size {
			return &decodeError{msg: "input string too short", typ: val.Type()}
		}
		slice := val.Slice(0, vlen).Interface().([]byte)
		if err := s.readFull(slice); err != nil {
			return err
		}
//拒绝应该使用单字节编码的情况。
		if size == 1 && slice[0] < 128 {
			return wrapStreamError(ErrCanonSize, val.Type())
		}
	case List:
		return wrapStreamError(ErrExpectedString, val.Type())
	}
	return nil
}

func makeStructDecoder(typ reflect.Type) (decoder, error) {
	fields, err := structFields(typ)
	if err != nil {
		return nil, err
	}
	dec := func(s *Stream, val reflect.Value) (err error) {
		if _, err := s.List(); err != nil {
			return wrapStreamError(err, typ)
		}
		for _, f := range fields {
			err := f.info.decoder(s, val.Field(f.index))
			if err == EOL {
				return &decodeError{msg: "too few elements", typ: typ}
			} else if err != nil {
				return addErrorContext(err, "."+typ.Field(f.index).Name)
			}
		}
		return wrapStreamError(s.ListEnd(), typ)
	}
	return dec, nil
}

//makeptrdecoder创建一个解码为
//指针的元素类型。
func makePtrDecoder(typ reflect.Type) (decoder, error) {
	etype := typ.Elem()
	etypeinfo, err := cachedTypeInfo1(etype, tags{})
	if err != nil {
		return nil, err
	}
	dec := func(s *Stream, val reflect.Value) (err error) {
		newval := val
		if val.IsNil() {
			newval = reflect.New(etype)
		}
		if err = etypeinfo.decoder(s, newval.Elem()); err == nil {
			val.Set(newval)
		}
		return err
	}
	return dec, nil
}

//makeoptionalptrDecoder创建一个解码空值的解码器
//为零。将非空值解码为元素类型的值，
//就像makeptrecoder一样。
//
//此解码器用于结构标记为“nil”的指针类型结构字段。
func makeOptionalPtrDecoder(typ reflect.Type) (decoder, error) {
	etype := typ.Elem()
	etypeinfo, err := cachedTypeInfo1(etype, tags{})
	if err != nil {
		return nil, err
	}
	dec := func(s *Stream, val reflect.Value) (err error) {
		kind, size, err := s.Kind()
		if err != nil || size == 0 && kind != Byte {
//重新武装S类。这很重要，因为输入
//位置必须前进到下一个值，即使
//我们什么都不看。
			s.kind = -1
//将指针设置为零。
			val.Set(reflect.Zero(typ))
			return err
		}
		newval := val
		if val.IsNil() {
			newval = reflect.New(etype)
		}
		if err = etypeinfo.decoder(s, newval.Elem()); err == nil {
			val.Set(newval)
		}
		return err
	}
	return dec, nil
}

var ifsliceType = reflect.TypeOf([]interface{}{})

func decodeInterface(s *Stream, val reflect.Value) error {
	if val.Type().NumMethod() != 0 {
		return fmt.Errorf("rlp: type %v is not RLP-serializable", val.Type())
	}
	kind, _, err := s.Kind()
	if err != nil {
		return err
	}
	if kind == List {
		slice := reflect.New(ifsliceType).Elem()
		if err := decodeListSlice(s, slice, decodeInterface); err != nil {
			return err
		}
		val.Set(slice)
	} else {
		b, err := s.Bytes()
		if err != nil {
			return err
		}
		val.Set(reflect.ValueOf(b))
	}
	return nil
}

//此解码器用于类型的非指针值
//使用指针接收器实现解码器接口的方法。
func decodeDecoderNoPtr(s *Stream, val reflect.Value) error {
	return val.Addr().Interface().(Decoder).DecodeRLP(s)
}

func decodeDecoder(s *Stream, val reflect.Value) error {
//如果类型为
//使用指针接收器实现解码器（即始终）
//因为它可能专门处理空值。
//在这种情况下，我们需要在这里分配一个，就像makeptrdecoder一样。
	if val.Kind() == reflect.Ptr && val.IsNil() {
		val.Set(reflect.New(val.Type().Elem()))
	}
	return val.Interface().(Decoder).DecodeRLP(s)
}

//kind表示RLP流中包含的值的类型。
type Kind int

const (
	Byte Kind = iota
	String
	List
)

func (k Kind) String() string {
	switch k {
	case Byte:
		return "Byte"
	case String:
		return "String"
	case List:
		return "List"
	default:
		return fmt.Sprintf("Unknown(%d)", k)
	}
}

//bytereader必须由流的任何输入读取器实现。它
//由bufio.reader和bytes.reader等实现。
type ByteReader interface {
	io.Reader
	io.ByteReader
}

//流可用于输入流的逐段解码。这个
//如果输入非常大或解码规则
//类型取决于输入结构。流不保留
//内部缓冲区。解码值后，输入读卡器将
//位于下一个值的类型信息之前。
//
//当解码列表时，输入位置达到声明的
//列表的长度，所有操作都将返回错误eol。
//必须使用list end继续确认列表结尾
//正在读取封闭列表。
//
//流对于并发使用不安全。
type Stream struct {
	r ByteReader

//要从r中读取的剩余字节数。
	remaining uint64
	limited   bool

//整数译码辅助缓冲区
	uintbuf []byte

kind    Kind   //未来的价值
size    uint64 //前面的价值大小
byteval byte   //类型标记中单个字节的值
kinderr error  //上次readkind出错
	stack   []listpos
}

type listpos struct{ pos, size uint64 }

//Newstream创建了一个新的解码流读取R。
//
//如果r实现了bytereader接口，那么流将
//不要引入任何缓冲。
//
//对于非顶级值，流返回errelemtoolarge
//用于不适合封闭列表的值。
//
//流支持可选的输入限制。如果设置了限制，则
//任何顶级值的大小将与其余值进行检查
//输入长度。遇到超过值的流操作
//剩余的输入长度将返回errValuetoolArge。极限
//可以通过为inputLimit传递非零值来设置。
//
//如果r是bytes.reader或strings.reader，则输入限制设置为
//r的基础数据的长度，除非显式限制为
//提供。
func NewStream(r io.Reader, inputLimit uint64) *Stream {
	s := new(Stream)
	s.Reset(r, inputLimit)
	return s
}

//newliststream创建一个新的流，它假装被定位
//在给定长度的编码列表中。
func NewListStream(r io.Reader, len uint64) *Stream {
	s := new(Stream)
	s.Reset(r, len)
	s.kind = List
	s.size = len
	return s
}

//字节读取一个rlp字符串并将其内容作为字节片返回。
//如果输入不包含rlp字符串，则返回
//错误将是errExpectedString。
func (s *Stream) Bytes() ([]byte, error) {
	kind, size, err := s.Kind()
	if err != nil {
		return nil, err
	}
	switch kind {
	case Byte:
s.kind = -1 //重整种类
		return []byte{s.byteval}, nil
	case String:
		b := make([]byte, size)
		if err = s.readFull(b); err != nil {
			return nil, err
		}
		if size == 1 && b[0] < 128 {
			return nil, ErrCanonSize
		}
		return b, nil
	default:
		return nil, ErrExpectedString
	}
}

//raw读取包含rlp类型信息的原始编码值。
func (s *Stream) Raw() ([]byte, error) {
	kind, size, err := s.Kind()
	if err != nil {
		return nil, err
	}
	if kind == Byte {
s.kind = -1 //重整种类
		return []byte{s.byteval}, nil
	}
//原始头已被读取，不再是
//可用。阅读内容并在其前面放置一个新标题。
	start := headsize(size)
	buf := make([]byte, uint64(start)+size)
	if err := s.readFull(buf[start:]); err != nil {
		return nil, err
	}
	if kind == String {
		puthead(buf, 0x80, 0xB7, size)
	} else {
		puthead(buf, 0xC0, 0xF7, size)
	}
	return buf, nil
}

//uint读取最多8个字节的rlp字符串并返回其内容
//作为无符号整数。如果输入不包含rlp字符串，则
//返回的错误将是errExpectedString。
func (s *Stream) Uint() (uint64, error) {
	return s.uint(64)
}

func (s *Stream) uint(maxbits int) (uint64, error) {
	kind, size, err := s.Kind()
	if err != nil {
		return 0, err
	}
	switch kind {
	case Byte:
		if s.byteval == 0 {
			return 0, ErrCanonInt
		}
s.kind = -1 //重整种类
		return uint64(s.byteval), nil
	case String:
		if size > uint64(maxbits/8) {
			return 0, errUintOverflow
		}
		v, err := s.readUint(byte(size))
		switch {
		case err == ErrCanonSize:
//调整错误，因为我们现在没有读取大小。
			return 0, ErrCanonInt
		case err != nil:
			return 0, err
		case size > 0 && v < 128:
			return 0, ErrCanonSize
		default:
			return v, nil
		}
	default:
		return 0, ErrExpectedString
	}
}

//bool读取最多1字节的rlp字符串并返回其内容
//作为布尔值。如果输入不包含rlp字符串，则
//返回的错误将是errExpectedString。
func (s *Stream) Bool() (bool, error) {
	num, err := s.uint(8)
	if err != nil {
		return false, err
	}
	switch num {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("rlp: invalid boolean value: %d", num)
	}
}

//list开始解码rlp列表。如果输入不包含
//列表中，返回的错误将是errExpectedList。当名单的
//已到达END，任何流操作都将返回EOL。
func (s *Stream) List() (size uint64, err error) {
	kind, size, err := s.Kind()
	if err != nil {
		return 0, err
	}
	if kind != List {
		return 0, ErrExpectedList
	}
	s.stack = append(s.stack, listpos{0, size})
	s.kind = -1
	s.size = 0
	return size, nil
}

//listened返回到封闭列表。
//输入读取器必须位于列表的末尾。
func (s *Stream) ListEnd() error {
	if len(s.stack) == 0 {
		return errNotInList
	}
	tos := s.stack[len(s.stack)-1]
	if tos.pos != tos.size {
		return errNotAtEOL
	}
s.stack = s.stack[:len(s.stack)-1] //流行音乐
	if len(s.stack) > 0 {
		s.stack[len(s.stack)-1].pos += tos.size
	}
	s.kind = -1
	s.size = 0
	return nil
}

//解码解码解码值并将结果存储在指向的值中
//按VAL。有关解码功能，请参阅文档。
//了解解码规则。
func (s *Stream) Decode(val interface{}) error {
	if val == nil {
		return errDecodeIntoNil
	}
	rval := reflect.ValueOf(val)
	rtyp := rval.Type()
	if rtyp.Kind() != reflect.Ptr {
		return errNoPointer
	}
	if rval.IsNil() {
		return errDecodeIntoNil
	}
	info, err := cachedTypeInfo(rtyp.Elem(), tags{})
	if err != nil {
		return err
	}

	err = info.decoder(s, rval.Elem())
	if decErr, ok := err.(*decodeError); ok && len(decErr.ctx) > 0 {
//将解码目标类型添加到错误中，以便上下文具有更多含义
		decErr.ctx = append(decErr.ctx, fmt.Sprint("(", rtyp.Elem(), ")"))
	}
	return err
}

//重置将丢弃有关当前解码上下文的任何信息
//并从R开始读取。此方法旨在促进重用
//在许多解码操作中预先分配的流。
//
//如果r不同时实现bytereader，则流将自己执行。
//缓冲。
func (s *Stream) Reset(r io.Reader, inputLimit uint64) {
	if inputLimit > 0 {
		s.remaining = inputLimit
		s.limited = true
	} else {
//尝试自动发现
//从字节片读取时的限制。
		switch br := r.(type) {
		case *bytes.Reader:
			s.remaining = uint64(br.Len())
			s.limited = true
		case *strings.Reader:
			s.remaining = uint64(br.Len())
			s.limited = true
		default:
			s.limited = false
		}
	}
//如果没有缓冲区，用缓冲区包装R。
	bufr, ok := r.(ByteReader)
	if !ok {
		bufr = bufio.NewReader(r)
	}
	s.r = bufr
//重置解码上下文。
	s.stack = s.stack[:0]
	s.size = 0
	s.kind = -1
	s.kinderr = nil
	if s.uintbuf == nil {
		s.uintbuf = make([]byte, 8)
	}
}

//kind返回
//输入流。
//
//返回的大小是组成该值的字节数。
//对于kind==byte，大小为零，因为值为
//包含在类型标记中。
//
//第一次调用kind将从输入中读取大小信息
//读卡器并将其保留在
//价值。后续对kind的调用（直到值被解码）
//不会提升输入读取器并返回缓存信息。
func (s *Stream) Kind() (kind Kind, size uint64, err error) {
	var tos *listpos
	if len(s.stack) > 0 {
		tos = &s.stack[len(s.stack)-1]
	}
	if s.kind < 0 {
		s.kinderr = nil
//如果我们在
//最内层列表。
		if tos != nil && tos.pos == tos.size {
			return 0, 0, EOL
		}
		s.kind, s.size, s.kinderr = s.readKind()
		if s.kinderr == nil {
			if tos == nil {
//在顶层，检查该值是否较小
//大于剩余的输入长度。
				if s.limited && s.size > s.remaining {
					s.kinderr = ErrValueTooLarge
				}
			} else {
//在列表中，检查该值是否溢出列表。
				if s.size > tos.size-tos.pos {
					s.kinderr = ErrElemTooLarge
				}
			}
		}
	}
//注意：这可能会返回一个生成的粘性错误
//通过早期的readkind调用。
	return s.kind, s.size, s.kinderr
}

func (s *Stream) readKind() (kind Kind, size uint64, err error) {
	b, err := s.readByte()
	if err != nil {
		if len(s.stack) == 0 {
//在顶层，将误差调整为实际的EOF。IOF是
//被调用方用于确定何时停止解码。
			switch err {
			case io.ErrUnexpectedEOF:
				err = io.EOF
			case ErrValueTooLarge:
				err = io.EOF
			}
		}
		return 0, 0, err
	}
	s.byteval = 0
	switch {
	case b < 0x80:
//对于值在[0x00，0x7f]范围内的单个字节，该字节
//是它自己的RLP编码。
		s.byteval = b
		return Byte, 0, nil
	case b < 0xB8:
//否则，如果字符串的长度为0-55字节，
//RLP编码由一个值为0x80的单字节加上
//字符串的长度，后跟字符串。第一个的范围
//因此，字节为[0x80，0xB7]。
		return String, uint64(b - 0x80), nil
	case b < 0xC0:
//如果字符串的长度超过55个字节，则
//RLP编码由一个单字节组成，值为0xB7加上长度
//以二进制形式表示的字符串长度，后跟
//字符串，后跟字符串。例如，长度为1024的字符串
//将编码为0xB90400，后跟字符串。范围
//因此，第一个字节是[0xB8，0xBF]。
		size, err = s.readUint(b - 0xB7)
		if err == nil && size < 56 {
			err = ErrCanonSize
		}
		return String, size, err
	case b < 0xF8:
//如果列表的总有效负载
//（即，所有项目的组合长度）为0-55字节长，
//RLP编码由一个值为0xC0的单字节加上长度组成。
//列表的后面是
//项目。因此，第一个字节的范围是[0xc0，0xf7]。
		return List, uint64(b - 0xC0), nil
	default:
//如果列表的总有效负载长度超过55字节，
//RLP编码由一个值为0xF7的单字节组成。
//加上以二进制表示的有效载荷长度
//形式，后跟有效载荷的长度，然后是
//项目的rlp编码的串联。这个
//因此，第一个字节的范围是[0xf8，0xff]。
		size, err = s.readUint(b - 0xF7)
		if err == nil && size < 56 {
			err = ErrCanonSize
		}
		return List, size, err
	}
}

func (s *Stream) readUint(size byte) (uint64, error) {
	switch size {
	case 0:
s.kind = -1 //重整种类
		return 0, nil
	case 1:
		b, err := s.readByte()
		return uint64(b), err
	default:
		start := int(8 - size)
		for i := 0; i < start; i++ {
			s.uintbuf[i] = 0
		}
		if err := s.readFull(s.uintbuf[start:]); err != nil {
			return 0, err
		}
		if s.uintbuf[start] == 0 {
//注意：readuint也用于解码整数
//价值观。需要调整误差
//在本例中是errconInt。
			return 0, ErrCanonSize
		}
		return binary.BigEndian.Uint64(s.uintbuf), nil
	}
}

func (s *Stream) readFull(buf []byte) (err error) {
	if err := s.willRead(uint64(len(buf))); err != nil {
		return err
	}
	var nn, n int
	for n < len(buf) && err == nil {
		nn, err = s.r.Read(buf[n:])
		n += nn
	}
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return err
}

func (s *Stream) readByte() (byte, error) {
	if err := s.willRead(1); err != nil {
		return 0, err
	}
	b, err := s.r.ReadByte()
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return b, err
}

func (s *Stream) willRead(n uint64) error {
s.kind = -1 //重整种类

	if len(s.stack) > 0 {
//检查列表溢出
		tos := s.stack[len(s.stack)-1]
		if n > tos.size-tos.pos {
			return ErrElemTooLarge
		}
		s.stack[len(s.stack)-1].pos += n
	}
	if s.limited {
		if n > s.remaining {
			return ErrValueTooLarge
		}
		s.remaining -= n
	}
	return nil
}
