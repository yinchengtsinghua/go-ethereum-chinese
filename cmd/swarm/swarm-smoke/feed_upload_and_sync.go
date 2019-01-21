
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/swarm/api/client"
	"github.com/ethereum/go-ethereum/swarm/spancontext"
	"github.com/ethereum/go-ethereum/swarm/storage/feed"
	"github.com/ethereum/go-ethereum/swarm/testutil"
	colorable "github.com/mattn/go-colorable"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pborman/uuid"
	cli "gopkg.in/urfave/cli.v1"
)

const (
	feedRandomDataLength = 8
)

func cliFeedUploadAndSync(c *cli.Context) error {
	metrics.GetOrRegisterCounter("feed-and-sync", nil).Inc(1)
	log.Root().SetHandler(log.CallerFileHandler(log.LvlFilterHandler(log.Lvl(verbosity), log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(true)))))

	errc := make(chan error)
	go func() {
		errc <- feedUploadAndSync(c)
	}()

	select {
	case err := <-errc:
		if err != nil {
			metrics.GetOrRegisterCounter("feed-and-sync.fail", nil).Inc(1)
		}
		return err
	case <-time.After(time.Duration(timeout) * time.Second):
		metrics.GetOrRegisterCounter("feed-and-sync.timeout", nil).Inc(1)
		return fmt.Errorf("timeout after %v sec", timeout)
	}
}

