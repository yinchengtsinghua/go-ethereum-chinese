
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

//+构建Linux Darwin Freebsd

package fuse

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/swarm/api"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/testutil"
	colorable "github.com/mattn/go-colorable"
)

var (
	loglevel    = flag.Int("loglevel", 4, "verbosity of logs")
	rawlog      = flag.Bool("rawlog", false, "turn off terminal formatting in logs")
	longrunning = flag.Bool("longrunning", false, "do run long-running tests")
)

func init() {
	flag.Parse()
	log.PrintOrigins(true)
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(!*rawlog))))
}

type fileInfo struct {
	perm     uint64
	uid      int
	gid      int
	contents []byte
}

//从提供的名称和内容地图创建文件，并通过API将其上传到Swarm
func createTestFilesAndUploadToSwarm(t *testing.T, api *api.API, files map[string]fileInfo, uploadDir string, toEncrypt bool) string {

//迭代地图
	for fname, finfo := range files {
		actualPath := filepath.Join(uploadDir, fname)
		filePath := filepath.Dir(actualPath)

//创建目录
		err := os.MkdirAll(filePath, 0777)
		if err != nil {
			t.Fatalf("Error creating directory '%v' : %v", filePath, err)
		}

//创建文件
		fd, err1 := os.OpenFile(actualPath, os.O_RDWR|os.O_CREATE, os.FileMode(finfo.perm))
		if err1 != nil {
			t.Fatalf("Error creating file %v: %v", actualPath, err1)
		}

//将内容写入文件
		_, err = fd.Write(finfo.contents)
		if err != nil {
			t.Fatalf("Error writing to file '%v' : %v", filePath, err)
		}
  /*
    注意@wholecode:尚不清楚为什么将chown命令添加到测试套件中。
    在个别测试中，有些文件是用不同的权限初始化的，
    导致未检查的Chown错误。
     添加检查后，测试将失败。

    那么为什么要先办理这张支票呢？
    暂时禁用

     错误=fd.chown（finfo.uid，finfo.gid）
     如果犯错！= nIL{
       t.fatalf（“错误chown文件“%v”：%v”，文件路径，错误）
    }
  **/

		err = fd.Chmod(os.FileMode(finfo.perm))
		if err != nil {
			t.Fatalf("Error chmod file '%v' : %v", filePath, err)
		}
		err = fd.Sync()
		if err != nil {
			t.Fatalf("Error sync file '%v' : %v", filePath, err)
		}
		err = fd.Close()
		if err != nil {
			t.Fatalf("Error closing file '%v' : %v", filePath, err)
		}
	}

//将目录上传到swarm并返回hash
	bzzhash, err := Upload(uploadDir, "", api, toEncrypt)
	if err != nil {
		t.Fatalf("Error uploading directory %v: %vm encryption: %v", uploadDir, err, toEncrypt)
	}

	return bzzhash
}

//通过fuse在文件系统上安装一个swarm散列作为目录
func mountDir(t *testing.T, api *api.API, files map[string]fileInfo, bzzHash string, mountDir string) *SwarmFS {
	swarmfs := NewSwarmFS(api)
	_, err := swarmfs.Mount(bzzHash, mountDir)
	if isFUSEUnsupportedError(err) {
		t.Skip("FUSE not supported:", err)
	} else if err != nil {
		t.Fatalf("Error mounting hash %v: %v", bzzHash, err)
	}

//安装了检查目录
	found := false
	mi := swarmfs.Listmounts()
	for _, minfo := range mi {
		minfo.lock.RLock()
		if minfo.MountPoint == mountDir {
			if minfo.StartManifest != bzzHash ||
				minfo.LatestManifest != bzzHash ||
				minfo.fuseConnection == nil {
				minfo.lock.RUnlock()
				t.Fatalf("Error mounting: exp(%s): act(%s)", bzzHash, minfo.StartManifest)
			}
			found = true
		}
		minfo.lock.RUnlock()
	}

//测试列表
	if !found {
		t.Fatalf("Error getting mounts information for %v: %v", mountDir, err)
	}

//检查文件及其属性是否符合预期
	compareGeneratedFileWithFileInMount(t, files, mountDir)

	return swarmfs
}

//检查文件及其属性是否符合预期
func compareGeneratedFileWithFileInMount(t *testing.T, files map[string]fileInfo, mountDir string) {
	err := filepath.Walk(mountDir, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}
		fname := path[len(mountDir)+1:]
		if _, ok := files[fname]; !ok {
			t.Fatalf(" file %v present in mount dir and is not expected", fname)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Error walking dir %v", mountDir)
	}

	for fname, finfo := range files {
		destinationFile := filepath.Join(mountDir, fname)

		dfinfo, err := os.Stat(destinationFile)
		if err != nil {
			t.Fatalf("Destination file %v missing in mount: %v", fname, err)
		}

		if int64(len(finfo.contents)) != dfinfo.Size() {
			t.Fatalf("file %v Size mismatch  source (%v) vs destination(%v)", fname, int64(len(finfo.contents)), dfinfo.Size())
		}

		if dfinfo.Mode().Perm().String() != "-rwx------" {
			t.Fatalf("file %v Permission mismatch source (-rwx------) vs destination(%v)", fname, dfinfo.Mode().Perm())
		}

		fileContents, err := ioutil.ReadFile(filepath.Join(mountDir, fname))
		if err != nil {
			t.Fatalf("Could not readfile %v : %v", fname, err)
		}
		if !bytes.Equal(fileContents, finfo.contents) {
			t.Fatalf("File %v contents mismatch: %v , %v", fname, fileContents, finfo.contents)
		}
//TODO:检查uid和gid
	}
}

//检查所提供内容的已安装文件
func checkFile(t *testing.T, testMountDir, fname string, contents []byte) {
	destinationFile := filepath.Join(testMountDir, fname)
	dfinfo, err1 := os.Stat(destinationFile)
	if err1 != nil {
		t.Fatalf("Could not stat file %v", destinationFile)
	}
	if dfinfo.Size() != int64(len(contents)) {
		t.Fatalf("Mismatch in size  actual(%v) vs expected(%v)", dfinfo.Size(), int64(len(contents)))
	}

	fd, err2 := os.OpenFile(destinationFile, os.O_RDONLY, os.FileMode(0665))
	if err2 != nil {
		t.Fatalf("Could not open file %v", destinationFile)
	}
	newcontent := make([]byte, len(contents))
	_, err := fd.Read(newcontent)
	if err != nil {
		t.Fatalf("Could not read from file %v", err)
	}
	err = fd.Close()
	if err != nil {
		t.Fatalf("Could not close file %v", err)
	}

	if !bytes.Equal(contents, newcontent) {
		t.Fatalf("File content mismatch expected (%v): received (%v) ", contents, newcontent)
	}
}

func isDirEmpty(name string) bool {
	f, err := os.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()

	_, err = f.Readdirnames(1)

	return err == io.EOF
}

type testAPI struct {
	api *api.API
}

type testData struct {
	testDir       string
	testUploadDir string
	testMountDir  string
	bzzHash       string
	files         map[string]fileInfo
	toEncrypt     bool
	swarmfs       *SwarmFS
}

//创建测试的根目录
func (ta *testAPI) initSubtest(name string) (*testData, error) {
	var err error
	d := &testData{}
	d.testDir, err = ioutil.TempDir(os.TempDir(), name)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create test dir: %v", err)
	}
	return d, nil
}

