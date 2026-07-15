package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ai-companion/internal/models"
)

type ConversationService struct {
	db       *sql.DB
	convsDir string
	writeMu  sync.Mutex
}

func NewConversationService(db *sql.DB) *ConversationService {
	return &ConversationService{db: db}
}

func (s *ConversationService) SetConversationsDir(dir string) {
	s.convsDir = dir
	if dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}
}

// CreateConversation 创建新对话
func (s *ConversationService) CreateConversation(title string) (*models.Conversation, error) {
	today := time.Now().Format("2006-01-02")
	if title == "" {
		title = "新对话"
	}

	result, err := s.db.Exec(
		"INSERT INTO conversations (date, title, created_at) VALUES (?, ?, datetime('now'))",
		today, title,
	)
	if err != nil {
		return nil, err
	}
	lastID, _ := result.LastInsertId()

	conv := &models.Conversation{
		ID:        int(lastID),
		Date:      today,
		Title:     title,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	return conv, nil
}

// ListConversations 获取所有对话列表（按更新时间倒序）
func (s *ConversationService) ListConversations() ([]models.Conversation, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.date, c.title, c.agent_route, c.created_at,
		       COALESCE(MAX(m.timestamp), c.created_at) as last_msg_time
		FROM conversations c
		LEFT JOIN messages m ON m.conversation_id = c.id
		GROUP BY c.id
		ORDER BY last_msg_time DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []models.Conversation
	for rows.Next() {
		var c models.Conversation
		var title, agentRoute sql.NullString
		var lastMsgTime sql.NullString
		if err := rows.Scan(&c.ID, &c.Date, &title, &agentRoute, &c.CreatedAt, &lastMsgTime); err != nil {
			continue
		}
		c.Title = title.String
		c.AgentRoute = agentRoute.String
		convs = append(convs, c)
	}
	return convs, nil
}

// GetConversation 获取单个对话
func (s *ConversationService) GetConversation(id int) (*models.Conversation, error) {
	var c models.Conversation
	var title, agentRoute sql.NullString
	err := s.db.QueryRow(
		"SELECT id, date, title, agent_route, created_at FROM conversations WHERE id = ?",
		id,
	).Scan(&c.ID, &c.Date, &title, &agentRoute, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.Title = title.String
	c.AgentRoute = agentRoute.String
	return &c, nil
}

// RenameConversation 重命名对话
func (s *ConversationService) RenameConversation(id int, title string) error {
	_, err := s.db.Exec("UPDATE conversations SET title = ? WHERE id = ?", title, id)
	return err
}

// DeleteConversation 删除对话及其所有消息
func (s *ConversationService) DeleteConversation(id int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM messages WHERE conversation_id = ?", id)
	if err != nil {
		return err
	}
	_, err = tx.Exec("DELETE FROM conversations WHERE id = ?", id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetMessagesByConversationID 获取指定对话的所有消息
func (s *ConversationService) GetMessagesByConversationID(conversationID int) ([]models.Message, error) {
	rows, err := s.db.Query(
		`SELECT id, conversation_id, role, content, emotion, timestamp
		 FROM messages
		 WHERE conversation_id = ?
		 ORDER BY timestamp`,
		conversationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		var emotion sql.NullString
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &emotion, &m.Timestamp); err != nil {
			continue
		}
		if emotion.Valid {
			m.Emotion = emotion.String
		}
		messages = append(messages, m)
	}
	return messages, nil
}

// SaveMessageToConversation 保存消息到指定对话
func (s *ConversationService) SaveMessageToConversation(conversationID int, role, content, emotion string) (int, error) {
	result, err := s.db.Exec(
		"INSERT INTO messages (conversation_id, role, content, emotion, timestamp) VALUES (?, ?, ?, ?, datetime('now'))",
		conversationID, role, content, emotion,
	)
	if err != nil {
		return 0, err
	}
	lastID, _ := result.LastInsertId()

	// 同步写入 JSON 文件
	s.appendToJSONFile(fmt.Sprintf("conv_%d", conversationID), conversationID, role, content, emotion)

	return int(lastID), nil
}

// GetOrCreateTodayConversation 获取或创建今日默认对话（兼容旧接口）
func (s *ConversationService) GetOrCreateTodayConversation() (int, error) {
	today := time.Now().Format("2006-01-02")

	var convID int
	err := s.db.QueryRow("SELECT id FROM conversations WHERE date = ? ORDER BY id ASC LIMIT 1", today).Scan(&convID)
	if err == sql.ErrNoRows {
		title := fmt.Sprintf("%s 的对话", today)
		result, err := s.db.Exec(
			"INSERT INTO conversations (date, title, created_at) VALUES (?, ?, datetime('now'))",
			today, title,
		)
		if err != nil {
			return 0, err
		}
		lastID, _ := result.LastInsertId()
		return int(lastID), nil
	}
	if err != nil {
		return 0, err
	}
	return convID, nil
}

// SaveMessage 保存消息（兼容旧接口：使用今日默认对话）
func (s *ConversationService) SaveMessage(role, content, emotion string) error {
	convID, err := s.GetOrCreateTodayConversation()
	if err != nil {
		return err
	}
	_, err = s.SaveMessageToConversation(convID, role, content, emotion)
	return err
}

// summarizeTitle 根据首条消息生成简短标题
func summarizeTitle(content string) string {
	runes := []rune(content)
	if len(runes) > 20 {
		return string(runes[:20]) + "..."
	}
	if content == "" {
		return "对话"
	}
	return content
}

// appendToJSONFile 追加消息到 JSON 文件
func (s *ConversationService) appendToJSONFile(key string, convID int, role, content, emotion string) {
	if s.convsDir == "" {
		return
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	filePath := filepath.Join(s.convsDir, key+".json")

	var doc conversationDoc
	if data, err := os.ReadFile(filePath); err == nil {
		_ = json.Unmarshal(data, &doc)
	}

	if doc.ConversationID == 0 {
		doc.ConversationID = convID
		doc.Messages = []conversationMsg{}
	}

	doc.Messages = append(doc.Messages, conversationMsg{
		Role:      role,
		Content:   content,
		Emotion:   emotion,
		Timestamp: time.Now().Format(time.RFC3339),
	})

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filePath, data, 0644)
}

type conversationDoc struct {
	ConversationID int               `json:"conversation_id"`
	Date           string            `json:"date,omitempty"`
	Messages       []conversationMsg `json:"messages"`
}

type conversationMsg struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Emotion   string `json:"emotion,omitempty"`
	Timestamp string `json:"timestamp"`
}

// GetMessages 获取指定日期的消息（兼容旧接口）
func (s *ConversationService) GetMessages(date string) ([]models.Message, error) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	rows, err := s.db.Query(
		`SELECT m.id, m.conversation_id, m.role, m.content, m.emotion, m.timestamp
		 FROM messages m
		 JOIN conversations c ON m.conversation_id = c.id
		 WHERE c.date = ?
		 ORDER BY m.timestamp`,
		date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		var emotion sql.NullString
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &emotion, &m.Timestamp); err != nil {
			continue
		}
		if emotion.Valid {
			m.Emotion = emotion.String
		}
		messages = append(messages, m)
	}
	return messages, nil
}

