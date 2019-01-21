
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

//包ethash实现ethash工作证明共识引擎。
package ethash

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	mmap "github.com/edsrzf/mmap-go"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/hashicorp/golang-lru/simplelru"
)

var ErrInvalidDumpMagic = errors.New("invalid dump magic")

var (
//two256是表示2^256的大整数
	two256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))

//sharedethash是可以在多个用户之间共享的完整实例。
	sharedEthash = New(Config{"", 3, 0, "", 1, 0, ModeNormal}, nil, false)

//AlgorithmRevision是用于文件命名的数据结构版本。
	algorithmRevision = 23

//dumpmagic是一个数据集转储头，用于检查数据转储是否正常。
	dumpMagic = []uint32{0xbaddcafe, 0xfee1dead}
)

//Islittleendian返回本地系统是以小规模还是大规模运行
//结束字节顺序。
func isLittleEndian() bool {
	n := uint32(0x01020304)
	return *(*byte)(unsafe.Pointer(&n)) == 0x04
}

//memory map尝试为只读访问存储uint32s的映射文件。
func memoryMap(path string) (*os.File, mmap.MMap, []uint32, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, nil, nil, err
	}
	mem, buffer, err := memoryMapFile(file, false)
	if err != nil {
		file.Close()
		return nil, nil, nil, err
	}
	for i, magic := range dumpMagic {
		if buffer[i] != magic {
			mem.Unmap()
			file.Close()
			return nil, nil, nil, ErrInvalidDumpMagic
		}
	}
	return file, mem, buffer[len(dumpMagic):], err
}

//memoryMapFile尝试对已打开的文件描述符进行内存映射。
func memoryMapFile(file *os.File, write bool) (mmap.MMap, []uint32, error) {
//尝试内存映射文件
	flag := mmap.RDONLY
	if write {
		flag = mmap.RDWR
	}
	mem, err := mmap.Map(file, flag, 0)
	if err != nil {
		return nil, nil, err
	}
//是的，我们设法记忆地图文件，这里是龙。
	header := *(*reflect.SliceHeader)(unsafe.Pointer(&mem))
	header.Len /= 4
	header.Cap /= 4

	return mem, *(*[]uint32)(unsafe.Pointer(&header)), nil
}

//memoryMandGenerate尝试对uint32s的临时文件进行内存映射以进行写入
//访问，用生成器中的数据填充它，然后将其移动到最终版本
//请求路径。
func memoryMapAndGenerate(path string, size uint64, generator func(buffer []uint32)) (*os.File, mmap.MMap, []uint32, error) {
//确保数据文件夹存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, nil, nil, err
	}
//创建一个巨大的临时空文件来填充数据
	temp := path + "." + strconv.Itoa(rand.Int())

	dump, err := os.Create(temp)
	if err != nil {
		return nil, nil, nil, err
	}
	if err = dump.Truncate(int64(len(dumpMagic))*4 + int64(size)); err != nil {
		return nil, nil, nil, err
	}
//内存映射要写入的文件并用生成器填充它
	mem, buffer, err := memoryMapFile(dump, true)
	if err != nil {
		dump.Close()
		return nil, nil, nil, err
	}
	copy(buffer, dumpMagic)

	data := buffer[len(dumpMagic):]
	generator(data)

	if err := mem.Unmap(); err != nil {
		return nil, nil, nil, err
	}
	if err := dump.Close(); err != nil {
		return nil, nil, nil, err
	}
	if err := os.Rename(temp, path); err != nil {
		return nil, nil, nil, err
	}
	return memoryMap(path)
}

//lru按缓存或数据集的最后使用时间跟踪它们，最多保留n个缓存或数据集。
type lru struct {
	what string
	new  func(epoch uint64) interface{}
	mu   sync.Mutex
//项目保存在LRU缓存中，但有一种特殊情况：
//我们总是保留一个项目为（最高看到的时代）+1作为“未来项目”。
	cache      *simplelru.LRU
	future     uint64
	futureItem interface{}
}

