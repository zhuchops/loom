package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type pluginConfig struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Command []string `json:"command"`
}

// languageConfig describes one language. The name is the LSP language ID.
// Unset fields keep the built-in value when a config file overrides a language.
type languageConfig struct {
	Extensions []string `json:"extensions,omitempty"`
	Server     []string `json:"server,omitempty"`
	Format     []string `json:"format,omitempty"`
	Roots      []string `json:"roots,omitempty"`
}

type config struct {
	RelativeNumbers bool `json:"relative_numbers"`
	FormatOnSave    bool `json:"format_on_save"`
	OrganizeImports bool `json:"organize_imports"`
	RestoreSession  bool `json:"restore_session"`

	Profile      string                       `json:"profile"`
	Bindings     map[string]map[string]string `json:"bindings"`
	Languages    map[string]languageConfig    `json:"languages"`
	Plugins      []pluginConfig               `json:"plugins"`
	AutoComplete bool                         `json:"auto_complete"`
	AutoPairs    bool                         `json:"auto_pairs"`
}

// Language servers and formatters are used only when their command is on PATH.
func defaultLanguages() map[string]languageConfig {
	tsServer := []string{"typescript-language-server", "--stdio"}
	prettier := []string{"prettier", "--stdin-filepath", "{path}"}
	return map[string]languageConfig{
		"go":              {Extensions: []string{"go"}, Server: []string{"gopls"}},
		"typescript":      {Extensions: []string{"ts", "mts", "cts"}, Server: tsServer, Format: prettier},
		"typescriptreact": {Extensions: []string{"tsx"}, Server: tsServer, Format: prettier},
		"javascript":      {Extensions: []string{"js", "mjs", "cjs"}, Server: tsServer, Format: prettier},
		"javascriptreact": {Extensions: []string{"jsx"}, Server: tsServer, Format: prettier},
		"python":          {Extensions: []string{"py", "pyi"}, Server: []string{"pyright-langserver", "--stdio"}, Format: []string{"ruff", "format", "--stdin-filename", "{path}", "-"}},
		"rust":            {Extensions: []string{"rs"}, Server: []string{"rust-analyzer"}, Format: []string{"rustfmt"}},
		"c":               {Extensions: []string{"c", "h"}, Server: []string{"clangd"}, Format: []string{"clang-format"}},
		"cpp":             {Extensions: []string{"cc", "cpp", "cxx", "hpp", "hh"}, Server: []string{"clangd"}, Format: []string{"clang-format"}},
		"ruby":            {Extensions: []string{"rb"}, Server: []string{"ruby-lsp"}},
		"lua":             {Extensions: []string{"lua"}, Server: []string{"lua-language-server"}, Format: []string{"stylua", "-"}},
		"shellscript":     {Extensions: []string{"sh", "bash", "zsh"}, Server: []string{"bash-language-server", "start"}, Format: []string{"shfmt"}},
		"json":            {Extensions: []string{"json", "jsonc"}, Format: prettier},
		"yaml":            {Extensions: []string{"yml", "yaml"}, Format: prettier},
		"markdown":        {Extensions: []string{"md", "markdown"}, Format: prettier},
		"toml":            {Extensions: []string{"toml"}},
		"css":             {Extensions: []string{"css"}, Format: prettier},
		"html":            {Extensions: []string{"html", "htm"}, Format: prettier},
		"sql":             {Extensions: []string{"sql"}},
	}
}

// languageFor returns the language name for a path. Unknown extensions name themselves.
func (c config) languageFor(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	for _, name := range sortedKeys(c.Languages) {
		if slices.Contains(c.Languages[name].Extensions, ext) {
			return name
		}
	}
	return ext
}

// claimExtensions gives an extension to one language only, so the user's choice wins.
func claimExtensions(languages map[string]languageConfig, owner string, extensions []string) {
	for name, lang := range languages {
		if name == owner {
			continue
		}
		lang.Extensions = slices.DeleteFunc(slices.Clone(lang.Extensions), func(ext string) bool {
			return slices.Contains(extensions, ext)
		})
		languages[name] = lang
	}
}

func configPath() string {
	root := os.Getenv("XDG_CONFIG_HOME")
	if root == "" {
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, ".config")
	}
	return filepath.Join(root, "loom", "config.json")
}

