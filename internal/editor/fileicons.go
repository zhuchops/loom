package editor

import (
	"path/filepath"
	"strings"
)

// fileStyle is how a kind of file is marked, in both front ends: the desktop
// app draws mark (a monogram of up to three letters, or "@name" for one of its
// line icons) in ink; the terminal draws nerd, a Nerd Font glyph.
type fileStyle struct {
	mark string
	ink  byte // r(ed) y(ellow) g(reen) b(lue) p(urple) o(range) d(im)
	nerd rune
}

// Nerd Font glyphs. Material Design ones (U+F0000…) and Seti ones (U+E5FA…)
// are in every Nerd Font since 3.0.
const (
	nfGo         = 0xf07d3
	nfLock       = 0xf033e
	nfCog        = 0xf0493
	nfDocker     = 0xf0868
	nfGit        = 0xf02a2
	nfGithub     = 0xf02a4
	nfGitlab     = 0xf0ba0
	nfRust       = 0xf1617
	nfPython     = 0xf0320
	nfJS         = 0xf031e
	nfTS         = 0xf06e6
	nfRuby       = 0xf0d2d
	nfJava       = 0xf0b37
	nfC          = 0xf0671
	nfCpp        = 0xf0672
	nfCsharp     = 0xf031b
	nfSwift      = 0xf06e5
	nfKotlin     = 0xf1219
	nfPHP        = 0xf031f
	nfHaskell    = 0xf0c92
	nfR          = 0xf07d4
	nfLua        = 0xf08b1
	nfMarkdown   = 0xf0354
	nfJSON       = 0xf0626
	nfConsole    = 0xf018d
	nfHTML       = 0xf031d
	nfCSS        = 0xf031c
	nfSass       = 0xf07ec
	nfDatabase   = 0xf01bc
	nfImage      = 0xf02e9
	nfSVG        = 0xf0721
	nfPDF        = 0xf0226
	nfArchive    = 0xf05c4
	nfLicense    = 0xf0fc3
	nfKey        = 0xf0306
	nfCert       = 0xf0124
	nfBook       = 0xf05da
	nfText       = 0xf09ed
	nfDocument   = 0xf0219
	nfMusic      = 0xf0223
	nfVideo      = 0xf022b
	nfFont       = 0xf06d6
	nfExcel      = 0xf021b
	nfWord       = 0xf022c
	nfSlides     = 0xf0227
	nfNix        = 0xf1105
	nfTerraform  = 0xf1062
	nfXML        = 0xf05c0
	nfGraphQL    = 0xf0877
	nfReact      = 0xf0708
	nfVue        = 0xf0844
	nfAngular    = 0xf06b2
	nfNode       = 0xf0399
	nfNPM        = 0xf06f7
	nfPackage    = 0xf03d6
	nfWebpack    = 0xf072b
	nfBabel      = 0xf0a25
	nfESLint     = 0xf0c7a
	nfTailwind   = 0xf13ff
	nfTable      = 0xf0218
	nfNotebook   = 0xf082e
	nfApple      = 0xf0035
	nfAndroid    = 0xf0032
	nfKubernetes = 0xf10fe
	nfTools      = 0xf1064
	nfBuild      = 0xf1323
	nfShield     = 0xf099d
	nfEmail      = 0xf01ee
	nfInfo       = 0xf02fd
	nfBraces     = 0xf0169
	nfLambda     = 0xf0627
	nfBinary     = 0xf12a7
	nfRocket     = 0xf14de
	nfCalendar   = 0xf00ed
	nfWeb        = 0xf059f
	nfAnsible    = 0xf109a
	nfElixir     = 0xe62d
	nfScala      = 0xe68e
	nfDart       = 0xe64c
	nfZig        = 0xe6a9
	nfNim        = 0xe677
	nfClojure    = 0xe642
	nfSvelte     = 0xe697
	nfYarn       = 0xe6a7
	nfGradle     = 0xe660
	nfPrisma     = 0xe684
	nfJulia      = 0xe624
	nfOCaml      = 0xe67a
	nfCrystal    = 0xe62f
	nfPerl       = 0xe67e
	nfElm        = 0xe62c
	nfPureScript = 0xe630
	nfHaxe       = 0xe666
	nfVala       = 0xe69e
	nfMakefile   = 0xe673
	nfBazel      = 0xe63a
	nfVim        = 0xe62b
	nfErlang     = 0xe7b1
	nfGroovy     = 0xe775
	nfFSharp     = 0xe7a7
)

