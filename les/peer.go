
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

//包les实现轻以太坊子协议。
package les

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/les/flowcontrol"
	"github.com/ethereum/go-ethereum/light"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	errClosed             = errors.New("peer set is closed")
	errAlreadyRegistered  = errors.New("peer is already registered")
	errNotRegistered      = errors.New("peer is not registered")
	errInvalidHelpTrieReq = errors.New("invalid help trie request")
)

const maxResponseErrors = 50 //number of invalid responses tolerated (makes the protocol less brittle but still avoids spam)

const (
	announceTypeNone = iota
	announceTypeSimple
	announceTypeSigned
)

type peer struct {
	*p2p.Peer

	rw p2p.MsgReadWriter

version int    //协议版本协商
network uint64 //正在打开网络ID

	announceType, requestAnnounceType uint64

	id string

	headInfo *announceData
	lock     sync.RWMutex

	announceChn chan announceData
	sendQueue   *execQueue

	poolEntry      *poolEntry
	hasBlock       func(common.Hash, uint64, bool) bool
	responseErrors int

fcClient       *flowcontrol.ClientNode //如果对等端仅为服务器，则为零
fcServer       *flowcontrol.ServerNode //如果对等端仅为客户端，则为零
	fcServerParams *flowcontrol.ServerParams
	fcCosts        requestCostTable
}

func newPeer(version int, network uint64, p *p2p.Peer, rw p2p.MsgReadWriter) *peer {
	id := p.ID()

	return &peer{
		Peer:        p,
		rw:          rw,
		version:     version,
		network:     network,
		id:          fmt.Sprintf("%x", id[:8]),
		announceChn: make(chan announceData, 20),
	}
}

func (p *peer) canQueue() bool {
	return p.sendQueue.canQueue()
}

func (p *peer) queueSend(f func()) {
	p.sendQueue.queue(f)
}

//INFO收集并返回有关对等机的已知元数据集合。
func (p *peer) Info() *eth.PeerInfo {
	return &eth.PeerInfo{
		Version:    p.version,
		Difficulty: p.Td(),
		Head:       fmt.Sprintf("%x", p.Head()),
	}
}

//Head retrieves a copy of the current head (most recent) hash of the peer.
func (p *peer) Head() (hash common.Hash) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	copy(hash[:], p.headInfo.Hash[:])
	return hash
}

func (p *peer) HeadAndTd() (hash common.Hash, td *big.Int) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	copy(hash[:], p.headInfo.Hash[:])
	return hash, p.headInfo.Td
}

func (p *peer) headBlockInfo() blockInfo {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return blockInfo{Hash: p.headInfo.Hash, Number: p.headInfo.Number, Td: p.headInfo.Td}
}

//td检索对等机的当前总难度。
func (p *peer) Td() *big.Int {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return new(big.Int).Set(p.headInfo.Td)
}

//waitbefore实现distpeer接口
func (p *peer) waitBefore(maxCost uint64) (time.Duration, float64) {
	return p.fcServer.CanSend(maxCost)
}

func sendRequest(w p2p.MsgWriter, msgcode, reqID, cost uint64, data interface{}) error {
	type req struct {
		ReqID uint64
		Data  interface{}
	}
	return p2p.Send(w, msgcode, req{reqID, data})
}

func sendResponse(w p2p.MsgWriter, msgcode, reqID, bv uint64, data interface{}) error {
	type resp struct {
		ReqID, BV uint64
		Data      interface{}
	}
	return p2p.Send(w, msgcode, resp{reqID, bv, data})
}

func (p *peer) GetRequestCost(msgcode uint64, amount int) uint64 {
	p.lock.RLock()
	defer p.lock.RUnlock()

	cost := p.fcCosts[msgcode].baseCost + p.fcCosts[msgcode].reqCost*uint64(amount)
	if cost > p.fcServerParams.BufLimit {
		cost = p.fcServerParams.BufLimit
	}
	return cost
}

//has block检查对等端是否具有给定的块
func (p *peer) HasBlock(hash common.Hash, number uint64, hasState bool) bool {
	p.lock.RLock()
	hasBlock := p.hasBlock
	p.lock.RUnlock()
	return hasBlock != nil && hasBlock(hash, number, hasState)
}

