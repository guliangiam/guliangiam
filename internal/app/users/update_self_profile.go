package users

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

// UpdateSelfProfileCommand 是会话主体对**自己**档案的部分合并更新（PATCH
// /api/v1/auth/profile）。与管理员 PUT /users/{id}/profile 的全量替换不同：
// 指针字段 nil 表示「不改」，只有非 nil 字段覆盖现值；未传字段原样保留。
//
// 安全边界：
//   - 没有 Email/Phone 字段——联系方式必须走 /api/v1/auth/account 的验证码
//     绑定流程，自助档案编辑绝不改动联系方式及其验证状态。
//   - OrganizationID/UserID 由 transport 从可信会话注入，请求体永远不能
//     指定操作对象；仓储按 GetByIDForOrganization 做归属裁决。
type UpdateSelfProfileCommand struct {
	OrganizationID string
	UserID         string

	DisplayName           *string
	FirstName             *string
	LastName              *string
	AvatarURL             *string
	CountryCode           *string
	Region                *string
	Location              *string
	Birthday              *string
	Gender                *string
	Bio                   *string
	RealName              *string
	IDCardType            *string
	IDCard                *string
	PassportNo            *string
	DriverLicenseNo       *string
	IDCardExpiry          *string
	IssueCountry          *string
	IssuingAuthority      *string
	PassportPhotoURL      *string
	DriverLicenseFrontURL *string
	DriverLicenseBackURL  *string
	Affiliation           *string
	Homepage              *string
	Education             *string
	Title                 *string
	Language              *string
	IDCardFront           *string
	IDCardBack            *string
	IDCardWithPerson      *string
	Properties            map[string]any
}

// overrideSelfProfileField applies one optional (nil = keep) string field.
func overrideSelfProfileField(target *string, value *string) {
	if value != nil {
		*target = *value
	}
}

// mergeSelfProfileProperties copies the stored properties and overlays the
// request: a null or explicit empty string deletes the key (与管理员 PUT 的
// normalizeProfileProperties 空值语义一致)，其余键覆盖。无独立列的档案字段
// （affiliation/homepage/education/title/language/idCardFront/idCardBack/
// idCardWithPerson）随后按指针语义折叠进同一 map。
func mergeSelfProfileProperties(current map[string]any, command UpdateSelfProfileCommand) map[string]any {
	merged := make(map[string]any, len(current)+len(command.Properties))
	for key, value := range current {
		merged[key] = value
	}
	for key, value := range command.Properties {
		if value == nil {
			delete(merged, key)
			continue
		}
		if text, ok := value.(string); ok && text == "" {
			delete(merged, key)
			continue
		}
		merged[key] = value
	}
	for key, value := range map[string]*string{
		profileKeyAffiliation: command.Affiliation,
		profileKeyHomepage:    command.Homepage,
		profileKeyEducation:   command.Education,
		profileKeyTitle:       command.Title,
		profileKeyLanguage:    command.Language,
		"idCardFront":         command.IDCardFront,
		"idCardBack":          command.IDCardBack,
		"idCardWithPerson":    command.IDCardWithPerson,
	} {
		if value == nil {
			continue
		}
		if *value == "" {
			delete(merged, key)
			continue
		}
		merged[key] = *value
	}
	return merged
}