//TODO:使用清单+提取重复代码检索
func feedUploadAndSync(c *cli.Context) error {
	defer func(now time.Time) { log.Info("total time", "time", time.Since(now), "size (kb)", filesize) }(time.Now())

	generateEndpoints(scheme, cluster, appName, from, to)

	log.Info("generating and uploading feeds to " + endpoints[0] + " and syncing")

//创建一个随机的私钥来为更新签名并派生地址
	pkFile, err := ioutil.TempFile("", "swarm-feed-smoke-test")
	if err != nil {
		return err
	}
	defer pkFile.Close()
	defer os.Remove(pkFile.Name())

	privkeyHex := "0000000000000000000000000000000000000000000000000000000000001976"
	privKey, err := crypto.HexToECDSA(privkeyHex)
	if err != nil {
		return err
	}
	user := crypto.PubkeyToAddress(privKey.PublicKey)
	userHex := hexutil.Encode(user.Bytes())

//将私钥保存到文件
	_, err = io.WriteString(pkFile, privkeyHex)
	if err != nil {
		return err
	}

//保留主题和副标题的十六进制字符串
	var topicHex string
	var subTopicHex string

//
//与主题异或（如果没有主题，则为零值主题）
	var subTopicOnlyHex string
	var mergedSubTopicHex string

//生成随机主题和副标题并在其上加上十六进制
	topicBytes, err := generateRandomData(feed.TopicLength)
	topicHex = hexutil.Encode(topicBytes)
	subTopicBytes, err := generateRandomData(8)
	subTopicHex = hexutil.Encode(subTopicBytes)
	if err != nil {
		return err
	}
	mergedSubTopic, err := feed.NewTopic(subTopicHex, topicBytes)
	if err != nil {
		return err
	}
	mergedSubTopicHex = hexutil.Encode(mergedSubTopic[:])
	subTopicOnlyBytes, err := feed.NewTopic(subTopicHex, nil)
	if err != nil {
		return err
	}
	subTopicOnlyHex = hexutil.Encode(subTopicOnlyBytes[:])

//创建源清单，仅主题
	var out bytes.Buffer
	cmd := exec.Command("swarm", "--bzzapi", endpoints[0], "feed", "create", "--topic", topicHex, "--user", userHex)
	cmd.Stdout = &out
	log.Debug("create feed manifest topic cmd", "cmd", cmd)
	err = cmd.Run()
	if err != nil {
		return err
	}
	manifestWithTopic := strings.TrimRight(out.String(), string([]byte{0x0a}))
	if len(manifestWithTopic) != 64 {
		return fmt.Errorf("unknown feed create manifest hash format (topic): (%d) %s", len(out.String()), manifestWithTopic)
	}
	log.Debug("create topic feed", "manifest", manifestWithTopic)
	out.Reset()

//创建源清单，仅副标题
	cmd = exec.Command("swarm", "--bzzapi", endpoints[0], "feed", "create", "--name", subTopicHex, "--user", userHex)
	cmd.Stdout = &out
	log.Debug("create feed manifest subtopic cmd", "cmd", cmd)
	err = cmd.Run()
	if err != nil {
		return err
	}
	manifestWithSubTopic := strings.TrimRight(out.String(), string([]byte{0x0a}))
	if len(manifestWithSubTopic) != 64 {
		return fmt.Errorf("unknown feed create manifest hash format (subtopic): (%d) %s", len(out.String()), manifestWithSubTopic)
	}
	log.Debug("create subtopic feed", "manifest", manifestWithTopic)
	out.Reset()

//
	cmd = exec.Command("swarm", "--bzzapi", endpoints[0], "feed", "create", "--topic", topicHex, "--name", subTopicHex, "--user", userHex)
	cmd.Stdout = &out
	log.Debug("create feed manifest mergetopic cmd", "cmd", cmd)
	err = cmd.Run()
	if err != nil {
		log.Error(err.Error())
		return err
	}
	manifestWithMergedTopic := strings.TrimRight(out.String(), string([]byte{0x0a}))
	if len(manifestWithMergedTopic) != 64 {
		return fmt.Errorf("unknown feed create manifest hash format (mergedtopic): (%d) %s", len(out.String()), manifestWithMergedTopic)
	}
	log.Debug("create mergedtopic feed", "manifest", manifestWithMergedTopic)
	out.Reset()

//创建测试数据
	data, err := generateRandomData(feedRandomDataLength)
	if err != nil {
		return err
	}
	h := md5.New()
	h.Write(data)
	dataHash := h.Sum(nil)
	dataHex := hexutil.Encode(data)

//更新主题
	cmd = exec.Command("swarm", "--bzzaccount", pkFile.Name(), "--bzzapi", endpoints[0], "feed", "update", "--topic", topicHex, dataHex)
	cmd.Stdout = &out
	log.Debug("update feed manifest topic cmd", "cmd", cmd)
	err = cmd.Run()
	if err != nil {
		return err
	}
	log.Debug("feed update topic", "out", out)
	out.Reset()

//用副标题更新
	cmd = exec.Command("swarm", "--bzzaccount", pkFile.Name(), "--bzzapi", endpoints[0], "feed", "update", "--name", subTopicHex, dataHex)
	cmd.Stdout = &out
	log.Debug("update feed manifest subtopic cmd", "cmd", cmd)
	err = cmd.Run()
	if err != nil {
		return err
	}
	log.Debug("feed update subtopic", "out", out)
	out.Reset()

//使用合并主题更新
	cmd = exec.Command("swarm", "--bzzaccount", pkFile.Name(), "--bzzapi", endpoints[0], "feed", "update", "--topic", topicHex, "--name", subTopicHex, dataHex)
	cmd.Stdout = &out
	log.Debug("update feed manifest merged topic cmd", "cmd", cmd)
	err = cmd.Run()
	if err != nil {
		return err
	}
	log.Debug("feed update mergedtopic", "out", out)
	out.Reset()

	time.Sleep(3 * time.Second)

//
	wg := sync.WaitGroup{}
	for _, endpoint := range endpoints {
//原始检索，仅主题
		for _, hex := range []string{topicHex, subTopicOnlyHex, mergedSubTopicHex} {
			wg.Add(1)
			ruid := uuid.New()[:8]
			go func(hex string, endpoint string, ruid string) {
				for {
					err := fetchFeed(hex, userHex, endpoint, dataHash, ruid)
					if err != nil {
						continue
					}

					wg.Done()
					return
				}
			}(hex, endpoint, ruid)

		}
	}
	wg.Wait()
	log.Info("all endpoints synced random data successfully")

//上载测试文件
	seed := int(time.Now().UnixNano() / 1e6)
	log.Info("feed uploading to "+endpoints[0]+" and syncing", "seed", seed)

	randomBytes := testutil.RandomBytes(seed, filesize*1000)

	hash, err := upload(&randomBytes, endpoints[0])
	if err != nil {
		return err
	}
	hashBytes, err := hexutil.Decode("0x" + hash)
	if err != nil {
		return err
	}
	multihashHex := hexutil.Encode(hashBytes)
	fileHash, err := digest(bytes.NewReader(randomBytes))
	if err != nil {
		return err
	}

	log.Info("uploaded successfully", "hash", hash, "digest", fmt.Sprintf("%x", fileHash))

//
	cmd = exec.Command("swarm", "--bzzaccount", pkFile.Name(), "--bzzapi", endpoints[0], "feed", "update", "--topic", topicHex, multihashHex)
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return err
	}
	log.Debug("feed update topic", "out", out)
	out.Reset()

