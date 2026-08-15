# Flowpan-light

Author: `yyllaa`

Flowpan-light SDK for calling Flowpan's external 115 storage gateway.

This repository currently publishes only the SDK and API documentation. The main Flowpan-light project will be added later.

## What It Does

The SDK calls:

```text
/api/v1/storage/115/{driver}/{action}
```

It lets another backend project use the 115 storage capabilities exposed by your Flowpan-light service without receiving your Flowpan admin account, 115 Cookie, or OpenAPI credentials.

Supported driver modes:

- `auto`: recommended default. Flowpan-light chooses OpenAPI when available, otherwise Cookie.
- `open`: force OpenAPI.
- `cookie`: force Cookie.

## Included SDKs

- Go: `sdk/go/flowpan115`
- Python: `sdk/python/flowpan115.py`

Both SDKs use only the standard library.

## Go Example

```go
package main

import (
	"context"
	"fmt"

	"github.com/yyllaa/Flowpan-light/sdk/go/flowpan115"
)

func main() {
	client := flowpan115.New("http://127.0.0.1:8088", "external-token")
	ctx := context.Background()

	client.SetDefaultDriver(flowpan115.DriverAuto)
	usage, err := client.Usage(ctx, "")
	if err != nil {
		panic(err)
	}
	fmt.Println(usage.Driver, usage.Result.Used, usage.Result.Available)

	// Force Cookie for one request.
	dir, err := client.EnsureDir(ctx, flowpan115.DriverCookie, 0, "test")
	if err != nil {
		panic(err)
	}
	fmt.Println(dir.Result.CID)

	// Use a dedicated OpenAPI client view.
	openClient := client.WithDefaultDriver(flowpan115.DriverOpen)
	page, err := openClient.ListPage(ctx, "", flowpan115.ListPageRequest{
		CID:     dir.Result.CID,
		Offset:  0,
		Limit:   100,
		ShowDir: flowpan115.Bool(true),
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(page.Result.Total)
}
```

## Python Example

```python
from sdk.python.flowpan115 import DRIVER_AUTO, Flowpan115Client

client = Flowpan115Client("http://127.0.0.1:8088", "external-token")

client.set_default_driver(DRIVER_AUTO)
usage = client.usage()
print(usage.driver, usage.result["used"], usage.result["available"])

# Force Cookie for one request.
dir_res = client.ensure_dir("test", driver="cookie")
cid = dir_res.result["cid"]

# Use a dedicated OpenAPI client view.
open_client = client.with_driver("open")
page = open_client.list_page(cid=cid, limit=100, show_dir=True)
print(page.result["total"])
```

## Authentication

Enable the external API in Flowpan-light and use:

```http
Authorization: Bearer <external_api_token>
```

Do not reuse the MoviePilot plugin event token.

Recommended scopes:

- `read`: list, search, item detail, usage, download URL.
- `write`: create directory, rename, move, copy, paste, star.
- `upload`: upload bridge operations.
- `delete`: delete operations.

## Documentation

- [External API standard](docs/外部115存储API标准.md)
- [SDK guide](docs/外部115存储SDK.md)
