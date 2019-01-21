
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package log

import (
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"sync"

	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-stack/stack"
)

//处理程序定义日志记录的写入位置和方式。
//记录器通过写入处理程序来打印其日志记录。
//处理程序是可组合的，为您提供了很大的组合灵活性
//它们可以实现适合您的应用程序的日志结构。
type Handler interface {
	Log(r *Record) error
}

//funchandler返回一个处理程序，该处理程序用给定的
//功能。
func FuncHandler(fn func(r *Record) error) Handler {
	return funcHandler(fn)
}

type funcHandler func(r *Record) error

func (h funcHandler) Log(r *Record) error {
	return h(r)
}

//StreamHandler将日志记录写入IO.Writer
//以给定的格式。可以使用流处理程序
//轻松开始向其他人写入日志记录
//输出。
//
//StreamHandler使用LazyHandler和Synchandler进行自我包装
//评估惰性对象并执行安全的并发写入。
func StreamHandler(wr io.Writer, fmtr Format) Handler {
	h := FuncHandler(func(r *Record) error {
		_, err := wr.Write(fmtr.Format(r))
		return err
	})
	return LazyHandler(SyncHandler(h))
}

//可以将同步处理程序包装在处理程序周围，以确保
//一次只能执行一个日志操作。这是必要的
//用于线程安全的并发写入。
func SyncHandler(h Handler) Handler {
	var mu sync.Mutex
	return FuncHandler(func(r *Record) error {
		defer mu.Unlock()
		mu.Lock()
		return h.Log(r)
	})
}

//file handler返回一个将日志记录写入给定文件的处理程序
//使用给定格式。如果路径
//文件处理程序将附加到给定的文件。如果没有，
//文件处理程序将使用模式0644创建文件。
func FileHandler(path string, fmtr Format) (Handler, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return closingHandler{f, StreamHandler(f, fmtr)}, nil
}

//CountingWriter包装WriteCloser对象以计算写入的字节数。
type countingWriter struct {
w     io.WriteCloser //被包裹的物体
count uint           //写入的字节数
}

//写入将字节计数器增加写入的字节数。
//实现WriteCloser接口。
func (w *countingWriter) Write(p []byte) (n int, err error) {
	n, err = w.w.Write(p)
	w.count += uint(n)
	return n, err
}

//close实现了writecloser接口。
func (w *countingWriter) Close() error {
	return w.w.Close()
}

//prepfile在给定路径打开日志文件，并切断无效部分
//因为上一次执行可能被中断而结束。
//假定以'\n'结尾的每一行都包含有效的日志记录。
func prepFile(path string) (*countingWriter, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	_, err = f.Seek(-1, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 1)
	var cut int64
	for {
		if _, err := f.Read(buf); err != nil {
			return nil, err
		}
		if buf[0] == '\n' {
			break
		}
		if _, err = f.Seek(-2, io.SeekCurrent); err != nil {
			return nil, err
		}
		cut++
	}
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	ns := fi.Size() - cut
	if err = f.Truncate(ns); err != nil {
		return nil, err
	}
	return &countingWriter{w: f, count: uint(ns)}, nil
}

//RotatingFileHandler返回一个将日志记录写入文件块的处理程序
//在给定的路径上。当文件大小达到限制时，处理程序将创建
//以将包含的第一条日志记录的时间戳命名的新文件。
func RotatingFileHandler(path string, limit uint, formatter Format) (Handler, error) {
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`\.log$`)
	last := len(files) - 1
	for last >= 0 && (!files[last].Mode().IsRegular() || !re.MatchString(files[last].Name())) {
		last--
	}
	var counter *countingWriter
	if last >= 0 && files[last].Size() < int64(limit) {
//打开最后一个文件，继续写入，直到其大小达到限制。
		if counter, err = prepFile(filepath.Join(path, files[last].Name())); err != nil {
			return nil, err
		}
	}
	if counter == nil {
		counter = new(countingWriter)
	}
	h := StreamHandler(counter, formatter)

	return FuncHandler(func(r *Record) error {
		if counter.count > limit {
			counter.Close()
			counter.w = nil
		}
		if counter.w == nil {
			f, err := os.OpenFile(
				filepath.Join(path, fmt.Sprintf("%s.log", strings.Replace(r.Time.Format("060102150405.00"), ".", "", 1))),
				os.O_CREATE|os.O_APPEND|os.O_WRONLY,
				0600,
			)
			if err != nil {
				return err
			}
			counter.w = f
			counter.count = 0
		}
		return h.Log(r)
	}), nil
}

