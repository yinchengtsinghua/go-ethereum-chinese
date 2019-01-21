
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
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"

	"github.com/ethereum/go-ethereum/internal/jsre"
	"github.com/ethereum/go-ethereum/internal/web3ext"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/mattn/go-colorable"
	"github.com/peterh/liner"
	"github.com/robertkrimen/otto"
)

var (
	passwordRegexp = regexp.MustCompile(`personal.[nus]`)
	onlyWhitespace = regexp.MustCompile(`^\s*$`)
	exit           = regexp.MustCompile(`^\s*exit\s*;*\s*$`)
)

//HistoryFile是数据目录中用于存储输入回滚的文件。
const HistoryFile = "history"

//默认提示是用于用户输入查询的默认提示行前缀。
const DefaultPrompt = "> "

//config是配置的集合，用于微调
//javascript控制台。
type Config struct {
DataDir  string       //存储控制台历史记录的数据目录
DocRoot  string       //从何处加载javascript文件的文件系统路径
Client   *rpc.Client  //通过RPC客户端执行以太坊请求
Prompt   string       //输入提示前缀字符串（默认为默认提示）
Prompter UserPrompter //输入提示以允许交互式用户反馈（默认为TerminalPrompter）
Printer  io.Writer    //要序列化任何显示字符串的输出编写器（默认为os.stdout）
Preload  []string     //要预加载的javascript文件的绝对路径
}

//控制台是一个JavaScript解释的运行时环境。它是一个成熟的
//通过外部或进程内RPC连接到运行节点的javascript控制台
//客户端。
type Console struct {
client   *rpc.Client  //通过RPC客户端执行以太坊请求
jsre     *jsre.JSRE   //运行解释器的javascript运行时环境
prompt   string       //输入提示前缀字符串
prompter UserPrompter //输入提示以允许交互式用户反馈
histPath string       //控制台回滚历史记录的绝对路径
history  []string     //控制台维护的滚动历史记录
printer  io.Writer    //要将任何显示字符串序列化到的输出编写器
}

//new初始化javascript解释的运行时环境并设置默认值
//使用config结构。
func New(config Config) (*Console, error) {
//优雅地处理未设置的配置值
	if config.Prompter == nil {
		config.Prompter = Stdin
	}
	if config.Prompt == "" {
		config.Prompt = DefaultPrompt
	}
	if config.Printer == nil {
		config.Printer = colorable.NewColorableStdout()
	}
//初始化控制台并返回
	console := &Console{
		client:   config.Client,
		jsre:     jsre.New(config.DocRoot, config.Printer),
		prompt:   config.Prompt,
		prompter: config.Prompter,
		printer:  config.Printer,
		histPath: filepath.Join(config.DataDir, HistoryFile),
	}
	if err := os.MkdirAll(config.DataDir, 0700); err != nil {
		return nil, err
	}
	if err := console.init(config.Preload); err != nil {
		return nil, err
	}
	return console, nil
}

