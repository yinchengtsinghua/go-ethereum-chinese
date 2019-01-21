
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

//签名者是一个实用程序，可以用来对事务和
//任意数据。
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/signer/core"
	"github.com/ethereum/go-ethereum/signer/rules"
	"github.com/ethereum/go-ethereum/signer/storage"
	"gopkg.in/urfave/cli.v1"
)

//ExternalApiVersion--请参阅extapi_changelog.md
const ExternalAPIVersion = "4.0.0"

//InternalApiVersion--请参阅intapi_changelog.md
const InternalAPIVersion = "3.0.0"

const legalWarning = `
WARNING! 

Clef is alpha software, and not yet publically released. This software has _not_ been audited, and there
are no guarantees about the workings of this software. It may contain severe flaws. You should not use this software
unless you agree to take full responsibility for doing so, and know what you are doing. 

TLDR; THIS IS NOT PRODUCTION-READY SOFTWARE! 

`

var (
	logLevelFlag = cli.IntFlag{
		Name:  "loglevel",
		Value: 4,
		Usage: "log level to emit to the screen",
	}
	advancedMode = cli.BoolFlag{
		Name:  "advanced",
		Usage: "If enabled, issues warnings instead of rejections for suspicious requests. Default off",
	}
	keystoreFlag = cli.StringFlag{
		Name:  "keystore",
		Value: filepath.Join(node.DefaultDataDir(), "keystore"),
		Usage: "Directory for the keystore",
	}
	configdirFlag = cli.StringFlag{
		Name:  "configdir",
		Value: DefaultConfigDir(),
		Usage: "Directory for Clef configuration",
	}
	rpcPortFlag = cli.IntFlag{
		Name:  "rpcport",
		Usage: "HTTP-RPC server listening port",
		Value: node.DefaultHTTPPort + 5,
	}
	signerSecretFlag = cli.StringFlag{
		Name:  "signersecret",
		Usage: "A file containing the (encrypted) master seed to encrypt Clef data, e.g. keystore credentials and ruleset hash",
	}
	dBFlag = cli.StringFlag{
		Name:  "4bytedb",
		Usage: "File containing 4byte-identifiers",
		Value: "./4byte.json",
	}
	customDBFlag = cli.StringFlag{
		Name:  "4bytedb-custom",
		Usage: "File used for writing new 4byte-identifiers submitted via API",
		Value: "./4byte-custom.json",
	}
	auditLogFlag = cli.StringFlag{
		Name:  "auditlog",
		Usage: "File used to emit audit logs. Set to \"\" to disable",
		Value: "audit.log",
	}
	ruleFlag = cli.StringFlag{
		Name:  "rules",
		Usage: "Enable rule-engine",
		Value: "rules.json",
	}
	stdiouiFlag = cli.BoolFlag{
		Name: "stdio-ui",
		Usage: "Use STDIN/STDOUT as a channel for an external UI. " +
			"This means that an STDIN/STDOUT is used for RPC-communication with a e.g. a graphical user " +
			"interface, and can be used when Clef is started by an external process.",
	}
	testFlag = cli.BoolFlag{
		Name:  "stdio-ui-test",
		Usage: "Mechanism to test interface between Clef and UI. Requires 'stdio-ui'.",
	}
	app         = cli.NewApp()
	initCommand = cli.Command{
		Action:    utils.MigrateFlags(initializeSecrets),
		Name:      "init",
		Usage:     "Initialize the signer, generate secret storage",
		ArgsUsage: "",
		Flags: []cli.Flag{
			logLevelFlag,
			configdirFlag,
		},
		Description: `
The init command generates a master seed which Clef can use to store credentials and data needed for 
the rule-engine to work.`,
	}
	attestCommand = cli.Command{
		Action:    utils.MigrateFlags(attestFile),
		Name:      "attest",
		Usage:     "Attest that a js-file is to be used",
		ArgsUsage: "<sha256sum>",
		Flags: []cli.Flag{
			logLevelFlag,
			configdirFlag,
			signerSecretFlag,
		},
		Description: `
The attest command stores the sha256 of the rule.js-file that you want to use for automatic processing of 
incoming requests. 

Whenever you make an edit to the rule file, you need to use attestation to tell 
Clef that the file is 'safe' to execute.`,
	}

	setCredentialCommand = cli.Command{
		Action:    utils.MigrateFlags(setCredential),
		Name:      "setpw",
		Usage:     "Store a credential for a keystore file",
		ArgsUsage: "<address>",
		Flags: []cli.Flag{
			logLevelFlag,
			configdirFlag,
			signerSecretFlag,
		},
		Description: `
		The setpw command stores a password for a given address (keyfile). If you enter a blank passphrase, it will 
remove any stored credential for that address (keyfile)
`,
	}
)

