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

func TestAvatar_SelfUploadAndDelete(t *testing.T) {
	_, _, token := createAndLoginUser(t, "viewer", "av-self")

	uploadResp := uploadAvatar(t, token, "avatar.png", smallPNG)
	defer uploadResp.Body.Close()

	requireStatus(t, uploadResp, http.StatusOK)
	upload := decodeAvatarResponse(t, uploadResp)
	avatarURL := requireAvatarURL(t, upload)

	getResp := getMe(t, token)
	defer getResp.Body.Close()
	me := decodeMeResponse(t, getResp)

	if me.Data.AvatarURL == nil {
		t.Fatal("avatar_url not visible via GET /me")
	}
	if *me.Data.AvatarURL != *avatarURL {
		t.Errorf("avatar_url mismatch: upload=%s, me=%s", *avatarURL, *me.Data.AvatarURL)
	}

	delResp := deleteAvatar(t, token)
	defer delResp.Body.Close()

	requireStatus(t, delResp, http.StatusOK)
	requireNoAvatarURL(t, decodeAvatarResponse(t, delResp))
}

func TestAvatar_AdminUploadAndDeleteForUser(t *testing.T) {
	id, _ := createUser(t, "viewer", "av-admin-target")

	uploadResp := uploadUserAvatar(t, adminToken, id, "avatar.png", smallPNG)
	defer uploadResp.Body.Close()

	requireStatus(t, uploadResp, http.StatusOK)
	upload := decodeAvatarResponse(t, uploadResp)
	avatarURL := requireAvatarURL(t, upload)

	getResp := getUser(t, id)
	defer getResp.Body.Close()
	user := decodeUserResponse(t, getResp)

	if user.Data.AvatarURL == nil {
		t.Fatal("avatar_url not visible via GET /users/:id")
	}
	if *user.Data.AvatarURL != *avatarURL {
		t.Errorf("avatar_url mismatch: upload=%s, get=%s", *avatarURL, *user.Data.AvatarURL)
	}

	delResp := deleteUserAvatar(t, adminToken, id)
	defer delResp.Body.Close()

	requireStatus(t, delResp, http.StatusOK)
	requireNoAvatarURL(t, decodeAvatarResponse(t, delResp))
}

func TestAvatar_UploadReplacesExisting(t *testing.T) {
	_, _, token := createAndLoginUser(t, "viewer", "av-replace")

	firstResp := uploadAvatar(t, token, "first.png", smallPNG)
	defer firstResp.Body.Close()

	first := decodeAvatarResponse(t, firstResp)
	if first.Data.AvatarURL == nil {
		t.Fatal("first upload did not set avatar_url")
	}
	key1 := strings.TrimPrefix(*first.Data.AvatarURL, e2eS3Endpoint+"/"+e2eS3Bucket+"/")

	secondResp := uploadAvatar(t, token, "second.png", smallPNG)
	defer secondResp.Body.Close()

	second := decodeAvatarResponse(t, secondResp)
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
	resp := uploadAvatar(t, "", "test.png", smallPNG)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusUnauthorized)
}

func TestAvatar_ViewerCannotUploadForOther(t *testing.T) {
	_, _, viewerToken := createAndLoginUser(t, "viewer", "av-viewer-auth")

	targetID, _ := createUser(t, "viewer", "av-viewer-target")

	resp := uploadUserAvatar(t, viewerToken, targetID, "avatar.png", smallPNG)
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusForbidden, "FORBIDDEN")
}

func TestAvatar_DeleteNoOpWhenNoAvatar(t *testing.T) {
	_, _, token := createAndLoginUser(t, "viewer", "av-delete-noop")

	resp := deleteAvatar(t, token)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)
	requireNoAvatarURL(t, decodeAvatarResponse(t, resp))
}

func TestAvatar_MissingFormFieldTriggersDelete(t *testing.T) {
	_, _, token := createAndLoginUser(t, "viewer", "av-missing-field")

	uploadResp := uploadAvatar(t, token, "avatar.png", smallPNG)
	defer uploadResp.Body.Close()
	requireStatus(t, uploadResp, http.StatusOK)

	deleteResp := doMultipartRequestNoFile("/api/v1/panel/me/avatar", token)
	defer deleteResp.Body.Close()

	requireStatus(t, deleteResp, http.StatusOK)
	requireNoAvatarURL(t, decodeAvatarResponse(t, deleteResp))
}

