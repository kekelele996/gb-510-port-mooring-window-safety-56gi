package service

import "errors"

var (
	ErrInvalidTransition = errors.New("requested status transition is not allowed")
	ErrInvalidInput      = errors.New("business input validation failed")
	ErrUnauthorized      = errors.New("invalid username or password")
	ErrInactiveUser      = errors.New("user account is inactive")
	ErrSelfApproval      = errors.New("the submitter cannot approve the same safety clearance")
	ErrReviewerRequired  = errors.New("a reviewer or administrator must perform the second confirmation")
	ErrWindowVersion     = errors.New("the weather window version is missing or changed")
	ErrWindowNotFound    = errors.New("the linked weather window was not found for this facility and code")
	ErrWindowNotSafe     = errors.New("the linked weather window is not currently safe")
	ErrWindowRestricted  = errors.New("the linked weather window is restricted")
	ErrWindowExpired     = errors.New("the linked weather window is expired or past its validity")
	ErrOperatorRequired  = errors.New("an operator or administrator must resubmit the clearance")
)
