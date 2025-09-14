package user

import (
	"fmt"

	"github.com/suuuth/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

type NivekUserService interface {
	Signup(request SignupRequest) (bool, error)
	Login(request LoginRequest) (*User, error)
	Logout(request LogoutRequest) (bool, error)

	GetUserById(id int) (*User, error)
	DeleteUserById(id int) error
}

type nivekUserServiceImpl struct {
	nivek     nivek.NivekService
	userTable db.Collection
}

func NewService(service nivek.NivekService) NivekUserService {
	return &nivekUserServiceImpl{
		nivek:     service,
		userTable: service.Postgres().GetDefaultConnection().Collection(TableUser),
	}
}

func (s *nivekUserServiceImpl) GetUserById(id int) (*User, error) {
	var user User

	if err := s.userTable.Find(db.Cond{"id": id}).One(&user); err != nil {
		return nil, fmt.Errorf("error getting user by id: %w", err)
	}

	return &user, nil
}

func (s *nivekUserServiceImpl) DeleteUserById(id int) error {
	if err := s.userTable.Find(db.Cond{"id": id}).Delete(); err != nil {
		return fmt.Errorf("error deleting user by id: %w", err)
	}

	return nil
}

type UpdateUserRequest struct {
	User // pass in entire user struct - just write the whole thing to DB instead of inserting individual cols
}

func (s *nivekUserServiceImpl) UpdateUser(request *UpdateUserRequest) (*User, error) {
	if err := s.userTable.Find(db.Cond{"id": request.User.Id}).Update(request.User); err != nil {
		return nil, fmt.Errorf("error updating user: %w", err)
	}

	return &request.User, nil
}
