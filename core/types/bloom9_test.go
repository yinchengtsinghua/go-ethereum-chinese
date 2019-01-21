
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

package types

import (
	"math/big"
	"testing"
)

func TestBloom(t *testing.T) {
	positive := []string{
		"testtest",
		"test",
		"hallo",
		"other",
	}
	negative := []string{
		"tes",
		"lo",
	}

	var bloom Bloom
	for _, data := range positive {
		bloom.Add(new(big.Int).SetBytes([]byte(data)))
	}

	for _, data := range positive {
		if !bloom.TestBytes([]byte(data)) {
			t.Error("expected", data, "to test true")
		}
	}
	for _, data := range negative {
		if bloom.TestBytes([]byte(data)) {
			t.Error("did not expect", data, "to test true")
		}
	}
}

/*
进口（
 “测试”

 “github.com/ethereum/go-ethereum/core/state”
）

func测试bloom9（t*testing.t）
 测试用例：=[]字节（“测试测试”）
 bin：=logsbloom（[]state.log_
  测试用例，[]字节[]字节（“hellohello”），零，
 }。字节（）
 res：=bloomlookup（bin，测试用例）

 如果！RES{
  t.errorf（“Bloom查找失败”）
 }
}


func测试地址（t*testing.t）
 块：=&block
 block.coinbase=common.hex2bytes（“2234AE42D6DD7384BC8584E50419EA3AC75B83F”）。
 fmt.printf（“%x\n”，crypto.keccak256（block.coinbase））。

 bin:=创建Bloom（块）
 fmt.printf（“bin=%x\n”，common.leftpaddbytes（bin，64））。
}
**/

