package manager

import (
	"context"
	"io"
	"net/http"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("opa policy manager", Ordered, func() {
	Context("policy reconciliation", func() {
		It("should add policy to empty actual list", func() {
			expected := []string{"policy1", "policy2"}
			actual := []string{}
			toBeAdded, toBeRemove := MergePolicies(expected, actual)
			Expect(toBeAdded).To(Equal(expected))
			Expect(toBeRemove).To(BeEmpty())
		})

		It("should add policy to actual list", func() {
			expected := []string{"policy1", "policy2"}
			actual := []string{"policy1"}
			toBeAdded, toBeRemove := MergePolicies(expected, actual)
			Expect(toBeAdded).To(Equal([]string{"policy2"}))
			Expect(toBeRemove).To(BeEmpty())
		})

		It("should remove policy from actual list", func() {
			expected := []string{"policy1"}
			actual := []string{"policy1", "policy2"}
			toBeAdded, toBeRemove := MergePolicies(expected, actual)
			Expect(toBeAdded).To(BeEmpty())
			Expect(toBeRemove).To(Equal([]string{"policy2"}))
		})

		It("should add and remove policies from actual list", func() {
			expected := []string{"policy1", "policy2"}
			actual := []string{"policy2", "policy3"}
			toBeAdded, toBeRemove := MergePolicies(expected, actual)
			Expect(toBeAdded).To(Equal([]string{"policy1"}))
			Expect(toBeRemove).To(Equal([]string{"policy3"}))
		})
	})

	Context("opa integration", func() {
		var url string = "http://localhost:8181"
		var cmd exec.Cmd

		BeforeEach(func() {
			cmd := exec.Command("opa", "run", "-s")
			err := cmd.Start()
			Expect(err).To(BeNil())
			time.Sleep(1 * time.Second)
		})

		It("should push policies", func() {
			By("pushing a policy")
			rule := `package test
default allow = false`
			err := PushPolicies(context.TODO(), url, map[string]string{"policy1": rule})
			Expect(err).To(BeNil())
			By("checking the policy is available in OPA")
			resp, err := http.Get(url + "/v1/policies/policy1")
			Expect(err).To(BeNil())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			By("checking the policy evaluation")
			resp.Body.Close()
			resp, err = http.Get(url + "/v1/data/test/allow")
			Expect(err).To(BeNil())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, err := io.ReadAll(resp.Body)
			Expect(err).To(BeNil())
			Expect(string(body)).To(Equal(`{"result":false}` + "\n"))
		})

		It("should delete a policy", func() {
			By("pushing a policy")
			rule := `package test
default allow = false`
			err := PushPolicies(context.TODO(), url, map[string]string{"policy1": rule})
			Expect(err).To(BeNil())
			By("deleting the policy")
			err = DeletePolicies(context.TODO(), url, []string{"policy1"})
			Expect(err).To(BeNil())
			By("checking the policy is not available in OPA")
			resp, err := http.Get(url + "/v1/policies/policy1")
			Expect(err).To(BeNil())
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		AfterEach(func() {
			cmd.Wait()
		})
	})
})
