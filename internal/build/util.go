
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

package build

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

var DryRunFlag = flag.Bool("n", false, "dry run, don't execute commands")

//mustrun执行给定的命令并退出主机进程
//任何错误。
func MustRun(cmd *exec.Cmd) {
	fmt.Println(">>>", strings.Join(cmd.Args, " "))
	if !*DryRunFlag {
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	}
}

func MustRunCommand(cmd string, args ...string) {
	MustRun(exec.Command(cmd, args...))
}

//gopath返回gopath环境
//变量应设置为。
func GOPATH() string {
	if os.Getenv("GOPATH") == "" {
		log.Fatal("GOPATH is not set")
	}
	return os.Getenv("GOPATH")
}

var warnedAboutGit bool

//RunGit runs a git subcommand and returns its output.
//命令必须成功完成。
func RunGit(args ...string) string {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err == exec.ErrNotFound {
		if !warnedAboutGit {
			log.Println("Warning: can't find 'git' in PATH")
			warnedAboutGit = true
		}
		return ""
	} else if err != nil {
		log.Fatal(strings.Join(cmd.Args, " "), ": ", err, "\n", stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

//ReGGITFILE返回.git目录中的文件内容。
func readGitFile(file string) string {
	content, err := ioutil.ReadFile(path.Join(".git", file))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

//渲染将给定的模板文件渲染为输出文件。
func Render(templateFile, outputFile string, outputPerm os.FileMode, x interface{}) {
	tpl := template.Must(template.ParseFiles(templateFile))
	render(tpl, outputFile, outputPerm, x)
}

//renderString将给定的模板字符串呈现为outputfile。
func RenderString(templateContent, outputFile string, outputPerm os.FileMode, x interface{}) {
	tpl := template.Must(template.New("").Parse(templateContent))
	render(tpl, outputFile, outputPerm, x)
}

func render(tpl *template.Template, outputFile string, outputPerm os.FileMode, x interface{}) {
	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		log.Fatal(err)
	}
	out, err := os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY|os.O_EXCL, outputPerm)
	if err != nil {
		log.Fatal(err)
	}
	if err := tpl.Execute(out, x); err != nil {
		log.Fatal(err)
	}
	if err := out.Close(); err != nil {
		log.Fatal(err)
	}
}

//复制文件复制文件。
func CopyFile(dst, src string, mode os.FileMode) {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		log.Fatal(err)
	}
	destFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		log.Fatal(err)
	}
	defer destFile.Close()

	srcFile, err := os.Open(src)
	if err != nil {
		log.Fatal(err)
	}
	defer srcFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		log.Fatal(err)
	}
}

//go tool返回运行go工具的命令。它使用Goroot而不是Path
//因此，由生成执行的go命令使用与运行的“主机”相同的go版本
//编译代码。例如
//
///usr/lib/go-1.11/bin/go运行build/ci.go…
//
//使用Go 1.11运行并从同一个goroot调用Go 1.11工具。这也很重要
//因为主机上的runtime.version检查应该与运行的工具匹配。
func GoTool(tool string, args ...string) *exec.Cmd {
	args = append([]string{tool}, args...)
	return exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), args...)
}

//ExpPkPaseServEndoor扩展了CMD/GO导入路径模式，跳过
//自动售货的包裹。
func ExpandPackagesNoVendor(patterns []string) []string {
	expand := false
	for _, pkg := range patterns {
		if strings.Contains(pkg, "...") {
			expand = true
		}
	}
	if expand {
		cmd := GoTool("list", patterns...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("package listing failed: %v\n%s", err, string(out))
		}
		var packages []string
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.Contains(line, "/vendor/") {
				packages = append(packages, strings.TrimSpace(line))
			}
		}
		return packages
	}
	return patterns
}
