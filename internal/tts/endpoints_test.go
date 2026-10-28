package tts_test

import (
	"testing"

	"github.com/scuba-plaza/mufradat/internal/itest"
)

func TestEveryEndpointHasAnIntegrationTest(t *testing.T) {
	itest.EndpointsAreTested(t, "tts", "../../INTEGRATION_TESTS.md")
}
