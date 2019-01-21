
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

//go:生成mime gen--types=/../../cmd/swarm/mime gen/mime.types--package=api--out=gen_mime.go
//go：生成gofmt-s-w gen_mime.go

import (
	"archive/tar"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"path"
	"strings"

	"bytes"
	"mime"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/ens"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/spancontext"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/feed"
	"github.com/ethereum/go-ethereum/swarm/storage/feed/lookup"

	opentracing "github.com/opentracing/opentracing-go"
)

var (
	apiResolveCount        = metrics.NewRegisteredCounter("api.resolve.count", nil)
	apiResolveFail         = metrics.NewRegisteredCounter("api.resolve.fail", nil)
	apiPutCount            = metrics.NewRegisteredCounter("api.put.count", nil)
	apiPutFail             = metrics.NewRegisteredCounter("api.put.fail", nil)
	apiGetCount            = metrics.NewRegisteredCounter("api.get.count", nil)
	apiGetNotFound         = metrics.NewRegisteredCounter("api.get.notfound", nil)
	apiGetHTTP300          = metrics.NewRegisteredCounter("api.get.http.300", nil)
	apiManifestUpdateCount = metrics.NewRegisteredCounter("api.manifestupdate.count", nil)
	apiManifestUpdateFail  = metrics.NewRegisteredCounter("api.manifestupdate.fail", nil)
	apiManifestListCount   = metrics.NewRegisteredCounter("api.manifestlist.count", nil)
	apiManifestListFail    = metrics.NewRegisteredCounter("api.manifestlist.fail", nil)
	apiDeleteCount         = metrics.NewRegisteredCounter("api.delete.count", nil)
	apiDeleteFail          = metrics.NewRegisteredCounter("api.delete.fail", nil)
	apiGetTarCount         = metrics.NewRegisteredCounter("api.gettar.count", nil)
	apiGetTarFail          = metrics.NewRegisteredCounter("api.gettar.fail", nil)
	apiUploadTarCount      = metrics.NewRegisteredCounter("api.uploadtar.count", nil)
	apiUploadTarFail       = metrics.NewRegisteredCounter("api.uploadtar.fail", nil)
	apiModifyCount         = metrics.NewRegisteredCounter("api.modify.count", nil)
	apiModifyFail          = metrics.NewRegisteredCounter("api.modify.fail", nil)
	apiAddFileCount        = metrics.NewRegisteredCounter("api.addfile.count", nil)
	apiAddFileFail         = metrics.NewRegisteredCounter("api.addfile.fail", nil)
	apiRmFileCount         = metrics.NewRegisteredCounter("api.removefile.count", nil)
	apiRmFileFail          = metrics.NewRegisteredCounter("api.removefile.fail", nil)
	apiAppendFileCount     = metrics.NewRegisteredCounter("api.appendfile.count", nil)
	apiAppendFileFail      = metrics.NewRegisteredCounter("api.appendfile.fail", nil)
	apiGetInvalid          = metrics.NewRegisteredCounter("api.get.invalid", nil)
)

//解析程序接口使用ens将域名解析为哈希
type Resolver interface {
	Resolve(string) (common.Hash, error)
}

