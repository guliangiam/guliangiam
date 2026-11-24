package scim

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/scim/v2/Users",
			OperationID: "ScimListUsers",
			Summary:     "SCIM list users",
			Permission:  "scim.users",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "POST",
			Pattern:     "/scim/v2/Users",
			OperationID: "ScimCreateUser",
			Summary:     "SCIM create user",
			Permission:  "scim.users",
			Surface:     route.SurfaceProtocol,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/scim/v2/Users/{id}",
			OperationID: "ScimGetUser",
			Summary:     "SCIM get user",
			Permission:  "scim.users",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "PUT",
			Pattern:     "/scim/v2/Users/{id}",
			OperationID: "ScimReplaceUser",
			Summary:     "SCIM replace user",
			Permission:  "scim.users",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "PATCH",
			Pattern:     "/scim/v2/Users/{id}",
			OperationID: "ScimPatchUser",
			Summary:     "SCIM patch user",
			Permission:  "scim.users",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "DELETE",
			Pattern:     "/scim/v2/Users/{id}",
			OperationID: "ScimDeleteUser",
			Summary:     "SCIM delete user",
			Permission:  "scim.users",
			Surface:     route.SurfaceProtocol,

			SuccessStatus: 204,
		},
		{
			Method:      "GET",
			Pattern:     "/scim/v2/Groups",
			OperationID: "ScimListGroups",
			Summary:     "SCIM list groups",
			Permission:  "scim.groups",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "POST",
			Pattern:     "/scim/v2/Groups",
			OperationID: "ScimCreateGroup",
			Summary:     "SCIM create group",
			Permission:  "scim.groups",
			Surface:     route.SurfaceProtocol,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/scim/v2/Groups/{id}",
			OperationID: "ScimGetGroup",
			Summary:     "SCIM get group",
			Permission:  "scim.groups",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "PUT",
			Pattern:     "/scim/v2/Groups/{id}",
			OperationID: "ScimReplaceGroup",
			Summary:     "SCIM replace group",
			Permission:  "scim.groups",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "PATCH",
			Pattern:     "/scim/v2/Groups/{id}",
			OperationID: "ScimPatchGroup",
			Summary:     "SCIM patch group",
			Permission:  "scim.groups",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "DELETE",
			Pattern:     "/scim/v2/Groups/{id}",
			OperationID: "ScimDeleteGroup",
			Summary:     "SCIM delete group",
			Permission:  "scim.groups",
			Surface:     route.SurfaceProtocol,

			SuccessStatus: 204,
		},
		{
			Method:      "GET",
			Pattern:     "/scim/v2/ServiceProviderConfig",
			OperationID: "ScimGetServiceProviderConfig",
			Summary:     "SCIM service provider config",
			Permission:  "scim.discovery",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/scim/v2/ResourceTypes",
			OperationID: "ScimGetResourceTypes",
			Summary:     "SCIM resource types",
			Permission:  "scim.discovery",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/scim/v2/Schemas",
			OperationID: "ScimGetSchemas",
			Summary:     "SCIM schemas",
			Permission:  "scim.discovery",
			Surface:     route.SurfaceProtocol,
		},
	}
}