func init() {
	app.Name = "Clef"
	app.Usage = "Manage Ethereum account operations"
	app.Flags = []cli.Flag{
		logLevelFlag,
		keystoreFlag,
		configdirFlag,
		utils.NetworkIdFlag,
		utils.LightKDFFlag,
		utils.NoUSBFlag,
		utils.RPCListenAddrFlag,
		utils.RPCVirtualHostsFlag,
		utils.IPCDisabledFlag,
		utils.IPCPathFlag,
		utils.RPCEnabledFlag,
		rpcPortFlag,
		signerSecretFlag,
		dBFlag,
		customDBFlag,
		auditLogFlag,
		ruleFlag,
		stdiouiFlag,
		testFlag,
		advancedMode,
	}
	app.Action = signer
	app.Commands = []cli.Command{initCommand, attestCommand, setCredentialCommand}

}
func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initializeSecrets(c *cli.Context) error {
	if err := initialize(c); err != nil {
		return err
	}
	configDir := c.GlobalString(configdirFlag.Name)

	masterSeed := make([]byte, 256)
	num, err := io.ReadFull(rand.Reader, masterSeed)
	if err != nil {
		return err
	}
	if num != len(masterSeed) {
		return fmt.Errorf("failed to read enough random")
	}

	n, p := keystore.StandardScryptN, keystore.StandardScryptP
	if c.GlobalBool(utils.LightKDFFlag.Name) {
		n, p = keystore.LightScryptN, keystore.LightScryptP
	}
	text := "The master seed of clef is locked with a password. Please give a password. Do not forget this password."
	var password string
	for {
		password = getPassPhrase(text, true)
		if err := core.ValidatePasswordFormat(password); err != nil {
			fmt.Printf("invalid password: %v\n", err)
		} else {
			break
		}
	}
	cipherSeed, err := encryptSeed(masterSeed, []byte(password), n, p)
	if err != nil {
		return fmt.Errorf("failed to encrypt master seed: %v", err)
	}

	err = os.Mkdir(configDir, 0700)
	if err != nil && !os.IsExist(err) {
		return err
	}
	location := filepath.Join(configDir, "masterseed.json")
	if _, err := os.Stat(location); err == nil {
		return fmt.Errorf("file %v already exists, will not overwrite", location)
	}
	err = ioutil.WriteFile(location, cipherSeed, 0400)
	if err != nil {
		return err
	}
	fmt.Printf("A master seed has been generated into %s\n", location)
	fmt.Printf(`
This is required to be able to store credentials, such as : 
* Passwords for keystores (used by rule engine)
* Storage for javascript rules
* Hash of rule-file

You should treat that file with utmost secrecy, and make a backup of it. 
NOTE: This file does not contain your accounts. Those need to be backed up separately!

`)
	return nil
}
func attestFile(ctx *cli.Context) error {
	if len(ctx.Args()) < 1 {
		utils.Fatalf("This command requires an argument.")
	}
	if err := initialize(ctx); err != nil {
		return err
	}

	stretchedKey, err := readMasterKey(ctx, nil)
	if err != nil {
		utils.Fatalf(err.Error())
	}
	configDir := ctx.GlobalString(configdirFlag.Name)
	vaultLocation := filepath.Join(configDir, common.Bytes2Hex(crypto.Keccak256([]byte("vault"), stretchedKey)[:10]))
	confKey := crypto.Keccak256([]byte("config"), stretchedKey)

//初始化加密存储
	configStorage := storage.NewAESEncryptedStorage(filepath.Join(vaultLocation, "config.json"), confKey)
	val := ctx.Args().First()
	configStorage.Put("ruleset_sha256", val)
	log.Info("Ruleset attestation updated", "sha256", val)
	return nil
}

