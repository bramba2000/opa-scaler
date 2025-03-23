package e2e

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/bramba2000/opa-scaler/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("opaengine", Ordered, Focus, func() {
	Context("Operator", func() {
		// projectimage stores the name of the image used in the example
		var projectimage = "example.com/opa-scaler:v0.0.1"

		BeforeAll(func() {
			By("building the manager(Operator) image")
			cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectimage))
			_, err := utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("installing CRDs")
			cmd = exec.Command("make", "install")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("deploying the controller-manager")
			cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			By("deleting all opaengine resources")
			cmd := exec.Command("kubectl", "delete", "opaengine", "--all", "-n", namespace)
			_, _ = utils.Run(cmd)
		})

		It("should run successfully", func() {
			var controllerPodName string

			verifyControllerUp := func() error {
				// Get pod name

				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				if len(podNames) != 1 {
					return fmt.Errorf("expect 1 controller pods running, but got %d", len(podNames))
				}
				controllerPodName = podNames[0]
				ExpectWithOffset(2, controllerPodName).Should(ContainSubstring("controller-manager"))

				// Validate pod status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				status, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("controller pod in %s status", status)
				}
				return nil
			}
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())
		})

		It("should create deployment and service on new opaengine creation", func() {
			By("creating opaengine")
			cmd := exec.Command("kubectl", "apply", "-f", "config/samples/v1alpha1_opaengine.yaml", "-n", namespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("validating that the deployment and service are created")
			verifyDeploymentService := func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "service", "opaengine-sample", "-n", namespace)
				_, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command("kubectl", "get", "deployment", "opaengine-sample", "-n", namespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				cmd = exec.Command("kubectl", "get", "opaengine", "opaengine-sample", "-n", namespace, "-o", "json")
				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				var opaEngineData map[string]interface{}
				g.Expect(json.Unmarshal(podOutput, &opaEngineData)).To(Succeed())
				g.Expect(opaEngineData["status"]).ToNot(BeNil())
				g.Expect(opaEngineData["status"].(map[string]interface{})["conditions"]).ToNot(BeNil())
				conditionLen := len(opaEngineData["status"].(map[string]interface{})["conditions"].([]interface{}))
				g.Expect(conditionLen).To(BeNumerically(">", 0))
				g.Expect(opaEngineData["status"].(map[string]interface{})["conditions"].([]interface{})[conditionLen-1]).ToNot(BeNil())
				g.Expect(opaEngineData["status"].(map[string]interface{})["conditions"].([]interface{})[conditionLen-1].(map[string]interface{})["type"]).To(Equal("Available"))
				g.Expect(opaEngineData["status"].(map[string]interface{})["conditions"].([]interface{})[conditionLen-1].(map[string]interface{})["status"]).To(Equal("True"))
			}
			EventuallyWithOffset(1, verifyDeploymentService, time.Second*10, time.Second).Should(Succeed())
		})

		It("should delete deployment and service on opaengine deletion", func() {
			By("creating opaengine")
			cmd := exec.Command("kubectl", "apply", "-f", "config/samples/v1alpha1_opaengine.yaml", "-n", namespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("deleting opaengine")
			cmd = exec.Command("kubectl", "delete", "opaengine", "opaengine-sample", "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("validating that the deployment and service are deleted")
			verifyDeploymentService := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "service", "opaengine-sample", "-n", namespace)
				out, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
				g.Expect(out).To(ContainSubstring("not found"))

				cmd = exec.Command("kubectl", "get", "deployment", "opaengine-sample", "-n", namespace)
				out, err = utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
				g.Expect(out).To(ContainSubstring("not found"))
			}
			EventuallyWithOffset(1, verifyDeploymentService, time.Second*10, time.Second).Should(Succeed())
		})

		It("should push the policy in the spec to the engine", Focus, func() {
			By("creating opaengine")
			cmd := exec.Command("kubectl", "apply", "-f", "config/samples/v1alpha1_opaengine.yaml", "-n", namespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			cmd = exec.Command("kubectl", "wait", "deployment", "opaengine-sample", "--for", "condition=Available", "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("exposing the opaengine service")
			forward := exec.Command("kubectl", "port-forward", "svc/opaengine-sample", "8181:8181", "-n", namespace)
			err = utils.Start(forward)

			Expect(err).NotTo(HaveOccurred())

			By("alter the policy in the spec")
			cmd = exec.Command("kubectl", "patch", "opaengine", "opaengine-sample", "-n", namespace, "--type", "merge", "-p", `{"spec": {"policies": ["test-policy"]}}`)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("validating that the policy is pushed to the engine")
			Eventually(func(g Gomega) {
				cmd := exec.Command("curl", "localhost:8181/v1/policies/test-policy")
				out, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(ContainSubstring("test-policy"))
				g.Expect(out).To(ContainSubstring("result"))
			}).Should(Succeed())

		})
	})
})
