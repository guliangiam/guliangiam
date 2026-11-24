package facetemplates

// MySQL 8.4+ 镜像：与 postgres/facetemplates 同域接口。template_ref 保存绑定
// (table, column, user_id) 的 v2 信封密文；user_id UNIQUE，重采走
// ON DUPLICATE KEY UPDATE 原位更新。占位符为 ?，列顺序与 scanOne 严格一致。

const queryEnroll = "INSERT INTO `user_face_templates` " +
	"(`id`, `organization_id`, `user_id`, `template_ref`, `provider`, `provider_ref`, " +
	"`status`, `revoked_at`, `created_at`, `updated_at`, `revision`) " +
	"VALUES (?, ?, ?, ?, ?, ?, 'active', NULL, ?, ?, 1) " +
	"ON DUPLICATE KEY UPDATE " +
	"`template_ref` = VALUES(`template_ref`), " +
	"`provider` = VALUES(`provider`), " +
	"`provider_ref` = VALUES(`provider_ref`), " +
	"`status` = 'active', " +
	"`revoked_at` = NULL, " +
	"`updated_at` = VALUES(`updated_at`), " +
	"`revision` = `revision` + 1"

const queryGetActiveByUser = "SELECT `id`, `organization_id`, `user_id`, `template_ref`, `provider`, `provider_ref`, " +
	"`status`, `revoked_at`, `created_at`, `updated_at` " +
	"FROM `user_face_templates` " +
	"WHERE `organization_id` = ? AND `user_id` = ? AND `status` = 'active'"

const queryRevokeByUser = "UPDATE `user_face_templates` " +
	"SET `status` = 'revoked', `revoked_at` = ?, `updated_at` = ?, `revision` = `revision` + 1 " +
	"WHERE `organization_id` = ? AND `user_id` = ? AND `status` = 'active'"
