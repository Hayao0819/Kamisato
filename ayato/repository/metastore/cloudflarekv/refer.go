package cloudflarekv

import "github.com/cloudflare/cloudflare-go"

func (c *CloudflareKV) PackageFile(packageName string) (string, error) {
	key := packageName

	v, err := c.client.GetWorkersKV(c.ctx, c.accountIdentifier(), cloudflare.GetWorkersKVParams{
		NamespaceID: c.namespaceId,
		Key:         key,
	})

	if err != nil {
		return "", nil
	}

	return string(v), nil
}
