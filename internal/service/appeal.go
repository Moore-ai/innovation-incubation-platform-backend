package service

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/pkg/errcode"
)

type AppealService struct {
	repo *repository.AppealRepo
}

func NewAppealService(repo *repository.AppealRepo) *AppealService {
	return &AppealService{repo: repo}
}

func (s *AppealService) Submit(ctx context.Context, req *dto.SubmitAppealReq, submitterID uint, applicantType model.ApplicantType) (*model.Appeal, error) {
	pt := model.ProblemType(req.ProblemType)
	if !pt.IsValid() {
		return nil, errcode.ErrInvalidParams.WithMsg("无效的问题类型")
	}
	appeal := &model.Appeal{
		Identifier:    req.Identifier,
		ProblemType:   pt,
		Department:    req.Department,
		Content:       req.Content,
		Status:        model.AppealPending,
		ApplicantType: applicantType,
		SubmittedBy:   submitterID,
	}
	if err := s.repo.Create(appeal); err != nil {
		return nil, errcode.ErrInternal.WithMsg("提交诉求失败")
	}
	return appeal, nil
}

func (s *AppealService) ListBySubmitter(ctx context.Context, submitterID uint, page, pageSize int) ([]model.Appeal, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return s.repo.ListBySubmitter(submitterID, page, pageSize)
}

func (s *AppealService) ListAll(ctx context.Context, status, problemType string, page, pageSize int) ([]model.Appeal, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return s.repo.ListAll(status, problemType, page, pageSize)
}

func (s *AppealService) UpdateStatus(ctx context.Context, appealID uint, req *dto.UpdateAppealStatusReq) error {
	appeal, err := s.repo.FindByID(appealID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrNotFound
		}
		return errcode.ErrInternal.WithMsg("查询诉求失败")
	}
	newStatus := model.AppealStatus(req.Status)
	if !newStatus.IsValid() {
		return errcode.ErrInvalidParams.WithMsg("无效的诉求状态")
	}
	if appeal.Status == model.AppealProcessed && newStatus == model.AppealPending {
		return errcode.ErrStatusInvalid.WithMsg("已处理的诉求无法回退为待处理")
	}
	return s.repo.UpdateStatus(appealID, newStatus)
}
