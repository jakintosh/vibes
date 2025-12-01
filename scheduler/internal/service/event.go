package service

import (
	"errors"
	"time"

	db "event-scheduler/internal/db"

	"github.com/google/uuid"
)

var (
	ErrConflict                = errors.New("conflict detected")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	ErrInvalidPayment          = errors.New("invalid payment details")
)

func GetEvent(id string) (*db.Event, error) {
	event, err := db.GetEvent(id)
	if err != nil {
		return nil, err
	}

	if err := normalizePayment(event); err != nil {
		return nil, err
	}

	return event, nil
}

func AcceptEvent(id string, start, end time.Time, proposedCost float64, deposit float64) error {
	event, err := db.GetEvent(id)
	if err != nil {
		return err
	}

	if event.Status != db.StatusRequested {
		return ErrInvalidStatusTransition
	}

	if proposedCost < 0 || deposit < 0 || deposit > proposedCost {
		return ErrInvalidPayment
	}

	// Check for conflicts with existing accepted events
	allEvents, err := db.GetEvents()
	if err != nil {
		return err
	}

	for _, e := range allEvents {
		if isScheduledStatus(e.Status) && e.AcceptedDate != nil {
			// Check overlap
			if start.Before(e.AcceptedDate.End) && end.After(e.AcceptedDate.Start) {
				return ErrConflict
			}
		}
	}

	newDate := db.EventDate{Start: start, End: end}
	return db.AcceptEvent(id, newDate, proposedCost, deposit)
}

func CreateEvent(e db.Event) error {
	// Set system fields
	e.ID = uuid.New().String()
	e.Status = db.StatusRequested
	e.PaymentStatus = db.PaymentProposed
	// CreatedAt is handled by db.CreateEvent if zero, but we can set it here too
	e.CreatedAt = time.Now()

	return db.CreateEvent(e)
}

func ConfirmEvent(id string) error {
	event, err := db.GetEvent(id)
	if err != nil {
		return err
	}

	if event.Status != db.StatusAccepted || event.AcceptedDate == nil {
		return ErrInvalidStatusTransition
	}

	if err := db.UpdateEventStatus(id, db.StatusConfirmed); err != nil {
		return err
	}

	return db.UpdatePaymentStatus(id, db.PaymentDue)
}

func WithdrawEvent(id string) error {
	event, err := db.GetEvent(id)
	if err != nil {
		return err
	}

	if event.Status != db.StatusAccepted {
		return ErrInvalidStatusTransition
	}

	return db.UpdateEventStatus(id, db.StatusWithdrawn)
}

func DenyEvent(id string) error {
	event, err := db.GetEvent(id)
	if err != nil {
		return err
	}

	if event.Status != db.StatusRequested {
		return ErrInvalidStatusTransition
	}

	return db.UpdateEventStatus(id, db.StatusDenied)
}

func CancelEvent(id string) error {
	event, err := db.GetEvent(id)
	if err != nil {
		return err
	}

	if event.Status != db.StatusAccepted && event.Status != db.StatusConfirmed {
		return ErrInvalidStatusTransition
	}

	return db.UpdateEventStatus(id, db.StatusCanceled)
}

func isScheduledStatus(status db.EventStatus) bool {
	return status == db.StatusAccepted || status == db.StatusConfirmed
}

func normalizePayment(event *db.Event) error {
	newStatus := event.PaymentStatus
	if newStatus == "" {
		newStatus = db.PaymentProposed
	}

	if newStatus == db.PaymentPaid && event.AmountReceived < event.ProposedCost {
		newStatus = db.PaymentDue
	}

	if (newStatus == db.PaymentDue || newStatus == db.PaymentPaid) && event.AmountReceived >= event.ProposedCost {
		newStatus = db.PaymentPaid
	}

	if newStatus == db.PaymentPaid && event.AcceptedDate != nil {
		now := time.Now()
		if now.After(event.AcceptedDate.End) {
			if event.DepositAmount > 0 {
				newStatus = db.PaymentRefundable
			} else {
				newStatus = db.PaymentSettled
			}
		}
	}

	if newStatus != event.PaymentStatus {
		event.PaymentStatus = newStatus
		return db.UpdatePaymentStatus(event.ID, newStatus)
	}

	event.PaymentStatus = newStatus
	return nil
}

func RecordPayment(id string, amount float64) error {
	if amount <= 0 {
		return ErrInvalidPayment
	}

	event, err := db.GetEvent(id)
	if err != nil {
		return err
	}

	if event.PaymentStatus == "" {
		event.PaymentStatus = db.PaymentProposed
	}

	if event.PaymentStatus != db.PaymentDue && event.PaymentStatus != db.PaymentPaid {
		return ErrInvalidStatusTransition
	}

	newTotal := event.AmountReceived + amount
	newStatus := event.PaymentStatus
	if newTotal >= event.ProposedCost {
		newStatus = db.PaymentPaid
	} else {
		newStatus = db.PaymentDue
	}

	if newStatus == db.PaymentPaid && event.AcceptedDate != nil && time.Now().After(event.AcceptedDate.End) {
		if event.DepositAmount > 0 {
			newStatus = db.PaymentRefundable
		} else {
			newStatus = db.PaymentSettled
		}
	}

	return db.UpdatePayment(event.ID, newStatus, newTotal)
}

func SettleRefundable(id string) error {
	event, err := db.GetEvent(id)
	if err != nil {
		return err
	}

	if event.PaymentStatus != db.PaymentRefundable {
		return ErrInvalidStatusTransition
	}

	return db.UpdatePaymentStatus(id, db.PaymentSettled)
}