//ResolveValidator用于验证包含的冲突解决程序
type ResolveValidator interface {
	Resolver
	Owner(node [32]byte) (common.Address, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
}

//多解析程序返回noresolvererrror。如果没有解析程序，则解析
//可以找到地址。
type NoResolverError struct {
	TLD string
}

//NewNoresolveError为给定的顶级域创建NoresolveError
func NewNoResolverError(tld string) *NoResolverError {
	return &NoResolverError{TLD: tld}
}

//错误NoresolveError实现错误
func (e *NoResolverError) Error() string {
	if e.TLD == "" {
		return "no ENS resolver"
	}
	return fmt.Sprintf("no ENS endpoint configured to resolve .%s TLD names", e.TLD)
}

//多解析器用于基于TLD解析URL地址。
//每个TLD可以有多个解析器，并且来自
//将返回序列中的第一个。
type MultiResolver struct {
	resolvers map[string][]ResolveValidator
	nameHash  func(string) common.Hash
}

//multisolver选项为multisolver设置选项，并用作
//其构造函数的参数。
type MultiResolverOption func(*MultiResolver)

//带冲突解决程序的多冲突解决程序选项将冲突解决程序添加到冲突解决程序列表中
//对于特定的TLD。如果tld是空字符串，将添加冲突解决程序
//到默认冲突解决程序列表中，将用于解决问题的冲突解决程序
//没有指定TLD解析程序的地址。
func MultiResolverOptionWithResolver(r ResolveValidator, tld string) MultiResolverOption {
	return func(m *MultiResolver) {
		m.resolvers[tld] = append(m.resolvers[tld], r)
	}
}

//newmultisolver创建multisolver的新实例。
func NewMultiResolver(opts ...MultiResolverOption) (m *MultiResolver) {
	m = &MultiResolver{
		resolvers: make(map[string][]ResolveValidator),
		nameHash:  ens.EnsNode,
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

//解析通过TLD选择一个解析程序来解析地址。
//如果有更多的默认解析器，或者特定的TLD，
//第一个不返回错误的哈希
//将被退回。
func (m *MultiResolver) Resolve(addr string) (h common.Hash, err error) {
	rs, err := m.getResolveValidator(addr)
	if err != nil {
		return h, err
	}
	for _, r := range rs {
		h, err = r.Resolve(addr)
		if err == nil {
			return
		}
	}
	return
}

//GetResolveValidator使用主机名检索与顶级域关联的冲突解决程序
func (m *MultiResolver) getResolveValidator(name string) ([]ResolveValidator, error) {
	rs := m.resolvers[""]
	tld := path.Ext(name)
	if tld != "" {
		tld = tld[1:]
		rstld, ok := m.resolvers[tld]
		if ok {
			return rstld, nil
		}
	}
	if len(rs) == 0 {
		return rs, NewNoResolverError(tld)
	}
	return rs, nil
}

/*
API实现了与Web服务器/文件系统相关的内容存储和检索
在文件存储上
它是包含在以太坊堆栈中的文件存储的公共接口。
**/

type API struct {
	feed      *feed.Handler
	fileStore *storage.FileStore
	dns       Resolver
	Decryptor func(context.Context, string) DecryptFunc
}

//new api API构造函数初始化新的API实例。
func NewAPI(fileStore *storage.FileStore, dns Resolver, feedHandler *feed.Handler, pk *ecdsa.PrivateKey) (self *API) {
	self = &API{
		fileStore: fileStore,
		dns:       dns,
		feed:      feedHandler,
		Decryptor: func(ctx context.Context, credentials string) DecryptFunc {
			return self.doDecrypt(ctx, credentials, pk)
		},
	}
	return
}

//检索文件存储读取器API
func (a *API) Retrieve(ctx context.Context, addr storage.Address) (reader storage.LazySectionReader, isEncrypted bool) {
	return a.fileStore.Retrieve(ctx, addr)
}

//store包装嵌入式文件存储的store api调用
func (a *API) Store(ctx context.Context, data io.Reader, size int64, toEncrypt bool) (addr storage.Address, wait func(ctx context.Context) error, err error) {
	log.Debug("api.store", "size", size)
	return a.fileStore.Store(ctx, data, size, toEncrypt)
}

//将名称解析为内容寻址哈希
//其中地址可以是ENS名称，也可以是内容寻址哈希
func (a *API) Resolve(ctx context.Context, address string) (storage.Address, error) {
//如果未配置DNS，则返回错误
	if a.dns == nil {
		if hashMatcher.MatchString(address) {
			return common.Hex2Bytes(address), nil
		}
		apiResolveFail.Inc(1)
		return nil, fmt.Errorf("no DNS to resolve name: %q", address)
	}
//尝试解析地址
	resolved, err := a.dns.Resolve(address)
	if err != nil {
		if hashMatcher.MatchString(address) {
			return common.Hex2Bytes(address), nil
		}
		return nil, err
	}
	return resolved[:], nil
}

//解析使用多解析程序将URI解析为地址。
func (a *API) ResolveURI(ctx context.Context, uri *URI, credentials string) (storage.Address, error) {
	apiResolveCount.Inc(1)
	log.Trace("resolving", "uri", uri.Addr)

	var sp opentracing.Span
	ctx, sp = spancontext.StartSpan(
		ctx,
		"api.resolve")
	defer sp.Finish()

//如果URI不可变，请检查地址是否像哈希
	if uri.Immutable() {
		key := uri.Address()
		if key == nil {
			return nil, fmt.Errorf("immutable address not a content hash: %q", uri.Addr)
		}
		return key, nil
	}

	addr, err := a.Resolve(ctx, uri.Addr)
	if err != nil {
		return nil, err
	}

	if uri.Path == "" {
		return addr, nil
	}
	walker, err := a.NewManifestWalker(ctx, addr, a.Decryptor(ctx, credentials), nil)
	if err != nil {
		return nil, err
	}
	var entry *ManifestEntry
	walker.Walk(func(e *ManifestEntry) error {
//如果条目与路径匹配，则设置条目并停止
//步行
		if e.Path == uri.Path {
			entry = e
//返回错误以取消漫游
			return errors.New("found")
		}
//忽略非清单文件
		if e.ContentType != ManifestType {
			return nil
		}
//如果清单的路径是
//请求的路径，通过返回
//零，继续走
		if strings.HasPrefix(uri.Path, e.Path) {
			return nil
		}
		return ErrSkipManifest
	})
	if entry == nil {
		return nil, errors.New("not found")
	}
	addr = storage.Address(common.Hex2Bytes(entry.Hash))
	return addr, nil
}

//Put在文件存储区顶部提供单实例清单创建
func (a *API) Put(ctx context.Context, content string, contentType string, toEncrypt bool) (k storage.Address, wait func(context.Context) error, err error) {
	apiPutCount.Inc(1)
	r := strings.NewReader(content)
	key, waitContent, err := a.fileStore.Store(ctx, r, int64(len(content)), toEncrypt)
	if err != nil {
		apiPutFail.Inc(1)
		return nil, nil, err
	}
	manifest := fmt.Sprintf(`{"entries":[{"hash":"%v","contentType":"%s"}]}`, key, contentType)
	r = strings.NewReader(manifest)
	key, waitManifest, err := a.fileStore.Store(ctx, r, int64(len(manifest)), toEncrypt)
	if err != nil {
		apiPutFail.Inc(1)
		return nil, nil, err
	}
	return key, func(ctx context.Context) error {
		err := waitContent(ctx)
		if err != nil {
			return err
		}
		return waitManifest(ctx)
	}, nil
}

//get使用迭代清单检索和前缀匹配
//使用文件存储检索解析内容的基路径
//它返回一个段落阅读器、mimetype、状态、实际内容的键和一个错误。
func (a *API) Get(ctx context.Context, decrypt DecryptFunc, manifestAddr storage.Address, path string) (reader storage.LazySectionReader, mimeType string, status int, contentAddr storage.Address, err error) {
	log.Debug("api.get", "key", manifestAddr, "path", path)
	apiGetCount.Inc(1)
	trie, err := loadManifest(ctx, a.fileStore, manifestAddr, nil, decrypt)
	if err != nil {
		apiGetNotFound.Inc(1)
		status = http.StatusNotFound
		return nil, "", http.StatusNotFound, nil, err
	}

	log.Debug("trie getting entry", "key", manifestAddr, "path", path)
	entry, _ := trie.getEntry(path)

	if entry != nil {
		log.Debug("trie got entry", "key", manifestAddr, "path", path, "entry.Hash", entry.Hash)

		if entry.ContentType == ManifestType {
			log.Debug("entry is manifest", "key", manifestAddr, "new key", entry.Hash)
			adr, err := hex.DecodeString(entry.Hash)
			if err != nil {
				return nil, "", 0, nil, err
			}
			return a.Get(ctx, decrypt, adr, entry.Path)
		}

//如果这是群饲料清单，我们需要做一些额外的工作
		if entry.ContentType == FeedContentType {
			if entry.Feed == nil {
				return reader, mimeType, status, nil, fmt.Errorf("Cannot decode Feed in manifest")
			}
			_, err := a.feed.Lookup(ctx, feed.NewQueryLatest(entry.Feed, lookup.NoClue))
			if err != nil {
				apiGetNotFound.Inc(1)
				status = http.StatusNotFound
				log.Debug(fmt.Sprintf("get feed update content error: %v", err))
				return reader, mimeType, status, nil, err
			}
//获取更新的数据
			_, contentAddr, err := a.feed.GetContent(entry.Feed)
			if err != nil {
				apiGetNotFound.Inc(1)
				status = http.StatusNotFound
				log.Warn(fmt.Sprintf("get feed update content error: %v", err))
				return reader, mimeType, status, nil, err
			}

//提取内容哈希
			if len(contentAddr) != storage.AddressLength {
				apiGetInvalid.Inc(1)
				status = http.StatusUnprocessableEntity
				errorMessage := fmt.Sprintf("invalid swarm hash in feed update. Expected %d bytes. Got %d", storage.AddressLength, len(contentAddr))
				log.Warn(errorMessage)
				return reader, mimeType, status, nil, errors.New(errorMessage)
			}
			manifestAddr = storage.Address(contentAddr)
			log.Trace("feed update contains swarm hash", "key", manifestAddr)

//获取蜂群散列点指向的清单
			trie, err := loadManifest(ctx, a.fileStore, manifestAddr, nil, NOOPDecrypt)
			if err != nil {
				apiGetNotFound.Inc(1)
				status = http.StatusNotFound
				log.Warn(fmt.Sprintf("loadManifestTrie (feed update) error: %v", err))
				return reader, mimeType, status, nil, err
			}

//最后，获取清单条目
//它始终是路径“”上的条目
			entry, _ = trie.getEntry(path)
			if entry == nil {
				status = http.StatusNotFound
				apiGetNotFound.Inc(1)
				err = fmt.Errorf("manifest (feed update) entry for '%s' not found", path)
				log.Trace("manifest (feed update) entry not found", "key", manifestAddr, "path", path)
				return reader, mimeType, status, nil, err
			}
		}

//无论是feed更新清单还是普通清单，我们都将在此收敛
//获取清单入口点的关键点，如果不含糊，则提供服务
		contentAddr = common.Hex2Bytes(entry.Hash)
		status = entry.Status
		if status == http.StatusMultipleChoices {
			apiGetHTTP300.Inc(1)
			return nil, entry.ContentType, status, contentAddr, err
		}
		mimeType = entry.ContentType
		log.Debug("content lookup key", "key", contentAddr, "mimetype", mimeType)
		reader, _ = a.fileStore.Retrieve(ctx, contentAddr)
	} else {
//未找到条目
		status = http.StatusNotFound
		apiGetNotFound.Inc(1)
		err = fmt.Errorf("Not found: could not find resource '%s'", path)
		log.Trace("manifest entry not found", "key", contentAddr, "path", path)
	}
	return
}

func (a *API) Delete(ctx context.Context, addr string, path string) (storage.Address, error) {
	apiDeleteCount.Inc(1)
	uri, err := Parse("bzz:/" + addr)
	if err != nil {
		apiDeleteFail.Inc(1)
		return nil, err
	}
	key, err := a.ResolveURI(ctx, uri, EMPTY_CREDENTIALS)

	if err != nil {
		return nil, err
	}
	newKey, err := a.UpdateManifest(ctx, key, func(mw *ManifestWriter) error {
		log.Debug(fmt.Sprintf("removing %s from manifest %s", path, key.Log()))
		return mw.RemoveEntry(path)
	})
	if err != nil {
		apiDeleteFail.Inc(1)
		return nil, err
	}

	return newKey, nil
}

//GetDirectorytar以tarstream形式获取请求的目录
//它返回一个IO.reader和一个错误。不要忘记关闭（）返回的readcloser
func (a *API) GetDirectoryTar(ctx context.Context, decrypt DecryptFunc, uri *URI) (io.ReadCloser, error) {
	apiGetTarCount.Inc(1)
	addr, err := a.Resolve(ctx, uri.Addr)
	if err != nil {
		return nil, err
	}
	walker, err := a.NewManifestWalker(ctx, addr, decrypt, nil)
	if err != nil {
		apiGetTarFail.Inc(1)
		return nil, err
	}

	piper, pipew := io.Pipe()

	tw := tar.NewWriter(pipew)

	go func() {
		err := walker.Walk(func(entry *ManifestEntry) error {
//忽略清单（walk将重复出现在清单中）
			if entry.ContentType == ManifestType {
				return nil
			}

//检索条目的密钥和大小
			reader, _ := a.Retrieve(ctx, storage.Address(common.Hex2Bytes(entry.Hash)))
			size, err := reader.Size(ctx, nil)
			if err != nil {
				return err
			}

//为条目编写tar头
			hdr := &tar.Header{
				Name:    entry.Path,
				Mode:    entry.Mode,
				Size:    size,
				ModTime: entry.ModTime,
				Xattrs: map[string]string{
					"user.swarm.content-type": entry.ContentType,
				},
			}

			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}

//将文件复制到tar流中
			n, err := io.Copy(tw, io.LimitReader(reader, hdr.Size))
			if err != nil {
				return err
			} else if n != size {
				return fmt.Errorf("error writing %s: expected %d bytes but sent %d", entry.Path, size, n)
			}

			return nil
		})
//关闭tar writer，然后关闭pipew
//将剩余数据刷新到pipew
//不考虑误差值
		tw.Close()
		if err != nil {
			apiGetTarFail.Inc(1)
			pipew.CloseWithError(err)
		} else {
			pipew.Close()
		}
	}()

	return piper, nil
}

//GetManifestList列出指定地址和前缀的清单项
//并将其作为清单返回
func (a *API) GetManifestList(ctx context.Context, decryptor DecryptFunc, addr storage.Address, prefix string) (list ManifestList, err error) {
	apiManifestListCount.Inc(1)
	walker, err := a.NewManifestWalker(ctx, addr, decryptor, nil)
	if err != nil {
		apiManifestListFail.Inc(1)
		return ManifestList{}, err
	}

	err = walker.Walk(func(entry *ManifestEntry) error {
//处理非清单文件
		if entry.ContentType != ManifestType {
//如果文件没有指定的前缀，则忽略该文件
			if !strings.HasPrefix(entry.Path, prefix) {
				return nil
			}

//如果前缀后面的路径包含斜杠，请添加
//列表的公共前缀，否则添加条目
			suffix := strings.TrimPrefix(entry.Path, prefix)
			if index := strings.Index(suffix, "/"); index > -1 {
				list.CommonPrefixes = append(list.CommonPrefixes, prefix+suffix[:index+1])
				return nil
			}
			if entry.Path == "" {
				entry.Path = "/"
			}
			list.Entries = append(list.Entries, entry)
			return nil
		}

//如果清单的路径是指定前缀的前缀
//然后通过返回nil和
//继续散步
		if strings.HasPrefix(prefix, entry.Path) {
			return nil
		}

//如果清单的路径具有指定的前缀，则如果
//前缀后面的路径包含斜杠，请添加一个公共前缀
//到列表并跳过清单，否则递归到
//通过返回零并继续行走来显示
		if strings.HasPrefix(entry.Path, prefix) {
			suffix := strings.TrimPrefix(entry.Path, prefix)
			if index := strings.Index(suffix, "/"); index > -1 {
				list.CommonPrefixes = append(list.CommonPrefixes, prefix+suffix[:index+1])
				return ErrSkipManifest
			}
			return nil
		}

//清单既没有前缀，也不需要递归到
//所以跳过它
		return ErrSkipManifest
	})

	if err != nil {
		apiManifestListFail.Inc(1)
		return ManifestList{}, err
	}

	return list, nil
}

func (a *API) UpdateManifest(ctx context.Context, addr storage.Address, update func(mw *ManifestWriter) error) (storage.Address, error) {
	apiManifestUpdateCount.Inc(1)
	mw, err := a.NewManifestWriter(ctx, addr, nil)
	if err != nil {
		apiManifestUpdateFail.Inc(1)
		return nil, err
	}

	if err := update(mw); err != nil {
		apiManifestUpdateFail.Inc(1)
		return nil, err
	}

	addr, err = mw.Store()
	if err != nil {
		apiManifestUpdateFail.Inc(1)
		return nil, err
	}
	log.Debug(fmt.Sprintf("generated manifest %s", addr))
	return addr, nil
}

//修改加载清单并在重新计算和存储清单之前检查内容哈希。
func (a *API) Modify(ctx context.Context, addr storage.Address, path, contentHash, contentType string) (storage.Address, error) {
	apiModifyCount.Inc(1)
	quitC := make(chan bool)
	trie, err := loadManifest(ctx, a.fileStore, addr, quitC, NOOPDecrypt)
	if err != nil {
		apiModifyFail.Inc(1)
		return nil, err
	}
	if contentHash != "" {
		entry := newManifestTrieEntry(&ManifestEntry{
			Path:        path,
			ContentType: contentType,
		}, nil)
		entry.Hash = contentHash
		trie.addEntry(entry, quitC)
	} else {
		trie.deleteEntry(path, quitC)
	}

	if err := trie.recalcAndStore(); err != nil {
		apiModifyFail.Inc(1)
		return nil, err
	}
	return trie.ref, nil
}

//addfile创建一个新的清单条目，将其添加到swarm，然后将文件添加到swarm。
func (a *API) AddFile(ctx context.Context, mhash, path, fname string, content []byte, nameresolver bool) (storage.Address, string, error) {
	apiAddFileCount.Inc(1)

	uri, err := Parse("bzz:/" + mhash)
	if err != nil {
		apiAddFileFail.Inc(1)
		return nil, "", err
	}
	mkey, err := a.ResolveURI(ctx, uri, EMPTY_CREDENTIALS)
	if err != nil {
		apiAddFileFail.Inc(1)
		return nil, "", err
	}

//修剪我们添加的根目录
	if path[:1] == "/" {
		path = path[1:]
	}

	entry := &ManifestEntry{
		Path:        filepath.Join(path, fname),
		ContentType: mime.TypeByExtension(filepath.Ext(fname)),
		Mode:        0700,
		Size:        int64(len(content)),
		ModTime:     time.Now(),
	}

	mw, err := a.NewManifestWriter(ctx, mkey, nil)
	if err != nil {
		apiAddFileFail.Inc(1)
		return nil, "", err
	}

	fkey, err := mw.AddEntry(ctx, bytes.NewReader(content), entry)
	if err != nil {
		apiAddFileFail.Inc(1)
		return nil, "", err
	}

	newMkey, err := mw.Store()
	if err != nil {
		apiAddFileFail.Inc(1)
		return nil, "", err

	}

	return fkey, newMkey.String(), nil
}

func (a *API) UploadTar(ctx context.Context, bodyReader io.ReadCloser, manifestPath, defaultPath string, mw *ManifestWriter) (storage.Address, error) {
	apiUploadTarCount.Inc(1)
	var contentKey storage.Address
	tr := tar.NewReader(bodyReader)
	defer bodyReader.Close()
	var defaultPathFound bool
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			apiUploadTarFail.Inc(1)
			return nil, fmt.Errorf("error reading tar stream: %s", err)
		}

//仅存储常规文件
		if !hdr.FileInfo().Mode().IsRegular() {
			continue
		}

//在请求的路径下添加条目
		manifestPath := path.Join(manifestPath, hdr.Name)
		contentType := hdr.Xattrs["user.swarm.content-type"]
		if contentType == "" {
			contentType = mime.TypeByExtension(filepath.Ext(hdr.Name))
		}
//检测内容类型（“”）
		entry := &ManifestEntry{
			Path:        manifestPath,
			ContentType: contentType,
			Mode:        hdr.Mode,
			Size:        hdr.Size,
			ModTime:     hdr.ModTime,
		}
		contentKey, err = mw.AddEntry(ctx, tr, entry)
		if err != nil {
			apiUploadTarFail.Inc(1)
			return nil, fmt.Errorf("error adding manifest entry from tar stream: %s", err)
		}
		if hdr.Name == defaultPath {
			contentType := hdr.Xattrs["user.swarm.content-type"]
			if contentType == "" {
				contentType = mime.TypeByExtension(filepath.Ext(hdr.Name))
			}

			entry := &ManifestEntry{
				Hash:        contentKey.Hex(),
Path:        "", //缺省条目
				ContentType: contentType,
				Mode:        hdr.Mode,
				Size:        hdr.Size,
				ModTime:     hdr.ModTime,
			}
			contentKey, err = mw.AddEntry(ctx, nil, entry)
			if err != nil {
				apiUploadTarFail.Inc(1)
				return nil, fmt.Errorf("error adding default manifest entry from tar stream: %s", err)
			}
			defaultPathFound = true
		}
	}
	if defaultPath != "" && !defaultPathFound {
		return contentKey, fmt.Errorf("default path %q not found", defaultPath)
	}
	return contentKey, nil
}

