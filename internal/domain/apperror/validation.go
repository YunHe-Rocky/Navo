package apperror

import (
	"fmt"
	"strings"
)

const (
	CodeRequired          = "required"
	CodeInvalid           = "invalid"
	CodeMutuallyExclusive = "mutually_exclusive"
	CodeOutOfRange        = "out_of_range"
)

// ValidationError is safe to expose through IPC because it never contains secrets.
type ValidationError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	messages := make([]string, 0, len(e))
	for _, item := range e {
		messages = append(messages, item.Error())
	}
	return strings.Join(messages, "; ")
}

func (e ValidationErrors) Err() error {
	if len(e) == 0 {
		return nil
	}
	return e
}
