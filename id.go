package digitalgoods

import (
	"crypto/rand"
	"encoding/binary"
)

const purchaseDigits = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ123456789"

const payDigits = "ABCDEFGHJKLMNPQRSTUVWXYZ123456789"

func newDigit(charset string) byte {
	b := make([]byte, 8)
	n, err := rand.Read(b)
	if n != 8 {
		panic(n)
	} else if err != nil {
		panic(err)
	}
	return charset[uint(binary.BigEndian.Uint64(b))%uint(len(charset))]
}

func newID(length int, charset string) string {
	var result = make([]byte, length)
	for i := range result {
		result[i] = newDigit(charset)
	}
	return string(result)
}

// 16 digits * log2(58) = 94 bits
func NewPurchaseID() string {
	return newID(16, purchaseDigits)
}

// 8 digits * log2(33) = 40 bits
func NewPayID() string {
	return newID(8, payDigits)
}