//newlru为验证缓存创建新的最近使用最少的缓存
//或挖掘数据集。
func newlru(what string, maxItems int, new func(epoch uint64) interface{}) *lru {
	if maxItems <= 0 {
		maxItems = 1
	}
	cache, _ := simplelru.NewLRU(maxItems, func(key, value interface{}) {
		log.Trace("Evicted ethash "+what, "epoch", key)
	})
	return &lru{what: what, new: new, cache: cache}
}

//get为给定的epoch检索或创建项。第一个返回值总是
//非零。如果LRU认为某个项目在
//不久的将来。
func (lru *lru) get(epoch uint64) (item, future interface{}) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

//获取或创建请求的epoch的项。
	item, ok := lru.cache.Get(epoch)
	if !ok {
		if lru.future > 0 && lru.future == epoch {
			item = lru.futureItem
		} else {
			log.Trace("Requiring new ethash "+lru.what, "epoch", epoch)
			item = lru.new(epoch)
		}
		lru.cache.Add(epoch, item)
	}
//如果epoch大于以前看到的值，则更新“future item”。
	if epoch < maxEpoch-1 && lru.future < epoch+1 {
		log.Trace("Requiring new future ethash "+lru.what, "epoch", epoch+1)
		future = lru.new(epoch + 1)
		lru.future = epoch + 1
		lru.futureItem = future
	}
	return item, future
}

//cache用一些元数据包装ethash缓存，以便于并发使用。
type cache struct {
epoch uint64    //与此缓存相关的epoch
dump  *os.File  //内存映射缓存的文件描述符
mmap  mmap.MMap //释放前内存映射到取消映射
cache []uint32  //实际缓存数据内容（可能是内存映射）
once  sync.Once //确保只生成一次缓存
}

//new cache创建一个新的ethash验证缓存，并将其作为普通缓存返回
//可在LRU缓存中使用的接口。
func newCache(epoch uint64) interface{} {
	return &cache{epoch: epoch}
}

//generate确保在使用前生成缓存内容。
func (c *cache) generate(dir string, limit int, test bool) {
	c.once.Do(func() {
		size := cacheSize(c.epoch*epochLength + 1)
		seed := seedHash(c.epoch*epochLength + 1)
		if test {
			size = 1024
		}
//如果我们不在磁盘上存储任何内容，则生成并返回。
		if dir == "" {
			c.cache = make([]uint32, size/4)
			generateCache(c.cache, c.epoch, seed)
			return
		}
//磁盘存储是必需的，这会变得花哨。
		var endian string
		if !isLittleEndian() {
			endian = ".be"
		}
		path := filepath.Join(dir, fmt.Sprintf("cache-R%d-%x%s", algorithmRevision, seed[:8], endian))
		logger := log.New("epoch", c.epoch)

//我们将对该文件进行mmap，确保在
//缓存将变为未使用。
		runtime.SetFinalizer(c, (*cache).finalizer)

//尝试从磁盘和内存中加载文件
		var err error
		c.dump, c.mmap, c.cache, err = memoryMap(path)
		if err == nil {
			logger.Debug("Loaded old ethash cache from disk")
			return
		}
		logger.Debug("Failed to load old ethash cache", "err", err)

//以前没有可用的缓存，请创建新的缓存文件以填充
		c.dump, c.mmap, c.cache, err = memoryMapAndGenerate(path, size, func(buffer []uint32) { generateCache(buffer, c.epoch, seed) })
		if err != nil {
			logger.Error("Failed to generate mapped ethash cache", "err", err)

			c.cache = make([]uint32, size/4)
			generateCache(c.cache, c.epoch, seed)
		}
//迭代所有以前的实例并删除旧实例
		for ep := int(c.epoch) - limit; ep >= 0; ep-- {
			seed := seedHash(uint64(ep)*epochLength + 1)
			path := filepath.Join(dir, fmt.Sprintf("cache-R%d-%x%s", algorithmRevision, seed[:8], endian))
			os.Remove(path)
		}
	})
}

