
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
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func hexAddr(a string) common.Address { return common.BytesToAddress(common.FromHex(a)) }
func mixAddr(a string) (*common.MixedcaseAddress, error) {
	return common.NewMixedcaseAddressFromString(a)
}
func toHexBig(h string) hexutil.Big {
	b := big.NewInt(0).SetBytes(common.FromHex(h))
	return hexutil.Big(*b)
}
func toHexUint(h string) hexutil.Uint64 {
	b := big.NewInt(0).SetBytes(common.FromHex(h))
	return hexutil.Uint64(b.Uint64())
}
func dummyTxArgs(t txtestcase) *SendTxArgs {
	to, _ := mixAddr(t.to)
	from, _ := mixAddr(t.from)
	n := toHexUint(t.n)
	gas := toHexUint(t.g)
	gasPrice := toHexBig(t.gp)
	value := toHexBig(t.value)
	var (
		data, input *hexutil.Bytes
	)
	if t.d != "" {
		a := hexutil.Bytes(common.FromHex(t.d))
		data = &a
	}
	if t.i != "" {
		a := hexutil.Bytes(common.FromHex(t.i))
		input = &a

	}
	return &SendTxArgs{
		From:     *from,
		To:       to,
		Value:    value,
		Nonce:    n,
		GasPrice: gasPrice,
		Gas:      gas,
		Data:     data,
		Input:    input,
	}
}

type txtestcase struct {
	from, to, n, g, gp, value, d, i string
	expectErr                       bool
	numMessages                     int
}

func TestValidator(t *testing.T) {
	var (
//使用空数据库，对ABI特定的东西还有其他测试
		db, _ = NewEmptyAbiDB()
		v     = NewValidator(db)
	)
	testcases := []txtestcase{
//校验和无效
		{from: "000000000000000000000000000000000000dead", to: "000000000000000000000000000000000000dead",
			n: "0x01", g: "0x20", gp: "0x40", value: "0x01", numMessages: 1},
//有效的0x0000000000000000000000000000000标题
		{from: "000000000000000000000000000000000000dead", to: "0x000000000000000000000000000000000000dEaD",
			n: "0x01", g: "0x20", gp: "0x40", value: "0x01", numMessages: 0},
//输入和数据冲突
		{from: "000000000000000000000000000000000000dead", to: "0x000000000000000000000000000000000000dEaD",
			n: "0x01", g: "0x20", gp: "0x40", value: "0x01", d: "0x01", i: "0x02", expectErr: true},
//无法分析数据
		{from: "000000000000000000000000000000000000dead", to: "0x000000000000000000000000000000000000dEaD",
			n: "0x01", g: "0x20", gp: "0x40", value: "0x01", d: "0x0102", numMessages: 1},
//无法分析（输入时）数据
		{from: "000000000000000000000000000000000000dead", to: "0x000000000000000000000000000000000000dEaD",
			n: "0x01", g: "0x20", gp: "0x40", value: "0x01", i: "0x0102", numMessages: 1},
//发送到0
		{from: "000000000000000000000000000000000000dead", to: "0x0000000000000000000000000000000000000000",
			n: "0x01", g: "0x20", gp: "0x40", value: "0x01", numMessages: 1},
//创建空合同（无值）
		{from: "000000000000000000000000000000000000dead", to: "",
			n: "0x01", g: "0x20", gp: "0x40", value: "0x00", numMessages: 1},
//创建空合同（带值）
		{from: "000000000000000000000000000000000000dead", to: "",
			n: "0x01", g: "0x20", gp: "0x40", value: "0x01", expectErr: true},
//用于创建的小负载
		{from: "000000000000000000000000000000000000dead", to: "",
			n: "0x01", g: "0x20", gp: "0x40", value: "0x01", d: "0x01", numMessages: 1},
	}
	for i, test := range testcases {
		msgs, err := v.ValidateTransaction(dummyTxArgs(test), nil)
		if err == nil && test.expectErr {
			t.Errorf("Test %d, expected error", i)
			for _, msg := range msgs.Messages {
				fmt.Printf("* %s: %s\n", msg.Typ, msg.Message)
			}
		}
		if err != nil && !test.expectErr {
			t.Errorf("Test %d, unexpected error: %v", i, err)
		}
		if err == nil {
			got := len(msgs.Messages)
			if got != test.numMessages {
				for _, msg := range msgs.Messages {
					fmt.Printf("* %s: %s\n", msg.Typ, msg.Message)
				}
				t.Errorf("Test %d, expected %d messages, got %d", i, test.numMessages, got)
			} else {
//调试打印输出，稍后删除
				for _, msg := range msgs.Messages {
					fmt.Printf("* [%d] %s: %s\n", i, msg.Typ, msg.Message)
				}
				fmt.Println()
			}
		}
	}
}

func TestPasswordValidation(t *testing.T) {
	testcases := []struct {
		pw         string
		shouldFail bool
	}{
		{"test", true},
		{"testtest\xbd\xb2\x3d\xbc\x20\xe2\x8c\x98", true},
		{"placeOfInterest⌘", true},
		{"password\nwith\nlinebreak", true},
		{"password\twith\vtabs", true},
//OK密码
		{"password WhichIsOk", false},
		{"passwordOk!@#$%^&*()", false},
		{"12301203123012301230123012", false},
	}
	for _, test := range testcases {
		err := ValidatePasswordFormat(test.pw)
		if err == nil && test.shouldFail {
			t.Errorf("password '%v' should fail validation", test.pw)
		} else if err != nil && !test.shouldFail {

			t.Errorf("password '%v' shound not fail validation, but did: %v", test.pw, err)
		}
	}
}
