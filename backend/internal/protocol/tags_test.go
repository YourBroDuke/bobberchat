package protocol

import "testing"

func TestParseTagFamily(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want string
	}{
		{name: "empty tag", tag: "", want: ""},
		{name: "special standalone context-provide", tag: TagContextProvide, want: TagContextProvide},
		{name: "special standalone no-response", tag: TagNoResponse, want: TagNoResponse},
		{name: "known namespaced tag", tag: TagRequestData, want: TagRequest},
		{name: "custom reverse dns tag", tag: "com.example.feature-action", want: "com"},
		{name: "no separator", tag: "request", want: "request"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTagFamily(tt.tag)
			if got != tt.want {
				t.Fatalf("ParseTagFamily(%q) = %q, want %q", tt.tag, got, tt.want)
			}
		})
	}
}

func TestIsValidTag(t *testing.T) {
	known := []string{
		TagRequestData,
		TagRequestAction,
		TagResponseSuccess,
		TagResponseError,
		TagResponsePartial,
		TagContextProvideDefault,
		TagNoResponseDefault,
		TagProgressUpdate,
		TagProgressStage,
		TagErrorRecoverable,
		TagErrorFatal,
		TagSystemHeartbeat,
		TagSystemJoin,
		TagSystemLeave,
	}

	for _, tag := range known {
		tag := tag
		t.Run("known/"+tag, func(t *testing.T) {
			if !IsValidTag(tag) {
				t.Fatalf("IsValidTag(%q) = false, want true", tag)
			}
		})
	}

	tests := []struct {
		name string
		tag  string
		want bool
	}{
		{name: "custom reverse dns tag", tag: "com.example.feature-action", want: true},
		{name: "custom numeric token", tag: "com.example.v2", want: true},
		{name: "family only request", tag: TagRequest, want: true},
		{name: "empty", tag: "", want: false},
		{name: "leading space", tag: " request.data", want: false},
		{name: "trailing space", tag: "request.data ", want: false},
		{name: "uppercase", tag: "Request.Data", want: false},
		{name: "empty segment", tag: "request..data", want: false},
		{name: "special characters", tag: "request.$data", want: false},
		{name: "underscore not allowed", tag: "request.my_tag", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidTag(tt.tag)
			if got != tt.want {
				t.Fatalf("IsValidTag(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

func TestRequiresResponse(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want bool
	}{
		{name: "request data requires response", tag: TagRequestData, want: true},
		{name: "request family requires response", tag: TagRequest, want: true},
		{name: "response success does not require response", tag: TagResponseSuccess, want: false},
		{name: "no response does not require response", tag: TagNoResponse, want: false},
		{name: "custom reverse dns does not require response", tag: "com.example.feature-action", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := RequiresResponse(tt.tag)
			if got != tt.want {
				t.Fatalf("RequiresResponse(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want bool
	}{
		{name: "no response is terminal", tag: TagNoResponse, want: true},
		{name: "response success is terminal", tag: TagResponseSuccess, want: true},
		{name: "response error is terminal", tag: TagResponseError, want: true},
		{name: "response partial is not terminal", tag: TagResponsePartial, want: false},
		{name: "request data is not terminal", tag: TagRequestData, want: false},
		{name: "custom reverse dns is not terminal", tag: "com.example.done", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := IsTerminal(tt.tag)
			if got != tt.want {
				t.Fatalf("IsTerminal(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

func TestIsValidTagFormat(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want bool
	}{
		{name: "single token", tag: "request", want: true},
		{name: "multi token lowercase", tag: "request.data", want: true},
		{name: "reverse dns with hyphen", tag: "com.example.feature-action", want: true},
		{name: "leading space", tag: " request.data", want: false},
		{name: "trailing space", tag: "request.data ", want: false},
		{name: "internal space", tag: "request. data", want: false},
		{name: "empty segment", tag: "request..data", want: false},
		{name: "uppercase segment", tag: "request.Data", want: false},
		{name: "special character", tag: "request.@data", want: false},
		{name: "empty string", tag: "", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := isValidTagFormat(tt.tag)
			if got != tt.want {
				t.Fatalf("isValidTagFormat(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

func TestIsValidTagToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{name: "letters", token: "request", want: true},
		{name: "alphanumeric", token: "v2", want: true},
		{name: "hyphen", token: "feature-action", want: true},
		{name: "empty", token: "", want: false},
		{name: "uppercase", token: "Request", want: false},
		{name: "space", token: "re quest", want: false},
		{name: "underscore", token: "feature_action", want: false},
		{name: "special character", token: "feature!", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := isValidTagToken(tt.token)
			if got != tt.want {
				t.Fatalf("isValidTagToken(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}
