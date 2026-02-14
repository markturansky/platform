package sessions_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"gopkg.in/resty.v1"

	"github.com/ambient/platform/components/ambient-api-server/pkg/api/openapi"
	"github.com/ambient/platform/components/ambient-api-server/test"
)

func TestSessionGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, _, err := client.DefaultAPI.ApiAmbientApiServerV1SessionsIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	_, resp, err := client.DefaultAPI.ApiAmbientApiServerV1SessionsIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	sessionModel, err := newSession(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	sessionOutput, resp, err := client.DefaultAPI.ApiAmbientApiServerV1SessionsIdGet(ctx, sessionModel.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*sessionOutput.Id).To(Equal(sessionModel.ID), "found object does not match test object")
	Expect(*sessionOutput.Kind).To(Equal("Session"))
	Expect(*sessionOutput.Href).To(Equal(fmt.Sprintf("/api/ambient-api-server/v1/sessions/%s", sessionModel.ID)))
	Expect(*sessionOutput.CreatedAt).To(BeTemporally("~", sessionModel.CreatedAt))
	Expect(*sessionOutput.UpdatedAt).To(BeTemporally("~", sessionModel.UpdatedAt))
}

func TestSessionPost(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	creator, err := newUser("test-creator-post")
	Expect(err).NotTo(HaveOccurred(), "Error creating creator user")
	assignee, err := newUser("test-assignee-post")
	Expect(err).NotTo(HaveOccurred(), "Error creating assignee user")
	wf, err := newParentWorkflow()
	Expect(err).NotTo(HaveOccurred(), "Error creating parent workflow")

	sessionInput := openapi.Session{
		Name:            "test-name",
		RepoUrl:         openapi.PtrString("test-repo_url"),
		Prompt:          openapi.PtrString("test-prompt"),
		CreatedByUserId: openapi.PtrString(creator.ID),
		AssignedUserId:  openapi.PtrString(assignee.ID),
		WorkflowId:      openapi.PtrString(wf.ID),
	}

	sessionOutput, resp, err := client.DefaultAPI.ApiAmbientApiServerV1SessionsPost(ctx).Session(sessionInput).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*sessionOutput.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*sessionOutput.Kind).To(Equal("Session"))
	Expect(*sessionOutput.Href).To(Equal(fmt.Sprintf("/api/ambient-api-server/v1/sessions/%s", *sessionOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Post(h.RestURL("/sessions"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestSessionPatch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	sessionModel, err := newSession(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	sessionOutput, resp, err := client.DefaultAPI.ApiAmbientApiServerV1SessionsIdPatch(ctx, sessionModel.ID).SessionPatchRequest(openapi.SessionPatchRequest{}).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*sessionOutput.Id).To(Equal(sessionModel.ID))
	Expect(*sessionOutput.CreatedAt).To(BeTemporally("~", sessionModel.CreatedAt))
	Expect(*sessionOutput.Kind).To(Equal("Session"))
	Expect(*sessionOutput.Href).To(Equal(fmt.Sprintf("/api/ambient-api-server/v1/sessions/%s", *sessionOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Patch(h.RestURL("/sessions/foo"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestSessionPaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, err := newSessionList("Bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	list, _, err := client.DefaultAPI.ApiAmbientApiServerV1SessionsGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting session list: %v", err)
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultAPI.ApiAmbientApiServerV1SessionsGet(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting session list: %v", err)
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

func TestSessionListSearch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	sessions, err := newSessionList("bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	search := fmt.Sprintf("id in ('%s')", sessions[0].ID)
	list, _, err := client.DefaultAPI.ApiAmbientApiServerV1SessionsGet(ctx).Search(search).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting session list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(sessions[0].ID))
}
