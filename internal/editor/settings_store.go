package editor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Settings are read from three files, each on top of the one before:
//
//	config.json    the user's config
//	settings.json  beside it: what Loom writes for settings newer than 0.17
//	.loom.json     in the project root, restricted
//
// settings.json keeps an older loom on the same machine working, since that
// refuses a config.json with keys it doesn't know.

func addedPath(path string) string {
	if path == "" {
		return ""
	}
	added := filepath.Join(filepath.Dir(path), "settings.json")
	if added == filepath.Clean(path) {
		return "" // -config named that file itself
	}
	return added
}

func (e *editor) userConfigPath() string {
	if e.cfg.path != "" {
		return e.cfg.path
	}
	return configPath()
}

// configRoot is where a project config may be. With no project there is none.
func (e *editor) configRoot() string {
	if e.noProject {
		return ""
	}
	return e.root
}

// projectSets reports whether the open project decides this setting.
func (e *editor) projectSets(key string) bool { return e.cfg.fromProject[key] }

// userSets reports whether the user's files change this setting.
func (e *editor) userSets(key string) bool { return e.cfg.fromUser[key] }

type configDoc struct {
	path    string
	doc     map[string]any
	changed bool
}

func readDoc(path string) (*configDoc, error) {
	d := &configDoc{path: path, doc: map[string]any{}}
	if path == "" {
		return d, nil
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return d, nil
	}
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return d, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber() // numbers we don't touch are written back as they were
	if err := dec.Decode(&d.doc); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", filepath.Base(path), err)
	}
	if d.doc == nil {
		d.doc = map[string]any{}
	}
	return d, nil
}

func (d *configDoc) get(path ...string) (any, bool) {
	var at any = d.doc
	for _, key := range path {
		m, ok := at.(map[string]any)
		if !ok {
			return nil, false
		}
		if at, ok = m[key]; !ok {
			return nil, false
		}
	}
	return at, true
}

func (d *configDoc) set(value any, path ...string) {
	at := d.doc
	for _, key := range path[:len(path)-1] {
		next, ok := at[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			at[key] = next
		}
		at = next
	}
	at[path[len(path)-1]] = value
	d.changed = true
}

// remove deletes a value and any object that leaves empty.
func (d *configDoc) remove(path ...string) {
	var drop func(m map[string]any, path []string) bool
	drop = func(m map[string]any, path []string) bool {
		v, ok := m[path[0]]
		if !ok {
			return false
		}
		if len(path) > 1 {
			child, ok := v.(map[string]any)
			if !ok || !drop(child, path[1:]) {
				return false
			}
			if len(child) > 0 {
				return true
			}
		}
		delete(m, path[0])
		return true
	}
	if drop(d.doc, path) {
		d.changed = true
	}
}

// editUserConfig changes the user's files. Nothing is written unless the
// result is a config Loom could start with.
func (e *editor) editUserConfig(change func(main, added *configDoc) error) error {
	path := e.userConfigPath()
	main, err := readDoc(path)
	if err != nil {
		return err
	}
	added, err := readDoc(addedPath(path))
	if err != nil {
		return err
	}
	if added.path == "" {
		added = main // everything lives in the one file
	}
	if err := change(main, added); err != nil {
		return err
	}
	pending := map[string][]byte{}
	for _, d := range []*configDoc{main, added} {
		if !d.changed {
			continue
		}
		var out bytes.Buffer
		enc := json.NewEncoder(&out)
		enc.SetEscapeHTML(false) // keys such as ">" stay readable
		enc.SetIndent("", "  ")
		if err := enc.Encode(d.doc); err != nil {
			return err
		}
		pending[d.path] = out.Bytes()
	}
	if len(pending) == 0 {
		return nil
	}
	cfg, err := loadLayers(path, e.cfg.explicit, e.configRoot(), pending)
	if err != nil {
		return err
	}
	cfg.Profile = e.cfg.Profile
	keys, err := compileKeys(cfg, e.commands)
	if err != nil {
		return err
	}
	if err := checkTheme(cfg); err != nil {
		return err
	}
	for path, text := range pending {
		if err := replaceFile(path, text); err != nil {
			return err
		}
	}
	return e.useConfig(cfg, keys)
}

