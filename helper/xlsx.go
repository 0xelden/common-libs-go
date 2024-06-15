package helper

import (
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/tealeg/xlsx"
	"github.com/0xelden/common-libs-go/serror"
)

func ParseCellAsFloat64(sheet *xlsx.Sheet, row, col int) (value float64, serr serror.SError) {
	cell := strings.TrimSpace(sheet.Cell(row, col).String())
	if cell == "" || cell == "-" {
		return 0, nil
	}
	value = StringToFloat(cell, -4269)
	if value == -4269 {
		return 0, serror.Newf("sheet:%s, row:%d, col:%d failed to parse float value, got:%v", sheet.Name, row, col, cell)
	}
	return value, nil
}

func StrictParseCellAsString(sheet *xlsx.Sheet, row, col int) (value string, serr serror.SError) {
	cell := strings.TrimSpace(sheet.Cell(row, col).String())
	if cell == "" {
		return value, serror.Newf("sheet:%s, row:%d, col:%d failed to parse required string value, got:%v", sheet.Name, row, col, cell)
	}
	return cell, nil
}

func ParseCellAsString(sheet *xlsx.Sheet, row, col int, defaultStr string) *string {
	cell := strings.TrimSpace(sheet.Cell(row, col).String())
	if cell == "" {
		if defaultStr != "" {
			return &defaultStr
		}
		return nil
	}
	return &cell
}

func NormalizeCellUpper(sheet *xlsx.Sheet, row, col int, defaultStr ...string) string {
	return strings.ToUpper(strings.TrimSpace(Deref(ParseCellAsString(sheet, row, col, Coalesce(defaultStr...)))))
}

func NormalizeCellLower(sheet *xlsx.Sheet, row, col int, defaultStr ...string) string {
	return strings.ToLower(strings.TrimSpace(Deref(ParseCellAsString(sheet, row, col, Coalesce(defaultStr...)))))
}

func ParseCellAsDatetime(sheet *xlsx.Sheet, row, col int, defaultDatetime *time.Time, layout ...string) *time.Time {
	cell := strings.TrimSpace(sheet.Cell(row, col).String())
	if cell == "" {
		return defaultDatetime
	}

	for _, v := range layout {
		res, err := time.Parse(v, cell)
		if err == nil {
			return &res
		}
	}

	// parse date here
	res, err := sheet.Cell(row, col).GetTime(false)
	if err == nil {
		return &res
	}
	res, err = sheet.Cell(row, col).GetTime(true)
	if err == nil {
		return &res
	}

	// handle string valued datetime
	res, err = time.Parse("2006-01-02", cell)
	if err == nil {
		return &res
	}
	res, err = dateparse.ParseAny(cell)
	if err == nil {
		return &res
	}

	return defaultDatetime
}