//removefile删除清单中的文件条目。
func (a *API) RemoveFile(ctx context.Context, mhash string, path string, fname string, nameresolver bool) (string, error) {
	apiRmFileCount.Inc(1)

	uri, err := Parse("bzz:/" + mhash)
	if err != nil {
		apiRmFileFail.Inc(1)
		return "", err
	}
	mkey, err := a.ResolveURI(ctx, uri, EMPTY_CREDENTIALS)
	if err != nil {
		apiRmFileFail.Inc(1)
		return "", err
	}

//修剪我们添加的根目录
	if path[:1] == "/" {
		path = path[1:]
	}

	mw, err := a.NewManifestWriter(ctx, mkey, nil)
	if err != nil {
		apiRmFileFail.Inc(1)
		return "", err
	}

	err = mw.RemoveEntry(filepath.Join(path, fname))
	if err != nil {
		apiRmFileFail.Inc(1)
		return "", err
	}

	newMkey, err := mw.Store()
	if err != nil {
		apiRmFileFail.Inc(1)
		return "", err

	}

	return newMkey.String(), nil
}

//AppendFile删除旧清单，将文件条目附加到新清单，并将其添加到Swarm。
func (a *API) AppendFile(ctx context.Context, mhash, path, fname string, existingSize int64, content []byte, oldAddr storage.Address, offset int64, addSize int64, nameresolver bool) (storage.Address, string, error) {
	apiAppendFileCount.Inc(1)

	buffSize := offset + addSize
	if buffSize < existingSize {
		buffSize = existingSize
	}

	buf := make([]byte, buffSize)

	oldReader, _ := a.Retrieve(ctx, oldAddr)
	io.ReadAtLeast(oldReader, buf, int(offset))

	newReader := bytes.NewReader(content)
	io.ReadAtLeast(newReader, buf[offset:], int(addSize))

	if buffSize < existingSize {
		io.ReadAtLeast(oldReader, buf[addSize:], int(buffSize))
	}

	combinedReader := bytes.NewReader(buf)
	totalSize := int64(len(buf))

//todo（jmozah）：准备好后使用金字塔chunker附加
//oldreader：=a.retrieve（oldkey）
//newreader:=字节。newreader（内容）
//组合读卡器：=IO.MultiReader（OldReader、NewReader）

	uri, err := Parse("bzz:/" + mhash)
	if err != nil {
		apiAppendFileFail.Inc(1)
		return nil, "", err
	}
	mkey, err := a.ResolveURI(ctx, uri, EMPTY_CREDENTIALS)
	if err != nil {
		apiAppendFileFail.Inc(1)
		return nil, "", err
	}

//修剪我们添加的根目录
	if path[:1] == "/" {
		path = path[1:]
	}

	mw, err := a.NewManifestWriter(ctx, mkey, nil)
	if err != nil {
		apiAppendFileFail.Inc(1)
		return nil, "", err
	}

	err = mw.RemoveEntry(filepath.Join(path, fname))
	if err != nil {
		apiAppendFileFail.Inc(1)
		return nil, "", err
	}

	entry := &ManifestEntry{
		Path:        filepath.Join(path, fname),
		ContentType: mime.TypeByExtension(filepath.Ext(fname)),
		Mode:        0700,
		Size:        totalSize,
		ModTime:     time.Now(),
	}

	fkey, err := mw.AddEntry(ctx, io.Reader(combinedReader), entry)
	if err != nil {
		apiAppendFileFail.Inc(1)
		return nil, "", err
	}

	newMkey, err := mw.Store()
	if err != nil {
		apiAppendFileFail.Inc(1)
		return nil, "", err

	}

	return fkey, newMkey.String(), nil
}

