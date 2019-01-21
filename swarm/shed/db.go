
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

//包棚提供了一个简单的抽象组件来组成
//对按字段和索引组织的存储数据进行更复杂的操作。
//
//唯一保存有关群存储块数据的逻辑信息的类型
//元数据是项。这一部分主要不是为了
//性能原因。
package shed

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
openFileLimit              = 128 //级别ldb openfilescachecapacity的限制。
	writePauseWarningThrottler = 1 * time.Minute
)

//DB在级别数据库上提供抽象，以便
//使用字段和有序索引实现复杂结构。
//它提供了一个模式功能来存储字段和索引
//有关命名和类型的信息。
type DB struct {
	ldb *leveldb.DB

compTimeMeter    metrics.Meter //用于测量数据库压缩所花费的总时间的仪表
compReadMeter    metrics.Meter //测量压实过程中读取数据的仪表
compWriteMeter   metrics.Meter //测量压实过程中写入数据的仪表
writeDelayNMeter metrics.Meter //用于测量数据库压缩导致的写入延迟数的仪表
writeDelayMeter  metrics.Meter //用于测量数据库压缩导致的写入延迟持续时间的仪表
diskReadMeter    metrics.Meter //测量有效数据读取量的仪表
diskWriteMeter   metrics.Meter //测量写入数据有效量的仪表

quitChan chan chan error //退出通道以在关闭数据库之前停止度量集合
}

