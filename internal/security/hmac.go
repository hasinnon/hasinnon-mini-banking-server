package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func SignHMAC(body []byte, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func VerifyHMAC(body []byte, sigHex, key string) bool {
	if sigHex == "" {
		return false
	}
	want := SignHMAC(body, key)
	return hmac.Equal([]byte(want), []byte(sigHex))
}