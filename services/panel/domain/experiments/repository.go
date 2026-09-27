package experiments

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/audit"
	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

type experimentAuditState struct {
	Status          Status `json:"status"`
	Version         int    `json:"version"`
	GuardrailPaused bool   `json:"guardrail_paused"`
}

func appendExperimentStateAudit(ctx context.Context, tx pgx.Tx, experimentID, callerID, action, reason string, before, after experimentAuditState) error {
	return audit.Append(ctx, tx, audit.Record{
		ActorType:    "user",
		ActorID:      callerID,
		Action:       action,
		ResourceType: "experiment",
		ResourceID:   experimentID,
		Reason:       reason,
		Before:       before,
		After:        after,
	})
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db}
}

func mapUniqueViolation(err error) error {
	if !database.IsUniqueViolation(err) {
		return err
	}
	if strings.Contains(err.Error(), "uniq_active_experiment_per_flag") {
		return ErrFlagBusy
	}
	if strings.Contains(err.Error(), "experiments_name_key") {
		return ErrConflictName
	}
	if strings.Contains(err.Error(), "uq_variants_version_name") {
		return ErrInvalidVariants
	}
	return ErrConflictName
}

func (r *Repository) CreateExperiment(ctx context.Context, exp Experiment, weightsTotal int, targeting *Targeting, salt string) (expID, versionID string, err error) {
	expUID, err := uuid.NewV7()
	if err != nil {
		return "", "", err
	}
	verUID, err := uuid.NewV7()
	if err != nil {
		return "", "", err
	}
	expID = expUID.String()
	versionID = verUID.String()

	var tgt any
	if targeting != nil {
		tgt = *targeting
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const expQ = `
INSERT INTO experiments(id, flag_id, name, description, status, owner_id, created_by, updated_by)
    VALUES ($1, $2, $3, $4, 'draft', $5, $6, $7)`
	if _, err = tx.Exec(ctx, expQ, expID, exp.FlagID, exp.Name, exp.Description, exp.OwnerID, exp.CreatedBy, exp.UpdatedBy); err != nil {
		if database.IsForeignKeyViolation(err) {
			return "", "", ErrFlagNotFound
		}
		return "", "", mapUniqueViolation(err)
	}

	const verQ = `
INSERT INTO experiment_versions(id, experiment_id, version_num, weights_total, targeting_expr, distribution_salt, created_by)
    VALUES ($1, $2, 1, $3, $4, $5, $6)`
	if _, err = tx.Exec(ctx, verQ, versionID, expID, weightsTotal, tgt, salt, exp.CreatedBy); err != nil {
		return "", "", err
	}

	const curQ = `UPDATE experiments SET current_version_id = $1 WHERE id = $2`
	if _, err = tx.Exec(ctx, curQ, versionID, expID); err != nil {
		return "", "", err
	}
	if err := audit.Append(ctx, tx, audit.Record{
		ActorType:    "user",
		ActorID:      exp.CreatedBy,
		Action:       "experiment.created",
		ResourceType: "experiment",
		ResourceID:   expID,
		After:        experimentAuditState{Status: StatusDraft, Version: 1},
	}); err != nil {
		return "", "", err
	}

	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return expID, versionID, nil
}

const experimentColumns = `
    e.id,
    e.flag_id,
    e.name,
    e.description,
    e.status,
    e.current_version_id::text,
    e.owner_id::text,
    e.version,
    e.guardrail_paused,
    e.completion_decision::text,
    e.completion_reason,
    e.created_by::text,
    e.updated_by::text,
    e.created_at,
    e.updated_at`

func scanExperiment(row pgx.Row) (Experiment, error) {
	var (
		exp                Experiment
		currentVersionID   *string
		description        *string
		completionReason   *string
		completionDecision *string
	)
	err := row.Scan(
		&exp.ID,
		&exp.FlagID,
		&exp.Name,
		&description,
		&exp.Status,
		&currentVersionID,
		&exp.OwnerID,
		&exp.Version,
		&exp.GuardrailPaused,
		&completionDecision,
		&completionReason,
		&exp.CreatedBy,
		&exp.UpdatedBy,
		&exp.CreatedAt,
		&exp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Experiment{}, ErrNotFound
		}
		return Experiment{}, err
	}
	exp.Description = description
	exp.CurrentVersionID = currentVersionID
	if completionDecision != nil {
		exp.CompletionDecision = (*CompletionDecision)(completionDecision)
	}
	exp.CompletionReason = completionReason
	return exp, nil
}

