
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

package bloombits

import (
	"errors"

	"github.com/ethereum/go-ethereum/core/types"
)

var (
//如果用户尝试添加更多Bloom筛选器，则返回errSectionOutofBounds
//批处理的可用空间不足，或者如果尝试检索超过容量。
	errSectionOutOfBounds = errors.New("section out of bounds")

//如果用户尝试检索指定的
//比容量大一点。
	errBloomBitOutOfBounds = errors.New("bloom bit out of bounds")
)

//发电机接收许多布卢姆滤波器并生成旋转的布卢姆位
//用于批量过滤。
type Generator struct {
blooms   [types.BloomBitLength][]byte //每比特匹配的旋转花束
sections uint                         //要一起批处理的节数
nextSec  uint                         //添加花束时要设置的下一节
}

//NewGenerator创建一个旋转的Bloom Generator，它可以迭代填充
//批量布卢姆过滤器的钻头。
func NewGenerator(sections uint) (*Generator, error) {
	if sections%8 != 0 {
		return nil, errors.New("section count not multiple of 8")
	}
	b := &Generator{sections: sections}
	for i := 0; i < types.BloomBitLength; i++ {
		b.blooms[i] = make([]byte, sections/8)
	}
	return b, nil
}

//addbloom接受一个bloom过滤器并设置相应的位列
//在记忆中。
func (b *Generator) AddBloom(index uint, bloom types.Bloom) error {
//确保我们添加的布卢姆过滤器不会超过我们的容量
	if b.nextSec >= b.sections {
		return errSectionOutOfBounds
	}
	if b.nextSec != index {
		return errors.New("bloom filter with unexpected index")
	}
//旋转花束并插入我们的收藏
	byteIndex := b.nextSec / 8
	bitMask := byte(1) << byte(7-b.nextSec%8)

	for i := 0; i < types.BloomBitLength; i++ {
		bloomByteIndex := types.BloomByteLength - 1 - i/8
		bloomBitMask := byte(1) << byte(i%8)

		if (bloom[bloomByteIndex] & bloomBitMask) != 0 {
			b.blooms[i][byteIndex] |= bitMask
		}
	}
	b.nextSec++

	return nil
}

//位集返回属于给定位索引的位向量。
//花开了。
func (b *Generator) Bitset(idx uint) ([]byte, error) {
	if b.nextSec != b.sections {
		return nil, errors.New("bloom not fully generated yet")
	}
	if idx >= types.BloomBitLength {
		return nil, errBloomBitOutOfBounds
	}
	return b.blooms[idx], nil
}
