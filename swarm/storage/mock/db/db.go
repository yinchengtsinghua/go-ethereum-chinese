
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

//包DB实现了一个模拟存储，它将所有块数据保存在LevelDB数据库中。
package db

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/storage/mock"
)

//GlobalStore包含正在存储的LevelDB数据库
//所有群节点的块数据。
//使用关闭方法关闭GlobalStore需要
//释放数据库使用的资源。
type GlobalStore struct {
	db *leveldb.DB
}

//NewGlobalStore创建了一个新的GlobalStore实例。
func NewGlobalStore(path string) (s *GlobalStore, err error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	return &GlobalStore{
		db: db,
	}, nil
}

//close释放基础级别db使用的资源。
func (s *GlobalStore) Close() error {
	return s.db.Close()
}

//new nodestore返回一个新的nodestore实例，用于检索和存储
//仅对地址为的节点进行数据块处理。
func (s *GlobalStore) NewNodeStore(addr common.Address) *mock.NodeStore {
	return mock.NewNodeStore(addr, s)
}

//如果节点存在键为的块，则get返回块数据
//地址地址。
func (s *GlobalStore) Get(addr common.Address, key []byte) (data []byte, err error) {
	has, err := s.db.Has(nodeDBKey(addr, key), nil)
	if err != nil {
		return nil, mock.ErrNotFound
	}
	if !has {
		return nil, mock.ErrNotFound
	}
	data, err = s.db.Get(dataDBKey(key), nil)
	if err == leveldb.ErrNotFound {
		err = mock.ErrNotFound
	}
	return
}

//Put保存带有地址addr的节点的块数据。
func (s *GlobalStore) Put(addr common.Address, key []byte, data []byte) error {
	batch := new(leveldb.Batch)
	batch.Put(nodeDBKey(addr, key), nil)
	batch.Put(dataDBKey(key), data)
	return s.db.Write(batch, nil)
}

//删除删除对地址为addr的节点的块引用。
func (s *GlobalStore) Delete(addr common.Address, key []byte) error {
	batch := new(leveldb.Batch)
	batch.Delete(nodeDBKey(addr, key))
	return s.db.Write(batch, nil)
}

//haskey返回带有addr的节点是否包含键。
func (s *GlobalStore) HasKey(addr common.Address, key []byte) bool {
	has, err := s.db.Has(nodeDBKey(addr, key), nil)
	if err != nil {
		has = false
	}
	return has
}

//import从包含导出块数据的读卡器读取tar存档。
//它返回导入的块的数量和错误。
func (s *GlobalStore) Import(r io.Reader) (n int, err error) {
	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return n, err
		}

		data, err := ioutil.ReadAll(tr)
		if err != nil {
			return n, err
		}

		var c mock.ExportedChunk
		if err = json.Unmarshal(data, &c); err != nil {
			return n, err
		}

		batch := new(leveldb.Batch)
		for _, addr := range c.Addrs {
			batch.Put(nodeDBKeyHex(addr, hdr.Name), nil)
		}

		batch.Put(dataDBKey(common.Hex2Bytes(hdr.Name)), c.Data)
		if err = s.db.Write(batch, nil); err != nil {
			return n, err
		}

		n++
	}
	return n, err
}

//将包含所有块数据的tar存档导出到写入程序
//商店。它返回导出的块的数量和错误。
func (s *GlobalStore) Export(w io.Writer) (n int, err error) {
	tw := tar.NewWriter(w)
	defer tw.Close()

	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	encoder := json.NewEncoder(buf)

	iter := s.db.NewIterator(util.BytesPrefix(nodeKeyPrefix), nil)
	defer iter.Release()

	var currentKey string
	var addrs []common.Address

	saveChunk := func(hexKey string) error {
		key := common.Hex2Bytes(hexKey)

		data, err := s.db.Get(dataDBKey(key), nil)
		if err != nil {
			return err
		}

		buf.Reset()
		if err = encoder.Encode(mock.ExportedChunk{
			Addrs: addrs,
			Data:  data,
		}); err != nil {
			return err
		}

		d := buf.Bytes()
		hdr := &tar.Header{
			Name: hexKey,
			Mode: 0644,
			Size: int64(len(d)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write(d); err != nil {
			return err
		}
		n++
		return nil
	}

	for iter.Next() {
		k := bytes.TrimPrefix(iter.Key(), nodeKeyPrefix)
		i := bytes.Index(k, []byte("-"))
		if i < 0 {
			continue
		}
		hexKey := string(k[:i])

		if currentKey == "" {
			currentKey = hexKey
		}

		if hexKey != currentKey {
			if err = saveChunk(currentKey); err != nil {
				return n, err
			}

			addrs = addrs[:0]
		}

		currentKey = hexKey
		addrs = append(addrs, common.BytesToAddress(k[i:]))
	}

	if len(addrs) > 0 {
		if err = saveChunk(currentKey); err != nil {
			return n, err
		}
	}

	return n, err
}

var (
	nodeKeyPrefix = []byte("node-")
	dataKeyPrefix = []byte("data-")
)

//nodedbkey为键/节点映射构造数据库键。
func nodeDBKey(addr common.Address, key []byte) []byte {
	return nodeDBKeyHex(addr, common.Bytes2Hex(key))
}

//nodedbkeyhex为键/节点映射构造数据库键
//使用键的十六进制字符串表示形式。
func nodeDBKeyHex(addr common.Address, hexKey string) []byte {
	return append(append(nodeKeyPrefix, []byte(hexKey+"-")...), addr[:]...)
}

//datadbkey为键/数据存储构造数据库键。
func dataDBKey(key []byte) []byte {
	return append(dataKeyPrefix, key...)
}
