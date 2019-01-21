
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 P_ter Szil_gyi。版权所有。
//此源代码的使用受可以找到的BSD样式许可证的控制
//在许可证文件中。

//+建设！AMD64！ARM64

//包bn256在256位的barreto-naehrig曲线上实现了最佳的ate对。
package bn256

import "github.com/ethereum/go-ethereum/crypto/bn256/google"

//g1是一个抽象的循环群。零值适合用作
//操作的输出，但不能用作输入。
type G1 = bn256.G1

//g2是一个抽象的循环群。零值适合用作
//操作的输出，但不能用作输入。
type G2 = bn256.G2

//pairingcheck计算一组点的最佳ate对。
func PairingCheck(a []*G1, b []*G2) bool {
	return bn256.PairingCheck(a, b)
}
