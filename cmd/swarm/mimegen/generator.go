
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

//标准的“mime”包依赖于系统设置，请参阅mime.osinitmime
//Swarm将在许多操作系统/平台/Docker上运行，并且必须表现出类似的行为。
//此命令生成代码以添加基于mime.types文件的常见mime类型
//
//mailcap提供的mime.types文件，遵循https://www.iana.org/assignments/media-types/media-types.xhtml
//
//
//docker run--rm-v$（pwd）：/tmp-alpine:edge/bin/sh-c“apk-add-u mailcap；mv/etc/mime.types/tmp”

import (
	"bufio"
	"bytes"
	"flag"
	"html/template"
	"io/ioutil"
	"strings"

	"log"
)

var (
	typesFlag   = flag.String("types", "", "Input mime.types file")
	packageFlag = flag.String("package", "", "Golang package in output file")
	outFlag     = flag.String("out", "", "Output file name for the generated mime types")
)

type mime struct {
	Name string
	Exts []string
}

type templateParams struct {
	PackageName string
	Mimes       []mime
}

func main() {
//分析并确保指定了所有需要的输入
	flag.Parse()
	if *typesFlag == "" {
		log.Fatalf("--types is required")
	}
	if *packageFlag == "" {
		log.Fatalf("--types is required")
	}
	if *outFlag == "" {
		log.Fatalf("--out is required")
	}

	params := templateParams{
		PackageName: *packageFlag,
	}

	types, err := ioutil.ReadFile(*typesFlag)
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(types))
	for scanner.Scan() {
		txt := scanner.Text()
		if strings.HasPrefix(txt, "#") || len(txt) == 0 {
			continue
		}
		parts := strings.Fields(txt)
		if len(parts) == 1 {
			continue
		}
		params.Mimes = append(params.Mimes, mime{parts[0], parts[1:]})
	}

	if err = scanner.Err(); err != nil {
		log.Fatal(err)
	}

	result := bytes.NewBuffer([]byte{})

	if err := template.Must(template.New("_").Parse(tpl)).Execute(result, params); err != nil {
		log.Fatal(err)
	}

	if err := ioutil.WriteFile(*outFlag, result.Bytes(), 0600); err != nil {
		log.Fatal(err)
	}
}

var tpl = `//代码由github.com/ethereum/go-ethereum/cmd/swarm/mimegen生成。不要编辑。

package {{ .PackageName }}

import "mime"
func init() {
	var mimeTypes = map[string]string{
{{- range .Mimes -}}
	{{ $name := .Name -}}
	{{- range .Exts }}
		".{{ . }}": "{{ $name | html }}",
	{{- end }}
{{- end }}
	}
	for ext, name := range mimeTypes {
		if err := mime.AddExtensionType(ext, name); err != nil {
			panic(err)
		}
	}
}
`
