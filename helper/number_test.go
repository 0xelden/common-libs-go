package helper

import (
	"errors"
	"testing"
	"time"
)

func TestMonthToRoman(t *testing.T) {
	tests := []struct {
		input  int
		output string
		err    error
	}{
		{1, "I", nil},
		{4, "IV", nil},
		{9, "IX", nil},
		{58, "LVIII", nil},
		{1994, "MCMXCIV", nil},
		{2023, "MMXXIII", nil},
		{3999, "MMMCMXCIX", nil},
		{0, "", errors.New("invalid input: number must be between 1 and 3999")},
		{4000, "", errors.New("invalid input: number must be between 1 and 3999")},
		{-10, "", errors.New("invalid input: number must be between 1 and 3999")},
	}

	for _, test := range tests {
		result, err := MonthToRoman(time.Month(test.input))
		if result != test.output {
			t.Errorf("MonthToRoman(%d) = %s; want %s", test.input, result, test.output)
		}
		if (err == nil && test.err != nil) || (err != nil && test.err == nil) || (err != nil && test.err != nil && err.Error() != test.err.Error()) {
			t.Errorf("MonthToRoman(%d) error = %v; want %v", test.input, err, test.err)
		}
	}
}
