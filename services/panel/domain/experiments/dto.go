package experiments

import (
	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
)

type CreateExperimentRequest struct {
	FlagID       string         `json:"flag_id" binding:"required,uuid"`
	Name         string         `json:"name" binding:"required,min=1,max=256"`
	Description  string         `json:"description"`
	WeightsTotal int            `json:"weights_total"`
	Targeting    *Targeting     `json:"targeting"`
	Variants     []VariantInput `json:"variants"`
}

type UpdateDraftRequest struct {
	Version     int     `json:"version" binding:"required,min=1"`
	Name        string  `json:"name" binding:"omitempty,min=1,max=256"`
	Description *string `json:"description"`
}

type CreateVersionRequest struct {
	Version      int            `json:"version" binding:"required,min=1"`
	WeightsTotal int            `json:"weights_total"`
	Targeting    *Targeting     `json:"targeting"`
	Variants     []VariantInput `json:"variants"`
}

type SetVariantsRequest struct {
	Version  int            `json:"version" binding:"required,min=1"`
	Variants []VariantInput `json:"variants" binding:"required,min=2"`
}

type TransitionRequest struct {
	Version int `json:"version" binding:"required,min=1"`
}

type CompleteRequest struct {
	Version         int                `json:"version" binding:"required,min=1"`
	Decision        CompletionDecision `json:"decision" binding:"required,oneof=rollout_winner rollback no_effect"`
	Reason          string             `json:"reason" binding:"required,min=1,max=4096"`
	WinnerVariantID *string            `json:"winner_variant_id"`
}

type RolloutRequest struct {
	Version         int     `json:"version" binding:"required,min=1"`
	Reason          string  `json:"reason" binding:"required,min=1,max=4096"`
	WinnerVariantID *string `json:"winner_variant_id" binding:"required"`
}

type VariantResponse struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Value     VariantValue `json:"value"`
	WeightBP  int          `json:"weight_bp"`
	IsControl bool         `json:"is_control"`
}

type VersionResponse struct {
	api.ResourceResponse
	ExperimentID     string     `json:"experiment_id"`
	VersionNum       int        `json:"version_num"`
	ReviewID         *string    `json:"review_id"`
	WeightsTotal     int        `json:"weights_total"`
	Targeting        *Targeting `json:"targeting"`
	DistributionSalt string     `json:"distribution_salt"`
	CreatedBy        string     `json:"created_by"`
}

type ExperimentResponse struct {
	api.ResourceResponse
	FlagID             string              `json:"flag_id"`
	Name               string              `json:"name"`
	Description        *string             `json:"description"`
	Status             Status              `json:"status"`
	CurrentVersionID   *string             `json:"current_version_id"`
	OwnerID            string              `json:"owner_id"`
	Version            int                 `json:"version"`
	GuardrailPaused    bool                `json:"guardrail_paused"`
	CompletionDecision *CompletionDecision `json:"completion_decision"`
	CompletionReason   *string             `json:"completion_reason"`
	CreatedBy          *users.UserResponse `json:"created_by"`
	UpdatedBy          *users.UserResponse `json:"updated_by"`
	CurrentVersion     *VersionResponse    `json:"current_version"`
	Variants           []VariantResponse   `json:"variants"`
}

type PaginatedExperimentResponse struct {
	Data []ExperimentResponse `json:"data"`
	Meta api.PaginationMeta   `json:"meta"`
}

func ToResponse(d ExperimentDetail) ExperimentResponse {
	var createdBy *users.UserResponse
	if d.CreatedBy != nil {
		resp := users.ToResponse(*d.CreatedBy)
		createdBy = &resp
	}
	var updatedBy *users.UserResponse
	if d.UpdatedBy != nil {
		resp := users.ToResponse(*d.UpdatedBy)
		updatedBy = &resp
	}
	resp := ExperimentResponse{
		ResourceResponse: api.ResourceResponse{
			ID:        d.Experiment.ID,
			CreatedAt: d.Experiment.CreatedAt,
			UpdatedAt: d.Experiment.UpdatedAt,
		},
		FlagID:             d.Experiment.FlagID,
		Name:               d.Experiment.Name,
		Description:        d.Experiment.Description,
		Status:             d.Experiment.Status,
		CurrentVersionID:   d.Experiment.CurrentVersionID,
		OwnerID:            d.Experiment.OwnerID,
		Version:            d.Experiment.Version,
		GuardrailPaused:    d.Experiment.GuardrailPaused,
		CompletionDecision: d.Experiment.CompletionDecision,
		CompletionReason:   d.Experiment.CompletionReason,
		CreatedBy:          createdBy,
		UpdatedBy:          updatedBy,
	}
	if d.CurrentVersion != nil {
		resp.CurrentVersion = &VersionResponse{
			ResourceResponse: api.ResourceResponse{
				ID:        d.CurrentVersion.ID,
				CreatedAt: d.CurrentVersion.CreatedAt,
				UpdatedAt: d.CurrentVersion.CreatedAt,
			},
			ExperimentID:     d.CurrentVersion.ExperimentID,
			VersionNum:       d.CurrentVersion.VersionNum,
			ReviewID:         d.CurrentVersion.ReviewID,
			WeightsTotal:     d.CurrentVersion.WeightsTotal,
			Targeting:        d.CurrentVersion.Targeting,
			DistributionSalt: d.CurrentVersion.DistributionSalt,
			CreatedBy:        d.CurrentVersion.CreatedBy,
		}
	}
	for _, v := range d.Variants {
		resp.Variants = append(resp.Variants, VariantResponse{
			ID:        v.ID,
			Name:      v.Name,
			Value:     v.Value,
			WeightBP:  v.WeightBP,
			IsControl: v.IsControl,
		})
	}
	return resp
}
