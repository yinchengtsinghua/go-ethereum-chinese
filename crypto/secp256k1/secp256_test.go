
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Jeffrey Wilcke、Felix Lange、Gustav Simonsson。版权所有。
//此源代码的使用受BSD样式许可证的控制，该许可证可在
//许可证文件。

package secp256k1

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"io"
	"testing"
)

const TestCount = 1000

func generateKeyPair() (pubkey, privkey []byte) {
	key, err := ecdsa.GenerateKey(S256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	pubkey = elliptic.Marshal(S256(), key.X, key.Y)

	privkey = make([]byte, 32)
	blob := key.D.Bytes()
	copy(privkey[32-len(blob):], blob)

	return pubkey, privkey
}

func csprngEntropy(n int) []byte {
	buf := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		panic("reading from crypto/rand failed: " + err.Error())
	}
	return buf
}

func randSig() []byte {
	sig := csprngEntropy(65)
	sig[32] &= 0x70
	sig[64] %= 4
	return sig
}

//延展性试验
//签名ecdsa s值的最高位必须是0，在第33个字节中
func compactSigCheck(t *testing.T, sig []byte) {
	var b = int(sig[32])
	if b < 0 {
		t.Errorf("highest bit is negative: %d", b)
	}
	if ((b >> 7) == 1) != ((b & 0x80) == 0x80) {
		t.Errorf("highest bit: %d bit >> 7: %d", b, b>>7)
	}
	if (b & 0x80) == 0x80 {
		t.Errorf("highest bit: %d bit & 0x80: %d", b, b&0x80)
	}
}

func TestSignatureValidity(t *testing.T) {
	pubkey, seckey := generateKeyPair()
	msg := csprngEntropy(32)
	sig, err := Sign(msg, seckey)
	if err != nil {
		t.Errorf("signature error: %s", err)
	}
	compactSigCheck(t, sig)
	if len(pubkey) != 65 {
		t.Errorf("pubkey length mismatch: want: 65 have: %d", len(pubkey))
	}
	if len(seckey) != 32 {
		t.Errorf("seckey length mismatch: want: 32 have: %d", len(seckey))
	}
	if len(sig) != 65 {
		t.Errorf("sig length mismatch: want: 65 have: %d", len(sig))
	}
	recid := int(sig[64])
	if recid > 4 || recid < 0 {
		t.Errorf("sig recid mismatch: want: within 0 to 4 have: %d", int(sig[64]))
	}
}

func TestInvalidRecoveryID(t *testing.T) {
	_, seckey := generateKeyPair()
	msg := csprngEntropy(32)
	sig, _ := Sign(msg, seckey)
	sig[64] = 99
	_, err := RecoverPubkey(msg, sig)
	if err != ErrInvalidRecoveryID {
		t.Fatalf("got %q, want %q", err, ErrInvalidRecoveryID)
	}
}

func TestSignAndRecover(t *testing.T) {
	pubkey1, seckey := generateKeyPair()
	msg := csprngEntropy(32)
	sig, err := Sign(msg, seckey)
	if err != nil {
		t.Errorf("signature error: %s", err)
	}
	pubkey2, err := RecoverPubkey(msg, sig)
	if err != nil {
		t.Errorf("recover error: %s", err)
	}
	if !bytes.Equal(pubkey1, pubkey2) {
		t.Errorf("pubkey mismatch: want: %x have: %x", pubkey1, pubkey2)
	}
}

func TestSignDeterministic(t *testing.T) {
	_, seckey := generateKeyPair()
	msg := make([]byte, 32)
	copy(msg, "hi there")

	sig1, err := Sign(msg, seckey)
	if err != nil {
		t.Fatal(err)
	}
	sig2, err := Sign(msg, seckey)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sig1, sig2) {
		t.Fatal("signatures not equal")
	}
}

func TestRandomMessagesWithSameKey(t *testing.T) {
	pubkey, seckey := generateKeyPair()
	keys := func() ([]byte, []byte) {
		return pubkey, seckey
	}
	signAndRecoverWithRandomMessages(t, keys)
}