//sendAnnounce通过
//哈希通知。
func (p *peer) SendAnnounce(request announceData) error {
	return p2p.Send(p.rw, AnnounceMsg, request)
}

//sendblockheaders向远程对等端发送一批块头。
func (p *peer) SendBlockHeaders(reqID, bv uint64, headers []*types.Header) error {
	return sendResponse(p.rw, BlockHeadersMsg, reqID, bv, headers)
}

//sendblockbodiesrlp从发送一批块内容到远程对等机
//已经用rlp编码的格式。
func (p *peer) SendBlockBodiesRLP(reqID, bv uint64, bodies []rlp.RawValue) error {
	return sendResponse(p.rw, BlockBodiesMsg, reqID, bv, bodies)
}

//sendcoderlp发送一批任意内部数据，与
//已请求哈希。
func (p *peer) SendCode(reqID, bv uint64, data [][]byte) error {
	return sendResponse(p.rw, CodeMsg, reqID, bv, data)
}

//sendReceiptsRLP发送一批交易凭证，与
//从已经RLP编码的格式请求的。
func (p *peer) SendReceiptsRLP(reqID, bv uint64, receipts []rlp.RawValue) error {
	return sendResponse(p.rw, ReceiptsMsg, reqID, bv, receipts)
}

//sendproofs发送一批遗留的les/1 merkle proofs，与请求的对应。
func (p *peer) SendProofs(reqID, bv uint64, proofs proofsData) error {
	return sendResponse(p.rw, ProofsV1Msg, reqID, bv, proofs)
}

//SendProofsV2 sends a batch of merkle proofs, corresponding to the ones requested.
func (p *peer) SendProofsV2(reqID, bv uint64, proofs light.NodeList) error {
	return sendResponse(p.rw, ProofsV2Msg, reqID, bv, proofs)
}

//sendHeaderTops发送一批与请求的LES/1标题校对相对应的旧LES/1标题校对。
func (p *peer) SendHeaderProofs(reqID, bv uint64, proofs []ChtResp) error {
	return sendResponse(p.rw, HeaderProofsMsg, reqID, bv, proofs)
}

//sendHelperTrieProofs发送一批与请求的对应的HelperTrie Proofs。
func (p *peer) SendHelperTrieProofs(reqID, bv uint64, resp HelperTrieResps) error {
	return sendResponse(p.rw, HelperTrieProofsMsg, reqID, bv, resp)
}

//sendtxstatus发送一批与请求的事务状态记录相对应的事务状态记录。
func (p *peer) SendTxStatus(reqID, bv uint64, stats []txStatus) error {
	return sendResponse(p.rw, TxStatusMsg, reqID, bv, stats)
}

//RequestHeadersByHash获取与
//基于源块哈希的指定头查询。
func (p *peer) RequestHeadersByHash(reqID, cost uint64, origin common.Hash, amount int, skip int, reverse bool) error {
	p.Log().Debug("Fetching batch of headers", "count", amount, "fromhash", origin, "skip", skip, "reverse", reverse)
	return sendRequest(p.rw, GetBlockHeadersMsg, reqID, cost, &getBlockHeadersData{Origin: hashOrNumber{Hash: origin}, Amount: uint64(amount), Skip: uint64(skip), Reverse: reverse})
}

//RequestHeadersByNumber获取与
//指定的头查询，基于源块的编号。
func (p *peer) RequestHeadersByNumber(reqID, cost, origin uint64, amount int, skip int, reverse bool) error {
	p.Log().Debug("Fetching batch of headers", "count", amount, "fromnum", origin, "skip", skip, "reverse", reverse)
	return sendRequest(p.rw, GetBlockHeadersMsg, reqID, cost, &getBlockHeadersData{Origin: hashOrNumber{Number: origin}, Amount: uint64(amount), Skip: uint64(skip), Reverse: reverse})
}

//请求主体获取一批与哈希对应的块的主体
//明确规定。
func (p *peer) RequestBodies(reqID, cost uint64, hashes []common.Hash) error {
	p.Log().Debug("Fetching batch of block bodies", "count", len(hashes))
	return sendRequest(p.rw, GetBlockBodiesMsg, reqID, cost, hashes)
}

