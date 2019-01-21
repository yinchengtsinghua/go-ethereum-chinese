
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

//此文件包含用于与Trezor硬件交互的实现
//钱包。有线协议规范可在Satoshilabs网站上找到：
//https://doc.satoshilabs.com/trezor-tech/api-protobuf.html网站

package usbwallet

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/usbwallet/internal/trezor"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/golang/protobuf/proto"
)

//如果打开trezor需要PIN代码，则返回errtrezorpinneed。在
//在这种情况下，调用应用程序应显示一个pinpad并将
//编码的密码短语。
var ErrTrezorPINNeeded = errors.New("trezor: pin needed")

//errTrezorreplyinvalidHeader是Trezor数据交换返回的错误消息
//如果设备回复的标题不匹配。这通常意味着设备
//处于浏览器模式。
var errTrezorReplyInvalidHeader = errors.New("trezor: invalid reply header")

//Trezordriver实现了与Trezor硬件钱包的通信。
type trezorDriver struct {
device  io.ReadWriter //通过USB设备连接进行通信
version [3]uint32     //Trezor固件的当前版本
label   string        //Trezor设备的当前文本标签
pinwait bool          //标记设备是否正在等待PIN输入
failure error         //任何使设备无法使用的故障
log     log.Logger    //上下文记录器，用其ID标记Trezor
}

//NewTrezorDriver创建Trezor USB协议驱动程序的新实例。
func newTrezorDriver(logger log.Logger) driver {
	return &trezorDriver{
		log: logger,
	}
}

//状态实现帐户。钱包，无论Trezor是否打开、关闭
//或者以太坊应用程序是否没有启动。
func (w *trezorDriver) Status() (string, error) {
	if w.failure != nil {
		return fmt.Sprintf("Failed: %v", w.failure), w.failure
	}
	if w.device == nil {
		return "Closed", w.failure
	}
	if w.pinwait {
		return fmt.Sprintf("Trezor v%d.%d.%d '%s' waiting for PIN", w.version[0], w.version[1], w.version[2], w.label), w.failure
	}
	return fmt.Sprintf("Trezor v%d.%d.%d '%s' online", w.version[0], w.version[1], w.version[2], w.label), w.failure
}

//open实现usbwallet.driver，尝试初始化到的连接
//Trezor硬件钱包。初始化trezor是一个两阶段操作：
//*第一阶段是初始化连接并读取钱包的
//特征。如果提供的密码短语为空，则调用此阶段。这个
//设备将显示精确定位结果，并返回相应的
//通知用户需要第二个打开阶段时出错。
//*第二阶段是解锁对trezor的访问，由
//用户实际上提供了一个将键盘键盘映射到PIN的密码短语
//用户数（根据显示的精确定位随机排列）。
func (w *trezorDriver) Open(device io.ReadWriter, passphrase string) error {
	w.device, w.failure = device, nil

//如果请求阶段1，初始化连接并等待用户回调
	if passphrase == "" {
//如果我们已经在等待密码输入，Insta返回
		if w.pinwait {
			return ErrTrezorPINNeeded
		}
//初始化与设备的连接
		features := new(trezor.Features)
		if _, err := w.trezorExchange(&trezor.Initialize{}, features); err != nil {
			return err
		}
		w.version = [3]uint32{features.GetMajorVersion(), features.GetMinorVersion(), features.GetPatchVersion()}
		w.label = features.GetLabel()

//执行手动ping，强制设备请求其PIN
		askPin := true
		res, err := w.trezorExchange(&trezor.Ping{PinProtection: &askPin}, new(trezor.PinMatrixRequest), new(trezor.Success))
		if err != nil {
			return err
		}
//只有在设备尚未解锁时才返回PIN请求
		if res == 1 {
return nil //设备以trezor.success响应。
		}
		w.pinwait = true
		return ErrTrezorPINNeeded
	}
//第2阶段要求实际输入PIN
	w.pinwait = false

	if _, err := w.trezorExchange(&trezor.PinMatrixAck{Pin: &passphrase}, new(trezor.Success)); err != nil {
		w.failure = err
		return err
	}
	return nil
}