func setCredential(ctx *cli.Context) error {
	if len(ctx.Args()) < 1 {
		utils.Fatalf("This command requires an address to be passed as an argument.")
	}
	if err := initialize(ctx); err != nil {
		return err
	}

	address := ctx.Args().First()
	password := getPassPhrase("Enter a passphrase to store with this address.", true)

	stretchedKey, err := readMasterKey(ctx, nil)
	if err != nil {
		utils.Fatalf(err.Error())
	}
	configDir := ctx.GlobalString(configdirFlag.Name)
	vaultLocation := filepath.Join(configDir, common.Bytes2Hex(crypto.Keccak256([]byte("vault"), stretchedKey)[:10]))
	pwkey := crypto.Keccak256([]byte("credentials"), stretchedKey)

//初始化加密存储
	pwStorage := storage.NewAESEncryptedStorage(filepath.Join(vaultLocation, "credentials.json"), pwkey)
	pwStorage.Put(address, password)
	log.Info("Credential store updated", "key", address)
	return nil
}

func initialize(c *cli.Context) error {
//设置记录器以打印所有内容
	logOutput := os.Stdout
	if c.GlobalBool(stdiouiFlag.Name) {
		logOutput = os.Stderr
//如果使用stdioui，则无法执行“确认”流
		fmt.Fprintf(logOutput, legalWarning)
	} else {
		if !confirm(legalWarning) {
			return fmt.Errorf("aborted by user")
		}
	}

	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(c.Int(logLevelFlag.Name)), log.StreamHandler(logOutput, log.TerminalFormat(true))))
	return nil
}

