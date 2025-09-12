package user

import (
	"fmt"

	"github.com/suuuth/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

type Service struct {
	nivek     nivek.NivekService
	userTable db.Collection
}

func NewService(service nivek.NivekService) *Service {
	return &Service{
		nivek:     service,
		userTable: service.Postgres().GetDefaultConnection().Collection(TableUser),
	}
}

func (s *Service) GetUserById(id int) (*User, error) {
	var user User

	if err := s.userTable.Find(db.Cond{"id": id}).One(&user); err != nil {
		return nil, fmt.Errorf("error getting user by id: %w", err)
	}

	return &user, nil
}
