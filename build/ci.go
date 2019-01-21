
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

//+不建

/*
从连续集成脚本调用CI命令。

用法：go run build/ci.go<command>><command flags/arguments>

可用命令包括：

   安装[-architecture][-cc compiler][packages…]——构建包和可执行文件
   测试[-coverage][packages…]——运行测试
   lint—运行某些预先选定的linter
   archive[-architecture][-type zip tar][-signer key envvar][-upload dest]--存档生成的工件
   importkeys—从env导入签名密钥
   debsrc[-signer key id][-upload dest]--创建Debian源包
   nsis--创建Windows nsis安装程序
   aar[-local][-sign key id][-deploy repo][-upload dest]--创建Android存档
   xcode[-local][-sign key id][-deploy repo][-upload dest]--创建iOS xcode框架
   xgo[-alltools][options]--根据选项交叉构建
   purge[-store blobstore][-days threshold]--从blobstore中清除旧存档

对于所有命令，-n防止执行外部程序（干运行模式）。

**/

package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/internal/build"
	"github.com/ethereum/go-ethereum/params"
	sv "github.com/ethereum/go-ethereum/swarm/version"
)

var (
//最后出现在geth*.zip存档中的文件。
	gethArchiveFiles = []string{
		"COPYING",
		executablePath("geth"),
	}

//最后出现在geth alltools*.zip存档中的文件。
	allToolsArchiveFiles = []string{
		"COPYING",
		executablePath("abigen"),
		executablePath("bootnode"),
		executablePath("evm"),
		executablePath("geth"),
		executablePath("puppeth"),
		executablePath("rlpdump"),
		executablePath("wnode"),
	}

//最后出现在swarm.zip文件中的文件。
	swarmArchiveFiles = []string{
		"COPYING",
		executablePath("swarm"),
	}

//为此处列出的所有可执行文件创建Debian包。
	debExecutables = []debExecutable{
		{
			BinaryName:  "abigen",
			Description: "Source code generator to convert Ethereum contract definitions into easy to use, compile-time type-safe Go packages.",
		},
		{
			BinaryName:  "bootnode",
			Description: "Ethereum bootnode.",
		},
		{
			BinaryName:  "evm",
			Description: "Developer utility version of the EVM (Ethereum Virtual Machine) that is capable of running bytecode snippets within a configurable environment and execution mode.",
		},
		{
			BinaryName:  "geth",
			Description: "Ethereum CLI client.",
		},
		{
			BinaryName:  "puppeth",
			Description: "Ethereum private network manager.",
		},
		{
			BinaryName:  "rlpdump",
			Description: "Developer utility tool that prints RLP structures.",
		},
		{
			BinaryName:  "wnode",
			Description: "Ethereum Whisper diagnostic tool",
		},
	}

//为此处列出的所有可执行文件创建Debian包。
	debSwarmExecutables = []debExecutable{
		{
			BinaryName:  "swarm",
			PackageName: "ethereum-swarm",
			Description: "Ethereum Swarm daemon and tools",
		},
	}

	debEthereum = debPackage{
		Name:        "ethereum",
		Version:     params.Version,
		Executables: debExecutables,
	}

	debSwarm = debPackage{
		Name:        "ethereum-swarm",
		Version:     sv.Version,
		Executables: debSwarmExecutables,
	}

//Debian元软件包，用于构建和推送到Ubuntu PPA
	debPackages = []debPackage{
		debSwarm,
		debEthereum,
	}

//要由xgo命令交叉编译的包
	allCrossCompiledArchiveFiles = append(allToolsArchiveFiles, swarmArchiveFiles...)

//为其创建包的发行版。
//注：Vivid不支持，因为它没有Golang-1.6包。
//注意：不支持wily，因为它在lanchpad上被正式否决。
//注意：不支持Yakkety，因为它在Lanchpad上被正式否决。
//注意：不支持Zesty，因为它在LanchPad上被正式否决。
//注意：不支持Artful，因为它在Lanchpad上被正式否决。
	debDistros = []string{"trusty", "xenial", "bionic", "cosmic"}
)

var GOBIN, _ = filepath.Abs(filepath.Join("build", "bin"))

func executablePath(name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(GOBIN, name)
}

