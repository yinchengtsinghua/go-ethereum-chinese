
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

package clique

import (
	"bytes"
	"encoding/json"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	lru "github.com/hashicorp/golang-lru"
)

//投票代表授权签名人修改
//授权列表。
type Vote struct {
Signer    common.Address `json:"signer"`    //投票的授权签署人
Block     uint64         `json:"block"`     //投票所投的区号（过期旧票）
Address   common.Address `json:"address"`   //正在投票更改其授权的帐户
Authorize bool           `json:"authorize"` //是否授权或取消对投票帐户的授权
}

//计票是一种简单的计票方式，用来保持当前的计票结果。投票赞成
//反对这项提议并不算在内，因为它等同于不投票。
type Tally struct {
Authorize bool `json:"authorize"` //投票是授权还是踢某人
Votes     int  `json:"votes"`     //到目前为止想要通过提案的票数
}

//快照是在给定时间点上投票的授权状态。
type Snapshot struct {
config   *params.CliqueConfig //微调行为的一致引擎参数
sigcache *lru.ARCCache        //缓存最近的块签名以加快ecrecover

Number  uint64                      `json:"number"`  //创建快照的块号
Hash    common.Hash                 `json:"hash"`    //创建快照的块哈希
Signers map[common.Address]struct{} `json:"signers"` //此时的授权签名人集合
Recents map[uint64]common.Address   `json:"recents"` //垃圾邮件保护的最近签名者集
Votes   []*Vote                     `json:"votes"`   //按时间顺序投票的名单
Tally   map[common.Address]Tally    `json:"tally"`   //当前投票计数以避免重新计算
}

//SignerAscending实现排序接口，以允许对地址列表进行排序
type signersAscending []common.Address

func (s signersAscending) Len() int           { return len(s) }
func (s signersAscending) Less(i, j int) bool { return bytes.Compare(s[i][:], s[j][:]) < 0 }
func (s signersAscending) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

//NewSnapshot使用指定的启动参数创建新快照。这个
//方法不初始化最近的签名者集，因此仅当用于
//创世纪板块。
func newSnapshot(config *params.CliqueConfig, sigcache *lru.ARCCache, number uint64, hash common.Hash, signers []common.Address) *Snapshot {
	snap := &Snapshot{
		config:   config,
		sigcache: sigcache,
		Number:   number,
		Hash:     hash,
		Signers:  make(map[common.Address]struct{}),
		Recents:  make(map[uint64]common.Address),
		Tally:    make(map[common.Address]Tally),
	}
	for _, signer := range signers {
		snap.Signers[signer] = struct{}{}
	}
	return snap
}

//LoadSnapshot从数据库加载现有快照。
func loadSnapshot(config *params.CliqueConfig, sigcache *lru.ARCCache, db ethdb.Database, hash common.Hash) (*Snapshot, error) {
	blob, err := db.Get(append([]byte("clique-"), hash[:]...))
	if err != nil {
		return nil, err
	}
	snap := new(Snapshot)
	if err := json.Unmarshal(blob, snap); err != nil {
		return nil, err
	}
	snap.config = config
	snap.sigcache = sigcache

	return snap, nil
}

//存储将快照插入数据库。
func (s *Snapshot) store(db ethdb.Database) error {
	blob, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return db.Put(append([]byte("clique-"), s.Hash[:]...), blob)
}

//复制创建快照的深度副本，尽管不是单个投票。
func (s *Snapshot) copy() *Snapshot {
	cpy := &Snapshot{
		config:   s.config,
		sigcache: s.sigcache,
		Number:   s.Number,
		Hash:     s.Hash,
		Signers:  make(map[common.Address]struct{}),
		Recents:  make(map[uint64]common.Address),
		Votes:    make([]*Vote, len(s.Votes)),
		Tally:    make(map[common.Address]Tally),
	}
	for signer := range s.Signers {
		cpy.Signers[signer] = struct{}{}
	}
	for block, signer := range s.Recents {
		cpy.Recents[block] = signer
	}
	for address, tally := range s.Tally {
		cpy.Tally[address] = tally
	}
	copy(cpy.Votes, s.Votes)

	return cpy
}

//validvote返回在
//给定快照上下文（例如，不要尝试添加已授权的签名者）。
func (s *Snapshot) validVote(address common.Address, authorize bool) bool {
	_, signer := s.Signers[address]
	return (signer && !authorize) || (!signer && authorize)
}

//Cast在计票中添加了新的选票。
func (s *Snapshot) cast(address common.Address, authorize bool) bool {
//确保投票有意义
	if !s.validVote(address, authorize) {
		return false
	}
//将投票投到现有或新的计票中
	if old, ok := s.Tally[address]; ok {
		old.Votes++
		s.Tally[address] = old
	} else {
		s.Tally[address] = Tally{Authorize: authorize, Votes: 1}
	}
	return true
}

