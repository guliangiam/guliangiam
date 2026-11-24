package iga

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type CreateAccessRequestCommand struct {
	OrganizationID, RequesterID  string
	RoleID, ApplicationID        string
	EnforcerID, Resource, Action string
	Reason                       string
	StartsAt, ExpiresAt          time.Time
}

type AccessRequestActionCommand struct {
	OrganizationID, RequestID, ActorID string
	Revision                           int64
	Comment                            string
}

type AccessRequestDTO struct {
	ID                string    `json:"id"`
	OrganizationID    string    `json:"organizationId"`
	RequesterID       string    `json:"requesterId"`
	RoleID            string    `json:"roleId"`
	ApplicationID     string    `json:"applicationId"`
	EnforcerID        string    `json:"enforcerId"`
	Resource          string    `json:"resource"`
	Action            string    `json:"action"`
	Reason            string    `json:"reason"`
	Status            string    `json:"status"`
	StartsAt          time.Time `json:"startsAt"`
	ExpiresAt         time.Time `json:"expiresAt"`
	ManagerApproverID string    `json:"managerApproverId"`
	SAPApproverID     string    `json:"sapApproverId"`
	LifecycleID       string    `json:"lifecycleId"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	Revision          int64     `json:"revision"`
}

type TicketMessageDTO struct {
	ID        string    `json:"id"`
	TicketID  string    `json:"ticketId"`
	AuthorID  string    `json:"authorId"`
	Action    string    `json:"action"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type AccessRequestDetail struct {
	Request       AccessRequestDTO   `json:"request"`
	Messages      []TicketMessageDTO `json:"messages"`
	ApprovalChain []string           `json:"approvalChain"`
}

func toAccessRequestDTO(item *domain.AccessRequest) AccessRequestDTO {
	return AccessRequestDTO{
		ID: item.ID, OrganizationID: item.OrganizationID, RequesterID: item.RequesterID,
		RoleID: item.RoleID, ApplicationID: item.ApplicationID, EnforcerID: item.EnforcerID,
		Resource: item.Resource, Action: item.Action, Reason: item.Reason, Status: string(item.Status),
		StartsAt: item.StartsAt, ExpiresAt: item.ExpiresAt,
		ManagerApproverID: item.ManagerApproverID, SAPApproverID: item.SAPApproverID,
		LifecycleID: item.LifecycleID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, Revision: item.Revision,
	}
}

func (s Service) ListAccessRequests(ctx context.Context, q domain.ListAccessRequestsQuery) ([]AccessRequestDTO, int64, error) {
	if q.OrganizationID == "" {
		return nil, 0, apperrors.NewInvalidArgument("organizationId required")
	}
	var items []*domain.AccessRequest
	var total int64
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var err error
		items, total, err = tx.IGA().ListAccessRequests(ctx, q)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]AccessRequestDTO, len(items))
	for i, item := range items {
		out[i] = toAccessRequestDTO(item)
	}
	return out, total, nil
}

