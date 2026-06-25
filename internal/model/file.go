package model

type File struct {
	BaseModel
	Filename    string `gorm:"size:255;not null" json:"filename"`
	MimeType    string `gorm:"size:128" json:"mime_type"`
	Size        int64  `json:"size"`
	StoragePath string `gorm:"size:512" json:"-"`
	UploadedBy  uint   `gorm:"index" json:"-"`
	RawText     string `gorm:"type:text" json:"-"`
	Summary     string `gorm:"type:text" json:"summary"`
}

func (File) TableName() string { return "files" }
