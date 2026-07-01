// Package repository music_tag_repository.go - 音乐标签仓库层
// 音乐标签的增删改查、模糊匹配、相似度计算
package repository

import (
	"context"
	"strings"

	"github.com/kyangconn/music-online-go/internal/domain"
	"gorm.io/gorm"
)

type musicTagRepository struct {
	db *gorm.DB
}

// MusicTagRepository 音乐标签仓库接口
type MusicTagRepository interface {
	// Create 创建新的音乐标签
	Create(ctx context.Context, tag *domain.MusicTag) error
	// GetByID 根据ID获取音乐标签
	GetByID(ctx context.Context, id uint) (*domain.MusicTag, error)
	// GetByMusicBrainzID 根据MusicBrainz ID获取音乐标签
	GetByMusicBrainzID(ctx context.Context, musicBrainzID string) (*domain.MusicTag, error)
	// GetByMusicBrainzArtistID 根据MusicBrainz艺术家ID获取音乐标签列表
	GetByMusicBrainzArtistID(ctx context.Context, artistID string) ([]*domain.MusicTag, error)
	// GetByArtistAndTitle 根据艺术家和标题获取音乐标签
	GetByArtistAndTitle(ctx context.Context, artist, title string) (*domain.MusicTag, error)
	// Update 更新音乐标签
	Update(ctx context.Context, id uint, tag *domain.MusicTag) error
	// Delete 删除音乐标签
	Delete(ctx context.Context, id uint) error
	// Search 根据搜索参数查询音乐标签
	Search(ctx context.Context, params *domain.TagSearchParams) ([]*domain.MusicTag, int64, error)
	// FindOrCreate 查找或创建音乐标签
	FindOrCreate(ctx context.Context, tag *domain.MusicTag) (*domain.MusicTag, error)
	// IncrementUseCount 增加音乐标签的使用次数
	IncrementUseCount(ctx context.Context, id uint) error
	// FuzzySearch 对音乐标签进行模糊匹配
	FuzzySearch(ctx context.Context, tag *domain.MusicTag) (*domain.MusicTag, float64, error)
	// CountAll 统计所有音乐标签数量
	CountAll(ctx context.Context) (int64, error)
}

// NewMusicTagRepository 创建音乐标签仓库实例
func NewMusicTagRepository(db *gorm.DB) MusicTagRepository {
	return &musicTagRepository{db: db}
}

// Create 创建新的音乐标签
func (r *musicTagRepository) Create(ctx context.Context, tag *domain.MusicTag) error {
	return r.db.WithContext(ctx).Create(tag).Error
}

// CountAll 统计所有音乐标签数量
func (r *musicTagRepository) CountAll(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.MusicTag{}).Count(&count).Error
	return count, err
}

// GetByID 根据ID获取音乐标签
// 按ID查询音乐标签，返回标签实体和错误信息
func (r *musicTagRepository) GetByID(ctx context.Context, id uint) (*domain.MusicTag, error) {
	return r.getSingleTag(ctx, "id = ?", id)
}

// GetByMusicBrainzID 根据MusicBrainz ID获取音乐标签
// 按MusicBrainz ID查询音乐标签，返回标签实体和错误信息
func (r *musicTagRepository) GetByMusicBrainzID(ctx context.Context, musicBrainzID string) (*domain.MusicTag, error) {
	return r.getSingleTag(ctx, "music_brainz_id = ?", musicBrainzID)
}

// GetByMusicBrainzArtistID 根据MusicBrainz艺术家ID获取音乐标签列表
// 按MusicBrainz艺术家ID查询音乐标签列表，返回标签实体列表和错误信息
func (r *musicTagRepository) GetByMusicBrainzArtistID(ctx context.Context, artistID string) ([]*domain.MusicTag, error) {
	return r.getMultipleTags(ctx, "music_brainz_artist_id = ?", artistID)
}

// GetByArtistAndTitle 根据艺术家和标题获取音乐标签
// 按艺术家和标题查询音乐标签，返回标签实体和错误信息
func (r *musicTagRepository) GetByArtistAndTitle(ctx context.Context, artist, title string) (*domain.MusicTag, error) {
	return r.getSingleTag(ctx, "artist = ? AND title = ?", strings.ToLower(artist), strings.ToLower(title))
}

