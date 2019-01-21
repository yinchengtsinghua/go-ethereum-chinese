
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
	"os"
	"path/filepath"
	"sync"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/ethereum/go-ethereum/swarm/log"
	"golang.org/x/net/context"
)

var (
	_ fs.Node                = (*SwarmDir)(nil)
	_ fs.NodeRequestLookuper = (*SwarmDir)(nil)
	_ fs.HandleReadDirAller  = (*SwarmDir)(nil)
	_ fs.NodeCreater         = (*SwarmDir)(nil)
	_ fs.NodeRemover         = (*SwarmDir)(nil)
	_ fs.NodeMkdirer         = (*SwarmDir)(nil)
)

type SwarmDir struct {
	inode       uint64
	name        string
	path        string
	directories []*SwarmDir
	files       []*SwarmFile

	mountInfo *MountInfo
	lock      *sync.RWMutex
}

func NewSwarmDir(fullpath string, minfo *MountInfo) *SwarmDir {
	log.Debug("swarmfs", "NewSwarmDir", fullpath)
	newdir := &SwarmDir{
		inode:       NewInode(),
		name:        filepath.Base(fullpath),
		path:        fullpath,
		directories: []*SwarmDir{},
		files:       []*SwarmFile{},
		mountInfo:   minfo,
		lock:        &sync.RWMutex{},
	}
	return newdir
}

func (sd *SwarmDir) Attr(ctx context.Context, a *fuse.Attr) error {
	sd.lock.RLock()
	defer sd.lock.RUnlock()
	a.Inode = sd.inode
	a.Mode = os.ModeDir | 0700
	a.Uid = uint32(os.Getuid())
	a.Gid = uint32(os.Getegid())
	return nil
}

func (sd *SwarmDir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	log.Debug("swarmfs", "Lookup", req.Name)
	for _, n := range sd.files {
		if n.name == req.Name {
			return n, nil
		}
	}
	for _, n := range sd.directories {
		if n.name == req.Name {
			return n, nil
		}
	}
	return nil, fuse.ENOENT
}

func (sd *SwarmDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	log.Debug("swarmfs ReadDirAll")
	var children []fuse.Dirent
	for _, file := range sd.files {
		children = append(children, fuse.Dirent{Inode: file.inode, Type: fuse.DT_File, Name: file.name})
	}
	for _, dir := range sd.directories {
		children = append(children, fuse.Dirent{Inode: dir.inode, Type: fuse.DT_Dir, Name: dir.name})
	}
	return children, nil
}

func (sd *SwarmDir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	log.Debug("swarmfs Create", "path", sd.path, "req.Name", req.Name)

	newFile := NewSwarmFile(sd.path, req.Name, sd.mountInfo)
newFile.fileSize = 0 //0意味着，文件还没有在Swarm中，只是创建了

	sd.lock.Lock()
	defer sd.lock.Unlock()
	sd.files = append(sd.files, newFile)

	return newFile, newFile, nil
}

func (sd *SwarmDir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	log.Debug("swarmfs Remove", "path", sd.path, "req.Name", req.Name)

	if req.Dir && sd.directories != nil {
		newDirs := []*SwarmDir{}
		for _, dir := range sd.directories {
			if dir.name == req.Name {
				removeDirectoryFromSwarm(dir)
			} else {
				newDirs = append(newDirs, dir)
			}
		}
		if len(sd.directories) > len(newDirs) {
			sd.lock.Lock()
			defer sd.lock.Unlock()
			sd.directories = newDirs
		}
		return nil
	} else if !req.Dir && sd.files != nil {
		newFiles := []*SwarmFile{}
		for _, f := range sd.files {
			if f.name == req.Name {
				removeFileFromSwarm(f)
			} else {
				newFiles = append(newFiles, f)
			}
		}
		if len(sd.files) > len(newFiles) {
			sd.lock.Lock()
			defer sd.lock.Unlock()
			sd.files = newFiles
		}
		return nil
	}
	return fuse.ENOENT
}

func (sd *SwarmDir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	log.Debug("swarmfs Mkdir", "path", sd.path, "req.Name", req.Name)
	newDir := NewSwarmDir(filepath.Join(sd.path, req.Name), sd.mountInfo)
	sd.lock.Lock()
	defer sd.lock.Unlock()
	sd.directories = append(sd.directories, newDir)

	return newDir, nil
}
