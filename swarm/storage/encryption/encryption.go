
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

package encryption

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"hash"
	"sync"
)

const KeyLength = 32

type Key []byte

type Encryption interface {
	Encrypt(data []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
}

type encryption struct {
key      Key              //加密密钥（hashsize bytes long）
keyLen   int              //密钥长度=分组密码块的长度
padding  int              //如果大于0，加密会将数据填充到此
initCtr  uint32           //用于计数器模式块密码的初始计数器
hashFunc func() hash.Hash //哈希构造函数函数
}

//new构造新的加密/解密程序
func New(key Key, padding int, initCtr uint32, hashFunc func() hash.Hash) *encryption {
	return &encryption{
		key:      key,
		keyLen:   len(key),
		padding:  padding,
		initCtr:  initCtr,
		hashFunc: hashFunc,
	}
}

//Encrypt加密数据并在指定时进行填充
func (e *encryption) Encrypt(data []byte) ([]byte, error) {
	length := len(data)
	outLength := length
	isFixedPadding := e.padding > 0
	if isFixedPadding {
		if length > e.padding {
			return nil, fmt.Errorf("Data length longer than padding, data length %v padding %v", length, e.padding)
		}
		outLength = e.padding
	}
	out := make([]byte, outLength)
	e.transform(data, out)
	return out, nil
}

//decrypt解密数据，如果使用填充，调用方必须知道原始长度并截断
func (e *encryption) Decrypt(data []byte) ([]byte, error) {
	length := len(data)
	if e.padding > 0 && length != e.padding {
		return nil, fmt.Errorf("Data length different than padding, data length %v padding %v", length, e.padding)
	}
	out := make([]byte, length)
	e.transform(data, out)
	return out, nil
}

//
func (e *encryption) transform(in, out []byte) {
	inLength := len(in)
	wg := sync.WaitGroup{}
	wg.Add((inLength-1)/e.keyLen + 1)
	for i := 0; i < inLength; i += e.keyLen {
		l := min(e.keyLen, inLength-i)
//每段调用转换（异步）
		go func(i int, x, y []byte) {
			defer wg.Done()
			e.Transcrypt(i, x, y)
		}(i/e.keyLen, in[i:i+l], out[i:i+l])
	}
//如果出局时间较长，则补上其余部分。
	pad(out[inLength:])
	wg.Wait()
}

//用于分段转换
//如果输入短于输出，则使用填充
func (e *encryption) Transcrypt(i int, in []byte, out []byte) {
//带计数器的第一个哈希键（初始计数器+I）
	hasher := e.hashFunc()
	hasher.Write(e.key)

	ctrBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(ctrBytes, uint32(i)+e.initCtr)
	hasher.Write(ctrBytes)

	ctrHash := hasher.Sum(nil)
	hasher.Reset()

//第二轮哈希选择披露
	hasher.Write(ctrHash)
	segmentKey := hasher.Sum(nil)
	hasher.Reset()

//输入的XOR字节正常运行时间长度（输出必须至少与之相同）
	inLength := len(in)
	for j := 0; j < inLength; j++ {
		out[j] = in[j] ^ segmentKey[j]
	}
//如果超出长度，则插入填充
	pad(out[inLength:])
}

func pad(b []byte) {
	l := len(b)
	for total := 0; total < l; {
		read, _ := rand.Read(b[total:])
		total += read
	}
}

//GenerateRandomKey生成长度为l的随机键
func GenerateRandomKey(l int) Key {
	key := make([]byte, l)
	var total int
	for total < l {
		read, _ := rand.Read(key[total:])
		total += read
	}
	return key
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
