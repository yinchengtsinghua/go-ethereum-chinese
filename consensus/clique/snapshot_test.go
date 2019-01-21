
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
	"crypto/ecdsa"
	"sort"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
)

//TestAccountPool是一个用于维护当前活动的测试人员帐户的池，
//从下面测试中使用的文本名称映射到实际的以太坊私有
//能够签署事务的密钥。
type testerAccountPool struct {
	accounts map[string]*ecdsa.PrivateKey
}

func newTesterAccountPool() *testerAccountPool {
	return &testerAccountPool{
		accounts: make(map[string]*ecdsa.PrivateKey),
	}
}

//
//
func (ap *testerAccountPool) checkpoint(header *types.Header, signers []string) {
	auths := make([]common.Address, len(signers))
	for i, signer := range signers {
		auths[i] = ap.address(signer)
	}
	sort.Sort(signersAscending(auths))
	for i, auth := range auths {
		copy(header.Extra[extraVanity+i*common.AddressLength:], auth.Bytes())
	}
}

//地址通过标签检索测试人员帐户的以太坊地址，创建
//如果以前的帐户还不存在，则为新帐户。
func (ap *testerAccountPool) address(account string) common.Address {
//返回非地址的零帐户
	if account == "" {
		return common.Address{}
	}
//确保我们有帐户的持久密钥
	if ap.accounts[account] == nil {
		ap.accounts[account], _ = crypto.GenerateKey()
	}
//解析并返回以太坊地址
	return crypto.PubkeyToAddress(ap.accounts[account].PublicKey)
}

//符号为给定的块计算一组数字签名并将其嵌入
//回到标题。
func (ap *testerAccountPool) sign(header *types.Header, signer string) {
//确保我们有签名者的持久密钥
	if ap.accounts[signer] == nil {
		ap.accounts[signer], _ = crypto.GenerateKey()
	}
//在头上签名并将签名嵌入额外的数据中
	sig, _ := crypto.Sign(sigHash(header).Bytes(), ap.accounts[signer])
	copy(header.Extra[len(header.Extra)-extraSeal:], sig)
}

//testervote表示一个由parcitular帐户签名的单个块，其中
//该账户可能会或可能不会投集团票。
type testerVote struct {
	signer     string
	voted      string
	auth       bool
	checkpoint []string
	newbatch   bool
}

