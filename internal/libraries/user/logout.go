package user

type LogoutRequest struct{}

func (s *nivekUserServiceImpl) Logout(request LogoutRequest) (bool, error) {
	return true, nil
}
