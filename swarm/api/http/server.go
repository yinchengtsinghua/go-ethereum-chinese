
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

/*
Swarm的简单HTTP服务器接口
**/

package http

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/swarm/api"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/feed"
	"github.com/rs/cors"
)

var (
	postRawCount    = metrics.NewRegisteredCounter("api.http.post.raw.count", nil)
	postRawFail     = metrics.NewRegisteredCounter("api.http.post.raw.fail", nil)
	postFilesCount  = metrics.NewRegisteredCounter("api.http.post.files.count", nil)
	postFilesFail   = metrics.NewRegisteredCounter("api.http.post.files.fail", nil)
	deleteCount     = metrics.NewRegisteredCounter("api.http.delete.count", nil)
	deleteFail      = metrics.NewRegisteredCounter("api.http.delete.fail", nil)
	getCount        = metrics.NewRegisteredCounter("api.http.get.count", nil)
	getFail         = metrics.NewRegisteredCounter("api.http.get.fail", nil)
	getFileCount    = metrics.NewRegisteredCounter("api.http.get.file.count", nil)
	getFileNotFound = metrics.NewRegisteredCounter("api.http.get.file.notfound", nil)
	getFileFail     = metrics.NewRegisteredCounter("api.http.get.file.fail", nil)
	getListCount    = metrics.NewRegisteredCounter("api.http.get.list.count", nil)
	getListFail     = metrics.NewRegisteredCounter("api.http.get.list.fail", nil)
)

type methodHandler map[string]http.Handler

func (m methodHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	v, ok := m[r.Method]
	if ok {
		v.ServeHTTP(rw, r)
		return
	}
	rw.WriteHeader(http.StatusMethodNotAllowed)
}

func NewServer(api *api.API, corsString string) *Server {
	var allowedOrigins []string
	for _, domain := range strings.Split(corsString, ",") {
		allowedOrigins = append(allowedOrigins, strings.TrimSpace(domain))
	}
	c := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{http.MethodPost, http.MethodGet, http.MethodDelete, http.MethodPatch, http.MethodPut},
		MaxAge:         600,
		AllowedHeaders: []string{"*"},
	})

	server := &Server{api: api}

	defaultMiddlewares := []Adapter{
		RecoverPanic,
		SetRequestID,
		SetRequestHost,
		InitLoggingResponseWriter,
		ParseURI,
		InstrumentOpenTracing,
	}

	mux := http.NewServeMux()
	mux.Handle("/bzz:/", methodHandler{
		"GET": Adapt(
			http.HandlerFunc(server.HandleBzzGet),
			defaultMiddlewares...,
		),
		"POST": Adapt(
			http.HandlerFunc(server.HandlePostFiles),
			defaultMiddlewares...,
		),
		"DELETE": Adapt(
			http.HandlerFunc(server.HandleDelete),
			defaultMiddlewares...,
		),
	})
	mux.Handle("/bzz-raw:/", methodHandler{
		"GET": Adapt(
			http.HandlerFunc(server.HandleGet),
			defaultMiddlewares...,
		),
		"POST": Adapt(
			http.HandlerFunc(server.HandlePostRaw),
			defaultMiddlewares...,
		),
	})
	mux.Handle("/bzz-immutable:/", methodHandler{
		"GET": Adapt(
			http.HandlerFunc(server.HandleBzzGet),
			defaultMiddlewares...,
		),
	})
	mux.Handle("/bzz-hash:/", methodHandler{
		"GET": Adapt(
			http.HandlerFunc(server.HandleGet),
			defaultMiddlewares...,
		),
	})
	mux.Handle("/bzz-list:/", methodHandler{
		"GET": Adapt(
			http.HandlerFunc(server.HandleGetList),
			defaultMiddlewares...,
		),
	})
	mux.Handle("/bzz-feed:/", methodHandler{
		"GET": Adapt(
			http.HandlerFunc(server.HandleGetFeed),
			defaultMiddlewares...,
		),
		"POST": Adapt(
			http.HandlerFunc(server.HandlePostFeed),
			defaultMiddlewares...,
		),
	})

	mux.Handle("/", methodHandler{
		"GET": Adapt(
			http.HandlerFunc(server.HandleRootPaths),
			SetRequestID,
			InitLoggingResponseWriter,
		),
	})
	server.Handler = c.Handler(mux)

	return server
}

