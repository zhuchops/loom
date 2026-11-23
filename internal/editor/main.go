package editor

import (
	"flag"
	"fmt"
	"os"
)

const version = "0.29.6"

// Main runs the editor as the loom command and exits the process on failure.
func Main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "loom:", err)
		os.Exit(1)
	}
}

func run() error {
	e, explicit, done, err := prepare(false)
	if err != nil || done {
		return err
	}
	return runTerminal(e, explicit)
}

// prepare parses the command line and builds the editor. done reports that a
// flag such as -version already did all the work. With tolerant set, a config
// that can't be used is reported in the editor instead of stopping it: a
// window opened from Finder has nowhere else to say so.
func prepare(tolerant bool) (e *editor, explicit bool, done bool, err error) {
	profile := flag.String("profile", "", "editing profile (vim)")
	path := flag.String("config", "", "path to a JSON config")
	showVersion := flag.Bool("version", false, "print the editor version")
	dump := flag.Bool("dump-config", false, "print the default JSON config")
	if tolerant {
		flag.BoolVar(&newWindow, "new", false, "open in a new window instead of an already open one")
		flag.BoolVar(&newWindow, "n", false, "same as -new")
		flag.BoolVar(&foreground, "foreground", false, "stay attached to the terminal and print the log there")
		flag.BoolVar(&foreground, "f", false, "same as -foreground")
	}
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
	if tolerant && !newWindow && *path == "" && forwardToRunning(flag.Args()) {
		return nil, false, true, nil // an open window took it
	}
	if tolerant {
		if done, err := detachFromTerminal(); done || err != nil {
			return nil, false, true, err
		}
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
		reopen := true
		if cfg, err := readConfig(*path, explicit); err == nil {
			reopen = cfg.ReopenProject
		}
		if last := lastProject(); reopen && last != "" {
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
	var broken error
	fallback := func(err error) (config, error) {
		if !tolerant {
			return config{}, err
		}
		again := broken != nil
		if !again {
			broken = err
			// Keep the files that are fine: without the project's, or failing
			// that config.json alone.
			if cfg, ok := usableConfig(*path, explicit); ok {
				return cfg, nil
			}
		}
		cfg := defaultConfig()
		cfg.path, cfg.explicit = *path, explicit
		return cfg, nil
	}
	cfg, err := loadConfig(*path, explicit, configRoot)
	if err != nil {
		if cfg, err = fallback(err); err != nil {
			return nil, false, true, err
		}
	}
	if *profile != "" {
		cfg.Profile = *profile
	}
	if err := loadTheme(cfg); err != nil {
		if cfg, err = fallback(err); err != nil {
			return nil, false, true, err
		}
	}
	b, err := loadBuffer(target)
	if err != nil {
		return nil, false, true, err
	}
	e, err = newEditor(cfg, b, root)
	if err != nil {
		if cfg, err = fallback(err); err != nil {
			return nil, false, true, err
		}
		if e, err = newEditor(cfg, b, root); err != nil {
			return nil, false, true, err
		}
	}
	if broken != nil {
		e.configProblem = broken
		e.notify("Config: " + broken.Error())
	}
	if noProject {
		e.enterNoProject()
	}
	return e, target != "", false, nil
}

// usableConfig is as much of the user's config as loads.
func usableConfig(path string, explicit bool) (config, bool) {
	if cfg, err := readConfig(path, explicit); err == nil && checkTheme(cfg) == nil {
		return cfg, true
	}
	cfg := defaultConfig()
	cfg.path, cfg.explicit = path, explicit
	cfg, err := applyConfig(cfg, path, explicit, false)
	return cfg, err == nil && checkTheme(cfg) == nil
}
