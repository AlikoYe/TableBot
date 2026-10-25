package bot

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/AlikoYe/TableBot/backend/internal/booking"
)

// Every user-facing text the bot sends lives in this file.

const (
	textWelcome      = "👋 Welcome to TableBot!\nBook a table at restaurants, cafes, billiards and more in a few taps."
	textMenuButton   = "Open TableBot"
	textOpenApp      = "📱 Open TableBot"
	textConfirm      = "✅ Confirm"
	textReject       = "❌ Reject"
	textOutcomeOK    = "✅ Confirmed"
	textOutcomeNo    = "❌ Rejected"
	textNotPending   = "⚠️ This booking is no longer pending."
	textForbidden    = "⛔ Only the venue owner can do this."
	textNotFound     = "⚠️ Booking not found."
	textTryAgain     = "⚠️ Something went wrong, please try again."
	textNoLongerOpen = "⚠️ No longer pending"
)

// timeRange formats a booking's slot in its venue time zone, e.g. "Tue 27 Oct, 19:00–21:00".
func timeRange(b booking.Booking) string {
	return b.StartsAt.Format("Mon 2 Jan, 15:04") + "–" + b.EndsAt.Format("15:04")
}

func guestLine(b booking.Booking) string {
	name := b.GuestInfo.Name
	if name == "" {
		name = "Guest"
	}
	if b.GuestInfo.Username != nil && *b.GuestInfo.Username != "" {
		name += " (@" + *b.GuestInfo.Username + ")"
	}
	return name
}

func guestsWord(n int) string {
	if n == 1 {
		return "1 guest"
	}
	return fmt.Sprintf("%d guests", n)
}

func place(b booking.Booking) string {
	return fmt.Sprintf("🏠 %s · table %s", b.Venue.Name, b.Table.Label)
}

// textNewBooking is sent to the owner when a guest requests a table.
func textNewBooking(b booking.Booking, now time.Time) string {
	lines := []string{
		"🆕 New booking request",
		place(b),
		"👥 " + guestsWord(b.Guests),
		"🕖 " + timeRange(b),
		"🙋 " + guestLine(b),
	}
	if b.SpecialRequest != "" {
		lines = append(lines, "💬 "+b.SpecialRequest)
	}
	if b.ExpiresAt != nil {
		mins := int(math.Ceil(b.ExpiresAt.Sub(now).Minutes()))
		lines = append(lines, fmt.Sprintf("⏳ Expires in %d min", max(mins, 0)))
	}
	return strings.Join(lines, "\n")
}

// textDecidedForOwner replaces the owner's request message after a decision.
func textDecidedForOwner(b booking.Booking, confirmed bool) string {
	outcome := textOutcomeNo
	if confirmed {
		outcome = textOutcomeOK
	}
	return strings.Join([]string{
		"📋 Booking request",
		place(b),
		"👥 " + guestsWord(b.Guests),
		"🕖 " + timeRange(b),
		"🙋 " + guestLine(b),
		"",
		outcome,
	}, "\n")
}

func textCancelledForOwner(b booking.Booking) string {
	return strings.Join([]string{
		"🚫 The guest cancelled a booking",
		place(b),
		"🕖 " + timeRange(b),
		"🙋 " + guestLine(b),
	}, "\n")
}

func textConfirmedForGuest(b booking.Booking) string {
	return strings.Join([]string{
		"✅ Your booking is confirmed!",
		place(b),
		"👥 " + guestsWord(b.Guests),
		"🕖 " + timeRange(b),
	}, "\n")
}

func textRejectedForGuest(b booking.Booking) string {
	return fmt.Sprintf("❌ Sorry, %s can't take your booking for %s.\nPlease try another time or table.", b.Venue.Name, timeRange(b))
}

func textExpiredForGuest(b booking.Booking) string {
	return fmt.Sprintf("⌛ %s did not respond in time, so your request for %s has expired.\nPlease try booking again.", b.Venue.Name, timeRange(b))
}

func textReminderForGuest(b booking.Booking) string {
	return strings.Join([]string{
		"⏰ Reminder: your table is booked soon",
		place(b),
		"👥 " + guestsWord(b.Guests),
		"🕖 " + timeRange(b),
	}, "\n")
}
