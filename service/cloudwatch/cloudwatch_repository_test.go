package cloudwatch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSingleWindow(t *testing.T) {
	now := time.Date(2026, 11, 12, 18, 37, 42, 0, time.UTC)

	// Recent data: minute granularity, period spans the window exactly.
	start, end, period, err := SingleWindow(now.Add(-24*time.Hour), now, now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 11, 11, 18, 37, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2026, 11, 12, 18, 37, 0, 0, time.UTC), end)
	assert.Equal(t, int32(24*3600), period)

	// Older than 15 days: 5-minute units.
	start, end, period, err = SingleWindow(now.Add(-30*24*time.Hour), now, now)
	require.NoError(t, err)
	assert.Zero(t, start.Minute()%5)
	assert.Zero(t, end.Minute()%5)
	assert.Zero(t, period%300)

	// Older than 63 days: hourly units.
	_, _, period, err = SingleWindow(now.Add(-90*24*time.Hour), now, now)
	require.NoError(t, err)
	assert.Zero(t, period%3600)

	// A window shorter than the unit still measures one unit.
	_, _, period, err = SingleWindow(now.Add(-10*time.Second), now, now)
	require.NoError(t, err)
	assert.Equal(t, int32(60), period)

	_, _, _, err = SingleWindow(now, now.Add(-time.Hour), now)
	assert.Error(t, err)
}
