
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

package dashboard

//执行：生成纱线--cwd./资产安装
//go：生成纱线——cwd./资产构建
//go:生成go bindata-nometadata-o assets.go-prefix assets-nocompress-pkg dashboard assets/index.html assets/bundle.js
//go:generate sh-c“sed's var bundlejs//nolint:misspell\\n&assets.go>assets.go.tmp&&mv assets.go.tmp assets.go”
//go:generate sh-c“sed's var indexhtml//nolint:misspell\\n&assets.go>assets.go.tmp&&mv assets.go.tmp assets.go”
//go：生成gofmt-w-s资产。

import (
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"io"

	"github.com/elastic/gosigar"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/mohae/deepcopy"
	"golang.org/x/net/websocket"
)

const (
activeMemorySampleLimit   = 200 //活动内存数据样本的最大数目
virtualMemorySampleLimit  = 200 //虚拟内存数据示例的最大数目
networkIngressSampleLimit = 200 //最大网络入口数据样本数
networkEgressSampleLimit  = 200 //网络出口数据样本的最大数目
processCPUSampleLimit     = 200 //最大进程CPU数据样本数
systemCPUSampleLimit      = 200 //系统CPU数据样本的最大数目
diskReadSampleLimit       = 200 //磁盘读取数据样本的最大数目
diskWriteSampleLimit      = 200 //磁盘写入数据示例的最大数目
)

var nextID uint32 //下一个连接ID

//仪表板包含仪表板内部。
type Dashboard struct {
	config *Config

	listener net.Listener
conns    map[uint32]*client //当前活动的WebSocket连接
	history  *Message
lock     sync.RWMutex //锁定保护仪表板内部

	logdir string

quit chan chan error //用于优美出口的通道
	wg   sync.WaitGroup
}

//客户端表示与远程浏览器的活动WebSocket连接。
type client struct {
conn   *websocket.Conn //特定的Live WebSocket连接
msg    chan *Message   //更新消息的消息队列
logger log.Logger      //Logger for the particular live websocket connection
}

//新建创建具有给定配置的新仪表板实例。
func New(config *Config, commit string, logdir string) *Dashboard {
	now := time.Now()
	versionMeta := ""
	if len(params.VersionMeta) > 0 {
		versionMeta = fmt.Sprintf(" (%s)", params.VersionMeta)
	}
	return &Dashboard{
		conns:  make(map[uint32]*client),
		config: config,
		quit:   make(chan chan error),
		history: &Message{
			General: &GeneralMessage{
				Commit:  commit,
				Version: fmt.Sprintf("v%d.%d.%d%s", params.VersionMajor, params.VersionMinor, params.VersionPatch, versionMeta),
			},
			System: &SystemMessage{
				ActiveMemory:   emptyChartEntries(now, activeMemorySampleLimit, config.Refresh),
				VirtualMemory:  emptyChartEntries(now, virtualMemorySampleLimit, config.Refresh),
				NetworkIngress: emptyChartEntries(now, networkIngressSampleLimit, config.Refresh),
				NetworkEgress:  emptyChartEntries(now, networkEgressSampleLimit, config.Refresh),
				ProcessCPU:     emptyChartEntries(now, processCPUSampleLimit, config.Refresh),
				SystemCPU:      emptyChartEntries(now, systemCPUSampleLimit, config.Refresh),
				DiskRead:       emptyChartEntries(now, diskReadSampleLimit, config.Refresh),
				DiskWrite:      emptyChartEntries(now, diskWriteSampleLimit, config.Refresh),
			},
		},
		logdir: logdir,
	}
}

//EmptyChartEntries返回一个包含空样本数量限制的ChartEntry数组。
func emptyChartEntries(t time.Time, limit int, refresh time.Duration) ChartEntries {
	ce := make(ChartEntries, limit)
	for i := 0; i < limit; i++ {
		ce[i] = &ChartEntry{
			Time: t.Add(-time.Duration(i) * refresh),
		}
	}
	return ce
}

//协议实现了node.service接口。
func (db *Dashboard) Protocols() []p2p.Protocol { return nil }

//API实现了node.service接口。
func (db *Dashboard) APIs() []rpc.API { return nil }

