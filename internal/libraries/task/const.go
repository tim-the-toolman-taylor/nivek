package task

import (
	"time"

	"github.com/google/uuid"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

const TableTask = "task"

func getTaskTable(nivek nivek.NivekService) db.Collection {
	return nivek.Postgres().GetDefaultConnection().Collection(TableTask)
}

type PriorityLevel string
type TaskStatus string

const (
	PriorityLow    PriorityLevel = "low"
	PriorityMedium PriorityLevel = "medium"
	PriorityHigh   PriorityLevel = "high"
	PriorityUrgent PriorityLevel = "urgent"
)

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusCompleted  TaskStatus = "completed"
	StatusCancelled  TaskStatus = "cancelled"
)

type Task struct {
	Id   int       `json:"id" db:"id"`     // not null
	Uuid uuid.UUID `json:"uuid" db:"uuid"` // not null

	UserId int `json:"user_id" db:"user_id"`

	Title       string        `json:"title" db:"title"` // not null
	Description *string       `json:"description,omitempty" db:"description"`
	Priority    PriorityLevel `json:"priority" db:"priority"` // enum
	Status      TaskStatus    `json:"status" db:"status"`     // enum

	ExpiresAt   *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`

	IsImportant       bool `json:"is_important" db:"is_important"`
	Position          int  `json:"position" db:"position"`
	EstimatedDuration *int `json:"estimated_duration,omitempty" db:"estimated_duration"`
	ActualDuration    int  `json:"actual_duration,omitempty" db:"actual_duration"`
}

type CreateTaskRequest struct {
	Title             string        `json:"title" validate:"required,max=255"`
	Description       *string       `json:"description,omitempty"`
	Priority          PriorityLevel `json:"priority,omitempty"`
	ExpiresAt         *time.Time    `json:"expires_at,omitempty"`
	IsImportant       bool          `json:"is_important,omitempty"`
	EstimatedDuration *int          `json:"estimated_duration,omitempty"`
}
