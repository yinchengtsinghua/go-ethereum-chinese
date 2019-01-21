
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
package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rlp"
)

//用于测试
type HeadlessUI struct {
	controller chan string
}

func (ui *HeadlessUI) OnInputRequired(info UserInputRequest) (UserInputResponse, error) {
	return UserInputResponse{}, errors.New("not implemented")
}

func (ui *HeadlessUI) OnSignerStartup(info StartupInfo) {
}

func (ui *HeadlessUI) OnApprovedTx(tx ethapi.SignTransactionResult) {
	fmt.Printf("OnApproved()\n")
}

func (ui *HeadlessUI) ApproveTx(request *SignTxRequest) (SignTxResponse, error) {

	switch <-ui.controller {
	case "Y":
		return SignTxResponse{request.Transaction, true, <-ui.controller}, nil
case "M": //修改
		old := big.Int(request.Transaction.Value)
		newVal := big.NewInt(0).Add(&old, big.NewInt(1))
		request.Transaction.Value = hexutil.Big(*newVal)
		return SignTxResponse{request.Transaction, true, <-ui.controller}, nil
	default:
		return SignTxResponse{request.Transaction, false, ""}, nil
	}
}

func (ui *HeadlessUI) ApproveSignData(request *SignDataRequest) (SignDataResponse, error) {
	if "Y" == <-ui.controller {
		return SignDataResponse{true, <-ui.controller}, nil
	}
	return SignDataResponse{false, ""}, nil
}

func (ui *HeadlessUI) ApproveExport(request *ExportRequest) (ExportResponse, error) {
	return ExportResponse{<-ui.controller == "Y"}, nil

}

func (ui *HeadlessUI) ApproveImport(request *ImportRequest) (ImportResponse, error) {
	if "Y" == <-ui.controller {
		return ImportResponse{true, <-ui.controller, <-ui.controller}, nil
	}
	return ImportResponse{false, "", ""}, nil
}

func (ui *HeadlessUI) ApproveListing(request *ListRequest) (ListResponse, error) {
	switch <-ui.controller {
	case "A":
		return ListResponse{request.Accounts}, nil
	case "1":
		l := make([]Account, 1)
		l[0] = request.Accounts[1]
		return ListResponse{l}, nil
	default:
		return ListResponse{nil}, nil
	}
}

func (ui *HeadlessUI) ApproveNewAccount(request *NewAccountRequest) (NewAccountResponse, error) {
	if "Y" == <-ui.controller {
		return NewAccountResponse{true, <-ui.controller}, nil
	}
	return NewAccountResponse{false, ""}, nil
}

func (ui *HeadlessUI) ShowError(message string) {
//stdout用于通信
	fmt.Fprintln(os.Stderr, message)
}

func (ui *HeadlessUI) ShowInfo(message string) {
//stdout用于通信
	fmt.Fprintln(os.Stderr, message)
}

