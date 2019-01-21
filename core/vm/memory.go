
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Go Ethereum作者
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

package vm

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common/math"
)

//内存为以太坊虚拟机实现了一个简单的内存模型。
type Memory struct {
	store       []byte
	lastGasCost uint64
}

//new memory返回新的内存模型。
func NewMemory() *Memory {
	return &Memory{}
}

//将“设置偏移量+大小”设置为“值”
func (m *Memory) Set(offset, size uint64, value []byte) {
//偏移量可能大于0，大小等于0。这是因为
//当大小为零（no-op）时，CalcMemsize（common.go）可能返回0。
	if size > 0 {
//存储长度不能小于偏移量+大小。
//在设置内存之前，应调整存储的大小
		if offset+size > uint64(len(m.store)) {
			panic("invalid memory: store empty")
		}
		copy(m.store[offset:offset+size], value)
	}
}

//set32将从偏移量开始的32个字节设置为val值，用零左填充到
//32字节。
func (m *Memory) Set32(offset uint64, val *big.Int) {
//存储长度不能小于偏移量+大小。
//在设置内存之前，应调整存储的大小
	if offset+32 > uint64(len(m.store)) {
		panic("invalid memory: store empty")
	}
//把记忆区归零
	copy(m.store[offset:offset+32], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
//填写相关位
	math.ReadBits(val, m.store[offset:offset+32])
}

//调整大小将内存大小调整为
func (m *Memory) Resize(size uint64) {
	if uint64(m.Len()) < size {
		m.store = append(m.store, make([]byte, size-uint64(m.Len()))...)
	}
}

//get返回偏移量+作为新切片的大小
func (m *Memory) Get(offset, size int64) (cpy []byte) {
	if size == 0 {
		return nil
	}

	if len(m.store) > int(offset) {
		cpy = make([]byte, size)
		copy(cpy, m.store[offset:offset+size])

		return
	}

	return
}

//getptr返回偏移量+大小
func (m *Memory) GetPtr(offset, size int64) []byte {
	if size == 0 {
		return nil
	}

	if len(m.store) > int(offset) {
		return m.store[offset : offset+size]
	}

	return nil
}

//len返回背衬片的长度
func (m *Memory) Len() int {
	return len(m.store)
}

//数据返回备份切片
func (m *Memory) Data() []byte {
	return m.store
}

//打印转储内存的内容。
func (m *Memory) Print() {
	fmt.Printf("### mem %d bytes ###\n", len(m.store))
	if len(m.store) > 0 {
		addr := 0
		for i := 0; i+32 <= len(m.store); i += 32 {
			fmt.Printf("%03d: % x\n", addr, m.store[i:i+32])
			addr++
		}
	} else {
		fmt.Println("-- empty --")
	}
	fmt.Println("####################")
}
