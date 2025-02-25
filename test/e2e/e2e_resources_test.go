package e2e

import (
	"os/exec"
	"strings"

	"github.com/bramba2000/opa-scaler/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("resources", Ordered, func() {
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	AfterAll(func() {
		By("removing manager namespace")
		cmd := exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	Context("Policy", func() {
		BeforeAll(func() {
			By("installing the CRDs")
			cmd := exec.Command("make", "install")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		policyMeta := `apiVersion: opas.polimi.it/v1alpha1
kind: Policy
metadata:
  labels:
    app.kubernetes.io/name: opa-scaler
    app.kubernetes.io/managed-by: kustomize
  name: policy
  namespace: ` + namespace

		It("should create a policy with rego code", func() {
			policy := policyMeta + `
spec:
  rego: |
    package main
    default allow := false`
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(policy)
			GinkgoWriter.Println(policy)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create a policy with an image", func() {
			policy := policyMeta + `
spec:
  image: "https://ghcr.io/example/example-policy:v0.0.1"`
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(policy)
			GinkgoWriter.Println(policy)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not create a policy without rego code or image", func() {
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(policyMeta)
			_, err := utils.Run(cmd)
			Expect(err).To(HaveOccurred())
		})
	})
})
