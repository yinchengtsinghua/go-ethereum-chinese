
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

//包绑定生成以太坊契约go绑定。
//
//有关详细的使用文档和教程，请访问以太坊wiki页面：
//https://github.com/ethereum/go-ethereum/wiki/native-dapps:-转到绑定到ethereum合同
package bind

import (
	"bytes"
	"fmt"
	"go/format"
	"regexp"
	"strings"
	"text/template"
	"unicode"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

//lang是为其生成绑定的目标编程语言选择器。
type Lang int

const (
	LangGo Lang = iota
	LangJava
	LangObjC
)

//bind围绕一个契约abi生成一个go包装器。这个包装不代表
//在客户端代码中使用，而不是作为中间结构使用，
//强制执行编译时类型安全和命名约定，而不是必须
//手动维护在运行时中断的硬编码字符串。
func Bind(types []string, abis []string, bytecodes []string, pkg string, lang Lang) (string, error) {
//处理每个要求约束的单独合同
	contracts := make(map[string]*tmplContract)

	for i := 0; i < len(types); i++ {
//分析实际的ABI以生成绑定
		evmABI, err := abi.JSON(strings.NewReader(abis[i]))
		if err != nil {
			return "", err
		}
//从JSONABI中删除任何空白
		strippedABI := strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, abis[i])

//提取调用和事务处理方法；事件；并按字母顺序排序
		var (
			calls     = make(map[string]*tmplMethod)
			transacts = make(map[string]*tmplMethod)
			events    = make(map[string]*tmplEvent)
		)
		for _, original := range evmABI.Methods {
//规范资本案例和非匿名输入/输出的方法
			normalized := original
			normalized.Name = methodNormalizer[lang](original.Name)

			normalized.Inputs = make([]abi.Argument, len(original.Inputs))
			copy(normalized.Inputs, original.Inputs)
			for j, input := range normalized.Inputs {
				if input.Name == "" {
					normalized.Inputs[j].Name = fmt.Sprintf("arg%d", j)
				}
			}
			normalized.Outputs = make([]abi.Argument, len(original.Outputs))
			copy(normalized.Outputs, original.Outputs)
			for j, output := range normalized.Outputs {
				if output.Name != "" {
					normalized.Outputs[j].Name = capitalise(output.Name)
				}
			}
//将方法附加到调用或事务处理列表中
			if original.Const {
				calls[original.Name] = &tmplMethod{Original: original, Normalized: normalized, Structured: structured(original.Outputs)}
			} else {
				transacts[original.Name] = &tmplMethod{Original: original, Normalized: normalized, Structured: structured(original.Outputs)}
			}
		}
		for _, original := range evmABI.Events {
//跳过匿名事件，因为它们不支持显式筛选
			if original.Anonymous {
				continue
			}
//规范资本案例和非匿名输出的事件
			normalized := original
			normalized.Name = methodNormalizer[lang](original.Name)

			normalized.Inputs = make([]abi.Argument, len(original.Inputs))
			copy(normalized.Inputs, original.Inputs)
			for j, input := range normalized.Inputs {
//索引字段是输入，非索引字段是输出
				if input.Indexed {
					if input.Name == "" {
						normalized.Inputs[j].Name = fmt.Sprintf("arg%d", j)
					}
				}
			}
//将事件追加到累加器列表
			events[original.Name] = &tmplEvent{Original: original, Normalized: normalized}
		}
		contracts[types[i]] = &tmplContract{
			Type:        capitalise(types[i]),
			InputABI:    strings.Replace(strippedABI, "\"", "\\\"", -1),
			InputBin:    strings.TrimSpace(bytecodes[i]),
			Constructor: evmABI.Constructor,
			Calls:       calls,
			Transacts:   transacts,
			Events:      events,
		}
	}
//生成合同模板数据内容并呈现
	data := &tmplData{
		Package:   pkg,
		Contracts: contracts,
	}
	buffer := new(bytes.Buffer)

	funcs := map[string]interface{}{
		"bindtype":      bindType[lang],
		"bindtopictype": bindTopicType[lang],
		"namedtype":     namedType[lang],
		"capitalise":    capitalise,
		"decapitalise":  decapitalise,
	}
	tmpl := template.Must(template.New("").Funcs(funcs).Parse(tmplSource[lang]))
	if err := tmpl.Execute(buffer, data); err != nil {
		return "", err
	}
//对于go绑定，通过gofmt传递代码以清除它
	if lang == LangGo {
		code, err := format.Source(buffer.Bytes())
		if err != nil {
			return "", fmt.Errorf("%v\n%s", err, buffer)
		}
		return string(code), nil
	}
//对于所有其他人来说，现在就按原样返回
	return buffer.String(), nil
}

