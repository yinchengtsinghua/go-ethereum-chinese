
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/debug"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/swarm"
	bzzapi "github.com/ethereum/go-ethereum/swarm/api"
	swarmmetrics "github.com/ethereum/go-ethereum/swarm/metrics"
	"github.com/ethereum/go-ethereum/swarm/tracing"
	sv "github.com/ethereum/go-ethereum/swarm/version"

	"gopkg.in/urfave/cli.v1"
)

const clientIdentifier = "swarm"
const helpTemplate = `NAME:
{{.HelpName}} - {{.Usage}}

USAGE:
{{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}}{{if .VisibleFlags}} [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}{{if .Category}}

CATEGORY:
{{.Category}}{{end}}{{if .Description}}

DESCRIPTION:
{{.Description}}{{end}}{{if .VisibleFlags}}

OPTIONS:
{{range .VisibleFlags}}{{.}}
{{end}}{{end}}
`

var (
gitCommit string //git sha1提交发布的哈希（通过链接器标志设置）
)

//声明几个常量错误消息，对以后测试中的错误检查比较很有用
var (
	SWARM_ERR_NO_BZZACCOUNT   = "bzzaccount option is required but not set; check your config file, command line or environment variables"
	SWARM_ERR_SWAP_SET_NO_API = "SWAP is enabled but --swap-api is not set"
)

//此帮助命令将添加到未显式定义它的任何子命令中
var defaultSubcommandHelp = cli.Command{
	Action:             func(ctx *cli.Context) { cli.ShowCommandHelpAndExit(ctx, "", 1) },
	CustomHelpTemplate: helpTemplate,
	Name:               "help",
	Usage:              "shows this help",
	Hidden:             true,
}

var defaultNodeConfig = node.DefaultConfig

//这个init函数设置默认值，以便cmd/swarm可以与geth一起运行。
func init() {
	defaultNodeConfig.Name = clientIdentifier
	defaultNodeConfig.Version = sv.VersionWithCommit(gitCommit)
	defaultNodeConfig.P2P.ListenAddr = ":30399"
	defaultNodeConfig.IPCPath = "bzzd.ipc"
//设置--help显示的标志默认值。
	utils.ListenPortFlag.Value = 30399
}

var app = utils.NewApp("", "Ethereum Swarm")

