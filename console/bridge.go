
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
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/usbwallet"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/robertkrimen/otto"
)

//Bridge是一组javascript实用程序方法，用于连接.js运行时。
//环境和支持远程方法调用的go-rpc连接。
type bridge struct {
client   *rpc.Client  //通过RPC客户端执行以太坊请求
prompter UserPrompter //输入提示以允许交互式用户反馈
printer  io.Writer    //要将任何显示字符串序列化到的输出编写器
}

//Newbridge在一个RPC客户机上创建了一个新的javascript包装器。
func newBridge(client *rpc.Client, prompter UserPrompter, printer io.Writer) *bridge {
	return &bridge{
		client:   client,
		prompter: prompter,
		printer:  printer,
	}
}

//newaccount是围绕使用
//不回显密码提示获取密码短语并执行原始密码
//rpc方法（保存在jeth.newaccount中）用于实际执行rpc调用。
func (b *bridge) NewAccount(call otto.FunctionCall) (response otto.Value) {
	var (
		password string
		confirm  string
		err      error
	)
	switch {
//未指定密码，提示用户输入密码
	case len(call.ArgumentList) == 0:
		if password, err = b.prompter.PromptPassword("Passphrase: "); err != nil {
			throwJSException(err.Error())
		}
		if confirm, err = b.prompter.PromptPassword("Repeat passphrase: "); err != nil {
			throwJSException(err.Error())
		}
		if password != confirm {
			throwJSException("passphrases don't match!")
		}

//指定了单个字符串密码，请使用
	case len(call.ArgumentList) == 1 && call.Argument(0).IsString():
		password, _ = call.Argument(0).ToString()

//否则会因某些错误而失败
	default:
		throwJSException("expected 0 or 1 string argument")
	}
//获取密码，执行呼叫并返回
	ret, err := call.Otto.Call("jeth.newAccount", nil, password)
	if err != nil {
		throwJSException(err.Error())
	}
	return ret
}

//openwallet是一个围绕personal.openwallet的包装，它可以解释和
//对某些错误消息作出反应，例如trezor pin matrix请求。
func (b *bridge) OpenWallet(call otto.FunctionCall) (response otto.Value) {
//确保我们有一个指定要打开的钱包
	if !call.Argument(0).IsString() {
		throwJSException("first argument must be the wallet URL to open")
	}
	wallet := call.Argument(0)

	var passwd otto.Value
	if call.Argument(1).IsUndefined() || call.Argument(1).IsNull() {
		passwd, _ = otto.ToValue("")
	} else {
		passwd = call.Argument(1)
	}
//打开钱包，如果成功的话返回
	val, err := call.Otto.Call("jeth.openWallet", nil, wallet, passwd)
	if err == nil {
		return val
	}
//钱包打开失败，除非是密码输入，否则报告错误
	if !strings.HasSuffix(err.Error(), usbwallet.ErrTrezorPINNeeded.Error()) {
		throwJSException(err.Error())
	}
//请求Trezor管脚矩阵输入，向用户显示矩阵并获取数据
	fmt.Fprintf(b.printer, "Look at the device for number positions\n\n")
	fmt.Fprintf(b.printer, "7 | 8 | 9\n")
	fmt.Fprintf(b.printer, "--+---+--\n")
	fmt.Fprintf(b.printer, "4 | 5 | 6\n")
	fmt.Fprintf(b.printer, "--+---+--\n")
	fmt.Fprintf(b.printer, "1 | 2 | 3\n\n")

	if input, err := b.prompter.PromptPassword("Please enter current PIN: "); err != nil {
		throwJSException(err.Error())
	} else {
		passwd, _ = otto.ToValue(input)
	}
	if val, err = call.Otto.Call("jeth.openWallet", nil, wallet, passwd); err != nil {
		throwJSException(err.Error())
	}
	return val
}

//UnlockAccount是围绕Personal.UnlockAccount RPC方法的包装，
//使用非回显密码提示获取密码短语并执行
//
//RPC调用。
func (b *bridge) UnlockAccount(call otto.FunctionCall) (response otto.Value) {
//确保我们指定了要解锁的帐户
	if !call.Argument(0).IsString() {
		throwJSException("first argument must be the account to unlock")
	}
	account := call.Argument(0)

//如果未给出密码或是空值，提示用户输入密码。
	var passwd otto.Value

	if call.Argument(1).IsUndefined() || call.Argument(1).IsNull() {
		fmt.Fprintf(b.printer, "Unlock account %s\n", account)
		if input, err := b.prompter.PromptPassword("Passphrase: "); err != nil {
			throwJSException(err.Error())
		} else {
			passwd, _ = otto.ToValue(input)
		}
	} else {
		if !call.Argument(1).IsString() {
			throwJSException("password must be a string")
		}
		passwd = call.Argument(1)
	}
//第三个参数是帐户必须解锁的持续时间。
	duration := otto.NullValue()
	if call.Argument(2).IsDefined() && !call.Argument(2).IsNull() {
		if !call.Argument(2).IsNumber() {
			throwJSException("unlock duration must be a number")
		}
		duration = call.Argument(2)
	}
//将请求发送到后端并返回
	val, err := call.Otto.Call("jeth.unlockAccount", nil, account, passwd, duration)
	if err != nil {
		throwJSException(err.Error())
	}
	return val
}

