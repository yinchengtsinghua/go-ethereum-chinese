
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
	"encoding/json"
	"hash"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/feed/lookup"
)

//请求表示对订阅源更新消息进行签名或签名的请求
type Request struct {
Update     //将放在块上的实际内容，较少签名
	Signature  *Signature
idAddr     storage.Address //更新的缓存块地址（未序列化，供内部使用）
binaryData []byte          //缓存的序列化数据（不再序列化！，用于效率/内部使用）
}

//updateRequestJSON表示JSON序列化的updateRequest
type updateRequestJSON struct {
	ID
	ProtocolVersion uint8  `json:"protocolVersion"`
	Data            string `json:"data,omitempty"`
	Signature       string `json:"signature,omitempty"`
}

//请求布局
//更新字节
//签名长度字节
const minimumSignedUpdateLength = minimumUpdateDataLength + signatureLength

//newfirstrequest返回准备好签名的请求以发布第一个源更新
func NewFirstRequest(topic Topic) *Request {

	request := new(Request)

//获取当前时间
	now := TimestampProvider.Now().Time
	request.Epoch = lookup.GetFirstEpoch(now)
	request.Feed.Topic = topic
	request.Header.Version = ProtocolVersion

	return request
}

//setdata存储提要更新将使用的有效负载数据
func (r *Request) SetData(data []byte) {
	r.data = data
	r.Signature = nil
}

//如果此请求为签名更新建模，或者为签名请求，则is update返回true
func (r *Request) IsUpdate() bool {
	return r.Signature != nil
}

//验证签名是否有效
func (r *Request) Verify() (err error) {
	if len(r.data) == 0 {
		return NewError(ErrInvalidValue, "Update does not contain data")
	}
	if r.Signature == nil {
		return NewError(ErrInvalidSignature, "Missing signature field")
	}

	digest, err := r.GetDigest()
	if err != nil {
		return err
	}

//获取签名者的地址（它还检查签名是否有效）
	r.Feed.User, err = getUserAddr(digest, *r.Signature)
	if err != nil {
		return err
	}

//检查块中包含的查找信息是否与updateaddr（块搜索键）匹配。
//用于检索此块的
//如果验证失败，则有人伪造了块。
	if !bytes.Equal(r.idAddr, r.Addr()) {
		return NewError(ErrInvalidSignature, "Signature address does not match with update user address")
	}

	return nil
}

//签名执行签名以验证更新消息
func (r *Request) Sign(signer Signer) error {
	r.Feed.User = signer.Address()
r.binaryData = nil           //使序列化数据无效
digest, err := r.GetDigest() //计算摘要并序列化为.BinaryData
	if err != nil {
		return err
	}

	signature, err := signer.Sign(digest)
	if err != nil {
		return err
	}

//尽管签名者接口返回签名者的公共地址，
//从签名中恢复它以查看它们是否匹配
	userAddr, err := getUserAddr(digest, signature)
	if err != nil {
		return NewError(ErrInvalidSignature, "Error verifying signature")
	}

if userAddr != signer.Address() { //健全性检查以确保签名者声明的地址与用于签名的地址相同！
		return NewError(ErrInvalidSignature, "Signer address does not match update user address")
	}

	r.Signature = &signature
	r.idAddr = r.Addr()
	return nil
}

//GetDigest创建用于签名的源更新摘要
//序列化的负载缓存在.BinaryData中
func (r *Request) GetDigest() (result common.Hash, err error) {
	hasher := hashPool.Get().(hash.Hash)
	defer hashPool.Put(hasher)
	hasher.Reset()
	dataLength := r.Update.binaryLength()
	if r.binaryData == nil {
		r.binaryData = make([]byte, dataLength+signatureLength)
		if err := r.Update.binaryPut(r.binaryData[:dataLength]); err != nil {
			return result, err
		}
	}
hasher.Write(r.binaryData[:dataLength]) //除了签名以外的一切。

	return common.BytesToHash(hasher.Sum(nil)), nil
}

