// Package service music_tag_service.go - 音乐标签服务层
// 该文件包含音乐标签的业务逻辑，负责处理标签的创建、更新、搜索和匹配等操作
package service

import (
	"context"
	"strings"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/repository"
)

// MusicTagService 音乐标签服务接口
// 负责处理音乐标签相关的业务逻辑，包括创建、更新、搜索、匹配等操作
type MusicTagService interface {
	Create(ctx context.Context, req *domain.CreateMusicTagRequest) (*domain.MusicTag, error)
	GetByID(ctx context.Context, id uint) (*domain.MusicTagResponse, error)
	GetByMusicBrainzID(ctx context.Context, mbid string) (*domain.MusicTagResponse, error)
	Update(ctx context.Context, id uint, req *domain.UpdateMusicTagRequest) (*domain.MusicTag, error)
	Delete(ctx context.Context, id uint) error
	Search(ctx context.Context, params *domain.TagSearchParams) ([]*domain.MusicTagResponse, int64, error)
	FindOrCreate(ctx context.Context, tag *domain.MusicTag) (*domain.MusicTag, error)
	IncrementUseCount(ctx context.Context, id uint) error
	CountAll(ctx context.Context) (int64, error)
	MatchTags(ctx context.Context, req *domain.CreateMusicTagRequest) (*domain.MusicTag, bool, error)
}

// musicTagService 音乐标签服务实现
type musicTagService struct {
	repo repository.MusicTagRepository
}

// NewMusicTagService 创建新的音乐标签服务实例
func NewMusicTagService(repo repository.MusicTagRepository) MusicTagService {
	return &musicTagService{repo: repo}
}

// Create 创建新的音乐标签
func (s *musicTagService) Create(ctx context.Context, req *domain.CreateMusicTagRequest) (*domain.MusicTag, error) {
	// 将请求转换为领域模型
	tag := s.convertCreateRequestToTag(req)

	// 生成搜索向量用于模糊匹配
	tag.SearchVector = s.generateSearchVector(tag)

	// 检查标签是否已存在
	existing, err := s.FindOrCreate(ctx, tag)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	// 创建新标签
	if err := s.repo.Create(ctx, tag); err != nil {
		return nil, err
	}

	return tag, nil
}

// convertCreateRequestToTag 将创建请求转换为音乐标签领域模型
func (s *musicTagService) convertCreateRequestToTag(req *domain.CreateMusicTagRequest) *domain.MusicTag {
	tag := &domain.MusicTag{
		Artist:              normalizeString(req.Artist),
		Title:               normalizeString(req.Title),
		Album:               normalizeString(req.Album),
		AlbumArtist:         normalizeString(req.AlbumArtist),
		Genre:               normalizeString(req.Genre),
		Comment:             normalizeString(req.Comment),
		MusicBrainzID:       strings.TrimSpace(req.MusicBrainzID),
		MusicBrainzArtistID: strings.TrimSpace(req.MusicBrainzArtistID),
	}

	// 处理可选字段
	if req.TrackNumber != nil {
		tag.TrackNumber = *req.TrackNumber
	}
	if req.DiscNumber != nil {
		tag.DiscNumber = *req.DiscNumber
	}
	if req.Year != nil {
		tag.Year = *req.Year
	}
	if req.Duration != nil {
		tag.Duration = *req.Duration
	}

	return tag
}

// GetByID 根据ID获取音乐标签
// 按ID查询音乐标签，返回标签响应实体和错误信息
func (s *musicTagService) GetByID(ctx context.Context, id uint) (*domain.MusicTagResponse, error) {
	tag, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return tag.ToResponse(), nil
}

// GetByMusicBrainzID 根据MusicBrainz ID获取音乐标签
func (s *musicTagService) GetByMusicBrainzID(ctx context.Context, mbid string) (*domain.MusicTagResponse, error) {
	tag, err := s.repo.GetByMusicBrainzID(ctx, mbid)
	if err != nil {
		return nil, err
	}
	return tag.ToResponse(), nil
}

// Update 更新现有的音乐标签
// 按ID查询音乐标签，更新标签字段，重新生成搜索向量，最后保存更新后的标签
func (s *musicTagService) Update(ctx context.Context, id uint, req *domain.UpdateMusicTagRequest) (*domain.MusicTag, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 更新标签字段
	s.updateTagFromRequest(existing, req)

	// 重新生成搜索向量
	existing.SearchVector = s.generateSearchVector(existing)

	if err := s.repo.Update(ctx, id, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// updateTagFromRequest 根据更新请求更新标签字段
func (s *musicTagService) updateTagFromRequest(tag *domain.MusicTag, req *domain.UpdateMusicTagRequest) {
	if req.Artist != nil {
		tag.Artist = normalizeString(*req.Artist)
	}
	if req.Title != nil {
		tag.Title = normalizeString(*req.Title)
	}
	if req.Album != nil {
		tag.Album = normalizeString(*req.Album)
	}
	if req.AlbumArtist != nil {
		tag.AlbumArtist = normalizeString(*req.AlbumArtist)
	}
	if req.Genre != nil {
		tag.Genre = normalizeString(*req.Genre)
	}
	if req.Comment != nil {
		tag.Comment = normalizeString(*req.Comment)
	}
	if req.MusicBrainzID != nil {
		tag.MusicBrainzID = strings.TrimSpace(*req.MusicBrainzID)
	}
	if req.MusicBrainzArtistID != nil {
		tag.MusicBrainzArtistID = strings.TrimSpace(*req.MusicBrainzArtistID)
	}
	if req.TrackNumber != nil {
		tag.TrackNumber = *req.TrackNumber
	}
	if req.DiscNumber != nil {
		tag.DiscNumber = *req.DiscNumber
	}
	if req.Year != nil {
		tag.Year = *req.Year
	}
	if req.Duration != nil {
		tag.Duration = *req.Duration
	}
}

// Delete 根据ID删除音乐标签
// 删除指定ID的音乐标签，返回错误信息
func (s *musicTagService) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

// Search 根据搜索参数查找音乐标签
// 根据传入的搜索参数查询音乐标签，返回标签响应列表、总数和错误信息
func (s *musicTagService) Search(ctx context.Context, params *domain.TagSearchParams) ([]*domain.MusicTagResponse, int64, error) {
	tags, total, err := s.repo.Search(ctx, params)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]*domain.MusicTagResponse, len(tags))
	for i, tag := range tags {
		responses[i] = tag.ToResponse()
	}

	return responses, total, nil
}

