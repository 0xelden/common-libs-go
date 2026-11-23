package helper

import (
	cryptoRand "crypto/rand"
	"math/big"
	"unsafe"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"
const letterIdxMask = 1<<6 - 1

func RandStringUnsafe(n int) string {
	b := make([]byte, n)
	buf := make([]byte, n*2+8)
	for i := 0; i < n; {
		_, _ = cryptoRand.Read(buf)
		for _, v := range buf {
			if idx := int(v & letterIdxMask); idx < len(letterBytes) {
				b[i] = letterBytes[idx]
				if i++; i == n {
					break
				}
			}
		}
	}

	// nosemgrep
	return *(*string)(unsafe.Pointer(&b)) // #nosec G103
}

const (
	lower   = "bcdefhkmnpqrtwxyz" // remove ambigous characters
	upper   = "ABCDEFGHKLMNPQRTWXYZ"
	digits  = "234578"
	symbols = `!#$%&*+/<=>?@_`
	// full character pool
	pool = lower + upper + digits + symbols
)

// RandPassword returns a string that always contains
// at least one lowercase, one uppercase, one digit and one symbol
// Panics only if crypto/rand fails (extremely unlikely)
func RandPassword(size int) string {
	// Guaranteed 1-of-each
	seed := []byte{
		lower[randomIndex(len(lower))],
		upper[randomIndex(len(upper))],
		digits[randomIndex(len(digits))],
		symbols[randomIndex(len(symbols))],
	}

	// Fill to full length from the entire pool
	for len(seed) < size {
		seed = append(seed, pool[randomIndex(len(pool))])
	}

	// Fisher-Yates shuffle to randomise positions
	for i := len(seed) - 1; i > 0; i-- {
		j := randomIndex(i + 1)
		seed[i], seed[j] = seed[j], seed[i]
	}

	return string(seed)
}

func randomIndex(max int) int {
	n, err := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(max)))
	if err != nil {
		panic(err)
	}
	return int(n.Int64())
}
