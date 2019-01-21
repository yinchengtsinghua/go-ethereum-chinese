
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
//此文件是Go以太坊的一部分。
//
//Go以太坊是免费软件：您可以重新发布和/或修改它
//根据GNU通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊的分布希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU通用公共许可证了解更多详细信息。
//
//你应该已经收到一份GNU通用公共许可证的副本
//一起去以太坊吧。如果没有，请参见<http://www.gnu.org/licenses/>。

package rules

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/signer/core"
	"github.com/ethereum/go-ethereum/signer/rules/deps"
	"github.com/ethereum/go-ethereum/signer/storage"
	"github.com/robertkrimen/otto"
)

var (
	BigNumber_JS = deps.MustAsset("bignumber.js")
)

//consoleoutput是console.log和console.error方法的重写，用于
//将输出流传输到配置的输出流，而不是stdout。
func consoleOutput(call otto.FunctionCall) otto.Value {
	output := []string{"JS:> "}
	for _, argument := range call.ArgumentList {
		output = append(output, fmt.Sprintf("%v", argument))
	}
	fmt.Fprintln(os.Stdout, strings.Join(output, " "))
	return otto.Value{}
}

//rulesetui提供了一个signerui的实现，它评估一个javascript
//每个定义的UI方法的文件
type rulesetUI struct {
next        core.SignerUI //下一个处理程序，用于手动处理
	storage     storage.Storage
	credentials storage.Storage
jsRules     string //使用的规则
}

func NewRuleEvaluator(next core.SignerUI, jsbackend, credentialsBackend storage.Storage) (*rulesetUI, error) {
	c := &rulesetUI{
		next:        next,
		storage:     jsbackend,
		credentials: credentialsBackend,
		jsRules:     "",
	}

	return c, nil
}

func (r *rulesetUI) Init(javascriptRules string) error {
	r.jsRules = javascriptRules
	return nil
}
func (r *rulesetUI) execute(jsfunc string, jsarg interface{}) (otto.Value, error) {

//每次实例化一个新的VM引擎
	vm := otto.New()
//设置本机回调
	consoleObj, _ := vm.Get("console")
	consoleObj.Object().Set("log", consoleOutput)
	consoleObj.Object().Set("error", consoleOutput)
	vm.Set("storage", r.storage)

//加载引导程序库
	script, err := vm.Compile("bignumber.js", BigNumber_JS)
	if err != nil {
		log.Warn("Failed loading libraries", "err", err)
		return otto.UndefinedValue(), err
	}
	vm.Run(script)

//运行实际规则实现
	_, err = vm.Run(r.jsRules)
	if err != nil {
		log.Warn("Execution failed", "err", err)
		return otto.UndefinedValue(), err
	}

//和实际通话
//所有调用都是对象，其中参数是该对象中的键。
//为了在JS和Go之间提供额外的隔离，我们在Go端将其序列化为JSON，
//并在JS端对其进行反序列化。

	jsonbytes, err := json.Marshal(jsarg)
	if err != nil {
		log.Warn("failed marshalling data", "data", jsarg)
		return otto.UndefinedValue(), err
	}
//现在，我们称之为foobar（json.parse（<jsondata>）。
	var call string
	if len(jsonbytes) > 0 {
		call = fmt.Sprintf("%v(JSON.parse(%v))", jsfunc, string(jsonbytes))
	} else {
		call = fmt.Sprintf("%v()", jsfunc)
	}
	return vm.Run(call)
}

func (r *rulesetUI) checkApproval(jsfunc string, jsarg []byte, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	v, err := r.execute(jsfunc, string(jsarg))
	if err != nil {
		log.Info("error occurred during execution", "error", err)
		return false, err
	}
	result, err := v.ToString()
	if err != nil {
		log.Info("error occurred during response unmarshalling", "error", err)
		return false, err
	}
	if result == "Approve" {
		log.Info("Op approved")
		return true, nil
	} else if result == "Reject" {
		log.Info("Op rejected")
		return false, nil
	}
	return false, fmt.Errorf("Unknown response")
}

