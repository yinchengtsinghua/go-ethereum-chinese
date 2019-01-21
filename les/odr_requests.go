
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

//package light实现可按需检索的状态和链对象
//对于以太坊Light客户端。
package les

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/light"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

var (
	errInvalidMessageType  = errors.New("invalid message type")
	errInvalidEntryCount   = errors.New("invalid number of response entries")
	errHeaderUnavailable   = errors.New("header unavailable")
	errTxHashMismatch      = errors.New("transaction hash mismatch")
	errUncleHashMismatch   = errors.New("uncle hash mismatch")
	errReceiptHashMismatch = errors.New("receipt hash mismatch")
	errDataHashMismatch    = errors.New("data hash mismatch")
	errCHTHashMismatch     = errors.New("cht hash mismatch")
	errCHTNumberMismatch   = errors.New("cht number mismatch")
	errUselessNodes        = errors.New("useless nodes in merkle proof nodeset")
)

type LesOdrRequest interface {
	GetCost(*peer) uint64
	CanSend(*peer) bool
	Request(uint64, *peer) error
	Validate(ethdb.Database, *Msg) error
}

func LesRequest(req light.OdrRequest) LesOdrRequest {
	switch r := req.(type) {
	case *light.BlockRequest:
		return (*BlockRequest)(r)
	case *light.ReceiptsRequest:
		return (*ReceiptsRequest)(r)
	case *light.TrieRequest:
		return (*TrieRequest)(r)
	case *light.CodeRequest:
		return (*CodeRequest)(r)
	case *light.ChtRequest:
		return (*ChtRequest)(r)
	case *light.BloomRequest:
		return (*BloomRequest)(r)
	default:
		return nil
	}
}

//BlockRequest是块体的ODR请求类型
type BlockRequest light.BlockRequest

//getcost根据服务返回给定ODR请求的成本
//同行成本表（lesodrequest的实现）
func (r *BlockRequest) GetCost(peer *peer) uint64 {
	return peer.GetRequestCost(GetBlockBodiesMsg, 1)
}

//cansend告诉某个对等机是否适合服务于给定的请求
func (r *BlockRequest) CanSend(peer *peer) bool {
	return peer.HasBlock(r.Hash, r.Number, false)
}

//请求向LES网络发送一个ODR请求（LESODRREQUEST的实现）
func (r *BlockRequest) Request(reqID uint64, peer *peer) error {
	peer.Log().Debug("Requesting block body", "hash", r.Hash)
	return peer.RequestBodies(reqID, r.GetCost(peer), []common.Hash{r.Hash})
}

//有效处理来自LES网络的ODR请求回复消息
//returns true and stores results in memory if the message was a valid reply
//到请求（lesodrequest的实现）
func (r *BlockRequest) Validate(db ethdb.Database, msg *Msg) error {
	log.Debug("Validating block body", "hash", r.Hash)

//确保我们有一个正确的信息与一个单一的块体
	if msg.MsgType != MsgBlockBodies {
		return errInvalidMessageType
	}
	bodies := msg.Obj.([]*types.Body)
	if len(bodies) != 1 {
		return errInvalidEntryCount
	}
	body := bodies[0]

//检索存储的头并根据它验证块内容
	header := rawdb.ReadHeader(db, r.Hash, r.Number)
	if header == nil {
		return errHeaderUnavailable
	}
	if header.TxHash != types.DeriveSha(types.Transactions(body.Transactions)) {
		return errTxHashMismatch
	}
	if header.UncleHash != types.CalcUncleHash(body.Uncles) {
		return errUncleHashMismatch
	}
//Validations passed, encode and store RLP
	data, err := rlp.EncodeToBytes(body)
	if err != nil {
		return err
	}
	r.Rlp = data
	return nil
}

//ReceiptsRequest是按块哈希列出的块接收的ODR请求类型
type ReceiptsRequest light.ReceiptsRequest

//getcost根据服务返回给定ODR请求的成本
//同行成本表（lesodrequest的实现）
func (r *ReceiptsRequest) GetCost(peer *peer) uint64 {
	return peer.GetRequestCost(GetReceiptsMsg, 1)
}

