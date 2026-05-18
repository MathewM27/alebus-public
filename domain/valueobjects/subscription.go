package valueobjects

import "time"

type SubscriptionPlan int 

const (
	SubscriptionPlanFree SubscriptionPlan = iota
	SubscriptionPlanBasic
	SubscriptionPlanPremium
)

type SubscriptionStatus int 

const (
	SubscriptionStatusActive SubscriptionStatus = iota
	SubscriptionStatusInactive
	SubscriptionStatusSuspended
	SubscriptionStatusCancelled
	SubscriptionStatusExpired
)

type Subscription struct {
	Status 	 SubscriptionStatus
	Plan    SubscriptionPlan
	StartDate time.Time
	ExpiryDate time.Time
}

func (s Subscription) IsActive() bool {
	return s.Status == SubscriptionStatusActive && time.Now().Before(s.ExpiryDate)
}

func (s Subscription) GetMaxConcurrentJourneys() int {
	switch s.Plan {
		case SubscriptionPlanFree:
			return 1
		case SubscriptionPlanBasic:
			return 1
		case SubscriptionPlanPremium:
			return 999
		default:
			return 0
	}
}