package procurement

import "fmt"

// allowedTransitions maps status → set of next statuses (manual-order mode).
var allowedTransitions = map[string]map[string]bool{
	StatusDraft: {
		StatusPendingConfirm: true,
		StatusCancelled:      true,
	},
	StatusPendingConfirm: {
		StatusPlacing:   true,
		StatusCancelled: true,
	},
	StatusPlacing: {
		StatusPlaced:    true,
		StatusFailed:    true,
		StatusCancelled: true,
	},
	StatusPlaced: {
		StatusPaid:      true,
		StatusFailed:    true,
		StatusCancelled: true,
	},
	StatusPaid: {
		StatusShipped: true,
		StatusFailed:  true,
	},
	StatusShipped: {
		StatusDelivered: true,
	},
	StatusFailed: {
		StatusPlacing:   true, // retry
		StatusCancelled: true,
		StatusVoided:    true,
	},
	StatusDelivered: {
		StatusVoided: true,
	},
	StatusCancelled: {
		StatusVoided: true,
	},
	StatusVoided: {},
}

// CanTransition reports whether from → to is a legal status move.
func CanTransition(from, to string) bool {
	next, ok := allowedTransitions[from]
	return ok && next[to]
}

// ErrIllegalTransition builds the standard error for a rejected move.
func ErrIllegalTransition(from, to string) error {
	return fmt.Errorf("%w: illegal transition %s -> %s", ErrConflict, from, to)
}
