package postgres

import (
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

func toUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

func fromUUID(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuidBytesToString(u.Bytes)
}

func uuidBytesToString(b [16]byte) string {
	const hex = "0123456789abcdef"
	buf := make([]byte, 36)
	pos := 0
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			buf[pos] = '-'
			pos++
		}
		buf[pos] = hex[v>>4]
		buf[pos+1] = hex[v&0x0f]
		pos += 2
	}
	return string(buf)
}

func toNumeric(d decimal.Decimal) pgtype.Numeric {
	coefficient := d.Coefficient()
	exp := int32(d.Exponent())
	return pgtype.Numeric{
		Int:   coefficient,
		Exp:   exp,
		Valid: true,
	}
}

func fromNumeric(n pgtype.Numeric) decimal.Decimal {
	if !n.Valid || n.Int == nil {
		return decimal.Zero
	}
	return decimal.NewFromBigInt(new(big.Int).Set(n.Int), n.Exp)
}

func toText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func fromText(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func toTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func fromTimestamptz(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func toDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

func fromDate(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}
