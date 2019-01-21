
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
//此文件是Go以太坊的一部分。
//
//Go以太坊是免费软件：您可以重新发布和/或修改它
//根据GNU通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊的分布希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU通用公共许可证了解更多详细信息。
//
//你应该已经收到一份GNU通用公共许可证的副本
//一起去以太坊吧。如果没有，请参见<http://www.gnu.org/licenses/>。

package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	gorand "math/rand"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/swarm/api"
	swarmapi "github.com/ethereum/go-ethereum/swarm/api/client"
	"github.com/ethereum/go-ethereum/swarm/testutil"
	"golang.org/x/crypto/sha3"
)

const (
	hashRegexp = `[a-f\d]{128}`
	data       = "notsorandomdata"
)

var DefaultCurve = crypto.S256()

func TestACT(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	initCluster(t)

	cases := []struct {
		name string
		f    func(t *testing.T)
	}{
		{"Password", testPassword},
		{"PK", testPK},
		{"ACTWithoutBogus", testACTWithoutBogus},
		{"ACTWithBogus", testACTWithBogus},
	}

	for _, tc := range cases {
		t.Run(tc.name, tc.f)
	}
}

//
//
//
//
//
func testPassword(t *testing.T) {
	dataFilename := testutil.TempFileWithContent(t, data)
	defer os.RemoveAll(dataFilename)

//用“swarm up”上传文件，并期望得到一个哈希值
	up := runSwarm(t,
		"--bzzapi",
		cluster.Nodes[0].URL,
		"up",
		"--encrypt",
		dataFilename)
	_, matches := up.ExpectRegexp(hashRegexp)
	up.ExpectExit()

	if len(matches) < 1 {
		t.Fatal("no matches found")
	}

	ref := matches[0]
	tmp, err := ioutil.TempDir("", "swarm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	password := "smth"
	passwordFilename := testutil.TempFileWithContent(t, "smth")
	defer os.RemoveAll(passwordFilename)

	up = runSwarm(t,
		"access",
		"new",
		"pass",
		"--dry-run",
		"--password",
		passwordFilename,
		ref,
	)

	_, matches = up.ExpectRegexp(".+")
	up.ExpectExit()

	if len(matches) == 0 {
		t.Fatalf("stdout not matched")
	}

	var m api.Manifest

	err = json.Unmarshal([]byte(matches[0]), &m)
	if err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if len(m.Entries) != 1 {
		t.Fatalf("expected one manifest entry, got %v", len(m.Entries))
	}

	e := m.Entries[0]

	ct := "application/bzz-manifest+json"
	if e.ContentType != ct {
		t.Errorf("expected %q content type, got %q", ct, e.ContentType)
	}

	if e.Access == nil {
		t.Fatal("manifest access is nil")
	}

	a := e.Access

	if a.Type != "pass" {
		t.Errorf(`got access type %q, expected "pass"`, a.Type)
	}
	if len(a.Salt) < 32 {
		t.Errorf(`got salt with length %v, expected not less the 32 bytes`, len(a.Salt))
	}
	if a.KdfParams == nil {
		t.Fatal("manifest access kdf params is nil")
	}
	if a.Publisher != "" {
		t.Fatal("should be empty")
	}

	client := swarmapi.NewClient(cluster.Nodes[0].URL)

	hash, err := client.UploadManifest(&m, false)
	if err != nil {
		t.Fatal(err)
	}

	url := cluster.Nodes[0].URL + "/" + "bzz:/" + hash

	httpClient := &http.Client{}
	response, err := httpClient.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatal("should be a 401")
	}
	authHeader := response.Header.Get("WWW-Authenticate")
	if authHeader == "" {
		t.Fatal("should be something here")
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("", password)

	response, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Errorf("expected status %v, got %v", http.StatusOK, response.StatusCode)
	}
	d, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(d) != data {
		t.Errorf("expected decrypted data %q, got %q", data, string(d))
	}

	wrongPasswordFilename := testutil.TempFileWithContent(t, "just wr0ng")
	defer os.RemoveAll(wrongPasswordFilename)

//下载带有错误密码的“swarm down”文件
	up = runSwarm(t,
		"--bzzapi",
		cluster.Nodes[0].URL,
		"down",
		"bzz:/"+hash,
		tmp,
		"--password",
		wrongPasswordFilename)

	_, matches = up.ExpectRegexp("unauthorized")
	if len(matches) != 1 && matches[0] != "unauthorized" {
		t.Fatal(`"unauthorized" not found in output"`)
	}
	up.ExpectExit()
}