//这个init函数创建cli.app。
func init() {
	app.Action = bzzd
	app.Version = sv.ArchiveVersion(gitCommit)
	app.Copyright = "Copyright 2013-2016 The go-ethereum Authors"
	app.Commands = []cli.Command{
		{
			Action:             version,
			CustomHelpTemplate: helpTemplate,
			Name:               "version",
			Usage:              "Print version numbers",
			Description:        "The output of this command is supposed to be machine-readable",
		},
		{
			Action:             keys,
			CustomHelpTemplate: helpTemplate,
			Name:               "print-keys",
			Flags:              []cli.Flag{SwarmCompressedFlag},
			Usage:              "Print public key information",
			Description:        "The output of this command is supposed to be machine-readable",
		},
//看上传
		upCommand,
//参见Access
		accessCommand,
//见饲料
		feedCommand,
//参见List.Go
		listCommand,
//看HAH.GO
		hashCommand,
//参见下载
		downloadCommand,
//见宣言
		manifestCommand,
//见鬼去吧
		fsCommand,
//参见D.Go
		dbCommand,
//参见CONG.GO
		DumpConfigCommand,
	}

//将隐藏的帮助子命令附加到具有子命令的所有命令
//如果上面已经定义了帮助命令，则该命令将优先。
	addDefaultHelpSubcommands(app.Commands)

	sort.Sort(cli.CommandsByName(app.Commands))

	app.Flags = []cli.Flag{
		utils.IdentityFlag,
		utils.DataDirFlag,
		utils.BootnodesFlag,
		utils.KeyStoreDirFlag,
		utils.ListenPortFlag,
		utils.NoDiscoverFlag,
		utils.DiscoveryV5Flag,
		utils.NetrestrictFlag,
		utils.NodeKeyFileFlag,
		utils.NodeKeyHexFlag,
		utils.MaxPeersFlag,
		utils.NATFlag,
		utils.IPCDisabledFlag,
		utils.IPCPathFlag,
		utils.PasswordFileFlag,
//BZZD特定标志
		CorsStringFlag,
		EnsAPIFlag,
		SwarmTomlConfigPathFlag,
		SwarmSwapEnabledFlag,
		SwarmSwapAPIFlag,
		SwarmSyncDisabledFlag,
		SwarmSyncUpdateDelay,
		SwarmMaxStreamPeerServersFlag,
		SwarmLightNodeEnabled,
		SwarmDeliverySkipCheckFlag,
		SwarmListenAddrFlag,
		SwarmPortFlag,
		SwarmAccountFlag,
		SwarmNetworkIdFlag,
		ChequebookAddrFlag,
//上传标志
		SwarmApiFlag,
		SwarmRecursiveFlag,
		SwarmWantManifestFlag,
		SwarmUploadDefaultPath,
		SwarmUpFromStdinFlag,
		SwarmUploadMimeType,
//存储标志
		SwarmStorePath,
		SwarmStoreCapacity,
		SwarmStoreCacheCapacity,
	}
	rpcFlags := []cli.Flag{
		utils.WSEnabledFlag,
		utils.WSListenAddrFlag,
		utils.WSPortFlag,
		utils.WSApiFlag,
		utils.WSAllowedOriginsFlag,
	}
	app.Flags = append(app.Flags, rpcFlags...)
	app.Flags = append(app.Flags, debug.Flags...)
	app.Flags = append(app.Flags, swarmmetrics.Flags...)
	app.Flags = append(app.Flags, tracing.Flags...)
	app.Before = func(ctx *cli.Context) error {
		runtime.GOMAXPROCS(runtime.NumCPU())
		if err := debug.Setup(ctx, ""); err != nil {
			return err
		}
		swarmmetrics.Setup(ctx)
		tracing.Setup(ctx)
		return nil
	}
	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		return nil
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func keys(ctx *cli.Context) error {
	privateKey := getPrivKey(ctx)
	pub := hex.EncodeToString(crypto.FromECDSAPub(&privateKey.PublicKey))
	pubCompressed := hex.EncodeToString(crypto.CompressPubkey(&privateKey.PublicKey))
	if !ctx.Bool(SwarmCompressedFlag.Name) {
		fmt.Println(fmt.Sprintf("publicKey=%s", pub))
	}
	fmt.Println(fmt.Sprintf("publicKeyCompressed=%s", pubCompressed))
	return nil
}

func version(ctx *cli.Context) error {
	fmt.Println(strings.Title(clientIdentifier))
	fmt.Println("Version:", sv.VersionWithMeta)
	if gitCommit != "" {
		fmt.Println("Git Commit:", gitCommit)
	}
	fmt.Println("Go Version:", runtime.Version())
	fmt.Println("OS:", runtime.GOOS)
	return nil
}

func bzzd(ctx *cli.Context) error {
//从所有可用源生成有效的bzzapi.config:
//默认配置、文件配置、命令行和env vars

	bzzconfig, err := buildConfig(ctx)
	if err != nil {
		utils.Fatalf("unable to configure swarm: %v", err)
	}

	cfg := defaultNodeConfig

//PSS在WS上运行
	cfg.WSModules = append(cfg.WSModules, "pss")

//geth只支持--datadir通过命令行
//为了在Swarm中保持一致，如果我们通过环境变量传递--datadir
//
	if _, err := os.Stat(bzzconfig.Path); err == nil {
		cfg.DataDir = bzzconfig.Path
	}

//可以选择在配置节点之前设置引导节点
	setSwarmBootstrapNodes(ctx, &cfg)
//设置以太坊节点
	utils.SetNodeConfig(ctx, &cfg)
	stack, err := node.New(&cfg)
	if err != nil {
		utils.Fatalf("can't create node: %v", err)
	}

//配置阶段完成后，需要执行一些步骤，
//由于覆盖行为
	initSwarmNode(bzzconfig, stack, ctx)
//在以太坊节点中将bzz注册为node.service
	registerBzzService(bzzconfig, stack)
//启动节点
	utils.StartNode(stack)

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGTERM)
		defer signal.Stop(sigc)
		<-sigc
		log.Info("Got sigterm, shutting swarm down...")
		stack.Stop()
	}()

	stack.Wait()
	return nil
}

func registerBzzService(bzzconfig *bzzapi.Config, stack *node.Node) {
//定义Swarm服务引导功能
	boot := func(_ *node.ServiceContext) (node.Service, error) {
//在生产中，mockstore必须始终为零。
		return swarm.NewSwarm(bzzconfig, nil)
	}
//在以太坊节点中注册
	if err := stack.Register(boot); err != nil {
		utils.Fatalf("Failed to register the Swarm service: %v", err)
	}
}

