
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/sctx"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"golang.org/x/crypto/scrypt"
	"golang.org/x/crypto/sha3"
	cli "gopkg.in/urfave/cli.v1"
)

var (
	ErrDecrypt                = errors.New("cant decrypt - forbidden")
	ErrUnknownAccessType      = errors.New("unknown access type (or not implemented)")
	ErrDecryptDomainForbidden = errors.New("decryption request domain forbidden - can only decrypt on localhost")
	AllowedDecryptDomains     = []string{
		"localhost",
		"127.0.0.1",
	}
)

const EMPTY_CREDENTIALS = ""

type AccessEntry struct {
	Type      AccessType
	Publisher string
	Salt      []byte
	Act       string
	KdfParams *KdfParams
}

type DecryptFunc func(*ManifestEntry) error

func (a *AccessEntry) MarshalJSON() (out []byte, err error) {

	return json.Marshal(struct {
		Type      AccessType `json:"type,omitempty"`
		Publisher string     `json:"publisher,omitempty"`
		Salt      string     `json:"salt,omitempty"`
		Act       string     `json:"act,omitempty"`
		KdfParams *KdfParams `json:"kdf_params,omitempty"`
	}{
		Type:      a.Type,
		Publisher: a.Publisher,
		Salt:      hex.EncodeToString(a.Salt),
		Act:       a.Act,
		KdfParams: a.KdfParams,
	})

}

func (a *AccessEntry) UnmarshalJSON(value []byte) error {
	v := struct {
		Type      AccessType `json:"type,omitempty"`
		Publisher string     `json:"publisher,omitempty"`
		Salt      string     `json:"salt,omitempty"`
		Act       string     `json:"act,omitempty"`
		KdfParams *KdfParams `json:"kdf_params,omitempty"`
	}{}

	err := json.Unmarshal(value, &v)
	if err != nil {
		return err
	}
	a.Act = v.Act
	a.KdfParams = v.KdfParams
	a.Publisher = v.Publisher
	a.Salt, err = hex.DecodeString(v.Salt)
	if err != nil {
		return err
	}
	if len(a.Salt) != 32 {
		return errors.New("salt should be 32 bytes long")
	}
	a.Type = v.Type
	return nil
}

type KdfParams struct {
	N int `json:"n"`
	P int `json:"p"`
	R int `json:"r"`
}

type AccessType string

const AccessTypePass = AccessType("pass")
const AccessTypePK = AccessType("pk")
const AccessTypeACT = AccessType("act")

//NewAccessEntryPassword创建清单访问项，以便创建受密码保护的操作
func NewAccessEntryPassword(salt []byte, kdfParams *KdfParams) (*AccessEntry, error) {
	if len(salt) != 32 {
		return nil, fmt.Errorf("salt should be 32 bytes long")
	}
	return &AccessEntry{
		Type:      AccessTypePass,
		Salt:      salt,
		KdfParams: kdfParams,
	}, nil
}

//NewAccessEntryPk创建一个清单访问条目，以便创建一个由一对椭圆曲线键保护的行为。
func NewAccessEntryPK(publisher string, salt []byte) (*AccessEntry, error) {
	if len(publisher) != 66 {
		return nil, fmt.Errorf("publisher should be 66 characters long, got %d", len(publisher))
	}
	if len(salt) != 32 {
		return nil, fmt.Errorf("salt should be 32 bytes long")
	}
	return &AccessEntry{
		Type:      AccessTypePK,
		Publisher: publisher,
		Salt:      salt,
	}, nil
}

//newaccessentryat创建一个清单访问条目，以便创建一个由EC密钥和密码组合保护的行为。
func NewAccessEntryACT(publisher string, salt []byte, act string) (*AccessEntry, error) {
	if len(salt) != 32 {
		return nil, fmt.Errorf("salt should be 32 bytes long")
	}
	if len(publisher) != 66 {
		return nil, fmt.Errorf("publisher should be 66 characters long")
	}

	return &AccessEntry{
		Type:      AccessTypeACT,
		Publisher: publisher,
		Salt:      salt,
		Act:       act,
		KdfParams: DefaultKdfParams,
	}, nil
}

