package user

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type SignupRequest struct {
	Username string `db:"username" json:"username"`
	Email    string `db:"email" json:"email"`
	Password string `db:"password" json:"password"`
}

func (s *nivekUserServiceImpl) Signup(request SignupRequest) (bool, error) {
	result, err := s.userTable.Insert(request)

	if err != nil {
		return false, fmt.Errorf("error inserting user: %s", err.Error())
	}

	logrus.Infof("User %d created", result.ID())
	return true, nil
}
