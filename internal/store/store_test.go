package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.json")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return st
}

func TestAddToggleRemoveSite(t *testing.T) {
	st := openTestStore(t)

	site, err := st.AddSite("YouTube", []string{"youtube.com"}, "youtube")
	if err != nil {
		t.Fatalf("AddSite() error = %v", err)
	}
	if len(st.Sites()) != 1 {
		t.Fatalf("expected 1 site, got %d", len(st.Sites()))
	}

	if err := st.SetEnabled(site.ID, true); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}
	domains := st.BlockedDomains()
	if len(domains) != 1 || domains[0] != "youtube.com" {
		t.Fatalf("BlockedDomains() = %v, want [youtube.com]", domains)
	}

	if err := st.SetEnabled(site.ID, false); err != nil {
		t.Fatalf("SetEnabled(false) error = %v", err)
	}
	if len(st.BlockedDomains()) != 0 {
		t.Fatalf("expected 0 blocked domains after disabling")
	}

	if err := st.RemoveSite(site.ID); err != nil {
		t.Fatalf("RemoveSite() error = %v", err)
	}
	if len(st.Sites()) != 0 {
		t.Fatalf("expected 0 sites after removal")
	}
}

func TestSessionBlocksChangesAndExpires(t *testing.T) {
	st := openTestStore(t)
	site, _ := st.AddSite("TikTok", []string{"tiktok.com"}, "tiktok")

	// Start a short session of 50ms.
	sess, err := st.StartSession([]string{site.ID}, 50*time.Millisecond, "study")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}
	if !sess.Active {
		t.Fatalf("session should be active")
	}

	// The site should have automatically turned into Enabled=true.
	if !st.Sites()[0].Enabled {
		t.Fatalf("the site should have been automatically enabled when starting the session")
	}

	// Trying to disable it while the session is active must fail.
	if err := st.SetEnabled(site.ID, false); err == nil {
		t.Fatalf("expected an error when trying to disable a site locked by an active session")
	}

	// Trying to delete it must also fail.
	if err := st.RemoveSite(site.ID); err == nil {
		t.Fatalf("expected an error when trying to remove a site locked by an active session")
	}

	// Wait for it to expire.
	time.Sleep(80 * time.Millisecond)
	expired := st.ExpireSessionIfDue()
	if !expired {
		t.Fatalf("the session should have expired")
	}

	// Now it should be allowed to be disabled.
	if err := st.SetEnabled(site.ID, false); err != nil {
		t.Fatalf("SetEnabled(false) after session expiration, error = %v", err)
	}
}

func TestStartSession_RejectsWhileAnotherActive(t *testing.T) {
	st := openTestStore(t)
	site, _ := st.AddSite("Instagram", []string{"instagram.com"}, "instagram")

	if _, err := st.StartSession([]string{site.ID}, time.Minute, "one"); err != nil {
		t.Fatalf("first StartSession() error = %v", err)
	}
	if _, err := st.StartSession([]string{site.ID}, time.Minute, "two"); err == nil {
		t.Fatalf("expected an error when starting a second session while the first one is still active")
	}
}

func TestPersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	st1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st1.AddSite("Twitter", []string{"x.com"}, "twitter"); err != nil {
		t.Fatal(err)
	}

	// Reopen from the same path on disk.
	st2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(st2.Sites()) != 1 || st2.Sites()[0].Name != "Twitter" {
		t.Fatalf("data was not properly persisted or reloaded across openings")
	}
}
