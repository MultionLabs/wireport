package jointokens

import "time"

type JoinToken struct {
	Token     string    `gorm:"type:text;primaryKey;uniqueIndex:idx_join_token"`
	CreatedAt time.Time `gorm:"type:timestamp;not null"`
}
