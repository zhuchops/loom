package editor

import (
	"errors"
	"strings"
	"time"
)

// Git from the command palette: commit, amend, push, pull, fetch, sync,
// stash and branches, for the repository the project is in. The Source
// Control panel uses the same actions.

func (e *editor) gitActionCommands() {
	e.add("git-commit", "Git: Commit…", func(e *editor) { e.gitCommit(false) })
	e.add("git-commit-all", "Git: Commit All…", func(e *editor) { e.gitCommit(true) })
	e.add("git-amend", "Git: Amend Last Commit…", func(e *editor) { e.gitAmend() })
	e.add("git-push", "Git: Push", func(e *editor) { e.gitPush() })
	e.add("git-pull", "Git: Pull", func(e *editor) { e.gitRun("pull", []string{"pull", "--ff-only"}) })
	e.add("git-pull-rebase", "Git: Pull (Rebase)", func(e *editor) { e.gitRun("pull", []string{"pull", "--rebase"}) })
	e.add("git-fetch", "Git: Fetch", func(e *editor) { e.gitRun("fetch", []string{"fetch", "--all", "--prune"}) })
	e.add("git-sync", "Git: Sync (Pull, then Push)", func(e *editor) { e.gitSync() })
	e.add("git-stash", "Git: Stash Changes", func(e *editor) {
		e.gitRun("stash", []string{"stash", "push", "--include-untracked", "-m", "Loom " + time.Now().Format("2006-01-02 15:04")})
	})
	e.add("git-stash-pop", "Git: Pop Stash", func(e *editor) { e.gitRun("stash pop", []string{"stash", "pop"}) })
	e.add("git-new-branch", "Git: Create Branch…", func(e *editor) { e.gitNewBranch() })
	e.add("git-stage-file", "Git: Stage This File", func(e *editor) { e.gitThisFile("add", "--") })
	e.add("git-unstage-file", "Git: Unstage This File", func(e *editor) { e.gitThisFile("restore", "--staged", "--") })
}

