
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

package mailserver

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv6"
)

const powRequirement = 0.00001

var keyID string
var shh *whisper.Whisper
var seed = time.Now().Unix()

type ServerTestParams struct {
	topic whisper.TopicType
	low   uint32
	upp   uint32
	key   *ecdsa.PrivateKey
}

func assert(statement bool, text string, t *testing.T) {
	if !statement {
		t.Fatal(text)
	}
}

func TestDBKey(t *testing.T) {
	var h common.Hash
	i := uint32(time.Now().Unix())
	k := NewDbKey(i, h)
	assert(len(k.raw) == common.HashLength+4, "wrong DB key length", t)
	assert(byte(i%0x100) == k.raw[3], "raw representation should be big endian", t)
	assert(byte(i/0x1000000) == k.raw[0], "big endian expected", t)
}

func generateEnvelope(t *testing.T) *whisper.Envelope {
	h := crypto.Keccak256Hash([]byte("test sample data"))
	params := &whisper.MessageParams{
		KeySym:   h[:],
		Topic:    whisper.TopicType{0x1F, 0x7E, 0xA1, 0x7F},
		Payload:  []byte("test payload"),
		PoW:      powRequirement,
		WorkTime: 2,
	}

	msg, err := whisper.NewSentMessage(params)
	if err != nil {
		t.Fatalf("failed to create new message with seed %d: %s.", seed, err)
	}
	env, err := msg.Wrap(params)
	if err != nil {
		t.Fatalf("failed to wrap with seed %d: %s.", seed, err)
	}
	return env
}

func TestMailServer(t *testing.T) {
	const password = "password_for_this_test"
	const dbPath = "whisper-server-test"

	dir, err := ioutil.TempDir("", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	var server WMailServer
	shh = whisper.New(&whisper.DefaultConfig)
	shh.RegisterServer(&server)

	err = server.Init(shh, dir, password, powRequirement)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	keyID, err = shh.AddSymKeyFromPassword(password)
	if err != nil {
		t.Fatalf("Failed to create symmetric key for mail request: %s", err)
	}

	rand.Seed(seed)
	env := generateEnvelope(t)
	server.Archive(env)
	deliverTest(t, &server, env)
}

func deliverTest(t *testing.T, server *WMailServer, env *whisper.Envelope) {
	id, err := shh.NewKeyPair()
	if err != nil {
		t.Fatalf("failed to generate new key pair with seed %d: %s.", seed, err)
	}
	testPeerID, err := shh.GetPrivateKey(id)
	if err != nil {
		t.Fatalf("failed to retrieve new key pair with seed %d: %s.", seed, err)
	}
	birth := env.Expiry - env.TTL
	p := &ServerTestParams{
		topic: env.Topic,
		low:   birth - 1,
		upp:   birth + 1,
		key:   testPeerID,
	}

	singleRequest(t, server, env, p, true)

	p.low, p.upp = birth+1, 0xffffffff
	singleRequest(t, server, env, p, false)

	p.low, p.upp = 0, birth-1
	singleRequest(t, server, env, p, false)

	p.low = birth - 1
	p.upp = birth + 1
	p.topic[0] = 0xFF
	singleRequest(t, server, env, p, false)
}

func singleRequest(t *testing.T, server *WMailServer, env *whisper.Envelope, p *ServerTestParams, expect bool) {
	request := createRequest(t, p)
	src := crypto.FromECDSAPub(&p.key.PublicKey)
	ok, lower, upper, bloom := server.validateRequest(src, request)
	if !ok {
		t.Fatalf("request validation failed, seed: %d.", seed)
	}
	if lower != p.low {
		t.Fatalf("request validation failed (lower bound), seed: %d.", seed)
	}
	if upper != p.upp {
		t.Fatalf("request validation failed (upper bound), seed: %d.", seed)
	}
	expectedBloom := whisper.TopicToBloom(p.topic)
	if !bytes.Equal(bloom, expectedBloom) {
		t.Fatalf("request validation failed (topic), seed: %d.", seed)
	}

	var exist bool
	mail := server.processRequest(nil, p.low, p.upp, bloom)
	for _, msg := range mail {
		if msg.Hash() == env.Hash() {
			exist = true
			break
		}
	}

	if exist != expect {
		t.Fatalf("error: exist = %v, seed: %d.", exist, seed)
	}

	src[0]++
	ok, lower, upper, bloom = server.validateRequest(src, request)
	if !ok {
//无论签名如何，请求都应有效。
		t.Fatalf("request validation false negative, seed: %d (lower: %d, upper: %d).", seed, lower, upper)
	}
}

func createRequest(t *testing.T, p *ServerTestParams) *whisper.Envelope {
	bloom := whisper.TopicToBloom(p.topic)
	data := make([]byte, 8)
	binary.BigEndian.PutUint32(data, p.low)
	binary.BigEndian.PutUint32(data[4:], p.upp)
	data = append(data, bloom...)

	key, err := shh.GetSymKey(keyID)
	if err != nil {
		t.Fatalf("failed to retrieve sym key with seed %d: %s.", seed, err)
	}

	params := &whisper.MessageParams{
		KeySym:   key,
		Topic:    p.topic,
		Payload:  data,
		PoW:      powRequirement * 2,
		WorkTime: 2,
		Src:      p.key,
	}

	msg, err := whisper.NewSentMessage(params)
	if err != nil {
		t.Fatalf("failed to create new message with seed %d: %s.", seed, err)
	}
	env, err := msg.Wrap(params)
	if err != nil {
		t.Fatalf("failed to wrap with seed %d: %s.", seed, err)
	}
	return env
}
