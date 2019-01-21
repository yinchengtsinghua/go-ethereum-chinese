
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
/*
包log15为最佳实践日志提供了一个独立、简单的工具包，即
人和机器都可读。它是根据标准库的IO和NET/HTTP建模的
包装。

此包强制您只记录键/值对。键必须是字符串。值可能是
任何你喜欢的类型。默认输出格式为logfmt，但也可以选择使用
如果你觉得合适的话，就改为JSON。以下是您登录的方式：

    log.info（“访问页面”，“路径”，r.url.path，“用户ID”，user.id）

这将输出如下行：

     lvl=info t=2014-05-02t16:07:23-0700 msg=“page accessed”path=/org/71/profile user_id=9

入门

要开始，您需要导入库：

    导入日志“github.com/inconschrevable/log15”


现在您可以开始记录了：

    FUNC主体（）
        log.info（“程序启动”，“args”，os.args（））
    }


公约

因为记录人类有意义的信息是很常见的，也是很好的实践，所以每个人的第一个论点
日志记录方法是*隐式*键'msg'的值。

此外，为消息选择的级别将自动添加键“lvl”，因此
将当前时间戳与键“t”一起使用。

您可以将任何附加上下文作为一组键/值对提供给日志函数。允许登录15
你喜欢简洁、有序和快速，而不是安全。这是一个合理的折衷
日志功能。您不需要显式地声明键/值，log15理解它们是交替的
在变量参数列表中：

    log warn（“大小越界”，“低”，“低”，“高”，“高”，“val”，val）

如果您确实支持类型安全，则可以选择传递一个log.ctx：

    log.warn（“大小越界”，log.ctx“低”：下限，“高”：上限，“val”：val）


上下文记录器

通常，您希望将上下文添加到记录器中，以便跟踪与之关联的操作。一个HTTP协议
请求就是一个很好的例子。您可以轻松地创建具有自动包含上下文的新记录器。
每条记录线：

    请求记录器：=log.new（“path”，r.url.path）

    /以后
    requestlogger.debug（“db txn commit”，“duration”，txtimer.finish（））

这将输出一条日志行，其中包含连接到记录器的路径上下文：

    lvl=dbug t=2014-05-02t16:07:23-0700 path=/repo/12/add25oke msg=“db txn commit”持续时间=0.12


处理程序

处理程序接口定义日志行的打印位置和格式。汉德勒是
受net/http处理程序接口启发的单个接口：

    类型处理程序接口
        日志（R*记录）错误
    }


处理程序可以筛选记录、格式化它们，或者分派到多个其他处理程序。
此包为常见的日志记录模式实现了许多处理程序
易于组合以创建灵活的自定义日志结构。

下面是一个将logfmt输出打印到stdout的示例处理程序：

    处理程序：=log.streamHandler（os.stdout，log.logfmtformat（））

下面是一个示例处理程序，它遵从其他两个处理程序。一个处理程序只打印记录
从logfmt中的rpc包到标准输出。其他打印错误级别的记录
json格式输出到文件/var/log/service.json中或更高版本

    处理程序：=log.multihandler（
        log.lvlfilterhandler（log.lvlerror，log.must.filehandler（“/var/log/service.json”，log.jsonformat（）），
        log.matchfilterhandler（“pkg”，“app/rpc”log.stdouthandler（））
    ）

记录文件名和行号

此包实现了三个处理程序，将调试信息添加到
Context、CallerFileHandler、CallerFundler和CallerStackHandler。这里是
将每个日志记录调用的源文件和行号添加到
语境。

    h：=log.CallerFileHandler（log.stdouthandler）
    log.root（）.sethandler（h）
    …
    log.error（“打开文件”，“err”，err）

这将输出如下行：

    lvl=eror t=2014-05-02t16:07:23-0700 msg=“open file”err=“file not found”caller=data.go:42

下面是一个记录调用堆栈而不仅仅是调用站点的示例。

    h：=log.callerstackhandler（“%+v”，log.stdouthandler）
    log.root（）.sethandler（h）
    …
    log.error（“打开文件”，“err”，err）

这将输出如下行：

    lvl=eror t=2014-05-02t16:07:23-0700 msg=“open file”err=“找不到文件”stack=“[pkg/data.go:42 pkg/cmd/main.go]”

“%+v”格式指示处理程序包含源文件的路径
相对于编译时gopath。github.com/go-stack/stack包
记录可用的格式化谓词和修饰符的完整列表。

自定义处理程序

处理程序接口非常简单，编写自己的接口也很简单。让我们创建一个
尝试写入一个处理程序的示例处理程序，但如果失败，则返回到
写入另一个处理程序，并包括尝试写入时遇到的错误
去初选。当试图通过网络套接字登录时，这可能很有用，但如果是这样的话
无法将这些记录记录记录到磁盘上的文件中。

    类型backuphandler结构
        主处理机
        辅助处理程序
    }

    func（h*backuphandler）日志（r*record）错误
        错误：=h.primary.log（r）
        如果犯错！= nIL{
            r.ctx=append（ctx，“主错误”，err）
            返回H.secondary.log（r）
        }
        返回零
    }

此模式非常有用，以至于处理任意数量处理程序的通用版本
包含在这个名为failhandler的库中。

记录昂贵的操作

有时，您希望记录计算非常昂贵的值，但不想支付
如果您没有将日志记录级别提高到较高的详细级别，计算它们的价格。

此包提供了一个简单的类型，用于注释要评估的日志记录操作。
懒惰地，就在它即将被记录时，这样就不会在上游处理程序
过滤掉它。只需将不带参数的任何函数与log.lazy类型一起包装。例如：

    func factorrsakey（）（factors[]int）
        //返回一个非常大的数的因子
    }

    log.debug（“factors”，log.lazy factorsakey）

如果由于任何原因（如错误级别的日志记录）未记录此消息，则
从未对factorsakey进行过评估。

动态上下文值

可以使用相同的log.lazy机制将上下文附加到您希望成为的记录器上。
在记录消息时计算，但在创建记录器时不计算。例如，让我们想象一下
有玩家对象的游戏：

    类型播放器结构
        名称字符串
        活布尔
        日志记录器
    }

你总是想记录一个玩家的名字，不管他们是活的还是死的，所以当你创建这个玩家时
对象，可以执行以下操作：

    p：=&player name:name，alive:true
    p.logger=log.new（“名称”，p.name，“活动”，p.live）

直到现在，即使玩家死了，日志记录程序仍会报告他们还活着，因为日志记录
创建记录器时将计算上下文。通过使用惰性包装器，我们可以推迟评估
玩家是否对每条日志消息都是活动的，这样日志记录将反映玩家的
当前状态，无论何时写入日志消息：

    p：=&player name:name，alive:true
    isalive：=func（）bool返回p.live
    player.logger=log.new（“名称”，p.name，“活动”，log.lazy isalive）

终端格式

如果log15检测到stdout是终端，它将配置默认值
它的处理程序（即log.stdouthandler）使用TerminalFormat。这种格式
为您的终端很好地记录，包括基于颜色编码的输出
在日志级别上。

错误处理

因为log15允许您绕过类型系统，所以有几种方法可以指定
日志函数的参数无效。例如，你可以包装一些不是
带log.lazy的零参数函数，或传递不是字符串的上下文键。因为日志记录库
通常是报告错误的机制，对于日志记录功能来说是很麻烦的
返回错误。相反，log15通过向您提供以下保证来处理错误：

-任何包含错误的日志记录仍将打印，并将错误解释为日志记录的一部分。

-任何包含错误的日志记录都将包含上下文键log15_错误，使您能够轻松
（如果您愿意，自动）检测您的任何日志记录调用是否传递了错误值。

了解这一点，您可能会想知道为什么处理程序接口可以在其日志方法中返回错误值。处理程序
只有当错误无法将其日志记录写入外部源时，才鼓励返回错误，例如
Syslog守护程序没有响应。这允许构造有用的处理程序来处理这些失败。
就像那个失败者。

图书馆使用

log15旨在对库作者有用，作为提供可配置的日志记录到
他们图书馆的用户。在库中使用的最佳实践是始终禁用记录器的所有输出
默认情况下，并提供库的使用者可以配置的公共记录器实例。像这样：

    包装你的衣服

    导入“github.com/inconschrevable/log15”

    var log=log.new（）。

    函数（）
        log.setHandler（log.discardHandler（））
    }

如果您的库用户愿意，可以启用它：

    导入“github.com/inconschrevable/log15”
    导入“example.com/yourlib”

    FUNC主体（）
        处理程序：=//自定义处理程序设置
        yourlib.log.sethandler（处理程序）
    }

附加记录器上下文的最佳实践

将上下文附加到记录器的能力非常强大。你应该在哪里做，为什么？
我喜欢将一个记录器直接嵌入到我的应用程序中的任何持久对象中，并添加
唯一的跟踪上下文键。例如，假设我正在编写一个Web浏览器：

    类型选项卡结构
        URL字符串
        渲染*渲染上下文
        /…

        记录器
    }

    func newtab（url字符串）*tab_
        返回和标签{
            /…
            网址：

            记录器：log.new（“url”，url），
        }
    }

当创建一个新的选项卡时，我会为它分配一个具有以下URL的记录器：
该选项卡作为上下文，以便通过日志轻松跟踪。
现在，每当我们对选项卡执行任何操作时，我们都将使用
嵌入式记录器，它将自动包含选项卡标题：

    tab.debug（“移动位置”，“idx”，tab.idx）

只有一个问题。如果标签URL改变了怎么办？我们可以
使用log.lazy确保始终写入当前的URL，但是
这意味着我们无法追踪标签的整个生命周期
用户导航到新的URL后登录。

相反，考虑一下要附加到记录器上的值
与您考虑在SQL数据库模式中作为键使用的方法相同。
如果可以使用在
对象，这样做。但除此之外，log15的ext包有一个方便的randid
函数来生成可能称为“代理键”的内容
它们只是用于跟踪的随机十六进制标识符。回到我们的身边
例如，我们希望像这样设置记录器：

        导入logext“github.com/inconschrevable/log15/ext”

        T:=和Tab {
            /…
            网址：
        }

        t.logger=log.new（“id”，logext.randid（8），“url”，log.lazy_t.geturl）
        返回T

现在，即使在加载新的URL时，我们也会有一个唯一的可跟踪标识符，但是
我们仍然可以在日志消息中看到选项卡的当前URL。

必须

对于所有可以返回错误的处理程序函数，都有一个版本
函数，它不会返回错误，但会在失败时出现恐慌。它们都有
在“必须”对象上。例如：

    log.must.filehandler（“/path”，log.jsonformat）
    log must.nethandler（“tcp”，“：1234”，log.jsonformat）

灵感与信用

以下所有优秀的项目都激发了这个图书馆的设计灵感：

code.google.com/p/log4go

github.com/op/go-logging

github.com/technoweenie/grohl公司

github.com/sirusen/logrus公司

github.com/kr/logfmt

github.com/spacemonkeygo/spacelog/空间日志

Golang的stdlib，特别是IO和net/http

名字

https://xkcd.com/927/

**/

package log
