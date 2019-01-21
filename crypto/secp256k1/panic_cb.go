
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Jeffrey Wilcke、Felix Lange、Gustav Simonsson。版权所有。
//此源代码的使用受BSD样式许可证的控制，该许可证可在
//许可证文件。

package secp256k1

import "C"
import "unsafe"

//将libsecp256k1内部故障转换为
//恢复性恐慌。

//出口secp256k1gopanicilegal
func secp256k1GoPanicIllegal(msg *C.char, data unsafe.Pointer) {
	panic("illegal argument: " + C.GoString(msg))
}

//导出secp256k1gopanicerror
func secp256k1GoPanicError(msg *C.char, data unsafe.Pointer) {
	panic("internal error: " + C.GoString(msg))
}
