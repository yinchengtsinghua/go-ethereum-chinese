
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

//p2psim为模拟HTTP API提供命令行客户端。
//
//下面是用第一个节点创建2节点网络的示例
//连接到第二个：
//
//$p2psim节点创建
//创建节点01
//
//$p2psim节点开始节点01
//启动NODE01
//
//$p2psim节点创建
//创建NoDE02
//
//$p2psim节点开始节点02
//启动NODE02
//
//$p2psim节点连接node01 node02
//已将node01连接到node02
//
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/rpc"
	"gopkg.in/urfave/cli.v1"
)

var client *simulations.Client

func main() {
	app := cli.NewApp()
	app.Usage = "devp2p simulation command-line client"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "api",
Value:  "http://本地主机：8888“，
			Usage:  "simulation API URL",
			EnvVar: "P2PSIM_API_URL",
		},
	}
	app.Before = func(ctx *cli.Context) error {
		client = simulations.NewClient(ctx.GlobalString("api"))
		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:   "show",
			Usage:  "show network information",
			Action: showNetwork,
		},
		{
			Name:   "events",
			Usage:  "stream network events",
			Action: streamNetwork,
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "current",
					Usage: "get existing nodes and conns first",
				},
				cli.StringFlag{
					Name:  "filter",
					Value: "",
					Usage: "message filter",
				},
			},
		},
		{
			Name:   "snapshot",
			Usage:  "create a network snapshot to stdout",
			Action: createSnapshot,
		},
		{
			Name:   "load",
			Usage:  "load a network snapshot from stdin",
			Action: loadSnapshot,
		},
		{
			Name:   "node",
			Usage:  "manage simulation nodes",
			Action: listNodes,
			Subcommands: []cli.Command{
				{
					Name:   "list",
					Usage:  "list nodes",
					Action: listNodes,
				},
				{
					Name:   "create",
					Usage:  "create a node",
					Action: createNode,
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "name",
							Value: "",
							Usage: "node name",
						},
						cli.StringFlag{
							Name:  "services",
							Value: "",
							Usage: "node services (comma separated)",
						},
						cli.StringFlag{
							Name:  "key",
							Value: "",
							Usage: "node private key (hex encoded)",
						},
					},
				},
				{
					Name:      "show",
					ArgsUsage: "<node>",
					Usage:     "show node information",
					Action:    showNode,
				},
				{
					Name:      "start",
					ArgsUsage: "<node>",
					Usage:     "start a node",
					Action:    startNode,
				},
				{
					Name:      "stop",
					ArgsUsage: "<node>",
					Usage:     "stop a node",
					Action:    stopNode,
				},
				{
					Name:      "connect",
					ArgsUsage: "<node> <peer>",
					Usage:     "connect a node to a peer node",
					Action:    connectNode,
				},
				{
					Name:      "disconnect",
					ArgsUsage: "<node> <peer>",
					Usage:     "disconnect a node from a peer node",
					Action:    disconnectNode,
				},
				{
					Name:      "rpc",
					ArgsUsage: "<node> <method> [<args>]",
					Usage:     "call a node RPC method",
					Action:    rpcNode,
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "subscribe",
							Usage: "method is a subscription",
						},
					},
				},
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func showNetwork(ctx *cli.Context) error {
	if len(ctx.Args()) != 0 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	network, err := client.GetNetwork()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(ctx.App.Writer, 1, 2, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "NODES\t%d\n", len(network.Nodes))
	fmt.Fprintf(w, "CONNS\t%d\n", len(network.Conns))
	return nil
}

