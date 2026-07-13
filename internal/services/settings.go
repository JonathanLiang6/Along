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

// SettingsService 设置服务
type SettingsService struct {
	db           *sql.DB
	encryptionKey []byte
	keyOnce      sync.Once
}

// NewSettingsService 创建设置服务
func NewSettingsService(db *sql.DB) *SettingsService {
	return &SettingsService{db: db}
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

		// 对 api_key 自动解密
		if key == "api_key" {
			decrypted, err := s.decrypt(value)
			if err != nil {
				// 解密失败，使用原值
				settings[key] = value
			} else {
				settings[key] = decrypted
			}
		} else {
			settings[key] = value
		}
	}
	return settings, nil
}

// Get 获取单个设置
func (s *SettingsService) Get(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	// 对 api_key 自动解密
	if key == "api_key" {
		decrypted, decryptErr := s.decrypt(value)
		if decryptErr != nil {
			// 解密失败，返回原值
			return value, nil
		}
		return decrypted, nil
	}

	return value, nil
}

// Set 保存设置
func (s *SettingsService) Set(key, value string) error {
	// 对 api_key 自动加密
	if key == "api_key" && value != "" {
		encrypted, err := s.encrypt(value)
		if err != nil {
			return fmt.Errorf("加密失败: %w", err)
		}
		value = encrypted
	}

	_, err := s.db.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, datetime('now'))
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("保存设置失败: %w", err)
	}
	return nil
}
