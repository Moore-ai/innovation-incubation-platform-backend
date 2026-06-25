package service

import (
	"context"
	"log/slog"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/filematch"
)

// PrefillApplication prefills application materials with the user's uploaded files
// by matching material names against file names using the filematch package.
// Returns a list of MaterialFileItem — one per material, each with matched file IDs.
func (s *AIService) PrefillApplication(ctx context.Context, userID uint, policyID uint) ([]model.MaterialFileItem, error) {
	policy, err := s.govRepo.FindPolicyByID(policyID)
	if err != nil {
		return nil, errcode.ErrNotFound.WithMsg("政策不存在")
	}
	if policy.Requirements == nil || len(policy.Requirements.ApplicationMaterials) == 0 {
		return nil, errcode.ErrInvalidParams.WithMsg("政策暂无申报材料要求")
	}

	// 获取用户历史文件
	files, _, err := s.fileRepo.ListByUploader(userID, 1, 1000)
	if err != nil {
		slog.Warn("failed to list user files for prefill", "user_id", userID, "error", err)
		// 文件列表获取失败时返回空结果（材料名称仍然返回）
		var result []model.MaterialFileItem
		for _, m := range policy.Requirements.ApplicationMaterials {
			result = append(result, model.MaterialFileItem{Name: m.Name})
		}
		return result, nil
	}

	cfg := s.fileMatchCfg
	var result []model.MaterialFileItem
	for _, m := range policy.Requirements.ApplicationMaterials {
		item := model.MaterialFileItem{Name: m.Name}
		match := filematch.Match(m.Name, m.MaterialFormats, files, cfg)
		if match != nil {
			item.FileIDs = []uint{match.FileID}
			if match.Warning != "" {
				slog.Warn("file match warning", "material", m.Name, "warning", match.Warning)
			}
		}
		result = append(result, item)
	}
	return result, nil
}
