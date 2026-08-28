package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var smallPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
	0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
	0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
	0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
	0x44, 0xae, 0x42, 0x60, 0x82,
}

var smallJPEG = []byte{
	0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46,
	0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
	0x00, 0x01, 0x00, 0x00, 0xff, 0xd9,
}

var smallWebP = []byte{
	0x52, 0x49, 0x46, 0x46, 0x24, 0x00, 0x00, 0x00,
	0x57, 0x45, 0x42, 0x50, 0x56, 0x50, 0x38, 0x20,
	0x18, 0x00, 0x00, 0x00, 0x30, 0x00, 0x00, 0x00,
	0x01, 0x00, 0x01, 0x00, 0x30, 0x00, 0x00, 0x00,
	0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x01, 0x00, 0x01, 0x00, 0x56, 0x50, 0x38, 0x20,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x80,
	0x00, 0x00, 0x01, 0x00, 0x01, 0x80, 0x00, 0x00,
}

type avatarUploadResult struct {
	Data struct {
		AvatarURL *string `json:"avatar_url"`
	} `json:"data"`
	Success bool `json:"success"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func TestAvatar_SelfUploadAndDelete(t *testing.T) {
	_, email := createUser(t, "viewer", "av-self")
	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("login failed")
	}

	uploadResp := doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "avatar.png", smallPNG)
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusOK {
		t.Fatalf("upload status: got %d, want 200", uploadResp.StatusCode)
	}

	var upload avatarUploadResult
	decodeJSON(uploadResp, &upload)

	if !upload.Success {
		t.Fatalf("upload response: success=false, error=%v", upload.Error)
	}
	if upload.Data.AvatarURL == nil {
		t.Fatal("expected avatar_url to be set after upload")
	}
	if !strings.Contains(*upload.Data.AvatarURL, "avatars/") {
		t.Errorf("avatar_url does not contain expected path: %s", *upload.Data.AvatarURL)
	}

	getResp := doRequest(http.MethodGet, "/api/panel/v1/me", token, nil)
	defer getResp.Body.Close()

	var me struct {
		Data struct {
			AvatarURL *string `json:"avatar_url"`
		} `json:"data"`
	}
	decodeJSON(getResp, &me)

	if me.Data.AvatarURL == nil {
		t.Fatal("avatar_url not visible via GET /me")
	}
	if *me.Data.AvatarURL != *upload.Data.AvatarURL {
		t.Errorf("avatar_url mismatch: upload=%s, me=%s", *upload.Data.AvatarURL, *me.Data.AvatarURL)
	}

	delResp := doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "", nil)
	defer delResp.Body.Close()

	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("delete status: got %d, want 200", delResp.StatusCode)
	}

	var del avatarUploadResult
	decodeJSON(delResp, &del)

	if del.Data.AvatarURL != nil {
		t.Errorf("expected avatar_url null after delete, got %s", *del.Data.AvatarURL)
	}
}

func TestAvatar_AdminUploadAndDeleteForUser(t *testing.T) {
	id, _ := createUser(t, "viewer", "av-admin-target")

	uploadResp := doMultipartRequest(fmt.Sprintf("/api/panel/v1/users/%d/avatar", id), adminToken, "avatar", "avatar.png", smallPNG)
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusOK {
		t.Fatalf("upload status: got %d, want 200", uploadResp.StatusCode)
	}

	var upload avatarUploadResult
	decodeJSON(uploadResp, &upload)

	if upload.Data.AvatarURL == nil {
		t.Fatal("expected avatar_url to be set after admin upload")
	}

	getResp := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/users/%d", id), adminToken, nil)
	defer getResp.Body.Close()

	var user struct {
		Data struct {
			AvatarURL *string `json:"avatar_url"`
		} `json:"data"`
	}
	decodeJSON(getResp, &user)

	if user.Data.AvatarURL == nil {
		t.Fatal("avatar_url not visible via GET /users/:id")
	}
	if *user.Data.AvatarURL != *upload.Data.AvatarURL {
		t.Errorf("avatar_url mismatch: upload=%s, get=%s", *upload.Data.AvatarURL, *user.Data.AvatarURL)
	}

	delResp := doMultipartRequest(fmt.Sprintf("/api/panel/v1/users/%d/avatar", id), adminToken, "avatar", "", nil)
	defer delResp.Body.Close()

	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("delete status: got %d, want 200", delResp.StatusCode)
	}

	var del avatarUploadResult
	decodeJSON(delResp, &del)

	if del.Data.AvatarURL != nil {
		t.Errorf("expected avatar_url null after delete, got %s", *del.Data.AvatarURL)
	}
}

func TestAvatar_UploadReplacesExisting(t *testing.T) {
	_, email := createUser(t, "viewer", "av-replace")
	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("login failed")
	}

	firstResp := doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "first.png", smallPNG)
	defer firstResp.Body.Close()

	var first avatarUploadResult
	decodeJSON(firstResp, &first)

	if first.Data.AvatarURL == nil {
		t.Fatal("first upload did not set avatar_url")
	}
	firstURL := *first.Data.AvatarURL

	key1 := strings.TrimPrefix(firstURL, e2eS3Endpoint+"/"+e2eS3Bucket+"/")

	secondResp := doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "second.png", smallPNG)
	defer secondResp.Body.Close()

	var second avatarUploadResult
	decodeJSON(secondResp, &second)

	if second.Data.AvatarURL == nil {
		t.Fatal("second upload did not set avatar_url")
	}

	key2 := strings.TrimPrefix(*second.Data.AvatarURL, e2eS3Endpoint+"/"+e2eS3Bucket+"/")
	if key1 != key2 {
		t.Errorf("expected same deterministic key for same user, got %q and %q", key1, key2)
	}

	if !s3ObjectExists(t, key2) {
		t.Errorf("S3 object %q does not exist after replacement", key2)
	}
}

func TestAvatar_UnauthenticatedRejected(t *testing.T) {
	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/panel/v1/me/avatar"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			resp := doMultipartRequest(ep.path, "", "avatar", "test.png", smallPNG)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("status: got %d, want 401", resp.StatusCode)
			}
		})
	}
}

func TestAvatar_ViewerCannotUploadForOther(t *testing.T) {
	_, viewerEmail := createUser(t, "viewer", "av-viewer-auth")
	viewerToken := login(viewerEmail, "testpass123")
	if viewerToken == "" {
		t.Fatal("login failed")
	}

	targetID, _ := createUser(t, "viewer", "av-viewer-target")

	resp := doMultipartRequest(fmt.Sprintf("/api/panel/v1/users/%d/avatar", targetID), viewerToken, "avatar", "avatar.png", smallPNG)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status: got %d, want 403", resp.StatusCode)
	}

	var errResp struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(resp, &errResp)

	if errResp.Success {
		t.Error("expected success=false")
	}
	if errResp.Error.Code != "FORBIDDEN" {
		t.Errorf("error code: got %q, want %q", errResp.Error.Code, "FORBIDDEN")
	}
}

func TestAvatar_EmptyFileTriggersDelete(t *testing.T) {
	_, email := createUser(t, "viewer", "av-empty-file")
	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("login failed")
	}

	uploadResp := doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "avatar.png", smallPNG)
	defer uploadResp.Body.Close()
	if uploadResp.StatusCode != http.StatusOK {
		t.Fatalf("initial upload: got %d", uploadResp.StatusCode)
	}

	var before avatarUploadResult
	decodeJSON(uploadResp, &before)
	if before.Data.AvatarURL == nil {
		t.Fatal("expected avatar_url after initial upload")
	}

	deleteResp := doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "avatar.png", nil)
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("delete-via-empty status: got %d, want 200", deleteResp.StatusCode)
	}

	var after avatarUploadResult
	decodeJSON(deleteResp, &after)

	if after.Data.AvatarURL != nil {
		t.Errorf("expected avatar_url null after empty-file delete, got %s", *after.Data.AvatarURL)
	}
}

func TestAvatar_MissingFormFieldTriggersDelete(t *testing.T) {
	_, email := createUser(t, "viewer", "av-missing-field")
	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("login failed")
	}

	uploadResp := doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "avatar.png", smallPNG)
	defer uploadResp.Body.Close()
	if uploadResp.StatusCode != http.StatusOK {
		t.Fatalf("initial upload: got %d", uploadResp.StatusCode)
	}

	deleteResp := doMultipartRequestNoFile("/api/panel/v1/me/avatar", token)
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("delete-via-missing-field status: got %d, want 200", deleteResp.StatusCode)
	}

	var after avatarUploadResult
	decodeJSON(deleteResp, &after)

	if after.Data.AvatarURL != nil {
		t.Errorf("expected avatar_url null after missing-field delete, got %s", *after.Data.AvatarURL)
	}
}

func TestAvatar_DeleteNoOpWhenNoAvatar(t *testing.T) {
	_, email := createUser(t, "viewer", "av-delete-noop")
	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("login failed")
	}

	resp := doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	var result avatarUploadResult
	decodeJSON(resp, &result)

	if result.Data.AvatarURL != nil {
		t.Errorf("expected avatar_url null for no-op delete, got %s", *result.Data.AvatarURL)
	}
}

func TestAvatar_InvalidFileType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		data     []byte
	}{
		{"exe", "malware.exe", []byte("not-a-real-exe")},
		{"gif", "image.gif", []byte("GIF89a")},
		{"bmp", "image.bmp", []byte("BM")},
		{"no-extension", "avatar", smallPNG},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := doMultipartRequest("/api/panel/v1/me/avatar", adminToken, "avatar", tc.filename, tc.data)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status: got %d, want 400", resp.StatusCode)
			}

			var errResp struct {
				Success bool `json:"success"`
				Error   struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			decodeJSON(resp, &errResp)

			if errResp.Success {
				t.Error("expected success=false")
			}
			if errResp.Error.Code != "BAD_REQUEST" {
				t.Errorf("error code: got %q, want %q", errResp.Error.Code, "BAD_REQUEST")
			}
		})
	}
}

func TestAvatar_OversizedFileRejected(t *testing.T) {
	big := make([]byte, 6<<20)
	for i := range big {
		big[i] = 0xFF
	}

	resp := doMultipartRequest("/api/panel/v1/me/avatar", adminToken, "avatar", "big.png", big)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}

	var errResp struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeJSON(resp, &errResp)

	if errResp.Error.Code != "BAD_REQUEST" {
		t.Errorf("error code: got %q, want %q", errResp.Error.Code, "BAD_REQUEST")
	}
	if !strings.Contains(errResp.Error.Message, "5 MB") {
		t.Errorf("error message should mention size limit: %s", errResp.Error.Message)
	}
}

func TestAvatar_ValidTypesAccepted(t *testing.T) {
	types := []struct {
		name     string
		filename string
		data     []byte
	}{
		{"jpeg", "photo.jpeg", smallJPEG},
		{"jpg", "photo.jpg", smallJPEG},
		{"webp", "image.webp", smallWebP},
	}

	for _, tc := range types {
		t.Run(tc.name, func(t *testing.T) {
			_, email := createUser(t, "viewer", "av-type-"+tc.name)
			token := login(email, "testpass123")
			if token == "" {
				t.Fatal("login failed")
			}

			resp := doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", tc.filename, tc.data)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status: got %d, want 200", resp.StatusCode)
			}

			var result avatarUploadResult
			decodeJSON(resp, &result)

			if result.Data.AvatarURL == nil {
				t.Error("expected avatar_url to be set")
			}

			doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "", nil)
		})
	}
}

func TestAvatar_UploadNoSecretsInResponse(t *testing.T) {
	_, email := createUser(t, "viewer", "av-secrets")
	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("login failed")
	}

	resp := doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "avatar.png", smallPNG)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	leaked := []string{
		"minioadmin",
		"secret_key",
		"access_key",
		"password_hash",
	}

	for _, pattern := range leaked {
		if strings.Contains(s, pattern) {
			t.Errorf("response leaks sensitive data: contains %q", pattern)
		}
	}
}

func TestAvatar_ResponseContentTypeJSON(t *testing.T) {
	_, email := createUser(t, "viewer", "av-ct")
	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("login failed")
	}

	resp := doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "avatar.png", smallPNG)
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
}

func TestAvatar_UploadSetsS3ObjectKey(t *testing.T) {
	_, email := createUser(t, "viewer", "av-s3key")
	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("login failed")
	}

	resp := doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "avatar.png", smallPNG)
	defer resp.Body.Close()

	var result avatarUploadResult
	decodeJSON(resp, &result)

	if result.Data.AvatarURL == nil {
		t.Fatal("expected avatar_url after upload")
	}

	avatarURL := *result.Data.AvatarURL

	expectedPrefix := e2eS3Endpoint + "/" + e2eS3Bucket + "/avatars/"
	if !strings.HasPrefix(avatarURL, expectedPrefix) {
		t.Errorf("avatar_url prefix: got %q, want prefix %q", avatarURL, expectedPrefix)
	}

	// verify object exists in S3
	key := strings.TrimPrefix(avatarURL, e2eS3Endpoint+"/"+e2eS3Bucket+"/")
	if !s3ObjectExists(t, key) {
		t.Errorf("S3 object %q does not exist", key)
	}

	doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "", nil)
}

func TestAvatar_DifferentUsersHaveDifferentKeys(t *testing.T) {
	_, email1 := createUser(t, "viewer", "av-key-a")
	token1 := login(email1, "testpass123")
	if token1 == "" {
		t.Fatal("login failed")
	}

	_, email2 := createUser(t, "viewer", "av-key-b")
	token2 := login(email2, "testpass123")
	if token2 == "" {
		t.Fatal("login failed")
	}

	resp1 := doMultipartRequest("/api/panel/v1/me/avatar", token1, "avatar", "avatar.png", smallPNG)
	defer resp1.Body.Close()
	var r1 avatarUploadResult
	decodeJSON(resp1, &r1)

	resp2 := doMultipartRequest("/api/panel/v1/me/avatar", token2, "avatar", "avatar.png", smallPNG)
	defer resp2.Body.Close()
	var r2 avatarUploadResult
	decodeJSON(resp2, &r2)

	if r1.Data.AvatarURL == nil || r2.Data.AvatarURL == nil {
		t.Fatal("expected both avatar_urls to be set")
	}
	if *r1.Data.AvatarURL == *r2.Data.AvatarURL {
		t.Errorf("different users got same avatar_url: %s", *r1.Data.AvatarURL)
	}

	key1 := strings.TrimPrefix(*r1.Data.AvatarURL, e2eS3Endpoint+"/"+e2eS3Bucket+"/")
	key2 := strings.TrimPrefix(*r2.Data.AvatarURL, e2eS3Endpoint+"/"+e2eS3Bucket+"/")
	if key1 == key2 {
		t.Errorf("different users got same S3 key: %s", key1)
	}

	doMultipartRequest("/api/panel/v1/me/avatar", token1, "avatar", "", nil)
	doMultipartRequest("/api/panel/v1/me/avatar", token2, "avatar", "", nil)
}

func TestAvatar_AdminUploadNonexistentUser(t *testing.T) {
	resp := doMultipartRequest("/api/panel/v1/users/999999999/avatar", adminToken, "avatar", "avatar.png", smallPNG)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestAvatar_InvalidUserID(t *testing.T) {
	resp := doMultipartRequest("/api/panel/v1/users/abc/avatar", adminToken, "avatar", "avatar.png", smallPNG)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestAvatar_NegativeUserID(t *testing.T) {
	resp := doMultipartRequest("/api/panel/v1/users/-1/avatar", adminToken, "avatar", "avatar.png", smallPNG)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestAvatar_ResponseSchema(t *testing.T) {
	_, email := createUser(t, "viewer", "av-schema")
	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("login failed")
	}

	resp := doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "avatar.png", smallPNG)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	required := []string{`"id"`, `"username"`, `"email"`, `"role"`, `"avatar_url"`, `"created_at"`, `"updated_at"`, `"success"`}
	for _, field := range required {
		if !strings.Contains(s, field) {
			t.Errorf("response missing required field %s", field)
		}
	}

	forbidden := []string{`"password"`, `"password_hash"`}
	for _, field := range forbidden {
		if strings.Contains(s, field) {
			t.Errorf("response must not contain %s", field)
		}
	}

	doMultipartRequest("/api/panel/v1/me/avatar", token, "avatar", "", nil)
}

// --- helpers ---

func doMultipartRequestNoFile(path, token string) *http.Response {
	body := new(bytes.Buffer)
	boundary := "----E2ENoFileBoundary"
	body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	body.WriteString("Content-Disposition: form-data; name=\"other_field\"\r\n\r\n")
	body.WriteString("value\r\n")
	body.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	req, err := http.NewRequest(http.MethodPost, baseURL+path, body)
	if err != nil {
		panic(fmt.Sprintf("create request: %v", err))
	}

	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(fmt.Sprintf("do request: %v", err))
	}
	return resp
}

func s3ObjectExists(t *testing.T, key string) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", "")),
	)
	if err != nil {
		t.Logf("s3 config: %v", err)
		return false
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(e2eS3Endpoint)
		o.UsePathStyle = true
	})

	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(e2eS3Bucket),
		Key:    aws.String(key),
	})

	return err == nil
}
