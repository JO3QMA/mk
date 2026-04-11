package chat_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/lib/pq"
	corechat "github.com/shiroha-a/mk/internal/core/chat"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fake repository ---

type fakeChatRepo struct {
	mu          sync.Mutex
	rooms       map[string]*model.ChatRoom
	messages    map[string]*model.ChatMessage
	memberships map[string]*model.ChatRoomMembership // key: userID:roomID

	createErr error
	updateErr error
	deleteErr error
}

func newFakeRepo() *fakeChatRepo {
	return &fakeChatRepo{
		rooms:       make(map[string]*model.ChatRoom),
		messages:    make(map[string]*model.ChatMessage),
		memberships: make(map[string]*model.ChatRoomMembership),
	}
}

func (r *fakeChatRepo) CreateRoom(room *model.ChatRoom) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rooms[room.ID] = room
	return nil
}
func (r *fakeChatRepo) FindRoomByID(id string) (*model.ChatRoom, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if room, ok := r.rooms[id]; ok {
		return room, nil
	}
	return nil, errors.New("not found")
}
func (r *fakeChatRepo) UpdateRoom(_ *model.ChatRoom) error                   { return nil }
func (r *fakeChatRepo) DeleteRoom(_ string) error                            { return nil }
func (r *fakeChatRepo) ListRoomsByOwner(_ string) ([]*model.ChatRoom, error) { return nil, nil }
func (r *fakeChatRepo) ListJoinedRooms(_ string) ([]*model.ChatRoom, error)  { return nil, nil }

func (r *fakeChatRepo) CreateMessage(msg *model.ChatMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	clone := *msg
	r.messages[msg.ID] = &clone
	return nil
}
func (r *fakeChatRepo) FindMessageByID(id string) (*model.ChatMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if msg, ok := r.messages[id]; ok {
		clone := *msg
		return &clone, nil
	}
	return nil, errors.New("not found")
}
func (r *fakeChatRepo) UpdateMessage(msg *model.ChatMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateErr != nil {
		return r.updateErr
	}
	clone := *msg
	r.messages[msg.ID] = &clone
	return nil
}
func (r *fakeChatRepo) DeleteMessage(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleteErr != nil {
		return r.deleteErr
	}
	delete(r.messages, id)
	return nil
}
func (r *fakeChatRepo) ListMessagesByRoom(_ string, _ int) ([]*model.ChatMessage, error) {
	return nil, nil
}
func (r *fakeChatRepo) ListMessagesByUser(_, _ string, _ int) ([]*model.ChatMessage, error) {
	return nil, nil
}
func (r *fakeChatRepo) SearchMessages(_, _ string, _ int) ([]*model.ChatMessage, error) {
	return nil, nil
}
func (r *fakeChatRepo) CreateMembership(m *model.ChatRoomMembership) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memberships[m.UserID+":"+m.RoomID] = m
	return nil
}
func (r *fakeChatRepo) FindMembership(userID, roomID string) (*model.ChatRoomMembership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.memberships[userID+":"+roomID]; ok {
		return m, nil
	}
	return nil, errors.New("not found")
}
func (r *fakeChatRepo) UpdateMembership(_ *model.ChatRoomMembership) error { return nil }
func (r *fakeChatRepo) DeleteMembership(_, _ string) error                 { return nil }
func (r *fakeChatRepo) ListMembersByRoom(_ string) ([]*model.ChatRoomMembership, error) {
	return nil, nil
}
func (r *fakeChatRepo) CreateInvitation(_ *model.ChatRoomInvitation) error { return nil }
func (r *fakeChatRepo) DeleteInvitation(_ string) error                    { return nil }
func (r *fakeChatRepo) FindInvitation(_, _ string) (*model.ChatRoomInvitation, error) {
	return nil, errors.New("not found")
}
func (r *fakeChatRepo) CountUnread(_ string) (int64, error) { return 0, nil }
func (r *fakeChatRepo) MarkRead(userID, messageID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if msg, ok := r.messages[messageID]; ok {
		msg.Reads = append(msg.Reads, userID)
	}
	return nil
}

// --- capture publisher ---

type capturePublisher struct {
	mu        sync.Mutex
	userCalls []userCall
	roomCalls []roomCall
}

type userCall struct {
	from, to, eventType string
	body                any
}

