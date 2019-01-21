
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

//包裹集团实施权威证明共识引擎。
package clique

import (
	"bytes"
	"errors"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	lru "github.com/hashicorp/golang-lru"
	"golang.org/x/crypto/sha3"
)

const (
checkpointInterval = 1024 //将投票快照保存到数据库之后的块数
inmemorySnapshots  = 128  //要保留在内存中的最近投票快照数
inmemorySignatures = 4096 //要保存在内存中的最近块签名数

wiggleTime = 500 * time.Millisecond //允许并发签名者的随机延迟（每个签名者）
)

//集团权威证明协议常数。
var (
epochLength = uint64(30000) //在其之后检查和重置挂起投票的默认块数

extraVanity = 32 //固定为签名者虚荣保留的额外数据前缀字节数
extraSeal   = 65 //固定为签名者密封保留的额外数据后缀字节数

nonceAuthVote = hexutil.MustDecode("0xffffffffffffffff") //要在添加新签名者时投票的Magic nonce编号
nonceDropVote = hexutil.MustDecode("0x0000000000000000") //要在删除签名者时投票的Magic nonce编号。

uncleHash = types.CalcUncleHash(nil) //作为叔叔，Keccak256（rlp（[]）在POW之外总是毫无意义的。

diffInTurn = big.NewInt(2) //阻止依次签名的困难
diffNoTurn = big.NewInt(1) //阻止错误签名的困难
)

//将块标记为无效的各种错误消息。这些应该是私人的
//防止在
//代码库，如果引擎被换出，则固有的中断。请把普通
//共识包中的错误类型。
var (
//当请求块的签名者列表时，返回errunknownblock。
//这不是本地区块链的一部分。
	errUnknownBlock = errors.New("unknown block")

//如果检查点/时代转换，则返回errInvalidCheckpoint受益人
//块的受益人设置为非零。
	errInvalidCheckpointBeneficiary = errors.New("beneficiary in checkpoint block non-zero")

//
//
	errInvalidVote = errors.New("vote nonce not 0x00..0 or 0xff..f")

//如果检查点/epoch转换块，则返回errInvalidCheckpointVote
//将投票当前设置为非零。
	errInvalidCheckpointVote = errors.New("vote nonce in checkpoint block non-zero")

//如果块的额外数据节短于
//32字节，这是存储签名者虚荣所必需的。
	errMissingVanity = errors.New("extra-data 32 byte vanity prefix missing")

//如果块的额外数据节似乎不存在，则返回errmissingsignature
//包含65字节的secp256k1签名。
	errMissingSignature = errors.New("extra-data 65 byte signature suffix missing")

//如果非检查点块中包含签名者数据，则返回errExtrasigners。
//它们的额外数据字段。
	errExtraSigners = errors.New("non-checkpoint block contains extra signer list")

//如果检查点块包含
//签名者列表无效（即不能被20字节整除）。
	errInvalidCheckpointSigners = errors.New("invalid signer list on checkpoint block")

//如果检查点块包含
//与本地节点计算的签名者不同的签名者列表。
	errMismatchingCheckpointSigners = errors.New("mismatching signer list on checkpoint block")

//如果块的mix digest为非零，则返回errInvalidMixDigest。
	errInvalidMixDigest = errors.New("non-zero mix digest")

//如果块包含非空的叔叔列表，则返回errInvalidUncleHash。
	errInvalidUncleHash = errors.New("non empty uncle hash")

//如果块的难度不是1或2，则返回errInvalid难度。
	errInvalidDifficulty = errors.New("invalid difficulty")

//如果块的难度与
//转动签名者。
	errWrongDifficulty = errors.New("wrong difficulty")

//如果块的时间戳低于，则返回errInvalidTimestamp
//上一个块的时间戳+最小块周期。
	ErrInvalidTimestamp = errors.New("invalid timestamp")

//如果尝试授权列表，则返回errInvalidVotingChain
//通过超出范围或不连续的标题进行修改。
	errInvalidVotingChain = errors.New("invalid voting chain")

//如果标题由非授权实体签名，则返回errUnauthorizedSigner。
	errUnauthorizedSigner = errors.New("unauthorized signer")

//如果标题由授权实体签名，则返回errrRecentlySigned
//最近已经签名的邮件头，因此暂时不允许。
	errRecentlySigned = errors.New("recently signed")
)

//signerfn是一个签名者回调函数，用于请求哈希由
//备用账户。
type SignerFn func(accounts.Account, []byte) ([]byte, error)

