
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

package eth

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
)

const (
//DefaultTraceTimeout是单个事务可以执行的时间量
//默认情况下，在强制中止之前。
	defaultTraceTimeout = 5 * time.Second

//defaulttracereexec是跟踪程序愿意返回的块数。
//重新执行以产生运行特定
//痕迹。
	defaultTraceReexec = uint64(128)
)

//traceconfig保存跟踪函数的额外参数。
type TraceConfig struct {
	*vm.LogConfig
	Tracer  *string
	Timeout *string
	Reexec  *uint64
}

//StdTraceConfig holds extra parameters to standard-json trace functions.
type StdTraceConfig struct {
	*vm.LogConfig
	Reexec *uint64
	TxHash common.Hash
}

//txtracesult是单个事务跟踪的结果。
type txTraceResult struct {
Result interface{} `json:"result,omitempty"` //示踪剂产生的示踪结果
Error  string      `json:"error,omitempty"`  //示踪剂产生的示踪失效
}

//当整个链为
//被追踪。
type blockTraceTask struct {
statedb *state.StateDB   //准备跟踪的中间状态
block   *types.Block     //用于跟踪事务的块
rootref common.Hash      //为此任务保留的trie根引用
results []*txTraceResult //跟踪结果按任务进行
}

//blocktraceresult represets当一个完整的
//正在跟踪链。
type blockTraceResult struct {
Block  hexutil.Uint64   `json:"block"`  //与此跟踪对应的块号
Hash   common.Hash      `json:"hash"`   //与此跟踪对应的块哈希
Traces []*txTraceResult `json:"traces"` //跟踪任务生成的结果
}

//txtracetask表示当整个块
//正在跟踪。
type txTraceTask struct {
statedb *state.StateDB //准备跟踪的中间状态
index   int            //块中的事务偏移量
}

//tracechain返回在执行evm期间创建的结构化日志
//在两个块之间（不包括start），并将它们作为JSON对象返回。
func (api *PrivateDebugAPI) TraceChain(ctx context.Context, start, end rpc.BlockNumber, config *TraceConfig) (*rpc.Subscription, error) {
//获取要跟踪的块间隔
	var from, to *types.Block

	switch start {
	case rpc.PendingBlockNumber:
		from = api.eth.miner.PendingBlock()
	case rpc.LatestBlockNumber:
		from = api.eth.blockchain.CurrentBlock()
	default:
		from = api.eth.blockchain.GetBlockByNumber(uint64(start))
	}
	switch end {
	case rpc.PendingBlockNumber:
		to = api.eth.miner.PendingBlock()
	case rpc.LatestBlockNumber:
		to = api.eth.blockchain.CurrentBlock()
	default:
		to = api.eth.blockchain.GetBlockByNumber(uint64(end))
	}
//如果我们找到了所有的区块，就追踪这条链。
	if from == nil {
		return nil, fmt.Errorf("starting block #%d not found", start)
	}
	if to == nil {
		return nil, fmt.Errorf("end block #%d not found", end)
	}
	if from.Number().Cmp(to.Number()) >= 0 {
		return nil, fmt.Errorf("end block (#%d) needs to come after start block (#%d)", end, start)
	}
	return api.traceChain(ctx, from, to, config)
}

//tracechain根据提供的配置配置配置新的跟踪程序，以及
//执行中包含的所有事务。返回值将是一个项目
//每个事务，取决于请求的跟踪程序。
func (api *PrivateDebugAPI) traceChain(ctx context.Context, start, end *types.Block, config *TraceConfig) (*rpc.Subscription, error) {
//跟踪链是一个**长**的操作，只处理订阅
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}
	sub := notifier.CreateSubscription()

//在进行任何工作之前，确保我们有一个有效的启动状态
	origin := start.NumberU64()
