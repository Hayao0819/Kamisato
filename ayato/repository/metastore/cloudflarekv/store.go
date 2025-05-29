package cloudflarekv

import "github.com/cloudflare/cloudflare-go"

func (c *CloudflareKV) StorePackageFile(packageName, filePath string) error {
	key := packageName
	val := []byte(filePath)

	_, err := c.client.WriteWorkersKVEntry(c.ctx, c.accountIdentifier(), cloudflare.WriteWorkersKVEntryParams{
		NamespaceID: c.namespaceId,
		Key:         key,
		Value:       val,
	})

	return err
}
