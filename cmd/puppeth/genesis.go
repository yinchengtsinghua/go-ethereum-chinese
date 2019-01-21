
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
//此文件是Go以太坊的一部分。
//
//Go以太坊是免费软件：您可以重新发布和/或修改它
//根据GNU通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊的分布希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU通用公共许可证了解更多详细信息。
//
//你应该已经收到一份GNU通用公共许可证的副本
//一起去以太坊吧。如果没有，请参见<http://www.gnu.org/licenses/>。

package main

import (
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	math2 "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
)

//alethgenesspec表示
//C++ EthUUM实现。
type alethGenesisSpec struct {
	SealEngine string `json:"sealEngine"`
	Params     struct {
		AccountStartNonce       math2.HexOrDecimal64   `json:"accountStartNonce"`
		MaximumExtraDataSize    hexutil.Uint64         `json:"maximumExtraDataSize"`
		HomesteadForkBlock      hexutil.Uint64         `json:"homesteadForkBlock"`
		DaoHardforkBlock        math2.HexOrDecimal64   `json:"daoHardforkBlock"`
		EIP150ForkBlock         hexutil.Uint64         `json:"EIP150ForkBlock"`
		EIP158ForkBlock         hexutil.Uint64         `json:"EIP158ForkBlock"`
		ByzantiumForkBlock      hexutil.Uint64         `json:"byzantiumForkBlock"`
		ConstantinopleForkBlock hexutil.Uint64         `json:"constantinopleForkBlock"`
		MinGasLimit             hexutil.Uint64         `json:"minGasLimit"`
		MaxGasLimit             hexutil.Uint64         `json:"maxGasLimit"`
		TieBreakingGas          bool                   `json:"tieBreakingGas"`
		GasLimitBoundDivisor    math2.HexOrDecimal64   `json:"gasLimitBoundDivisor"`
		MinimumDifficulty       *hexutil.Big           `json:"minimumDifficulty"`
		DifficultyBoundDivisor  *math2.HexOrDecimal256 `json:"difficultyBoundDivisor"`
		DurationLimit           *math2.HexOrDecimal256 `json:"durationLimit"`
		BlockReward             *hexutil.Big           `json:"blockReward"`
		NetworkID               hexutil.Uint64         `json:"networkID"`
		ChainID                 hexutil.Uint64         `json:"chainID"`
		AllowFutureBlocks       bool                   `json:"allowFutureBlocks"`
	} `json:"params"`

	Genesis struct {
		Nonce      hexutil.Bytes  `json:"nonce"`
		Difficulty *hexutil.Big   `json:"difficulty"`
		MixHash    common.Hash    `json:"mixHash"`
		Author     common.Address `json:"author"`
		Timestamp  hexutil.Uint64 `json:"timestamp"`
		ParentHash common.Hash    `json:"parentHash"`
		ExtraData  hexutil.Bytes  `json:"extraData"`
		GasLimit   hexutil.Uint64 `json:"gasLimit"`
	} `json:"genesis"`

	Accounts map[common.UnprefixedAddress]*alethGenesisSpecAccount `json:"accounts"`
}

//alethgenesspeccount是预先准备好的genesis帐户和/或预编译的
//合同定义。
type alethGenesisSpecAccount struct {
	Balance     *math2.HexOrDecimal256   `json:"balance"`
	Nonce       uint64                   `json:"nonce,omitempty"`
	Precompiled *alethGenesisSpecBuiltin `json:"precompiled,omitempty"`
}

//alethgenesspecbuiltin是预编译的合同定义。
type alethGenesisSpecBuiltin struct {
	Name          string                         `json:"name,omitempty"`
	StartingBlock hexutil.Uint64                 `json:"startingBlock,omitempty"`
	Linear        *alethGenesisSpecLinearPricing `json:"linear,omitempty"`
}

type alethGenesisSpecLinearPricing struct {
	Base uint64 `json:"base"`
	Word uint64 `json:"word"`
}

