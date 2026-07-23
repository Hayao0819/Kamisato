package s3

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
)

// stagingPrefix keys every staged object below a segment reponame.Validate can
// never accept as a repo name ('$' is outside its grammar), so a staged key can
// never collide with a real repo's object space.
const stagingPrefix = "$staging"

func stagedKey(id, name string) (string, error) {
	if err := blob.ValidatePathComponent(id); err != nil {
		return "", err
	}
	if err := blob.ValidatePathComponent(name); err != nil {
		return "", err
	}
	return stagingPrefix + "/" + id + "/" + name, nil
}

func (s *S3) PresignStagedPut(id, name string, ttl time.Duration) (string, error) {
	result, err := s.presignStagedPut(id, name, ttl)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (s *S3) presignStagedPut(id, name string, ttl time.Duration) (*v4.PresignedHTTPRequest, error) {
	k, err := stagedKey(id, name)
	if err != nil {
		return nil, err
	}
	presignClient := awss3.NewPresignClient(s.storage)
	result, err := presignClient.PresignPutObject(s.ctx, s.putObjectInput(k, nil), func(po *awss3.PresignOptions) {
		po.Expires = ttl
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create staged presigned PUT for %s: %w", k, err)
	}
	return result, nil
}

func (s *S3) FetchStaged(id, name string) (platform.File, error) {
	k, err := stagedKey(id, name)
	if err != nil {
		return nil, err
	}
	return s.getObject(k)
}

func (s *S3) DeleteStaged(id string) error {
	if err := blob.ValidatePathComponent(id); err != nil {
		return err
	}
	prefix := stagingPrefix + "/" + id + "/"
	keys, err := s.listAllKeys(prefix)
	if err != nil {
		return fmt.Errorf("list staged intent %s: %w", id, err)
	}
	for _, key := range keys {
		if err := s.deleteObject(key); err != nil {
			return fmt.Errorf("delete staged object %s: %w", key, err)
		}
	}
	return nil
}

// ListStagedIntents lists the whole staging subtree (no delimiter) and groups
// objects by their first path segment (the intent id), since a per-id
// directory listing would need one request per id.
func (s *S3) ListStagedIntents() ([]blob.StagedIntent, error) {
	result, err := s.listObjectsV2(stagingPrefix+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("list staged intents: %w", err)
	}
	newest := make(map[string]time.Time)
	for _, obj := range result.Contents {
		rest := strings.TrimPrefix(aws.ToString(obj.Key), stagingPrefix+"/")
		id, _, ok := strings.Cut(rest, "/")
		if !ok || id == "" {
			continue
		}
		modTime := aws.ToTime(obj.LastModified)
		if current, exists := newest[id]; !exists || modTime.After(current) {
			newest[id] = modTime
		}
	}
	intents := make([]blob.StagedIntent, 0, len(newest))
	for id, modTime := range newest {
		intents = append(intents, blob.StagedIntent{ID: id, ModTime: modTime})
	}
	return intents, nil
}
