package acl

import (
	"context"
	"strconv"
	"strings"

	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/shared"
)

const (
	Local Level = iota
	Holding
	Global
)

type Level int

func (l Level) String() string {
	return strconv.Itoa(int(l))
}

func GetAccessLevel(ctx context.Context) Level {
	slug := helper.ContextGetString(ctx, shared.X_Slug)
	if len(slug) == 0 {
		return Local
	}
	part := strings.Split(slug, "/")
	if len(part) < 3 {
		return Local
	}
	return ParseAccessLevel(part[2])
}

func ParseAccessLevel(action string) Level {
	if len(action) < 4 {
		return Local
	}
	switch action[:3] {
	case "hd-":
		return Holding
	case "gl-":
		return Global
	default:
		return Local
	}
}