database := state.NewDatabaseWithCache(api.eth.ChainDb(), 16) //链追踪可能从Genesis开始。

	if number := start.NumberU64(); number > 0 {
		start = api.eth.blockchain.GetBlock(start.ParentHash(), start.NumberU64()-1)
		if start == nil {
			return nil, fmt.Errorf("parent block #%d not found", number-1)
		}
	}
	statedb, err := state.New(start.Root(), database)
	if err != nil {
//如果缺少起始状态，则允许重新执行一些块。
		reexec := defaultTraceReexec
		if config != nil && config.Reexec != nil {
			reexec = *config.Reexec
		}
//查找具有可用状态的最新块
		for i := uint64(0); i < reexec; i++ {
			start = api.eth.blockchain.GetBlock(start.ParentHash(), start.NumberU64()-1)
			if start == nil {
				break
			}
			if statedb, err = state.New(start.Root(), database); err == nil {
				break
			}
		}
//如果我们还没有州政府的支持，那就纾困吧。
		if err != nil {
			switch err.(type) {
			case *trie.MissingNodeError:
				return nil, errors.New("required historical state unavailable")
			default:
				return nil, err
			}
		}
	}
//为每个块同时执行链中包含的所有事务
	blocks := int(end.NumberU64() - origin)

	threads := runtime.NumCPU()
	if threads > blocks {
		threads = blocks
	}
	var (
		pend    = new(sync.WaitGroup)
		tasks   = make(chan *blockTraceTask, threads)
		results = make(chan *blockTraceTask, threads)
	)
	for th := 0; th < threads; th++ {
		pend.Add(1)
		go func() {
			defer pend.Done()

//获取并执行下一个块跟踪任务
			for task := range tasks {
				signer := types.MakeSigner(api.config, task.block.Number())

//跟踪包含在
				for i, tx := range task.block.Transactions() {
					msg, _ := tx.AsMessage(signer)
					vmctx := core.NewEVMContext(msg, task.block.Header(), api.eth.blockchain, nil)

					res, err := api.traceTx(ctx, msg, vmctx, task.statedb, config)
					if err != nil {
						task.results[i] = &txTraceResult{Error: err.Error()}
						log.Warn("Tracing failed", "hash", tx.Hash(), "block", task.block.NumberU64(), "err", err)
						break
					}
					task.statedb.Finalise(true)
					task.results[i] = &txTraceResult{Result: res}
				}
//将结果返回给用户或在拆卸时中止
				select {
				case results <- task:
				case <-notifier.Closed():
					return
				}
			}
		}()
	}
//启动一个GODUTIN将所有的块输入示踪剂
	begin := time.Now()

	go func() {
		var (
			logged time.Time
			number uint64
			traced uint64
			failed error
			proot  common.Hash
		)
//确保所有出口通道上的物品都被正确清理干净。
		defer func() {
			close(tasks)
			pend.Wait()

			switch {
			case failed != nil:
				log.Warn("Chain tracing failed", "start", start.NumberU64(), "end", end.NumberU64(), "transactions", traced, "elapsed", time.Since(begin), "err", failed)
			case number < end.NumberU64():
				log.Warn("Chain tracing aborted", "start", start.NumberU64(), "end", end.NumberU64(), "abort", number, "transactions", traced, "elapsed", time.Since(begin))
			default:
				log.Info("Chain tracing finished", "start", start.NumberU64(), "end", end.NumberU64(), "transactions", traced, "elapsed", time.Since(begin))
			}
			close(results)
		}()
//同时将所有块都输入跟踪程序以及快速处理
		for number = start.NumberU64() + 1; number <= end.NumberU64(); number++ {
//如果请求中断，则停止跟踪
			select {
			case <-notifier.Closed():
				return
			default:
			}
//如果经过足够长的时间，则打印进度日志
			if time.Since(logged) > 8*time.Second {
				if number > origin {
					nodes, imgs := database.TrieDB().Size()
					log.Info("Tracing chain segment", "start", origin, "end", end.NumberU64(), "current", number, "transactions", traced, "elapsed", time.Since(begin), "memory", nodes+imgs)
				} else {
					log.Info("Preparing state for chain trace", "block", number, "start", origin, "elapsed", time.Since(begin))
				}
				logged = time.Now()
			}
//检索下一个要跟踪的块
			block := api.eth.blockchain.GetBlockByNumber(number)
			if block == nil {
				failed = fmt.Errorf("block #%d not found", number)
				break
			}
//将块发送到并发跟踪程序（如果不是在快进阶段）
			if number > origin {
				txs := block.Transactions()

				select {
				case tasks <- &blockTraceTask{statedb: statedb.Copy(), block: block, rootref: proot, results: make([]*txTraceResult, len(txs))}:
				case <-notifier.Closed():
					return
				}
				traced += uint64(len(txs))
			}
//快速生成下一个状态快照，无需跟踪
			_, _, _, err := api.eth.blockchain.Processor().Process(block, statedb, vm.Config{})
			if err != nil {
				failed = err
				break
			}
//最终确定状态，以便将任何修改写入trie
			root, err := statedb.Commit(true)
			if err != nil {
				failed = err
				break
			}
			if err := statedb.Reset(root); err != nil {
				failed = err
				break
			}
//两次参考Trie，一次为我们，一次为示踪剂
			database.TrieDB().Reference(root, common.Hash{})
			if number >= origin {
				database.TrieDB().Reference(root, common.Hash{})
			}
//取消引用我们自己已经完成的所有尝试
			if proot != (common.Hash{}) {
				database.TrieDB().Dereference(proot)
			}
			proot = root

//托多（卡拉贝拉）：我们需要预成像吗？他们不会积累太多吗？
		}
	}()

