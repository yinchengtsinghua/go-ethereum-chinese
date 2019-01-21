
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

package storage

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage/mock"
)

type LocalStoreParams struct {
	*StoreParams
	ChunkDbPath string
	Validators  []ChunkValidator `toml:"-"`
}

func NewDefaultLocalStoreParams() *LocalStoreParams {
	return &LocalStoreParams{
		StoreParams: NewDefaultStoreParams(),
	}
}

//这只能在所有配置选项（文件、命令行、env vars）之后最终设置。
//已经过评估
func (p *LocalStoreParams) Init(path string) {
	if p.ChunkDbPath == "" {
		p.ChunkDbPath = filepath.Join(path, "chunks")
	}
}

//localstore是InMemory数据库与磁盘持久化数据库的组合
//使用任意2个chunkstore实现带有回退（缓存）逻辑的get/put
type LocalStore struct {
	Validators []ChunkValidator
	memStore   *MemStore
	DbStore    *LDBStore
	mu         sync.Mutex
}

//此构造函数使用memstore和dbstore作为组件
func NewLocalStore(params *LocalStoreParams, mockStore *mock.NodeStore) (*LocalStore, error) {
	ldbparams := NewLDBStoreParams(params.StoreParams, params.ChunkDbPath)
	dbStore, err := NewMockDbStore(ldbparams, mockStore)
	if err != nil {
		return nil, err
	}
	return &LocalStore{
		memStore:   NewMemStore(params.StoreParams, dbStore),
		DbStore:    dbStore,
		Validators: params.Validators,
	}, nil
}

func NewTestLocalStoreForAddr(params *LocalStoreParams) (*LocalStore, error) {
	ldbparams := NewLDBStoreParams(params.StoreParams, params.ChunkDbPath)
	dbStore, err := NewLDBStore(ldbparams)
	if err != nil {
		return nil, err
	}
	localStore := &LocalStore{
		memStore:   NewMemStore(params.StoreParams, dbStore),
		DbStore:    dbStore,
		Validators: params.Validators,
	}
	return localStore, nil
}

//如果块通过任何本地存储验证程序，则isvalid返回true。
//如果localstore没有验证器，isvalid也返回true。
func (ls *LocalStore) isValid(chunk Chunk) bool {
//默认情况下，块是有效的。如果我们有0个验证器，那么所有的块都是有效的。
	valid := true

//validators包含每个块类型一个validator的列表。
//如果一个验证器成功，那么块是有效的
	for _, v := range ls.Validators {
		if valid = v.Validate(chunk); valid {
			break
		}
	}
	return valid
}

//Put负责验证和存储块
//通过使用配置的chunkvalidator、memstore和ldbstore。
//如果区块无效，则其getErrored函数将
//返回errchunkinvalid。
//此方法将检查块是否已在memstore中
//如果是的话，它会退回的。如果出现错误
//memstore.get，将通过调用getErrored返回
//在块上。
//此方法负责关闭chunk.reqc通道
//当块存储在memstore中时。
//在ldbstore.put之后，确保memstore
//包含具有相同数据但没有reqc通道的块。
func (ls *LocalStore) Put(ctx context.Context, chunk Chunk) error {
	if !ls.isValid(chunk) {
		return ErrChunkInvalid
	}

	log.Trace("localstore.put", "key", chunk.Address())
	ls.mu.Lock()
	defer ls.mu.Unlock()

	_, err := ls.memStore.Get(ctx, chunk.Address())
	if err == nil {
		return nil
	}
	if err != nil && err != ErrChunkNotFound {
		return err
	}
	ls.memStore.Put(ctx, chunk)
	err = ls.DbStore.Put(ctx, chunk)
	return err
}

//get（chunk*chunk）在本地商店中查找一个chunk
//在获取块之前，此方法正在阻塞
//因此，如果
//chunkstore是远程的，可以有很长的延迟
func (ls *LocalStore) Get(ctx context.Context, addr Address) (chunk Chunk, err error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	return ls.get(ctx, addr)
}

func (ls *LocalStore) get(ctx context.Context, addr Address) (chunk Chunk, err error) {
	chunk, err = ls.memStore.Get(ctx, addr)

	if err != nil && err != ErrChunkNotFound {
		metrics.GetOrRegisterCounter("localstore.get.error", nil).Inc(1)
		return nil, err
	}

	if err == nil {
		metrics.GetOrRegisterCounter("localstore.get.cachehit", nil).Inc(1)
		go ls.DbStore.MarkAccessed(addr)
		return chunk, nil
	}

	metrics.GetOrRegisterCounter("localstore.get.cachemiss", nil).Inc(1)
	chunk, err = ls.DbStore.Get(ctx, addr)
	if err != nil {
		metrics.GetOrRegisterCounter("localstore.get.error", nil).Inc(1)
		return nil, err
	}

	ls.memStore.Put(ctx, chunk)
	return chunk, nil
}

func (ls *LocalStore) FetchFunc(ctx context.Context, addr Address) func(context.Context) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	_, err := ls.get(ctx, addr)
	if err == nil {
		return nil
	}
	return func(context.Context) error {
		return err
	}
}

func (ls *LocalStore) BinIndex(po uint8) uint64 {
	return ls.DbStore.BinIndex(po)
}

func (ls *LocalStore) Iterator(from uint64, to uint64, po uint8, f func(Address, uint64) bool) error {
	return ls.DbStore.SyncIterator(from, to, po, f)
}

//关闭本地存储
func (ls *LocalStore) Close() {
	ls.DbStore.Close()
}

//迁移检查数据存储架构与运行时架构并运行
//迁移如果不匹配
func (ls *LocalStore) Migrate() error {
	actualDbSchema, err := ls.DbStore.GetSchema()
	if err != nil {
		log.Error(err.Error())
		return err
	}

	if actualDbSchema == CurrentDbSchema {
		return nil
	}

	log.Debug("running migrations for", "schema", actualDbSchema, "runtime-schema", CurrentDbSchema)

	if actualDbSchema == DbSchemaNone {
		ls.migrateFromNoneToPurity()
		actualDbSchema = DbSchemaPurity
	}

	if err := ls.DbStore.PutSchema(actualDbSchema); err != nil {
		return err
	}

	if actualDbSchema == DbSchemaPurity {
		if err := ls.migrateFromPurityToHalloween(); err != nil {
			return err
		}
		actualDbSchema = DbSchemaHalloween
	}

	if err := ls.DbStore.PutSchema(actualDbSchema); err != nil {
		return err
	}
	return nil
}

func (ls *LocalStore) migrateFromNoneToPurity() {
//删除无效的块，即不通过的块
//任何ls.validator
	ls.DbStore.Cleanup(func(c *chunk) bool {
		return !ls.isValid(c)
	})
}

func (ls *LocalStore) migrateFromPurityToHalloween() error {
	return ls.DbStore.CleanGCIndex()
}
