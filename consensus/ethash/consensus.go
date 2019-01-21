
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

package ethash

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
)

//Ethash工作证明协议常数。
var (
FrontierBlockReward       = big.NewInt(5e+18) //在魏块奖励成功开采块
ByzantiumBlockReward      = big.NewInt(3e+18) //从拜占庭向上成功开采一个区块，在魏城获得区块奖励
ConstantinopleBlockReward = big.NewInt(2e+18) //从君士坦丁堡向上成功开采一个区块，在魏城获得区块奖励。
maxUncles                 = 2                 //单个块中允许的最大叔叔数
allowedFutureBlockTime    = 15 * time.Second  //从当前时间算起的最大时间，在考虑将来的块之前

//计算困难度是君士坦丁堡的难度调整算法。
//它返回在给定
//父块的时间和难度。计算使用拜占庭规则，但使用
//炸弹偏移5m。
//规范EIP-1234:https://eips.ethereum.org/eips/eip-1234
	calcDifficultyConstantinople = makeDifficultyCalculator(big.NewInt(5000000))

//拜占庭算法是一种难度调整算法。它返回
//在给定
//父块的时间和难度。计算使用拜占庭规则。
//规范EIP-649:https://eips.ethereum.org/eips/eip-649
	calcDifficultyByzantium = makeDifficultyCalculator(big.NewInt(3000000))
)

//将块标记为无效的各种错误消息。这些应该是私人的
//防止在
//代码库，如果引擎被换出，则固有的中断。请把普通
//共识包中的错误类型。
var (
	errLargeBlockTime    = errors.New("timestamp too big")
	errZeroBlockTime     = errors.New("timestamp equals parent's")
	errTooManyUncles     = errors.New("too many uncles")
	errDuplicateUncle    = errors.New("duplicate uncle")
	errUncleIsAncestor   = errors.New("uncle is ancestor")
	errDanglingUncle     = errors.New("uncle's parent is not ancestor")
	errInvalidDifficulty = errors.New("non-positive difficulty")
	errInvalidMixDigest  = errors.New("invalid mix digest")
	errInvalidPoW        = errors.New("invalid proof-of-work")
)

//作者实现共识引擎，返回头部的coinbase作为
//工作证明证实了该区块的作者。
func (ethash *Ethash) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

//验证标题检查标题是否符合
//库存以太坊Ethash发动机。
func (ethash *Ethash) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
//如果我们正在运行一个完整的引擎伪造，接受任何有效的输入。
	if ethash.config.PowMode == ModeFullFake {
		return nil
	}
//如果知道收割台，或其父项不知道，则短路
	number := header.Number.Uint64()
	if chain.GetHeader(header.Hash(), number) != nil {
		return nil
	}
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
//通过健康检查，进行适当的验证
	return ethash.verifyHeader(chain, header, parent, false, seal)
}

//VerifyHeaders类似于VerifyHeader，但会验证一批头
//同时地。该方法返回退出通道以中止操作，并且
//用于检索异步验证的结果通道。
func (ethash *Ethash) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
//如果我们正在运行一个完整的引擎伪造，接受任何有效的输入。
	if ethash.config.PowMode == ModeFullFake || len(headers) == 0 {
		abort, results := make(chan struct{}), make(chan error, len(headers))
		for i := 0; i < len(headers); i++ {
			results <- nil
		}
		return abort, results
	}

//生成尽可能多的工作线程
	workers := runtime.GOMAXPROCS(0)
	if len(headers) < workers {
		workers = len(headers)
	}

//创建任务通道并生成验证程序
	var (
		inputs = make(chan int)
		done   = make(chan int, workers)
		errors = make([]error, len(headers))
		abort  = make(chan struct{})
	)
	for i := 0; i < workers; i++ {
		go func() {
			for index := range inputs {
				errors[index] = ethash.verifyHeaderWorker(chain, headers, seals, index)
				done <- index
			}
		}()
	}

	errorsOut := make(chan error, len(headers))
	go func() {
		defer close(inputs)
		var (
			in, out = 0, 0
			checked = make([]bool, len(headers))
			inputs  = inputs
		)
		for {
			select {
			case inputs <- in:
				if in++; in == len(headers) {
//已到达邮件头的结尾。停止向工人发送。
					inputs = nil
				}
			case index := <-done:
				for checked[index] = true; checked[out]; out++ {
					errorsOut <- errors[out]
					if out == len(headers)-1 {
						return
					}
				}
			case <-abort:
				return
			}
		}
	}()
	return abort, errorsOut
}