func main() {
	log.SetFlags(log.Lshortfile)

	if _, err := os.Stat(filepath.Join("build", "ci.go")); os.IsNotExist(err) {
		log.Fatal("this script must be run from the root of the repository")
	}
	if len(os.Args) < 2 {
		log.Fatal("need subcommand as first argument")
	}
	switch os.Args[1] {
	case "install":
		doInstall(os.Args[2:])
	case "test":
		doTest(os.Args[2:])
	case "lint":
		doLint(os.Args[2:])
	case "archive":
		doArchive(os.Args[2:])
	case "debsrc":
		doDebianSource(os.Args[2:])
	case "nsis":
		doWindowsInstaller(os.Args[2:])
	case "aar":
		doAndroidArchive(os.Args[2:])
	case "xcode":
		doXCodeFramework(os.Args[2:])
	case "xgo":
		doXgo(os.Args[2:])
	case "purge":
		doPurge(os.Args[2:])
	default:
		log.Fatal("unknown command ", os.Args[1])
	}
}

//编译

func doInstall(cmdline []string) {
	var (
		arch = flag.String("arch", "", "Architecture to cross build for")
		cc   = flag.String("cc", "", "C compiler to cross build with")
	)
	flag.CommandLine.Parse(cmdline)
	env := build.Env()

//检查Go版本。人们经常公开关于汇编的问题
//落伍失败。这样可以省去他们的麻烦。
	if !strings.Contains(runtime.Version(), "devel") {
//由于无法进行文本比较，因此请计算次要版本号（1.10<1.9）
		var minor int
		fmt.Sscanf(strings.TrimPrefix(runtime.Version(), "go1."), "%d", &minor)

		if minor < 9 {
			log.Println("You have Go version", runtime.Version())
			log.Println("go-ethereum requires at least Go version 1.9 and cannot")
			log.Println("be compiled with an earlier version. Please upgrade your Go installation.")
			os.Exit(1)
		}
	}
//编译作为参数提供的包，如果没有参数，则编译所有内容。
	packages := []string{"./..."}
	if flag.NArg() > 0 {
		packages = flag.Args()
	}
	packages = build.ExpandPackagesNoVendor(packages)

	if *arch == "" || *arch == runtime.GOARCH {
		goinstall := goTool("install", buildFlags(env)...)
		goinstall.Args = append(goinstall.Args, "-v")
		goinstall.Args = append(goinstall.Args, packages...)
		build.MustRun(goinstall)
		return
	}
//如果我们交叉编译到armv5 armv6或armv7，请清除以前的任何版本。
	if *arch == "arm" {
		os.RemoveAll(filepath.Join(runtime.GOROOT(), "pkg", runtime.GOOS+"_arm"))
		for _, path := range filepath.SplitList(build.GOPATH()) {
			os.RemoveAll(filepath.Join(path, "pkg", runtime.GOOS+"_arm"))
		}
	}
//看来我们是在交叉编译，围绕禁地工作
	goinstall := goToolArch(*arch, *cc, "install", buildFlags(env)...)
	goinstall.Args = append(goinstall.Args, "-v")
	goinstall.Args = append(goinstall.Args, []string{"-buildmode", "archive"}...)
	goinstall.Args = append(goinstall.Args, packages...)
	build.MustRun(goinstall)

	if cmds, err := ioutil.ReadDir("cmd"); err == nil {
		for _, cmd := range cmds {
			pkgs, err := parser.ParseDir(token.NewFileSet(), filepath.Join(".", "cmd", cmd.Name()), nil, parser.PackageClauseOnly)
			if err != nil {
				log.Fatal(err)
			}
			for name := range pkgs {
				if name == "main" {
					gobuild := goToolArch(*arch, *cc, "build", buildFlags(env)...)
					gobuild.Args = append(gobuild.Args, "-v")
					gobuild.Args = append(gobuild.Args, []string{"-o", executablePath(cmd.Name())}...)
					gobuild.Args = append(gobuild.Args, "."+string(filepath.Separator)+filepath.Join("cmd", cmd.Name()))
					build.MustRun(gobuild)
					break
				}
			}
		}
	}
}

func buildFlags(env build.Environment) (flags []string) {
	var ld []string
	if env.Commit != "" {
		ld = append(ld, "-X", "main.gitCommit="+env.Commit)
	}
	if runtime.GOOS == "darwin" {
		ld = append(ld, "-s")
	}

	if len(ld) > 0 {
		flags = append(flags, "-ldflags", strings.Join(ld, " "))
	}
	return flags
}

func goTool(subcmd string, args ...string) *exec.Cmd {
	return goToolArch(runtime.GOARCH, os.Getenv("CC"), subcmd, args...)
}

func goToolArch(arch string, cc string, subcmd string, args ...string) *exec.Cmd {
	cmd := build.GoTool(subcmd, args...)
	cmd.Env = []string{"GOPATH=" + build.GOPATH()}
	if arch == "" || arch == runtime.GOARCH {
		cmd.Env = append(cmd.Env, "GOBIN="+GOBIN)
	} else {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=1")
		cmd.Env = append(cmd.Env, "GOARCH="+arch)
	}
	if cc != "" {
		cmd.Env = append(cmd.Env, "CC="+cc)
	}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GOPATH=") || strings.HasPrefix(e, "GOBIN=") {
			continue
		}
		cmd.Env = append(cmd.Env, e)
	}
	return cmd
}

