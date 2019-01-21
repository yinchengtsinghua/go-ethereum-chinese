
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

//包含core/types包中的所有包装器。

package geth

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv6"
)

//nonce是一个64位散列，它证明（与mix散列组合在一起）
//对一个块进行了足够的计算。
type Nonce struct {
	nonce types.BlockNonce
}

//getbytes检索块nonce的字节表示形式。
func (n *Nonce) GetBytes() []byte {
	return n.nonce[:]
}

//gethex检索块nonce的十六进制字符串表示形式。
func (n *Nonce) GetHex() string {
	return fmt.Sprintf("0x%x", n.nonce[:])
}

//Bloom表示256位Bloom筛选器。
type Bloom struct {
	bloom types.Bloom
}

//GetBytes检索Bloom筛选器的字节表示形式。
func (b *Bloom) GetBytes() []byte {
	return b.bloom[:]
}

//GetHex检索Bloom筛选器的十六进制字符串表示形式。
func (b *Bloom) GetHex() string {
	return fmt.Sprintf("0x%x", b.bloom[:])
}

//header表示以太坊区块链中的区块头。
type Header struct {
	header *types.Header
}

//newheaderfromrlp解析来自rlp数据转储的头。
func NewHeaderFromRLP(data []byte) (*Header, error) {
	h := &Header{
		header: new(types.Header),
	}
	if err := rlp.DecodeBytes(common.CopyBytes(data), h.header); err != nil {
		return nil, err
	}
	return h, nil
}

//encoderlp将头编码为rlp数据转储。
func (h *Header) EncodeRLP() ([]byte, error) {
	return rlp.EncodeToBytes(h.header)
}

//NewHeaderFromJSON解析JSON数据转储中的头。
func NewHeaderFromJSON(data string) (*Header, error) {
	h := &Header{
		header: new(types.Header),
	}
	if err := json.Unmarshal([]byte(data), h.header); err != nil {
		return nil, err
	}
	return h, nil
}

//encodejson将头编码到json数据转储中。
func (h *Header) EncodeJSON() (string, error) {
	data, err := json.Marshal(h.header)
	return string(data), err
}

func (h *Header) GetParentHash() *Hash   { return &Hash{h.header.ParentHash} }
func (h *Header) GetUncleHash() *Hash    { return &Hash{h.header.UncleHash} }
func (h *Header) GetCoinbase() *Address  { return &Address{h.header.Coinbase} }
func (h *Header) GetRoot() *Hash         { return &Hash{h.header.Root} }
func (h *Header) GetTxHash() *Hash       { return &Hash{h.header.TxHash} }
func (h *Header) GetReceiptHash() *Hash  { return &Hash{h.header.ReceiptHash} }
func (h *Header) GetBloom() *Bloom       { return &Bloom{h.header.Bloom} }
func (h *Header) GetDifficulty() *BigInt { return &BigInt{h.header.Difficulty} }
func (h *Header) GetNumber() int64       { return h.header.Number.Int64() }
func (h *Header) GetGasLimit() int64     { return int64(h.header.GasLimit) }
func (h *Header) GetGasUsed() int64      { return int64(h.header.GasUsed) }
func (h *Header) GetTime() int64         { return h.header.Time.Int64() }
func (h *Header) GetExtra() []byte       { return h.header.Extra }
func (h *Header) GetMixDigest() *Hash    { return &Hash{h.header.MixDigest} }
func (h *Header) GetNonce() *Nonce       { return &Nonce{h.header.Nonce} }
func (h *Header) GetHash() *Hash         { return &Hash{h.header.Hash()} }

//Headers表示一个头段。
type Headers struct{ headers []*types.Header }

//SIZE返回切片中的头数。
func (h *Headers) Size() int {
	return len(h.headers)
}

//get返回切片中给定索引处的头。
func (h *Headers) Get(index int) (header *Header, _ error) {
	if index < 0 || index >= len(h.headers) {
		return nil, errors.New("index out of bounds")
	}
	return &Header{h.headers[index]}, nil
}

//块表示以太坊区块链中的整个块。
type Block struct {
	block *types.Block
}

//newblockfromrlp解析来自rlp数据转储的块。
func NewBlockFromRLP(data []byte) (*Block, error) {
	b := &Block{
		block: new(types.Block),
	}
	if err := rlp.DecodeBytes(common.CopyBytes(data), b.block); err != nil {
		return nil, err
	}
	return b, nil
}