//继续读取跟踪结果并将其传输给用户
	go func() {
		var (
			done = make(map[uint64]*blockTraceResult)
			next = origin + 1
		)
		for res := range results {
//排队等待下一个接收结果
			result := &blockTraceResult{
				Block:  hexutil.Uint64(res.block.NumberU64()),
				Hash:   res.block.Hash(),
				Traces: res.results,
			}
			done[uint64(result.Block)] = result

//取消引用此任务在内存中保留的任何paret尝试
			database.TrieDB().Dereference(res.rootref)

//流完成对用户的跟踪，在第一个错误上中止
			for result, ok := done[next]; ok; result, ok = done[next] {
				if len(result.Traces) > 0 || next == end.NumberU64() {
					notifier.Notify(sub.ID, result)
				}
				delete(done, next)
				next++
			}
		}
	}()
	return sub, nil
}

//traceBlockByNumber返回在执行期间创建的结构化日志
//EVM并将其作为JSON对象返回。
func (api *PrivateDebugAPI) TraceBlockByNumber(ctx context.Context, number rpc.BlockNumber, config *TraceConfig) ([]*txTraceResult, error) {
//获取要跟踪的块
	var block *types.Block

	switch number {
	case rpc.PendingBlockNumber:
		block = api.eth.miner.PendingBlock()
	case rpc.LatestBlockNumber:
		block = api.eth.blockchain.CurrentBlock()
	default:
		block = api.eth.blockchain.GetBlockByNumber(uint64(number))
	}
//如果找到块，跟踪它
	if block == nil {
		return nil, fmt.Errorf("block #%d not found", number)
	}
	return api.traceBlock(ctx, block, config)
}

//traceBlockByHash返回在执行期间创建的结构化日志
//EVM并将其作为JSON对象返回。
func (api *PrivateDebugAPI) TraceBlockByHash(ctx context.Context, hash common.Hash, config *TraceConfig) ([]*txTraceResult, error) {
	block := api.eth.blockchain.GetBlockByHash(hash)
	if block == nil {
		return nil, fmt.Errorf("block %#x not found", hash)
	}
	return api.traceBlock(ctx, block, config)
}

//traceblock返回在执行evm期间创建的结构化日志
//并将它们作为JSON对象返回。
func (api *PrivateDebugAPI) TraceBlock(ctx context.Context, blob []byte, config *TraceConfig) ([]*txTraceResult, error) {
	block := new(types.Block)
	if err := rlp.Decode(bytes.NewReader(blob), block); err != nil {
		return nil, fmt.Errorf("could not decode block: %v", err)
	}
	return api.traceBlock(ctx, block, config)
}

