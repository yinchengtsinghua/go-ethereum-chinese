
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

//此文件包含用于与分类帐硬件交互的实现
//钱包。有线协议规范可在分类账Blue Github报告中找到：
//https://raw.githubusercontent.com/ledgerhq/blue-app-eth/master/doc/eth app.asc

package usbwallet

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

//LedgerProcode是对支持的分类帐操作码进行编码的枚举。
type ledgerOpcode byte

//LedgerParam1是一个枚举，用于编码支持的分类帐参数
//特定操作码。相同的参数值可以在操作码之间重复使用。
type ledgerParam1 byte

//LedgerParam2是一个枚举，用于编码支持的分类帐参数
//特定操作码。相同的参数值可以在操作码之间重复使用。
type ledgerParam2 byte

const (
ledgerOpRetrieveAddress  ledgerOpcode = 0x02 //返回给定BIP 32路径的公钥和以太坊地址
ledgerOpSignTransaction  ledgerOpcode = 0x04 //让用户验证参数后签署以太坊事务
ledgerOpGetConfiguration ledgerOpcode = 0x06 //返回特定钱包应用程序配置

ledgerP1DirectlyFetchAddress    ledgerParam1 = 0x00 //直接从钱包返回地址
ledgerP1InitTransactionData     ledgerParam1 = 0x00 //用于签名的第一个事务数据块
ledgerP1ContTransactionData     ledgerParam1 = 0x80 //用于签名的后续事务数据块
ledgerP2DiscardAddressChainCode ledgerParam2 = 0x00 //不要随地址返回链代码
)

//errledgereplyinvalidheader是分类帐数据交换返回的错误消息。
//如果设备回复的标题不匹配。这通常意味着设备
//处于浏览器模式。
var errLedgerReplyInvalidHeader = errors.New("ledger: invalid reply header")

//errlegrinvalidversionreply是由分类帐版本检索返回的错误消息
//当响应确实到达，但它不包含预期的数据时。
var errLedgerInvalidVersionReply = errors.New("ledger: invalid version reply")

//LedgerDriver实现与Ledger硬件钱包的通信。
type ledgerDriver struct {
device  io.ReadWriter //通过USB设备连接进行通信
version [3]byte       //分类帐固件的当前版本（如果应用程序脱机，则为零）
browser bool          //标记分类帐是否处于浏览器模式（答复通道不匹配）
failure error         //任何使设备无法使用的故障
log     log.Logger    //上下文记录器，用其ID标记分类帐
}

//NewledgerDriver创建LedgerUSB协议驱动程序的新实例。
func newLedgerDriver(logger log.Logger) driver {
	return &ledgerDriver{
		log: logger,
	}
}

//状态实现usbwallet.driver，返回分类帐可以返回的各种状态
//目前在。
func (w *ledgerDriver) Status() (string, error) {
	if w.failure != nil {
		return fmt.Sprintf("Failed: %v", w.failure), w.failure
	}
	if w.browser {
		return "Ethereum app in browser mode", w.failure
	}
	if w.offline() {
		return "Ethereum app offline", w.failure
	}
	return fmt.Sprintf("Ethereum app v%d.%d.%d online", w.version[0], w.version[1], w.version[2]), w.failure
}

//离线返回钱包和以太坊应用程序是否离线。
//
//该方法假定状态锁被保持！
func (w *ledgerDriver) offline() bool {
	return w.version == [3]byte{0, 0, 0}
}

//open实现usbwallet.driver，尝试初始化到的连接
//
//参数被静默丢弃。
func (w *ledgerDriver) Open(device io.ReadWriter, passphrase string) error {
	w.device, w.failure = device, nil

	_, err := w.ledgerDerive(accounts.DefaultBaseDerivationPath)
	if err != nil {
//以太坊应用程序未运行或处于浏览器模式，无需执行其他操作，返回
		if err == errLedgerReplyInvalidHeader {
			w.browser = true
		}
		return nil
	}
//尝试解析以太坊应用程序的版本，将在v1.0.2之前失败
	if w.version, err = w.ledgerVersion(); err != nil {
w.version = [3]byte{1, 0, 0} //假设最坏情况，无法验证v1.0.0或v1.0.1
	}
	return nil
}

