package domain

// Errores de aplicación con semántica HTTP conocida por la capa delivery para
// mapearlos a statuses. Los usecases los devuelven directamente.

type BadRequestError struct{ Message string }

func (e *BadRequestError) Error() string { return e.Message }

func NewBadRequestError(msg string) error { return &BadRequestError{Message: msg} }

type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }

func NewNotFoundError(msg string) error { return &NotFoundError{Message: msg} }

type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }

func NewConflictError(msg string) error { return &ConflictError{Message: msg} }

type UnauthorizedError struct{ Message string }

func (e *UnauthorizedError) Error() string { return e.Message }

func NewUnauthorizedError(msg string) error { return &UnauthorizedError{Message: msg} }

type ForbiddenError struct{ Message string }

func (e *ForbiddenError) Error() string { return e.Message }

func NewForbiddenError(msg string) error { return &ForbiddenError{Message: msg} }
