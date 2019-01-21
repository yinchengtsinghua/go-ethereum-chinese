
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package log

import (
	"os"
)

var (
	root          = &logger{[]interface{}{}, new(swapHandler)}
	StdoutHandler = StreamHandler(os.Stdout, LogfmtFormat())
	StderrHandler = StreamHandler(os.Stderr, LogfmtFormat())
)

func init() {
	root.SetHandler(DiscardHandler())
}

//new返回具有给定上下文的新记录器。
//new是根（）的方便别名。new
func New(ctx ...interface{}) Logger {
	return root.New(ctx...)
}

//根返回根记录器
func Root() Logger {
	return root
}

//以下函数绕过导出的记录器方法（logger.debug，
//等）保持所有日志记录器路径的调用深度相同。
//运行时。调用方（2）总是在客户端代码中引用调用站点。

//trace是根（）的方便别名。
func Trace(msg string, ctx ...interface{}) {
	root.write(msg, LvlTrace, ctx, skipLevel)
}

//debug是根（）的方便别名。debug
func Debug(msg string, ctx ...interface{}) {
	root.write(msg, LvlDebug, ctx, skipLevel)
}

//info是根（）的方便别名。info
func Info(msg string, ctx ...interface{}) {
	root.write(msg, LvlInfo, ctx, skipLevel)
}

//warn是根（）的方便别名。warn
func Warn(msg string, ctx ...interface{}) {
	root.write(msg, LvlWarn, ctx, skipLevel)
}

//错误是根（）的方便别名。错误
func Error(msg string, ctx ...interface{}) {
	root.write(msg, LvlError, ctx, skipLevel)
}

//crit是root（）的方便别名。
func Crit(msg string, ctx ...interface{}) {
	root.write(msg, LvlCrit, ctx, skipLevel)
	os.Exit(1)
}

//输出是一个方便的写别名，允许修改
//调用深度（要跳过的堆栈帧数）。
//CallDepth影响日志消息的报告行号。
//CallDepth为零将报告输出的直接调用方。
//非零callDepth跳过的堆栈帧越多。
func Output(msg string, lvl Lvl, calldepth int, ctx ...interface{}) {
	root.write(msg, lvl, ctx, calldepth+skipLevel)
}