func defaultConfig() config {
	return config{
		Profile: "vim", AutoComplete: true, AutoPairs: true, FormatOnSave: true, RestoreSession: true,
		Languages: defaultLanguages(),
		Bindings: map[string]map[string]string{
			"global": {"ctrl+p": "files", "ctrl+s": "save"},
			"insert": {"ctrl+space": "complete", "left": "insert-left", "right": "insert-right", "up": "insert-up", "down": "insert-down",
				"backspace": "backspace", "delete": "delete-forward", "enter": "newline", "tab": "insert-tab", "esc": "normal"},
			"modal": {"h": "left", "j": "down", "k": "up", "l": "right", "left": "left", "down": "down", "up": "up", "right": "right",
				"f": "find-char-f", "F": "find-char-F", "t": "find-char-t", "T": "find-char-T", ";": "find-char-next", ",": "find-char-prev",
				"w": "word", "b": "back-word", "e": "word-end", "0": "home", "$": "end", "g g": "top", "G": "bottom",
				"space f": "files", "space /": "project-search", "space e": "explorer", "space x": "explorer-popup",
				"space p": "palette", "space s": "toggle-sidebar", "space S": "close-explorer",
				"space b": "buffers", "space w": "save", "space a": "code-actions", "space r": "rename-symbol",
				"space d": "file-problems", "space D": "problems", "space N": "messages",
				"g o": "directory-jump", "g t": "tab-next", "g T": "tab-prev", "g S": "split-join",
				"space t h": "tab-left", "space t l": "tab-right", "space t 0": "tab-first", "space t $": "tab-last", "space t x": "close-tab",
				"space t 1": "tab-1", "space t 2": "tab-2", "space t 3": "tab-3", "space t 4": "tab-4", "space t 5": "tab-5",
				"space t 6": "tab-6", "space t 7": "tab-7", "space t 8": "tab-8", "space t 9": "tab-9",
				"space z": "zen", "space n": "relative-numbers", "space q": "close-split",
				"space g g": "git-panel", "space g d": "git-diff", "space g b": "git-branch",
				"space o": "symbols", "space O": "workspace-symbols",
				"ctrl+w v": "split", "ctrl+w s": "split-below", "ctrl+w h": "pane-h", "ctrl+w j": "pane-j", "ctrl+w k": "pane-k", "ctrl+w l": "pane-l",
				"ctrl+w left": "pane-h", "ctrl+w down": "pane-j", "ctrl+w up": "pane-k", "ctrl+w right": "pane-l",
				"ctrl+w w": "focus-pane", "ctrl+w ctrl+w": "focus-pane", "ctrl+w c": "close-split", "ctrl+w o": "only-pane",
				"g d": "definition", "g r": "references", "g h": "hover", "g ]": "jump-forward", "ctrl+o": "jump-back",
				"i": "insert", "a": "append", "A": "append-line", "I": "insert-line", "o": "open-below", "O": "open-above",
				"v": "select", "u": "undo", "ctrl+r": "redo", "x": "delete-forward", "p": "paste-after", "P": "paste-before",
				"/": "search", ".": "repeat", "] d": "next-problem", "[ d": "prev-problem", "n": "next-match", "N": "prev-match", ":": "ex", "esc": "normal"},
			"vim.normal": {"d": "delete-op", "c": "change-op", "y": "yank-op", ">": "indent-op", "<": "outdent-op", "q": "macro-record", "@": "macro-play",
				"g c c": "toggle-comment",
				"z a":   "fold-toggle", "z c": "fold-close", "z o": "fold-open", "z M": "fold-close-all", "z R": "fold-open-all", "g i": "implementation",
				"V": "select-line", "^": "first-nonblank", "D": "delete-to-end", "C": "change-to-end", "Y": "yank-line",
				"r": "replace-character", "J": "join-lines", "%": "match-bracket",
				"s a": "mini-add-motion", "s d": "mini-delete", "s r": "mini-replace", "s f": "mini-find", "s F": "mini-find-back", "s h": "mini-highlight"},
			"vim.select": {"d": "delete-selection", "c": "change-selection", "y": "yank-selection", ">": "indent-selection", "<": "outdent-selection", "S": "surround-selection", "i": "inner-object", "a": "around-object",
				"g c": "toggle-comment",
				"V":   "select-line", "o": "select-other-end", "ctrl+c": "normal", "^": "first-nonblank", "x": "delete-selection", "J": "join-lines", "%": "match-bracket", "p": "visual-paste", "P": "visual-paste-keep",
				"s a": "mini-add-selection"},
		},
	}
}

