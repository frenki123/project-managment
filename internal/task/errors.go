package task

type ErrorKind string

const (
	InvalidInput ErrorKind = "invalid_input"
	NotFound     ErrorKind = "not_found"
	Conflict     ErrorKind = "conflict"
)

type Error struct {
	Kind    ErrorKind
	Message string
}

func (e Error) Error() string { return e.Message }

func Invalid(message string) error       { return Error{Kind: InvalidInput, Message: message} }
func Missing(message string) error       { return Error{Kind: NotFound, Message: message} }
func ConflictError(message string) error { return Error{Kind: Conflict, Message: message} }
