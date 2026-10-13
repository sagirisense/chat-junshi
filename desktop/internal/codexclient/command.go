package codexclient

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

func isFile(path string) bool { info, err := os.Stat(path); return err == nil && !info.IsDir() }

func Command() ([]string, error) {
	home, _ := os.UserHomeDir()
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local = filepath.Join(home, "AppData", "Local")
	}
	// Prefer the desktop client's bundled CLI over npm/PATH installations.
	binaries, _ := filepath.Glob(filepath.Join(local, "OpenAI", "Codex", "bin", "*", "codex.exe"))
	type desktopBinary struct {
		path string
		info os.FileInfo
	}
	available := []desktopBinary{}
	for _, path := range binaries {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			available = append(available, desktopBinary{path, info})
		}
	}
	sort.SliceStable(available, func(i, j int) bool { return available[i].info.ModTime().After(available[j].info.ModTime()) })
	if len(available) > 0 {
		return []string{available[0].path, "app-server"}, nil
	}
	shim, _ := exec.LookPath("codex.cmd")
	if shim == "" {
		shim, _ = exec.LookPath("codex")
	}
	if shim == "" {
		home, _ := os.UserHomeDir()
		roaming := os.Getenv("APPDATA")
		local := os.Getenv("LOCALAPPDATA")
		if roaming == "" {
			roaming = filepath.Join(home, "AppData", "Roaming")
		}
		if local == "" {
			local = filepath.Join(home, "AppData", "Local")
		}
		candidates := []string{filepath.Join(roaming, "npm", "codex.cmd")}
		for _, root := range []string{roaming, local} {
			versions, _ := filepath.Glob(filepath.Join(root, "fnm", "node-versions", "*", "installation", "codex.cmd"))
			sort.Sort(sort.Reverse(sort.StringSlice(versions)))
			candidates = append(candidates, versions...)
		}
		for _, path := range candidates {
			if isFile(path) {
				shim = path
				break
			}
		}
	}
	if shim == "" {
		return nil, errors.New("未找到 Codex CLI，请安装并登录，或使用自定义 API")
	}
	ext := strings.ToLower(filepath.Ext(shim))
	if ext != ".cmd" && ext != ".ps1" {
		return []string{shim, "app-server"}, nil
	}
	directory := filepath.Dir(shim)
	pkg := filepath.Join(directory, "node_modules", "@openai", "codex")
	script := filepath.Join(pkg, "bin", "codex.js")
	arch, target := "x64", "x86_64-pc-windows-msvc"
	if runtime.GOARCH == "arm64" {
		arch, target = "arm64", "aarch64-pc-windows-msvc"
	}
	for _, vendor := range []string{filepath.Join(pkg, "node_modules", "@openai", "codex-win32-"+arch, "vendor"), filepath.Join(filepath.Dir(pkg), "codex-win32-"+arch, "vendor"), filepath.Join(pkg, "vendor")} {
		binary := filepath.Join(vendor, target, "bin", "codex.exe")
		if isFile(binary) {
			return []string{binary, "app-server"}, nil
		}
	}
	node := filepath.Join(directory, "node.exe")
	if !isFile(node) {
		node, _ = exec.LookPath("node")
	}
	if node == "" || !isFile(script) {
		return nil, errors.New("未找到 Codex 的 Node 入口，请检查 CLI 安装")
	}
	return []string{node, script, "app-server"}, nil
}