//bindtype是一组类型绑定器，可以将solidity类型转换为支持的类型
//编程语言类型。
var bindType = map[Lang]func(kind abi.Type) string{
	LangGo:   bindTypeGo,
	LangJava: bindTypeJava,
}

//绑定生成器的助手函数。
//在内部类型匹配后读取不匹配的字符，
//（因为内部类型是total类型声明的前缀），
//查找包装内部类型的有效数组（可能是动态数组），
//并返回这些数组的大小。
//
//返回的数组大小与solidity签名的顺序相同；首先是内部数组大小。
//数组大小也可以为“”，表示动态数组。
func wrapArray(stringKind string, innerLen int, innerMapping string) (string, []string) {
	remainder := stringKind[innerLen:]
//找到所有尺寸
	matches := regexp.MustCompile(`\[(\d*)\]`).FindAllStringSubmatch(remainder, -1)
	parts := make([]string, 0, len(matches))
	for _, match := range matches {
//从正则表达式匹配中获取组1
		parts = append(parts, match[1])
	}
	return innerMapping, parts
}

//将数组大小转换为内部类型（嵌套）数组的Go-Lang声明。
//如果arraysizes为空，只返回内部类型。
func arrayBindingGo(inner string, arraySizes []string) string {
	out := ""
//预先处理所有阵列大小，从外部（结束阵列大小）到内部（开始阵列大小）
	for i := len(arraySizes) - 1; i >= 0; i-- {
		out += "[" + arraySizes[i] + "]"
	}
	out += inner
	return out
}

//bindtypego将solidity类型转换为go类型。因为没有清晰的地图
//从所有solidity类型到go类型（例如uint17），那些不能精确
//mapped将使用升序类型（例如*big.int）。
func bindTypeGo(kind abi.Type) string {
	stringKind := kind.String()
	innerLen, innerMapping := bindUnnestedTypeGo(stringKind)
	return arrayBindingGo(wrapArray(stringKind, innerLen, innerMapping))
}

//bindtypego的内部函数，它查找StringKind的内部类型。
//（或者，如果类型本身不是数组或切片，则仅限于该类型本身）
//将返回匹配部分的长度和转换后的类型。
func bindUnnestedTypeGo(stringKind string) (int, string) {

	switch {
	case strings.HasPrefix(stringKind, "address"):
		return len("address"), "common.Address"

	case strings.HasPrefix(stringKind, "bytes"):
		parts := regexp.MustCompile(`bytes([0-9]*)`).FindStringSubmatch(stringKind)
		return len(parts[0]), fmt.Sprintf("[%s]byte", parts[1])

	case strings.HasPrefix(stringKind, "int") || strings.HasPrefix(stringKind, "uint"):
		parts := regexp.MustCompile(`(u)?int([0-9]*)`).FindStringSubmatch(stringKind)
		switch parts[2] {
		case "8", "16", "32", "64":
			return len(parts[0]), fmt.Sprintf("%sint%s", parts[1], parts[2])
		}
		return len(parts[0]), "*big.Int"

	case strings.HasPrefix(stringKind, "bool"):
		return len("bool"), "bool"

	case strings.HasPrefix(stringKind, "string"):
		return len("string"), "string"

	default:
		return len(stringKind), stringKind
	}
}

//将数组大小转换为内部类型（嵌套）数组的Java声明。
//如果arraysizes为空，只返回内部类型。
func arrayBindingJava(inner string, arraySizes []string) string {
//Java数组类型声明不包括长度。
	return inner + strings.Repeat("[]", len(arraySizes))
}

//bdType Java将一个稳固类型转换为Java类型。因为没有清晰的地图
//从所有Solidity类型到Java类型（例如UIT17），那些不能精确的类型。
//mapped将使用升序类型（例如bigdecimal）。
func bindTypeJava(kind abi.Type) string {
	stringKind := kind.String()
	innerLen, innerMapping := bindUnnestedTypeJava(stringKind)
	return arrayBindingJava(wrapArray(stringKind, innerLen, innerMapping))
}