func (s Service) GetAccessRequest(ctx context.Context, orgID, id string) (*AccessRequestDetail, error) {
	if orgID == "" || id == "" {
		return nil, apperrors.NewInvalidArgument("organizationId and id required")
	}
	var item *domain.AccessRequest
	var messages []*domain.TicketMessage
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var err error
		item, err = tx.IGA().GetAccessRequest(ctx, orgID, id, false)
		if err != nil {
			return err
		}
		messages, err = tx.IGA().ListTicketMessages(ctx, orgID, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	detail := &AccessRequestDetail{Request: toAccessRequestDTO(item), Messages: make([]TicketMessageDTO, len(messages)), ApprovalChain: []string{approverRoleManager, approverRoleSAP}}
	for i, message := range messages {
		detail.Messages[i] = TicketMessageDTO{ID: message.ID, TicketID: message.TicketID, AuthorID: message.AuthorID, Action: message.Action, Content: message.Content, CreatedAt: message.CreatedAt}
	}
	return detail, nil
}

func (s Service) CreateAccessRequest(ctx context.Context, cmd CreateAccessRequestCommand) (*AccessRequestDTO, error) {
	if cmd.OrganizationID == "" || cmd.RequesterID == "" || cmd.RoleID == "" || cmd.ApplicationID == "" || cmd.Resource == "" || cmd.Action == "" {
		return nil, apperrors.NewInvalidArgument("organization, requester, role, application, resource and action are required")
	}
	if cmd.StartsAt.IsZero() || !cmd.ExpiresAt.After(cmd.StartsAt) {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "a valid startsAt/expiresAt window is required", map[string]any{"field": "startsAt", "expected": "before expiresAt"})
	}
	if !cmd.ExpiresAt.After(s.now()) {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "expiresAt must be in the future", map[string]any{"field": "expiresAt", "value": cmd.ExpiresAt.UTC().Format(time.RFC3339), "expected": "future"})
	}
	id, err := newIGAIdentifier()
	if err != nil {
		return nil, err
	}
	now := s.now()
	item := &domain.AccessRequest{
		ID: id, OrganizationID: cmd.OrganizationID, RequesterID: cmd.RequesterID,
		RoleID: cmd.RoleID, ApplicationID: cmd.ApplicationID, EnforcerID: cmd.EnforcerID,
		Resource: cmd.Resource, Action: cmd.Action, Reason: cmd.Reason,
		Status: domain.RequestSubmitted, StartsAt: cmd.StartsAt, ExpiresAt: cmd.ExpiresAt,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if item.EnforcerID == "" {
			item.EnforcerID, err = tx.IGA().ResolveDefaultEnforcer(ctx, item.OrganizationID)
			if err != nil {
				return err
			}
		}
		validity, err := tx.IGA().ValidateAccessTargets(ctx, item.OrganizationID, item.RequesterID, item.RoleID, item.ApplicationID, item.EnforcerID)
		if err != nil {
			return err
		}
		if !validity.Valid() || validity.Subject == "" {
			return apperrors.NewForbidden("invalid access request target")
		}
		checker := SODChecker{UOW: tx, Clock: s.Clock}
		if err := checker.ValidateConflict(ctx, item.OrganizationID, item.RoleID, item.RequesterID); err != nil {
			return apperrors.NewForbidden(err.Error())
		}
		if err := tx.IGA().CreateAccessRequest(ctx, item); err != nil {
			return err
		}
		return s.recordRequestEvent(ctx, tx, item, item.RequesterID, "submitted", item.Reason, "iga.ticket.submitted", map[string]any{keyTicketID: item.ID, "roleId": item.RoleID, keyStartsAt: item.StartsAt, keyExpiresAt: item.ExpiresAt})
	})
	if err != nil {
		return nil, err
	}
	dto := toAccessRequestDTO(item)
	return &dto, nil
}

func (s Service) SubmitAccessRequest(ctx context.Context, cmd AccessRequestActionCommand) (*AccessRequestDTO, error) {
	return s.transitionAccessRequest(ctx, cmd, domain.RequestSubmitted, domain.RequestPendingManager, "submitted_for_approval", "iga.ticket.pending_manager", func(item *domain.AccessRequest) error {
		if item.RequesterID != cmd.ActorID {
			return apperrors.NewForbidden("only the requester can submit the request")
		}
		return nil
	})
}

func (s Service) ApproveAccessRequest(ctx context.Context, stage string, cmd AccessRequestActionCommand) (*AccessRequestDTO, error) {
	var expected domain.RequestStatus
	switch strings.ToLower(stage) {
	case approverRoleManager:
		expected = domain.RequestPendingManager
	case approverRoleSAP:
		expected = domain.RequestPendingSAP
	default:
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "approval stage must be manager or sap", map[string]any{"field": "stage", "value": strings.ToLower(stage), "allowed": append([]string{}, approvalStages...)})
	}
	var result *AccessRequestDTO
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		item, err := s.lockRequest(ctx, tx, cmd, expected)
		if err != nil {
			return err
		}
		if item.RequesterID == cmd.ActorID {
			return apperrors.NewForbidden("requester cannot approve their own request")
		}
		if expected == domain.RequestPendingSAP && item.ManagerApproverID == cmd.ActorID {
			return apperrors.NewForbidden("manager and SAP approvers must differ")
		}
		if expected == domain.RequestPendingManager {
			item.ManagerApproverID = cmd.ActorID
		} else {
			item.SAPApproverID = cmd.ActorID
		}
		if err := s.transitionRequest(ctx, tx, item, expected, domain.RequestApproved, cmd.ActorID, "approved_"+strings.ToLower(stage), cmd.Comment, "iga.ticket.approved", map[string]any{keyTicketID: item.ID, "stage": strings.ToLower(stage), "approverId": cmd.ActorID}); err != nil {
			return err
		}
		if expected == domain.RequestPendingManager {
			if err := s.transitionRequest(ctx, tx, item, domain.RequestApproved, domain.RequestPendingSAP, cmd.ActorID, "pending_sap", "", "iga.ticket.pending_sap", map[string]any{keyTicketID: item.ID}); err != nil {
				return err
			}
		} else if err := s.provisionRequest(ctx, tx, item, cmd.ActorID); err != nil {
			return err
		}
		dto := toAccessRequestDTO(item)
		result = &dto
		return nil
	})
	return result, err
}