func (s *Server) ListenAndServe(addr string) error {
	s.listenAddr = addr
	return http.ListenAndServe(addr, s)
}

//用于注册BZZ URL方案处理程序的浏览器API:
//https://developer.mozilla.org/en/docs/web-based_协议处理程序
//用于注册BZZ URL方案处理程序的电子（铬）API：
//https://github.com/atom/electron/blob/master/docs/api/protocol.md网站
type Server struct {
	http.Handler
	api        *api.API
	listenAddr string
}

func (s *Server) HandleBzzGet(w http.ResponseWriter, r *http.Request) {
	log.Debug("handleBzzGet", "ruid", GetRUID(r.Context()), "uri", r.RequestURI)
	if r.Header.Get("Accept") == "application/x-tar" {
		uri := GetURI(r.Context())
		_, credentials, _ := r.BasicAuth()
		reader, err := s.api.GetDirectoryTar(r.Context(), s.api.Decryptor(r.Context(), credentials), uri)
		if err != nil {
			if isDecryptError(err) {
				w.Header().Set("WWW-Authenticate", fmt.Sprintf("Basic realm=%q", uri.Address().String()))
				respondError(w, r, err.Error(), http.StatusUnauthorized)
				return
			}
			respondError(w, r, fmt.Sprintf("Had an error building the tarball: %v", err), http.StatusInternalServerError)
			return
		}
		defer reader.Close()

		w.Header().Set("Content-Type", "application/x-tar")

		fileName := uri.Addr
		if found := path.Base(uri.Path); found != "" && found != "." && found != "/" {
			fileName = found
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s.tar\"", fileName))

		w.WriteHeader(http.StatusOK)
		io.Copy(w, reader)
		return
	}

	s.HandleGetFile(w, r)
}

func (s *Server) HandleRootPaths(w http.ResponseWriter, r *http.Request) {
	switch r.RequestURI {
	case "/":
		respondTemplate(w, r, "landing-page", "Swarm: Please request a valid ENS or swarm hash with the appropriate bzz scheme", 200)
		return
	case "/robots.txt":
		w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
		fmt.Fprintf(w, "User-agent: *\nDisallow: /")
	case "/favicon.ico":
		w.WriteHeader(http.StatusOK)
		w.Write(faviconBytes)
	default:
		respondError(w, r, "Not Found", http.StatusNotFound)
	}
}

//handlePostraw处理对raw bzz raw的post请求：/uri，存储该请求
//swarm中的body并返回结果存储地址作为文本/纯响应
func (s *Server) HandlePostRaw(w http.ResponseWriter, r *http.Request) {
	ruid := GetRUID(r.Context())
	log.Debug("handle.post.raw", "ruid", ruid)

	postRawCount.Inc(1)

	toEncrypt := false
	uri := GetURI(r.Context())
	if uri.Addr == "encrypt" {
		toEncrypt = true
	}

	if uri.Path != "" {
		postRawFail.Inc(1)
		respondError(w, r, "raw POST request cannot contain a path", http.StatusBadRequest)
		return
	}

	if uri.Addr != "" && uri.Addr != "encrypt" {
		postRawFail.Inc(1)
		respondError(w, r, "raw POST request addr can only be empty or \"encrypt\"", http.StatusBadRequest)
		return
	}

	if r.Header.Get("Content-Length") == "" {
		postRawFail.Inc(1)
		respondError(w, r, "missing Content-Length header in request", http.StatusBadRequest)
		return
	}

	addr, _, err := s.api.Store(r.Context(), r.Body, r.ContentLength, toEncrypt)
	if err != nil {
		postRawFail.Inc(1)
		respondError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Debug("stored content", "ruid", ruid, "key", addr)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, addr)
}

