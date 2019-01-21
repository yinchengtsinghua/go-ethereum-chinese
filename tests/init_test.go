
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

package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/params"
)

var (
	baseDir            = filepath.Join(".", "testdata")
	blockTestDir       = filepath.Join(baseDir, "BlockchainTests")
	stateTestDir       = filepath.Join(baseDir, "GeneralStateTests")
	transactionTestDir = filepath.Join(baseDir, "TransactionTests")
	vmTestDir          = filepath.Join(baseDir, "VMTests")
	rlpTestDir         = filepath.Join(baseDir, "RLPTests")
	difficultyTestDir  = filepath.Join(baseDir, "BasicTests")
)

func readJSON(reader io.Reader, value interface{}) error {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("error reading JSON file: %v", err)
	}
	if err = json.Unmarshal(data, &value); err != nil {
		if syntaxerr, ok := err.(*json.SyntaxError); ok {
			line := findLine(data, syntaxerr.Offset)
			return fmt.Errorf("JSON syntax error at line %v: %v", line, err)
		}
		return err
	}
	return nil
}

func readJSONFile(fn string, value interface{}) error {
	file, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer file.Close()

	err = readJSON(file, value)
	if err != nil {
		return fmt.Errorf("%s in file %s", err.Error(), fn)
	}
	return nil
}

//findline将给定偏移量的行号返回到数据中。
func findLine(data []byte, offset int64) (line int) {
	line = 1
	for i, r := range string(data) {
		if int64(i) >= offset {
			return
		}
		if r == '\n' {
			line++
		}
	}
	return
}

//TestMatcher控制对测试的跳过和链配置分配。
type testMatcher struct {
	configpat    []testConfig
	failpat      []testFailure
	skiploadpat  []*regexp.Regexp
	slowpat      []*regexp.Regexp
	whitelistpat *regexp.Regexp
}

type testConfig struct {
	p      *regexp.Regexp
	config params.ChainConfig
}

type testFailure struct {
	p      *regexp.Regexp
	reason string
}

//使用-short标志时，SkipShortMode跳过匹配的测试。
func (tm *testMatcher) slow(pattern string) {
	tm.slowpat = append(tm.slowpat, regexp.MustCompile(pattern))
}

//SkipLoad跳过与模式匹配的测试的JSON加载。
func (tm *testMatcher) skipLoad(pattern string) {
	tm.skiploadpat = append(tm.skiploadpat, regexp.MustCompile(pattern))
}

//失败为匹配模式的测试添加预期失败。
func (tm *testMatcher) fails(pattern string, reason string) {
	if reason == "" {
		panic("empty fail reason")
	}
	tm.failpat = append(tm.failpat, testFailure{regexp.MustCompile(pattern), reason})
}

func (tm *testMatcher) whitelist(pattern string) {
	tm.whitelistpat = regexp.MustCompile(pattern)
}

//config为匹配模式的测试定义链配置。
func (tm *testMatcher) config(pattern string, cfg params.ChainConfig) {
	tm.configpat = append(tm.configpat, testConfig{regexp.MustCompile(pattern), cfg})
}

//findskip根据测试跳过模式匹配名称。
func (tm *testMatcher) findSkip(name string) (reason string, skipload bool) {
	isWin32 := runtime.GOARCH == "386" && runtime.GOOS == "windows"
	for _, re := range tm.slowpat {
		if re.MatchString(name) {
			if testing.Short() {
				return "skipped in -short mode", false
			}
			if isWin32 {
				return "skipped on 32bit windows", false
			}
		}
	}
	for _, re := range tm.skiploadpat {
		if re.MatchString(name) {
			return "skipped by skipLoad", true
		}
	}
	return "", false
}

//findconfig返回与定义的模式匹配的链配置。
func (tm *testMatcher) findConfig(name string) *params.ChainConfig {
//todo（fjl）：当min go版本为1.8时，可以从testing.t派生名称。
	for _, m := range tm.configpat {
		if m.p.MatchString(name) {
			return &m.config
		}
	}
	return new(params.ChainConfig)
}

