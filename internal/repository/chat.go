package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// ChatRepository handles chat-related persistence.
type ChatRepository interface {
	// Room operations
	CreateRoom(room *model.ChatRoom) error
	FindRoomByID(id string) (*model.ChatRoom, error)
	UpdateRoom(room *model.ChatRoom) error
	DeleteRoom(id string) error
	ListRoomsByOwner(ownerID string) ([]*model.ChatRoom, error)
	ListJoinedRooms(userID string) ([]*model.ChatRoom, error)

	// Message operations
	CreateMessage(msg *model.ChatMessage) error
	FindMessageByID(id string) (*model.ChatMessage, error)
	DeleteMessage(id string) error
	ListMessagesByRoom(roomID string, limit int) ([]*model.ChatMessage, error)
	ListMessagesByUser(userID, otherUserID string, limit int) ([]*model.ChatMessage, error)
	SearchMessages(userID, query string, limit int) ([]*model.ChatMessage, error)

	// Membership operations
	CreateMembership(m *model.ChatRoomMembership) error
	FindMembership(userID, roomID string) (*model.ChatRoomMembership, error)
	UpdateMembership(m *model.ChatRoomMembership) error
	DeleteMembership(userID, roomID string) error
	ListMembersByRoom(roomID string) ([]*model.ChatRoomMembership, error)

	// Invitation operations
	CreateInvitation(inv *model.ChatRoomInvitation) error
	DeleteInvitation(id string) error
	FindInvitation(userID, roomID string) (*model.ChatRoomInvitation, error)

	// Unread count
	CountUnread(userID string) (int64, error)

	// Mark read
	MarkRead(userID, messageID string) error
}

type chatRepository struct {
	db *gorm.DB
}

// NewChatRepository creates a new ChatRepository.
func NewChatRepository(db *gorm.DB) ChatRepository {
	return &chatRepository{db: db}
}

func (r *chatRepository) CreateRoom(room *model.ChatRoom) error {
	return r.db.Create(room).Error
}

func (r *chatRepository) FindRoomByID(id string) (*model.ChatRoom, error) {
	var room model.ChatRoom
	if err := r.db.Preload("Owner").Where(`"id" = ?`, id).First(&room).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *chatRepository) UpdateRoom(room *model.ChatRoom) error {
	return r.db.Save(room).Error
}

func (r *chatRepository) DeleteRoom(id string) error {
	return r.db.Where(`"id" = ?`, id).Delete(&model.ChatRoom{}).Error
}

func (r *chatRepository) ListRoomsByOwner(ownerID string) ([]*model.ChatRoom, error) {
	var rooms []*model.ChatRoom
	if err := r.db.Where(`"ownerId" = ?`, ownerID).Order(`"id" DESC`).Find(&rooms).Error; err != nil {
		return nil, err
	}
	return rooms, nil
}

func (r *chatRepository) ListJoinedRooms(userID string) ([]*model.ChatRoom, error) {
	var rooms []*model.ChatRoom
	if err := r.db.Joins(`JOIN "chat_room_membership" ON "chat_room_membership"."roomId" = "chat_room"."id"`).
		Where(`"chat_room_membership"."userId" = ?`, userID).
		Order(`"chat_room"."id" DESC`).Find(&rooms).Error; err != nil {
		return nil, err
	}
	return rooms, nil
}

func (r *chatRepository) CreateMessage(msg *model.ChatMessage) error {
	return r.db.Create(msg).Error
}

func (r *chatRepository) FindMessageByID(id string) (*model.ChatMessage, error) {
	var msg model.ChatMessage
	if err := r.db.Preload("FromUser").Where(`"id" = ?`, id).First(&msg).Error; err != nil {
		return nil, err
	}
	return &msg, nil
}

func (r *chatRepository) DeleteMessage(id string) error {
	return r.db.Where(`"id" = ?`, id).Delete(&model.ChatMessage{}).Error
}

func (r *chatRepository) ListMessagesByRoom(roomID string, limit int) ([]*model.ChatMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	var msgs []*model.ChatMessage
	if err := r.db.Preload("FromUser").Where(`"toRoomId" = ?`, roomID).
		Order(`"id" DESC`).Limit(limit).Find(&msgs).Error; err != nil {
		return nil, err
	}
	return msgs, nil
}

func (r *chatRepository) ListMessagesByUser(userID, otherUserID string, limit int) ([]*model.ChatMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	var msgs []*model.ChatMessage
	if err := r.db.Preload("FromUser").
		Where(`("fromUserId" = ? AND "toUserId" = ?) OR ("fromUserId" = ? AND "toUserId" = ?)`,
			userID, otherUserID, otherUserID, userID).
		Order(`"id" DESC`).Limit(limit).Find(&msgs).Error; err != nil {
		return nil, err
	}
	return msgs, nil
}

func (r *chatRepository) SearchMessages(userID, query string, limit int) ([]*model.ChatMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	var msgs []*model.ChatMessage
	if err := r.db.Preload("FromUser").
		Where(`("fromUserId" = ? OR "toUserId" = ?) AND "text" ILIKE ?`, userID, userID, "%"+query+"%").
		Order(`"id" DESC`).Limit(limit).Find(&msgs).Error; err != nil {
		return nil, err
	}
	return msgs, nil
}

func (r *chatRepository) CreateMembership(m *model.ChatRoomMembership) error {
	return r.db.Create(m).Error
}

func (r *chatRepository) FindMembership(userID, roomID string) (*model.ChatRoomMembership, error) {
	var m model.ChatRoomMembership
	if err := r.db.Where(`"userId" = ? AND "roomId" = ?`, userID, roomID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *chatRepository) UpdateMembership(m *model.ChatRoomMembership) error {
	return r.db.Save(m).Error
}

func (r *chatRepository) DeleteMembership(userID, roomID string) error {
	return r.db.Where(`"userId" = ? AND "roomId" = ?`, userID, roomID).Delete(&model.ChatRoomMembership{}).Error
}

func (r *chatRepository) ListMembersByRoom(roomID string) ([]*model.ChatRoomMembership, error) {
	var members []*model.ChatRoomMembership
	if err := r.db.Preload("User").Where(`"roomId" = ?`, roomID).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (r *chatRepository) CreateInvitation(inv *model.ChatRoomInvitation) error {
	return r.db.Create(inv).Error
}

func (r *chatRepository) DeleteInvitation(id string) error {
	return r.db.Where(`"id" = ?`, id).Delete(&model.ChatRoomInvitation{}).Error
}

func (r *chatRepository) FindInvitation(userID, roomID string) (*model.ChatRoomInvitation, error) {
	var inv model.ChatRoomInvitation
	if err := r.db.Where(`"userId" = ? AND "roomId" = ?`, userID, roomID).First(&inv).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *chatRepository) CountUnread(userID string) (int64, error) {
	var count int64
	if err := r.db.Model(&model.ChatMessage{}).
		Where(`"toUserId" = ? AND NOT ("reads" @> ARRAY[?]::varchar[])`, userID, userID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *chatRepository) MarkRead(userID, messageID string) error {
	return r.db.Exec(`UPDATE "chat_message" SET "reads" = array_append("reads", ?) WHERE "id" = ? AND NOT ("reads" @> ARRAY[?]::varchar[])`,
		userID, messageID, userID).Error
}
