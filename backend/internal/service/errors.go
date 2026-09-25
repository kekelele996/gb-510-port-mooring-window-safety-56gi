package service

import "errors"

var (
	ErrInvalidTransition = errors.New("requested status transition is not allowed")
	ErrInvalidInput      = errors.New("business input validation failed")
	ErrUnauthorized      = errors.New("invalid username or password")
	ErrInactiveUser      = errors.New("user account is inactive")
	ErrSelfApproval      = errors.New("the submitter cannot approve the same safety clearance")
	ErrReviewerRequired  = errors.New("a reviewer or administrator must perform the second confirmation")
	ErrWindowVersion     = errors.New("the linked weather window version changed; please resubmit the safety confirmation")
	ErrWindowLinked      = errors.New("no weather window matches the clearance facility and window code")
	ErrWindowState       = errors.New("the linked weather window is restricted or expired; clearance must be resubmitted")
	ErrWindowExpired     = errors.New("the linked weather window passed its expiry time; clearance must be resubmitted")
)
