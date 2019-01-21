
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
	"strings"
	"testing"
)

const methoddata = `
[
	{ "type" : "function", "name" : "balance", "constant" : true },
	{ "type" : "function", "name" : "send", "constant" : false, "inputs" : [ { "name" : "amount", "type" : "uint256" } ] },
	{ "type" : "function", "name" : "transfer", "constant" : false, "inputs" : [ { "name" : "from", "type" : "address" }, { "name" : "to", "type" : "address" }, { "name" : "value", "type" : "uint256" } ], "outputs" : [ { "name" : "success", "type" : "bool" } ]  }
]`

func TestMethodString(t *testing.T) {
	var table = []struct {
		method      string
		expectation string
	}{
		{
			method:      "balance",
			expectation: "function balance() constant returns()",
		},
		{
			method:      "send",
			expectation: "function send(uint256 amount) returns()",
		},
		{
			method:      "transfer",
			expectation: "function transfer(address from, address to, uint256 value) returns(bool success)",
		},
	}

	abi, err := JSON(strings.NewReader(methoddata))
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range table {
		got := abi.Methods[test.method].String()
		if got != test.expectation {
			t.Errorf("expected string to be %s, got %s", test.expectation, got)
		}
	}
}
