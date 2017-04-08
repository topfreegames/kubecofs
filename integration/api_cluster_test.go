// mystack-controller api
// +build integration
// https://github.com/topfreegames/mystack-controller
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package integration_test

import (
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/topfreegames/mystack-controller/api"

	"net/http"
	"net/http/httptest"
)

var _ = Describe("Cluster", func() {

	var (
		recorder       *httptest.ResponseRecorder
		clusterName    = "myCustomApps"
		clusterHandler *ClusterHandler
		yaml1          = `
services:
  test0:
    image: svc1
    port: 5000
apps:
  test1:
    image: app1
    port: 5000
`
	)

	BeforeEach(func() {
		recorder = httptest.NewRecorder()
		clusterHandler = &ClusterHandler{App: app}
	})

	Describe("PUT /clusters/{name}/run", func() {

		var (
			err     error
			request *http.Request
			route   = fmt.Sprintf("/clusters/%s/run", clusterName)
		)

		BeforeEach(func() {
			clusterHandler.Method = "run"
			request, err = http.NewRequest("PUT", route, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should run existing clusterName", func() {
			route = fmt.Sprintf("/cluster-configs/%s/create", clusterName)
			createRequest, err := http.NewRequest("PUT", route, nil)
			Expect(err).NotTo(HaveOccurred())

			clusterConfigHandler := &ClusterConfigHandler{App: app, Method: "create"}
			ctx := NewContextWithClusterConfig(createRequest.Context(), yaml1)
			clusterConfigHandler.ServeHTTP(recorder, createRequest.WithContext(ctx))

			recorder = httptest.NewRecorder()
			ctx = NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			Expect(recorder.Body.String()).To(Equal(`{"status": "ok"}`))
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("should return error 422 when run non existing clusterName", func() {
			ctx := NewContextWithEmail(request.Context(), "derp@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			//TODO: change this to 422 (THIS IS URGENT)
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			bodyJSON := make(map[string]string)
			json.Unmarshal(recorder.Body.Bytes(), &bodyJSON)
			Expect(bodyJSON["code"]).To(Equal("OFF-001"))
			Expect(bodyJSON["description"]).To(Equal("sql: no rows in result set"))
			Expect(bodyJSON["error"]).To(Equal("Error creating cluster"))
		})
	})

	Describe("PUT /clusters/{name}/delete", func() {

		var (
			err     error
			request *http.Request
			route   = fmt.Sprintf("/clusters/%s/delete", clusterName)
		)

		BeforeEach(func() {
			clusterHandler.Method = "delete"
			request, err = http.NewRequest("PUT", route, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete existing clusterName", func() {
			route = fmt.Sprintf("/cluster-configs/%s/create", clusterName)
			createRequest, _ := http.NewRequest("PUT", route, nil)
			clusterConfigHandler := &ClusterConfigHandler{App: app, Method: "create"}
			ctx := NewContextWithClusterConfig(createRequest.Context(), yaml1)
			clusterConfigHandler.ServeHTTP(recorder, createRequest.WithContext(ctx))

			clusterHandler.Method = "run"
			route = fmt.Sprintf("/clusters/%s/run", clusterName)
			runRequest, _ := http.NewRequest("PUT", route, nil)
			recorder = httptest.NewRecorder()
			ctx = NewContextWithEmail(runRequest.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, runRequest.WithContext(ctx))

			clusterHandler.Method = "delete"
			recorder = httptest.NewRecorder()
			ctx = NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Code).To(Equal(http.StatusOK))
			Expect(recorder.Body.String()).To(Equal(`{"status": "ok"}`))
			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
		})

		It("should return error 422 when deleting non existing clusterName", func() {
			ctx := NewContextWithEmail(request.Context(), "derp@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			//TODO: change this to 422 (THIS IS URGENT)
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			bodyJSON := make(map[string]string)
			json.Unmarshal(recorder.Body.Bytes(), &bodyJSON)
			Expect(bodyJSON["code"]).To(Equal("OFF-001"))
			Expect(bodyJSON["description"]).To(Equal("sql: no rows in result set"))
			Expect(bodyJSON["error"]).To(Equal("Error retrieving cluster"))
		})
	})
})
