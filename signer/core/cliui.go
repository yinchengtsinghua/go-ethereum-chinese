
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

package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"sync"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/crypto/ssh/terminal"
)

type CommandlineUI struct {
	in *bufio.Reader
	mu sync.Mutex
}

func NewCommandlineUI() *CommandlineUI {
	return &CommandlineUI{in: bufio.NewReader(os.Stdin)}
}

//readstring从stdin读取一行，从空格中剪裁if，强制
//非空性。
func (ui *CommandlineUI) readString() string {
	for {
		fmt.Printf("> ")
		text, err := ui.in.ReadString('\n')
		if err != nil {
			log.Crit("Failed to read user input", "err", err)
		}
		if text = strings.TrimSpace(text); text != "" {
			return text
		}
	}
}

//readpassword从stdin读取一行，从尾随的new
//行并返回它。输入将不会被回送。
func (ui *CommandlineUI) readPassword() string {
	fmt.Printf("Enter password to approve:\n")
	fmt.Printf("> ")

	text, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		log.Crit("Failed to read password", "err", err)
	}
	fmt.Println()
	fmt.Println("-----------------------")
	return string(text)
}

//readpassword从stdin读取一行，从尾随的new
//行并返回它。输入将不会被回送。
func (ui *CommandlineUI) readPasswordText(inputstring string) string {
	fmt.Printf("Enter %s:\n", inputstring)
	fmt.Printf("> ")
	text, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		log.Crit("Failed to read password", "err", err)
	}
	fmt.Println("-----------------------")
	return string(text)
}

func (ui *CommandlineUI) OnInputRequired(info UserInputRequest) (UserInputResponse, error) {
	fmt.Println(info.Title)
	fmt.Println(info.Prompt)
	if info.IsPassword {
		text, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			log.Error("Failed to read password", "err", err)
		}
		fmt.Println("-----------------------")
		return UserInputResponse{string(text)}, err
	}
	text := ui.readString()
	fmt.Println("-----------------------")
	return UserInputResponse{text}, nil
}

//如果用户输入“是”，则confirm返回true，否则返回false
func (ui *CommandlineUI) confirm() bool {
	fmt.Printf("Approve? [y/N]:\n")
	if ui.readString() == "y" {
		return true
	}
	fmt.Println("-----------------------")
	return false
}

func showMetadata(metadata Metadata) {
	fmt.Printf("Request context:\n\t%v -> %v -> %v\n", metadata.Remote, metadata.Scheme, metadata.Local)
	fmt.Printf("\nAdditional HTTP header data, provided by the external caller:\n")
	fmt.Printf("\tUser-Agent: %v\n\tOrigin: %v\n", metadata.UserAgent, metadata.Origin)
}

//approvetx提示用户确认请求签署交易
func (ui *CommandlineUI) ApproveTx(request *SignTxRequest) (SignTxResponse, error) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	weival := request.Transaction.Value.ToInt()
	fmt.Printf("--------- Transaction request-------------\n")
	if to := request.Transaction.To; to != nil {
		fmt.Printf("to:    %v\n", to.Original())
		if !to.ValidChecksum() {
			fmt.Printf("\nWARNING: Invalid checksum on to-address!\n\n")
		}
	} else {
		fmt.Printf("to:    <contact creation>\n")
	}
	fmt.Printf("from:     %v\n", request.Transaction.From.String())
	fmt.Printf("value:    %v wei\n", weival)
	fmt.Printf("gas:      %v (%v)\n", request.Transaction.Gas, uint64(request.Transaction.Gas))
	fmt.Printf("gasprice: %v wei\n", request.Transaction.GasPrice.ToInt())
	fmt.Printf("nonce:    %v (%v)\n", request.Transaction.Nonce, uint64(request.Transaction.Nonce))
	if request.Transaction.Data != nil {
		d := *request.Transaction.Data
		if len(d) > 0 {

			fmt.Printf("data:     %v\n", hexutil.Encode(d))
		}
	}
	if request.Callinfo != nil {
		fmt.Printf("\nTransaction validation:\n")
		for _, m := range request.Callinfo {
			fmt.Printf("  * %s : %s\n", m.Typ, m.Message)
		}
		fmt.Println()

	}
	fmt.Printf("\n")
	showMetadata(request.Meta)
	fmt.Printf("-------------------------------------------\n")
	if !ui.confirm() {
		return SignTxResponse{request.Transaction, false, ""}, nil
	}
	return SignTxResponse{request.Transaction, true, ui.readPassword()}, nil
}

