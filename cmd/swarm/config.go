
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	cli "gopkg.in/urfave/cli.v1"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/naoina/toml"

	bzzapi "github.com/ethereum/go-ethereum/swarm/api"
)

var (
//
	DumpConfigCommand = cli.Command{
		Action:      utils.MigrateFlags(dumpConfig),
		Name:        "dumpconfig",
		Usage:       "Show configuration values",
		ArgsUsage:   "",
		Flags:       app.Flags,
		Category:    "MISCELLANEOUS COMMANDS",
		Description: `The dumpconfig command shows configuration values.`,
	}

//
	SwarmTomlConfigPathFlag = cli.StringFlag{
		Name:  "config",
		Usage: "TOML configuration file",
	}
)

//
const (
	SWARM_ENV_CHEQUEBOOK_ADDR         = "SWARM_CHEQUEBOOK_ADDR"
	SWARM_ENV_ACCOUNT                 = "SWARM_ACCOUNT"
	SWARM_ENV_LISTEN_ADDR             = "SWARM_LISTEN_ADDR"
	SWARM_ENV_PORT                    = "SWARM_PORT"
	SWARM_ENV_NETWORK_ID              = "SWARM_NETWORK_ID"
	SWARM_ENV_SWAP_ENABLE             = "SWARM_SWAP_ENABLE"
	SWARM_ENV_SWAP_API                = "SWARM_SWAP_API"
	SWARM_ENV_SYNC_DISABLE            = "SWARM_SYNC_DISABLE"
	SWARM_ENV_SYNC_UPDATE_DELAY       = "SWARM_ENV_SYNC_UPDATE_DELAY"
	SWARM_ENV_MAX_STREAM_PEER_SERVERS = "SWARM_ENV_MAX_STREAM_PEER_SERVERS"
	SWARM_ENV_LIGHT_NODE_ENABLE       = "SWARM_LIGHT_NODE_ENABLE"
	SWARM_ENV_DELIVERY_SKIP_CHECK     = "SWARM_DELIVERY_SKIP_CHECK"
	SWARM_ENV_ENS_API                 = "SWARM_ENS_API"
	SWARM_ENV_ENS_ADDR                = "SWARM_ENS_ADDR"
	SWARM_ENV_CORS                    = "SWARM_CORS"
	SWARM_ENV_BOOTNODES               = "SWARM_BOOTNODES"
	SWARM_ENV_PSS_ENABLE              = "SWARM_PSS_ENABLE"
	SWARM_ENV_STORE_PATH              = "SWARM_STORE_PATH"
	SWARM_ENV_STORE_CAPACITY          = "SWARM_STORE_CAPACITY"
	SWARM_ENV_STORE_CACHE_CAPACITY    = "SWARM_STORE_CACHE_CAPACITY"
	SWARM_ACCESS_PASSWORD             = "SWARM_ACCESS_PASSWORD"
	SWARM_AUTO_DEFAULTPATH            = "SWARM_AUTO_DEFAULTPATH"
	GETH_ENV_DATADIR                  = "GETH_DATADIR"
)

//这些设置确保toml键使用与go struct字段相同的名称。
var tomlSettings = toml.Config{
	NormFieldName: func(rt reflect.Type, key string) string {
		return key
	},
	FieldToKey: func(rt reflect.Type, field string) string {
		return field
	},
	MissingField: func(rt reflect.Type, field string) error {
		link := ""
		if unicode.IsUpper(rune(rt.Name()[0])) && rt.PkgPath() != "main" {
			link = fmt.Sprintf(", check github.com/ethereum/go-ethereum/swarm/api/config.go for available fields")
		}
		return fmt.Errorf("field '%s' is not defined in %s%s", field, rt.String(), link)
	},
}

//在启动群节点之前，构建配置
func buildConfig(ctx *cli.Context) (config *bzzapi.Config, err error) {
//首先创建默认配置
	config = bzzapi.NewConfig()
//
	config, err = configFileOverride(config, ctx)
	if err != nil {
		return nil, err
	}
//覆盖环境变量提供的设置
	config = envVarsOverride(config)
//覆盖命令行提供的设置
	config = cmdLineOverride(config, ctx)
//
	err = validateConfig(config)

	return
}

//最后，在配置构建阶段完成后，初始化
func initSwarmNode(config *bzzapi.Config, stack *node.Node, ctx *cli.Context) {
//此时，应在配置中设置所有变量。
//
	prvkey := getAccount(config.BzzAccount, ctx, stack)
//
	config.Path = expandPath(stack.InstanceDir())
//
	config.Init(prvkey)
//
	log.Debug("Starting Swarm with the following parameters:")
//创建配置后，将其打印到屏幕
	log.Debug(printConfig(config))
}