//close实现usbwallet.driver，清理和元数据维护在
//分类帐驱动程序。
func (w *ledgerDriver) Close() error {
	w.browser, w.version = false, [3]byte{}
	return nil
}

//heartbeat实现usbwallet.driver，对
//看它是否仍然在线。
func (w *ledgerDriver) Heartbeat() error {
	if _, err := w.ledgerVersion(); err != nil && err != errLedgerInvalidVersionReply {
		w.failure = err
		return err
	}
	return nil
}

//派生实现usbwallet.driver，向分类帐发送派生请求
//并返回位于该派生路径上的以太坊地址。
func (w *ledgerDriver) Derive(path accounts.DerivationPath) (common.Address, error) {
	return w.ledgerDerive(path)
}

//signtx实现usbwallet.driver，将事务发送到分类帐并
//正在等待用户确认或拒绝该事务。
//
//注意，如果运行在Ledger钱包上的以太坊应用程序的版本是
//太旧，无法签署EIP-155交易，但要求这样做，还是有错误
//将返回，而不是在宅基地模式中静默签名。
func (w *ledgerDriver) SignTx(path accounts.DerivationPath, tx *types.Transaction, chainID *big.Int) (common.Address, *types.Transaction, error) {
//如果以太坊应用程序不运行，则中止
	if w.offline() {
		return common.Address{}, nil, accounts.ErrWalletClosed
	}
//确保钱包能够签署给定的交易
	if chainID != nil && w.version[0] <= 1 && w.version[1] <= 0 && w.version[2] <= 2 {
		return common.Address{}, nil, fmt.Errorf("Ledger v%d.%d.%d doesn't support signing this transaction, please update to v1.0.3 at least", w.version[0], w.version[1], w.version[2])
	}
//收集的所有信息和元数据均已签出，请求签名
	return w.ledgerSign(path, tx, chainID)
}

//LedgerVersion检索正在运行的以太坊钱包应用程序的当前版本
//在分类帐钱包上。
//
//版本检索协议定义如下：
//
//CLA INS P1 P2 LC LE
//---+-----+---+---+---+----
//e0 06 00 00 04
//
//没有输入数据，输出数据为：
//
//说明长度
//——————————————————————————————————————————————————————————————————————————————————————————————————————————————————————
//标志01：用户1字节启用的任意数据签名
//应用程序主版本1字节
//应用程序次要版本1字节
//应用补丁版本1字节
func (w *ledgerDriver) ledgerVersion() ([3]byte, error) {
//发送请求并等待响应
	reply, err := w.ledgerExchange(ledgerOpGetConfiguration, 0, 0, nil)
	if err != nil {
		return [3]byte{}, err
	}
	if len(reply) != 4 {
		return [3]byte{}, errLedgerInvalidVersionReply
	}
//缓存版本以备将来参考
	var version [3]byte
	copy(version[:], reply[1:])
	return version, nil
}

