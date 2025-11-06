package helper

import (
	"github.com/google/uuid"
)

func DeferOrString(p *string, def string) string {
	if p != nil && *p != "" {
		return *p
	}
	return def
}

func GenerateID() string {
	return uuid.NewString()
}
