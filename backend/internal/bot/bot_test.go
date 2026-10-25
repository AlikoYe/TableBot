package bot

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/AlikoYe/TableBot/backend/internal/booking"
)

type sent struct {
	chatID int64
	text   string
	markup models.ReplyMarkup
}

// fakeSender records calls instead of talking to Telegram.
type fakeSender struct {
	mu      sync.Mutex
	sends   []sent
	edits   []sent
	answers []tgbot.AnswerCallbackQueryParams
	err     error
}

func (f *fakeSender) SendMessage(_ context.Context, p *tgbot.SendMessageParams) (*models.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sends = append(f.sends, sent{chatID: p.ChatID.(int64), text: p.Text, markup: p.ReplyMarkup})
	return &models.Message{}, f.err
}

func (f *fakeSender) EditMessageText(_ context.Context, p *tgbot.EditMessageTextParams) (*models.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.edits = append(f.edits, sent{chatID: p.ChatID.(int64), text: p.Text, markup: p.ReplyMarkup})
	return &models.Message{}, f.err
}

func (f *fakeSender) AnswerCallbackQuery(_ context.Context, p *tgbot.AnswerCallbackQueryParams) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answers = append(f.answers, *p)
	return true, f.err
}

var almaty, _ = time.LoadLocation("Asia/Almaty")

func sampleBooking() booking.Booking {
	start := time.Date(2026, 10, 27, 19, 0, 0, 0, almaty)
	exp := time.Date(2026, 10, 25, 18, 15, 0, 0, almaty)
	user := "aigerim"
	b := booking.Booking{
		ID: 42, Status: "pending", Guests: 4, StartsAt: start, EndsAt: start.Add(2 * time.Hour),
		ExpiresAt: &exp, SpecialRequest: "Window seat please",
		Venue: booking.VenueRef{ID: 1, Name: "Nomad Bistro"}, Table: booking.TableRef{ID: 3, Label: "T3", Seats: 4},
		UserID: 2, OwnerID: 1,
	}
	b.GuestInfo = booking.Guest{Name: "Aigerim S.", Username: &user}
	return b
}

func testNotifier(send Sender, logBuf *bytes.Buffer) *Notifier {
	n := NewNotifier(send, "https://app.example.com", slog.New(slog.NewTextHandler(logBuf, nil)))
	n.now = func() time.Time { return time.Date(2026, 10, 25, 18, 0, 0, 0, almaty) }
	return n
}

func TestTimeRange(t *testing.T) {
	if got := timeRange(sampleBooking()); got != "Tue 27 Oct, 19:00–21:00" {
		t.Fatalf("timeRange = %q", got)
	}
}

func TestBookingCreatedMessagesOwnerWithButtons(t *testing.T) {
	fs := &fakeSender{}
	n := testNotifier(fs, &bytes.Buffer{})
	if err := n.BookingCreated(context.Background(), sampleBooking()); err != nil {
		t.Fatal(err)
	}
	n.Wait()
	if len(fs.sends) != 1 || fs.sends[0].chatID != 1 {
		t.Fatalf("sends = %+v, want one message to owner 1", fs.sends)
	}
	text := fs.sends[0].text
	for _, want := range []string{"Nomad Bistro", "table T3", "4 guests", "Tue 27 Oct, 19:00–21:00", "Aigerim S. (@aigerim)", "Window seat please", "Expires in 15 min"} {
		if !strings.Contains(text, want) {
			t.Errorf("owner message missing %q:\n%s", want, text)
		}
	}
	kb := fs.sends[0].markup.(*models.InlineKeyboardMarkup)
	row := kb.InlineKeyboard[0]
	if row[0].Text != textConfirm || row[0].CallbackData != "bk:c:42" || row[1].Text != textReject || row[1].CallbackData != "bk:r:42" {
		t.Errorf("decision buttons = %+v", row)
	}
	if web := kb.InlineKeyboard[1][0]; web.WebApp == nil || web.WebApp.URL != "https://app.example.com" {
		t.Errorf("web app button = %+v", web)
	}
}

func TestGuestAndOwnerMessages(t *testing.T) {
	fs := &fakeSender{}
	n := testNotifier(fs, &bytes.Buffer{})
	ctx := context.Background()
	b := sampleBooking()

	confirmed := b
	confirmed.Status = "confirmed"
	rejected := b
	rejected.Status = "rejected"
	// Wait after each call: background sends may otherwise arrive in any order.
	for _, send := range []func() error{
		func() error { return n.BookingDecided(ctx, confirmed) },
		func() error { return n.BookingDecided(ctx, rejected) },
		func() error { return n.BookingExpired(ctx, b) },
		func() error { return n.BookingReminder(ctx, confirmed) },
		func() error { return n.BookingCancelled(ctx, b) },
	} {
		_ = send()
		n.Wait()
	}

	want := []struct {
		chat int64
		text string
	}{
		{2, "confirmed"}, {2, "can't take your booking"}, {2, "did not respond in time"}, {2, "Reminder"}, {1, "cancelled"},
	}
	if len(fs.sends) != len(want) {
		t.Fatalf("got %d messages, want %d", len(fs.sends), len(want))
	}
	for i, w := range want {
		if fs.sends[i].chatID != w.chat || !strings.Contains(fs.sends[i].text, w.text) {
			t.Errorf("message %d = to %d %q, want to %d containing %q", i, fs.sends[i].chatID, fs.sends[i].text, w.chat, w.text)
		}
	}
}