//nethandler打开指定地址的套接字并写入记录
//通过连接。
func NetHandler(network, addr string, fmtr Format) (Handler, error) {
	conn, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}

	return closingHandler{conn, StreamHandler(conn, fmtr)}, nil
}

//xxx:closinghandler目前基本上没有使用过
//当处理程序接口支持
//可能的close（）操作
type closingHandler struct {
	io.WriteCloser
	Handler
}

func (h *closingHandler) Close() error {
	return h.WriteCloser.Close()
}

//CallerFileHandler返回一个处理程序，该处理程序添加
//使用键“caller”调用上下文的函数。
func CallerFileHandler(h Handler) Handler {
	return FuncHandler(func(r *Record) error {
		r.Ctx = append(r.Ctx, "caller", fmt.Sprint(r.Call))
		return h.Log(r)
	})
}

//CallerFundler返回一个将调用函数名添加到
//带有键“fn”的上下文。
func CallerFuncHandler(h Handler) Handler {
	return FuncHandler(func(r *Record) error {
		r.Ctx = append(r.Ctx, "fn", formatCall("%+n", r.Call))
		return h.Log(r)
	})
}

//此函数用于请在go<1.8时进行vet。
func formatCall(format string, c stack.Call) string {
	return fmt.Sprintf(format, c)
}

//CallersTackHandler返回一个向上下文添加堆栈跟踪的处理程序
//用“叠加”键。堆栈跟踪的格式为以空格分隔的
//匹配的[]中的呼叫站点。首先列出最近的呼叫站点。
//每个呼叫站点都按照格式进行格式化。参见以下文件：
//包github.com/go-stack/stack以获取支持的格式列表。
func CallerStackHandler(format string, h Handler) Handler {
	return FuncHandler(func(r *Record) error {
		s := stack.Trace().TrimBelow(r.Call).TrimRuntime()
		if len(s) > 0 {
			r.Ctx = append(r.Ctx, "stack", fmt.Sprintf(format, s))
		}
		return h.Log(r)
	})
}

//filterhandler返回只将记录写入
//如果给定函数的计算结果为true，则包装处理程序。例如，
//仅记录“err”键不为零的记录：
//
//logger.sethandler（filterhandler（func（r*record）bool_
//对于i：=0；i<len（r.ctx）；i+=2
//如果r.ctx[i]=“err”
//返回r.ctx[i+1]！=零
//}
//}
//返回假
//}，h）
//
func FilterHandler(fn func(r *Record) bool, h Handler) Handler {
	return FuncHandler(func(r *Record) error {
		if fn(r) {
			return h.Log(r)
		}
		return nil
	})
}

//MatchFilterHandler返回只写入记录的处理程序
//如果日志中的给定键
//上下文与值匹配。例如，只记录
//从您的UI包：
//
//log.matchfilterhandler（“pkg”，“app/ui”，log.stdouthandler）
//
func MatchFilterHandler(key string, value interface{}, h Handler) Handler {
	return FilterHandler(func(r *Record) (pass bool) {
		switch key {
		case r.KeyNames.Lvl:
			return r.Lvl == value
		case r.KeyNames.Time:
			return r.Time == value
		case r.KeyNames.Msg:
			return r.Msg == value
		}

		for i := 0; i < len(r.Ctx); i += 2 {
			if r.Ctx[i] == key {
				return r.Ctx[i+1] == value
			}
		}
		return false
	}, h)
}

//lvlFilterHandler返回一个只写
//低于给定详细程度的记录
//级别为包装处理程序。例如，仅
//日志错误/crit记录：
//
//log.lvlfilterhandler（log.lvlerror，log.stdouthandler）
//
func LvlFilterHandler(maxLvl Lvl, h Handler) Handler {
	return FilterHandler(func(r *Record) (pass bool) {
		return r.Lvl <= maxLvl
	}, h)
}

//多处理程序向其每个处理程序发送任何写操作。
//这对于写入不同类型的日志信息很有用
//去不同的地方。例如，登录到一个文件并
//标准误差：
//
//日志.multihandler（
//log.must.filehandler（“/var/log/app.log”，log.logfmtformat（）），
//日志.stderrhandler）
//
func MultiHandler(hs ...Handler) Handler {
	return FuncHandler(func(r *Record) error {
		for _, h := range hs {
//如何处理失败？
			h.Log(r)
		}
		return nil
	})
}

