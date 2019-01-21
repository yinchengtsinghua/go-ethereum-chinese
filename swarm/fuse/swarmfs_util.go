
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
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/ethereum/go-ethereum/swarm/log"
)

func externalUnmount(mountPoint string) error {
	ctx, cancel := context.WithTimeout(context.Background(), unmountTimeout)
	defer cancel()

//尝试通用umount。
	if err := exec.CommandContext(ctx, "umount", mountPoint).Run(); err == nil {
		return nil
	}
//如果umount不工作，请尝试使用fuse-specific命令。
	switch runtime.GOOS {
	case "darwin":
		return exec.CommandContext(ctx, "diskutil", "umount", mountPoint).Run()
	case "linux":
		return exec.CommandContext(ctx, "fusermount", "-u", mountPoint).Run()
	default:
		return fmt.Errorf("swarmfs unmount: unimplemented")
	}
}

func addFileToSwarm(sf *SwarmFile, content []byte, size int) error {
	fkey, mhash, err := sf.mountInfo.swarmApi.AddFile(context.TODO(), sf.mountInfo.LatestManifest, sf.path, sf.name, content, true)
	if err != nil {
		return err
	}

	sf.lock.Lock()
	defer sf.lock.Unlock()
	sf.addr = fkey
	sf.fileSize = int64(size)

	sf.mountInfo.lock.Lock()
	defer sf.mountInfo.lock.Unlock()
	sf.mountInfo.LatestManifest = mhash

	log.Info("swarmfs added new file:", "fname", sf.name, "new Manifest hash", mhash)
	return nil
}

func removeFileFromSwarm(sf *SwarmFile) error {
	mkey, err := sf.mountInfo.swarmApi.RemoveFile(context.TODO(), sf.mountInfo.LatestManifest, sf.path, sf.name, true)
	if err != nil {
		return err
	}

	sf.mountInfo.lock.Lock()
	defer sf.mountInfo.lock.Unlock()
	sf.mountInfo.LatestManifest = mkey

	log.Info("swarmfs removed file:", "fname", sf.name, "new Manifest hash", mkey)
	return nil
}

func removeDirectoryFromSwarm(sd *SwarmDir) error {
	if len(sd.directories) == 0 && len(sd.files) == 0 {
		return nil
	}

	for _, d := range sd.directories {
		err := removeDirectoryFromSwarm(d)
		if err != nil {
			return err
		}
	}

	for _, f := range sd.files {
		err := removeFileFromSwarm(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func appendToExistingFileInSwarm(sf *SwarmFile, content []byte, offset int64, length int64) error {
	fkey, mhash, err := sf.mountInfo.swarmApi.AppendFile(context.TODO(), sf.mountInfo.LatestManifest, sf.path, sf.name, sf.fileSize, content, sf.addr, offset, length, true)
	if err != nil {
		return err
	}

	sf.lock.Lock()
	defer sf.lock.Unlock()
	sf.addr = fkey
	sf.fileSize = sf.fileSize + int64(len(content))

	sf.mountInfo.lock.Lock()
	defer sf.mountInfo.lock.Unlock()
	sf.mountInfo.LatestManifest = mhash

	log.Info("swarmfs appended file:", "fname", sf.name, "new Manifest hash", mhash)
	return nil
}
