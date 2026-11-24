# Loom 0.8

Loom is a Go terminal editor with a Zed-inspired workspace, Vim-style editing, language-server integration, a live Markdown preview, and eleven color themes (Everforest by default).

## Run it

### Install

With Homebrew (macOS and Linux). The formula builds the terminal editor from source; on macOS the cask installs the window version, `Loom.app`, from the latest release into Applications with a `loom-desktop` command:

```sh
brew tap zhuchops/loom https://github.com/zhuchops/loom
brew install loom                   # the terminal editor
brew install --cask loom-desktop    # Loom.app and loom-desktop (macOS)
```

The app is not notarized, so the cask clears the quarantine flag Homebrew puts on downloads; otherwise macOS would refuse to open it. After each release the workflow points the formula and the cask at it, so `brew upgrade` picks it up.

Or the ready-made builds from the GitHub releases, for macOS and Linux on arm64 and amd64:

```sh
curl -fsSL https://raw.githubusercontent.com/zhuchops/loom/main/install.sh | sh
curl -fsSL https://raw.githubusercontent.com/zhuchops/loom/main/install.sh | sh -s -- --desktop   # also the window version
```

`install.sh` puts `loom` in `~/.local/bin` (`--dir` for elsewhere), checks every download against the release's `checksums.txt`, and with `--desktop` installs `Loom.app` (one app for Apple silicon and Intel) into `/Applications` with a `loom-desktop` command beside `loom`, or on Linux the `loom-desktop` binary, which needs WebKitGTK 4.1 and GTK 3 (it says which packages when they are missing). `--version v0.31.0` installs a given release. The Linux desktop build is made on Ubuntu 24.04, so it needs a glibc at least that new.

Releases are built by `.github/workflows/release.yml` for every pushed `v*` tag, with `packaging/release.sh` (`terminal`, `macos` or `linux`), which also works by hand. There is no Windows build yet.

### With Go

Install with Go 1.26 or later:

```sh
go install github.com/zhuchops/loom@latest
```

Run `loom` from the directory you want to edit. If your shell cannot find it, add Go's binary directory to your PATH. The default is `$(go env GOPATH)/bin` unless you set `GOBIN`.

### Installing on Linux

