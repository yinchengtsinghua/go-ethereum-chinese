
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

package feed

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/feed/lookup"
)

func areEqualJSON(s1, s2 string) (bool, error) {
//技巧的功劳：turtlemonvh https://gist.github.com/turtlemonvh/e4f7404e283887fadb8ad275a99596f67
	var o1 interface{}
	var o2 interface{}

	err := json.Unmarshal([]byte(s1), &o1)
	if err != nil {
		return false, fmt.Errorf("Error mashalling string 1 :: %s", err.Error())
	}
	err = json.Unmarshal([]byte(s2), &o2)
	if err != nil {
		return false, fmt.Errorf("Error mashalling string 2 :: %s", err.Error())
	}

	return reflect.DeepEqual(o1, o2), nil
}

//TestEncodingDecodingUpdateRequests确保正确序列化请求
//同时还通过加密方式检查只有提要的所有者才能更新它。
func TestEncodingDecodingUpdateRequests(t *testing.T) {

charlie := newCharlieSigner() //查理
bob := newBobSigner()         //鲍勃

//为我们的好人查理的名字创建一个提要
	topic, _ := NewTopic("a good topic name", nil)
	firstRequest := NewFirstRequest(topic)
	firstRequest.User = charlie.Address()

//我们现在对创建消息进行编码，以模拟通过网络发送的消息。
	messageRawData, err := firstRequest.MarshalJSON()
	if err != nil {
		t.Fatalf("Error encoding first feed update request: %s", err)
	}

//…消息到达并被解码…
	var recoveredFirstRequest Request
	if err := recoveredFirstRequest.UnmarshalJSON(messageRawData); err != nil {
		t.Fatalf("Error decoding first feed update request: %s", err)
	}

//…但是验证应该失败，因为它没有签名！
	if err := recoveredFirstRequest.Verify(); err == nil {
		t.Fatal("Expected Verify to fail since the message is not signed")
	}

//我们现在假设feed ypdate是被创建和传播的。

	const expectedSignature = "0x7235b27a68372ddebcf78eba48543fa460864b0b0e99cb533fcd3664820e603312d29426dd00fb39628f5299480a69bf6e462838d78de49ce0704c754c9deb2601"
	const expectedJSON = `{"feed":{"topic":"0x6120676f6f6420746f706963206e616d65000000000000000000000000000000","user":"0x876a8936a7cd0b79ef0735ad0896c1afe278781c"},"epoch":{"time":1000,"level":1},"protocolVersion":0,"data":"0x5468697320686f75722773207570646174653a20537761726d2039392e3020686173206265656e2072656c656173656421"}`

//将一个未签名的更新请求放在一起，我们将序列化该请求以将其发送给签名者。
	data := []byte("This hour's update: Swarm 99.0 has been released!")
	request := &Request{
		Update: Update{
			ID: ID{
				Epoch: lookup.Epoch{
					Time:  1000,
					Level: 1,
				},
				Feed: firstRequest.Update.Feed,
			},
			data: data,
		},
	}

	messageRawData, err = request.MarshalJSON()
	if err != nil {
		t.Fatalf("Error encoding update request: %s", err)
	}

	equalJSON, err := areEqualJSON(string(messageRawData), expectedJSON)
	if err != nil {
		t.Fatalf("Error decoding update request JSON: %s", err)
	}
	if !equalJSON {
		t.Fatalf("Received a different JSON message. Expected %s, got %s", expectedJSON, string(messageRawData))
	}

//现在，编码的消息messagerawdata通过网络发送并到达签名者。

//尝试从编码的消息中提取更新请求
	var recoveredRequest Request
	if err := recoveredRequest.UnmarshalJSON(messageRawData); err != nil {
		t.Fatalf("Error decoding update request: %s", err)
	}

//对请求进行签名，看看它是否与上面预先定义的签名匹配。
	if err := recoveredRequest.Sign(charlie); err != nil {
		t.Fatalf("Error signing request: %s", err)
	}

	compareByteSliceToExpectedHex(t, "signature", recoveredRequest.Signature[:], expectedSignature)

//弄乱签名看看会发生什么。为了改变签名，我们简单地将其解码为JSON
//更改签名字段。
	var j updateRequestJSON
	if err := json.Unmarshal([]byte(expectedJSON), &j); err != nil {
		t.Fatal("Error unmarshalling test json, check expectedJSON constant")
	}
	j.Signature = "Certainly not a signature"
corruptMessage, _ := json.Marshal(j) //用错误的签名对邮件进行编码
	var corruptRequest Request
	if err = corruptRequest.UnmarshalJSON(corruptMessage); err == nil {
		t.Fatal("Expected DecodeUpdateRequest to fail when trying to interpret a corrupt message with an invalid signature")
	}

//现在假设Bob想要创建一个关于同一个feed的更新，
//用他的私人钥匙签名
	if err := request.Sign(bob); err != nil {
		t.Fatalf("Error signing: %s", err)
	}

//现在，Bob对消息进行编码，以便通过网络发送…
	messageRawData, err = request.MarshalJSON()
	if err != nil {
		t.Fatalf("Error encoding message:%s", err)
	}

//…消息到达我们的群节点并被解码。
	recoveredRequest = Request{}
	if err := recoveredRequest.UnmarshalJSON(messageRawData); err != nil {
		t.Fatalf("Error decoding message:%s", err)
	}

//在检查鲍勃的最新消息之前，让我们先看看如果我们搞砸了会发生什么。
//在签名的时候看看verify是否捕捉到它
savedSignature := *recoveredRequest.Signature                               //保存签名供以后使用
binary.LittleEndian.PutUint64(recoveredRequest.Signature[5:], 556845463424) //写一些随机数据来破坏签名
	if err = recoveredRequest.Verify(); err == nil {
		t.Fatal("Expected Verify to fail on corrupt signature")
	}

//从腐败中恢复Bob的签名
	*recoveredRequest.Signature = savedSignature

//现在签名没有损坏
	if err = recoveredRequest.Verify(); err != nil {
		t.Fatal(err)
	}

//重用对象并用我们朋友Charlie的私钥签名
	if err := recoveredRequest.Sign(charlie); err != nil {
		t.Fatalf("Error signing with the correct private key: %s", err)
	}

//现在，验证应该可以工作，因为这个更新现在属于查理。
	if err = recoveredRequest.Verify(); err != nil {
		t.Fatalf("Error verifying that Charlie, can sign a reused request object:%s", err)
	}

//混淆查找键以确保验证失败：
recoveredRequest.Time = 77999 //这将更改查找键
	if err = recoveredRequest.Verify(); err == nil {
		t.Fatalf("Expected Verify to fail since the lookup key has been altered")
	}
}