//sign是围绕personal.sign rpc方法的包装，该方法使用非回显密码。
//提示获取密码短语并执行原始RPC方法（保存在
//用它来实际执行RPC调用。
func (b *bridge) Sign(call otto.FunctionCall) (response otto.Value) {
	var (
		message = call.Argument(0)
		account = call.Argument(1)
		passwd  = call.Argument(2)
	)

	if !message.IsString() {
		throwJSException("first argument must be the message to sign")
	}
	if !account.IsString() {
		throwJSException("second argument must be the account to sign with")
	}

//如果未提供密码或密码为空，请询问用户并确保密码为字符串
	if passwd.IsUndefined() || passwd.IsNull() {
		fmt.Fprintf(b.printer, "Give password for account %s\n", account)
		if input, err := b.prompter.PromptPassword("Passphrase: "); err != nil {
			throwJSException(err.Error())
		} else {
			passwd, _ = otto.ToValue(input)
		}
	}
	if !passwd.IsString() {
		throwJSException("third argument must be the password to unlock the account")
	}

//将请求发送到后端并返回
	val, err := call.Otto.Call("jeth.sign", nil, message, account, passwd)
	if err != nil {
		throwJSException(err.Error())
	}
	return val
}

//睡眠将在指定的秒数内阻止控制台。
func (b *bridge) Sleep(call otto.FunctionCall) (response otto.Value) {
	if call.Argument(0).IsNumber() {
		sleep, _ := call.Argument(0).ToInteger()
		time.Sleep(time.Duration(sleep) * time.Second)
		return otto.TrueValue()
	}
	return throwJSException("usage: sleep(<number of seconds>)")
}

//SleepBlocks将为指定数量的新块（可选）阻止控制台
//直到达到给定的超时。
func (b *bridge) SleepBlocks(call otto.FunctionCall) (response otto.Value) {
	var (
		blocks = int64(0)
sleep  = int64(9999999999999999) //无限期地
	)
//分析睡眠的输入参数
	nArgs := len(call.ArgumentList)
	if nArgs == 0 {
		throwJSException("usage: sleepBlocks(<n blocks>[, max sleep in seconds])")
	}
	if nArgs >= 1 {
		if call.Argument(0).IsNumber() {
			blocks, _ = call.Argument(0).ToInteger()
		} else {
			throwJSException("expected number as first argument")
		}
	}
	if nArgs >= 2 {
		if call.Argument(1).IsNumber() {
			sleep, _ = call.Argument(1).ToInteger()
		} else {
			throwJSException("expected number as second argument")
		}
	}
//通过控制台，这将允许Web3调用适当的
//如果收到延迟的响应或通知，则进行回调。
	blockNumber := func() int64 {
		result, err := call.Otto.Run("eth.blockNumber")
		if err != nil {
			throwJSException(err.Error())
		}
		block, err := result.ToInteger()
		if err != nil {
			throwJSException(err.Error())
		}
		return block
	}
//轮询当前块号，直到超时为止
	targetBlockNr := blockNumber() + blocks
	deadline := time.Now().Add(time.Duration(sleep) * time.Second)

	for time.Now().Before(deadline) {
		if blockNumber() >= targetBlockNr {
			return otto.TrueValue()
		}
		time.Sleep(time.Second)
	}
	return otto.FalseValue()
}

type jsonrpcCall struct {
	ID     int64
	Method string
	Params []interface{}
}

//send实现Web3提供程序“send”方法。
func (b *bridge) Send(call otto.FunctionCall) (response otto.Value) {
//将请求改为go值。
	JSON, _ := call.Otto.Object("JSON")
	reqVal, err := JSON.Call("stringify", call.Argument(0))
	if err != nil {
		throwJSException(err.Error())
	}
	var (
		rawReq = reqVal.String()
		dec    = json.NewDecoder(strings.NewReader(rawReq))
		reqs   []jsonrpcCall
		batch  bool
	)
dec.UseNumber() //避免浮标64
	if rawReq[0] == '[' {
		batch = true
		dec.Decode(&reqs)
	} else {
		batch = false
		reqs = make([]jsonrpcCall, 1)
		dec.Decode(&reqs[0])
	}

//执行请求。
	resps, _ := call.Otto.Object("new Array()")
	for _, req := range reqs {
		resp, _ := call.Otto.Object(`({"jsonrpc":"2.0"})`)
		resp.Set("id", req.ID)
		var result json.RawMessage
		err = b.client.Call(&result, req.Method, req.Params...)
		switch err := err.(type) {
		case nil:
			if result == nil {
//特殊情况为空，因为它被解码为空
//出于某种原因，原始消息。
				resp.Set("result", otto.NullValue())
			} else {
				resultVal, err := JSON.Call("parse", string(result))
				if err != nil {
					setError(resp, -32603, err.Error())
				} else {
					resp.Set("result", resultVal)
				}
			}
		case rpc.Error:
			setError(resp, err.ErrorCode(), err.Error())
		default:
			setError(resp, -32603, err.Error())
		}
		resps.Call("push", resp)
	}

//返回对回调的响应（如果提供）
//
	if batch {
		response = resps.Value()
	} else {
		response, _ = resps.Get("0")
	}
	if fn := call.Argument(1); fn.Class() == "Function" {
		fn.Call(otto.NullValue(), otto.NullValue(), response)
		return otto.UndefinedValue()
	}
	return response
}

func setError(resp *otto.Object, code int, msg string) {
	resp.Set("error", map[string]interface{}{"code": code, "message": msg})
}

//throwjsException在otto.value上崩溃。奥托虚拟机将从
//惊慌失措，将msg作为一个javascript错误抛出。
func throwJSException(msg interface{}) otto.Value {
	val, err := otto.ToValue(msg)
	if err != nil {
		log.Error("Failed to serialize JavaScript exception", "exception", msg, "err", err)
	}
	panic(val)
}