//上载数据和装载目录
func (ta *testAPI) uploadAndMount(dat *testData, t *testing.T) (*testData, error) {
//创建上载目录
	err := os.MkdirAll(dat.testUploadDir, 0777)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create upload dir: %v", err)
	}
//创建安装目录
	err = os.MkdirAll(dat.testMountDir, 0777)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create mount dir: %v", err)
	}
//上传文件
	dat.bzzHash = createTestFilesAndUploadToSwarm(t, ta.api, dat.files, dat.testUploadDir, dat.toEncrypt)
	log.Debug("Created test files and uploaded to Swarm")
//装入目录
	dat.swarmfs = mountDir(t, ta.api, dat.files, dat.bzzHash, dat.testMountDir)
	log.Debug("Mounted swarm fs")
	return dat, nil
}

//向测试目录树中添加目录
func addDir(root string, name string) (string, error) {
	d := filepath.Join(root, name)
	err := os.MkdirAll(d, 0777)
	if err != nil {
		return "", fmt.Errorf("Couldn't create dir inside test dir: %v", err)
	}
	return d, nil
}

func (ta *testAPI) mountListAndUnmountEncrypted(t *testing.T) {
	log.Debug("Starting mountListAndUnmountEncrypted test")
	ta.mountListAndUnmount(t, true)
	log.Debug("Test mountListAndUnmountEncrypted terminated")
}

func (ta *testAPI) mountListAndUnmountNonEncrypted(t *testing.T) {
	log.Debug("Starting mountListAndUnmountNonEncrypted test")
	ta.mountListAndUnmount(t, false)
	log.Debug("Test mountListAndUnmountNonEncrypted terminated")
}