//testpk测试在双方（发布者和被授予者）之间正确创建行为清单。
//测试创建伪内容，将其加密上载，然后使用访问项创建包装清单。
//参与方-节点（发布者），上载到第二个节点（也是被授予者），然后消失。
//
//
func testPK(t *testing.T) {
	dataFilename := testutil.TempFileWithContent(t, data)
	defer os.RemoveAll(dataFilename)

//用“swarm up”上传文件，并期望得到一个哈希值
	up := runSwarm(t,
		"--bzzapi",
		cluster.Nodes[0].URL,
		"up",
		"--encrypt",
		dataFilename)
	_, matches := up.ExpectRegexp(hashRegexp)
	up.ExpectExit()

	if len(matches) < 1 {
		t.Fatal("no matches found")
	}

	ref := matches[0]
	pk := cluster.Nodes[0].PrivateKey
	granteePubKey := crypto.CompressPubkey(&pk.PublicKey)

	publisherDir, err := ioutil.TempDir("", "swarm-account-dir-temp")
	if err != nil {
		t.Fatal(err)
	}

	passwordFilename := testutil.TempFileWithContent(t, testPassphrase)
	defer os.RemoveAll(passwordFilename)

	_, publisherAccount := getTestAccount(t, publisherDir)
	up = runSwarm(t,
		"--bzzaccount",
		publisherAccount.Address.String(),
		"--password",
		passwordFilename,
		"--datadir",
		publisherDir,
		"--bzzapi",
		cluster.Nodes[0].URL,
		"access",
		"new",
		"pk",
		"--dry-run",
		"--grant-key",
		hex.EncodeToString(granteePubKey),
		ref,
	)

	_, matches = up.ExpectRegexp(".+")
	up.ExpectExit()

	if len(matches) == 0 {
		t.Fatalf("stdout not matched")
	}

//
	publicKeyFromDataDir := runSwarm(t,
		"--bzzaccount",
		publisherAccount.Address.String(),
		"--password",
		passwordFilename,
		"--datadir",
		publisherDir,
		"print-keys",
		"--compressed",
	)
	_, publicKeyString := publicKeyFromDataDir.ExpectRegexp(".+")
	publicKeyFromDataDir.ExpectExit()
	pkComp := strings.Split(publicKeyString[0], "=")[1]
	var m api.Manifest

	err = json.Unmarshal([]byte(matches[0]), &m)
	if err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if len(m.Entries) != 1 {
		t.Fatalf("expected one manifest entry, got %v", len(m.Entries))
	}

	e := m.Entries[0]

	ct := "application/bzz-manifest+json"
	if e.ContentType != ct {
		t.Errorf("expected %q content type, got %q", ct, e.ContentType)
	}

	if e.Access == nil {
		t.Fatal("manifest access is nil")
	}

	a := e.Access

	if a.Type != "pk" {
		t.Errorf(`got access type %q, expected "pk"`, a.Type)
	}
	if len(a.Salt) < 32 {
		t.Errorf(`got salt with length %v, expected not less the 32 bytes`, len(a.Salt))
	}
	if a.KdfParams != nil {
		t.Fatal("manifest access kdf params should be nil")
	}
	if a.Publisher != pkComp {
		t.Fatal("publisher key did not match")
	}
	client := swarmapi.NewClient(cluster.Nodes[0].URL)

	hash, err := client.UploadManifest(&m, false)
	if err != nil {
		t.Fatal(err)
	}

	httpClient := &http.Client{}

	url := cluster.Nodes[0].URL + "/" + "bzz:/" + hash
	response, err := httpClient.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatal("should be a 200")
	}
	d, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(d) != data {
		t.Errorf("expected decrypted data %q, got %q", data, string(d))
	}
}

