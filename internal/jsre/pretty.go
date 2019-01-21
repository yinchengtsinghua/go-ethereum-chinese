
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

package jsre

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/robertkrimen/otto"
)

const (
	maxPrettyPrintLevel = 3
	indentString        = "  "
)

var (
	FunctionColor = color.New(color.FgMagenta).SprintfFunc()
	SpecialColor  = color.New(color.Bold).SprintfFunc()
	NumberColor   = color.New(color.FgRed).SprintfFunc()
	StringColor   = color.New(color.FgGreen).SprintfFunc()
	ErrorColor    = color.New(color.FgHiRed).SprintfFunc()
)

//打印对象时隐藏这些字段。
var boringKeys = map[string]bool{
	"valueOf":              true,
	"toString":             true,
	"toLocaleString":       true,
	"hasOwnProperty":       true,
	"isPrototypeOf":        true,
	"propertyIsEnumerable": true,
	"constructor":          true,
}

//预打印将值写入标准输出。
func prettyPrint(vm *otto.Otto, value otto.Value, w io.Writer) {
	ppctx{vm: vm, w: w}.printValue(value, 0, false)
}

//PrettyError将错误写入标准输出。
func prettyError(vm *otto.Otto, err error, w io.Writer) {
	failure := err.Error()
	if ottoErr, ok := err.(*otto.Error); ok {
		failure = ottoErr.String()
	}
	fmt.Fprint(w, ErrorColor("%s", failure))
}

func (re *JSRE) prettyPrintJS(call otto.FunctionCall) otto.Value {
	for _, v := range call.ArgumentList {
		prettyPrint(call.Otto, v, re.output)
		fmt.Fprintln(re.output)
	}
	return otto.UndefinedValue()
}

type ppctx struct {
	vm *otto.Otto
	w  io.Writer
}

func (ctx ppctx) indent(level int) string {
	return strings.Repeat(indentString, level)
}

func (ctx ppctx) printValue(v otto.Value, level int, inArray bool) {
	switch {
	case v.IsObject():
		ctx.printObject(v.Object(), level, inArray)
	case v.IsNull():
		fmt.Fprint(ctx.w, SpecialColor("null"))
	case v.IsUndefined():
		fmt.Fprint(ctx.w, SpecialColor("undefined"))
	case v.IsString():
		s, _ := v.ToString()
		fmt.Fprint(ctx.w, StringColor("%q", s))
	case v.IsBoolean():
		b, _ := v.ToBoolean()
		fmt.Fprint(ctx.w, SpecialColor("%t", b))
	case v.IsNaN():
		fmt.Fprint(ctx.w, NumberColor("NaN"))
	case v.IsNumber():
		s, _ := v.ToString()
		fmt.Fprint(ctx.w, NumberColor("%s", s))
	default:
		fmt.Fprint(ctx.w, "<unprintable>")
	}
}

