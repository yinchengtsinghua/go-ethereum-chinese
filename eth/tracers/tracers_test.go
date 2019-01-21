
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

package tracers

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/tests"
)

//要生成新的calltracer测试，请将下面的maketest方法复制粘贴到
//一个geth控制台，用要导出的事务散列调用它。

/*
//maketest通过运行预状态重新组装和
//调用trace run，将收集到的所有信息组装到一个测试用例中。
var maketest=功能（tx，倒带）
  //从块、事务和预存数据生成Genesis块
  var block=eth.getblock（eth.getTransaction（tx）.blockHash）；
  var genesis=eth.getblock（block.parenthash）；

  删除genesis.gasused；
  删除genesis.logsbloom；
  删除genesis.parenthash；
  删除genesis.receiptsroot；
  删除Genesis.sha3uncles；
  删除genesis.size；
  删除genesis.transactions；
  删除genesis.transactionsroot；
  删除genesis.uncles；

  genesis.gaslimit=genesis.gaslimit.toString（）；
  genesis.number=genesis.number.toString（）；
  genesis.timestamp=genesis.timestamp.toString（）；

  genesis.alloc=debug.traceTransaction（tx，tracer:“prestatedtracer”，rewind:rewind）；
  for（genesis.alloc中的var键）
    genesis.alloc[key].nonce=genesis.alloc[key].nonce.toString（）；
  }
  genesis.config=admin.nodeinfo.protocols.eth.config；

  //生成调用跟踪并生成测试输入
  var result=debug.traceTransaction（tx，tracer:“calltracer”，rewind:rewind）；
  删除result.time；

  console.log（json.stringify（
    创世纪：创世纪，
    语境：{
      编号：block.number.toString（），
      困难：障碍。困难，
      timestamp:block.timestamp.toString（），
      gaslimit:block.gaslimit.toString（），
      矿工：block.miner，
    }
    输入：eth.getrawtransaction（tx）
    结果：
  }，NULL，2）；
}
**/


//CallTrace是CallTracer运行的结果。
type callTrace struct {
	Type    string          `json:"type"`
	From    common.Address  `json:"from"`
	To      common.Address  `json:"to"`
	Input   hexutil.Bytes   `json:"input"`
	Output  hexutil.Bytes   `json:"output"`
	Gas     *hexutil.Uint64 `json:"gas,omitempty"`
	GasUsed *hexutil.Uint64 `json:"gasUsed,omitempty"`
	Value   *hexutil.Big    `json:"value,omitempty"`
	Error   string          `json:"error,omitempty"`
	Calls   []callTrace     `json:"calls,omitempty"`
}

type callContext struct {
	Number     math.HexOrDecimal64   `json:"number"`
	Difficulty *math.HexOrDecimal256 `json:"difficulty"`
	Time       math.HexOrDecimal64   `json:"timestamp"`
	GasLimit   math.HexOrDecimal64   `json:"gasLimit"`
	Miner      common.Address        `json:"miner"`
}

//CallTracerTest定义一个测试来检查调用跟踪程序。
type callTracerTest struct {
	Genesis *core.Genesis `json:"genesis"`
	Context *callContext  `json:"context"`
	Input   string        `json:"input"`
	Result  *callTrace    `json:"result"`
}

