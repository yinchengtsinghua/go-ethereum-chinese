
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
//
package rules

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/signer/core"
	"github.com/ethereum/go-ethereum/signer/storage"
)

const JS = `
/*
这是一个JavaScript规则文件的示例实现。

当签名者通过外部API接收到请求时，将计算相应的方法。
可能发生三件事：

1。方法返回“批准”。这意味着操作是允许的。
2。方法返回“reject”。这意味着操作被拒绝。
三。其他任何内容；其他返回值[*]，方法未实现或在处理过程中发生异常。这意味着
操作将继续通过用户选择的常规UI方法进行手动处理。

[*]注意：未来版本的规则集可能会使用更复杂的基于JSON的返回值，因此可能不会
只响应批准/拒绝/手动，但也修改响应。例如，选择只列出一个，而不是全部
列表请求中的帐户。以上几点将继续适用于非基于JSON的响应（“批准”/“拒绝”）。

*/


function ApproveListing(request){
	console.log("In js approve listing");
	console.log(request.accounts[3].Address)
	console.log(request.meta.Remote)
	return "Approve"
}

function ApproveTx(request){
	console.log("test");
	console.log("from");
	return "Reject";
}

function test(thing){
	console.log(thing.String())
}

`

func mixAddr(a string) (*common.MixedcaseAddress, error) {
	return common.NewMixedcaseAddressFromString(a)
}

type alwaysDenyUI struct{}

func (alwaysDenyUI) OnInputRequired(info core.UserInputRequest) (core.UserInputResponse, error) {
	return core.UserInputResponse{}, nil
}

func (alwaysDenyUI) OnSignerStartup(info core.StartupInfo) {
}

func (alwaysDenyUI) OnMasterPassword(request *core.PasswordRequest) (core.PasswordResponse, error) {
	return core.PasswordResponse{}, nil
}

func (alwaysDenyUI) ApproveTx(request *core.SignTxRequest) (core.SignTxResponse, error) {
	return core.SignTxResponse{Transaction: request.Transaction, Approved: false, Password: ""}, nil
}

func (alwaysDenyUI) ApproveSignData(request *core.SignDataRequest) (core.SignDataResponse, error) {
	return core.SignDataResponse{Approved: false, Password: ""}, nil
}

func (alwaysDenyUI) ApproveExport(request *core.ExportRequest) (core.ExportResponse, error) {
	return core.ExportResponse{Approved: false}, nil
}

func (alwaysDenyUI) ApproveImport(request *core.ImportRequest) (core.ImportResponse, error) {
	return core.ImportResponse{Approved: false, OldPassword: "", NewPassword: ""}, nil
}

func (alwaysDenyUI) ApproveListing(request *core.ListRequest) (core.ListResponse, error) {
	return core.ListResponse{Accounts: nil}, nil
}

func (alwaysDenyUI) ApproveNewAccount(request *core.NewAccountRequest) (core.NewAccountResponse, error) {
	return core.NewAccountResponse{Approved: false, Password: ""}, nil
}

func (alwaysDenyUI) ShowError(message string) {
	panic("implement me")
}

func (alwaysDenyUI) ShowInfo(message string) {
	panic("implement me")
}

func (alwaysDenyUI) OnApprovedTx(tx ethapi.SignTransactionResult) {
	panic("implement me")
}

func initRuleEngine(js string) (*rulesetUI, error) {
	r, err := NewRuleEvaluator(&alwaysDenyUI{}, storage.NewEphemeralStorage(), storage.NewEphemeralStorage())
	if err != nil {
		return nil, fmt.Errorf("failed to create js engine: %v", err)
	}
	if err = r.Init(js); err != nil {
		return nil, fmt.Errorf("failed to load bootstrap js: %v", err)
	}
	return r, nil
}