//请求代码从节点的已知状态获取一批任意数据
//与指定哈希对应的数据。
func (p *peer) RequestCode(reqID, cost uint64, reqs []CodeReq) error {
	p.Log().Debug("Fetching batch of codes", "count", len(reqs))
	return sendRequest(p.rw, GetCodeMsg, reqID, cost, reqs)
}

//RequestReceipts从远程节点获取一批事务收据。
func (p *peer) RequestReceipts(reqID, cost uint64, hashes []common.Hash) error {
	p.Log().Debug("Fetching batch of receipts", "count", len(hashes))
	return sendRequest(p.rw, GetReceiptsMsg, reqID, cost, hashes)
}

//RequestProof从远程节点获取一批Merkle Proof。
func (p *peer) RequestProofs(reqID, cost uint64, reqs []ProofReq) error {
	p.Log().Debug("Fetching batch of proofs", "count", len(reqs))
	switch p.version {
	case lpv1:
		return sendRequest(p.rw, GetProofsV1Msg, reqID, cost, reqs)
	case lpv2:
		return sendRequest(p.rw, GetProofsV2Msg, reqID, cost, reqs)
	default:
		panic(nil)
	}
}

//RequestHelperTreeProofs从远程节点获取一批HelperTreeMerkle Proofs。
func (p *peer) RequestHelperTrieProofs(reqID, cost uint64, data interface{}) error {
	switch p.version {
	case lpv1:
		reqs, ok := data.([]ChtReq)
		if !ok {
			return errInvalidHelpTrieReq
		}
		p.Log().Debug("Fetching batch of header proofs", "count", len(reqs))
		return sendRequest(p.rw, GetHeaderProofsMsg, reqID, cost, reqs)
	case lpv2:
		reqs, ok := data.([]HelperTrieReq)
		if !ok {
			return errInvalidHelpTrieReq
		}
		p.Log().Debug("Fetching batch of HelperTrie proofs", "count", len(reqs))
		return sendRequest(p.rw, GetHelperTrieProofsMsg, reqID, cost, reqs)
	default:
		panic(nil)
	}
}

//RequestTxStatus从远程节点获取一批事务状态记录。
func (p *peer) RequestTxStatus(reqID, cost uint64, txHashes []common.Hash) error {
	p.Log().Debug("Requesting transaction status", "count", len(txHashes))
	return sendRequest(p.rw, GetTxStatusMsg, reqID, cost, txHashes)
}

//SendTxStatus sends a batch of transactions to be added to the remote transaction pool.
func (p *peer) SendTxs(reqID, cost uint64, txs types.Transactions) error {
	p.Log().Debug("Fetching batch of transactions", "count", len(txs))
	switch p.version {
	case lpv1:
return p2p.Send(p.rw, SendTxMsg, txs) //旧消息格式不包括reqid
	case lpv2:
		return sendRequest(p.rw, SendTxV2Msg, reqID, cost, txs)
	default:
		panic(nil)
	}
}

type keyValueEntry struct {
	Key   string
	Value rlp.RawValue
}
type keyValueList []keyValueEntry
type keyValueMap map[string]rlp.RawValue

func (l keyValueList) add(key string, val interface{}) keyValueList {
	var entry keyValueEntry
	entry.Key = key
	if val == nil {
		val = uint64(0)
	}
	enc, err := rlp.EncodeToBytes(val)
	if err == nil {
		entry.Value = enc
	}
	return append(l, entry)
}

func (l keyValueList) decode() keyValueMap {
	m := make(keyValueMap)
	for _, entry := range l {
		m[entry.Key] = entry.Value
	}
	return m
}

func (m keyValueMap) get(key string, val interface{}) error {
	enc, ok := m[key]
	if !ok {
		return errResp(ErrMissingKey, "%s", key)
	}
	if val == nil {
		return nil
	}
	return rlp.DecodeBytes(enc, val)
}