//handlePostfiles处理对
//bzz:/<hash>/<path>其中包含单个文件或多个文件
//（tar存档或多部分表单），将这些文件添加到
//现有清单或<path>下的新清单，并返回
//作为文本/纯响应的结果清单哈希
func (s *Server) HandlePostFiles(w http.ResponseWriter, r *http.Request) {
	ruid := GetRUID(r.Context())
	log.Debug("handle.post.files", "ruid", ruid)
	postFilesCount.Inc(1)

	contentType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		postFilesFail.Inc(1)
		respondError(w, r, err.Error(), http.StatusBadRequest)
		return
	}

	toEncrypt := false
	uri := GetURI(r.Context())
	if uri.Addr == "encrypt" {
		toEncrypt = true
	}

	var addr storage.Address
	if uri.Addr != "" && uri.Addr != "encrypt" {
		addr, err = s.api.Resolve(r.Context(), uri.Addr)
		if err != nil {
			postFilesFail.Inc(1)
			respondError(w, r, fmt.Sprintf("cannot resolve %s: %s", uri.Addr, err), http.StatusInternalServerError)
			return
		}
		log.Debug("resolved key", "ruid", ruid, "key", addr)
	} else {
		addr, err = s.api.NewManifest(r.Context(), toEncrypt)
		if err != nil {
			postFilesFail.Inc(1)
			respondError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Debug("new manifest", "ruid", ruid, "key", addr)
	}

	newAddr, err := s.api.UpdateManifest(r.Context(), addr, func(mw *api.ManifestWriter) error {
		switch contentType {
		case "application/x-tar":
			_, err := s.handleTarUpload(r, mw)
			if err != nil {
				respondError(w, r, fmt.Sprintf("error uploading tarball: %v", err), http.StatusInternalServerError)
				return err
			}
			return nil
		case "multipart/form-data":
			return s.handleMultipartUpload(r, params["boundary"], mw)

		default:
			return s.handleDirectUpload(r, mw)
		}
	})
	if err != nil {
		postFilesFail.Inc(1)
		respondError(w, r, fmt.Sprintf("cannot create manifest: %s", err), http.StatusInternalServerError)
		return
	}

	log.Debug("stored content", "ruid", ruid, "key", newAddr)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, newAddr)
}

func (s *Server) handleTarUpload(r *http.Request, mw *api.ManifestWriter) (storage.Address, error) {
	log.Debug("handle.tar.upload", "ruid", GetRUID(r.Context()))

	defaultPath := r.URL.Query().Get("defaultpath")

	key, err := s.api.UploadTar(r.Context(), r.Body, GetURI(r.Context()).Path, defaultPath, mw)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (s *Server) handleMultipartUpload(r *http.Request, boundary string, mw *api.ManifestWriter) error {
	ruid := GetRUID(r.Context())
	log.Debug("handle.multipart.upload", "ruid", ruid)
	mr := multipart.NewReader(r.Body, boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("error reading multipart form: %s", err)
		}

		var size int64
		var reader io.Reader
		if contentLength := part.Header.Get("Content-Length"); contentLength != "" {
			size, err = strconv.ParseInt(contentLength, 10, 64)
			if err != nil {
				return fmt.Errorf("error parsing multipart content length: %s", err)
			}
			reader = part
		} else {
//将部件复制到tmp文件以获取其大小
			tmp, err := ioutil.TempFile("", "swarm-multipart")
			if err != nil {
				return err
			}
			defer os.Remove(tmp.Name())
			defer tmp.Close()
			size, err = io.Copy(tmp, part)
			if err != nil {
				return fmt.Errorf("error copying multipart content: %s", err)
			}
			if _, err := tmp.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("error copying multipart content: %s", err)
			}
			reader = tmp
		}

//在请求的路径下添加条目
		name := part.FileName()
		if name == "" {
			name = part.FormName()
		}
		uri := GetURI(r.Context())
		path := path.Join(uri.Path, name)
		entry := &api.ManifestEntry{
			Path:        path,
			ContentType: part.Header.Get("Content-Type"),
			Size:        size,
		}
		log.Debug("adding path to new manifest", "ruid", ruid, "bytes", entry.Size, "path", entry.Path)
		contentKey, err := mw.AddEntry(r.Context(), reader, entry)
		if err != nil {
			return fmt.Errorf("error adding manifest entry from multipart form: %s", err)
		}
		log.Debug("stored content", "ruid", ruid, "key", contentKey)
	}
}

