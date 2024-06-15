package cli

import (
	"fmt"
	"os"
	"runtime"
	"text/tabwriter"

	_ "github.com/joho/godotenv/autoload"
	flag "github.com/spf13/pflag"
	"github.com/0xelden/common-libs-go/helper"
)

var (
	Version string
	Commit  string
	Created string
	Build   string
)

var (
	flagVersion bool
	command     = helper.Index(os.Args, 1, "")
)

func init() {
	flag.BoolVarP(&flagVersion, "version", "v", false, "print version")
	flag.Parse()
	if command == "dump" {

	}
	if flagVersion {
		printVersion()
		os.Exit(0)
	}
}

func printVersion() {
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	rt := fmt.Sprintf("%s %s %s %dCPU", runtime.GOOS, runtime.GOARCH, runtime.Version(), runtime.NumCPU())
	_, _ = fmt.Fprintln(w, "VERSION\t: "+Version)
	_, _ = fmt.Fprintln(w, "COMMIT\t: "+Commit)
	_, _ = fmt.Fprintln(w, "BUILD\t: "+Build)
	_, _ = fmt.Fprintln(w, "CREATED\t: "+Created)
	_, _ = fmt.Fprintln(w, "RUNTIME\t: "+rt)
	_, _ = fmt.Fprintln(w, "APP_DEPLOYMENT\t: "+helper.Env("APP_DEPLOYMENT"))
	_ = w.Flush()
}
