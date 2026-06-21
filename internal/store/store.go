// Package store manages the persistence of FocusGuard configuration (site list and active focus session) in a JSON file, using atomic writing and a mutex for thread-safe concurrency.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Site represents a blockable entry (a service with one or more domains).
type Site struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Domains []string `json:"domains"`
	Enabled bool     `json:"enabled"` // true = currently blocked
	Preset  string   `json:"preset"`  // empty if added manually
}

// Session represents a timed "focus session". While Active is true and EndsAt has not passed, sites in SiteIDs cannot be unblocked.
type Session struct {
	Active  bool      `json:"active"`
	EndsAt  time.Time `json:"ends_at"`
	SiteIDs []string  `json:"site_ids"`
	Label   string    `json:"label"`
}

type data struct {
	Sites   []Site  `json:"sites"`
	Session Session `json:"session"`
}

// Store is the thread-safe store backed by a JSON file on disk.
type Store struct {
	mu   sync.Mutex
	path string
	d    data
}

// Open loads (or creates) the state file at the given path.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		s.d = data{Sites: []Site{}, Session: Session{}}
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.d); err != nil {
		return nil, fmt.Errorf("corrupt state file at %s: %w", path, err)
	}
	return s, nil
}

func (s *Store) saveLocked() error {
	raw, err := json.MarshalIndent(s.d, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func newID() string {
	b := make([]byte, 6)
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}

// Sites returns a copy of all sites.
func (s *Store) Sites() []Site {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Site, len(s.d.Sites))
	copy(out, s.d.Sites)
	return out
}

// AddSite adds a new site and persists it.
func (s *Store) AddSite(name string, domains []string, preset string) (Site, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	site := Site{ID: newID(), Name: name, Domains: domains, Enabled: false, Preset: preset}
	s.d.Sites = append(s.d.Sites, site)
	if err := s.saveLocked(); err != nil {
		return Site{}, err
	}
	return site, nil
}

// RemoveSite removes a site by ID. Fails if it is locked by an active session.
func (s *Store) RemoveSite(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isLockedLocked(id) {
		return fmt.Errorf("the site is locked by an active focus session")
	}
	idx := -1
	for i, site := range s.d.Sites {
		if site.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("site not found")
	}
	s.d.Sites = append(s.d.Sites[:idx], s.d.Sites[idx+1:]...)
	return s.saveLocked()
}

// SetEnabled enables/disables the blocking of a site. Fails if it is locked by an active focus session and an attempt is made to disable it.
func (s *Store) SetEnabled(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !enabled && s.isLockedLocked(id) {
		return fmt.Errorf("this site is locked by an active focus session until %s", s.d.Session.EndsAt.Format("15:04"))
	}
	found := false
	for i := range s.d.Sites {
		if s.d.Sites[i].ID == id {
			s.d.Sites[i].Enabled = enabled
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("site not found")
	}
	return s.saveLocked()
}

func (s *Store) isLockedLocked(id string) bool {
	if !s.d.Session.Active || time.Now().After(s.d.Session.EndsAt) {
		return false
	}
	for _, sid := range s.d.Session.SiteIDs {
		if sid == id {
			return true
		}
	}
	return false
}

// Session returns the current session (it may be expired; the caller checks EndsAt).
func (s *Store) Session() Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.d.Session
}

// StartSession starts a focus session: it marks the given sites as enabled and locks them (they cannot be disabled) until it ends.
func (s *Store) StartSession(siteIDs []string, duration time.Duration, label string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.d.Session.Active && time.Now().Before(s.d.Session.EndsAt) {
		return Session{}, fmt.Errorf("there is already an active focus session until %s", s.d.Session.EndsAt.Format("15:04"))
	}

	valid := map[string]bool{}
	for _, site := range s.d.Sites {
		valid[site.ID] = true
	}
	for _, id := range siteIDs {
		if !valid[id] {
			return Session{}, fmt.Errorf("invalid site: %s", id)
		}
	}

	for i := range s.d.Sites {
		for _, id := range siteIDs {
			if s.d.Sites[i].ID == id {
				s.d.Sites[i].Enabled = true
			}
		}
	}

	sess := Session{
		Active:  true,
		EndsAt:  time.Now().Add(duration),
		SiteIDs: siteIDs,
		Label:   label,
	}
	s.d.Session = sess
	if err := s.saveLocked(); err != nil {
		return Session{}, err
	}
	return sess, nil
}

// ExpireSessionIfDue marks the session as inactive if EndsAt has already passed.
// Returns true if there was an active session that just expired.
func (s *Store) ExpireSessionIfDue() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.d.Session.Active && time.Now().After(s.d.Session.EndsAt) {
		s.d.Session.Active = false
		s.saveLocked() //nolint:errcheck
		return true
	}
	return false
}

// BlockedDomains returns the combined list of domains from all sites currently enabled (either via manual toggle or focus session).
func (s *Store) BlockedDomains() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, site := range s.d.Sites {
		if site.Enabled {
			out = append(out, site.Domains...)
		}
	}
	return out
}
