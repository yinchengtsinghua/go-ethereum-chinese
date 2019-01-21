
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

package light

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/bitutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

//indexerconfig包括一组用于链索引器的配置。
type IndexerConfig struct {
//用于创建CHT的块频率。
	ChtSize uint64

//特殊的辅助字段表示服务器配置的客户端chtsize，否则表示服务器的chtsize。
	PairChtSize uint64

//生成/接受规范哈希帮助trie所需的确认数。
	ChtConfirms uint64

//用于创建新Bloom位的块频率。
	BloomSize uint64

//在认为一个大方坯段可能是最终的及其旋转的钻头之前所需的确认数量。
//计算。
	BloomConfirms uint64

//创建Bloomtrie的块频率。
	BloomTrieSize uint64

//生成/接受Bloom Trie所需的确认数。
	BloomTrieConfirms uint64
}

var (
//DefaultServerIndexerConfig将一组配置包装为服务器端的默认索引器配置。
	DefaultServerIndexerConfig = &IndexerConfig{
		ChtSize:           params.CHTFrequencyServer,
		PairChtSize:       params.CHTFrequencyClient,
		ChtConfirms:       params.HelperTrieProcessConfirmations,
		BloomSize:         params.BloomBitsBlocks,
		BloomConfirms:     params.BloomConfirms,
		BloomTrieSize:     params.BloomTrieFrequency,
		BloomTrieConfirms: params.HelperTrieProcessConfirmations,
	}
//default client indexer config将一组配置包装为客户端的默认索引器配置。
	DefaultClientIndexerConfig = &IndexerConfig{
		ChtSize:           params.CHTFrequencyClient,
		PairChtSize:       params.CHTFrequencyServer,
		ChtConfirms:       params.HelperTrieConfirmations,
		BloomSize:         params.BloomBitsBlocksClient,
		BloomConfirms:     params.HelperTrieConfirmations,
		BloomTrieSize:     params.BloomTrieFrequency,
		BloomTrieConfirms: params.HelperTrieConfirmations,
	}
//test server indexer config将一组配置包装为服务器端的测试索引器配置。
	TestServerIndexerConfig = &IndexerConfig{
		ChtSize:           64,
		PairChtSize:       512,
		ChtConfirms:       4,
		BloomSize:         64,
		BloomConfirms:     4,
		BloomTrieSize:     512,
		BloomTrieConfirms: 4,
	}
//test client indexer config将一组配置包装为客户端的测试索引器配置。
	TestClientIndexerConfig = &IndexerConfig{
		ChtSize:           512,
		PairChtSize:       64,
		ChtConfirms:       32,
		BloomSize:         512,
		BloomConfirms:     32,
		BloomTrieSize:     512,
		BloomTrieConfirms: 32,
	}
)

//TrustedCheckpoints将每个已知的检查点与它所属的链的Genesis哈希相关联
var trustedCheckpoints = map[common.Hash]*params.TrustedCheckpoint{
	params.MainnetGenesisHash: params.MainnetTrustedCheckpoint,
	params.TestnetGenesisHash: params.TestnetTrustedCheckpoint,
	params.RinkebyGenesisHash: params.RinkebyTrustedCheckpoint,
}

var (
	ErrNoTrustedCht       = errors.New("no trusted canonical hash trie")
	ErrNoTrustedBloomTrie = errors.New("no trusted bloom trie")
	ErrNoHeader           = errors.New("header not found")
chtPrefix             = []byte("chtRoot-") //chtprefix+chtnum（uint64 big endian）->trie根哈希
	ChtTablePrefix        = "cht-"
)

//chtnode结构以rlp编码格式存储在规范哈希trie中。
type ChtNode struct {
	Hash common.Hash
	Td   *big.Int
}

//getchtroot从数据库中读取与给定节关联的cht根
//请注意，SECTIONIDX是根据LES/1 CHT截面尺寸指定的。
func GetChtRoot(db ethdb.Database, sectionIdx uint64, sectionHead common.Hash) common.Hash {
	var encNumber [8]byte
	binary.BigEndian.PutUint64(encNumber[:], sectionIdx)
	data, _ := db.Get(append(append(chtPrefix, encNumber[:]...), sectionHead.Bytes()...))
	return common.BytesToHash(data)
}

//storechtroot将与给定节关联的cht根写入数据库
//请注意，SECTIONIDX是根据LES/1 CHT截面尺寸指定的。
func StoreChtRoot(db ethdb.Database, sectionIdx uint64, sectionHead, root common.Hash) {
	var encNumber [8]byte
	binary.BigEndian.PutUint64(encNumber[:], sectionIdx)
	db.Put(append(append(chtPrefix, encNumber[:]...), sectionHead.Bytes()...), root.Bytes())
}

//chindexerbackend实现core.chainindexerbackend。
type ChtIndexerBackend struct {
	diskdb, trieTable    ethdb.Database
	odr                  OdrBackend
	triedb               *trie.Database
	section, sectionSize uint64
	lastHash             common.Hash
	trie                 *trie.Trie
}