//failhandler将所有日志记录写入第一个处理程序
//已指定，但如果
//第一个处理程序失败，对所有指定的处理程序依此类推。
//例如，您可能希望登录到网络套接字，但故障转移
//如果网络出现故障，则写入文件，然后
//如果文件写入失败，则标准输出：
//
//日志.failhandler（
//log.must.nethandler（“tcp”，“：9090”，log.jsonformat（）），
//log.must.filehandler（“/var/log/app.log”，log.logfmtformat（）），
//日志.stdouthandler）
//
//不转到第一个处理程序的所有写入操作都将添加键为的上下文
//“故障转移”窗体，用于解释在
//尝试写入列表中处理程序之前的处理程序。
func FailoverHandler(hs ...Handler) Handler {
	return FuncHandler(func(r *Record) error {
		var err error
		for i, h := range hs {
			err = h.Log(r)
			if err == nil {
				return nil
			}
			r.Ctx = append(r.Ctx, fmt.Sprintf("failover_err_%d", i), err)
		}

		return err
	})
}

//channelhandler将所有记录写入给定的通道。
//如果通道已满，则会阻塞。用于异步处理
//对于日志消息，它由BufferedHandler使用。
func ChannelHandler(recs chan<- *Record) Handler {
	return FuncHandler(func(r *Record) error {
		recs <- r
		return nil
	})
}

//BufferedHandler将所有记录写入缓冲的
//给定大小的通道，冲入包装的
//处理程序，只要它可用于写入。既然这些
//写操作是异步进行的，所有写操作都是对BufferedHandler进行的
//永远不要返回错误，包装处理程序中的任何错误都将被忽略。
func BufferedHandler(bufSize int, h Handler) Handler {
	recs := make(chan *Record, bufSize)
	go func() {
		for m := range recs {
			_ = h.Log(m)
		}
	}()
	return ChannelHandler(recs)
}

//Lazyhandler在评估后将所有值写入打包的处理程序
//记录上下文中的任何惰性函数。它已经包好了
//关于这个库中的streamhandler和sysloghandler，您只需要
//如果您编写自己的处理程序。
func LazyHandler(h Handler) Handler {
	return FuncHandler(func(r *Record) error {
//遍历值（奇数索引）并重新分配
//任何延迟fn对其执行结果的值
		hadErr := false
		for i := 1; i < len(r.Ctx); i += 2 {
			lz, ok := r.Ctx[i].(Lazy)
			if ok {
				v, err := evaluateLazy(lz)
				if err != nil {
					hadErr = true
					r.Ctx[i] = err
				} else {
					if cs, ok := v.(stack.CallStack); ok {
						v = cs.TrimBelow(r.Call).TrimRuntime()
					}
					r.Ctx[i] = v
				}
			}
		}

		if hadErr {
			r.Ctx = append(r.Ctx, errorKey, "bad lazy")
		}

		return h.Log(r)
	})
}

func evaluateLazy(lz Lazy) (interface{}, error) {
	t := reflect.TypeOf(lz.Fn)

	if t.Kind() != reflect.Func {
		return nil, fmt.Errorf("INVALID_LAZY, not func: %+v", lz.Fn)
	}

	if t.NumIn() > 0 {
		return nil, fmt.Errorf("INVALID_LAZY, func takes args: %+v", lz.Fn)
	}

	if t.NumOut() == 0 {
		return nil, fmt.Errorf("INVALID_LAZY, no func return val: %+v", lz.Fn)
	}

	value := reflect.ValueOf(lz.Fn)
	results := value.Call([]reflect.Value{})
	if len(results) == 1 {
		return results[0].Interface(), nil
	}
	values := make([]interface{}, len(results))
	for i, v := range results {
		values[i] = v.Interface()
	}
	return values, nil
}

//DiscardHandler报告所有写入操作均成功，但不执行任何操作。
//它对于在运行时通过
//记录器的sethandler方法。
func DiscardHandler() Handler {
	return FuncHandler(func(r *Record) error {
		return nil
	})
}

//必须提供以下处理程序创建函数
//它不返回错误参数只返回一个处理程序
//失败时出现恐慌：filehandler、nethandler、sysloghandler、syslognethandler
var Must muster

func must(h Handler, err error) Handler {
	if err != nil {
		panic(err)
	}
	return h
}

type muster struct{}

func (m muster) FileHandler(path string, fmtr Format) Handler {
	return must(FileHandler(path, fmtr))
}

func (m muster) NetHandler(network, addr string, fmtr Format) Handler {
	return must(NetHandler(network, addr, fmtr))
}