// GetRecentMessages 获取最近N条消息（兼容旧接口，跨所有对话）
func (s *ConversationService) GetRecentMessages(limit int) ([]models.Message, error) {
	rows, err := s.db.Query(
		"SELECT id, conversation_id, role, content, emotion, timestamp FROM messages ORDER BY timestamp DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		var emotion sql.NullString
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &emotion, &m.Timestamp); err != nil {
			continue
		}
		if emotion.Valid {
			m.Emotion = emotion.String
		}
		messages = append(messages, m)
	}
	// 反转顺序
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

// SearchMessages 按关键词搜索消息
func (s *ConversationService) SearchMessages(keyword string, limit int) ([]models.Message, error) {
	if keyword == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		`SELECT id, conversation_id, role, content, emotion, timestamp
		 FROM messages
		 WHERE content LIKE ?
		 ORDER BY timestamp DESC
		 LIMIT ?`,
		"%"+keyword+"%", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		var emotion sql.NullString
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &emotion, &m.Timestamp); err != nil {
			continue
		}
		if emotion.Valid {
			m.Emotion = emotion.String
		}
		messages = append(messages, m)
	}
	return messages, nil
}

// UpdateConversationTitleByFirstMessage 用首条用户消息更新对话标题
func (s *ConversationService) UpdateConversationTitleByFirstMessage(conversationID int, content string) error {
	title := summarizeTitle(content)
	_, err := s.db.Exec("UPDATE conversations SET title = ? WHERE id = ? AND (title IS NULL OR title = '' OR title = '新对话')", title, conversationID)
	return err
}
