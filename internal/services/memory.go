package services

import (
	"database/sql"
	"strings"

	"ai-companion/internal/models"
)

// MemoryService 记忆服务
type MemoryService struct {
	db *sql.DB
}

// NewMemoryService 创建记忆服务
func NewMemoryService(db *sql.DB) *MemoryService {
	return &MemoryService{db: db}
}

// scanMemoryRows 安全 scan 记忆行（处理 NULL 字段）
func scanMemoryRows(rows *sql.Rows) (models.Memory, error) {
	var m models.Memory
	var source, tags sql.NullString
	var confidence sql.NullFloat64
	if err := rows.Scan(&m.ID, &m.Type, &m.Content, &source, &confidence, &m.CreatedAt, &m.UpdatedAt, &tags); err != nil {
		return m, err
	}
	m.Source = source.String
	m.Tags = tags.String
	if confidence.Valid {
		m.Confidence = confidence.Float64
	}
	return m, nil
}

// scanMemoryRow 安全 scan 记忆行（用于 QueryRow）
func scanMemoryRow(row *sql.Row) (models.Memory, error) {
	var m models.Memory
	var source, tags sql.NullString
	var confidence sql.NullFloat64
	if err := row.Scan(&m.ID, &m.Type, &m.Content, &source, &confidence, &m.CreatedAt, &m.UpdatedAt, &tags); err != nil {
		return m, err
	}
	m.Source = source.String
	m.Tags = tags.String
	if confidence.Valid {
		m.Confidence = confidence.Float64
	}
	return m, nil
}

