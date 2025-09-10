package user

import (
	"fmt"

	"github.com/upper/db/v4"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Service) Login(request LoginRequest) (*User, error) {
	var usr User
	err := s.userTable.Find(db.Cond{
		"email":    request.Email,
		"password": request.Password,
	}).One(&usr)

	if err != nil {
		return nil, fmt.Errorf(
			"error during login attempt - %s: %s",
			request.Email,
			err.Error(),
		)
	}

	return &usr, nil
}