//checkfailure检查是否需要失败。
func (tm *testMatcher) checkFailure(t *testing.T, name string, err error) error {
//todo（fjl）：当min go版本为1.8时，可以从t派生名称
	failReason := ""
	for _, m := range tm.failpat {
		if m.p.MatchString(name) {
			failReason = m.reason
			break
		}
	}
	if failReason != "" {
		t.Logf("expected failure: %s", failReason)
		if err != nil {
			t.Logf("error: %v", err)
			return nil
		}
		return fmt.Errorf("test succeeded unexpectedly")
	}
	return err
}

//walk为给定目录中的所有子测试调用其runtest参数。
//
//runtest应该是func类型的函数（t*testing.t，name string，x<testtype>），
//其中test type是包含在测试文件中的测试类型。
func (tm *testMatcher) walk(t *testing.T, dir string, runTest interface{}) {
//浏览目录。
	dirinfo, err := os.Stat(dir)
	if os.IsNotExist(err) || !dirinfo.IsDir() {
		fmt.Fprintf(os.Stderr, "can't find test files in %s, did you clone the tests submodule?\n", dir)
		t.Skip("missing test files")
	}
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		name := filepath.ToSlash(strings.TrimPrefix(path, dir+string(filepath.Separator)))
		if info.IsDir() {
			if _, skipload := tm.findSkip(name + "/"); skipload {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".json" {
			t.Run(name, func(t *testing.T) { tm.runTestFile(t, path, name, runTest) })
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (tm *testMatcher) runTestFile(t *testing.T, path, name string, runTest interface{}) {
	if r, _ := tm.findSkip(name); r != "" {
		t.Skip(r)
	}
	if tm.whitelistpat != nil {
		if !tm.whitelistpat.MatchString(name) {
			t.Skip("Skipped by whitelist")
		}
	}
	t.Parallel()

//将文件作为map[string]<testtype>加载。
	m := makeMapFromTestFunc(runTest)
	if err := readJSONFile(path, m.Addr().Interface()); err != nil {
		t.Fatal(err)
	}

//从映射中运行所有测试。如果文件中只有一个测试，则不要在子测试中换行。
	keys := sortedMapKeys(m)
	if len(keys) == 1 {
		runTestFunc(runTest, t, name, m, keys[0])
	} else {
		for _, key := range keys {
			name := name + "/" + key
			t.Run(key, func(t *testing.T) {
				if r, _ := tm.findSkip(name); r != "" {
					t.Skip(r)
				}
				runTestFunc(runTest, t, name, m, key)
			})
		}
	}
}

func makeMapFromTestFunc(f interface{}) reflect.Value {
	stringT := reflect.TypeOf("")
	testingT := reflect.TypeOf((*testing.T)(nil))
	ftyp := reflect.TypeOf(f)
	if ftyp.Kind() != reflect.Func || ftyp.NumIn() != 3 || ftyp.NumOut() != 0 || ftyp.In(0) != testingT || ftyp.In(1) != stringT {
		panic(fmt.Sprintf("bad test function type: want func(*testing.T, string, <TestType>), have %s", ftyp))
	}
	testType := ftyp.In(2)
	mp := reflect.New(reflect.MapOf(stringT, testType))
	return mp.Elem()
}

func sortedMapKeys(m reflect.Value) []string {
	keys := make([]string, m.Len())
	for i, k := range m.MapKeys() {
		keys[i] = k.String()
	}
	sort.Strings(keys)
	return keys
}

func runTestFunc(runTest interface{}, t *testing.T, name string, m reflect.Value, key string) {
	reflect.ValueOf(runTest).Call([]reflect.Value{
		reflect.ValueOf(t),
		reflect.ValueOf(name),
		m.MapIndex(reflect.ValueOf(key)),
	})
}