//noopdecrypt是一个通用的解密函数，在真正的行为解密功能所在的地方传递给API。
//或者不需要，或者不能在即时范围内实现
func NOOPDecrypt(*ManifestEntry) error {
	return nil
}

var DefaultKdfParams = NewKdfParams(262144, 1, 8)

//newkdfarams返回具有给定scrypt参数的kdfarams结构
func NewKdfParams(n, p, r int) *KdfParams {

	return &KdfParams{
		N: n,
		P: p,
		R: r,
	}
}

//newsessionkeypassword基于共享机密（密码）和给定的salt创建会话密钥
//和访问项中的kdf参数
func NewSessionKeyPassword(password string, accessEntry *AccessEntry) ([]byte, error) {
	if accessEntry.Type != AccessTypePass && accessEntry.Type != AccessTypeACT {
		return nil, errors.New("incorrect access entry type")

	}
	return sessionKeyPassword(password, accessEntry.Salt, accessEntry.KdfParams)
}

func sessionKeyPassword(password string, salt []byte, kdfParams *KdfParams) ([]byte, error) {
	return scrypt.Key(
		[]byte(password),
		salt,
		kdfParams.N,
		kdfParams.R,
		kdfParams.P,
		32,
	)
}

//newsessionkeypk为给定的密钥对和给定的salt值使用ECDH共享机密创建新的act会话密钥
func NewSessionKeyPK(private *ecdsa.PrivateKey, public *ecdsa.PublicKey, salt []byte) ([]byte, error) {
	granteePubEcies := ecies.ImportECDSAPublic(public)
	privateKey := ecies.ImportECDSA(private)

	bytes, err := privateKey.GenerateShared(granteePubEcies, 16, 16)
	if err != nil {
		return nil, err
	}
	bytes = append(salt, bytes...)
	sessionKey := crypto.Keccak256(bytes)
	return sessionKey, nil
}

func (a *API) doDecrypt(ctx context.Context, credentials string, pk *ecdsa.PrivateKey) DecryptFunc {
	return func(m *ManifestEntry) error {
		if m.Access == nil {
			return nil
		}

		allowed := false
		requestDomain := sctx.GetHost(ctx)
		for _, v := range AllowedDecryptDomains {
			if strings.Contains(requestDomain, v) {
				allowed = true
			}
		}

		if !allowed {
			return ErrDecryptDomainForbidden
		}

		switch m.Access.Type {
		case "pass":
			if credentials != "" {
				key, err := NewSessionKeyPassword(credentials, m.Access)
				if err != nil {
					return err
				}

				ref, err := hex.DecodeString(m.Hash)
				if err != nil {
					return err
				}

				enc := NewRefEncryption(len(ref) - 8)
				decodedRef, err := enc.Decrypt(ref, key)
				if err != nil {
					return ErrDecrypt
				}

				m.Hash = hex.EncodeToString(decodedRef)
				m.Access = nil
				return nil
			}
			return ErrDecrypt
		case "pk":
			publisherBytes, err := hex.DecodeString(m.Access.Publisher)
			if err != nil {
				return ErrDecrypt
			}
			publisher, err := crypto.DecompressPubkey(publisherBytes)
			if err != nil {
				return ErrDecrypt
			}
			key, err := NewSessionKeyPK(pk, publisher, m.Access.Salt)
			if err != nil {
				return ErrDecrypt
			}
			ref, err := hex.DecodeString(m.Hash)
			if err != nil {
				return err
			}

			enc := NewRefEncryption(len(ref) - 8)
			decodedRef, err := enc.Decrypt(ref, key)
			if err != nil {
				return ErrDecrypt
			}

			m.Hash = hex.EncodeToString(decodedRef)
			m.Access = nil
			return nil
		case "act":
			var (
				sessionKey []byte
				err        error
			)

			publisherBytes, err := hex.DecodeString(m.Access.Publisher)
			if err != nil {
				return ErrDecrypt
			}
			publisher, err := crypto.DecompressPubkey(publisherBytes)
			if err != nil {
				return ErrDecrypt
			}

			sessionKey, err = NewSessionKeyPK(pk, publisher, m.Access.Salt)
			if err != nil {
				return ErrDecrypt
			}

			found, ciphertext, decryptionKey, err := a.getACTDecryptionKey(ctx, storage.Address(common.Hex2Bytes(m.Access.Act)), sessionKey)
			if err != nil {
				return err
			}
			if !found {
//尝试返回到密码
				if credentials != "" {
					sessionKey, err = NewSessionKeyPassword(credentials, m.Access)
					if err != nil {
						return err
					}
					found, ciphertext, decryptionKey, err = a.getACTDecryptionKey(ctx, storage.Address(common.Hex2Bytes(m.Access.Act)), sessionKey)
					if err != nil {
						return err
					}
					if !found {
						return ErrDecrypt
					}
				} else {
					return ErrDecrypt
				}
			}
			enc := NewRefEncryption(len(ciphertext) - 8)
			decodedRef, err := enc.Decrypt(ciphertext, decryptionKey)
			if err != nil {
				return ErrDecrypt
			}

			ref, err := hex.DecodeString(m.Hash)
			if err != nil {
				return err
			}

			enc = NewRefEncryption(len(ref) - 8)
			decodedMainRef, err := enc.Decrypt(ref, decodedRef)
			if err != nil {
				return ErrDecrypt
			}
			m.Hash = hex.EncodeToString(decodedMainRef)
			m.Access = nil
			return nil
		}
		return ErrUnknownAccessType
	}
}

