package guardrail

import (
	"os"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// finding is one guardrail hit: a category with severity and detail.
type finding struct {
	cat    Category
	severe bool
	detail string
}

// systemRoots are top-level directories whose destruction is catastrophic.
var systemRoots = map[string]bool{
	"bin": true, "boot": true, "dev": true, "etc": true, "home": true,
	"lib": true, "lib64": true, "opt": true, "proc": true, "root": true,
	"sbin": true, "sys": true, "usr": true, "var": true,
}

// destructiveOps delete or destroy data (severity by target, D23).
var destructiveOps = map[string]bool{
	"rm": true, "shred": true, "truncate": true, "unlink": true,
}

// writeOps modify the filesystem; their destination is classified.
var writeOps = map[string]bool{
	"mv": true, "cp": true, "install": true, "ln": true,
	"mkdir": true, "touch": true, "tee": true,
}

// privilegeCmds escalate or manage the system: ASK in balanced.
var privilegeCmds = map[string]bool{
	"sudo": true, "su": true, "doas": true, "pkexec": true,
	"systemctl": true, "mount": true, "umount": true, "fdisk": true, "parted": true,
}

// hardSystemCmds are REFUSE in all modes (D22).
var hardSystemCmds = map[string]bool{
	"shutdown": true, "reboot": true, "halt": true, "poweroff": true, "mkfs": true,
}

// ttyHangers block waiting on a terminal unless stdin is redirected.
var ttyHangers = map[string]bool{
	"vim": true, "vi": true, "nano": true, "less": true, "more": true,
	"top": true, "htop": true, "watch": true, "bc": true,
	"mysql": true, "psql": true, "read": true, "rlwrap": true,
}

// scriptInterps hang only without a script/-c/command argument.
var scriptInterps = map[string]bool{
	"python": true, "python3": true, "node": true, "ssh": true, "cat": true,
}

// shellNames are shell interpreters (pipe-to-shell, D23).
var shellNames = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "fish": true,
}

// downloaders fetch remote content; piping them to a shell is hard REFUSE (D22).
var downloaders = map[string]bool{"curl": true, "wget": true}

func detect(file *syntax.File, projectDir string) []finding {
	var fs []finding
	syntax.Walk(file, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.Stmt:
			fs = checkStmt(fs, n, projectDir)
		case *syntax.BinaryCmd:
			if n.Op == syntax.Pipe || n.Op == syntax.PipeAll {
				fs = checkPipe(fs, n)
			}
		case *syntax.FuncDecl:
			fs = checkFunc(fs, n)
		case *syntax.WhileClause:
			fs = checkWhile(fs, n)
		}
		return true
	})
	return fs
}

func checkStmt(fs []finding, st *syntax.Stmt, projectDir string) []finding {
	if call, ok := st.Cmd.(*syntax.CallExpr); ok {
		fs = checkCall(fs, call, st, projectDir)
	}
	for _, r := range st.Redirs {
		fs = checkRedirect(fs, r, projectDir)
	}
	return fs
}

func checkCall(fs []finding, call *syntax.CallExpr, st *syntax.Stmt, projectDir string) []finding {
	name, known := callName(call)
	if !known {
		return fs
	}
	base := filepath.Base(name)
	args := literalArgs(call)
	nonFlags := nonFlagArgs(args)

	switch {
	case hardSystemCmds[base] || strings.HasPrefix(base, "mkfs."):
		fs = append(fs, finding{CatPrivilege, true, base})
	case privilegeCmds[base]:
		fs = append(fs, finding{CatPrivilege, false, base})
	case base == "git":
		fs = checkGit(fs, args)
	case base == "dd":
		if hasArg(args, "if=/dev/zero") && !hasArg(args, "of=/dev/null") {
			fs = append(fs, finding{CatRunaway, false, "dd zero-fill"})
		}
	case base == "yes":
		if stHasWriteRedirect(st) {
			fs = append(fs, finding{CatRunaway, false, "yes > file"})
		}
	case destructiveOps[base]:
		for _, a := range nonFlags {
			fs = classifyWriteTarget(fs, resolvePath(a, projectDir), projectDir, base+" "+a)
		}
	case writeOps[base]:
		if len(nonFlags) > 0 {
			dest := nonFlags[len(nonFlags)-1] // mv/cp/install/ln: last arg is the destination
			fs = classifyWriteTarget(fs, resolvePath(dest, projectDir), projectDir, base+" "+dest)
		}
	default:
		// reads and general commands: only secrets are checked
		for _, a := range nonFlags {
			if protectedPath(resolvePath(a, projectDir)) {
				fs = append(fs, finding{CatProtectedPath, false, a})
			}
		}
	}

	if isTTYHanger(base, args, st) {
		fs = append(fs, finding{CatTTYHanger, false, base})
	}
	return fs
}

