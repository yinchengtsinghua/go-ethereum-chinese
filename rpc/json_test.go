
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

package rpc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"testing"
)

type RWC struct {
	*bufio.ReadWriter
}

func (rwc *RWC) Close() error {
	return nil
}

func TestJSONRequestParsing(t *testing.T) {
	server := NewServer()
	service := new(Service)

	if err := server.RegisterName("calc", service); err != nil {
		t.Fatalf("%v", err)
	}

	req := bytes.NewBufferString(`{"id": 1234, "jsonrpc": "2.0", "method": "calc_add", "params": [11, 22]}`)
	var str string
	reply := bytes.NewBufferString(str)
	rw := &RWC{bufio.NewReadWriter(bufio.NewReader(req), bufio.NewWriter(reply))}

	codec := NewJSONCodec(rw)

	requests, batch, err := codec.ReadRequestHeaders()
	if err != nil {
		t.Fatalf("%v", err)
	}

	if batch {
		t.Fatalf("Request isn't a batch")
	}

	if len(requests) != 1 {
		t.Fatalf("Expected 1 request but got %d requests - %v", len(requests), requests)
	}

	if requests[0].service != "calc" {
		t.Fatalf("Expected service 'calc' but got '%s'", requests[0].service)
	}

	if requests[0].method != "add" {
		t.Fatalf("Expected method 'Add' but got '%s'", requests[0].method)
	}

	if rawId, ok := requests[0].id.(*json.RawMessage); ok {
		id, e := strconv.ParseInt(string(*rawId), 0, 64)
		if e != nil {
			t.Fatalf("%v", e)
		}
		if id != 1234 {
			t.Fatalf("Expected id 1234 but got %d", id)
		}
	} else {
		t.Fatalf("invalid request, expected *json.RawMesage got %T", requests[0].id)
	}

	var arg int
	args := []reflect.Type{reflect.TypeOf(arg), reflect.TypeOf(arg)}

	v, err := codec.ParseRequestArguments(args, requests[0].params)
	if err != nil {
		t.Fatalf("%v", err)
	}

	if len(v) != 2 {
		t.Fatalf("Expected 2 argument values, got %d", len(v))
	}

	if v[0].Int() != 11 || v[1].Int() != 22 {
		t.Fatalf("expected %d == 11 && %d == 22", v[0].Int(), v[1].Int())
	}
}

func TestJSONRequestParamsParsing(t *testing.T) {

	var (
		stringT = reflect.TypeOf("")
		intT    = reflect.TypeOf(0)
		intPtrT = reflect.TypeOf(new(int))

		stringV = reflect.ValueOf("abc")
		i       = 1
		intV    = reflect.ValueOf(i)
		intPtrV = reflect.ValueOf(&i)
	)

	var validTests = []struct {
		input    string
		argTypes []reflect.Type
		expected []reflect.Value
	}{
		{`[]`, []reflect.Type{}, []reflect.Value{}},
		{`[]`, []reflect.Type{intPtrT}, []reflect.Value{intPtrV}},
		{`[1]`, []reflect.Type{intT}, []reflect.Value{intV}},
		{`[1,"abc"]`, []reflect.Type{intT, stringT}, []reflect.Value{intV, stringV}},
		{`[null]`, []reflect.Type{intPtrT}, []reflect.Value{intPtrV}},
		{`[null,"abc"]`, []reflect.Type{intPtrT, stringT, intPtrT}, []reflect.Value{intPtrV, stringV, intPtrV}},
		{`[null,"abc",null]`, []reflect.Type{intPtrT, stringT, intPtrT}, []reflect.Value{intPtrV, stringV, intPtrV}},
	}

	codec := jsonCodec{}

	for _, test := range validTests {
		params := (json.RawMessage)([]byte(test.input))
		args, err := codec.ParseRequestArguments(test.argTypes, params)

		if err != nil {
			t.Fatal(err)
		}

		var match []interface{}
		json.Unmarshal([]byte(test.input), &match)

		if len(args) != len(test.argTypes) {
			t.Fatalf("expected %d parsed args, got %d", len(test.argTypes), len(args))
		}

		for i, arg := range args {
			expected := test.expected[i]

			if arg.Kind() != expected.Kind() {
				t.Errorf("expected type for param %d in %s", i, test.input)
			}

			if arg.Kind() == reflect.Int && arg.Int() != expected.Int() {
				t.Errorf("expected int(%d), got int(%d) in %s", expected.Int(), arg.Int(), test.input)
			}

			if arg.Kind() == reflect.String && arg.String() != expected.String() {
				t.Errorf("expected string(%s), got string(%s) in %s", expected.String(), arg.String(), test.input)
			}
		}
	}

	var invalidTests = []struct {
		input    string
		argTypes []reflect.Type
	}{
		{`[]`, []reflect.Type{intT}},
		{`[null]`, []reflect.Type{intT}},
		{`[1]`, []reflect.Type{stringT}},
		{`[1,2]`, []reflect.Type{stringT}},
		{`["abc", null]`, []reflect.Type{stringT, intT}},
	}

	for i, test := range invalidTests {
		if _, err := codec.ParseRequestArguments(test.argTypes, test.input); err == nil {
			t.Errorf("expected test %d - %s to fail", i, test.input)
		}
	}
}