func (ctx ppctx) printObject(obj *otto.Object, level int, inArray bool) {
	switch obj.Class() {
	case "Array", "GoArray":
		lv, _ := obj.Get("length")
		len, _ := lv.ToInteger()
		if len == 0 {
			fmt.Fprintf(ctx.w, "[]")
			return
		}
		if level > maxPrettyPrintLevel {
			fmt.Fprint(ctx.w, "[...]")
			return
		}
		fmt.Fprint(ctx.w, "[")
		for i := int64(0); i < len; i++ {
			el, err := obj.Get(strconv.FormatInt(i, 10))
			if err == nil {
				ctx.printValue(el, level+1, true)
			}
			if i < len-1 {
				fmt.Fprintf(ctx.w, ", ")
			}
		}
		fmt.Fprint(ctx.w, "]")

	case "Object":
//将bignumber.js中的值打印为常规数字。
		if ctx.isBigNumber(obj) {
			fmt.Fprint(ctx.w, NumberColor("%s", toString(obj)))
			return
		}
//否则，打印所有缩进的字段，但如果太深则停止。
		keys := ctx.fields(obj)
		if len(keys) == 0 {
			fmt.Fprint(ctx.w, "{}")
			return
		}
		if level > maxPrettyPrintLevel {
			fmt.Fprint(ctx.w, "{...}")
			return
		}
		fmt.Fprintln(ctx.w, "{")
		for i, k := range keys {
			v, _ := obj.Get(k)
			fmt.Fprintf(ctx.w, "%s%s: ", ctx.indent(level+1), k)
			ctx.printValue(v, level+1, false)
			if i < len(keys)-1 {
				fmt.Fprintf(ctx.w, ",")
			}
			fmt.Fprintln(ctx.w)
		}
		if inArray {
			level--
		}
		fmt.Fprintf(ctx.w, "%s}", ctx.indent(level))

	case "Function":
//如果可能，使用ToString（）显示参数列表。
		if robj, err := obj.Call("toString"); err != nil {
			fmt.Fprint(ctx.w, FunctionColor("function()"))
		} else {
			desc := strings.Trim(strings.Split(robj.String(), "{")[0], " \t\n")
			desc = strings.Replace(desc, " (", "(", 1)
			fmt.Fprint(ctx.w, FunctionColor("%s", desc))
		}

	case "RegExp":
		fmt.Fprint(ctx.w, StringColor("%s", toString(obj)))

	default:
		if v, _ := obj.Get("toString"); v.IsFunction() && level <= maxPrettyPrintLevel {
			s, _ := obj.Call("toString")
			fmt.Fprintf(ctx.w, "<%s %s>", obj.Class(), s.String())
		} else {
			fmt.Fprintf(ctx.w, "<%s>", obj.Class())
		}
	}
}

func (ctx ppctx) fields(obj *otto.Object) []string {
	var (
		vals, methods []string
		seen          = make(map[string]bool)
	)
	add := func(k string) {
		if seen[k] || boringKeys[k] || strings.HasPrefix(k, "_") {
			return
		}
		seen[k] = true
		if v, _ := obj.Get(k); v.IsFunction() {
			methods = append(methods, k)
		} else {
			vals = append(vals, k)
		}
	}
	iterOwnAndConstructorKeys(ctx.vm, obj, add)
	sort.Strings(vals)
	sort.Strings(methods)
	return append(vals, methods...)
}

func iterOwnAndConstructorKeys(vm *otto.Otto, obj *otto.Object, f func(string)) {
	seen := make(map[string]bool)
	iterOwnKeys(vm, obj, func(prop string) {
		seen[prop] = true
		f(prop)
	})
	if cp := constructorPrototype(obj); cp != nil {
		iterOwnKeys(vm, cp, func(prop string) {
			if !seen[prop] {
				f(prop)
			}
		})
	}
}

func iterOwnKeys(vm *otto.Otto, obj *otto.Object, f func(string)) {
	Object, _ := vm.Object("Object")
	rv, _ := Object.Call("getOwnPropertyNames", obj.Value())
	gv, _ := rv.Export()
	switch gv := gv.(type) {
	case []interface{}:
		for _, v := range gv {
			f(v.(string))
		}
	case []string:
		for _, v := range gv {
			f(v)
		}
	default:
		panic(fmt.Errorf("Object.getOwnPropertyNames returned unexpected type %T", gv))
	}
}

func (ctx ppctx) isBigNumber(v *otto.Object) bool {
//使用自定义构造函数处理数字。
	if v, _ := v.Get("constructor"); v.Object() != nil {
		if strings.HasPrefix(toString(v.Object()), "function BigNumber") {
			return true
		}
	}
//处理默认构造函数。
	BigNumber, _ := ctx.vm.Object("BigNumber.prototype")
	if BigNumber == nil {
		return false
	}
	bv, _ := BigNumber.Call("isPrototypeOf", v)
	b, _ := bv.ToBoolean()
	return b
}

func toString(obj *otto.Object) string {
	s, _ := obj.Call("toString")
	return s.String()
}

func constructorPrototype(obj *otto.Object) *otto.Object {
	if v, _ := obj.Get("constructor"); v.Object() != nil {
		if v, _ = v.Object().Get("prototype"); v.Object() != nil {
			return v.Object()
		}
	}
	return nil
}