//终结器取消映射内存并关闭文件。
func (c *cache) finalizer() {
	if c.mmap != nil {
		c.mmap.Unmap()
		c.dump.Close()
		c.mmap, c.dump = nil, nil
	}
}

//数据集使用一些元数据包装ethash数据集，以便于并发使用。
type dataset struct {
epoch   uint64    //与此缓存相关的epoch
dump    *os.File  //内存映射缓存的文件描述符
mmap    mmap.MMap //释放前内存映射到取消映射
dataset []uint32  //实际缓存数据内容
once    sync.Once //确保只生成一次缓存
done    uint32    //用于确定生成状态的原子标记
}

//NewDataSet创建一个新的ethash挖掘数据集，并将其作为简单的go返回
//可在LRU缓存中使用的接口。
func newDataset(epoch uint64) interface{} {
	return &dataset{epoch: epoch}
}

//生成确保在使用前生成数据集内容。
func (d *dataset) generate(dir string, limit int, test bool) {
	d.once.Do(func() {
//标记完成后生成的数据集。这是遥控器需要的
		defer atomic.StoreUint32(&d.done, 1)

		csize := cacheSize(d.epoch*epochLength + 1)
		dsize := datasetSize(d.epoch*epochLength + 1)
		seed := seedHash(d.epoch*epochLength + 1)
		if test {
			csize = 1024
			dsize = 32 * 1024
		}
//如果我们不在磁盘上存储任何内容，则生成并返回
		if dir == "" {
			cache := make([]uint32, csize/4)
			generateCache(cache, d.epoch, seed)

			d.dataset = make([]uint32, dsize/4)
			generateDataset(d.dataset, d.epoch, cache)

			return
		}
//磁盘存储是必需的，这会变得花哨。
		var endian string
		if !isLittleEndian() {
			endian = ".be"
		}
		path := filepath.Join(dir, fmt.Sprintf("full-R%d-%x%s", algorithmRevision, seed[:8], endian))
		logger := log.New("epoch", d.epoch)

//我们将对该文件进行mmap，确保在
//缓存将变为未使用。
		runtime.SetFinalizer(d, (*dataset).finalizer)

//尝试从磁盘和内存中加载文件
		var err error
		d.dump, d.mmap, d.dataset, err = memoryMap(path)
		if err == nil {
			logger.Debug("Loaded old ethash dataset from disk")
			return
		}
		logger.Debug("Failed to load old ethash dataset", "err", err)

//没有以前的数据集可用，请创建新的数据集文件来填充
		cache := make([]uint32, csize/4)
		generateCache(cache, d.epoch, seed)

		d.dump, d.mmap, d.dataset, err = memoryMapAndGenerate(path, dsize, func(buffer []uint32) { generateDataset(buffer, d.epoch, cache) })
		if err != nil {
			logger.Error("Failed to generate mapped ethash dataset", "err", err)

			d.dataset = make([]uint32, dsize/2)
			generateDataset(d.dataset, d.epoch, cache)
		}
//迭代所有以前的实例并删除旧实例
		for ep := int(d.epoch) - limit; ep >= 0; ep-- {
			seed := seedHash(uint64(ep)*epochLength + 1)
			path := filepath.Join(dir, fmt.Sprintf("full-R%d-%x%s", algorithmRevision, seed[:8], endian))
			os.Remove(path)
		}
	})
}

//generated返回此特定数据集是否已完成生成
//或者没有（可能根本没有启动）。这对远程矿工很有用
//默认为验证缓存，而不是在DAG代上阻塞。
func (d *dataset) generated() bool {
	return atomic.LoadUint32(&d.done) == 1
}

//终结器关闭所有打开的文件处理程序和内存映射。
func (d *dataset) finalizer() {
	if d.mmap != nil {
		d.mmap.Unmap()
		d.dump.Close()
		d.mmap, d.dump = nil, nil
	}
}

