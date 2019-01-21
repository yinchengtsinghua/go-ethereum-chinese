
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2014 Go Ethereum作者
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

package keystore

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func tmpKeyStoreIface(t *testing.T, encrypted bool) (dir string, ks keyStore) {
	d, err := ioutil.TempDir("", "geth-keystore-test")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted {
		ks = &keyStorePassphrase{d, veryLightScryptN, veryLightScryptP, true}
	} else {
		ks = &keyStorePlain{d}
	}
	return d, ks
}

func TestKeyStorePlain(t *testing.T) {
	dir, ks := tmpKeyStoreIface(t, false)
	defer os.RemoveAll(dir)

pass := "" //未使用，但API要求
	k1, account, err := storeNewKey(ks, rand.Reader, pass)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := ks.GetKey(k1.Address, account.URL.Path, pass)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k1.Address, k2.Address) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k1.PrivateKey, k2.PrivateKey) {
		t.Fatal(err)
	}
}

func TestKeyStorePassphrase(t *testing.T) {
	dir, ks := tmpKeyStoreIface(t, true)
	defer os.RemoveAll(dir)

	pass := "foo"
	k1, account, err := storeNewKey(ks, rand.Reader, pass)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := ks.GetKey(k1.Address, account.URL.Path, pass)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k1.Address, k2.Address) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k1.PrivateKey, k2.PrivateKey) {
		t.Fatal(err)
	}
}

func TestKeyStorePassphraseDecryptionFail(t *testing.T) {
	dir, ks := tmpKeyStoreIface(t, true)
	defer os.RemoveAll(dir)

	pass := "foo"
	k1, account, err := storeNewKey(ks, rand.Reader, pass)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ks.GetKey(k1.Address, account.URL.Path, "bar"); err != ErrDecrypt {
		t.Fatalf("wrong error for invalid passphrase\ngot %q\nwant %q", err, ErrDecrypt)
	}
}

func TestImportPreSaleKey(t *testing.T) {
	dir, ks := tmpKeyStoreIface(t, true)
	defer os.RemoveAll(dir)

//生成的预售密钥文件的文件内容：
//python pyethsaletool.py genwallet
//密码为“foo”
	fileContent := "{\"encseed\": \"26d87f5f2bf9835f9a47eefae571bc09f9107bb13d54ff12a4ec095d01f83897494cf34f7bed2ed34126ecba9db7b62de56c9d7cd136520a0427bfb11b8954ba7ac39b90d4650d3448e31185affcd74226a68f1e94b1108e6e0a4a91cdd83eba\", \"ethaddr\": \"d4584b5f6229b7be90727b0fc8c6b91bb427821f\", \"email\": \"gustav.simonsson@gmail.com\", \"btcaddr\": \"1EVknXyFC68kKNLkh6YnKzW41svSRoaAcx\"}"
	pass := "foo"
	account, _, err := importPreSaleKey(ks, []byte(fileContent), pass)
	if err != nil {
		t.Fatal(err)
	}
	if account.Address != common.HexToAddress("d4584b5f6229b7be90727b0fc8c6b91bb427821f") {
		t.Errorf("imported account has wrong address %x", account.Address)
	}
	if !strings.HasPrefix(account.URL.Path, dir) {
		t.Errorf("imported account file not in keystore directory: %q", account.URL)
	}
}

//在以太坊JSON测试中测试和使用密钥存储测试；
//测试数据密钥存储测试/basic_tests.json
type KeyStoreTestV3 struct {
	Json     encryptedKeyJSONV3
	Password string
	Priv     string
}

type KeyStoreTestV1 struct {
	Json     encryptedKeyJSONV1
	Password string
	Priv     string
}

func TestV3_PBKDF2_1(t *testing.T) {
	t.Parallel()
	tests := loadKeyStoreTestV3("testdata/v3_test_vector.json", t)
	testDecryptV3(tests["wikipage_test_vector_pbkdf2"], t)
}

var testsSubmodule = filepath.Join("..", "..", "tests", "testdata", "KeyStoreTests")

func skipIfSubmoduleMissing(t *testing.T) {
	if !common.FileExist(testsSubmodule) {
		t.Skipf("can't find JSON tests from submodule at %s", testsSubmodule)
	}
}

func TestV3_PBKDF2_2(t *testing.T) {
	skipIfSubmoduleMissing(t)
	t.Parallel()
	tests := loadKeyStoreTestV3(filepath.Join(testsSubmodule, "basic_tests.json"), t)
	testDecryptV3(tests["test1"], t)
}

