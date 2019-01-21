
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

package console

import (
	"fmt"
	"strings"

	"github.com/peterh/liner"
)

//stdin保存stdin行阅读器（也使用stdout进行打印提示）。
//只有这个读卡器可以用于输入，因为它保留一个内部缓冲区。
var Stdin = newTerminalPrompter()

//userprompter定义控制台提示用户
//各种类型的输入。
type UserPrompter interface {
//promptinput向用户显示给定的提示并请求一些文本
//要输入的数据，返回用户的输入。
	PromptInput(prompt string) (string, error)

//提示密码向用户显示给定的提示并请求一些文本
//要输入的数据，但不能回送到终端。
//方法返回用户提供的输入。
	PromptPassword(prompt string) (string, error)

//promptconfirm向用户显示给定的提示并请求布尔值
//做出选择，返回该选择。
	PromptConfirm(prompt string) (bool, error)

//sethistory设置prompter允许的输入回滚历史记录
//要回滚到的用户。
	SetHistory(history []string)

//AppendHistory将一个条目追加到回滚历史记录。应该叫它
//如果且仅当追加提示是有效命令时。
	AppendHistory(command string)

//清除历史记录清除整个历史记录
	ClearHistory()

//setwordCompleter设置提示器将调用的完成函数
//当用户按Tab键时获取完成候选项。
	SetWordCompleter(completer WordCompleter)
}

//WordCompleter使用光标位置获取当前编辑的行，并
//返回要完成的部分单词的完成候选词。如果
//电话是“你好，我！！光标在第一个“！”之前，（“你好，
//哇！！，9）传递给可能返回的完成者（“hello，”，“world”，
//“Word”}，“！！！！“你好，世界！！.
type WordCompleter func(line string, pos int) (string, []string, string)

//TerminalPrompter是一个由liner包支持的用户Prompter。它支持
//提示用户输入各种输入，其中包括不回显密码
//输入。
type terminalPrompter struct {
	*liner.State
	warned     bool
	supported  bool
	normalMode liner.ModeApplier
	rawMode    liner.ModeApplier
}

//NewTerminalPrompter创建了一个基于行的用户输入提示器，用于
//标准输入和输出流。
func newTerminalPrompter() *terminalPrompter {
	p := new(terminalPrompter)
//在调用newliner之前获取原始模式。
//这通常是常规的“煮熟”模式，其中字符回音。
	normalMode, _ := liner.TerminalMode()
//打开班轮。它切换到原始模式。
	p.State = liner.NewLiner()
	rawMode, err := liner.TerminalMode()
	if err != nil || !liner.TerminalSupported() {
		p.supported = false
	} else {
		p.supported = true
		p.normalMode = normalMode
		p.rawMode = rawMode
//在不提示的情况下切换回正常模式。
		normalMode.ApplyMode()
	}
	p.SetCtrlCAborts(true)
	p.SetTabCompletionStyle(liner.TabPrints)
	p.SetMultiLineMode(true)
	return p
}

//promptinput向用户显示给定的提示并请求一些文本
//要输入的数据，返回用户的输入。
func (p *terminalPrompter) PromptInput(prompt string) (string, error) {
	if p.supported {
		p.rawMode.ApplyMode()
		defer p.normalMode.ApplyMode()
	} else {
//liner试图巧妙地打印提示
//如果输入被重定向，则不打印任何内容。
//总是通过打印提示来取消智能。
		fmt.Print(prompt)
		prompt = ""
		defer fmt.Println()
	}
	return p.State.Prompt(prompt)
}

//提示密码向用户显示给定的提示并请求一些文本
//要输入的数据，但不能回送到终端。
//方法返回用户提供的输入。
func (p *terminalPrompter) PromptPassword(prompt string) (passwd string, err error) {
	if p.supported {
		p.rawMode.ApplyMode()
		defer p.normalMode.ApplyMode()
		return p.State.PasswordPrompt(prompt)
	}
	if !p.warned {
		fmt.Println("!! Unsupported terminal, password will be echoed.")
		p.warned = true
	}
//正如在提示中一样，在这里处理打印提示，而不是依赖于衬线。
	fmt.Print(prompt)
	passwd, err = p.State.Prompt("")
	fmt.Println()
	return passwd, err
}

//promptconfirm向用户显示给定的提示并请求布尔值
//做出选择，返回该选择。
func (p *terminalPrompter) PromptConfirm(prompt string) (bool, error) {
	input, err := p.Prompt(prompt + " [y/N] ")
	if len(input) > 0 && strings.ToUpper(input[:1]) == "Y" {
		return true, nil
	}
	return false, err
}

//sethistory设置prompter允许的输入回滚历史记录
//要回滚到的用户。
func (p *terminalPrompter) SetHistory(history []string) {
	p.State.ReadHistory(strings.NewReader(strings.Join(history, "\n")))
}

//AppendHistory将一个条目追加到回滚历史记录。
func (p *terminalPrompter) AppendHistory(command string) {
	p.State.AppendHistory(command)
}

//清除历史记录清除整个历史记录
func (p *terminalPrompter) ClearHistory() {
	p.State.ClearHistory()
}

//setwordCompleter设置提示器将调用的完成函数
//当用户按Tab键时获取完成候选项。
func (p *terminalPrompter) SetWordCompleter(completer WordCompleter) {
	p.State.SetWordCompleter(liner.WordCompleter(completer))
}
