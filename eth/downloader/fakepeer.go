
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

package downloader

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
)

//FakePeer是一个模拟下载程序对等机，在本地数据库实例上运行。
//而不是实际的活动节点。它对测试和实现很有用
//从现有本地数据库同步命令。
type FakePeer struct {
	id string
	db ethdb.Database
	hc *core.HeaderChain
	dl *Downloader
}

//newfakepeer用给定的数据源创建一个新的模拟下载器对等。
func NewFakePeer(id string, db ethdb.Database, hc *core.HeaderChain, dl *Downloader) *FakePeer {
	return &FakePeer{id: id, db: db, hc: hc, dl: dl}
}

//head实现downloader.peer，返回当前head哈希和数字
//最著名的标题。
func (p *FakePeer) Head() (common.Hash, *big.Int) {
	header := p.hc.CurrentHeader()
	return header.Hash(), header.Number
}

//requestHeadersByHash实现downloader.peer，返回一批头
//由源哈希和关联的查询参数定义。
func (p *FakePeer) RequestHeadersByHash(hash common.Hash, amount int, skip int, reverse bool) error {
	var (
		headers []*types.Header
		unknown bool
	)
	for !unknown && len(headers) < amount {
		origin := p.hc.GetHeaderByHash(hash)
		if origin == nil {
			break
		}
		number := origin.Number.Uint64()
		headers = append(headers, origin)
		if reverse {
			for i := 0; i <= skip; i++ {
				if header := p.hc.GetHeader(hash, number); header != nil {
					hash = header.ParentHash
					number--
				} else {
					unknown = true
					break
				}
			}
		} else {
			var (
				current = origin.Number.Uint64()
				next    = current + uint64(skip) + 1
			)
			if header := p.hc.GetHeaderByNumber(next); header != nil {
				if p.hc.GetBlockHashesFromHash(header.Hash(), uint64(skip+1))[skip] == hash {
					hash = header.Hash()
				} else {
					unknown = true
				}
			} else {
				unknown = true
			}
		}
	}
	p.dl.DeliverHeaders(p.id, headers)
	return nil
}

//requestHeadersByNumber实现downloader.peer，返回一批头
//由原点编号和关联的查询参数定义。
func (p *FakePeer) RequestHeadersByNumber(number uint64, amount int, skip int, reverse bool) error {
	var (
		headers []*types.Header
		unknown bool
	)
	for !unknown && len(headers) < amount {
		origin := p.hc.GetHeaderByNumber(number)
		if origin == nil {
			break
		}
		if reverse {
			if number >= uint64(skip+1) {
				number -= uint64(skip + 1)
			} else {
				unknown = true
			}
		} else {
			number += uint64(skip + 1)
		}
		headers = append(headers, origin)
	}
	p.dl.DeliverHeaders(p.id, headers)
	return nil
}

//请求体实现downloader.peer，返回一批块体
//对应于指定的块散列。
func (p *FakePeer) RequestBodies(hashes []common.Hash) error {
	var (
		txs    [][]*types.Transaction
		uncles [][]*types.Header
	)
	for _, hash := range hashes {
		block := rawdb.ReadBlock(p.db, hash, *p.hc.GetBlockNumber(hash))

		txs = append(txs, block.Transactions())
		uncles = append(uncles, block.Uncles())
	}
	p.dl.DeliverBodies(p.id, txs, uncles)
	return nil
}

//requestReceipts实现downloader.peer，返回一批事务
//与指定的块哈希相对应的收据。
func (p *FakePeer) RequestReceipts(hashes []common.Hash) error {
	var receipts [][]*types.Receipt
	for _, hash := range hashes {
		receipts = append(receipts, rawdb.ReadReceipts(p.db, hash, *p.hc.GetBlockNumber(hash)))
	}
	p.dl.DeliverReceipts(p.id, receipts)
	return nil
}

//RequestNodeData implements downloader.Peer, returning a batch of state trie
//与指定的trie散列对应的节点。
func (p *FakePeer) RequestNodeData(hashes []common.Hash) error {
	var data [][]byte
	for _, hash := range hashes {
		if entry, err := p.db.Get(hash.Bytes()); err == nil {
			data = append(data, entry)
		}
	}
	p.dl.DeliverNodeData(p.id, data)
	return nil
}