func (p *peer) sendReceiveHandshake(sendList keyValueList) (keyValueList, error) {
//在新线程中发送自己的握手
	errc := make(chan error, 1)
	go func() {
		errc <- p2p.Send(p.rw, StatusMsg, sendList)
	}()
//同时检索远程状态消息
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return nil, err
	}
	if msg.Code != StatusMsg {
		return nil, errResp(ErrNoStatusMsg, "first msg has code %x (!= %x)", msg.Code, StatusMsg)
	}
	if msg.Size > ProtocolMaxMsgSize {
		return nil, errResp(ErrMsgTooLarge, "%v > %v", msg.Size, ProtocolMaxMsgSize)
	}
//解码握手
	var recvList keyValueList
	if err := msg.Decode(&recvList); err != nil {
		return nil, errResp(ErrDecode, "msg %v: %v", msg, err)
	}
	if err := <-errc; err != nil {
		return nil, err
	}
	return recvList, nil
}

//握手执行LES协议握手，协商版本号，
//网络ID，困难，头和创世块。
func (p *peer) Handshake(td *big.Int, head common.Hash, headNum uint64, genesis common.Hash, server *LesServer) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	var send keyValueList
	send = send.add("protocolVersion", uint64(p.version))
	send = send.add("networkId", p.network)
	send = send.add("headTd", td)
	send = send.add("headHash", head)
	send = send.add("headNum", headNum)
	send = send.add("genesisHash", genesis)
	if server != nil {
		send = send.add("serveHeaders", nil)
		send = send.add("serveChainSince", uint64(0))
		send = send.add("serveStateSince", uint64(0))
		send = send.add("txRelay", nil)
		send = send.add("flowControl/BL", server.defParams.BufLimit)
		send = send.add("flowControl/MRR", server.defParams.MinRecharge)
		list := server.fcCostStats.getCurrentList()
		send = send.add("flowControl/MRC", list)
		p.fcCosts = list.decode()
	} else {
p.requestAnnounceType = announceTypeSimple //设置为默认值，直到实现“非常轻”的客户机模式
		send = send.add("announceType", p.requestAnnounceType)
	}
	recvList, err := p.sendReceiveHandshake(send)
	if err != nil {
		return err
	}
	recv := recvList.decode()

	var rGenesis, rHash common.Hash
	var rVersion, rNetwork, rNum uint64
	var rTd *big.Int

	if err := recv.get("protocolVersion", &rVersion); err != nil {
		return err
	}
	if err := recv.get("networkId", &rNetwork); err != nil {
		return err
	}
	if err := recv.get("headTd", &rTd); err != nil {
		return err
	}
	if err := recv.get("headHash", &rHash); err != nil {
		return err
	}
	if err := recv.get("headNum", &rNum); err != nil {
		return err
	}
	if err := recv.get("genesisHash", &rGenesis); err != nil {
		return err
	}

	if rGenesis != genesis {
		return errResp(ErrGenesisBlockMismatch, "%x (!= %x)", rGenesis[:8], genesis[:8])
	}
	if rNetwork != p.network {
		return errResp(ErrNetworkIdMismatch, "%d (!= %d)", rNetwork, p.network)
	}
	if int(rVersion) != p.version {
		return errResp(ErrProtocolVersionMismatch, "%d (!= %d)", rVersion, p.version)
	}
	if server != nil {
//在我们拥有适当的对等连接API之前，允许LES连接到其他服务器
  /*f recv.get（“servstatesince”，nil）==nil
   返回errrresp（erruselesspeer，“需要的客户机，得到的服务器”）
  */

		if recv.get("announceType", &p.announceType) != nil {
			p.announceType = announceTypeSimple
		}
		p.fcClient = flowcontrol.NewClientNode(server.fcManager, server.defParams)
	} else {
		if recv.get("serveChainSince", nil) != nil {
			return errResp(ErrUselessPeer, "peer cannot serve chain")
		}
		if recv.get("serveStateSince", nil) != nil {
			return errResp(ErrUselessPeer, "peer cannot serve state")
		}
		if recv.get("txRelay", nil) != nil {
			return errResp(ErrUselessPeer, "peer cannot relay transactions")
		}
		params := &flowcontrol.ServerParams{}
		if err := recv.get("flowControl/BL", &params.BufLimit); err != nil {
			return err
		}
		if err := recv.get("flowControl/MRR", &params.MinRecharge); err != nil {
			return err
		}
		var MRC RequestCostList
		if err := recv.get("flowControl/MRC", &MRC); err != nil {
			return err
		}
		p.fcServerParams = params
		p.fcServer = flowcontrol.NewServerNode(params)
		p.fcCosts = MRC.decode()
	}

	p.headInfo = &announceData{Td: rTd, Hash: rHash, Number: rNum}
	return nil
}

