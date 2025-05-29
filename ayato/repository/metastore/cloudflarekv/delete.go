package cloudflarekv

import "github.com/cloudflare/cloudflare-go"

func (c *CloudflareKV) DeletePackageFileEntry(packageName string) error {
	key := packageName
	_, err := c.client.DeleteWorkersKVEntry(c.ctx, c.accountIdentifier(), cloudflare.DeleteWorkersKVEntryParams{
		NamespaceID: c.namespaceId,
		Key:         key,
	})
	return err
}
