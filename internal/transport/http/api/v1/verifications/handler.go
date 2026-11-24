package verifications

import (
	"context"

	appverifications "github.com/guliangiam/guliangiam/internal/app/verifications"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appverifications.Service
}

func (h Handler) List(ctx context.Context, request ListVerificationsRequest) (ListVerificationsResponse, error) {
	items, total, err := h.Service.ListVerifications(ctx, appverifications.ListVerificationsQuery(request))
	if err != nil {
		return ListVerificationsResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*VerificationResponse, error) {
	verification, err := h.Service.GetVerification(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	return toResponse(verification), nil
}

func (h Handler) SendVerificationCode(ctx context.Context, request SendVerificationCodeRequest) (*SendVerificationCodeResponse, error) {
	result, err := h.Service.SendVerification(ctx, appverifications.SendVerificationCommand{
		OrganizationID: request.OrganizationID,
		ApplicationID:  request.ApplicationID,
		Type:           request.Type,
		Recipient:      request.Recipient,
		CaptchaToken:   request.CaptchaToken,
		CaptchaID:      request.CaptchaID,
		ClientIP:       request.ClientIP,
	})
	if err != nil {
		return nil, err
	}

	return &SendVerificationCodeResponse{
		ID:        result.ID,
		Type:      result.Type,
		Recipient: result.Recipient,
		ExpiresAt: result.ExpiresAt,
	}, nil
}

func (h Handler) VerifyCode(ctx context.Context, request VerifyCodeRequest) error {
	return h.Service.VerifyCode(ctx, appverifications.VerifyCodeCommand{
		Type:           request.Type,
		Recipient:      request.Recipient,
		Code:           request.Code,
		ClientIP:       request.ClientIP,
		OrganizationID: request.OrganizationID,
		ApplicationID:  request.ApplicationID,
	})
}
