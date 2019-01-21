
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
	"fmt"
	"io"
	"math/big"
	"reflect"
	"sync"
)

var (
//通用编码值。
//这些在实现encoderlp时很有用。
	EmptyString = []byte{0x80}
	EmptyList   = []byte{0xC0}
)

//编码器由需要自定义的类型实现
//编码规则或要对私有字段进行编码。
type Encoder interface {
//encoderlp应将其接收器的rlp编码写入w。
//如果实现是一个指针方法，它也可以是
//要求零指针。
//
//实现应该生成有效的rlp。写入的数据是
//目前尚未验证，但将来的版本可能会验证。它是
//建议只写一个值，但写多个值
//也允许有值或没有值。
	EncodeRLP(io.Writer) error
}

//encode将val的rlp编码写入w。注意，encode可以
//在某些情况下执行许多小的写入操作。考虑使w
//缓冲的。
//
//encode使用以下与类型相关的编码规则：
//
//如果类型实现编码器接口，则编码调用
//EncodeRLP。这是真的，即使没有指针，请看
//编码器文件。
//
//要对指针进行编码，将对指向的值进行编码。为零
//指针，encode将对类型的零值进行编码。零
//指向结构类型的指针始终编码为空的rlp列表。
//指向数组的nil指针编码为空列表（或空字符串
//如果数组具有元素类型byte）。
//
//结构值被编码为其所有编码的rlp列表
//公共领域。支持递归结构类型。
//
//为了对切片和数组进行编码，元素被编码为rlp
//值元素的列表。注意数组和切片
//元素类型uint8或byte始终编码为rlp字符串。
//
//go字符串编码为rlp字符串。
//
//无符号整数值编码为RLP字符串。零总是
//编码为空的rlp字符串。encode还支持*big.int。
//
//接口值编码为接口中包含的值。
//
//不支持布尔值，也不支持有符号整数，浮动
//点编号、地图、通道和功能。
func Encode(w io.Writer, val interface{}) error {
	if outer, ok := w.(*encbuf); ok {
//encode被某个类型的encoderlp调用。
//避免通过直接写入外部encbuf进行复制。
		return outer.encode(val)
	}
	eb := encbufPool.Get().(*encbuf)
	defer encbufPool.Put(eb)
	eb.reset()
	if err := eb.encode(val); err != nil {
		return err
	}
	return eb.toWriter(w)
}

//encodeToBytes返回val的rlp编码。
//编码规则请参见Encode文档。
func EncodeToBytes(val interface{}) ([]byte, error) {
	eb := encbufPool.Get().(*encbuf)
	defer encbufPool.Put(eb)
	eb.reset()
	if err := eb.encode(val); err != nil {
		return nil, err
	}
	return eb.toBytes(), nil
}

//encodetoreader返回一个读卡器，其中val的rlp编码来自该读卡器
//可以阅读。返回的大小是已编码的
//数据。
//
//编码规则请参见Encode文档。
func EncodeToReader(val interface{}) (size int, r io.Reader, err error) {
	eb := encbufPool.Get().(*encbuf)
	eb.reset()
	if err := eb.encode(val); err != nil {
		return 0, nil, err
	}
	return eb.size(), &encReader{buf: eb}, nil
}

type encbuf struct {
str     []byte      //字符串数据，包含除列表头以外的所有内容
lheads  []*listhead //所有列表标题
lhsize  int         //所有编码列表头的大小总和
sizebuf []byte      //用于uint编码的9字节辅助缓冲区
}

type listhead struct {
offset int //字符串数据中此头的索引
size   int //编码数据的总大小（包括列表头）
}

//编码写入头到给定的缓冲区，该缓冲区必须至少为
//9字节长。它返回编码的字节。
func (head *listhead) encode(buf []byte) []byte {
	return buf[:puthead(buf, 0xC0, 0xF7, uint64(head.size))]
}

//headsize返回列表或字符串头的大小
//对于给定大小的值。
func headsize(size uint64) int {
	if size < 56 {
		return 1
	}
	return 1 + intsize(size)
}

//puthead将列表或字符串头写入buf。
//buf的长度必须至少为9个字节。
func puthead(buf []byte, smalltag, largetag byte, size uint64) int {
	if size < 56 {
		buf[0] = smalltag + byte(size)
		return 1
	}
	sizesize := putint(buf[1:], size)
	buf[0] = largetag + byte(sizesize)
	return sizesize + 1
}

//围场是集合的。
var encbufPool = sync.Pool{
	New: func() interface{} { return &encbuf{sizebuf: make([]byte, 9)} },
}

func (w *encbuf) reset() {
	w.lhsize = 0
	if w.str != nil {
		w.str = w.str[:0]
	}
	if w.lheads != nil {
		w.lheads = w.lheads[:0]
	}
}

