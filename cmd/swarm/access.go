
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
package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/swarm/api"
	"github.com/ethereum/go-ethereum/swarm/api/client"
	"gopkg.in/urfave/cli.v1"
)

var (
	salt          = make([]byte, 32)
	accessCommand = cli.Command{
		CustomHelpTemplate: helpTemplate,
		Name:               "access",
		Usage:              "encrypts a reference and embeds it into a root manifest",
		ArgsUsage:          "<ref>",
		Description:        "encrypts a reference and embeds it into a root manifest",
		Subcommands: []cli.Command{
			{
				CustomHelpTemplate: helpTemplate,
				Name:               "new",
				Usage:              "encrypts a reference and embeds it into a root manifest",
				ArgsUsage:          "<ref>",
				Description:        "encrypts a reference and embeds it into a root access manifest and prints the resulting manifest",
				Subcommands: []cli.Command{
					{
						Action:             accessNewPass,
						CustomHelpTemplate: helpTemplate,
						Flags: []cli.Flag{
							utils.PasswordFileFlag,
							SwarmDryRunFlag,
						},
						Name:        "pass",
						Usage:       "encrypts a reference with a password and embeds it into a root manifest",
						ArgsUsage:   "<ref>",
						Description: "encrypts a reference and embeds it into a root access manifest and prints the resulting manifest",
					},
					{
						Action:             accessNewPK,
						CustomHelpTemplate: helpTemplate,
						Flags: []cli.Flag{
							utils.PasswordFileFlag,
							SwarmDryRunFlag,
							SwarmAccessGrantKeyFlag,
						},
						Name:        "pk",
						Usage:       "encrypts a reference with the node's private key and a given grantee's public key and embeds it into a root manifest",
						ArgsUsage:   "<ref>",
						Description: "encrypts a reference and embeds it into a root access manifest and prints the resulting manifest",
					},
					{
						Action:             accessNewACT,
						CustomHelpTemplate: helpTemplate,
						Flags: []cli.Flag{
							SwarmAccessGrantKeysFlag,
							SwarmDryRunFlag,
							utils.PasswordFileFlag,
						},
						Name:        "act",
						Usage:       "encrypts a reference with the node's private key and a given grantee's public key and embeds it into a root manifest",
						ArgsUsage:   "<ref>",
						Description: "encrypts a reference and embeds it into a root access manifest and prints the resulting manifest",
					},
				},
			},
		},
	}
)

func init() {
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		panic("reading from crypto/rand failed: " + err.Error())
	}
}

func accessNewPass(ctx *cli.Context) {
	args := ctx.Args()
	if len(args) != 1 {
		utils.Fatalf("Expected 1 argument - the ref")
	}

	var (
		ae        *api.AccessEntry
		accessKey []byte
		err       error
		ref       = args[0]
		password  = getPassPhrase("", 0, makePasswordList(ctx))
		dryRun    = ctx.Bool(SwarmDryRunFlag.Name)
	)
	accessKey, ae, err = api.DoPassword(ctx, password, salt)
	if err != nil {
		utils.Fatalf("error getting session key: %v", err)
	}
	m, err := api.GenerateAccessControlManifest(ctx, ref, accessKey, ae)
	if err != nil {
		utils.Fatalf("had an error generating the manifest: %v", err)
	}
	if dryRun {
		err = printManifests(m, nil)
		if err != nil {
			utils.Fatalf("had an error printing the manifests: %v", err)
		}
	} else {
		err = uploadManifests(ctx, m, nil)
		if err != nil {
			utils.Fatalf("had an error uploading the manifests: %v", err)
		}
	}
}

func accessNewPK(ctx *cli.Context) {
	args := ctx.Args()
	if len(args) != 1 {
		utils.Fatalf("Expected 1 argument - the ref")
	}

	var (
		ae               *api.AccessEntry
		sessionKey       []byte
		err              error
		ref              = args[0]
		privateKey       = getPrivKey(ctx)
		granteePublicKey = ctx.String(SwarmAccessGrantKeyFlag.Name)
		dryRun           = ctx.Bool(SwarmDryRunFlag.Name)
	)
	sessionKey, ae, err = api.DoPK(ctx, privateKey, granteePublicKey, salt)
	if err != nil {
		utils.Fatalf("error getting session key: %v", err)
	}
	m, err := api.GenerateAccessControlManifest(ctx, ref, sessionKey, ae)
	if err != nil {
		utils.Fatalf("had an error generating the manifest: %v", err)
	}
	if dryRun {
		err = printManifests(m, nil)
		if err != nil {
			utils.Fatalf("had an error printing the manifests: %v", err)
		}
	} else {
		err = uploadManifests(ctx, m, nil)
		if err != nil {
			utils.Fatalf("had an error uploading the manifests: %v", err)
		}
	}
}

