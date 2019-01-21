
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

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/swarm/storage/feed"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

const (
	ManifestType    = "application/bzz-manifest+json"
	FeedContentType = "application/bzz-feed"

	manifestSizeLimit = 5 * 1024 * 1024
)

//清单代表一个群体清单
type Manifest struct {
	Entries []ManifestEntry `json:"entries,omitempty"`
}

//manifest entry表示群清单中的条目
type ManifestEntry struct {
	Hash        string       `json:"hash,omitempty"`
	Path        string       `json:"path,omitempty"`
	ContentType string       `json:"contentType,omitempty"`
	Mode        int64        `json:"mode,omitempty"`
	Size        int64        `json:"size,omitempty"`
	ModTime     time.Time    `json:"mod_time,omitempty"`
	Status      int          `json:"status,omitempty"`
	Access      *AccessEntry `json:"access,omitempty"`
	Feed        *feed.Feed   `json:"feed,omitempty"`
}

//manifestList表示清单中列出文件的结果
type ManifestList struct {
	CommonPrefixes []string         `json:"common_prefixes,omitempty"`
	Entries        []*ManifestEntry `json:"entries,omitempty"`
}

//new manifest创建并存储新的空清单
func (a *API) NewManifest(ctx context.Context, toEncrypt bool) (storage.Address, error) {
	var manifest Manifest
	data, err := json.Marshal(&manifest)
	if err != nil {
		return nil, err
	}
	addr, wait, err := a.Store(ctx, bytes.NewReader(data), int64(len(data)), toEncrypt)
	if err != nil {
		return nil, err
	}
	err = wait(ctx)
	return addr, err
}

//支持来自BZZ的群源的清单黑客：方案
//有关更多信息，请参阅swarm/api/api.go:api.get（）。
func (a *API) NewFeedManifest(ctx context.Context, feed *feed.Feed) (storage.Address, error) {
	var manifest Manifest
	entry := ManifestEntry{
		Feed:        feed,
		ContentType: FeedContentType,
	}
	manifest.Entries = append(manifest.Entries, entry)
	data, err := json.Marshal(&manifest)
	if err != nil {
		return nil, err
	}
	addr, wait, err := a.Store(ctx, bytes.NewReader(data), int64(len(data)), false)
	if err != nil {
		return nil, err
	}
	err = wait(ctx)
	return addr, err
}

//manifestwriter用于添加和删除基础清单中的条目
type ManifestWriter struct {
	api   *API
	trie  *manifestTrie
	quitC chan bool
}

func (a *API) NewManifestWriter(ctx context.Context, addr storage.Address, quitC chan bool) (*ManifestWriter, error) {
	trie, err := loadManifest(ctx, a.fileStore, addr, quitC, NOOPDecrypt)
	if err != nil {
		return nil, fmt.Errorf("error loading manifest %s: %s", addr, err)
	}
	return &ManifestWriter{a, trie, quitC}, nil
}

//附录存储给定的数据并将结果地址添加到清单中
func (m *ManifestWriter) AddEntry(ctx context.Context, data io.Reader, e *ManifestEntry) (addr storage.Address, err error) {
	entry := newManifestTrieEntry(e, nil)
	if data != nil {
		var wait func(context.Context) error
		addr, wait, err = m.api.Store(ctx, data, e.Size, m.trie.encrypted)
		if err != nil {
			return nil, err
		}
		err = wait(ctx)
		if err != nil {
			return nil, err
		}
		entry.Hash = addr.Hex()
	}
	if entry.Hash == "" {
		return addr, errors.New("missing entry hash")
	}
	m.trie.addEntry(entry, m.quitC)
	return addr, nil
}

//removeentry从清单中删除给定路径
func (m *ManifestWriter) RemoveEntry(path string) error {
	m.trie.deleteEntry(path, m.quitC)
	return nil
}

//存储存储清单，返回结果存储地址
func (m *ManifestWriter) Store() (storage.Address, error) {
	return m.trie.ref, m.trie.recalcAndStore()
}

//manifestwalker用于递归地遍历清单中的条目和
//它的所有子流形
type ManifestWalker struct {
	api   *API
	trie  *manifestTrie
	quitC chan bool
}