//makecache生成一个新的ethash缓存，并可以选择将其存储到磁盘。
func MakeCache(block uint64, dir string) {
	c := cache{epoch: block / epochLength}
	c.generate(dir, math.MaxInt32, false)
}

//makedataset生成一个新的ethash数据集，并可以选择将其存储到磁盘。
func MakeDataset(block uint64, dir string) {
	d := dataset{epoch: block / epochLength}
	d.generate(dir, math.MaxInt32, false)
}

//模式定义了ethash引擎所做的POW验证的类型和数量。
type Mode uint

const (
	ModeNormal Mode = iota
	ModeShared
	ModeTest
	ModeFake
	ModeFullFake
)

//config是ethash的配置参数。
type Config struct {
	CacheDir       string
	CachesInMem    int
	CachesOnDisk   int
	DatasetDir     string
	DatasetsInMem  int
	DatasetsOnDisk int
	PowMode        Mode
}

//sealttask用远程密封器螺纹的相对结果通道包装密封块。
type sealTask struct {
	block   *types.Block
	results chan<- *types.Block
}

//mineresult包装指定块的POW解决方案参数。
type mineResult struct {
	nonce     types.BlockNonce
	mixDigest common.Hash
	hash      common.Hash

	errc chan error
}

//哈希率包装远程密封程序提交的哈希率。
type hashrate struct {
	id   common.Hash
	ping time.Time
	rate uint64

	done chan struct{}
}

//密封件包裹远程密封件的密封件工作包。
type sealWork struct {
	errc chan error
	res  chan [4]string
}

//ethash是基于实施ethash的工作证明的共识引擎。
//算法。
type Ethash struct {
	config Config

caches   *lru //内存缓存以避免重新生成太频繁
datasets *lru //内存中的数据集，以避免过于频繁地重新生成

//采矿相关领域
rand     *rand.Rand    //当前正确播种的随机源
threads  int           //中频挖掘要挖掘的线程数
update   chan struct{} //更新挖掘参数的通知通道
hashrate metrics.Meter //米跟踪平均哈希率

//远程密封相关字段
workCh       chan *sealTask   //通知通道将新工作和相关结果通道推送到远程封口机
fetchWorkCh  chan *sealWork   //用于远程封口机获取采矿作业的通道
submitWorkCh chan *mineResult //用于远程封口机提交其采矿结果的通道
fetchRateCh  chan chan uint64 //用于收集本地或远程密封程序提交的哈希率的通道。
submitRateCh chan *hashrate   //用于远程密封程序提交其挖掘哈希的通道

//下面的字段是用于测试的挂钩
shared    *Ethash       //共享POW验证程序以避免缓存重新生成
fakeFail  uint64        //即使在假模式下也无法进行电源检查的块号
fakeDelay time.Duration //从验证返回前的睡眠时间延迟

lock      sync.Mutex      //
closeOnce sync.Once       //确保出口通道不会关闭两次。
exitCh    chan chan error //退出后端线程的通知通道
}

//new创建一个完整的ethash pow方案，并为
//远程挖掘，也可以选择将新工作通知一批远程服务
//包装。
func New(config Config, notify []string, noverify bool) *Ethash {
	if config.CachesInMem <= 0 {
		log.Warn("One ethash cache must always be in memory", "requested", config.CachesInMem)
		config.CachesInMem = 1
	}
	if config.CacheDir != "" && config.CachesOnDisk > 0 {
		log.Info("Disk storage enabled for ethash caches", "dir", config.CacheDir, "count", config.CachesOnDisk)
	}
	if config.DatasetDir != "" && config.DatasetsOnDisk > 0 {
		log.Info("Disk storage enabled for ethash DAGs", "dir", config.DatasetDir, "count", config.DatasetsOnDisk)
	}
	ethash := &Ethash{
		config:       config,
		caches:       newlru("cache", config.CachesInMem, newCache),
		datasets:     newlru("dataset", config.DatasetsInMem, newDataset),
		update:       make(chan struct{}),
		hashrate:     metrics.NewMeterForced(),
		workCh:       make(chan *sealTask),
		fetchWorkCh:  make(chan *sealWork),
		submitWorkCh: make(chan *mineResult),
		fetchRateCh:  make(chan chan uint64),
		submitRateCh: make(chan *hashrate),
		exitCh:       make(chan chan error),
	}
	go ethash.remote(notify, noverify)
	return ethash
}