//traceblockfromfile返回在执行期间创建的结构化日志
//EVM并将其作为JSON对象返回。
func (api *PrivateDebugAPI) TraceBlockFromFile(ctx context.Context, file string, config *TraceConfig) ([]*txTraceResult, error) {
	blob, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %v", err)
	}
	return api.TraceBlock(ctx, blob, config)
}

//tracebadblockbyhash返回在执行期间创建的结构化日志
//EVM针对从坏块池中提取的块，并将其作为JSON返回
//对象。
func (api *PrivateDebugAPI) TraceBadBlock(ctx context.Context, hash common.Hash, config *TraceConfig) ([]*txTraceResult, error) {
	blocks := api.eth.blockchain.BadBlocks()
	for _, block := range blocks {
		if block.Hash() == hash {
			return api.traceBlock(ctx, block, config)
		}
	}
	return nil, fmt.Errorf("bad block %#x not found", hash)
}

//StutalTraceBufftoFILE转储了在
//将EVM执行到本地文件系统并返回文件列表
//给呼叫者。
func (api *PrivateDebugAPI) StandardTraceBlockToFile(ctx context.Context, hash common.Hash, config *StdTraceConfig) ([]string, error) {
	block := api.eth.blockchain.GetBlockByHash(hash)
	if block == nil {
		return nil, fmt.Errorf("block %#x not found", hash)
	}
	return api.standardTraceBlockToFile(ctx, block, config)
}

//StandardTraceBadBlockToFile转储在
//对从坏的池中拉到的块执行EVM
//本地文件系统，并将文件列表返回给调用方。
func (api *PrivateDebugAPI) StandardTraceBadBlockToFile(ctx context.Context, hash common.Hash, config *StdTraceConfig) ([]string, error) {
	blocks := api.eth.blockchain.BadBlocks()
	for _, block := range blocks {
		if block.Hash() == hash {
			return api.standardTraceBlockToFile(ctx, block, config)
		}
	}
	return nil, fmt.Errorf("bad block %#x not found", hash)
}

//跟踪块根据提供的配置配置配置新的跟踪程序，以及
//执行中包含的所有事务。返回值将是一个项目
//每个事务，取决于请求的跟踪程序。
func (api *PrivateDebugAPI) traceBlock(ctx context.Context, block *types.Block, config *TraceConfig) ([]*txTraceResult, error) {
//创建父状态数据库
	if err := api.eth.engine.VerifyHeader(api.eth.blockchain, block.Header(), true); err != nil {
		return nil, err
	}
	parent := api.eth.blockchain.GetBlock(block.ParentHash(), block.NumberU64()-1)
	if parent == nil {
		return nil, fmt.Errorf("parent %#x not found", block.ParentHash())
	}
	reexec := defaultTraceReexec
	if config != nil && config.Reexec != nil {
		reexec = *config.Reexec
	}
	statedb, err := api.computeStateDB(parent, reexec)
	if err != nil {
		return nil, err
	}
//同时执行块内包含的所有事务
	var (
		signer = types.MakeSigner(api.config, block.Number())

		txs     = block.Transactions()
		results = make([]*txTraceResult, len(txs))

		pend = new(sync.WaitGroup)
		jobs = make(chan *txTraceTask, len(txs))
	)
	threads := runtime.NumCPU()
	if threads > len(txs) {
		threads = len(txs)
	}
	for th := 0; th < threads; th++ {
		pend.Add(1)
		go func() {
			defer pend.Done()

//获取并执行下一个事务跟踪任务
			for task := range jobs {
				msg, _ := txs[task.index].AsMessage(signer)
				vmctx := core.NewEVMContext(msg, block.Header(), api.eth.blockchain, nil)

				res, err := api.traceTx(ctx, msg, vmctx, task.statedb, config)
				if err != nil {
					results[task.index] = &txTraceResult{Error: err.Error()}
					continue
				}
				results[task.index] = &txTraceResult{Result: res}
			}
		}()
	}
//将事务输入跟踪程序并返回
	var failed error
	for i, tx := range txs {
//Send the trace task over for execution
		jobs <- &txTraceTask{statedb: statedb.Copy(), index: i}

//快速生成下一个状态快照，无需跟踪
		msg, _ := tx.AsMessage(signer)
		vmctx := core.NewEVMContext(msg, block.Header(), api.eth.blockchain, nil)

		vmenv := vm.NewEVM(vmctx, statedb, api.config, vm.Config{})
		if _, _, _, err := core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(msg.Gas())); err != nil {
			failed = err
			break
		}
//最终确定状态，以便将任何修改写入trie
		statedb.Finalise(true)
	}
	close(jobs)
	pend.Wait()

