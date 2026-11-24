package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DispatchResult aggregates the outcome of a single sync extension-point
// invocation. ADR-029 semantics: the dispatcher runs every active action
// subscribed to the trigger in declaration order; the first action that
// emits a non-nil business error ("deny") short-circuits the rest and
// Allow=false.
type DispatchResult struct {
	// Allow is true when every participating action returned a nil
	// business error. This is the value the caller feeds back to the
	// decision plane (allow login / allow token issuance / ...).
	Allow bool

	// Reasons collects the user-facing deny reason string (one per
	// denying action). Empty when Allow=true.
	Reasons []string

	// ActionIDs lists every action that was consulted, in execution
	// order — useful for auditing and for correlating with execution_log.
	ActionIDs []string

	// Elapsed is wall-clock time spent dispatching the entire trigger.
	Elapsed time.Duration
}

// actionsUoW is the narrow UnitOfWork sub-interface that Dispatcher
// actually needs. A production UnitOfWork (see internal/app/uow/uow.go)
// already satisfies this by virtue of embedding ActionsUoW + exposing
// WithinTransaction. Defining it separately lets tests inject a fake
// without re-implementing 60+ unrelated nested-repository methods.
type actionsUoW interface {
	Actions() actionsRepository
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx actionsUoW) error) error
}

// actionsRepository is the sub-intersection of domain/actions.Repository
// that the Dispatcher uses. Declared here so the app layer doesn't need
// a full Repository stub with 10+ methods in its tests. All argument
// types resolve to the app-level aliases declared in service.go.
type actionsRepository interface {
	List(ctx context.Context, query ListActionsQuery) ([]*Action, int64, error)
	GetLatestVersion(ctx context.Context, actionID, organizationID string) (*ActionVersion, error)
	CreateExecutionLog(ctx context.Context, log *ExecutionLog) error
	UpdateExecutionLog(ctx context.Context, cmd UpdateExecutionLogCommand) error
}

// Dispatcher wires a Service, an Executor, and a narrow UoW together to
// drive a single sync extension point (pre_login / pre_token / ...). It
// owns execution-log lifecycle: pending → running → success/failed/timeout.
type Dispatcher struct {
	Service  Service
	Executor Executor
	UOW      actionsUoW

	// DefaultExecOpts supplies the resource limits applied to each action
	// invocation. NormalizeExecOptions is applied per run, so partial
	// fills are fine.
	DefaultExecOpts ExecOptions
}

// NewDispatcher wires up Service + Executor + UoW. Uses the Service's
// embedded UnitOfWork if uow is nil. Executor must be non-nil — pass
// NoopExecutor to preserve existing Run semantics in tests.
//
// The third argument is typed actionsUoW so production code can pass the
// full UnitOfWork (it implements the narrow interface), and tests can
// pass a minimal fake.
func NewDispatcher(svc Service, exec Executor, uow actionsUoW) *Dispatcher {
	if exec == nil {
		exec = NoopExecutor{}
	}
	if uow == nil {
		uow = unitOfWorkAdapter{svc.UOW}
	}
	return &Dispatcher{Service: svc, Executor: exec, UOW: uow}
}

// unitOfWorkAdapter bridges the app-level UnitOfWork (which declares
// WithinTransaction with UnitOfWork as its tx argument) to the narrower
// actionsUoW interface. The adapter re-enters itself as the tx so nested
// calls see the same underlying repos.
type unitOfWorkAdapter struct {
	inner ActionsUoW
}

func (a unitOfWorkAdapter) Actions() actionsRepository { return a.inner.Actions() }
func (a unitOfWorkAdapter) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx actionsUoW) error) error {
	return a.inner.WithinTransaction(ctx, func(ctx context.Context, tx UnitOfWork) error {
		return fn(ctx, unitOfWorkAdapter{tx})
	})
}

// DispatchPreLogin dispatches pre_login sync extension point.
func (d *Dispatcher) DispatchPreLogin(ctx context.Context, orgID string, input json.RawMessage) (*DispatchResult, error) {
	return d.dispatch(ctx, orgID, TriggerPreLogin, input)
}

// DispatchPreToken dispatches pre_token sync extension point.
func (d *Dispatcher) DispatchPreToken(ctx context.Context, orgID string, input json.RawMessage) (*DispatchResult, error) {
	return d.dispatch(ctx, orgID, TriggerPreToken, input)
}

// DispatchPostRegistration dispatches post_registration sync extension point.
func (d *Dispatcher) DispatchPostRegistration(ctx context.Context, orgID string, input json.RawMessage) (*DispatchResult, error) {
	return d.dispatch(ctx, orgID, TriggerPostRegistration, input)
}

// DispatchConsent dispatches consent sync extension point.
func (d *Dispatcher) DispatchConsent(ctx context.Context, orgID string, input json.RawMessage) (*DispatchResult, error) {
	return d.dispatch(ctx, orgID, TriggerConsent, input)
}

// DispatchMFA dispatches mfa sync extension point.
func (d *Dispatcher) DispatchMFA(ctx context.Context, orgID string, input json.RawMessage) (*DispatchResult, error) {
	return d.dispatch(ctx, orgID, TriggerMFA, input)
}

