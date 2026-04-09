package charts

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApRequestChart_AllCounters(t *testing.T) {
	engine, repo, _ := newTestEngine(t, SchemaApRequest())
	ac := NewApRequestChart(engine)
	require.Same(t, engine, ac.Chart())

	require.NoError(t, ac.DeliverSucceeded())
	require.NoError(t, ac.DeliverSucceeded())
	require.NoError(t, ac.DeliverFailed())
	require.NoError(t, ac.Inbox())
	require.NoError(t, ac.Inbox())
	require.NoError(t, ac.Inbox())
	require.NoError(t, engine.Save(context.Background()))

	row := repo.hour[""][0]
	assert.Equal(t, int64(2), toInt64(row.Cols["deliverSucceeded"]))
	assert.Equal(t, int64(1), toInt64(row.Cols["deliverFailed"]))
	assert.Equal(t, int64(3), toInt64(row.Cols["inboxReceived"]))
}
