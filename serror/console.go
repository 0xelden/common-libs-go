package serror

type Color int

type ColorType int

const (
	FOREGROUND ColorType = 1 + iota
)

const (
	DEFAULT Color = 1 + iota
	BLACK
	RED
	GREEN
	YELLOW
	BLUE
	MAGENTA
	CYAN
	LIGHT_GRAY
	DARK_GRAY
	LIGHT_RED
	LIGHT_GREEN
	LIGHT_YELLOW
	LIGHT_BLUE
	LIGHT_MAGENTA
	LIGHT_CYAN
	WHITE
)

var EscChar = "\x1B"
var ResetChar = EscChar + "[0m"

func GetColorCode(c Color, t ColorType) (string, bool) {
	fcs := map[Color][]string{
		DEFAULT:       {"39", "49"},
		BLACK:         {"30", "40"},
		RED:           {"31", "41"},
		GREEN:         {"32", "42"},
		YELLOW:        {"33", "43"},
		BLUE:          {"34", "44"},
		MAGENTA:       {"35", "45"},
		CYAN:          {"36", "46"},
		LIGHT_GRAY:    {"37", "47"},
		DARK_GRAY:     {"90", "100"},
		LIGHT_RED:     {"91", "101"},
		LIGHT_GREEN:   {"92", "102"},
		LIGHT_YELLOW:  {"93", "103"},
		LIGHT_BLUE:    {"94", "104"},
		LIGHT_MAGENTA: {"95", "105"},
		LIGHT_CYAN:    {"96", "106"},
		WHITE:         {"97", "107"},
	}

	cc, ok := fcs[c]
	if ok {
		if t == FOREGROUND {
			return cc[0], true
		}
		return cc[1], true
	}
	return "", false
}

func ApplyForeColor(s string, c Color) string {
	col, ok := GetColorCode(c, FOREGROUND)
	if ok {
		return EscChar + "[" + col + "m" + s + ResetChar
	}
	return s
}
