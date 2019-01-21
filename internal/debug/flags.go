
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

package debug

import (
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/metrics/exp"
	"github.com/fjl/memsize/memsizeui"
	colorable "github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"gopkg.in/urfave/cli.v1"
)

var Memsize memsizeui.Handler

var (
	verbosityFlag = cli.IntFlag{
		Name:  "verbosity",
		Usage: "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail",
		Value: 3,
	}
	vmoduleFlag = cli.StringFlag{
		Name:  "vmodule",
  /*ge: "Per-module verbosity: comma-separated list of <pattern>=<level> (e.g. eth/*=5,p2p=4)",
  值：“
 }
 backtraceAtFlag=cli.stringFlag_
  名称：“回溯”，
  用法：“在特定的日志记录语句（例如\”block.go:271\“）请求堆栈跟踪”，
  值：“
 }
 debugflag=cli.boolflag_
  名称：“调试”，
  Usage: "Prepends log messages with call-site location (file and line number)",
 }
 pprofflag=cli.boolflag_
  姓名：“PPROF”，
  用法：“启用pprof http服务器”，
 }
 pprofPortFlag=cli.intFlag_
  姓名：“PrPopPt”，
  用法：“pprof http服务器侦听端口”，
  值：6060，
 }
 pprofaddflag=cli.stringflag_
  名称：“pprofaddr”，
  Usage: "pprof HTTP server listening interface",
  值：“127.0.0.1”，
 }
 memprofilerateflag=cli.intflag_
  名称：“MemProfileRate”，
  用法：“以给定速率打开内存分析”，
  值：runtime.memprofilerate，
 }
 blockProfileRateFlag=cli.intFlag_
  Name:  "blockprofilerate",
  用法：“以给定速率打开块分析”，
 }
 cpuprofileFlag = cli.StringFlag{
  名称：“cpuprofile”，
  用法：“将CPU配置文件写入给定文件”，
 }
 traceFlag=cli.stringFlag_
  姓名：“追踪”，
  用法：“将执行跟踪写入给定文件”，
 }
）

//标志保存调试所需的所有命令行标志。
var flags=[]cli.flag_
 详细标志，vmoduleflag，backtraceatflag，debugflag，
 pprofFlag、pprofAddFlag、pprofPortFlag
 memprofilerateflag、blockprofilerateflag、cpupprofileflag、traceflag、
}

var
 Ostream日志处理程序
 glogger*日志.gloghandler
）

函数（）
 usecolor := (isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd())) && os.Getenv("TERM") !=“哑巴”
 输出：=io.writer（os.stderr）
 如果使用颜色{
  输出=可着色。新的可着色stderr（）
 }
 ostream = log.StreamHandler(output, log.TerminalFormat(usecolor))
 glogger=log.newgloghandler（奥斯特里姆）
}

//St设置基于CLI标志初始化配置文件和日志记录。
//应该在程序中尽早调用它。
func设置（ctx*cli.context，logdir string）错误
 /测井
 log printorigins（ctx.globalbool（debugflag.name））。
 如果Logdir！=“{”
  rfh，err：=log.rotatingfilehandler（
   洛迪尔
   262144，
   log.JSONFormatOrderedEx(false, true),
  ）
  如果犯错！= nIL{
   返回错误
  }
  glogger.sethandler（log.multihandler（ostream，rfh））。
 }
 glogger.verbosity（log.lvl（ctx.globalint（verbosityflag.name）））
 glogger.vmodule（ctx.globalString（vmoduleFlag.name））。
 glogger.backtraceat（ctx.globalstring（backtraceatflag.name））。
 log.root（）.sethandler（glogger）

 //分析，跟踪
 runtime.memprofilerate=ctx.globalint（memprofilerateflag.name）
 Handler.SetBlockProfileRate(ctx.GlobalInt(blockprofilerateFlag.Name))
 如果tracefile：=ctx.globalstring（traceflag.name）；tracefile！=“{”
  如果错误：=handler.startgotrace（tracefile）；错误！= nIL{
   返回错误
  }
 }
 如果cpufile：=ctx.globalString（cpuProfileFlag.name）；cpufile！=“{”
  if err := Handler.StartCPUProfile(cpuFile); err != nIL{
   返回错误
  }
 }

 //PPROF服务器
 如果ctx.globalbool（pprofflag.name）
  地址：=fmt.sprintf（“%s:%d”，ctx.globalString（pprofaddflag.name），ctx.globalint（pprofportflag.name））
  startpprof（地址）
 }
 返回零
}

func StartPProf（地址字符串）{
 //在任何/debug/metrics请求中将go metrics挂接到expvar中，加载所有var
 //从注册表到ExpVar，并执行常规ExpVaR处理程序。
 exp.exp（metrics.defaultregistry）
 HTTP.句柄（“/MeistSe/”，http.StripPrefix（“/MESIZE”和MESIZE））
 log.info（“启动pprof服务器”，“addr”，fmt.sprintf（“http://%s/debug/pprof”，address））。
 转到函数（）
  如果错误：=http.listenandserve（address，nil）；错误！= nIL{
   log.error（“运行pprof server失败”，“err”，err）
  }
 }（）
}

//exit停止所有正在运行的配置文件，将其输出刷新到
//各自的文件。
FUNC退出（）{
 handler.stopcupprofile（）处理程序
 handler.stopgotrace（）。
}