//encoderlp将块编码为rlp数据转储。
func (b *Block) EncodeRLP() ([]byte, error) {
	return rlp.EncodeToBytes(b.block)
}

//NewblockFromJSON解析来自JSON数据转储的块。
func NewBlockFromJSON(data string) (*Block, error) {
	b := &Block{
		block: new(types.Block),
	}
	if err := json.Unmarshal([]byte(data), b.block); err != nil {
		return nil, err
	}
	return b, nil
}

//encodejson将块编码为json数据转储。
func (b *Block) EncodeJSON() (string, error) {
	data, err := json.Marshal(b.block)
	return string(data), err
}

func (b *Block) GetParentHash() *Hash           { return &Hash{b.block.ParentHash()} }
func (b *Block) GetUncleHash() *Hash            { return &Hash{b.block.UncleHash()} }
func (b *Block) GetCoinbase() *Address          { return &Address{b.block.Coinbase()} }
func (b *Block) GetRoot() *Hash                 { return &Hash{b.block.Root()} }
func (b *Block) GetTxHash() *Hash               { return &Hash{b.block.TxHash()} }
func (b *Block) GetReceiptHash() *Hash          { return &Hash{b.block.ReceiptHash()} }
func (b *Block) GetBloom() *Bloom               { return &Bloom{b.block.Bloom()} }
func (b *Block) GetDifficulty() *BigInt         { return &BigInt{b.block.Difficulty()} }
func (b *Block) GetNumber() int64               { return b.block.Number().Int64() }
func (b *Block) GetGasLimit() int64             { return int64(b.block.GasLimit()) }
func (b *Block) GetGasUsed() int64              { return int64(b.block.GasUsed()) }
func (b *Block) GetTime() int64                 { return b.block.Time().Int64() }
func (b *Block) GetExtra() []byte               { return b.block.Extra() }
func (b *Block) GetMixDigest() *Hash            { return &Hash{b.block.MixDigest()} }
func (b *Block) GetNonce() int64                { return int64(b.block.Nonce()) }
func (b *Block) GetHash() *Hash                 { return &Hash{b.block.Hash()} }
func (b *Block) GetHeader() *Header             { return &Header{b.block.Header()} }
func (b *Block) GetUncles() *Headers            { return &Headers{b.block.Uncles()} }
func (b *Block) GetTransactions() *Transactions { return &Transactions{b.block.Transactions()} }
func (b *Block) GetTransaction(hash *Hash) *Transaction {
	return &Transaction{b.block.Transaction(hash.hash)}
}

//事务表示单个以太坊事务。
type Transaction struct {
	tx *types.Transaction
}

//newTransaction创建具有给定属性的新事务。
func NewTransaction(nonce int64, to *Address, amount *BigInt, gasLimit int64, gasPrice *BigInt, data []byte) *Transaction {
	return &Transaction{types.NewTransaction(uint64(nonce), to.address, amount.bigint, uint64(gasLimit), gasPrice.bigint, common.CopyBytes(data))}
}

//newTransactionFromRLP解析来自RLP数据转储的事务。
func NewTransactionFromRLP(data []byte) (*Transaction, error) {
	tx := &Transaction{
		tx: new(types.Transaction),
	}
	if err := rlp.DecodeBytes(common.CopyBytes(data), tx.tx); err != nil {
		return nil, err
	}
	return tx, nil
}

//encoderlp将事务编码为rlp数据转储。
func (tx *Transaction) EncodeRLP() ([]byte, error) {
	return rlp.EncodeToBytes(tx.tx)
}

//NewTransactionFromJSON解析来自JSON数据转储的事务。
func NewTransactionFromJSON(data string) (*Transaction, error) {
	tx := &Transaction{
		tx: new(types.Transaction),
	}
	if err := json.Unmarshal([]byte(data), tx.tx); err != nil {
		return nil, err
	}
	return tx, nil
}

//encodejson将事务编码为json数据转储。
func (tx *Transaction) EncodeJSON() (string, error) {
	data, err := json.Marshal(tx.tx)
	return string(data), err
}