//运行测试
//
//“测试”还包括静态分析工具，如vet。

func doTest(cmdline []string) {
	coverage := flag.Bool("coverage", false, "Whether to record code coverage")
	flag.CommandLine.Parse(cmdline)
	env := build.Env()

	packages := []string{"./..."}
	if len(flag.CommandLine.Args()) > 0 {
		packages = flag.CommandLine.Args()
	}
	packages = build.ExpandPackagesNoVendor(packages)

//运行实际测试。
//一次测试一个包。CI建设者很慢
//一些测试在负载下会超时。
	gotest := goTool("test", buildFlags(env)...)
	gotest.Args = append(gotest.Args, "-p", "1", "-timeout", "5m")
	if *coverage {
		gotest.Args = append(gotest.Args, "-covermode=atomic", "-cover")
	}

	gotest.Args = append(gotest.Args, packages...)
	build.MustRun(gotest)
}

//在请求的包上运行gometalinter
func doLint(cmdline []string) {
	flag.CommandLine.Parse(cmdline)

	packages := []string{"./..."}
	if len(flag.CommandLine.Args()) > 0 {
		packages = flag.CommandLine.Args()
	}
//获取metallinter并安装所有支持的过梁
	build.MustRun(goTool("get", "gopkg.in/alecthomas/gometalinter.v2"))
	build.MustRunCommand(filepath.Join(GOBIN, "gometalinter.v2"), "--install")

//一起运行快速过梁
	configs := []string{
		"--vendor",
		"--tests",
		"--deadline=2m",
		"--disable-all",
		"--enable=goimports",
		"--enable=varcheck",
		"--enable=vet",
		"--enable=gofmt",
		"--enable=misspell",
		"--enable=goconst",
"--min-occurrences=6", //为GOCOST
	}
	build.MustRunCommand(filepath.Join(GOBIN, "gometalinter.v2"), append(configs, packages...)...)

//一个接一个地运行缓慢的过梁
	for _, linter := range []string{"unconvert", "gosimple"} {
		configs = []string{"--vendor", "--tests", "--deadline=10m", "--disable-all", "--enable=" + linter}
		build.MustRunCommand(filepath.Join(GOBIN, "gometalinter.v2"), append(configs, packages...)...)
	}
}

//释放包装
func doArchive(cmdline []string) {
	var (
		arch   = flag.String("arch", runtime.GOARCH, "Architecture cross packaging")
		atype  = flag.String("type", "zip", "Type of archive to write (zip|tar)")
		signer = flag.String("signer", "", `Environment variable holding the signing key (e.g. LINUX_SIGNING_KEY)`)
		upload = flag.String("upload", "", `Destination to upload the archives (usually "gethstore/builds")`)
		ext    string
	)
	flag.CommandLine.Parse(cmdline)
	switch *atype {
	case "zip":
		ext = ".zip"
	case "tar":
		ext = ".tar.gz"
	default:
		log.Fatal("unknown archive type: ", atype)
	}

	var (
		env = build.Env()

		basegeth = archiveBasename(*arch, params.ArchiveVersion(env.Commit))
		geth     = "geth-" + basegeth + ext
		alltools = "geth-alltools-" + basegeth + ext

		baseswarm = archiveBasename(*arch, sv.ArchiveVersion(env.Commit))
		swarm     = "swarm-" + baseswarm + ext
	)
	maybeSkipArchive(env)
	if err := build.WriteArchive(geth, gethArchiveFiles); err != nil {
		log.Fatal(err)
	}
	if err := build.WriteArchive(alltools, allToolsArchiveFiles); err != nil {
		log.Fatal(err)
	}
	if err := build.WriteArchive(swarm, swarmArchiveFiles); err != nil {
		log.Fatal(err)
	}
	for _, archive := range []string{geth, alltools, swarm} {
		if err := archiveUpload(archive, *upload, *signer); err != nil {
			log.Fatal(err)
		}
	}
}

func archiveBasename(arch string, archiveVersion string) string {
	platform := runtime.GOOS + "-" + arch
	if arch == "arm" {
		platform += os.Getenv("GOARM")
	}
	if arch == "android" {
		platform = "android-all"
	}
	if arch == "ios" {
		platform = "ios-all"
	}
	return platform + "-" + archiveVersion
}

