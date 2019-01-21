
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
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

//chainindexerbackend定义在中处理链段所需的方法
//并将段结果写入数据库。这些可以
//用于创建筛选器bloom或chts。
type ChainIndexerBackend interface {
//重置启动新链段的处理，可能终止
//任何部分完成的操作（如果是REORG）。
	Reset(ctx context.Context, section uint64, prevHead common.Hash) error

//在链段的下一个收割台上进行加工。呼叫者
//将确保头的顺序。
	Process(ctx context.Context, header *types.Header) error

//提交完成节元数据并将其存储到数据库中。
	Commit() error
}

//ChainIndexerChain接口用于将索引器连接到区块链
type ChainIndexerChain interface {
//当前头检索最新的本地已知头。
	CurrentHeader() *types.Header

//subscribeChainHeadEvent订阅新的头段通知。
	SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription
}

//链式索引器对
//规范链（如blooombits和cht结构）。链索引器是
//通过事件系统通过启动
//Goroutine中的ChainHeadEventLoop。
//
//还可以添加使用父级输出的子链索引器
//节索引器。这些子索引器仅接收新的头通知
//在完成整个部分之后，或者在回滚的情况下，
//影响已完成的节。
type ChainIndexer struct {
chainDb  ethdb.Database      //链接数据库以索引来自的数据
indexDb  ethdb.Database      //要写入索引元数据的数据库的前缀表视图
backend  ChainIndexerBackend //生成索引数据内容的后台处理器
children []*ChainIndexer     //将链更新级联到的子索引器

active    uint32          //标记事件循环是否已启动
update    chan struct{}   //应处理邮件头的通知通道
quit      chan chan error //退出频道以删除正在运行的Goroutines
	ctx       context.Context
	ctxCancel func()

sectionSize uint64 //要处理的单个链段中的块数
confirmsReq uint64 //处理已完成段之前的确认数

storedSections uint64 //成功编入数据库的节数
knownSections  uint64 //已知完整的节数（按块）
cascadedHead   uint64 //层叠到子索引器的上一个已完成节的块号

checkpointSections uint64      //检查站覆盖的区段数
checkpointHead     common.Hash //检查站所属科长

throttling time.Duration //磁盘限制以防止大量升级占用资源

	log  log.Logger
	lock sync.RWMutex
}

//NewChainIndexer创建一个新的链索引器以在其上进行后台处理
//经过一定数量的确认之后，给定大小的链段。
//Throttling参数可用于防止数据库不稳定。
func NewChainIndexer(chainDb, indexDb ethdb.Database, backend ChainIndexerBackend, section, confirm uint64, throttling time.Duration, kind string) *ChainIndexer {
	c := &ChainIndexer{
		chainDb:     chainDb,
		indexDb:     indexDb,
		backend:     backend,
		update:      make(chan struct{}, 1),
		quit:        make(chan chan error),
		sectionSize: section,
		confirmsReq: confirm,
		throttling:  throttling,
		log:         log.New("type", kind),
	}
//初始化与数据库相关的字段并启动更新程序
	c.loadValidSections()
	c.ctx, c.ctxCancel = context.WithCancel(context.Background())

	go c.updateLoop()

	return c
}

//添加检查点添加检查点。从未加工过截面和链条
//在此点之前不可用。索引器假定
//后端有足够的可用信息来处理后续部分。
//
//注意：knownsections==0，storedsections==checkpointsections直到
//同步到达检查点
func (c *ChainIndexer) AddCheckpoint(section uint64, shead common.Hash) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.checkpointSections = section + 1
	c.checkpointHead = shead

	if section < c.storedSections {
		return
	}
	c.setSectionHead(section, shead)
	c.setValidSections(section + 1)
}

//Start创建一个goroutine以将链头事件馈送到索引器中
//级联后台处理。孩子们不需要开始，他们
//父母会通知他们新的活动。
func (c *ChainIndexer) Start(chain ChainIndexerChain) {
	events := make(chan ChainHeadEvent, 10)
	sub := chain.SubscribeChainHeadEvent(events)

	go c.eventLoop(chain.CurrentHeader(), events, sub)
}

//关闭索引器的所有goroutine并返回任何错误
//这可能发生在内部。
func (c *ChainIndexer) Close() error {
	var errs []error

	c.ctxCancel()

//关闭主更新循环
	errc := make(chan error)
	c.quit <- errc
	if err := <-errc; err != nil {
		errs = append(errs, err)
	}
//如果需要，请关闭辅助事件循环
	if atomic.LoadUint32(&c.active) != 0 {
		c.quit <- errc
		if err := <-errc; err != nil {
			errs = append(errs, err)
		}
	}
//关闭所有子项
	for _, child := range c.children {
		if err := child.Close(); err != nil {
			errs = append(errs, err)
		}
	}
//返回任何失败
	switch {
	case len(errs) == 0:
		return nil

	case len(errs) == 1:
		return errs[0]

	default:
		return fmt.Errorf("%v", errs)
	}
}