func getAccount(bzzaccount string, ctx *cli.Context, stack *node.Node) *ecdsa.PrivateKey {
//帐户是必需的
	if bzzaccount == "" {
		utils.Fatalf(SWARM_ERR_NO_BZZACCOUNT)
	}
//尝试将arg作为十六进制密钥文件加载。
	if key, err := crypto.LoadECDSA(bzzaccount); err == nil {
		log.Info("Swarm account key loaded", "address", crypto.PubkeyToAddress(key.PublicKey))
		return key
	}
//
	am := stack.AccountManager()
	ks := am.Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)

	return decryptStoreAccount(ks, bzzaccount, utils.MakePasswordList(ctx))
}

//getprivkey返回指定bzzaccount的私钥
//
func getPrivKey(ctx *cli.Context) *ecdsa.PrivateKey {
//像在bzzd操作中一样启动群节点
	bzzconfig, err := buildConfig(ctx)
	if err != nil {
		utils.Fatalf("unable to configure swarm: %v", err)
	}
	cfg := defaultNodeConfig
	if _, err := os.Stat(bzzconfig.Path); err == nil {
		cfg.DataDir = bzzconfig.Path
	}
	utils.SetNodeConfig(ctx, &cfg)
	stack, err := node.New(&cfg)
	if err != nil {
		utils.Fatalf("can't create node: %v", err)
	}
	return getAccount(bzzconfig.BzzAccount, ctx, stack)
}

func decryptStoreAccount(ks *keystore.KeyStore, account string, passwords []string) *ecdsa.PrivateKey {
	var a accounts.Account
	var err error
	if common.IsHexAddress(account) {
		a, err = ks.Find(accounts.Account{Address: common.HexToAddress(account)})
	} else if ix, ixerr := strconv.Atoi(account); ixerr == nil && ix > 0 {
		if accounts := ks.Accounts(); len(accounts) > ix {
			a = accounts[ix]
		} else {
			err = fmt.Errorf("index %d higher than number of accounts %d", ix, len(accounts))
		}
	} else {
		utils.Fatalf("Can't find swarm account key %s", account)
	}
	if err != nil {
		utils.Fatalf("Can't find swarm account key: %v - Is the provided bzzaccount(%s) from the right datadir/Path?", err, account)
	}
	keyjson, err := ioutil.ReadFile(a.URL.Path)
	if err != nil {
		utils.Fatalf("Can't load swarm account key: %v", err)
	}
	for i := 0; i < 3; i++ {
		password := getPassPhrase(fmt.Sprintf("Unlocking swarm account %s [%d/3]", a.Address.Hex(), i+1), i, passwords)
		key, err := keystore.DecryptKey(keyjson, password)
		if err == nil {
			return key.PrivateKey
		}
	}
	utils.Fatalf("Can't decrypt swarm account key")
	return nil
}

//getpassphrase通过获取与bzz帐户关联的密码
//从预先加载的密码列表中，或通过交互方式向用户请求密码。
func getPassPhrase(prompt string, i int, passwords []string) string {
//非交互式
	if len(passwords) > 0 {
		if i < len(passwords) {
			return passwords[i]
		}
		return passwords[len(passwords)-1]
	}

//回退到交互模式
	if prompt != "" {
		fmt.Println(prompt)
	}
	password, err := console.Stdin.PromptPassword("Passphrase: ")
	if err != nil {
		utils.Fatalf("Failed to read passphrase: %v", err)
	}
	return password
}

//adddefaulthelpsubcommand通过定义的cli命令扫描并添加
//每个的基本帮助子命令
//
func addDefaultHelpSubcommands(commands []cli.Command) {
	for i := range commands {
		cmd := &commands[i]
		if cmd.Subcommands != nil {
			cmd.Subcommands = append(cmd.Subcommands, defaultSubcommandHelp)
			addDefaultHelpSubcommands(cmd.Subcommands)
		}
	}
}

func setSwarmBootstrapNodes(ctx *cli.Context, cfg *node.Config) {
	if ctx.GlobalIsSet(utils.BootnodesFlag.Name) || ctx.GlobalIsSet(utils.BootnodesV4Flag.Name) {
		return
	}

	cfg.P2P.BootstrapNodes = []*enode.Node{}

	for _, url := range SwarmBootnodes {
		node, err := enode.ParseV4(url)
		if err != nil {
			log.Error("Bootstrap URL invalid", "enode", url, "err", err)
		}
		cfg.P2P.BootstrapNodes = append(cfg.P2P.BootstrapNodes, node)
	}
	log.Debug("added default swarm bootnodes", "length", len(cfg.P2P.BootstrapNodes))
}