func TestAvatar_RejectsInvalidUploads(t *testing.T) {
	big := make([]byte, 6<<20)
	for i := range big {
		big[i] = 0xFF
	}

	cases := []struct {
		name        string
		filename    string
		data        []byte
		wantMessage string
	}{
		{"exe file", "malware.exe", []byte("not-a-real-exe"), ""},
		{"gif file", "image.gif", []byte("GIF89a"), ""},
		{"bmp file", "image.bmp", []byte("BM"), ""},
		{"no extension", "avatar", smallPNG, ""},
		{"oversized file", "big.png", big, "5 MB"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := uploadAvatar(t, adminToken, tc.filename, tc.data)
			defer resp.Body.Close()

			result := requireErrorResponse(t, resp, http.StatusBadRequest, "BAD_REQUEST")
			if tc.wantMessage != "" && !strings.Contains(result.Error.Message, tc.wantMessage) {
				t.Errorf("error message should contain %q: got %q", tc.wantMessage, result.Error.Message)
			}
		})
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
			_, _, token := createAndLoginUser(t, "viewer", "av-type-"+tc.name)

			resp := uploadAvatar(t, token, tc.filename, tc.data)
			defer resp.Body.Close()

			requireStatus(t, resp, http.StatusOK)
			result := decodeAvatarResponse(t, resp)

			if result.Data.AvatarURL == nil {
				t.Error("expected avatar_url to be set")
			}

			deleteAvatar(t, token)
		})
	}
}

func TestAvatar_UploadNoSecretsInResponse(t *testing.T) {
	_, _, token := createAndLoginUser(t, "viewer", "av-secrets")

	resp := uploadAvatar(t, token, "avatar.png", smallPNG)
	defer resp.Body.Close()

	requireNoSensitiveFields(t, resp, "minioadmin", "secret_key", "access_key", "password_hash")
}

func TestAvatar_ResponseContentTypeJSON(t *testing.T) {
	_, _, token := createAndLoginUser(t, "viewer", "av-ct")

	resp := uploadAvatar(t, token, "avatar.png", smallPNG)
	defer resp.Body.Close()

	requireJSONContentType(t, resp)
}

func TestAvatar_UploadSetsS3ObjectKey(t *testing.T) {
	_, _, token := createAndLoginUser(t, "viewer", "av-s3key")

	resp := uploadAvatar(t, token, "avatar.png", smallPNG)
	defer resp.Body.Close()

	result := decodeAvatarResponse(t, resp)
	if result.Data.AvatarURL == nil {
		t.Fatal("expected avatar_url after upload")
	}

	avatarURL := *result.Data.AvatarURL
	expectedPrefix := e2eS3Endpoint + "/" + e2eS3Bucket + "/avatars/"
	if !strings.HasPrefix(avatarURL, expectedPrefix) {
		t.Errorf("avatar_url prefix: got %q, want prefix %q", avatarURL, expectedPrefix)
	}

	key := strings.TrimPrefix(avatarURL, e2eS3Endpoint+"/"+e2eS3Bucket+"/")
	if !s3ObjectExists(t, key) {
		t.Errorf("S3 object %q does not exist", key)
	}

	deleteAvatar(t, token)
}

func TestAvatar_DifferentUsersHaveDifferentKeys(t *testing.T) {
	_, _, token1 := createAndLoginUser(t, "viewer", "av-key-a")
	_, _, token2 := createAndLoginUser(t, "viewer", "av-key-b")

	resp1 := uploadAvatar(t, token1, "avatar.png", smallPNG)
	defer resp1.Body.Close()
	r1 := decodeAvatarResponse(t, resp1)

	resp2 := uploadAvatar(t, token2, "avatar.png", smallPNG)
	defer resp2.Body.Close()
	r2 := decodeAvatarResponse(t, resp2)

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

	deleteAvatar(t, token1)
	deleteAvatar(t, token2)
}

func TestAvatar_RejectsInvalidOrNonexistentUserIDs(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		wantStatus int
		wantCode   string
	}{
		{"nonexistent user", "/api/v1/panel/users/0198f4c0-dead-7000-8000-000000000001/avatar", http.StatusNotFound, "NOT_FOUND"},
		{"non-numeric id", "/api/v1/panel/users/abc/avatar", http.StatusBadRequest, "BAD_REQUEST"},
		{"negative id", "/api/v1/panel/users/-1/avatar", http.StatusBadRequest, "BAD_REQUEST"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doMultipartRequest(tc.path, adminToken, "avatar", "avatar.png", smallPNG)
			defer resp.Body.Close()
			requireErrorResponse(t, resp, tc.wantStatus, tc.wantCode)
		})
	}
}

func TestAvatar_ResponseSchema(t *testing.T) {
	_, _, token := createAndLoginUser(t, "viewer", "av-schema")

	resp := uploadAvatar(t, token, "avatar.png", smallPNG)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	required := []string{`"id"`, `"full_name"`, `"email"`, `"role"`, `"avatar_url"`, `"created_at"`, `"updated_at"`, `"success"`}
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

	deleteAvatar(t, token)
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