func (r *Repository) GetState(ctx context.Context, id string) (Experiment, error) {
	q := `SELECT ` + experimentColumns + ` FROM experiments e WHERE e.id = $1`
	return scanExperiment(r.db.QueryRow(ctx, q, id))
}

func (r *Repository) GetDetail(ctx context.Context, id string) (ExperimentDetail, error) {
	var d ExperimentDetail

	q := `
SELECT
` + experimentColumns + `,
    cb.id,
    cb.full_name,
    cb.email,
    cb.role,
    cb.avatar_url,
    cb.created_at,
    cb.updated_at,
    ub.id,
    ub.full_name,
    ub.email,
    ub.role,
    ub.avatar_url,
    ub.created_at,
    ub.updated_at
FROM
    experiments e
    JOIN users cb ON e.created_by = cb.id
    JOIN users ub ON e.updated_by = ub.id
WHERE
    e.id = $1`

	var (
		exp                Experiment
		description        *string
		currentVersionID   *string
		completionDecision *string
		completionReason   *string
		creator            users.User
		updater            users.User
		creatorAvatar      *string
		updaterAvatar      *string
		creatorRole        users.Role
		updaterRole        users.Role
		creatorCreated     time.Time
		creatorUpdated     time.Time
		updaterCreated     time.Time
		updaterUpdated     time.Time
	)
	err := r.db.QueryRow(ctx, q, id).Scan(
		&exp.ID,
		&exp.FlagID,
		&exp.Name,
		&description,
		&exp.Status,
		&currentVersionID,
		&exp.OwnerID,
		&exp.Version,
		&exp.GuardrailPaused,
		&completionDecision,
		&completionReason,
		&exp.CreatedBy,
		&exp.UpdatedBy,
		&exp.CreatedAt,
		&exp.UpdatedAt,
		&creator.ID,
		&creator.FullName,
		&creator.Email,
		&creatorRole,
		&creatorAvatar,
		&creatorCreated,
		&creatorUpdated,
		&updater.ID,
		&updater.FullName,
		&updater.Email,
		&updaterRole,
		&updaterAvatar,
		&updaterCreated,
		&updaterUpdated,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExperimentDetail{}, ErrNotFound
		}
		return ExperimentDetail{}, err
	}
	exp.Description = description
	exp.CurrentVersionID = currentVersionID
	if completionDecision != nil {
		exp.CompletionDecision = (*CompletionDecision)(completionDecision)
	}
	exp.CompletionReason = completionReason
	creator.Role = creatorRole
	creator.CreatedAt = creatorCreated
	creator.UpdatedAt = creatorUpdated
	updater.Role = updaterRole
	updater.CreatedAt = updaterCreated
	updater.UpdatedAt = updaterUpdated
	d.Experiment = exp
	d.CreatedBy = &creator
	d.UpdatedBy = &updater

	if currentVersionID != nil {
		ver, err := r.getVersion(ctx, *currentVersionID)
		if err != nil {
			return ExperimentDetail{}, err
		}
		d.CurrentVersion = &ver
		d.Variants, err = r.listVariants(ctx, ver.ID)
		if err != nil {
			return ExperimentDetail{}, err
		}
	}
	return d, nil
}

func scanVersion(row pgx.Row) (ExperimentVersion, error) {
	var (
		ver       ExperimentVersion
		reviewID  *string
		targeting []byte
	)
	err := row.Scan(
		&ver.ID,
		&ver.ExperimentID,
		&ver.VersionNum,
		&reviewID,
		&ver.WeightsTotal,
		&targeting,
		&ver.DistributionSalt,
		&ver.CreatedAt,
		&ver.CreatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExperimentVersion{}, ErrVersionNotFound
		}
		return ExperimentVersion{}, err
	}
	ver.ReviewID = reviewID
	if targeting != nil {
		t := Targeting(targeting)
		ver.Targeting = &t
	}
	return ver, nil
}

