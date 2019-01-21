
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

//包调试接口转到运行时调试工具。
//这个包主要是胶水代码使这些设施可用
//通过cli和rpc子系统。如果你想从Go代码中使用它们，
//改用包运行时。
package debug

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

//处理程序是全局调试处理程序。
var Handler = new(HandlerT)

//handlert实现调试API。
//不要创建此类型的值，请使用
//而是在处理程序变量中。
type HandlerT struct {
	mu        sync.Mutex
	cpuW      io.WriteCloser
	cpuFile   string
	traceW    io.WriteCloser
	traceFile string
}

//冗长设置了原木冗长的天花板。单个包装的冗长程度
//源文件可以使用vmodule来提升。
func (*HandlerT) Verbosity(level int) {
	glogger.Verbosity(log.Lvl(level))
}

//vmodule设置日志冗长模式。有关
//模式语法。
func (*HandlerT) Vmodule(pattern string) error {
	return glogger.Vmodule(pattern)
}

//backtraceat设置日志backtrace位置。有关详细信息，请参阅包日志
//模式语法。
func (*HandlerT) BacktraceAt(location string) error {
	return glogger.BacktraceAt(location)
}

//MEMSTATS返回详细的运行时内存统计信息。
func (*HandlerT) MemStats() *runtime.MemStats {
	s := new(runtime.MemStats)
	runtime.ReadMemStats(s)
	return s
}

//gcstats返回gc统计信息。
func (*HandlerT) GcStats() *debug.GCStats {
	s := new(debug.GCStats)
	debug.ReadGCStats(s)
	return s
}

//cpuprofile打开cpu配置文件达nsec秒并写入
//配置文件数据到文件。
func (h *HandlerT) CpuProfile(file string, nsec uint) error {
	if err := h.StartCPUProfile(file); err != nil {
		return err
	}
	time.Sleep(time.Duration(nsec) * time.Second)
	h.StopCPUProfile()
	return nil
}

//startcpuprofile打开CPU配置文件，写入给定文件。
func (h *HandlerT) StartCPUProfile(file string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cpuW != nil {
		return errors.New("CPU profiling already in progress")
	}
	f, err := os.Create(expandHome(file))
	if err != nil {
		return err
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return err
	}
	h.cpuW = f
	h.cpuFile = file
	log.Info("CPU profiling started", "dump", h.cpuFile)
	return nil
}

//stopcupprofile停止正在进行的CPU配置文件。
func (h *HandlerT) StopCPUProfile() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	pprof.StopCPUProfile()
	if h.cpuW == nil {
		return errors.New("CPU profiling not in progress")
	}
	log.Info("Done writing CPU profile", "dump", h.cpuFile)
	h.cpuW.Close()
	h.cpuW = nil
	h.cpuFile = ""
	return nil
}

//gotrace打开对nsec秒的跟踪并写入
//将数据跟踪到文件。
func (h *HandlerT) GoTrace(file string, nsec uint) error {
	if err := h.StartGoTrace(file); err != nil {
		return err
	}
	time.Sleep(time.Duration(nsec) * time.Second)
	h.StopGoTrace()
	return nil
}

//BoeStand将GOOTONE配置文件转换为NSEC秒，并将配置文件数据写入
//文件。它使用1的配置率来获取最准确的信息。如果不同的利率是
//desired, set the rate and write the profile manually.
func (*HandlerT) BlockProfile(file string, nsec uint) error {
	runtime.SetBlockProfileRate(1)
	time.Sleep(time.Duration(nsec) * time.Second)
	defer runtime.SetBlockProfileRate(0)
	return writeProfile("block", file)
}

//setBlockProfileRate设置goroutine块配置文件数据收集的速率。
//速率0禁用块分析。
func (*HandlerT) SetBlockProfileRate(rate int) {
	runtime.SetBlockProfileRate(rate)
}

//WriteBlockProfile将goroutine阻塞配置文件写入给定文件。
func (*HandlerT) WriteBlockProfile(file string) error {
	return writeProfile("block", file)
}

//mutex profile打开mutex配置文件达nsec秒，并将配置文件数据写入文件。
//它使用1的配置率来获取最准确的信息。如果不同的利率是
//需要时，设置速率并手动写入配置文件。
func (*HandlerT) MutexProfile(file string, nsec uint) error {
	runtime.SetMutexProfileFraction(1)
	time.Sleep(time.Duration(nsec) * time.Second)
	defer runtime.SetMutexProfileFraction(0)
	return writeProfile("mutex", file)
}

//setmutexprofilefraction设置mutex分析的速率。
func (*HandlerT) SetMutexProfileFraction(rate int) {
	runtime.SetMutexProfileFraction(rate)
}

//WriteMutexProfile writes a goroutine blocking profile to the given file.
func (*HandlerT) WriteMutexProfile(file string) error {
	return writeProfile("mutex", file)
}

//WriteMemProfile将分配配置文件写入给定文件。
//请注意，无法通过API设置分析速率，
//必须在命令行上设置。
func (*HandlerT) WriteMemProfile(file string) error {
	return writeProfile("heap", file)
}

//Stacks返回所有goroutine堆栈的打印表示。
func (*HandlerT) Stacks() string {
	buf := new(bytes.Buffer)
	pprof.Lookup("goroutine").WriteTo(buf, 2)
	return buf.String()
}

//FreeOSMemory returns unused memory to the OS.
func (*HandlerT) FreeOSMemory() {
	debug.FreeOSMemory()
}

//setgcPercent设置垃圾收集目标百分比。它返回上一个
//设置。负值将禁用gc。
func (*HandlerT) SetGCPercent(v int) int {
	return debug.SetGCPercent(v)
}

func writeProfile(name, file string) error {
	p := pprof.Lookup(name)
	log.Info("Writing profile records", "count", p.Count(), "type", name, "dump", file)
	f, err := os.Create(expandHome(file))
	if err != nil {
		return err
	}
	defer f.Close()
	return p.WriteTo(f, 0)
}

//expands home directory in file paths.
//~someuser/tmp将不会扩展。
func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		home := os.Getenv("HOME")
		if home == "" {
			if usr, err := user.Current(); err == nil {
				home = usr.HomeDir
			}
		}
		if home != "" {
			p = home + p[1:]
		}
	}
	return filepath.Clean(p)
}