//测试团体签名者投票是否正确地评估各种简单和
//复杂的场景，以及一些特殊的角落案例会正确失败。
func TestClique(t *testing.T) {
//定义要测试的各种投票方案
	tests := []struct {
		epoch   uint64
		signers []string
		votes   []testerVote
		results []string
		failure error
	}{
		{
//单签名人，无投票权
			signers: []string{"A"},
			votes:   []testerVote{{signer: "A"}},
			results: []string{"A"},
		}, {
//单个签名人，投票添加两个其他人（只接受第一个，第二个需要2票）
			signers: []string{"A"},
			votes: []testerVote{
				{signer: "A", voted: "B", auth: true},
				{signer: "B"},
				{signer: "A", voted: "C", auth: true},
			},
			results: []string{"A", "B"},
		}, {
//两个签名者，投票加三个（只接受前两个，第三个已经需要3票）
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: true},
				{signer: "B", voted: "C", auth: true},
				{signer: "A", voted: "D", auth: true},
				{signer: "B", voted: "D", auth: true},
				{signer: "C"},
				{signer: "A", voted: "E", auth: true},
				{signer: "B", voted: "E", auth: true},
			},
			results: []string{"A", "B", "C", "D"},
		}, {
//单个签名者，放弃自己（很奇怪，但明确允许这样做的话就少了一个死角）
			signers: []string{"A"},
			votes: []testerVote{
				{signer: "A", voted: "A", auth: false},
			},
			results: []string{},
		}, {
//两个签名者，实际上需要双方同意放弃其中一个（未履行）
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A", voted: "B", auth: false},
			},
			results: []string{"A", "B"},
		}, {
//两个签名者，实际上需要双方同意放弃其中一个（满足）
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A", voted: "B", auth: false},
				{signer: "B", voted: "B", auth: false},
			},
			results: []string{"A"},
		}, {
//三个签名者，其中两个决定放弃第三个
			signers: []string{"A", "B", "C"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: false},
				{signer: "B", voted: "C", auth: false},
			},
			results: []string{"A", "B"},
		}, {
//四个签名者，两个的共识不足以让任何人放弃
			signers: []string{"A", "B", "C", "D"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: false},
				{signer: "B", voted: "C", auth: false},
			},
			results: []string{"A", "B", "C", "D"},
		}, {
//四个签名者，三个人的共识已经足够让某人离开
			signers: []string{"A", "B", "C", "D"},
			votes: []testerVote{
				{signer: "A", voted: "D", auth: false},
				{signer: "B", voted: "D", auth: false},
				{signer: "C", voted: "D", auth: false},
			},
			results: []string{"A", "B", "C"},
		}, {
//每个签名者对每个目标的授权计数一次
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: true},
				{signer: "B"},
				{signer: "A", voted: "C", auth: true},
				{signer: "B"},
				{signer: "A", voted: "C", auth: true},
			},
			results: []string{"A", "B"},
		}, {
//允许同时授权多个帐户
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: true},
				{signer: "B"},
				{signer: "A", voted: "D", auth: true},
				{signer: "B"},
				{signer: "A"},
				{signer: "B", voted: "D", auth: true},
				{signer: "A"},
				{signer: "B", voted: "C", auth: true},
			},
			results: []string{"A", "B", "C", "D"},
		}, {
//每个目标的每个签名者对取消授权计数一次
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A", voted: "B", auth: false},
				{signer: "B"},
				{signer: "A", voted: "B", auth: false},
				{signer: "B"},
				{signer: "A", voted: "B", auth: false},
			},
			results: []string{"A", "B"},
		}, {
//允许同时解除多个帐户的授权
			signers: []string{"A", "B", "C", "D"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: false},
				{signer: "B"},
				{signer: "C"},
				{signer: "A", voted: "D", auth: false},
				{signer: "B"},
				{signer: "C"},
				{signer: "A"},
				{signer: "B", voted: "D", auth: false},
				{signer: "C", voted: "D", auth: false},
				{signer: "A"},
				{signer: "B", voted: "C", auth: false},
			},
			results: []string{"A", "B"},
		}, {
//取消授权签名者的投票将立即被丢弃（取消授权投票）
			signers: []string{"A", "B", "C"},
			votes: []testerVote{
				{signer: "C", voted: "B", auth: false},
				{signer: "A", voted: "C", auth: false},
				{signer: "B", voted: "C", auth: false},
				{signer: "A", voted: "B", auth: false},
			},
			results: []string{"A", "B"},
		}, {
//来自未授权签名者的投票将立即丢弃（授权投票）
			signers: []string{"A", "B", "C"},
			votes: []testerVote{
				{signer: "C", voted: "B", auth: false},
				{signer: "A", voted: "C", auth: false},
				{signer: "B", voted: "C", auth: false},
				{signer: "A", voted: "B", auth: false},
			},
			results: []string{"A", "B"},
		}, {
//不允许级联更改，只有被投票的帐户才可以更改
			signers: []string{"A", "B", "C", "D"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: false},
				{signer: "B"},
				{signer: "C"},
				{signer: "A", voted: "D", auth: false},
				{signer: "B", voted: "C", auth: false},
				{signer: "C"},
				{signer: "A"},
				{signer: "B", voted: "D", auth: false},
				{signer: "C", voted: "D", auth: false},
			},
			results: []string{"A", "B", "C"},
		}, {
//达成共识的变化超出范围（通过deauth）触摸执行
			signers: []string{"A", "B", "C", "D"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: false},
				{signer: "B"},
				{signer: "C"},
				{signer: "A", voted: "D", auth: false},
				{signer: "B", voted: "C", auth: false},
				{signer: "C"},
				{signer: "A"},
				{signer: "B", voted: "D", auth: false},
				{signer: "C", voted: "D", auth: false},
				{signer: "A"},
				{signer: "C", voted: "C", auth: true},
			},
			results: []string{"A", "B"},
		}, {
//达成共识的变化（通过deauth）可能会在第一次接触时失去共识。
			signers: []string{"A", "B", "C", "D"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: false},
				{signer: "B"},
				{signer: "C"},
				{signer: "A", voted: "D", auth: false},
				{signer: "B", voted: "C", auth: false},
				{signer: "C"},
				{signer: "A"},
				{signer: "B", voted: "D", auth: false},
				{signer: "C", voted: "D", auth: false},
				{signer: "A"},
				{signer: "B", voted: "C", auth: true},
			},
			results: []string{"A", "B", "C"},
		}, {
//确保挂起的投票不会在授权状态更改后继续有效。这个
//只有快速添加、删除签名者，然后
//阅读（或相反），而其中一个最初的选民投了。如果A
//过去的投票被保存在系统中的某个位置，这将干扰
//最终签名者结果。
			signers: []string{"A", "B", "C", "D", "E"},
			votes: []testerVote{
{signer: "A", voted: "F", auth: true}, //授权F，需要3票
				{signer: "B", voted: "F", auth: true},
				{signer: "C", voted: "F", auth: true},
{signer: "D", voted: "F", auth: false}, //取消F的授权，需要4票（保持A以前的投票“不变”）。
				{signer: "E", voted: "F", auth: false},
				{signer: "B", voted: "F", auth: false},
				{signer: "C", voted: "F", auth: false},
{signer: "D", voted: "F", auth: true}, //几乎授权F，需要2/3票
				{signer: "E", voted: "F", auth: true},
{signer: "B", voted: "A", auth: false}, //取消授权A，需要3票
				{signer: "C", voted: "A", auth: false},
				{signer: "D", voted: "A", auth: false},
{signer: "B", voted: "F", auth: true}, //完成授权F，需要3/3票
			},
			results: []string{"B", "C", "D", "E", "F"},
		}, {
//epoch转换重置所有投票以允许链检查点
			epoch:   3,
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: true},
				{signer: "B"},
				{signer: "A", checkpoint: []string{"A", "B"}},
				{signer: "B", voted: "C", auth: true},
			},
			results: []string{"A", "B"},
		}, {
//
			signers: []string{"A"},
			votes: []testerVote{
				{signer: "B"},
			},
			failure: errUnauthorizedSigner,
		}, {
//签署了recenty的授权签署者不应再次签署
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A"},
				{signer: "A"},
			},
			failure: errRecentlySigned,
		}, {
//
			epoch:   3,
			signers: []string{"A", "B", "C"},
			votes: []testerVote{
				{signer: "A"},
				{signer: "B"},
				{signer: "A", checkpoint: []string{"A", "B", "C"}},
				{signer: "A"},
			},
			failure: errRecentlySigned,
		}, {
//最近的签名不应在新的
//批处理（https://github.com/ethereum/go-ethereum/issues/17593）。虽然这
//似乎过于具体和奇怪，这是一个由双方意见分歧。
			epoch:   3,
			signers: []string{"A", "B", "C"},
			votes: []testerVote{
				{signer: "A"},
				{signer: "B"},
				{signer: "A", checkpoint: []string{"A", "B", "C"}},
				{signer: "A", newbatch: true},
			},
			failure: errRecentlySigned,
		},
	}
