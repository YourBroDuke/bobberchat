package protocol

import "strings"

const (
	TagRequest        = "request"
	TagResponse       = "response"
	TagContextProvide = "context-provide"
	TagNoResponse     = "no-response"
	TagProgress       = "progress"
	TagError          = "error"
	TagApproval       = "approval"
	TagSystem         = "system"
)

const (
	TagRequestData     = "request.data"
	TagRequestAction   = "request.action"
	TagRequestApproval = "request.approval"

	TagResponseSuccess = "response.success"
	TagResponseError   = "response.error"
	TagResponsePartial = "response.partial"

	TagContextProvideDefault = "context-provide"
	TagNoResponseDefault     = "no-response"

	TagProgressUpdate = "progress.update"
	TagProgressStage  = "progress.stage"

	TagErrorRecoverable = "error.recoverable"
	TagErrorFatal       = "error.fatal"

	TagApprovalRequest = "approval.request"
	TagApprovalGranted = "approval.granted"
	TagApprovalDenied  = "approval.denied"

	TagSystemHeartbeat = "system.heartbeat"
	TagSystemJoin      = "system.join"
	TagSystemLeave     = "system.leave"
)

var knownTags = map[string]struct{}{
	TagRequestData:          {},
	TagRequestAction:        {},
	TagRequestApproval:      {},
	TagResponseSuccess:      {},
	TagResponseError:        {},
	TagResponsePartial:      {},
	TagContextProvideDefault: {},
	TagNoResponseDefault:     {},
	TagProgressUpdate:       {},
	TagProgressStage:        {},
	TagErrorRecoverable:     {},
	TagErrorFatal:           {},
	TagApprovalRequest:      {},
	TagApprovalGranted:      {},
	TagApprovalDenied:       {},
	TagSystemHeartbeat:      {},
	TagSystemJoin:           {},
	TagSystemLeave:          {},
}

var knownFamilies = map[string]struct{}{
	TagRequest:        {},
	TagResponse:       {},
	TagContextProvide: {},
	TagNoResponse:     {},
	TagProgress:       {},
	TagError:          {},
	TagApproval:       {},
	TagSystem:         {},
}

func ParseTagFamily(tag string) string {
	if tag == "" {
		return ""
	}

	if tag == TagContextProvide || tag == TagNoResponse {
		return tag
	}

	family, _, found := strings.Cut(tag, ".")
	if !found {
		return tag
	}

	return family
}

func IsValidTag(tag string) bool {
	if tag == "" {
		return false
	}

	if _, ok := knownTags[tag]; ok {
		return true
	}

	if !isValidTagFormat(tag) {
		return false
	}

	family := ParseTagFamily(tag)
	if _, ok := knownFamilies[family]; ok {
		return true
	}

	return true
}

func RequiresResponse(tag string) bool {
	return ParseTagFamily(tag) == TagRequest
}

func IsTerminal(tag string) bool {
	return tag == TagNoResponse || tag == TagResponseSuccess || tag == TagResponseError
}

func isValidTagFormat(tag string) bool {
	if strings.TrimSpace(tag) != tag {
		return false
	}

	parts := strings.Split(tag, ".")
	for _, part := range parts {
		if !isValidTagToken(part) {
			return false
		}
	}

	return true
}

func isValidTagToken(token string) bool {
	if token == "" {
		return false
	}

	for _, r := range token {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}

		return false
	}

	return true
}
