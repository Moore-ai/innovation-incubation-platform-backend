package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/internal/service"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type FileController struct {
	svc *service.FileService
	cfg *config.Config
}

func NewFileController(svc *service.FileService, cfg *config.Config) *FileController {
	return &FileController{svc: svc, cfg: cfg}
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

	f, err := ctl.svc.Upload(c.Request.Context(), file, header.Filename, header.Header.Get("Content-Type"), header.Size, middleware.GetUserID(c))
	if err != nil {
		response.Error(c, err)
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
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	if role == "government" {
		if userIDStr := c.Query("user_id"); userIDStr != "" {
			id, err := strconv.ParseUint(userIDStr, 10, 64)
			if err != nil {
				response.Error(c, errcode.ErrInvalidParams.WithMsg("user_id 参数无效"))
				return
			}
			list, total, err := ctl.svc.ListByUploader(uint(id), page, pageSize)
			if err != nil {
				response.Error(c, errcode.ErrInternal)
				return
			}
			response.SuccessPage(c, list, total, page, pageSize)
			return
		}
		list, total, err := ctl.svc.ListAll(page, pageSize)
		if err != nil {
			response.Error(c, errcode.ErrInternal)
			return
		}
		response.SuccessPage(c, list, total, page, pageSize)
		return
	}

	list, total, err := ctl.svc.ListByUploader(userID, page, pageSize)
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}
	response.SuccessPage(c, list, total, page, pageSize)
}

func (ctl *FileController) DeleteFile(c *gin.Context) {
	if middleware.GetRole(c) != "government" {
		response.Error(c, errcode.ErrForbidden)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrInvalidParams)
		return
	}

	f, err := ctl.svc.GetMeta(uint(id))
	if err != nil {
		response.Error(c, errcode.ErrNotFound.WithMsg("文件不存在"))
		return
	}

	if ctl.svc.IsReferenced(uint(id)) {
		response.Error(c, errcode.ErrInvalidParams.WithMsg("文件正在被入驻记录引用，无法删除"))
		return
	}

	if err := ctl.svc.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"file_id": f.ID})
}

func (ctl *FileController) GetUploadLimit(c *gin.Context) {
	response.Success(c, gin.H{
		"max_size_mb":        ctl.cfg.Upload.MaxSizeMB,
		"allowed_extensions": ctl.cfg.Upload.AllowedExtensions,
	})
}

func (ctl *FileController) Download(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetRole(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrInvalidParams)
		return
	}

	f, err := ctl.svc.GetMeta(uint(id))
	if err != nil {
		response.Error(c, errcode.ErrNotFound.WithMsg("文件不存在"))
		return
	}

	if f.UploadedBy != userID && role != "government" {
		hasAccess, err := ctl.svc.HasFileAccess(f.ID, userID)
		if err != nil {
			response.Error(c, errcode.ErrInternal)
			return
		}
		if !hasAccess {
			response.Error(c, errcode.ErrForbidden)
			return
		}
	}

	reader, err := ctl.svc.Open(c.Request.Context(), uint(id))
	if err != nil {
		response.Error(c, err)
		return
	}
	defer reader.Close()

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, url.PathEscape(f.Filename), url.PathEscape(f.Filename)))
	c.Header("Content-Type", f.MimeType)
	http.ServeContent(c.Writer, c.Request, f.Filename, f.CreatedAt, reader)
}