//运行场景并测试它们
	for i, tt := range tests {
//创建帐户池并生成初始签名者集
		accounts := newTesterAccountPool()

		signers := make([]common.Address, len(tt.signers))
		for j, signer := range tt.signers {
			signers[j] = accounts.address(signer)
		}
		for j := 0; j < len(signers); j++ {
			for k := j + 1; k < len(signers); k++ {
				if bytes.Compare(signers[j][:], signers[k][:]) > 0 {
					signers[j], signers[k] = signers[k], signers[j]
				}
			}
		}
//使用初始签名者集创建Genesis块
		genesis := &core.Genesis{
			ExtraData: make([]byte, extraVanity+common.AddressLength*len(signers)+extraSeal),
		}
		for j, signer := range signers {
			copy(genesis.ExtraData[extraVanity+j*common.AddressLength:], signer[:])
		}
//创建一个原始的区块链，注入Genesis
		db := ethdb.NewMemDatabase()
		genesis.Commit(db)

//
		config := *params.TestChainConfig
		config.Clique = &params.CliqueConfig{
			Period: 1,
			Epoch:  tt.epoch,
		}
		engine := New(config.Clique, db)
		engine.fakeDiff = true

		blocks, _ := core.GenerateChain(&config, genesis.ToBlock(db), engine, db, len(tt.votes), func(j int, gen *core.BlockGen) {
//
			gen.SetCoinbase(accounts.address(tt.votes[j].voted))
			if tt.votes[j].auth {
				var nonce types.BlockNonce
				copy(nonce[:], nonceAuthVote)
				gen.SetNonce(nonce)
			}
		})
//遍历块并分别密封它们
		for j, block := range blocks {
//获取标题并准备签名
			header := block.Header()
			if j > 0 {
				header.ParentHash = blocks[j-1].Hash()
			}
			header.Extra = make([]byte, extraVanity+extraSeal)
			if auths := tt.votes[j].checkpoint; auths != nil {
				header.Extra = make([]byte, extraVanity+len(auths)*common.AddressLength+extraSeal)
				accounts.checkpoint(header, auths)
			}
header.Difficulty = diffInTurn //忽略，我们只需要一个有效的数字

//生成签名，将其嵌入头和块中
			accounts.sign(header, tt.votes[j].signer)
			blocks[j] = block.WithSeal(header)
		}
//将块拆分为单独的导入批次（Cornercase测试）
		batches := [][]*types.Block{nil}
		for j, block := range blocks {
			if tt.votes[j].newbatch {
				batches = append(batches, nil)
			}
			batches[len(batches)-1] = append(batches[len(batches)-1], block)
		}
//把所有的头条都传给小集团，确保理货成功。
		chain, err := core.NewBlockChain(db, nil, &config, engine, vm.Config{}, nil)
		if err != nil {
			t.Errorf("test %d: failed to create test chain: %v", i, err)
			continue
		}
		failed := false
		for j := 0; j < len(batches)-1; j++ {
			if k, err := chain.InsertChain(batches[j]); err != nil {
				t.Errorf("test %d: failed to import batch %d, block %d: %v", i, j, k, err)
				failed = true
				break
			}
		}
		if failed {
			continue
		}
		if _, err = chain.InsertChain(batches[len(batches)-1]); err != tt.failure {
			t.Errorf("test %d: failure mismatch: have %v, want %v", i, err, tt.failure)
		}
		if tt.failure != nil {
			continue
		}
//未生成或请求失败，生成最终投票快照
		head := blocks[len(blocks)-1]

		snap, err := engine.snapshot(chain, head.NumberU64(), head.Hash(), nil)
		if err != nil {
			t.Errorf("test %d: failed to retrieve voting snapshot: %v", i, err)
			continue
		}
//验证签名者的最终列表与预期列表
		signers = make([]common.Address, len(tt.results))
		for j, signer := range tt.results {
			signers[j] = accounts.address(signer)
		}
		for j := 0; j < len(signers); j++ {
			for k := j + 1; k < len(signers); k++ {
				if bytes.Compare(signers[j][:], signers[k][:]) > 0 {
					signers[j], signers[k] = signers[k], signers[j]
				}
			}
		}
		result := snap.signers()
		if len(result) != len(signers) {
			t.Errorf("test %d: signers mismatch: have %x, want %x", i, result, signers)
			continue
		}
		for j := 0; j < len(result); j++ {
			if !bytes.Equal(result[j][:], signers[j][:]) {
				t.Errorf("test %d, signer %d: signer mismatch: have %x, want %x", i, j, result[j], signers[j])
			}
		}
	}
}
