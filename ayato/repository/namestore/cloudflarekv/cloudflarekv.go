package cloudflarekv

import (
	"context"
	"errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/namestore/cloudflarekv/logger"
	utils "github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/cloudflare/cloudflare-go"
)

// CloudflareKV は Cloudflare Workers KV ベースの NameStore 実装です。
type CloudflareKV struct {
	client      *cloudflare.API
	accountId   string
	namespaceId string
	ctx         context.Context
}

func (c *CloudflareKV) accountIdentifier() *cloudflare.ResourceContainer {
	return cloudflare.AccountIdentifier(c.accountId)
}

func NewCloudflareKV(account, token, namespace string) (*CloudflareKV, error) {
	c, err := cloudflare.NewWithAPIToken(token, cloudflare.UsingLogger(logger.Default()))
	if err != nil {
		return nil, err
	}
	return &CloudflareKV{
		client:      c,
		accountId:   account,
		namespaceId: namespace,
		ctx:         context.Background(),
	}, nil
}

func (c *CloudflareKV) StorePackageFile(packageName, filePath string) error {
	_, err := c.client.WriteWorkersKVEntry(c.ctx, c.accountIdentifier(), cloudflare.WriteWorkersKVEntryParams{
		NamespaceID: c.namespaceId,
		Key:         packageName,
		Value:       []byte(filePath),
	})

	return err
}

func (c *CloudflareKV) PackageFile(packageName string) (string, error) {
	v, err := c.client.GetWorkersKV(c.ctx, c.accountIdentifier(), cloudflare.GetWorkersKVParams{
		NamespaceID: c.namespaceId,
		Key:         packageName,
	})
	if err != nil {
		// 正当な not-found（キー無し）のみ空文字を返し、それ以外の API エラーは握り潰さない。
		var notFound *cloudflare.NotFoundError
		if errors.As(err, &notFound) {
			return "", nil
		}
		return "", utils.WrapErr(err, "failed to get worker KV entry")
	}

	return string(v), nil
}

func (c *CloudflareKV) DeletePackageFileEntry(packageName string) error {
	_, err := c.client.DeleteWorkersKVEntry(c.ctx, c.accountIdentifier(), cloudflare.DeleteWorkersKVEntryParams{
		NamespaceID: c.namespaceId,
		Key:         packageName,
	})
	return utils.WrapErr(err, "failed to delete worker KV entry")
}
