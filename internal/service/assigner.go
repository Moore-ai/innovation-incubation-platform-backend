package service

import (
	"sync/atomic"

	"innovation-incubation-platform-backend/internal/repository"
)

type Assigner struct {
	repo   *repository.CommonRepo
	cursor atomic.Int64
}

func NewAssigner(repo *repository.CommonRepo) *Assigner {
	return &Assigner{repo: repo}
}

// Next 轮询返回指定角色的下一个用户 ID
func (a *Assigner) Next(role string) (uint, error) {
	users, err := a.repo.FindUserIDsByRole(role)
	if err != nil || len(users) == 0 {
		return 0, err
	}
	next := a.cursor.Add(1) - 1
	return users[int(next)%len(users)], nil
}
