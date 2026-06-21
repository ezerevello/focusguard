// Package hostsfile handles the secure editing of the system hosts file (Linux: /etc/hosts, Windows: C:\Windows\System32\drivers\etc\hosts) to redirect blocked domains to localhost.
package hostsfile

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	startMarker = "# === FocusGuard START (managed automatically, do not edit manually) ==="
	endMarker   = "# === FocusGuard END ==="
)

// Path returns the hosts file path depending on the operating system.
func Path() string {
	if runtime.GOOS == "windows" {
		winDir := os.Getenv("WINDIR")
		if winDir == "" {
			winDir = `C:\Windows`
		}
		return filepath.Join(winDir, "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}

// Apply rewrites the FocusGuard-managed block within the hosts file with the given list of domains (without touching the rest of the file).
func Apply(domains []string) error {
	path := Path()

	original, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read %s: %w (are you running as administrator/root?)", path, err)
	}

	kept := stripManagedBlock(string(original))

	var b strings.Builder
	b.WriteString(kept)
	if !strings.HasSuffix(kept, "\n") {
		b.WriteString("\n")
	}

	if len(domains) > 0 {
		b.WriteString(startMarker + "\n")
		uniq := dedupSorted(domains)
		for _, d := range uniq {
			b.WriteString(fmt.Sprintf("127.0.0.1 %s\n", d))
			b.WriteString(fmt.Sprintf("127.0.0.1 www.%s\n", d))
			b.WriteString(fmt.Sprintf("::1 %s\n", d))
			b.WriteString(fmt.Sprintf("::1 www.%s\n", d))
		}
		b.WriteString(endMarker + "\n")
	}

	if err := atomicWrite(path, b.String()); err != nil {
		return err
	}

	flushDNSCache() // best-effort, ignore errors
	return nil
}

// CurrentlyBlocked reads the managed block and returns the domains currently redirected (useful for diagnostics/status).
func CurrentlyBlocked() ([]string, error) {
	original, err := os.ReadFile(Path())
	if err != nil {
		return nil, err
	}
	in := false
	seen := map[string]bool{}
	var out []string
	sc := bufio.NewScanner(strings.NewReader(string(original)))
	for sc.Scan() {
		line := sc.Text()
		trim := strings.TrimSpace(line)
		if trim == startMarker {
			in = true
			continue
		}
		if trim == endMarker {
			in = false
			continue
		}
		if in {
			fields := strings.Fields(trim)
			if len(fields) == 2 {
				d := strings.TrimPrefix(fields[1], "www.")
				if !seen[d] {
					seen[d] = true
					out = append(out, d)
				}
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

func stripManagedBlock(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	in := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == startMarker {
			in = true
			continue
		}
		if trim == endMarker {
			in = false
			continue
		}
		if in {
			continue
		}
		out = append(out, line)
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

func dedupSorted(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range in {
		d = strings.ToLower(strings.TrimSpace(d))
		d = strings.TrimPrefix(d, "www.")
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// atomicWrite writes to a temporary file in the same directory and then renames it, to avoid leaving the hosts file corrupted if something fails halfway through.
func atomicWrite(path, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".focusguard-hosts-*")
	if err != nil {
		return fmt.Errorf("could not create temporary file (permissions?): %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op if the rename was successful

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if info, statErr := os.Stat(path); statErr == nil {
		_ = os.Chmod(tmpPath, info.Mode())
	}

	if err := os.Rename(tmpPath, path); err != nil {
		if werr := os.WriteFile(path, []byte(content), 0644); werr != nil {
			return fmt.Errorf("could not write to %s: %w", path, werr)
		}
	}
	return nil
}

// flushDNSCache attempts to clear the system's DNS cache. It's best-effort: if the command does not exist or fails, we simply ignore it.
func flushDNSCache() {
	var cmds [][]string
	switch runtime.GOOS {
		case "windows":
			cmds = [][]string{{"ipconfig", "/flushdns"}}
		case "linux":
			cmds = [][]string{
				{"resolvectl", "flush-caches"},
				{"systemd-resolve", "--flush-caches"},
				{"nscd", "-i", "hosts"},
			}
		case "darwin":
			cmds = [][]string{
				{"dscacheutil", "-flushcache"},
				{"killall", "-HUP", "mDNSResponder"},
			}
	}
	for _, c := range cmds {
		exec.Command(c[0], c[1:]...).Run() //nolint:errcheck
	}
}