//如果执行失败，则中止
	if failed != nil {
		return nil, failed
	}
	return results, nil
}

//StandardTraceBlockToFile配置使用标准JSON输出的新跟踪程序，
//并跟踪完整块或单个事务。返回值将
//每个事务跟踪一个文件名。
func (api *PrivateDebugAPI) standardTraceBlockToFile(ctx context.Context, block *types.Block, config *StdTraceConfig) ([]string, error) {
//如果我们在跟踪单个事务，请确保它存在
	if config != nil && config.TxHash != (common.Hash{}) {
		var exists bool
		for _, tx := range block.Transactions() {
			if exists = (tx.Hash() == config.TxHash); exists {
				break
			}
		}
		if !exists {
			return nil, fmt.Errorf("transaction %#x not found in block", config.TxHash)
		}
	}
//创建父状态数据库
	if err := api.eth.engine.VerifyHeader(api.eth.blockchain, block.Header(), true); err != nil {
		return nil, err
	}
	parent := api.eth.blockchain.GetBlock(block.ParentHash(), block.NumberU64()-1)
	if parent == nil {
		return nil, fmt.Errorf("parent %#x not found", block.ParentHash())
	}
	reexec := defaultTraceReexec
	if config != nil && config.Reexec != nil {
		reexec = *config.Reexec
	}
	statedb, err := api.computeStateDB(parent, reexec)
	if err != nil {
		return nil, err
	}
//检索跟踪配置，或使用默认值
	var (
		logConfig vm.LogConfig
		txHash    common.Hash
	)
	if config != nil {
		if config.LogConfig != nil {
			logConfig = *config.LogConfig
		}
		txHash = config.TxHash
	}
	logConfig.Debug = true

//Execute transaction, either tracing all or just the requested one
	var (
		signer = types.MakeSigner(api.config, block.Number())
		dumps  []string
	)
	for i, tx := range block.Transactions() {
//为未跟踪的执行准备传输
		var (
			msg, _ = tx.AsMessage(signer)
			vmctx  = core.NewEVMContext(msg, block.Header(), api.eth.blockchain, nil)

			vmConf vm.Config
			dump   *os.File
			err    error
		)
//如果事务需要跟踪，则交换配置
		if tx.Hash() == txHash || txHash == (common.Hash{}) {
//生成唯一的临时文件以将其转储到
			prefix := fmt.Sprintf("block_%#x-%d-%#x-", block.Hash().Bytes()[:4], i, tx.Hash().Bytes()[:4])

			dump, err = ioutil.TempFile(os.TempDir(), prefix)
			if err != nil {
				return nil, err
			}
			dumps = append(dumps, dump.Name())

//将noop记录器换成标准跟踪程序
			vmConf = vm.Config{
				Debug:                   true,
				Tracer:                  vm.NewJSONLogger(&logConfig, bufio.NewWriter(dump)),
				EnablePreimageRecording: true,
			}
		}
//执行事务并将任何跟踪刷新到磁盘
		vmenv := vm.NewEVM(vmctx, statedb, api.config, vmConf)
		_, _, _, err = core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(msg.Gas()))

		if dump != nil {
			dump.Close()
			log.Info("Wrote standard trace", "file", dump.Name())
		}
		if err != nil {
			return dumps, err
		}
//最终确定状态，以便将任何修改写入trie
		statedb.Finalise(true)

//如果我们跟踪了我们要查找的事务，则中止
		if tx.Hash() == txHash {
			break
		}
	}
	return dumps, nil
}

