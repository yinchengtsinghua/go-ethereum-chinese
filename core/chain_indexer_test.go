
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

package core

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
)

//使用随机参数运行多个测试。
func TestChainIndexerSingle(t *testing.T) {
	for i := 0; i < 10; i++ {
		testChainIndexer(t, 1)
	}
}

//使用随机参数和不同数量的
//链后端。
func TestChainIndexerWithChildren(t *testing.T) {
	for i := 2; i < 8; i++ {
		testChainIndexer(t, i)
	}
}

//TestChainIndexer使用单个链索引器或
//多个后端。节大小和所需的确认计数参数
//是随机的。
func testChainIndexer(t *testing.T, count int) {
	db := ethdb.NewMemDatabase()
	defer db.Close()

//创建索引器链并确保它们都报告为空
	backends := make([]*testChainIndexBackend, count)
	for i := 0; i < count; i++ {
		var (
			sectionSize = uint64(rand.Intn(100) + 1)
			confirmsReq = uint64(rand.Intn(10))
		)
		backends[i] = &testChainIndexBackend{t: t, processCh: make(chan uint64)}
		backends[i].indexer = NewChainIndexer(db, ethdb.NewTable(db, string([]byte{byte(i)})), backends[i], sectionSize, confirmsReq, 0, fmt.Sprintf("indexer-%d", i))

		if sections, _, _ := backends[i].indexer.Sections(); sections != 0 {
			t.Fatalf("Canonical section count mismatch: have %v, want %v", sections, 0)
		}
		if i > 0 {
			backends[i-1].indexer.AddChildIndexer(backends[i].indexer)
		}
	}
defer backends[0].indexer.Close() //父索引器关闭子级
//通知ping根索引器有关新头或REORG的信息，然后
//如果节可处理，则处理块
	notify := func(headNum, failNum uint64, reorg bool) {
		backends[0].indexer.newHead(headNum, reorg)
		if reorg {
			for _, backend := range backends {
				headNum = backend.reorg(headNum)
				backend.assertSections()
			}
			return
		}
		var cascade bool
		for _, backend := range backends {
			headNum, cascade = backend.assertBlocks(headNum, failNum)
			if !cascade {
				break
			}
			backend.assertSections()
		}
	}
//inject将新的随机规范头直接插入数据库
	inject := func(number uint64) {
		header := &types.Header{Number: big.NewInt(int64(number)), Extra: big.NewInt(rand.Int63()).Bytes()}
		if number > 0 {
			header.ParentHash = rawdb.ReadCanonicalHash(db, number-1)
		}
		rawdb.WriteHeader(db, header)
		rawdb.WriteCanonicalHash(db, header.Hash(), number)
	}
//使用已存在的链启动索引器
	for i := uint64(0); i <= 100; i++ {
		inject(i)
	}
	notify(100, 100, false)

//逐个添加新块
	for i := uint64(101); i <= 1000; i++ {
		inject(i)
		notify(i, i, false)
	}
//做一次练习
	notify(500, 500, true)

//创建新的叉子
	for i := uint64(501); i <= 1000; i++ {
		inject(i)
		notify(i, i, false)
	}
	for i := uint64(1001); i <= 1500; i++ {
		inject(i)
	}
//处理可用块少于通知块的方案失败
	notify(2000, 1500, false)

//通知有关REORG的信息（如果在处理过程中发生，可能会导致丢失的块）
	notify(1500, 1500, true)

//创建新的叉子
	for i := uint64(1501); i <= 2000; i++ {
		inject(i)
		notify(i, i, false)
	}
}

//testchainindexbackend实现chainindexerbackend
type testChainIndexBackend struct {
	t                          *testing.T
	indexer                    *ChainIndexer
	section, headerCnt, stored uint64
	processCh                  chan uint64
}

//断言节验证链索引器的节数是否正确。
func (b *testChainIndexBackend) assertSections() {
//如果不匹配，继续尝试3秒钟
	var sections uint64
	for i := 0; i < 300; i++ {
		sections, _, _ = b.indexer.Sections()
		if sections == b.stored {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	b.t.Fatalf("Canonical section count mismatch: have %v, want %v", sections, b.stored)
}

//断言块需要在新块到达后进行处理调用。如果
//failnum<headnum，然后我们将模拟发生REORG的场景
//在处理开始并且一个部分的处理失败之后。
func (b *testChainIndexBackend) assertBlocks(headNum, failNum uint64) (uint64, bool) {
	var sections uint64
	if headNum >= b.indexer.confirmsReq {
		sections = (headNum + 1 - b.indexer.confirmsReq) / b.indexer.sectionSize
		if sections > b.stored {
//预期已处理的块
			for expectd := b.stored * b.indexer.sectionSize; expectd < sections*b.indexer.sectionSize; expectd++ {
				if expectd > failNum {
//在处理开始后回滚，不需要更多的处理调用
//等待更新完成，以确保处理实际失败
					var updating bool
					for i := 0; i < 300; i++ {
						b.indexer.lock.Lock()
						updating = b.indexer.knownSections > b.indexer.storedSections
						b.indexer.lock.Unlock()
						if !updating {
							break
						}
						time.Sleep(10 * time.Millisecond)
					}
					if updating {
						b.t.Fatalf("update did not finish")
					}
					sections = expectd / b.indexer.sectionSize
					break
				}
				select {
				case <-time.After(10 * time.Second):
					b.t.Fatalf("Expected processed block #%d, got nothing", expectd)
				case processed := <-b.processCh:
					if processed != expectd {
						b.t.Errorf("Expected processed block #%d, got #%d", expectd, processed)
					}
				}
			}
			b.stored = sections
		}
	}
	if b.stored == 0 {
		return 0, false
	}
	return b.stored*b.indexer.sectionSize - 1, true
}

func (b *testChainIndexBackend) reorg(headNum uint64) uint64 {
	firstChanged := headNum / b.indexer.sectionSize
	if firstChanged < b.stored {
		b.stored = firstChanged
	}
	return b.stored * b.indexer.sectionSize
}

func (b *testChainIndexBackend) Reset(ctx context.Context, section uint64, prevHead common.Hash) error {
	b.section = section
	b.headerCnt = 0
	return nil
}

func (b *testChainIndexBackend) Process(ctx context.Context, header *types.Header) error {
	b.headerCnt++
	if b.headerCnt > b.indexer.sectionSize {
		b.t.Error("Processing too many headers")
	}
//t.processch<-header.number.uint64（）。
	select {
	case <-time.After(10 * time.Second):
		b.t.Fatal("Unexpected call to Process")
	case b.processCh <- header.Number.Uint64():
	}
	return nil
}

func (b *testChainIndexBackend) Commit() error {
	if b.headerCnt != b.indexer.sectionSize {
		b.t.Error("Not enough headers processed")
	}
	return nil
}
