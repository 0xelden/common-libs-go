package helper

import (
	"strings"
	"sync"
	"testing"
)

func TestRandStringUnsafe(t *testing.T) {
	seen := map[string]int{}
	for i := 0; i < 10_000; i++ {
		got := RandStringUnsafe(9)
		if seen[got] == 1 {
			t.Errorf("RandStringUnsafe() duplicate")
		}
		seen[got] = 1
	}
}

func TestRandStringUnsafeConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for g := 0; g < 32; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1_000; i++ {
				got := RandStringUnsafe(16)
				if len(got) != 16 || strings.Trim(got, letterBytes) != "" {
					t.Errorf("RandStringUnsafe() = %q", got)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestRandPassword(t *testing.T) {
	const runs = 10_000
	const size = 8

	for i := 0; i < runs; i++ {
		s := RandPassword(8)
		if len(s) != size {
			t.Fatalf("expected length 8, got %d (%q)", len(s), s)
		}

		//t.Log(s)

		var hasLower, hasUpper, hasDigit, hasSymbol bool
		for _, r := range s {
			switch {
			case 'a' <= r && r <= 'z':
				hasLower = true
			case 'A' <= r && r <= 'Z':
				hasUpper = true
			case '0' <= r && r <= '9':
				hasDigit = true
			default:
				hasSymbol = true
			}
		}

		if !hasLower || !hasUpper || !hasDigit || !hasSymbol {
			t.Fatalf("token %q missing class: lower=%v upper=%v digit=%v symbol=%v",
				s, hasLower, hasUpper, hasDigit, hasSymbol)
		}
	}
}
