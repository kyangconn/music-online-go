// Package domain session.go - 可撤销登录会话模型
//
// 每个浏览器/客户端登录对应一行 Session：refresh token 以 SHA-256 哈希
// 形式存储，轮换时原地更新哈希，撤销时写入 RevokedAt。服务端始终掌握
// 会话生命周期，access token 只携带短时身份信息并引用会话 ID。
package domain

import "time"

// Session represents one server-side login session that owns a refresh token.
type Session struct {
	ID          uint   `gorm:"primarykey"`
	UserID      uint   `gorm:"not null;index"`
	RefreshHash string `gorm:"size:64;not null;uniqueIndex"` // SHA-256 hex of the opaque refresh token
	UserAgent   string `gorm:"size:512"`
	IPAddress   string `gorm:"size:64"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	LastUsedAt  time.Time  `gorm:"index"`
	ExpiresAt   time.Time  `gorm:"not null;index"`
	RevokedAt   *time.Time `gorm:"index"`
}
