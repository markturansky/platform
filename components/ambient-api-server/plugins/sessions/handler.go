package sessions

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/ambient/platform/components/ambient-api-server/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/pkg/handlers"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
)

var _ handlers.RestHandler = sessionHandler{}

type sessionHandler struct {
	session SessionService
	generic services.GenericService
}

func NewSessionHandler(session SessionService, generic services.GenericService) *sessionHandler {
	return &sessionHandler{
		session: session,
		generic: generic,
	}
}

func (h sessionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var session openapi.Session
	cfg := &handlers.HandlerConfig{
		Body: &session,
		Validators: []handlers.Validate{
			handlers.ValidateEmpty(&session, "Id", "id"),
		},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			sessionModel := ConvertSession(session)
			sessionModel, err := h.session.Create(ctx, sessionModel)
			if err != nil {
				return nil, err
			}
			return PresentSession(sessionModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusCreated)
}

func (h sessionHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var patch openapi.SessionPatchRequest

	cfg := &handlers.HandlerConfig{
		Body:       &patch,
		Validators: []handlers.Validate{},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			id := mux.Vars(r)["id"]
			found, err := h.session.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			if patch.Name != nil {
				found.Name = *patch.Name
			}
			if patch.RepoUrl != nil {
				found.RepoUrl = patch.RepoUrl
			}
			if patch.Prompt != nil {
				found.Prompt = patch.Prompt
			}
			if patch.CreatedByUserId != nil {
				found.CreatedByUserId = patch.CreatedByUserId
			}
			if patch.AssignedUserId != nil {
				found.AssignedUserId = patch.AssignedUserId
			}
			if patch.WorkflowId != nil {
				found.WorkflowId = patch.WorkflowId
			}

			sessionModel, err := h.session.Replace(ctx, found)
			if err != nil {
				return nil, err
			}
			return PresentSession(sessionModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusOK)
}

func (h sessionHandler) List(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()

			listArgs := services.NewListArguments(r.URL.Query())
			var sessions []Session
			paging, err := h.generic.List(ctx, "id", listArgs, &sessions)
			if err != nil {
				return nil, err
			}
			sessionList := openapi.SessionList{
				Kind:  "SessionList",
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.Session{},
			}

			for _, session := range sessions {
				converted := PresentSession(&session)
				sessionList.Items = append(sessionList.Items, converted)
			}
			if listArgs.Fields != nil {
				filteredItems, err := presenters.SliceFilter(listArgs.Fields, sessionList.Items)
				if err != nil {
					return nil, err
				}
				return filteredItems, nil
			}
			return sessionList, nil
		},
	}

	handlers.HandleList(w, r, cfg)
}

func (h sessionHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			session, err := h.session.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			return PresentSession(session), nil
		},
	}

	handlers.HandleGet(w, r, cfg)
}

func (h sessionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			err := h.session.Delete(ctx, id)
			if err != nil {
				return nil, err
			}
			return nil, nil
		},
	}
	handlers.HandleDelete(w, r, cfg, http.StatusNoContent)
}
