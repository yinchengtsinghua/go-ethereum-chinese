
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

//包bmt是基于hashsize段的简单非当前引用实现
//任意但固定的最大chunksize上的二进制merkle树哈希
//
//此实现不利用任何并行列表和使用
//内存远比需要的多，但很容易看出它是正确的。
//它可以用于生成用于优化实现的测试用例。
//在bmt_test.go中对引用散列器的正确性进行了额外的检查。
//＊TestFisher
//*测试bmthshercorrection函数
package bmt

import (
	"hash"
)

//refhasher是BMT的非优化易读参考实现
type RefHasher struct {
maxDataLength int       //c*hashsize，其中c=2^ceil（log2（count）），其中count=ceil（length/hashsize）
sectionLength int       //2＊尺寸
hasher        hash.Hash //基哈希函数（keccak256 sha3）
}

//NewRefHasher返回新的RefHasher
func NewRefHasher(hasher BaseHasherFunc, count int) *RefHasher {
	h := hasher()
	hashsize := h.Size()
	c := 2
	for ; c < count; c *= 2 {
	}
	return &RefHasher{
		sectionLength: 2 * hashsize,
		maxDataLength: c * hashsize,
		hasher:        h,
	}
}

//hash返回字节片的bmt哈希
//实现swarmhash接口
func (rh *RefHasher) Hash(data []byte) []byte {
//如果数据小于基长度（maxdatalength），我们将提供零填充。
	d := make([]byte, rh.maxDataLength)
	length := len(data)
	if length > rh.maxDataLength {
		length = rh.maxDataLength
	}
	copy(d, data[:length])
	return rh.hash(d, rh.maxDataLength)
}

//数据的长度maxdatalength=segmentsize*2^k
//哈希在给定切片的两半递归调用自身
//连接结果，并返回该结果的哈希值
//如果d的长度是2*SegmentSize，则只返回该节的哈希值。
func (rh *RefHasher) hash(data []byte, length int) []byte {
	var section []byte
	if length == rh.sectionLength {
//部分包含两个数据段（D）
		section = data
	} else {
//部分包含左右BMT子目录的哈希
//通过在数据的左半部分和右半部分递归调用哈希来计算
		length /= 2
		section = append(rh.hash(data[:length], length), rh.hash(data[length:], length)...)
	}
	rh.hasher.Reset()
	rh.hasher.Write(section)
	return rh.hasher.Sum(nil)
}