//EventLoop是索引器的辅助-可选-事件循环，仅
//已启动，以便最外部的索引器将链头事件推送到处理中
//排队。
func (c *ChainIndexer) eventLoop(currentHeader *types.Header, events chan ChainHeadEvent, sub event.Subscription) {
//将链索引器标记为活动，需要额外拆卸
	atomic.StoreUint32(&c.active, 1)

	defer sub.Unsubscribe()

//启动初始的新head事件以开始任何未完成的处理
	c.newHead(currentHeader.Number.Uint64(), false)

	var (
		prevHeader = currentHeader
		prevHash   = currentHeader.Hash()
	)
	for {
		select {
		case errc := <-c.quit:
//链索引器终止，报告无故障并中止
			errc <- nil
			return

		case ev, ok := <-events:
//收到新事件，确保不是零（关闭）并更新
			if !ok {
				errc := <-c.quit
				errc <- nil
				return
			}
			header := ev.Block.Header()
			if header.ParentHash != prevHash {
//如果需要，重新组合到公共祖先（可能不存在于光同步模式中，请跳过重新组合）
//托多（卡拉贝尔，兹费尔福迪）：这似乎有点脆弱，我们能明确地检测到这个病例吗？

				if rawdb.ReadCanonicalHash(c.chainDb, prevHeader.Number.Uint64()) != prevHash {
					if h := rawdb.FindCommonAncestor(c.chainDb, prevHeader, header); h != nil {
						c.newHead(h.Number.Uint64(), true)
					}
				}
			}
			c.newHead(header.Number.Uint64(), false)

			prevHeader, prevHash = header, header.Hash()
		}
	}
}

//newhead通知索引器有关新链头和/或重新排序的信息。
func (c *ChainIndexer) newHead(head uint64, reorg bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

//如果发生了REORG，则在该点之前使所有部分无效
	if reorg {
//将已知的节号恢复到REORG点
		known := head / c.sectionSize
		stored := known
		if known < c.checkpointSections {
			known = 0
		}
		if stored < c.checkpointSections {
			stored = c.checkpointSections
		}
		if known < c.knownSections {
			c.knownSections = known
		}
//将存储的部分从数据库还原到REORG点
		if stored < c.storedSections {
			c.setValidSections(stored)
		}
//将新的头编号更新到最终确定的节结尾并通知子级
		head = known * c.sectionSize

		if head < c.cascadedHead {
			c.cascadedHead = head
			for _, child := range c.children {
				child.newHead(c.cascadedHead, true)
			}
		}
		return
	}
//无REORG，计算新已知部分的数量，如果足够高则更新
	var sections uint64
	if head >= c.confirmsReq {
		sections = (head + 1 - c.confirmsReq) / c.sectionSize
		if sections < c.checkpointSections {
			sections = 0
		}
		if sections > c.knownSections {
			if c.knownSections < c.checkpointSections {
//同步已到达检查点，请验证分区头
				syncedHead := rawdb.ReadCanonicalHash(c.chainDb, c.checkpointSections*c.sectionSize-1)
				if syncedHead != c.checkpointHead {
					c.log.Error("Synced chain does not match checkpoint", "number", c.checkpointSections*c.sectionSize-1, "expected", c.checkpointHead, "synced", syncedHead)
					return
				}
			}
			c.knownSections = sections

			select {
			case c.update <- struct{}{}:
			default:
			}
		}
	}
}

//updateLoop是推动链段的索引器的主要事件循环
//进入处理后端。
func (c *ChainIndexer) updateLoop() {
	var (
		updating bool
		updated  time.Time
	)

	for {
		select {
		case errc := <-c.quit:
//链索引器终止，报告无故障并中止
			errc <- nil
			return

		case <-c.update:
//节头已完成（或回滚），请更新索引
			c.lock.Lock()
			if c.knownSections > c.storedSections {
//定期向用户打印升级日志消息
				if time.Since(updated) > 8*time.Second {
					if c.knownSections > c.storedSections+1 {
						updating = true
						c.log.Info("Upgrading chain index", "percentage", c.storedSections*100/c.knownSections)
					}
					updated = time.Now()
				}
//缓存当前节计数和头以允许解锁互斥体
				section := c.storedSections
				var oldHead common.Hash
				if section > 0 {
					oldHead = c.SectionHead(section - 1)
				}
//在后台处理新定义的节
				c.lock.Unlock()
				newHead, err := c.processSection(section, oldHead)
				if err != nil {
					select {
					case <-c.ctx.Done():
						<-c.quit <- nil
						return
					default:
					}
					c.log.Error("Section processing failed", "error", err)
				}
				c.lock.Lock()

//如果处理成功且没有发生重新排序，则标记该节已完成
				if err == nil && oldHead == c.SectionHead(section-1) {
					c.setSectionHead(section, newHead)
					c.setValidSections(section + 1)
					if c.storedSections == c.knownSections && updating {
						updating = false
						c.log.Info("Finished upgrading chain index")
					}
					c.cascadedHead = c.storedSections*c.sectionSize - 1
					for _, child := range c.children {
						c.log.Trace("Cascading chain index update", "head", c.cascadedHead)
						child.newHead(c.cascadedHead, false)
					}
				} else {
//如果处理失败，在进一步通知之前不要重试
					c.log.Debug("Chain index processing failed", "section", section, "err", err)
					c.knownSections = c.storedSections
				}
			}
//如果还有其他部分需要处理，请重新安排
			if c.knownSections > c.storedSections {
				time.AfterFunc(c.throttling, func() {
					select {
					case c.update <- struct{}{}:
					default:
					}
				})
			}
			c.lock.Unlock()
		}
	}
}