func (ethash *Ethash) verifyHeaderWorker(chain consensus.ChainReader, headers []*types.Header, seals []bool, index int) error {
	var parent *types.Header
	if index == 0 {
		parent = chain.GetHeader(headers[0].ParentHash, headers[0].Number.Uint64()-1)
	} else if headers[index-1].Hash() == headers[index].ParentHash {
		parent = headers[index-1]
	}
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	if chain.GetHeader(headers[index].Hash(), headers[index].Number.Uint64()) != nil {
return nil //已知块体
	}
	return ethash.verifyHeader(chain, headers[index], parent, false, seals[index])
}

//验证叔父验证给定区块的叔父是否符合共识
//股票以太坊的规则。
func (ethash *Ethash) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
//如果我们正在运行一个完整的引擎伪造，接受任何有效的输入。
	if ethash.config.PowMode == ModeFullFake {
		return nil
	}
//验证此块中最多包含2个叔叔
	if len(block.Uncles()) > maxUncles {
		return errTooManyUncles
	}
//收集过去的叔叔和祖先
	uncles, ancestors := mapset.NewSet(), make(map[common.Hash]*types.Header)

	number, parent := block.NumberU64()-1, block.ParentHash()
	for i := 0; i < 7; i++ {
		ancestor := chain.GetBlock(parent, number)
		if ancestor == nil {
			break
		}
		ancestors[ancestor.Hash()] = ancestor.Header()
		for _, uncle := range ancestor.Uncles() {
			uncles.Add(uncle.Hash())
		}
		parent, number = ancestor.ParentHash(), number-1
	}
	ancestors[block.Hash()] = block.Header()
	uncles.Add(block.Hash())

//确认每个叔叔都是最近的，但不是祖先
	for _, uncle := range block.Uncles() {
//确保每个叔叔只奖励一次
		hash := uncle.Hash()
		if uncles.Contains(hash) {
			return errDuplicateUncle
		}
		uncles.Add(hash)

//确保叔叔有一个有效的祖先
		if ancestors[hash] != nil {
			return errUncleIsAncestor
		}
		if ancestors[uncle.ParentHash] == nil || uncle.ParentHash == block.ParentHash() {
			return errDanglingUncle
		}
		if err := ethash.verifyHeader(chain, uncle, ancestors[uncle.ParentHash], true, true); err != nil {
			return err
		}
	}
	return nil
}

//验证标题检查标题是否符合
//库存以太坊Ethash发动机。
//见YP第4.3.4节。”块头有效期”
func (ethash *Ethash) verifyHeader(chain consensus.ChainReader, header, parent *types.Header, uncle bool, seal bool) error {
//确保头的额外数据部分的大小合理
	if uint64(len(header.Extra)) > params.MaximumExtraDataSize {
		return fmt.Errorf("extra-data too long: %d > %d", len(header.Extra), params.MaximumExtraDataSize)
	}
//验证头的时间戳
	if uncle {
		if header.Time.Cmp(math.MaxBig256) > 0 {
			return errLargeBlockTime
		}
	} else {
		if header.Time.Cmp(big.NewInt(time.Now().Add(allowedFutureBlockTime).Unix())) > 0 {
			return consensus.ErrFutureBlock
		}
	}
	if header.Time.Cmp(parent.Time) <= 0 {
		return errZeroBlockTime
	}
//根据时间戳和父块的难度验证块的难度
	expected := ethash.CalcDifficulty(chain, header.Time.Uint64(), parent)

	if expected.Cmp(header.Difficulty) != 0 {
		return fmt.Errorf("invalid difficulty: have %v, want %v", header.Difficulty, expected)
	}
//确认气体限值<=2^63-1
	cap := uint64(0x7fffffffffffffff)
	if header.GasLimit > cap {
		return fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, cap)
	}
//确认所用气体<=气体限值
	if header.GasUsed > header.GasLimit {
		return fmt.Errorf("invalid gasUsed: have %d, gasLimit %d", header.GasUsed, header.GasLimit)
	}

