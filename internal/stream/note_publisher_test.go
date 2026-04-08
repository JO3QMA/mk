package stream

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	coredrive "github.com/shiroha-a/mk/internal/core/drive"
	corenotification "github.com/shiroha-a/mk/internal/core/notification"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubPublisher captures Publish calls.
type stubPublisher struct {
	mu       sync.Mutex
	topics   []string
	payloads []any
	err      error
}

func (s *stubPublisher) Publish(_ context.Context, channel string, payload any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.topics = append(s.topics, channel)
	s.payloads = append(s.payloads, payload)
	return nil
}

func TestNotePublisher_PublishesPackedNote(t *testing.T) {
	pub := &stubPublisher{}
	idGen, _ := id.NewGenerator("aidx")
	np := NewNotePublisher(pub, idGen)

	text := "hello"
	n := &model.Note{
		ID:         idGen.Generate(time.Now()),
		UserID:     "u1",
		Text:       &text,
		Visibility: model.NoteVisibilityPublic,
	}
	user := &model.User{ID: "u1", Username: "alice"}
	np.PublishNote("homeTimeline:u1", n, user)

	require.Len(t, pub.topics, 1)
	assert.Equal(t, "homeTimeline:u1", pub.topics[0])
}

func TestNotePublisher_NilPubIsNoOp(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	np := NewNotePublisher(nil, idGen)
	np.PublishNote("topic", &model.Note{}, &model.User{})
}

func TestNotePublisher_NilNoteIsNoOp(t *testing.T) {
	pub := &stubPublisher{}
	idGen, _ := id.NewGenerator("aidx")
	np := NewNotePublisher(pub, idGen)
	np.PublishNote("topic", nil, &model.User{})
	assert.Empty(t, pub.topics)
}

func TestNotePublisher_NilAuthorIsNoOp(t *testing.T) {
	pub := &stubPublisher{}
	idGen, _ := id.NewGenerator("aidx")
	np := NewNotePublisher(pub, idGen)
	np.PublishNote("topic", &model.Note{}, nil)
	assert.Empty(t, pub.topics)
}

func TestNotePublisher_MarshalErrorIsLoggedNotPropagated(t *testing.T) {
	pub := &stubPublisher{}
	idGen, _ := id.NewGenerator("aidx")
	np := NewNotePublisher(pub, idGen)
	// datatypes.JSON は invalid JSON で MarshalJSON が失敗する
	n := &model.Note{
		ID:        idGen.Generate(time.Now()),
		UserID:    "u1",
		Reactions: []byte("{not json"),
	}
	np.PublishNote("topic", n, &model.User{ID: "u1"})
	// publish 自体は呼ばれない (Marshal 失敗で early return)
	assert.Empty(t, pub.topics)
}

func TestNotePublisher_PublishErrorIsLoggedNotPropagated(t *testing.T) {
	pub := &stubPublisher{err: errors.New("redis down")}
	idGen, _ := id.NewGenerator("aidx")
	np := NewNotePublisher(pub, idGen)

	n := &model.Note{ID: idGen.Generate(time.Now()), UserID: "u1"}
	np.PublishNote("topic", n, &model.User{ID: "u1"})
	// Publish errored but the call still returned without panic.
}

// --- NotificationPublisher --------------------------------------------------

func TestNotificationPublisher_PublishesPayload(t *testing.T) {
	pub := &stubPublisher{}
	np := NewNotificationPublisher(pub)
	n := &corenotification.Notification{ID: "n1", Type: corenotification.TypeFollow, NotifierID: "u2"}
	np.PublishNotification("alice", n)
	require.Len(t, pub.topics, 1)
	assert.Equal(t, "notifications:alice", pub.topics[0])
}

func TestNotificationPublisher_NilPubIsNoOp(t *testing.T) {
	np := NewNotificationPublisher(nil)
	np.PublishNotification("alice", &corenotification.Notification{})
}

func TestNotificationPublisher_NilNotificationIsNoOp(t *testing.T) {
	pub := &stubPublisher{}
	np := NewNotificationPublisher(pub)
	np.PublishNotification("alice", nil)
	assert.Empty(t, pub.topics)
}

func TestNotificationPublisher_EmptyNotifieeIsNoOp(t *testing.T) {
	pub := &stubPublisher{}
	np := NewNotificationPublisher(pub)
	np.PublishNotification("", &corenotification.Notification{})
	assert.Empty(t, pub.topics)
}

func TestNotificationPublisher_PublishErrorIsLogged(t *testing.T) {
	pub := &stubPublisher{err: errors.New("redis down")}
	np := NewNotificationPublisher(pub)
	np.PublishNotification("alice", &corenotification.Notification{ID: "n1"})
}

func TestNotificationPublisher_MarshalErrorIsLogged(t *testing.T) {
	pub := &stubPublisher{}
	np := NewNotificationPublisher(pub)
	// chan は JSON.Marshal で失敗する
	np.PublishNotification("alice", &corenotification.Notification{
		ID:    "n1",
		Extra: map[string]any{"ch": make(chan int)},
	})
	assert.Empty(t, pub.topics)
}

// --- DrivePublisher ---------------------------------------------------------

func TestDrivePublisher_PublishesEvent(t *testing.T) {
	pub := &stubPublisher{}
	dp := NewDrivePublisher(pub)
	f := &model.DriveFile{ID: "f1", Name: "x.png"}
	dp.PublishDriveEvent("alice", "fileCreated", f)
	require.Len(t, pub.topics, 1)
	assert.Equal(t, "drive:alice", pub.topics[0])
}

func TestDrivePublisher_NilPubIsNoOp(t *testing.T) {
	dp := NewDrivePublisher(nil)
	dp.PublishDriveEvent("alice", "fileCreated", &model.DriveFile{})
}

func TestDrivePublisher_EmptyArgsAreNoOps(t *testing.T) {
	pub := &stubPublisher{}
	dp := NewDrivePublisher(pub)
	dp.PublishDriveEvent("", "fileCreated", &model.DriveFile{})
	dp.PublishDriveEvent("alice", "", &model.DriveFile{})
	dp.PublishDriveEvent("alice", "fileCreated", nil)
	assert.Empty(t, pub.topics)
}

func TestDrivePublisher_PublishErrorIsLogged(t *testing.T) {
	pub := &stubPublisher{err: errors.New("redis down")}
	dp := NewDrivePublisher(pub)
	dp.PublishDriveEvent("alice", "fileCreated", &model.DriveFile{ID: "f1"})
}

// DrivePublisher が core/drive.StreamingPublisher を実装していることの動的確認。
// 静的アサーションは note_publisher.go に置いてある。
func TestDrivePublisher_ImplementsServiceInterface(t *testing.T) {
	var _ coredrive.StreamingPublisher = (*DrivePublisher)(nil)
}

func TestDrivePublisher_MarshalErrorIsLogged(t *testing.T) {
	pub := &stubPublisher{}
	dp := NewDrivePublisher(pub)
	// datatypes.JSON は invalid JSON で MarshalJSON が失敗する
	f := &model.DriveFile{ID: "f1", Properties: []byte("{not json")}
	dp.PublishDriveEvent("alice", "fileCreated", f)
	assert.Empty(t, pub.topics)
}

// NotificationPublisher が core/notification.StreamingPublisher を実装している
// ことを確認。
func TestNotificationPublisher_ImplementsServiceInterface(t *testing.T) {
	var _ corenotification.StreamingPublisher = (*NotificationPublisher)(nil)
}
