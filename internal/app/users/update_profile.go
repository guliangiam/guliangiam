package users

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

type UpdateUserProfileCommand struct {
	OrganizationID        string
	UserID                string
	DisplayName           string
	FirstName             string
	LastName              string
	AvatarURL             string
	Email                 string
	Phone                 string
	CountryCode           string
	Region                string
	Location              string
	Birthday              string
	Gender                string
	Bio                   string
	RealName              string
	IDCardType            string
	IDCard                string
	PassportNo            string
	DriverLicenseNo       string
	IDCardExpiry          string
	IssueCountry          string
	IssuingAuthority      string
	PassportPhotoURL      string
	DriverLicenseFrontURL string
	DriverLicenseBackURL  string
	Properties            map[string]any
}

// normalizeProfileProperties 统一空值语义：PUT /profile 是全量替换，
// null 与空串都表示删除该键，避免空串穿透「必填/存在性」判定。
func normalizeProfileProperties(properties map[string]any) map[string]any {
	if properties == nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(properties))
	for key, value := range properties {
		if value == nil {
			continue
		}
		if text, ok := value.(string); ok && text == "" {
			continue
		}
		result[key] = value
	}
	return result
}

func (s *Service) UpdateUserProfile(ctx context.Context, command UpdateUserProfileCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if err := validateUpdateUserProfileCommand(command); err != nil {
		return err
	}
	command.Properties = normalizeProfileProperties(command.Properties)

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Resolve ownership first: a foreign user must leave profile untouched.
		user, err := tx.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID)
		if err != nil {
			return err
		}

		// 证件号永不返回原文（规范 9.5），客户端无法回传存储值：PUT 全量
		// 替换中空值 = 保持存储值（与凭据族「输入留空保持当前值」同口径）。
		// 代价是证件号不能再通过空串清除，清除需走独立的受控流程。
		if command.IDCard == "" {
			command.IDCard = user.Profile.IDCard
		}
		if command.PassportNo == "" {
			command.PassportNo = user.Profile.PassportNo
		}
		if command.DriverLicenseNo == "" {
			command.DriverLicenseNo = user.Profile.DriverLicenseNo
		}

		// 规范 9.3：已验证用户的实名字段后端锁定。
		if err := ensureVerifiedProfileFieldsLocked(user, command.RealName, command.IDCardType, command.IDCard, command.Properties); err != nil {
			return err
		}

		if err := s.validateCustomAttributeProperties(ctx, tx, command.OrganizationID, command.Properties); err != nil {
			return err
		}

		// 联系方式一旦变更，原验证结果立即失效：管理员或用户本人都必须先
		// 重新走验证码流程，不能把旧地址的验证状态带到新地址。
		emailVerified := user.Profile.EmailVerified
		if !strings.EqualFold(strings.TrimSpace(command.Email), strings.TrimSpace(user.Profile.Email)) {
			emailVerified = false
		}
		phoneVerified := user.Profile.PhoneVerified
		if strings.TrimSpace(command.Phone) != strings.TrimSpace(user.Profile.Phone) {
			phoneVerified = false
		}

		profile := userdomain.UserProfile{
			UserID:                command.UserID,
			DisplayName:           command.DisplayName,
			FirstName:             command.FirstName,
			LastName:              command.LastName,
			AvatarURL:             command.AvatarURL,
			Email:                 command.Email,
			EmailVerified:         emailVerified,
			Phone:                 command.Phone,
			PhoneVerified:         phoneVerified,
			CountryCode:           command.CountryCode,
			Region:                command.Region,
			Location:              command.Location,
			Birthday:              command.Birthday,
			Gender:                command.Gender,
			Bio:                   command.Bio,
			RealName:              command.RealName,
			IDCardType:            command.IDCardType,
			IDCard:                command.IDCard,
			PassportNo:            command.PassportNo,
			DriverLicenseNo:       command.DriverLicenseNo,
			IDCardExpiry:          command.IDCardExpiry,
			IssueCountry:          command.IssueCountry,
			IssuingAuthority:      command.IssuingAuthority,
			PassportPhotoURL:      command.PassportPhotoURL,
			DriverLicenseFrontURL: command.DriverLicenseFrontURL,
			DriverLicenseBackURL:  command.DriverLicenseBackURL,
			Properties:            command.Properties,
		}

		if v, ok := command.Properties[profileKeyAffiliation]; ok {
			if s, ok := v.(string); ok {
				profile.Affiliation = s
			}
		}

		user.Profile = profile
		user.UpdatedAt = s.Clock.Now()
		user.Revision++

		if err := tx.Users().UpdateForOrganization(ctx, user, command.OrganizationID); err != nil {
			return err
		}

		if err := tx.Users().UpdateProfile(ctx, &profile); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			"user.updated",
			"User",
			user.ID,
			"user",
			"user.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