func archiveUpload(archive string, blobstore string, signer string) error {
//如果请求签名，则生成签名文件
	if signer != "" {
		pgpkey, err := base64.StdEncoding.DecodeString(os.Getenv(signer))
		if err != nil {
			return fmt.Errorf("invalid base64 %s", signer)
		}
		if err := build.PGPSignFile(archive, archive+".asc", string(pgpkey)); err != nil {
			return err
		}
	}
//如果请求上载到Azure，则可能使用其签名推送存档
	if blobstore != "" {
		auth := build.AzureBlobstoreConfig{
			Account:   strings.Split(blobstore, "/")[0],
			Token:     os.Getenv("AZURE_BLOBSTORE_TOKEN"),
			Container: strings.SplitN(blobstore, "/", 2)[1],
		}
		if err := build.AzureBlobstoreUpload(archive, filepath.Base(archive), auth); err != nil {
			return err
		}
		if signer != "" {
			if err := build.AzureBlobstoreUpload(archive+".asc", filepath.Base(archive+".asc"), auth); err != nil {
				return err
			}
		}
	}
	return nil
}

//跳过某些生成配置的存档。
func maybeSkipArchive(env build.Environment) {
	if env.IsPullRequest {
		log.Printf("skipping because this is a PR build")
		os.Exit(0)
	}
	if env.IsCronJob {
		log.Printf("skipping because this is a cron job")
		os.Exit(0)
	}
	if env.Branch != "master" && !strings.HasPrefix(env.Tag, "v1.") {
		log.Printf("skipping because branch %q, tag %q is not on the whitelist", env.Branch, env.Tag)
		os.Exit(0)
	}
}

//Debian包装
func doDebianSource(cmdline []string) {
	var (
		signer  = flag.String("signer", "", `Signing key name, also used as package author`)
		upload  = flag.String("upload", "", `Where to upload the source package (usually "ppa:ethereum/ethereum")`)
		workdir = flag.String("workdir", "", `Output directory for packages (uses temp dir if unset)`)
		now     = time.Now()
	)
	flag.CommandLine.Parse(cmdline)
	*workdir = makeWorkdir(*workdir)
	env := build.Env()
	maybeSkipArchive(env)

//导入签名密钥。
	if b64key := os.Getenv("PPA_SIGNING_KEY"); b64key != "" {
		key, err := base64.StdEncoding.DecodeString(b64key)
		if err != nil {
			log.Fatal("invalid base64 PPA_SIGNING_KEY")
		}
		gpg := exec.Command("gpg", "--import")
		gpg.Stdin = bytes.NewReader(key)
		build.MustRun(gpg)
	}

//创建Debian包并上载它们
	for _, pkg := range debPackages {
		for _, distro := range debDistros {
			meta := newDebMetadata(distro, *signer, env, now, pkg.Name, pkg.Version, pkg.Executables)
			pkgdir := stageDebianSource(*workdir, meta)
			debuild := exec.Command("debuild", "-S", "-sa", "-us", "-uc")
			debuild.Dir = pkgdir
			build.MustRun(debuild)

			changes := fmt.Sprintf("%s_%s_source.changes", meta.Name(), meta.VersionString())
			changes = filepath.Join(*workdir, changes)
			if *signer != "" {
				build.MustRunCommand("debsign", changes)
			}
			if *upload != "" {
				build.MustRunCommand("dput", *upload, changes)
			}
		}
	}
}

func makeWorkdir(wdflag string) string {
	var err error
	if wdflag != "" {
		err = os.MkdirAll(wdflag, 0744)
	} else {
		wdflag, err = ioutil.TempDir("", "geth-build-")
	}
	if err != nil {
		log.Fatal(err)
	}
	return wdflag
}

func isUnstableBuild(env build.Environment) bool {
	if env.Tag != "" {
		return false
	}
	return true
}

type debPackage struct {
Name        string          //要生成的Debian包的名称，例如“以太坊”或“以太坊群”
Version     string          //debpackage的干净版本，例如1.8.12或0.3.0，没有任何元数据
Executables []debExecutable //要包含在包中的可执行文件
}

type debMetadata struct {
	Env build.Environment

	PackageName string

//进入正在构建的以太坊版本。注意这个
//不是Debian包版本。包版本
//由versionString构造。
	Version string

Author       string //“name<email>”，同时选择签名密钥
	Distro, Time string
	Executables  []debExecutable
}

type debExecutable struct {
	PackageName string
	BinaryName  string
	Description string
}

//package返回包的名称（如果存在），或者
//回退到BinaryName
func (d debExecutable) Package() string {
	if d.PackageName != "" {
		return d.PackageName
	}
	return d.BinaryName
}

