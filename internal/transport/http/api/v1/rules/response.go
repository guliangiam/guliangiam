package rules

import (
	apprules "github.com/guliangiam/guliangiam/internal/app/rules"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ListRulesResponse = presenter.ListResponse[apprules.RuleListItem]
type RuleResponse = apprules.RuleDetail