// replaceFile swaps in new text at once, following a symlinked dotfile.
func replaceFile(path string, text []byte) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".loom-settings-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(text)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(f.Name(), mode)
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func (e *editor) setSetting(key string, value any) error {
	if e.projectSets(key) {
		return fmt.Errorf("%s is set by this project's .loom.json", key)
	}
	s := settingByKey[key]
	if v, ok := value.(float64); ok {
		v = math.Round(v*100) / 100
		value = v
		if _, whole := e.cfg.value(key).(int); whole {
			value = int(math.Round(v))
		}
	}
	return e.editUserConfig(func(main, added *configDoc) error {
		_, inMain := main.get(key)
		_, inAdded := added.get(key)
		if inAdded || s.added && !inMain {
			added.set(value, key)
		} else {
			main.set(value, key)
		}
		return nil
	})
}

func (e *editor) resetSetting(key string) error {
	if key == "theme" {
		if err := os.Remove(themeStatePath()); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	err := e.editUserConfig(func(main, added *configDoc) error {
		main.remove(key)
		added.remove(key)
		return nil
	})
	if err == nil && key == "theme" && e.cfg.Theme == "" {
		// Nothing pins a theme any more, so this is the built-in one.
		e.showTheme(defaultTheme())
	}
	return err
}

// setLanguageSetting changes one language. A nil value goes back to the
// global setting.
func (e *editor) setLanguageSetting(lang, key string, value any) error {
	if e.projectSets("languages." + lang + "." + key) {
		return fmt.Errorf("%s for %s is set by this project's .loom.json", key, lang)
	}
	return e.editUserConfig(func(main, added *configDoc) error {
		switch _, inAdded := added.get("languages", lang, key); {
		case value == nil:
			main.remove("languages", lang, key)
			added.remove("languages", lang, key)
		case inAdded:
			added.set(value, "languages", lang, key)
		default:
			main.set(value, "languages", lang, key)
		}
		return nil
	})
}

// Commands that 0.17 doesn't have. A binding to one in config.json would stop
// that version from starting.
var addedCommands = []string{"open-project", "close-project", "recent-projects"}

// bindingScope is where a change made for one mode belongs.
func (e *editor) bindingScope(mode string) string {
	if mode == "insert" {
		return "insert"
	}
	return e.cfg.Profile + "." + mode
}

func keysFor(keys map[string]string, id string) []string {
	var out []string
	for seq, bound := range keys {
		if bound == id {
			out = append(out, seq)
		}
	}
	slices.SortFunc(out, func(a, b string) int {
		if len(a) != len(b) {
			return len(a) - len(b)
		}
		return strings.Compare(a, b)
	})
	return out
}

// bindKey makes sequence the only key for a command in one mode. An empty
// sequence leaves the command without keys.
func (e *editor) bindKey(mode, id, sequence string) error {
	if mode == "insert" && len([]rune(sequence)) == 1 || mode == "insert" && sequence == "space" {
		return fmt.Errorf("%s is needed for typing in Insert mode", keyLabel(sequence))
	}
	scope := e.bindingScope(mode)
	old := keysFor(e.keys[e.cfg.Profile+"."+mode], id)
	err := e.editUserConfig(func(main, added *configDoc) error {
		put := func(seq, command string) {
			if _, ok := added.get("bindings", scope, seq); ok || slices.Contains(addedCommands, command) {
				added.set(command, "bindings", scope, seq)
			} else {
				main.set(command, "bindings", scope, seq)
			}
		}
		for _, seq := range old {
			if seq != sequence {
				put(seq, "")
			}
		}
		if sequence != "" {
			put(sequence, id)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if now := keysFor(e.keys[e.cfg.Profile+"."+mode], id); sequence != "" && !slices.Contains(now, sequence) {
		return fmt.Errorf("saved, but this project's .loom.json gives those keys to something else")
	}
	return nil
}

// resetKey undoes the user's changes to one command's keys in one mode.
func (e *editor) resetKey(mode, id string) error {
	builtin, err := compileKeys(defaultConfig(), e.commands)
	if err != nil {
		return err
	}
	defaults := builtin[e.cfg.Profile+"."+mode]
	scope := e.bindingScope(mode)
	return e.editUserConfig(func(main, added *configDoc) error {
		for _, d := range []*configDoc{main, added} {
			v, _ := d.get("bindings", scope)
			bound, _ := v.(map[string]any)
			for seq, command := range bound {
				clean := strings.Join(strings.Fields(seq), " ")
				if command == id || defaults[clean] == id {
					d.remove("bindings", scope, seq)
				}
			}
		}
		return nil
	})
}
