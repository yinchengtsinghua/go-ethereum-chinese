
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2012 Go作者。版权所有。
//此源代码的使用受BSD样式的控制
//可以在许可文件中找到的许可证。

package bn256

import (
	"math/big"
)

func bigFromBase10(s string) *big.Int {
	n, _ := new(big.Int).SetString(s, 10)
	return n
}

//u是确定质点的bn参数：1868033³。
var u = bigFromBase10("4965661367192848881")

//order是G_和G癓中的元素数：36U_+36U³+18U²+6U+1。
var Order = bigFromBase10("21888242871839275222246405745257275088548364400416034343698204186575808495617")

//P是一个基本字段：36U_+36U³+24U²+6U+1。
var P = bigFromBase10("21888242871839275222246405745257275088696311157297823662689037894645226208583")

//p2是p，表示为64位小尾数字。
var p2 = [4]uint64{0x3c208c16d87cfd47, 0x97816a916871ca8d, 0xb85045b68181585d, 0x30644e72e131a029}

//np是p的负倒数，mod 2^256。
var np = [4]uint64{0x87d20782e4866389, 0x9ede7d651eca6ac9, 0xd8afcbd01833da80, 0xf57a22b791888c6b}

//RN1是r^1，其中r=2^256 mod p.
var rN1 = &gfP{0xed84884a014afa37, 0xeb2022850278edf8, 0xcf63e9cfb74492d9, 0x2e67157159e5c639}

//r2是r^2，其中r=2^256 mod p。
var r2 = &gfP{0xf32cfc5b538afa89, 0xb5e71911d44501fb, 0x47ab1eff0a417ff6, 0x06d89f71cab8351f}

//r3是r^3，其中r=2^256 mod p。
var r3 = &gfP{0xb1cd6dafda1530df, 0x62f210e6a7283db6, 0xef7f0b0c0ada0afb, 0x20fd6e902d592544}

//xitopmins1over6是ξ^（（p-1）/6），其中ξ=i+9。
var xiToPMinus1Over6 = &gfP2{gfP{0xa222ae234c492d72, 0xd00f02a4565de15b, 0xdc2ff3a253dfc926, 0x10a75716b3899551}, gfP{0xaf9ba69633144907, 0xca6b1d7387afb78a, 0x11bded5ef08a2087, 0x02f34d751a1f3a7c}}

//xitopmins1over3是ξ^（（p-1）/3），其中ξ=i+9。
var xiToPMinus1Over3 = &gfP2{gfP{0x6e849f1ea0aa4757, 0xaa1c7b6d89f89141, 0xb6e713cdfae0ca3a, 0x26694fbb4e82ebc3}, gfP{0xb5773b104563ab30, 0x347f91c8a9aa6454, 0x7a007127242e0991, 0x1956bcd8118214ec}}

//xitopmins1over2是ξ^（（p-1）/2），其中ξ=i+9。
var xiToPMinus1Over2 = &gfP2{gfP{0xa1d77ce45ffe77c7, 0x07affd117826d1db, 0x6d16bd27bb7edc6b, 0x2c87200285defecc}, gfP{0xe4bbdd0c2936b629, 0xbb30f162e133bacb, 0x31a9d1b6f9645366, 0x253570bea500f8dd}}

//xitopsquaredminus1 over3是ξ^（（p²-1）/3），其中ξ=i+9。
var xiToPSquaredMinus1Over3 = &gfP{0x3350c88e13e80b9c, 0x7dce557cdb5e56b9, 0x6001b4b8b615564a, 0x2682e617020217e0}

//xito2squaredminus2 over3是ξ^（（2p²-2）/3），其中ξ=i+9（单位的三次根，mod p）。
var xiTo2PSquaredMinus2Over3 = &gfP{0x71930c11d782e155, 0xa6bb947cffbe3323, 0xaa303344d4741444, 0x2c3b3f0d26594943}

//xitopsquaredminus1 over6是ξ^（（1p²-1）/6），其中ξ=i+9（1的立方根，mod p）。
var xiToPSquaredMinus1Over6 = &gfP{0xca8d800500fa1bf2, 0xf0c5d61468b39769, 0x0e201271ad0d4418, 0x04290f65bad856e6}

//xito2pMinus2over3是ξ^（（2p-2）/3），其中ξ=i+9。
var xiTo2PMinus2Over3 = &gfP2{gfP{0x5dddfd154bd8c949, 0x62cb29a5a4445b60, 0x37bc870a0c7dd2b9, 0x24830a9d3171f0fd}, gfP{0x7361d77f843abe92, 0xa5bb2bd3273411fb, 0x9c941f314b3e2399, 0x15df9cddbb9fd3ec}}
