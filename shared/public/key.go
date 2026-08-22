package public

import (
	"errors"
	"fmt"
)

const DefaultPrefix = "auth:session"

func GetKey(subjectType, sessionID string) (string, error) {
	if subjectType == "" || sessionID == "" {
		return "", errors.New("subjectType and sessionID are required")

	}
	return fmt.Sprintf("%s:%s:%s", DefaultPrefix, subjectType, sessionID), nil
}