//swarmfs_Unix使用的buildDirectoryTree
func (a *API) BuildDirectoryTree(ctx context.Context, mhash string, nameresolver bool) (addr storage.Address, manifestEntryMap map[string]*manifestTrieEntry, err error) {

	uri, err := Parse("bzz:/" + mhash)
	if err != nil {
		return nil, nil, err
	}
	addr, err = a.Resolve(ctx, uri.Addr)
	if err != nil {
		return nil, nil, err
	}

	quitC := make(chan bool)
	rootTrie, err := loadManifest(ctx, a.fileStore, addr, quitC, NOOPDecrypt)
	if err != nil {
		return nil, nil, fmt.Errorf("can't load manifest %v: %v", addr.String(), err)
	}

	manifestEntryMap = map[string]*manifestTrieEntry{}
	err = rootTrie.listWithPrefix(uri.Path, quitC, func(entry *manifestTrieEntry, suffix string) {
		manifestEntryMap[suffix] = entry
	})

	if err != nil {
		return nil, nil, fmt.Errorf("list with prefix failed %v: %v", addr.String(), err)
	}
	return addr, manifestEntryMap, nil
}

//feedslookup在特定时间点发现swarm feeds更新，或最新更新
func (a *API) FeedsLookup(ctx context.Context, query *feed.Query) ([]byte, error) {
	_, err := a.feed.Lookup(ctx, query)
	if err != nil {
		return nil, err
	}
	var data []byte
	_, data, err = a.feed.GetContent(&query.Feed)
	if err != nil {
		return nil, err
	}
	return data, nil
}