// GetMemories 获取记忆列表
func (s *MemoryService) GetMemories(memoryType string) ([]models.Memory, error) {
	var query string
	var args []interface{}

	if memoryType == "" {
		query = "SELECT id, type, content, source, confidence, created_at, updated_at, tags FROM memories ORDER BY updated_at DESC"
	} else {
		query = "SELECT id, type, content, source, confidence, created_at, updated_at, tags FROM memories WHERE type = ? ORDER BY updated_at DESC"
		args = append(args, memoryType)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []models.Memory
	for rows.Next() {
		m, err := scanMemoryRows(rows)
		if err != nil {
			continue
		}
		memories = append(memories, m)
	}
	return memories, nil
}

// SaveMemory 添加或合并记忆
func (s *MemoryService) SaveMemory(content, memoryType string, confidence float64, source string) (*models.Memory, error) {
	// 1. 简单去重：查找同类型的相似记忆
	rows, err := s.db.Query(
		"SELECT id, type, content, source, confidence, created_at, updated_at, tags FROM memories WHERE type = ? AND content LIKE ? ORDER BY updated_at DESC LIMIT 5",
		memoryType, "%"+truncateForLike(content)+"%",
	)
	if err == nil {
		for rows.Next() {
			m, scanErr := scanMemoryRows(rows)
			if scanErr != nil {
				continue
			}
			if isSimilar(m.Content, content) {
				rows.Close()
				newConf := m.Confidence + 0.1
				if newConf > 1.0 {
					newConf = 1.0
				}
				_, _ = s.db.Exec(
					"UPDATE memories SET updated_at = datetime('now'), confidence = ? WHERE id = ?",
					newConf, m.ID,
				)
				m.Confidence = newConf
				return &m, nil
			}
		}
		rows.Close()
	}

	// 2. 插入新记忆
	result, err := s.db.Exec(
		"INSERT INTO memories (type, content, source, confidence, created_at, updated_at) VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))",
		memoryType, content, source, confidence,
	)
	if err != nil {
		return nil, err
	}
	lastID, _ := result.LastInsertId()
	return &models.Memory{
		ID:         int(lastID),
		Type:       memoryType,
		Content:    content,
		Source:     source,
		Confidence: confidence,
	}, nil
}

// AddMemory 添加记忆（兼容老接口）
func (s *MemoryService) AddMemory(memoryType, content, source string, confidence float64) error {
	_, err := s.SaveMemory(content, memoryType, confidence, source)
	return err
}

// UpdateMemory 更新记忆
func (s *MemoryService) UpdateMemory(id int, content string) error {
	_, err := s.db.Exec(
		"UPDATE memories SET content = ?, updated_at = datetime('now'), confidence = 1.0 WHERE id = ?",
		content, id,
	)
	return err
}

// DeleteMemory 删除记忆
func (s *MemoryService) DeleteMemory(id int) error {
	_, err := s.db.Exec("DELETE FROM memories WHERE id = ?", id)
	return err
}

// SearchMemories 搜索记忆（关键词匹配）
func (s *MemoryService) SearchMemories(keyword string) ([]models.Memory, error) {
	if strings.TrimSpace(keyword) == "" {
		return s.GetMemories("")
	}
	keywords := strings.Fields(keyword)
	if len(keywords) == 0 {
		return s.GetMemories("")
	}

	conditions := make([]string, 0, len(keywords))
	args := make([]interface{}, 0, len(keywords))
	for _, kw := range keywords {
		conditions = append(conditions, "content LIKE ?")
		args = append(args, "%"+kw+"%")
	}
	q := "SELECT id, type, content, source, confidence, created_at, updated_at, tags FROM memories WHERE " +
		strings.Join(conditions, " AND ") +
		" ORDER BY confidence DESC LIMIT 50"

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []models.Memory
	for rows.Next() {
		m, err := scanMemoryRows(rows)
		if err != nil {
			continue
		}
		memories = append(memories, m)
	}
	return memories, nil
}

// GetKeyMemories 取得置信度最高的关键记忆
func (s *MemoryService) GetKeyMemories(limit int) ([]models.Memory, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := s.db.Query(
		"SELECT id, type, content, source, confidence, created_at, updated_at, tags FROM memories WHERE confidence >= 0.5 ORDER BY confidence DESC, updated_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []models.Memory
	for rows.Next() {
		m, err := scanMemoryRows(rows)
		if err != nil {
			continue
		}
		memories = append(memories, m)
	}
	return memories, nil
}

// GetCountByType 按类型统计记忆数量
func (s *MemoryService) GetCountByType() (map[string]int, error) {
	rows, err := s.db.Query("SELECT type, COUNT(*) FROM memories GROUP BY type")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{
		"L1": 0, "L2": 0, "L3": 0, "L4": 0, "L5": 0,
	}
	for rows.Next() {
		var t string
		var c int
		if err := rows.Scan(&t, &c); err == nil {
			counts[t] = c
		}
	}
	return counts, nil
}

// GetObservations 获取观察列表
func (s *MemoryService) GetObservations() ([]models.Observation, error) {
	rows, err := s.db.Query("SELECT id, content, type, displayed, created_at FROM observations WHERE displayed = 0 ORDER BY created_at DESC LIMIT 3")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var observations []models.Observation
	for rows.Next() {
		var o models.Observation
		var obsType sql.NullString
		if err := rows.Scan(&o.ID, &o.Content, &obsType, &o.Displayed, &o.CreatedAt); err != nil {
			continue
		}
		o.Type = obsType.String
		observations = append(observations, o)
	}
	return observations, nil
}

// MarkObservationDisplayed 标记观察已展示
func (s *MemoryService) MarkObservationDisplayed(id int) error {
	_, err := s.db.Exec("UPDATE observations SET displayed = 1 WHERE id = ?", id)
	return err
}

// AddObservation 添加观察
func (s *MemoryService) AddObservation(content, observationType string) error {
	_, err := s.db.Exec(
		"INSERT INTO observations (content, type, displayed, created_at) VALUES (?, ?, 0, datetime('now'))",
		content, observationType,
	)
	return err
}

// SaveReflection 保存复盘报告
func (s *MemoryService) SaveReflection(r *models.Reflection) error {
	_, err := s.db.Exec(
		"INSERT INTO reflections (period_start, period_end, growth_analysis, relationship_analysis, project_review, observations, created_at) VALUES (?, ?, ?, ?, ?, ?, datetime('now'))",
		r.PeriodStart, r.PeriodEnd, r.GrowthAnalysis, r.RelationshipAnalysis, r.ProjectReview, r.Observations,
	)
	return err
}

// GetUndiscussedMemories 获取重要但长时间未讨论的记忆
func (s *MemoryService) GetUndiscussedMemories(days int) ([]models.Memory, error) {
	query := `
		SELECT id, type, content, source, confidence, created_at, updated_at, tags
		FROM memories
		WHERE confidence >= 0.7
		AND datetime(updated_at) < datetime('now', '-' || ? || ' days')
		ORDER BY confidence DESC, updated_at ASC
		LIMIT 5
	`
	rows, err := s.db.Query(query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []models.Memory
	for rows.Next() {
		m, err := scanMemoryRows(rows)
		if err != nil {
			continue
		}
		memories = append(memories, m)
	}
	return memories, nil
}

// MarkMemoryDiscussed 标记记忆已被讨论
func (s *MemoryService) MarkMemoryDiscussed(id int) error {
	_, err := s.db.Exec("UPDATE memories SET updated_at = datetime('now') WHERE id = ?", id)
	return err
}

// truncateForLike 截取前 16 字符用于 LIKE
func truncateForLike(s string) string {
	runes := []rune(s)
	if len(runes) > 16 {
		return string(runes[:16])
	}
	return s
}

// isSimilar 简易相似度判断
func isSimilar(a, b string) bool {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 || len(br) == 0 {
		return false
	}
	shorter, longer := ar, br
	if len(br) < len(ar) {
		shorter, longer = br, ar
	}
	if len(shorter) < 4 {
		return false
	}
	for i := 0; i+len(shorter) <= len(longer); i++ {
		match := true
		for j := 0; j < len(shorter); j++ {
			if longer[i+j] != shorter[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
