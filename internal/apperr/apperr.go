package apperr

type Error struct {
	Status  int
	Message string
}

func (e Error) Error() string {
	return e.Message
}

func New(status int, message string) Error {
	return Error{Status: status, Message: message}
}
