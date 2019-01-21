
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
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/swarm/chunk"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/feed/lookup"
)

var (
	loglevel  = flag.Int("loglevel", 3, "loglevel")
	startTime = Timestamp{
		Time: uint64(4200),
	}
	cleanF       func()
	subtopicName = "føø.bar"
)

func init() {
	flag.Parse()
	log.Root().SetHandler(log.CallerFileHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(os.Stderr, log.TerminalFormat(true)))))
}

//模拟时间提供程序
type fakeTimeProvider struct {
	currentTime uint64
}

func (f *fakeTimeProvider) Tick() {
	f.currentTime++
}

func (f *fakeTimeProvider) Set(time uint64) {
	f.currentTime = time
}

func (f *fakeTimeProvider) FastForward(offset uint64) {
	f.currentTime += offset
}

func (f *fakeTimeProvider) Now() Timestamp {
	return Timestamp{
		Time: f.currentTime,
	}
}

//根据期间和版本进行更新并检索它们
func TestFeedsHandler(t *testing.T) {

//生成假时间提供程序
	clock := &fakeTimeProvider{
currentTime: startTime.Time, //时钟从t=4200开始
	}

//包含私钥的签名者
	signer := newAliceSigner()

	feedsHandler, datadir, teardownTest, err := setupTest(clock, signer)
	if err != nil {
		t.Fatal(err)
	}
	defer teardownTest()

//创建新源
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	topic, _ := NewTopic("Mess with Swarm feeds code and see what ghost catches you", nil)
	fd := Feed{
		Topic: topic,
		User:  signer.Address(),
	}

//更新数据：
	updates := []string{
"blinky", //t＝4200
"pinky",  //t＝4242
"inky",   //t＝4284
"clyde",  //t＝4285
	}

request := NewFirstRequest(fd.Topic) //此时间戳更新时间t=4200（开始时间）
	chunkAddress := make(map[string]storage.Address)
	data := []byte(updates[0])
	request.SetData(data)
	if err := request.Sign(signer); err != nil {
		t.Fatal(err)
	}
	chunkAddress[updates[0]], err = feedsHandler.Update(ctx, request)
	if err != nil {
		t.Fatal(err)
	}

//向前移动时钟21秒
clock.FastForward(21) //t＝4221

request, err = feedsHandler.NewRequest(ctx, &request.Feed) //此时间戳更新t=4221
	if err != nil {
		t.Fatal(err)
	}
	if request.Epoch.Base() != 0 || request.Epoch.Level != lookup.HighestLevel-1 {
		t.Fatalf("Suggested epoch BaseTime should be 0 and Epoch level should be %d", lookup.HighestLevel-1)
	}

request.Epoch.Level = lookup.HighestLevel //强制25级而不是24级使其失效
	data = []byte(updates[1])
	request.SetData(data)
	if err := request.Sign(signer); err != nil {
		t.Fatal(err)
	}
	chunkAddress[updates[1]], err = feedsHandler.Update(ctx, request)
	if err == nil {
		t.Fatal("Expected update to fail since an update in this epoch already exists")
	}

//向前移动时钟21秒
clock.FastForward(21) //t＝4242
	request, err = feedsHandler.NewRequest(ctx, &request.Feed)
	if err != nil {
		t.Fatal(err)
	}
	request.SetData(data)
	if err := request.Sign(signer); err != nil {
		t.Fatal(err)
	}
	chunkAddress[updates[1]], err = feedsHandler.Update(ctx, request)
	if err != nil {
		t.Fatal(err)
	}

//向前移动时钟42秒
clock.FastForward(42) //t＝4284
	request, err = feedsHandler.NewRequest(ctx, &request.Feed)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(updates[2])
	request.SetData(data)
	if err := request.Sign(signer); err != nil {
		t.Fatal(err)
	}
	chunkAddress[updates[2]], err = feedsHandler.Update(ctx, request)
	if err != nil {
		t.Fatal(err)
	}

//向前拨钟1秒
clock.FastForward(1) //t＝4285
	request, err = feedsHandler.NewRequest(ctx, &request.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if request.Epoch.Base() != 0 || request.Epoch.Level != 22 {
		t.Fatalf("Expected epoch base time to be %d, got %d. Expected epoch level to be %d, got %d", 0, request.Epoch.Base(), 22, request.Epoch.Level)
	}
	data = []byte(updates[3])
	request.SetData(data)

	if err := request.Sign(signer); err != nil {
		t.Fatal(err)
	}
	chunkAddress[updates[3]], err = feedsHandler.Update(ctx, request)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)
	feedsHandler.Close()

//检查关闭后我们可以检索更新
clock.FastForward(2000) //t＝6285

	feedParams := &HandlerParams{}

	feedsHandler2, err := NewTestHandler(datadir, feedParams)
	if err != nil {
		t.Fatal(err)
	}

	update2, err := feedsHandler2.Lookup(ctx, NewQueryLatest(&request.Feed, lookup.NoClue))
	if err != nil {
		t.Fatal(err)
	}

//最后一次更新应该是“clyde”
	if !bytes.Equal(update2.data, []byte(updates[len(updates)-1])) {
		t.Fatalf("feed update data was %v, expected %v", string(update2.data), updates[len(updates)-1])
	}
	if update2.Level != 22 {
		t.Fatalf("feed update epoch level was %d, expected 22", update2.Level)
	}
	if update2.Base() != 0 {
		t.Fatalf("feed update epoch base time was %d, expected 0", update2.Base())
	}
	log.Debug("Latest lookup", "epoch base time", update2.Base(), "epoch level", update2.Level, "data", update2.data)

//特定时间点
	update, err := feedsHandler2.Lookup(ctx, NewQuery(&request.Feed, 4284, lookup.NoClue))
	if err != nil {
		t.Fatal(err)
	}
//检查数据
	if !bytes.Equal(update.data, []byte(updates[2])) {
		t.Fatalf("feed update data (historical) was %v, expected %v", string(update2.data), updates[2])
	}
	log.Debug("Historical lookup", "epoch base time", update2.Base(), "epoch level", update2.Level, "data", update2.data)

//超过第一个会产生错误
	update, err = feedsHandler2.Lookup(ctx, NewQuery(&request.Feed, startTime.Time-1, lookup.NoClue))
	if err == nil {
		t.Fatalf("expected previous to fail, returned epoch %s data %v", update.Epoch.String(), update.data)
	}

}

