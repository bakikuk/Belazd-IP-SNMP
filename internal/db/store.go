package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bakikuk/belazd-ip-snmp/internal/config"
	_ "modernc.org/sqlite"
)

// Store persists Belazd configuration in SQLite.
type Store struct {
	path string
	db   *sql.DB
}

// Open creates or opens the database at path.
func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultDataPath()
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{path: path, db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schemaV1); err != nil {
		return err
	}
	return migrateV2(s.db)
}

func migrateV2(db *sql.DB) error {
	var hasCol int
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('queues') WHERE name='capture_iface'`).Scan(&hasCol)
	if err != nil {
		return err
	}
	if hasCol == 0 {
		if _, err := db.Exec(`ALTER TABLE queues ADD COLUMN capture_iface TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

// Path returns the database file path.
func (s *Store) Path() string { return s.path }

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// HasUsers reports whether any login accounts exist.
func (s *Store) HasUsers() (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n > 0, err
}

// IsEmpty reports no settings, queues, or users (fresh DB).
func (s *Store) IsEmpty() (bool, error) {
	var settings, queues, users int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&settings); err != nil {
		return false, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM queues`).Scan(&queues); err != nil {
		return false, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		return false, err
	}
	return settings == 0 && queues == 0 && users == 0, nil
}

type settingsBlob struct {
	Capture config.CaptureConfig `json:"capture"`
	SNMP    config.SNMPConfig    `json:"snmp"`
	HTTP    config.HTTPConfig    `json:"http"`
	Meta    config.Meta          `json:"meta"`
}

// LoadConfig reads the full in-memory config from SQLite.
func (s *Store) LoadConfig() (*config.Config, error) {
	c := config.Default()
	row := s.db.QueryRow(`SELECT value FROM settings WHERE key = 'core'`)
	var raw string
	err := row.Scan(&raw)
	if err == sql.ErrNoRows {
		// partial empty
	} else if err != nil {
		return nil, err
	} else {
		var blob settingsBlob
		if err := json.Unmarshal([]byte(raw), &blob); err != nil {
			return nil, err
		}
		c.Capture = blob.Capture
		c.SNMP = blob.SNMP
		c.HTTP = blob.HTTP
		c.Meta = blob.Meta
		config.RepairSNMPFromCoreJSON(&c.SNMP, []byte(raw))
	}
	c.Capture.NormalizeInterfaces()
	c.HTTP.Listen = config.FixedHTTPListen

	rows, err := s.db.Query(`SELECT idx, target, label, capture_iface FROM queues ORDER BY idx`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	c.Queues = nil
	for rows.Next() {
		var q config.Queue
		if err := rows.Scan(&q.Index, &q.Target, &q.Label, &q.CaptureIface); err != nil {
			return nil, err
		}
		c.Queues = append(c.Queues, q)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	grows, err := s.db.Query(`SELECT id, name, permissions_json FROM user_groups ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer grows.Close()
	c.UserGroups = nil
	for grows.Next() {
		var g config.UserGroup
		var permJSON string
		if err := grows.Scan(&g.ID, &g.Name, &permJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(permJSON), &g.Permissions); err != nil {
			return nil, err
		}
		g.Permissions.BackfillLegacyAbout()
		c.UserGroups = append(c.UserGroups, g)
	}
	if err := grows.Err(); err != nil {
		return nil, err
	}

	users, err := s.db.Query(`SELECT username, group_id, password_hash FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer users.Close()
	c.Users = nil
	for users.Next() {
		var u config.UserAccount
		if err := users.Scan(&u.Username, &u.GroupID, &u.PasswordHash); err != nil {
			return nil, err
		}
		c.Users = append(c.Users, u)
	}
	return c, users.Err()
}

// SaveConfig replaces persisted state from c (transactional).
func (s *Store) SaveConfig(c *config.Config) error {
	c.HTTP.Listen = config.FixedHTTPListen
	c.Capture.NormalizeInterfaces()
	if c.Capture.Snaplen <= 0 {
		c.Capture.Snaplen = 65535
	}
	if c.Meta.NextIndex <= 0 {
		c.Meta.NextIndex = 1
	}
	blob := settingsBlob{
		Capture: c.Capture,
		SNMP:    c.SNMP,
		HTTP:    c.HTTP,
		Meta:    c.Meta,
	}
	coreJSON, err := json.Marshal(blob)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`INSERT INTO settings(key,value) VALUES('core',?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, string(coreJSON)); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM queues`); err != nil {
		return err
	}
	for _, q := range c.Queues {
		if _, err := tx.Exec(`INSERT INTO queues(idx,target,label,capture_iface) VALUES(?,?,?,?)`, q.Index, q.Target, q.Label, q.CaptureIface); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM users`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM user_groups`); err != nil {
		return err
	}
	for _, g := range c.UserGroups {
		pj, err := json.Marshal(g.Permissions)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO user_groups(id,name,permissions_json) VALUES(?,?,?)`, g.ID, g.Name, string(pj)); err != nil {
			return err
		}
	}
	for _, u := range c.Users {
		if _, err := tx.Exec(`INSERT INTO users(username,group_id,password_hash) VALUES(?,?,?)`, u.Username, u.GroupID, u.PasswordHash); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := os.Chmod(s.path, 0o600); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("chmod db: %w", err)
	}
	return nil
}
