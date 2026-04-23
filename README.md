# spiffe-spire-istio

## Part 1 - Learn with Go (hands-on, internal understanding)

### Goal

Build 2 services:

- **server** (accepts mTLS)
- **client** (connects via mTLS)

Both:

- Get identity from SPIRE
- Authenticate each other using SPIFFE IDs

## Step-by-step

1. Initialize Go module
   ```bash
   go mod init a2solution.ca/spiffe-spire-istio
   ```

2. Install Go SPIFFE library
   ```bash
   go get github.com/spiffe/go-spiffe/v2
   ```

3. Resolve dependencies
   ```bash
   go mod tidy
   ```

4. Build the client
   ```bash
   go build -o spiffe-client .
   ```