//如果提供了配置文件，configfileoverride将用配置文件覆盖当前配置。
func configFileOverride(config *bzzapi.Config, ctx *cli.Context) (*bzzapi.Config, error) {
	var err error

//
	if ctx.GlobalIsSet(SwarmTomlConfigPathFlag.Name) {
		var filepath string
		if filepath = ctx.GlobalString(SwarmTomlConfigPathFlag.Name); filepath == "" {
			utils.Fatalf("Config file flag provided with invalid file path")
		}
		var f *os.File
		f, err = os.Open(filepath)
		if err != nil {
			return nil, err
		}
		defer f.Close()

//
//
//如果文件中没有条目，则保留默认条目。
		err = tomlSettings.NewDecoder(f).Decode(&config)
//将文件名添加到具有行号的错误中。
		if _, ok := err.(*toml.LineError); ok {
			err = errors.New(filepath + ", " + err.Error())
		}
	}
	return config, err
}

//
//
func cmdLineOverride(currentConfig *bzzapi.Config, ctx *cli.Context) *bzzapi.Config {

	if keyid := ctx.GlobalString(SwarmAccountFlag.Name); keyid != "" {
		currentConfig.BzzAccount = keyid
	}

	if chbookaddr := ctx.GlobalString(ChequebookAddrFlag.Name); chbookaddr != "" {
		currentConfig.Contract = common.HexToAddress(chbookaddr)
	}

	if networkid := ctx.GlobalString(SwarmNetworkIdFlag.Name); networkid != "" {
		id, err := strconv.ParseUint(networkid, 10, 64)
		if err != nil {
			utils.Fatalf("invalid cli flag %s: %v", SwarmNetworkIdFlag.Name, err)
		}
		if id != 0 {
			currentConfig.NetworkID = id
		}
	}

	if ctx.GlobalIsSet(utils.DataDirFlag.Name) {
		if datadir := ctx.GlobalString(utils.DataDirFlag.Name); datadir != "" {
			currentConfig.Path = expandPath(datadir)
		}
	}

	bzzport := ctx.GlobalString(SwarmPortFlag.Name)
	if len(bzzport) > 0 {
		currentConfig.Port = bzzport
	}

	if bzzaddr := ctx.GlobalString(SwarmListenAddrFlag.Name); bzzaddr != "" {
		currentConfig.ListenAddr = bzzaddr
	}

	if ctx.GlobalIsSet(SwarmSwapEnabledFlag.Name) {
		currentConfig.SwapEnabled = true
	}

	if ctx.GlobalIsSet(SwarmSyncDisabledFlag.Name) {
		currentConfig.SyncEnabled = false
	}

	if d := ctx.GlobalDuration(SwarmSyncUpdateDelay.Name); d > 0 {
		currentConfig.SyncUpdateDelay = d
	}

//包括0在内的任何值都可以接受
	currentConfig.MaxStreamPeerServers = ctx.GlobalInt(SwarmMaxStreamPeerServersFlag.Name)

	if ctx.GlobalIsSet(SwarmLightNodeEnabled.Name) {
		currentConfig.LightNodeEnabled = true
	}

	if ctx.GlobalIsSet(SwarmDeliverySkipCheckFlag.Name) {
		currentConfig.DeliverySkipCheck = true
	}

	currentConfig.SwapAPI = ctx.GlobalString(SwarmSwapAPIFlag.Name)
	if currentConfig.SwapEnabled && currentConfig.SwapAPI == "" {
		utils.Fatalf(SWARM_ERR_SWAP_SET_NO_API)
	}

	if ctx.GlobalIsSet(EnsAPIFlag.Name) {
		ensAPIs := ctx.GlobalStringSlice(EnsAPIFlag.Name)
//保留向后兼容性以禁用ens with--ens api=“”
		if len(ensAPIs) == 1 && ensAPIs[0] == "" {
			ensAPIs = nil
		}
		for i := range ensAPIs {
			ensAPIs[i] = expandPath(ensAPIs[i])
		}

		currentConfig.EnsAPIs = ensAPIs
	}

	if cors := ctx.GlobalString(CorsStringFlag.Name); cors != "" {
		currentConfig.Cors = cors
	}

	if storePath := ctx.GlobalString(SwarmStorePath.Name); storePath != "" {
		currentConfig.LocalStoreParams.ChunkDbPath = storePath
	}

	if storeCapacity := ctx.GlobalUint64(SwarmStoreCapacity.Name); storeCapacity != 0 {
		currentConfig.LocalStoreParams.DbCapacity = storeCapacity
	}

	if storeCacheCapacity := ctx.GlobalUint(SwarmStoreCacheCapacity.Name); storeCacheCapacity != 0 {
		currentConfig.LocalStoreParams.CacheCapacity = storeCacheCapacity
	}

	return currentConfig

}

