
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

//这是一个简单的耳语节点。它可以用作独立的引导节点。
//也可用于不同的测试和诊断目的。

package main

import (
	"bufio"
	"crypto/ecdsa"
	crand "crypto/rand"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/whisper/mailserver"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv6"
	"golang.org/x/crypto/pbkdf2"
)

const quitCommand = "~Q"
const entropySize = 32

//独生子女
var (
	server     *p2p.Server
	shh        *whisper.Whisper
	done       chan struct{}
	mailServer mailserver.WMailServer
	entropy    [entropySize]byte

	input = bufio.NewReader(os.Stdin)
)

//加密
var (
	symKey  []byte
	pub     *ecdsa.PublicKey
	asymKey *ecdsa.PrivateKey
	nodeid  *ecdsa.PrivateKey
	topic   whisper.TopicType

	asymKeyID    string
	asymFilterID string
	symFilterID  string
	symPass      string
	msPassword   string
)

//CMD参数
var (
	bootstrapMode  = flag.Bool("standalone", false, "boostrap node: don't initiate connection to peers, just wait for incoming connections")
	forwarderMode  = flag.Bool("forwarder", false, "forwarder mode: only forward messages, neither encrypt nor decrypt messages")
	mailServerMode = flag.Bool("mailserver", false, "mail server mode: delivers expired messages on demand")
	requestMail    = flag.Bool("mailclient", false, "request expired messages from the bootstrap server")
	asymmetricMode = flag.Bool("asym", false, "use asymmetric encryption")
	generateKey    = flag.Bool("generatekey", false, "generate and show the private key")
	fileExMode     = flag.Bool("fileexchange", false, "file exchange mode")
	fileReader     = flag.Bool("filereader", false, "load and decrypt messages saved as files, display as plain text")
	testMode       = flag.Bool("test", false, "use of predefined parameters for diagnostics (password, etc.)")
	echoMode       = flag.Bool("echo", false, "echo mode: prints some arguments for diagnostics")

	argVerbosity = flag.Int("verbosity", int(log.LvlError), "log verbosity level")
	argTTL       = flag.Uint("ttl", 30, "time-to-live for messages in seconds")
	argWorkTime  = flag.Uint("work", 5, "work time in seconds")
	argMaxSize   = flag.Uint("maxsize", uint(whisper.DefaultMaxMessageSize), "max size of message")
	argPoW       = flag.Float64("pow", whisper.DefaultMinimumPoW, "PoW for normal messages in float format (e.g. 2.7)")
	argServerPoW = flag.Float64("mspow", whisper.DefaultMinimumPoW, "PoW requirement for Mail Server request")

	argIP      = flag.String("ip", "", "IP address and port of this node (e.g. 127.0.0.1:30303)")
	argPub     = flag.String("pub", "", "public key for asymmetric encryption")
	argDBPath  = flag.String("dbpath", "", "path to the server's DB directory")
	argIDFile  = flag.String("idfile", "", "file name with node id (private key)")
argEnode   = flag.String("boot", "", "bootstrap node you want to connect to (e.g. enode://E454……08D50@52.176.211.200:16428）”）
	argTopic   = flag.String("topic", "", "topic in hexadecimal format (e.g. 70a4beef)")
	argSaveDir = flag.String("savedir", "", "directory where all incoming messages will be saved as files")
)

func main() {
	processArgs()
	initialize()
	run()
	shutdown()
}

func processArgs() {
	flag.Parse()

	if len(*argIDFile) > 0 {
		var err error
		nodeid, err = crypto.LoadECDSA(*argIDFile)
		if err != nil {
			utils.Fatalf("Failed to load file [%s]: %s.", *argIDFile, err)
		}
	}

const enodePrefix = "enode://“
	if len(*argEnode) > 0 {
		if (*argEnode)[:len(enodePrefix)] != enodePrefix {
			*argEnode = enodePrefix + *argEnode
		}
	}

	if len(*argTopic) > 0 {
		x, err := hex.DecodeString(*argTopic)
		if err != nil {
			utils.Fatalf("Failed to parse the topic: %s", err)
		}
		topic = whisper.BytesToTopic(x)
	}

	if *asymmetricMode && len(*argPub) > 0 {
		var err error
		if pub, err = crypto.UnmarshalPubkey(common.FromHex(*argPub)); err != nil {
			utils.Fatalf("invalid public key")
		}
	}

	if len(*argSaveDir) > 0 {
		if _, err := os.Stat(*argSaveDir); os.IsNotExist(err) {
			utils.Fatalf("Download directory '%s' does not exist", *argSaveDir)
		}
	} else if *fileExMode {
		utils.Fatalf("Parameter 'savedir' is mandatory for file exchange mode")
	}

	if *echoMode {
		echo()
	}
}

func echo() {
	fmt.Printf("ttl = %d \n", *argTTL)
	fmt.Printf("workTime = %d \n", *argWorkTime)
	fmt.Printf("pow = %f \n", *argPoW)
	fmt.Printf("mspow = %f \n", *argServerPoW)
	fmt.Printf("ip = %s \n", *argIP)
	fmt.Printf("pub = %s \n", common.ToHex(crypto.FromECDSAPub(pub)))
	fmt.Printf("idfile = %s \n", *argIDFile)
	fmt.Printf("dbpath = %s \n", *argDBPath)
	fmt.Printf("boot = %s \n", *argEnode)
}

func initialize() {
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(*argVerbosity), log.StreamHandler(os.Stderr, log.TerminalFormat(false))))

	done = make(chan struct{})
	var peers []*enode.Node
	var err error

	if *generateKey {
		key, err := crypto.GenerateKey()
		if err != nil {
			utils.Fatalf("Failed to generate private key: %s", err)
		}
		k := hex.EncodeToString(crypto.FromECDSA(key))
		fmt.Printf("Random private key: %s \n", k)
		os.Exit(0)
	}

	if *testMode {
symPass = "wwww" //ASCII码：0x77777777
		msPassword = "wwww"
	}

	if *bootstrapMode {
		if len(*argIP) == 0 {
			argIP = scanLineA("Please enter your IP and port (e.g. 127.0.0.1:30348): ")
		}
	} else if *fileReader {
		*bootstrapMode = true
	} else {
		if len(*argEnode) == 0 {
			argEnode = scanLineA("Please enter the peer's enode: ")
		}
		peer := enode.MustParseV4(*argEnode)
		peers = append(peers, peer)
	}

	if *mailServerMode {
		if len(msPassword) == 0 {
			msPassword, err = console.Stdin.PromptPassword("Please enter the Mail Server password: ")
			if err != nil {
				utils.Fatalf("Failed to read Mail Server password: %s", err)
			}
		}
	}

	cfg := &whisper.Config{
		MaxMessageSize:     uint32(*argMaxSize),
		MinimumAcceptedPOW: *argPoW,
	}

	shh = whisper.New(cfg)

	if *argPoW != whisper.DefaultMinimumPoW {
		err := shh.SetMinimumPoW(*argPoW)
		if err != nil {
			utils.Fatalf("Failed to set PoW: %s", err)
		}
	}

	if uint32(*argMaxSize) != whisper.DefaultMaxMessageSize {
		err := shh.SetMaxMessageSize(uint32(*argMaxSize))
		if err != nil {
			utils.Fatalf("Failed to set max message size: %s", err)
		}
	}

	asymKeyID, err = shh.NewKeyPair()
	if err != nil {
		utils.Fatalf("Failed to generate a new key pair: %s", err)
	}

	asymKey, err = shh.GetPrivateKey(asymKeyID)
	if err != nil {
		utils.Fatalf("Failed to retrieve a new key pair: %s", err)
	}

	if nodeid == nil {
		tmpID, err := shh.NewKeyPair()
		if err != nil {
			utils.Fatalf("Failed to generate a new key pair: %s", err)
		}

		nodeid, err = shh.GetPrivateKey(tmpID)
		if err != nil {
			utils.Fatalf("Failed to retrieve a new key pair: %s", err)
		}
	}

	maxPeers := 80
	if *bootstrapMode {
		maxPeers = 800
	}

	_, err = crand.Read(entropy[:])
	if err != nil {
		utils.Fatalf("crypto/rand failed: %s", err)
	}

	if *mailServerMode {
		shh.RegisterServer(&mailServer)
		if err := mailServer.Init(shh, *argDBPath, msPassword, *argServerPoW); err != nil {
			utils.Fatalf("Failed to init MailServer: %s", err)
		}
	}

	server = &p2p.Server{
		Config: p2p.Config{
			PrivateKey:     nodeid,
			MaxPeers:       maxPeers,
			Name:           common.MakeName("wnode", "6.0"),
			Protocols:      shh.Protocols(),
			ListenAddr:     *argIP,
			NAT:            nat.Any(),
			BootstrapNodes: peers,
			StaticNodes:    peers,
			TrustedNodes:   peers,
		},
	}
}