// getSingleTag 通用单条记录查询函数
func (r *musicTagRepository) getSingleTag(ctx context.Context, condition string, args ...interface{}) (*domain.MusicTag, error) {
	var tag domain.MusicTag
	err := r.db.WithContext(ctx).Where(condition, args...).First(&tag).Error
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

// getMultipleTags 通用多条记录查询函数
func (r *musicTagRepository) getMultipleTags(ctx context.Context, condition string, args ...interface{}) ([]*domain.MusicTag, error) {
	var tags []*domain.MusicTag
	err := r.db.WithContext(ctx).Where(condition, args...).Find(&tags).Error
	if err != nil {
		return nil, err
	}
	return tags, nil
}

// Update 更新音乐标签
func (r *musicTagRepository) Update(ctx context.Context, _ uint, tag *domain.MusicTag) error {
	return r.db.WithContext(ctx).Save(tag).Error
}

// Delete 删除音乐标签
func (r *musicTagRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.MusicTag{}, id).Error
}

// Search 根据搜索参数查询音乐标签
func (r *musicTagRepository) Search(ctx context.Context, params *domain.TagSearchParams) ([]*domain.MusicTag, int64, error) {
	var tags []*domain.MusicTag
	var total int64

	query := r.db.WithContext(ctx).Model(&domain.MusicTag{})

	// 构建搜索条件
	query = r.buildSearchConditions(query, params)

	// 获取总记录数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 应用分页
	limit := params.GetLimit()
	offset := params.GetOffset()
	query = query.Limit(limit).Offset(offset)

	// 排序：按使用次数降序，然后按创建时间降序
	query = query.Order("use_count DESC")
	query = query.Order("created_at DESC")

	if err := query.Find(&tags).Error; err != nil {
		return nil, 0, err
	}

	return tags, total, nil
}

// buildSearchConditions 构建搜索条件
func (r *musicTagRepository) buildSearchConditions(query *gorm.DB, params *domain.TagSearchParams) *gorm.DB {
	// 艺术家搜索条件
	if params.Artist != "" {
		query = query.Where("LOWER(artist) LIKE ?", "%"+strings.ToLower(params.Artist)+"%")
	}

	// 标题搜索条件
	if params.Title != "" {
		query = query.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(params.Title)+"%")
	}

	// 专辑搜索条件
	if params.Album != "" {
		query = query.Where("LOWER(album) LIKE ?", "%"+strings.ToLower(params.Album)+"%")
	}

	// 专辑艺术家搜索条件
	if params.AlbumArtist != "" {
		query = query.Where("LOWER(album_artist) LIKE ?", "%"+strings.ToLower(params.AlbumArtist)+"%")
	}

	// 流派搜索条件
	if params.Genre != "" {
		query = query.Where("LOWER(genre) LIKE ?", "%"+strings.ToLower(params.Genre)+"%")
	}

	// 年份搜索条件
	if params.Year != nil {
		query = query.Where("year = ?", *params.Year)
	}

	// 年份范围搜索条件
	if params.MinYear != nil {
		query = query.Where("year >= ?", *params.MinYear)
	}
	if params.MaxYear != nil {
		query = query.Where("year <= ?", *params.MaxYear)
	}

	// 时长范围搜索条件
	if params.MinDuration != nil {
		query = query.Where("duration >= ?", *params.MinDuration)
	}
	if params.MaxDuration != nil {
		query = query.Where("duration <= ?", *params.MaxDuration)
	}

	// MusicBrainz ID 搜索条件
	if params.MusicBrainzID != "" {
		query = query.Where("music_brainz_id = ?", params.MusicBrainzID)
	}

	return query
}

// FindOrCreate 查找或创建音乐标签
func (r *musicTagRepository) FindOrCreate(ctx context.Context, tag *domain.MusicTag) (*domain.MusicTag, error) {
	// 尝试精确匹配艺术家和标题
	exactMatch, err := r.GetByArtistAndTitle(ctx, tag.Artist, tag.Title)
	if err != nil {
		return nil, err
	}
	if exactMatch != nil {
		return exactMatch, nil
	}

	// 尝试模糊匹配
	fuzzyMatch, _, err := r.FuzzySearch(ctx, tag)
	if err != nil {
		return nil, err
	}
	if fuzzyMatch != nil {
		return fuzzyMatch, nil
	}

	return nil, nil
}

// IncrementUseCount 增加音乐标签的使用次数
func (r *musicTagRepository) IncrementUseCount(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&domain.MusicTag{}).Where("id = ?", id).UpdateColumn("use_count", gorm.Expr("use_count + 1")).Error
}

// FuzzySearch 对音乐标签进行模糊匹配
func (r *musicTagRepository) FuzzySearch(ctx context.Context, tag *domain.MusicTag) (*domain.MusicTag, float64, error) {
	// 构建查询条件
	conditions, params := r.buildFuzzySearchConditions(tag)
	if len(conditions) == 0 {
		return nil, 0, nil
	}

	// 执行查询
	candidates, err := r.executeFuzzySearchQuery(ctx, conditions, params)
	if err != nil {
		return nil, 0, err
	}
	if len(candidates) == 0 {
		return nil, 0, nil
	}

	// 计算相似度并找到最佳匹配
	return r.findBestMatch(tag, candidates)
}