func checkRedirect(fs []finding, r *syntax.Redirect, projectDir string) []finding {
	path, known := wordLiteral(r.Word)
	if !known {
		return fs
	}
	resolved := resolvePath(path, projectDir)
	if isWriteOp(r.Op) {
		if resolved == "/dev/null" {
			return fs // writing to the null device discards output — never destructive
		}
		return classifyWriteTarget(fs, resolved, projectDir, "redirect "+path)
	}
	// read redirects: only secrets matter
	if protectedPath(resolved) {
		fs = append(fs, finding{CatProtectedPath, false, "read " + path})
	}
	return fs
}

// classifyWriteTarget classifies a write/destroy target path:
// system root or project ancestor -> severe destructive (hard REFUSE);
// secret path -> protected; absolute path outside the project -> escape.
func classifyWriteTarget(fs []finding, p, projectDir, detail string) []finding {
	switch {
	case destructiveTarget(p, projectDir):
		fs = append(fs, finding{CatDestructiveFS, true, detail})
	case protectedPath(p):
		fs = append(fs, finding{CatProtectedPath, false, detail})
	case isEscape(p, projectDir):
		fs = append(fs, finding{CatPathEscape, false, detail})
	}
	return fs
}

func checkGit(fs []finding, args []string) []finding {
	danger := false
	switch {
	case hasArg(args, "push") && hasAnyArg(args, "-f", "--force", "--force-with-lease"):
		danger = true
	case hasArg(args, "reset") && hasArg(args, "--hard"):
		danger = true
	case hasArg(args, "clean") && (hasShortFlag(args, 'f') || hasArg(args, "--force")) && (hasShortFlag(args, 'd') || hasShortFlag(args, 'x')):
		danger = true
	case hasArg(args, "filter-branch"):
		danger = true
	case hasArg(args, "branch") && hasArg(args, "-D"):
		danger = true
	case hasArg(args, "checkout") && hasArg(args, "--"):
		danger = true
	}
	if danger {
		fs = append(fs, finding{CatGitIrreversible, false, "git"})
	}
	return fs
}

func checkPipe(fs []finding, b *syntax.BinaryCmd) []finding {
	left := pipeEnd(b, true)
	right := pipeEnd(b, false)
	if !shellNames[right] {
		return fs
	}
	f := finding{CatPipeToShell, false, "pipe to " + right}
	if downloaders[left] {
		f.severe = true
		f.detail = left + "|" + right
	}
	return append(fs, f)
}

// pipeEnd walks a pipe chain to its leftmost or rightmost command name.
func pipeEnd(b *syntax.BinaryCmd, left bool) string {
	cur := b
	for {
		st := cur.Y
		if left {
			st = cur.X
		}
		if nb, ok := st.Cmd.(*syntax.BinaryCmd); ok && (nb.Op == syntax.Pipe || nb.Op == syntax.PipeAll) {
			cur = nb
			continue
		}
		if call, ok := st.Cmd.(*syntax.CallExpr); ok {
			if name, known := callName(call); known {
				return filepath.Base(name)
			}
		}
		return ""
	}
}

func checkFunc(fs []finding, f *syntax.FuncDecl) []finding {
	if f.Name == nil || f.Body == nil {
		return fs
	}
	selfPiped := false
	syntax.Walk(f.Body, func(n syntax.Node) bool {
		b, ok := n.(*syntax.BinaryCmd)
		if !ok || (b.Op != syntax.Pipe && b.Op != syntax.PipeAll) {
			return true
		}
		if callsName(b.X, f.Name.Value) && callsName(b.Y, f.Name.Value) {
			selfPiped = true
		}
		return true
	})
	if selfPiped {
		fs = append(fs, finding{CatRunaway, true, "fork bomb " + f.Name.Value})
	}
	return fs
}

func callsName(st *syntax.Stmt, name string) bool {
	call, ok := st.Cmd.(*syntax.CallExpr)
	if !ok {
		return false
	}
	n, known := callName(call)
	return known && filepath.Base(n) == name
}

func checkWhile(fs []finding, w *syntax.WhileClause) []finding {
	infinite := false
	for _, c := range w.Cond {
		call, ok := c.Cmd.(*syntax.CallExpr)
		if !ok {
			continue
		}
		name, known := callName(call)
		if !known {
			continue
		}
		if (w.Until && name == "false") || (!w.Until && (name == "true" || name == ":")) {
			infinite = true
		}
	}
	if infinite {
		fs = append(fs, finding{CatRunaway, false, "infinite loop"})
	}
	return fs
}

