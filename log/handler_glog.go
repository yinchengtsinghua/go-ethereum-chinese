
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

package log

import (
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

//当用户vmodule模式无效时返回errvmodulesyntax。
var errVmoduleSyntax = errors.New("expect comma-separated list of filename=N")

//当用户回溯模式无效时，返回errTraceSyntax。
var errTraceSyntax = errors.New("expect file.go:234")

//Gloghandler是一个日志处理程序，模拟谷歌的过滤功能。
//glog logger：设置全局日志级别；用callsite模式覆盖
//匹配；并在特定位置请求回溯。
type GlogHandler struct {
origin Handler //此包装的原始处理程序

level     uint32 //当前日志级别，可原子访问
override  uint32 //标记是否使用原子可访问的重写
backtrace uint32 //标记是否设置回溯位置

patterns  []pattern       //要重写的模式的当前列表
siteCache map[uintptr]Lvl //调用站点模式计算的缓存
location  string          //文件：进行堆栈转储的行位置
lock      sync.RWMutex    //锁定保护覆盖模式列表
}

//newgloghandler创建了一个新的日志处理程序，其过滤功能与
//谷歌的Glog日志。返回的处理程序实现处理程序。
func NewGlogHandler(h Handler) *GlogHandler {
	return &GlogHandler{
		origin: h,
	}
}

//sethandler更新处理程序以将记录写入指定的子处理程序。
func (h *GlogHandler) SetHandler(nh Handler) {
	h.origin = nh
}

//模式包含vmodule选项的筛选器，保持详细级别
//和要匹配的文件模式。
type pattern struct {
	pattern *regexp.Regexp
	level   Lvl
}

//冗长设置了发光的冗长天花板。单个包装的冗长程度
//源文件可以使用vmodule来提升。
func (h *GlogHandler) Verbosity(level Lvl) {
	atomic.StoreUint32(&h.level, uint32(level))
}

//vmodule设置glog冗长模式。
//
//参数的语法是以逗号分隔的pattern=n列表，其中
//模式是文本文件名或“glob”模式匹配，n是v级别。
//
//例如：
//
//pattern=“gopher.go=3”
//在所有名为“gopher.go”的go文件中将v级别设置为3
//
//模式=“FoO＝3”
//将导入路径以“foo”结尾的任何包的所有文件中的v设置为3
//
/*pattern=“foo/*=3”
//在导入路径包含“foo”的任何包的所有文件中，将v设置为3
func（h*gloghandler）vmodule（ruleset string）错误
 var过滤器[]模式
 对于u，规则：=range strings.split（ruleset，“，”）
  //可以忽略尾随逗号等空字符串
  如果len（rule）==0
   持续
  }
  //确保我们有模式=级别筛选规则
  部分：=strings.split（rule，“=”）
  如果莱恩（零件）！= 2 {
   返回errvmodulesyntax
  }
  零件[0]=字符串.trimspace（零件[0]）
  部件[1]=字符串。Trimspace（部件[1]）
  如果len（零件[0]）=0 len（零件[1]）=0
   返回errvmodulesyntax
  }
  //分析级别，如果正确，则组装筛选规则
  级别，错误：=strconv.atoi（部件[1]）
  如果犯错！= nIL{
   返回errvmodulesyntax
  }
  如果水平<＝0
   继续//忽略。这是无害的，但没有必要支付管理费用。
  }
  //将规则模式编译为正则表达式
  匹配器=：“*”
  对于u，comp：=range strings.split（parts[0]，“/”）
   如果comp=“*”
    匹配器+=“（/.*）？”
   其他，如果有！=“{”
    matcher+=“/”+regexp.quoteteta（comp）
   }
  }
  如果！字符串.hassuffix（部件[0]，“.go”）；
   Matcher+=“/[^/]+\\.go”
  }
  matcher=matcher+“$”

  re，：=regexp.compile（matcher）
  filter=append（filter，pattern re，lvl（level））
 }
 //换掉新过滤系统的vmodule模式
 H.Lo.C.（）
 延迟h.lock.unlock（）

 h.patterns=过滤器
 h.sitecache=make（映射[uintptr]lvl）
 atomic.storeuint32（&h.override，uint32（len（filter）））

 返回零
}

//backtraceat设置glog backtrace位置。当设置为文件和行时
//保存日志语句的数字，堆栈跟踪将写入信息
//每当执行命中该语句时记录。
/ /
//与vmodule不同，“.go”必须存在。
func（h*gloghandler）backtraceat（location string）错误
 //确保回溯位置包含两个非空元素
 部件：=strings.split（位置，“：”）
 如果莱恩（零件）！= 2 {
  返回errtraceSyntax
 }
 零件[0]=字符串.trimspace（零件[0]）
 部件[1]=字符串。Trimspace（部件[1]）
 如果len（零件[0]）=0 len（零件[1]）=0
  返回errtraceSyntax
 }
 //确保.go前缀存在且该行有效
 如果！字符串.hassuffix（部件[0]，“.go”）；
  返回errtraceSyntax
 }
 如果uuErr：=strconv.atoi（第[1]部分）；Err！= nIL{
  返回errtraceSyntax
 }
 //一切似乎都有效
 H.Lo.C.（）
 延迟h.lock.unlock（）

 H.位置=位置
 atomic.storeuint32（&h.backtrace，uint32（len（location）））

 返回零
}

//日志实现handler.log，通过全局、本地筛选日志记录
//和回溯过滤器，如果允许它通过，最后发出它。
func（h*gloghandler）日志（r*record）错误
 //如果请求回溯，请检查这是否是调用站点
 如果atomic.loaduint32（&h.backtrace）>0
  //这里的一切都很慢。尽管我们可以缓存呼叫站点
  //和vmodule一样，回溯非常罕见，不值得额外增加
  / /复杂性。
  H.锁定（）
  匹配：=h.location==r.call.string（）
  h.lock.runlock（）。

  如果匹配{
   //调用站点匹配，将日志级别提升为INFO并收集堆栈
   吕林佛

   buf：=make（[]字节，1024*1024）
   buf=buf[：runtime.stack（buf，true）]
   r.msg+=“\n\n”+字符串（buf）
  }
 }
 //如果全局日志级别允许，则快速跟踪日志记录
 如果atomic.loadunt32（&h.level）>=uint32（r.lvl）
  返回h.origin.log（r）
 }
 //如果不存在本地重写，则快速跟踪跳过
 如果atomic.loaduint32（&h.override）==0
  返回零
 }
 //检查调用站点缓存中以前计算的日志级别
 H.锁定（）
 lvl，确定：=h.sitecache[r.call.pc（）]
 h.lock.runlock（）。

 //如果我们还没有缓存调用站点，请计算它
 如果！好吧{
  H.Lo.C.（）
  对于u，规则：=范围h.模式
   if rule.pattern.matchString（fmt.sprintf（“%+s”，r.call））
    h.sitecache[r.call.pc（）]，lvl，ok=rule.level，rule.level，真
    打破
   }
  }
  //如果没有匹配的规则，记得下次删除日志
  如果！好吧{
   h.sitecache[r.call.pc（）]=0
  }
  锁定（）
 }
 如果lvl>=r.lvl
  返回h.origin.log（r）
 }
 返回零
}
