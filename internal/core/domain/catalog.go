package domain

import "gorm.io/gorm"

// Resource kinds
const (
	ResourceKindCourt = "court"
	ResourceKindPitch = "pitch"
	ResourceKindRing  = "ring"
	ResourceKindOther = "other"
)

// Sport is something clients can book (tennis, boxing, soccer, ...)
type Sport struct {
	gorm.Model
	BranchID    uint   `gorm:"not null;index;uniqueIndex:idx_sports_slug,priority:1,where:deleted_at IS NULL"`
	Name        string `gorm:"type:varchar(120);not null"`
	Slug        string `gorm:"type:varchar(140);not null;uniqueIndex:idx_sports_slug,priority:2,where:deleted_at IS NULL"`
	Description string `gorm:"type:text"`
	ImageKey    string `gorm:"type:varchar(255)"` // object key in file storage, never a full URL
	Active      bool   `gorm:"not null"`          // always set explicitly: gorm would write the default for false
	// Type is one of SportTypes: what customers filter places by
	Type string `gorm:"type:varchar(30);not null;default:other;index"`
}

// Resource is a bookable place: a tennis court, a soccer pitch, a boxing ring
type Resource struct {
	gorm.Model
	BranchID uint   `gorm:"not null;index"`
	SportID  uint   `gorm:"not null;index"`
	Name     string `gorm:"type:varchar(120);not null"`
	Kind     string `gorm:"type:varchar(20);not null"`
	Active   bool   `gorm:"not null"` // always set explicitly: gorm would write the default for false
}

// Coach is a trainer managed by the branch; coaches do not log in
type Coach struct {
	gorm.Model
	BranchID uint    `gorm:"not null;index"`
	Name     string  `gorm:"type:varchar(120);not null"`
	Bio      string  `gorm:"type:text"`
	Phone    string  `gorm:"type:varchar(20)"`
	ImageKey string  `gorm:"type:varchar(255)"`
	Active   bool    `gorm:"not null"` // always set explicitly: gorm would write the default for false
	Sports   []Sport `gorm:"many2many:coach_sports"`
}
