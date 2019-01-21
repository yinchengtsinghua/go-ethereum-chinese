
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

package storage

import (
	"strconv"
	"testing"
)

//testNexibility用显式验证邻近函数
//表驱动测试中的值。它高度依赖于
//maxpo常量，它将案例验证为maxpo=32。
func TestProximity(t *testing.T) {
//来自base2编码字符串的整数
	bx := func(s string) uint8 {
		i, err := strconv.ParseUint(s, 2, 8)
		if err != nil {
			t.Fatal(err)
		}
		return uint8(i)
	}
//根据最大采购订单调整预期料仓
	limitPO := func(po uint8) uint8 {
		if po > MaxPO {
			return MaxPO
		}
		return po
	}
	base := []byte{bx("00000000"), bx("00000000"), bx("00000000"), bx("00000000")}
	for _, tc := range []struct {
		addr []byte
		po   uint8
	}{
		{
			addr: base,
			po:   MaxPO,
		},
		{
			addr: []byte{bx("10000000"), bx("00000000"), bx("00000000"), bx("00000000")},
			po:   limitPO(0),
		},
		{
			addr: []byte{bx("01000000"), bx("00000000"), bx("00000000"), bx("00000000")},
			po:   limitPO(1),
		},
		{
			addr: []byte{bx("00100000"), bx("00000000"), bx("00000000"), bx("00000000")},
			po:   limitPO(2),
		},
		{
			addr: []byte{bx("00010000"), bx("00000000"), bx("00000000"), bx("00000000")},
			po:   limitPO(3),
		},
		{
			addr: []byte{bx("00001000"), bx("00000000"), bx("00000000"), bx("00000000")},
			po:   limitPO(4),
		},
		{
			addr: []byte{bx("00000100"), bx("00000000"), bx("00000000"), bx("00000000")},
			po:   limitPO(5),
		},
		{
			addr: []byte{bx("00000010"), bx("00000000"), bx("00000000"), bx("00000000")},
			po:   limitPO(6),
		},
		{
			addr: []byte{bx("00000001"), bx("00000000"), bx("00000000"), bx("00000000")},
			po:   limitPO(7),
		},
		{
			addr: []byte{bx("00000000"), bx("10000000"), bx("00000000"), bx("00000000")},
			po:   limitPO(8),
		},
		{
			addr: []byte{bx("00000000"), bx("01000000"), bx("00000000"), bx("00000000")},
			po:   limitPO(9),
		},
		{
			addr: []byte{bx("00000000"), bx("00100000"), bx("00000000"), bx("00000000")},
			po:   limitPO(10),
		},
		{
			addr: []byte{bx("00000000"), bx("00010000"), bx("00000000"), bx("00000000")},
			po:   limitPO(11),
		},
		{
			addr: []byte{bx("00000000"), bx("00001000"), bx("00000000"), bx("00000000")},
			po:   limitPO(12),
		},
		{
			addr: []byte{bx("00000000"), bx("00000100"), bx("00000000"), bx("00000000")},
			po:   limitPO(13),
		},
		{
			addr: []byte{bx("00000000"), bx("00000010"), bx("00000000"), bx("00000000")},
			po:   limitPO(14),
		},
		{
			addr: []byte{bx("00000000"), bx("00000001"), bx("00000000"), bx("00000000")},
			po:   limitPO(15),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("10000000"), bx("00000000")},
			po:   limitPO(16),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("01000000"), bx("00000000")},
			po:   limitPO(17),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("00100000"), bx("00000000")},
			po:   limitPO(18),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("00010000"), bx("00000000")},
			po:   limitPO(19),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("00001000"), bx("00000000")},
			po:   limitPO(20),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("00000100"), bx("00000000")},
			po:   limitPO(21),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("00000010"), bx("00000000")},
			po:   limitPO(22),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("00000001"), bx("00000000")},
			po:   limitPO(23),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("00000000"), bx("10000000")},
			po:   limitPO(24),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("00000000"), bx("01000000")},
			po:   limitPO(25),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("00000000"), bx("00100000")},
			po:   limitPO(26),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("00000000"), bx("00010000")},
			po:   limitPO(27),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("00000000"), bx("00001000")},
			po:   limitPO(28),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("00000000"), bx("00000100")},
			po:   limitPO(29),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("00000000"), bx("00000010")},
			po:   limitPO(30),
		},
		{
			addr: []byte{bx("00000000"), bx("00000000"), bx("00000000"), bx("00000001")},
			po:   limitPO(31),
		},
	} {
		got := uint8(Proximity(base, tc.addr))
		if got != tc.po {
			t.Errorf("got %v bin, want %v", got, tc.po)
		}
	}
}
