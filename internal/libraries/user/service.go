package user

import (
	"errors"
	"fmt"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

type NivekUserService interface {
	Logout(request LogoutRequest) (bool, error)

	CreateNewUser(newUser *User) error
	GetAllActiveUsers() ([]User, error)
	GetUserById(id int) (*User, error)
	GetUserByUsername(username string) (*User, error)
	DeleteUserById(id int) error
	GetUserByBroadcasterId(id string) (*User, error)
	UpdateUser(u *User) error
	PutChannelState(broadcasterUserLogin string, isLive bool) error

	FindOrCreateByTwitchID(profile TwitchProfile) (*User, bool, error)
}

type TwitchProfile struct {
	ID          string
	Login       string
	DisplayName string
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

func (s *nivekUserServiceImpl) CreateNewUser(newUser *User) error {
	if err := s.userTable.InsertReturning(newUser); err != nil {
		return fmt.Errorf("failed to create new user record in db: %s", err.Error())
	}

	return nil
}

func (s *nivekUserServiceImpl) GetAllActiveUsers() ([]User, error) {
	var users []User

	if err := s.userTable.Find(db.Cond{
		"bot_opt_in": true,
	}).All(&users); err != nil {
		return nil, fmt.Errorf("error getting all users: %w", err)
	}

	return users, nil
}

// GetUserByUsername - used for self-healing legacy users
func (s *nivekUserServiceImpl) GetUserByUsername(username string) (*User, error) {
	var user User

	if err := s.userTable.Find(db.Cond{"username": username}).One(&user); err != nil {
		return nil, fmt.Errorf("failed to fetch user by username: %w", err)
	}

	return &user, nil
}

func (s *nivekUserServiceImpl) GetUserById(id int) (*User, error) {
	var user User

	if err := s.userTable.Find(db.Cond{"id": id}).One(&user); err != nil {
		return nil, fmt.Errorf("error getting user by id: %w", err)
	}

	return &user, nil
}

func (s *nivekUserServiceImpl) GetUserByBroadcasterId(id string) (*User, error) {
	var user User

	if err := s.userTable.Find(db.Cond{"twitch_id": id}).One(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *nivekUserServiceImpl) UpdateUser(u *User) error {
	if err := s.userTable.UpdateReturning(u); err != nil {
		return fmt.Errorf("failed to update user: %+v - %w", u, err)
	}

	return nil
}

func (s *nivekUserServiceImpl) PutChannelState(broadcasterUserLogin string, isLive bool) error {
	// UpdateReturning updates the row identified by the item's primary key, so
	// load the user by twitch_login first to get their id (and current fields),
	// then flip is_live and write it back.
	var user User
	if err := s.userTable.Find(db.Cond{"twitch_login": broadcasterUserLogin}).One(&user); err != nil {
		return fmt.Errorf("failed to load broadcaster %s for state update: %w", broadcasterUserLogin, err)
	}

	user.IsLive = isLive
	if err := s.userTable.UpdateReturning(&user); err != nil {
		return fmt.Errorf("failed to update broadcaster state for broadcaster %s state %v - %w", broadcasterUserLogin, isLive, err)
	}

	return nil
}

func (s *nivekUserServiceImpl) DeleteUserById(id int) error {
	if err := s.userTable.Find(db.Cond{"id": id}).Delete(); err != nil {
		return fmt.Errorf("error deleting user by id: %w", err)
	}

	return nil
}

// FindOrCreateByTwitchID resolves the canonical user row for a Twitch login.
// Lookup order:
//  1. By twitch_id — the canonical key for OAuth-created users.
//  2. By username (case-insensitive) — legacy rows from the pre-OAuth
//     email/password era used the streamer's Twitch login as their username,
//     so a match here means we're claiming an existing row instead of
//     stranding it. Backfill the Twitch columns onto the existing row and
//     return it so user_id stays stable.
//  3. Otherwise INSERT a new row.
//
// Display name + login are refreshed on every login so renames on Twitch
// propagate to our DB.
func (s *nivekUserServiceImpl) FindOrCreateByTwitchID(profile TwitchProfile) (*User, bool, error) {
	var existing User
	err := s.userTable.Find(db.Cond{"twitch_id": profile.ID}).One(&existing)
	if err == nil {
		if derefOrEmpty(existing.TwitchLogin) != profile.Login || derefOrEmpty(existing.TwitchDisplayName) != profile.DisplayName {
			existing.TwitchLogin = &profile.Login
			existing.TwitchDisplayName = &profile.DisplayName
			existing.Username = profile.Login
			if err := s.userTable.Find(db.Cond{"id": existing.Id}).Update(existing); err != nil {
				return nil, false, fmt.Errorf("error refreshing twitch user fields: %w", err)
			}
		}
		return &existing, false, nil
	}
	if !errors.Is(err, db.ErrNoMoreRows) {
		return nil, false, fmt.Errorf("error looking up user by twitch_id: %w", err)
	}

	// No twitch_id match — look for a legacy row by username before inserting.
	// ILIKE handles any case mismatch between historical stored usernames and
	// the lowercase login Twitch returns.
	var legacy User
	err = s.userTable.Find(db.Cond{"username ILIKE": profile.Login}).One(&legacy)
	if err == nil {
		legacy.TwitchID = &profile.ID
		legacy.TwitchLogin = &profile.Login
		legacy.TwitchDisplayName = &profile.DisplayName
		legacy.Username = profile.Login
		if err := s.userTable.Find(db.Cond{"id": legacy.Id}).Update(legacy); err != nil {
			return nil, false, fmt.Errorf("error backfilling twitch fields onto legacy user: %w", err)
		}
		return &legacy, false, nil
	}
	if !errors.Is(err, db.ErrNoMoreRows) {
		return nil, false, fmt.Errorf("error looking up legacy user by username: %w", err)
	}

	newUser := User{
		Username:          profile.Login,
		TwitchID:          &profile.ID,
		TwitchLogin:       &profile.Login,
		TwitchDisplayName: &profile.DisplayName,
	}
	result, err := s.userTable.Insert(newUser)
	if err != nil {
		return nil, false, fmt.Errorf("error inserting twitch user: %w", err)
	}

	newUser.Id = int(result.ID().(int64))

	return &newUser, true, nil
}

func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
