
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
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pborman/uuid"
	"gopkg.in/urfave/cli.v1"
)

type outputGenerate struct {
	Address      string
	AddressEIP55 string
}

var commandGenerate = cli.Command{
	Name:      "generate",
	Usage:     "generate new keyfile",
	ArgsUsage: "[ <keyfile> ]",
	Description: `
Generate a new keyfile.

If you want to encrypt an existing private key, it can be specified by setting
--privatekey with the location of the file containing the private key.
`,
	Flags: []cli.Flag{
		passphraseFlag,
		jsonFlag,
		cli.StringFlag{
			Name:  "privatekey",
			Usage: "file containing a raw private key to encrypt",
		},
	},
	Action: func(ctx *cli.Context) error {
//检查是否指定了关键文件路径，并确保它不存在。
		keyfilepath := ctx.Args().First()
		if keyfilepath == "" {
			keyfilepath = defaultKeyfileName
		}
		if _, err := os.Stat(keyfilepath); err == nil {
			utils.Fatalf("Keyfile already exists at %s.", keyfilepath)
		} else if !os.IsNotExist(err) {
			utils.Fatalf("Error checking if keyfile exists: %v", err)
		}

		var privateKey *ecdsa.PrivateKey
		var err error
		if file := ctx.String("privatekey"); file != "" {
//从文件加载私钥。
			privateKey, err = crypto.LoadECDSA(file)
			if err != nil {
				utils.Fatalf("Can't load private key: %v", err)
			}
		} else {
//如果未加载，则生成随机。
			privateKey, err = crypto.GenerateKey()
			if err != nil {
				utils.Fatalf("Failed to generate random private key: %v", err)
			}
		}

//使用随机UUID创建keyfile对象。
		id := uuid.NewRandom()
		key := &keystore.Key{
			Id:         id,
			Address:    crypto.PubkeyToAddress(privateKey.PublicKey),
			PrivateKey: privateKey,
		}

//用密码短语加密密钥。
		passphrase := promptPassphrase(true)
		keyjson, err := keystore.EncryptKey(key, passphrase, keystore.StandardScryptN, keystore.StandardScryptP)
		if err != nil {
			utils.Fatalf("Error encrypting key: %v", err)
		}

//将文件存储到磁盘。
		if err := os.MkdirAll(filepath.Dir(keyfilepath), 0700); err != nil {
			utils.Fatalf("Could not create directory %s", filepath.Dir(keyfilepath))
		}
		if err := ioutil.WriteFile(keyfilepath, keyjson, 0600); err != nil {
			utils.Fatalf("Failed to write keyfile to %s: %v", keyfilepath, err)
		}

//输出一些信息。
		out := outputGenerate{
			Address: key.Address.Hex(),
		}
		if ctx.Bool(jsonFlag.Name) {
			mustPrintJSON(out)
		} else {
			fmt.Println("Address:", out.Address)
		}
		return nil
	},
}
