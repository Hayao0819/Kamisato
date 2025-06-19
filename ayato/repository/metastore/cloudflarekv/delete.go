package cloudflarekv

import (
	"github.com/cloudflare/cloudflare-go"

	"github.com/cockroachdb/errors"
)

func (c *CloudflareKV) DeletePackageFileEntry(packageName string) error {
	key := packageName
	_, err := c.client.DeleteWorkersKVEntry(c.ctx, c.accountIdentifier(), cloudflare.DeleteWorkersKVEntryParams{
		NamespaceID: c.namespaceId,
		Key:         key,
	})
	return errors.Wrap(err, "failed to delete worker KV entry")
}
