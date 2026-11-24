package regions

import "context"

// DeploymentRegionLister reads the deployment region registry for management
// UIs (organization region assignment). It is read-only; writes stay with the
// platform control plane.
type DeploymentRegionLister interface {
	ListDeploymentRegions(ctx context.Context) ([]*DeploymentRegion, error)
}