//newchtIndexer创建cht链索引器
func NewChtIndexer(db ethdb.Database, odr OdrBackend, size, confirms uint64) *core.ChainIndexer {
	trieTable := ethdb.NewTable(db, ChtTablePrefix)
	backend := &ChtIndexerBackend{
		diskdb:      db,
		odr:         odr,
		trieTable:   trieTable,
triedb:      trie.NewDatabaseWithCache(trieTable, 1), //使用一个很小的缓存只会降低内存
		sectionSize: size,
	}
	return core.NewChainIndexer(db, ethdb.NewTable(db, "chtIndex-"), backend, size, confirms, time.Millisecond*100, "cht")
}

//fetchMissingNodes尝试从
//ODR后端，以便能够添加新条目和计算后续根散列
func (c *ChtIndexerBackend) fetchMissingNodes(ctx context.Context, section uint64, root common.Hash) error {
	batch := c.trieTable.NewBatch()
	r := &ChtRequest{ChtRoot: root, ChtNum: section - 1, BlockNum: section*c.sectionSize - 1, Config: c.odr.IndexerConfig()}
	for {
		err := c.odr.Retrieve(ctx, r)
		switch err {
		case nil:
			r.Proof.Store(batch)
			return batch.Write()
		case ErrNoPeers:
//如果没有要服务的对等端，请稍后重试
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second * 10):
//保持在循环中再试一次
			}
		default:
			return err
		}
	}
}

//重置实现core.chainindexerbackend
func (c *ChtIndexerBackend) Reset(ctx context.Context, section uint64, lastSectionHead common.Hash) error {
	var root common.Hash
	if section > 0 {
		root = GetChtRoot(c.diskdb, section-1, lastSectionHead)
	}
	var err error
	c.trie, err = trie.New(root, c.triedb)

	if err != nil && c.odr != nil {
		err = c.fetchMissingNodes(ctx, section, root)
		if err == nil {
			c.trie, err = trie.New(root, c.triedb)
		}
	}

	c.section = section
	return err
}

//进程实现core.chainindexerbackend
func (c *ChtIndexerBackend) Process(ctx context.Context, header *types.Header) error {
	hash, num := header.Hash(), header.Number.Uint64()
	c.lastHash = hash

	td := rawdb.ReadTd(c.diskdb, hash, num)
	if td == nil {
		panic(nil)
	}
	var encNumber [8]byte
	binary.BigEndian.PutUint64(encNumber[:], num)
	data, _ := rlp.EncodeToBytes(ChtNode{hash, td})
	c.trie.Update(encNumber[:], data)
	return nil
}

//commit实现core.chainindexerbackend
func (c *ChtIndexerBackend) Commit() error {
	root, err := c.trie.Commit(nil)
	if err != nil {
		return err
	}
	c.triedb.Commit(root, false)

	if ((c.section+1)*c.sectionSize)%params.CHTFrequencyClient == 0 {
		log.Info("Storing CHT", "section", c.section*c.sectionSize/params.CHTFrequencyClient, "head", fmt.Sprintf("%064x", c.lastHash), "root", fmt.Sprintf("%064x", root))
	}
	StoreChtRoot(c.diskdb, c.section, c.lastHash, root)
	return nil
}

var (
bloomTriePrefix      = []byte("bltRoot-") //bloomtrieffix+bloomtrienum（uint64 big endian）->trie根哈希
	BloomTrieTablePrefix = "blt-"
)

//GetBloomTrieRoot从数据库中读取与给定节关联的BloomTrie根。
func GetBloomTrieRoot(db ethdb.Database, sectionIdx uint64, sectionHead common.Hash) common.Hash {
	var encNumber [8]byte
	binary.BigEndian.PutUint64(encNumber[:], sectionIdx)
	data, _ := db.Get(append(append(bloomTriePrefix, encNumber[:]...), sectionHead.Bytes()...))
	return common.BytesToHash(data)
}

//StoreBloomTrieRoot将与给定节关联的BloomTrie根写入数据库
func StoreBloomTrieRoot(db ethdb.Database, sectionIdx uint64, sectionHead, root common.Hash) {
	var encNumber [8]byte
	binary.BigEndian.PutUint64(encNumber[:], sectionIdx)
	db.Put(append(append(bloomTriePrefix, encNumber[:]...), sectionHead.Bytes()...), root.Bytes())
}

//BloomTrieIndexerBackend实现core.chainIndexerBackend
type BloomTrieIndexerBackend struct {
	diskdb, trieTable ethdb.Database
	triedb            *trie.Database
	odr               OdrBackend
	section           uint64
	parentSize        uint64
	size              uint64
	bloomTrieRatio    uint64
	trie              *trie.Trie
	sectionHeads      []common.Hash
}

