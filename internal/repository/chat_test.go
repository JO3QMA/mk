package repository

import (
	"testing"

	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatRepository_Rooms(t *testing.T) {
	repo := NewChatRepository(testDB)
	user := insertTestUser(t, "u_chat_1", "chatuser1")
	defer cleanupUser(t, user.ID)

	// CreateRoom
	room := &model.ChatRoom{ID: "cr_1", Name: "Test Room", OwnerID: user.ID, Description: "desc"}
	require.NoError(t, repo.CreateRoom(room))
	defer testDB.Exec(`DELETE FROM "chat_room" WHERE id = ?`, room.ID)

	// FindRoomByID
	found, err := repo.FindRoomByID("cr_1")
	require.NoError(t, err)
	assert.Equal(t, "Test Room", found.Name)

	// FindRoomByID - not found
	_, err = repo.FindRoomByID("ghost")
	assert.Error(t, err)

	// UpdateRoom
	found.Name = "Updated"
	require.NoError(t, repo.UpdateRoom(found))

	// ListRoomsByOwner
	rooms, err := repo.ListRoomsByOwner(user.ID)
	require.NoError(t, err)
	assert.Len(t, rooms, 1)
	assert.Equal(t, "Updated", rooms[0].Name)

	// ListJoinedRooms (empty - no membership yet)
	joined, err := repo.ListJoinedRooms(user.ID)
	require.NoError(t, err)
	assert.Empty(t, joined)

	// DeleteRoom
	require.NoError(t, repo.DeleteRoom("cr_1"))
	_, err = repo.FindRoomByID("cr_1")
	assert.Error(t, err)
}

func TestChatRepository_Messages(t *testing.T) {
	repo := NewChatRepository(testDB)
	user1 := insertTestUser(t, "u_chat_2", "chatuser2")
	user2 := insertTestUser(t, "u_chat_3", "chatuser3")
	defer cleanupUser(t, user1.ID)
	defer cleanupUser(t, user2.ID)

	room := &model.ChatRoom{ID: "cr_2", Name: "Room", OwnerID: user1.ID}
	require.NoError(t, repo.CreateRoom(room))
	defer testDB.Exec(`DELETE FROM "chat_room" WHERE id = ?`, room.ID)

	// CreateMessage (room message)
	msg := &model.ChatMessage{
		ID: "cm_1", FromUserID: user1.ID, ToRoomID: &room.ID,
		Reads: pq.StringArray{}, Reactions: pq.StringArray{},
	}
	text := "hello"
	msg.Text = &text
	require.NoError(t, repo.CreateMessage(msg))
	defer testDB.Exec(`DELETE FROM "chat_message" WHERE id = ?`, msg.ID)

	// FindMessageByID
	found, err := repo.FindMessageByID("cm_1")
	require.NoError(t, err)
	assert.Equal(t, "hello", *found.Text)

	// FindMessageByID - not found
	_, err = repo.FindMessageByID("ghost")
	assert.Error(t, err)

	// ListMessagesByRoom
	msgs, err := repo.ListMessagesByRoom(room.ID, 10)
	require.NoError(t, err)
	assert.Len(t, msgs, 1)

	// ListMessagesByRoom - default limit
	msgs2, err := repo.ListMessagesByRoom(room.ID, 0)
	require.NoError(t, err)
	assert.Len(t, msgs2, 1)

	// CreateMessage (DM)
	dm := &model.ChatMessage{
		ID: "cm_2", FromUserID: user1.ID, ToUserID: &user2.ID,
		Reads: pq.StringArray{}, Reactions: pq.StringArray{},
	}
	dmText := "dm"
	dm.Text = &dmText
	require.NoError(t, repo.CreateMessage(dm))
	defer testDB.Exec(`DELETE FROM "chat_message" WHERE id = ?`, dm.ID)

	// ListMessagesByUser
	dms, err := repo.ListMessagesByUser(user1.ID, user2.ID, 10)
	require.NoError(t, err)
	assert.Len(t, dms, 1)

	// ListMessagesByUser - default limit
	dms2, err := repo.ListMessagesByUser(user1.ID, user2.ID, 0)
	require.NoError(t, err)
	assert.Len(t, dms2, 1)

	// SearchMessages
	results, err := repo.SearchMessages(user1.ID, "dm", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// SearchMessages - default limit
	results2, err := repo.SearchMessages(user1.ID, "dm", 0)
	require.NoError(t, err)
	assert.Len(t, results2, 1)

	// MarkRead
	require.NoError(t, repo.MarkRead(user2.ID, "cm_2"))

	// CountUnread
	count, err := repo.CountUnread(user2.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// DeleteMessage
	require.NoError(t, repo.DeleteMessage("cm_1"))
}

func TestChatRepository_Membership(t *testing.T) {
	repo := NewChatRepository(testDB)
	user := insertTestUser(t, "u_chat_4", "chatuser4")
	defer cleanupUser(t, user.ID)

	room := &model.ChatRoom{ID: "cr_3", Name: "Room", OwnerID: user.ID}
	require.NoError(t, repo.CreateRoom(room))
	defer testDB.Exec(`DELETE FROM "chat_room" WHERE id = ?`, room.ID)

	// CreateMembership
	mem := &model.ChatRoomMembership{ID: "mem_1", UserID: user.ID, RoomID: room.ID}
	require.NoError(t, repo.CreateMembership(mem))
	defer testDB.Exec(`DELETE FROM "chat_room_membership" WHERE id = ?`, mem.ID)

	// FindMembership
	found, err := repo.FindMembership(user.ID, room.ID)
	require.NoError(t, err)
	assert.Equal(t, false, found.IsMuted)

	// FindMembership - not found
	_, err = repo.FindMembership("ghost", room.ID)
	assert.Error(t, err)

	// UpdateMembership
	found.IsMuted = true
	require.NoError(t, repo.UpdateMembership(found))

	// ListMembersByRoom
	members, err := repo.ListMembersByRoom(room.ID)
	require.NoError(t, err)
	assert.Len(t, members, 1)

	// ListJoinedRooms (now has membership)
	joined, err := repo.ListJoinedRooms(user.ID)
	require.NoError(t, err)
	assert.Len(t, joined, 1)

	// DeleteMembership
	require.NoError(t, repo.DeleteMembership(user.ID, room.ID))
}

func TestChatRepository_Invitation(t *testing.T) {
	repo := NewChatRepository(testDB)
	user := insertTestUser(t, "u_chat_5", "chatuser5")
	defer cleanupUser(t, user.ID)

	room := &model.ChatRoom{ID: "cr_4", Name: "Room", OwnerID: user.ID}
	require.NoError(t, repo.CreateRoom(room))
	defer testDB.Exec(`DELETE FROM "chat_room" WHERE id = ?`, room.ID)

	// CreateInvitation
	inv := &model.ChatRoomInvitation{ID: "inv_1", UserID: user.ID, RoomID: room.ID}
	require.NoError(t, repo.CreateInvitation(inv))
	defer testDB.Exec(`DELETE FROM "chat_room_invitation" WHERE id = ?`, inv.ID)

	// FindInvitation
	found, err := repo.FindInvitation(user.ID, room.ID)
	require.NoError(t, err)
	assert.Equal(t, "inv_1", found.ID)

	// FindInvitation - not found
	_, err = repo.FindInvitation("ghost", room.ID)
	assert.Error(t, err)

	// DeleteInvitation
	require.NoError(t, repo.DeleteInvitation("inv_1"))
}