func TestListRequest(t *testing.T) {
	accs := make([]core.Account, 5)

	for i := range accs {
		addr := fmt.Sprintf("000000000000000000000000000000000000000%x", i)
		acc := core.Account{
			Address: common.BytesToAddress(common.Hex2Bytes(addr)),
			URL:     accounts.URL{Scheme: "test", Path: fmt.Sprintf("acc-%d", i)},
		}
		accs[i] = acc
	}

	js := `function ApproveListing(){ return "Approve" }`

	r, err := initRuleEngine(js)
	if err != nil {
		t.Errorf("Couldn't create evaluator %v", err)
		return
	}
	resp, _ := r.ApproveListing(&core.ListRequest{
		Accounts: accs,
		Meta:     core.Metadata{Remote: "remoteip", Local: "localip", Scheme: "inproc"},
	})
	if len(resp.Accounts) != len(accs) {
		t.Errorf("Expected check to resolve to 'Approve'")
	}
}

func TestSignTxRequest(t *testing.T) {

	js := `
	function ApproveTx(r){
		console.log("transaction.from", r.transaction.from);
		console.log("transaction.to", r.transaction.to);
		console.log("transaction.value", r.transaction.value);
		console.log("transaction.nonce", r.transaction.nonce);
		if(r.transaction.from.toLowerCase()=="0x0000000000000000000000000000000000001337"){ return "Approve"}
		if(r.transaction.from.toLowerCase()=="0x000000000000000000000000000000000000dead"){ return "Reject"}
	}`

	r, err := initRuleEngine(js)
	if err != nil {
		t.Errorf("Couldn't create evaluator %v", err)
		return
	}
	to, err := mixAddr("000000000000000000000000000000000000dead")
	if err != nil {
		t.Error(err)
		return
	}
	from, err := mixAddr("0000000000000000000000000000000000001337")

	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("to %v", to.Address().String())
	resp, err := r.ApproveTx(&core.SignTxRequest{
		Transaction: core.SendTxArgs{
			From: *from,
			To:   to},
		Callinfo: nil,
		Meta:     core.Metadata{Remote: "remoteip", Local: "localip", Scheme: "inproc"},
	})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if !resp.Approved {
		t.Errorf("Expected check to resolve to 'Approve'")
	}
}

type dummyUI struct {
	calls []string
}

func (d *dummyUI) OnInputRequired(info core.UserInputRequest) (core.UserInputResponse, error) {
	d.calls = append(d.calls, "OnInputRequired")
	return core.UserInputResponse{}, nil
}

func (d *dummyUI) ApproveTx(request *core.SignTxRequest) (core.SignTxResponse, error) {
	d.calls = append(d.calls, "ApproveTx")
	return core.SignTxResponse{}, core.ErrRequestDenied
}

func (d *dummyUI) ApproveSignData(request *core.SignDataRequest) (core.SignDataResponse, error) {
	d.calls = append(d.calls, "ApproveSignData")
	return core.SignDataResponse{}, core.ErrRequestDenied
}

func (d *dummyUI) ApproveExport(request *core.ExportRequest) (core.ExportResponse, error) {
	d.calls = append(d.calls, "ApproveExport")
	return core.ExportResponse{}, core.ErrRequestDenied
}

func (d *dummyUI) ApproveImport(request *core.ImportRequest) (core.ImportResponse, error) {
	d.calls = append(d.calls, "ApproveImport")
	return core.ImportResponse{}, core.ErrRequestDenied
}

func (d *dummyUI) ApproveListing(request *core.ListRequest) (core.ListResponse, error) {
	d.calls = append(d.calls, "ApproveListing")
	return core.ListResponse{}, core.ErrRequestDenied
}

func (d *dummyUI) ApproveNewAccount(request *core.NewAccountRequest) (core.NewAccountResponse, error) {
	d.calls = append(d.calls, "ApproveNewAccount")
	return core.NewAccountResponse{}, core.ErrRequestDenied
}

func (d *dummyUI) ShowError(message string) {
	d.calls = append(d.calls, "ShowError")
}

func (d *dummyUI) ShowInfo(message string) {
	d.calls = append(d.calls, "ShowInfo")
}

func (d *dummyUI) OnApprovedTx(tx ethapi.SignTransactionResult) {
	d.calls = append(d.calls, "OnApprovedTx")
}

func (d *dummyUI) OnMasterPassword(request *core.PasswordRequest) (core.PasswordResponse, error) {
	return core.PasswordResponse{}, nil
}

func (d *dummyUI) OnSignerStartup(info core.StartupInfo) {
}