//new db构造新的db并验证架构
//如果它存在于给定路径上的数据库中。
//metricsPrefix用于为给定数据库收集度量。
func NewDB(path string, metricsPrefix string) (db *DB, err error) {
	ldb, err := leveldb.OpenFile(path, &opt.Options{
		OpenFilesCacheCapacity: openFileLimit,
	})
	if err != nil {
		return nil, err
	}
	db = &DB{
		ldb: ldb,
	}

	if _, err = db.getSchema(); err != nil {
		if err == leveldb.ErrNotFound {
//用已初始化的默认字段保存架构
			if err = db.putSchema(schema{
				Fields:  make(map[string]fieldSpec),
				Indexes: make(map[byte]indexSpec),
			}); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

//为DB配置仪表
	db.configure(metricsPrefix)

//为定期度量收集器创建退出通道并运行它
	db.quitChan = make(chan chan error)

	go db.meter(10 * time.Second)

	return db, nil
}

//将Wrapps-LevelDB-Put方法添加到增量度量计数器。
func (db *DB) Put(key []byte, value []byte) (err error) {
	err = db.ldb.Put(key, value, nil)
	if err != nil {
		metrics.GetOrRegisterCounter("DB.putFail", nil).Inc(1)
		return err
	}
	metrics.GetOrRegisterCounter("DB.put", nil).Inc(1)
	return nil
}

//get wrapps leveldb get方法以增加度量计数器。
func (db *DB) Get(key []byte) (value []byte, err error) {
	value, err = db.ldb.Get(key, nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			metrics.GetOrRegisterCounter("DB.getNotFound", nil).Inc(1)
		} else {
			metrics.GetOrRegisterCounter("DB.getFail", nil).Inc(1)
		}
		return nil, err
	}
	metrics.GetOrRegisterCounter("DB.get", nil).Inc(1)
	return value, nil
}

//DELETE包装级别DB DELETE方法以增加度量计数器。
func (db *DB) Delete(key []byte) (err error) {
	err = db.ldb.Delete(key, nil)
	if err != nil {
		metrics.GetOrRegisterCounter("DB.deleteFail", nil).Inc(1)
		return err
	}
	metrics.GetOrRegisterCounter("DB.delete", nil).Inc(1)
	return nil
}

//NewIterator将LevelDB NewIterator方法包装为增量度量计数器。
func (db *DB) NewIterator() iterator.Iterator {
	metrics.GetOrRegisterCounter("DB.newiterator", nil).Inc(1)

	return db.ldb.NewIterator(nil, nil)
}

//WRITEBATCH将LEVELDB WRITE方法包装为增量度量计数器。
func (db *DB) WriteBatch(batch *leveldb.Batch) (err error) {
	err = db.ldb.Write(batch, nil)
	if err != nil {
		metrics.GetOrRegisterCounter("DB.writebatchFail", nil).Inc(1)
		return err
	}
	metrics.GetOrRegisterCounter("DB.writebatch", nil).Inc(1)
	return nil
}

//关闭关闭级别数据库。
func (db *DB) Close() (err error) {
	close(db.quitChan)
	return db.ldb.Close()
}

//配置配置数据库度量收集器
func (db *DB) configure(prefix string) {
//在请求的前缀处初始化所有度量收集器
	db.compTimeMeter = metrics.NewRegisteredMeter(prefix+"compact/time", nil)
	db.compReadMeter = metrics.NewRegisteredMeter(prefix+"compact/input", nil)
	db.compWriteMeter = metrics.NewRegisteredMeter(prefix+"compact/output", nil)
	db.diskReadMeter = metrics.NewRegisteredMeter(prefix+"disk/read", nil)
	db.diskWriteMeter = metrics.NewRegisteredMeter(prefix+"disk/write", nil)
	db.writeDelayMeter = metrics.NewRegisteredMeter(prefix+"compact/writedelay/duration", nil)
	db.writeDelayNMeter = metrics.NewRegisteredMeter(prefix+"compact/writedelay/counter", nil)
}

func (db *DB) meter(refresh time.Duration) {
//创建计数器以存储当前和以前的压缩值
	compactions := make([][]float64, 2)
	for i := 0; i < 2; i++ {
		compactions[i] = make([]float64, 3)
	}
//为iostats创建存储。
	var iostats [2]float64

//为写入延迟创建存储和警告日志跟踪程序。
	var (
		delaystats      [2]int64
		lastWritePaused time.Time
	)

	var (
		errc chan error
		merr error
	)

//无限迭代并收集统计信息
	for i := 1; errc == nil && merr == nil; i++ {
//检索数据库状态
		stats, err := db.ldb.GetProperty("leveldb.stats")
		if err != nil {
			log.Error("Failed to read database stats", "err", err)
			merr = err
			continue
		}
//找到压缩表，跳过标题
		lines := strings.Split(stats, "\n")
		for len(lines) > 0 && strings.TrimSpace(lines[0]) != "Compactions" {
			lines = lines[1:]
		}
		if len(lines) <= 3 {
			log.Error("Compaction table not found")
			merr = errors.New("compaction table not found")
			continue
		}
		lines = lines[3:]

//遍历所有表行，并累积条目
		for j := 0; j < len(compactions[i%2]); j++ {
			compactions[i%2][j] = 0
		}
		for _, line := range lines {
			parts := strings.Split(line, "|")
			if len(parts) != 6 {
				break
			}
			for idx, counter := range parts[3:] {
				value, err := strconv.ParseFloat(strings.TrimSpace(counter), 64)
				if err != nil {
					log.Error("Compaction entry parsing failed", "err", err)
					merr = err
					continue
				}
				compactions[i%2][idx] += value
			}
		}
//更新所有要求的仪表
		if db.compTimeMeter != nil {
			db.compTimeMeter.Mark(int64((compactions[i%2][0] - compactions[(i-1)%2][0]) * 1000 * 1000 * 1000))
		}
		if db.compReadMeter != nil {
			db.compReadMeter.Mark(int64((compactions[i%2][1] - compactions[(i-1)%2][1]) * 1024 * 1024))
		}
		if db.compWriteMeter != nil {
			db.compWriteMeter.Mark(int64((compactions[i%2][2] - compactions[(i-1)%2][2]) * 1024 * 1024))
		}

//检索写入延迟统计信息
		writedelay, err := db.ldb.GetProperty("leveldb.writedelay")
		if err != nil {
			log.Error("Failed to read database write delay statistic", "err", err)
			merr = err
			continue
		}
		var (
			delayN        int64
			delayDuration string
			duration      time.Duration
			paused        bool
		)
		if n, err := fmt.Sscanf(writedelay, "DelayN:%d Delay:%s Paused:%t", &delayN, &delayDuration, &paused); n != 3 || err != nil {
			log.Error("Write delay statistic not found")
			merr = err
			continue
		}
		duration, err = time.ParseDuration(delayDuration)
		if err != nil {
			log.Error("Failed to parse delay duration", "err", err)
			merr = err
			continue
		}
		if db.writeDelayNMeter != nil {
			db.writeDelayNMeter.Mark(delayN - delaystats[0])
		}
		if db.writeDelayMeter != nil {
			db.writeDelayMeter.Mark(duration.Nanoseconds() - delaystats[1])
		}
//如果显示了数据库正在执行压缩的警告，则
//警告将被保留一分钟，以防压倒用户。
		if paused && delayN-delaystats[0] == 0 && duration.Nanoseconds()-delaystats[1] == 0 &&
			time.Now().After(lastWritePaused.Add(writePauseWarningThrottler)) {
			log.Warn("Database compacting, degraded performance")
			lastWritePaused = time.Now()
		}
		delaystats[0], delaystats[1] = delayN, duration.Nanoseconds()

//检索数据库iostats。
		ioStats, err := db.ldb.GetProperty("leveldb.iostats")
		if err != nil {
			log.Error("Failed to read database iostats", "err", err)
			merr = err
			continue
		}
		var nRead, nWrite float64
		parts := strings.Split(ioStats, " ")
		if len(parts) < 2 {
			log.Error("Bad syntax of ioStats", "ioStats", ioStats)
			merr = fmt.Errorf("bad syntax of ioStats %s", ioStats)
			continue
		}
		if n, err := fmt.Sscanf(parts[0], "Read(MB):%f", &nRead); n != 1 || err != nil {
			log.Error("Bad syntax of read entry", "entry", parts[0])
			merr = err
			continue
		}
		if n, err := fmt.Sscanf(parts[1], "Write(MB):%f", &nWrite); n != 1 || err != nil {
			log.Error("Bad syntax of write entry", "entry", parts[1])
			merr = err
			continue
		}
		if db.diskReadMeter != nil {
			db.diskReadMeter.Mark(int64((nRead - iostats[0]) * 1024 * 1024))
		}
		if db.diskWriteMeter != nil {
			db.diskWriteMeter.Mark(int64((nWrite - iostats[1]) * 1024 * 1024))
		}
		iostats[0], iostats[1] = nRead, nWrite

//睡一会儿，然后重复统计数据收集
		select {
		case errc = <-db.quitChan:
//停止请求，停止锤击数据库
		case <-time.After(refresh):
//超时，收集一组新的统计信息
		}
	}

	if errc == nil {
		errc = <-db.quitChan
	}
	errc <- merr
}
