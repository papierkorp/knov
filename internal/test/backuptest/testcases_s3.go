package backuptest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"knov/internal/backup"
	"knov/internal/logging"
	"knov/internal/test"
)

// caseS3TargetRoundtrip is an opt-in integration check for the S3 BackupTarget: it only runs when
// KNOV_TEST_S3_ENDPOINT is set (point it at a throwaway MinIO / scratch bucket, with
// KNOV_TEST_S3_BUCKET and, if the endpoint needs them, KNOV_TEST_S3_ACCESS_KEY /
// KNOV_TEST_S3_SECRET_KEY / KNOV_TEST_S3_REGION / KNOV_TEST_S3_USE_SSL); otherwise it reports
// success as skipped. It exercises the minio-go quirks target_s3.go is written around - the
// up-front stat probes that turn a lazy GetObject into a prompt not-found, the already-exists
// guard on Write, idempotent Unlock, and the read-modify-write event log surviving a fresh target
// instance - none of which the localTarget cases can catch. Everything lands under a unique
// knov-backuptest-<ns>/ key prefix that the case sweeps on the way out, so it never collides with
// real backup objects even against a shared bucket.
func caseS3TargetRoundtrip() test.CaseResult {
	name := "s3-target-roundtrip"

	endpoint := os.Getenv("KNOV_TEST_S3_ENDPOINT")
	if endpoint == "" {
		return test.SkipCase(name, "KNOV_TEST_S3_ENDPOINT not set")
	}

	bucket := os.Getenv("KNOV_TEST_S3_BUCKET")
	if bucket == "" {
		return errCase(name, fmt.Errorf("KNOV_TEST_S3_ENDPOINT set without KNOV_TEST_S3_BUCKET"))
	}
	useSSL, _ := strconv.ParseBool(os.Getenv("KNOV_TEST_S3_USE_SSL"))
	cfg := backup.S3Config{
		Endpoint:  endpoint,
		Region:    os.Getenv("KNOV_TEST_S3_REGION"),
		Bucket:    bucket,
		Prefix:    fmt.Sprintf("knov-backuptest-%d/", time.Now().UnixNano()),
		AccessKey: os.Getenv("KNOV_TEST_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("KNOV_TEST_S3_SECRET_KEY"),
		UseSSL:    useSSL,
	}

	target, err := backup.NewS3Target(cfg)
	if err != nil {
		return errCase(name, err)
	}
	defer sweepS3Prefix(cfg)

	payload := bytes.Repeat([]byte("knov-s3-probe\n"), 1024)
	if err := target.Write("set-a", bytes.NewReader(payload)); err != nil {
		return errCase(name, err)
	}
	dupErr := target.Write("set-a", bytes.NewReader(payload)) // exists() guard, not a silent overwrite
	_, missErr := target.Read("set-b")                        // lazy-GetObject quirk: must fail promptly

	rc, err := target.Read("set-a")
	if err != nil {
		return errCase(name, err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return errCase(name, err)
	}

	names, err := target.List()
	if err != nil {
		return errCase(name, err)
	}

	if err := target.Lock("set-a"); err != nil {
		return errCase(name, err)
	}
	locked, err := target.Locked("set-a")
	if err != nil {
		return errCase(name, err)
	}
	lockedNames, err := target.LockedNames()
	if err != nil {
		return errCase(name, err)
	}
	if err := target.Unlock("set-a"); err != nil {
		return errCase(name, err)
	}
	unlockAgainErr := target.Unlock("set-a") // S3 DELETE is idempotent - must not error

	if err := target.LogEvent(backup.EventBackup, "set-a", backup.SourceManual); err != nil {
		return errCase(name, err)
	}
	if err := target.LogEvent(backup.EventRestore, "set-a", backup.SourceRestore); err != nil {
		return errCase(name, err)
	}
	reopened, err := backup.NewS3Target(cfg)
	if err != nil {
		return errCase(name, err)
	}
	events, err := reopened.Events()
	if err != nil {
		return errCase(name, err)
	}

	if err := target.Delete("set-a"); err != nil {
		return errCase(name, err)
	}
	after, err := target.List()
	if err != nil {
		return errCase(name, err)
	}

	ok := dupErr != nil &&
		missErr != nil &&
		bytes.Equal(got, payload) &&
		len(names) == 1 && names[0] == "set-a" &&
		locked && lockedNames["set-a"] &&
		unlockAgainErr == nil &&
		len(events) == 2 &&
		events[0].Kind == backup.EventBackup && events[1].Kind == backup.EventRestore &&
		len(after) == 0

	cr := test.CaseResult{
		Name:     name,
		Expected: "Write guards duplicates, Read of a missing set errors promptly, bytes/list/lock/event-log round-trip, Unlock is idempotent, Delete clears the set",
		Actual: fmt.Sprintf("dupErr=%v missErr=%v bytesOK=%v names=%v locked=%v lockedNames=%v unlockAgainErr=%v events=%d after=%v",
			dupErr != nil, missErr != nil, bytes.Equal(got, payload), names, locked, lockedNames, unlockAgainErr, len(events), after),
		Success: ok,
	}
	if !ok {
		cr.Error = "S3 BackupTarget did not round-trip as expected"
	}
	return cr
}

// sweepS3Prefix removes every object under cfg.Prefix - teardown for caseS3TargetRoundtrip, which
// uses a unique prefix per run so this can never touch anything else in the bucket.
func sweepS3Prefix(cfg backup.S3Config) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		logging.LogWarning(logging.KeyApp, "backuptest: s3 sweep skipped, client init failed: %v", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for obj := range client.ListObjects(ctx, cfg.Bucket, minio.ListObjectsOptions{Prefix: cfg.Prefix, Recursive: true}) {
		if obj.Err != nil {
			logging.LogWarning(logging.KeyApp, "backuptest: s3 sweep list error under %s: %v", cfg.Prefix, obj.Err)
			continue
		}
		if err := client.RemoveObject(ctx, cfg.Bucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
			logging.LogWarning(logging.KeyApp, "backuptest: s3 sweep failed to remove %s: %v", obj.Key, err)
		}
	}
}
