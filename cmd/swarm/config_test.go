
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
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/docker/docker/pkg/reexec"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm"
	"github.com/ethereum/go-ethereum/swarm/api"
)

func TestConfigDump(t *testing.T) {
	swarm := runSwarm(t, "dumpconfig")
	defaultConf := api.NewConfig()
	out, err := tomlSettings.Marshal(&defaultConf)
	if err != nil {
		t.Fatal(err)
	}
	swarm.Expect(string(out))
	swarm.ExpectExit()
}

func TestConfigFailsSwapEnabledNoSwapApi(t *testing.T) {
	flags := []string{
		fmt.Sprintf("--%s", SwarmNetworkIdFlag.Name), "42",
		fmt.Sprintf("--%s", SwarmPortFlag.Name), "54545",
		fmt.Sprintf("--%s", SwarmSwapEnabledFlag.Name),
	}

	swarm := runSwarm(t, flags...)
	swarm.Expect("Fatal: " + SWARM_ERR_SWAP_SET_NO_API + "\n")
	swarm.ExpectExit()
}

func TestConfigFailsNoBzzAccount(t *testing.T) {
	flags := []string{
		fmt.Sprintf("--%s", SwarmNetworkIdFlag.Name), "42",
		fmt.Sprintf("--%s", SwarmPortFlag.Name), "54545",
	}

	swarm := runSwarm(t, flags...)
	swarm.Expect("Fatal: " + SWARM_ERR_NO_BZZACCOUNT + "\n")
	swarm.ExpectExit()
}