func streamNetwork(ctx *cli.Context) error {
	if len(ctx.Args()) != 0 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	events := make(chan *simulations.Event)
	sub, err := client.SubscribeNetwork(events, simulations.SubscribeOpts{
		Current: ctx.Bool("current"),
		Filter:  ctx.String("filter"),
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	enc := json.NewEncoder(ctx.App.Writer)
	for {
		select {
		case event := <-events:
			if err := enc.Encode(event); err != nil {
				return err
			}
		case err := <-sub.Err():
			return err
		}
	}
}

func createSnapshot(ctx *cli.Context) error {
	if len(ctx.Args()) != 0 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	snap, err := client.CreateSnapshot()
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(snap)
}

func loadSnapshot(ctx *cli.Context) error {
	if len(ctx.Args()) != 0 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	snap := &simulations.Snapshot{}
	if err := json.NewDecoder(os.Stdin).Decode(snap); err != nil {
		return err
	}
	return client.LoadSnapshot(snap)
}

func listNodes(ctx *cli.Context) error {
	if len(ctx.Args()) != 0 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	nodes, err := client.GetNodes()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(ctx.App.Writer, 1, 2, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "NAME\tPROTOCOLS\tID\n")
	for _, node := range nodes {
		fmt.Fprintf(w, "%s\t%s\t%s\n", node.Name, strings.Join(protocolList(node), ","), node.ID)
	}
	return nil
}

func protocolList(node *p2p.NodeInfo) []string {
	protos := make([]string, 0, len(node.Protocols))
	for name := range node.Protocols {
		protos = append(protos, name)
	}
	return protos
}

func createNode(ctx *cli.Context) error {
	if len(ctx.Args()) != 0 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	config := adapters.RandomNodeConfig()
	config.Name = ctx.String("name")
	if key := ctx.String("key"); key != "" {
		privKey, err := crypto.HexToECDSA(key)
		if err != nil {
			return err
		}
		config.ID = enode.PubkeyToIDV4(&privKey.PublicKey)
		config.PrivateKey = privKey
	}
	if services := ctx.String("services"); services != "" {
		config.Services = strings.Split(services, ",")
	}
	node, err := client.CreateNode(config)
	if err != nil {
		return err
	}
	fmt.Fprintln(ctx.App.Writer, "Created", node.Name)
	return nil
}

func showNode(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) != 1 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	nodeName := args[0]
	node, err := client.GetNode(nodeName)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(ctx.App.Writer, 1, 2, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "NAME\t%s\n", node.Name)
	fmt.Fprintf(w, "PROTOCOLS\t%s\n", strings.Join(protocolList(node), ","))
	fmt.Fprintf(w, "ID\t%s\n", node.ID)
	fmt.Fprintf(w, "ENODE\t%s\n", node.Enode)
	for name, proto := range node.Protocols {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "--- PROTOCOL INFO: %s\n", name)
		fmt.Fprintf(w, "%v\n", proto)
		fmt.Fprintf(w, "---\n")
	}
	return nil
}

func startNode(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) != 1 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	nodeName := args[0]
	if err := client.StartNode(nodeName); err != nil {
		return err
	}
	fmt.Fprintln(ctx.App.Writer, "Started", nodeName)
	return nil
}

func stopNode(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) != 1 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	nodeName := args[0]
	if err := client.StopNode(nodeName); err != nil {
		return err
	}
	fmt.Fprintln(ctx.App.Writer, "Stopped", nodeName)
	return nil
}

func connectNode(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) != 2 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	nodeName := args[0]
	peerName := args[1]
	if err := client.ConnectNode(nodeName, peerName); err != nil {
		return err
	}
	fmt.Fprintln(ctx.App.Writer, "Connected", nodeName, "to", peerName)
	return nil
}

func disconnectNode(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) != 2 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	nodeName := args[0]
	peerName := args[1]
	if err := client.DisconnectNode(nodeName, peerName); err != nil {
		return err
	}
	fmt.Fprintln(ctx.App.Writer, "Disconnected", nodeName, "from", peerName)
	return nil
}

func rpcNode(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) < 2 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	nodeName := args[0]
	method := args[1]
	rpcClient, err := client.RPCClient(context.Background(), nodeName)
	if err != nil {
		return err
	}
	if ctx.Bool("subscribe") {
		return rpcSubscribe(rpcClient, ctx.App.Writer, method, args[3:]...)
	}
	var result interface{}
	params := make([]interface{}, len(args[3:]))
	for i, v := range args[3:] {
		params[i] = v
	}
	if err := rpcClient.Call(&result, method, params...); err != nil {
		return err
	}
	return json.NewEncoder(ctx.App.Writer).Encode(result)
}

func rpcSubscribe(client *rpc.Client, out io.Writer, method string, args ...string) error {
	parts := strings.SplitN(method, "_", 2)
	namespace := parts[0]
	method = parts[1]
	ch := make(chan interface{})
	subArgs := make([]interface{}, len(args)+1)
	subArgs[0] = method
	for i, v := range args {
		subArgs[i+1] = v
	}
	sub, err := client.Subscribe(context.Background(), namespace, ch, subArgs...)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	enc := json.NewEncoder(out)
	for {
		select {
		case v := <-ch:
			if err := enc.Encode(v); err != nil {
				return err
			}
		case err := <-sub.Err():
			return err
		}
	}
}
