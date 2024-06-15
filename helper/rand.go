package helper

import (
	cryptoRand "crypto/rand"
	"math/big"
	"math/rand"
	"time"
	"unsafe"
)

// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go#:~:text=func%20RandStringBytesMaskImprSrcUnsafe(n,unsafe.Pointer(%26b))%0A%7D

var src = rand.NewSource(time.Now().UnixNano())

const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func RandStringUnsafe(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	// nosemgrep
	return *(*string)(unsafe.Pointer(&b))
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