//cansend告诉某个对等机是否适合服务于给定的请求
func (r *ReceiptsRequest) CanSend(peer *peer) bool {
	return peer.HasBlock(r.Hash, r.Number, false)
}

//请求向LES网络发送一个ODR请求（LESODRREQUEST的实现）
func (r *ReceiptsRequest) Request(reqID uint64, peer *peer) error {
	peer.Log().Debug("Requesting block receipts", "hash", r.Hash)
	return peer.RequestReceipts(reqID, r.GetCost(peer), []common.Hash{r.Hash})
}

//有效处理来自LES网络的ODR请求回复消息
//如果消息是有效的答复，则返回true并将结果存储在内存中
//到请求（lesodrequest的实现）
func (r *ReceiptsRequest) Validate(db ethdb.Database, msg *Msg) error {
	log.Debug("Validating block receipts", "hash", r.Hash)

//确保我们有一个正确的消息和一个单块收据
	if msg.MsgType != MsgReceipts {
		return errInvalidMessageType
	}
	receipts := msg.Obj.([]types.Receipts)
	if len(receipts) != 1 {
		return errInvalidEntryCount
	}
	receipt := receipts[0]

//检索存储的标题并根据其验证收据内容
	header := rawdb.ReadHeader(db, r.Hash, r.Number)
	if header == nil {
		return errHeaderUnavailable
	}
	if header.ReceiptHash != types.DeriveSha(receipt) {
		return errReceiptHashMismatch
	}
//验证通过，存储并返回
	r.Receipts = receipt
	return nil
}

type ProofReq struct {
	BHash       common.Hash
	AccKey, Key []byte
	FromLevel   uint
}

//状态/存储trie项的ODR请求类型，请参见leSodrRequest接口
type TrieRequest light.TrieRequest

//getcost根据服务返回给定ODR请求的成本
//同行成本表（lesodrequest的实现）
func (r *TrieRequest) GetCost(peer *peer) uint64 {
	switch peer.version {
	case lpv1:
		return peer.GetRequestCost(GetProofsV1Msg, 1)
	case lpv2:
		return peer.GetRequestCost(GetProofsV2Msg, 1)
	default:
		panic(nil)
	}
}

//cansend告诉某个对等机是否适合服务于给定的请求
func (r *TrieRequest) CanSend(peer *peer) bool {
	return peer.HasBlock(r.Id.BlockHash, r.Id.BlockNumber, true)
}

//请求向LES网络发送一个ODR请求（LESODRREQUEST的实现）
func (r *TrieRequest) Request(reqID uint64, peer *peer) error {
	peer.Log().Debug("Requesting trie proof", "root", r.Id.Root, "key", r.Key)
	req := ProofReq{
		BHash:  r.Id.BlockHash,
		AccKey: r.Id.AccKey,
		Key:    r.Key,
	}
	return peer.RequestProofs(reqID, r.GetCost(peer), []ProofReq{req})
}

//有效处理来自LES网络的ODR请求回复消息
//如果消息是有效的答复，则返回true并将结果存储在内存中
//到请求（lesodrequest的实现）
func (r *TrieRequest) Validate(db ethdb.Database, msg *Msg) error {
	log.Debug("Validating trie proof", "root", r.Id.Root, "key", r.Key)

	switch msg.MsgType {
	case MsgProofsV1:
		proofs := msg.Obj.([]light.NodeList)
		if len(proofs) != 1 {
			return errInvalidEntryCount
		}
		nodeSet := proofs[0].NodeSet()
//验证证明，如果签出则保存
		if _, _, err := trie.VerifyProof(r.Id.Root, r.Key, nodeSet); err != nil {
			return fmt.Errorf("merkle proof verification failed: %v", err)
		}
		r.Proof = nodeSet
		return nil

	case MsgProofsV2:
		proofs := msg.Obj.(light.NodeList)
//验证证明，如果签出则保存
		nodeSet := proofs.NodeSet()
		reads := &readTraceDB{db: nodeSet}
		if _, _, err := trie.VerifyProof(r.Id.Root, r.Key, reads); err != nil {
			return fmt.Errorf("merkle proof verification failed: %v", err)
		}
//检查VerifyProof是否已读取所有节点
		if len(reads.reads) != nodeSet.KeyCount() {
			return errUselessNodes
		}
		r.Proof = nodeSet
		return nil

	default:
		return errInvalidMessageType
	}
}