func signer(c *cli.Context) error {
	if err := initialize(c); err != nil {
		return err
	}
	var (
		ui core.SignerUI
	)
	if c.GlobalBool(stdiouiFlag.Name) {
		log.Info("Using stdin/stdout as UI-channel")
		ui = core.NewStdIOUI()
	} else {
		log.Info("Using CLI as UI-channel")
		ui = core.NewCommandlineUI()
	}
	fourByteDb := c.GlobalString(dBFlag.Name)
	fourByteLocal := c.GlobalString(customDBFlag.Name)
	db, err := core.NewAbiDBFromFiles(fourByteDb, fourByteLocal)
	if err != nil {
		utils.Fatalf(err.Error())
	}
	log.Info("Loaded 4byte db", "signatures", db.Size(), "file", fourByteDb, "local", fourByteLocal)

	var (
		api core.ExternalAPI
	)

	configDir := c.GlobalString(configdirFlag.Name)
	if stretchedKey, err := readMasterKey(c, ui); err != nil {
		log.Info("No master seed provided, rules disabled", "error", err)
	} else {

		if err != nil {
			utils.Fatalf(err.Error())
		}
		vaultLocation := filepath.Join(configDir, common.Bytes2Hex(crypto.Keccak256([]byte("vault"), stretchedKey)[:10]))

//生成特定于域的密钥
		pwkey := crypto.Keccak256([]byte("credentials"), stretchedKey)
		jskey := crypto.Keccak256([]byte("jsstorage"), stretchedKey)
		confkey := crypto.Keccak256([]byte("config"), stretchedKey)

//初始化加密存储
		pwStorage := storage.NewAESEncryptedStorage(filepath.Join(vaultLocation, "credentials.json"), pwkey)
		jsStorage := storage.NewAESEncryptedStorage(filepath.Join(vaultLocation, "jsstorage.json"), jskey)
		configStorage := storage.NewAESEncryptedStorage(filepath.Join(vaultLocation, "config.json"), confkey)

//我们有规则文件吗？
		ruleJS, err := ioutil.ReadFile(c.GlobalString(ruleFlag.Name))
		if err != nil {
			log.Info("Could not load rulefile, rules not enabled", "file", "rulefile")
		} else {
			hasher := sha256.New()
			hasher.Write(ruleJS)
			shasum := hasher.Sum(nil)
			storedShasum := configStorage.Get("ruleset_sha256")
			if storedShasum != hex.EncodeToString(shasum) {
				log.Info("Could not validate ruleset hash, rules not enabled", "got", hex.EncodeToString(shasum), "expected", storedShasum)
			} else {
//初始化规则
				ruleEngine, err := rules.NewRuleEvaluator(ui, jsStorage, pwStorage)
				if err != nil {
					utils.Fatalf(err.Error())
				}
				ruleEngine.Init(string(ruleJS))
				ui = ruleEngine
				log.Info("Rule engine configured", "file", c.String(ruleFlag.Name))
			}
		}
	}

	apiImpl := core.NewSignerAPI(
		c.GlobalInt64(utils.NetworkIdFlag.Name),
		c.GlobalString(keystoreFlag.Name),
		c.GlobalBool(utils.NoUSBFlag.Name),
		ui, db,
		c.GlobalBool(utils.LightKDFFlag.Name),
		c.GlobalBool(advancedMode.Name))
	api = apiImpl
//审计日志
	if logfile := c.GlobalString(auditLogFlag.Name); logfile != "" {
		api, err = core.NewAuditLogger(logfile, api)
		if err != nil {
			utils.Fatalf(err.Error())
		}
		log.Info("Audit logs configured", "file", logfile)
	}
//向服务器注册签名者API
	var (
		extapiURL = "n/a"
		ipcapiURL = "n/a"
	)
	rpcAPI := []rpc.API{
		{
			Namespace: "account",
			Public:    true,
			Service:   api,
			Version:   "1.0"},
	}
	if c.GlobalBool(utils.RPCEnabledFlag.Name) {

		vhosts := splitAndTrim(c.GlobalString(utils.RPCVirtualHostsFlag.Name))
		cors := splitAndTrim(c.GlobalString(utils.RPCCORSDomainFlag.Name))

//启动HTTP服务器
		httpEndpoint := fmt.Sprintf("%s:%d", c.GlobalString(utils.RPCListenAddrFlag.Name), c.Int(rpcPortFlag.Name))
		listener, _, err := rpc.StartHTTPEndpoint(httpEndpoint, rpcAPI, []string{"account"}, cors, vhosts, rpc.DefaultHTTPTimeouts)
		if err != nil {
			utils.Fatalf("Could not start RPC api: %v", err)
		}
extapiURL = fmt.Sprintf("http://%s“，httpendpoint）
		log.Info("HTTP endpoint opened", "url", extapiURL)

		defer func() {
			listener.Close()
			log.Info("HTTP endpoint closed", "url", httpEndpoint)
		}()

	}
	if !c.GlobalBool(utils.IPCDisabledFlag.Name) {
		if c.IsSet(utils.IPCPathFlag.Name) {
			ipcapiURL = c.GlobalString(utils.IPCPathFlag.Name)
		} else {
			ipcapiURL = filepath.Join(configDir, "clef.ipc")
		}

		listener, _, err := rpc.StartIPCEndpoint(ipcapiURL, rpcAPI)
		if err != nil {
			utils.Fatalf("Could not start IPC api: %v", err)
		}
		log.Info("IPC endpoint opened", "url", ipcapiURL)
		defer func() {
			listener.Close()
			log.Info("IPC endpoint closed", "url", ipcapiURL)
		}()

	}

	if c.GlobalBool(testFlag.Name) {
		log.Info("Performing UI test")
		go testExternalUI(apiImpl)
	}
	ui.OnSignerStartup(core.StartupInfo{
		Info: map[string]interface{}{
			"extapi_version": ExternalAPIVersion,
			"intapi_version": InternalAPIVersion,
			"extapi_http":    extapiURL,
			"extapi_ipc":     ipcapiURL,
		},
	})

	abortChan := make(chan os.Signal)
	signal.Notify(abortChan, os.Interrupt)

	sig := <-abortChan
	log.Info("Exiting...", "signal", sig)

	return nil
}

//splitandtrim拆分由逗号分隔的输入
//并修剪子字符串中多余的空白。
func splitAndTrim(input string) []string {
	result := strings.Split(input, ",")
	for i, r := range result {
		result[i] = strings.TrimSpace(r)
	}
	return result
}

//defaultconfigdir是用于保管库和其他目录的默认配置目录
//持久性要求。
func DefaultConfigDir() string {
//尝试将数据文件夹放在用户的home目录中
	home := homeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Signer")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "Signer")
		} else {
			return filepath.Join(home, ".clef")
		}
	}
