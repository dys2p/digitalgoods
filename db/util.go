package db

// Mask replaces all but the first four letters of a string by asterisks.
func Mask(s string, keep int) string {
	r := []rune(s)
	for i := keep; i < len(r); i++ {
		r[i] = '*'
	}
	return string(r)
}