func newDebMetadata(distro, author string, env build.Environment, t time.Time, name string, version string, exes []debExecutable) debMetadata {
	if author == "" {
//没有签名密钥，请使用默认作者。
		author = "Ethereum Builds <fjl@ethereum.org>"
	}
	return debMetadata{
		PackageName: name,
		Env:         env,
		Author:      author,
		Distro:      distro,
		Version:     version,
		Time:        t.Format(time.RFC1123Z),
		Executables: exes,
	}
}

//name返回依赖于
//在所有可执行包上。
func (meta debMetadata) Name() string {
	if isUnstableBuild(meta.Env) {
		return meta.PackageName + "-unstable"
	}
	return meta.PackageName
}

//versionString返回包的Debian版本。
func (meta debMetadata) VersionString() string {
	vsn := meta.Version
	if meta.Env.Buildnum != "" {
		vsn += "+build" + meta.Env.Buildnum
	}
	if meta.Distro != "" {
		vsn += "+" + meta.Distro
	}
	return vsn
}

//exelist返回所有可执行包的列表。
func (meta debMetadata) ExeList() string {
	names := make([]string, len(meta.Executables))
	for i, e := range meta.Executables {
		names[i] = meta.ExeName(e)
	}
	return strings.Join(names, ", ")
}

//exename返回可执行包的包名称。
func (meta debMetadata) ExeName(exe debExecutable) string {
	if isUnstableBuild(meta.Env) {
		return exe.Package() + "-unstable"
	}
	return exe.Package()
}

//execonflicts返回conflicts字段的内容
//对于可执行包。
func (meta debMetadata) ExeConflicts(exe debExecutable) string {
	if isUnstableBuild(meta.Env) {
//设置冲突列表，以便*-不稳定的包
//不能与常规版本一起安装。
//
//https://www.debian.org/doc/debian-policy/ch-relationships.html网站
//对冲突非常明确：并说应该
//首选，冲突文件应通过
//交替。我们最终可能会这样做，但使用冲突是
//现在更容易了。
		return "ethereum, " + exe.Package()
	}
	return ""
}

func stageDebianSource(tmpdir string, meta debMetadata) (pkgdir string) {
	pkg := meta.Name() + "-" + meta.VersionString()
	pkgdir = filepath.Join(tmpdir, pkg)
	if err := os.Mkdir(pkgdir, 0755); err != nil {
		log.Fatal(err)
	}

//复制源代码。
	build.MustRunCommand("git", "checkout-index", "-a", "--prefix", pkgdir+string(filepath.Separator))

//将Debian构建文件放置到位。
	debian := filepath.Join(pkgdir, "debian")
	build.Render("build/deb/"+meta.PackageName+"/deb.rules", filepath.Join(debian, "rules"), 0755, meta)
	build.Render("build/deb/"+meta.PackageName+"/deb.changelog", filepath.Join(debian, "changelog"), 0644, meta)
	build.Render("build/deb/"+meta.PackageName+"/deb.control", filepath.Join(debian, "control"), 0644, meta)
	build.Render("build/deb/"+meta.PackageName+"/deb.copyright", filepath.Join(debian, "copyright"), 0644, meta)
	build.RenderString("8\n", filepath.Join(debian, "compat"), 0644, meta)
	build.RenderString("3.0 (native)\n", filepath.Join(debian, "source/format"), 0644, meta)
	for _, exe := range meta.Executables {
		install := filepath.Join(debian, meta.ExeName(exe)+".install")
		docs := filepath.Join(debian, meta.ExeName(exe)+".docs")
		build.Render("build/deb/"+meta.PackageName+"/deb.install", install, 0644, exe)
		build.Render("build/deb/"+meta.PackageName+"/deb.docs", docs, 0644, exe)
	}

	return pkgdir
}

