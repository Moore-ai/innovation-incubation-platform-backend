package model

func AllModels() []any {
	return []any{
		&User{},
		&Enterprise{},
		&Carrier{},
		&IncubationRecord{},
		&MajorChange{},
		&Policy{},
		&PolicyApplication{},
		&Approval{},
		&PerformanceTemplate{},
		&PerformanceCampaign{},
		&PerformanceSubmission{},
		&File{},
		&Government{},
		&Notification{},
		&AccountDeletionRequest{},
		&PolicyFollow{},
		&Appeal{},
	}
}
