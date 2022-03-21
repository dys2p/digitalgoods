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

// NewPayID returns a randomly created six-digit ID. It is not guaranteed to be unique. You should try at least five times.
//
// Risk estimation:
//
//   - 33^6 different combinations, approx 10^9
//   - assuming there are 10^6 orders
//   - risk of individual order ID being not unique: 10^-3
//   - try with five different IDs: 10^-15
//   - divide by 10^6 order
//   - = overall risk of five non-unique IDs: 10^-9 or less
func NewPayID() string {
	return newID(6, payDigits)
}
