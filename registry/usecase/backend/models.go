package main

import "time"

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	OrgName   string    `json:"org_name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UseCase struct {
	ID                string    `json:"id"`
	Title             string    `json:"title"`
	Slug              string    `json:"slug"`
	Description       string    `json:"description"`
	Domain            string    `json:"domain"`
	IndustryTags      string    `json:"industry_tags,omitempty"`
	RiskLevel         string    `json:"risk_level"`
	ROIPotential      string    `json:"roi_potential"`
	Status            string    `json:"status"`
	ImplStatus        string    `json:"impl_status"`
	Visibility        string    `json:"visibility"`
	SafetyPII         bool      `json:"safety_pii"`
	SafetyAutonomous  bool      `json:"safety_autonomous"`
	SafetyBlastRadius string    `json:"safety_blast_radius,omitempty"`
	SafetyOversight   string    `json:"safety_oversight,omitempty"`
	ForkedFrom        *string   `json:"forked_from,omitempty"`
	ForkCount         int       `json:"fork_count"`
	AuthorID          string    `json:"author_id"`
	Author            *User     `json:"author,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