Loom needs Go 1.26 or later, and several distributions ship an older one. Check with `go version`. If it is older, take Go from [go.dev/dl](https://go.dev/dl/) instead of the package manager (use `arm64` in the file name on ARM machines):

```sh
curl -LO https://go.dev/dl/go1.26.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc   # or ~/.zshrc
```

Open a new shell, then `go install github.com/zhuchops/loom@latest` and run `loom`. To pin a release use `@v0.27.0` instead of `@latest`. If the Go proxy has not seen a new tag yet, run `GOPROXY=direct go install github.com/zhuchops/loom@latest`.

The terminal version needs only Go. For the system clipboard install `wl-clipboard` (Wayland) or `xclip` (X11). For file icons use a terminal font from [Nerd Fonts](https://www.nerdfonts.com/). Language servers and formatters are picked up from `PATH` when present.

| Distribution | Go, clipboard (terminal version) |
|---|---|
| Debian, Ubuntu, Mint | `sudo apt install golang-go wl-clipboard xclip git` |
| Fedora, RHEL, Rocky, Alma | `sudo dnf install golang wl-clipboard xclip git` |
| Arch, Manjaro, EndeavourOS | `sudo pacman -S go wl-clipboard xclip git` |
| openSUSE | `sudo zypper install go wl-clipboard xclip git` |
| Alpine | `sudo apk add go wl-clipboard xclip git` |
| NixOS, or Nix anywhere | `nix shell nixpkgs#go nixpkgs#wl-clipboard nixpkgs#xclip nixpkgs#git` |

If `go version` prints less than 1.26 after that, use the tarball above.

The desktop window (`loom-desktop`) is drawn in the system web view through [Wails](https://wails.io). It is built with cgo and needs a C compiler plus the development files for GTK 3 and WebKitGTK. Current distributions ship WebKitGTK 4.1, which takes the extra build tag `webkit2_41`; on an older one that only has 4.0 (Ubuntu 22.04, Debian 11) install the 4.0 package and leave the tag out. The Linux desktop build has not been tested, so treat these lists as a starting point:

| Distribution | Build dependencies |
|---|---|
| Debian, Ubuntu, Mint | `sudo apt install gcc pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev` |
| Fedora, RHEL | `sudo dnf install gcc pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel` |
| Arch, Manjaro | `sudo pacman -S gcc pkgconf gtk3 webkit2gtk-4.1` |
| openSUSE, Alpine, others | a C compiler, `pkg-config`, and the GTK 3 and WebKitGTK 4.1 development packages |

Then:

```sh
go install -tags desktop,production,webkit2_41 github.com/zhuchops/loom/cmd/loom-desktop@latest
loom-desktop
```

On macOS the system already has WebKit; there the tags are `desktop,production`. The `production` tag is Wails' own: without it the program stops at start and asks for it.

If the build stops on a missing header, install the development package that provides it and run the command again.

To build from a source checkout:

```sh
go build -o loom .
```

Launch it from the directory you want to browse. Bare `loom` starts on the Dashboard as tab 1 and restores any file tabs from the project session. Pass a file path to start in that file:

```sh
loom
loom .
loom path/to/project
loom main.go
```

`:w` saves, `:w PATH` saves to a path, `:q` quits, and `:q!` discards unsaved changes and quits.

A directory argument sets the project root and opens its Dashboard and saved session.

On the Dashboard, use `j`/`k` or the arrow keys and Enter, or click an action. Run `:dashboard` to return after closing the tab.

## Shortcuts

| Action | Vim |
| --- | --- |
| Quick Open / Go to File | Ctrl+P |
| Command Palette | : or Space p |
| Focus sidebar | Space e |
| Open editable directory popup | Space x |
| Toggle sidebar / close explorer | Space s / Space S |
| Search project | Space / |
| Split right / switch panes / close split | Ctrl+W v / w / c |
| Go to definition / references / hover | g d / g r / g h |
| Go to type definition | g y |
| File problems / project problems | Space d / Space D |
| Notification history | Space N |
| Next / previous problem | ] d / [ d |
| Toggle comment on current / selected lines | gcc / gc in Select mode |
| Find / till character forward or backward | f / F / t / T, then the character |
| Repeat last character find / reverse it | ; / , |
| Switch open buffer | Space b |
| Save | Ctrl+S |
| Manual completion | Ctrl+Space in Insert |
| Choose a completion | ↑ / ↓ or Ctrl+N / Ctrl+P |
| Accept or dismiss a completion | Tab, Enter or Ctrl+Y / Esc |
| Search the current buffer | /, then n or N to repeat |
| Find and replace | Space h |
| Undo / redo / repeat edit | u / Ctrl+R / . |
| Markdown preview | Space m |
| Symbols in file / in workspace | Space o / Space O |
| Fold / unfold, go to implementation | z a / z c / z o, g i |

Completion opens below the cursor and moves above it when more room is available there and the list would otherwise be clipped. It opens only after you type a letter, not after punctuation such as `{` or `,`, and the list narrows as you keep typing, matching letters in order like VS Code. Accepting a function, method or constructor adds `()`. The cursor goes after the brackets when the server's signature shows no parameters, and between them otherwise, or when there is no signature to check. In the second case typing `)` steps over the closer. In Insert mode `Ctrl+N` also opens the list, and inside a string that starts with `./`, `../`, `/` or `~/` it completes file names. `Ctrl+P` still opens Go to File when no list is showing.

The defaults use no Option, Command, function keys, or Ctrl+Shift chords. Space, g, and Ctrl+W start chords in Normal and Select modes. Ctrl+P and Ctrl+S work in every mode. Ctrl+Space starts completion in Insert. Press Esc to leave Insert before using a Space chord. Inside a picker, type to filter, use ↑/↓ to navigate, and press Enter or Esc. The input line of a picker or prompt (Rename Symbol, a file rename, the palette) has a caret: ←/→ or Ctrl+B/Ctrl+F move it, Home/End or Ctrl+A/Ctrl+E jump to the ends, Alt+←/→ (or Ctrl+←/→) move by word, Backspace and Delete (Ctrl+D) erase around it, Alt+Backspace or Ctrl+W (Ctrl+Backspace) erase the word before it, Alt+Delete the word after it, Ctrl+U clears what is before it and Ctrl+K what follows. Holding Shift with any of the arrow, word, Home and End keys selects; typing, pasting, Backspace and Delete then act on the selection, and an arrow without Shift drops it at that side. The mouse works there too: click to place the caret, Shift+click or drag to select, double-click for a word and triple-click for the whole line. The find bar's Find and Replace rows behave the same way, and text it is opened with starts selected, so typing replaces it. In the desktop app on macOS, Command+←/→ go to the ends (with Shift they select), Command+Delete clears to the start and Command+Fn+Delete to the end, and Edit ▸ Select All, Copy, Cut and Paste (⌘A, ⌘C, ⌘X, ⌘V) work on the field while a picker or the find bar is open. Ctrl+Backspace in Insert mode deletes the word before the cursor. Search for **Show Keybindings** in the command palette to inspect the active mode.

In Vim, `f.` moves to the next period, `;` repeats that move, and `,` finds a period in the opposite direction. Counts work too: `3;` repeats the find three times. Character finds stay on the current line and also work in Select mode. `.` repeats the last edit.

Vim supports counts, text objects, macros, and a single selection. A typing burst and a macro replay in one buffer each form one undo step.

### Familiar keys

Besides the Vim keys, the usual editor shortcuts work in every mode: `Ctrl+F` find, `Ctrl+Z` / `Ctrl+Y` undo and redo (without leaving Insert), `Ctrl+A` select all, `Ctrl+B` toggle the sidebar, `Ctrl+Q` quit, `PageUp` / `PageDown`, `Home` / `End`, `Ctrl+Home` / `Ctrl+End`, and `Ctrl+←` / `Ctrl+→` by word. In Insert, `Home` goes to the first character, then to column 0, and `Shift+Tab` outdents the line. In Select mode `Ctrl+C` copies and `Ctrl+X` cuts. Remap any of them under `bindings`.

### Indentation settings

`tab_size` (1–16, default 4) sets how wide a tab is drawn and how many spaces make one indent. `indent_style` is `"auto"` (tabs in Go and Makefiles, spaces elsewhere), `"tabs"` or `"spaces"`. `Tab` in Insert follows it, inserting spaces up to the next tab stop when spaces are in use. The **Open Config File** command opens (and creates, if needed) your config.

### Settings, reload and project config

**Space ,**, **Settings** in the palette or `⌘,` opens the settings. A change is applied and saved at once.

The desktop app has a settings window. Type to search all of it, move with the arrows, change with `←` and `→`, switch pages with Tab. A dot marks what you changed; **Reset** (or Delete) goes back to the default. The terminal shows a list of the settings that mean something in a terminal.

| Page | Settings |
| --- | --- |
| Appearance | Default color theme (`theme`), animations (`animations`) |
| Fonts, code | `font_family`, `font_size`, `font_weight`, `font_bold_weight`, `line_height`, `letter_spacing`, `font_ligatures`, `italic_comments`, `font_fallback` |
| Fonts, interface | `ui_font_family`, `ui_font_size` (scales the whole interface), `ui_font_weight` |
| Editor | `line_numbers`, `relative_numbers`, `highlight_line`, `indent_guides`, `tab_size`, `indent_style`, `scrolloff`, `sticky_scroll` (breadcrumbs), `smooth_scroll`, `auto_complete`, `auto_pairs`, `hover_docs` |
| Cursor | `cursor_normal`, `cursor_insert` (`block`, `bar` or `underline`), `cursor_blink`, `cursor_glide` |
| Interface | `path_bar`, `status_bar`, `scrollbar` |
| Files | `format_on_save`, `organize_imports`, `reopen_project`, `restore_session` |
| Languages | Tab size, indent style and format on save per language, and whether its server and formatter are installed |
| Keyboard | The keys of every command in Normal, Visual and Insert mode. Click one and press the new keys. `⌘` shortcuts belong to the menu bar and can't be changed. |

Settings live in `~/.config/loom/`, in two files read in this order:

- `config.json`: your config. The settings window writes here only what Loom 0.17 already understood.
- `settings.json`: settings added since, such as fonts. An older `loom` on the same machine doesn't read it, and would refuse to start if these keys were in `config.json`. You can still write any key in either file yourself; `settings.json` wins.

A file you save inside Loom reloads automatically, and **Reload Config** does it on demand. A broken file is reported and the running settings stay; nothing writes over a file that isn't valid JSON. If the desktop app can't read a file when it starts, it says so and starts with the rest. Plugins are read only at startup.

Code fonts must be monospaced. Fonts installed while Loom runs show up the next time you open the settings. Ligatures need a letter spacing of 0.

Settings can be set per language, and per project:

```json
{
  "tab_size": 4,
  "languages": {
    "python": { "tab_size": 4, "indent_style": "spaces", "format_on_save": false },
    "go": { "indent_style": "tabs", "tab_size": 8 }
  }
}
```

A `.loom.json` in the project root is read on top of your config, and what it sets is marked **Project** in the settings. Because a repository is not trusted, it may change settings, per-language settings and bindings, but not language servers, formatters or plugins; such a file is rejected. Closing or switching projects drops its settings.

### Alt keys

Alt (Option) combinations are now recognised, so you can bind `alt+up`, `alt+x` and so on. By default `Alt+↑` / `Alt+↓` move the line or selected lines, `Alt+←` / `Alt+→` move by word and `Alt+Backspace` deletes a word. Terminals send Alt as an Escape prefix; on macOS, set Option to "Use as Meta" in your terminal so it is not turned into special characters.

### Ex ranges: substitute, global, delete

`:[range]s/pattern/replacement/[flags]` substitutes. Ranges are `%` (whole file), `N`, `N,M`, `.`, `$`, marks such as `'a`, and offsets like `.,+3`; with a selection the prompt starts with `'<,'>`. Flags are `g` (every match on a line) and `i` (ignore case). Patterns use Go regular expressions, so groups are `(…)`; the replacement takes `&` (whole match), `\1`–`\9`, `\n` and `\t`. `:g/pattern/d` deletes matching lines, `:g/pattern/s/a/b/` substitutes only on them, `:v/pattern/d` acts on the lines that do not match, and `:[range]d` deletes lines. Each command is one undo step and reports how many changes it made.

### Marks

`m{a-z}` sets a mark, `'{a-z}` jumps to its line and `` `{a-z} `` to the exact spot; `''` jumps back. Marks follow their text as you edit. `'<` and `'>` are the last selection.

### Registers and clipboard history

`"{name}` before a yank, delete or paste picks a register: `a`–`z` keep text to themselves (capitals append), `0` holds the last yank, `1`–`9` the last deletes, `"+` and `"*` are the system clipboard, and `"_` throws the text away without touching anything. **Space y** opens the registers and the history of the last 30 copies; pick one to paste it.

### Visual block

`Ctrl+V` selects a rectangle. `y`, `d`, `x` and `c` act on it; `I` and `A` insert at its left or right edge and repeat what you type on every line when you press Esc. Pasting a block pastes its lines as ordinary text.

### Breadcrumbs

The chain of functions and blocks around the cursor, like `Server › Handle › if ok`, follows the file in the desktop app's path bar and is in the status line in a terminal. Scopes come from indentation, so it works in every language. Turn it off with `"sticky_scroll": false` or in Settings (the key keeps its old name so existing configs still load). Nothing is pinned over the code.

### Recent files, jump list, project replace

**Space F** lists recently opened files in this project, and they also come first in Go to File (**Ctrl+P**). **Space j** lists the jump list, newest first. **Space H** replaces across the project: type what to find (literal text, or `re:` for a regular expression; ignores case unless you type a capital), then the replacement. A list shows every matching line; the first entry replaces them all. Files open in a buffer are edited in place, so you can undo and nothing is saved for you; other files are written directly. Binary, hidden, `node_modules`, `vendor` and `target` files are skipped. Regular expression replacements use `$1` for groups.

### More Vim motions

`*` and `#` search for the word under the cursor, `{` and `}` move by paragraph, `H` `M` `L` jump to the top, middle and bottom of the window, `Ctrl+D` / `Ctrl+U` scroll half a page, `zz` `zt` `zb` place the cursor line, `gv` reselects the last selection, `~` toggles case, and in Select mode `~`, `U` and `gu` / `gU` change case. `Ctrl+C` leaves Insert mode.

### Look and feel

- **Tab bar:** NvChad-style tabs with a file icon, the name and a close button; the active tab sits on the editor background, unsaved tabs show a yellow dot, and `‹` / `›` appear when tabs scroll out of view.
- **Framed popups:** hover and diagnostic boxes, completion, function signature hints, which-key and menus all sit in their own rounded frame. A tall hover box now opens on the roomier side of the pointer, never covers it and never grows past half the screen, so moving the mouse away always dismisses it.
- **Smart Enter:** Enter after `{`, `(` or `[` indents the new line one level deeper (also after a Python block colon and Go `case`/`default:` labels), and between a matching pair (`{}`, `()`, `[]`) it opens an indented middle line with the closer below. `o` follows the same rule.
- **Pane borders:** the dividers around the pane you are working in are drawn in the accent color, and the divider you are dragging turns heavy.
- **Animations:** resizing a pane with `Ctrl+W > < + - =`, evening out with a double-click, or opening a split glides over about 160 ms instead of jumping. Dragging with the mouse follows the pointer exactly. Turn it off with `"animations": false` (or in Settings) for reduced motion.
- **Indent guides:** faint vertical lines mark each indent level in leading whitespace; `"indent_guides": false` hides them.
- **Dashboard:** a block-letter logo, the project and git branch, and icon buttons with their shortcuts, followed by your five most recent files. Move with `j`/`k` or the arrow keys, open with Enter or a click.
- **Status line:** NvChad layout. A mode block coloured by mode (blue normal, purple insert, cyan visual, orange replace, yellow prompt/explorer), the editing profile, file and problems in the middle, and on the right the git branch, language, a folder block with the project name and a coloured cursor-position block. The icons use a Nerd Font.

### Desktop app

`sh packaging/macos/build-app.sh` builds `dist/Loom.app`; pass a folder to build somewhere else. The icon, two pairs of threads woven into an L, is drawn in code by `internal/brand`; the logo inside the app is the same drawing, simplified at small sizes. `go build -tags desktop,production -o loom-desktop ./cmd/loom-desktop` builds just the binary (git ignores it). It is the same editor, with the same keys, themes and config as the terminal version, in a native window whose interface is drawn by the system web view ([Wails](https://wails.io); WebKit on macOS and Linux). Go keeps the editor and lays the code out on a character grid as the terminal does; each change sends the rows that changed and the state of everything around them to the page, which draws them with HTML and CSS and sends keys, typing (input methods included, with their candidates at the cursor), clicks and the wheel back. Keys reach the page through a hidden text field; macOS's typing helpers (Writing Tools and the Siri writing suggestions, inline predictions, corrections and substitutions) are turned off for Loom, the first two in the web view itself and in AppKit's small writing panel, which otherwise shows up beside the field (in a corner of the screen while the field is off screen), so they don't pop up, and the field only moves to the cursor while an input method is composing.

The status bar's segments (encoding, line endings, language and the cursor position, which is a button) share one line. The title bar, sidebar, tabs and status bar are translucent, so on macOS the desktop shows through them, blurred, in dark or light glass to match the theme. Pickers, popups, panels and dialogs are glass too: they blur the code under them.

- **Title bar:** the project and how many sessions are open (click to switch), the branch (click to switch), a search field, and buttons for Source Control, the Command Palette and Settings.
- **Sidebar:** the file tree with git state, buttons for a new file, a new folder and filtering. Right-click for actions; drag its edge to resize it. Drag a file or folder onto a folder to move it there, or onto a file to move it beside that file; a closed folder the drag rests on opens, and Esc cancels. Open tabs follow what they show.
- **Selecting with the mouse:** dragging a selection past the top or bottom of the text (or its sides) scrolls after a short pause, slowly just past the edge and faster the further out the pointer goes. In a terminal, which ends at its last row, resting on the pane's outermost row does it.
- **Tabs and path:** drag tabs to reorder them or onto the text to split, middle-click or × to close. A tinted bar beneath shows the file's path and the functions and blocks around the cursor.
- **Pickers and panels:** the palette, file search with a highlighted preview, settings with switches, Source Control with stage and commit buttons, and a diff with both line numbers.
- **Messages** appear as a toast at the bottom right. Click one for the history.
- **Unsaved work:** closing a tab with its button, closing the window and quitting all ask first.

| macOS shortcut | Action |
| --- | --- |
| `⌘P` / `⇧⌘P` | Go to File / Command Palette |
| `⌘O` / `⇧⌘O` | Open a file / a project with Finder |
| `⌘K` | Switch session |
| `⌘N`, `⌘S`, `⇧⌘S`, `⌘W` | New file, Save, Save As, Close Tab |
| `⌘Z` / `⇧⌘Z` | Undo / Redo |
| `⌘X`, `⌘C`, `⌘V`, `⌘A` | Cut, Copy, Paste, Select All |
| `⌘F`, `⌥⌘F`, `⇧⌘F` | Find, Replace, Find in Project |
| `⌘B`, `⇧⌘G`, `⇧⌘M` | Sidebar, Source Control, Problems |
| `⌘L`, `⌘,` | Go to Line, Settings |
| `⌘=`, `⌘-`, `⌘0` | Code font size, saved |
| `⌘←` / `⌘→` | Start / end of line |

On macOS, Command belongs to the menu bar and Control to Vim, so `⌘D` is not `Ctrl+D`. Control shortcuts also work on a Russian keyboard layout. Option acts as Alt.

Interface text uses the system font and code Menlo until you pick others in Settings → Fonts, whose pickers list the installed families the window finds among common code and interface fonts (weights 300 to 800; a weight a font lacks is made up). Secondary text and syntax colors are nudged where a theme would make them unreadable. Started from Finder, Loom takes `PATH` from your login shell so language servers and formatters are found. Set `"animations": false` to turn off the gliding cursor and scrolling.

The app is signed ad hoc, not notarized. It has only been used on macOS. The Windows build does not compile yet (`recovery.go` uses a Unix call), and the Linux desktop build is untested.

#### Remote sessions over SSH

The desktop app can show an editor that runs on another machine, as VS Code and Zed do: the files, git, project search, language servers and formatters are all on the server, and the window, the clipboard and the keyboard are here. Connect with `loom ssh`, which is `ssh` with the desktop app's socket forwarded to the server, then open what you want there:

```sh
loom ssh me@build-box          # takes every ssh argument
loom-desktop .                 # on the server: a new Loom window here, for that folder there
loom-desktop src/main.go       # or for a file
```

The server needs only the terminal `loom` (the install script and the Homebrew formula on Linux also make `loom-desktop` a link to it); WebKit is not needed there. The Loom app must be open here. Each remote session gets a window of its own, with the server's name in its title bar; the editor uses the server's own Loom config. Copying puts text on this computer's clipboard and pasting takes it from there. File panels are replaced by a path prompt, since they would browse the wrong computer. Closing the window ends the editor on the server, asking first about unsaved files; if the SSH connection drops, the window closes.

To use plain `ssh` instead, add the forward to `~/.ssh/config`; `loom ssh-config` prints the lines with this computer's socket filled in. On the server, `loom-desktop` finds the socket through `LC_LOOM_SOCKET` (passed when the server accepts `LC_*` variables, as most do) or by looking for `/tmp/loom-fwd-*.sock` of your user, the newest first. Outside such a session, a `loom-desktop` that is only the terminal `loom` says how to connect.

#### Opening files and projects from a terminal

The desktop app is a single window. Starting it again with a path hands the path to the window that is already open and exits at once:

```sh
loom-desktop ./notes.txt       # a file opens in the project it belongs to, or in the shown one
loom-desktop ~/code/api        # a folder opens as a session, or shows the one it already has
loom-desktop                   # just brings the open window forward
loom-desktop -new ./notes.txt  # a window of its own (-n works too)
loom-desktop -foreground .     # keep the terminal attached and print the log there (-f works too)
```

Started from a terminal, `loom-desktop` runs in the background: the prompt comes back after a moment and the log goes to `desktop.log` in the state folder (`~/.local/state/loom`), which each start replaces. If it cannot start, because of a bad flag or a config it cannot use, the message is printed in the terminal and the exit status is 1. Use `-foreground` to debug, or when a service manager wants the process to stay. Started from Finder or a launcher it has no terminal and nothing changes.

A file goes to the open session whose folder holds it (the deepest one), and the window comes to the front. A folder that is not open yet becomes a new session. If no window is open, the first launch opens one, and later launches find it. Relative paths and `~` count from where you typed the command. The hand-over uses a socket named `desktop.sock` in Loom's state folder (`~/.local/state/loom`), readable only by you; a window started with `-new` does not take it. With `-config` a launch always opens its own window. On macOS the binary inside the bundle is `Loom.app/Contents/MacOS/loom`, so a shell alias or a symlink to it gives a command: `ln -s /Applications/Loom.app/Contents/MacOS/loom ~/.local/bin/loom-app`. `open -a Loom file` only brings the app forward; it does not pass the file.

### Touchpad scrolling

A swipe scrolls along one axis only. The first direction of a swipe wins, and the sideways drift of your finger is ignored until the wheel pauses for about 150 ms. This applies everywhere that scrolls: editors, the explorer, pickers and previews.

### Scrolling tabs

When there are more tabs than fit, scroll the wheel or touchpad over the tab bar (either axis) to slide through them without switching buffer. Switching to another tab brings it back into view.

### Vim compatibility

| Keys | Action in the Vim profile |
| --- | --- |
| `V`, `Vj` | Select the current line or extend the selection across two lines. Use `d`, `c`, or `y` to operate on whole lines. |
| `v` / `V` in Visual mode | Switch selection type or exit when the type already matches. |
| `o` in Visual mode | Move the cursor to the other end of the selection. |
| `p` / `P` in Visual mode | Replace the selected text. `P` preserves the unnamed register. |
| `>>` / `<<`, `>` / `<` + motion | Indent or outdent lines. Counts work, as in `3>>`. |
| `>` / `<` in Visual mode | Indent or outdent the selected lines. The selection stays active until you press Esc. `3>` shifts three times. |
| `za` / `zc` / `zo` | Toggle, close, or open the fold at the cursor. Folds follow indentation, and a closing `}` joins its block. `j` and `k` skip folded lines. |
| `zM` / `zR` | Close or open every fold in the file. Folds reset when lines are added or removed. |
| `g i` | Go to implementations. A single result jumps straight there. |
| `space o` / `space O` | Pick a symbol in this file, or in the whole workspace. Workspace results update as you type. |
| Typing `./`, `../`, `/`, or `~/` inside a string | Complete file and folder names from disk. Folders come first and end with `/`. |
| `j` / `k` |  Keep the wanted column across short lines. After `$` they stay at the end of every line, including empty ones. |
| `^`, `3gg`, `3G` | Move to the first nonblank character of the current line or line 3. |
| `f` / `F`, `t` / `T`, `;` / `,` | Find a character on the current line, stop just before it, or repeat the last find in either direction. |
| `%`, `50%` | Jump between matching parentheses, brackets, or braces, or jump to a percentage of the file with a count. |
| `D` / `C` / `Y` | Delete to end of line, change to end of line, or yank whole lines. |
| `r`, `J` | Replace a character or join lines. |

Vim features still missing include blockwise Visual mode (`Ctrl+V`), `gv`, named registers and marks, Replace mode (`R`), `W`/`B`/`E` WORD motions, paragraph motions, the `:s` substitute command, Ex ranges, and Vimscript.

### Surround

| Keys | Action |
| --- | --- |
| `saiw)` | Wrap the word in parentheses. |
| `sa_)` | Wrap the current line. |
| `sa)` in Visual mode | Wrap the selection. |
| `sd)` / `sr)]` | Delete parentheses or replace them with brackets. |
| `sf)` / `sF)` / `sh)` | Jump to the right or left delimiter, or briefly highlight both. |

An opening bracket as the output adds spaces, as in `saiw(` → `( word )`. The closing bracket gives `(word)`. For deletion or replacement, an opening bracket also removes padding inside the pair. Delimiter IDs include brackets, quotes, and single-character pairs. The lookup uses an enclosing pair. Function, tag, and custom surround definitions are currently unavailable. The existing `ys`, `ds`, `cs`, and Visual `S` mappings also work.

### Split and join arguments

Press `gS` inside a bracketed argument or item list, or select **Toggle Split Join** in the command palette. A single-line list expands to one item per line. A multiline list joins onto one line. Nested groups and quoted strings keep their own commas. Go lists get a trailing comma when expanded. Lists containing comments, ambiguous syntax, or multiline items can be skipped.

## Normal-mode shortcuts

Both editing profiles use the same leaders. Press Space, g, or Ctrl+W to see continuations in which-key. Ctrl+Space completes in Insert.

| Keys | Action |
| --- | --- |
| Space f / Space / | Quick Open / project search |
| Space e / Space x | Focus sidebar / editable directory popup |
| Space b / Space w | Buffers / save |
| Space a / Space r / Space d | Code actions / rename symbol / file problems |
| g d / g r / g h | Definition / references / hover |
| g y | Type definition: the declaration of the symbol's type, such as `ent.User` for a variable `user` |
| Ctrl+O / g ] | Previous / next jump |
| gg | Top of file |
| Space m | Toggle Markdown preview |
| Space o / Space O | Symbols in this file / workspace |
| g i | Go to implementation |

Definition jumps, reference results, project search results, and Ex line addresses enter jump history. Taking a new jump clears the forward history.

## Tabs and panes

Tabs are numbered in the tab bar. A split opens another view of the current buffer, with its own cursor and scrolling. Both views share text and undo. Tab commands change the buffer in the active pane.

| Keys | Action |
| --- | --- |
| `gt` / `gT` | Next / previous tab |
| `Space t 1` … `Space t 9` or `Ngt` | Go to tab number |
| `Space t h` / `Space t l` | Move tab left / right |
| `Space t 0` / `Space t $` | First / last tab |
| `Space t x` | Close buffer tab |
| `Ctrl+W v` / `Ctrl+W s` | Split right / below |
| `Ctrl+W h/j/k/l` or arrow keys | Focus a pane by direction |
| `Ctrl+W w` | Cycle panes |
| `Ctrl+W >` / `Ctrl+W <` | Make the active pane wider / narrower. A count works, as in `3 Ctrl+W >` |
| `Ctrl+W +` / `Ctrl+W -` | Make the active pane taller / shorter |
| `Ctrl+W =` | Make every split even again |
| `Ctrl+W c` / `Ctrl+W o` | Close active pane / keep only active pane |
| `Space z` | Toggle Zen mode |
| `Space n` | Toggle relative line numbers |
| `Space q` | Close active pane |

Drag the divider between two panes with the mouse to resize them, and double-click it to even the split out. Panes keep a minimum size, and sizes are saved with the session.

Ex commands include `:split [file]`, `:vsplit [file]`, `:close`, `:only`, `:tabnext [number]`, `:tabprev`, `:tabfirst`, `:tablast`, `:tabmove [+/-]N`, and `:tabclose`. **Close Pane** (`:close`) closes the active tab when only one pane is open. **Close Tab** (`:tabclose` or `:close tab`) closes the active buffer tab. A bare `:tabmove` moves the tab to the end. An unsigned position is zero-based. `:q` closes the active pane when several are open. `:qa` quits the editor once no session has unsaved changes; `:qa!` quits and discards them in all of them. Set line numbers with `:set rnu`, `:set nornu`, or `:set rnu!`. Relative numbering keeps the current line's absolute number visible.

Zen mode shows the active pane with a centered text area and the status line. Toggling it off reveals the same split layout, tabs, and sidebar.

### Selecting past the edge

While you drag a selection with the mouse, resting the pointer on or beyond the top, bottom, left or right edge of the pane scrolls the text so you can keep selecting. The further out the pointer, the faster it scrolls, and it keeps going while you hold still.

## Notifications

Press **Space N**, run `:messages`, or click the status bar to open **Notification History**. It shows the latest 200 messages from the current session, newest first. Type to filter and press Enter or click a result to read its full text. Use the arrow keys, Page Up and Page Down, or the mouse wheel to scroll. Esc returns to the list with the filter and selection preserved.

## Automatic pairs

In Insert mode, typing `(`, `[`, `{`, a single or double quote, or a backtick can insert its matching closer and leave the cursor between the two characters. Type the generated closer to move past it, or press Backspace inside an empty generated pair to remove both characters. In Select mode, type an opening delimiter to wrap the selection. Paste inserts its original text. Set `"auto_pairs": false` in your config to turn this off.

Press Enter between adjacent braces to put the closing brace on its own line and place the cursor on a blank line inside. Loom adds one tab in Go or in a tab-indented block, and four spaces elsewhere.

## Find and replace

`/` opens a find panel at the top right of the pane, and `Space h` (or **Find and Replace** in the palette) opens it with the replace row. Matches highlight as you type, the current one brighter, and the cursor jumps to the nearest match. The panel shows `3 of 12`, `No results`, or `Invalid` for a bad regex. Selected text, or for `Space h` the word under the cursor, is prefilled and the first keystroke replaces it.

| Key | Action |
| --- | --- |
| Enter | Next match. After `/`, Enter keeps the first match, closes the panel and leaves `n` / `N` working. In the replace field it replaces the current match and moves to the next |
| Down / Ctrl+N, Up / Ctrl+P | Next / previous match |
| Tab | Switch between the find and replace fields |
| Ctrl+T / Ctrl+O / Ctrl+E | Match case / whole word / regular expression |
| Ctrl+A | Replace all, as one undo step |
| Ctrl+R | Show or hide the replace row |
| Ctrl+U / Ctrl+W | Clear the field / delete the last word |
| Esc | Close the panel and clear the highlights |

Search ignores case until you turn on match case. In regex mode `^` and `$` match at every line, and the replacement can use `$1` or `${name}` groups. The buttons in the panel (`Aa`, `ab`, `.*`, arrows, Replace, All, `×`) work with the mouse too.

## System clipboard

Yank, delete and change copy to the system clipboard, and paste reads it, so text moves freely between Loom and other applications. Loom uses `pbcopy`/`pbpaste` on macOS, `wl-copy`/`wl-paste`, `xclip` or `xsel` on Linux, and `clip.exe` on Windows. It also sends an OSC 52 sequence, which reaches your local clipboard over SSH in terminals that allow it. Text copied elsewhere pastes as plain characters; text you yanked in Loom keeps its line-wise behaviour. Pasting from another application with the terminal's own paste shortcut works as before.

## Markdown preview

`Space m` (or **Toggle Markdown Preview** in the palette) opens a rendered preview in a split beside a Markdown file. It redraws as you type and scrolls to follow your cursor; the block under the cursor gets an accent bar. The wheel scrolls it by hand until the cursor moves again. The preview never takes focus, and `Space m` again closes it. It renders headings, emphasis, inline code, links, lists with task boxes, quotes, tables, rules and highlighted code fences.

## Languages

Built-in language entries, with a language server and formatter used only when they are on `PATH`: Go, [Dingo](https://github.com/MadAppGang/dingo) (`.dingo`, the meta-language that compiles to Go; `dingo-lsp`, with its own highlighter for `enum`, `match`, `=>`, `->`, `?`, `?.` and `??`; there is no formatter entry, as `dingo fmt` is not set up for stdin). `dingo-lsp` passes gopls's edit positions on without translating them to the `.dingo` file, so Loom ignores the range of a completion unless it sits on the cursor's line and does not apply the server's extra edits in `.dingo` files; when one of them adds an import, Loom adds that import to the `.dingo` file itself. `dingo-lsp` also answers from the Go file it last generated, so a package that is not imported yet gets nothing from it. When the server has no completions after `name.`, Loom in a `.dingo` file offers the exports of the standard package of that name (read with `go doc`, so `go` must be on `PATH`), and accepting one adds the `import` to the file you are editing, into an `import (…)` block when there is one. For a word that is not after a dot it also adds the Go and Dingo keywords and, from two letters on, the names of standard packages that are not imported yet (`pac`, `strin`); accepting a package adds its import. What `dingo-lsp` itself does not offer stays missing: its own list of abilities (version 0.1.0) has completion, hover, definition, references, implementations, symbols, call hierarchy, code actions and formatting, and no signature help, rename or semantic colors, TypeScript, JavaScript, Python, Rust, C, C++, Java (`jdtls`), Kotlin, Scala, Swift, C#, PHP, Dart, Zig, Haskell, Elixir, OCaml, Ruby, Perl, R, Lua, assembly (`.asm`, `.s`, `.nasm`, `.inc`; `asm-lsp`), Dockerfile, Makefile, CMake, Protobuf, GraphQL, Terraform, XML, shell, SQL, JSON, YAML, TOML, CSS, HTML and Markdown. Files such as `Dockerfile` and `Makefile` are recognised by name. Every language gets syntax colors and `gcc` comment toggling even without a server. Elixir (`.ex`, `.exs`; [Expert](https://github.com/elixir-lang/expert), the official language server, started as `expert --stdio`; formatted with `mix format -`) has its own highlighter for atoms, sigils, `@attributes`, `@doc` heredocs, `?`/`!` names, keyword keys and pipelines, indents after `do` and `->`, and names `defmodule`/`def`/`defp` in the path bar; `.heex`, `.eex` and `.leex` templates are recognised too. In the desktop app Elixir files get an `EX` file icon (`<>` for templates). Add or override entries under `languages` in your config.

## Color themes

Pick the default theme in **Settings → Appearance**, or run **Color Theme** from the command palette. Either saves it as `theme` in your config. Built in: nvchad (default, the NvChad One Dark palette), everforest, gruvbox, nord, dracula, tokyo-night, catppuccin, one-dark, monokai, solarized-dark, solarized-light, and terminal. The `terminal` theme draws with your terminal's own background and 16 ANSI colors, so it follows whatever palette the terminal uses. A project can set its own with `theme` in its `.loom.json`.

## File explorer and mouse

The sidebar is a collapsible project tree. Each row shows a folder or file-type icon from Material Design Icons in Nerd Font, so the terminal needs a Nerd Font Mono variant. `Space e` focuses the sidebar. `Space x` opens an editable directory buffer in a centered popup. It starts beside the active file, or at the project root for an untitled buffer.

Edit the listing with Vim motions. For example, `cc` replaces a filename, `o` adds a new entry, and `dd` removes an entry.

- Edit a filename in place to rename it. Enter a relative path such as `src/main.go` to move it into an existing folder.
- Add a line to create a file. End the name with `/` to create a directory.
- Delete a line to move the entry to Trash.
- Press Enter on an entry to open it. The first row is `../`. Enter on that row or press `-` to go up. Use `go` for the directory picker, or type `:cd ../sibling`, `:cd ~/projects`, or an absolute path. This changes the popup directory. Project search and the sidebar keep their original project root.
- Use `:w` or Ctrl+S to review changes. Enter applies the plan. Esc cancels the review and keeps the edits. `:q!` discards unapplied edits.

The sidebar has its own navigation and file actions:

| Key | Action |
| --- | --- |
| j / k, gg / G | Move down or up, jump to top or bottom |
| h / l | Collapse or go to parent, expand or enter |
| Enter / o | Toggle a folder or open a file |
| a / A | Create a file / folder under the selected folder |
| r | Rename the selection. Ctrl+U clears the prefilled name |
| m | Move to a project-relative path. An existing directory keeps the current name |
| x, then p | Cut the selected entry, navigate to a destination, then move it there |
| d | Move to Trash after typing yes |
| / | Filter every file in the project by name, including collapsed folders. Enter keeps the filter, Esc clears it |
| s | Cycle the sort order: name, modified, size, type. Folders stay first |
| R / ? | Refresh / show help |
| Esc / q | Return to the editor |

Drag a tab onto a pane to move it. Near the left, right, top or bottom edge a highlight shows the split it will make, and the middle opens the tab in that pane. Drag a tab along the tab bar to reorder it. Click a tab or explorer entry to open it. Click in the editor to place the cursor, drag to select, or double-click to select a word. The wheel scrolls the pane or explorer under the pointer. Scrolling does not move the cursor: when its line leaves the window the cursor is not drawn, and the view returns to it on the next key press. Drag the sidebar divider to resize it. Right-click for editor or file actions. Completion and picker rows are clickable.

## Command palette

Press `:` or `Space p` to open the same searchable palette. Type an exact Ex command or filter by the command name. Each entry shows its shortcut on the right, followed by the matching Ex command when there is one. Use `:42` or `:goto 42` to jump to a line, `:$` for the last line, `:noh` to hide search highlights, and `:e path` to open a file. `n` and `N` continue the last search after `:noh`.

Popups are centered and take their colors from the active theme. Modal prompts dim the workspace behind them. Start a chord such as Space, g, or Ctrl+W to see available continuations. The status bar shows the active mode and file, unsaved edits, diagnostics, cursor position, and file format.

## Git

Git changes made outside Loom show at once: when a commit, a checkout, a pull or a stash from a terminal touches the repository, the file marks, the changed-line gutter and the Source Control panel follow within a second (Loom watches the few files git changes for these, so it costs nothing on a large repository). Edits other programs make to files that aren't open are picked up every few seconds where `git status` is quick. The branch shows how far it is from its upstream, like `main ↑2 ↓1`, in the title bar, the status line and the panel.

Git commands are in the command palette: **Git: Commit…** (what is staged, or every change when nothing is), **Commit All…**, **Amend Last Commit…** (starting from the last message), **Push** (which publishes a branch that has no upstream yet), **Pull**, **Pull (Rebase)**, **Fetch** (all remotes, pruning), **Sync** (pull with a rebase, then push), **Stash Changes**, **Pop Stash**, **Create Branch…**, **Stage This File** and **Unstage This File**, besides **Switch Branch**. Network commands run in the background and say how they went; files they change reload. In the Source Control panel `f` fetches and `S` syncs, next to `c` commit, `p` push and `P` pull, and the desktop panel has Fetch, Pull, Push, Sync and Commit buttons.

The status bar shows the current branch. Git commands run with the `git` on your `PATH`.

| Keys | Action |
| --- | --- |
| Space g g | Source Control popup |
| Space g d | Diff of the current file against HEAD |
| Space g b | Switch branch |

In Source Control, rows are grouped into Staged, Changes, and Untracked. Keys:

| Key | Action |
| --- | --- |
| j / k, g / G | Move, top, bottom |
| s / Space | Stage the file, or unstage it in the Staged group |
| a / A | Stage all, unstage all |
| d | Discard changes to a file in Changes, after typing `yes` |
| Enter | Open the file's diff |
| c | Commit the staged files with a message |
| p / P | Push, or pull with `--ff-only` |
| b | Switch branch |
| R / Esc | Refresh / close |

The diff popup shows the file's changes against HEAD. Use `]c` and `[c` to move between hunks, and Enter to open the file at the hunk. Diffs use the saved file, not unsaved edits. Push and pull never prompt for credentials, so set up a credential helper or SSH key first. Untracked files can't be discarded here. Delete them in the explorer instead.

## Project tools

Project search uses `rg` from `PATH`. It shows the matching line and jumps to the match on Enter. Git status marks modified and untracked files in the explorer, and the active file's gutter shows line changes against HEAD. The command palette includes **Refresh Git Status**.

Formatting runs on save when `format_on_save` is on, which is the default. Go uses gofmt, or goimports when `"organize_imports": true`. Other languages use the `format` command in their `languages` entry, with the buffer text on stdin. A formatter that is not installed is skipped, so saving still works. If a formatter reports an error, Loom still saves the current text and shows the error. Set `"format_on_save": false` to turn formatting off. Project sessions restore saved files, cursor positions, splits, and sidebar state. Set `"restore_session": false` to disable session restore.

## Code colors and language tools

Loom reads languages from the `languages` table in your config. Each entry names a language by its LSP language ID and sets its file extensions, language server, formatter, and project root markers. The built-in table is listed under Languages above. Run `loom -dump-config` to see every default.

A language server or formatter is used only when its command is on `PATH`. Missing tools are skipped without an error. Go gets the full set: gopls, syntax colors, and gofmt or goimports. For other languages, the language server adds completion, hover, definition, references, diagnostics, rename, and code actions, and it also provides semantic colors. Syntax colors without a server are built in for every language listed above. Completion inserts plain text, apart from the `()` added after functions. The popup opens by itself after a letter, and after a `.` that follows a name or a closing bracket (`IO.`) when the server lists `.` as a trigger. Servers that send functions as snippets, such as Expert (`puts(${1:item})`), are handled by inserting the name and the brackets; other snippets are skipped. Elixir projects are rooted at the nearest `mix.exs`.

Use **Restart Language Server** in the command palette or run `:lsp-restart` from a file tab. Loom restarts that file's language server for its workspace and sends the current contents of its open buffers, including unsaved changes.

The status bar shows errors and warnings for the current file and for the whole project, as `file E0 W0 · project E0 W0`. The project count includes every file under the project root that the language server reports, whether or not it is open. Space D opens Project Problems, and Space d opens File Problems. Each popup lists errors and warnings with their file and line. Type to filter, for example `error` or a file name. Enter jumps to the location. Project counts depend on the server. gopls reports every file in the module, but a server that reports only open files gives a partial count. A language's server starts when you open a file of that language, so its files are counted from then on.

Diagnostics underline the reported range in the editor. Errors use red, warnings use amber, and information and hints use teal. In either Problems picker, the preview shows the full message, line and column, and nearby source with the same underline.

Hover over an underlined diagnostic or its gutter marker to read its full message. Hover over a symbol to see language server documentation after a brief pause; when the symbol has a diagnostic, both appear together. When the symbol has a type declared elsewhere (a variable `user` of type `*ent.User`), the documentation of that type follows its own, read from the type's file even when it is not open; in `g h` it is a separate **Type Documentation** entry. Messages wrap to the popup width; scroll over the popup for messages taller than the terminal. Hover Documentation also has a wrapped, scrollable preview.

In Problems, press Enter or click an item to jump to its source. Hover over the underlined code or press `g h` to inspect the diagnostic. The `g h` list offers diagnostic and documentation entries; select one to read its wrapped details. Notification History also opens scrollable details with Enter. Esc returns to the list.

Picker previews grow with the terminal. Use Page Up and Page Down or scroll over the preview to see more. In Insert mode, language servers that support signature help show the function signature near the cursor and highlight the active parameter as you type arguments.

File, buffer, search, and reference pickers preview source as you move. The Command Palette and Code Actions picker use compact lists. Selecting a code action that edits files opens a confirmation with the proposed changes. Completion shows details and documentation when there is room.

To add a language, or change one, list only what differs. Set `extensions` to claim file types, `server` for the language server, `format` for the formatter, and `roots` for extra project markers. The `{path}` placeholder is replaced with the file path. This example adds Nim and changes the Python formatter:

```json
{
  "profile": "vim",
  "auto_complete": true,
  "languages": {
    "nim": {
      "extensions": ["nim"],
      "server": ["nimlangserver"],
      "format": ["nimpretty", "{path}"],
      "roots": ["nim.cfg"]
    },
    "python": {
      "format": ["black", "-q", "-"]
    }
  }
}
```

Setting `extensions` replaces that language's list, and an extension belongs to one language. The older `language_servers` key still works and sets only the server command. Set `"server": []` to turn off a language server for one language.

## Code actions and rename

Use Space a at the cursor or on a selection. For rename, use Space r and enter a new name. Ctrl+U clears the suggested name. Review the affected files and text edits, then press Enter to apply them to open or newly opened buffers. Each affected buffer gets one undo step. Save the modified buffers when ready. Actions that only run a server command and workspace edits that add, rename, or remove files are currently unsupported.

## Disk changes and recovery

Loom checks open files once a second. A clean buffer reloads after an external edit. When saving a buffer that also changed on disk, Loom shows the differences and lets you keep editing, reload from disk, or overwrite. A stale preview cannot be used to overwrite a newer disk version.

Unsaved text is checkpointed about once a second under your local state directory. After a crash or interrupted session, Loom offers to restore those buffers on startup. Use **Recover Unsaved Buffers** from the palette to revisit a deferred recovery. `:q!` discards the current session's unsaved changes. Recovery covers text buffers.

## One keymap

Loom reads `~/.config/loom/config.json` by default. Set `XDG_CONFIG_HOME` to change the config root, or use `-config PATH` for a specific file. Run `loom -dump-config` for the full default config. Your file only needs to contain overrides:

```json
{
  "profile": "vim",
  "bindings": {
    "global": {
      "space e": "",
      "space E": "explorer"
    },
    "vim.select": {
      "s": "surround-selection"
    }
  }
}
```

Loom merges `global`, then `modal`, then the active profile's `.normal` or `.select` scope. In Insert it merges `global` then `insert`. An empty string removes the key from that scope's effective map. Commands and chords are validated at startup, including cases where a complete binding shadows a longer chord. Binding names are case sensitive, and `space` denotes the Space key. Pickers and completion popups handle their navigation keys while open.

## Plugins

A plugin can be any executable that reads JSON on stdin and writes JSON on stdout. Each configured plugin registers one command that appears in the palette and can be bound in any scope. Commands run from the directory where Loom was launched.

Build the included example and open a file using the sample config from this directory:

```sh
go build -o loom-uppercase ./examples/uppercase
./loom -config examples/config.json main.go
```

Select text with `v` and a motion. Run **Uppercase Selection** from the palette or press `g U` in Select mode. The example is written in Go. To write another plugin, use this version 1 request shape:

```json
{
  "version": 1,
  "path": "/absolute/path/to/main.go",
  "text": "hello",
  "cursor": {"line": 0, "character": 0},
  "selection": {
    "start": {"line": 0, "character": 0},
    "end": {"line": 0, "character": 5}
  }
}
```

Reply with a message, edits, or both:

```json
{
  "message": "Uppercased selection",
  "edits": [
    {
      "range": {
        "start": {"line": 0, "character": 0},
        "end": {"line": 0, "character": 5}
      },
      "newText": "HELLO"
    }
  ]
}
```

Positions are zero-based UTF-16 offsets within a line and range ends are exclusive. `selection` is omitted when there is no explicit selection. All ranges refer to the input snapshot. Edits apply together as one undo step if the buffer is still current. Plugins have eight seconds to reply and run with your account permissions.


## Maintaining Loom

The root holds only `main.go`, which calls `editor.Main()`, so `go install github.com/zhuchops/loom@latest` still works. Everything else is under `internal/`:

- `internal/editor` is the editor: buffers, panes, rendering, Vim input, git, completion and the language-server session. The files named below live here.
- `internal/syntax` holds the language table, the highlighters and comment delimiters. It returns token kinds, not colors.
- `internal/markdown` turns Markdown into wrapped rows of styled cells for the preview. It returns roles, not colors.
- `internal/theme` holds the built-in themes as plain data.
- `internal/jsonrpc` is the JSON-RPC client that talks to language servers.

Those four packages never import the editor. To add a language, edit `internal/syntax` and `defaultLanguages` in `config.go`. To add a theme, add a line to `internal/theme`.

Bubble Tea v2 owns the terminal lifecycle, input decoding, and terminal output. `terminal.go` translates Bubble Tea messages into Loom input and delivers asynchronous editor updates on the same event loop. `terminal_frame.go` draws into an Ultraviolet cell buffer so split geometry, popup placement, dimming, and mouse coordinates can share the existing renderer. The text-editing commands and language-server requests do not depend on Bubble Tea.

- `editor.go` owns modal input and the active buffer. `config.go` merges and validates named command bindings. The palette and which-key read from the same command registry.
- `panes.go` owns the split tree, including each split's size ratio, the resize commands and divider geometry; dragging a divider is handled in `mouse.go`. Leaves hold independent viewport state and refer to shared buffers. Only the active leaf projects its cursor and scroll fields into the buffer for the editing engine. Rendering an inactive leaf temporarily projects that leaf's view and restores the active projection afterward.
- `workspace.go` owns the ordered buffer tabs and session persistence. Closing a pane leaves its buffer open. Closing a tab replaces that buffer in every pane that was showing it. Session loading accepts the older two-pane format and prunes missing files.
- `directory.go` runs the modal engine on a scratch listing. Newline tags associate edited rows with original entries, and a reserved tag identifies the parent row. A write first validates the plan, then stages sources and rolls back failed filesystem moves. Navigation checks for unapplied edits.
- `language.go`, `lsp.go`, and `actions.go` handle LSP transport and edits. Worker goroutines send callbacks to the editor event loop. Workspace edits validate versions and disk content before applying edits.
- `sessions.go` keeps an editor per open project and switches the front end between them. Only the shown one sets the working directory, theme and tab width; the event loop keeps serving the others.
- `disk.go` and `recovery.go` implement conflict review and per-session snapshots. The editor event loop is the owner of UI state.
- `theme.go` applies a theme from `internal/theme` to the renderer's styles and saves the choice. `mdpreview.go` draws the Markdown preview pane, which `panes.go` stores as a leaf that never takes focus. `tabdrag.go` handles dragging a tab into a split.
- `fold.go` computes indentation folds, `symbols.go` the symbol pickers, `pathcomplete.go` file-name completion, and `tree.go` the explorer, including its filter and sort.
- `search.go` finds matches (plain, whole word, regex) and caches them per buffer version. `findbar.go` and `findbar_draw.go` are the find and replace panel: its input, its replace actions and its drawing.
- `notifications.go` keeps the full text of status messages and draws their detail view. `pairs.go` tracks generated delimiters through buffer edits and undo.

`go test -tags desktop,production ./internal/editor` also tests the desktop app's side of the bridge to its page (keys, the grid rows, the chrome, pickers, settings, dialogs, launches) without opening a window. The page itself is in `internal/editor/wailsui`. To check it by eye, `LOOM_DEMO=script loom-desktop` plays a file of lines such as `keys space f`, `type text`, `run palette`, `open path` and `sleep 300` once the window is up.

Every change gets a README update and a version bump in `internal/editor/main.go`, committed as `vX.Y.Z`. Run `go test -race ./...` from this directory. Set `LOOM_TEST_GOPLS=1` to enable the tests against `gopls` on PATH. Renderer tests inspect the in-memory frame, and terminal adapter tests send Bubble Tea messages directly.

## Projects

Loom starts with no project open. The dashboard (the Welcome page in the desktop app) offers **Open Project** (also `:project [path]` and the command palette), which opens the Finder folder picker on macOS (a built-in folder browser elsewhere); **Recent Projects** lists remembered folders and also accepts a typed path. Opening a project switches the file tree, search, git and language servers to that folder and restores its session. Loom remembers the last open project and reopens it the next time you start it without arguments; **Close Project** on the last open session clears that memory so the next start is empty again. `loom <dir>` opens a specific folder, and `loom .` opens the current one. File, search and git commands tell you to open a project first when none is open.

### Sessions

Every project you open gets a session of its own, and any number can be open at once. Each keeps its tabs, splits, unsaved changes, undo, language servers and git state while you work in another, so switching never asks you to save. Registers, macros and what you copied are shared.

**Space Tab**, `:sessions`, `⌘K` or a click on the project's name opens the fuzzy switcher. The previous session comes first, so Space Tab then Enter flips between two. Under the open sessions are **New Session…** and your recent projects; a typed path works too. **Ctrl+X** closes the selected session, which refuses while it has unsaved changes. Close Project does the same for the one on screen and goes to the one before it.

Open sessions are listed again after a restart, and each starts with its saved tabs when you first switch to it, so language servers run only for the ones you have used. The list is `open-sessions` in `~/.local/state/loom`. Crash recovery covers all of them.

Quitting looks at every session: `:q` takes you to the first with unsaved changes, `:q!` never discards changes in a session out of sight, and `:qa!` discards them all. The desktop app's Save All saves in every session.