func (tx *Transaction) GetData() []byte      { return tx.tx.Data() }
func (tx *Transaction) GetGas() int64        { return int64(tx.tx.Gas()) }
func (tx *Transaction) GetGasPrice() *BigInt { return &BigInt{tx.tx.GasPrice()} }
func (tx *Transaction) GetValue() *BigInt    { return &BigInt{tx.tx.Value()} }
func (tx *Transaction) GetNonce() int64      { return int64(tx.tx.Nonce()) }

func (tx *Transaction) GetHash() *Hash   { return &Hash{tx.tx.Hash()} }
func (tx *Transaction) GetCost() *BigInt { return &BigInt{tx.tx.Cost()} }

//已弃用：GetSighash无法知道要使用哪个签名者。
func (tx *Transaction) GetSigHash() *Hash { return &Hash{types.HomesteadSigner{}.Hash(tx.tx)} }

//已弃用：使用ethereumclient.TransactionSender
func (tx *Transaction) GetFrom(chainID *BigInt) (address *Address, _ error) {
	var signer types.Signer = types.HomesteadSigner{}
	if chainID != nil {
		signer = types.NewEIP155Signer(chainID.bigint)
	}
	from, err := types.Sender(signer, tx.tx)
	return &Address{from}, err
}

func (tx *Transaction) GetTo() *Address {
	if to := tx.tx.To(); to != nil {
		return &Address{*to}
	}
	return nil
}

func (tx *Transaction) WithSignature(sig []byte, chainID *BigInt) (signedTx *Transaction, _ error) {
	var signer types.Signer = types.HomesteadSigner{}
	if chainID != nil {
		signer = types.NewEIP155Signer(chainID.bigint)
	}
	rawTx, err := tx.tx.WithSignature(signer, common.CopyBytes(sig))
	return &Transaction{rawTx}, err
}

//事务表示事务的一部分。
type Transactions struct{ txs types.Transactions }

//SIZE返回切片中的事务数。
func (txs *Transactions) Size() int {
	return len(txs.txs)
}

//get从切片返回给定索引处的事务。
func (txs *Transactions) Get(index int) (tx *Transaction, _ error) {
	if index < 0 || index >= len(txs.txs) {
		return nil, errors.New("index out of bounds")
	}
	return &Transaction{txs.txs[index]}, nil
}

//收据表示交易的结果。
type Receipt struct {
	receipt *types.Receipt
}

//newReceiptFromRLP解析来自RLP数据转储的事务收据。
func NewReceiptFromRLP(data []byte) (*Receipt, error) {
	r := &Receipt{
		receipt: new(types.Receipt),
	}
	if err := rlp.DecodeBytes(common.CopyBytes(data), r.receipt); err != nil {
		return nil, err
	}
	return r, nil
}

//encoderlp将事务收据编码为rlp数据转储。
func (r *Receipt) EncodeRLP() ([]byte, error) {
	return rlp.EncodeToBytes(r.receipt)
}

//NewReceiptFromJSON解析来自JSON数据转储的事务回执。
func NewReceiptFromJSON(data string) (*Receipt, error) {
	r := &Receipt{
		receipt: new(types.Receipt),
	}
	if err := json.Unmarshal([]byte(data), r.receipt); err != nil {
		return nil, err
	}
	return r, nil
}

//encodejson将事务收据编码为json数据转储。
func (r *Receipt) EncodeJSON() (string, error) {
	data, err := rlp.EncodeToBytes(r.receipt)
	return string(data), err
}

func (r *Receipt) GetStatus() int               { return int(r.receipt.Status) }
func (r *Receipt) GetPostState() []byte         { return r.receipt.PostState }
func (r *Receipt) GetCumulativeGasUsed() int64  { return int64(r.receipt.CumulativeGasUsed) }
func (r *Receipt) GetBloom() *Bloom             { return &Bloom{r.receipt.Bloom} }
func (r *Receipt) GetLogs() *Logs               { return &Logs{r.receipt.Logs} }
func (r *Receipt) GetTxHash() *Hash             { return &Hash{r.receipt.TxHash} }
func (r *Receipt) GetContractAddress() *Address { return &Address{r.receipt.ContractAddress} }
func (r *Receipt) GetGasUsed() int64            { return int64(r.receipt.GasUsed) }

//信息表示关于耳语节点的诊断信息。
type Info struct {
	info *whisper.Info
}

//new message表示通过rpc发布的新低语消息。
type NewMessage struct {
	newMessage *whisper.NewMessage
}

