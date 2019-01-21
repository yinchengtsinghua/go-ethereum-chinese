
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2014 Go Ethereum作者
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

package trie

//trie键有三种不同的编码：
//
//keybytes编码包含实际的密钥，而不包含其他任何内容。此编码是
//大多数API函数的输入。
//
//十六进制编码包含键的每个半字节和可选尾随字节。
//值为0x10的“终止符”字节，指示键处的节点是否
//包含一个值。十六进制键编码用于内存中加载的节点，因为它
//方便进入。
//
//紧凑型编码由以太坊黄纸（称为“十六进制前缀”）定义
//并包含密钥的字节和标志。高尖的
//第一个字节包含标志；编码长度的奇数和
//第二个最低的编码键处的节点是否为值节点。低啃咬
//对于偶数个半字节和第一个半字节，第一个字节的为零。
//如果是奇数。所有剩余的笔尖（现在是偶数）都合适
//到剩余的字节。压缩编码用于存储在磁盘上的节点。

func hexToCompact(hex []byte) []byte {
	terminator := byte(0)
	if hasTerm(hex) {
		terminator = 1
		hex = hex[:len(hex)-1]
	}
	buf := make([]byte, len(hex)/2+1)
buf[0] = terminator << 5 //标志字节
	if len(hex)&1 == 1 {
buf[0] |= 1 << 4 //奇数旗
buf[0] |= hex[0] //第一个半字节包含在第一个字节中
		hex = hex[1:]
	}
	decodeNibbles(hex, buf[1:])
	return buf
}

func compactToHex(compact []byte) []byte {
	base := keybytesToHex(compact)
//删除终止符标志
	if base[0] < 2 {
		base = base[:len(base)-1]
	}
//应用奇数旗
	chop := 2 - base[0]&1
	return base[chop:]
}

func keybytesToHex(str []byte) []byte {
	l := len(str)*2 + 1
	var nibbles = make([]byte, l)
	for i, b := range str {
		nibbles[i*2] = b / 16
		nibbles[i*2+1] = b % 16
	}
	nibbles[l-1] = 16
	return nibbles
}

//十六进制字节将十六进制字节转换为键字节。
//这只能用于长度均匀的键。
func hexToKeybytes(hex []byte) []byte {
	if hasTerm(hex) {
		hex = hex[:len(hex)-1]
	}
	if len(hex)&1 != 0 {
		panic("can't convert hex key of odd length")
	}
	key := make([]byte, len(hex)/2)
	decodeNibbles(hex, key)
	return key
}

func decodeNibbles(nibbles []byte, bytes []byte) {
	for bi, ni := 0, 0; ni < len(nibbles); bi, ni = bi+1, ni+2 {
		bytes[bi] = nibbles[ni]<<4 | nibbles[ni+1]
	}
}

//prefixlen返回A和B的公共前缀的长度。
func prefixLen(a, b []byte) int {
	var i, length = 0, len(a)
	if len(b) < length {
		length = len(b)
	}
	for ; i < length; i++ {
		if a[i] != b[i] {
			break
		}
	}
	return i
}

//hasterm返回十六进制键是否具有终止符标志。
func hasTerm(s []byte) bool {
	return len(s) > 0 && s[len(s)-1] == 16
}
