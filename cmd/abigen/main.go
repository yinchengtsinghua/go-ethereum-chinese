
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
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common/compiler"
)

var (
	abiFlag = flag.String("abi", "", "Path to the Ethereum contract ABI json to bind, - for STDIN")
	binFlag = flag.String("bin", "", "Path to the Ethereum contract bytecode (generate deploy method)")
	typFlag = flag.String("type", "", "Struct name for the binding (default = package name)")

	solFlag  = flag.String("sol", "", "Path to the Ethereum contract Solidity source to build and bind")
	solcFlag = flag.String("solc", "solc", "Solidity compiler to use if source builds are requested")
	excFlag  = flag.String("exc", "", "Comma separated types to exclude from binding")

	pkgFlag  = flag.String("pkg", "", "Package name to generate the binding into")
	outFlag  = flag.String("out", "", "Output file for the generated binding (default = stdout)")
	langFlag = flag.String("lang", "go", "Destination language for the bindings (go, java, objc)")
)

func main() {
//分析并确保指定了所有需要的输入
	flag.Parse()

	if *abiFlag == "" && *solFlag == "" {
		fmt.Printf("No contract ABI (--abi) or Solidity source (--sol) specified\n")
		os.Exit(-1)
	} else if (*abiFlag != "" || *binFlag != "" || *typFlag != "") && *solFlag != "" {
		fmt.Printf("Contract ABI (--abi), bytecode (--bin) and type (--type) flags are mutually exclusive with the Solidity source (--sol) flag\n")
		os.Exit(-1)
	}
	if *pkgFlag == "" {
		fmt.Printf("No destination package specified (--pkg)\n")
		os.Exit(-1)
	}
	var lang bind.Lang
	switch *langFlag {
	case "go":
		lang = bind.LangGo
	case "java":
		lang = bind.LangJava
	case "objc":
		lang = bind.LangObjC
	default:
		fmt.Printf("Unsupported destination language \"%s\" (--lang)\n", *langFlag)
		os.Exit(-1)
	}
//如果指定了整个solidity代码，则基于该代码构建和绑定
	var (
		abis  []string
		bins  []string
		types []string
	)
	if *solFlag != "" || (*abiFlag == "-" && *pkgFlag == "") {
//生成要从绑定中排除的类型列表
		exclude := make(map[string]bool)
		for _, kind := range strings.Split(*excFlag, ",") {
			exclude[strings.ToLower(kind)] = true
		}

		var contracts map[string]*compiler.Contract
		var err error
		if *solFlag != "" {
			contracts, err = compiler.CompileSolidity(*solcFlag, *solFlag)
			if err != nil {
				fmt.Printf("Failed to build Solidity contract: %v\n", err)
				os.Exit(-1)
			}
		} else {
			contracts, err = contractsFromStdin()
			if err != nil {
				fmt.Printf("Failed to read input ABIs from STDIN: %v\n", err)
				os.Exit(-1)
			}
		}
//收集所有未排除的合同进行约束
		for name, contract := range contracts {
			if exclude[strings.ToLower(name)] {
				continue
			}
abi, _ := json.Marshal(contract.Info.AbiDefinition) //扁平化编译器分析
			abis = append(abis, string(abi))
			bins = append(bins, contract.Code)

			nameParts := strings.Split(name, ":")
			types = append(types, nameParts[len(nameParts)-1])
		}
	} else {
//否则，从参数中加载abi、可选字节码和类型名。
		var abi []byte
		var err error
		if *abiFlag == "-" {
			abi, err = ioutil.ReadAll(os.Stdin)
		} else {
			abi, err = ioutil.ReadFile(*abiFlag)
		}
		if err != nil {
			fmt.Printf("Failed to read input ABI: %v\n", err)
			os.Exit(-1)
		}
		abis = append(abis, string(abi))

		bin := []byte{}
		if *binFlag != "" {
			if bin, err = ioutil.ReadFile(*binFlag); err != nil {
				fmt.Printf("Failed to read input bytecode: %v\n", err)
				os.Exit(-1)
			}
		}
		bins = append(bins, string(bin))

		kind := *typFlag
		if kind == "" {
			kind = *pkgFlag
		}
		types = append(types, kind)
	}
//生成合同绑定
	code, err := bind.Bind(types, abis, bins, *pkgFlag, lang)
	if err != nil {
		fmt.Printf("Failed to generate ABI binding: %v\n", err)
		os.Exit(-1)
	}
//将其刷新为文件或显示在标准输出上
	if *outFlag == "" {
		fmt.Printf("%s\n", code)
		return
	}
	if err := ioutil.WriteFile(*outFlag, []byte(code), 0600); err != nil {
		fmt.Printf("Failed to write ABI binding: %v\n", err)
		os.Exit(-1)
	}
}

func contractsFromStdin() (map[string]*compiler.Contract, error) {
	bytes, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}
	return compiler.ParseCombinedJSON(bytes, "", "", "", "")
}
