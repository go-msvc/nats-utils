package datatype

import (
	"crypto/cipher"
	"crypto/des"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/go-msvc/errors"
)

var (
	block cipher.Block
)

const (
	strPrefix = "###:"
	// Same key as used in NGF
	keyText = "4d65822107fcfd5278629a0f5f3f164f55104dc76695721d"
) // const

type EncStr struct {
	plain string
	enc   string
} // EncStr

// Module init function
func init() {

	key, err := hex.DecodeString(
		keyText)

	if err != nil {
		panic(fmt.Sprintf("%+v", errors.Wrapf(err,
			"Failed to Decode String")))
	} // if failed to decode

	if block, err = des.NewTripleDESCipher(key); err != nil {
		panic(fmt.Sprintf("%+v", errors.Wrapf(err,
			"Failed to create new Cipher")))
	} // if failed to create cipher

	//replacer = strings.NewReplacer("\x00", "")

} // init ()

// Init initialises the EncStr from an encrypted string value
func (encStr *EncStr) Init(encString string) error {

	const method = "Init"

	if encStr == nil {

		return errors.Errorf("Invalid parameters %p.%s",
			encStr,
			method)

	} // if invalid params

	var err error

	if encStr.plain, err = DecryptString(encString); err != nil {

		return errors.Wrapf(err,
			"Failed to decrypt string")

	} // if failed to decrypt string

	encStr.enc = EncryptString(encStr.plain)

	return nil

} // EncStr.Init ()

// MustInit initialises the EncStr from an encrypted string value, but panics
// if the initialisation fails
func (encStr *EncStr) MustInit(encString string) {

	if err := encStr.Init(encString); err != nil {
		panic(fmt.Sprintf("%+v", errors.Wrapf(err,
			"Failed to init EnsStr")))
	} // of failed to init

} // EncStr.MustInit()

// Format Ensure that passwords are not printed anywhere
func (encStr EncStr) Format(f fmt.State, c rune) {

	_, _ = f.Write([]byte(encStr.StringEnc()))

} // EncStr.Format()

// UnmarshalText ...
func (encStr *EncStr) UnmarshalText(b []byte) error {

	if err := encStr.Init(string(b)); err != nil {
		return errors.Wrapf(err,
			"Failed to initialise Encrypted String")
	} // if failed to init

	return nil

} // EncStr.UnmarshalText()

// MarshalText ...
func (encStr EncStr) MarshalText() ([]byte, error) {

	return []byte(encStr.StringEnc()), nil

} // EncStr.MarshalText()

// Value ...
func (encStr EncStr) Value() (any, error) {

	return encStr.StringEnc(), nil

} // EncStr.Value()

// Scan ...
func (encStr *EncStr) Scan(value interface{}) error {

	switch v := value.(type) {

	case []byte:
		if err := encStr.Init(string(v)); err != nil {
			return errors.Wrapf(err,
				"Failed to initialise EncStr from []byte")
		}
	case string:
		if err := encStr.Init(v); err != nil {
			return errors.Wrapf(err,
				"Failed to initialise EncStr from string")
		}
	default:
		return errors.Errorf("EncStr cannot be initialised from %T",
			v)
	} // switch

	return nil

} // EncStr.Scan()

// StringPlain returns an unencrypted version of the string
func (encStr EncStr) StringPlain() string {

	return encStr.plain

} // EncStr.String()

// StringEnc returns an encrypted version of the string
func (encStr EncStr) StringEnc() string {

	return encStr.enc

} // EncStr.StringEnc()

// DecryptString decrypts the supplied string
func DecryptString(encString string) (string, error) {

	if strings.HasPrefix(encString, strPrefix) {

		enc := encString[len(strPrefix):]

		src, err := base64.StdEncoding.DecodeString(
			enc)

		if err != nil {
			return "", errors.Wrapf(err,
				"Failed to Decode String %s",
				enc)
		} // if failed to decode

		if len(src)%block.BlockSize() != 0 {
			return "", errors.Errorf(
				"Encrypted string length invalid")
		} // if length invalid

		dst := make([]byte, len(src))

		for i := 0; i < len(src); i += block.BlockSize() {
			block.Decrypt(
				dst[i:i+block.BlockSize()],
				src[i:i+block.BlockSize()])
		} // for each block

		splits := strings.Split(
			string(dst),
			"\x00")

		if len(splits) > 0 {
			return splits[0], nil
		} else {
			return "", nil
		}

	} else {

		return encString, nil
	}

} // DecryptString()

// MustDecryptString decrypt the supplied string, but panics if there's an error
func MustDecryptString(encString string) string {

	if plain, err := DecryptString(encString); err != nil {
		panic(fmt.Sprintf("%+v", errors.Wrapf(err,
			"Failed to decrypt string")))
	} else {
		return plain
	}

} // MustDecryptString()

// EncryptString takes a plaintext string, and returns an encrypted version
// of the string
func EncryptString(plain string) string {

	size := len(plain)
	mod := size % block.BlockSize()

	if mod != 0 {
		size += block.BlockSize() - mod
	}

	dstEnc := make([]byte, size)

	for i := 0; i < size; i += block.BlockSize() {
		block.Encrypt(
			dstEnc[i:i+block.BlockSize()],
			[]byte(plain)[i:i+block.BlockSize()])
	} // for each block

	return strPrefix + base64.StdEncoding.EncodeToString(dstEnc)

} // EncryptString()