func (s Service) ProvisionAccessRequest(ctx context.Context, cmd AccessRequestActionCommand) (*AccessRequestDTO, error) {
	var result *AccessRequestDTO
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		item, err := s.lockRequest(ctx, tx, cmd, domain.RequestApproved)
		if err != nil {
			return err
		}
		if item.SAPApproverID == "" || item.SAPApproverID != cmd.ActorID {
			return apperrors.NewForbidden("only the SAP approver can provision the request")
		}
		if err := s.provisionRequest(ctx, tx, item, cmd.ActorID); err != nil {
			return err
		}
		dto := toAccessRequestDTO(item)
		result = &dto
		return nil
	})
	return result, err
}

func (s Service) RejectAccessRequest(ctx context.Context, stage string, cmd AccessRequestActionCommand) (*AccessRequestDTO, error) {
	var expected domain.RequestStatus
	switch strings.ToLower(stage) {
	case approverRoleManager:
		expected = domain.RequestPendingManager
	case approverRoleSAP:
		expected = domain.RequestPendingSAP
	default:
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "approval stage must be manager or sap", map[string]any{"field": "stage", "value": strings.ToLower(stage), "allowed": append([]string{}, approvalStages...)})
	}
	return s.transitionAccessRequest(ctx, cmd, expected, domain.RequestRejected, "rejected", "iga.ticket.rejected", func(item *domain.AccessRequest) error {
		if item.RequesterID == cmd.ActorID {
			return apperrors.NewForbidden("requester cannot reject as an approver")
		}
		if expected == domain.RequestPendingSAP && item.ManagerApproverID == cmd.ActorID {
			return apperrors.NewForbidden("manager and SAP approvers must differ")
		}
		return nil
	})
}

func (s Service) WithdrawAccessRequest(ctx context.Context, cmd AccessRequestActionCommand) (*AccessRequestDTO, error) {
	item, err := s.UOW.IGA().GetAccessRequest(ctx, cmd.OrganizationID, cmd.RequestID, false)
	if err != nil {
		return nil, err
	}
	if item.Status != domain.RequestSubmitted && item.Status != domain.RequestPendingManager && item.Status != domain.RequestPendingSAP {
		return nil, apperrors.NewConflict("request cannot be withdrawn in its current state")
	}
	return s.transitionAccessRequest(ctx, cmd, item.Status, domain.RequestWithdrawn, "withdrawn", "iga.ticket.withdrawn", func(item *domain.AccessRequest) error {
		if item.RequesterID != cmd.ActorID {
			return apperrors.NewForbidden("only the requester can withdraw the request")
		}
		return nil
	})
}

func (s Service) transitionAccessRequest(ctx context.Context, cmd AccessRequestActionCommand, expected, next domain.RequestStatus, action, eventType string, authorize func(*domain.AccessRequest) error) (*AccessRequestDTO, error) {
	var result *AccessRequestDTO
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		item, err := s.lockRequest(ctx, tx, cmd, expected)
		if err != nil {
			return err
		}
		if err := authorize(item); err != nil {
			return err
		}
		if err := s.transitionRequest(ctx, tx, item, expected, next, cmd.ActorID, action, cmd.Comment, eventType, map[string]any{keyTicketID: item.ID, "from": expected, "to": next}); err != nil {
			return err
		}
		dto := toAccessRequestDTO(item)
		result = &dto
		return nil
	})
	return result, err
}