func TestNoOwnerMeansNoMessage(t *testing.T) {
	fs := &fakeSender{}
	b := sampleBooking()
	b.OwnerID = 0
	n := testNotifier(fs, &bytes.Buffer{})
	_ = n.BookingCreated(context.Background(), b)
	n.Wait()
	if len(fs.sends) != 0 {
		t.Fatalf("sent %d messages for a venue without owner", len(fs.sends))
	}
}

func TestSendErrorsAreSwallowedAndLogged(t *testing.T) {
	fs := &fakeSender{err: errors.New("Forbidden: bot was blocked by the user")}
	var logs bytes.Buffer
	n := testNotifier(fs, &logs)
	if err := n.BookingCreated(context.Background(), sampleBooking()); err != nil {
		t.Fatalf("BookingCreated returned %v; send errors must be swallowed", err)
	}
	if err := n.BookingDecided(context.Background(), sampleBooking()); err != nil {
		t.Fatalf("BookingDecided returned %v", err)
	}
	n.Wait()
	out := logs.String()
	if strings.Count(out, "level=WARN") != 2 || !strings.Contains(out, "booking_id=42") || !strings.Contains(out, "blocked") {
		t.Fatalf("expected two warn lines with booking_id and the error, got:\n%s", out)
	}
}

// blockingSender holds every send until release is closed and records the context it got.
type blockingSender struct {
	fakeSender
	started chan struct{}
	release chan struct{}
	ctxErr  chan error
}

func (s *blockingSender) SendMessage(ctx context.Context, p *tgbot.SendMessageParams) (*models.Message, error) {
	close(s.started)
	select {
	case <-s.release:
	case <-ctx.Done():
	}
	err := ctx.Err()
	s.ctxErr <- err
	msg, _ := s.fakeSender.SendMessage(ctx, p)
	return msg, err // like a real client: a dead context is an error
}

func TestSendsDoNotBlockTheCaller(t *testing.T) {
	bs := &blockingSender{started: make(chan struct{}), release: make(chan struct{}), ctxErr: make(chan error, 1)}
	n := testNotifier(bs, &bytes.Buffer{})

	// A request context that is cancelled as soon as the handler returns.
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan struct{})
	go func() {
		_ = n.BookingCreated(ctx, sampleBooking())
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("BookingCreated blocked on a slow Telegram send")
	}
	cancel()
	<-bs.started

	close(bs.release)
	n.Wait()
	if err := <-bs.ctxErr; err != nil {
		t.Fatalf("send context was cancelled with the request: %v", err)
	}
	if len(bs.sends) != 1 {
		t.Fatalf("sends = %d, want 1", len(bs.sends))
	}
}

func TestSendTimeoutIsLogged(t *testing.T) {
	bs := &blockingSender{started: make(chan struct{}), release: make(chan struct{}), ctxErr: make(chan error, 1)}
	var logs bytes.Buffer
	n := testNotifier(bs, &logs)
	n.timeout = 20 * time.Millisecond

	_ = n.BookingDecided(context.Background(), sampleBooking())
	n.Wait()
	if err := <-bs.ctxErr; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("send ctx err = %v, want deadline exceeded", err)
	}
	if !strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), "booking_id=42") {
		t.Fatalf("timeout not logged:\n%s", logs.String())
	}
}

func TestStartRepliesWithWebAppButton(t *testing.T) {
	fs := &fakeSender{}
	b := NewWithSender(fs, nil, "https://app.example.com", slog.New(slog.DiscardHandler))
	b.handleStart(context.Background(), nil, &models.Update{Message: &models.Message{Chat: models.Chat{ID: 5}}})
	if len(fs.sends) != 1 || fs.sends[0].chatID != 5 || fs.sends[0].text != textWelcome {
		t.Fatalf("sends = %+v", fs.sends)
	}
	if got := buttons(fs.sends[0].markup); got != textOpenApp {
		t.Errorf("buttons = %q", got)
	}
}

func TestCallbackWithBadData(t *testing.T) {
	fs := &fakeSender{}
	b := NewWithSender(fs, nil, "", slog.New(slog.DiscardHandler))
	b.HandleCallback(context.Background(), &models.CallbackQuery{ID: "q", Data: "bk:x:1"})
	if len(fs.answers) != 1 || !fs.answers[0].ShowAlert {
		t.Fatalf("answers = %+v", fs.answers)
	}
}