func startServer() error {
	err := server.Start()
	if err != nil {
		fmt.Printf("Failed to start Whisper peer: %s.", err)
		return err
	}

	fmt.Printf("my public key: %s \n", common.ToHex(crypto.FromECDSAPub(&asymKey.PublicKey)))
	fmt.Println(server.NodeInfo().Enode)

	if *bootstrapMode {
		configureNode()
		fmt.Println("Bootstrap Whisper node started")
	} else {
		fmt.Println("Whisper node started")
//首先看看我们是否可以建立连接，然后请求用户输入
		waitForConnection(true)
		configureNode()
	}

	if *fileExMode {
		fmt.Printf("Please type the file name to be send. To quit type: '%s'\n", quitCommand)
	} else if *fileReader {
		fmt.Printf("Please type the file name to be decrypted. To quit type: '%s'\n", quitCommand)
	} else if !*forwarderMode {
		fmt.Printf("Please type the message. To quit type: '%s'\n", quitCommand)
	}
	return nil
}

func configureNode() {
	var err error
	var p2pAccept bool

	if *forwarderMode {
		return
	}

	if *asymmetricMode {
		if len(*argPub) == 0 {
			s := scanLine("Please enter the peer's public key: ")
			b := common.FromHex(s)
			if b == nil {
				utils.Fatalf("Error: can not convert hexadecimal string")
			}
			if pub, err = crypto.UnmarshalPubkey(b); err != nil {
				utils.Fatalf("Error: invalid peer public key")
			}
		}
	}

	if *requestMail {
		p2pAccept = true
		if len(msPassword) == 0 {
			msPassword, err = console.Stdin.PromptPassword("Please enter the Mail Server password: ")
			if err != nil {
				utils.Fatalf("Failed to read Mail Server password: %s", err)
			}
		}
	}

	if !*asymmetricMode && !*forwarderMode {
		if len(symPass) == 0 {
			symPass, err = console.Stdin.PromptPassword("Please enter the password for symmetric encryption: ")
			if err != nil {
				utils.Fatalf("Failed to read passphrase: %v", err)
			}
		}

		symKeyID, err := shh.AddSymKeyFromPassword(symPass)
		if err != nil {
			utils.Fatalf("Failed to create symmetric key: %s", err)
		}
		symKey, err = shh.GetSymKey(symKeyID)
		if err != nil {
			utils.Fatalf("Failed to save symmetric key: %s", err)
		}
		if len(*argTopic) == 0 {
			generateTopic([]byte(symPass))
		}

		fmt.Printf("Filter is configured for the topic: %x \n", topic)
	}

	if *mailServerMode {
		if len(*argDBPath) == 0 {
			argDBPath = scanLineA("Please enter the path to DB file: ")
		}
	}

	symFilter := whisper.Filter{
		KeySym:   symKey,
		Topics:   [][]byte{topic[:]},
		AllowP2P: p2pAccept,
	}
	symFilterID, err = shh.Subscribe(&symFilter)
	if err != nil {
		utils.Fatalf("Failed to install filter: %s", err)
	}

	asymFilter := whisper.Filter{
		KeyAsym:  asymKey,
		Topics:   [][]byte{topic[:]},
		AllowP2P: p2pAccept,
	}
	asymFilterID, err = shh.Subscribe(&asymFilter)
	if err != nil {
		utils.Fatalf("Failed to install filter: %s", err)
	}
}