func TestConfigCmdLineOverrides(t *testing.T) {
	dir, err := ioutil.TempDir("", "bzztest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	conf, account := getTestAccount(t, dir)
	node := &testNode{Dir: dir}

//指定端口
	httpPort, err := assignTCPPort()
	if err != nil {
		t.Fatal(err)
	}

	flags := []string{
		fmt.Sprintf("--%s", SwarmNetworkIdFlag.Name), "42",
		fmt.Sprintf("--%s", SwarmPortFlag.Name), httpPort,
		fmt.Sprintf("--%s", SwarmSyncDisabledFlag.Name),
		fmt.Sprintf("--%s", CorsStringFlag.Name), "*",
		fmt.Sprintf("--%s", SwarmAccountFlag.Name), account.Address.String(),
		fmt.Sprintf("--%s", SwarmDeliverySkipCheckFlag.Name),
		fmt.Sprintf("--%s", EnsAPIFlag.Name), "",
		fmt.Sprintf("--%s", utils.DataDirFlag.Name), dir,
		fmt.Sprintf("--%s", utils.IPCPathFlag.Name), conf.IPCPath,
	}
	node.Cmd = runSwarm(t, flags...)
	node.Cmd.InputLine(testPassphrase)
	defer func() {
		if t.Failed() {
			node.Shutdown()
		}
	}()
//等待节点启动
	for start := time.Now(); time.Since(start) < 10*time.Second; time.Sleep(50 * time.Millisecond) {
		node.Client, err = rpc.Dial(conf.IPCEndpoint())
		if err == nil {
			break
		}
	}
	if node.Client == nil {
		t.Fatal(err)
	}

//加载信息
	var info swarm.Info
	if err := node.Client.Call(&info, "bzz_info"); err != nil {
		t.Fatal(err)
	}

	if info.Port != httpPort {
		t.Fatalf("Expected port to be %s, got %s", httpPort, info.Port)
	}

	if info.NetworkID != 42 {
		t.Fatalf("Expected network ID to be %d, got %d", 42, info.NetworkID)
	}

	if info.SyncEnabled {
		t.Fatal("Expected Sync to be disabled, but is true")
	}

	if !info.DeliverySkipCheck {
		t.Fatal("Expected DeliverySkipCheck to be enabled, but it is not")
	}

	if info.Cors != "*" {
		t.Fatalf("Expected Cors flag to be set to %s, got %s", "*", info.Cors)
	}

	node.Shutdown()
}

func TestConfigFileOverrides(t *testing.T) {

//指定端口
	httpPort, err := assignTCPPort()
	if err != nil {
		t.Fatal(err)
	}

//创建配置文件
//首先，创建默认配置
	defaultConf := api.NewConfig()
//更改一些值以测试它们是否已加载
	defaultConf.SyncEnabled = false
	defaultConf.DeliverySkipCheck = true
	defaultConf.NetworkID = 54
	defaultConf.Port = httpPort
	defaultConf.DbCapacity = 9000000
	defaultConf.HiveParams.KeepAliveInterval = 6000000000
	defaultConf.Swap.Params.Strategy.AutoCashInterval = 600 * time.Second
//
//
	out, err := tomlSettings.Marshal(&defaultConf)
	if err != nil {
		t.Fatalf("Error creating TOML file in TestFileOverride: %v", err)
	}
//创建文件
	f, err := ioutil.TempFile("", "testconfig.toml")
	if err != nil {
		t.Fatalf("Error writing TOML file in TestFileOverride: %v", err)
	}
//写入文件
	_, err = f.WriteString(string(out))
	if err != nil {
		t.Fatalf("Error writing TOML file in TestFileOverride: %v", err)
	}
	f.Sync()

	dir, err := ioutil.TempDir("", "bzztest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	conf, account := getTestAccount(t, dir)
	node := &testNode{Dir: dir}

	flags := []string{
		fmt.Sprintf("--%s", SwarmTomlConfigPathFlag.Name), f.Name(),
		fmt.Sprintf("--%s", SwarmAccountFlag.Name), account.Address.String(),
		fmt.Sprintf("--%s", EnsAPIFlag.Name), "",
		fmt.Sprintf("--%s", utils.DataDirFlag.Name), dir,
		fmt.Sprintf("--%s", utils.IPCPathFlag.Name), conf.IPCPath,
	}
	node.Cmd = runSwarm(t, flags...)
	node.Cmd.InputLine(testPassphrase)
	defer func() {
		if t.Failed() {
			node.Shutdown()
		}
	}()
//等待节点启动
	for start := time.Now(); time.Since(start) < 10*time.Second; time.Sleep(50 * time.Millisecond) {
		node.Client, err = rpc.Dial(conf.IPCEndpoint())
		if err == nil {
			break
		}
	}
	if node.Client == nil {
		t.Fatal(err)
	}

//加载信息
	var info swarm.Info
	if err := node.Client.Call(&info, "bzz_info"); err != nil {
		t.Fatal(err)
	}

	if info.Port != httpPort {
		t.Fatalf("Expected port to be %s, got %s", httpPort, info.Port)
	}

	if info.NetworkID != 54 {
		t.Fatalf("Expected network ID to be %d, got %d", 54, info.NetworkID)
	}

	if info.SyncEnabled {
		t.Fatal("Expected Sync to be disabled, but is true")
	}

	if !info.DeliverySkipCheck {
		t.Fatal("Expected DeliverySkipCheck to be enabled, but it is not")
	}

	if info.DbCapacity != 9000000 {
		t.Fatalf("Expected network ID to be %d, got %d", 54, info.NetworkID)
	}

	if info.HiveParams.KeepAliveInterval != 6000000000 {
		t.Fatalf("Expected HiveParams KeepAliveInterval to be %d, got %d", uint64(6000000000), uint64(info.HiveParams.KeepAliveInterval))
	}

	if info.Swap.Params.Strategy.AutoCashInterval != 600*time.Second {
		t.Fatalf("Expected SwapParams AutoCashInterval to be %ds, got %d", 600, info.Swap.Params.Strategy.AutoCashInterval)
	}

//如果info.syncparams.keybufferresize！= 512 {
//t.fatalf（“预期的info.syncparams.keybufferresize为%d，得到的是%d”，512，info.syncparams.keybufferresize）
//}

	node.Shutdown()
}

func TestConfigEnvVars(t *testing.T) {
//指定端口
	httpPort, err := assignTCPPort()
	if err != nil {
		t.Fatal(err)
	}

	envVars := os.Environ()
	envVars = append(envVars, fmt.Sprintf("%s=%s", SwarmPortFlag.EnvVar, httpPort))
	envVars = append(envVars, fmt.Sprintf("%s=%s", SwarmNetworkIdFlag.EnvVar, "999"))
	envVars = append(envVars, fmt.Sprintf("%s=%s", CorsStringFlag.EnvVar, "*"))
	envVars = append(envVars, fmt.Sprintf("%s=%s", SwarmSyncDisabledFlag.EnvVar, "true"))
	envVars = append(envVars, fmt.Sprintf("%s=%s", SwarmDeliverySkipCheckFlag.EnvVar, "true"))

	dir, err := ioutil.TempDir("", "bzztest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	conf, account := getTestAccount(t, dir)
	node := &testNode{Dir: dir}
	flags := []string{
		fmt.Sprintf("--%s", SwarmAccountFlag.Name), account.Address.String(),
		"--ens-api", "",
		"--datadir", dir,
		"--ipcpath", conf.IPCPath,
	}

//node.cmd=runswarm（t，flags…）
//node.cmd.cmd.env=环境变量
//
	cmd := &exec.Cmd{
		Path:   reexec.Self(),
		Args:   append([]string{"swarm-test"}, flags...),
		Stderr: os.Stderr,
		Stdout: os.Stdout,
	}
	cmd.Env = envVars
//stdout，错误：=cmd.stdopupipe（）
//如果犯错！= nIL{
//致死性（Err）
//}
//
	var stdin io.WriteCloser
	if stdin, err = cmd.StdinPipe(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

//命令输入行（testpassphrase）
	io.WriteString(stdin, testPassphrase+"\n")
	defer func() {
		if t.Failed() {
			node.Shutdown()
			cmd.Process.Kill()
		}
	}()
//等待节点启动
	for start := time.Now(); time.Since(start) < 10*time.Second; time.Sleep(50 * time.Millisecond) {
		node.Client, err = rpc.Dial(conf.IPCEndpoint())
		if err == nil {
			break
		}
	}

	if node.Client == nil {
		t.Fatal(err)
	}

//加载信息
	var info swarm.Info
	if err := node.Client.Call(&info, "bzz_info"); err != nil {
		t.Fatal(err)
	}

	if info.Port != httpPort {
		t.Fatalf("Expected port to be %s, got %s", httpPort, info.Port)
	}

	if info.NetworkID != 999 {
		t.Fatalf("Expected network ID to be %d, got %d", 999, info.NetworkID)
	}

	if info.Cors != "*" {
		t.Fatalf("Expected Cors flag to be set to %s, got %s", "*", info.Cors)
	}

	if info.SyncEnabled {
		t.Fatal("Expected Sync to be disabled, but is true")
	}

	if !info.DeliverySkipCheck {
		t.Fatal("Expected DeliverySkipCheck to be enabled, but it is not")
	}

	node.Shutdown()
	cmd.Process.Kill()
}

func TestConfigCmdLineOverridesFile(t *testing.T) {

//指定端口
	httpPort, err := assignTCPPort()
	if err != nil {
		t.Fatal(err)
	}

//创建配置文件
//首先，创建默认配置
	defaultConf := api.NewConfig()
//更改一些值以测试它们是否已加载
	defaultConf.SyncEnabled = true
	defaultConf.NetworkID = 54
	defaultConf.Port = "8588"
	defaultConf.DbCapacity = 9000000
	defaultConf.HiveParams.KeepAliveInterval = 6000000000
	defaultConf.Swap.Params.Strategy.AutoCashInterval = 600 * time.Second
//
//创建toml文件
	out, err := tomlSettings.Marshal(&defaultConf)
	if err != nil {
		t.Fatalf("Error creating TOML file in TestFileOverride: %v", err)
	}
//写入文件
	fname := "testconfig.toml"
	f, err := ioutil.TempFile("", fname)
	if err != nil {
		t.Fatalf("Error writing TOML file in TestFileOverride: %v", err)
	}
	defer os.Remove(fname)
//写入文件
	_, err = f.WriteString(string(out))
	if err != nil {
		t.Fatalf("Error writing TOML file in TestFileOverride: %v", err)
	}
	f.Sync()

	dir, err := ioutil.TempDir("", "bzztest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	conf, account := getTestAccount(t, dir)
	node := &testNode{Dir: dir}

	expectNetworkId := uint64(77)

	flags := []string{
		fmt.Sprintf("--%s", SwarmNetworkIdFlag.Name), "77",
		fmt.Sprintf("--%s", SwarmPortFlag.Name), httpPort,
		fmt.Sprintf("--%s", SwarmSyncDisabledFlag.Name),
		fmt.Sprintf("--%s", SwarmTomlConfigPathFlag.Name), f.Name(),
		fmt.Sprintf("--%s", SwarmAccountFlag.Name), account.Address.String(),
		fmt.Sprintf("--%s", EnsAPIFlag.Name), "",
		fmt.Sprintf("--%s", utils.DataDirFlag.Name), dir,
		fmt.Sprintf("--%s", utils.IPCPathFlag.Name), conf.IPCPath,
	}
	node.Cmd = runSwarm(t, flags...)
	node.Cmd.InputLine(testPassphrase)
	defer func() {
		if t.Failed() {
			node.Shutdown()
		}
	}()
//等待节点启动
	for start := time.Now(); time.Since(start) < 10*time.Second; time.Sleep(50 * time.Millisecond) {
		node.Client, err = rpc.Dial(conf.IPCEndpoint())
		if err == nil {
			break
		}
	}
	if node.Client == nil {
		t.Fatal(err)
	}

//加载信息
	var info swarm.Info
	if err := node.Client.Call(&info, "bzz_info"); err != nil {
		t.Fatal(err)
	}

	if info.Port != httpPort {
		t.Fatalf("Expected port to be %s, got %s", httpPort, info.Port)
	}

	if info.NetworkID != expectNetworkId {
		t.Fatalf("Expected network ID to be %d, got %d", expectNetworkId, info.NetworkID)
	}

	if info.SyncEnabled {
		t.Fatal("Expected Sync to be disabled, but is true")
	}

	if info.LocalStoreParams.DbCapacity != 9000000 {
		t.Fatalf("Expected Capacity to be %d, got %d", 9000000, info.LocalStoreParams.DbCapacity)
	}

	if info.HiveParams.KeepAliveInterval != 6000000000 {
		t.Fatalf("Expected HiveParams KeepAliveInterval to be %d, got %d", uint64(6000000000), uint64(info.HiveParams.KeepAliveInterval))
	}

	if info.Swap.Params.Strategy.AutoCashInterval != 600*time.Second {
		t.Fatalf("Expected SwapParams AutoCashInterval to be %ds, got %d", 600, info.Swap.Params.Strategy.AutoCashInterval)
	}

//如果info.syncparams.keybufferresize！= 512 {
//t.fatalf（“预期的info.syncparams.keybufferresize为%d，得到的是%d”，512，info.syncparams.keybufferresize）
//}

	node.Shutdown()
}

func TestConfigValidate(t *testing.T) {
	for _, c := range []struct {
		cfg *api.Config
		err string
	}{
		{
			cfg: &api.Config{EnsAPIs: []string{
				"/data/testnet/geth.ipc",
			}},
		},
		{
			cfg: &api.Config{EnsAPIs: []string{
"http://127.0.0.1:1234“，
			}},
		},
		{
			cfg: &api.Config{EnsAPIs: []string{
"ws://127.0.0.1:1234“，
			}},
		},
		{
			cfg: &api.Config{EnsAPIs: []string{
				"test:/data/testnet/geth.ipc",
			}},
		},
		{
			cfg: &api.Config{EnsAPIs: []string{
"test:ws://127.0.0.1:1234“，
			}},
		},
		{
			cfg: &api.Config{EnsAPIs: []string{
				"314159265dD8dbb310642f98f50C066173C1259b@/data/testnet/geth.ipc",
			}},
		},
		{
			cfg: &api.Config{EnsAPIs: []string{
"314159265dD8dbb310642f98f50C066173C1259b@http://127.0.0.1:1234“，
			}},
		},
		{
			cfg: &api.Config{EnsAPIs: []string{
"314159265dD8dbb310642f98f50C066173C1259b@ws://127.0.0.1:1234“，
			}},
		},
		{
			cfg: &api.Config{EnsAPIs: []string{
				"test:314159265dD8dbb310642f98f50C066173C1259b@/data/testnet/geth.ipc",
			}},
		},
		{
			cfg: &api.Config{EnsAPIs: []string{
"eth:314159265dD8dbb310642f98f50C066173C1259b@http://127.0.0.1:1234“，
			}},
		},
		{
			cfg: &api.Config{EnsAPIs: []string{
"eth:314159265dD8dbb310642f98f50C066173C1259b@ws://
			}},
		},
		{
			cfg: &api.Config{EnsAPIs: []string{
				"eth:",
			}},
			err: "invalid format [tld:][contract-addr@]url for ENS API endpoint configuration \"eth:\": missing url",
		},
		{
			cfg: &api.Config{EnsAPIs: []string{
				"314159265dD8dbb310642f98f50C066173C1259b@",
			}},
			err: "invalid format [tld:][contract-addr@]url for ENS API endpoint configuration \"314159265dD8dbb310642f98f50C066173C1259b@\": missing url",
		},
		{
			cfg: &api.Config{EnsAPIs: []string{
				":314159265dD8dbb310642f98f50C066173C1259",
			}},
			err: "invalid format [tld:][contract-addr@]url for ENS API endpoint configuration \":314159265dD8dbb310642f98f50C066173C1259\": missing tld",
		},
		{
			cfg: &api.Config{EnsAPIs: []string{
				"@/data/testnet/geth.ipc",
			}},
			err: "invalid format [tld:][contract-addr@]url for ENS API endpoint configuration \"@/data/testnet/geth.ipc\": missing contract address",
		},
	} {
		err := validateConfig(c.cfg)
		if c.err != "" && err.Error() != c.err {
			t.Errorf("expected error %q, got %q", c.err, err)
		}
		if c.err == "" && err != nil {
			t.Errorf("unexpected error %q", err)
		}
	}
}

func assignTCPPort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return "", err
	}
	return port, nil
}
