package cloudflarekv

import (
	utils "github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/cloudflare/cloudflare-go"
)

func (c *CloudflareKV) DeletePackageFileEntry(packageName string) error {
	key := packageName
	_, err := c.client.DeleteWorkersKVEntry(c.ctx, c.accountIdentifier(), cloudflare.DeleteWorkersKVEntryParams{
		NamespaceID: c.namespaceId,
		Key:         key,
	})
	return utils.WrapErr(err, "failed to delete worker KV entry")
}