//sighash返回用作权限证明输入的哈希
//签署。它是除65字节签名之外的整个头的哈希
//包含在额外数据的末尾。
//
//注意，该方法要求额外数据至少为65字节，否则
//恐慌。这样做是为了避免意外使用这两个表单（存在签名
//或者不是），这可能会被滥用，从而为同一个头产生不同的散列。
func sigHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()

	rlp.Encode(hasher, []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
header.Extra[:len(header.Extra)-65], //是的，如果多余的太短，这会很恐慌的
		header.MixDigest,
		header.Nonce,
	})
	hasher.Sum(hash[:0])
	return hash
}

//ecrecover从签名的头中提取以太坊帐户地址。
func ecrecover(header *types.Header, sigcache *lru.ARCCache) (common.Address, error) {
//如果签名已经缓存，则返回
	hash := header.Hash()
	if address, known := sigcache.Get(hash); known {
		return address.(common.Address), nil
	}
//从头中检索签名额外数据
	if len(header.Extra) < extraSeal {
		return common.Address{}, errMissingSignature
	}
	signature := header.Extra[len(header.Extra)-extraSeal:]

//恢复公钥和以太坊地址
	pubkey, err := crypto.Ecrecover(sigHash(header).Bytes(), signature)
	if err != nil {
		return common.Address{}, err
	}
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])

	sigcache.Add(hash, signer)
	return signer, nil
}

//集团是权威的证明，共识引擎建议支持
//Ropsten攻击后的以太坊测试网。
type Clique struct {
config *params.CliqueConfig //共识引擎配置参数
db     ethdb.Database       //存储和检索快照检查点的数据库

recents    *lru.ARCCache //最近块的快照以加快重新排序
signatures *lru.ARCCache //加快开采速度的近期区块特征

proposals map[common.Address]bool //我们正在推动的最新提案清单

signer common.Address //签名密钥的以太坊地址
signFn SignerFn       //用于授权哈希的签名程序函数
lock   sync.RWMutex   //保护签名者字段

//以下字段仅用于测试
fakeDiff bool //跳过难度验证
}

//新创建的集团权威证明共识引擎
//签名者设置为用户提供的签名者。
func New(config *params.CliqueConfig, db ethdb.Database) *Clique {
//将所有缺少的共识参数设置为默认值
	conf := *config
	if conf.Epoch == 0 {
		conf.Epoch = epochLength
	}
//分配快照缓存并创建引擎
	recents, _ := lru.NewARC(inmemorySnapshots)
	signatures, _ := lru.NewARC(inmemorySignatures)

	return &Clique{
		config:     &conf,
		db:         db,
		recents:    recents,
		signatures: signatures,
		proposals:  make(map[common.Address]bool),
	}
}

//作者实现共识引擎，返回以太坊地址恢复
//从标题的额外数据部分的签名。
func (c *Clique) Author(header *types.Header) (common.Address, error) {
	return ecrecover(header, c.signatures)
}

//verifyheader检查头是否符合共识规则。
func (c *Clique) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
	return c.verifyHeader(chain, header, nil)
}

//VerifyHeaders类似于VerifyHeader，但会验证一批头。这个
//方法返回一个退出通道以中止操作，并返回一个结果通道以
//检索异步验证（顺序是输入切片的顺序）。
func (c *Clique) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for i, header := range headers {
			err := c.verifyHeader(chain, header, headers[:i])

			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results
}

