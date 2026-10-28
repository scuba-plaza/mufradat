package mistral_test

import (
	"testing"

	"github.com/scuba-plaza/mufradat/internal/itest"
)

func TestEveryEndpointHasAnIntegrationTest(t *testing.T) {
	itest.EndpointsAreTested(t, "mistral", "../../INTEGRATION_TESTS.md")
}