//
//
func envVarsOverride(currentConfig *bzzapi.Config) (config *bzzapi.Config) {

	if keyid := os.Getenv(SWARM_ENV_ACCOUNT); keyid != "" {
		currentConfig.BzzAccount = keyid
	}

	if chbookaddr := os.Getenv(SWARM_ENV_CHEQUEBOOK_ADDR); chbookaddr != "" {
		currentConfig.Contract = common.HexToAddress(chbookaddr)
	}

	if networkid := os.Getenv(SWARM_ENV_NETWORK_ID); networkid != "" {
		id, err := strconv.ParseUint(networkid, 10, 64)
		if err != nil {
			utils.Fatalf("invalid environment variable %s: %v", SWARM_ENV_NETWORK_ID, err)
		}
		if id != 0 {
			currentConfig.NetworkID = id
		}
	}

	if datadir := os.Getenv(GETH_ENV_DATADIR); datadir != "" {
		currentConfig.Path = expandPath(datadir)
	}

	bzzport := os.Getenv(SWARM_ENV_PORT)
	if len(bzzport) > 0 {
		currentConfig.Port = bzzport
	}

	if bzzaddr := os.Getenv(SWARM_ENV_LISTEN_ADDR); bzzaddr != "" {
		currentConfig.ListenAddr = bzzaddr
	}

	if swapenable := os.Getenv(SWARM_ENV_SWAP_ENABLE); swapenable != "" {
		swap, err := strconv.ParseBool(swapenable)
		if err != nil {
			utils.Fatalf("invalid environment variable %s: %v", SWARM_ENV_SWAP_ENABLE, err)
		}
		currentConfig.SwapEnabled = swap
	}

	if syncdisable := os.Getenv(SWARM_ENV_SYNC_DISABLE); syncdisable != "" {
		sync, err := strconv.ParseBool(syncdisable)
		if err != nil {
			utils.Fatalf("invalid environment variable %s: %v", SWARM_ENV_SYNC_DISABLE, err)
		}
		currentConfig.SyncEnabled = !sync
	}

	if v := os.Getenv(SWARM_ENV_DELIVERY_SKIP_CHECK); v != "" {
		skipCheck, err := strconv.ParseBool(v)
		if err != nil {
			currentConfig.DeliverySkipCheck = skipCheck
		}
	}

	if v := os.Getenv(SWARM_ENV_SYNC_UPDATE_DELAY); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			utils.Fatalf("invalid environment variable %s: %v", SWARM_ENV_SYNC_UPDATE_DELAY, err)
		}
		currentConfig.SyncUpdateDelay = d
	}

	if max := os.Getenv(SWARM_ENV_MAX_STREAM_PEER_SERVERS); max != "" {
		m, err := strconv.Atoi(max)
		if err != nil {
			utils.Fatalf("invalid environment variable %s: %v", SWARM_ENV_MAX_STREAM_PEER_SERVERS, err)
		}
		currentConfig.MaxStreamPeerServers = m
	}

	if lne := os.Getenv(SWARM_ENV_LIGHT_NODE_ENABLE); lne != "" {
		lightnode, err := strconv.ParseBool(lne)
		if err != nil {
			utils.Fatalf("invalid environment variable %s: %v", SWARM_ENV_LIGHT_NODE_ENABLE, err)
		}
		currentConfig.LightNodeEnabled = lightnode
	}

	if swapapi := os.Getenv(SWARM_ENV_SWAP_API); swapapi != "" {
		currentConfig.SwapAPI = swapapi
	}

	if currentConfig.SwapEnabled && currentConfig.SwapAPI == "" {
		utils.Fatalf(SWARM_ERR_SWAP_SET_NO_API)
	}

	if ensapi := os.Getenv(SWARM_ENV_ENS_API); ensapi != "" {
		currentConfig.EnsAPIs = strings.Split(ensapi, ",")
	}

	if ensaddr := os.Getenv(SWARM_ENV_ENS_ADDR); ensaddr != "" {
		currentConfig.EnsRoot = common.HexToAddress(ensaddr)
	}

	if cors := os.Getenv(SWARM_ENV_CORS); cors != "" {
		currentConfig.Cors = cors
	}

	return currentConfig
}

//dumpconfig是dumpconfig命令。
//将默认配置写入stdout
func dumpConfig(ctx *cli.Context) error {
	cfg, err := buildConfig(ctx)
	if err != nil {
		utils.Fatalf(fmt.Sprintf("Uh oh - dumpconfig triggered an error %v", err))
	}
	comment := ""
	out, err := tomlSettings.Marshal(&cfg)
	if err != nil {
		return err
	}
	io.WriteString(os.Stdout, comment)
	os.Stdout.Write(out)
	return nil
}

//验证配置参数
func validateConfig(cfg *bzzapi.Config) (err error) {
	for _, ensAPI := range cfg.EnsAPIs {
		if ensAPI != "" {
			if err := validateEnsAPIs(ensAPI); err != nil {
				return fmt.Errorf("invalid format [tld:][contract-addr@]url for ENS API endpoint configuration %q: %v", ensAPI, err)
			}
		}
	}
	return nil
}

//验证ENSAPI配置参数
func validateEnsAPIs(s string) (err error) {
//
	if strings.HasPrefix(s, "@") {
		return errors.New("missing contract address")
	}
//丢失网址
	if strings.HasSuffix(s, "@") {
		return errors.New("missing url")
	}
//漏译
	if strings.HasPrefix(s, ":") {
		return errors.New("missing tld")
	}
//丢失网址
	if strings.HasSuffix(s, ":") {
		return errors.New("missing url")
	}
	return nil
}

//将配置打印为字符串
func printConfig(config *bzzapi.Config) string {
	out, err := tomlSettings.Marshal(&config)
	if err != nil {
		return fmt.Sprintf("Something is not right with the configuration: %v", err)
	}
	return string(out)
}