//FeedsNewRequest创建一个请求对象来更新特定的源
func (a *API) FeedsNewRequest(ctx context.Context, feed *feed.Feed) (*feed.Request, error) {
	return a.feed.NewRequest(ctx, feed)
}

//feedsupdate发布给定源的新更新
func (a *API) FeedsUpdate(ctx context.Context, request *feed.Request) (storage.Address, error) {
	return a.feed.Update(ctx, request)
}

//当查找源清单失败时返回errCanNotLoadFeedManifest
var ErrCannotLoadFeedManifest = errors.New("Cannot load feed manifest")

//当提供的地址返回的不是有效清单时，将返回errnaFeedManifest。
var ErrNotAFeedManifest = errors.New("Not a feed manifest")

//resolveFeedManifest检索给定地址的swarm feed清单，并返回引用的feed。
func (a *API) ResolveFeedManifest(ctx context.Context, addr storage.Address) (*feed.Feed, error) {
	trie, err := loadManifest(ctx, a.fileStore, addr, nil, NOOPDecrypt)
	if err != nil {
		return nil, ErrCannotLoadFeedManifest
	}

	entry, _ := trie.getEntry("")
	if entry.ContentType != FeedContentType {
		return nil, ErrNotAFeedManifest
	}

	return entry.Feed, nil
}