//verifyheader检查一个header是否符合共识规则。
//调用者可以选择按一批父级（升序）传递以避免
//从数据库中查找。这对于并发验证很有用
//一批新的头文件。
func (c *Clique) verifyHeader(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {
	if header.Number == nil {
		return errUnknownBlock
	}
	number := header.Number.Uint64()

//不要浪费时间检查未来的街区
	if header.Time.Cmp(big.NewInt(time.Now().Unix())) > 0 {
		return consensus.ErrFutureBlock
	}
//检查点块需要强制零受益人
	checkpoint := (number % c.config.Epoch) == 0
	if checkpoint && header.Coinbase != (common.Address{}) {
		return errInvalidCheckpointBeneficiary
	}
//nonce必须是0x00..0或0xff..f，在检查点上强制使用零
	if !bytes.Equal(header.Nonce[:], nonceAuthVote) && !bytes.Equal(header.Nonce[:], nonceDropVote) {
		return errInvalidVote
	}
	if checkpoint && !bytes.Equal(header.Nonce[:], nonceDropVote) {
		return errInvalidCheckpointVote
	}
//检查额外数据是否包含虚荣和签名
	if len(header.Extra) < extraVanity {
		return errMissingVanity
	}
	if len(header.Extra) < extraVanity+extraSeal {
		return errMissingSignature
	}
//确保额外数据包含检查点上的签名者列表，但不包含其他数据。
	signersBytes := len(header.Extra) - extraVanity - extraSeal
	if !checkpoint && signersBytes != 0 {
		return errExtraSigners
	}
	if checkpoint && signersBytes%common.AddressLength != 0 {
		return errInvalidCheckpointSigners
	}
//确保混合摘要为零，因为我们当前没有分叉保护
	if header.MixDigest != (common.Hash{}) {
		return errInvalidMixDigest
	}
//确保该区块不包含任何在POA中无意义的叔叔。
	if header.UncleHash != uncleHash {
		return errInvalidUncleHash
	}
//确保块的难度有意义（此时可能不正确）
	if number > 0 {
		if header.Difficulty == nil || (header.Difficulty.Cmp(diffInTurn) != 0 && header.Difficulty.Cmp(diffNoTurn) != 0) {
			return errInvalidDifficulty
		}
	}
//如果所有检查都通过，则验证硬分叉的任何特殊字段
	if err := misc.VerifyForkHashes(chain.Config(), header, false); err != nil {
		return err
	}
//通过所有基本检查，验证级联字段
	return c.verifyCascadingFields(chain, header, parents)
}

//verifycascadingfields验证所有不独立的头字段，
//而是依赖于前一批头文件。呼叫者可以选择通过
//在一批家长中（升序），以避免从
//数据库。这对于同时验证一批新头文件很有用。
func (c *Clique) verifyCascadingFields(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {
//Genesis区块始终是有效的死胡同
	number := header.Number.Uint64()
	if number == 0 {
		return nil
	}
//确保块的时间戳与其父块的时间戳不太接近
	var parent *types.Header
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetHeader(header.ParentHash, number-1)
	}
	if parent == nil || parent.Number.Uint64() != number-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}
	if parent.Time.Uint64()+c.config.Period > header.Time.Uint64() {
		return ErrInvalidTimestamp
	}
//检索验证此头并缓存它所需的快照
	snap, err := c.snapshot(chain, number-1, header.ParentHash, parents)
	if err != nil {
		return err
	}
//如果该块是检查点块，请验证签名者列表
	if number%c.config.Epoch == 0 {
		signers := make([]byte, len(snap.Signers)*common.AddressLength)
		for i, signer := range snap.signers() {
			copy(signers[i*common.AddressLength:], signer[:])
		}
		extraSuffix := len(header.Extra) - extraSeal
		if !bytes.Equal(header.Extra[extraVanity:extraSuffix], signers) {
			return errMismatchingCheckpointSigners
		}
	}
//所有基本检查通过，确认密封并返回
	return c.verifySeal(chain, header, parents)
}

//快照在给定时间点检索授权快照。
func (c *Clique) snapshot(chain consensus.ChainReader, number uint64, hash common.Hash, parents []*types.Header) (*Snapshot, error) {
//在内存或磁盘上搜索快照以查找检查点
	var (
		headers []*types.Header
		snap    *Snapshot
	)
	for snap == nil {
//如果找到内存中的快照，请使用
		if s, ok := c.recents.Get(hash); ok {
			snap = s.(*Snapshot)
			break
		}
//如果可以找到磁盘上的检查点快照，请使用
		if number%checkpointInterval == 0 {
			if s, err := loadSnapshot(c.config, c.signatures, c.db, hash); err == nil {
				log.Trace("Loaded voting snapshot from disk", "number", number, "hash", hash)
				snap = s
				break
			}
		}
//如果我们在一个检查点块，拍一张快照
		if number == 0 || (number%c.config.Epoch == 0 && chain.GetHeaderByNumber(number-1) == nil) {
			checkpoint := chain.GetHeaderByNumber(number)
			if checkpoint != nil {
				hash := checkpoint.Hash()

				signers := make([]common.Address, (len(checkpoint.Extra)-extraVanity-extraSeal)/common.AddressLength)
				for i := 0; i < len(signers); i++ {
					copy(signers[i][:], checkpoint.Extra[extraVanity+i*common.AddressLength:])
				}
				snap = newSnapshot(c.config, c.signatures, number, hash, signers)
				if err := snap.store(c.db); err != nil {
					return nil, err
				}
				log.Info("Stored checkpoint snapshot to disk", "number", number, "hash", hash)
				break
			}
		}
//没有此头的快照，收集头并向后移动
		var header *types.Header
		if len(parents) > 0 {
//如果我们有明确的父母，从那里挑选（强制）
			header = parents[len(parents)-1]
			if header.Hash() != hash || header.Number.Uint64() != number {
				return nil, consensus.ErrUnknownAncestor
			}
			parents = parents[:len(parents)-1]
		} else {
//没有明确的父级（或不再存在），请访问数据库
			header = chain.GetHeader(hash, number)
			if header == nil {
				return nil, consensus.ErrUnknownAncestor
			}
		}
		headers = append(headers, header)
		number, hash = number-1, header.ParentHash
	}
//找到上一个快照，在其上应用任何挂起的头
	for i := 0; i < len(headers)/2; i++ {
		headers[i], headers[len(headers)-1-i] = headers[len(headers)-1-i], headers[i]
	}
	snap, err := snap.apply(headers)
	if err != nil {
		return nil, err
	}
	c.recents.Add(snap.Hash, snap)

//如果生成了新的检查点快照，请保存到磁盘
	if snap.Number%checkpointInterval == 0 && len(headers) > 0 {
		if err = snap.store(c.db); err != nil {
			return nil, err
		}
		log.Trace("Stored voting snapshot to disk", "number", snap.Number, "hash", snap.Hash)
	}
	return snap, err
}