//ComputeTestedB检索与某个块关联的状态数据库。
//如果给定块没有本地可用的状态，则有许多块
//试图重新执行以生成所需状态。
func (api *PrivateDebugAPI) computeStateDB(block *types.Block, reexec uint64) (*state.StateDB, error) {
//如果我们的状态完全可用，请使用
	statedb, err := api.eth.blockchain.StateAt(block.Root())
	if err == nil {
		return statedb, nil
	}
//否则，尝试重新执行块，直到找到状态或达到限制
	origin := block.NumberU64()
	database := state.NewDatabaseWithCache(api.eth.ChainDb(), 16)

	for i := uint64(0); i < reexec; i++ {
		block = api.eth.blockchain.GetBlock(block.ParentHash(), block.NumberU64()-1)
		if block == nil {
			break
		}
		if statedb, err = state.New(block.Root(), database); err == nil {
			break
		}
	}
	if err != nil {
		switch err.(type) {
		case *trie.MissingNodeError:
			return nil, fmt.Errorf("required historical state unavailable (reexec=%d)", reexec)
		default:
			return nil, err
		}
	}
//状态在历史点可用，重新生成
	var (
		start  = time.Now()
		logged time.Time
		proot  common.Hash
	)
	for block.NumberU64() < origin {
//如果经过足够长的时间，则打印进度日志
		if time.Since(logged) > 8*time.Second {
			log.Info("Regenerating historical state", "block", block.NumberU64()+1, "target", origin, "remaining", origin-block.NumberU64()-1, "elapsed", time.Since(start))
			logged = time.Now()
		}
//检索下一个块以重新生成并处理它
		if block = api.eth.blockchain.GetBlockByNumber(block.NumberU64() + 1); block == nil {
			return nil, fmt.Errorf("block #%d not found", block.NumberU64()+1)
		}
		_, _, _, err := api.eth.blockchain.Processor().Process(block, statedb, vm.Config{})
		if err != nil {
			return nil, fmt.Errorf("processing block %d failed: %v", block.NumberU64(), err)
		}
//最终确定状态，以便将任何修改写入trie
		root, err := statedb.Commit(api.eth.blockchain.Config().IsEIP158(block.Number()))
		if err != nil {
			return nil, err
		}
		if err := statedb.Reset(root); err != nil {
			return nil, fmt.Errorf("state reset after block %d failed: %v", block.NumberU64(), err)
		}
		database.TrieDB().Reference(root, common.Hash{})
		if proot != (common.Hash{}) {
			database.TrieDB().Dereference(proot)
		}
		proot = root
	}
	nodes, imgs := database.TrieDB().Size()
	log.Info("Historical state regenerated", "block", block.NumberU64(), "elapsed", time.Since(start), "nodes", nodes, "preimages", imgs)
	return statedb, nil
}

//traceTransaction返回执行evm期间创建的结构化日志
//并将它们作为JSON对象返回。
func (api *PrivateDebugAPI) TraceTransaction(ctx context.Context, hash common.Hash, config *TraceConfig) (interface{}, error) {
//检索事务并组装其EVM上下文
	tx, blockHash, _, index := rawdb.ReadTransaction(api.eth.ChainDb(), hash)
	if tx == nil {
		return nil, fmt.Errorf("transaction %#x not found", hash)
	}
	reexec := defaultTraceReexec
	if config != nil && config.Reexec != nil {
		reexec = *config.Reexec
	}
	msg, vmctx, statedb, err := api.computeTxEnv(blockHash, int(index), reexec)
	if err != nil {
		return nil, err
	}
//跟踪事务和返回
	return api.traceTx(ctx, msg, vmctx, statedb, config)
}