//bindtypejava的内部函数，它查找StringKind的内部类型。
//（或者，如果类型本身不是数组或切片，则仅限于该类型本身）
//将返回匹配部分的长度和转换后的类型。
func bindUnnestedTypeJava(stringKind string) (int, string) {

	switch {
	case strings.HasPrefix(stringKind, "address"):
		parts := regexp.MustCompile(`address(\[[0-9]*\])?`).FindStringSubmatch(stringKind)
		if len(parts) != 2 {
			return len(stringKind), stringKind
		}
		if parts[1] == "" {
			return len("address"), "Address"
		}
		return len(parts[0]), "Addresses"

	case strings.HasPrefix(stringKind, "bytes"):
		parts := regexp.MustCompile(`bytes([0-9]*)`).FindStringSubmatch(stringKind)
		if len(parts) != 2 {
			return len(stringKind), stringKind
		}
		return len(parts[0]), "byte[]"

	case strings.HasPrefix(stringKind, "int") || strings.HasPrefix(stringKind, "uint"):
//注意，uint和int（不带数字）也匹配，
//它们的大小为256，将转换为bigint（默认值）。
		parts := regexp.MustCompile(`(u)?int([0-9]*)`).FindStringSubmatch(stringKind)
		if len(parts) != 3 {
			return len(stringKind), stringKind
		}

		namedSize := map[string]string{
			"8":  "byte",
			"16": "short",
			"32": "int",
			"64": "long",
		}[parts[2]]

//默认为bigint
		if namedSize == "" {
			namedSize = "BigInt"
		}
		return len(parts[0]), namedSize

	case strings.HasPrefix(stringKind, "bool"):
		return len("bool"), "boolean"

	case strings.HasPrefix(stringKind, "string"):
		return len("string"), "String"

	default:
		return len(stringKind), stringKind
	}
}

//bindtopictype是一组类型绑定器，将solidity类型转换为
//支持的编程语言主题类型。
var bindTopicType = map[Lang]func(kind abi.Type) string{
	LangGo:   bindTopicTypeGo,
	LangJava: bindTopicTypeJava,
}

//bindtypego将solidity主题类型转换为go类型。几乎是一样的
//功能与简单类型相同，但动态类型转换为哈希。
func bindTopicTypeGo(kind abi.Type) string {
	bound := bindTypeGo(kind)
	if bound == "string" || bound == "[]byte" {
		bound = "common.Hash"
	}
	return bound
}

//bdIdTyGO将一个坚固性主题类型转换为Java主题类型。几乎是一样的
//功能与简单类型相同，但动态类型转换为哈希。
func bindTopicTypeJava(kind abi.Type) string {
	bound := bindTypeJava(kind)
	if bound == "String" || bound == "Bytes" {
		bound = "Hash"
	}
	return bound
}

//NamedType是一组将特定语言类型转换为
//方法名中使用的命名版本。
var namedType = map[Lang]func(string, abi.Type) string{
	LangGo:   func(string, abi.Type) string { panic("this shouldn't be needed") },
	LangJava: namedTypeJava,
}

//NamedTypeJava将一些基元数据类型转换为可以
//用作方法名称的一部分。
func namedTypeJava(javaKind string, solKind abi.Type) string {
	switch javaKind {
	case "byte[]":
		return "Binary"
	case "byte[][]":
		return "Binaries"
	case "string":
		return "String"
	case "string[]":
		return "Strings"
	case "boolean":
		return "Bool"
	case "boolean[]":
		return "Bools"
	case "BigInt[]":
		return "BigInts"
	default:
		parts := regexp.MustCompile(`(u)?int([0-9]*)(\[[0-9]*\])?`).FindStringSubmatch(solKind.String())
		if len(parts) != 4 {
			return javaKind
		}
		switch parts[2] {
		case "8", "16", "32", "64":
			if parts[3] == "" {
				return capitalise(fmt.Sprintf("%sint%s", parts[1], parts[2]))
			}
			return capitalise(fmt.Sprintf("%sint%ss", parts[1], parts[2]))

		default:
			return javaKind
		}
	}
}

//methodNormalizer是一个名称转换器，它将solidity方法名称修改为
//符合目标语言命名概念。
var methodNormalizer = map[Lang]func(string) string{
	LangGo:   abi.ToCamelCase,
	LangJava: decapitalise,
}

//capitale生成一个以大写字符开头的驼色大小写字符串。
func capitalise(input string) string {
	return abi.ToCamelCase(input)
}

//无头化生成一个以小写字符开头的驼色大小写字符串。
func decapitalise(input string) string {
	if len(input) == 0 {
		return input
	}

	goForm := abi.ToCamelCase(input)
	return strings.ToLower(goForm[:1]) + goForm[1:]
}

//结构化检查ABI数据类型列表是否有足够的信息
//通过适当的go结构或如果需要平面返回，则进行操作。
func structured(args abi.Arguments) bool {
	if len(args) < 2 {
		return false
	}
	exists := make(map[string]bool)
	for _, out := range args {
//如果名称是匿名的，则无法组织成结构
		if out.Name == "" {
			return false
		}
//如果规范化或冲突时字段名为空（var、var、_var、_var），
//我们不能组织成一个结构
		field := capitalise(out.Name)
		if field == "" || exists[field] {
			return false
		}
		exists[field] = true
	}
	return true
}
