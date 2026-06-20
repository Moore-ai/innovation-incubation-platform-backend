package controller

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type FileController struct {
	fileRepo *repository.FileRepo
	cfg      *config.Config
}

func NewFileController(fileRepo *repository.FileRepo, cfg *config.Config) *FileController {
	return &FileController{fileRepo: fileRepo, cfg: cfg}
}

func (ctl *FileController) Upload(c *gin.Context) {
	if ctl.cfg.Upload.MaxSizeMB > 0 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ctl.cfg.Upload.MaxSizeMB*1024*1024)
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg("文件过大或上传失败"))
		return
	}
	defer file.Close()

	buf := make([]byte, header.Size)
	if _, err := io.ReadFull(file, buf); err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}

	f := &model.File{
		Filename:   header.Filename,
		MimeType:   header.Header.Get("Content-Type"),
		Size:       header.Size,
		Data:       buf,
		UploadedBy: middleware.GetUserID(c),
	}

	if err := ctl.fileRepo.Create(f); err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}

	response.Success(c, gin.H{
		"file_id":   f.ID,
		"filename":  f.Filename,
		"mime_type": f.MimeType,
		"size":      f.Size,
	})
}

func (ctl *FileController) ListFiles(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetRole(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if role == "government" {
		if userIDStr := c.Query("user_id"); userIDStr != "" {
			if id, err := strconv.ParseUint(userIDStr, 10, 64); err == nil {
				list, total, err := ctl.fileRepo.ListByUploader(uint(id), page, pageSize)
				if err != nil {
					response.Error(c, errcode.ErrInternal)
					return
				}
				response.SuccessPage(c, list, total, page, pageSize)
				return
			}
		}
		list, total, err := ctl.fileRepo.ListAll(page, pageSize)
		if err != nil {
			response.Error(c, errcode.ErrInternal)
			return
		}
		response.SuccessPage(c, list, total, page, pageSize)
		return
	}

	list, total, err := ctl.fileRepo.ListByUploader(userID, page, pageSize)
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}
	response.SuccessPage(c, list, total, page, pageSize)
}

func (ctl *FileController) DeleteFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrInvalidParams)
		return
	}

	if middleware.GetRole(c) != "government" {
		response.Error(c, errcode.ErrForbidden)
		return
	}

	f, err := ctl.fileRepo.FindByID(uint(id))
	if err != nil {
		response.Error(c, errcode.ErrNotFound.WithMsg("文件不存在"))
		return
	}

	if ctl.fileRepo.IsReferenced(uint(id)) {
		response.Error(c, errcode.ErrInvalidParams.WithMsg("文件正在被入驻记录引用，无法删除"))
		return
	}

	if err := ctl.fileRepo.Delete(uint(id)); err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}
	response.Success(c, gin.H{"file_id": f.ID})
}

func (ctl *FileController) GetUploadLimit(c *gin.Context) {
	response.Success(c, gin.H{
		"max_size_mb": ctl.cfg.Upload.MaxSizeMB,
	})
}

func (ctl *FileController) Download(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrInvalidParams)
		return
	}

	f, err := ctl.fileRepo.FindByID(uint(id))
	if err != nil {
		response.Error(c, errcode.ErrNotFound.WithMsg("文件不存在"))
		return
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetRole(c)

	if f.UploadedBy != userID && role != "government" {
		hasAccess, _ := ctl.fileRepo.CheckFileAccess(f.ID, userID)
		if !hasAccess {
			response.Error(c, errcode.ErrForbidden)
			return
		}
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, url.PathEscape(f.Filename)))
	c.Header("Content-Type", f.MimeType)
	c.Data(200, f.MimeType, f.Data)
}
