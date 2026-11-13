package cloudtrail

import (
	"context"
	stderrors "errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/service"
)

const testPageSize = 50

// fakeLookup serves pages of testPageSize events, failing the request for page failPage (0 = never)
// and taking delay per request.
type fakeLookup struct {
	pages    int
	failPage int
	delay    time.Duration

	mu         sync.Mutex
	calls      int
	maxResults []int32
}

func (f *fakeLookup) LookupEvents(ctx context.Context, in *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	f.mu.Lock()
	f.calls++
	f.maxResults = append(f.maxResults, aws.ToInt32(in.MaxResults))
	f.mu.Unlock()

	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	page := 1
	if in.NextToken != nil {
		page, _ = strconv.Atoi(*in.NextToken)
	}

	if page == f.failPage {
		return nil, fmt.Errorf("page %d failed", page)
	}

	out := &cloudtrail.LookupEventsOutput{}
	for i := range testPageSize {
		id := fmt.Sprintf("p%d-e%d", page, i)
		out.Events = append(out.Events, types.Event{
			EventId:         aws.String(id),
			CloudTrailEvent: aws.String(fmt.Sprintf(`{"eventID":%q}`, id)),
		})
	}

	if page < f.pages {
		out.NextToken = aws.String(strconv.Itoa(page + 1))
	}

	return out, nil
}

func (f *fakeLookup) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.calls
}

func newTestRepository(ctx context.Context, api cloudtrail.LookupEventsAPIClient) *CloudTrailRepository {
	return &CloudTrailRepository{ctx: ctx, client: &v3.Client{}, lookupClient: api}
}

// A whole page arrives in one burst; a reader that is busy between receives must still get all of it.
func TestListEventsByInputAsyncDeliversEveryEventToSlowReader(t *testing.T) {
	api := &fakeLookup{pages: 3}
	source, errChan := newTestRepository(context.Background(), api).
		ListEventsByInputAsync(&cloudtrail.LookupEventsInput{})

	seen := map[string]bool{}
	for e := range source {
		time.Sleep(100 * time.Microsecond)
		seen[e.GetId()] = true
	}

	if err, ok := <-errChan; ok {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := 3 * testPageSize; len(seen) != want {
		t.Fatalf("received %d distinct events, want %d", len(seen), want)
	}
}

func TestListEventsByInputStopsOnPageError(t *testing.T) {
	api := &fakeLookup{pages: 5, failPage: 2}

	done := make(chan struct{})
	var events []Event
	var err error
	go func() {
		defer close(done)
		events, err = newTestRepository(context.Background(), api).
			ListEventsByInput(&cloudtrail.LookupEventsInput{})
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("lookup did not finish after a page error; LookupEvents called %d times", api.callCount())
	}

	if err == nil {
		t.Fatal("expected the page error to be returned")
	}

	if len(events) != testPageSize {
		t.Fatalf("received %d events, want the %d from the page before the failure", len(events), testPageSize)
	}

	if calls := api.callCount(); calls != 2 {
		t.Fatalf("LookupEvents called %d times, want 2", calls)
	}
}

// A reader that only ranges over events must not hang when the lookup fails.
func TestListEventsByInputAsyncErrorDoesNotBlockReaderIgnoringErrors(t *testing.T) {
	api := &fakeLookup{pages: 5, failPage: 2}
	source, _ := newTestRepository(context.Background(), api).
		ListEventsByInputAsync(&cloudtrail.LookupEventsInput{})

	done := make(chan int)
	go func() {
		n := 0
		for range source {
			n++
		}
		done <- n
	}()

	select {
	case n := <-done:
		if n != testPageSize {
			t.Fatalf("received %d events, want %d", n, testPageSize)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("event channel was never closed")
	}
}

func TestListEventsByInputAsyncStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// slow pages, so the producer is still paging when the reader cancels
	api := &fakeLookup{pages: 1000, delay: 20 * time.Millisecond}
	source, _ := newTestRepository(ctx, api).
		ListEventsByInputAsync(&cloudtrail.LookupEventsInput{})

	<-source
	cancel()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-source:
			if !ok {
				if calls := api.callCount(); calls > 3 {
					t.Fatalf("LookupEvents called %d times, want paging to stop at cancel", calls)
				}
				return
			}
		case <-deadline:
			t.Fatal("event channel was never closed after cancel")
		}
	}
}