//uncast从计票中删除先前的投票。
func (s *Snapshot) uncast(address common.Address, authorize bool) bool {
//如果没有计票结果，那是悬而未决的投票，就投吧。
	tally, ok := s.Tally[address]
	if !ok {
		return false
	}
//确保我们只还原已计数的投票
	if tally.Authorize != authorize {
		return false
	}
//否则恢复投票
	if tally.Votes > 1 {
		tally.Votes--
		s.Tally[address] = tally
	} else {
		delete(s.Tally, address)
	}
	return true
}

//应用通过将给定的头应用于创建新的授权快照
//原来的那个。
func (s *Snapshot) apply(headers []*types.Header) (*Snapshot, error) {
//不允许传入清除器代码的头
	if len(headers) == 0 {
		return s, nil
	}
//健全性检查标题是否可以应用
	for i := 0; i < len(headers)-1; i++ {
		if headers[i+1].Number.Uint64() != headers[i].Number.Uint64()+1 {
			return nil, errInvalidVotingChain
		}
	}
	if headers[0].Number.Uint64() != s.Number+1 {
		return nil, errInvalidVotingChain
	}
//遍历头并创建新快照
	snap := s.copy()

	for _, header := range headers {
//删除检查点块上的所有投票
		number := header.Number.Uint64()
		if number%s.config.Epoch == 0 {
			snap.Votes = nil
			snap.Tally = make(map[common.Address]Tally)
		}
//从最近列表中删除最早的签名者，以允许其再次签名。
		if limit := uint64(len(snap.Signers)/2 + 1); number >= limit {
			delete(snap.Recents, number-limit)
		}
//解析授权密钥并检查签名者
		signer, err := ecrecover(header, s.sigcache)
		if err != nil {
			return nil, err
		}
		if _, ok := snap.Signers[signer]; !ok {
			return nil, errUnauthorizedSigner
		}
		for _, recent := range snap.Recents {
			if recent == signer {
				return nil, errRecentlySigned
			}
		}
		snap.Recents[number] = signer

//标题已授权，放弃签名者以前的任何投票
		for i, vote := range snap.Votes {
			if vote.Signer == signer && vote.Address == header.Coinbase {
//从缓存的计数中取消投票
				snap.uncast(vote.Address, vote.Authorize)

//取消按时间顺序排列的投票
				snap.Votes = append(snap.Votes[:i], snap.Votes[i+1:]...)
break //只允许一票
			}
		}
//统计签名者的新投票
		var authorize bool
		switch {
		case bytes.Equal(header.Nonce[:], nonceAuthVote):
			authorize = true
		case bytes.Equal(header.Nonce[:], nonceDropVote):
			authorize = false
		default:
			return nil, errInvalidVote
		}
		if snap.cast(header.Coinbase, authorize) {
			snap.Votes = append(snap.Votes, &Vote{
				Signer:    signer,
				Block:     number,
				Address:   header.Coinbase,
				Authorize: authorize,
			})
		}
//如果投票通过，则更新签名者列表
		if tally := snap.Tally[header.Coinbase]; tally.Votes > len(snap.Signers)/2 {
			if tally.Authorize {
				snap.Signers[header.Coinbase] = struct{}{}
			} else {
				delete(snap.Signers, header.Coinbase)

//签名者列表收缩，删除所有剩余的最近缓存
				if limit := uint64(len(snap.Signers)/2 + 1); number >= limit {
					delete(snap.Recents, number-limit)
				}
//放弃取消授权签名者所投的任何以前的票
				for i := 0; i < len(snap.Votes); i++ {
					if snap.Votes[i].Signer == header.Coinbase {
//从缓存的计数中取消投票
						snap.uncast(snap.Votes[i].Address, snap.Votes[i].Authorize)

//取消按时间顺序排列的投票
						snap.Votes = append(snap.Votes[:i], snap.Votes[i+1:]...)

						i--
					}
				}
			}
//放弃对刚刚更改的帐户的任何以前的投票
			for i := 0; i < len(snap.Votes); i++ {
				if snap.Votes[i].Address == header.Coinbase {
					snap.Votes = append(snap.Votes[:i], snap.Votes[i+1:]...)
					i--
				}
			}
			delete(snap.Tally, header.Coinbase)
		}
	}
	snap.Number += uint64(len(headers))
	snap.Hash = headers[len(headers)-1].Hash()

	return snap, nil
}

//签名者按升序检索授权签名者列表。
func (s *Snapshot) signers() []common.Address {
	sigs := make([]common.Address, 0, len(s.Signers))
	for sig := range s.Signers {
		sigs = append(sigs, sig)
	}
	sort.Sort(signersAscending(sigs))
	return sigs
}

//如果给定块高度的签名者依次是或不是，则Inturn返回。
func (s *Snapshot) inturn(number uint64, signer common.Address) bool {
	signers, offset := s.signers(), 0
	for offset < len(signers) && signers[offset] != signer {
		offset++
	}
	return (number % uint64(len(signers))) == uint64(offset)
}
