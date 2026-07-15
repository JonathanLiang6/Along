package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
)

type SettingChangeHandler func(key, oldValue, newValue string) error

// SettingsService 设置服务
type SettingsService struct {
	db            *sql.DB
	encryptionKey []byte
	keyOnce       sync.Once
	mu            sync.RWMutex
	hooks         map[string][]SettingChangeHandler
	defaults      map[string]string
}

// NewSettingsService 创建设置服务
func NewSettingsService(db *sql.DB) *SettingsService {
	s := &SettingsService{
		db:    db,
		hooks: make(map[string][]SettingChangeHandler),
		defaults: map[string]string{
			"api_provider":         "deepseek",
			"theme":                "dark",
			"auto_start":           "false",
			"system_tray_enabled":  "true",
			"close_behavior":       "tray",
			"font_size":            "14px",
			"onboarding_completed": "false",
		},
	}
	return s
}

// InitDefaults 初始化默认设置（幂等）
func (s *SettingsService) InitDefaults() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, defaultValue := range s.defaults {
		current, err := s.getUnlocked(key)
		if err != nil {
			fmt.Printf("读取设置 %s 失败: %v\n", key, err)
			continue
		}
		if current == "" {
			if err := s.setUnlocked(key, defaultValue); err != nil {
				fmt.Printf("初始化默认设置 %s 失败: %v\n", key, err)
			}
		}
	}
	fmt.Println("默认设置初始化完成")
	return nil
}

// OnChange 注册设置变更钩子
func (s *SettingsService) OnChange(key string, handler SettingChangeHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hooks[key] = append(s.hooks[key], handler)
}

// triggerHooks 触发变更钩子（调用方已持有写锁）
func (s *SettingsService) triggerHooks(key, oldValue, newValue string) error {
	hooks := s.hooks[key]
	if len(hooks) == 0 {
		return nil
	}
	for _, hook := range hooks {
		if err := hook(key, oldValue, newValue); err != nil {
			fmt.Printf("设置变更钩子执行失败 [%s]: %v\n", key, err)
		}
	}
	return nil
}

// getEncryptionKey 获取加密密钥（基于机器名和用户名生成）
func (s *SettingsService) getEncryptionKey() []byte {
	s.keyOnce.Do(func() {
		// 使用机器名和用户名生成密钥
		hostname, _ := os.Hostname()
		username := os.Getenv("USERNAME")
		if username == "" {
			username = os.Getenv("USER")
		}

		// 组合信息并生成 SHA-256 哈希作为密钥
		data := hostname + username + "ai-companion-secret-salt"
		hash := sha256.Sum256([]byte(data))
		s.encryptionKey = hash[:]
	})
	return s.encryptionKey
}

// encrypt 使用 AES-256-GCM 加密数据
func (s *SettingsService) encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key := s.getEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建加密块失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 模式失败: %w", err)
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}

	// 加密数据
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// decrypt 使用 AES-256-GCM 解密数据
func (s *SettingsService) decrypt(ciphertextHex string) (string, error) {
	if ciphertextHex == "" {
		return "", nil
	}

	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		// 如果不是有效的十六进制字符串，可能是未加密的旧数据，直接返回
		return ciphertextHex, nil
	}

	key := s.getEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建加密块失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 模式失败: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		// 数据太短，可能是未加密的旧数据，直接返回
		return ciphertextHex, nil
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// 解密失败，可能是未加密的旧数据，直接返回
		return ciphertextHex, nil
	}

	return string(plaintext), nil
}

// GetAll 获取所有设置
func (s *SettingsService) GetAll() (map[string]string, error) {
	if s == nil || s.db == nil {
		return map[string]string{}, fmt.Errorf("设置服务未初始化")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		settings[key] = s.decryptIfNeeded(key, value)
	}
	return settings, nil
}

// Get 获取单个设置
func (s *SettingsService) Get(key string) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("设置服务未初始化")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getUnlocked(key)
}

// getUnlocked 不加锁的内部读取
func (s *SettingsService) getUnlocked(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return s.decryptIfNeeded(key, value), nil
}

// decryptIfNeeded 需要时解密
func (s *SettingsService) decryptIfNeeded(key, value string) string {
	if key == "api_key" {
		decrypted, err := s.decrypt(value)
		if err != nil {
			return value
		}
		return decrypted
	}
	return value
}

// Set 保存设置
func (s *SettingsService) Set(key, value string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("设置服务未初始化")
	}
	if key == "" {
		return fmt.Errorf("设置键不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	oldValue, _ := s.getUnlocked(key)
	if oldValue == value {
		return nil
	}

	if err := s.setUnlocked(key, value); err != nil {
		return err
	}

	s.triggerHooks(key, oldValue, value)
	return nil
}

// setUnlocked 不加锁的内部写入
func (s *SettingsService) setUnlocked(key, value string) error {
	storedValue := value
	if key == "api_key" && value != "" {
		encrypted, err := s.encrypt(value)
		if err != nil {
			return fmt.Errorf("加密失败: %w", err)
		}
		storedValue = encrypted
	}

	_, err := s.db.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, datetime('now'))
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, storedValue,
	)
	if err != nil {
		return fmt.Errorf("保存设置失败: %w", err)
	}
	return nil
}