func getTestRequest() *Request {
	return &Request{
		Update: *getTestFeedUpdate(),
	}
}

func TestUpdateChunkSerializationErrorChecking(t *testing.T) {

//如果块太小，则ParseUpdate测试失败
	var r Request
	if err := r.fromChunk(storage.NewChunk(storage.ZeroAddr, make([]byte, minimumUpdateDataLength-1+signatureLength))); err == nil {
		t.Fatalf("Expected request.fromChunk to fail when chunkData contains less than %d bytes", minimumUpdateDataLength)
	}

	r = *getTestRequest()

	_, err := r.toChunk()
	if err == nil {
		t.Fatal("Expected request.toChunk to fail when there is no data")
	}
r.data = []byte("Al bien hacer jamás le falta premio") //输入任意长度的数据
	_, err = r.toChunk()
	if err == nil {
		t.Fatal("expected request.toChunk to fail when there is no signature")
	}

	charlie := newCharlieSigner()
	if err := r.Sign(charlie); err != nil {
		t.Fatalf("error signing:%s", err)
	}

	chunk, err := r.toChunk()
	if err != nil {
		t.Fatalf("error creating update chunk:%s", err)
	}

	compareByteSliceToExpectedHex(t, "chunk", chunk.Data(), "0x0000000000000000776f726c64206e657773207265706f72742c20657665727920686f7572000000876a8936a7cd0b79ef0735ad0896c1afe278781ce803000000000019416c206269656e206861636572206a616dc3a173206c652066616c7461207072656d696f5a0ffe0bc27f207cd5b00944c8b9cee93e08b89b5ada777f123ac535189333f174a6a4ca2f43a92c4a477a49d774813c36ce8288552c58e6205b0ac35d0507eb00")

	var recovered Request
	recovered.fromChunk(chunk)
	if !reflect.DeepEqual(recovered, r) {
		t.Fatal("Expected recovered feed update request to equal the original one")
	}
}

//检查签名地址是否与更新签名者地址匹配
func TestReverse(t *testing.T) {

	epoch := lookup.Epoch{
		Time:  7888,
		Level: 6,
	}

//生成假时间提供程序
	timeProvider := &fakeTimeProvider{
		currentTime: startTime.Time,
	}

//包含私钥的签名者
	signer := newAliceSigner()

//设置RPC并创建源处理程序
	_, _, teardownTest, err := setupTest(timeProvider, signer)
	if err != nil {
		t.Fatal(err)
	}
	defer teardownTest()

	topic, _ := NewTopic("Cervantes quotes", nil)
	fd := Feed{
		Topic: topic,
		User:  signer.Address(),
	}

	data := []byte("Donde una puerta se cierra, otra se abre")

	request := new(Request)
	request.Feed = fd
	request.Epoch = epoch
	request.data = data

//为此请求生成区块键
	key := request.Addr()

	if err = request.Sign(signer); err != nil {
		t.Fatal(err)
	}

	chunk, err := request.toChunk()
	if err != nil {
		t.Fatal(err)
	}

//检查我们是否可以从更新块的签名中恢复所有者帐户
	var checkUpdate Request
	if err := checkUpdate.fromChunk(chunk); err != nil {
		t.Fatal(err)
	}
	checkdigest, err := checkUpdate.GetDigest()
	if err != nil {
		t.Fatal(err)
	}
	recoveredAddr, err := getUserAddr(checkdigest, *checkUpdate.Signature)
	if err != nil {
		t.Fatalf("Retrieve address from signature fail: %v", err)
	}
	originalAddr := crypto.PubkeyToAddress(signer.PrivKey.PublicKey)

//检查从块中检索到的元数据是否与我们提供的元数据匹配
	if recoveredAddr != originalAddr {
		t.Fatalf("addresses dont match: %x != %x", originalAddr, recoveredAddr)
	}

	if !bytes.Equal(key[:], chunk.Address()[:]) {
		t.Fatalf("Expected chunk key '%x', was '%x'", key, chunk.Address())
	}
	if epoch != checkUpdate.Epoch {
		t.Fatalf("Expected epoch to be '%s', was '%s'", epoch.String(), checkUpdate.Epoch.String())
	}
	if !bytes.Equal(data, checkUpdate.data) {
		t.Fatalf("Expected data '%x', was '%x'", data, checkUpdate.data)
	}
}