//init从远程RPC提供程序检索可用的API并初始化
//控制台的javascript名称空间基于公开的模块。
func (c *Console) init(preload []string) error {
//初始化javascript<->go-rpc桥
	bridge := newBridge(c.client, c.prompter, c.printer)
	c.jsre.Set("jeth", struct{}{})

	jethObj, _ := c.jsre.Get("jeth")
	jethObj.Object().Set("send", bridge.Send)
	jethObj.Object().Set("sendAsync", bridge.Send)

	consoleObj, _ := c.jsre.Get("console")
	consoleObj.Object().Set("log", c.consoleOutput)
	consoleObj.Object().Set("error", c.consoleOutput)

//加载所有内部实用程序javascript库
	if err := c.jsre.Compile("bignumber.js", jsre.BigNumber_JS); err != nil {
		return fmt.Errorf("bignumber.js: %v", err)
	}
	if err := c.jsre.Compile("web3.js", jsre.Web3_JS); err != nil {
		return fmt.Errorf("web3.js: %v", err)
	}
	if _, err := c.jsre.Run("var Web3 = require('web3');"); err != nil {
		return fmt.Errorf("web3 require: %v", err)
	}
	if _, err := c.jsre.Run("var web3 = new Web3(jeth);"); err != nil {
		return fmt.Errorf("web3 provider: %v", err)
	}
//将支持的API加载到JavaScript运行时环境中
	apis, err := c.client.SupportedModules()
	if err != nil {
		return fmt.Errorf("api modules: %v", err)
	}
	flatten := "var eth = web3.eth; var personal = web3.personal; "
	for api := range apis {
		if api == "web3" {
continue //手动映射或忽略
		}
		if file, ok := web3ext.Modules[api]; ok {
//加载模块的扩展。
			if err = c.jsre.Compile(fmt.Sprintf("%s.js", api), file); err != nil {
				return fmt.Errorf("%s.js: %v", api, err)
			}
			flatten += fmt.Sprintf("var %s = web3.%s; ", api, api)
		} else if obj, err := c.jsre.Run("web3." + api); err == nil && obj.IsObject() {
//启用web3.js内置扩展（如果可用）。
			flatten += fmt.Sprintf("var %s = web3.%s; ", api, api)
		}
	}
	if _, err = c.jsre.Run(flatten); err != nil {
		return fmt.Errorf("namespace flattening: %v", err)
	}
//初始化全局名称寄存器（暂时禁用）
//c.jsre.run（`var globalregistrar=eth.contract（`+registrar.globalregistrabi+`）；registrar=globalregistrar.at（`+registrar.globalregistraradr+`）；`）

//如果控制台处于交互模式，则使用与仪器密码相关的方法查询用户
	if c.prompter != nil {
//检索要检测的帐户管理对象
		personal, err := c.jsre.Get("personal")
		if err != nil {
			return err
		}
//重写openwallet、unlockaccount、newaccount和sign方法，因为
//这些需要用户交互。在控制台中分配这些方法
//原始Web3回调。这些将由jeth.*方法在
//他们从用户那里获得了密码，并将原始Web3请求发送到
//后端。
if obj := personal.Object(); obj != nil { //确保通过接口启用个人API
			if _, err = c.jsre.Run(`jeth.openWallet = personal.openWallet;`); err != nil {
				return fmt.Errorf("personal.openWallet: %v", err)
			}
			if _, err = c.jsre.Run(`jeth.unlockAccount = personal.unlockAccount;`); err != nil {
				return fmt.Errorf("personal.unlockAccount: %v", err)
			}
			if _, err = c.jsre.Run(`jeth.newAccount = personal.newAccount;`); err != nil {
				return fmt.Errorf("personal.newAccount: %v", err)
			}
			if _, err = c.jsre.Run(`jeth.sign = personal.sign;`); err != nil {
				return fmt.Errorf("personal.sign: %v", err)
			}
			obj.Set("openWallet", bridge.OpenWallet)
			obj.Set("unlockAccount", bridge.UnlockAccount)
			obj.Set("newAccount", bridge.NewAccount)
			obj.Set("sign", bridge.Sign)
		}
	}
//admin.sleep和admin.sleepBlocks由控制台提供，而不是由rpc层提供。
	admin, err := c.jsre.Get("admin")
	if err != nil {
		return err
	}
if obj := admin.Object(); obj != nil { //确保通过接口启用管理API
		obj.Set("sleepBlocks", bridge.SleepBlocks)
		obj.Set("sleep", bridge.Sleep)
		obj.Set("clearHistory", c.clearHistory)
	}
//在启动控制台之前预加载任何javascript文件
	for _, path := range preload {
		if err := c.jsre.Exec(path); err != nil {
			failure := err.Error()
			if ottoErr, ok := err.(*otto.Error); ok {
				failure = ottoErr.String()
			}
			return fmt.Errorf("%s: %v", path, failure)
		}
	}
//配置控制台的输入Prompter以完成回滚和选项卡。
	if c.prompter != nil {
		if content, err := ioutil.ReadFile(c.histPath); err != nil {
			c.prompter.SetHistory(nil)
		} else {
			c.history = strings.Split(string(content), "\n")
			c.prompter.SetHistory(c.history)
		}
		c.prompter.SetWordCompleter(c.AutoCompleteInput)
	}
	return nil
}

func (c *Console) clearHistory() {
	c.history = nil
	c.prompter.ClearHistory()
	if err := os.Remove(c.histPath); err != nil {
		fmt.Fprintln(c.printer, "can't delete history file:", err)
	} else {
		fmt.Fprintln(c.printer, "history file deleted")
	}
}

//consoleoutput是console.log和console.error方法的重写，用于
//将输出流传输到配置的输出流，而不是stdout。
func (c *Console) consoleOutput(call otto.FunctionCall) otto.Value {
	output := []string{}
	for _, argument := range call.ArgumentList {
		output = append(output, fmt.Sprintf("%v", argument))
	}
	fmt.Fprintln(c.printer, strings.Join(output, " "))
	return otto.Value{}
}

//autocompleteinput是一个预先组装好的单词完成器，供用户使用。
//输入prompter以向用户提供有关可用方法的提示。
func (c *Console) AutoCompleteInput(line string, pos int) (string, []string, string) {
//不能为空输入提供完成
	if len(line) == 0 || pos == 0 {
		return "", nil, ""
	}
//将数据分块到相关部分进行自动完成
//例如，如果是嵌套行eth.getbalance（eth.coinb<tab><tab>
	start := pos - 1
	for ; start > 0; start-- {
//跳过所有方法和名称空间（即包括点）
		if line[start] == '.' || (line[start] >= 'a' && line[start] <= 'z') || (line[start] >= 'A' && line[start] <= 'Z') {
			continue
		}
//以特殊方式处理Web3（即其他数字不会自动完成）
		if start >= 3 && line[start-3:start] == "web3" {
			start -= 3
			continue
		}
//我们碰到了一个意想不到的字符，这里是自动完成表单
		start++
		break
	}
	return line[:start], c.jsre.CompleteKeywords(line[start:pos]), line[pos:]
}

