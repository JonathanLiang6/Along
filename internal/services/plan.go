package services

import (
	"database/sql"
	"fmt"
	"time"

	"ai-companion/internal/models"
)

// PlanService 计划服务（学习/项目/习惯/生活）
type PlanService struct {
	db *sql.DB
}

// NewPlanService 创建计划服务
func NewPlanService(db *sql.DB) *PlanService {
	return &PlanService{db: db}
}

// ==================== Goal 相关 ====================

func scanGoalRow(rows *sql.Row) (*models.Goal, error) {
	var g models.Goal
	var desc, targetDate, focus, nextStep, compNote, mood sql.NullString
	if err := rows.Scan(&g.ID, &g.Title, &desc, &g.Type, &g.Status,
		&g.StartDate, &targetDate, &focus, &nextStep, &compNote,
		&g.Progress, &mood, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return nil, err
	}
	g.Description = desc.String
	g.TargetDate = targetDate.String
	g.CurrentFocus = focus.String
	g.NextStep = nextStep.String
	g.CompanionNote = compNote.String
	g.Mood = mood.String
	return &g, nil
}

func scanGoalRows(rows *sql.Rows) (models.Goal, error) {
	var g models.Goal
	var desc, targetDate, focus, nextStep, compNote, mood sql.NullString
	if err := rows.Scan(&g.ID, &g.Title, &desc, &g.Type, &g.Status,
		&g.StartDate, &targetDate, &focus, &nextStep, &compNote,
		&g.Progress, &mood, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return g, err
	}
	g.Description = desc.String
	g.TargetDate = targetDate.String
	g.CurrentFocus = focus.String
	g.NextStep = nextStep.String
	g.CompanionNote = compNote.String
	g.Mood = mood.String
	return g, nil
}