// gitTop is the repository's top folder, or "" with a message when the
// project isn't in one.
func (e *editor) gitTop() string {
	if e.git.top != "" {
		return e.git.top
	}
	if e.panel != nil && e.panel.top != "" {
		return e.panel.top
	}
	out, err := gitExec(e.root, 5*time.Second, "rev-parse", "--show-toplevel")
	if err != nil {
		e.notify("Not a git repository")
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitRun runs git steps one after another off the editor's goroutine,
// reports the last output and brings the marks, the panel and open files up
// to date.
func (e *editor) gitRun(what string, steps ...[]string) {
	top := e.gitTop()
	if top == "" {
		return
	}
	e.notify("git " + what + "…")
	go func() {
		var out []byte
		var err error
		for _, step := range steps {
			if out, err = gitExec(top, 3*time.Minute, step...); err != nil {
				break
			}
		}
		e.async <- func() {
			if err != nil {
				e.notify(err.Error())
			} else if message := strings.TrimSpace(string(out)); message != "" {
				e.notify(message)
			} else {
				e.notify("git " + what + " finished")
			}
			e.afterGit()
		}
	}()
}

// afterGit catches up with a repository git just changed: files a pull or a
// stash rewrote reload, and the marks and the panel follow.
func (e *editor) afterGit() {
	e.checkDisk()
	e.reloadSourceControl()
	e.refreshGit()
}

// gitPush pushes, publishing the branch to the first remote when it has no
// upstream yet.
func (e *editor) gitPush() {
	top := e.gitTop()
	if top == "" {
		return
	}
	if _, err := gitExec(top, 5*time.Second, "rev-parse", "--abbrev-ref", "@{upstream}"); err == nil {
		e.gitRun("push", []string{"push"})
		return
	}
	out, err := gitExec(top, 5*time.Second, "remote")
	remotes := strings.Fields(string(out))
	if err != nil || len(remotes) == 0 {
		e.notify("No remote to push to")
		return
	}
	remote := remotes[0]
	for _, r := range remotes {
		if r == "origin" {
			remote = r
		}
	}
	e.gitRun("push", []string{"push", "--set-upstream", remote, "HEAD"})
}

// gitSync pulls with a rebase and then pushes.
func (e *editor) gitSync() {
	top := e.gitTop()
	if top == "" {
		return
	}
	if _, err := gitExec(top, 5*time.Second, "rev-parse", "--abbrev-ref", "@{upstream}"); err != nil {
		e.gitPush() // nothing to pull from yet
		return
	}
	e.gitRun("sync", []string{"pull", "--rebase"}, []string{"push"})
}

// gitStaged reports whether anything is staged.
func gitStaged(top string) bool {
	_, err := gitExec(top, 5*time.Second, "diff", "--cached", "--quiet")
	return err != nil
}

// gitChanged reports whether the working tree has any change at all.
func gitChanged(top string) bool {
	out, err := gitExec(top, 5*time.Second, "status", "--porcelain")
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// gitCommit asks for a message and commits what is staged; with all, or
// when nothing is staged, it stages every change first.
func (e *editor) gitCommit(all bool) {
	top := e.gitTop()
	if top == "" {
		return
	}
	if !all && !gitStaged(top) {
		all = true // as VS Code does: no staged changes means commit them all
	}
	if all && !gitChanged(top) {
		e.notify("Nothing to commit")
		return
	}
	title := "Commit message"
	if all {
		title = "Commit message (all changes)"
	}
	e.openMenu(&menu{title: title, submit: func(message string) {
		message = strings.TrimSpace(message)
		if message == "" {
			e.notify("Commit message is empty")
			return
		}
		if all {
			if _, err := gitExec(top, 30*time.Second, "add", "-A"); err != nil {
				e.notify(err.Error())
				return
			}
		}
		e.gitCommitNow(top, "commit", "-m", message)
	}})
}

// gitAmend rewrites the last commit with what is staged and a message that
// starts as the old one.
func (e *editor) gitAmend() {
	top := e.gitTop()
	if top == "" {
		return
	}
	last, err := gitExec(top, 5*time.Second, "log", "-1", "--format=%B")
	if err != nil {
		e.notify("No commit to amend")
		return
	}
	e.openMenu(&menu{title: "Amend message", input: strings.TrimSpace(strings.SplitN(string(last), "\n", 2)[0]), submit: func(message string) {
		message = strings.TrimSpace(message)
		if message == "" {
			e.notify("Commit message is empty")
			return
		}
		e.gitCommitNow(top, "commit", "--amend", "-m", message)
	}})
}

func (e *editor) gitCommitNow(top string, args ...string) {
	out, err := gitExec(top, 30*time.Second, args...)
	if err != nil {
		e.notify(err.Error())
		return
	}
	e.notify(firstLine(string(out)))
	e.afterGit()
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

func (e *editor) gitNewBranch() {
	top := e.gitTop()
	if top == "" {
		return
	}
	e.openMenu(&menu{title: "New branch name", submit: func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, err := gitExec(top, 5*time.Second, "check-ref-format", "--branch", name); err != nil {
			e.notify(errors.New("not a valid branch name: " + name).Error())
			return
		}
		if _, err := gitExec(top, 10*time.Second, "switch", "-c", name); err != nil {
			e.notify(err.Error())
			return
		}
		e.notify("Switched to a new branch " + name)
		e.afterGit()
	}})
}

// gitThisFile runs a git command on the file in the active tab.
func (e *editor) gitThisFile(args ...string) {
	top := e.gitTop()
	if top == "" || e.b.path == "" || e.b.dashboard {
		return
	}
	if _, err := gitExec(top, 10*time.Second, append(args, e.b.path)...); err != nil {
		e.notify(err.Error())
		return
	}
	e.afterGit()
}