func generateTopic(password []byte) {
	x := pbkdf2.Key(password, password, 4096, 128, sha512.New)
	for i := 0; i < len(x); i++ {
		topic[i%whisper.TopicLength] ^= x[i]
	}
}

func waitForConnection(timeout bool) {
	var cnt int
	var connected bool
	for !connected {
		time.Sleep(time.Millisecond * 50)
		connected = server.PeerCount() > 0
		if timeout {
			cnt++
			if cnt > 1000 {
				utils.Fatalf("Timeout expired, failed to connect")
			}
		}
	}

	fmt.Println("Connected to peer.")
}

func run() {
	err := startServer()
	if err != nil {
		return
	}
	defer server.Stop()
	shh.Start(nil)
	defer shh.Stop()

	if !*forwarderMode {
		go messageLoop()
	}

	if *requestMail {
		requestExpiredMessagesLoop()
	} else if *fileExMode {
		sendFilesLoop()
	} else if *fileReader {
		fileReaderLoop()
	} else {
		sendLoop()
	}
}

func shutdown() {
	close(done)
	mailServer.Close()
}

func sendLoop() {
	for {
		s := scanLine("")
		if s == quitCommand {
			fmt.Println("Quit command received")
			return
		}
		sendMsg([]byte(s))
		if *asymmetricMode {
//为方便起见，请打印您自己的消息，
//因为在非对称模式下是不可能解密的
			timestamp := time.Now().Unix()
			from := crypto.PubkeyToAddress(asymKey.PublicKey)
			fmt.Printf("\n%d <%x>: %s\n", timestamp, from, s)
		}
	}
}