func (a *API) NewManifestWalker(ctx context.Context, addr storage.Address, decrypt DecryptFunc, quitC chan bool) (*ManifestWalker, error) {
	trie, err := loadManifest(ctx, a.fileStore, addr, quitC, decrypt)
	if err != nil {
		return nil, fmt.Errorf("error loading manifest %s: %s", addr, err)
	}
	return &ManifestWalker{a, trie, quitC}, nil
}

//errskipmanifest用作walkfn的返回值，以指示
//应跳过清单
var ErrSkipManifest = errors.New("skip this manifest")

//walkfn是递归访问的每个条目调用的函数类型
//显性步行
type WalkFn func(entry *ManifestEntry) error

//以递归方式遍历清单，为中的每个条目调用walkfn
//清单，包括子流形
func (m *ManifestWalker) Walk(walkFn WalkFn) error {
	return m.walk(m.trie, "", walkFn)
}

func (m *ManifestWalker) walk(trie *manifestTrie, prefix string, walkFn WalkFn) error {
	for _, entry := range &trie.entries {
		if entry == nil {
			continue
		}
		entry.Path = prefix + entry.Path
		err := walkFn(&entry.ManifestEntry)
		if err != nil {
			if entry.ContentType == ManifestType && err == ErrSkipManifest {
				continue
			}
			return err
		}
		if entry.ContentType != ManifestType {
			continue
		}
		if err := trie.loadSubTrie(entry, nil); err != nil {
			return err
		}
		if err := m.walk(entry.subtrie, entry.Path, walkFn); err != nil {
			return err
		}
	}
	return nil
}

type manifestTrie struct {
	fileStore *storage.FileStore
entries   [257]*manifestTrieEntry //按basepath的第一个字符索引，条目[256]是空的basepath条目。
ref       storage.Address         //如果裁判！=nil，存储
	encrypted bool
	decrypt   DecryptFunc
}

func newManifestTrieEntry(entry *ManifestEntry, subtrie *manifestTrie) *manifestTrieEntry {
	return &manifestTrieEntry{
		ManifestEntry: *entry,
		subtrie:       subtrie,
	}
}

type manifestTrieEntry struct {
	ManifestEntry

	subtrie *manifestTrie
}

func loadManifest(ctx context.Context, fileStore *storage.FileStore, addr storage.Address, quitC chan bool, decrypt DecryptFunc) (trie *manifestTrie, err error) { //非递归子树按需下载
	log.Trace("manifest lookup", "addr", addr)
//通过文件存储检索清单
	manifestReader, isEncrypted := fileStore.Retrieve(ctx, addr)
	log.Trace("reader retrieved", "addr", addr)
	return readManifest(manifestReader, addr, fileStore, isEncrypted, quitC, decrypt)
}

func readManifest(mr storage.LazySectionReader, addr storage.Address, fileStore *storage.FileStore, isEncrypted bool, quitC chan bool, decrypt DecryptFunc) (trie *manifestTrie, err error) { //非递归子树按需下载

//TODO检查超大清单的大小
	size, err := mr.Size(mr.Context(), quitC)
if err != nil { //大小＝0
//无法确定大小意味着没有根块
		log.Trace("manifest not found", "addr", addr)
		err = fmt.Errorf("Manifest not Found")
		return
	}
	if size > manifestSizeLimit {
		log.Warn("manifest exceeds size limit", "addr", addr, "size", size, "limit", manifestSizeLimit)
		err = fmt.Errorf("Manifest size of %v bytes exceeds the %v byte limit", size, manifestSizeLimit)
		return
	}
	manifestData := make([]byte, size)
	read, err := mr.Read(manifestData)
	if int64(read) < size {
		log.Trace("manifest not found", "addr", addr)
		if err == nil {
			err = fmt.Errorf("Manifest retrieval cut short: read %v, expect %v", read, size)
		}
		return
	}

	log.Debug("manifest retrieved", "addr", addr)
	var man struct {
		Entries []*manifestTrieEntry `json:"entries"`
	}
	err = json.Unmarshal(manifestData, &man)
	if err != nil {
		err = fmt.Errorf("Manifest %v is malformed: %v", addr.Log(), err)
		log.Trace("malformed manifest", "addr", addr)
		return
	}

	log.Trace("manifest entries", "addr", addr, "len", len(man.Entries))

	trie = &manifestTrie{
		fileStore: fileStore,
		encrypted: isEncrypted,
		decrypt:   decrypt,
	}
	for _, entry := range man.Entries {
		err = trie.addEntry(entry, quitC)
		if err != nil {
			return
		}
	}
	return
}