//欢迎显示当前geth实例的摘要和有关
//控制台的可用模块。
func (c *Console) Welcome() {
//打印一些通用的geth元数据
	fmt.Fprintf(c.printer, "Welcome to the Geth JavaScript console!\n\n")
	c.jsre.Run(`
		console.log("instance: " + web3.version.node);
		console.log("coinbase: " + eth.coinbase);
		console.log("at block: " + eth.blockNumber + " (" + new Date(1000 * eth.getBlock(eth.blockNumber).timestamp) + ")");
		console.log(" datadir: " + admin.datadir);
	`)
//列出用户可以调用的所有支持模块
	if apis, err := c.client.SupportedModules(); err == nil {
		modules := make([]string, 0, len(apis))
		for api, version := range apis {
			modules = append(modules, fmt.Sprintf("%s:%s", api, version))
		}
		sort.Strings(modules)
		fmt.Fprintln(c.printer, " modules:", strings.Join(modules, " "))
	}
	fmt.Fprintln(c.printer)
}

//evaluate执行代码并将结果漂亮地打印到指定的输出
//溪流。
func (c *Console) Evaluate(statement string) error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(c.printer, "[native] error: %v\n", r)
		}
	}()
	return c.jsre.Evaluate(statement, c.printer)
}

//Interactive启动一个交互式用户会话，在该会话中输入来自
//the configured user prompter.
func (c *Console) Interactive() {
	var (
prompt    = c.prompt          //当前提示行（用于多行输入）
indents   = 0                 //当前输入缩进数（用于多行输入）
input     = ""                //当前用户输入
scheduler = make(chan string) //发送下一个提示并接收输入的通道
	)
//启动Goroutine以侦听提示请求并返回输入
	go func() {
		for {
//读取下一个用户输入
			line, err := c.prompter.PromptInput(<-scheduler)
			if err != nil {
//如果出现错误，请清除提示或失败。
if err == liner.ErrPromptAborted { //Ctrl—C
					prompt, indents, input = c.prompt, 0, ""
					scheduler <- ""
					continue
				}
				close(scheduler)
				return
			}
//检索用户输入、发送解释和循环
			scheduler <- line
		}
	}()
//如果输入为空，我们需要退出时也监视ctrl-c
	abort := make(chan os.Signal, 1)
	signal.Notify(abort, syscall.SIGINT, syscall.SIGTERM)

//开始向用户发送提示并读取输入
	for {
//发送下一个提示，触发输入读取并处理结果
		scheduler <- prompt
		select {
		case <-abort:
//用户强制相当于控制台
			fmt.Fprintln(c.printer, "caught interrupt, exiting")
			return

		case line, ok := <-scheduler:
//提示器返回用户输入，处理特殊情况
			if !ok || (indents <= 0 && exit.MatchString(line)) {
				return
			}
			if onlyWhitespace.MatchString(line) {
				continue
			}
//将行附加到输入并检查多行解释
			input += line + "\n"

			indents = countIndents(input)
			if indents <= 0 {
				prompt = c.prompt
			} else {
				prompt = strings.Repeat(".", indents*3) + " "
			}
//如果所有需要的行都存在，请保存命令并运行
			if indents <= 0 {
				if len(input) > 0 && input[0] != ' ' && !passwordRegexp.MatchString(input) {
					if command := strings.TrimSpace(input); len(c.history) == 0 || command != c.history[len(c.history)-1] {
						c.history = append(c.history, command)
						if c.prompter != nil {
							c.prompter.AppendHistory(command)
						}
					}
				}
				c.Evaluate(input)
				input = ""
			}
		}
	}
}

//countindents返回给定输入的标识数。
//如果输入无效，例如var a=，结果可以是负数。
func countIndents(input string) int {
	var (
		indents     = 0
		inString    = false
strOpenChar = ' '   //跟踪字符串open char以允许var str=“i'm…..”；
charEscaped = false //如果前一个字符是'\'字符，则允许var str=“abc\”def；
	)

	for _, c := range input {
		switch c {
		case '\\':
//在字符串中指示下一个字符为转义字符，而上一个字符不转义此反斜杠
			if !charEscaped && inString {
				charEscaped = true
			}
		case '\'', '"':
if inString && !charEscaped && strOpenChar == c { //结束字符串
				inString = false
} else if !inString && !charEscaped { //开始字符串
				inString = true
				strOpenChar = c
			}
			charEscaped = false
		case '{', '(':
if !inString { //在字符串中忽略括号，允许var str=“a”；而不缩进
				indents++
			}
			charEscaped = false
		case '}', ')':
			if !inString {
				indents--
			}
			charEscaped = false
		default:
			charEscaped = false
		}
	}

	return indents
}

//执行运行指定为参数的javascript文件。
func (c *Console) Execute(path string) error {
	return c.jsre.Exec(path)
}

//停止清理控制台并终止运行时环境。
func (c *Console) Stop(graceful bool) error {
	if err := ioutil.WriteFile(c.histPath, []byte(strings.Join(c.history, "\n")), 0600); err != nil {
		return err
	}
if err := os.Chmod(c.histPath, 0600); err != nil { //强制0600，即使以前不同
		return err
	}
	c.jsre.Stop(graceful)
	return nil
}