//encbuf实现io.writer，以便将其传递到encoderlp。
func (w *encbuf) Write(b []byte) (int, error) {
	w.str = append(w.str, b...)
	return len(b), nil
}

func (w *encbuf) encode(val interface{}) error {
	rval := reflect.ValueOf(val)
	ti, err := cachedTypeInfo(rval.Type(), tags{})
	if err != nil {
		return err
	}
	return ti.writer(rval, w)
}

func (w *encbuf) encodeStringHeader(size int) {
	if size < 56 {
		w.str = append(w.str, 0x80+byte(size))
	} else {
//TODO:直接编码到w.str
		sizesize := putint(w.sizebuf[1:], uint64(size))
		w.sizebuf[0] = 0xB7 + byte(sizesize)
		w.str = append(w.str, w.sizebuf[:sizesize+1]...)
	}
}

func (w *encbuf) encodeString(b []byte) {
	if len(b) == 1 && b[0] <= 0x7F {
//适合单字节，无字符串头
		w.str = append(w.str, b[0])
	} else {
		w.encodeStringHeader(len(b))
		w.str = append(w.str, b...)
	}
}

func (w *encbuf) list() *listhead {
	lh := &listhead{offset: len(w.str), size: w.lhsize}
	w.lheads = append(w.lheads, lh)
	return lh
}

func (w *encbuf) listEnd(lh *listhead) {
	lh.size = w.size() - lh.offset - lh.size
	if lh.size < 56 {
w.lhsize++ //编码为种类标记的长度
	} else {
		w.lhsize += 1 + intsize(uint64(lh.size))
	}
}

func (w *encbuf) size() int {
	return len(w.str) + w.lhsize
}

func (w *encbuf) toBytes() []byte {
	out := make([]byte, w.size())
	strpos := 0
	pos := 0
	for _, head := range w.lheads {
//在头之前写入字符串数据
		n := copy(out[pos:], w.str[strpos:head.offset])
		pos += n
		strpos += n
//写标题
		enc := head.encode(out[pos:])
		pos += len(enc)
	}
//将字符串数据复制到最后一个列表头之后
	copy(out[pos:], w.str[strpos:])
	return out
}

func (w *encbuf) toWriter(out io.Writer) (err error) {
	strpos := 0
	for _, head := range w.lheads {
//在头之前写入字符串数据
		if head.offset-strpos > 0 {
			n, err := out.Write(w.str[strpos:head.offset])
			strpos += n
			if err != nil {
				return err
			}
		}
//写标题
		enc := head.encode(w.sizebuf)
		if _, err = out.Write(enc); err != nil {
			return err
		}
	}
	if strpos < len(w.str) {
//在最后一个列表头后写入字符串数据
		_, err = out.Write(w.str[strpos:])
	}
	return err
}

//Encrader是EncodeToReader返回的IO.Reader。
//它在EOF时释放ENCBUF。
type encReader struct {
buf    *encbuf //我们正在读取的缓冲区。当我们到达终点时，这是零。
lhpos  int     //正在读取的列表标题索引
strpos int     //字符串缓冲区中的当前位置
piece  []byte  //下一篇要读
}

func (r *encReader) Read(b []byte) (n int, err error) {
	for {
		if r.piece = r.next(); r.piece == nil {
//在EOF时将编码缓冲区放回池中
//是第一次遇到。后续呼叫仍返回EOF
//但是缓冲区不再有效。
			if r.buf != nil {
				encbufPool.Put(r.buf)
				r.buf = nil
			}
			return n, io.EOF
		}
		nn := copy(b[n:], r.piece)
		n += nn
		if nn < len(r.piece) {
//这件不合适，下次见。
			r.piece = r.piece[nn:]
			return n, nil
		}
		r.piece = nil
	}
}

//Next返回下一段要读取的数据。
//在EOF时返回零。
func (r *encReader) next() []byte {
	switch {
	case r.buf == nil:
		return nil

	case r.piece != nil:
//仍有数据可供读取。
		return r.piece

	case r.lhpos < len(r.buf.lheads):
//我们在最后一个列表标题之前。
		head := r.buf.lheads[r.lhpos]
		sizebefore := head.offset - r.strpos
		if sizebefore > 0 {
//头前的字符串数据。
			p := r.buf.str[r.strpos:head.offset]
			r.strpos += sizebefore
			return p
		}
		r.lhpos++
		return head.encode(r.buf.sizebuf)

	case r.strpos < len(r.buf.str):
//在所有列表头之后，在末尾字符串数据。
		p := r.buf.str[r.strpos:]
		r.strpos = len(r.buf.str)
		return p

	default:
		return nil
	}
}