//newalethgenesspec将go-ethereum genesis块转换为aleth特定的块。
//链规范格式。
func newAlethGenesisSpec(network string, genesis *core.Genesis) (*alethGenesisSpec, error) {
//Go Ethereum和Aleth之间目前仅支持ethash
	if genesis.Config.Ethash == nil {
		return nil, errors.New("unsupported consensus engine")
	}
//以aleth格式重新构造链规范
	spec := &alethGenesisSpec{
		SealEngine: "Ethash",
	}
//一些默认值
	spec.Params.AccountStartNonce = 0
	spec.Params.TieBreakingGas = false
	spec.Params.AllowFutureBlocks = false
	spec.Params.DaoHardforkBlock = 0

	spec.Params.HomesteadForkBlock = (hexutil.Uint64)(genesis.Config.HomesteadBlock.Uint64())
	spec.Params.EIP150ForkBlock = (hexutil.Uint64)(genesis.Config.EIP150Block.Uint64())
	spec.Params.EIP158ForkBlock = (hexutil.Uint64)(genesis.Config.EIP158Block.Uint64())

//
	if num := genesis.Config.ByzantiumBlock; num != nil {
		spec.setByzantium(num)
	}
//君士坦丁堡
	if num := genesis.Config.ConstantinopleBlock; num != nil {
		spec.setConstantinople(num)
	}

	spec.Params.NetworkID = (hexutil.Uint64)(genesis.Config.ChainID.Uint64())
	spec.Params.ChainID = (hexutil.Uint64)(genesis.Config.ChainID.Uint64())
	spec.Params.MaximumExtraDataSize = (hexutil.Uint64)(params.MaximumExtraDataSize)
	spec.Params.MinGasLimit = (hexutil.Uint64)(params.MinGasLimit)
	spec.Params.MaxGasLimit = (hexutil.Uint64)(math.MaxInt64)
	spec.Params.MinimumDifficulty = (*hexutil.Big)(params.MinimumDifficulty)
	spec.Params.DifficultyBoundDivisor = (*math2.HexOrDecimal256)(params.DifficultyBoundDivisor)
	spec.Params.GasLimitBoundDivisor = (math2.HexOrDecimal64)(params.GasLimitBoundDivisor)
	spec.Params.DurationLimit = (*math2.HexOrDecimal256)(params.DurationLimit)
	spec.Params.BlockReward = (*hexutil.Big)(ethash.FrontierBlockReward)

	spec.Genesis.Nonce = (hexutil.Bytes)(make([]byte, 8))
	binary.LittleEndian.PutUint64(spec.Genesis.Nonce[:], genesis.Nonce)

	spec.Genesis.MixHash = genesis.Mixhash
	spec.Genesis.Difficulty = (*hexutil.Big)(genesis.Difficulty)
	spec.Genesis.Author = genesis.Coinbase
	spec.Genesis.Timestamp = (hexutil.Uint64)(genesis.Timestamp)
	spec.Genesis.ParentHash = genesis.ParentHash
	spec.Genesis.ExtraData = (hexutil.Bytes)(genesis.ExtraData)
	spec.Genesis.GasLimit = (hexutil.Uint64)(genesis.GasLimit)

	for address, account := range genesis.Alloc {
		spec.setAccount(address, account)
	}

	spec.setPrecompile(1, &alethGenesisSpecBuiltin{Name: "ecrecover",
		Linear: &alethGenesisSpecLinearPricing{Base: 3000}})
	spec.setPrecompile(2, &alethGenesisSpecBuiltin{Name: "sha256",
		Linear: &alethGenesisSpecLinearPricing{Base: 60, Word: 12}})
	spec.setPrecompile(3, &alethGenesisSpecBuiltin{Name: "ripemd160",
		Linear: &alethGenesisSpecLinearPricing{Base: 600, Word: 120}})
	spec.setPrecompile(4, &alethGenesisSpecBuiltin{Name: "identity",
		Linear: &alethGenesisSpecLinearPricing{Base: 15, Word: 3}})
	if genesis.Config.ByzantiumBlock != nil {
		spec.setPrecompile(5, &alethGenesisSpecBuiltin{Name: "modexp",
			StartingBlock: (hexutil.Uint64)(genesis.Config.ByzantiumBlock.Uint64())})
		spec.setPrecompile(6, &alethGenesisSpecBuiltin{Name: "alt_bn128_G1_add",
			StartingBlock: (hexutil.Uint64)(genesis.Config.ByzantiumBlock.Uint64()),
			Linear:        &alethGenesisSpecLinearPricing{Base: 500}})
		spec.setPrecompile(7, &alethGenesisSpecBuiltin{Name: "alt_bn128_G1_mul",
			StartingBlock: (hexutil.Uint64)(genesis.Config.ByzantiumBlock.Uint64()),
			Linear:        &alethGenesisSpecLinearPricing{Base: 40000}})
		spec.setPrecompile(8, &alethGenesisSpecBuiltin{Name: "alt_bn128_pairing_product",
			StartingBlock: (hexutil.Uint64)(genesis.Config.ByzantiumBlock.Uint64())})
	}
	return spec, nil
}