type roomCall struct {
	roomID, eventType string
	body              any
}

func (p *capturePublisher) PublishUserMessage(_ context.Context, from, to, t string, body any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.userCalls = append(p.userCalls, userCall{from, to, t, body})
}

func (p *capturePublisher) PublishRoomMessage(_ context.Context, roomID, t string, body any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.roomCalls = append(p.roomCalls, roomCall{roomID, t, body})
}

// --- helper ---

func newSvc(t *testing.T) (*corechat.Service, *fakeChatRepo, *capturePublisher) {
	t.Helper()
	repo := newFakeRepo()
	pub := &capturePublisher{}
	idGen, _ := id.NewGenerator("aidx")
	svc := corechat.NewService(repo, idGen)
	svc.SetStreamingPublisher(pub)
	return svc, repo, pub
}

// --- CreateMessageToUser ---

func TestCreateMessageToUser_Success(t *testing.T) {
	svc, _, pub := newSvc(t)
	msg, err := svc.CreateMessageToUser(context.Background(), "alice", "bob", "hello", "")
	require.NoError(t, err)
	assert.NotEmpty(t, msg.ID)
	require.NotNil(t, msg.Text)
	assert.Equal(t, "hello", *msg.Text)
	require.Len(t, pub.userCalls, 1)
	assert.Equal(t, "alice", pub.userCalls[0].from)
	assert.Equal(t, "bob", pub.userCalls[0].to)
	assert.Equal(t, corechat.EventMessage, pub.userCalls[0].eventType)
}

func TestCreateMessageToUser_WithFile(t *testing.T) {
	svc, _, _ := newSvc(t)
	msg, err := svc.CreateMessageToUser(context.Background(), "alice", "bob", "", "file_x")
	require.NoError(t, err)
	require.NotNil(t, msg.FileID)
	assert.Equal(t, "file_x", *msg.FileID)
}

func TestCreateMessageToUser_EmptySender(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.CreateMessageToUser(context.Background(), "", "bob", "hi", "")
	assert.ErrorIs(t, err, corechat.ErrInvalidTarget)
}

func TestCreateMessageToUser_EmptyRecipient(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.CreateMessageToUser(context.Background(), "alice", "", "hi", "")
	assert.ErrorIs(t, err, corechat.ErrInvalidTarget)
}

func TestCreateMessageToUser_RepoError(t *testing.T) {
	svc, repo, pub := newSvc(t)
	repo.createErr = errors.New("db boom")
	_, err := svc.CreateMessageToUser(context.Background(), "alice", "bob", "hi", "")
	assert.Error(t, err)
	assert.Empty(t, pub.userCalls)
}

// --- CreateMessageToRoom ---

func seedRoom(t *testing.T, repo *fakeChatRepo, id string, owner string, members ...string) {
	t.Helper()
	repo.rooms[id] = &model.ChatRoom{ID: id, Name: "test-room", OwnerID: owner}
	for _, u := range members {
		repo.memberships[u+":"+id] = &model.ChatRoomMembership{UserID: u, RoomID: id}
	}
}

func TestCreateMessageToRoom_Success(t *testing.T) {
	svc, repo, pub := newSvc(t)
	seedRoom(t, repo, "r1", "alice", "bob")

	msg, err := svc.CreateMessageToRoom(context.Background(), "bob", "r1", "hello room", "")
	require.NoError(t, err)
	require.NotNil(t, msg.ToRoomID)
	assert.Equal(t, "r1", *msg.ToRoomID)
	require.Len(t, pub.roomCalls, 1)
	assert.Equal(t, "r1", pub.roomCalls[0].roomID)
	assert.Equal(t, corechat.EventMessage, pub.roomCalls[0].eventType)
}

func TestCreateMessageToRoom_OwnerIsImplicitMember(t *testing.T) {
	svc, repo, _ := newSvc(t)
	seedRoom(t, repo, "r1", "alice") // bob not a member

	_, err := svc.CreateMessageToRoom(context.Background(), "alice", "r1", "hi", "")
	require.NoError(t, err)
}

func TestCreateMessageToRoom_NotMember(t *testing.T) {
	svc, repo, _ := newSvc(t)
	seedRoom(t, repo, "r1", "alice")

	_, err := svc.CreateMessageToRoom(context.Background(), "carol", "r1", "hi", "")
	assert.ErrorIs(t, err, corechat.ErrForbidden)
}

