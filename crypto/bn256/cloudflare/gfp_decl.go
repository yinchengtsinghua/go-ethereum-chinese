
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//+建立AMD64，！通用ARM64，！通用的

package bn256

//此文件包含特定于体系结构的转发声明
//这些函数的程序集实现，前提是它们存在。

import (
	"golang.org/x/sys/cpu"
)

//诺林特
var hasBMI2 = cpu.X86.HasBMI2

//逃走
func gfpNeg(c, a *gfP)

//逃走
func gfpAdd(c, a, b *gfP)

//逃走
func gfpSub(c, a, b *gfP)

//逃走
func gfpMul(c, a, b *gfP)