//processSection通过调用后端函数来处理整个部分，而
//确保通过的收割台的连续性。因为链互斥体不是
//在处理过程中，连续性可以通过一个长的REORG来打破，其中
//case函数返回时出错。
func (c *ChainIndexer) processSection(section uint64, lastHead common.Hash) (common.Hash, error) {
	c.log.Trace("Processing new chain section", "section", section)

//复位和部分处理

	if err := c.backend.Reset(c.ctx, section, lastHead); err != nil {
		c.setValidSections(0)
		return common.Hash{}, err
	}

	for number := section * c.sectionSize; number < (section+1)*c.sectionSize; number++ {
		hash := rawdb.ReadCanonicalHash(c.chainDb, number)
		if hash == (common.Hash{}) {
			return common.Hash{}, fmt.Errorf("canonical block #%d unknown", number)
		}
		header := rawdb.ReadHeader(c.chainDb, hash, number)
		if header == nil {
			return common.Hash{}, fmt.Errorf("block #%d [%x…] not found", number, hash[:4])
		} else if header.ParentHash != lastHead {
			return common.Hash{}, fmt.Errorf("chain reorged during section processing")
		}
		if err := c.backend.Process(c.ctx, header); err != nil {
			return common.Hash{}, err
		}
		lastHead = header.Hash()
	}
	if err := c.backend.Commit(); err != nil {
		return common.Hash{}, err
	}
	return lastHead, nil
}

//区段返回索引器维护的已处理区段数。
//以及关于最后一个为潜在规范索引的头的信息
//验证。
func (c *ChainIndexer) Sections() (uint64, uint64, common.Hash) {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.storedSections, c.storedSections*c.sectionSize - 1, c.SectionHead(c.storedSections - 1)
}

//AddChildIndexer添加了一个子链索引器，该子链索引器可以使用此子链索引器的输出
func (c *ChainIndexer) AddChildIndexer(indexer *ChainIndexer) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.children = append(c.children, indexer)

//将所有挂起的更新层叠到新子级
	sections := c.storedSections
	if c.knownSections < sections {
//如果一个部分是“存储的”但不是“已知的”，那么它是一个没有
//可用的链数据，因此我们还不应该级联它
		sections = c.knownSections
	}
	if sections > 0 {
		indexer.newHead(sections*c.sectionSize-1, false)
	}
}

//loadvalidSections从索引数据库中读取有效节数。
//并且缓存进入本地状态。
func (c *ChainIndexer) loadValidSections() {
	data, _ := c.indexDb.Get([]byte("count"))
	if len(data) == 8 {
		c.storedSections = binary.BigEndian.Uint64(data)
	}
}

//setvalidSections将有效节数写入索引数据库
func (c *ChainIndexer) setValidSections(sections uint64) {
//设置数据库中有效节的当前数目
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], sections)
	c.indexDb.Put([]byte("count"), data[:])

//删除所有重新排序的部分，同时缓存有效数据
	for c.storedSections > sections {
		c.storedSections--
		c.removeSectionHead(c.storedSections)
	}
c.storedSections = sections //如果新的>旧的，则需要
}

//sectionhead从
//索引数据库。
func (c *ChainIndexer) SectionHead(section uint64) common.Hash {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], section)

	hash, _ := c.indexDb.Get(append([]byte("shead"), data[:]...))
	if len(hash) == len(common.Hash{}) {
		return common.BytesToHash(hash)
	}
	return common.Hash{}
}

//setSectionHead将已处理节的最后一个块哈希写入索引
//数据库。
func (c *ChainIndexer) setSectionHead(section uint64, hash common.Hash) {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], section)

	c.indexDb.Put(append([]byte("shead"), data[:]...), hash.Bytes())
}

//removeSectionHead从索引中删除对已处理节的引用
//数据库。
func (c *ChainIndexer) removeSectionHead(section uint64) {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], section)

	c.indexDb.Delete(append([]byte("shead"), data[:]...))
}
