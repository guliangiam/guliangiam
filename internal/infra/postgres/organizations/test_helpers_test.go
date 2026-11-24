package organizations

import "time"

func mustParseTime() time.Time {
	t, err := time.Parse(time.RFC3339, "2024-01-01T00:00:00Z")
	if err != nil {
		panic(err)
	}
	return t
}