// FindOrCreate 查找现有标签或创建新标签
// 按照以下顺序尝试匹配：精确匹配 -> MusicBrainz ID匹配 -> MusicBrainz艺术家ID匹配 -> 模糊匹配
func (s *musicTagService) FindOrCreate(ctx context.Context, tag *domain.MusicTag) (*domain.MusicTag, error) {
	// 1. 尝试精确匹配（艺术家和标题）
	exactMatch, err := s.repo.GetByArtistAndTitle(ctx, tag.Artist, tag.Title)
	if err != nil {
		return nil, err
	}
	if exactMatch != nil {
		return exactMatch, nil
	}

	// 2. 尝试MusicBrainz ID匹配
	if tag.MusicBrainzID != "" {
		if mbMatch, err := s.repo.GetByMusicBrainzID(ctx, tag.MusicBrainzID); err == nil && mbMatch != nil {
			return mbMatch, nil
		}
	}

	// 3. 尝试MusicBrainz艺术家ID匹配
	if tag.MusicBrainzArtistID != "" {
		if artistMatches, err := s.repo.GetByMusicBrainzArtistID(ctx, tag.MusicBrainzArtistID); err == nil && len(artistMatches) > 0 {
			// 返回该艺术家使用次数最多的标签
			bestMatch := artistMatches[0]
			for _, match := range artistMatches {
				if match.UseCount > bestMatch.UseCount {
					bestMatch = match
				}
			}
			return bestMatch, nil
		}
	}

	// 4. 尝试模糊匹配
	fuzzyMatch, _, err := s.repo.FuzzySearch(ctx, tag)
	if err != nil {
		return nil, err
	}
	if fuzzyMatch != nil {
		return fuzzyMatch, nil
	}

	// 没有找到匹配的标签
	return nil, nil
}

// IncrementUseCount 增加标签的使用计数
func (s *musicTagService) IncrementUseCount(ctx context.Context, id uint) error {
	return s.repo.IncrementUseCount(ctx, id)
}

// CountAll 统计所有音乐标签数量
func (s *musicTagService) CountAll(ctx context.Context) (int64, error) {
	return s.repo.CountAll(ctx)
}

// MatchTags 尝试匹配传入的标签数据与现有标签
// 根据艺术家和标题进行精确匹配，若未找到则尝试模糊匹配（相似度阈值0.7）
func (s *musicTagService) MatchTags(ctx context.Context, req *domain.CreateMusicTagRequest) (*domain.MusicTag, bool, error) {
	// 将请求转换为标签领域模型
	tag := s.convertCreateRequestToTag(req)

	// 1. 尝试精确匹配
	exactMatch, err := s.repo.GetByArtistAndTitle(ctx, tag.Artist, tag.Title)
	if err != nil {
		return nil, false, err
	}
	if exactMatch != nil {
		return exactMatch, true, nil
	}

	// 2. 尝试模糊匹配（相似度阈值0.7）
	fuzzyMatch, score, err := s.repo.FuzzySearch(ctx, tag)
	if err != nil {
		return nil, false, err
	}
	if fuzzyMatch != nil && score >= 0.7 {
		return fuzzyMatch, true, nil
	}

	// 没有找到匹配的标签
	return nil, false, nil
}

// generateSearchVector 为模糊匹配创建搜索向量
// 将标签的关键字段（艺术家、标题、专辑等）组合成一个搜索字符串
func (s *musicTagService) generateSearchVector(tag *domain.MusicTag) string {
	var parts []string

	if tag.Artist != "" {
		parts = append(parts, tag.Artist)
	}
	if tag.Title != "" {
		parts = append(parts, tag.Title)
	}
	if tag.Album != "" {
		parts = append(parts, tag.Album)
	}
	if tag.AlbumArtist != "" {
		parts = append(parts, tag.AlbumArtist)
	}
	if tag.Genre != "" {
		parts = append(parts, tag.Genre)
	}

	return strings.Join(parts, " ")
}

// Helper functions

// normalizeString 规范化字符串
// 去除空格、转换为小写、替换特殊字符，用于搜索和匹配
func normalizeString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "&", "and")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "#", "")
	return s
}