func (r *rulesetUI) ApproveTx(request *core.SignTxRequest) (core.SignTxResponse, error) {
	jsonreq, err := json.Marshal(request)
	approved, err := r.checkApproval("ApproveTx", jsonreq, err)
	if err != nil {
		log.Info("Rule-based approval error, going to manual", "error", err)
		return r.next.ApproveTx(request)
	}

	if approved {
		return core.SignTxResponse{
				Transaction: request.Transaction,
				Approved:    true,
				Password:    r.lookupPassword(request.Transaction.From.Address()),
			},
			nil
	}
	return core.SignTxResponse{Approved: false}, err
}

func (r *rulesetUI) lookupPassword(address common.Address) string {
	return r.credentials.Get(strings.ToLower(address.String()))
}

func (r *rulesetUI) ApproveSignData(request *core.SignDataRequest) (core.SignDataResponse, error) {
	jsonreq, err := json.Marshal(request)
	approved, err := r.checkApproval("ApproveSignData", jsonreq, err)
	if err != nil {
		log.Info("Rule-based approval error, going to manual", "error", err)
		return r.next.ApproveSignData(request)
	}
	if approved {
		return core.SignDataResponse{Approved: true, Password: r.lookupPassword(request.Address.Address())}, nil
	}
	return core.SignDataResponse{Approved: false, Password: ""}, err
}

func (r *rulesetUI) ApproveExport(request *core.ExportRequest) (core.ExportResponse, error) {
	jsonreq, err := json.Marshal(request)
	approved, err := r.checkApproval("ApproveExport", jsonreq, err)
	if err != nil {
		log.Info("Rule-based approval error, going to manual", "error", err)
		return r.next.ApproveExport(request)
	}
	if approved {
		return core.ExportResponse{Approved: true}, nil
	}
	return core.ExportResponse{Approved: false}, err
}

func (r *rulesetUI) ApproveImport(request *core.ImportRequest) (core.ImportResponse, error) {
//这不能由规则处理，需要设置密码
//发送到下一个
	return r.next.ApproveImport(request)
}

//规则未处理OnInputRequired
func (r *rulesetUI) OnInputRequired(info core.UserInputRequest) (core.UserInputResponse, error) {
	return r.next.OnInputRequired(info)
}

func (r *rulesetUI) ApproveListing(request *core.ListRequest) (core.ListResponse, error) {
	jsonreq, err := json.Marshal(request)
	approved, err := r.checkApproval("ApproveListing", jsonreq, err)
	if err != nil {
		log.Info("Rule-based approval error, going to manual", "error", err)
		return r.next.ApproveListing(request)
	}
	if approved {
		return core.ListResponse{Accounts: request.Accounts}, nil
	}
	return core.ListResponse{}, err
}

func (r *rulesetUI) ApproveNewAccount(request *core.NewAccountRequest) (core.NewAccountResponse, error) {
//这不能由规则处理，需要设置密码
//发送到下一个
	return r.next.ApproveNewAccount(request)
}

func (r *rulesetUI) ShowError(message string) {
	log.Error(message)
	r.next.ShowError(message)
}

func (r *rulesetUI) ShowInfo(message string) {
	log.Info(message)
	r.next.ShowInfo(message)
}

func (r *rulesetUI) OnSignerStartup(info core.StartupInfo) {
	jsonInfo, err := json.Marshal(info)
	if err != nil {
		log.Warn("failed marshalling data", "data", info)
		return
	}
	r.next.OnSignerStartup(info)
	_, err = r.execute("OnSignerStartup", string(jsonInfo))
	if err != nil {
		log.Info("error occurred during execution", "error", err)
	}
}

func (r *rulesetUI) OnApprovedTx(tx ethapi.SignTransactionResult) {
	jsonTx, err := json.Marshal(tx)
	if err != nil {
		log.Warn("failed marshalling transaction", "tx", tx)
		return
	}
	_, err = r.execute("OnApprovedTx", string(jsonTx))
	if err != nil {
		log.Info("error occurred during execution", "error", err)
	}
}
