package auth

import (
	"context"
	"log/slog"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

// applyLoginSessionPolicies 是所有登录方式共用的「登录成功、会话已创建」收口点，
// 把与认证方式无关的会话策略与 signin 统计集中到一处（《开发设计》22.5）：
//
//  1. 应用级排他登录：application.EnableExclusiveSignin 为真时，撤销同一 user+app 下
//     除新会话以外的全部会话（application 为 nil 的路径没有应用维度，跳过）；
//  2. 组织级并发会话上限：按 organization_security_policies 的并发会话策略执行
//     revoke_oldest / reject_new（reject_new 时返回错误，本次登录整体失败）；
//  3. signin 统计：清零失败次数、记录 LastSigninAt/LastSigninIP，并留 user.updated 审计。
//
// 参数 session 是刚创建并已落库的会话：它既是排他/并发撤销时的排除项，也是
// LastSigninIP 的来源；调用方必须先创建会话，并发上限的语义才是「含新会话在内不超过 N」。
//
// 该函数此前只存在于密码登录路径，LDAP/社交/Kerberos 各抄了一份（且都漏了并发上限），
// Face ID / 扫码 / WebAuthn 三条服务端登录路径则完全没有——同一租户的策略会随
// 「用户点了哪个登录按钮」而变。现在七条路径共用本函数。
func (s *Service) applyLoginSessionPolicies(
	ctx context.Context,
	tx appuow.UnitOfWork,
	user *userdomain.User,
	application *applicationdomain.Application,
	organizationID string,
	session *sessiondomain.Session,
	now time.Time,
) error {
	if application != nil && application.EnableExclusiveSignin {
		if _, err := tx.Sessions().RevokeByUserAndApp(ctx, user.ID, application.ID, session.ID, now); err != nil {
			return err
		}
	}

	if err := enforceConcurrentSessionLimitWithTx(ctx, tx, user.ID, organizationID, session.ID, now, user.IsAdmin, session.AuthenticationLevel); err != nil {
		return err
	}

	user.SigninWrongTimes = 0
	user.LastSigninAt = &now
	user.LastSigninIP = session.IPAddress
	user.UpdatedAt = now
	user.Revision++
	if err := tx.Users().Update(ctx, user); err != nil {
		return err
	}

	// actorID 必须是 UUID：records.actor_id 是 uuid 列，传用户名会在插入时
	// 报 invalid input syntax for type uuid，审计写不进去还会连带回滚整个登录事务。
	return s.embedAuditOutboxPlain(
		tx,
		ctx,
		"",
		"",
		"",
		user.OrganizationID,
		user.ID,
		"user.updated",
		"user",
		user.ID,
		"user",
		"user.updated",
		map[string]any{},
	)
}

// warnAnomalousLogin 在同一事务内做异常登录检测，命中或失败都只记录告警，绝不阻断
// 登录（与密码登录路径同一口径）。IP 为空时检测函数本身直接返回空报告。
func (s *Service) warnAnomalousLogin(ctx context.Context, tx appuow.UnitOfWork, userID, ipAddress string) {
	report := detectAnomalousLoginWithTx(ctx, tx, s.GeoLookup, userID, ipAddress)
	if report.IsAnomalous {
		slog.WarnContext(ctx, "anomalous login detected",
			"user_id", userID,
			"ip", ipAddress,
			"reasons", report.Reasons,
		)
	}
}