//确认气体限值保持在允许范围内
	diff := int64(parent.GasLimit) - int64(header.GasLimit)
	if diff < 0 {
		diff *= -1
	}
	limit := parent.GasLimit / params.GasLimitBoundDivisor

	if uint64(diff) >= limit || header.GasLimit < params.MinGasLimit {
		return fmt.Errorf("invalid gas limit: have %d, want %d += %d", header.GasLimit, parent.GasLimit, limit)
	}
//验证块号是否为父块的+1
	if diff := new(big.Int).Sub(header.Number, parent.Number); diff.Cmp(big.NewInt(1)) != 0 {
		return consensus.ErrInvalidNumber
	}
//确认固定气缸体的发动机专用密封件
	if seal {
		if err := ethash.VerifySeal(chain, header); err != nil {
			return err
		}
	}
//如果所有检查都通过，则验证硬分叉的任何特殊字段
	if err := misc.VerifyDAOHeaderExtraData(chain.Config(), header); err != nil {
		return err
	}
	if err := misc.VerifyForkHashes(chain.Config(), header, uncle); err != nil {
		return err
	}
	return nil
}

//计算难度是难度调整算法。它返回
//新块在创建时应该具有的困难
//考虑到父块的时间和难度。
func (ethash *Ethash) CalcDifficulty(chain consensus.ChainReader, time uint64, parent *types.Header) *big.Int {
	return CalcDifficulty(chain.Config(), time, parent)
}

//计算难度是难度调整算法。它返回
//新块在创建时应该具有的困难
//考虑到父块的时间和难度。
func CalcDifficulty(config *params.ChainConfig, time uint64, parent *types.Header) *big.Int {
	next := new(big.Int).Add(parent.Number, big1)
	switch {
	case config.IsConstantinople(next):
		return calcDifficultyConstantinople(time, parent)
	case config.IsByzantium(next):
		return calcDifficultyByzantium(time, parent)
	case config.IsHomestead(next):
		return calcDifficultyHomestead(time, parent)
	default:
		return calcDifficultyFrontier(time, parent)
	}
}

//一些奇怪的常量，以避免为它们分配常量内存。
var (
	expDiffPeriod = big.NewInt(100000)
	big1          = big.NewInt(1)
	big2          = big.NewInt(2)
	big9          = big.NewInt(9)
	big10         = big.NewInt(10)
	bigMinus99    = big.NewInt(-99)
)

//makedifficulticcalculator创建具有给定炸弹延迟的difficulticcalculator。
//难度是根据拜占庭规则计算的，这与宅基地在
//叔叔是如何影响计算的
func makeDifficultyCalculator(bombDelay *big.Int) func(time uint64, parent *types.Header) *big.Int {
//注意，下面的计算将查看下面的父编号1
//块编号。因此，我们从给定的延迟中删除一个
	bombDelayFromParent := new(big.Int).Sub(bombDelay, big1)
	return func(time uint64, parent *types.Header) *big.Int {
//https://github.com/ethereum/eips/issues/100。
//算法：
//diff=（父级diff+
//（父级差异/2048*最大值（（如果len（parent.uncles）为2，否则为1）-（（timestamp-parent.timestamp）//9），-99））
//）+2^（周期计数-2）

		bigTime := new(big.Int).SetUint64(time)
		bigParentTime := new(big.Int).Set(parent.Time)

//保持中间值以使算法更易于读取和审计
		x := new(big.Int)
		y := new(big.Int)

//（2如果len（parent_uncles）else 1）-（block_timestamp-parent_timestamp）//9
		x.Sub(bigTime, bigParentTime)
		x.Div(x, big9)
		if parent.UncleHash == types.EmptyUncleHash {
			x.Sub(big1, x)
		} else {
			x.Sub(big2, x)
		}
//max（（如果len（parent_uncles）为2，否则为1）-（block_timestamp-parent_timestamp）//9，-99）
		if x.Cmp(bigMinus99) < 0 {
			x.Set(bigMinus99)
		}
//父级差异+（父级差异/2048*最大值（（如果len（parent.uncles）为2，否则为1）-（（timestamp-parent.timestamp）//9），-99））
		y.Div(parent.Difficulty, params.DifficultyBoundDivisor)
		x.Mul(y, x)
		x.Add(parent.Difficulty, x)

//最小难度可以是（指数因子之前）
		if x.Cmp(params.MinimumDifficulty) < 0 {
			x.Set(params.MinimumDifficulty)
		}
//计算冰河期延迟的假街区数
//规格：https://eips.ethereum.org/eips/eip-1234
		fakeBlockNumber := new(big.Int)
		if parent.Number.Cmp(bombDelayFromParent) >= 0 {
			fakeBlockNumber = fakeBlockNumber.Sub(parent.Number, bombDelayFromParent)
		}
//对于指数因子
		periodCount := fakeBlockNumber
		periodCount.Div(periodCount, expDiffPeriod)

//指数因子，通常称为“炸弹”
//diff=diff+2^（周期计数-2）
		if periodCount.Cmp(big1) > 0 {
			y.Sub(periodCount, big2)
			y.Exp(big2, y, nil)
			x.Add(x, y)
		}
		return x
	}
}

