package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type store struct{ db *sql.DB }

type session struct {
	ID, Sub, Email, CSRF string
	Expires              time.Time
}
type deviceRow struct {
	Name, UUID, Status, StatusLabel, StatusClass string
	Expires, ExpiresISO                          string
	CanCancel                                    bool
}

func openStore(path string) (*store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &store{db: db}
	schema := `
CREATE TABLE IF NOT EXISTS oauth_states (
 id TEXT PRIMARY KEY, state_hash BLOB NOT NULL, nonce TEXT NOT NULL,
 verifier TEXT NOT NULL, expires_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
 id TEXT PRIMARY KEY, sub TEXT NOT NULL, email TEXT NOT NULL, csrf TEXT NOT NULL,
 expires_at INTEGER NOT NULL, created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS enrollments (
 id TEXT PRIMARY KEY, sub TEXT NOT NULL, email TEXT NOT NULL, device_name TEXT NOT NULL,
 device_uuid TEXT NOT NULL UNIQUE, identity_cn TEXT NOT NULL UNIQUE,
 challenge_hash BLOB NOT NULL UNIQUE, challenge_expires_at INTEGER NOT NULL,
 transaction_id TEXT, consumed_at INTEGER, issued_at INTEGER, expires_at INTEGER NOT NULL,
 serial TEXT, status TEXT NOT NULL DEFAULT 'pending', revoked_at INTEGER,
 audit_ref TEXT, created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS enrollments_sub_idx ON enrollments(sub, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS enrollments_serial_idx ON enrollments(serial) WHERE serial IS NOT NULL;
`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *store) putOAuth(ctx context.Context, id string, stateHash []byte, nonce, verifier string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO oauth_states VALUES(?,?,?,?,?)`,
		id, stateHash, nonce, verifier, expires.Unix())
	return err
}

func (s *store) consumeOAuth(ctx context.Context, id string, now time.Time) ([]byte, string, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", "", err
	}
	defer tx.Rollback()
	var hash []byte
	var nonce, verifier string
	var expires int64
	err = tx.QueryRowContext(ctx, `SELECT state_hash,nonce,verifier,expires_at FROM oauth_states WHERE id=?`, id).
		Scan(&hash, &nonce, &verifier, &expires)
	if err != nil || expires < now.Unix() {
		return nil, "", "", errors.New("invalid or expired oidc state")
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM oauth_states WHERE id=?`, id); err != nil {
		return nil, "", "", err
	}
	return hash, nonce, verifier, tx.Commit()
}

func (s *store) createSession(ctx context.Context, sess session) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions VALUES(?,?,?,?,?,?)`,
		sess.ID, sess.Sub, sess.Email, sess.CSRF, sess.Expires.Unix(), time.Now().UTC().Unix())
	return err
}

func (s *store) getSession(ctx context.Context, id string, now time.Time) (session, error) {
	var out session
	var expires int64
	err := s.db.QueryRowContext(ctx, `SELECT id,sub,email,csrf,expires_at FROM sessions WHERE id=? AND expires_at>=?`,
		id, now.Unix()).Scan(&out.ID, &out.Sub, &out.Email, &out.CSRF, &expires)
	out.Expires = time.Unix(expires, 0).UTC()
	return out, err
}

func (s *store) createEnrollment(ctx context.Context, e enrollment, maxPerDay int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	since := time.Now().UTC().Add(-24 * time.Hour).Unix()
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM enrollments
WHERE sub=? AND created_at>=? AND status!='cancelled'`, e.Sub, since).Scan(&count); err != nil {
		return err
	}
	if count >= maxPerDay {
		return errors.New("daily enrollment limit reached")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO enrollments
(id,sub,email,device_name,device_uuid,identity_cn,challenge_hash,challenge_expires_at,expires_at,audit_ref,created_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?)`, e.ID, e.Sub, e.Email, e.DeviceName, e.DeviceUUID, e.IdentityCN,
		e.ChallengeHash, e.ChallengeExpires.Unix(), e.Expires.Unix(), e.AuditRef, time.Now().UTC().Unix())
	if err != nil {
		return err
	}
	return tx.Commit()
}

type enrollment struct {
	ID, Sub, Email, DeviceName, DeviceUUID, IdentityCN, AuditRef string
	ChallengeHash                                                []byte
	ChallengeExpires, Expires                                    time.Time
}

func (s *store) consumeChallenge(ctx context.Context, hash []byte, transactionID string, now time.Time, retryWindow time.Duration) (enrollment, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return enrollment{}, false, err
	}
	defer tx.Rollback()
	var e enrollment
	var challengeExpires, expires int64
	var existing sql.NullString
	var consumed sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT id,sub,email,device_name,device_uuid,identity_cn,
challenge_hash,challenge_expires_at,expires_at,transaction_id,consumed_at,audit_ref
FROM enrollments WHERE challenge_hash=? AND status IN ('pending','requested')`, hash).Scan(
		&e.ID, &e.Sub, &e.Email, &e.DeviceName, &e.DeviceUUID, &e.IdentityCN,
		&e.ChallengeHash, &challengeExpires, &expires, &existing, &consumed, &e.AuditRef)
	if err != nil || challengeExpires < now.Unix() {
		return enrollment{}, false, nil
	}
	e.ChallengeExpires, e.Expires = time.Unix(challengeExpires, 0), time.Unix(expires, 0)
	switch {
	case !existing.Valid:
		_, err = tx.ExecContext(ctx, `UPDATE enrollments SET transaction_id=?,consumed_at=?,status='requested' WHERE id=? AND transaction_id IS NULL`,
			transactionID, now.Unix(), e.ID)
	case existing.String == transactionID && consumed.Valid && now.Before(time.Unix(consumed.Int64, 0).Add(retryWindow)):
		// SCEP devices may retry the identical transaction briefly.
	default:
		return enrollment{}, false, nil
	}
	if err != nil {
		return enrollment{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return enrollment{}, false, err
	}
	return e, true, nil
}