func TestPrestateTracerCreate2(t *testing.T) {
	unsigned_tx := types.NewTransaction(1, common.HexToAddress("0x00000000000000000000000000000000deadbeef"),
		new(big.Int), 5000000, big.NewInt(1), []byte{})

	privateKeyECDSA, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	signer := types.NewEIP155Signer(big.NewInt(1))
	tx, err := types.SignTx(unsigned_tx, signer, privateKeyECDSA)
	if err != nil {
		t.Fatalf("err %v", err)
	}
 /*
  这是来自瘦create2-eip上的一个测试向量

     地址：0x00000000000000000000000000000000标题
     salt 0x00000000000000000000000000000000000000000000000000000000cafebabe
     初始化代码0XDeadBeef
     gas (assuming no mem expansion): 32006
     结果：0x60f3f640a8508fc6a86d45df051962668e1e8ac7
 **/

	origin, _ := signer.Sender(tx)
	context := vm.Context{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		Origin:      origin,
		Coinbase:    common.Address{},
		BlockNumber: new(big.Int).SetUint64(8000000),
		Time:        new(big.Int).SetUint64(5),
		Difficulty:  big.NewInt(0x30000),
		GasLimit:    uint64(6000000),
		GasPrice:    big.NewInt(1),
	}
	alloc := core.GenesisAlloc{}
//代码将“死区”推入内存，然后其他参数，并调用create2，然后返回
//住址
	alloc[common.HexToAddress("0x00000000000000000000000000000000deadbeef")] = core.GenesisAccount{
		Nonce:   1,
		Code:    hexutil.MustDecode("0x63deadbeef60005263cafebabe6004601c6000F560005260206000F3"),
		Balance: big.NewInt(1),
	}
	alloc[origin] = core.GenesisAccount{
		Nonce:   1,
		Code:    []byte{},
		Balance: big.NewInt(500000000000000),
	}
	statedb := tests.MakePreState(ethdb.NewMemDatabase(), alloc)
//创建跟踪程序、EVM环境并运行它
	tracer, err := New("prestateTracer")
	if err != nil {
		t.Fatalf("failed to create call tracer: %v", err)
	}
	evm := vm.NewEVM(context, statedb, params.MainnetChainConfig, vm.Config{Debug: true, Tracer: tracer})

	msg, err := tx.AsMessage(signer)
	if err != nil {
		t.Fatalf("failed to prepare transaction for tracing: %v", err)
	}
	st := core.NewStateTransition(evm, msg, new(core.GasPool).AddGas(tx.Gas()))
	if _, _, _, err = st.TransitionDb(); err != nil {
		t.Fatalf("failed to execute transaction: %v", err)
	}
//检索跟踪结果并与标准具进行比较
	res, err := tracer.GetResult()
	if err != nil {
		t.Fatalf("failed to retrieve trace result: %v", err)
	}
	ret := make(map[string]interface{})
	if err := json.Unmarshal(res, &ret); err != nil {
		t.Fatalf("failed to unmarshal trace result: %v", err)
	}
	if _, has := ret["0x60f3f640a8508fc6a86d45df051962668e1e8ac7"]; !has {
		t.Fatalf("Expected 0x60f3f640a8508fc6a86d45df051962668e1e8ac7 in result")
	}
}

//迭代跟踪测试工具中的所有输入输出数据集，并
//对它们运行javascript跟踪程序。
func TestCallTracer(t *testing.T) {
	files, err := ioutil.ReadDir("testdata")
	if err != nil {
		t.Fatalf("failed to retrieve tracer test suite: %v", err)
	}
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), "call_tracer_") {
			continue
		}
file := file //捕获范围变量
		t.Run(camel(strings.TrimSuffix(strings.TrimPrefix(file.Name(), "call_tracer_"), ".json")), func(t *testing.T) {
			t.Parallel()

//找到呼叫追踪测试，从磁盘读取
			blob, err := ioutil.ReadFile(filepath.Join("testdata", file.Name()))
			if err != nil {
				t.Fatalf("failed to read testcase: %v", err)
			}
			test := new(callTracerTest)
			if err := json.Unmarshal(blob, test); err != nil {
				t.Fatalf("failed to parse testcase: %v", err)
			}
//使用给定的预状态配置区块链
			tx := new(types.Transaction)
			if err := rlp.DecodeBytes(common.FromHex(test.Input), tx); err != nil {
				t.Fatalf("failed to parse testcase input: %v", err)
			}
			signer := types.MakeSigner(test.Genesis.Config, new(big.Int).SetUint64(uint64(test.Context.Number)))
			origin, _ := signer.Sender(tx)

			context := vm.Context{
				CanTransfer: core.CanTransfer,
				Transfer:    core.Transfer,
				Origin:      origin,
				Coinbase:    test.Context.Miner,
				BlockNumber: new(big.Int).SetUint64(uint64(test.Context.Number)),
				Time:        new(big.Int).SetUint64(uint64(test.Context.Time)),
				Difficulty:  (*big.Int)(test.Context.Difficulty),
				GasLimit:    uint64(test.Context.GasLimit),
				GasPrice:    tx.GasPrice(),
			}
			statedb := tests.MakePreState(ethdb.NewMemDatabase(), test.Genesis.Alloc)

//创建跟踪程序、EVM环境并运行它
			tracer, err := New("callTracer")
			if err != nil {
				t.Fatalf("failed to create call tracer: %v", err)
			}
			evm := vm.NewEVM(context, statedb, test.Genesis.Config, vm.Config{Debug: true, Tracer: tracer})

			msg, err := tx.AsMessage(signer)
			if err != nil {
				t.Fatalf("failed to prepare transaction for tracing: %v", err)
			}
			st := core.NewStateTransition(evm, msg, new(core.GasPool).AddGas(tx.Gas()))
			if _, _, _, err = st.TransitionDb(); err != nil {
				t.Fatalf("failed to execute transaction: %v", err)
			}
//检索跟踪结果并与标准具进行比较
			res, err := tracer.GetResult()
			if err != nil {
				t.Fatalf("failed to retrieve trace result: %v", err)
			}
			ret := new(callTrace)
			if err := json.Unmarshal(res, ret); err != nil {
				t.Fatalf("failed to unmarshal trace result: %v", err)
			}

			if !reflect.DeepEqual(ret, test.Result) {
				t.Fatalf("trace mismatch: \nhave %+v\nwant %+v", ret, test.Result)
			}
		})
	}
}
