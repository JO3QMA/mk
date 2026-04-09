package testutil

import (
	"strings"
	"time"

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

func (m *MockUserRepository) Create(u *model.User) error {
	m.Users[u.ID] = u
	return nil
}

func (m *MockUserRepository) FindByID(id string) (*model.User, error) {
	u, ok := m.Users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *MockUserRepository) FindByURI(uri string) (*model.User, error) {
	for _, u := range m.Users {
		if u.URI != nil && *u.URI == uri {
			return u, nil
		}
	}
	return nil, ErrNotFound
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

func (m *MockUserRepository) SearchByUsername(query string, limit, offset int) ([]*model.User, error) {
	var matches []*model.User
	for _, u := range m.Users {
		if len(u.UsernameLower) >= len(query) && u.UsernameLower[:len(query)] == query {
			matches = append(matches, u)
		}
	}
	if offset >= len(matches) {
		return nil, nil
	}
	end := min(offset+limit, len(matches))
	return matches[offset:end], nil
}

func (m *MockUserRepository) UpdateUser(userID string, fields map[string]any) error {
	u, ok := m.Users[userID]
	if !ok {
		return ErrNotFound
	}
	applyUserFields(u, fields)
	return nil
}

func (m *MockUserRepository) CreateProfile(profile *model.UserProfile) error {
	m.Profiles[profile.UserID] = profile
	return nil
}

func (m *MockUserRepository) ListUsers(filter model.UserListFilter) ([]*model.User, error) {
	var result []*model.User
	for _, u := range m.Users {
		switch filter.Origin {
		case "local":
			if u.Host != nil {
				continue
			}
		case "remote":
			if u.Host == nil {
				continue
			}
		}
		switch filter.State {
		case "suspended":
			if !u.IsSuspended {
				continue
			}
		case "alive":
			if u.IsSuspended {
				continue
			}
		}
		result = append(result, u)
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 10
	}
	offset := filter.Offset
	if offset >= len(result) {
		return nil, nil
	}
	end := min(offset+limit, len(result))
	return result[offset:end], nil
}

func (m *MockUserRepository) UpdateProfile(userID string, fields map[string]any) error {
	p, ok := m.Profiles[userID]
	if !ok {
		// 既存プロフィールがなければ作成する(本物のDBではFK制約があるが、テストのモックでは緩い)
		p = &model.UserProfile{UserID: userID}
		m.Profiles[userID] = p
	}
	applyProfileFields(p, fields)
	return nil
}

// applyUserFields は単純な型の代表例にだけ対応する。新しいフィールドを使う場合はここに追加する。
func applyUserFields(u *model.User, fields map[string]any) {
	for k, v := range fields {
		switch k {
		case "name":
			if s, ok := v.(*string); ok {
				u.Name = s
			}
		case "inbox":
			if s, ok := v.(*string); ok {
				u.Inbox = s
			}
		case "sharedInbox":
			if s, ok := v.(*string); ok {
				u.SharedInbox = s
			}
		case "lastFetchedAt":
			if t, ok := v.(*time.Time); ok {
				u.LastFetchedAt = t
			}
		case "isLocked":
			if b, ok := v.(bool); ok {
				u.IsLocked = b
			}
		case "isBot":
			if b, ok := v.(bool); ok {
				u.IsBot = b
			}
		case "isCat":
			if b, ok := v.(bool); ok {
				u.IsCat = b
			}
		case "isExplorable":
			if b, ok := v.(bool); ok {
				u.IsExplorable = b
			}
		case "hideOnlineStatus":
			if b, ok := v.(bool); ok {
				u.HideOnlineStatus = b
			}
		case "isSuspended":
			if b, ok := v.(bool); ok {
				u.IsSuspended = b
			}
		case "isDeleted":
			if b, ok := v.(bool); ok {
				u.IsDeleted = b
			}
		case "token":
			if s, ok := v.(string); ok {
				u.Token = &s
			}
		}
	}
}

func applyProfileFields(p *model.UserProfile, fields map[string]any) {
	for k, v := range fields {
		switch k {
		case "description":
			if s, ok := v.(*string); ok {
				p.Description = s
			}
		case "location":
			if s, ok := v.(*string); ok {
				p.Location = s
			}
		case "birthday":
			if s, ok := v.(*string); ok {
				p.Birthday = s
			}
		case "lang":
			if s, ok := v.(*string); ok {
				p.Lang = s
			}
		case "alwaysMarkNsfw":
			if b, ok := v.(bool); ok {
				p.AlwaysMarkNsfw = b
			}
		case "autoSensitive":
			if b, ok := v.(bool); ok {
				p.AutoSensitive = b
			}
		case "noCrawle":
			if b, ok := v.(bool); ok {
				p.NoCrawle = b
			}
		case "preventAiLearning":
			if b, ok := v.(bool); ok {
				p.PreventAiLearning = b
			}
		case "password":
			if s, ok := v.(string); ok {
				p.Password = &s
			}
		}
	}
}

// MockNoteRepository is a test double for repository.NoteRepository.
type MockNoteRepository struct {
	Notes          map[string]*model.Note
	ReactionCounts map[string]map[string]int // noteID -> reaction -> count
}

func NewMockNoteRepository() *MockNoteRepository {
	return &MockNoteRepository{
		Notes:          make(map[string]*model.Note),
		ReactionCounts: make(map[string]map[string]int),
	}
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

func (m *MockNoteRepository) FindByURI(uri string) (*model.Note, error) {
	for _, n := range m.Notes {
		if n.URI != nil && *n.URI == uri {
			return n, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockNoteRepository) Delete(note *model.Note) error {
	delete(m.Notes, note.ID)
	return nil
}

func (m *MockNoteRepository) Update(note *model.Note, column string, value any) error {
	return nil
}

// UpdateFields applies field updates to the in-memory note. テストで参照される
// 列だけを反映する; 拡張時はここに追記する。
func (m *MockNoteRepository) UpdateFields(noteID string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	n, ok := m.Notes[noteID]
	if !ok {
		return ErrNotFound
	}
	for k, v := range fields {
		switch k {
		case "text":
			if s, ok := v.(*string); ok {
				n.Text = s
			}
		case "cw":
			if s, ok := v.(*string); ok {
				n.CW = s
			}
		case "mentions":
			if a, ok := v.([]string); ok {
				n.Mentions = a
			}
		}
	}
	return nil
}

// IncrementCount mutates the in-memory note's counter column for tests.
func (m *MockNoteRepository) IncrementCount(noteID, column string, delta int) error {
	n, ok := m.Notes[noteID]
	if !ok {
		return ErrNotFound
	}
	switch column {
	case "renoteCount":
		n.RenoteCount += int16(delta)
	case "repliesCount":
		n.RepliesCount += int16(delta)
	}
	return nil
}

// IncrementReaction adjusts an in-memory reaction count map.
// テスト用に Reactions を JSON Map にデコードして加算する。
func (m *MockNoteRepository) IncrementReaction(noteID, reaction string, delta int) error {
	n, ok := m.Notes[noteID]
	if !ok {
		return ErrNotFound
	}
	if m.ReactionCounts == nil {
		m.ReactionCounts = make(map[string]map[string]int)
	}
	if m.ReactionCounts[noteID] == nil {
		m.ReactionCounts[noteID] = make(map[string]int)
	}
	c := m.ReactionCounts[noteID][reaction] + delta
	if c <= 0 {
		delete(m.ReactionCounts[noteID], reaction)
	} else {
		m.ReactionCounts[noteID][reaction] = c
	}
	_ = n // 実際のJSONBは更新しない
	return nil
}

// ListRenotesOf returns notes whose renoteId equals noteID.
func (m *MockNoteRepository) ListRenotesOf(noteID string, untilID, sinceID string, limit int) ([]*model.Note, error) {
	return m.listFiltered(func(n *model.Note) bool {
		return n.RenoteID != nil && *n.RenoteID == noteID
	}, untilID, sinceID, limit), nil
}

// ListRepliesOf returns notes whose replyId equals noteID.
func (m *MockNoteRepository) ListRepliesOf(noteID string, untilID, sinceID string, limit int) ([]*model.Note, error) {
	return m.listFiltered(func(n *model.Note) bool {
		return n.ReplyID != nil && *n.ReplyID == noteID
	}, untilID, sinceID, limit), nil
}

// ListChildrenOf returns notes that reply to or quote the given noteID.
func (m *MockNoteRepository) ListChildrenOf(noteID string, untilID, sinceID string, limit int) ([]*model.Note, error) {
	return m.listFiltered(func(n *model.Note) bool {
		if n.ReplyID != nil && *n.ReplyID == noteID {
			return true
		}
		if n.RenoteID != nil && *n.RenoteID == noteID {
			return true
		}
		return false
	}, untilID, sinceID, limit), nil
}

// SearchByFilter returns public/home notes matching the filter (text ILIKE +
// optional userId / channelId / host)。`Host == "."` はローカル限定。
func (m *MockNoteRepository) SearchByFilter(f model.NoteSearchFilter) ([]*model.Note, error) {
	q := strings.ToLower(f.Query)
	limit := f.Limit
	if limit <= 0 {
		limit = 10
	}
	return m.listFiltered(func(n *model.Note) bool {
		if n.Visibility != model.NoteVisibilityPublic && n.Visibility != model.NoteVisibilityHome {
			return false
		}
		if n.Text == nil {
			return false
		}
		if !strings.Contains(strings.ToLower(*n.Text), q) {
			return false
		}
		if f.UserID != "" && n.UserID != f.UserID {
			return false
		}
		if f.ChannelID != "" {
			if n.ChannelID == nil || *n.ChannelID != f.ChannelID {
				return false
			}
		}
		if f.Host != "" {
			if f.Host == "." {
				if n.UserHost != nil {
					return false
				}
			} else {
				if n.UserHost == nil || *n.UserHost != f.Host {
					return false
				}
			}
		}
		return true
	}, f.UntilID, f.SinceID, limit), nil
}

// listFiltered iterates the in-memory notes, applies filter, sorts by id desc,
// and returns up to `limit` entries.
func (m *MockNoteRepository) listFiltered(filter func(*model.Note) bool, untilID, sinceID string, limit int) []*model.Note {
	var out []*model.Note
	for _, n := range m.Notes {
		if !filter(n) {
			continue
		}
		if untilID != "" && n.ID >= untilID {
			continue
		}
		if sinceID != "" && n.ID <= sinceID {
			continue
		}
		out = append(out, n)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i].ID < out[j].ID {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ListByChannelID returns notes posted to the given channel sorted by id desc.
func (m *MockNoteRepository) ListByChannelID(channelID string, untilID, sinceID string, limit int) ([]*model.Note, error) {
	return m.listFiltered(func(n *model.Note) bool {
		return n.ChannelID != nil && *n.ChannelID == channelID
	}, untilID, sinceID, limit), nil
}

func (m *MockNoteRepository) ListByUserID(userID string, untilID, sinceID string, limit int) ([]*model.Note, error) {
	var notes []*model.Note
	for _, n := range m.Notes {
		if n.UserID != userID {
			continue
		}
		if untilID != "" && n.ID >= untilID {
			continue
		}
		if sinceID != "" && n.ID <= sinceID {
			continue
		}
		notes = append(notes, n)
	}
	// id降順でソート
	for i := 0; i < len(notes); i++ {
		for j := i + 1; j < len(notes); j++ {
			if notes[i].ID < notes[j].ID {
				notes[i], notes[j] = notes[j], notes[i]
			}
		}
	}
	if limit > 0 && len(notes) > limit {
		notes = notes[:limit]
	}
	return notes, nil
}

func (m *MockNoteRepository) FindManyByIDsWithUser(ids []string) ([]*model.Note, error) {
	out := make([]*model.Note, 0, len(ids))
	for _, id := range ids {
		if n, ok := m.Notes[id]; ok {
			out = append(out, n)
		}
	}
	return out, nil
}

func (m *MockNoteRepository) ListFeatured(limit, offset int) ([]*model.Note, error) {
	var result []*model.Note
	for _, n := range m.Notes {
		if string(n.Visibility) == "public" {
			result = append(result, n)
		}
	}
	if limit <= 0 {
		limit = 10
	}
	if offset >= len(result) {
		return nil, nil
	}
	end := min(offset+limit, len(result))
	return result[offset:end], nil
}

func (m *MockNoteRepository) FindRenoteByUser(userID, renoteID string) (*model.Note, error) {
	for _, n := range m.Notes {
		if n.UserID == userID && n.RenoteID != nil && *n.RenoteID == renoteID && n.Text == nil {
			return n, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockNoteRepository) ListMentions(userID string, limit int, _, _ string) ([]*model.Note, error) {
	var result []*model.Note
	for _, n := range m.Notes {
		for _, mention := range n.Mentions {
			if mention == userID {
				result = append(result, n)
				break
			}
		}
	}
	if limit <= 0 {
		limit = 10
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// MockNoteFavoriteRepository is a test double for repository.NoteFavoriteRepository.
type MockNoteFavoriteRepository struct {
	Favorites map[string]*model.NoteFavorite // keyed by "userId:noteId"
}

func NewMockNoteFavoriteRepository() *MockNoteFavoriteRepository {
	return &MockNoteFavoriteRepository{Favorites: make(map[string]*model.NoteFavorite)}
}

func (m *MockNoteFavoriteRepository) Create(f *model.NoteFavorite) error {
	m.Favorites[f.UserID+":"+f.NoteID] = f
	return nil
}

func (m *MockNoteFavoriteRepository) Delete(userID, noteID string) error {
	delete(m.Favorites, userID+":"+noteID)
	return nil
}

func (m *MockNoteFavoriteRepository) Exists(userID, noteID string) (bool, error) {
	_, ok := m.Favorites[userID+":"+noteID]
	return ok, nil
}

func (m *MockNoteFavoriteRepository) ListByUser(userID string, limit, offset int) ([]*model.NoteFavorite, error) {
	var result []*model.NoteFavorite
	for _, f := range m.Favorites {
		if f.UserID == userID {
			result = append(result, f)
		}
	}
	return result, nil
}

// MockNoteReactionRepository is a test double for repository.NoteReactionRepository.
type MockNoteReactionRepository struct {
	Reactions map[string]*model.NoteReaction // keyed by id
	CreateErr error                          // optional error to return on Create
}

func NewMockNoteReactionRepository() *MockNoteReactionRepository {
	return &MockNoteReactionRepository{Reactions: make(map[string]*model.NoteReaction)}
}

func (m *MockNoteReactionRepository) Create(r *model.NoteReaction) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.Reactions[r.ID] = r
	return nil
}

func (m *MockNoteReactionRepository) Delete(r *model.NoteReaction) error {
	delete(m.Reactions, r.ID)
	return nil
}

func (m *MockNoteReactionRepository) FindByPair(userID, noteID string) (*model.NoteReaction, error) {
	for _, r := range m.Reactions {
		if r.UserID == userID && r.NoteID == noteID {
			return r, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockNoteReactionRepository) ListByNoteID(noteID, untilID, sinceID string, limit int, reaction string) ([]*model.NoteReaction, error) {
	var rows []*model.NoteReaction
	for _, r := range m.Reactions {
		if r.NoteID != noteID {
			continue
		}
		if reaction != "" && r.Reaction != reaction {
			continue
		}
		if untilID != "" && r.ID >= untilID {
			continue
		}
		if sinceID != "" && r.ID <= sinceID {
			continue
		}
		rows = append(rows, r)
	}
	// id降順
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].ID < rows[j].ID {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

// MockEmojiRepository is a test double for repository.EmojiRepository.
type MockEmojiRepository struct {
	// keyed by "name@host" (host="" for local)
	Emojis map[string]*model.Emoji
}

func NewMockEmojiRepository() *MockEmojiRepository {
	return &MockEmojiRepository{Emojis: make(map[string]*model.Emoji)}
}

func (m *MockEmojiRepository) Create(e *model.Emoji) error {
	key := e.Name + "@"
	if e.Host != nil {
		key += *e.Host
	}
	m.Emojis[key] = e
	return nil
}

func (m *MockEmojiRepository) FindByID(id string) (*model.Emoji, error) {
	for _, e := range m.Emojis {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockEmojiRepository) UpdateFields(id string, fields map[string]any) error {
	for _, e := range m.Emojis {
		if e.ID == id {
			if v, ok := fields["name"]; ok {
				e.Name = v.(string)
			}
			if v, ok := fields["category"]; ok {
				s := v.(string)
				e.Category = &s
			}
			return nil
		}
	}
	return ErrNotFound
}

func (m *MockEmojiRepository) Delete(id string) error {
	for k, e := range m.Emojis {
		if e.ID == id {
			delete(m.Emojis, k)
			return nil
		}
	}
	return nil
}

func (m *MockEmojiRepository) ListWithFilter(query, category string, local bool, limit, offset int) ([]*model.Emoji, error) {
	var result []*model.Emoji
	for _, e := range m.Emojis {
		if local && e.Host != nil {
			continue
		}
		result = append(result, e)
	}
	if limit <= 0 {
		limit = 50
	}
	if offset >= len(result) {
		return nil, nil
	}
	end := min(offset+limit, len(result))
	return result[offset:end], nil
}

func (m *MockEmojiRepository) ListLocal() ([]*model.Emoji, error) {
	var result []*model.Emoji
	for _, e := range m.Emojis {
		if e.Host == nil {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *MockEmojiRepository) FindByNameAndHost(name string, host *string) (*model.Emoji, error) {
	key := name + "@"
	if host != nil {
		key += *host
	}
	e, ok := m.Emojis[key]
	if !ok {
		return nil, ErrNotFound
	}
	return e, nil
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

func (m *MockMetaRepository) Update(fields map[string]any) error {
	if m.Meta == nil {
		m.Meta = &model.Meta{ID: "x"}
	}
	if v, ok := fields["rootUserId"]; ok {
		if s, ok := v.(string); ok {
			m.Meta.RootUserID = &s
		}
	}
	return nil
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

// MockUserNotePiningRepository is a test double for repository.UserNotePiningRepository.
type MockUserNotePiningRepository struct {
	Pinings map[string]*model.UserNotePining // keyed by ID
}

func NewMockUserNotePiningRepository() *MockUserNotePiningRepository {
	return &MockUserNotePiningRepository{Pinings: make(map[string]*model.UserNotePining)}
}

func (m *MockUserNotePiningRepository) Create(p *model.UserNotePining) error {
	m.Pinings[p.ID] = p
	return nil
}

func (m *MockUserNotePiningRepository) Delete(p *model.UserNotePining) error {
	delete(m.Pinings, p.ID)
	return nil
}

func (m *MockUserNotePiningRepository) FindByPair(userID, noteID string) (*model.UserNotePining, error) {
	for _, p := range m.Pinings {
		if p.UserID == userID && p.NoteID == noteID {
			return p, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockUserNotePiningRepository) ListByUser(userID string) ([]*model.UserNotePining, error) {
	var rows []*model.UserNotePining
	for _, p := range m.Pinings {
		if p.UserID == userID {
			rows = append(rows, p)
		}
	}
	return rows, nil
}

func (m *MockUserNotePiningRepository) CountByUser(userID string) (int, error) {
	count := 0
	for _, p := range m.Pinings {
		if p.UserID == userID {
			count++
		}
	}
	return count, nil
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

func (m *MockPollRepository) FindByNoteID(noteID string) (*model.Poll, error) {
	p, ok := m.Polls[noteID]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (m *MockPollRepository) IncrementVote(noteID string, choice int, delta int) error {
	p, ok := m.Polls[noteID]
	if !ok {
		return ErrNotFound
	}
	if choice < 0 || choice >= len(p.Votes) {
		return nil
	}
	p.Votes[choice] += int64(delta)
	return nil
}

// MockPollVoteRepository is a test double for repository.PollVoteRepository.
type MockPollVoteRepository struct {
	Votes map[string]*model.PollVote // keyed by id
}

func NewMockPollVoteRepository() *MockPollVoteRepository {
	return &MockPollVoteRepository{Votes: make(map[string]*model.PollVote)}
}

func (m *MockPollVoteRepository) Create(v *model.PollVote) error {
	m.Votes[v.ID] = v
	return nil
}

func (m *MockPollVoteRepository) FindByUserAndChoice(userID, noteID string, choice int) (*model.PollVote, error) {
	for _, v := range m.Votes {
		if v.UserID == userID && v.NoteID == noteID && v.Choice == choice {
			return v, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockPollVoteRepository) CountByUserAndNote(userID, noteID string) (int64, error) {
	var n int64
	for _, v := range m.Votes {
		if v.UserID == userID && v.NoteID == noteID {
			n++
		}
	}
	return n, nil
}

func (m *MockPollVoteRepository) ListByNoteID(noteID string) ([]*model.PollVote, error) {
	var rows []*model.PollVote
	for _, v := range m.Votes {
		if v.NoteID == noteID {
			rows = append(rows, v)
		}
	}
	return rows, nil
}

// MockInstanceRepository is a test double for repository.InstanceRepository.
type MockInstanceRepository struct {
	Instances map[string]*model.Instance // keyed by host
	CreateErr error
	UpdateErr error
}

// NewMockInstanceRepository creates an empty MockInstanceRepository.
func NewMockInstanceRepository() *MockInstanceRepository {
	return &MockInstanceRepository{Instances: make(map[string]*model.Instance)}
}

func (m *MockInstanceRepository) Create(i *model.Instance) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.Instances[i.Host] = i
	return nil
}

func (m *MockInstanceRepository) FindByHost(host string) (*model.Instance, error) {
	i, ok := m.Instances[host]
	if !ok {
		return nil, ErrNotFound
	}
	return i, nil
}

func (m *MockInstanceRepository) UpdateFields(host string, fields map[string]any) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	if len(fields) == 0 {
		return nil
	}
	inst, ok := m.Instances[host]
	if !ok {
		return ErrNotFound
	}
	applyInstanceFields(inst, fields)
	return nil
}

func (m *MockInstanceRepository) IncrementCount(host, column string, delta int) error {
	inst, ok := m.Instances[host]
	if !ok {
		return ErrNotFound
	}
	switch column {
	case "usersCount":
		inst.UsersCount += delta
	case "notesCount":
		inst.NotesCount += delta
	case "followingCount":
		inst.FollowingCount += delta
	case "followersCount":
		inst.FollowersCount += delta
	}
	return nil
}

// List returns all stored instances filtered by the most common predicates.
// 並び順は host 昇順 (テストの安定性のため)。
func (m *MockInstanceRepository) List(filter model.InstanceListFilter) ([]*model.Instance, error) {
	var rows []*model.Instance
	for _, inst := range m.Instances {
		if filter.Host != "" && !strings.Contains(inst.Host, filter.Host) {
			continue
		}
		if filter.Suspended != nil {
			suspended := inst.SuspensionState != model.SuspensionStateNone
			if suspended != *filter.Suspended {
				continue
			}
		}
		if filter.NotResponding != nil && inst.IsNotResponding != *filter.NotResponding {
			continue
		}
		if filter.Federating != nil && *filter.Federating &&
			inst.FollowingCount == 0 && inst.FollowersCount == 0 {
			continue
		}
		if filter.Subscribing != nil && *filter.Subscribing && inst.FollowersCount == 0 {
			continue
		}
		if filter.Publishing != nil && *filter.Publishing && inst.FollowingCount == 0 {
			continue
		}
		rows = append(rows, inst)
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].Host > rows[j].Host {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if filter.Offset >= len(rows) {
		return nil, nil
	}
	end := filter.Offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[filter.Offset:end], nil
}

func applyInstanceFields(i *model.Instance, fields map[string]any) {
	for k, v := range fields {
		switch k {
		case "name":
			if s, ok := v.(*string); ok {
				i.Name = s
			}
		case "description":
			if s, ok := v.(*string); ok {
				i.Description = s
			}
		case "softwareName":
			if s, ok := v.(*string); ok {
				i.SoftwareName = s
			}
		case "softwareVersion":
			if s, ok := v.(*string); ok {
				i.SoftwareVersion = s
			}
		case "iconUrl":
			if s, ok := v.(*string); ok {
				i.IconURL = s
			}
		case "faviconUrl":
			if s, ok := v.(*string); ok {
				i.FaviconURL = s
			}
		case "themeColor":
			if s, ok := v.(*string); ok {
				i.ThemeColor = s
			}
		case "openRegistrations":
			if b, ok := v.(*bool); ok {
				i.OpenRegistrations = b
			}
		case "infoUpdatedAt":
			if t, ok := v.(*time.Time); ok {
				i.InfoUpdatedAt = t
			}
		case "latestRequestReceivedAt":
			if t, ok := v.(*time.Time); ok {
				i.LatestRequestReceivedAt = t
			}
		case "isNotResponding":
			if b, ok := v.(bool); ok {
				i.IsNotResponding = b
			}
		case "notRespondingSince":
			if t, ok := v.(*time.Time); ok {
				i.NotRespondingSince = t
			}
		case "suspensionState":
			if s, ok := v.(model.SuspensionState); ok {
				i.SuspensionState = s
			}
		case "moderationNote":
			if s, ok := v.(string); ok {
				i.ModerationNote = s
			}
		}
	}
}

// MockClipRepository is a test double for repository.ClipRepository.
type MockClipRepository struct {
	Clips     map[string]*model.Clip
	CreateErr error
	UpdateErr error
}

// NewMockClipRepository creates an empty MockClipRepository.
func NewMockClipRepository() *MockClipRepository {
	return &MockClipRepository{Clips: make(map[string]*model.Clip)}
}

func (m *MockClipRepository) Create(c *model.Clip) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.Clips[c.ID] = c
	return nil
}

func (m *MockClipRepository) FindByID(id string) (*model.Clip, error) {
	c, ok := m.Clips[id]
	if !ok {
		return nil, ErrNotFound
	}
	return c, nil
}

func (m *MockClipRepository) UpdateFields(clipID string, fields map[string]any) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	if len(fields) == 0 {
		return nil
	}
	c, ok := m.Clips[clipID]
	if !ok {
		return ErrNotFound
	}
	applyClipFields(c, fields)
	return nil
}

func (m *MockClipRepository) Delete(c *model.Clip) error {
	delete(m.Clips, c.ID)
	return nil
}

func (m *MockClipRepository) ListByUser(userID string, limit, offset int) ([]*model.Clip, error) {
	var rows []*model.Clip
	for _, c := range m.Clips {
		if c.UserID == userID {
			rows = append(rows, c)
		}
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].ID < rows[j].ID {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset >= len(rows) {
		return nil, nil
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[offset:end], nil
}

func (m *MockClipRepository) IncrementCount(clipID, column string, delta int) error {
	c, ok := m.Clips[clipID]
	if !ok {
		return ErrNotFound
	}
	if column == "notesCount" {
		c.NotesCount += delta
	}
	return nil
}

func applyClipFields(c *model.Clip, fields map[string]any) {
	for k, v := range fields {
		switch k {
		case "name":
			if s, ok := v.(string); ok {
				c.Name = s
			}
		case "description":
			if s, ok := v.(*string); ok {
				c.Description = s
			}
		case "isPublic":
			if b, ok := v.(bool); ok {
				c.IsPublic = b
			}
		case "lastClippedAt":
			if t, ok := v.(*time.Time); ok {
				c.LastClippedAt = t
			}
		}
	}
}

// MockClipNoteRepository is a test double for repository.ClipNoteRepository.
type MockClipNoteRepository struct {
	Entries map[string]*model.ClipNote
}

// NewMockClipNoteRepository creates an empty MockClipNoteRepository.
func NewMockClipNoteRepository() *MockClipNoteRepository {
	return &MockClipNoteRepository{Entries: make(map[string]*model.ClipNote)}
}

func (m *MockClipNoteRepository) Create(cn *model.ClipNote) error {
	m.Entries[cn.ID] = cn
	return nil
}

func (m *MockClipNoteRepository) Delete(cn *model.ClipNote) error {
	delete(m.Entries, cn.ID)
	return nil
}

func (m *MockClipNoteRepository) FindByPair(clipID, noteID string) (*model.ClipNote, error) {
	for _, cn := range m.Entries {
		if cn.ClipID == clipID && cn.NoteID == noteID {
			return cn, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockClipNoteRepository) ListByClip(clipID string, untilID, sinceID string, limit int) ([]*model.ClipNote, error) {
	var rows []*model.ClipNote
	for _, cn := range m.Entries {
		if cn.ClipID != clipID {
			continue
		}
		if untilID != "" && cn.ID >= untilID {
			continue
		}
		if sinceID != "" && cn.ID <= sinceID {
			continue
		}
		rows = append(rows, cn)
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].ID < rows[j].ID {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

// MockPageRepository is a test double for repository.PageRepository.
type MockPageRepository struct {
	Pages     map[string]*model.Page
	CreateErr error
	UpdateErr error
}

// NewMockPageRepository creates an empty MockPageRepository.
func NewMockPageRepository() *MockPageRepository {
	return &MockPageRepository{Pages: make(map[string]*model.Page)}
}

func (m *MockPageRepository) Create(p *model.Page) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.Pages[p.ID] = p
	return nil
}

func (m *MockPageRepository) FindByID(id string) (*model.Page, error) {
	p, ok := m.Pages[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (m *MockPageRepository) FindByUserAndName(userID, name string) (*model.Page, error) {
	for _, p := range m.Pages {
		if p.UserID == userID && p.Name == name {
			return p, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockPageRepository) UpdateFields(pageID string, fields map[string]any) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	if len(fields) == 0 {
		return nil
	}
	p, ok := m.Pages[pageID]
	if !ok {
		return ErrNotFound
	}
	applyPageFields(p, fields)
	return nil
}

func (m *MockPageRepository) Delete(p *model.Page) error {
	delete(m.Pages, p.ID)
	return nil
}

func (m *MockPageRepository) ListByUser(userID string, limit, offset int) ([]*model.Page, error) {
	var rows []*model.Page
	for _, p := range m.Pages {
		if p.UserID == userID {
			rows = append(rows, p)
		}
	}
	// updatedAt 降順だが、テストの安定性のため updatedAt が同じ場合は ID 降順で解決する。
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].UpdatedAt.Before(rows[j].UpdatedAt) ||
				(rows[i].UpdatedAt.Equal(rows[j].UpdatedAt) && rows[i].ID < rows[j].ID) {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	return paginatePages(rows, limit, offset), nil
}

func (m *MockPageRepository) ListFeatured(limit, offset int) ([]*model.Page, error) {
	var rows []*model.Page
	for _, p := range m.Pages {
		if p.Visibility == model.PageVisibilityPublic {
			rows = append(rows, p)
		}
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].LikedCount < rows[j].LikedCount ||
				(rows[i].LikedCount == rows[j].LikedCount && rows[i].ID < rows[j].ID) {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	return paginatePages(rows, limit, offset), nil
}

func (m *MockPageRepository) IncrementCount(pageID, column string, delta int) error {
	p, ok := m.Pages[pageID]
	if !ok {
		return ErrNotFound
	}
	if column == "likedCount" {
		p.LikedCount += delta
	}
	return nil
}

func paginatePages(rows []*model.Page, limit, offset int) []*model.Page {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset >= len(rows) {
		return nil
	}
	end := min(offset+limit, len(rows))
	return rows[offset:end]
}

func applyPageFields(p *model.Page, fields map[string]any) {
	for k, v := range fields {
		switch k {
		case "title":
			if s, ok := v.(string); ok {
				p.Title = s
			}
		case "name":
			if s, ok := v.(string); ok {
				p.Name = s
			}
		case "summary":
			if s, ok := v.(*string); ok {
				p.Summary = s
			}
		case "alignCenter":
			if b, ok := v.(bool); ok {
				p.AlignCenter = b
			}
		case "hideTitleWhenPinned":
			if b, ok := v.(bool); ok {
				p.HideTitleWhenPinned = b
			}
		case "font":
			if s, ok := v.(string); ok {
				p.Font = s
			}
		case "eyeCatchingImageId":
			if s, ok := v.(*string); ok {
				p.EyeCatchingImageID = s
			}
		case "content":
			if b, ok := v.([]byte); ok {
				p.Content = b
			}
		case "variables":
			if b, ok := v.([]byte); ok {
				p.Variables = b
			}
		case "script":
			if s, ok := v.(string); ok {
				p.Script = s
			}
		case "visibility":
			if vis, ok := v.(model.PageVisibility); ok {
				p.Visibility = vis
			}
		case "updatedAt":
			if t, ok := v.(time.Time); ok {
				p.UpdatedAt = t
			}
		}
	}
}

// MockFlashRepository is a test double for repository.FlashRepository.
type MockFlashRepository struct {
	Flashes   map[string]*model.Flash
	CreateErr error
	UpdateErr error
}

// NewMockFlashRepository creates an empty MockFlashRepository.
func NewMockFlashRepository() *MockFlashRepository {
	return &MockFlashRepository{Flashes: make(map[string]*model.Flash)}
}

func (m *MockFlashRepository) Create(f *model.Flash) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.Flashes[f.ID] = f
	return nil
}

func (m *MockFlashRepository) FindByID(id string) (*model.Flash, error) {
	f, ok := m.Flashes[id]
	if !ok {
		return nil, ErrNotFound
	}
	return f, nil
}

func (m *MockFlashRepository) UpdateFields(flashID string, fields map[string]any) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	if len(fields) == 0 {
		return nil
	}
	f, ok := m.Flashes[flashID]
	if !ok {
		return ErrNotFound
	}
	applyFlashFields(f, fields)
	return nil
}

func (m *MockFlashRepository) Delete(f *model.Flash) error {
	delete(m.Flashes, f.ID)
	return nil
}

func (m *MockFlashRepository) ListByUser(userID string, limit, offset int) ([]*model.Flash, error) {
	var rows []*model.Flash
	for _, f := range m.Flashes {
		if f.UserID == userID {
			rows = append(rows, f)
		}
	}
	sortFlashesByUpdatedDesc(rows)
	return paginateFlashes(rows, limit, offset), nil
}

func (m *MockFlashRepository) ListFeatured(limit, offset int) ([]*model.Flash, error) {
	var rows []*model.Flash
	for _, f := range m.Flashes {
		rows = append(rows, f)
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].LikedCount < rows[j].LikedCount ||
				(rows[i].LikedCount == rows[j].LikedCount && rows[i].ID < rows[j].ID) {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	return paginateFlashes(rows, limit, offset), nil
}

func (m *MockFlashRepository) Search(query string, limit, offset int) ([]*model.Flash, error) {
	var rows []*model.Flash
	q := strings.ToLower(query)
	for _, f := range m.Flashes {
		if strings.Contains(strings.ToLower(f.Title), q) ||
			strings.Contains(strings.ToLower(f.Summary), q) {
			rows = append(rows, f)
		}
	}
	sortFlashesByUpdatedDesc(rows)
	return paginateFlashes(rows, limit, offset), nil
}

func (m *MockFlashRepository) IncrementCount(flashID, column string, delta int) error {
	f, ok := m.Flashes[flashID]
	if !ok {
		return ErrNotFound
	}
	if column == "likedCount" {
		f.LikedCount += delta
	}
	return nil
}

func sortFlashesByUpdatedDesc(rows []*model.Flash) {
	for i := range rows {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].UpdatedAt.Before(rows[j].UpdatedAt) ||
				(rows[i].UpdatedAt.Equal(rows[j].UpdatedAt) && rows[i].ID < rows[j].ID) {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
}

func paginateFlashes(rows []*model.Flash, limit, offset int) []*model.Flash {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset >= len(rows) {
		return nil
	}
	end := min(offset+limit, len(rows))
	return rows[offset:end]
}

func applyFlashFields(f *model.Flash, fields map[string]any) {
	for k, v := range fields {
		switch k {
		case "title":
			if s, ok := v.(string); ok {
				f.Title = s
			}
		case "summary":
			if s, ok := v.(string); ok {
				f.Summary = s
			}
		case "script":
			if s, ok := v.(string); ok {
				f.Script = s
			}
		case "permissions":
			if arr, ok := v.([]string); ok {
				f.Permissions = arr
			}
		case "visibility":
			if s, ok := v.(string); ok {
				f.Visibility = s
			}
		case "updatedAt":
			if t, ok := v.(time.Time); ok {
				f.UpdatedAt = t
			}
		}
	}
}

// MockFlashLikeRepository is a test double for repository.FlashLikeRepository.
type MockFlashLikeRepository struct {
	Likes map[string]*model.FlashLike
}

// NewMockFlashLikeRepository creates an empty MockFlashLikeRepository.
func NewMockFlashLikeRepository() *MockFlashLikeRepository {
	return &MockFlashLikeRepository{Likes: make(map[string]*model.FlashLike)}
}

func (m *MockFlashLikeRepository) Create(l *model.FlashLike) error {
	m.Likes[l.ID] = l
	return nil
}

func (m *MockFlashLikeRepository) Delete(l *model.FlashLike) error {
	delete(m.Likes, l.ID)
	return nil
}

func (m *MockFlashLikeRepository) FindByPair(userID, flashID string) (*model.FlashLike, error) {
	for _, l := range m.Likes {
		if l.UserID == userID && l.FlashID == flashID {
			return l, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockFlashLikeRepository) Exists(userID, flashID string) (bool, error) {
	_, err := m.FindByPair(userID, flashID)
	return err == nil, nil
}

func (m *MockFlashLikeRepository) ListByUser(userID string, limit, offset int) ([]*model.FlashLike, error) {
	var rows []*model.FlashLike
	for _, l := range m.Likes {
		if l.UserID == userID {
			rows = append(rows, l)
		}
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].ID < rows[j].ID {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset >= len(rows) {
		return nil, nil
	}
	end := min(offset+limit, len(rows))
	return rows[offset:end], nil
}

// MockPageLikeRepository is a test double for repository.PageLikeRepository.
type MockPageLikeRepository struct {
	Likes map[string]*model.PageLike
}

// NewMockPageLikeRepository creates an empty MockPageLikeRepository.
func NewMockPageLikeRepository() *MockPageLikeRepository {
	return &MockPageLikeRepository{Likes: make(map[string]*model.PageLike)}
}

func (m *MockPageLikeRepository) Create(l *model.PageLike) error {
	m.Likes[l.ID] = l
	return nil
}

func (m *MockPageLikeRepository) Delete(l *model.PageLike) error {
	delete(m.Likes, l.ID)
	return nil
}

func (m *MockPageLikeRepository) FindByPair(userID, pageID string) (*model.PageLike, error) {
	for _, l := range m.Likes {
		if l.UserID == userID && l.PageID == pageID {
			return l, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockPageLikeRepository) Exists(userID, pageID string) (bool, error) {
	_, err := m.FindByPair(userID, pageID)
	return err == nil, nil
}

// MockAntennaRepository is a test double for repository.AntennaRepository.
type MockAntennaRepository struct {
	Antennas  map[string]*model.Antenna
	CreateErr error
	UpdateErr error
}

// NewMockAntennaRepository creates an empty MockAntennaRepository.
func NewMockAntennaRepository() *MockAntennaRepository {
	return &MockAntennaRepository{Antennas: make(map[string]*model.Antenna)}
}

func (m *MockAntennaRepository) Create(a *model.Antenna) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.Antennas[a.ID] = a
	return nil
}

func (m *MockAntennaRepository) FindByID(id string) (*model.Antenna, error) {
	a, ok := m.Antennas[id]
	if !ok {
		return nil, ErrNotFound
	}
	return a, nil
}

func (m *MockAntennaRepository) UpdateFields(antennaID string, fields map[string]any) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	if len(fields) == 0 {
		return nil
	}
	a, ok := m.Antennas[antennaID]
	if !ok {
		return ErrNotFound
	}
	applyAntennaFields(a, fields)
	return nil
}

func (m *MockAntennaRepository) Delete(a *model.Antenna) error {
	delete(m.Antennas, a.ID)
	return nil
}

func (m *MockAntennaRepository) ListByUser(userID string) ([]*model.Antenna, error) {
	var rows []*model.Antenna
	for _, a := range m.Antennas {
		if a.UserID == userID {
			rows = append(rows, a)
		}
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].ID < rows[j].ID {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	return rows, nil
}

func (m *MockAntennaRepository) ListAllActive() ([]*model.Antenna, error) {
	var rows []*model.Antenna
	for _, a := range m.Antennas {
		if a.IsActive {
			rows = append(rows, a)
		}
	}
	return rows, nil
}

func applyAntennaFields(a *model.Antenna, fields map[string]any) {
	for k, v := range fields {
		switch k {
		case "name":
			if s, ok := v.(string); ok {
				a.Name = s
			}
		case "src":
			if s, ok := v.(model.AntennaSource); ok {
				a.Src = s
			}
		case "users":
			if arr, ok := v.([]string); ok {
				a.Users = arr
			}
		case "keywords":
			if b, ok := v.([]byte); ok {
				a.Keywords = b
			}
		case "excludeKeywords":
			if b, ok := v.([]byte); ok {
				a.ExcludeKeywords = b
			}
		case "caseSensitive":
			if b, ok := v.(bool); ok {
				a.CaseSensitive = b
			}
		case "excludeBots":
			if b, ok := v.(bool); ok {
				a.ExcludeBots = b
			}
		case "withReplies":
			if b, ok := v.(bool); ok {
				a.WithReplies = b
			}
		case "withFile":
			if b, ok := v.(bool); ok {
				a.WithFile = b
			}
		case "isActive":
			if b, ok := v.(bool); ok {
				a.IsActive = b
			}
		case "localOnly":
			if b, ok := v.(bool); ok {
				a.LocalOnly = b
			}
		case "lastUsedAt":
			if t, ok := v.(time.Time); ok {
				a.LastUsedAt = t
			}
		}
	}
}

// MockChannelRepository is a test double for repository.ChannelRepository.
type MockChannelRepository struct {
	Channels  map[string]*model.Channel
	CreateErr error
	UpdateErr error
}

// NewMockChannelRepository creates an empty MockChannelRepository.
func NewMockChannelRepository() *MockChannelRepository {
	return &MockChannelRepository{Channels: make(map[string]*model.Channel)}
}

func (m *MockChannelRepository) Create(c *model.Channel) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.Channels[c.ID] = c
	return nil
}

func (m *MockChannelRepository) FindByID(id string) (*model.Channel, error) {
	c, ok := m.Channels[id]
	if !ok {
		return nil, ErrNotFound
	}
	return c, nil
}

func (m *MockChannelRepository) UpdateFields(channelID string, fields map[string]any) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	if len(fields) == 0 {
		return nil
	}
	c, ok := m.Channels[channelID]
	if !ok {
		return ErrNotFound
	}
	applyChannelFields(c, fields)
	return nil
}

func (m *MockChannelRepository) IncrementCount(channelID, column string, delta int) error {
	c, ok := m.Channels[channelID]
	if !ok {
		return ErrNotFound
	}
	switch column {
	case "notesCount":
		c.NotesCount += delta
	case "usersCount":
		c.UsersCount += delta
	}
	return nil
}

// List returns channels matching the most common filter predicates. テストの
// 安定性のため id 昇順で返す。
func (m *MockChannelRepository) List(filter model.ChannelListFilter) ([]*model.Channel, error) {
	var rows []*model.Channel
	for _, c := range m.Channels {
		if filter.OwnerID != "" {
			if c.UserID == nil || *c.UserID != filter.OwnerID {
				continue
			}
		}
		if filter.Query != "" && !strings.Contains(c.Name, filter.Query) {
			continue
		}
		if filter.IsArchived != nil && c.IsArchived != *filter.IsArchived {
			continue
		}
		rows = append(rows, c)
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].ID > rows[j].ID {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if filter.Offset >= len(rows) {
		return nil, nil
	}
	end := filter.Offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[filter.Offset:end], nil
}

func applyChannelFields(c *model.Channel, fields map[string]any) {
	for k, v := range fields {
		switch k {
		case "name":
			if s, ok := v.(string); ok {
				c.Name = s
			}
		case "description":
			if s, ok := v.(*string); ok {
				c.Description = s
			}
		case "color":
			if s, ok := v.(string); ok {
				c.Color = s
			}
		case "isArchived":
			if b, ok := v.(bool); ok {
				c.IsArchived = b
			}
		case "isSensitive":
			if b, ok := v.(bool); ok {
				c.IsSensitive = b
			}
		case "lastNotedAt":
			if t, ok := v.(*time.Time); ok {
				c.LastNotedAt = t
			}
		}
	}
}

// MockChannelFollowingRepository is a test double for the channel_following
// repository.
type MockChannelFollowingRepository struct {
	Followings map[string]*model.ChannelFollowing
}

// NewMockChannelFollowingRepository creates an empty MockChannelFollowingRepository.
func NewMockChannelFollowingRepository() *MockChannelFollowingRepository {
	return &MockChannelFollowingRepository{Followings: make(map[string]*model.ChannelFollowing)}
}

func (m *MockChannelFollowingRepository) Create(f *model.ChannelFollowing) error {
	m.Followings[f.ID] = f
	return nil
}

func (m *MockChannelFollowingRepository) Delete(f *model.ChannelFollowing) error {
	delete(m.Followings, f.ID)
	return nil
}

func (m *MockChannelFollowingRepository) FindByPair(followerID, channelID string) (*model.ChannelFollowing, error) {
	for _, f := range m.Followings {
		if f.FollowerID == followerID && f.FolloweeID == channelID {
			return f, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockChannelFollowingRepository) Exists(followerID, channelID string) (bool, error) {
	_, err := m.FindByPair(followerID, channelID)
	return err == nil, nil
}

func (m *MockChannelFollowingRepository) ListFollowed(userID string, limit, offset int) ([]*model.ChannelFollowing, error) {
	var rows []*model.ChannelFollowing
	for _, f := range m.Followings {
		if f.FollowerID == userID {
			rows = append(rows, f)
		}
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].ID < rows[j].ID {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset >= len(rows) {
		return nil, nil
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[offset:end], nil
}

// MockUserKeypairRepository is a test double for repository.UserKeypairRepository.
type MockUserKeypairRepository struct {
	Keypairs map[string]*model.UserKeypair // keyed by userID
}

func NewMockUserKeypairRepository() *MockUserKeypairRepository {
	return &MockUserKeypairRepository{Keypairs: make(map[string]*model.UserKeypair)}
}

func (m *MockUserKeypairRepository) Create(k *model.UserKeypair) error {
	m.Keypairs[k.UserID] = k
	return nil
}

func (m *MockUserKeypairRepository) FindByUserID(userID string) (*model.UserKeypair, error) {
	k, ok := m.Keypairs[userID]
	if !ok {
		return nil, ErrNotFound
	}
	return k, nil
}

// MockFollowingRepository is a test double for repository.FollowingRepository.
type MockFollowingRepository struct {
	Followings map[string]*model.Following // keyed by ID
	// RemoteInboxes stores per-followee inbox lists used by
	// ListRemoteFollowerInboxes. テスト側で明示的に登録する。
	RemoteInboxes map[string][]string
}

func NewMockFollowingRepository() *MockFollowingRepository {
	return &MockFollowingRepository{
		Followings:    make(map[string]*model.Following),
		RemoteInboxes: make(map[string][]string),
	}
}

// ListRemoteFollowerInboxes returns the inbox URLs registered for the given
// followee. テストでは MockFollowingRepository.RemoteInboxes を直接埋めて使う。
func (m *MockFollowingRepository) ListRemoteFollowerInboxes(userID string) ([]string, error) {
	return m.RemoteInboxes[userID], nil
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

// ---------------------------------------------------------------------------
// MockUserListRepository
// ---------------------------------------------------------------------------

type MockUserListRepository struct {
	Lists   map[string]*model.UserList
	Members []*model.UserListMembership
}

func NewMockUserListRepository() *MockUserListRepository {
	return &MockUserListRepository{Lists: make(map[string]*model.UserList)}
}

func (m *MockUserListRepository) Create(list *model.UserList) error {
	m.Lists[list.ID] = list
	return nil
}

func (m *MockUserListRepository) FindByID(id string) (*model.UserList, error) {
	l, ok := m.Lists[id]
	if !ok {
		return nil, ErrNotFound
	}
	return l, nil
}

func (m *MockUserListRepository) ListByUser(userID string) ([]*model.UserList, error) {
	var result []*model.UserList
	for _, l := range m.Lists {
		if l.UserID == userID {
			result = append(result, l)
		}
	}
	return result, nil
}

func (m *MockUserListRepository) Delete(id string) error {
	delete(m.Lists, id)
	return nil
}

func (m *MockUserListRepository) AddMember(mem *model.UserListMembership) error {
	m.Members = append(m.Members, mem)
	return nil
}

func (m *MockUserListRepository) RemoveMember(listID, userID string) error {
	var filtered []*model.UserListMembership
	for _, mem := range m.Members {
		if !(mem.UserListID == listID && mem.UserID == userID) {
			filtered = append(filtered, mem)
		}
	}
	m.Members = filtered
	return nil
}

func (m *MockUserListRepository) ListMembers(listID string) ([]*model.UserListMembership, error) {
	var result []*model.UserListMembership
	for _, mem := range m.Members {
		if mem.UserListID == listID {
			result = append(result, mem)
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// MockAnnouncementRepository
// ---------------------------------------------------------------------------

type MockAnnouncementRepository struct {
	Items map[string]*model.Announcement
	Reads map[string]bool // keyed by "userId:announcementId"
}

func NewMockAnnouncementRepository() *MockAnnouncementRepository {
	return &MockAnnouncementRepository{
		Items: make(map[string]*model.Announcement),
		Reads: make(map[string]bool),
	}
}

func (m *MockAnnouncementRepository) Create(a *model.Announcement) error {
	m.Items[a.ID] = a
	return nil
}

func (m *MockAnnouncementRepository) FindByID(id string) (*model.Announcement, error) {
	a, ok := m.Items[id]
	if !ok {
		return nil, ErrNotFound
	}
	return a, nil
}

func (m *MockAnnouncementRepository) List(activeOnly bool, limit, offset int) ([]*model.Announcement, error) {
	var result []*model.Announcement
	for _, a := range m.Items {
		if activeOnly && !a.IsActive {
			result = append(result, a) // activeOnly=false の場合も含める
			continue
		}
		result = append(result, a)
	}
	if limit <= 0 {
		limit = 10
	}
	if offset >= len(result) {
		return nil, nil
	}
	end := min(offset+limit, len(result))
	return result[offset:end], nil
}

func (m *MockAnnouncementRepository) UpdateFields(id string, fields map[string]any) error {
	a, ok := m.Items[id]
	if !ok {
		return ErrNotFound
	}
	if v, ok := fields["title"]; ok {
		a.Title = v.(string)
	}
	if v, ok := fields["isActive"]; ok {
		a.IsActive = v.(bool)
	}
	return nil
}

func (m *MockAnnouncementRepository) Delete(id string) error {
	delete(m.Items, id)
	return nil
}

func (m *MockAnnouncementRepository) MarkRead(read *model.AnnouncementRead) error {
	m.Reads[read.UserID+":"+read.AnnouncementID] = true
	return nil
}

func (m *MockAnnouncementRepository) IsRead(userID, announcementID string) (bool, error) {
	return m.Reads[userID+":"+announcementID], nil
}

func (m *MockAnnouncementRepository) UnreadForUser(userID string) ([]*model.Announcement, error) {
	var result []*model.Announcement
	for _, a := range m.Items {
		if a.IsActive && !m.Reads[userID+":"+a.ID] {
			result = append(result, a)
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// MockRegistryRepository
// ---------------------------------------------------------------------------

// MockRegistryRepository is a test double for repository.RegistryRepository.
type MockRegistryRepository struct {
	Items map[string]*model.RegistryItem // keyed by "userId:key:scope"
}

func NewMockRegistryRepository() *MockRegistryRepository {
	return &MockRegistryRepository{Items: make(map[string]*model.RegistryItem)}
}

func (m *MockRegistryRepository) rkey(userID, key string, scope []string) string {
	return userID + ":" + key + ":" + strings.Join(scope, ",")
}

func (m *MockRegistryRepository) Get(userID, key string, scope []string, _ *string) (*model.RegistryItem, error) {
	item, ok := m.Items[m.rkey(userID, key, scope)]
	if !ok {
		return nil, ErrNotFound
	}
	return item, nil
}

func (m *MockRegistryRepository) Set(item *model.RegistryItem) error {
	m.Items[m.rkey(item.UserID, item.Key, item.Scope)] = item
	return nil
}

func (m *MockRegistryRepository) GetAll(userID string, scope []string, _ *string) ([]*model.RegistryItem, error) {
	var result []*model.RegistryItem
	prefix := userID + ":"
	for k, item := range m.Items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			result = append(result, item)
		}
	}
	return result, nil
}

func (m *MockRegistryRepository) KeysWithType(userID string, scope []string, domain *string) (map[string]string, error) {
	items, _ := m.GetAll(userID, scope, domain)
	result := make(map[string]string, len(items))
	for _, item := range items {
		result[item.Key] = "object"
	}
	return result, nil
}

func (m *MockRegistryRepository) Remove(userID, key string, scope []string, _ *string) error {
	delete(m.Items, m.rkey(userID, key, scope))
	return nil
}

// ---------------------------------------------------------------------------
// MockAbuseReportRepository
// ---------------------------------------------------------------------------

type MockAbuseReportRepository struct {
	Reports map[string]*model.AbuseUserReport
}

func NewMockAbuseReportRepository() *MockAbuseReportRepository {
	return &MockAbuseReportRepository{Reports: make(map[string]*model.AbuseUserReport)}
}

func (m *MockAbuseReportRepository) Create(r *model.AbuseUserReport) error {
	m.Reports[r.ID] = r
	return nil
}

func (m *MockAbuseReportRepository) FindByID(id string) (*model.AbuseUserReport, error) {
	r, ok := m.Reports[id]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

func (m *MockAbuseReportRepository) List(resolved *bool, limit, offset int) ([]*model.AbuseUserReport, error) {
	var result []*model.AbuseUserReport
	for _, r := range m.Reports {
		if resolved != nil && r.Resolved != *resolved {
			continue
		}
		result = append(result, r)
	}
	if limit <= 0 {
		limit = 10
	}
	if offset >= len(result) {
		return nil, nil
	}
	end := min(offset+limit, len(result))
	return result[offset:end], nil
}

func (m *MockAbuseReportRepository) UpdateFields(id string, fields map[string]any) error {
	r, ok := m.Reports[id]
	if !ok {
		return ErrNotFound
	}
	if v, ok := fields["resolved"]; ok {
		r.Resolved = v.(bool)
	}
	if v, ok := fields["resolvedAs"]; ok {
		s := v.(string)
		r.ResolvedAs = &s
	}
	return nil
}

// ---------------------------------------------------------------------------
// MockModerationLogRepository
// ---------------------------------------------------------------------------

type MockModerationLogRepository struct {
	Logs []*model.ModerationLog
}

func NewMockModerationLogRepository() *MockModerationLogRepository {
	return &MockModerationLogRepository{}
}

func (m *MockModerationLogRepository) Create(log *model.ModerationLog) error {
	m.Logs = append(m.Logs, log)
	return nil
}

func (m *MockModerationLogRepository) List(limit, offset int) ([]*model.ModerationLog, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset >= len(m.Logs) {
		return nil, nil
	}
	end := min(offset+limit, len(m.Logs))
	return m.Logs[offset:end], nil
}

// ---------------------------------------------------------------------------
// MockRoleRepository
// ---------------------------------------------------------------------------

// MockRoleRepository is a test double for repository.RoleRepository.
type MockRoleRepository struct {
	Roles map[string]*model.Role
}

func NewMockRoleRepository() *MockRoleRepository {
	return &MockRoleRepository{Roles: make(map[string]*model.Role)}
}

func (m *MockRoleRepository) Create(role *model.Role) error {
	m.Roles[role.ID] = role
	return nil
}

func (m *MockRoleRepository) FindByID(id string) (*model.Role, error) {
	r, ok := m.Roles[id]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

func (m *MockRoleRepository) List() ([]*model.Role, error) {
	result := make([]*model.Role, 0, len(m.Roles))
	for _, r := range m.Roles {
		result = append(result, r)
	}
	return result, nil
}

func (m *MockRoleRepository) UpdateFields(id string, fields map[string]any) error {
	r, ok := m.Roles[id]
	if !ok {
		return ErrNotFound
	}
	if v, ok := fields["name"]; ok {
		r.Name = v.(string)
	}
	if v, ok := fields["description"]; ok {
		r.Description = v.(string)
	}
	if v, ok := fields["isModerator"]; ok {
		r.IsModerator = v.(bool)
	}
	if v, ok := fields["isAdministrator"]; ok {
		r.IsAdministrator = v.(bool)
	}
	if v, ok := fields["isPublic"]; ok {
		r.IsPublic = v.(bool)
	}
	return nil
}

func (m *MockRoleRepository) Delete(id string) error {
	delete(m.Roles, id)
	return nil
}

// ---------------------------------------------------------------------------
// MockRoleAssignmentRepository
// ---------------------------------------------------------------------------

// MockRoleAssignmentRepository is a test double for repository.RoleAssignmentRepository.
type MockRoleAssignmentRepository struct {
	Assignments map[string]*model.RoleAssignment // keyed by "userId:roleId"
	RoleRepo    *MockRoleRepository
}

func NewMockRoleAssignmentRepository(roleRepo *MockRoleRepository) *MockRoleAssignmentRepository {
	return &MockRoleAssignmentRepository{
		Assignments: make(map[string]*model.RoleAssignment),
		RoleRepo:    roleRepo,
	}
}

func (m *MockRoleAssignmentRepository) key(userID, roleID string) string {
	return userID + ":" + roleID
}

func (m *MockRoleAssignmentRepository) Create(a *model.RoleAssignment) error {
	m.Assignments[m.key(a.UserID, a.RoleID)] = a
	return nil
}

func (m *MockRoleAssignmentRepository) Delete(userID, roleID string) error {
	delete(m.Assignments, m.key(userID, roleID))
	return nil
}

func (m *MockRoleAssignmentRepository) ListByUser(userID string) ([]*model.RoleAssignment, error) {
	var result []*model.RoleAssignment
	now := time.Now()
	for _, a := range m.Assignments {
		if a.UserID == userID && (a.ExpiresAt == nil || a.ExpiresAt.After(now)) {
			// ロール情報を付与
			if m.RoleRepo != nil {
				if r, ok := m.RoleRepo.Roles[a.RoleID]; ok {
					a.Role = r
				}
			}
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *MockRoleAssignmentRepository) ListByRole(roleID string, limit, offset int) ([]*model.RoleAssignment, error) {
	var result []*model.RoleAssignment
	now := time.Now()
	for _, a := range m.Assignments {
		if a.RoleID == roleID && (a.ExpiresAt == nil || a.ExpiresAt.After(now)) {
			result = append(result, a)
		}
	}
	if offset >= len(result) {
		return nil, nil
	}
	end := min(offset+limit, len(result))
	if limit <= 0 {
		end = len(result)
	}
	return result[offset:end], nil
}

func (m *MockRoleAssignmentRepository) Exists(userID, roleID string) (bool, error) {
	a, ok := m.Assignments[m.key(userID, roleID)]
	if !ok {
		return false, nil
	}
	now := time.Now()
	if a.ExpiresAt != nil && a.ExpiresAt.Before(now) {
		return false, nil
	}
	return true, nil
}
