package rental

import (
	"errors"
	"fmt"
	"time"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/listing"
)

// DefaultCommissionBPS is the platform commission in basis points (12%).
const DefaultCommissionBPS int64 = 1200

// QuoteInput is the input to the pricing pipeline.
type QuoteInput struct {
	Snapshot      ListingSnapshot
	StartsAt      time.Time
	EndsAt        time.Time
	WithOperator  bool
	CommissionBPS int64
}

// PriceQuote is the F3 pricing pipeline (skill: pre-implementation-design —
// one atomic function, ~28 lines).
func PriceQuote(in QuoteInput) (MoneyBreakdown, error) {
	if err := ValidateWindow(in.StartsAt, in.EndsAt, time.Time{}, 0); err != nil {
		return MoneyBreakdown{}, err
	}
	if in.WithOperator && in.Snapshot.Operator.Mode == string(listing.OperatorNone) {
		return MoneyBreakdown{}, ErrOperatorNotAvailable
	}
	hours := rentHours(in.StartsAt, in.EndsAt, in.Snapshot.PriceUnit)
	rent := rentCents(in.Snapshot.PriceAmountCents, hours)
	operator := operatorCents(in.Snapshot, hours, in.WithOperator)
	deposit := in.Snapshot.DepositCents
	if deposit < 0 {
		return MoneyBreakdown{}, fmt.Errorf("%w: deposit must be >= 0", ErrInvalidInput)
	}
	total := rent + operator + deposit
	commissionable := rent + operator
	commission := applyCommission(commissionable, effectiveBPS(in.CommissionBPS))
	owner, op := splitLiquids(rent, operator, in.Snapshot, commission)
	return MoneyBreakdown{
		RentCents:             rent,
		OperatorCents:         operator,
		DepositCents:          deposit,
		TotalCents:            total,
		CommissionBaseCents:   commissionable,
		CommissionCents:       commission,
		OwnerPayoutCents:      owner,
		OperatorPayoutCents:   op,
	}, nil
}

func rentHours(start, end time.Time, unit string) int64 {
	switch unit {
	case "hour", "":
		d := end.Sub(start)
		if d <= 0 {
			return 0
		}
		h := d / time.Hour
		if d%time.Hour != 0 {
			h++
		}
		return int64(h)
	case "day":
		d := end.Sub(start)
		if d <= 0 {
			return 0
		}
		days := d / (24 * time.Hour)
		if d%(24*time.Hour) != 0 {
			days++
		}
		return int64(days)
	}
	return 0
}

func rentCents(pricePerUnit int64, units int64) int64 {
	if pricePerUnit < 0 || units < 0 {
		return 0
	}
	return pricePerUnit * units
}

func operatorCents(snap ListingSnapshot, hours int64, withOperator bool) int64 {
	if !withOperator {
		return 0
	}
	if snap.Operator.HourlyRateCents <= 0 {
		return 0
	}
	h := hours
	if int64(snap.Operator.MinHours) > h {
		h = int64(snap.Operator.MinHours)
	}
	return snap.Operator.HourlyRateCents * h
}

func applyCommission(base, bps int64) int64 {
	if base <= 0 || bps <= 0 {
		return 0
	}
	return (base * bps) / 10000
}

func splitLiquids(rent, operatorTotal int64, snap ListingSnapshot, commission int64) (owner, op int64) {
	if rent < 0 || operatorTotal < 0 || commission < 0 {
		return 0, 0
	}
	if snap.Operator.IsOwner {
		return rent + operatorTotal - commission, 0
	}
	base := rent + operatorTotal
	if base == 0 {
		return 0, 0
	}
	commRent := (rent * commission) / base
	commOp := commission - commRent
	return rent - commRent, operatorTotal - commOp
}

// ApplyToRental copies the MoneyBreakdown fields onto the Rental.
func (b MoneyBreakdown) ApplyToRental(r *Rental) {
	r.RentCents = b.RentCents
	r.OperatorCents = b.OperatorCents
	r.DepositCents = b.DepositCents
	r.CommissionCents = b.CommissionCents
	r.OwnerPayoutCents = b.OwnerPayoutCents
	r.OperatorPayoutCents = b.OperatorPayoutCents
}

// ReceiptFromRental builds the tenant-facing receipt from a rental.
func ReceiptFromRental(r Rental, b MoneyBreakdown) Receipt {
	return Receipt{
		RentalID:            r.ID,
		TenantAccountID:     r.TenantAccountID,
		RentCents:           b.RentCents,
		OperatorCents:       b.OperatorCents,
		DepositCents:        b.DepositCents,
		TotalCents:          b.TotalCents,
		CommissionBaseCents: b.CommissionBaseCents,
		CommissionCents:     b.CommissionCents,
		OwnerPayoutCents:    b.OwnerPayoutCents,
		OperatorPayoutCents: b.OperatorPayoutCents,
		ListingSnapshot:     r.ListingSnapshot,
		WindowStartsAt:      r.StartsAt,
		WindowEndsAt:        r.EndsAt,
	}
}

// Receipt is the tenant-visible write-once snapshot.
type Receipt struct {
	RentalID            string          `json:"rental_id"`
	TenantAccountID     string          `json:"tenant_account_id"`
	RentCents           int64           `json:"rent_cents"`
	OperatorCents       int64           `json:"operator_cents"`
	DepositCents        int64           `json:"deposit_cents"`
	TotalCents          int64           `json:"total_cents"`
	CommissionBaseCents int64           `json:"commission_base_cents"`
	CommissionCents     int64           `json:"commission_cents"`
	OwnerPayoutCents    int64           `json:"owner_payout_cents"`
	OperatorPayoutCents int64           `json:"operator_payout_cents"`
	ListingSnapshot     ListingSnapshot `json:"listing_snapshot"`
	WindowStartsAt      time.Time       `json:"window_starts_at"`
	WindowEndsAt        time.Time       `json:"window_ends_at"`
	IssuedAt            time.Time       `json:"issued_at"`
}

func effectiveBPS(bps int64) int64 {
	if bps <= 0 {
		return DefaultCommissionBPS
	}
	return bps
}

// errPriceNotImplemented is a sentinel kept for future pricing modes.
var errPriceNotImplemented = errors.New("rental: pricing mode not implemented")