func sendFilesLoop() {
	for {
		s := scanLine("")
		if s == quitCommand {
			fmt.Println("Quit command received")
			return
		}
		b, err := ioutil.ReadFile(s)
		if err != nil {
			fmt.Printf(">>> Error: %s \n", err)
		} else {
			h := sendMsg(b)
			if (h == common.Hash{}) {
				fmt.Printf(">>> Error: message was not sent \n")
			} else {
				timestamp := time.Now().Unix()
				from := crypto.PubkeyToAddress(asymKey.PublicKey)
				fmt.Printf("\n%d <%x>: sent message with hash %x\n", timestamp, from, h)
			}
		}
	}
}

func fileReaderLoop() {
	watcher1 := shh.GetFilter(symFilterID)
	watcher2 := shh.GetFilter(asymFilterID)
	if watcher1 == nil && watcher2 == nil {
		fmt.Println("Error: neither symmetric nor asymmetric filter is installed")
		return
	}

	for {
		s := scanLine("")
		if s == quitCommand {
			fmt.Println("Quit command received")
			return
		}
		raw, err := ioutil.ReadFile(s)
		if err != nil {
			fmt.Printf(">>> Error: %s \n", err)
		} else {
env := whisper.Envelope{Data: raw} //主题为零
msg := env.Open(watcher1)          //不管主题如何，都强制打开信封
			if msg == nil {
				msg = env.Open(watcher2)
			}
			if msg == nil {
				fmt.Printf(">>> Error: failed to decrypt the message \n")
			} else {
				printMessageInfo(msg)
			}
		}
	}
}

