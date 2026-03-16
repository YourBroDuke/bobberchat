package azurecs

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bobberchat/bobberchat/backend/internal/email"
)

const apiVersion = "2023-03-31"

type sendRequest struct {
	SenderAddress string     `json:"senderAddress"`
	Recipients    recipients `json:"recipients"`
	Content       content    `json:"content"`
}

type recipients struct {
	To []address `json:"to"`
}

type address struct {
	Address     string `json:"address"`
	DisplayName string `json:"displayName,omitempty"`
}

type content struct {
	Subject   string `json:"subject"`
	HTML      string `json:"html,omitempty"`
	PlainText string `json:"plainText,omitempty"`
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type Sender struct {
	endpoint  string
	accessKey string
	from      string
	client    *http.Client
}

// New creates an ACS email sender. The connectionString format is:
// "endpoint=https://<resource>.communication.azure.com/;accesskey=<base64key>"
func New(connectionString, fromAddress string) *Sender {
	endpoint, accessKey := parseConnectionString(connectionString)
	return &Sender{
		endpoint:  endpoint,
		accessKey: accessKey,
		from:      fromAddress,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *Sender) SendEmail(ctx context.Context, msg email.Message) error {
	if s == nil || s.endpoint == "" || s.accessKey == "" {
		return fmt.Errorf("azurecs: sender not configured (missing endpoint or access key)")
	}

	body := sendRequest{
		SenderAddress: s.from,
		Recipients: recipients{
			To: []address{{Address: msg.To}},
		},
		Content: content{
			Subject:   msg.Subject,
			HTML:      msg.HTML,
			PlainText: msg.Text,
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("azurecs: marshal request: %w", err)
	}

	endpoint := strings.TrimSuffix(s.endpoint, "/")
	reqURL, err := url.Parse(fmt.Sprintf("%s/emails:send?api-version=%s", endpoint, apiVersion))
	if err != nil {
		return fmt.Errorf("azurecs: parse url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("azurecs: create request: %w", err)
	}

	if err := signRequest(s.accessKey, req, payload); err != nil {
		return fmt.Errorf("azurecs: sign request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("azurecs: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted {
		return nil
	}

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var errResp errorResponse
	if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Message != "" {
		return fmt.Errorf("azurecs: status %d: %s (code=%s)", resp.StatusCode, errResp.Error.Message, errResp.Error.Code)
	}

	return fmt.Errorf("azurecs: unexpected status %d: %s", resp.StatusCode, string(respBody))
}

// signRequest adds HMAC-SHA256 authentication headers required by ACS.
func signRequest(accessKey string, req *http.Request, body []byte) error {
	key, err := base64.StdEncoding.DecodeString(accessKey)
	if err != nil {
		return fmt.Errorf("decode access key: %w", err)
	}

	timestamp := time.Now().UTC().Format(http.TimeFormat)
	contentHash := computeContentHash(body)

	pathAndQuery := req.URL.Path
	if req.URL.RawQuery != "" {
		pathAndQuery += "?" + req.URL.RawQuery
	}

	stringToSign := fmt.Sprintf("POST\n%s\n%s;%s;%s", pathAndQuery, timestamp, req.URL.Host, contentHash)
	signature := computeHMAC(stringToSign, key)

	req.Header.Set("x-ms-date", timestamp)
	req.Header.Set("x-ms-content-sha256", contentHash)
	req.Header.Set("Authorization", "HMAC-SHA256 SignedHeaders=x-ms-date;host;x-ms-content-sha256&Signature="+signature)

	return nil
}

func computeContentHash(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func computeHMAC(data string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// parseConnectionString extracts endpoint and accesskey from an ACS connection string.
// Format: "endpoint=https://...;accesskey=..."
func parseConnectionString(cs string) (endpoint, accessKey string) {
	for _, part := range strings.Split(cs, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "endpoint=") {
			endpoint = strings.TrimSpace(part[len("endpoint="):])
		} else if strings.HasPrefix(strings.ToLower(part), "accesskey=") {
			accessKey = strings.TrimSpace(part[len("accesskey="):])
		}
	}
	return endpoint, accessKey
}
