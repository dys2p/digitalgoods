package db

import (
	"crypto/rand"
	"encoding/binary"
)

const idBytes = "abcdefghijkmnopqrstuvwxyzABCDEFGHKLMNPQRSTUVWXYZ123456789"

func newDigit() byte {
	b := make([]byte, 8)
	n, err := rand.Read(b)
	if n != 8 {
		panic(n)
	} else if err != nil {
		panic(err)
	}
	return idBytes[uint(binary.BigEndian.Uint64(b))%uint(len(idBytes))]
}

// 16 digits = 93 bit
func NewID16() string {
	var result = make([]byte, 16)
	for i := range result {
		result[i] = newDigit()
	}
	return string(result)
}
