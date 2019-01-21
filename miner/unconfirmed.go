
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

package miner

import (
	"container/ring"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

//未确认的块集使用chainretriever来验证
//挖掘块是否为规范链的一部分。
type chainRetriever interface {
//GetHeaderByNumber检索与块号关联的规范头。
	GetHeaderByNumber(number uint64) *types.Header

//GetBlockByNumber检索与块号关联的规范块。
	GetBlockByNumber(number uint64) *types.Block
}

//unconfirmedBlock是关于本地挖掘块的一小部分元数据集合。
//它被放入一个未确认的集合中，用于规范链包含跟踪。
type unconfirmedBlock struct {
	index uint64
	hash  common.Hash
}

//unconfirmedBlocks实现数据结构以维护本地挖掘的块
//尚未达到足够的成熟度，无法保证连锁经营。它是
//当先前挖掘的块被挖掘时，矿工用来向用户提供日志。
//有一个足够高的保证不会被重新排列出规范链。
type unconfirmedBlocks struct {
chain  chainRetriever //通过区块链验证规范状态
depth  uint           //丢弃以前块的深度
blocks *ring.Ring     //阻止信息以允许规范链交叉检查
lock   sync.RWMutex   //防止字段并发访问
}

//NewUnconfirmedBlocks返回新的数据结构以跟踪当前未确认的块。
func newUnconfirmedBlocks(chain chainRetriever, depth uint) *unconfirmedBlocks {
	return &unconfirmedBlocks{
		chain: chain,
		depth: depth,
	}
}

//insert向未确认的块集添加新的块。
func (set *unconfirmedBlocks) Insert(index uint64, hash common.Hash) {
//如果在当地开采了一个新的矿块，就要把足够旧的矿块移开。
	set.Shift(index)

//将新项创建为其自己的环
	item := ring.New(1)
	item.Value = &unconfirmedBlock{
		index: index,
		hash:  hash,
	}
//设置为初始环或附加到结尾
	set.lock.Lock()
	defer set.lock.Unlock()

	if set.blocks == nil {
		set.blocks = item
	} else {
		set.blocks.Move(-1).Link(item)
	}
//显示一个日志，供用户通知未确认的新挖掘块
	log.Info("🔨 mined potential block", "number", index, "hash", hash)
}

//SHIFT从集合中删除所有未确认的块，这些块超过未确认的集合深度
//允许，对照标准链检查它们是否包含或过时。
//报告。
func (set *unconfirmedBlocks) Shift(height uint64) {
	set.lock.Lock()
	defer set.lock.Unlock()

	for set.blocks != nil {
//检索下一个未确认的块，如果太新则中止
		next := set.blocks.Value.(*unconfirmedBlock)
		if next.index+uint64(set.depth) > height {
			break
		}
//块似乎超出深度允许，检查规范状态
		header := set.chain.GetHeaderByNumber(next.index)
		switch {
		case header == nil:
			log.Warn("Failed to retrieve header of mined block", "number", next.index, "hash", next.hash)
		case header.Hash() == next.hash:
			log.Info("🔗 block reached canonical chain", "number", next.index, "hash", next.hash)
		default:
//块不规范，请检查是否有叔叔或丢失的块
			included := false
			for number := next.index; !included && number < next.index+uint64(set.depth) && number <= height; number++ {
				if block := set.chain.GetBlockByNumber(number); block != nil {
					for _, uncle := range block.Uncles() {
						if uncle.Hash() == next.hash {
							included = true
							break
						}
					}
				}
			}
			if included {
				log.Info("⑂ block became an uncle", "number", next.index, "hash", next.hash)
			} else {
				log.Info("😱 block lost", "number", next.index, "hash", next.hash)
			}
		}
//把木块从环里拿出来
		if set.blocks.Value == set.blocks.Next().Value {
			set.blocks = nil
		} else {
			set.blocks = set.blocks.Move(-1)
			set.blocks.Unlink(1)
			set.blocks = set.blocks.Move(1)
		}
	}
}
