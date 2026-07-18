// Package gcs 以 Google Cloud Storage 實作 domain/meeting.AudioStorage。
// 簽名走 IAM signBlob（impersonation）：本地用開發者帳號代簽、Cloud Run 用 runtime SA，
// 兩邊零金鑰檔、同一條程式路徑。
package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"cloud.google.com/go/storage"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

const signedURLTTL = 15 * time.Minute

type Storage struct {
	client      *storage.Client
	iamClient   *credentials.IamCredentialsClient
	bucket      string
	signerEmail string
}

var _ domainmeeting.AudioStorage = (*Storage)(nil)

func New(ctx context.Context, bucket, signerEmail string) (*Storage, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs.New storage client: %w", err)
	}
	iamClient, err := credentials.NewIamCredentialsClient(ctx)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("gcs.New iam client: %w", err)
	}
	return &Storage{client: client, iamClient: iamClient, bucket: bucket, signerEmail: signerEmail}, nil
}

func (s *Storage) Close() error {
	s.iamClient.Close()
	return s.client.Close()
}

func (s *Storage) SignedUploadURL(ctx context.Context, objectPath, contentType string, maxBytes int64) (domainmeeting.UploadTarget, error) {
	lengthRange := fmt.Sprintf("x-goog-content-length-range:0,%d", maxBytes)
	url, err := s.client.Bucket(s.bucket).SignedURL(objectPath, &storage.SignedURLOptions{
		Scheme:         storage.SigningSchemeV4,
		Method:         http.MethodPut,
		Expires:        time.Now().Add(signedURLTTL),
		ContentType:    contentType,
		Headers:        []string{lengthRange},
		GoogleAccessID: s.signerEmail,
		SignBytes: func(b []byte) ([]byte, error) {
			resp, err := s.iamClient.SignBlob(ctx, &credentialspb.SignBlobRequest{
				Name:    "projects/-/serviceAccounts/" + s.signerEmail,
				Payload: b,
			})
			if err != nil {
				return nil, fmt.Errorf("gcs signBlob: %w", err)
			}
			return resp.SignedBlob, nil
		},
	})
	if err != nil {
		return domainmeeting.UploadTarget{}, fmt.Errorf("gcs.SignedUploadURL: %w", err)
	}
	return domainmeeting.UploadTarget{
		URL: url,
		Headers: map[string]string{
			"Content-Type":                contentType,
			"x-goog-content-length-range": fmt.Sprintf("0,%d", maxBytes),
		},
	}, nil
}

func (s *Storage) Download(ctx context.Context, objectPath string) (io.ReadCloser, int64, error) {
	r, err := s.client.Bucket(s.bucket).Object(objectPath).NewReader(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("gcs.Download: %w", err)
	}
	return r, r.Attrs.Size, nil
}

func (s *Storage) Exists(ctx context.Context, objectPath string) (bool, error) {
	_, err := s.client.Bucket(s.bucket).Object(objectPath).Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("gcs.Exists: %w", err)
	}
	return true, nil
}
