package task

import (
	"fmt"

	"github.com/suuuth/nivek/internal/libraries/nivek"
	"github.com/suuuth/nivek/internal/libraries/user"
	"github.com/upper/db/v4"
)

type NivekTaskService interface {
	GetTasks(user *user.User) ([]Task, error)
	CreateTask(user *user.User, newTaskRequest *CreateTaskRequest) (db.ID, error)
}

type nivekTaskServiceImpl struct {
	nivek.NivekService
	db.Collection
}

func NewNivekTaskService(nivek nivek.NivekService) NivekTaskService {
	return &nivekTaskServiceImpl{
		nivek,
		getTaskTable(nivek),
	}
}

func (s *nivekTaskServiceImpl) GetTasks(user *user.User) ([]Task, error) {
	var tasks []Task
	if err := s.Collection.Find(db.Cond{"user_id": user.Id}).All(&tasks); err != nil {
		return nil, fmt.Errorf("error getting tasks for user %d from db: %w", user.Id, err)
	}

	return tasks, nil
}

func (s *nivekTaskServiceImpl) GetTask(user *user.User, taskId int) (*Task, error) {
	var task Task
	if err := s.Collection.Find(db.Cond{
		"id":      taskId,
		"user_id": user.Id,
	}).One(&task); err != nil {
		return nil, fmt.Errorf("error getting task id %d for user %d from db: %w", taskId, user.Id, err)
	}

	return &task, nil
}

func (s *nivekTaskServiceImpl) CreateTask(user *user.User, newTaskRequest *CreateTaskRequest) (db.ID, error) {
	newTask := Task{
		UserId: user.Id,

		Title:       newTaskRequest.Title,
		Description: newTaskRequest.Description,
		Priority:    newTaskRequest.Priority,
		Status:      StatusPending,

		ExpiresAt: newTaskRequest.ExpiresAt,

		IsImportant:       newTaskRequest.IsImportant,
		EstimatedDuration: newTaskRequest.EstimatedDuration,
	}

	result, err := s.Collection.Insert(newTask)
	if err != nil {
		return nil, fmt.Errorf("error creating new task psql record for user %d: %w", user.Id, err)
	}

	return result.ID(), nil
}
