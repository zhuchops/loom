package editor

import (
	"flag"
	"fmt"
	"os"
)

const version = "0.23.0"

// Main runs the editor as the loom command and exits the process on failure.
func Main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "loom:", err)
		os.Exit(1)
	}
}

func run() error {
	e, explicit, done, err := prepare()
	if err != nil || done {
		return err
	}
	return runTerminal(e, explicit)
}

// prepare parses the command line and builds the editor. done reports that a
// flag such as -version already did all the work.
func prepare() (e *editor, explicit bool, done bool, err error) {
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
		return nil, false, true, nil
	}
	if *dump {
		return nil, false, true, printConfig()
	}
	if flag.NArg() > 1 {
		flag.Usage()
		return nil, false, true, fmt.Errorf("expected at most one file or directory")
	}
	explicit = *path != ""
	if !explicit {
		*path = configPath()
	}
	target := flag.Arg(0)
	noProject := false
	if flag.NArg() == 0 {
		// Without an argument loom reopens the last project, or starts empty.
		if last := lastProject(); last != "" {
			if err := os.Chdir(last); err != nil {
				return nil, false, true, err
			}
		} else {
			noProject = true
		}
	}
	if info, statErr := os.Stat(target); statErr == nil && info.IsDir() {
		if err := os.Chdir(target); err != nil {
			return nil, false, true, err
		}
		target = ""
	}
	root, err := os.Getwd()
	if err != nil {
		return nil, false, true, err
	}
	if noProject {
		root, _ = os.UserHomeDir()
		if root == "" {
			root = "/"
		}
	}
	configRoot := root
	if noProject {
		configRoot = ""
	}
	cfg, err := loadConfig(*path, explicit, configRoot)
	if err != nil {
		return nil, false, true, err
	}
	if *profile != "" {
		cfg.Profile = *profile
	}
	if err := loadTheme(cfg); err != nil {
		return nil, false, true, err
	}
	b, err := loadBuffer(target)
	if err != nil {
		return nil, false, true, err
	}
	e, err = newEditor(cfg, b, root)
	if err != nil {
		return nil, false, true, err
	}
	e.configFile, e.configExplicit = *path, explicit
	if noProject {
		e.enterNoProject()
	}
	return e, target != "", false, nil
}
