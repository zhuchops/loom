package editor

import (
	"flag"
	"fmt"
	"os"
)

const version = "0.11.1"

// Main runs the editor as the loom command and exits the process on failure.
func Main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "loom:", err)
		os.Exit(1)
	}
}

func run() error {
	profile := flag.String("profile", "", "editing profile (vim)")
	path := flag.String("config", "", "path to a JSON config")
	showVersion := flag.Bool("version", false, "print the editor version")
	dump := flag.Bool("dump-config", false, "print the default JSON config")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: loom [flags] [file or directory]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showVersion {
		fmt.Println("loom " + version)
		return nil
	}
	if *dump {
		return printConfig()
	}
	if flag.NArg() > 1 {
		flag.Usage()
		return fmt.Errorf("expected at most one file or directory")
	}
	explicit := *path != ""
	if !explicit {
		*path = configPath()
	}
	cfg, err := readConfig(*path, explicit)
	if err != nil {
		return fmt.Errorf("config %s: %w", *path, err)
	}
	if *profile != "" {
		cfg.Profile = *profile
	}
	target := flag.Arg(0)
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		if err := os.Chdir(target); err != nil {
			return err
		}
		target = ""
	}
	if err := loadTheme(cfg); err != nil {
		return err
	}
	b, err := loadBuffer(target)
	if err != nil {
		return err
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	e, err := newEditor(cfg, b, root)
	if err != nil {
		return err
	}
	return runTerminal(e, target != "")
}