const versionColumns = `
    v.id,
    v.experiment_id,
    v.version_num,
    v.review_id::text,
    v.weights_total,
    v.targeting_expr,
    v.distribution_salt,
    v.created_at,
    v.created_by::text`

func (r *Repository) getVersion(ctx context.Context, versionID string) (ExperimentVersion, error) {
	q := `SELECT ` + versionColumns + ` FROM experiment_versions v WHERE v.id = $1`
	return scanVersion(r.db.QueryRow(ctx, q, versionID))
}

func (r *Repository) listVariants(ctx context.Context, versionID string) ([]Variant, error) {
	const q = `
SELECT
    id,
    version_id,
    name,
    value,
    weight_bp,
    is_control
FROM
    variants
WHERE
    version_id = $1
ORDER BY
    name`
	rows, err := r.db.Query(ctx, q, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Variant
	for rows.Next() {
		var v Variant
		var value []byte
		if err := rows.Scan(&v.ID, &v.VersionID, &v.Name, &value, &v.WeightBP, &v.IsControl); err != nil {
			return nil, err
		}
		v.Value = VariantValue(value)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) CreateVersion(ctx context.Context, experimentID string, weightsTotal int, targeting *Targeting, salt, callerID string, resetToDraft bool) (versionID string, versionNum int, err error) {
	verUID, err := uuid.NewV7()
	if err != nil {
		return "", 0, err
	}
	versionID = verUID.String()

	var tgt any
	if targeting != nil {
		tgt = *targeting
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var before experimentAuditState
	const lockQ = `SELECT status, version, guardrail_paused FROM experiments WHERE id = $1 FOR UPDATE`
	if err = tx.QueryRow(ctx, lockQ, experimentID).Scan(&before.Status, &before.Version, &before.GuardrailPaused); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", 0, ErrNotFound
		}
		return "", 0, err
	}

	var maxNum *int
	const maxQ = `SELECT MAX(version_num) FROM experiment_versions WHERE experiment_id = $1`
	if err = tx.QueryRow(ctx, maxQ, experimentID).Scan(&maxNum); err != nil {
		return "", 0, err
	}
	versionNum = 1
	if maxNum != nil {
		versionNum = *maxNum + 1
	}

	const verQ = `
INSERT INTO experiment_versions(id, experiment_id, version_num, weights_total, targeting_expr, distribution_salt, created_by)
    VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err = tx.Exec(ctx, verQ, versionID, experimentID, versionNum, weightsTotal, tgt, salt, callerID); err != nil {
		if strings.Contains(err.Error(), "uq_versions_experiment_num") {
			return "", 0, ErrVersionConflict
		}
		return "", 0, err
	}

	const curQ = `UPDATE experiments SET current_version_id = $1, version = version + 1, updated_by = $2 WHERE id = $3`
	if _, err = tx.Exec(ctx, curQ, versionID, callerID, experimentID); err != nil {
		return "", 0, err
	}

	if resetToDraft {
		const resetQ = `UPDATE experiments SET status = 'draft', completion_decision = NULL, completion_reason = NULL WHERE id = $1`
		if _, err = tx.Exec(ctx, resetQ, experimentID); err != nil {
			return "", 0, err
		}
		after := experimentAuditState{Status: StatusDraft, Version: before.Version + 1, GuardrailPaused: before.GuardrailPaused}
		if err := appendExperimentStateAudit(ctx, tx, experimentID, callerID, "experiment.version_created_status_reset", "", before, after); err != nil {
			return "", 0, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return "", 0, err
	}
	return versionID, versionNum, nil
}

func (r *Repository) SetVariants(ctx context.Context, experimentID, versionID string, inputs []VariantInput, expectedVersion int, callerID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const bumpQ = `UPDATE experiments SET version = version + 1, updated_by = $1 WHERE id = $2 AND version = $3`
	tag, err := tx.Exec(ctx, bumpQ, callerID, experimentID, expectedVersion)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrVersionConflict
	}

	if _, err = tx.Exec(ctx, `DELETE FROM variants WHERE version_id = $1`, versionID); err != nil {
		return err
	}

	const insQ = `
INSERT INTO variants(id, version_id, name, value, weight_bp, is_control)
    VALUES ($1, $2, $3, $4, $5, $6)`
	for _, in := range inputs {
		uid, err := uuid.NewV7()
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, insQ, uid.String(), versionID, in.Name, in.Value, in.WeightBP, in.IsControl); err != nil {
			return mapUniqueViolation(err)
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) Transition(ctx context.Context, id string, from, to Status, expectedVersion int, callerID string, guardrailPaused *bool) (Experiment, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Experiment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var before experimentAuditState
	err = tx.QueryRow(ctx, `
SELECT
    status,
    version,
    guardrail_paused
FROM
    experiments
WHERE
    id = $1
FOR UPDATE`, id).Scan(&before.Status, &before.Version, &before.GuardrailPaused)
	if errors.Is(err, pgx.ErrNoRows) {
		return Experiment{}, ErrNotFound
	}
	if err != nil {
		return Experiment{}, err
	}
	if before.Status != from {
		return Experiment{}, ErrInvalidTransition
	}
	if before.Version != expectedVersion {
		return Experiment{}, ErrVersionConflict
	}
	guardrailState := before.GuardrailPaused
	if guardrailPaused != nil {
		guardrailState = *guardrailPaused
	}
	q := `
UPDATE
    experiments AS e
SET
    status = $1,
    version = e.version + 1,
    guardrail_paused = COALESCE($2, e.guardrail_paused),
    updated_by = $3
WHERE
    e.id = $4 AND e.status = $5 AND e.version = $6
RETURNING` + experimentColumns
	row := tx.QueryRow(ctx, q, string(to), guardrailPaused, callerID, id, string(from), expectedVersion)
	exp, err := scanExperiment(row)
	if err != nil {
		return Experiment{}, err
	}
	after := experimentAuditState{Status: to, Version: exp.Version, GuardrailPaused: guardrailState}
	if err := appendExperimentStateAudit(ctx, tx, id, callerID, "experiment.status_transition", "", before, after); err != nil {
		return Experiment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Experiment{}, err
	}
	return exp, nil
}

func (r *Repository) RollbackToControl(ctx context.Context, id string, expectedVersion int, callerID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		status            Status
		flagID            string
		currentVersionID  *string
		currentVersionNum int
		guardrailPaused   bool
	)
	err = tx.QueryRow(ctx, `
SELECT
    status,
    flag_id::text,
    current_version_id::text,
	version,
	guardrail_paused
FROM
    experiments
WHERE
    id = $1
FOR UPDATE`, id).Scan(&status, &flagID, &currentVersionID, &currentVersionNum, &guardrailPaused)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != StatusRunning {
		return ErrInvalidTransition
	}
	if currentVersionNum != expectedVersion {
		return ErrVersionConflict
	}
	if err := setFlagDefaultToControl(ctx, tx, flagID, currentVersionID, callerID); err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
UPDATE
    experiments
SET
    status = 'paused',
    guardrail_paused = TRUE,
    version = version + 1,
    updated_by = $1
WHERE
    id = $2
    AND status = 'running'
    AND version = $3`, callerID, id, expectedVersion)
	if err != nil {
		return err
	}
	before := experimentAuditState{Status: status, Version: currentVersionNum, GuardrailPaused: guardrailPaused}
	after := experimentAuditState{Status: StatusPaused, Version: currentVersionNum + 1, GuardrailPaused: true}
	if err := appendExperimentStateAudit(ctx, tx, id, callerID, "experiment.guardrail_rollback", "", before, after); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func setFlagDefaultToControl(ctx context.Context, tx pgx.Tx, flagID string, versionID *string, callerID string) error {
	if versionID == nil {
		return ErrInvalidVariants
	}

	var controlValue string
	err := tx.QueryRow(ctx, `
SELECT
    value::text
FROM
    variants
WHERE
    version_id = $1
    AND is_control`, *versionID).Scan(&controlValue)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidVariants
	}
	if err != nil {
		return err
	}

	return updateFlagDefault(ctx, tx, flagID, controlValue, callerID)
}