func (s *Server) handleDirectUpload(r *http.Request, mw *api.ManifestWriter) error {
	ruid := GetRUID(r.Context())
	log.Debug("handle.direct.upload", "ruid", ruid)
	key, err := mw.AddEntry(r.Context(), r.Body, &api.ManifestEntry{
		Path:        GetURI(r.Context()).Path,
		ContentType: r.Header.Get("Content-Type"),
		Mode:        0644,
		Size:        r.ContentLength,
	})
	if err != nil {
		return err
	}
	log.Debug("stored content", "ruid", ruid, "key", key)
	return nil
}

//handleDelete处理对bzz:/<manifest>/<path>的删除请求，删除
//<path>from<manifest>并返回结果清单哈希作为
//文本/纯响应
func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	ruid := GetRUID(r.Context())
	uri := GetURI(r.Context())
	log.Debug("handle.delete", "ruid", ruid)
	deleteCount.Inc(1)
	newKey, err := s.api.Delete(r.Context(), uri.Addr, uri.Path)
	if err != nil {
		deleteFail.Inc(1)
		respondError(w, r, fmt.Sprintf("could not delete from manifest: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, newKey)
}

//处理源清单创建和源更新
//post请求允许feeds包中定义的JSON结构：`feed.updateRequestJSON`
//请求可以是a）创建源清单，b）更新源，或c）a+b:创建源清单并发布第一个更新
func (s *Server) HandlePostFeed(w http.ResponseWriter, r *http.Request) {
	ruid := GetRUID(r.Context())
	uri := GetURI(r.Context())
	log.Debug("handle.post.feed", "ruid", ruid)
	var err error

//创建和更新必须发送feed.updateRequestJSON JSON结构
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		respondError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

	fd, err := s.api.ResolveFeed(r.Context(), uri, r.URL.Query())
if err != nil { //无法分析查询字符串或检索清单
		getFail.Inc(1)
		httpStatus := http.StatusBadRequest
		if err == api.ErrCannotLoadFeedManifest || err == api.ErrCannotResolveFeedURI {
			httpStatus = http.StatusNotFound
		}
		respondError(w, r, fmt.Sprintf("cannot retrieve feed from manifest: %s", err), httpStatus)
		return
	}

	var updateRequest feed.Request
	updateRequest.Feed = *fd
	query := r.URL.Query()

if err := updateRequest.FromValues(query, body); err != nil { //解码来自查询参数的请求
		respondError(w, r, err.Error(), http.StatusBadRequest)
		return
	}

	switch {
	case updateRequest.IsUpdate():
//验证签名是否完整，以及签名者是否获得授权。
//更新此源
//请尽早检查，以避免创建源，然后无法设置其第一次更新。
		if err = updateRequest.Verify(); err != nil {
			respondError(w, r, err.Error(), http.StatusForbidden)
			return
		}
		_, err = s.api.FeedsUpdate(r.Context(), &updateRequest)
		if err != nil {
			respondError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		fallthrough
	case query.Get("manifest") == "1":
//我们创建一个清单，以便稍后使用bzz://later检索源更新
//此清单具有特殊的“源类型”清单，并保存
//用于稍后检索源更新的源标识
		m, err := s.api.NewFeedManifest(r.Context(), &updateRequest.Feed)
		if err != nil {
			respondError(w, r, fmt.Sprintf("failed to create feed manifest: %v", err), http.StatusInternalServerError)
			return
		}
//清单的密钥将传回客户端
//客户端可以通过其feed成员直接访问feed
//清单键可以设置为ENS名称的冲突解决程序中的内容
		outdata, err := json.Marshal(m)
		if err != nil {
			respondError(w, r, fmt.Sprintf("failed to create json response: %s", err), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, string(outdata))

		w.Header().Add("Content-type", "application/json")
	default:
		respondError(w, r, "Missing signature in feed update request", http.StatusBadRequest)
	}
}

//handlegetfeed检索swarm feed更新：
//bzz feed://<manifest address or ens name>-获取最新的源更新，给定清单地址
//-或
//直接指定用户+主题（可选）、子主题名称（可选），不带清单：
//BZZ进料：//？user=0x…&topic=0x…&name=subtopic名称
//主题默认为0x000…如果未指定。
//如果未指定名称，则默认为空字符串。
//因此，空名称和主题引用用户的默认提要。
//
//可选参数：
//time=xx-在time之前获取最新更新（以epoch秒为单位）
//hint.time=xx-提示查找算法在该时间前后查找更新
//hint.level=xx-提示查找算法查找此频率级别附近的更新
//meta=1-获取源元数据和状态信息，而不是执行源查询
//注意：meta=1将在不久的将来被弃用
func (s *Server) HandleGetFeed(w http.ResponseWriter, r *http.Request) {
	ruid := GetRUID(r.Context())
	uri := GetURI(r.Context())
	log.Debug("handle.get.feed", "ruid", ruid)
	var err error

	fd, err := s.api.ResolveFeed(r.Context(), uri, r.URL.Query())
if err != nil { //无法分析查询字符串或检索清单
		getFail.Inc(1)
		httpStatus := http.StatusBadRequest
		if err == api.ErrCannotLoadFeedManifest || err == api.ErrCannotResolveFeedURI {
			httpStatus = http.StatusNotFound
		}
		respondError(w, r, fmt.Sprintf("cannot retrieve feed information from manifest: %s", err), httpStatus)
		return
	}

//确定查询是指定了期间和版本，还是指定了元数据查询
	if r.URL.Query().Get("meta") == "1" {
		unsignedUpdateRequest, err := s.api.FeedsNewRequest(r.Context(), fd)
		if err != nil {
			getFail.Inc(1)
			respondError(w, r, fmt.Sprintf("cannot retrieve feed metadata for feed=%s: %s", fd.Hex(), err), http.StatusNotFound)
			return
		}
		rawResponse, err := unsignedUpdateRequest.MarshalJSON()
		if err != nil {
			respondError(w, r, fmt.Sprintf("cannot encode unsigned feed update request: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Add("Content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, string(rawResponse))
		return
	}

	lookupParams := &feed.Query{Feed: *fd}
if err = lookupParams.FromValues(r.URL.Query()); err != nil { //分析期间，版本
		respondError(w, r, fmt.Sprintf("invalid feed update request:%s", err), http.StatusBadRequest)
		return
	}

	data, err := s.api.FeedsLookup(r.Context(), lookupParams)

//switch语句中的任何错误都将在此处结束
	if err != nil {
		code, err2 := s.translateFeedError(w, r, "feed lookup fail", err)
		respondError(w, r, err2.Error(), code)
		return
	}

//一切正常，提供检索到的更新
	log.Debug("Found update", "feed", fd.Hex(), "ruid", ruid)
	w.Header().Set("Content-Type", api.MimeOctetStream)
	http.ServeContent(w, r, "", time.Now(), bytes.NewReader(data))
}

func (s *Server) translateFeedError(w http.ResponseWriter, r *http.Request, supErr string, err error) (int, error) {
	code := 0
	defaultErr := fmt.Errorf("%s: %v", supErr, err)
	rsrcErr, ok := err.(*feed.Error)
	if !ok && rsrcErr != nil {
		code = rsrcErr.Code()
	}
	switch code {
	case storage.ErrInvalidValue:
		return http.StatusBadRequest, defaultErr
	case storage.ErrNotFound, storage.ErrNotSynced, storage.ErrNothingToReturn, storage.ErrInit:
		return http.StatusNotFound, defaultErr
	case storage.ErrUnauthorized, storage.ErrInvalidSignature:
		return http.StatusUnauthorized, defaultErr
	case storage.ErrDataOverflow:
		return http.StatusRequestEntityTooLarge, defaultErr
	}

	return http.StatusInternalServerError, defaultErr
}

//handleget处理get请求
//-bzz raw://<key>并用存储在
//给定的存储密钥
//-bzz hash://<key>并用存储内容的哈希进行响应
//在给定的存储键上作为文本/纯响应
func (s *Server) HandleGet(w http.ResponseWriter, r *http.Request) {
	ruid := GetRUID(r.Context())
	uri := GetURI(r.Context())
	log.Debug("handle.get", "ruid", ruid, "uri", uri)
	getCount.Inc(1)
	_, pass, _ := r.BasicAuth()

	addr, err := s.api.ResolveURI(r.Context(), uri, pass)
	if err != nil {
		getFail.Inc(1)
		respondError(w, r, fmt.Sprintf("cannot resolve %s: %s", uri.Addr, err), http.StatusNotFound)
		return
	}
w.Header().Set("Cache-Control", "max-age=2147483648, immutable") //URL的类型为bzz://<hex key>/path，因此我们确信它是不可变的。

	log.Debug("handle.get: resolved", "ruid", ruid, "key", addr)

//如果设置了path，则将<key>解释为清单并返回
//给定路径的原始条目

	etag := common.Bytes2Hex(addr)
	noneMatchEtag := r.Header.Get("If-None-Match")
w.Header().Set("ETag", fmt.Sprintf("%q", etag)) //将etag设置为manifest key或raw entry key。
	if noneMatchEtag != "" {
		if bytes.Equal(storage.Address(common.Hex2Bytes(noneMatchEtag)), addr) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

//通过检索文件的大小检查根块是否存在
	reader, isEncrypted := s.api.Retrieve(r.Context(), addr)
	if _, err := reader.Size(r.Context(), nil); err != nil {
		getFail.Inc(1)
		respondError(w, r, fmt.Sprintf("root chunk not found %s: %s", addr, err), http.StatusNotFound)
		return
	}

	w.Header().Set("X-Decrypted", fmt.Sprintf("%v", isEncrypted))

	switch {
	case uri.Raw():
//允许请求使用查询覆盖内容类型
//参数
		if typ := r.URL.Query().Get("content_type"); typ != "" {
			w.Header().Set("Content-Type", typ)
		}
		http.ServeContent(w, r, "", time.Now(), reader)
	case uri.Hash():
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, addr)
	}
}

//handleGetList处理对bzz list的get请求：/<manifest>/<path>并返回
//<manifest>中<path>下包含的所有文件的列表，分组为
//使用“/”作为分隔符的常见前缀
func (s *Server) HandleGetList(w http.ResponseWriter, r *http.Request) {
	ruid := GetRUID(r.Context())
	uri := GetURI(r.Context())
	_, credentials, _ := r.BasicAuth()
	log.Debug("handle.get.list", "ruid", ruid, "uri", uri)
	getListCount.Inc(1)

//确保根路径有一个尾随斜杠，以便相对URL工作
	if uri.Path == "" && !strings.HasSuffix(r.URL.Path, "/") {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
		return
	}

	addr, err := s.api.Resolve(r.Context(), uri.Addr)
	if err != nil {
		getListFail.Inc(1)
		respondError(w, r, fmt.Sprintf("cannot resolve %s: %s", uri.Addr, err), http.StatusNotFound)
		return
	}
	log.Debug("handle.get.list: resolved", "ruid", ruid, "key", addr)

	list, err := s.api.GetManifestList(r.Context(), s.api.Decryptor(r.Context(), credentials), addr, uri.Path)
	if err != nil {
		getListFail.Inc(1)
		if isDecryptError(err) {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf("Basic realm=%q", addr.String()))
			respondError(w, r, err.Error(), http.StatusUnauthorized)
			return
		}
		respondError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

//如果客户端需要HTML（例如浏览器），则将列表呈现为
//带相对URL的HTML索引
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		w.Header().Set("Content-Type", "text/html")
		err := TemplatesMap["bzz-list"].Execute(w, &htmlListData{
			URI: &api.URI{
				Scheme: "bzz",
				Addr:   uri.Addr,
				Path:   uri.Path,
			},
			List: &list,
		})
		if err != nil {
			getListFail.Inc(1)
			log.Error(fmt.Sprintf("error rendering list HTML: %s", err))
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&list)
}

//handlegetfile处理对bzz://<manifest>><path>的get请求并响应
//文件内容位于给定的<manifest>
func (s *Server) HandleGetFile(w http.ResponseWriter, r *http.Request) {
	ruid := GetRUID(r.Context())
	uri := GetURI(r.Context())
	_, credentials, _ := r.BasicAuth()
	log.Debug("handle.get.file", "ruid", ruid, "uri", r.RequestURI)
	getFileCount.Inc(1)

//确保根路径有一个尾随斜杠，以便相对URL工作
	if uri.Path == "" && !strings.HasSuffix(r.URL.Path, "/") {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
		return
	}
	var err error
	manifestAddr := uri.Address()

	if manifestAddr == nil {
		manifestAddr, err = s.api.Resolve(r.Context(), uri.Addr)
		if err != nil {
			getFileFail.Inc(1)
			respondError(w, r, fmt.Sprintf("cannot resolve %s: %s", uri.Addr, err), http.StatusNotFound)
			return
		}
	} else {
w.Header().Set("Cache-Control", "max-age=2147483648, immutable") //URL的类型为bzz://<hex key>/path，因此我们确信它是不可变的。
	}

	log.Debug("handle.get.file: resolved", "ruid", ruid, "key", manifestAddr)

	reader, contentType, status, contentKey, err := s.api.Get(r.Context(), s.api.Decryptor(r.Context(), credentials), manifestAddr, uri.Path)

	etag := common.Bytes2Hex(contentKey)
	noneMatchEtag := r.Header.Get("If-None-Match")
w.Header().Set("ETag", fmt.Sprintf("%q", etag)) //将etag设置为实际内容键。
	if noneMatchEtag != "" {
		if bytes.Equal(storage.Address(common.Hex2Bytes(noneMatchEtag)), contentKey) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	if err != nil {
		if isDecryptError(err) {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf("Basic realm=%q", manifestAddr))
			respondError(w, r, err.Error(), http.StatusUnauthorized)
			return
		}

		switch status {
		case http.StatusNotFound:
			getFileNotFound.Inc(1)
			respondError(w, r, err.Error(), http.StatusNotFound)
		default:
			getFileFail.Inc(1)
			respondError(w, r, err.Error(), http.StatusInternalServerError)
		}
		return
	}

//请求导致文件不明确
//例如，清单中提供/readme.md和readinglist.txt。
	if status == http.StatusMultipleChoices {
		list, err := s.api.GetManifestList(r.Context(), s.api.Decryptor(r.Context(), credentials), manifestAddr, uri.Path)
		if err != nil {
			getFileFail.Inc(1)
			if isDecryptError(err) {
				w.Header().Set("WWW-Authenticate", fmt.Sprintf("Basic realm=%q", manifestAddr))
				respondError(w, r, err.Error(), http.StatusUnauthorized)
				return
			}
			respondError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Debug(fmt.Sprintf("Multiple choices! --> %v", list), "ruid", ruid)
//显示指向可用条目的良好页面链接
		ShowMultipleChoices(w, r, list)
		return
	}

//通过检索文件的大小检查根块是否存在
	if _, err := reader.Size(r.Context(), nil); err != nil {
		getFileNotFound.Inc(1)
		respondError(w, r, fmt.Sprintf("file not found %s: %s", uri, err), http.StatusNotFound)
		return
	}

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}

	fileName := uri.Addr
	if found := path.Base(uri.Path); found != "" && found != "." && found != "/" {
		fileName = found
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", fileName))

	http.ServeContent(w, r, fileName, time.Now(), newBufferedReadSeeker(reader, getFileBufferSize))
}

//在Lazychunkreader上用于bufio.reader的缓冲区大小传递给
//handlegetfile中的http.serveContent。
//警告：此值影响块请求和chunker join goroutine的数量
//按文件请求。
//建议值是IO的4倍。复制默认缓冲区值32KB。
const getFileBufferSize = 4 * 32 * 1024

//BufferedReadSeeker将Bufio.Reader包装为公开Seek方法
//来自NewBufferedReadSeeker中提供的IO.readSeeker。
type bufferedReadSeeker struct {
	r io.Reader
	s io.Seeker
}

//NewBufferedReadSeeker创建BufferedReadSeeker的新实例，
//输出IO.readseeker。参数“size”是读取缓冲区的大小。
func newBufferedReadSeeker(readSeeker io.ReadSeeker, size int) bufferedReadSeeker {
	return bufferedReadSeeker{
		r: bufio.NewReaderSize(readSeeker, size),
		s: readSeeker,
	}
}

func (b bufferedReadSeeker) Read(p []byte) (n int, err error) {
	return b.r.Read(p)
}

func (b bufferedReadSeeker) Seek(offset int64, whence int) (int64, error) {
	return b.s.Seek(offset, whence)
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func isDecryptError(err error) bool {
	return strings.Contains(err.Error(), api.ErrDecrypt.Error())
}