type CodeReq struct {
	BHash  common.Hash
	AccKey []byte
}

//节点数据的ODR请求类型（用于检索合同代码），请参见LESODRREQUEST接口
type CodeRequest light.CodeRequest

//getcost根据服务返回给定ODR请求的成本
//同行成本表（lesodrequest的实现）
func (r *CodeRequest) GetCost(peer *peer) uint64 {
	return peer.GetRequestCost(GetCodeMsg, 1)
}

//cansend告诉某个对等机是否适合服务于给定的请求
func (r *CodeRequest) CanSend(peer *peer) bool {
	return peer.HasBlock(r.Id.BlockHash, r.Id.BlockNumber, true)
}

//请求向LES网络发送一个ODR请求（LESODRREQUEST的实现）
func (r *CodeRequest) Request(reqID uint64, peer *peer) error {
	peer.Log().Debug("Requesting code data", "hash", r.Hash)
	req := CodeReq{
		BHash:  r.Id.BlockHash,
		AccKey: r.Id.AccKey,
	}
	return peer.RequestCode(reqID, r.GetCost(peer), []CodeReq{req})
}

//有效处理来自LES网络的ODR请求回复消息
//如果消息是有效的答复，则返回true并将结果存储在内存中
//到请求（lesodrequest的实现）
func (r *CodeRequest) Validate(db ethdb.Database, msg *Msg) error {
	log.Debug("Validating code data", "hash", r.Hash)

//确保我们有一个带有单个代码元素的正确消息
	if msg.MsgType != MsgCode {
		return errInvalidMessageType
	}
	reply := msg.Obj.([][]byte)
	if len(reply) != 1 {
		return errInvalidEntryCount
	}
	data := reply[0]

//验证数据并存储是否签出
	if hash := crypto.Keccak256Hash(data); r.Hash != hash {
		return errDataHashMismatch
	}
	r.Data = data
	return nil
}

const (
//helper trie类型常量
htCanonical = iota //规范哈希trie
htBloomBits        //布卢姆斯特里

//适用于所有helper trie请求
	auxRoot = 1
//适用于htcanonical
	auxHeader = 2
)

type HelperTrieReq struct {
	Type              uint
	TrieIdx           uint64
	Key               []byte
	FromLevel, AuxReq uint
}

type HelperTrieResps struct { //描述所有响应，而不仅仅是单个响应
	Proofs  light.NodeList
	AuxData [][]byte
}

//遗产LES / 1
type ChtReq struct {
	ChtNum, BlockNum uint64
	FromLevel        uint
}

//遗产LES / 1
type ChtResp struct {
	Header *types.Header
	Proof  []rlp.RawValue
}

//ODR request type for requesting headers by Canonical Hash Trie, see LesOdrRequest interface
type ChtRequest light.ChtRequest

//getcost根据服务返回给定ODR请求的成本
//同行成本表（lesodrequest的实现）
func (r *ChtRequest) GetCost(peer *peer) uint64 {
	switch peer.version {
	case lpv1:
		return peer.GetRequestCost(GetHeaderProofsMsg, 1)
	case lpv2:
		return peer.GetRequestCost(GetHelperTrieProofsMsg, 1)
	default:
		panic(nil)
	}
}

//cansend告诉某个对等机是否适合服务于给定的请求
func (r *ChtRequest) CanSend(peer *peer) bool {
	peer.lock.RLock()
	defer peer.lock.RUnlock()

	return peer.headInfo.Number >= r.Config.ChtConfirms && r.ChtNum <= (peer.headInfo.Number-r.Config.ChtConfirms)/r.Config.ChtSize
}