func NewNewMessage() *NewMessage {
	nm := &NewMessage{
		newMessage: new(whisper.NewMessage),
	}
	return nm
}

func (nm *NewMessage) GetSymKeyID() string         { return nm.newMessage.SymKeyID }
func (nm *NewMessage) SetSymKeyID(symKeyID string) { nm.newMessage.SymKeyID = symKeyID }
func (nm *NewMessage) GetPublicKey() []byte        { return nm.newMessage.PublicKey }
func (nm *NewMessage) SetPublicKey(publicKey []byte) {
	nm.newMessage.PublicKey = common.CopyBytes(publicKey)
}
func (nm *NewMessage) GetSig() string                  { return nm.newMessage.Sig }
func (nm *NewMessage) SetSig(sig string)               { nm.newMessage.Sig = sig }
func (nm *NewMessage) GetTTL() int64                   { return int64(nm.newMessage.TTL) }
func (nm *NewMessage) SetTTL(ttl int64)                { nm.newMessage.TTL = uint32(ttl) }
func (nm *NewMessage) GetPayload() []byte              { return nm.newMessage.Payload }
func (nm *NewMessage) SetPayload(payload []byte)       { nm.newMessage.Payload = common.CopyBytes(payload) }
func (nm *NewMessage) GetPowTime() int64               { return int64(nm.newMessage.PowTime) }
func (nm *NewMessage) SetPowTime(powTime int64)        { nm.newMessage.PowTime = uint32(powTime) }
func (nm *NewMessage) GetPowTarget() float64           { return nm.newMessage.PowTarget }
func (nm *NewMessage) SetPowTarget(powTarget float64)  { nm.newMessage.PowTarget = powTarget }
func (nm *NewMessage) GetTargetPeer() string           { return nm.newMessage.TargetPeer }
func (nm *NewMessage) SetTargetPeer(targetPeer string) { nm.newMessage.TargetPeer = targetPeer }
func (nm *NewMessage) GetTopic() []byte                { return nm.newMessage.Topic[:] }
func (nm *NewMessage) SetTopic(topic []byte)           { nm.newMessage.Topic = whisper.BytesToTopic(topic) }

//消息表示耳语消息。
type Message struct {
	message *whisper.Message
}

func (m *Message) GetSig() []byte      { return m.message.Sig }
func (m *Message) GetTTL() int64       { return int64(m.message.TTL) }
func (m *Message) GetTimestamp() int64 { return int64(m.message.Timestamp) }
func (m *Message) GetPayload() []byte  { return m.message.Payload }
func (m *Message) GetPoW() float64     { return m.message.PoW }
func (m *Message) GetHash() []byte     { return m.message.Hash }
func (m *Message) GetDst() []byte      { return m.message.Dst }

//消息表示消息数组。
type Messages struct {
	messages []*whisper.Message
}

//SIZE返回切片中的消息数。
func (m *Messages) Size() int {
	return len(m.messages)
}

//get返回切片中给定索引处的消息。
func (m *Messages) Get(index int) (message *Message, _ error) {
	if index < 0 || index >= len(m.messages) {
		return nil, errors.New("index out of bounds")
	}
	return &Message{m.messages[index]}, nil
}

//条件保存入站消息的各种筛选选项。
type Criteria struct {
	criteria *whisper.Criteria
}

func NewCriteria(topic []byte) *Criteria {
	c := &Criteria{
		criteria: new(whisper.Criteria),
	}
	encodedTopic := whisper.BytesToTopic(topic)
	c.criteria.Topics = []whisper.TopicType{encodedTopic}
	return c
}

func (c *Criteria) GetSymKeyID() string                 { return c.criteria.SymKeyID }
func (c *Criteria) SetSymKeyID(symKeyID string)         { c.criteria.SymKeyID = symKeyID }
func (c *Criteria) GetPrivateKeyID() string             { return c.criteria.PrivateKeyID }
func (c *Criteria) SetPrivateKeyID(privateKeyID string) { c.criteria.PrivateKeyID = privateKeyID }
func (c *Criteria) GetSig() []byte                      { return c.criteria.Sig }
func (c *Criteria) SetSig(sig []byte)                   { c.criteria.Sig = common.CopyBytes(sig) }
func (c *Criteria) GetMinPow() float64                  { return c.criteria.MinPow }
func (c *Criteria) SetMinPow(pow float64)               { c.criteria.MinPow = pow }