func accessNewACT(ctx *cli.Context) {
	args := ctx.Args()
	if len(args) != 1 {
		utils.Fatalf("Expected 1 argument - the ref")
	}

	var (
		ae                   *api.AccessEntry
		actManifest          *api.Manifest
		accessKey            []byte
		err                  error
		ref                  = args[0]
		pkGrantees           = []string{}
		passGrantees         = []string{}
		pkGranteesFilename   = ctx.String(SwarmAccessGrantKeysFlag.Name)
		passGranteesFilename = ctx.String(utils.PasswordFileFlag.Name)
		privateKey           = getPrivKey(ctx)
		dryRun               = ctx.Bool(SwarmDryRunFlag.Name)
	)
	if pkGranteesFilename == "" && passGranteesFilename == "" {
		utils.Fatalf("you have to provide either a grantee public-keys file or an encryption passwords file (or both)")
	}

	if pkGranteesFilename != "" {
		bytes, err := ioutil.ReadFile(pkGranteesFilename)
		if err != nil {
			utils.Fatalf("had an error reading the grantee public key list")
		}
		pkGrantees = strings.Split(strings.Trim(string(bytes), "\n"), "\n")
	}

	if passGranteesFilename != "" {
		bytes, err := ioutil.ReadFile(passGranteesFilename)
		if err != nil {
			utils.Fatalf("could not read password filename: %v", err)
		}
		passGrantees = strings.Split(strings.Trim(string(bytes), "\n"), "\n")
	}
	accessKey, ae, actManifest, err = api.DoACT(ctx, privateKey, salt, pkGrantees, passGrantees)
	if err != nil {
		utils.Fatalf("error generating ACT manifest: %v", err)
	}

	if err != nil {
		utils.Fatalf("error getting session key: %v", err)
	}
	m, err := api.GenerateAccessControlManifest(ctx, ref, accessKey, ae)
	if err != nil {
		utils.Fatalf("error generating root access manifest: %v", err)
	}

	if dryRun {
		err = printManifests(m, actManifest)
		if err != nil {
			utils.Fatalf("had an error printing the manifests: %v", err)
		}
	} else {
		err = uploadManifests(ctx, m, actManifest)
		if err != nil {
			utils.Fatalf("had an error uploading the manifests: %v", err)
		}
	}
}

func printManifests(rootAccessManifest, actManifest *api.Manifest) error {
	js, err := json.Marshal(rootAccessManifest)
	if err != nil {
		return err
	}
	fmt.Println(string(js))

	if actManifest != nil {
		js, err := json.Marshal(actManifest)
		if err != nil {
			return err
		}
		fmt.Println(string(js))
	}
	return nil
}

func uploadManifests(ctx *cli.Context, rootAccessManifest, actManifest *api.Manifest) error {
	bzzapi := strings.TrimRight(ctx.GlobalString(SwarmApiFlag.Name), "/")
	client := client.NewClient(bzzapi)

	var (
		key string
		err error
	)
	if actManifest != nil {
		key, err = client.UploadManifest(actManifest, false)
		if err != nil {
			return err
		}

		rootAccessManifest.Entries[0].Access.Act = key
	}
	key, err = client.UploadManifest(rootAccessManifest, false)
	if err != nil {
		return err
	}
	fmt.Println(key)
	return nil
}

//makepasswordlist从global--password标志指定的文件中读取密码行
//以及同一个子命令——密码标志。
//此函数是utils.makepasswordlist的分支，用于查找子命令的CLI上下文。
//函数ctx.setglobal未设置可访问的全局标志值
//
func makePasswordList(ctx *cli.Context) []string {
	path := ctx.GlobalString(utils.PasswordFileFlag.Name)
	if path == "" {
		path = ctx.String(utils.PasswordFileFlag.Name)
		if path == "" {
			return nil
		}
	}
	text, err := ioutil.ReadFile(path)
	if err != nil {
		utils.Fatalf("Failed to read password file: %v", err)
	}
	lines := strings.Split(string(text), "\n")
//对DOS行结尾进行消毒。
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	return lines
}
