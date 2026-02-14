package sessions

import (
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"gorm.io/gorm"
)

type Session struct {
	api.Meta
	Name            string  `json:"name"`
	RepoUrl         *string `json:"repo_url"`
	Prompt          *string `json:"prompt"`
	CreatedByUserId *string `json:"created_by_user_id"`
	AssignedUserId  *string `json:"assigned_user_id"`
	WorkflowId      *string `json:"workflow_id"`
}

type SessionList []*Session
type SessionIndex map[string]*Session

func (l SessionList) Index() SessionIndex {
	index := SessionIndex{}
	for _, o := range l {
		index[o.ID] = o
	}
	return index
}

func (d *Session) BeforeCreate(tx *gorm.DB) error {
	d.ID = api.NewID()
	return nil
}

type SessionPatchRequest struct {
	Name            *string `json:"name,omitempty"`
	RepoUrl         *string `json:"repo_url,omitempty"`
	Prompt          *string `json:"prompt,omitempty"`
	CreatedByUserId *string `json:"created_by_user_id,omitempty"`
	AssignedUserId  *string `json:"assigned_user_id,omitempty"`
	WorkflowId      *string `json:"workflow_id,omitempty"`
}
