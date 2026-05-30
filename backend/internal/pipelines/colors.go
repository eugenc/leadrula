package pipelines

import "github.com/echayko/leadrula/backend/pkg/httpx"

var ValidColors = []string{
	"gray", "jade", "blue", "amber", "red", "purple", "teal", "orange", "pink", "slate",
}

func validateColor(c string) error {
	for _, v := range ValidColors {
		if c == v {
			return nil
		}
	}
	return httpx.Validation("invalid stage color")
}
