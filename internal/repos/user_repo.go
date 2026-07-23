package repos

import (
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

func (r *UserRepo) BindSession(sid, userID string) error {
	_, err := r.DB.Exec(`INSERT INTO sessions(id,user_id,last_seen) 
                          VALUES(?,?,CURRENT_TIMESTAMP)
                          ON CONFLICT(id) DO UPDATE SET user_id=excluded.user_id,last_seen=CURRENT_TIMESTAMP`, sid, userID)
	return err
}

func (r *UserRepo) SessionUser(sid string) (*domain.User, error) {
	var u domain.User
	err := r.DB.Get(&u, `
      SELECT u.id,u.email,u.name,u.password_hash,u.role
      FROM sessions s 
      JOIN users u ON u.id=s.user_id
      WHERE s.id=?`, sid)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) UnbindSession(sid string) error {
	_, err := r.DB.Exec(`UPDATE sessions SET user_id=NULL,last_seen=CURRENT_TIMESTAMP WHERE id=?`, sid)
	return err
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

	// Finally delete user
	if _, err := tx.Exec(`DELETE FROM users WHERE id=?`, userID); err != nil {
		return err
	}

	return tx.Commit()
}
