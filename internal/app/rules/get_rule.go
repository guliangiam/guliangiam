package rules

import "context"

func (s Service) GetRule(ctx context.Context, organizationID, id string) (*RuleDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	rule, err := s.UOW.Rules().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	detail := toDetail(rule)
	return &detail, nil
}