var (
	encoderInterface = reflect.TypeOf(new(Encoder)).Elem()
	big0             = big.NewInt(0)
)

//makewriter为给定类型创建一个writer函数。
func makeWriter(typ reflect.Type, ts tags) (writer, error) {
	kind := typ.Kind()
	switch {
	case typ == rawValueType:
		return writeRawValue, nil
	case typ.Implements(encoderInterface):
		return writeEncoder, nil
	case kind != reflect.Ptr && reflect.PtrTo(typ).Implements(encoderInterface):
		return writeEncoderNoPtr, nil
	case kind == reflect.Interface:
		return writeInterface, nil
	case typ.AssignableTo(reflect.PtrTo(bigInt)):
		return writeBigIntPtr, nil
	case typ.AssignableTo(bigInt):
		return writeBigIntNoPtr, nil
	case isUint(kind):
		return writeUint, nil
	case kind == reflect.Bool:
		return writeBool, nil
	case kind == reflect.String:
		return writeString, nil
	case kind == reflect.Slice && isByte(typ.Elem()):
		return writeBytes, nil
	case kind == reflect.Array && isByte(typ.Elem()):
		return writeByteArray, nil
	case kind == reflect.Slice || kind == reflect.Array:
		return makeSliceWriter(typ, ts)
	case kind == reflect.Struct:
		return makeStructWriter(typ)
	case kind == reflect.Ptr:
		return makePtrWriter(typ)
	default:
		return nil, fmt.Errorf("rlp: type %v is not RLP-serializable", typ)
	}
}

func isByte(typ reflect.Type) bool {
	return typ.Kind() == reflect.Uint8 && !typ.Implements(encoderInterface)
}

func writeRawValue(val reflect.Value, w *encbuf) error {
	w.str = append(w.str, val.Bytes()...)
	return nil
}

func writeUint(val reflect.Value, w *encbuf) error {
	i := val.Uint()
	if i == 0 {
		w.str = append(w.str, 0x80)
	} else if i < 128 {
//适合单字节
		w.str = append(w.str, byte(i))
	} else {
//TODO:直接将int编码为w.str
		s := putint(w.sizebuf[1:], i)
		w.sizebuf[0] = 0x80 + byte(s)
		w.str = append(w.str, w.sizebuf[:s+1]...)
	}
	return nil
}

func writeBool(val reflect.Value, w *encbuf) error {
	if val.Bool() {
		w.str = append(w.str, 0x01)
	} else {
		w.str = append(w.str, 0x80)
	}
	return nil
}

func writeBigIntPtr(val reflect.Value, w *encbuf) error {
	ptr := val.Interface().(*big.Int)
	if ptr == nil {
		w.str = append(w.str, 0x80)
		return nil
	}
	return writeBigInt(ptr, w)
}

func writeBigIntNoPtr(val reflect.Value, w *encbuf) error {
	i := val.Interface().(big.Int)
	return writeBigInt(&i, w)
}

func writeBigInt(i *big.Int, w *encbuf) error {
	if cmp := i.Cmp(big0); cmp == -1 {
		return fmt.Errorf("rlp: cannot encode negative *big.Int")
	} else if cmp == 0 {
		w.str = append(w.str, 0x80)
	} else {
		w.encodeString(i.Bytes())
	}
	return nil
}

func writeBytes(val reflect.Value, w *encbuf) error {
	w.encodeString(val.Bytes())
	return nil
}

func writeByteArray(val reflect.Value, w *encbuf) error {
	if !val.CanAddr() {
//切片要求值是可寻址的。
//通过复制使其可寻址。
		copy := reflect.New(val.Type()).Elem()
		copy.Set(val)
		val = copy
	}
	size := val.Len()
	slice := val.Slice(0, size).Bytes()
	w.encodeString(slice)
	return nil
}

func writeString(val reflect.Value, w *encbuf) error {
	s := val.String()
	if len(s) == 1 && s[0] <= 0x7f {
//适合单字节，无字符串头
		w.str = append(w.str, s[0])
	} else {
		w.encodeStringHeader(len(s))
		w.str = append(w.str, s...)
	}
	return nil
}

func writeEncoder(val reflect.Value, w *encbuf) error {
	return val.Interface().(Encoder).EncodeRLP(w)
}

//WriteEncoderNoptr处理实现编码器的非指针值
//带指针接收器。
func writeEncoderNoPtr(val reflect.Value, w *encbuf) error {
	if !val.CanAddr() {
//我们找不到地址。它可以使
//通过创建一个浅副本可寻址的值，但是
//造成其他问题，所以我们还没有做。
//
//包json只是不调用marshaljson来处理类似的情况
//但对值进行编码，就好像它没有实现
//接口。我们不想那样处理。
		return fmt.Errorf("rlp: game over: unadressable value of type %v, EncodeRLP is pointer method", val.Type())
	}
	return val.Addr().Interface().(Encoder).EncodeRLP(w)
}