// Styles shared by many names.
var (
	styleLock      = fileStyle{"@lock", 'd', nfLock}
	styleConfig    = fileStyle{"@gear", 'd', nfCog}
	styleEnv       = fileStyle{"@key", 'y', nfKey}
	styleShell     = fileStyle{"$", 'g', nfConsole}
	styleDocker    = fileStyle{"@whale", 'b', nfDocker}
	styleGit       = fileStyle{"git", 'o', nfGit}
	styleLicense   = fileStyle{"@scale", 'y', nfLicense}
	styleReadme    = fileStyle{"@book", 'b', nfBook}
	styleText      = fileStyle{"@lines", 'd', nfText}
	styleImage     = fileStyle{"@image", 'p', nfImage}
	styleArchive   = fileStyle{"@box", 'o', nfArchive}
	styleAudio     = fileStyle{"@note", 'p', nfMusic}
	styleVideo     = fileStyle{"@film", 'r', nfVideo}
	styleFont      = fileStyle{"@font", 'r', nfFont}
	styleCert      = fileStyle{"@cert", 'y', nfCert}
	styleSecret    = fileStyle{"@key", 'o', nfKey}
	styleBinary    = fileStyle{"01", 'd', nfBinary}
	styleJS        = fileStyle{"JS", 'y', nfJS}
	styleTS        = fileStyle{"TS", 'b', nfTS}
	styleTSDecl    = fileStyle{"DTS", 'b', nfTS}
	styleJSON      = fileStyle{"{}", 'y', nfJSON}
	styleYAML      = fileStyle{"YM", 'r', nfCog}
	styleTOML      = fileStyle{"TM", 'o', nfCog}
	styleNPM       = fileStyle{"npm", 'r', nfNPM}
	styleNode      = fileStyle{"@hex", 'g', nfNode}
	styleMake      = fileStyle{"#", 'o', nfMakefile}
	styleBuild     = fileStyle{"@wrench", 'o', nfBuild}
	styleTest      = fileStyle{"@flask", 'g', nfTools}
	styleGoMod     = fileStyle{"GO", 'p', nfGo}
	styleCargo     = fileStyle{"@crate", 'o', nfRust}
	stylePython    = fileStyle{"PY", 'y', nfPython}
	styleRuby      = fileStyle{"RB", 'r', nfRuby}
	styleGradle    = fileStyle{"GR", 'g', nfGradle}
	styleElixir    = fileStyle{"EX", 'p', nfElixir}
	styleCI        = fileStyle{"@play", 'b', nfRocket}
	styleIgnore    = fileStyle{"@slash", 'd', nfCog}
	styleEditor    = fileStyle{"@gear", 'b', nfCog}
	styleLint      = fileStyle{"@check", 'p', nfESLint}
	stylePrettier  = fileStyle{"@brush", 'b', nfCog}
	styleBabel     = fileStyle{"BB", 'y', nfBabel}
	styleVite      = fileStyle{"@bolt", 'p', nfRocket}
	styleWebpack   = fileStyle{"WP", 'b', nfWebpack}
	styleTailwind  = fileStyle{"TW", 'b', nfTailwind}
	styleNix       = fileStyle{"NIX", 'b', nfNix}
	styleTF        = fileStyle{"TF", 'p', nfTerraform}
	styleK8s       = fileStyle{"K8", 'b', nfKubernetes}
	styleSwift     = fileStyle{"SW", 'o', nfSwift}
	styleSQL       = fileStyle{"DB", 'p', nfDatabase}
	styleData      = fileStyle{"@db", 'd', nfDatabase}
	styleCSV       = fileStyle{"@table", 'g', nfTable}
	styleMarkdown  = fileStyle{"MD", 'b', nfMarkdown}
	styleChangelog = fileStyle{"@clock", 'g', nfCalendar}
	styleAuthors   = fileStyle{"@people", 'b', nfInfo}
	styleHTML      = fileStyle{"<>", 'o', nfHTML}
	styleCSS       = fileStyle{"#", 'p', nfCSS}
	styleSass      = fileStyle{"SC", 'r', nfSass}
	styleProto     = fileStyle{"PB", 'b', nfBraces}
	styleGraphQL   = fileStyle{"GQ", 'p', nfGraphQL}
	stylePHP       = fileStyle{"PHP", 'p', nfPHP}
	styleDeno      = fileStyle{"DN", 'g', nfJS}
	styleBun       = fileStyle{"BUN", 'y', nfJS}
	styleBazel     = fileStyle{"BZ", 'g', nfBazel}
	styleCMake     = fileStyle{"CM", 'b', nfTools}
)

