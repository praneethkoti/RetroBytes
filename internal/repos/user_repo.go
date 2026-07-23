package repos

import (
	"time"

	"retrobytes/internal/domain"

	"github.com/jmoiron/sqlx"
)

type UserRepo struct{ DB *sqlx.DB }

func NewUserRepo(db *sqlx.DB) *UserRepo { return &UserRepo{DB: db} }

func (r *UserRepo) ByEmail(email string) (*domain.User, error) {
	var u domain.User
	err := r.DB.Get(&u, `SELECT id,email,name,password_hash,role FROM users WHERE LOWER(email)=LOWER(?)`, email)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) ByID(id string) (*domain.User, error) {
	var u domain.User
	err := r.DB.Get(&u, `SELECT id,email,name,password_hash,role FROM users WHERE id=?`, id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// BindSession attaches userID to the session row and sets an absolute expiry
// (now + ttl). The expiry is stored in UTC RFC3339 so it compares correctly
// against SQLite's CURRENT_TIMESTAMP (also UTC) in SessionUser.
func (r *UserRepo) BindSession(sid, userID string, ttl time.Duration) error {
	expiresAt := time.Now().UTC().Add(ttl).Format("2006-01-02 15:04:05")
	_, err := r.DB.Exec(`INSERT INTO sessions(id,user_id,last_seen,expires_at)
                          VALUES(?,?,CURRENT_TIMESTAMP,?)
                          ON CONFLICT(id) DO UPDATE SET user_id=excluded.user_id,last_seen=CURRENT_TIMESTAMP,expires_at=excluded.expires_at`, sid, userID, expiresAt)
	return err
}

// SessionUser resolves the logged-in user for a session id. Expired sessions
// (expires_at in the past) do not resolve a user, so a stolen or idle session
// stops working once it has aged past its TTL.
func (r *UserRepo) SessionUser(sid string) (*domain.User, error) {
	var u domain.User
	err := r.DB.Get(&u, `
      SELECT u.id,u.email,u.name,u.password_hash,u.role
      FROM sessions s
      JOIN users u ON u.id=s.user_id
      WHERE s.id=?
        AND (s.expires_at IS NULL OR s.expires_at > CURRENT_TIMESTAMP)`, sid)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) UnbindSession(sid string) error {
	_, err := r.DB.Exec(`UPDATE sessions SET user_id=NULL,last_seen=CURRENT_TIMESTAMP,expires_at=NULL WHERE id=?`, sid)
	return err
}

// RotateSession issues a fresh session id on login to defeat session fixation.
// It re-keys the caller's anonymous cart and wishlist (both keyed by session
// id, id == session_id) to the new id, binds the new id to the user with a
// fresh expiry, and removes the old anonymous session row. All in one
// transaction so a partial rotation cannot leave the old id usable.
func (r *UserRepo) RotateSession(oldSID, newSID, userID string, ttl time.Duration) error {
	expiresAt := time.Now().UTC().Add(ttl).Format("2006-01-02 15:04:05")

	tx, err := r.DB.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Re-key cart (parent id and child cart_items.cart_id) if one exists.
	if _, err := tx.Exec(`UPDATE cart_items SET cart_id=? WHERE cart_id=?`, newSID, oldSID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE carts SET id=?, session_id=? WHERE session_id=?`, newSID, newSID, oldSID); err != nil {
		return err
	}

	// Re-key wishlist (parent id and child wishlist_items.wishlist_id) if one exists.
	if _, err := tx.Exec(`UPDATE wishlist_items SET wishlist_id=? WHERE wishlist_id=?`, newSID, oldSID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE wishlists SET id=?, session_id=? WHERE session_id=?`, newSID, newSID, oldSID); err != nil {
		return err
	}

	// Remove the old anonymous session so the pre-login sid can never resolve a user.
	if _, err := tx.Exec(`DELETE FROM sessions WHERE id=?`, oldSID); err != nil {
		return err
	}

	// Bind the new session id to the user with a fresh expiry.
	if _, err := tx.Exec(`INSERT INTO sessions(id,user_id,last_seen,expires_at)
                          VALUES(?,?,CURRENT_TIMESTAMP,?)
                          ON CONFLICT(id) DO UPDATE SET user_id=excluded.user_id,last_seen=CURRENT_TIMESTAMP,expires_at=excluded.expires_at`,
		newSID, userID, expiresAt); err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteUserCascade cancels orders and deletes user-related data (sessions, carts, wishlists) while keeping orders for audit.
func (r *UserRepo) DeleteUserCascade(userID string) error {
	tx, err := r.DB.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Find session IDs for this user
	var sessionIDs []string
	if err := tx.Select(&sessionIDs, `SELECT id FROM sessions WHERE user_id=?`, userID); err != nil {
		return err
	}

	// Cancel orders tied to those sessions (retain rows for audit)
	if len(sessionIDs) > 0 {
		query, args, err := sqlx.In(`UPDATE orders SET status='CANCELED' WHERE session_id IN (?)`, sessionIDs)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(query, args...); err != nil {
			return err
		}
		// Delete carts (cart_items cascade)
		query, args, err = sqlx.In(`DELETE FROM carts WHERE id IN (?)`, sessionIDs)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(query, args...); err != nil {
			return err
		}
		// Delete wishlists (wishlist_items cascade)
		query, args, err = sqlx.In(`DELETE FROM wishlists WHERE id IN (?)`, sessionIDs)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(query, args...); err != nil {
			return err
		}
		// Delete sessions
		query, args, err = sqlx.In(`DELETE FROM sessions WHERE id IN (?)`, sessionIDs)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(query, args...); err != nil {
			return err
		}
	}

	// Delete the user-scoped wishlist (keyed by the user's id; wishlist_items
	// cascade). This is separate from the session-keyed cleanup above because
	// wishlists are now bound to the authenticated user id, not the session.
	if _, err := tx.Exec(`DELETE FROM wishlists WHERE id=?`, userID); err != nil {
		return err
	}

	// Finally delete user
	if _, err := tx.Exec(`DELETE FROM users WHERE id=?`, userID); err != nil {
		return err
	}

	return tx.Commit()
}
