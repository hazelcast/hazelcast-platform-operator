package util

import "math/rand"

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"0123456789"

func RandString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