//创建更新块。
func (r *Request) toChunk() (storage.Chunk, error) {

//检查更新是否已签名和序列化
//为了提高效率，数据在签名期间序列化并缓存在
//在.getDigest（）中计算签名摘要时的BinaryData字段
	if r.Signature == nil || r.binaryData == nil {
		return nil, NewError(ErrInvalidSignature, "toChunk called without a valid signature or payload data. Call .Sign() first.")
	}

	updateLength := r.Update.binaryLength()

//签名是块数据中的最后一项
	copy(r.binaryData[updateLength:], r.Signature[:])

	chunk := storage.NewChunk(r.idAddr, r.binaryData)
	return chunk, nil
}

//FromChunk从块数据填充此结构。它不验证签名是否有效。
func (r *Request) fromChunk(chunk storage.Chunk) error {
//有关更新块布局，请参见请求定义

	chunkdata := chunk.Data()

//反序列化源更新部分
	if err := r.Update.binaryGet(chunkdata[:len(chunkdata)-signatureLength]); err != nil {
		return err
	}

//提取签名
	var signature *Signature
	cursor := r.Update.binaryLength()
	sigdata := chunkdata[cursor : cursor+signatureLength]
	if len(sigdata) > 0 {
		signature = &Signature{}
		copy(signature[:], sigdata)
	}

	r.Signature = signature
	r.idAddr = chunk.Address()
	r.binaryData = chunkdata

	return nil

}

//FromValues从字符串键值存储中反序列化此实例
//用于分析查询字符串
func (r *Request) FromValues(values Values, data []byte) error {
	signatureBytes, err := hexutil.Decode(values.Get("signature"))
	if err != nil {
		r.Signature = nil
	} else {
		if len(signatureBytes) != signatureLength {
			return NewError(ErrInvalidSignature, "Incorrect signature length")
		}
		r.Signature = new(Signature)
		copy(r.Signature[:], signatureBytes)
	}
	err = r.Update.FromValues(values, data)
	if err != nil {
		return err
	}
	r.idAddr = r.Addr()
	return err
}

//AppendValues将此结构序列化到提供的字符串键值存储区中
//用于生成查询字符串
func (r *Request) AppendValues(values Values) []byte {
	if r.Signature != nil {
		values.Set("signature", hexutil.Encode(r.Signature[:]))
	}
	return r.Update.AppendValues(values)
}

//fromjson接受更新请求json并填充更新请求
func (r *Request) fromJSON(j *updateRequestJSON) error {

	r.ID = j.ID
	r.Header.Version = j.ProtocolVersion

	var err error
	if j.Data != "" {
		r.data, err = hexutil.Decode(j.Data)
		if err != nil {
			return NewError(ErrInvalidValue, "Cannot decode data")
		}
	}

	if j.Signature != "" {
		sigBytes, err := hexutil.Decode(j.Signature)
		if err != nil || len(sigBytes) != signatureLength {
			return NewError(ErrInvalidSignature, "Cannot decode signature")
		}
		r.Signature = new(Signature)
		r.idAddr = r.Addr()
		copy(r.Signature[:], sigBytes)
	}
	return nil
}

//unmarshaljson接受存储在字节数组中的json结构并填充请求对象
//实现json.unmasheler接口
func (r *Request) UnmarshalJSON(rawData []byte) error {
	var requestJSON updateRequestJSON
	if err := json.Unmarshal(rawData, &requestJSON); err != nil {
		return err
	}
	return r.fromJSON(&requestJSON)
}

//marshaljson接受更新请求并将其作为json结构编码到字节数组中
//实现json.Marshaler接口
func (r *Request) MarshalJSON() (rawData []byte, err error) {
	var signatureString, dataString string
	if r.Signature != nil {
		signatureString = hexutil.Encode(r.Signature[:])
	}
	if r.data != nil {
		dataString = hexutil.Encode(r.data)
	}

	requestJSON := &updateRequestJSON{
		ID:              r.ID,
		ProtocolVersion: r.Header.Version,
		Data:            dataString,
		Signature:       signatureString,
	}

	return json.Marshal(requestJSON)
}
