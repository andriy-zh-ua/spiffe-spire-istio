package main

import (
	"context"
	"log"
	"math/rand/v2"
	"os"
	"os/user"
	"time"

	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"fmt"
)

// isTemporarySPIREError determines if an error from SPIRE is temporary and worth retrying
//
// Parameters:
// - err: The error to check
//
// Returns:
// - bool: true if the error is temporary, false otherwise
func isTemporarySPIREError(err error) bool {
	// Handle context cancellation first
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	// Handle gRPC status errors
	if statusErr, ok := status.FromError(err); ok {
		switch statusErr.Code() {
		// Temporary errors that might resolve with retries
		case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted,
			codes.Aborted, codes.OutOfRange:
			log.Printf("Classified as temporary SPIRE error: %v (code: %v)", err, statusErr.Code())
			return true
		// Permanent errors that won't be resolved with retries
		case codes.PermissionDenied, codes.Unauthenticated, codes.InvalidArgument,
			codes.NotFound, codes.AlreadyExists, codes.FailedPrecondition:
			log.Printf("Classified as permanent SPIRE error: %v (code: %v)", err, statusErr.Code())
			return false
		}
	}

	errStr := err.Error()
	log.Printf("Unhandled error: %s", errStr)

	// Treat unhandled errors as permanent to avoid infinite retries
	return false
}

// newX509SourceWithRetry creates a new X.509 source with retry logic
//
// Parameters:
// - ctxParent: The parent context for the operation
// - timeout: The timeout for each attempt
// - maxRetries: The maximum number of retries
// - socketPath: The path to the SPIRE agent socket
//
// Returns:
// - *workloadapi.X509Source: The created X.509 source
// - error: An error if the operation fails
func newX509SourceWithRetry(ctxParent context.Context, timeout time.Duration, maxRetries int, socketPath string) (*workloadapi.X509Source, error) {
	// Create a fresh timeout context for each attempt
	ctx, cancel := context.WithTimeout(ctxParent, timeout)
	defer cancel()

	var lastErr error

	// Retry maxRetries times
	for attempt := range maxRetries {
		// Create a new X.509 source
		// - automatic certificate rotation
		// - watches for updates to the certificate
		source, err := workloadapi.NewX509Source(
			ctx,
			// Specify the socket path for the SPIRE agent as SPIFFE_ENDPOINT_SOCKET env variable is not set
			workloadapi.WithClientOptions(
				workloadapi.WithAddr(socketPath),
			),
		)

		if err == nil {
			log.Printf("✅ Successfully created X509Source after %d attempts", attempt+1)
			return source, nil
		}

		lastErr = err

		// Check if the error is temporary
		if !isTemporarySPIREError(err) {
			return nil, fmt.Errorf("permanent error creating X509Source after %d attempts: %w", attempt+1, err)
		}

		// Exponential backoff calculation: 1s, 2s, 4s, 8s, 16s ... capped at timeout, with random jitter
		backoff := time.Duration(1<<uint(attempt)) * time.Second
		backoff = min(backoff, timeout)

		// Add random jitter to the backoff (important in distributed systems)
		backoff = time.Duration(rand.Float64() * float64(backoff))

		log.Printf("⚠️ Temporary error creating X509Source after %d attempts: %v. Retrying in %v...", attempt+1, err, backoff)

		// Wait for context awareness
		select {
		// Wait for the backoff period and continue to the next iteration
		case <-time.After(backoff):
		// Context was cancelled, return the error
		case <-ctxParent.Done():
			return nil, ctxParent.Err()
		}
	}

	return nil, fmt.Errorf("failed to create X509Source after %d attempts: %w", maxRetries, lastErr)
}

func main() {
	const (
		maxRetries = 5
		maxTimeout = 30 * time.Second
	)

	log.Println("========================== SPIFFE Client Starting ==========================")

	currentUser, err := user.Current()
	if err == nil {
		log.Printf("Current username : %s", currentUser.Username)
		log.Printf("Current UID      : %s", currentUser.Uid)
		log.Printf("Current GID      : %s", currentUser.Gid)
	} else {
		log.Printf("❌ Failed to get current user: %v", err)
		os.Exit(1)
	}

	// Get socket path from environment variable
	socketPath := os.Getenv("SPIFFE_ENDPOINT_SOCKET")
	if socketPath == "" {
		log.Println("❌ SPIFFE_ENDPOINT_SOCKET environment variable is not set!")
		os.Exit(1)
	}
	log.Printf("✅ SPIFFE_ENDPOINT_SOCKET found: %s", socketPath)

	log.Println("🚀 Starting SPIFFE workload API client...")

	// Create a context representing the lifetime of the application
	ctxParent, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("🚀 Creating X.509 source...")

	// Create a new X.509 source
	x509Source, err := newX509SourceWithRetry(ctxParent, maxTimeout, maxRetries, socketPath)
	if err != nil {
		log.Printf("❌ Error creating X.509 source: %v", err)
		os.Exit(1)
	}
	defer x509Source.Close()

	// Get the X.509 SVID
	svid, err := x509Source.GetX509SVID()
	if err != nil {
		log.Printf("❌ Error getting X.509 SVID: %v", err)
		os.Exit(1)
	}
	log.Printf("✅ X.509 SVID obtained successfully: %s", svid.ID.String())
	log.Println("Certificate expires on (UTC): " + svid.Certificates[0].NotAfter.Format(time.RFC3339))
	log.Println("Certificate expires on (Local): " + svid.Certificates[0].NotAfter.Local().Format("2006-01-02 15:04:05 MST"))

	// Get the X.509 bundle - Root CA certificate for the trust domain
	bundle, err := x509Source.GetX509BundleForTrustDomain(svid.ID.TrustDomain())
	if err != nil {
		log.Printf("❌ Error getting X.509 bundle: %v", err)
		os.Exit(1)
	}
	log.Println("✅ X.509 Bundle obtained successfully")
	log.Println("Trust Domain: " + bundle.TrustDomain().String())

	// Watch for SVID rotations
	log.Println("🔄 Watching for SVID rotations...")
	for {
		select {
		// Only execute when the SVID is updated
		case <-x509Source.Updated():
			log.Println("🔔 SVID rotation detected!")
			log.Println("⏰ Current time: " + time.Now().Local().Format("2006-01-02 15:04:05 MST"))

			// Get the updated SVID
			svid, err := x509Source.GetX509SVID()
			if err != nil {
				log.Printf("❌ Error getting X.509 SVID: %v", err)
				os.Exit(1)
			}
			log.Println("🔔 SVID rotated!")
			log.Println("New Certificate expires on (UTC): " + svid.Certificates[0].NotAfter.Format(time.RFC3339))
			log.Println("New Certificate expires on (Local): " + svid.Certificates[0].NotAfter.Local().Format("2006-01-02 15:04:05 MST"))

		// Only execute when the context is done
		case <-ctxParent.Done():
			log.Println("🛑 Stop SVID watcher")
			os.Exit(0)
		}
	}
}
