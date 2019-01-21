
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

package eth

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/bitutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
)

const (
//BloomServiceThreads是以太坊全局使用的Goroutine数。
//实例到服务BloomBits查找所有正在运行的筛选器。
	bloomServiceThreads = 16

//BloomFilterThreads是每个筛选器本地使用的goroutine数，用于
//将请求多路传输到全局服务goroutine。
	bloomFilterThreads = 3

//BloomRetrievalBatch是要服务的最大Bloom位检索数。
//一批。
	bloomRetrievalBatch = 16

//BloomRetrievalWait是等待足够的Bloom位请求的最长时间。
//累积请求整个批（避免滞后）。
	bloomRetrievalWait = time.Duration(0)
)

//StartBloomHandlers启动一批Goroutine以接受BloomBit数据库
//retrievals from possibly a range of filters and serving the data to satisfy.
func (eth *Ethereum) startBloomHandlers(sectionSize uint64) {
	for i := 0; i < bloomServiceThreads; i++ {
		go func() {
			for {
				select {
				case <-eth.shutdownChan:
					return

				case request := <-eth.bloomRequests:
					task := <-request
					task.Bitsets = make([][]byte, len(task.Sections))
					for i, section := range task.Sections {
						head := rawdb.ReadCanonicalHash(eth.chainDb, (section+1)*sectionSize-1)
						if compVector, err := rawdb.ReadBloomBits(eth.chainDb, task.Bit, section, head); err == nil {
							if blob, err := bitutil.DecompressBytes(compVector, int(sectionSize/8)); err == nil {
								task.Bitsets[i] = blob
							} else {
								task.Error = err
							}
						} else {
							task.Error = err
						}
					}
					request <- task
				}
			}
		}()
	}
}

const (
//BloomThrottling是处理两个连续索引之间的等待时间。
//部分。它在链升级期间很有用，可以防止磁盘过载。
	bloomThrottling = 100 * time.Millisecond
)

//BloomIndexer实现core.chainIndexer，建立旋转的BloomBits索引
//对于以太坊头段Bloom过滤器，允许快速过滤。
type BloomIndexer struct {
size    uint64               //要为其生成bloombits的节大小
db      ethdb.Database       //要将索引数据和元数据写入的数据库实例
gen     *bloombits.Generator //发电机旋转盛开钻头，装入盛开指数
section uint64               //节是当前正在处理的节号
head    common.Hash          //head是最后处理的头的哈希值
}

//newbloomindexer返回一个链索引器，它为
//用于快速日志筛选的规范链。
func NewBloomIndexer(db ethdb.Database, size, confirms uint64) *core.ChainIndexer {
	backend := &BloomIndexer{
		db:   db,
		size: size,
	}
	table := ethdb.NewTable(db, string(rawdb.BloomBitsIndexPrefix))

	return core.NewChainIndexer(db, table, backend, size, confirms, bloomThrottling, "bloombits")
}

//reset实现core.chainindexerbackend，启动新的bloombits索引
//部分。
func (b *BloomIndexer) Reset(ctx context.Context, section uint64, lastSectionHead common.Hash) error {
	gen, err := bloombits.NewGenerator(uint(b.size))
	b.gen, b.section, b.head = gen, section, common.Hash{}
	return err
}

//进程实现了core.chainindexerbackend，将新头的bloom添加到
//索引。
func (b *BloomIndexer) Process(ctx context.Context, header *types.Header) error {
	b.gen.AddBloom(uint(header.Number.Uint64()-b.section*b.size), header.Bloom)
	b.head = header.Hash()
	return nil
}

//commit实现core.chainindexerbackend，完成bloom部分和
//把它写进数据库。
func (b *BloomIndexer) Commit() error {
	batch := b.db.NewBatch()
	for i := 0; i < types.BloomBitLength; i++ {
		bits, err := b.gen.Bitset(uint(i))
		if err != nil {
			return err
		}
		rawdb.WriteBloomBits(batch, uint(i), b.section, b.head, bitutil.CompressBytes(bits))
	}
	return batch.Write()
}
