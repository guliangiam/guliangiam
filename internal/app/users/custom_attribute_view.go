package users

import customattributes "github.com/guliangiam/guliangiam/internal/domain/customattributes"

// CustomAttributeView is the transport-neutral projection of a custom attribute
// definition. Keeping it in the app layer prevents transport from importing the
// domain package (check-layers Rule 2).
type CustomAttributeView struct {
	ID          string
	Key         string
	DataType    string
	Required    bool
	Options     []string
	Writable    bool
	Visibility  string
	Description string
	Enabled     bool
	Revision    int64
}

func ToCustomAttributeView(definition *customattributes.Definition) CustomAttributeView {
	options := definition.Options
	if options == nil {
		options = []string{}
	}
	return CustomAttributeView{
		ID: definition.ID, Key: definition.Key, DataType: string(definition.DataType),
		Required: definition.Required, Options: options, Writable: definition.Writable,
		Visibility: string(definition.Visibility), Description: definition.Description,
		Enabled: definition.Enabled, Revision: definition.Revision,
	}
}