func TestRandomMessagesWithRandomKeys(t *testing.T) {
	keys := func() ([]byte, []byte) {
		pubkey, seckey := generateKeyPair()
		return pubkey, seckey
	}
	signAndRecoverWithRandomMessages(t, keys)
}

func signAndRecoverWithRandomMessages(t *testing.T, keys func() ([]byte, []byte)) {
	for i := 0; i < TestCount; i++ {
		pubkey1, seckey := keys()
		msg := csprngEntropy(32)
		sig, err := Sign(msg, seckey)
		if err != nil {
			t.Fatalf("signature error: %s", err)
		}
		if sig == nil {
			t.Fatal("signature is nil")
		}
		compactSigCheck(t, sig)

//托多：我们为什么要翻转恢复ID？
		sig[len(sig)-1] %= 4

		pubkey2, err := RecoverPubkey(msg, sig)
		if err != nil {
			t.Fatalf("recover error: %s", err)
		}
		if pubkey2 == nil {
			t.Error("pubkey is nil")
		}
		if !bytes.Equal(pubkey1, pubkey2) {
			t.Fatalf("pubkey mismatch: want: %x have: %x", pubkey1, pubkey2)
		}
	}
}

func TestRecoveryOfRandomSignature(t *testing.T) {
	pubkey1, _ := generateKeyPair()
	msg := csprngEntropy(32)

	for i := 0; i < TestCount; i++ {
//恢复有时可以工作，但如果是这样，则应该始终给出错误的pubkey
		pubkey2, _ := RecoverPubkey(msg, randSig())
		if bytes.Equal(pubkey1, pubkey2) {
			t.Fatalf("iteration: %d: pubkey mismatch: do NOT want %x: ", i, pubkey2)
		}
	}
}

func TestRandomMessagesAgainstValidSig(t *testing.T) {
	pubkey1, seckey := generateKeyPair()
	msg := csprngEntropy(32)
	sig, _ := Sign(msg, seckey)

	for i := 0; i < TestCount; i++ {
		msg = csprngEntropy(32)
		pubkey2, _ := RecoverPubkey(msg, sig)
//恢复有时可以工作，但如果是这样，则应该始终给出错误的pubkey
		if bytes.Equal(pubkey1, pubkey2) {
			t.Fatalf("iteration: %d: pubkey mismatch: do NOT want %x: ", i, pubkey2)
		}
	}
}

//Useful when the underlying libsecp256k1 API changes to quickly
//只检查恢复函数而不使用签名函数
func TestRecoverSanity(t *testing.T) {
	msg, _ := hex.DecodeString("ce0677bb30baa8cf067c88db9811f4333d131bf8bcf12fe7065d211dce971008")
	sig, _ := hex.DecodeString("90f27b8b488db00b00606796d2987f6a5f59ae62ea05effe84fef5b8b0e549984a691139ad57a3f0b906637673aa2f63d1f55cb1a69199d4009eea23ceaddc9301")
	pubkey1, _ := hex.DecodeString("04e32df42865e97135acfb65f3bae71bdc86f4d49150ad6a440b6f15878109880a0a2b2667f7e725ceea70c673093bf67663e0312623c8e091b13cf2c0f11ef652")
	pubkey2, err := RecoverPubkey(msg, sig)
	if err != nil {
		t.Fatalf("recover error: %s", err)
	}
	if !bytes.Equal(pubkey1, pubkey2) {
		t.Errorf("pubkey mismatch: want: %x have: %x", pubkey1, pubkey2)
	}
}

func BenchmarkSign(b *testing.B) {
	_, seckey := generateKeyPair()
	msg := csprngEntropy(32)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		Sign(msg, seckey)
	}
}

func BenchmarkRecover(b *testing.B) {
	msg := csprngEntropy(32)
	_, seckey := generateKeyPair()
	sig, _ := Sign(msg, seckey)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		RecoverPubkey(msg, sig)
	}
}
