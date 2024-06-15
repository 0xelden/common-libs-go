package helper

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Floor pembulatan ke bawah dengan unit faktor
// Contoh:
// Floor(0.363636, 0.001) //  0.363
// Floor(0.363636, 0.01)  //  0.36
// Floor(0.363636, 0.1)   //  0.30000000000000004
// Floor(0.363636, 0.05)  //  0.35000000000000003
// Floor(3.2, 1)          //  3
// Floor(32, 5)           //  30
// Floor(33, 5)           //  30
// Floor(32, 10)          //  30
func Floor(number, unit float64) float64 {
	return math.Floor(number/unit) * unit
}

// Ceil pembulatan ke atas dengan unit faktor
// Contoh:
// Ceil(0.363636, 0.001) //  0.364
// Ceil(0.363636, 0.01)  //  0.37
// Ceil(0.363636, 0.1)   //  0.4
// Ceil(0.363636, 0.05)  //  0.4
// Ceil(3.2, 1)          //  4
// Ceil(32, 5)           //  35
// Ceil(33, 5)           //  35
// Ceil(32, 10)          //  40
func Ceil(number, unit float64) float64 {
	return math.Ceil(number/unit) * unit
}

// RemoveDecimal menghapus decimal part dari sebuah float
// Contoh:
// RemoveDecimal(0.363636)  //  0
// RemoveDecimal(3.2)       //  3
func RemoveDecimal(number float64) float64 {
	return float64(int64(number))
}

// AdjustDecimals melakukan truncate pada decimal part sesuai dgn precision yg dipilih
// Contoh:
// 3.123456789 precision 0 => 3
// 3.123456789 precision 1 => 3.1
// 3.123456789 precision 2 => 3.12
// 3.123456789 precision 3 => 3.123
// 3.123456789 precision 4 => 3.1235
// 3.123456789 precision 5 => 3.12346
// 3.123456789 precision 6 => 3.123457
// 3.123456789 precision 7 => 3.1234568
// 3.123456789 precision 8 => 3.12345679
// 3.123456789 precision 9 => 3.123456789
func AdjustDecimals(num float64, precision int) (float64, error) {
	formatString := fmt.Sprintf("%%.%df", precision)
	strNum := fmt.Sprintf(formatString, num)
	return strconv.ParseFloat(strNum, 64)
}

// MonthToRoman converts an integer to its Roman numeral representation
func MonthToRoman(number time.Month) (string, error) {
	if number < 1 || number > 3999 {
		return "", errors.New("invalid input: number must be between 1 and 3999")
	}

	// Define the Roman numeral symbols and their corresponding values
	values := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	symbols := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}

	var result strings.Builder
	for i := 0; i < len(values); i++ {
		for int(number) >= values[i] {
			result.WriteString(symbols[i])
			number -= time.Month(values[i])
		}
	}

	return result.String(), nil
}
