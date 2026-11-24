// wire_jml.go — ADR-030 JML 生产装配（BE-D-11）。
package bootstrap

import (
	"context"

	jmlapp "github.com/guliangiam/guliangiam/internal/app/jml"
	appscim "github.com/guliangiam/guliangiam/internal/app/scim"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	usersdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// jmlRoleBindingCapability 镜像 app/jml 对 users 仓储的结构性要求。
// postgres 实现满足；MySQL 镜像未实现（OPEN-API-37）→ 探测失败 → 不注入
// JML（扩展点关闭，users/scim 保持原有行为），而不是运行期才报 Unavailable。
type jmlRoleBindingCapability interface {
	CreateRoleBinding(ctx context.Context, binding *usersdomain.UserRoleBinding) error
	DeleteRoleBinding(ctx context.Context, userID, roleID string) error
	DeleteRoleBindingsByUser(ctx context.Context, userID string) error
	ListRoleBindingsByUser(ctx context.Context, userID string) ([]usersdomain.UserRoleBinding, error)
}

// jmlLifecycleFor 在 users 仓储具备精细化角色绑定能力时返回事务感知的
// JML 端口，否则返回 nil 接口（调用方 nil → 跳过 JML 编排）。
func jmlLifecycleFor(uow appuow.UnitOfWork, clk clock.Clock) jmlapp.LifecyclePort {
	if _, ok := uow.Users().(jmlRoleBindingCapability); !ok {
		return nil
	}
	return jmlapp.NewUoWLifecycle(clk)
}

// newSCIMService 构造 SCIM 服务并按能力注入 JML（Joiner/Leaver 与业务同事务）。
func newSCIMService(uuidGenerator platformuuid.Generator, uow appuow.UnitOfWork) appscim.Service {
	clk := clock.SystemClock{}
	svc := appscim.NewService(clk, uuidGenerator, uow)
	svc.JML = jmlLifecycleFor(uow, clk)
	return svc
}
