package sessions

import (
	"github.com/ambient/platform/components/ambient-api-server/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/pkg/util"
)

func ConvertSession(session openapi.Session) *Session {
	c := &Session{
		Meta: api.Meta{
			ID: util.NilToEmptyString(session.Id),
		},
	}
	c.Name = session.Name
	c.RepoUrl = session.RepoUrl
	c.Prompt = session.Prompt
	c.CreatedByUserId = session.CreatedByUserId
	c.AssignedUserId = session.AssignedUserId
	c.WorkflowId = session.WorkflowId

	if session.CreatedAt != nil {
		c.CreatedAt = *session.CreatedAt
		c.UpdatedAt = *session.UpdatedAt
	}

	return c
}

func PresentSession(session *Session) openapi.Session {
	reference := presenters.PresentReference(session.ID, session)
	return openapi.Session{
		Id:              reference.Id,
		Kind:            reference.Kind,
		Href:            reference.Href,
		CreatedAt:       openapi.PtrTime(session.CreatedAt),
		UpdatedAt:       openapi.PtrTime(session.UpdatedAt),
		Name:            session.Name,
		RepoUrl:         session.RepoUrl,
		Prompt:          session.Prompt,
		CreatedByUserId: session.CreatedByUserId,
		AssignedUserId:  session.AssignedUserId,
		WorkflowId:      session.WorkflowId,
	}
}