func setFlagDefaultToWinner(ctx context.Context, tx pgx.Tx, flagID string, versionID *string, winnerVariantID string, callerID string) error {
	if versionID == nil {
		return ErrInvalidVariants
	}

	var winnerValue string
	err := tx.QueryRow(ctx, `
SELECT
    value::text
FROM
    variants
WHERE
    version_id = $1
    AND id = $2`, *versionID, winnerVariantID).Scan(&winnerValue)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrWinnerRequired
	}
	if err != nil {
		return err
	}

	return updateFlagDefault(ctx, tx, flagID, winnerValue, callerID)
}

func updateFlagDefault(ctx context.Context, tx pgx.Tx, flagID, value, callerID string) error {
	tag, err := tx.Exec(ctx, `
UPDATE
    flags
SET
    default_value = $1::jsonb,
    updated_by = $2
WHERE
    id = $3`, value, callerID, flagID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrFlagNotFound
	}
	return nil
}

func (r *Repository) UpdateDraft(ctx context.Context, id string, name, description string, expectedVersion int, callerID string) (Experiment, error) {
	var desc any
	if description != "" {
		desc = description
	}
	q := `
UPDATE
    experiments AS e
SET
    name = COALESCE(NULLIF($1, ''), e.name),
    description = COALESCE($2, e.description),
    version = e.version + 1,
    updated_by = $3
WHERE
    e.id = $4 AND e.status = 'draft' AND e.version = $5
RETURNING` + experimentColumns
	row := r.db.QueryRow(ctx, q, name, desc, callerID, id, expectedVersion)
	exp, err := scanExperiment(row)
	if err == nil {
		return exp, nil
	}
	if !errors.Is(err, ErrNotFound) {
		if database.IsUniqueViolation(err) {
			return Experiment{}, ErrConflictName
		}
		return Experiment{}, err
	}
	current, getErr := r.GetState(ctx, id)
	if getErr != nil {
		return Experiment{}, getErr
	}
	if current.Status != StatusDraft {
		return Experiment{}, ErrInvalidTransition
	}
	return Experiment{}, ErrVersionConflict
}