//Windows安装程序
func doWindowsInstaller(cmdline []string) {
//分析标志并在prs上生成跳过安装程序
	var (
		arch    = flag.String("arch", runtime.GOARCH, "Architecture for cross build packaging")
		signer  = flag.String("signer", "", `Environment variable holding the signing key (e.g. WINDOWS_SIGNING_KEY)`)
		upload  = flag.String("upload", "", `Destination to upload the archives (usually "gethstore/builds")`)
		workdir = flag.String("workdir", "", `Output directory for packages (uses temp dir if unset)`)
	)
	flag.CommandLine.Parse(cmdline)
	*workdir = makeWorkdir(*workdir)
	env := build.Env()
	maybeSkipArchive(env)

//安装程序中包含的聚合二进制文件
	var (
		devTools []string
		allTools []string
		gethTool string
	)
	for _, file := range allToolsArchiveFiles {
if file == "COPYING" { //许可证，稍后复制
			continue
		}
		allTools = append(allTools, filepath.Base(file))
		if filepath.Base(file) == "geth.exe" {
			gethTool = file
		} else {
			devTools = append(devTools, file)
		}
	}

//呈现NSI脚本：安装程序NSI包含两个安装程序部分，
//第一部分包含geth二进制文件，第二部分包含dev工具。
	templateData := map[string]interface{}{
		"License":  "COPYING",
		"Geth":     gethTool,
		"DevTools": devTools,
	}
	build.Render("build/nsis.geth.nsi", filepath.Join(*workdir, "geth.nsi"), 0644, nil)
	build.Render("build/nsis.install.nsh", filepath.Join(*workdir, "install.nsh"), 0644, templateData)
	build.Render("build/nsis.uninstall.nsh", filepath.Join(*workdir, "uninstall.nsh"), 0644, allTools)
	build.Render("build/nsis.pathupdate.nsh", filepath.Join(*workdir, "PathUpdate.nsh"), 0644, nil)
	build.Render("build/nsis.envvarupdate.nsh", filepath.Join(*workdir, "EnvVarUpdate.nsh"), 0644, nil)
	build.CopyFile(filepath.Join(*workdir, "SimpleFC.dll"), "build/nsis.simplefc.dll", 0755)
	build.CopyFile(filepath.Join(*workdir, "COPYING"), "COPYING", 0755)

//构建安装程序。这假设所有需要的文件以前
//构建（不要混合构建和打包以保持交叉编译的复杂性
//最低限度）。
	version := strings.Split(params.Version, ".")
	if env.Commit != "" {
		version[2] += "-" + env.Commit[:8]
	}
	installer, _ := filepath.Abs("geth-" + archiveBasename(*arch, params.ArchiveVersion(env.Commit)) + ".exe")
	build.MustRunCommand("makensis.exe",
		"/DOUTPUTFILE="+installer,
		"/DMAJORVERSION="+version[0],
		"/DMINORVERSION="+version[1],
		"/DBUILDVERSION="+version[2],
		"/DARCH="+*arch,
		filepath.Join(*workdir, "geth.nsi"),
	)

//签署并发布安装程序。
	if err := archiveUpload(installer, *upload, *signer); err != nil {
		log.Fatal(err)
	}
}

//Android档案

func doAndroidArchive(cmdline []string) {
	var (
		local  = flag.Bool("local", false, `Flag whether we're only doing a local build (skip Maven artifacts)`)
		signer = flag.String("signer", "", `Environment variable holding the signing key (e.g. ANDROID_SIGNING_KEY)`)
deploy = flag.String("deploy", "", `Destination to deploy the archive (usually "https://oss.sonatype.org”）`）
		upload = flag.String("upload", "", `Destination to upload the archive (usually "gethstore/builds")`)
	)
	flag.CommandLine.Parse(cmdline)
	env := build.Env()

//健全性检查sdk和ndk是否已安装和设置
	if os.Getenv("ANDROID_HOME") == "" {
		log.Fatal("Please ensure ANDROID_HOME points to your Android SDK")
	}
	if os.Getenv("ANDROID_NDK") == "" {
		log.Fatal("Please ensure ANDROID_NDK points to your Android NDK")
	}
//构建Android存档和Maven资源
	build.MustRun(goTool("get", "golang.org/x/mobile/cmd/gomobile", "golang.org/x/mobile/cmd/gobind"))
	build.MustRun(gomobileTool("init", "--ndk", os.Getenv("ANDROID_NDK")))
	build.MustRun(gomobileTool("bind", "-ldflags", "-s -w", "--target", "android", "--javapkg", "org.ethereum", "-v", "github.com/ethereum/go-ethereum/mobile"))

	if *local {
//如果我们在本地构建，请将bundle复制到build dir并跳过maven
		os.Rename("geth.aar", filepath.Join(GOBIN, "geth.aar"))
		return
	}
	meta := newMavenMetadata(env)
	build.Render("build/mvn.pom", meta.Package+".pom", 0755, meta)

//跳过Maven Deploy和Azure上载以进行pr生成
	maybeSkipArchive(env)

//签名并将存档上载到Azure
	archive := "geth-" + archiveBasename("android", params.ArchiveVersion(env.Commit)) + ".aar"
	os.Rename("geth.aar", archive)

	if err := archiveUpload(archive, *upload, *signer); err != nil {
		log.Fatal(err)
	}
//签名并将所有工件上传到Maven Central
	os.Rename(archive, meta.Package+".aar")
	if *signer != "" && *deploy != "" {
//将签名密钥导入本地GPG实例
		b64key := os.Getenv(*signer)
		key, err := base64.StdEncoding.DecodeString(b64key)
		if err != nil {
			log.Fatalf("invalid base64 %s", *signer)
		}
		gpg := exec.Command("gpg", "--import")
		gpg.Stdin = bytes.NewReader(key)
		build.MustRun(gpg)

		keyID, err := build.PGPKeyID(string(key))
		if err != nil {
			log.Fatal(err)
		}
//将工件上传到Sonatype和/或Maven Central
		repo := *deploy + "/service/local/staging/deploy/maven2"
		if meta.Develop {
			repo = *deploy + "/content/repositories/snapshots"
		}
		build.MustRunCommand("mvn", "gpg:sign-and-deploy-file", "-e", "-X",
			"-settings=build/mvn.settings", "-Durl="+repo, "-DrepositoryId=ossrh",
			"-Dgpg.keyname="+keyID,
			"-DpomFile="+meta.Package+".pom", "-Dfile="+meta.Package+".aar")
	}
}

func gomobileTool(subcmd string, args ...string) *exec.Cmd {
	cmd := exec.Command(filepath.Join(GOBIN, "gomobile"), subcmd)
	cmd.Args = append(cmd.Args, args...)
	cmd.Env = []string{
		"GOPATH=" + build.GOPATH(),
		"PATH=" + GOBIN + string(os.PathListSeparator) + os.Getenv("PATH"),
	}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GOPATH=") || strings.HasPrefix(e, "PATH=") {
			continue
		}
		cmd.Env = append(cmd.Env, e)
	}
	return cmd
}