//LedgerDrive从分类帐中检索当前活动的以太坊地址
//钱包在指定的衍生路径。
//
//地址派生协议定义如下：
//
//CLA INS P1 P2 LC LE
//---+-----+---+---+---+----
//e0 02 00返回地址
//01显示地址，返回前确认
//00:不返回链码
//01：返回链码
//γvar 00
//
//其中输入数据为：
//
//说明长度
//——————————————————————————————————————————————————————————————————————————————————————————————————————————————————————
//要执行的BIP 32派生数（最大10）1字节
//第一个派生索引（big endian）4字节
//……4字节
//上次派生索引（big endian）4字节
//
//输出数据为：
//
//说明长度
//——————————+———————————————————
//公钥长度1字节
//未压缩公钥任意
//以太坊地址长度1字节
//以太坊地址40字节十六进制ASCII
//链码（如果要求）32字节
func (w *ledgerDriver) ledgerDerive(derivationPath []uint32) (common.Address, error) {
//将派生路径展平到分类帐请求中
	path := make([]byte, 1+4*len(derivationPath))
	path[0] = byte(len(derivationPath))
	for i, component := range derivationPath {
		binary.BigEndian.PutUint32(path[1+4*i:], component)
	}
//发送请求并等待响应
	reply, err := w.ledgerExchange(ledgerOpRetrieveAddress, ledgerP1DirectlyFetchAddress, ledgerP2DiscardAddressChainCode, path)
	if err != nil {
		return common.Address{}, err
	}
//丢弃公钥，我们暂时不需要它
	if len(reply) < 1 || len(reply) < 1+int(reply[0]) {
		return common.Address{}, errors.New("reply lacks public key entry")
	}
	reply = reply[1+int(reply[0]):]

//提取以太坊十六进制地址字符串
	if len(reply) < 1 || len(reply) < 1+int(reply[0]) {
		return common.Address{}, errors.New("reply lacks address entry")
	}
	hexstr := reply[1 : 1+int(reply[0])]

//将十六进制sting解码为以太坊地址并返回
	var address common.Address
	if _, err = hex.Decode(address[:], hexstr); err != nil {
		return common.Address{}, err
	}
	return address, nil
}

//Ledgersign将交易发送到Ledger钱包，并等待用户
//确认或拒绝交易。
//
//事务签名协议定义如下：
//
//CLA INS P1 P2 LC LE
//---+-----+---+---+---+----
//e0 04 00：第一个事务数据块
//80：后续交易数据块
//00变量变量
//
//其中，第一个事务块（前255个字节）的输入为：
//
//说明长度
//————————————————————————————————————————————————————————————————————————————————————————————————————————————————————
//要执行的BIP 32派生数（最大10）1字节
//第一个派生索引（big endian）4字节
//……4字节
//上次派生索引（big endian）4字节
//RLP事务块任意
//
//后续事务块（前255个字节）的输入为：
//
//说明长度
//————————+——————————
//RLP事务块任意
//
//输出数据为：
//
//说明长度
//-----------+-----
//签名v 1字节
//签名R 32字节
//签名S 32字节
func (w *ledgerDriver) ledgerSign(derivationPath []uint32, tx *types.Transaction, chainID *big.Int) (common.Address, *types.Transaction, error) {
//将派生路径展平到分类帐请求中
	path := make([]byte, 1+4*len(derivationPath))
	path[0] = byte(len(derivationPath))
	for i, component := range derivationPath {
		binary.BigEndian.PutUint32(path[1+4*i:], component)
	}
//根据请求的是遗留签名还是EIP155签名创建事务RLP
	var (
		txrlp []byte
		err   error
	)
	if chainID == nil {
		if txrlp, err = rlp.EncodeToBytes([]interface{}{tx.Nonce(), tx.GasPrice(), tx.Gas(), tx.To(), tx.Value(), tx.Data()}); err != nil {
			return common.Address{}, nil, err
		}
	} else {
		if txrlp, err = rlp.EncodeToBytes([]interface{}{tx.Nonce(), tx.GasPrice(), tx.Gas(), tx.To(), tx.Value(), tx.Data(), chainID, big.NewInt(0), big.NewInt(0)}); err != nil {
			return common.Address{}, nil, err
		}
	}
	payload := append(path, txrlp...)

//发送请求并等待响应
	var (
		op    = ledgerP1InitTransactionData
		reply []byte
	)
	for len(payload) > 0 {
//计算下一个数据块的大小
		chunk := 255
		if chunk > len(payload) {
			chunk = len(payload)
		}
//发送块，确保正确处理
		reply, err = w.ledgerExchange(ledgerOpSignTransaction, op, 0, payload[:chunk])
		if err != nil {
			return common.Address{}, nil, err
		}
//移动有效负载并确保后续块标记为
		payload = payload[chunk:]
		op = ledgerP1ContTransactionData
	}
//提取以太坊签名并进行健全性验证
	if len(reply) != 65 {
		return common.Address{}, nil, errors.New("reply lacks signature")
	}
	signature := append(reply[1:], reply[0])

//基于链ID创建正确的签名者和签名转换
	var signer types.Signer
	if chainID == nil {
		signer = new(types.HomesteadSigner)
	} else {
		signer = types.NewEIP155Signer(chainID)
		signature[64] -= byte(chainID.Uint64()*2 + 35)
	}
	signed, err := tx.WithSignature(signer, signature)
	if err != nil {
		return common.Address{}, nil, err
	}
	sender, err := types.Sender(signer, signed)
	if err != nil {
		return common.Address{}, nil, err
	}
	return sender, signed, nil
}

