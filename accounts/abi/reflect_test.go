
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2019 Go Ethereum作者
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
	"reflect"
	"testing"
)

type reflectTest struct {
	name  string
	args  []string
	struc interface{}
	want  map[string]string
	err   string
}

var reflectTests = []reflectTest{
	{
		name: "OneToOneCorrespondance",
		args: []string{"fieldA"},
		struc: struct {
			FieldA int `abi:"fieldA"`
		}{},
		want: map[string]string{
			"fieldA": "FieldA",
		},
	},
	{
		name: "MissingFieldsInStruct",
		args: []string{"fieldA", "fieldB"},
		struc: struct {
			FieldA int `abi:"fieldA"`
		}{},
		want: map[string]string{
			"fieldA": "FieldA",
		},
	},
	{
		name: "MoreFieldsInStructThanArgs",
		args: []string{"fieldA"},
		struc: struct {
			FieldA int `abi:"fieldA"`
			FieldB int
		}{},
		want: map[string]string{
			"fieldA": "FieldA",
		},
	},
	{
		name: "MissingFieldInArgs",
		args: []string{"fieldA"},
		struc: struct {
			FieldA int `abi:"fieldA"`
			FieldB int `abi:"fieldB"`
		}{},
		err: "struct: abi tag 'fieldB' defined but not found in abi",
	},
	{
		name: "NoAbiDescriptor",
		args: []string{"fieldA"},
		struc: struct {
			FieldA int
		}{},
		want: map[string]string{
			"fieldA": "FieldA",
		},
	},
	{
		name: "NoArgs",
		args: []string{},
		struc: struct {
			FieldA int `abi:"fieldA"`
		}{},
		err: "struct: abi tag 'fieldA' defined but not found in abi",
	},
	{
		name: "DifferentName",
		args: []string{"fieldB"},
		struc: struct {
			FieldA int `abi:"fieldB"`
		}{},
		want: map[string]string{
			"fieldB": "FieldA",
		},
	},
	{
		name: "DifferentName",
		args: []string{"fieldB"},
		struc: struct {
			FieldA int `abi:"fieldB"`
		}{},
		want: map[string]string{
			"fieldB": "FieldA",
		},
	},
	{
		name: "MultipleFields",
		args: []string{"fieldA", "fieldB"},
		struc: struct {
			FieldA int `abi:"fieldA"`
			FieldB int `abi:"fieldB"`
		}{},
		want: map[string]string{
			"fieldA": "FieldA",
			"fieldB": "FieldB",
		},
	},
	{
		name: "MultipleFieldsABIMissing",
		args: []string{"fieldA", "fieldB"},
		struc: struct {
			FieldA int `abi:"fieldA"`
			FieldB int
		}{},
		want: map[string]string{
			"fieldA": "FieldA",
			"fieldB": "FieldB",
		},
	},
	{
		name: "NameConflict",
		args: []string{"fieldB"},
		struc: struct {
			FieldA int `abi:"fieldB"`
			FieldB int
		}{},
		err: "abi: multiple variables maps to the same abi field 'fieldB'",
	},
	{
		name: "Underscored",
		args: []string{"_"},
		struc: struct {
			FieldA int
		}{},
		err: "abi: purely underscored output cannot unpack to struct",
	},
	{
		name: "DoubleMapping",
		args: []string{"fieldB", "fieldC", "fieldA"},
		struc: struct {
			FieldA int `abi:"fieldC"`
			FieldB int
		}{},
		err: "abi: multiple outputs mapping to the same struct field 'FieldA'",
	},
	{
		name: "AlreadyMapped",
		args: []string{"fieldB", "fieldB"},
		struc: struct {
			FieldB int `abi:"fieldB"`
		}{},
		err: "struct: abi tag in 'FieldB' already mapped",
	},
}

func TestReflectNameToStruct(t *testing.T) {
	for _, test := range reflectTests {
		t.Run(test.name, func(t *testing.T) {
			m, err := mapArgNamesToStructFields(test.args, reflect.ValueOf(test.struc))
			if len(test.err) > 0 {
				if err == nil || err.Error() != test.err {
					t.Fatalf("Invalid error: expected %v, got %v", test.err, err)
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				for fname := range test.want {
					if m[fname] != test.want[fname] {
						t.Fatalf("Incorrect value for field %s: expected %v, got %v", fname, test.want[fname], m[fname])
					}
				}
			}
		})
	}
}