//请求向LES网络发送一个ODR请求（LESODRREQUEST的实现）
func (r *ChtRequest) Request(reqID uint64, peer *peer) error {
	peer.Log().Debug("Requesting CHT", "cht", r.ChtNum, "block", r.BlockNum)
	var encNum [8]byte
	binary.BigEndian.PutUint64(encNum[:], r.BlockNum)
	req := HelperTrieReq{
		Type:    htCanonical,
		TrieIdx: r.ChtNum,
		Key:     encNum[:],
		AuxReq:  auxHeader,
	}
	switch peer.version {
	case lpv1:
		var reqsV1 ChtReq
		if req.Type != htCanonical || req.AuxReq != auxHeader || len(req.Key) != 8 {
			return fmt.Errorf("Request invalid in LES/1 mode")
		}
		blockNum := binary.BigEndian.Uint64(req.Key)
//将helpertrie请求转换为旧的cht请求
		reqsV1 = ChtReq{ChtNum: (req.TrieIdx + 1) * (r.Config.ChtSize / r.Config.PairChtSize), BlockNum: blockNum, FromLevel: req.FromLevel}
		return peer.RequestHelperTrieProofs(reqID, r.GetCost(peer), []ChtReq{reqsV1})
	case lpv2:
		return peer.RequestHelperTrieProofs(reqID, r.GetCost(peer), []HelperTrieReq{req})
	default:
		panic(nil)
	}
}

//有效处理来自LES网络的ODR请求回复消息
//如果消息是有效的答复，则返回true并将结果存储在内存中
//到请求（lesodrequest的实现）
func (r *ChtRequest) Validate(db ethdb.Database, msg *Msg) error {
	log.Debug("Validating CHT", "cht", r.ChtNum, "block", r.BlockNum)

	switch msg.MsgType {
case MsgHeaderProofs: //LES/1向后兼容性
		proofs := msg.Obj.([]ChtResp)
		if len(proofs) != 1 {
			return errInvalidEntryCount
		}
		proof := proofs[0]

//验证CHT
		var encNumber [8]byte
		binary.BigEndian.PutUint64(encNumber[:], r.BlockNum)

		value, _, err := trie.VerifyProof(r.ChtRoot, encNumber[:], light.NodeList(proof.Proof).NodeSet())
		if err != nil {
			return err
		}
		var node light.ChtNode
		if err := rlp.DecodeBytes(value, &node); err != nil {
			return err
		}
		if node.Hash != proof.Header.Hash() {
			return errCHTHashMismatch
		}
//验证通过，存储并返回
		r.Header = proof.Header
		r.Proof = light.NodeList(proof.Proof).NodeSet()
		r.Td = node.Td
	case MsgHelperTrieProofs:
		resp := msg.Obj.(HelperTrieResps)
		if len(resp.AuxData) != 1 {
			return errInvalidEntryCount
		}
		nodeSet := resp.Proofs.NodeSet()
		headerEnc := resp.AuxData[0]
		if len(headerEnc) == 0 {
			return errHeaderUnavailable
		}
		header := new(types.Header)
		if err := rlp.DecodeBytes(headerEnc, header); err != nil {
			return errHeaderUnavailable
		}

//验证CHT
		var encNumber [8]byte
		binary.BigEndian.PutUint64(encNumber[:], r.BlockNum)

		reads := &readTraceDB{db: nodeSet}
		value, _, err := trie.VerifyProof(r.ChtRoot, encNumber[:], reads)
		if err != nil {
			return fmt.Errorf("merkle proof verification failed: %v", err)
		}
		if len(reads.reads) != nodeSet.KeyCount() {
			return errUselessNodes
		}

		var node light.ChtNode
		if err := rlp.DecodeBytes(value, &node); err != nil {
			return err
		}
		if node.Hash != header.Hash() {
			return errCHTHashMismatch
		}
		if r.BlockNum != header.Number.Uint64() {
			return errCHTNumberMismatch
		}
//验证通过，存储并返回
		r.Header = header
		r.Proof = nodeSet
		r.Td = node.Td
	default:
		return errInvalidMessageType
	}
	return nil
}

