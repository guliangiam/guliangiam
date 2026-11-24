package organizations

import (
	"context"
	"testing"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	orgmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestValidateOrganizationParentChain_Root(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := orgmock.NewMockRepository(ctrl)
	require.NoError(t, validateOrganizationParentChain(context.Background(), repo, "org-new", ""))
}

func TestValidateOrganizationParentChain_MissingParent(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := orgmock.NewMockRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), "missing").Return(nil, apperrors.NewNotFound("organization not found"))

	err := validateOrganizationParentChain(context.Background(), repo, "org-new", "missing")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "parentId", appErr.Meta["field"])
}

func TestValidateOrganizationParentChain_Cycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := orgmock.NewMockRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), "org-2").Return(&orgdomain.Organization{ID: "org-2", ParentID: "org-1"}, nil)
	repo.EXPECT().GetByID(gomock.Any(), "org-1").Return(&orgdomain.Organization{ID: "org-1"}, nil)

	err := validateOrganizationParentChain(context.Background(), repo, "org-1", "org-2")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "org-1", appErr.Meta["cycleAt"])
}

func TestValidateOrganizationParentChain_Normal(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := orgmock.NewMockRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), "org-2").Return(&orgdomain.Organization{ID: "org-2", ParentID: "org-1"}, nil)
	repo.EXPECT().GetByID(gomock.Any(), "org-1").Return(&orgdomain.Organization{ID: "org-1"}, nil)

	require.NoError(t, validateOrganizationParentChain(context.Background(), repo, "org-new", "org-2"))
}

func TestValidateOrganizationParentChain_DepthExceeded(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := orgmock.NewMockRepository(ctrl)
	chain := make(map[string]*orgdomain.Organization, orgdomain.MaxOrganizationTreeDepth)
	id := func(index int) string { return "org-" + string(rune('a'+index)) }
	for index := 0; index < orgdomain.MaxOrganizationTreeDepth; index++ {
		current := id(index)
		parent := ""
		if index < orgdomain.MaxOrganizationTreeDepth-1 {
			parent = id(index + 1)
		}
		chain[current] = &orgdomain.Organization{ID: current, ParentID: parent}
	}
	repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, organizationID string) (*orgdomain.Organization, error) {
			return chain[organizationID], nil
		}).AnyTimes()

	err := validateOrganizationParentChain(context.Background(), repo, "org-new", id(0))
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, orgdomain.MaxOrganizationTreeDepth, appErr.Meta["maxDepth"])
}