// fileNames are whole file names, in lower case.
var fileNames = map[string]fileStyle{
	// Lock files.
	"cargo.lock": styleLock, "package-lock.json": styleLock, "npm-shrinkwrap.json": styleLock, "yarn.lock": styleLock,
	"pnpm-lock.yaml": styleLock, "bun.lockb": styleLock, "bun.lock": styleLock, "deno.lock": styleLock,
	"gemfile.lock": styleLock, "podfile.lock": styleLock, "poetry.lock": styleLock, "pipfile.lock": styleLock,
	"pdm.lock": styleLock, "uv.lock": styleLock, "composer.lock": styleLock, "mix.lock": styleLock,
	"flake.lock": styleLock, "go.sum": styleLock, "go.work.sum": styleLock, "packages.lock.json": styleLock,
	"package.resolved": styleLock, "cartfile.resolved": styleLock, "pubspec.lock": styleLock, "conan.lock": styleLock,
	".terraform.lock.hcl": styleLock, "gradle.lockfile": styleLock, "manifest.toml": styleLock, "shard.lock": styleLock,

	// Manifests and build files.
	"cargo.toml": styleCargo, "rust-toolchain": styleCargo, "rust-toolchain.toml": styleCargo, "clippy.toml": styleCargo,
	"rustfmt.toml": styleCargo, ".rustfmt.toml": styleCargo, "build.rs": styleCargo,
	"go.mod": styleGoMod, "go.work": styleGoMod,
	"package.json": styleNPM, ".npmrc": styleNPM, ".npmignore": styleNPM,
	".nvmrc": styleNode, ".node-version": styleNode,
	".yarnrc": {"@yarn", 'b', nfYarn}, ".yarnrc.yml": {"@yarn", 'b', nfYarn},
	"pnpm-workspace.yaml": {"PN", 'o', nfPackage}, ".pnpmfile.cjs": {"PN", 'o', nfPackage},
	"deno.json": styleDeno, "deno.jsonc": styleDeno, "bunfig.toml": styleBun,
	"pyproject.toml": stylePython, "setup.py": stylePython, "setup.cfg": stylePython, "pipfile": stylePython,
	"tox.ini": stylePython, "manage.py": stylePython, ".python-version": stylePython, "noxfile.py": stylePython,
	"gemfile": styleRuby, "rakefile": styleRuby, "podfile": styleRuby, "guardfile": styleRuby, "config.ru": styleRuby,
	".ruby-version": styleRuby, ".rubocop.yml": styleRuby, "brewfile": styleRuby, "vagrantfile": styleRuby, "fastfile": styleRuby,
	"composer.json": stylePHP, "artisan": stylePHP,
	"mix.exs": styleElixir, ".formatter.exs": styleElixir,
	"build.gradle": styleGradle, "build.gradle.kts": styleGradle, "settings.gradle": styleGradle,
	"settings.gradle.kts": styleGradle, "gradle.properties": styleGradle, "gradlew": styleGradle, "gradlew.bat": styleGradle,
	"pom.xml": {"MVN", 'r', nfJava}, "build.sbt": {"SBT", 'r', nfScala},
	"package.swift": styleSwift, "cartfile": styleSwift, ".swiftlint.yml": styleSwift, ".swift-version": styleSwift,
	"pubspec.yaml": {"DT", 'b', nfDart},
	"build.zig":    {"ZIG", 'o', nfZig}, "build.zig.zon": {"ZIG", 'o', nfZig},
	"cmakelists.txt": styleCMake, "meson.build": styleBuild, "meson_options.txt": styleBuild, "configure": styleBuild,
	"configure.ac": styleBuild, "build.ninja": styleBuild, "sconstruct": styleBuild,
	"makefile": styleMake, "gnumakefile": styleMake, "justfile": styleMake, ".justfile": styleMake, "taskfile.yml": styleMake,
	"taskfile.yaml": styleMake, "rakefile.rb": styleRuby,
	"build": styleBazel, "build.bazel": styleBazel, "workspace": styleBazel, "workspace.bazel": styleBazel,
	"module.bazel": styleBazel, ".bazelrc": styleBazel, ".bazelversion": styleBazel,
	"flake.nix": styleNix, "default.nix": styleNix, "shell.nix": styleNix,
	".tool-versions": styleConfig, ".mise.toml": styleConfig, "mise.toml": styleConfig, ".envrc": styleEnv,
	"procfile": styleBuild, "jenkinsfile": styleCI, ".travis.yml": styleCI, "appveyor.yml": styleCI,
	"azure-pipelines.yml": styleCI, ".gitlab-ci.yml": {"@play", 'o', nfGitlab}, "bitbucket-pipelines.yml": styleCI,
	"netlify.toml": styleCI, "vercel.json": styleCI, "fly.toml": styleCI, "render.yaml": styleCI, "app.yaml": styleCI,
	"chart.yaml": styleK8s, "values.yaml": styleK8s, "kustomization.yaml": styleK8s, "skaffold.yaml": styleK8s,
	"ansible.cfg": {"@gear", 'r', nfAnsible}, "playbook.yml": {"AN", 'r', nfAnsible},

	// Containers.
	"dockerfile": styleDocker, "containerfile": styleDocker, ".dockerignore": styleDocker,
	"docker-compose.yml": styleDocker, "docker-compose.yaml": styleDocker, "compose.yml": styleDocker, "compose.yaml": styleDocker,

	// Version control and editors.
	".gitignore": styleGit, ".gitattributes": styleGit, ".gitmodules": styleGit, ".gitkeep": styleGit,
	".git-blame-ignore-revs": styleGit, ".mailmap": styleGit, ".gitconfig": styleGit,
	"codeowners": {"@people", 'o', nfGithub}, "funding.yml": {"@heart", 'r', nfGithub},
	"dependabot.yml": {"@robot", 'b', nfGithub}, "renovate.json": {"@robot", 'b', nfGithub},
	".editorconfig": styleEditor, ".vimrc": {"VIM", 'g', nfVim}, ".gvimrc": {"VIM", 'g', nfVim},
	"init.vim": {"VIM", 'g', nfVim}, ".ignore": styleIgnore, ".rgignore": styleIgnore, ".fdignore": styleIgnore,
	".prettierignore": styleIgnore, ".eslintignore": styleIgnore, ".stylelintignore": styleIgnore,
	".vscodeignore": styleIgnore, ".helmignore": styleIgnore, ".gcloudignore": styleIgnore,

	// Shell start-up files.
	".bashrc": styleShell, ".bash_profile": styleShell, ".bash_aliases": styleShell, ".bash_logout": styleShell,
	".zshrc": styleShell, ".zshenv": styleShell, ".zprofile": styleShell, ".zlogin": styleShell, ".profile": styleShell,
	".inputrc": styleShell, "config.fish": styleShell,

	// Tools' config.
	"tsconfig.json": styleTS, "jsconfig.json": styleJS, ".babelrc": styleBabel, ".swcrc": styleConfig,
	".browserslistrc": styleConfig, "browserslist": styleConfig, ".prettierrc": stylePrettier,
	".eslintrc": styleLint, ".stylelintrc": styleLint, ".markdownlint.json": styleLint, ".golangci.yml": styleLint,
	".golangci.yaml": styleLint, ".pre-commit-config.yaml": styleLint, ".flake8": styleLint, ".pylintrc": styleLint,
	"ruff.toml": styleLint, ".ruff.toml": styleLint, "mypy.ini": styleLint, ".mypy.ini": styleLint, "biome.json": styleLint,
	".clang-format": stylePrettier, ".clang-tidy": styleLint, ".clangd": styleConfig,
	"goreleaser.yml": styleBuild, ".goreleaser.yml": styleBuild, ".goreleaser.yaml": styleBuild,
	".air.toml": styleConfig, "wails.json": styleConfig, "tauri.conf.json": styleConfig, "turbo.json": styleConfig,
	"nx.json": styleConfig, "lerna.json": styleConfig, "angular.json": {"NG", 'r', nfAngular},
	".loom.json": styleConfig, "components.json": styleConfig,

	// Documents that every project has.
	"readme": styleReadme, "license": styleLicense, "licence": styleLicense, "copying": styleLicense, "unlicense": styleLicense,
	"notice": styleLicense, "patents": styleLicense, "changelog": styleChangelog, "changes": styleChangelog,
	"history": styleChangelog, "news": styleChangelog, "authors": styleAuthors, "contributors": styleAuthors,
	"maintainers": styleAuthors, "owners": styleAuthors, "security.md": {"@shield", 'r', nfShield},
	"code_of_conduct.md": {"@heart", 'r', nfInfo}, "contributing.md": {"@people", 'g', nfInfo},
	"todo": {"@check", 'y', nfText}, "todo.md": {"@check", 'y', nfText},
	"robots.txt": {"@robot", 'd', nfWeb}, "humans.txt": styleAuthors, "favicon.ico": styleImage,
	"version": styleText, ".version": styleText,

	// Secrets and keys.
	"id_rsa": styleSecret, "id_ed25519": styleSecret, "id_ecdsa": styleSecret, "id_dsa": styleSecret,
	"authorized_keys": styleSecret, "known_hosts": styleSecret, ".htpasswd": styleSecret, ".netrc": styleSecret,
	".pgpass": styleSecret, "credentials": styleSecret, ".htaccess": styleConfig,

	".ds_store": {"@apple", 'd', nfApple},
}