//当ENS解析器无法将名称转换为群提要时，将返回errcannotresolvefeeduri。
var ErrCannotResolveFeedURI = errors.New("Cannot resolve Feed URI")

//当提供的值不足或无效，无法重新创建
//从他们那里汲取营养。
var ErrCannotResolveFeed = errors.New("Cannot resolve Feed")

//ResolveFeed尝试从清单中提取源信息（如果提供）
//如果不是，它将尝试从一组键值对中提取提要。
func (a *API) ResolveFeed(ctx context.Context, uri *URI, values feed.Values) (*feed.Feed, error) {
	var fd *feed.Feed
	var err error
	if uri.Addr != "" {
//解析内容键。
		manifestAddr := uri.Address()
		if manifestAddr == nil {
			manifestAddr, err = a.Resolve(ctx, uri.Addr)
			if err != nil {
				return nil, ErrCannotResolveFeedURI
			}
		}

//从清单中获取蜂群饲料
		fd, err = a.ResolveFeedManifest(ctx, manifestAddr)
		if err != nil {
			return nil, err
		}
		log.Debug("handle.get.feed: resolved", "manifestkey", manifestAddr, "feed", fd.Hex())
	} else {
		var f feed.Feed
		if err := f.FromValues(values); err != nil {
			return nil, ErrCannotResolveFeed

		}
		fd = &f
	}
	return fd, nil
}

//HTTP内容类型头的mimeotetstream默认值
const MimeOctetStream = "application/octet-stream"

//通过文件扩展名检测ContentType，或回退到内容探查
func DetectContentType(fileName string, f io.ReadSeeker) (string, error) {
	ctype := mime.TypeByExtension(filepath.Ext(fileName))
	if ctype != "" {
		return ctype, nil
	}

//保存/回滚以从文件开头获取内容探测
	currentPosition, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return MimeOctetStream, fmt.Errorf("seeker can't seek, %s", err)
	}

//读取块以决定UTF-8文本和二进制文件
	var buf [512]byte
	n, _ := f.Read(buf[:])
	ctype = http.DetectContentType(buf[:n])

_, err = f.Seek(currentPosition, io.SeekStart) //倒带以输出整个文件
	if err != nil {
		return MimeOctetStream, fmt.Errorf("seeker can't seek, %s", err)
	}

	return ctype, nil
}