// dispatch is the shared execution pipeline for all 5 triggers.
// Steps:
//  1. resolve active actions subscribing to the trigger
//  2. for each action, fetch latest code version
//  3. create pending execution_log
//  4. transition log to running, invoke Executor with per-action ExecOptions
//  5. on short-circuit (business deny) or completion, finalise log state
func (d *Dispatcher) dispatch(ctx context.Context, orgID string, trigger TriggerType, input json.RawMessage) (*DispatchResult, error) {
	if orgID == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "organizationId is required")
	}

	start := time.Now()
	result := &DispatchResult{Allow: true}

	// 1. resolve active actions for this trigger
	status := ActionStatusActive
	actions, _, err := d.UOW.Actions().List(ctx, ListActionsQuery{
		OrganizationID: orgID,
		TriggerType:    &trigger,
		Status:         &status,
		PageSize:       100,
	})
	if err != nil {
		return nil, apperrors.WrapInternal(err, "list actions")
	}

	for _, act := range actions {
		result.ActionIDs = append(result.ActionIDs, act.ID)

		// 2. latest code version
		version, err := d.UOW.Actions().GetLatestVersion(ctx, act.ID, orgID)
		if err != nil {
			if ferr := d.recordFailure(ctx, orgID, act, input, err); ferr != nil {
				return nil, ferr
			}
			result.Allow = false
			result.Reasons = append(result.Reasons, fmt.Sprintf("%s: %v", act.Name, err))
			break
		}

		// 3. create pending execution_log
		logID, err := d.Service.NewUUID()
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "generate execution_log id for %s", act.ID)
		}
		log := &ExecutionLog{
			ID:        logID,
			ActionID:  act.ID,
			Version:   version.Version,
			Status:    ExecStatusPending,
			Input:     input,
			CreatedAt: d.Service.Now(),
		}
		if err := d.UOW.Actions().CreateExecutionLog(ctx, log); err != nil {
			return nil, apperrors.WrapInternalf(err, "create execution_log for %s", act.ID)
		}

		// 4. transition to running (best-effort)
		d.markRunning(ctx, orgID, logID)

		// 5. execute with per-action ctx timeout
		opts := NormalizeExecOptions(d.DefaultExecOpts)
		runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		output, execErr, runErr := d.Executor.Execute(runCtx, version.Code, input, opts)
		ctxErr := runCtx.Err()
		cancel()

		if runErr != nil {
			status := ExecStatusFailed
			if ctxErr != nil {
				status = ExecStatusTimeout
			}
			d.finaliseLog(ctx, orgID, logID, status, output, runErr)
			result.Allow = false
			result.Reasons = append(result.Reasons, fmt.Sprintf("%s: sandbox %s", act.Name, status))
			break
		}
		if execErr != nil {
			d.finaliseLog(ctx, orgID, logID, ExecStatusFailed, output, execErr)
			result.Allow = false
			result.Reasons = append(result.Reasons, fmt.Sprintf("%s: %s", act.Name, strings.TrimSpace(execErr.Error())))
			break
		}
		d.finaliseLog(ctx, orgID, logID, ExecStatusSuccess, output, nil)
	}

	result.Elapsed = time.Since(start)
	return result, nil
}

func (d *Dispatcher) recordFailure(ctx context.Context, orgID string, act *Action, input json.RawMessage, err error) error {
	logID, idErr := d.Service.NewUUID()
	if idErr != nil {
		return apperrors.WrapInternalf(idErr, "generate execution_log id for %s", act.ID)
	}
	log := &ExecutionLog{
		ID:        logID,
		ActionID:  act.ID,
		Status:    ExecStatusFailed,
		Input:     input,
		CreatedAt: d.Service.Now(),
	}
	errStr := err.Error()
	return d.UOW.WithinTransaction(ctx, func(ctx context.Context, tx actionsUoW) error {
		if cerr := tx.Actions().CreateExecutionLog(ctx, log); cerr != nil {
			return cerr
		}
		return tx.Actions().UpdateExecutionLog(ctx, UpdateExecutionLogCommand{
			OrganizationID: orgID,
			LogID:          log.ID,
			Status:         ExecStatusFailed,
			Error:          &errStr,
		})
	})
}

func (d *Dispatcher) markRunning(ctx context.Context, orgID, logID string) {
	_ = d.UOW.Actions().UpdateExecutionLog(ctx, UpdateExecutionLogCommand{
		OrganizationID: orgID,
		LogID:          logID,
		Status:         ExecStatusRunning,
	})
}

func (d *Dispatcher) finaliseLog(ctx context.Context, orgID, logID string, status ExecutionStatus, output []byte, runErr error) {
	cmd := UpdateExecutionLogCommand{
		OrganizationID: orgID,
		LogID:          logID,
		Status:         status,
		Output:         json.RawMessage(output),
	}
	if runErr != nil {
		e := runErr.Error()
		cmd.Error = &e
	}
	_ = d.UOW.WithinTransaction(ctx, func(ctx context.Context, tx actionsUoW) error {
		return tx.Actions().UpdateExecutionLog(ctx, cmd)
	})
}
