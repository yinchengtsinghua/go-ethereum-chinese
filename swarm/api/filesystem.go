
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
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

const maxParallelFiles = 5

type FileSystem struct {
	api *API
}

func NewFileSystem(api *API) *FileSystem {
	return &FileSystem{api}
}

//upload将本地目录复制为清单文件并将其上载
//使用文件存储存储
//此函数等待存储块。
//TODO:本地路径应指向清单
//
//已弃用：请改用HTTP API
func (fs *FileSystem) Upload(lpath, index string, toEncrypt bool) (string, error) {
	var list []*manifestTrieEntry
	localpath, err := filepath.Abs(filepath.Clean(lpath))
	if err != nil {
		return "", err
	}

	f, err := os.Open(localpath)
	if err != nil {
		return "", err
	}
	stat, err := f.Stat()
	if err != nil {
		return "", err
	}

	var start int
	if stat.IsDir() {
		start = len(localpath)
		log.Debug(fmt.Sprintf("uploading '%s'", localpath))
		err = filepath.Walk(localpath, func(path string, info os.FileInfo, err error) error {
			if (err == nil) && !info.IsDir() {
				if len(path) <= start {
					return fmt.Errorf("Path is too short")
				}
				if path[:start] != localpath {
					return fmt.Errorf("Path prefix of '%s' does not match localpath '%s'", path, localpath)
				}
				entry := newManifestTrieEntry(&ManifestEntry{Path: filepath.ToSlash(path)}, nil)
				list = append(list, entry)
			}
			return err
		})
		if err != nil {
			return "", err
		}
	} else {
		dir := filepath.Dir(localpath)
		start = len(dir)
		if len(localpath) <= start {
			return "", fmt.Errorf("Path is too short")
		}
		if localpath[:start] != dir {
			return "", fmt.Errorf("Path prefix of '%s' does not match dir '%s'", localpath, dir)
		}
		entry := newManifestTrieEntry(&ManifestEntry{Path: filepath.ToSlash(localpath)}, nil)
		list = append(list, entry)
	}

	errors := make([]error, len(list))
	sem := make(chan bool, maxParallelFiles)
	defer close(sem)

	for i, entry := range list {
		sem <- true
		go func(i int, entry *manifestTrieEntry) {
			defer func() { <-sem }()

			f, err := os.Open(entry.Path)
			if err != nil {
				errors[i] = err
				return
			}
			defer f.Close()

			stat, err := f.Stat()
			if err != nil {
				errors[i] = err
				return
			}

			var hash storage.Address
			var wait func(context.Context) error
			ctx := context.TODO()
			hash, wait, err = fs.api.fileStore.Store(ctx, f, stat.Size(), toEncrypt)
			if err != nil {
				errors[i] = err
				return
			}
			if hash != nil {
				list[i].Hash = hash.Hex()
			}
			if err := wait(ctx); err != nil {
				errors[i] = err
				return
			}

			list[i].ContentType, err = DetectContentType(f.Name(), f)
			if err != nil {
				errors[i] = err
				return
			}

		}(i, entry)
	}
	for i := 0; i < cap(sem); i++ {
		sem <- true
	}

	trie := &manifestTrie{
		fileStore: fs.api.fileStore,
	}
	quitC := make(chan bool)
	for i, entry := range list {
		if errors[i] != nil {
			return "", errors[i]
		}
		entry.Path = RegularSlashes(entry.Path[start:])
		if entry.Path == index {
			ientry := newManifestTrieEntry(&ManifestEntry{
				ContentType: entry.ContentType,
			}, nil)
			ientry.Hash = entry.Hash
			trie.addEntry(ientry, quitC)
		}
		trie.addEntry(entry, quitC)
	}

	err2 := trie.recalcAndStore()
	var hs string
	if err2 == nil {
		hs = trie.ref.Hex()
	}
	return hs, err2
}

//下载复制本地文件系统上的清单basepath结构
//在局部路径下
//
//已弃用：请改用HTTP API
func (fs *FileSystem) Download(bzzpath, localpath string) error {
	lpath, err := filepath.Abs(filepath.Clean(localpath))
	if err != nil {
		return err
	}
	err = os.MkdirAll(lpath, os.ModePerm)
	if err != nil {
		return err
	}

//解析主机和端口
	uri, err := Parse(path.Join("bzz:/", bzzpath))
	if err != nil {
		return err
	}
	addr, err := fs.api.Resolve(context.TODO(), uri.Addr)
	if err != nil {
		return err
	}
	path := uri.Path

	if len(path) > 0 {
		path += "/"
	}

	quitC := make(chan bool)
	trie, err := loadManifest(context.TODO(), fs.api.fileStore, addr, quitC, NOOPDecrypt)
	if err != nil {
		log.Warn(fmt.Sprintf("fs.Download: loadManifestTrie error: %v", err))
		return err
	}

	type downloadListEntry struct {
		addr storage.Address
		path string
	}

	var list []*downloadListEntry
	var mde error

	prevPath := lpath
	err = trie.listWithPrefix(path, quitC, func(entry *manifestTrieEntry, suffix string) {
		log.Trace(fmt.Sprintf("fs.Download: %#v", entry))

		addr = common.Hex2Bytes(entry.Hash)
		path := lpath + "/" + suffix
		dir := filepath.Dir(path)
		if dir != prevPath {
			mde = os.MkdirAll(dir, os.ModePerm)
			prevPath = dir
		}
		if (mde == nil) && (path != dir+"/") {
			list = append(list, &downloadListEntry{addr: addr, path: path})
		}
	})
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	errC := make(chan error)
	done := make(chan bool, maxParallelFiles)
	for i, entry := range list {
		select {
		case done <- true:
			wg.Add(1)
		case <-quitC:
			return fmt.Errorf("aborted")
		}
		go func(i int, entry *downloadListEntry) {
			defer wg.Done()
			err := retrieveToFile(quitC, fs.api.fileStore, entry.addr, entry.path)
			if err != nil {
				select {
				case errC <- err:
				case <-quitC:
				}
				return
			}
			<-done
		}(i, entry)
	}
	go func() {
		wg.Wait()
		close(errC)
	}()
	select {
	case err = <-errC:
		return err
	case <-quitC:
		return fmt.Errorf("aborted")
	}
}

func retrieveToFile(quitC chan bool, fileStore *storage.FileStore, addr storage.Address, path string) error {
f, err := os.Create(path) //TODO:基本路径分隔符
	if err != nil {
		return err
	}
	reader, _ := fileStore.Retrieve(context.TODO(), addr)
	writer := bufio.NewWriter(f)
	size, err := reader.Size(context.TODO(), quitC)
	if err != nil {
		return err
	}
	if _, err = io.CopyN(writer, reader, size); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	return f.Close()
}
