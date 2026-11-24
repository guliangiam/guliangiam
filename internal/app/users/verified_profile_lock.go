package users

import (
	"reflect"
	"strings"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 规范 9.3：isVerified=true 后 realName/idCardType/idCard/idCardImages 锁定，
// UpdateProfile（PUT 全量替换）与 UpdateSelfProfile（PATCH 指针合并）都必须
// 在后端拒绝修改。前端 disabled 只是交互层，不构成边界。
//
// 比较发生在「保留/合并语义应用之后」：PUT 的空值保留（证件号永不回传原文）
// 与 PATCH 的 nil=不改都会把未触碰的锁定字段折叠回存储值，因此只有真正改动
// 才会触发拒绝。
var lockedVerifiedPropertyKeys = []string{"idCardFront", "idCardBack", "idCardWithPerson"}

func ensureVerifiedProfileFieldsLocked(user *userdomain.User, realName, idCardType, idCard string, properties map[string]any) error {
	if !user.IsVerified {
		return nil
	}
	changed := make([]string, 0, len(lockedVerifiedPropertyKeys)+3)
	if realName != user.Profile.RealName {
		changed = append(changed, "realName")
	}
	if idCardType != user.Profile.IDCardType {
		changed = append(changed, "idCardType")
	}
	if idCard != user.Profile.IDCard {
		changed = append(changed, "idCard")
	}
	for _, key := range lockedVerifiedPropertyKeys {
		if !lockedPropertyUnchanged(user.Profile.Properties, properties, key) {
			changed = append(changed, key)
		}
	}
	if len(changed) > 0 {
		return apperrors.NewInvalidArgumentf(
			"identity fields are locked after verification: %s", strings.Join(changed, ", "))
	}
	return nil
}

// lockedPropertyUnchanged reports whether one locked property key survives the
// incoming map unchanged. A missing key on either side counts as a delete and
// therefore as a change.
func lockedPropertyUnchanged(stored, incoming map[string]any, key string) bool {
	storedValue, storedOK := stored[key]
	incomingValue, incomingOK := incoming[key]
	if storedOK != incomingOK {
		return false
	}
	if !storedOK {
		return true
	}
	return reflect.DeepEqual(storedValue, incomingValue)
}
