package cloudflarekv

import (
	"context"

	"github.com/Hayao0819/Kamisato/ayato/repository/metastore/cloudflarekv/logger"
	"github.com/cloudflare/cloudflare-go"
)

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