//因为我们无法猜测一个稳定的位置，所以返回空的，稍后再处理
	return ""
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}
func readMasterKey(ctx *cli.Context, ui core.SignerUI) ([]byte, error) {
	var (
		file      string
		configDir = ctx.GlobalString(configdirFlag.Name)
	)
	if ctx.GlobalIsSet(signerSecretFlag.Name) {
		file = ctx.GlobalString(signerSecretFlag.Name)
	} else {
		file = filepath.Join(configDir, "masterseed.json")
	}
	if err := checkFile(file); err != nil {
		return nil, err
	}
	cipherKey, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var password string
//如果ui不是nil，从ui获取密码。
	if ui != nil {
		resp, err := ui.OnInputRequired(core.UserInputRequest{
			Title:      "Master Password",
			Prompt:     "Please enter the password to decrypt the master seed",
			IsPassword: true})
		if err != nil {
			return nil, err
		}
		password = resp.Text
	} else {
		password = getPassPhrase("Decrypt master seed of clef", false)
	}
	masterSeed, err := decryptSeed(cipherKey, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt the master seed of clef")
	}
	if len(masterSeed) < 256 {
		return nil, fmt.Errorf("master seed of insufficient length, expected >255 bytes, got %d", len(masterSeed))
	}

//创建保管库位置
	vaultLocation := filepath.Join(configDir, common.Bytes2Hex(crypto.Keccak256([]byte("vault"), masterSeed)[:10]))
	err = os.Mkdir(vaultLocation, 0700)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	return masterSeed, nil
}

//check file是检查文件
//*存在
//
func checkFile(filename string) error {
	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("failed stat on %s: %v", filename, err)
	}
//检查Unix权限位
	if info.Mode().Perm()&0377 != 0 {
		return fmt.Errorf("file (%v) has insecure file permissions (%v)", filename, info.Mode().String())
	}
	return nil
}

//确认显示文本并请求用户确认
func confirm(text string) bool {
	fmt.Printf(text)
	fmt.Printf("\nEnter 'ok' to proceed:\n>")

	text, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		log.Crit("Failed to read user input", "err", err)
	}

	if text := strings.TrimSpace(text); text == "ok" {
		return true
	}
	return false
}

func testExternalUI(api *core.SignerAPI) {

	ctx := context.WithValue(context.Background(), "remote", "clef binary")
	ctx = context.WithValue(ctx, "scheme", "in-proc")
	ctx = context.WithValue(ctx, "local", "main")

	errs := make([]string, 0)

	api.UI.ShowInfo("Testing 'ShowInfo'")
	api.UI.ShowError("Testing 'ShowError'")

	checkErr := func(method string, err error) {
		if err != nil && err != core.ErrRequestDenied {
			errs = append(errs, fmt.Sprintf("%v: %v", method, err.Error()))
		}
	}
	var err error

	_, err = api.SignTransaction(ctx, core.SendTxArgs{From: common.MixedcaseAddress{}}, nil)
	checkErr("SignTransaction", err)
	_, err = api.Sign(ctx, common.MixedcaseAddress{}, common.Hex2Bytes("01020304"))
	checkErr("Sign", err)
	_, err = api.List(ctx)
	checkErr("List", err)
	_, err = api.New(ctx)
	checkErr("New", err)
	_, err = api.Export(ctx, common.Address{})
	checkErr("Export", err)
	_, err = api.Import(ctx, json.RawMessage{})
	checkErr("Import", err)

	api.UI.ShowInfo("Tests completed")

	if len(errs) > 0 {
		log.Error("Got errors")
		for _, e := range errs {
			log.Error(e)
		}
	} else {
		log.Info("No errors")
	}

}