// filePrefixes match names that start a certain way, such as variants of a
// known file. Longer prefixes come first.
var filePrefixes = []struct {
	prefix string
	style  fileStyle
}{
	{"docker-compose", styleDocker}, {"dockerfile", styleDocker}, {"containerfile", styleDocker}, {"compose.", styleDocker},
	{".env", styleEnv}, {"env.", styleEnv},
	{"readme", styleReadme}, {"license", styleLicense}, {"licence", styleLicense}, {"copying", styleLicense},
	{"changelog", styleChangelog}, {"changes.", styleChangelog}, {"history.", styleChangelog}, {"authors", styleAuthors},
	{"tsconfig", styleTS}, {"jsconfig", styleJS},
	{"vite.config", styleVite}, {"vitest.config", styleTest}, {"webpack", styleWebpack}, {"rollup.config", styleBuild},
	{"esbuild", styleBuild}, {"tailwind.config", styleTailwind}, {"postcss.config", styleCSS},
	{"next.config", fileStyle{"NX", 'd', nfReact}}, {"nuxt.config", fileStyle{"NU", 'g', nfVue}},
	{"svelte.config", fileStyle{"SV", 'r', nfSvelte}}, {"astro.config", fileStyle{"AS", 'o', nfRocket}},
	{"remix.config", fileStyle{"RX", 'b', nfReact}}, {"vue.config", fileStyle{"VUE", 'g', nfVue}},
	{"jest.config", styleTest}, {"playwright.config", styleTest}, {"cypress.config", styleTest}, {"karma.conf", styleTest},
	{"babel.config", styleBabel}, {".babelrc", styleBabel},
	{".eslintrc", styleLint}, {"eslint.config", styleLint}, {".stylelintrc", styleLint}, {"stylelint.config", styleLint},
	{"commitlint.config", styleLint}, {".commitlintrc", styleLint}, {".lintstagedrc", styleLint}, {"lint-staged.config", styleLint},
	{".prettierrc", stylePrettier}, {"prettier.config", stylePrettier},
	{"makefile", styleMake}, {"cmakelists", styleCMake}, {"gemfile", styleRuby}, {"vagrantfile", styleRuby},
	{"jenkinsfile", styleCI}, {"requirements", stylePython}, {"constraints", stylePython},
	{"drizzle.config", styleSQL}, {"knexfile", styleSQL}, {"ormconfig", styleSQL},
	{".gitlab-ci", fileStyle{"@play", 'o', nfGitlab}}, {".git", styleGit},
}

