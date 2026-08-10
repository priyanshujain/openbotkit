package main

import "testing"

func TestValidateRejectsInvalidDomain(t *testing.T) {
	req := &useCaseRequest{
		Title:       "Test",
		Description: "Test",
		Domain:      "NotARealDomain",
	}
	err := validateUseCaseRequest(req)
	if err == nil {
		t.Fatal("expected error for invalid domain")
	}
}

func TestValidateAcceptsValidRequest(t *testing.T) {
	req := &useCaseRequest{
		Title:        "Test",
		Description:  "Test",
		Domain:       "Sales",
		RiskLevel:    "low",
		ROIPotential: "high",
		Status:       "draft",
		ImplStatus:   "evaluating",
		Visibility:   "public",
	}
	err := validateUseCaseRequest(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateRejectsLongTitle(t *testing.T) {
	req := &useCaseRequest{
		Title:       string(make([]byte, 201)),
		Description: "Test",
		Domain:      "Sales",
	}
	err := validateUseCaseRequest(req)
	if err == nil {
		t.Fatal("expected error for long title")
	}
}

func TestValidateRejectsInvalidRiskLevel(t *testing.T) {
	req := &useCaseRequest{
		Title:       "Test",
		Description: "Test",
		Domain:      "Sales",
		RiskLevel:   "extreme",
	}
	err := validateUseCaseRequest(req)
	if err == nil {
		t.Fatal("expected error for invalid risk level")
	}
}
