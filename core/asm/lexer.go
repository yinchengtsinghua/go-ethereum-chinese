
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
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

//statefn在
//lexer解析
//当前状态。
type stateFn func(*lexer) stateFn

//当lexer发现
//一种新的可分割代币。这些都送过去了
//词汇的标记通道
type token struct {
	typ    tokenType
	lineno int
	text   string
}

//tokentype是lexer的不同类型
//能够解析并返回。
type tokenType int

const (
eof              tokenType = iota //文件结束
lineStart                         //行开始时发出
lineEnd                           //行结束时发出
invalidStatement                  //任何无效的语句
element                           //元素分析期间的任何元素
label                             //找到标签时发出标签
labelDef                          //找到新标签时发出标签定义
number                            //找到数字时发出数字
stringValue                       //当找到字符串时，将发出StringValue

Numbers            = "1234567890"                                           //表示任何十进制数的字符
HexadecimalNumbers = Numbers + "aAbBcCdDeEfF"                               //表示任何十六进制的字符
Alpha              = "abcdefghijklmnopqrstuwvxyzABCDEFGHIJKLMNOPQRSTUWVXYZ" //表示字母数字的字符
)

//字符串实现字符串
func (it tokenType) String() string {
	if int(it) > len(stringtokenTypes) {
		return "invalid"
	}
	return stringtokenTypes[it]
}

var stringtokenTypes = []string{
	eof:              "EOF",
	invalidStatement: "invalid statement",
	element:          "element",
	lineEnd:          "end of line",
	lineStart:        "new line",
	label:            "label",
	labelDef:         "label definition",
	number:           "number",
	stringValue:      "string",
}

//lexer是解析的基本构造
//源代码并将其转换为令牌。
//标记由编译器解释。
type lexer struct {
input string //输入包含程序的源代码

tokens chan token //令牌用于将令牌传递给侦听器
state  stateFn    //当前状态函数

lineno            int //源文件中的当前行号
start, pos, width int //词法和返回值的位置

debug bool //触发调试输出的标志
}

//lex使用给定的源按名称对程序进行lex。它返回一个
//传递令牌的通道。
func Lex(name string, source []byte, debug bool) <-chan token {
	ch := make(chan token)
	l := &lexer{
		input:  string(source),
		tokens: ch,
		state:  lexLine,
		debug:  debug,
	}
	go func() {
		l.emit(lineStart)
		for l.state != nil {
			l.state = l.state(l)
		}
		l.emit(eof)
		close(l.tokens)
	}()

	return ch
}

//next返回程序源中的下一个rune。
func (l *lexer) next() (rune rune) {
	if l.pos >= len(l.input) {
		l.width = 0
		return 0
	}
	rune, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	return rune
}

//备份备份最后一个已分析的元素（多字符）
func (l *lexer) backup() {
	l.pos -= l.width
}

//Peek返回下一个符文，但不前进搜索者
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

//忽略推进搜索者并忽略值
func (l *lexer) ignore() {
	l.start = l.pos
}

//接受检查给定输入是否与下一个rune匹配
func (l *lexer) accept(valid string) bool {
	if strings.ContainsRune(valid, l.next()) {
		return true
	}

	l.backup()

	return false
}

//acceptrun将继续推进seeker直到有效
//再也见不到了。
func (l *lexer) acceptRun(valid string) {
	for strings.ContainsRune(valid, l.next()) {
	}
	l.backup()
}

//acceptrununtil是acceptrun的倒数，将继续
//把探索者推进直到找到符文。
func (l *lexer) acceptRunUntil(until rune) bool {
//继续运行，直到找到符文
	for i := l.next(); !strings.ContainsRune(string(until), i); i = l.next() {
		if i == 0 {
			return false
		}
	}

	return true
}

//blob返回当前值
func (l *lexer) blob() string {
	return l.input[l.start:l.pos]
}

//向令牌通道发送新令牌以进行处理
func (l *lexer) emit(t tokenType) {
	token := token{t, l.lineno, l.blob()}

	if l.debug {
		fmt.Fprintf(os.Stderr, "%04d: (%-20v) %s\n", token.lineno, token.typ, token.text)
	}

	l.tokens <- token
	l.start = l.pos
}

//lexline是用于词法处理行的状态函数
func lexLine(l *lexer) stateFn {
	for {
		switch r := l.next(); {
		case r == '\n':
			l.emit(lineEnd)
			l.ignore()
			l.lineno++

			l.emit(lineStart)
		case r == ';' && l.peek() == ';':
			return lexComment
		case isSpace(r):
			l.ignore()
		case isLetter(r) || r == '_':
			return lexElement
		case isNumber(r):
			return lexNumber
		case r == '@':
			l.ignore()
			return lexLabel
		case r == '"':
			return lexInsideString
		default:
			return nil
		}
	}
}

//lexcomment分析当前位置直到结束
//并丢弃文本。
func lexComment(l *lexer) stateFn {
	l.acceptRunUntil('\n')
	l.ignore()

	return lexLine
}

//lexmlabel解析当前标签，发出并返回
//lex文本状态函数用于推进分析
//过程。
func lexLabel(l *lexer) stateFn {
	l.acceptRun(Alpha + "_")

	l.emit(label)

	return lexLine
}

//lexinsideString对字符串内部进行lexi，直到
//state函数查找右引号。
//它返回lex文本状态函数。
func lexInsideString(l *lexer) stateFn {
	if l.acceptRunUntil('"') {
		l.emit(stringValue)
	}

	return lexLine
}

func lexNumber(l *lexer) stateFn {
	acceptance := Numbers
	if l.accept("0") || l.accept("xX") {
		acceptance = HexadecimalNumbers
	}
	l.acceptRun(acceptance)

	l.emit(number)

	return lexLine
}

func lexElement(l *lexer) stateFn {
	l.acceptRun(Alpha + "_" + Numbers)

	if l.peek() == ':' {
		l.emit(labelDef)

		l.accept(":")
		l.ignore()
	} else {
		l.emit(element)
	}
	return lexLine
}

func isLetter(t rune) bool {
	return unicode.IsLetter(t)
}

func isSpace(t rune) bool {
	return unicode.IsSpace(t)
}

func isNumber(t rune) bool {
	return unicode.IsNumber(t)
}
