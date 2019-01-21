
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

package asm

import (
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/vm"
)

//编译器包含有关已分析源的信息
//并持有程序的令牌。
type Compiler struct {
	tokens []token
	binary []interface{}

	labels map[string]int

	pc, pos int

	debug bool
}

//NewCompiler返回新分配的编译器。
func NewCompiler(debug bool) *Compiler {
	return &Compiler{
		labels: make(map[string]int),
		debug:  debug,
	}
}

//feed将令牌馈送到ch，并由
//编译器。
//
//feed是编译阶段的第一个步骤，因为它
//收集程序中使用的标签并保留
//用于确定位置的程序计数器
//跳跃的目的地。标签不能用于
//第二阶段推标签并确定正确的
//位置。
func (c *Compiler) Feed(ch <-chan token) {
	for i := range ch {
		switch i.typ {
		case number:
			num := math.MustParseBig256(i.text).Bytes()
			if len(num) == 0 {
				num = []byte{0}
			}
			c.pc += len(num)
		case stringValue:
			c.pc += len(i.text) - 2
		case element:
			c.pc++
		case labelDef:
			c.labels[i.text] = c.pc
			c.pc++
		case label:
			c.pc += 5
		}

		c.tokens = append(c.tokens, i)
	}
	if c.debug {
		fmt.Fprintln(os.Stderr, "found", len(c.labels), "labels")
	}
}

//编译编译当前令牌并返回一个
//可由EVM解释的二进制字符串
//如果失败了就会出错。
//
//编译是编译阶段的第二个阶段
//它将令牌编译为EVM指令。
func (c *Compiler) Compile() (string, []error) {
	var errors []error
//继续循环令牌，直到
//堆栈已耗尽。
	for c.pos < len(c.tokens) {
		if err := c.compileLine(); err != nil {
			errors = append(errors, err)
		}
	}

//将二进制转换为十六进制
	var bin string
	for _, v := range c.binary {
		switch v := v.(type) {
		case vm.OpCode:
			bin += fmt.Sprintf("%x", []byte{byte(v)})
		case []byte:
			bin += fmt.Sprintf("%x", v)
		}
	}
	return bin, errors
}

//next返回下一个标记并递增
//位置。
func (c *Compiler) next() token {
	token := c.tokens[c.pos]
	c.pos++
	return token
}

//compileline编译单行指令，例如
//“push 1”，“jump@label”。
func (c *Compiler) compileLine() error {
	n := c.next()
	if n.typ != lineStart {
		return compileErr(n, n.typ.String(), lineStart.String())
	}

	lvalue := c.next()
	switch lvalue.typ {
	case eof:
		return nil
	case element:
		if err := c.compileElement(lvalue); err != nil {
			return err
		}
	case labelDef:
		c.compileLabel()
	case lineEnd:
		return nil
	default:
		return compileErr(lvalue, lvalue.text, fmt.Sprintf("%v or %v", labelDef, element))
	}

	if n := c.next(); n.typ != lineEnd {
		return compileErr(n, n.text, lineEnd.String())
	}

	return nil
}

//compileNumber将数字编译为字节
func (c *Compiler) compileNumber(element token) (int, error) {
	num := math.MustParseBig256(element.text).Bytes()
	if len(num) == 0 {
		num = []byte{0}
	}
	c.pushBin(num)
	return len(num), nil
}

//compileElement编译元素（push&label或两者兼有）
//以二进制表示，如果语句不正确，则可能出错。
//喂的地方。
func (c *Compiler) compileElement(element token) error {
//
//
	if isJump(element.text) {
		rvalue := c.next()
		switch rvalue.typ {
		case number:
//TODO了解如何正确返回错误
			c.compileNumber(rvalue)
		case stringValue:
//字符串被引用，请删除它们。
			c.pushBin(rvalue.text[1 : len(rvalue.text)-2])
		case label:
			c.pushBin(vm.PUSH4)
			pos := big.NewInt(int64(c.labels[rvalue.text])).Bytes()
			pos = append(make([]byte, 4-len(pos)), pos...)
			c.pushBin(pos)
		default:
			return compileErr(rvalue, rvalue.text, "number, string or label")
		}
//推动操作
		c.pushBin(toBinary(element.text))
		return nil
	} else if isPush(element.text) {
//把手推。按从左到右读取。
		var value []byte

		rvalue := c.next()
		switch rvalue.typ {
		case number:
			value = math.MustParseBig256(rvalue.text).Bytes()
			if len(value) == 0 {
				value = []byte{0}
			}
		case stringValue:
			value = []byte(rvalue.text[1 : len(rvalue.text)-1])
		case label:
			value = make([]byte, 4)
			copy(value, big.NewInt(int64(c.labels[rvalue.text])).Bytes())
		default:
			return compileErr(rvalue, rvalue.text, "number, string or label")
		}

		if len(value) > 32 {
			return fmt.Errorf("%d type error: unsupported string or number with size > 32", rvalue.lineno)
		}

		c.pushBin(vm.OpCode(int(vm.PUSH1) - 1 + len(value)))
		c.pushBin(value)
	} else {
		c.pushBin(toBinary(element.text))
	}

	return nil
}

//compileLabel将JumpDest推送到二进制切片。
func (c *Compiler) compileLabel() {
	c.pushBin(vm.JUMPDEST)
}

//推杆将值V推送到二进制堆栈。
func (c *Compiler) pushBin(v interface{}) {
	if c.debug {
		fmt.Printf("%d: %v\n", len(c.binary), v)
	}
	c.binary = append(c.binary, v)
}

//ispush返回字符串op是否为
//推（n）。
func isPush(op string) bool {
	return strings.ToUpper(op) == "PUSH"
}

//is jump返回字符串op是否为jump（i）
func isJump(op string) bool {
	return strings.ToUpper(op) == "JUMPI" || strings.ToUpper(op) == "JUMP"
}

//ToBinary将文本转换为vm.opcode
func toBinary(text string) vm.OpCode {
	return vm.StringToOp(strings.ToUpper(text))
}

type compileError struct {
	got  string
	want string

	lineno int
}

func (err compileError) Error() string {
	return fmt.Sprintf("%d syntax error: unexpected %v, expected %v", err.lineno, err.got, err.want)
}

func compileErr(c token, got, want string) error {
	return compileError{
		got:    got,
		want:   want,
		lineno: c.lineno,
	}
}