//Start启动数据收集线程和仪表板的侦听服务器。
//实现node.service接口。
func (db *Dashboard) Start(server *p2p.Server) error {
	log.Info("Starting dashboard")

	db.wg.Add(2)
	go db.collectData()
	go db.streamLogs()

	http.HandleFunc("/", db.webHandler)
	http.Handle("/api", websocket.Handler(db.apiHandler))

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", db.config.Host, db.config.Port))
	if err != nil {
		return err
	}
	db.listener = listener

	go http.Serve(listener, nil)

	return nil
}

//stop停止数据收集线程和仪表板的连接侦听器。
//实现node.service接口。
func (db *Dashboard) Stop() error {
//关闭连接侦听器。
	var errs []error
	if err := db.listener.Close(); err != nil {
		errs = append(errs, err)
	}
//关闭收集器。
	errc := make(chan error, 1)
	for i := 0; i < 2; i++ {
		db.quit <- errc
		if err := <-errc; err != nil {
			errs = append(errs, err)
		}
	}
//关闭连接。
	db.lock.Lock()
	for _, c := range db.conns {
		if err := c.conn.Close(); err != nil {
			c.logger.Warn("Failed to close connection", "err", err)
		}
	}
	db.lock.Unlock()

//等到每一条血路都结束。
	db.wg.Wait()
	log.Info("Dashboard stopped")

	var err error
	if len(errs) > 0 {
		err = fmt.Errorf("%v", errs)
	}

	return err
}

//WebHandler处理所有非API请求，只需扁平化并返回仪表板网站。
func (db *Dashboard) webHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("Request", "URL", r.URL)

	path := r.URL.String()
	if path == "/" {
		path = "/index.html"
	}
	blob, err := Asset(path[1:])
	if err != nil {
		log.Warn("Failed to load the asset", "path", path, "err", err)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Write(blob)
}

//apiHandler处理仪表板的请求。
func (db *Dashboard) apiHandler(conn *websocket.Conn) {
	id := atomic.AddUint32(&nextID, 1)
	client := &client{
		conn:   conn,
		msg:    make(chan *Message, 128),
		logger: log.New("id", id),
	}
	done := make(chan struct{})

//开始侦听要发送的消息。
	db.wg.Add(1)
	go func() {
		defer db.wg.Done()

		for {
			select {
			case <-done:
				return
			case msg := <-client.msg:
				if err := websocket.JSON.Send(client.conn, msg); err != nil {
					client.logger.Warn("Failed to send the message", "msg", msg, "err", err)
					client.conn.Close()
					return
				}
			}
		}
	}()

	db.lock.Lock()
//发送过去的数据。
	client.msg <- deepcopy.Copy(db.history).(*Message)
//开始跟踪连接并在连接丢失时丢弃。
	db.conns[id] = client
	db.lock.Unlock()
	defer func() {
		db.lock.Lock()
		delete(db.conns, id)
		db.lock.Unlock()
	}()
	for {
		r := new(Request)
		if err := websocket.JSON.Receive(conn, r); err != nil {
			if err != io.EOF {
				client.logger.Warn("Failed to receive request", "err", err)
			}
			close(done)
			return
		}
		if r.Logs != nil {
			db.handleLogRequest(r.Logs, client)
		}
	}
}

//MeterCollector返回一个函数，该函数检索特定的仪表。
func meterCollector(name string) func() int64 {
	if metric := metrics.DefaultRegistry.Get(name); metric != nil {
		m := metric.(metrics.Meter)
		return func() int64 {
			return m.Count()
		}
	}
	return func() int64 {
		return 0
	}
}