func readConfig(path string, explicit bool) (config, error) {
	cfg := defaultConfig()
	f, err := os.Open(path)
	if os.IsNotExist(err) && !explicit {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	var overlay struct {
		RelativeNumbers *bool `json:"relative_numbers"`
		FormatOnSave    *bool `json:"format_on_save"`
		OrganizeImports *bool `json:"organize_imports"`
		RestoreSession  *bool `json:"restore_session"`

		Profile      *string                      `json:"profile"`
		Bindings     map[string]map[string]string `json:"bindings"`
		Languages    map[string]languageConfig    `json:"languages"`
		Plugins      []pluginConfig               `json:"plugins"`
		AutoComplete *bool                        `json:"auto_complete"`
		AutoPairs    *bool                        `json:"auto_pairs"`

		// Older configs set only the server command for a language.
		LanguageServers map[string][]string `json:"language_servers"`
	}
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(&overlay); err != nil {
		return cfg, err
	}
	if overlay.RelativeNumbers != nil {
		cfg.RelativeNumbers = *overlay.RelativeNumbers
	}
	if overlay.FormatOnSave != nil {
		cfg.FormatOnSave = *overlay.FormatOnSave
	}
	if overlay.OrganizeImports != nil {
		cfg.OrganizeImports = *overlay.OrganizeImports
	}
	if overlay.RestoreSession != nil {
		cfg.RestoreSession = *overlay.RestoreSession
	}
	if overlay.Profile != nil {
		cfg.Profile = *overlay.Profile
	}
	if overlay.AutoComplete != nil {
		cfg.AutoComplete = *overlay.AutoComplete
	}
	if overlay.AutoPairs != nil {
		cfg.AutoPairs = *overlay.AutoPairs
	}
	for _, name := range sortedKeys(overlay.Languages) {
		lang := overlay.Languages[name]
		base := cfg.Languages[name]
		if lang.Extensions != nil {
			base.Extensions = lang.Extensions
			claimExtensions(cfg.Languages, name, lang.Extensions)
		}
		if lang.Server != nil {
			base.Server = lang.Server
		}
		if lang.Format != nil {
			base.Format = lang.Format
		}
		if lang.Roots != nil {
			base.Roots = lang.Roots
		}
		cfg.Languages[name] = base
	}
	for _, name := range sortedKeys(overlay.LanguageServers) {
		base := cfg.Languages[name]
		base.Server = overlay.LanguageServers[name]
		cfg.Languages[name] = base
	}
	for scope, keys := range overlay.Bindings {
		dst, ok := cfg.Bindings[scope]
		if !ok {
			return cfg, fmt.Errorf("unknown binding scope %q", scope)
		}
		for sequence, command := range keys {
			sequence = strings.Join(strings.Fields(sequence), " ")
			if sequence == "" {
				return cfg, fmt.Errorf("empty key sequence in %s", scope)
			}
			dst[sequence] = command
		}
	}
	cfg.Plugins = overlay.Plugins
	return cfg, nil
}

func scopes(profile, mode string) []string {
	if mode == "insert" {
		return []string{"global", "insert"}
	}
	return []string{"global", "modal", profile + "." + mode}
}

func compileKeys(cfg config, commands map[string]command) (map[string]map[string]string, error) {
	if cfg.Profile != "vim" {
		return nil, fmt.Errorf("profile must be vim")
	}
	for scope, keys := range cfg.Bindings {
		for key, id := range keys {
			if id != "" && commands[id].run == nil {
				return nil, fmt.Errorf("%s %q refers to unknown command %q", scope, key, id)
			}
		}
	}
	result := make(map[string]map[string]string)
	for _, profile := range []string{"vim"} {
		for _, mode := range []string{"normal", "insert", "select"} {
			keys := make(map[string]string)
			for _, scope := range scopes(profile, mode) {
				for sequence, id := range cfg.Bindings[scope] {
					if id == "" {
						delete(keys, sequence)
					} else {
						keys[sequence] = id
					}
				}
			}
			for sequence := range keys {
				parts := strings.Fields(sequence)
				if mode == "insert" && len(parts) > 1 && (parts[0] == "space" || len([]rune(parts[0])) == 1) {
					return nil, fmt.Errorf("%s: insert chords must start with a control or named key: %q", profile, sequence)
				}
				for i := 1; i < len(parts); i++ {
					prefix := strings.Join(parts[:i], " ")
					if _, ok := keys[prefix]; ok {
						return nil, fmt.Errorf("%s.%s: %q conflicts with chord %q", profile, mode, prefix, sequence)
					}
				}
			}
			result[profile+"."+mode] = keys
		}
	}
	return result, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func printConfig() error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(defaultConfig())
}