func (s Service) lockRequest(ctx context.Context, tx appuow.UnitOfWork, cmd AccessRequestActionCommand, expected domain.RequestStatus) (*domain.AccessRequest, error) {
	if cmd.OrganizationID == "" || cmd.RequestID == "" || cmd.ActorID == "" || cmd.Revision <= 0 {
		return nil, apperrors.NewInvalidArgument("organization, request, actor and positive revision are required")
	}
	item, err := tx.IGA().GetAccessRequest(ctx, cmd.OrganizationID, cmd.RequestID, true)
	if err != nil {
		return nil, err
	}
	if item.Revision != cmd.Revision {
		return nil, apperrors.NewConflict("access request revision conflict")
	}
	if item.Status != expected {
		return nil, apperrors.NewConflict("access request is not in the required state")
	}
	return item, nil
}

func (s Service) transitionRequest(ctx context.Context, tx appuow.UnitOfWork, item *domain.AccessRequest, expected, next domain.RequestStatus, actorID, action, comment, eventType string, payload map[string]any) error {
	item.Status = next
	item.UpdatedAt = s.now()
	updated, err := tx.IGA().UpdateAccessRequest(ctx, item, expected)
	if err != nil {
		return err
	}
	if !updated {
		return apperrors.NewConflict("access request transition conflict")
	}
	return s.recordRequestEvent(ctx, tx, item, actorID, action, comment, eventType, payload)
}

func (s Service) recordRequestEvent(ctx context.Context, tx appuow.UnitOfWork, item *domain.AccessRequest, actorID, action, content, eventType string, payload map[string]any) error {
	messageID, err := newIGAIdentifier()
	if err != nil {
		return err
	}
	if err := tx.IGA().AddTicketMessage(ctx, &domain.TicketMessage{ID: messageID, TicketID: item.ID, AuthorID: actorID, Action: action, Content: content, CreatedAt: s.now()}); err != nil {
		return err
	}
	return s.embedAuditEvent(
		tx,
		ctx,
		item.OrganizationID,
		actorID,
		eventType,
		"access_request",
		item.ID,
		"access_request",
		eventType,
		payload,
	)
}

func (s Service) provisionRequest(ctx context.Context, tx appuow.UnitOfWork, item *domain.AccessRequest, actorID string) error {
	if item.Status != domain.RequestApproved {
		return apperrors.NewConflict("only approved requests can be provisioned")
	}
	now := s.now()
	if !item.ExpiresAt.After(now) {
		return apperrors.NewConflict("access request window has expired")
	}
	validity, err := tx.IGA().ValidateAccessTargets(ctx, item.OrganizationID, item.RequesterID, item.RoleID, item.ApplicationID, item.EnforcerID)
	if err != nil {
		return err
	}
	if !validity.Valid() || validity.Subject == "" {
		return apperrors.NewForbidden("access target is no longer valid")
	}
	lifecycleID, err := newIGAIdentifier()
	if err != nil {
		return err
	}
	lifecycle := &domain.AccessLifecycle{
		ID: lifecycleID, OrganizationID: item.OrganizationID, UserID: item.RequesterID,
		RoleID: item.RoleID, TicketID: item.ID, EnforcerID: item.EnforcerID,
		Resource: item.Resource, Action: item.Action, GrantedAt: now,
		StartsAt: item.StartsAt, ExpiresAt: item.ExpiresAt, Status: domain.LifecycleActive,
		CreatedAt: now, Revision: 1,
	}
	if err := createJITGrant(ctx, tx, s.Clock, lifecycle, validity.Subject, validity.OrganizationSlug); err != nil {
		return err
	}
	item.LifecycleID = lifecycleID
	return s.transitionRequest(ctx, tx, item, domain.RequestApproved, domain.RequestProvisioned, actorID, "provisioned", "", "iga.jit.granted", map[string]any{keyTicketID: item.ID, keyLifecycleID: lifecycleID, keyPolicyID: lifecycle.PolicyID, keyPrivileged: true, keyStartsAt: item.StartsAt, keyExpiresAt: item.ExpiresAt})
}
