
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

//+不建

/*
此命令在所有源文件的基础上生成GPL许可证头。
您可以每月运行一次，在剪切发布之前或只是
无论什么时候你喜欢。

 运行update-license.go

所有作者（贡献代码的人）都列在
作者档案。使用
邮件映射文件。可以使用.mailmap设置规范名称和
每个作者的地址。有关
.mailmap格式。

请查看结果差异，以检查是否正确
执行版权分配。
**/


package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
)

var (
//只考虑具有这些扩展名的文件
	extensions = []string{".go", ".js", ".qml"}

//将跳过具有任何这些前缀的路径
	skipPrefixes = []string{
//镗孔材料
		"vendor/", "tests/testdata/", "build/",
//不要保留自动贩卖的来源
		"cmd/internal/browser",
		"consensus/ethash/xor.go",
		"crypto/bn256/",
		"crypto/ecies/",
		"crypto/secp256k1/curve.go",
		"crypto/sha3/",
		"internal/jsre/deps",
		"log/",
		"common/bitutil/bitutil",
//不许可生成的文件
		"contracts/chequebook/contract/code.go",
	}

//具有此前缀的路径被授权为gpl。所有其他文件都是lgpl。
	gplPrefixes = []string{"cmd/"}

//此regexp必须与
//每个文件的开头。
licenseCommentRE = regexp.MustCompile(`^//\s*（版权此文件是的一部分）。*？n？//*？\n）*\n*`

//
	authorsFileHeader = "# This is the official list of go-ethereum authors for copyright purposes.\n\n"
)

//此模板生成许可证注释。
//它的输入是一个信息结构。
var licenseT = template.Must(template.New("").Parse(`
//版权.year Go Ethereum作者
//此文件是.whole false的一部分。
//
//.整个真是免费软件：您可以重新发布和/或修改它
//根据GNU许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//。整个真分布的目的是希望它有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU许可证了解更多详细信息。
//
//您应该已经收到一份GNU许可证的副本。
//连同。整个错误。如果没有，请参见<http://www.gnu.org/licenses/>。

`[1:]))

type info struct {
	file string
	Year int64
}

func (i info) License() string {
	if i.gpl() {
		return "General Public License"
	}
	return "Lesser General Public License"
}

func (i info) ShortLicense() string {
	if i.gpl() {
		return "GPL"
	}
	return "LGPL"
}

func (i info) Whole(startOfSentence bool) string {
	if i.gpl() {
		return "go-ethereum"
	}
	if startOfSentence {
		return "The go-ethereum library"
	}
	return "the go-ethereum library"
}

func (i info) gpl() bool {
	for _, p := range gplPrefixes {
		if strings.HasPrefix(i.file, p) {
			return true
		}
	}
	return false
}

func main() {
	var (
		files = getFiles()
		filec = make(chan string)
		infoc = make(chan *info, 20)
		wg    sync.WaitGroup
	)

	writeAuthors(files)

	go func() {
		for _, f := range files {
			filec <- f
		}
		close(filec)
	}()
	for i := runtime.NumCPU(); i >= 0; i-- {
//获取文件信息很慢，需要并行处理。
//它遍历每个文件的git历史记录。
		wg.Add(1)
		go getInfo(filec, infoc, &wg)
	}
	go func() {
		wg.Wait()
		close(infoc)
	}()
	writeLicenses(infoc)
}

func skipFile(path string) bool {
	if strings.Contains(path, "/testdata/") {
		return true
	}
	for _, p := range skipPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func getFiles() []string {
	cmd := exec.Command("git", "ls-tree", "-r", "--name-only", "HEAD")
	var files []string
	err := doLines(cmd, func(line string) {
		if skipFile(line) {
			return
		}
		ext := filepath.Ext(line)
		for _, wantExt := range extensions {
			if ext == wantExt {
				goto keep
			}
		}
		return
	keep:
		files = append(files, line)
	})
	if err != nil {
		log.Fatal("error getting files:", err)
	}
	return files
}

var authorRegexp = regexp.MustCompile(`\s*[0-9]+\s*(.*)`)

func gitAuthors(files []string) []string {
	cmds := []string{"shortlog", "-s", "-n", "-e", "HEAD", "--"}
	cmds = append(cmds, files...)
	cmd := exec.Command("git", cmds...)
	var authors []string
	err := doLines(cmd, func(line string) {
		m := authorRegexp.FindStringSubmatch(line)
		if len(m) > 1 {
			authors = append(authors, m[1])
		}
	})
	if err != nil {
		log.Fatalln("error getting authors:", err)
	}
	return authors
}

func readAuthors() []string {
	content, err := ioutil.ReadFile("AUTHORS")
	if err != nil && !os.IsNotExist(err) {
		log.Fatalln("error reading AUTHORS:", err)
	}
	var authors []string
	for _, a := range bytes.Split(content, []byte("\n")) {
		if len(a) > 0 && a[0] != '#' {
			authors = append(authors, string(a))
		}
	}
//通过.mailmap重新传输现有作者。
//这将捕获电子邮件地址更改。
	authors = mailmapLookup(authors)
	return authors
}

func mailmapLookup(authors []string) []string {
	if len(authors) == 0 {
		return nil
	}
	cmds := []string{"check-mailmap", "--"}
	cmds = append(cmds, authors...)
	cmd := exec.Command("git", cmds...)
	var translated []string
	err := doLines(cmd, func(line string) {
		translated = append(translated, line)
	})
	if err != nil {
		log.Fatalln("error translating authors:", err)
	}
	return translated
}

func writeAuthors(files []string) {
	merge := make(map[string]bool)
//添加Git作为贡献者X报告的作者。
//这是作者信息的主要来源。
	for _, a := range gitAuthors(files) {
		merge[a] = true
	}
//从文件添加现有作者。这应该确保我们
//永远不要失去作者，即使Git不再列出他们。我们也可以
//以这种方式手动添加作者。
	for _, a := range readAuthors() {
		merge[a] = true
	}
//将排序的作者列表写回文件。
	var result []string
	for a := range merge {
		result = append(result, a)
	}
	sort.Strings(result)
	content := new(bytes.Buffer)
	content.WriteString(authorsFileHeader)
	for _, a := range result {
		content.WriteString(a)
		content.WriteString("\n")
	}
	fmt.Println("writing AUTHORS")
	if err := ioutil.WriteFile("AUTHORS", content.Bytes(), 0644); err != nil {
		log.Fatalln(err)
	}
}

func getInfo(files <-chan string, out chan<- *info, wg *sync.WaitGroup) {
	for file := range files {
		stat, err := os.Lstat(file)
		if err != nil {
			fmt.Printf("ERROR %s: %v\n", file, err)
			continue
		}
		if !stat.Mode().IsRegular() {
			continue
		}
		if isGenerated(file) {
			continue
		}
		info, err := fileInfo(file)
		if err != nil {
			fmt.Printf("ERROR %s: %v\n", file, err)
			continue
		}
		out <- info
	}
	wg.Done()
}

func isGenerated(file string) bool {
	fd, err := os.Open(file)
	if err != nil {
		return false
	}
	defer fd.Close()
	buf := make([]byte, 2048)
	n, _ := fd.Read(buf)
	buf = buf[:n]
	for _, l := range bytes.Split(buf, []byte("\n")) {
if bytes.HasPrefix(l, []byte("//代码生成”））；
			return true
		}
	}
	return false
}

//fileinfo查找提交给定文件的最低年份。
func fileInfo(file string) (*info, error) {
	info := &info{file: file, Year: int64(time.Now().Year())}
	cmd := exec.Command("git", "log", "--follow", "--find-renames=80", "--find-copies=80", "--pretty=format:%ai", "--", file)
	err := doLines(cmd, func(line string) {
		y, err := strconv.ParseInt(line[:4], 10, 64)
		if err != nil {
			fmt.Printf("cannot parse year: %q", line[:4])
		}
		if y < info.Year {
			info.Year = y
		}
	})
	return info, err
}

func writeLicenses(infos <-chan *info) {
	for i := range infos {
		writeLicense(i)
	}
}

func writeLicense(info *info) {
	fi, err := os.Stat(info.file)
	if os.IsNotExist(err) {
		fmt.Println("skipping (does not exist)", info.file)
		return
	}
	if err != nil {
		log.Fatalf("error stat'ing %s: %v\n", info.file, err)
	}
	content, err := ioutil.ReadFile(info.file)
	if err != nil {
		log.Fatalf("error reading %s: %v\n", info.file, err)
	}
//构造新的文件内容。
	buf := new(bytes.Buffer)
	licenseT.Execute(buf, info)
	if m := licenseCommentRE.FindIndex(content); m != nil && m[0] == 0 {
		buf.Write(content[:m[0]])
		buf.Write(content[m[1]:])
	} else {
		buf.Write(content)
	}
//将其写入文件。
	if bytes.Equal(content, buf.Bytes()) {
		fmt.Println("skipping (no changes)", info.file)
		return
	}
	fmt.Println("writing", info.ShortLicense(), info.file)
	if err := ioutil.WriteFile(info.file, buf.Bytes(), fi.Mode()); err != nil {
		log.Fatalf("error writing %s: %v", info.file, err)
	}
}

func doLines(cmd *exec.Cmd, f func(string)) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	s := bufio.NewScanner(stdout)
	for s.Scan() {
		f(s.Text())
	}
	if s.Err() != nil {
		return s.Err()
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%v (for %s)", err, strings.Join(cmd.Args, " "))
	}
	return nil
}
