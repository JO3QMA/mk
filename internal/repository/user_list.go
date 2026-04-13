package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// UserListRepository provides data access for user lists.
type UserListRepository interface {
	Create(list *model.UserList) error
	FindByID(id string) (*model.UserList, error)
	ListByUser(userID string) ([]*model.UserList, error)
	Delete(id string) error
	AddMember(m *model.UserListMembership) error
	RemoveMember(listID, userID string) error
	ListMembers(listID string) ([]*model.UserListMembership, error)
	UpdateList(id string, fields map[string]any) error
	UpdateMembership(listID, userID string, withReplies bool) error
}

type userListRepository struct {
	db *gorm.DB
}

func NewUserListRepository(db *gorm.DB) UserListRepository {
	return &userListRepository{db: db}
}

func (r *userListRepository) Create(list *model.UserList) error {
	return r.db.Create(list).Error
}

func (r *userListRepository) FindByID(id string) (*model.UserList, error) {
	var list model.UserList
	if err := r.db.Where("id = ?", id).First(&list).Error; err != nil {
		return nil, err
	}
	return &list, nil
}

func (r *userListRepository) ListByUser(userID string) ([]*model.UserList, error) {
	var lists []*model.UserList
	if err := r.db.Where("\"userId\" = ?", userID).Order("id DESC").Find(&lists).Error; err != nil {
		return nil, err
	}
	return lists, nil
}

func (r *userListRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.UserList{}).Error
}

func (r *userListRepository) AddMember(m *model.UserListMembership) error {
	return r.db.Create(m).Error
}

func (r *userListRepository) RemoveMember(listID, userID string) error {
	return r.db.Where("\"userListId\" = ? AND \"userId\" = ?", listID, userID).
		Delete(&model.UserListMembership{}).Error
}

func (r *userListRepository) ListMembers(listID string) ([]*model.UserListMembership, error) {
	var members []*model.UserListMembership
	if err := r.db.Preload("User").Where("\"userListId\" = ?", listID).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (r *userListRepository) UpdateList(id string, fields map[string]any) error {
	return r.db.Model(&model.UserList{}).Where("id = ?", id).Updates(fields).Error
}

func (r *userListRepository) UpdateMembership(listID, userID string, withReplies bool) error {
	result := r.db.Model(&model.UserListMembership{}).
		Where("\"userListId\" = ? AND \"userId\" = ?", listID, userID).
		Update("withReplies", withReplies)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