//CalcDifficultyHomeStead是难度调整算法。它返回
//在给定
//父块的时间和难度。计算使用宅基地规则。
func calcDifficultyHomestead(time uint64, parent *types.Header) *big.Int {
//https://github.com/ethereum/eips/blob/master/eips/eip-2.md网站
//算法：
//diff=（父级diff+
//（parent_diff/2048*最大值（1-（block_timestamp-parent_timestamp）//10，-99））
//）+2^（周期计数-2）

	bigTime := new(big.Int).SetUint64(time)
	bigParentTime := new(big.Int).Set(parent.Time)

//保持中间值以使算法更易于读取和审计
	x := new(big.Int)
	y := new(big.Int)

//1-（block_timestamp-parent_timestamp）//10
	x.Sub(bigTime, bigParentTime)
	x.Div(x, big10)
	x.Sub(big1, x)

//max（1-（block_timestamp-parent_timestamp）//10，-99）
	if x.Cmp(bigMinus99) < 0 {
		x.Set(bigMinus99)
	}
//（parent_diff+parent_diff//2048*最大值（1-（block_timestamp-parent_timestamp）//10，-99））
	y.Div(parent.Difficulty, params.DifficultyBoundDivisor)
	x.Mul(y, x)
	x.Add(parent.Difficulty, x)

//最小难度可以是（指数因子之前）
	if x.Cmp(params.MinimumDifficulty) < 0 {
		x.Set(params.MinimumDifficulty)
	}
//对于指数因子
	periodCount := new(big.Int).Add(parent.Number, big1)
	periodCount.Div(periodCount, expDiffPeriod)

//指数因子，通常称为“炸弹”
//diff=diff+2^（周期计数-2）
	if periodCount.Cmp(big1) > 0 {
		y.Sub(periodCount, big2)
		y.Exp(big2, y, nil)
		x.Add(x, y)
	}
	return x
}

//计算难度边界是难度调整算法。它返回
//在给定父级的情况下创建新块时应具有的困难
//布洛克的时间和难度。计算使用边界规则。
func calcDifficultyFrontier(time uint64, parent *types.Header) *big.Int {
	diff := new(big.Int)
	adjust := new(big.Int).Div(parent.Difficulty, params.DifficultyBoundDivisor)
	bigTime := new(big.Int)
	bigParentTime := new(big.Int)

	bigTime.SetUint64(time)
	bigParentTime.Set(parent.Time)

	if bigTime.Sub(bigTime, bigParentTime).Cmp(params.DurationLimit) < 0 {
		diff.Add(parent.Difficulty, adjust)
	} else {
		diff.Sub(parent.Difficulty, adjust)
	}
	if diff.Cmp(params.MinimumDifficulty) < 0 {
		diff.Set(params.MinimumDifficulty)
	}

	periodCount := new(big.Int).Add(parent.Number, big1)
	periodCount.Div(periodCount, expDiffPeriod)
	if periodCount.Cmp(big1) > 0 {
//diff=diff+2^（周期计数-2）
		expDiff := periodCount.Sub(periodCount, big2)
		expDiff.Exp(big2, expDiff, nil)
		diff.Add(diff, expDiff)
		diff = math.BigMax(diff, params.MinimumDifficulty)
	}
	return diff
}

//验证seal是否执行共识引擎，检查给定的块是否满足
//POW难度要求。
func (ethash *Ethash) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	return ethash.verifySeal(chain, header, false)
}