//verifyuncles实现converse.engine，始终返回任何
//因为这个共识机制不允许叔叔。
func (c *Clique) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed")
	}
	return nil
}

//验证seal是否执行consension.engine，检查签名是否包含
//头部满足共识协议要求。
func (c *Clique) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	return c.verifySeal(chain, header, nil)
}

//verifyseal检查标题中包含的签名是否满足
//共识协议要求。该方法接受可选的父级列表
//还不属于本地区块链以生成快照的头段
//从…
func (c *Clique) verifySeal(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {
//验证不支持Genesis块
	number := header.Number.Uint64()
	if number == 0 {
		return errUnknownBlock
	}
//检索验证此头并缓存它所需的快照
	snap, err := c.snapshot(chain, number-1, header.ParentHash, parents)
	if err != nil {
		return err
	}

//解析授权密钥并检查签名者
	signer, err := ecrecover(header, c.signatures)
	if err != nil {
		return err
	}
	if _, ok := snap.Signers[signer]; !ok {
		return errUnauthorizedSigner
	}
	for seen, recent := range snap.Recents {
		if recent == signer {
//签名者在Recents中，只有当当前块不将其移出时才会失败。
			if limit := uint64(len(snap.Signers)/2 + 1); seen > number-limit {
				return errRecentlySigned
			}
		}
	}
//确保难度与签名人的转弯度相对应。
	if !c.fakeDiff {
		inturn := snap.inturn(header.Number.Uint64(), signer)
		if inturn && header.Difficulty.Cmp(diffInTurn) != 0 {
			return errWrongDifficulty
		}
		if !inturn && header.Difficulty.Cmp(diffNoTurn) != 0 {
			return errWrongDifficulty
		}
	}
	return nil
}

//准备执行共识。引擎，准备
//用于在顶部运行事务的标题。
func (c *Clique) Prepare(chain consensus.ChainReader, header *types.Header) error {
//如果街区不是检查站，随机投票（现在足够好了）
	header.Coinbase = common.Address{}
	header.Nonce = types.BlockNonce{}

	number := header.Number.Uint64()
//组装投票快照以检查哪些投票有意义
	snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return err
	}
	if number%c.config.Epoch != 0 {
		c.lock.RLock()

//收集所有有意义的投票提案
		addresses := make([]common.Address, 0, len(c.proposals))
		for address, authorize := range c.proposals {
			if snap.validVote(address, authorize) {
				addresses = append(addresses, address)
			}
		}
//如果有悬而未决的提案，就投票表决。
		if len(addresses) > 0 {
			header.Coinbase = addresses[rand.Intn(len(addresses))]
			if c.proposals[header.Coinbase] {
				copy(header.Nonce[:], nonceAuthVote)
			} else {
				copy(header.Nonce[:], nonceDropVote)
			}
		}
		c.lock.RUnlock()
	}
//设置正确的难度
	header.Difficulty = CalcDifficulty(snap, c.signer)

//确保额外的数据包含所有的组件
	if len(header.Extra) < extraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, extraVanity-len(header.Extra))...)
	}
	header.Extra = header.Extra[:extraVanity]

	if number%c.config.Epoch == 0 {
		for _, signer := range snap.signers() {
			header.Extra = append(header.Extra, signer[:]...)
		}
	}
	header.Extra = append(header.Extra, make([]byte, extraSeal)...)

//混合摘要现在保留，设置为空
	header.MixDigest = common.Hash{}