//Newbloomtrieeindexer创建了一个bloomtrie链索引器
func NewBloomTrieIndexer(db ethdb.Database, odr OdrBackend, parentSize, size uint64) *core.ChainIndexer {
	trieTable := ethdb.NewTable(db, BloomTrieTablePrefix)
	backend := &BloomTrieIndexerBackend{
		diskdb:     db,
		odr:        odr,
		trieTable:  trieTable,
triedb:     trie.NewDatabaseWithCache(trieTable, 1), //使用一个很小的缓存只会降低内存
		parentSize: parentSize,
		size:       size,
	}
	backend.bloomTrieRatio = size / parentSize
	backend.sectionHeads = make([]common.Hash, backend.bloomTrieRatio)
	return core.NewChainIndexer(db, ethdb.NewTable(db, "bltIndex-"), backend, size, 0, time.Millisecond*100, "bloomtrie")
}

//fetchMissingNodes尝试从
//ODR后端，以便能够添加新条目和计算后续根散列
func (b *BloomTrieIndexerBackend) fetchMissingNodes(ctx context.Context, section uint64, root common.Hash) error {
	indexCh := make(chan uint, types.BloomBitLength)
	type res struct {
		nodes *NodeSet
		err   error
	}
	resCh := make(chan res, types.BloomBitLength)
	for i := 0; i < 20; i++ {
		go func() {
			for bitIndex := range indexCh {
				r := &BloomRequest{BloomTrieRoot: root, BloomTrieNum: section - 1, BitIdx: bitIndex, SectionIndexList: []uint64{section - 1}, Config: b.odr.IndexerConfig()}
				for {
					if err := b.odr.Retrieve(ctx, r); err == ErrNoPeers {
//如果没有要服务的对等端，请稍后重试
						select {
						case <-ctx.Done():
							resCh <- res{nil, ctx.Err()}
							return
						case <-time.After(time.Second * 10):
//保持在循环中再试一次
						}
					} else {
						resCh <- res{r.Proofs, err}
						break
					}
				}
			}
		}()
	}

	for i := uint(0); i < types.BloomBitLength; i++ {
		indexCh <- i
	}
	close(indexCh)
	batch := b.trieTable.NewBatch()
	for i := uint(0); i < types.BloomBitLength; i++ {
		res := <-resCh
		if res.err != nil {
			return res.err
		}
		res.nodes.Store(batch)
	}
	return batch.Write()
}

//重置实现core.chainindexerbackend
func (b *BloomTrieIndexerBackend) Reset(ctx context.Context, section uint64, lastSectionHead common.Hash) error {
	var root common.Hash
	if section > 0 {
		root = GetBloomTrieRoot(b.diskdb, section-1, lastSectionHead)
	}
	var err error
	b.trie, err = trie.New(root, b.triedb)
	if err != nil && b.odr != nil {
		err = b.fetchMissingNodes(ctx, section, root)
		if err == nil {
			b.trie, err = trie.New(root, b.triedb)
		}
	}
	b.section = section
	return err
}

//进程实现core.chainindexerbackend
func (b *BloomTrieIndexerBackend) Process(ctx context.Context, header *types.Header) error {
	num := header.Number.Uint64() - b.section*b.size
	if (num+1)%b.parentSize == 0 {
		b.sectionHeads[num/b.parentSize] = header.Hash()
	}
	return nil
}

//commit实现core.chainindexerbackend
func (b *BloomTrieIndexerBackend) Commit() error {
	var compSize, decompSize uint64

	for i := uint(0); i < types.BloomBitLength; i++ {
		var encKey [10]byte
		binary.BigEndian.PutUint16(encKey[0:2], uint16(i))
		binary.BigEndian.PutUint64(encKey[2:10], b.section)
		var decomp []byte
		for j := uint64(0); j < b.bloomTrieRatio; j++ {
			data, err := rawdb.ReadBloomBits(b.diskdb, i, b.section*b.bloomTrieRatio+j, b.sectionHeads[j])
			if err != nil {
				return err
			}
			decompData, err2 := bitutil.DecompressBytes(data, int(b.parentSize/8))
			if err2 != nil {
				return err2
			}
			decomp = append(decomp, decompData...)
		}
		comp := bitutil.CompressBytes(decomp)

		decompSize += uint64(len(decomp))
		compSize += uint64(len(comp))
		if len(comp) > 0 {
			b.trie.Update(encKey[:], comp)
		} else {
			b.trie.Delete(encKey[:])
		}
	}
	root, err := b.trie.Commit(nil)
	if err != nil {
		return err
	}
	b.triedb.Commit(root, false)

	sectionHead := b.sectionHeads[b.bloomTrieRatio-1]
	log.Info("Storing bloom trie", "section", b.section, "head", fmt.Sprintf("%064x", sectionHead), "root", fmt.Sprintf("%064x", root), "compression", float64(compSize)/float64(decompSize))
	StoreBloomTrieRoot(b.diskdb, b.section, sectionHead, root)
	return nil
}