//TestForwarding测试规则引擎是否正确地将请求分派给下一个调用方
func TestForwarding(t *testing.T) {

	js := ""
	ui := &dummyUI{make([]string, 0)}
	jsBackend := storage.NewEphemeralStorage()
	credBackend := storage.NewEphemeralStorage()
	r, err := NewRuleEvaluator(ui, jsBackend, credBackend)
	if err != nil {
		t.Fatalf("Failed to create js engine: %v", err)
	}
	if err = r.Init(js); err != nil {
		t.Fatalf("Failed to load bootstrap js: %v", err)
	}
	r.ApproveSignData(nil)
	r.ApproveTx(nil)
	r.ApproveImport(nil)
	r.ApproveNewAccount(nil)
	r.ApproveListing(nil)
	r.ApproveExport(nil)
	r.ShowError("test")
	r.ShowInfo("test")

//这个没有转发
	r.OnApprovedTx(ethapi.SignTransactionResult{})

	expCalls := 8
	if len(ui.calls) != expCalls {

		t.Errorf("Expected %d forwarded calls, got %d: %s", expCalls, len(ui.calls), strings.Join(ui.calls, ","))

	}

}

func TestMissingFunc(t *testing.T) {
	r, err := initRuleEngine(JS)
	if err != nil {
		t.Errorf("Couldn't create evaluator %v", err)
		return
	}

	_, err = r.execute("MissingMethod", "test")

	if err == nil {
		t.Error("Expected error")
	}

	approved, err := r.checkApproval("MissingMethod", nil, nil)
	if err == nil {
		t.Errorf("Expected missing method to yield error'")
	}
	if approved {
		t.Errorf("Expected missing method to cause non-approval")
	}
	fmt.Printf("Err %v", err)

}
func TestStorage(t *testing.T) {

	js := `
	function testStorage(){
		storage.Put("mykey", "myvalue")
		a = storage.Get("mykey")

storage.Put("mykey", ["a", "list"])  	//应生成“a，list”
		a += storage.Get("mykey")


storage.Put("mykey", {"an": "object"}) 	//应导致“[对象对象]”
		a += storage.Get("mykey")


storage.Put("mykey", JSON.stringify({"an": "object"})) //应导致“”an“：”对象“”
		a += storage.Get("mykey")

a += storage.Get("missingkey")		//缺少键应导致空字符串
storage.Put("","missing key==noop") //无法使用0长度密钥存储
a += storage.Get("")				//应导致“”。

		var b = new BigNumber(2)
var c = new BigNumber(16)//“0xF0”，16）
		var d = b.plus(c)
		console.log(d)
		return a
	}
`
	r, err := initRuleEngine(js)
	if err != nil {
		t.Errorf("Couldn't create evaluator %v", err)
		return
	}

	v, err := r.execute("testStorage", nil)

	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	retval, err := v.ToString()

	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	exp := `myvaluea,list[object Object]{"an":"object"}`
	if retval != exp {
		t.Errorf("Unexpected data, expected '%v', got '%v'", exp, retval)
	}
	fmt.Printf("Err %v", err)

}

const ExampleTxWindow = `
	function big(str){
		if(str.slice(0,2) == "0x"){ return new BigNumber(str.slice(2),16)}
		return new BigNumber(str)
	}

//时间窗口：1周
	var window = 1000* 3600*24*7;

//极限：1醚
	var limit = new BigNumber("1e18");

	function isLimitOk(transaction){
		var value = big(transaction.value)
//启动窗口功能
		var windowstart = new Date().getTime() - window;

		var txs = [];
		var stored = storage.Get('txs');

		if(stored != ""){
			txs = JSON.parse(stored)
		}
//首先，删除时间窗口之外的所有内容
		var newtxs = txs.filter(function(tx){return tx.tstamp > windowstart});
		console.log(txs, newtxs.length);

//第二，合计当前总额
		sum = new BigNumber(0)

		sum = newtxs.reduce(function(agg, tx){ return big(tx.value).plus(agg)}, sum);
		console.log("ApproveTx > Sum so far", sum);
		console.log("ApproveTx > Requested", value.toNumber());

//我们会超过每周限额吗？
		return sum.plus(value).lt(limit)

	}
	function ApproveTx(r){
		console.log(r)
		console.log(typeof(r))
		if (isLimitOk(r.transaction)){
			return "Approve"
		}
		return "Nope"
	}

 /*
 *OnApprovedTx（str）在交易被批准和签署后调用。参数
  *“response_str”包含将发送给外部调用方的返回值。
 *此方法的返回值为ignore-进行此回调的原因是允许
 *用于跟踪已批准交易的规则集。
 *
 *在执行速率限制规则时，应使用此回调。
 *如果规则的响应既不包含“批准”也不包含“拒绝”，则发送将进入手动处理。如果用户
 *然后接受事务，将调用此方法。
 *
 *tldr；使用此方法跟踪已签名的事务，而不是使用approvetx中的数据。
 **/

 	function OnApprovedTx(resp){
		var value = big(resp.tx.value)
		var txs = []
//加载存储的事务
		var stored = storage.Get('txs');
		if(stored != ""){
			txs = JSON.parse(stored)
		}
//将此添加到存储
		txs.push({tstamp: new Date().getTime(), value: value});
		storage.Put("txs", JSON.stringify(txs));
	}

`

