package testutil

import (
	"github.com/shiroha-a/mk/internal/model"
)

// MockUserRepository is a test double for repository.UserRepository.
type MockUserRepository struct {
	Users                 map[string]*model.User        // keyed by ID
	Tokens                map[string]*model.User        // keyed by token
	Profiles              map[string]*model.UserProfile // keyed by userID
	FindByUsernameLowerFn func(username string, host *string) (*model.User, error)
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		Users:    make(map[string]*model.User),
		Tokens:   make(map[string]*model.User),
		Profiles: make(map[string]*model.UserProfile),
	}
}

func (m *MockUserRepository) FindByID(id string) (*model.User, error) {
	u, ok := m.Users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *MockUserRepository) FindByToken(token string) (*model.User, error) {
	u, ok := m.Tokens[token]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *MockUserRepository) FindByUsernameLower(username string, host *string) (*model.User, error) {
	if m.FindByUsernameLowerFn != nil {
		return m.FindByUsernameLowerFn(username, host)
	}
	for _, u := range m.Users {
		if u.UsernameLower == username {
			if host == nil && u.Host == nil {
				return u, nil
			}
			if host != nil && u.Host != nil && *host == *u.Host {
				return u, nil
			}
		}
	}
	return nil, ErrNotFound
}

func (m *MockUserRepository) FindProfileByUserID(userID string) (*model.UserProfile, error) {
	p, ok := m.Profiles[userID]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (m *MockUserRepository) IncrementFollowingCount(userID string, delta int) error {
	if u, ok := m.Users[userID]; ok {
		u.FollowingCount += delta
	}
	return nil
}

func (m *MockUserRepository) IncrementFollowersCount(userID string, delta int) error {
	if u, ok := m.Users[userID]; ok {
		u.FollowersCount += delta
	}
	return nil
}

// MockNoteRepository is a test double for repository.NoteRepository.
type MockNoteRepository struct {
	Notes map[string]*model.Note
}

func NewMockNoteRepository() *MockNoteRepository {
	return &MockNoteRepository{Notes: make(map[string]*model.Note)}
}

func (m *MockNoteRepository) Create(note *model.Note) error {
	m.Notes[note.ID] = note
	return nil
}

func (m *MockNoteRepository) FindByID(id string) (*model.Note, error) {
	n, ok := m.Notes[id]
	if !ok {
		return nil, ErrNotFound
	}
	return n, nil
}

func (m *MockNoteRepository) FindByIDWithUser(id string) (*model.Note, error) {
	return m.FindByID(id)
}

func (m *MockNoteRepository) Delete(note *model.Note) error {
	delete(m.Notes, note.ID)
	return nil
}

func (m *MockNoteRepository) Update(note *model.Note, column string, value any) error {
	return nil
}

// MockMetaRepository is a test double for repository.MetaRepository.
type MockMetaRepository struct {
	Meta *model.Meta
}

func NewMockMetaRepository() *MockMetaRepository {
	return &MockMetaRepository{}
}

func (m *MockMetaRepository) Fetch() (*model.Meta, error) {
	if m.Meta == nil {
		return nil, ErrNotFound
	}
	return m.Meta, nil
}

// MockAccessTokenRepository is a test double for repository.AccessTokenRepository.
type MockAccessTokenRepository struct {
	Tokens map[string]*model.AccessToken // keyed by hash
}

func NewMockAccessTokenRepository() *MockAccessTokenRepository {
	return &MockAccessTokenRepository{Tokens: make(map[string]*model.AccessToken)}
}

func (m *MockAccessTokenRepository) FindByHash(hash string) (*model.AccessToken, error) {
	t, ok := m.Tokens[hash]
	if !ok {
		return nil, ErrNotFound
	}
	return t, nil
}

// MockPollRepository is a test double for repository.PollRepository.
type MockPollRepository struct {
	Polls map[string]*model.Poll
}

func NewMockPollRepository() *MockPollRepository {
	return &MockPollRepository{Polls: make(map[string]*model.Poll)}
}

func (m *MockPollRepository) Create(poll *model.Poll) error {
	m.Polls[poll.NoteID] = poll
	return nil
}

// MockFollowingRepository is a test double for repository.FollowingRepository.
type MockFollowingRepository struct {
	Followings map[string]*model.Following // keyed by ID
}

func NewMockFollowingRepository() *MockFollowingRepository {
	return &MockFollowingRepository{Followings: make(map[string]*model.Following)}
}

func (m *MockFollowingRepository) Create(f *model.Following) error {
	m.Followings[f.ID] = f
	return nil
}

func (m *MockFollowingRepository) Delete(f *model.Following) error {
	delete(m.Followings, f.ID)
	return nil
}

func (m *MockFollowingRepository) FindByPair(followerID, followeeID string) (*model.Following, error) {
	for _, f := range m.Followings {
		if f.FollowerID == followerID && f.FolloweeID == followeeID {
			return f, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockFollowingRepository) Exists(followerID, followeeID string) (bool, error) {
	_, err := m.FindByPair(followerID, followeeID)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (m *MockFollowingRepository) ListFollowers(userID string, limit, offset int) ([]*model.Following, error) {
	var rows []*model.Following
	for _, f := range m.Followings {
		if f.FolloweeID == userID {
			rows = append(rows, f)
		}
	}
	return paginate(rows, limit, offset), nil
}

func (m *MockFollowingRepository) ListFollowing(userID string, limit, offset int) ([]*model.Following, error) {
	var rows []*model.Following
	for _, f := range m.Followings {
		if f.FollowerID == userID {
			rows = append(rows, f)
		}
	}
	return paginate(rows, limit, offset), nil
}

// MockFollowRequestRepository is a test double for repository.FollowRequestRepository.
type MockFollowRequestRepository struct {
	Requests map[string]*model.FollowRequest
}

func NewMockFollowRequestRepository() *MockFollowRequestRepository {
	return &MockFollowRequestRepository{Requests: make(map[string]*model.FollowRequest)}
}

func (m *MockFollowRequestRepository) Create(r *model.FollowRequest) error {
	m.Requests[r.ID] = r
	return nil
}

func (m *MockFollowRequestRepository) Delete(r *model.FollowRequest) error {
	delete(m.Requests, r.ID)
	return nil
}

func (m *MockFollowRequestRepository) FindByPair(followerID, followeeID string) (*model.FollowRequest, error) {
	for _, r := range m.Requests {
		if r.FollowerID == followerID && r.FolloweeID == followeeID {
			return r, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockFollowRequestRepository) Exists(followerID, followeeID string) (bool, error) {
	_, err := m.FindByPair(followerID, followeeID)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (m *MockFollowRequestRepository) ListReceived(userID string, limit, offset int) ([]*model.FollowRequest, error) {
	var rows []*model.FollowRequest
	for _, r := range m.Requests {
		if r.FolloweeID == userID {
			rows = append(rows, r)
		}
	}
	return paginateRequests(rows, limit, offset), nil
}

func (m *MockFollowRequestRepository) ListSent(userID string, limit, offset int) ([]*model.FollowRequest, error) {
	var rows []*model.FollowRequest
	for _, r := range m.Requests {
		if r.FollowerID == userID {
			rows = append(rows, r)
		}
	}
	return paginateRequests(rows, limit, offset), nil
}

func paginate(rows []*model.Following, limit, offset int) []*model.Following {
	if offset >= len(rows) {
		return nil
	}
	end := min(offset+limit, len(rows))
	return rows[offset:end]
}

func paginateRequests(rows []*model.FollowRequest, limit, offset int) []*model.FollowRequest {
	if offset >= len(rows) {
		return nil
	}
	end := min(offset+limit, len(rows))
	return rows[offset:end]
}