func TestV3_PBKDF2_3(t *testing.T) {
	skipIfSubmoduleMissing(t)
	t.Parallel()
	tests := loadKeyStoreTestV3(filepath.Join(testsSubmodule, "basic_tests.json"), t)
	testDecryptV3(tests["python_generated_test_with_odd_iv"], t)
}

func TestV3_PBKDF2_4(t *testing.T) {
	skipIfSubmoduleMissing(t)
	t.Parallel()
	tests := loadKeyStoreTestV3(filepath.Join(testsSubmodule, "basic_tests.json"), t)
	testDecryptV3(tests["evilnonce"], t)
}

func TestV3_Scrypt_1(t *testing.T) {
	t.Parallel()
	tests := loadKeyStoreTestV3("testdata/v3_test_vector.json", t)
	testDecryptV3(tests["wikipage_test_vector_scrypt"], t)
}

func TestV3_Scrypt_2(t *testing.T) {
	skipIfSubmoduleMissing(t)
	t.Parallel()
	tests := loadKeyStoreTestV3(filepath.Join(testsSubmodule, "basic_tests.json"), t)
	testDecryptV3(tests["test2"], t)
}

func TestV1_1(t *testing.T) {
	t.Parallel()
	tests := loadKeyStoreTestV1("testdata/v1_test_vector.json", t)
	testDecryptV1(tests["test1"], t)
}

func TestV1_2(t *testing.T) {
	t.Parallel()
	ks := &keyStorePassphrase{"testdata/v1", LightScryptN, LightScryptP, true}
	addr := common.HexToAddress("cb61d5a9c4896fb9658090b597ef0e7be6f7b67e")
	file := "testdata/v1/cb61d5a9c4896fb9658090b597ef0e7be6f7b67e/cb61d5a9c4896fb9658090b597ef0e7be6f7b67e"
	k, err := ks.GetKey(addr, file, "g")
	if err != nil {
		t.Fatal(err)
	}
	privHex := hex.EncodeToString(crypto.FromECDSA(k.PrivateKey))
	expectedHex := "d1b1178d3529626a1a93e073f65028370d14c7eb0936eb42abef05db6f37ad7d"
	if privHex != expectedHex {
		t.Fatal(fmt.Errorf("Unexpected privkey: %v, expected %v", privHex, expectedHex))
	}
}

func testDecryptV3(test KeyStoreTestV3, t *testing.T) {
	privBytes, _, err := decryptKeyV3(&test.Json, test.Password)
	if err != nil {
		t.Fatal(err)
	}
	privHex := hex.EncodeToString(privBytes)
	if test.Priv != privHex {
		t.Fatal(fmt.Errorf("Decrypted bytes not equal to test, expected %v have %v", test.Priv, privHex))
	}
}

func testDecryptV1(test KeyStoreTestV1, t *testing.T) {
	privBytes, _, err := decryptKeyV1(&test.Json, test.Password)
	if err != nil {
		t.Fatal(err)
	}
	privHex := hex.EncodeToString(privBytes)
	if test.Priv != privHex {
		t.Fatal(fmt.Errorf("Decrypted bytes not equal to test, expected %v have %v", test.Priv, privHex))
	}
}

func loadKeyStoreTestV3(file string, t *testing.T) map[string]KeyStoreTestV3 {
	tests := make(map[string]KeyStoreTestV3)
	err := common.LoadJSON(file, &tests)
	if err != nil {
		t.Fatal(err)
	}
	return tests
}

func loadKeyStoreTestV1(file string, t *testing.T) map[string]KeyStoreTestV1 {
	tests := make(map[string]KeyStoreTestV1)
	err := common.LoadJSON(file, &tests)
	if err != nil {
		t.Fatal(err)
	}
	return tests
}

func TestKeyForDirectICAP(t *testing.T) {
	t.Parallel()
	key := NewKeyForDirectICAP(rand.Reader)
	if !strings.HasPrefix(key.Address.Hex(), "0x00") {
		t.Errorf("Expected first address byte to be zero, have: %s", key.Address.Hex())
	}
}

func TestV3_31_Byte_Key(t *testing.T) {
	t.Parallel()
	tests := loadKeyStoreTestV3("testdata/v3_test_vector.json", t)
	testDecryptV3(tests["31_byte_key"], t)
}

func TestV3_30_Byte_Key(t *testing.T) {
	t.Parallel()
	tests := loadKeyStoreTestV3("testdata/v3_test_vector.json", t)
	testDecryptV3(tests["30_byte_key"], t)
}
