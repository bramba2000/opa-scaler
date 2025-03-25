package manager

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

func MergePolicies(expected, actual []string) (toBeAdded, toBeRemove []string) {
	toBeAdded = []string{}
	toBeRemove = []string{}

	for _, p := range expected {
		found := false
		for _, a := range actual {
			if p == a {
				found = true
				break
			}
		}
		if !found {
			toBeAdded = append(toBeAdded, p)
		}
	}

	for _, p := range actual {
		found := false
		for _, a := range expected {
			if p == a {
				found = true
				break
			}
		}
		if !found {
			toBeRemove = append(toBeRemove, p)
		}
	}

	return toBeAdded, toBeRemove
}

func PushPolicies(ctx context.Context, opaUrl string, policies map[string]string) ([]string, error) {
	logger := log.FromContext(ctx)
	added := make([]string, 0, len(policies))
	logger.Info("Pushing policies", "count", len(policies))
	for name, policy := range policies {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, opaUrl+"/v1/policies/"+name, nil)
		if err != nil {
			return added, err
		}
		req.Header.Set("Content-Type", "text/plain")
		req.Body = io.NopCloser(strings.NewReader(policy))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return added, err
		}
		defer resp.Body.Close()
		logger.Info("Pushing policy", "name", name, "status", resp.Status)
		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return added, fmt.Errorf("failed to push policy %s: %s", name, err)
			}
			return added, fmt.Errorf("failed to push policy %s: %s\n%s", name, resp.Status, string(body))
		}
		added = append(added, name)
	}
	return added, nil
}

func DeletePolicies(ctx context.Context, opaUrl string, policies []string) ([]string, error) {
	removed := make([]string, 0, len(policies))
	for _, name := range policies {
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, opaUrl+"/v1/policies/"+name, nil)
		if err != nil {
			return removed, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return removed, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return removed, fmt.Errorf("failed to delete policy %s: %s", name, err)
			}
			return removed, fmt.Errorf("failed to delete policy %s: %s\n%s", name, resp.Status, string(body))
		}
		removed = append(removed, name)
	}
	return removed, nil
}