func scanLine(prompt string) string {
	if len(prompt) > 0 {
		fmt.Print(prompt)
	}
	txt, err := input.ReadString('\n')
	if err != nil {
		utils.Fatalf("input error: %s", err)
	}
	txt = strings.TrimRight(txt, "\n\r")
	return txt
}

func scanLineA(prompt string) *string {
	s := scanLine(prompt)
	return &s
}

func scanUint(prompt string) uint32 {
	s := scanLine(prompt)
	i, err := strconv.Atoi(s)
	if err != nil {
		utils.Fatalf("Fail to parse the lower time limit: %s", err)
	}
	return uint32(i)
}

func sendMsg(payload []byte) common.Hash {
	params := whisper.MessageParams{
		Src:      asymKey,
		Dst:      pub,
		KeySym:   symKey,
		Payload:  payload,
		Topic:    topic,
		TTL:      uint32(*argTTL),
		PoW:      *argPoW,
		WorkTime: uint32(*argWorkTime),
	}

	msg, err := whisper.NewSentMessage(&params)
	if err != nil {
		utils.Fatalf("failed to create new message: %s", err)
	}

	envelope, err := msg.Wrap(&params)
	if err != nil {
		fmt.Printf("failed to seal message: %v \n", err)
		return common.Hash{}
	}

	err = shh.Send(envelope)
	if err != nil {
		fmt.Printf("failed to send message: %v \n", err)
		return common.Hash{}
	}

	return envelope.Hash()
}

func messageLoop() {
	sf := shh.GetFilter(symFilterID)
	if sf == nil {
		utils.Fatalf("symmetric filter is not installed")
	}

	af := shh.GetFilter(asymFilterID)
	if af == nil {
		utils.Fatalf("asymmetric filter is not installed")
	}

	ticker := time.NewTicker(time.Millisecond * 50)

	for {
		select {
		case <-ticker.C:
			m1 := sf.Retrieve()
			m2 := af.Retrieve()
			messages := append(m1, m2...)
			for _, msg := range messages {
				reportedOnce := false
				if !*fileExMode && len(msg.Payload) <= 2048 {
					printMessageInfo(msg)
					reportedOnce = true
				}

//在指定argsavedir时保存所有消息。
//FileExMode仅指定消息保存后如何显示在控制台上。
//如果fileexmode==true，则只显示哈希值，因为消息可能太大。
				if len(*argSaveDir) > 0 {
					writeMessageToFile(*argSaveDir, msg, !reportedOnce)
				}
			}
		case <-done:
			return
		}
	}
}

func printMessageInfo(msg *whisper.ReceivedMessage) {
timestamp := fmt.Sprintf("%d", msg.Sent) //用于诊断的Unix时间戳
	text := string(msg.Payload)

	var address common.Address
	if msg.Src != nil {
		address = crypto.PubkeyToAddress(*msg.Src)
	}

	if whisper.IsPubKeyEqual(msg.Src, &asymKey.PublicKey) {
fmt.Printf("\n%s <%x>: %s\n", timestamp, address, text) //我的信息
	} else {
fmt.Printf("\n%s [%x]: %s\n", timestamp, address, text) //来自对等方的消息
	}
}

