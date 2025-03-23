package manager

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
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

func PushPolicies(ctx context.Context, opaUrl string, policies map[string]string) error {
	for name, policy := range policies {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, opaUrl+"/v1/policies/"+name, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "text/plain")
		req.Body = io.NopCloser(strings.NewReader(policy))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to push policy %s: %s", name, err)
			}
			return fmt.Errorf("failed to push policy %s: %s\n%s", name, resp.Status, string(body))
		}
	}
	return nil
}

func DeletePolicies(ctx context.Context, opaUrl string, policies []string) error {
	for _, name := range policies {
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, opaUrl+"/v1/policies/"+name, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to delete policy %s: %s", name, err)
			}
			return fmt.Errorf("failed to delete policy %s: %s\n%s", name, resp.Status, string(body))
		}
	}
	return nil
}