func (a *API) getACTDecryptionKey(ctx context.Context, actManifestAddress storage.Address, sessionKey []byte) (found bool, ciphertext, decryptionKey []byte, err error) {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(append(sessionKey, 0))
	lookupKey := hasher.Sum(nil)
	hasher.Reset()

	hasher.Write(append(sessionKey, 1))
	accessKeyDecryptionKey := hasher.Sum(nil)
	hasher.Reset()

	lk := hex.EncodeToString(lookupKey)
	list, err := a.GetManifestList(ctx, NOOPDecrypt, actManifestAddress, lk)
	if err != nil {
		return false, nil, nil, err
	}
	for _, v := range list.Entries {
		if v.Path == lk {
			cipherTextBytes, err := hex.DecodeString(v.Hash)
			if err != nil {
				return false, nil, nil, err
			}
			return true, cipherTextBytes, accessKeyDecryptionKey, nil
		}
	}
	return false, nil, nil, nil
}

func GenerateAccessControlManifest(ctx *cli.Context, ref string, accessKey []byte, ae *AccessEntry) (*Manifest, error) {
	refBytes, err := hex.DecodeString(ref)
	if err != nil {
		return nil, err
	}
//用accesskey加密ref
	enc := NewRefEncryption(len(refBytes))
	encrypted, err := enc.Encrypt(refBytes, accessKey)
	if err != nil {
		return nil, err
	}

	m := &Manifest{
		Entries: []ManifestEntry{
			{
				Hash:        hex.EncodeToString(encrypted),
				ContentType: ManifestType,
				ModTime:     time.Now(),
				Access:      ae,
			},
		},
	}

	return m, nil
}

//dopk是cli api的一个助手函数，用于处理
//根据cli上下文、ec键和salt创建会话键和访问项
func DoPK(ctx *cli.Context, privateKey *ecdsa.PrivateKey, granteePublicKey string, salt []byte) (sessionKey []byte, ae *AccessEntry, err error) {
	if granteePublicKey == "" {
		return nil, nil, errors.New("need a grantee Public Key")
	}
	b, err := hex.DecodeString(granteePublicKey)
	if err != nil {
		log.Error("error decoding grantee public key", "err", err)
		return nil, nil, err
	}

	granteePub, err := crypto.DecompressPubkey(b)
	if err != nil {
		log.Error("error decompressing grantee public key", "err", err)
		return nil, nil, err
	}

	sessionKey, err = NewSessionKeyPK(privateKey, granteePub, salt)
	if err != nil {
		log.Error("error getting session key", "err", err)
		return nil, nil, err
	}

	ae, err = NewAccessEntryPK(hex.EncodeToString(crypto.CompressPubkey(&privateKey.PublicKey)), salt)
	if err != nil {
		log.Error("error generating access entry", "err", err)
		return nil, nil, err
	}

	return sessionKey, ae, nil
}

