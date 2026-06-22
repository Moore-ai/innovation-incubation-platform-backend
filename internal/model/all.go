package model

func AllModels() []any {
	return []any{
		&User{},
		&Enterprise{},
		&Carrier{},
		&IncubationRecord{},
		&MajorChange{},
		&PolicyTemplate{},
		&Policy{},
		&PolicyApplication{},
		&Approval{},
		&PerformanceTemplate{},
		&PerformanceCampaign{},
		&PerformanceSubmission{},
		&File{},
		&Notification{},
		&AccountDeletionRequest{},
	}
}