const Day = 60 * 60 * 24
const Year = Day * 365
const Month = Day * 30

func generateData(x uint64) []byte {
	return []byte(fmt.Sprintf("%d", x))
}

func TestSparseUpdates(t *testing.T) {

//生成假时间提供程序
	timeProvider := &fakeTimeProvider{
		currentTime: startTime.Time,
	}

//包含私钥的签名者
	signer := newAliceSigner()

	rh, datadir, teardownTest, err := setupTest(timeProvider, signer)
	if err != nil {
		t.Fatal(err)
	}
	defer teardownTest()
	defer os.RemoveAll(datadir)

//创建新源
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	topic, _ := NewTopic("Very slow updates", nil)
	fd := Feed{
		Topic: topic,
		User:  signer.Address(),
	}

//从UNIX 0到今天，每5年发布一次更新
	today := uint64(1533799046)
	var epoch lookup.Epoch
	var lastUpdateTime uint64
	for T := uint64(0); T < today; T += 5 * Year {
		request := NewFirstRequest(fd.Topic)
		request.Epoch = lookup.GetNextEpoch(epoch, T)
request.data = generateData(T) //这会生成一些依赖于t的数据，因此我们可以稍后检查
		request.Sign(signer)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := rh.Update(ctx, request); err != nil {
			t.Fatal(err)
		}
		epoch = request.Epoch
		lastUpdateTime = T
	}

	query := NewQuery(&fd, today, lookup.NoClue)

	_, err = rh.Lookup(ctx, query)
	if err != nil {
		t.Fatal(err)
	}

	_, content, err := rh.GetContent(&fd)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(generateData(lastUpdateTime), content) {
		t.Fatalf("Expected to recover last written value %d, got %s", lastUpdateTime, string(content))
	}

//查找最接近35*年+6*月（~2005年6月）的更新：
//因为我们每5年更新一次，所以应该可以找到35*年的更新。

	query.TimeLimit = 35*Year + 6*Month

	_, err = rh.Lookup(ctx, query)
	if err != nil {
		t.Fatal(err)
	}

	_, content, err = rh.GetContent(&fd)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(generateData(35*Year), content) {
		t.Fatalf("Expected to recover %d, got %s", 35*Year, string(content))
	}
}