func (s *store) listDevices(ctx context.Context, sub string) ([]deviceRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT device_name,device_uuid,status,expires_at
FROM enrollments WHERE sub=? AND status!='cancelled' ORDER BY created_at DESC`, sub)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []deviceRow
	for rows.Next() {
		var d deviceRow
		var expires int64
		if err := rows.Scan(&d.Name, &d.UUID, &d.Status, &expires); err != nil {
			return nil, err
		}
		expiry := time.Unix(expires, 0).UTC()
		d.Expires = expiry.Format("02.01.2006")
		d.ExpiresISO = expiry.Format(time.RFC3339)
		d.StatusLabel, d.StatusClass = deviceStatusPresentation(d.Status)
		d.CanCancel = d.Status == "pending" || d.Status == "requested"
		out = append(out, d)
	}
	return out, rows.Err()
}

func deviceStatusPresentation(status string) (string, string) {
	switch status {
	case "issued":
		return "Aktiv", "status-good"
	case "pending":
		return "Profil bereit", "status-warn"
	case "requested":
		return "Wird ausgestellt", "status-warn"
	case "revoked":
		return "Gesperrt", "status-danger"
	default:
		return "Unbekannt", "status-neutral"
	}
}

func (s *store) cancelEnrollment(ctx context.Context, sub, deviceUUID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE enrollments
SET status='cancelled',challenge_expires_at=0
WHERE sub=? AND device_uuid=? AND serial IS NULL AND status IN ('pending','requested')`,
		sub, deviceUUID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return errors.New("pending enrollment not found")
	}
	return nil
}

func (s *store) listAdmin(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT sub,device_name,device_uuid,coalesce(serial,'UNBOUND'),status,
datetime(expires_at,'unixepoch') FROM enrollments ORDER BY created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sub, name, uuid, serial, status, expires string
		if err := rows.Scan(&sub, &name, &uuid, &serial, &status, &expires); err != nil {
			return err
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\n", sub, name, uuid, serial, status, expires)
	}
	return rows.Err()
}

func (s *store) bindSerial(ctx context.Context, uuid, serial string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE enrollments SET serial=?,status='issued',issued_at=? WHERE device_uuid=? AND serial IS NULL`,
		serial, time.Now().UTC().Unix(), uuid)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return errors.New("device not found or serial already bound")
	}
	return nil
}

func (s *store) markRevoked(ctx context.Context, serial string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE enrollments
SET status='revoked',revoked_at=? WHERE serial=? AND status!='revoked'`,
		time.Now().UTC().Unix(), serial)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return errors.New("certificate not found or already revoked")
	}
	return nil
}
