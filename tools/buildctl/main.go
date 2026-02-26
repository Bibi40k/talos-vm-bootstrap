package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Bibi40k/talos-vm-bootstrap/internal/tooling/buildctl"
)

func main() {
	if len(os.Args) < 2 {
		fail("usage: buildctl <preflight|require-config|require-out> [flags]")
	}

	switch os.Args[1] {
	case "preflight":
		runPreflight(os.Args[2:])
	case "require-config":
		runRequireConfig(os.Args[2:])
	case "require-out":
		runRequireOut(os.Args[2:])
	default:
		fail("unknown command: " + os.Args[1])
	}
}

func runPreflight(args []string) {
	fs := flag.NewFlagSet("preflight", flag.ExitOnError)
	requiredGo := fs.String("required-go", "", "required Go version")
	_ = fs.Parse(args)
	if err := buildctl.EnsureGoToolchain(strings.TrimSpace(*requiredGo)); err != nil {
		fail(err.Error())
	}
}

func runRequireConfig(args []string) {
	fs := flag.NewFlagSet("require-config", flag.ExitOnError)
	path := fs.String("path", "", "path to required config file")
	_ = fs.Parse(args)
	if err := buildctl.RequireConfig(strings.TrimSpace(*path)); err != nil {
		fail(err.Error())
	}
}

func runRequireOut(args []string) {
	fs := flag.NewFlagSet("require-out", flag.ExitOnError)
	out := fs.String("out", "", "OUT target path")
	_ = fs.Parse(args)
	if err := buildctl.RequireOut(strings.TrimSpace(*out)); err != nil {
		fail(err.Error())
	}
}

func fail(msg string) {
	_, _ = fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
