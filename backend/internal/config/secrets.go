package config

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	smpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

const gcsmPrefix = "gcsm://"

// ResolveSecretValue resolves a single value that may contain a gcsm:// prefix.
// If the value does not start with gcsm://, it is returned as-is.
// The projectID is used for short-form expansion (e.g. gcsm://secret-name).
func ResolveSecretValue(value, projectID string) (string, error) {
	if !strings.HasPrefix(value, gcsmPrefix) {
		return value, nil
	}

	name := strings.TrimPrefix(value, gcsmPrefix)
	if !strings.Contains(name, "/") {
		name = fmt.Sprintf("projects/%s/secrets/%s/versions/latest", projectID, name)
	}

	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("create secret manager client: %w", err)
	}
	defer client.Close()

	result, err := client.AccessSecretVersion(ctx, &smpb.AccessSecretVersionRequest{
		Name: name,
	})
	if err != nil {
		return "", fmt.Errorf("access secret %q: %w", name, err)
	}

	return string(result.Payload.Data), nil
}

// resolveSecrets scans all string fields on cfg for the gcsm:// prefix
// and replaces them with the secret value fetched from GCP Secret Manager.
//
// Short form:  gcsm://secret-name → projects/{project}/secrets/secret-name/versions/latest
// Full form:   gcsm://projects/proj/secrets/name/versions/3
//
// The project ID for short-form expansion comes from cfg.FirestoreProjectID,
// which should already be populated from the env file (Layer 2).
func resolveSecrets(cfg *Config) error {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	type secretRef struct {
		fieldName  string
		secretName string
		fieldIdx   int
	}

	var refs []secretRef
	for i := 0; i < t.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() != reflect.String {
			continue
		}
		val := field.String()
		if strings.HasPrefix(val, gcsmPrefix) {
			refs = append(refs, secretRef{
				fieldName:  t.Field(i).Name,
				secretName: strings.TrimPrefix(val, gcsmPrefix),
				fieldIdx:   i,
			})
		}
	}

	if len(refs) == 0 {
		return nil
	}

	slog.Info("resolving secrets from GCP Secret Manager", "count", len(refs))

	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("create secret manager client: %w", err)
	}
	defer client.Close()

	for _, ref := range refs {
		name := ref.secretName
		if !strings.Contains(name, "/") {
			// Short form — expand using project ID
			name = fmt.Sprintf("projects/%s/secrets/%s/versions/latest",
				cfg.FirestoreProjectID, ref.secretName)
		}

		result, err := client.AccessSecretVersion(ctx, &smpb.AccessSecretVersionRequest{
			Name: name,
		})
		if err != nil {
			return fmt.Errorf("access secret %q for field %s: %w", ref.secretName, ref.fieldName, err)
		}

		v.Field(ref.fieldIdx).SetString(string(result.Payload.Data))
		slog.Info("resolved secret", "field", ref.fieldName)
	}

	return nil
}