//
func testACTWithoutBogus(t *testing.T) {
	testACT(t, 0)
}

//
func testACTWithBogus(t *testing.T) {
	testACT(t, 100)
}

//测试测试E2E的创建、上传和下载，带有EC密钥和密码保护的ACT访问控制
//测试触发一个3节点的集群，然后随机选择2个节点，这些节点将充当数据的被授予者。
//设置并使用密码保护行为。第三个节点应该无法对引用进行解码，因为它不会被授予访问权限。
//
//
func testACT(t *testing.T, bogusEntries int) {
	var uploadThroughNode = cluster.Nodes[0]
	client := swarmapi.NewClient(uploadThroughNode.URL)

	r1 := gorand.New(gorand.NewSource(time.Now().UnixNano()))
nodeToSkip := r1.Intn(clusterSize) //
	dataFilename := testutil.TempFileWithContent(t, data)
	defer os.RemoveAll(dataFilename)

//用“swarm up”上传文件，并期望得到一个哈希值
	up := runSwarm(t,
		"--bzzapi",
		cluster.Nodes[0].URL,
		"up",
		"--encrypt",
		dataFilename)
	_, matches := up.ExpectRegexp(hashRegexp)
	up.ExpectExit()

	if len(matches) < 1 {
		t.Fatal("no matches found")
	}

	ref := matches[0]
	grantees := []string{}
	for i, v := range cluster.Nodes {
		if i == nodeToSkip {
			continue
		}
		pk := v.PrivateKey
		granteePubKey := crypto.CompressPubkey(&pk.PublicKey)
		grantees = append(grantees, hex.EncodeToString(granteePubKey))
	}

	if bogusEntries > 0 {
		bogusGrantees := []string{}

		for i := 0; i < bogusEntries; i++ {
			prv, err := ecies.GenerateKey(rand.Reader, DefaultCurve, nil)
			if err != nil {
				t.Fatal(err)
			}
			bogusGrantees = append(bogusGrantees, hex.EncodeToString(crypto.CompressPubkey(&prv.ExportECDSA().PublicKey)))
		}
		r2 := gorand.New(gorand.NewSource(time.Now().UnixNano()))
		for i := 0; i < len(grantees); i++ {
			insertAtIdx := r2.Intn(len(bogusGrantees))
			bogusGrantees = append(bogusGrantees[:insertAtIdx], append([]string{grantees[i]}, bogusGrantees[insertAtIdx:]...)...)
		}
		grantees = bogusGrantees
	}
	granteesPubkeyListFile := testutil.TempFileWithContent(t, strings.Join(grantees, "\n"))
	defer os.RemoveAll(granteesPubkeyListFile)

	publisherDir, err := ioutil.TempDir("", "swarm-account-dir-temp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(publisherDir)

	passwordFilename := testutil.TempFileWithContent(t, testPassphrase)
	defer os.RemoveAll(passwordFilename)
	actPasswordFilename := testutil.TempFileWithContent(t, "smth")
	defer os.RemoveAll(actPasswordFilename)
	_, publisherAccount := getTestAccount(t, publisherDir)
	up = runSwarm(t,
		"--bzzaccount",
		publisherAccount.Address.String(),
		"--password",
		passwordFilename,
		"--datadir",
		publisherDir,
		"--bzzapi",
		cluster.Nodes[0].URL,
		"access",
		"new",
		"act",
		"--grant-keys",
		granteesPubkeyListFile,
		"--password",
		actPasswordFilename,
		ref,
	)

	_, matches = up.ExpectRegexp(`[a-f\d]{64}`)
	up.ExpectExit()

	if len(matches) == 0 {
		t.Fatalf("stdout not matched")
	}

//
	publicKeyFromDataDir := runSwarm(t,
		"--bzzaccount",
		publisherAccount.Address.String(),
		"--password",
		passwordFilename,
		"--datadir",
		publisherDir,
		"print-keys",
		"--compressed",
	)
	_, publicKeyString := publicKeyFromDataDir.ExpectRegexp(".+")
	publicKeyFromDataDir.ExpectExit()
	pkComp := strings.Split(publicKeyString[0], "=")[1]

	hash := matches[0]
	m, _, err := client.DownloadManifest(hash)
	if err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if len(m.Entries) != 1 {
		t.Fatalf("expected one manifest entry, got %v", len(m.Entries))
	}

	e := m.Entries[0]

	ct := "application/bzz-manifest+json"
	if e.ContentType != ct {
		t.Errorf("expected %q content type, got %q", ct, e.ContentType)
	}

	if e.Access == nil {
		t.Fatal("manifest access is nil")
	}

	a := e.Access

	if a.Type != "act" {
		t.Fatalf(`got access type %q, expected "act"`, a.Type)
	}
	if len(a.Salt) < 32 {
		t.Fatalf(`got salt with length %v, expected not less the 32 bytes`, len(a.Salt))
	}

	if a.Publisher != pkComp {
		t.Fatal("publisher key did not match")
	}
	httpClient := &http.Client{}

//除了跳过的节点之外，所有节点都应该能够解密内容
	for i, node := range cluster.Nodes {
		log.Debug("trying to fetch from node", "node index", i)

		url := node.URL + "/" + "bzz:/" + hash
		response, err := httpClient.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		log.Debug("got response from node", "response code", response.StatusCode)

		if i == nodeToSkip {
			log.Debug("reached node to skip", "status code", response.StatusCode)

			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("should be a 401")
			}

//
passwordUrl := strings.Replace(url, "http://
			response, err = httpClient.Get(passwordUrl)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusOK {
				t.Fatal("should be a 200")
			}

//现在尝试使用错误的密码，预计401
passwordUrl = strings.Replace(url, "http://
			response, err = httpClient.Get(passwordUrl)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatal("should be a 401")
			}
			continue
		}

		if response.StatusCode != http.StatusOK {
			t.Fatal("should be a 200")
		}
		d, err := ioutil.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(d) != data {
			t.Errorf("expected decrypted data %q, got %q", data, string(d))
		}
	}
}

//
//
func TestKeypairSanity(t *testing.T) {
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		t.Fatalf("reading from crypto/rand failed: %v", err.Error())
	}
	sharedSecret := "a85586744a1ddd56a7ed9f33fa24f40dd745b3a941be296a0d60e329dbdb896d"

	for i, v := range []struct {
		publisherPriv string
		granteePub    string
	}{
		{
			publisherPriv: "ec5541555f3bc6376788425e9d1a62f55a82901683fd7062c5eddcc373a73459",
			granteePub:    "0226f213613e843a413ad35b40f193910d26eb35f00154afcde9ded57479a6224a",
		},
		{
			publisherPriv: "70c7a73011aa56584a0009ab874794ee7e5652fd0c6911cd02f8b6267dd82d2d",
			granteePub:    "02e6f8d5e28faaa899744972bb847b6eb805a160494690c9ee7197ae9f619181db",
		},
	} {
		b, _ := hex.DecodeString(v.granteePub)
		granteePub, _ := crypto.DecompressPubkey(b)
		publisherPrivate, _ := crypto.HexToECDSA(v.publisherPriv)

		ssKey, err := api.NewSessionKeyPK(publisherPrivate, granteePub, salt)
		if err != nil {
			t.Fatal(err)
		}

		hasher := sha3.NewLegacyKeccak256()
		hasher.Write(salt)
		shared, err := hex.DecodeString(sharedSecret)
		if err != nil {
			t.Fatal(err)
		}
		hasher.Write(shared)
		sum := hasher.Sum(nil)

		if !bytes.Equal(ssKey, sum) {
			t.Fatalf("%d: got a session key mismatch", i)
		}
	}
}
