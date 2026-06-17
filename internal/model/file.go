package model

type File struct {
	BaseModel
	Filename   string `gorm:"size:255;not null" json:"filename"`
	MimeType   string `gorm:"size:128" json:"mime_type"`
	Size       int64  `json:"size"`
	Data       []byte `gorm:"type:bytea;not null" json:"-"`
	UploadedBy uint   `gorm:"index" json:"-"`
}

func (File) TableName() string { return "files" }