//
//从预加载的密码短语列表中，或从用户交互式请求。
//TODO:有许多“getpassphrase”函数，最好将它们抽象为一个函数。
func getPassPhrase(prompt string, confirmation bool) string {
	fmt.Println(prompt)
	password, err := console.Stdin.PromptPassword("Passphrase: ")
	if err != nil {
		utils.Fatalf("Failed to read passphrase: %v", err)
	}
	if confirmation {
		confirm, err := console.Stdin.PromptPassword("Repeat passphrase: ")
		if err != nil {
			utils.Fatalf("Failed to read passphrase confirmation: %v", err)
		}
		if password != confirm {
			utils.Fatalf("Passphrases do not match")
		}
	}
	return password
}

type encryptedSeedStorage struct {
	Description string              `json:"description"`
	Version     int                 `json:"version"`
	Params      keystore.CryptoJSON `json:"params"`
}

//encryptseed使用的方案与keystore使用的方案类似，但包装方式不同，
//加密主种子
func encryptSeed(seed []byte, auth []byte, scryptN, scryptP int) ([]byte, error) {
	cryptoStruct, err := keystore.EncryptDataV3(seed, auth, scryptN, scryptP)
	if err != nil {
		return nil, err
	}
	return json.Marshal(&encryptedSeedStorage{"Clef seed", 1, cryptoStruct})
}

//解密种子解密主种子
func decryptSeed(keyjson []byte, auth string) ([]byte, error) {
	var encSeed encryptedSeedStorage
	if err := json.Unmarshal(keyjson, &encSeed); err != nil {
		return nil, err
	}
	if encSeed.Version != 1 {
		log.Warn(fmt.Sprintf("unsupported encryption format of seed: %d, operation will likely fail", encSeed.Version))
	}
	seed, err := keystore.DecryptDataV3(encSeed.Params, auth)
	if err != nil {
		return nil, err
	}
	return seed, err
}

/*


curl-h“content-type:application/json”-x post--data'“jsonrpc”：“2.0”，“method”：“account_new”，“params”：[“test”]，“id”：67“localhost:8550”

//列出帐户

curl-i-h“内容类型：application/json”-x post--data'“jsonrpc”：“2.0”，“method”：“account_list”，“params”：[“”]，“id”：67”http://localhost:8550/


//安全端（0x12）
//4401A6E4000000000000000000000000000000000000000000000000000000000012

/供给ABI
curl-i-h“content-type:application/json”-x post--data'“jsonrpc”：“2.0”，“method”：“account-signtransaction”，“params”：[“from”：“0x82A2A876D39022B3019932D30CD9C97AD5616813”，“gas”：“0x333”，“gasprice”：“0x123”，“nonce”：“0x 0”，“to”：“0x07A565B7ED7D7A68680A4C162885BEDB695FE0”，“value”：“0x10”，“data”：“0x4401A6E400000000000000000000000000000000000000000”000000000000000000 12“”，“测试”]，“ID”：67“http://localhost:8550/

/不提供
curl-i-h“content-type:application/json”-x post--data'“jsonrpc”：“2.0”，“method”：“account-signtransaction”，“params”：[“from”：“0x82A2A876D39022B3019932D30CD9C97AD5616813”，“gas”：“0x333”，“gasprice”：“0x123”，“nonce”：“0x 0”，“to”：“0x07A565B7ED7D7A68680A4C162885BEDB695FE0”，“value”：“0x10”，“data”：“0x4401A6E400000000000000000000000000000000000000000”000000000000000000 12“”，“id”：67”http://localhost:8550/

/符号数据

curl-i-h“内容类型：application/json”-x post--数据'“jsonrpc”：“2.0”，“方法”：“帐户符号”，“参数”：[“0x694267f14675d7e1b9494fd8d72fe1755710fa”，“bazonk gaz baz”]，“id”：67“http://localhost:8550/


*/