//NewTester创建了一个小型ethash pow方案，该方案仅用于测试
//目的。
func NewTester(notify []string, noverify bool) *Ethash {
	ethash := &Ethash{
		config:       Config{PowMode: ModeTest},
		caches:       newlru("cache", 1, newCache),
		datasets:     newlru("dataset", 1, newDataset),
		update:       make(chan struct{}),
		hashrate:     metrics.NewMeterForced(),
		workCh:       make(chan *sealTask),
		fetchWorkCh:  make(chan *sealWork),
		submitWorkCh: make(chan *mineResult),
		fetchRateCh:  make(chan chan uint64),
		submitRateCh: make(chan *hashrate),
		exitCh:       make(chan chan error),
	}
	go ethash.remote(notify, noverify)
	return ethash
}

//Newfaker创建了一个具有假POW方案的ethash共识引擎，该方案接受
//所有区块的封条都是有效的，尽管它们仍然必须符合以太坊。
//共识规则。
func NewFaker() *Ethash {
	return &Ethash{
		config: Config{
			PowMode: ModeFake,
		},
	}
}

//newfakefailer创建了一个具有假POW方案的ethash共识引擎，
//接受除指定的单个块之外的所有块，尽管它们
//仍然必须遵守以太坊共识规则。
func NewFakeFailer(fail uint64) *Ethash {
	return &Ethash{
		config: Config{
			PowMode: ModeFake,
		},
		fakeFail: fail,
	}
}

//Newfakedelayer创建了一个具有假POW方案的ethash共识引擎，
//接受所有块为有效，但将验证延迟一段时间
//他们仍然必须遵守以太坊共识规则。
func NewFakeDelayer(delay time.Duration) *Ethash {
	return &Ethash{
		config: Config{
			PowMode: ModeFake,
		},
		fakeDelay: delay,
	}
}

//newfullfaker创建了一个具有完全伪造方案的ethash共识引擎，
//接受所有块为有效块，而不检查任何共识规则。
func NewFullFaker() *Ethash {
	return &Ethash{
		config: Config{
			PowMode: ModeFullFake,
		},
	}
}

//newshared创建一个在所有运行的请求者之间共享的完整大小的ethash pow
//在同样的过程中。
func NewShared() *Ethash {
	return &Ethash{shared: sharedEthash}
}

//关闭关闭退出通道以通知所有后端线程退出。
func (ethash *Ethash) Close() error {
	var err error
	ethash.closeOnce.Do(func() {
//如果没有分配出口通道，则短路。
		if ethash.exitCh == nil {
			return
		}
		errc := make(chan error)
		ethash.exitCh <- errc
		err = <-errc
		close(ethash.exitCh)
	})
	return err
}

//缓存尝试检索指定块号的验证缓存
//首先检查内存中缓存的列表，然后检查缓存
//存储在磁盘上，如果找不到，则最终生成一个。
func (ethash *Ethash) cache(block uint64) *cache {
	epoch := block / epochLength
	currentI, futureI := ethash.caches.get(epoch)
	current := currentI.(*cache)

//等待生成完成。
	current.generate(ethash.config.CacheDir, ethash.config.CachesOnDisk, ethash.config.PowMode == ModeTest)

//如果我们需要一个新的未来缓存，现在是重新生成它的好时机。
	if futureI != nil {
		future := futureI.(*cache)
		go future.generate(ethash.config.CacheDir, ethash.config.CachesOnDisk, ethash.config.PowMode == ModeTest)
	}
	return current
}