//验证Seal检查块是否满足POW难度要求，
//或者使用通常的ethash缓存，或者使用完整的DAG
//以加快远程挖掘。
func (ethash *Ethash) verifySeal(chain consensus.ChainReader, header *types.Header, fulldag bool) error {
//如果我们开的是假战俘，接受任何有效的印章。
	if ethash.config.PowMode == ModeFake || ethash.config.PowMode == ModeFullFake {
		time.Sleep(ethash.fakeDelay)
		if ethash.fakeFail == header.Number.Uint64() {
			return errInvalidPoW
		}
		return nil
	}
//如果我们正在运行一个共享的POW，请将验证委托给它。
	if ethash.shared != nil {
		return ethash.shared.verifySeal(chain, header, fulldag)
	}
//确保我们有一个有效的障碍。
	if header.Difficulty.Sign() <= 0 {
		return errInvalidDifficulty
	}
//重新计算摘要值和POW值
	number := header.Number.Uint64()

	var (
		digest []byte
		result []byte
	)
//如果请求快速但繁重的POW验证，请使用ethash数据集
	if fulldag {
		dataset := ethash.dataset(number, true)
		if dataset.generated() {
			digest, result = hashimotoFull(dataset.dataset, ethash.SealHash(header).Bytes(), header.Nonce.Uint64())

//数据集在终结器中未映射。确保数据集保持活动状态
//直到调用后桥本满，所以在使用时不会取消映射。
			runtime.KeepAlive(dataset)
		} else {
//数据集尚未生成，请不要挂起，改用缓存
			fulldag = false
		}
	}
//如果请求缓慢但轻微的POW验证（或DAG尚未就绪），请使用ethash缓存
	if !fulldag {
		cache := ethash.cache(number)

		size := datasetSize(number)
		if ethash.config.PowMode == ModeTest {
			size = 32 * 1024
		}
		digest, result = hashimotoLight(size, cache.cache, ethash.SealHash(header).Bytes(), header.Nonce.Uint64())

//在终结器中取消映射缓存。确保缓存保持活动状态
//直到调用桥本灯后，才能在使用时取消映射。
		runtime.KeepAlive(cache)
	}
//对照标题中提供的值验证计算值
	if !bytes.Equal(header.MixDigest[:], digest) {
		return errInvalidMixDigest
	}
	target := new(big.Int).Div(two256, header.Difficulty)
	if new(big.Int).SetBytes(result).Cmp(target) > 0 {
		return errInvalidPoW
	}
	return nil
}

//准备执行共识。引擎，初始化
//头符合ethash协议。更改是以内联方式完成的。
func (ethash *Ethash) Prepare(chain consensus.ChainReader, header *types.Header) error {
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Difficulty = ethash.CalcDifficulty(chain, header.Time.Uint64(), parent)
	return nil
}

//达成共识。引擎，积木，叔叔奖励，
//设置最终状态并组装块。
func (ethash *Ethash) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
//累积任何块和叔叔奖励并提交最终状态根
	accumulateRewards(chain.Config(), state, header, uncles)
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))

//收割台似乎已完成，组装成一个块并返回
	return types.NewBlock(header, txs, uncles, receipts), nil
}

//sealHash返回块在被密封之前的哈希。
func (ethash *Ethash) SealHash(header *types.Header) (hash common.Hash) {
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
		header.Extra,
	})
	hasher.Sum(hash[:0])
	return hash
}

//一些奇怪的常量，以避免为它们分配常量内存。
var (
	big8  = big.NewInt(8)
	big32 = big.NewInt(32)
)

//累加后，将给定块的coinbase用于采矿。
//奖赏。总奖励包括静态块奖励和
//包括叔叔。每个叔叔街区的硬币库也会得到奖励。
func accumulateRewards(config *params.ChainConfig, state *state.StateDB, header *types.Header, uncles []*types.Header) {
//根据链进程选择正确的区块奖励
	blockReward := FrontierBlockReward
	if config.IsByzantium(header.Number) {
		blockReward = ByzantiumBlockReward
	}
	if config.IsConstantinople(header.Number) {
		blockReward = ConstantinopleBlockReward
	}
//为矿工和任何包括叔叔的人累积奖励
	reward := new(big.Int).Set(blockReward)
	r := new(big.Int)
	for _, uncle := range uncles {
		r.Add(uncle.Number, big8)
		r.Sub(r, header.Number)
		r.Mul(r, blockReward)
		r.Div(r, big8)
		state.AddBalance(uncle.Coinbase, r)

		r.Div(blockReward, big32)
		reward.Add(reward, r)
	}
	state.AddBalance(header.Coinbase, reward)
}
