# Netlify DNS Zone File Generator

Generate a zone file from your Netlify DNS records.

**Intended use case**: to transfer DNS records from Netlify a platform that supports importing a zone file.

**How does it work**: Netlify does not provide an option to export records to a zone file, but it does provide an [API](https://open-api.netlify.com/#tag/dnsZone/operation/getDnsRecords) that lists them. This tool uses the data from this API and creates a zone file out of it.

## Usage

1. Create a [Netlify Personal Access Token](https://app.netlify.com/user/applications#personal-access-tokens).

### Run locally

1. Clone this repository:
    ```bash
    git clone https://github.com/devindford/netlify-dns-zone-file.git
    ```
1. Export the netlify token as an environment variable:
    ```bash
    export NETLIFY_TOKEN=<your token here>
    ```
1. Add your `netlify.toml` file to the root directory so we can create proper CNAME redirects for those endpoints

1. Run the tool. The output will contain the names of the `.zone` files that were generated.
    ```bash
    go run .
    ```

## Troubleshooting

The tool has only been tested with one domain - when transferring it from Netlify to Cloudflare.
The record types that it has been confirmed to handle include A, CNAME, NETLIFY (ignored), MX and TXT.

If you notice errors when importing the generated zone file, please open [an issue](https://github.com/devindford/netlify-dns-zone-file/issues/new) to report them.

## License

MIT

## References

- [Wikipedia](https://en.wikipedia.org/wiki/Zone_file)
- [Netlify Open API docs](https://open-api.netlify.com/#tag/dnsZone/operation/getDnsRecords)