func isTTYHanger(base string, args []string, st *syntax.Stmt) bool {
	if !ttyHangers[base] && !scriptInterps[base] {
		return false
	}
	for _, r := range st.Redirs {
		if r.Op == syntax.RdrIn || r.Op == syntax.Hdoc || r.Op == syntax.DashHdoc {
			return false // stdin redirected: runs to completion
		}
	}
	switch base {
	case "ssh":
		return len(args) < 2 // ssh host (no command) hangs; ssh host cmd does not
	case "python", "python3", "node":
		for _, a := range args {
			if a == "-c" || a == "-e" || (!strings.HasPrefix(a, "-") && a != "-") {
				return false // -c or a script file: runs to completion
			}
		}
		return true
	case "cat":
		for _, a := range args {
			if !strings.HasPrefix(a, "-") {
				return false // reads a file, not stdin
			}
		}
		return true // cat with no file args reads stdin
	}
	return true
}

// ---- path helpers ----

// resolvePath expands ~ and resolves relative paths against the project dir.
func resolvePath(p, projectDir string) string {
	p = strings.TrimSpace(p)
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
	} else if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			p = home
		}
	}
	if !filepath.IsAbs(p) {
		return filepath.Join(projectDir, p)
	}
	return filepath.Clean(p)
}

// destructiveTarget reports whether a path is a system root or the
// project dir itself (or an ancestor of it) — destroying it is catastrophic.
func destructiveTarget(p, projectDir string) bool {
	p = filepath.Clean(p)
	if p == "/" {
		return true
	}
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	if len(parts) > 0 && systemRoots[parts[0]] {
		return true
	}
	proj := filepath.Clean(projectDir)
	return p == proj || strings.HasPrefix(proj, p+"/")
}

// isEscape reports whether an absolute path lies outside the project dir.
func isEscape(p, projectDir string) bool {
	p = filepath.Clean(p)
	if !filepath.IsAbs(p) {
		return false // relative paths resolve inside the project by construction
	}
	proj := filepath.Clean(projectDir)
	return p != proj && !strings.HasPrefix(p, proj+"/")
}

// protectedPath reports secret paths: dotenv/credential/key files,
// ~/.ssh, ~/.gnupg, ~/.aws, system shadow files, and ~/.fender (API keys).
func protectedPath(p string) bool {
	base := strings.ToLower(filepath.Base(p))
	lower := strings.ToLower(p)
	switch {
	case base == ".env", strings.HasPrefix(base, ".env."),
		base == ".netrc", base == ".npmrc", base == ".pgpass", base == ".git-credentials",
		strings.HasSuffix(base, ".pem"), strings.HasSuffix(base, ".key"),
		strings.HasSuffix(base, ".p12"), strings.HasSuffix(base, ".pfx"),
		base == "id_rsa", base == "id_ed25519", base == "id_dsa", base == "id_ecdsa":
		return true
	}
	for _, dir := range []string{".ssh", ".gnupg", ".aws"} {
		if strings.HasPrefix(lower, dir+"/") || strings.Contains(lower, "/"+dir+"/") {
			return true
		}
	}
	switch lower {
	case "/etc/shadow", "/etc/sudoers", "/etc/gshadow":
		return true
	}
	if home, err := os.UserHomeDir(); err == nil {
		if p == filepath.Join(home, ".fender") || strings.HasPrefix(p, filepath.Join(home, ".fender")+"/") {
			return true
		}
	}
	return false
}

// ---- small helpers ----

func isWriteOp(op syntax.RedirOperator) bool {
	switch op {
	case syntax.RdrOut, syntax.AppOut, syntax.ClbOut, syntax.RdrAll, syntax.AppAll:
		return true
	}
	return false
}

func stHasWriteRedirect(st *syntax.Stmt) bool {
	for _, r := range st.Redirs {
		if isWriteOp(r.Op) {
			return true
		}
	}
	return false
}

func hasArg(args []string, s string) bool {
	for _, a := range args {
		if a == s {
			return true
		}
	}
	return false
}

func hasAnyArg(args []string, set ...string) bool {
	for _, a := range args {
		for _, s := range set {
			if a == s {
				return true
			}
		}
	}
	return false
}

// hasShortFlag reports whether any single-dash arg contains the rune
// (handles combined clusters like -fdx).
func hasShortFlag(args []string, r rune) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsRune(a, r) {
			return true
		}
	}
	return false
}
