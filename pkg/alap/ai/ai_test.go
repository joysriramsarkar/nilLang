package ai

import (
	"strings"
	"testing"
)

func TestHallucinationGuard(t *testing.T) {
	truth := NewApplicationTruth()

	// 1. Test Valid Member: Button.onClick
	validRes := truth.ValidateMemberAccess("Button", "onClick")
	if !validRes.Valid {
		t.Errorf("expected Button.onClick to be valid")
	}

	// 2. Test Hallucinated Member from Section 15 of refactor.md: button.saveToCloud()
	hallucinatedRes := truth.ValidateMemberAccess("Button", "saveToCloud")
	if hallucinatedRes.Valid {
		t.Errorf("expected button.saveToCloud to be flagged as invalid")
	}
	if !strings.Contains(hallucinatedRes.ErrorMessage, "Unknown member: button.saveToCloud()") {
		t.Errorf("unexpected error message: %s", hallucinatedRes.ErrorMessage)
	}
	if len(hallucinatedRes.AvailableMembers) == 0 {
		t.Errorf("expected available members list for AI feedback")
	}
}

func TestVerifiedNoveltyPipeline(t *testing.T) {
	pipeline := NewVerifiedNoveltyPipeline()

	// Proposal matching Section 15 of refactor.md: SaveToCloudButton
	proposal := &ComponentProposal{
		Name:        "SaveToCloudButton",
		Author:      "AI-Agent",
		Status:      StatusExperimental,
		SourceCode:  "component SaveToCloudButton { render { button \"Save to Cloud\" } }",
		Tests:       "test { assert(true) }",
		Permissions: []string{"Network"},
	}

	report := pipeline.Verify(proposal)
	if report.FinalStatus != StatusVerified {
		t.Errorf("expected proposal to be VERIFIED, got %s", report.FinalStatus)
	}
	if len(report.Stages) != 6 {
		t.Errorf("expected 6 pipeline stages, got %d", len(report.Stages))
	}
	if proposal.Status != StatusVerified {
		t.Errorf("proposal status not updated to VERIFIED")
	}
}
