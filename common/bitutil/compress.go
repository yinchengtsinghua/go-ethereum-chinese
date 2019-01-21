
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

package bitutil

import "errors"

var (
//如果引用的字节
//位集头溢出输入数据。
	errMissingData = errors.New("missing bytes on input")

//如果没有使用所有字节，则从解压返回errunreferenceddata。
//在对输入数据进行解压缩之后。
	errUnreferencedData = errors.New("extra bytes on input")

//如果位集头具有
//定义的比特数多于可用的目标缓冲区空间数。
	errExceededTarget = errors.New("target data size exceeded")

//如果中引用了数据字节，则从解压缩返回errZeroContent。
//位集头实际上是一个零字节。
	errZeroContent = errors.New("zero byte in input content")
)

//由compressBytes和compressBytes实现的压缩算法是
//针对包含大量零字节的稀疏输入数据进行了优化。减压
//需要解压数据长度的知识。
//
//压缩工程如下：
//
//如果数据只包含零，
//compressbytes（data）==nil
//否则，如果len（data）<=1，
//compressBytes（data）==数据
//否则：
//compressbytes（data）==附加（compressbytes（nonzerobitset（data）），nonzerobytes（data）…）
//哪里
//非零位集（data）是一个带有len（data）位（msb first）的位向量：
//非零位集（数据）[I/8]&&（1<（7-I%8））！=0，如果数据[i]！= 0
//len（非零位集（数据））==（len（数据）+7）/8
//非零字节（数据）包含相同顺序的非零字节数据

//compressBytes根据稀疏位集压缩输入字节片
//表示算法。如果结果大于原始输入，则不
//压缩完成。
func CompressBytes(data []byte) []byte {
	if out := bitsetEncodeBytes(data); len(out) < len(data) {
		return out
	}
	cpy := make([]byte, len(data))
	copy(cpy, data)
	return cpy
}

//bitsetEncodeBytes根据稀疏数据压缩输入字节片
//位集表示算法。
func bitsetEncodeBytes(data []byte) []byte {
//空切片压缩为零
	if len(data) == 0 {
		return nil
	}
//单字节片压缩为零或保留单字节
	if len(data) == 1 {
		if data[0] == 0 {
			return nil
		}
		return data
	}
//计算集合字节的位集，并收集非零字节
	nonZeroBitset := make([]byte, (len(data)+7)/8)
	nonZeroBytes := make([]byte, 0, len(data))

	for i, b := range data {
		if b != 0 {
			nonZeroBytes = append(nonZeroBytes, b)
			nonZeroBitset[i/8] |= 1 << byte(7-i%8)
		}
	}
	if len(nonZeroBytes) == 0 {
		return nil
	}
	return append(bitsetEncodeBytes(nonZeroBitset), nonZeroBytes...)
}

//解压缩字节用已知的目标大小解压缩数据。如果输入数据
//匹配目标的大小，这意味着在第一个压缩过程中没有进行压缩
//地点。
func DecompressBytes(data []byte, target int) ([]byte, error) {
	if len(data) > target {
		return nil, errExceededTarget
	}
	if len(data) == target {
		cpy := make([]byte, len(data))
		copy(cpy, data)
		return cpy, nil
	}
	return bitsetDecodeBytes(data, target)
}

//bitsetdecodebytes用已知的目标大小解压缩数据。
func bitsetDecodeBytes(data []byte, target int) ([]byte, error) {
	out, size, err := bitsetDecodePartialBytes(data, target)
	if err != nil {
		return nil, err
	}
	if size != len(data) {
		return nil, errUnreferencedData
	}
	return out, nil
}

//BitsetDecodePartialBytes以已知的目标大小解压缩数据，但确实如此
//不强制使用所有输入字节。除了减压
//输出，函数返回相应的压缩输入数据的长度
//因为输入片可能更长。
func bitsetDecodePartialBytes(data []byte, target int) ([]byte, int, error) {
//健全性检查0个目标以避免无限递归
	if target == 0 {
		return nil, 0, nil
	}
//处理零和单字节角情况
	decomp := make([]byte, target)
	if len(data) == 0 {
		return decomp, 0, nil
	}
	if target == 1 {
decomp[0] = data[0] //复制以避免引用输入切片
		if data[0] != 0 {
			return decomp, 1, nil
		}
		return decomp, 0, nil
	}
//解压集合字节的位集并分配非零字节
	nonZeroBitset, ptr, err := bitsetDecodePartialBytes(data, (target+7)/8)
	if err != nil {
		return nil, ptr, err
	}
	for i := 0; i < 8*len(nonZeroBitset); i++ {
		if nonZeroBitset[i/8]&(1<<byte(7-i%8)) != 0 {
//确保我们有足够的数据插入正确的插槽
			if ptr >= len(data) {
				return nil, 0, errMissingData
			}
			if i >= len(decomp) {
				return nil, 0, errExceededTarget
			}
//确保数据有效并推入插槽
			if data[ptr] == 0 {
				return nil, 0, errZeroContent
			}
			decomp[i] = data[ptr]
			ptr++
		}
	}
	return decomp, ptr, nil
}