//ApproveSignData提示用户确认请求签署数据
func (ui *CommandlineUI) ApproveSignData(request *SignDataRequest) (SignDataResponse, error) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	fmt.Printf("-------- Sign data request--------------\n")
	fmt.Printf("Account:  %s\n", request.Address.String())
	fmt.Printf("message:  \n%q\n", request.Message)
	fmt.Printf("raw data: \n%v\n", request.Rawdata)
	fmt.Printf("message hash:  %v\n", request.Hash)
	fmt.Printf("-------------------------------------------\n")
	showMetadata(request.Meta)
	if !ui.confirm() {
		return SignDataResponse{false, ""}, nil
	}
	return SignDataResponse{true, ui.readPassword()}, nil
}

//approveexport提示用户确认导出加密帐户json
func (ui *CommandlineUI) ApproveExport(request *ExportRequest) (ExportResponse, error) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	fmt.Printf("-------- Export Account request--------------\n")
	fmt.Printf("A request has been made to export the (encrypted) keyfile\n")
	fmt.Printf("Approving this operation means that the caller obtains the (encrypted) contents\n")
	fmt.Printf("\n")
	fmt.Printf("Account:  %x\n", request.Address)
//fmt.printf（“keyfile:\n%v\n”，request.file）
	fmt.Printf("-------------------------------------------\n")
	showMetadata(request.Meta)
	return ExportResponse{ui.confirm()}, nil
}

//approveImport提示用户确认导入账号json
func (ui *CommandlineUI) ApproveImport(request *ImportRequest) (ImportResponse, error) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	fmt.Printf("-------- Import Account request--------------\n")
	fmt.Printf("A request has been made to import an encrypted keyfile\n")
	fmt.Printf("-------------------------------------------\n")
	showMetadata(request.Meta)
	if !ui.confirm() {
		return ImportResponse{false, "", ""}, nil
	}
	return ImportResponse{true, ui.readPasswordText("Old password"), ui.readPasswordText("New password")}, nil
}

//批准提示用户确认列出帐户
//用户界面可以修改要列出的科目列表
func (ui *CommandlineUI) ApproveListing(request *ListRequest) (ListResponse, error) {

	ui.mu.Lock()
	defer ui.mu.Unlock()

	fmt.Printf("-------- List Account request--------------\n")
	fmt.Printf("A request has been made to list all accounts. \n")
	fmt.Printf("You can select which accounts the caller can see\n")
	for _, account := range request.Accounts {
		fmt.Printf("  [x] %v\n", account.Address.Hex())
		fmt.Printf("    URL: %v\n", account.URL)
		fmt.Printf("    Type: %v\n", account.Typ)
	}
	fmt.Printf("-------------------------------------------\n")
	showMetadata(request.Meta)
	if !ui.confirm() {
		return ListResponse{nil}, nil
	}
	return ListResponse{request.Accounts}, nil
}

//ApproveWaccount提示用户确认创建新帐户，并显示给调用方
func (ui *CommandlineUI) ApproveNewAccount(request *NewAccountRequest) (NewAccountResponse, error) {

	ui.mu.Lock()
	defer ui.mu.Unlock()

	fmt.Printf("-------- New Account request--------------\n\n")
	fmt.Printf("A request has been made to create a new account. \n")
	fmt.Printf("Approving this operation means that a new account is created,\n")
	fmt.Printf("and the address is returned to the external caller\n\n")
	showMetadata(request.Meta)
	if !ui.confirm() {
		return NewAccountResponse{false, ""}, nil
	}
	return NewAccountResponse{true, ui.readPassword()}, nil
}

//ShowError向用户显示错误消息
func (ui *CommandlineUI) ShowError(message string) {
	fmt.Printf("-------- Error message from Clef-----------\n")
	fmt.Println(message)
	fmt.Printf("-------------------------------------------\n")
}

//ShowInfo向用户显示信息消息
func (ui *CommandlineUI) ShowInfo(message string) {
	fmt.Printf("Info: %v\n", message)
}

func (ui *CommandlineUI) OnApprovedTx(tx ethapi.SignTransactionResult) {
	fmt.Printf("Transaction signed:\n ")
	spew.Dump(tx.Tx)
}

func (ui *CommandlineUI) OnSignerStartup(info StartupInfo) {

	fmt.Printf("------- Signer info -------\n")
	for k, v := range info.Info {
		fmt.Printf("* %v : %v\n", k, v)
	}
}