//CollectData收集仪表板上绘图所需的数据。
func (db *Dashboard) collectData() {
	defer db.wg.Done()

	systemCPUUsage := gosigar.Cpu{}
	systemCPUUsage.Get()
	var (
		mem runtime.MemStats

		collectNetworkIngress = meterCollector("p2p/InboundTraffic")
		collectNetworkEgress  = meterCollector("p2p/OutboundTraffic")
		collectDiskRead       = meterCollector("eth/db/chaindata/disk/read")
		collectDiskWrite      = meterCollector("eth/db/chaindata/disk/write")

		prevNetworkIngress = collectNetworkIngress()
		prevNetworkEgress  = collectNetworkEgress()
		prevProcessCPUTime = getProcessCPUTime()
		prevSystemCPUUsage = systemCPUUsage
		prevDiskRead       = collectDiskRead()
		prevDiskWrite      = collectDiskWrite()

		frequency = float64(db.config.Refresh / time.Second)
		numCPU    = float64(runtime.NumCPU())
	)

	for {
		select {
		case errc := <-db.quit:
			errc <- nil
			return
		case <-time.After(db.config.Refresh):
			systemCPUUsage.Get()
			var (
				curNetworkIngress = collectNetworkIngress()
				curNetworkEgress  = collectNetworkEgress()
				curProcessCPUTime = getProcessCPUTime()
				curSystemCPUUsage = systemCPUUsage
				curDiskRead       = collectDiskRead()
				curDiskWrite      = collectDiskWrite()

				deltaNetworkIngress = float64(curNetworkIngress - prevNetworkIngress)
				deltaNetworkEgress  = float64(curNetworkEgress - prevNetworkEgress)
				deltaProcessCPUTime = curProcessCPUTime - prevProcessCPUTime
				deltaSystemCPUUsage = curSystemCPUUsage.Delta(prevSystemCPUUsage)
				deltaDiskRead       = curDiskRead - prevDiskRead
				deltaDiskWrite      = curDiskWrite - prevDiskWrite
			)
			prevNetworkIngress = curNetworkIngress
			prevNetworkEgress = curNetworkEgress
			prevProcessCPUTime = curProcessCPUTime
			prevSystemCPUUsage = curSystemCPUUsage
			prevDiskRead = curDiskRead
			prevDiskWrite = curDiskWrite

			now := time.Now()

			runtime.ReadMemStats(&mem)
			activeMemory := &ChartEntry{
				Time:  now,
				Value: float64(mem.Alloc) / frequency,
			}
			virtualMemory := &ChartEntry{
				Time:  now,
				Value: float64(mem.Sys) / frequency,
			}
			networkIngress := &ChartEntry{
				Time:  now,
				Value: deltaNetworkIngress / frequency,
			}
			networkEgress := &ChartEntry{
				Time:  now,
				Value: deltaNetworkEgress / frequency,
			}
			processCPU := &ChartEntry{
				Time:  now,
				Value: deltaProcessCPUTime / frequency / numCPU * 100,
			}
			systemCPU := &ChartEntry{
				Time:  now,
				Value: float64(deltaSystemCPUUsage.Sys+deltaSystemCPUUsage.User) / frequency / numCPU,
			}
			diskRead := &ChartEntry{
				Time:  now,
				Value: float64(deltaDiskRead) / frequency,
			}
			diskWrite := &ChartEntry{
				Time:  now,
				Value: float64(deltaDiskWrite) / frequency,
			}
			sys := db.history.System
			db.lock.Lock()
			sys.ActiveMemory = append(sys.ActiveMemory[1:], activeMemory)
			sys.VirtualMemory = append(sys.VirtualMemory[1:], virtualMemory)
			sys.NetworkIngress = append(sys.NetworkIngress[1:], networkIngress)
			sys.NetworkEgress = append(sys.NetworkEgress[1:], networkEgress)
			sys.ProcessCPU = append(sys.ProcessCPU[1:], processCPU)
			sys.SystemCPU = append(sys.SystemCPU[1:], systemCPU)
			sys.DiskRead = append(sys.DiskRead[1:], diskRead)
			sys.DiskWrite = append(sys.DiskWrite[1:], diskWrite)
			db.lock.Unlock()

			db.sendToAll(&Message{
				System: &SystemMessage{
					ActiveMemory:   ChartEntries{activeMemory},
					VirtualMemory:  ChartEntries{virtualMemory},
					NetworkIngress: ChartEntries{networkIngress},
					NetworkEgress:  ChartEntries{networkEgress},
					ProcessCPU:     ChartEntries{processCPU},
					SystemCPU:      ChartEntries{systemCPU},
					DiskRead:       ChartEntries{diskRead},
					DiskWrite:      ChartEntries{diskWrite},
				},
			})
		}
	}
}

//sendToAll将给定消息发送到活动仪表板。
func (db *Dashboard) sendToAll(msg *Message) {
	db.lock.Lock()
	for _, c := range db.conns {
		select {
		case c.msg <- msg:
		default:
			c.conn.Close()
		}
	}
	db.lock.Unlock()
}
