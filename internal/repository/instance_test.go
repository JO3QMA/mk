package repository

import (
	"context"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupInstance(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "instance" WHERE id = ?`, id)
}

func newTestInstance(id, host string) *model.Instance {
	return &model.Instance{
		ID:               id,
		Host:             host,
		FirstRetrievedAt: time.Now(),
		SuspensionState:  model.SuspensionStateNone,
	}
}

func TestInstanceRepository_CreateAndFindByHost(t *testing.T) {
	repo := NewInstanceRepository(testDB)
	inst := newTestInstance("i_ir_1", "alpha.example")
	require.NoError(t, repo.Create(inst))
	defer cleanupInstance(t, inst.ID)

	got, err := repo.FindByHost("alpha.example")
	require.NoError(t, err)
	assert.Equal(t, "alpha.example", got.Host)
}

func TestInstanceRepository_FindByHost_NotFound(t *testing.T) {
	repo := NewInstanceRepository(testDB)
	_, err := repo.FindByHost("missing.example")
	assert.Error(t, err)
}

func TestInstanceRepository_FindManyByHosts(t *testing.T) {
	repo := NewInstanceRepository(testDB)
	inst1 := newTestInstance("i_fmbh_1", "fmbh1.example")
	inst2 := newTestInstance("i_fmbh_2", "fmbh2.example")
	inst3 := newTestInstance("i_fmbh_3", "fmbh3.example")
	require.NoError(t, repo.Create(inst1))
	defer cleanupInstance(t, inst1.ID)
	require.NoError(t, repo.Create(inst2))
	defer cleanupInstance(t, inst2.ID)
	require.NoError(t, repo.Create(inst3))
	defer cleanupInstance(t, inst3.ID)

	// batch fetch 2 existing + 1 missing host
	got, err := repo.FindManyByHosts([]string{"fmbh1.example", "fmbh3.example", "missing.example"})
	require.NoError(t, err)
	require.Len(t, got, 2)
	hosts := []string{got[0].Host, got[1].Host}
	assert.ElementsMatch(t, []string{"fmbh1.example", "fmbh3.example"}, hosts)
}

func TestInstanceRepository_FindManyByHosts_Empty(t *testing.T) {
	repo := NewInstanceRepository(testDB)
	got, err := repo.FindManyByHosts(nil)
	require.NoError(t, err)
	assert.Nil(t, got)

	got, err = repo.FindManyByHosts([]string{})
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestInstanceRepository_UpdateFields(t *testing.T) {
	repo := NewInstanceRepository(testDB)
	inst := newTestInstance("i_ir_2", "beta.example")
	require.NoError(t, repo.Create(inst))
	defer cleanupInstance(t, inst.ID)

	name := "Beta"
	desc := "Beta instance"
	require.NoError(t, repo.UpdateFields("beta.example", map[string]any{
		"name":        &name,
		"description": &desc,
	}))

	got, err := repo.FindByHost("beta.example")
	require.NoError(t, err)
	require.NotNil(t, got.Name)
	assert.Equal(t, "Beta", *got.Name)
	require.NotNil(t, got.Description)
	assert.Equal(t, "Beta instance", *got.Description)
}

func TestInstanceRepository_UpdateFields_NoOp(t *testing.T) {
	repo := NewInstanceRepository(testDB)
	require.NoError(t, repo.UpdateFields("any.example", nil))
}

func TestInstanceRepository_IncrementCount(t *testing.T) {
	repo := NewInstanceRepository(testDB)
	inst := newTestInstance("i_ir_3", "gamma.example")
	require.NoError(t, repo.Create(inst))
	defer cleanupInstance(t, inst.ID)

	require.NoError(t, repo.IncrementCount("gamma.example", "usersCount", 3))
	require.NoError(t, repo.IncrementCount("gamma.example", "usersCount", -1))

	got, err := repo.FindByHost("gamma.example")
	require.NoError(t, err)
	assert.Equal(t, 2, got.UsersCount)
}

func TestInstanceRepository_List_Filters(t *testing.T) {
	repo := NewInstanceRepository(testDB)
	a := newTestInstance("i_ir_4a", "list-a.example")
	a.UsersCount = 5
	a.NotesCount = 100
	a.FollowingCount = 1
	b := newTestInstance("i_ir_4b", "list-b.example")
	b.UsersCount = 1
	b.NotesCount = 10
	b.FollowersCount = 1
	b.IsNotResponding = true
	c := newTestInstance("i_ir_4c", "list-c.example")
	c.SuspensionState = model.SuspensionStateManuallySuspended

	for _, inst := range []*model.Instance{a, b, c} {
		require.NoError(t, repo.Create(inst))
		defer cleanupInstance(t, inst.ID)
	}

	rows, err := repo.List(model.InstanceListFilter{Host: "list-"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(rows), 3)

	suspendedTrue := true
	rows, err = repo.List(model.InstanceListFilter{Host: "list-", Suspended: &suspendedTrue})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "list-c.example", rows[0].Host)

	suspendedFalse := false
	rows, err = repo.List(model.InstanceListFilter{Host: "list-", Suspended: &suspendedFalse})
	require.NoError(t, err)
	assert.Len(t, rows, 2)

	notRespTrue := true
	rows, err = repo.List(model.InstanceListFilter{Host: "list-", NotResponding: &notRespTrue})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "list-b.example", rows[0].Host)

	federating := true
	rows, err = repo.List(model.InstanceListFilter{Host: "list-", Federating: &federating})
	require.NoError(t, err)
	assert.Len(t, rows, 2) // a と b

	notFederating := false
	rows, err = repo.List(model.InstanceListFilter{Host: "list-", Federating: &notFederating})
	require.NoError(t, err)
	assert.Len(t, rows, 1) // c (followingCount=0 AND followersCount=0)
	assert.Equal(t, "list-c.example", rows[0].Host)

	subscribing := true
	rows, err = repo.List(model.InstanceListFilter{Host: "list-", Subscribing: &subscribing})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "list-b.example", rows[0].Host)

	notSubscribing := false
	rows, err = repo.List(model.InstanceListFilter{Host: "list-", Subscribing: &notSubscribing})
	require.NoError(t, err)
	assert.Len(t, rows, 2) // a と c

	publishing := true
	rows, err = repo.List(model.InstanceListFilter{Host: "list-", Publishing: &publishing})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "list-a.example", rows[0].Host)

	notPublishing := false
	rows, err = repo.List(model.InstanceListFilter{Host: "list-", Publishing: &notPublishing})
	require.NoError(t, err)
	assert.Len(t, rows, 2) // b と c
}

func TestInstanceRepository_List_Sort(t *testing.T) {
	repo := NewInstanceRepository(testDB)
	a := newTestInstance("i_ir_5a", "sort-a.example")
	a.UsersCount = 1
	a.NotesCount = 1
	b := newTestInstance("i_ir_5b", "sort-b.example")
	b.UsersCount = 5
	b.NotesCount = 5
	for _, inst := range []*model.Instance{a, b} {
		require.NoError(t, repo.Create(inst))
		defer cleanupInstance(t, inst.ID)
	}

	for _, sortBy := range []string{
		"+host", "-host", "+notes", "-notes", "+users", "-users",
		"+firstRetrievedAt", "",
	} {
		rows, err := repo.List(model.InstanceListFilter{Host: "sort-", SortBy: sortBy, Limit: 10})
		require.NoError(t, err)
		assert.Len(t, rows, 2)
	}
}

func TestInstanceRepository_List_LimitClamp(t *testing.T) {
	repo := NewInstanceRepository(testDB)
	rows, err := repo.List(model.InstanceListFilter{Limit: 9999})
	require.NoError(t, err)
	assert.NotNil(t, rows)

	rows, err = repo.List(model.InstanceListFilter{Limit: -10})
	require.NoError(t, err)
	assert.NotNil(t, rows)
}

func TestInstanceRepository_List_QueryError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db := testDB.WithContext(ctx)
	repo := NewInstanceRepository(db)
	_, err := repo.List(model.InstanceListFilter{})
	assert.Error(t, err)
}
