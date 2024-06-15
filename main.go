package main

import (
	"context"
	"os"
	"strings"

	_ "github.com/joho/godotenv/autoload"
	_ "github.com/0xelden/common-libs-go/cli"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/logger"
	"github.com/0xelden/common-libs-go/serror"
)

func main() {
	command := helper.Index(os.Args, 1)

	matched, serr := onCommand(context.Background(), command)
	if serr != nil {
		logger.Err(serr)
		return
	}
	if !matched {
		logger.Infof("usage: %s dump <db,...>", os.Args[0])
	}
}

func onCommand(ctx context.Context, command string) (matched bool, serr serror.SError) {
	switch command {
	case "dump":
		db := helper.Filter(strings.Split(helper.Index(os.Args, 2), ","), func(v string) bool {
			return v != ""
		})
		helper.Trace(ctx, "db", db)
		if len(db) == 0 {
			return true, serror.New("dump failed, invalid db name")
		}
		for _, v := range db {
			helper.Trace(ctx, v, true)
			logger.Infof("start dump %v", v)
			err := helper.PgDump(ctx, v)
			if err != nil {
				return true, serror.NewFromError(err)
			}
			logger.Infof("end dump %v", v)
		}
		return true, nil
	default:
		return false, nil
	}
}