func (spec *alethGenesisSpec) setPrecompile(address byte, data *alethGenesisSpecBuiltin) {
	if spec.Accounts == nil {
		spec.Accounts = make(map[common.UnprefixedAddress]*alethGenesisSpecAccount)
	}
	addr := common.UnprefixedAddress(common.BytesToAddress([]byte{address}))
	if _, exist := spec.Accounts[addr]; !exist {
		spec.Accounts[addr] = &alethGenesisSpecAccount{}
	}
	spec.Accounts[addr].Precompiled = data
}

func (spec *alethGenesisSpec) setAccount(address common.Address, account core.GenesisAccount) {
	if spec.Accounts == nil {
		spec.Accounts = make(map[common.UnprefixedAddress]*alethGenesisSpecAccount)
	}

	a, exist := spec.Accounts[common.UnprefixedAddress(address)]
	if !exist {
		a = &alethGenesisSpecAccount{}
		spec.Accounts[common.UnprefixedAddress(address)] = a
	}
	a.Balance = (*math2.HexOrDecimal256)(account.Balance)
	a.Nonce = account.Nonce

}

func (spec *alethGenesisSpec) setByzantium(num *big.Int) {
	spec.Params.ByzantiumForkBlock = hexutil.Uint64(num.Uint64())
}

func (spec *alethGenesisSpec) setConstantinople(num *big.Int) {
	spec.Params.ConstantinopleForkBlock = hexutil.Uint64(num.Uint64())
}

//paritychainspec是奇偶校验使用的链规范格式。
type parityChainSpec struct {
	Name    string `json:"name"`
	Datadir string `json:"dataDir"`
	Engine  struct {
		Ethash struct {
			Params struct {
				MinimumDifficulty      *hexutil.Big      `json:"minimumDifficulty"`
				DifficultyBoundDivisor *hexutil.Big      `json:"difficultyBoundDivisor"`
				DurationLimit          *hexutil.Big      `json:"durationLimit"`
				BlockReward            map[string]string `json:"blockReward"`
				DifficultyBombDelays   map[string]string `json:"difficultyBombDelays"`
				HomesteadTransition    hexutil.Uint64    `json:"homesteadTransition"`
				EIP100bTransition      hexutil.Uint64    `json:"eip100bTransition"`
			} `json:"params"`
		} `json:"Ethash"`
	} `json:"engine"`

	Params struct {
		AccountStartNonce     hexutil.Uint64       `json:"accountStartNonce"`
		MaximumExtraDataSize  hexutil.Uint64       `json:"maximumExtraDataSize"`
		MinGasLimit           hexutil.Uint64       `json:"minGasLimit"`
		GasLimitBoundDivisor  math2.HexOrDecimal64 `json:"gasLimitBoundDivisor"`
		NetworkID             hexutil.Uint64       `json:"networkID"`
		ChainID               hexutil.Uint64       `json:"chainID"`
		MaxCodeSize           hexutil.Uint64       `json:"maxCodeSize"`
		MaxCodeSizeTransition hexutil.Uint64       `json:"maxCodeSizeTransition"`
		EIP98Transition       hexutil.Uint64       `json:"eip98Transition"`
		EIP150Transition      hexutil.Uint64       `json:"eip150Transition"`
		EIP160Transition      hexutil.Uint64       `json:"eip160Transition"`
		EIP161abcTransition   hexutil.Uint64       `json:"eip161abcTransition"`
		EIP161dTransition     hexutil.Uint64       `json:"eip161dTransition"`
		EIP155Transition      hexutil.Uint64       `json:"eip155Transition"`
		EIP140Transition      hexutil.Uint64       `json:"eip140Transition"`
		EIP211Transition      hexutil.Uint64       `json:"eip211Transition"`
		EIP214Transition      hexutil.Uint64       `json:"eip214Transition"`
		EIP658Transition      hexutil.Uint64       `json:"eip658Transition"`
		EIP145Transition      hexutil.Uint64       `json:"eip145Transition"`
		EIP1014Transition     hexutil.Uint64       `json:"eip1014Transition"`
		EIP1052Transition     hexutil.Uint64       `json:"eip1052Transition"`
		EIP1283Transition     hexutil.Uint64       `json:"eip1283Transition"`
	} `json:"params"`

	Genesis struct {
		Seal struct {
			Ethereum struct {
				Nonce   hexutil.Bytes `json:"nonce"`
				MixHash hexutil.Bytes `json:"mixHash"`
			} `json:"ethereum"`
		} `json:"seal"`

		Difficulty *hexutil.Big   `json:"difficulty"`
		Author     common.Address `json:"author"`
		Timestamp  hexutil.Uint64 `json:"timestamp"`
		ParentHash common.Hash    `json:"parentHash"`
		ExtraData  hexutil.Bytes  `json:"extraData"`
		GasLimit   hexutil.Uint64 `json:"gasLimit"`
	} `json:"genesis"`

	Nodes    []string                                             `json:"nodes"`
	Accounts map[common.UnprefixedAddress]*parityChainSpecAccount `json:"accounts"`
}