func dummyTx(value hexutil.Big) *core.SignTxRequest {

	to, _ := mixAddr("000000000000000000000000000000000000dead")
	from, _ := mixAddr("000000000000000000000000000000000000dead")
	n := hexutil.Uint64(3)
	gas := hexutil.Uint64(21000)
	gasPrice := hexutil.Big(*big.NewInt(2000000))

	return &core.SignTxRequest{
		Transaction: core.SendTxArgs{
			From:     *from,
			To:       to,
			Value:    value,
			Nonce:    n,
			GasPrice: gasPrice,
			Gas:      gas,
		},
		Callinfo: []core.ValidationInfo{
			{Typ: "Warning", Message: "All your base are bellong to us"},
		},
		Meta: core.Metadata{Remote: "remoteip", Local: "localip", Scheme: "inproc"},
	}
}
func dummyTxWithV(value uint64) *core.SignTxRequest {

	v := big.NewInt(0).SetUint64(value)
	h := hexutil.Big(*v)
	return dummyTx(h)
}
func dummySigned(value *big.Int) *types.Transaction {
	to := common.HexToAddress("000000000000000000000000000000000000dead")
	gas := uint64(21000)
	gasPrice := big.NewInt(2000000)
	data := make([]byte, 0)
	return types.NewTransaction(3, to, value, gas, gasPrice, data)

}
func TestLimitWindow(t *testing.T) {

	r, err := initRuleEngine(ExampleTxWindow)
	if err != nil {
		t.Errorf("Couldn't create evaluator %v", err)
		return
	}

//0.3乙醚：429D069189E0000 wei
	v := big.NewInt(0).SetBytes(common.Hex2Bytes("0429D069189E0000"))
	h := hexutil.Big(*v)
//前三个应该成功
	for i := 0; i < 3; i++ {
		unsigned := dummyTx(h)
		resp, err := r.ApproveTx(unsigned)
		if err != nil {
			t.Errorf("Unexpected error %v", err)
		}
		if !resp.Approved {
			t.Errorf("Expected check to resolve to 'Approve'")
		}
//创建虚拟签名事务

		response := ethapi.SignTransactionResult{
			Tx:  dummySigned(v),
			Raw: common.Hex2Bytes("deadbeef"),
		}
		r.OnApprovedTx(response)
	}
//第四个应该失败
	resp, _ := r.ApproveTx(dummyTx(h))
	if resp.Approved {
		t.Errorf("Expected check to resolve to 'Reject'")
	}

}

//dontcallme用作不希望调用的下一个处理程序-它调用测试失败
type dontCallMe struct {
	t *testing.T
}

func (d *dontCallMe) OnInputRequired(info core.UserInputRequest) (core.UserInputResponse, error) {
	d.t.Fatalf("Did not expect next-handler to be called")
	return core.UserInputResponse{}, nil
}

func (d *dontCallMe) OnSignerStartup(info core.StartupInfo) {
}

func (d *dontCallMe) OnMasterPassword(request *core.PasswordRequest) (core.PasswordResponse, error) {
	return core.PasswordResponse{}, nil
}

