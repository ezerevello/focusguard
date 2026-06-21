package hostsfile

import (
	"os"
	"strings"
	"testing"
)

// These tests exercise the internal logic (stripManagedBlock, dedupSorted, atomicWrite) directly, without touching the system's actual hosts file.
//Path() is fixed to /etc/hosts or the Windows path, so high-level tests are run against a temporary file replicating the same steps as Apply().

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}
	return string(b)
}

func TestStripManagedBlock(t *testing.T) {
	original := "127.0.0.1 localhost\n" +
	startMarker + "\n" +
	"127.0.0.1 youtube.com\n" +
	endMarker + "\n" +
	"10.0.0.5 myserver\n"

	got := stripManagedBlock(original)
	want := "127.0.0.1 localhost\n10.0.0.5 myserver"
	if got != want {
		t.Fatalf("stripManagedBlock() = %q, want %q", got, want)
	}
}

func TestStripManagedBlock_NoExistingBlock(t *testing.T) {
	original := "127.0.0.1 localhost\n::1 localhost\n"
	got := stripManagedBlock(original)
	want := "127.0.0.1 localhost\n::1 localhost"
	if got != want {
		t.Fatalf("stripManagedBlock() = %q, want %q", got, want)
	}
}

func TestDedupSorted(t *testing.T) {
	in := []string{"YouTube.com", "www.youtube.com", "  instagram.com ", "youtube.com"}
	got := dedupSorted(in)
	want := []string{"instagram.com", "youtube.com"}
	if len(got) != len(want) {
		t.Fatalf("dedupSorted() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dedupSorted() = %v, want %v", got, want)
		}
	}
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/hosts"

	if err := atomicWrite(path, "test line\n"); err != nil {
		t.Fatalf("atomicWrite() error = %v", err)
	}
	if got := readFile(t, path); !strings.Contains(got, "test line") {
		t.Fatalf("unexpected content: %q", got)
	}

	// Overwriting again must work (simulates a second application).
	if err := atomicWrite(path, "another line\n"); err != nil {
		t.Fatalf("second atomicWrite() error = %v", err)
	}
	got := readFile(t, path)
	if strings.Contains(got, "test line") || !strings.Contains(got, "another line") {
		t.Fatalf("the second write did not replace the content: %q", got)
	}
}

func TestFullApplyFlow_AgainstFakeHostsFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/hosts"
	if err := atomicWrite(path, "127.0.0.1 localhost\n::1 localhost\n"); err != nil {
		t.Fatal(err)
	}

	apply := func(domains []string) string {
		original := readFile(t, path)
		kept := stripManagedBlock(original)
		var b strings.Builder
		b.WriteString(kept)
		if !strings.HasSuffix(kept, "\n") {
			b.WriteString("\n")
		}
		if len(domains) > 0 {
			b.WriteString(startMarker + "\n")
			for _, d := range dedupSorted(domains) {
				b.WriteString("127.0.0.1 " + d + "\n")
				b.WriteString("127.0.0.1 www." + d + "\n")
			}
			b.WriteString(endMarker + "\n")
		}
		if err := atomicWrite(path, b.String()); err != nil {
			t.Fatal(err)
		}
		return readFile(t, path)
	}

	out := apply([]string{"youtube.com", "tiktok.com"})
	if !strings.Contains(out, "127.0.0.1 youtube.com") {
		t.Fatalf("expected youtube.com to be blocked, got: %s", out)
	}
	if !strings.Contains(out, "127.0.0.1 localhost") {
		t.Fatalf("original content was lost, got: %s", out)
	}

	// Re-applying with a different list should not duplicate or leave garbage.
	out2 := apply([]string{"instagram.com"})
	if strings.Contains(out2, "youtube.com") {
		t.Fatalf("youtube.com should have been removed, got: %s", out2)
	}
	if !strings.Contains(out2, "instagram.com") {
		t.Fatalf("expected instagram.com to be blocked, got: %s", out2)
	}
	if strings.Count(out2, startMarker) != 1 {
		t.Fatalf("the marker should not be duplicated, got: %s", out2)
	}

	// Applying an empty list should completely clear the managed block.
	out3 := apply(nil)
	if strings.Contains(out3, startMarker) {
		t.Fatalf("the block should have been removed, got: %s", out3)
	}
	if !strings.Contains(out3, "127.0.0.1 localhost") {
		t.Fatalf("original content was lost after clearing, got: %s", out3)
	}
}
