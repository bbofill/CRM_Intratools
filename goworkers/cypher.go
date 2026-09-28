package goworkers

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// this is very best with assimetric encryption like this:
// 1. simmetric encryption for cookies (same key for modules and controller because controller also need to read the cookie)
// 2. controller uncipher logs with private key B
// 2. Module cipher logs with public key B

//this way only controller can read the logs. this is useful because
//there is no access control for module within the api db calls.
//this implies we can't limit what module is using what table
//this way ensure that module never can read the logs it didn't matter
//what module are you using to send the logs.
//additionallly (and better) the only way to read and verify the log is within a module
// this log/adm module is the only who can decrypt logs, controller acts as timestamper
// controller also add a few fields like moduleID, username, etc.

// aes encrypt with mC.CookieEncryptionKey
func encryptCookie(text string) (string, error) {
	block, err := aes.NewCipher(decodeB64String(mC.CookieEncryptionKey))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(text), nil)
	return encodeB64String(ciphertext), nil
}

// aes decrypt with mC.CookieEncryptionKey
func decryptCookie(enc string) (string, error) {
	data := decodeB64String(enc)
	block, err := aes.NewCipher(decodeB64String(mC.CookieEncryptionKey))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", err
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// Log cypher
func encryptLogWithGPG(logContent string) string {

	var buf bytes.Buffer
	pubKeyFile, err := os.Open(mC.LogCypherPubKeyFilePath)

	if err != nil {
		return "" // Fatal: fmt.Errorf("no se pudo abrir la clave pública: %w", err)
	}

	defer pubKeyFile.Close()

	entityList, err := openpgp.ReadArmoredKeyRing(pubKeyFile)
	if err != nil {
		return "" // Fatal: fmt.Errorf("Invalid Public Key: %w", err)
	}

	writer, err := openpgp.Encrypt(&buf, entityList, nil, nil, nil)
	if err != nil {
		return "" // Fatal: fmt.Errorf("Fail cyphering Log: %w", err)
	}

	_, err = writer.Write([]byte(logContent))
	if err != nil {
		return "" //Fatal: err
	}

	writer.Close()

	return buf.String()
}