func (r *Repository) Complete(ctx context.Context, id string, from Status, expectedVersion int, callerID string, decision CompletionDecision, reason string, winnerVariantID *string) (Experiment, error) {
	switch decision {
	case DecisionRolloutWinner:
		return r.completeRolloutWinner(ctx, id, from, expectedVersion, callerID, reason, winnerVariantID)
	case DecisionRollback:
		return r.completeRollbackToControl(ctx, id, from, expectedVersion, callerID, reason)
	case DecisionNoEffect:
		return r.completeNoEffect(ctx, id, from, expectedVersion, callerID, reason)
	default:
		return Experiment{}, ErrInvalidTransition
	}
}

func (r *Repository) completeNoEffect(ctx context.Context, id string, from Status, expectedVersion int, callerID, reason string) (Experiment, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Experiment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var before experimentAuditState
	err = tx.QueryRow(ctx, `
SELECT
    status,
    version,
    guardrail_paused
FROM
    experiments
WHERE
    id = $1
FOR UPDATE`, id).Scan(&before.Status, &before.Version, &before.GuardrailPaused)
	if errors.Is(err, pgx.ErrNoRows) {
		return Experiment{}, ErrNotFound
	}
	if err != nil {
		return Experiment{}, err
	}
	if before.Status != from || (from != StatusRunning && from != StatusPaused) {
		return Experiment{}, ErrInvalidTransition
	}
	if before.Version != expectedVersion {
		return Experiment{}, ErrVersionConflict
	}

	result, err := scanExperiment(tx.QueryRow(ctx, `
UPDATE
    experiments AS e
SET
    status = 'completed',
    version = e.version + 1,
    completion_decision = 'no_effect',
    completion_reason = $1,
    updated_by = $2
WHERE
    e.id = $3
    AND e.status = $4
    AND e.version = $5
RETURNING`+experimentColumns, reason, callerID, id, string(from), expectedVersion))
	if err != nil {
		return Experiment{}, err
	}
	after := experimentAuditState{Status: StatusCompleted, Version: result.Version, GuardrailPaused: before.GuardrailPaused}
	if err := appendExperimentStateAudit(ctx, tx, id, callerID, "experiment.completed.no_effect", reason, before, after); err != nil {
		return Experiment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Experiment{}, err
	}
	return result, nil
}

