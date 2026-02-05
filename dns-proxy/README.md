# DNS Reverse Proxy

A simple DNS reverse proxy in Go with a "rewrite A AAAA" feature and fallback.

## Features

- **Rewrite A to AAAA**: When an A record is queried, the proxy first tries to fetch AAAA records from the upstream.
- **CNAME Chasing**: If a CNAME is encountered, the proxy automatically follows it to find the final A or AAAA records, ensuring the full chain is returned.
- **Fallback to A**: If no AAAA records are found (even after chasing CNAMEs), the proxy falls back to fetching and returning A records.
- **Support for UDP and TCP**: Listens on both protocols.
- **Configurable**: Upstream DNS server, listen address, and timeout can be configured via flags.

## Usage

### Run the proxy

```bash
go run main.go -listen :8053 -upstream 8.8.8.8:53
```

### Options

- `-listen`: The address to listen on (default `:8053`).
- `-upstream`: The upstream DNS server (default `8.8.8.8:53`).
- `-timeout`: Timeout for upstream queries (default `5s`).

### Testing

You can use `dig` to test the proxy:

```bash
# Query for a domain with AAAA records (should return AAAA records even though A was requested)
dig @localhost -p 8053 google.com A

# Query for a domain with only A records (should fall back to A records)
dig @localhost -p 8053 v4.ident.me A
```

## How it works

1.  The proxy receives a DNS query.
2.  If the query is for type `A`:
    -   It creates a new query for type `AAAA`.
    -   It sends the `AAAA` query to the upstream.
    -   If the upstream returns one or more `AAAA` records, the proxy returns these records to the client, but keeps the original query in the response's Question section (to satisfy most DNS clients).
    -   If the upstream returns no `AAAA` records or the query fails, it proceeds to step 3.
3.  For all other query types (including `A` when `AAAA` was not found):
    -   It forwards the original query to the upstream and returns the response to the client.
