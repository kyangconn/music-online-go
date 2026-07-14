// Package service music_service_admin.go - 音乐管理服务
// 包含管理员操作：删除音乐等功能
package service

import "context"

func (s *musicService) AdminDelete(ctx context.Context, id uint) error {
	// 检查音乐是否存在
	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// 管理员可以直接删除，不需要检查所有权
	return s.deleteMusic(ctx, id)
}
