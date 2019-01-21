
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

//包含与Whisper协议信封对象关联的测试。

package whisperv6

import (
	mrand "math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestEnvelopeOpenAcceptsOnlyOneKeyTypeInFilter(t *testing.T) {
	symKey := make([]byte, aesKeyLength)
	mrand.Read(symKey)

	asymKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed GenerateKey with seed %d: %s.", seed, err)
	}

	params := MessageParams{
		PoW:      0.01,
		WorkTime: 1,
		TTL:      uint32(mrand.Intn(1024)),
		Payload:  make([]byte, 50),
		KeySym:   symKey,
		Dst:      nil,
	}

	mrand.Read(params.Payload)

	msg, err := NewSentMessage(&params)
	if err != nil {
		t.Fatalf("failed to create new message with seed %d: %s.", seed, err)
	}

	e, err := msg.Wrap(&params)
	if err != nil {
		t.Fatalf("Failed to Wrap the message in an envelope with seed %d: %s", seed, err)
	}

	f := Filter{KeySym: symKey, KeyAsym: asymKey}

	decrypted := e.Open(&f)
	if decrypted != nil {
		t.Fatalf("Managed to decrypt a message with an invalid filter, seed %d", seed)
	}
}
