package service

import "github.com/kyangconn/music-online-go/internal/domain"

func (s *userService) UpdateUserStatus(id uint, isActive bool) error {
	return s.userRepo.UpdateStatus(id, isActive)
}

func (s *userService) UpdateUserRole(id uint, role string) error {
	return s.userRepo.UpdateRole(id, role)
}

func (s *userService) SearchUsers(query string, page, pageSize int) ([]*domain.UserResponse, int64, error) {
	users, total, err := s.userRepo.Search(query, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var userResponses []*domain.UserResponse
	for _, user := range users {
		userResponses = append(userResponses, user.ToResponse())
	}

	return userResponses, total, nil
}
