package manager

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOpa(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Opa Suite")
}
