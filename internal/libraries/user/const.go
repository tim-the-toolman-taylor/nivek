package user

const TableUser = "users"

type User struct {
	Id        int    `db:"id" json:"id"`
	Username  string `db:"username" json:"username"`
	Email     string `db:"email" json:"email"`
	CreatedAt string `db:"created_at" json:"created_at"`
	Role      string `db:"role" json:"role"`
}
