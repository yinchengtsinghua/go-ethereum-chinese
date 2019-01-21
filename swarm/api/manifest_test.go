
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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/swarm/storage"
)

func manifest(paths ...string) (manifestReader storage.LazySectionReader) {
	var entries []string
	for _, path := range paths {
		entry := fmt.Sprintf(`{"path":"%s"}`, path)
		entries = append(entries, entry)
	}
	manifest := fmt.Sprintf(`{"entries":[%s]}`, strings.Join(entries, ","))
	return &storage.LazyTestSectionReader{
		SectionReader: io.NewSectionReader(strings.NewReader(manifest), 0, int64(len(manifest))),
	}
}

func testGetEntry(t *testing.T, path, match string, multiple bool, paths ...string) *manifestTrie {
	quitC := make(chan bool)
	fileStore := storage.NewFileStore(nil, storage.NewFileStoreParams())
	ref := make([]byte, fileStore.HashSize())
	trie, err := readManifest(manifest(paths...), ref, fileStore, false, quitC, NOOPDecrypt)
	if err != nil {
		t.Errorf("unexpected error making manifest: %v", err)
	}
	checkEntry(t, path, match, multiple, trie)
	return trie
}

func checkEntry(t *testing.T, path, match string, multiple bool, trie *manifestTrie) {
	entry, fullpath := trie.getEntry(path)
	if match == "-" && entry != nil {
		t.Errorf("expected no match for '%s', got '%s'", path, fullpath)
	} else if entry == nil {
		if match != "-" {
			t.Errorf("expected entry '%s' to match '%s', got no match", match, path)
		}
	} else if fullpath != match {
		t.Errorf("incorrect entry retrieved for '%s'. expected path '%v', got '%s'", path, match, fullpath)
	}

	if multiple && entry.Status != http.StatusMultipleChoices {
		t.Errorf("Expected %d Multiple Choices Status for path %s, match %s, got %d", http.StatusMultipleChoices, path, match, entry.Status)
	} else if !multiple && entry != nil && entry.Status == http.StatusMultipleChoices {
		t.Errorf("Were not expecting %d Multiple Choices Status for path %s, match %s, but got it", http.StatusMultipleChoices, path, match)
	}
}

func TestGetEntry(t *testing.T) {
//文件系统清单始终包含规范化路径
	testGetEntry(t, "a", "a", false, "a")
	testGetEntry(t, "b", "-", false, "a")
testGetEntry(t, "/a//“，”A“，假，”A“）
//退路
	testGetEntry(t, "/a", "", false, "")
	testGetEntry(t, "/a/b", "a/b", false, "a/b")
//最长/最深数学
	testGetEntry(t, "read", "read", true, "readme.md", "readit.md")
	testGetEntry(t, "rf", "-", false, "readme.md", "readit.md")
	testGetEntry(t, "readme", "readme", false, "readme.md")
	testGetEntry(t, "readme", "-", false, "readit.md")
	testGetEntry(t, "readme.md", "readme.md", false, "readme.md")
	testGetEntry(t, "readme.md", "-", false, "readit.md")
	testGetEntry(t, "readmeAmd", "-", false, "readit.md")
	testGetEntry(t, "readme.mdffff", "-", false, "readme.md")
	testGetEntry(t, "ab", "ab", true, "ab/cefg", "ab/cedh", "ab/kkkkkk")
	testGetEntry(t, "ab/ce", "ab/ce", true, "ab/cefg", "ab/cedh", "ab/ceuuuuuuuuuu")
	testGetEntry(t, "abc", "abc", true, "abcd", "abczzzzef", "abc/def", "abc/e/g")
	testGetEntry(t, "a/b", "a/b", true, "a", "a/bc", "a/ba", "a/b/c")
	testGetEntry(t, "a/b", "a/b", false, "a", "a/b", "a/bb", "a/b/c")
testGetEntry(t, "//A//B/“，”A/B“，假，”A“，”A/B“，”A/BB“，”A/B/C“）
}

func TestExactMatch(t *testing.T) {
	quitC := make(chan bool)
	mf := manifest("shouldBeExactMatch.css", "shouldBeExactMatch.css.map")
	fileStore := storage.NewFileStore(nil, storage.NewFileStoreParams())
	ref := make([]byte, fileStore.HashSize())
	trie, err := readManifest(mf, ref, fileStore, false, quitC, nil)
	if err != nil {
		t.Errorf("unexpected error making manifest: %v", err)
	}
	entry, _ := trie.getEntry("shouldBeExactMatch.css")
	if entry.Path != "" {
		t.Errorf("Expected entry to match %s, got: %s", "shouldBeExactMatch.css", entry.Path)
	}
	if entry.Status == http.StatusMultipleChoices {
		t.Errorf("Got status %d, which is unexepcted", http.StatusMultipleChoices)
	}
}

func TestDeleteEntry(t *testing.T) {

}

//testaddfilewithmanifestPath测试在路径中添加项
//已经作为清单存在，只是将条目添加到清单
//而不是用条目替换清单
func TestAddFileWithManifestPath(t *testing.T) {
//创建包含“ab”和“ac”的清单
	manifest, _ := json.Marshal(&Manifest{
		Entries: []ManifestEntry{
			{Path: "ab", Hash: "ab"},
			{Path: "ac", Hash: "ac"},
		},
	})
	reader := &storage.LazyTestSectionReader{
		SectionReader: io.NewSectionReader(bytes.NewReader(manifest), 0, int64(len(manifest))),
	}
	fileStore := storage.NewFileStore(nil, storage.NewFileStoreParams())
	ref := make([]byte, fileStore.HashSize())
	trie, err := readManifest(reader, ref, fileStore, false, nil, NOOPDecrypt)
	if err != nil {
		t.Fatal(err)
	}
	checkEntry(t, "ab", "ab", false, trie)
	checkEntry(t, "ac", "ac", false, trie)

//现在添加路径“a”并检查我们仍然可以得到“ab”和“ac”
	entry := &manifestTrieEntry{}
	entry.Path = "a"
	entry.Hash = "a"
	trie.addEntry(entry, nil)
	checkEntry(t, "ab", "ab", false, trie)
	checkEntry(t, "ac", "ac", false, trie)
	checkEntry(t, "a", "a", false, trie)
}

//testreadmanifestsuperlimit创建一个清单阅读器，其中数据的长度超过
//manifestSizeLimit并检查readManifest函数是否返回准确的错误
//消息。
//清单数据不是JSON编码格式，因此无法
//如果限制检查失败，则成功分析尝试。
func TestReadManifestOverSizeLimit(t *testing.T) {
	manifest := make([]byte, manifestSizeLimit+1)
	reader := &storage.LazyTestSectionReader{
		SectionReader: io.NewSectionReader(bytes.NewReader(manifest), 0, int64(len(manifest))),
	}
	_, err := readManifest(reader, storage.Address{}, nil, false, nil, NOOPDecrypt)
	if err == nil {
		t.Fatal("got no error from readManifest")
	}
//错误消息是HTTP响应正文的一部分
//这证明了精确的字符串验证是正确的。
	got := err.Error()
	want := fmt.Sprintf("Manifest size of %v bytes exceeds the %v byte limit", len(manifest), manifestSizeLimit)
	if got != want {
		t.Fatalf("got error mesage %q, expected %q", got, want)
	}
}