type BloomReq struct {
	BloomTrieNum, BitIdx, SectionIndex, FromLevel uint64
}

//用于通过规范哈希检索请求头的ODR请求类型，请参见leSodrRequest接口
type BloomRequest light.BloomRequest

//getcost根据服务返回给定ODR请求的成本
//同行成本表（lesodrequest的实现）
func (r *BloomRequest) GetCost(peer *peer) uint64 {
	return peer.GetRequestCost(GetHelperTrieProofsMsg, len(r.SectionIndexList))
}

//cansend告诉某个对等机是否适合服务于给定的请求
func (r *BloomRequest) CanSend(peer *peer) bool {
	peer.lock.RLock()
	defer peer.lock.RUnlock()

	if peer.version < lpv2 {
		return false
	}
	return peer.headInfo.Number >= r.Config.BloomTrieConfirms && r.BloomTrieNum <= (peer.headInfo.Number-r.Config.BloomTrieConfirms)/r.Config.BloomTrieSize
}

//请求向LES网络发送一个ODR请求（LESODRREQUEST的实现）
func (r *BloomRequest) Request(reqID uint64, peer *peer) error {
	peer.Log().Debug("Requesting BloomBits", "bloomTrie", r.BloomTrieNum, "bitIdx", r.BitIdx, "sections", r.SectionIndexList)
	reqs := make([]HelperTrieReq, len(r.SectionIndexList))

	var encNumber [10]byte
	binary.BigEndian.PutUint16(encNumber[:2], uint16(r.BitIdx))

	for i, sectionIdx := range r.SectionIndexList {
		binary.BigEndian.PutUint64(encNumber[2:], sectionIdx)
		reqs[i] = HelperTrieReq{
			Type:    htBloomBits,
			TrieIdx: r.BloomTrieNum,
			Key:     common.CopyBytes(encNumber[:]),
		}
	}
	return peer.RequestHelperTrieProofs(reqID, r.GetCost(peer), reqs)
}

//有效处理来自LES网络的ODR请求回复消息
//如果消息是有效的答复，则返回true并将结果存储在内存中
//到请求（lesodrequest的实现）
func (r *BloomRequest) Validate(db ethdb.Database, msg *Msg) error {
	log.Debug("Validating BloomBits", "bloomTrie", r.BloomTrieNum, "bitIdx", r.BitIdx, "sections", r.SectionIndexList)

//确保我们有一个正确的消息和一个证明元素
	if msg.MsgType != MsgHelperTrieProofs {
		return errInvalidMessageType
	}
	resps := msg.Obj.(HelperTrieResps)
	proofs := resps.Proofs
	nodeSet := proofs.NodeSet()
	reads := &readTraceDB{db: nodeSet}

	r.BloomBits = make([][]byte, len(r.SectionIndexList))

//核实证据
	var encNumber [10]byte
	binary.BigEndian.PutUint16(encNumber[:2], uint16(r.BitIdx))

	for i, idx := range r.SectionIndexList {
		binary.BigEndian.PutUint64(encNumber[2:], idx)
		value, _, err := trie.VerifyProof(r.BloomTrieRoot, encNumber[:], reads)
		if err != nil {
			return err
		}
		r.BloomBits[i] = value
	}

	if len(reads.reads) != nodeSet.KeyCount() {
		return errUselessNodes
	}
	r.Proofs = nodeSet
	return nil
}

//readTraceDB stores the keys of database reads. We use this to check that received node
//集合只包含使证明通过所需的trie节点。
type readTraceDB struct {
	db    trie.DatabaseReader
	reads map[string]struct{}
}

//get返回存储节点
func (db *readTraceDB) Get(k []byte) ([]byte, error) {
	if db.reads == nil {
		db.reads = make(map[string]struct{})
	}
	db.reads[string(k)] = struct{}{}
	return db.db.Get(k)
}

//如果节点集包含给定的键，则返回true
func (db *readTraceDB) Has(key []byte) (bool, error) {
	_, err := db.Get(key)
	return err == nil, nil
}