//确保时间戳具有正确的延迟
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Time = new(big.Int).Add(parent.Time, new(big.Int).SetUint64(c.config.Period))
	if header.Time.Int64() < time.Now().Unix() {
		header.Time = big.NewInt(time.Now().Unix())
	}
	return nil
}

//完成执行共识。引擎，确保没有设置叔叔，也没有阻止
//奖励，并返回最后一个块。
func (c *Clique) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
//在POA中没有集体奖励，所以国家保持原样，叔叔们被抛弃。
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	header.UncleHash = types.CalcUncleHash(nil)

//组装并返回最后一个密封块
	return types.NewBlock(header, txs, nil, receipts), nil
}

//authorize向共识引擎注入一个私钥以创建新的块
//用。
func (c *Clique) Authorize(signer common.Address, signFn SignerFn) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.signer = signer
	c.signFn = signFn
}

//seal实现共识。引擎，试图创建一个密封块使用
//本地签名凭据。
func (c *Clique) Seal(chain consensus.ChainReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	header := block.Header()

//不支持密封Genesis块
	number := header.Number.Uint64()
	if number == 0 {
		return errUnknownBlock
	}
//对于0周期链条，拒绝密封空块（无奖励，但会旋转密封）
	if c.config.Period == 0 && len(block.Transactions()) == 0 {
		log.Info("Sealing paused, waiting for transactions")
		return nil
	}
//在整个密封过程中不要保留签名者字段
	c.lock.RLock()
	signer, signFn := c.signer, c.signFn
	c.lock.RUnlock()

//如果我们未经授权在一个街区内签字，我们就要出狱。
	snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return err
	}
	if _, authorized := snap.Signers[signer]; !authorized {
		return errUnauthorizedSigner
	}
//如果我们是最近的签名者，请等待下一个块
	for seen, recent := range snap.Recents {
		if recent == signer {
//签名者在Recents中，只有在当前块不将其移出时才等待
			if limit := uint64(len(snap.Signers)/2 + 1); number < limit || seen > number-limit {
				log.Info("Signed recently, must wait for others")
				return nil
			}
		}
	}
//太好了，协议允许我们签字，等我们的时间
delay := time.Unix(header.Time.Int64(), 0).Sub(time.Now()) //诺林：天哪
	if header.Difficulty.Cmp(diffNoTurn) == 0 {
//现在轮到我们明确签名了，请稍等一下。
		wiggle := time.Duration(len(snap.Signers)/2+1) * wiggleTime
		delay += time.Duration(rand.Int63n(int64(wiggle)))

		log.Trace("Out-of-turn signing requested", "wiggle", common.PrettyDuration(wiggle))
	}
//在所有东西上签名！
	sighash, err := signFn(accounts.Account{Address: signer}, sigHash(header).Bytes())
	if err != nil {
		return err
	}
	copy(header.Extra[len(header.Extra)-extraSeal:], sighash)
//等待密封终止或延迟超时。
	log.Trace("Waiting for slot to sign and propagate", "delay", common.PrettyDuration(delay))
	go func() {
		select {
		case <-stop:
			return
		case <-time.After(delay):
		}

		select {
		case results <- block.WithSeal(header):
		default:
			log.Warn("Sealing result is not read by miner", "sealhash", c.SealHash(header))
		}
	}()

	return nil
}

//计算难度是难度调整算法。它又回到了困难中
//一个新的块应该基于链中以前的块和
//当前签名者。
func (c *Clique) CalcDifficulty(chain consensus.ChainReader, time uint64, parent *types.Header) *big.Int {
	snap, err := c.snapshot(chain, parent.Number.Uint64(), parent.Hash(), nil)
	if err != nil {
		return nil
	}
	return CalcDifficulty(snap, c.signer)
}

//计算难度是难度调整算法。它又回到了困难中
//一个新的块应该基于链中以前的块和
//当前签名者。
func CalcDifficulty(snap *Snapshot, signer common.Address) *big.Int {
	if snap.inturn(snap.Number+1, signer) {
		return new(big.Int).Set(diffInTurn)
	}
	return new(big.Int).Set(diffNoTurn)
}

//sealHash返回块在被密封之前的哈希。
func (c *Clique) SealHash(header *types.Header) common.Hash {
	return sigHash(header)
}

//CLOSE实现共识引擎。这是一个没有背景线的小集团的noop。
func (c *Clique) Close() error {
	return nil
}

//API实现共识引擎，返回面向用户的RPC API以允许
//控制签名者投票。
func (c *Clique) APIs(chain consensus.ChainReader) []rpc.API {
	return []rpc.API{{
		Namespace: "clique",
		Version:   "1.0",
		Service:   &API{chain: chain, clique: c},
		Public:    false,
	}}
}
