
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

//+构建Linux Darwin Freebsd

package fuse

import (
	"errors"
	"io"
	"os"
	"sync"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"golang.org/x/net/context"
)

const (
MaxAppendFileSize = 10485760 //10MB
)

var (
	errInvalidOffset           = errors.New("Invalid offset during write")
	errFileSizeMaxLimixReached = errors.New("File size exceeded max limit")
)

var (
	_ fs.Node         = (*SwarmFile)(nil)
	_ fs.HandleReader = (*SwarmFile)(nil)
	_ fs.HandleWriter = (*SwarmFile)(nil)
)

type SwarmFile struct {
	inode    uint64
	name     string
	path     string
	addr     storage.Address
	fileSize int64
	reader   storage.LazySectionReader

	mountInfo *MountInfo
	lock      *sync.RWMutex
}

func NewSwarmFile(path, fname string, minfo *MountInfo) *SwarmFile {
	newFile := &SwarmFile{
		inode:    NewInode(),
		name:     fname,
		path:     path,
		addr:     nil,
fileSize: -1, //-1意味着，文件已经存在于Swarm中，您只需从Swarm中获取大小即可。
		reader:   nil,

		mountInfo: minfo,
		lock:      &sync.RWMutex{},
	}
	return newFile
}

func (sf *SwarmFile) Attr(ctx context.Context, a *fuse.Attr) error {
	log.Debug("swarmfs Attr", "path", sf.path)
	sf.lock.Lock()
	defer sf.lock.Unlock()
	a.Inode = sf.inode
//TODO:需要获取权限作为参数
	a.Mode = 0700
	a.Uid = uint32(os.Getuid())
	a.Gid = uint32(os.Getegid())

	if sf.fileSize == -1 {
		reader, _ := sf.mountInfo.swarmApi.Retrieve(ctx, sf.addr)
		quitC := make(chan bool)
		size, err := reader.Size(ctx, quitC)
		if err != nil {
			log.Error("Couldnt get size of file %s : %v", sf.path, err)
			return err
		}
		sf.fileSize = size
		log.Trace("swarmfs Attr", "size", size)
		close(quitC)
	}
	a.Size = uint64(sf.fileSize)
	return nil
}

func (sf *SwarmFile) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	log.Debug("swarmfs Read", "path", sf.path, "req.String", req.String())
	sf.lock.RLock()
	defer sf.lock.RUnlock()
	if sf.reader == nil {
		sf.reader, _ = sf.mountInfo.swarmApi.Retrieve(ctx, sf.addr)
	}
	buf := make([]byte, req.Size)
	n, err := sf.reader.ReadAt(buf, req.Offset)
	if err == io.ErrUnexpectedEOF || err == io.EOF {
		err = nil
	}
	resp.Data = buf[:n]
	sf.reader = nil

	return err
}

func (sf *SwarmFile) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	log.Debug("swarmfs Write", "path", sf.path, "req.String", req.String())
	if sf.fileSize == 0 && req.Offset == 0 {
//创建新文件
		err := addFileToSwarm(sf, req.Data, len(req.Data))
		if err != nil {
			return err
		}
		resp.Size = len(req.Data)
	} else if req.Offset <= sf.fileSize {
		totalSize := sf.fileSize + int64(len(req.Data))
		if totalSize > MaxAppendFileSize {
			log.Warn("swarmfs Append file size reached (%v) : (%v)", sf.fileSize, len(req.Data))
			return errFileSizeMaxLimixReached
		}

		err := appendToExistingFileInSwarm(sf, req.Data, req.Offset, int64(len(req.Data)))
		if err != nil {
			return err
		}
		resp.Size = len(req.Data)
	} else {
		log.Warn("swarmfs Invalid write request size(%v) : off(%v)", sf.fileSize, req.Offset)
		return errInvalidOffset
	}
	return nil
}
