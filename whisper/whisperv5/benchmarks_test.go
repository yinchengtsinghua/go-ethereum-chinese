
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

package whisperv5

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func BenchmarkDeriveKeyMaterial(b *testing.B) {
	for i := 0; i < b.N; i++ {
		deriveKeyMaterial([]byte("test"), 0)
	}
}

func BenchmarkEncryptionSym(b *testing.B) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		b.Fatalf("failed generateMessageParams with seed %d: %s.", seed, err)
	}

	for i := 0; i < b.N; i++ {
		msg, _ := NewSentMessage(params)
		_, err := msg.Wrap(params)
		if err != nil {
			b.Errorf("failed Wrap with seed %d: %s.", seed, err)
			b.Errorf("i = %d, len(msg.Raw) = %d, params.Payload = %d.", i, len(msg.Raw), len(params.Payload))
			return
		}
	}
}

func BenchmarkEncryptionAsym(b *testing.B) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		b.Fatalf("failed generateMessageParams with seed %d: %s.", seed, err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		b.Fatalf("failed GenerateKey with seed %d: %s.", seed, err)
	}
	params.KeySym = nil
	params.Dst = &key.PublicKey

	for i := 0; i < b.N; i++ {
		msg, _ := NewSentMessage(params)
		_, err := msg.Wrap(params)
		if err != nil {
			b.Fatalf("failed Wrap with seed %d: %s.", seed, err)
		}
	}
}

func BenchmarkDecryptionSymValid(b *testing.B) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		b.Fatalf("failed generateMessageParams with seed %d: %s.", seed, err)
	}
	msg, _ := NewSentMessage(params)
	env, err := msg.Wrap(params)
	if err != nil {
		b.Fatalf("failed Wrap with seed %d: %s.", seed, err)
	}
	f := Filter{KeySym: params.KeySym}

	for i := 0; i < b.N; i++ {
		msg := env.Open(&f)
		if msg == nil {
			b.Fatalf("failed to open with seed %d.", seed)
		}
	}
}

func BenchmarkDecryptionSymInvalid(b *testing.B) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		b.Fatalf("failed generateMessageParams with seed %d: %s.", seed, err)
	}
	msg, _ := NewSentMessage(params)
	env, err := msg.Wrap(params)
	if err != nil {
		b.Fatalf("failed Wrap with seed %d: %s.", seed, err)
	}
	f := Filter{KeySym: []byte("arbitrary stuff here")}

	for i := 0; i < b.N; i++ {
		msg := env.Open(&f)
		if msg != nil {
			b.Fatalf("opened envelope with invalid key, seed: %d.", seed)
		}
	}
}

func BenchmarkDecryptionAsymValid(b *testing.B) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		b.Fatalf("failed generateMessageParams with seed %d: %s.", seed, err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		b.Fatalf("failed GenerateKey with seed %d: %s.", seed, err)
	}
	f := Filter{KeyAsym: key}
	params.KeySym = nil
	params.Dst = &key.PublicKey
	msg, _ := NewSentMessage(params)
	env, err := msg.Wrap(params)
	if err != nil {
		b.Fatalf("failed Wrap with seed %d: %s.", seed, err)
	}

	for i := 0; i < b.N; i++ {
		msg := env.Open(&f)
		if msg == nil {
			b.Fatalf("fail to open, seed: %d.", seed)
		}
	}
}

func BenchmarkDecryptionAsymInvalid(b *testing.B) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		b.Fatalf("failed generateMessageParams with seed %d: %s.", seed, err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		b.Fatalf("failed GenerateKey with seed %d: %s.", seed, err)
	}
	params.KeySym = nil
	params.Dst = &key.PublicKey
	msg, _ := NewSentMessage(params)
	env, err := msg.Wrap(params)
	if err != nil {
		b.Fatalf("failed Wrap with seed %d: %s.", seed, err)
	}

	key, err = crypto.GenerateKey()
	if err != nil {
		b.Fatalf("failed GenerateKey with seed %d: %s.", seed, err)
	}
	f := Filter{KeyAsym: key}

	for i := 0; i < b.N; i++ {
		msg := env.Open(&f)
		if msg != nil {
			b.Fatalf("opened envelope with invalid key, seed: %d.", seed)
		}
	}
}

func increment(x []byte) {
	for i := 0; i < len(x); i++ {
		x[i]++
		if x[i] != 0 {
			break
		}
	}
}

func BenchmarkPoW(b *testing.B) {
	InitSingleTest()

	params, err := generateMessageParams()
	if err != nil {
		b.Fatalf("failed generateMessageParams with seed %d: %s.", seed, err)
	}
	params.Payload = make([]byte, 32)
	params.PoW = 10.0
	params.TTL = 1

	for i := 0; i < b.N; i++ {
		increment(params.Payload)
		msg, _ := NewSentMessage(params)
		_, err := msg.Wrap(params)
		if err != nil {
			b.Fatalf("failed Wrap with seed %d: %s.", seed, err)
		}
	}
}