// UpdateSelfProfile atomically reads the caller's profile, overlays only the
// supplied fields and writes it back in one transaction. Email/Phone and their
// verified flags are carried over verbatim — the self-service face has no
// contact-binding capability. The merged profile is projected onto the admin
// UpdateUserProfileCommand so validateUpdateUserProfileCommand applies with
// exactly the same strictness (homepage URL, document lengths, date form,
// properties bounds); nothing is relaxed for the self-service caller.
//
// 审计只记录 selfService 标记，绝不携带 RealName/IDCard 等 PII 明文。
func (s *Service) UpdateSelfProfile(ctx context.Context, command UpdateSelfProfileCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Resolve ownership first: a foreign user must leave profile untouched.
		user, err := tx.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID)
		if err != nil {
			return err
		}

		profile := user.Profile
		overrideSelfProfileField(&profile.DisplayName, command.DisplayName)
		overrideSelfProfileField(&profile.FirstName, command.FirstName)
		overrideSelfProfileField(&profile.LastName, command.LastName)
		overrideSelfProfileField(&profile.AvatarURL, command.AvatarURL)
		overrideSelfProfileField(&profile.CountryCode, command.CountryCode)
		overrideSelfProfileField(&profile.Region, command.Region)
		overrideSelfProfileField(&profile.Location, command.Location)
		overrideSelfProfileField(&profile.Birthday, command.Birthday)
		overrideSelfProfileField(&profile.Gender, command.Gender)
		overrideSelfProfileField(&profile.Bio, command.Bio)
		overrideSelfProfileField(&profile.RealName, command.RealName)
		overrideSelfProfileField(&profile.IDCardType, command.IDCardType)
		overrideSelfProfileField(&profile.IDCard, command.IDCard)
		overrideSelfProfileField(&profile.PassportNo, command.PassportNo)
		overrideSelfProfileField(&profile.DriverLicenseNo, command.DriverLicenseNo)
		overrideSelfProfileField(&profile.IDCardExpiry, command.IDCardExpiry)
		overrideSelfProfileField(&profile.IssueCountry, command.IssueCountry)
		overrideSelfProfileField(&profile.IssuingAuthority, command.IssuingAuthority)
		overrideSelfProfileField(&profile.PassportPhotoURL, command.PassportPhotoURL)
		overrideSelfProfileField(&profile.DriverLicenseFrontURL, command.DriverLicenseFrontURL)
		overrideSelfProfileField(&profile.DriverLicenseBackURL, command.DriverLicenseBackURL)
		// Email/Phone/EmailVerified/PhoneVerified 一律保持原值：自助面没有
		// 联系方式绑定能力，验证状态只能由验证码流程改写。

		properties := mergeSelfProfileProperties(profile.Properties, command)
		// 规范 9.3：已验证用户的实名字段后端锁定（nil=不改已折叠回存储值，
		// 只有显式改动才会触发拒绝）。
		if err := ensureVerifiedProfileFieldsLocked(user, profile.RealName, profile.IDCardType, profile.IDCard, properties); err != nil {
			return err
		}
		// Affiliation 与管理员 PUT 同口径：从合并后的 properties 派生，
		// 键被删除时列值随之清空。
		profile.Affiliation = ""
		if value, ok := properties[profileKeyAffiliation]; ok {
			if text, ok := value.(string); ok {
				profile.Affiliation = text
			}
		}
		profile.Properties = properties
		profile.UserID = user.ID

		// 校验不放宽：把合并后的完整档案投影成管理员命令，复用同一校验。
		merged := UpdateUserProfileCommand{
			OrganizationID:        command.OrganizationID,
			UserID:                command.UserID,
			DisplayName:           profile.DisplayName,
			FirstName:             profile.FirstName,
			LastName:              profile.LastName,
			AvatarURL:             profile.AvatarURL,
			Email:                 profile.Email,
			Phone:                 profile.Phone,
			CountryCode:           profile.CountryCode,
			Region:                profile.Region,
			Location:              profile.Location,
			Birthday:              profile.Birthday,
			Gender:                profile.Gender,
			Bio:                   profile.Bio,
			RealName:              profile.RealName,
			IDCardType:            profile.IDCardType,
			IDCard:                profile.IDCard,
			PassportNo:            profile.PassportNo,
			DriverLicenseNo:       profile.DriverLicenseNo,
			IDCardExpiry:          profile.IDCardExpiry,
			IssueCountry:          profile.IssueCountry,
			IssuingAuthority:      profile.IssuingAuthority,
			PassportPhotoURL:      profile.PassportPhotoURL,
			DriverLicenseFrontURL: profile.DriverLicenseFrontURL,
			DriverLicenseBackURL:  profile.DriverLicenseBackURL,
			Properties:            properties,
		}
		if err := validateUpdateUserProfileCommand(merged); err != nil {
			return err
		}
		if err := s.validateCustomAttributeProperties(ctx, tx, command.OrganizationID, properties); err != nil {
			return err
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

		return s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			command.UserID,
			"user.updated",
			"User",
			user.ID,
			"user",
			"user.updated",
			map[string]any{"selfService": true},
		)
	})
}