//ParityChainspeccount是预先准备好的Genesis帐户和/或预编译的
//合同定义。
type parityChainSpecAccount struct {
	Balance math2.HexOrDecimal256   `json:"balance"`
	Nonce   math2.HexOrDecimal64    `json:"nonce,omitempty"`
	Builtin *parityChainSpecBuiltin `json:"builtin,omitempty"`
}

//paritychainspeccuiltin是预编译的合同定义。
type parityChainSpecBuiltin struct {
	Name       string                  `json:"name,omitempty"`
	ActivateAt math2.HexOrDecimal64    `json:"activate_at,omitempty"`
	Pricing    *parityChainSpecPricing `json:"pricing,omitempty"`
}

//ParityChainSecPricing表示内置的不同定价模型
//合同可能会宣传使用。
type parityChainSpecPricing struct {
	Linear       *parityChainSpecLinearPricing       `json:"linear,omitempty"`
	ModExp       *parityChainSpecModExpPricing       `json:"modexp,omitempty"`
	AltBnPairing *parityChainSpecAltBnPairingPricing `json:"alt_bn128_pairing,omitempty"`
}

type parityChainSpecLinearPricing struct {
	Base uint64 `json:"base"`
	Word uint64 `json:"word"`
}

type parityChainSpecModExpPricing struct {
	Divisor uint64 `json:"divisor"`
}

type parityChainSpecAltBnPairingPricing struct {
	Base uint64 `json:"base"`
	Pair uint64 `json:"pair"`
}