func (mt *manifestTrie) addEntry(entry *manifestTrieEntry, quitC chan bool) error {
mt.ref = nil //trie已修改，需要根据需要重新计算哈希

	if entry.ManifestEntry.Access != nil {
		if mt.decrypt == nil {
			return errors.New("dont have decryptor")
		}

		err := mt.decrypt(&entry.ManifestEntry)
		if err != nil {
			return err
		}
	}

	if len(entry.Path) == 0 {
		mt.entries[256] = entry
		return nil
	}

	b := entry.Path[0]
	oldentry := mt.entries[b]
	if (oldentry == nil) || (oldentry.Path == entry.Path && oldentry.ContentType != ManifestType) {
		mt.entries[b] = entry
		return nil
	}

	cpl := 0
	for (len(entry.Path) > cpl) && (len(oldentry.Path) > cpl) && (entry.Path[cpl] == oldentry.Path[cpl]) {
		cpl++
	}

	if (oldentry.ContentType == ManifestType) && (cpl == len(oldentry.Path)) {
		if mt.loadSubTrie(oldentry, quitC) != nil {
			return nil
		}
		entry.Path = entry.Path[cpl:]
		oldentry.subtrie.addEntry(entry, quitC)
		oldentry.Hash = ""
		return nil
	}

	commonPrefix := entry.Path[:cpl]

	subtrie := &manifestTrie{
		fileStore: mt.fileStore,
		encrypted: mt.encrypted,
	}
	entry.Path = entry.Path[cpl:]
	oldentry.Path = oldentry.Path[cpl:]
	subtrie.addEntry(entry, quitC)
	subtrie.addEntry(oldentry, quitC)

	mt.entries[b] = newManifestTrieEntry(&ManifestEntry{
		Path:        commonPrefix,
		ContentType: ManifestType,
	}, subtrie)
	return nil
}

func (mt *manifestTrie) getCountLast() (cnt int, entry *manifestTrieEntry) {
	for _, e := range &mt.entries {
		if e != nil {
			cnt++
			entry = e
		}
	}
	return
}

func (mt *manifestTrie) deleteEntry(path string, quitC chan bool) {
mt.ref = nil //trie已修改，需要根据需要重新计算哈希

	if len(path) == 0 {
		mt.entries[256] = nil
		return
	}

	b := path[0]
	entry := mt.entries[b]
	if entry == nil {
		return
	}
	if entry.Path == path {
		mt.entries[b] = nil
		return
	}

	epl := len(entry.Path)
	if (entry.ContentType == ManifestType) && (len(path) >= epl) && (path[:epl] == entry.Path) {
		if mt.loadSubTrie(entry, quitC) != nil {
			return
		}
		entry.subtrie.deleteEntry(path[epl:], quitC)
		entry.Hash = ""
//如果子树少于2个元素，则删除它
		cnt, lastentry := entry.subtrie.getCountLast()
		if cnt < 2 {
			if lastentry != nil {
				lastentry.Path = entry.Path + lastentry.Path
			}
			mt.entries[b] = lastentry
		}
	}
}

func (mt *manifestTrie) recalcAndStore() error {
	if mt.ref != nil {
		return nil
	}

	var buffer bytes.Buffer
	buffer.WriteString(`{"entries":[`)

	list := &Manifest{}
	for _, entry := range &mt.entries {
		if entry != nil {
if entry.Hash == "" { //托多：平行化
				err := entry.subtrie.recalcAndStore()
				if err != nil {
					return err
				}
				entry.Hash = entry.subtrie.ref.Hex()
			}
			list.Entries = append(list.Entries, entry.ManifestEntry)
		}

	}

	manifest, err := json.Marshal(list)
	if err != nil {
		return err
	}

	sr := bytes.NewReader(manifest)
	ctx := context.TODO()
	addr, wait, err2 := mt.fileStore.Store(ctx, sr, int64(len(manifest)), mt.encrypted)
	if err2 != nil {
		return err2
	}
	err2 = wait(ctx)
	mt.ref = addr
	return err2
}

func (mt *manifestTrie) loadSubTrie(entry *manifestTrieEntry, quitC chan bool) (err error) {
	if entry.ManifestEntry.Access != nil {
		if mt.decrypt == nil {
			return errors.New("dont have decryptor")
		}

		err := mt.decrypt(&entry.ManifestEntry)
		if err != nil {
			return err
		}
	}

	if entry.subtrie == nil {
		hash := common.Hex2Bytes(entry.Hash)
		entry.subtrie, err = loadManifest(context.TODO(), mt.fileStore, hash, quitC, mt.decrypt)
entry.Hash = "" //可能不匹配，应重新计算
	}
	return
}

