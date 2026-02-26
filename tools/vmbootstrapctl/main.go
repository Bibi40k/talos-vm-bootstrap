package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Bibi40k/talos-vm-bootstrap/internal/tooling/vmbootstrap"
)

func main() {
	if len(os.Args) < 2 {
		failf("usage: vmbootstrapctl <resolve|install|check-update|update-pin|sync-assets> [flags]")
	}

	switch os.Args[1] {
	case "resolve":
		runResolve(os.Args[2:])
	case "install":
		runInstall(os.Args[2:])
	case "check-update":
		runCheckUpdate(os.Args[2:])
	case "update-pin":
		runUpdatePin(os.Args[2:])
	case "sync-assets":
		runSyncAssets(os.Args[2:])
	default:
		failf("unknown command: %s", os.Args[1])
	}
}

func runResolve(args []string) {
	fs := flag.NewFlagSet("resolve", flag.ExitOnError)
	bin := fs.String("bin", "bin/vmbootstrap", "vmbootstrap binary path or command")
	repo := fs.String("repo", "../vmware-vm-bootstrap", "vmware-vm-bootstrap repo path")
	autoBuild := fs.Bool("auto-build", false, "auto-build vmbootstrap from repo if missing")
	_ = fs.Parse(args)

	resolved, err := vmbootstrap.ResolveBinary(vmbootstrap.ResolveOptions{
		Bin:       strings.TrimSpace(*bin),
		Repo:      strings.TrimSpace(*repo),
		AutoBuild: *autoBuild,
	})
	if err != nil {
		failf("resolve vmbootstrap binary: %v", err)
	}
	fmt.Println(resolved)
}

func runInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	dir := fs.String("dir", "bin", "install directory for vmbootstrap binary")
	notifyUpdate := fs.Bool("notify-update", true, "print update notice if module pin is behind latest")
	_ = fs.Parse(args)

	if *notifyUpdate {
		current, latest, hasUpdate, err := vmbootstrap.IsUpdateAvailable()
		if err == nil && hasUpdate {
			_, _ = fmt.Fprintf(os.Stderr, "vmbootstrap update available: %s -> %s\n", current, latest)
			_, _ = fmt.Fprintln(os.Stderr, "Run: make update-vmbootstrap-pin install-vmbootstrap")
		}
	}

	version, path, err := vmbootstrap.InstallPinnedToDir(strings.TrimSpace(*dir))
	if err != nil {
		failf("install vmbootstrap: %v", err)
	}
	fmt.Printf("INSTALLED %s %s\n", version, path)
}

func runCheckUpdate(args []string) {
	fs := flag.NewFlagSet("check-update", flag.ExitOnError)
	emitStatus := fs.Bool("emit-status", false, "emit machine-readable status line")
	_ = fs.Parse(args)

	current, latest, hasUpdate, err := vmbootstrap.IsUpdateAvailable()
	if err != nil {
		failf("check vmbootstrap update: %v", err)
	}
	if *emitStatus {
		if hasUpdate {
			fmt.Printf("UPDATE %s %s\n", current, latest)
		} else {
			fmt.Printf("OK %s\n", current)
		}
		return
	}
	if hasUpdate {
		fmt.Printf("Update available: %s -> %s\n", current, latest)
		fmt.Println("Run: make update-vmbootstrap-pin install-vmbootstrap")
		return
	}
	fmt.Printf("vmbootstrap pin is up-to-date: %s\n", current)
}

func runUpdatePin(_ []string) {
	version, err := vmbootstrap.UpdatePinToLatest()
	if err != nil {
		failf("update vmbootstrap pin: %v", err)
	}
	fmt.Println(version)
}

func runSyncAssets(args []string) {
	fs := flag.NewFlagSet("sync-assets", flag.ExitOnError)
	repoRoot := fs.String("repo-root", ".", "repository root where files are synced")
	force := fs.Bool("force", false, "overwrite local changes if assets differ")
	_ = fs.Parse(args)

	files, err := vmbootstrap.SyncPinnedAssets(strings.TrimSpace(*repoRoot), *force)
	if err != nil {
		failf("sync vmbootstrap assets: %v", err)
	}
	for _, f := range files {
		fmt.Printf("SYNCED %s\n", f)
	}
}

func failf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