//字符串实现fmt.stringer。
func (p *peer) String() string {
	return fmt.Sprintf("Peer %s [%s]", p.id,
		fmt.Sprintf("les/%d", p.version),
	)
}

//peersetnotify是一个回调接口，用于通知服务有关添加或
//移除的对等体
type peerSetNotify interface {
	registerPeer(*peer)
	unregisterPeer(*peer)
}

//Peerset表示当前参与的活动对等机的集合
//光以太坊子协议。
type peerSet struct {
	peers      map[string]*peer
	lock       sync.RWMutex
	notifyList []peerSetNotify
	closed     bool
}

//new peer set创建一个新的对等集来跟踪活动参与者。
func newPeerSet() *peerSet {
	return &peerSet{
		peers: make(map[string]*peer),
	}
}

//notify添加要通知的有关添加或删除对等点的服务
func (ps *peerSet) notify(n peerSetNotify) {
	ps.lock.Lock()
	ps.notifyList = append(ps.notifyList, n)
	peers := make([]*peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		peers = append(peers, p)
	}
	ps.lock.Unlock()

	for _, p := range peers {
		n.registerPeer(p)
	}
}

//寄存器向工作集中注入一个新的对等点，或者返回一个错误，如果
//对等机已经知道。
func (ps *peerSet) Register(p *peer) error {
	ps.lock.Lock()
	if ps.closed {
		ps.lock.Unlock()
		return errClosed
	}
	if _, ok := ps.peers[p.id]; ok {
		ps.lock.Unlock()
		return errAlreadyRegistered
	}
	ps.peers[p.id] = p
	p.sendQueue = newExecQueue(100)
	peers := make([]peerSetNotify, len(ps.notifyList))
	copy(peers, ps.notifyList)
	ps.lock.Unlock()

	for _, n := range peers {
		n.registerPeer(p)
	}
	return nil
}

//注销从活动集删除远程对等，进一步禁用
//对该特定实体采取的行动。它还会在网络层启动断开连接。
func (ps *peerSet) Unregister(id string) error {
	ps.lock.Lock()
	if p, ok := ps.peers[id]; !ok {
		ps.lock.Unlock()
		return errNotRegistered
	} else {
		delete(ps.peers, id)
		peers := make([]peerSetNotify, len(ps.notifyList))
		copy(peers, ps.notifyList)
		ps.lock.Unlock()

		for _, n := range peers {
			n.unregisterPeer(p)
		}
		p.sendQueue.quit()
		p.Peer.Disconnect(p2p.DiscUselessPeer)
		return nil
	}
}

//AllPeerIDs returns a list of all registered peer IDs
func (ps *peerSet) AllPeerIDs() []string {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	res := make([]string, len(ps.peers))
	idx := 0
	for id := range ps.peers {
		res[idx] = id
		idx++
	}
	return res
}

//对等端检索具有给定ID的注册对等端。
func (ps *peerSet) Peer(id string) *peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.peers[id]
}

//len返回集合中当前的对等数。
func (ps *peerSet) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return len(ps.peers)
}

//BestPeer以当前最高的总难度检索已知的对等。
func (ps *peerSet) BestPeer() *peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	var (
		bestPeer *peer
		bestTd   *big.Int
	)
	for _, p := range ps.peers {
		if td := p.Td(); bestPeer == nil || td.Cmp(bestTd) > 0 {
			bestPeer, bestTd = p, td
		}
	}
	return bestPeer
}

//all peers返回列表中的所有对等方
func (ps *peerSet) AllPeers() []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*peer, len(ps.peers))
	i := 0
	for _, peer := range ps.peers {
		list[i] = peer
		i++
	}
	return list
}

//关闭将断开所有对等机的连接。
//关闭后不能注册新的对等方。
func (ps *peerSet) Close() {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	for _, p := range ps.peers {
		p.Disconnect(p2p.DiscQuitting)
	}
	ps.closed = true
}
