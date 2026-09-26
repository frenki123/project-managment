package monthlock

type ErrorKind string

const (
	InvalidInput ErrorKind = "invalid_input"
)

type Error struct {
	Kind    ErrorKind
	Message string
}

func (e Error) Error() string { return e.Message }

func Invalid(message string) error { return Error{Kind: InvalidInput, Message: message} }