//close实现usbwallet.driver，清理和元数据维护在
//Trezor司机。
func (w *trezorDriver) Close() error {
	w.version, w.label, w.pinwait = [3]uint32{}, "", false
	return nil
}

//heartbeat实现usbwallet.driver，对
//Trezor看看它是否仍然在线。
func (w *trezorDriver) Heartbeat() error {
	if _, err := w.trezorExchange(&trezor.Ping{}, new(trezor.Success)); err != nil {
		w.failure = err
		return err
	}
	return nil
}

//派生实现usbwallet.driver，向trezor发送派生请求
//并返回位于该派生路径上的以太坊地址。
func (w *trezorDriver) Derive(path accounts.DerivationPath) (common.Address, error) {
	return w.trezorDerive(path)
}

//signtx实现usbwallet.driver，将事务发送到trezor并
//正在等待用户确认或拒绝该事务。
func (w *trezorDriver) SignTx(path accounts.DerivationPath, tx *types.Transaction, chainID *big.Int) (common.Address, *types.Transaction, error) {
	if w.device == nil {
		return common.Address{}, nil, accounts.ErrWalletClosed
	}
	return w.trezorSign(path, tx, chainID)
}

//TrezorDrive向Trezor设备发送派生请求并返回
//以太坊地址位于该路径上。
func (w *trezorDriver) trezorDerive(derivationPath []uint32) (common.Address, error) {
	address := new(trezor.EthereumAddress)
	if _, err := w.trezorExchange(&trezor.EthereumGetAddress{AddressN: derivationPath}, address); err != nil {
		return common.Address{}, err
	}
	return common.BytesToAddress(address.GetAddress()), nil
}