type mavenMetadata struct {
	Version      string
	Package      string
	Develop      bool
	Contributors []mavenContributor
}

type mavenContributor struct {
	Name  string
	Email string
}

func newMavenMetadata(env build.Environment) mavenMetadata {
//从repo根目录收集作者列表
	contribs := []mavenContributor{}
	if authors, err := os.Open("AUTHORS"); err == nil {
		defer authors.Close()

		scanner := bufio.NewScanner(authors)
		for scanner.Scan() {
//跳过作者列表中的任何空白
			line := strings.TrimSpace(scanner.Text())
			if line == "" || line[0] == '#' {
				continue
			}
//拆分作者并插入为参与者
			re := regexp.MustCompile("([^<]+) <(.+)>")
			parts := re.FindStringSubmatch(line)
			if len(parts) == 3 {
				contribs = append(contribs, mavenContributor{Name: parts[1], Email: parts[2]})
			}
		}
	}
//呈现版本和包字符串
	version := params.Version
	if isUnstableBuild(env) {
		version += "-SNAPSHOT"
	}
	return mavenMetadata{
		Version:      version,
		Package:      "geth-" + version,
		Develop:      isUnstableBuild(env),
		Contributors: contribs,
	}
}

//Xcode框架

func doXCodeFramework(cmdline []string) {
	var (
		local  = flag.Bool("local", false, `Flag whether we're only doing a local build (skip Maven artifacts)`)
		signer = flag.String("signer", "", `Environment variable holding the signing key (e.g. IOS_SIGNING_KEY)`)
		deploy = flag.String("deploy", "", `Destination to deploy the archive (usually "trunk")`)
		upload = flag.String("upload", "", `Destination to upload the archives (usually "gethstore/builds")`)
	)
	flag.CommandLine.Parse(cmdline)
	env := build.Env()

//构建iOS Xcode框架
	build.MustRun(goTool("get", "golang.org/x/mobile/cmd/gomobile", "golang.org/x/mobile/cmd/gobind"))
	build.MustRun(gomobileTool("init"))
	bind := gomobileTool("bind", "-ldflags", "-s -w", "--target", "ios", "--tags", "ios", "-v", "github.com/ethereum/go-ethereum/mobile")

	if *local {
//如果我们在本地生成，请使用build文件夹，然后停止
		bind.Dir, _ = filepath.Abs(GOBIN)
		build.MustRun(bind)
		return
	}
	archive := "geth-" + archiveBasename("ios", params.ArchiveVersion(env.Commit))
	if err := os.Mkdir(archive, os.ModePerm); err != nil {
		log.Fatal(err)
	}
	bind.Dir, _ = filepath.Abs(archive)
	build.MustRun(bind)
	build.MustRunCommand("tar", "-zcvf", archive+".tar.gz", archive)

//跳过cocoapods deploy和azure upload for pr builds
	maybeSkipArchive(env)

//签名并将框架上载到Azure
	if err := archiveUpload(archive+".tar.gz", *upload, *signer); err != nil {
		log.Fatal(err)
	}
//准备并上传一个podspec到cocoapods
	if *deploy != "" {
		meta := newPodMetadata(env, archive)
		build.Render("build/pod.podspec", "Geth.podspec", 0755, meta)
		build.MustRunCommand("pod", *deploy, "push", "Geth.podspec", "--allow-warnings", "--verbose")
	}
}