func TestCreateMessageToRoom_RoomNotFound(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.CreateMessageToRoom(context.Background(), "alice", "ghost", "hi", "")
	assert.ErrorIs(t, err, corechat.ErrNotFound)
}

func TestCreateMessageToRoom_EmptyTarget(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.CreateMessageToRoom(context.Background(), "alice", "", "hi", "")
	assert.ErrorIs(t, err, corechat.ErrInvalidTarget)
}

func TestCreateMessageToRoom_WithFile(t *testing.T) {
	svc, repo, _ := newSvc(t)
	seedRoom(t, repo, "r1", "alice")
	msg, err := svc.CreateMessageToRoom(context.Background(), "alice", "r1", "", "file_y")
	require.NoError(t, err)
	require.NotNil(t, msg.FileID)
}

func TestCreateMessageToRoom_RepoError(t *testing.T) {
	svc, repo, _ := newSvc(t)
	seedRoom(t, repo, "r1", "alice")
	repo.createErr = errors.New("db boom")
	_, err := svc.CreateMessageToRoom(context.Background(), "alice", "r1", "hi", "")
	assert.Error(t, err)
}

// --- DeleteMessage ---

func TestDeleteMessage_UserDM(t *testing.T) {
	svc, _, pub := newSvc(t)
	msg, _ := svc.CreateMessageToUser(context.Background(), "alice", "bob", "hi", "")
	pub.userCalls = nil // reset

	require.NoError(t, svc.DeleteMessage(context.Background(), "alice", msg.ID))
	require.Len(t, pub.userCalls, 1)
	assert.Equal(t, corechat.EventDeleted, pub.userCalls[0].eventType)
}

func TestDeleteMessage_Room(t *testing.T) {
	svc, repo, pub := newSvc(t)
	seedRoom(t, repo, "r1", "alice")
	msg, _ := svc.CreateMessageToRoom(context.Background(), "alice", "r1", "hi", "")
	pub.roomCalls = nil

	require.NoError(t, svc.DeleteMessage(context.Background(), "alice", msg.ID))
	require.Len(t, pub.roomCalls, 1)
	assert.Equal(t, corechat.EventDeleted, pub.roomCalls[0].eventType)
}

func TestDeleteMessage_NotFound(t *testing.T) {
	svc, _, _ := newSvc(t)
	err := svc.DeleteMessage(context.Background(), "alice", "ghost")
	assert.ErrorIs(t, err, corechat.ErrNotFound)
}

func TestDeleteMessage_NotAuthor(t *testing.T) {
	svc, _, _ := newSvc(t)
	msg, _ := svc.CreateMessageToUser(context.Background(), "alice", "bob", "hi", "")
	err := svc.DeleteMessage(context.Background(), "carol", msg.ID)
	assert.ErrorIs(t, err, corechat.ErrForbidden)
}

func TestDeleteMessage_RepoError(t *testing.T) {
	svc, repo, _ := newSvc(t)
	msg, _ := svc.CreateMessageToUser(context.Background(), "alice", "bob", "hi", "")
	repo.deleteErr = errors.New("db boom")
	err := svc.DeleteMessage(context.Background(), "alice", msg.ID)
	assert.Error(t, err)
}

// --- UpdateMessage ---

func TestUpdateMessage_Success(t *testing.T) {
	svc, _, pub := newSvc(t)
	msg, _ := svc.CreateMessageToUser(context.Background(), "alice", "bob", "original", "")
	pub.userCalls = nil

	updated, err := svc.UpdateMessage(context.Background(), "alice", msg.ID, "edited")
	require.NoError(t, err)
	require.NotNil(t, updated.Text)
	assert.Equal(t, "edited", *updated.Text)
	require.Len(t, pub.userCalls, 1)
	assert.Equal(t, corechat.EventEdited, pub.userCalls[0].eventType)
}

func TestUpdateMessage_Room(t *testing.T) {
	svc, repo, pub := newSvc(t)
	seedRoom(t, repo, "r1", "alice")
	msg, _ := svc.CreateMessageToRoom(context.Background(), "alice", "r1", "original", "")
	pub.roomCalls = nil

	_, err := svc.UpdateMessage(context.Background(), "alice", msg.ID, "edited")
	require.NoError(t, err)
	require.Len(t, pub.roomCalls, 1)
	assert.Equal(t, corechat.EventEdited, pub.roomCalls[0].eventType)
}

