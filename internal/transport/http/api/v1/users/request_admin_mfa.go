package users

import appusers "github.com/guliangiam/guliangiam/internal/app/users"

// AdminRebindTOTPRequest is the second-confirmation body for a platform
// administrator TOTP reset. It deliberately carries no organizationId: the
// target tenant is resolved from the user row, not from client input.
type AdminRebindTOTPRequest struct {
	Confirm bool   `json:"confirm"`
	Reason  string `json:"reason"`
}

func (r AdminRebindTOTPRequest) toCommand(actorID, actorPrincipalType, targetUserID string) appusers.AdminRebindTOTPCommand {
	return appusers.AdminRebindTOTPCommand{
		ActorID:            actorID,
		ActorPrincipalType: actorPrincipalType,
		TargetUserID:       targetUserID,
		Reason:             r.Reason,
		Confirm:            r.Confirm,
	}
}