// buildFuzzySearchConditions 构建模糊搜索条件
func (r *musicTagRepository) buildFuzzySearchConditions(tag *domain.MusicTag) ([]string, []interface{}) {
	var conditions []string
	var params []interface{}

	if tag.Artist != "" {
		conditions = append(conditions, "LOWER(artist) LIKE ?")
		params = append(params, "%"+strings.ToLower(tag.Artist)+"%")
	}
	if tag.Title != "" {
		conditions = append(conditions, "LOWER(title) LIKE ?")
		params = append(params, "%"+strings.ToLower(tag.Title)+"%")
	}
	if tag.Album != "" {
		conditions = append(conditions, "LOWER(album) LIKE ?")
		params = append(params, "%"+strings.ToLower(tag.Album)+"%")
	}
	if tag.Genre != "" {
		conditions = append(conditions, "LOWER(genre) LIKE ?")
		params = append(params, "%"+strings.ToLower(tag.Genre)+"%")
	}

	return conditions, params
}

// executeFuzzySearchQuery 执行模糊搜索查询
func (r *musicTagRepository) executeFuzzySearchQuery(ctx context.Context, conditions []string, params []interface{}) ([]*domain.MusicTag, error) {
	var candidates []*domain.MusicTag
	query := r.db.WithContext(ctx).Model(&domain.MusicTag{})

	query = query.Where(strings.Join(conditions, " AND "), params...)
	query = query.Order("use_count DESC").Limit(10)

	if err := query.Find(&candidates).Error; err != nil {
		return nil, err
	}

	return candidates, nil
}

// findBestMatch 在候选列表中查找最佳匹配
func (r *musicTagRepository) findBestMatch(tag *domain.MusicTag, candidates []*domain.MusicTag) (*domain.MusicTag, float64, error) {
	bestMatch := candidates[0]
	bestScore := calculateSimilarity(tag, candidates[0])

	for i := 1; i < len(candidates); i++ {
		score := calculateSimilarity(tag, candidates[i])
		if score > bestScore {
			bestScore = score
			bestMatch = candidates[i]
		}
	}

	return bestMatch, bestScore, nil
}

// calculateSimilarity 计算两个音乐标签之间的相似度分数
func calculateSimilarity(tag1, tag2 *domain.MusicTag) float64 {
	var score float64
	var totalFields float64

	// 计算艺术家相似度（权重：30%）
	score, totalFields = calculateFieldSimilarity(tag1.Artist, tag2.Artist, score, totalFields)

	// 计算标题相似度（权重：40%）
	score, totalFields = calculateFieldSimilarity(tag1.Title, tag2.Title, score, totalFields)

	// 计算专辑相似度（权重：15%）
	score, totalFields = calculateFieldSimilarity(tag1.Album, tag2.Album, score, totalFields)

	// 计算流派相似度（权重：10%）
	score, totalFields = calculateFieldSimilarity(tag1.Genre, tag2.Genre, score, totalFields)

	// 计算年份相似度（权重：5%）
	score, totalFields = calculateYearSimilarity(tag1.Year, tag2.Year, score, totalFields)

	if totalFields == 0 {
		return 0
	}

	return score / totalFields
}

// calculateFieldSimilarity 计算字符串字段的相似度
func calculateFieldSimilarity(field1, field2 string, currentScore, currentTotal float64) (float64, float64) {
	if field1 == "" || field2 == "" {
		return currentScore, currentTotal
	}

	currentTotal += 1
	if field1 == field2 {
		return currentScore + 1.0, currentTotal
	}

	if strings.Contains(field1, field2) || strings.Contains(field2, field1) {
		return currentScore + 0.5, currentTotal
	}

	return currentScore, currentTotal
}

// calculateYearSimilarity 计算年份字段的相似度
func calculateYearSimilarity(year1, year2 int, currentScore, currentTotal float64) (float64, float64) {
	if year1 == 0 || year2 == 0 {
		return currentScore, currentTotal
	}

	currentTotal += 1
	if year1 == year2 {
		return currentScore + 1.0, currentTotal
	}

	yearDiff := abs(year1 - year2)
	if yearDiff <= 1 {
		return currentScore + 0.8, currentTotal
	}
	if yearDiff <= 2 {
		return currentScore + 0.5, currentTotal
	}

	return currentScore, currentTotal
}

// abs 计算整数的绝对值
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
