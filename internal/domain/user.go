package domain

type User struct {
	ID    string `db:"id"`
	Email string `db:"email"`
	Name  string `db:"name"`
	Hash  string `db:"password_hash"`
	Role  string `db:"role"`
}