func TestValidator(t *testing.T) {

//生成假时间提供程序
	timeProvider := &fakeTimeProvider{
		currentTime: startTime.Time,
	}

//包含私钥的签名者。爱丽丝会是个好女孩
	signer := newAliceSigner()

//设置SIM时间提供程序
	rh, _, teardownTest, err := setupTest(timeProvider, signer)
	if err != nil {
		t.Fatal(err)
	}
	defer teardownTest()

//创建新的饲料
	topic, _ := NewTopic(subtopicName, nil)
	fd := Feed{
		Topic: topic,
		User:  signer.Address(),
	}
	mr := NewFirstRequest(fd.Topic)

//带地址的块
	data := []byte("foo")
	mr.SetData(data)
	if err := mr.Sign(signer); err != nil {
		t.Fatalf("sign fail: %v", err)
	}

	chunk, err := mr.toChunk()
	if err != nil {
		t.Fatal(err)
	}
	if !rh.Validate(chunk) {
		t.Fatal("Chunk validator fail on update chunk")
	}

	address := chunk.Address()
//弄乱地址
	address[0] = 11
	address[15] = 99

	if rh.Validate(storage.NewChunk(address, chunk.Data())) {
		t.Fatal("Expected Validate to fail with false chunk address")
	}
}

//测试内容地址验证器是否正确检查数据
//通过内容地址验证器传递源更新块的测试
//这个测试中有一些冗余，因为它还测试内容寻址块，
//此验证器应将其评估为无效块
func TestValidatorInStore(t *testing.T) {

//生成假时间提供程序
	TimestampProvider = &fakeTimeProvider{
		currentTime: startTime.Time,
	}

//包含私钥的签名者
	signer := newAliceSigner()

//设置本地存储
	datadir, err := ioutil.TempDir("", "storage-testfeedsvalidator")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(datadir)

	handlerParams := storage.NewDefaultLocalStoreParams()
	handlerParams.Init(datadir)
	store, err := storage.NewLocalStore(handlerParams, nil)
	if err != nil {
		t.Fatal(err)
	}

//设置Swarm Feeds处理程序并将其添加为本地商店的验证程序
	fhParams := &HandlerParams{}
	fh := NewHandler(fhParams)
	store.Validators = append(store.Validators, fh)

//创建内容寻址块，一个好，一个坏
	chunks := storage.GenerateRandomChunks(chunk.DefaultSize, 2)
	goodChunk := chunks[0]
	badChunk := storage.NewChunk(chunks[1].Address(), goodChunk.Data())

	topic, _ := NewTopic("xyzzy", nil)
	fd := Feed{
		Topic: topic,
		User:  signer.Address(),
	}

//使用正确的PublicKey创建源更新区块
	id := ID{
		Epoch: lookup.Epoch{Time: 42,
			Level: 1,
		},
		Feed: fd,
	}

	updateAddr := id.Addr()
	data := []byte("bar")

	r := new(Request)
	r.idAddr = updateAddr
	r.Update.ID = id
	r.data = data

	r.Sign(signer)

	uglyChunk, err := r.toChunk()
	if err != nil {
		t.Fatal(err)
	}

//将块放入存储并检查其错误状态
	err = store.Put(context.Background(), goodChunk)
	if err == nil {
		t.Fatal("expected error on good content address chunk with feed update validator only, but got nil")
	}
	err = store.Put(context.Background(), badChunk)
	if err == nil {
		t.Fatal("expected error on bad content address chunk with feed update validator only, but got nil")
	}
	err = store.Put(context.Background(), uglyChunk)
	if err != nil {
		t.Fatalf("expected no error on feed update chunk with feed update validator only, but got: %s", err)
	}
}

//创建RPC和源处理程序
func setupTest(timeProvider timestampProvider, signer Signer) (fh *TestHandler, datadir string, teardown func(), err error) {

	var fsClean func()
	var rpcClean func()
	cleanF = func() {
		if fsClean != nil {
			fsClean()
		}
		if rpcClean != nil {
			rpcClean()
		}
	}

//临时数据
	datadir, err = ioutil.TempDir("", "fh")
	if err != nil {
		return nil, "", nil, err
	}
	fsClean = func() {
		os.RemoveAll(datadir)
	}

	TimestampProvider = timeProvider
	fhParams := &HandlerParams{}
	fh, err = NewTestHandler(datadir, fhParams)
	return fh, datadir, cleanF, err
}

func newAliceSigner() *GenericSigner {
	privKey, _ := crypto.HexToECDSA("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	return NewGenericSigner(privKey)
}

func newBobSigner() *GenericSigner {
	privKey, _ := crypto.HexToECDSA("accedeaccedeaccedeaccedeaccedeaccedeaccedeaccedeaccedeaccedecaca")
	return NewGenericSigner(privKey)
}

func newCharlieSigner() *GenericSigner {
	privKey, _ := crypto.HexToECDSA("facadefacadefacadefacadefacadefacadefacadefacadefacadefacadefaca")
	return NewGenericSigner(privKey)
}
