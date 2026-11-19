# Loom 0.8

Loom is a Go terminal editor with a Zed-inspired workspace, Vim-style editing, language-server integration, a live Markdown preview, and eleven color themes (Everforest by default).

## Run it

Install with Go 1.26 or later:

```sh
go install github.com/zhuchops/loom@latest
```

Run `loom` from the directory you want to edit. If your shell cannot find it, add Go's binary directory to your PATH. The default is `$(go env GOPATH)/bin` unless you set `GOBIN`.

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

The defaults use no Option, Command, function keys, or Ctrl+Shift chords. Space, g, and Ctrl+W start chords in Normal and Select modes. Ctrl+P and Ctrl+S work in every mode. Ctrl+Space starts completion in Insert. Press Esc to leave Insert before using a Space chord. Inside a picker, type to filter, use ↑/↓ to navigate, and press Enter or Esc. Search for **Show Keybindings** in the command palette to inspect the active mode.

In Vim, `f.` moves to the next period, `;` repeats that move, and `,` finds a period in the opposite direction. Counts work too: `3;` repeats the find three times. Character finds stay on the current line and also work in Select mode. `.` repeats the last edit.

Vim supports counts, text objects, macros, and a single selection. A typing burst and a macro replay in one buffer each form one undo step.

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

Ex commands include `:split [file]`, `:vsplit [file]`, `:close`, `:only`, `:tabnext [number]`, `:tabprev`, `:tabfirst`, `:tablast`, `:tabmove [+/-]N`, and `:tabclose`. **Close Pane** (`:close`) closes the active tab when only one pane is open. **Close Tab** (`:tabclose` or `:close tab`) closes the active buffer tab. A bare `:tabmove` moves the tab to the end. An unsigned position is zero-based. `:q` closes the active pane when several are open. `:qa` quits the editor. Set line numbers with `:set rnu`, `:set nornu`, or `:set rnu!`. Relative numbering keeps the current line's absolute number visible.

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

Built-in language entries, with a language server and formatter used only when they are on `PATH`: Go, TypeScript, JavaScript, Python, Rust, C, C++, Java (`jdtls`), Kotlin, Scala, Swift, C#, PHP, Dart, Zig, Haskell, Elixir, OCaml, Ruby, Perl, R, Lua, assembly (`.asm`, `.s`, `.nasm`, `.inc`; `asm-lsp`), Dockerfile, Makefile, CMake, Protobuf, GraphQL, Terraform, XML, shell, SQL, JSON, YAML, TOML, CSS, HTML and Markdown. Files such as `Dockerfile` and `Makefile` are recognised by name. Every language gets syntax colors and `gcc` comment toggling even without a server. Add or override entries under `languages` in your config.

## Color themes

Run **Color Theme** from the command palette to switch themes. Your choice is remembered next time. Built in: everforest (default), gruvbox, nord, dracula, tokyo-night, catppuccin, one-dark, monokai, solarized-dark, solarized-light, and terminal. The `terminal` theme draws with your terminal's own background and 16 ANSI colors, so it follows whatever palette the terminal uses. Set `"theme": "nord"` in your config to pin one.

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

Drag a tab onto a pane to move it. Near the left, right, top or bottom edge a highlight shows the split it will make, and the middle opens the tab in that pane. Drag a tab along the tab bar to reorder it. Click a tab or explorer entry to open it. Click in the editor to place the cursor, drag to select, or double-click to select a word. The wheel scrolls the pane or explorer under the pointer. Drag the sidebar divider to resize it. Right-click for editor or file actions. Completion and picker rows are clickable.

## Command palette

Press `:` or `Space p` to open the same searchable palette. Type an exact Ex command or filter by the command name. Each entry shows its shortcut on the right, followed by the matching Ex command when there is one. Use `:42` or `:goto 42` to jump to a line, `:$` for the last line, `:noh` to hide search highlights, and `:e path` to open a file. `n` and `N` continue the last search after `:noh`.

Popups are centered and take their colors from the active theme. Modal prompts dim the workspace behind them. Start a chord such as Space, g, or Ctrl+W to see available continuations. The status bar shows the active mode and file, unsaved edits, diagnostics, cursor position, and file format.

## Git

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

A language server or formatter is used only when its command is on `PATH`. Missing tools are skipped without an error. Go gets the full set: gopls, syntax colors, and gofmt or goimports. For other languages, the language server adds completion, hover, definition, references, diagnostics, rename, and code actions, and it also provides semantic colors. Syntax colors without a server are built in for every language listed above. Completion inserts plain text, apart from the `()` added after functions.

Use **Restart Language Server** in the command palette or run `:lsp-restart` from a file tab. Loom restarts that file's language server for its workspace and sends the current contents of its open buffers, including unsaved changes.

The status bar shows errors and warnings for the current file and for the whole project, as `file E0 W0 · project E0 W0`. The project count includes every file under the project root that the language server reports, whether or not it is open. Space D opens Project Problems, and Space d opens File Problems. Each popup lists errors and warnings with their file and line. Type to filter, for example `error` or a file name. Enter jumps to the location. Project counts depend on the server. gopls reports every file in the module, but a server that reports only open files gives a partial count. A language's server starts when you open a file of that language, so its files are counted from then on.

Diagnostics underline the reported range in the editor. Errors use red, warnings use amber, and information and hints use teal. In either Problems picker, the preview shows the full message, line and column, and nearby source with the same underline.

Hover over an underlined diagnostic or its gutter marker to read its full message. Hover over a symbol to see language server documentation after a brief pause; when the symbol has a diagnostic, both appear together. Messages wrap to the popup width; scroll over the popup for messages taller than the terminal. Hover Documentation also has a wrapped, scrollable preview.

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
- `disk.go` and `recovery.go` implement conflict review and per-session snapshots. The editor event loop is the owner of UI state.
- `theme.go` applies a theme from `internal/theme` to the renderer's styles and saves the choice. `mdpreview.go` draws the Markdown preview pane, which `panes.go` stores as a leaf that never takes focus. `tabdrag.go` handles dragging a tab into a split.
- `fold.go` computes indentation folds, `symbols.go` the symbol pickers, `pathcomplete.go` file-name completion, and `tree.go` the explorer, including its filter and sort.
- `search.go` finds matches (plain, whole word, regex) and caches them per buffer version. `findbar.go` and `findbar_draw.go` are the find and replace panel: its input, its replace actions and its drawing.
- `notifications.go` keeps the full text of status messages and draws their detail view. `pairs.go` tracks generated delimiters through buffer edits and undo.

Every change gets a README update and a version bump in `internal/editor/main.go`, committed as `vX.Y.Z`. Run `go test -race ./...` from this directory. Set `LOOM_TEST_GOPLS=1` to enable the tests against `gopls` on PATH. Renderer tests inspect the in-memory frame, and terminal adapter tests send Bubble Tea messages directly.