// fileExts are extensions, in lower case. Two-part ones are tried first.
var fileExts = map[string]fileStyle{
	".d.ts": styleTSDecl, ".d.mts": styleTSDecl, ".d.cts": styleTSDecl,
	".test.ts": styleTest, ".spec.ts": styleTest, ".test.tsx": styleTest, ".spec.tsx": styleTest,
	".test.js": styleTest, ".spec.js": styleTest, ".test.jsx": styleTest, ".spec.jsx": styleTest,
	".tar.gz": styleArchive, ".tar.xz": styleArchive, ".tar.bz2": styleArchive, ".tar.zst": styleArchive, ".tar.lz4": styleArchive,
	".blade.php": stylePHP, ".module.css": styleCSS, ".module.scss": styleSass, ".min.js": styleJS, ".min.css": styleCSS,
	".code-workspace": styleEditor,

	// Languages.
	".go": {"GO", 'b', nfGo}, ".tmpl": {"GO", 'd', nfGo}, ".gotmpl": {"GO", 'd', nfGo},
	".ts": styleTS, ".mts": styleTS, ".cts": styleTS, ".tsx": {"TSX", 'b', nfReact},
	".js": styleJS, ".mjs": styleJS, ".cjs": styleJS, ".jsx": {"JSX", 'y', nfReact},
	".py": stylePython, ".pyi": stylePython, ".pyw": stylePython, ".pyx": stylePython, ".pxd": stylePython, ".ipynb": {"@book", 'o', nfNotebook},
	".rs": {"RS", 'o', nfRust}, ".rb": styleRuby, ".erb": {"ERB", 'r', nfRuby}, ".rake": styleRuby, ".gemspec": styleRuby,
	".java": {"JV", 'o', nfJava}, ".class": styleBinary, ".jar": styleArchive,
	".kt": {"KT", 'p', nfKotlin}, ".kts": {"KT", 'p', nfKotlin},
	".scala": {"SC", 'r', nfScala}, ".sc": {"SC", 'r', nfScala}, ".sbt": {"SBT", 'r', nfScala},
	".groovy": {"GV", 'b', nfGroovy}, ".gradle": styleGradle,
	".c": {"C", 'b', nfC}, ".h": {"H", 'p', nfC},
	".cpp": {"C+", 'b', nfCpp}, ".cc": {"C+", 'b', nfCpp}, ".cxx": {"C+", 'b', nfCpp}, ".c++": {"C+", 'b', nfCpp},
	".hpp": {"H+", 'p', nfCpp}, ".hh": {"H+", 'p', nfCpp}, ".hxx": {"H+", 'p', nfCpp}, ".ipp": {"H+", 'p', nfCpp}, ".inl": {"H+", 'p', nfCpp},
	".m": {"OC", 'b', nfApple}, ".mm": {"OC", 'b', nfApple},
	".cs": {"C#", 'g', nfCsharp}, ".csx": {"C#", 'g', nfCsharp}, ".csproj": {"C#", 'p', nfCsharp}, ".sln": {"VS", 'p', nfCsharp},
	".fs": {"F#", 'b', nfFSharp}, ".fsx": {"F#", 'b', nfFSharp}, ".fsi": {"F#", 'b', nfFSharp}, ".fsproj": {"F#", 'p', nfFSharp},
	".vb": {"VB", 'b', nfCsharp}, ".xaml": {"<>", 'b', nfXML},
	".swift": styleSwift, ".dart": {"DT", 'b', nfDart},
	".php": stylePHP, ".phtml": stylePHP,
	".lua": {"LU", 'b', nfLua}, ".luau": {"LU", 'b', nfLua},
	".ex": styleElixir, ".exs": styleElixir, ".heex": {"<>", 'p', nfElixir}, ".eex": {"<>", 'p', nfElixir}, ".leex": {"<>", 'p', nfElixir},
	".erl": {"ERL", 'r', nfErlang}, ".hrl": {"ERL", 'r', nfErlang},
	".hs": {"HS", 'p', nfHaskell}, ".lhs": {"HS", 'p', nfHaskell}, ".cabal": {"HS", 'p', nfHaskell},
	".ml": {"ML", 'o', nfOCaml}, ".mli": {"ML", 'o', nfOCaml}, ".re": {"RE", 'r', nfOCaml},
	".clj": {"CLJ", 'g', nfClojure}, ".cljs": {"CLJ", 'g', nfClojure}, ".cljc": {"CLJ", 'g', nfClojure}, ".edn": {"EDN", 'g', nfClojure},
	".elm": {"ELM", 'b', nfElm}, ".purs": {"PS", 'd', nfPureScript},
	".jl": {"JL", 'p', nfJulia}, ".r": {"R", 'b', nfR}, ".rmd": {"R", 'b', nfR},
	".pl": {"PL", 'b', nfPerl}, ".pm": {"PL", 'b', nfPerl},
	".zig": {"ZIG", 'o', nfZig}, ".zon": {"ZIG", 'o', nfZig}, ".nim": {"NIM", 'y', nfNim}, ".cr": {"CR", 'd', nfCrystal},
	".v": {"V", 'b', nfBraces}, ".sv": {"SV", 'b', nfBraces}, ".vhd": {"VHD", 'g', nfBraces},
	".odin": {"OD", 'b', nfBraces}, ".gleam": {"GL", 'p', nfBraces}, ".hx": {"HX", 'o', nfHaxe}, ".vala": {"VA", 'p', nfVala},
	".sol": {"SOL", 'd', nfBraces}, ".move": {"MV", 'b', nfBraces}, ".cairo": {"CA", 'o', nfBraces},
	".lisp": {"λ", 'p', nfLambda}, ".el": {"λ", 'p', nfLambda}, ".scm": {"λ", 'p', nfLambda}, ".rkt": {"λ", 'r', nfLambda}, ".fnl": {"λ", 'g', nfLambda},
	".dingo": {"DG", 'o', nfGo},
	".asm":   {"ASM", 'r', nfBinary}, ".s": {"ASM", 'r', nfBinary}, ".wat": {"WA", 'p', nfBinary}, ".wasm": {"WA", 'p', nfBinary},
	".vim": {"VIM", 'g', nfVim}, ".vue": {"VUE", 'g', nfVue}, ".svelte": {"SV", 'r', nfSvelte}, ".astro": {"AS", 'o', nfRocket},
	".prisma": {"PR", 'b', nfPrisma}, ".graphql": styleGraphQL, ".gql": styleGraphQL, ".proto": styleProto,
	".thrift": styleProto, ".avsc": styleProto, ".capnp": styleProto,
	".nix": styleNix, ".tf": styleTF, ".tfvars": styleTF, ".hcl": styleTF, ".bicep": {"BI", 'b', nfBraces},

	// Shell and scripts.
	".sh": styleShell, ".bash": styleShell, ".zsh": styleShell, ".fish": styleShell, ".ksh": styleShell, ".csh": styleShell,
	".command": styleShell, ".ps1": {"PS", 'b', nfConsole}, ".psm1": {"PS", 'b', nfConsole}, ".bat": {"BAT", 'g', nfConsole},
	".cmd": {"BAT", 'g', nfConsole}, ".awk": styleShell, ".mk": styleMake, ".mak": styleMake, ".just": styleMake,
	".ninja": styleBuild, ".cmake": styleCMake, ".bzl": styleBazel, ".bazel": styleBazel, ".star": styleBazel,
	".dockerfile": styleDocker, ".containerfile": styleDocker,

	// Web.
	".html": styleHTML, ".htm": styleHTML, ".xhtml": styleHTML, ".css": styleCSS, ".scss": styleSass, ".sass": styleSass,
	".less": {"LS", 'b', nfCSS}, ".styl": {"ST", 'g', nfCSS}, ".pcss": styleCSS, ".postcss": styleCSS,
	".hbs": {"<>", 'o', nfHTML}, ".handlebars": {"<>", 'o', nfHTML}, ".mustache": {"<>", 'o', nfHTML}, ".njk": {"<>", 'g', nfHTML},
	".ejs": {"<>", 'y', nfHTML}, ".pug": {"<>", 'r', nfHTML}, ".haml": {"<>", 'y', nfHTML}, ".slim": {"<>", 'r', nfHTML},
	".twig": {"<>", 'g', nfHTML}, ".liquid": {"<>", 'b', nfHTML}, ".jinja": {"<>", 'r', nfHTML}, ".j2": {"<>", 'r', nfHTML},
	".mdx": {"MDX", 'y', nfMarkdown}, ".map": {"MAP", 'd', nfJSON},

	// Data and config.
	".json": styleJSON, ".jsonc": styleJSON, ".json5": styleJSON, ".jsonl": styleJSON, ".ndjson": styleJSON, ".geojson": styleJSON,
	".webmanifest": styleJSON, ".har": styleJSON,
	".yaml": styleYAML, ".yml": styleYAML, ".toml": styleTOML, ".ini": {"INI", 'o', nfCog}, ".cfg": styleConfig, ".conf": styleConfig,
	".config": styleConfig, ".properties": styleConfig, ".plist": {"@apple", 'd', nfApple}, ".reg": styleConfig,
	".env": styleEnv, ".editorconfig": styleEditor, ".gitignore": styleGit,
	".xml": {"<>", 'o', nfXML}, ".xsd": {"<>", 'o', nfXML}, ".xsl": {"<>", 'o', nfXML}, ".svg": {"SVG", 'o', nfSVG},
	".csv": styleCSV, ".tsv": styleCSV, ".xls": {"@table", 'g', nfExcel}, ".xlsx": {"@table", 'g', nfExcel}, ".ods": styleCSV,
	".sql": styleSQL, ".psql": styleSQL, ".mysql": styleSQL, ".pgsql": styleSQL,
	".db": styleData, ".sqlite": styleData, ".sqlite3": styleData, ".parquet": styleData, ".avro": styleData,
	".arrow": styleData, ".feather": styleData, ".duckdb": styleData, ".mdb": styleData, ".dump": styleData,
	".pkl": styleData, ".pickle": styleData, ".npy": styleData, ".npz": styleData, ".h5": styleData, ".hdf5": styleData,
	".lock": styleLock, ".lockb": styleLock, ".sum": styleLock,

	// Text and documents.
	".md": styleMarkdown, ".markdown": styleMarkdown, ".mdown": styleMarkdown, ".mkd": styleMarkdown,
	".rst": {"RST", 'b', nfText}, ".adoc": {"AD", 'r', nfText}, ".asciidoc": {"AD", 'r', nfText}, ".org": {"ORG", 'g', nfText},
	".tex": {"TEX", 'g', nfText}, ".bib": {"BIB", 'g', nfText}, ".sty": {"TEX", 'g', nfText}, ".cls": {"TEX", 'g', nfText},
	".typ": {"TYP", 'b', nfText}, ".txt": styleText, ".text": styleText, ".log": {"@lines", 'd', nfText}, ".out": styleText,
	".rtf": {"@doc", 'b', nfDocument}, ".doc": {"@doc", 'b', nfWord}, ".docx": {"@doc", 'b', nfWord}, ".odt": {"@doc", 'b', nfDocument},
	".pages": {"@doc", 'o', nfDocument}, ".pdf": {"PDF", 'r', nfPDF}, ".epub": {"@book", 'g', nfBook},
	".ppt": {"@slides", 'o', nfSlides}, ".pptx": {"@slides", 'o', nfSlides}, ".odp": {"@slides", 'o', nfSlides},
	".diff": {"±", 'g', nfGit}, ".patch": {"±", 'g', nfGit}, ".ics": {"@clock", 'r', nfCalendar}, ".eml": {"@mail", 'b', nfEmail},
	".http": {"@globe", 'g', nfWeb}, ".rest": {"@globe", 'g', nfWeb}, ".url": {"@globe", 'b', nfWeb}, ".webloc": {"@globe", 'b', nfWeb},

	// Media.
	".png": styleImage, ".jpg": styleImage, ".jpeg": styleImage, ".gif": styleImage, ".webp": styleImage, ".avif": styleImage,
	".heic": styleImage, ".heif": styleImage, ".bmp": styleImage, ".tif": styleImage, ".tiff": styleImage, ".ico": styleImage,
	".icns": styleImage, ".psd": {"PS", 'b', nfImage}, ".ai": {"AI", 'o', nfImage}, ".sketch": {"@image", 'y', nfImage},
	".fig": {"@image", 'p', nfImage}, ".xcf": styleImage, ".raw": styleImage, ".cr2": styleImage, ".nef": styleImage, ".dng": styleImage,
	".mp3": styleAudio, ".wav": styleAudio, ".flac": styleAudio, ".ogg": styleAudio, ".m4a": styleAudio, ".aac": styleAudio,
	".opus": styleAudio, ".aiff": styleAudio, ".mid": styleAudio, ".midi": styleAudio,
	".mp4": styleVideo, ".mov": styleVideo, ".mkv": styleVideo, ".webm": styleVideo, ".avi": styleVideo, ".m4v": styleVideo,
	".wmv": styleVideo, ".flv": styleVideo,
	".ttf": styleFont, ".otf": styleFont, ".woff": styleFont, ".woff2": styleFont, ".eot": styleFont, ".ttc": styleFont,
	".glb": {"3D", 'b', nfImage}, ".gltf": {"3D", 'b', nfImage}, ".obj": {"3D", 'b', nfImage}, ".fbx": {"3D", 'b', nfImage},
	".stl": {"3D", 'b', nfImage}, ".blend": {"3D", 'o', nfImage},

	// Archives, packages and binaries.
	".zip": styleArchive, ".tar": styleArchive, ".gz": styleArchive, ".tgz": styleArchive, ".bz2": styleArchive,
	".xz": styleArchive, ".zst": styleArchive, ".7z": styleArchive, ".rar": styleArchive, ".lz4": styleArchive,
	".br": styleArchive, ".deb": styleArchive, ".rpm": styleArchive, ".apk": {"@box", 'g', nfAndroid},
	".aab": {"@box", 'g', nfAndroid}, ".ipa": {"@box", 'b', nfApple}, ".dmg": {"@box", 'd', nfApple}, ".pkg": {"@box", 'd', nfApple},
	".iso": styleArchive, ".img": styleArchive, ".whl": {"@box", 'y', nfPython}, ".egg": {"@box", 'y', nfPython},
	".gem": {"@box", 'r', nfRuby}, ".crate": styleCargo, ".nupkg": {"@box", 'p', nfCsharp}, ".vsix": {"@box", 'b', nfPackage},
	".exe": styleBinary, ".dll": styleBinary, ".so": styleBinary, ".dylib": styleBinary, ".a": styleBinary, ".lib": styleBinary,
	".o": styleBinary, ".bin": styleBinary, ".elf": styleBinary, ".pyc": styleBinary, ".pyo": styleBinary,
	".beam": styleBinary, ".dsym": styleBinary,

	// Keys and certificates.
	".pem": styleCert, ".crt": styleCert, ".cer": styleCert, ".der": styleCert, ".csr": styleCert, ".p12": styleCert,
	".pfx": styleCert, ".jks": styleCert, ".keystore": styleCert, ".pub": styleSecret, ".asc": styleSecret, ".gpg": styleSecret,
	".sig": styleSecret, ".key": styleSecret, ".age": styleSecret, ".kdbx": styleSecret,
}

// fileStyleOf finds how to mark the file at path. Names are tried before
// prefixes, two-part extensions before one-part ones, and dotfiles ending in
// "rc" count as config.
func fileStyleOf(path string) (fileStyle, bool) {
	name := strings.ToLower(filepath.Base(path))
	if s, ok := fileNames[name]; ok {
		return s, true
	}
	for _, p := range filePrefixes {
		if strings.HasPrefix(name, p.prefix) {
			return p.style, true
		}
	}
	if i := strings.Index(name[1:], "."); i >= 0 {
		// Every extension the name has, longest first: a.test.ts tries
		// ".test.ts", then ".ts".
		rest := name[i+1:]
		for {
			if s, ok := fileExts[rest]; ok {
				return s, true
			}
			j := strings.Index(rest[1:], ".")
			if j < 0 {
				break
			}
			rest = rest[j+1:]
		}
	}
	if strings.HasPrefix(name, ".") && (strings.HasSuffix(name, "rc") || strings.HasSuffix(name, "config")) {
		return styleConfig, true
	}
	return fileStyle{}, false
}