func TestUpdateMessage_NotFound(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.UpdateMessage(context.Background(), "alice", "ghost", "x")
	assert.ErrorIs(t, err, corechat.ErrNotFound)
}

func TestUpdateMessage_NotAuthor(t *testing.T) {
	svc, _, _ := newSvc(t)
	msg, _ := svc.CreateMessageToUser(context.Background(), "alice", "bob", "hi", "")
	_, err := svc.UpdateMessage(context.Background(), "carol", msg.ID, "edited")
	assert.ErrorIs(t, err, corechat.ErrForbidden)
}

func TestUpdateMessage_RepoError(t *testing.T) {
	svc, repo, _ := newSvc(t)
	msg, _ := svc.CreateMessageToUser(context.Background(), "alice", "bob", "hi", "")
	repo.updateErr = errors.New("db boom")
	_, err := svc.UpdateMessage(context.Background(), "alice", msg.ID, "edited")
	assert.Error(t, err)
}

// --- MarkReadByMessageID ---

func TestMarkReadByMessageID_DM(t *testing.T) {
	svc, _, pub := newSvc(t)
	msg, _ := svc.CreateMessageToUser(context.Background(), "alice", "bob", "hi", "")
	pub.userCalls = nil

	require.NoError(t, svc.MarkReadByMessageID(context.Background(), "bob", msg.ID))
	require.Len(t, pub.userCalls, 1)
	assert.Equal(t, corechat.EventRead, pub.userCalls[0].eventType)
}

func TestMarkReadByMessageID_Room(t *testing.T) {
	svc, repo, pub := newSvc(t)
	seedRoom(t, repo, "r1", "alice", "bob")
	msg, _ := svc.CreateMessageToRoom(context.Background(), "alice", "r1", "hi", "")
	pub.roomCalls = nil

	require.NoError(t, svc.MarkReadByMessageID(context.Background(), "bob", msg.ID))
	require.Len(t, pub.roomCalls, 1)
	assert.Equal(t, corechat.EventRead, pub.roomCalls[0].eventType)
}

func TestMarkReadByMessageID_NotFound(t *testing.T) {
	svc, _, _ := newSvc(t)
	err := svc.MarkReadByMessageID(context.Background(), "bob", "ghost")
	assert.ErrorIs(t, err, corechat.ErrNotFound)
}

// --- IsRoomMember ---

func TestIsRoomMember_Member(t *testing.T) {
	svc, repo, _ := newSvc(t)
	seedRoom(t, repo, "r1", "alice", "bob")
	ok, err := svc.IsRoomMember("bob", "r1")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestIsRoomMember_Owner(t *testing.T) {
	svc, repo, _ := newSvc(t)
	seedRoom(t, repo, "r1", "alice") // alice is owner, no explicit membership
	ok, err := svc.IsRoomMember("alice", "r1")
	require.NoError(t, err)
	assert.True(t, ok, "owner should implicitly be a member")
}

func TestIsRoomMember_NonMember(t *testing.T) {
	svc, repo, _ := newSvc(t)
	seedRoom(t, repo, "r1", "alice")
	ok, err := svc.IsRoomMember("carol", "r1")
	require.NoError(t, err)
	assert.False(t, ok)
}

// --- nil publisher fallback ---

func TestService_NilPublisherStillWorks(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	svc := corechat.NewService(newFakeRepo(), idGen)
	// No publisher set; operations should still succeed (no panic)
	_, err := svc.CreateMessageToUser(context.Background(), "alice", "bob", "hi", "")
	assert.NoError(t, err)
}

// --- sanity: reads array mutation round-trip ---

func TestMarkRead_AppendsToReads(t *testing.T) {
	svc, repo, _ := newSvc(t)
	msg, _ := svc.CreateMessageToUser(context.Background(), "alice", "bob", "hi", "")
	require.NoError(t, svc.MarkReadByMessageID(context.Background(), "bob", msg.ID))
	stored := repo.messages[msg.ID]
	require.NotNil(t, stored)
	assert.Equal(t, pq.StringArray{"bob"}, stored.Reads)
}
