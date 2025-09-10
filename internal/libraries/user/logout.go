package user

type LogoutRequest struct{}

func (s *Service) Logout(request *LogoutRequest) (bool, error) {
	if request == nil {
		return false, nil
	}

	return true, nil
}