func tmpDirName(t *testing.T) string {
	d, err := ioutil.TempDir("", "eth-keystore-test")
	if err != nil {
		t.Fatal(err)
	}
	d, err = filepath.EvalSymlinks(d)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func setup(t *testing.T) (*SignerAPI, chan string) {

	controller := make(chan string, 20)

	db, err := NewAbiDBFromFile("../../cmd/clef/4byte.json")
	if err != nil {
		utils.Fatalf(err.Error())
	}
	var (
		ui  = &HeadlessUI{controller}
		api = NewSignerAPI(
			1,
			tmpDirName(t),
			true,
			ui,
			db,
			true, true)
	)
	return api, controller
}
func createAccount(control chan string, api *SignerAPI, t *testing.T) {

	control <- "Y"
	control <- "a_long_password"
	_, err := api.New(context.Background())
	if err != nil {
		t.Fatal(err)
	}
//允许传播更改的一段时间
	time.Sleep(250 * time.Millisecond)
}

func failCreateAccountWithPassword(control chan string, api *SignerAPI, password string, t *testing.T) {

	control <- "Y"
	control <- password
	control <- "Y"
	control <- password
	control <- "Y"
	control <- password

	acc, err := api.New(context.Background())
	if err == nil {
		t.Fatal("Should have returned an error")
	}
	if acc.Address != (common.Address{}) {
		t.Fatal("Empty address should be returned")
	}
}

func failCreateAccount(control chan string, api *SignerAPI, t *testing.T) {
	control <- "N"
	acc, err := api.New(context.Background())
	if err != ErrRequestDenied {
		t.Fatal(err)
	}
	if acc.Address != (common.Address{}) {
		t.Fatal("Empty address should be returned")
	}
}

func list(control chan string, api *SignerAPI, t *testing.T) []common.Address {
	control <- "A"
	list, err := api.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func TestNewAcc(t *testing.T) {
	api, control := setup(t)
	verifyNum := func(num int) {
		if list := list(control, api, t); len(list) != num {
			t.Errorf("Expected %d accounts, got %d", num, len(list))
		}
	}
//测试创建和创建拒绝
	createAccount(control, api, t)
	createAccount(control, api, t)
	failCreateAccount(control, api, t)
	failCreateAccount(control, api, t)
	createAccount(control, api, t)
	failCreateAccount(control, api, t)
	createAccount(control, api, t)
	failCreateAccount(control, api, t)

	verifyNum(4)

//由于密码错误，无法创建此文件
	failCreateAccountWithPassword(control, api, "short", t)
	failCreateAccountWithPassword(control, api, "longerbutbad\rfoo", t)

	verifyNum(4)

//测试列表：
//列出一个帐户
	control <- "1"
	list, err := api.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("List should only show one Account")
	}
//拒绝上市
	control <- "Nope"
	list, err = api.List(context.Background())
	if len(list) != 0 {
		t.Fatalf("List should be empty")
	}
	if err != ErrRequestDenied {
		t.Fatal("Expected deny")
	}
}

func TestSignData(t *testing.T) {
	api, control := setup(t)
//创建两个帐户
	createAccount(control, api, t)
	createAccount(control, api, t)
	control <- "1"
	list, err := api.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	a := common.NewMixedcaseAddress(list[0])

	control <- "Y"
	control <- "wrongpassword"
	h, err := api.Sign(context.Background(), a, []byte("EHLO world"))
	if h != nil {
		t.Errorf("Expected nil-data, got %x", h)
	}
	if err != keystore.ErrDecrypt {
		t.Errorf("Expected ErrLocked! %v", err)
	}
	control <- "No way"
	h, err = api.Sign(context.Background(), a, []byte("EHLO world"))
	if h != nil {
		t.Errorf("Expected nil-data, got %x", h)
	}
	if err != ErrRequestDenied {
		t.Errorf("Expected ErrRequestDenied! %v", err)
	}
	control <- "Y"
	control <- "a_long_password"
	h, err = api.Sign(context.Background(), a, []byte("EHLO world"))
	if err != nil {
		t.Fatal(err)
	}
	if h == nil || len(h) != 65 {
		t.Errorf("Expected 65 byte signature (got %d bytes)", len(h))
	}
}
func mkTestTx(from common.MixedcaseAddress) SendTxArgs {
	to := common.NewMixedcaseAddress(common.HexToAddress("0x1337"))
	gas := hexutil.Uint64(21000)
	gasPrice := (hexutil.Big)(*big.NewInt(2000000000))
	value := (hexutil.Big)(*big.NewInt(1e18))
	nonce := (hexutil.Uint64)(0)
	data := hexutil.Bytes(common.Hex2Bytes("01020304050607080a"))
	tx := SendTxArgs{
		From:     from,
		To:       &to,
		Gas:      gas,
		GasPrice: gasPrice,
		Value:    value,
		Data:     &data,
		Nonce:    nonce}
	return tx
}

func TestSignTx(t *testing.T) {
	var (
		list      []common.Address
		res, res2 *ethapi.SignTransactionResult
		err       error
	)

	api, control := setup(t)
	createAccount(control, api, t)
	control <- "A"
	list, err = api.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	a := common.NewMixedcaseAddress(list[0])

	methodSig := "test(uint)"
	tx := mkTestTx(a)

	control <- "Y"
	control <- "wrongpassword"
	res, err = api.SignTransaction(context.Background(), tx, &methodSig)
	if res != nil {
		t.Errorf("Expected nil-response, got %v", res)
	}
	if err != keystore.ErrDecrypt {
		t.Errorf("Expected ErrLocked! %v", err)
	}
	control <- "No way"
	res, err = api.SignTransaction(context.Background(), tx, &methodSig)
	if res != nil {
		t.Errorf("Expected nil-response, got %v", res)
	}
	if err != ErrRequestDenied {
		t.Errorf("Expected ErrRequestDenied! %v", err)
	}
	control <- "Y"
	control <- "a_long_password"
	res, err = api.SignTransaction(context.Background(), tx, &methodSig)

	if err != nil {
		t.Fatal(err)
	}
	parsedTx := &types.Transaction{}
	rlp.Decode(bytes.NewReader(res.Raw), parsedTx)

//用户界面不应修改Tx
	if parsedTx.Value().Cmp(tx.Value.ToInt()) != 0 {
		t.Errorf("Expected value to be unchanged, expected %v got %v", tx.Value, parsedTx.Value())
	}
	control <- "Y"
	control <- "a_long_password"

	res2, err = api.SignTransaction(context.Background(), tx, &methodSig)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(res.Raw, res2.Raw) {
		t.Error("Expected tx to be unmodified by UI")
	}

//Tx由用户界面修改
	control <- "M"
	control <- "a_long_password"

	res2, err = api.SignTransaction(context.Background(), tx, &methodSig)
	if err != nil {
		t.Fatal(err)
	}
	parsedTx2 := &types.Transaction{}
	rlp.Decode(bytes.NewReader(res.Raw), parsedTx2)

//用户界面应修改Tx
	if parsedTx2.Value().Cmp(tx.Value.ToInt()) != 0 {
		t.Errorf("Expected value to be unchanged, got %v", parsedTx.Value())
	}
	if bytes.Equal(res.Raw, res2.Raw) {
		t.Error("Expected tx to be modified by UI")
	}

}

/*
func测试同步响应（t*testing.t）

 //设置一个帐户
 API，控件：=设置（T）
 创建帐户（控件、API、T）

 //两个事务，第二个事务的值大于第一个事务的值
 tx1：=mktestx（）。
 newval：=big.newint（0）.add（（*big.int）（tx1.value），big.newint（1））
 tx2：=mktestx（）。
 tx2.value=（*hexUtil.big）（newval）

 控件<-“w”//等待
 控制<-“Y”//
 控件<-“a_long_password”
 控制<-“Y”//
 控件<-“a_long_password”

 无功误差

 h1，err：=api.signTransaction（context.background（），common.hextoAddress（“1111”），tx1，nil）
 h2，错误：=api.signTransaction（context.background（），common.hextoAddress（“2222”），tx2，nil）


 }
**/