//tracetx根据提供的配置配置配置新的跟踪程序，以及
//在提供的环境中执行给定的消息。返回值将
//be tracer dependent.
func (api *PrivateDebugAPI) traceTx(ctx context.Context, message core.Message, vmctx vm.Context, statedb *state.StateDB, config *TraceConfig) (interface{}, error) {
//组装结构化记录器或JavaScript跟踪程序
	var (
		tracer vm.Tracer
		err    error
	)
	switch {
	case config != nil && config.Tracer != nil:
//定义单个事务跟踪的有意义的超时
		timeout := defaultTraceTimeout
		if config.Timeout != nil {
			if timeout, err = time.ParseDuration(*config.Timeout); err != nil {
				return nil, err
			}
		}
//构造要用其执行的javascript跟踪程序
		if tracer, err = tracers.New(*config.Tracer); err != nil {
			return nil, err
		}
//处理超时和RPC取消
		deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
		go func() {
			<-deadlineCtx.Done()
			tracer.(*tracers.Tracer).Stop(errors.New("execution timeout"))
		}()
		defer cancel()

	case config == nil:
		tracer = vm.NewStructLogger(nil)

	default:
		tracer = vm.NewStructLogger(config.LogConfig)
	}
//在启用跟踪的情况下运行事务。
	vmenv := vm.NewEVM(vmctx, statedb, api.config, vm.Config{Debug: true, Tracer: tracer})

	ret, gas, failed, err := core.ApplyMessage(vmenv, message, new(core.GasPool).AddGas(message.Gas()))
	if err != nil {
		return nil, fmt.Errorf("tracing failed: %v", err)
	}
//根据跟踪类型、格式和返回输出
	switch tracer := tracer.(type) {
	case *vm.StructLogger:
		return &ethapi.ExecutionResult{
			Gas:         gas,
			Failed:      failed,
			ReturnValue: fmt.Sprintf("%x", ret),
			StructLogs:  ethapi.FormatLogs(tracer.StructLogs()),
		}, nil

	case *tracers.Tracer:
		return tracer.GetResult()

	default:
		panic(fmt.Sprintf("bad tracer type %T", tracer))
	}
}

//computetxenv返回特定事务的执行环境。
func (api *PrivateDebugAPI) computeTxEnv(blockHash common.Hash, txIndex int, reexec uint64) (core.Message, vm.Context, *state.StateDB, error) {
//创建父状态数据库
	block := api.eth.blockchain.GetBlockByHash(blockHash)
	if block == nil {
		return nil, vm.Context{}, nil, fmt.Errorf("block %#x not found", blockHash)
	}
	parent := api.eth.blockchain.GetBlock(block.ParentHash(), block.NumberU64()-1)
	if parent == nil {
		return nil, vm.Context{}, nil, fmt.Errorf("parent %#x not found", block.ParentHash())
	}
	statedb, err := api.computeStateDB(parent, reexec)
	if err != nil {
		return nil, vm.Context{}, nil, err
	}
//重新计算达到目标索引的事务。
	signer := types.MakeSigner(api.config, block.Number())

	for idx, tx := range block.Transactions() {
//Assemble the transaction call message and return if the requested offset
		msg, _ := tx.AsMessage(signer)
		context := core.NewEVMContext(msg, block.Header(), api.eth.blockchain, nil)
		if idx == txIndex {
			return msg, context, statedb, nil
		}
//尚未搜索到事务，请在当前状态的基础上执行
		vmenv := vm.NewEVM(context, statedb, api.config, vm.Config{})
		if _, _, _, err := core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(tx.Gas())); err != nil {
			return nil, vm.Context{}, nil, fmt.Errorf("transaction %#x failed: %v", tx.Hash(), err)
		}
//确保对国家进行任何修改
		statedb.Finalise(true)
	}
	return nil, vm.Context{}, nil, fmt.Errorf("transaction index %d out of range for block %#x", txIndex, blockHash)
}
