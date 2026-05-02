package kvs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type mockS3Client struct {
	objects map[string][]byte
	getErr  error
}

func newMockS3Client() *mockS3Client {
	return &mockS3Client{objects: map[string][]byte{}}
}

func (m *mockS3Client) GetObject(ctx context.Context, in *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	data, ok := m.objects[*in.Key]
	if !ok {
		return nil, &s3types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(data))}, nil
}

func (m *mockS3Client) PutObject(ctx context.Context, in *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	data, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	m.objects[*in.Key] = data
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	delete(m.objects, *in.Key)
	return &s3.DeleteObjectOutput{}, nil
}

func (m *mockS3Client) ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, opts ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	prefix := ""
	if in.Prefix != nil {
		prefix = *in.Prefix
	}
	var contents []s3types.Object
	for k := range m.objects {
		if strings.HasPrefix(k, prefix) {
			key := k
			contents = append(contents, s3types.Object{Key: aws.String(key)})
		}
	}
	return &s3.ListObjectsV2Output{Contents: contents}, nil
}

func newTestS3(t *testing.T) (*S3, *mockS3Client) {
	t.Helper()
	dir := t.TempDir()
	mock := newMockS3Client()
	s := &S3{
		Bucket:  "testbucket",
		Prefix:  "myteam/myapp/",
		Region:  "ap-northeast-1",
		cl:      mock,
		ctx:     context.Background(),
		dir:     dir,
		MaxSize: DefaultMaxSize,
	}
	return s, mock
}

func TestS3WriteAndRead(t *testing.T) {
	s, mock := newTestS3(t)
	data := []byte("hello s3")

	if err := s.Write("artifact.tar.gz", data); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if got, ok := mock.objects["myteam/myapp/artifact.tar.gz"]; !ok {
		t.Fatal("expected object stored under prefixed key")
	} else if !bytes.Equal(got, data) {
		t.Errorf("stored bytes mismatch: got %q want %q", got, data)
	}

	localPath := filepath.Join(s.dir, "artifact.tar.gz")
	if got, err := os.ReadFile(localPath); err != nil {
		t.Fatalf("expected local stage file: %v", err)
	} else if !bytes.Equal(got, data) {
		t.Errorf("staged bytes mismatch: got %q want %q", got, data)
	}

	got, err := s.Read("artifact.tar.gz")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("Read bytes mismatch: got %q want %q", got, data)
	}
}

func TestS3ReadFromCloudStagesLocally(t *testing.T) {
	s, mock := newTestS3(t)
	mock.objects["myteam/myapp/x"] = []byte("from-cloud")

	got, err := s.Read("x")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != "from-cloud" {
		t.Errorf("got %q", got)
	}

	// Subsequent reads should hit local even when cloud client errors.
	mock.getErr = errors.New("cloud unavailable")
	got2, err := s.Read("x")
	if err != nil {
		t.Fatalf("Read after stage: %v", err)
	}
	if string(got2) != "from-cloud" {
		t.Errorf("staged read mismatch: %q", got2)
	}
}

func TestS3ReadNotFound(t *testing.T) {
	s, _ := newTestS3(t)
	_, err := s.Read("missing")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

func TestS3List(t *testing.T) {
	s, mock := newTestS3(t)
	mock.objects["myteam/myapp/a"] = []byte("1")
	mock.objects["myteam/myapp/b"] = []byte("2")
	mock.objects["other/c"] = []byte("3") // outside prefix, must be ignored

	keys, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Errorf("unexpected keys: %v", keys)
	}
}

func TestS3Delete(t *testing.T) {
	s, mock := newTestS3(t)
	if err := s.Write("k", []byte("v")); err != nil {
		t.Fatal(err)
	}

	if err := s.Delete("k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := mock.objects["myteam/myapp/k"]; ok {
		t.Error("expected object removed from cloud")
	}
	if IsFileExist(filepath.Join(s.dir, "k")) {
		t.Error("expected local stage removed")
	}
}

func TestS3PathTraversal(t *testing.T) {
	s, _ := newTestS3(t)
	if err := s.Write("../evil", []byte("x")); err == nil {
		t.Error("expected path traversal error on Write")
	}
	if _, err := s.Read("../evil"); err == nil {
		t.Error("expected path traversal error on Read")
	}
}

func TestNewS3URLParse(t *testing.T) {
	tests := []struct {
		desc      string
		url       string
		region    string
		bucket    string
		prefix    string
		expectErr bool
	}{
		{"basic", "s3://ap-northeast-1/mybucket", "ap-northeast-1", "mybucket", "", false},
		{"with prefix", "s3://ap-northeast-1/mybucket/team/app", "ap-northeast-1", "mybucket", "team/app/", false},
		{"missing region", "s3:///mybucket", "", "", "", true},
		{"missing bucket", "s3://ap-northeast-1/", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			s, err := NewS3(context.Background(), tt.url, nil)
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.Region != tt.region {
				t.Errorf("region: got %q want %q", s.Region, tt.region)
			}
			if s.Bucket != tt.bucket {
				t.Errorf("bucket: got %q want %q", s.Bucket, tt.bucket)
			}
			if s.Prefix != tt.prefix {
				t.Errorf("prefix: got %q want %q", s.Prefix, tt.prefix)
			}
		})
	}
}

// Compile-time check that *S3 satisfies KVS.
var _ KVS = (*S3)(nil)