type podMetadata struct {
	Version      string
	Commit       string
	Archive      string
	Contributors []podContributor
}

type podContributor struct {
	Name  string
	Email string
}

func newPodMetadata(env build.Environment, archive string) podMetadata {
//从repo根目录收集作者列表
	contribs := []podContributor{}
	if authors, err := os.Open("AUTHORS"); err == nil {
		defer authors.Close()

		scanner := bufio.NewScanner(authors)
		for scanner.Scan() {
//跳过作者列表中的任何空白
			line := strings.TrimSpace(scanner.Text())
			if line == "" || line[0] == '#' {
				continue
			}
//拆分作者并插入为参与者
			re := regexp.MustCompile("([^<]+) <(.+)>")
			parts := re.FindStringSubmatch(line)
			if len(parts) == 3 {
				contribs = append(contribs, podContributor{Name: parts[1], Email: parts[2]})
			}
		}
	}
	version := params.Version
	if isUnstableBuild(env) {
		version += "-unstable." + env.Buildnum
	}
	return podMetadata{
		Archive:      archive,
		Version:      version,
		Commit:       env.Commit,
		Contributors: contribs,
	}
}

//交叉汇编

func doXgo(cmdline []string) {
	var (
		alltools = flag.Bool("alltools", false, `Flag whether we're building all known tools, or only on in particular`)
	)
	flag.CommandLine.Parse(cmdline)
	env := build.Env()

//确保xgo可用于交叉编译
	gogetxgo := goTool("get", "github.com/karalabe/xgo")
	build.MustRun(gogetxgo)

//如果请求所有工具生成，则生成生成器需要的所有内容
	args := append(buildFlags(env), flag.Args()...)

	if *alltools {
		args = append(args, []string{"--dest", GOBIN}...)
		for _, res := range allCrossCompiledArchiveFiles {
			if strings.HasPrefix(res, GOBIN) {
//找到二进制工具，显式地交叉构建它
				args = append(args, "./"+filepath.Join("cmd", filepath.Base(res)))
				xgo := xgoTool(args)
				build.MustRun(xgo)
				args = args[:len(args)-1]
			}
		}
		return
	}
//否则，执行显式交叉编译
	path := args[len(args)-1]
	args = append(args[:len(args)-1], []string{"--dest", GOBIN, path}...)

	xgo := xgoTool(args)
	build.MustRun(xgo)
}

func xgoTool(args []string) *exec.Cmd {
	cmd := exec.Command(filepath.Join(GOBIN, "xgo"), args...)
	cmd.Env = []string{
		"GOPATH=" + build.GOPATH(),
		"GOBIN=" + GOBIN,
	}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GOPATH=") || strings.HasPrefix(e, "GOBIN=") {
			continue
		}
		cmd.Env = append(cmd.Env, e)
	}
	return cmd
}

//二元分布清理

func doPurge(cmdline []string) {
	var (
		store = flag.String("store", "", `Destination from where to purge archives (usually "gethstore/builds")`)
		limit = flag.Int("days", 30, `Age threshold above which to delete unstable archives`)
	)
	flag.CommandLine.Parse(cmdline)

	if env := build.Env(); !env.IsCronJob {
		log.Printf("skipping because not a cron job")
		os.Exit(0)
	}
//创建Azure身份验证并列出当前存档
	auth := build.AzureBlobstoreConfig{
		Account:   strings.Split(*store, "/")[0],
		Token:     os.Getenv("AZURE_BLOBSTORE_TOKEN"),
		Container: strings.SplitN(*store, "/", 2)[1],
	}
	blobs, err := build.AzureBlobstoreList(auth)
	if err != nil {
		log.Fatal(err)
	}
//迭代blob，收集并排序所有不稳定的构建
	for i := 0; i < len(blobs); i++ {
		if !strings.Contains(blobs[i].Name, "unstable") {
			blobs = append(blobs[:i], blobs[i+1:]...)
			i--
		}
	}
	for i := 0; i < len(blobs); i++ {
		for j := i + 1; j < len(blobs); j++ {
			if blobs[i].Properties.LastModified.After(blobs[j].Properties.LastModified) {
				blobs[i], blobs[j] = blobs[j], blobs[i]
			}
		}
	}
//筛选出比给定阈值更新的所有存档
	for i, blob := range blobs {
		if time.Since(blob.Properties.LastModified) < time.Duration(*limit)*24*time.Hour {
			blobs = blobs[:i]
			break
		}
	}
//删除所有标记为此类的内容并返回
	if err := build.AzureBlobstoreDelete(auth, blobs); err != nil {
		log.Fatal(err)
	}
}
