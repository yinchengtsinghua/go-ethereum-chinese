
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

//包RAWDB包含低级别数据库访问器的集合。
package rawdb

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/metrics"
)

//下面的字段定义低级数据库模式前缀。
var (
//databaseverisionkey跟踪当前数据库版本。
	databaseVerisionKey = []byte("DatabaseVersion")

//HeadHeaderKey跟踪最新的已知头散列。
	headHeaderKey = []byte("LastHeader")

//headblockkey跟踪最新的已知完整块哈希。
	headBlockKey = []byte("LastBlock")

//HeadFastBlockKey在快速同步期间跟踪最新的已知不完整块的哈希。
	headFastBlockKey = []byte("LastFast")

//FastTrieProgressKey跟踪在快速同步期间导入的Trie条目数。
	fastTrieProgressKey = []byte("TrieSync")

//数据项前缀（使用单字节避免混合数据类型，避免使用“i”，用于索引）。
headerPrefix       = []byte("h") //headerPrefix+num（uint64 big endian）+hash->header
headerTDSuffix     = []byte("t") //headerPrefix+num（uint64 big endian）+hash+headerTsuffix->td
headerHashSuffix   = []byte("n") //headerPrefix+num（uint64 big endian）+headerHashSuffix->hash
headerNumberPrefix = []byte("H") //headerNumberPrefix+hash->num（uint64 big endian）

blockBodyPrefix     = []byte("b") //blockbodyprefix+num（uint64 big endian）+hash->block body
blockReceiptsPrefix = []byte("r") //blockReceiptsPrefix+num（uint64 big endian）+hash->block receipts

txLookupPrefix  = []byte("l") //txlookupprefix+hash->交易/收据查找元数据
bloomBitsPrefix = []byte("B") //bloombitsprefix+bit（uint16 big endian）+section（uint64 big endian）+hash->bloom位

preimagePrefix = []byte("secure-key-")      //preimagePrefix + hash -> preimage
configPrefix   = []byte("ethereum-config-") //数据库的配置前缀

//链索引前缀（使用'i`+单字节以避免混合数据类型）。
BloomBitsIndexPrefix = []byte("iB") //BloomBitsIndexPrefix是跟踪其进展的链表索引器的数据表。

	preimageCounter    = metrics.NewRegisteredCounter("db/preimage/total", nil)
	preimageHitCounter = metrics.NewRegisteredCounter("db/preimage/hits", nil)
)

//txLookupEntry是一个位置元数据，用于帮助查找
//只给出散列值的交易或收据。
type TxLookupEntry struct {
	BlockHash  common.Hash
	BlockIndex uint64
	Index      uint64
}

//encodeBlockNumber将块编号编码为big endian uint64
func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

//headerkey=headerprefix+num（uint64 big endian）+哈希
func headerKey(number uint64, hash common.Hash) []byte {
	return append(append(headerPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

//headertdkey=headerprefix+num（uint64 big endian）+hash+headertdsuffix
func headerTDKey(number uint64, hash common.Hash) []byte {
	return append(headerKey(number, hash), headerTDSuffix...)
}

//headerhashkey=headerprefix+num（uint64 big endian）+headerhashsuffix
func headerHashKey(number uint64) []byte {
	return append(append(headerPrefix, encodeBlockNumber(number)...), headerHashSuffix...)
}

//headerNumberKey=headerNumberPrefix+hash
func headerNumberKey(hash common.Hash) []byte {
	return append(headerNumberPrefix, hash.Bytes()...)
}

//blockBodyKey = blockBodyPrefix + num (uint64 big endian) + hash
func blockBodyKey(number uint64, hash common.Hash) []byte {
	return append(append(blockBodyPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

//blockReceiptskey=blockReceiptsPrefix+num（uint64 big endian）+哈希
func blockReceiptsKey(number uint64, hash common.Hash) []byte {
	return append(append(blockReceiptsPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

//txLookupKey=txLookupPrefix+哈希
func txLookupKey(hash common.Hash) []byte {
	return append(txLookupPrefix, hash.Bytes()...)
}

//bloomBitsKey = bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) + hash
func bloomBitsKey(bit uint, section uint64, hash common.Hash) []byte {
	key := append(append(bloomBitsPrefix, make([]byte, 10)...), hash.Bytes()...)

	binary.BigEndian.PutUint16(key[1:], uint16(bit))
	binary.BigEndian.PutUint64(key[3:], section)

	return key
}

//preImageKey=preImagePrefix+哈希
func preimageKey(hash common.Hash) []byte {
	return append(preimagePrefix, hash.Bytes()...)
}

//configkey=configPrefix+哈希
func configKey(hash common.Hash) []byte {
	return append(configPrefix, hash.Bytes()...)
}
