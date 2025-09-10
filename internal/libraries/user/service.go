package user

import (
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
