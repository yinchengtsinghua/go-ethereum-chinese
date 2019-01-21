
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

//+构建GouuZZ

package bitutil

import "bytes"

//Fuzz实现了Go-Fuzz引信方法来测试各种编码方法
//调用。
func Fuzz(data []byte) int {
	if len(data) == 0 {
		return -1
	}
	if data[0]%2 == 0 {
		return fuzzEncode(data[1:])
	}
	return fuzzDecode(data[1:])
}

//Fuzzencode实现了一种go-fuzz引信方法来测试位集编码和
//解码算法。
func fuzzEncode(data []byte) int {
	proc, _ := bitsetDecodeBytes(bitsetEncodeBytes(data), len(data))
	if !bytes.Equal(data, proc) {
		panic("content mismatch")
	}
	return 0
}

//fuzzdecode实现了一种go-fuzz引信方法来测试位解码和
//重新编码算法。
func fuzzDecode(data []byte) int {
	blob, err := bitsetDecodeBytes(data, 1024)
	if err != nil {
		return 0
	}
	if comp := bitsetEncodeBytes(blob); !bytes.Equal(comp, data) {
		panic("content mismatch")
	}
	return 0
}