func (mt *manifestTrie) listWithPrefixInt(prefix, rp string, quitC chan bool, cb func(entry *manifestTrieEntry, suffix string)) error {
	plen := len(prefix)
	var start, stop int
	if plen == 0 {
		start = 0
		stop = 256
	} else {
		start = int(prefix[0])
		stop = start
	}

	for i := start; i <= stop; i++ {
		select {
		case <-quitC:
			return fmt.Errorf("aborted")
		default:
		}
		entry := mt.entries[i]
		if entry != nil {
			epl := len(entry.Path)
			if entry.ContentType == ManifestType {
				l := plen
				if epl < l {
					l = epl
				}
				if prefix[:l] == entry.Path[:l] {
					err := mt.loadSubTrie(entry, quitC)
					if err != nil {
						return err
					}
					err = entry.subtrie.listWithPrefixInt(prefix[l:], rp+entry.Path[l:], quitC, cb)
					if err != nil {
						return err
					}
				}
			} else {
				if (epl >= plen) && (prefix == entry.Path[:plen]) {
					cb(entry, rp+entry.Path[plen:])
				}
			}
		}
	}
	return nil
}

func (mt *manifestTrie) listWithPrefix(prefix string, quitC chan bool, cb func(entry *manifestTrieEntry, suffix string)) (err error) {
	return mt.listWithPrefixInt(prefix, "", quitC, cb)
}

func (mt *manifestTrie) findPrefixOf(path string, quitC chan bool) (entry *manifestTrieEntry, pos int) {
	log.Trace(fmt.Sprintf("findPrefixOf(%s)", path))

	if len(path) == 0 {
		return mt.entries[256], 0
	}

//查看第一个字符是否在清单项中
	b := path[0]
	entry = mt.entries[b]
	if entry == nil {
		return mt.entries[256], 0
	}

	epl := len(entry.Path)
	log.Trace(fmt.Sprintf("path = %v  entry.Path = %v  epl = %v", path, entry.Path, epl))
	if len(path) <= epl {
		if entry.Path[:len(path)] == path {
			if entry.ContentType == ManifestType {
				err := mt.loadSubTrie(entry, quitC)
				if err == nil && entry.subtrie != nil {
					subentries := entry.subtrie.entries
					for i := 0; i < len(subentries); i++ {
						sub := subentries[i]
						if sub != nil && sub.Path == "" {
							return sub, len(path)
						}
					}
				}
				entry.Status = http.StatusMultipleChoices
			}
			pos = len(path)
			return
		}
		return nil, 0
	}
	if path[:epl] == entry.Path {
		log.Trace(fmt.Sprintf("entry.ContentType = %v", entry.ContentType))
//子条目是清单、加载子条目
		if entry.ContentType == ManifestType && (strings.Contains(entry.Path, path) || strings.Contains(path, entry.Path)) {
			err := mt.loadSubTrie(entry, quitC)
			if err != nil {
				return nil, 0
			}
			sub, pos := entry.subtrie.findPrefixOf(path[epl:], quitC)
			if sub != nil {
				entry = sub
				pos += epl
				return sub, pos
			} else if path == entry.Path {
				entry.Status = http.StatusMultipleChoices
			}

		} else {
//条目不是清单，请将其返回
			if path != entry.Path {
				return nil, 0
			}
		}
	}
	return nil, 0
}

//文件系统清单始终包含规范化路径
//没有前导或尾随斜杠，内部只有单个斜杠
func RegularSlashes(path string) (res string) {
	for i := 0; i < len(path); i++ {
		if (path[i] != '/') || ((i > 0) && (path[i-1] != '/')) {
			res = res + path[i:i+1]
		}
	}
	if (len(res) > 0) && (res[len(res)-1] == '/') {
		res = res[:len(res)-1]
	}
	return
}

func (mt *manifestTrie) getEntry(spath string) (entry *manifestTrieEntry, fullpath string) {
	path := RegularSlashes(spath)
	var pos int
	quitC := make(chan bool)
	entry, pos = mt.findPrefixOf(path, quitC)
	return entry, path[:pos]
}