//doact是cli api的一个助手函数，用于处理
//在给定cli上下文、ec密钥、密码授予者和salt的情况下，创建访问密钥、访问条目和行为清单（包括上载它）
func DoACT(ctx *cli.Context, privateKey *ecdsa.PrivateKey, salt []byte, grantees []string, encryptPasswords []string) (accessKey []byte, ae *AccessEntry, actManifest *Manifest, err error) {
	if len(grantees) == 0 && len(encryptPasswords) == 0 {
		return nil, nil, nil, errors.New("did not get any grantee public keys or any encryption passwords")
	}

	publisherPub := hex.EncodeToString(crypto.CompressPubkey(&privateKey.PublicKey))
	grantees = append(grantees, publisherPub)

	accessKey = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		panic("reading from crypto/rand failed: " + err.Error())
	}
	if _, err := io.ReadFull(rand.Reader, accessKey); err != nil {
		panic("reading from crypto/rand failed: " + err.Error())
	}

	lookupPathEncryptedAccessKeyMap := make(map[string]string)
	i := 0
	for _, v := range grantees {
		i++
		if v == "" {
			return nil, nil, nil, errors.New("need a grantee Public Key")
		}
		b, err := hex.DecodeString(v)
		if err != nil {
			log.Error("error decoding grantee public key", "err", err)
			return nil, nil, nil, err
		}

		granteePub, err := crypto.DecompressPubkey(b)
		if err != nil {
			log.Error("error decompressing grantee public key", "err", err)
			return nil, nil, nil, err
		}
		sessionKey, err := NewSessionKeyPK(privateKey, granteePub, salt)
		if err != nil {
			return nil, nil, nil, err
		}

		hasher := sha3.NewLegacyKeccak256()
		hasher.Write(append(sessionKey, 0))
		lookupKey := hasher.Sum(nil)

		hasher.Reset()
		hasher.Write(append(sessionKey, 1))

		accessKeyEncryptionKey := hasher.Sum(nil)

		enc := NewRefEncryption(len(accessKey))
		encryptedAccessKey, err := enc.Encrypt(accessKey, accessKeyEncryptionKey)
		if err != nil {
			return nil, nil, nil, err
		}
		lookupPathEncryptedAccessKeyMap[hex.EncodeToString(lookupKey)] = hex.EncodeToString(encryptedAccessKey)
	}

	for _, pass := range encryptPasswords {
		sessionKey, err := sessionKeyPassword(pass, salt, DefaultKdfParams)
		if err != nil {
			return nil, nil, nil, err
		}
		hasher := sha3.NewLegacyKeccak256()
		hasher.Write(append(sessionKey, 0))
		lookupKey := hasher.Sum(nil)

		hasher.Reset()
		hasher.Write(append(sessionKey, 1))

		accessKeyEncryptionKey := hasher.Sum(nil)

		enc := NewRefEncryption(len(accessKey))
		encryptedAccessKey, err := enc.Encrypt(accessKey, accessKeyEncryptionKey)
		if err != nil {
			return nil, nil, nil, err
		}
		lookupPathEncryptedAccessKeyMap[hex.EncodeToString(lookupKey)] = hex.EncodeToString(encryptedAccessKey)
	}

	m := &Manifest{
		Entries: []ManifestEntry{},
	}

	for k, v := range lookupPathEncryptedAccessKeyMap {
		m.Entries = append(m.Entries, ManifestEntry{
			Path:        k,
			Hash:        v,
			ContentType: "text/plain",
		})
	}

	ae, err = NewAccessEntryACT(hex.EncodeToString(crypto.CompressPubkey(&privateKey.PublicKey)), salt, "")
	if err != nil {
		return nil, nil, nil, err
	}

	return accessKey, ae, m, nil
}

//dopassword是cli api的一个助手函数，用于处理
//根据cli上下文、密码和salt创建会话密钥和访问条目。
//默认情况下-DefaultKdfParams用作scrypt参数
func DoPassword(ctx *cli.Context, password string, salt []byte) (sessionKey []byte, ae *AccessEntry, err error) {
	ae, err = NewAccessEntryPassword(salt, DefaultKdfParams)
	if err != nil {
		return nil, nil, err
	}

	sessionKey, err = NewSessionKeyPassword(password, ae)
	if err != nil {
		return nil, nil, err
	}
	return sessionKey, ae, nil
}