func (r *Repository) completeRolloutWinner(ctx context.Context, id string, from Status, expectedVersion int, callerID, reason string, winnerVariantID *string) (Experiment, error) {
	if winnerVariantID == nil || *winnerVariantID == "" {
		return Experiment{}, ErrWinnerRequired
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Experiment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		status           Status
		flagID           string
		currentVersionID *string
		version          int
		guardrailPaused  bool
	)
	err = tx.QueryRow(ctx, `
SELECT
    status,
    flag_id::text,
    current_version_id::text,
	version,
	guardrail_paused
FROM
    experiments
WHERE
    id = $1
FOR UPDATE`, id).Scan(&status, &flagID, &currentVersionID, &version, &guardrailPaused)
	if errors.Is(err, pgx.ErrNoRows) {
		return Experiment{}, ErrNotFound
	}
	if err != nil {
		return Experiment{}, err
	}
	if status != from || (from != StatusRunning && from != StatusPaused) {
		return Experiment{}, ErrInvalidTransition
	}
	if version != expectedVersion {
		return Experiment{}, ErrVersionConflict
	}
	if err := setFlagDefaultToWinner(ctx, tx, flagID, currentVersionID, *winnerVariantID, callerID); err != nil {
		return Experiment{}, err
	}

	result, err := scanExperiment(tx.QueryRow(ctx, `
UPDATE
    experiments AS e
SET
    status = 'completed',
    version = e.version + 1,
    completion_decision = 'rollout_winner',
    completion_reason = $1,
    updated_by = $2
WHERE
    e.id = $3
    AND e.status = $4
    AND e.version = $5
RETURNING`+experimentColumns, reason, callerID, id, string(from), expectedVersion))
	if err != nil {
		return Experiment{}, err
	}
	before := experimentAuditState{Status: status, Version: version, GuardrailPaused: guardrailPaused}
	after := experimentAuditState{Status: StatusCompleted, Version: result.Version, GuardrailPaused: guardrailPaused}
	if err := appendExperimentStateAudit(ctx, tx, id, callerID, "experiment.completed.rollout_winner", reason, before, after); err != nil {
		return Experiment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Experiment{}, err
	}
	return result, nil
}

func (r *Repository) completeRollbackToControl(ctx context.Context, id string, from Status, expectedVersion int, callerID, reason string) (Experiment, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Experiment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		status           Status
		flagID           string
		currentVersionID *string
		version          int
		guardrailPaused  bool
	)
	err = tx.QueryRow(ctx, `
SELECT
    status,
    flag_id::text,
    current_version_id::text,
	version,
	guardrail_paused
FROM
    experiments
WHERE
    id = $1
FOR UPDATE`, id).Scan(&status, &flagID, &currentVersionID, &version, &guardrailPaused)
	if errors.Is(err, pgx.ErrNoRows) {
		return Experiment{}, ErrNotFound
	}
	if err != nil {
		return Experiment{}, err
	}
	if status != from || (from != StatusRunning && from != StatusPaused) {
		return Experiment{}, ErrInvalidTransition
	}
	if version != expectedVersion {
		return Experiment{}, ErrVersionConflict
	}
	if err := setFlagDefaultToControl(ctx, tx, flagID, currentVersionID, callerID); err != nil {
		return Experiment{}, err
	}

	result, err := scanExperiment(tx.QueryRow(ctx, `
UPDATE
    experiments AS e
SET
    status = 'completed',
    version = e.version + 1,
    completion_decision = 'rollback',
    completion_reason = $1,
    updated_by = $2
WHERE
    e.id = $3
    AND e.status = $4
    AND e.version = $5
RETURNING`+experimentColumns, reason, callerID, id, string(from), expectedVersion))
	if err != nil {
		return Experiment{}, err
	}
	before := experimentAuditState{Status: status, Version: version, GuardrailPaused: guardrailPaused}
	after := experimentAuditState{Status: StatusCompleted, Version: result.Version, GuardrailPaused: guardrailPaused}
	if err := appendExperimentStateAudit(ctx, tx, id, callerID, "experiment.completed.rollback", reason, before, after); err != nil {
		return Experiment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Experiment{}, err
	}
	return result, nil
}

