package webhooks

import "testing"

// BE-B-17: headers must be a JSON object with string values — exactly the
// shape the delivery worker decodes (map[string]string). Valid-but-non-object
// JSON used to pass validation and then be silently dropped on every delivery.
func TestValidateWebhookHeaders_RequiresStringMapObject(t *testing.T) {
	valid := []string{
		"",
		"   ",
		`{}`,
		`{"X-Token":"abc"}`,
		` {"X-Token":"abc","X-Other":"def"} `,
	}
	for _, headers := range valid {
		if err := validateWebhookHeaders(headers); err != nil {
			t.Fatalf("headers %q must be accepted: %v", headers, err)
		}
	}

	invalid := []string{
		`{not json`,
		`[{"name":"X-Token","value":"abc"}]`, // legacy FE array shape
		`"X-Token: abc"`,
		`42`,
		`null`,
		`{"X-Token":42}`,      // non-string value
		`{"X-Token":{"a":1}}`, // nested object value
	}
	for _, headers := range invalid {
		assertUnprocessableEntity(t, validateWebhookHeaders(headers))
	}
}