//Trezorsign将事务发送到Trezor钱包，并等待用户
//确认或拒绝交易。
func (w *trezorDriver) trezorSign(derivationPath []uint32, tx *types.Transaction, chainID *big.Int) (common.Address, *types.Transaction, error) {
//创建事务启动消息
	data := tx.Data()
	length := uint32(len(data))

	request := &trezor.EthereumSignTx{
		AddressN:   derivationPath,
		Nonce:      new(big.Int).SetUint64(tx.Nonce()).Bytes(),
		GasPrice:   tx.GasPrice().Bytes(),
		GasLimit:   new(big.Int).SetUint64(tx.Gas()).Bytes(),
		Value:      tx.Value().Bytes(),
		DataLength: &length,
	}
	if to := tx.To(); to != nil {
request.To = (*to)[:] //非合同部署，显式设置收件人
	}
if length > 1024 { //如果请求发送数据块
		request.DataInitialChunk, data = data[:1024], data[1024:]
	} else {
		request.DataInitialChunk, data = data, nil
	}
if chainID != nil { //EIP-155事务，显式设置链ID（仅支持32位！？）
		id := uint32(chainID.Int64())
		request.ChainId = &id
	}
//发送初始消息和流内容，直到返回签名
	response := new(trezor.EthereumTxRequest)
	if _, err := w.trezorExchange(request, response); err != nil {
		return common.Address{}, nil, err
	}
	for response.DataLength != nil && int(*response.DataLength) <= len(data) {
		chunk := data[:*response.DataLength]
		data = data[*response.DataLength:]

		if _, err := w.trezorExchange(&trezor.EthereumTxAck{DataChunk: chunk}, response); err != nil {
			return common.Address{}, nil, err
		}
	}
//提取以太坊签名并进行健全性验证
	if len(response.GetSignatureR()) == 0 || len(response.GetSignatureS()) == 0 || response.GetSignatureV() == 0 {
		return common.Address{}, nil, errors.New("reply lacks signature")
	}
	signature := append(append(response.GetSignatureR(), response.GetSignatureS()...), byte(response.GetSignatureV()))

//基于链ID创建正确的签名者和签名转换
	var signer types.Signer
	if chainID == nil {
		signer = new(types.HomesteadSigner)
	} else {
		signer = types.NewEIP155Signer(chainID)
		signature[64] -= byte(chainID.Uint64()*2 + 35)
	}
//在事务中插入最终签名并检查发送者的健全性
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

//trezor exchange执行与trezor钱包的数据交换，并向其发送
//并检索响应。如果可能有多个响应，则
//方法还将返回所用目标对象的索引。
func (w *trezorDriver) trezorExchange(req proto.Message, results ...proto.Message) (int, error) {
//构造原始消息有效负载以进行分组
	data, err := proto.Marshal(req)
	if err != nil {
		return 0, err
	}
	payload := make([]byte, 8+len(data))
	copy(payload, []byte{0x23, 0x23})
	binary.BigEndian.PutUint16(payload[2:], trezor.Type(req))
	binary.BigEndian.PutUint32(payload[4:], uint32(len(data)))
	copy(payload[8:], data)

//将所有块流式传输到设备
	chunk := make([]byte, 64)
chunk[0] = 0x3f //报告ID幻数

	for len(payload) > 0 {
//构造新消息到流，如果需要，用零填充
		if len(payload) > 63 {
			copy(chunk[1:], payload[:63])
			payload = payload[63:]
		} else {
			copy(chunk[1:], payload)
			copy(chunk[1+len(payload):], make([]byte, 63-len(payload)))
			payload = nil
		}
//发送到设备
		w.log.Trace("Data chunk sent to the Trezor", "chunk", hexutil.Bytes(chunk))
		if _, err := w.device.Write(chunk); err != nil {
			return 0, err
		}
	}
//将回复从钱包中以64字节的块流式返回
	var (
		kind  uint16
		reply []byte
	)
	for {
//从Trezor钱包中读取下一块
		if _, err := io.ReadFull(w.device, chunk); err != nil {
			return 0, err
		}
		w.log.Trace("Data chunk received from the Trezor", "chunk", hexutil.Bytes(chunk))

//确保传输头匹配
		if chunk[0] != 0x3f || (len(reply) == 0 && (chunk[1] != 0x23 || chunk[2] != 0x23)) {
			return 0, errTrezorReplyInvalidHeader
		}
//如果是第一个块，则检索回复消息类型和总消息长度
		var payload []byte

		if len(reply) == 0 {
			kind = binary.BigEndian.Uint16(chunk[3:5])
			reply = make([]byte, 0, int(binary.BigEndian.Uint32(chunk[5:9])))
			payload = chunk[9:]
		} else {
			payload = chunk[1:]
		}
//追加到答复并在填写时停止
		if left := cap(reply) - len(reply); left > len(payload) {
			reply = append(reply, payload...)
		} else {
			reply = append(reply, payload[:left]...)
			break
		}
	}
//尝试将答复解析为请求的答复消息
	if kind == uint16(trezor.MessageType_MessageType_Failure) {
//Trezor返回一个失败，提取并返回消息
		failure := new(trezor.Failure)
		if err := proto.Unmarshal(reply, failure); err != nil {
			return 0, err
		}
		return 0, errors.New("trezor: " + failure.GetMessage())
	}
	if kind == uint16(trezor.MessageType_MessageType_ButtonRequest) {
//Trezor正在等待用户确认、确认并等待下一条消息
		return w.trezorExchange(&trezor.ButtonAck{}, results...)
	}
	for i, res := range results {
		if trezor.Type(res) == kind {
			return i, proto.Unmarshal(reply, res)
		}
	}
	expected := make([]string, len(results))
	for i, res := range results {
		expected[i] = trezor.Name(trezor.Type(res))
	}
	return 0, fmt.Errorf("trezor: expected reply types %s, got %s", expected, trezor.Name(kind))
}