//数据集尝试检索指定块号的挖掘数据集
//首先检查内存中的数据集列表，然后检查DAG
//存储在磁盘上，如果找不到，则最终生成一个。
//
//如果指定了异步，那么不仅是将来，而且当前的DAG也是
//在后台线程上生成。
func (ethash *Ethash) dataset(block uint64, async bool) *dataset {
//检索请求的ethash数据集
	epoch := block / epochLength
	currentI, futureI := ethash.datasets.get(epoch)
	current := currentI.(*dataset)

//如果指定了异步，则在后台线程中生成所有内容
	if async && !current.generated() {
		go func() {
			current.generate(ethash.config.DatasetDir, ethash.config.DatasetsOnDisk, ethash.config.PowMode == ModeTest)

			if futureI != nil {
				future := futureI.(*dataset)
				future.generate(ethash.config.DatasetDir, ethash.config.DatasetsOnDisk, ethash.config.PowMode == ModeTest)
			}
		}()
	} else {
//请求了阻止生成，或已完成生成
		current.generate(ethash.config.DatasetDir, ethash.config.DatasetsOnDisk, ethash.config.PowMode == ModeTest)

		if futureI != nil {
			future := futureI.(*dataset)
			go future.generate(ethash.config.DatasetDir, ethash.config.DatasetsOnDisk, ethash.config.PowMode == ModeTest)
		}
	}
	return current
}

//线程返回当前启用的挖掘线程数。这不
//一定意味着采矿正在进行！
func (ethash *Ethash) Threads() int {
	ethash.lock.Lock()
	defer ethash.lock.Unlock()

	return ethash.threads
}

//setthreads更新当前启用的挖掘线程数。打电话
//此方法不启动挖掘，只设置线程计数。如果为零
//指定，矿工将使用机器的所有核心。设置线程
//允许计数低于零，将导致矿工闲置，没有任何
//正在完成的工作。
func (ethash *Ethash) SetThreads(threads int) {
	ethash.lock.Lock()
	defer ethash.lock.Unlock()

//如果我们运行的是一个共享的POW，则改为设置线程计数
	if ethash.shared != nil {
		ethash.shared.SetThreads(threads)
		return
	}
//更新螺纹并对任何运行密封进行ping操作，以拉入任何更改。
	ethash.threads = threads
	select {
	case ethash.update <- struct{}{}:
	default:
	}
}

//hashRate实现pow，返回搜索调用的测量速率
//最后一分钟的每秒。
//注意，返回的哈希率包括本地哈希率，但也包括
//所有远程矿工的哈希率。
func (ethash *Ethash) Hashrate() float64 {
//如果在正常/测试模式下运行ethash，则短路。
	if ethash.config.PowMode != ModeNormal && ethash.config.PowMode != ModeTest {
		return ethash.hashrate.Rate1()
	}
	var res = make(chan uint64, 1)

	select {
	case ethash.fetchRateCh <- res:
	case <-ethash.exitCh:
//仅当ethash停止时返回本地哈希率。
		return ethash.hashrate.Rate1()
	}

//收集远程密封程序提交的总哈希率。
	return ethash.hashrate.Rate1() + float64(<-res)
}

//API实现共识引擎，返回面向用户的RPC API。
func (ethash *Ethash) APIs(chain consensus.ChainReader) []rpc.API {
//为了确保向后兼容性，我们公开了ethash RPC API
//Eth和Ethash名称空间。
	return []rpc.API{
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   &API{ethash},
			Public:    true,
		},
		{
			Namespace: "ethash",
			Version:   "1.0",
			Service:   &API{ethash},
			Public:    true,
		},
	}
}

//seedhash是用于生成验证缓存和挖掘的种子
//数据集。
func SeedHash(block uint64) []byte {
	return seedHash(block)
}