func (r *Repository) ListVariants(ctx context.Context, versionID string) ([]Variant, error) {
	return r.listVariants(ctx, versionID)
}

type RunningExperiment struct {
	Experiment Experiment
	FlagKey    string
	Version    ExperimentVersion
	Variants   []Variant
}

func (r *Repository) ListRunning(ctx context.Context) ([]RunningExperiment, error) {
	const q = `
SELECT
    f.key,` + experimentColumns + `
FROM
    experiments e
    JOIN flags f ON e.flag_id = f.id
WHERE
    e.status = 'running'
ORDER BY
    f.key, e.id`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RunningExperiment
	for rows.Next() {
		var (
			re       RunningExperiment
			desc     *string
			curVerID *string
			compDec  *string
			compRea  *string
		)
		if err := rows.Scan(
			&re.FlagKey,
			&re.Experiment.ID,
			&re.Experiment.FlagID,
			&re.Experiment.Name,
			&desc,
			&re.Experiment.Status,
			&curVerID,
			&re.Experiment.OwnerID,
			&re.Experiment.Version,
			&re.Experiment.GuardrailPaused,
			&compDec,
			&compRea,
			&re.Experiment.CreatedBy,
			&re.Experiment.UpdatedBy,
			&re.Experiment.CreatedAt,
			&re.Experiment.UpdatedAt,
		); err != nil {
			return nil, err
		}
		re.Experiment.Description = desc
		re.Experiment.CurrentVersionID = curVerID
		if compDec != nil {
			re.Experiment.CompletionDecision = (*CompletionDecision)(compDec)
		}
		re.Experiment.CompletionReason = compRea
		if curVerID == nil {
			continue
		}
		ver, err := r.getVersion(ctx, *curVerID)
		if err != nil {
			return nil, err
		}
		re.Version = ver
		re.Variants, err = r.listVariants(ctx, ver.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, re)
	}
	return out, rows.Err()
}

func (r *Repository) CountActiveByFlag(ctx context.Context, flagID, excludeID string) (int64, error) {
	const q = `
SELECT
    count(*)
FROM
    experiments
WHERE
    flag_id = $1 AND status IN ('running', 'paused') AND id <> $2`
	var n int64
	err := r.db.QueryRow(ctx, q, flagID, excludeID).Scan(&n)
	return n, err
}

func (r *Repository) List(ctx context.Context, limit, offset int, filter ListFilter) ([]Experiment, error) {
	q := `SELECT ` + experimentColumns + ` FROM experiments e`
	where, args := experimentWhere(filter)
	q += where
	q += ` ORDER BY e.created_at DESC LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Experiment
	for rows.Next() {
		exp, err := scanExperiment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, exp)
	}
	return out, rows.Err()
}

func (r *Repository) Count(ctx context.Context, filter ListFilter) (int64, error) {
	q := `SELECT count(*) FROM experiments e`
	where, args := experimentWhere(filter)
	q += where
	var n int64
	err := r.db.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}

func experimentWhere(filter ListFilter) (string, []any) {
	clauses := make([]string, 0, 3)
	args := []any{}
	if filter.Status != "" {
		args = append(args, string(filter.Status))
		clauses = append(clauses, `e.status = $`+strconv.Itoa(len(args)))
	}
	if filter.FlagID != nil {
		args = append(args, *filter.FlagID)
		clauses = append(clauses, `e.flag_id = $`+strconv.Itoa(len(args)))
	}
	if filter.Search != nil {
		args = append(args, "%"+*filter.Search+"%")
		clauses = append(clauses, `e.name ILIKE $`+strconv.Itoa(len(args)))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}