//用副标题更新文件
	cmd = exec.Command("swarm", "--bzzaccount", pkFile.Name(), "--bzzapi", endpoints[0], "feed", "update", "--name", subTopicHex, multihashHex)
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return err
	}
	log.Debug("feed update subtopic", "out", out)
	out.Reset()

//用合并的主题更新文件
	cmd = exec.Command("swarm", "--bzzaccount", pkFile.Name(), "--bzzapi", endpoints[0], "feed", "update", "--topic", topicHex, "--name", subTopicHex, multihashHex)
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return err
	}
	log.Debug("feed update mergedtopic", "out", out)
	out.Reset()

	time.Sleep(3 * time.Second)

	for _, endpoint := range endpoints {

//清单检索，仅主题
		for _, url := range []string{manifestWithTopic, manifestWithSubTopic, manifestWithMergedTopic} {
			wg.Add(1)
			ruid := uuid.New()[:8]
			go func(url string, endpoint string, ruid string) {
				for {
					err := fetch(url, endpoint, fileHash, ruid)
					if err != nil {
						continue
					}

					wg.Done()
					return
				}
			}(url, endpoint, ruid)
		}

	}
	wg.Wait()
	log.Info("all endpoints synced random file successfully")

	return nil
}

func fetchFeed(topic string, user string, endpoint string, original []byte, ruid string) error {
	ctx, sp := spancontext.StartSpan(context.Background(), "feed-and-sync.fetch")
	defer sp.Finish()

	log.Trace("sleeping", "ruid", ruid)
	time.Sleep(3 * time.Second)

	log.Trace("http get request (feed)", "ruid", ruid, "api", endpoint, "topic", topic, "user", user)

	var tn time.Time
	reqUri := endpoint + "/bzz-feed:/?topic=" + topic + "&user=" + user
	req, _ := http.NewRequest("GET", reqUri, nil)

	opentracing.GlobalTracer().Inject(
		sp.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header))

	trace := client.GetClientTrace("feed-and-sync - http get", "feed-and-sync", ruid, &tn)

	req = req.WithContext(httptrace.WithClientTrace(ctx, trace))
	transport := http.DefaultTransport

//transport.tlsclientconfig=&tls.config不安全的skipverify:true

	tn = time.Now()
	res, err := transport.RoundTrip(req)
	if err != nil {
		log.Error(err.Error(), "ruid", ruid)
		return err
	}

	log.Trace("http get response (feed)", "ruid", ruid, "api", endpoint, "topic", topic, "user", user, "code", res.StatusCode, "len", res.ContentLength)

	if res.StatusCode != 200 {
		return fmt.Errorf("expected status code %d, got %v (ruid %v)", 200, res.StatusCode, ruid)
	}

	defer res.Body.Close()

	rdigest, err := digest(res.Body)
	if err != nil {
		log.Warn(err.Error(), "ruid", ruid)
		return err
	}

	if !bytes.Equal(rdigest, original) {
		err := fmt.Errorf("downloaded imported file md5=%x is not the same as the generated one=%x", rdigest, original)
		log.Warn(err.Error(), "ruid", ruid)
		return err
	}

	log.Trace("downloaded file matches random file", "ruid", ruid, "len", res.ContentLength)

	return nil
}