func writeInterface(val reflect.Value, w *encbuf) error {
	if val.IsNil() {
//写空列表。这与之前的RLP一致
//我们拥有的编码器，因此应该避免
//问题。
		w.str = append(w.str, 0xC0)
		return nil
	}
	eval := val.Elem()
	ti, err := cachedTypeInfo(eval.Type(), tags{})
	if err != nil {
		return err
	}
	return ti.writer(eval, w)
}

func makeSliceWriter(typ reflect.Type, ts tags) (writer, error) {
	etypeinfo, err := cachedTypeInfo1(typ.Elem(), tags{})
	if err != nil {
		return nil, err
	}
	writer := func(val reflect.Value, w *encbuf) error {
		if !ts.tail {
			defer w.listEnd(w.list())
		}
		vlen := val.Len()
		for i := 0; i < vlen; i++ {
			if err := etypeinfo.writer(val.Index(i), w); err != nil {
				return err
			}
		}
		return nil
	}
	return writer, nil
}

func makeStructWriter(typ reflect.Type) (writer, error) {
	fields, err := structFields(typ)
	if err != nil {
		return nil, err
	}
	writer := func(val reflect.Value, w *encbuf) error {
		lh := w.list()
		for _, f := range fields {
			if err := f.info.writer(val.Field(f.index), w); err != nil {
				return err
			}
		}
		w.listEnd(lh)
		return nil
	}
	return writer, nil
}

func makePtrWriter(typ reflect.Type) (writer, error) {
	etypeinfo, err := cachedTypeInfo1(typ.Elem(), tags{})
	if err != nil {
		return nil, err
	}

//确定零指针处理程序
	var nilfunc func(*encbuf) error
	kind := typ.Elem().Kind()
	switch {
	case kind == reflect.Array && isByte(typ.Elem().Elem()):
		nilfunc = func(w *encbuf) error {
			w.str = append(w.str, 0x80)
			return nil
		}
	case kind == reflect.Struct || kind == reflect.Array:
		nilfunc = func(w *encbuf) error {
//对结构/数组的零值进行编码可能会触发
//无限递归，避免这种情况。
			w.listEnd(w.list())
			return nil
		}
	default:
		zero := reflect.Zero(typ.Elem())
		nilfunc = func(w *encbuf) error {
			return etypeinfo.writer(zero, w)
		}
	}

	writer := func(val reflect.Value, w *encbuf) error {
		if val.IsNil() {
			return nilfunc(w)
		}
		return etypeinfo.writer(val.Elem(), w)
	}
	return writer, err
}

//putint用big endian字节将i写入b的开头
//顺序，使用表示i所需的最小字节数。
func putint(b []byte, i uint64) (size int) {
	switch {
	case i < (1 << 8):
		b[0] = byte(i)
		return 1
	case i < (1 << 16):
		b[0] = byte(i >> 8)
		b[1] = byte(i)
		return 2
	case i < (1 << 24):
		b[0] = byte(i >> 16)
		b[1] = byte(i >> 8)
		b[2] = byte(i)
		return 3
	case i < (1 << 32):
		b[0] = byte(i >> 24)
		b[1] = byte(i >> 16)
		b[2] = byte(i >> 8)
		b[3] = byte(i)
		return 4
	case i < (1 << 40):
		b[0] = byte(i >> 32)
		b[1] = byte(i >> 24)
		b[2] = byte(i >> 16)
		b[3] = byte(i >> 8)
		b[4] = byte(i)
		return 5
	case i < (1 << 48):
		b[0] = byte(i >> 40)
		b[1] = byte(i >> 32)
		b[2] = byte(i >> 24)
		b[3] = byte(i >> 16)
		b[4] = byte(i >> 8)
		b[5] = byte(i)
		return 6
	case i < (1 << 56):
		b[0] = byte(i >> 48)
		b[1] = byte(i >> 40)
		b[2] = byte(i >> 32)
		b[3] = byte(i >> 24)
		b[4] = byte(i >> 16)
		b[5] = byte(i >> 8)
		b[6] = byte(i)
		return 7
	default:
		b[0] = byte(i >> 56)
		b[1] = byte(i >> 48)
		b[2] = byte(i >> 40)
		b[3] = byte(i >> 32)
		b[4] = byte(i >> 24)
		b[5] = byte(i >> 16)
		b[6] = byte(i >> 8)
		b[7] = byte(i)
		return 8
	}
}

//IntSize计算存储i所需的最小字节数。
func intsize(i uint64) (size int) {
	for size = 1; ; size++ {
		if i >>= 8; i == 0 {
			return size
		}
	}
}