func writeMessageToFile(dir string, msg *whisper.ReceivedMessage, show bool) {
	if len(dir) == 0 {
		return
	}

	timestamp := fmt.Sprintf("%d", msg.Sent)
	name := fmt.Sprintf("%x", msg.EnvelopeHash)

	var address common.Address
	if msg.Src != nil {
		address = crypto.PubkeyToAddress(*msg.Src)
	}

	env := shh.GetEnvelope(msg.EnvelopeHash)
	if env == nil {
		fmt.Printf("\nUnexpected error: envelope not found: %x\n", msg.EnvelopeHash)
		return
	}

//这是一个示例代码；如果不想保存自己的消息，请取消注释。
//如果whisper.ispubkeyequal（msg.src，&asymkey.publickey）
//fmt.printf（“\n%s<%x>：我收到的消息，未保存：“%s'\n”，时间戳，地址，名称）
//返回
//}

	fullpath := filepath.Join(dir, name)
	err := ioutil.WriteFile(fullpath, env.Data, 0644)
	if err != nil {
		fmt.Printf("\n%s {%x}: message received but not saved: %s\n", timestamp, address, err)
	} else if show {
		fmt.Printf("\n%s {%x}: message received and saved as '%s' (%d bytes)\n", timestamp, address, name, len(env.Data))
	}
}

func requestExpiredMessagesLoop() {
	var key, peerID, bloom []byte
	var timeLow, timeUpp uint32
	var t string
	var xt whisper.TopicType

	keyID, err := shh.AddSymKeyFromPassword(msPassword)
	if err != nil {
		utils.Fatalf("Failed to create symmetric key for mail request: %s", err)
	}
	key, err = shh.GetSymKey(keyID)
	if err != nil {
		utils.Fatalf("Failed to save symmetric key for mail request: %s", err)
	}
	peerID = extractIDFromEnode(*argEnode)
	shh.AllowP2PMessagesFromPeer(peerID)

	for {
		timeLow = scanUint("Please enter the lower limit of the time range (unix timestamp): ")
		timeUpp = scanUint("Please enter the upper limit of the time range (unix timestamp): ")
		t = scanLine("Enter the topic (hex). Press enter to request all messages, regardless of the topic: ")
		if len(t) == whisper.TopicLength*2 {
			x, err := hex.DecodeString(t)
			if err != nil {
				fmt.Printf("Failed to parse the topic: %s \n", err)
				continue
			}
			xt = whisper.BytesToTopic(x)
			bloom = whisper.TopicToBloom(xt)
			obfuscateBloom(bloom)
		} else if len(t) == 0 {
			bloom = whisper.MakeFullNodeBloom()
		} else {
			fmt.Println("Error: topic is invalid, request aborted")
			continue
		}

		if timeUpp == 0 {
			timeUpp = 0xFFFFFFFF
		}

		data := make([]byte, 8, 8+whisper.BloomFilterSize)
		binary.BigEndian.PutUint32(data, timeLow)
		binary.BigEndian.PutUint32(data[4:], timeUpp)
		data = append(data, bloom...)

		var params whisper.MessageParams
		params.PoW = *argServerPoW
		params.Payload = data
		params.KeySym = key
		params.Src = asymKey
		params.WorkTime = 5

		msg, err := whisper.NewSentMessage(&params)
		if err != nil {
			utils.Fatalf("failed to create new message: %s", err)
		}
		env, err := msg.Wrap(&params)
		if err != nil {
			utils.Fatalf("Wrap failed: %s", err)
		}

		err = shh.RequestHistoricMessages(peerID, env)
		if err != nil {
			utils.Fatalf("Failed to send P2P message: %s", err)
		}

		time.Sleep(time.Second * 5)
	}
}

func extractIDFromEnode(s string) []byte {
	n, err := enode.ParseV4(s)
	if err != nil {
		utils.Fatalf("Failed to parse enode: %s", err)
	}
	return n.ID().Bytes()
}

//模糊布卢姆增加16个随机位到布卢姆
//筛选，以便混淆包含的主题。
//它在每个会话中都具有确定性。
//尽管有额外的位，它将平均匹配
//比完整节点的Bloom过滤器少32000倍的消息。
func obfuscateBloom(bloom []byte) {
	const half = entropySize / 2
	for i := 0; i < half; i++ {
		x := int(entropy[i])
		if entropy[half+i] < 128 {
			x += 256
		}

bloom[x/8] = 1 << uint(x%8) //设置位号x
	}
}