//NewParityChainspec将Go-Ethereum Genesis块转换为特定于奇偶校验的块。
//链规范格式。
func newParityChainSpec(network string, genesis *core.Genesis, bootnodes []string) (*parityChainSpec, error) {
//在go-ethereum和parity之间，目前只支持ethash。
	if genesis.Config.Ethash == nil {
		return nil, errors.New("unsupported consensus engine")
	}
//以奇偶校验格式重新构造链规范
	spec := &parityChainSpec{
		Name:    network,
		Nodes:   bootnodes,
		Datadir: strings.ToLower(network),
	}
	spec.Engine.Ethash.Params.BlockReward = make(map[string]string)
	spec.Engine.Ethash.Params.DifficultyBombDelays = make(map[string]string)
//边疆
	spec.Engine.Ethash.Params.MinimumDifficulty = (*hexutil.Big)(params.MinimumDifficulty)
	spec.Engine.Ethash.Params.DifficultyBoundDivisor = (*hexutil.Big)(params.DifficultyBoundDivisor)
	spec.Engine.Ethash.Params.DurationLimit = (*hexutil.Big)(params.DurationLimit)
	spec.Engine.Ethash.Params.BlockReward["0x0"] = hexutil.EncodeBig(ethash.FrontierBlockReward)

//家宅
	spec.Engine.Ethash.Params.HomesteadTransition = hexutil.Uint64(genesis.Config.HomesteadBlock.Uint64())

//橘子哨子：150
//https://github.com/ethereum/eips/blob/master/eips/eip-608.md网站
	spec.Params.EIP150Transition = hexutil.Uint64(genesis.Config.EIP150Block.Uint64())

//假龙：155、160、161、170
//https://github.com/ethereum/eips/blob/master/eips/eip-607.md网站
	spec.Params.EIP155Transition = hexutil.Uint64(genesis.Config.EIP155Block.Uint64())
	spec.Params.EIP160Transition = hexutil.Uint64(genesis.Config.EIP155Block.Uint64())
	spec.Params.EIP161abcTransition = hexutil.Uint64(genesis.Config.EIP158Block.Uint64())
	spec.Params.EIP161dTransition = hexutil.Uint64(genesis.Config.EIP158Block.Uint64())

//拜占庭
	if num := genesis.Config.ByzantiumBlock; num != nil {
		spec.setByzantium(num)
	}
//君士坦丁堡
	if num := genesis.Config.ConstantinopleBlock; num != nil {
		spec.setConstantinople(num)
	}
	spec.Params.MaximumExtraDataSize = (hexutil.Uint64)(params.MaximumExtraDataSize)
	spec.Params.MinGasLimit = (hexutil.Uint64)(params.MinGasLimit)
	spec.Params.GasLimitBoundDivisor = (math2.HexOrDecimal64)(params.GasLimitBoundDivisor)
	spec.Params.NetworkID = (hexutil.Uint64)(genesis.Config.ChainID.Uint64())
	spec.Params.ChainID = (hexutil.Uint64)(genesis.Config.ChainID.Uint64())
	spec.Params.MaxCodeSize = params.MaxCodeSize
//Geth已将其设置为零
	spec.Params.MaxCodeSizeTransition = 0

//禁用这个
	spec.Params.EIP98Transition = math.MaxInt64

	spec.Genesis.Seal.Ethereum.Nonce = (hexutil.Bytes)(make([]byte, 8))
	binary.LittleEndian.PutUint64(spec.Genesis.Seal.Ethereum.Nonce[:], genesis.Nonce)

	spec.Genesis.Seal.Ethereum.MixHash = (hexutil.Bytes)(genesis.Mixhash[:])
	spec.Genesis.Difficulty = (*hexutil.Big)(genesis.Difficulty)
	spec.Genesis.Author = genesis.Coinbase
	spec.Genesis.Timestamp = (hexutil.Uint64)(genesis.Timestamp)
	spec.Genesis.ParentHash = genesis.ParentHash
	spec.Genesis.ExtraData = (hexutil.Bytes)(genesis.ExtraData)
	spec.Genesis.GasLimit = (hexutil.Uint64)(genesis.GasLimit)

	spec.Accounts = make(map[common.UnprefixedAddress]*parityChainSpecAccount)
	for address, account := range genesis.Alloc {
		bal := math2.HexOrDecimal256(*account.Balance)

		spec.Accounts[common.UnprefixedAddress(address)] = &parityChainSpecAccount{
			Balance: bal,
			Nonce:   math2.HexOrDecimal64(account.Nonce),
		}
	}
	spec.setPrecompile(1, &parityChainSpecBuiltin{Name: "ecrecover",
		Pricing: &parityChainSpecPricing{Linear: &parityChainSpecLinearPricing{Base: 3000}}})

	spec.setPrecompile(2, &parityChainSpecBuiltin{
		Name: "sha256", Pricing: &parityChainSpecPricing{Linear: &parityChainSpecLinearPricing{Base: 60, Word: 12}},
	})
	spec.setPrecompile(3, &parityChainSpecBuiltin{
		Name: "ripemd160", Pricing: &parityChainSpecPricing{Linear: &parityChainSpecLinearPricing{Base: 600, Word: 120}},
	})
	spec.setPrecompile(4, &parityChainSpecBuiltin{
		Name: "identity", Pricing: &parityChainSpecPricing{Linear: &parityChainSpecLinearPricing{Base: 15, Word: 3}},
	})
	if genesis.Config.ByzantiumBlock != nil {
		blnum := math2.HexOrDecimal64(genesis.Config.ByzantiumBlock.Uint64())
		spec.setPrecompile(5, &parityChainSpecBuiltin{
			Name: "modexp", ActivateAt: blnum, Pricing: &parityChainSpecPricing{ModExp: &parityChainSpecModExpPricing{Divisor: 20}},
		})
		spec.setPrecompile(6, &parityChainSpecBuiltin{
			Name: "alt_bn128_add", ActivateAt: blnum, Pricing: &parityChainSpecPricing{Linear: &parityChainSpecLinearPricing{Base: 500}},
		})
		spec.setPrecompile(7, &parityChainSpecBuiltin{
			Name: "alt_bn128_mul", ActivateAt: blnum, Pricing: &parityChainSpecPricing{Linear: &parityChainSpecLinearPricing{Base: 40000}},
		})
		spec.setPrecompile(8, &parityChainSpecBuiltin{
			Name: "alt_bn128_pairing", ActivateAt: blnum, Pricing: &parityChainSpecPricing{AltBnPairing: &parityChainSpecAltBnPairingPricing{Base: 100000, Pair: 80000}},
		})
	}
	return spec, nil
}