//LedgerxChange执行与Ledger钱包的数据交换，并向其发送
//并检索响应。
//
//公共传输头定义如下：
//
//说明长度
//————————————————————————————————————————————————————————————————————————————————————————————————————————————————
//通信信道ID（big endian）2字节
//命令标记1字节
//包序列索引（big endian）2字节
//有效载荷任意
//
//通信信道ID允许命令在同一个信道上复用
//物理链路。暂时不使用，应设置为0101
//为了避免与忽略前导00字节的实现的兼容性问题。
//
//命令标记描述消息内容。使用标签“apdu（0x05）”作为标准
//apdu有效负载，或标记\ping（0x02），用于简单链接测试。
//
//包序列索引描述了分段有效负载的当前序列。
//第一个片段索引是0x00。
//
//APDU命令有效负载编码如下：
//
//说明长度
//—————————————————————————
//apdu长度（big endian）2字节
//apdu cla 1字节
//apdu-ins 1字节
//APDU P1 1字节
//apdu p2 1字节
//apdu长度1字节
//可选APDU数据任意
func (w *ledgerDriver) ledgerExchange(opcode ledgerOpcode, p1 ledgerParam1, p2 ledgerParam2, data []byte) ([]byte, error) {
//构造消息有效负载，可能拆分为多个块
	apdu := make([]byte, 2, 7+len(data))

	binary.BigEndian.PutUint16(apdu, uint16(5+len(data)))
	apdu = append(apdu, []byte{0xe0, byte(opcode), byte(p1), byte(p2), byte(len(data))}...)
	apdu = append(apdu, data...)

//将所有块流式传输到设备
header := []byte{0x01, 0x01, 0x05, 0x00, 0x00} //附加了通道ID和命令标记
	chunk := make([]byte, 64)
	space := len(chunk) - len(header)

	for i := 0; len(apdu) > 0; i++ {
//构造要传输的新消息
		chunk = append(chunk[:0], header...)
		binary.BigEndian.PutUint16(chunk[3:], uint16(i))

		if len(apdu) > space {
			chunk = append(chunk, apdu[:space]...)
			apdu = apdu[space:]
		} else {
			chunk = append(chunk, apdu...)
			apdu = nil
		}
//发送到设备
		w.log.Trace("Data chunk sent to the Ledger", "chunk", hexutil.Bytes(chunk))
		if _, err := w.device.Write(chunk); err != nil {
			return nil, err
		}
	}
//将回复从钱包中以64字节的块流式返回
	var reply []byte
chunk = chunk[:64] //是的，我们肯定有足够的空间
	for {
//从分类帐钱包中读取下一块
		if _, err := io.ReadFull(w.device, chunk); err != nil {
			return nil, err
		}
		w.log.Trace("Data chunk received from the Ledger", "chunk", hexutil.Bytes(chunk))

//确保传输头匹配
		if chunk[0] != 0x01 || chunk[1] != 0x01 || chunk[2] != 0x05 {
			return nil, errLedgerReplyInvalidHeader
		}
//如果是第一个块，则检索消息的总长度
		var payload []byte

		if chunk[3] == 0x00 && chunk[4] == 0x00 {
			reply = make([]byte, 0, int(binary.BigEndian.Uint16(chunk[5:7])))
			payload = chunk[7:]
		} else {
			payload = chunk[5:]
		}
//追加到答复并在填写时停止
		if left := cap(reply) - len(reply); left > len(payload) {
			reply = append(reply, payload...)
		} else {
			reply = append(reply, payload[:left]...)
			break
		}
	}
	return reply[:len(reply)-2], nil
}