// GetAllGoals 获取所有计划
func (s *PlanService) GetAllGoals() ([]models.Goal, error) {
	rows, err := s.db.Query(`SELECT id, title, description, type, status, 
		start_date, target_date, current_focus, next_step, companion_note,
		progress, mood, created_at, updated_at 
		FROM goals ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var goals []models.Goal
	for rows.Next() {
		g, err := scanGoalRows(rows)
		if err != nil {
			continue
		}
		goals = append(goals, g)
	}
	return goals, nil
}

// GetGoalsByType 按类型获取
func (s *PlanService) GetGoalsByType(goalType string) ([]models.Goal, error) {
	rows, err := s.db.Query(`SELECT id, title, description, type, status, 
		start_date, target_date, current_focus, next_step, companion_note,
		progress, mood, created_at, updated_at 
		FROM goals WHERE type = ? ORDER BY updated_at DESC`, goalType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var goals []models.Goal
	for rows.Next() {
		g, err := scanGoalRows(rows)
		if err != nil {
			continue
		}
		goals = append(goals, g)
	}
	return goals, nil
}

// GetGoalByID 获取单个计划
func (s *PlanService) GetGoalByID(id int) (*models.Goal, error) {
	row := s.db.QueryRow(`SELECT id, title, description, type, status, 
		start_date, target_date, current_focus, next_step, companion_note,
		progress, mood, created_at, updated_at 
		FROM goals WHERE id = ?`, id)
	return scanGoalRow(row)
}

// CreateGoal 创建计划
func (s *PlanService) CreateGoal(title, description, goalType string) (*models.Goal, error) {
	if title == "" {
		return nil, fmt.Errorf("计划名称不能为空")
	}
	if goalType == "" {
		goalType = "project"
	}

	today := time.Now().Format("2006-01-02")
	companionNote := s.defaultCompanionNote(goalType)

	result, err := s.db.Exec(
		`INSERT INTO goals (title, description, type, status, start_date, 
			companion_note, progress, created_at, updated_at)
		 VALUES (?, ?, ?, 'active', ?, ?, 0, datetime('now'), datetime('now'))`,
		title, description, goalType, today, companionNote,
	)
	if err != nil {
		return nil, err
	}

	lastID, _ := result.LastInsertId()
	now := time.Now().Format(time.RFC3339)
	return &models.Goal{
		ID:            int(lastID),
		Title:         title,
		Description:   description,
		Type:          goalType,
		Status:        "active",
		StartDate:     today,
		CompanionNote: companionNote,
		Progress:      0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// defaultCompanionNote Along 的开场白
func (s *PlanService) defaultCompanionNote(goalType string) string {
	switch goalType {
	case "learning":
		return "好呀，我们一起学。不用急，我们一步步来。"
	case "project":
		return "有想法就好，我陪着你做。先从哪里开始呢？"
	case "habit":
		return "习惯养成慢慢来，能坚持就是胜利。我帮你记着。"
	case "life":
		return "想做的事就去试试吧，我陪你。"
	default:
		return "好，我们一起来做这件事。"
	}
}

// UpdateGoal 更新计划基本信息
func (s *PlanService) UpdateGoal(id int, title, description, status, currentFocus, nextStep, mood string, progress int) error {
	if title == "" {
		return fmt.Errorf("计划名称不能为空")
	}
	_, err := s.db.Exec(
		`UPDATE goals SET title = ?, description = ?, status = ?, 
		 current_focus = ?, next_step = ?, mood = ?, progress = ?,
		 updated_at = datetime('now') WHERE id = ?`,
		title, description, status, currentFocus, nextStep, mood, progress, id,
	)
	return err
}

// UpdateCompanionNote 更新 Along 的话
func (s *PlanService) UpdateCompanionNote(id int, note string) error {
	_, err := s.db.Exec(
		`UPDATE goals SET companion_note = ?, updated_at = datetime('now') WHERE id = ?`,
		note, id,
	)
	return err
}

// DeleteGoal 删除计划（级联删除里程碑和记录）
func (s *PlanService) DeleteGoal(id int) error {
	_, err := s.db.Exec("DELETE FROM goals WHERE id = ?", id)
	return err
}

// ==================== Milestone 相关 ====================

func scanMilestoneRows(rows *sql.Rows) (models.Milestone, error) {
	var m models.Milestone
	var desc, compAt, compComment sql.NullString
	if err := rows.Scan(&m.ID, &m.GoalID, &m.Title, &desc,
		&m.Status, &compAt, &compComment, &m.OrderIndex); err != nil {
		return m, err
	}
	m.Description = desc.String
	m.CompletedAt = compAt.String
	m.CompanionComment = compComment.String
	return m, nil
}

// GetMilestones 获取计划的所有里程碑
func (s *PlanService) GetMilestones(goalID int) ([]models.Milestone, error) {
	rows, err := s.db.Query(`SELECT id, goal_id, title, description, status, 
		completed_at, companion_comment, order_index 
		FROM milestones WHERE goal_id = ? ORDER BY order_index ASC, id ASC`, goalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var milestones []models.Milestone
	for rows.Next() {
		m, err := scanMilestoneRows(rows)
		if err != nil {
			continue
		}
		milestones = append(milestones, m)
	}
	return milestones, nil
}

// AddMilestone 添加里程碑
func (s *PlanService) AddMilestone(goalID int, title, description string) (*models.Milestone, error) {
	if title == "" {
		return nil, fmt.Errorf("里程碑名称不能为空")
	}
	// 计算 order_index
	var maxIdx int
	s.db.QueryRow("SELECT COALESCE(MAX(order_index), -1) FROM milestones WHERE goal_id = ?", goalID).Scan(&maxIdx)
	orderIdx := maxIdx + 1

	result, err := s.db.Exec(
		`INSERT INTO milestones (goal_id, title, description, status, order_index)
		 VALUES (?, ?, ?, 'pending', ?)`,
		goalID, title, description, orderIdx,
	)
	if err != nil {
		return nil, err
	}
	lastID, _ := result.LastInsertId()
	return &models.Milestone{
		ID:          int(lastID),
		GoalID:      goalID,
		Title:       title,
		Description: description,
		Status:      "pending",
		OrderIndex:  orderIdx,
	}, nil
}

// UpdateMilestone 更新里程碑
func (s *PlanService) UpdateMilestone(id int, title, description, status string) error {
	if title == "" {
		return fmt.Errorf("里程碑名称不能为空")
	}
	var compAt interface{}
	if status == "completed" {
		compAt = time.Now().Format("2006-01-02")
	} else {
		compAt = nil
	}
	_, err := s.db.Exec(
		`UPDATE milestones SET title = ?, description = ?, status = ?, 
		 completed_at = ? WHERE id = ?`,
		title, description, status, compAt, id,
	)
	return err
}

// CompleteMilestone 完成里程碑
func (s *PlanService) CompleteMilestone(id int, companionComment string) error {
	today := time.Now().Format("2006-01-02")
	_, err := s.db.Exec(
		`UPDATE milestones SET status = 'completed', completed_at = ?, 
		 companion_comment = ? WHERE id = ?`,
		today, companionComment, id,
	)
	if err != nil {
		return err
	}
	// 更新计划进度
	return s.recalcProgressFromMilestones(id)
}

func (s *PlanService) recalcProgressFromMilestones(milestoneID int) error {
	var goalID int
	err := s.db.QueryRow("SELECT goal_id FROM milestones WHERE id = ?", milestoneID).Scan(&goalID)
	if err != nil {
		return err
	}

	var total, completed int
	s.db.QueryRow("SELECT COUNT(*) FROM milestones WHERE goal_id = ?", goalID).Scan(&total)
	s.db.QueryRow("SELECT COUNT(*) FROM milestones WHERE goal_id = ? AND status = 'completed'", goalID).Scan(&completed)

	if total == 0 {
		return nil
	}
	progress := int(float64(completed) / float64(total) * 100)
	_, err = s.db.Exec("UPDATE goals SET progress = ?, updated_at = datetime('now') WHERE id = ?", progress, goalID)
	return err
}

// DeleteMilestone 删除里程碑
func (s *PlanService) DeleteMilestone(id int) error {
	_, err := s.db.Exec("DELETE FROM milestones WHERE id = ?", id)
	return err
}

// ==================== CheckIn 相关 ====================

func scanCheckInRows(rows *sql.Rows) (models.CheckIn, error) {
	var c models.CheckIn
	var mood, compResp sql.NullString
	if err := rows.Scan(&c.ID, &c.GoalID, &c.Date, &c.Content,
		&mood, &compResp, &c.CreatedAt); err != nil {
		return c, err
	}
	c.Mood = mood.String
	c.CompanionResponse = compResp.String
	return c, nil
}

// GetCheckIns 获取计划的所有记录
func (s *PlanService) GetCheckIns(goalID int) ([]models.CheckIn, error) {
	rows, err := s.db.Query(`SELECT id, goal_id, date, content, mood, 
		companion_response, created_at 
		FROM check_ins WHERE goal_id = ? ORDER BY date DESC, id DESC`, goalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checkIns []models.CheckIn
	for rows.Next() {
		c, err := scanCheckInRows(rows)
		if err != nil {
			continue
		}
		checkIns = append(checkIns, c)
	}
	return checkIns, nil
}

// AddCheckIn 添加记录
func (s *PlanService) AddCheckIn(goalID int, content, mood, companionResponse string) (*models.CheckIn, error) {
	if content == "" {
		return nil, fmt.Errorf("记录内容不能为空")
	}
	today := time.Now().Format("2006-01-02")

	result, err := s.db.Exec(
		`INSERT INTO check_ins (goal_id, date, content, mood, companion_response)
		 VALUES (?, ?, ?, ?, ?)`,
		goalID, today, content, mood, companionResponse,
	)
	if err != nil {
		return nil, err
	}
	lastID, _ := result.LastInsertId()
	return &models.CheckIn{
		ID:                int(lastID),
		GoalID:            goalID,
		Date:              today,
		Content:           content,
		Mood:              mood,
		CompanionResponse: companionResponse,
	}, nil
}

// DeleteCheckIn 删除记录
func (s *PlanService) DeleteCheckIn(id int) error {
	_, err := s.db.Exec("DELETE FROM check_ins WHERE id = ?", id)
	return err
}

// SearchGoals 搜索计划
func (s *PlanService) SearchGoals(keyword string) ([]models.Goal, error) {
	if keyword == "" {
		return s.GetAllGoals()
	}
	rows, err := s.db.Query(
		`SELECT id, title, description, type, status, 
		 start_date, target_date, current_focus, next_step, companion_note,
		 progress, mood, created_at, updated_at 
		 FROM goals WHERE title LIKE ? OR description LIKE ? OR current_focus LIKE ?
		 ORDER BY updated_at DESC`,
		"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var goals []models.Goal
	for rows.Next() {
		g, err := scanGoalRows(rows)
		if err != nil {
			continue
		}
		goals = append(goals, g)
	}
	return goals, nil
}
