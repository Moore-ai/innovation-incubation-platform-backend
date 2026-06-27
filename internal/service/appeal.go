package service

import (
	"context"

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

func (s *AppealService) Submit(ctx context.Context, req *dto.SubmitAppealReq, submitterID uint) (*model.Appeal, error) {
	pt := model.ProblemType(req.ProblemType)
	if !pt.IsValid() {
		return nil, errcode.ErrInvalidParams.WithMsg("无效的问题类型")
	}
	appeal := &model.Appeal{
		Identifier:  req.Identifier,
		ProblemType: pt,
		Department:  req.Department,
		Content:     req.Content,
		Status:      model.AppealPending,
		SubmittedBy: submitterID,
	}
	if err := s.repo.Create(appeal); err != nil {
		return nil, err
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
	_, err := s.repo.FindByID(appealID)
	if err != nil {
		return errcode.ErrNotFound
	}
	return s.repo.UpdateStatus(appealID, model.AppealStatus(req.Status))
}
