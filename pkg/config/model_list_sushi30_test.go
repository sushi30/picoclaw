// model_list_sushi30_test.go — sushi30 fork regression tests for ModelConfig
// JSON unmarshalling of the singular "api_key" field.
//
// Background: ModelConfig only declares APIKeys (plural, json:"api_keys").
// The singular "api_key" field used in v2 configs was silently dropped by the
// standard JSON unmarshaller, leaving APIKeys empty and causing:
//   failed to create provider: api_key or api_base is required for HTTP-based protocol "openrouter"
//
// Fix: ModelConfig.UnmarshalJSON in pkg/config/config.go now accepts both
// "api_key" (singular) and "api_keys" (plural) and merges them into APIKeys.
// These tests guard against that regression being re-introduced by a future
// upstream merge or rebase.

package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModelConfig_SingularAPIKey_JSON verifies that a literal api_key value in
// a model_list entry is unmarshalled into APIKey().
func TestModelConfig_SingularAPIKey_JSON(t *testing.T) {
	data := `{
		"model_name": "test-model",
		"model":      "openrouter/test/model",
		"api_key":    "sk-literal-key"
	}`

	var cfg ModelConfig
	require.NoError(t, json.Unmarshal([]byte(data), &cfg))
	assert.Equal(t, "sk-literal-key", cfg.APIKey(),
		"singular api_key must be available via APIKey()")
}

// TestModelConfig_SingularAPIKey_EnvRef verifies that an env:// reference in
// the singular api_key field is resolved to the environment variable value.
func TestModelConfig_SingularAPIKey_EnvRef(t *testing.T) {
	t.Setenv("PICOCLAW_TEST_MODEL_KEY", "sk-from-env")

	data := `{
		"model_name": "test-model",
		"model":      "openrouter/test/model",
		"api_key":    "env://PICOCLAW_TEST_MODEL_KEY"
	}`

	var cfg ModelConfig
	require.NoError(t, json.Unmarshal([]byte(data), &cfg))
	assert.Equal(t, "sk-from-env", cfg.APIKey(),
		"env:// api_key must resolve to the environment variable value")
}

// TestModelConfig_PluralAPIKeys_StillWork verifies that the existing api_keys
// (plural) field continues to work after the UnmarshalJSON change.
func TestModelConfig_PluralAPIKeys_StillWork(t *testing.T) {
	data := `{
		"model_name": "test-model",
		"model":      "openrouter/test/model",
		"api_keys":   ["sk-a", "sk-b"]
	}`

	var cfg ModelConfig
	require.NoError(t, json.Unmarshal([]byte(data), &cfg))
	assert.Equal(t, "sk-a", cfg.APIKey(),
		"first entry of api_keys must be returned by APIKey()")
	assert.Len(t, cfg.APIKeys, 2)
}

// TestModelConfig_BothAPIKeyFields_SingularWins verifies that when both
// api_key and api_keys are present, the singular api_key ends up at index 0.
func TestModelConfig_BothAPIKeyFields_SingularWins(t *testing.T) {
	data := `{
		"model_name": "test-model",
		"model":      "openrouter/test/model",
		"api_key":    "sk-primary",
		"api_keys":   ["sk-fallback"]
	}`

	var cfg ModelConfig
	require.NoError(t, json.Unmarshal([]byte(data), &cfg))
	assert.Equal(t, "sk-primary", cfg.APIKey(),
		"singular api_key must take slot 0 when both fields are present")
	assert.Len(t, cfg.APIKeys, 2,
		"api_keys entries must be preserved after singular api_key is prepended")
}

// TestModelList_EnvRef_ResolvedByGateway mimics how a real v2 config.json
// model_list array is loaded and checks that APIKey() returns the resolved
// env var value, not an empty string.
func TestModelList_EnvRef_ResolvedByGateway(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-openrouter-test")

	data := `[
		{
			"model_name": "glm-4.5",
			"model":      "openrouter/z-ai/glm-4.5",
			"api_key":    "env://OPENROUTER_API_KEY"
		},
		{
			"model_name": "gpt-4o",
			"model":      "openrouter/openai/gpt-4o",
			"api_key":    "env://OPENROUTER_API_KEY"
		}
	]`

	var list SecureModelList
	require.NoError(t, json.Unmarshal([]byte(data), &list))
	require.Len(t, list, 2)

	for _, m := range list {
		assert.Equal(t, "sk-openrouter-test", m.APIKey(),
			"model %q: api_key env:// must resolve, not return empty string", m.ModelName)
	}
}