func (spec *parityChainSpec) setPrecompile(address byte, data *parityChainSpecBuiltin) {
	if spec.Accounts == nil {
		spec.Accounts = make(map[common.UnprefixedAddress]*parityChainSpecAccount)
	}
	a := common.UnprefixedAddress(common.BytesToAddress([]byte{address}))
	if _, exist := spec.Accounts[a]; !exist {
		spec.Accounts[a] = &parityChainSpecAccount{}
	}
	spec.Accounts[a].Builtin = data
}

func (spec *parityChainSpec) setByzantium(num *big.Int) {
	spec.Engine.Ethash.Params.BlockReward[hexutil.EncodeBig(num)] = hexutil.EncodeBig(ethash.ByzantiumBlockReward)
	spec.Engine.Ethash.Params.DifficultyBombDelays[hexutil.EncodeBig(num)] = hexutil.EncodeUint64(3000000)
	n := hexutil.Uint64(num.Uint64())
	spec.Engine.Ethash.Params.EIP100bTransition = n
	spec.Params.EIP140Transition = n
	spec.Params.EIP211Transition = n
	spec.Params.EIP214Transition = n
	spec.Params.EIP658Transition = n
}

func (spec *parityChainSpec) setConstantinople(num *big.Int) {
	spec.Engine.Ethash.Params.BlockReward[hexutil.EncodeBig(num)] = hexutil.EncodeBig(ethash.ConstantinopleBlockReward)
	spec.Engine.Ethash.Params.DifficultyBombDelays[hexutil.EncodeBig(num)] = hexutil.EncodeUint64(2000000)
	n := hexutil.Uint64(num.Uint64())
	spec.Params.EIP145Transition = n
	spec.Params.EIP1014Transition = n
	spec.Params.EIP1052Transition = n
	spec.Params.EIP1283Transition = n
}

//pyethereumgeneisspec表示
//python-ethereum实现。
type pyEthereumGenesisSpec struct {
	Nonce      hexutil.Bytes     `json:"nonce"`
	Timestamp  hexutil.Uint64    `json:"timestamp"`
	ExtraData  hexutil.Bytes     `json:"extraData"`
	GasLimit   hexutil.Uint64    `json:"gasLimit"`
	Difficulty *hexutil.Big      `json:"difficulty"`
	Mixhash    common.Hash       `json:"mixhash"`
	Coinbase   common.Address    `json:"coinbase"`
	Alloc      core.GenesisAlloc `json:"alloc"`
	ParentHash common.Hash       `json:"parentHash"`
}

//Newpyethereumgeneisspec将Go-Ethereum Genesis块转换为特定于奇偶校验的块。
//链规范格式。
func newPyEthereumGenesisSpec(network string, genesis *core.Genesis) (*pyEthereumGenesisSpec, error) {
//Go以太坊和Pyetereum目前仅支持ethash
	if genesis.Config.Ethash == nil {
		return nil, errors.New("unsupported consensus engine")
	}
	spec := &pyEthereumGenesisSpec{
		Timestamp:  (hexutil.Uint64)(genesis.Timestamp),
		ExtraData:  genesis.ExtraData,
		GasLimit:   (hexutil.Uint64)(genesis.GasLimit),
		Difficulty: (*hexutil.Big)(genesis.Difficulty),
		Mixhash:    genesis.Mixhash,
		Coinbase:   genesis.Coinbase,
		Alloc:      genesis.Alloc,
		ParentHash: genesis.ParentHash,
	}
	spec.Nonce = (hexutil.Bytes)(make([]byte, 8))
	binary.LittleEndian.PutUint64(spec.Nonce[:], genesis.Nonce)

	return spec, nil
}