// A reader that falls more than a buffer behind gets an error, not a list that only looks complete.
func TestListEventsByInputAsyncReportsOverflow(t *testing.T) {
	fullPages := EventsBufferSize / testPageSize
	api := &fakeLookup{pages: fullPages + 5}

	source, errChan := newTestRepository(context.Background(), api).
		ListEventsByInputAsync(&cloudtrail.LookupEventsInput{})

	// read nothing until the producer gives up
	select {
	case err, ok := <-errChan:
		if !ok || err == nil {
			t.Fatal("expected an overflow error, got a clean close")
		}
		if !stderrors.Is(err, service.ErrChannelFull) {
			t.Fatalf("error %v does not wrap service.ErrChannelFull", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("producer neither finished nor reported the overflow")
	}

	n := 0
	for range source {
		n++
	}

	if n != EventsBufferSize {
		t.Fatalf("buffered %d events, want %d", n, EventsBufferSize)
	}

	if calls := api.callCount(); calls != fullPages+1 {
		t.Fatalf("LookupEvents called %d times, want %d", calls, fullPages+1)
	}
}

func TestListEventsByLookupAsyncInvalidLookupClosesEvents(t *testing.T) {
	lookup := NewLookupMiddleware().WithHandler(func(q *cloudtrail.LookupEventsInput) (*cloudtrail.LookupEventsInput, error) {
		return q, fmt.Errorf("invalid lookup")
	})

	source, errChan := newTestRepository(context.Background(), &fakeLookup{}).ListEventsByLookupAsync(lookup)

	select {
	case _, ok := <-source:
		if ok {
			t.Fatal("expected no events for an invalid lookup")
		}
	case <-time.After(time.Second):
		t.Fatal("event channel for an invalid lookup is not closed")
	}

	if err, ok := <-errChan; !ok || err == nil {
		t.Fatal("expected the lookup error")
	}
}

// LookupEvents rejects a page size above 50, so a larger MaxResults is a total spread over pages.
func TestListEventsByInputLimitAbovePageSize(t *testing.T) {
	api := &fakeLookup{pages: 10}
	query := &cloudtrail.LookupEventsInput{MaxResults: aws.Int32(120)}

	events, err := newTestRepository(context.Background(), api).ListEventsByInput(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 120 {
		t.Fatalf("received %d events, want 120", len(events))
	}

	if calls := api.callCount(); calls != 3 {
		t.Fatalf("LookupEvents called %d times, want 3", calls)
	}

	for i, size := range api.maxResults {
		if size != LookupEventsMaxPageSize {
			t.Fatalf("request %d asked for a page of %d, want %d", i, size, LookupEventsMaxPageSize)
		}
	}

	if aws.ToInt32(query.MaxResults) != 120 {
		t.Fatalf("caller's query was modified: MaxResults is %d", aws.ToInt32(query.MaxResults))
	}
}

// CloudTrail takes a single lookup attribute. WithResource on a lookup that already has one drops
// the resource filter without an error, by design, and keeps the attribute set first.
func TestLookupWithResourceDroppedWhenAttributeSet(t *testing.T) {
	lookup := NewLookupMiddleware().
		WithEventName("RunInstances").
		WithResource(NewEvent(&v3.Client{}, types.Event{EventId: aws.String("i-123")}, CloudTrailEvent{}))

	if errs, ok := lookup.Errors(); !ok {
		t.Fatalf("WithResource reported %v, want the resource filter dropped silently", errs)
	}

	attrs := lookup.Get().LookupAttributes
	if len(attrs) != 1 || attrs[0].AttributeKey != types.LookupAttributeKeyEventName {
		t.Fatalf("lookup holds %v, want only the event name attribute", attrs)
	}
}

func TestEventWithoutReadOnly(t *testing.T) {
	e := NewEvent(&v3.Client{}, types.Event{EventId: aws.String("e1")}, CloudTrailEvent{})

	if e.IsReadOnly() || e.GetReadOnly() != "" {
		t.Fatalf("event without ReadOnly reported %q", e.GetReadOnly())
	}
}