func (d *dontCallMe) ApproveTx(request *core.SignTxRequest) (core.SignTxResponse, error) {
	d.t.Fatalf("Did not expect next-handler to be called")
	return core.SignTxResponse{}, core.ErrRequestDenied
}

func (d *dontCallMe) ApproveSignData(request *core.SignDataRequest) (core.SignDataResponse, error) {
	d.t.Fatalf("Did not expect next-handler to be called")
	return core.SignDataResponse{}, core.ErrRequestDenied
}

func (d *dontCallMe) ApproveExport(request *core.ExportRequest) (core.ExportResponse, error) {
	d.t.Fatalf("Did not expect next-handler to be called")
	return core.ExportResponse{}, core.ErrRequestDenied
}

func (d *dontCallMe) ApproveImport(request *core.ImportRequest) (core.ImportResponse, error) {
	d.t.Fatalf("Did not expect next-handler to be called")
	return core.ImportResponse{}, core.ErrRequestDenied
}

func (d *dontCallMe) ApproveListing(request *core.ListRequest) (core.ListResponse, error) {
	d.t.Fatalf("Did not expect next-handler to be called")
	return core.ListResponse{}, core.ErrRequestDenied
}

func (d *dontCallMe) ApproveNewAccount(request *core.NewAccountRequest) (core.NewAccountResponse, error) {
	d.t.Fatalf("Did not expect next-handler to be called")
	return core.NewAccountResponse{}, core.ErrRequestDenied
}

func (d *dontCallMe) ShowError(message string) {
	d.t.Fatalf("Did not expect next-handler to be called")
}

func (d *dontCallMe) ShowInfo(message string) {
	d.t.Fatalf("Did not expect next-handler to be called")
}

func (d *dontCallMe) OnApprovedTx(tx ethapi.SignTransactionResult) {
	d.t.Fatalf("Did not expect next-handler to be called")
}

//testcontext清除了规则引擎不在多个请求上保留变量的测试。
//如果是这样，那就不好了，因为开发人员可能会依赖它来存储数据，
//而不是使用基于磁盘的数据存储
func TestContextIsCleared(t *testing.T) {

	js := `
	function ApproveTx(){
		if (typeof foobar == 'undefined') {
			foobar = "Approve"
 		}
		console.log(foobar)
		if (foobar == "Approve"){
			foobar = "Reject"
		}else{
			foobar = "Approve"
		}
		return foobar
	}
	`
	ui := &dontCallMe{t}
	r, err := NewRuleEvaluator(ui, storage.NewEphemeralStorage(), storage.NewEphemeralStorage())
	if err != nil {
		t.Fatalf("Failed to create js engine: %v", err)
	}
	if err = r.Init(js); err != nil {
		t.Fatalf("Failed to load bootstrap js: %v", err)
	}
	tx := dummyTxWithV(0)
	r1, _ := r.ApproveTx(tx)
	r2, _ := r.ApproveTx(tx)
	if r1.Approved != r2.Approved {
		t.Errorf("Expected execution context to be cleared between executions")
	}
}

func TestSignData(t *testing.T) {

	js := `function ApproveListing(){
    return "Approve"
}
function ApproveSignData(r){
    if( r.address.toLowerCase() == "0x694267f14675d7e1b9494fd8d72fefe1755710fa")
    {
        if(r.message.indexOf("bazonk") >= 0){
            return "Approve"
        }
        return "Reject"
    }
//否则进入手动处理
}`
	r, err := initRuleEngine(js)
	if err != nil {
		t.Errorf("Couldn't create evaluator %v", err)
		return
	}
	message := []byte("baz bazonk foo")
	hash, msg := core.SignHash(message)
	raw := hexutil.Bytes(message)
	addr, _ := mixAddr("0x694267f14675d7e1b9494fd8d72fefe1755710fa")

	fmt.Printf("address %v %v\n", addr.String(), addr.Original())
	resp, err := r.ApproveSignData(&core.SignDataRequest{
		Address: *addr,
		Message: msg,
		Hash:    hash,
		Meta:    core.Metadata{Remote: "remoteip", Local: "localip", Scheme: "inproc"},
		Rawdata: raw,
	})
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if !resp.Approved {
		t.Fatalf("Expected approved")
	}
}
