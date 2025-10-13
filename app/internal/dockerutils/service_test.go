package dockerutils

import (
	"strings"
	"testing"
)

func TestListAllContainerLabels(t *testing.T) {
	labels, err := ListAllContainerLabels()

	if err != nil {

		if strings.Contains(err.Error(), "Cannot connect to the Docker daemon") {
			t.Skip("Docker daemon not available, skipping test")
		}

		t.Fatalf("ListAllContainerLabels failed: %v", err)
	}

	if labels == nil {
		t.Error("Expected labels map to be non-nil")
	}

	for containerName, containerLabels := range labels {
		t.Logf("Container: %s", containerName)

		for labelName, labelValue := range containerLabels {
			t.Logf("Label:: %s = %s", labelName, labelValue)
		}
	}
}