//安装目录卸载，然后检查目录是否为空
func (ta *testAPI) mountListAndUnmount(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("mountListAndUnmount")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "testUploadDir")
	dat.testMountDir = filepath.Join(dat.testDir, "testMountDir")
	dat.files = make(map[string]fileInfo)

	dat.files["1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}
	dat.files["2.txt"] = fileInfo{0711, 333, 444, testutil.RandomBytes(2, 10)}
	dat.files["3.txt"] = fileInfo{0622, 333, 444, testutil.RandomBytes(3, 100)}
	dat.files["4.txt"] = fileInfo{0533, 333, 444, testutil.RandomBytes(4, 1024)}
	dat.files["5.txt"] = fileInfo{0544, 333, 444, testutil.RandomBytes(5, 10)}
	dat.files["6.txt"] = fileInfo{0555, 333, 444, testutil.RandomBytes(6, 10)}
	dat.files["7.txt"] = fileInfo{0666, 333, 444, testutil.RandomBytes(7, 10)}
	dat.files["8.txt"] = fileInfo{0777, 333, 333, testutil.RandomBytes(8, 10)}
	dat.files["11.txt"] = fileInfo{0777, 333, 444, testutil.RandomBytes(9, 10)}
	dat.files["111.txt"] = fileInfo{0777, 333, 444, testutil.RandomBytes(10, 10)}
	dat.files["two/2.txt"] = fileInfo{0777, 333, 444, testutil.RandomBytes(11, 10)}
	dat.files["two/2/2.txt"] = fileInfo{0777, 333, 444, testutil.RandomBytes(12, 10)}
	dat.files["two/2./2.txt"] = fileInfo{0777, 444, 444, testutil.RandomBytes(13, 10)}
	dat.files["twice/2.txt"] = fileInfo{0777, 444, 333, testutil.RandomBytes(14, 200)}
	dat.files["one/two/three/four/five/six/seven/eight/nine/10.txt"] = fileInfo{0777, 333, 444, testutil.RandomBytes(15, 10240)}
	dat.files["one/two/three/four/five/six/six"] = fileInfo{0777, 333, 444, testutil.RandomBytes(16, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()
//检查卸载
	_, err = dat.swarmfs.Unmount(dat.testMountDir)
	if err != nil {
		t.Fatalf("could not unmount  %v", dat.bzzHash)
	}
	log.Debug("Unmount successful")
	if !isDirEmpty(dat.testMountDir) {
		t.Fatalf("unmount didnt work for %v", dat.testMountDir)
	}
	log.Debug("subtest terminated")
}

func (ta *testAPI) maxMountsEncrypted(t *testing.T) {
	log.Debug("Starting maxMountsEncrypted test")
	ta.runMaxMounts(t, true)
	log.Debug("Test maxMountsEncrypted terminated")
}

func (ta *testAPI) maxMountsNonEncrypted(t *testing.T) {
	log.Debug("Starting maxMountsNonEncrypted test")
	ta.runMaxMounts(t, false)
	log.Debug("Test maxMountsNonEncrypted terminated")
}

//装入几个不同的目录，直到达到最大值
func (ta *testAPI) runMaxMounts(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("runMaxMounts")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "max-upload1")
	dat.testMountDir = filepath.Join(dat.testDir, "max-mount1")
	dat.files = make(map[string]fileInfo)
	dat.files["1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

	dat.testUploadDir = filepath.Join(dat.testDir, "max-upload2")
	dat.testMountDir = filepath.Join(dat.testDir, "max-mount2")
	dat.files["2.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}

	dat.testUploadDir = filepath.Join(dat.testDir, "max-upload3")
	dat.testMountDir = filepath.Join(dat.testDir, "max-mount3")
	dat.files["3.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}

	dat.testUploadDir = filepath.Join(dat.testDir, "max-upload4")
	dat.testMountDir = filepath.Join(dat.testDir, "max-mount4")
	dat.files["4.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}

	dat.testUploadDir = filepath.Join(dat.testDir, "max-upload5")
	dat.testMountDir = filepath.Join(dat.testDir, "max-mount5")
	dat.files["5.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}

//现在尝试一个附加的装载，如果由于达到最大装载量而失败
	testUploadDir6 := filepath.Join(dat.testDir, "max-upload6")
	err = os.MkdirAll(testUploadDir6, 0777)
	if err != nil {
		t.Fatalf("Couldn't create upload dir 6: %v", err)
	}
	dat.files["6.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}
	testMountDir6 := filepath.Join(dat.testDir, "max-mount6")
	err = os.MkdirAll(testMountDir6, 0777)
	if err != nil {
		t.Fatalf("Couldn't create mount dir 5: %v", err)
	}
	bzzHash6 := createTestFilesAndUploadToSwarm(t, ta.api, dat.files, testUploadDir6, toEncrypt)
	log.Debug("Created test files and uploaded to swarm with uploadDir6")
	_, err = dat.swarmfs.Mount(bzzHash6, testMountDir6)
	if err == nil {
		t.Fatalf("Expected this mount to fail due to exceeding max number of allowed mounts, but succeeded. %v", bzzHash6)
	}
	log.Debug("Maximum mount reached, additional mount failed. Correct.")
}

func (ta *testAPI) remountEncrypted(t *testing.T) {
	log.Debug("Starting remountEncrypted test")
	ta.remount(t, true)
	log.Debug("Test remountEncrypted terminated")
}
func (ta *testAPI) remountNonEncrypted(t *testing.T) {
	log.Debug("Starting remountNonEncrypted test")
	ta.remount(t, false)
	log.Debug("Test remountNonEncrypted terminated")
}

//第二次重新安装同一哈希和已装入点中的不同哈希的测试
func (ta *testAPI) remount(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("remount")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "remount-upload1")
	dat.testMountDir = filepath.Join(dat.testDir, "remount-mount1")
	dat.files = make(map[string]fileInfo)

	dat.files["1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

//尝试第二次装载相同的哈希
	testMountDir2, err2 := addDir(dat.testDir, "remount-mount2")
	if err2 != nil {
		t.Fatalf("Error creating second mount dir: %v", err2)
	}
	_, err2 = dat.swarmfs.Mount(dat.bzzHash, testMountDir2)
	if err2 != nil {
		t.Fatalf("Error mounting hash second time on different dir  %v", dat.bzzHash)
	}

//在已装入的点中装入另一个哈希
	dat.files["2.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}
	testUploadDir2, err3 := addDir(dat.testDir, "remount-upload2")
	if err3 != nil {
		t.Fatalf("Error creating second upload dir: %v", err3)
	}
	bzzHash2 := createTestFilesAndUploadToSwarm(t, ta.api, dat.files, testUploadDir2, toEncrypt)
	_, err = swarmfs.Mount(bzzHash2, dat.testMountDir)
	if err == nil {
		t.Fatalf("Error mounting hash  %v", bzzHash2)
	}
	log.Debug("Mount on existing mount point failed. Correct.")

//安装不存在的哈希
	failDir, err3 := addDir(dat.testDir, "remount-fail")
	if err3 != nil {
		t.Fatalf("Error creating remount dir: %v", bzzHash2)
	}
	failHash := "0xfea11223344"
	_, err = swarmfs.Mount(failHash, failDir)
	if err == nil {
		t.Fatalf("Expected this mount to fail due to non existing hash. But succeeded %v", failHash)
	}
	log.Debug("Nonexistent hash hasn't been mounted. Correct.")
}

func (ta *testAPI) unmountEncrypted(t *testing.T) {
	log.Debug("Starting unmountEncrypted test")
	ta.unmount(t, true)
	log.Debug("Test unmountEncrypted terminated")
}

func (ta *testAPI) unmountNonEncrypted(t *testing.T) {
	log.Debug("Starting unmountNonEncrypted test")
	ta.unmount(t, false)
	log.Debug("Test unmountNonEncrypted terminated")
}

//安装，然后卸载并检查是否已卸载
func (ta *testAPI) unmount(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("unmount")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "ex-upload1")
	dat.testMountDir = filepath.Join(dat.testDir, "ex-mount1")
	dat.files = make(map[string]fileInfo)
	dat.files["1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

	_, err = dat.swarmfs.Unmount(dat.testMountDir)
	if err != nil {
		t.Fatalf("could not unmount  %v", dat.bzzHash)
	}
	log.Debug("Unmounted Dir")

	mi := swarmfs.Listmounts()
	log.Debug("Going to list mounts")
	for _, minfo := range mi {
		log.Debug("Mount point in list: ", "point", minfo.MountPoint)
		if minfo.MountPoint == dat.testMountDir {
			t.Fatalf("mount state not cleaned up in unmount case %v", dat.testMountDir)
		}
	}
	log.Debug("subtest terminated")
}

func (ta *testAPI) unmountWhenResourceBusyEncrypted(t *testing.T) {
	log.Debug("Starting unmountWhenResourceBusyEncrypted test")
	ta.unmountWhenResourceBusy(t, true)
	log.Debug("Test unmountWhenResourceBusyEncrypted terminated")
}
func (ta *testAPI) unmountWhenResourceBusyNonEncrypted(t *testing.T) {
	log.Debug("Starting unmountWhenResourceBusyNonEncrypted test")
	ta.unmountWhenResourceBusy(t, false)
	log.Debug("Test unmountWhenResourceBusyNonEncrypted terminated")
}

//资源繁忙时卸载；应失败
func (ta *testAPI) unmountWhenResourceBusy(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("unmountWhenResourceBusy")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "ex-upload1")
	dat.testMountDir = filepath.Join(dat.testDir, "ex-mount1")
	dat.files = make(map[string]fileInfo)
	dat.files["1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

//在安装的目录中创建一个文件，然后尝试卸载-应该失败
	actualPath := filepath.Join(dat.testMountDir, "2.txt")
//d，err：=os.openfile（actualpath，os.o_dwr，os.filemode（0700））。
	d, err := os.Create(actualPath)
	if err != nil {
		t.Fatalf("Couldn't create new file: %v", err)
	}
//在安装此测试之前，我们需要手动关闭文件
//但万一出错，我们也推迟吧
	defer d.Close()
	_, err = d.Write(testutil.RandomBytes(1, 10))
	if err != nil {
		t.Fatalf("Couldn't write to file: %v", err)
	}
	log.Debug("Bytes written")

	_, err = dat.swarmfs.Unmount(dat.testMountDir)
	if err == nil {
		t.Fatalf("Expected mount to fail due to resource busy, but it succeeded...")
	}
//免费资源
	err = d.Close()
	if err != nil {
		t.Fatalf("Couldn't close file!  %v", dat.bzzHash)
	}
	log.Debug("File closed")

//现在在显式关闭文件后卸载
	_, err = dat.swarmfs.Unmount(dat.testMountDir)
	if err != nil {
		t.Fatalf("Expected mount to succeed after freeing resource, but it failed: %v", err)
	}
//检查DIR是否仍然安装
	mi := dat.swarmfs.Listmounts()
	log.Debug("Going to list mounts")
	for _, minfo := range mi {
		log.Debug("Mount point in list: ", "point", minfo.MountPoint)
		if minfo.MountPoint == dat.testMountDir {
			t.Fatalf("mount state not cleaned up in unmount case %v", dat.testMountDir)
		}
	}
	log.Debug("subtest terminated")
}

func (ta *testAPI) seekInMultiChunkFileEncrypted(t *testing.T) {
	log.Debug("Starting seekInMultiChunkFileEncrypted test")
	ta.seekInMultiChunkFile(t, true)
	log.Debug("Test seekInMultiChunkFileEncrypted terminated")
}

func (ta *testAPI) seekInMultiChunkFileNonEncrypted(t *testing.T) {
	log.Debug("Starting seekInMultiChunkFileNonEncrypted test")
	ta.seekInMultiChunkFile(t, false)
	log.Debug("Test seekInMultiChunkFileNonEncrypted terminated")
}

//在mounted dir中打开一个文件并转到某个位置
func (ta *testAPI) seekInMultiChunkFile(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("seekInMultiChunkFile")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "seek-upload1")
	dat.testMountDir = filepath.Join(dat.testDir, "seek-mount")
	dat.files = make(map[string]fileInfo)
	dat.files["1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10240)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

//在mounted dir中打开文件并查找第二个块
	actualPath := filepath.Join(dat.testMountDir, "1.txt")
	d, err := os.OpenFile(actualPath, os.O_RDONLY, os.FileMode(0700))
	if err != nil {
		t.Fatalf("Couldn't open file: %v", err)
	}
	log.Debug("Opened file")
	defer func() {
		err := d.Close()
		if err != nil {
			t.Fatalf("Error closing file! %v", err)
		}
	}()

	_, err = d.Seek(5000, 0)
	if err != nil {
		t.Fatalf("Error seeking in file: %v", err)
	}

	contents := make([]byte, 1024)
	_, err = d.Read(contents)
	if err != nil {
		t.Fatalf("Error reading file: %v", err)
	}
	log.Debug("Read contents")
	finfo := dat.files["1.txt"]

	if !bytes.Equal(finfo.contents[:6024][5000:], contents) {
		t.Fatalf("File seek contents mismatch")
	}
	log.Debug("subtest terminated")
}

func (ta *testAPI) createNewFileEncrypted(t *testing.T) {
	log.Debug("Starting createNewFileEncrypted test")
	ta.createNewFile(t, true)
	log.Debug("Test createNewFileEncrypted terminated")
}

func (ta *testAPI) createNewFileNonEncrypted(t *testing.T) {
	log.Debug("Starting createNewFileNonEncrypted test")
	ta.createNewFile(t, false)
	log.Debug("Test createNewFileNonEncrypted terminated")
}

//在已安装的swarm目录中创建新文件，
//卸载fuse dir，然后重新安装以查看新文件是否仍然存在。
func (ta *testAPI) createNewFile(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("createNewFile")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "create-upload1")
	dat.testMountDir = filepath.Join(dat.testDir, "create-mount")
	dat.files = make(map[string]fileInfo)
	dat.files["1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}
	dat.files["five.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(2, 10)}
	dat.files["six.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(3, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

//在根目录中创建一个新文件并检查
	actualPath := filepath.Join(dat.testMountDir, "2.txt")
	d, err1 := os.OpenFile(actualPath, os.O_RDWR|os.O_CREATE, os.FileMode(0665))
	if err1 != nil {
		t.Fatalf("Could not open file %s : %v", actualPath, err1)
	}
	defer d.Close()
	log.Debug("Opened file")
	contents := testutil.RandomBytes(1, 11)
	log.Debug("content read")
	_, err = d.Write(contents)
	if err != nil {
		t.Fatalf("Couldn't write contents: %v", err)
	}
	log.Debug("content written")
	err = d.Close()
	if err != nil {
		t.Fatalf("Couldn't close file: %v", err)
	}
	log.Debug("file closed")

	mi, err2 := dat.swarmfs.Unmount(dat.testMountDir)
	if err2 != nil {
		t.Fatalf("Could not unmount %v", err2)
	}
	log.Debug("Directory unmounted")

	testMountDir2, err3 := addDir(dat.testDir, "create-mount2")
	if err3 != nil {
		t.Fatalf("Error creating mount dir2: %v", err3)
	}
//再上车看看情况是否正常
	dat.files["2.txt"] = fileInfo{0700, 333, 444, contents}
	_ = mountDir(t, ta.api, dat.files, mi.LatestManifest, testMountDir2)
	log.Debug("Directory mounted again")

	checkFile(t, testMountDir2, "2.txt", contents)
	_, err2 = dat.swarmfs.Unmount(testMountDir2)
	if err2 != nil {
		t.Fatalf("Could not unmount %v", err2)
	}
	log.Debug("subtest terminated")
}

func (ta *testAPI) createNewFileInsideDirectoryEncrypted(t *testing.T) {
	log.Debug("Starting createNewFileInsideDirectoryEncrypted test")
	ta.createNewFileInsideDirectory(t, true)
	log.Debug("Test createNewFileInsideDirectoryEncrypted terminated")
}

func (ta *testAPI) createNewFileInsideDirectoryNonEncrypted(t *testing.T) {
	log.Debug("Starting createNewFileInsideDirectoryNonEncrypted test")
	ta.createNewFileInsideDirectory(t, false)
	log.Debug("Test createNewFileInsideDirectoryNonEncrypted terminated")
}

//在装载中的目录内创建新文件
func (ta *testAPI) createNewFileInsideDirectory(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("createNewFileInsideDirectory")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "createinsidedir-upload")
	dat.testMountDir = filepath.Join(dat.testDir, "createinsidedir-mount")
	dat.files = make(map[string]fileInfo)
	dat.files["one/1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

//在现有目录中创建新文件并检查
	dirToCreate := filepath.Join(dat.testMountDir, "one")
	actualPath := filepath.Join(dirToCreate, "2.txt")
	d, err1 := os.OpenFile(actualPath, os.O_RDWR|os.O_CREATE, os.FileMode(0665))
	if err1 != nil {
		t.Fatalf("Could not create file %s : %v", actualPath, err1)
	}
	defer d.Close()
	log.Debug("File opened")
	contents := testutil.RandomBytes(1, 11)
	log.Debug("Content read")
	_, err = d.Write(contents)
	if err != nil {
		t.Fatalf("Error writing random bytes into file %v", err)
	}
	log.Debug("Content written")
	err = d.Close()
	if err != nil {
		t.Fatalf("Error closing file %v", err)
	}
	log.Debug("File closed")

	mi, err2 := dat.swarmfs.Unmount(dat.testMountDir)
	if err2 != nil {
		t.Fatalf("Could not unmount %v", err2)
	}
	log.Debug("Directory unmounted")

	testMountDir2, err3 := addDir(dat.testDir, "createinsidedir-mount2")
	if err3 != nil {
		t.Fatalf("Error creating mount dir2: %v", err3)
	}
//再上车看看情况是否正常
	dat.files["one/2.txt"] = fileInfo{0700, 333, 444, contents}
	_ = mountDir(t, ta.api, dat.files, mi.LatestManifest, testMountDir2)
	log.Debug("Directory mounted again")

	checkFile(t, testMountDir2, "one/2.txt", contents)
	_, err = dat.swarmfs.Unmount(testMountDir2)
	if err != nil {
		t.Fatalf("could not unmount  %v", dat.bzzHash)
	}
	log.Debug("subtest terminated")
}

func (ta *testAPI) createNewFileInsideNewDirectoryEncrypted(t *testing.T) {
	log.Debug("Starting createNewFileInsideNewDirectoryEncrypted test")
	ta.createNewFileInsideNewDirectory(t, true)
	log.Debug("Test createNewFileInsideNewDirectoryEncrypted terminated")
}

func (ta *testAPI) createNewFileInsideNewDirectoryNonEncrypted(t *testing.T) {
	log.Debug("Starting createNewFileInsideNewDirectoryNonEncrypted test")
	ta.createNewFileInsideNewDirectory(t, false)
	log.Debug("Test createNewFileInsideNewDirectoryNonEncrypted terminated")
}

//在mount中创建一个新目录和一个新文件
func (ta *testAPI) createNewFileInsideNewDirectory(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("createNewFileInsideNewDirectory")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "createinsidenewdir-upload")
	dat.testMountDir = filepath.Join(dat.testDir, "createinsidenewdir-mount")
	dat.files = make(map[string]fileInfo)
	dat.files["1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

//在现有目录中创建新文件并检查
	dirToCreate, err2 := addDir(dat.testMountDir, "one")
	if err2 != nil {
		t.Fatalf("Error creating mount dir2: %v", err2)
	}
	actualPath := filepath.Join(dirToCreate, "2.txt")
	d, err1 := os.OpenFile(actualPath, os.O_RDWR|os.O_CREATE, os.FileMode(0665))
	if err1 != nil {
		t.Fatalf("Could not create file %s : %v", actualPath, err1)
	}
	defer d.Close()
	log.Debug("File opened")
	contents := testutil.RandomBytes(1, 11)
	log.Debug("content read")
	_, err = d.Write(contents)
	if err != nil {
		t.Fatalf("Error writing to file: %v", err)
	}
	log.Debug("content written")
	err = d.Close()
	if err != nil {
		t.Fatalf("Error closing file: %v", err)
	}
	log.Debug("File closed")

	mi, err2 := dat.swarmfs.Unmount(dat.testMountDir)
	if err2 != nil {
		t.Fatalf("Could not unmount %v", err2)
	}
	log.Debug("Directory unmounted")

//再上车看看情况是否正常
	dat.files["one/2.txt"] = fileInfo{0700, 333, 444, contents}
	_ = mountDir(t, ta.api, dat.files, mi.LatestManifest, dat.testMountDir)
	log.Debug("Directory mounted again")

	checkFile(t, dat.testMountDir, "one/2.txt", contents)
	_, err2 = dat.swarmfs.Unmount(dat.testMountDir)
	if err2 != nil {
		t.Fatalf("Could not unmount %v", err2)
	}
	log.Debug("subtest terminated")
}

func (ta *testAPI) removeExistingFileEncrypted(t *testing.T) {
	log.Debug("Starting removeExistingFileEncrypted test")
	ta.removeExistingFile(t, true)
	log.Debug("Test removeExistingFileEncrypted terminated")
}

func (ta *testAPI) removeExistingFileNonEncrypted(t *testing.T) {
	log.Debug("Starting removeExistingFileNonEncrypted test")
	ta.removeExistingFile(t, false)
	log.Debug("Test removeExistingFileNonEncrypted terminated")
}

//删除装载中的现有文件
func (ta *testAPI) removeExistingFile(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("removeExistingFile")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "remove-upload")
	dat.testMountDir = filepath.Join(dat.testDir, "remove-mount")
	dat.files = make(map[string]fileInfo)
	dat.files["1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}
	dat.files["five.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(2, 10)}
	dat.files["six.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(3, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

//删除根目录中的文件并检查
	actualPath := filepath.Join(dat.testMountDir, "five.txt")
	err = os.Remove(actualPath)
	if err != nil {
		t.Fatalf("Error removing file! %v", err)
	}
	mi, err2 := dat.swarmfs.Unmount(dat.testMountDir)
	if err2 != nil {
		t.Fatalf("Could not unmount %v", err2)
	}
	log.Debug("Directory unmounted")

//再上车看看情况是否正常
	delete(dat.files, "five.txt")
	_ = mountDir(t, ta.api, dat.files, mi.LatestManifest, dat.testMountDir)
	_, err = os.Stat(actualPath)
	if err == nil {
		t.Fatal("Expected file to not be present in re-mount after removal, but it is there")
	}
	_, err2 = dat.swarmfs.Unmount(dat.testMountDir)
	if err2 != nil {
		t.Fatalf("Could not unmount %v", err2)
	}
	log.Debug("subtest terminated")
}

func (ta *testAPI) removeExistingFileInsideDirEncrypted(t *testing.T) {
	log.Debug("Starting removeExistingFileInsideDirEncrypted test")
	ta.removeExistingFileInsideDir(t, true)
	log.Debug("Test removeExistingFileInsideDirEncrypted terminated")
}

func (ta *testAPI) removeExistingFileInsideDirNonEncrypted(t *testing.T) {
	log.Debug("Starting removeExistingFileInsideDirNonEncrypted test")
	ta.removeExistingFileInsideDir(t, false)
	log.Debug("Test removeExistingFileInsideDirNonEncrypted terminated")
}

//删除装载中目录内的文件
func (ta *testAPI) removeExistingFileInsideDir(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("removeExistingFileInsideDir")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "remove-upload")
	dat.testMountDir = filepath.Join(dat.testDir, "remove-mount")
	dat.files = make(map[string]fileInfo)
	dat.files["1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}
	dat.files["one/five.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(2, 10)}
	dat.files["one/six.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(3, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

//删除根目录中的文件并检查
	actualPath := filepath.Join(dat.testMountDir, "one")
	actualPath = filepath.Join(actualPath, "five.txt")
	err = os.Remove(actualPath)
	if err != nil {
		t.Fatalf("Error removing file! %v", err)
	}
	mi, err2 := dat.swarmfs.Unmount(dat.testMountDir)
	if err2 != nil {
		t.Fatalf("Could not unmount %v", err2)
	}
	log.Debug("Directory unmounted")

//再上车看看情况是否正常
	delete(dat.files, "one/five.txt")
	_ = mountDir(t, ta.api, dat.files, mi.LatestManifest, dat.testMountDir)
	_, err = os.Stat(actualPath)
	if err == nil {
		t.Fatal("Expected file to not be present in re-mount after removal, but it is there")
	}

	okPath := filepath.Join(dat.testMountDir, "one")
	okPath = filepath.Join(okPath, "six.txt")
	_, err = os.Stat(okPath)
	if err != nil {
		t.Fatal("Expected file to be present in re-mount after removal, but it is not there")
	}
	_, err2 = dat.swarmfs.Unmount(dat.testMountDir)
	if err2 != nil {
		t.Fatalf("Could not unmount %v", err2)
	}
	log.Debug("subtest terminated")
}

func (ta *testAPI) removeNewlyAddedFileEncrypted(t *testing.T) {
	log.Debug("Starting removeNewlyAddedFileEncrypted test")
	ta.removeNewlyAddedFile(t, true)
	log.Debug("Test removeNewlyAddedFileEncrypted terminated")
}

func (ta *testAPI) removeNewlyAddedFileNonEncrypted(t *testing.T) {
	log.Debug("Starting removeNewlyAddedFileNonEncrypted test")
	ta.removeNewlyAddedFile(t, false)
	log.Debug("Test removeNewlyAddedFileNonEncrypted terminated")
}

//在mount中添加一个文件，然后将其删除；on remount文件不应在该位置
func (ta *testAPI) removeNewlyAddedFile(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("removeNewlyAddedFile")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "removenew-upload")
	dat.testMountDir = filepath.Join(dat.testDir, "removenew-mount")
	dat.files = make(map[string]fileInfo)
	dat.files["1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}
	dat.files["five.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(2, 10)}
	dat.files["six.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(3, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

//添加新文件并删除它
	dirToCreate := filepath.Join(dat.testMountDir, "one")
	err = os.MkdirAll(dirToCreate, os.FileMode(0665))
	if err != nil {
		t.Fatalf("Error creating dir in mounted dir: %v", err)
	}
	actualPath := filepath.Join(dirToCreate, "2.txt")
	d, err1 := os.OpenFile(actualPath, os.O_RDWR|os.O_CREATE, os.FileMode(0665))
	if err1 != nil {
		t.Fatalf("Could not create file %s : %v", actualPath, err1)
	}
	defer d.Close()
	log.Debug("file opened")
	contents := testutil.RandomBytes(1, 11)
	log.Debug("content read")
	_, err = d.Write(contents)
	if err != nil {
		t.Fatalf("Error writing random bytes to file: %v", err)
	}
	log.Debug("content written")
	err = d.Close()
	if err != nil {
		t.Fatalf("Error closing file: %v", err)
	}
	log.Debug("file closed")

	checkFile(t, dat.testMountDir, "one/2.txt", contents)
	log.Debug("file checked")

	err = os.Remove(actualPath)
	if err != nil {
		t.Fatalf("Error removing file: %v", err)
	}
	log.Debug("file removed")

	mi, err2 := dat.swarmfs.Unmount(dat.testMountDir)
	if err2 != nil {
		t.Fatalf("Could not unmount %v", err2)
	}
	log.Debug("Directory unmounted")

	testMountDir2, err3 := addDir(dat.testDir, "removenew-mount2")
	if err3 != nil {
		t.Fatalf("Error creating mount dir2: %v", err3)
	}
//再上车看看情况是否正常
	_ = mountDir(t, ta.api, dat.files, mi.LatestManifest, testMountDir2)
	log.Debug("Directory mounted again")

	if dat.bzzHash != mi.LatestManifest {
		t.Fatalf("same contents different hash orig(%v): new(%v)", dat.bzzHash, mi.LatestManifest)
	}
	_, err2 = dat.swarmfs.Unmount(testMountDir2)
	if err2 != nil {
		t.Fatalf("Could not unmount %v", err2)
	}
	log.Debug("subtest terminated")
}

func (ta *testAPI) addNewFileAndModifyContentsEncrypted(t *testing.T) {
	log.Debug("Starting addNewFileAndModifyContentsEncrypted test")
	ta.addNewFileAndModifyContents(t, true)
	log.Debug("Test addNewFileAndModifyContentsEncrypted terminated")
}

func (ta *testAPI) addNewFileAndModifyContentsNonEncrypted(t *testing.T) {
	log.Debug("Starting addNewFileAndModifyContentsNonEncrypted test")
	ta.addNewFileAndModifyContents(t, false)
	log.Debug("Test addNewFileAndModifyContentsNonEncrypted terminated")
}

//添加新文件并修改内容；重新安装并检查修改后的文件是否完整
func (ta *testAPI) addNewFileAndModifyContents(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("addNewFileAndModifyContents")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "modifyfile-upload")
	dat.testMountDir = filepath.Join(dat.testDir, "modifyfile-mount")
	dat.files = make(map[string]fileInfo)
	dat.files["1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}
	dat.files["five.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(2, 10)}
	dat.files["six.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(3, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

//在根目录中创建新文件
	actualPath := filepath.Join(dat.testMountDir, "2.txt")
	d, err1 := os.OpenFile(actualPath, os.O_RDWR|os.O_CREATE, os.FileMode(0665))
	if err1 != nil {
		t.Fatalf("Could not create file %s : %v", actualPath, err1)
	}
	defer d.Close()
//将一些随机数据写入文件
	log.Debug("file opened")
	line1 := []byte("Line 1")
	_, err = rand.Read(line1)
	if err != nil {
		t.Fatalf("Error writing random bytes to byte array: %v", err)
	}
	log.Debug("line read")
	_, err = d.Write(line1)
	if err != nil {
		t.Fatalf("Error writing random bytes to file: %v", err)
	}
	log.Debug("line written")
	err = d.Close()
	if err != nil {
		t.Fatalf("Error closing file: %v", err)
	}
	log.Debug("file closed")

//在安装的目录上卸载哈希
	mi1, err2 := dat.swarmfs.Unmount(dat.testMountDir)
	if err2 != nil {
		t.Fatalf("Could not unmount %v", err2)
	}
	log.Debug("Directory unmounted")

//挂载到另一个目录以查看修改后的文件是否正确。
	testMountDir2, err3 := addDir(dat.testDir, "modifyfile-mount2")
	if err3 != nil {
		t.Fatalf("Error creating mount dir2: %v", err3)
	}
	dat.files["2.txt"] = fileInfo{0700, 333, 444, line1}
	_ = mountDir(t, ta.api, dat.files, mi1.LatestManifest, testMountDir2)
	log.Debug("Directory mounted again")

	checkFile(t, testMountDir2, "2.txt", line1)
	log.Debug("file checked")

//卸载第二个目录
	mi2, err4 := dat.swarmfs.Unmount(testMountDir2)
	if err4 != nil {
		t.Fatalf("Could not unmount %v", err4)
	}
	log.Debug("Directory unmounted again")

//在原始目录上重新装载并修改文件
//让我们先清理安装的目录：删除…
	err = os.RemoveAll(dat.testMountDir)
	if err != nil {
		t.Fatalf("Error cleaning up mount dir: %v", err)
	}
//…并重新创建
	err = os.MkdirAll(dat.testMountDir, 0777)
	if err != nil {
		t.Fatalf("Error re-creating mount dir: %v", err)
	}
//现在重新安装
	_ = mountDir(t, ta.api, dat.files, mi2.LatestManifest, dat.testMountDir)
	log.Debug("Directory mounted yet again")

//打开文件….
	fd, err5 := os.OpenFile(actualPath, os.O_RDWR|os.O_APPEND, os.FileMode(0665))
	if err5 != nil {
		t.Fatalf("Could not create file %s : %v", actualPath, err5)
	}
	defer fd.Close()
	log.Debug("file opened")
//…修改一些东西
	line2 := []byte("Line 2")
	_, err = rand.Read(line2)
	if err != nil {
		t.Fatalf("Error modifying random bytes to byte array: %v", err)
	}
	log.Debug("line read")
	_, err = fd.Seek(int64(len(line1)), 0)
	if err != nil {
		t.Fatalf("Error seeking position for modification: %v", err)
	}
	_, err = fd.Write(line2)
	if err != nil {
		t.Fatalf("Error modifying file: %v", err)
	}
	log.Debug("line written")
	err = fd.Close()
	if err != nil {
		t.Fatalf("Error closing modified file; %v", err)
	}
	log.Debug("file closed")

//卸载已修改的目录
	mi3, err6 := dat.swarmfs.Unmount(dat.testMountDir)
	if err6 != nil {
		t.Fatalf("Could not unmount %v", err6)
	}
	log.Debug("Directory unmounted yet again")

//现在重新安装到另一个目录，并检查修改后的文件是否正常。
	testMountDir4, err7 := addDir(dat.testDir, "modifyfile-mount4")
	if err7 != nil {
		t.Fatalf("Could not unmount %v", err7)
	}
	b := [][]byte{line1, line2}
	line1and2 := bytes.Join(b, []byte(""))
	dat.files["2.txt"] = fileInfo{0700, 333, 444, line1and2}
	_ = mountDir(t, ta.api, dat.files, mi3.LatestManifest, testMountDir4)
	log.Debug("Directory mounted final time")

	checkFile(t, testMountDir4, "2.txt", line1and2)
	_, err = dat.swarmfs.Unmount(testMountDir4)
	if err != nil {
		t.Fatalf("Could not unmount %v", err)
	}
	log.Debug("subtest terminated")
}

func (ta *testAPI) removeEmptyDirEncrypted(t *testing.T) {
	log.Debug("Starting removeEmptyDirEncrypted test")
	ta.removeEmptyDir(t, true)
	log.Debug("Test removeEmptyDirEncrypted terminated")
}

func (ta *testAPI) removeEmptyDirNonEncrypted(t *testing.T) {
	log.Debug("Starting removeEmptyDirNonEncrypted test")
	ta.removeEmptyDir(t, false)
	log.Debug("Test removeEmptyDirNonEncrypted terminated")
}

//移除一个空的dir inside mount
func (ta *testAPI) removeEmptyDir(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("removeEmptyDir")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "rmdir-upload")
	dat.testMountDir = filepath.Join(dat.testDir, "rmdir-mount")
	dat.files = make(map[string]fileInfo)
	dat.files["1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}
	dat.files["five.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(2, 10)}
	dat.files["six.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(3, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

	_, err2 := addDir(dat.testMountDir, "newdir")
	if err2 != nil {
		t.Fatalf("Could not unmount %v", err2)
	}
	mi, err := dat.swarmfs.Unmount(dat.testMountDir)
	if err != nil {
		t.Fatalf("Could not unmount %v", err)
	}
	log.Debug("Directory unmounted")
//只要添加一个空目录，哈希就不会改变；测试这个
	if dat.bzzHash != mi.LatestManifest {
		t.Fatalf("same contents different hash orig(%v): new(%v)", dat.bzzHash, mi.LatestManifest)
	}
	log.Debug("subtest terminated")
}

func (ta *testAPI) removeDirWhichHasFilesEncrypted(t *testing.T) {
	log.Debug("Starting removeDirWhichHasFilesEncrypted test")
	ta.removeDirWhichHasFiles(t, true)
	log.Debug("Test removeDirWhichHasFilesEncrypted terminated")
}
func (ta *testAPI) removeDirWhichHasFilesNonEncrypted(t *testing.T) {
	log.Debug("Starting removeDirWhichHasFilesNonEncrypted test")
	ta.removeDirWhichHasFiles(t, false)
	log.Debug("Test removeDirWhichHasFilesNonEncrypted terminated")
}

//删除包含文件的目录；检查重新安装文件是否存在
func (ta *testAPI) removeDirWhichHasFiles(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("removeDirWhichHasFiles")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "rmdir-upload")
	dat.testMountDir = filepath.Join(dat.testDir, "rmdir-mount")
	dat.files = make(map[string]fileInfo)
	dat.files["one/1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}
	dat.files["two/five.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(2, 10)}
	dat.files["two/six.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(3, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

//删除挂载目录中的目录及其所有文件
	dirPath := filepath.Join(dat.testMountDir, "two")
	err = os.RemoveAll(dirPath)
	if err != nil {
		t.Fatalf("Error removing directory in mounted dir: %v", err)
	}

	mi, err2 := dat.swarmfs.Unmount(dat.testMountDir)
	if err2 != nil {
		t.Fatalf("Could not unmount %v ", err2)
	}
	log.Debug("Directory unmounted")

//我们删除了操作系统中的文件，所以让我们也在文件映射中删除它们。
	delete(dat.files, "two/five.txt")
	delete(dat.files, "two/six.txt")

//再次装载并查看删除的文件是否确实已被删除
	testMountDir2, err3 := addDir(dat.testDir, "remount-mount2")
	if err3 != nil {
		t.Fatalf("Could not unmount %v", err3)
	}
	_ = mountDir(t, ta.api, dat.files, mi.LatestManifest, testMountDir2)
	log.Debug("Directory mounted")
	actualPath := filepath.Join(dirPath, "five.txt")
	_, err = os.Stat(actualPath)
	if err == nil {
		t.Fatal("Expected file to not be present in re-mount after removal, but it is there")
	}
	_, err = os.Stat(dirPath)
	if err == nil {
		t.Fatal("Expected file to not be present in re-mount after removal, but it is there")
	}
	_, err = dat.swarmfs.Unmount(testMountDir2)
	if err != nil {
		t.Fatalf("Could not unmount %v", err)
	}
	log.Debug("subtest terminated")
}

func (ta *testAPI) removeDirWhichHasSubDirsEncrypted(t *testing.T) {
	log.Debug("Starting removeDirWhichHasSubDirsEncrypted test")
	ta.removeDirWhichHasSubDirs(t, true)
	log.Debug("Test removeDirWhichHasSubDirsEncrypted terminated")
}

func (ta *testAPI) removeDirWhichHasSubDirsNonEncrypted(t *testing.T) {
	log.Debug("Starting removeDirWhichHasSubDirsNonEncrypted test")
	ta.removeDirWhichHasSubDirs(t, false)
	log.Debug("Test removeDirWhichHasSubDirsNonEncrypted terminated")
}

//删除mount中包含子目录的目录；重新安装时，检查它们是否不存在
func (ta *testAPI) removeDirWhichHasSubDirs(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("removeDirWhichHasSubDirs")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "rmsubdir-upload")
	dat.testMountDir = filepath.Join(dat.testDir, "rmsubdir-mount")
	dat.files = make(map[string]fileInfo)
	dat.files["one/1.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(1, 10)}
	dat.files["two/three/2.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(2, 10)}
	dat.files["two/three/3.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(3, 10)}
	dat.files["two/four/5.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(4, 10)}
	dat.files["two/four/6.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(5, 10)}
	dat.files["two/four/six/7.txt"] = fileInfo{0700, 333, 444, testutil.RandomBytes(6, 10)}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

	dirPath := filepath.Join(dat.testMountDir, "two")
	err = os.RemoveAll(dirPath)
	if err != nil {
		t.Fatalf("Error removing directory in mounted dir: %v", err)
	}

//删除挂载目录中的目录及其所有文件
	mi, err2 := dat.swarmfs.Unmount(dat.testMountDir)
	if err2 != nil {
		t.Fatalf("Could not unmount %v ", err2)
	}
	log.Debug("Directory unmounted")

//我们删除了操作系统中的文件，所以让我们也在文件映射中删除它们。
	delete(dat.files, "two/three/2.txt")
	delete(dat.files, "two/three/3.txt")
	delete(dat.files, "two/four/5.txt")
	delete(dat.files, "two/four/6.txt")
	delete(dat.files, "two/four/six/7.txt")

//再上车看看情况是否正常
	testMountDir2, err3 := addDir(dat.testDir, "remount-mount2")
	if err3 != nil {
		t.Fatalf("Could not unmount %v", err3)
	}
	_ = mountDir(t, ta.api, dat.files, mi.LatestManifest, testMountDir2)
	log.Debug("Directory mounted again")
	actualPath := filepath.Join(dirPath, "three")
	actualPath = filepath.Join(actualPath, "2.txt")
	_, err = os.Stat(actualPath)
	if err == nil {
		t.Fatal("Expected file to not be present in re-mount after removal, but it is there")
	}
	actualPath = filepath.Join(dirPath, "four")
	_, err = os.Stat(actualPath)
	if err == nil {
		t.Fatal("Expected file to not be present in re-mount after removal, but it is there")
	}
	_, err = os.Stat(dirPath)
	if err == nil {
		t.Fatal("Expected file to not be present in re-mount after removal, but it is there")
	}
	_, err = dat.swarmfs.Unmount(testMountDir2)
	if err != nil {
		t.Fatalf("Could not unmount %v", err)
	}
	log.Debug("subtest terminated")
}

func (ta *testAPI) appendFileContentsToEndEncrypted(t *testing.T) {
	log.Debug("Starting appendFileContentsToEndEncrypted test")
	ta.appendFileContentsToEnd(t, true)
	log.Debug("Test appendFileContentsToEndEncrypted terminated")
}

func (ta *testAPI) appendFileContentsToEndNonEncrypted(t *testing.T) {
	log.Debug("Starting appendFileContentsToEndNonEncrypted test")
	ta.appendFileContentsToEnd(t, false)
	log.Debug("Test appendFileContentsToEndNonEncrypted terminated")
}

//将内容追加到文件结尾；重新装载并检查其完整性
func (ta *testAPI) appendFileContentsToEnd(t *testing.T, toEncrypt bool) {
	dat, err := ta.initSubtest("appendFileContentsToEnd")
	if err != nil {
		t.Fatalf("Couldn't initialize subtest dirs: %v", err)
	}
	defer os.RemoveAll(dat.testDir)

	dat.toEncrypt = toEncrypt
	dat.testUploadDir = filepath.Join(dat.testDir, "appendlargefile-upload")
	dat.testMountDir = filepath.Join(dat.testDir, "appendlargefile-mount")
	dat.files = make(map[string]fileInfo)

	line1 := testutil.RandomBytes(1, 10)

	dat.files["1.txt"] = fileInfo{0700, 333, 444, line1}

	dat, err = ta.uploadAndMount(dat, t)
	if err != nil {
		t.Fatalf("Error during upload of files to swarm / mount of swarm dir: %v", err)
	}
	defer dat.swarmfs.Stop()

	actualPath := filepath.Join(dat.testMountDir, "1.txt")
	fd, err4 := os.OpenFile(actualPath, os.O_RDWR|os.O_APPEND, os.FileMode(0665))
	if err4 != nil {
		t.Fatalf("Could not create file %s : %v", actualPath, err4)
	}
	defer fd.Close()
	log.Debug("file opened")
	line2 := testutil.RandomBytes(1, 5)
	log.Debug("line read")
	_, err = fd.Seek(int64(len(line1)), 0)
	if err != nil {
		t.Fatalf("Error searching for position to append: %v", err)
	}
	_, err = fd.Write(line2)
	if err != nil {
		t.Fatalf("Error appending: %v", err)
	}
	log.Debug("line written")
	err = fd.Close()
	if err != nil {
		t.Fatalf("Error closing file: %v", err)
	}
	log.Debug("file closed")

	mi1, err5 := dat.swarmfs.Unmount(dat.testMountDir)
	if err5 != nil {
		t.Fatalf("Could not unmount %v ", err5)
	}
	log.Debug("Directory unmounted")

//再次装载并查看附加文件是否正确
	b := [][]byte{line1, line2}
	line1and2 := bytes.Join(b, []byte(""))
	dat.files["1.txt"] = fileInfo{0700, 333, 444, line1and2}
	testMountDir2, err6 := addDir(dat.testDir, "remount-mount2")
	if err6 != nil {
		t.Fatalf("Could not unmount %v", err6)
	}
	_ = mountDir(t, ta.api, dat.files, mi1.LatestManifest, testMountDir2)
	log.Debug("Directory mounted")

	checkFile(t, testMountDir2, "1.txt", line1and2)

	_, err = dat.swarmfs.Unmount(testMountDir2)
	if err != nil {
		t.Fatalf("Could not unmount %v", err)
	}
	log.Debug("subtest terminated")
}

//运行所有测试
func TestFUSE(t *testing.T) {
	t.Skip("disable fuse tests until they are stable")
//为Swarm创建数据目录
	datadir, err := ioutil.TempDir("", "fuse")
	if err != nil {
		t.Fatalf("unable to create temp dir: %v", err)
	}
	defer os.RemoveAll(datadir)

	fileStore, err := storage.NewLocalFileStore(datadir, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	ta := &testAPI{api: api.NewAPI(fileStore, nil, nil, nil)}

//运行一组简短的测试
//约时间：28秒
	t.Run("mountListAndUnmountEncrypted", ta.mountListAndUnmountEncrypted)
	t.Run("remountEncrypted", ta.remountEncrypted)
	t.Run("unmountWhenResourceBusyNonEncrypted", ta.unmountWhenResourceBusyNonEncrypted)
	t.Run("removeExistingFileEncrypted", ta.removeExistingFileEncrypted)
	t.Run("addNewFileAndModifyContentsNonEncrypted", ta.addNewFileAndModifyContentsNonEncrypted)
	t.Run("removeDirWhichHasFilesNonEncrypted", ta.removeDirWhichHasFilesNonEncrypted)
	t.Run("appendFileContentsToEndEncrypted", ta.appendFileContentsToEndEncrypted)

//提供longrunning标志以执行所有测试
//长时间运行大约时间：140秒
	if *longrunning {
		t.Run("mountListAndUnmountNonEncrypted", ta.mountListAndUnmountNonEncrypted)
		t.Run("maxMountsEncrypted", ta.maxMountsEncrypted)
		t.Run("maxMountsNonEncrypted", ta.maxMountsNonEncrypted)
		t.Run("remountNonEncrypted", ta.remountNonEncrypted)
		t.Run("unmountEncrypted", ta.unmountEncrypted)
		t.Run("unmountNonEncrypted", ta.unmountNonEncrypted)
		t.Run("unmountWhenResourceBusyEncrypted", ta.unmountWhenResourceBusyEncrypted)
		t.Run("unmountWhenResourceBusyNonEncrypted", ta.unmountWhenResourceBusyNonEncrypted)
		t.Run("seekInMultiChunkFileEncrypted", ta.seekInMultiChunkFileEncrypted)
		t.Run("seekInMultiChunkFileNonEncrypted", ta.seekInMultiChunkFileNonEncrypted)
		t.Run("createNewFileEncrypted", ta.createNewFileEncrypted)
		t.Run("createNewFileNonEncrypted", ta.createNewFileNonEncrypted)
		t.Run("createNewFileInsideDirectoryEncrypted", ta.createNewFileInsideDirectoryEncrypted)
		t.Run("createNewFileInsideDirectoryNonEncrypted", ta.createNewFileInsideDirectoryNonEncrypted)
		t.Run("createNewFileInsideNewDirectoryEncrypted", ta.createNewFileInsideNewDirectoryEncrypted)
		t.Run("createNewFileInsideNewDirectoryNonEncrypted", ta.createNewFileInsideNewDirectoryNonEncrypted)
		t.Run("removeExistingFileNonEncrypted", ta.removeExistingFileNonEncrypted)
		t.Run("removeExistingFileInsideDirEncrypted", ta.removeExistingFileInsideDirEncrypted)
		t.Run("removeExistingFileInsideDirNonEncrypted", ta.removeExistingFileInsideDirNonEncrypted)
		t.Run("removeNewlyAddedFileEncrypted", ta.removeNewlyAddedFileEncrypted)
		t.Run("removeNewlyAddedFileNonEncrypted", ta.removeNewlyAddedFileNonEncrypted)
		t.Run("addNewFileAndModifyContentsEncrypted", ta.addNewFileAndModifyContentsEncrypted)
		t.Run("removeEmptyDirEncrypted", ta.removeEmptyDirEncrypted)
		t.Run("removeEmptyDirNonEncrypted", ta.removeEmptyDirNonEncrypted)
		t.Run("removeDirWhichHasFilesEncrypted", ta.removeDirWhichHasFilesEncrypted)
		t.Run("removeDirWhichHasSubDirsEncrypted", ta.removeDirWhichHasSubDirsEncrypted)
		t.Run("removeDirWhichHasSubDirsNonEncrypted", ta.removeDirWhichHasSubDirsNonEncrypted)
		t.Run("appendFileContentsToEndNonEncrypted", ta.appendFileContentsToEndNonEncrypted)
	}
}

func Upload(uploadDir, index string, a *api.API, toEncrypt bool) (hash string, err error) {
	fs := api.NewFileSystem(a)
	hash, err = fs.Upload(uploadDir, index, toEncrypt)
	return hash, err
}
